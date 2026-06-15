package grpcsvc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrServiceNotFound    = errors.New("grpcsvc: service not found")
	ErrServiceExists      = errors.New("grpcsvc: service already exists")
	ErrMethodNotFound     = errors.New("grpcsvc: method not found")
	ErrInvalidServiceDesc = errors.New("grpcsvc: invalid service descriptor")
	ErrInvalidMethodDesc  = errors.New("grpcsvc: invalid method descriptor")
	ErrServerStopped      = errors.New("grpcsvc: server is stopped")
	ErrDeadlineExceeded   = errors.New("grpcsvc: deadline exceeded")
	ErrStreamClosed       = errors.New("grpcsvc: stream is closed")
	ErrNilHandler         = errors.New("grpcsvc: handler cannot be nil")
)

const (
	UnaryRPC        RPCType = iota
	ServerStreamingRPC
	ClientStreamingRPC
	BidiStreamingRPC
)

type RPCType int

func (t RPCType) String() string {
	switch t {
	case UnaryRPC:
		return "Unary"
	case ServerStreamingRPC:
		return "ServerStreaming"
	case ClientStreamingRPC:
		return "ClientStreaming"
	case BidiStreamingRPC:
		return "BidiStreaming"
	default:
		return "Unknown"
	}
}

type MD map[string][]string

func NewMD() MD {
	return make(MD)
}

func (m MD) Get(key string) []string {
	if m == nil {
		return nil
	}
	return m[key]
}

func (m MD) Set(key, value string) {
	m[key] = []string{value}
}

func (m MD) Add(key, value string) {
	m[key] = append(m[key], value)
}

func (m MD) Delete(key string) {
	delete(m, key)
}

func (m MD) Len() int {
	return len(m)
}

func (m MD) Copy() MD {
	cp := make(MD, len(m))
	for k, v := range m {
		cp[k] = make([]string, len(v))
		copy(cp[k], v)
	}
	return cp
}

type mdKey struct{}

func NewContextWithMD(ctx context.Context, md MD) context.Context {
	return context.WithValue(ctx, mdKey{}, md)
}

func FromContext(ctx context.Context) (MD, bool) {
	md, ok := ctx.Value(mdKey{}).(MD)
	return md, ok
}

type trailerKey struct{}

func NewContextWithTrailer(ctx context.Context) context.Context {
	var trailer MD
	return context.WithValue(ctx, trailerKey{}, &trailer)
}

func SetTrailer(ctx context.Context, md MD) {
	if trailer, ok := ctx.Value(trailerKey{}).(*MD); ok {
		if *trailer == nil {
			*trailer = NewMD()
		}
		for k, v := range md {
			(*trailer)[k] = v
		}
	}
}

func TrailerFromContext(ctx context.Context) (MD, bool) {
	md, ok := ctx.Value(trailerKey{}).(*MD)
	if !ok || md == nil || *md == nil {
		return nil, false
	}
	return (*md).Copy(), true
}

type Stream interface {
	Context() context.Context
	SendMsg(msg interface{}) error
	RecvMsg(msg interface{}) error
	SetHeader(md MD)
	SetTrailer(md MD)
	Close() error
	Closed() bool
}

type ServerStream interface {
	Stream
}

type ClientStream interface {
	Stream
}

type UnaryHandler func(ctx context.Context, req interface{}) (interface{}, error)

type StreamHandler func(srv interface{}, stream Stream) error

type UnaryInterceptor func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error)

type StreamInterceptor func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error

type UnaryServerInfo struct {
	Server      interface{}
	FullMethod  string
	ServiceName string
	MethodName  string
}

type StreamServerInfo struct {
	Server      interface{}
	FullMethod  string
	ServiceName string
	MethodName  string
	IsClientStream bool
	IsServerStream bool
}

type MethodDesc struct {
	MethodName  string
	Handler     UnaryHandler
}

type StreamDesc struct {
	StreamName   string
	Handler      StreamHandler
	ServerStreams bool
	ClientStreams bool
}

type ServiceDesc struct {
	ServiceName string
	HandlerType interface{}
	Methods     []MethodDesc
	Streams     []StreamDesc
	Metadata    interface{}
}

type service struct {
	name    string
	impl    interface{}
	methods map[string]*MethodDesc
	streams map[string]*StreamDesc
}

type Server struct {
	mu              sync.RWMutex
	services        map[string]*service
	unaryInterceptors  []UnaryInterceptor
	streamInterceptors []StreamInterceptor
	running         bool
	options         ServerOptions
}

type ServerOptions struct {
	MaxConcurrentStreams uint32
	ConnectionTimeout    time.Duration
}

func DefaultServerOptions() ServerOptions {
	return ServerOptions{
		MaxConcurrentStreams: 100,
		ConnectionTimeout:    30 * time.Second,
	}
}

func NewServer() *Server {
	return NewServerWithOptions(DefaultServerOptions())
}

func NewServerWithOptions(opts ServerOptions) *Server {
	return &Server{
		services:        make(map[string]*service),
		unaryInterceptors:  make([]UnaryInterceptor, 0),
		streamInterceptors: make([]StreamInterceptor, 0),
		running:         true,
		options:         opts,
	}
}

func (s *Server) RegisterService(sd *ServiceDesc, srv interface{}) error {
	if sd == nil {
		return ErrInvalidServiceDesc
	}
	if sd.ServiceName == "" {
		return ErrInvalidServiceDesc
	}
	if srv == nil {
		return ErrInvalidServiceDesc
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return ErrServerStopped
	}

	if _, exists := s.services[sd.ServiceName]; exists {
		return ErrServiceExists
	}

	svc := &service{
		name:    sd.ServiceName,
		impl:    srv,
		methods: make(map[string]*MethodDesc),
		streams: make(map[string]*StreamDesc),
	}

	for i := range sd.Methods {
		m := &sd.Methods[i]
		if m.MethodName == "" {
			return ErrInvalidMethodDesc
		}
		if m.Handler == nil {
			return ErrNilHandler
		}
		svc.methods[m.MethodName] = m
	}

	for i := range sd.Streams {
		st := &sd.Streams[i]
		if st.StreamName == "" {
			return ErrInvalidMethodDesc
		}
		if st.Handler == nil {
			return ErrNilHandler
		}
		svc.streams[st.StreamName] = st
	}

	s.services[sd.ServiceName] = svc
	return nil
}

func (s *Server) AddUnaryInterceptor(interceptor UnaryInterceptor) error {
	if interceptor == nil {
		return errors.New("grpcsvc: interceptor cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return ErrServerStopped
	}

	s.unaryInterceptors = append(s.unaryInterceptors, interceptor)
	return nil
}

func (s *Server) AddStreamInterceptor(interceptor StreamInterceptor) error {
	if interceptor == nil {
		return errors.New("grpcsvc: interceptor cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return ErrServerStopped
	}

	s.streamInterceptors = append(s.streamInterceptors, interceptor)
	return nil
}

func (s *Server) UnaryInterceptorChain() UnaryInterceptor {
	s.mu.RLock()
	defer s.mu.RUnlock()

	interceptors := make([]UnaryInterceptor, len(s.unaryInterceptors))
	copy(interceptors, s.unaryInterceptors)

	return func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error) {
		chain := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			next := chain
			chain = func(currentCtx context.Context, currentReq interface{}) (interface{}, error) {
				return interceptor(currentCtx, currentReq, info, next)
			}
		}
		return chain(ctx, req)
	}
}

func (s *Server) StreamInterceptorChain() StreamInterceptor {
	s.mu.RLock()
	defer s.mu.RUnlock()

	interceptors := make([]StreamInterceptor, len(s.streamInterceptors))
	copy(interceptors, s.streamInterceptors)

	return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
		chain := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			next := chain
			chain = func(currentSrv interface{}, currentStream Stream) error {
				var cs ServerStream = currentStream.(ServerStream)
				return interceptor(currentSrv, cs, info, next)
			}
		}
		return chain(srv, ss)
	}
}

func (s *Server) Invoke(ctx context.Context, serviceName, methodName string, req interface{}) (interface{}, error) {
	s.mu.RLock()
	if !s.running {
		s.mu.RUnlock()
		return nil, ErrServerStopped
	}

	svc, ok := s.services[serviceName]
	if !ok {
		s.mu.RUnlock()
		return nil, ErrServiceNotFound
	}

	md, ok := svc.methods[methodName]
	if !ok {
		s.mu.RUnlock()
		return nil, ErrMethodNotFound
	}

	chain := s.UnaryInterceptorChain()
	impl := svc.impl
	s.mu.RUnlock()

	if err := checkDeadline(ctx); err != nil {
		return nil, err
	}

	if _, ok := ctx.Value(trailerKey{}).(*MD); !ok {
		ctx = NewContextWithTrailer(ctx)
	}

	info := &UnaryServerInfo{
		Server:      impl,
		FullMethod:  fmt.Sprintf("/%s/%s", serviceName, methodName),
		ServiceName: serviceName,
		MethodName:  methodName,
	}

	resp, err := chain(ctx, req, info, func(ctx context.Context, req interface{}) (interface{}, error) {
		if err := checkDeadline(ctx); err != nil {
			return nil, err
		}
		return md.Handler(ctx, req)
	})

	return resp, err
}

func (s *Server) NewStream(ctx context.Context, serviceName, streamName string) (Stream, error) {
	s.mu.RLock()
	if !s.running {
		s.mu.RUnlock()
		return nil, ErrServerStopped
	}

	svc, ok := s.services[serviceName]
	if !ok {
		s.mu.RUnlock()
		return nil, ErrServiceNotFound
	}

	sd, ok := svc.streams[streamName]
	if !ok {
		s.mu.RUnlock()
		return nil, ErrMethodNotFound
	}
	s.mu.RUnlock()

	if err := checkDeadline(ctx); err != nil {
		return nil, err
	}

	if _, ok := ctx.Value(trailerKey{}).(*MD); !ok {
		ctx = NewContextWithTrailer(ctx)
	}

	stream := &serverStream{
		ctx:       ctx,
		sendCh:    make(chan interface{}, 64),
		recvCh:    make(chan interface{}, 64),
		header:    NewMD(),
		trailer:   NewMD(),
		streamDesc: sd,
	}

	return stream, nil
}

func (s *Server) HandleStream(ctx context.Context, serviceName, streamName string, stream Stream) error {
	s.mu.RLock()
	if !s.running {
		s.mu.RUnlock()
		return ErrServerStopped
	}

	svc, ok := s.services[serviceName]
	if !ok {
		s.mu.RUnlock()
		return ErrServiceNotFound
	}

	sd, ok := svc.streams[streamName]
	if !ok {
		s.mu.RUnlock()
		return ErrMethodNotFound
	}

	chain := s.StreamInterceptorChain()
	impl := svc.impl
	s.mu.RUnlock()

	if err := checkDeadline(ctx); err != nil {
		return err
	}

	info := &StreamServerInfo{
		Server:         impl,
		FullMethod:     fmt.Sprintf("/%s/%s", serviceName, streamName),
		ServiceName:    serviceName,
		MethodName:     streamName,
		IsClientStream: sd.ClientStreams,
		IsServerStream: sd.ServerStreams,
	}

	var ss ServerStream = stream
	handler := func(srv interface{}, st Stream) error {
		if err := checkDeadline(st.Context()); err != nil {
			return err
		}
		return sd.Handler(srv, st)
	}
	err := chain(impl, ss, info, handler)

	return err
}

func (s *Server) GetService(serviceName string) (*ServiceDesc, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	svc, ok := s.services[serviceName]
	if !ok {
		return nil, false
	}

	sd := &ServiceDesc{
		ServiceName: svc.name,
		Methods:     make([]MethodDesc, 0, len(svc.methods)),
		Streams:     make([]StreamDesc, 0, len(svc.streams)),
	}

	for _, m := range svc.methods {
		sd.Methods = append(sd.Methods, *m)
	}
	for _, st := range svc.streams {
		sd.Streams = append(sd.Streams, *st)
	}

	return sd, true
}

func (s *Server) ServiceCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.services)
}

func (s *Server) MethodCount(serviceName string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	svc, ok := s.services[serviceName]
	if !ok {
		return 0
	}
	return len(svc.methods) + len(svc.streams)
}

func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
}

func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

type serverStream struct {
	ctx        context.Context
	sendCh     chan interface{}
	recvCh     chan interface{}
	header     MD
	trailer    MD
	streamDesc *StreamDesc
	mu         sync.RWMutex
	closed     bool
	closeOnce  sync.Once
}

func (ss *serverStream) Context() context.Context {
	return ss.ctx
}

func (ss *serverStream) SendMsg(msg interface{}) error {
	ss.mu.RLock()
	if ss.closed {
		ss.mu.RUnlock()
		return ErrStreamClosed
	}
	ss.mu.RUnlock()

	if err := checkDeadline(ss.ctx); err != nil {
		return err
	}

	select {
	case ss.sendCh <- msg:
		return nil
	case <-ss.ctx.Done():
		if errors.Is(ss.ctx.Err(), context.DeadlineExceeded) {
			return ErrDeadlineExceeded
		}
		return ss.ctx.Err()
	}
}

func (ss *serverStream) RecvMsg(msg interface{}) error {
	ss.mu.RLock()
	if ss.closed {
		ss.mu.RUnlock()
		return ErrStreamClosed
	}
	ss.mu.RUnlock()

	if err := checkDeadline(ss.ctx); err != nil {
		return err
	}

	select {
	case _, ok := <-ss.recvCh:
		if !ok {
			return ErrStreamClosed
		}
		return nil
	case <-ss.ctx.Done():
		if errors.Is(ss.ctx.Err(), context.DeadlineExceeded) {
			return ErrDeadlineExceeded
		}
		return ss.ctx.Err()
	}
}

func (ss *serverStream) SetHeader(md MD) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.header == nil {
		ss.header = NewMD()
	}
	for k, v := range md {
		ss.header[k] = v
	}
}

func (ss *serverStream) SetTrailer(md MD) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.trailer == nil {
		ss.trailer = NewMD()
	}
	for k, v := range md {
		ss.trailer[k] = v
	}
	SetTrailer(ss.ctx, ss.trailer.Copy())
}

func (ss *serverStream) Close() error {
	ss.closeOnce.Do(func() {
		ss.mu.Lock()
		ss.closed = true
		close(ss.sendCh)
		close(ss.recvCh)
		ss.mu.Unlock()
	})
	return nil
}

func (ss *serverStream) Closed() bool {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.closed
}

func (ss *serverStream) Send(msg interface{}) error {
	return ss.SendMsg(msg)
}

func (ss *serverStream) Recv() (interface{}, error) {
	ss.mu.RLock()
	if ss.closed {
		ss.mu.RUnlock()
		return nil, ErrStreamClosed
	}
	ss.mu.RUnlock()

	if err := checkDeadline(ss.ctx); err != nil {
		return nil, err
	}

	select {
	case msg, ok := <-ss.recvCh:
		if !ok {
			return nil, ErrStreamClosed
		}
		return msg, nil
	case <-ss.ctx.Done():
		if errors.Is(ss.ctx.Err(), context.DeadlineExceeded) {
			return nil, ErrDeadlineExceeded
		}
		return nil, ss.ctx.Err()
	}
}

func (ss *serverStream) PutRecv(msg interface{}) error {
	ss.mu.RLock()
	if ss.closed {
		ss.mu.RUnlock()
		return ErrStreamClosed
	}
	ss.mu.RUnlock()

	select {
	case ss.recvCh <- msg:
		return nil
	case <-ss.ctx.Done():
		if errors.Is(ss.ctx.Err(), context.DeadlineExceeded) {
			return ErrDeadlineExceeded
		}
		return ss.ctx.Err()
	}
}

func checkDeadline(ctx context.Context) error {
	if deadline, ok := ctx.Deadline(); ok {
		if time.Now().After(deadline) {
			return ErrDeadlineExceeded
		}
	}
	return nil
}

func ChainUnaryInterceptors(interceptors ...UnaryInterceptor) UnaryInterceptor {
	if len(interceptors) == 0 {
		return nil
	}

	return func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error) {
		chain := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			next := chain
			chain = func(currentCtx context.Context, currentReq interface{}) (interface{}, error) {
				return interceptor(currentCtx, currentReq, info, next)
			}
		}
		return chain(ctx, req)
	}
}

func ChainStreamInterceptors(interceptors ...StreamInterceptor) StreamInterceptor {
	if len(interceptors) == 0 {
		return nil
	}

	return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
		chain := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			next := chain
			chain = func(currentSrv interface{}, currentStream Stream) error {
				var cs ServerStream = currentStream.(ServerStream)
				return interceptor(currentSrv, cs, info, next)
			}
		}
		return chain(srv, ss)
	}
}

func WithIncomingMetadata(ctx context.Context, md MD) context.Context {
	return NewContextWithMD(ctx, md)
}

func GetIncomingMetadata(ctx context.Context) (MD, bool) {
	return FromContext(ctx)
}

func SetHeader(ctx context.Context, md MD) {
	if stream, ok := ctx.Value(streamKey{}).(Stream); ok {
		stream.SetHeader(md)
	}
}

type streamKey struct{}

func newContextWithStream(ctx context.Context, stream Stream) context.Context {
	return context.WithValue(ctx, streamKey{}, stream)
}
