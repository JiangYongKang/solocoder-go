package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type ParallelNode struct {
	baseNode
	Nodes           []Node
	MaxFailures     int
	Timeout         time.Duration
	completedNodes  map[string]bool
}

func NewParallelNode(name string, nodes ...Node) *ParallelNode {
	return &ParallelNode{
		baseNode:       newBaseNode(NodeTypeParallel, name),
		Nodes:          nodes,
		MaxFailures:    0,
		Timeout:        0,
		completedNodes: make(map[string]bool),
	}
}

func (n *ParallelNode) AddNode(node Node) *ParallelNode {
	n.Nodes = append(n.Nodes, node)
	return n
}

func (n *ParallelNode) SetMaxFailures(max int) *ParallelNode {
	n.MaxFailures = max
	return n
}

func (n *ParallelNode) SetTimeout(timeout time.Duration) *ParallelNode {
	n.Timeout = timeout
	return n
}

func (n *ParallelNode) Execute(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
	return n.ExecuteWithState(ctx, execCtx, nil)
}

func (n *ParallelNode) ExecuteWithState(ctx context.Context, execCtx *ExecutionContext, nodeState *NodeExecutionState) (*NodeResult, error) {
	return executeWithRetry(ctx, n, execCtx, func(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
		result := newNodeResult(n.ID)
		childResults := make(map[string]*NodeResult)
		childResultsMu := sync.Mutex{}
		wfState := GetWorkflowState(ctx)

		if nodeState == nil {
			n.completedNodes = make(map[string]bool)
		} else if nodeState.InternalState != nil {
			if state, ok := nodeState.InternalState.(map[string]interface{}); ok {
				if completed, ok := state["completed_nodes"].(map[string]interface{}); ok {
					n.completedNodes = make(map[string]bool)
					for k, v := range completed {
						if b, ok := v.(bool); ok {
							n.completedNodes[k] = b
						}
					}
				}
			}
		}

		if len(n.Nodes) == 0 {
			return completeResult(result, childResults, nil), nil
		}

		var wg sync.WaitGroup
		var failureCount int32
		var cancel context.CancelFunc
		execCtxForChildren := execCtx

		if n.Timeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, n.Timeout)
			defer cancel()
		}

		terminate := make(chan struct{})
		var terminated int32

		for _, node := range n.Nodes {
			nodeID := node.GetID()

			if wfState != nil && isNodeCompleted(nodeID, wfState) {
				childResult := getCompletedNodeResult(nodeID, wfState)
				if childResult != nil {
					childResultsMu.Lock()
					childResults[nodeID] = childResult
					childResultsMu.Unlock()
					n.completedNodes[nodeID] = true
				}
				continue
			}

			if n.completedNodes[nodeID] {
				continue
			}

			wg.Add(1)
			go func(node Node) {
				defer wg.Done()

				select {
				case <-terminate:
					childResultsMu.Lock()
					childResults[node.GetID()] = &NodeResult{
						NodeID:     node.GetID(),
						Status:     NodeStatusSkipped,
						Error:      "parallel group terminated due to too many failures",
						StartedAt:  time.Now(),
						FinishedAt: time.Now(),
					}
					childResultsMu.Unlock()
					return
				default:
				}

				var childResult *NodeResult
				var err error
				if wfState != nil {
					childResult, err = executeNodeWithState(ctx, node, execCtxForChildren, wfState)
				} else {
					childResult, err = node.Execute(ctx, execCtxForChildren)
				}

				childResultsMu.Lock()
				if childResult != nil {
					childResults[node.GetID()] = childResult
				} else {
					childResults[node.GetID()] = &NodeResult{
						NodeID:     node.GetID(),
						Status:     NodeStatusFailed,
						Error:      fmt.Sprintf("node execution error: %v", err),
						StartedAt:  time.Now(),
						FinishedAt: time.Now(),
					}
				}
				childResultsMu.Unlock()

				if err != nil || (childResult != nil && childResult.Status == NodeStatusFailed) {
					newCount := atomic.AddInt32(&failureCount, 1)
					if n.MaxFailures >= 0 && int(newCount) > n.MaxFailures {
						if atomic.CompareAndSwapInt32(&terminated, 0, 1) {
							close(terminate)
							if cancel != nil {
								cancel()
							}
						}
					}
				}
			}(node)
		}

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-ctx.Done():
		}

		finalFailureCount := atomic.LoadInt32(&failureCount)

		if n.MaxFailures >= 0 && int(finalFailureCount) > n.MaxFailures {
			result.Status = NodeStatusFailed
			result.Error = fmt.Sprintf("parallel group failed: %d failures exceeded max tolerance %d", finalFailureCount, n.MaxFailures)
			completeResult(result, childResults, errors.New(result.Error))
			return result, errors.New(result.Error)
		}

		if ctx.Err() != nil && ctx.Err() != context.Canceled {
			result.Status = NodeStatusFailed
			result.Error = fmt.Sprintf("parallel group timeout or canceled: %v", ctx.Err())
			completeResult(result, childResults, ctx.Err())
			return result, ctx.Err()
		}

		for id := range childResults {
			n.completedNodes[id] = true
		}

		return completeResult(result, childResults, nil), nil
	})
}

func (n *ParallelNode) GetState() NodeStateData {
	completed := make(map[string]bool)
	for k, v := range n.completedNodes {
		completed[k] = v
	}
	return map[string]interface{}{
		"node_id":         n.ID,
		"completed_nodes": completed,
	}
}

func (n *ParallelNode) RestoreState(state NodeStateData) {
	if state == nil {
		return
	}
	if stateMap, ok := state.(map[string]interface{}); ok {
		if completed, ok := stateMap["completed_nodes"].(map[string]interface{}); ok {
			n.completedNodes = make(map[string]bool)
			for k, v := range completed {
				if b, ok := v.(bool); ok {
					n.completedNodes[k] = b
				}
			}
		}
	}
}
