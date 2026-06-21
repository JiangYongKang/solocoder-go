package distsess

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type ChangeHandler func(notification ChangeNotification)

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

type Node struct {
	msgSent             uint64
	msgRecv             uint64
	syncedCount         uint64
	rejectCount         uint64
	ID                  string
	store               *TieredStore
	inbox               chan *Message
	cluster             *Cluster
	running             bool
	stopCh              chan struct{}
	wg                  sync.WaitGroup
	changeHandlers      []ChangeHandler
	handlerMu           sync.RWMutex
	cleanupTicker       *time.Ticker
	cleanupStopCh       chan struct{}
}

func NewCluster(cfg Config) *Cluster {
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = DefaultTTL
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = DefaultCleanupInterval
	}
	if cfg.SyncBuffer <= 0 {
		cfg.SyncBuffer = DefaultSyncBuffer
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

func (c *Cluster) AddNode(nodeID string, persistence PersistenceStore) (*Node, error) {
	if nodeID == "" {
		return nil, ErrNodeNotFound
	}

	c.nodesMu.Lock()
	defer c.nodesMu.Unlock()

	if !c.running {
		return nil, ErrClusterStopped
	}

	if _, exists := c.nodes[nodeID]; exists {
		return nil, fmt.Errorf("distsess: node %s already exists", nodeID)
	}

	nodeCfg := c.cfg
	nodeCfg.NodeID = nodeID

	store, err := NewTieredStore(nodeCfg, persistence)
	if err != nil {
		return nil, err
	}

	node := &Node{
		ID:            nodeID,
		store:         store,
		inbox:         make(chan *Message, c.cfg.SyncBuffer),
		cluster:       c,
		running:       true,
		stopCh:        make(chan struct{}),
		cleanupStopCh: make(chan struct{}),
	}

	c.nodes[nodeID] = node

	node.wg.Add(1)
	go node.messageLoop()

	if c.cfg.CleanupInterval > 0 {
		node.cleanupTicker = time.NewTicker(c.cfg.CleanupInterval)
		go node.cleanupLoop()
	}

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
	if node.cleanupTicker != nil {
		node.cleanupTicker.Stop()
		close(node.cleanupStopCh)
	}
	node.wg.Wait()
	close(node.inbox)

	return node.store.Close()
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

func (c *Cluster) unicast(fromNodeID, toNodeID string, msg *Message) error {
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

	select {
	case node.inbox <- &msgCopy:
		atomic.AddUint64(&node.msgRecv, 1)
		return nil
	default:
		return fmt.Errorf("%w: message channel full", ErrPersistenceFailed)
	}
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
		if node.cleanupTicker != nil {
			node.cleanupTicker.Stop()
			close(node.cleanupStopCh)
		}
	}

	c.wg.Wait()
	for _, node := range nodes {
		node.wg.Wait()
		close(node.inbox)
		_ = node.store.Close()
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

func (n *Node) cleanupLoop() {
	for {
		select {
		case <-n.cleanupStopCh:
			return
		case <-n.cleanupTicker.C:
			_ = n.store.CleanupExpired()
		}
	}
}

func (n *Node) handleMessage(msg *Message) {
	switch msg.Type {
	case MsgChangeNotify:
		n.handleChangeNotify(msg)
	case MsgInvalidate:
		n.handleInvalidate(msg)
	case MsgSyncRequest:
		n.handleSyncRequest(msg)
	case MsgSyncResponse:
		n.handleSyncResponse(msg)
	}
}

func (n *Node) handleChangeNotify(msg *Message) error {
	if msg.Session == nil {
		return nil
	}

	local, _ := n.store.GetWithoutRenew(msg.SessionID)
	if local != nil && msg.Session.Version <= local.Version {
		atomic.AddUint64(&n.rejectCount, 1)
		return fmt.Errorf("%w: session=%s local=%d msg=%d from=%s",
			ErrVersionTooOld, msg.SessionID, local.Version, msg.Session.Version, msg.FromNodeID)
	}

	if n.store.mergeRemoteSession(msg.Session) {
		atomic.AddUint64(&n.syncedCount, 1)
		n.notifyChangeHandlers(ChangeNotification{
			Type:       ChangeTypeUpdate,
			SessionID:  msg.SessionID,
			Version:    msg.Session.Version,
			NodeID:     msg.FromNodeID,
			Timestamp:  msg.Timestamp,
			DataDigest: msg.Digest,
			Data:       msg.Session,
		})
	}

	return nil
}

func (n *Node) handleInvalidate(msg *Message) {
	n.store.applyRemoteDelete(msg.SessionID, msg.Version)
	n.notifyChangeHandlers(ChangeNotification{
		Type:      ChangeTypeDelete,
		SessionID: msg.SessionID,
		Version:   msg.Version,
		NodeID:    msg.FromNodeID,
		Timestamp: msg.Timestamp,
	})
}

func (n *Node) handleSyncRequest(msg *Message) {
	sessions, err := n.store.GetAll()
	if err != nil {
		return
	}

	resp := &Message{
		Type:       MsgSyncResponse,
		FromNodeID: n.ID,
		ToNodeID:   msg.FromNodeID,
		Sessions:   sessions,
		Timestamp:  time.Now(),
	}

	_ = n.cluster.unicast(n.ID, msg.FromNodeID, resp)
	atomic.AddUint64(&n.msgSent, 1)
}

func (n *Node) handleSyncResponse(msg *Message) {
	for _, session := range msg.Sessions {
		if n.store.mergeRemoteSession(session) {
			atomic.AddUint64(&n.syncedCount, 1)
		}
	}
}

func (n *Node) Get(sessionID string) (*Session, error) {
	session, err := n.store.Get(sessionID)
	if err != nil {
		return nil, err
	}

	if n.cluster.cfg.EnableSync {
		notifyMsg := &Message{
			Type:       MsgChangeNotify,
			FromNodeID: n.ID,
			SessionID:  session.ID,
			Session:    session,
			Version:    session.Version,
			Timestamp:  time.Now(),
			Digest:     computeDataDigest(session.Data),
		}
		n.cluster.broadcast(n.ID, notifyMsg)
		atomic.AddUint64(&n.msgSent, 1)
	}

	return session, nil
}

func (n *Node) Set(sessionID string, data SessionData, ttl ...time.Duration) (*Session, error) {
	sessionTTL := n.cluster.cfg.DefaultTTL
	if len(ttl) > 0 && ttl[0] > 0 {
		sessionTTL = ttl[0]
	}

	session, err := n.store.SetWithTTL(sessionID, data, sessionTTL)
	if err != nil {
		return nil, err
	}

	if n.cluster.cfg.EnableSync {
		changeType := ChangeTypeCreate
		if session.Version > 1 {
			changeType = ChangeTypeUpdate
		}

		notifyMsg := &Message{
			Type:       MsgChangeNotify,
			FromNodeID: n.ID,
			SessionID:  sessionID,
			Session:    session,
			Version:    session.Version,
			Timestamp:  time.Now(),
			Digest:     computeDataDigest(session.Data),
		}
		n.cluster.broadcast(n.ID, notifyMsg)
		atomic.AddUint64(&n.msgSent, 1)

		n.notifyChangeHandlers(ChangeNotification{
			Type:       changeType,
			SessionID:  sessionID,
			Version:    session.Version,
			NodeID:     n.ID,
			Timestamp:  time.Now(),
			DataDigest: notifyMsg.Digest,
			Data:       session,
		})
	}

	return session, nil
}

func (n *Node) Delete(sessionID string) (bool, error) {
	session, _ := n.store.GetWithoutRenew(sessionID)
	existed, err := n.store.Delete(sessionID)
	if err != nil {
		return false, err
	}

	if existed && n.cluster.cfg.EnableSync {
		version := uint64(0)
		if session != nil {
			version = session.Version + 1
		}

		invalidMsg := &Message{
			Type:       MsgInvalidate,
			FromNodeID: n.ID,
			SessionID:  sessionID,
			Version:    version,
			Timestamp:  time.Now(),
		}
		n.cluster.broadcast(n.ID, invalidMsg)
		atomic.AddUint64(&n.msgSent, 1)

		n.notifyChangeHandlers(ChangeNotification{
			Type:      ChangeTypeDelete,
			SessionID: sessionID,
			Version:   version,
			NodeID:    n.ID,
			Timestamp: time.Now(),
		})
	}

	return existed, nil
}

func (n *Node) Renew(sessionID string) (*Session, error) {
	session, err := n.store.Renew(sessionID)
	if err != nil {
		return nil, err
	}

	if n.cluster.cfg.EnableSync {
		notifyMsg := &Message{
			Type:       MsgChangeNotify,
			FromNodeID: n.ID,
			SessionID:  sessionID,
			Session:    session,
			Version:    session.Version,
			Timestamp:  time.Now(),
			Digest:     computeDataDigest(session.Data),
		}
		n.cluster.broadcast(n.ID, notifyMsg)
		atomic.AddUint64(&n.msgSent, 1)

		n.notifyChangeHandlers(ChangeNotification{
			Type:       ChangeTypeRenew,
			SessionID:  sessionID,
			Version:    session.Version,
			NodeID:     n.ID,
			Timestamp:  time.Now(),
			DataDigest: notifyMsg.Digest,
			Data:       session,
		})
	}

	return session, nil
}

func (n *Node) GetWithoutRenew(sessionID string) (*Session, error) {
	return n.store.GetWithoutRenew(sessionID)
}

func (n *Node) Exists(sessionID string) bool {
	return n.store.Exists(sessionID)
}

func (n *Node) GetAll() ([]*Session, error) {
	return n.store.GetAll()
}

func (n *Node) AddChangeHandler(handler ChangeHandler) {
	if handler == nil {
		return
	}
	n.handlerMu.Lock()
	n.changeHandlers = append(n.changeHandlers, handler)
	n.handlerMu.Unlock()
}

func (n *Node) notifyChangeHandlers(notification ChangeNotification) {
	n.handlerMu.RLock()
	handlers := make([]ChangeHandler, len(n.changeHandlers))
	copy(handlers, n.changeHandlers)
	n.handlerMu.RUnlock()

	for _, h := range handlers {
		func() {
			defer func() { recover() }()
			h(notification)
		}()
	}
}

func (n *Node) SyncWith(targetNodeID string) error {
	req := &Message{
		Type:       MsgSyncRequest,
		FromNodeID: n.ID,
		Timestamp:  time.Now(),
	}

	return n.cluster.unicast(n.ID, targetNodeID, req)
}

func (n *Node) Stats() Stats {
	stats := n.store.getStats()
	stats.SyncedCount = atomic.LoadUint64(&n.syncedCount)
	return stats
}

func (n *Node) MessageStats() (sent, recv, reject uint64) {
	sent = atomic.LoadUint64(&n.msgSent)
	recv = atomic.LoadUint64(&n.msgRecv)
	reject = atomic.LoadUint64(&n.rejectCount)
	return
}

func (n *Node) CleanupExpired() int {
	return n.store.CleanupExpired()
}

func (n *Node) Clear() error {
	return n.store.Clear()
}
