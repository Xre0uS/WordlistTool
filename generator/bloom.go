package generator

import (
	"math"
	"unsafe"
)

// bloom implements a memory-efficient bloom filter for approximate deduplication.
// Uses two independent hash functions (FNV-1a based) to set/check k positions.
//
// Memory usage: ~bits/8 bytes. For 500M expected items at 1% FPR: ~570MB.
// For 100M items at 1% FPR: ~114MB.
type bloom struct {
	bits []uint64
	k    uint   // number of hash functions
	m    uint64 // total bits
}

// newBloom creates a bloom filter sized for expectedItems at the given false-positive rate.
// maxMemoryMB caps the memory usage; if the optimal size exceeds this, the FPR will be higher.
func newBloom(expectedItems int64, fpr float64, maxMemoryMB int) *bloom {
	if expectedItems <= 0 {
		expectedItems = 1_000_000
	}
	if fpr <= 0 || fpr >= 1 {
		fpr = 0.01
	}

	// Optimal number of bits: m = -n*ln(p) / (ln2)^2
	n := float64(expectedItems)
	m := uint64(math.Ceil(-n * math.Log(fpr) / (math.Ln2 * math.Ln2)))

	// Optimal number of hash functions: k = (m/n) * ln2
	k := uint(math.Ceil(float64(m) / n * math.Ln2))
	if k < 2 {
		k = 2
	}
	if k > 16 {
		k = 16
	}

	// Cap memory
	maxBits := uint64(maxMemoryMB) * 1024 * 1024 * 8
	if m > maxBits {
		m = maxBits
	}

	// Round up to multiple of 64
	words := (m + 63) / 64
	m = words * 64

	return &bloom{
		bits: make([]uint64, words),
		k:    k,
		m:    m,
	}
}

// memoryMB returns the approximate memory usage in MB.
func (b *bloom) memoryMB() int {
	return int(unsafe.Sizeof(uint64(0)) * uintptr(len(b.bits)) / (1024 * 1024))
}

// AddAndCheck atomically adds the item and returns true if it was likely already present.
// This is NOT thread-safe — each goroutine should have its own bloom or use external sync.
func (b *bloom) TestAndAdd(data string) bool {
	h1, h2 := b.hash(data)
	alreadyPresent := true
	for i := uint(0); i < b.k; i++ {
		pos := (h1 + uint64(i)*h2) % b.m
		wordIdx := pos / 64
		bitIdx := pos % 64
		mask := uint64(1) << bitIdx
		if b.bits[wordIdx]&mask == 0 {
			alreadyPresent = false
			b.bits[wordIdx] |= mask
		}
	}
	return alreadyPresent
}

// hash computes two independent 64-bit hashes using FNV-1a with different offsets.
func (b *bloom) hash(s string) (uint64, uint64) {
	// FNV-1a 64-bit
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)

	h1 := uint64(offset64)
	h2 := uint64(offset64 ^ 0x6c62272e07bb0142) // different seed

	for i := 0; i < len(s); i++ {
		c := uint64(s[i])
		h1 ^= c
		h1 *= prime64
		h2 ^= c
		h2 *= prime64 + 2 // slightly different multiplier
	}

	return h1, h2
}
