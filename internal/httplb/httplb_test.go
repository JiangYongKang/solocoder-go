package httplb

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

func TestNewBackendServer_InvalidWeight(t *testing.T) {
	_, err := NewBackendServer("localhost:8080", 0)
	if err != ErrInvalidWeight {
		t.Errorf("expected ErrInvalidWeight, got %v", err)
	}

	_, err = NewBackendServer("localhost:8080", -1)
	if err != ErrInvalidWeight {
		t.Errorf("expected ErrInvalidWeight, got %v", err)
	}
}

func TestBackendServer_StatusTransitions(t *testing.T) {
	s, err := NewBackendServer("localhost:8080", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !s.IsHealthy() {
		t.Error("expected server to be healthy initially")
	}
	if s.Status() != StatusUp {
		t.Errorf("expected StatusUp, got %v", s.Status())
	}

	s.MarkDraining()
	if s.IsHealthy() {
		t.Error("expected server to be unhealthy after draining")
	}
	if s.Status() != StatusDraining {
		t.Errorf("expected StatusDraining, got %v", s.Status())
	}

	s.MarkDown()
	if s.Status() != StatusDown {
		t.Errorf("expected StatusDown, got %v", s.Status())
	}

	s.MarkUp()
	if !s.IsHealthy() {
		t.Error("expected server to be healthy after MarkUp")
	}
	if s.Status() != StatusUp {
		t.Errorf("expected StatusUp, got %v", s.Status())
	}
}

func TestBackendServer_ConnectionTracking(t *testing.T) {
	s, _ := NewBackendServer("localhost:8080", 1)

	if s.ActiveConn() != 0 {
		t.Errorf("expected 0 active connections, got %d", s.ActiveConn())
	}

	s.IncConn()
	if s.ActiveConn() != 1 {
		t.Errorf("expected 1 active connection, got %d", s.ActiveConn())
	}

	s.IncConn()
	s.IncConn()
	if s.ActiveConn() != 3 {
		t.Errorf("expected 3 active connections, got %d", s.ActiveConn())
	}

	s.DecConn()
	if s.ActiveConn() != 2 {
		t.Errorf("expected 2 active connections, got %d", s.ActiveConn())
	}
}

func TestServerPool_BasicOperations(t *testing.T) {
	sp := NewServerPool()

	if sp.ServerCount() != 0 {
		t.Errorf("expected 0 servers, got %d", sp.ServerCount())
	}

	err := sp.AddServer("server1:8080", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sp.ServerCount() != 1 {
		t.Errorf("expected 1 server, got %d", sp.ServerCount())
	}
	if sp.HealthyCount() != 1 {
		t.Errorf("expected 1 healthy server, got %d", sp.HealthyCount())
	}

	err = sp.AddServer("server1:8080", 1)
	if err != ErrServerExists {
		t.Errorf("expected ErrServerExists, got %v", err)
	}

	err = sp.AddServer("server2:8080", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sp.ServerCount() != 2 {
		t.Errorf("expected 2 servers, got %d", sp.ServerCount())
	}

	s, ok := sp.GetServer("server1:8080")
	if !ok {
		t.Fatal("expected to find server1")
	}
	if s.Address != "server1:8080" {
		t.Errorf("expected address server1:8080, got %s", s.Address)
	}

	_, ok = sp.GetServer("nonexistent:8080")
	if ok {
		t.Error("expected to not find nonexistent server")
	}
}

func TestServerPool_RemoveServer(t *testing.T) {
	sp := NewServerPool()
	sp.AddServer("server1:8080", 1)
	sp.AddServer("server2:8080", 1)
	sp.AddServer("server3:8080", 1)

	err := sp.RemoveServer("server2:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sp.ServerCount() != 2 {
		t.Errorf("expected 2 servers, got %d", sp.ServerCount())
	}

	servers := sp.GetAllServers()
	if len(servers) != 2 {
		t.Errorf("expected 2 servers in list, got %d", len(servers))
	}
	if servers[0].Address != "server1:8080" || servers[1].Address != "server3:8080" {
		t.Errorf("expected order [server1, server3], got [%s, %s]", servers[0].Address, servers[1].Address)
	}

	err = sp.RemoveServer("nonexistent:8080")
	if err != ErrServerNotFound {
		t.Errorf("expected ErrServerNotFound, got %v", err)
	}
}

func TestServerPool_DrainAndRestore(t *testing.T) {
	sp := NewServerPool()
	sp.AddServer("server1:8080", 1)
	sp.AddServer("server2:8080", 1)

	if sp.HealthyCount() != 2 {
		t.Errorf("expected 2 healthy servers, got %d", sp.HealthyCount())
	}

	err := sp.DrainServer("server1:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sp.HealthyCount() != 1 {
		t.Errorf("expected 1 healthy server after drain, got %d", sp.HealthyCount())
	}

	healthy := sp.GetHealthyServers()
	if len(healthy) != 1 || healthy[0].Address != "server2:8080" {
		t.Errorf("expected only server2 to be healthy")
	}

	err = sp.RestoreServer("server1:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sp.HealthyCount() != 2 {
		t.Errorf("expected 2 healthy servers after restore, got %d", sp.HealthyCount())
	}

	err = sp.DrainServer("nonexistent:8080")
	if err != ErrServerNotFound {
		t.Errorf("expected ErrServerNotFound, got %v", err)
	}

	err = sp.RestoreServer("nonexistent:8080")
	if err != ErrServerNotFound {
		t.Errorf("expected ErrServerNotFound, got %v", err)
	}
}

func TestRoundRobin_Basic(t *testing.T) {
	servers := []string{"s1:8080", "s2:8080", "s3:8080"}
	rr, err := NewRoundRobin(servers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	order := make([]string, 6)
	for i := 0; i < 6; i++ {
		s, err := rr.Next("")
		if err != nil {
			t.Fatalf("Next failed: %v", err)
		}
		order[i] = s.Address
		s.DecConn()
	}

	expected := []string{"s1:8080", "s2:8080", "s3:8080", "s1:8080", "s2:8080", "s3:8080"}
	for i := range expected {
		if order[i] != expected[i] {
			t.Errorf("index %d: expected %s, got %s", i, expected[i], order[i])
		}
	}
}

func TestRoundRobin_EmptyServers(t *testing.T) {
	rr, err := NewRoundRobin([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = rr.Next("")
	if err != ErrNoHealthyServer {
		t.Errorf("expected ErrNoHealthyServer, got %v", err)
	}
}

func TestRoundRobin_DrainedServer(t *testing.T) {
	servers := []string{"s1:8080", "s2:8080"}
	rr, err := NewRoundRobin(servers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rr.DrainServer("s1:8080")

	for i := 0; i < 5; i++ {
		s, err := rr.Next("")
		if err != nil {
			t.Fatalf("Next failed: %v", err)
		}
		if s.Address != "s2:8080" {
			t.Errorf("expected s2:8080, got %s", s.Address)
		}
		s.DecConn()
	}
}

func TestRoundRobin_AddRemoveServer(t *testing.T) {
	rr, err := NewRoundRobin([]string{"s1:8080"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = rr.AddServer("s2:8080", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rr.ServerCount() != 2 {
		t.Errorf("expected 2 servers, got %d", rr.ServerCount())
	}

	order := make([]string, 4)
	for i := 0; i < 4; i++ {
		s, _ := rr.Next("")
		order[i] = s.Address
		s.DecConn()
	}
	if order[0] != "s1:8080" || order[1] != "s2:8080" || order[2] != "s1:8080" || order[3] != "s2:8080" {
		t.Errorf("expected round-robin between s1 and s2, got %v", order)
	}

	err = rr.RemoveServer("s1:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rr.ServerCount() != 1 {
		t.Errorf("expected 1 server, got %d", rr.ServerCount())
	}

	s, _ := rr.Next("")
	if s.Address != "s2:8080" {
		t.Errorf("expected only s2:8080, got %s", s.Address)
	}
	s.DecConn()
}

func TestLeastConnections_Basic(t *testing.T) {
	servers := []string{"s1:8080", "s2:8080", "s3:8080"}
	lc, err := NewLeastConnections(servers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s1, err := lc.Next("")
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}
	if s1.Address != "s1:8080" {
		t.Errorf("expected s1:8080 first (tie-break by order), got %s", s1.Address)
	}

	s2, _ := lc.Next("")
	if s2.Address != "s2:8080" {
		t.Errorf("expected s2:8080 (least connections, tie-break by order), got %s", s2.Address)
	}

	s1.DecConn()

	s3, _ := lc.Next("")
	if s3.Address != "s1:8080" {
		t.Errorf("expected s1:8080 (now has 0, least), got %s", s3.Address)
	}
	s3.DecConn()
	s2.DecConn()
}

func TestLeastConnections_AllSame(t *testing.T) {
	servers := []string{"s1:8080", "s2:8080"}
	lc, _ := NewLeastConnections(servers)

	s1, _ := lc.Next("")
	s2, _ := lc.Next("")

	if s1.Address != "s1:8080" {
		t.Errorf("expected first to be s1:8080, got %s", s1.Address)
	}
	if s2.Address != "s2:8080" {
		t.Errorf("expected second to be s2:8080, got %s", s2.Address)
	}

	s1.DecConn()
	s2.DecConn()
}

func TestLeastConnections_EmptyServers(t *testing.T) {
	lc, err := NewLeastConnections([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = lc.Next("")
	if err != ErrNoHealthyServer {
		t.Errorf("expected ErrNoHealthyServer, got %v", err)
	}
}

func TestLeastConnections_DrainedServer(t *testing.T) {
	servers := []string{"s1:8080", "s2:8080"}
	lc, _ := NewLeastConnections(servers)

	lc.DrainServer("s1:8080")

	for i := 0; i < 5; i++ {
		s, err := lc.Next("")
		if err != nil {
			t.Fatalf("Next failed: %v", err)
		}
		if s.Address != "s2:8080" {
			t.Errorf("expected s2:8080, got %s", s.Address)
		}
		s.DecConn()
	}
}

func TestWeightedRoundRobin_Basic(t *testing.T) {
	servers := []string{"s1:8080", "s2:8080", "s3:8080"}
	weights := []int{3, 1, 1}
	wrr, err := NewWeightedRoundRobin(servers, weights)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	counts := make(map[string]int)
	totalRequests := 500
	for i := 0; i < totalRequests; i++ {
		s, err := wrr.Next("")
		if err != nil {
			t.Fatalf("Next failed: %v", err)
		}
		counts[s.Address]++
		s.DecConn()
	}

	ratio1 := float64(counts["s1:8080"]) / float64(totalRequests)
	ratio2 := float64(counts["s2:8080"]) / float64(totalRequests)
	ratio3 := float64(counts["s3:8080"]) / float64(totalRequests)

	expectedRatio1 := 3.0 / 5.0
	expectedRatio2 := 1.0 / 5.0
	expectedRatio3 := 1.0 / 5.0

	tolerance := 0.05
	if abs(ratio1-expectedRatio1) > tolerance {
		t.Errorf("s1 ratio: expected ~%.2f, got %.2f", expectedRatio1, ratio1)
	}
	if abs(ratio2-expectedRatio2) > tolerance {
		t.Errorf("s2 ratio: expected ~%.2f, got %.2f", expectedRatio2, ratio2)
	}
	if abs(ratio3-expectedRatio3) > tolerance {
		t.Errorf("s3 ratio: expected ~%.2f, got %.2f", expectedRatio3, ratio3)
	}
}

func TestWeightedRoundRobin_SmoothDistribution(t *testing.T) {
	servers := []string{"a:8080", "b:8080"}
	weights := []int{3, 1}
	wrr, _ := NewWeightedRoundRobin(servers, weights)

	sequence := make([]string, 4)
	for i := 0; i < 4; i++ {
		s, _ := wrr.Next("")
		sequence[i] = s.Address
		s.DecConn()
	}

	aCount := 0
	bCount := 0
	for _, s := range sequence {
		if s == "a:8080" {
			aCount++
		} else if s == "b:8080" {
			bCount++
		}
	}

	if aCount != 3 {
		t.Errorf("expected 3 a's, got %d", aCount)
	}
	if bCount != 1 {
		t.Errorf("expected 1 b, got %d", bCount)
	}
}

func TestWeightedRoundRobin_EqualWeights(t *testing.T) {
	servers := []string{"s1:8080", "s2:8080", "s3:8080"}
	weights := []int{1, 1, 1}
	wrr, err := NewWeightedRoundRobin(servers, weights)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	counts := make(map[string]int)
	for i := 0; i < 300; i++ {
		s, _ := wrr.Next("")
		counts[s.Address]++
		s.DecConn()
	}

	for _, s := range servers {
		if abs(float64(counts[s]-100)) > 10 {
			t.Errorf("expected ~100 requests for %s, got %d", s, counts[s])
		}
	}
}

func TestWeightedRoundRobin_MismatchedLengths(t *testing.T) {
	servers := []string{"s1:8080", "s2:8080"}
	weights := []int{1}
	_, err := NewWeightedRoundRobin(servers, weights)
	if err == nil {
		t.Error("expected error for mismatched server/weight lengths")
	}
}

func TestWeightedRoundRobin_EmptyServers(t *testing.T) {
	wrr, err := NewWeightedRoundRobin([]string{}, []int{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = wrr.Next("")
	if err != ErrNoHealthyServer {
		t.Errorf("expected ErrNoHealthyServer, got %v", err)
	}
}

func TestWeightedRoundRobin_AddRemoveServer(t *testing.T) {
	wrr, err := NewWeightedRoundRobin([]string{"s1:8080"}, []int{1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = wrr.AddServer("s2:8080", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wrr.ServerCount() != 2 {
		t.Errorf("expected 2 servers, got %d", wrr.ServerCount())
	}

	counts := make(map[string]int)
	for i := 0; i < 300; i++ {
		s, _ := wrr.Next("")
		counts[s.Address]++
		s.DecConn()
	}

	ratioS1 := float64(counts["s1:8080"]) / 300.0
	ratioS2 := float64(counts["s2:8080"]) / 300.0

	if abs(ratioS1-1.0/3.0) > 0.05 {
		t.Errorf("s1 ratio: expected ~%.2f, got %.2f", 1.0/3.0, ratioS1)
	}
	if abs(ratioS2-2.0/3.0) > 0.05 {
		t.Errorf("s2 ratio: expected ~%.2f, got %.2f", 2.0/3.0, ratioS2)
	}

	err = wrr.RemoveServer("s1:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wrr.ServerCount() != 1 {
		t.Errorf("expected 1 server, got %d", wrr.ServerCount())
	}

	for i := 0; i < 10; i++ {
		s, _ := wrr.Next("")
		if s.Address != "s2:8080" {
			t.Errorf("expected only s2:8080, got %s", s.Address)
		}
		s.DecConn()
	}
}

func TestConsistentHash_Basic(t *testing.T) {
	servers := []string{"s1:8080", "s2:8080", "s3:8080"}
	ch, err := NewConsistentHash(servers, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	key := "/api/v1/users"
	s1, err := ch.Next(key)
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}
	s1.DecConn()

	s2, err := ch.Next(key)
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}
	s2.DecConn()

	if s1.Address != s2.Address {
		t.Error("same key should map to same server")
	}
}

func TestConsistentHash_DifferentKeys(t *testing.T) {
	servers := []string{"s1:8080", "s2:8080", "s3:8080", "s4:8080", "s5:8080"}
	ch, _ := NewConsistentHash(servers, 100)

	counts := make(map[string]int)
	totalKeys := 1000
	for i := 0; i < totalKeys; i++ {
		key := "/resource/" + string(rune('a'+i%26)) + "/" + string(rune('0'+i%10))
		s, err := ch.Next(key)
		if err != nil {
			t.Fatalf("Next failed: %v", err)
		}
		counts[s.Address]++
		s.DecConn()
	}

	if len(counts) < 2 {
		t.Errorf("expected distribution across at least 2 servers, got %d", len(counts))
	}
}

func TestConsistentHash_AddServerMinimalImpact(t *testing.T) {
	servers := []string{"s1:8080", "s2:8080", "s3:8080"}
	ch, _ := NewConsistentHash(servers, 100)

	mapping := make(map[string]string)
	numKeys := 1000
	for i := 0; i < numKeys; i++ {
		key := "key_" + string(rune('a'+i%26)) + "_" + string(rune('0'+i%10)) + "_" + string(rune('A'+i%26))
		s, _ := ch.Next(key)
		mapping[key] = s.Address
		s.DecConn()
	}

	ch.AddServer("s4:8080", 1)

	changed := 0
	for i := 0; i < numKeys; i++ {
		key := "key_" + string(rune('a'+i%26)) + "_" + string(rune('0'+i%10)) + "_" + string(rune('A'+i%26))
		s, _ := ch.Next(key)
		if mapping[key] != s.Address {
			changed++
		}
		s.DecConn()
	}

	impactRatio := float64(changed) / float64(numKeys)
	if impactRatio > 0.4 {
		t.Errorf("adding 1 server to 3 should affect ~25%% of keys, got %.2f%%", impactRatio*100)
	}
}

func TestConsistentHash_RemoveServerMinimalImpact(t *testing.T) {
	servers := []string{"s1:8080", "s2:8080", "s3:8080", "s4:8080"}
	ch, _ := NewConsistentHash(servers, 100)

	mapping := make(map[string]string)
	numKeys := 1000
	for i := 0; i < numKeys; i++ {
		key := "key_" + string(rune('a'+i%26)) + "_" + string(rune('0'+i%10))
		s, _ := ch.Next(key)
		mapping[key] = s.Address
		s.DecConn()
	}

	ch.RemoveServer("s3:8080")

	changed := 0
	for i := 0; i < numKeys; i++ {
		key := "key_" + string(rune('a'+i%26)) + "_" + string(rune('0'+i%10))
		s, _ := ch.Next(key)
		if mapping[key] != s.Address {
			changed++
		}
		s.DecConn()
	}

	impactRatio := float64(changed) / float64(numKeys)
	if impactRatio > 0.4 {
		t.Errorf("removing 1 server from 4 should affect ~25%% of keys, got %.2f%%", impactRatio*100)
	}
}

func TestConsistentHash_EmptyServers(t *testing.T) {
	ch, err := NewConsistentHash([]string{}, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = ch.Next("test")
	if err != ErrNoHealthyServer {
		t.Errorf("expected ErrNoHealthyServer, got %v", err)
	}
}

func TestConsistentHash_DrainedServer(t *testing.T) {
	servers := []string{"s1:8080", "s2:8080"}
	ch, _ := NewConsistentHash(servers, 50)

	ch.DrainServer("s1:8080")

	counts := make(map[string]int)
	for i := 0; i < 100; i++ {
		key := "key_" + string(rune('a'+i%26))
		s, err := ch.Next(key)
		if err != nil && err != ErrNoHealthyServer {
			t.Fatalf("unexpected error: %v", err)
		}
		if s != nil {
			counts[s.Address]++
			s.DecConn()
		}
	}

	_, hasS1 := counts["s1:8080"]
	if hasS1 {
		t.Error("drained server should not receive requests")
	}

	_, hasS2 := counts["s2:8080"]
	if !hasS2 {
		t.Error("s2 should receive all requests")
	}
}

func TestConsistentHash_DefaultVirtualNodes(t *testing.T) {
	ch, err := NewConsistentHash([]string{"s1:8080"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s, err := ch.Next("test")
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}
	if s.Address != "s1:8080" {
		t.Errorf("expected s1:8080, got %s", s.Address)
	}
	s.DecConn()
}

func TestHTTPLoadBalancer_New(t *testing.T) {
	tests := []struct {
		name      string
		algorithm Algorithm
		wantErr   bool
	}{
		{"round_robin", AlgorithmRoundRobin, false},
		{"least_connections", AlgorithmLeastConnections, false},
		{"weighted_round_robin", AlgorithmWeightedRR, false},
		{"consistent_hash", AlgorithmConsistentHash, false},
		{"unknown", Algorithm("unknown"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Algorithm: tt.algorithm,
				Servers:   []string{"s1:8080", "s2:8080"},
				Weights:   []int{1, 1},
			}
			_, err := NewHTTPLoadBalancer(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewHTTPLoadBalancer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHTTPLoadBalancer_ServeHTTP(t *testing.T) {
	cfg := Config{
		Algorithm: AlgorithmRoundRobin,
		Servers:   []string{"backend1:8080", "backend2:8080"},
	}
	lb, err := NewHTTPLoadBalancer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	lb.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	backend := w.Header().Get("X-Backend-Server")
	if backend == "" {
		t.Error("expected X-Backend-Server header")
	}
}

func TestHTTPLoadBalancer_CustomHashKey(t *testing.T) {
	cfg := Config{
		Algorithm:    AlgorithmConsistentHash,
		Servers:      []string{"s1:8080", "s2:8080"},
		VirtualNodes: 50,
		HashKeyFunc: func(r *http.Request) string {
			return r.RemoteAddr
		},
	}
	lb, err := NewHTTPLoadBalancer(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req1 := httptest.NewRequest("GET", "/path1", nil)
	req1.RemoteAddr = "192.168.1.1:12345"

	req2 := httptest.NewRequest("GET", "/path2", nil)
	req2.RemoteAddr = "192.168.1.1:54321"

	s1, _ := lb.NextServer(req1)
	s2, _ := lb.NextServer(req2)

	if s1.Address != s2.Address {
		t.Error("same client IP should map to same server")
	}
	s1.DecConn()
	s2.DecConn()
}

func TestHTTPLoadBalancer_NoHealthyServers(t *testing.T) {
	cfg := Config{
		Algorithm: AlgorithmRoundRobin,
		Servers:   []string{"s1:8080"},
	}
	lb, _ := NewHTTPLoadBalancer(cfg)

	lb.DrainServer("s1:8080")

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	lb.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHTTPLoadBalancer_DynamicManagement(t *testing.T) {
	cfg := Config{
		Algorithm: AlgorithmRoundRobin,
		Servers:   []string{"s1:8080"},
	}
	lb, _ := NewHTTPLoadBalancer(cfg)

	if lb.ServerCount() != 1 {
		t.Errorf("expected 1 server, got %d", lb.ServerCount())
	}

	err := lb.AddServer("s2:8080", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lb.ServerCount() != 2 {
		t.Errorf("expected 2 servers, got %d", lb.ServerCount())
	}

	err = lb.DrainServer("s1:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lb.HealthyCount() != 1 {
		t.Errorf("expected 1 healthy server, got %d", lb.HealthyCount())
	}

	err = lb.RestoreServer("s1:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lb.HealthyCount() != 2 {
		t.Errorf("expected 2 healthy servers, got %d", lb.HealthyCount())
	}

	err = lb.RemoveServer("s2:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lb.ServerCount() != 1 {
		t.Errorf("expected 1 server, got %d", lb.ServerCount())
	}
}

func TestConcurrent_RoundRobin(t *testing.T) {
	servers := []string{"s1:8080", "s2:8080", "s3:8080"}
	rr, _ := NewRoundRobin(servers)

	var wg sync.WaitGroup
	numGoroutines := 20
	iterations := 100

	var totalRequests int64

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				s, err := rr.Next("")
				if err != nil {
					t.Errorf("Next failed: %v", err)
					return
				}
				atomic.AddInt64(&totalRequests, 1)
				s.DecConn()
			}
		}()
	}

	wg.Wait()

	expected := int64(numGoroutines * iterations)
	if totalRequests != expected {
		t.Errorf("expected %d requests, got %d", expected, totalRequests)
	}

	totalActive := int64(0)
	for _, s := range rr.Servers() {
		totalActive += s.ActiveConn()
	}
	if totalActive != 0 {
		t.Errorf("expected 0 active connections after all done, got %d", totalActive)
	}
}

func TestConcurrent_LeastConnections(t *testing.T) {
	servers := []string{"s1:8080", "s2:8080", "s3:8080"}
	lc, _ := NewLeastConnections(servers)

	var wg sync.WaitGroup
	numGoroutines := 20
	iterations := 100

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				s, err := lc.Next("")
				if err != nil {
					t.Errorf("Next failed: %v", err)
					return
				}
				s.DecConn()
			}
		}()
	}

	wg.Wait()

	counts := make(map[string]int64)
	for _, s := range lc.Servers() {
		counts[s.Address] = s.ActiveConn()
	}

	totalActive := int64(0)
	for _, c := range counts {
		totalActive += c
	}
	if totalActive != 0 {
		t.Errorf("expected 0 active connections, got %d", totalActive)
	}
}

func TestConcurrent_ConsistentHash(t *testing.T) {
	servers := []string{"s1:8080", "s2:8080", "s3:8080"}
	ch, _ := NewConsistentHash(servers, 50)

	var wg sync.WaitGroup
	numGoroutines := 20
	iterations := 100

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				key := "key_" + string(rune('a'+(i%26))) + "_" + string(rune('0'+id))
				s, err := ch.Next(key)
				if err != nil {
					t.Errorf("Next failed: %v", err)
					return
				}
				s.DecConn()
			}
		}(g)
	}

	wg.Wait()

	totalActive := int64(0)
	for _, s := range ch.Servers() {
		totalActive += s.ActiveConn()
	}
	if totalActive != 0 {
		t.Errorf("expected 0 active connections, got %d", totalActive)
	}
}

func TestConcurrent_AddRemoveServers(t *testing.T) {
	servers := []string{"s1:8080", "s2:8080", "s3:8080"}
	rr, _ := NewRoundRobin(servers)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		counter := 0
		for {
			select {
			case <-stop:
				return
			default:
				s, err := rr.Next("")
				if err == nil {
					s.DecConn()
				}
				counter++
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			rr.AddServer("dynamic_"+string(rune('a'+i%26))+":8080", 1)
			rr.RemoveServer("dynamic_"+string(rune('a'+i%26))+":8080")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			rr.DrainServer("s1:8080")
			rr.RestoreServer("s1:8080")
		}
	}()

	for i := 0; i < 50; i++ {
		rr.AddServer("dyn_"+string(rune('A'+i%26))+":8080", 1)
		rr.RemoveServer("dyn_"+string(rune('A'+i%26))+":8080")
	}

	close(stop)
	wg.Wait()
}

func TestBalancer_InterfaceCompliance(t *testing.T) {
	servers := []string{"s1:8080", "s2:8080"}
	weights := []int{1, 1}

	var _ Balancer

	rr, _ := NewRoundRobin(servers)
	var _ Balancer = rr

	lc, _ := NewLeastConnections(servers)
	var _ Balancer = lc

	wrr, _ := NewWeightedRoundRobin(servers, weights)
	var _ Balancer = wrr

	ch, _ := NewConsistentHash(servers, 50)
	var _ Balancer = ch
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
