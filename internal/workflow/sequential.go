package workflow

import (
	"context"
	"errors"
	"fmt"
)

type SequentialNode struct {
	baseNode
	Nodes        []Node
	currentIndex int
}

func NewSequentialNode(name string, nodes ...Node) *SequentialNode {
	return &SequentialNode{
		baseNode:     newBaseNode(NodeTypeSequential, name),
		Nodes:        nodes,
		currentIndex: 0,
	}
}

func (n *SequentialNode) AddNode(node Node) *SequentialNode {
	n.Nodes = append(n.Nodes, node)
	return n
}

func (n *SequentialNode) Execute(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
	return n.ExecuteWithState(ctx, execCtx, nil)
}

func (n *SequentialNode) ExecuteWithState(ctx context.Context, execCtx *ExecutionContext, nodeState *NodeExecutionState) (*NodeResult, error) {
	return executeWithRetry(ctx, n, execCtx, func(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
		result := newNodeResult(n.ID)
		childResults := make(map[string]*NodeResult)
		wfState := GetWorkflowState(ctx)

		startIndex := n.currentIndex
		if nodeState == nil {
			n.currentIndex = 0
			startIndex = 0
		} else if nodeState.InternalState != nil {
			if state, ok := nodeState.InternalState.(map[string]interface{}); ok {
				if idx, ok := state["current_index"].(int); ok {
					startIndex = idx
					n.currentIndex = idx
				}
			}
		}

		for i := startIndex; i < len(n.Nodes); i++ {
			if ctx.Err() != nil {
				return completeResult(result, childResults, ctx.Err()), ctx.Err()
			}

			node := n.Nodes[i]
			n.currentIndex = i

			var childResult *NodeResult
			var err error

			if wfState != nil && isNodeCompleted(node.GetID(), wfState) {
				childResult = getCompletedNodeResult(node.GetID(), wfState)
				if childResult != nil {
					childResults[node.GetID()] = childResult
				}
			} else if wfState != nil {
				childResult, err = executeNodeWithState(ctx, node, execCtx, wfState)
				if childResult != nil {
					childResults[node.GetID()] = childResult
				}
			} else {
				childResult, err = node.Execute(ctx, execCtx)
				if childResult != nil {
					childResults[node.GetID()] = childResult
				}
			}

			if err != nil || (childResult != nil && childResult.Status == NodeStatusFailed) {
				result.Status = NodeStatusFailed
				if err != nil {
					result.Error = fmt.Sprintf("node %d (%s) failed: %v", i, node.GetName(), err)
				} else if childResult != nil {
					result.Error = fmt.Sprintf("node %d (%s) failed: %s", i, node.GetName(), childResult.Error)
				}
				completeResult(result, childResults, errors.New(result.Error))
				return result, errors.New(result.Error)
			}
		}

		n.currentIndex = len(n.Nodes)
		return completeResult(result, childResults, nil), nil
	})
}

func (n *SequentialNode) GetState() NodeStateData {
	return map[string]interface{}{
		"node_id":        n.ID,
		"current_index":  n.currentIndex,
	}
}

func (n *SequentialNode) RestoreState(state NodeStateData) {
	if state == nil {
		return
	}
	if stateMap, ok := state.(map[string]interface{}); ok {
		if idx, ok := stateMap["current_index"].(int); ok {
			n.currentIndex = idx
		}
	}
}
