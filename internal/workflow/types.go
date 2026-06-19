package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrWorkflowNotFound      = errors.New("workflow: workflow not found")
	ErrNodeNotFound        = errors.New("workflow: node not found")
	ErrInvalidState         = errors.New("workflow: invalid state")
	ErrConditionNotMet     = errors.New("workflow: condition not met")
	ErrMaxFailuresExceeded = errors.New("workflow: max failures exceeded")
	ErrWorkflowCanceled     = errors.New("workflow: workflow canceled")
)

func asInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int8:
		return int(val), true
	case int16:
		return int(val), true
	case int32:
		return int(val), true
	case int64:
		return int(val), true
	case uint:
		return int(val), true
	case uint8:
		return int(val), true
	case uint16:
		return int(val), true
	case uint32:
		return int(val), true
	case uint64:
		return int(val), true
	case float32:
		return int(val), true
	case float64:
		return int(val), true
	case string:
		var i int
		_, err := fmt.Sscanf(val, "%d", &i)
		return i, err == nil
	default:
		return 0, false
	}
}

type NodeType string

const (
	NodeTypeTask        NodeType = "task"
	NodeTypeSequential  NodeType = "sequential"
	NodeTypeParallel    NodeType = "parallel"
	NodeTypeConditional NodeType = "conditional"
	NodeTypeLoop        NodeType = "loop"
)

type NodeStatus string

const (
	NodeStatusPending   NodeStatus = "pending"
	NodeStatusRunning   NodeStatus = "running"
	NodeStatusCompleted NodeStatus = "completed"
	NodeStatusFailed    NodeStatus = "failed"
	NodeStatusSkipped   NodeStatus = "skipped"
)

type WorkflowStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "pending"
	WorkflowStatusRunning WorkflowStatus = "running"
	WorkflowStatusCompleted WorkflowStatus = "completed"
	WorkflowStatusFailed    WorkflowStatus = "failed"
	WorkflowStatusPaused    WorkflowStatus = "paused"
)

type RetryStrategy string

const (
	RetryFixed    RetryStrategy = "fixed"
	RetryLinear  RetryStrategy = "linear"
	RetryExponential RetryStrategy = "exponential"
)

type RetryConfig struct {
	MaxRetries  int
	Interval    time.Duration
	Strategy  RetryStrategy
	BackoffFactor float64
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:  0,
		Interval:    0,
		Strategy:  RetryFixed,
		BackoffFactor: 1.0,
	}
}

type NodeResult struct {
	NodeID     string
	Status     NodeStatus
	Output     interface{}
	Error      string
	RetryCount int
	Duration   time.Duration
	StartedAt  time.Time
	FinishedAt time.Time
}

type Condition struct {
	Field    string
	Operator string
	Value    interface{}
}

func (c *Condition) Evaluate(ctx *ExecutionContext) bool {
	if c == nil || (c.Field == "" && c.Operator == "" && c.Value == nil) {
		return true
	}

	actual, ok := ctx.Get(c.Field)
	if !ok {
		return false
	}

	switch strings.ToLower(c.Operator) {
	case "eq", "==":
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", c.Value)
	case "ne", "!=":
		return fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", c.Value)
	case "contains":
		actualStr := fmt.Sprintf("%v", actual)
		valueStr := fmt.Sprintf("%v", c.Value)
		return strings.Contains(actualStr, valueStr)
	case "notcontains":
		actualStr := fmt.Sprintf("%v", actual)
		valueStr := fmt.Sprintf("%v", c.Value)
		return !strings.Contains(actualStr, valueStr)
	case "gt", ">":
		return compareNumbers(actual, c.Value) > 0
	case "gte", ">=":
		return compareNumbers(actual, c.Value) >= 0
	case "lt", "<":
		return compareNumbers(actual, c.Value) < 0
	case "lte", "<=":
		return compareNumbers(actual, c.Value) <= 0
	default:
		return false
	}
}

func compareNumbers(a, b interface{}) int {
	af, aok := toFloat64(a)
	bf, bok := toFloat64(b)
	if !aok || !bok {
		return -2
	}
	if af > bf {
		return 1
	} else if af < bf {
		return -1
	}
	return 0
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case int:
		return float64(val), true
	case int8:
		return float64(val), true
	case int16:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint8:
		return float64(val), true
	case uint16:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint64:
		return float64(val), true
	case float32:
		return float64(val), true
	case float64:
		return val, true
	case string:
		var f float64
		_, err := fmt.Sscanf(val, "%f", &f)
		return f, err == nil
	default:
		return 0, false
	}
}

type ExecutionContext struct {
	mu     sync.RWMutex
	values map[string]interface{}
}

func NewExecutionContext() *ExecutionContext {
	return &ExecutionContext{
		values: make(map[string]interface{}),
	}
}

func (ctx *ExecutionContext) Set(key string, value interface{}) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.values[key] = value
}

func (ctx *ExecutionContext) Get(key string) (interface{}, bool) {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	val, ok := ctx.values[key]
	return val, ok
}

func (ctx *ExecutionContext) GetString(key string) (string, bool) {
	val, ok := ctx.Get(key)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%v", val), true
}

func (ctx *ExecutionContext) GetInt(key string) (int, bool) {
	val, ok := ctx.Get(key)
	if !ok {
		return 0, false
	}
	f, ok := toFloat64(val)
	if !ok {
		return 0, false
	}
	return int(f), true
}

func (ctx *ExecutionContext) Clone() *ExecutionContext {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	newCtx := NewExecutionContext()
	for k, v := range ctx.values {
		newCtx.values[k] = v
	}
	return newCtx
}

func (ctx *ExecutionContext) Values() map[string]interface{} {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	result := make(map[string]interface{})
	for k, v := range ctx.values {
		result[k] = v
	}
	return result
}

type NodeStateData interface{}

type NodeExecutionState struct {
	NodeID        string
	Completed     bool
	Result        *NodeResult
	InternalState NodeStateData
}

type workflowStateKey struct{}

func WithWorkflowState(ctx context.Context, state *WorkflowState) context.Context {
	return context.WithValue(ctx, workflowStateKey{}, state)
}

func GetWorkflowState(ctx context.Context) *WorkflowState {
	if state, ok := ctx.Value(workflowStateKey{}).(*WorkflowState); ok {
		return state
	}
	return nil
}

type Node interface {
	GetID() string
	GetType() NodeType
	GetName() string
	SetName(string)
	GetRetryConfig() RetryConfig
	SetRetryConfig(RetryConfig)
	Execute(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error)
	ExecuteWithState(ctx context.Context, execCtx *ExecutionContext, nodeState *NodeExecutionState) (*NodeResult, error)
	GetState() NodeStateData
	RestoreState(state NodeStateData)
}

type WorkflowState struct {
	WorkflowID    string
	Status        WorkflowStatus
	CurrentNodeID string
	CompletedNodes []string
	NodeResults   map[string]*NodeResult
	NodeStates    map[string]*NodeExecutionState
	Context       map[string]interface{}
	Error         string
	CreatedAt     time.Time
	StartedAt     time.Time
	FinishedAt    time.Time
}

type NodeState struct {
	NodeID   string
	Name     string
	Type     NodeType
	Status   NodeStatus
	Result   *NodeResult
	Children []NodeState
}

type WorkflowDefinition struct {
	ID          string
	Name        string
	Description string
	RootNode    Node
	Version     string
}

type WorkflowResult struct {
	WorkflowID string
	Status     WorkflowStatus
	Results    map[string]*NodeResult
	Context    map[string]interface{}
	Error      string
	Duration   time.Duration
}
