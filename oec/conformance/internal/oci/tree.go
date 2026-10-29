package oci

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

// Node is one entry of an image file system.
type Node struct {
	Path string
	Type byte // tar type flag (TypeReg, TypeDir, TypeSymlink, TypeLink, …)
	Size int64
	Mode int64
	Link string // symlink target as written, or the absolute hard link target
	Data []byte
	Kept bool // Data holds the content
	// Layer is the index of the layer that wrote the entry (-1: implied
	// parent directory).
	Layer int
	desc  Descriptor
	comp  string
}

// IsDir reports whether the node is a directory.
func (n *Node) IsDir() bool { return n.Type == tar.TypeDir }

// Tree is the file system of a platform image: its layers applied in order
// (OCI image specification, "Applying Changesets", with whiteouts).
type Tree struct {
	nodes map[string]*Node
	l     *Layout
	// Incomplete names why the tree misses content (a layer that is
	// unreadable, missing or zstd-compressed); "" if it is complete.
	Incomplete string
}

// Errors of Tree lookups.
var (
	ErrNotExist = errors.New("does not exist")
	ErrLoop     = errors.New("too many levels of symbolic links")
	ErrNotDir   = errors.New("not a directory")
)

// maxHops is the symlink limit of Linux (MAXSYMLINKS).
const maxHops = 40

// Tree applies the layers of a platform image.
func (l *Layout) Tree(m *Manifest) *Tree {
	t := &Tree{nodes: map[string]*Node{"/": {Path: "/", Type: tar.TypeDir, Layer: -1}}, l: l}
	for i, ld := range m.Layers {
		comp, known := layerTypes[ld.MediaType]
		res := l.blobs[ld.Digest]
		lr := l.layers[ld.Digest]
		switch {
		case !known || res == nil || res.err != "" || lr == nil:
			t.Incomplete = fmt.Sprintf("layer %d (%s) is unknown, missing or does not match its descriptor", i, ld.Digest)
			return t
		case lr.err != "":
			t.Incomplete = fmt.Sprintf("layer %d (%s) is unreadable: %s", i, ld.Digest, lr.err)
			return t
		case lr.unsupported != "":
			t.Incomplete = fmt.Sprintf("layer %d (%s): %s", i, ld.Digest, lr.unsupported)
			return t
		}
		t.apply(i, ld, comp, lr.entries)
	}
	return t
}

func (t *Tree) removeBelow(dir string) {
	prefix := strings.TrimSuffix(dir, "/") + "/"
	for p := range t.nodes {
		if strings.HasPrefix(p, prefix) {
			delete(t.nodes, p)
		}
	}
}

func (t *Tree) apply(layer int, d Descriptor, comp string, entries []*Node) {
	// Whiteouts hide entries of the lower layers only.
	for _, e := range entries {
		dir, base := path.Split(e.Path)
		dir = path.Clean(dir)
		switch {
		case base == ".wh..wh..opq":
			t.removeBelow(dir)
		case strings.HasPrefix(base, ".wh."):
			p := path.Join(dir, strings.TrimPrefix(base, ".wh."))
			delete(t.nodes, p)
			t.removeBelow(p)
		}
	}
	for _, e := range entries {
		base := path.Base(e.Path)
		if strings.HasPrefix(base, ".wh.") || e.Path == "/" {
			continue
		}
		for dir := path.Dir(e.Path); dir != "/"; dir = path.Dir(dir) {
			if p, ok := t.nodes[dir]; !ok || !p.IsDir() {
				if ok {
					t.removeBelow(dir)
				}
				t.nodes[dir] = &Node{Path: dir, Type: tar.TypeDir, Layer: -1}
			}
		}
		if old, ok := t.nodes[e.Path]; ok && old.IsDir() && !e.IsDir() {
			t.removeBelow(e.Path)
		}
		n := *e
		n.Layer, n.desc, n.comp = layer, d, comp
		t.nodes[e.Path] = &n
	}
}

// Lstat returns the node at p without following a final symlink; the
// parent directories are resolved.
func (t *Tree) Lstat(p string) (*Node, error) {
	p = clean(p)
	if p == "/" {
		return t.nodes["/"], nil
	}
	dir, err := t.resolve(path.Dir(p), true, new(int))
	if err != nil {
		return nil, err
	}
	n, ok := t.nodes[path.Join(dir, path.Base(p))]
	if !ok {
		return nil, fmt.Errorf("%s: %w", p, ErrNotExist)
	}
	return n, nil
}

// Resolve returns the real path of p, following every symlink (as the
// kernel does inside the container: absolute targets start at the image
// root, ".." stops at the root).
func (t *Tree) Resolve(p string) (string, error) { return t.resolve(clean(p), true, new(int)) }

func (t *Tree) resolve(p string, follow bool, hops *int) (string, error) {
	cur := "/"
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	for i, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			cur = path.Dir(cur)
			continue
		}
		next := path.Join(cur, part)
		n, ok := t.nodes[next]
		if !ok {
			return "", fmt.Errorf("%s: %w", next, ErrNotExist)
		}
		last := i == len(parts)-1
		if n.Type == tar.TypeSymlink && (follow || !last) {
			*hops++
			if *hops > maxHops {
				return "", fmt.Errorf("%s: %w", next, ErrLoop)
			}
			target := n.Link
			if !strings.HasPrefix(target, "/") {
				target = path.Join(cur, target)
			}
			rest := strings.Join(parts[i+1:], "/")
			r, err := t.resolve(clean(path.Join(target, rest)), follow, hops)
			if err != nil {
				return "", err
			}
			return r, nil
		}
		if !last && !n.IsDir() {
			return "", fmt.Errorf("%s: %w", next, ErrNotDir)
		}
		cur = next
	}
	return cur, nil
}

// Stat returns the node p resolves to.
func (t *Tree) Stat(p string) (*Node, error) {
	r, err := t.Resolve(p)
	if err != nil {
		return nil, err
	}
	return t.nodes[r], nil
}

// List returns the entries directly inside the directory p resolves to,
// sorted by name.
func (t *Tree) List(p string) ([]*Node, error) {
	r, err := t.Resolve(p)
	if err != nil {
		return nil, err
	}
	if n := t.nodes[r]; !n.IsDir() {
		return nil, fmt.Errorf("%s: %w", p, ErrNotDir)
	}
	prefix := strings.TrimSuffix(r, "/") + "/"
	var out []*Node
	for q, n := range t.nodes {
		if strings.HasPrefix(q, prefix) && !strings.Contains(q[len(prefix):], "/") {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// Walk returns every node at or below the path p (not resolved), sorted.
func (t *Tree) Walk(p string) []*Node {
	p = clean(p)
	prefix := strings.TrimSuffix(p, "/") + "/"
	var out []*Node
	for q, n := range t.nodes {
		if q == p || strings.HasPrefix(q, prefix) {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// ReadFile returns the content of the regular file p resolves to (a hard
// link is read through its target).
func (t *Tree) ReadFile(p string) ([]byte, error) {
	n, err := t.Stat(p)
	if err != nil {
		return nil, err
	}
	for hops := 0; n.Type == tar.TypeLink; hops++ {
		target, ok := t.nodes[n.Link]
		if !ok || hops > maxHops {
			return nil, fmt.Errorf("%s: hard link to %s: %w", p, n.Link, ErrNotExist)
		}
		n = target
	}
	if n.Type != tar.TypeReg {
		return nil, fmt.Errorf("%s: not a regular file", p)
	}
	if n.Kept {
		return n.Data, nil
	}
	if n.Size > MaxContent {
		return nil, fmt.Errorf("%s: larger than %d bytes", p, MaxContent)
	}
	return t.l.content(n)
}

// content re-reads one entry from its layer.
func (l *Layout) content(n *Node) ([]byte, error) {
	rc, err := l.open(blobPath(n.desc.Digest))
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var src io.Reader = bufio.NewReader(rc)
	if n.comp == "gzip" {
		zr, err := gzip.NewReader(src)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		src = zr
	}
	tr := tar.NewReader(src)
	var data []byte
	found := false
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if clean(h.Name) == n.Path { // the last entry of a path wins
			if data, err = io.ReadAll(io.LimitReader(tr, MaxContent)); err != nil {
				return nil, err
			}
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("%s: not found in layer %d", n.Path, n.Layer)
	}
	return data, nil
}
