package raftlog

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrNodeStopped          = errors.New("raftlog: node is stopped")
	ErrNotLeader            = errors.New("raftlog: node is not the leader")
	ErrInvalidIndex         = errors.New("raftlog: invalid log index")
	ErrLogCompacted         = errors.New("raftlog: log has been compacted")
	ErrSnapshotInstalling   = errors.New("raftlog: snapshot installation in progress")
	ErrConfigChangeInFlight = errors.New("raftlog: configuration change already in flight")
	ErrEmptyConfig          = errors.New("raftlog: configuration cannot be empty")
	ErrNodeNotFound         = errors.New("raftlog: node not found in cluster")
	ErrNodeExists           = errors.New("raftlog: node already exists in cluster")
	ErrTransportClosed      = errors.New("raftlog: transport is closed")
	ErrApplyFailed          = errors.New("raftlog: failed to apply log entry")
)

const (
	DefaultElectionTimeoutMin = 150 * time.Millisecond
	DefaultElectionTimeoutMax = 300 * time.Millisecond
	DefaultHeartbeatInterval  = 50 * time.Millisecond
	DefaultSnapshotThreshold  = 1000
	DefaultSnapshotChunkSize  = 64 * 1024
)

type NodeState int

const (
	Follower NodeState = iota
	Candidate
	Leader
)

func (s NodeState) String() string {
	switch s {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Leader:
		return "Leader"
	default:
		return "Unknown"
	}
}

type LogEntryType int

const (
	LogEntryNormal LogEntryType = iota
	LogEntryConfigJoint
	LogEntryConfigNew
)

type LogEntry struct {
	Term    int
	Index   int
	Type    LogEntryType
	Command []byte
}

type Configuration struct {
	Nodes map[string]bool
}

func NewConfiguration(nodes []string) *Configuration {
	c := &Configuration{Nodes: make(map[string]bool)}
	for _, n := range nodes {
		c.Nodes[n] = true
	}
	return c
}

func (c *Configuration) Clone() *Configuration {
	nc := &Configuration{Nodes: make(map[string]bool)}
	for k, v := range c.Nodes {
		nc.Nodes[k] = v
	}
	return nc
}

func (c *Configuration) Contains(nodeID string) bool {
	return c.Nodes[nodeID]
}

func (c *Configuration) Size() int {
	return len(c.Nodes)
}

func (c *Configuration) Quorum() int {
	return len(c.Nodes)/2 + 1
}

func (c *Configuration) NodeIDs() []string {
	ids := make([]string, 0, len(c.Nodes))
	for id := range c.Nodes {
		ids = append(ids, id)
	}
	return ids
}

type JointConfiguration struct {
	Old *Configuration
	New *Configuration
}

func (jc *JointConfiguration) Contains(nodeID string) bool {
	return jc.Old.Contains(nodeID) || jc.New.Contains(nodeID)
}

func (jc *JointConfiguration) OldQuorum() int {
	return jc.Old.Quorum()
}

func (jc *JointConfiguration) NewQuorum() int {
	return jc.New.Quorum()
}

type Snapshot struct {
	LastIncludedIndex int
	LastIncludedTerm  int
	Config            *Configuration
	Data              []byte
}

type RequestVoteRequest struct {
	Term         int
	CandidateID  string
	LastLogIndex int
	LastLogTerm  int
	IsConfig     bool
}

type RequestVoteReply struct {
	Term        int
	VoteGranted bool
}

type AppendEntriesRequest struct {
	Term         int
	LeaderID     string
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []*LogEntry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term          int
	Success       bool
	MatchIndex    int
	ConflictTerm  int
	ConflictIndex int
}

type InstallSnapshotRequest struct {
	Term              int
	LeaderID          string
	LastIncludedIndex int
	LastIncludedTerm  int
	Config            *Configuration
	Data              []byte
	Done              bool
}

type InstallSnapshotReply struct {
	Term    int
	Success bool
}

type StateMachine interface {
	Apply(entry *LogEntry) error
	ApplySnapshot(snapshot *Snapshot) error
	Snapshot() ([]byte, error)
}

type MemoryStateMachine struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewMemoryStateMachine() *MemoryStateMachine {
	return &MemoryStateMachine{
		data: make(map[string]string),
	}
}

func (sm *MemoryStateMachine) Apply(entry *LogEntry) error {
	if entry.Type != LogEntryNormal {
		return nil
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.data[string(entry.Command)] = string(entry.Command)
	return nil
}

func (sm *MemoryStateMachine) ApplySnapshot(snapshot *Snapshot) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(snapshot.Data) == 0 {
		sm.data = make(map[string]string)
		return nil
	}

	decoder := gob.NewDecoder(bytes.NewReader(snapshot.Data))
	var data map[string]string
	if err := decoder.Decode(&data); err != nil {
		return err
	}
	sm.data = data
	return nil
}

func (sm *MemoryStateMachine) Snapshot() ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(sm.data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (sm *MemoryStateMachine) Get(key string) (string, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	v, ok := sm.data[key]
	return v, ok
}

func (sm *MemoryStateMachine) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.data)
}

type RaftConfig struct {
	ElectionTimeoutMin time.Duration
	ElectionTimeoutMax time.Duration
	HeartbeatInterval  time.Duration
	SnapshotThreshold  int
	SnapshotChunkSize  int
}

func DefaultRaftConfig() RaftConfig {
	return RaftConfig{
		ElectionTimeoutMin: DefaultElectionTimeoutMin,
		ElectionTimeoutMax: DefaultElectionTimeoutMax,
		HeartbeatInterval:  DefaultHeartbeatInterval,
		SnapshotThreshold:  DefaultSnapshotThreshold,
		SnapshotChunkSize:  DefaultSnapshotChunkSize,
	}
}

type ApplyResult struct {
	Index int
	Term  int
	Err   error
}

type RaftNode struct {
	id        string
	cfg       RaftConfig
	state     NodeState
	sm        StateMachine
	transport Transport

	mu sync.Mutex

	currentTerm int
	votedFor    string
	log         []*LogEntry
	logOffset   int

	commitIndex int
	lastApplied int

	nextIndex  map[string]int
	matchIndex map[string]int

	votesReceived map[string]bool

	leaderID string

	config          *Configuration
	joint           *JointConfiguration
	configCommitted bool

	running bool
	stopped bool

	electionTimer  *time.Timer
	heartbeatTimer *time.Timer

	commitReady chan struct{}
	applyCh     chan *ApplyResult

	lastSnapshotIndex int
	lastSnapshotTerm  int

	randState int64

	configChangeInFlight bool
	snapshotInstalling   bool
}

type Transport interface {
	SendRequestVote(target string, req *RequestVoteRequest) (*RequestVoteReply, error)
	SendAppendEntries(target string, req *AppendEntriesRequest) (*AppendEntriesReply, error)
	SendInstallSnapshot(target string, req *InstallSnapshotRequest) (*InstallSnapshotReply, error)
	RegisterNode(id string, node *RaftNode)
	UnregisterNode(id string)
	Close()
}

type MemoryTransport struct {
	mu     sync.RWMutex
	nodes  map[string]*RaftNode
	delay  time.Duration
	closed bool
}

func NewMemoryTransport() *MemoryTransport {
	return &MemoryTransport{
		nodes: make(map[string]*RaftNode),
	}
}

func NewMemoryTransportWithDelay(delay time.Duration) *MemoryTransport {
	return &MemoryTransport{
		nodes: make(map[string]*RaftNode),
		delay: delay,
	}
}

func (t *MemoryTransport) RegisterNode(id string, node *RaftNode) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.nodes[id] = node
}

func (t *MemoryTransport) UnregisterNode(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.nodes, id)
}

func (t *MemoryTransport) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	t.nodes = make(map[string]*RaftNode)
}

func (t *MemoryTransport) getNode(id string) (*RaftNode, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed {
		return nil, false
	}
	n, ok := t.nodes[id]
	return n, ok
}

func (t *MemoryTransport) SendRequestVote(target string, req *RequestVoteRequest) (*RequestVoteReply, error) {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return nil, ErrTransportClosed
	}
	t.mu.RUnlock()
	node, ok := t.getNode(target)
	if !ok {
		return nil, ErrNodeNotFound
	}
	if t.delay > 0 {
		time.Sleep(t.delay)
	}
	return node.HandleRequestVote(req), nil
}

func (t *MemoryTransport) SendAppendEntries(target string, req *AppendEntriesRequest) (*AppendEntriesReply, error) {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return nil, ErrTransportClosed
	}
	t.mu.RUnlock()
	node, ok := t.getNode(target)
	if !ok {
		return nil, ErrNodeNotFound
	}
	if t.delay > 0 {
		time.Sleep(t.delay)
	}
	return node.HandleAppendEntries(req), nil
}

func (t *MemoryTransport) SendInstallSnapshot(target string, req *InstallSnapshotRequest) (*InstallSnapshotReply, error) {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return nil, ErrTransportClosed
	}
	t.mu.RUnlock()
	node, ok := t.getNode(target)
	if !ok {
		return nil, ErrNodeNotFound
	}
	if t.delay > 0 {
		time.Sleep(t.delay)
	}
	return node.HandleInstallSnapshot(req), nil
}

func NewRaftNode(id string, config RaftConfig, sm StateMachine, transport Transport, initialNodes []string) *RaftNode {
	n := &RaftNode{
		id:          id,
		cfg:         config,
		state:       Follower,
		sm:          sm,
		transport:   transport,
		currentTerm: 0,
		votedFor:    "",
		log:         make([]*LogEntry, 0),
		logOffset:   0,
		commitIndex: 0,
		lastApplied: 0,
		nextIndex:   make(map[string]int),
		matchIndex:  make(map[string]int),
		votesReceived: make(map[string]bool),
		config:      NewConfiguration(initialNodes),
		commitReady: make(chan struct{}, 1),
		applyCh:     make(chan *ApplyResult, 256),
		randState:   time.Now().UnixNano() + int64(len(id)),
	}

	if config.ElectionTimeoutMin <= 0 {
		n.cfg.ElectionTimeoutMin = DefaultElectionTimeoutMin
	}
	if config.ElectionTimeoutMax <= 0 {
		n.cfg.ElectionTimeoutMax = DefaultElectionTimeoutMax
	}
	if config.HeartbeatInterval <= 0 {
		n.cfg.HeartbeatInterval = DefaultHeartbeatInterval
	}
	if config.SnapshotThreshold <= 0 {
		n.cfg.SnapshotThreshold = DefaultSnapshotThreshold
	}
	if config.SnapshotChunkSize <= 0 {
		n.cfg.SnapshotChunkSize = DefaultSnapshotChunkSize
	}

	n.appendEntry(&LogEntry{Term: 0, Index: 0, Type: LogEntryNormal, Command: nil})

	return n
}

func (n *RaftNode) ID() string {
	return n.id
}

func (n *RaftNode) State() NodeState {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.state
}

func (n *RaftNode) CurrentTerm() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.currentTerm
}

func (n *RaftNode) LeaderID() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.leaderID
}

func (n *RaftNode) CommitIndex() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.commitIndex
}

func (n *RaftNode) LastApplied() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.lastApplied
}

func (n *RaftNode) LastLogIndex() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.lastLogIndex()
}

func (n *RaftNode) lastLogIndex() int {
	if len(n.log) == 0 {
		return n.logOffset
	}
	return n.log[len(n.log)-1].Index
}

func (n *RaftNode) LastLogTerm() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.lastLogTerm()
}

func (n *RaftNode) lastLogTerm() int {
	if len(n.log) == 0 {
		return n.lastSnapshotTerm
	}
	return n.log[len(n.log)-1].Term
}

func (n *RaftNode) getLogEntry(index int) *LogEntry {
	if index < n.logOffset {
		return nil
	}
	idx := index - n.logOffset
	if idx < 0 || idx >= len(n.log) {
		return nil
	}
	return n.log[idx]
}

func (n *RaftNode) GetLogEntry(index int) (*LogEntry, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if index < n.logOffset {
		return nil, ErrLogCompacted
	}
	entry := n.getLogEntry(index)
	if entry == nil {
		return nil, ErrInvalidIndex
	}
	return entry, nil
}

func (n *RaftNode) appendEntry(entry *LogEntry) {
	n.log = append(n.log, entry)
}

func (n *RaftNode) truncateLog(index int) {
	if index < n.logOffset {
		n.log = make([]*LogEntry, 0)
		n.logOffset = index
		return
	}
	idx := index - n.logOffset
	if idx >= 0 && idx < len(n.log) {
		n.log = n.log[:idx]
	}
}

func (n *RaftNode) Start() {
	n.mu.Lock()
	if n.running {
		n.mu.Unlock()
		return
	}
	n.running = true
	n.stopped = false
	n.mu.Unlock()

	n.transport.RegisterNode(n.id, n)

	n.electionTimer = time.NewTimer(n.randomElectionTimeout())
	n.heartbeatTimer = time.NewTimer(n.cfg.HeartbeatInterval)
	n.heartbeatTimer.Stop()

	go n.run()
	go n.applyLoop()
}

func (n *RaftNode) Stop() {
	n.mu.Lock()
	if n.stopped {
		n.mu.Unlock()
		return
	}
	n.stopped = true
	n.running = false
	n.mu.Unlock()

	if n.electionTimer != nil {
		n.electionTimer.Stop()
	}
	if n.heartbeatTimer != nil {
		n.heartbeatTimer.Stop()
	}

	n.transport.UnregisterNode(n.id)
}

func (n *RaftNode) randomElectionTimeout() time.Duration {
	r := n.nextRand()
	delta := r % (int64(n.cfg.ElectionTimeoutMax) - int64(n.cfg.ElectionTimeoutMin))
	if delta < 0 {
		delta = -delta
	}
	return n.cfg.ElectionTimeoutMin + time.Duration(delta)
}

func (n *RaftNode) nextRand() int64 {
	n.randState = (n.randState*1103515245 + 12345) & 0x7fffffff
	if n.randState < 0 {
		n.randState = -n.randState
	}
	return n.randState
}

func (n *RaftNode) run() {
	for {
		n.mu.Lock()
		running := n.running
		n.mu.Unlock()
		if !running {
			return
		}

		select {
		case <-n.electionTimer.C:
			n.handleElectionTimeout()
		case <-n.heartbeatTimer.C:
			n.handleHeartbeatTimeout()
		case <-n.commitReady:
			n.notifyApply()
		}
	}
}

func (n *RaftNode) handleElectionTimeout() {
	n.mu.Lock()
	if n.state == Leader || !n.running {
		n.mu.Unlock()
		return
	}

	n.becomeCandidate()
	n.mu.Unlock()

	n.startElection()
}

func (n *RaftNode) handleHeartbeatTimeout() {
	n.mu.Lock()
	if n.state != Leader || !n.running {
		n.mu.Unlock()
		return
	}
	n.mu.Unlock()

	n.sendHeartbeats()
}

func (n *RaftNode) becomeCandidate() {
	n.state = Candidate
	n.currentTerm++
	n.votedFor = n.id
	n.votesReceived = make(map[string]bool)
	n.votesReceived[n.id] = true
	n.leaderID = ""
	n.resetElectionTimer()
	n.checkElectionWin()
}

func (n *RaftNode) becomeFollower(term int) {
	n.state = Follower
	n.currentTerm = term
	n.votedFor = ""
	n.leaderID = ""
	n.resetElectionTimer()
}

func (n *RaftNode) becomeLeader() {
	n.state = Leader
	n.votedFor = ""
	n.leaderID = n.id

	lastIdx := n.lastLogIndex()
	allNodes := n.currentNodes()
	for _, nodeID := range allNodes {
		if nodeID == n.id {
			continue
		}
		n.nextIndex[nodeID] = lastIdx + 1
		n.matchIndex[nodeID] = 0
	}

	n.electionTimer.Stop()
	n.heartbeatTimer.Reset(n.cfg.HeartbeatInterval)
}

func (n *RaftNode) currentNodes() []string {
	if n.joint != nil {
		result := make(map[string]bool)
		for id := range n.joint.Old.Nodes {
			result[id] = true
		}
		for id := range n.joint.New.Nodes {
			result[id] = true
		}
		ids := make([]string, 0, len(result))
		for id := range result {
			ids = append(ids, id)
		}
		return ids
	}
	return n.config.NodeIDs()
}

func (n *RaftNode) resetElectionTimer() {
	if n.electionTimer != nil {
		n.electionTimer.Stop()
	}
	n.electionTimer = time.NewTimer(n.randomElectionTimeout())
}

func (n *RaftNode) startElection() {
	n.mu.Lock()
	term := n.currentTerm
	lastLogIdx := n.lastLogIndex()
	lastLogTerm := n.lastLogTerm()
	nodes := n.currentNodes()
	n.mu.Unlock()

	for _, nodeID := range nodes {
		if nodeID == n.id {
			continue
		}
		go func(target string) {
			req := &RequestVoteRequest{
				Term:         term,
				CandidateID:  n.id,
				LastLogIndex: lastLogIdx,
				LastLogTerm:  lastLogTerm,
			}
			reply, err := n.transport.SendRequestVote(target, req)
			if err != nil {
				return
			}
			n.handleRequestVoteReply(term, target, reply)
		}(nodeID)
	}
}

func (n *RaftNode) HandleRequestVote(req *RequestVoteRequest) *RequestVoteReply {
	n.mu.Lock()
	defer n.mu.Unlock()

	reply := &RequestVoteReply{
		Term:        n.currentTerm,
		VoteGranted: false,
	}

	if req.Term > n.currentTerm {
		n.becomeFollower(req.Term)
	}

	if req.Term < n.currentTerm {
		reply.Term = n.currentTerm
		return reply
	}

	canVote := n.votedFor == "" || n.votedFor == req.CandidateID

	logOk := n.isLogUpToDate(req.LastLogIndex, req.LastLogTerm)

	if canVote && logOk {
		n.votedFor = req.CandidateID
		reply.VoteGranted = true
		n.resetElectionTimer()
	}

	reply.Term = n.currentTerm
	return reply
}

func (n *RaftNode) isLogUpToDate(otherIndex, otherTerm int) bool {
	myLastTerm := n.lastLogTerm()
	myLastIdx := n.lastLogIndex()

	if otherTerm > myLastTerm {
		return true
	}
	if otherTerm == myLastTerm && otherIndex >= myLastIdx {
		return true
	}
	return false
}

func (n *RaftNode) handleRequestVoteReply(originalTerm int, voterID string, reply *RequestVoteReply) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.state != Candidate {
		return
	}

	if reply.Term > n.currentTerm {
		n.becomeFollower(reply.Term)
		return
	}

	if reply.Term < originalTerm {
		return
	}

	if reply.VoteGranted {
		n.votesReceived[voterID] = true
	}

	n.checkElectionWin()
}

func (n *RaftNode) checkElectionWin() {
	if n.state != Candidate {
		return
	}

	if n.joint != nil {
		oldVotes := 0
		newVotes := 0
		for voterID := range n.votesReceived {
			if n.joint.Old.Contains(voterID) {
				oldVotes++
			}
			if n.joint.New.Contains(voterID) {
				newVotes++
			}
		}
		if oldVotes >= n.joint.OldQuorum() && newVotes >= n.joint.NewQuorum() {
			n.becomeLeader()
			go n.sendHeartbeats()
		}
		return
	}

	voteCount := len(n.votesReceived)
	if voteCount >= n.config.Quorum() {
		n.becomeLeader()
		go n.sendHeartbeats()
	}
}

func (n *RaftNode) sendHeartbeats() {
	n.mu.Lock()
	state := n.state
	term := n.currentTerm
	leaderID := n.id
	commitIndex := n.commitIndex
	nodes := n.currentNodes()
	n.mu.Unlock()

	if state != Leader {
		return
	}

	for _, nodeID := range nodes {
		if nodeID == n.id {
			continue
		}
		go func(target string) {
			n.mu.Lock()
			nextIdx := n.nextIndex[target]
			if nextIdx == 0 {
				nextIdx = n.lastLogIndex() + 1
			}

			if nextIdx <= n.logOffset {
				lastIdx := n.lastSnapshotIndex
				lastTerm := n.lastSnapshotTerm
				cfg := n.config.Clone()
				curTerm := n.currentTerm
				n.mu.Unlock()

				data, err := n.sm.Snapshot()
				if err != nil {
					return
				}
				req := &InstallSnapshotRequest{
					Term:              curTerm,
					LeaderID:          leaderID,
					LastIncludedIndex: lastIdx,
					LastIncludedTerm:  lastTerm,
					Config:            cfg,
					Data:              data,
					Done:              true,
				}
				reply, err := n.transport.SendInstallSnapshot(target, req)
				if err != nil {
					return
				}
				n.handleInstallSnapshotReply(target, reply)
				return
			}

			prevLogIndex := nextIdx - 1
			prevLogTerm := 0
			if prevLogIndex >= n.logOffset {
				entry := n.getLogEntry(prevLogIndex)
				if entry != nil {
					prevLogTerm = entry.Term
				}
			} else if prevLogIndex == n.logOffset-1 && prevLogIndex == n.lastSnapshotIndex {
				prevLogTerm = n.lastSnapshotTerm
			}

			var entries []*LogEntry
			if nextIdx <= n.lastLogIndex() {
				start := nextIdx - n.logOffset
				if start >= 0 && start < len(n.log) {
					entries = make([]*LogEntry, len(n.log)-start)
					copy(entries, n.log[start:])
				}
			}

			req := &AppendEntriesRequest{
				Term:         term,
				LeaderID:     leaderID,
				PrevLogIndex: prevLogIndex,
				PrevLogTerm:  prevLogTerm,
				Entries:      entries,
				LeaderCommit: commitIndex,
			}
			n.mu.Unlock()

			reply, err := n.transport.SendAppendEntries(target, req)
			if err != nil {
				return
			}
			n.handleAppendEntriesReply(target, reply)
		}(nodeID)
	}

	n.mu.Lock()
	if n.state == Leader && n.running {
		n.heartbeatTimer.Reset(n.cfg.HeartbeatInterval)
	}
	n.mu.Unlock()
}

func (n *RaftNode) HandleAppendEntries(req *AppendEntriesRequest) *AppendEntriesReply {
	n.mu.Lock()
	defer n.mu.Unlock()

	reply := &AppendEntriesReply{
		Term:       n.currentTerm,
		Success:    false,
		MatchIndex: 0,
	}

	if req.Term > n.currentTerm {
		n.becomeFollower(req.Term)
	}

	if req.Term < n.currentTerm {
		reply.Term = n.currentTerm
		return reply
	}

	if n.state == Candidate {
		n.becomeFollower(req.Term)
	}

	n.resetElectionTimer()
	n.leaderID = req.LeaderID

	if req.PrevLogIndex == 0 {
	} else if req.PrevLogIndex < n.logOffset {
		if req.PrevLogIndex == n.lastSnapshotIndex && n.lastSnapshotTerm == req.PrevLogTerm {
		} else {
			reply.ConflictIndex = n.logOffset + 1
			reply.ConflictTerm = n.lastSnapshotTerm
			return reply
		}
	} else {
		if req.PrevLogIndex > n.lastLogIndex() {
			reply.ConflictIndex = n.lastLogIndex() + 1
			reply.ConflictTerm = -1
			return reply
		}

		prevEntry := n.getLogEntry(req.PrevLogIndex)
		if prevEntry == nil || prevEntry.Term != req.PrevLogTerm {
			if prevEntry != nil {
				reply.ConflictTerm = prevEntry.Term
			}
			for i := req.PrevLogIndex - 1; i >= n.logOffset; i-- {
				e := n.getLogEntry(i)
				if e != nil && (reply.ConflictTerm == 0 || e.Term != reply.ConflictTerm) {
					reply.ConflictIndex = i + 1
					break
				}
			}
			if reply.ConflictIndex == 0 {
				reply.ConflictIndex = n.logOffset + 1
			}
			return reply
		}
	}

	if len(req.Entries) > 0 {
		insertIdx := req.PrevLogIndex + 1
		for _, entry := range req.Entries {
			if insertIdx <= n.lastLogIndex() {
				existing := n.getLogEntry(insertIdx)
				if existing != nil && existing.Term != entry.Term {
					n.truncateLog(insertIdx)
					n.appendEntry(entry)
				}
			} else {
				n.appendEntry(entry)
			}
			insertIdx++
		}
		reply.MatchIndex = req.PrevLogIndex + len(req.Entries)
	} else {
		reply.MatchIndex = req.PrevLogIndex
	}

	reply.Success = true

	if req.LeaderCommit > n.commitIndex {
		lastNewEntry := req.PrevLogIndex + len(req.Entries)
		if req.LeaderCommit < lastNewEntry {
			n.commitIndex = req.LeaderCommit
		} else {
			n.commitIndex = lastNewEntry
		}
		n.notifyApply()
	}

	reply.Term = n.currentTerm
	return reply
}

func (n *RaftNode) handleAppendEntriesReply(target string, reply *AppendEntriesReply) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.state != Leader {
		return
	}

	if reply.Term > n.currentTerm {
		n.becomeFollower(reply.Term)
		return
	}

	if reply.Term < n.currentTerm {
		return
	}

	if reply.Success {
		n.matchIndex[target] = reply.MatchIndex
		n.nextIndex[target] = reply.MatchIndex + 1
		n.updateCommitIndex()
		n.checkConfigProgress()
	} else {
		if reply.ConflictTerm < 0 {
			n.nextIndex[target] = reply.ConflictIndex
		} else {
			found := false
			for i := n.lastLogIndex(); i >= n.logOffset; i-- {
				e := n.getLogEntry(i)
				if e != nil && e.Term == reply.ConflictTerm {
					n.nextIndex[target] = i + 1
					found = true
					break
				}
			}
			if !found {
				n.nextIndex[target] = reply.ConflictIndex
			}
		}
		if n.nextIndex[target] < 1 {
			n.nextIndex[target] = 1
		}
	}
}

func (n *RaftNode) updateCommitIndex() {
	if n.state != Leader {
		return
	}

	if n.joint != nil {
		for i := n.commitIndex + 1; i <= n.lastLogIndex(); i++ {
			entry := n.getLogEntry(i)
			if entry == nil {
				continue
			}
			if entry.Term != n.currentTerm {
				continue
			}
			if entry.Type == LogEntryConfigJoint || entry.Type == LogEntryConfigNew {
				continue
			}

			oldMatch := 1
			newMatch := 1
			for nodeID := range n.joint.Old.Nodes {
				if nodeID == n.id {
					continue
				}
				if n.matchIndex[nodeID] >= i {
					oldMatch++
				}
			}
			for nodeID := range n.joint.New.Nodes {
				if nodeID == n.id {
					continue
				}
				if n.matchIndex[nodeID] >= i {
					newMatch++
				}
			}

			if oldMatch >= n.joint.OldQuorum() && newMatch >= n.joint.NewQuorum() {
				n.commitIndex = i
				n.notifyApply()
			}
		}
		return
	}

	for i := n.commitIndex + 1; i <= n.lastLogIndex(); i++ {
		entry := n.getLogEntry(i)
		if entry == nil {
			continue
		}
		if entry.Term != n.currentTerm {
			continue
		}

		matchCount := 1
		for nodeID := range n.config.Nodes {
			if nodeID == n.id {
				continue
			}
			if n.matchIndex[nodeID] >= i {
				matchCount++
			}
		}

		if matchCount >= n.config.Quorum() {
			n.commitIndex = i
			n.notifyApply()
		}
	}
}

func (n *RaftNode) checkConfigProgress() {
	if n.joint == nil || n.state != Leader {
		return
	}

	jointIndex := -1
	for i := n.commitIndex + 1; i <= n.lastLogIndex(); i++ {
		entry := n.getLogEntry(i)
		if entry != nil && entry.Type == LogEntryConfigJoint {
			jointIndex = i
			break
		}
	}

	if jointIndex > 0 {
		oldMatch := 1
		newMatch := 1
		for nodeID := range n.joint.Old.Nodes {
			if nodeID == n.id {
				continue
			}
			if n.matchIndex[nodeID] >= jointIndex {
				oldMatch++
			}
		}
		for nodeID := range n.joint.New.Nodes {
			if nodeID == n.id {
				continue
			}
			if n.matchIndex[nodeID] >= jointIndex {
				newMatch++
			}
		}

		if oldMatch >= n.joint.OldQuorum() && newMatch >= n.joint.NewQuorum() {
			if jointIndex > n.commitIndex {
				n.commitIndex = jointIndex
				n.notifyApply()
			}

			newConfigEntry := &LogEntry{
				Term:    n.currentTerm,
				Index:   n.lastLogIndex() + 1,
				Type:    LogEntryConfigNew,
				Command: nil,
			}
			n.appendEntry(newConfigEntry)

			go n.sendHeartbeats()
		}
	}

	newConfigIndex := -1
	for i := n.lastLogIndex(); i > n.commitIndex; i-- {
		entry := n.getLogEntry(i)
		if entry != nil && entry.Type == LogEntryConfigNew {
			newConfigIndex = i
			break
		}
	}

	if newConfigIndex > 0 {
		oldMatch := 1
		newMatch := 1
		for nodeID := range n.joint.Old.Nodes {
			if nodeID == n.id {
				continue
			}
			if n.matchIndex[nodeID] >= newConfigIndex {
				oldMatch++
			}
		}
		for nodeID := range n.joint.New.Nodes {
			if nodeID == n.id {
				continue
			}
			if n.matchIndex[nodeID] >= newConfigIndex {
				newMatch++
			}
		}

		if oldMatch >= n.joint.OldQuorum() && newMatch >= n.joint.NewQuorum() {
			if newConfigIndex > n.commitIndex {
				n.commitIndex = newConfigIndex
				n.notifyApply()
			}
			n.config = n.joint.New.Clone()
			n.joint = nil
			n.configCommitted = true
			n.configChangeInFlight = false
		}
	}
}

func (n *RaftNode) notifyApply() {
	select {
	case n.commitReady <- struct{}{}:
	default:
	}
}

func (n *RaftNode) applyCommitted() {
	for n.lastApplied < n.commitIndex {
		n.lastApplied++
		entry := n.getLogEntry(n.lastApplied)
		if entry == nil {
			continue
		}

		applyErr := error(nil)
		if entry.Type == LogEntryConfigJoint {
		} else if entry.Type == LogEntryConfigNew {
		} else {
			if err := n.sm.Apply(entry); err != nil {
				applyErr = fmt.Errorf("%w: %v", ErrApplyFailed, err)
			}
		}

		select {
		case n.applyCh <- &ApplyResult{
			Index: entry.Index,
			Term:  entry.Term,
			Err:   applyErr,
		}:
		default:
		}
	}
}

func (n *RaftNode) applyLoop() {
	for {
		n.mu.Lock()
		running := n.running
		n.mu.Unlock()
		if !running {
			return
		}

		n.mu.Lock()
		if n.lastApplied < n.commitIndex {
			n.applyCommitted()
		}
		n.mu.Unlock()

		time.Sleep(5 * time.Millisecond)
	}
}

func (n *RaftNode) Propose(command []byte) (int, int, error) {
	n.mu.Lock()
	if n.state != Leader {
		n.mu.Unlock()
		return 0, 0, ErrNotLeader
	}
	if !n.running {
		n.mu.Unlock()
		return 0, 0, ErrNodeStopped
	}
	if n.configChangeInFlight {
		n.mu.Unlock()
		return 0, 0, ErrConfigChangeInFlight
	}
	if n.snapshotInstalling {
		n.mu.Unlock()
		return 0, 0, ErrSnapshotInstalling
	}

	entry := &LogEntry{
		Term:    n.currentTerm,
		Index:   n.lastLogIndex() + 1,
		Type:    LogEntryNormal,
		Command: command,
	}
	n.appendEntry(entry)
	idx := entry.Index
	term := entry.Term
	n.mu.Unlock()

	go n.sendHeartbeats()

	return idx, term, nil
}

func (n *RaftNode) HandleInstallSnapshot(req *InstallSnapshotRequest) *InstallSnapshotReply {
	n.mu.Lock()
	defer n.mu.Unlock()

	reply := &InstallSnapshotReply{
		Term:    n.currentTerm,
		Success: false,
	}

	if req.Term > n.currentTerm {
		n.becomeFollower(req.Term)
	}

	if req.Term < n.currentTerm {
		reply.Term = n.currentTerm
		return reply
	}

	if req.LastIncludedIndex <= n.lastSnapshotIndex {
		reply.Success = true
		return reply
	}

	snap := &Snapshot{
		LastIncludedIndex: req.LastIncludedIndex,
		LastIncludedTerm:  req.LastIncludedTerm,
		Config:            req.Config,
		Data:              req.Data,
	}

	if err := n.sm.ApplySnapshot(snap); err != nil {
		return reply
	}

	n.lastSnapshotIndex = req.LastIncludedIndex
	n.lastSnapshotTerm = req.LastIncludedTerm
	n.logOffset = req.LastIncludedIndex
	n.log = []*LogEntry{}

	if req.Config != nil {
		n.config = req.Config.Clone()
	}

	if req.LastIncludedIndex > n.commitIndex {
		n.commitIndex = req.LastIncludedIndex
	}
	if req.LastIncludedIndex > n.lastApplied {
		n.lastApplied = req.LastIncludedIndex
	}

	reply.Success = true
	return reply
}

func (n *RaftNode) handleInstallSnapshotReply(target string, reply *InstallSnapshotReply) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.state != Leader {
		return
	}

	if reply.Term > n.currentTerm {
		n.becomeFollower(reply.Term)
		return
	}

	if reply.Term < n.currentTerm {
		return
	}

	if reply.Success {
		n.matchIndex[target] = n.lastSnapshotIndex
		n.nextIndex[target] = n.lastSnapshotIndex + 1
	}
}

func (n *RaftNode) CompactLog(index int) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if index > n.commitIndex {
		return ErrInvalidIndex
	}
	if index <= n.logOffset {
		return nil
	}

	entry := n.getLogEntry(index)
	if entry == nil {
		return ErrInvalidIndex
	}

	n.lastSnapshotIndex = index
	n.lastSnapshotTerm = entry.Term
	if index-n.logOffset < len(n.log) {
		n.log = n.log[index-n.logOffset:]
	} else {
		n.log = []*LogEntry{}
	}
	n.logOffset = index

	return nil
}

func (n *RaftNode) AddNode(nodeID string) error {
	n.mu.Lock()
	if n.state != Leader {
		n.mu.Unlock()
		return ErrNotLeader
	}
	if n.configChangeInFlight {
		n.mu.Unlock()
		return ErrConfigChangeInFlight
	}
	if n.config.Contains(nodeID) {
		n.mu.Unlock()
		return ErrNodeExists
	}

	newNodes := append(n.config.NodeIDs(), nodeID)
	newConfig := NewConfiguration(newNodes)

	n.joint = &JointConfiguration{
		Old: n.config.Clone(),
		New: newConfig,
	}

	jointEntry := &LogEntry{
		Term:    n.currentTerm,
		Index:   n.lastLogIndex() + 1,
		Type:    LogEntryConfigJoint,
		Command: nil,
	}
	n.appendEntry(jointEntry)

	n.configChangeInFlight = true
	n.configCommitted = false

	n.mu.Unlock()

	go n.sendHeartbeats()

	return nil
}

func (n *RaftNode) RemoveNode(nodeID string) error {
	n.mu.Lock()
	if n.state != Leader {
		n.mu.Unlock()
		return ErrNotLeader
	}
	if n.configChangeInFlight {
		n.mu.Unlock()
		return ErrConfigChangeInFlight
	}
	if !n.config.Contains(nodeID) {
		n.mu.Unlock()
		return ErrNodeNotFound
	}

	newNodes := make([]string, 0)
	for _, id := range n.config.NodeIDs() {
		if id != nodeID {
			newNodes = append(newNodes, id)
		}
	}
	if len(newNodes) == 0 {
		n.mu.Unlock()
		return ErrEmptyConfig
	}

	newConfig := NewConfiguration(newNodes)

	n.joint = &JointConfiguration{
		Old: n.config.Clone(),
		New: newConfig,
	}

	jointEntry := &LogEntry{
		Term:    n.currentTerm,
		Index:   n.lastLogIndex() + 1,
		Type:    LogEntryConfigJoint,
		Command: nil,
	}
	n.appendEntry(jointEntry)

	n.configChangeInFlight = true
	n.configCommitted = false

	n.mu.Unlock()

	go n.sendHeartbeats()

	return nil
}

func (n *RaftNode) Config() *Configuration {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.config.Clone()
}

func (n *RaftNode) ApplyCh() <-chan *ApplyResult {
	return n.applyCh
}

type Cluster struct {
	mu        sync.RWMutex
	nodes     map[string]*RaftNode
	transport *MemoryTransport
}

func NewCluster(nodeIDs []string, cfg RaftConfig, smFactory func() StateMachine) (*Cluster, error) {
	if len(nodeIDs) == 0 {
		return nil, ErrEmptyConfig
	}

	transport := NewMemoryTransport()
	c := &Cluster{
		nodes:     make(map[string]*RaftNode),
		transport: transport,
	}

	for _, id := range nodeIDs {
		var sm StateMachine
		if smFactory != nil {
			sm = smFactory()
		} else {
			sm = NewMemoryStateMachine()
		}
		node := NewRaftNode(id, cfg, sm, transport, nodeIDs)
		c.nodes[id] = node
	}

	return c, nil
}

func (c *Cluster) Start() {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, node := range c.nodes {
		node.Start()
	}
}

func (c *Cluster) Stop() {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, node := range c.nodes {
		node.Stop()
	}
	c.transport.Close()
}

func (c *Cluster) GetNode(id string) (*RaftNode, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	n, ok := c.nodes[id]
	return n, ok
}

func (c *Cluster) Nodes() map[string]*RaftNode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]*RaftNode)
	for k, v := range c.nodes {
		result[k] = v
	}
	return result
}

func (c *Cluster) Leader() *RaftNode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, node := range c.nodes {
		if node.State() == Leader {
			return node
		}
	}
	return nil
}

func (c *Cluster) WaitForLeader(timeout time.Duration) (*RaftNode, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if leader := c.Leader(); leader != nil {
			return leader, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil, fmt.Errorf("no leader elected within %v", timeout)
}

func (c *Cluster) AddNode(id string, sm StateMachine) error {
	c.mu.Lock()
	if _, exists := c.nodes[id]; exists {
		c.mu.Unlock()
		return ErrNodeExists
	}
	c.mu.Unlock()

	leader := c.Leader()
	if leader == nil {
		return ErrNotLeader
	}

	cfg := leader.cfg
	if sm == nil {
		sm = NewMemoryStateMachine()
	}

	node := NewRaftNode(id, cfg, sm, c.transport, c.NodeIDs())
	c.mu.Lock()
	c.nodes[id] = node
	c.mu.Unlock()

	node.Start()

	if err := leader.AddNode(id); err != nil {
		c.mu.Lock()
		delete(c.nodes, id)
		c.mu.Unlock()
		node.Stop()
		return err
	}

	return nil
}

func (c *Cluster) RemoveNode(id string) error {
	leader := c.Leader()
	if leader == nil {
		return ErrNotLeader
	}

	if err := leader.RemoveNode(id); err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond)

	c.mu.Lock()
	if node, ok := c.nodes[id]; ok {
		node.Stop()
		delete(c.nodes, id)
	}
	c.mu.Unlock()

	return nil
}

func (c *Cluster) NodeIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ids := make([]string, 0, len(c.nodes))
	for id := range c.nodes {
		ids = append(ids, id)
	}
	return ids
}

func (c *Cluster) NodeCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.nodes)
}

func init() {
	_ = atomic.Int32{}
}
