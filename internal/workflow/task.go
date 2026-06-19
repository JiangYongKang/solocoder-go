package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type TaskHandler func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error)

type TaskNode struct {
	baseNode
	Handler TaskHandler
}

func NewTaskNode(name string, handler TaskHandler) *TaskNode {
	if handler == nil {
		handler = func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
			return nil, nil
		}
	}
	return &TaskNode{
		baseNode: newBaseNode(NodeTypeTask, name),
		Handler:  handler,
	}
}

func (n *TaskNode) Execute(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
	return n.ExecuteWithState(ctx, execCtx, nil)
}

func (n *TaskNode) ExecuteWithState(ctx context.Context, execCtx *ExecutionContext, nodeState *NodeExecutionState) (*NodeResult, error) {
	return executeWithRetry(ctx, n, execCtx, func(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
		result := newNodeResult(n.ID)

		if ctx.Err() != nil {
			return completeResult(result, nil, ctx.Err()), ctx.Err()
		}

		output, err := n.Handler(ctx, execCtx)
		if err != nil {
			return completeResult(result, nil, err), err
		}

		return completeResult(result, output, nil), nil
	})
}

func (n *TaskNode) GetState() NodeStateData {
	return nil
}

func (n *TaskNode) RestoreState(state NodeStateData) {
}

type CallbackTaskNode struct {
	baseNode
	OnExecute func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error)
}

func NewCallbackTaskNode(name string, fn func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error)) *CallbackTaskNode {
	return &CallbackTaskNode{
		baseNode:  newBaseNode(NodeTypeTask, name),
		OnExecute: fn,
	}
}

func (n *CallbackTaskNode) Execute(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
	return n.ExecuteWithState(ctx, execCtx, nil)
}

func (n *CallbackTaskNode) ExecuteWithState(ctx context.Context, execCtx *ExecutionContext, nodeState *NodeExecutionState) (*NodeResult, error) {
	if n.OnExecute == nil {
		result := newNodeResult(n.ID)
		return completeResult(result, nil, nil), nil
	}
	return executeWithRetry(ctx, n, execCtx, func(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
		result := newNodeResult(n.ID)
		output, err := n.OnExecute(ctx, execCtx)
		if err != nil {
			return completeResult(result, nil, err), err
		}
		return completeResult(result, output, nil), nil
	})
}

func (n *CallbackTaskNode) GetState() NodeStateData {
	return nil
}

func (n *CallbackTaskNode) RestoreState(state NodeStateData) {
}

type SetContextTask struct {
	baseNode
	Key   string
	Value interface{}
}

func NewSetContextTask(name string, key string, value interface{}) *SetContextTask {
	return &SetContextTask{
		baseNode: newBaseNode(NodeTypeTask, name),
		Key:      key,
		Value:    value,
	}
}

func (n *SetContextTask) Execute(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
	return n.ExecuteWithState(ctx, execCtx, nil)
}

func (n *SetContextTask) ExecuteWithState(ctx context.Context, execCtx *ExecutionContext, nodeState *NodeExecutionState) (*NodeResult, error) {
	return executeWithRetry(ctx, n, execCtx, func(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
		result := newNodeResult(n.ID)
		execCtx.Set(n.Key, n.Value)
		return completeResult(result, map[string]interface{}{n.Key: n.Value}, nil), nil
	})
}

func (n *SetContextTask) GetState() NodeStateData {
	return nil
}

func (n *SetContextTask) RestoreState(state NodeStateData) {
}

type FailTask struct {
	baseNode
	Error error
}

func NewFailTask(name string, err error) *FailTask {
	if err == nil {
		err = fmt.Errorf("task failed")
	}
	return &FailTask{
		baseNode: newBaseNode(NodeTypeTask, name),
		Error:    err,
	}
}

func (n *FailTask) Execute(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
	return n.ExecuteWithState(ctx, execCtx, nil)
}

func (n *FailTask) ExecuteWithState(ctx context.Context, execCtx *ExecutionContext, nodeState *NodeExecutionState) (*NodeResult, error) {
	return executeWithRetry(ctx, n, execCtx, func(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
		result := newNodeResult(n.ID)
		return completeResult(result, nil, n.Error), n.Error
	})
}

func (n *FailTask) GetState() NodeStateData {
	return nil
}

func (n *FailTask) RestoreState(state NodeStateData) {
}

type DelayTask struct {
	baseNode
	Duration time.Duration
	Output   interface{}
}

func NewDelayTask(name string, duration time.Duration, output interface{}) *DelayTask {
	return &DelayTask{
		baseNode: newBaseNode(NodeTypeTask, name),
		Duration: duration,
		Output:   output,
	}
}

func (n *DelayTask) Execute(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
	return n.ExecuteWithState(ctx, execCtx, nil)
}

func (n *DelayTask) ExecuteWithState(ctx context.Context, execCtx *ExecutionContext, nodeState *NodeExecutionState) (*NodeResult, error) {
	return executeWithRetry(ctx, n, execCtx, func(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
		result := newNodeResult(n.ID)
		if n.Duration > 0 {
			select {
			case <-ctx.Done():
				return completeResult(result, nil, ctx.Err()), ctx.Err()
			case <-time.After(n.Duration):
			}
		}
		return completeResult(result, n.Output, nil), nil
	})
}

func (n *DelayTask) GetState() NodeStateData {
	return nil
}

func (n *DelayTask) RestoreState(state NodeStateData) {
}

type FlakyTask struct {
	baseNode
	FailCount int
	Output    interface{}
	failures  int
	mu        sync.Mutex
}

func NewFlakyTask(name string, failCount int, output interface{}) *FlakyTask {
	return &FlakyTask{
		baseNode:  newBaseNode(NodeTypeTask, name),
		FailCount: failCount,
		Output:    output,
	}
}

func (n *FlakyTask) Execute(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
	return n.ExecuteWithState(ctx, execCtx, nil)
}

func (n *FlakyTask) ExecuteWithState(ctx context.Context, execCtx *ExecutionContext, nodeState *NodeExecutionState) (*NodeResult, error) {
	return executeWithRetry(ctx, n, execCtx, func(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
		n.mu.Lock()
		n.failures++
		currentFailures := n.failures
		n.mu.Unlock()

		result := newNodeResult(n.ID)
		if currentFailures <= n.FailCount {
			err := fmt.Errorf("flaky task failed on attempt %d", currentFailures)
			return completeResult(result, nil, err), err
		}
		return completeResult(result, n.Output, nil), nil
	})
}

func (n *FlakyTask) GetState() NodeStateData {
	n.mu.Lock()
	defer n.mu.Unlock()
	return map[string]interface{}{"failures": n.failures}
}

func (n *FlakyTask) RestoreState(state NodeStateData) {
	if state == nil {
		return
	}
	if stateMap, ok := state.(map[string]interface{}); ok {
		if failures, ok := stateMap["failures"].(int); ok {
			n.mu.Lock()
			n.failures = failures
			n.mu.Unlock()
		}
	}
}

func (n *FlakyTask) Reset() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.failures = 0
}
