package host

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/report"
)

// Facts are the measured properties of the platform host (RIG 5.1). A zero
// value is unknown.
type Facts struct {
	OS             string // runtime.GOOS of the system D8 runs on
	Kernel         string // kernel release
	Distribution   string // PRETTY_NAME of /etc/os-release (of the container image if D8 runs in a container)
	Machine        string // machine of the kernel: x86_64, aarch64, armv7l, …
	Memory         uint64 // main memory visible to the operating system in octets
	Storage        uint64 // non-volatile memory in octets
	StorageSource  string // what Storage measures
	Interfaces     []Interface
	InterfacesRead bool     // Interfaces were read (none is a result)
	Daemons        []string // default endpoints of container daemons on this system, in order
	InContainer    bool     // D8 runs in a container (/.dockerenv, /run/.containerenv)
	RebootRequired bool     // /var/run/reboot-required (Debian, Ubuntu)
}

// Interface is a network interface of the platform host.
type Interface struct {
	Name     string
	Up       bool
	Loopback bool
	Physical bool // Linux: a device (/sys/class/net/<name>/device); elsewhere: a hardware address
}

// Local measures the properties of the system D8 runs on.
func Local() *Facts {
	f := &Facts{OS: runtime.GOOS}
	local(f)
	if ifs, err := net.Interfaces(); err == nil {
		f.InterfacesRead = true
		for _, i := range ifs {
			f.Interfaces = append(f.Interfaces, Interface{Name: i.Name, Up: i.Flags&net.FlagUp != 0, Loopback: i.Flags&net.FlagLoopback != 0, Physical: physical(i)})
		}
	}
	return f
}

// Thresholds of RIG 5.1: the minimums are the nominal sizes of the
// installed memory with 1 GB = 10^9 octets (SF-116). The memory a system
// shows is less than its nominal size (firmware, kernel and graphics
// reservations; the user area of a flash medium), so a value somewhat below
// the minimum is judged manually against the data sheet.
const (
	MinMemory        = 1_000_000_000 // 1 GB RAM
	ManualMemory     = 768 << 20     // below: fail; up to MinMemory: manual
	MinStorage       = 4_000_000_000 // 4 GB non-volatile memory
	ManualStorage    = 3_500_000_000 // below: fail; up to MinStorage: manual
	defaultDaemonTry = 2 * time.Second
)

// Architecture maps the machine of a kernel to an architecture of RIG 5.1
// (ARMv7, ARMv8, amd64); "" if it is none of them.
func Architecture(machine string) string {
	switch strings.ToLower(machine) {
	case "x86_64", "amd64", "x64":
		return "amd64"
	case "aarch64", "arm64", "aarch64_be", "armv8l", "armv8b":
		return "ARMv8"
	case "armv7l", "armv7", "armv7hl", "armhf":
		return "ARMv7"
	}
	return ""
}

// platform judges T8-HST-04 (RIG 5.1).
func (r *runner) platform(ctx context.Context) {
	f := r.facts
	p := &report.Platform{OS: f.OS, Kernel: f.Kernel, Distribution: f.Distribution, Machine: f.Machine, Memory: f.Memory, Storage: f.Storage, StorageSource: f.StorageSource}
	r.info.Platform = p
	r.hardware(p)
	r.linux()
	r.daemon(ctx, p)
	r.messageBus(ctx, p)
}

// hardware judges REQ-PLT-001: architecture, memory, non-volatile memory
// and a network interface.
func (r *runner) hardware(p *report.Platform) {
	f := r.facts
	switch arch := Architecture(f.Machine); {
	case f.Machine == "":
		r.inconclusive(TPlatform, []string{reqHardware}, "", "the processor architecture is unknown on this system")
	case arch == "":
		r.violation(TPlatform, reqHardware, "", "processor architecture %s is none of ARMv7, ARMv8 and amd64 (RIG 5.1)", f.Machine)
	default:
		r.checked(TPlatform, reqHardware, "", "processor architecture %s (%s)", arch, f.Machine)
	}
	switch m := f.Memory; {
	case m == 0:
		r.inconclusive(TPlatform, []string{reqHardware}, "", "the main memory is unknown on this system")
	case m >= MinMemory:
		r.checked(TPlatform, reqHardware, "", "main memory %d octets", m)
	case m >= ManualMemory:
		r.reported(TPlatform, reqHardware, "main memory %d octets visible to the operating system, less than 1 GB (10^9 octets) by less than firmware and kernel usually reserve: compare the nominal size of the installed memory with the data sheet (RIG 5.1)", m)
	default:
		r.violation(TPlatform, reqHardware, "", "main memory %d octets, less than 1 GB (RIG 5.1)", m)
	}
	switch s := f.Storage; {
	case s == 0:
		r.inconclusive(TPlatform, []string{reqHardware}, "", "the non-volatile memory is unknown on this system")
	case s >= MinStorage:
		r.checked(TPlatform, reqHardware, "", "non-volatile memory %d octets (%s)", s, f.StorageSource)
	case s >= ManualStorage:
		r.reported(TPlatform, reqHardware, "non-volatile memory %d octets (%s), less than 4 GB (4·10^9 octets) by less than the nominal size of a medium usually exceeds its user area: compare the nominal size of the installed memory with the data sheet (RIG 5.1)", s, f.StorageSource)
	default:
		r.violation(TPlatform, reqHardware, "", "non-volatile memory %d octets (%s), less than 4 GB (RIG 5.1)", s, f.StorageSource)
	}
	if !f.InterfacesRead {
		r.inconclusive(TPlatform, []string{reqHardware}, "", "the network interfaces are unknown on this system")
		return
	}
	var up, down, virtual []string
	for _, i := range f.Interfaces {
		switch {
		case i.Loopback:
		case i.Physical && i.Up:
			up = append(up, i.Name)
		case i.Physical:
			down = append(down, i.Name)
		case i.Up:
			virtual = append(virtual, i.Name)
		}
	}
	switch {
	case len(up) > 0:
		p.Interfaces = up
		r.checked(TPlatform, reqHardware, "", "network interface %s", strings.Join(up, ", "))
	case len(down) > 0 || len(virtual) > 0:
		p.Interfaces = append(append([]string{}, down...), virtual...)
		r.reported(TPlatform, reqHardware, "no network interface of a device is up (down: %s; virtual: %s); in a container D8 sees the interfaces of the host only with --network host: check the interfaces of the edge computer (RIG 5.1)",
			orNone(down), orNone(virtual))
	default:
		r.violation(TPlatform, reqHardware, "", "no network interface besides loopback (RIG 5.1)")
	}
}

func orNone(s []string) string {
	if len(s) == 0 {
		return "none"
	}
	return strings.Join(s, ", ")
}

// linux judges REQ-PLT-003: a Linux-based distribution; the patch level is
// reported and judged manually.
func (r *runner) linux() {
	f := r.facts
	switch {
	case f.OS == "":
		r.inconclusive(TPlatform, []string{reqLinux}, "", "the operating system is unknown")
		return
	case f.OS != "linux":
		r.violation(TPlatform, reqLinux, "", "operating system %s, not a Linux-based distribution (RIG 5.1)", f.OS)
		return
	}
	r.checked(TPlatform, reqLinux, "", "Linux kernel %s", f.Kernel)
	dist := f.Distribution
	if dist == "" {
		dist = "distribution unknown (no /etc/os-release)"
	}
	if f.InContainer {
		dist += " (the image of the D8 container, not of the host)"
	}
	extra := ""
	if f.RebootRequired {
		extra = "; the system asks for a reboot (/var/run/reboot-required)"
	}
	r.reported(TPlatform, reqLinux, "patch level: kernel %s, %s%s; whether it is the current patch level is judged manually (RIG 5.1)", f.Kernel, dist, extra)
}

// daemon judges REQ-PLT-006: a container daemon D8 can connect to.
func (r *runner) daemon(ctx context.Context, p *report.Platform) {
	eps, explicit := r.daemonEndpoints()
	var denied, failed []string
	for _, ep := range eps {
		err := r.c.DialDaemon(ctx, ep)
		switch {
		case err == nil:
			p.Daemon = ep
			r.checked(TPlatform, reqDaemon, "", "container daemon at %s", ep)
			return
		case errors.Is(err, errUnsupported):
			r.inconclusive(TPlatform, []string{reqDaemon}, "", "%s: %v", ep, err)
			return
		case errors.Is(err, fs.ErrPermission):
			denied = append(denied, ep)
		case errors.Is(err, fs.ErrNotExist) && !explicit:
		default:
			failed = append(failed, fmt.Sprintf("%s: %v", ep, err))
		}
	}
	switch {
	case len(denied) > 0:
		p.Daemon = denied[0]
		r.reported(TPlatform, reqDaemon, "a container daemon socket %s exists, but D8 may not connect (permission denied); run D8 as a user of the daemon to judge it (RIG 5.1)", strings.Join(denied, ", "))
	case len(failed) > 0:
		r.violation(TPlatform, reqDaemon, "", "no container daemon answers: %s (RIG 5.1)", strings.Join(failed, "; "))
	default:
		r.violation(TPlatform, reqDaemon, "", "no container daemon found at %s; --daemon names another endpoint (RIG 5.1)", strings.Join(eps, ", "))
	}
}

// daemonEndpoints returns the endpoints to try: --daemon, DOCKER_HOST,
// CONTAINER_HOST, else the default sockets of the system.
func (r *runner) daemonEndpoints() ([]string, bool) {
	if r.c.Daemon != "" {
		return []string{endpoint(r.c.Daemon)}, true
	}
	var out []string
	for _, env := range []string{"DOCKER_HOST", "CONTAINER_HOST"} {
		if v := r.c.Getenv(env); v != "" {
			out = append(out, endpoint(v))
		}
	}
	if len(out) > 0 {
		return out, true
	}
	return r.facts.Daemons, false
}

// endpoint normalizes a socket path or named pipe to a URL.
func endpoint(s string) string {
	switch {
	case strings.Contains(s, "://"):
		return s
	case strings.HasPrefix(s, `\\.\pipe\`):
		return "npipe://" + strings.ReplaceAll(s, `\`, "/")
	}
	return "unix://" + s
}

var errUnsupported = errors.New("endpoint not supported by D8")

// dialDaemon connects to the endpoint of a container daemon and closes the
// connection: unix://, tcp:// and npipe:// (Windows).
func dialDaemon(ctx context.Context, ep string, timeout time.Duration) error {
	u, err := url.Parse(ep)
	if err != nil {
		return err
	}
	if timeout <= 0 || timeout > defaultDaemonTry {
		timeout = defaultDaemonTry
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var d net.Dialer
	var c io.Closer
	switch u.Scheme {
	case "unix":
		p := u.Path
		if p == "" {
			p = u.Opaque
		}
		if _, err := os.Stat(p); err != nil {
			return err
		}
		c, err = d.DialContext(ctx, "unix", p)
	case "tcp":
		c, err = d.DialContext(ctx, "tcp", u.Host)
	case "npipe":
		c, err = openPipe(strings.ReplaceAll(u.Host+u.Path, "/", `\`))
	default:
		return fmt.Errorf("%w: %s", errUnsupported, u.Scheme)
	}
	if err != nil {
		return err
	}
	return c.Close()
}

// messageBus judges REQ-PLT-007 and REQ-PLT-008: D8 connects to the broker
// of broker.json with the login of the application of the root and waits
// for a publication of the OEC Registry.
func (r *runner) messageBus(ctx context.Context, p *report.Platform) {
	reqs := []string{reqBroker, reqRegistry}
	if r.c.Offline {
		r.inconclusive(TPlatform, reqs, "", "--offline: the Message Bus is not contacted")
		return
	}
	c, err := r.connection()
	if err != nil {
		r.inconclusive(TPlatform, reqs, "", "D8 cannot connect like the application of the root: %v", err)
		return
	}
	p.Broker = c.Broker
	c.ClientID, c.ConnectTimeout = mqttwire.ClientID("oec-infra-host-"), r.c.Timeout
	cl, err := mqttwire.Dial(ctx, c)
	if err != nil {
		r.violation(TPlatform, reqBroker, "", "no connection to the broker %s of broker.json with the login of the application: %v (RIG 5.1)", c.Broker, err)
		r.inconclusive(TPlatform, []string{reqRegistry}, "", "no connection to the broker")
		return
	}
	defer cl.Close()
	r.checked(TPlatform, reqBroker, "", "connected to the broker %s", c.Broker)
	found := make(chan string, 1)
	if err := cl.Subscribe(ctx, "Oi4v2/Registry/+/Pub/#", 0, func(m mqttwire.Message) {
		lv := strings.Split(m.Topic, "/")
		if len(lv) < 3 {
			return
		}
		raw, err := base64.RawURLEncoding.DecodeString(lv[2])
		if err != nil {
			return
		}
		select {
		case found <- string(raw):
		default:
		}
	}); err != nil {
		r.inconclusive(TPlatform, []string{reqRegistry}, "", "subscribe Oi4v2/Registry/+/Pub/#: %v", err)
		return
	}
	select {
	case il := <-found:
		p.Registry = il
		r.checked(TPlatform, reqRegistry, "", "OEC Registry %s", il)
	case <-time.After(r.c.RegistryWait):
		r.violation(TPlatform, reqRegistry, "", "no OEC Registry published on Oi4v2/Registry/+/Pub/# within %s (RIG 5.1)", r.c.RegistryWait)
	case <-ctx.Done():
		r.inconclusive(TPlatform, []string{reqRegistry}, "", "interrupted")
	}
}

func jsonDecoder(r io.Reader) *json.Decoder { return json.NewDecoder(r) }

func brokerURL(addr string, port int) string {
	return "mqtts://" + net.JoinHostPort(addr, strconv.Itoa(port))
}
