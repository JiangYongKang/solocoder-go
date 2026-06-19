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
	Branches        []Branch
	DefaultBranch   Node
	selectedNodeID  string
	executed        bool
}

func NewConditionalNode(name string) *ConditionalNode {
	return &ConditionalNode{
		baseNode:       newBaseNode(NodeTypeConditional, name),
		selectedNodeID: "",
		executed:       false,
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
	return n.ExecuteWithState(ctx, execCtx, nil)
}

func (n *ConditionalNode) ExecuteWithState(ctx context.Context, execCtx *ExecutionContext, nodeState *NodeExecutionState) (*NodeResult, error) {
	return executeWithRetry(ctx, n, execCtx, func(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
		result := newNodeResult(n.ID)
		wfState := GetWorkflowState(ctx)

		if nodeState == nil {
			n.selectedNodeID = ""
			n.executed = false
		} else if nodeState.InternalState != nil {
			if state, ok := nodeState.InternalState.(map[string]interface{}); ok {
				if selected, ok := state["selected_node_id"].(string); ok {
					n.selectedNodeID = selected
				}
				if executed, ok := state["executed"].(bool); ok {
					n.executed = executed
				}
			}
		}

		if ctx.Err() != nil {
			return completeResult(result, nil, ctx.Err()), ctx.Err()
		}

		var selectedNode Node
		var matchedCondition *Condition

		if nodeState != nil && n.executed && n.selectedNodeID != "" {
			for _, branch := range n.Branches {
				if branch.Node.GetID() == n.selectedNodeID {
					selectedNode = branch.Node
					matchedCondition = branch.Condition
					break
				}
			}
			if selectedNode == nil && n.DefaultBranch != nil && n.DefaultBranch.GetID() == n.selectedNodeID {
				selectedNode = n.DefaultBranch
			}
		}

		if selectedNode == nil {
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
		}

		if selectedNode == nil {
			err := fmt.Errorf("no matching branch and no default branch configured")
			result.Error = err.Error()
			return completeResult(result, nil, err), err
		}

		n.selectedNodeID = selectedNode.GetID()
		n.executed = true

		output := map[string]interface{}{
			"selected_branch":   selectedNode.GetName(),
			"selected_node_id":  selectedNode.GetID(),
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

		var childResult *NodeResult
		var err error

		if wfState != nil && isNodeCompleted(selectedNode.GetID(), wfState) {
			childResult = getCompletedNodeResult(selectedNode.GetID(), wfState)
		} else if wfState != nil {
			childResult, err = executeNodeWithState(ctx, selectedNode, execCtx, wfState)
		} else {
			childResult, err = selectedNode.Execute(ctx, execCtx)
		}

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

func (n *ConditionalNode) GetState() NodeStateData {
	return map[string]interface{}{
		"node_id":         n.ID,
		"selected_node_id": n.selectedNodeID,
		"executed":        n.executed,
	}
}

func (n *ConditionalNode) RestoreState(state NodeStateData) {
	if state == nil {
		return
	}
	if stateMap, ok := state.(map[string]interface{}); ok {
		if selected, ok := stateMap["selected_node_id"].(string); ok {
			n.selectedNodeID = selected
		}
		if executed, ok := stateMap["executed"].(bool); ok {
			n.executed = executed
		}
	}
}
