package kvstore

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
)

type BloomFilter struct {
	bitArray  []bool
	size      uint
	hashCount uint
}

func NewBloomFilter(capacity uint, falsePositiveRate float64) *BloomFilter {
	if capacity == 0 {
		capacity = 1000
	}
	if falsePositiveRate <= 0 || falsePositiveRate >= 1 {
		falsePositiveRate = 0.01
	}

	size := optimalSize(capacity, falsePositiveRate)
	hashCount := optimalHashCount(size, capacity)

	return &BloomFilter{
		bitArray:  make([]bool, size),
		size:      size,
		hashCount: hashCount,
	}
}

func optimalSize(capacity uint, falsePositiveRate float64) uint {
	return uint(math.Ceil(-float64(capacity) * math.Log(falsePositiveRate) / (math.Log(2) * math.Log(2))))
}

func optimalHashCount(size uint, capacity uint) uint {
	return uint(math.Ceil(float64(size) / float64(capacity) * math.Log(2)))
}

func doubleHash(key string, size uint) (uint, uint) {
	sum := sha256.Sum256([]byte(key))
	h1 := binary.BigEndian.Uint64(sum[0:8])
	h2 := binary.BigEndian.Uint64(sum[8:16])
	return uint(h1 % uint64(size)), uint(h2 % uint64(size))
}

func (bf *BloomFilter) Add(key string) {
	h1, h2 := doubleHash(key, bf.size)
	for i := uint(0); i < bf.hashCount; i++ {
		index := (h1 + i*h2) % bf.size
		bf.bitArray[index] = true
	}
}

func (bf *BloomFilter) MightContain(key string) bool {
	h1, h2 := doubleHash(key, bf.size)
	for i := uint(0); i < bf.hashCount; i++ {
		index := (h1 + i*h2) % bf.size
		if !bf.bitArray[index] {
			return false
		}
	}
	return true
}

func (bf *BloomFilter) Reset() {
	for i := range bf.bitArray {
		bf.bitArray[i] = false
	}
}

func (bf *BloomFilter) Size() uint {
	return bf.size
}

func (bf *BloomFilter) HashCount() uint {
	return bf.hashCount
}
