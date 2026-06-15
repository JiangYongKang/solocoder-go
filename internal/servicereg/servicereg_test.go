package servicereg

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if !r.IsRunning() {
		t.Error("registry should be running (accepting operations) after creation")
	}
	if r.ServiceCount() != 0 {
		t.Errorf("expected 0 services, got %d", r.ServiceCount())
	}
	r.Stop()
}

func TestNewRegistryConfigDefaults(t *testing.T) {
	cfg := DefaultRegistryConfig()
	r := NewRegistry(cfg)
	defer r.Stop()

	if r.cfg.HeartbeatTTL != defaultHeartbeatTTL {
		t.Errorf("expected default HeartbeatTTL %v, got %v", defaultHeartbeatTTL, r.cfg.HeartbeatTTL)
	}
	if r.cfg.CheckInterval != defaultCheckInterval {
		t.Errorf("expected default CheckInterval %v, got %v", defaultCheckInterval, r.cfg.CheckInterval)
	}

	cfg2 := RegistryConfig{HeartbeatTTL: -1, CheckInterval: -1}
	r2 := NewRegistry(cfg2)
	defer r2.Stop()
	if r2.cfg.HeartbeatTTL != defaultHeartbeatTTL {
		t.Errorf("expected -1 to map to default HeartbeatTTL %v, got %v", defaultHeartbeatTTL, r2.cfg.HeartbeatTTL)
	}
	if r2.cfg.CheckInterval != defaultCheckInterval {
		t.Errorf("expected -1 to map to default CheckInterval %v, got %v", defaultCheckInterval, r2.cfg.CheckInterval)
	}
}

func TestRegister(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	inst := &ServiceInstance{
		ID:          "inst-1",
		ServiceName: "user-service",
		Address:     "127.0.0.1:8080",
	}
	err := r.Register(inst)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if r.InstanceCount("user-service") != 1 {
		t.Errorf("expected 1 instance, got %d", r.InstanceCount("user-service"))
	}
	if r.ServiceCount() != 1 {
		t.Errorf("expected 1 service, got %d", r.ServiceCount())
	}
}

func TestRegisterDuplicate(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	inst := &ServiceInstance{
		ID:          "inst-1",
		ServiceName: "user-service",
		Address:     "127.0.0.1:8080",
	}
	r.Register(inst)

	err := r.Register(inst)
	if err != ErrInstanceExists {
		t.Errorf("expected ErrInstanceExists, got %v", err)
	}
}

func TestRegisterInvalid(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	tests := []struct {
		name string
		inst *ServiceInstance
	}{
		{"nil", nil},
		{"empty ID", &ServiceInstance{ServiceName: "svc", Address: "addr"}},
		{"empty ServiceName", &ServiceInstance{ID: "id", Address: "addr"}},
		{"empty Address", &ServiceInstance{ID: "id", ServiceName: "svc"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Register(tt.inst)
			if err != ErrInvalidInstance {
				t.Errorf("expected ErrInvalidInstance for %s, got %v", tt.name, err)
			}
		})
	}
}

func TestRegisterStopped(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	r.Stop()

	inst := &ServiceInstance{ID: "id", ServiceName: "svc", Address: "addr"}
	err := r.Register(inst)
	if err != ErrRegistryStopped {
		t.Errorf("expected ErrRegistryStopped, got %v", err)
	}
}

func TestDeregister(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
	r.Register(inst)

	err := r.Deregister("svc", "inst-1")
	if err != nil {
		t.Fatalf("Deregister failed: %v", err)
	}
	if r.InstanceCount("svc") != 0 {
		t.Errorf("expected 0 instances after deregister, got %d", r.InstanceCount("svc"))
	}
	if r.ServiceCount() != 0 {
		t.Errorf("expected 0 services after deregister, got %d", r.ServiceCount())
	}
}

func TestDeregisterNotFound(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	err := r.Deregister("svc", "inst-1")
	if err != ErrServiceNotFound {
		t.Errorf("expected ErrServiceNotFound, got %v", err)
	}

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
	r.Register(inst)

	err = r.Deregister("svc", "inst-2")
	if err != ErrInstanceNotFound {
		t.Errorf("expected ErrInstanceNotFound, got %v", err)
	}
}

func TestDeregisterStopped(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	r.Stop()

	err := r.Deregister("svc", "inst-1")
	if err != ErrRegistryStopped {
		t.Errorf("expected ErrRegistryStopped, got %v", err)
	}
}

func TestHeartbeat(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	inst := &ServiceInstance{
		ID:          "inst-1",
		ServiceName: "svc",
		Address:     "127.0.0.1:8080",
		Health:      HealthStatus{CPUUsage: 50.0, MemoryUsage: 60.0, RequestSuccessRate: 99.5},
	}
	r.Register(inst)

	before := inst.LastHeartbeat
	time.Sleep(time.Millisecond)

	health := HealthStatus{CPUUsage: 75.0, MemoryUsage: 80.0, RequestSuccessRate: 98.0}
	err := r.Heartbeat("svc", "inst-1", health)
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	h, err := r.GetHealth("svc", "inst-1")
	if err != nil {
		t.Fatalf("GetHealth failed: %v", err)
	}
	if h.CPUUsage != 75.0 {
		t.Errorf("expected CPUUsage 75.0, got %f", h.CPUUsage)
	}
	if h.MemoryUsage != 80.0 {
		t.Errorf("expected MemoryUsage 80.0, got %f", h.MemoryUsage)
	}
	if h.RequestSuccessRate != 98.0 {
		t.Errorf("expected RequestSuccessRate 98.0, got %f", h.RequestSuccessRate)
	}

	svc, _ := r.instances["svc"]
	updatedInst := svc["inst-1"]
	if !updatedInst.LastHeartbeat.After(before) {
		t.Error("expected LastHeartbeat to be updated after heartbeat")
	}
}

func TestHeartbeatNotFound(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	err := r.Heartbeat("svc", "inst-1", HealthStatus{})
	if err != ErrServiceNotFound {
		t.Errorf("expected ErrServiceNotFound, got %v", err)
	}

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
	r.Register(inst)

	err = r.Heartbeat("svc", "inst-2", HealthStatus{})
	if err != ErrInstanceNotFound {
		t.Errorf("expected ErrInstanceNotFound, got %v", err)
	}
}

func TestHeartbeatStopped(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	r.Stop()

	err := r.Heartbeat("svc", "inst-1", HealthStatus{})
	if err != ErrRegistryStopped {
		t.Errorf("expected ErrRegistryStopped, got %v", err)
	}
}

func TestGetInstances(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	inst1 := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr1"}
	inst2 := &ServiceInstance{ID: "inst-2", ServiceName: "svc", Address: "addr2"}
	r.Register(inst1)
	r.Register(inst2)

	instances, err := r.GetInstances("svc")
	if err != nil {
		t.Fatalf("GetInstances failed: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}

	ids := make(map[string]bool)
	for _, inst := range instances {
		ids[inst.ID] = true
	}
	if !ids["inst-1"] || !ids["inst-2"] {
		t.Error("missing expected instances")
	}
}

func TestGetInstancesReturnsCopies(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr1"}
	r.Register(inst)

	instances, _ := r.GetInstances("svc")
	instances[0].Address = "modified"

	instances2, _ := r.GetInstances("svc")
	if instances2[0].Address == "modified" {
		t.Error("GetInstances should return copies, but modifications affected internal state")
	}
}

func TestGetInstancesServiceNotFound(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	_, err := r.GetInstances("nonexistent")
	if err != ErrServiceNotFound {
		t.Errorf("expected ErrServiceNotFound, got %v", err)
	}
}

func TestGetInstancesStopped(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	r.Stop()

	_, err := r.GetInstances("svc")
	if err != ErrRegistryStopped {
		t.Errorf("expected ErrRegistryStopped, got %v", err)
	}
}

func TestGetHealth(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	inst := &ServiceInstance{
		ID:          "inst-1",
		ServiceName: "svc",
		Address:     "addr",
		Health:      HealthStatus{CPUUsage: 30.0, MemoryUsage: 50.0, RequestSuccessRate: 99.9},
	}
	r.Register(inst)

	h, err := r.GetHealth("svc", "inst-1")
	if err != nil {
		t.Fatalf("GetHealth failed: %v", err)
	}
	if h.CPUUsage != 30.0 {
		t.Errorf("expected CPUUsage 30.0, got %f", h.CPUUsage)
	}
	if h.MemoryUsage != 50.0 {
		t.Errorf("expected MemoryUsage 50.0, got %f", h.MemoryUsage)
	}
	if h.RequestSuccessRate != 99.9 {
		t.Errorf("expected RequestSuccessRate 99.9, got %f", h.RequestSuccessRate)
	}
}

func TestGetHealthReturnsCopy(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	inst := &ServiceInstance{
		ID:          "inst-1",
		ServiceName: "svc",
		Address:     "addr",
		Health:      HealthStatus{CPUUsage: 30.0},
	}
	r.Register(inst)

	h, _ := r.GetHealth("svc", "inst-1")
	h.CPUUsage = 999.0

	h2, _ := r.GetHealth("svc", "inst-1")
	if h2.CPUUsage == 999.0 {
		t.Error("GetHealth should return a copy")
	}
}

func TestGetHealthNotFound(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	_, err := r.GetHealth("svc", "inst-1")
	if err != ErrServiceNotFound {
		t.Errorf("expected ErrServiceNotFound, got %v", err)
	}

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
	r.Register(inst)

	_, err = r.GetHealth("svc", "inst-2")
	if err != ErrInstanceNotFound {
		t.Errorf("expected ErrInstanceNotFound, got %v", err)
	}
}

func TestGetHealthStopped(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	r.Stop()

	_, err := r.GetHealth("svc", "inst-1")
	if err != ErrRegistryStopped {
		t.Errorf("expected ErrRegistryStopped, got %v", err)
	}
}

func TestSubscribe(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	var receivedEvent ServiceChangeEvent
	var eventMu sync.Mutex

	subID, err := r.Subscribe("svc", func(event ServiceChangeEvent) {
		eventMu.Lock()
		receivedEvent = event
		eventMu.Unlock()
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	if subID == "" {
		t.Error("expected non-empty subscriber ID")
	}
	if r.SubscriberCount("svc") != 1 {
		t.Errorf("expected 1 subscriber, got %d", r.SubscriberCount("svc"))
	}

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
	r.Register(inst)

	eventMu.Lock()
	if receivedEvent.ServiceName != "svc" {
		t.Errorf("expected service name svc, got %s", receivedEvent.ServiceName)
	}
	if receivedEvent.Action != "register" {
		t.Errorf("expected action register, got %s", receivedEvent.Action)
	}
	if len(receivedEvent.Instances) != 1 {
		t.Errorf("expected 1 instance in event, got %d", len(receivedEvent.Instances))
	}
	if receivedEvent.Instances[0].ID != "inst-1" {
		t.Errorf("expected instance ID inst-1, got %s", receivedEvent.Instances[0].ID)
	}
	eventMu.Unlock()
}

func TestSubscribeNilHandler(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	_, err := r.Subscribe("svc", nil)
	if err == nil {
		t.Error("expected error for nil handler")
	}
}

func TestSubscribeStopped(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	r.Stop()

	_, err := r.Subscribe("svc", func(event ServiceChangeEvent) {})
	if err != ErrRegistryStopped {
		t.Errorf("expected ErrRegistryStopped, got %v", err)
	}
}

func TestUnsubscribe(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	subID, _ := r.Subscribe("svc", func(event ServiceChangeEvent) {})

	err := r.Unsubscribe("svc", subID)
	if err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}
	if r.SubscriberCount("svc") != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", r.SubscriberCount("svc"))
	}
}

func TestUnsubscribeNotFound(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	err := r.Unsubscribe("svc", "nonexistent")
	if err != ErrSubscriberNotFound {
		t.Errorf("expected ErrSubscriberNotFound, got %v", err)
	}

	subID, _ := r.Subscribe("svc", func(event ServiceChangeEvent) {})
	err = r.Unsubscribe("svc", "wrong-id")
	if err != ErrSubscriberNotFound {
		t.Errorf("expected ErrSubscriberNotFound, got %v", err)
	}

	err = r.Unsubscribe("other-svc", subID)
	if err != ErrSubscriberNotFound {
		t.Errorf("expected ErrSubscriberNotFound, got %v", err)
	}
}

func TestUnsubscribeStopped(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	r.Stop()

	err := r.Unsubscribe("svc", "sub-1")
	if err != ErrRegistryStopped {
		t.Errorf("expected ErrRegistryStopped, got %v", err)
	}
}

func TestSubscribeDeregisterEvent(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	var receivedAction string
	var eventMu sync.Mutex

	r.Subscribe("svc", func(event ServiceChangeEvent) {
		eventMu.Lock()
		receivedAction = event.Action
		eventMu.Unlock()
	})

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
	r.Register(inst)

	eventMu.Lock()
	if receivedAction != "register" {
		t.Errorf("expected action register, got %s", receivedAction)
	}
	eventMu.Unlock()

	r.Deregister("svc", "inst-1")

	eventMu.Lock()
	if receivedAction != "deregister" {
		t.Errorf("expected action deregister, got %s", receivedAction)
	}
	eventMu.Unlock()
}

func TestMultipleSubscribers(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	var count int32

	r.Subscribe("svc", func(event ServiceChangeEvent) {
		atomic.AddInt32(&count, 1)
	})
	r.Subscribe("svc", func(event ServiceChangeEvent) {
		atomic.AddInt32(&count, 1)
	})

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
	r.Register(inst)

	if atomic.LoadInt32(&count) != 2 {
		t.Errorf("expected 2 notifications, got %d", atomic.LoadInt32(&count))
	}
}

func TestHeartbeatExpiry(t *testing.T) {
	cfg := RegistryConfig{
		HeartbeatTTL:  100 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
	}
	r := NewRegistry(cfg)
	r.Start()
	defer r.Stop()

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
	r.Register(inst)

	if r.InstanceCount("svc") != 1 {
		t.Fatalf("expected 1 instance before expiry, got %d", r.InstanceCount("svc"))
	}

	time.Sleep(300 * time.Millisecond)

	if r.InstanceCount("svc") != 0 {
		t.Errorf("expected 0 instances after heartbeat timeout, got %d", r.InstanceCount("svc"))
	}
}

func TestHeartbeatKeepsAlive(t *testing.T) {
	cfg := RegistryConfig{
		HeartbeatTTL:  200 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
	}
	r := NewRegistry(cfg)
	r.Start()
	defer r.Stop()

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
	r.Register(inst)

	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for i := 0; i < 6; i++ {
			<-ticker.C
			r.Heartbeat("svc", "inst-1", HealthStatus{CPUUsage: float64(i)})
		}
	}()

	time.Sleep(350 * time.Millisecond)

	if r.InstanceCount("svc") != 1 {
		t.Errorf("expected 1 instance with active heartbeat, got %d", r.InstanceCount("svc"))
	}
}

func TestExpiryNotifiesSubscribers(t *testing.T) {
	cfg := RegistryConfig{
		HeartbeatTTL:  100 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
	}
	r := NewRegistry(cfg)
	r.Start()
	defer r.Stop()

	var receivedAction string
	var eventMu sync.Mutex

	r.Subscribe("svc", func(event ServiceChangeEvent) {
		eventMu.Lock()
		receivedAction = event.Action
		eventMu.Unlock()
	})

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
	r.Register(inst)

	eventMu.Lock()
	receivedAction = ""
	eventMu.Unlock()

	time.Sleep(300 * time.Millisecond)

	eventMu.Lock()
	if receivedAction != "expire" {
		t.Errorf("expected action expire, got %s", receivedAction)
	}
	eventMu.Unlock()
}

func TestRoundRobinLB(t *testing.T) {
	lb := NewRoundRobinLB()

	instances := []*ServiceInstance{
		{ID: "inst-1", ServiceName: "svc", Address: "addr1"},
		{ID: "inst-2", ServiceName: "svc", Address: "addr2"},
		{ID: "inst-3", ServiceName: "svc", Address: "addr3"},
	}

	selected := make(map[string]int)
	for i := 0; i < 9; i++ {
		inst, err := lb.Select(instances)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}
		selected[inst.ID]++
	}

	for _, id := range []string{"inst-1", "inst-2", "inst-3"} {
		if selected[id] != 3 {
			t.Errorf("expected 3 selections for %s, got %d", id, selected[id])
		}
	}
}

func TestRoundRobinLBEmpty(t *testing.T) {
	lb := NewRoundRobinLB()

	_, err := lb.Select([]*ServiceInstance{})
	if err != ErrNoInstances {
		t.Errorf("expected ErrNoInstances, got %v", err)
	}

	_, err = lb.Select(nil)
	if err != ErrNoInstances {
		t.Errorf("expected ErrNoInstances for nil, got %v", err)
	}
}

func TestRoundRobinLBSingleInstance(t *testing.T) {
	lb := NewRoundRobinLB()

	instances := []*ServiceInstance{
		{ID: "inst-1", ServiceName: "svc", Address: "addr1"},
	}

	for i := 0; i < 5; i++ {
		inst, err := lb.Select(instances)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}
		if inst.ID != "inst-1" {
			t.Errorf("expected inst-1, got %s", inst.ID)
		}
	}
}

func TestRandomLB(t *testing.T) {
	lb := NewRandomLB()

	instances := []*ServiceInstance{
		{ID: "inst-1", ServiceName: "svc", Address: "addr1"},
		{ID: "inst-2", ServiceName: "svc", Address: "addr2"},
		{ID: "inst-3", ServiceName: "svc", Address: "addr3"},
	}

	selected := make(map[string]bool)
	for i := 0; i < 100; i++ {
		inst, err := lb.Select(instances)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}
		selected[inst.ID] = true
	}

	if len(selected) < 2 {
		t.Errorf("random LB should select multiple instances over 100 calls, got %d unique", len(selected))
	}
}

func TestRandomLBEmpty(t *testing.T) {
	lb := NewRandomLB()

	_, err := lb.Select([]*ServiceInstance{})
	if err != ErrNoInstances {
		t.Errorf("expected ErrNoInstances, got %v", err)
	}

	_, err = lb.Select(nil)
	if err != ErrNoInstances {
		t.Errorf("expected ErrNoInstances for nil, got %v", err)
	}
}

func TestRandomLBSingleInstance(t *testing.T) {
	lb := NewRandomLB()

	instances := []*ServiceInstance{
		{ID: "inst-1", ServiceName: "svc", Address: "addr1"},
	}

	for i := 0; i < 5; i++ {
		inst, err := lb.Select(instances)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}
		if inst.ID != "inst-1" {
			t.Errorf("expected inst-1, got %s", inst.ID)
		}
	}
}

func TestLoadBalancerInterface(t *testing.T) {
	var lb LoadBalancer

	lb = NewRoundRobinLB()
	instances := []*ServiceInstance{{ID: "a", ServiceName: "svc", Address: "addr"}}
	inst, err := lb.Select(instances)
	if err != nil || inst.ID != "a" {
		t.Errorf("RoundRobinLB should implement LoadBalancer interface")
	}

	lb = NewRandomLB()
	inst, err = lb.Select(instances)
	if err != nil || inst.ID != "a" {
		t.Errorf("RandomLB should implement LoadBalancer interface")
	}
}

func TestRegisterMultipleServices(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	inst1 := &ServiceInstance{ID: "inst-1", ServiceName: "svc-a", Address: "addr1"}
	inst2 := &ServiceInstance{ID: "inst-2", ServiceName: "svc-b", Address: "addr2"}
	inst3 := &ServiceInstance{ID: "inst-3", ServiceName: "svc-a", Address: "addr3"}

	r.Register(inst1)
	r.Register(inst2)
	r.Register(inst3)

	if r.ServiceCount() != 2 {
		t.Errorf("expected 2 services, got %d", r.ServiceCount())
	}
	if r.InstanceCount("svc-a") != 2 {
		t.Errorf("expected 2 instances for svc-a, got %d", r.InstanceCount("svc-a"))
	}
	if r.InstanceCount("svc-b") != 1 {
		t.Errorf("expected 1 instance for svc-b, got %d", r.InstanceCount("svc-b"))
	}
}

func TestDeregisterRemovesEmptyService(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
	r.Register(inst)

	r.Deregister("svc", "inst-1")

	if r.ServiceCount() != 0 {
		t.Errorf("expected 0 services after deregister, got %d", r.ServiceCount())
	}

	_, err := r.GetInstances("svc")
	if err != ErrServiceNotFound {
		t.Errorf("expected ErrServiceNotFound for empty service, got %v", err)
	}
}

func TestRegisterSameIDDifferentServices(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	inst1 := &ServiceInstance{ID: "inst-1", ServiceName: "svc-a", Address: "addr1"}
	inst2 := &ServiceInstance{ID: "inst-1", ServiceName: "svc-b", Address: "addr2"}

	err := r.Register(inst1)
	if err != nil {
		t.Fatalf("Register inst1 failed: %v", err)
	}
	err = r.Register(inst2)
	if err != nil {
		t.Fatalf("Register inst2 should succeed (different service), got %v", err)
	}

	if r.InstanceCount("svc-a") != 1 {
		t.Errorf("expected 1 instance for svc-a, got %d", r.InstanceCount("svc-a"))
	}
	if r.InstanceCount("svc-b") != 1 {
		t.Errorf("expected 1 instance for svc-b, got %d", r.InstanceCount("svc-b"))
	}
}

func TestStopIdempotent(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	r.Stop()
	r.Stop()
}

func TestInstanceCountNonexistent(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	if r.InstanceCount("nonexistent") != 0 {
		t.Errorf("expected 0 for nonexistent service, got %d", r.InstanceCount("nonexistent"))
	}
}

func TestSubscriberCountNonexistent(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	if r.SubscriberCount("nonexistent") != 0 {
		t.Errorf("expected 0 for nonexistent service, got %d", r.SubscriberCount("nonexistent"))
	}
}

func TestExpireOnlyTimedOut(t *testing.T) {
	cfg := RegistryConfig{
		HeartbeatTTL:  200 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
	}
	r := NewRegistry(cfg)
	r.Start()
	defer r.Stop()

	inst1 := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr1"}
	inst2 := &ServiceInstance{ID: "inst-2", ServiceName: "svc", Address: "addr2"}
	r.Register(inst1)
	r.Register(inst2)

	stopBeat := make(chan struct{})
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopBeat:
				return
			case <-ticker.C:
				r.Heartbeat("svc", "inst-2", HealthStatus{CPUUsage: 50.0})
			}
		}
	}()

	time.Sleep(350 * time.Millisecond)
	close(stopBeat)

	if r.InstanceCount("svc") != 1 {
		t.Errorf("expected 1 instance (inst-2 kept alive), got %d", r.InstanceCount("svc"))
	}

	instances, _ := r.GetInstances("svc")
	if len(instances) != 1 || instances[0].ID != "inst-2" {
		t.Error("expected inst-2 to survive")
	}
}

func TestExpireMultipleServices(t *testing.T) {
	cfg := RegistryConfig{
		HeartbeatTTL:  100 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
	}
	r := NewRegistry(cfg)
	r.Start()
	defer r.Stop()

	inst1 := &ServiceInstance{ID: "inst-1", ServiceName: "svc-a", Address: "addr1"}
	inst2 := &ServiceInstance{ID: "inst-2", ServiceName: "svc-b", Address: "addr2"}
	r.Register(inst1)
	r.Register(inst2)

	time.Sleep(300 * time.Millisecond)

	if r.ServiceCount() != 0 {
		t.Errorf("expected 0 services after all expired, got %d", r.ServiceCount())
	}
}

func TestExpireInstancesWhenStopped(t *testing.T) {
	cfg := RegistryConfig{
		HeartbeatTTL:  50 * time.Millisecond,
		CheckInterval: 10 * time.Millisecond,
	}
	r := NewRegistry(cfg)
	r.Start()

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
	r.Register(inst)

	r.Stop()

	r.expireInstances()

	if r.InstanceCount("svc") != 1 {
		t.Errorf("expireInstances should not remove instances after stop")
	}
}

func TestConcurrentRegisterDeregister(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n * 2)

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			inst := &ServiceInstance{
				ID:          "inst-" + string(rune('A'+i%26)),
				ServiceName: "svc",
				Address:     "addr",
			}
			r.Register(inst)
		}(i)
	}

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			inst := &ServiceInstance{
				ID:          "inst-" + string(rune('A'+i%26)),
				ServiceName: "svc",
				Address:     "addr",
			}
			r.Deregister(inst.ServiceName, inst.ID)
		}(i)
	}

	wg.Wait()
}

func TestConcurrentHeartbeat(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
	r.Register(inst)

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			r.Heartbeat("svc", "inst-1", HealthStatus{CPUUsage: float64(i)})
		}(i)
	}

	wg.Wait()
}

func TestSubscribeDifferentServices(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	var eventsA []ServiceChangeEvent
	var eventsB []ServiceChangeEvent
	var mu sync.Mutex

	r.Subscribe("svc-a", func(event ServiceChangeEvent) {
		mu.Lock()
		eventsA = append(eventsA, event)
		mu.Unlock()
	})
	r.Subscribe("svc-b", func(event ServiceChangeEvent) {
		mu.Lock()
		eventsB = append(eventsB, event)
		mu.Unlock()
	})

	instA := &ServiceInstance{ID: "inst-a1", ServiceName: "svc-a", Address: "addr1"}
	instB := &ServiceInstance{ID: "inst-b1", ServiceName: "svc-b", Address: "addr2"}
	r.Register(instA)
	r.Register(instB)

	mu.Lock()
	if len(eventsA) != 1 {
		t.Errorf("expected 1 event for svc-a, got %d", len(eventsA))
	}
	if len(eventsA) > 0 && eventsA[0].ServiceName != "svc-a" {
		t.Errorf("expected service name svc-a, got %s", eventsA[0].ServiceName)
	}
	if len(eventsB) != 1 {
		t.Errorf("expected 1 event for svc-b, got %d", len(eventsB))
	}
	if len(eventsB) > 0 && eventsB[0].ServiceName != "svc-b" {
		t.Errorf("expected service name svc-b, got %s", eventsB[0].ServiceName)
	}
	mu.Unlock()
}

func TestEventInstancesAfterDeregister(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	var lastEvent ServiceChangeEvent
	var eventMu sync.Mutex

	r.Subscribe("svc", func(event ServiceChangeEvent) {
		eventMu.Lock()
		lastEvent = event
		eventMu.Unlock()
	})

	inst1 := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr1"}
	inst2 := &ServiceInstance{ID: "inst-2", ServiceName: "svc", Address: "addr2"}
	r.Register(inst1)
	r.Register(inst2)

	r.Deregister("svc", "inst-1")

	eventMu.Lock()
	if lastEvent.Action != "deregister" {
		t.Errorf("expected action deregister, got %s", lastEvent.Action)
	}
	if len(lastEvent.Instances) != 1 {
		t.Errorf("expected 1 instance remaining in event, got %d", len(lastEvent.Instances))
	}
	if len(lastEvent.Instances) > 0 && lastEvent.Instances[0].ID != "inst-2" {
		t.Errorf("expected remaining instance inst-2, got %s", lastEvent.Instances[0].ID)
	}
	eventMu.Unlock()
}

func TestRoundRobinLBDistribution(t *testing.T) {
	lb := NewRoundRobinLB()

	instances := []*ServiceInstance{
		{ID: "a", ServiceName: "svc", Address: "addr1"},
		{ID: "b", ServiceName: "svc", Address: "addr2"},
		{ID: "c", ServiceName: "svc", Address: "addr3"},
		{ID: "d", ServiceName: "svc", Address: "addr4"},
	}

	order := make([]string, 0, 8)
	for i := 0; i < 8; i++ {
		inst, err := lb.Select(instances)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}
		order = append(order, inst.ID)
	}

	expected := []string{"a", "b", "c", "d", "a", "b", "c", "d"}
	for i, got := range order {
		if got != expected[i] {
			t.Errorf("at position %d, expected %s, got %s", i, expected[i], got)
		}
	}
}

func TestConcurrentRoundRobin(t *testing.T) {
	lb := NewRoundRobinLB()

	instances := []*ServiceInstance{
		{ID: "a", ServiceName: "svc", Address: "addr1"},
		{ID: "b", ServiceName: "svc", Address: "addr2"},
	}

	const n = 1000
	var wg sync.WaitGroup
	wg.Add(n)

	countA := int32(0)
	countB := int32(0)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			inst, err := lb.Select(instances)
			if err != nil {
				t.Errorf("Select failed: %v", err)
				return
			}
			if inst.ID == "a" {
				atomic.AddInt32(&countA, 1)
			} else {
				atomic.AddInt32(&countB, 1)
			}
		}()
	}

	wg.Wait()

	total := atomic.LoadInt32(&countA) + atomic.LoadInt32(&countB)
	if total != n {
		t.Errorf("expected %d total selections, got %d", n, total)
	}
}

func TestSubscribeNoSubscribers(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
	err := r.Register(inst)
	if err != nil {
		t.Fatalf("Register should succeed without subscribers, got %v", err)
	}
}

func TestUnsubscribeLastSubscriber(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	subID, _ := r.Subscribe("svc", func(event ServiceChangeEvent) {})
	r.Unsubscribe("svc", subID)

	if r.SubscriberCount("svc") != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", r.SubscriberCount("svc"))
	}

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
	err := r.Register(inst)
	if err != nil {
		t.Fatalf("Register should succeed after all subscribers removed, got %v", err)
	}
}

func TestExpireWithSubscriberNotification(t *testing.T) {
	cfg := RegistryConfig{
		HeartbeatTTL:  100 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
	}
	r := NewRegistry(cfg)
	r.Start()
	defer r.Stop()

	var events []ServiceChangeEvent
	var mu sync.Mutex

	r.Subscribe("svc", func(event ServiceChangeEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
	r.Register(inst)

	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events (register + expire), got %d", len(events))
	}

	registerFound := false
	expireFound := false
	for _, e := range events {
		if e.Action == "register" {
			registerFound = true
		}
		if e.Action == "expire" {
			expireFound = true
		}
	}
	if !registerFound {
		t.Error("expected register event")
	}
	if !expireFound {
		t.Error("expected expire event")
	}
	mu.Unlock()
}

func TestHealthStatusUpdate(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	inst := &ServiceInstance{
		ID:          "inst-1",
		ServiceName: "svc",
		Address:     "addr",
		Health:      HealthStatus{CPUUsage: 10.0, MemoryUsage: 20.0, RequestSuccessRate: 100.0},
	}
	r.Register(inst)

	for i := 0; i < 5; i++ {
		health := HealthStatus{
			CPUUsage:          float64(10 + i*10),
			MemoryUsage:       float64(20 + i*5),
			RequestSuccessRate: 100.0 - float64(i),
		}
		err := r.Heartbeat("svc", "inst-1", health)
		if err != nil {
			t.Fatalf("Heartbeat %d failed: %v", i, err)
		}
	}

	h, err := r.GetHealth("svc", "inst-1")
	if err != nil {
		t.Fatalf("GetHealth failed: %v", err)
	}
	if h.CPUUsage != 50.0 {
		t.Errorf("expected CPUUsage 50.0, got %f", h.CPUUsage)
	}
	if h.MemoryUsage != 40.0 {
		t.Errorf("expected MemoryUsage 40.0, got %f", h.MemoryUsage)
	}
	if h.RequestSuccessRate != 96.0 {
		t.Errorf("expected RequestSuccessRate 96.0, got %f", h.RequestSuccessRate)
	}
}

func TestEndToEndLifecycle(t *testing.T) {
	cfg := RegistryConfig{
		HeartbeatTTL:  200 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
	}
	r := NewRegistry(cfg)
	r.Start()
	defer r.Stop()

	var events []string
	var mu sync.Mutex

	r.Subscribe("order-service", func(event ServiceChangeEvent) {
		mu.Lock()
		events = append(events, event.Action)
		mu.Unlock()
	})

	inst1 := &ServiceInstance{ID: "order-1", ServiceName: "order-service", Address: "10.0.0.1:8080"}
	inst2 := &ServiceInstance{ID: "order-2", ServiceName: "order-service", Address: "10.0.0.2:8080"}
	r.Register(inst1)
	r.Register(inst2)

	instances, _ := r.GetInstances("order-service")
	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}

	lb := NewRoundRobinLB()
	selected1, _ := lb.Select(instances)
	selected2, _ := lb.Select(instances)
	if selected1.ID == selected2.ID {
		t.Error("round-robin should select different instances")
	}

	for i := 0; i < 5; i++ {
		r.Heartbeat("order-service", "order-1", HealthStatus{
			CPUUsage:          float64(30 + i),
			MemoryUsage:       float64(50 + i),
			RequestSuccessRate: 99.9,
		})
		time.Sleep(50 * time.Millisecond)
	}

	h, _ := r.GetHealth("order-service", "order-1")
	if h.CPUUsage != 34.0 {
		t.Errorf("expected CPUUsage 34.0, got %f", h.CPUUsage)
	}

	r.Deregister("order-service", "order-2")
	if r.InstanceCount("order-service") != 1 {
		t.Errorf("expected 1 instance after deregister, got %d", r.InstanceCount("order-service"))
	}

	time.Sleep(400 * time.Millisecond)

	if r.InstanceCount("order-service") != 0 {
		t.Errorf("expected 0 instances after heartbeat expiry, got %d", r.InstanceCount("order-service"))
	}

	mu.Lock()
	if len(events) < 3 {
		t.Errorf("expected at least 3 events, got %d", len(events))
	}

	registerCount := 0
	for _, a := range events {
		if a == "register" {
			registerCount++
		}
	}
	if registerCount != 2 {
		t.Errorf("expected 2 register events, got %d", registerCount)
	}
	mu.Unlock()
}

func TestZeroHeartbeatTTL(t *testing.T) {
	cfg := RegistryConfig{HeartbeatTTL: 0, CheckInterval: 0}
	r := NewRegistry(cfg)
	defer r.Stop()

	if r.cfg.HeartbeatTTL != defaultHeartbeatTTL {
		t.Errorf("expected default HeartbeatTTL for 0, got %v", r.cfg.HeartbeatTTL)
	}
	if r.cfg.CheckInterval != defaultCheckInterval {
		t.Errorf("expected default CheckInterval for 0, got %v", r.cfg.CheckInterval)
	}
}

func TestStartAlreadyRunning(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	r.Start()
	time.Sleep(50 * time.Millisecond)
	r.Stop()
}

func TestStopWithoutStart(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	r.Stop()
}

func TestMultipleServicesExpiry(t *testing.T) {
	cfg := RegistryConfig{
		HeartbeatTTL:  100 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
	}
	r := NewRegistry(cfg)
	r.Start()
	defer r.Stop()

	instA := &ServiceInstance{ID: "inst-a", ServiceName: "svc-a", Address: "addr1"}
	instB := &ServiceInstance{ID: "inst-b", ServiceName: "svc-b", Address: "addr2"}
	r.Register(instA)
	r.Register(instB)

	time.Sleep(300 * time.Millisecond)

	if r.ServiceCount() != 0 {
		t.Errorf("expected 0 services after all expired, got %d", r.ServiceCount())
	}
}

func TestPartialExpiryWithinService(t *testing.T) {
	cfg := RegistryConfig{
		HeartbeatTTL:  150 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
	}
	r := NewRegistry(cfg)
	r.Start()
	defer r.Stop()

	inst1 := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr1"}
	inst2 := &ServiceInstance{ID: "inst-2", ServiceName: "svc", Address: "addr2"}
	inst3 := &ServiceInstance{ID: "inst-3", ServiceName: "svc", Address: "addr3"}
	r.Register(inst1)
	r.Register(inst2)
	r.Register(inst3)

	stopBeat := make(chan struct{})
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopBeat:
				return
			case <-ticker.C:
				r.Heartbeat("svc", "inst-2", HealthStatus{CPUUsage: 50.0})
			}
		}
	}()

	time.Sleep(350 * time.Millisecond)
	close(stopBeat)

	if r.InstanceCount("svc") != 1 {
		t.Errorf("expected 1 instance (inst-2 kept alive), got %d", r.InstanceCount("svc"))
	}

	instances, _ := r.GetInstances("svc")
	if len(instances) != 1 || instances[0].ID != "inst-2" {
		t.Error("expected inst-2 to be the surviving instance")
	}
}

func TestReRegisterAfterDeregister(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
	r.Register(inst)
	r.Deregister("svc", "inst-1")

	err := r.Register(inst)
	if err != nil {
		t.Fatalf("Register after deregister should succeed, got %v", err)
	}
	if r.InstanceCount("svc") != 1 {
		t.Errorf("expected 1 instance after reregister, got %d", r.InstanceCount("svc"))
	}
}

func TestReRegisterAfterExpiry(t *testing.T) {
	cfg := RegistryConfig{
		HeartbeatTTL:  50 * time.Millisecond,
		CheckInterval: 20 * time.Millisecond,
	}
	r := NewRegistry(cfg)
	r.Start()
	defer r.Stop()

	inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
	r.Register(inst)

	time.Sleep(200 * time.Millisecond)

	if r.InstanceCount("svc") != 0 {
		t.Fatalf("expected 0 instances after expiry, got %d", r.InstanceCount("svc"))
	}

	err := r.Register(inst)
	if err != nil {
		t.Fatalf("Register after expiry should succeed, got %v", err)
	}
	if r.InstanceCount("svc") != 1 {
		t.Errorf("expected 1 instance after reregister, got %d", r.InstanceCount("svc"))
	}
}

func TestSubscribeMultipleEvents(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	var actions []string
	var mu sync.Mutex

	r.Subscribe("svc", func(event ServiceChangeEvent) {
		mu.Lock()
		actions = append(actions, event.Action)
		mu.Unlock()
	})

	inst1 := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr1"}
	inst2 := &ServiceInstance{ID: "inst-2", ServiceName: "svc", Address: "addr2"}

	r.Register(inst1)
	r.Register(inst2)
	r.Deregister("svc", "inst-1")

	mu.Lock()
	expected := []string{"register", "register", "deregister"}
	if len(actions) != len(expected) {
		t.Fatalf("expected %d actions, got %d", len(expected), len(actions))
	}
	for i, got := range actions {
		if got != expected[i] {
			t.Errorf("at position %d, expected %s, got %s", i, expected[i], got)
		}
	}
	mu.Unlock()
}

func TestEventInstanceCount(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	var lastEvent ServiceChangeEvent
	var eventMu sync.Mutex

	r.Subscribe("svc", func(event ServiceChangeEvent) {
		eventMu.Lock()
		lastEvent = event
		eventMu.Unlock()
	})

	inst1 := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr1"}
	inst2 := &ServiceInstance{ID: "inst-2", ServiceName: "svc", Address: "addr2"}

	r.Register(inst1)
	eventMu.Lock()
	if len(lastEvent.Instances) != 1 {
		t.Errorf("after first register, expected 1 instance, got %d", len(lastEvent.Instances))
	}
	eventMu.Unlock()

	r.Register(inst2)
	eventMu.Lock()
	if len(lastEvent.Instances) != 2 {
		t.Errorf("after second register, expected 2 instances, got %d", len(lastEvent.Instances))
	}
	eventMu.Unlock()
}

func TestRandomLBDistribution(t *testing.T) {
	lb := NewRandomLB()

	instances := []*ServiceInstance{
		{ID: "a", ServiceName: "svc", Address: "addr1"},
		{ID: "b", ServiceName: "svc", Address: "addr2"},
	}

	counts := make(map[string]int)
	for i := 0; i < 1000; i++ {
		inst, err := lb.Select(instances)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}
		counts[inst.ID]++
	}

	for _, id := range []string{"a", "b"} {
		if counts[id] < 350 {
			t.Errorf("expected roughly 500 selections for %s, got %d", id, counts[id])
		}
	}
}

func TestConcurrentSubscribeUnsubscribe(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())
	defer r.Stop()

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n * 2)

	subIDs := make([]string, n)

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			id, _ := r.Subscribe("svc", func(event ServiceChangeEvent) {})
			subIDs[i] = id
		}(i)
	}

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			if subIDs[i] != "" {
				r.Unsubscribe("svc", subIDs[i])
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrentStartStop(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())

	const n = 10
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			r.Start()
		}()
	}

	wg.Wait()

	r.Stop()
}

func TestStartStopNoPanic(t *testing.T) {
	for round := 0; round < 3; round++ {
		r := NewRegistry(DefaultRegistryConfig())
		r.Start()
		time.Sleep(10 * time.Millisecond)
		r.Stop()
		r.Start()
		time.Sleep(10 * time.Millisecond)
		r.Stop()
	}
}

func TestStartStopWithExpiryLoop(t *testing.T) {
	cfg := RegistryConfig{
		HeartbeatTTL:  50 * time.Millisecond,
		CheckInterval: 20 * time.Millisecond,
	}
	r := NewRegistry(cfg)
	r.Start()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			r.Stop()
			time.Sleep(10 * time.Millisecond)
			r.Start()
			time.Sleep(10 * time.Millisecond)
		}
	}()
	go func() {
		defer wg.Done()
		inst := &ServiceInstance{ID: "inst-1", ServiceName: "svc", Address: "addr"}
		for i := 0; i < 5; i++ {
			r.Register(inst)
			r.Deregister("svc", "inst-1")
			time.Sleep(20 * time.Millisecond)
		}
	}()

	wg.Wait()

	r.Stop()
}

func TestMultiServiceExpireNotificationConsistency(t *testing.T) {
	cfg := RegistryConfig{
		HeartbeatTTL:  100 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
	}
	r := NewRegistry(cfg)
	r.Start()
	defer r.Stop()

	var events []ServiceChangeEvent
	var mu sync.Mutex

	for _, svcName := range []string{"svc-a", "svc-b", "svc-c"} {
		svc := svcName
		r.Subscribe(svc, func(event ServiceChangeEvent) {
			mu.Lock()
			events = append(events, event)
			mu.Unlock()
		})
	}

	instA1 := &ServiceInstance{ID: "inst-a1", ServiceName: "svc-a", Address: "addr-a1"}
	instA2 := &ServiceInstance{ID: "inst-a2", ServiceName: "svc-a", Address: "addr-a2"}
	instB1 := &ServiceInstance{ID: "inst-b1", ServiceName: "svc-b", Address: "addr-b1"}
	instC1 := &ServiceInstance{ID: "inst-c1", ServiceName: "svc-c", Address: "addr-c1"}
	instC2 := &ServiceInstance{ID: "inst-c2", ServiceName: "svc-c", Address: "addr-c2"}

	r.Register(instA1)
	r.Register(instA2)
	r.Register(instB1)
	r.Register(instC1)
	r.Register(instC2)

	mu.Lock()
	events = nil
	mu.Unlock()

	stopBeat := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopBeat:
				return
			case <-ticker.C:
				inst := &ServiceInstance{ID: "inst-c3", ServiceName: "svc-c", Address: "addr-c3"}
				r.Register(inst)
				r.Deregister("svc-c", "inst-c3")
			}
		}
	}()

	time.Sleep(300 * time.Millisecond)
	close(stopBeat)

	mu.Lock()
	defer mu.Unlock()

	for _, event := range events {
		if event.Action != "expire" {
			continue
		}
		for _, inst := range event.Instances {
			if inst.ID == "inst-c3" {
				t.Errorf("expire notification for %s contains inst-c3 which was never in the expired snapshot",
					event.ServiceName)
			}
		}
	}
}

func TestMultiServiceExpireNotificationAtomic(t *testing.T) {
	cfg := RegistryConfig{
		HeartbeatTTL:  80 * time.Millisecond,
		CheckInterval: 20 * time.Millisecond,
	}
	r := NewRegistry(cfg)

	r.Subscribe("svc-a", func(event ServiceChangeEvent) {})
	r.Subscribe("svc-b", func(event ServiceChangeEvent) {})

	instA := &ServiceInstance{ID: "inst-a1", ServiceName: "svc-a", Address: "addr-a1"}
	instB := &ServiceInstance{ID: "inst-b1", ServiceName: "svc-b", Address: "addr-b1"}
	r.Register(instA)
	r.Register(instB)

	stopCh := make(chan struct{})
	var atomicViolation int32

	go func() {
		for {
			select {
			case <-stopCh:
				return
			default:
				inst := &ServiceInstance{ID: "inst-b-new", ServiceName: "svc-b", Address: "addr-b-new"}
				err := r.Register(inst)
				if err == nil {
					r.Deregister("svc-b", "inst-b-new")
				}
				time.Sleep(time.Microsecond)
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)
	close(stopCh)

	if atomic.LoadInt32(&atomicViolation) != 0 {
		t.Errorf("detected %d atomic violations in multi-service expire notifications", atomicViolation)
	}

	r.Stop()
}

func TestExpireNotificationSnapshot(t *testing.T) {
	cfg := RegistryConfig{
		HeartbeatTTL:  100 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
	}
	r := NewRegistry(cfg)
	r.Start()
	defer r.Stop()

	var lastEvent ServiceChangeEvent
	var hasEvent bool
	var eventMu sync.Mutex

	r.Subscribe("svc-a", func(event ServiceChangeEvent) {
		eventMu.Lock()
		if event.Action == "expire" {
			lastEvent = event
			hasEvent = true
		}
		eventMu.Unlock()
	})

	instA1 := &ServiceInstance{ID: "inst-a1", ServiceName: "svc-a", Address: "addr-a1"}
	instA2 := &ServiceInstance{ID: "inst-a2", ServiceName: "svc-a", Address: "addr-a2"}
	r.Register(instA1)
	r.Register(instA2)

	go func() {
		time.Sleep(120 * time.Millisecond)
		instA3 := &ServiceInstance{ID: "inst-a3", ServiceName: "svc-a", Address: "addr-a3"}
		r.Register(instA3)
	}()

	time.Sleep(300 * time.Millisecond)

	eventMu.Lock()
	defer eventMu.Unlock()

	if !hasEvent {
		t.Fatal("expected at least one expire event")
	}

	for _, inst := range lastEvent.Instances {
		if inst.ID == "inst-a3" {
			t.Error("expire event contains inst-a3 which was registered after the expiry scan started")
		}
	}

	ids := make(map[string]bool)
	for _, inst := range lastEvent.Instances {
		ids[inst.ID] = true
	}

	if ids["inst-a1"] || ids["inst-a2"] {
		t.Error("expire event should not contain expired instances")
	}
}

func TestStartStopRestart(t *testing.T) {
	r := NewRegistry(DefaultRegistryConfig())

	r.Start()
	time.Sleep(10 * time.Millisecond)
	if !r.IsRunning() {
		t.Error("expected registry to be running after Start")
	}

	r.Stop()
	if r.IsRunning() {
		t.Error("expected registry to be stopped after Stop")
	}

	r.Start()
	time.Sleep(10 * time.Millisecond)
	if !r.IsRunning() {
		t.Error("expected registry to be running after restart")
	}

	r.Stop()
}

func TestRegisterDuringExpireNotifications(t *testing.T) {
	cfg := RegistryConfig{
		HeartbeatTTL:  100 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
	}
	r := NewRegistry(cfg)
	r.Start()
	defer r.Stop()

	r.Subscribe("svc-a", func(event ServiceChangeEvent) {
		time.Sleep(10 * time.Millisecond)
	})
	r.Subscribe("svc-b", func(event ServiceChangeEvent) {
		time.Sleep(10 * time.Millisecond)
	})

	for _, svcName := range []string{"svc-a", "svc-b"} {
		for i := 0; i < 5; i++ {
			inst := &ServiceInstance{
				ID:          svcName + "-" + string(rune('0'+i)),
				ServiceName: svcName,
				Address:     "addr-" + svcName + "-" + string(rune('0'+i)),
			}
			r.Register(inst)
		}
	}

	registerDone := make(chan struct{})
	go func() {
		defer close(registerDone)
		for i := 0; i < 20; i++ {
			inst := &ServiceInstance{
				ID:          "new-" + string(rune('0'+i%10)),
				ServiceName: "svc-c",
				Address:     "new-addr",
			}
			r.Register(inst)
			r.Deregister("svc-c", "new-"+string(rune('0'+i%10)))
			time.Sleep(5 * time.Millisecond)
		}
	}()

	time.Sleep(300 * time.Millisecond)

	<-registerDone
}
