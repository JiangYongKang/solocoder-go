package webhook

import (
	"bytes"
	"container/heap"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"
)

const (
	defaultTimeout      = 10 * time.Second
	defaultWorkerCount  = 4
	shutdownTimeout     = 30 * time.Second
)

type pendingDelivery struct {
	callback    *Callback
	attempt     int
	scheduledAt time.Time
	data        interface{}
}

type deliveryHeap []*pendingDelivery

func (h deliveryHeap) Len() int { return len(h) }

func (h deliveryHeap) Less(i, j int) bool {
	return h[i].scheduledAt.Before(h[j].scheduledAt)
}

func (h deliveryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *deliveryHeap) Push(x interface{}) {
	*h = append(*h, x.(*pendingDelivery))
}

func (h *deliveryHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[0 : n-1]
	return item
}

func (h *deliveryHeap) Peek() *pendingDelivery {
	if len(*h) == 0 {
		return nil
	}
	return (*h)[0]
}

type Scheduler struct {
	activeCount     int64
	nextID          uint64
	mu              sync.Mutex
	callbacks       map[string]*Callback
	deliveries      map[string][]*Delivery
	deliveryResults map[string]*DeliveryResult
	pending         *deliveryHeap
	running         bool
	stopCh          chan struct{}
	wakeCh          chan struct{}
	wg              sync.WaitGroup
	sem             chan struct{}
	idMu            sync.Mutex
	httpClient      HTTPClient
	notifyChan      map[string]chan struct{}
	notifyMu        sync.Mutex
	workerCount     int
}

type SchedulerConfig struct {
	WorkerCount int
	HTTPClient  HTTPClient
}

func NewScheduler(cfg SchedulerConfig) *Scheduler {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = defaultWorkerCount
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: defaultTimeout}
	}

	pending := make(deliveryHeap, 0)

	return &Scheduler{
		callbacks:       make(map[string]*Callback),
		deliveries:      make(map[string][]*Delivery),
		deliveryResults: make(map[string]*DeliveryResult),
		pending:         &pending,
		sem:             make(chan struct{}, cfg.WorkerCount),
		httpClient:      cfg.HTTPClient,
		workerCount:     cfg.WorkerCount,
		notifyChan:      make(map[string]chan struct{}),
	}
}

func (s *Scheduler) generateID(prefix string) string {
	s.idMu.Lock()
	defer s.idMu.Unlock()
	s.nextID++
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), s.nextID)
}

func (s *Scheduler) wake() {
	select {
	case <-s.wakeCh:
	default:
	}
	close(s.wakeCh)
	s.wakeCh = make(chan struct{})
}

func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.wakeCh = make(chan struct{})
	pending := make(deliveryHeap, 0)
	s.pending = &pending
	s.mu.Unlock()

	s.wg.Add(1)
	go s.dispatchLoop()
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.wake()
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(shutdownTimeout):
	}
}

func validateMethod(method string) bool {
	m := strings.ToUpper(method)
	validMethods := map[string]bool{
		http.MethodGet:     true,
		http.MethodPost:    true,
		http.MethodPut:     true,
		http.MethodDelete:  true,
		http.MethodPatch:   true,
		http.MethodHead:    true,
		http.MethodOptions: true,
	}
	return validMethods[m]
}

func validateURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if u.Host == "" {
		return false
	}
	return true
}

func (s *Scheduler) Register(rawURL string, method string, opts ...CallbackOption) (string, error) {
	if !validateURL(rawURL) {
		return "", ErrInvalidURL
	}
	if !validateMethod(method) {
		return "", ErrInvalidMethod
	}

	cb := &Callback{
		ID:          s.generateID("cb"),
		URL:         rawURL,
		Method:      strings.ToUpper(method),
		Headers:     make(map[string]string),
		RetryPolicy: DefaultRetryPolicy(),
		Timeout:     defaultTimeout,
		Status:      CallbackStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	for _, opt := range opts {
		opt(cb)
	}

	if err := cb.RetryPolicy.Validate(); err != nil {
		return "", err
	}
	if cb.Timeout <= 0 {
		return "", ErrInvalidTimeout
	}

	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return "", ErrSchedulerStopped
	}
	if _, exists := s.callbacks[cb.ID]; exists {
		s.mu.Unlock()
		return "", ErrCallbackAlreadyExists
	}
	s.callbacks[cb.ID] = cb
	s.mu.Unlock()

	return cb.ID, nil
}

func (s *Scheduler) RegisterWithID(id string, rawURL string, method string, opts ...CallbackOption) error {
	if !validateURL(rawURL) {
		return ErrInvalidURL
	}
	if !validateMethod(method) {
		return ErrInvalidMethod
	}

	cb := &Callback{
		ID:          id,
		URL:         rawURL,
		Method:      strings.ToUpper(method),
		Headers:     make(map[string]string),
		RetryPolicy: DefaultRetryPolicy(),
		Timeout:     defaultTimeout,
		Status:      CallbackStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	for _, opt := range opts {
		opt(cb)
	}

	if err := cb.RetryPolicy.Validate(); err != nil {
		return err
	}
	if cb.Timeout <= 0 {
		return ErrInvalidTimeout
	}

	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return ErrSchedulerStopped
	}
	if _, exists := s.callbacks[cb.ID]; exists {
		s.mu.Unlock()
		return ErrCallbackAlreadyExists
	}
	s.callbacks[cb.ID] = cb
	s.mu.Unlock()

	return nil
}

func (s *Scheduler) Trigger(callbackID string) error {
	return s.TriggerWithData(callbackID, nil)
}

func (s *Scheduler) TriggerWithData(callbackID string, data interface{}) error {
	s.mu.Lock()
	cb, exists := s.callbacks[callbackID]
	if !exists {
		s.mu.Unlock()
		return ErrCallbackNotFound
	}
	if !s.running {
		s.mu.Unlock()
		return ErrSchedulerStopped
	}
	if cb.Status == CallbackStatusCancelled {
		s.mu.Unlock()
		return ErrCallbackCancelled
	}

	pd := &pendingDelivery{
		callback:    cb,
		attempt:     0,
		scheduledAt: time.Now(),
		data:        data,
	}
	heap.Push(s.pending, pd)
	s.wake()
	s.mu.Unlock()

	return nil
}

func (s *Scheduler) Cancel(callbackID string) error {
	s.mu.Lock()
	cb, exists := s.callbacks[callbackID]
	if !exists {
		s.mu.Unlock()
		return ErrCallbackNotFound
	}
	cb.Status = CallbackStatusCancelled
	cb.UpdatedAt = time.Now()
	_, hasResult := s.deliveryResults[callbackID]
	s.mu.Unlock()

	if !hasResult {
		s.markFinalResult(callbackID, nil, true, nil)
	}
	return nil
}

func (s *Scheduler) GetCallback(callbackID string) (*Callback, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cb, exists := s.callbacks[callbackID]
	if !exists {
		return nil, ErrCallbackNotFound
	}
	return cb, nil
}

func (s *Scheduler) GetDeliveries(callbackID string) ([]*Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.callbacks[callbackID]; !exists {
		return nil, ErrCallbackNotFound
	}

	dels, ok := s.deliveries[callbackID]
	if !ok {
		return []*Delivery{}, nil
	}
	result := make([]*Delivery, len(dels))
	copy(result, dels)
	return result, nil
}

func (s *Scheduler) GetResult(callbackID string) (*DeliveryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, ok := s.deliveryResults[callbackID]
	if !ok {
		return nil, ErrDeliveryNotFound
	}
	return result, nil
}

func (s *Scheduler) WaitForResult(ctx context.Context, callbackID string) (*DeliveryResult, error) {
	s.mu.Lock()
	result, ok := s.deliveryResults[callbackID]
	if ok {
		s.mu.Unlock()
		return result, nil
	}
	s.mu.Unlock()

	s.notifyMu.Lock()
	ch, exists := s.notifyChan[callbackID]
	if !exists {
		ch = make(chan struct{})
		s.notifyChan[callbackID] = ch
	}
	s.notifyMu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ch:
	}

	s.mu.Lock()
	result, ok = s.deliveryResults[callbackID]
	s.mu.Unlock()
	if ok {
		return result, nil
	}
	return nil, ErrDeliveryNotFound
}

func (s *Scheduler) CallbackCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.callbacks)
}

func (s *Scheduler) PendingCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pending.Len() + int(atomic.LoadInt64(&s.activeCount))
}

func (s *Scheduler) ActiveCount() int {
	return int(atomic.LoadInt64(&s.activeCount))
}

func (s *Scheduler) dispatchLoop() {
	defer s.wg.Done()

	for {
		s.mu.Lock()

		if !s.running && s.pending.Len() == 0 && atomic.LoadInt64(&s.activeCount) == 0 {
			s.mu.Unlock()
			return
		}

		if s.pending.Len() == 0 {
			wakeCh := s.wakeCh
			stopCh := s.stopCh
			isStopping := !s.running
			s.mu.Unlock()

			if isStopping {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			select {
			case <-stopCh:
				continue
			case <-wakeCh:
				continue
			}
		}

		pd := s.pending.Peek()
		if pd == nil {
			s.mu.Unlock()
			continue
		}

		waitTime := time.Until(pd.scheduledAt)
		if waitTime > 0 {
			wakeCh := s.wakeCh
			stopCh := s.stopCh
			timer := time.NewTimer(waitTime)
			s.mu.Unlock()

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
		}

		pd = heap.Pop(s.pending).(*pendingDelivery)
		sem := s.sem
		wakeCh := s.wakeCh
		stopCh := s.stopCh
		isStopping := !s.running
		s.mu.Unlock()

		var acquired bool
		if isStopping {
			sem <- struct{}{}
			acquired = true
		} else {
			select {
			case sem <- struct{}{}:
				acquired = true
			case <-stopCh:
				s.mu.Lock()
				heap.Push(s.pending, pd)
				s.mu.Unlock()
			case <-wakeCh:
				s.mu.Lock()
				heap.Push(s.pending, pd)
				s.wake()
				s.mu.Unlock()
			}
		}

		if !acquired {
			continue
		}

		atomic.AddInt64(&s.activeCount, 1)
		s.wg.Add(1)
		go func(p *pendingDelivery) {
			defer s.wg.Done()
			defer atomic.AddInt64(&s.activeCount, -1)
			defer func() { <-sem }()
			s.executeDelivery(p)
		}(pd)
	}
}

func (s *Scheduler) executeDelivery(pd *pendingDelivery) {
	s.mu.Lock()
	cb := pd.callback
	if cb.Status == CallbackStatusCancelled {
		s.mu.Unlock()
		s.markFinalResult(cb.ID, nil, true, nil)
		return
	}
	attempt := pd.attempt + 1
	data := pd.data
	s.mu.Unlock()

	delivery := s.sendRequest(cb, attempt, data)

	s.mu.Lock()
	s.deliveries[cb.ID] = append(s.deliveries[cb.ID], delivery)
	s.mu.Unlock()

	if delivery.Status == DeliveryStatusSucceeded {
		s.mu.Lock()
		cb.Status = CallbackStatusSucceeded
		cb.UpdatedAt = time.Now()
		s.mu.Unlock()
		s.markFinalResult(cb.ID, delivery, true, nil)
		return
	}

	if attempt > cb.RetryPolicy.MaxRetries {
		s.mu.Lock()
		cb.Status = CallbackStatusFailed
		cb.UpdatedAt = time.Now()
		s.mu.Unlock()
		s.markFinalResult(cb.ID, delivery, true, fmt.Errorf("max retries exhausted: %s", delivery.Error))
		return
	}

	s.mu.Lock()
	if cb.Status == CallbackStatusCancelled {
		s.mu.Unlock()
		s.markFinalResult(cb.ID, delivery, true, nil)
		return
	}
	delay := cb.RetryPolicy.BackoffDelay(attempt)
	next := &pendingDelivery{
		callback:    cb,
		attempt:     attempt,
		scheduledAt: time.Now().Add(delay),
		data:        data,
	}
	heap.Push(s.pending, next)
	s.wake()
	s.mu.Unlock()
}

func (s *Scheduler) sendRequest(cb *Callback, attempt int, data interface{}) *Delivery {
	delivery := &Delivery{
		ID:         s.generateID("dlv"),
		CallbackID: cb.ID,
		Attempt:    attempt,
		Status:     DeliveryStatusPending,
		StartedAt:  time.Now(),
	}

	body, err := renderTemplate(cb.BodyTemplate, data)
	if err != nil {
		delivery.FinishedAt = time.Now()
		delivery.Duration = delivery.FinishedAt.Sub(delivery.StartedAt)
		delivery.Status = DeliveryStatusFailed
		delivery.Error = fmt.Sprintf("render template: %v", err)
		return delivery
	}

	req, err := http.NewRequest(cb.Method, cb.URL, bytes.NewReader(body))
	if err != nil {
		delivery.FinishedAt = time.Now()
		delivery.Duration = delivery.FinishedAt.Sub(delivery.StartedAt)
		delivery.Status = DeliveryStatusFailed
		delivery.Error = fmt.Sprintf("create request: %v", err)
		return delivery
	}

	for k, v := range cb.Headers {
		req.Header.Set(k, v)
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	if cb.Secret != "" {
		sig := GenerateHMACSHA256(cb.Secret, body, timestamp)
		req.Header.Set(SignatureHeader, sig)
		req.Header.Set(TimestampHeader, timestamp)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	ctx, cancel := context.WithTimeout(context.Background(), cb.Timeout)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		delivery.FinishedAt = time.Now()
		delivery.Duration = delivery.FinishedAt.Sub(delivery.StartedAt)
		if ctx.Err() == context.DeadlineExceeded || (err != nil && strings.Contains(err.Error(), "context deadline exceeded")) {
			delivery.Status = DeliveryStatusTimeout
			delivery.Error = "request timeout"
		} else {
			delivery.Status = DeliveryStatusFailed
			delivery.Error = fmt.Sprintf("send request: %v", err)
		}
		return delivery
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	delivery.StatusCode = resp.StatusCode
	delivery.ResponseBody = string(respBody)
	delivery.FinishedAt = time.Now()
	delivery.Duration = delivery.FinishedAt.Sub(delivery.StartedAt)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		delivery.Status = DeliveryStatusSucceeded
	} else {
		delivery.Status = DeliveryStatusFailed
		delivery.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return delivery
}

func (s *Scheduler) markFinalResult(callbackID string, delivery *Delivery, final bool, err error) {
	s.mu.Lock()
	s.deliveryResults[callbackID] = &DeliveryResult{
		Delivery: delivery,
		Final:    final,
		Error:    err,
	}
	s.mu.Unlock()

	s.notifyMu.Lock()
	if ch, ok := s.notifyChan[callbackID]; ok {
		close(ch)
		delete(s.notifyChan, callbackID)
	}
	s.notifyMu.Unlock()
}

func renderTemplate(tpl string, data interface{}) ([]byte, error) {
	if tpl == "" {
		return []byte{}, nil
	}
	if data == nil {
		return []byte(tpl), nil
	}
	t, err := template.New("webhook").Parse(tpl)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	return buf.Bytes(), nil
}
