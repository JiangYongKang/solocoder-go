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
	return executeWithRetry(ctx, n, execCtx, func(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
		result := newNodeResult(n.ID)
		execCtx.Set(n.Key, n.Value)
		return completeResult(result, map[string]interface{}{n.Key: n.Value}, nil), nil
	})
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
	return executeWithRetry(ctx, n, execCtx, func(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error) {
		result := newNodeResult(n.ID)
		return completeResult(result, nil, n.Error), n.Error
	})
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

func (n *FlakyTask) Reset() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.failures = 0
}
