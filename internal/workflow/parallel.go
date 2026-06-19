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
	MaxFailures    int
	Timeout        time.Duration
}

func NewParallelNode(name string, nodes ...Node) *ParallelNode {
	return &ParallelNode{
		baseNode:    newBaseNode(NodeTypeParallel, name),
		Nodes:       nodes,
		MaxFailures: 0,
		Timeout:     0,
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
	return executeWithRetry(ctx, n, execCtx, func(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
		result := newNodeResult(n.ID)
		childResults := make(map[string]*NodeResult)
		childResultsMu := sync.Mutex{}

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

				childResult, err := node.Execute(ctx, execCtxForChildren)

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

		return completeResult(result, childResults, nil), nil
	})
}
