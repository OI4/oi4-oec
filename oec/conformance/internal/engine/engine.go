// Package engine is the own, minimal client of the Docker Engine API that
// oec-infra uses (DESIGN.md §9, ADR-0025 item 7; C-073, C-074). Podman
// serves the same endpoints in its compatibility API. It reads with GET
// /_ping, /version, /containers/json, /containers/{id}/json,
// /networks/{id} and /images/{name}/json. It writes only for the sandbox of
// oec-infra run (C-074) and only its own containers: Create needs the name
// prefix OwnPrefix and the label LabelOwner=OwnerRun; Start, Stop and Remove
// first inspect the container and refuse every other one (ErrNotOwn). It
// never pulls, loads or removes images and never creates or removes
// volumes or networks. It speaks HTTP/1.1 over one connection per request
// (Connection: close) to a Unix socket, a TCP port or a Windows named pipe;
// the paths are unversioned (the engine answers in its own API version,
// which the report names).
package engine

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

// Errors of the client.
var (
	// ErrUnsupported: an endpoint scheme D8 does not support (e.g.
	// ssh://) or a named pipe on a system other than Windows.
	ErrUnsupported = errors.New("endpoint not supported by D8")
	// ErrNotFound: the engine answers 404 (no such container or network).
	ErrNotFound = errors.New("not found")
	// ErrNotEngine: the endpoint answers, but not as an Engine API (e.g.
	// the gRPC socket of containerd).
	ErrNotEngine = errors.New("no Docker/Podman Engine API")
	// ErrNotOwn: a container oec-infra did not create (no OwnPrefix name or
	// no LabelOwner=OwnerRun label); the client does not change it.
	ErrNotOwn = errors.New("not a container of oec-infra run")
)

// The marks of the containers oec-infra run creates (C-074): every one
// carries the name prefix and the owner label; the run label tells the
// runs apart.
const (
	OwnPrefix  = "oec-d8-"
	LabelOwner = "org.oi4.oec-infra"
	OwnerRun   = "run"
	LabelRun   = "org.oi4.oec-infra.run"
)

// DefaultTry is the longest time one connection attempt to an endpoint may
// take.
const DefaultTry = 2 * time.Second

// DefaultTimeout is the longest time of one request.
const DefaultTimeout = 10 * time.Second

// MaxResponse limits the body of one answer (a container list of a large
// platform stays far below).
const MaxResponse = 32 << 20

// Endpoint normalizes a socket path or a named pipe to a URL; a URL stays
// as it is.
func Endpoint(s string) string {
	switch {
	case strings.Contains(s, "://"):
		return s
	case strings.HasPrefix(s, `\\.\pipe\`):
		return "npipe://" + strings.ReplaceAll(s, `\`, "/")
	}
	return "unix://" + s
}

// Endpoints returns the endpoints to try and whether they were named
// explicitly: the explicit one (--daemon), else DOCKER_HOST and
// CONTAINER_HOST, else the defaults of the system.
func Endpoints(explicit string, getenv func(string) string, defaults []string) ([]string, bool) {
	if explicit != "" {
		return []string{Endpoint(explicit)}, true
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	var out []string
	for _, env := range []string{"DOCKER_HOST", "CONTAINER_HOST"} {
		if v := getenv(env); v != "" {
			out = append(out, Endpoint(v))
		}
	}
	if len(out) > 0 {
		return out, true
	}
	return defaults, false
}

// target is a parsed endpoint.
type target struct{ network, addr string }

func parse(ep string) (target, error) {
	u, err := url.Parse(Endpoint(ep))
	if err != nil {
		return target{}, err
	}
	switch u.Scheme {
	case "unix":
		p := u.Path
		if p == "" {
			p = u.Opaque
		}
		return target{"unix", p}, nil
	case "tcp":
		if u.Host == "" {
			return target{}, fmt.Errorf("%s: no host", ep)
		}
		return target{"tcp", u.Host}, nil
	case "npipe":
		return target{"npipe", strings.ReplaceAll(u.Host+u.Path, "/", `\`)}, nil
	}
	return target{}, fmt.Errorf("%w: %s", ErrUnsupported, u.Scheme)
}

func (t target) dial(ctx context.Context) (net.Conn, error) {
	var d net.Dialer
	switch t.network {
	case "unix":
		if _, err := os.Stat(t.addr); err != nil {
			return nil, err
		}
		return d.DialContext(ctx, "unix", t.addr)
	case "tcp":
		return d.DialContext(ctx, "tcp", t.addr)
	}
	return openPipe(ctx, t.addr)
}

// Probe connects to the endpoint of a container daemon and closes the
// connection (REQ-PLT-006, oec-infra host): unix://, tcp:// and npipe://
// (Windows). The attempt takes at most timeout (≤ 0 or more than
// DefaultTry: DefaultTry).
func Probe(ctx context.Context, ep string, timeout time.Duration) error {
	t, err := parse(ep)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, try(timeout))
	defer cancel()
	c, err := t.dial(ctx)
	if err != nil {
		return err
	}
	return c.Close()
}

func try(timeout time.Duration) time.Duration {
	if timeout <= 0 || timeout > DefaultTry {
		return DefaultTry
	}
	return timeout
}

// Client is a client of one Engine API endpoint.
type Client struct {
	endpoint string
	target   target
	timeout  time.Duration
}

// New returns a client of the endpoint; every request takes at most
// timeout (≤ 0: DefaultTimeout). It does not connect.
func New(ep string, timeout time.Duration) (*Client, error) {
	t, err := parse(ep)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{endpoint: Endpoint(ep), target: t, timeout: timeout}, nil
}

// Open returns a client of the endpoint after a successful /_ping.
func Open(ctx context.Context, ep string, timeout time.Duration) (*Client, error) {
	c, err := New(ep, timeout)
	if err != nil {
		return nil, err
	}
	if err := c.Ping(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// Connect opens the first endpoint that answers as an Engine API: the
// explicit one, else DOCKER_HOST and CONTAINER_HOST, else the defaults (a
// default socket that does not exist is skipped). It returns the endpoints
// it tried.
func Connect(ctx context.Context, explicit string, getenv func(string) string, defaults []string, timeout time.Duration) (*Client, []string, error) {
	eps, named := Endpoints(explicit, getenv, defaults)
	var tried []string
	var last error
	for _, ep := range eps {
		cl, err := Open(ctx, ep, timeout)
		if err == nil {
			return cl, []string{ep}, nil
		}
		if !named && (errors.Is(err, fs.ErrNotExist) || errors.Is(err, ErrUnsupported)) {
			continue
		}
		tried, last = append(tried, ep), err
	}
	if last == nil {
		return nil, eps, fmt.Errorf("none of the default endpoints exists")
	}
	return nil, tried, last
}

// Endpoint returns the endpoint of the client.
func (c *Client) Endpoint() string { return c.endpoint }

// DialError is a failed connection to the endpoint (no answer at all).
type DialError struct {
	Endpoint string
	Err      error
}

func (e *DialError) Error() string { return e.Endpoint + ": " + e.Err.Error() }
func (e *DialError) Unwrap() error { return e.Err }

// Ping checks that the endpoint is an Engine API: GET /_ping answers OK
// within DefaultTry. A connection error is a *DialError; an endpoint that
// answers otherwise is ErrNotEngine.
func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, try(c.timeout))
	defer cancel()
	body, err := c.do(ctx, http.MethodGet, "/_ping", nil, nil)
	var de *DialError
	switch {
	case errors.As(err, &de):
		return err
	case err != nil:
		return fmt.Errorf("%w at %s: %v", ErrNotEngine, c.endpoint, err)
	case strings.TrimSpace(string(body)) != "OK":
		return fmt.Errorf("%w at %s: /_ping answers %q", ErrNotEngine, c.endpoint, truncate(string(body), 40))
	}
	return nil
}

// StatusError is an answer of the engine other than 2xx.
type StatusError struct {
	Method  string // "" is GET
	Path    string
	Code    int
	Message string
}

func (e *StatusError) Error() string {
	m := e.Method
	if m == "" {
		m = http.MethodGet
	}
	if e.Message != "" {
		return fmt.Sprintf("%s %s: %d %s", m, e.Path, e.Code, e.Message)
	}
	return fmt.Sprintf("%s %s: %d %s", m, e.Path, e.Code, http.StatusText(e.Code))
}

// Is maps 404 to ErrNotFound.
func (e *StatusError) Is(target error) bool {
	return target == ErrNotFound && e.Code == http.StatusNotFound
}

// do sends one request over a new connection and returns the body of a
// 2xx answer; body (JSON) only for POST.
func (c *Client) do(ctx context.Context, method, p string, query url.Values, body []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	conn, err := c.target.dial(ctx)
	if err != nil {
		return nil, &DialError{Endpoint: c.endpoint, Err: err}
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Unix(1, 0)) })
	defer stop()
	host := "engine"
	if c.target.network == "tcp" {
		host = c.target.addr
	}
	u := &url.URL{Scheme: "http", Host: host, Path: p, RawQuery: query.Encode()}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
	if err != nil {
		return nil, err
	}
	req.Close = true
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "oec-infra")
	if err := req.Write(conn); err != nil {
		return nil, ctxErr(ctx, method, p, err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return nil, ctxErr(ctx, method, p, err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponse+1))
	if err != nil {
		return nil, ctxErr(ctx, method, p, err)
	}
	if len(out) > MaxResponse {
		return nil, fmt.Errorf("%s %s: answer larger than %d octets", method, p, MaxResponse)
	}
	if resp.StatusCode/100 != 2 {
		var m struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(out, &m)
		se := &StatusError{Path: p, Code: resp.StatusCode, Message: m.Message}
		if method != http.MethodGet {
			se.Method = method
		}
		return nil, se
	}
	return out, nil
}

func ctxErr(ctx context.Context, method, p string, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%s %s: %w (%v)", method, p, ctx.Err(), err)
	}
	return fmt.Errorf("%s %s: %w", method, p, err)
}

func (c *Client) getJSON(ctx context.Context, p string, query url.Values, v any) error {
	body, err := c.do(ctx, http.MethodGet, p, query, nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("GET %s: %w", p, err)
	}
	return nil
}

// Version is the subset of GET /version D8 reports.
type Version struct {
	Platform struct {
		Name string `json:"Name"`
	} `json:"Platform"`
	Components []struct {
		Name    string `json:"Name"`
		Version string `json:"Version"`
	} `json:"Components"`
	Version       string `json:"Version"`
	APIVersion    string `json:"ApiVersion"`
	MinAPIVersion string `json:"MinAPIVersion"`
	Os            string `json:"Os"`
	Arch          string `json:"Arch"`
}

// Product names the engine: the Podman component if there is one, else
// the platform name, else "Docker Engine".
func (v Version) Product() string {
	for _, c := range v.Components {
		if strings.Contains(c.Name, "Podman") {
			return c.Name
		}
	}
	if v.Platform.Name != "" {
		return v.Platform.Name
	}
	return "Docker Engine"
}

// Version returns GET /version.
func (c *Client) Version(ctx context.Context) (Version, error) {
	var v Version
	err := c.getJSON(ctx, "/version", nil, &v)
	return v, err
}

// Summary is one entry of GET /containers/json.
type Summary struct {
	ID    string   `json:"Id"`
	Names []string `json:"Names"`
	Image string   `json:"Image"`
	State string   `json:"State"`
	// Labels decide whether a container is one of oec-infra run; never
	// reported.
	Labels map[string]string `json:"Labels"`
}

// Name returns the first name without the leading slash.
func (s Summary) Name() string {
	if len(s.Names) == 0 {
		return ""
	}
	return strings.TrimPrefix(s.Names[0], "/")
}

// Containers returns GET /containers/json; all: also the containers that
// do not run.
func (c *Client) Containers(ctx context.Context, all bool) ([]Summary, error) {
	q := url.Values{}
	if all {
		q.Set("all", "1")
	}
	var out []Summary
	err := c.getJSON(ctx, "/containers/json", q, &out)
	return out, err
}

// Container is the subset of GET /containers/{id}/json D8 judges.
type Container struct {
	ID    string `json:"Id"`
	Name  string `json:"Name"`
	Image string `json:"Image"`
	State struct {
		Status   string `json:"Status"`
		Running  bool   `json:"Running"`
		ExitCode int    `json:"ExitCode"`
	} `json:"State"`
	Config struct {
		Hostname string `json:"Hostname"`
		Image    string `json:"Image"`
		// Labels decide whether a container is one of oec-infra run
		// (Own); never reported.
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	HostConfig struct {
		NetworkMode string            `json:"NetworkMode"`
		Tmpfs       map[string]string `json:"Tmpfs"`
	} `json:"HostConfig"`
	NetworkSettings struct {
		Networks map[string]struct {
			NetworkID string `json:"NetworkID"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
	Mounts []Mount `json:"Mounts"`
}

// ContainerName returns the name without the leading slash.
func (c *Container) ContainerName() string { return strings.TrimPrefix(c.Name, "/") }

// ShortID returns the first 12 characters of the id.
func (c *Container) ShortID() string {
	if len(c.ID) > 12 {
		return c.ID[:12]
	}
	return c.ID
}

// Mount is one mount of a container. Type is bind, volume, tmpfs, npipe,
// image or cluster; Source is the path on the host (for a volume: its
// mount point on the host of the engine).
type Mount struct {
	Type        string `json:"Type"`
	Name        string `json:"Name,omitempty"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
	Mode        string `json:"Mode"`
	RW          bool   `json:"RW"`
}

// AllMounts returns the mounts of the container including the --tmpfs
// mounts of the host configuration (Docker does not list them in Mounts),
// sorted by destination.
func (c *Container) AllMounts() []Mount {
	out := append([]Mount{}, c.Mounts...)
	for dst, opts := range c.HostConfig.Tmpfs {
		dup := false
		for _, m := range c.Mounts {
			dup = dup || path.Clean(m.Destination) == path.Clean(dst)
		}
		if !dup {
			out = append(out, Mount{Type: "tmpfs", Destination: dst, Mode: opts, RW: !hasOpt(opts, "ro")})
		}
	}
	sortMounts(out)
	return out
}

func sortMounts(m []Mount) {
	for i := 1; i < len(m); i++ {
		for j := i; j > 0 && m[j].Destination < m[j-1].Destination; j-- {
			m[j], m[j-1] = m[j-1], m[j]
		}
	}
}

func hasOpt(opts, o string) bool {
	for _, x := range strings.Split(opts, ",") {
		if x == o {
			return true
		}
	}
	return false
}

// Inspect returns GET /containers/{id}/json; id is an id, a unique id
// prefix or a name. ErrNotFound: no such container.
func (c *Client) Inspect(ctx context.Context, id string) (*Container, error) {
	var out Container
	if err := c.getJSON(ctx, "/containers/"+url.PathEscape(id)+"/json", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Network is the subset of GET /networks/{id} D8 judges.
type Network struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Driver string `json:"Driver"`
	IPAM   struct {
		Config []struct {
			Subnet  string `json:"Subnet"`
			Gateway string `json:"Gateway"`
		} `json:"Config"`
	} `json:"IPAM"`
}

// Network returns GET /networks/{id}. ErrNotFound: no such network.
func (c *Client) Network(ctx context.Context, id string) (*Network, error) {
	var out Network
	if err := c.getJSON(ctx, "/networks/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Within reports whether the slash path p is dir or below it.
func Within(p, dir string) bool {
	p, dir = path.Clean(p), path.Clean(dir)
	return p == dir || dir == "/" || strings.HasPrefix(p, dir+"/")
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
