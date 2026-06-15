package graphdb

import (
	"container/heap"
	"errors"
	"sort"
	"sync"
)

var (
	ErrNodeNotFound       = errors.New("node not found")
	ErrNodeExists         = errors.New("node already exists")
	ErrEdgeNotFound       = errors.New("edge not found")
	ErrEdgeExists         = errors.New("edge already exists")
	ErrSelfLoop           = errors.New("self-loop edge not allowed")
	ErrNegativeWeight     = errors.New("negative edge weight not allowed")
	ErrInvalidStartNode   = errors.New("invalid start node")
	ErrInvalidEndNode     = errors.New("invalid end node")
	ErrMaxDepthNonPositive = errors.New("max depth must be positive")
	ErrNoPath             = errors.New("no path exists between nodes")
)

type Node struct {
	ID         string
	Properties map[string]interface{}
}

type Edge struct {
	From       string
	To         string
	Weight     float64
	Label      string
	Properties map[string]interface{}
}

type edgeItem struct {
	edge   *Edge
	sorted bool
}

type Graph struct {
	nodes     map[string]*Node
	outEdges  map[string][]*Edge
	inEdges   map[string][]*Edge
	outSorted map[string]bool
	inSorted  map[string]bool
	mu        sync.RWMutex
}

type PathResult struct {
	Nodes  []string
	Weight float64
}

func NewGraph() *Graph {
	return &Graph{
		nodes:     make(map[string]*Node),
		outEdges:  make(map[string][]*Edge),
		inEdges:   make(map[string][]*Edge),
		outSorted: make(map[string]bool),
		inSorted:  make(map[string]bool),
	}
}

func (g *Graph) AddNode(id string, properties map[string]interface{}) error {
	if id == "" {
		return ErrNodeNotFound
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[id]; exists {
		return ErrNodeExists
	}

	props := make(map[string]interface{})
	if properties != nil {
		for k, v := range properties {
			props[k] = v
		}
	}

	g.nodes[id] = &Node{
		ID:         id,
		Properties: props,
	}
	g.outEdges[id] = []*Edge{}
	g.inEdges[id] = []*Edge{}
	g.outSorted[id] = true
	g.inSorted[id] = true

	return nil
}

func (g *Graph) RemoveNode(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[id]; !exists {
		return ErrNodeNotFound
	}

	for _, edge := range g.outEdges[id] {
		g.removeFromInEdges(edge.To, id)
	}
	delete(g.outEdges, id)
	delete(g.outSorted, id)

	for _, edge := range g.inEdges[id] {
		g.removeFromOutEdges(edge.From, id)
	}
	delete(g.inEdges, id)
	delete(g.inSorted, id)

	delete(g.nodes, id)

	return nil
}

func (g *Graph) removeFromOutEdges(from, to string) {
	edges := g.outEdges[from]
	for i, e := range edges {
		if e.To == to {
			g.outEdges[from] = append(edges[:i], edges[i+1:]...)
			return
		}
	}
}

func (g *Graph) removeFromInEdges(to, from string) {
	edges := g.inEdges[to]
	for i, e := range edges {
		if e.From == from {
			g.inEdges[to] = append(edges[:i], edges[i+1:]...)
			return
		}
	}
}

func (g *Graph) HasNode(id string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, exists := g.nodes[id]
	return exists
}

func (g *Graph) GetNode(id string) (*Node, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	node, exists := g.nodes[id]
	if !exists {
		return nil, ErrNodeNotFound
	}
	props := make(map[string]interface{})
	for k, v := range node.Properties {
		props[k] = v
	}
	return &Node{
		ID:         node.ID,
		Properties: props,
	}, nil
}

func (g *Graph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

func (g *Graph) AddEdge(from, to string, weight float64, label string, properties map[string]interface{}) error {
	if from == to {
		return ErrSelfLoop
	}
	if weight < 0 {
		return ErrNegativeWeight
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[from]; !exists {
		return ErrInvalidStartNode
	}
	if _, exists := g.nodes[to]; !exists {
		return ErrInvalidEndNode
	}

	for _, e := range g.outEdges[from] {
		if e.To == to {
			return ErrEdgeExists
		}
	}

	props := make(map[string]interface{})
	if properties != nil {
		for k, v := range properties {
			props[k] = v
		}
	}

	edge := &Edge{
		From:       from,
		To:         to,
		Weight:     weight,
		Label:      label,
		Properties: props,
	}

	g.outEdges[from] = append(g.outEdges[from], edge)
	g.inEdges[to] = append(g.inEdges[to], edge)
	g.outSorted[from] = false
	g.inSorted[to] = false

	return nil
}

func (g *Graph) RemoveEdge(from, to string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[from]; !exists {
		return ErrInvalidStartNode
	}
	if _, exists := g.nodes[to]; !exists {
		return ErrInvalidEndNode
	}

	found := false
	outList := g.outEdges[from]
	for i, e := range outList {
		if e.To == to {
			g.outEdges[from] = append(outList[:i], outList[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return ErrEdgeNotFound
	}

	inList := g.inEdges[to]
	for i, e := range inList {
		if e.From == from {
			g.inEdges[to] = append(inList[:i], inList[i+1:]...)
			break
		}
	}

	return nil
}

func (g *Graph) HasEdge(from, to string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, e := range g.outEdges[from] {
		if e.To == to {
			return true
		}
	}
	return false
}

func (g *Graph) GetEdge(from, to string) (*Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, e := range g.outEdges[from] {
		if e.To == to {
			props := make(map[string]interface{})
			for k, v := range e.Properties {
				props[k] = v
			}
			return &Edge{
				From:       e.From,
				To:         e.To,
				Weight:     e.Weight,
				Label:      e.Label,
				Properties: props,
			}, nil
		}
	}
	return nil, ErrEdgeNotFound
}

func (g *Graph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	count := 0
	for _, edges := range g.outEdges {
		count += len(edges)
	}
	return count
}

func (g *Graph) sortEdges(edges []*Edge) {
	sort.Slice(edges, func(i, j int) bool {
		return edges[i].Weight < edges[j].Weight
	})
}

func (g *Graph) GetOutEdges(nodeID string) ([]*Edge, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[nodeID]; !exists {
		return nil, ErrNodeNotFound
	}

	if !g.outSorted[nodeID] {
		g.sortEdges(g.outEdges[nodeID])
		g.outSorted[nodeID] = true
	}

	result := make([]*Edge, 0, len(g.outEdges[nodeID]))
	for _, e := range g.outEdges[nodeID] {
		props := make(map[string]interface{})
		for k, v := range e.Properties {
			props[k] = v
		}
		result = append(result, &Edge{
			From:       e.From,
			To:         e.To,
			Weight:     e.Weight,
			Label:      e.Label,
			Properties: props,
		})
	}
	return result, nil
}

func (g *Graph) GetInEdges(nodeID string) ([]*Edge, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, exists := g.nodes[nodeID]; !exists {
		return nil, ErrNodeNotFound
	}

	if !g.inSorted[nodeID] {
		g.sortEdges(g.inEdges[nodeID])
		g.inSorted[nodeID] = true
	}

	result := make([]*Edge, 0, len(g.inEdges[nodeID]))
	for _, e := range g.inEdges[nodeID] {
		props := make(map[string]interface{})
		for k, v := range e.Properties {
			props[k] = v
		}
		result = append(result, &Edge{
			From:       e.From,
			To:         e.To,
			Weight:     e.Weight,
			Label:      e.Label,
			Properties: props,
		})
	}
	return result, nil
}

func (g *Graph) BFS(start string, maxDepth int) ([]string, error) {
	if maxDepth <= 0 {
		return nil, ErrMaxDepthNonPositive
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, exists := g.nodes[start]; !exists {
		return nil, ErrNodeNotFound
	}

	visited := make(map[string]bool)
	visited[start] = true
	result := []string{start}

	type queueItem struct {
		nodeID string
		depth  int
	}
	queue := []queueItem{{nodeID: start, depth: 0}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.depth >= maxDepth {
			continue
		}

		for _, e := range g.outEdges[current.nodeID] {
			if !visited[e.To] {
				visited[e.To] = true
				result = append(result, e.To)
				queue = append(queue, queueItem{nodeID: e.To, depth: current.depth + 1})
			}
		}
	}

	return result, nil
}

func (g *Graph) DFS(start string, maxDepth int) ([]string, error) {
	if maxDepth <= 0 {
		return nil, ErrMaxDepthNonPositive
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, exists := g.nodes[start]; !exists {
		return nil, ErrNodeNotFound
	}

	visited := make(map[string]bool)
	result := []string{}

	var dfs func(nodeID string, depth int)
	dfs = func(nodeID string, depth int) {
		visited[nodeID] = true
		result = append(result, nodeID)

		if depth >= maxDepth {
			return
		}

		for _, e := range g.outEdges[nodeID] {
			if !visited[e.To] {
				dfs(e.To, depth+1)
			}
		}
	}

	dfs(start, 0)
	return result, nil
}

type pqItem struct {
	nodeID string
	dist   float64
	index  int
}

type priorityQueue []*pqItem

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].dist < pq[j].dist
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*pqItem)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

func (g *Graph) ShortestPath(from, to string) (*PathResult, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, exists := g.nodes[from]; !exists {
		return nil, ErrInvalidStartNode
	}
	if _, exists := g.nodes[to]; !exists {
		return nil, ErrInvalidEndNode
	}
	if from == to {
		return &PathResult{
			Nodes:  []string{from},
			Weight: 0,
		}, nil
	}

	dist := make(map[string]float64)
	prev := make(map[string]string)
	visited := make(map[string]bool)

	for id := range g.nodes {
		dist[id] = 1e18
	}
	dist[from] = 0

	pq := &priorityQueue{}
	heap.Init(pq)
	heap.Push(pq, &pqItem{nodeID: from, dist: 0})

	for pq.Len() > 0 {
		current := heap.Pop(pq).(*pqItem)
		u := current.nodeID

		if visited[u] {
			continue
		}
		visited[u] = true

		if u == to {
			break
		}

		for _, e := range g.outEdges[u] {
			v := e.To
			if !visited[v] {
				alt := dist[u] + e.Weight
				if alt < dist[v] {
					dist[v] = alt
					prev[v] = u
					heap.Push(pq, &pqItem{nodeID: v, dist: alt})
				}
			}
		}
	}

	if dist[to] == 1e18 {
		return nil, ErrNoPath
	}

	path := []string{}
	current := to
	for current != from {
		path = append([]string{current}, path...)
		current = prev[current]
	}
	path = append([]string{from}, path...)

	return &PathResult{
		Nodes:  path,
		Weight: dist[to],
	}, nil
}
