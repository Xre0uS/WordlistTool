package generator

import (
	"fmt"
	"strings"
	"testing"
)

func TestWordSetRoundTrip(t *testing.T) {
	// A tiny initial table forces heavy probing and repeated growth.
	w := newWordSet(1)

	const n = 200_000
	for i := 0; i < n; i++ {
		word := []byte(fmt.Sprintf("candidate-%d!", i))
		if present, _ := w.Insert(word, i%2 == 0); present {
			t.Fatalf("word %d reported as already present", i)
		}
	}
	if w.Len() != n {
		t.Fatalf("Len() = %d, want %d", w.Len(), n)
	}

	for i := 0; i < n; i++ {
		word := []byte(fmt.Sprintf("candidate-%d!", i))
		present, flag := w.Lookup(word)
		if !present {
			t.Fatalf("word %d missing after growth", i)
		}
		if flag != (i%2 == 0) {
			t.Fatalf("word %d flag = %v, want %v", i, flag, i%2 == 0)
		}
	}

	for i := 0; i < 1000; i++ {
		if present, _ := w.Lookup([]byte(fmt.Sprintf("absent-%d", i))); present {
			t.Fatalf("absent word %d reported present", i)
		}
	}
}

// A word that does not fit pads to the next chunk; one larger than a chunk gets
// a chunk of its own.
func TestWordSetChunkBoundaries(t *testing.T) {
	w := newWordSet(1024)

	words := [][]byte{
		[]byte("short"),
		[]byte(strings.Repeat("a", wsFirstChunk-1)),
		[]byte(strings.Repeat("b", wsChunkSize+1000)),
		[]byte(strings.Repeat("c", wsChunkSize*2)),
		[]byte("after-the-big-ones"),
	}

	for _, word := range words {
		if present, _ := w.Insert(word, false); present {
			t.Fatalf("len %d reported as already present", len(word))
		}
	}
	for _, word := range words {
		present, _ := w.Lookup(word)
		if !present {
			t.Fatalf("len %d missing", len(word))
		}
	}
	if w.Len() != int64(len(words)) {
		t.Fatalf("Len() = %d, want %d", w.Len(), len(words))
	}
}

// Words sharing a bucket must all survive — the probe-and-compare path that
// keeps full mode free of fingerprint mode's collision risk.
func TestWordSetBucketCollisions(t *testing.T) {
	w := newWordSet(1)
	mask := w.mask

	var same [][]byte
	for i := 0; len(same) < 32; i++ {
		word := []byte(fmt.Sprintf("bucket-%d", i))
		if fingerprint63(word)&mask == 0 {
			same = append(same, word)
		}
	}

	for _, word := range same {
		w.Insert(word, false)
	}
	for _, word := range same {
		if present, _ := w.Lookup(word); !present {
			t.Fatalf("%q lost to a bucket collision", word)
		}
	}
}

func TestHashSetFlagAndGrowth(t *testing.T) {
	h := newHashSet(1)

	const n = 100_000
	for i := 0; i < n; i++ {
		fp := fingerprint63([]byte(fmt.Sprintf("word-%d", i)))
		if present, _ := h.Insert(fp, i%3 == 0); present {
			t.Fatalf("word %d reported as already present", i)
		}
	}
	for i := 0; i < n; i++ {
		fp := fingerprint63([]byte(fmt.Sprintf("word-%d", i)))
		present, flag := h.Insert(fp, false)
		if !present {
			t.Fatalf("word %d missing after growth", i)
		}
		if flag != (i%3 == 0) {
			t.Fatalf("word %d flag = %v, want %v", i, flag, i%3 == 0)
		}
	}
	if h.Len() != n {
		t.Fatalf("Len() = %d, want %d", h.Len(), n)
	}
}
