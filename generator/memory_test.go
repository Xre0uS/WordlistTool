package generator

import (
	"strings"
	"testing"
)

func TestParseMemAvailable(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		want  int64
		found bool
	}{
		{
			name: "realistic",
			body: "MemTotal:       98213620 kB\nMemFree:        55895256 kB\n" +
				"MemAvailable:   55276452 kB\nBuffers:          123 kB\n",
			want:  55276452 * 1024,
			found: true,
		},
		{
			name:  "first line",
			body:  "MemAvailable:   1024 kB\nMemTotal: 2048 kB\n",
			want:  1024 * 1024,
			found: true,
		},
		{
			name:  "odd whitespace",
			body:  "MemAvailable:\t\t 2048    kB\n",
			want:  2048 * 1024,
			found: true,
		},
		{
			name:  "absent",
			body:  "MemTotal:       98213620 kB\nMemFree:        55895256 kB\n",
			found: false,
		},
		{
			name:  "unparseable",
			body:  "MemAvailable:   lots kB\n",
			found: false,
		},
		{
			name:  "empty",
			body:  "",
			found: false,
		},
		{
			// A longer field name sharing the prefix must not satisfy it.
			name:  "near miss",
			body:  "MemAvailableHuge: 10 kB\n",
			found: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := parseMemAvailable(strings.NewReader(tt.body))
			if found != tt.found {
				t.Fatalf("found = %v, want %v", found, tt.found)
			}
			if found && got != tt.want {
				t.Errorf("got %d bytes, want %d", got, tt.want)
			}
		})
	}
}

func TestParseCgroupMemoryMax(t *testing.T) {
	tests := []struct {
		name    string
		max     string
		current string
		want    int64
		limited bool
	}{
		{name: "headroom", max: "2147483648\n", current: "1073741824\n", want: 1073741824, limited: true},
		{name: "no limit", max: "max\n", current: "123\n", limited: false},
		{name: "empty", max: "", current: "", limited: false},
		{name: "over limit", max: "1000", current: "2000", want: 0, limited: true},
		{name: "unreadable current", max: "4096", current: "garbage", want: 4096, limited: true},
		{name: "unparseable max", max: "garbage", current: "0", limited: false},
		{name: "zero max", max: "0", current: "0", limited: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, limited := parseCgroupMemoryMax(tt.max, tt.current)
			if limited != tt.limited {
				t.Fatalf("limited = %v, want %v", limited, tt.limited)
			}
			if limited && got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseCgroupV2Path(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		want  string
		found bool
	}{
		{
			name:  "unified",
			body:  "0::/user.slice/user-1000.slice/app.slice/some.scope\n",
			want:  "/user.slice/user-1000.slice/app.slice/some.scope",
			found: true,
		},
		{name: "root", body: "0::/\n", want: "/", found: true},
		{
			name:  "v1 only",
			body:  "11:memory:/user.slice\n4:cpu,cpuacct:/user.slice\n",
			found: false,
		},
		{
			name:  "v1 then unified",
			body:  "11:memory:/user.slice\n0::/wanted\n",
			want:  "/wanted",
			found: true,
		},
		{name: "empty", body: "", found: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := parseCgroupV2Path(strings.NewReader(tt.body))
			if found != tt.found {
				t.Fatalf("found = %v, want %v", found, tt.found)
			}
			if found && got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMemoryBudget(t *testing.T) {
	t.Run("explicit wins", func(t *testing.T) {
		b := newMemBudget(2048, "-mem")
		if b.unlimited() {
			t.Fatal("an explicit cap must not be unlimited")
		}
		if b.limit != 2048<<20 {
			t.Errorf("limit = %d, want %d", b.limit, 2048<<20)
		}
		if b.source != "-mem" {
			t.Errorf("source = %q, want the flag name", b.source)
		}
	})

	t.Run("derived or unlimited", func(t *testing.T) {
		// Whatever this platform reports, the result must be usable: either a
		// cap at or above the floor, or no cap at all. Never a tiny or
		// negative ceiling that would refuse every run.
		b := newMemBudget(0, "-mem")
		if b.unlimited() {
			if b.limit != 0 {
				t.Errorf("unlimited budget carries limit %d, want 0", b.limit)
			}
			return
		}
		if b.limit < memBudgetFloor {
			t.Errorf("limit %d is under the %d floor", b.limit, int64(memBudgetFloor))
		}
		if b.available <= 0 {
			t.Errorf("derived budget reports %d available", b.available)
		}
	})

	t.Run("uncapped on request", func(t *testing.T) {
		b := uncappedBudget("-mem")
		if !b.unlimited() {
			t.Fatal("-no-mem-cap must be unlimited")
		}
		if b.exceeded(1 << 60) {
			t.Error("an unlimited budget cannot be exceeded")
		}
		if !strings.Contains(b.describe(), "uncapped") {
			t.Errorf("describe() = %q, want it to say uncapped", b.describe())
		}
	})

	t.Run("exceeded and clamp", func(t *testing.T) {
		b := newMemBudget(100, "-mem")
		if b.exceeded(100 << 20) {
			t.Error("planning exactly the budget must be allowed")
		}
		if !b.exceeded(101 << 20) {
			t.Error("planning over the budget must be refused")
		}
		if got := b.clampMB(1024); got != 100 {
			t.Errorf("clampMB(1024) = %d, want 100", got)
		}
		if got := b.clampMB(50); got != 50 {
			t.Errorf("clampMB(50) = %d, want 50 left alone", got)
		}

		u := uncappedBudget("-mem")
		if got := u.clampMB(4096); got != 4096 {
			t.Errorf("an uncapped budget clamped %d", got)
		}
	})

	t.Run("refusal names both figures", func(t *testing.T) {
		b := newMemBudget(100, "-mem")
		msg := b.refusal(64 << 30)
		for _, want := range []string{"64.00 GB", "100.00 MB"} {
			if !strings.Contains(msg, want) {
				t.Errorf("refusal %q is missing %q", msg, want)
			}
		}
	})
}

// The ceiling has to stop a set that outgrows its estimate, and it must stop it
// by failing rather than by quietly dropping words: a diff that loses an
// exclude word reports an untested word as tested.
func TestSetLimitStopsGrowth(t *testing.T) {
	t.Run("hashSet", func(t *testing.T) {
		h := newHashSet(0)
		h.SetLimit(64 << 10) // smaller than the first grow needs

		var inserted int64
		for i := 0; i < 1_000_000; i++ {
			if h.Err() != nil {
				break
			}
			if present, _ := h.Insert(fingerprint63([]byte(itoa(i))), false); !present {
				inserted++
			}
		}

		if h.Err() == nil {
			t.Fatal("the set grew past its limit without reporting it")
		}
		if h.Len() == 0 {
			t.Error("nothing was stored before the limit bit")
		}
		if h.memoryBytes() > 64<<10 {
			t.Errorf("held %s against a %s limit",
				humanSize(h.memoryBytes()), humanSize(64<<10))
		}
		// Once stopped it must stay stopped, not resume storing.
		before := h.Len()
		h.Insert(fingerprint63([]byte("after-the-limit")), false)
		if h.Len() != before {
			t.Error("the set kept storing after the limit")
		}
	})

	t.Run("wordSet", func(t *testing.T) {
		// Room for several arena chunks, so words land before the ceiling bites;
		// a limit under one chunk legitimately refuses the very first word.
		const limit = 1 << 20
		w := newWordSet(0)
		w.SetLimit(limit)

		for i := 0; i < 1_000_000; i++ {
			if w.Err() != nil {
				break
			}
			w.Insert([]byte("candidate-"+itoa(i)), false)
		}

		if w.Err() == nil {
			t.Fatal("the set grew past its limit without reporting it")
		}
		if w.Len() == 0 {
			t.Error("nothing was stored before the limit bit")
		}
		if w.memoryBytes() > limit {
			t.Errorf("held %s against a %s limit",
				humanSize(w.memoryBytes()), humanSize(limit))
		}
	})

	t.Run("a limit under one arena chunk refuses cleanly", func(t *testing.T) {
		w := newWordSet(0)
		w.SetLimit(1 << 10)
		w.Insert([]byte("word"), false)
		if w.Err() == nil {
			t.Fatal("want a refusal, got none")
		}
		if w.Len() != 0 {
			t.Errorf("stored %d words it had no room for", w.Len())
		}
	})

	t.Run("no limit grows freely", func(t *testing.T) {
		h := newHashSet(0)
		for i := 0; i < 200_000; i++ {
			h.Insert(fingerprint63([]byte(itoa(i))), false)
		}
		if h.Err() != nil {
			t.Fatalf("an unlimited set reported %v", h.Err())
		}
		if h.Len() == 0 {
			t.Error("an unlimited set stored nothing")
		}
	})
}

// itoa keeps the test free of a strconv import clash with the table above.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	n := len(b)
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	return string(b[n:])
}
