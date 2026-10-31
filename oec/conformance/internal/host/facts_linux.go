package host

import (
	"bufio"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/OI4/oec/conformance/internal/engine"
)

func local(f *Facts) {
	var u syscall.Utsname
	if syscall.Uname(&u) == nil {
		f.Kernel, f.Machine = chars(u.Release[:]), chars(u.Machine[:])
	}
	f.Memory = memTotal()
	f.Storage, f.StorageSource = blockDevices()
	if f.Storage == 0 {
		var st syscall.Statfs_t
		if syscall.Statfs("/", &st) == nil {
			f.Storage, f.StorageSource = st.Blocks*uint64(st.Bsize), "file system /"
		}
	}
	f.Distribution = osRelease("/etc/os-release")
	f.InContainer = exists("/.dockerenv") || exists("/run/.containerenv")
	f.RebootRequired = exists("/var/run/reboot-required")
	f.Daemons = append(engine.Defaults(), "unix:///run/containerd/containerd.sock", "unix:///run/k3s/containerd/containerd.sock", "unix:///var/run/crio/crio.sock")
}

// chars converts a NUL-terminated field of Utsname (int8 or uint8 by
// architecture).
func chars[T int8 | uint8](a []T) string {
	b := make([]byte, 0, len(a))
	for _, c := range a {
		if c == 0 {
			break
		}
		b = append(b, byte(c))
	}
	return string(b)
}

// memTotal is MemTotal of /proc/meminfo in octets.
func memTotal() uint64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) >= 2 && fs[0] == "MemTotal:" {
			v, err := strconv.ParseUint(fs[1], 10, 64)
			if err != nil {
				return 0
			}
			if len(fs) > 2 && fs[2] == "kB" {
				v *= 1024
			}
			return v
		}
	}
	return 0
}

// blockDevices sums the size of the block devices of a device (not loop,
// ram, device mapper) in /sys/block.
func blockDevices() (uint64, string) {
	ents, err := os.ReadDir("/sys/block")
	if err != nil {
		return 0, ""
	}
	var sum uint64
	var names []string
	for _, e := range ents {
		dir := filepath.Join("/sys/block", e.Name())
		if !exists(filepath.Join(dir, "device")) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, "size"))
		if err != nil {
			continue
		}
		v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
		if err != nil || v == 0 {
			continue
		}
		sum += v * 512
		names = append(names, e.Name())
	}
	if sum == 0 {
		return 0, ""
	}
	sort.Strings(names)
	return sum, "block devices " + strings.Join(names, ", ")
}

func physical(i net.Interface) bool {
	return i.Flags&net.FlagLoopback == 0 && exists(filepath.Join("/sys/class/net", i.Name, "device"))
}
