package shardkv

import (
	"fmt"
	"sync"
	"time"
)

type ShardKVConfig struct {
	VirtualNodes int
	ReplicaCount int
	WriteQuorum  int
}

func DefaultShardKVConfig() ShardKVConfig {
	return ShardKVConfig{
		VirtualNodes: 100,
		ReplicaCount: 2,
		WriteQuorum:  2,
	}
}

type ShardKVCluster struct {
	mu          sync.RWMutex
	config      ShardKVConfig
	hashRing    *HashRing
	shards      map[string]*Shard
	downShards  map[string]struct{}
	migrating   bool
	migratingMu sync.Mutex
}

func NewShardKVCluster() *ShardKVCluster {
	return NewShardKVClusterWithConfig(DefaultShardKVConfig())
}

func NewShardKVClusterWithConfig(config ShardKVConfig) *ShardKVCluster {
	if config.VirtualNodes <= 0 {
		config.VirtualNodes = 100
	}
	if config.ReplicaCount <= 0 {
		config.ReplicaCount = 1
	}
	if config.WriteQuorum <= 0 {
		config.WriteQuorum = 1
	}
	if config.WriteQuorum > config.ReplicaCount {
		config.WriteQuorum = config.ReplicaCount
	}

	return &ShardKVCluster{
		config:     config,
		hashRing:   NewHashRing(config.VirtualNodes),
		shards:     make(map[string]*Shard),
		downShards: make(map[string]struct{}),
	}
}

func (c *ShardKVCluster) AddShard(shardID string) error {
	c.mu.Lock()
	if _, exists := c.shards[shardID]; exists {
		c.mu.Unlock()
		return fmt.Errorf("shard %s already exists", shardID)
	}

	shard := NewShard(shardID)
	c.shards[shardID] = shard
	nodeCount := c.hashRing.NodeCount()
	c.mu.Unlock()

	c.hashRing.AddNode(shardID)

	if nodeCount > 0 {
		c.migrateOnAdd(shardID)
	}

	return nil
}

func (c *ShardKVCluster) RemoveShard(shardID string) error {
	c.mu.RLock()
	shard, exists := c.shards[shardID]
	c.mu.RUnlock()

	if !exists {
		return fmt.Errorf("shard %s not found", shardID)
	}

	if c.hashRing.NodeCount() <= 1 {
		return fmt.Errorf("cannot remove the last shard")
	}

	shard.SetStatus(ShardStatusMigrating)

	c.hashRing.RemoveNode(shardID)

	c.migrateOnRemove(shardID)

	c.mu.Lock()
	delete(c.shards, shardID)
	delete(c.downShards, shardID)
	c.mu.Unlock()

	return nil
}

func (c *ShardKVCluster) migrateOnAdd(newShardID string) {
	c.migratingMu.Lock()
	c.migrating = true
	c.migratingMu.Unlock()

	defer func() {
		c.migratingMu.Lock()
		c.migrating = false
		c.migratingMu.Unlock()
	}()

	c.mu.RLock()
	allShards := make([]*Shard, 0, len(c.shards))
	for _, s := range c.shards {
		if s.ID() != newShardID {
			allShards = append(allShards, s)
		}
	}
	newShard := c.shards[newShardID]
	replicaCount := c.config.ReplicaCount
	c.mu.RUnlock()

	for _, oldShard := range allShards {
		if oldShard.Status() == ShardStatusDown {
			continue
		}

		keys := oldShard.GetAllKeys()
		for _, key := range keys {
			replicaNodes := c.hashRing.GetReplicaNodes(key, replicaCount)

			newShardIsReplica := false
			for _, n := range replicaNodes {
				if n == newShardID {
					newShardIsReplica = true
					break
				}
			}
			if !newShardIsReplica {
				continue
			}

			val, err := oldShard.Get(key)
			if err != nil {
				continue
			}

			newShard.ForcePut(key, val)

			oldShardStillReplica := false
			for _, n := range replicaNodes {
				if n == oldShard.ID() {
					oldShardStillReplica = true
					break
				}
			}
			if !oldShardStillReplica {
				oldShard.ForceDelete(key)
			}
		}
	}
}

func (c *ShardKVCluster) migrateOnRemove(removedShardID string) {
	c.migratingMu.Lock()
	c.migrating = true
	c.migratingMu.Unlock()

	defer func() {
		c.migratingMu.Lock()
		c.migrating = false
		c.migratingMu.Unlock()
	}()

	c.mu.RLock()
	removedShard := c.shards[removedShardID]
	replicaCount := c.config.ReplicaCount
	remainingShardIDs := make([]string, 0, len(c.shards))
	for id := range c.shards {
		if id != removedShardID {
			remainingShardIDs = append(remainingShardIDs, id)
		}
	}
	c.mu.RUnlock()

	keys := removedShard.GetAllKeys()
	for _, key := range keys {
		val, err := removedShard.Get(key)
		if err != nil {
			continue
		}

		newRingReplicas := c.hashRing.GetReplicaNodes(key, replicaCount)
		newRingReplicaSet := make(map[string]struct{}, len(newRingReplicas))
		for _, n := range newRingReplicas {
			newRingReplicaSet[n] = struct{}{}
		}

		existingReplicas := 0
		c.mu.RLock()
		for _, sid := range remainingShardIDs {
			if _, isLegal := newRingReplicaSet[sid]; !isLegal {
				continue
			}
			s, exists := c.shards[sid]
			if !exists || s.Status() == ShardStatusDown {
				continue
			}
			if s.HasKey(key) {
				existingReplicas++
			}
		}
		c.mu.RUnlock()

		needed := replicaCount - existingReplicas
		if needed <= 0 {
			continue
		}

		for _, tid := range newRingReplicas {
			if needed <= 0 {
				break
			}
			c.mu.RLock()
			targetShard, exists := c.shards[tid]
			c.mu.RUnlock()
			if !exists || targetShard.Status() == ShardStatusDown {
				continue
			}
			if targetShard.HasKey(key) {
				continue
			}

			targetShard.ForcePut(key, val)
			needed--
		}

		if needed > 0 {
			for _, sid := range remainingShardIDs {
				if needed <= 0 {
					break
				}
				if _, isLegal := newRingReplicaSet[sid]; isLegal {
					continue
				}
				c.mu.RLock()
				targetShard, exists := c.shards[sid]
				c.mu.RUnlock()
				if !exists || targetShard.Status() == ShardStatusDown {
					continue
				}
				if targetShard.HasKey(key) {
					continue
				}

				targetShard.ForcePut(key, val)
				needed--
			}
		}
	}
}

func (c *ShardKVCluster) MarkShardDown(shardID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	shard, exists := c.shards[shardID]
	if !exists {
		return fmt.Errorf("shard %s not found", shardID)
	}

	shard.SetStatus(ShardStatusDown)
	c.downShards[shardID] = struct{}{}

	return nil
}

func (c *ShardKVCluster) MarkShardUp(shardID string) error {
	c.mu.RLock()
	shard, exists := c.shards[shardID]
	c.mu.RUnlock()

	if !exists {
		return fmt.Errorf("shard %s not found", shardID)
	}

	c.syncFromReplicas(shardID)

	c.mu.Lock()
	shard.SetStatus(ShardStatusUp)
	delete(c.downShards, shardID)
	c.mu.Unlock()

	return nil
}

func (c *ShardKVCluster) syncFromReplicas(recoveredShardID string) {
	c.mu.RLock()
	recoveredShard := c.shards[recoveredShardID]
	allShardIDs := make([]string, 0, len(c.shards))
	for id := range c.shards {
		if id != recoveredShardID {
			allShardIDs = append(allShardIDs, id)
		}
	}
	replicaCount := c.config.ReplicaCount
	c.mu.RUnlock()

	if recoveredShard == nil {
		return
	}

	for _, otherID := range allShardIDs {
		c.mu.RLock()
		otherShard := c.shards[otherID]
		c.mu.RUnlock()

		if otherShard == nil || otherShard.Status() == ShardStatusDown {
			continue
		}

		allData := otherShard.GetAllData()
		for key, val := range allData {
			replicaNodes := c.hashRing.GetReplicaNodes(key, replicaCount)
			isOwner := false
			for _, n := range replicaNodes {
				if n == recoveredShardID {
					isOwner = true
					break
				}
			}
			if isOwner {
				recoveredShard.ForcePut(key, val)
			}
		}
	}
}

func (c *ShardKVCluster) getAvailableReplicaNodes(key string) []string {
	baseNodes := c.hashRing.GetReplicaNodes(key, c.config.ReplicaCount)

	available := make([]string, 0, len(baseNodes))
	c.mu.RLock()
	for _, nid := range baseNodes {
		_, exists := c.shards[nid]
		_, isDown := c.downShards[nid]
		if exists && !isDown {
			available = append(available, nid)
		}
	}

	if len(available) >= c.config.WriteQuorum {
		c.mu.RUnlock()
		return available
	}

	needed := c.config.WriteQuorum - len(available)
	if needed > 0 {
		extraNodes := c.hashRing.GetReplicaNodes(key, c.config.ReplicaCount+10)
		baseSet := make(map[string]struct{})
		for _, n := range baseNodes {
			baseSet[n] = struct{}{}
		}

		for _, nid := range extraNodes {
			if needed <= 0 {
				break
			}
			if _, isBase := baseSet[nid]; isBase {
				continue
			}
			_, exists := c.shards[nid]
			_, isDown := c.downShards[nid]
			if exists && !isDown {
				available = append(available, nid)
				needed--
			}
		}
	}
	c.mu.RUnlock()

	return available
}

func (c *ShardKVCluster) Get(key string) ([]byte, error) {
	replicaNodes := c.hashRing.GetReplicaNodes(key, c.config.ReplicaCount+5)
	if len(replicaNodes) == 0 {
		return nil, ErrNoAvailable
	}

	var lastErr error
	for _, nodeID := range replicaNodes {
		c.mu.RLock()
		shard, exists := c.shards[nodeID]
		_, isDown := c.downShards[nodeID]
		c.mu.RUnlock()

		if !exists || isDown {
			continue
		}

		if shard.Status() == ShardStatusDown {
			continue
		}

		val, err := shard.Get(key)
		if err == nil {
			return val, nil
		}
		if err != ErrKeyNotFound {
			lastErr = err
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ErrKeyNotFound
}

func (c *ShardKVCluster) Put(key string, value []byte) error {
	writeNodes := c.getAvailableReplicaNodes(key)
	if len(writeNodes) == 0 {
		return ErrNoAvailable
	}

	successCount := 0
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for _, nodeID := range writeNodes {
		wg.Add(1)
		go func(nid string) {
			defer wg.Done()

			c.mu.RLock()
			shard, exists := c.shards[nid]
			_, isDown := c.downShards[nid]
			c.mu.RUnlock()

			if !exists || isDown {
				return
			}

			if shard.Status() == ShardStatusDown {
				return
			}

			err := shard.Put(key, value)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			} else {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(nodeID)
	}

	wg.Wait()

	if successCount >= c.config.WriteQuorum {
		return nil
	}

	if firstErr != nil {
		return firstErr
	}
	return ErrQuorumFailed
}

func (c *ShardKVCluster) Delete(key string) error {
	writeNodes := c.getAvailableReplicaNodes(key)
	if len(writeNodes) == 0 {
		return ErrNoAvailable
	}

	successCount := 0
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	notFoundCount := 0

	for _, nodeID := range writeNodes {
		wg.Add(1)
		go func(nid string) {
			defer wg.Done()

			c.mu.RLock()
			shard, exists := c.shards[nid]
			_, isDown := c.downShards[nid]
			c.mu.RUnlock()

			if !exists || isDown {
				return
			}

			if shard.Status() == ShardStatusDown {
				return
			}

			err := shard.Delete(key)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			} else if err == ErrKeyNotFound {
				mu.Lock()
				notFoundCount++
				mu.Unlock()
			} else {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(nodeID)
	}

	wg.Wait()

	if successCount >= c.config.WriteQuorum {
		return nil
	}

	if successCount+notFoundCount >= c.config.WriteQuorum && notFoundCount > 0 && successCount == 0 {
		return ErrKeyNotFound
	}

	if successCount > 0 {
		return nil
	}

	if firstErr != nil {
		return firstErr
	}
	return ErrQuorumFailed
}

func (c *ShardKVCluster) HasKey(key string) bool {
	replicaNodes := c.hashRing.GetReplicaNodes(key, c.config.ReplicaCount+5)
	if len(replicaNodes) == 0 {
		return false
	}

	for _, nodeID := range replicaNodes {
		c.mu.RLock()
		shard, exists := c.shards[nodeID]
		_, isDown := c.downShards[nodeID]
		c.mu.RUnlock()

		if !exists || isDown {
			continue
		}

		if shard.Status() == ShardStatusDown {
			continue
		}

		if shard.HasKey(key) {
			return true
		}
	}

	return false
}

func (c *ShardKVCluster) ShardCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.shards)
}

func (c *ShardKVCluster) GetShardIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ids := make([]string, 0, len(c.shards))
	for id := range c.shards {
		ids = append(ids, id)
	}
	return ids
}

func (c *ShardKVCluster) GetShard(shardID string) (*Shard, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	shard, exists := c.shards[shardID]
	return shard, exists
}

func (c *ShardKVCluster) TotalDataCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	for _, shard := range c.shards {
		if _, isDown := c.downShards[shard.ID()]; isDown {
			continue
		}
		count += shard.DataCount()
	}
	return count
}

func (c *ShardKVCluster) GetConfig() ShardKVConfig {
	return c.config
}

func (c *ShardKVCluster) IsMigrating() bool {
	c.migratingMu.Lock()
	defer c.migratingMu.Unlock()
	return c.migrating
}

func (c *ShardKVCluster) WaitForMigration() {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c.migratingMu.Lock()
		migrating := c.migrating
		c.migratingMu.Unlock()
		if !migrating {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}
