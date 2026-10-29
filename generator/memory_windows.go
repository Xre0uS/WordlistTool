//go:build windows

package generator

import (
	"syscall"
	"unsafe"
)

// memoryStatusEx mirrors MEMORYSTATUSEX. The first field is the struct's own
// size, which GlobalMemoryStatusEx requires the caller to fill in before the
// call or it refuses.
type memoryStatusEx struct {
	length               uint32
	memoryLoad           uint32
	totalPhys            uint64
	availPhys            uint64
	totalPageFile        uint64
	availPageFile        uint64
	totalVirtual         uint64
	availVirtual         uint64
	availExtendedVirtual uint64
}

// availableMemory reports the physical memory still free. The page file is left
// out on purpose: a run that fits only by swapping is a run that thrashes.
func availableMemory() (int64, bool) {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

	var st memoryStatusEx
	st.length = uint32(unsafe.Sizeof(st))

	r, _, _ := proc.Call(uintptr(unsafe.Pointer(&st)))
	if r == 0 || st.availPhys == 0 {
		return 0, false
	}
	return int64(st.availPhys), true
}
