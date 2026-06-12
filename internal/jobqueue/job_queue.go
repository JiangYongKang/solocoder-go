package jobqueue

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrQueueStopped    = errors.New("jobqueue: queue is stopped")
	ErrJobNotFound     = errors.New("jobqueue: job not found")
	ErrHandlerNotSet   = errors.New("jobqueue: handler function not set")
	ErrPoolSizeInvalid = errors.New("jobqueue: pool size must be greater than 0")
)

type JobHandler func(ctx context.Context, job *Job) (interface{}, error)

type Config struct {
	PoolSize        int
	DefaultMaxRetry int
	ShutdownTimeout time.Duration
}

type JobQueue struct {
	cfg              Config
	mu               sync.Mutex
	pq               *priorityQueue
	dq               *delayQueue
	deadLetters      []*Job
	jobs             map[string]*Job
	handler          JobHandler
	running          bool
	stopCh           chan struct{}
	shutdownCh       chan struct{}
	wakeCh           chan struct{}
	sem              chan struct{}
	activeCount      int64
	waitingDispatch  int64
	wg               sync.WaitGroup
	nextID           uint64
	idMu             sync.Mutex
	successResults   map[string]*JobResult
	deadLetterResults map[string]*JobResult
	resultsMu        sync.RWMutex
	notifyChan       map[string]chan struct{}
	notifyMu         sync.Mutex
}

func NewJobQueue(cfg Config) (*JobQueue, error) {
	if cfg.PoolSize <= 0 {
		return nil, ErrPoolSizeInvalid
	}
	if cfg.DefaultMaxRetry < 0 {
		cfg.DefaultMaxRetry = 0
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 30 * time.Second
	}

	pq := NewPriorityQueue()
	dq := NewDelayQueue()

	return &JobQueue{
		cfg:         cfg,
		pq:          pq,
		dq:          dq,
		deadLetters: make([]*Job, 0),
		jobs:        make(map[string]*Job),
		stopCh:      make(chan struct{}),
		shutdownCh:  make(chan struct{}),
		wakeCh:      make(chan struct{}),
		sem:         make(chan struct{}, cfg.PoolSize),
		successResults:   make(map[string]*JobResult),
		deadLetterResults: make(map[string]*JobResult),
		notifyChan:  make(map[string]chan struct{}),
	}, nil
}

func (jq *JobQueue) generateID() string {
	jq.idMu.Lock()
	defer jq.idMu.Unlock()
	jq.nextID++
	return fmt.Sprintf("job-%d-%d", time.Now().UnixNano(), jq.nextID)
}

func (jq *JobQueue) SetHandler(handler JobHandler) {
	jq.mu.Lock()
	defer jq.mu.Unlock()
	jq.handler = handler
}

func (jq *JobQueue) Start() error {
	jq.mu.Lock()
	if jq.handler == nil {
		jq.mu.Unlock()
		return ErrHandlerNotSet
	}
	if jq.running {
		jq.mu.Unlock()
		return nil
	}
	jq.running = true
	jq.stopCh = make(chan struct{})
	jq.shutdownCh = make(chan struct{})
	jq.wakeCh = make(chan struct{})
	jq.mu.Unlock()

	jq.wg.Add(1)
	go jq.dispatchLoop()

	return nil
}

func (jq *JobQueue) Stop() {
	jq.mu.Lock()
	if !jq.running {
		jq.mu.Unlock()
		return
	}
	jq.running = false
	close(jq.stopCh)
	jq.wake()
	jq.mu.Unlock()

	done := make(chan struct{})
	go func() {
		jq.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(jq.cfg.ShutdownTimeout):
	}

	close(jq.shutdownCh)
}

func (jq *JobQueue) wake() {
	select {
	case <-jq.wakeCh:
	default:
	}
	close(jq.wakeCh)
	jq.wakeCh = make(chan struct{})
}

func (jq *JobQueue) Enqueue(id string, priority int, payload interface{}, delay time.Duration) (string, error) {
	return jq.enqueueInternal(id, priority, payload, delay, jq.cfg.DefaultMaxRetry)
}

func (jq *JobQueue) EnqueueWithRetry(id string, priority int, payload interface{}, delay time.Duration, maxRetries int) (string, error) {
	if maxRetries < 0 {
		maxRetries = 0
	}
	return jq.enqueueInternal(id, priority, payload, delay, maxRetries)
}

func (jq *JobQueue) enqueueInternal(id string, priority int, payload interface{}, delay time.Duration, maxRetries int) (string, error) {
	jq.mu.Lock()
	if !jq.running {
		jq.mu.Unlock()
		return "", ErrQueueStopped
	}
	jq.mu.Unlock()

	if id == "" {
		id = jq.generateID()
	}

	job := NewJob(id, priority, payload, maxRetries, delay)

	jq.mu.Lock()
	if !jq.running {
		jq.mu.Unlock()
		return "", ErrQueueStopped
	}
	if _, exists := jq.jobs[id]; exists {
		jq.mu.Unlock()
		return id, nil
	}
	jq.jobs[id] = job

	if delay > 0 {
		heap.Push(jq.dq, job)
	} else {
		heap.Push(jq.pq, job)
	}
	jq.wake()
	jq.mu.Unlock()

	return id, nil
}

func (jq *JobQueue) dispatchLoop() {
	defer jq.wg.Done()

	for {
		jq.mu.Lock()

		for jq.dq.Len() > 0 {
			job := jq.dq.Peek()
			if job.IsReady() {
				heap.Pop(jq.dq)
				heap.Push(jq.pq, job)
			} else {
				break
			}
		}

		if !jq.running && jq.pq.Len() == 0 && jq.dq.Len() == 0 && atomic.LoadInt64(&jq.waitingDispatch) == 0 {
			jq.mu.Unlock()
			return
		}

		if jq.pq.Len() == 0 {
			var timer *time.Timer
			if jq.dq.Len() > 0 {
				job := jq.dq.Peek()
				waitTime := time.Until(job.ReadyTime)
				if waitTime > 0 {
					timer = time.NewTimer(waitTime)
				}
			}
			wakeCh := jq.wakeCh
			stopCh := jq.stopCh
			isStopping := !jq.running
			jq.mu.Unlock()

			if timer != nil {
				select {
				case <-stopCh:
					timer.Stop()
					continue
				case <-wakeCh:
					timer.Stop()
					continue
				case <-timer.C:
					continue
				}
			} else if isStopping {
				time.Sleep(10 * time.Millisecond)
				continue
			} else {
				select {
				case <-stopCh:
					continue
				case <-wakeCh:
					continue
				}
			}
		}

		if jq.pq.Len() == 0 {
			jq.mu.Unlock()
			continue
		}

		job := heap.Pop(jq.pq).(*Job)
		if job.Status == JobStatusDeadLetter || job.Status == JobStatusCompleted {
			delete(jq.jobs, job.ID)
			jq.mu.Unlock()
			continue
		}

		atomic.AddInt64(&jq.waitingDispatch, 1)
		sem := jq.sem
		wakeCh := jq.wakeCh
		stopCh := jq.stopCh
		isStopping := !jq.running
		jq.mu.Unlock()

		var acquired bool
		if isStopping {
			sem <- struct{}{}
			acquired = true
		} else {
			select {
			case sem <- struct{}{}:
				acquired = true
			case <-stopCh:
			case <-wakeCh:
			}
		}

		atomic.AddInt64(&jq.waitingDispatch, -1)

		if !acquired {
			jq.mu.Lock()
			heap.Push(jq.pq, job)
			jq.wake()
			jq.mu.Unlock()
			continue
		}

		atomic.AddInt64(&jq.activeCount, 1)
		jq.wg.Add(1)
		go func(j *Job) {
			defer jq.wg.Done()
			defer atomic.AddInt64(&jq.activeCount, -1)
			defer func() { <-jq.sem }()
			jq.executeJob(j)
		}(job)
	}
}

func (jq *JobQueue) executeJob(job *Job) {
	jq.mu.Lock()
	job.Status = JobStatusRunning
	jq.mu.Unlock()

	result, err := jq.safeExecute(job)

	jq.mu.Lock()
	defer jq.mu.Unlock()

	if err == nil {
		job.Status = JobStatusCompleted
		job.Result = result
		job.Error = nil
		jq.storeSuccessResult(job.ID, result)
		jq.notifyJobComplete(job.ID)
		delete(jq.jobs, job.ID)
		return
	}

	job.RetryCount++
	job.Error = err

	if job.RetryCount > job.MaxRetries {
		job.Status = JobStatusDeadLetter
		jq.deadLetters = append(jq.deadLetters, job)
		jq.storeDeadLetterResult(job.ID, err)
		jq.notifyJobComplete(job.ID)
		delete(jq.jobs, job.ID)
		return
	}

	backoff := job.BackoffDelay()
	job.ReadyTime = time.Now().Add(backoff)
	job.EnqueueTime = time.Now()
	job.Status = JobStatusFailed
	heap.Push(jq.dq, job)
	jq.wake()
}

func (jq *JobQueue) safeExecute(job *Job) (result interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("job panicked: %v", r)
			result = nil
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return jq.handler(ctx, job)
}

func (jq *JobQueue) storeSuccessResult(jobID string, result interface{}) {
	jq.resultsMu.Lock()
	defer jq.resultsMu.Unlock()
	jq.successResults[jobID] = &JobResult{
		JobID:  jobID,
		Result: result,
		Error:  nil,
	}
}

func (jq *JobQueue) storeDeadLetterResult(jobID string, err error) {
	jq.resultsMu.Lock()
	defer jq.resultsMu.Unlock()
	jq.deadLetterResults[jobID] = &JobResult{
		JobID:  jobID,
		Result: nil,
		Error:  err,
	}
}

func (jq *JobQueue) lookupResult(jobID string) (*JobResult, bool) {
	if res, ok := jq.successResults[jobID]; ok {
		return res, true
	}
	if res, ok := jq.deadLetterResults[jobID]; ok {
		return res, true
	}
	return nil, false
}

func (jq *JobQueue) notifyJobComplete(jobID string) {
	jq.notifyMu.Lock()
	defer jq.notifyMu.Unlock()
	if ch, ok := jq.notifyChan[jobID]; ok {
		close(ch)
		delete(jq.notifyChan, jobID)
	}
}

func (jq *JobQueue) WaitForResult(ctx context.Context, jobID string) (*JobResult, error) {
	jq.resultsMu.RLock()
	if res, ok := jq.lookupResult(jobID); ok {
		jq.resultsMu.RUnlock()
		return res, nil
	}
	jq.resultsMu.RUnlock()

	jq.notifyMu.Lock()
	ch, ok := jq.notifyChan[jobID]
	if !ok {
		ch = make(chan struct{})
		jq.notifyChan[jobID] = ch
	}
	jq.notifyMu.Unlock()

	if ok {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ch:
		}
	} else {
		jq.resultsMu.RLock()
		if res, ok := jq.lookupResult(jobID); ok {
			jq.resultsMu.RUnlock()
			return res, nil
		}
		jq.resultsMu.RUnlock()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ch:
		}
	}

	jq.resultsMu.RLock()
	defer jq.resultsMu.RUnlock()
	if res, ok := jq.lookupResult(jobID); ok {
		return res, nil
	}
	return nil, ErrJobNotFound
}

func (jq *JobQueue) GetResult(jobID string) (*JobResult, error) {
	jq.resultsMu.RLock()
	defer jq.resultsMu.RUnlock()
	if res, ok := jq.lookupResult(jobID); ok {
		return res, nil
	}
	return nil, ErrJobNotFound
}

func (jq *JobQueue) GetJobStatus(jobID string) (JobStatus, error) {
	jq.mu.Lock()
	defer jq.mu.Unlock()
	if job, ok := jq.jobs[jobID]; ok {
		return job.Status, nil
	}
	for _, j := range jq.deadLetters {
		if j.ID == jobID {
			return j.Status, nil
		}
	}
	jq.resultsMu.RLock()
	defer jq.resultsMu.RUnlock()
	if _, ok := jq.successResults[jobID]; ok {
		return JobStatusCompleted, nil
	}
	return "", ErrJobNotFound
}

func (jq *JobQueue) PendingCount() int {
	jq.mu.Lock()
	defer jq.mu.Unlock()
	return jq.pq.Len() + jq.dq.Len() + int(atomic.LoadInt64(&jq.waitingDispatch))
}

func (jq *JobQueue) ActiveCount() int {
	return int(atomic.LoadInt64(&jq.activeCount))
}

func (jq *JobQueue) DeadLetterCount() int {
	jq.mu.Lock()
	defer jq.mu.Unlock()
	return len(jq.deadLetters)
}

func (jq *JobQueue) GetDeadLetters() []*Job {
	jq.mu.Lock()
	defer jq.mu.Unlock()
	result := make([]*Job, len(jq.deadLetters))
	copy(result, jq.deadLetters)
	return result
}

func (jq *JobQueue) ClearDeadLetters() {
	jq.mu.Lock()
	defer jq.mu.Unlock()
	jq.deadLetters = jq.deadLetters[:0]
}

func (jq *JobQueue) CompletedCount() int {
	jq.resultsMu.RLock()
	defer jq.resultsMu.RUnlock()
	return len(jq.successResults)
}

func (jq *JobQueue) FailedCount() int {
	jq.resultsMu.RLock()
	defer jq.resultsMu.RUnlock()
	return len(jq.deadLetterResults)
}

func (jq *JobQueue) ClearResults() {
	jq.resultsMu.Lock()
	defer jq.resultsMu.Unlock()
	jq.successResults = make(map[string]*JobResult)
	jq.deadLetterResults = make(map[string]*JobResult)
}
