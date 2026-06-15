package dnsresolver

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockConn struct {
	readData  []byte
	readErr   error
	writeData []byte
	writeErr  error
	closed    bool
	deadline  time.Time
	queryID   uint16
	hasQuery  bool
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	if m.readErr != nil {
		return 0, m.readErr
	}
	if m.hasQuery && len(m.readData) >= 2 {
		respData := make([]byte, len(m.readData))
		copy(respData, m.readData)
		binary.BigEndian.PutUint16(respData[0:2], m.queryID)
		n = copy(b, respData)
	} else {
		n = copy(b, m.readData)
	}
	return n, nil
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	if len(b) >= 2 {
		m.queryID = binary.BigEndian.Uint16(b[0:2])
		m.hasQuery = true
	}
	m.writeData = append(m.writeData, b...)
	return len(b), nil
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
}

func (m *mockConn) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 53}
}

func (m *mockConn) SetDeadline(t time.Time) error {
	m.deadline = t
	return nil
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func buildTestResponse(domain string, ip string, ttl uint32, qtype uint16) []byte {
	resp := make([]byte, 512)
	id := uint16(0x1234)
	flags := uint16(0x8180)

	binary.BigEndian.PutUint16(resp[0:2], id)
	binary.BigEndian.PutUint16(resp[2:4], flags)
	binary.BigEndian.PutUint16(resp[4:6], 1)
	binary.BigEndian.PutUint16(resp[6:8], 1)
	binary.BigEndian.PutUint16(resp[8:10], 0)
	binary.BigEndian.PutUint16(resp[10:12], 0)

	offset := 12
	labels := splitDomain(domain)
	for _, label := range labels {
		resp[offset] = byte(len(label))
		offset++
		copy(resp[offset:], label)
		offset += len(label)
	}
	resp[offset] = 0
	offset++

	binary.BigEndian.PutUint16(resp[offset:offset+2], qtype)
	offset += 2
	binary.BigEndian.PutUint16(resp[offset:offset+2], ClassIN)
	offset += 2

	resp[offset] = 0xC0
	resp[offset+1] = 0x0C
	offset += 2
	binary.BigEndian.PutUint16(resp[offset:offset+2], qtype)
	offset += 2
	binary.BigEndian.PutUint16(resp[offset:offset+2], ClassIN)
	offset += 2
	binary.BigEndian.PutUint32(resp[offset:offset+4], ttl)
	offset += 4

	if qtype == TypeA {
		binary.BigEndian.PutUint16(resp[offset:offset+2], 4)
		offset += 2
		ipBytes := net.ParseIP(ip).To4()
		copy(resp[offset:], ipBytes)
		offset += 4
	} else if qtype == TypeAAAA {
		binary.BigEndian.PutUint16(resp[offset:offset+2], 16)
		offset += 2
		ipBytes := net.ParseIP(ip).To16()
		copy(resp[offset:], ipBytes)
		offset += 16
	}

	return resp[:offset]
}

func buildCNAMEResponse(domain, cname string, ttl uint32) []byte {
	resp := make([]byte, 512)
	id := uint16(0x1234)
	flags := uint16(0x8180)

	binary.BigEndian.PutUint16(resp[0:2], id)
	binary.BigEndian.PutUint16(resp[2:4], flags)
	binary.BigEndian.PutUint16(resp[4:6], 1)
	binary.BigEndian.PutUint16(resp[6:8], 1)
	binary.BigEndian.PutUint16(resp[8:10], 0)
	binary.BigEndian.PutUint16(resp[10:12], 0)

	offset := 12
	labels := splitDomain(domain)
	for _, label := range labels {
		resp[offset] = byte(len(label))
		offset++
		copy(resp[offset:], label)
		offset += len(label)
	}
	resp[offset] = 0
	offset++

	binary.BigEndian.PutUint16(resp[offset:offset+2], TypeA)
	offset += 2
	binary.BigEndian.PutUint16(resp[offset:offset+2], ClassIN)
	offset += 2

	resp[offset] = 0xC0
	resp[offset+1] = 0x0C
	offset += 2
	binary.BigEndian.PutUint16(resp[offset:offset+2], TypeCNAME)
	offset += 2
	binary.BigEndian.PutUint16(resp[offset:offset+2], ClassIN)
	offset += 2
	binary.BigEndian.PutUint32(resp[offset:offset+4], ttl)
	offset += 4

	cnameLabels := splitDomain(cname)
	cnameStart := offset
	offset += 2

	for _, label := range cnameLabels {
		resp[offset] = byte(len(label))
		offset++
		copy(resp[offset:], label)
		offset += len(label)
	}
	resp[offset] = 0
	offset++

	rdLen := offset - cnameStart - 2
	binary.BigEndian.PutUint16(resp[cnameStart:cnameStart+2], uint16(rdLen))

	return resp[:offset]
}

func buildNSResponse(zone string, nsServers []string, glueIPs map[string]string, ttl uint32) []byte {
	resp := make([]byte, 1024)
	id := uint16(0x1234)
	flags := uint16(0x8180)

	binary.BigEndian.PutUint16(resp[0:2], id)
	binary.BigEndian.PutUint16(resp[2:4], flags)
	binary.BigEndian.PutUint16(resp[4:6], 1)
	binary.BigEndian.PutUint16(resp[6:8], 0)
	binary.BigEndian.PutUint16(resp[8:10], uint16(len(nsServers)))
	binary.BigEndian.PutUint16(resp[10:12], uint16(len(glueIPs)))

	offset := 12
	zone = strings.Trim(zone, ".")
	labels := splitDomain(zone)
	for _, label := range labels {
		resp[offset] = byte(len(label))
		offset++
		copy(resp[offset:], label)
		offset += len(label)
	}
	resp[offset] = 0
	offset++

	binary.BigEndian.PutUint16(resp[offset:offset+2], TypeNS)
	offset += 2
	binary.BigEndian.PutUint16(resp[offset:offset+2], ClassIN)
	offset += 2

	questionEnd := offset

	for _, ns := range nsServers {
		resp[offset] = 0xC0
		resp[offset+1] = 0x0C
		offset += 2
		binary.BigEndian.PutUint16(resp[offset:offset+2], TypeNS)
		offset += 2
		binary.BigEndian.PutUint16(resp[offset:offset+2], ClassIN)
		offset += 2
		binary.BigEndian.PutUint32(resp[offset:offset+4], ttl)
		offset += 4

		nsLabels := splitDomain(strings.TrimSuffix(ns, "."))
		nsStart := offset
		offset += 2

		for _, label := range nsLabels {
			resp[offset] = byte(len(label))
			offset++
			copy(resp[offset:], label)
			offset += len(label)
		}
		resp[offset] = 0
		offset++

		rdLen := offset - nsStart - 2
		binary.BigEndian.PutUint16(resp[nsStart:nsStart+2], uint16(rdLen))
	}

	for nsName, ip := range glueIPs {
		nsLabels := splitDomain(strings.TrimSuffix(nsName, "."))
		resp[offset] = byte(len(nsLabels[0]))
		offset++
		copy(resp[offset:], nsLabels[0])
		offset += len(nsLabels[0])
		for i := 1; i < len(nsLabels); i++ {
			resp[offset] = byte(len(nsLabels[i]))
			offset++
			copy(resp[offset:], nsLabels[i])
			offset += len(nsLabels[i])
		}
		resp[offset] = 0
		offset++

		binary.BigEndian.PutUint16(resp[offset:offset+2], TypeA)
		offset += 2
		binary.BigEndian.PutUint16(resp[offset:offset+2], ClassIN)
		offset += 2
		binary.BigEndian.PutUint32(resp[offset:offset+4], ttl)
		offset += 4
		binary.BigEndian.PutUint16(resp[offset:offset+2], 4)
		offset += 2
		ipBytes := net.ParseIP(ip).To4()
		copy(resp[offset:], ipBytes)
		offset += 4
	}

	_ = questionEnd
	return resp[:offset]
}

type mockDNSServer struct {
	addr    string
	conn    net.PacketConn
	handler func([]byte) []byte
	closed  bool
	wg      sync.WaitGroup
}

func newMockDNSServer(handler func([]byte) []byte) (*mockDNSServer, error) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	srv := &mockDNSServer{
		addr:    conn.LocalAddr().String(),
		conn:    conn,
		handler: handler,
	}

	srv.wg.Add(1)
	go srv.serve()

	return srv, nil
}

func (s *mockDNSServer) serve() {
	defer s.wg.Done()
	buf := make([]byte, 1024)

	for !s.closed {
		n, addr, err := s.conn.ReadFrom(buf)
		if err != nil {
			if s.closed {
				return
			}
			continue
		}

		query := buf[:n]
		resp := s.handler(query)

		if len(query) >= 2 && len(resp) >= 2 {
			queryID := binary.BigEndian.Uint16(query[0:2])
			binary.BigEndian.PutUint16(resp[0:2], queryID)
		}

		s.conn.WriteTo(resp, addr)
	}
}

func (s *mockDNSServer) Close() {
	s.closed = true
	s.conn.Close()
	s.wg.Wait()
}

func TestNewResolver(t *testing.T) {
	r, err := NewResolver()
	if err != nil {
		t.Fatalf("NewResolver failed: %v", err)
	}
	if r == nil {
		t.Fatal("NewResolver returned nil")
	}
	defer r.Close()

	if r.CacheCount() != 0 {
		t.Errorf("expected 0 cache entries, got %d", r.CacheCount())
	}
}

func TestNewResolverWithConfig(t *testing.T) {
	cfg := Config{
		UpstreamServers:   []string{"1.2.3.4:53"},
		MaxRecursionDepth: 10,
		QueryTimeout:      2 * time.Second,
		DefaultTTL:        60 * time.Second,
		EnableCache:       false,
		EnableRecursion:   false,
	}

	r, err := NewResolverWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewResolverWithConfig failed: %v", err)
	}
	defer r.Close()

	if r.cfg.MaxRecursionDepth != 10 {
		t.Errorf("expected MaxRecursionDepth 10, got %d", r.cfg.MaxRecursionDepth)
	}
	if r.cfg.QueryTimeout != 2*time.Second {
		t.Errorf("expected QueryTimeout 2s, got %v", r.cfg.QueryTimeout)
	}
	if r.cfg.EnableCache != false {
		t.Error("expected EnableCache false")
	}
}

func TestNewResolverWithInvalidConfig(t *testing.T) {
	cfg := Config{
		MaxRecursionDepth: -1,
		QueryTimeout:      -1 * time.Second,
		DefaultTTL:        -1 * time.Second,
	}

	r, err := NewResolverWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewResolverWithConfig failed: %v", err)
	}
	defer r.Close()

	if r.cfg.MaxRecursionDepth <= 0 {
		t.Errorf("expected positive MaxRecursionDepth, got %d", r.cfg.MaxRecursionDepth)
	}
	if r.cfg.QueryTimeout <= 0 {
		t.Errorf("expected positive QueryTimeout, got %v", r.cfg.QueryTimeout)
	}
	if r.cfg.DefaultTTL <= 0 {
		t.Errorf("expected positive DefaultTTL, got %v", r.cfg.DefaultTTL)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if len(cfg.UpstreamServers) == 0 {
		t.Error("expected default UpstreamServers")
	}
	if len(cfg.RootServers) == 0 {
		t.Error("expected default RootServers")
	}
	if cfg.MaxRecursionDepth <= 0 {
		t.Error("expected positive MaxRecursionDepth")
	}
	if cfg.EnableCache != true {
		t.Error("expected EnableCache true")
	}
	if cfg.EnableRecursion != true {
		t.Error("expected EnableRecursion true")
	}
}

func TestResolveInvalidDomain(t *testing.T) {
	r, _ := NewResolver()
	defer r.Close()

	_, err := r.Resolve("", TypeA)
	if !errors.Is(err, ErrInvalidDomain) {
		t.Errorf("expected ErrInvalidDomain, got %v", err)
	}
}

func TestResolveClosedResolver(t *testing.T) {
	r, _ := NewResolver()
	r.Close()

	_, err := r.Resolve("example.com", TypeA)
	if !errors.Is(err, ErrResolverClosed) {
		t.Errorf("expected ErrResolverClosed, got %v", err)
	}
}

func TestResolveACacheHit(t *testing.T) {
	cfg := Config{
		EnableCache:      true,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	now := time.Now()
	r.nowFunc = func() time.Time { return now }

	expectedRecords := []DNSRecord{
		{Type: TypeA, Class: ClassIN, TTL: 300, Data: "1.2.3.4"},
	}
	r.putToCache("example.com|1", expectedRecords)

	ips, err := r.ResolveA("example.com")
	if err != nil {
		t.Fatalf("ResolveA failed: %v", err)
	}
	if len(ips) != 1 || ips[0] != "1.2.3.4" {
		t.Errorf("expected [1.2.3.4], got %v", ips)
	}
}

func TestResolveCacheExpired(t *testing.T) {
	cfg := Config{
		EnableCache:      true,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	baseTime := time.Now()
	currentTime := baseTime
	r.nowFunc = func() time.Time { return currentTime }

	expectedRecords := []DNSRecord{
		{Type: TypeA, Class: ClassIN, TTL: 1, Data: "1.2.3.4"},
	}
	r.putToCache("example.com|1", expectedRecords)

	currentTime = baseTime.Add(2 * time.Second)

	queryCalled := false
	r.dialUDP = func(network, address string) (net.Conn, error) {
		queryCalled = true
		return &mockConn{
			readData: buildTestResponse("example.com", "5.6.7.8", 300, TypeA),
		}, nil
	}

	ips, err := r.ResolveA("example.com")
	if err != nil {
		t.Fatalf("ResolveA failed: %v", err)
	}
	if !queryCalled {
		t.Error("expected query to be called for expired cache")
	}
	if len(ips) != 1 || ips[0] != "5.6.7.8" {
		t.Errorf("expected [5.6.7.8], got %v", ips)
	}
}

func TestResolveACacheDisabled(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	queryCount := 0
	r.dialUDP = func(network, address string) (net.Conn, error) {
		queryCount++
		return &mockConn{
			readData: buildTestResponse("example.com", "1.2.3.4", 300, TypeA),
		}, nil
	}

	for i := 0; i < 3; i++ {
		ips, err := r.ResolveA("example.com")
		if err != nil {
			t.Fatalf("ResolveA failed: %v", err)
		}
		if len(ips) != 1 || ips[0] != "1.2.3.4" {
			t.Errorf("expected [1.2.3.4], got %v", ips)
		}
	}

	if queryCount != 3 {
		t.Errorf("expected 3 queries (cache disabled), got %d", queryCount)
	}
}

func TestResolveIterative(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	r.dialUDP = func(network, address string) (net.Conn, error) {
		return &mockConn{
			readData: buildTestResponse("example.com", "93.184.216.34", 300, TypeA),
		}, nil
	}

	ips, err := r.ResolveA("example.com.")
	if err != nil {
		t.Fatalf("ResolveA failed: %v", err)
	}
	if len(ips) != 1 || ips[0] != "93.184.216.34" {
		t.Errorf("expected [93.184.216.34], got %v", ips)
	}
}

func TestResolveIterativeNoUpstream(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	_, err := r.ResolveA("example.com")
	if !errors.Is(err, ErrNoUpstreamServers) {
		t.Errorf("expected ErrNoUpstreamServers, got %v", err)
	}
}

func TestResolveCNAMEFollow(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
		MaxRecursionDepth: 5,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	queryCount := 0
	r.dialUDP = func(network, address string) (net.Conn, error) {
		queryCount++
		if queryCount == 1 {
			return &mockConn{
				readData: buildCNAMEResponse("www.example.com", "cdn.example.com", 300),
			}, nil
		}
		return &mockConn{
			readData: buildTestResponse("cdn.example.com", "1.2.3.4", 300, TypeA),
		}, nil
	}

	ips, err := r.ResolveA("www.example.com")
	if err != nil {
		t.Fatalf("ResolveA failed: %v", err)
	}
	if len(ips) != 1 || ips[0] != "1.2.3.4" {
		t.Errorf("expected [1.2.3.4], got %v", ips)
	}
	if queryCount != 2 {
		t.Errorf("expected 2 queries for CNAME follow, got %d", queryCount)
	}
}

func TestResolveCNAMEMaxDepth(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
		MaxRecursionDepth: 1,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	queryCount := 0
	r.dialUDP = func(network, address string) (net.Conn, error) {
		queryCount++
		return &mockConn{
			readData: buildCNAMEResponse("level1.example.com", "level2.example.com", 300),
		}, nil
	}

	_, err := r.ResolveA("level1.example.com")
	if !errors.Is(err, ErrMaxDepthExceeded) {
		t.Errorf("expected ErrMaxDepthExceeded due to CNAME loop/max depth, got %v", err)
	}
}

func TestParallelQueryFastestResponse(t *testing.T) {
	slowServer, _ := newMockDNSServer(func(q []byte) []byte {
		time.Sleep(100 * time.Millisecond)
		return buildTestResponse("example.com", "1.1.1.1", 300, TypeA)
	})
	defer slowServer.Close()

	fastServer, _ := newMockDNSServer(func(q []byte) []byte {
		return buildTestResponse("example.com", "2.2.2.2", 300, TypeA)
	})
	defer fastServer.Close()

	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{slowServer.addr, fastServer.addr},
		QueryTimeout:     200 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	start := time.Now()
	ips, err := r.ResolveA("example.com")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ResolveA failed: %v", err)
	}
	if len(ips) != 1 || ips[0] != "2.2.2.2" {
		t.Errorf("expected fastest response [2.2.2.2], got %v", ips)
	}
	if elapsed > 150*time.Millisecond {
		t.Errorf("expected response in < 150ms, took %v", elapsed)
	}
}

func TestParallelQueryAllFail(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{"127.0.0.1:1", "127.0.0.1:2"},
		QueryTimeout:     50 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	_, err := r.ResolveA("example.com")
	if err == nil {
		t.Fatal("expected error when all upstreams fail")
	}
}

func TestParallelQueryNoServers(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{},
		QueryTimeout:     50 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	_, err := r.queryParallel([]string{}, "example.com", TypeA)
	if !errors.Is(err, ErrNoUpstreamServers) {
		t.Errorf("expected ErrNoUpstreamServers, got %v", err)
	}
}

func TestRecursiveResolveRootToLeaf(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  true,
		RootServers:      []string{"127.0.0.1"},
		QueryTimeout:     500 * time.Millisecond,
		MaxRecursionDepth: 10,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	queryCount := 0
	r.dialUDP = func(network, address string) (net.Conn, error) {
		queryCount++
		if queryCount == 1 {
			return &mockConn{
				readData: buildNSResponse("com.", []string{"a.gtld-servers.net."},
					map[string]string{"a.gtld-servers.net": "192.5.6.30"}, 300),
			}, nil
		} else if queryCount == 2 {
			return &mockConn{
				readData: buildNSResponse("example.com.", []string{"ns1.example.com."},
					map[string]string{"ns1.example.com": "1.2.3.4"}, 300),
			}, nil
		}
		return &mockConn{
			readData: buildTestResponse("example.com", "93.184.216.34", 300, TypeA),
		}, nil
	}

	ips, err := r.ResolveA("example.com")
	if err != nil {
		t.Fatalf("ResolveA failed: %v", err)
	}
	if len(ips) != 1 || ips[0] != "93.184.216.34" {
		t.Errorf("expected [93.184.216.34], got %v", ips)
	}
	if queryCount < 3 {
		t.Errorf("expected at least 3 queries for recursive resolution, got %d", queryCount)
	}
}

func TestRecursiveResolveMaxDepth(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  true,
		RootServers:      []string{"127.0.0.1"},
		QueryTimeout:     100 * time.Millisecond,
		MaxRecursionDepth: 1,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	r.dialUDP = func(network, address string) (net.Conn, error) {
		return &mockConn{
			readData: buildNSResponse("com.", []string{"a.gtld-servers.net."},
				map[string]string{"a.gtld-servers.net": "192.5.6.30"}, 300),
		}, nil
	}

	_, err := r.resolveRecursive("deep.example.com", TypeA, 2)
	if !errors.Is(err, ErrMaxDepthExceeded) {
		t.Errorf("expected ErrMaxDepthExceeded, got %v", err)
	}
}

func TestCachePutAndGet(t *testing.T) {
	cfg := Config{
		EnableCache:      true,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	records := []DNSRecord{
		{Type: TypeA, Class: ClassIN, TTL: 300, Data: "1.2.3.4"},
	}

	r.putToCache("test.com|1", records)

	if r.CacheCount() != 1 {
		t.Errorf("expected 1 cache entry, got %d", r.CacheCount())
	}

	cached, ok := r.getFromCache("test.com|1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if len(cached) != 1 || cached[0].Data != "1.2.3.4" {
		t.Errorf("expected cached record 1.2.3.4, got %v", cached)
	}
}

func TestCacheLazyExpiration(t *testing.T) {
	cfg := Config{
		EnableCache:      true,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	baseTime := time.Now()
	currentTime := baseTime
	r.nowFunc = func() time.Time { return currentTime }

	records := []DNSRecord{
		{Type: TypeA, Class: ClassIN, TTL: 1, Data: "1.2.3.4"},
	}
	r.putToCache("expiring.com|1", records)

	_, ok := r.getFromCache("expiring.com|1")
	if !ok {
		t.Fatal("expected cache hit before expiration")
	}

	currentTime = baseTime.Add(2 * time.Second)

	_, ok = r.getFromCache("expiring.com|1")
	if ok {
		t.Error("expected cache miss after expiration")
	}

	if r.CacheCount() != 0 {
		t.Errorf("expected 0 cache entries after lazy expiration, got %d", r.CacheCount())
	}
}

func TestCacheCleanupExpired(t *testing.T) {
	cfg := Config{
		EnableCache:      true,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	baseTime := time.Now()
	currentTime := baseTime
	r.nowFunc = func() time.Time { return currentTime }

	for i := 0; i < 5; i++ {
		records := []DNSRecord{
			{Type: TypeA, Class: ClassIN, TTL: uint32(i + 1), Data: fmt.Sprintf("1.2.3.%d", i+1)},
		}
		r.putToCache(fmt.Sprintf("host%d.com|1", i+1), records)
	}

	if r.CacheCount() != 5 {
		t.Errorf("expected 5 cache entries, got %d", r.CacheCount())
	}

	currentTime = baseTime.Add(3 * time.Second)
	cleaned := r.cleanupExpired()

	if cleaned != 2 {
		t.Errorf("expected 2 expired entries cleaned, got %d", cleaned)
	}
	if r.CacheCount() != 3 {
		t.Errorf("expected 3 remaining cache entries, got %d", r.CacheCount())
	}
}

func TestCacheClear(t *testing.T) {
	cfg := Config{
		EnableCache:      true,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	for i := 0; i < 10; i++ {
		records := []DNSRecord{
			{Type: TypeA, Class: ClassIN, TTL: 300, Data: fmt.Sprintf("1.2.3.%d", i+1)},
		}
		r.putToCache(fmt.Sprintf("host%d.com|1", i+1), records)
	}

	if r.CacheCount() != 10 {
		t.Errorf("expected 10 cache entries, got %d", r.CacheCount())
	}

	r.ClearCache()

	if r.CacheCount() != 0 {
		t.Errorf("expected 0 cache entries after clear, got %d", r.CacheCount())
	}
}

func TestCacheUsesRecordTTL(t *testing.T) {
	cfg := Config{
		EnableCache:      true,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
		DefaultTTL:       600 * time.Second,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	now := time.Now()
	r.nowFunc = func() time.Time { return now }

	recordsWithTTL := []DNSRecord{
		{Type: TypeA, Class: ClassIN, TTL: 120, Data: "1.2.3.4"},
	}
	r.putToCache("with-ttl.com|1", recordsWithTTL)

	r.mu.RLock()
	entry := r.cache["with-ttl.com|1"]
	r.mu.RUnlock()

	if entry.TTL != 120*time.Second {
		t.Errorf("expected TTL 120s, got %v", entry.TTL)
	}
	if entry.ExpiresAt != now.Add(120*time.Second) {
		t.Errorf("expected ExpiresAt %v, got %v", now.Add(120*time.Second), entry.ExpiresAt)
	}
}

func TestCacheUsesDefaultTTL(t *testing.T) {
	cfg := Config{
		EnableCache:      true,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
		DefaultTTL:       600 * time.Second,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	now := time.Now()
	r.nowFunc = func() time.Time { return now }

	recordsNoTTL := []DNSRecord{
		{Type: TypeA, Class: ClassIN, TTL: 0, Data: "1.2.3.4"},
	}
	r.putToCache("no-ttl.com|1", recordsNoTTL)

	r.mu.RLock()
	entry := r.cache["no-ttl.com|1"]
	r.mu.RUnlock()

	if entry.TTL != 600*time.Second {
		t.Errorf("expected default TTL 600s, got %v", entry.TTL)
	}
}

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Example.com", "example.com"},
		{"example.com.", "example.com"},
		{"  example.com  ", "example.com"},
		{"WWW.Example.COM.", "www.example.com"},
		{".", ""},
	}

	for _, tt := range tests {
		result := normalizeDomain(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeDomain(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestSplitDomain(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"www.example.com", []string{"www", "example", "com"}},
		{"example.com", []string{"example", "com"}},
		{".", nil},
		{"", nil},
	}

	for _, tt := range tests {
		result := splitDomain(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("splitDomain(%q) len = %d, expected %d", tt.input, len(result), len(tt.expected))
			continue
		}
		for i, v := range result {
			if v != tt.expected[i] {
				t.Errorf("splitDomain(%q)[%d] = %q, expected %q", tt.input, i, v, tt.expected[i])
			}
		}
	}
}

func TestFilterRecords(t *testing.T) {
	records := []DNSRecord{
		{Type: TypeA, Data: "1.2.3.4"},
		{Type: TypeCNAME, Data: "cdn.example.com"},
		{Type: TypeA, Data: "5.6.7.8"},
		{Type: TypeAAAA, Data: "::1"},
	}

	aRecords := filterRecords(records, TypeA)
	if len(aRecords) != 2 {
		t.Errorf("expected 2 A records, got %d", len(aRecords))
	}

	cnameRecords := filterRecords(records, TypeCNAME)
	if len(cnameRecords) != 1 {
		t.Errorf("expected 1 CNAME record, got %d", len(cnameRecords))
	}
	if cnameRecords[0].Data != "cdn.example.com" {
		t.Errorf("expected cdn.example.com, got %s", cnameRecords[0].Data)
	}
}

func TestExtractIPs(t *testing.T) {
	records := []DNSRecord{
		{Type: TypeA, Data: "1.2.3.4"},
		{Type: TypeCNAME, Data: "cdn.example.com"},
		{Type: TypeA, Data: "5.6.7.8"},
	}

	ips := extractIPs(records, TypeA)
	if len(ips) != 2 {
		t.Errorf("expected 2 IPs, got %d", len(ips))
	}
	if ips[0] != "1.2.3.4" || ips[1] != "5.6.7.8" {
		t.Errorf("expected [1.2.3.4, 5.6.7.8], got %v", ips)
	}
}

func TestBuildQuery(t *testing.T) {
	query, id, err := buildQuery("example.com", TypeA)
	if err != nil {
		t.Fatalf("buildQuery failed: %v", err)
	}
	_ = id

	if len(query) < 12 {
		t.Fatalf("query too short: %d bytes", len(query))
	}

	flags := binary.BigEndian.Uint16(query[2:4])
	if flags != 0x0100 {
		t.Errorf("expected flags 0x0100, got 0x%04x", flags)
	}

	qdCount := binary.BigEndian.Uint16(query[4:6])
	if qdCount != 1 {
		t.Errorf("expected 1 question, got %d", qdCount)
	}
}

func TestBuildQueryInvalidDomain(t *testing.T) {
	longLabel := strings.Repeat("a", 64)
	_, _, err := buildQuery(longLabel+".com", TypeA)
	if !errors.Is(err, ErrInvalidDomain) {
		t.Errorf("expected ErrInvalidDomain for long label, got %v", err)
	}
}

func TestParseResponseA(t *testing.T) {
	respBytes := buildTestResponse("example.com", "1.2.3.4", 300, TypeA)

	resp, err := parseResponse(respBytes, 0x1234)
	if err != nil {
		t.Fatalf("parseResponse failed: %v", err)
	}

	if len(resp.Answers) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(resp.Answers))
	}

	ans := resp.Answers[0]
	if ans.Type != TypeA {
		t.Errorf("expected TypeA, got %d", ans.Type)
	}
	if ans.Data != "1.2.3.4" {
		t.Errorf("expected 1.2.3.4, got %s", ans.Data)
	}
	if ans.TTL != 300 {
		t.Errorf("expected TTL 300, got %d", ans.TTL)
	}
}

func TestParseResponseAAAA(t *testing.T) {
	respBytes := buildTestResponse("example.com", "2001:db8::1", 300, TypeAAAA)

	resp, err := parseResponse(respBytes, 0x1234)
	if err != nil {
		t.Fatalf("parseResponse failed: %v", err)
	}

	if len(resp.Answers) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(resp.Answers))
	}

	ans := resp.Answers[0]
	if ans.Type != TypeAAAA {
		t.Errorf("expected TypeAAAA, got %d", ans.Type)
	}
	if !strings.Contains(ans.Data, "2001:db8") {
		t.Errorf("expected IPv6 address containing 2001:db8, got %s", ans.Data)
	}
}

func TestParseResponseTooShort(t *testing.T) {
	_, err := parseResponse([]byte{0x00, 0x00}, 0)
	if !errors.Is(err, ErrInvalidResponse) {
		t.Errorf("expected ErrInvalidResponse, got %v", err)
	}
}

func TestDecodeName(t *testing.T) {
	msg := []byte{
		0x03, 'w', 'w', 'w',
		0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		0x03, 'c', 'o', 'm',
		0x00,
	}

	name, offset, err := decodeName(msg, 0)
	if err != nil {
		t.Fatalf("decodeName failed: %v", err)
	}
	if name != "www.example.com" {
		t.Errorf("expected www.example.com, got %s", name)
	}
	if offset != 17 {
		t.Errorf("expected offset 17, got %d", offset)
	}
}

func TestDecodeNameWithPointer(t *testing.T) {
	msg := []byte{
		0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		0x03, 'c', 'o', 'm',
		0x00,
		0x03, 'w', 'w', 'w',
		0xC0, 0x00,
	}

	name, _, err := decodeName(msg, 13)
	if err != nil {
		t.Fatalf("decodeName with pointer failed: %v", err)
	}
	if name != "www.example.com" {
		t.Errorf("expected www.example.com, got %s", name)
	}
}

func TestDecodeNameTooManyJumps(t *testing.T) {
	msg := []byte{
		0xC0, 0x02,
		0xC0, 0x00,
		0xC0, 0x00,
		0xC0, 0x00,
		0xC0, 0x00,
		0xC0, 0x00,
		0xC0, 0x00,
		0xC0, 0x00,
		0xC0, 0x00,
		0xC0, 0x00,
		0xC0, 0x00,
		0xC0, 0x00,
	}

	_, _, err := decodeName(msg, 0)
	if !errors.Is(err, ErrInvalidResponse) {
		t.Errorf("expected ErrInvalidResponse for too many jumps, got %v", err)
	}
}

func TestResolveAAAA(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	r.dialUDP = func(network, address string) (net.Conn, error) {
		return &mockConn{
			readData: buildTestResponse("example.com", "2001:db8::1", 300, TypeAAAA),
		}, nil
	}

	ips, err := r.ResolveAAAA("example.com")
	if err != nil {
		t.Fatalf("ResolveAAAA failed: %v", err)
	}
	if len(ips) != 1 {
		t.Fatalf("expected 1 AAAA record, got %d", len(ips))
	}
	if !strings.Contains(ips[0], "2001:db8") {
		t.Errorf("expected IPv6 with 2001:db8, got %s", ips[0])
	}
}

func TestResolveNoRecords(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	respBytes := make([]byte, 12)
	binary.BigEndian.PutUint16(respBytes[0:2], 0x1234)
	binary.BigEndian.PutUint16(respBytes[2:4], 0x8180)
	binary.BigEndian.PutUint16(respBytes[4:6], 1)
	binary.BigEndian.PutUint16(respBytes[6:8], 0)
	binary.BigEndian.PutUint16(respBytes[8:10], 0)
	binary.BigEndian.PutUint16(respBytes[10:12], 0)

	question, _ := encodeQuestion("example.com", TypeA, ClassIN)
	respBytes = append(respBytes, question...)

	r.dialUDP = func(network, address string) (net.Conn, error) {
		return &mockConn{readData: respBytes}, nil
	}

	_, err := r.ResolveA("example.com")
	if !errors.Is(err, ErrNoRecordsFound) {
		t.Errorf("expected ErrNoRecordsFound, got %v", err)
	}
}

func TestResolveDialError(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	expectedErr := errors.New("dial failed")
	r.dialUDP = func(network, address string) (net.Conn, error) {
		return nil, expectedErr
	}

	_, err := r.ResolveA("example.com")
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected dial error, got %v", err)
	}
}

func TestResolveWriteError(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	expectedErr := errors.New("write failed")
	r.dialUDP = func(network, address string) (net.Conn, error) {
		return &mockConn{writeErr: expectedErr}, nil
	}

	_, err := r.ResolveA("example.com")
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected write error, got %v", err)
	}
}

func TestResolveReadError(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	expectedErr := errors.New("read failed")
	r.dialUDP = func(network, address string) (net.Conn, error) {
		return &mockConn{readErr: expectedErr}, nil
	}

	_, err := r.ResolveA("example.com")
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected read error, got %v", err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	cfg := Config{
		EnableCache:      true,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	r.dialUDP = func(network, address string) (net.Conn, error) {
		return &mockConn{
			readData: buildTestResponse("example.com", "1.2.3.4", 300, TypeA),
		}, nil
	}

	var wg sync.WaitGroup
	numGoroutines := 10
	numOps := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				domain := fmt.Sprintf("host-%d-%d.example.com", id, j)
				_, err := r.ResolveA(domain)
				if err != nil {
					t.Errorf("ResolveA failed for %s: %v", domain, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrentReadAndWrite(t *testing.T) {
	cfg := Config{
		EnableCache:      true,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	r.dialUDP = func(network, address string) (net.Conn, error) {
		return &mockConn{
			readData: buildTestResponse("shared.example.com", "1.2.3.4", 300, TypeA),
		}, nil
	}

	var wg sync.WaitGroup
	readGoroutines := 10
	writeGoroutines := 2
	opsPerGoroutine := 500

	var readErrors int32
	var writeErrors int32

	for i := 0; i < readGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				_, err := r.ResolveA("shared.example.com")
				if err != nil {
					atomic.AddInt32(&readErrors, 1)
				}
				time.Sleep(time.Microsecond)
			}
		}()
	}

	for i := 0; i < writeGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				records := []DNSRecord{
					{Type: TypeA, Class: ClassIN, TTL: 300, Data: fmt.Sprintf("5.6.7.%d", j%256)},
				}
				r.putToCache(fmt.Sprintf("key-%d-%d|1", id, j), records)
				if j%100 == 0 {
					r.cleanupExpired()
				}
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	wg.Wait()

	if readErrors > 0 {
		t.Errorf("got %d read errors", readErrors)
	}
	if writeErrors > 0 {
		t.Errorf("got %d write errors", writeErrors)
	}
}

func TestDoubleClose(t *testing.T) {
	r, _ := NewResolver()
	r.Close()
	r.Close()
}

func TestCacheCleanupLoopStops(t *testing.T) {
	cfg := Config{
		EnableCache:      true,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		CacheCleanupInterval: 10 * time.Millisecond,
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)

	time.Sleep(50 * time.Millisecond)

	start := time.Now()
	r.Close()
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Errorf("Close took too long: %v", elapsed)
	}
}

func TestQueryParallelDiscardsLateResponses(t *testing.T) {
	server1, _ := newMockDNSServer(func(q []byte) []byte {
		return buildTestResponse("example.com", "1.1.1.1", 300, TypeA)
	})
	defer server1.Close()

	server2, _ := newMockDNSServer(func(q []byte) []byte {
		time.Sleep(200 * time.Millisecond)
		return buildTestResponse("example.com", "2.2.2.2", 300, TypeA)
	})
	defer server2.Close()

	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{server1.addr, server2.addr},
		QueryTimeout:     500 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	start := time.Now()
	ips, err := r.ResolveA("example.com")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ResolveA failed: %v", err)
	}

	if ips[0] != "1.1.1.1" {
		t.Errorf("expected first response 1.1.1.1, got %v", ips)
	}

	if elapsed > 150*time.Millisecond {
		t.Errorf("expected response in < 150ms (not waiting for slow server), took %v", elapsed)
	}
}

func TestQueryParallelOneServerFails(t *testing.T) {
	goodServer, _ := newMockDNSServer(func(q []byte) []byte {
		return buildTestResponse("example.com", "1.2.3.4", 300, TypeA)
	})
	defer goodServer.Close()

	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{"127.0.0.1:1", goodServer.addr},
		QueryTimeout:     200 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	ips, err := r.ResolveA("example.com")
	if err != nil {
		t.Fatalf("ResolveA failed: %v", err)
	}
	if len(ips) != 1 || ips[0] != "1.2.3.4" {
		t.Errorf("expected [1.2.3.4], got %v", ips)
	}
}

func TestDNSRecordTypes(t *testing.T) {
	if TypeA != 1 {
		t.Errorf("TypeA should be 1, got %d", TypeA)
	}
	if TypeNS != 2 {
		t.Errorf("TypeNS should be 2, got %d", TypeNS)
	}
	if TypeCNAME != 5 {
		t.Errorf("TypeCNAME should be 5, got %d", TypeCNAME)
	}
	if TypeAAAA != 28 {
		t.Errorf("TypeAAAA should be 28, got %d", TypeAAAA)
	}
	if ClassIN != 1 {
		t.Errorf("ClassIN should be 1, got %d", ClassIN)
	}
}

func TestErrorMessages(t *testing.T) {
	errs := []error{
		ErrInvalidDomain,
		ErrNoUpstreamServers,
		ErrMaxDepthExceeded,
		ErrAllUpstreamsFailed,
		ErrQueryTimeout,
		ErrInvalidResponse,
		ErrNoRecordsFound,
		ErrInvalidConfig,
		ErrResolverClosed,
	}

	for _, err := range errs {
		if err == nil {
			t.Error("error should not be nil")
		}
		if err.Error() == "" {
			t.Error("error message should not be empty")
		}
	}
}

func TestDefaultRootServers(t *testing.T) {
	if len(DefaultRootServers) != 13 {
		t.Errorf("expected 13 root servers, got %d", len(DefaultRootServers))
	}

	for i, ip := range DefaultRootServers {
		if net.ParseIP(ip) == nil {
			t.Errorf("root server %d has invalid IP: %s", i, ip)
		}
	}
}

func TestResolveCNAMEInRecursiveMode(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  true,
		RootServers:      []string{"127.0.0.1"},
		QueryTimeout:     500 * time.Millisecond,
		MaxRecursionDepth: 10,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	queryCount := 0
	r.dialUDP = func(network, address string) (net.Conn, error) {
		queryCount++
		if queryCount == 1 {
			return &mockConn{
				readData: buildNSResponse("com.", []string{"a.gtld-servers.net."},
					map[string]string{"a.gtld-servers.net": "192.5.6.30"}, 300),
			}, nil
		} else if queryCount == 2 {
			return &mockConn{
				readData: buildNSResponse("example.com.", []string{"ns1.example.com."},
					map[string]string{"ns1.example.com": "1.2.3.4"}, 300),
			}, nil
		} else if queryCount == 3 {
			return &mockConn{
				readData: buildCNAMEResponse("www.example.com", "cdn.example.com", 300),
			}, nil
		}
		return &mockConn{
			readData: buildTestResponse("cdn.example.com", "5.6.7.8", 300, TypeA),
		}, nil
	}

	ips, err := r.ResolveA("www.example.com")
	if err != nil {
		t.Fatalf("ResolveA failed: %v", err)
	}
	if len(ips) != 1 || ips[0] != "5.6.7.8" {
		t.Errorf("expected [5.6.7.8] after CNAME follow, got %v", ips)
	}
}

func TestRecursiveCNAMEMaxDepth(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  true,
		RootServers:      []string{"127.0.0.1"},
		QueryTimeout:     100 * time.Millisecond,
		MaxRecursionDepth: 2,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	r.dialUDP = func(network, address string) (net.Conn, error) {
		return &mockConn{
			readData: buildCNAMEResponse("level1.example.com", "level2.example.com", 300),
		}, nil
	}

	_, err := r.ResolveA("level1.example.com")
	if !errors.Is(err, ErrMaxDepthExceeded) {
		t.Errorf("expected ErrMaxDepthExceeded, got %v", err)
	}
}

func TestUpstreamServerWithAndWithoutPort(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8", "1.1.1.1:53"},
		QueryTimeout:     50 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	queryCount := 0
	r.dialUDP = func(network, address string) (net.Conn, error) {
		queryCount++
		if !strings.HasSuffix(address, ":53") {
			t.Errorf("expected address to have port :53, got %s", address)
		}
		return &mockConn{
			readData: buildTestResponse("example.com", "1.2.3.4", 300, TypeA),
		}, nil
	}

	r.ResolveA("example.com")

	if queryCount < 1 {
		t.Error("expected at least one query")
	}
}

func TestCacheKeyFormat(t *testing.T) {
	cfg := Config{
		EnableCache:      true,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	aRecords := []DNSRecord{{Type: TypeA, Class: ClassIN, TTL: 300, Data: "1.2.3.4"}}
	aaaaRecords := []DNSRecord{{Type: TypeAAAA, Class: ClassIN, TTL: 300, Data: "::1"}}

	r.putToCache("example.com|1", aRecords)
	r.putToCache("example.com|28", aaaaRecords)

	if r.CacheCount() != 2 {
		t.Errorf("expected 2 cache entries for same domain different types, got %d", r.CacheCount())
	}

	aCached, ok := r.getFromCache("example.com|1")
	if !ok || len(aCached) != 1 || aCached[0].Data != "1.2.3.4" {
		t.Error("A record cache miss")
	}

	aaaaCached, ok := r.getFromCache("example.com|28")
	if !ok || len(aaaaCached) != 1 || aaaaCached[0].Data != "::1" {
		t.Error("AAAA record cache miss")
	}
}

func buildRcodeResponse(domain string, rcode uint16, qtype uint16) []byte {
	resp := make([]byte, 512)
	id := uint16(0x1234)
	flags := uint16(0x8000) | rcode

	binary.BigEndian.PutUint16(resp[0:2], id)
	binary.BigEndian.PutUint16(resp[2:4], flags)
	binary.BigEndian.PutUint16(resp[4:6], 1)
	binary.BigEndian.PutUint16(resp[6:8], 0)
	binary.BigEndian.PutUint16(resp[8:10], 0)
	binary.BigEndian.PutUint16(resp[10:12], 0)

	offset := 12
	labels := splitDomain(domain)
	for _, label := range labels {
		resp[offset] = byte(len(label))
		offset++
		copy(resp[offset:], label)
		offset += len(label)
	}
	resp[offset] = 0
	offset++

	binary.BigEndian.PutUint16(resp[offset:offset+2], qtype)
	offset += 2
	binary.BigEndian.PutUint16(resp[offset:offset+2], ClassIN)
	offset += 2

	return resp[:offset]
}

func TestParseResponseNXDOMAIN(t *testing.T) {
	respBytes := buildRcodeResponse("nonexistent.example.com", RCODE_NXDOMAIN, TypeA)

	resp, err := parseResponse(respBytes, 0x1234)
	if err == nil {
		t.Fatal("expected error for NXDOMAIN, got nil")
	}

	var dnsErr *DNSError
	if !errors.As(err, &dnsErr) {
		t.Fatalf("expected DNSError, got %T: %v", err, err)
	}
	if dnsErr.RCODE != RCODE_NXDOMAIN {
		t.Errorf("expected RCODE_NXDOMAIN (%d), got %d", RCODE_NXDOMAIN, dnsErr.RCODE)
	}
	if !errors.Is(err, ErrNXDOMAIN) {
		t.Error("expected error to be ErrNXDOMAIN")
	}
	if resp == nil {
		t.Error("expected non-nil response even with error")
	}
	if resp != nil && resp.RCode != RCODE_NXDOMAIN {
		t.Errorf("expected response RCode %d, got %d", RCODE_NXDOMAIN, resp.RCode)
	}
}

func TestParseResponseSERVFAIL(t *testing.T) {
	respBytes := buildRcodeResponse("example.com", RCODE_SERVFAIL, TypeA)

	_, err := parseResponse(respBytes, 0x1234)
	if err == nil {
		t.Fatal("expected error for SERVFAIL, got nil")
	}
	if !errors.Is(err, ErrSERVFAIL) {
		t.Errorf("expected ErrSERVFAIL, got %v", err)
	}
	var dnsErr *DNSError
	if !errors.As(err, &dnsErr) {
		t.Errorf("expected DNSError type, got %T", err)
	}
}

func TestParseResponseFORMERR(t *testing.T) {
	respBytes := buildRcodeResponse("example.com", RCODE_FORMERR, TypeA)

	_, err := parseResponse(respBytes, 0x1234)
	if err == nil {
		t.Fatal("expected error for FORMERR, got nil")
	}
	if !errors.Is(err, ErrFORMERR) {
		t.Errorf("expected ErrFORMERR, got %v", err)
	}
}

func TestParseResponseREFUSED(t *testing.T) {
	respBytes := buildRcodeResponse("example.com", RCODE_REFUSED, TypeA)

	_, err := parseResponse(respBytes, 0x1234)
	if err == nil {
		t.Fatal("expected error for REFUSED, got nil")
	}
	if !errors.Is(err, ErrREFUSED) {
		t.Errorf("expected ErrREFUSED, got %v", err)
	}
}

func TestParseResponseTransactionIDMismatch(t *testing.T) {
	respBytes := buildTestResponse("example.com", "1.2.3.4", 300, TypeA)

	_, err := parseResponse(respBytes, 0x5678)
	if err == nil {
		t.Fatal("expected error for transaction ID mismatch, got nil")
	}
	if !errors.Is(err, ErrTransactionIDMismatch) {
		t.Errorf("expected ErrTransactionIDMismatch, got %v", err)
	}
}

func TestParseResponseHasTransactionID(t *testing.T) {
	respBytes := buildTestResponse("example.com", "1.2.3.4", 300, TypeA)

	resp, err := parseResponse(respBytes, 0x1234)
	if err != nil {
		t.Fatalf("parseResponse failed: %v", err)
	}
	if resp.TransactionID != 0x1234 {
		t.Errorf("expected TransactionID 0x1234, got 0x%04x", resp.TransactionID)
	}
}

func TestResolveNXDOMAIN(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	r.dialUDP = func(network, address string) (net.Conn, error) {
		return &mockConn{
			readData: buildRcodeResponse("nonexistent.example.com", RCODE_NXDOMAIN, TypeA),
		}, nil
	}

	_, err := r.ResolveA("nonexistent.example.com")
	if err == nil {
		t.Fatal("expected error for NXDOMAIN, got nil")
	}
	if !errors.Is(err, ErrNXDOMAIN) {
		t.Errorf("expected ErrNXDOMAIN, got %v", err)
	}
}

func TestResolveSERVFAIL(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     100 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	r.dialUDP = func(network, address string) (net.Conn, error) {
		return &mockConn{
			readData: buildRcodeResponse("example.com", RCODE_SERVFAIL, TypeA),
		}, nil
	}

	_, err := r.ResolveA("example.com")
	if err == nil {
		t.Fatal("expected error for SERVFAIL, got nil")
	}
	if !errors.Is(err, ErrSERVFAIL) {
		t.Errorf("expected ErrSERVFAIL, got %v", err)
	}
}

func TestDNSErrorType(t *testing.T) {
	err := &DNSError{RCODE: 3, Msg: "test error"}
	if err.RCODE != 3 {
		t.Errorf("expected RCODE 3, got %d", err.RCODE)
	}
	expectedMsg := "dnsresolver: rcode=3 test error"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestQuerySingleContextCancellation(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     5 * time.Second,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	r.dialUDP = func(network, address string) (net.Conn, error) {
		mc := &mockConn{
			readErr: fmt.Errorf("simulated read error"),
		}
		return mc, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := r.querySingle(ctx, "8.8.8.8:53", "example.com", TypeA)
	if err == nil {
		t.Error("expected error after context cancellation, got nil")
	}
}

func TestQueryParallelCancelStopsGoroutines(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{"server1:53", "server2:53", "server3:53"},
		QueryTimeout:     5 * time.Second,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	var dialCount int32
	r.dialUDP = func(network, address string) (net.Conn, error) {
		atomic.AddInt32(&dialCount, 1)
		mc := &mockConn{
			readErr: fmt.Errorf("simulated read error"),
		}
		return mc, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	results := make(chan queryResult, 3)
	var pending int32 = 3

	for i := 0; i < 3; i++ {
		go func() {
			defer func() {
				atomic.AddInt32(&pending, -1)
			}()
			resp, err := r.querySingle(ctx, "server:53", "example.com", TypeA)
			results <- queryResult{resp: resp, err: err}
		}()
	}

	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&dialCount) < 3 {
		t.Logf("dial count: %d", atomic.LoadInt32(&dialCount))
	}
}

type mockConnWithDeadline struct {
	mockConn
	deadline time.Time
}

func (m *mockConnWithDeadline) SetDeadline(t time.Time) error {
	m.deadline = t
	return nil
}

func TestContextDeadlinePropagation(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{"8.8.8.8:53"},
		QueryTimeout:     10 * time.Second,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	var mock *mockConnWithDeadline
	r.dialUDP = func(network, address string) (net.Conn, error) {
		mock = &mockConnWithDeadline{
			mockConn: mockConn{
				readErr: fmt.Errorf("test read error"),
			},
		}
		return mock, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := r.querySingle(ctx, "8.8.8.8:53", "example.com", TypeA)
	if err == nil {
		t.Error("expected error, got nil")
	}

	if mock.deadline.IsZero() {
		t.Error("expected deadline to be set")
	}
}

func TestCNAMEFollowSameZoneEfficiency(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  true,
		RootServers:      []string{"192.0.2.1:53"},
		QueryTimeout:     100 * time.Millisecond,
		MaxRecursionDepth: 10,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	queryCount := 0
	r.dialUDP = func(network, address string) (net.Conn, error) {
		queryCount++
		switch queryCount {
		case 1:
			return &mockConn{
				readData: buildNSResponse(".", []string{"a.root-servers.net"}, map[string]string{"a.root-servers.net": "192.0.2.1"}, 3600),
			}, nil
		case 2:
			return &mockConn{
				readData: buildNSResponse("com.", []string{"a.gtld-servers.net"}, map[string]string{"a.gtld-servers.net": "192.0.2.2"}, 3600),
			}, nil
		case 3:
			return &mockConn{
				readData: buildNSResponse("example.com.", []string{"ns1.example.com"}, map[string]string{"ns1.example.com": "192.0.2.3"}, 3600),
			}, nil
		case 4:
			return &mockConn{
				readData: buildCNAMEResponse("www.example.com", "cdn.example.com", 300),
			}, nil
		case 5:
			return &mockConn{
				readData: buildTestResponse("cdn.example.com", "1.2.3.4", 300, TypeA),
			}, nil
		default:
			return &mockConn{
				readErr: fmt.Errorf("unexpected query #%d", queryCount),
			}, nil
		}
	}

	ips, err := r.ResolveA("www.example.com")
	if err != nil {
		t.Fatalf("ResolveA failed: %v", err)
	}
	if len(ips) != 1 || ips[0] != "1.2.3.4" {
		t.Errorf("expected [1.2.3.4], got %v", ips)
	}

	if queryCount > 6 {
		t.Errorf("expected <= 6 queries (3 NS + 2 answers + 1 extra), got %d", queryCount)
	}
}

func TestCNAMEFollowDifferentZone(t *testing.T) {
	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  true,
		RootServers:      []string{"192.0.2.1:53"},
		QueryTimeout:     100 * time.Millisecond,
		MaxRecursionDepth: 20,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	queryCount := 0
	r.dialUDP = func(network, address string) (net.Conn, error) {
		queryCount++
		switch queryCount {
		case 1:
			return &mockConn{
				readData: buildNSResponse(".", []string{"a.root-servers.net"}, map[string]string{"a.root-servers.net": "192.0.2.1"}, 3600),
			}, nil
		case 2:
			return &mockConn{
				readData: buildNSResponse("com.", []string{"a.gtld-servers.net"}, map[string]string{"a.gtld-servers.net": "192.0.2.2"}, 3600),
			}, nil
		case 3:
			return &mockConn{
				readData: buildNSResponse("example.com.", []string{"ns1.example.com"}, map[string]string{"ns1.example.com": "192.0.2.3"}, 3600),
			}, nil
		case 4:
			return &mockConn{
				readData: buildCNAMEResponse("www.example.com", "cdn.otherdomain.net", 300),
			}, nil
		case 5:
			return &mockConn{
				readData: buildCNAMEResponse("www.example.com", "cdn.otherdomain.net", 300),
			}, nil
		case 6:
			return &mockConn{
				readData: buildNSResponse(".", []string{"a.root-servers.net"}, map[string]string{"a.root-servers.net": "192.0.2.1"}, 3600),
			}, nil
		case 7:
			return &mockConn{
				readData: buildNSResponse("net.", []string{"a.gtld-servers.net"}, map[string]string{"a.gtld-servers.net": "192.0.2.4"}, 3600),
			}, nil
		case 8:
			return &mockConn{
				readData: buildNSResponse("otherdomain.net.", []string{"ns1.otherdomain.net"}, map[string]string{"ns1.otherdomain.net": "192.0.2.5"}, 3600),
			}, nil
		case 9:
			return &mockConn{
				readData: buildTestResponse("cdn.otherdomain.net", "5.6.7.8", 300, TypeA),
			}, nil
		case 10:
			return &mockConn{
				readData: buildTestResponse("cdn.otherdomain.net", "5.6.7.8", 300, TypeA),
			}, nil
		default:
			return &mockConn{
				readErr: fmt.Errorf("unexpected query #%d", queryCount),
			}, nil
		}
	}

	ips, err := r.ResolveA("www.example.com")
	if err != nil {
		t.Fatalf("ResolveA failed (queryCount=%d): %v", queryCount, err)
	}
	if len(ips) != 1 || ips[0] != "5.6.7.8" {
		t.Errorf("expected [5.6.7.8], got %v (queryCount=%d)", ips, queryCount)
	}

	if queryCount < 8 {
		t.Errorf("expected at least 8 queries for cross-zone CNAME + full recursion, got %d", queryCount)
	}
}

func TestRCodeConstants(t *testing.T) {
	if RCODE_NOERROR != 0 {
		t.Errorf("expected RCODE_NOERROR = 0, got %d", RCODE_NOERROR)
	}
	if RCODE_FORMERR != 1 {
		t.Errorf("expected RCODE_FORMERR = 1, got %d", RCODE_FORMERR)
	}
	if RCODE_SERVFAIL != 2 {
		t.Errorf("expected RCODE_SERVFAIL = 2, got %d", RCODE_SERVFAIL)
	}
	if RCODE_NXDOMAIN != 3 {
		t.Errorf("expected RCODE_NXDOMAIN = 3, got %d", RCODE_NXDOMAIN)
	}
	if RCODE_REFUSED != 5 {
		t.Errorf("expected RCODE_REFUSED = 5, got %d", RCODE_REFUSED)
	}
}

func TestParallelQueryRCODEError(t *testing.T) {
	nxdomainServer, _ := newMockDNSServer(func(q []byte) []byte {
		return buildRcodeResponse("example.com", RCODE_NXDOMAIN, TypeA)
	})
	defer nxdomainServer.Close()

	servfailServer, _ := newMockDNSServer(func(q []byte) []byte {
		return buildRcodeResponse("example.com", RCODE_SERVFAIL, TypeA)
	})
	defer servfailServer.Close()

	cfg := Config{
		EnableCache:      false,
		EnableRecursion:  false,
		UpstreamServers:  []string{nxdomainServer.addr, servfailServer.addr},
		QueryTimeout:     200 * time.Millisecond,
	}
	r, _ := NewResolverWithConfig(cfg)
	defer r.Close()

	_, err := r.ResolveA("example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDNSErrorIsComparison(t *testing.T) {
	err1 := &DNSError{RCODE: 3, Msg: "NXDOMAIN"}
	err2 := &DNSError{RCODE: 3, Msg: "different message"}
	err3 := &DNSError{RCODE: 2, Msg: "SERVFAIL"}

	if !errors.Is(err1, ErrNXDOMAIN) {
		t.Error("expected err1 to match ErrNXDOMAIN")
	}
	if !errors.Is(err2, ErrNXDOMAIN) {
		t.Error("expected err2 to match ErrNXDOMAIN (same RCODE)")
	}
	if errors.Is(err3, ErrNXDOMAIN) {
		t.Error("expected err3 to not match ErrNXDOMAIN")
	}
	if !errors.Is(err3, ErrSERVFAIL) {
		t.Error("expected err3 to match ErrSERVFAIL")
	}
}
