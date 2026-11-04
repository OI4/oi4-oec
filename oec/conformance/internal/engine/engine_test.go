package engine_test

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	. "github.com/OI4/oec/conformance/internal/engine"
	"github.com/OI4/oec/conformance/internal/engine/enginetest"
)

// TestClient reads the recorded documents over TCP and a Unix socket.
func TestClient(t *testing.T) {
	rec := enginetest.Recorded(t)
	for _, start := range []func(testing.TB, enginetest.Engine) *enginetest.Server{enginetest.Start, enginetest.StartUnix} {
		srv := start(t, rec.Engine(rec.App, rec.Host, rec.Stopped, rec.Sidecar))
		t.Run(strings.SplitN(srv.Endpoint(), ":", 2)[0], func(t *testing.T) {
			ctx := context.Background()
			c, err := Open(ctx, srv.Endpoint(), 0)
			if err != nil {
				t.Fatal(err)
			}
			if c.Endpoint() != srv.Endpoint() {
				t.Errorf("endpoint %s", c.Endpoint())
			}
			v, err := c.Version(ctx)
			if err != nil || v.Version != "28.4.0" || v.APIVersion != "1.51" || v.Os != "linux" || v.Arch != "arm64" || !strings.HasPrefix(v.Product(), "Docker Desktop") {
				t.Errorf("version %+v (%s), %v", v, v.Product(), err)
			}
			running, err := c.Containers(ctx, false)
			if err != nil || len(running) != 3 {
				t.Errorf("running %+v, %v", running, err)
			}
			all, err := c.Containers(ctx, true)
			if err != nil || len(all) != 4 || all[2].Name() != "oec-rec-stopped" || all[2].State != "created" {
				t.Errorf("all %+v, %v", all, err)
			}
			app, err := c.Inspect(ctx, "oec-rec-app-0b6a2c1e-4f3d-4c8a-9e21-7d5f3a9b8c40")
			if err != nil {
				t.Fatal(err)
			}
			if app.ContainerName() != app.Config.Hostname || app.HostConfig.NetworkMode != "oec-rec-net" || !app.State.Running || len(app.ShortID()) != 12 {
				t.Errorf("app %+v", app)
			}
			var dsts []string
			for _, m := range app.AllMounts() {
				dsts = append(dsts, m.Type+":"+m.Destination)
			}
			if want := []string{"bind:/etc/oi4/certs", "bind:/etc/oi4/mqtt", "volume:/etc/oi4/app", "volume:/mosquitto/data", "volume:/mosquitto/log", "volume:/opt/oi4/app", "bind:/run/secrets/oi4", "tmpfs:/tmp"}; !sameSet(dsts, want) {
				t.Errorf("mounts %v, want %v", dsts, want)
			}
			host, err := c.Inspect(ctx, rec.Host["Id"].(string)[:8])
			if err != nil || host.HostConfig.NetworkMode != "host" {
				t.Fatalf("host %+v, %v", host, err)
			}
			var tmpfs, image bool
			for _, m := range host.AllMounts() {
				tmpfs = tmpfs || m.Type == "tmpfs" && m.Destination == "/etc/oi4/app" && m.RW
				image = image || m.Type == "image" && m.Destination == "/opt/oi4/app" && !m.RW
			}
			if !tmpfs || !image {
				t.Errorf("host mounts %+v", host.AllMounts())
			}
			id := app.NetworkSettings.Networks["oec-rec-net"].NetworkID
			n, err := c.Network(ctx, id)
			if err != nil || n.Name != "oec-rec-net" || n.Driver != "bridge" {
				t.Errorf("network %+v, %v", n, err)
			}
			if _, err := c.Inspect(ctx, "nosuch"); !errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "No such container") {
				t.Errorf("unknown container: %v", err)
			}
			if _, err := c.Network(ctx, "nosuch"); !errors.Is(err, ErrNotFound) {
				t.Errorf("unknown network: %v", err)
			}
			for _, r := range srv.Requests() {
				if strings.HasPrefix(r, "/v1.") {
					t.Errorf("versioned path %s", r)
				}
			}
			if !slices.Contains(srv.Requests(), "/containers/json?all=1") {
				t.Errorf("requests %v", srv.Requests())
			}
		})
	}
}

func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// TestOpen: an endpoint that is no Engine API, a missing socket and a
// failure of the engine.
func TestOpen(t *testing.T) {
	ctx := context.Background()
	rec := enginetest.Recorded(t)
	not := enginetest.Start(t, enginetest.Engine{NotEngine: true})
	if _, err := Open(ctx, not.Endpoint(), 0); !errors.Is(err, ErrNotEngine) {
		t.Errorf("not an engine: %v", err)
	}
	var de *DialError
	if _, err := Open(ctx, "unix://"+filepath.Join(t.TempDir(), "missing.sock"), 0); !errors.As(err, &de) || !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing socket: %v", err)
	}
	if _, err := Open(ctx, "ssh://user@host", 0); !errors.Is(err, ErrUnsupported) {
		t.Errorf("ssh: %v", err)
	}
	e := rec.Engine(rec.App)
	e.Fail = []string{"/containers/"}
	srv := enginetest.Start(t, e)
	c, err := Open(ctx, srv.Endpoint(), 0)
	if err != nil {
		t.Fatal(err)
	}
	var se *StatusError
	if _, err := c.Inspect(ctx, "x"); !errors.As(err, &se) || se.Code != 500 || errors.Is(err, ErrNotFound) {
		t.Errorf("failure: %v", err)
	}
}

// TestTimeout: an endpoint that accepts but never answers ends with the
// timeout of the client.
func TestTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close()
		}
	}()
	c, err := New("tcp://"+ln.Addr().String(), 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := c.Version(context.Background()); err == nil || time.Since(start) > 3*time.Second {
		t.Errorf("silent endpoint: %v after %v", err, time.Since(start))
	}
}

func TestEndpoints(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	def := []string{"unix:///var/run/docker.sock"}
	if eps, x := Endpoints("", env(nil), def); x || !slices.Equal(eps, def) {
		t.Errorf("defaults: %v %v", eps, x)
	}
	if eps, x := Endpoints("", env(map[string]string{"DOCKER_HOST": "tcp://h:2375", "CONTAINER_HOST": "/run/p.sock"}), def); !x || !slices.Equal(eps, []string{"tcp://h:2375", "unix:///run/p.sock"}) {
		t.Errorf("environment: %v %v", eps, x)
	}
	if eps, x := Endpoints(`\\.\pipe\docker_engine`, env(map[string]string{"DOCKER_HOST": "tcp://h:2375"}), def); !x || !slices.Equal(eps, []string{"npipe:////./pipe/docker_engine"}) {
		t.Errorf("explicit: %v %v", eps, x)
	}
	if len(Defaults()) == 0 {
		t.Error("no defaults")
	}
}

func TestWithin(t *testing.T) {
	for _, c := range []struct {
		p, dir string
		want   bool
	}{{"/etc/oi4", "/etc/oi4", true}, {"/etc/oi4/app/", "/etc/oi4", true}, {"/etc/oi4x", "/etc/oi4", false}, {"/etc", "/etc/oi4", false}, {"/x", "/", true}} {
		if Within(c.p, c.dir) != c.want {
			t.Errorf("Within(%s, %s) != %v", c.p, c.dir, c.want)
		}
	}
}

func TestEndpoint(t *testing.T) {
	for in, want := range map[string]string{"/var/run/docker.sock": "unix:///var/run/docker.sock", "tcp://h:2375": "tcp://h:2375", `\\.\pipe\docker_engine`: "npipe:////./pipe/docker_engine"} {
		if got := Endpoint(in); got != want {
			t.Errorf("%s: %s, want %s", in, got, want)
		}
	}
}

// TestProbe connects to a socket, a TCP port and a missing socket.
func TestProbe(t *testing.T) {
	ctx := context.Background()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	if err := Probe(ctx, "tcp://"+ln.Addr().String(), 0); err != nil {
		t.Errorf("tcp: %v", err)
	}
	missing := filepath.Join(t.TempDir(), "missing.sock")
	if err := Probe(ctx, "unix://"+missing, 0); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing socket: %v", err)
	}
	if err := Probe(ctx, "ssh://user@host", 0); !errors.Is(err, ErrUnsupported) {
		t.Errorf("ssh: %v", err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if err := Probe(ctx, "npipe:////./pipe/docker_engine", 0); !errors.Is(err, ErrUnsupported) {
		t.Errorf("npipe: %v", err)
	}
	// A short path: the limit of a socket path is about 104 octets.
	dir, err := os.MkdirTemp("", "d8")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	sock := filepath.Join(dir, "d.sock")
	ul, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			c, err := ul.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	if err := Probe(ctx, "unix://"+sock, 0); err != nil {
		t.Errorf("unix: %v", err)
	}
	ul.Close()
	// A socket file without a listener (a stopped daemon): refused.
	stale := filepath.Join(dir, "stale.sock")
	sl, err := net.Listen("unix", stale)
	if err != nil {
		t.Fatal(err)
	}
	sl.(*net.UnixListener).SetUnlinkOnClose(false)
	sl.Close()
	if err := Probe(ctx, "unix://"+stale, 0); err == nil || errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
		t.Errorf("stale socket: %v, want connection refused", err)
	}
}

// TestWrite: the client creates, starts, stops and removes only its own
// containers (label and name prefix); a foreign container is refused
// before any request that changes it (C-074).
func TestWrite(t *testing.T) {
	rec := enginetest.Recorded(t)
	e := rec.Engine(rec.App)
	e.Images = []enginetest.Doc{enginetest.Image("sha256:"+strings.Repeat("ab", 32), []string{"oec-d8-sim:good"}, "linux", "arm64", []string{"/oec-run-sim"}, []string{"--fault", "good"})}
	started := make(chan enginetest.Started, 1)
	e.Run = func(ctx context.Context, s enginetest.Started) int {
		started <- s
		<-ctx.Done()
		return 143
	}
	srv := enginetest.Start(t, e)
	ctx := context.Background()
	c, err := Open(ctx, srv.Endpoint(), 0)
	if err != nil {
		t.Fatal(err)
	}
	img, err := c.Image(ctx, "oec-d8-sim:good")
	if err != nil || img.Os != "linux" || img.Architecture != "arm64" || !strings.HasPrefix(img.ID, "sha256:abab") {
		t.Fatalf("image %+v, %v", img, err)
	}
	if _, err := c.Image(ctx, "oec-d8-sim:nosuch"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown image: %v", err)
	}
	gw, err := c.Gateway(ctx, "bridge")
	if err != nil || gw != "172.17.0.1" {
		t.Errorf("gateway %q, %v", gw, err)
	}
	own := map[string]string{LabelOwner: OwnerRun, LabelRun: "r1"}
	spec := CreateSpec{Name: OwnPrefix + "run-x", Image: "oec-d8-sim:good", Hostname: OwnPrefix + "run-x", Labels: own,
		Binds: []string{"/tmp/a:/etc/oi4/mqtt:ro", `C:\tmp\b:/etc/oi4/app`}, ExtraHosts: []string{"oec-d8-broker:host-gateway"}, NetworkMode: "bridge"}
	for _, bad := range []CreateSpec{
		{Name: "app", Image: spec.Image, Labels: own},
		{Name: spec.Name, Image: spec.Image, Labels: map[string]string{LabelRun: "r1"}},
	} {
		if _, err := c.Create(ctx, bad); !errors.Is(err, ErrNotOwn) {
			t.Errorf("create %+v: %v", bad, err)
		}
	}
	appName := rec.App["Name"].(string)[1:]
	for name, op := range map[string]func(string) error{
		"start":  func(id string) error { return c.Start(ctx, id) },
		"stop":   func(id string) error { return c.Stop(ctx, id, time.Second) },
		"remove": func(id string) error { return c.Remove(ctx, id) },
	} {
		if err := op(appName); !errors.Is(err, ErrNotOwn) {
			t.Errorf("%s of a foreign container: %v", name, err)
		}
	}
	if w := srv.Writes(); len(w) != 0 {
		t.Fatalf("writes %v", w)
	}
	id, err := c.Create(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Start(ctx, id); err != nil {
		t.Fatal(err)
	}
	s := <-started
	if s.Name != spec.Name || s.Hostname != spec.Hostname || s.Binds["/etc/oi4/mqtt"] != "/tmp/a" || !s.ReadOnly["/etc/oi4/mqtt"] ||
		s.Binds["/etc/oi4/app"] != `C:\tmp\b` || s.ReadOnly["/etc/oi4/app"] ||
		!slices.Equal(s.Cmd, []string{"/oec-run-sim", "--fault", "good"}) || !slices.Equal(s.ExtraHosts, spec.ExtraHosts) {
		t.Errorf("started %+v", s)
	}
	ctr, err := c.Inspect(ctx, id)
	if err != nil || !ctr.State.Running || ctr.Config.Labels[LabelRun] != "r1" || ctr.ContainerName() != spec.Name {
		t.Errorf("inspect %+v, %v", ctr, err)
	}
	list, err := c.Own(ctx, "r1")
	if err != nil || len(list) != 1 || list[0].ID != id {
		t.Errorf("own %+v, %v", list, err)
	}
	if list, _ := c.Own(ctx, "r2"); len(list) != 0 {
		t.Errorf("own r2 %+v", list)
	}
	if err := c.Stop(ctx, id, time.Second); err != nil {
		t.Fatal(err)
	}
	if ctr, _ := c.Inspect(ctx, id); ctr.State.Running || ctr.State.ExitCode != 143 {
		t.Errorf("stopped %+v", ctr.State)
	}
	if err := c.Stop(ctx, id, time.Second); err != nil {
		t.Errorf("stop of a stopped container: %v", err)
	}
	if err := c.Remove(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Inspect(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("removed: %v", err)
	}
	want := []string{"POST /containers/create?name=" + spec.Name, "POST /containers/" + id + "/start", "POST /containers/" + id + "/stop?t=1", "DELETE /containers/" + id + "?force=1&v=1"}
	if w := srv.Writes(); !slices.Equal(w, want) {
		t.Errorf("writes %v, want %v", w, want)
	}
}
