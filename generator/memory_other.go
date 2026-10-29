//go:build !linux && !windows

package generator

// availableMemory has no portable answer, so every other platform runs
// uncapped: the caller treats a false here as no ceiling rather than as none
// allowed.
func availableMemory() (int64, bool) { return 0, false }
