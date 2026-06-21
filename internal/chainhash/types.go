package chainhash

import "sync"

const (
	DefaultVirtualNodes = 100
	DefaultWeight       = 1
	MaxHashValue uint64 = 1<<64 - 1
)

type NodeInfo struct {
	ID     string `serialize:"id"`
	Weight int    `serialize:"weight"`
	Addr   string `serialize:"addr,omitempty"`
	Data   map[string]interface{} `serialize:"data,omitempty"`
}

type VirtualNode struct {
	Hash     uint64 `serialize:"hash"`
	NodeID   string `serialize:"node_id"`
	Index    int    `serialize:"index"`
}

type HashRange struct {
	Start uint64 `serialize:"start"`
	End   uint64 `serialize:"end"`
}

type MigrationInfo struct {
	AffectedRanges []HashRange       `serialize:"affected_ranges"`
	FromNode       string            `serialize:"from_node"`
	ToNode         string            `serialize:"to_node"`
	EstimatedCount int64             `serialize:"estimated_count"`
	TotalKeys      int64             `serialize:"total_keys"`
	MigrationRatio float64           `serialize:"migration_ratio"`
}

type RingSnapshot struct {
	Version      int                    `serialize:"version"`
	VirtualNodes int                    `serialize:"virtual_nodes"`
	Nodes        []NodeInfo             `serialize:"nodes"`
	VNodes       []VirtualNode          `serialize:"vnodes"`
	TotalKeys    int64                  `serialize:"total_keys,omitempty"`
	Metadata     map[string]interface{} `serialize:"metadata,omitempty"`
}

type HashRing struct {
	mu           sync.RWMutex
	virtualNodes int
	nodes        map[string]*NodeInfo
	vnodeMap     map[uint64]string
	ring         []uint64
	totalKeys    int64
	metadata     map[string]interface{}
}
