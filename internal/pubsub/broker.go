package pubsub

import (
	"container/list"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrBrokerStopped         = errors.New("pubsub: broker is stopped")
	ErrConsumerNotFound      = errors.New("pubsub: consumer not found")
	ErrConsumerExists        = errors.New("pubsub: consumer already exists")
	ErrSubscriptionNotFound  = errors.New("pubsub: subscription not found")
	ErrTopicInvalid          = errors.New("pubsub: invalid topic format")
	ErrMessageNotFound       = errors.New("pubsub: message not found")
	ErrBackpressureFull      = errors.New("pubsub: consumer backpressure buffer full")
	ErrConsumerDisconnected  = errors.New("pubsub: consumer is disconnected")
)

const (
	defaultAckTimeout     = 30 * time.Second
	defaultMaxRetry       = 3
	defaultMaxUnacked     = 100
	defaultConsumerBuffer = 1024
)

type Config struct {
	AckTimeout     time.Duration
	MaxRetry       int
	MaxUnacked     int
	ConsumerBuffer int
}

func DefaultConfig() Config {
	return Config{
		AckTimeout:     defaultAckTimeout,
		MaxRetry:       defaultMaxRetry,
		MaxUnacked:     defaultMaxUnacked,
		ConsumerBuffer: defaultConsumerBuffer,
	}
}

type Message struct {
	ID        string
	Topic     string
	Payload   interface{}
	Timestamp time.Time
	RetryCount int
}

type MessageStatus int

const (
	MessageStatusPending MessageStatus = iota
	MessageStatusDelivered
	MessageStatusAcked
	MessageStatusDead
)

type pendingMessage struct {
	msg          *Message
	status       MessageStatus
	deliverAt    time.Time
	lastDeliver  time.Time
	subscriberID string
	element      *list.Element
}

type Consumer struct {
	unackedCount  int64
	ID            string
	ch            chan *Message
	connected     bool
	disconnected  bool
	closeOnce     sync.Once
	maxUnacked    int
	pending       map[string]*pendingMessage
	pendingList   *list.List
	durableBuffer []*Message
}

type Subscription struct {
	ConsumerID  string
	TopicFilter string
	Durable     bool
}

type topicNode struct {
	children    map[string]*topicNode
	subscribers map[string]*Subscription
	wildcardOne map[string]*Subscription
	wildcardAny map[string]*Subscription
}

func newTopicNode() *topicNode {
	return &topicNode{
		children:    make(map[string]*topicNode),
		subscribers: make(map[string]*Subscription),
		wildcardOne: make(map[string]*Subscription),
		wildcardAny: make(map[string]*Subscription),
	}
}

func (n *topicNode) isEmpty() bool {
	return len(n.children) == 0 &&
		len(n.subscribers) == 0 &&
		len(n.wildcardOne) == 0 &&
		len(n.wildcardAny) == 0
}

type Broker struct {
	nextMsgID     uint64
	cfg           Config
	mu            sync.RWMutex
	consumers     map[string]*Consumer
	subscriptions map[string][]*Subscription
	topicTree     *topicNode
	deadLetters   []*Message
	running       bool
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

func NewBroker(cfg Config) *Broker {
	if cfg.AckTimeout <= 0 {
		cfg.AckTimeout = defaultAckTimeout
	}
	if cfg.MaxRetry < 0 {
		cfg.MaxRetry = defaultMaxRetry
	}
	if cfg.MaxUnacked <= 0 {
		cfg.MaxUnacked = defaultMaxUnacked
	}
	if cfg.ConsumerBuffer <= 0 {
		cfg.ConsumerBuffer = defaultConsumerBuffer
	}

	return &Broker{
		cfg:           cfg,
		consumers:     make(map[string]*Consumer),
		subscriptions: make(map[string][]*Subscription),
		topicTree:     newTopicNode(),
		deadLetters:   make([]*Message, 0),
		stopCh:        make(chan struct{}),
		running:       true,
	}
}

func (b *Broker) generateMsgID() string {
	id := atomic.AddUint64(&b.nextMsgID, 1)
	return fmt.Sprintf("msg-%d-%d", time.Now().UnixNano(), id)
}

func (b *Broker) validateTopic(topic string) error {
	if topic == "" {
		return ErrTopicInvalid
	}
	parts := strings.Split(topic, ".")
	for _, p := range parts {
		if p == "" {
			return ErrTopicInvalid
		}
	}
	return nil
}

func (b *Broker) validateFilter(filter string) error {
	if filter == "" {
		return ErrTopicInvalid
	}
	parts := strings.Split(filter, ".")
	for i, p := range parts {
		if p == "" {
			return ErrTopicInvalid
		}
		if p == "#" && i != len(parts)-1 {
			return ErrTopicInvalid
		}
		if p != "*" && p != "#" {
			if strings.ContainsAny(p, "*#") {
				return ErrTopicInvalid
			}
		}
	}
	return nil
}

func (b *Broker) matchTopic(filter, topic string) bool {
	if err := b.validateFilter(filter); err != nil {
		return false
	}
	if err := b.validateTopic(topic); err != nil {
		return false
	}
	filterParts := strings.Split(filter, ".")
	topicParts := strings.Split(topic, ".")
	return matchParts(filterParts, topicParts, 0, 0)
}

func matchParts(filterParts, topicParts []string, fi, ti int) bool {
	if fi == len(filterParts) {
		return ti == len(topicParts)
	}

	fp := filterParts[fi]

	if fp == "#" {
		for i := ti; i <= len(topicParts); i++ {
			if matchParts(filterParts, topicParts, fi+1, i) {
				return true
			}
		}
		return false
	}

	if ti >= len(topicParts) {
		return false
	}

	if fp == "*" {
		return matchParts(filterParts, topicParts, fi+1, ti+1)
	}

	if fp == topicParts[ti] {
		return matchParts(filterParts, topicParts, fi+1, ti+1)
	}

	return false
}

func (b *Broker) addToTopicTree(sub *Subscription) {
	filter := sub.TopicFilter
	parts := strings.Split(filter, ".")
	node := b.topicTree

	for i, p := range parts {
		if i == len(parts)-1 {
			switch p {
			case "#":
				node.wildcardAny[sub.ConsumerID] = sub
			case "*":
				node.wildcardOne[sub.ConsumerID] = sub
			default:
				if _, ok := node.children[p]; !ok {
					node.children[p] = newTopicNode()
				}
				node.children[p].subscribers[sub.ConsumerID] = sub
			}
		} else {
			if _, ok := node.children[p]; !ok {
				node.children[p] = newTopicNode()
			}
			node = node.children[p]
		}
	}
}

func (b *Broker) removeFromTopicTree(sub *Subscription) {
	filter := sub.TopicFilter
	parts := strings.Split(filter, ".")
	node := b.topicTree

	type pathEntry struct {
		parent *topicNode
		part   string
	}
	path := make([]pathEntry, 0, len(parts))

	for i, p := range parts {
		if i == len(parts)-1 {
			switch p {
			case "#":
				delete(node.wildcardAny, sub.ConsumerID)
			case "*":
				delete(node.wildcardOne, sub.ConsumerID)
			default:
				if child, ok := node.children[p]; ok {
					delete(child.subscribers, sub.ConsumerID)
					if child.isEmpty() {
						delete(node.children, p)
					}
				}
			}
		} else {
			if child, ok := node.children[p]; ok {
				path = append(path, pathEntry{parent: node, part: p})
				node = child
			} else {
				return
			}
		}
	}

	for i := len(path) - 1; i >= 0; i-- {
		entry := path[i]
		if child, ok := entry.parent.children[entry.part]; ok {
			if child.isEmpty() {
				delete(entry.parent.children, entry.part)
			} else {
				break
			}
		}
	}
}

func (b *Broker) findMatchingSubs(topic string) []*Subscription {
	result := make([]*Subscription, 0)
	parts := strings.Split(topic, ".")
	b.collectMatches(b.topicTree, parts, 0, &result)
	return result
}

func (b *Broker) collectMatches(node *topicNode, parts []string, idx int, result *[]*Subscription) {
	for _, sub := range node.wildcardAny {
		*result = append(*result, sub)
	}

	if idx == len(parts) {
		return
	}

	part := parts[idx]
	isLast := idx == len(parts)-1

	if isLast {
		for _, sub := range node.wildcardOne {
			*result = append(*result, sub)
		}
	}

	if child, ok := node.children["*"]; ok {
		if isLast {
			for _, sub := range child.subscribers {
				*result = append(*result, sub)
			}
			for _, sub := range child.wildcardAny {
				*result = append(*result, sub)
			}
		} else {
			b.collectMatches(child, parts, idx+1, result)
		}
	}

	if child, ok := node.children[part]; ok {
		if isLast {
			for _, sub := range child.subscribers {
				*result = append(*result, sub)
			}
			for _, sub := range child.wildcardAny {
				*result = append(*result, sub)
			}
		} else {
			b.collectMatches(child, parts, idx+1, result)
		}
	}
}

func (b *Broker) AddConsumer(id string) (<-chan *Message, error) {
	return b.AddConsumerWithOptions(id, b.cfg.MaxUnacked)
}

func (b *Broker) AddConsumerWithOptions(id string, maxUnacked int) (<-chan *Message, error) {
	if id == "" {
		return nil, ErrConsumerNotFound
	}
	if maxUnacked <= 0 {
		maxUnacked = b.cfg.MaxUnacked
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return nil, ErrBrokerStopped
	}

	if _, exists := b.consumers[id]; exists {
		return nil, ErrConsumerExists
	}

	c := &Consumer{
		ID:           id,
		ch:           make(chan *Message, b.cfg.ConsumerBuffer),
		connected:    true,
		disconnected: false,
		maxUnacked:   maxUnacked,
		pending:      make(map[string]*pendingMessage),
		pendingList:  list.New(),
	}

	b.consumers[id] = c
	return c.ch, nil
}

func (b *Broker) RemoveConsumer(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return ErrBrokerStopped
	}

	c, exists := b.consumers[id]
	if !exists {
		return ErrConsumerNotFound
	}

	subs, ok := b.subscriptions[id]
	if ok {
		for _, sub := range subs {
			b.removeFromTopicTree(sub)
		}
		delete(b.subscriptions, id)
	}

	c.disconnected = true
	c.connected = false
	c.closeOnce.Do(func() {
		close(c.ch)
	})

	delete(b.consumers, id)
	return nil
}

func (b *Broker) Subscribe(consumerID, topicFilter string) error {
	return b.SubscribeDurable(consumerID, topicFilter, false)
}

func (b *Broker) SubscribeDurable(consumerID, topicFilter string, durable bool) error {
	if err := b.validateFilter(topicFilter); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return ErrBrokerStopped
	}

	_, exists := b.consumers[consumerID]
	if !exists {
		return ErrConsumerNotFound
	}

	subs := b.subscriptions[consumerID]
	for _, s := range subs {
		if s.TopicFilter == topicFilter {
			return nil
		}
	}

	sub := &Subscription{
		ConsumerID:  consumerID,
		TopicFilter: topicFilter,
		Durable:     durable,
	}

	subs = append(subs, sub)
	b.subscriptions[consumerID] = subs
	b.addToTopicTree(sub)

	return nil
}

func (b *Broker) Unsubscribe(consumerID, topicFilter string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return ErrBrokerStopped
	}

	subs, ok := b.subscriptions[consumerID]
	if !ok {
		return ErrSubscriptionNotFound
	}

	found := -1
	var sub *Subscription
	for i, s := range subs {
		if s.TopicFilter == topicFilter {
			found = i
			sub = s
			break
		}
	}

	if found == -1 {
		return ErrSubscriptionNotFound
	}

	b.removeFromTopicTree(sub)
	subs = append(subs[:found], subs[found+1:]...)
	if len(subs) == 0 {
		delete(b.subscriptions, consumerID)
	} else {
		b.subscriptions[consumerID] = subs
	}

	return nil
}

func (b *Broker) Publish(topic string, payload interface{}) (string, error) {
	if err := b.validateTopic(topic); err != nil {
		return "", err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return "", ErrBrokerStopped
	}

	msg := &Message{
		ID:        b.generateMsgID(),
		Topic:     topic,
		Payload:   payload,
		Timestamp: time.Now(),
	}

	subs := b.findMatchingSubs(topic)

	type consumerSub struct {
		c       *Consumer
		durable bool
	}
	consumerMap := make(map[string]*consumerSub)
	for _, sub := range subs {
		c, ok := b.consumers[sub.ConsumerID]
		if !ok {
			continue
		}
		if cs, exists := consumerMap[sub.ConsumerID]; exists {
			if sub.Durable {
				cs.durable = true
			}
		} else {
			consumerMap[sub.ConsumerID] = &consumerSub{
				c:       c,
				durable: sub.Durable,
			}
		}
	}

	for _, cs := range consumerMap {
		dummySub := &Subscription{
			ConsumerID:  cs.c.ID,
			TopicFilter: topic,
			Durable:     cs.durable,
		}
		b.deliverToConsumer(cs.c, dummySub, msg)
	}

	return msg.ID, nil
}

func (b *Broker) deliverToConsumer(c *Consumer, sub *Subscription, msg *Message) {
	if !c.connected {
		if sub.Durable {
			c.durableBuffer = append(c.durableBuffer, msg)
		}
		return
	}

	if atomic.LoadInt64(&c.unackedCount) >= int64(c.maxUnacked) {
		if sub.Durable {
			c.durableBuffer = append(c.durableBuffer, msg)
		}
		return
	}

	msgCopy := *msg
	msgCopy.RetryCount = 0

	select {
	case c.ch <- &msgCopy:
		pm := &pendingMessage{
			msg:          &msgCopy,
			status:       MessageStatusDelivered,
			deliverAt:    time.Now().Add(b.cfg.AckTimeout),
			lastDeliver:  time.Now(),
			subscriberID: sub.ConsumerID,
		}
		pm.element = c.pendingList.PushBack(pm)
		c.pending[msgCopy.ID] = pm
		atomic.AddInt64(&c.unackedCount, 1)
	default:
		if sub.Durable {
			c.durableBuffer = append(c.durableBuffer, msg)
		}
	}
}

func (b *Broker) Ack(consumerID, messageID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return ErrBrokerStopped
	}

	c, ok := b.consumers[consumerID]
	if !ok {
		return ErrConsumerNotFound
	}

	pm, ok := c.pending[messageID]
	if !ok {
		return ErrMessageNotFound
	}

	if pm.status != MessageStatusDelivered && pm.status != MessageStatusPending {
		return ErrMessageNotFound
	}

	pm.status = MessageStatusAcked
	c.pendingList.Remove(pm.element)
	delete(c.pending, messageID)
	atomic.AddInt64(&c.unackedCount, -1)

	b.flushDurableBuffer(c)

	return nil
}

func (b *Broker) flushDurableBuffer(c *Consumer) {
	if len(c.durableBuffer) == 0 {
		return
	}

	subs, ok := b.subscriptions[c.ID]
	if !ok || len(subs) == 0 {
		return
	}

	buffer := c.durableBuffer
	c.durableBuffer = make([]*Message, 0)

	for _, msg := range buffer {
		if atomic.LoadInt64(&c.unackedCount) >= int64(c.maxUnacked) {
			c.durableBuffer = append(c.durableBuffer, msg)
			continue
		}

		matched := false
		for _, sub := range subs {
			if b.matchTopic(sub.TopicFilter, msg.Topic) {
				matched = true
				b.deliverToConsumer(c, sub, msg)
				break
			}
		}
		if !matched {
			continue
		}
	}
}

func (b *Broker) Nack(consumerID, messageID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return ErrBrokerStopped
	}

	c, ok := b.consumers[consumerID]
	if !ok {
		return ErrConsumerNotFound
	}

	pm, ok := c.pending[messageID]
	if !ok {
		return ErrMessageNotFound
	}

	if pm.status != MessageStatusDelivered && pm.status != MessageStatusPending {
		return ErrMessageNotFound
	}

	b.redeliverOrDeadLetter(c, pm)
	return nil
}

func (b *Broker) redeliverOrDeadLetter(c *Consumer, pm *pendingMessage) {
	if pm.status != MessageStatusDelivered && pm.status != MessageStatusPending {
		return
	}

	pm.msg.RetryCount++

	c.pendingList.Remove(pm.element)
	delete(c.pending, pm.msg.ID)
	atomic.AddInt64(&c.unackedCount, -1)

	if pm.msg.RetryCount > b.cfg.MaxRetry {
		pm.status = MessageStatusDead
		b.deadLetters = append(b.deadLetters, pm.msg)
		b.flushDurableBuffer(c)
		return
	}

	if !c.connected {
		pm.status = MessageStatusPending
		return
	}

	if atomic.LoadInt64(&c.unackedCount) >= int64(c.maxUnacked) {
		pm.status = MessageStatusPending
		c.durableBuffer = append(c.durableBuffer, pm.msg)
		return
	}

	msgCopy := *pm.msg
	select {
	case c.ch <- &msgCopy:
		newPM := &pendingMessage{
			msg:          &msgCopy,
			status:       MessageStatusDelivered,
			deliverAt:    time.Now().Add(b.cfg.AckTimeout),
			lastDeliver:  time.Now(),
			subscriberID: c.ID,
		}
		newPM.element = c.pendingList.PushBack(newPM)
		c.pending[msgCopy.ID] = newPM
		atomic.AddInt64(&c.unackedCount, 1)
	default:
		pm.status = MessageStatusPending
		c.durableBuffer = append(c.durableBuffer, pm.msg)
		b.flushDurableBuffer(c)
	}
}

func (b *Broker) DisconnectConsumer(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return ErrBrokerStopped
	}

	c, exists := b.consumers[id]
	if !exists {
		return ErrConsumerNotFound
	}

	if !c.connected {
		return nil
	}

	c.connected = false
	subs, ok := b.subscriptions[id]
	if ok {
		for _, sub := range subs {
			if sub.Durable {
				for e := c.pendingList.Front(); e != nil; e = e.Next() {
					pm := e.Value.(*pendingMessage)
					if pm.status == MessageStatusDelivered {
						pm.status = MessageStatusPending
						c.durableBuffer = append(c.durableBuffer, pm.msg)
					}
				}
				c.pendingList.Init()
				atomic.StoreInt64(&c.unackedCount, 0)
				break
			}
		}
	}

	return nil
}

func (b *Broker) ReconnectConsumer(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return ErrBrokerStopped
	}

	c, exists := b.consumers[id]
	if !exists {
		return ErrConsumerNotFound
	}

	if c.connected {
		return nil
	}

	if c.disconnected {
		return ErrConsumerDisconnected
	}

	c.connected = true
	b.flushDurableBuffer(c)
	return nil
}

func (b *Broker) ProcessTimeouts() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return
	}

	now := time.Now()

	for _, c := range b.consumers {
		var next *list.Element
		for e := c.pendingList.Front(); e != nil; e = next {
			next = e.Next()
			pm := e.Value.(*pendingMessage)
			if pm.status == MessageStatusDelivered && now.After(pm.deliverAt) {
				b.redeliverOrDeadLetter(c, pm)
			}
		}
	}
}

func (b *Broker) Start() {
	b.mu.Lock()
	if !b.running {
		b.running = true
		b.stopCh = make(chan struct{})
	}
	b.mu.Unlock()

	b.wg.Add(1)
	go b.timeoutLoop()
}

func (b *Broker) timeoutLoop() {
	defer b.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.ProcessTimeouts()
		}
	}
}

func (b *Broker) Stop() {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return
	}
	b.running = false
	close(b.stopCh)

	for _, c := range b.consumers {
		c.disconnected = true
		c.connected = false
		c.closeOnce.Do(func() {
			close(c.ch)
		})
	}
	b.mu.Unlock()

	b.wg.Wait()
}

func (b *Broker) GetDeadLetters() []*Message {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]*Message, len(b.deadLetters))
	for i, msg := range b.deadLetters {
		msgCopy := *msg
		result[i] = &msgCopy
	}
	return result
}

func (b *Broker) DeadLetterCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.deadLetters)
}

func (b *Broker) ClearDeadLetters() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.deadLetters = b.deadLetters[:0]
}

func (b *Broker) UnackedCount(consumerID string) (int, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	c, ok := b.consumers[consumerID]
	if !ok {
		return 0, ErrConsumerNotFound
	}

	return int(atomic.LoadInt64(&c.unackedCount)), nil
}

func (b *Broker) PendingCount(consumerID string) (int, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	c, ok := b.consumers[consumerID]
	if !ok {
		return 0, ErrConsumerNotFound
	}

	return len(c.durableBuffer), nil
}

func (b *Broker) ConsumerCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.consumers)
}

func (b *Broker) SubscriptionCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	count := 0
	for _, subs := range b.subscriptions {
		count += len(subs)
	}
	return count
}

func (b *Broker) GetMessageStatus(consumerID, messageID string) (MessageStatus, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	c, ok := b.consumers[consumerID]
	if !ok {
		return 0, ErrConsumerNotFound
	}

	pm, ok := c.pending[messageID]
	if ok {
		return pm.status, nil
	}

	for _, dl := range b.deadLetters {
		if dl.ID == messageID {
			return MessageStatusDead, nil
		}
	}

	return 0, ErrMessageNotFound
}
