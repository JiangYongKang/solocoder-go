package graphdb

import (
	"testing"
)

func TestNewGraph(t *testing.T) {
	g := NewGraph()
	if g == nil {
		t.Fatal("NewGraph returned nil")
	}
	if g.NodeCount() != 0 {
		t.Errorf("expected 0 nodes, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}
}

func TestAddNode(t *testing.T) {
	g := NewGraph()

	err := g.AddNode("A", nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !g.HasNode("A") {
		t.Error("expected node A to exist")
	}
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}

	props := map[string]interface{}{"name": "Alice", "age": 30}
	err = g.AddNode("B", props)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !g.HasNode("B") {
		t.Error("expected node B to exist")
	}

	nodeB, err := g.GetNode("B")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if nodeB.Properties["name"] != "Alice" {
		t.Errorf("expected name Alice, got %v", nodeB.Properties["name"])
	}
	if nodeB.Properties["age"] != 30 {
		t.Errorf("expected age 30, got %v", nodeB.Properties["age"])
	}
}

func TestAddNode_EmptyID(t *testing.T) {
	g := NewGraph()
	err := g.AddNode("", nil)
	if err != ErrNodeNotFound {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestAddNode_Duplicate(t *testing.T) {
	g := NewGraph()
	err := g.AddNode("A", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = g.AddNode("A", nil)
	if err != ErrNodeExists {
		t.Errorf("expected ErrNodeExists, got %v", err)
	}
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
}

func TestRemoveNode(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	g.AddNode("B", nil)
	g.AddEdge("A", "B", 1.0, "", nil)

	err := g.RemoveNode("A")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if g.HasNode("A") {
		t.Error("expected node A to be removed")
	}
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges after removing node, got %d", g.EdgeCount())
	}
	if g.HasEdge("A", "B") {
		t.Error("expected edge A->B to be removed")
	}
}

func TestRemoveNode_NotFound(t *testing.T) {
	g := NewGraph()
	err := g.RemoveNode("X")
	if err != ErrNodeNotFound {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestRemoveNode_RemovesIncidentEdges(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	g.AddNode("B", nil)
	g.AddNode("C", nil)
	g.AddEdge("A", "B", 1, "", nil)
	g.AddEdge("B", "C", 2, "", nil)
	g.AddEdge("A", "C", 3, "", nil)

	g.RemoveNode("B")

	if g.HasEdge("A", "B") {
		t.Error("edge A->B should be removed")
	}
	if g.HasEdge("B", "C") {
		t.Error("edge B->C should be removed")
	}
	if !g.HasEdge("A", "C") {
		t.Error("edge A->C should still exist")
	}
	if g.EdgeCount() != 1 {
		t.Errorf("expected 1 edge, got %d", g.EdgeCount())
	}
}

func TestHasNode(t *testing.T) {
	g := NewGraph()
	if g.HasNode("A") {
		t.Error("node A should not exist")
	}
	g.AddNode("A", nil)
	if !g.HasNode("A") {
		t.Error("node A should exist")
	}
}

func TestGetNode(t *testing.T) {
	g := NewGraph()
	_, err := g.GetNode("X")
	if err != ErrNodeNotFound {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}

	props := map[string]interface{}{"key": "value"}
	g.AddNode("A", props)
	node, err := g.GetNode("A")
	if err != nil {
		t.Fatal(err)
	}
	if node.ID != "A" {
		t.Errorf("expected ID A, got %s", node.ID)
	}
	if node.Properties["key"] != "value" {
		t.Errorf("expected value, got %v", node.Properties["key"])
	}
}

func TestGetNode_DefensiveCopy(t *testing.T) {
	g := NewGraph()
	props := map[string]interface{}{"key": "value"}
	g.AddNode("A", props)

	node, _ := g.GetNode("A")
	node.Properties["key"] = "changed"

	node2, _ := g.GetNode("A")
	if node2.Properties["key"] != "value" {
		t.Error("modifying returned node should not affect stored node")
	}
}

func TestNodeCount(t *testing.T) {
	g := NewGraph()
	if g.NodeCount() != 0 {
		t.Errorf("expected 0, got %d", g.NodeCount())
	}
	g.AddNode("A", nil)
	g.AddNode("B", nil)
	if g.NodeCount() != 2 {
		t.Errorf("expected 2, got %d", g.NodeCount())
	}
	g.RemoveNode("A")
	if g.NodeCount() != 1 {
		t.Errorf("expected 1, got %d", g.NodeCount())
	}
}

func TestAddEdge(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	g.AddNode("B", nil)

	err := g.AddEdge("A", "B", 2.5, "connects", map[string]interface{}{"type": "road"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !g.HasEdge("A", "B") {
		t.Error("expected edge A->B to exist")
	}
	if g.EdgeCount() != 1 {
		t.Errorf("expected 1 edge, got %d", g.EdgeCount())
	}

	edge, err := g.GetEdge("A", "B")
	if err != nil {
		t.Fatal(err)
	}
	if edge.Weight != 2.5 {
		t.Errorf("expected weight 2.5, got %f", edge.Weight)
	}
	if edge.Label != "connects" {
		t.Errorf("expected label 'connects', got %s", edge.Label)
	}
	if edge.Properties["type"] != "road" {
		t.Errorf("expected type 'road', got %v", edge.Properties["type"])
	}
}

func TestAddEdge_SelfLoop(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	err := g.AddEdge("A", "A", 1, "", nil)
	if err != ErrSelfLoop {
		t.Errorf("expected ErrSelfLoop, got %v", err)
	}
}

func TestAddEdge_NegativeWeight(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	g.AddNode("B", nil)
	err := g.AddEdge("A", "B", -1, "", nil)
	if err != ErrNegativeWeight {
		t.Errorf("expected ErrNegativeWeight, got %v", err)
	}
}

func TestAddEdge_InvalidStart(t *testing.T) {
	g := NewGraph()
	g.AddNode("B", nil)
	err := g.AddEdge("A", "B", 1, "", nil)
	if err != ErrInvalidStartNode {
		t.Errorf("expected ErrInvalidStartNode, got %v", err)
	}
}

func TestAddEdge_InvalidEnd(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	err := g.AddEdge("A", "B", 1, "", nil)
	if err != ErrInvalidEndNode {
		t.Errorf("expected ErrInvalidEndNode, got %v", err)
	}
}

func TestAddEdge_Duplicate(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	g.AddNode("B", nil)
	err := g.AddEdge("A", "B", 1, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = g.AddEdge("A", "B", 2, "", nil)
	if err != ErrEdgeExists {
		t.Errorf("expected ErrEdgeExists, got %v", err)
	}
	if g.EdgeCount() != 1 {
		t.Errorf("expected 1 edge, got %d", g.EdgeCount())
	}
}

func TestRemoveEdge(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	g.AddNode("B", nil)
	g.AddEdge("A", "B", 1, "", nil)

	err := g.RemoveEdge("A", "B")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if g.HasEdge("A", "B") {
		t.Error("edge should be removed")
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}
}

func TestRemoveEdge_NotFound(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	g.AddNode("B", nil)
	err := g.RemoveEdge("A", "B")
	if err != ErrEdgeNotFound {
		t.Errorf("expected ErrEdgeNotFound, got %v", err)
	}
}

func TestRemoveEdge_InvalidStart(t *testing.T) {
	g := NewGraph()
	g.AddNode("B", nil)
	err := g.RemoveEdge("A", "B")
	if err != ErrInvalidStartNode {
		t.Errorf("expected ErrInvalidStartNode, got %v", err)
	}
}

func TestRemoveEdge_InvalidEnd(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	err := g.RemoveEdge("A", "B")
	if err != ErrInvalidEndNode {
		t.Errorf("expected ErrInvalidEndNode, got %v", err)
	}
}

func TestHasEdge(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	g.AddNode("B", nil)
	if g.HasEdge("A", "B") {
		t.Error("edge should not exist")
	}
	g.AddEdge("A", "B", 1, "", nil)
	if !g.HasEdge("A", "B") {
		t.Error("edge should exist")
	}
	if g.HasEdge("B", "A") {
		t.Error("reverse edge should not exist")
	}
}

func TestGetEdge(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	g.AddNode("B", nil)

	_, err := g.GetEdge("A", "B")
	if err != ErrEdgeNotFound {
		t.Errorf("expected ErrEdgeNotFound, got %v", err)
	}

	g.AddEdge("A", "B", 5, "test", map[string]interface{}{"x": 1})
	edge, err := g.GetEdge("A", "B")
	if err != nil {
		t.Fatal(err)
	}
	if edge.From != "A" || edge.To != "B" {
		t.Errorf("wrong edge endpoints: %s->%s", edge.From, edge.To)
	}
	if edge.Weight != 5 {
		t.Errorf("expected weight 5, got %f", edge.Weight)
	}
}

func TestEdgeCount(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	g.AddNode("B", nil)
	g.AddNode("C", nil)
	g.AddEdge("A", "B", 1, "", nil)
	g.AddEdge("B", "C", 2, "", nil)
	if g.EdgeCount() != 2 {
		t.Errorf("expected 2 edges, got %d", g.EdgeCount())
	}
}

func TestGetOutEdges(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	g.AddNode("B", nil)
	g.AddNode("C", nil)
	g.AddNode("D", nil)

	g.AddEdge("A", "C", 3, "", nil)
	g.AddEdge("A", "B", 1, "", nil)
	g.AddEdge("A", "D", 2, "", nil)

	edges, err := g.GetOutEdges("A")
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 3 {
		t.Errorf("expected 3 out edges, got %d", len(edges))
	}

	for i := 1; i < len(edges); i++ {
		if edges[i].Weight < edges[i-1].Weight {
			t.Errorf("edges not sorted by weight: %v, %v at %d", edges[i-1].Weight, edges[i].Weight, i)
		}
	}

	if edges[0].To != "B" || edges[1].To != "D" || edges[2].To != "C" {
		t.Errorf("unexpected edge order: %v, %v, %v", edges[0].To, edges[1].To, edges[2].To)
	}
}

func TestGetOutEdges_NotFound(t *testing.T) {
	g := NewGraph()
	_, err := g.GetOutEdges("X")
	if err != ErrNodeNotFound {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestGetInEdges(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	g.AddNode("B", nil)
	g.AddNode("C", nil)
	g.AddNode("D", nil)

	g.AddEdge("B", "D", 5, "", nil)
	g.AddEdge("A", "D", 1, "", nil)
	g.AddEdge("C", "D", 3, "", nil)

	edges, err := g.GetInEdges("D")
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 3 {
		t.Errorf("expected 3 in edges, got %d", len(edges))
	}

	for i := 1; i < len(edges); i++ {
		if edges[i].Weight < edges[i-1].Weight {
			t.Errorf("edges not sorted by weight: %v, %v at %d", edges[i-1].Weight, edges[i].Weight, i)
		}
	}
}

func TestGetInEdges_NotFound(t *testing.T) {
	g := NewGraph()
	_, err := g.GetInEdges("X")
	if err != ErrNodeNotFound {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestBFS_Simple(t *testing.T) {
	g := NewGraph()
	for _, id := range []string{"A", "B", "C", "D", "E"} {
		g.AddNode(id, nil)
	}
	g.AddEdge("A", "B", 1, "", nil)
	g.AddEdge("A", "C", 1, "", nil)
	g.AddEdge("B", "D", 1, "", nil)
	g.AddEdge("C", "E", 1, "", nil)

	result, err := g.BFS("A", 10)
	if err != nil {
		t.Fatal(err)
	}
	if result[0] != "A" {
		t.Errorf("expected first node A, got %s", result[0])
	}
	if len(result) != 5 {
		t.Errorf("expected 5 nodes, got %d: %v", len(result), result)
	}

	visited := make(map[string]bool)
	for _, id := range result {
		if visited[id] {
			t.Errorf("duplicate node %s in BFS result", id)
		}
		visited[id] = true
	}
}

func TestBFS_Disconnected(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	g.AddNode("B", nil)
	g.AddNode("C", nil)
	g.AddEdge("A", "B", 1, "", nil)

	result, err := g.BFS("A", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 nodes (A, B), got %d: %v", len(result), result)
	}
	for _, id := range result {
		if id == "C" {
			t.Error("C should not be reachable from A")
		}
	}
}

func TestBFS_MaxDepth(t *testing.T) {
	g := NewGraph()
	for _, id := range []string{"A", "B", "C", "D", "E"} {
		g.AddNode(id, nil)
	}
	g.AddEdge("A", "B", 1, "", nil)
	g.AddEdge("B", "C", 1, "", nil)
	g.AddEdge("C", "D", 1, "", nil)
	g.AddEdge("D", "E", 1, "", nil)

	result, err := g.BFS("A", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 nodes with depth 2, got %d: %v", len(result), result)
	}
	expected := map[string]bool{"A": true, "B": true, "C": true}
	for _, id := range result {
		if !expected[id] {
			t.Errorf("unexpected node %s in result", id)
		}
	}
}

func TestBFS_InvalidStart(t *testing.T) {
	g := NewGraph()
	_, err := g.BFS("X", 10)
	if err != ErrNodeNotFound {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestBFS_InvalidMaxDepth(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	_, err := g.BFS("A", 0)
	if err != ErrMaxDepthNonPositive {
		t.Errorf("expected ErrMaxDepthNonPositive, got %v", err)
	}
	_, err = g.BFS("A", -1)
	if err != ErrMaxDepthNonPositive {
		t.Errorf("expected ErrMaxDepthNonPositive, got %v", err)
	}
}

func TestDFS_Simple(t *testing.T) {
	g := NewGraph()
	for _, id := range []string{"A", "B", "C", "D"} {
		g.AddNode(id, nil)
	}
	g.AddEdge("A", "B", 1, "", nil)
	g.AddEdge("A", "C", 1, "", nil)
	g.AddEdge("B", "D", 1, "", nil)

	result, err := g.DFS("A", 10)
	if err != nil {
		t.Fatal(err)
	}
	if result[0] != "A" {
		t.Errorf("expected first node A, got %s", result[0])
	}
	if len(result) != 4 {
		t.Errorf("expected 4 nodes, got %d: %v", len(result), result)
	}

	visited := make(map[string]bool)
	for _, id := range result {
		if visited[id] {
			t.Errorf("duplicate node %s in DFS result", id)
		}
		visited[id] = true
	}
}

func TestDFS_Disconnected(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	g.AddNode("B", nil)
	g.AddNode("C", nil)
	g.AddEdge("A", "B", 1, "", nil)

	result, err := g.DFS("A", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 nodes, got %d: %v", len(result), result)
	}
}

func TestDFS_MaxDepth(t *testing.T) {
	g := NewGraph()
	for _, id := range []string{"A", "B", "C", "D", "E"} {
		g.AddNode(id, nil)
	}
	g.AddEdge("A", "B", 1, "", nil)
	g.AddEdge("B", "C", 1, "", nil)
	g.AddEdge("C", "D", 1, "", nil)
	g.AddEdge("D", "E", 1, "", nil)

	result, err := g.DFS("A", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 nodes, got %d: %v", len(result), result)
	}
}

func TestDFS_InvalidStart(t *testing.T) {
	g := NewGraph()
	_, err := g.DFS("X", 10)
	if err != ErrNodeNotFound {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestDFS_InvalidMaxDepth(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	_, err := g.DFS("A", 0)
	if err != ErrMaxDepthNonPositive {
		t.Errorf("expected ErrMaxDepthNonPositive, got %v", err)
	}
}

func TestShortestPath_Simple(t *testing.T) {
	g := NewGraph()
	for _, id := range []string{"A", "B", "C", "D"} {
		g.AddNode(id, nil)
	}
	g.AddEdge("A", "B", 4, "", nil)
	g.AddEdge("A", "C", 2, "", nil)
	g.AddEdge("B", "D", 3, "", nil)
	g.AddEdge("C", "B", 1, "", nil)
	g.AddEdge("C", "D", 5, "", nil)

	result, err := g.ShortestPath("A", "D")
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d: %v", len(result.Nodes), result.Nodes)
	}
	if result.Nodes[0] != "A" || result.Nodes[len(result.Nodes)-1] != "D" {
		t.Errorf("invalid path endpoints: %v", result.Nodes)
	}
	if result.Weight != 6 {
		t.Errorf("expected weight 6, got %f", result.Weight)
	}
}

func TestShortestPath_SameNode(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	result, err := g.ShortestPath("A", "A")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Nodes) != 1 || result.Nodes[0] != "A" {
		t.Errorf("expected [A], got %v", result.Nodes)
	}
	if result.Weight != 0 {
		t.Errorf("expected weight 0, got %f", result.Weight)
	}
}

func TestShortestPath_NoPath(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	g.AddNode("B", nil)
	g.AddNode("C", nil)
	g.AddEdge("A", "B", 1, "", nil)

	_, err := g.ShortestPath("A", "C")
	if err != ErrNoPath {
		t.Errorf("expected ErrNoPath, got %v", err)
	}
}

func TestShortestPath_InvalidStart(t *testing.T) {
	g := NewGraph()
	g.AddNode("B", nil)
	_, err := g.ShortestPath("A", "B")
	if err != ErrInvalidStartNode {
		t.Errorf("expected ErrInvalidStartNode, got %v", err)
	}
}

func TestShortestPath_InvalidEnd(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	_, err := g.ShortestPath("A", "B")
	if err != ErrInvalidEndNode {
		t.Errorf("expected ErrInvalidEndNode, got %v", err)
	}
}

func TestShortestPath_DirectVsIndirect(t *testing.T) {
	g := NewGraph()
	for _, id := range []string{"A", "B", "C"} {
		g.AddNode(id, nil)
	}
	g.AddEdge("A", "C", 10, "", nil)
	g.AddEdge("A", "B", 1, "", nil)
	g.AddEdge("B", "C", 1, "", nil)

	result, err := g.ShortestPath("A", "C")
	if err != nil {
		t.Fatal(err)
	}
	if result.Weight != 2 {
		t.Errorf("expected weight 2, got %f", result.Weight)
	}
	if len(result.Nodes) != 3 {
		t.Errorf("expected path A->B->C (3 nodes), got %v", result.Nodes)
	}
}

func TestOutEdgesSortedAfterInsertions(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	g.AddNode("B", nil)
	g.AddNode("C", nil)
	g.AddNode("D", nil)

	g.AddEdge("A", "D", 10, "", nil)
	g.AddEdge("A", "B", 2, "", nil)
	g.AddEdge("A", "C", 5, "", nil)

	edges, err := g.GetOutEdges("A")
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(edges))
	}
	for i := 1; i < len(edges); i++ {
		if edges[i].Weight < edges[i-1].Weight {
			t.Errorf("edges should be sorted by weight ascending")
		}
	}
}

func TestConcurrentAddNodes(t *testing.T) {
	g := NewGraph()
	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			g.AddNode("A"+string(rune('0'+i%10)), nil)
		}
		done <- true
	}()
	go func() {
		for i := 0; i < 100; i++ {
			g.AddNode("B"+string(rune('0'+i%10)), nil)
		}
		done <- true
	}()
	<-done
	<-done

	count := g.NodeCount()
	if count <= 0 {
		t.Errorf("expected positive node count, got %d", count)
	}
}

func TestAddNode_PropertiesNil(t *testing.T) {
	g := NewGraph()
	err := g.AddNode("A", nil)
	if err != nil {
		t.Fatal(err)
	}
	node, _ := g.GetNode("A")
	if node.Properties == nil {
		t.Error("Properties should be initialized even when nil passed")
	}
	if len(node.Properties) != 0 {
		t.Errorf("expected empty properties, got %v", node.Properties)
	}
}

func TestAddEdge_ZeroWeight(t *testing.T) {
	g := NewGraph()
	g.AddNode("A", nil)
	g.AddNode("B", nil)
	err := g.AddEdge("A", "B", 0, "", nil)
	if err != nil {
		t.Errorf("zero weight should be allowed, got %v", err)
	}
}
