package chainhash

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

const (
	CurrentVersion = 1
)

func (hr *HashRing) Snapshot() *RingSnapshot {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	nodes := make([]NodeInfo, 0, len(hr.nodes))
	for _, node := range hr.nodes {
		nodes = append(nodes, *node)
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	vnodes := make([]VirtualNode, 0, len(hr.vnodeMap))
	for _, node := range hr.nodes {
		vnCount := hr.calculateVirtualNodeCount(node.Weight)
		for i := 0; i < vnCount; i++ {
			virtualKey := generateVirtualKey(node.ID, i)
			hash := hashKey(virtualKey)
			vnodes = append(vnodes, VirtualNode{
				Hash:   hash,
				NodeID: node.ID,
				Index:  i,
			})
		}
	}

	sort.Slice(vnodes, func(i, j int) bool {
		return vnodes[i].Hash < vnodes[j].Hash
	})

	metadata := make(map[string]interface{})
	for k, v := range hr.metadata {
		metadata[k] = v
	}

	return &RingSnapshot{
		Version:      CurrentVersion,
		VirtualNodes: hr.virtualNodes,
		Nodes:        nodes,
		VNodes:       vnodes,
		TotalKeys:    hr.totalKeys,
		Metadata:     metadata,
	}
}

func (hr *HashRing) Restore(snapshot *RingSnapshot) error {
	if snapshot == nil {
		return ErrDeserializationFailed
	}
	if snapshot.Version < 1 {
		return fmt.Errorf("%w: unsupported version %d", ErrDeserializationFailed, snapshot.Version)
	}
	if snapshot.VirtualNodes <= 0 {
		return ErrInvalidVirtualNodes
	}

	hr.mu.Lock()
	defer hr.mu.Unlock()

	hr.virtualNodes = snapshot.VirtualNodes
	hr.totalKeys = snapshot.TotalKeys

	hr.nodes = make(map[string]*NodeInfo)
	for i := range snapshot.Nodes {
		node := snapshot.Nodes[i]
		if node.ID == "" {
			return fmt.Errorf("%w: empty node id in snapshot", ErrDeserializationFailed)
		}
		if node.Weight <= 0 {
			return fmt.Errorf("%w: invalid weight %d for node %s", ErrDeserializationFailed, node.Weight, node.ID)
		}
		hr.nodes[node.ID] = &node
	}

	hr.vnodeMap = make(map[uint64]string)
	hr.ring = make([]uint64, 0, len(snapshot.VNodes))
	for _, vn := range snapshot.VNodes {
		if _, exists := hr.nodes[vn.NodeID]; !exists {
			return fmt.Errorf("%w: virtual node references unknown node %s", ErrDeserializationFailed, vn.NodeID)
		}
		hr.vnodeMap[vn.Hash] = vn.NodeID
		hr.ring = append(hr.ring, vn.Hash)
	}

	sort.Slice(hr.ring, func(i, j int) bool {
		return hr.ring[i] < hr.ring[j]
	})

	hr.metadata = make(map[string]interface{})
	if snapshot.Metadata != nil {
		for k, v := range snapshot.Metadata {
			hr.metadata[k] = v
		}
	}

	return nil
}

func (hr *HashRing) MarshalJSON() ([]byte, error) {
	snapshot := hr.Snapshot()
	data, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSerializationFailed, err)
	}
	return data, nil
}

func (hr *HashRing) UnmarshalJSON(data []byte) error {
	var snapshot RingSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("%w: %v", ErrDeserializationFailed, err)
	}
	return hr.Restore(&snapshot)
}

func (hr *HashRing) SaveToFile(path string) error {
	data, err := hr.MarshalJSON()
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("%w: %v", ErrFileIO, err)
	}

	return nil
}

func LoadFromFile(path string) (*HashRing, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFileIO, err)
	}

	hr, err := NewHashRing(DefaultVirtualNodes)
	if err != nil {
		return nil, err
	}

	if err := hr.UnmarshalJSON(data); err != nil {
		return nil, err
	}

	return hr, nil
}
