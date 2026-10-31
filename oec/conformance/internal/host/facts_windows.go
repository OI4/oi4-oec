package host

import (
	"net"
	"os"
	"syscall"
	"unsafe"

	"github.com/OI4/oec/conformance/internal/engine"
)

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	globalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	getDiskFreeSpaceExW  = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// memoryStatusEx is MEMORYSTATUSEX.
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

func local(f *Facts) {
	f.Machine = os.Getenv("PROCESSOR_ARCHITEW6432")
	if f.Machine == "" {
		f.Machine = os.Getenv("PROCESSOR_ARCHITECTURE")
	}
	m := memoryStatusEx{length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	if r, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m))); r != 0 {
		f.Memory = m.totalPhys
	}
	drive := os.Getenv("SystemDrive") + `\`
	if p, err := syscall.UTF16PtrFromString(drive); err == nil {
		var free, total, totalFree uint64
		if r, _, _ := getDiskFreeSpaceExW.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&free)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&totalFree))); r != 0 {
			f.Storage, f.StorageSource = total, "volume "+drive
		}
	}
	f.Daemons = engine.Defaults()
}

func physical(i net.Interface) bool {
	return i.Flags&net.FlagLoopback == 0 && len(i.HardwareAddr) > 0
}
