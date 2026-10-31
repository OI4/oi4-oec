package run

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OI4/oec/conformance"
	"github.com/OI4/oec/conformance/catalog"
	"github.com/OI4/oec/conformance/internal/engine"
	"github.com/OI4/oec/conformance/internal/engine/enginetest"
	"github.com/OI4/oec/conformance/internal/oci/ocitest"
	"github.com/OI4/oec/conformance/internal/run/runtest"
	"github.com/OI4/oec/conformance/internal/run/simapp"
	"github.com/OI4/oec/conformance/internal/verdict"
)

func setup(t *testing.T) ([]verdict.Test, verdict.Requirements) {
	t.Helper()
	cat, err := catalog.Load(catalog.D8File)
	if err != nil {
		t.Fatal(err)
	}
	reqs, err := verdict.LoadRequirements()
	if err != nil {
		t.Fatal(err)
	}
	return Tests(cat), reqs
}

func TestCatalogStatus(t *testing.T) {
	tests, _ := setup(t)
	for _, tc := range tests {
		if (tc.Status == "implemented") != Implemented[tc.ID] {
			t.Errorf("%s: catalog status %s, implemented here %v", tc.ID, tc.Status, Implemented[tc.ID])
		}
		if !equalSet(tc.Reqs, reqsOf[tc.ID]) {
			t.Errorf("%s: catalog requirements %v, judged here %v", tc.ID, tc.Reqs, reqsOf[tc.ID])
		}
	}
	if len(tests) != 3 {
		t.Errorf("%d run tests in the catalog", len(tests))
	}
}

// TestFixtureCoverage: every run test has a positive fixture (it passes)
// and a negative fixture (an expectation worse than pass); every fault of
// the simulated application has a fixture.
func TestFixtureCoverage(t *testing.T) {
	fixtures, err := conformance.RunFixtures()
	if err != nil {
		t.Fatal(err)
	}
	for id := range Implemented {
		pos, neg := false, false
		for _, f := range fixtures {
			e, ok := f.Expect[id]
			pos = pos || f.Positive && f.Runs(id) && (!ok || e.Outcome == string(verdict.Pass))
			neg = neg || ok && e.Outcome != string(verdict.Pass) && e.Outcome != string(verdict.Skip)
		}
		if !pos || !neg {
			t.Errorf("%s: positive fixture %v, negative fixture %v", id, pos, neg)
		}
	}
	for _, fault := range simapp.Faults {
		if !slices.ContainsFunc(fixtures, func(f conformance.Fixture) bool { return f.Fault == string(fault) }) {
			t.Errorf("no fixture for the fault %s", fault)
		}
	}
}

// Short times for the fake: the simulated application connects within
// milliseconds.
func short(c Config) Config {
	c.Wait, c.Settle, c.Poll, c.Grace = 2*time.Second, 300*time.Millisecond, 100*time.Millisecond, time.Second
	return c
}

const otherRef = "oec-d8-other:1"

// scenario builds the configuration and the fake engine of a fixture
// (testdata/run/index.yaml).
func scenario(t *testing.T, f conformance.Fixture) (Config, *enginetest.Server) {
	t.Helper()
	e := runtest.Engine(t)
	c := Config{
		Image: runtest.Ref(simapp.Good), Tests: f.Tests, Timeout: 10 * time.Second, Listen: "127.0.0.1",
		Getenv: func(string) string { return "" }, Defaults: []string{}, TempDir: t.TempDir(),
	}
	daemon := ""
	switch {
	case simapp.Known(simapp.Fault(f.Fault)):
		c.Image = runtest.Ref(simapp.Fault(f.Fault))
	case f.Fault == TBus+"/ip-address":
		c.BrokerHost = "127.0.0.1"
	case f.Fault == TSecret+"/only":
	case f.Fault == "image/other-entrypoint":
		e.Images = append(e.Images, enginetest.Image(ocitest.Digest("sha256", []byte(otherRef)), []string{otherRef}, "linux", "arm64", []string{"/bin/oec-app"}, nil))
		c.Image = otherRef
	case f.Fault == "engine/unreachable":
		daemon = "unix://" + filepath.ToSlash(filepath.Join(t.TempDir(), "missing.sock"))
	case f.Fault == "engine/create-fails":
		e.Fail = []string{"/containers/create"}
	default:
		t.Fatalf("no scenario for fault %q", f.Fault)
	}
	s := enginetest.Start(t, e)
	if daemon == "" {
		daemon = s.Endpoint()
	}
	c.Daemon = daemon
	return short(c), s
}

// left returns the containers of oec-infra run the engine still has.
func left(t *testing.T, s *enginetest.Server) []string {
	t.Helper()
	cl, err := engine.New(s.Endpoint(), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	list, err := cl.Containers(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, c := range list {
		if strings.HasPrefix(c.Name(), engine.OwnPrefix) {
			out = append(out, c.Name())
		}
	}
	return out
}

// TestFixtures runs every run fixture of testdata/run/index.yaml and checks
// the verdict of its selected tests against the expectation, the cleanup
// and the report.
func TestFixtures(t *testing.T) {
	tests, reqs := setup(t)
	fixtures, err := conformance.RunFixtures()
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no run fixtures")
	}
	for _, f := range fixtures {
		t.Run(f.Fault, func(t *testing.T) {
			t.Parallel()
			if f.Status != "draft" && f.Status != "reviewed" {
				t.Errorf("status %q", f.Status)
			}
			for id := range f.Expect {
				if !Implemented[id] || !f.Runs(id) {
					t.Errorf("expectation for %s, which is no implemented run test of the fixture", id)
				}
			}
			c, s := scenario(t, f)
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			res, err := Run(ctx, c)
			if err != nil {
				t.Fatal(err)
			}
			for _, tr := range verdict.Build(tests, res.Observations, reqs, 3) {
				if !f.Runs(tr.ID) || !Implemented[tr.ID] {
					continue
				}
				want, ok := f.Expect[tr.ID]
				if !ok {
					want.Outcome = string(verdict.Pass)
				}
				if string(tr.Outcome) != want.Outcome {
					t.Errorf("%s: outcome %s, want %s (%+v)", tr.ID, tr.Outcome, want.Outcome, tr.Reqs)
					continue
				}
				var with []string
				for _, r := range tr.Reqs {
					switch {
					case string(r.Outcome) == want.Outcome:
						with = append(with, r.Req)
					case r.Outcome != verdict.Skip && !verdict.Worse(tr.Outcome, r.Outcome):
						t.Errorf("%s: %s has %s, not better than the test outcome %s", tr.ID, r.Req, r.Outcome, tr.Outcome)
					}
				}
				if want.Outcome != string(verdict.Pass) && !equalSet(with, want.Reqs) {
					t.Errorf("%s: requirements with %s %v, want %v (%+v)", tr.ID, want.Outcome, with, want.Reqs, tr.Reqs)
				}
				if f.Positive && tr.Outcome != verdict.Pass {
					t.Errorf("%s: a positive fixture must pass (%+v)", tr.ID, tr.Reqs)
				}
			}
			checkRun(t, f, c, res, s)
		})
	}
}

// checkRun: every phase is removed, no container and no storage is left,
// the report has the phases of the selection.
func checkRun(t *testing.T, f conformance.Fixture, c Config, res Result, s *enginetest.Server) {
	t.Helper()
	if l := left(t, s); len(l) != 0 {
		t.Errorf("containers left: %v", l)
	}
	if res.Run.Cleanup != "" {
		t.Errorf("cleanup %q", res.Run.Cleanup)
	}
	if ents, err := os.ReadDir(c.TempDir); err != nil || len(ents) != 0 {
		t.Errorf("storages left: %v %v", ents, err)
	}
	if f.Fault == "engine/unreachable" {
		if len(res.Run.Phases) != 0 {
			t.Errorf("phases %+v", res.Run.Phases)
		}
		return
	}
	want := []string{PhaseCredentials, PhaseCertificate, PhaseUntrusted}
	if !c.Runs(TBus) {
		want = want[:2]
	}
	var got []string
	for _, p := range res.Run.Phases {
		got = append(got, p.Name)
		if !strings.HasPrefix(p.Container, NamePrefix) || !uuidRE.MatchString(strings.TrimPrefix(p.Container, NamePrefix)) || p.Port == 0 {
			t.Errorf("phase %+v", p)
		}
		if f.Fault == "engine/create-fails" {
			if p.Started || p.Removed || !strings.HasPrefix(p.Error, "create:") {
				t.Errorf("phase %+v", p)
			}
			continue
		}
		if !p.Started || !p.Removed || p.Error != "" {
			t.Errorf("phase %+v", p)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("phases %v, want %v", got, want)
	}
	for _, w := range s.Writes() {
		if !strings.HasPrefix(w, "POST /containers/") && !strings.HasPrefix(w, "DELETE /containers/") {
			t.Errorf("write %s", w)
		}
	}
}

var uuidRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func equalSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	sort.Strings(a)
	sort.Strings(b)
	return slices.Equal(a, b)
}

func fixture(t *testing.T, fault string) conformance.Fixture {
	t.Helper()
	fixtures, err := conformance.RunFixtures()
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(fixtures, func(f conformance.Fixture) bool { return f.Fault == fault })
	if i < 0 {
		t.Fatalf("no fixture %s", fault)
	}
	return fixtures[i]
}

// TestForeign: a run touches only its own containers – neither the other
// containers of the engine nor a container of another run, even with the
// marks of oec-infra run.
func TestForeign(t *testing.T) {
	rec := enginetest.Recorded(t)
	other := enginetest.Clone(rec.Stopped)
	enginetest.Set(other, "Id", strings.Repeat("0ddba11", 9)+"0")
	enginetest.Set(other, "Name", "/"+NamePrefix+"0b6a2c1e-4f3d-4c8a-9e21-7d5f3a9b8c40")
	enginetest.Set(other, "Config.Labels", map[string]any{engine.LabelOwner: engine.OwnerRun, engine.LabelRun: "another-run"})
	e := runtest.Engine(t)
	e.Containers = []enginetest.Doc{rec.App, rec.Host, rec.Stopped, rec.Sidecar, other}
	s := enginetest.Start(t, e)
	c := short(Config{Image: runtest.Ref(simapp.Good), Daemon: s.Endpoint(), Listen: "127.0.0.1", Getenv: func(string) string { return "" }, Defaults: []string{}, TempDir: t.TempDir()})
	if _, err := Run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	cl, err := engine.New(s.Endpoint(), 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	list, err := cl.Containers(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, x := range list {
		names = append(names, x.Name())
	}
	want := []string{enginetest.Get(rec.App, "Name").(string), enginetest.Get(rec.Host, "Name").(string), enginetest.Get(rec.Stopped, "Name").(string), enginetest.Get(rec.Sidecar, "Name").(string), NamePrefix + "0b6a2c1e-4f3d-4c8a-9e21-7d5f3a9b8c40"}
	for i := range want {
		want[i] = strings.TrimPrefix(want[i], "/")
	}
	if !equalSet(names, want) {
		t.Errorf("containers %v, want %v", names, want)
	}
	for _, w := range s.Writes() {
		for _, id := range []string{"0ddba11", enginetest.Get(rec.App, "Id").(string)[:12]} {
			if strings.Contains(w, id) {
				t.Errorf("write to a foreign container: %s", w)
			}
		}
	}
}

// TestLeak: the result names no secret, no username and no host path; the
// created containers carry the marks and the storages of RIG 4.1.
func TestLeak(t *testing.T) {
	var (
		mu   sync.Mutex
		seen []enginetest.Started
	)
	e := runtest.Engine(t)
	run := e.Run
	e.Run = func(ctx context.Context, s enginetest.Started) int {
		mu.Lock()
		seen = append(seen, s)
		mu.Unlock()
		return run(ctx, s)
	}
	s := enginetest.Start(t, e)
	c := short(Config{Image: runtest.Ref(simapp.Good), Daemon: s.Endpoint(), Listen: "127.0.0.1", Getenv: func(string) string { return "" }, Defaults: []string{}, TempDir: t.TempDir()})
	res, err := Run(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(res)
	if strings.Contains(string(b), c.TempDir) || strings.Contains(string(b), filepath.ToSlash(c.TempDir)) {
		t.Errorf("the result names the storages")
	}
	if regexp.MustCompile(`oec-d8-[0-9a-f]{8}"|[0-9a-f]{12}:[0-9a-f]{12}|PRIVATE KEY|CERTIFICATE`).Match(b) {
		t.Errorf("the result names a secret: %s", b)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 3 {
		t.Fatalf("%d containers started", len(seen))
	}
	for i, st := range seen {
		if st.Hostname != st.Name || !engine.Owned(st.Name, st.Labels) || st.Labels[engine.LabelRun] == "" || st.NetworkMode != "bridge" {
			t.Errorf("container %d: %+v", i, st)
		}
		if !slices.Equal(st.ExtraHosts, []string{DefaultBrokerHost + ":host-gateway"}) {
			t.Errorf("extra hosts %v", st.ExtraHosts)
		}
		for dst, ro := range map[string]bool{DirMQTT: true, DirCerts: true, DirSecrets: true, DirAppEtc: false, DirAppOpt: false} {
			if src, ok := st.Binds[dst]; !ok || st.ReadOnly[dst] != ro || !strings.HasPrefix(src, c.TempDir) {
				t.Errorf("container %d: bind %s: %q ro %v", i, dst, src, st.ReadOnly[dst])
			}
		}
	}
	if seen[0].Labels[engine.LabelRun] != seen[2].Labels[engine.LabelRun] {
		t.Errorf("the phases have different run labels")
	}
}

// TestUsage: errors of the run.
func TestUsage(t *testing.T) {
	s := enginetest.Start(t, runtest.Engine(t))
	base := short(Config{Daemon: s.Endpoint(), Listen: "127.0.0.1", Getenv: func(string) string { return "" }, Defaults: []string{}, TempDir: t.TempDir()})
	c := base
	c.Image = "oec-d8-sim:no-such-fault"
	if _, err := Run(context.Background(), c); !errors.Is(err, ErrNoImage) || !strings.Contains(err.Error(), "never pulls") {
		t.Errorf("unknown image: %v", err)
	}
	c = base
	c.Image, c.Listen = runtest.Ref(simapp.Good), "192.0.2.1"
	if _, err := Run(context.Background(), c); err == nil || !strings.Contains(err.Error(), "--listen") {
		t.Errorf("listen: %v", err)
	}
	if l := left(t, s); len(l) != 0 {
		t.Errorf("containers left: %v", l)
	}
	// A remote engine is refused before any request.
	for _, rc := range []Config{
		{Daemon: "tcp://192.0.2.1:2375"},
		{Getenv: func(k string) string { return map[string]string{"DOCKER_HOST": "ssh://user@docker.example"}[k] }},
		{Getenv: func(k string) string { return map[string]string{"CONTAINER_HOST": "tcp://192.0.2.1:2375"}[k] }},
	} {
		rc.Image, rc.Defaults, rc.TempDir = runtest.Ref(simapp.Good), []string{}, base.TempDir
		start := time.Now()
		if _, err := Run(context.Background(), rc); !errors.Is(err, ErrRemote) || time.Since(start) > time.Second {
			t.Errorf("%+v: %v after %s", rc, err, time.Since(start))
		}
	}
	c = base
	c.Image, c.Tests = runtest.Ref(simapp.Good), []string{"T8-CTR-*"}
	res, err := Run(context.Background(), c)
	if err != nil || len(res.Observations) != 0 || len(res.Run.Phases) != 0 {
		t.Errorf("no run test selected: %+v %v", res, err)
	}
}

func TestLocal(t *testing.T) {
	for ep, want := range map[string]bool{
		"unix:///var/run/docker.sock":              true,
		"npipe:////./pipe/docker_engine":           true,
		"tcp://127.0.0.1:2375":                     true,
		"tcp://localhost:2375":                     true,
		"tcp://[::1]:2375":                         true,
		"tcp://10.0.0.5:2375":                      false,
		"tcp://docker.example:2376":                false,
		"ssh://user@docker.example":                false,
		"https://docker.example:2376":              false,
		"/var/run/docker.sock":                     true,
		"unix:///run/user/1000/podman/podman.sock": true,
	} {
		if Local(ep) != want {
			t.Errorf("Local(%s) = %v", ep, !want)
		}
	}
}

func TestUUID(t *testing.T) {
	a, b := UUID(), UUID()
	if !uuidRE.MatchString(a) || a == b {
		t.Errorf("UUID %s %s", a, b)
	}
}
