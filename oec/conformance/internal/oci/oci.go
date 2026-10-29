// Package oci reads an OCI image offline for oec-infra image (DESIGN.md §9):
// an OCI image layout (a directory with oci-layout, index.json and
// blobs/<alg>/<hex>) or the same layout as a tar archive (oci-archive, the
// format of `docker save` since Docker 25, `podman save --format
// oci-archive` and `skopeo copy … oci-archive:`). It verifies every blob
// against its descriptor, walks the image index to the platform images and
// applies the layers to a file tree (with whiteouts). Violations of the
// OCI image specification are collected as problems; only an input that is
// no image layout at all is an error. Standard library only (ADR-0025
// amendment C-070).
package oci

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Media types the reader knows.
const (
	MediaIndex       = "application/vnd.oci.image.index.v1+json"
	MediaManifest    = "application/vnd.oci.image.manifest.v1+json"
	MediaConfig      = "application/vnd.oci.image.config.v1+json"
	DockerList       = "application/vnd.docker.distribution.manifest.list.v2+json"
	DockerManifest   = "application/vnd.docker.distribution.manifest.v2+json"
	DockerConfig     = "application/vnd.docker.container.image.v1+json"
	dockerSchema1    = "application/vnd.docker.distribution.manifest.v1+json"
	dockerSchema1JWS = "application/vnd.docker.distribution.manifest.v1+prettyjws"
	attestationKey   = "vnd.docker.reference.type"
)

var layerTypes = map[string]string{ // media type → compression
	"application/vnd.oci.image.layer.v1.tar":                       "",
	"application/vnd.oci.image.layer.v1.tar+gzip":                  "gzip",
	"application/vnd.oci.image.layer.v1.tar+zstd":                  "zstd",
	"application/vnd.oci.image.layer.nondistributable.v1.tar":      "",
	"application/vnd.oci.image.layer.nondistributable.v1.tar+gzip": "gzip",
	"application/vnd.oci.image.layer.nondistributable.v1.tar+zstd": "zstd",
	"application/vnd.docker.image.rootfs.diff.tar":                 "",
	"application/vnd.docker.image.rootfs.diff.tar.gzip":            "gzip",
	"application/vnd.docker.image.rootfs.foreign.diff.tar":         "",
	"application/vnd.docker.image.rootfs.foreign.diff.tar.gzip":    "gzip",
}

// MaxContent is the largest file whose content the tree keeps.
const MaxContent = 64 << 20

// maxDepth bounds nested image indexes.
const maxDepth = 4

// ErrNotLayout is returned for an input that is no OCI image layout.
var ErrNotLayout = errors.New("not an OCI image layout")

// Platform of an image.
type Platform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant,omitempty"`
	OSVersion    string `json:"os.version,omitempty"`
}

// String is os/arch[/variant].
func (p Platform) String() string {
	s := p.OS + "/" + p.Architecture
	if p.Variant != "" {
		s += "/" + p.Variant
	}
	return s
}

// Descriptor of a blob.
type Descriptor struct {
	MediaType    string            `json:"mediaType"`
	Digest       string            `json:"digest"`
	Size         int64             `json:"size"`
	Platform     *Platform         `json:"platform,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	ArtifactType string            `json:"artifactType,omitempty"`
}

// Kinds of problems.
const (
	Structure = "structure" // the layout or a document violates the OCI image specification
	Integrity = "integrity" // a blob does not match its descriptor (size, digest)
)

// Problem is one violation of the OCI image specification.
type Problem struct {
	Kind   string
	Where  string // layout path or descriptor location, e.g. index.json /manifests/0
	Image  string // platform of the image it belongs to, if any
	Detail string
}

// Config is the part of an image config the checks need.
type Config struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant,omitempty"`
	Config       struct {
		Volumes map[string]json.RawMessage `json:"Volumes"`
		User    string                     `json:"User"`
	} `json:"config"`
	RootFS struct {
		Type    string   `json:"type"`
		DiffIDs []string `json:"diff_ids"`
	} `json:"rootfs"`
}

// Manifest is one image manifest reached from index.json.
type Manifest struct {
	Where      string
	Descriptor Descriptor
	MediaType  string
	Platform   Platform // from the config, else from the descriptor
	Config     *Config
	Layers     []Descriptor
	// Runnable: an image of a real platform (not an attestation manifest,
	// not os or architecture "unknown", an image config).
	Runnable bool
}

// Image is what Read found in the layout.
type Image struct {
	Source    string
	Form      string // oci-layout | oci-archive
	Digest    string // digest of the one descriptor of index.json, "" if it lists several
	MediaType string
	// Index: the image is an image index (several manifests in index.json
	// or a referenced index).
	Index      bool
	Manifests  []*Manifest
	Algorithms []string // digest algorithms of all descriptors, sorted
	Problems   []Problem
}

// Runnable returns the runnable platform images.
func (img *Image) Runnable() []*Manifest {
	var out []*Manifest
	for _, m := range img.Manifests {
		if m.Runnable {
			out = append(out, m)
		}
	}
	return out
}

// Layout is an opened image layout.
type Layout struct {
	source string
	form   string
	dir    string
	file   *os.File
	tarIx  map[string]section
	blobs  map[string]*blobResult  // digest → verification
	layers map[string]*layerResult // digest → parsed layer
	keep   func(string) bool
}

type section struct{ off, size int64 }

type blobResult struct {
	data []byte // documents only (small blobs)
	err  string // missing, size or digest mismatch
	kind string
}

type layerResult struct {
	entries     []*Node
	diffID      string
	unsupported string // compression without a decoder (zstd)
	err         string
}

// Open opens a layout directory or archive. keep selects the paths of the
// image file system whose content is kept in memory (nil: none).
func Open(p string, keep func(string) bool) (*Layout, error) {
	st, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	l := &Layout{source: p, blobs: map[string]*blobResult{}, layers: map[string]*layerResult{}, keep: keep}
	if l.keep == nil {
		l.keep = func(string) bool { return false }
	}
	if st.IsDir() {
		l.form, l.dir = "oci-layout", p
	} else {
		l.form = "oci-archive"
		if err := l.indexTar(p); err != nil {
			return nil, err
		}
	}
	_, errLayout := l.stat("oci-layout")
	_, errIndex := l.stat("index.json")
	if errLayout != nil && errIndex != nil {
		hint := ""
		if _, err := l.stat("manifest.json"); err == nil {
			hint = " (a Docker archive without index.json: save it with Docker 25 or later, `podman save --format oci-archive` or `skopeo copy docker-archive:… oci-archive:…`)"
		}
		l.Close()
		return nil, fmt.Errorf("%s: %w: no oci-layout and no index.json%s", p, ErrNotLayout, hint)
	}
	return l, nil
}

// Close releases the archive.
func (l *Layout) Close() {
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(b []byte) (int, error) {
	n, err := c.r.Read(b)
	c.n += int64(n)
	return n, err
}

func (l *Layout) indexTar(p string) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	var magic [2]byte
	if _, err := io.ReadFull(f, magic[:]); err == nil && magic[0] == 0x1f && magic[1] == 0x8b {
		f.Close()
		return fmt.Errorf("%s: %w: a gzip-compressed archive (decompress it first)", p, ErrNotLayout)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return err
	}
	cr := &countingReader{r: f}
	tr := tar.NewReader(cr)
	l.tarIx = map[string]section{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			f.Close()
			return fmt.Errorf("%s: %w: %v", p, ErrNotLayout, err)
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		name := strings.TrimPrefix(path.Clean("/"+h.Name), "/")
		l.tarIx[name] = section{off: cr.n, size: h.Size}
	}
	l.file = f
	return nil
}

func (l *Layout) stat(name string) (int64, error) {
	if l.tarIx != nil {
		s, ok := l.tarIx[name]
		if !ok {
			return 0, os.ErrNotExist
		}
		return s.size, nil
	}
	st, err := os.Stat(filepath.Join(l.dir, filepath.FromSlash(name)))
	if err != nil {
		return 0, err
	}
	if !st.Mode().IsRegular() {
		return 0, os.ErrNotExist
	}
	return st.Size(), nil
}

func (l *Layout) open(name string) (io.ReadCloser, error) {
	if l.tarIx != nil {
		s, ok := l.tarIx[name]
		if !ok {
			return nil, os.ErrNotExist
		}
		return io.NopCloser(io.NewSectionReader(l.file, s.off, s.size)), nil
	}
	return os.Open(filepath.Join(l.dir, filepath.FromSlash(name)))
}

func (l *Layout) readFile(name string, limit int64) ([]byte, error) {
	rc, err := l.open(name)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("larger than %d bytes", limit)
	}
	return data, nil
}

var (
	digestRe = regexp.MustCompile(`^[a-z0-9]+(?:[.+_-][a-z0-9]+)*:[a-zA-Z0-9=_-]+$`)
	encRe    = map[string]*regexp.Regexp{"sha256": regexp.MustCompile(`^[a-f0-9]{64}$`), "sha512": regexp.MustCompile(`^[a-f0-9]{128}$`)}
)

// CheckDigest returns why a digest string is not a verifiable digest of
// the OCI image specification ("" if it is).
func CheckDigest(d string) string {
	if !digestRe.MatchString(d) {
		return fmt.Sprintf("digest %q does not follow the digest grammar", d)
	}
	alg, enc, _ := strings.Cut(d, ":")
	re, ok := encRe[alg]
	if !ok {
		return fmt.Sprintf("digest algorithm %q is not registered (sha256, sha512)", alg)
	}
	if !re.MatchString(enc) {
		return fmt.Sprintf("digest %q: the encoded part is not %d lowercase hex characters", d, map[string]int{"sha256": 64, "sha512": 128}[alg])
	}
	return ""
}

func newHash(alg string) hash.Hash {
	if alg == "sha512" {
		return sha512.New()
	}
	return sha256.New()
}

func blobPath(d string) string {
	alg, enc, _ := strings.Cut(d, ":")
	return "blobs/" + alg + "/" + enc
}

type reader struct {
	l     *Layout
	img   *Image
	algs  map[string]bool
	seenM map[string]*Manifest
}

func (r *reader) problem(kind, where, image, format string, args ...any) {
	r.img.Problems = append(r.img.Problems, Problem{Kind: kind, Where: where, Image: image, Detail: fmt.Sprintf(format, args...)})
}

// Read reads and verifies the image of the layout.
func (l *Layout) Read() (*Image, error) {
	r := &reader{l: l, img: &Image{Source: l.source, Form: l.form}, algs: map[string]bool{}, seenM: map[string]*Manifest{}}
	img := r.img
	if raw, err := l.readFile("oci-layout", 1<<20); err != nil {
		r.problem(Structure, "oci-layout", "", "the file oci-layout is missing")
	} else {
		var ol struct {
			Version *string `json:"imageLayoutVersion"`
		}
		if err := json.Unmarshal(raw, &ol); err != nil || ol.Version == nil {
			r.problem(Structure, "oci-layout", "", "oci-layout is no JSON object with imageLayoutVersion")
		} else if *ol.Version != "1.0.0" {
			r.problem(Structure, "oci-layout", "", "imageLayoutVersion %q, expected \"1.0.0\"", *ol.Version)
		}
	}
	raw, err := l.readFile("index.json", 16<<20)
	if err != nil {
		r.problem(Structure, "index.json", "", "the file index.json is missing or unreadable")
		return img, nil
	}
	idx, ok := r.parseIndex(raw, "index.json", MediaIndex, true)
	if !ok {
		return img, nil
	}
	switch len(idx.Manifests) {
	case 0:
		r.problem(Structure, "index.json", "", "index.json lists no manifest: the layout holds no image")
	case 1:
		d := idx.Manifests[0]
		img.Digest, img.MediaType = d.Digest, d.MediaType
		r.walk(d, "index.json /manifests/0", 0, true)
	default:
		img.Index, img.MediaType = true, MediaIndex
		for i, d := range idx.Manifests {
			r.walk(d, fmt.Sprintf("index.json /manifests/%d", i), 1, false)
		}
	}
	if len(img.Runnable()) == 0 && len(img.Problems) == 0 {
		r.problem(Structure, "index.json", "", "the layout holds no runnable container image (only attestations or artifacts)")
	}
	for a := range r.algs {
		img.Algorithms = append(img.Algorithms, a)
	}
	sort.Strings(img.Algorithms)
	return img, nil
}

type index struct {
	SchemaVersion *int         `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Manifests     []Descriptor `json:"manifests"`
}

func strictJSON(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("trailing data after the JSON value")
	}
	return nil
}

func (r *reader) parseIndex(raw []byte, where, mediaType string, layoutIndex bool) (*index, bool) {
	var idx index
	if err := strictJSON(raw, &idx); err != nil {
		r.problem(Structure, where, "", "not a valid image index: %v", err)
		return nil, false
	}
	if idx.SchemaVersion == nil || *idx.SchemaVersion != 2 {
		r.problem(Structure, where, "", "schemaVersion must be 2")
	}
	if idx.MediaType != "" && idx.MediaType != mediaType && !(layoutIndex && idx.MediaType == MediaIndex) {
		r.problem(Structure, where, "", "mediaType %q differs from the descriptor's %q", idx.MediaType, mediaType)
	}
	if layoutIndex && idx.MediaType != "" && idx.MediaType != MediaIndex {
		r.problem(Structure, where, "", "index.json must be an OCI image index, mediaType %q", idx.MediaType)
	}
	if idx.Manifests == nil {
		r.problem(Structure, where, "", "manifests is missing")
	}
	return &idx, true
}

// blob checks a descriptor and verifies its blob; it returns the content of
// a document (limit > 0).
func (r *reader) blob(d Descriptor, where, image string, limit int64) ([]byte, bool) {
	switch {
	case d.MediaType == "":
		r.problem(Structure, where, image, "the descriptor has no mediaType")
	case d.Size < 0:
		r.problem(Structure, where, image, "the descriptor has a negative size")
	}
	if msg := CheckDigest(d.Digest); msg != "" {
		r.problem(Integrity, where, image, "%s", msg)
		return nil, false
	}
	alg, _, _ := strings.Cut(d.Digest, ":")
	r.algs[alg] = true
	res, ok := r.l.blobs[d.Digest]
	if !ok {
		res = r.l.verify(d, limit)
		r.l.blobs[d.Digest] = res
	}
	if res.err != "" {
		r.problem(res.kind, where, image, "blob %s: %s", d.Digest, res.err)
		return nil, false
	}
	if size := int64(len(res.data)); limit > 0 && size != d.Size {
		r.problem(Integrity, where, image, "blob %s has %d bytes, the descriptor says %d", d.Digest, size, d.Size)
		return nil, false
	}
	return res.data, true
}

func (l *Layout) verify(d Descriptor, limit int64) *blobResult {
	p := blobPath(d.Digest)
	size, err := l.stat(p)
	if err != nil {
		return &blobResult{kind: Structure, err: "missing in the layout (" + p + ")"}
	}
	if size != d.Size {
		return &blobResult{kind: Integrity, err: fmt.Sprintf("has %d bytes, the descriptor says %d", size, d.Size)}
	}
	if limit > 0 && size > limit {
		return &blobResult{kind: Structure, err: fmt.Sprintf("a document of %d bytes is too large to read", size)}
	}
	rc, err := l.open(p)
	if err != nil {
		return &blobResult{kind: Structure, err: err.Error()}
	}
	defer rc.Close()
	alg, enc, _ := strings.Cut(d.Digest, ":")
	h := newHash(alg)
	var buf bytes.Buffer
	w := io.Writer(h)
	if limit > 0 {
		w = io.MultiWriter(h, &buf)
	}
	if _, err := io.Copy(w, rc); err != nil {
		return &blobResult{kind: Structure, err: err.Error()}
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != enc {
		return &blobResult{kind: Integrity, err: fmt.Sprintf("content does not match the digest (%s:%s)", alg, got)}
	}
	res := &blobResult{}
	if limit > 0 {
		res.data = buf.Bytes()
	}
	return res
}

func (r *reader) walk(d Descriptor, where string, depth int, top bool) {
	switch d.MediaType {
	case MediaIndex, DockerList:
		if top {
			r.img.Index = true
		}
		if depth >= maxDepth {
			r.problem(Structure, where, "", "image indexes nested deeper than %d levels", maxDepth)
			return
		}
		raw, ok := r.blob(d, where, "", 16<<20)
		if !ok {
			return
		}
		idx, ok := r.parseIndex(raw, where+" → "+d.Digest, d.MediaType, false)
		if !ok {
			return
		}
		for i, c := range idx.Manifests {
			r.walk(c, fmt.Sprintf("%s → /manifests/%d", where, i), depth+1, false)
		}
	case MediaManifest, DockerManifest:
		r.manifest(d, where)
	case dockerSchema1, dockerSchema1JWS:
		r.problem(Structure, where, "", "Docker image manifest schema 1 (%s) is not OCI-compatible", d.MediaType)
	default:
		if top || d.MediaType == "" {
			r.problem(Structure, where, "", "unknown manifest media type %q", d.MediaType)
			return
		}
		r.blob(d, where, "", 0) // an artifact or a foreign document: verified, not judged
	}
}

type manifest struct {
	SchemaVersion *int         `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Config        *Descriptor  `json:"config"`
	Layers        []Descriptor `json:"layers"`
}

func (r *reader) manifest(d Descriptor, where string) {
	if m, ok := r.seenM[d.Digest]; ok {
		r.img.Manifests = append(r.img.Manifests, m)
		return
	}
	plat := ""
	if d.Platform != nil {
		plat = d.Platform.String()
	}
	m := &Manifest{Where: where, Descriptor: d, MediaType: d.MediaType}
	if d.Platform != nil {
		m.Platform = *d.Platform
	}
	r.seenM[d.Digest] = m
	r.img.Manifests = append(r.img.Manifests, m)
	attestation := d.Annotations[attestationKey] == "attestation-manifest"
	raw, ok := r.blob(d, where, plat, 16<<20)
	if !ok {
		return
	}
	at := where + " → " + d.Digest
	var mf manifest
	if err := strictJSON(raw, &mf); err != nil {
		r.problem(Structure, at, plat, "not a valid image manifest: %v", err)
		return
	}
	if mf.SchemaVersion == nil || *mf.SchemaVersion != 2 {
		r.problem(Structure, at, plat, "schemaVersion must be 2")
	}
	if mf.MediaType != "" && mf.MediaType != d.MediaType {
		r.problem(Structure, at, plat, "mediaType %q differs from the descriptor's %q", mf.MediaType, d.MediaType)
	}
	if d.MediaType == DockerManifest && mf.MediaType == "" {
		r.problem(Structure, at, plat, "a Docker v2 manifest must carry its mediaType")
	}
	if mf.Config == nil {
		r.problem(Structure, at, plat, "config is missing")
		return
	}
	m.Layers = mf.Layers
	if mf.Config.MediaType != MediaConfig && mf.Config.MediaType != DockerConfig {
		// An artifact manifest (OCI 1.1): verified, not an image.
		r.blob(*mf.Config, at+" /config", plat, 0)
		if !attestation && d.Platform == nil {
			r.problem(Structure, at+" /config", plat, "config mediaType %q is not an image config: the manifest is no container image", mf.Config.MediaType)
		}
		return
	}
	craw, ok := r.blob(*mf.Config, at+" /config", plat, 16<<20)
	if !ok {
		return
	}
	var cfg Config
	if err := strictJSON(craw, &cfg); err != nil {
		r.problem(Structure, at+" /config", plat, "not a valid image config: %v", err)
		return
	}
	m.Config = &cfg
	if cfg.OS == "" || cfg.Architecture == "" {
		r.problem(Structure, at+" /config", plat, "the image config must name os and architecture")
	}
	cp := Platform{Architecture: cfg.Architecture, OS: cfg.OS, Variant: cfg.Variant}
	if d.Platform != nil && (d.Platform.OS != cp.OS || d.Platform.Architecture != cp.Architecture || d.Platform.Variant != "" && cfg.Variant != "" && d.Platform.Variant != cfg.Variant) {
		r.problem(Structure, where+" /platform", plat, "the platform %s of the descriptor differs from the image config (%s)", d.Platform, cp)
	}
	if d.Platform == nil || d.Platform.Variant == "" {
		m.Platform = cp
	} else {
		m.Platform = Platform{Architecture: cp.Architecture, OS: cp.OS, Variant: d.Platform.Variant}
	}
	plat = m.Platform.String()
	m.Runnable = !attestation && cp.OS != "unknown" && cp.Architecture != "unknown" && cp.OS != "" && cp.Architecture != ""
	if cfg.RootFS.Type != "layers" {
		r.problem(Structure, at+" /config", plat, "rootfs.type must be \"layers\"")
	}
	if len(cfg.RootFS.DiffIDs) != len(mf.Layers) {
		r.problem(Structure, at+" /config", plat, "rootfs.diff_ids has %d entries for %d layers", len(cfg.RootFS.DiffIDs), len(mf.Layers))
	}
	if len(mf.Layers) == 0 && m.Runnable {
		r.problem(Structure, at, plat, "the image has no layer")
	}
	for i, ld := range mf.Layers {
		lw := fmt.Sprintf("%s /layers/%d", at, i)
		comp, known := layerTypes[ld.MediaType]
		if !known {
			r.problem(Structure, lw, plat, "unknown layer media type %q", ld.MediaType)
			continue
		}
		if _, ok := r.blob(ld, lw, plat, 0); !ok {
			continue
		}
		lr := r.l.layer(ld, comp)
		if lr.err != "" {
			r.problem(Structure, lw, plat, "layer %s: %s", ld.Digest, lr.err)
			continue
		}
		if lr.unsupported == "" && i < len(cfg.RootFS.DiffIDs) && cfg.RootFS.DiffIDs[i] != lr.diffID {
			r.problem(Integrity, lw, plat, "the uncompressed layer has the digest %s, rootfs.diff_ids/%d says %s", lr.diffID, i, cfg.RootFS.DiffIDs[i])
		}
	}
}

// layer reads a verified layer once: diff_id and the entries of its tar.
func (l *Layout) layer(d Descriptor, comp string) *layerResult {
	if lr, ok := l.layers[d.Digest]; ok {
		return lr
	}
	lr := &layerResult{}
	l.layers[d.Digest] = lr
	if comp == "zstd" {
		lr.unsupported = "zstd-compressed layers are not read (no decoder in the Go standard library)"
		return lr
	}
	rc, err := l.open(blobPath(d.Digest))
	if err != nil {
		lr.err = err.Error()
		return lr
	}
	defer rc.Close()
	var src io.Reader = bufio.NewReader(rc)
	if comp == "gzip" {
		zr, err := gzip.NewReader(src)
		if err != nil {
			lr.err = "not gzip-compressed: " + err.Error()
			return lr
		}
		defer zr.Close()
		src = zr
	}
	h := sha256.New()
	tr := tar.NewReader(io.TeeReader(src, h))
	for {
		hd, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			lr.err = "not a tar archive: " + err.Error()
			return lr
		}
		n := &Node{Path: clean(hd.Name), Type: hd.Typeflag, Size: hd.Size, Mode: hd.Mode}
		switch hd.Typeflag {
		case tar.TypeSymlink:
			n.Link = hd.Linkname
		case tar.TypeLink:
			n.Link = clean(hd.Linkname)
		case tar.TypeReg, tar.TypeRegA:
			n.Type = tar.TypeReg
			if l.keep(n.Path) && hd.Size <= MaxContent {
				data, err := io.ReadAll(tr)
				if err != nil {
					lr.err = "not a tar archive: " + err.Error()
					return lr
				}
				n.Data, n.Kept = data, true
			}
		}
		lr.entries = append(lr.entries, n)
	}
	if _, err := io.Copy(io.Discard, io.TeeReader(src, h)); err != nil {
		lr.err = "truncated: " + err.Error()
		return lr
	}
	lr.diffID = "sha256:" + hex.EncodeToString(h.Sum(nil))
	return lr
}

func clean(name string) string { return path.Clean("/" + name) }
