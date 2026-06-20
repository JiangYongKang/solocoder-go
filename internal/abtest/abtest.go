package abtest

import (
	"errors"
	"fmt"
	"hash/fnv"
	"sync"
)

var (
	ErrEmptyUserID           = errors.New("abtest: empty user id")
	ErrEmptyExperimentID     = errors.New("abtest: empty experiment id")
	ErrExperimentNotFound    = errors.New("abtest: experiment not found")
	ErrExperimentExists      = errors.New("abtest: experiment already exists")
	ErrInvalidTrafficPercent = errors.New("abtest: invalid traffic percentage")
	ErrTrafficExceedsLimit   = errors.New("abtest: traffic percentage exceeds 100")
	ErrEmptyGroupName        = errors.New("abtest: empty group name")
	ErrEmptyMetricName       = errors.New("abtest: empty metric name")
)

const (
	BucketCount = 100

	GroupControl    = "control"
	GroupExperiment = "experiment"
	GroupNoAssign   = "no_assign"
)

type Experiment struct {
	ID                  string
	ExperimentGroupPct  int
	ControlGroupPct     int
	ExperimentGroupName string
	ControlGroupName    string
}

type GroupMetrics struct {
	EventCount map[string]int64
	MetricSum  map[string]float64
}

type ExperimentMetrics struct {
	GroupMetrics map[string]*GroupMetrics
}

type ABTest struct {
	mu          sync.RWMutex
	experiments map[string]*Experiment
	metrics     map[string]*ExperimentMetrics
}

func NewABTest() *ABTest {
	return &ABTest{
		experiments: make(map[string]*Experiment),
		metrics:     make(map[string]*ExperimentMetrics),
	}
}

func HashBucket(userID string) (int, error) {
	if userID == "" {
		return 0, ErrEmptyUserID
	}
	h := fnv.New32a()
	h.Write([]byte(userID))
	hash := h.Sum32()
	return int(hash % uint32(BucketCount)), nil
}

func HashBucketWithExperiment(userID, experimentID string) (int, error) {
	if userID == "" {
		return 0, ErrEmptyUserID
	}
	if experimentID == "" {
		return 0, ErrEmptyExperimentID
	}

	h1 := fnv.New32a()
	h1.Write([]byte(userID))
	userHash := h1.Sum32()

	h2 := fnv.New32a()
	h2.Write([]byte(experimentID))
	expHash := h2.Sum32()

	mixed := userHash ^ (expHash * 2654435761)
	mixed = (mixed ^ (mixed >> 16)) * 2246822507
	mixed = (mixed ^ (mixed >> 13)) * 3266489909
	mixed = mixed ^ (mixed >> 16)

	return int(mixed % uint32(BucketCount)), nil
}

func (ab *ABTest) AddExperiment(exp *Experiment) error {
	if exp == nil {
		return ErrExperimentNotFound
	}
	if exp.ID == "" {
		return ErrEmptyExperimentID
	}
	if exp.ExperimentGroupPct < 0 || exp.ControlGroupPct < 0 {
		return ErrInvalidTrafficPercent
	}
	if exp.ExperimentGroupPct+exp.ControlGroupPct > BucketCount {
		return ErrTrafficExceedsLimit
	}
	if exp.ExperimentGroupName == "" {
		exp.ExperimentGroupName = GroupExperiment
	}
	if exp.ControlGroupName == "" {
		exp.ControlGroupName = GroupControl
	}

	ab.mu.Lock()
	defer ab.mu.Unlock()

	if _, exists := ab.experiments[exp.ID]; exists {
		return ErrExperimentExists
	}

	ab.experiments[exp.ID] = &Experiment{
		ID:                  exp.ID,
		ExperimentGroupPct:  exp.ExperimentGroupPct,
		ControlGroupPct:     exp.ControlGroupPct,
		ExperimentGroupName: exp.ExperimentGroupName,
		ControlGroupName:    exp.ControlGroupName,
	}
	ab.metrics[exp.ID] = &ExperimentMetrics{
		GroupMetrics: make(map[string]*GroupMetrics),
	}
	ab.metrics[exp.ID].GroupMetrics[exp.ExperimentGroupName] = &GroupMetrics{
		EventCount: make(map[string]int64),
		MetricSum:  make(map[string]float64),
	}
	ab.metrics[exp.ID].GroupMetrics[exp.ControlGroupName] = &GroupMetrics{
		EventCount: make(map[string]int64),
		MetricSum:  make(map[string]float64),
	}
	ab.metrics[exp.ID].GroupMetrics[GroupNoAssign] = &GroupMetrics{
		EventCount: make(map[string]int64),
		MetricSum:  make(map[string]float64),
	}

	return nil
}

func (ab *ABTest) RemoveExperiment(experimentID string) error {
	if experimentID == "" {
		return ErrEmptyExperimentID
	}

	ab.mu.Lock()
	defer ab.mu.Unlock()

	if _, exists := ab.experiments[experimentID]; !exists {
		return ErrExperimentNotFound
	}

	delete(ab.experiments, experimentID)
	delete(ab.metrics, experimentID)

	return nil
}

func (ab *ABTest) GetExperiment(experimentID string) (*Experiment, error) {
	if experimentID == "" {
		return nil, ErrEmptyExperimentID
	}

	ab.mu.RLock()
	defer ab.mu.RUnlock()

	exp, exists := ab.experiments[experimentID]
	if !exists {
		return nil, ErrExperimentNotFound
	}
	return &Experiment{
		ID:                  exp.ID,
		ExperimentGroupPct:  exp.ExperimentGroupPct,
		ControlGroupPct:     exp.ControlGroupPct,
		ExperimentGroupName: exp.ExperimentGroupName,
		ControlGroupName:    exp.ControlGroupName,
	}, nil
}

func (ab *ABTest) ListExperiments() []*Experiment {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	result := make([]*Experiment, 0, len(ab.experiments))
	for _, exp := range ab.experiments {
		result = append(result, &Experiment{
			ID:                  exp.ID,
			ExperimentGroupPct:  exp.ExperimentGroupPct,
			ControlGroupPct:     exp.ControlGroupPct,
			ExperimentGroupName: exp.ExperimentGroupName,
			ControlGroupName:    exp.ControlGroupName,
		})
	}
	return result
}

func (ab *ABTest) AssignGroup(userID, experimentID string) (string, error) {
	if userID == "" {
		return "", ErrEmptyUserID
	}
	if experimentID == "" {
		return "", ErrEmptyExperimentID
	}

	bucket, err := HashBucketWithExperiment(userID, experimentID)
	if err != nil {
		return "", err
	}

	ab.mu.RLock()
	defer ab.mu.RUnlock()

	exp, exists := ab.experiments[experimentID]
	if !exists {
		return "", ErrExperimentNotFound
	}

	if bucket < exp.ExperimentGroupPct {
		return exp.ExperimentGroupName, nil
	} else if bucket < exp.ExperimentGroupPct+exp.ControlGroupPct {
		return exp.ControlGroupName, nil
	}

	return GroupNoAssign, nil
}

func (ab *ABTest) AssignAllExperiments(userID string) (map[string]string, error) {
	if userID == "" {
		return nil, ErrEmptyUserID
	}

	ab.mu.RLock()
	defer ab.mu.RUnlock()

	result := make(map[string]string, len(ab.experiments))
	for expID, exp := range ab.experiments {
		bucket, err := HashBucketWithExperiment(userID, expID)
		if err != nil {
			return nil, err
		}
		if bucket < exp.ExperimentGroupPct {
			result[expID] = exp.ExperimentGroupName
		} else if bucket < exp.ExperimentGroupPct+exp.ControlGroupPct {
			result[expID] = exp.ControlGroupName
		} else {
			result[expID] = GroupNoAssign
		}
	}

	return result, nil
}

func (ab *ABTest) RecordMetric(experimentID, groupName, metricName string, value float64) error {
	if experimentID == "" {
		return ErrEmptyExperimentID
	}
	if groupName == "" {
		return ErrEmptyGroupName
	}
	if metricName == "" {
		return ErrEmptyMetricName
	}

	ab.mu.Lock()
	defer ab.mu.Unlock()

	expMetrics, exists := ab.metrics[experimentID]
	if !exists {
		return ErrExperimentNotFound
	}

	groupMetrics, exists := expMetrics.GroupMetrics[groupName]
	if !exists {
		return fmt.Errorf("abtest: group '%s' not found in experiment '%s'", groupName, experimentID)
	}

	groupMetrics.EventCount[metricName]++
	groupMetrics.MetricSum[metricName] += value

	return nil
}

func (ab *ABTest) GetExperimentMetrics(experimentID string) (*ExperimentMetrics, error) {
	if experimentID == "" {
		return nil, ErrEmptyExperimentID
	}

	ab.mu.RLock()
	defer ab.mu.RUnlock()

	expMetrics, exists := ab.metrics[experimentID]
	if !exists {
		return nil, ErrExperimentNotFound
	}

	result := &ExperimentMetrics{
		GroupMetrics: make(map[string]*GroupMetrics),
	}
	for groupName, gm := range expMetrics.GroupMetrics {
		result.GroupMetrics[groupName] = &GroupMetrics{
			EventCount: make(map[string]int64),
			MetricSum:  make(map[string]float64),
		}
		for k, v := range gm.EventCount {
			result.GroupMetrics[groupName].EventCount[k] = v
		}
		for k, v := range gm.MetricSum {
			result.GroupMetrics[groupName].MetricSum[k] = v
		}
	}

	return result, nil
}

func (ab *ABTest) GetGroupMetric(experimentID, groupName, metricName string) (int64, float64, error) {
	if experimentID == "" {
		return 0, 0, ErrEmptyExperimentID
	}
	if groupName == "" {
		return 0, 0, ErrEmptyGroupName
	}
	if metricName == "" {
		return 0, 0, ErrEmptyMetricName
	}

	ab.mu.RLock()
	defer ab.mu.RUnlock()

	expMetrics, exists := ab.metrics[experimentID]
	if !exists {
		return 0, 0, ErrExperimentNotFound
	}

	groupMetrics, exists := expMetrics.GroupMetrics[groupName]
	if !exists {
		return 0, 0, fmt.Errorf("abtest: group '%s' not found in experiment '%s'", groupName, experimentID)
	}

	return groupMetrics.EventCount[metricName], groupMetrics.MetricSum[metricName], nil
}

func (ab *ABTest) ResetExperimentMetrics(experimentID string) error {
	if experimentID == "" {
		return ErrEmptyExperimentID
	}

	ab.mu.Lock()
	defer ab.mu.Unlock()

	expMetrics, exists := ab.metrics[experimentID]
	if !exists {
		return ErrExperimentNotFound
	}

	for _, gm := range expMetrics.GroupMetrics {
		gm.EventCount = make(map[string]int64)
		gm.MetricSum = make(map[string]float64)
	}

	return nil
}
