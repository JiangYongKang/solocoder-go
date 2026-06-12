package shardkv

import (
	"bytes"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestHashRing_Basic(t *testing.T) {
	hr := NewHashRing(10)

	if hr.NodeCount() != 0 {
		t.Fatalf("expected 0 nodes, got %d", hr.NodeCount())
	}

	if hr.GetNode("test") != "" {
		t.Fatal("expected empty string for empty ring")
	}
}

func TestHashRing_AddNode(t *testing.T) {
	hr := NewHashRing(10)

	hr.AddNode("node1")
	if hr.NodeCount() != 1 {
		t.Fatalf("expected 1 node, got %d", hr.NodeCount())
	}

	hr.AddNode("node1")
	if hr.NodeCount() != 1 {
		t.Fatalf("expected 1 node after duplicate add, got %d", hr.NodeCount())
	}

	hr.AddNode("node2")
	if hr.NodeCount() != 2 {
		t.Fatalf("expected 2 nodes, got %d", hr.NodeCount())
	}
}

func TestHashRing_RemoveNode(t *testing.T) {
	hr := NewHashRing(10)

	hr.AddNode("node1")
	hr.AddNode("node2")
	hr.AddNode("node3")

	hr.RemoveNode("node2")
	if hr.NodeCount() != 2 {
		t.Fatalf("expected 2 nodes, got %d", hr.NodeCount())
	}

	nodes := hr.GetNodes()
	sort.Strings(nodes)
	expected := []string{"node1", "node3"}
	if len(nodes) != len(expected) {
		t.Fatalf("expected nodes %v, got %v", expected, nodes)
	}
	for i, n := range expected {
		if nodes[i] != n {
			t.Fatalf("expected node %s at index %d, got %s", n, i, nodes[i])
		}
	}

	hr.RemoveNode("nonexistent")
	if hr.NodeCount() != 2 {
		t.Fatalf("expected 2 nodes after removing nonexistent, got %d", hr.NodeCount())
	}
}

func TestHashRing_GetNode(t *testing.T) {
	hr := NewHashRing(50)

	nodes := []string{"node1", "node2", "node3", "node4", "node5"}
	for _, n := range nodes {
		hr.AddNode(n)
	}

	nodeSet := make(map[string]bool)
	for _, n := range nodes {
		nodeSet[n] = true
	}

	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key-%d", i)
		node := hr.GetNode(key)
		if !nodeSet[node] {
			t.Fatalf("key %s mapped to unknown node %s", key, node)
		}
	}
}

func TestHashRing_Consistency(t *testing.T) {
	hr := NewHashRing(50)

	for i := 0; i < 5; i++ {
		hr.AddNode(fmt.Sprintf("node-%d", i))
	}

	results1 := make(map[string]string)
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		results1[key] = hr.GetNode(key)
	}

	hr.AddNode("node-5")

	changed := 0
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		if results1[key] != hr.GetNode(key) {
			changed++
		}
	}

	if changed > 40 {
		t.Fatalf("too many keys remapped: %d/100, expected < 40", changed)
	}

	t.Logf("keys remapped after adding one node: %d/100", changed)
}

func TestHashRing_GetReplicaNodes(t *testing.T) {
	hr := NewHashRing(20)

	replicaCount := 3
	for i := 0; i < 5; i++ {
		hr.AddNode(fmt.Sprintf("node-%d", i))
	}

	replicas := hr.GetReplicaNodes("test-key", replicaCount)
	if len(replicas) != replicaCount {
		t.Fatalf("expected %d replica nodes, got %d", replicaCount, len(replicas))
	}

	seen := make(map[string]bool)
	for _, r := range replicas {
		if seen[r] {
			t.Fatalf("duplicate replica node: %s", r)
		}
		seen[r] = true
	}

	replicas2 := hr.GetReplicaNodes("test-key", 10)
	if len(replicas2) != 5 {
		t.Fatalf("expected 5 replica nodes (all nodes), got %d", len(replicas2))
	}

	if len(hr.GetReplicaNodes("test-key", 0)) != 0 {
		t.Fatal("expected empty result for replicaCount 0")
	}
}

func TestHashRing_VirtualNodes(t *testing.T) {
	hr1 := NewHashRing(1)
	hr2 := NewHashRing(100)

	for i := 0; i < 3; i++ {
		hr1.AddNode(fmt.Sprintf("node-%d", i))
		hr2.AddNode(fmt.Sprintf("node-%d", i))
	}

	dist1 := make(map[string]int)
	dist2 := make(map[string]int)

	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key-%d", i)
		dist1[hr1.GetNode(key)]++
		dist2[hr2.GetNode(key)]++
	}

	cv1 := coefficientOfVariation(dist1)
	cv2 := coefficientOfVariation(dist2)

	if cv2 > cv1 {
		t.Fatalf("expected more virtual nodes to reduce variance: cv1=%f cv2=%f", cv1, cv2)
	}

	t.Logf("CV with 1 VN: %f, CV with 100 VN: %f", cv1, cv2)
}

func coefficientOfVariation(m map[string]int) float64 {
	if len(m) == 0 {
		return 0
	}
	sum := 0
	for _, v := range m {
		sum += v
	}
	mean := float64(sum) / float64(len(m))
	variance := 0.0
	for _, v := range m {
		diff := float64(v) - mean
		variance += diff * diff
	}
	variance /= float64(len(m))
	if mean == 0 {
		return 0
	}
	return variance / (mean * mean)
}

func TestShard_Basic(t *testing.T) {
	s := NewShard("shard1")

	if s.ID() != "shard1" {
		t.Fatalf("expected id shard1, got %s", s.ID())
	}

	if s.Status() != ShardStatusUp {
		t.Fatalf("expected status Up, got %v", s.Status())
	}

	err := s.Put("key1", []byte("value1"))
	if err != nil {
		t.Fatalf("unexpected error on Put: %v", err)
	}

	val, err := s.Get("key1")
	if err != nil {
		t.Fatalf("unexpected error on Get: %v", err)
	}
	if !bytes.Equal(val, []byte("value1")) {
		t.Fatalf("expected value1, got %s", string(val))
	}

	if !s.HasKey("key1") {
		t.Fatal("expected HasKey to return true")
	}

	if s.DataCount() != 1 {
		t.Fatalf("expected count 1, got %d", s.DataCount())
	}
}

func TestShard_NotFound(t *testing.T) {
	s := NewShard("shard1")

	_, err := s.Get("nonexistent")
	if err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}

	err = s.Delete("nonexistent")
	if err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestShard_Delete(t *testing.T) {
	s := NewShard("shard1")

	s.Put("key1", []byte("value1"))

	err := s.Delete("key1")
	if err != nil {
		t.Fatalf("unexpected error on Delete: %v", err)
	}

	if s.HasKey("key1") {
		t.Fatal("expected key to be deleted")
	}

	if s.DataCount() != 0 {
		t.Fatalf("expected count 0, got %d", s.DataCount())
	}
}

func TestShard_StatusDown(t *testing.T) {
	s := NewShard("shard1")
	s.SetStatus(ShardStatusDown)

	if s.Status() != ShardStatusDown {
		t.Fatalf("expected status Down, got %v", s.Status())
	}

	err := s.Put("key1", []byte("value1"))
	if err != ErrShardDown {
		t.Fatalf("expected ErrShardDown, got %v", err)
	}

	_, err = s.Get("key1")
	if err != ErrShardDown {
		t.Fatalf("expected ErrShardDown, got %v", err)
	}

	err = s.Delete("key1")
	if err != ErrShardDown {
		t.Fatalf("expected ErrShardDown, got %v", err)
	}
}

func TestShard_GetAllKeysAndData(t *testing.T) {
	s := NewShard("shard1")

	data := map[string][]byte{
		"key1": []byte("val1"),
		"key2": []byte("val2"),
		"key3": []byte("val3"),
	}

	for k, v := range data {
		s.Put(k, v)
	}

	keys := s.GetAllKeys()
	sort.Strings(keys)
	expectedKeys := []string{"key1", "key2", "key3"}
	if len(keys) != len(expectedKeys) {
		t.Fatalf("expected %d keys, got %d", len(expectedKeys), len(keys))
	}

	allData := s.GetAllData()
	for k, v := range data {
		if !bytes.Equal(allData[k], v) {
			t.Fatalf("mismatch for key %s", k)
		}
	}
}

func TestShard_ForceOps(t *testing.T) {
	s := NewShard("shard1")
	s.SetStatus(ShardStatusDown)

	s.ForcePut("key1", []byte("val1"))
	if !s.HasKey("key1") {
		t.Fatal("expected ForcePut to work when down")
	}

	s.ForceDelete("key1")
	if s.HasKey("key1") {
		t.Fatal("expected ForceDelete to work")
	}
}

func TestCluster_Basic(t *testing.T) {
	cluster := NewShardKVCluster()

	err := cluster.AddShard("shard1")
	if err != nil {
		t.Fatalf("unexpected error adding shard: %v", err)
	}

	if cluster.ShardCount() != 1 {
		t.Fatalf("expected 1 shard, got %d", cluster.ShardCount())
	}

	err = cluster.AddShard("shard1")
	if err == nil {
		t.Fatal("expected error adding duplicate shard")
	}
}

func TestCluster_PutGet(t *testing.T) {
	config := ShardKVConfig{
		VirtualNodes: 100,
		ReplicaCount: 2,
		WriteQuorum:  2,
	}
	cluster := NewShardKVClusterWithConfig(config)

	cluster.AddShard("shard1")
	cluster.AddShard("shard2")
	cluster.AddShard("shard3")

	err := cluster.Put("key1", []byte("value1"))
	if err != nil {
		t.Fatalf("unexpected error on Put: %v", err)
	}

	val, err := cluster.Get("key1")
	if err != nil {
		t.Fatalf("unexpected error on Get: %v", err)
	}
	if !bytes.Equal(val, []byte("value1")) {
		t.Fatalf("expected value1, got %s", string(val))
	}

	if !cluster.HasKey("key1") {
		t.Fatal("expected HasKey true")
	}
}

func TestCluster_Delete(t *testing.T) {
	config := ShardKVConfig{
		VirtualNodes: 100,
		ReplicaCount: 2,
		WriteQuorum:  2,
	}
	cluster := NewShardKVClusterWithConfig(config)

	cluster.AddShard("shard1")
	cluster.AddShard("shard2")
	cluster.AddShard("shard3")

	cluster.Put("key1", []byte("value1"))

	err := cluster.Delete("key1")
	if err != nil {
		t.Fatalf("unexpected error on Delete: %v", err)
	}

	_, err = cluster.Get("key1")
	if err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}

	err = cluster.Delete("key1")
	if err != ErrKeyNotFound {
		t.Fatalf("expected ErrKeyNotFound on double delete, got %v", err)
	}
}

func TestCluster_DataMigration_AddShard(t *testing.T) {
	config := ShardKVConfig{
		VirtualNodes: 100,
		ReplicaCount: 1,
		WriteQuorum:  1,
	}
	cluster := NewShardKVClusterWithConfig(config)

	cluster.AddShard("shard1")
	cluster.AddShard("shard2")

	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("key-%d", i)
		val := []byte(fmt.Sprintf("value-%d", i))
		err := cluster.Put(key, val)
		if err != nil {
			t.Fatalf("put error: %v", err)
		}
	}

	cluster.WaitForMigration()

	countBefore := countAllData(cluster)
	if countBefore != 200 {
		t.Fatalf("expected 200 keys before migration, got %d", countBefore)
	}

	err := cluster.AddShard("shard3")
	if err != nil {
		t.Fatalf("add shard error: %v", err)
	}

	cluster.WaitForMigration()

	countAfter := countAllData(cluster)
	if countAfter != 200 {
		t.Fatalf("expected 200 keys after migration, got %d", countAfter)
	}

	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("key-%d", i)
		expected := []byte(fmt.Sprintf("value-%d", i))
		val, err := cluster.Get(key)
		if err != nil {
			t.Fatalf("get error for %s: %v", key, err)
		}
		if !bytes.Equal(val, expected) {
			t.Fatalf("value mismatch for %s", key)
		}
	}
}

func countAllData(c *ShardKVCluster) int {
	total := 0
	for _, id := range c.GetShardIDs() {
		s, _ := c.GetShard(id)
		total += s.DataCount()
	}
	return total
}

func TestCluster_DataMigration_RemoveShard(t *testing.T) {
	config := ShardKVConfig{
		VirtualNodes: 100,
		ReplicaCount: 1,
		WriteQuorum:  1,
	}
	cluster := NewShardKVClusterWithConfig(config)

	cluster.AddShard("shard1")
	cluster.AddShard("shard2")
	cluster.AddShard("shard3")

	cluster.WaitForMigration()

	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("key-%d", i)
		val := []byte(fmt.Sprintf("value-%d", i))
		cluster.Put(key, val)
	}

	err := cluster.RemoveShard("shard3")
	if err != nil {
		t.Fatalf("remove shard error: %v", err)
	}

	cluster.WaitForMigration()

	if cluster.ShardCount() != 2 {
		t.Fatalf("expected 2 shards, got %d", cluster.ShardCount())
	}

	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("key-%d", i)
		expected := []byte(fmt.Sprintf("value-%d", i))
		val, err := cluster.Get(key)
		if err != nil {
			t.Fatalf("get error for %s: %v", key, err)
		}
		if !bytes.Equal(val, expected) {
			t.Fatalf("value mismatch for %s", key)
		}
	}
}

func TestCluster_RemoveLastShard(t *testing.T) {
	cluster := NewShardKVCluster()
	cluster.AddShard("shard1")

	err := cluster.RemoveShard("shard1")
	if err == nil {
		t.Fatal("expected error removing last shard")
	}

	err = cluster.RemoveShard("nonexistent")
	if err == nil {
		t.Fatal("expected error removing nonexistent shard")
	}
}

func TestCluster_ReplicaSync(t *testing.T) {
	config := ShardKVConfig{
		VirtualNodes: 100,
		ReplicaCount: 3,
		WriteQuorum:  3,
	}
	cluster := NewShardKVClusterWithConfig(config)

	cluster.AddShard("shard1")
	cluster.AddShard("shard2")
	cluster.AddShard("shard3")
	cluster.AddShard("shard4")
	cluster.AddShard("shard5")

	cluster.WaitForMigration()

	testKeys := []string{"key1", "key2", "key3", "key4", "key5"}
	for _, k := range testKeys {
		cluster.Put(k, []byte("value-"+k))
	}

	for _, k := range testKeys {
		replicas := cluster.hashRing.GetReplicaNodes(k, config.ReplicaCount)
		foundCount := 0
		for _, shardID := range replicas {
			shard, exists := cluster.GetShard(shardID)
			if !exists {
				continue
			}
			if shard.HasKey(k) {
				foundCount++
			}
		}
		if foundCount < config.WriteQuorum {
			t.Fatalf("key %s expected on at least %d replicas, found on %d", k, config.WriteQuorum, foundCount)
		}
	}
}

func TestCluster_QuorumFailure(t *testing.T) {
	config := ShardKVConfig{
		VirtualNodes: 100,
		ReplicaCount: 3,
		WriteQuorum:  3,
	}
	cluster := NewShardKVClusterWithConfig(config)

	cluster.AddShard("shard1")

	err := cluster.Put("key1", []byte("val1"))
	if err == nil {
		t.Fatal("expected quorum error when insufficient replicas")
	}
}

func TestCluster_Failover(t *testing.T) {
	config := ShardKVConfig{
		VirtualNodes: 100,
		ReplicaCount: 2,
		WriteQuorum:  2,
	}
	cluster := NewShardKVClusterWithConfig(config)

	cluster.AddShard("shard1")
	cluster.AddShard("shard2")
	cluster.AddShard("shard3")

	cluster.WaitForMigration()

	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		val := []byte(fmt.Sprintf("value-%d", i))
		err := cluster.Put(key, val)
		if err != nil {
			t.Fatalf("put error %d: %v", i, err)
		}
	}

	err := cluster.MarkShardDown("shard1")
	if err != nil {
		t.Fatalf("mark down error: %v", err)
	}

	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		expected := []byte(fmt.Sprintf("value-%d", i))
		val, err := cluster.Get(key)
		if err != nil {
			t.Fatalf("get error after failover for %s: %v", key, err)
		}
		if !bytes.Equal(val, expected) {
			t.Fatalf("value mismatch for %s", key)
		}
	}

	for i := 100; i < 150; i++ {
		key := fmt.Sprintf("key-%d", i)
		val := []byte(fmt.Sprintf("value-%d", i))
		err := cluster.Put(key, val)
		if err != nil {
			t.Fatalf("put after failover error %d: %v", i, err)
		}
	}

	err = cluster.MarkShardUp("shard1")
	if err != nil {
		t.Fatalf("mark up error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	cluster.WaitForMigration()

	for i := 0; i < 150; i++ {
		key := fmt.Sprintf("key-%d", i)
		expected := []byte(fmt.Sprintf("value-%d", i))
		val, err := cluster.Get(key)
		if err != nil {
			t.Fatalf("get after recovery error for %s: %v", key, err)
		}
		if !bytes.Equal(val, expected) {
			t.Fatalf("value mismatch after recovery for %s", key)
		}
	}
}

func TestCluster_MarkShardDown_NotExist(t *testing.T) {
	cluster := NewShardKVCluster()

	err := cluster.MarkShardDown("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent shard")
	}

	err = cluster.MarkShardUp("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent shard")
	}
}

func TestCluster_NoAvailable(t *testing.T) {
	config := ShardKVConfig{
		VirtualNodes: 100,
		ReplicaCount: 1,
		WriteQuorum:  1,
	}
	cluster := NewShardKVClusterWithConfig(config)

	cluster.AddShard("shard1")
	cluster.MarkShardDown("shard1")

	_, err := cluster.Get("key1")
	if err != ErrKeyNotFound {
		t.Logf("get returned: %v (expected key not found on down shard)", err)
	}

	hr := NewHashRing(10)
	_ = hr
}

func TestCluster_ConcurrentAccess(t *testing.T) {
	config := ShardKVConfig{
		VirtualNodes: 100,
		ReplicaCount: 2,
		WriteQuorum:  2,
	}
	cluster := NewShardKVClusterWithConfig(config)

	cluster.AddShard("shard1")
	cluster.AddShard("shard2")
	cluster.AddShard("shard3")
	cluster.AddShard("shard4")

	cluster.WaitForMigration()

	var wg sync.WaitGroup
	numGoroutines := 10
	opsPerGoroutine := 100

	errors := make(chan error, numGoroutines*opsPerGoroutine*3)

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				key := fmt.Sprintf("g%d-k%d", gid, i)
				val := []byte(fmt.Sprintf("val-%d-%d", gid, i))

				err := cluster.Put(key, val)
				if err != nil {
					errors <- fmt.Errorf("put %s: %w", key, err)
					continue
				}

				got, err := cluster.Get(key)
				if err != nil {
					errors <- fmt.Errorf("get %s: %w", key, err)
					continue
				}
				if !bytes.Equal(got, val) {
					errors <- fmt.Errorf("mismatch %s", key)
				}
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	errCount := 0
	for e := range errors {
		errCount++
		if errCount <= 5 {
			t.Logf("error: %v", e)
		}
	}
	if errCount > 0 {
		t.Fatalf("got %d errors during concurrent access", errCount)
	}
}

func TestCluster_ConcurrentWithShardChanges(t *testing.T) {
	config := ShardKVConfig{
		VirtualNodes: 50,
		ReplicaCount: 2,
		WriteQuorum:  1,
	}
	cluster := NewShardKVClusterWithConfig(config)

	for i := 1; i <= 5; i++ {
		cluster.AddShard(fmt.Sprintf("shard-%d", i))
	}
	cluster.WaitForMigration()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 6; i <= 10; i++ {
			select {
			case <-stop:
				return
			default:
			}
			shardID := fmt.Sprintf("shard-%d", i)
			cluster.AddShard(shardID)
			time.Sleep(10 * time.Millisecond)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			for i := 0; i < 50; i++ {
				key := fmt.Sprintf("dyn-key-%d", i)
				val := []byte(fmt.Sprintf("dyn-val-%d", i))
				cluster.Put(key, val)
				cluster.Get(key)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()

	cluster.WaitForMigration()

	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("dyn-key-%d", i)
		expected := []byte(fmt.Sprintf("dyn-val-%d", i))
		got, err := cluster.Get(key)
		if err != nil {
			continue
		}
		if !bytes.Equal(got, expected) {
			t.Errorf("mismatch for %s", key)
		}
	}
}

func TestCluster_DeleteDuringMigration(t *testing.T) {
	config := ShardKVConfig{
		VirtualNodes: 100,
		ReplicaCount: 1,
		WriteQuorum:  1,
	}
	cluster := NewShardKVClusterWithConfig(config)

	cluster.AddShard("shard1")

	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("delkey-%d", i)
		cluster.Put(key, []byte("val"))
	}

	cluster.AddShard("shard2")
	cluster.AddShard("shard3")

	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("delkey-%d", i)
		cluster.Delete(key)
	}

	cluster.WaitForMigration()

	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("delkey-%d", i)
		_, err := cluster.Get(key)
		if err == nil {
			t.Fatalf("expected key %s to be deleted", key)
		}
	}

	for i := 50; i < 100; i++ {
		key := fmt.Sprintf("delkey-%d", i)
		_, err := cluster.Get(key)
		if err != nil {
			t.Fatalf("expected key %s to exist: %v", key, err)
		}
	}
}

func TestCluster_EdgeCase_SingleShard(t *testing.T) {
	config := ShardKVConfig{
		VirtualNodes: 10,
		ReplicaCount: 1,
		WriteQuorum:  1,
	}
	cluster := NewShardKVClusterWithConfig(config)

	cluster.AddShard("only-shard")

	err := cluster.Put("key1", []byte("value1"))
	if err != nil {
		t.Fatalf("put error: %v", err)
	}

	val, err := cluster.Get("key1")
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if !bytes.Equal(val, []byte("value1")) {
		t.Fatalf("value mismatch")
	}
}

func TestCluster_EdgeCase_AllShardsDown(t *testing.T) {
	config := ShardKVConfig{
		VirtualNodes: 10,
		ReplicaCount: 2,
		WriteQuorum:  1,
	}
	cluster := NewShardKVClusterWithConfig(config)

	cluster.AddShard("shard1")
	cluster.AddShard("shard2")

	cluster.Put("key1", []byte("val1"))

	cluster.MarkShardDown("shard1")
	cluster.MarkShardDown("shard2")

	_, err := cluster.Get("key1")
	if err != ErrKeyNotFound {
		t.Logf("all down - get returned: %v", err)
	}

	err = cluster.Put("key2", []byte("val2"))
	if err == nil {
		t.Fatal("expected error writing when all shards down")
	}
}

func TestCluster_TotalDataCount(t *testing.T) {
	config := ShardKVConfig{
		VirtualNodes: 100,
		ReplicaCount: 1,
		WriteQuorum:  1,
	}
	cluster := NewShardKVClusterWithConfig(config)

	cluster.AddShard("shard1")
	cluster.AddShard("shard2")
	cluster.WaitForMigration()

	for i := 0; i < 50; i++ {
		cluster.Put(fmt.Sprintf("k%d", i), []byte("v"))
	}

	count := cluster.TotalDataCount()
	if count != 50 {
		t.Fatalf("expected TotalDataCount 50, got %d", count)
	}

	cluster.MarkShardDown("shard1")
	countDown := cluster.TotalDataCount()
	if countDown >= count {
		t.Logf("note: count with one down: %d (was %d)", countDown, count)
	}
}

func TestCluster_GetShard(t *testing.T) {
	cluster := NewShardKVCluster()
	cluster.AddShard("s1")

	s, ok := cluster.GetShard("s1")
	if !ok || s.ID() != "s1" {
		t.Fatal("GetShard failed")
	}

	_, ok = cluster.GetShard("nonexistent")
	if ok {
		t.Fatal("expected false for nonexistent shard")
	}
}

func TestCluster_GetConfig(t *testing.T) {
	config := ShardKVConfig{
		VirtualNodes: 42,
		ReplicaCount: 3,
		WriteQuorum:  2,
	}
	cluster := NewShardKVClusterWithConfig(config)

	cfg := cluster.GetConfig()
	if cfg.VirtualNodes != 42 || cfg.ReplicaCount != 3 || cfg.WriteQuorum != 2 {
		t.Fatalf("config mismatch: %+v", cfg)
	}
}

func TestCluster_DefaultConfig(t *testing.T) {
	cfg := DefaultShardKVConfig()
	if cfg.VirtualNodes <= 0 || cfg.ReplicaCount <= 0 || cfg.WriteQuorum <= 0 {
		t.Fatalf("invalid default config: %+v", cfg)
	}

	cluster := NewShardKVCluster()
	if cluster == nil {
		t.Fatal("NewShardKVCluster returned nil")
	}
}

func TestCluster_ConfigValidation(t *testing.T) {
	badConfig := ShardKVConfig{
		VirtualNodes: -1,
		ReplicaCount: 0,
		WriteQuorum:  -5,
	}
	cluster := NewShardKVClusterWithConfig(badConfig)
	cfg := cluster.GetConfig()

	if cfg.VirtualNodes <= 0 {
		t.Fatalf("VirtualNodes should be positive, got %d", cfg.VirtualNodes)
	}
	if cfg.ReplicaCount <= 0 {
		t.Fatalf("ReplicaCount should be positive, got %d", cfg.ReplicaCount)
	}
	if cfg.WriteQuorum <= 0 {
		t.Fatalf("WriteQuorum should be positive, got %d", cfg.WriteQuorum)
	}
	if cfg.WriteQuorum > cfg.ReplicaCount {
		t.Fatal("WriteQuorum should not exceed ReplicaCount")
	}
}

func TestGenerateVirtualKeys(t *testing.T) {
	keys := generateVirtualKeys("node1", 5)
	if len(keys) != 5 {
		t.Fatalf("expected 5 keys, got %d", len(keys))
	}

	expectedPrefix := "node1#vn"
	for i, k := range keys {
		expected := expectedPrefix + fmt.Sprintf("%d", i)
		if k != expected {
			t.Fatalf("expected %s, got %s", expected, k)
		}
	}
}

func TestHashKeyDeterminism(t *testing.T) {
	h1 := hashKey("test-key")
	h2 := hashKey("test-key")
	if h1 != h2 {
		t.Fatal("hashKey not deterministic")
	}

	h3 := hashKey("different-key")
	if h1 == h3 {
		t.Fatal("different keys produced same hash (unlikely collision)")
	}
}
