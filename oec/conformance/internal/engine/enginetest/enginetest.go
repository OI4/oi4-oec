// Package enginetest is a fake Docker/Podman Engine API for the self-tests
// of oec-infra (C-073, C-074): it serves recorded inspect documents (GET
// /_ping, /version, /containers/json, /containers/{id}/json,
// /networks/{id}, /images/{name}/json) over TCP loopback or a Unix socket,
// without a container daemon. With Engine.Run it also creates, starts,
// stops and removes containers (oec-infra run): a started container runs
// the hook in a goroutine – a simulated application in the process.
// Without Run every request other than GET fails the test.
package enginetest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
)

// Doc is a JSON document of the Engine API.
type Doc = map[string]any

// Engine is the state the fake serves.
type Engine struct {
	Version    Doc
	Containers []Doc // inspect documents
	Networks   []Doc
	// NotEngine: /_ping answers like a service other than an Engine API.
	NotEngine bool
	// Fail: paths (prefixes) answered with 500 (every method).
	Fail []string
	// Images are the image inspect documents (Image builds one); GET
	// /images/{name}/json finds them by id or repository tag.
	Images []Doc
	// Run accepts POST /containers/create, …/start, …/stop and DELETE
	// /containers/{id}; a started container runs Run until it returns (the
	// exit code) or its context ends (stop, remove, end of the test).
	Run func(ctx context.Context, s Started) int
}

// Started describes a started container to Engine.Run.
type Started struct {
	ID, Name, Hostname string
	Image              Doc
	Cmd                []string          // Entrypoint followed by Cmd of the image
	Binds              map[string]string // container path → host path
	ReadOnly           map[string]bool   // container path → bind is read-only
	ExtraHosts         []string
	Labels             map[string]string
	NetworkMode        string
}

// Image returns an image inspect document.
func Image(id string, tags []string, os, arch string, entrypoint, cmd []string) Doc {
	anys := func(s []string) []any {
		out := []any{}
		for _, x := range s {
			out = append(out, x)
		}
		return out
	}
	return Doc{"Id": id, "RepoTags": anys(tags), "Os": os, "Architecture": arch,
		"Config": map[string]any{"Entrypoint": anys(entrypoint), "Cmd": anys(cmd)}}
}

// Server is a running fake.
type Server struct {
	srv *httptest.Server
	ep  string

	mu       sync.Mutex
	engine   Engine
	requests []string
	writes   []string
	running  map[string]*proc // by container id
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

type proc struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func newServer(t testing.TB, e Engine) *Server {
	s := &Server{engine: e, running: map[string]*proc{}}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	return s
}

// close stops every simulated container and waits for it.
func (s *Server) close() {
	s.cancel()
	s.wg.Wait()
}

// Start serves the engine over TCP loopback until the test ends.
func Start(t testing.TB, e Engine) *Server {
	t.Helper()
	s := newServer(t, e)
	s.srv = httptest.NewServer(s.handler(t))
	s.ep = "tcp://" + s.srv.Listener.Addr().String()
	t.Cleanup(func() { s.srv.Close(); s.close() })
	return s
}

// StartUnix serves the engine over a Unix socket until the test ends.
func StartUnix(t testing.TB, e Engine) *Server {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Unix sockets: not on Windows")
	}
	// A short path: the limit of a socket path is about 104 octets.
	dir, err := os.MkdirTemp("", "eng")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "e.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	s := newServer(t, e)
	s.srv = httptest.NewUnstartedServer(s.handler(t))
	s.srv.Listener = ln
	s.srv.Start()
	s.ep = "unix://" + sock
	t.Cleanup(func() { s.srv.Close(); s.close() })
	return s
}

// Endpoint returns the endpoint of the fake (tcp:// or unix://).
func (s *Server) Endpoint() string { return s.ep }

// Requests returns the paths requested so far (with query).
func (s *Server) Requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.requests)
}

// Writes returns the requests other than GET so far ("POST /path?query").
func (s *Server) Writes() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.writes)
}

// Containers returns copies of the container documents the engine holds.
func (s *Server) Containers() []Doc {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Doc
	for _, c := range s.engine.Containers {
		out = append(out, Clone(c))
	}
	return out
}

func (s *Server) handler(t testing.TB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, r.URL.RequestURI())
		if r.Method != http.MethodGet {
			s.writes = append(s.writes, r.Method+" "+r.URL.RequestURI())
		}
		e := s.engine
		s.mu.Unlock()
		p := r.URL.Path
		for _, f := range e.Fail {
			if strings.HasPrefix(p, f) {
				message(w, http.StatusInternalServerError, "injected failure")
				return
			}
		}
		if r.Method != http.MethodGet {
			if e.Run == nil {
				t.Errorf("enginetest: %s %s: D8 only reads", r.Method, r.URL)
				http.Error(w, "read only", http.StatusMethodNotAllowed)
				return
			}
			s.write(w, r)
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		e = s.engine
		parts := strings.Split(strings.Trim(p, "/"), "/")
		switch {
		case p == "/_ping" && e.NotEngine:
			http.Error(w, "page not found", http.StatusNotFound)
		case p == "/_ping":
			w.Header().Set("Api-Version", str(e.Version, "ApiVersion"))
			fmt.Fprint(w, "OK")
		case p == "/version":
			write(w, e.Version)
		case p == "/containers/json":
			all := r.URL.Query().Get("all")
			var filters map[string][]string
			if f := r.URL.Query().Get("filters"); f != "" {
				if err := json.Unmarshal([]byte(f), &filters); err != nil {
					message(w, http.StatusBadRequest, "invalid filter")
					return
				}
			}
			var out []Doc
			for _, c := range e.Containers {
				running, _ := Get(c, "State.Running").(bool)
				if !running && all != "1" && all != "true" || !labelled(c, filters["label"]) {
					continue
				}
				out = append(out, Doc{"Id": c["Id"], "Names": []string{"/" + strings.TrimPrefix(str(c, "Name"), "/")}, "Image": Get(c, "Config.Image"), "State": Get(c, "State.Status"), "Labels": Get(c, "Config.Labels")})
			}
			if out == nil {
				out = []Doc{}
			}
			write(w, out)
		case len(parts) == 3 && parts[0] == "containers" && parts[2] == "json":
			if c := find(e.Containers, parts[1]); c != nil {
				write(w, c)
				return
			}
			message(w, http.StatusNotFound, "No such container: "+parts[1])
		case len(parts) >= 3 && parts[0] == "images" && parts[len(parts)-1] == "json":
			ref := strings.Join(parts[1:len(parts)-1], "/")
			if img := findImage(e.Images, ref); img != nil {
				write(w, img)
				return
			}
			message(w, http.StatusNotFound, "No such image: "+ref)
		case len(parts) == 2 && parts[0] == "networks":
			if n := find(e.Networks, parts[1]); n != nil {
				write(w, n)
				return
			}
			message(w, http.StatusNotFound, "network "+parts[1]+" not found")
		default:
			message(w, http.StatusNotFound, "page not found")
		}
	})
}

// labelled reports whether the container has every label filter ("k" or
// "k=v").
func labelled(c Doc, filters []string) bool {
	labels, _ := Get(c, "Config.Labels").(map[string]any)
	for _, f := range filters {
		k, v, hasV := strings.Cut(f, "=")
		got, ok := labels[k].(string)
		if !ok || hasV && got != v {
			return false
		}
	}
	return true
}

// findImage finds an image by id, unique id prefix or tag (":latest" if
// the reference has no tag).
func findImage(images []Doc, ref string) Doc {
	if !strings.Contains(ref, ":") || strings.LastIndex(ref, "/") > strings.LastIndex(ref, ":") {
		ref += ":latest"
	}
	for _, img := range images {
		id := str(img, "Id")
		if id == ref || strings.TrimPrefix(id, "sha256:") == strings.TrimSuffix(ref, ":latest") {
			return img
		}
		tags, _ := img["RepoTags"].([]any)
		for _, tg := range tags {
			if tg == ref {
				return img
			}
		}
	}
	return nil
}

// write handles the requests of oec-infra run.
func (s *Server) write(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/containers/create":
		s.create(w, r)
	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "containers" && parts[2] == "start":
		s.start(w, parts[1])
	case r.Method == http.MethodPost && len(parts) == 3 && parts[0] == "containers" && parts[2] == "stop":
		s.stop(w, parts[1], false)
	case r.Method == http.MethodDelete && len(parts) == 2 && parts[0] == "containers":
		s.stop(w, parts[1], true)
	default:
		message(w, http.StatusNotFound, "page not found")
	}
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Image      string            `json:"Image"`
		Hostname   string            `json:"Hostname"`
		Labels     map[string]string `json:"Labels"`
		HostConfig struct {
			Binds       []string `json:"Binds"`
			ExtraHosts  []string `json:"ExtraHosts"`
			NetworkMode string   `json:"NetworkMode"`
		} `json:"HostConfig"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		message(w, http.StatusBadRequest, err.Error())
		return
	}
	name := r.URL.Query().Get("name")
	s.mu.Lock()
	defer s.mu.Unlock()
	img := findImage(s.engine.Images, body.Image)
	if img == nil {
		message(w, http.StatusNotFound, "No such image: "+body.Image)
		return
	}
	if name != "" && find(s.engine.Containers, name) != nil {
		message(w, http.StatusConflict, "Conflict. The container name \"/"+name+"\" is already in use")
		return
	}
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	if name == "" {
		name = "fake_" + id[:6]
	}
	labels := map[string]any{}
	for k, v := range body.Labels {
		labels[k] = v
	}
	var mounts []any
	for _, bind := range body.HostConfig.Binds {
		// host:container[:mode]; a Windows host path has a colon itself.
		src, mode := bind, ""
		if i := strings.LastIndex(src, ":"); i >= 0 && (src[i+1:] == "ro" || src[i+1:] == "rw") {
			src, mode = src[:i], src[i+1:]
		}
		i := strings.LastIndex(src, ":")
		if i <= 0 {
			message(w, http.StatusBadRequest, "invalid bind "+bind)
			return
		}
		mounts = append(mounts, Mount("bind", src[:i], src[i+1:], mode != "ro"))
	}
	hosts := []any{}
	for _, h := range body.HostConfig.ExtraHosts {
		hosts = append(hosts, h)
	}
	binds := []any{}
	for _, b := range body.HostConfig.Binds {
		binds = append(binds, b)
	}
	c := Doc{
		"Id": id, "Name": "/" + name, "Image": img["Id"],
		"State":      map[string]any{"Status": "created", "Running": false, "ExitCode": 0},
		"Config":     map[string]any{"Hostname": body.Hostname, "Image": body.Image, "Labels": labels},
		"HostConfig": map[string]any{"NetworkMode": body.HostConfig.NetworkMode, "Binds": binds, "ExtraHosts": hosts},
		"Mounts":     mounts,
	}
	s.engine.Containers = append(s.engine.Containers, c)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(Doc{"Id": id, "Warnings": []any{}})
}

func strs(v any) []string {
	var out []string
	l, _ := v.([]any)
	for _, x := range l {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func (s *Server) start(w http.ResponseWriter, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := find(s.engine.Containers, key)
	if c == nil {
		message(w, http.StatusNotFound, "No such container: "+key)
		return
	}
	id := str(c, "Id")
	if s.running[id] != nil {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	img := findImage(s.engine.Images, str(c, "Image"))
	st := Started{ID: id, Name: strings.TrimPrefix(str(c, "Name"), "/"), Hostname: Get(c, "Config.Hostname").(string), Image: Clone(img),
		Binds: map[string]string{}, ReadOnly: map[string]bool{}, ExtraHosts: strs(Get(c, "HostConfig.ExtraHosts")), Labels: map[string]string{},
		NetworkMode: fmt.Sprint(Get(c, "HostConfig.NetworkMode"))}
	st.Cmd = append(strs(Get(img, "Config.Entrypoint")), strs(Get(img, "Config.Cmd"))...)
	for _, m := range Mounts(c) {
		m := m.(map[string]any)
		st.Binds[str(m, "Destination")] = str(m, "Source")
		st.ReadOnly[str(m, "Destination")] = m["RW"] == false
	}
	for k, v := range Get(c, "Config.Labels").(map[string]any) {
		st.Labels[k], _ = v.(string)
	}
	ctx, cancel := context.WithCancel(s.ctx)
	p := &proc{cancel: cancel, done: make(chan struct{})}
	s.running[id] = p
	Set(c, "State.Status", "running")
	Set(c, "State.Running", true)
	run := s.engine.Run
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer close(p.done)
		code := run(ctx, st)
		cancel()
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.running, id)
		if c := find(s.engine.Containers, id); c != nil {
			Set(c, "State.Status", "exited")
			Set(c, "State.Running", false)
			Set(c, "State.ExitCode", code)
		}
	}()
	w.WriteHeader(http.StatusNoContent)
}

// stop stops a running container (stop) or removes a container (remove,
// force).
func (s *Server) stop(w http.ResponseWriter, key string, remove bool) {
	s.mu.Lock()
	c := find(s.engine.Containers, key)
	if c == nil {
		s.mu.Unlock()
		message(w, http.StatusNotFound, "No such container: "+key)
		return
	}
	id := str(c, "Id")
	p := s.running[id]
	s.mu.Unlock()
	if p == nil && !remove {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if p != nil {
		p.cancel()
		<-p.done
	}
	if remove {
		s.mu.Lock()
		s.engine.Containers = slices.DeleteFunc(s.engine.Containers, func(d Doc) bool { return str(d, "Id") == id })
		s.mu.Unlock()
	}
	w.WriteHeader(http.StatusNoContent)
}

// find returns the document with the id, the name or the unique id prefix.
func find(docs []Doc, key string) Doc {
	var prefix []Doc
	for _, d := range docs {
		id, name := str(d, "Id"), strings.TrimPrefix(str(d, "Name"), "/")
		switch {
		case id == key || name == strings.TrimPrefix(key, "/"):
			return d
		case len(key) >= 3 && strings.HasPrefix(id, key):
			prefix = append(prefix, d)
		}
	}
	if len(prefix) == 1 {
		return prefix[0]
	}
	return nil
}

func write(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func message(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(Doc{"message": msg})
}

func str(d Doc, k string) string {
	s, _ := d[k].(string)
	return s
}

// Get returns the value at the dotted path ("HostConfig.NetworkMode").
func Get(d Doc, dotted string) any {
	var v any = d
	for _, k := range strings.Split(dotted, ".") {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}

// Set sets the value at the dotted path and creates missing objects.
func Set(d Doc, dotted string, v any) {
	keys := strings.Split(dotted, ".")
	m := d
	for _, k := range keys[:len(keys)-1] {
		next, ok := m[k].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[k] = next
		}
		m = next
	}
	m[keys[len(keys)-1]] = v
}

// Load reads a JSON document.
func Load(t testing.TB, file string) Doc {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var d Doc
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	return d
}

// Clone returns a deep copy of the document.
func Clone(d Doc) Doc {
	b, err := json.Marshal(d)
	if err != nil {
		panic(err)
	}
	var out Doc
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return out
}

// Mounts returns the mounts of an inspect document.
func Mounts(d Doc) []any {
	m, _ := d["Mounts"].([]any)
	return m
}

// Mount is a mount entry of an inspect document.
func Mount(typ, source, destination string, rw bool) Doc {
	mode := "rw"
	if !rw {
		mode = "ro"
	}
	m := Doc{"Type": typ, "Source": source, "Destination": destination, "Mode": mode, "RW": rw, "Propagation": "rprivate"}
	if typ == "volume" {
		m["Driver"] = "local"
		m["Propagation"] = ""
		m["Mode"] = "z"
	}
	return m
}

// SetMounts replaces the mounts with the destination of one of ms and
// appends ms.
func SetMounts(d Doc, ms ...Doc) {
	var out []any
	for _, m := range Mounts(d) {
		dst := str(m.(map[string]any), "Destination")
		keep := true
		for _, n := range ms {
			if dst == str(n, "Destination") {
				keep = false
			}
		}
		if keep {
			out = append(out, m)
		}
	}
	for _, m := range ms {
		out = append(out, m)
	}
	d["Mounts"] = out
}

// RemoveMount removes the mounts at the destination (also from
// HostConfig.Tmpfs).
func RemoveMount(d Doc, destination string) {
	var out []any
	for _, m := range Mounts(d) {
		if str(m.(map[string]any), "Destination") != destination {
			out = append(out, m)
		}
	}
	d["Mounts"] = out
	if t, ok := Get(d, "HostConfig.Tmpfs").(map[string]any); ok {
		delete(t, destination)
	}
}

// Recording are the documents recorded from Docker Engine 28.4 (API 1.51,
// Docker Desktop on macOS) in testdata/containers/engine (C-073): the
// conforming application App (bridge network oec-rec-net, own volumes for
// /etc/oi4/app and /opt/oi4/app, read-only binds for the secrets and the
// MQTT storage), Host (host network, a bind on /opt/oi4/licenses, a tmpfs
// on /etc/oi4/app, an image mount on /opt/oi4/app), Stopped (created, never
// started, network none, the volume of App on /etc/oi4/app) and Sidecar
// (network of App: container:<id>). Every call returns fresh copies.
type Recording struct {
	Version                     Doc
	App, Host, Stopped, Sidecar Doc
	Networks                    map[string]Doc // by name
}

// Dir returns the directory of the recorded documents.
func Dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "containers", "engine")
}

// Recorded loads the recorded documents.
func Recorded(t testing.TB) Recording {
	t.Helper()
	dir := Dir()
	r := Recording{
		Version:  Load(t, filepath.Join(dir, "version.json")),
		App:      Load(t, filepath.Join(dir, "containers", "app.json")),
		Host:     Load(t, filepath.Join(dir, "containers", "host.json")),
		Stopped:  Load(t, filepath.Join(dir, "containers", "stopped.json")),
		Sidecar:  Load(t, filepath.Join(dir, "containers", "sidecar.json")),
		Networks: map[string]Doc{},
	}
	for _, n := range []string{"bridge", "host", "none", "oec-rec-net"} {
		r.Networks[n] = Load(t, filepath.Join(dir, "networks", n+".json"))
	}
	return r
}

// Engine returns an engine with the version, every recorded network and
// the containers.
func (r Recording) Engine(containers ...Doc) Engine {
	e := Engine{Version: r.Version, Containers: containers}
	for _, n := range []string{"bridge", "host", "none", "oec-rec-net"} {
		e.Networks = append(e.Networks, r.Networks[n])
	}
	return e
}
