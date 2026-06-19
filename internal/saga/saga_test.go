package saga

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewCoordinator(t *testing.T) {
	c := NewCoordinator()
	if c == nil {
		t.Fatal("NewCoordinator returned nil")
	}
	if c.sagas == nil {
		t.Error("sagas map not initialized")
	}
	if c.executions == nil {
		t.Error("executions map not initialized")
	}
	if c.logs == nil {
		t.Error("logs slice not initialized")
	}
	if c.logsByTransaction == nil {
		t.Error("logsByTransaction map not initialized")
	}
	if c.pendingInterventions == nil {
		t.Error("pendingInterventions slice not initialized")
	}
	if c.runningSagas == nil {
		t.Error("runningSagas map not initialized")
	}
}

func TestNewSaga(t *testing.T) {
	c := NewCoordinator()

	saga, err := c.NewSaga("test-saga", "Test Saga")
	if err != nil {
		t.Fatalf("NewSaga failed: %v", err)
	}
	if saga == nil {
		t.Fatal("NewSaga returned nil saga")
	}
	if saga.ID != "test-saga" {
		t.Errorf("expected saga ID 'test-saga', got '%s'", saga.ID)
	}
	if saga.Name != "Test Saga" {
		t.Errorf("expected saga name 'Test Saga', got '%s'", saga.Name)
	}
	if len(saga.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(saga.Steps))
	}

	_, err = c.NewSaga("test-saga", "Duplicate")
	if !errors.Is(err, ErrSagaAlreadyExists) {
		t.Errorf("expected ErrSagaAlreadyExists, got %v", err)
	}
}

func TestAddStep(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("test-saga", "Test Saga")

	err := saga.AddStep("", "Empty ID", func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
		return nil, nil
	}, nil)
	if !errors.Is(err, ErrInvalidStepID) {
		t.Errorf("expected ErrInvalidStepID for empty ID, got %v", err)
	}

	err = saga.AddStep("step1", "Step 1", nil, nil)
	if err == nil {
		t.Error("expected error for nil forward function")
	}

	forward := func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
		return "result1", nil
	}
	compensate := func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
		return nil, nil
	}

	err = saga.AddStep("step1", "Step 1", forward, compensate)
	if err != nil {
		t.Fatalf("AddStep failed: %v", err)
	}
	if len(saga.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(saga.Steps))
	}

	err = saga.AddStep("step1", "Duplicate Step", forward, compensate)
	if !errors.Is(err, ErrStepAlreadyExists) {
		t.Errorf("expected ErrStepAlreadyExists, got %v", err)
	}

	err = saga.AddStep("step2", "Step 2", forward, nil)
	if err != nil {
		t.Fatalf("AddStep with nil compensate failed: %v", err)
	}
	if len(saga.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(saga.Steps))
	}
}

func TestGetStep(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("test-saga", "Test Saga")

	forward := func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
		return nil, nil
	}
	saga.AddStep("step1", "Step 1", forward, nil)

	step, err := saga.GetStep("step1")
	if err != nil {
		t.Fatalf("GetStep failed: %v", err)
	}
	if step.ID != "step1" {
		t.Errorf("expected step ID 'step1', got '%s'", step.ID)
	}

	_, err = saga.GetStep("nonexistent")
	if !errors.Is(err, ErrStepNotFound) {
		t.Errorf("expected ErrStepNotFound, got %v", err)
	}
}

func TestGetSaga(t *testing.T) {
	c := NewCoordinator()
	c.NewSaga("test-saga", "Test Saga")

	saga, err := c.GetSaga("test-saga")
	if err != nil {
		t.Fatalf("GetSaga failed: %v", err)
	}
	if saga.ID != "test-saga" {
		t.Errorf("expected saga ID 'test-saga', got '%s'", saga.ID)
	}

	_, err = c.GetSaga("nonexistent")
	if !errors.Is(err, ErrSagaNotFound) {
		t.Errorf("expected ErrSagaNotFound, got %v", err)
	}
}

func TestExecute_Success(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("order-saga", "Order Processing")

	step1Called := false
	step2Called := false
	step3Called := false

	saga.AddStep("reserve-inventory", "Reserve Inventory",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			step1Called = true
			return "inventory-reserved-123", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, nil
		},
	)

	saga.AddStep("charge-payment", "Charge Payment",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			step2Called = true
			inventoryResult, ok := data["reserve-inventory"].(string)
			if !ok || inventoryResult != "inventory-reserved-123" {
				t.Error("context data not passed correctly")
			}
			return "payment-charged-456", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, nil
		},
	)

	saga.AddStep("create-order", "Create Order",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			step3Called = true
			paymentResult, ok := data["charge-payment"].(string)
			if !ok || paymentResult != "payment-charged-456" {
				t.Error("context data not passed correctly")
			}
			initialValue, ok := data["order-id"].(string)
			if !ok || initialValue != "initial-order-789" {
				t.Error("initial data not available")
			}
			return "order-created-789", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, nil
		},
	)

	initialData := map[string]interface{}{
		"order-id": "initial-order-789",
	}

	result, err := c.Execute(context.Background(), "order-saga", initialData)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Status != StatusSuccess {
		t.Errorf("expected StatusSuccess, got %v", result.Status)
	}
	if result.Error != nil {
		t.Errorf("expected no error, got %v", result.Error)
	}
	if !step1Called || !step2Called || !step3Called {
		t.Error("not all steps were called")
	}
	if len(result.StepResults) != 3 {
		t.Errorf("expected 3 step results, got %d", len(result.StepResults))
	}
	for _, stepResult := range result.StepResults {
		if stepResult.Status != StatusSuccess {
			t.Errorf("step %s expected StatusSuccess, got %v", stepResult.StepID, stepResult.Status)
		}
	}
	if len(result.Compensations) != 0 {
		t.Errorf("expected 0 compensations, got %d", len(result.Compensations))
	}
	if result.Duration < 0 {
		t.Error("expected non-negative duration")
	}
	if result.StartTime.IsZero() || result.EndTime.IsZero() {
		t.Error("expected non-zero start and end times")
	}
}

func TestExecute_FailureOnFirstStep(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("test-saga", "Test Saga")

	compensateCalled := false

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("step 1 failed")
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			compensateCalled = true
			return nil, nil
		},
	)

	saga.AddStep("step2", "Step 2",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			t.Error("step2 should not be called")
			return nil, nil
		},
		nil,
	)

	result, err := c.Execute(context.Background(), "test-saga", nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %v", result.Status)
	}
	if result.Error == nil {
		t.Error("expected error in result")
	}
	if compensateCalled {
		t.Error("compensation should not be called for failed step")
	}
	if len(result.Compensations) != 0 {
		t.Errorf("expected 0 compensations, got %d", len(result.Compensations))
	}
	if result.NeedsIntervention {
		t.Error("expected no intervention needed since no compensations ran")
	}
	if errors.Is(result.Error, ErrCompensationFailed) {
		t.Error("expected ErrCompensationFailed NOT to be in error chain since no compensation failed")
	}
}

func TestExecute_FailureOnThirdStep(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("test-saga", "Test Saga")

	comp1Called := false
	comp2Called := false
	compOrder := make([]string, 0)

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return "result1", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			comp1Called = true
			compOrder = append(compOrder, "step1")
			return nil, nil
		},
	)

	saga.AddStep("step2", "Step 2",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return "result2", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			comp2Called = true
			compOrder = append(compOrder, "step2")
			return nil, nil
		},
	)

	saga.AddStep("step3", "Step 3",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("step 3 failed")
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			t.Error("step3 compensation should not be called")
			return nil, nil
		},
	)

	result, err := c.Execute(context.Background(), "test-saga", nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %v", result.Status)
	}
	if !comp1Called || !comp2Called {
		t.Error("compensations for successful steps should be called")
	}
	if len(compOrder) != 2 || compOrder[0] != "step2" || compOrder[1] != "step1" {
		t.Errorf("compensations should be called in reverse order, got %v", compOrder)
	}
	if len(result.Compensations) != 2 {
		t.Errorf("expected 2 compensations, got %d", len(result.Compensations))
	}

	if _, ok := result.Compensations["step2-compensate"]; !ok {
		t.Error("expected compensation key 'step2-compensate'")
	}
	if _, ok := result.Compensations["step1-compensate"]; !ok {
		t.Error("expected compensation key 'step1-compensate'")
	}

	for _, compResult := range result.Compensations {
		if compResult.Status != StatusSuccess {
			t.Errorf("compensation %s expected StatusSuccess, got %v", compResult.StepID, compResult.Status)
		}
		if compResult.StepID != "step1-compensate" && compResult.StepID != "step2-compensate" {
			t.Errorf("unexpected compensation StepID: %s", compResult.StepID)
		}
	}

	if result.NeedsIntervention {
		t.Error("expected no intervention since all compensations succeeded")
	}
	if errors.Is(result.Error, ErrCompensationFailed) {
		t.Error("expected ErrCompensationFailed NOT to be in error chain since all compensations succeeded")
	}
}

func TestExecute_CompensationFailure(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("test-saga", "Test Saga")

	comp2Called := false

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return "result1", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("compensation 1 failed")
		},
	)

	saga.AddStep("step2", "Step 2",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return "result2", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			comp2Called = true
			return nil, nil
		},
	)

	saga.AddStep("step3", "Step 3",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("step 3 failed")
		},
		nil,
	)

	result, err := c.Execute(context.Background(), "test-saga", nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %v", result.Status)
	}
	if !result.NeedsIntervention {
		t.Error("expected NeedsIntervention to be true")
	}
	if len(result.InterventionNotes) != 1 {
		t.Errorf("expected 1 intervention note, got %d", len(result.InterventionNotes))
	}

	note := result.InterventionNotes[0]
	if note.StepID != "step1-compensate" {
		t.Errorf("expected intervention StepID 'step1-compensate', got '%s'", note.StepID)
	}
	if note.ForwardStepID != "step1" {
		t.Errorf("expected intervention ForwardStepID 'step1', got '%s'", note.ForwardStepID)
	}
	if note.StepID == note.ForwardStepID {
		t.Error("StepID and ForwardStepID should be distinct")
	}

	if !errors.Is(result.Error, ErrCompensationFailed) {
		t.Errorf("expected ErrCompensationFailed in error chain, got %v", result.Error)
	}

	if !comp2Called {
		t.Error("step2 compensation should still be called even if step1 compensation fails")
	}

	pending := c.GetPendingInterventions()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending intervention, got %d", len(pending))
	}
	if pending[0].StepID != "step1-compensate" {
		t.Errorf("expected pending intervention StepID 'step1-compensate', got '%s'", pending[0].StepID)
	}
	if pending[0].ForwardStepID != "step1" {
		t.Errorf("expected pending intervention ForwardStepID 'step1', got '%s'", pending[0].ForwardStepID)
	}
}

func TestExecute_MultipleCompensationFailures(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("test-saga", "Test Saga")

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return "result1", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("compensation 1 failed")
		},
	)

	saga.AddStep("step2", "Step 2",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return "result2", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("compensation 2 failed")
		},
	)

	saga.AddStep("step3", "Step 3",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("step 3 failed")
		},
		nil,
	)

	result, err := c.Execute(context.Background(), "test-saga", nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.NeedsIntervention {
		t.Error("expected NeedsIntervention to be true")
	}
	if len(result.InterventionNotes) != 2 {
		t.Errorf("expected 2 intervention notes, got %d", len(result.InterventionNotes))
	}

	if !errors.Is(result.Error, ErrCompensationFailed) {
		t.Errorf("expected ErrCompensationFailed in error chain, got %v", result.Error)
	}

	pending := c.GetPendingInterventions()
	if len(pending) != 2 {
		t.Errorf("expected 2 pending interventions, got %d", len(pending))
	}

	stepIDs := make(map[string]bool)
	for _, p := range pending {
		stepIDs[p.StepID] = true
	}
	if !stepIDs["step1-compensate"] || !stepIDs["step2-compensate"] {
		t.Errorf("expected pending intervention StepIDs 'step1-compensate' and 'step2-compensate', got %v", stepIDs)
	}
}

func TestExecute_NoCompensationFunction(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("test-saga", "Test Saga")

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return "result1", nil
		},
		nil,
	)

	saga.AddStep("step2", "Step 2",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("step 2 failed")
		},
		nil,
	)

	result, err := c.Execute(context.Background(), "test-saga", nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %v", result.Status)
	}
	if result.NeedsIntervention {
		t.Error("expected NeedsIntervention to be false when no compensation function")
	}
	if len(result.Compensations) != 0 {
		t.Errorf("expected 0 compensations recorded, got %d", len(result.Compensations))
	}

	logs, _ := c.GetLogs(result.ID)
	for _, log := range logs {
		if log.OperationType == OpTypeCompensation && log.Status == StatusSuccess {
			t.Errorf("found fake compensation success log: stepID=%s, details=%s", log.StepID, log.Details)
		}
	}
}

func TestExecute_NoSteps(t *testing.T) {
	c := NewCoordinator()
	c.NewSaga("empty-saga", "Empty Saga")

	_, err := c.Execute(context.Background(), "empty-saga", nil)
	if !errors.Is(err, ErrNoStepsRegistered) {
		t.Errorf("expected ErrNoStepsRegistered, got %v", err)
	}
}

func TestExecute_NonexistentSaga(t *testing.T) {
	c := NewCoordinator()

	_, err := c.Execute(context.Background(), "nonexistent", nil)
	if !errors.Is(err, ErrSagaNotFound) {
		t.Errorf("expected ErrSagaNotFound, got %v", err)
	}
}

func TestExecute_SagaRunningConcurrentGuard(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("guarded-saga", "Guarded Saga")

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			time.Sleep(200 * time.Millisecond)
			return nil, nil
		},
		nil,
	)

	var wg sync.WaitGroup
	var firstErr error
	var secondErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, firstErr = c.Execute(context.Background(), "guarded-saga", nil)
	}()
	time.Sleep(50 * time.Millisecond)
	go func() {
		defer wg.Done()
		_, secondErr = c.Execute(context.Background(), "guarded-saga", nil)
	}()
	wg.Wait()

	if firstErr != nil {
		t.Errorf("first execution should succeed, got %v", firstErr)
	}
	if !errors.Is(secondErr, ErrSagaRunning) {
		t.Errorf("second execution should return ErrSagaRunning, got %v", secondErr)
	}
}

func TestExecute_SagaRunningAllowsSequentialExecution(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("seq-saga", "Sequential Saga")

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, nil
		},
		nil,
	)

	result1, err1 := c.Execute(context.Background(), "seq-saga", nil)
	if err1 != nil {
		t.Fatalf("first execution failed: %v", err1)
	}
	if result1.Status != StatusSuccess {
		t.Errorf("first execution expected StatusSuccess, got %v", result1.Status)
	}

	result2, err2 := c.Execute(context.Background(), "seq-saga", nil)
	if err2 != nil {
		t.Fatalf("second execution failed: %v", err2)
	}
	if result2.Status != StatusSuccess {
		t.Errorf("second execution expected StatusSuccess, got %v", result2.Status)
	}
}

func TestExecute_DifferentSagasConcurrent(t *testing.T) {
	c := NewCoordinator()
	saga1, _ := c.NewSaga("saga-1", "Saga 1")
	saga2, _ := c.NewSaga("saga-2", "Saga 2")

	saga1.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			time.Sleep(100 * time.Millisecond)
			return nil, nil
		},
		nil,
	)

	saga2.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			time.Sleep(100 * time.Millisecond)
			return nil, nil
		},
		nil,
	)

	var wg sync.WaitGroup
	var err1, err2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err1 = c.Execute(context.Background(), "saga-1", nil)
	}()
	go func() {
		defer wg.Done()
		_, err2 = c.Execute(context.Background(), "saga-2", nil)
	}()
	wg.Wait()

	if err1 != nil {
		t.Errorf("saga-1 execution failed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("saga-2 execution failed: %v", err2)
	}
}

func TestExecute_ContextCancelled(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("test-saga", "Test Saga")

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return "result1", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, nil
		},
	)

	saga.AddStep("step2", "Step 2",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			time.Sleep(100 * time.Millisecond)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return "result2", nil
			}
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, nil
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result, err := c.Execute(ctx, "test-saga", nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %v", result.Status)
	}
}

func TestExecute_PanicRecovery(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("test-saga", "Test Saga")

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			panic("something went wrong")
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, nil
		},
	)

	result, err := c.Execute(context.Background(), "test-saga", nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %v", result.Status)
	}
	if result.Error == nil {
		t.Error("expected error from panic recovery")
	}
	if result.StepResults["step1"].Status != StatusFailed {
		t.Error("expected step1 status to be Failed")
	}
}

func TestExecute_CompensationPanicRecovery(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("test-saga", "Test Saga")

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return "result1", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			panic("compensation panicked")
		},
	)

	saga.AddStep("step2", "Step 2",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("step 2 failed")
		},
		nil,
	)

	result, err := c.Execute(context.Background(), "test-saga", nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %v", result.Status)
	}
	if !result.NeedsIntervention {
		t.Error("expected NeedsIntervention to be true")
	}
	if len(result.InterventionNotes) != 1 {
		t.Errorf("expected 1 intervention note, got %d", len(result.InterventionNotes))
	}
	if result.InterventionNotes[0].StepID != "step1-compensate" {
		t.Errorf("expected intervention StepID 'step1-compensate', got '%s'", result.InterventionNotes[0].StepID)
	}
	if result.InterventionNotes[0].ForwardStepID != "step1" {
		t.Errorf("expected intervention ForwardStepID 'step1', got '%s'", result.InterventionNotes[0].ForwardStepID)
	}
	if !errors.Is(result.Error, ErrCompensationFailed) {
		t.Errorf("expected ErrCompensationFailed in error chain, got %v", result.Error)
	}
}

func TestExecute_ErrCompensationFailedInErrorChain(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("test-saga", "Test Saga")

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return "result1", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("compensation failed")
		},
	)

	saga.AddStep("step2", "Step 2",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("step 2 forward failed")
		},
		nil,
	)

	result, _ := c.Execute(context.Background(), "test-saga", nil)

	if !errors.Is(result.Error, ErrCompensationFailed) {
		t.Errorf("expected ErrCompensationFailed in error chain, got %v", result.Error)
	}
	if !result.NeedsIntervention {
		t.Error("expected NeedsIntervention to be true")
	}
}

func TestGetLogs(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("test-saga", "Test Saga")

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return "result1", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, nil
		},
	)

	saga.AddStep("step2", "Step 2",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("step 2 failed")
		},
		nil,
	)

	result, _ := c.Execute(context.Background(), "test-saga", nil)

	logs, err := c.GetLogs(result.ID)
	if err != nil {
		t.Fatalf("GetLogs failed: %v", err)
	}
	if len(logs) == 0 {
		t.Error("expected non-empty logs")
	}

	for _, log := range logs {
		if log.TransactionID != result.ID {
			t.Errorf("expected transaction ID %s, got %s", result.ID, log.TransactionID)
		}
		if log.Timestamp.IsZero() {
			t.Error("expected non-zero timestamp")
		}
	}

	_, err = c.GetLogs("nonexistent")
	if !errors.Is(err, ErrExecutionNotFound) {
		t.Errorf("expected ErrExecutionNotFound, got %v", err)
	}
}

func TestGetAllLogs(t *testing.T) {
	c := NewCoordinator()
	saga1, _ := c.NewSaga("saga1", "Saga 1")
	saga2, _ := c.NewSaga("saga2", "Saga 2")

	saga1.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, nil
		},
		nil,
	)

	saga2.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, nil
		},
		nil,
	)

	c.Execute(context.Background(), "saga1", nil)
	c.Execute(context.Background(), "saga2", nil)

	logs := c.GetAllLogs()
	if len(logs) == 0 {
		t.Error("expected non-empty logs")
	}
}

func TestGetPendingInterventions(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("test-saga", "Test Saga")

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return "result1", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("compensation failed")
		},
	)

	saga.AddStep("step2", "Step 2",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("step 2 failed")
		},
		nil,
	)

	pending := c.GetPendingInterventions()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending interventions initially, got %d", len(pending))
	}

	c.Execute(context.Background(), "test-saga", nil)

	pending = c.GetPendingInterventions()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending intervention, got %d", len(pending))
	}
	if pending[0].Resolved {
		t.Error("expected intervention to be unresolved")
	}
	if pending[0].FailureReason == "" {
		t.Error("expected non-empty failure reason")
	}
	if pending[0].FailureTime.IsZero() {
		t.Error("expected non-zero failure time")
	}
	if pending[0].StepID != "step1-compensate" {
		t.Errorf("expected StepID 'step1-compensate', got '%s'", pending[0].StepID)
	}
	if pending[0].ForwardStepID != "step1" {
		t.Errorf("expected ForwardStepID 'step1', got '%s'", pending[0].ForwardStepID)
	}
}

func TestResolveIntervention(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("test-saga", "Test Saga")

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return "result1", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("compensation failed")
		},
	)

	saga.AddStep("step2", "Step 2",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("step 2 failed")
		},
		nil,
	)

	result, _ := c.Execute(context.Background(), "test-saga", nil)

	err := c.ResolveIntervention(result.ID, "step1-compensate", "Manually compensated")
	if err != nil {
		t.Fatalf("ResolveIntervention failed: %v", err)
	}

	pending := c.GetPendingInterventions()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending interventions after resolve, got %d", len(pending))
	}

	exec, _ := c.GetExecution(result.ID)
	if exec.NeedsIntervention {
		t.Error("expected NeedsIntervention to be false after all resolved")
	}

	err = c.ResolveIntervention("nonexistent", "step1-compensate", "test")
	if !errors.Is(err, ErrInterventionNotFound) {
		t.Errorf("expected ErrInterventionNotFound, got %v", err)
	}
}

func TestResolveMultipleInterventions(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("test-saga", "Test Saga")

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return "result1", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("compensation 1 failed")
		},
	)

	saga.AddStep("step2", "Step 2",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return "result2", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("compensation 2 failed")
		},
	)

	saga.AddStep("step3", "Step 3",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("step 3 failed")
		},
		nil,
	)

	result, _ := c.Execute(context.Background(), "test-saga", nil)

	err := c.ResolveIntervention(result.ID, "step2-compensate", "Resolved step2")
	if err != nil {
		t.Fatalf("ResolveIntervention for step2 failed: %v", err)
	}

	pending := c.GetPendingInterventions()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending intervention, got %d", len(pending))
	}

	exec, _ := c.GetExecution(result.ID)
	if !exec.NeedsIntervention {
		t.Error("expected NeedsIntervention to still be true")
	}

	err = c.ResolveIntervention(result.ID, "step1-compensate", "Resolved step1")
	if err != nil {
		t.Fatalf("ResolveIntervention for step1 failed: %v", err)
	}

	pending = c.GetPendingInterventions()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending interventions, got %d", len(pending))
	}

	exec, _ = c.GetExecution(result.ID)
	if exec.NeedsIntervention {
		t.Error("expected NeedsIntervention to be false")
	}
}

func TestGetExecution(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("test-saga", "Test Saga")

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, nil
		},
		nil,
	)

	result, _ := c.Execute(context.Background(), "test-saga", nil)

	exec, err := c.GetExecution(result.ID)
	if err != nil {
		t.Fatalf("GetExecution failed: %v", err)
	}
	if exec.ID != result.ID {
		t.Errorf("expected execution ID %s, got %s", result.ID, exec.ID)
	}

	_, err = c.GetExecution("nonexistent")
	if !errors.Is(err, ErrExecutionNotFound) {
		t.Errorf("expected ErrExecutionNotFound, got %v", err)
	}
}

func TestGetExecutionsBySaga(t *testing.T) {
	c := NewCoordinator()
	saga1, _ := c.NewSaga("saga1", "Saga 1")
	saga2, _ := c.NewSaga("saga2", "Saga 2")

	saga1.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, nil
		},
		nil,
	)

	saga2.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, nil
		},
		nil,
	)

	c.Execute(context.Background(), "saga1", nil)
	c.Execute(context.Background(), "saga1", nil)
	c.Execute(context.Background(), "saga2", nil)

	execs1 := c.GetExecutionsBySaga("saga1")
	if len(execs1) != 2 {
		t.Errorf("expected 2 executions for saga1, got %d", len(execs1))
	}

	execs2 := c.GetExecutionsBySaga("saga2")
	if len(execs2) != 1 {
		t.Errorf("expected 1 execution for saga2, got %d", len(execs2))
	}

	execs3 := c.GetExecutionsBySaga("nonexistent")
	if len(execs3) != 0 {
		t.Errorf("expected 0 executions for nonexistent saga, got %d", len(execs3))
	}
}

func TestRemoveSaga(t *testing.T) {
	c := NewCoordinator()
	c.NewSaga("test-saga", "Test Saga")

	err := c.RemoveSaga("test-saga")
	if err != nil {
		t.Fatalf("RemoveSaga failed: %v", err)
	}

	_, err = c.GetSaga("test-saga")
	if !errors.Is(err, ErrSagaNotFound) {
		t.Errorf("expected ErrSagaNotFound after remove, got %v", err)
	}

	err = c.RemoveSaga("nonexistent")
	if !errors.Is(err, ErrSagaNotFound) {
		t.Errorf("expected ErrSagaNotFound for nonexistent, got %v", err)
	}
}

func TestOperationStatus_String(t *testing.T) {
	tests := []struct {
		status   OperationStatus
		expected string
	}{
		{StatusPending, "Pending"},
		{StatusRunning, "Running"},
		{StatusSuccess, "Success"},
		{StatusFailed, "Failed"},
		{OperationStatus(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.status.String() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.status.String())
			}
		})
	}
}

func TestOperationType_String(t *testing.T) {
	tests := []struct {
		opType   OperationType
		expected string
	}{
		{OpTypeForward, "Forward"},
		{OpTypeCompensation, "Compensation"},
		{OperationType(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.opType.String() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.opType.String())
			}
		})
	}
}

func TestConcurrentExecute_DifferentSagas(t *testing.T) {
	c := NewCoordinator()

	var counter int
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		sagaID := fmt.Sprintf("concurrent-saga-%d", i)
		saga, _ := c.NewSaga(sagaID, fmt.Sprintf("Concurrent Saga %d", i))
		saga.AddStep("step1", "Step 1",
			func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
				mu.Lock()
				counter++
				mu.Unlock()
				time.Sleep(10 * time.Millisecond)
				return nil, nil
			},
			nil,
		)
	}

	var wg sync.WaitGroup
	numRuns := 10

	for i := 0; i < numRuns; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sagaID := fmt.Sprintf("concurrent-saga-%d", idx)
			c.Execute(context.Background(), sagaID, nil)
		}(i)
	}

	wg.Wait()

	if counter != numRuns {
		t.Errorf("expected counter to be %d, got %d", numRuns, counter)
	}

	allLogs := c.GetAllLogs()
	if len(allLogs) == 0 {
		t.Error("expected logs from concurrent executions")
	}
}

func TestLogEntry_CompleteTrace(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("trace-saga", "Trace Saga")

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return "result1", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, nil
		},
	)

	saga.AddStep("step2", "Step 2",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("step 2 failed")
		},
		nil,
	)

	result, _ := c.Execute(context.Background(), "trace-saga", nil)

	logs, _ := c.GetLogs(result.ID)

	hasStart := false
	hasStep1Start := false
	hasStep1Success := false
	hasStep2Start := false
	hasStep2Fail := false
	hasCompStart := false
	hasCompSuccess := false
	hasSagaFail := false
	hasNoCompensationLogForStep2 := true

	for _, log := range logs {
		switch {
		case log.StepID == "" && log.OperationType == OpTypeForward && log.Status == StatusRunning:
			hasStart = true
		case log.StepID == "step1" && log.OperationType == OpTypeForward && log.Status == StatusRunning:
			hasStep1Start = true
		case log.StepID == "step1" && log.OperationType == OpTypeForward && log.Status == StatusSuccess:
			hasStep1Success = true
		case log.StepID == "step2" && log.OperationType == OpTypeForward && log.Status == StatusRunning:
			hasStep2Start = true
		case log.StepID == "step2" && log.OperationType == OpTypeForward && log.Status == StatusFailed:
			hasStep2Fail = true
		case log.StepID == "step1-compensate" && log.OperationType == OpTypeCompensation && log.Status == StatusRunning:
			hasCompStart = true
		case log.StepID == "step1-compensate" && log.OperationType == OpTypeCompensation && log.Status == StatusSuccess:
			hasCompSuccess = true
		case log.StepID == "" && log.OperationType == OpTypeForward && log.Status == StatusFailed:
			hasSagaFail = true
		}

		if log.StepID == "step2" && log.OperationType == OpTypeCompensation {
			hasNoCompensationLogForStep2 = false
		}
	}

	if !hasStart {
		t.Error("missing saga start log")
	}
	if !hasStep1Start {
		t.Error("missing step1 start log")
	}
	if !hasStep1Success {
		t.Error("missing step1 success log")
	}
	if !hasStep2Start {
		t.Error("missing step2 start log")
	}
	if !hasStep2Fail {
		t.Error("missing step2 fail log")
	}
	if !hasCompStart {
		t.Error("missing compensation start log")
	}
	if !hasCompSuccess {
		t.Error("missing compensation success log")
	}
	if !hasSagaFail {
		t.Error("missing saga fail log")
	}
	if !hasNoCompensationLogForStep2 {
		t.Error("step2 (nil CompensateFunc) should not produce compensation log entries")
	}
}

func TestDataIsolationBetweenExecutions(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("isolation-saga", "Isolation Saga")

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			if existing, ok := data["counter"]; ok {
				return existing, nil
			}
			data["counter"] = 1
			return 1, nil
		},
		nil,
	)

	result1, _ := c.Execute(context.Background(), "isolation-saga", nil)
	result2, _ := c.Execute(context.Background(), "isolation-saga", nil)

	if result1.Data["counter"] != result2.Data["counter"] {
		t.Error("data should be isolated between executions")
	}

	if result1.ID == result2.ID {
		t.Error("execution IDs should be unique")
	}
}

func TestInitialDataNotModified(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("test-saga", "Test Saga")

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			data["modified"] = true
			return nil, nil
		},
		nil,
	)

	initialData := map[string]interface{}{
		"key": "value",
	}

	_, err := c.Execute(context.Background(), "test-saga", initialData)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if _, ok := initialData["modified"]; ok {
		t.Error("initial data map should not be modified")
	}
}

func TestEmptyInitialData(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("test-saga", "Test Saga")

	saga.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			if data == nil {
				return nil, errors.New("data should not be nil")
			}
			return nil, nil
		},
		nil,
	)

	result, err := c.Execute(context.Background(), "test-saga", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Status != StatusSuccess {
		t.Errorf("expected StatusSuccess, got %v", result.Status)
	}
}

func TestLargeNumberOfSteps(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("large-saga", "Large Saga")

	numSteps := 100
	for i := 0; i < numSteps; i++ {
		stepID := fmt.Sprintf("step-%d", i)
		err := saga.AddStep(stepID, fmt.Sprintf("Step %d", i),
			func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
				return i, nil
			},
			func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
				return nil, nil
			},
		)
		if err != nil {
			t.Fatalf("AddStep failed: %v", err)
		}
	}

	if len(saga.Steps) != numSteps {
		t.Errorf("expected %d steps, got %d", numSteps, len(saga.Steps))
	}

	result, err := c.Execute(context.Background(), "large-saga", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Status != StatusSuccess {
		t.Errorf("expected StatusSuccess, got %v", result.Status)
	}
	if len(result.StepResults) != numSteps {
		t.Errorf("expected %d step results, got %d", numSteps, len(result.StepResults))
	}

	logs, _ := c.GetLogs(result.ID)
	if len(logs) < numSteps*2 {
		t.Errorf("expected at least %d log entries, got %d", numSteps*2, len(logs))
	}
}

func TestStepOrderPreserved(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("order-saga", "Order Saga")

	order := make([]int, 0)
	var mu sync.Mutex

	for i := 0; i < 5; i++ {
		stepIndex := i
		err := saga.AddStep(fmt.Sprintf("step-%d", i), fmt.Sprintf("Step %d", i),
			func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
				mu.Lock()
				order = append(order, stepIndex)
				mu.Unlock()
				return nil, nil
			},
			nil,
		)
		if err != nil {
			t.Fatalf("AddStep failed: %v", err)
		}
	}

	_, err := c.Execute(context.Background(), "order-saga", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	expected := []int{0, 1, 2, 3, 4}
	if len(order) != len(expected) {
		t.Errorf("expected %d steps executed, got %d", len(expected), len(order))
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("expected step %d at index %d, got %d", v, i, order[i])
		}
	}
}

func TestCompensationOrderPreserved(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("comp-order-saga", "Compensation Order Saga")

	compOrder := make([]int, 0)
	var mu sync.Mutex

	for i := 0; i < 3; i++ {
		stepIndex := i
		err := saga.AddStep(fmt.Sprintf("step-%d", i), fmt.Sprintf("Step %d", i),
			func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
				return nil, nil
			},
			func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
				mu.Lock()
				compOrder = append(compOrder, stepIndex)
				mu.Unlock()
				return nil, nil
			},
		)
		if err != nil {
			t.Fatalf("AddStep failed: %v", err)
		}
	}

	saga.AddStep("fail-step", "Fail Step",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("failed")
		},
		nil,
	)

	_, err := c.Execute(context.Background(), "comp-order-saga", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	expected := []int{2, 1, 0}
	if len(compOrder) != len(expected) {
		t.Errorf("expected %d compensations executed, got %d", len(expected), len(compOrder))
	}
	for i, v := range expected {
		if compOrder[i] != v {
			t.Errorf("expected compensation step %d at index %d, got %d", v, i, compOrder[i])
		}
	}
}

func TestCompensationFailure_DistinctStepIDs(t *testing.T) {
	c := NewCoordinator()
	saga, _ := c.NewSaga("distinct-id-saga", "Distinct ID Saga")

	saga.AddStep("forward-op", "Forward Operation",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return "forward-result", nil
		},
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("compensation error")
		},
	)

	saga.AddStep("fail-step", "Fail Step",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, errors.New("trigger failure")
		},
		nil,
	)

	result, _ := c.Execute(context.Background(), "distinct-id-saga", nil)

	if len(result.InterventionNotes) != 1 {
		t.Fatalf("expected 1 intervention note, got %d", len(result.InterventionNotes))
	}

	note := result.InterventionNotes[0]
	if note.StepID == note.ForwardStepID {
		t.Errorf("StepID and ForwardStepID should be distinct, both are '%s'", note.StepID)
	}
	if note.ForwardStepID != "forward-op" {
		t.Errorf("expected ForwardStepID 'forward-op', got '%s'", note.ForwardStepID)
	}
	if note.StepID != "forward-op-compensate" {
		t.Errorf("expected StepID 'forward-op-compensate', got '%s'", note.StepID)
	}

	compResult, ok := result.Compensations["forward-op-compensate"]
	if !ok {
		t.Error("expected compensation result at key 'forward-op-compensate'")
	} else {
		if compResult.StepID != "forward-op-compensate" {
			t.Errorf("expected StepResult.StepID 'forward-op-compensate', got '%s'", compResult.StepID)
		}
	}
}

func TestErrorClassification(t *testing.T) {
	c := NewCoordinator()
	sagaNotFound, _ := c.NewSaga("saga-not-found", "Saga Not Found")

	_, err := c.GetSaga("nonexistent")
	if !errors.Is(err, ErrSagaNotFound) {
		t.Errorf("GetSaga: expected ErrSagaNotFound, got %v", err)
	}

	_, err = c.GetExecution("nonexistent")
	if !errors.Is(err, ErrExecutionNotFound) {
		t.Errorf("GetExecution: expected ErrExecutionNotFound, got %v", err)
	}

	_, err = c.GetLogs("nonexistent")
	if !errors.Is(err, ErrExecutionNotFound) {
		t.Errorf("GetLogs: expected ErrExecutionNotFound, got %v", err)
	}

	sagaNotFound.AddStep("step1", "Step 1",
		func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
			return nil, nil
		},
		nil,
	)

	execResult, _ := c.Execute(context.Background(), "saga-not-found", nil)

	_, err = c.GetExecution(execResult.ID)
	if err != nil {
		t.Errorf("GetExecution for existing: unexpected error %v", err)
	}

	_, err = c.GetLogs(execResult.ID)
	if err != nil {
		t.Errorf("GetLogs for existing: unexpected error %v", err)
	}

	if errors.Is(err, ErrSagaNotFound) {
		t.Error("GetLogs for existing execution should NOT return ErrSagaNotFound")
	}
}
