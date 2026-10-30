package host

import (
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

func local(f *Facts) {
	f.Machine, _ = syscall.Sysctl("hw.machine")
	f.Kernel, _ = syscall.Sysctl("kern.osrelease")
	if s, err := syscall.Sysctl("hw.memsize"); err == nil {
		// Sysctl returns the octets of the uint64 without trailing NULs.
		b := make([]byte, 8)
		copy(b, s)
		f.Memory = binary.LittleEndian.Uint64(b)
	}
	var st syscall.Statfs_t
	if syscall.Statfs("/", &st) == nil {
		f.Storage, f.StorageSource = st.Blocks*uint64(st.Bsize), "file system /"
	}
	f.Daemons = []string{"unix:///var/run/docker.sock"}
	if h, err := os.UserHomeDir(); err == nil {
		f.Daemons = append(f.Daemons, "unix://"+filepath.Join(h, ".docker", "run", "docker.sock"))
	}
}

func physical(i net.Interface) bool {
	return i.Flags&net.FlagLoopback == 0 && len(i.HardwareAddr) > 0
}
