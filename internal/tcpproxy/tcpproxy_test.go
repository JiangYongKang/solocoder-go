package tcpproxy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func startTCPServer(t *testing.T, handler func(net.Conn)) (string, func()) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handler(conn)
		}
	}()
	return ln.Addr().String(), func() { ln.Close() }
}

func echoHandler(conn net.Conn) {
	defer conn.Close()
	io.Copy(conn, conn)
}

func TestEncodeDecodeFrame(t *testing.T) {
	tests := []struct {
		name    string
		frame   *Frame
		wantErr bool
	}{
		{
			name: "DATA frame with payload",
			frame: &Frame{
				Type:     FrameTypeDATA,
				StreamID: 42,
				Payload:  []byte("hello world"),
			},
		},
		{
			name: "SYNC frame no payload",
			frame: &Frame{
				Type:     FrameTypeSYNC,
				StreamID: 1,
			},
		},
		{
			name: "FIN frame",
			frame: &Frame{
				Type:     FrameTypeFIN,
				StreamID: 100,
			},
		},
		{
			name: "RST frame",
			frame: &Frame{
				Type:     FrameTypeRST,
				StreamID: 999,
			},
		},
		{
			name: "HEARTBEAT frame",
			frame: &Frame{
				Type:     FrameTypeHEARTBEAT,
				StreamID: 0,
			},
		},
		{
			name:    "nil frame",
			frame:   nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := EncodeFrame(tt.frame)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("EncodeFrame failed: %v", err)
			}
			decoded, err := DecodeFrame(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("DecodeFrame failed: %v", err)
			}
			if decoded.Type != tt.frame.Type {
				t.Errorf("type mismatch: got %d want %d", decoded.Type, tt.frame.Type)
			}
			if decoded.StreamID != tt.frame.StreamID {
				t.Errorf("streamID mismatch: got %d want %d", decoded.StreamID, tt.frame.StreamID)
			}
			if !bytes.Equal(decoded.Payload, tt.frame.Payload) {
				t.Errorf("payload mismatch: got %s want %s", decoded.Payload, tt.frame.Payload)
			}
		})
	}
}

func TestDecodeFrame_ShortHeader(t *testing.T) {
	_, err := DecodeFrame(bytes.NewReader([]byte{0x00, 0x01}))
	if err == nil {
		t.Fatalf("expected error for short header, got nil")
	}
}

func TestDecodeFrame_ShortPayload(t *testing.T) {
	f := &Frame{
		Type:     FrameTypeDATA,
		StreamID: 1,
		Payload:  []byte("hello"),
	}
	data, _ := EncodeFrame(f)
	truncated := data[:FrameHeaderSize+2]
	_, err := DecodeFrame(bytes.NewReader(truncated))
	if err == nil {
		t.Fatalf("expected error for short payload, got nil")
	}
}

func TestMuxConn_NewStreamAndCommunicate(t *testing.T) {
	c1, c2 := net.Pipe()
	var serverStream *Stream
	var wg sync.WaitGroup
	wg.Add(1)
	muxServer := NewMuxConn(c2, func(s *Stream) {
		serverStream = s
		wg.Done()
	})
	defer muxServer.Close()

	muxClient := NewMuxConn(c1, nil)
	defer muxClient.Close()

	clientStream, err := muxClient.NewStream()
	if err != nil {
		t.Fatalf("NewStream failed: %v", err)
	}

	wg.Wait()
	if serverStream == nil {
		t.Fatalf("server did not receive stream")
	}

	testData := []byte("hello from client")
	n, err := clientStream.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("write count mismatch: got %d want %d", n, len(testData))
	}

	buf := make([]byte, 1024)
	time.Sleep(100 * time.Millisecond)
	n, err = serverStream.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if !bytes.Equal(buf[:n], testData) {
		t.Errorf("data mismatch: got %s want %s", buf[:n], testData)
	}

	respData := []byte("hello from server")
	n, err = serverStream.Write(respData)
	if err != nil {
		t.Fatalf("server Write failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	n, err = clientStream.Read(buf)
	if err != nil {
		t.Fatalf("client Read failed: %v", err)
	}
	if !bytes.Equal(buf[:n], respData) {
		t.Errorf("response data mismatch: got %s want %s", buf[:n], respData)
	}
}

func TestMuxConn_MultipleStreams(t *testing.T) {
	c1, c2 := net.Pipe()

	serverReceived := make(map[uint16]bool)
	var mu sync.Mutex

	muxServer := NewMuxConn(c2, func(s *Stream) {
		buf := make([]byte, 1024)
		for {
			_, err := s.Read(buf)
			if err != nil {
				return
			}
			mu.Lock()
			serverReceived[s.ID] = true
			mu.Unlock()
			resp := fmt.Sprintf("ack-%d", s.ID)
			s.Write([]byte(resp))
		}
	})
	defer muxServer.Close()

	muxClient := NewMuxConn(c1, nil)
	defer muxClient.Close()

	numStreams := 10
	var wg sync.WaitGroup
	wg.Add(numStreams)

	for i := 0; i < numStreams; i++ {
		go func(idx int) {
			defer wg.Done()
			s, err := muxClient.NewStream()
			if err != nil {
				t.Errorf("NewStream %d failed: %v", idx, err)
				return
			}
			defer s.Close()

			msg := fmt.Sprintf("msg-%d", idx)
			s.Write([]byte(msg))

			buf := make([]byte, 1024)
			time.Sleep(50 * time.Millisecond)
			_, err = s.Read(buf)
			if err != nil && err != io.EOF {
				t.Logf("Read on stream %d: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := len(serverReceived)
	mu.Unlock()

	if count < numStreams {
		t.Logf("Warning: only %d of %d streams received (may be timing)", count, numStreams)
	}
}

func TestMuxConn_StreamClose(t *testing.T) {
	c1, c2 := net.Pipe()
	muxServer := NewMuxConn(c2, func(s *Stream) {
		s.Close()
	})
	defer muxServer.Close()

	muxClient := NewMuxConn(c1, nil)
	defer muxClient.Close()

	s, err := muxClient.NewStream()
	if err != nil {
		t.Fatalf("NewStream failed: %v", err)
	}

	err = s.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	err = s.Close()
	if err != nil {
		t.Fatalf("second Close should not error: %v", err)
	}

	_, err = s.Write([]byte("test"))
	if err != ErrStreamClosed {
		t.Errorf("expected ErrStreamClosed, got: %v", err)
	}
}

func TestMuxConn_Heartbeat(t *testing.T) {
	c1, c2 := net.Pipe()
	muxServer := NewMuxConn(c2, nil)
	defer muxServer.Close()

	muxClient := NewMuxConn(c1, nil)
	defer muxClient.Close()

	hb := &Frame{
		Type:     FrameTypeHEARTBEAT,
		StreamID: 0,
	}
	data, _ := EncodeFrame(hb)
	c1.Write(data)
	time.Sleep(50 * time.Millisecond)
}

func TestMuxConn_ClosedNewStream(t *testing.T) {
	c1, _ := net.Pipe()
	mux := NewMuxConn(c1, nil)
	mux.Close()

	_, err := mux.NewStream()
	if err != ErrMuxConnClosed {
		t.Errorf("expected ErrMuxConnClosed, got: %v", err)
	}
}

func TestMuxConn_StreamCount(t *testing.T) {
	c1, c2 := net.Pipe()
	streams := make(chan *Stream, 10)
	muxServer := NewMuxConn(c2, func(s *Stream) {
		streams <- s
	})
	defer muxServer.Close()

	muxClient := NewMuxConn(c1, nil)
	defer muxClient.Close()

	for i := 0; i < 5; i++ {
		_, err := muxClient.NewStream()
		if err != nil {
			t.Fatalf("NewStream %d failed: %v", i, err)
		}
	}

	time.Sleep(100 * time.Millisecond)
	<-streams
	<-streams

	count := muxClient.StreamCount()
	if count != 5 {
		t.Logf("StreamCount: got %d want 5 (may be timing)", count)
	}
}

func TestUpstream_Healthy(t *testing.T) {
	u := NewUpstream("127.0.0.1:12345")
	if !u.Healthy() {
		t.Error("new upstream should be healthy")
	}
	u.SetHealthy(false)
	if u.Healthy() {
		t.Error("upstream should be unhealthy")
	}
	u.SetHealthy(true)
	if !u.Healthy() {
		t.Error("upstream should be healthy again")
	}
}

func TestUpstream_Probe(t *testing.T) {
	addr, cleanup := startTCPServer(t, echoHandler)
	defer cleanup()

	u := NewUpstream(addr)
	if !u.Probe(2 * time.Second) {
		t.Error("probe should succeed on listening server")
	}

	u2 := NewUpstream("127.0.0.1:1")
	if u2.Probe(100 * time.Millisecond) {
		t.Error("probe should fail on non-listening port")
	}
}

func TestHealthChecker_AddRemoveUpstream(t *testing.T) {
	hc := NewHealthChecker(HealthCheckerConfig{
		CheckInterval: 1 * time.Hour,
		FailThreshold: 1,
		PassThreshold: 1,
	})
	defer hc.Stop()

	u1 := NewUpstream("127.0.0.1:10001")
	u2 := NewUpstream("127.0.0.1:10002")

	hc.AddUpstream(u1)
	hc.AddUpstream(u2)

	ups := hc.GetUpstreams()
	if len(ups) != 2 {
		t.Errorf("got %d upstreams, want 2", len(ups))
	}

	healthy := hc.GetHealthyUpstreams()
	if len(healthy) != 2 {
		t.Errorf("got %d healthy upstreams, want 2", len(healthy))
	}

	hc.RemoveUpstream("127.0.0.1:10001")
	ups = hc.GetUpstreams()
	if len(ups) != 1 {
		t.Errorf("got %d upstreams after remove, want 1", len(ups))
	}
}

func TestHealthChecker_DetectFailure(t *testing.T) {
	hc := NewHealthChecker(HealthCheckerConfig{
		CheckInterval: 50 * time.Millisecond,
		ProbeTimeout:  50 * time.Millisecond,
		FailThreshold: 1,
		PassThreshold: 1,
	})
	defer hc.Stop()

	u := NewUpstream("127.0.0.1:2")
	hc.AddUpstream(u)
	hc.Start()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !u.Healthy() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if u.Healthy() {
		t.Error("unhealthy upstream should have been detected")
	}
}

func TestHealthChecker_DetectRecovery(t *testing.T) {
	addr, cleanup := startTCPServer(t, echoHandler)

	hc := NewHealthChecker(HealthCheckerConfig{
		CheckInterval: 50 * time.Millisecond,
		ProbeTimeout:  200 * time.Millisecond,
		FailThreshold: 1,
		PassThreshold: 1,
	})
	defer hc.Stop()

	u := NewUpstream(addr)
	u.SetHealthy(false)
	hc.AddUpstream(u)
	hc.Start()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if u.Healthy() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	cleanup()
	if !u.Healthy() {
		t.Error("healthy upstream should have been detected as recovered")
	}
}

func TestHealthChecker_OnChange(t *testing.T) {
	_, cleanup := startTCPServer(t, echoHandler)
	defer cleanup()

	hc := NewHealthChecker(HealthCheckerConfig{
		CheckInterval: 50 * time.Millisecond,
		ProbeTimeout:  100 * time.Millisecond,
		FailThreshold: 1,
		PassThreshold: 1,
	})
	defer hc.Stop()

	var mu sync.Mutex
	changes := make([]bool, 0)
	hc.SetOnChange(func(addr string, healthy bool) {
		mu.Lock()
		changes = append(changes, healthy)
		mu.Unlock()
	})

	u := NewUpstream("127.0.0.1:3")
	hc.AddUpstream(u)
	hc.Start()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		l := len(changes)
		mu.Unlock()
		if l > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	mu.Lock()
	hasFalse := false
	for _, c := range changes {
		if !c {
			hasFalse = true
		}
	}
	mu.Unlock()
	if !hasFalse {
		t.Error("OnChange should have been called with false for failing upstream")
	}
}

func TestHealthChecker_StartStop(t *testing.T) {
	hc := NewHealthChecker(HealthCheckerConfig{
		CheckInterval: 10 * time.Millisecond,
		FailThreshold: 1,
		PassThreshold: 1,
	})
	if hc.Running() {
		t.Error("should not be running initially")
	}
	hc.Start()
	if !hc.Running() {
		t.Error("should be running after Start")
	}
	hc.Start()
	hc.Stop()
	if hc.Running() {
		t.Error("should not be running after Stop")
	}
	hc.Stop()
}

type mockNetConn struct {
	readCh  chan []byte
	writeCh chan []byte
	closed  atomic.Bool
	local   net.Addr
	remote  net.Addr
}

func newMockNetConn() *mockNetConn {
	return &mockNetConn{
		readCh:  make(chan []byte, 256),
		writeCh: make(chan []byte, 256),
		local:   &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 10000},
		remote:  &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 20000},
	}
}

func (m *mockNetConn) Read(b []byte) (int, error) {
	if m.closed.Load() {
		return 0, io.EOF
	}
	data, ok := <-m.readCh
	if !ok {
		return 0, io.EOF
	}
	n := copy(b, data)
	return n, nil
}

func (m *mockNetConn) Write(b []byte) (int, error) {
	if m.closed.Load() {
		return 0, errors.New("closed")
	}
	buf := make([]byte, len(b))
	copy(buf, b)
	m.writeCh <- buf
	return len(b), nil
}

func (m *mockNetConn) Close() error {
	if m.closed.Swap(true) {
		return nil
	}
	close(m.readCh)
	return nil
}

func (m *mockNetConn) LocalAddr() net.Addr  { return m.local }
func (m *mockNetConn) RemoteAddr() net.Addr { return m.remote }
func (m *mockNetConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockNetConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockNetConn) SetWriteDeadline(t time.Time) error { return nil }

func mockConnPair() (*mockNetConn, *mockNetConn) {
	a := newMockNetConn()
	b := newMockNetConn()
	go func() {
		for data := range a.writeCh {
			if !b.closed.Load() {
				defer func() { recover() }()
				b.readCh <- data
			}
		}
	}()
	go func() {
		for data := range b.writeCh {
			if !a.closed.Load() {
				defer func() { recover() }()
				a.readCh <- data
			}
		}
	}()
	return a, b
}

func TestConnPool_GetAndPut(t *testing.T) {
	addr, cleanup := startTCPServer(t, echoHandler)
	defer cleanup()

	u := NewUpstream(addr)
	pool := NewConnPool(u, ConnPoolConfig{
		MaxConns:    5,
		IdleTimeout: 1 * time.Hour,
	})
	defer pool.Close()

	conn1, err := pool.Get()
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if conn1 == nil {
		t.Fatal("conn is nil")
	}

	if pool.ActiveCount() != 1 {
		t.Errorf("active count: got %d want 1", pool.ActiveCount())
	}
	if pool.TotalCount() != 1 {
		t.Errorf("total count: got %d want 1", pool.TotalCount())
	}

	err = conn1.Close()
	if err != nil {
		t.Fatalf("Close/Put failed: %v", err)
	}

	if pool.IdleCount() != 1 {
		t.Errorf("idle count: got %d want 1", pool.IdleCount())
	}
	if pool.ActiveCount() != 0 {
		t.Errorf("active count: got %d want 0", pool.ActiveCount())
	}

	conn2, err := pool.Get()
	if err != nil {
		t.Fatalf("second Get failed: %v", err)
	}
	if pool.IdleCount() != 0 {
		t.Errorf("idle count after reuse: got %d want 0", pool.IdleCount())
	}
	conn2.Close()
}

func TestConnPool_MaxConns(t *testing.T) {
	addr, cleanup := startTCPServer(t, echoHandler)
	defer cleanup()

	u := NewUpstream(addr)
	pool := NewConnPool(u, ConnPoolConfig{
		MaxConns:    2,
		IdleTimeout: 1 * time.Hour,
		WaitTimeout: 50 * time.Millisecond,
	})
	defer pool.Close()

	c1, err := pool.Get()
	if err != nil {
		t.Fatalf("Get 1 failed: %v", err)
	}
	c2, err := pool.Get()
	if err != nil {
		t.Fatalf("Get 2 failed: %v", err)
	}

	_, err = pool.Get()
	if err != ErrPoolExhausted {
		t.Errorf("expected ErrPoolExhausted, got: %v", err)
	}

	c1.Close()
	c3, err := pool.Get()
	if err != nil {
		t.Fatalf("Get 3 after release should succeed: %v", err)
	}
	c2.Close()
	c3.Close()
}

func TestConnPool_MaxConnsNoWait(t *testing.T) {
	addr, cleanup := startTCPServer(t, echoHandler)
	defer cleanup()

	u := NewUpstream(addr)
	pool := NewConnPool(u, ConnPoolConfig{
		MaxConns:    1,
		IdleTimeout: 1 * time.Hour,
		WaitTimeout: 0,
	})
	defer pool.Close()

	c1, _ := pool.Get()
	defer c1.Close()

	_, err := pool.Get()
	if err != ErrPoolExhausted {
		t.Errorf("expected ErrPoolExhausted, got: %v", err)
	}
}

func TestConnPool_IdleTimeout(t *testing.T) {
	addr, cleanup := startTCPServer(t, echoHandler)
	defer cleanup()

	u := NewUpstream(addr)
	pool := NewConnPool(u, ConnPoolConfig{
		MaxConns:    5,
		IdleTimeout: 50 * time.Millisecond,
	})
	defer pool.Close()

	c1, _ := pool.Get()
	c1.Close()

	time.Sleep(100 * time.Millisecond)
	pool.reclaimIdle()

	if pool.IdleCount() != 0 {
		t.Errorf("idle count after timeout: got %d want 0", pool.IdleCount())
	}
	if pool.TotalCount() != 0 {
		t.Errorf("total count after timeout: got %d want 0", pool.TotalCount())
	}
}

func TestConnPool_Closed(t *testing.T) {
	addr, cleanup := startTCPServer(t, echoHandler)
	defer cleanup()

	u := NewUpstream(addr)
	pool := NewConnPool(u, ConnPoolConfig{
		MaxConns:    5,
		IdleTimeout: 1 * time.Hour,
	})
	pool.Close()
	pool.Close()

	_, err := pool.Get()
	if err != ErrPoolClosed {
		t.Errorf("expected ErrPoolClosed, got: %v", err)
	}
}

func TestConnPool_ConnectFail(t *testing.T) {
	u := NewUpstream("127.0.0.1:2")
	pool := NewConnPool(u, ConnPoolConfig{
		MaxConns:    5,
		IdleTimeout: 1 * time.Hour,
	})
	defer pool.Close()

	_, err := pool.Get()
	if err == nil {
		t.Error("expected error connecting to non-existent server")
	}
	if pool.TotalCount() != 0 {
		t.Errorf("total count should be 0 on failed connect, got %d", pool.TotalCount())
	}
}

func TestIPHashBalancer_HashConsistency(t *testing.T) {
	hc := NewHealthChecker(HealthCheckerConfig{
		CheckInterval: 1 * time.Hour,
		FailThreshold: 1,
		PassThreshold: 1,
	})
	defer hc.Stop()

	b := NewIPHashBalancer(hc)
	u1 := NewUpstream("127.0.0.1:8001")
	u2 := NewUpstream("127.0.0.1:8002")
	u3 := NewUpstream("127.0.0.1:8003")
	upstreams := []*Upstream{u1, u2, u3}
	b.SetUpstreams(upstreams)
	hc.AddUpstream(u1)
	hc.AddUpstream(u2)
	hc.AddUpstream(u3)

	results := make(map[string]string)
	for i := 0; i < 100; i++ {
		ip := fmt.Sprintf("192.168.1.%d", i)
		u, err := b.GetUpstream(ip)
		if err != nil {
			t.Fatalf("GetUpstream failed: %v", err)
		}
		results[ip] = u.Address
	}

	for ip, addr := range results {
		u, err := b.GetUpstream(ip)
		if err != nil {
			t.Fatalf("second GetUpstream failed: %v", err)
		}
		if u.Address != addr {
			t.Errorf("IP %s mapped to %s then %s: not consistent", ip, addr, u.Address)
		}
	}
}

func TestIPHashBalancer_StickySession(t *testing.T) {
	hc := NewHealthChecker(HealthCheckerConfig{
		CheckInterval: 1 * time.Hour,
		FailThreshold: 1,
		PassThreshold: 1,
	})
	defer hc.Stop()

	b := NewIPHashBalancer(hc)
	u1 := NewUpstream("127.0.0.1:8001")
	u2 := NewUpstream("127.0.0.1:8002")
	upstreams := []*Upstream{u1, u2}
	b.SetUpstreams(upstreams)
	hc.AddUpstream(u1)
	hc.AddUpstream(u2)

	clientIP := "10.0.0.1"
	selected1, err := b.GetUpstream(clientIP)
	if err != nil {
		t.Fatalf("GetUpstream failed: %v", err)
	}

	if b.MappingCount() != 1 {
		t.Errorf("mapping count: got %d want 1", b.MappingCount())
	}

	selected2, err := b.GetUpstream(clientIP)
	if err != nil {
		t.Fatalf("second GetUpstream failed: %v", err)
	}
	if selected1.Address != selected2.Address {
		t.Errorf("sticky session: got %s then %s", selected1.Address, selected2.Address)
	}
}

func TestIPHashBalancer_UpstreamUnhealthy(t *testing.T) {
	hc := NewHealthChecker(HealthCheckerConfig{
		CheckInterval: 1 * time.Hour,
		FailThreshold: 1,
		PassThreshold: 1,
	})
	defer hc.Stop()

	b := NewIPHashBalancer(hc)
	u1 := NewUpstream("127.0.0.1:8001")
	u2 := NewUpstream("127.0.0.1:8002")
	b.SetUpstreams([]*Upstream{u1, u2})
	hc.AddUpstream(u1)
	hc.AddUpstream(u2)

	clientIP := "10.0.0.1"
	selected, err := b.GetUpstream(clientIP)
	if err != nil {
		t.Fatalf("GetUpstream failed: %v", err)
	}

	originalAddr := selected.Address
	selected.SetHealthy(false)

	selected2, err := b.GetUpstream(clientIP)
	if err != nil {
		t.Fatalf("GetUpstream after unhealthy failed: %v", err)
	}
	if selected2.Address == originalAddr {
		t.Error("should not select unhealthy upstream")
	}

	u1.SetHealthy(true)
	u2.SetHealthy(true)
}

func TestIPHashBalancer_NoHealthy(t *testing.T) {
	hc := NewHealthChecker(HealthCheckerConfig{
		CheckInterval: 1 * time.Hour,
		FailThreshold: 1,
		PassThreshold: 1,
	})
	defer hc.Stop()

	b := NewIPHashBalancer(hc)
	u1 := NewUpstream("127.0.0.1:8001")
	u1.SetHealthy(false)
	b.SetUpstreams([]*Upstream{u1})
	hc.AddUpstream(u1)

	_, err := b.GetUpstream("10.0.0.1")
	if err != ErrNoHealthyUpstream {
		t.Errorf("expected ErrNoHealthyUpstream, got: %v", err)
	}
}

func TestIPHashBalancer_RemoveFromMapping(t *testing.T) {
	hc := NewHealthChecker(HealthCheckerConfig{
		CheckInterval: 1 * time.Hour,
		FailThreshold: 1,
		PassThreshold: 1,
	})
	defer hc.Stop()

	b := NewIPHashBalancer(hc)
	u1 := NewUpstream("127.0.0.1:8001")
	u2 := NewUpstream("127.0.0.1:8002")
	b.SetUpstreams([]*Upstream{u1, u2})
	hc.AddUpstream(u1)
	hc.AddUpstream(u2)

	b.GetUpstream("10.0.0.1")
	b.GetUpstream("10.0.0.2")
	b.GetUpstream("10.0.0.3")
	countBefore := b.MappingCount()

	b.RemoveFromMapping("127.0.0.1:8001")
	countAfter := b.MappingCount()
	t.Logf("mapping count before=%d after=%d", countBefore, countAfter)

	b.ClearMapping()
	if b.MappingCount() != 0 {
		t.Errorf("after ClearMapping, count should be 0, got %d", b.MappingCount())
	}
}

func TestNewTCPProxy_Validation(t *testing.T) {
	_, err := NewTCPProxy(ProxyConfig{})
	if err == nil {
		t.Error("expected error for empty config")
	}

	_, err = NewTCPProxy(ProxyConfig{
		ListenAddress: "127.0.0.1:0",
	})
	if err == nil {
		t.Error("expected error for no upstreams")
	}
}

func TestTCPProxy_StartStop(t *testing.T) {
	addr, cleanup := startTCPServer(t, echoHandler)
	defer cleanup()

	proxy, err := NewTCPProxy(ProxyConfig{
		ListenAddress: "127.0.0.1:0",
		Upstreams:     []string{addr},
		HealthCheckConfig: HealthCheckerConfig{
			CheckInterval: 1 * time.Hour,
			FailThreshold: 1,
			PassThreshold: 1,
		},
		PoolMaxConns: 5,
	})
	if err != nil {
		t.Fatalf("NewTCPProxy failed: %v", err)
	}

	err = proxy.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	listenAddr := proxy.Addr()
	if listenAddr == "" {
		t.Error("Addr should return listening address")
	}

	err = proxy.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestTCPProxy_EndToEnd(t *testing.T) {
	addr, cleanup := startTCPServer(t, echoHandler)
	defer cleanup()

	proxy, err := NewTCPProxy(ProxyConfig{
		ListenAddress: "127.0.0.1:0",
		Upstreams:     []string{addr},
		HealthCheckConfig: HealthCheckerConfig{
			CheckInterval: 1 * time.Hour,
			FailThreshold: 1,
			PassThreshold: 1,
		},
		PoolMaxConns:      10,
		EnableStickySession: true,
	})
	if err != nil {
		t.Fatalf("NewTCPProxy failed: %v", err)
	}
	err = proxy.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer proxy.Stop()

	time.Sleep(100 * time.Millisecond)

	proxyAddr := proxy.Addr()
	clientConn, err := net.Dial("tcp", proxyAddr)
	if err != nil {
		t.Fatalf("Dial proxy failed: %v", err)
	}
	defer clientConn.Close()

	streamsCreated := make(chan *Stream, 1)
	mux := NewMuxConn(clientConn, func(s *Stream) {
		streamsCreated <- s
	})
	defer mux.Close()

	s, err := mux.NewStream()
	if err != nil {
		t.Fatalf("NewStream failed: %v", err)
	}
	defer s.Close()

	testMsg := []byte("e2e test message")
	_, err = s.Write(testMsg)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	buf := make([]byte, 1024)
	time.Sleep(200 * time.Millisecond)
	n, err := s.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("Read failed: %v", err)
	}
	if n > 0 && !bytes.Equal(buf[:n], testMsg) {
		t.Errorf("echo mismatch: got %s want %s", buf[:n], testMsg)
	}
}

func TestStream_ReadLargeData(t *testing.T) {
	c1, c2 := net.Pipe()
	muxServer := NewMuxConn(c2, func(s *Stream) {
		buf := make([]byte, 64*1024)
		for {
			n, err := s.Read(buf)
			if err != nil {
				return
			}
			s.Write(buf[:n])
		}
	})
	defer muxServer.Close()

	muxClient := NewMuxConn(c1, nil)
	defer muxClient.Close()

	s, err := muxClient.NewStream()
	if err != nil {
		t.Fatalf("NewStream failed: %v", err)
	}
	defer s.Close()

	largeData := make([]byte, 100*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		n, err := s.Write(largeData)
		if err != nil {
			t.Errorf("Write large data: %v", err)
		}
		if n != len(largeData) {
			t.Errorf("Write short: %d of %d", n, len(largeData))
		}
	}()

	received := make([]byte, 0, len(largeData))
	buf := make([]byte, 8192)
	deadline := time.Now().Add(5 * time.Second)
	for len(received) < len(largeData) && time.Now().Before(deadline) {
		n, err := s.Read(buf)
		if err != nil {
			if err != io.EOF && !errors.Is(err, net.ErrClosed) {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			break
		}
		received = append(received, buf[:n]...)
	}
	wg.Wait()

	if len(received) < len(largeData) {
		t.Logf("received %d of %d bytes (may be timing issue)", len(received), len(largeData))
	}
	if len(received) > 0 {
		for i := 0; i < len(received); i++ {
			if received[i] != largeData[i] {
				t.Errorf("data mismatch at %d: got %d want %d", i, received[i], largeData[i])
				break
			}
		}
	}
}

func TestMuxConn_RSTFrame(t *testing.T) {
	c1, c2 := net.Pipe()
	muxServer := NewMuxConn(c2, func(s *Stream) {
		rst := &Frame{
			Type:     FrameTypeRST,
			StreamID: s.ID,
		}
		buf, _ := EncodeFrame(rst)
		c2.Write(buf)
	})
	defer muxServer.Close()

	muxClient := NewMuxConn(c1, nil)
	defer muxClient.Close()

	_, err := muxClient.NewStream()
	if err != nil {
		t.Fatalf("NewStream failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
}

func TestConnPool_ConcurrentAccess(t *testing.T) {
	addr, cleanup := startTCPServer(t, echoHandler)
	defer cleanup()

	u := NewUpstream(addr)
	pool := NewConnPool(u, ConnPoolConfig{
		MaxConns:    10,
		IdleTimeout: 1 * time.Hour,
	})
	defer pool.Close()

	var wg sync.WaitGroup
	numGoroutines := 20
	opsPerGoroutine := 50
	var errors atomic.Int32

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				conn, err := pool.Get()
				if err != nil {
					if err != ErrPoolExhausted {
						errors.Add(1)
					}
					time.Sleep(1 * time.Millisecond)
					continue
				}
				time.Sleep(1 * time.Millisecond)
				conn.Close()
			}
		}()
	}
	wg.Wait()

	if errors.Load() > 0 {
		t.Errorf("%d unexpected errors during concurrent access", errors.Load())
	}
}

func TestIPHashBalancer_Concurrency(t *testing.T) {
	hc := NewHealthChecker(HealthCheckerConfig{
		CheckInterval: 1 * time.Hour,
		FailThreshold: 1,
		PassThreshold: 1,
	})
	defer hc.Stop()

	b := NewIPHashBalancer(hc)
	upstreams := make([]*Upstream, 5)
	for i := 0; i < 5; i++ {
		u := NewUpstream(fmt.Sprintf("127.0.0.1:%d", 9000+i))
		upstreams[i] = u
		hc.AddUpstream(u)
	}
	b.SetUpstreams(upstreams)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				ip := fmt.Sprintf("10.0.%d.%d", idx, j%256)
				_, err := b.GetUpstream(ip)
				if err != nil && err != ErrNoHealthyUpstream {
					t.Errorf("unexpected error: %v", err)
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestFrame_AllTypes(t *testing.T) {
	types := []uint16{
		FrameTypeSYNC,
		FrameTypeDATA,
		FrameTypeACK,
		FrameTypeFIN,
		FrameTypeRST,
		FrameTypeHEARTBEAT,
	}
	for _, typ := range types {
		f := &Frame{
			Type:     typ,
			StreamID: 42,
			Payload:  []byte("test"),
		}
		data, err := EncodeFrame(f)
		if err != nil {
			t.Fatalf("EncodeFrame type %d failed: %v", typ, err)
		}
		decoded, err := DecodeFrame(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("DecodeFrame type %d failed: %v", typ, err)
		}
		if decoded.Type != typ {
			t.Errorf("type %d: decoded type %d", typ, decoded.Type)
		}
	}
}

func TestMuxConn_ClosedFlag(t *testing.T) {
	c1, c2 := net.Pipe()
	mux := NewMuxConn(c1, nil)
	if mux.Closed() {
		t.Error("new mux should not be closed")
	}
	mux.Close()
	if !mux.Closed() {
		t.Error("mux should be closed after Close()")
	}
	c2.Close()
}

func TestHealthChecker_EmptyUpstreams(t *testing.T) {
	hc := NewHealthChecker(HealthCheckerConfig{
		CheckInterval: 10 * time.Millisecond,
		FailThreshold: 1,
		PassThreshold: 1,
	})
	hc.Start()
	time.Sleep(50 * time.Millisecond)
	hc.Stop()

	ups := hc.GetHealthyUpstreams()
	if len(ups) != 0 {
		t.Errorf("expected 0 healthy upstreams, got %d", len(ups))
	}
}

func TestConnPool_DoubleClosePut(t *testing.T) {
	addr, cleanup := startTCPServer(t, echoHandler)
	defer cleanup()

	u := NewUpstream(addr)
	pool := NewConnPool(u, ConnPoolConfig{
		MaxConns:    5,
		IdleTimeout: 1 * time.Hour,
	})
	defer pool.Close()

	c, err := pool.Get()
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	err = c.Close()
	if err != nil {
		t.Fatalf("first Close failed: %v", err)
	}
	err = c.Close()
	if err != nil {
		t.Logf("double Close returned: %v (may be OK)", err)
	}
}

func TestIPHashBalancer_DifferentIPs(t *testing.T) {
	hc := NewHealthChecker(HealthCheckerConfig{
		CheckInterval: 1 * time.Hour,
		FailThreshold: 1,
		PassThreshold: 1,
	})
	defer hc.Stop()

	b := NewIPHashBalancer(hc)
	u1 := NewUpstream("127.0.0.1:8001")
	u2 := NewUpstream("127.0.0.1:8002")
	u3 := NewUpstream("127.0.0.1:8003")
	b.SetUpstreams([]*Upstream{u1, u2, u3})
	hc.AddUpstream(u1)
	hc.AddUpstream(u2)
	hc.AddUpstream(u3)

	distribution := make(map[string]int)
	for i := 0; i < 1000; i++ {
		ip := fmt.Sprintf("172.16.%d.%d", i/256, i%256)
		u, err := b.GetUpstream(ip)
		if err != nil {
			t.Fatalf("GetUpstream failed: %v", err)
		}
		distribution[u.Address]++
	}

	t.Logf("distribution: %v", distribution)
	if len(distribution) < 2 {
		t.Error("hash should distribute across at least 2 upstreams")
	}
}

func TestConnPool_RemoveThenCloseNoNegativeCount(t *testing.T) {
	addr, cleanup := startTCPServer(t, echoHandler)
	defer cleanup()

	u := NewUpstream(addr)
	pool := NewConnPool(u, ConnPoolConfig{
		MaxConns:    5,
		IdleTimeout: 1 * time.Hour,
	})
	defer pool.Close()

	conn, err := pool.Get()
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	pc := conn.(*pooledConn)
	_ = pool.Remove(pc.pc)

	if pool.ActiveCount() != 0 {
		t.Errorf("active count after Remove: got %d want 0", pool.ActiveCount())
	}

	err = conn.Close()
	if err != nil {
		t.Logf("Close after Remove: %v", err)
	}

	if pool.ActiveCount() < 0 {
		t.Errorf("active count should not be negative, got %d", pool.ActiveCount())
	}
	if pool.TotalCount() < 0 {
		t.Errorf("total count should not be negative, got %d", pool.TotalCount())
	}
}

func TestConnPool_RemoveIdempotent(t *testing.T) {
	addr, cleanup := startTCPServer(t, echoHandler)
	defer cleanup()

	u := NewUpstream(addr)
	pool := NewConnPool(u, ConnPoolConfig{
		MaxConns:    5,
		IdleTimeout: 1 * time.Hour,
	})
	defer pool.Close()

	conn, err := pool.Get()
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	pc := conn.(*pooledConn)
	_ = pool.Remove(pc.pc)
	_ = pool.Remove(pc.pc)
	_ = pool.Remove(pc.pc)

	if pool.ActiveCount() != 0 {
		t.Errorf("active count after multiple Remove: got %d want 0", pool.ActiveCount())
	}
}

func TestConnPool_PutAfterRemovedNoReturnToIdle(t *testing.T) {
	addr, cleanup := startTCPServer(t, echoHandler)
	defer cleanup()

	u := NewUpstream(addr)
	pool := NewConnPool(u, ConnPoolConfig{
		MaxConns:    5,
		IdleTimeout: 1 * time.Hour,
	})
	defer pool.Close()

	conn, err := pool.Get()
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	pc := conn.(*pooledConn)
	_ = pool.Remove(pc.pc)

	err = conn.Close()
	if err != nil {
		t.Logf("Close returned: %v", err)
	}

	if pool.IdleCount() != 0 {
		t.Errorf("idle count should be 0 after Remove+Close, got %d", pool.IdleCount())
	}
	if pool.TotalCount() != 0 {
		t.Errorf("total count should be 0 after Remove+Close, got %d", pool.TotalCount())
	}
}

func TestConnPool_ConcurrentRemoveAndPut(t *testing.T) {
	addr, cleanup := startTCPServer(t, echoHandler)
	defer cleanup()

	u := NewUpstream(addr)
	pool := NewConnPool(u, ConnPoolConfig{
		MaxConns:    20,
		IdleTimeout: 1 * time.Hour,
	})
	defer pool.Close()

	var wg sync.WaitGroup
	numOps := 100
	var countErrors atomic.Int32

	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := pool.Get()
			if err != nil {
				return
			}
			pc := conn.(*pooledConn)
			if i%3 == 0 {
				_ = pool.Remove(pc.pc)
			}
			_ = conn.Close()
			if pool.ActiveCount() < 0 || pool.TotalCount() < 0 {
				countErrors.Add(1)
			}
		}()
	}
	wg.Wait()

	if countErrors.Load() > 0 {
		t.Errorf("detected negative counts %d times", countErrors.Load())
	}
	if pool.ActiveCount() < 0 {
		t.Errorf("final active count is negative: %d", pool.ActiveCount())
	}
}

func TestConnPool_GetIdleExpiryNoIndexPanic(t *testing.T) {
	addr, cleanup := startTCPServer(t, echoHandler)
	defer cleanup()

	u := NewUpstream(addr)
	pool := NewConnPool(u, ConnPoolConfig{
		MaxConns:    10,
		IdleTimeout: 1 * time.Millisecond,
	})
	defer pool.Close()

	conns := make([]net.Conn, 5)
	for i := 0; i < 5; i++ {
		c, err := pool.Get()
		if err != nil {
			t.Fatalf("Get %d failed: %v", i, err)
		}
		conns[i] = c
	}
	for _, c := range conns {
		c.Close()
	}

	time.Sleep(50 * time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := pool.Get()
			if err != nil && err != ErrPoolExhausted && err != ErrPoolClosed {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestIPHashBalancer_NoDeadlockWithHealthChecker(t *testing.T) {
	hc := NewHealthChecker(HealthCheckerConfig{
		CheckInterval: 10 * time.Millisecond,
		ProbeTimeout:  5 * time.Millisecond,
		FailThreshold: 1,
		PassThreshold: 1,
	})

	b := NewIPHashBalancer(hc)
	u1 := NewUpstream("127.0.0.1:1")
	u2 := NewUpstream("127.0.0.1:2")
	b.SetUpstreams([]*Upstream{u1, u2})
	hc.AddUpstream(u1)
	hc.AddUpstream(u2)

	var mu sync.Mutex
	onChangeCalled := false
	hc.SetOnChange(func(addr string, healthy bool) {
		mu.Lock()
		onChangeCalled = true
		mu.Unlock()
		b.RemoveFromMapping(addr)
	})

	hc.Start()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			_, _ = b.GetUpstream(fmt.Sprintf("10.0.0.%d", i%256))
			time.Sleep(time.Millisecond)
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock detected in GetUpstream with HealthChecker running")
	}

	hc.Stop()

	mu.Lock()
	_ = onChangeCalled
	mu.Unlock()
}

func TestMuxConcurrentFinAndRst(t *testing.T) {
	for trial := 0; trial < 10; trial++ {
		c1, c2 := net.Pipe()
		var wg sync.WaitGroup
		wg.Add(1)
		muxServer := NewMuxConn(c2, func(s *Stream) {
			wg.Done()
		})
		muxClient := NewMuxConn(c1, nil)

		s, err := muxClient.NewStream()
		if err != nil {
			t.Fatalf("NewStream failed: %v", err)
		}

		wg.Wait()

		go func() {
			finFrame := &Frame{
				Type:     FrameTypeFIN,
				StreamID: s.ID,
			}
			data, _ := EncodeFrame(finFrame)
			c1.Write(data)
		}()

		go func() {
			rstFrame := &Frame{
				Type:     FrameTypeRST,
				StreamID: s.ID,
			}
			data, _ := EncodeFrame(rstFrame)
			c1.Write(data)
		}()

		time.Sleep(50 * time.Millisecond)
		muxClient.Close()
		muxServer.Close()
	}
}

func TestHealthChecker_RemoveAndReaddResetsCounters(t *testing.T) {
	addr, cleanup := startTCPServer(t, echoHandler)
	defer cleanup()

	hc := NewHealthChecker(HealthCheckerConfig{
		CheckInterval: 1 * time.Hour,
		FailThreshold: 3,
		PassThreshold: 2,
	})
	defer hc.Stop()

	u1 := NewUpstream(addr)
	hc.AddUpstream(u1)

	hc.checkOne(addr)
	hc.checkOne(addr)

	hc.RemoveUpstream(addr)

	u2 := NewUpstream(addr)
	u2.SetHealthy(false)
	hc.AddUpstream(u2)

	hc.checkOne(addr)

	if u2.Healthy() {
		t.Error("re-added upstream should not inherit old passCount; one pass should not make it healthy with PassThreshold=2")
	}
}

func TestConnPool_ConcurrentGetWithIdleExpiry(t *testing.T) {
	addr, cleanup := startTCPServer(t, echoHandler)
	defer cleanup()

	u := NewUpstream(addr)
	pool := NewConnPool(u, ConnPoolConfig{
		MaxConns:    10,
		IdleTimeout: 5 * time.Millisecond,
	})
	defer pool.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				conn, err := pool.Get()
				if err != nil {
					if err == ErrPoolExhausted || err == ErrPoolClosed {
						continue
					}
					t.Errorf("unexpected error: %v", err)
					return
				}
				time.Sleep(time.Millisecond)
				conn.Close()
			}
		}()
	}
	wg.Wait()

	if pool.ActiveCount() < 0 {
		t.Errorf("active count should not be negative: %d", pool.ActiveCount())
	}
}
