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
		HandlerType: svc,
		Methods: []MethodDesc{
			{
				MethodName: "Echo",
				Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
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
						err := stream.RecvMsg(nil)
						if err != nil {
							if errors.Is(err, ErrStreamClosed) {
								break
							}
							return err
						}
						count++
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
						err := stream.RecvMsg(nil)
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

	interceptor1 := func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
		order = append(order, "before1")
		err := handler(srv, ss)
		order = append(order, "after1")
		return err
	}

	interceptor2 := func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
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

	interceptor := func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
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

	interceptor := func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
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
	sd := newEchoService_desc()
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

func TestServerStream(t *testing.T) {
	s := NewServer()
	sd := newEchoService_desc()
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

func TestNewStream_ServiceNotFound(t *testing.T) {
	s := NewServer()
	_, err := s.NewStream(context.Background(), "NonExistent", "Stream")
	if !errors.Is(err, ErrServiceNotFound) {
		t.Errorf("expected ErrServiceNotFound, got %v", err)
	}
}

func TestNewStream_StreamNotFound(t *testing.T) {
	s := NewServer()
	sd := newEchoService_desc()
	s.RegisterService(sd, &echoService{})

	_, err := s.NewStream(context.Background(), "EchoService", "NonExistent")
	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("expected ErrMethodNotFound, got %v", err)
	}
}

func TestNewStream_ServerStopped(t *testing.T) {
	s := NewServer()
	sd := newEchoService_desc()
	s.RegisterService(sd, &echoService{})
	s.Stop()

	_, err := s.NewStream(context.Background(), "EchoService", "ServerStream")
	if !errors.Is(err, ErrServerStopped) {
		t.Errorf("expected ErrServerStopped, got %v", err)
	}
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
	sd := newEchoService_desc()
	s.RegisterService(sd, &echoService{})

	stream, _ := s.NewStream(context.Background(), "EchoService", "ServerStream")
	err := s.HandleStream(context.Background(), "EchoService", "NonExistent", stream)
	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("expected ErrMethodNotFound, got %v", err)
	}
}

func TestHandleStream_ServerStopped(t *testing.T) {
	s := NewServer()
	sd := newEchoService_desc()
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
	sd := newEchoService_desc()
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
	sd := newEchoService_desc()
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
	sd := newEchoService_desc()
	s.RegisterService(sd, &echoService{})

	stream, _ := s.NewStream(context.Background(), "EchoService", "ServerStream")
	stream.Close()

	err := stream.RecvMsg(nil)
	if !errors.Is(err, ErrStreamClosed) {
		t.Errorf("expected ErrStreamClosed, got %v", err)
	}
}

func TestStream_SetHeader(t *testing.T) {
	s := NewServer()
	sd := newEchoService_desc()
	s.RegisterService(sd, &echoService{})

	stream, _ := s.NewStream(context.Background(), "EchoService", "ServerStream")

	md := NewMD()
	md.Set("x-custom", "value")
	stream.SetHeader(md)
}

func TestStream_SetTrailer(t *testing.T) {
	s := NewServer()
	sd := newEchoService_desc()
	s.RegisterService(sd, &echoService{})

	stream, _ := s.NewStream(context.Background(), "EchoService", "ServerStream")

	md := NewMD()
	md.Set("x-status", "ok")
	stream.SetTrailer(md)
}

func TestStream_Context(t *testing.T) {
	s := NewServer()
	sd := newEchoService_desc()
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

	sd := newEchoService_desc()
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

type mockServerStream struct{}

func (m *mockServerStream) Context() context.Context    { return context.Background() }
func (m *mockServerStream) SendMsg(msg interface{}) error { return nil }
func (m *mockServerStream) RecvMsg(msg interface{}) error { return nil }
func (m *mockServerStream) SetHeader(md MD)              {}
func (m *mockServerStream) SetTrailer(md MD)             {}
func (m *mockServerStream) Close() error                 { return nil }
func (m *mockServerStream) Closed() bool                 { return false }

func TestChainStreamInterceptors(t *testing.T) {
	var order []string

	i1 := func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
		order = append(order, "before1")
		err := handler(srv, ss)
		order = append(order, "after1")
		return err
	}

	i2 := func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
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

	var ss ServerStream = &mockServerStream{}
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
	sd := newEchoService_desc()
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
	sd := newEchoService_desc()
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
	sd := newEchoService_desc()
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
	sd := newEchoService_desc()
	s.RegisterService(sd, &echoService{})

	stream, err := s.NewStream(context.Background(), "EchoService", "BidiStream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ss := stream.(*serverStream)

	err = ss.PutRecv("hello")
	if err != nil {
		t.Fatalf("unexpected error putting recv: %v", err)
	}

	msg, err := ss.Recv()
	if err != nil {
		t.Fatalf("unexpected error receiving: %v", err)
	}
	if msg != "hello" {
		t.Errorf("expected hello, got %v", msg)
	}
}

func TestServerStream_RecvOnClosedChannel(t *testing.T) {
	s := NewServer()
	sd := newEchoService_desc()
	s.RegisterService(sd, &echoService{})

	stream, _ := s.NewStream(context.Background(), "EchoService", "BidiStream")
	ss := stream.(*serverStream)
	stream.Close()

	_, err := ss.Recv()
	if !errors.Is(err, ErrStreamClosed) {
		t.Errorf("expected ErrStreamClosed, got %v", err)
	}
}

func TestServerStream_PutRecvOnClosed(t *testing.T) {
	s := NewServer()
	sd := newEchoService_desc()
	s.RegisterService(sd, &echoService{})

	stream, _ := s.NewStream(context.Background(), "EchoService", "BidiStream")
	ss := stream.(*serverStream)
	stream.Close()

	err := ss.PutRecv("test")
	if !errors.Is(err, ErrStreamClosed) {
		t.Errorf("expected ErrStreamClosed, got %v", err)
	}
}

func newEchoService_desc() *ServiceDesc {
	return newEchoServiceDesc()
}
