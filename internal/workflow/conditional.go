package workflow

import (
	"context"
	"errors"
	"fmt"
)

type Branch struct {
	Condition *Condition
	Node      Node
}

type ConditionalNode struct {
	baseNode
	Branches      []Branch
	DefaultBranch Node
}

func NewConditionalNode(name string) *ConditionalNode {
	return &ConditionalNode{
		baseNode: newBaseNode(NodeTypeConditional, name),
	}
}

func (n *ConditionalNode) AddBranch(condition *Condition, node Node) *ConditionalNode {
	n.Branches = append(n.Branches, Branch{
		Condition: condition,
		Node:      node,
	})
	return n
}

func (n *ConditionalNode) SetDefaultBranch(node Node) *ConditionalNode {
	n.DefaultBranch = node
	return n
}

func (n *ConditionalNode) Execute(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
	return executeWithRetry(ctx, n, execCtx, func(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
		result := newNodeResult(n.ID)

		if ctx.Err() != nil {
			return completeResult(result, nil, ctx.Err()), ctx.Err()
		}

		var selectedNode Node
		var matchedCondition *Condition

		for i := range n.Branches {
			branch := &n.Branches[i]
			if branch.Condition == nil || branch.Condition.Evaluate(execCtx) {
				selectedNode = branch.Node
				matchedCondition = branch.Condition
				break
			}
		}

		if selectedNode == nil && n.DefaultBranch != nil {
			selectedNode = n.DefaultBranch
		}

		if selectedNode == nil {
			err := fmt.Errorf("no matching branch and no default branch configured")
			result.Error = err.Error()
			return completeResult(result, nil, err), err
		}

		output := map[string]interface{}{
			"selected_branch": selectedNode.GetName(),
			"selected_node_id": selectedNode.GetID(),
		}
		if matchedCondition != nil {
			output["matched_condition"] = map[string]interface{}{
				"field":    matchedCondition.Field,
				"operator": matchedCondition.Operator,
				"value":    matchedCondition.Value,
			}
		} else if n.DefaultBranch != nil && selectedNode == n.DefaultBranch {
			output["default_branch"] = true
		}

		childResult, err := selectedNode.Execute(ctx, execCtx)
		if childResult != nil {
			output["branch_result"] = childResult
		}

		if err != nil || (childResult != nil && childResult.Status == NodeStatusFailed) {
			result.Output = output
			if err != nil {
				result.Error = fmt.Sprintf("branch execution failed: %v", err)
			} else if childResult != nil {
				result.Error = fmt.Sprintf("branch execution failed: %s", childResult.Error)
			}
			completeResult(result, output, errors.New(result.Error))
			return result, errors.New(result.Error)
		}

		return completeResult(result, output, nil), nil
	})
}
