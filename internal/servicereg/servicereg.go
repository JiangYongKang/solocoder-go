package servicereg

import (
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrInstanceNotFound   = errors.New("servicereg: instance not found")
	ErrInstanceExists     = errors.New("servicereg: instance already exists")
	ErrServiceNotFound    = errors.New("servicereg: service not found")
	ErrRegistryStopped    = errors.New("servicereg: registry is stopped")
	ErrSubscriberNotFound = errors.New("servicereg: subscriber not found")
	ErrNoInstances        = errors.New("servicereg: no available instances")
	ErrInvalidInstance    = errors.New("servicereg: invalid instance")
)

const (
	defaultHeartbeatTTL = 30 * time.Second
	defaultCheckInterval = 1 * time.Second
)

type HealthStatus struct {
	CPUUsage          float64
	MemoryUsage       float64
	RequestSuccessRate float64
}

type ServiceInstance struct {
	ID            string
	ServiceName   string
	Address       string
	Health        HealthStatus
	LastHeartbeat time.Time
}

type RegistryConfig struct {
	HeartbeatTTL   time.Duration
	CheckInterval  time.Duration
}

func DefaultRegistryConfig() RegistryConfig {
	return RegistryConfig{
		HeartbeatTTL:  defaultHeartbeatTTL,
		CheckInterval: defaultCheckInterval,
	}
}

type ServiceChangeEvent struct {
	ServiceName string
	Instances   []*ServiceInstance
	Action      string
}

type SubscriberFunc func(event ServiceChangeEvent)

type subscriber struct {
	ID      string
	Handler SubscriberFunc
}

type Registry struct {
	mu          sync.RWMutex
	instances   map[string]map[string]*ServiceInstance
	subscribers map[string]map[string]*subscriber
	cfg         RegistryConfig
	running     bool
	stopCh      chan struct{}
	wg          sync.WaitGroup
	nextSubID   uint64
}

func NewRegistry(cfg RegistryConfig) *Registry {
	if cfg.HeartbeatTTL <= 0 {
		cfg.HeartbeatTTL = defaultHeartbeatTTL
	}
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = defaultCheckInterval
	}

	return &Registry{
		instances:   make(map[string]map[string]*ServiceInstance),
		subscribers: make(map[string]map[string]*subscriber),
		cfg:         cfg,
		stopCh:      make(chan struct{}),
		running:     true,
	}
}

func (r *Registry) Register(inst *ServiceInstance) error {
	if inst == nil || inst.ID == "" || inst.ServiceName == "" || inst.Address == "" {
		return ErrInvalidInstance
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return ErrRegistryStopped
	}

	svc, ok := r.instances[inst.ServiceName]
	if !ok {
		svc = make(map[string]*ServiceInstance)
		r.instances[inst.ServiceName] = svc
	}

	if _, exists := svc[inst.ID]; exists {
		return ErrInstanceExists
	}

	now := time.Now()
	inst.LastHeartbeat = now
	svc[inst.ID] = inst

	r.notifySubscribersLocked(inst.ServiceName, "register")

	return nil
}

func (r *Registry) Deregister(serviceName, instanceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return ErrRegistryStopped
	}

	svc, ok := r.instances[serviceName]
	if !ok {
		return ErrServiceNotFound
	}

	if _, exists := svc[instanceID]; !exists {
		return ErrInstanceNotFound
	}

	delete(svc, instanceID)

	if len(svc) == 0 {
		delete(r.instances, serviceName)
	}

	r.notifySubscribersLocked(serviceName, "deregister")

	return nil
}

func (r *Registry) Heartbeat(serviceName, instanceID string, health HealthStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return ErrRegistryStopped
	}

	svc, ok := r.instances[serviceName]
	if !ok {
		return ErrServiceNotFound
	}

	inst, exists := svc[instanceID]
	if !exists {
		return ErrInstanceNotFound
	}

	inst.LastHeartbeat = time.Now()
	inst.Health = health

	return nil
}

func (r *Registry) GetInstances(serviceName string) ([]*ServiceInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.running {
		return nil, ErrRegistryStopped
	}

	svc, ok := r.instances[serviceName]
	if !ok {
		return nil, ErrServiceNotFound
	}

	result := make([]*ServiceInstance, 0, len(svc))
	for _, inst := range svc {
		instCopy := *inst
		result = append(result, &instCopy)
	}

	return result, nil
}

func (r *Registry) GetHealth(serviceName, instanceID string) (*HealthStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.running {
		return nil, ErrRegistryStopped
	}

	svc, ok := r.instances[serviceName]
	if !ok {
		return nil, ErrServiceNotFound
	}

	inst, exists := svc[instanceID]
	if !exists {
		return nil, ErrInstanceNotFound
	}

	healthCopy := inst.Health
	return &healthCopy, nil
}

func (r *Registry) Subscribe(serviceName string, handler SubscriberFunc) (string, error) {
	if handler == nil {
		return "", errors.New("servicereg: handler cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return "", ErrRegistryStopped
	}

	id := r.generateSubID()

	sub := &subscriber{
		ID:      id,
		Handler: handler,
	}

	if _, ok := r.subscribers[serviceName]; !ok {
		r.subscribers[serviceName] = make(map[string]*subscriber)
	}
	r.subscribers[serviceName][id] = sub

	return id, nil
}

func (r *Registry) Unsubscribe(serviceName, subscriberID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return ErrRegistryStopped
	}

	subs, ok := r.subscribers[serviceName]
	if !ok {
		return ErrSubscriberNotFound
	}

	if _, exists := subs[subscriberID]; !exists {
		return ErrSubscriberNotFound
	}

	delete(subs, subscriberID)
	if len(subs) == 0 {
		delete(r.subscribers, serviceName)
	}

	return nil
}

func (r *Registry) notifySubscribersLocked(serviceName string, action string) {
	subs, ok := r.subscribers[serviceName]
	if !ok || len(subs) == 0 {
		return
	}

	svc := r.instances[serviceName]
	instances := make([]*ServiceInstance, 0, len(svc))
	for _, inst := range svc {
		instCopy := *inst
		instances = append(instances, &instCopy)
	}

	event := ServiceChangeEvent{
		ServiceName: serviceName,
		Instances:   instances,
		Action:      action,
	}

	handlers := make([]SubscriberFunc, 0, len(subs))
	for _, sub := range subs {
		handlers = append(handlers, sub.Handler)
	}

	r.mu.Unlock()
	for _, h := range handlers {
		h(event)
	}
	r.mu.Lock()
}

func (r *Registry) generateSubID() string {
	id := atomic.AddUint64(&r.nextSubID, 1)
	return "sub-" + time.Now().Format("20060102150405") + "-" + uint64ToStr(id)
}

func uint64ToStr(n uint64) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 20)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}

func (r *Registry) Start() {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		r.wg.Add(1)
		go r.expiryLoop()
		return
	}
	r.running = true
	r.stopCh = make(chan struct{})
	r.mu.Unlock()

	r.wg.Add(1)
	go r.expiryLoop()
}

func (r *Registry) expiryLoop() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.expireInstances()
		}
	}
}

func (r *Registry) expireInstances() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return
	}

	now := time.Now()
	changed := make(map[string]bool)

	for serviceName, svc := range r.instances {
		for instID, inst := range svc {
			if now.Sub(inst.LastHeartbeat) > r.cfg.HeartbeatTTL {
				delete(svc, instID)
				changed[serviceName] = true
			}
		}
		if len(svc) == 0 {
			delete(r.instances, serviceName)
		}
	}

	for serviceName := range changed {
		r.notifySubscribersLocked(serviceName, "expire")
	}
}

func (r *Registry) Stop() {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return
	}
	r.running = false
	close(r.stopCh)
	r.mu.Unlock()

	r.wg.Wait()
}

func (r *Registry) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

func (r *Registry) InstanceCount(serviceName string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	svc, ok := r.instances[serviceName]
	if !ok {
		return 0
	}
	return len(svc)
}

func (r *Registry) ServiceCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.instances)
}

func (r *Registry) SubscriberCount(serviceName string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	subs, ok := r.subscribers[serviceName]
	if !ok {
		return 0
	}
	return len(subs)
}

type LoadBalancer interface {
	Select(instances []*ServiceInstance) (*ServiceInstance, error)
}

type RoundRobinLB struct {
	counter uint64
}

func NewRoundRobinLB() *RoundRobinLB {
	return &RoundRobinLB{}
}

func (lb *RoundRobinLB) Select(instances []*ServiceInstance) (*ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, ErrNoInstances
	}
	idx := atomic.AddUint64(&lb.counter, 1) - 1
	return instances[idx%uint64(len(instances))], nil
}

type RandomLB struct{}

func NewRandomLB() *RandomLB {
	return &RandomLB{}
}

func (lb *RandomLB) Select(instances []*ServiceInstance) (*ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, ErrNoInstances
	}
	idx := rand.Intn(len(instances))
	return instances[idx], nil
}
