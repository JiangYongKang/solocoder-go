package workflow

import (
	"context"
	"errors"
	"fmt"
)

type SequentialNode struct {
	baseNode
	Nodes []Node
}

func NewSequentialNode(name string, nodes ...Node) *SequentialNode {
	return &SequentialNode{
		baseNode: newBaseNode(NodeTypeSequential, name),
		Nodes:    nodes,
	}
}

func (n *SequentialNode) AddNode(node Node) *SequentialNode {
	n.Nodes = append(n.Nodes, node)
	return n
}

func (n *SequentialNode) Execute(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
	return executeWithRetry(ctx, n, execCtx, func(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
		result := newNodeResult(n.ID)
		childResults := make(map[string]*NodeResult)

		for i, node := range n.Nodes {
			if ctx.Err() != nil {
				return completeResult(result, childResults, ctx.Err()), ctx.Err()
			}

			childResult, err := node.Execute(ctx, execCtx)
			if childResult != nil {
				childResults[node.GetID()] = childResult
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

		return completeResult(result, childResults, nil), nil
	})
}

type SequentialNodeState struct {
	NodeID       string
	CurrentIndex int
	ChildStates  map[string]*NodeResult
}

func (n *SequentialNode) GetState() *SequentialNodeState {
	return &SequentialNodeState{
		NodeID:       n.ID,
		CurrentIndex: 0,
		ChildStates:  make(map[string]*NodeResult),
	}
}

func (n *SequentialNode) RestoreState(state *SequentialNodeState) {
	if state == nil {
		return
	}
}
