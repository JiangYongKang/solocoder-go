package chainhash

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"sort"
)

func hashKey(key string) uint64 {
	h := sha1.New()
	h.Write([]byte(key))
	sum := h.Sum(nil)
	return binary.BigEndian.Uint64(sum[:8])
}

func generateVirtualKey(nodeID string, index int) string {
	return fmt.Sprintf("%s#vn%d", nodeID, index)
}

func NewHashRing(virtualNodes int) (*HashRing, error) {
	if virtualNodes <= 0 {
		return nil, ErrInvalidVirtualNodes
	}
	return &HashRing{
		virtualNodes: virtualNodes,
		nodes:        make(map[string]*NodeInfo),
		vnodeMap:     make(map[uint64]string),
		ring:         make([]uint64, 0),
		metadata:     make(map[string]interface{}),
	}, nil
}

func (hr *HashRing) calculateVirtualNodeCount(weight int) int {
	if weight <= 0 {
		weight = DefaultWeight
	}
	return hr.virtualNodes * weight
}

func (hr *HashRing) rebuildRing() {
	hr.ring = hr.ring[:0]
	for hash := range hr.vnodeMap {
		hr.ring = append(hr.ring, hash)
	}
	sort.Slice(hr.ring, func(i, j int) bool {
		return hr.ring[i] < hr.ring[j]
	})
}

func (hr *HashRing) AddNode(nodeID string, weight int) error {
	return hr.AddNodeWithInfo(NodeInfo{
		ID:     nodeID,
		Weight: weight,
	})
}

func (hr *HashRing) AddNodeWithInfo(info NodeInfo) error {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	if info.ID == "" {
		return ErrEmptyNodeID
	}
	if info.Weight <= 0 {
		return ErrInvalidWeight
	}
	if _, exists := hr.nodes[info.ID]; exists {
		return ErrNodeAlreadyExists
	}

	hr.nodes[info.ID] = &info

	vnCount := hr.calculateVirtualNodeCount(info.Weight)
	for i := 0; i < vnCount; i++ {
		virtualKey := generateVirtualKey(info.ID, i)
		hash := hashKey(virtualKey)
		hr.vnodeMap[hash] = info.ID
		hr.ring = append(hr.ring, hash)
	}

	sort.Slice(hr.ring, func(i, j int) bool {
		return hr.ring[i] < hr.ring[j]
	})

	return nil
}

func (hr *HashRing) RemoveNode(nodeID string) ([]MigrationInfo, error) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	if nodeID == "" {
		return nil, ErrEmptyNodeID
	}

	node, exists := hr.nodes[nodeID]
	if !exists {
		return nil, ErrNodeNotFound
	}

	migrationInfos := hr.calculateRemoveMigration(nodeID)

	vnCount := hr.calculateVirtualNodeCount(node.Weight)
	for i := 0; i < vnCount; i++ {
		virtualKey := generateVirtualKey(nodeID, i)
		hash := hashKey(virtualKey)
		delete(hr.vnodeMap, hash)
	}

	delete(hr.nodes, nodeID)
	hr.rebuildRing()

	return migrationInfos, nil
}

func (hr *HashRing) UpdateNodeWeight(nodeID string, newWeight int) ([]MigrationInfo, error) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	if nodeID == "" {
		return nil, ErrEmptyNodeID
	}
	if newWeight <= 0 {
		return nil, ErrInvalidWeight
	}

	node, exists := hr.nodes[nodeID]
	if !exists {
		return nil, ErrNodeNotFound
	}

	oldWeight := node.Weight
	if oldWeight == newWeight {
		return nil, nil
	}

	migrationInfos := hr.calculateWeightMigration(nodeID, oldWeight, newWeight)

	oldVnCount := hr.calculateVirtualNodeCount(oldWeight)
	for i := 0; i < oldVnCount; i++ {
		virtualKey := generateVirtualKey(nodeID, i)
		hash := hashKey(virtualKey)
		delete(hr.vnodeMap, hash)
	}

	node.Weight = newWeight
	newVnCount := hr.calculateVirtualNodeCount(newWeight)
	for i := 0; i < newVnCount; i++ {
		virtualKey := generateVirtualKey(nodeID, i)
		hash := hashKey(virtualKey)
		hr.vnodeMap[hash] = nodeID
	}

	hr.rebuildRing()

	return migrationInfos, nil
}

func (hr *HashRing) GetNode(key string) (string, error) {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	if len(hr.ring) == 0 {
		return "", ErrEmptyRing
	}

	hash := hashKey(key)
	idx := sort.Search(len(hr.ring), func(i int) bool {
		return hr.ring[i] >= hash
	})

	if idx == len(hr.ring) {
		idx = 0
	}

	return hr.vnodeMap[hr.ring[idx]], nil
}

func (hr *HashRing) GetNodes(key string, n int) ([]string, error) {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	if len(hr.ring) == 0 {
		return nil, ErrEmptyRing
	}
	if n <= 0 {
		return nil, nil
	}
	if n > len(hr.nodes) {
		n = len(hr.nodes)
	}

	hash := hashKey(key)
	idx := sort.Search(len(hr.ring), func(i int) bool {
		return hr.ring[i] >= hash
	})

	if idx == len(hr.ring) {
		idx = 0
	}

	visited := make(map[string]struct{})
	result := make([]string, 0, n)

	for i := 0; i < len(hr.ring) && len(result) < n; i++ {
		ringIdx := (idx + i) % len(hr.ring)
		nodeID := hr.vnodeMap[hr.ring[ringIdx]]
		if _, exists := visited[nodeID]; !exists {
			visited[nodeID] = struct{}{}
			result = append(result, nodeID)
		}
	}

	return result, nil
}

func (hr *HashRing) NodeExists(nodeID string) bool {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	_, exists := hr.nodes[nodeID]
	return exists
}

func (hr *HashRing) GetNodeInfo(nodeID string) (*NodeInfo, error) {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	node, exists := hr.nodes[nodeID]
	if !exists {
		return nil, ErrNodeNotFound
	}

	info := *node
	return &info, nil
}

func (hr *HashRing) GetAllNodes() []NodeInfo {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	nodes := make([]NodeInfo, 0, len(hr.nodes))
	for _, node := range hr.nodes {
		nodes = append(nodes, *node)
	}
	return nodes
}

func (hr *HashRing) NodeCount() int {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	return len(hr.nodes)
}

func (hr *HashRing) VirtualNodeCount() int {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	return len(hr.vnodeMap)
}

func (hr *HashRing) SetTotalKeys(count int64) {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	hr.totalKeys = count
}

func (hr *HashRing) GetTotalKeys() int64 {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	return hr.totalKeys
}

func (hr *HashRing) GetMetadata(key string) (interface{}, bool) {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	val, exists := hr.metadata[key]
	return val, exists
}

func (hr *HashRing) SetMetadata(key string, value interface{}) {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	hr.metadata[key] = value
}

func (hr *HashRing) calculateRemoveMigration(nodeID string) []MigrationInfo {
	if len(hr.nodes) <= 1 {
		return nil
	}

	vnCount := hr.calculateVirtualNodeCount(hr.nodes[nodeID].Weight)
	affectedRanges := make([]HashRange, 0)

	for i := 0; i < vnCount; i++ {
		virtualKey := generateVirtualKey(nodeID, i)
		hash := hashKey(virtualKey)

		prevIdx := sort.Search(len(hr.ring), func(j int) bool {
			return hr.ring[j] >= hash
		})

		var start uint64
		if prevIdx == 0 {
			start = hr.ring[len(hr.ring)-1] + 1
		} else {
			start = hr.ring[prevIdx-1] + 1
		}

		affectedRanges = append(affectedRanges, HashRange{
			Start: start,
			End:   hash,
		})
	}

	affectedRanges = mergeRanges(affectedRanges)

	migrationMap := make(map[string][]HashRange)
	for _, r := range affectedRanges {
		tempHash := r.End
		idx := sort.Search(len(hr.ring), func(j int) bool {
			return hr.ring[j] >= tempHash
		})
		if idx == len(hr.ring) {
			idx = 0
		}

		nextIdx := (idx + 1) % len(hr.ring)
		for nextIdx != idx {
			nextNode := hr.vnodeMap[hr.ring[nextIdx]]
			if nextNode != nodeID {
				migrationMap[nextNode] = append(migrationMap[nextNode], r)
				break
			}
			nextIdx = (nextIdx + 1) % len(hr.ring)
		}
	}

	result := make([]MigrationInfo, 0, len(migrationMap))
	totalAffected := int64(0)
	for toNode, ranges := range migrationMap {
		mergedRanges := mergeRanges(ranges)
		estimatedCount := hr.estimateAffectedKeys(mergedRanges)
		totalAffected += estimatedCount
		result = append(result, MigrationInfo{
			AffectedRanges: mergedRanges,
			FromNode:       nodeID,
			ToNode:         toNode,
			EstimatedCount: estimatedCount,
			TotalKeys:      hr.totalKeys,
			MigrationRatio: float64(estimatedCount) / float64(maxInt64(hr.totalKeys, 1)),
		})
	}

	return result
}

func (hr *HashRing) calculateWeightMigration(nodeID string, oldWeight, newWeight int) []MigrationInfo {
	oldVnCount := hr.calculateVirtualNodeCount(oldWeight)
	newVnCount := hr.calculateVirtualNodeCount(newWeight)

	if oldVnCount == newVnCount {
		return nil
	}

	oldHashes := make(map[uint64]struct{})
	for i := 0; i < oldVnCount; i++ {
		virtualKey := generateVirtualKey(nodeID, i)
		hash := hashKey(virtualKey)
		oldHashes[hash] = struct{}{}
	}

	newHashes := make(map[uint64]struct{})
	for i := 0; i < newVnCount; i++ {
		virtualKey := generateVirtualKey(nodeID, i)
		hash := hashKey(virtualKey)
		newHashes[hash] = struct{}{}
	}

	removedRanges := make([]HashRange, 0)
	for hash := range oldHashes {
		if _, exists := newHashes[hash]; !exists {
			prevIdx := sort.Search(len(hr.ring), func(j int) bool {
				return hr.ring[j] >= hash
			})

			var start uint64
			if prevIdx == 0 {
				start = hr.ring[len(hr.ring)-1] + 1
			} else {
				start = hr.ring[prevIdx-1] + 1
			}

			removedRanges = append(removedRanges, HashRange{
				Start: start,
				End:   hash,
			})
		}
	}

	addedRanges := make([]HashRange, 0)
	for hash := range newHashes {
		if _, exists := oldHashes[hash]; !exists {
			prevIdx := sort.Search(len(hr.ring), func(j int) bool {
				return hr.ring[j] >= hash
			})

			var start uint64
			if prevIdx == 0 {
				start = hr.ring[len(hr.ring)-1] + 1
			} else {
				start = hr.ring[prevIdx-1] + 1
			}

			addedRanges = append(addedRanges, HashRange{
				Start: start,
				End:   hash,
			})
		}
	}

	result := make([]MigrationInfo, 0)

	removedRanges = mergeRanges(removedRanges)
	if len(removedRanges) > 0 {
		migrationMap := make(map[string][]HashRange)
		for _, r := range removedRanges {
			tempHash := r.End
			idx := sort.Search(len(hr.ring), func(j int) bool {
				return hr.ring[j] >= tempHash
			})
			if idx == len(hr.ring) {
				idx = 0
			}

			nextIdx := (idx + 1) % len(hr.ring)
			for nextIdx != idx {
				nextNode := hr.vnodeMap[hr.ring[nextIdx]]
				if nextNode != nodeID {
					migrationMap[nextNode] = append(migrationMap[nextNode], r)
					break
				}
				nextIdx = (nextIdx + 1) % len(hr.ring)
			}
		}

		for toNode, ranges := range migrationMap {
			mergedRanges := mergeRanges(ranges)
			estimatedCount := hr.estimateAffectedKeys(mergedRanges)
			result = append(result, MigrationInfo{
				AffectedRanges: mergedRanges,
				FromNode:       nodeID,
				ToNode:         toNode,
				EstimatedCount: estimatedCount,
				TotalKeys:      hr.totalKeys,
				MigrationRatio: float64(estimatedCount) / float64(maxInt64(hr.totalKeys, 1)),
			})
		}
	}

	addedRanges = mergeRanges(addedRanges)
	if len(addedRanges) > 0 {
		estimatedCount := hr.estimateAffectedKeys(addedRanges)
		result = append(result, MigrationInfo{
			AffectedRanges: addedRanges,
			FromNode:       "",
			ToNode:         nodeID,
			EstimatedCount: estimatedCount,
			TotalKeys:      hr.totalKeys,
			MigrationRatio: float64(estimatedCount) / float64(maxInt64(hr.totalKeys, 1)),
		})
	}

	return result
}

func (hr *HashRing) CalculateAddMigration(nodeID string, weight int) ([]MigrationInfo, error) {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	if nodeID == "" {
		return nil, ErrEmptyNodeID
	}
	if weight <= 0 {
		return nil, ErrInvalidWeight
	}
	if _, exists := hr.nodes[nodeID]; exists {
		return nil, ErrNodeAlreadyExists
	}

	vnCount := hr.calculateVirtualNodeCount(weight)
	newHashes := make([]uint64, 0, vnCount)
	for i := 0; i < vnCount; i++ {
		virtualKey := generateVirtualKey(nodeID, i)
		hash := hashKey(virtualKey)
		newHashes = append(newHashes, hash)
	}

	sort.Slice(newHashes, func(i, j int) bool {
		return newHashes[i] < newHashes[j]
	})

	allHashes := make([]uint64, 0, len(hr.ring)+len(newHashes))
	allHashes = append(allHashes, hr.ring...)
	allHashes = append(allHashes, newHashes...)
	sort.Slice(allHashes, func(i, j int) bool {
		return allHashes[i] < allHashes[j]
	})

	affectedRanges := make([]HashRange, 0)
	for _, hash := range newHashes {
		idx := sort.Search(len(allHashes), func(j int) bool {
			return allHashes[j] >= hash
		})

		var start uint64
		if idx == 0 {
			start = allHashes[len(allHashes)-1] + 1
		} else {
			start = allHashes[idx-1] + 1
		}

		affectedRanges = append(affectedRanges, HashRange{
			Start: start,
			End:   hash,
		})
	}

	affectedRanges = mergeRanges(affectedRanges)

	migrationMap := make(map[string][]HashRange)
	for _, r := range affectedRanges {
		origIdx := sort.Search(len(hr.ring), func(j int) bool {
			return hr.ring[j] >= r.End
		})
		if origIdx == len(hr.ring) {
			origIdx = 0
		}
		fromNode := hr.vnodeMap[hr.ring[origIdx]]
		migrationMap[fromNode] = append(migrationMap[fromNode], r)
	}

	result := make([]MigrationInfo, 0, len(migrationMap))
	for fromNode, ranges := range migrationMap {
		mergedRanges := mergeRanges(ranges)
		estimatedCount := hr.estimateAffectedKeys(mergedRanges)
		result = append(result, MigrationInfo{
			AffectedRanges: mergedRanges,
			FromNode:       fromNode,
			ToNode:         nodeID,
			EstimatedCount: estimatedCount,
			TotalKeys:      hr.totalKeys,
			MigrationRatio: float64(estimatedCount) / float64(maxInt64(hr.totalKeys, 1)),
		})
	}

	return result, nil
}

func (hr *HashRing) estimateAffectedKeys(ranges []HashRange) int64 {
	if hr.totalKeys <= 0 {
		return 0
	}

	totalRange := uint64(0)
	for _, r := range ranges {
		if r.End >= r.Start {
			totalRange += r.End - r.Start + 1
		} else {
			totalRange += (MaxHashValue - r.Start + 1) + r.End
		}
	}

	ratio := float64(totalRange) / float64(MaxHashValue)
	return int64(float64(hr.totalKeys) * ratio)
}

func mergeRanges(ranges []HashRange) []HashRange {
	if len(ranges) <= 1 {
		return ranges
	}

	sort.Slice(ranges, func(i, j int) bool {
		return ranges[i].Start < ranges[j].Start
	})

	merged := make([]HashRange, 0)
	current := ranges[0]

	for i := 1; i < len(ranges); i++ {
		if ranges[i].Start <= current.End+1 {
			if ranges[i].End > current.End {
				current.End = ranges[i].End
			}
		} else {
			merged = append(merged, current)
			current = ranges[i]
		}
	}

	merged = append(merged, current)
	return merged
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
