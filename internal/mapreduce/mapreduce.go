package mapreduce

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"sync"
	"sync/atomic"
)

var (
	ErrNoInputData      = errors.New("mapreduce: no input data provided")
	ErrNoMapFunc        = errors.New("mapreduce: map function not set")
	ErrNoReduceFunc     = errors.New("mapreduce: reduce function not set")
	ErrInvalidReduceNum = errors.New("mapreduce: number of reduce tasks must be greater than 0")
	ErrInvalidRetry     = errors.New("mapreduce: max retries must be >= 0")
	ErrAlreadyRunning   = errors.New("mapreduce: job already running")
	ErrJobNotRunning    = errors.New("mapreduce: job not running")
	ErrTaskFailed       = errors.New("mapreduce: task failed after max retries")
)

type KeyValue struct {
	Key   string
	Value interface{}
}

type MapFunc func(ctx context.Context, key string, value interface{}) ([]KeyValue, error)

type ReduceFunc func(ctx context.Context, key string, values []interface{}) (interface{}, error)

type PartitionFunc func(key string, numPartitions int) int

type MergeFunc func(results []interface{}) (interface{}, error)

func HashPartition(key string, numPartitions int) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32()) % numPartitions
}

type ShuffleMode int

const (
	ShuffleSync  ShuffleMode = iota
	ShuffleAsync
)

type TaskStatus int

const (
	TaskStatusPending TaskStatus = iota
	TaskStatusRunning
	TaskStatusCompleted
	TaskStatusFailed
)

type TaskError struct {
	TaskType string
	TaskID   int
	Attempt  int
	Err      error
}

func (te *TaskError) Error() string {
	return fmt.Sprintf("mapreduce: %s task %d failed on attempt %d: %v", te.TaskType, te.TaskID, te.Attempt, te.Err)
}

func (te *TaskError) Unwrap() error {
	return te.Err
}

type Config struct {
	MapFunc        MapFunc
	ReduceFunc     ReduceFunc
	NumReduce      int
	MaxRetries     int
	ShuffleMode    ShuffleMode
	PartitionFunc  PartitionFunc
	MergeFunc      MergeFunc
}

type mapTaskResult struct {
	taskID int
	kvs    []KeyValue
	err    error
}

type reduceTaskResult struct {
	taskID int
	result interface{}
	err    error
}

type MapReduce struct {
	completedMapCount    int64
	completedReduceCount int64

	cfg Config

	input []KeyValue

	status    TaskStatus
	statusMu  sync.RWMutex

	mapResults    []map[int][]KeyValue
	mapResultsMu  []sync.Mutex

	reduceResults []interface{}
	reduceErrs    []error

	errCh chan *TaskError

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	result     interface{}
	resultErr  error
	resultOnce sync.Once

	onComplete chan struct{}
}

func NewMapReduce(cfg Config) (*MapReduce, error) {
	if cfg.MapFunc == nil {
		return nil, ErrNoMapFunc
	}
	if cfg.ReduceFunc == nil {
		return nil, ErrNoReduceFunc
	}
	if cfg.NumReduce <= 0 {
		return nil, ErrInvalidReduceNum
	}
	if cfg.MaxRetries < 0 {
		return nil, ErrInvalidRetry
	}
	if cfg.PartitionFunc == nil {
		cfg.PartitionFunc = HashPartition
	}

	mr := &MapReduce{
		cfg:           cfg,
		mapResults:    make([]map[int][]KeyValue, cfg.NumReduce),
		mapResultsMu:  make([]sync.Mutex, cfg.NumReduce),
		reduceResults: make([]interface{}, cfg.NumReduce),
		reduceErrs:    make([]error, cfg.NumReduce),
		errCh:         make(chan *TaskError, cfg.NumReduce+cfg.NumReduce),
		onComplete:    make(chan struct{}),
	}

	for i := 0; i < cfg.NumReduce; i++ {
		mr.mapResults[i] = make(map[int][]KeyValue)
	}

	return mr, nil
}

func (mr *MapReduce) SetInput(input []KeyValue) {
	mr.statusMu.RLock()
	if mr.status == TaskStatusRunning {
		mr.statusMu.RUnlock()
		return
	}
	mr.statusMu.RUnlock()
	mr.input = input
}

func (mr *MapReduce) Run(ctx context.Context) (interface{}, error) {
	mr.statusMu.Lock()
	if mr.status == TaskStatusRunning {
		mr.statusMu.Unlock()
		return nil, ErrAlreadyRunning
	}
	if len(mr.input) == 0 {
		mr.statusMu.Unlock()
		return nil, ErrNoInputData
	}
	if mr.status == TaskStatusCompleted || mr.status == TaskStatusFailed {
		mr.resetState()
	}
	mr.status = TaskStatusRunning
	mr.statusMu.Unlock()

	mr.ctx, mr.cancel = context.WithCancel(ctx)
	defer mr.cancel()

	var (
		result interface{}
		runErr error
	)

	defer func() {
		mr.statusMu.Lock()
		if mr.status == TaskStatusRunning {
			if runErr != nil {
				mr.status = TaskStatusFailed
			} else {
				mr.status = TaskStatusCompleted
			}
		}
		mr.statusMu.Unlock()
		close(mr.onComplete)
	}()

	mapResultsCh := make(chan mapTaskResult, len(mr.input))
	if err := mr.runMapTasks(mapResultsCh); err != nil {
		runErr = err
		return nil, err
	}

	switch mr.cfg.ShuffleMode {
	case ShuffleSync:
		if err := mr.shuffleSync(mapResultsCh); err != nil {
			runErr = err
			return nil, err
		}
		if err := mr.runReduceTasks(); err != nil {
			runErr = err
			return nil, err
		}
	case ShuffleAsync:
		if err := mr.shuffleAsync(mapResultsCh); err != nil {
			runErr = err
			return nil, err
		}
	}

	result, runErr = mr.mergeResults()
	return result, runErr
}

func (mr *MapReduce) resetState() {
	mr.mapResults = make([]map[int][]KeyValue, mr.cfg.NumReduce)
	for i := 0; i < mr.cfg.NumReduce; i++ {
		mr.mapResults[i] = make(map[int][]KeyValue)
	}
	mr.reduceResults = make([]interface{}, mr.cfg.NumReduce)
	mr.reduceErrs = make([]error, mr.cfg.NumReduce)
	atomic.StoreInt64(&mr.completedMapCount, 0)
	atomic.StoreInt64(&mr.completedReduceCount, 0)
	mr.result = nil
	mr.resultErr = nil
	mr.resultOnce = sync.Once{}
	mr.onComplete = make(chan struct{})
	mr.errCh = make(chan *TaskError, mr.cfg.NumReduce+mr.cfg.NumReduce)
}

func (mr *MapReduce) runMapTasks(resultsCh chan<- mapTaskResult) error {
	for i, kv := range mr.input {
		taskID := i
		inputKV := kv
		mr.wg.Add(1)
		go func() {
			defer mr.wg.Done()
			mr.executeMapWithRetry(taskID, inputKV, resultsCh)
		}()
	}
	return nil
}

func (mr *MapReduce) executeMapWithRetry(taskID int, input KeyValue, resultsCh chan<- mapTaskResult) {
	var result mapTaskResult
	for attempt := 0; attempt <= mr.cfg.MaxRetries; attempt++ {
		select {
		case <-mr.ctx.Done():
			result = mapTaskResult{taskID: taskID, err: mr.ctx.Err()}
			resultsCh <- result
			return
		default:
		}

		kvs, err := mr.safeExecuteMap(input)
		if err == nil {
			result = mapTaskResult{taskID: taskID, kvs: kvs}
			resultsCh <- result
			return
		}

		if attempt < mr.cfg.MaxRetries {
			continue
		}

		mr.errCh <- &TaskError{
			TaskType: "map",
			TaskID:   taskID,
			Attempt:  attempt + 1,
			Err:      err,
		}
		result = mapTaskResult{taskID: taskID, err: fmt.Errorf("map task %d failed after %d attempts: %w", taskID, attempt+1, err)}
		resultsCh <- result
		return
	}
}

func (mr *MapReduce) safeExecuteMap(input KeyValue) (kvs []KeyValue, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("map task panicked: %v", r)
			kvs = nil
		}
	}()
	return mr.cfg.MapFunc(mr.ctx, input.Key, input.Value)
}

func (mr *MapReduce) safeExecuteReduce(key string, values []interface{}) (result interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("reduce task panicked: %v", r)
			result = nil
		}
	}()
	return mr.cfg.ReduceFunc(mr.ctx, key, values)
}

func (mr *MapReduce) shuffleSync(mapResultsCh <-chan mapTaskResult) error {
	totalMapTasks := len(mr.input)
	received := 0
	failedCount := 0

	for received < totalMapTasks {
		select {
		case <-mr.ctx.Done():
			return mr.ctx.Err()
		case result := <-mapResultsCh:
			received++
			if result.err != nil {
				failedCount++
				continue
			}
			mr.partitionIntermediate(result.kvs, result.taskID)
		}
	}

	if failedCount > 0 {
		return fmt.Errorf("mapreduce: %d map task(s) failed", failedCount)
	}

	atomic.StoreInt64(&mr.completedMapCount, int64(totalMapTasks))
	return nil
}

func (mr *MapReduce) shuffleAsync(mapResultsCh <-chan mapTaskResult) error {
	totalMapTasks := len(mr.input)
	var received int64
	var mapFailed int64

	mapReportedChs := make([]chan mapReport, mr.cfg.NumReduce)
	for i := 0; i < mr.cfg.NumReduce; i++ {
		mapReportedChs[i] = make(chan mapReport, totalMapTasks)
	}

	reduceErrCh := make(chan error, mr.cfg.NumReduce)

	var reduceWg sync.WaitGroup
	for r := 0; r < mr.cfg.NumReduce; r++ {
		reduceID := r
		reduceWg.Add(1)
		mr.wg.Add(1)
		go func() {
			defer reduceWg.Done()
			defer mr.wg.Done()

			reportedCount := 0
			localGrouped := make(map[string][]interface{})

			for reportedCount < totalMapTasks {
				select {
				case <-mr.ctx.Done():
					reduceErrCh <- mr.ctx.Err()
					return

				case report := <-mapReportedChs[reduceID]:
					reportedCount++
					if report.err == nil {
						mr.mapResultsMu[reduceID].Lock()
						if kvs, ok := mr.mapResults[reduceID][report.mapID]; ok {
							for _, kv := range kvs {
								localGrouped[kv.Key] = append(localGrouped[kv.Key], kv.Value)
							}
						}
						mr.mapResultsMu[reduceID].Unlock()
					}
				}
			}

			if atomic.LoadInt64(&mapFailed) > 0 {
				return
			}

			mr.finalizeAndRunReduce(reduceID, localGrouped, nil, reduceErrCh)
		}()
	}

	var consumerErr error
	consumerDone := make(chan struct{})

	go func() {
		defer close(consumerDone)
		for atomic.LoadInt64(&received) < int64(totalMapTasks) {
			select {
			case <-mr.ctx.Done():
				consumerErr = mr.ctx.Err()
				return
			case result := <-mapResultsCh:
				atomic.AddInt64(&received, 1)
				if result.err != nil {
					atomic.AddInt64(&mapFailed, 1)
					for r := 0; r < mr.cfg.NumReduce; r++ {
						select {
						case mapReportedChs[r] <- mapReport{mapID: result.taskID, err: result.err}:
						case <-mr.ctx.Done():
							return
						}
					}
					continue
				}

				mr.partitionIntermediate(result.kvs, result.taskID)
				atomic.AddInt64(&mr.completedMapCount, 1)

				affectedPartitions := make(map[int]bool)
				for _, kv := range result.kvs {
					p := mr.cfg.PartitionFunc(kv.Key, mr.cfg.NumReduce)
					if p < 0 {
						p = 0
					}
					if p >= mr.cfg.NumReduce {
						p = mr.cfg.NumReduce - 1
					}
					affectedPartitions[p] = true
				}

				for r := 0; r < mr.cfg.NumReduce; r++ {
					report := mapReport{mapID: result.taskID}
					if affectedPartitions[r] {
						report.hasData = true
					}
					select {
					case mapReportedChs[r] <- report:
					case <-mr.ctx.Done():
						return
					}
				}
			}
		}
	}()

	<-consumerDone

	reduceWg.Wait()
	for r := 0; r < mr.cfg.NumReduce; r++ {
		close(mapReportedChs[r])
	}
	close(reduceErrCh)

	if consumerErr != nil {
		return consumerErr
	}

	if atomic.LoadInt64(&mapFailed) > 0 {
		return fmt.Errorf("mapreduce: %d map task(s) failed", atomic.LoadInt64(&mapFailed))
	}

	var reduceErrs []error
	for err := range reduceErrCh {
		if err != nil {
			reduceErrs = append(reduceErrs, err)
		}
	}
	if len(reduceErrs) > 0 {
		return fmt.Errorf("mapreduce: %d reduce task(s) failed: %v", len(reduceErrs), reduceErrs[0])
	}

	return nil
}

type mapReport struct {
	mapID   int
	hasData bool
	err     error
}

func (mr *MapReduce) finalizeAndRunReduce(
	reduceID int,
	localGrouped map[string][]interface{},
	_ map[int]bool,
	errCh chan<- error,
) {
	if len(localGrouped) == 0 {
		mr.reduceResults[reduceID] = nil
		atomic.AddInt64(&mr.completedReduceCount, 1)
		return
	}

	keys := make([]string, 0, len(localGrouped))
	for key := range localGrouped {
		keys = append(keys, key)
	}

	for attempt := 0; attempt <= mr.cfg.MaxRetries; attempt++ {
		select {
		case <-mr.ctx.Done():
			mr.reduceErrs[reduceID] = mr.ctx.Err()
			errCh <- mr.ctx.Err()
			return
		default:
		}

		var partitionResults []KeyValue
		var allErr error
		for _, key := range keys {
			values := localGrouped[key]
			result, err := mr.safeExecuteReduce(key, values)
			if err != nil {
				allErr = err
				break
			}
			partitionResults = append(partitionResults, KeyValue{Key: key, Value: result})
		}

		if allErr == nil {
			mr.reduceResults[reduceID] = partitionResults
			atomic.AddInt64(&mr.completedReduceCount, 1)
			return
		}

		if attempt < mr.cfg.MaxRetries {
			partitionResults = nil
			continue
		}

		mr.errCh <- &TaskError{
			TaskType: "reduce",
			TaskID:   reduceID,
			Attempt:  attempt + 1,
			Err:      allErr,
		}
		mr.reduceErrs[reduceID] = fmt.Errorf("reduce task %d failed after %d attempts: %w", reduceID, attempt+1, allErr)
		errCh <- mr.reduceErrs[reduceID]
		return
	}
}

func (mr *MapReduce) partitionIntermediate(kvs []KeyValue, mapTaskID int) {
	for _, kv := range kvs {
		partition := mr.cfg.PartitionFunc(kv.Key, mr.cfg.NumReduce)
		if partition < 0 {
			partition = 0
		}
		if partition >= mr.cfg.NumReduce {
			partition = mr.cfg.NumReduce - 1
		}

		mr.mapResultsMu[partition].Lock()
		mr.mapResults[partition][mapTaskID] = append(mr.mapResults[partition][mapTaskID], kv)
		mr.mapResultsMu[partition].Unlock()
	}
}

func (mr *MapReduce) getGroupedIntermediate(partition int) map[string][]interface{} {
	grouped := make(map[string][]interface{})
	mr.mapResultsMu[partition].Lock()
	defer mr.mapResultsMu[partition].Unlock()

	for _, kvs := range mr.mapResults[partition] {
		for _, kv := range kvs {
			grouped[kv.Key] = append(grouped[kv.Key], kv.Value)
		}
	}
	return grouped
}

func (mr *MapReduce) runReduceTasks() error {
	var wg sync.WaitGroup
	errCh := make(chan error, mr.cfg.NumReduce)

	for r := 0; r < mr.cfg.NumReduce; r++ {
		reduceID := r
		wg.Add(1)
		go func() {
			defer wg.Done()
			mr.executeReduceWithRetry(reduceID, errCh)
		}()
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("mapreduce: %d reduce task(s) failed: %v", len(errs), errs[0])
	}

	return nil
}

func (mr *MapReduce) executeReduceWithRetry(reduceID int, errCh chan<- error) {
	grouped := mr.getGroupedIntermediate(reduceID)

	if len(grouped) == 0 {
		mr.reduceResults[reduceID] = nil
		atomic.AddInt64(&mr.completedReduceCount, 1)
		return
	}

	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}

	for attempt := 0; attempt <= mr.cfg.MaxRetries; attempt++ {
		select {
		case <-mr.ctx.Done():
			mr.reduceErrs[reduceID] = mr.ctx.Err()
			errCh <- mr.ctx.Err()
			return
		default:
		}

		var partitionResults []KeyValue
		var allErr error
		for _, key := range keys {
			values := grouped[key]
			result, err := mr.safeExecuteReduce(key, values)
			if err != nil {
				allErr = err
				break
			}
			partitionResults = append(partitionResults, KeyValue{Key: key, Value: result})
		}

		if allErr == nil {
			mr.reduceResults[reduceID] = partitionResults
			atomic.AddInt64(&mr.completedReduceCount, 1)
			return
		}

		if attempt < mr.cfg.MaxRetries {
			partitionResults = nil
			continue
		}

		mr.errCh <- &TaskError{
			TaskType: "reduce",
			TaskID:   reduceID,
			Attempt:  attempt + 1,
			Err:      allErr,
		}
		mr.reduceErrs[reduceID] = fmt.Errorf("reduce task %d failed after %d attempts: %w", reduceID, attempt+1, allErr)
		errCh <- mr.reduceErrs[reduceID]
		return
	}
}

func (mr *MapReduce) mergeResults() (interface{}, error) {
	for i, err := range mr.reduceErrs {
		if err != nil {
			return nil, fmt.Errorf("mapreduce: reduce task %d failed: %w", i, err)
		}
	}

	var results []interface{}
	for _, r := range mr.reduceResults {
		results = append(results, r)
	}

	if mr.cfg.MergeFunc != nil {
		merged, err := mr.cfg.MergeFunc(results)
		if err != nil {
			return nil, fmt.Errorf("mapreduce: merge failed: %w", err)
		}
		mr.resultOnce.Do(func() {
			mr.result = merged
		})
		return merged, nil
	}

	mr.resultOnce.Do(func() {
		mr.result = results
	})
	return results, nil
}

func (mr *MapReduce) Status() TaskStatus {
	mr.statusMu.RLock()
	defer mr.statusMu.RUnlock()
	return mr.status
}

func (mr *MapReduce) Result() (interface{}, error) {
	return mr.result, mr.resultErr
}

func (mr *MapReduce) CompletedMapCount() int64 {
	return atomic.LoadInt64(&mr.completedMapCount)
}

func (mr *MapReduce) CompletedReduceCount() int64 {
	return atomic.LoadInt64(&mr.completedReduceCount)
}

func (mr *MapReduce) Errors() []*TaskError {
	var errs []*TaskError
	for {
		select {
		case te := <-mr.errCh:
			errs = append(errs, te)
		default:
			return errs
		}
	}
}

func (mr *MapReduce) Cancel() {
	mr.statusMu.Lock()
	defer mr.statusMu.Unlock()
	if mr.status == TaskStatusRunning {
		mr.status = TaskStatusFailed
		if mr.cancel != nil {
			mr.cancel()
		}
	}
}

func (mr *MapReduce) Wait(ctx context.Context) (interface{}, error) {
	done := make(chan struct{})
	var result interface{}
	var err error

	go func() {
		mr.wg.Wait()
		result, err = mr.Result()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done:
		return result, err
	}
}

func (mr *MapReduce) Done() <-chan struct{} {
	return mr.onComplete
}
