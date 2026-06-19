package raftlog

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNodeStateString(t *testing.T) {
	tests := []struct {
		state    NodeState
		expected string
	}{
		{Follower, "Follower"},
		{Candidate, "Candidate"},
		{Leader, "Leader"},
		{NodeState(99), "Unknown"},
	}
	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("NodeState(%d).String() = %s, want %s", tt.state, tt.state.String(), tt.expected)
		}
	}
}

func TestConfiguration(t *testing.T) {
	nodes := []string{"n1", "n2", "n3"}
	cfg := NewConfiguration(nodes)

	if cfg.Size() != 3 {
		t.Errorf("Expected size 3, got %d", cfg.Size())
	}

	if cfg.Quorum() != 2 {
		t.Errorf("Expected quorum 2, got %d", cfg.Quorum())
	}

	if !cfg.Contains("n1") {
		t.Error("Expected n1 to be in config")
	}

	if cfg.Contains("n4") {
		t.Error("Expected n4 not to be in config")
	}

	clone := cfg.Clone()
	if clone.Size() != cfg.Size() {
		t.Error("Clone should have same size")
	}

	ids := cfg.NodeIDs()
	if len(ids) != 3 {
		t.Errorf("Expected 3 node IDs, got %d", len(ids))
	}
}

func TestNewRaftNode(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node := NewRaftNode("node1", cfg, sm, transport, []string{"node1", "node2", "node3"})

	if node.ID() != "node1" {
		t.Errorf("Expected ID node1, got %s", node.ID())
	}

	if node.State() != Follower {
		t.Errorf("Expected initial state Follower, got %s", node.State())
	}

	if node.CurrentTerm() != 0 {
		t.Errorf("Expected initial term 0, got %d", node.CurrentTerm())
	}

	if node.LastLogIndex() != 0 {
		t.Errorf("Expected last log index 0, got %d", node.LastLogIndex())
	}

	if node.CommitIndex() != 0 {
		t.Errorf("Expected commit index 0, got %d", node.CommitIndex())
	}
}

func TestDefaultRaftConfig(t *testing.T) {
	cfg := DefaultRaftConfig()

	if cfg.ElectionTimeoutMin != DefaultElectionTimeoutMin {
		t.Error("Default election timeout min mismatch")
	}
	if cfg.ElectionTimeoutMax != DefaultElectionTimeoutMax {
		t.Error("Default election timeout max mismatch")
	}
	if cfg.HeartbeatInterval != DefaultHeartbeatInterval {
		t.Error("Default heartbeat interval mismatch")
	}
	if cfg.SnapshotThreshold != DefaultSnapshotThreshold {
		t.Error("Default snapshot threshold mismatch")
	}
}

func TestNewCluster(t *testing.T) {
	nodeIDs := []string{"n1", "n2", "n3"}
	cfg := DefaultRaftConfig()

	cluster, err := NewCluster(nodeIDs, cfg, nil)
	if err != nil {
		t.Fatalf("NewCluster failed: %v", err)
	}
	defer cluster.Stop()

	if cluster.NodeCount() != 3 {
		t.Errorf("Expected 3 nodes, got %d", cluster.NodeCount())
	}

	if _, ok := cluster.GetNode("n1"); !ok {
		t.Error("Expected node n1 to exist")
	}

	if _, ok := cluster.GetNode("n4"); ok {
		t.Error("Expected node n4 not to exist")
	}
}

func TestNewClusterEmpty(t *testing.T) {
	_, err := NewCluster([]string{}, DefaultRaftConfig(), nil)
	if !errors.Is(err, ErrEmptyConfig) {
		t.Errorf("Expected ErrEmptyConfig, got %v", err)
	}
}

func TestLeaderElection_3Nodes(t *testing.T) {
	nodeIDs := []string{"n1", "n2", "n3"}
	cfg := RaftConfig{
		ElectionTimeoutMin: 50 * time.Millisecond,
		ElectionTimeoutMax: 100 * time.Millisecond,
		HeartbeatInterval:  20 * time.Millisecond,
	}

	cluster, err := NewCluster(nodeIDs, cfg, nil)
	if err != nil {
		t.Fatalf("NewCluster failed: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	leader, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	if leader == nil {
		t.Fatal("Leader is nil")
	}

	if leader.State() != Leader {
		t.Errorf("Expected leader state, got %s", leader.State())
	}

	if leader.CurrentTerm() < 1 {
		t.Errorf("Expected term >= 1, got %d", leader.CurrentTerm())
	}

	leaderID := leader.ID()

	time.Sleep(100 * time.Millisecond)

	leader2 := cluster.Leader()
	if leader2 == nil || leader2.ID() != leaderID {
		t.Error("Leader should remain stable")
	}
}

func TestLeaderElection_5Nodes(t *testing.T) {
	nodeIDs := []string{"n1", "n2", "n3", "n4", "n5"}
	cfg := RaftConfig{
		ElectionTimeoutMin: 50 * time.Millisecond,
		ElectionTimeoutMax: 100 * time.Millisecond,
		HeartbeatInterval:  20 * time.Millisecond,
	}

	cluster, err := NewCluster(nodeIDs, cfg, nil)
	if err != nil {
		t.Fatalf("NewCluster failed: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	leader, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	if leader == nil {
		t.Fatal("Leader is nil")
	}

	term := leader.CurrentTerm()
	if term < 1 {
		t.Errorf("Expected term >= 1, got %d", term)
	}
}

func TestFollowerReceivesHeartbeat(t *testing.T) {
	nodeIDs := []string{"n1", "n2", "n3"}
	cfg := RaftConfig{
		ElectionTimeoutMin: 50 * time.Millisecond,
		ElectionTimeoutMax: 100 * time.Millisecond,
		HeartbeatInterval:  20 * time.Millisecond,
	}

	cluster, err := NewCluster(nodeIDs, cfg, nil)
	if err != nil {
		t.Fatalf("NewCluster failed: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	leader, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	leaderID := leader.ID()

	for id, node := range cluster.Nodes() {
		if id == leaderID {
			continue
		}
		if node.LeaderID() != leaderID {
			t.Errorf("Follower %s should have leader %s, but has %s", id, leaderID, node.LeaderID())
		}
	}
}

func TestProposeLogEntry(t *testing.T) {
	nodeIDs := []string{"n1", "n2", "n3"}
	cfg := RaftConfig{
		ElectionTimeoutMin: 50 * time.Millisecond,
		ElectionTimeoutMax: 100 * time.Millisecond,
		HeartbeatInterval:  10 * time.Millisecond,
	}

	cluster, err := NewCluster(nodeIDs, cfg, nil)
	if err != nil {
		t.Fatalf("NewCluster failed: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	leader, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	idx, term, err := leader.Propose([]byte("test-command"))
	if err != nil {
		t.Fatalf("Propose failed: %v", err)
	}

	if idx <= 0 {
		t.Errorf("Expected positive index, got %d", idx)
	}

	if term <= 0 {
		t.Errorf("Expected positive term, got %d", term)
	}

	deadline := time.Now().Add(2 * time.Second)
	committed := false
	for time.Now().Before(deadline) {
		if leader.CommitIndex() >= idx {
			committed = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !committed {
		t.Errorf("Log entry not committed within timeout, commitIndex=%d, expected >=%d", leader.CommitIndex(), idx)
	}
}

func TestProposeFromFollower(t *testing.T) {
	nodeIDs := []string{"n1", "n2", "n3"}
	cfg := RaftConfig{
		ElectionTimeoutMin: 150 * time.Millisecond,
		ElectionTimeoutMax: 300 * time.Millisecond,
		HeartbeatInterval:  50 * time.Millisecond,
	}

	cluster, err := NewCluster(nodeIDs, cfg, nil)
	if err != nil {
		t.Fatalf("NewCluster failed: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	leader, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	leader = cluster.Leader()
	if leader == nil {
		t.Fatal("No leader after sleep")
	}

	leaderID := leader.ID()
	var follower *RaftNode
	for id, node := range cluster.Nodes() {
		if id != leaderID && node.State() == Follower {
			follower = node
			break
		}
	}

	if follower == nil {
		t.Fatal("No follower found")
	}

	_, _, err = follower.Propose([]byte("test"))
	if !errors.Is(err, ErrNotLeader) {
		if follower.State() == Leader {
			t.Skip("Follower became leader due to election, skipping")
			return
		}
		t.Errorf("Expected ErrNotLeader, got %v", err)
	}
}

func TestLogReplication(t *testing.T) {
	nodeIDs := []string{"n1", "n2", "n3"}
	cfg := RaftConfig{
		ElectionTimeoutMin: 50 * time.Millisecond,
		ElectionTimeoutMax: 100 * time.Millisecond,
		HeartbeatInterval:  10 * time.Millisecond,
	}

	cluster, err := NewCluster(nodeIDs, cfg, nil)
	if err != nil {
		t.Fatalf("NewCluster failed: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	leader, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	numEntries := 10
	for i := 0; i < numEntries; i++ {
		_, _, err := leader.Propose([]byte("command-" + string(rune('0'+i))))
		if err != nil {
			t.Fatalf("Propose %d failed: %v", i, err)
		}
	}

	deadline := time.Now().Add(3 * time.Second)
	allReplicated := false
	for time.Now().Before(deadline) {
		allReplicated = true
		leaderLastIndex := leader.LastLogIndex()
		for id, node := range cluster.Nodes() {
			if id == leader.ID() {
				continue
			}
			if node.LastLogIndex() < leaderLastIndex {
				allReplicated = false
				break
			}
		}
		if allReplicated {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !allReplicated {
		t.Error("Log entries not replicated to all nodes")
	}

	leaderLastIndex := leader.LastLogIndex()
	if leaderLastIndex < numEntries {
		t.Errorf("Expected at least %d log entries, got %d", numEntries, leaderLastIndex)
	}
}

func TestStateMachineApply(t *testing.T) {
	nodeIDs := []string{"n1", "n2", "n3"}
	cfg := RaftConfig{
		ElectionTimeoutMin: 50 * time.Millisecond,
		ElectionTimeoutMax: 100 * time.Millisecond,
		HeartbeatInterval:  10 * time.Millisecond,
	}

	var sms []*MemoryStateMachine
	smFactory := func() StateMachine {
		sm := NewMemoryStateMachine()
		sms = append(sms, sm)
		return sm
	}

	cluster, err := NewCluster(nodeIDs, cfg, smFactory)
	if err != nil {
		t.Fatalf("NewCluster failed: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	leader, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	numEntries := 5
	for i := 0; i < numEntries; i++ {
		cmd := "key-" + string(rune('A'+i))
		_, _, err := leader.Propose([]byte(cmd))
		if err != nil {
			t.Fatalf("Propose %d failed: %v", i, err)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	allApplied := false
	for time.Now().Before(deadline) {
		allApplied = true
		leaderApplied := leader.LastApplied()
		if leaderApplied < numEntries {
			allApplied = false
		}
		if allApplied {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !allApplied {
		t.Errorf("Log entries not applied, lastApplied=%d, expected >=%d", leader.LastApplied(), numEntries)
	}
}

func TestRequestVote_StaleTerm(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node1 := NewRaftNode("n1", cfg, sm, transport, []string{"n1", "n2"})
	node2 := NewRaftNode("n2", cfg, sm, transport, []string{"n1", "n2"})

	node1.currentTerm = 5
	node2.currentTerm = 3

	reply := node2.HandleRequestVote(&RequestVoteRequest{
		Term:         3,
		CandidateID:  "n2",
		LastLogIndex: 0,
		LastLogTerm:  0,
	})

	if reply.Term != 3 {
		t.Errorf("Expected reply term 3, got %d", reply.Term)
	}

	if !reply.VoteGranted {
		t.Error("Expected vote granted for same term")
	}
}

func TestRequestVote_HigherTerm(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node1 := NewRaftNode("n1", cfg, sm, transport, []string{"n1", "n2"})
	node2 := NewRaftNode("n2", cfg, sm, transport, []string{"n1", "n2"})

	node1.becomeCandidate()
	node1Term := node1.currentTerm

	reply := node2.HandleRequestVote(&RequestVoteRequest{
		Term:         node1Term,
		CandidateID:  "n1",
		LastLogIndex: 0,
		LastLogTerm:  0,
	})

	if reply.Term < node1Term {
		t.Errorf("Expected reply term >= %d, got %d", node1Term, reply.Term)
	}
}

func TestAppendEntries_StaleTerm(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node1 := NewRaftNode("n1", cfg, sm, transport, []string{"n1", "n2"})
	node2 := NewRaftNode("n2", cfg, sm, transport, []string{"n1", "n2"})

	node1.currentTerm = 5
	node2.currentTerm = 3

	reply := node2.HandleAppendEntries(&AppendEntriesRequest{
		Term:         2,
		LeaderID:     "n1",
		PrevLogIndex: 0,
		PrevLogTerm:  0,
		Entries:      nil,
		LeaderCommit: 0,
	})

	if reply.Success {
		t.Error("Expected failure for stale term")
	}
	if reply.Term != 3 {
		t.Errorf("Expected reply term 3, got %d", reply.Term)
	}
}

func TestAppendEntries_LogConsistency(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node := NewRaftNode("n1", cfg, sm, transport, []string{"n1", "n2"})

	node.appendEntry(&LogEntry{Term: 1, Index: 1, Type: LogEntryNormal, Command: nil})
	node.appendEntry(&LogEntry{Term: 1, Index: 2, Type: LogEntryNormal, Command: nil})
	node.appendEntry(&LogEntry{Term: 2, Index: 3, Type: LogEntryNormal, Command: nil})

	reply := node.HandleAppendEntries(&AppendEntriesRequest{
		Term:         2,
		LeaderID:     "n2",
		PrevLogIndex: 2,
		PrevLogTerm:  1,
		Entries:      []*LogEntry{{Term: 2, Index: 3, Type: LogEntryNormal, Command: nil}},
		LeaderCommit: 2,
	})

	if !reply.Success {
		t.Errorf("Expected success, got failure")
	}
}

func TestAppendEntries_LogInconsistency(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node := NewRaftNode("n1", cfg, sm, transport, []string{"n1", "n2"})

	node.appendEntry(&LogEntry{Term: 1, Index: 1, Type: LogEntryNormal, Command: nil})
	node.appendEntry(&LogEntry{Term: 2, Index: 2, Type: LogEntryNormal, Command: nil})

	reply := node.HandleAppendEntries(&AppendEntriesRequest{
		Term:         2,
		LeaderID:     "n2",
		PrevLogIndex: 2,
		PrevLogTerm:  1,
		Entries:      []*LogEntry{{Term: 2, Index: 3, Type: LogEntryNormal, Command: nil}},
		LeaderCommit: 2,
	})

	if reply.Success {
		t.Error("Expected failure for log inconsistency")
	}

	if reply.ConflictIndex <= 0 {
		t.Errorf("Expected conflict index > 0, got %d", reply.ConflictIndex)
	}
}

func TestSnapshotInstall(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node := NewRaftNode("n1", cfg, sm, transport, []string{"n1", "n2"})

	snapData, err := sm.Snapshot()
	if err != nil {
		t.Fatalf("Failed to create snapshot data: %v", err)
	}

	req := &InstallSnapshotRequest{
		Term:              1,
		LeaderID:          "n2",
		LastIncludedIndex: 100,
		LastIncludedTerm:  5,
		Config:            NewConfiguration([]string{"n1", "n2"}),
		Data:              snapData,
		Done:              true,
	}

	reply := node.HandleInstallSnapshot(req)

	if !reply.Success {
		t.Error("Expected snapshot install to succeed")
	}

	if node.LastLogIndex() < 100 {
		t.Errorf("Expected log offset >= 100, got %d", node.LastLogIndex())
	}

	if node.CommitIndex() < 100 {
		t.Errorf("Expected commit index >= 100, got %d", node.CommitIndex())
	}
}

func TestSnapshotInstall_StaleTerm(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node := NewRaftNode("n1", cfg, sm, transport, []string{"n1", "n2"})
	node.currentTerm = 5

	req := &InstallSnapshotRequest{
		Term:              3,
		LeaderID:          "n2",
		LastIncludedIndex: 100,
		LastIncludedTerm:  2,
		Config:            NewConfiguration([]string{"n1", "n2"}),
		Data:              []byte("snapshot-data"),
		Done:              true,
	}

	reply := node.HandleInstallSnapshot(req)

	if reply.Success {
		t.Error("Expected failure for stale term")
	}
}

func TestSnapshotInstall_AlreadyApplied(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node := NewRaftNode("n1", cfg, sm, transport, []string{"n1", "n2"})
	node.lastSnapshotIndex = 200
	node.lastSnapshotTerm = 10
	node.logOffset = 200

	req := &InstallSnapshotRequest{
		Term:              5,
		LeaderID:          "n2",
		LastIncludedIndex: 100,
		LastIncludedTerm:  5,
		Config:            NewConfiguration([]string{"n1", "n2"}),
		Data:              []byte("snapshot-data"),
		Done:              true,
	}

	reply := node.HandleInstallSnapshot(req)

	if !reply.Success {
		t.Error("Expected success for already applied snapshot")
	}
}

func TestCompactLog(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node := NewRaftNode("n1", cfg, sm, transport, []string{"n1"})

	for i := 1; i <= 10; i++ {
		node.appendEntry(&LogEntry{Term: 1, Index: i, Type: LogEntryNormal, Command: nil})
	}
	node.commitIndex = 8

	err := node.CompactLog(5)
	if err != nil {
		t.Fatalf("CompactLog failed: %v", err)
	}

	if node.lastSnapshotIndex != 5 {
		t.Errorf("Expected snapshot index 5, got %d", node.lastSnapshotIndex)
	}

	if node.logOffset != 5 {
		t.Errorf("Expected log offset 5, got %d", node.logOffset)
	}

	if node.LastLogIndex() != 10 {
		t.Errorf("Expected last log index 10, got %d", node.LastLogIndex())
	}
}

func TestCompactLog_BeforeCommit(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node := NewRaftNode("n1", cfg, sm, transport, []string{"n1"})

	for i := 1; i <= 10; i++ {
		node.appendEntry(&LogEntry{Term: 1, Index: i, Type: LogEntryNormal, Command: nil})
	}
	node.commitIndex = 3

	err := node.CompactLog(5)
	if !errors.Is(err, ErrInvalidIndex) {
		t.Errorf("Expected ErrInvalidIndex, got %v", err)
	}
}

func TestCompactLog_AlreadyCompacted(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node := NewRaftNode("n1", cfg, sm, transport, []string{"n1"})
	node.logOffset = 10
	node.lastSnapshotIndex = 10
	node.commitIndex = 20

	err := node.CompactLog(5)
	if err != nil {
		t.Errorf("Expected no error for already compacted, got %v", err)
	}
}

func TestMemoryStateMachine(t *testing.T) {
	sm := NewMemoryStateMachine()

	entry := &LogEntry{
		Term:    1,
		Index:   1,
		Type:    LogEntryNormal,
		Command: []byte("test-key"),
	}

	err := sm.Apply(entry)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	val, ok := sm.Get("test-key")
	if !ok {
		t.Error("Expected key to exist")
	}
	if val != "test-key" {
		t.Errorf("Expected value 'test-key', got '%s'", val)
	}

	if sm.Count() != 1 {
		t.Errorf("Expected count 1, got %d", sm.Count())
	}
}

func TestMemoryStateMachine_Snapshot(t *testing.T) {
	sm := NewMemoryStateMachine()

	keys := []string{"key-A", "key-B", "key-C", "key-D", "key-E"}
	for i, key := range keys {
		entry := &LogEntry{
			Term:    1,
			Index:   i + 1,
			Type:    LogEntryNormal,
			Command: []byte(key),
		}
		err := sm.Apply(entry)
		if err != nil {
			t.Fatalf("Apply failed: %v", err)
		}
	}

	for _, key := range keys {
		val, ok := sm.Get(key)
		if !ok {
			t.Errorf("Expected key %s to exist before snapshot", key)
		}
		if val != key {
			t.Errorf("Expected value %s for key %s before snapshot, got %s", key, key, val)
		}
	}

	snapData, err := sm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	if len(snapData) == 0 {
		t.Error("Snapshot data should not be empty")
	}

	sm2 := NewMemoryStateMachine()
	snap := &Snapshot{
		LastIncludedIndex: 5,
		LastIncludedTerm:  1,
		Data:              snapData,
	}

	err = sm2.ApplySnapshot(snap)
	if err != nil {
		t.Fatalf("ApplySnapshot failed: %v", err)
	}

	if sm2.Count() != 5 {
		t.Errorf("Expected count 5 after snapshot restore, got %d", sm2.Count())
	}

	for _, key := range keys {
		val, ok := sm2.Get(key)
		if !ok {
			t.Errorf("Expected key %s to exist after restore", key)
		}
		if val != key {
			t.Errorf("Expected value %s for key %s after restore, got %s", key, key, val)
		}
	}

	smEmpty := NewMemoryStateMachine()
	emptySnap := &Snapshot{
		LastIncludedIndex: 0,
		LastIncludedTerm:  0,
		Data:              nil,
	}
	err = smEmpty.ApplySnapshot(emptySnap)
	if err != nil {
		t.Fatalf("ApplySnapshot with nil data failed: %v", err)
	}
	if smEmpty.Count() != 0 {
		t.Errorf("Expected count 0 after empty snapshot apply, got %d", smEmpty.Count())
	}

	smEmpty2 := NewMemoryStateMachine()
	emptySnap2 := &Snapshot{
		LastIncludedIndex: 0,
		LastIncludedTerm:  0,
		Data:              []byte{},
	}
	err = smEmpty2.ApplySnapshot(emptySnap2)
	if err != nil {
		t.Fatalf("ApplySnapshot with empty byte slice failed: %v", err)
	}
	if smEmpty2.Count() != 0 {
		t.Errorf("Expected count 0 after empty byte slice snapshot apply, got %d", smEmpty2.Count())
	}

	sm3 := NewMemoryStateMachine()
	corruptSnap := &Snapshot{
		LastIncludedIndex: 5,
		LastIncludedTerm:  1,
		Data:              []byte("this-is-not-valid-gob-data"),
	}
	err = sm3.ApplySnapshot(corruptSnap)
	if err == nil {
		t.Error("Expected error when applying corrupt snapshot data")
	}

	sm4 := NewMemoryStateMachine()
	for i := 0; i < 100; i++ {
		entry := &LogEntry{
			Term:    1,
			Index:   i + 1,
			Type:    LogEntryNormal,
			Command: []byte(fmt.Sprintf("large-key-%04d", i)),
		}
		err := sm4.Apply(entry)
		if err != nil {
			t.Fatalf("Apply large data failed: %v", err)
		}
	}
	if sm4.Count() != 100 {
		t.Fatalf("Expected count 100 for large test, got %d", sm4.Count())
	}
	largeSnapData, err := sm4.Snapshot()
	if err != nil {
		t.Fatalf("Large snapshot failed: %v", err)
	}
	sm5 := NewMemoryStateMachine()
	err = sm5.ApplySnapshot(&Snapshot{
		LastIncludedIndex: 100,
		LastIncludedTerm:  1,
		Data:              largeSnapData,
	})
	if err != nil {
		t.Fatalf("Apply large snapshot failed: %v", err)
	}
	if sm5.Count() != 100 {
		t.Errorf("Expected count 100 after large snapshot restore, got %d", sm5.Count())
	}
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("large-key-%04d", i)
		val, ok := sm5.Get(key)
		if !ok {
			t.Errorf("Expected key %s to exist after large restore", key)
		}
		if val != key {
			t.Errorf("Expected value %s for key %s after large restore, got %s", key, key, val)
		}
	}

	sm6 := NewMemoryStateMachine()
	for i, key := range keys {
		entry := &LogEntry{
			Term:    1,
			Index:   i + 1,
			Type:    LogEntryNormal,
			Command: []byte(key),
		}
		sm6.Apply(entry)
	}
	incrementalSnap, _ := sm6.Snapshot()
	incrementalEntry := &LogEntry{
		Term:    2,
		Index:   6,
		Type:    LogEntryNormal,
		Command: []byte("key-F"),
	}
	sm6.Apply(incrementalEntry)

	sm7 := NewMemoryStateMachine()
	sm7.ApplySnapshot(&Snapshot{
		LastIncludedIndex: 5,
		LastIncludedTerm:  1,
		Data:              incrementalSnap,
	})
	sm7.Apply(incrementalEntry)
	if sm7.Count() != 6 {
		t.Errorf("Expected count 6 after incremental apply, got %d", sm7.Count())
	}
	if _, ok := sm7.Get("key-F"); !ok {
		t.Error("Expected key-F to exist after incremental apply")
	}
}

func TestSnapshotInstallingError(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node := NewRaftNode("n1", cfg, sm, transport, []string{"n1", "n2"})
	node.state = Leader
	node.currentTerm = 1
	node.running = true

	node.snapshotInstalling = true

	_, _, err := node.Propose([]byte("test-data"))
	if !errors.Is(err, ErrSnapshotInstalling) {
		t.Errorf("Expected ErrSnapshotInstalling, got err=%v", err)
	}

	node.snapshotInstalling = false

	idx, term, err := node.Propose([]byte("test-data-2"))
	if err != nil {
		t.Fatalf("Expected no error after snapshot installing cleared, got err=%v", err)
	}
	if idx <= 0 {
		t.Errorf("Expected positive index, got %d", idx)
	}
	if term != 1 {
		t.Errorf("Expected term 1, got %d", term)
	}
}

func TestAddNode(t *testing.T) {
	nodeIDs := []string{"n1", "n2", "n3"}
	cfg := RaftConfig{
		ElectionTimeoutMin: 50 * time.Millisecond,
		ElectionTimeoutMax: 100 * time.Millisecond,
		HeartbeatInterval:  10 * time.Millisecond,
	}

	cluster, err := NewCluster(nodeIDs, cfg, nil)
	if err != nil {
		t.Fatalf("NewCluster failed: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	leader, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	err = leader.AddNode("n4")
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	configCompleted := false
	for time.Now().Before(deadline) {
		cfg := leader.Config()
		if cfg.Contains("n4") && !leader.configChangeInFlight && leader.joint == nil {
			configCompleted = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !configCompleted {
		t.Errorf("Config change not completed. leader.config.Contains(n4)=%v, configChangeInFlight=%v, joint=%v",
			leader.Config().Contains("n4"), leader.configChangeInFlight, leader.joint != nil)
	}

	leaderCfg := leader.Config()
	if !leaderCfg.Contains("n4") {
		t.Error("Final config should contain n4")
	}
	if leaderCfg.Size() != 4 {
		t.Errorf("Expected config size 4, got %d", leaderCfg.Size())
	}
	if leader.configChangeInFlight {
		t.Error("Config change should be completed")
	}
	if leader.joint != nil {
		t.Error("Joint config should be cleared")
	}
}

func TestRemoveNode(t *testing.T) {
	nodeIDs := []string{"n1", "n2", "n3", "n4", "n5"}
	cfg := RaftConfig{
		ElectionTimeoutMin: 50 * time.Millisecond,
		ElectionTimeoutMax: 100 * time.Millisecond,
		HeartbeatInterval:  10 * time.Millisecond,
	}

	cluster, err := NewCluster(nodeIDs, cfg, nil)
	if err != nil {
		t.Fatalf("NewCluster failed: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	leader, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	err = leader.RemoveNode("n5")
	if err != nil {
		t.Fatalf("RemoveNode failed: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	configCompleted := false
	for time.Now().Before(deadline) {
		cfg := leader.Config()
		if !cfg.Contains("n5") && !leader.configChangeInFlight && leader.joint == nil {
			configCompleted = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !configCompleted {
		t.Errorf("Config change not completed. leader.config.Contains(n5)=%v, configChangeInFlight=%v, joint=%v",
			leader.Config().Contains("n5"), leader.configChangeInFlight, leader.joint != nil)
	}

	leaderCfg := leader.Config()
	if leaderCfg.Contains("n5") {
		t.Error("Final config should not contain n5")
	}
	if leaderCfg.Size() != 4 {
		t.Errorf("Expected config size 4, got %d", leaderCfg.Size())
	}
	if leader.configChangeInFlight {
		t.Error("Config change should be completed")
	}
	if leader.joint != nil {
		t.Error("Joint config should be cleared")
	}
}

func TestAddNode_NotLeader(t *testing.T) {
	nodeIDs := []string{"n1", "n2", "n3"}
	cfg := RaftConfig{
		ElectionTimeoutMin: 50 * time.Millisecond,
		ElectionTimeoutMax: 100 * time.Millisecond,
		HeartbeatInterval:  20 * time.Millisecond,
	}

	cluster, err := NewCluster(nodeIDs, cfg, nil)
	if err != nil {
		t.Fatalf("NewCluster failed: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	leader, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	var follower *RaftNode
	for id, node := range cluster.Nodes() {
		if id != leader.ID() {
			follower = node
			break
		}
	}

	err = follower.AddNode("n4")
	if !errors.Is(err, ErrNotLeader) {
		t.Errorf("Expected ErrNotLeader, got %v", err)
	}
}

func TestRemoveNode_NotLeader(t *testing.T) {
	nodeIDs := []string{"n1", "n2", "n3"}
	cfg := RaftConfig{
		ElectionTimeoutMin: 50 * time.Millisecond,
		ElectionTimeoutMax: 100 * time.Millisecond,
		HeartbeatInterval:  20 * time.Millisecond,
	}

	cluster, err := NewCluster(nodeIDs, cfg, nil)
	if err != nil {
		t.Fatalf("NewCluster failed: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	leader, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	var follower *RaftNode
	for id, node := range cluster.Nodes() {
		if id != leader.ID() {
			follower = node
			break
		}
	}

	err = follower.RemoveNode("n3")
	if !errors.Is(err, ErrNotLeader) {
		t.Errorf("Expected ErrNotLeader, got %v", err)
	}
}

func TestAddNode_AlreadyExists(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node := NewRaftNode("n1", cfg, sm, transport, []string{"n1", "n2"})
	node.state = Leader
	node.currentTerm = 1

	err := node.AddNode("n2")
	if !errors.Is(err, ErrNodeExists) {
		t.Errorf("Expected ErrNodeExists, got %v", err)
	}
}

func TestRemoveNode_NotExists(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node := NewRaftNode("n1", cfg, sm, transport, []string{"n1", "n2"})
	node.state = Leader
	node.currentTerm = 1

	err := node.RemoveNode("n3")
	if !errors.Is(err, ErrNodeNotFound) {
		t.Errorf("Expected ErrNodeNotFound, got %v", err)
	}
}

func TestRemoveNode_EmptyConfig(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node := NewRaftNode("n1", cfg, sm, transport, []string{"n1"})
	node.state = Leader
	node.currentTerm = 1

	err := node.RemoveNode("n1")
	if !errors.Is(err, ErrEmptyConfig) {
		t.Errorf("Expected ErrEmptyConfig, got %v", err)
	}
}

func TestClusterStartStop(t *testing.T) {
	nodeIDs := []string{"n1", "n2", "n3"}
	cfg := DefaultRaftConfig()

	cluster, err := NewCluster(nodeIDs, cfg, nil)
	if err != nil {
		t.Fatalf("NewCluster failed: %v", err)
	}

	cluster.Start()
	cluster.Stop()
}

func TestNodeStartStop(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node := NewRaftNode("n1", cfg, sm, transport, []string{"n1"})

	node.Start()
	node.Start()

	if !node.running {
		t.Error("Node should be running")
	}

	node.Stop()
	node.Stop()

	if !node.stopped {
		t.Error("Node should be stopped")
	}
}

func TestPropose_StoppedNode(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node := NewRaftNode("n1", cfg, sm, transport, []string{"n1"})
	node.state = Leader
	node.currentTerm = 1
	node.running = false
	node.stopped = true

	_, _, err := node.Propose([]byte("test"))
	if !errors.Is(err, ErrNodeStopped) {
		t.Errorf("Expected ErrNodeStopped, got %v", err)
	}
}

func TestMemoryTransport(t *testing.T) {
	transport := NewMemoryTransport()

	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()
	node := NewRaftNode("n1", cfg, sm, transport, []string{"n1"})

	transport.RegisterNode("n1", node)

	_, err := transport.SendRequestVote("n1", &RequestVoteRequest{
		Term:         1,
		CandidateID:  "n2",
		LastLogIndex: 0,
		LastLogTerm:  0,
	})
	if err != nil {
		t.Errorf("SendRequestVote failed: %v", err)
	}

	_, err = transport.SendRequestVote("n2", &RequestVoteRequest{})
	if !errors.Is(err, ErrNodeNotFound) {
		t.Errorf("Expected ErrNodeNotFound, got %v", err)
	}

	transport.UnregisterNode("n1")
	transport.Close()
}

func TestMemoryTransport_WithDelay(t *testing.T) {
	transport := NewMemoryTransportWithDelay(1 * time.Millisecond)

	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()
	node := NewRaftNode("n1", cfg, sm, transport, []string{"n1"})

	transport.RegisterNode("n1", node)

	start := time.Now()
	_, err := transport.SendRequestVote("n1", &RequestVoteRequest{
		Term:         1,
		CandidateID:  "n2",
		LastLogIndex: 0,
		LastLogTerm:  0,
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("SendRequestVote failed: %v", err)
	}

	if elapsed < 1*time.Millisecond {
		t.Errorf("Expected delay of at least 1ms, got %v", elapsed)
	}

	transport.Close()
}

func TestJointConfiguration(t *testing.T) {
	oldCfg := NewConfiguration([]string{"n1", "n2", "n3"})
	newCfg := NewConfiguration([]string{"n1", "n2", "n3", "n4"})

	joint := &JointConfiguration{
		Old: oldCfg,
		New: newCfg,
	}

	if !joint.Contains("n1") {
		t.Error("n1 should be in joint config")
	}

	if !joint.Contains("n4") {
		t.Error("n4 should be in joint config")
	}

	if joint.Contains("n5") {
		t.Error("n5 should not be in joint config")
	}

	if joint.OldQuorum() != 2 {
		t.Errorf("Expected old quorum 2, got %d", joint.OldQuorum())
	}

	if joint.NewQuorum() != 3 {
		t.Errorf("Expected new quorum 3, got %d", joint.NewQuorum())
	}
}

func TestConcurrentPropose(t *testing.T) {
	nodeIDs := []string{"n1", "n2", "n3"}
	cfg := RaftConfig{
		ElectionTimeoutMin: 50 * time.Millisecond,
		ElectionTimeoutMax: 100 * time.Millisecond,
		HeartbeatInterval:  10 * time.Millisecond,
	}

	cluster, err := NewCluster(nodeIDs, cfg, nil)
	if err != nil {
		t.Fatalf("NewCluster failed: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	leader, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	var wg sync.WaitGroup
	numGoroutines := 5
	numPerGoroutine := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numPerGoroutine; j++ {
				cmd := "concurrent-" + string(rune('A'+id)) + "-" + string(rune('0'+j))
				_, _, err := leader.Propose([]byte(cmd))
				if err != nil && !errors.Is(err, ErrNotLeader) && !errors.Is(err, ErrConfigChangeInFlight) {
					t.Errorf("Propose failed: %v", err)
					return
				}
			}
		}(i)
	}

	wg.Wait()

	deadline := time.Now().Add(3 * time.Second)
	committed := false
	for time.Now().Before(deadline) {
		if leader.CommitIndex() >= numGoroutines*numPerGoroutine/2 {
			committed = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !committed {
		t.Errorf("Not enough entries committed, commitIndex=%d", leader.CommitIndex())
	}
}

func TestLogEntryTypes(t *testing.T) {
	entry1 := &LogEntry{Type: LogEntryNormal}
	entry2 := &LogEntry{Type: LogEntryConfigJoint}
	entry3 := &LogEntry{Type: LogEntryConfigNew}

	if entry1.Type != LogEntryNormal {
		t.Error("Expected LogEntryNormal")
	}

	if entry2.Type != LogEntryConfigJoint {
		t.Error("Expected LogEntryConfigJoint")
	}

	if entry3.Type != LogEntryConfigNew {
		t.Error("Expected LogEntryConfigNew")
	}
}

func TestSnapshotStruct(t *testing.T) {
	cfg := NewConfiguration([]string{"n1", "n2"})
	snap := &Snapshot{
		LastIncludedIndex: 100,
		LastIncludedTerm:  5,
		Config:            cfg,
		Data:              []byte("test-data"),
	}

	if snap.LastIncludedIndex != 100 {
		t.Errorf("Expected 100, got %d", snap.LastIncludedIndex)
	}

	if snap.LastIncludedTerm != 5 {
		t.Errorf("Expected 5, got %d", snap.LastIncludedTerm)
	}

	if len(snap.Data) == 0 {
		t.Error("Data should not be empty")
	}
}

func TestApplyResult(t *testing.T) {
	result := &ApplyResult{
		Index: 42,
		Term:  3,
		Err:   nil,
	}

	if result.Index != 42 {
		t.Errorf("Expected index 42, got %d", result.Index)
	}

	if result.Term != 3 {
		t.Errorf("Expected term 3, got %d", result.Term)
	}

	if result.Err != nil {
		t.Errorf("Expected nil error, got %v", result.Err)
	}
}

func TestLeaderElection_SingleNode(t *testing.T) {
	nodeIDs := []string{"n1"}
	cfg := RaftConfig{
		ElectionTimeoutMin: 50 * time.Millisecond,
		ElectionTimeoutMax: 100 * time.Millisecond,
		HeartbeatInterval:  20 * time.Millisecond,
	}

	cluster, err := NewCluster(nodeIDs, cfg, nil)
	if err != nil {
		t.Fatalf("NewCluster failed: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	leader, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	if leader == nil {
		t.Fatal("Leader is nil")
	}

	if leader.ID() != "n1" {
		t.Errorf("Expected leader n1, got %s", leader.ID())
	}
}

func TestProposeMultipleEntries(t *testing.T) {
	nodeIDs := []string{"n1", "n2", "n3"}
	cfg := RaftConfig{
		ElectionTimeoutMin: 50 * time.Millisecond,
		ElectionTimeoutMax: 100 * time.Millisecond,
		HeartbeatInterval:  10 * time.Millisecond,
	}

	cluster, err := NewCluster(nodeIDs, cfg, nil)
	if err != nil {
		t.Fatalf("NewCluster failed: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	leader, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	numEntries := 20
	for i := 0; i < numEntries; i++ {
		idx, _, err := leader.Propose([]byte("entry-" + string(rune('0'+i%10))))
		if err != nil {
			t.Fatalf("Propose %d failed: %v", i, err)
		}
		if idx != i+1 {
			t.Errorf("Expected index %d, got %d", i+1, idx)
		}
	}

	deadline := time.Now().Add(3 * time.Second)
	committed := false
	for time.Now().Before(deadline) {
		if leader.CommitIndex() >= numEntries {
			committed = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !committed {
		t.Errorf("Not all entries committed, commitIndex=%d, expected >=%d", leader.CommitIndex(), numEntries)
	}
}

func TestJointConfig_LogReplicationDuringConfigChange(t *testing.T) {
	nodeIDs := []string{"n1", "n2", "n3"}
	cfg := RaftConfig{
		ElectionTimeoutMin: 150 * time.Millisecond,
		ElectionTimeoutMax: 300 * time.Millisecond,
		HeartbeatInterval:  50 * time.Millisecond,
	}

	var sms []*MemoryStateMachine
	smFactory := func() StateMachine {
		sm := NewMemoryStateMachine()
		sms = append(sms, sm)
		return sm
	}

	cluster, err := NewCluster(nodeIDs, cfg, smFactory)
	if err != nil {
		t.Fatalf("NewCluster failed: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	leader, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	preCommitIndex := leader.CommitIndex()

	err = leader.AddNode("n4")
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	leader = cluster.Leader()
	if leader == nil {
		leader, err = cluster.WaitForLeader(2 * time.Second)
		if err != nil {
			t.Fatalf("No leader after config change start: %v", err)
		}
	}

	var entriesDuringChange []int
	for i := 0; i < 5; i++ {
		idx, _, err := leader.Propose([]byte("during-config-" + string(rune('A'+i))))
		if err == nil {
			entriesDuringChange = append(entriesDuringChange, idx)
		} else if errors.Is(err, ErrNotLeader) {
			leader = cluster.Leader()
			if leader == nil {
				leader, err = cluster.WaitForLeader(1 * time.Second)
				if err != nil {
					t.Fatalf("No leader during config change: %v", err)
				}
			}
			i--
			continue
		} else if !errors.Is(err, ErrConfigChangeInFlight) {
			t.Fatalf("Propose during config change failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if len(entriesDuringChange) == 0 {
		t.Fatal("No entries were proposed during config change")
	}

	deadline := time.Now().Add(5 * time.Second)
	allCommitted := false
	configDone := false
	for time.Now().Before(deadline) {
		leader = cluster.Leader()
		if leader == nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		allCommitted = true
		for _, idx := range entriesDuringChange {
			if leader.CommitIndex() < idx {
				allCommitted = false
				break
			}
		}

		if !leader.configChangeInFlight && leader.joint == nil {
			configDone = true
		}

		if allCommitted && configDone {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !allCommitted {
		t.Errorf("Not all entries committed during config change. commitIndex=%d, expected >=%d",
			leader.CommitIndex(), entriesDuringChange[len(entriesDuringChange)-1])
	}

	if leader.CommitIndex() <= preCommitIndex {
		t.Errorf("Commit index should have advanced during config change. Before: %d, After: %d",
			preCommitIndex, leader.CommitIndex())
	}

	leader = cluster.Leader()
	if leader == nil {
		leader, err = cluster.WaitForLeader(2 * time.Second)
		if err != nil {
			t.Fatalf("No leader at end of test: %v", err)
		}
	}

	leaderCfg := leader.Config()
	if !leaderCfg.Contains("n4") {
		t.Errorf("Final config should contain n4. Config nodes: %v", leaderCfg.NodeIDs())
	}
	if leader.configChangeInFlight {
		t.Error("Config change should be completed")
	}
	if leader.joint != nil {
		t.Error("Joint config should be cleared")
	}
}

func TestAddNode_WithConcurrentPropose(t *testing.T) {
	nodeIDs := []string{"n1", "n2", "n3"}
	cfg := RaftConfig{
		ElectionTimeoutMin: 50 * time.Millisecond,
		ElectionTimeoutMax: 100 * time.Millisecond,
		HeartbeatInterval:  10 * time.Millisecond,
	}

	cluster, err := NewCluster(nodeIDs, cfg, nil)
	if err != nil {
		t.Fatalf("NewCluster failed: %v", err)
	}
	defer cluster.Stop()

	cluster.Start()

	leader, err := cluster.WaitForLeader(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to elect leader: %v", err)
	}

	var wg sync.WaitGroup
	var proposeErr error
	var addNodeErr error

	wg.Add(2)

	go func() {
		defer wg.Done()
		addNodeErr = leader.AddNode("n4")
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_, _, err := leader.Propose([]byte("concurrent-" + string(rune('A'+i%10))))
			if err != nil && !errors.Is(err, ErrNotLeader) && !errors.Is(err, ErrConfigChangeInFlight) {
				proposeErr = err
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	wg.Wait()

	if addNodeErr != nil {
		t.Fatalf("AddNode failed: %v", addNodeErr)
	}

	if proposeErr != nil {
		t.Fatalf("Concurrent propose failed: %v", proposeErr)
	}

	deadline := time.Now().Add(3 * time.Second)
	configCompleted := false
	for time.Now().Before(deadline) {
		cfg := leader.Config()
		if cfg.Contains("n4") && !leader.configChangeInFlight && leader.joint == nil {
			configCompleted = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !configCompleted {
		t.Error("Config change not completed")
	}

	leaderCfg := leader.Config()
	if !leaderCfg.Contains("n4") {
		t.Error("Final config should contain n4")
	}
	if leaderCfg.Size() != 4 {
		t.Errorf("Expected config size 4, got %d", leaderCfg.Size())
	}
}

func TestLogCompactedError(t *testing.T) {
	transport := NewMemoryTransport()
	sm := NewMemoryStateMachine()
	cfg := DefaultRaftConfig()

	node := NewRaftNode("n1", cfg, sm, transport, []string{"n1"})
	node.state = Leader
	node.currentTerm = 1

	for i := 1; i <= 20; i++ {
		node.appendEntry(&LogEntry{Term: 1, Index: i, Type: LogEntryNormal, Command: nil})
	}
	node.commitIndex = 15
	node.lastApplied = 15

	err := node.CompactLog(10)
	if err != nil {
		t.Fatalf("CompactLog failed: %v", err)
	}

	entry, err := node.GetLogEntry(5)
	if !errors.Is(err, ErrLogCompacted) {
		t.Errorf("Expected ErrLogCompacted for index 5, got err=%v, entry=%v", err, entry)
	}
	if entry != nil {
		t.Errorf("Expected nil entry for compacted index, got %v", entry)
	}

	entry, err = node.GetLogEntry(100)
	if !errors.Is(err, ErrInvalidIndex) {
		t.Errorf("Expected ErrInvalidIndex for index 100, got err=%v, entry=%v", err, entry)
	}
	if entry != nil {
		t.Errorf("Expected nil entry for invalid index, got %v", entry)
	}

	entry, err = node.GetLogEntry(15)
	if err != nil {
		t.Errorf("Expected no error for valid index 15, got err=%v", err)
	}
	if entry == nil {
		t.Error("Expected non-nil entry for valid index 15")
	}
	if entry.Index != 15 {
		t.Errorf("Expected entry index 15, got %d", entry.Index)
	}

	entry, err = node.GetLogEntry(10)
	if err != nil {
		t.Errorf("Expected no error for index 10 (compaction boundary), got err=%v", err)
	}
	if entry == nil {
		t.Error("Expected non-nil entry for index 10")
	}
}
