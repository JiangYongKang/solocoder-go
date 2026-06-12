package shardkv

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"
	"sync"
)

type HashRing struct {
	mu           sync.RWMutex
	virtualNodes int
	ring         []uint64
	hashMap      map[uint64]string
	nodeSet      map[string]struct{}
}

func NewHashRing(virtualNodes int) *HashRing {
	if virtualNodes <= 0 {
		virtualNodes = 100
	}
	return &HashRing{
		virtualNodes: virtualNodes,
		hashMap:      make(map[uint64]string),
		nodeSet:      make(map[string]struct{}),
	}
}

func hashKey(key string) uint64 {
	h := sha1.New()
	h.Write([]byte(key))
	sum := h.Sum(nil)
	return binary.BigEndian.Uint64(sum[:8])
}

func (hr *HashRing) AddNode(nodeID string) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	if _, exists := hr.nodeSet[nodeID]; exists {
		return
	}
	hr.nodeSet[nodeID] = struct{}{}

	for i := 0; i < hr.virtualNodes; i++ {
		virtualKey := fmt.Sprintf("%s#vn%d", nodeID, i)
		hash := hashKey(virtualKey)
		hr.ring = append(hr.ring, hash)
		hr.hashMap[hash] = nodeID
	}

	sort.Slice(hr.ring, func(i, j int) bool {
		return hr.ring[i] < hr.ring[j]
	})
}

func (hr *HashRing) RemoveNode(nodeID string) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	if _, exists := hr.nodeSet[nodeID]; !exists {
		return
	}
	delete(hr.nodeSet, nodeID)

	for i := 0; i < hr.virtualNodes; i++ {
		virtualKey := fmt.Sprintf("%s#vn%d", nodeID, i)
		hash := hashKey(virtualKey)
		delete(hr.hashMap, hash)
	}

	newRing := make([]uint64, 0, len(hr.ring))
	for _, h := range hr.ring {
		if _, exists := hr.hashMap[h]; exists {
			newRing = append(newRing, h)
		}
	}
	hr.ring = newRing
}

func (hr *HashRing) GetNode(key string) string {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	if len(hr.ring) == 0 {
		return ""
	}

	hash := hashKey(key)
	idx := sort.Search(len(hr.ring), func(i int) bool {
		return hr.ring[i] >= hash
	})

	if idx == len(hr.ring) {
		idx = 0
	}

	return hr.hashMap[hr.ring[idx]]
}

func (hr *HashRing) GetNodes() []string {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	nodes := make([]string, 0, len(hr.nodeSet))
	for node := range hr.nodeSet {
		nodes = append(nodes, node)
	}
	return nodes
}

func (hr *HashRing) NodeCount() int {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	return len(hr.nodeSet)
}

func (hr *HashRing) GetReplicaNodes(key string, replicaCount int) []string {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	if len(hr.ring) == 0 || replicaCount <= 0 {
		return nil
	}

	hash := hashKey(key)
	idx := sort.Search(len(hr.ring), func(i int) bool {
		return hr.ring[i] >= hash
	})

	if idx == len(hr.ring) {
		idx = 0
	}

	visited := make(map[string]struct{})
	result := make([]string, 0, replicaCount)

	for i := 0; i < len(hr.ring) && len(result) < replicaCount; i++ {
		ringIdx := (idx + i) % len(hr.ring)
		nodeID := hr.hashMap[hr.ring[ringIdx]]
		if _, exists := visited[nodeID]; !exists {
			visited[nodeID] = struct{}{}
			result = append(result, nodeID)
		}
	}

	return result
}

func generateVirtualKeys(nodeID string, virtualNodes int) []string {
	keys := make([]string, virtualNodes)
	for i := 0; i < virtualNodes; i++ {
		keys[i] = nodeID + "#vn" + strconv.Itoa(i)
	}
	return keys
}
