//go:build linux

package generator

import (
	"os"
	"path"
)

// availableMemory reports what a run can reasonably allocate: the kernel's
// MemAvailable, lowered to the headroom left in any cgroup v2 memory limit that
// applies. A container capped at 2 GB on a 96 GB host gets 2 GB.
func availableMemory() (int64, bool) {
	var best int64
	var ok bool

	if f, err := os.Open("/proc/meminfo"); err == nil {
		if n, found := parseMemAvailable(f); found {
			best, ok = n, true
		}
		f.Close()
	}

	for _, n := range cgroupHeadroom() {
		if !ok || n < best {
			best, ok = n, true
		}
	}

	return best, ok
}

// cgroupHeadroom walks the process's own cgroup and every ancestor, because a
// limit set anywhere above it binds just as tightly.
func cgroupHeadroom() []int64 {
	f, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return nil
	}
	defer f.Close()

	rel, found := parseCgroupV2Path(f)
	if !found {
		return nil
	}

	const root = "/sys/fs/cgroup"
	var out []int64

	for dir := path.Join(root, rel); ; dir = path.Dir(dir) {
		max, err := os.ReadFile(path.Join(dir, "memory.max"))
		if err == nil {
			cur, _ := os.ReadFile(path.Join(dir, "memory.current"))
			if n, limited := parseCgroupMemoryMax(string(max), string(cur)); limited {
				out = append(out, n)
			}
		}
		if dir == root || dir == "/" || dir == "." {
			return out
		}
	}
}
