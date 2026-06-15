package grpcsvc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type echoService struct {
	calledCount int
	mu          sync.Mutex
}

func (s *echoService) incr() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calledCount++
}

func (s *echoService) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calledCount
}

func newEchoServiceDesc() *ServiceDesc {
	svc := &echoService{}
	return &ServiceDesc{
		ServiceName: "EchoService",
		Methods: []MethodDesc{
			{
				MethodName: "Echo",
				Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
					svc.incr()
					return req, nil
				},
			},
			{
				MethodName: "Hello",
				Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
					name := req.(string)
					return "Hello, " + name, nil
				},
			},
		},
		Streams: []StreamDesc{
			{
				StreamName:    "ServerStream",
				ServerStreams: true,
				ClientStreams: false,
				Handler: func(srv interface{}, stream Stream) error {
					for i := 0; i < 3; i++ {
						msg := fmt.Sprintf("msg-%d", i)
						if err := stream.SendMsg(msg); err != nil {
							return err
						}
					}
					return nil
				},
			},
			{
				StreamName:    "ClientStream",
				ServerStreams: false,
				ClientStreams: true,
				Handler: func(srv interface{}, stream Stream) error {
					count := 0
					for {
						_, err := stream.Recv()
						if err != nil {
							if errors.Is(err, ErrStreamClosed) {
								break
							}
							return err
						}
						count++
						if count == 5 {
							break
						}
					}
					return stream.SendMsg(fmt.Sprintf("received %d messages", count))
				},
			},
			{
				StreamName:    "BidiStream",
				ServerStreams: true,
				ClientStreams: true,
				Handler: func(srv interface{}, stream Stream) error {
					for {
						_, err := stream.Recv()
						if err != nil {
							if errors.Is(err, ErrStreamClosed) {
								return nil
							}
							return err
						}
						if err := stream.SendMsg("echo"); err != nil {
							return err
						}
					}
				},
			},
		},
	}
}

func TestNewServer(t *testing.T) {
	s := NewServer()
	if s == nil {
		t.Fatal("expected non-nil server")
	}
	if !s.IsRunning() {
		t.Error("expected server to be running")
	}
	if s.ServiceCount() != 0 {
		t.Errorf("expected 0 services, got %d", s.ServiceCount())
	}
}

func TestNewServerWithOptions(t *testing.T) {
	opts := ServerOptions{
		MaxConcurrentStreams: 200,
		ConnectionTimeout:    60 * time.Second,
	}
	s := NewServerWithOptions(opts)
	if s == nil {
		t.Fatal("expected non-nil server")
	}
	if !s.IsRunning() {
		t.Error("expected server to be running")
	}

	gotOpts := s.Options()
	if gotOpts.MaxConcurrentStreams != 200 {
		t.Errorf("expected MaxConcurrentStreams 200, got %d", gotOpts.MaxConcurrentStreams)
	}
	if gotOpts.ConnectionTimeout != 60*time.Second {
		t.Errorf("expected ConnectionTimeout 60s, got %v", gotOpts.ConnectionTimeout)
	}
}

func TestNewServerWithOptions_Defaults(t *testing.T) {
	opts := ServerOptions{}
	s := NewServerWithOptions(opts)
	gotOpts := s.Options()

	if gotOpts.MaxConcurrentStreams != 100 {
		t.Errorf("expected default MaxConcurrentStreams 100, got %d", gotOpts.MaxConcurrentStreams)
	}
	if gotOpts.ConnectionTimeout != 30*time.Second {
		t.Errorf("expected default ConnectionTimeout 30s, got %v", gotOpts.ConnectionTimeout)
	}
}

func TestRegisterService(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	svc := &echoService{}

	err := s.RegisterService(sd, svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.ServiceCount() != 1 {
		t.Errorf("expected 1 service, got %d", s.ServiceCount())
	}

	if count := s.MethodCount("EchoService"); count != 5 {
		t.Errorf("expected 5 methods (2 unary + 3 streams), got %d", count)
	}
}

func TestRegisterService_NilDescriptor(t *testing.T) {
	s := NewServer()
	err := s.RegisterService(nil, &echoService{})
	if !errors.Is(err, ErrInvalidServiceDesc) {
		t.Errorf("expected ErrInvalidServiceDesc, got %v", err)
	}
}

func TestRegisterService_EmptyServiceName(t *testing.T) {
	s := NewServer()
	sd := &ServiceDesc{ServiceName: ""}
	err := s.RegisterService(sd, &echoService{})
	if !errors.Is(err, ErrInvalidServiceDesc) {
		t.Errorf("expected ErrInvalidServiceDesc, got %v", err)
	}
}

func TestRegisterService_NilImpl(t *testing.T) {
	s := NewServer()
	sd := &ServiceDesc{ServiceName: "TestService"}
	err := s.RegisterService(sd, nil)
	if !errors.Is(err, ErrInvalidServiceDesc) {
		t.Errorf("expected ErrInvalidServiceDesc, got %v", err)
	}
}

func TestRegisterService_Duplicate(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	svc := &echoService{}

	err := s.RegisterService(sd, svc)
	if err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	err = s.RegisterService(sd, svc)
	if !errors.Is(err, ErrServiceExists) {
		t.Errorf("expected ErrServiceExists, got %v", err)
	}
}

func TestRegisterService_ServerStopped(t *testing.T) {
	s := NewServer()
	s.Stop()

	sd := newEchoServiceDesc()
	err := s.RegisterService(sd, &echoService{})
	if !errors.Is(err, ErrServerStopped) {
		t.Errorf("expected ErrServerStopped, got %v", err)
	}
}

func TestRegisterService_InvalidMethod(t *testing.T) {
	s := NewServer()
	sd := &ServiceDesc{
		ServiceName: "TestService",
		Methods: []MethodDesc{
			{MethodName: ""},
		},
	}
	err := s.RegisterService(sd, &echoService{})
	if !errors.Is(err, ErrInvalidMethodDesc) {
		t.Errorf("expected ErrInvalidMethodDesc, got %v", err)
	}
}

func TestRegisterService_NilHandler(t *testing.T) {
	s := NewServer()
	sd := &ServiceDesc{
		ServiceName: "TestService",
		Methods: []MethodDesc{
			{MethodName: "TestMethod", Handler: nil},
		},
	}
	err := s.RegisterService(sd, &echoService{})
	if !errors.Is(err, ErrNilHandler) {
		t.Errorf("expected ErrNilHandler, got %v", err)
	}
}

func TestGetService(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	result, ok := s.GetService("EchoService")
	if !ok {
		t.Fatal("expected service to exist")
	}
	if result.ServiceName != "EchoService" {
		t.Errorf("expected service name EchoService, got %s", result.ServiceName)
	}
	if len(result.Methods) != 2 {
		t.Errorf("expected 2 methods, got %d", len(result.Methods))
	}
	if len(result.Streams) != 3 {
		t.Errorf("expected 3 streams, got %d", len(result.Streams))
	}
}

func TestGetService_NotFound(t *testing.T) {
	s := NewServer()
	_, ok := s.GetService("NonExistent")
	if ok {
		t.Error("expected service not to exist")
	}
}

func TestInvoke_Unary(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	ctx := context.Background()
	resp, err := s.Invoke(ctx, "EchoService", "Echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "hello" {
		t.Errorf("expected hello, got %v", resp)
	}
}

func TestInvoke_Hello(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	ctx := context.Background()
	resp, err := s.Invoke(ctx, "EchoService", "Hello", "World")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "Hello, World" {
		t.Errorf("expected 'Hello, World', got %v", resp)
	}
}

func TestInvoke_ServiceNotFound(t *testing.T) {
	s := NewServer()
	ctx := context.Background()
	_, err := s.Invoke(ctx, "NonExistent", "Method", nil)
	if !errors.Is(err, ErrServiceNotFound) {
		t.Errorf("expected ErrServiceNotFound, got %v", err)
	}
}

func TestInvoke_MethodNotFound(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	ctx := context.Background()
	_, err := s.Invoke(ctx, "EchoService", "NonExistent", nil)
	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("expected ErrMethodNotFound, got %v", err)
	}
}

func TestInvoke_ServerStopped(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})
	s.Stop()

	ctx := context.Background()
	_, err := s.Invoke(ctx, "EchoService", "Echo", "hello")
	if !errors.Is(err, ErrServerStopped) {
		t.Errorf("expected ErrServerStopped, got %v", err)
	}
}

func TestUnaryInterceptor(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	var order []string

	interceptor1 := func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error) {
		order = append(order, "before1")
		resp, err := handler(ctx, req)
		order = append(order, "after1")
		return resp, err
	}

	interceptor2 := func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error) {
		order = append(order, "before2")
		resp, err := handler(ctx, req)
		order = append(order, "after2")
		return resp, err
	}

	s.AddUnaryInterceptor(interceptor1)
	s.AddUnaryInterceptor(interceptor2)

	ctx := context.Background()
	resp, err := s.Invoke(ctx, "EchoService", "Echo", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "test" {
		t.Errorf("expected test, got %v", resp)
	}

	expected := []string{"before1", "before2", "after2", "after1"}
	if len(order) != len(expected) {
		t.Fatalf("expected order length %d, got %d", len(expected), len(order))
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("expected order[%d] = %s, got %s", i, v, order[i])
		}
	}
}

func TestUnaryInterceptor_Info(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	var capturedInfo *UnaryServerInfo

	interceptor := func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error) {
		capturedInfo = info
		return handler(ctx, req)
	}

	s.AddUnaryInterceptor(interceptor)

	ctx := context.Background()
	s.Invoke(ctx, "EchoService", "Echo", "test")

	if capturedInfo == nil {
		t.Fatal("expected info to be captured")
	}
	if capturedInfo.FullMethod != "/EchoService/Echo" {
		t.Errorf("expected full method /EchoService/Echo, got %s", capturedInfo.FullMethod)
	}
	if capturedInfo.ServiceName != "EchoService" {
		t.Errorf("expected service name EchoService, got %s", capturedInfo.ServiceName)
	}
	if capturedInfo.MethodName != "Echo" {
		t.Errorf("expected method name Echo, got %s", capturedInfo.MethodName)
	}
}

func TestAddUnaryInterceptor_Nil(t *testing.T) {
	s := NewServer()
	err := s.AddUnaryInterceptor(nil)
	if err == nil {
		t.Error("expected error for nil interceptor")
	}
}

func TestAddUnaryInterceptor_ServerStopped(t *testing.T) {
	s := NewServer()
	s.Stop()

	interceptor := func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error) {
		return handler(ctx, req)
	}

	err := s.AddUnaryInterceptor(interceptor)
	if !errors.Is(err, ErrServerStopped) {
		t.Errorf("expected ErrServerStopped, got %v", err)
	}
}

func TestStreamInterceptor(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	var order []string

	interceptor1 := func(srv interface{}, ss Stream, info *StreamServerInfo, handler StreamHandler) error {
		order = append(order, "before1")
		err := handler(srv, ss)
		order = append(order, "after1")
		return err
	}

	interceptor2 := func(srv interface{}, ss Stream, info *StreamServerInfo, handler StreamHandler) error {
		order = append(order, "before2")
		err := handler(srv, ss)
		order = append(order, "after2")
		return err
	}

	s.AddStreamInterceptor(interceptor1)
	s.AddStreamInterceptor(interceptor2)

	stream, err := s.NewStream(context.Background(), "EchoService", "ServerStream")
	if err != nil {
		t.Fatalf("unexpected error creating stream: %v", err)
	}

	err = s.HandleStream(context.Background(), "EchoService", "ServerStream", stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"before1", "before2", "after2", "after1"}
	if len(order) != len(expected) {
		t.Fatalf("expected order length %d, got %d", len(expected), len(order))
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("expected order[%d] = %s, got %s", i, v, order[i])
		}
	}
}

func TestStreamInterceptor_Info(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	var capturedInfo *StreamServerInfo

	interceptor := func(srv interface{}, ss Stream, info *StreamServerInfo, handler StreamHandler) error {
		capturedInfo = info
		return handler(srv, ss)
	}

	s.AddStreamInterceptor(interceptor)

	stream, _ := s.NewStream(context.Background(), "EchoService", "ServerStream")
	s.HandleStream(context.Background(), "EchoService", "ServerStream", stream)

	if capturedInfo == nil {
		t.Fatal("expected info to be captured")
	}
	if capturedInfo.FullMethod != "/EchoService/ServerStream" {
		t.Errorf("expected full method /EchoService/ServerStream, got %s", capturedInfo.FullMethod)
	}
	if !capturedInfo.IsServerStream {
		t.Error("expected IsServerStream to be true")
	}
	if capturedInfo.IsClientStream {
		t.Error("expected IsClientStream to be false")
	}
}

func TestAddStreamInterceptor_Nil(t *testing.T) {
	s := NewServer()
	err := s.AddStreamInterceptor(nil)
	if err == nil {
		t.Error("expected error for nil interceptor")
	}
}

func TestAddStreamInterceptor_ServerStopped(t *testing.T) {
	s := NewServer()
	s.Stop()

	interceptor := func(srv interface{}, ss Stream, info *StreamServerInfo, handler StreamHandler) error {
		return handler(srv, ss)
	}

	err := s.AddStreamInterceptor(interceptor)
	if !errors.Is(err, ErrServerStopped) {
		t.Errorf("expected ErrServerStopped, got %v", err)
	}
}

func TestDeadlineExceeded(t *testing.T) {
	s := NewServer()
	sd := &ServiceDesc{
		ServiceName: "SlowService",
		Methods: []MethodDesc{
			{
				MethodName: "SlowMethod",
				Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
					time.Sleep(100 * time.Millisecond)
					return "done", nil
				},
			},
		},
	}
	s.RegisterService(sd, &echoService{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond)

	_, err := s.Invoke(ctx, "SlowService", "SlowMethod", nil)
	if !errors.Is(err, ErrDeadlineExceeded) {
		t.Errorf("expected ErrDeadlineExceeded, got %v", err)
	}
}

func TestDeadline_ContinuousCheck(t *testing.T) {
	s := NewServer()

	handlerStarted := make(chan struct{})
	handlerDone := make(chan struct{})
	defer close(handlerDone)

	sd := &ServiceDesc{
		ServiceName: "LongBlockingService",
		Methods: []MethodDesc{
			{
				MethodName: "LongBlockingMethod",
				Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
					close(handlerStarted)
					time.Sleep(500 * time.Millisecond)
					return "done", nil
				},
			},
		},
	}
	s.RegisterService(sd, &echoService{})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	resultCh := make(chan error, 1)
	go func() {
		_, err := s.Invoke(ctx, "LongBlockingService", "LongBlockingMethod", nil)
		resultCh <- err
	}()

	select {
	case <-handlerStarted:
	case <-time.After(1 * time.Second):
		t.Fatal("handler did not start")
	}

	select {
	case err := <-resultCh:
		if !errors.Is(err, ErrDeadlineExceeded) {
			t.Errorf("expected ErrDeadlineExceeded for long blocking handler, got %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("expected deadline check to interrupt handler within ticker interval")
	}
}

func TestDeadline_StreamContinuousCheck(t *testing.T) {
	s := NewServer()

	handlerStarted := make(chan struct{})

	sd := &ServiceDesc{
		ServiceName: "LongStreamService",
		Streams: []StreamDesc{
			{
				StreamName:    "LongStream",
				ServerStreams: true,
				Handler: func(srv interface{}, stream Stream) error {
					close(handlerStarted)
					time.Sleep(500 * time.Millisecond)
					return stream.SendMsg("done")
				},
			},
		},
	}
	s.RegisterService(sd, &echoService{})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	stream, err := s.NewStream(ctx, "LongStreamService", "LongStream")
	if err != nil {
		t.Fatalf("unexpected error creating stream: %v", err)
	}

	resultCh := make(chan error, 1)
	go func() {
		err := s.HandleStream(ctx, "LongStreamService", "LongStream", stream)
		resultCh <- err
	}()

	select {
	case <-handlerStarted:
	case <-time.After(1 * time.Second):
		t.Fatal("handler did not start")
	}

	select {
	case err := <-resultCh:
		if !errors.Is(err, ErrDeadlineExceeded) {
			t.Errorf("expected ErrDeadlineExceeded for long stream handler, got %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("expected deadline check to interrupt stream handler within ticker interval")
	}
}

func TestDeadline_ImmediateExceeded(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, err := s.Invoke(ctx, "EchoService", "Echo", "test")
	if !errors.Is(err, ErrDeadlineExceeded) {
		t.Errorf("expected ErrDeadlineExceeded, got %v", err)
	}
}

func TestDeadline_NoDeadline(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	ctx := context.Background()
	resp, err := s.Invoke(ctx, "EchoService", "Echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "hello" {
		t.Errorf("expected hello, got %v", resp)
	}
}

func TestMetadata(t *testing.T) {
	md := NewMD()
	md.Set("key1", "value1")
	md.Add("key2", "value2a")
	md.Add("key2", "value2b")

	if md.Len() != 2 {
		t.Errorf("expected 2 keys, got %d", md.Len())
	}

	vals := md.Get("key1")
	if len(vals) != 1 || vals[0] != "value1" {
		t.Errorf("expected [value1], got %v", vals)
	}

	vals = md.Get("key2")
	if len(vals) != 2 {
		t.Errorf("expected 2 values for key2, got %d", len(vals))
	}
}

func TestMetadata_Copy(t *testing.T) {
	md := NewMD()
	md.Set("key1", "value1")

	cp := md.Copy()
	cp.Set("key2", "value2")

	if md.Len() != 1 {
		t.Errorf("expected original to have 1 key, got %d", md.Len())
	}
	if cp.Len() != 2 {
		t.Errorf("expected copy to have 2 keys, got %d", cp.Len())
	}
}

func TestMetadata_Delete(t *testing.T) {
	md := NewMD()
	md.Set("key1", "value1")
	md.Set("key2", "value2")

	md.Delete("key1")

	if md.Len() != 1 {
		t.Errorf("expected 1 key, got %d", md.Len())
	}
	if _, ok := md["key1"]; ok {
		t.Error("expected key1 to be deleted")
	}
}

func TestMetadata_Nil(t *testing.T) {
	var md MD
	if md.Get("key") != nil {
		t.Error("expected nil from nil MD")
	}
	if md.Len() != 0 {
		t.Errorf("expected 0 length from nil MD, got %d", md.Len())
	}
}

func TestMetadataContext(t *testing.T) {
	md := NewMD()
	md.Set("x-request-id", "12345")

	ctx := NewContextWithMD(context.Background(), md)

	result, ok := FromContext(ctx)
	if !ok {
		t.Fatal("expected metadata in context")
	}
	if result.Get("x-request-id")[0] != "12345" {
		t.Errorf("expected 12345, got %s", result.Get("x-request-id")[0])
	}
}

func TestMetadataContext_NoMetadata(t *testing.T) {
	_, ok := FromContext(context.Background())
	if ok {
		t.Error("expected no metadata in context")
	}
}

func TestWithIncomingMetadata(t *testing.T) {
	md := NewMD()
	md.Set("authorization", "Bearer token")

	ctx := WithIncomingMetadata(context.Background(), md)

	result, ok := GetIncomingMetadata(ctx)
	if !ok {
		t.Fatal("expected metadata in context")
	}
	if result.Get("authorization")[0] != "Bearer token" {
		t.Errorf("expected Bearer token, got %s", result.Get("authorization")[0])
	}
}

func TestTrailer(t *testing.T) {
	s := NewServer()
	sd := &ServiceDesc{
		ServiceName: "TrailerService",
		Methods: []MethodDesc{
			{
				MethodName: "Method",
				Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
					trailer := NewMD()
					trailer.Set("x-trace-id", "trace-123")
					SetTrailer(ctx, trailer)
					return "ok", nil
				},
			},
		},
	}
	s.RegisterService(sd, &echoService{})

	ctx := NewContextWithTrailer(context.Background())
	resp, err := s.Invoke(ctx, "TrailerService", "Method", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Errorf("expected ok, got %v", resp)
	}

	trailer, ok := TrailerFromContext(ctx)
	if !ok {
		t.Fatal("expected trailer in context")
	}
	if trailer.Get("x-trace-id")[0] != "trace-123" {
		t.Errorf("expected trace-123, got %s", trailer.Get("x-trace-id")[0])
	}
}

func TestHeader(t *testing.T) {
	s := NewServer()
	sd := &ServiceDesc{
		ServiceName: "HeaderService",
		Methods: []MethodDesc{
			{
				MethodName: "Method",
				Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
					header := NewMD()
					header.Set("x-custom", "custom-value")
					SetHeader(ctx, header)
					return "ok", nil
				},
			},
		},
	}
	s.RegisterService(sd, &echoService{})

	ctx := NewContextWithHeader(context.Background())
	resp, err := s.Invoke(ctx, "HeaderService", "Method", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Errorf("expected ok, got %v", resp)
	}

	header, ok := HeaderFromContext(ctx)
	if !ok {
		t.Fatal("expected header in context")
	}
	if header.Get("x-custom")[0] != "custom-value" {
		t.Errorf("expected custom-value, got %s", header.Get("x-custom")[0])
	}
}

func TestStreamHeader(t *testing.T) {
	s := NewServer()
	sd := &ServiceDesc{
		ServiceName: "StreamHeaderService",
		Streams: []StreamDesc{
			{
				StreamName:    "Stream",
				ServerStreams: true,
				Handler: func(srv interface{}, stream Stream) error {
					header := NewMD()
					header.Set("x-stream-header", "stream-value")
					stream.SetHeader(header)
					return stream.SendMsg("hello")
				},
			},
		},
	}
	s.RegisterService(sd, &echoService{})

	stream, err := s.NewStream(context.Background(), "StreamHeaderService", "Stream")
	if err != nil {
		t.Fatalf("unexpected error creating stream: %v", err)
	}

	err = s.HandleStream(context.Background(), "StreamHeaderService", "Stream", stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	header, ok := stream.Header()
	if !ok {
		t.Fatal("expected header in stream")
	}
	if header.Get("x-stream-header")[0] != "stream-value" {
		t.Errorf("expected stream-value, got %s", header.Get("x-stream-header")[0])
	}

	ctxHeader, ok := HeaderFromContext(stream.Context())
	if !ok {
		t.Fatal("expected header in context")
	}
	if ctxHeader.Get("x-stream-header")[0] != "stream-value" {
		t.Errorf("expected stream-value in context, got %s", ctxHeader.Get("x-stream-header")[0])
	}
}

func TestServerStream(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	ctx := context.Background()
	stream, err := s.NewStream(ctx, "EchoService", "ServerStream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stream == nil {
		t.Fatal("expected non-nil stream")
	}

	err = s.HandleStream(ctx, "EchoService", "ServerStream", stream)
	if err != nil {
		t.Fatalf("unexpected error handling stream: %v", err)
	}
}

func TestClientStream_InterfaceIntegrity(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	ctx := context.Background()
	stream, err := s.NewStream(ctx, "EchoService", "ClientStream")
	if err != nil {
		t.Fatalf("unexpected error creating stream: %v", err)
	}

	var streamInterface Stream = stream

	handlerDone := make(chan error, 1)
	go func() {
		handlerDone <- s.HandleStream(ctx, "EchoService", "ClientStream", stream)
	}()

	for i := 0; i < 5; i++ {
		err := streamInterface.PutRecv(fmt.Sprintf("msg-%d", i))
		if err != nil {
			t.Fatalf("PutRecv through Stream interface failed: %v", err)
		}
	}

	resp, err := streamInterface.Send()
	if err != nil {
		t.Fatalf("Send through Stream interface failed: %v", err)
	}
	if resp != "received 5 messages" {
		t.Errorf("expected 'received 5 messages', got %v", resp)
	}

	stream.Close()

	select {
	case err := <-handlerDone:
		if err != nil && !errors.Is(err, ErrStreamClosed) {
			t.Fatalf("unexpected handler error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("handler did not complete")
	}
}

func TestBidiStream_InterfaceIntegrity(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	ctx := context.Background()
	stream, err := s.NewStream(ctx, "EchoService", "BidiStream")
	if err != nil {
		t.Fatalf("unexpected error creating stream: %v", err)
	}

	var streamInterface Stream = stream

	handlerDone := make(chan error, 1)
	go func() {
		handlerDone <- s.HandleStream(ctx, "EchoService", "BidiStream", stream)
	}()

	for i := 0; i < 3; i++ {
		err := streamInterface.PutRecv(fmt.Sprintf("ping-%d", i))
		if err != nil {
			t.Fatalf("PutRecv through Stream interface failed: %v", err)
		}

		msg, err := streamInterface.Send()
		if err != nil {
			t.Fatalf("Send through Stream interface failed: %v", err)
		}
		if msg != "echo" {
			t.Errorf("expected echo, got %v", msg)
		}
	}

	stream.Close()

	select {
	case err := <-handlerDone:
		if err != nil {
			t.Fatalf("unexpected handler error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("handler did not complete")
	}
}

func TestNewStream_ServiceNotFound(t *testing.T) {
	s := NewServer()
	_, err := s.NewStream(context.Background(), "NonExistent", "Stream")
	if !errors.Is(err, ErrServiceNotFound) {
		t.Errorf("expected ErrServiceNotFound, got %v", err)
	}
}

func TestNewStream_StreamNotFound(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	_, err := s.NewStream(context.Background(), "EchoService", "NonExistent")
	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("expected ErrMethodNotFound, got %v", err)
	}
}

func TestNewStream_ServerStopped(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})
	s.Stop()

	_, err := s.NewStream(context.Background(), "EchoService", "ServerStream")
	if !errors.Is(err, ErrServerStopped) {
		t.Errorf("expected ErrServerStopped, got %v", err)
	}
}

func TestNewStream_TooManyConcurrent(t *testing.T) {
	opts := ServerOptions{
		MaxConcurrentStreams: 2,
		ConnectionTimeout:    10 * time.Second,
	}
	s := NewServerWithOptions(opts)
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	stream1, err := s.NewStream(context.Background(), "EchoService", "ServerStream")
	if err != nil {
		t.Fatalf("failed to create stream 1: %v", err)
	}
	defer stream1.Close()

	stream2, err := s.NewStream(context.Background(), "EchoService", "ServerStream")
	if err != nil {
		t.Fatalf("failed to create stream 2: %v", err)
	}
	defer stream2.Close()

	if s.ActiveStreams() != 2 {
		t.Errorf("expected 2 active streams, got %d", s.ActiveStreams())
	}

	_, err = s.NewStream(context.Background(), "EchoService", "ServerStream")
	if !errors.Is(err, ErrTooManyStreams) {
		t.Errorf("expected ErrTooManyStreams, got %v", err)
	}

	stream2.Close()
	time.Sleep(10 * time.Millisecond)

	if s.ActiveStreams() != 1 {
		t.Errorf("expected 1 active stream after closing, got %d", s.ActiveStreams())
	}

	stream3, err := s.NewStream(context.Background(), "EchoService", "ServerStream")
	if err != nil {
		t.Fatalf("failed to create stream 3 after closing: %v", err)
	}
	defer stream3.Close()
}

func TestHandleStream_ServiceNotFound(t *testing.T) {
	s := NewServer()
	stream, _ := s.NewStream(context.Background(), "EchoService", "ServerStream")
	err := s.HandleStream(context.Background(), "NonExistent", "Stream", stream)
	if !errors.Is(err, ErrServiceNotFound) {
		t.Errorf("expected ErrServiceNotFound, got %v", err)
	}
}

func TestHandleStream_StreamNotFound(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	stream, _ := s.NewStream(context.Background(), "EchoService", "ServerStream")
	err := s.HandleStream(context.Background(), "EchoService", "NonExistent", stream)
	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("expected ErrMethodNotFound, got %v", err)
	}
}

func TestHandleStream_ServerStopped(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})
	s.Stop()

	stream, _ := s.NewStream(context.Background(), "EchoService", "ServerStream")
	err := s.HandleStream(context.Background(), "EchoService", "ServerStream", stream)
	if !errors.Is(err, ErrServerStopped) {
		t.Errorf("expected ErrServerStopped, got %v", err)
	}
}

func TestStream_Close(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	stream, err := s.NewStream(context.Background(), "EchoService", "ServerStream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stream.Closed() {
		t.Error("expected stream to be open")
	}

	err = stream.Close()
	if err != nil {
		t.Fatalf("unexpected error closing stream: %v", err)
	}

	if !stream.Closed() {
		t.Error("expected stream to be closed")
	}

	err = stream.Close()
	if err != nil {
		t.Errorf("expected no error on double close, got %v", err)
	}
}

func TestStream_SendOnClosed(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	stream, _ := s.NewStream(context.Background(), "EchoService", "ServerStream")
	stream.Close()

	err := stream.SendMsg("test")
	if !errors.Is(err, ErrStreamClosed) {
		t.Errorf("expected ErrStreamClosed, got %v", err)
	}
}

func TestStream_RecvOnClosed(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	stream, _ := s.NewStream(context.Background(), "EchoService", "ServerStream")
	stream.Close()

	err := stream.RecvMsg(nil)
	if !errors.Is(err, ErrStreamClosed) {
		t.Errorf("expected ErrStreamClosed, got %v", err)
	}
}

func TestStream_PutRecvOnClosed(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	stream, _ := s.NewStream(context.Background(), "EchoService", "ServerStream")
	stream.Close()

	err := stream.PutRecv("test")
	if !errors.Is(err, ErrStreamClosed) {
		t.Errorf("expected ErrStreamClosed, got %v", err)
	}
}

func TestStream_SetHeader(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	stream, _ := s.NewStream(context.Background(), "EchoService", "ServerStream")

	md := NewMD()
	md.Set("x-custom", "value")
	stream.SetHeader(md)

	header, ok := stream.Header()
	if !ok {
		t.Fatal("expected header to be set")
	}
	if header.Get("x-custom")[0] != "value" {
		t.Errorf("expected value, got %s", header.Get("x-custom")[0])
	}
}

func TestStream_SetTrailer(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	stream, _ := s.NewStream(context.Background(), "EchoService", "ServerStream")

	md := NewMD()
	md.Set("x-status", "ok")
	stream.SetTrailer(md)
}

func TestStream_Context(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	ctx := context.Background()
	stream, _ := s.NewStream(ctx, "EchoService", "ServerStream")

	if stream.Context() == nil {
		t.Error("expected non-nil context")
	}
}

func TestServerStop(t *testing.T) {
	s := NewServer()
	if !s.IsRunning() {
		t.Error("expected server to be running")
	}

	s.Stop()
	if s.IsRunning() {
		t.Error("expected server to be stopped")
	}

	s.Stop()
	if s.IsRunning() {
		t.Error("expected server to remain stopped")
	}
}

func TestServiceCount(t *testing.T) {
	s := NewServer()
	if s.ServiceCount() != 0 {
		t.Errorf("expected 0 services, got %d", s.ServiceCount())
	}

	sd1 := &ServiceDesc{
		ServiceName: "Service1",
		Methods: []MethodDesc{
			{MethodName: "Method1", Handler: func(ctx context.Context, req interface{}) (interface{}, error) { return nil, nil }},
		},
	}
	s.RegisterService(sd1, &echoService{})
	if s.ServiceCount() != 1 {
		t.Errorf("expected 1 service, got %d", s.ServiceCount())
	}

	sd2 := &ServiceDesc{
		ServiceName: "Service2",
		Methods: []MethodDesc{
			{MethodName: "Method2", Handler: func(ctx context.Context, req interface{}) (interface{}, error) { return nil, nil }},
		},
	}
	s.RegisterService(sd2, &echoService{})
	if s.ServiceCount() != 2 {
		t.Errorf("expected 2 services, got %d", s.ServiceCount())
	}
}

func TestMethodCount(t *testing.T) {
	s := NewServer()
	if s.MethodCount("NonExistent") != 0 {
		t.Errorf("expected 0 methods for non-existent service, got %d", s.MethodCount("NonExistent"))
	}

	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	if count := s.MethodCount("EchoService"); count != 5 {
		t.Errorf("expected 5 methods (2 unary + 3 streams), got %d", count)
	}
}

func TestRPCTypeString(t *testing.T) {
	tests := []struct {
		rpcType  RPCType
		expected string
	}{
		{UnaryRPC, "Unary"},
		{ServerStreamingRPC, "ServerStreaming"},
		{ClientStreamingRPC, "ClientStreaming"},
		{BidiStreamingRPC, "BidiStreaming"},
		{RPCType(99), "Unknown"},
	}

	for _, tt := range tests {
		if tt.rpcType.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.rpcType.String())
		}
	}
}

func TestChainUnaryInterceptors(t *testing.T) {
	var order []string

	i1 := func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error) {
		order = append(order, "before1")
		resp, err := handler(ctx, req)
		order = append(order, "after1")
		return resp, err
	}

	i2 := func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error) {
		order = append(order, "before2")
		resp, err := handler(ctx, req)
		order = append(order, "after2")
		return resp, err
	}

	chain := ChainUnaryInterceptors(i1, i2)
	info := &UnaryServerInfo{FullMethod: "/test/test"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		order = append(order, "handler")
		return "result", nil
	}

	resp, err := chain(context.Background(), "req", info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "result" {
		t.Errorf("expected result, got %v", resp)
	}

	expected := []string{"before1", "before2", "handler", "after2", "after1"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d elements, got %d", len(expected), len(order))
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("order[%d]: expected %s, got %s", i, v, order[i])
		}
	}
}

func TestChainUnaryInterceptors_Empty(t *testing.T) {
	chain := ChainUnaryInterceptors()
	if chain != nil {
		t.Error("expected nil chain for empty interceptors")
	}
}

type mockStream struct{}

func (m *mockStream) Context() context.Context    { return context.Background() }
func (m *mockStream) SendMsg(msg interface{}) error { return nil }
func (m *mockStream) RecvMsg(msg interface{}) error { return nil }
func (m *mockStream) Recv() (interface{}, error)   { return nil, nil }
func (m *mockStream) Send() (interface{}, error)   { return nil, nil }
func (m *mockStream) PutRecv(msg interface{}) error { return nil }
func (m *mockStream) SetHeader(md MD)              {}
func (m *mockStream) SetTrailer(md MD)             {}
func (m *mockStream) Header() (MD, bool)            { return nil, false }
func (m *mockStream) Close() error                 { return nil }
func (m *mockStream) Closed() bool                 { return false }

func TestChainStreamInterceptors(t *testing.T) {
	var order []string

	i1 := func(srv interface{}, ss Stream, info *StreamServerInfo, handler StreamHandler) error {
		order = append(order, "before1")
		err := handler(srv, ss)
		order = append(order, "after1")
		return err
	}

	i2 := func(srv interface{}, ss Stream, info *StreamServerInfo, handler StreamHandler) error {
		order = append(order, "before2")
		err := handler(srv, ss)
		order = append(order, "after2")
		return err
	}

	chain := ChainStreamInterceptors(i1, i2)
	info := &StreamServerInfo{FullMethod: "/test/stream"}
	handler := func(srv interface{}, stream Stream) error {
		order = append(order, "handler")
		return nil
	}

	var ss Stream = &mockStream{}
	err := chain(nil, ss, info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"before1", "before2", "handler", "after2", "after1"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d elements, got %d", len(expected), len(order))
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("order[%d]: expected %s, got %s", i, v, order[i])
		}
	}
}

func TestChainStreamInterceptors_Empty(t *testing.T) {
	chain := ChainStreamInterceptors()
	if chain != nil {
		t.Error("expected nil chain for empty interceptors")
	}
}

func TestConcurrentInvoke(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx := context.Background()
			req := fmt.Sprintf("req-%d", idx)
			resp, err := s.Invoke(ctx, "EchoService", "Echo", req)
			if err != nil {
				t.Errorf("goroutine %d: unexpected error: %v", idx, err)
			}
			if resp != req {
				t.Errorf("goroutine %d: expected %s, got %v", idx, req, resp)
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrentRegister(t *testing.T) {
	s := NewServer()
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sd := &ServiceDesc{
				ServiceName: fmt.Sprintf("Service-%d", idx),
				Methods: []MethodDesc{
					{
						MethodName: "Method",
						Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
							return req, nil
						},
					},
				},
			}
			err := s.RegisterService(sd, &echoService{})
			if err != nil {
				t.Errorf("goroutine %d: unexpected error: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()

	if s.ServiceCount() != numGoroutines {
		t.Errorf("expected %d services, got %d", numGoroutines, s.ServiceCount())
	}
}

func TestDeadlineInStream(t *testing.T) {
	s := NewServer()
	sd := &ServiceDesc{
		ServiceName: "SlowStreamService",
		Streams: []StreamDesc{
			{
				StreamName:    "SlowStream",
				ServerStreams: true,
				Handler: func(srv interface{}, stream Stream) error {
					time.Sleep(100 * time.Millisecond)
					return stream.SendMsg("done")
				},
			},
		},
	}
	s.RegisterService(sd, &echoService{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	stream, err := s.NewStream(ctx, "SlowStreamService", "SlowStream")
	if err != nil {
		t.Fatalf("unexpected error creating stream: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	err = s.HandleStream(ctx, "SlowStreamService", "SlowStream", stream)
	if !errors.Is(err, ErrDeadlineExceeded) {
		t.Errorf("expected ErrDeadlineExceeded, got %v", err)
	}
}

func TestInterceptorWithMetadata(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	authInterceptor := func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error) {
		md, ok := FromContext(ctx)
		if !ok {
			return nil, errors.New("no metadata")
		}
		token := md.Get("authorization")
		if len(token) == 0 {
			return nil, errors.New("unauthorized")
		}
		return handler(ctx, req)
	}

	s.AddUnaryInterceptor(authInterceptor)

	md := NewMD()
	md.Set("authorization", "Bearer valid-token")
	ctx := NewContextWithMD(context.Background(), md)

	resp, err := s.Invoke(ctx, "EchoService", "Echo", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "test" {
		t.Errorf("expected test, got %v", resp)
	}
}

func TestInterceptorWithMetadata_Unauthorized(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	authInterceptor := func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error) {
		md, ok := FromContext(ctx)
		if !ok {
			return nil, errors.New("no metadata")
		}
		token := md.Get("authorization")
		if len(token) == 0 {
			return nil, errors.New("unauthorized")
		}
		return handler(ctx, req)
	}

	s.AddUnaryInterceptor(authInterceptor)

	ctx := context.Background()
	_, err := s.Invoke(ctx, "EchoService", "Echo", "test")
	if err == nil {
		t.Error("expected error for missing authorization")
	}
}

func TestDefaultServerOptions(t *testing.T) {
	opts := DefaultServerOptions()
	if opts.MaxConcurrentStreams != 100 {
		t.Errorf("expected 100 max concurrent streams, got %d", opts.MaxConcurrentStreams)
	}
	if opts.ConnectionTimeout != 30*time.Second {
		t.Errorf("expected 30s connection timeout, got %v", opts.ConnectionTimeout)
	}
}

func TestServerStream_RecvAndSend(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	stream, err := s.NewStream(context.Background(), "EchoService", "BidiStream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = stream.PutRecv("hello")
	if err != nil {
		t.Fatalf("unexpected error putting recv: %v", err)
	}

	msg, err := stream.Recv()
	if err != nil {
		t.Fatalf("unexpected error receiving: %v", err)
	}
	if msg != "hello" {
		t.Errorf("expected hello, got %v", msg)
	}
}

func TestServerStream_RecvOnClosedChannel(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	stream, _ := s.NewStream(context.Background(), "EchoService", "BidiStream")
	stream.Close()

	_, err := stream.Recv()
	if !errors.Is(err, ErrStreamClosed) {
		t.Errorf("expected ErrStreamClosed, got %v", err)
	}
}

func TestConnectionTimeout(t *testing.T) {
	opts := ServerOptions{
		MaxConcurrentStreams: 100,
		ConnectionTimeout:    10 * time.Millisecond,
	}
	s := NewServerWithOptions(opts)

	sd := &ServiceDesc{
		ServiceName: "TimeoutService",
		Methods: []MethodDesc{
			{
				MethodName: "Method",
				Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
					time.Sleep(50 * time.Millisecond)
					return "done", nil
				},
			},
		},
	}
	s.RegisterService(sd, &echoService{})

	_, err := s.Invoke(context.Background(), "TimeoutService", "Method", nil)
	if !errors.Is(err, ErrDeadlineExceeded) {
		t.Errorf("expected ErrDeadlineExceeded from connection timeout, got %v", err)
	}
}

func TestConnectionTimeout_Stream(t *testing.T) {
	opts := ServerOptions{
		MaxConcurrentStreams: 100,
		ConnectionTimeout:    10 * time.Millisecond,
	}
	s := NewServerWithOptions(opts)

	sd := &ServiceDesc{
		ServiceName: "TimeoutStreamService",
		Streams: []StreamDesc{
			{
				StreamName:    "Stream",
				ServerStreams: true,
				Handler: func(srv interface{}, stream Stream) error {
					time.Sleep(50 * time.Millisecond)
					return stream.SendMsg("done")
				},
			},
		},
	}
	s.RegisterService(sd, &echoService{})

	stream, err := s.NewStream(context.Background(), "TimeoutStreamService", "Stream")
	if err != nil {
		t.Fatalf("unexpected error creating stream: %v", err)
	}

	err = s.HandleStream(context.Background(), "TimeoutStreamService", "Stream", stream)
	if !errors.Is(err, ErrDeadlineExceeded) {
		t.Errorf("expected ErrDeadlineExceeded from connection timeout, got %v", err)
	}
}

func TestMaxConcurrentStreams(t *testing.T) {
	opts := ServerOptions{
		MaxConcurrentStreams: 3,
		ConnectionTimeout:    30 * time.Second,
	}
	s := NewServerWithOptions(opts)
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	streams := make([]Stream, 0, 3)
	for i := 0; i < 3; i++ {
		stream, err := s.NewStream(context.Background(), "EchoService", "ServerStream")
		if err != nil {
			t.Fatalf("failed to create stream %d: %v", i, err)
		}
		streams = append(streams, stream)
		defer stream.Close()
	}

	if s.ActiveStreams() != 3 {
		t.Errorf("expected 3 active streams, got %d", s.ActiveStreams())
	}

	_, err := s.NewStream(context.Background(), "EchoService", "ServerStream")
	if !errors.Is(err, ErrTooManyStreams) {
		t.Errorf("expected ErrTooManyStreams, got %v", err)
	}

	streams[0].Close()
	time.Sleep(10 * time.Millisecond)

	if s.ActiveStreams() != 2 {
		t.Errorf("expected 2 active streams after closing, got %d", s.ActiveStreams())
	}

	stream4, err := s.NewStream(context.Background(), "EchoService", "ServerStream")
	if err != nil {
		t.Fatalf("failed to create stream after releasing slot: %v", err)
	}
	stream4.Close()
}

func TestStream_RecvInterface(t *testing.T) {
	s := NewServer()
	sd := newEchoServiceDesc()
	s.RegisterService(sd, &echoService{})

	stream, err := s.NewStream(context.Background(), "EchoService", "BidiStream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var streamInterface Stream = stream

	err = streamInterface.PutRecv("test")
	if err != nil {
		t.Fatalf("PutRecv through Stream interface failed: %v", err)
	}

	msg, err := streamInterface.Recv()
	if err != nil {
		t.Fatalf("Recv through Stream interface failed: %v", err)
	}
	if msg != "test" {
		t.Errorf("expected test, got %v", msg)
	}
}

func TestServerOptions_Options(t *testing.T) {
	opts := ServerOptions{
		MaxConcurrentStreams: 50,
		ConnectionTimeout:    15 * time.Second,
	}
	s := NewServerWithOptions(opts)

	got := s.Options()
	if got.MaxConcurrentStreams != 50 {
		t.Errorf("expected MaxConcurrentStreams 50, got %d", got.MaxConcurrentStreams)
	}
	if got.ConnectionTimeout != 15*time.Second {
		t.Errorf("expected ConnectionTimeout 15s, got %v", got.ConnectionTimeout)
	}
}
