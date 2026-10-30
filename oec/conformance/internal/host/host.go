// Package host implements the host tests of oec-infra host (DESIGN.md §9,
// catalog d8.yaml, mode host): T8-HST-01 the Message Bus storage, T8-HST-02
// the OI4 certificate and secret storages and T8-HST-04 the platform host.
// It reads the storages of RIG 4.1 below a root directory – the file system
// of a container (default /) or a directory with the same layout – and the
// measured properties of the system it runs on; it never writes. The
// platform tests connect to the container daemon and – like the application
// of the root – to the Message Bus.
package host

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/OI4/oec/conformance/catalog"
	"github.com/OI4/oec/conformance/internal/report"
	"github.com/OI4/oec/conformance/internal/schemas"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// The tests of the package. T8-HST-03 (application storages are not
// shared) needs the Engine API and comes with P5b increment 4.
const (
	TMQTT     = "T8-HST-01"
	TSecrets  = "T8-HST-02"
	TPlatform = "T8-HST-04"
)

// Implemented lists the tests this package implements; the catalog status
// must agree (fixtures_test.go).
var Implemented = map[string]bool{TMQTT: true, TSecrets: true, TPlatform: true}

// Defaults.
const (
	DefaultTimeout      = 5 * time.Second  // one connection
	DefaultRegistryWait = 65 * time.Second // a heartbeat of the Registry (60 s, MG 7.5.1, SF-106) and some slack
)

// The storages of RIG 4.1 below the root.
const (
	PathMQTT    = "etc/oi4/mqtt"    // Message Bus storage (RIG 4.1.1)
	PathCerts   = "etc/oi4/certs"   // OI4 certificate storage (RIG 4.1.2)
	PathSecrets = "run/secrets/oi4" // secret storage (RIG 4.1.3)
)

// Config of a run.
type Config struct {
	Root      string // the root directory; "": /
	MQTT      string // the Message Bus storage; "": Root/etc/oi4/mqtt
	Certs     string // the OI4 certificate storage; "": Root/etc/oi4/certs
	Secrets   string // the secret storage; "": Root/run/secrets/oi4
	Container string // the container name (RIG 4.2); "": Root/etc/hostname
	Daemon    string // the endpoint of the container daemon; "": DOCKER_HOST, CONTAINER_HOST, the default sockets
	Offline   bool   // the Message Bus is not contacted (REQ-PLT-007/008 inconclusive)

	RegistryWait time.Duration // discovery of the OEC Registry; 0: DefaultRegistryWait
	Timeout      time.Duration // one connection; 0: DefaultTimeout
	Tests        []string      // the selected tests (patterns); empty: every test

	// Facts are the properties of the platform host; nil: measured on
	// this system (Local).
	Facts *Facts
	// Mounts returns the mount table; nil: the one of this system
	// (Linux: /proc/self/mountinfo). ErrNoMountTable: none on this system.
	Mounts func() ([]Mount, error)
	// DialDaemon connects to the endpoint of a container daemon; nil: a
	// connection to the socket, named pipe or TCP port.
	DialDaemon func(ctx context.Context, endpoint string) error
	// Getenv reads DOCKER_HOST and CONTAINER_HOST; nil: os.Getenv.
	Getenv func(string) string
}

// Runs reports whether the configuration selects the test.
func (c Config) Runs(id string) bool {
	if len(c.Tests) == 0 {
		return true
	}
	for _, p := range c.Tests {
		if ok, _ := path.Match(p, id); ok {
			return true
		}
	}
	return false
}

// Tests returns the host tests of the D8 catalog for verdict.Build.
func Tests(cat *catalog.Catalog) []verdict.Test {
	var out []verdict.Test
	for _, t := range cat.Tests {
		if t.Mode == "host" {
			out = append(out, verdict.Test{ID: t.ID, Title: t.Title, Status: t.Status, Level: t.Level, Reqs: t.Reqs})
		}
	}
	return out
}

// Result of a run.
type Result struct {
	Host         report.Host
	Observations []verdict.Observation
}

const (
	reqMQTTStorage = "REQ-CNT-001"
	reqNoKeys      = "REQ-CNT-002"
	reqBrokerJSON  = "REQ-CNT-003"
	reqCA          = "REQ-CNT-005"
	reqCertStorage = "REQ-CNT-006"
	reqAppCert     = "REQ-CNT-007"
	reqSecrets     = "REQ-CNT-008"
	reqAddress     = "REQ-MQTT-003"
	reqUsername    = "REQ-MQTT-011"
	reqHardware    = "REQ-PLT-001"
	reqLinux       = "REQ-PLT-003"
	reqDaemon      = "REQ-PLT-006"
	reqBroker      = "REQ-PLT-007"
	reqRegistry    = "REQ-PLT-008"
)

type runner struct {
	c                    Config
	facts                *Facts
	schemas              *schemas.Set
	mounts               []Mount
	mountsKnown          bool
	mqtt, certs, secrets *store
	obs                  []verdict.Observation
	info                 report.Host
}

func (r *runner) add(test, req string, k verdict.Kind, file, format string, a ...any) {
	r.obs = append(r.obs, verdict.Observation{Test: test, Req: req, Kind: k, Evidence: verdict.Evidence{Path: file, Detail: fmt.Sprintf(format, a...)}})
}

func (r *runner) checked(test, req, file, format string, a ...any) {
	r.add(test, req, verdict.Checked, file, format, a...)
}

func (r *runner) violation(test, req, file, format string, a ...any) {
	r.add(test, req, verdict.Violation, file, format, a...)
}

func (r *runner) note(test, req, file, format string, a ...any) {
	r.add(test, req, verdict.Note, file, format, a...)
}

func (r *runner) reported(test, req, format string, a ...any) {
	r.add(test, req, verdict.Reported, "", format, a...)
}

// inconclusive marks requirements the run could not judge.
func (r *runner) inconclusive(test string, reqs []string, file, format string, a ...any) {
	for _, req := range reqs {
		r.obs = append(r.obs, verdict.Observation{Test: test, Req: req, Kind: verdict.Skipped, Inconclusive: true, Evidence: verdict.Evidence{Path: file, Detail: fmt.Sprintf(format, a...)}})
	}
}

// Run executes the selected tests.
func Run(ctx context.Context, c Config) Result {
	if c.Root == "" {
		c.Root = string(filepath.Separator)
	}
	if c.RegistryWait == 0 {
		c.RegistryWait = DefaultRegistryWait
	}
	if c.Timeout == 0 {
		c.Timeout = DefaultTimeout
	}
	if c.Getenv == nil {
		c.Getenv = os.Getenv
	}
	if c.DialDaemon == nil {
		timeout := c.Timeout
		c.DialDaemon = func(ctx context.Context, ep string) error { return dialDaemon(ctx, ep, timeout) }
	}
	if c.Container == "" {
		if b, err := os.ReadFile(filepath.Join(c.Root, "etc", "hostname")); err == nil {
			c.Container = strings.TrimSpace(string(b))
		}
	}
	in := func(over, rel string) string {
		if over != "" {
			return over
		}
		return filepath.Join(c.Root, filepath.FromSlash(rel))
	}
	r := &runner{c: c, facts: c.Facts}
	if r.facts == nil {
		r.facts = Local()
	}
	mounts := c.Mounts
	if mounts == nil {
		mounts = mountTable
	}
	var err error
	r.mounts, err = mounts()
	r.mountsKnown = err == nil
	r.info.Root, r.info.Container = c.Root, c.Container
	r.mqtt = r.scan("mqtt", in(c.MQTT, PathMQTT))
	r.certs = r.scan("certs", in(c.Certs, PathCerts))
	r.secrets = r.scan("secrets", in(c.Secrets, PathSecrets))
	if c.Runs(TMQTT) {
		r.messageBusStorage()
	}
	if c.Runs(TSecrets) {
		r.certificateStorages()
	}
	if c.Runs(TPlatform) {
		r.platform(ctx)
	}
	r.info.Storages = []report.Storage{r.mqtt.rep, r.certs.rep, r.secrets.rep}
	return Result{Host: r.info, Observations: r.obs}
}

// loadSchemas loads the schemas of oec/schema once.
func (r *runner) loadSchemas() (*schemas.Set, error) {
	if r.schemas != nil {
		return r.schemas, nil
	}
	s, err := schemas.Load()
	if err != nil {
		return nil, err
	}
	r.schemas = s
	return s, nil
}
