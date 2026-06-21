package chainhash

import (
	"fmt"
	"os"
	"sort"
	"sync"
	"testing"
)

func TestNewHashRing(t *testing.T) {
	tests := []struct {
		name         string
		virtualNodes int
		expectErr    bool
	}{
		{"valid virtual nodes", 100, false},
		{"zero virtual nodes", 0, true},
		{"negative virtual nodes", -10, true},
		{"one virtual node", 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hr, err := NewHashRing(tt.virtualNodes)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if err != ErrInvalidVirtualNodes {
					t.Errorf("expected ErrInvalidVirtualNodes, got %v", err)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if hr == nil {
				t.Errorf("expected HashRing, got nil")
				return
			}
			if hr.virtualNodes != tt.virtualNodes {
				t.Errorf("expected virtualNodes %d, got %d", tt.virtualNodes, hr.virtualNodes)
			}
			if hr.NodeCount() != 0 {
				t.Errorf("expected 0 nodes, got %d", hr.NodeCount())
			}
		})
	}
}

func TestAddNode(t *testing.T) {
	hr, _ := NewHashRing(10)

	t.Run("add valid node", func(t *testing.T) {
		err := hr.AddNode("node1", 1)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !hr.NodeExists("node1") {
			t.Errorf("node1 should exist")
		}
		if hr.NodeCount() != 1 {
			t.Errorf("expected 1 node, got %d", hr.NodeCount())
		}
		expectedVN := 10 * 1
		if hr.VirtualNodeCount() != expectedVN {
			t.Errorf("expected %d virtual nodes, got %d", expectedVN, hr.VirtualNodeCount())
		}
	})

	t.Run("add duplicate node", func(t *testing.T) {
		err := hr.AddNode("node1", 1)
		if err != ErrNodeAlreadyExists {
			t.Errorf("expected ErrNodeAlreadyExists, got %v", err)
		}
	})

	t.Run("add node with empty id", func(t *testing.T) {
		err := hr.AddNode("", 1)
		if err != ErrEmptyNodeID {
			t.Errorf("expected ErrEmptyNodeID, got %v", err)
		}
	})

	t.Run("add node with invalid weight", func(t *testing.T) {
		err := hr.AddNode("node2", 0)
		if err != ErrInvalidWeight {
			t.Errorf("expected ErrInvalidWeight, got %v", err)
		}
		err = hr.AddNode("node2", -5)
		if err != ErrInvalidWeight {
			t.Errorf("expected ErrInvalidWeight, got %v", err)
		}
	})

	t.Run("add node with higher weight", func(t *testing.T) {
		err := hr.AddNode("node2", 3)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		expectedVN := 10*1 + 10*3
		if hr.VirtualNodeCount() != expectedVN {
			t.Errorf("expected %d virtual nodes, got %d", expectedVN, hr.VirtualNodeCount())
		}
	})

	t.Run("add node with info", func(t *testing.T) {
		info := NodeInfo{
			ID:     "node3",
			Weight: 2,
			Addr:   "127.0.0.1:8080",
			Data:   map[string]interface{}{"rack": "rack1"},
		}
		err := hr.AddNodeWithInfo(info)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		nodeInfo, err := hr.GetNodeInfo("node3")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if nodeInfo.ID != "node3" {
			t.Errorf("expected ID node3, got %s", nodeInfo.ID)
		}
		if nodeInfo.Weight != 2 {
			t.Errorf("expected weight 2, got %d", nodeInfo.Weight)
		}
		if nodeInfo.Addr != "127.0.0.1:8080" {
			t.Errorf("expected addr 127.0.0.1:8080, got %s", nodeInfo.Addr)
		}
		if nodeInfo.Data["rack"] != "rack1" {
			t.Errorf("expected rack1, got %v", nodeInfo.Data["rack"])
		}
	})
}

func TestRemoveNode(t *testing.T) {
	hr, _ := NewHashRing(10)
	hr.AddNode("node1", 1)
	hr.AddNode("node2", 2)

	t.Run("remove existing node", func(t *testing.T) {
		migrations, err := hr.RemoveNode("node1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if hr.NodeExists("node1") {
			t.Errorf("node1 should not exist")
		}
		if hr.NodeCount() != 1 {
			t.Errorf("expected 1 node, got %d", hr.NodeCount())
		}
		if len(migrations) == 0 {
			t.Errorf("expected migration info, got none")
		}
	})

	t.Run("remove non-existent node", func(t *testing.T) {
		_, err := hr.RemoveNode("non-existent")
		if err != ErrNodeNotFound {
			t.Errorf("expected ErrNodeNotFound, got %v", err)
		}
	})

	t.Run("remove with empty id", func(t *testing.T) {
		_, err := hr.RemoveNode("")
		if err != ErrEmptyNodeID {
			t.Errorf("expected ErrEmptyNodeID, got %v", err)
		}
	})

	t.Run("remove last node", func(t *testing.T) {
		migrations, err := hr.RemoveNode("node2")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if migrations != nil {
			t.Errorf("expected nil migrations for last node, got %v", migrations)
		}
		if hr.NodeCount() != 0 {
			t.Errorf("expected 0 nodes, got %d", hr.NodeCount())
		}
	})
}

func TestGetNode(t *testing.T) {
	hr, _ := NewHashRing(100)

	t.Run("get from empty ring", func(t *testing.T) {
		_, err := hr.GetNode("key1")
		if err != ErrEmptyRing {
			t.Errorf("expected ErrEmptyRing, got %v", err)
		}
	})

	hr.AddNode("node1", 1)
	hr.AddNode("node2", 1)
	hr.AddNode("node3", 1)

	t.Run("get existing node", func(t *testing.T) {
		node, err := hr.GetNode("key1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if node != "node1" && node != "node2" && node != "node3" {
			t.Errorf("expected one of the nodes, got %s", node)
		}
	})

	t.Run("consistent hashing", func(t *testing.T) {
		node1, _ := hr.GetNode("mykey")
		node2, _ := hr.GetNode("mykey")
		if node1 != node2 {
			t.Errorf("consistent hashing failed: got %s then %s", node1, node2)
		}
	})
}

func TestGetNodes(t *testing.T) {
	hr, _ := NewHashRing(100)
	hr.AddNode("node1", 1)
	hr.AddNode("node2", 1)
	hr.AddNode("node3", 1)

	t.Run("get multiple nodes", func(t *testing.T) {
		nodes, err := hr.GetNodes("key1", 2)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(nodes) != 2 {
			t.Errorf("expected 2 nodes, got %d", len(nodes))
		}
		if nodes[0] == nodes[1] {
			t.Errorf("nodes should be distinct, got %v", nodes)
		}
	})

	t.Run("get all nodes", func(t *testing.T) {
		nodes, err := hr.GetNodes("key1", 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(nodes) != 3 {
			t.Errorf("expected 3 nodes, got %d", len(nodes))
		}
	})

	t.Run("get zero nodes", func(t *testing.T) {
		nodes, err := hr.GetNodes("key1", 0)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if nodes != nil {
			t.Errorf("expected nil, got %v", nodes)
		}
	})

	t.Run("get from empty ring", func(t *testing.T) {
		emptyHR, _ := NewHashRing(10)
		_, err := emptyHR.GetNodes("key1", 1)
		if err != ErrEmptyRing {
			t.Errorf("expected ErrEmptyRing, got %v", err)
		}
	})
}

func TestWeightDistribution(t *testing.T) {
	hr, _ := NewHashRing(100)

	hr.AddNode("node1", 1)
	hr.AddNode("node2", 2)
	hr.AddNode("node3", 3)

	totalVN := hr.VirtualNodeCount()
	expectedTotal := 100*1 + 100*2 + 100*3
	if totalVN != expectedTotal {
		t.Errorf("expected %d total virtual nodes, got %d", expectedTotal, totalVN)
	}

	counts := make(map[string]int)
	for _, nodeID := range hr.vnodeMap {
		counts[nodeID]++
	}

	if counts["node1"] != 100 {
		t.Errorf("node1 should have 100 vnodes, got %d", counts["node1"])
	}
	if counts["node2"] != 200 {
		t.Errorf("node2 should have 200 vnodes, got %d", counts["node2"])
	}
	if counts["node3"] != 300 {
		t.Errorf("node3 should have 300 vnodes, got %d", counts["node3"])
	}

	distribution := make(map[string]int)
	numKeys := 10000
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key%d", i)
		node, _ := hr.GetNode(key)
		distribution[node]++
	}

	ratio1 := float64(distribution["node1"]) / float64(numKeys)
	ratio2 := float64(distribution["node2"]) / float64(numKeys)
	ratio3 := float64(distribution["node3"]) / float64(numKeys)

	expectedRatio1 := 1.0 / 6.0
	expectedRatio2 := 2.0 / 6.0
	expectedRatio3 := 3.0 / 6.0

	tolerance := 0.1
	if abs(ratio1-expectedRatio1) > tolerance {
		t.Errorf("node1 distribution ratio %.4f differs from expected %.4f", ratio1, expectedRatio1)
	}
	if abs(ratio2-expectedRatio2) > tolerance {
		t.Errorf("node2 distribution ratio %.4f differs from expected %.4f", ratio2, expectedRatio2)
	}
	if abs(ratio3-expectedRatio3) > tolerance {
		t.Errorf("node3 distribution ratio %.4f differs from expected %.4f", ratio3, expectedRatio3)
	}
}

func TestUpdateNodeWeight(t *testing.T) {
	hr, _ := NewHashRing(10)
	hr.AddNode("node1", 1)
	hr.AddNode("node2", 1)

	t.Run("update weight", func(t *testing.T) {
		migrations, err := hr.UpdateNodeWeight("node1", 3)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(migrations) == 0 {
			t.Errorf("expected migration info")
		}

		info, _ := hr.GetNodeInfo("node1")
		if info.Weight != 3 {
			t.Errorf("expected weight 3, got %d", info.Weight)
		}

		expectedVN := 10*3 + 10*1
		if hr.VirtualNodeCount() != expectedVN {
			t.Errorf("expected %d virtual nodes, got %d", expectedVN, hr.VirtualNodeCount())
		}
	})

	t.Run("same weight", func(t *testing.T) {
		migrations, err := hr.UpdateNodeWeight("node1", 3)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if migrations != nil {
			t.Errorf("expected nil migrations for same weight")
		}
	})

	t.Run("decrease weight", func(t *testing.T) {
		hr2, _ := NewHashRing(10)
		hr2.AddNode("nodeA", 3)
		hr2.AddNode("nodeB", 1)
		hr2.SetTotalKeys(1000000)

		beforeMapping := make(map[string]string)
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("testkey%d", i)
			node, _ := hr2.GetNode(key)
			beforeMapping[key] = node
		}

		migrations, err := hr2.UpdateNodeWeight("nodeA", 1)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		info, _ := hr2.GetNodeInfo("nodeA")
		if info.Weight != 1 {
			t.Errorf("expected weight 1, got %d", info.Weight)
		}

		expectedVN := 10*1 + 10*1
		if hr2.VirtualNodeCount() != expectedVN {
			t.Errorf("expected %d virtual nodes, got %d", expectedVN, hr2.VirtualNodeCount())
		}

		if len(migrations) == 0 {
			t.Errorf("expected migration info for weight decrease")
		}

		hasRemovedMigration := false
		totalMigrated := int64(0)
		for _, m := range migrations {
			if m.FromNode == "nodeA" && m.ToNode != "nodeA" {
				hasRemovedMigration = true
				totalMigrated += m.EstimatedCount
			}
		}
		if !hasRemovedMigration {
			t.Errorf("expected migration from nodeA to other nodes (removedRanges)")
		}
		if totalMigrated <= 0 {
			t.Errorf("expected positive estimated count for removed ranges")
		}

		migratedKeys := 0
		stableKeys := 0
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("testkey%d", i)
			oldNode := beforeMapping[key]
			newNode, _ := hr2.GetNode(key)

			if oldNode == "nodeA" && newNode != "nodeA" {
				migratedKeys++
			} else if oldNode == newNode {
				stableKeys++
			}
		}

		if migratedKeys == 0 {
			t.Errorf("expected some keys to migrate away from nodeA after weight decrease")
		}

		expectedMigratedRatio := (3.0/4.0 - 1.0/2.0)
		actualMigratedRatio := float64(migratedKeys) / 1000.0
		tolerance := 0.15
		if abs(actualMigratedRatio-expectedMigratedRatio) > tolerance {
			t.Errorf("migration ratio %.4f differs from expected %.4f by more than tolerance %.4f",
				actualMigratedRatio, expectedMigratedRatio, tolerance)
		}
	})

	t.Run("non-existent node", func(t *testing.T) {
		_, err := hr.UpdateNodeWeight("non-existent", 2)
		if err != ErrNodeNotFound {
			t.Errorf("expected ErrNodeNotFound, got %v", err)
		}
	})

	t.Run("invalid weight", func(t *testing.T) {
		_, err := hr.UpdateNodeWeight("node1", 0)
		if err != ErrInvalidWeight {
			t.Errorf("expected ErrInvalidWeight, got %v", err)
		}
	})

	t.Run("empty node id", func(t *testing.T) {
		_, err := hr.UpdateNodeWeight("", 2)
		if err != ErrEmptyNodeID {
			t.Errorf("expected ErrEmptyNodeID, got %v", err)
		}
	})
}

func TestCalculateAddMigration(t *testing.T) {
	hr, _ := NewHashRing(10)
	hr.AddNode("node1", 1)
	hr.AddNode("node2", 1)
	hr.SetTotalKeys(1000000)

	t.Run("calculate migration before add", func(t *testing.T) {
		migrations, err := hr.CalculateAddMigration("node3", 1)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(migrations) == 0 {
			t.Errorf("expected migration info")
		}

		totalMigrate := int64(0)
		for _, m := range migrations {
			if m.ToNode != "node3" {
				t.Errorf("expected ToNode node3, got %s", m.ToNode)
			}
			if m.FromNode != "node1" && m.FromNode != "node2" {
				t.Errorf("expected FromNode node1 or node2, got %s", m.FromNode)
			}
			totalMigrate += m.EstimatedCount
		}

		expectedMigrate := int64(1000000 / 3)
		tolerance := float64(1000000) * 0.1
		if absInt64(totalMigrate-expectedMigrate) > int64(tolerance) {
			t.Errorf("estimated migration %d differs from expected %d", totalMigrate, expectedMigrate)
		}
	})

	t.Run("existing node", func(t *testing.T) {
		_, err := hr.CalculateAddMigration("node1", 1)
		if err != ErrNodeAlreadyExists {
			t.Errorf("expected ErrNodeAlreadyExists, got %v", err)
		}
	})

	t.Run("invalid params", func(t *testing.T) {
		_, err := hr.CalculateAddMigration("", 1)
		if err != ErrEmptyNodeID {
			t.Errorf("expected ErrEmptyNodeID, got %v", err)
		}
		_, err = hr.CalculateAddMigration("node3", 0)
		if err != ErrInvalidWeight {
			t.Errorf("expected ErrInvalidWeight, got %v", err)
		}
	})
}

func TestNodeRemovalMigration(t *testing.T) {
	hr, _ := NewHashRing(10)
	hr.AddNode("node1", 1)
	hr.AddNode("node2", 1)
	hr.AddNode("node3", 1)
	hr.SetTotalKeys(1000000)

	beforeMapping := make(map[string]string)
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("testkey%d", i)
		node, _ := hr.GetNode(key)
		beforeMapping[key] = node
	}

	migrations, err := hr.RemoveNode("node2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(migrations) < 1 {
		t.Fatalf("expected at least 1 migration, got %d", len(migrations))
	}

	affectedKeys := 0
	stableKeys := 0
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("testkey%d", i)
		oldNode := beforeMapping[key]
		newNode, _ := hr.GetNode(key)

		if oldNode == "node2" {
			if newNode == "node2" {
				t.Errorf("key %s should be migrated from node2", key)
			}
			affectedKeys++
		} else {
			if newNode != oldNode {
				t.Errorf("key %s changed from %s to %s unexpectedly", key, oldNode, newNode)
			}
			stableKeys++
		}
	}

	if affectedKeys == 0 {
		t.Errorf("expected some keys to be affected")
	}

	totalMigrate := int64(0)
	for _, m := range migrations {
		if m.FromNode != "node2" {
			t.Errorf("expected FromNode node2, got %s", m.FromNode)
		}
		totalMigrate += m.EstimatedCount
	}

	expectedRatio := 1.0 / 3.0
	actualRatio := float64(affectedKeys) / 1000.0
	if abs(actualRatio-expectedRatio) > 0.15 {
		t.Errorf("migration ratio %.4f differs from expected %.4f", actualRatio, expectedRatio)
	}
}

func TestSerialization(t *testing.T) {
	hr, _ := NewHashRing(50)
	hr.AddNodeWithInfo(NodeInfo{
		ID:     "node1",
		Weight: 2,
		Addr:   "192.168.1.1:8080",
	})
	hr.AddNodeWithInfo(NodeInfo{
		ID:     "node2",
		Weight: 3,
		Addr:   "192.168.1.2:8080",
	})
	hr.SetTotalKeys(100000)
	hr.SetMetadata("cluster", "prod")

	t.Run("snapshot and restore", func(t *testing.T) {
		snapshot := hr.Snapshot()

		if snapshot.Version != CurrentVersion {
			t.Errorf("expected version %d, got %d", CurrentVersion, snapshot.Version)
		}
		if snapshot.VirtualNodes != 50 {
			t.Errorf("expected virtualNodes 50, got %d", snapshot.VirtualNodes)
		}
		if len(snapshot.Nodes) != 2 {
			t.Errorf("expected 2 nodes, got %d", len(snapshot.Nodes))
		}
		if snapshot.TotalKeys != 100000 {
			t.Errorf("expected totalKeys 100000, got %d", snapshot.TotalKeys)
		}

		t.Run("verify vnode index field", func(t *testing.T) {
			node1VNodes := make([]VirtualNode, 0)
			node2VNodes := make([]VirtualNode, 0)
			for _, vn := range snapshot.VNodes {
				switch vn.NodeID {
				case "node1":
					node1VNodes = append(node1VNodes, vn)
				case "node2":
					node2VNodes = append(node2VNodes, vn)
				}
			}

			if len(node1VNodes) != 50*2 {
				t.Errorf("expected %d vnodes for node1, got %d", 50*2, len(node1VNodes))
			}
			if len(node2VNodes) != 50*3 {
				t.Errorf("expected %d vnodes for node2, got %d", 50*3, len(node2VNodes))
			}

			indexSet1 := make(map[int]bool)
			for _, vn := range node1VNodes {
				if vn.Index < 0 || vn.Index >= 50*2 {
					t.Errorf("node1 vnode index %d out of range [0, %d)", vn.Index, 50*2)
				}
				if indexSet1[vn.Index] {
					t.Errorf("node1 has duplicate vnode index %d", vn.Index)
				}
				indexSet1[vn.Index] = true
			}

			indexSet2 := make(map[int]bool)
			for _, vn := range node2VNodes {
				if vn.Index < 0 || vn.Index >= 50*3 {
					t.Errorf("node2 vnode index %d out of range [0, %d)", vn.Index, 50*3)
				}
				if indexSet2[vn.Index] {
					t.Errorf("node2 has duplicate vnode index %d", vn.Index)
				}
				indexSet2[vn.Index] = true
			}

			for i := 0; i < 50*2; i++ {
				if !indexSet1[i] {
					t.Errorf("node1 missing vnode index %d", i)
				}
			}
			for i := 0; i < 50*3; i++ {
				if !indexSet2[i] {
					t.Errorf("node2 missing vnode index %d", i)
				}
			}
		})

		hr2, _ := NewHashRing(10)
		err := hr2.Restore(snapshot)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if hr2.virtualNodes != 50 {
			t.Errorf("restored virtualNodes should be 50, got %d", hr2.virtualNodes)
		}
		if hr2.NodeCount() != 2 {
			t.Errorf("restored node count should be 2, got %d", hr2.NodeCount())
		}
		if hr2.GetTotalKeys() != 100000 {
			t.Errorf("restored totalKeys should be 100000, got %d", hr2.GetTotalKeys())
		}
		if hr2.VirtualNodeCount() != 50*2+50*3 {
			t.Errorf("restored vnode count mismatch")
		}

		val, exists := hr2.GetMetadata("cluster")
		if !exists || val != "prod" {
			t.Errorf("metadata not restored properly")
		}

		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key%d", i)
			n1, _ := hr.GetNode(key)
			n2, _ := hr2.GetNode(key)
			if n1 != n2 {
				t.Errorf("key %s mapping differs: %s vs %s", key, n1, n2)
			}
		}
	})

	t.Run("json marshal/unmarshal", func(t *testing.T) {
		data, err := hr.MarshalJSON()
		if err != nil {
			t.Errorf("marshal error: %v", err)
		}

		hr3, _ := NewHashRing(10)
		err = hr3.UnmarshalJSON(data)
		if err != nil {
			t.Errorf("unmarshal error: %v", err)
		}

		if hr3.NodeCount() != 2 {
			t.Errorf("expected 2 nodes, got %d", hr3.NodeCount())
		}
	})

	t.Run("file save/load", func(t *testing.T) {
		tmpfile, err := os.CreateTemp("", "chainhash-*.json")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpfile.Name())
		tmpfile.Close()

		err = hr.SaveToFile(tmpfile.Name())
		if err != nil {
			t.Errorf("save error: %v", err)
		}

		hr4, err := LoadFromFile(tmpfile.Name())
		if err != nil {
			t.Errorf("load error: %v", err)
		}

		if hr4.NodeCount() != 2 {
			t.Errorf("expected 2 nodes, got %d", hr4.NodeCount())
		}
		if hr4.GetTotalKeys() != 100000 {
			t.Errorf("expected totalKeys 100000, got %d", hr4.GetTotalKeys())
		}
	})

	t.Run("restore from nil snapshot", func(t *testing.T) {
		hr2, _ := NewHashRing(10)
		err := hr2.Restore(nil)
		if err != ErrDeserializationFailed {
			t.Errorf("expected ErrDeserializationFailed, got %v", err)
		}
	})

	t.Run("restore with invalid version", func(t *testing.T) {
		snapshot := &RingSnapshot{
			Version:      0,
			VirtualNodes: 10,
		}
		hr2, _ := NewHashRing(10)
		err := hr2.Restore(snapshot)
		if err == nil {
			t.Errorf("expected error for invalid version")
		}
	})

	t.Run("restore with unknown node", func(t *testing.T) {
		snapshot := &RingSnapshot{
			Version:      1,
			VirtualNodes: 10,
			Nodes: []NodeInfo{
				{ID: "node1", Weight: 1},
			},
			VNodes: []VirtualNode{
				{Hash: 123, NodeID: "node2"},
			},
		}
		hr2, _ := NewHashRing(10)
		err := hr2.Restore(snapshot)
		if err == nil {
			t.Errorf("expected error for unknown node reference")
		}
	})

	t.Run("load non-existent file", func(t *testing.T) {
		_, err := LoadFromFile("/non/existent/file.json")
		if err == nil {
			t.Errorf("expected error for non-existent file")
		}
	})
}

func TestConcurrency(t *testing.T) {
	hr, _ := NewHashRing(100)

	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				nodeID := fmt.Sprintf("node-%d-%d", id, j%5)
				if j%3 == 0 {
					hr.AddNode(nodeID, 1)
				} else if j%3 == 1 {
					hr.GetNode(fmt.Sprintf("key-%d", j))
				} else {
					hr.RemoveNode(nodeID)
				}
			}
		}(i)
	}

	wg.Wait()

	t.Run("verify node count", func(t *testing.T) {
		nodeCount := hr.NodeCount()
		maxPossibleNodes := numGoroutines * 5
		if nodeCount > maxPossibleNodes {
			t.Errorf("node count %d exceeds maximum expected %d", nodeCount, maxPossibleNodes)
		}
	})

	t.Run("verify ring sorted", func(t *testing.T) {
		if !sort.SliceIsSorted(hr.ring, func(i, j int) bool {
			return hr.ring[i] < hr.ring[j]
		}) {
			t.Errorf("ring should be sorted after concurrent operations")
		}

		prev := uint64(0)
		for idx, h := range hr.ring {
			if idx > 0 && h < prev {
				t.Errorf("ring not sorted at index %d: %d < %d", idx, h, prev)
			}
			prev = h
		}
	})

	t.Run("verify vnodeMap and ring consistency", func(t *testing.T) {
		if len(hr.ring) != len(hr.vnodeMap) {
			t.Errorf("ring length %d should equal vnodeMap length %d", len(hr.ring), len(hr.vnodeMap))
		}

		for _, hash := range hr.ring {
			if _, exists := hr.vnodeMap[hash]; !exists {
				t.Errorf("ring hash %d not found in vnodeMap", hash)
			}
		}

		ringSet := make(map[uint64]struct{}, len(hr.ring))
		for _, h := range hr.ring {
			ringSet[h] = struct{}{}
		}
		for hash := range hr.vnodeMap {
			if _, exists := ringSet[hash]; !exists {
				t.Errorf("vnodeMap hash %d not found in ring", hash)
			}
		}
	})

	t.Run("verify vnodeMap references valid nodes", func(t *testing.T) {
		for hash, nodeID := range hr.vnodeMap {
			if !hr.NodeExists(nodeID) {
				t.Errorf("vnode hash %d references non-existent node %s", hash, nodeID)
			}
		}
	})

	t.Run("verify virtual node count matches weight", func(t *testing.T) {
		expectedTotal := 0
		for _, node := range hr.nodes {
			expectedTotal += hr.virtualNodes * node.Weight
		}
		if hr.VirtualNodeCount() != expectedTotal {
			t.Errorf("expected %d virtual nodes, got %d", expectedTotal, hr.VirtualNodeCount())
		}
	})
}

func TestRingOrder(t *testing.T) {
	hr, _ := NewHashRing(10)
	hr.AddNode("node1", 1)

	if !sort.SliceIsSorted(hr.ring, func(i, j int) bool {
		return hr.ring[i] < hr.ring[j]
	}) {
		t.Errorf("ring should be sorted")
	}

	prev := uint64(0)
	for _, h := range hr.ring {
		if h < prev {
			t.Errorf("ring not sorted: %d < %d", h, prev)
		}
		prev = h
	}
}

func TestMetadata(t *testing.T) {
	hr, _ := NewHashRing(10)

	hr.SetMetadata("env", "test")
	hr.SetMetadata("version", 1)

	val, exists := hr.GetMetadata("env")
	if !exists {
		t.Errorf("env key should exist")
	}
	if val != "test" {
		t.Errorf("expected test, got %v", val)
	}

	val, exists = hr.GetMetadata("version")
	if !exists {
		t.Errorf("version key should exist")
	}
	if val != 1 {
		t.Errorf("expected 1, got %v", val)
	}

	_, exists = hr.GetMetadata("nonexistent")
	if exists {
		t.Errorf("nonexistent key should not exist")
	}
}

func TestTotalKeys(t *testing.T) {
	hr, _ := NewHashRing(10)
	hr.SetTotalKeys(5000)

	if hr.GetTotalKeys() != 5000 {
		t.Errorf("expected 5000, got %d", hr.GetTotalKeys())
	}

	hr.AddNode("node1", 1)
	hr.SetTotalKeys(10000)
	migrations, _ := hr.CalculateAddMigration("node2", 1)

	if len(migrations) > 0 && migrations[0].TotalKeys != 10000 {
		t.Errorf("migration should use updated totalKeys")
	}
}

func TestGetNodeInfo(t *testing.T) {
	hr, _ := NewHashRing(10)

	_, err := hr.GetNodeInfo("nonexistent")
	if err != ErrNodeNotFound {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}

	hr.AddNodeWithInfo(NodeInfo{
		ID:     "node1",
		Weight: 2,
		Addr:   "localhost:8080",
	})

	info, err := hr.GetNodeInfo("node1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if info.ID != "node1" {
		t.Errorf("expected node1, got %s", info.ID)
	}
	if info.Weight != 2 {
		t.Errorf("expected weight 2, got %d", info.Weight)
	}
	if info.Addr != "localhost:8080" {
		t.Errorf("expected addr localhost:8080, got %s", info.Addr)
	}

	info.Weight = 999
	info2, _ := hr.GetNodeInfo("node1")
	if info2.Weight != 2 {
		t.Errorf("GetNodeInfo should return a copy, not reference")
	}
}

func TestGetAllNodes(t *testing.T) {
	hr, _ := NewHashRing(10)

	hr.AddNode("node1", 1)
	hr.AddNode("node2", 2)
	hr.AddNode("node3", 3)

	nodes := hr.GetAllNodes()
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodes))
	}

	idSet := make(map[string]bool)
	for _, n := range nodes {
		idSet[n.ID] = true
	}

	if !idSet["node1"] || !idSet["node2"] || !idSet["node3"] {
		t.Errorf("not all nodes returned: %v", nodes)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func absInt64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
