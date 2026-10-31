// Package ocitest builds small, deterministic OCI image archives for the
// self-test of oec-infra image (DESIGN.md §9, §10): an OCI image layout
// (oci-layout, index.json, blobs/<alg>/<hex>) written as one tar archive.
// A Spec describes the images; hooks change the generated documents and
// blobs to inject the faults of the negative fixtures.
package ocitest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"path"
	"sort"
	"strings"
	"time"
)

// Media types.
const (
	MediaIndex      = "application/vnd.oci.image.index.v1+json"
	MediaManifest   = "application/vnd.oci.image.manifest.v1+json"
	MediaConfig     = "application/vnd.oci.image.config.v1+json"
	MediaLayer      = "application/vnd.oci.image.layer.v1.tar"
	MediaLayerGzip  = "application/vnd.oci.image.layer.v1.tar+gzip"
	DockerManifest  = "application/vnd.docker.distribution.manifest.v2+json"
	DockerList      = "application/vnd.docker.distribution.manifest.list.v2+json"
	DockerConfig    = "application/vnd.docker.container.image.v1+json"
	DockerLayerGzip = "application/vnd.docker.image.rootfs.diff.tar.gzip"
	// AttestationKey is the annotation of a buildx attestation manifest.
	AttestationKey = "vnd.docker.reference.type"
)

// Epoch is the modification time of every generated tar entry.
var Epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// File is one entry of a layer. Type is a tar type flag (default regular
// file); Link is the target of a symlink or hard link; Mode overrides the
// default permission bits (0644 file, 0755 directory), e.g. 0755 for an
// executable.
type File struct {
	Path string
	Data string
	Type byte
	Link string
	Mode int64
}

// Dir returns a directory entry.
func Dir(p string) File { return File{Path: p, Type: tar.TypeDir} }

// Symlink returns a symbolic link entry.
func Symlink(p, target string) File { return File{Path: p, Type: tar.TypeSymlink, Link: target} }

// Whiteout returns the whiteout entry that deletes p of a lower layer.
func Whiteout(p string) File { return File{Path: path.Join(path.Dir(p), ".wh."+path.Base(p))} }

// Layer is one layer of an image.
type Layer struct {
	Files []File
	Gzip  bool
}

// Image is one platform image.
type Image struct {
	OS, Arch, Variant string
	Layers            []Layer
	Volumes           []string
	// Attestation makes a buildx attestation manifest (platform
	// unknown/unknown, not runnable).
	Attestation bool
}

// Spec describes an image archive.
type Spec struct {
	Images []Image
	// Multi wraps the images in an image index referenced from index.json;
	// otherwise index.json lists the image manifests directly.
	Multi bool
	// Docker uses the Docker v2 schema 2 media types.
	Docker bool
	// Algorithm of the digests: sha256 (default) or sha512.
	Algorithm string
	// Hooks, applied in this order.
	Config   func(i int, cfg map[string]any)      // the image config of image i
	Manifest func(i int, m map[string]any)        // the manifest of image i
	Platform func(i int, platform map[string]any) // the platform of image i in its descriptor
	Index    func(idx map[string]any)             // the image index (Multi) or index.json
	Blobs    func(blobs map[string][]byte)        // the blobs by layout path
	Layout   func(files map[string][]byte)        // every file of the layout
}

type builder struct {
	alg   string
	blobs map[string][]byte
}

func (b *builder) add(mediaType string, data []byte) map[string]any {
	d := Digest(b.alg, data)
	b.blobs[BlobPath(d)] = data
	return map[string]any{"mediaType": mediaType, "digest": d, "size": len(data)}
}

// Digest returns the digest of data with the algorithm (sha256 or sha512).
func Digest(alg string, data []byte) string {
	if alg == "sha512" {
		s := sha512.Sum512(data)
		return "sha512:" + hex.EncodeToString(s[:])
	}
	s := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(s[:])
}

// BlobPath is the layout path of a blob.
func BlobPath(digest string) string {
	alg, enc, _ := strings.Cut(digest, ":")
	return "blobs/" + alg + "/" + enc
}

// JSON marshals v; maps are written with sorted keys.
func JSON(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return raw
}

// LayerTar returns the uncompressed tar of a layer.
func LayerTar(l Layer) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, f := range l.Files {
		h := &tar.Header{Name: f.Path, Typeflag: f.Type, Linkname: f.Link, ModTime: Epoch, Mode: 0o644, Format: tar.FormatPAX}
		switch h.Typeflag {
		case 0:
			h.Typeflag = tar.TypeReg
		case tar.TypeDir:
			h.Mode = 0o755
		case tar.TypeSymlink:
			h.Mode = 0o777
		}
		if f.Mode != 0 {
			h.Mode = f.Mode
		}
		if h.Typeflag == tar.TypeReg {
			h.Size = int64(len(f.Data))
		}
		if err := tw.WriteHeader(h); err != nil {
			panic(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(f.Data)); err != nil {
				panic(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// Gzip compresses data deterministically.
func Gzip(data []byte) []byte {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		panic(err)
	}
	zw.ModTime = Epoch
	if _, err := zw.Write(data); err != nil {
		panic(err)
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// Build returns the image archive of the spec.
func Build(s Spec) []byte {
	b := &builder{alg: s.Algorithm, blobs: map[string][]byte{}}
	mIndex, mManifest, mConfig, mLayer, mLayerGz := MediaIndex, MediaManifest, MediaConfig, MediaLayer, MediaLayerGzip
	if s.Docker {
		mIndex, mManifest, mConfig, mLayerGz = DockerList, DockerManifest, DockerConfig, DockerLayerGzip
	}
	descs := []any{}
	for i, img := range s.Images {
		layers, diffIDs := []any{}, []any{}
		for _, l := range img.Layers {
			raw := LayerTar(l)
			diffIDs = append(diffIDs, Digest("sha256", raw))
			if l.Gzip || s.Docker {
				layers = append(layers, b.add(mLayerGz, Gzip(raw)))
			} else {
				layers = append(layers, b.add(mLayer, raw))
			}
		}
		os, arch, variant := img.OS, img.Arch, img.Variant
		if img.Attestation {
			os, arch, variant = "unknown", "unknown", ""
		}
		cfg := map[string]any{
			"architecture": arch,
			"os":           os,
			"created":      Epoch.Format(time.RFC3339),
			"rootfs":       map[string]any{"type": "layers", "diff_ids": diffIDs},
		}
		if !img.Attestation {
			c := map[string]any{"Entrypoint": []any{"/app/run"}}
			if len(img.Volumes) > 0 {
				vols := map[string]any{}
				for _, v := range img.Volumes {
					vols[v] = map[string]any{}
				}
				c["Volumes"] = vols
			}
			cfg["config"] = c
		}
		if variant != "" {
			cfg["variant"] = variant
		}
		if s.Config != nil {
			s.Config(i, cfg)
		}
		m := map[string]any{
			"schemaVersion": 2,
			"mediaType":     mManifest,
			"config":        b.add(mConfig, JSON(cfg)),
			"layers":        layers,
		}
		if s.Manifest != nil {
			s.Manifest(i, m)
		}
		mt, _ := m["mediaType"].(string)
		if mt == "" {
			mt = mManifest
		}
		d := b.add(mt, JSON(m))
		plat := map[string]any{"architecture": arch, "os": os}
		if variant != "" {
			plat["variant"] = variant
		}
		if img.Attestation {
			d["annotations"] = map[string]any{AttestationKey: "attestation-manifest"}
		}
		if s.Platform != nil {
			s.Platform(i, plat)
		}
		d["platform"] = plat
		descs = append(descs, d)
	}
	var top map[string]any
	if s.Multi {
		idx := map[string]any{"schemaVersion": 2, "mediaType": mIndex, "manifests": descs}
		if s.Index != nil {
			s.Index(idx)
		}
		mt, _ := idx["mediaType"].(string)
		if mt == "" {
			mt = mIndex
		}
		top = map[string]any{"schemaVersion": 2, "mediaType": MediaIndex, "manifests": []any{b.add(mt, JSON(idx))}}
	} else {
		top = map[string]any{"schemaVersion": 2, "mediaType": MediaIndex, "manifests": descs}
		if s.Index != nil {
			s.Index(top)
		}
	}
	if s.Blobs != nil {
		s.Blobs(b.blobs)
	}
	files := map[string][]byte{
		"oci-layout": JSON(map[string]any{"imageLayoutVersion": "1.0.0"}),
		"index.json": JSON(top),
	}
	for p, data := range b.blobs {
		files[p] = data
	}
	if s.Layout != nil {
		s.Layout(files)
	}
	return Archive(files)
}

// Archive writes files as a tar archive, sorted, with directory entries.
func Archive(files map[string][]byte) []byte {
	names := make([]string, 0, len(files))
	dirs := map[string]bool{}
	for n := range files {
		names = append(names, n)
		for d := path.Dir(n); d != "."; d = path.Dir(d) {
			dirs[d] = true
		}
	}
	for d := range dirs {
		names = append(names, d+"/")
	}
	sort.Strings(names)
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, n := range names {
		h := &tar.Header{Name: n, ModTime: Epoch, Format: tar.FormatPAX}
		if strings.HasSuffix(n, "/") {
			h.Typeflag, h.Mode = tar.TypeDir, 0o755
		} else {
			h.Typeflag, h.Mode, h.Size = tar.TypeReg, 0o644, int64(len(files[n]))
		}
		if err := tw.WriteHeader(h); err != nil {
			panic(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write(files[n]); err != nil {
				panic(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}
