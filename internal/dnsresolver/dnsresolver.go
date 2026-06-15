package dnsresolver

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrInvalidDomain       = errors.New("dnsresolver: invalid domain name")
	ErrNoUpstreamServers   = errors.New("dnsresolver: no upstream servers configured")
	ErrMaxDepthExceeded    = errors.New("dnsresolver: maximum recursion depth exceeded")
	ErrAllUpstreamsFailed  = errors.New("dnsresolver: all upstream servers failed")
	ErrQueryTimeout        = errors.New("dnsresolver: query timed out")
	ErrInvalidResponse     = errors.New("dnsresolver: invalid DNS response")
	ErrNoRecordsFound      = errors.New("dnsresolver: no DNS records found")
	ErrInvalidConfig       = errors.New("dnsresolver: invalid configuration")
	ErrResolverClosed      = errors.New("dnsresolver: resolver is closed")
)

const (
	TypeA     uint16 = 1
	TypeNS    uint16 = 2
	TypeCNAME uint16 = 5
	TypeAAAA  uint16 = 28

	ClassIN uint16 = 1

	DefaultMaxRecursionDepth = 20
	DefaultQueryTimeout      = 5 * time.Second
	DefaultCacheTTL          = 300 * time.Second
	DefaultCleanupInterval   = 60 * time.Second
)

var DefaultRootServers = []string{
	"198.41.0.4",
	"199.9.14.201",
	"192.33.4.12",
	"199.7.91.13",
	"192.203.230.10",
	"192.5.5.241",
	"192.112.36.4",
	"198.97.190.53",
	"192.36.148.17",
	"192.58.128.30",
	"193.0.14.129",
	"199.7.83.42",
	"202.12.27.33",
}

type Config struct {
	UpstreamServers    []string
	RootServers        []string
	MaxRecursionDepth  int
	QueryTimeout       time.Duration
	DefaultTTL         time.Duration
	CacheCleanupInterval time.Duration
	EnableCache        bool
	EnableRecursion    bool
}

func DefaultConfig() Config {
	return Config{
		UpstreamServers:     []string{"8.8.8.8:53", "1.1.1.1:53"},
		RootServers:         DefaultRootServers,
		MaxRecursionDepth:   DefaultMaxRecursionDepth,
		QueryTimeout:        DefaultQueryTimeout,
		DefaultTTL:          DefaultCacheTTL,
		CacheCleanupInterval: DefaultCleanupInterval,
		EnableCache:         true,
		EnableRecursion:     true,
	}
}

type CacheEntry struct {
	Domain    string
	Records   []DNSRecord
	ExpiresAt time.Time
	TTL       time.Duration
}

type DNSRecord struct {
	Name  string
	Type  uint16
	Class uint16
	TTL   uint32
	Data  string
}

type DNSQuestion struct {
	Name  string
	Type  uint16
	Class uint16
}

type DNSResponse struct {
	Answers     []DNSRecord
	Authorities []DNSRecord
	Additionals []DNSRecord
	Flags       uint16
}

type cacheMap map[string]*CacheEntry

type Resolver struct {
	cfg          Config
	mu           sync.RWMutex
	cache        cacheMap
	closed       bool
	stopCh       chan struct{}
	wg           sync.WaitGroup
	dialUDP      func(network, address string) (net.Conn, error)
	nowFunc      func() time.Time
}

func NewResolver() (*Resolver, error) {
	return NewResolverWithConfig(DefaultConfig())
}

func NewResolverWithConfig(cfg Config) (*Resolver, error) {
	if cfg.MaxRecursionDepth <= 0 {
		cfg.MaxRecursionDepth = DefaultMaxRecursionDepth
	}
	if cfg.QueryTimeout <= 0 {
		cfg.QueryTimeout = DefaultQueryTimeout
	}
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = DefaultCacheTTL
	}
	if cfg.CacheCleanupInterval <= 0 {
		cfg.CacheCleanupInterval = DefaultCleanupInterval
	}
	if len(cfg.RootServers) == 0 {
		cfg.RootServers = DefaultRootServers
	}

	r := &Resolver{
		cfg:     cfg,
		cache:   make(cacheMap),
		stopCh:  make(chan struct{}),
		dialUDP: func(network, address string) (net.Conn, error) {
			return net.DialTimeout(network, address, cfg.QueryTimeout)
		},
		nowFunc: time.Now,
	}

	if cfg.EnableCache {
		r.wg.Add(1)
		go r.cacheCleanupLoop()
	}

	return r, nil
}

func (r *Resolver) Close() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	close(r.stopCh)
	r.mu.Unlock()
	r.wg.Wait()
}

func (r *Resolver) ResolveA(domain string) ([]string, error) {
	return r.Resolve(domain, TypeA)
}

func (r *Resolver) ResolveAAAA(domain string) ([]string, error) {
	return r.Resolve(domain, TypeAAAA)
}

func (r *Resolver) Resolve(domain string, qtype uint16) ([]string, error) {
	r.mu.RLock()
	if r.closed {
		r.mu.RUnlock()
		return nil, ErrResolverClosed
	}
	r.mu.RUnlock()

	if domain == "" {
		return nil, ErrInvalidDomain
	}

	domain = normalizeDomain(domain)

	cacheKey := fmt.Sprintf("%s|%d", domain, qtype)
	if r.cfg.EnableCache {
		if records, ok := r.getFromCache(cacheKey); ok {
			return extractIPs(records, qtype), nil
		}
	}

	var records []DNSRecord
	var err error

	if r.cfg.EnableRecursion {
		records, err = r.resolveRecursive(domain, qtype, 0)
	} else {
		records, err = r.resolveIterative(domain, qtype)
	}

	if err != nil {
		return nil, err
	}

	if r.cfg.EnableCache && len(records) > 0 {
		r.putToCache(cacheKey, records)
	}

	return extractIPs(records, qtype), nil
}

func (r *Resolver) resolveRecursive(domain string, qtype uint16, depth int) ([]DNSRecord, error) {
	if depth >= r.cfg.MaxRecursionDepth {
		return nil, ErrMaxDepthExceeded
	}

	servers := r.cfg.RootServers
	labels := splitDomain(domain)

	for i := 0; i <= len(labels); i++ {
		var zone string
		if i == 0 {
			zone = "."
		} else {
			zone = strings.Join(labels[len(labels)-i:], ".") + "."
		}

		resp, err := r.queryParallel(servers, zone, TypeNS)
		if err == nil && len(resp.Authorities) > 0 {
			nsRecords := filterRecords(resp.Authorities, TypeNS)
			if len(nsRecords) > 0 {
				glue := make(map[string][]string)
				for _, rr := range resp.Additionals {
					if rr.Type == TypeA || rr.Type == TypeAAAA {
						nsName := strings.TrimSuffix(rr.Name, ".")
						glue[nsName] = append(glue[nsName], rr.Data)
					}
				}

				var nextServers []string
				for _, ns := range nsRecords {
					nsName := strings.TrimSuffix(ns.Data, ".")
					if ips, ok := glue[nsName]; ok {
						nextServers = append(nextServers, ips...)
					} else {
						nsIPs, err := r.resolveRecursive(nsName, TypeA, depth+1)
						if err == nil && len(nsIPs) > 0 {
							nextServers = append(nextServers, extractIPs(nsIPs, TypeA)...)
						}
					}
				}

				if len(nextServers) > 0 {
					servers = nextServers
					continue
				}
			}
		}

		break
	}

	resp, err := r.queryParallel(servers, domain, qtype)
	if err != nil {
		return nil, err
	}

	answers := resp.Answers
	cnames := filterRecords(answers, TypeCNAME)
	for len(cnames) > 0 {
		cname := cnames[0].Data
		cname = strings.TrimSuffix(cname, ".")
		if depth+1 >= r.cfg.MaxRecursionDepth {
			return nil, ErrMaxDepthExceeded
		}
		return r.resolveRecursive(cname, qtype, depth+1)
	}

	targetRecords := filterRecords(answers, qtype)
	if len(targetRecords) == 0 {
		return nil, ErrNoRecordsFound
	}

	return answers, nil
}

func (r *Resolver) resolveIterative(domain string, qtype uint16) ([]DNSRecord, error) {
	if len(r.cfg.UpstreamServers) == 0 {
		return nil, ErrNoUpstreamServers
	}

	servers := make([]string, len(r.cfg.UpstreamServers))
	for i, s := range r.cfg.UpstreamServers {
		if !strings.Contains(s, ":") {
			servers[i] = s + ":53"
		} else {
			servers[i] = s
		}
	}

	resp, err := r.queryParallel(servers, domain, qtype)
	if err != nil {
		return nil, err
	}

	answers := resp.Answers
	cnames := filterRecords(answers, TypeCNAME)
	depth := 0
	for len(cnames) > 0 && depth < r.cfg.MaxRecursionDepth {
		cname := strings.TrimSuffix(cnames[0].Data, ".")
		resp, err = r.queryParallel(servers, cname, qtype)
		if err != nil {
			return nil, err
		}
		answers = resp.Answers
		cnames = filterRecords(answers, TypeCNAME)
		depth++
	}

	if len(cnames) > 0 && depth >= r.cfg.MaxRecursionDepth {
		return nil, ErrMaxDepthExceeded
	}

	targetRecords := filterRecords(answers, qtype)
	if len(targetRecords) == 0 {
		return nil, ErrNoRecordsFound
	}

	return answers, nil
}

type queryResult struct {
	resp *DNSResponse
	err  error
	server string
}

func (r *Resolver) queryParallel(servers []string, domain string, qtype uint16) (*DNSResponse, error) {
	if len(servers) == 0 {
		return nil, ErrNoUpstreamServers
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.QueryTimeout)
	defer cancel()

	results := make(chan queryResult, len(servers))
	var pending int32 = int32(len(servers))

	for _, server := range servers {
		go func(s string) {
			defer func() {
				if atomic.AddInt32(&pending, -1) == 0 {
					close(results)
				}
			}()

			addr := s
			if !strings.Contains(addr, ":") {
				addr = addr + ":53"
			}

			resp, err := r.querySingle(ctx, addr, domain, qtype)
			results <- queryResult{resp: resp, err: err, server: s}
		}(server)
	}

	var firstErr error
	var firstValidResp *DNSResponse
	for res := range results {
		if res.err == nil && res.resp != nil {
			if firstValidResp == nil {
				firstValidResp = res.resp
			}
			if len(res.resp.Answers) > 0 || len(res.resp.Authorities) > 0 {
				return res.resp, nil
			}
		}
		if firstErr == nil && res.err != nil {
			firstErr = res.err
		}
	}

	if firstValidResp != nil {
		return firstValidResp, nil
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, ErrAllUpstreamsFailed
}

func (r *Resolver) querySingle(ctx context.Context, server, domain string, qtype uint16) (*DNSResponse, error) {
	query, err := buildQuery(domain, qtype)
	if err != nil {
		return nil, err
	}

	conn, err := r.dialUDP("udp", server)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	conn.SetDeadline(r.nowFunc().Add(r.cfg.QueryTimeout))

	_, err = conn.Write(query)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}

	return parseResponse(buf[:n])
}

func (r *Resolver) getFromCache(key string) ([]DNSRecord, bool) {
	r.mu.RLock()
	entry, exists := r.cache[key]
	r.mu.RUnlock()

	if !exists {
		return nil, false
	}

	if r.nowFunc().After(entry.ExpiresAt) {
		r.mu.Lock()
		if e, ok := r.cache[key]; ok && r.nowFunc().After(e.ExpiresAt) {
			delete(r.cache, key)
		}
		r.mu.Unlock()
		return nil, false
	}

	return entry.Records, true
}

func (r *Resolver) putToCache(key string, records []DNSRecord) {
	var ttl time.Duration
	for _, rr := range records {
		if rr.TTL > 0 {
			ttl = time.Duration(rr.TTL) * time.Second
			break
		}
	}
	if ttl <= 0 {
		ttl = r.cfg.DefaultTTL
	}

	entry := &CacheEntry{
		Records:   records,
		ExpiresAt: r.nowFunc().Add(ttl),
		TTL:       ttl,
	}

	r.mu.Lock()
	r.cache[key] = entry
	r.mu.Unlock()
}

func (r *Resolver) cacheCleanupLoop() {
	defer r.wg.Done()
	ticker := time.NewTicker(r.cfg.CacheCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.cleanupExpired()
		}
	}
}

func (r *Resolver) cleanupExpired() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	now := r.nowFunc()
	for key, entry := range r.cache {
		if now.After(entry.ExpiresAt) {
			delete(r.cache, key)
			count++
		}
	}
	return count
}

func (r *Resolver) CacheCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.cache)
}

func (r *Resolver) ClearCache() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = make(cacheMap)
}

func normalizeDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimSuffix(domain, ".")
	return strings.ToLower(domain)
}

func splitDomain(domain string) []string {
	domain = strings.Trim(domain, ".")
	if domain == "" {
		return nil
	}
	return strings.Split(domain, ".")
}

func filterRecords(records []DNSRecord, rtype uint16) []DNSRecord {
	var filtered []DNSRecord
	for _, rr := range records {
		if rr.Type == rtype {
			filtered = append(filtered, rr)
		}
	}
	return filtered
}

func extractIPs(records []DNSRecord, qtype uint16) []string {
	var ips []string
	for _, rr := range records {
		if rr.Type == qtype {
			ips = append(ips, rr.Data)
		}
	}
	return ips
}

func buildQuery(domain string, qtype uint16) ([]byte, error) {
	id := uint16(rand.Intn(65535))
	flags := uint16(0x0100)

	question, err := encodeQuestion(domain, qtype, ClassIN)
	if err != nil {
		return nil, err
	}

	msg := make([]byte, 12+len(question))
	binary.BigEndian.PutUint16(msg[0:2], id)
	binary.BigEndian.PutUint16(msg[2:4], flags)
	binary.BigEndian.PutUint16(msg[4:6], 1)
	binary.BigEndian.PutUint16(msg[6:8], 0)
	binary.BigEndian.PutUint16(msg[8:10], 0)
	binary.BigEndian.PutUint16(msg[10:12], 0)
	copy(msg[12:], question)

	return msg, nil
}

func encodeQuestion(domain string, qtype, qclass uint16) ([]byte, error) {
	labels := splitDomain(domain)
	var buf []byte

	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return nil, ErrInvalidDomain
		}
		buf = append(buf, byte(len(label)))
		buf = append(buf, []byte(label)...)
	}
	buf = append(buf, 0)

	qtypeBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(qtypeBytes, qtype)
	buf = append(buf, qtypeBytes...)

	qclassBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(qclassBytes, qclass)
	buf = append(buf, qclassBytes...)

	return buf, nil
}

func parseResponse(msg []byte) (*DNSResponse, error) {
	if len(msg) < 12 {
		return nil, ErrInvalidResponse
	}

	id := binary.BigEndian.Uint16(msg[0:2])
	flags := binary.BigEndian.Uint16(msg[2:4])
	qdCount := binary.BigEndian.Uint16(msg[4:6])
	anCount := binary.BigEndian.Uint16(msg[6:8])
	nsCount := binary.BigEndian.Uint16(msg[8:10])
	arCount := binary.BigEndian.Uint16(msg[10:12])

	_ = id
	_ = flags

	offset := 12

	for i := 0; i < int(qdCount); i++ {
		_, n, err := decodeName(msg, offset)
		if err != nil {
			return nil, err
		}
		offset = n + 4
	}

	resp := &DNSResponse{
		Flags: flags,
	}

	var err error
	resp.Answers, offset, err = parseRRs(msg, offset, int(anCount))
	if err != nil {
		return nil, err
	}

	resp.Authorities, offset, err = parseRRs(msg, offset, int(nsCount))
	if err != nil {
		return nil, err
	}

	resp.Additionals, offset, err = parseRRs(msg, offset, int(arCount))
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func parseRRs(msg []byte, offset, count int) ([]DNSRecord, int, error) {
	var records []DNSRecord
	for i := 0; i < count; i++ {
		rr, n, err := parseRR(msg, offset)
		if err != nil {
			return nil, offset, err
		}
		records = append(records, rr)
		offset = n
	}
	return records, offset, nil
}

func parseRR(msg []byte, offset int) (DNSRecord, int, error) {
	var rr DNSRecord

	name, n, err := decodeName(msg, offset)
	if err != nil {
		return rr, offset, err
	}
	rr.Name = name
	offset = n

	if offset+10 > len(msg) {
		return rr, offset, ErrInvalidResponse
	}

	rr.Type = binary.BigEndian.Uint16(msg[offset : offset+2])
	rr.Class = binary.BigEndian.Uint16(msg[offset+2 : offset+4])
	rr.TTL = binary.BigEndian.Uint32(msg[offset+4 : offset+8])
	rdLength := binary.BigEndian.Uint16(msg[offset+8 : offset+10])
	offset += 10

	if offset+int(rdLength) > len(msg) {
		return rr, offset, ErrInvalidResponse
	}

	rdata := msg[offset : offset+int(rdLength)]
	offset += int(rdLength)

	switch rr.Type {
	case TypeA:
		if len(rdata) == 4 {
			rr.Data = net.IP(rdata).String()
		}
	case TypeAAAA:
		if len(rdata) == 16 {
			rr.Data = net.IP(rdata).String()
		}
	case TypeCNAME, TypeNS:
		cname, _, err := decodeName(msg, offset-int(rdLength))
		if err == nil {
			rr.Data = cname
		}
	default:
		rr.Data = fmt.Sprintf("%x", rdata)
	}

	_ = name
	return rr, offset, nil
}

func decodeName(msg []byte, offset int) (string, int, error) {
	var labels []string
	currentOffset := offset
	endOffset := offset
	jumped := false
	maxJumps := 10
	jumps := 0

	for {
		if currentOffset >= len(msg) {
			return "", endOffset, ErrInvalidResponse
		}

		length := int(msg[currentOffset])

		if length == 0 {
			currentOffset++
			if !jumped {
				endOffset = currentOffset
			}
			break
		}

		if (length & 0xC0) == 0xC0 {
			if currentOffset+1 >= len(msg) {
				return "", endOffset, ErrInvalidResponse
			}
			if jumps >= maxJumps {
				return "", endOffset, ErrInvalidResponse
			}
			pointer := ((length & 0x3F) << 8) | int(msg[currentOffset+1])
			if !jumped {
				endOffset = currentOffset + 2
				jumped = true
			}
			currentOffset = pointer
			jumps++
			continue
		}

		if length > 63 {
			return "", endOffset, ErrInvalidResponse
		}

		currentOffset++
		if currentOffset+length > len(msg) {
			return "", endOffset, ErrInvalidResponse
		}
		labels = append(labels, string(msg[currentOffset:currentOffset+length]))
		currentOffset += length

		if !jumped {
			endOffset = currentOffset
		}
	}

	return strings.Join(labels, "."), endOffset, nil
}
