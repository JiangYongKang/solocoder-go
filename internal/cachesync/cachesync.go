package cachesync

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrNodeNotFound       = errors.New("cachesync: node not found")
	ErrNodeExists         = errors.New("cachesync: node already exists")
	ErrKeyNotFound        = errors.New("cachesync: key not found")
	ErrLockTimeout        = errors.New("cachesync: lock acquisition timed out")
	ErrLockNotHeld        = errors.New("cachesync: lock not held by caller")
	ErrClusterStopped     = errors.New("cachesync: cluster is stopped")
	ErrInvalidMessageType = errors.New("cachesync: invalid message type")
	ErrVersionTooOld      = errors.New("cachesync: update rejected due to older version")
	ErrMessageDropped     = errors.New("cachesync: message dropped due to full channel")
)

const (
	defaultLockTimeout    = 5 * time.Second
	defaultReconcileInterval = 2 * time.Second
	defaultMessageBuffer  = 1024
)

type MessageType int

const (
	MsgUpdateNotify MessageType = iota
	MsgInvalidate
	MsgReconcileRequest
	MsgReconcileResponse
	MsgLockAcquire
	MsgLockRelease
	MsgLockGranted
	MsgLockDenied
)

type CacheEntry struct {
	Key       string
	Value     interface{}
	Version   uint64
	UpdatedAt time.Time
	NodeID    string
}

type Message struct {
	Type       MessageType
	FromNodeID string
	ToNodeID   string
	Key        string
	Value      interface{}
	Version    uint64
	Timestamp  time.Time
	LockHolder string
	LockTTL    time.Duration
	Entries    map[string]uint64
}

type lockInfo struct {
	holder     string
	expiresAt  time.Time
	acquiredAt time.Time
}

type Config struct {
	LockTimeout       time.Duration
	ReconcileInterval time.Duration
	MessageBuffer     int
}

func DefaultConfig() Config {
	return Config{
		LockTimeout:       defaultLockTimeout,
		ReconcileInterval: defaultReconcileInterval,
		MessageBuffer:     defaultMessageBuffer,
	}
}

type pendingLockStatus int

const (
	pendingLockStatusPending   pendingLockStatus = iota
	pendingLockStatusSucceeded
	pendingLockStatusFailed
)

type pendingLock struct {
	grantCh    chan bool
	denyCh     chan string
	grantedBy  map[string]struct{}
	grantedMu  sync.Mutex
	status     pendingLockStatus
	statusMu   sync.RWMutex
}

func assertStatusTransitionValid(oldStatus, newStatus pendingLockStatus) {
	if oldStatus != pendingLockStatusPending {
		panic(fmt.Sprintf("cachesync: invalid pendingLock status transition: %d -> %d (only Pending can transition to other states)", oldStatus, newStatus))
	}
}

type VersionRejectEvent struct {
	Key          string
	LocalVersion uint64
	MsgVersion   uint64
	FromNodeID   string
	RejectedAt   time.Time
}

type VersionRejectHandler func(event VersionRejectEvent)

const releaseMarkTTL = 5 * time.Second

type releaseMark struct {
	timestamp time.Time
	expiresAt time.Time
}

type Node struct {
	msgSent             uint64
	msgRecv             uint64
	rejectCount         uint64
	ID                  string
	cache               map[string]*CacheEntry
	locks               map[string]*lockInfo
	pendingLocks        map[string]*pendingLock
	releaseMarks        map[string]map[string]*releaseMark
	rejectHandlers      []VersionRejectHandler
	cacheMu             sync.RWMutex
	lockMu              sync.Mutex
	pendingLockMu       sync.Mutex
	rejectHandlerMu     sync.RWMutex
	inbox               chan *Message
	cluster             *Cluster
	running             bool
	stopCh              chan struct{}
	wg                  sync.WaitGroup
}

type Cluster struct {
	cfg          Config
	nodes        map[string]*Node
	nodesMu      sync.RWMutex
	running      bool
	stopCh       chan struct{}
	wg           sync.WaitGroup
	msgDropRate  float64
	msgDropMu    sync.Mutex
}

func NewCluster(cfg Config) *Cluster {
	if cfg.LockTimeout <= 0 {
		cfg.LockTimeout = defaultLockTimeout
	}
	if cfg.ReconcileInterval <= 0 {
		cfg.ReconcileInterval = defaultReconcileInterval
	}
	if cfg.MessageBuffer <= 0 {
		cfg.MessageBuffer = defaultMessageBuffer
	}

	return &Cluster{
		cfg:     cfg,
		nodes:   make(map[string]*Node),
		running: true,
		stopCh:  make(chan struct{}),
	}
}

func (c *Cluster) SetMessageDropRate(rate float64) {
	c.msgDropMu.Lock()
	defer c.msgDropMu.Unlock()
	c.msgDropRate = rate
}

func (c *Cluster) shouldDropMessage() bool {
	c.msgDropMu.Lock()
	defer c.msgDropMu.Unlock()
	if c.msgDropRate <= 0 {
		return false
	}
	return time.Now().UnixNano()%1000 < int64(c.msgDropRate*1000)
}

func (c *Cluster) AddNode(nodeID string) (*Node, error) {
	if nodeID == "" {
		return nil, ErrNodeNotFound
	}

	c.nodesMu.Lock()
	defer c.nodesMu.Unlock()

	if !c.running {
		return nil, ErrClusterStopped
	}

	if _, exists := c.nodes[nodeID]; exists {
		return nil, ErrNodeExists
	}

	node := &Node{
		ID:              nodeID,
		cache:           make(map[string]*CacheEntry),
		locks:           make(map[string]*lockInfo),
		pendingLocks:    make(map[string]*pendingLock),
		releaseMarks:    make(map[string]map[string]*releaseMark),
		rejectHandlers:  make([]VersionRejectHandler, 0),
		inbox:           make(chan *Message, c.cfg.MessageBuffer),
		cluster:         c,
		running:         true,
		stopCh:          make(chan struct{}),
	}

	c.nodes[nodeID] = node

	node.wg.Add(1)
	go node.messageLoop()

	return node, nil
}

func (c *Cluster) RemoveNode(nodeID string) error {
	c.nodesMu.Lock()
	node, exists := c.nodes[nodeID]
	if !exists {
		c.nodesMu.Unlock()
		return ErrNodeNotFound
	}
	delete(c.nodes, nodeID)
	c.nodesMu.Unlock()

	node.running = false
	close(node.stopCh)
	node.wg.Wait()
	close(node.inbox)

	return nil
}

func (c *Cluster) GetNode(nodeID string) (*Node, error) {
	c.nodesMu.RLock()
	defer c.nodesMu.RUnlock()

	node, exists := c.nodes[nodeID]
	if !exists {
		return nil, ErrNodeNotFound
	}
	return node, nil
}

func (c *Cluster) NodeCount() int {
	c.nodesMu.RLock()
	defer c.nodesMu.RUnlock()
	return len(c.nodes)
}

func (c *Cluster) broadcast(fromNodeID string, msg *Message) {
	c.nodesMu.RLock()
	nodes := make([]*Node, 0, len(c.nodes))
	for id, node := range c.nodes {
		if id == fromNodeID {
			continue
		}
		if c.shouldDropMessage() {
			continue
		}
		nodes = append(nodes, node)
	}
	c.nodesMu.RUnlock()

	for _, node := range nodes {
		msgCopy := *msg
		msgCopy.ToNodeID = node.ID
		func() {
			defer func() { recover() }()
			select {
			case node.inbox <- &msgCopy:
				atomic.AddUint64(&node.msgRecv, 1)
			default:
			}
		}()
	}
}

func (c *Cluster) unicast(fromNodeID, toNodeID string, msg *Message) (err error) {
	c.nodesMu.RLock()
	node, exists := c.nodes[toNodeID]
	running := c.running
	c.nodesMu.RUnlock()

	if !exists || !running {
		return ErrNodeNotFound
	}

	if c.shouldDropMessage() {
		return nil
	}

	msgCopy := *msg
	msgCopy.FromNodeID = fromNodeID
	msgCopy.ToNodeID = toNodeID

	defer func() {
		if r := recover(); r != nil {
			err = ErrClusterStopped
		}
	}()

	select {
	case node.inbox <- &msgCopy:
		atomic.AddUint64(&node.msgRecv, 1)
		return nil
	default:
		return ErrMessageDropped
	}
}

func (c *Cluster) reliableUnicast(fromNodeID, toNodeID string, msg *Message) error {
	c.nodesMu.RLock()
	node, exists := c.nodes[toNodeID]
	running := c.running
	c.nodesMu.RUnlock()

	if !exists || !running {
		return ErrNodeNotFound
	}

	if c.shouldDropMessage() {
		return nil
	}

	msgCopy := *msg
	msgCopy.FromNodeID = fromNodeID
	msgCopy.ToNodeID = toNodeID

	defer func() { recover() }()

	retries := 5
	retryDelay := 20 * time.Millisecond
	for i := 0; i < retries; i++ {
		select {
		case node.inbox <- &msgCopy:
			atomic.AddUint64(&node.msgRecv, 1)
			return nil
		case <-time.After(50 * time.Millisecond):
			if i < retries-1 {
				time.Sleep(retryDelay)
				continue
			}
		}
	}
	return ErrMessageDropped
}

func (c *Cluster) Stop() {
	c.nodesMu.Lock()
	if !c.running {
		c.nodesMu.Unlock()
		return
	}
	c.running = false
	close(c.stopCh)

	nodes := make([]*Node, 0, len(c.nodes))
	for _, n := range c.nodes {
		nodes = append(nodes, n)
	}
	c.nodesMu.Unlock()

	for _, node := range nodes {
		node.running = false
		close(node.stopCh)
	}

	c.wg.Wait()
	for _, node := range nodes {
		node.wg.Wait()
		close(node.inbox)
	}
}

func (c *Cluster) StartReconciler() {
	c.wg.Add(1)
	go c.reconcileLoop()
}

func (c *Cluster) reconcileLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.cfg.ReconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.runReconciliation()
		}
	}
}

func (c *Cluster) runReconciliation() {
	c.nodesMu.RLock()
	nodeIDs := make([]string, 0, len(c.nodes))
	for id := range c.nodes {
		nodeIDs = append(nodeIDs, id)
	}
	c.nodesMu.RUnlock()

	if len(nodeIDs) < 2 {
		return
	}

	versionMap := make(map[string]map[string]uint64)
	for _, id := range nodeIDs {
		c.nodesMu.RLock()
		node, ok := c.nodes[id]
		c.nodesMu.RUnlock()
		if !ok {
			continue
		}
		versionMap[id] = node.getAllVersions()
	}

	mergedVersions := make(map[string]uint64)
	mergedSources := make(map[string]string)
	for nodeID, versions := range versionMap {
		for key, version := range versions {
			if existing, ok := mergedVersions[key]; !ok || version > existing {
				mergedVersions[key] = version
				mergedSources[key] = nodeID
			}
		}
	}

	for nodeID, versions := range versionMap {
		c.nodesMu.RLock()
		node, ok := c.nodes[nodeID]
		c.nodesMu.RUnlock()
		if !ok {
			continue
		}

		for key, highestVersion := range mergedVersions {
			localVersion, hasLocal := versions[key]
			if !hasLocal || localVersion < highestVersion {
				sourceNodeID := mergedSources[key]
				if sourceNodeID == nodeID {
					continue
				}
				c.nodesMu.RLock()
				sourceNode, ok := c.nodes[sourceNodeID]
				c.nodesMu.RUnlock()
				if !ok {
					continue
				}
				sourceEntry := sourceNode.Get(key)
				if sourceEntry != nil {
					msg := &Message{
						Type:       MsgUpdateNotify,
						FromNodeID: sourceNodeID,
						Key:        key,
						Value:      sourceEntry.Value,
						Version:    sourceEntry.Version,
						Timestamp:  time.Now(),
					}
					_ = node.handleUpdateNotify(msg)
				}
			}
		}
	}
}

func (n *Node) messageLoop() {
	defer n.wg.Done()

	for {
		select {
		case <-n.stopCh:
			return
		case msg, ok := <-n.inbox:
			if !ok {
				return
			}
			n.handleMessage(msg)
		}
	}
}

func (n *Node) handleMessage(msg *Message) {
	switch msg.Type {
	case MsgUpdateNotify:
		n.handleUpdateNotify(msg)
	case MsgInvalidate:
		n.handleInvalidate(msg)
	case MsgLockAcquire:
		n.handleLockAcquire(msg)
	case MsgLockRelease:
		n.handleLockRelease(msg)
	case MsgReconcileRequest:
		n.handleReconcileRequest(msg)
	case MsgLockGranted:
		n.handleLockGranted(msg)
	case MsgLockDenied:
		n.handleLockDenied(msg)
	}
}

func (n *Node) handleLockGranted(msg *Message) {
	//
	// 注意：本函数的正确性依赖于 pendingLock 状态机的单向转换不变式：
	// Pending -> Succeeded 或 Pending -> Failed，状态一旦变更不可回退。
	// 因此在写入 grantedBy 前后两次读取 status 即可覆盖所有竞态场景，
	// 不存在"第一次读是 Pending，写入后变成 Failed，再变回 Pending"的可能。
	// 该不变式由 assertStatusTransitionValid 在状态转换处强制执行。
	//
	n.pendingLockMu.Lock()
	pl, ok := n.pendingLocks[msg.Key]
	n.pendingLockMu.Unlock()
	if !ok {
		n.lockMu.Lock()
		li, hasLock := n.locks[msg.Key]
		isHolder := hasLock && li.holder == n.ID && time.Now().Before(li.expiresAt)
		n.lockMu.Unlock()
		if !isHolder {
			releaseMsg := &Message{
				Type:      MsgLockRelease,
				FromNodeID: n.ID,
				Key:       msg.Key,
				Timestamp: time.Now(),
			}
			_ = n.cluster.reliableUnicast(n.ID, msg.FromNodeID, releaseMsg)
			atomic.AddUint64(&n.msgSent, 1)
		}
		return
	}

	pl.statusMu.RLock()
	status := pl.status
	pl.statusMu.RUnlock()

	if status == pendingLockStatusSucceeded {
		return
	}

	if status == pendingLockStatusFailed {
		releaseMsg := &Message{
			Type:      MsgLockRelease,
			FromNodeID: n.ID,
			Key:       msg.Key,
			Timestamp: time.Now(),
		}
		_ = n.cluster.reliableUnicast(n.ID, msg.FromNodeID, releaseMsg)
		atomic.AddUint64(&n.msgSent, 1)
		return
	}

	pl.grantedMu.Lock()
	pl.grantedBy[msg.FromNodeID] = struct{}{}
	pl.grantedMu.Unlock()

	pl.statusMu.RLock()
	status = pl.status
	pl.statusMu.RUnlock()

	if status == pendingLockStatusFailed {
		releaseMsg := &Message{
			Type:      MsgLockRelease,
			FromNodeID: n.ID,
			Key:       msg.Key,
			Timestamp: time.Now(),
		}
		_ = n.cluster.reliableUnicast(n.ID, msg.FromNodeID, releaseMsg)
		atomic.AddUint64(&n.msgSent, 1)
		return
	}

	select {
	case pl.grantCh <- true:
	default:
	}
}

func (n *Node) handleLockDenied(msg *Message) {
	n.pendingLockMu.Lock()
	pl, ok := n.pendingLocks[msg.Key]
	n.pendingLockMu.Unlock()
	if ok {
		select {
		case pl.denyCh <- msg.LockHolder:
		default:
		}
	}
}

func (n *Node) handleUpdateNotify(msg *Message) error {
	n.cacheMu.Lock()
	existing, ok := n.cache[msg.Key]
	if ok && msg.Version <= existing.Version {
		localVersion := existing.Version
		msgVersion := msg.Version
		fromNodeID := msg.FromNodeID
		key := msg.Key
		n.cacheMu.Unlock()

		event := VersionRejectEvent{
			Key:          key,
			LocalVersion: localVersion,
			MsgVersion:   msgVersion,
			FromNodeID:   fromNodeID,
			RejectedAt:   time.Now(),
		}
		atomic.AddUint64(&n.rejectCount, 1)

		n.rejectHandlerMu.RLock()
		handlers := make([]VersionRejectHandler, len(n.rejectHandlers))
		copy(handlers, n.rejectHandlers)
		n.rejectHandlerMu.RUnlock()

		for _, h := range handlers {
			func() {
				defer func() { recover() }()
				h(event)
			}()
		}

		return fmt.Errorf("%w: key=%s local=%d msg=%d from=%s",
			ErrVersionTooOld, key, localVersion, msgVersion, fromNodeID)
	}

	n.cache[msg.Key] = &CacheEntry{
		Key:       msg.Key,
		Value:     msg.Value,
		Version:   msg.Version,
		UpdatedAt: msg.Timestamp,
		NodeID:    msg.FromNodeID,
	}
	n.cacheMu.Unlock()
	return nil
}

func (n *Node) handleInvalidate(msg *Message) {
	n.cacheMu.Lock()
	defer n.cacheMu.Unlock()
	delete(n.cache, msg.Key)
}

func (n *Node) handleLockAcquire(msg *Message) {
	n.lockMu.Lock()
	defer n.lockMu.Unlock()

	n.cleanupExpiredReleaseMarks()

	if marks, ok := n.releaseMarks[msg.Key]; ok {
		if mark, ok := marks[msg.FromNodeID]; ok {
			if !msg.Timestamp.After(mark.timestamp) {
				return
			}
			delete(marks, msg.FromNodeID)
			if len(marks) == 0 {
				delete(n.releaseMarks, msg.Key)
			}
		}
	}

	li, ok := n.locks[msg.Key]
	if ok && time.Now().Before(li.expiresAt) {
		denyMsg := &Message{
			Type:       MsgLockDenied,
			FromNodeID: n.ID,
			Key:        msg.Key,
			LockHolder: li.holder,
			Timestamp:  time.Now(),
		}
		_ = n.cluster.reliableUnicast(n.ID, msg.FromNodeID, denyMsg)
		return
	}

	n.locks[msg.Key] = &lockInfo{
		holder:     msg.FromNodeID,
		expiresAt:  time.Now().Add(msg.LockTTL),
		acquiredAt: time.Now(),
	}

	grantMsg := &Message{
		Type:       MsgLockGranted,
		FromNodeID: n.ID,
		Key:        msg.Key,
		Timestamp:  time.Now(),
	}
	_ = n.cluster.reliableUnicast(n.ID, msg.FromNodeID, grantMsg)
}

func (n *Node) cleanupExpiredReleaseMarks() {
	now := time.Now()
	for key, marks := range n.releaseMarks {
		for holder, mark := range marks {
			if !now.Before(mark.expiresAt) {
				delete(marks, holder)
			}
		}
		if len(marks) == 0 {
			delete(n.releaseMarks, key)
		}
	}
}

func (n *Node) handleLockRelease(msg *Message) {
	n.lockMu.Lock()
	defer n.lockMu.Unlock()

	li, ok := n.locks[msg.Key]
	if ok && li.holder == msg.FromNodeID {
		delete(n.locks, msg.Key)
	}

	if _, ok := n.releaseMarks[msg.Key]; !ok {
		n.releaseMarks[msg.Key] = make(map[string]*releaseMark)
	}
	existing, ok := n.releaseMarks[msg.Key][msg.FromNodeID]
	if !ok || !msg.Timestamp.Before(existing.timestamp) {
		n.releaseMarks[msg.Key][msg.FromNodeID] = &releaseMark{
			timestamp: msg.Timestamp,
			expiresAt: msg.Timestamp.Add(releaseMarkTTL),
		}
	}
}

func (n *Node) handleReconcileRequest(msg *Message) {
	versions := n.getAllVersions()
	resp := &Message{
		Type:      MsgReconcileResponse,
		FromNodeID: n.ID,
		Entries:   versions,
		Timestamp: time.Now(),
	}
	_ = n.cluster.unicast(n.ID, msg.FromNodeID, resp)
}

func (n *Node) getAllVersions() map[string]uint64 {
	n.cacheMu.RLock()
	defer n.cacheMu.RUnlock()

	versions := make(map[string]uint64, len(n.cache))
	for k, v := range n.cache {
		versions[k] = v.Version
	}
	return versions
}

func (n *Node) Get(key string) *CacheEntry {
	n.cacheMu.RLock()
	defer n.cacheMu.RUnlock()

	entry, ok := n.cache[key]
	if !ok {
		return nil
	}
	entryCopy := *entry
	return &entryCopy
}

func (n *Node) GetAll() []*CacheEntry {
	n.cacheMu.RLock()
	defer n.cacheMu.RUnlock()

	entries := make([]*CacheEntry, 0, len(n.cache))
	for _, v := range n.cache {
		entryCopy := *v
		entries = append(entries, &entryCopy)
	}
	return entries
}

func (n *Node) Set(key string, value interface{}) *CacheEntry {
	if !n.running {
		return nil
	}

	n.cacheMu.Lock()
	newVersion := uint64(1)
	if existing, ok := n.cache[key]; ok {
		newVersion = existing.Version + 1
	}

	entry := &CacheEntry{
		Key:       key,
		Value:     value,
		Version:   newVersion,
		UpdatedAt: time.Now(),
		NodeID:    n.ID,
	}
	n.cache[key] = entry
	n.cacheMu.Unlock()

	notifyMsg := &Message{
		Type:      MsgUpdateNotify,
		FromNodeID: n.ID,
		Key:       key,
		Value:     value,
		Version:   newVersion,
		Timestamp: entry.UpdatedAt,
	}
	n.cluster.broadcast(n.ID, notifyMsg)
	atomic.AddUint64(&n.msgSent, 1)

	return entry
}

func (n *Node) SetWithInvalidate(key string, value interface{}) *CacheEntry {
	if !n.running {
		return nil
	}

	n.cacheMu.Lock()
	newVersion := uint64(1)
	if existing, ok := n.cache[key]; ok {
		newVersion = existing.Version + 1
	}

	entry := &CacheEntry{
		Key:       key,
		Value:     value,
		Version:   newVersion,
		UpdatedAt: time.Now(),
		NodeID:    n.ID,
	}
	n.cache[key] = entry
	n.cacheMu.Unlock()

	invalidMsg := &Message{
		Type:      MsgInvalidate,
		FromNodeID: n.ID,
		Key:       key,
		Timestamp: entry.UpdatedAt,
	}
	n.cluster.broadcast(n.ID, invalidMsg)
	atomic.AddUint64(&n.msgSent, 1)

	return entry
}

func (n *Node) Delete(key string) bool {
	n.cacheMu.Lock()
	_, existed := n.cache[key]
	if existed {
		delete(n.cache, key)
	}
	n.cacheMu.Unlock()

	if existed {
		invalidMsg := &Message{
			Type:      MsgInvalidate,
			FromNodeID: n.ID,
			Key:       key,
			Timestamp: time.Now(),
		}
		n.cluster.broadcast(n.ID, invalidMsg)
		atomic.AddUint64(&n.msgSent, 1)
	}

	return existed
}

func (c *Cluster) reliableBroadcast(fromNodeID string, msg *Message) {
	c.nodesMu.RLock()
	nodes := make([]*Node, 0, len(c.nodes))
	for id, node := range c.nodes {
		if id == fromNodeID {
			continue
		}
		if c.shouldDropMessage() {
			continue
		}
		nodes = append(nodes, node)
	}
	c.nodesMu.RUnlock()

	for _, node := range nodes {
		msgCopy := *msg
		msgCopy.ToNodeID = node.ID
		func(n *Node, m Message) {
			defer func() { recover() }()
			retries := 5
			retryDelay := 20 * time.Millisecond
			for i := 0; i < retries; i++ {
				select {
				case n.inbox <- &m:
					atomic.AddUint64(&n.msgRecv, 1)
					return
				case <-time.After(100 * time.Millisecond):
					if i < retries-1 {
						time.Sleep(retryDelay)
						continue
					}
				}
			}
		}(node, msgCopy)
	}
}

func (n *Node) rollbackLock(key string, pl *pendingLock) {
	releaseMsg := &Message{
		Type:      MsgLockRelease,
		FromNodeID: n.ID,
		Key:       key,
		Timestamp: time.Now(),
	}

	pl.grantedMu.Lock()
	grantedNodes := make([]string, 0, len(pl.grantedBy))
	for id := range pl.grantedBy {
		grantedNodes = append(grantedNodes, id)
	}
	pl.grantedMu.Unlock()

	for _, nodeID := range grantedNodes {
		_ = n.cluster.reliableUnicast(n.ID, nodeID, releaseMsg)
		atomic.AddUint64(&n.msgSent, 1)
	}

	n.cluster.reliableBroadcast(n.ID, releaseMsg)
	atomic.AddUint64(&n.msgSent, 1)
}

func (n *Node) startRollbackWatcher(key string, pl *pendingLock, watchWindow time.Duration) {
	pl.statusMu.Lock()
	assertStatusTransitionValid(pl.status, pendingLockStatusFailed)
	pl.status = pendingLockStatusFailed
	pl.statusMu.Unlock()

	n.rollbackLock(key, pl)

	time.Sleep(watchWindow)

	n.pendingLockMu.Lock()
	cur, ok := n.pendingLocks[key]
	if ok && cur == pl {
		delete(n.pendingLocks, key)
	}
	n.pendingLockMu.Unlock()
}

func (n *Node) Lock(key string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = n.cluster.cfg.LockTimeout
	}
	n.cluster.nodesMu.RLock()
	peerCount := len(n.cluster.nodes) - 1
	n.cluster.nodesMu.RUnlock()

	n.lockMu.Lock()
	li, ok := n.locks[key]
	if ok && time.Now().Before(li.expiresAt) {
		if li.holder == n.ID {
			n.lockMu.Unlock()
			return n.ID, nil
		}
		holder := li.holder
		n.lockMu.Unlock()
		return "", fmt.Errorf("%w: held by %s", ErrLockTimeout, holder)
	}
	n.lockMu.Unlock()

	pl := &pendingLock{
		grantCh:   make(chan bool, peerCount+1),
		denyCh:    make(chan string, peerCount+1),
		grantedBy: make(map[string]struct{}),
	}

	n.pendingLockMu.Lock()
	n.pendingLocks[key] = pl
	n.pendingLockMu.Unlock()

	acquireMsg := &Message{
		Type:       MsgLockAcquire,
		FromNodeID: n.ID,
		Key:        key,
		LockTTL:    timeout,
		Timestamp:  time.Now(),
	}

	n.cluster.broadcast(n.ID, acquireMsg)
	atomic.AddUint64(&n.msgSent, 1)

	grantCount := 0
	deniedHolder := ""
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	requiredGrants := peerCount
	if requiredGrants <= 0 {
		n.pendingLockMu.Lock()
		cur, ok := n.pendingLocks[key]
		if ok && cur == pl {
			delete(n.pendingLocks, key)
		}
		n.pendingLockMu.Unlock()

		n.lockMu.Lock()
		n.locks[key] = &lockInfo{
			holder:     n.ID,
			expiresAt:  time.Now().Add(timeout),
			acquiredAt: time.Now(),
		}
		n.lockMu.Unlock()
		return n.ID, nil
	}

	rollbackWindow := 1 * time.Second
	if timeout > rollbackWindow {
		rollbackWindow = timeout
	}

	for {
		select {
		case <-pl.grantCh:
			grantCount++
			if grantCount >= requiredGrants {
				pl.statusMu.Lock()
				assertStatusTransitionValid(pl.status, pendingLockStatusSucceeded)
				pl.status = pendingLockStatusSucceeded
				pl.statusMu.Unlock()

				n.pendingLockMu.Lock()
				cur, ok := n.pendingLocks[key]
				if ok && cur == pl {
					delete(n.pendingLocks, key)
				}
				n.pendingLockMu.Unlock()

				n.lockMu.Lock()
				n.locks[key] = &lockInfo{
					holder:     n.ID,
					expiresAt:  time.Now().Add(timeout),
					acquiredAt: time.Now(),
				}
				n.lockMu.Unlock()
				return n.ID, nil
			}
		case holder := <-pl.denyCh:
			deniedHolder = holder
			go n.startRollbackWatcher(key, pl, rollbackWindow)
			return "", fmt.Errorf("%w: held by %s", ErrLockTimeout, holder)
		case <-timer.C:
			go n.startRollbackWatcher(key, pl, rollbackWindow)
			if deniedHolder != "" {
				return "", fmt.Errorf("%w: timed out waiting for lock held by %s", ErrLockTimeout, deniedHolder)
			}
			return "", fmt.Errorf("%w: timed out waiting for lock consensus", ErrLockTimeout)
		}
	}
}

func (n *Node) Unlock(key string) error {
	n.lockMu.Lock()
	li, ok := n.locks[key]
	if !ok {
		n.lockMu.Unlock()
		return ErrLockNotHeld
	}
	if li.holder != n.ID {
		holder := li.holder
		n.lockMu.Unlock()
		return fmt.Errorf("%w: lock held by %s", ErrLockNotHeld, holder)
	}
	delete(n.locks, key)
	n.lockMu.Unlock()

	releaseMsg := &Message{
		Type:      MsgLockRelease,
		FromNodeID: n.ID,
		Key:       key,
		Timestamp: time.Now(),
	}
	n.cluster.broadcast(n.ID, releaseMsg)
	atomic.AddUint64(&n.msgSent, 1)

	return nil
}

func (n *Node) GetLockHolder(key string) string {
	n.lockMu.Lock()
	defer n.lockMu.Unlock()

	li, ok := n.locks[key]
	if !ok {
		return ""
	}
	if !time.Now().Before(li.expiresAt) {
		return ""
	}
	return li.holder
}

func (n *Node) IsLocked(key string) bool {
	return n.GetLockHolder(key) != ""
}

func (n *Node) Reconcile() {
	n.cluster.runReconciliation()
}

func (n *Node) Stats() (sent, recv, rejectCount uint64, cacheSize int, lockCount int) {
	sent = atomic.LoadUint64(&n.msgSent)
	recv = atomic.LoadUint64(&n.msgRecv)
	rejectCount = atomic.LoadUint64(&n.rejectCount)
	n.cacheMu.RLock()
	cacheSize = len(n.cache)
	n.cacheMu.RUnlock()
	n.lockMu.Lock()
	lockCount = len(n.locks)
	n.lockMu.Unlock()
	return
}

func (n *Node) AddVersionRejectHandler(handler VersionRejectHandler) {
	if handler == nil {
		return
	}
	n.rejectHandlerMu.Lock()
	n.rejectHandlers = append(n.rejectHandlers, handler)
	n.rejectHandlerMu.Unlock()
}

func (n *Node) VersionRejectCount() uint64 {
	return atomic.LoadUint64(&n.rejectCount)
}
