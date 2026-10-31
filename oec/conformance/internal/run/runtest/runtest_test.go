package runtest

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/OI4/oec/conformance/internal/engine/enginetest"
	"github.com/OI4/oec/conformance/internal/oci"
	"github.com/OI4/oec/conformance/internal/run/simapp"
)

func tarFiles(t *testing.T, p string) map[string][]byte {
	t.Helper()
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := map[string][]byte{}
	r := tar.NewReader(f)
	for {
		h, err := r.Next()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r)
		out[h.Name] = b
	}
}

func TestMapper(t *testing.T) {
	m := Mapper(map[string]string{"/etc/oi4": "/h/etc", "/etc/oi4/mqtt": "/h/mqtt"})
	for p, want := range map[string]string{
		"/etc/oi4/mqtt/broker.json": filepath.Join("/h/mqtt", "broker.json"),
		"/etc/oi4/certs/a.pem":      filepath.Join("/h/etc", "certs", "a.pem"),
		"/etc/oi4":                  filepath.Clean("/h/etc"),
		"/etc/oi4x/a":               filepath.Join(os.DevNull, "no-such-path"),
	} {
		if got := m(p); got != want {
			t.Errorf("%s: %s, want %s", p, got, want)
		}
	}
}

func TestDialer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	d := Dialer([]string{"broker:host-gateway", "other:10.0.0.1"})
	for _, addr := range []string{"broker:" + port, "127.0.0.1:" + port} {
		c, err := d(context.Background(), "tcp", addr)
		if err != nil {
			t.Errorf("%s: %v", addr, err)
			continue
		}
		c.Close()
	}
	var dns *net.DNSError
	if _, err := d(context.Background(), "tcp", "other:"+port); !errors.As(err, &dns) {
		t.Errorf("other: %v", err)
	}
	if _, err := d(context.Background(), "tcp", "broker:8883"); !errors.Is(err, syscall.ECONNREFUSED) {
		t.Errorf("8883: %v", err)
	}
}

func TestRunOtherCommand(t *testing.T) {
	if Run(context.Background(), enginetest.Started{Cmd: []string{"/bin/sh"}}) != 127 {
		t.Error("a foreign command must exit with 127")
	}
}

// TestArchive: the archive for docker load is a valid OCI layout with one
// runnable linux image per fault.
func TestArchive(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sim.tar")
	if err := os.WriteFile(p, Archive([]byte("#!binary"), "arm64", simapp.Good, simapp.Exit), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := oci.Open(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	img, err := l.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(img.Problems) != 0 || len(img.Runnable()) != 2 {
		t.Fatalf("problems %+v, %d runnable", img.Problems, len(img.Runnable()))
	}
	for _, m := range img.Runnable() {
		if m.Platform.OS != "linux" || m.Platform.Architecture != "arm64" {
			t.Errorf("platform %+v", m.Platform)
		}
	}
	// The classic image store of Docker loads by manifest.json.
	var docker []struct {
		Config   string
		RepoTags []string
		Layers   []string
	}
	files := tarFiles(t, p)
	if err := json.Unmarshal(files["manifest.json"], &docker); err != nil || len(docker) != 2 {
		t.Fatalf("manifest.json %v: %s", err, files["manifest.json"])
	}
	for i, f := range []simapp.Fault{simapp.Good, simapp.Exit} {
		d := docker[i]
		if len(d.RepoTags) != 1 || d.RepoTags[0] != Ref(f) || len(d.Layers) != 1 || files[d.Config] == nil || files[d.Layers[0]] == nil {
			t.Errorf("manifest.json %d: %+v", i, d)
		}
	}
	if n := len(Images("amd64")); n != len(simapp.Faults) {
		t.Errorf("%d images", n)
	}
}
