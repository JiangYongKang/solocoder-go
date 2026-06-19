package workflow

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

var (
	nodeIDCounter int64
	nodeIDMu      sync.Mutex
)

func GenerateNodeID() string {
	nodeIDMu.Lock()
	defer nodeIDMu.Unlock()
	nodeIDCounter++
	return fmt.Sprintf("node_%d_%s", nodeIDCounter, randomHex(4))
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

type baseNode struct {
	ID          string
	Name        string
	Type        NodeType
	RetryConfig RetryConfig
}

func newBaseNode(nodeType NodeType, name string) baseNode {
	if name == "" {
		name = string(nodeType)
	}
	return baseNode{
		ID:          GenerateNodeID(),
		Name:        name,
		Type:        nodeType,
		RetryConfig: DefaultRetryConfig(),
	}
}

func (n *baseNode) GetID() string {
	return n.ID
}

func (n *baseNode) GetType() NodeType {
	return n.Type
}

func (n *baseNode) GetName() string {
	return n.Name
}

func (n *baseNode) SetName(name string) {
	n.Name = name
}

func (n *baseNode) GetRetryConfig() RetryConfig {
	return n.RetryConfig
}

func (n *baseNode) SetRetryConfig(cfg RetryConfig) {
	n.RetryConfig = cfg
}

func executeWithRetry(ctx context.Context, node Node, execCtx *ExecutionContext, executeFn func(context.Context, *ExecutionContext) (*NodeResult, error)) (*NodeResult, error) {
	cfg := node.GetRetryConfig()
	if cfg.MaxRetries <= 0 {
		return executeFn(ctx, execCtx)
	}

	var lastResult *NodeResult
	var lastErr error
	retryCount := 0

	for retryCount <= cfg.MaxRetries {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		result, err := executeFn(ctx, execCtx)
		lastResult = result
		lastErr = err

		if err == nil && result != nil && result.Status == NodeStatusCompleted {
			result.RetryCount = retryCount
			return result, nil
		}

		retryCount++
		if retryCount > cfg.MaxRetries {
			break
		}

		interval := calculateRetryInterval(cfg, retryCount)
		if interval > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(interval):
			}
		}
	}

	if lastResult != nil {
		lastResult.RetryCount = retryCount - 1
	}
	return lastResult, lastErr
}

func calculateRetryInterval(cfg RetryConfig, attempt int) time.Duration {
	base := cfg.Interval
	if base < 0 {
		base = 100 * time.Millisecond
	}

	if base == 0 {
		return 0
	}

	switch cfg.Strategy {
	case RetryLinear:
		factor := float64(attempt)
		if cfg.BackoffFactor > 0 {
			factor = cfg.BackoffFactor * float64(attempt)
		}
		return time.Duration(float64(base) * factor)
	case RetryExponential:
		factor := 1.0
		if cfg.BackoffFactor > 0 {
			factor = cfg.BackoffFactor
		}
		return time.Duration(float64(base) * powFloat(factor, attempt-1))
	default:
		return base
	}
}

func powFloat(base float64, exp int) float64 {
	result := 1.0
	for i := 0; i < exp; i++ {
		result *= base
	}
	return result
}

func newNodeResult(nodeID string) *NodeResult {
	return &NodeResult{
		NodeID:    nodeID,
		Status:    NodeStatusRunning,
		StartedAt: time.Now(),
	}
}

func completeResult(result *NodeResult, output interface{}, err error) *NodeResult {
	result.FinishedAt = time.Now()
	result.Duration = result.FinishedAt.Sub(result.StartedAt)
	if err != nil {
		result.Status = NodeStatusFailed
		result.Error = err.Error()
	} else {
		result.Status = NodeStatusCompleted
		result.Output = output
	}
	return result
}
