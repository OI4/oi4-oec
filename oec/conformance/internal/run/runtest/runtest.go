// Package runtest provides the test images of oec-infra run (C-074): image
// documents oec-d8-sim:<fault> of the simulated application for the fake
// Engine API, the hook that runs it in process, and the OCI archive with
// the static binary oec-run-sim for the opt-in run with Docker.
package runtest

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/OI4/oec/conformance/internal/engine/enginetest"
	"github.com/OI4/oec/conformance/internal/oci/ocitest"
	"github.com/OI4/oec/conformance/internal/run/simapp"
)

// Repo is the repository of the test images; Entrypoint is the binary in
// them.
const (
	Repo       = "oec-d8-sim"
	Entrypoint = "/oec-run-sim"
)

// Ref is the image reference of a fault.
func Ref(f simapp.Fault) string { return Repo + ":" + string(f) }

// Images are the image documents of every fault (linux/arch).
func Images(arch string) []enginetest.Doc {
	var out []enginetest.Doc
	for _, f := range simapp.Faults {
		id := ocitest.Digest("sha256", []byte(Ref(f)))
		out = append(out, enginetest.Image(id, []string{Ref(f)}, "linux", arch, []string{Entrypoint}, []string{"--fault", string(f)}))
	}
	return out
}

// Engine returns the recorded engine (Docker 28.4, its networks) with the
// test images, running the simulated application in process.
func Engine(t testing.TB) enginetest.Engine {
	t.Helper()
	rec := enginetest.Recorded(t)
	e := rec.Engine()
	e.Images = Images("arm64")
	e.Run = Run
	return e
}

// Run runs the simulated application of a started container: the paths of
// the container map to the bind sources, a host of ExtraHosts with
// host-gateway resolves to loopback. A command other than the one of the
// test images exits with 127 like a missing executable.
func Run(ctx context.Context, s enginetest.Started) int {
	if len(s.Cmd) == 0 || s.Cmd[0] != Entrypoint {
		return 127
	}
	fs := flag.NewFlagSet(Entrypoint, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fault := fs.String("fault", string(simapp.Good), "")
	if err := fs.Parse(s.Cmd[1:]); err != nil || !simapp.Known(simapp.Fault(*fault)) {
		return 2
	}
	return simapp.Run(ctx, simapp.Options{
		Fault:    simapp.Fault(*fault),
		Hostname: s.Hostname,
		Path:     Mapper(s.Binds),
		Dialer:   Dialer(s.ExtraHosts),
		Retry:    200 * time.Millisecond,
	})
}

// Mapper maps a container path to the host path of the bind with the
// longest destination containing it; a path outside every bind maps to a
// path that does not exist.
func Mapper(binds map[string]string) func(string) string {
	return func(p string) string {
		p = path.Clean(p)
		best := ""
		for dst := range binds {
			if (p == dst || strings.HasPrefix(p, strings.TrimSuffix(dst, "/")+"/")) && len(dst) > len(best) {
				best = dst
			}
		}
		if best == "" {
			return filepath.Join(os.DevNull, "no-such-path")
		}
		return filepath.Join(binds[best], filepath.FromSlash(strings.TrimPrefix(p, best)))
	}
}

// Dialer resolves a host of ExtraHosts with host-gateway to loopback and
// dials an IP address directly; another host name is unknown.
func Dialer(extraHosts []string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	gw := map[string]bool{}
	for _, h := range extraHosts {
		if name, ip, ok := strings.Cut(h, ":"); ok && ip == "host-gateway" {
			gw[name] = true
		}
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		switch {
		case gw[host]:
			host = "127.0.0.1"
		case net.ParseIP(host) != nil:
		default:
			return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
		}
		// The fake host has no broker on the default port: the dial must
		// not reach a broker of the machine that runs the tests.
		if port == "8883" {
			return nil, &net.OpError{Op: "dial", Net: network, Err: syscall.ECONNREFUSED}
		}
		var d net.Dialer
		return d.DialContext(ctx, network, net.JoinHostPort(host, port))
	}
}

// Archive returns an OCI archive for docker load with one image per fault:
// the static binary oec-run-sim (linux/arch) as /oec-run-sim, Cmd --fault
// <fault>, named oec-d8-sim:<fault>.
func Archive(binary []byte, arch string, faults ...simapp.Fault) []byte {
	if len(faults) == 0 {
		faults = simapp.Faults
	}
	layer := ocitest.Layer{Gzip: true, Files: []ocitest.File{{Path: strings.TrimPrefix(Entrypoint, "/"), Data: string(binary), Mode: 0o755}}}
	var imgs []ocitest.Image
	for range faults {
		imgs = append(imgs, ocitest.Image{OS: "linux", Arch: arch, Layers: []ocitest.Layer{layer}})
	}
	return ocitest.Build(ocitest.Spec{
		Images: imgs,
		Config: func(i int, cfg map[string]any) {
			cfg["config"] = map[string]any{"Entrypoint": []any{Entrypoint}, "Cmd": []any{"--fault", string(faults[i])}}
		},
		Index: func(idx map[string]any) {
			for i, d := range idx["manifests"].([]any) {
				d.(map[string]any)["annotations"] = map[string]any{
					"io.containerd.image.name":          "docker.io/library/" + Ref(faults[i]),
					"org.opencontainers.image.ref.name": fmt.Sprint(faults[i]),
				}
			}
		},
		// The classic image store of Docker loads manifest.json (written
		// by docker save next to index.json), the containerd store
		// index.json.
		Layout: func(files map[string][]byte) {
			var idx struct {
				Manifests []struct{ Digest string } `json:"manifests"`
			}
			if json.Unmarshal(files["index.json"], &idx) != nil {
				return
			}
			var docker []map[string]any
			for i, d := range idx.Manifests {
				var m struct {
					Config struct{ Digest string }   `json:"config"`
					Layers []struct{ Digest string } `json:"layers"`
				}
				if json.Unmarshal(files[ocitest.BlobPath(d.Digest)], &m) != nil {
					return
				}
				var layers []string
				for _, l := range m.Layers {
					layers = append(layers, ocitest.BlobPath(l.Digest))
				}
				docker = append(docker, map[string]any{"Config": ocitest.BlobPath(m.Config.Digest), "RepoTags": []string{Ref(faults[i])}, "Layers": layers})
			}
			files["manifest.json"] = ocitest.JSON(docker)
		},
	})
}
