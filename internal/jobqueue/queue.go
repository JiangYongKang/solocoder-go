package jobqueue

import "container/heap"

type priorityQueue []*Job

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	if pq[i].Priority != pq[j].Priority {
		return pq[i].Priority > pq[j].Priority
	}
	return pq[i].EnqueueTime.Before(pq[j].EnqueueTime)
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *priorityQueue) Push(x interface{}) {
	job := x.(*Job)
	*pq = append(*pq, job)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	job := old[n-1]
	old[n-1] = nil
	*pq = old[0 : n-1]
	return job
}

type delayQueue []*Job

func (dq delayQueue) Len() int { return len(dq) }

func (dq delayQueue) Less(i, j int) bool {
	return dq[i].ReadyTime.Before(dq[j].ReadyTime)
}

func (dq delayQueue) Swap(i, j int) {
	dq[i], dq[j] = dq[j], dq[i]
}

func (dq *delayQueue) Push(x interface{}) {
	job := x.(*Job)
	*dq = append(*dq, job)
}

func (dq *delayQueue) Pop() interface{} {
	old := *dq
	n := len(old)
	job := old[n-1]
	old[n-1] = nil
	*dq = old[0 : n-1]
	return job
}

func (dq *delayQueue) Peek() *Job {
	if len(*dq) == 0 {
		return nil
	}
	return (*dq)[0]
}

func NewPriorityQueue() *priorityQueue {
	pq := &priorityQueue{}
	heap.Init(pq)
	return pq
}

func NewDelayQueue() *delayQueue {
	dq := &delayQueue{}
	heap.Init(dq)
	return dq
}
