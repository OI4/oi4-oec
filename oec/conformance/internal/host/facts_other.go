//go:build !linux && !darwin && !windows

package host

import "net"

// local: the properties of other systems are unknown.
func local(*Facts) {}

func physical(i net.Interface) bool {
	return i.Flags&net.FlagLoopback == 0 && len(i.HardwareAddr) > 0
}
