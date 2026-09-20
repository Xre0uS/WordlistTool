package generator

import (
	"math"
	"unsafe"
)

// bloom is a memory-efficient bloom filter for approximate deduplication.
type bloom struct {
	bits []uint64
	k    uint   // number of hash functions
	m    uint64 // total bits
}

// newBloom creates a bloom filter sized for expectedItems at the given false-positive rate.
// maxMemoryMB caps memory; if the optimal size exceeds this, the FPR will be higher.
func newBloom(expectedItems int64, fpr float64, maxMemoryMB int) *bloom {
	if expectedItems <= 0 {
		expectedItems = 1_000_000
	}
	if fpr <= 0 || fpr >= 1 {
		fpr = 0.01
	}

	// Optimal bit count: m = -n*ln(p) / (ln2)^2
	n := float64(expectedItems)
	m := uint64(math.Ceil(-n * math.Log(fpr) / (math.Ln2 * math.Ln2)))

	// Optimal hash count: k = (m/n) * ln2
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

func (b *bloom) memoryMB() int {
	return int(unsafe.Sizeof(uint64(0)) * uintptr(len(b.bits)) / (1024 * 1024))
}

func (b *bloom) memoryBytes() int64 {
	return int64(unsafe.Sizeof(uint64(0))) * int64(len(b.bits))
}

// fpr returns the false-positive rate with n items stored: (1 - e^(-kn/m))^k.
func (b *bloom) fpr(n int64) float64 {
	if n <= 0 || b.m == 0 {
		return 0
	}
	exp := math.Exp(-float64(b.k) * float64(n) / float64(b.m))
	return math.Pow(1-exp, float64(b.k))
}

// TestAndAdd returns true if the item was likely already present, then adds it.
func (b *bloom) TestAndAdd(data string) bool {
	h1, h2 := b.hash(data)
	return b.testAndAdd(h1, h2)
}

// TestAndAddBytes is TestAndAdd for a byte slice, skipping the string copy.
func (b *bloom) TestAndAddBytes(data []byte) bool {
	h1, h2 := b.hashBytes(data)
	return b.testAndAdd(h1, h2)
}

// TestBytes reports whether the item was likely added, without adding it.
func (b *bloom) TestBytes(data []byte) bool {
	h1, h2 := b.hashBytes(data)
	for i := uint(0); i < b.k; i++ {
		pos := (h1 + uint64(i)*h2) % b.m
		if b.bits[pos/64]&(uint64(1)<<(pos%64)) == 0 {
			return false
		}
	}
	return true
}

func (b *bloom) testAndAdd(h1, h2 uint64) bool {
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

func (b *bloom) hash(s string) (uint64, uint64) {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)

	h1 := uint64(offset64)
	h2 := uint64(offset64 ^ 0x6c62272e07bb0142)

	for i := 0; i < len(s); i++ {
		c := uint64(s[i])
		h1 ^= c
		h1 *= prime64
		h2 ^= c
		h2 *= prime64 + 2
	}

	return h1, h2
}

func (b *bloom) hashBytes(s []byte) (uint64, uint64) {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)

	h1 := uint64(offset64)
	h2 := uint64(offset64 ^ 0x6c62272e07bb0142)

	for i := 0; i < len(s); i++ {
		c := uint64(s[i])
		h1 ^= c
		h1 *= prime64
		h2 ^= c
		h2 *= prime64 + 2
	}

	return h1, h2
}
