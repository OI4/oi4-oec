// Package container implements the container tests of oec-infra container
// (DESIGN.md §9, catalog d8.yaml, mode container): T8-CTR-01 the container
// networks, T8-CTR-02 the container name and hostname and T8-CTR-03 the
// license folder that the platform must not overlay. It reads the inspect
// documents of running or created containers through the Docker/Podman
// Engine API (internal/engine); it never writes and never reports the
// environment, labels or mount sources of a container.
package container

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/OI4/oec/conformance/catalog"
	"github.com/OI4/oec/conformance/internal/engine"
	"github.com/OI4/oec/conformance/internal/report"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// The tests of the package.
const (
	TNetwork  = "T8-CTR-01"
	TName     = "T8-CTR-02"
	TLicenses = "T8-CTR-03"
)

// Implemented lists the tests this package implements; the catalog status
// must agree (fixtures_test.go).
var Implemented = map[string]bool{TNetwork: true, TName: true, TLicenses: true}

// DefaultTimeout is the longest time of one request to the Engine API.
const DefaultTimeout = engine.DefaultTimeout

// PathLicenses is the license folder of RIG 4.1.4 in the container.
const PathLicenses = "/opt/oi4/licenses"

const (
	reqNetworks = "REQ-CMP-031"
	reqBridge   = "REQ-CNT-023"
	reqName     = "REQ-CNT-019"
	reqUUID     = "REQ-CNT-020"
	reqLicenses = "REQ-CNT-017"
)

// reqsOf are the requirements of the tests (catalog d8.yaml).
var reqsOf = map[string][]string{
	TNetwork:  {reqNetworks, reqBridge},
	TName:     {reqName, reqUUID},
	TLicenses: {reqLicenses},
}

// ErrNoContainer: the engine does not know a named container (a usage
// error of the run, exit code 2).
var ErrNoContainer = errors.New("no such container")

// Config of a run.
type Config struct {
	Containers []string      // names, ids or unique id prefixes
	Daemon     string        // the Engine API endpoint; "": DOCKER_HOST, CONTAINER_HOST, the defaults
	Tests      []string      // the selected tests (patterns); empty: every test
	Timeout    time.Duration // one request; 0: DefaultTimeout
	// Defaults are the endpoints tried without --daemon and environment;
	// nil: engine.Defaults().
	Defaults []string
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

// Tests returns the container tests of the D8 catalog for verdict.Build.
func Tests(cat *catalog.Catalog) []verdict.Test {
	var out []verdict.Test
	for _, t := range cat.Tests {
		if t.Mode == "container" {
			out = append(out, verdict.Test{ID: t.ID, Title: t.Title, Status: t.Status, Level: t.Level, Reqs: t.Reqs})
		}
	}
	return out
}

// Result of a run.
type Result struct {
	Run          report.ContainerRun
	Observations []verdict.Observation
}

type runner struct {
	c        Config
	cl       *engine.Client
	obs      []verdict.Observation
	networks map[string]*network // by id or name
	list     []engine.Summary
	listErr  error
	listed   bool
}

type network struct {
	n   *engine.Network
	err error
}

func (r *runner) add(test, req string, k verdict.Kind, ctr, p, format string, a ...any) {
	r.obs = append(r.obs, verdict.Observation{Test: test, Req: req, Kind: k, Evidence: verdict.Evidence{Container: ctr, Path: p, Detail: fmt.Sprintf(format, a...)}})
}

// inconclusive marks the requirements of the selected tests the run could
// not judge.
func (r *runner) inconclusive(tests []string, ctr, format string, a ...any) {
	for _, t := range tests {
		for _, req := range reqsOf[t] {
			r.obs = append(r.obs, verdict.Observation{Test: t, Req: req, Kind: verdict.Skipped, Inconclusive: true, Evidence: verdict.Evidence{Container: ctr, Detail: fmt.Sprintf(format, a...)}})
		}
	}
}

// Run executes the selected tests on every container of the
// configuration. ErrNoContainer: the engine does not know a container.
func Run(ctx context.Context, c Config) (Result, error) {
	if c.Timeout == 0 {
		c.Timeout = DefaultTimeout
	}
	if c.Defaults == nil {
		c.Defaults = engine.Defaults()
	}
	r := &runner{c: c, networks: map[string]*network{}}
	var tests []string
	for _, t := range []string{TNetwork, TName, TLicenses} {
		if c.Runs(t) {
			tests = append(tests, t)
		}
	}
	var res Result
	res.Run.Containers = []report.Container{}
	cl, tried, err := engine.Connect(ctx, c.Daemon, c.Getenv, c.Defaults, c.Timeout)
	if err != nil {
		res.Run.Engine.Endpoint = strings.Join(tried, ", ")
		r.inconclusive(tests, "", "no Docker/Podman Engine API (%s): %v", res.Run.Engine.Endpoint, err)
		res.Observations = r.obs
		return res, nil
	}
	r.cl = cl
	res.Run.Engine = report.Engine{Endpoint: cl.Endpoint()}
	if v, err := cl.Version(ctx); err == nil {
		res.Run.Engine.Product, res.Run.Engine.Version, res.Run.Engine.API = v.Product(), v.Version, v.APIVersion
		res.Run.Engine.OS, res.Run.Engine.Arch = v.Os, v.Arch
	}
	for _, name := range c.Containers {
		ctr, err := cl.Inspect(ctx, name)
		switch {
		case errors.Is(err, engine.ErrNotFound):
			return res, fmt.Errorf("%w: %s (engine %s)", ErrNoContainer, name, cl.Endpoint())
		case err != nil:
			r.inconclusive(tests, name, "inspect: %v", err)
			continue
		}
		rep := r.describe(ctx, ctr)
		res.Run.Containers = append(res.Run.Containers, rep)
		if c.Runs(TNetwork) {
			r.networkTest(ctx, ctr)
		}
		if c.Runs(TName) {
			r.nameTest(ctx, ctr)
		}
		if c.Runs(TLicenses) {
			r.licenses(ctr)
		}
	}
	res.Observations = r.obs
	return res, nil
}

// Describe returns the report entry of an inspected container (no
// sources, no environment, no labels); networks without a driver.
func Describe(c *engine.Container) report.Container {
	out := report.Container{
		ID: c.ShortID(), Name: c.ContainerName(), Hostname: c.Config.Hostname, Image: c.Config.Image,
		Running: c.State.Running, NetworkMode: c.HostConfig.NetworkMode, Mounts: []report.ContainerMount{},
	}
	for _, n := range networkNames(c) {
		out.Networks = append(out.Networks, report.Network{Name: n})
	}
	for _, m := range c.AllMounts() {
		mode := "ro"
		if m.RW {
			mode = "rw"
		}
		out.Mounts = append(out.Mounts, report.ContainerMount{Destination: m.Destination, Type: m.Type, Mode: mode})
	}
	return out
}

func (r *runner) describe(ctx context.Context, c *engine.Container) report.Container {
	out := Describe(c)
	for i, n := range networkNames(c) {
		if nw := r.network(ctx, c, n); nw.err == nil {
			out.Networks[i].Driver = nw.n.Driver
		}
	}
	return out
}

func networkNames(c *engine.Container) []string {
	var out []string
	for n := range c.NetworkSettings.Networks {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// network looks up an attached network by its id (by its name if the
// engine reports no id).
func (r *runner) network(ctx context.Context, c *engine.Container, name string) *network {
	key := c.NetworkSettings.Networks[name].NetworkID
	if key == "" {
		key = name
	}
	if nw, ok := r.networks[key]; ok {
		return nw
	}
	n, err := r.cl.Network(ctx, key)
	nw := &network{n: n, err: err}
	r.networks[key] = nw
	return nw
}

// --- T8-CTR-01 networks -----------------------------------------------------

func (r *runner) networkTest(ctx context.Context, c *engine.Container) {
	name, via := c.ContainerName(), ""
	mode := c.HostConfig.NetworkMode
	if id, ok := strings.CutPrefix(mode, "container:"); ok {
		t, err := r.cl.Inspect(ctx, id)
		if err != nil {
			r.inconclusive([]string{TNetwork}, name, "network mode %s: the container is not readable: %v", mode, err)
			return
		}
		via = fmt.Sprintf(" (the network of container %s, network mode %s)", t.ContainerName(), mode)
		if strings.HasPrefix(t.HostConfig.NetworkMode, "container:") {
			r.both(verdict.Note, name, "network mode %s, which uses network mode %s: D8 does not follow a chain", mode, t.HostConfig.NetworkMode)
			return
		}
		c, mode = t, t.HostConfig.NetworkMode
	}
	switch mode {
	case "host":
		r.hostNetwork(name, via)
		return
	case "none":
		r.both(verdict.Note, name, "no network (network mode none)%s: neither a bridge nor the host network", via)
		return
	case "pasta", "slirp4netns":
		r.both(verdict.Note, name, "user-mode network %s%s: neither a bridge nor the host network", mode, via)
		return
	}
	nets := networkNames(c)
	if len(nets) == 0 {
		r.both(verdict.Note, name, "network mode %s%s: no network attached", mode, via)
		return
	}
	for _, n := range nets {
		nw := r.network(ctx, c, n)
		switch {
		case nw.err != nil:
			r.inconclusive([]string{TNetwork}, name, "network %s%s: the driver is not readable: %v", n, via, nw.err)
		case nw.n.Driver == "bridge":
			r.both(verdict.Checked, name, "network %s (driver bridge)%s", n, via)
		case nw.n.Driver == "host":
			r.hostNetwork(name, via)
		case nw.n.Driver == "null":
			r.both(verdict.Note, name, "network %s (driver null)%s: no network", n, via)
		default:
			r.both(verdict.Note, name, "network %s (driver %s)%s: RIG 4.4 compares only bridge and host networks", n, nw.n.Driver, via)
		}
	}
}

func (r *runner) both(k verdict.Kind, ctr, format string, a ...any) {
	r.add(TNetwork, reqNetworks, k, ctr, "", format, a...)
	r.add(TNetwork, reqBridge, k, ctr, "", format, a...)
}

func (r *runner) hostNetwork(ctr, via string) {
	const d = "host network%s: RIG 4.4 prefers a bridge network unless the application needs low-level Ethernet communication (judge the exception manually)"
	r.add(TNetwork, reqNetworks, verdict.Warning, ctr, "", d, via)
	r.add(TNetwork, reqBridge, verdict.Violation, ctr, "", d, via)
}

// --- T8-CTR-02 name and hostname --------------------------------------------

var (
	hostLabel = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?$`)
	uuidName  = regexp.MustCompile(`^(.+)-[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$`)
)

// MaxName is the longest container name (a host name label, RFC 1123).
const MaxName = 63

// MaxDefaultName is the longest default container name before -<uuid>.
const MaxDefaultName = 26

func (r *runner) nameTest(ctx context.Context, c *engine.Container) {
	name := c.ContainerName()
	ok := true
	bad := func(format string, a ...any) {
		ok = false
		r.add(TName, reqName, verdict.Violation, name, "", format, a...)
	}
	if len(name) > MaxName {
		bad("the container name has %d characters, more than %d", len(name), MaxName)
	}
	if !hostLabel.MatchString(name) {
		bad("the container name %q does not follow the host name rules of RFC 1123 (letters, digits and hyphens, not at the start or end)", name)
	}
	if c.Config.Hostname != name {
		bad("the hostname %q is not the container name %q", c.Config.Hostname, name)
	}
	if !r.listed {
		r.list, r.listErr = r.cl.Containers(ctx, true)
		r.listed = true
	}
	if r.listErr != nil {
		r.inconclusive([]string{TName}, name, "the containers of the engine are not readable: %v", r.listErr)
		ok = false
	} else {
		for _, s := range r.list {
			if s.ID != c.ID && strings.EqualFold(s.Name(), name) {
				bad("container %s has the same name as a host name (host names are case-insensitive)", s.Name())
			}
		}
	}
	if ok {
		r.add(TName, reqName, verdict.Checked, name, "", "the container name is a valid host name, unique within the engine and set as hostname (other engines of the platform are not seen)")
	}
	m := uuidName.FindStringSubmatch(name)
	switch {
	case m == nil:
		r.add(TName, reqUUID, verdict.Violation, name, "", "the container name %q has no -<uuid> postfix", name)
	case len(m[1]) > MaxDefaultName:
		r.add(TName, reqUUID, verdict.Violation, name, "", "the default container name %q has %d characters, more than %d", m[1], len(m[1]), MaxDefaultName)
	default:
		r.add(TName, reqUUID, verdict.Checked, name, "", "<DefaultContainerName>-<uuid>")
	}
}

// --- T8-CTR-03 /opt/oi4/licenses --------------------------------------------

var anonymous = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (r *runner) licenses(c *engine.Container) {
	name := c.ContainerName()
	var judged []engine.Mount
	var cover *engine.Mount
	for _, m := range c.AllMounts() {
		switch dst := path.Clean(m.Destination); {
		case engine.Within(PathLicenses, dst):
			if cover == nil || len(dst) > len(path.Clean(cover.Destination)) {
				m := m
				cover = &m
			}
		case engine.Within(dst, PathLicenses):
			judged = append(judged, m)
		}
	}
	if cover != nil {
		judged = append([]engine.Mount{*cover}, judged...)
	}
	if len(judged) == 0 {
		r.add(TLicenses, reqLicenses, verdict.Checked, name, PathLicenses, "no mount on %s: the content comes from the image", PathLicenses)
		return
	}
	for _, m := range judged {
		where := m.Destination
		switch {
		case path.Clean(where) == PathLicenses:
		case engine.Within(PathLicenses, where):
			where += " (covers " + PathLicenses + ")"
		default:
			where += " (below " + PathLicenses + ")"
		}
		switch m.Type {
		case "bind":
			r.add(TLicenses, reqLicenses, verdict.Violation, name, m.Destination, "a bind mount of the host on %s", where)
		case "tmpfs":
			r.add(TLicenses, reqLicenses, verdict.Violation, name, m.Destination, "a tmpfs on %s: the content does not come from the image", where)
		case "volume":
			vol := m.Name
			if anonymous.MatchString(vol) {
				vol = "(anonymous " + vol[:12] + ")"
			}
			r.add(TLicenses, reqLicenses, verdict.Reported, name, m.Destination, "volume %s on %s: allowed if it is populated from the image (REQ-CNT-018), judge manually", vol, where)
		case "image":
			r.add(TLicenses, reqLicenses, verdict.Note, name, m.Destination, "an image mount on %s: content of an image, not of the host", where)
		default:
			r.add(TLicenses, reqLicenses, verdict.Reported, name, m.Destination, "a mount of type %s on %s: judge manually", m.Type, where)
		}
	}
}
