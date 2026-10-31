// Package run implements the sandbox tests of oec-infra run (DESIGN.md §9,
// catalog d8.yaml, mode run): T8-RUN-01 the Message Bus storage, T8-RUN-02
// the certificate and secret storages and T8-RUN-03 the container name. It
// runs the application image in containers of its own through the
// Docker/Podman Engine API (internal/engine), with storages generated for
// the run and an MQTT test endpoint of its own, and removes only what it
// created (A-281 ff.). It never pulls, never creates volumes or networks
// and never reports a secret, a username, a log or a host path.
package run

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/OI4/oec/conformance/catalog"
	"github.com/OI4/oec/conformance/internal/engine"
	"github.com/OI4/oec/conformance/internal/report"
	"github.com/OI4/oec/conformance/internal/topic"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// The tests of the package.
const (
	TBus    = "T8-RUN-01"
	TSecret = "T8-RUN-02"
	TName   = "T8-RUN-03"
)

// Implemented lists the tests this package implements; the catalog status
// must agree (fixtures_test.go).
var Implemented = map[string]bool{TBus: true, TSecret: true, TName: true}

const (
	reqEnv      = "REQ-CMP-029"
	reqBus      = "REQ-CNT-004"
	reqSecrets  = "REQ-CNT-009"
	reqName     = "REQ-CNT-021"
	reqClientID = "REQ-MQTT-013"
)

// reqsOf are the requirements of the tests (catalog d8.yaml).
var reqsOf = map[string][]string{
	TBus:    {reqEnv, reqBus},
	TSecret: {reqSecrets},
	TName:   {reqName, reqClientID},
}

// The phases of a run: one container each.
const (
	PhaseCredentials = "credentials"
	PhaseCertificate = "certificate"
	PhaseUntrusted   = "untrusted"
)

// Defaults of the configuration.
const (
	DefaultTimeout       = engine.DefaultTimeout
	DefaultWait          = 30 * time.Second
	DefaultSettle        = 2 * time.Second
	DefaultPoll          = 500 * time.Millisecond
	DefaultGrace         = 5 * time.Second
	DefaultBrokerHost    = "oec-d8-broker"
	DefaultMaxPacketSize = 262144
	// NamePrefix: a container of the run is NamePrefix<uuid>; the name is
	// its hostname (RIG 4.2).
	NamePrefix = engine.OwnPrefix + "run-"
)

// ErrNoImage: the engine does not have the image (D8 never pulls; a usage
// error, exit code 2).
var ErrNoImage = errors.New("no such image")

// ErrRemote: the Engine API is not on this machine; the bind mounts of the
// generated storages and the test endpoint need a local engine (a usage
// error, exit code 2).
var ErrRemote = errors.New("the Engine API is remote")

// Config of a run.
type Config struct {
	Image   string        // the image reference or id
	Daemon  string        // the Engine API endpoint; "": DOCKER_HOST, CONTAINER_HOST, the defaults
	Tests   []string      // the selected tests (patterns); empty: every test
	Timeout time.Duration // one request; 0: DefaultTimeout
	// Defaults are the endpoints tried without --daemon and environment;
	// nil: engine.Defaults().
	Defaults []string
	// Getenv reads DOCKER_HOST and CONTAINER_HOST; nil: os.Getenv.
	Getenv func(string) string
	Wait   time.Duration // longest observation of a phase; 0: DefaultWait
	Settle time.Duration // observation after the Birth message; 0: DefaultSettle
	Poll   time.Duration // container state poll; 0: DefaultPoll
	Grace  time.Duration // stop: SIGTERM, after Grace SIGKILL; 0: DefaultGrace
	// Listen is the address of the test endpoint; "": 127.0.0.1 on macOS
	// and Windows (Docker Desktop forwards host-gateway there), the
	// gateway of the bridge network elsewhere.
	Listen string
	// BrokerHost is the Address of broker.json; a host name resolves to
	// host-gateway in the container (ExtraHosts), an IP address is used as
	// is. "": DefaultBrokerHost.
	BrokerHost    string
	MaxPacketSize int    // of broker.json; 0: DefaultMaxPacketSize
	TempDir       string // parent of the storages; "": os.TempDir()
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

// Tests returns the run tests of the D8 catalog for verdict.Build.
func Tests(cat *catalog.Catalog) []verdict.Test {
	var out []verdict.Test
	for _, t := range cat.Tests {
		if t.Mode == "run" {
			out = append(out, verdict.Test{ID: t.ID, Title: t.Title, Status: t.Status, Level: t.Level, Reqs: t.Reqs})
		}
	}
	return out
}

// Result of a run.
type Result struct {
	Run          report.Run
	Observations []verdict.Observation
}

// UUID returns a random UUID (version 4).
func UUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b)
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// Local reports whether an Engine API endpoint is on this machine: a Unix
// socket, a named pipe or TCP on a loopback address.
func Local(ep string) bool {
	u, err := url.Parse(engine.Endpoint(ep))
	if err != nil {
		return false
	}
	switch u.Scheme {
	case "unix", "npipe":
		return true
	case "tcp":
		h := u.Hostname()
		if h == "localhost" {
			return true
		}
		ip := net.ParseIP(h)
		return ip != nil && ip.IsLoopback()
	}
	return false
}

type runner struct {
	c     Config
	cl    *engine.Client
	runID string
	pki   *pki
	spec  *topic.Spec
	image string
	tests []string
}

// phaseResult is one phase with what the endpoint saw.
type phaseResult struct {
	rep   report.RunPhase
	conns []conn
	err   bool // the phase could not run
}

// Run executes the selected tests with the image. ErrNoImage, ErrRemote
// and an endpoint that cannot listen are errors of the run; an
// unreachable engine makes the tests inconclusive.
func Run(ctx context.Context, c Config) (Result, error) {
	if c.Timeout == 0 {
		c.Timeout = DefaultTimeout
	}
	if c.Defaults == nil {
		c.Defaults = engine.Defaults()
	}
	if c.Wait == 0 {
		c.Wait = DefaultWait
	}
	if c.Settle == 0 {
		c.Settle = DefaultSettle
	}
	if c.Poll == 0 {
		c.Poll = DefaultPoll
	}
	if c.Grace == 0 {
		c.Grace = DefaultGrace
	}
	if c.BrokerHost == "" {
		c.BrokerHost = DefaultBrokerHost
	}
	if c.MaxPacketSize == 0 {
		c.MaxPacketSize = DefaultMaxPacketSize
	}
	r := &runner{c: c, runID: UUID()}
	for _, t := range []string{TBus, TSecret, TName} {
		if c.Runs(t) {
			r.tests = append(r.tests, t)
		}
	}
	var res Result
	res.Run.Image.Ref = c.Image
	res.Run.Endpoint = report.RunEndpoint{Listen: c.Listen, BrokerHost: c.BrokerHost, MaxPacketSize: c.MaxPacketSize}
	res.Run.Phases = []report.RunPhase{}
	j := &judge{tests: r.tests}
	if len(r.tests) == 0 {
		return res, nil
	}
	// A named remote endpoint (--daemon, DOCKER_HOST, CONTAINER_HOST) is
	// refused before any request: the run never talks to an engine of
	// another machine.
	if eps, named := engine.Endpoints(c.Daemon, c.Getenv, c.Defaults); named {
		for _, ep := range eps {
			if !Local(ep) {
				res.Run.Engine.Endpoint = ep
				return res, fmt.Errorf("%w (%s): oec-infra run needs the engine of this machine (--daemon)", ErrRemote, ep)
			}
		}
	}
	cl, tried, err := engine.Connect(ctx, c.Daemon, c.Getenv, c.Defaults, c.Timeout)
	if err != nil {
		res.Run.Engine.Endpoint = strings.Join(tried, ", ")
		if res.Run.Endpoint.Listen == "" {
			res.Run.Endpoint.Listen = "-"
		}
		j.inconclusive("no Docker/Podman Engine API (%s): %v", res.Run.Engine.Endpoint, err)
		res.Observations = j.obs
		return res, nil
	}
	r.cl = cl
	res.Run.Engine = report.Engine{Endpoint: cl.Endpoint()}
	if v, err := cl.Version(ctx); err == nil {
		res.Run.Engine.Product, res.Run.Engine.Version, res.Run.Engine.API = v.Product(), v.Version, v.APIVersion
		res.Run.Engine.OS, res.Run.Engine.Arch = v.Os, v.Arch
	}
	if !Local(cl.Endpoint()) {
		return res, fmt.Errorf("%w (%s): oec-infra run needs the engine of this machine", ErrRemote, cl.Endpoint())
	}
	img, err := cl.Image(ctx, c.Image)
	switch {
	case errors.Is(err, engine.ErrNotFound):
		return res, fmt.Errorf("%w: %s (engine %s; D8 never pulls: pull or load it first)", ErrNoImage, c.Image, cl.Endpoint())
	case err != nil:
		return res, fmt.Errorf("image %s: %w", c.Image, err)
	}
	r.image = img.ID
	res.Run.Image.ID, res.Run.Image.OS, res.Run.Image.Arch = img.ID, img.Os, img.Architecture
	if c.Listen == "" {
		if c.Listen, err = r.listenAddr(ctx); err != nil {
			return res, err
		}
		res.Run.Endpoint.Listen = c.Listen
		r.c.Listen = c.Listen
	}
	if r.pki, err = newPKI(); err != nil {
		return res, err
	}
	if r.spec, err = topic.Load(); err != nil {
		return res, err
	}
	phases := []string{PhaseCredentials, PhaseCertificate}
	if c.Runs(TBus) {
		phases = append(phases, PhaseUntrusted)
	}
	var results []phaseResult
	for _, p := range phases {
		pr, err := r.phase(ctx, p)
		if err != nil {
			res.Run.Cleanup = r.cleanup(ctx)
			return res, err
		}
		results = append(results, pr)
		res.Run.Phases = append(res.Run.Phases, pr.rep)
		if ctx.Err() != nil {
			break
		}
	}
	res.Run.Cleanup = r.cleanup(ctx)
	if ctx.Err() != nil {
		j.inconclusive("the run was interrupted: %v", ctx.Err())
	} else {
		j.judge(results, c.Runs(TBus))
	}
	res.Observations = j.obs
	return res, nil
}

// listenAddr is the default address of the test endpoint.
func (r *runner) listenAddr(ctx context.Context) (string, error) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return "127.0.0.1", nil
	}
	gw, err := r.cl.Gateway(ctx, "bridge")
	if err != nil {
		return "", fmt.Errorf("the address of the test endpoint (--listen): %w", err)
	}
	return gw, nil
}

// cleanup removes every container of the run that is left (a phase
// interrupted between create and remove) and reports what failed.
func (r *runner) cleanup(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*r.c.Timeout+r.c.Grace)
	defer cancel()
	left, err := r.cl.Own(ctx, r.runID)
	if err != nil {
		return "listing the containers of the run: " + err.Error()
	}
	var probs []string
	for _, s := range left {
		if err := r.cl.Remove(ctx, s.ID); err != nil && !errors.Is(err, engine.ErrNotFound) {
			probs = append(probs, s.Name()+": "+err.Error())
		}
	}
	return strings.Join(probs, "; ")
}

// phase runs one container. An error ends the run (the endpoint cannot
// listen, the storages cannot be written).
func (r *runner) phase(ctx context.Context, kind string) (pr phaseResult, err error) {
	name := NamePrefix + UUID()
	pr = phaseResult{rep: report.RunPhase{Name: kind, Container: name, Connections: []report.RunConnection{}}}
	host := r.c.BrokerHost
	st := storages{address: host, maxPacketSize: r.c.MaxPacketSize, caPEM: r.pki.a.pem, name: name}
	var user string
	var pass []byte
	switch kind {
	case PhaseCredentials, PhaseUntrusted:
		st.unknown = true
		user = engine.OwnPrefix + randHex(4)
		pass = []byte(randHex(6) + ":" + randHex(6))
		st.username, st.password = user, pass
	case PhaseCertificate:
		st.brokerCAPEM = r.pki.b.pem
		st.passphrase = []byte(randHex(12))
		if st.certPEM, st.keyPEM, err = r.pki.a.client(name, st.passphrase); err != nil {
			return pr, err
		}
	}
	// The endpoint presents a certificate of the CA the storages name
	// (broker_ca.pem or ca.pem) – or, untrusted, of a CA the container
	// does not know; broker.pem is the certificate of the right CA.
	presented, stored := r.pki.a, r.pki.a
	switch kind {
	case PhaseCertificate:
		presented, stored = r.pki.b, r.pki.b
	case PhaseUntrusted:
		presented = r.pki.x
	}
	pl, tlsCert, err := presented.broker(host)
	if err != nil {
		return pr, err
	}
	st.brokerPEM = pl.certPEM
	if stored != presented {
		sl, _, err := stored.broker(host)
		if err != nil {
			return pr, err
		}
		st.brokerPEM = sl.certPEM
	}
	ep, err := listen(r.c.Listen, tlsCert, r.pki.a.pool(), user, pass, r.c.MaxPacketSize, r.spec)
	if err != nil {
		return pr, fmt.Errorf("the test endpoint cannot listen on %s (--listen): %w", r.c.Listen, err)
	}
	defer ep.Close()
	st.port = ep.Port()
	pr.rep.Port = st.port
	dir, err := os.MkdirTemp(r.c.TempDir, "oec-d8-run-")
	if err != nil {
		return pr, err
	}
	defer os.RemoveAll(dir)
	binds, err := st.write(dir)
	if err != nil {
		return pr, err
	}
	spec := engine.CreateSpec{
		Name: name, Image: r.image, Hostname: name, Binds: binds, NetworkMode: "bridge",
		Labels: map[string]string{engine.LabelOwner: engine.OwnerRun, engine.LabelRun: r.runID},
	}
	if net.ParseIP(host) == nil {
		spec.ExtraHosts = []string{host + ":host-gateway"}
	}
	id, err := r.cl.Create(ctx, spec)
	if err != nil {
		pr.rep.Error, pr.err = "create: "+err.Error(), true
		return pr, nil
	}
	start := time.Now()
	defer func() {
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*r.c.Timeout+r.c.Grace)
		defer cancel()
		if err := r.cl.Stop(cctx, id, r.c.Grace); err != nil && !errors.Is(err, engine.ErrNotFound) && pr.rep.Error == "" {
			pr.rep.Error = "stop: " + err.Error()
		}
		if err := r.cl.Remove(cctx, id); err == nil {
			pr.rep.Removed = true
		}
	}()
	if err := r.cl.Start(ctx, id); err != nil {
		pr.rep.Error, pr.err = "start: "+err.Error(), true
		return r.finish(ctx, pr, ep, id, start), nil
	}
	pr.rep.Started = true
	r.observe(ctx, ep, id)
	return r.finish(ctx, pr, ep, id, start), nil
}

// observe waits until the phase has an outcome: the Birth message and the
// settle time, a failed handshake, a refused CONNECT, the end of the
// container or the end of the wait.
func (r *runner) observe(ctx context.Context, ep *endpoint, id string) {
	deadline := time.NewTimer(r.c.Wait)
	defer deadline.Stop()
	poll := time.NewTicker(r.c.Poll)
	defer poll.Stop()
	var settle <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline.C:
			return
		case <-settle:
			return
		case <-poll.C:
			ctr, err := r.cl.Inspect(ctx, id)
			if err == nil && !ctr.State.Running && ctr.State.Status != "created" {
				return
			}
		case <-ep.Changed():
		}
		for _, c := range ep.Snapshot() {
			if c.Handshake == "failed" || c.Auth != "" && !c.Accepted {
				return
			}
			if c.birth != nil && settle == nil {
				settle = time.After(r.c.Settle)
			}
		}
	}
}

// finish records the state of the container and the connections.
func (r *runner) finish(ctx context.Context, pr phaseResult, ep *endpoint, id string, start time.Time) phaseResult {
	ictx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.c.Timeout)
	defer cancel()
	if ctr, err := r.cl.Inspect(ictx, id); err == nil {
		pr.rep.Running = ctr.State.Running
		if !ctr.State.Running && pr.rep.Started {
			code := ctr.State.ExitCode
			pr.rep.ExitCode = &code
		}
	}
	pr.rep.ObservedMS = time.Since(start).Milliseconds()
	pr.conns = ep.Snapshot()
	for _, c := range pr.conns {
		pr.rep.Connections = append(pr.rep.Connections, c.RunConnection)
		if pr.rep.Birth == nil && c.birth != nil {
			b := *c.birth
			pr.rep.Birth = &b
		}
	}
	return pr
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
