package generator

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// A run plans one large allocation up front, so it can say no before it starts
// rather than being OOM-killed halfway through — or taking something else on the
// box down with it, which is the failure that actually hurts. memBudget is the
// ceiling it plans against.
const (
	memBudgetShare = 75        // percent of the memory on offer a run will plan to use
	memBudgetFloor = 256 << 20 // a small job runs however busy the machine is
)

// parseMemAvailable reads MemAvailable out of /proc/meminfo content and returns
// it in bytes. The kernel's own estimate of what it can hand out without
// swapping is a better basis than MemFree, which ignores reclaimable cache.
func parseMemAvailable(r io.Reader) (int64, bool) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		rest, ok := strings.CutPrefix(sc.Text(), "MemAvailable:")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return 0, false
		}
		kb, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil || kb < 0 {
			return 0, false
		}
		return kb * 1024, true
	}
	return 0, false
}

// parseCgroupMemoryMax returns the headroom left in a cgroup v2 memory limit,
// from the contents of memory.max and memory.current. A limit of "max" is no
// limit at all, which reads as not found rather than as zero headroom.
func parseCgroupMemoryMax(max, current string) (int64, bool) {
	max = strings.TrimSpace(max)
	if max == "" || max == "max" {
		return 0, false
	}
	limit, err := strconv.ParseInt(max, 10, 64)
	if err != nil || limit <= 0 {
		return 0, false
	}

	// An unreadable current reads as nothing used: the limit still binds, and
	// overstating the headroom is better than refusing a run over a parse slip.
	used, err := strconv.ParseInt(strings.TrimSpace(current), 10, 64)
	if err != nil || used < 0 {
		used = 0
	}
	if used >= limit {
		return 0, true
	}
	return limit - used, true
}

// parseCgroupV2Path returns the unified-hierarchy path from /proc/self/cgroup,
// which is the line whose hierarchy id and controller list are both empty.
func parseCgroupV2Path(r io.Reader) (string, bool) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if path, ok := strings.CutPrefix(sc.Text(), "0::"); ok {
			return path, true
		}
	}
	return "", false
}

// memBudget is the ceiling a run plans against. A limit of 0 means no ceiling,
// which is what an unknown figure has to mean: refusing every run because the
// platform will not say how much memory it has would be worse than the OOM it
// is meant to prevent.
type memBudget struct {
	limit     int64  // 0 means uncapped
	available int64  // what was detected, 0 when unknown
	flag      string // the flag that sets it, for the remedies
	source    string // how the limit was arrived at, for the banner
}

// newMemBudget takes the cap in MB (0 when the flag was not given) and the name
// of the flag that sets it.
func newMemBudget(explicitMB int, flagName string) memBudget {
	if explicitMB > 0 {
		return memBudget{
			limit:  int64(explicitMB) << 20,
			flag:   flagName,
			source: flagName,
		}
	}

	avail, ok := availableMemory()
	if !ok || avail <= 0 {
		return memBudget{flag: flagName, source: "unknown"}
	}

	limit := avail / 100 * memBudgetShare
	if limit < memBudgetFloor {
		limit = memBudgetFloor
	}
	return memBudget{
		limit:     limit,
		available: avail,
		flag:      flagName,
		source:    fmt.Sprintf("%d%% of %s free", memBudgetShare, humanSize(avail)),
	}
}

// uncappedBudget is what -no-mem-cap asks for.
func uncappedBudget(flagName string) memBudget {
	return memBudget{flag: flagName, source: "uncapped on request"}
}

func (m memBudget) unlimited() bool { return m.limit <= 0 }

func (m memBudget) exceeded(planned int64) bool {
	return !m.unlimited() && planned > m.limit
}

// clampMB lowers a megabyte figure to the budget, for the modes that size a
// filter to a number rather than to the data.
func (m memBudget) clampMB(mb int) int {
	if m.unlimited() {
		return mb
	}
	if limitMB := int(m.limit >> 20); limitMB > 0 && mb > limitMB {
		return limitMB
	}
	return mb
}

// describe is the banner line.
func (m memBudget) describe() string {
	if m.unlimited() {
		if m.source == "unknown" {
			return "uncapped (this platform does not report free memory)"
		}
		return "uncapped on request"
	}
	return fmt.Sprintf("%s (%s)", humanSize(m.limit), m.source)
}

// refusal states what was planned against what is allowed. The remedies differ
// per command, so the caller adds those.
func (m memBudget) refusal(planned int64) string {
	return fmt.Sprintf("this run plans %s, over the %s memory budget (%s)",
		humanSize(planned), humanSize(m.limit), m.source)
}
