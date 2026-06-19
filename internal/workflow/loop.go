package workflow

import (
	"context"
	"errors"
	"fmt"
)

type LoopType string

const (
	LoopTypeFixed    LoopType = "fixed"
	LoopTypeDynamic  LoopType = "dynamic"
)

type LoopNode struct {
	baseNode
	Node            Node
	LoopType        LoopType
	Iterations      int
	IterationsKey   string
	ContinueOnError bool
	currentIteration int
}

func NewLoopNode(name string, node Node) *LoopNode {
	return &LoopNode{
		baseNode:         newBaseNode(NodeTypeLoop, name),
		Node:             node,
		LoopType:         LoopTypeFixed,
		Iterations:       1,
		ContinueOnError:  false,
		currentIteration: 0,
	}
}

func (n *LoopNode) SetFixedIterations(iterations int) *LoopNode {
	n.LoopType = LoopTypeFixed
	n.Iterations = iterations
	return n
}

func (n *LoopNode) SetDynamicIterations(key string) *LoopNode {
	n.LoopType = LoopTypeDynamic
	n.IterationsKey = key
	return n
}

func (n *LoopNode) SetContinueOnError(continueOnError bool) *LoopNode {
	n.ContinueOnError = continueOnError
	return n
}

func (n *LoopNode) getIterations(execCtx *ExecutionContext) (int, error) {
	switch n.LoopType {
	case LoopTypeDynamic:
		if n.IterationsKey == "" {
			return 0, fmt.Errorf("dynamic loop requires iterations key")
		}
		val, ok := execCtx.GetInt(n.IterationsKey)
		if !ok {
			return 0, fmt.Errorf("iterations key %s not found in context", n.IterationsKey)
		}
		if val < 0 {
			return 0, fmt.Errorf("iterations cannot be negative: %d", val)
		}
		return val, nil
	default:
		if n.Iterations < 0 {
			return 0, fmt.Errorf("iterations cannot be negative: %d", n.Iterations)
		}
		return n.Iterations, nil
	}
}

func (n *LoopNode) Execute(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
	return n.ExecuteWithState(ctx, execCtx, nil)
}

func (n *LoopNode) ExecuteWithState(ctx context.Context, execCtx *ExecutionContext, nodeState *NodeExecutionState) (*NodeResult, error) {
	return executeWithRetry(ctx, n, execCtx, func(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
		result := newNodeResult(n.ID)
		wfState := GetWorkflowState(ctx)

		if nodeState == nil {
			n.currentIteration = 0
		} else if nodeState.InternalState != nil {
			if state, ok := nodeState.InternalState.(map[string]interface{}); ok {
				if iter, ok := state["current_iteration"].(int); ok {
					n.currentIteration = iter
				}
			}
		}

		iterations, err := n.getIterations(execCtx)
		if err != nil {
			return completeResult(result, nil, err), err
		}

		iterationResults := make([]map[string]interface{}, 0, iterations)
		hasErrors := false
		lastError := ""

		iterationIndexKey := fmt.Sprintf("iteration_index_%s", n.ID)
		iterationCountKey := fmt.Sprintf("iteration_count_%s", n.ID)

		startIteration := n.currentIteration
		for i := startIteration; i < iterations; i++ {
			if ctx.Err() != nil {
				return completeResult(result, iterationResults, ctx.Err()), ctx.Err()
			}

			n.currentIteration = i

			execCtx.Set(iterationIndexKey, i)
			execCtx.Set(iterationCountKey, iterations)

			childExecCtx := execCtx.Clone()
			childExecCtx.Set("iteration_index", i)
			childExecCtx.Set("iteration_count", iterations)

			var childResult *NodeResult
			var childErr error

			childNodeKey := fmt.Sprintf("%s_iter_%d", n.Node.GetID(), i)
			if wfState != nil && isNodeCompleted(childNodeKey, wfState) {
				childResult = getCompletedNodeResult(childNodeKey, wfState)
			} else if wfState != nil {
				childResult, childErr = executeNodeWithState(ctx, n.Node, childExecCtx, wfState)
				if childResult != nil && childResult.Status == NodeStatusCompleted {
					wfState.NodeStates[childNodeKey] = &NodeExecutionState{
						NodeID:    childNodeKey,
						Completed: true,
						Result:    childResult,
					}
				}
			} else {
				childResult, childErr = n.Node.Execute(ctx, childExecCtx)
			}

			iterResult := map[string]interface{}{
				"iteration": i,
			}
			if childResult != nil {
				iterResult["result"] = childResult
			}
			if childErr != nil {
				iterResult["error"] = childErr.Error()
				hasErrors = true
				lastError = childErr.Error()
			} else if childResult != nil && childResult.Status == NodeStatusFailed {
				iterResult["error"] = childResult.Error
				hasErrors = true
				lastError = childResult.Error
			}

			iterationResults = append(iterationResults, iterResult)

			if (childErr != nil || (childResult != nil && childResult.Status == NodeStatusFailed)) && !n.ContinueOnError {
				result.Status = NodeStatusFailed
				result.Error = fmt.Sprintf("loop failed at iteration %d: %s", i, lastError)
				completeResult(result, iterationResults, errors.New(result.Error))
				return result, errors.New(result.Error)
			}

			for k, v := range childExecCtx.Values() {
				if k != "iteration_index" && k != "iteration_count" {
					execCtx.Set(k, v)
				}
			}
		}

		n.currentIteration = iterations

		output := map[string]interface{}{
			"total_iterations":  iterations,
			"iteration_results": iterationResults,
			"loop_type":         n.LoopType,
		}

		if hasErrors && n.ContinueOnError {
			output["has_errors"] = true
			output["last_error"] = lastError
			return completeResult(result, output, fmt.Errorf("loop completed with errors: %s", lastError)), fmt.Errorf("loop completed with errors")
		}

		output["has_errors"] = false
		return completeResult(result, output, nil), nil
	})
}

func (n *LoopNode) GetState() NodeStateData {
	return map[string]interface{}{
		"node_id":          n.ID,
		"current_iteration": n.currentIteration,
	}
}

func (n *LoopNode) RestoreState(state NodeStateData) {
	if state == nil {
		return
	}
	if stateMap, ok := state.(map[string]interface{}); ok {
		if iter, ok := stateMap["current_iteration"].(int); ok {
			n.currentIteration = iter
		}
	}
}
