# Workflow Engine Module

## Overview

The workflow engine module provides a flexible and extensible framework for defining and executing complex workflows with support for sequential, parallel, conditional, and loop execution patterns. The engine includes built-in support for node-level retry strategies, state persistence and recovery, and context-aware variable passing between nodes.

## Module Features

### 1. Sequential Execution
- Execute multiple task nodes in a predefined order
- Automatic triggering of next node after previous node completes
- Failure propagation: current node failure stops all subsequent nodes
- Execution results include outputs from all nodes and overall status

### 2. Parallel Execution
- Execute multiple task nodes concurrently
- Wait for all nodes to complete before continuing
- Configurable maximum failure tolerance
- Early termination when failure count exceeds threshold
- Optional timeout for parallel groups

### 3. Conditional Branching
- Route execution based on previous node output
- Support for multiple comparison operators (equals, not equals, contains, greater than, less than, etc.)
- Default branch support for fallback handling
- First-matching branch execution strategy

### 4. Loop Execution
- Fixed or dynamically determined iteration counts
- Iteration index and count available in execution context
- Configurable error handling: stop on error or continue iterations
- Context propagation between iterations

### 5. State Persistence and Recovery
- Serialize workflow state (completed nodes, current node, variables)
- Resume execution from saved state
- JSON-based serialization format
- Support for workflow pause/resume

### 6. Node-Level Retry Strategies
- Per-node retry configuration
- Multiple retry strategies: fixed, linear, exponential
- Configurable retry intervals and backoff factors
- Automatic retry on failure without external intervention

## Core Structures and Responsibilities

### Node Interface
All node types implement the `Node` interface:

```go
type Node interface {
    GetID() string
    GetType() NodeType
    GetName() string
    SetName(string)
    GetRetryConfig() RetryConfig
    SetRetryConfig(RetryConfig)
    Execute(ctx context.Context, execCtx *ExecutionContext) (*NodeResult, error)
}
```

### ExecutionContext
Thread-safe context for passing variables between nodes:
- `Set(key, value)` - Store a value
- `Get(key)` - Retrieve a value
- `GetString(key)` - Retrieve as string
- `GetInt(key)` - Retrieve as integer
- `Clone()` - Create a copy of the context
- `Values()` - Get all values as a map

### WorkflowEngine
Main entry point for workflow management:
- Register and manage workflow definitions
- Execute workflows with initial context
- Persist and load workflow states
- Manage workflow instances
- Support for cancellation and pausing

### WorkflowDefinition
Represents a workflow blueprint:
- Unique ID and name
- Root node (entry point)
- Optional version and description

### WorkflowState
Represents the runtime state of a workflow instance:
- Workflow ID and current status
- Completed nodes list
- Node execution results
- Variable context
- Timestamps (created, started, finished)

### NodeResult
Represents the execution result of a single node:
- Node ID and execution status
- Output data and error information
- Retry count and execution duration
- Timestamps (started, finished)

### RetryConfig
Configuration for node retry behavior:
- `MaxRetries` - Maximum retry attempts
- `Interval` - Base interval between retries
- `Strategy` - Retry strategy (fixed, linear, exponential)
- `BackoffFactor` - Multiplier for interval calculation

## Node Types and Execution Flow

### 1. SequentialNode (`NodeTypeSequential`)

**Purpose**: Execute child nodes one after another in order.

**Execution Flow**:
1. Start with the first child node
2. Execute the current node
3. If successful, proceed to the next node
4. If failed, mark the sequential node as failed and stop
5. After all nodes complete successfully, return combined results

**Behavior**:
- Context variables flow from earlier nodes to later nodes
- Any node failure immediately terminates the sequence
- Output contains results from all executed nodes

### 2. ParallelNode (`NodeTypeParallel`)

**Purpose**: Execute multiple child nodes concurrently.

**Execution Flow**:
1. Launch all child nodes in separate goroutines
2. Wait for all nodes to complete or for termination condition
3. Track failure count during execution
4. If failure count exceeds `MaxFailures`, terminate remaining nodes
5. Collect and return results from all nodes

**Configuration**:
- `MaxFailures` - Maximum allowed failures (-1 for unlimited)
- `Timeout` - Optional timeout for the entire parallel group

**Termination Conditions**:
- All nodes complete successfully
- Failure count exceeds threshold (remaining nodes are skipped)
- Context is canceled or timeout occurs

### 3. ConditionalNode (`NodeTypeConditional`)

**Purpose**: Route execution based on conditional evaluation.

**Execution Flow**:
1. Evaluate each branch's condition in order
2. Select the first branch with a matching condition
3. If no conditions match, use the default branch (if configured)
4. Execute the selected branch's node
5. Return the branch result as the conditional node's output

**Condition Operators**:
- `eq` / `==` - Equal to
- `ne` / `!=` - Not equal to
- `contains` - Contains substring
- `notcontains` - Does not contain substring
- `gt` / `>` - Greater than
- `gte` / `>=` - Greater than or equal to
- `lt` / `<` - Less than
- `lte` / `<=` - Less than or equal to

**Output**:
- `selected_branch` - Name of selected branch
- `matched_condition` - The condition that matched (if any)
- `default_branch` - True if default branch was taken
- `branch_result` - Execution result of the selected branch

### 4. LoopNode (`NodeTypeLoop`)

**Purpose**: Repeatedly execute a child node.

**Execution Flow**:
1. Determine the number of iterations (fixed or dynamic)
2. For each iteration:
   - Inject `iteration_index` and `iteration_count` into context
   - Create a cloned context for the iteration
   - Execute the child node
   - If failed and `ContinueOnError` is false, stop the loop
   - Propagate non-internal context variables to parent
3. Return combined iteration results

**Configuration**:
- `FixedIterations(n)` - Fixed number of iterations
- `DynamicIterations(key)` - Read iteration count from context key
- `ContinueOnError(bool)` - Whether to continue after iteration failure

**Iteration Context**:
Each iteration receives a cloned context with:
- `iteration_index` - Current iteration (0-based)
- `iteration_count` - Total number of iterations

## Usage Examples

### Basic Sequential Workflow

```go
package main

import (
    "context"
    "fmt"
    "solocoder-go/internal/workflow"
)

func main() {
    engine := workflow.NewWorkflowEngine()

    // Create tasks
    task1 := workflow.NewCallbackTaskNode("fetch_data", func(ctx context.Context, execCtx *workflow.ExecutionContext) (interface{}, error) {
        execCtx.Set("data", "raw_data")
        return "data_fetched", nil
    })

    task2 := workflow.NewCallbackTaskNode("process_data", func(ctx context.Context, execCtx *workflow.ExecutionContext) (interface{}, error) {
        data, _ := execCtx.GetString("data")
        processed := data + "_processed"
        execCtx.Set("processed_data", processed)
        return processed, nil
    })

    task3 := workflow.NewCallbackTaskNode("save_data", func(ctx context.Context, execCtx *workflow.ExecutionContext) (interface{}, error) {
        processed, _ := execCtx.GetString("processed_data")
        fmt.Printf("Saving: %s\n", processed)
        return "saved", nil
    })

    // Create sequential workflow
    root := workflow.NewSequentialNode("main", task1, task2, task3)

    // Register and execute
    wf := &workflow.WorkflowDefinition{
        ID:       "data_pipeline",
        Name:     "Data Processing Pipeline",
        RootNode: root,
    }
    engine.RegisterWorkflow(wf)

    result, err := engine.ExecuteWorkflow(context.Background(), "data_pipeline", nil)
    if err != nil {
        fmt.Printf("Workflow failed: %v\n", err)
        return
    }

    fmt.Printf("Workflow status: %s\n", result.Status)
}
```

### Parallel Workflow with Failure Tolerance

```go
engine := workflow.NewWorkflowEngine()

// Create parallel tasks
scanTask := workflow.NewCallbackTaskNode("scan", func(ctx context.Context, execCtx *workflow.ExecutionContext) (interface{}, error) {
    time.Sleep(100 * time.Millisecond)
    return "scan_complete", nil
})

analyzeTask := workflow.NewCallbackTaskNode("analyze", func(ctx context.Context, execCtx *workflow.ExecutionContext) (interface{}, error) {
    time.Sleep(150 * time.Millisecond)
    return "analyze_complete", nil
})

reportTask := workflow.NewCallbackTaskNode("report", func(ctx context.Context, execCtx *workflow.ExecutionContext) (interface{}, error) {
    time.Sleep(50 * time.Millisecond)
    return "report_complete", nil
})

// Parallel node allowing 1 failure
parallel := workflow.NewParallelNode("parallel_tasks", scanTask, analyzeTask, reportTask)
parallel.SetMaxFailures(1)  // Allow 1 failure before terminating
parallel.SetTimeout(1 * time.Second)

wf := &workflow.WorkflowDefinition{
    ID:       "parallel_processing",
    RootNode: parallel,
}
engine.RegisterWorkflow(wf)

result, err := engine.ExecuteWorkflow(context.Background(), "parallel_processing", nil)
```

### Conditional Workflow

```go
engine := workflow.NewWorkflowEngine()

// Check status task
checkStatus := workflow.NewCallbackTaskNode("check_status", func(ctx context.Context, execCtx *workflow.ExecutionContext) (interface{}, error) {
    execCtx.Set("user_type", "premium")
    execCtx.Set("account_balance", 1500)
    return "checked", nil
})

// Branch for premium users
premiumBranch := workflow.NewCallbackTaskNode("premium_processing", func(ctx context.Context, execCtx *workflow.ExecutionContext) (interface{}, error) {
    return "premium features activated", nil
})

// Branch for regular users
regularBranch := workflow.NewCallbackTaskNode("regular_processing", func(ctx context.Context, execCtx *workflow.ExecutionContext) (interface{}, error) {
    return "regular features activated", nil
})

// Default branch
defaultBranch := workflow.NewCallbackTaskNode("default_processing", func(ctx context.Context, execCtx *workflow.ExecutionContext) (interface{}, error) {
    return "default processing", nil
})

// Conditional node
conditional := workflow.NewConditionalNode("user_routing")
conditional.AddBranch(&workflow.Condition{
    Field:    "user_type",
    Operator: "eq",
    Value:    "premium",
}, premiumBranch)
conditional.AddBranch(&workflow.Condition{
    Field:    "account_balance",
    Operator: "gt",
    Value:    1000,
}, regularBranch)
conditional.SetDefaultBranch(defaultBranch)

// Sequential workflow: check status, then route
root := workflow.NewSequentialNode("main", checkStatus, conditional)
```

### Loop with Dynamic Iterations

```go
engine := workflow.NewWorkflowEngine()

// Setup task to determine iteration count
setup := workflow.NewSetContextTask("setup", "item_count", 5)

// Loop body: process each item
processItem := workflow.NewCallbackTaskNode("process_item", func(ctx context.Context, execCtx *workflow.ExecutionContext) (interface{}, error) {
    idx, _ := execCtx.GetInt("iteration_index")
    count, _ := execCtx.GetInt("iteration_count")
    
    result := fmt.Sprintf("processed item %d of %d", idx+1, count)
    execCtx.Set(fmt.Sprintf("item_%d_result", idx), result)
    
    return result, nil
})

// Loop node with dynamic iterations from context
loop := workflow.NewLoopNode("process_items", processItem)
loop.SetDynamicIterations("item_count")
loop.SetContinueOnError(true)

root := workflow.NewSequentialNode("main", setup, loop)
```

### Node with Retry Strategy

```go
// Flaky task that fails 2 times before succeeding
flakyTask := workflow.NewFlakyTask("unreliable_api", 2, "success")

// Configure retry: 3 retries with exponential backoff
flakyTask.SetRetryConfig(workflow.RetryConfig{
    MaxRetries:      3,
    Interval:        100 * time.Millisecond,
    Strategy:        workflow.RetryExponential,
    BackoffFactor:   2.0,
})

// This will execute: fail, wait 100ms, fail, wait 200ms, fail, wait 400ms, succeed
result, err := flakyTask.Execute(context.Background(), NewExecutionContext())
```

### State Persistence and Recovery

```go
engine := workflow.NewWorkflowEngine()

// Execute workflow and get state
state, err := engine.ExecuteWorkflowWithState(context.Background(), "my_workflow", initialContext)
if err != nil {
    // Handle error
}

// Save state to persistent storage
data, err := engine.SaveState(state)
if err != nil {
    // Handle error
}

// Later, load and resume the workflow
loadedState, err := engine.LoadState(data)
if err != nil {
    // Handle error
}

// Mark as paused (simulating interruption)
loadedState.Status = workflow.WorkflowStatusPaused

// Resume from where it left off
resumedState, err := engine.ResumeWorkflow(context.Background(), loadedState)
```

## Error Handling

The workflow engine provides comprehensive error handling:

- **Node-level errors**: Each node's Execute method returns an error and a NodeResult with details
- **Propagation**: Errors propagate up through parent nodes (sequential stops, parallel tracks)
- **Retry**: Nodes with retry configuration automatically retry on failure
- **Context cancellation**: Workflows respect context cancellation and timeouts
- **State tracking**: All errors are captured in the workflow state for debugging

## Thread Safety

- `ExecutionContext` uses `sync.RWMutex` for thread-safe variable access
- `WorkflowEngine` uses `sync.RWMutex` for workflow and instance management
- Parallel node execution uses proper synchronization with `sync.WaitGroup` and atomic operations
- Node state modifications in parallel contexts are protected by appropriate locking

## Best Practices

1. **Keep tasks focused**: Each task node should do one thing well
2. **Use descriptive names**: Clear node names help with debugging
3. **Leverage context**: Pass data between nodes using the execution context
4. **Configure retries appropriately**: Use retry only for transient failures
5. **Set timeouts**: Protect parallel and long-running operations with timeouts
6. **Handle errors explicitly**: Check workflow status and errors after execution
7. **Test edge cases**: Test failure scenarios, timeouts, and boundary conditions

## File Structure

```
internal/workflow/
├── types.go          # Core types, interfaces, and constants
├── node.go           # Base node implementation and helpers
├── task.go           # Task node implementations
├── sequential.go     # Sequential node
├── parallel.go       # Parallel node
├── conditional.go    # Conditional node
├── loop.go           # Loop node
├── engine.go         # Workflow engine
└── workflow_test.go  # Comprehensive unit tests
```
