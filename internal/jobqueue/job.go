package jobqueue

import (
	"time"
)

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusDeadLetter JobStatus = "dead_letter"
)

type Job struct {
	ID          string
	Priority    int
	Payload     interface{}
	Delay       time.Duration
	EnqueueTime time.Time
	ReadyTime   time.Time
	RetryCount  int
	MaxRetries  int
	Status      JobStatus
	Result      interface{}
	Error       error
}

type JobResult struct {
	JobID  string
	Result interface{}
	Error  error
}

func NewJob(id string, priority int, payload interface{}, maxRetries int, delay time.Duration) *Job {
	now := time.Now()
	return &Job{
		ID:         id,
		Priority:   priority,
		Payload:    payload,
		Delay:      delay,
		EnqueueTime: now,
		ReadyTime:  now.Add(delay),
		RetryCount: 0,
		MaxRetries: maxRetries,
		Status:     JobStatusPending,
	}
}

func (j *Job) IsReady() bool {
	return time.Now().After(j.ReadyTime) || time.Now().Equal(j.ReadyTime)
}

func (j *Job) BackoffDelay() time.Duration {
	base := 100 * time.Millisecond
	return base * time.Duration(1<<uint(j.RetryCount))
}
