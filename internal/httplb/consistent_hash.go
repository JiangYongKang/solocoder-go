package httplb

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"sort"
	"sync"
)

const DefaultVirtualNodes = 100

type hashNode struct {
	hash   uint64
	server string
}

type ConsistentHash struct {
	pool         *ServerPool
	virtualNodes int
	ring         []hashNode
	hashMap      map[uint64]string
	mu           sync.RWMutex
}

func NewConsistentHash(servers []string, virtualNodes int) (*ConsistentHash, error) {
	if virtualNodes <= 0 {
		virtualNodes = DefaultVirtualNodes
	}

	ch := &ConsistentHash{
		pool:         NewServerPool(),
		virtualNodes: virtualNodes,
		ring:         make([]hashNode, 0),
		hashMap:      make(map[uint64]string),
	}

	for _, addr := range servers {
		if err := ch.AddServer(addr, 1); err != nil {
			return nil, err
		}
	}

	return ch, nil
}

func hashKey(key string) uint64 {
	h := sha1.New()
	h.Write([]byte(key))
	sum := h.Sum(nil)
	return binary.BigEndian.Uint64(sum[:8])
}

func (ch *ConsistentHash) Next(key string) (*BackendServer, error) {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	if len(ch.ring) == 0 {
		return nil, ErrNoHealthyServer
	}

	hash := hashKey(key)
	idx := sort.Search(len(ch.ring), func(i int) bool {
		return ch.ring[i].hash >= hash
	})

	if idx == len(ch.ring) {
		idx = 0
	}

	address := ch.ring[idx].server
	server, ok := ch.pool.GetServer(address)
	if !ok || !server.IsHealthy() {
		return nil, ErrNoHealthyServer
	}

	server.IncConn()
	return server, nil
}

func (ch *ConsistentHash) AddServer(address string, weight int) error {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	if err := ch.pool.AddServer(address, weight); err != nil {
		return err
	}

	vnCount := ch.virtualNodes * weight
	for i := 0; i < vnCount; i++ {
		virtualKey := fmt.Sprintf("%s#vn%d", address, i)
		hash := hashKey(virtualKey)
		ch.ring = append(ch.ring, hashNode{hash: hash, server: address})
		ch.hashMap[hash] = address
	}

	sort.Slice(ch.ring, func(i, j int) bool {
		return ch.ring[i].hash < ch.ring[j].hash
	})

	return nil
}

func (ch *ConsistentHash) RemoveServer(address string) error {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	server, ok := ch.pool.GetServer(address)
	if !ok {
		return ErrServerNotFound
	}

	if err := ch.pool.RemoveServer(address); err != nil {
		return err
	}

	vnCount := ch.virtualNodes * server.Weight
	for i := 0; i < vnCount; i++ {
		virtualKey := fmt.Sprintf("%s#vn%d", address, i)
		hash := hashKey(virtualKey)
		delete(ch.hashMap, hash)
	}

	newRing := make([]hashNode, 0, len(ch.ring))
	for _, hn := range ch.ring {
		if _, exists := ch.hashMap[hn.hash]; exists {
			newRing = append(newRing, hn)
		}
	}
	ch.ring = newRing

	return nil
}

func (ch *ConsistentHash) Servers() []*BackendServer {
	return ch.pool.GetAllServers()
}

func (ch *ConsistentHash) DrainServer(address string) error {
	return ch.pool.DrainServer(address)
}

func (ch *ConsistentHash) RestoreServer(address string) error {
	return ch.pool.RestoreServer(address)
}

func (ch *ConsistentHash) ServerCount() int {
	return ch.pool.ServerCount()
}

func (ch *ConsistentHash) HealthyCount() int {
	return ch.pool.HealthyCount()
}
