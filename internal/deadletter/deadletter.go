package deadletter

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrProcessorStopped     = errors.New("processor is stopped")
	ErrMessageNotFound      = errors.New("message not found")
	ErrInvalidConfig        = errors.New("invalid configuration")
	ErrMaxRetriesExceeded   = errors.New("max retries exceeded")
	ErrAlreadyStarted       = errors.New("processor already started")
	ErrHandlerNotSet        = errors.New("handler not set")
)

type MessageStatus int

const (
	StatusPending MessageStatus = iota
	StatusRetrying
	StatusPermanentlyFailed
	StatusProcessed
)

type DelayStrategyType int

const (
	DelayStrategyFixed DelayStrategyType = iota
	DelayStrategyExponential
)

type DeadLetterMessage struct {
	ID             string
	OriginalTopic  string
	Payload        interface{}
	FailureReason  string
	TransferTime   time.Time
	RetryCount     int
	MaxRetries     int
	NextRetryAt    time.Time
	Status         MessageStatus
	LastError      string
}

type AlertInfo struct {
	TotalCount    int
	ReasonStats   map[string]int
	Threshold     int
	Timestamp     time.Time
}

type AlertCallback func(info AlertInfo)
type MessageHandler func(ctx context.Context, msg *DeadLetterMessage) error

type DelayStrategy struct {
	Type     DelayStrategyType
	Base     time.Duration
	Max      time.Duration
}

type Config struct {
	MaxRetries      int
	DelayStrategy   DelayStrategy
	AlertThreshold  int
	AlertCallback   AlertCallback
}

type Processor struct {
	mu           sync.Mutex
	cfg          Config
	messages     map[string]*DeadLetterMessage
	handler      MessageHandler
	running      bool
	stopCh       chan struct{}
	wakeCh       chan struct{}
	wg           sync.WaitGroup
	taskWg       sync.WaitGroup
	nextID       uint64
	idMu         sync.Mutex
	alertFired   bool
}

func NewProcessor(cfg Config) (*Processor, error) {
	if cfg.MaxRetries < 0 {
		return nil, fmt.Errorf("%w: MaxRetries must be >= 0", ErrInvalidConfig)
	}
	if cfg.AlertThreshold < 0 {
		return nil, fmt.Errorf("%w: AlertThreshold must be >= 0", ErrInvalidConfig)
	}
	if cfg.DelayStrategy.Base <= 0 {
		return nil, fmt.Errorf("%w: DelayStrategy.Base must be > 0", ErrInvalidConfig)
	}
	if cfg.DelayStrategy.Max < cfg.DelayStrategy.Base {
		return nil, fmt.Errorf("%w: DelayStrategy.Max must be >= Base", ErrInvalidConfig)
	}

	return &Processor{
		cfg:      cfg,
		messages: make(map[string]*DeadLetterMessage),
	}, nil
}

func (p *Processor) generateID() string {
	p.idMu.Lock()
	defer p.idMu.Unlock()
	p.nextID++
	return fmt.Sprintf("dl-%d-%d", time.Now().UnixNano(), p.nextID)
}

func (p *Processor) SetHandler(handler MessageHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handler = handler
}

func (p *Processor) Start() error {
	p.mu.Lock()
	if p.handler == nil {
		p.mu.Unlock()
		return ErrHandlerNotSet
	}
	if p.running {
		p.mu.Unlock()
		return ErrAlreadyStarted
	}
	p.running = true
	p.stopCh = make(chan struct{})
	p.wakeCh = make(chan struct{})
	p.mu.Unlock()

	p.wg.Add(1)
	go p.runLoop()
	return nil
}

func (p *Processor) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	close(p.stopCh)
	p.wake()
	p.mu.Unlock()

	p.wg.Wait()
	p.taskWg.Wait()
}

func (p *Processor) wake() {
	select {
	case <-p.wakeCh:
	default:
		close(p.wakeCh)
	}
	p.wakeCh = make(chan struct{})
}

func (p *Processor) MoveToDeadLetter(originalTopic string, payload interface{}, failureReason string, maxRetries int) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return "", ErrProcessorStopped
	}

	id := p.generateID()
	now := time.Now()
	nextRetryAt := p.calculateNextRetry(0)

	if maxRetries < 0 {
		maxRetries = p.cfg.MaxRetries
	}

	msg := &DeadLetterMessage{
		ID:            id,
		OriginalTopic: originalTopic,
		Payload:       payload,
		FailureReason: failureReason,
		TransferTime:  now,
		RetryCount:    0,
		MaxRetries:    maxRetries,
		NextRetryAt:   nextRetryAt,
		Status:        StatusPending,
		LastError:     failureReason,
	}

	p.messages[id] = msg
	p.wake()
	p.checkAlertThreshold()

	return id, nil
}

func (p *Processor) calculateNextRetry(retryCount int) time.Time {
	var delay time.Duration

	switch p.cfg.DelayStrategy.Type {
	case DelayStrategyExponential:
		delay = p.cfg.DelayStrategy.Base * time.Duration(1<<uint(retryCount))
		if delay > p.cfg.DelayStrategy.Max {
			delay = p.cfg.DelayStrategy.Max
		}
	default:
		delay = p.cfg.DelayStrategy.Base
	}

	return time.Now().Add(delay)
}

func (p *Processor) checkAlertThreshold() {
	if p.cfg.AlertThreshold == 0 || p.cfg.AlertCallback == nil {
		return
	}

	pendingCount := 0
	reasonStats := make(map[string]int)

	for _, msg := range p.messages {
		if msg.Status == StatusPending || msg.Status == StatusRetrying {
			pendingCount++
			reasonStats[msg.FailureReason]++
		}
	}

	if pendingCount >= p.cfg.AlertThreshold && !p.alertFired {
		p.alertFired = true
		go p.cfg.AlertCallback(AlertInfo{
			TotalCount:  pendingCount,
			ReasonStats: reasonStats,
			Threshold:   p.cfg.AlertThreshold,
			Timestamp:   time.Now(),
		})
	} else if pendingCount < p.cfg.AlertThreshold {
		p.alertFired = false
	}
}

func (p *Processor) GetMessage(id string) (*DeadLetterMessage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	msg, exists := p.messages[id]
	if !exists {
		return nil, ErrMessageNotFound
	}

	copyMsg := *msg
	return &copyMsg, nil
}

func (p *Processor) GetAllMessages() []*DeadLetterMessage {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := make([]*DeadLetterMessage, 0, len(p.messages))
	for _, msg := range p.messages {
		copyMsg := *msg
		result = append(result, &copyMsg)
	}
	return result
}

func (p *Processor) GetMessagesByStatus(status MessageStatus) []*DeadLetterMessage {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := make([]*DeadLetterMessage, 0)
	for _, msg := range p.messages {
		if msg.Status == status {
			copyMsg := *msg
			result = append(result, &copyMsg)
		}
	}
	return result
}

func (p *Processor) PendingCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	count := 0
	for _, msg := range p.messages {
		if msg.Status == StatusPending || msg.Status == StatusRetrying {
			count++
		}
	}
	return count
}

func (p *Processor) PermanentlyFailedCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	count := 0
	for _, msg := range p.messages {
		if msg.Status == StatusPermanentlyFailed {
			count++
		}
	}
	return count
}

func (p *Processor) RemoveMessage(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, exists := p.messages[id]
	if !exists {
		return ErrMessageNotFound
	}

	delete(p.messages, id)
	p.checkAlertThreshold()
	return nil
}

func (p *Processor) ClearPermanentlyFailed() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	count := 0
	for id, msg := range p.messages {
		if msg.Status == StatusPermanentlyFailed {
			delete(p.messages, id)
			count++
		}
	}
	return count
}

func (p *Processor) runLoop() {
	defer p.wg.Done()

	for {
		p.mu.Lock()
		if !p.running {
			p.mu.Unlock()
			return
		}

		now := time.Now()
		var earliestPending time.Time
		var hasEarliestPending bool
		var msgToRetry *DeadLetterMessage

		for _, msg := range p.messages {
			if msg.Status == StatusPending {
				if !hasEarliestPending || msg.NextRetryAt.Before(earliestPending) {
					earliestPending = msg.NextRetryAt
					hasEarliestPending = true
				}
				if !msg.NextRetryAt.After(now) {
					if msgToRetry == nil || msg.NextRetryAt.Before(msgToRetry.NextRetryAt) {
						msgToRetry = msg
					}
				}
			}
		}

		if msgToRetry != nil {
			msgToRetry.Status = StatusRetrying
			handler := p.handler
			msgCopy := *msgToRetry
			p.taskWg.Add(1)
			p.mu.Unlock()

			go p.processMessage(&msgCopy, handler)
			continue
		}

		var waitTime time.Duration
		if hasEarliestPending {
			waitTime = earliestPending.Sub(now)
			if waitTime < 0 {
				waitTime = 0
			}
		} else {
			waitTime = time.Hour
		}

		wakeCh := p.wakeCh
		stopCh := p.stopCh
		p.mu.Unlock()

		timer := time.NewTimer(waitTime)
		select {
		case <-timer.C:
		case <-wakeCh:
			timer.Stop()
		case <-stopCh:
			timer.Stop()
			return
		}
	}
}

func (p *Processor) processMessage(msg *DeadLetterMessage, handler MessageHandler) {
	defer p.taskWg.Done()

	ctx := context.Background()
	err := func() (retErr error) {
		defer func() {
			if r := recover(); r != nil {
				retErr = fmt.Errorf("handler panic: %v", r)
			}
		}()
		return handler(ctx, msg)
	}()

	p.mu.Lock()
	defer p.mu.Unlock()

	currentMsg, exists := p.messages[msg.ID]
	if !exists {
		return
	}

	if err == nil {
		currentMsg.Status = StatusProcessed
		delete(p.messages, msg.ID)
		p.checkAlertThreshold()
		return
	}

	currentMsg.RetryCount++
	currentMsg.LastError = err.Error()

	if currentMsg.RetryCount > currentMsg.MaxRetries {
		currentMsg.Status = StatusPermanentlyFailed
		p.checkAlertThreshold()
		return
	}

	currentMsg.NextRetryAt = p.calculateNextRetry(currentMsg.RetryCount)
	currentMsg.Status = StatusPending
	p.wake()
}

func (p *Processor) RetryMessage(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	msg, exists := p.messages[id]
	if !exists {
		return ErrMessageNotFound
	}

	if msg.Status == StatusPermanentlyFailed {
		return ErrMaxRetriesExceeded
	}

	msg.NextRetryAt = time.Now()
	msg.Status = StatusPending
	p.wake()
	return nil
}
