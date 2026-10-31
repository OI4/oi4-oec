package host

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/OI4/oec/conformance/internal/container"
	"github.com/OI4/oec/conformance/internal/engine"
	"github.com/OI4/oec/conformance/internal/report"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// The storages of a container T8-HST-03 judges (RIG 4.1.3, 4.1.4).
const (
	ContainerSecrets = "/run/secrets/oi4"
	ContainerAppEtc  = "/etc/oi4/app"
	ContainerAppOpt  = "/opt/oi4/app"
)

const (
	reqAppStorage = "REQ-CNT-011"
	reqOwnSecrets = "REQ-CNT-024"
)

// oi4Dirs: a container with a mount on or below one of them is an OI4
// container (an application of RIG 4.1).
var oi4Dirs = []string{"/etc/oi4", "/opt/oi4", ContainerSecrets}

// piece is the part of a host-side storage a container mounts on or below
// a storage path: the mount (Source: the effective source, for a covering
// mount the source joined with the rest of the path).
type piece struct {
	m   engine.Mount
	dst string // the destination of the piece in the container
}

// shareable reports whether another container can reach the piece through
// a mount: binds and volumes (tmpfs, image and the other types are not
// shared between containers).
func shareable(m engine.Mount) bool {
	return (m.Type == "bind" || m.Type == "volume") && (m.Source != "" || m.Name != "")
}

// overlaps reports whether two mounts reach the same host-side data: the
// same named volume or nested sources (as the engine reports them).
func overlaps(a, b engine.Mount) bool {
	if !shareable(a) || !shareable(b) {
		return false
	}
	if a.Type == "volume" && b.Type == "volume" && a.Name != "" && a.Name == b.Name {
		return true
	}
	if a.Source == "" || b.Source == "" {
		return false
	}
	return engine.Within(a.Source, b.Source) || engine.Within(b.Source, a.Source)
}

// pieces returns the mounts of the container that make up the storage at
// p: the covering mount (the one with the longest destination that
// contains p) and the mounts below p.
func pieces(c *engine.Container, p string) []piece {
	var out []piece
	var cover *engine.Mount
	for _, m := range c.AllMounts() {
		dst := path.Clean(m.Destination)
		switch {
		case dst != p && engine.Within(dst, p):
			out = append(out, piece{m: m, dst: dst})
		case engine.Within(p, dst):
			if cover == nil || len(dst) > len(path.Clean(cover.Destination)) {
				mm := m
				cover = &mm
			}
		}
	}
	if cover != nil {
		m := *cover
		if rest := strings.TrimPrefix(p, path.Clean(m.Destination)); rest != "" && m.Source != "" {
			// Only the part below the destination: compared by source.
			m.Source, m.Name = path.Join(m.Source, rest), ""
		}
		out = append([]piece{{m: m, dst: p}}, out...)
	}
	return out
}

// isOI4 reports whether the container mounts a storage of RIG 4.1: a
// mount on or below /etc/oi4, /opt/oi4 or /run/secrets/oi4, or a mount
// above one of them (e.g. on /run/secrets).
func isOI4(c *engine.Container) bool {
	for _, m := range c.AllMounts() {
		dst := path.Clean(m.Destination)
		for _, d := range oi4Dirs {
			if engine.Within(dst, d) || dst != "/" && engine.Within(d, dst) {
				return true
			}
		}
	}
	return false
}

// inspected is a container of the engine.
type inspected struct {
	c    *engine.Container
	name string
}

// storages judges T8-HST-03: the application storages (/etc/oi4/app,
// /opt/oi4/app, REQ-CNT-011) and the secret storage (REQ-CNT-024) of every
// OI4 container of the engine – running or not – are mapped from the host
// and no other container mounts them. The container D8 runs in (--self)
// is not judged and not compared.
func (r *runner) storages(ctx context.Context) {
	reqs := []string{reqAppStorage, reqOwnSecrets}
	cl, tried, err := engine.Connect(ctx, r.c.Daemon, r.c.Getenv, r.facts.Daemons, r.c.Timeout)
	if err != nil {
		r.info.Engine = &report.Engine{Endpoint: strings.Join(tried, ", ")}
		r.inconclusiveC(reqs, "", "no Docker/Podman Engine API (%s): %v", r.info.Engine.Endpoint, err)
		return
	}
	r.info.Engine = &report.Engine{Endpoint: cl.Endpoint()}
	if v, err := cl.Version(ctx); err == nil {
		e := r.info.Engine
		e.Product, e.Version, e.API, e.OS, e.Arch = v.Product(), v.Version, v.APIVersion, v.Os, v.Arch
	}
	list, err := cl.Containers(ctx, true)
	if err != nil {
		r.inconclusiveC(reqs, "", "container list: %v", err)
		return
	}
	var all []inspected
	selfSeen := r.c.Self == ""
	for _, s := range list {
		c, err := cl.Inspect(ctx, s.ID)
		switch {
		case errors.Is(err, engine.ErrNotFound):
			continue // removed meanwhile
		case err != nil:
			r.inconclusiveC(reqs, s.Name(), "inspect: %v", err)
			return
		}
		if r.isSelf(c) {
			selfSeen = true
			continue
		}
		all = append(all, inspected{c: c, name: c.ContainerName()})
	}
	if !selfSeen {
		r.inconclusiveC(reqs, r.c.Self, "--self %s: no such container of the engine %s", r.c.Self, cl.Endpoint())
		return
	}
	var oi4 []inspected
	for _, x := range all {
		if isOI4(x.c) {
			oi4 = append(oi4, x)
			r.info.Containers = append(r.info.Containers, container.Describe(x.c))
		}
	}
	if len(oi4) == 0 {
		r.skippedC(reqs, "no container of the engine %s mounts a storage of RIG 4.1 (/etc/oi4, /opt/oi4, /run/secrets/oi4)", cl.Endpoint())
		return
	}
	r.secretStorages(oi4, all)
	r.appStorages(oi4, all)
}

// isSelf reports whether the container is the one of --self (name, id or
// unique id prefix).
func (r *runner) isSelf(c *engine.Container) bool {
	s := strings.TrimPrefix(r.c.Self, "/")
	if s == "" {
		return false
	}
	return s == c.ContainerName() || (len(s) >= 3 && strings.HasPrefix(c.ID, s))
}

// sharedWith returns "container:destination" of every mount of another
// container that reaches the piece (stopped containers are marked).
func sharedWith(p piece, self *engine.Container, all []inspected) []string {
	var out []string
	if !shareable(p.m) {
		return nil
	}
	for _, o := range all {
		if o.c.ID == self.ID {
			continue
		}
		for _, m := range o.c.AllMounts() {
			if overlaps(p.m, m) {
				s := o.name + ":" + m.Destination
				if !o.c.State.Running {
					s += " (" + state(o.c) + ")"
				}
				out = append(out, s)
			}
		}
	}
	return out
}

func state(c *engine.Container) string {
	if c.State.Status != "" {
		return c.State.Status
	}
	return "not running"
}

// secretStorages judges REQ-CNT-024: no other container mounts the secret
// storage of an OI4 container.
func (r *runner) secretStorages(oi4, all []inspected) {
	judged := false
	for _, x := range oi4 {
		ps := pieces(x.c, ContainerSecrets)
		if len(ps) == 0 {
			continue
		}
		judged = true
		bad := false
		for _, p := range ps {
			if others := sharedWith(p, x.c, all); len(others) > 0 {
				bad = true
				r.addC(reqOwnSecrets, verdict.Violation, x.name, p.dst, "the %s mount on %s is also mounted by %s; every container has its own secret storage (RIG 4.1.3)", p.m.Type, p.dst, strings.Join(others, ", "))
			}
		}
		if !bad {
			r.addC(reqOwnSecrets, verdict.Checked, x.name, ContainerSecrets, "the secret storage (%s) is mounted by no other container", kinds(ps))
		}
	}
	if !judged {
		r.skippedC([]string{reqOwnSecrets}, "no OI4 container mounts a secret storage on %s", ContainerSecrets)
	}
}

// appStorages judges REQ-CNT-011: if the platform provides a persistent
// layer (an OI4 container has a bind or volume on or below an application
// storage), every OI4 container has its application storages mapped from
// the host and no other container mounts them.
func (r *runner) appStorages(oi4, all []inspected) {
	persistent := false
	for _, x := range oi4 {
		for _, p := range []string{ContainerAppEtc, ContainerAppOpt} {
			for _, pc := range pieces(x.c, p) {
				persistent = persistent || shareable(pc.m)
			}
		}
	}
	if !persistent {
		r.skippedC([]string{reqAppStorage}, "no OI4 container has a host-defined path (bind or volume) on %s or %s; the platform provides no persistent layer (RIG 4.1.4)", ContainerAppEtc, ContainerAppOpt)
		return
	}
	for _, x := range oi4 {
		for _, p := range []string{ContainerAppEtc, ContainerAppOpt} {
			ps := pieces(x.c, p)
			if len(ps) == 0 {
				r.addC(reqAppStorage, verdict.Reported, x.name, p, "no mount on %s; the application data stays in the container layer – whether the application uses the storage is judged manually (RIG 4.1.4)", p)
				continue
			}
			bad := false
			for _, pc := range ps {
				if !shareable(pc.m) {
					bad = true
					r.addC(reqAppStorage, verdict.Violation, x.name, pc.dst, "%s is no host-defined path but a mount of type %s (RIG 4.1.4)", pc.dst, pc.m.Type)
					continue
				}
				if others := sharedWith(pc, x.c, all); len(others) > 0 {
					bad = true
					r.addC(reqAppStorage, verdict.Violation, x.name, pc.dst, "the %s mount on %s is also mounted by %s; only one container has access to its application specific volume (RIG 4.1.4)", pc.m.Type, pc.dst, strings.Join(others, ", "))
				}
			}
			if !bad {
				r.addC(reqAppStorage, verdict.Checked, x.name, p, "%s is mapped from the host (%s) and mounted by no other container", p, kinds(ps))
			}
		}
	}
}

// kinds names the mount types of the pieces.
func kinds(ps []piece) string {
	var out []string
	for _, p := range ps {
		out = append(out, p.m.Type)
	}
	return strings.Join(out, ", ")
}

func (r *runner) addC(req string, k verdict.Kind, ctr, p, format string, a ...any) {
	r.obs = append(r.obs, verdict.Observation{Test: TStorages, Req: req, Kind: k, Evidence: verdict.Evidence{Container: ctr, Path: p, Detail: fmt.Sprintf(format, a...)}})
}

func (r *runner) inconclusiveC(reqs []string, ctr, format string, a ...any) {
	for _, req := range reqs {
		r.obs = append(r.obs, verdict.Observation{Test: TStorages, Req: req, Kind: verdict.Skipped, Inconclusive: true, Evidence: verdict.Evidence{Container: ctr, Detail: fmt.Sprintf(format, a...)}})
	}
}

func (r *runner) skippedC(reqs []string, format string, a ...any) {
	for _, req := range reqs {
		r.obs = append(r.obs, verdict.Observation{Test: TStorages, Req: req, Kind: verdict.Skipped, Evidence: verdict.Evidence{Detail: fmt.Sprintf(format, a...)}})
	}
}
