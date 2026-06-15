package tcpproxy

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrProxyClosed       = errors.New("tcpproxy: proxy is closed")
	ErrNoHealthyUpstream = errors.New("tcpproxy: no healthy upstream available")
	ErrStreamNotFound    = errors.New("tcpproxy: stream not found")
	ErrStreamClosed      = errors.New("tcpproxy: stream is closed")
	ErrPoolClosed        = errors.New("tcpproxy: pool is closed")
	ErrPoolExhausted     = errors.New("tcpproxy: pool exhausted")
	ErrInvalidFrame      = errors.New("tcpproxy: invalid frame")
	ErrMuxConnClosed     = errors.New("tcpproxy: mux connection closed")
)

const (
	FrameTypeSYNC      uint16 = 0x01
	FrameTypeDATA      uint16 = 0x02
	FrameTypeACK       uint16 = 0x03
	FrameTypeFIN       uint16 = 0x04
	FrameTypeRST       uint16 = 0x05
	FrameTypeHEARTBEAT uint16 = 0x06

	FrameHeaderSize = 8
)

type Frame struct {
	Type     uint16
	StreamID uint16
	Length   uint32
	Payload  []byte
}

func EncodeFrame(f *Frame) ([]byte, error) {
	if f == nil {
		return nil, ErrInvalidFrame
	}
	if f.Length != uint32(len(f.Payload)) {
		f.Length = uint32(len(f.Payload))
	}
	buf := make([]byte, FrameHeaderSize+len(f.Payload))
	binary.BigEndian.PutUint16(buf[0:2], f.Type)
	binary.BigEndian.PutUint16(buf[2:4], f.StreamID)
	binary.BigEndian.PutUint32(buf[4:8], f.Length)
	if len(f.Payload) > 0 {
		copy(buf[FrameHeaderSize:], f.Payload)
	}
	return buf, nil
}

func DecodeFrame(r io.Reader) (*Frame, error) {
	header := make([]byte, FrameHeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	f := &Frame{
		Type:     binary.BigEndian.Uint16(header[0:2]),
		StreamID: binary.BigEndian.Uint16(header[2:4]),
		Length:   binary.BigEndian.Uint32(header[4:8]),
	}
	if f.Length > 0 {
		f.Payload = make([]byte, f.Length)
		if _, err := io.ReadFull(r, f.Payload); err != nil {
			return nil, err
		}
	}
	return f, nil
}

type Stream struct {
	ID        uint16
	mux       *MuxConn
	readCh    chan []byte
	readBuf   []byte
	closeOnce sync.Once
	chCloseOnce sync.Once
	closed    atomic.Bool
	finSent   atomic.Bool
	finRecv   atomic.Bool
}

func newStream(id uint16, mux *MuxConn) *Stream {
	return &Stream{
		ID:     id,
		mux:    mux,
		readCh: make(chan []byte, 256),
	}
}

func (s *Stream) Read(p []byte) (int, error) {
	if s.closed.Load() && len(s.readBuf) == 0 && len(s.readCh) == 0 {
		return 0, io.EOF
	}
	if len(s.readBuf) > 0 {
		n := copy(p, s.readBuf)
		s.readBuf = s.readBuf[n:]
		return n, nil
	}
	data, ok := <-s.readCh
	if !ok {
		return 0, io.EOF
	}
	n := copy(p, data)
	if n < len(data) {
		s.readBuf = data[n:]
	}
	return n, nil
}

func (s *Stream) Write(p []byte) (int, error) {
	if s.closed.Load() {
		return 0, ErrStreamClosed
	}
	if s.mux.closed.Load() {
		return 0, ErrMuxConnClosed
	}
	frame := &Frame{
		Type:     FrameTypeDATA,
		StreamID: s.ID,
		Payload:  make([]byte, len(p)),
	}
	copy(frame.Payload, p)
	if err := s.mux.writeFrame(frame); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *Stream) Close() error {
	s.closeOnce.Do(func() {
		s.closed.Store(true)
		if !s.mux.closed.Load() && !s.finSent.Load() {
			s.finSent.Store(true)
			frame := &Frame{
				Type:     FrameTypeFIN,
				StreamID: s.ID,
			}
			_ = s.mux.writeFrame(frame)
		}
	})
	s.chCloseOnce.Do(func() {
		close(s.readCh)
	})
	return nil
}

func (s *Stream) pushData(data []byte) {
	if s.closed.Load() {
		return
	}
	defer func() {
		_ = recover()
	}()
	buf := make([]byte, len(data))
	copy(buf, data)
	s.readCh <- buf
}

type MuxConn struct {
	conn       net.Conn
	streams    map[uint16]*Stream
	streamMu   sync.RWMutex
	nextStream atomic.Uint32
	writeMu    sync.Mutex
	closed     atomic.Bool
	stopCh     chan struct{}
	wg         sync.WaitGroup
	onStream   func(*Stream)
}

func NewMuxConn(conn net.Conn, onStream func(*Stream)) *MuxConn {
	m := &MuxConn{
		conn:     conn,
		streams:  make(map[uint16]*Stream),
		stopCh:   make(chan struct{}),
		onStream: onStream,
	}
	m.wg.Add(1)
	go m.readLoop()
	return m
}

func (m *MuxConn) NewStream() (*Stream, error) {
	if m.closed.Load() {
		return nil, ErrMuxConnClosed
	}
	id := uint16(m.nextStream.Add(1))
	s := newStream(id, m)
	m.streamMu.Lock()
	m.streams[id] = s
	m.streamMu.Unlock()
	frame := &Frame{
		Type:     FrameTypeSYNC,
		StreamID: id,
	}
	if err := m.writeFrame(frame); err != nil {
		m.streamMu.Lock()
		delete(m.streams, id)
		m.streamMu.Unlock()
		return nil, err
	}
	return s, nil
}

func (m *MuxConn) writeFrame(f *Frame) error {
	if m.closed.Load() {
		return ErrMuxConnClosed
	}
	buf, err := EncodeFrame(f)
	if err != nil {
		return err
	}
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	if _, err := m.conn.Write(buf); err != nil {
		return err
	}
	return nil
}

func (m *MuxConn) readLoop() {
	defer m.wg.Done()
	for {
		select {
		case <-m.stopCh:
			return
		default:
		}
		f, err := DecodeFrame(m.conn)
		if err != nil {
			m.cleanupOnError()
			return
		}
		m.handleFrame(f)
	}
}

func (m *MuxConn) cleanupOnError() {
	if !m.closed.Swap(true) {
		close(m.stopCh)
		m.streamMu.Lock()
		for id, s := range m.streams {
			s.closed.Store(true)
			s.chCloseOnce.Do(func() {
				close(s.readCh)
			})
			delete(m.streams, id)
		}
		m.streamMu.Unlock()
		_ = m.conn.Close()
	}
}

func (m *MuxConn) handleFrame(f *Frame) {
	switch f.Type {
	case FrameTypeSYNC:
		m.handleSync(f)
	case FrameTypeDATA:
		m.handleData(f)
	case FrameTypeACK:
	case FrameTypeFIN:
		m.handleFin(f)
	case FrameTypeRST:
		m.handleRst(f)
	case FrameTypeHEARTBEAT:
		resp := &Frame{
			Type:     FrameTypeHEARTBEAT,
			StreamID: f.StreamID,
		}
		_ = m.writeFrame(resp)
	}
}

func (m *MuxConn) handleSync(f *Frame) {
	s := newStream(f.StreamID, m)
	m.streamMu.Lock()
	m.streams[f.StreamID] = s
	m.streamMu.Unlock()
	if m.onStream != nil {
		go m.onStream(s)
	}
}

func (m *MuxConn) handleData(f *Frame) {
	m.streamMu.RLock()
	s, ok := m.streams[f.StreamID]
	m.streamMu.RUnlock()
	if ok {
		s.pushData(f.Payload)
	}
}

func (m *MuxConn) handleFin(f *Frame) {
	m.streamMu.RLock()
	s, ok := m.streams[f.StreamID]
	m.streamMu.RUnlock()
	if ok {
		s.finRecv.Store(true)
		if s.finSent.Load() {
			m.removeStream(f.StreamID)
		}
		s.Close()
	}
}

func (m *MuxConn) handleRst(f *Frame) {
	m.streamMu.Lock()
	s, ok := m.streams[f.StreamID]
	if ok {
		delete(m.streams, f.StreamID)
	}
	m.streamMu.Unlock()
	if ok {
		s.closed.Store(true)
		s.chCloseOnce.Do(func() {
			close(s.readCh)
		})
	}
}

func (m *MuxConn) removeStream(id uint16) {
	m.streamMu.Lock()
	defer m.streamMu.Unlock()
	delete(m.streams, id)
}

func (m *MuxConn) StreamCount() int {
	m.streamMu.RLock()
	defer m.streamMu.RUnlock()
	return len(m.streams)
}

func (m *MuxConn) Close() error {
	m.cleanupOnError()
	m.wg.Wait()
	return nil
}

func (m *MuxConn) Closed() bool {
	return m.closed.Load()
}

type Upstream struct {
	Address string
	healthy atomic.Bool
}

func NewUpstream(addr string) *Upstream {
	u := &Upstream{Address: addr}
	u.healthy.Store(true)
	return u
}

func (u *Upstream) Healthy() bool {
	return u.healthy.Load()
}

func (u *Upstream) SetHealthy(h bool) {
	u.healthy.Store(h)
}

func (u *Upstream) Connect() (net.Conn, error) {
	return net.DialTimeout("tcp", u.Address, 5*time.Second)
}

func (u *Upstream) Probe(timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", u.Address, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

type HealthCheckerConfig struct {
	CheckInterval  time.Duration
	ProbeTimeout   time.Duration
	FailThreshold  int
	PassThreshold  int
}

type HealthChecker struct {
	cfg          HealthCheckerConfig
	upstreams    map[string]*upstreamHealth
	mu           sync.RWMutex
	stopCh       chan struct{}
	running      atomic.Bool
	wg           sync.WaitGroup
	onChange     func(addr string, healthy bool)
}

type upstreamHealth struct {
	upstream   *Upstream
	failCount  int
	passCount  int
	lastCheck  time.Time
}

func NewHealthChecker(cfg HealthCheckerConfig) *HealthChecker {
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 10 * time.Second
	}
	if cfg.ProbeTimeout <= 0 {
		cfg.ProbeTimeout = 3 * time.Second
	}
	if cfg.FailThreshold <= 0 {
		cfg.FailThreshold = 3
	}
	if cfg.PassThreshold <= 0 {
		cfg.PassThreshold = 2
	}
	return &HealthChecker{
		cfg:       cfg,
		upstreams: make(map[string]*upstreamHealth),
		stopCh:    make(chan struct{}),
	}
}

func (hc *HealthChecker) SetOnChange(fn func(addr string, healthy bool)) {
	hc.onChange = fn
}

func (hc *HealthChecker) AddUpstream(u *Upstream) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.upstreams[u.Address] = &upstreamHealth{
		upstream:  u,
		lastCheck: time.Now(),
	}
}

func (hc *HealthChecker) RemoveUpstream(addr string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	delete(hc.upstreams, addr)
}

func (hc *HealthChecker) GetUpstreams() []*Upstream {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	result := make([]*Upstream, 0, len(hc.upstreams))
	for _, uh := range hc.upstreams {
		result = append(result, uh.upstream)
	}
	return result
}

func (hc *HealthChecker) GetHealthyUpstreams() []*Upstream {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	result := make([]*Upstream, 0, len(hc.upstreams))
	for _, uh := range hc.upstreams {
		if uh.upstream.Healthy() {
			result = append(result, uh.upstream)
		}
	}
	return result
}

func (hc *HealthChecker) Start() {
	if hc.running.Swap(true) {
		return
	}
	hc.wg.Add(1)
	go hc.checkLoop()
}

func (hc *HealthChecker) Stop() {
	if !hc.running.Swap(false) {
		return
	}
	close(hc.stopCh)
	hc.wg.Wait()
}

func (hc *HealthChecker) checkLoop() {
	defer hc.wg.Done()
	ticker := time.NewTicker(hc.cfg.CheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-hc.stopCh:
			return
		case <-ticker.C:
			hc.checkAll()
		}
	}
}

func (hc *HealthChecker) checkAll() {
	hc.mu.RLock()
	addrs := make([]string, 0, len(hc.upstreams))
	for addr := range hc.upstreams {
		addrs = append(addrs, addr)
	}
	hc.mu.RUnlock()
	for _, addr := range addrs {
		hc.checkOne(addr)
	}
}

func (hc *HealthChecker) checkOne(addr string) {
	hc.mu.RLock()
	uh, ok := hc.upstreams[addr]
	if !ok {
		hc.mu.RUnlock()
		return
	}
	upstream := uh.upstream
	hc.mu.RUnlock()

	healthy := upstream.Probe(hc.cfg.ProbeTimeout)
	now := time.Now()

	hc.mu.Lock()
	defer hc.mu.Unlock()
	uh, ok = hc.upstreams[addr]
	if !ok {
		return
	}
	uh.lastCheck = now
	oldHealthy := uh.upstream.Healthy()

	if healthy {
		uh.failCount = 0
		uh.passCount++
		if !oldHealthy && uh.passCount >= hc.cfg.PassThreshold {
			uh.upstream.SetHealthy(true)
			if hc.onChange != nil {
				hc.onChange(addr, true)
			}
		}
	} else {
		uh.passCount = 0
		uh.failCount++
		if oldHealthy && uh.failCount >= hc.cfg.FailThreshold {
			uh.upstream.SetHealthy(false)
			if hc.onChange != nil {
				hc.onChange(addr, false)
			}
		}
	}
}

func (hc *HealthChecker) Running() bool {
	return hc.running.Load()
}

type ConnPoolConfig struct {
	MaxConns    int
	IdleTimeout time.Duration
	WaitTimeout time.Duration
}

type poolConn struct {
	conn       net.Conn
	upstream   *Upstream
	lastUsed   time.Time
	idle       atomic.Bool
}

type ConnPool struct {
	cfg         ConnPoolConfig
	upstream    *Upstream
	mu          sync.Mutex
	cond        *sync.Cond
	idleList    []*poolConn
	activeCount int
	closed      bool
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

func NewConnPool(upstream *Upstream, cfg ConnPoolConfig) *ConnPool {
	if cfg.MaxConns <= 0 {
		cfg.MaxConns = 10
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 5 * time.Minute
	}
	if cfg.WaitTimeout <= 0 {
		cfg.WaitTimeout = 0
	}
	p := &ConnPool{
		cfg:      cfg,
		upstream: upstream,
		stopCh:   make(chan struct{}),
	}
	p.cond = sync.NewCond(&p.mu)
	if cfg.IdleTimeout > 0 {
		p.wg.Add(1)
		go p.idleTimeoutLoop()
	}
	return p
}

func (p *ConnPool) Get() (net.Conn, error) {
	var deadline time.Time
	for {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return nil, ErrPoolClosed
		}
		for i := len(p.idleList) - 1; i >= 0; i-- {
			pc := p.idleList[i]
			if p.cfg.IdleTimeout > 0 && time.Since(pc.lastUsed) > p.cfg.IdleTimeout {
				p.idleList = append(p.idleList[:i], p.idleList[i+1:]...)
				p.mu.Unlock()
				_ = pc.conn.Close()
				p.mu.Lock()
				continue
			}
			p.idleList = append(p.idleList[:i], p.idleList[i+1:]...)
			pc.idle.Store(false)
			p.activeCount++
			p.mu.Unlock()
			return &pooledConn{pc: pc, pool: p}, nil
		}
		if p.activeCount < p.cfg.MaxConns {
			p.activeCount++
			p.mu.Unlock()
			conn, err := p.upstream.Connect()
			if err != nil {
				p.mu.Lock()
				p.activeCount--
				p.cond.Signal()
				p.mu.Unlock()
				return nil, err
			}
			pc := &poolConn{
				conn:     conn,
				upstream: p.upstream,
				lastUsed: time.Now(),
			}
			return &pooledConn{pc: pc, pool: p}, nil
		}
		if p.cfg.WaitTimeout == 0 {
			p.mu.Unlock()
			return nil, ErrPoolExhausted
		}
		if deadline.IsZero() {
			deadline = time.Now().Add(p.cfg.WaitTimeout)
			go func(d time.Time) {
				select {
				case <-time.After(time.Until(d)):
					p.mu.Lock()
					p.cond.Broadcast()
					p.mu.Unlock()
				case <-p.stopCh:
					return
				}
			}(deadline)
		}
		for {
			if p.closed {
				p.mu.Unlock()
				return nil, ErrPoolClosed
			}
			if time.Now().After(deadline) {
				p.mu.Unlock()
				return nil, ErrPoolExhausted
			}
			if len(p.idleList) > 0 || p.activeCount < p.cfg.MaxConns {
				break
			}
			p.cond.Wait()
		}
		p.mu.Unlock()
	}
}

func (p *ConnPool) Put(pc *poolConn) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		_ = pc.conn.Close()
		return ErrPoolClosed
	}
	p.activeCount--
	if p.cfg.IdleTimeout > 0 && time.Since(pc.lastUsed) > p.cfg.IdleTimeout {
		_ = pc.conn.Close()
		p.cond.Signal()
		return nil
	}
	pc.lastUsed = time.Now()
	pc.idle.Store(true)
	p.idleList = append(p.idleList, pc)
	p.cond.Signal()
	return nil
}

func (p *ConnPool) Remove(pc *poolConn) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activeCount--
	_ = pc.conn.Close()
	p.cond.Signal()
	return nil
}

func (p *ConnPool) idleTimeoutLoop() {
	defer p.wg.Done()
	checkInterval := p.cfg.IdleTimeout / 2
	if checkInterval <= 0 {
		checkInterval = p.cfg.IdleTimeout
	}
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.reclaimIdle()
		}
	}
}

func (p *ConnPool) reclaimIdle() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	var expired []*poolConn
	now := time.Now()
	i := 0
	for _, pc := range p.idleList {
		if now.Sub(pc.lastUsed) > p.cfg.IdleTimeout {
			expired = append(expired, pc)
		} else {
			p.idleList[i] = pc
			i++
		}
	}
	p.idleList = p.idleList[:i]
	if len(expired) > 0 {
		p.cond.Broadcast()
	}
	p.mu.Unlock()
	for _, pc := range expired {
		_ = pc.conn.Close()
	}
}

func (p *ConnPool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	close(p.stopCh)
	p.cond.Broadcast()
	toClose := make([]*poolConn, 0, len(p.idleList))
	toClose = append(toClose, p.idleList...)
	p.idleList = nil
	p.mu.Unlock()
	for _, pc := range toClose {
		_ = pc.conn.Close()
	}
	p.wg.Wait()
}

func (p *ConnPool) IdleCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.idleList)
}

func (p *ConnPool) ActiveCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.activeCount
}

func (p *ConnPool) TotalCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.idleList) + p.activeCount
}

type pooledConn struct {
	pc   *poolConn
	pool *ConnPool
}

func (c *pooledConn) Read(b []byte) (int, error) {
	n, err := c.pc.conn.Read(b)
	if err != nil && err != io.EOF {
		_ = c.pool.Remove(c.pc)
	}
	return n, err
}

func (c *pooledConn) Write(b []byte) (int, error) {
	n, err := c.pc.conn.Write(b)
	if err != nil {
		_ = c.pool.Remove(c.pc)
	}
	return n, err
}

func (c *pooledConn) Close() error {
	if c.pc.idle.Load() {
		return nil
	}
	return c.pool.Put(c.pc)
}

func (c *pooledConn) LocalAddr() net.Addr {
	return c.pc.conn.LocalAddr()
}

func (c *pooledConn) RemoteAddr() net.Addr {
	return c.pc.conn.RemoteAddr()
}

func (c *pooledConn) SetDeadline(t time.Time) error {
	return c.pc.conn.SetDeadline(t)
}

func (c *pooledConn) SetReadDeadline(t time.Time) error {
	return c.pc.conn.SetReadDeadline(t)
}

func (c *pooledConn) SetWriteDeadline(t time.Time) error {
	return c.pc.conn.SetWriteDeadline(t)
}

type IPHashBalancer struct {
	mu        sync.RWMutex
	upstreams []*Upstream
	mapping   map[string]*Upstream
	hc        *HealthChecker
}

func NewIPHashBalancer(hc *HealthChecker) *IPHashBalancer {
	b := &IPHashBalancer{
		mapping: make(map[string]*Upstream),
		hc:      hc,
	}
	return b
}

func (b *IPHashBalancer) hashIP(ip string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(ip))
	return h.Sum32()
}

func (b *IPHashBalancer) SetUpstreams(upstreams []*Upstream) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.upstreams = make([]*Upstream, len(upstreams))
	copy(b.upstreams, upstreams)
}

func (b *IPHashBalancer) GetUpstream(clientIP string) (*Upstream, error) {
	b.mu.RLock()
	var healthy []*Upstream
	if b.hc != nil {
		healthy = b.hc.GetHealthyUpstreams()
	} else {
		for _, u := range b.upstreams {
			if u.Healthy() {
				healthy = append(healthy, u)
			}
		}
	}
	if len(healthy) == 0 {
		b.mu.RUnlock()
		return nil, ErrNoHealthyUpstream
	}

	if mapped, ok := b.mapping[clientIP]; ok {
		for _, u := range healthy {
			if u.Address == mapped.Address {
				b.mu.RUnlock()
				return u, nil
			}
		}
	}

	hash := b.hashIP(clientIP)
	idx := int(hash % uint32(len(healthy)))
	selected := healthy[idx]
	b.mu.RUnlock()

	b.mu.Lock()
	b.mapping[clientIP] = selected
	b.mu.Unlock()
	return selected, nil
}

func (b *IPHashBalancer) RemoveFromMapping(addr string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ip, u := range b.mapping {
		if u.Address == addr {
			delete(b.mapping, ip)
		}
	}
}

func (b *IPHashBalancer) ClearMapping() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.mapping = make(map[string]*Upstream)
}

func (b *IPHashBalancer) MappingCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.mapping)
}

type ProxyConfig struct {
	ListenAddress     string
	Upstreams         []string
	PoolMaxConns      int
	PoolIdleTimeout   time.Duration
	PoolWaitTimeout   time.Duration
	HealthCheckConfig HealthCheckerConfig
	EnableStickySession bool
}

type TCPProxy struct {
	cfg         ProxyConfig
	listener    net.Listener
	hc          *HealthChecker
	balancer    *IPHashBalancer
	pools       map[string]*ConnPool
	poolsMu     sync.RWMutex
	muxes       map[string]*MuxConn
	muxesMu     sync.RWMutex
	closed      atomic.Bool
	closeOnce   sync.Once
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

func NewTCPProxy(cfg ProxyConfig) (*TCPProxy, error) {
	if cfg.ListenAddress == "" {
		return nil, errors.New("tcpproxy: listen address is required")
	}
	if len(cfg.Upstreams) == 0 {
		return nil, errors.New("tcpproxy: at least one upstream is required")
	}
	if cfg.PoolMaxConns <= 0 {
		cfg.PoolMaxConns = 10
	}
	if cfg.PoolIdleTimeout <= 0 {
		cfg.PoolIdleTimeout = 5 * time.Minute
	}

	p := &TCPProxy{
		cfg:     cfg,
		hc:      NewHealthChecker(cfg.HealthCheckConfig),
		pools:   make(map[string]*ConnPool),
		muxes:   make(map[string]*MuxConn),
		stopCh:  make(chan struct{}),
	}
	p.balancer = NewIPHashBalancer(p.hc)

	upstreams := make([]*Upstream, 0, len(cfg.Upstreams))
	for _, addr := range cfg.Upstreams {
		u := NewUpstream(addr)
		upstreams = append(upstreams, u)
		p.hc.AddUpstream(u)
		poolCfg := ConnPoolConfig{
			MaxConns:    cfg.PoolMaxConns,
			IdleTimeout: cfg.PoolIdleTimeout,
			WaitTimeout: cfg.PoolWaitTimeout,
		}
		p.pools[addr] = NewConnPool(u, poolCfg)
	}
	p.balancer.SetUpstreams(upstreams)
	p.hc.SetOnChange(func(addr string, healthy bool) {
		if !healthy {
			p.balancer.RemoveFromMapping(addr)
		}
	})
	return p, nil
}

func (p *TCPProxy) Start() error {
	if p.closed.Load() {
		return ErrProxyClosed
	}
	ln, err := net.Listen("tcp", p.cfg.ListenAddress)
	if err != nil {
		return fmt.Errorf("tcpproxy: listen failed: %w", err)
	}
	p.listener = ln
	p.hc.Start()
	p.wg.Add(1)
	go p.acceptLoop()
	return nil
}

func (p *TCPProxy) Stop() error {
	p.closeOnce.Do(func() {
		p.closed.Store(true)
		close(p.stopCh)
		if p.listener != nil {
			_ = p.listener.Close()
		}
		p.hc.Stop()
		p.muxesMu.Lock()
		for key, mux := range p.muxes {
			_ = mux.Close()
			delete(p.muxes, key)
		}
		p.muxesMu.Unlock()
		p.poolsMu.Lock()
		for _, pool := range p.pools {
			pool.Close()
		}
		p.poolsMu.Unlock()
	})
	p.wg.Wait()
	return nil
}

func (p *TCPProxy) acceptLoop() {
	defer p.wg.Done()
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			if p.closed.Load() {
				return
			}
			continue
		}
		p.wg.Add(1)
		go p.handleClientConn(conn)
	}
}

func (p *TCPProxy) handleClientConn(conn net.Conn) {
	defer p.wg.Done()
	clientIP, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

	onStream := func(s *Stream) {
		p.wg.Add(1)
		go p.handleStream(s, clientIP)
	}

	mux := NewMuxConn(conn, onStream)
	p.muxesMu.Lock()
	p.muxes[conn.RemoteAddr().String()] = mux
	p.muxesMu.Unlock()

	<-mux.stopCh
	p.muxesMu.Lock()
	delete(p.muxes, conn.RemoteAddr().String())
	p.muxesMu.Unlock()
}

func (p *TCPProxy) handleStream(s *Stream, clientIP string) {
	defer p.wg.Done()
	defer s.Close()

	var upstream *Upstream
	var err error
	if p.cfg.EnableStickySession {
		upstream, err = p.balancer.GetUpstream(clientIP)
	} else {
		healthy := p.hc.GetHealthyUpstreams()
		if len(healthy) == 0 {
			err = ErrNoHealthyUpstream
		} else {
			hash := p.balancer.hashIP(clientIP)
			upstream = healthy[int(hash%uint32(len(healthy)))]
		}
	}
	if err != nil {
		rstFrame := &Frame{
			Type:     FrameTypeRST,
			StreamID: s.ID,
		}
		_ = s.mux.writeFrame(rstFrame)
		return
	}

	p.poolsMu.RLock()
	pool, ok := p.pools[upstream.Address]
	p.poolsMu.RUnlock()
	if !ok {
		return
	}

	upstreamConn, err := pool.Get()
	if err != nil {
		rstFrame := &Frame{
			Type:     FrameTypeRST,
			StreamID: s.ID,
		}
		_ = s.mux.writeFrame(rstFrame)
		return
	}
	defer upstreamConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			n, rerr := s.Read(buf)
			if n > 0 {
				if _, werr := upstreamConn.Write(buf[:n]); werr != nil {
					return
				}
			}
			if rerr != nil {
				if tconn, ok := upstreamConn.(*pooledConn); ok {
					_ = tconn.pool.Remove(tconn.pc)
				}
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		buf := make([]byte, 32*1024)
		for {
			n, rerr := upstreamConn.Read(buf)
			if n > 0 {
				if _, werr := s.Write(buf[:n]); werr != nil {
					return
				}
			}
			if rerr != nil {
				return
			}
		}
	}()

	wg.Wait()
}

func (p *TCPProxy) Addr() string {
	if p.listener == nil {
		return ""
	}
	return p.listener.Addr().String()
}

func (p *TCPProxy) GetHealthChecker() *HealthChecker {
	return p.hc
}

func (p *TCPProxy) GetBalancer() *IPHashBalancer {
	return p.balancer
}

func (p *TCPProxy) GetPool(addr string) (*ConnPool, bool) {
	p.poolsMu.RLock()
	defer p.poolsMu.RUnlock()
	pool, ok := p.pools[addr]
	return pool, ok
}
