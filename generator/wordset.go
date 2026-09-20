package generator

import (
	"bytes"
	"encoding/binary"
)

const (
	wsChunkBits  = 23
	wsChunkSize  = 1 << wsChunkBits // max arena chunk, 8 MB
	wsFirstChunk = 64 << 10         // first chunk; doubles up to wsChunkSize

	wsFlagBit  = uint64(1) << 63
	wsTagShift = 56
	wsTagMask  = uint64(0x7f) << wsTagShift
	wsPosMask  = (uint64(1) << wsTagShift) - 1
)

// wordSet is an exact set of words: it probes on a hash but confirms each hit
// against the stored bytes, so no collision can drop a word. Words live in an
// arena of chunks, length-prefixed as a uvarint; a slot packs the emitted flag
// (bit 63), 7 hash bits as a tag (56-62) and the arena position plus one
// (0-55), 0 meaning empty. Neither part holds pointers, so the GC has nothing
// to scan, and the tag settles most probe mismatches without an arena read.
type wordSet struct {
	slots  []uint64
	mask   uint64
	count  int64
	growAt int64

	chunks     [][]byte
	cur        int // chunk being written, -1 before the first
	off        int // write offset within it
	nextSize   int
	arenaBytes int64
}

// wordSetBytesFor estimates memory for expected words over arenaBytes of data.
func wordSetBytesFor(expected, arenaBytes int64) int64 {
	return int64(hashSetSlotsFor(expected))*8 + arenaBytes
}

func newWordSet(expected int64) *wordSet {
	slots := hashSetSlotsFor(expected)
	return &wordSet{
		slots:    make([]uint64, slots),
		mask:     slots - 1,
		growAt:   int64(slots / 2),
		cur:      -1,
		nextSize: wsFirstChunk,
	}
}

// Lookup reports whether word is present and the flag it was stored with.
func (w *wordSet) Lookup(word []byte) (present, flag bool) {
	h := fingerprint63(word)
	tag := (h >> 49) & 0x7f
	idx := h & w.mask

	for {
		cur := w.slots[idx]
		if cur == 0 {
			return false, false
		}
		if (cur&wsTagMask)>>wsTagShift == tag && bytes.Equal(w.wordAt(cur), word) {
			return true, cur&wsFlagBit != 0
		}
		idx = (idx + 1) & w.mask
	}
}

// Insert adds word with the given flag unless present, returning whether it was
// present and the flag of the entry already there.
func (w *wordSet) Insert(word []byte, flag bool) (present, existingFlag bool) {
	h := fingerprint63(word)
	tag := (h >> 49) & 0x7f
	idx := h & w.mask

	for {
		cur := w.slots[idx]
		if cur == 0 {
			v := (uint64(w.store(word))+1)&wsPosMask | tag<<wsTagShift
			if flag {
				v |= wsFlagBit
			}
			w.slots[idx] = v
			w.count++
			if w.count >= w.growAt {
				w.grow()
			}
			return false, false
		}
		if (cur&wsTagMask)>>wsTagShift == tag && bytes.Equal(w.wordAt(cur), word) {
			return true, cur&wsFlagBit != 0
		}
		idx = (idx + 1) & w.mask
	}
}

// store appends word to the arena and returns its logical position.
func (w *wordSet) store(word []byte) int64 {
	var hdr [binary.MaxVarintLen64]byte
	hn := binary.PutUvarint(hdr[:], uint64(len(word)))
	need := hn + len(word)

	if w.cur < 0 || w.off+need > len(w.chunks[w.cur]) {
		w.newChunk(need)
	}

	pos := int64(w.cur)<<wsChunkBits + int64(w.off)
	c := w.chunks[w.cur]
	copy(c[w.off:], hdr[:hn])
	copy(c[w.off+hn:], word)
	w.off += need

	// An oversized chunk holds one word at offset 0; the rest lies in the next
	// chunk's address range, so seal it.
	if len(c) > wsChunkSize {
		w.off = len(c)
	}

	return pos
}

// newChunk ramps from wsFirstChunk up to wsChunkSize, so a small run does not
// pay for a full chunk.
func (w *wordSet) newChunk(need int) {
	size := w.nextSize
	if need > size {
		size = need
	}
	if w.nextSize < wsChunkSize {
		w.nextSize *= 2
	}
	w.chunks = append(w.chunks, make([]byte, size))
	w.cur = len(w.chunks) - 1
	w.off = 0
	w.arenaBytes += int64(size)
}

// wordAt returns the word a slot points at, aliasing the arena.
func (w *wordSet) wordAt(slot uint64) []byte {
	pos := int64(slot&wsPosMask) - 1
	c := w.chunks[pos>>wsChunkBits]
	off := int(pos & (wsChunkSize - 1))
	n, hn := binary.Uvarint(c[off:])
	start := off + hn
	return c[start : start+int(n)]
}

// grow doubles the index, rehashing each word out of the arena.
func (w *wordSet) grow() {
	old := w.slots
	n := uint64(len(old)) * 2

	w.slots = make([]uint64, n)
	w.mask = n - 1
	w.growAt = int64(n / 2)

	for _, v := range old {
		if v == 0 {
			continue
		}
		idx := fingerprint63(w.wordAt(v)) & w.mask
		for w.slots[idx] != 0 {
			idx = (idx + 1) & w.mask
		}
		w.slots[idx] = v
	}
}

func (w *wordSet) Len() int64 { return w.count }

func (w *wordSet) memoryBytes() int64 {
	return int64(len(w.slots))*8 + w.arenaBytes
}
