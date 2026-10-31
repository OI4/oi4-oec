package container

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/OI4/oec/conformance"
	"github.com/OI4/oec/conformance/catalog"
	"github.com/OI4/oec/conformance/internal/engine/enginetest"
	"github.com/OI4/oec/conformance/internal/verdict"
)

const uuid = "0b6a2c1e-4f3d-4c8a-9e21-7d5f3a9b8c40"

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
		want := reqsOf[tc.ID]
		if !equalSet(tc.Reqs, want) {
			t.Errorf("%s: catalog requirements %v, judged here %v", tc.ID, tc.Reqs, want)
		}
	}
	if len(tests) != 3 {
		t.Errorf("%d container tests in the catalog", len(tests))
	}
}

// TestFixtureCoverage: every container test has a positive fixture (it
// passes) and a negative fixture (an expectation worse than pass).
func TestFixtureCoverage(t *testing.T) {
	fixtures, err := conformance.ContainerFixtures()
	if err != nil {
		t.Fatal(err)
	}
	for id := range Implemented {
		pos, neg := false, false
		for _, f := range fixtures {
			e, ok := f.Expect[id]
			pos = pos || f.Positive && f.Runs(id) && (!ok || e.Outcome == string(verdict.Pass))
			neg = neg || ok && e.Outcome != string(verdict.Pass)
		}
		if !pos || !neg {
			t.Errorf("%s: positive fixture %v, negative fixture %v", id, pos, neg)
		}
	}
}

func name(d enginetest.Doc) string {
	return strings.TrimPrefix(enginetest.Get(d, "Name").(string), "/")
}

func rename(d enginetest.Doc, n string) {
	enginetest.Set(d, "Name", "/"+n)
	enginetest.Set(d, "Config.Hostname", n)
}

func networks(d enginetest.Doc, mode string, nets map[string]string) {
	enginetest.Set(d, "HostConfig.NetworkMode", mode)
	m := map[string]any{}
	for n, id := range nets {
		m[n] = map[string]any{"NetworkID": id}
	}
	enginetest.Set(d, "NetworkSettings.Networks", m)
}

func id(d enginetest.Doc) string { return enginetest.Get(d, "Id").(string) }

// podman is the version document of a Podman 5 service (synthetic, A-271).
func podman() enginetest.Doc {
	return enginetest.Doc{
		"Platform":      map[string]any{"Name": "linux/amd64/fedora-41"},
		"Components":    []any{map[string]any{"Name": "Podman Engine", "Version": "5.4.2", "Details": map[string]any{"APIVersion": "5.4.2", "MinAPIVersion": "4.0.0", "Os": "linux", "Arch": "amd64"}}},
		"Version":       "5.4.2",
		"ApiVersion":    "1.41",
		"MinAPIVersion": "1.24",
		"Os":            "linux",
		"Arch":          "amd64",
	}
}

// scenario builds the configuration and the fake engine of a fixture from
// the recorded documents (testdata/containers/index.yaml).
func scenario(t *testing.T, f conformance.Fixture) Config {
	t.Helper()
	rec := enginetest.Recorded(t)
	app, host, stopped, sidecar := rec.App, rec.Host, rec.Stopped, rec.Sidecar
	judge := app
	extra := []enginetest.Doc{}
	var nets []enginetest.Doc
	version := rec.Version
	daemon := ""
	notEngine := false
	switch f.Fault {
	case "none":
	case "recorded-host":
		judge = host
	case TNetwork + "/default-bridge":
		networks(app, "bridge", map[string]string{"bridge": id(rec.Networks["bridge"])})
	case TNetwork + "/sidecar":
		judge = sidecar
	case TNetwork + "/host-network":
		networks(app, "host", map[string]string{"host": id(rec.Networks["host"])})
	case TNetwork + "/sidecar-of-host":
		enginetest.Set(sidecar, "HostConfig.NetworkMode", "container:"+id(host))
		judge = sidecar
	case TNetwork + "/none":
		judge = stopped
	case TNetwork + "/macvlan":
		mac := enginetest.Doc{"Name": "oec-rec-mac", "Id": strings.Repeat("7a1c", 16), "Driver": "macvlan", "Scope": "local"}
		nets = append(nets, mac)
		networks(app, "oec-rec-mac", map[string]string{"oec-rec-mac": id(mac)})
	case TNetwork + "/pasta":
		version = podman()
		networks(app, "pasta", nil)
	case TNetwork + "/network-gone":
		delete(rec.Networks, "oec-rec-net")
	case TName + "/hostname-id":
		enginetest.Set(app, "Config.Hostname", id(app)[:12])
	case TName + "/hostname-case":
		enginetest.Set(app, "Config.Hostname", strings.ToUpper(name(app)))
	case TName + "/underscore":
		rename(app, "oec_rec_app-"+uuid)
	case TName + "/too-long":
		rename(app, strings.Repeat("a", MaxDefaultName+1)+"-"+uuid)
	case TName + "/max-length":
		rename(app, strings.Repeat("a", MaxDefaultName)+"-"+uuid)
	case TName + "/duplicate-case":
		twin := enginetest.Clone(stopped)
		enginetest.Set(twin, "Id", strings.Repeat("c0ffee", 10)+"c0ff")
		rename(twin, strings.ToUpper(name(app)))
		extra = append(extra, twin)
	case TName + "/no-uuid":
		rename(app, "oec-rec-app")
	case TName + "/podman":
		version = podman()
		enginetest.Set(app, "Name", name(app))
	case TLicenses + "/bind":
		enginetest.SetMounts(app, enginetest.Mount("bind", "/srv/oec/rec/lic", PathLicenses, false))
	case TLicenses + "/bind-parent":
		enginetest.SetMounts(app, enginetest.Mount("bind", "/srv/oec/rec/oi4", "/opt/oi4", false))
	case TLicenses + "/bind-below":
		enginetest.SetMounts(app, enginetest.Mount("bind", "/srv/oec/rec/LICENSE.txt", PathLicenses+"/extra.txt", false))
	case TLicenses + "/tmpfs":
		enginetest.Get(app, "HostConfig.Tmpfs").(map[string]any)[PathLicenses] = "ro"
	case TLicenses + "/volume":
		m := enginetest.Mount("volume", "/var/lib/docker/volumes/oec-rec-lic/_data", PathLicenses, false)
		m["Name"] = "oec-rec-lic"
		enginetest.SetMounts(app, m)
	case TLicenses + "/image-mount":
		m := enginetest.Mount("image", "", PathLicenses, false)
		m["Mode"], m["Propagation"] = "", ""
		enginetest.SetMounts(app, m)
	case "engine/unreachable":
		daemon = "unix://" + filepath.ToSlash(filepath.Join(t.TempDir(), "missing.sock"))
	case "engine/not-an-engine":
		notEngine = true
	default:
		t.Fatalf("no scenario for fault %q", f.Fault)
	}
	e := rec.Engine(append([]enginetest.Doc{app, host, stopped, sidecar}, extra...)...)
	e.Version, e.NotEngine = version, notEngine
	e.Networks = nil
	for _, n := range []string{"bridge", "host", "none", "oec-rec-net"} {
		if d, ok := rec.Networks[n]; ok {
			e.Networks = append(e.Networks, d)
		}
	}
	e.Networks = append(e.Networks, nets...)
	s := enginetest.Start(t, e)
	if daemon == "" {
		daemon = s.Endpoint()
	}
	return Config{
		Containers: []string{name(judge)},
		Daemon:     daemon,
		Tests:      f.Tests,
		Timeout:    10 * time.Second,
		Getenv:     func(string) string { return "" },
		Defaults:   []string{},
	}
}

// TestFixtures runs every container fixture of testdata/containers/index.yaml
// and checks the verdict of its selected tests against the expectation.
func TestFixtures(t *testing.T) {
	tests, reqs := setup(t)
	fixtures, err := conformance.ContainerFixtures()
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no container fixtures")
	}
	for _, f := range fixtures {
		t.Run(f.Fault, func(t *testing.T) {
			t.Parallel()
			if f.Status != "draft" && f.Status != "reviewed" {
				t.Errorf("status %q", f.Status)
			}
			for id := range f.Expect {
				if !Implemented[id] || !f.Runs(id) {
					t.Errorf("expectation for %s, which is no implemented container test of the fixture", id)
				}
			}
			c := scenario(t, f)
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
				var with, manual []string
				for _, r := range tr.Reqs {
					switch {
					case r.Outcome == verdict.Manual:
						manual = append(manual, r.Req)
					case string(r.Outcome) == want.Outcome:
						with = append(with, r.Req)
					case r.Outcome != verdict.Skip && !verdict.Worse(tr.Outcome, r.Outcome):
						t.Errorf("%s: %s has %s, not better than the test outcome %s", tr.ID, r.Req, r.Outcome, tr.Outcome)
					}
				}
				if want.Outcome != string(verdict.Pass) && !equalSet(with, want.Reqs) {
					t.Errorf("%s: requirements with %s %v, want %v (%+v)", tr.ID, want.Outcome, with, want.Reqs, tr.Reqs)
				}
				if !equalSet(manual, want.Manual) {
					t.Errorf("%s: requirements with a manual verdict %v, want %v (%+v)", tr.ID, manual, want.Manual, tr.Reqs)
				}
				if f.Positive && tr.Outcome != verdict.Pass {
					t.Errorf("%s: a positive fixture must pass (%+v)", tr.ID, tr.Reqs)
				}
			}
			if strings.HasPrefix(f.Fault, "engine/") {
				if len(res.Run.Containers) != 0 || res.Run.Engine.Endpoint == "" {
					t.Errorf("run %+v", res.Run)
				}
			} else if len(res.Run.Containers) != 1 || res.Run.Containers[0].Name != c.Containers[0] {
				t.Errorf("run %+v", res.Run)
			}
		})
	}
}

func equalSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	sort.Strings(a)
	sort.Strings(b)
	return slices.Equal(a, b)
}

func fixture(t *testing.T, fault string) conformance.Fixture {
	t.Helper()
	fixtures, err := conformance.ContainerFixtures()
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(fixtures, func(f conformance.Fixture) bool { return f.Fault == fault })
	if i < 0 {
		t.Fatalf("no fixture %s", fault)
	}
	return fixtures[i]
}

// TestReport: the run reports the engine and the container without mount
// sources, environment or labels.
func TestReport(t *testing.T) {
	c := scenario(t, fixture(t, "none"))
	res, err := Run(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	e := res.Run.Engine
	if e.Product != "Docker Desktop 4.47.0 (206054)" || e.Version != "28.4.0" || e.API != "1.51" || e.OS != "linux" || e.Arch != "arm64" || e.Endpoint != c.Daemon {
		t.Errorf("engine %+v", e)
	}
	ctr := res.Run.Containers[0]
	if ctr.ID != "b33a87362c17" || ctr.Hostname != ctr.Name || !ctr.Running || ctr.NetworkMode != "oec-rec-net" {
		t.Errorf("container %+v", ctr)
	}
	if len(ctr.Networks) != 1 || ctr.Networks[0].Name != "oec-rec-net" || ctr.Networks[0].Driver != "bridge" {
		t.Errorf("networks %+v", ctr.Networks)
	}
	dsts := map[string]string{}
	for _, m := range ctr.Mounts {
		dsts[m.Destination] = m.Type + " " + m.Mode
	}
	for dst, want := range map[string]string{"/etc/oi4/app": "volume rw", "/opt/oi4/app": "volume rw", "/run/secrets/oi4": "bind ro", "/etc/oi4/mqtt": "bind ro", "/etc/oi4/certs": "bind rw", "/tmp": "tmpfs rw"} {
		if dsts[dst] != want {
			t.Errorf("mount %s: %q, want %q", dst, dsts[dst], want)
		}
	}
	b, _ := json.Marshal(res)
	for _, leak := range []string{"/srv/oec", "/var/lib/docker", "Env", "Labels"} {
		if strings.Contains(string(b), leak) {
			t.Errorf("the result contains %q", leak)
		}
	}
}

func TestNoContainer(t *testing.T) {
	c := scenario(t, fixture(t, "none"))
	c.Containers = []string{"oec-no-such-container"}
	if _, err := Run(context.Background(), c); !errors.Is(err, ErrNoContainer) {
		t.Errorf("error %v, want ErrNoContainer", err)
	}
}

// TestLevel: T8-CTR-01 is a level 2 test (REQ-CMP-031 and REQ-CNT-023 do
// not apply at level 1).
func TestLevel(t *testing.T) {
	tests, reqs := setup(t)
	c := scenario(t, fixture(t, TNetwork+"/host-network"))
	res, err := Run(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	for _, tr := range verdict.Build(tests, res.Observations, reqs, 1) {
		if tr.ID == TNetwork && (tr.Outcome != verdict.Skip || !strings.Contains(tr.Reason, "level 2")) {
			t.Errorf("%s at level 1: %s %q", tr.ID, tr.Outcome, tr.Reason)
		}
	}
	for _, tr := range verdict.Build(tests, res.Observations, reqs, 2) {
		if tr.ID == TNetwork && tr.Outcome != verdict.Warn {
			t.Errorf("%s at level 2: %s", tr.ID, tr.Outcome)
		}
	}
}

func TestPodman(t *testing.T) {
	c := scenario(t, fixture(t, TName+"/podman"))
	res, err := Run(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if e := res.Run.Engine; e.Product != "Podman Engine" || e.Version != "5.4.2" || e.API != "1.41" {
		t.Errorf("engine %+v", e)
	}
}

// TestDefaults: without --daemon and environment the defaults are tried;
// default sockets that do not exist are skipped.
func TestDefaults(t *testing.T) {
	missing := "unix://" + filepath.ToSlash(filepath.Join(t.TempDir(), "missing.sock"))
	res, err := Run(context.Background(), Config{Containers: []string{"x"}, Getenv: func(string) string { return "" }, Defaults: []string{missing}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Observations) == 0 || !strings.Contains(res.Observations[0].Detail, "none of the default endpoints exists") || res.Run.Engine.Endpoint != missing {
		t.Errorf("observations %+v, engine %+v", res.Observations, res.Run.Engine)
	}
	f := fixture(t, "none")
	c := scenario(t, f)
	daemon := c.Daemon
	c.Daemon, c.Defaults = "", []string{missing, daemon}
	res, err = Run(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if res.Run.Engine.Endpoint != daemon {
		t.Errorf("engine %+v, want %s", res.Run.Engine, daemon)
	}
	c.Defaults = []string{}
	c.Getenv = func(k string) string {
		if k == "DOCKER_HOST" {
			return daemon
		}
		return ""
	}
	if res, err = Run(context.Background(), c); err != nil || res.Run.Engine.Endpoint != daemon {
		t.Errorf("DOCKER_HOST: engine %+v, %v", res.Run.Engine, err)
	}
}

// TestRecordings: the recorded documents contain no personal path and no
// recording directory (sanitized to /srv/oec/rec).
func TestRecordings(t *testing.T) {
	err := filepath.Walk(enginetest.Dir(), func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, leak := range []string{"/Users/", "/home/", "/tmp/oec-rec", "authors"} {
			if strings.Contains(string(b), leak) {
				t.Errorf("%s contains %q", p, leak)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
