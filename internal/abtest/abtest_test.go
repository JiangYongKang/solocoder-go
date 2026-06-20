package abtest

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func TestNewABTest(t *testing.T) {
	ab := NewABTest()
	if ab == nil {
		t.Fatal("NewABTest returned nil")
	}
	if len(ab.experiments) != 0 {
		t.Errorf("expected 0 experiments, got %d", len(ab.experiments))
	}
	if len(ab.metrics) != 0 {
		t.Errorf("expected 0 metrics, got %d", len(ab.metrics))
	}
}

func TestHashBucket_EmptyUserID(t *testing.T) {
	_, err := HashBucket("")
	if !errors.Is(err, ErrEmptyUserID) {
		t.Errorf("expected ErrEmptyUserID, got %v", err)
	}
}

func TestHashBucket_ValidRange(t *testing.T) {
	for i := 0; i < 1000; i++ {
		userID := fmt.Sprintf("user-%d", i)
		bucket, err := HashBucket(userID)
		if err != nil {
			t.Fatalf("HashBucket error for %s: %v", userID, err)
		}
		if bucket < 0 || bucket >= BucketCount {
			t.Errorf("bucket %d out of range [0, %d) for user %s", bucket, BucketCount, userID)
		}
	}
}

func TestHashBucket_Stability(t *testing.T) {
	userID := "stable-user-123"
	expected, err := HashBucket(userID)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 100; i++ {
		bucket, err := HashBucket(userID)
		if err != nil {
			t.Fatal(err)
		}
		if bucket != expected {
			t.Errorf("hash not stable: expected %d, got %d on iteration %d", expected, bucket, i)
		}
	}
}

func TestHashBucket_Distribution(t *testing.T) {
	bucketCounts := make([]int, BucketCount)
	n := 100000

	for i := 0; i < n; i++ {
		userID := fmt.Sprintf("dist-user-%d", i)
		bucket, _ := HashBucket(userID)
		bucketCounts[bucket]++
	}

	expected := n / BucketCount
	tolerance := expected / 2

	for i, count := range bucketCounts {
		if count < expected-tolerance || count > expected+tolerance {
			t.Errorf("bucket %d has %d entries, expected ~%d (tolerance ±%d)", i, count, expected, tolerance)
		}
	}
}

func TestHashBucketWithExperiment_EmptyUserID(t *testing.T) {
	_, err := HashBucketWithExperiment("", "exp-1")
	if !errors.Is(err, ErrEmptyUserID) {
		t.Errorf("expected ErrEmptyUserID, got %v", err)
	}
}

func TestHashBucketWithExperiment_EmptyExperimentID(t *testing.T) {
	_, err := HashBucketWithExperiment("user-1", "")
	if !errors.Is(err, ErrEmptyExperimentID) {
		t.Errorf("expected ErrEmptyExperimentID, got %v", err)
	}
}

func TestHashBucketWithExperiment_ValidRange(t *testing.T) {
	for i := 0; i < 1000; i++ {
		userID := fmt.Sprintf("user-%d", i)
		expID := fmt.Sprintf("exp-%d", i%10)
		bucket, err := HashBucketWithExperiment(userID, expID)
		if err != nil {
			t.Fatalf("HashBucketWithExperiment error: %v", err)
		}
		if bucket < 0 || bucket >= BucketCount {
			t.Errorf("bucket %d out of range", bucket)
		}
	}
}

func TestHashBucketWithExperiment_Stability(t *testing.T) {
	userID := "stable-user-456"
	expID := "exp-stable-1"
	expected, err := HashBucketWithExperiment(userID, expID)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 100; i++ {
		bucket, err := HashBucketWithExperiment(userID, expID)
		if err != nil {
			t.Fatal(err)
		}
		if bucket != expected {
			t.Errorf("hash not stable: expected %d, got %d", expected, bucket)
		}
	}
}

func TestHashBucketWithExperiment_Orthogonal(t *testing.T) {
	userID := "orthogonal-user-789"
	expIDs := []string{"exp-a", "exp-b", "exp-c", "exp-d", "exp-e"}

	buckets := make(map[string]int)
	for _, expID := range expIDs {
		bucket, _ := HashBucketWithExperiment(userID, expID)
		buckets[expID] = bucket
	}

	uniqueBuckets := make(map[int]bool)
	for _, b := range buckets {
		uniqueBuckets[b] = true
	}

	if len(uniqueBuckets) < 3 {
		t.Errorf("expected at least 3 different buckets for 5 experiments, got %d. Buckets: %v", len(uniqueBuckets), buckets)
	}
}

func TestHashBucketWithExperiment_Independence(t *testing.T) {
	userID := "user-indep-1"
	bucket1, _ := HashBucketWithExperiment(userID, "exp-1")
	bucket2, _ := HashBucketWithExperiment(userID, "exp-2")

	if bucket1 == bucket2 {
		t.Logf("Note: buckets happen to be equal (%d == %d), testing more users...", bucket1, bucket2)
	}

	collisions := 0
	total := 1000
	for i := 0; i < total; i++ {
		uid := fmt.Sprintf("user-indep-%d", i)
		b1, _ := HashBucketWithExperiment(uid, "exp-1")
		b2, _ := HashBucketWithExperiment(uid, "exp-2")
		if b1 == b2 {
			collisions++
		}
	}

	expectedCollisions := total / 100
	tolerance := expectedCollisions * 3
	if collisions > expectedCollisions+tolerance {
		t.Errorf("too many collisions: %d/%d, expected ~%d", collisions, total, expectedCollisions)
	}
}

func TestAddExperiment_NilExperiment(t *testing.T) {
	ab := NewABTest()
	err := ab.AddExperiment(nil)
	if !errors.Is(err, ErrExperimentNotFound) {
		t.Errorf("expected ErrExperimentNotFound, got %v", err)
	}
}

func TestAddExperiment_EmptyID(t *testing.T) {
	ab := NewABTest()
	err := ab.AddExperiment(&Experiment{
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})
	if !errors.Is(err, ErrEmptyExperimentID) {
		t.Errorf("expected ErrEmptyExperimentID, got %v", err)
	}
}

func TestAddExperiment_NegativeExperimentPct(t *testing.T) {
	ab := NewABTest()
	err := ab.AddExperiment(&Experiment{
		ID:                 "exp-neg-1",
		ExperimentGroupPct: -10,
		ControlGroupPct:    50,
	})
	if !errors.Is(err, ErrInvalidTrafficPercent) {
		t.Errorf("expected ErrInvalidTrafficPercent, got %v", err)
	}
}

func TestAddExperiment_NegativeControlPct(t *testing.T) {
	ab := NewABTest()
	err := ab.AddExperiment(&Experiment{
		ID:                 "exp-neg-2",
		ExperimentGroupPct: 50,
		ControlGroupPct:    -10,
	})
	if !errors.Is(err, ErrInvalidTrafficPercent) {
		t.Errorf("expected ErrInvalidTrafficPercent, got %v", err)
	}
}

func TestAddExperiment_Exceeds100(t *testing.T) {
	ab := NewABTest()
	err := ab.AddExperiment(&Experiment{
		ID:                 "exp-over-1",
		ExperimentGroupPct: 60,
		ControlGroupPct:    50,
	})
	if !errors.Is(err, ErrTrafficExceedsLimit) {
		t.Errorf("expected ErrTrafficExceedsLimit, got %v", err)
	}
}

func TestAddExperiment_Success(t *testing.T) {
	ab := NewABTest()
	err := ab.AddExperiment(&Experiment{
		ID:                 "exp-success-1",
		ExperimentGroupPct: 30,
		ControlGroupPct:    30,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ab.experiments) != 1 {
		t.Errorf("expected 1 experiment, got %d", len(ab.experiments))
	}
}

func TestAddExperiment_DefaultGroupNames(t *testing.T) {
	ab := NewABTest()
	err := ab.AddExperiment(&Experiment{
		ID:                 "exp-default-names",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})
	if err != nil {
		t.Fatal(err)
	}

	exp, _ := ab.GetExperiment("exp-default-names")
	if exp.ExperimentGroupName != GroupExperiment {
		t.Errorf("expected default experiment group name '%s', got '%s'", GroupExperiment, exp.ExperimentGroupName)
	}
	if exp.ControlGroupName != GroupControl {
		t.Errorf("expected default control group name '%s', got '%s'", GroupControl, exp.ControlGroupName)
	}
}

func TestAddExperiment_CustomGroupNames(t *testing.T) {
	ab := NewABTest()
	err := ab.AddExperiment(&Experiment{
		ID:                  "exp-custom-names",
		ExperimentGroupPct:  50,
		ControlGroupPct:     50,
		ExperimentGroupName: "version_b",
		ControlGroupName:    "version_a",
	})
	if err != nil {
		t.Fatal(err)
	}

	exp, _ := ab.GetExperiment("exp-custom-names")
	if exp.ExperimentGroupName != "version_b" {
		t.Errorf("expected custom experiment group name 'version_b', got '%s'", exp.ExperimentGroupName)
	}
	if exp.ControlGroupName != "version_a" {
		t.Errorf("expected custom control group name 'version_a', got '%s'", exp.ControlGroupName)
	}
}

func TestAddExperiment_Duplicate(t *testing.T) {
	ab := NewABTest()
	err := ab.AddExperiment(&Experiment{
		ID:                 "exp-dup",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = ab.AddExperiment(&Experiment{
		ID:                 "exp-dup",
		ExperimentGroupPct: 30,
		ControlGroupPct:    30,
	})
	if !errors.Is(err, ErrExperimentExists) {
		t.Errorf("expected ErrExperimentExists, got %v", err)
	}
}

func TestAddExperiment_ZeroTraffic(t *testing.T) {
	ab := NewABTest()
	err := ab.AddExperiment(&Experiment{
		ID:                 "exp-zero",
		ExperimentGroupPct: 0,
		ControlGroupPct:    0,
	})
	if err != nil {
		t.Fatalf("unexpected error for zero traffic: %v", err)
	}
}

func TestAddExperiment_100PercentTraffic(t *testing.T) {
	ab := NewABTest()
	err := ab.AddExperiment(&Experiment{
		ID:                 "exp-100",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})
	if err != nil {
		t.Fatalf("unexpected error for 100%% traffic: %v", err)
	}
}

func TestRemoveExperiment_EmptyID(t *testing.T) {
	ab := NewABTest()
	err := ab.RemoveExperiment("")
	if !errors.Is(err, ErrEmptyExperimentID) {
		t.Errorf("expected ErrEmptyExperimentID, got %v", err)
	}
}

func TestRemoveExperiment_NotFound(t *testing.T) {
	ab := NewABTest()
	err := ab.RemoveExperiment("non-existent")
	if !errors.Is(err, ErrExperimentNotFound) {
		t.Errorf("expected ErrExperimentNotFound, got %v", err)
	}
}

func TestRemoveExperiment_Success(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-remove",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})

	if len(ab.experiments) != 1 {
		t.Fatal("experiment not added")
	}

	err := ab.RemoveExperiment("exp-remove")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ab.experiments) != 0 {
		t.Errorf("expected 0 experiments after removal, got %d", len(ab.experiments))
	}
	if len(ab.metrics) != 0 {
		t.Errorf("expected 0 metrics after removal, got %d", len(ab.metrics))
	}
}

func TestGetExperiment_EmptyID(t *testing.T) {
	ab := NewABTest()
	_, err := ab.GetExperiment("")
	if !errors.Is(err, ErrEmptyExperimentID) {
		t.Errorf("expected ErrEmptyExperimentID, got %v", err)
	}
}

func TestGetExperiment_NotFound(t *testing.T) {
	ab := NewABTest()
	_, err := ab.GetExperiment("non-existent")
	if !errors.Is(err, ErrExperimentNotFound) {
		t.Errorf("expected ErrExperimentNotFound, got %v", err)
	}
}

func TestGetExperiment_Success(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-get",
		ExperimentGroupPct: 30,
		ControlGroupPct:    20,
	})

	exp, err := ab.GetExperiment("exp-get")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exp.ID != "exp-get" {
		t.Errorf("expected ID 'exp-get', got '%s'", exp.ID)
	}
	if exp.ExperimentGroupPct != 30 {
		t.Errorf("expected ExperimentGroupPct 30, got %d", exp.ExperimentGroupPct)
	}
	if exp.ControlGroupPct != 20 {
		t.Errorf("expected ControlGroupPct 20, got %d", exp.ControlGroupPct)
	}
}

func TestGetExperiment_ReturnsCopy(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                  "exp-copy",
		ExperimentGroupPct:  30,
		ControlGroupPct:     30,
		ExperimentGroupName: "exp_group",
		ControlGroupName:    "ctrl_group",
	})

	exp, _ := ab.GetExperiment("exp-copy")
	exp.ExperimentGroupPct = 100
	exp.ControlGroupPct = 0
	exp.ExperimentGroupName = "modified"
	exp.ControlGroupName = "modified_ctrl"

	internalExp, _ := ab.GetExperiment("exp-copy")
	if internalExp.ExperimentGroupPct != 30 {
		t.Errorf("internal experiment should not be modified, got ExperimentGroupPct %d", internalExp.ExperimentGroupPct)
	}
	if internalExp.ControlGroupPct != 30 {
		t.Errorf("internal experiment should not be modified, got ControlGroupPct %d", internalExp.ControlGroupPct)
	}
	if internalExp.ExperimentGroupName != "exp_group" {
		t.Errorf("internal experiment should not be modified, got ExperimentGroupName %s", internalExp.ExperimentGroupName)
	}
	if internalExp.ControlGroupName != "ctrl_group" {
		t.Errorf("internal experiment should not be modified, got ControlGroupName %s", internalExp.ControlGroupName)
	}
}

func TestListExperiments_ReturnsCopies(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-list-copy",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})

	exps := ab.ListExperiments()
	if len(exps) != 1 {
		t.Fatalf("expected 1 experiment, got %d", len(exps))
	}

	exps[0].ExperimentGroupPct = 100
	exps[0].ControlGroupPct = 0

	internalExps := ab.ListExperiments()
	if internalExps[0].ExperimentGroupPct != 50 {
		t.Errorf("internal experiment should not be modified, got ExperimentGroupPct %d", internalExps[0].ExperimentGroupPct)
	}
	if internalExps[0].ControlGroupPct != 50 {
		t.Errorf("internal experiment should not be modified, got ControlGroupPct %d", internalExps[0].ControlGroupPct)
	}
}

func TestAddExperiment_StoresCopy(t *testing.T) {
	ab := NewABTest()
	exp := &Experiment{
		ID:                 "exp-store-copy",
		ExperimentGroupPct: 10,
		ControlGroupPct:    20,
	}

	err := ab.AddExperiment(exp)
	if err != nil {
		t.Fatal(err)
	}

	exp.ExperimentGroupPct = 100
	exp.ControlGroupPct = 0

	internalExp, _ := ab.GetExperiment("exp-store-copy")
	if internalExp.ExperimentGroupPct != 10 {
		t.Errorf("stored experiment should not be affected by caller modification, got ExperimentGroupPct %d", internalExp.ExperimentGroupPct)
	}
	if internalExp.ControlGroupPct != 20 {
		t.Errorf("stored experiment should not be affected by caller modification, got ControlGroupPct %d", internalExp.ControlGroupPct)
	}
}

func TestConcurrent_ReadAndModifyExperimentPointer(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-race",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})

	var wg sync.WaitGroup
	numGoroutines := 20
	iterations := 100

	stop := make(chan struct{})

	for g := 0; g < numGoroutines/2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				select {
				case <-stop:
					return
				default:
					exp, _ := ab.GetExperiment("exp-race")
					_ = exp.ExperimentGroupPct
					_ = exp.ControlGroupPct
				}
			}
		}()
	}

	for g := 0; g < numGoroutines/2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				select {
				case <-stop:
					return
				default:
					exp, _ := ab.GetExperiment("exp-race")
					exp.ExperimentGroupPct = 999
					exp.ControlGroupPct = 999
					_ = exp
				}
			}
		}()
	}

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				select {
				case <-stop:
					return
				default:
					userID := fmt.Sprintf("user-race-%d-%d", gid, i)
					_, _ = ab.AssignGroup(userID, "exp-race")
				}
			}
		}(g)
	}

	wg.Wait()
	close(stop)

	internalExp, _ := ab.GetExperiment("exp-race")
	if internalExp.ExperimentGroupPct != 50 {
		t.Errorf("internal experiment should not be modified by concurrent readers, got %d", internalExp.ExperimentGroupPct)
	}
	if internalExp.ControlGroupPct != 50 {
		t.Errorf("internal experiment should not be modified by concurrent readers, got %d", internalExp.ControlGroupPct)
	}
}

func TestListExperiments_Empty(t *testing.T) {
	ab := NewABTest()
	exps := ab.ListExperiments()
	if len(exps) != 0 {
		t.Errorf("expected 0 experiments, got %d", len(exps))
	}
}

func TestListExperiments_Multiple(t *testing.T) {
	ab := NewABTest()
	for i := 0; i < 5; i++ {
		_ = ab.AddExperiment(&Experiment{
			ID:                 fmt.Sprintf("exp-list-%d", i),
			ExperimentGroupPct: 20,
			ControlGroupPct:    30,
		})
	}

	exps := ab.ListExperiments()
	if len(exps) != 5 {
		t.Errorf("expected 5 experiments, got %d", len(exps))
	}
}

func TestAssignGroup_EmptyUserID(t *testing.T) {
	ab := NewABTest()
	_, err := ab.AssignGroup("", "exp-1")
	if !errors.Is(err, ErrEmptyUserID) {
		t.Errorf("expected ErrEmptyUserID, got %v", err)
	}
}

func TestAssignGroup_EmptyExperimentID(t *testing.T) {
	ab := NewABTest()
	_, err := ab.AssignGroup("user-1", "")
	if !errors.Is(err, ErrEmptyExperimentID) {
		t.Errorf("expected ErrEmptyExperimentID, got %v", err)
	}
}

func TestAssignGroup_ExperimentNotFound(t *testing.T) {
	ab := NewABTest()
	_, err := ab.AssignGroup("user-1", "non-existent")
	if !errors.Is(err, ErrExperimentNotFound) {
		t.Errorf("expected ErrExperimentNotFound, got %v", err)
	}
}

func TestAssignGroup_AllExperiment(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-all-exp",
		ExperimentGroupPct: 100,
		ControlGroupPct:    0,
	})

	for i := 0; i < 100; i++ {
		userID := fmt.Sprintf("user-all-exp-%d", i)
		group, err := ab.AssignGroup(userID, "exp-all-exp")
		if err != nil {
			t.Fatal(err)
		}
		if group != GroupExperiment {
			t.Errorf("expected '%s', got '%s' for user %s", GroupExperiment, group, userID)
		}
	}
}

func TestAssignGroup_AllControl(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-all-ctrl",
		ExperimentGroupPct: 0,
		ControlGroupPct:    100,
	})

	for i := 0; i < 100; i++ {
		userID := fmt.Sprintf("user-all-ctrl-%d", i)
		group, err := ab.AssignGroup(userID, "exp-all-ctrl")
		if err != nil {
			t.Fatal(err)
		}
		if group != GroupControl {
			t.Errorf("expected '%s', got '%s' for user %s", GroupControl, group, userID)
		}
	}
}

func TestAssignGroup_AllNoAssign(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-none",
		ExperimentGroupPct: 0,
		ControlGroupPct:    0,
	})

	for i := 0; i < 100; i++ {
		userID := fmt.Sprintf("user-none-%d", i)
		group, err := ab.AssignGroup(userID, "exp-none")
		if err != nil {
			t.Fatal(err)
		}
		if group != GroupNoAssign {
			t.Errorf("expected '%s', got '%s' for user %s", GroupNoAssign, group, userID)
		}
	}
}

func TestAssignGroup_5050Split(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-5050",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})

	expCount := 0
	ctrlCount := 0
	n := 10000

	for i := 0; i < n; i++ {
		userID := fmt.Sprintf("user-5050-%d", i)
		group, _ := ab.AssignGroup(userID, "exp-5050")
		if group == GroupExperiment {
			expCount++
		} else if group == GroupControl {
			ctrlCount++
		}
	}

	total := expCount + ctrlCount
	if total != n {
		t.Errorf("expected all users assigned (exp+ctrl=%d), got %d", n, total)
	}

	expected := n / 2
	tolerance := expected / 10

	if expCount < expected-tolerance || expCount > expected+tolerance {
		t.Errorf("experiment group has %d users, expected ~%d (±%d)", expCount, expected, tolerance)
	}
	if ctrlCount < expected-tolerance || ctrlCount > expected+tolerance {
		t.Errorf("control group has %d users, expected ~%d (±%d)", ctrlCount, expected, tolerance)
	}
}

func TestAssignGroup_3020Split(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-3020",
		ExperimentGroupPct: 30,
		ControlGroupPct:    20,
	})

	expCount := 0
	ctrlCount := 0
	noAssignCount := 0
	n := 10000

	for i := 0; i < n; i++ {
		userID := fmt.Sprintf("user-3020-%d", i)
		group, _ := ab.AssignGroup(userID, "exp-3020")
		switch group {
		case GroupExperiment:
			expCount++
		case GroupControl:
			ctrlCount++
		case GroupNoAssign:
			noAssignCount++
		}
	}

	expectedExp := n * 30 / 100
	expectedCtrl := n * 20 / 100
	expectedNoAssign := n * 50 / 100
	tolerance := expectedExp / 10

	if expCount < expectedExp-tolerance || expCount > expectedExp+tolerance {
		t.Errorf("experiment group has %d users, expected ~%d", expCount, expectedExp)
	}
	if ctrlCount < expectedCtrl-tolerance || ctrlCount > expectedCtrl+tolerance {
		t.Errorf("control group has %d users, expected ~%d", ctrlCount, expectedCtrl)
	}
	if noAssignCount < expectedNoAssign-tolerance*2 || noAssignCount > expectedNoAssign+tolerance*2 {
		t.Errorf("no_assign group has %d users, expected ~%d", noAssignCount, expectedNoAssign)
	}
}

func TestAssignGroup_Stability(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-stable-assign",
		ExperimentGroupPct: 30,
		ControlGroupPct:    30,
	})

	userID := "user-stable-assign"
	expected, _ := ab.AssignGroup(userID, "exp-stable-assign")

	for i := 0; i < 100; i++ {
		group, err := ab.AssignGroup(userID, "exp-stable-assign")
		if err != nil {
			t.Fatal(err)
		}
		if group != expected {
			t.Errorf("group assignment not stable: expected '%s', got '%s'", expected, group)
		}
	}
}

func TestAssignGroup_CustomGroupNames(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                  "exp-custom",
		ExperimentGroupPct:  100,
		ControlGroupPct:     0,
		ExperimentGroupName: "new_ui",
		ControlGroupName:    "old_ui",
	})

	group, err := ab.AssignGroup("user-custom", "exp-custom")
	if err != nil {
		t.Fatal(err)
	}
	if group != "new_ui" {
		t.Errorf("expected 'new_ui', got '%s'", group)
	}
}

func TestAssignAllExperiments_EmptyUserID(t *testing.T) {
	ab := NewABTest()
	_, err := ab.AssignAllExperiments("")
	if !errors.Is(err, ErrEmptyUserID) {
		t.Errorf("expected ErrEmptyUserID, got %v", err)
	}
}

func TestAssignAllExperiments_NoExperiments(t *testing.T) {
	ab := NewABTest()
	result, err := ab.AssignAllExperiments("user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestAssignAllExperiments_MultipleExperiments(t *testing.T) {
	ab := NewABTest()
	for i := 0; i < 3; i++ {
		_ = ab.AddExperiment(&Experiment{
			ID:                 fmt.Sprintf("exp-all-%d", i),
			ExperimentGroupPct: 50,
			ControlGroupPct:    50,
		})
	}

	result, err := ab.AssignAllExperiments("user-multi")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 entries, got %d", len(result))
	}

	for i := 0; i < 3; i++ {
		expID := fmt.Sprintf("exp-all-%d", i)
		group, exists := result[expID]
		if !exists {
			t.Errorf("missing experiment %s in result", expID)
		}
		if group != GroupExperiment && group != GroupControl {
			t.Errorf("unexpected group '%s' for experiment %s", group, expID)
		}
	}
}

func TestAssignAllExperiments_Orthogonal(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-ortho-a",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-ortho-b",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})

	counts := map[[2]string]int{}
	total := 100000
	for i := 0; i < total; i++ {
		userID := fmt.Sprintf("user-%d-ortho-%d", i, i*i)
		result, _ := ab.AssignAllExperiments(userID)
		key := [2]string{result["exp-ortho-a"], result["exp-ortho-b"]}
		counts[key]++
	}

	expExp := counts[[2]string{GroupExperiment, GroupExperiment}]
	expCtrl := counts[[2]string{GroupExperiment, GroupControl}]
	ctrlExp := counts[[2]string{GroupControl, GroupExperiment}]
	ctrlCtrl := counts[[2]string{GroupControl, GroupControl}]

	expected := total / 4
	tolerance := expected / 5

	if expExp < expected-tolerance || expExp > expected+tolerance {
		t.Errorf("exp/exp count: %d, expected ~%d", expExp, expected)
	}
	if expCtrl < expected-tolerance || expCtrl > expected+tolerance {
		t.Errorf("exp/ctrl count: %d, expected ~%d", expCtrl, expected)
	}
	if ctrlExp < expected-tolerance || ctrlExp > expected+tolerance {
		t.Errorf("ctrl/exp count: %d, expected ~%d", ctrlExp, expected)
	}
	if ctrlCtrl < expected-tolerance || ctrlCtrl > expected+tolerance {
		t.Errorf("ctrl/ctrl count: %d, expected ~%d", ctrlCtrl, expected)
	}
}

func TestRecordMetric_EmptyExperimentID(t *testing.T) {
	ab := NewABTest()
	err := ab.RecordMetric("", GroupExperiment, "click", 1.0)
	if !errors.Is(err, ErrEmptyExperimentID) {
		t.Errorf("expected ErrEmptyExperimentID, got %v", err)
	}
}

func TestRecordMetric_EmptyGroupName(t *testing.T) {
	ab := NewABTest()
	err := ab.RecordMetric("exp-1", "", "click", 1.0)
	if !errors.Is(err, ErrEmptyGroupName) {
		t.Errorf("expected ErrEmptyGroupName, got %v", err)
	}
}

func TestRecordMetric_EmptyMetricName(t *testing.T) {
	ab := NewABTest()
	err := ab.RecordMetric("exp-1", GroupExperiment, "", 1.0)
	if !errors.Is(err, ErrEmptyMetricName) {
		t.Errorf("expected ErrEmptyMetricName, got %v", err)
	}
}

func TestRecordMetric_ExperimentNotFound(t *testing.T) {
	ab := NewABTest()
	err := ab.RecordMetric("non-existent", GroupExperiment, "click", 1.0)
	if !errors.Is(err, ErrExperimentNotFound) {
		t.Errorf("expected ErrExperimentNotFound, got %v", err)
	}
}

func TestRecordMetric_GroupNotFound(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-metric-group",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})

	err := ab.RecordMetric("exp-metric-group", "non-existent-group", "click", 1.0)
	if err == nil {
		t.Error("expected error for non-existent group, got nil")
	}
}

func TestRecordMetric_Success(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-metric-success",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})

	err := ab.RecordMetric("exp-metric-success", GroupExperiment, "click", 1.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count, sum, err := ab.GetGroupMetric("exp-metric-success", GroupExperiment, "click")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
	if sum != 1.0 {
		t.Errorf("expected sum 1.0, got %f", sum)
	}
}

func TestRecordMetric_Multiple(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-metric-multi",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})

	for i := 0; i < 10; i++ {
		_ = ab.RecordMetric("exp-metric-multi", GroupExperiment, "click", float64(i+1))
	}

	for i := 0; i < 5; i++ {
		_ = ab.RecordMetric("exp-metric-multi", GroupControl, "click", float64(i+1))
	}

	expCount, expSum, _ := ab.GetGroupMetric("exp-metric-multi", GroupExperiment, "click")
	ctrlCount, ctrlSum, _ := ab.GetGroupMetric("exp-metric-multi", GroupControl, "click")

	if expCount != 10 {
		t.Errorf("expected experiment count 10, got %d", expCount)
	}
	if expSum != 55.0 {
		t.Errorf("expected experiment sum 55.0, got %f", expSum)
	}
	if ctrlCount != 5 {
		t.Errorf("expected control count 5, got %d", ctrlCount)
	}
	if ctrlSum != 15.0 {
		t.Errorf("expected control sum 15.0, got %f", ctrlSum)
	}
}

func TestRecordMetric_NoAssignGroup(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-metric-noassign",
		ExperimentGroupPct: 0,
		ControlGroupPct:    0,
	})

	err := ab.RecordMetric("exp-metric-noassign", GroupNoAssign, "impression", 1.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count, sum, _ := ab.GetGroupMetric("exp-metric-noassign", GroupNoAssign, "impression")
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
	if sum != 1.0 {
		t.Errorf("expected sum 1.0, got %f", sum)
	}
}

func TestRecordMetric_NegativeValue(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-metric-neg",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})

	err := ab.RecordMetric("exp-metric-neg", GroupExperiment, "error_rate", -0.5)
	if err != nil {
		t.Fatalf("unexpected error for negative value: %v", err)
	}

	_, sum, _ := ab.GetGroupMetric("exp-metric-neg", GroupExperiment, "error_rate")
	if sum != -0.5 {
		t.Errorf("expected sum -0.5, got %f", sum)
	}
}

func TestGetExperimentMetrics_EmptyID(t *testing.T) {
	ab := NewABTest()
	_, err := ab.GetExperimentMetrics("")
	if !errors.Is(err, ErrEmptyExperimentID) {
		t.Errorf("expected ErrEmptyExperimentID, got %v", err)
	}
}

func TestGetExperimentMetrics_NotFound(t *testing.T) {
	ab := NewABTest()
	_, err := ab.GetExperimentMetrics("non-existent")
	if !errors.Is(err, ErrExperimentNotFound) {
		t.Errorf("expected ErrExperimentNotFound, got %v", err)
	}
}

func TestGetExperimentMetrics_Success(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-get-metrics",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})

	_ = ab.RecordMetric("exp-get-metrics", GroupExperiment, "click", 1.0)
	_ = ab.RecordMetric("exp-get-metrics", GroupExperiment, "click", 2.0)
	_ = ab.RecordMetric("exp-get-metrics", GroupControl, "click", 3.0)

	metrics, err := ab.GetExperimentMetrics("exp-get-metrics")
	if err != nil {
		t.Fatal(err)
	}

	if metrics.GroupMetrics[GroupExperiment].EventCount["click"] != 2 {
		t.Errorf("expected experiment event count 2, got %d", metrics.GroupMetrics[GroupExperiment].EventCount["click"])
	}
	if metrics.GroupMetrics[GroupExperiment].MetricSum["click"] != 3.0 {
		t.Errorf("expected experiment sum 3.0, got %f", metrics.GroupMetrics[GroupExperiment].MetricSum["click"])
	}
	if metrics.GroupMetrics[GroupControl].EventCount["click"] != 1 {
		t.Errorf("expected control event count 1, got %d", metrics.GroupMetrics[GroupControl].EventCount["click"])
	}
	if metrics.GroupMetrics[GroupControl].MetricSum["click"] != 3.0 {
		t.Errorf("expected control sum 3.0, got %f", metrics.GroupMetrics[GroupControl].MetricSum["click"])
	}
}

func TestGetExperimentMetrics_ReturnsCopy(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-get-metrics-copy",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})

	_ = ab.RecordMetric("exp-get-metrics-copy", GroupExperiment, "click", 1.0)

	metrics, _ := ab.GetExperimentMetrics("exp-get-metrics-copy")
	metrics.GroupMetrics[GroupExperiment].EventCount["click"] = 999
	metrics.GroupMetrics[GroupExperiment].MetricSum["click"] = 999.0

	count, sum, _ := ab.GetGroupMetric("exp-get-metrics-copy", GroupExperiment, "click")
	if count != 1 {
		t.Errorf("original data should not be modified, count is %d", count)
	}
	if sum != 1.0 {
		t.Errorf("original data should not be modified, sum is %f", sum)
	}
}

func TestGetGroupMetric_EmptyExperimentID(t *testing.T) {
	ab := NewABTest()
	_, _, err := ab.GetGroupMetric("", GroupExperiment, "click")
	if !errors.Is(err, ErrEmptyExperimentID) {
		t.Errorf("expected ErrEmptyExperimentID, got %v", err)
	}
}

func TestGetGroupMetric_EmptyGroupName(t *testing.T) {
	ab := NewABTest()
	_, _, err := ab.GetGroupMetric("exp-1", "", "click")
	if !errors.Is(err, ErrEmptyGroupName) {
		t.Errorf("expected ErrEmptyGroupName, got %v", err)
	}
}

func TestGetGroupMetric_EmptyMetricName(t *testing.T) {
	ab := NewABTest()
	_, _, err := ab.GetGroupMetric("exp-1", GroupExperiment, "")
	if !errors.Is(err, ErrEmptyMetricName) {
		t.Errorf("expected ErrEmptyMetricName, got %v", err)
	}
}

func TestGetGroupMetric_ExperimentNotFound(t *testing.T) {
	ab := NewABTest()
	_, _, err := ab.GetGroupMetric("non-existent", GroupExperiment, "click")
	if !errors.Is(err, ErrExperimentNotFound) {
		t.Errorf("expected ErrExperimentNotFound, got %v", err)
	}
}

func TestGetGroupMetric_GroupNotFound(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-group-not-found",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})

	_, _, err := ab.GetGroupMetric("exp-group-not-found", "non-existent", "click")
	if err == nil {
		t.Error("expected error for non-existent group, got nil")
	}
}

func TestGetGroupMetric_MetricNotRecorded(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-metric-not-recorded",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})

	count, sum, err := ab.GetGroupMetric("exp-metric-not-recorded", GroupExperiment, "not_recorded")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
	if sum != 0.0 {
		t.Errorf("expected sum 0.0, got %f", sum)
	}
}

func TestResetExperimentMetrics_EmptyID(t *testing.T) {
	ab := NewABTest()
	err := ab.ResetExperimentMetrics("")
	if !errors.Is(err, ErrEmptyExperimentID) {
		t.Errorf("expected ErrEmptyExperimentID, got %v", err)
	}
}

func TestResetExperimentMetrics_NotFound(t *testing.T) {
	ab := NewABTest()
	err := ab.ResetExperimentMetrics("non-existent")
	if !errors.Is(err, ErrExperimentNotFound) {
		t.Errorf("expected ErrExperimentNotFound, got %v", err)
	}
}

func TestResetExperimentMetrics_Success(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-reset",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})

	_ = ab.RecordMetric("exp-reset", GroupExperiment, "click", 1.0)
	_ = ab.RecordMetric("exp-reset", GroupControl, "click", 2.0)

	count, _, _ := ab.GetGroupMetric("exp-reset", GroupExperiment, "click")
	if count != 1 {
		t.Fatal("metric not recorded")
	}

	err := ab.ResetExperimentMetrics("exp-reset")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count, sum, _ := ab.GetGroupMetric("exp-reset", GroupExperiment, "click")
	if count != 0 {
		t.Errorf("expected count 0 after reset, got %d", count)
	}
	if sum != 0.0 {
		t.Errorf("expected sum 0.0 after reset, got %f", sum)
	}

	count, sum, _ = ab.GetGroupMetric("exp-reset", GroupControl, "click")
	if count != 0 {
		t.Errorf("expected control count 0 after reset, got %d", count)
	}
	if sum != 0.0 {
		t.Errorf("expected control sum 0.0 after reset, got %f", sum)
	}
}

func TestConcurrent_AddExperiment(t *testing.T) {
	ab := NewABTest()
	var wg sync.WaitGroup
	numGoroutines := 20

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			expID := fmt.Sprintf("exp-concurrent-add-%d", gid)
			_ = ab.AddExperiment(&Experiment{
				ID:                 expID,
				ExperimentGroupPct: 50,
				ControlGroupPct:    50,
			})
		}(g)
	}

	wg.Wait()

	if len(ab.experiments) != numGoroutines {
		t.Errorf("expected %d experiments, got %d", numGoroutines, len(ab.experiments))
	}
}

func TestConcurrent_AssignGroup(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-concurrent-assign",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})

	var wg sync.WaitGroup
	numGoroutines := 50
	iterations := 100

	var expCount int64
	var ctrlCount int64

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				userID := fmt.Sprintf("user-concurrent-%d-%d", gid, i)
				group, err := ab.AssignGroup(userID, "exp-concurrent-assign")
				if err != nil {
					return
				}
				if group == GroupExperiment {
					atomic.AddInt64(&expCount, 1)
				} else if group == GroupControl {
					atomic.AddInt64(&ctrlCount, 1)
				}
			}
		}(g)
	}

	wg.Wait()

	total := expCount + ctrlCount
	expected := int64(numGoroutines * iterations)
	if total != expected {
		t.Errorf("expected %d total assignments, got %d", expected, total)
	}
}

func TestConcurrent_RecordMetric(t *testing.T) {
	ab := NewABTest()
	_ = ab.AddExperiment(&Experiment{
		ID:                 "exp-concurrent-metric",
		ExperimentGroupPct: 50,
		ControlGroupPct:    50,
	})

	var wg sync.WaitGroup
	numGoroutines := 30
	iterations := 200

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = ab.RecordMetric("exp-concurrent-metric", GroupExperiment, "click", 1.0)
			}
		}(g)
	}

	wg.Wait()

	count, sum, _ := ab.GetGroupMetric("exp-concurrent-metric", GroupExperiment, "click")
	expected := int64(numGoroutines * iterations)
	if count != expected {
		t.Errorf("expected count %d, got %d", expected, count)
	}
	if sum != float64(expected) {
		t.Errorf("expected sum %f, got %f", float64(expected), sum)
	}
}

func TestFullWorkflow(t *testing.T) {
	ab := NewABTest()

	err := ab.AddExperiment(&Experiment{
		ID:                  "button_color_test",
		ExperimentGroupPct:  30,
		ControlGroupPct:     30,
		ExperimentGroupName: "green_button",
		ControlGroupName:    "blue_button",
	})
	if err != nil {
		t.Fatal(err)
	}

	userID := "user-workflow-123"
	group, err := ab.AssignGroup(userID, "button_color_test")
	if err != nil {
		t.Fatal(err)
	}

	if group != "green_button" && group != "blue_button" && group != GroupNoAssign {
		t.Errorf("unexpected group: %s", group)
	}

	for i := 0; i < 100; i++ {
		uid := fmt.Sprintf("user-workflow-%d", i)
		g, _ := ab.AssignGroup(uid, "button_color_test")
		_ = ab.RecordMetric("button_color_test", g, "page_view", 1.0)
		if g == "green_button" || g == "blue_button" {
			if i%10 == 0 {
				_ = ab.RecordMetric("button_color_test", g, "click", 1.0)
			}
		}
	}

	metrics, _ := ab.GetExperimentMetrics("button_color_test")

	for gName, gm := range metrics.GroupMetrics {
		pvCount := gm.EventCount["page_view"]
		clickCount := gm.EventCount["click"]
		t.Logf("Group %s: page_views=%d, clicks=%d", gName, pvCount, clickCount)
	}
}

func TestHashBucket_DeterministicWithDifferentIDs(t *testing.T) {
	users := []string{"alice", "bob", "charlie", "david", "eve", "frank", "grace", "henry"}
	buckets := make(map[string]int)

	for _, user := range users {
		b, _ := HashBucket(user)
		buckets[user] = b
	}

	for _, user := range users {
		b, _ := HashBucket(user)
		if b != buckets[user] {
			t.Errorf("hash not deterministic for %s: expected %d, got %d", user, buckets[user], b)
		}
	}
}

func TestHashBucketWithExperiment_DifferentExperimentsDifferentBuckets(t *testing.T) {
	userID := "test-user-xyz"
	experiments := []string{"exp-1", "exp-2", "exp-3", "exp-4", "exp-5"}

	firstBucket, _ := HashBucketWithExperiment(userID, experiments[0])
	allSame := true

	for _, exp := range experiments[1:] {
		b, _ := HashBucketWithExperiment(userID, exp)
		if b != firstBucket {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("all experiments returned the same bucket, orthogonality may be broken")
	}
}
