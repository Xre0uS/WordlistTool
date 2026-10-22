package generator

const (
	hsFlagBit = uint64(1) << 63
	hsFPMask  = hsFlagBit - 1
)

// fingerprint63 hashes b into a non-zero 63-bit fingerprint: FNV-1a plus the
// murmur3 finaliser, which fixes FNV's weak low-order avalanche.
func fingerprint63(b []byte) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)

	h := uint64(offset64)
	for i := 0; i < len(b); i++ {
		h ^= uint64(b[i])
		h *= prime64
	}

	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33

	h &= hsFPMask
	if h == 0 {
		h = 1
	}
	return h
}

// fingerprint63String is fingerprint63 for a string, skipping the byte copy.
func fingerprint63String(s string) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)

	h := uint64(offset64)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime64
	}

	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33

	h &= hsFPMask
	if h == 0 {
		h = 1
	}
	return h
}

// hashSet is an open-addressing set of 63-bit fingerprints, each carrying one
// flag bit; a slot of 0 means empty. Fingerprints rather than words keeps this
// to 8 bytes per slot, at a collision risk of about n^2 / 2^64 — roughly 1 in
// 3700 at 100M entries. See wordSet for the variant without that risk.
type hashSet struct {
	slots  []uint64
	mask   uint64
	count  int64
	growAt int64
}

// hashSetSlotsFor returns the slots newHashSet allocates, load factor <= 0.5.
func hashSetSlotsFor(expected int64) uint64 {
	const maxSlots = uint64(1) << 42 // far past anything practical

	slots := uint64(1024)
	if expected <= 0 {
		return slots
	}
	want := uint64(expected) * 2
	for slots < want {
		if slots >= maxSlots {
			return maxSlots
		}
		slots <<= 1
	}
	return slots
}

func newHashSet(expected int64) *hashSet {
	slots := hashSetSlotsFor(expected)
	return &hashSet{
		slots:  make([]uint64, slots),
		mask:   slots - 1,
		growAt: int64(slots / 2),
	}
}

// Lookup reports whether fp is present and the flag it was stored with.
func (h *hashSet) Lookup(fp uint64) (present, flag bool) {
	idx := fp & h.mask
	for {
		cur := h.slots[idx]
		if cur == 0 {
			return false, false
		}
		if cur&hsFPMask == fp {
			return true, cur&hsFlagBit != 0
		}
		idx = (idx + 1) & h.mask
	}
}

// Insert adds fp with the given flag unless present, returning whether it was
// present and the flag of the entry already there.
func (h *hashSet) Insert(fp uint64, flag bool) (present, existingFlag bool) {
	idx := fp & h.mask
	for {
		cur := h.slots[idx]
		if cur == 0 {
			v := fp
			if flag {
				v |= hsFlagBit
			}
			h.slots[idx] = v
			h.count++
			if h.count >= h.growAt {
				h.grow()
			}
			return false, false
		}
		if cur&hsFPMask == fp {
			return true, cur&hsFlagBit != 0
		}
		idx = (idx + 1) & h.mask
	}
}

// grow doubles the table. Peak memory is three times the old one, so size the
// set up front.
func (h *hashSet) grow() {
	old := h.slots
	n := uint64(len(old)) * 2

	h.slots = make([]uint64, n)
	h.mask = n - 1
	h.growAt = int64(n / 2)

	for _, v := range old {
		if v == 0 {
			continue
		}
		idx := (v & hsFPMask) & h.mask
		for h.slots[idx] != 0 {
			idx = (idx + 1) & h.mask
		}
		h.slots[idx] = v
	}
}

func (h *hashSet) Len() int64 { return h.count }

func (h *hashSet) memoryBytes() int64 { return int64(len(h.slots)) * 8 }
