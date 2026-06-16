package auditlog

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var (
	ErrLoggerStopped         = errors.New("audit logger is stopped")
	ErrLoggerAlreadyStarted  = errors.New("audit logger already started")
	ErrWriteFailed           = errors.New("audit log write failed after retries")
	ErrInvalidConfig         = errors.New("invalid configuration")
	ErrLogNotFound           = errors.New("audit log not found")
	ErrHashChainBroken       = errors.New("audit log hash chain is broken")
	ErrNilWriter             = errors.New("writer cannot be nil")
)

type OperationType int

const (
	OpCreate OperationType = iota
	OpRead
	OpUpdate
	OpDelete
	OpLogin
	OpLogout
	OpCustom
)

func (t OperationType) String() string {
	switch t {
	case OpCreate:
		return "CREATE"
	case OpRead:
		return "READ"
	case OpUpdate:
		return "UPDATE"
	case OpDelete:
		return "DELETE"
	case OpLogin:
		return "LOGIN"
	case OpLogout:
		return "LOGOUT"
	default:
		return "CUSTOM"
	}
}

type OperationResult int

const (
	ResultSuccess OperationResult = iota
	ResultFailure
)

func (r OperationResult) String() string {
	if r == ResultSuccess {
		return "SUCCESS"
	}
	return "FAILURE"
}

type AuditLog struct {
	EventID       string
	Timestamp     time.Time
	SubjectID     string
	Operation     OperationType
	OperationDesc string
	ResourceID    string
	ResourceType  string
	Result        OperationResult
	SourceIP      string
	UserAgent     string
	Detail        string
	PreviousHash  string
	CurrentHash   string
}

type Entry struct {
	SubjectID    string
	Operation    OperationType
	ResourceID   string
	ResourceType string
	Result       OperationResult
	SourceIP     string
	UserAgent    string
	Detail       string
}

type Writer interface {
	Write(log *AuditLog) error
}

type MemoryWriter struct {
	mu   sync.RWMutex
	logs []*AuditLog
}

func NewMemoryWriter() *MemoryWriter {
	return &MemoryWriter{
		logs: make([]*AuditLog, 0),
	}
}

func (mw *MemoryWriter) Write(log *AuditLog) error {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	copyLog := *log
	mw.logs = append(mw.logs, &copyLog)
	return nil
}

func (mw *MemoryWriter) ReadAll() []*AuditLog {
	mw.mu.RLock()
	defer mw.mu.RUnlock()
	result := make([]*AuditLog, len(mw.logs))
	for i, log := range mw.logs {
		cp := *log
		result[i] = &cp
	}
	return result
}

func (mw *MemoryWriter) Count() int {
	mw.mu.RLock()
	defer mw.mu.RUnlock()
	return len(mw.logs)
}

type Config struct {
	MaxRetries      int
	RetryInterval   time.Duration
	BufferSize      int
	WorkerCount     int
	EnableHashChain bool
}

func DefaultConfig() Config {
	return Config{
		MaxRetries:      3,
		RetryInterval:   50 * time.Millisecond,
		BufferSize:      1024,
		WorkerCount:     1,
		EnableHashChain: true,
	}
}

type DegradeHandler func(entry *Entry, err error)

type Logger struct {
	mu             sync.Mutex
	cfg            Config
	writer         Writer
	degradeHandler DegradeHandler

	logs        []*AuditLog
	lastHash    string
	idCounter   uint64

	buffer      chan *AuditLog
	started     bool
	stopped     bool
	stopCh      chan struct{}
	wg          sync.WaitGroup

	indexBySubject map[string][]*AuditLog
	indexByResource map[string][]*AuditLog
}

func NewLogger(writer Writer) (*Logger, error) {
	return NewLoggerWithConfig(writer, DefaultConfig())
}

func NewLoggerWithConfig(writer Writer, cfg Config) (*Logger, error) {
	if writer == nil {
		return nil, ErrNilWriter
	}
	if cfg.MaxRetries < 0 {
		return nil, fmt.Errorf("%w: MaxRetries must be >= 0", ErrInvalidConfig)
	}
	if cfg.RetryInterval < 0 {
		return nil, fmt.Errorf("%w: RetryInterval must be >= 0", ErrInvalidConfig)
	}
	if cfg.BufferSize <= 0 {
		return nil, fmt.Errorf("%w: BufferSize must be > 0", ErrInvalidConfig)
	}
	if cfg.WorkerCount <= 0 {
		return nil, fmt.Errorf("%w: WorkerCount must be > 0", ErrInvalidConfig)
	}

	return &Logger{
		cfg:             cfg,
		writer:          writer,
		logs:            make([]*AuditLog, 0),
		indexBySubject:  make(map[string][]*AuditLog),
		indexByResource: make(map[string][]*AuditLog),
		buffer:          make(chan *AuditLog, cfg.BufferSize),
		stopCh:          make(chan struct{}),
	}, nil
}

func (l *Logger) SetDegradeHandler(h DegradeHandler) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.degradeHandler = h
}

func (l *Logger) Start() error {
	l.mu.Lock()
	if l.started {
		l.mu.Unlock()
		return ErrLoggerAlreadyStarted
	}
	l.started = true
	l.stopped = false
	l.stopCh = make(chan struct{})
	l.mu.Unlock()

	for i := 0; i < l.cfg.WorkerCount; i++ {
		l.wg.Add(1)
		go l.workerLoop()
	}

	return nil
}

func (l *Logger) Stop() {
	l.mu.Lock()
	if l.stopped {
		l.mu.Unlock()
		return
	}
	l.stopped = true
	close(l.stopCh)
	l.mu.Unlock()

	l.wg.Wait()

	close(l.buffer)
	for log := range l.buffer {
		l.persistWithRetry(log)
	}
}

func (l *Logger) workerLoop() {
	defer l.wg.Done()
	for {
		select {
		case <-l.stopCh:
			return
		case log, ok := <-l.buffer:
			if !ok {
				return
			}
			l.persistWithRetry(log)
		}
	}
}

func (l *Logger) persistWithRetry(log *AuditLog) {
	var lastErr error
	for attempt := 0; attempt <= l.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(l.cfg.RetryInterval)
		}
		err := func() (retErr error) {
			defer func() {
				if r := recover(); r != nil {
					retErr = fmt.Errorf("writer panic: %v", r)
				}
			}()
			return l.writer.Write(log)
		}()
		if err == nil {
			return
		}
		lastErr = err
	}

	l.mu.Lock()
	handler := l.degradeHandler
	l.mu.Unlock()

	if handler != nil {
		entry := &Entry{
			SubjectID:    log.SubjectID,
			Operation:    log.Operation,
			ResourceID:   log.ResourceID,
			ResourceType: log.ResourceType,
			Result:       log.Result,
			SourceIP:     log.SourceIP,
			UserAgent:    log.UserAgent,
			Detail:       log.Detail,
		}
		func() {
			defer func() { _ = recover() }()
			handler(entry, fmt.Errorf("%w: %v", ErrWriteFailed, lastErr))
		}()
	}
}

func (l *Logger) generateEventID() string {
	l.idCounter++
	return fmt.Sprintf("audit-%d-%d", time.Now().UnixNano(), l.idCounter)
}

func computeHash(log *AuditLog) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s|%d|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s",
		log.EventID,
		log.Timestamp.UnixNano(),
		log.SubjectID,
		log.Operation.String(),
		log.OperationDesc,
		log.ResourceID,
		log.ResourceType,
		log.Result.String(),
		log.SourceIP,
		log.UserAgent,
		log.Detail,
		log.PreviousHash,
	)))
	return hex.EncodeToString(h.Sum(nil))
}

func (l *Logger) createLog(entry *Entry) *AuditLog {
	l.mu.Lock()
	defer l.mu.Unlock()

	log := &AuditLog{
		EventID:       l.generateEventID(),
		Timestamp:     time.Now(),
		SubjectID:     entry.SubjectID,
		Operation:     entry.Operation,
		OperationDesc: entry.Operation.String(),
		ResourceID:    entry.ResourceID,
		ResourceType:  entry.ResourceType,
		Result:        entry.Result,
		SourceIP:      entry.SourceIP,
		UserAgent:     entry.UserAgent,
		Detail:        entry.Detail,
		PreviousHash:  l.lastHash,
	}

	if l.cfg.EnableHashChain {
		log.CurrentHash = computeHash(log)
		l.lastHash = log.CurrentHash
	}

	cp := *log
	l.logs = append(l.logs, &cp)

	if entry.SubjectID != "" {
		l.indexBySubject[entry.SubjectID] = append(l.indexBySubject[entry.SubjectID], &cp)
	}
	if entry.ResourceID != "" {
		l.indexByResource[entry.ResourceID] = append(l.indexByResource[entry.ResourceID], &cp)
	}

	return log
}

func (l *Logger) Log(entry *Entry) error {
	if entry == nil {
		return errors.New("entry cannot be nil")
	}

	l.mu.Lock()
	if l.stopped {
		l.mu.Unlock()
		return ErrLoggerStopped
	}
	if !l.started {
		l.mu.Unlock()
		return ErrLoggerStopped
	}
	l.mu.Unlock()

	log := l.createLog(entry)

	select {
	case l.buffer <- log:
		return nil
	default:
		go l.persistWithRetry(log)
		return nil
	}
}

func (l *Logger) LogSync(entry *Entry) error {
	if entry == nil {
		return errors.New("entry cannot be nil")
	}

	l.mu.Lock()
	if l.stopped {
		l.mu.Unlock()
		return ErrLoggerStopped
	}
	l.mu.Unlock()

	log := l.createLog(entry)
	var lastErr error
	for attempt := 0; attempt <= l.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(l.cfg.RetryInterval)
		}
		err := l.writer.Write(log)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("%w: %v", ErrWriteFailed, lastErr)
}

type Query struct {
	SubjectID   string
	ResourceID  string
	StartTime   *time.Time
	EndTime     *time.Time
}

func (l *Logger) Query(q Query) []*AuditLog {
	l.mu.Lock()
	defer l.mu.Unlock()

	var candidates []*AuditLog

	switch {
	case q.SubjectID != "" && q.ResourceID != "":
		subjectLogs := l.indexBySubject[q.SubjectID]
		resourceLogs := l.indexByResource[q.ResourceID]
		resourceSet := make(map[string]bool, len(resourceLogs))
		for _, rl := range resourceLogs {
			resourceSet[rl.EventID] = true
		}
		for _, sl := range subjectLogs {
			if resourceSet[sl.EventID] {
				candidates = append(candidates, sl)
			}
		}
	case q.SubjectID != "":
		candidates = append(candidates, l.indexBySubject[q.SubjectID]...)
	case q.ResourceID != "":
		candidates = append(candidates, l.indexByResource[q.ResourceID]...)
	default:
		for _, log := range l.logs {
			candidates = append(candidates, log)
		}
	}

	result := make([]*AuditLog, 0, len(candidates))
	for _, log := range candidates {
		if q.StartTime != nil && log.Timestamp.Before(*q.StartTime) {
			continue
		}
		if q.EndTime != nil && log.Timestamp.After(*q.EndTime) {
			continue
		}
		cp := *log
		result = append(result, &cp)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	return result
}

func (l *Logger) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.logs)
}

func (l *Logger) GetByEventID(eventID string) (*AuditLog, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, log := range l.logs {
		if log.EventID == eventID {
			cp := *log
			return &cp, nil
		}
	}
	return nil, ErrLogNotFound
}

type VerificationResult struct {
	Valid         bool
	TamperedIndex int
	Message       string
}

func (l *Logger) VerifyIntegrity() VerificationResult {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.logs) == 0 {
		return VerificationResult{Valid: true, TamperedIndex: -1, Message: "no logs"}
	}

	if l.logs[0].PreviousHash != "" {
		return VerificationResult{
			Valid:         false,
			TamperedIndex: 0,
			Message:       "first log should have empty previous hash",
		}
	}

	for i, log := range l.logs {
		expectedHash := computeHash(log)
		if log.CurrentHash != expectedHash {
			return VerificationResult{
				Valid:         false,
				TamperedIndex: i,
				Message:       fmt.Sprintf("log at index %d (EventID=%s) has been tampered: hash mismatch", i, log.EventID),
			}
		}

		if i > 0 {
			prevHash := l.logs[i-1].CurrentHash
			if log.PreviousHash != prevHash {
				return VerificationResult{
					Valid:         false,
					TamperedIndex: i,
					Message:       fmt.Sprintf("hash chain broken at index %d (EventID=%s): previous hash does not match preceding log's hash", i, log.EventID),
				}
			}
		}
	}

	return VerificationResult{
		Valid:         true,
		TamperedIndex: -1,
		Message:       fmt.Sprintf("all %d logs are intact", len(l.logs)),
	}
}

func (l *Logger) tamperLog(index int, newDetail string) {
	if index >= 0 && index < len(l.logs) {
		l.logs[index].Detail = newDetail
	}
}

func (l *Logger) breakChain(index int) {
	if index >= 0 && index < len(l.logs) {
		l.logs[index].PreviousHash = "broken-hash"
	}
}
