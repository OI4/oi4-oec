// Package enginetest is a fake Docker/Podman Engine API for the self-tests
// of oec-infra (C-073): it serves recorded inspect documents (GET /_ping,
// /version, /containers/json, /containers/{id}/json, /networks/{id}) over
// TCP loopback or a Unix socket, without a container daemon. Every request
// other than GET fails the test (D8 only reads).
package enginetest

import (
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
	// Fail: paths (prefixes) answered with 500.
	Fail []string
}

// Server is a running fake.
type Server struct {
	srv *httptest.Server
	ep  string

	mu       sync.Mutex
	engine   Engine
	requests []string
}

// Start serves the engine over TCP loopback until the test ends.
func Start(t testing.TB, e Engine) *Server {
	t.Helper()
	s := &Server{engine: e}
	s.srv = httptest.NewServer(s.handler(t))
	s.ep = "tcp://" + s.srv.Listener.Addr().String()
	t.Cleanup(s.srv.Close)
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
	s := &Server{engine: e}
	s.srv = httptest.NewUnstartedServer(s.handler(t))
	s.srv.Listener = ln
	s.srv.Start()
	s.ep = "unix://" + sock
	t.Cleanup(s.srv.Close)
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

func (s *Server) handler(t testing.TB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, r.URL.RequestURI())
		e := s.engine
		s.mu.Unlock()
		if r.Method != http.MethodGet {
			t.Errorf("enginetest: %s %s: D8 only reads", r.Method, r.URL)
			http.Error(w, "read only", http.StatusMethodNotAllowed)
			return
		}
		p := r.URL.Path
		for _, f := range e.Fail {
			if strings.HasPrefix(p, f) {
				message(w, http.StatusInternalServerError, "injected failure")
				return
			}
		}
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
			var out []Doc
			for _, c := range e.Containers {
				running, _ := Get(c, "State.Running").(bool)
				if !running && all != "1" && all != "true" {
					continue
				}
				out = append(out, Doc{"Id": c["Id"], "Names": []string{"/" + strings.TrimPrefix(str(c, "Name"), "/")}, "Image": Get(c, "Config.Image"), "State": Get(c, "State.Status")})
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
