package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type WorkflowEngine struct {
	mu          sync.RWMutex
	workflows   map[string]*WorkflowDefinition
	instances   map[string]*WorkflowState
}

func NewWorkflowEngine() *WorkflowEngine {
	return &WorkflowEngine{
		workflows: make(map[string]*WorkflowDefinition),
		instances: make(map[string]*WorkflowState),
	}
}

func (e *WorkflowEngine) RegisterWorkflow(wf *WorkflowDefinition) error {
	if wf == nil {
		return fmt.Errorf("workflow definition is nil")
	}
	if wf.ID == "" {
		return fmt.Errorf("workflow ID is required")
	}
	if wf.RootNode == nil {
		return fmt.Errorf("workflow root node is required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.workflows[wf.ID] = wf
	return nil
}

func (e *WorkflowEngine) GetWorkflow(id string) (*WorkflowDefinition, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	wf, ok := e.workflows[id]
	return wf, ok
}

func (e *WorkflowEngine) ListWorkflows() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	ids := make([]string, 0, len(e.workflows))
	for id := range e.workflows {
		ids = append(ids, id)
	}
	return ids
}

func (e *WorkflowEngine) StartWorkflow(ctx context.Context, workflowID string, initialContext map[string]interface{}) (*WorkflowState, error) {
	_, ok := e.GetWorkflow(workflowID)
	if !ok {
		return nil, ErrWorkflowNotFound
	}

	state := &WorkflowState{
		WorkflowID:     workflowID,
		Status:         WorkflowStatusRunning,
		NodeResults:    make(map[string]*NodeResult),
		NodeStates:     make(map[string]*NodeExecutionState),
		CompletedNodes: make([]string, 0),
		Context:        make(map[string]interface{}),
		CreatedAt:      time.Now(),
		StartedAt:      time.Now(),
	}

	if initialContext != nil {
		for k, v := range initialContext {
			state.Context[k] = v
		}
	}

	instanceID := fmt.Sprintf("%s_%d", workflowID, time.Now().UnixNano())
	e.mu.Lock()
	e.instances[instanceID] = state
	e.mu.Unlock()

	return state, nil
}

func (e *WorkflowEngine) ExecuteWorkflow(ctx context.Context, workflowID string, initialContext map[string]interface{}) (*WorkflowResult, error) {
	wf, ok := e.GetWorkflow(workflowID)
	if !ok {
		return nil, ErrWorkflowNotFound
	}

	execCtx := NewExecutionContext()
	if initialContext != nil {
		for k, v := range initialContext {
			execCtx.Set(k, v)
		}
	}

	startTime := time.Now()
	result := &WorkflowResult{
		WorkflowID: workflowID,
		Status:     WorkflowStatusRunning,
		Results:    make(map[string]*NodeResult),
	}

	rootResult, err := wf.RootNode.Execute(ctx, execCtx)
	if rootResult != nil {
		result.Results[wf.RootNode.GetID()] = rootResult
		collectNodeResults(rootResult, result.Results)
	}

	duration := time.Since(startTime)
	result.Duration = duration

	if err != nil || (rootResult != nil && rootResult.Status == NodeStatusFailed) {
		result.Status = WorkflowStatusFailed
		if err != nil {
			result.Error = err.Error()
		} else if rootResult != nil {
			result.Error = rootResult.Error
			err = fmt.Errorf("%s", result.Error)
		}
	} else {
		result.Status = WorkflowStatusCompleted
	}

	result.Context = execCtx.Values()

	return result, err
}

func collectNodeResults(result *NodeResult, results map[string]*NodeResult) {
	if result == nil {
		return
	}
	results[result.NodeID] = result

	if outputMap, ok := result.Output.(map[string]*NodeResult); ok {
		for _, childResult := range outputMap {
			collectNodeResults(childResult, results)
		}
	} else if outputMap, ok := result.Output.(map[string]interface{}); ok {
		for _, v := range outputMap {
			if childResult, ok := v.(*NodeResult); ok {
				collectNodeResults(childResult, results)
			}
		}
	}
}

func collectCompletedNodes(result *NodeResult, completedNodes *[]string) {
	if result == nil || result.Status != NodeStatusCompleted {
		return
	}
	if !containsString(*completedNodes, result.NodeID) {
		*completedNodes = append(*completedNodes, result.NodeID)
	}

	if outputMap, ok := result.Output.(map[string]*NodeResult); ok {
		for _, childResult := range outputMap {
			collectCompletedNodes(childResult, completedNodes)
		}
	} else if outputMap, ok := result.Output.(map[string]interface{}); ok {
		for _, v := range outputMap {
			if childResult, ok := v.(*NodeResult); ok {
				collectCompletedNodes(childResult, completedNodes)
			}
		}
	}
}

func executeNodeWithState(ctx context.Context, node Node, execCtx *ExecutionContext, state *WorkflowState) (*NodeResult, error) {
	nodeID := node.GetID()

	if nodeState, exists := state.NodeStates[nodeID]; exists && nodeState.Completed {
		if nodeState.Result != nil {
			collectNodeResults(nodeState.Result, state.NodeResults)
			return nodeState.Result, nil
		}
	}

	if state.NodeStates == nil {
		state.NodeStates = make(map[string]*NodeExecutionState)
	}

	nodeState := &NodeExecutionState{
		NodeID: nodeID,
	}
	state.NodeStates[nodeID] = nodeState

	result, err := node.ExecuteWithState(ctx, execCtx, nodeState)

	nodeState.Completed = (err == nil && result != nil && result.Status == NodeStatusCompleted)
	nodeState.Result = result
	nodeState.InternalState = node.GetState()

	if result != nil {
		state.NodeResults[nodeID] = result
		collectNodeResults(result, state.NodeResults)
		collectCompletedNodes(result, &state.CompletedNodes)
	}

	if nodeState.Completed && !containsString(state.CompletedNodes, nodeID) {
		state.CompletedNodes = append(state.CompletedNodes, nodeID)
	}

	return result, err
}

func isNodeCompleted(nodeID string, state *WorkflowState) bool {
	if state.NodeStates == nil {
		return false
	}
	nodeState, exists := state.NodeStates[nodeID]
	return exists && nodeState.Completed
}

func getCompletedNodeResult(nodeID string, state *WorkflowState) *NodeResult {
	if state.NodeStates == nil {
		return nil
	}
	nodeState, exists := state.NodeStates[nodeID]
	if !exists || !nodeState.Completed {
		return nil
	}
	return nodeState.Result
}

func (e *WorkflowEngine) ExecuteWorkflowWithState(ctx context.Context, workflowID string, initialContext map[string]interface{}) (*WorkflowState, error) {
	wf, ok := e.GetWorkflow(workflowID)
	if !ok {
		return nil, ErrWorkflowNotFound
	}

	state := &WorkflowState{
		WorkflowID:     workflowID,
		Status:         WorkflowStatusRunning,
		NodeResults:    make(map[string]*NodeResult),
		NodeStates:     make(map[string]*NodeExecutionState),
		CompletedNodes: make([]string, 0),
		Context:        make(map[string]interface{}),
		CreatedAt:      time.Now(),
		StartedAt:      time.Now(),
	}

	if initialContext != nil {
		for k, v := range initialContext {
			state.Context[k] = v
		}
	}

	execCtx := NewExecutionContext()
	for k, v := range state.Context {
		execCtx.Set(k, v)
	}

	ctxWithState := WithWorkflowState(ctx, state)
	rootResult, err := executeNodeWithState(ctxWithState, wf.RootNode, execCtx, state)

	state.FinishedAt = time.Now()

	for k, v := range execCtx.Values() {
		state.Context[k] = v
	}

	if err != nil || (rootResult != nil && rootResult.Status == NodeStatusFailed) {
		state.Status = WorkflowStatusFailed
		if err != nil {
			state.Error = err.Error()
		} else if rootResult != nil {
			state.Error = rootResult.Error
			err = fmt.Errorf("%s", state.Error)
		}
	} else {
		state.Status = WorkflowStatusCompleted
	}

	instanceID := fmt.Sprintf("%s_%d", workflowID, time.Now().UnixNano())
	e.mu.Lock()
	e.instances[instanceID] = state
	e.mu.Unlock()

	return state, err
}

func (e *WorkflowEngine) ResumeWorkflow(ctx context.Context, state *WorkflowState) (*WorkflowState, error) {
	if state == nil {
		return nil, fmt.Errorf("state is nil")
	}

	wf, ok := e.GetWorkflow(state.WorkflowID)
	if !ok {
		return nil, ErrWorkflowNotFound
	}

	if state.Status != WorkflowStatusPaused && state.Status != WorkflowStatusFailed {
		return nil, fmt.Errorf("cannot resume workflow in status %s", state.Status)
	}

	if state.NodeStates == nil {
		state.NodeStates = make(map[string]*NodeExecutionState)
	}
	if state.NodeResults == nil {
		state.NodeResults = make(map[string]*NodeResult)
	}
	if state.CompletedNodes == nil {
		state.CompletedNodes = make([]string, 0)
	}
	if state.Context == nil {
		state.Context = make(map[string]interface{})
	}

	for nodeID, nodeState := range state.NodeStates {
		if nodeState != nil && nodeState.InternalState != nil {
			restoreNodeState(wf.RootNode, nodeID, nodeState.InternalState)
		}
	}

	execCtx := NewExecutionContext()
	for k, v := range state.Context {
		execCtx.Set(k, v)
	}

	state.Status = WorkflowStatusRunning
	state.StartedAt = time.Now()

	ctxWithState := WithWorkflowState(ctx, state)
	rootResult, err := executeNodeWithState(ctxWithState, wf.RootNode, execCtx, state)

	state.FinishedAt = time.Now()

	for k, v := range execCtx.Values() {
		state.Context[k] = v
	}

	if err != nil || (rootResult != nil && rootResult.Status == NodeStatusFailed) {
		state.Status = WorkflowStatusFailed
		if err != nil {
			state.Error = err.Error()
		} else if rootResult != nil {
			state.Error = rootResult.Error
			err = fmt.Errorf("%s", state.Error)
		}
	} else {
		state.Status = WorkflowStatusCompleted
	}

	return state, err
}

func restoreNodeState(node Node, targetID string, state NodeStateData) {
	if node.GetID() == targetID {
		node.RestoreState(state)
		return
	}

	switch n := node.(type) {
	case *SequentialNode:
		for _, child := range n.Nodes {
			restoreNodeState(child, targetID, state)
		}
	case *ParallelNode:
		for _, child := range n.Nodes {
			restoreNodeState(child, targetID, state)
		}
	case *ConditionalNode:
		for _, branch := range n.Branches {
			restoreNodeState(branch.Node, targetID, state)
		}
		if n.DefaultBranch != nil {
			restoreNodeState(n.DefaultBranch, targetID, state)
		}
	case *LoopNode:
		restoreNodeState(n.Node, targetID, state)
	}
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func (e *WorkflowEngine) GetInstanceState(instanceID string) (*WorkflowState, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	state, ok := e.instances[instanceID]
	return state, ok
}

func (e *WorkflowEngine) SaveState(state *WorkflowState) ([]byte, error) {
	if state == nil {
		return nil, fmt.Errorf("state is nil")
	}

	data, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize state: %w", err)
	}

	return data, nil
}

func (e *WorkflowEngine) LoadState(data []byte) (*WorkflowState, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data is empty")
	}

	var state WorkflowState
	err := json.Unmarshal(data, &state)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize state: %w", err)
	}

	if state.NodeResults == nil {
		state.NodeResults = make(map[string]*NodeResult)
	}
	if state.NodeStates == nil {
		state.NodeStates = make(map[string]*NodeExecutionState)
	}
	if state.CompletedNodes == nil {
		state.CompletedNodes = make([]string, 0)
	}
	if state.Context == nil {
		state.Context = make(map[string]interface{})
	}

	return &state, nil
}

func (e *WorkflowEngine) PauseWorkflow(instanceID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	state, ok := e.instances[instanceID]
	if !ok {
		return fmt.Errorf("workflow instance not found")
	}

	if state.Status != WorkflowStatusRunning {
		return fmt.Errorf("cannot pause workflow in status %s", state.Status)
	}

	state.Status = WorkflowStatusPaused
	return nil
}

func (e *WorkflowEngine) CancelWorkflow(instanceID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	state, ok := e.instances[instanceID]
	if !ok {
		return fmt.Errorf("workflow instance not found")
	}

	state.Status = WorkflowStatusFailed
	state.Error = "workflow canceled"
	state.FinishedAt = time.Now()
	return nil
}

func (e *WorkflowEngine) RemoveWorkflow(workflowID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.workflows[workflowID]; !ok {
		return ErrWorkflowNotFound
	}

	delete(e.workflows, workflowID)
	return nil
}

func (e *WorkflowEngine) ListInstances() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	ids := make([]string, 0, len(e.instances))
	for id := range e.instances {
		ids = append(ids, id)
	}
	return ids
}

func (e *WorkflowEngine) RemoveInstance(instanceID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.instances[instanceID]; !ok {
		return fmt.Errorf("instance not found")
	}

	delete(e.instances, instanceID)
	return nil
}
