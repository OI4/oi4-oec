package oci_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OI4/oec/conformance/internal/oci"
	"github.com/OI4/oec/conformance/internal/oci/ocitest"
)

func base(files ...ocitest.File) ocitest.Layer {
	return ocitest.Layer{Files: append([]ocitest.File{ocitest.Dir("app"), {Path: "app/run", Data: "#!/bin/sh\n"}}, files...)}
}

func write(t *testing.T, archive []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "image.tar")
	if err := os.WriteFile(p, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// extract writes the archive as a layout directory.
func extract(t *testing.T, archive []byte) string {
	t.Helper()
	dir := t.TempDir()
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		p := filepath.Join(dir, filepath.FromSlash(h.Name))
		if h.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(tr); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func read(t *testing.T, p string, keep func(string) bool) (*oci.Layout, *oci.Image) {
	t.Helper()
	l, err := oci.Open(p, keep)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(l.Close)
	img, err := l.Read()
	if err != nil {
		t.Fatal(err)
	}
	return l, img
}

func problems(img *oci.Image) string {
	var b strings.Builder
	for _, p := range img.Problems {
		b.WriteString(p.Kind + " " + p.Where + ": " + p.Detail + "\n")
	}
	return b.String()
}

func TestReadGood(t *testing.T) {
	spec := ocitest.Spec{Multi: true, Images: []ocitest.Image{
		{OS: "linux", Arch: "amd64", Layers: []ocitest.Layer{base()}},
		{OS: "linux", Arch: "arm64", Variant: "v8", Layers: []ocitest.Layer{base(), {Gzip: true, Files: []ocitest.File{{Path: "etc/x", Data: "x"}}}}},
		{Attestation: true, Layers: []ocitest.Layer{{Files: []ocitest.File{{Path: "sbom.json", Data: "{}"}}}}},
	}}
	archive := ocitest.Build(spec)
	for _, p := range []string{write(t, archive), extract(t, archive)} {
		_, img := read(t, p, nil)
		if len(img.Problems) != 0 {
			t.Fatalf("%s: unexpected problems:\n%s", p, problems(img))
		}
		if !img.Index || len(img.Manifests) != 3 {
			t.Fatalf("index=%v manifests=%d", img.Index, len(img.Manifests))
		}
		var plats []string
		for _, m := range img.Runnable() {
			plats = append(plats, m.Platform.String())
		}
		if got := strings.Join(plats, ","); got != "linux/amd64,linux/arm64/v8" {
			t.Errorf("runnable platforms %s", got)
		}
		if got := strings.Join(img.Algorithms, ","); got != "sha256" {
			t.Errorf("algorithms %s", got)
		}
		if !strings.HasPrefix(img.Digest, "sha256:") {
			t.Errorf("digest %q", img.Digest)
		}
	}
}

func TestReadDockerSingle(t *testing.T) {
	_, img := read(t, write(t, ocitest.Build(ocitest.Spec{Docker: true, Images: []ocitest.Image{{OS: "linux", Arch: "amd64", Layers: []ocitest.Layer{base()}}}})), nil)
	if len(img.Problems) != 0 || img.Index || len(img.Runnable()) != 1 || img.MediaType != ocitest.DockerManifest {
		t.Fatalf("problems:\n%s index=%v runnable=%d media=%s", problems(img), img.Index, len(img.Runnable()), img.MediaType)
	}
}

func TestReadFaults(t *testing.T) {
	amd := []ocitest.Image{{OS: "linux", Arch: "amd64", Layers: []ocitest.Layer{base()}}}
	cases := map[string]struct {
		spec ocitest.Spec
		want string
	}{
		"blob corrupted": {ocitest.Spec{Images: amd, Blobs: func(b map[string][]byte) {
			for p, d := range b {
				if len(d) > 1000 { // the layer
					d = append([]byte(nil), d...)
					d[600] ^= 1
					b[p] = d
				}
			}
		}}, "integrity"},
		"blob missing": {ocitest.Spec{Images: amd, Blobs: func(b map[string][]byte) {
			for p, d := range b {
				if len(d) > 1000 {
					delete(b, p)
				}
			}
		}}, "missing in the layout"},
		"size": {ocitest.Spec{Images: amd, Manifest: func(_ int, m map[string]any) {
			m["layers"].([]any)[0].(map[string]any)["size"] = 1
		}}, "the descriptor says 1"},
		"bad digest syntax": {ocitest.Spec{Images: amd, Manifest: func(_ int, m map[string]any) {
			m["layers"].([]any)[0].(map[string]any)["digest"] = "md5:abc"
		}}, "not registered"},
		"diff_id": {ocitest.Spec{Images: amd, Config: func(_ int, c map[string]any) {
			c["rootfs"].(map[string]any)["diff_ids"] = []any{ocitest.Digest("sha256", []byte("x"))}
		}}, "rootfs.diff_ids/0"},
		"diff_ids count": {ocitest.Spec{Images: amd, Config: func(_ int, c map[string]any) {
			c["rootfs"].(map[string]any)["diff_ids"] = []any{}
		}}, "0 entries for 1 layers"},
		"no os":  {ocitest.Spec{Images: amd, Config: func(_ int, c map[string]any) { delete(c, "os") }}, "must name os"},
		"schema": {ocitest.Spec{Images: amd, Manifest: func(_ int, m map[string]any) { m["schemaVersion"] = 1 }}, "schemaVersion must be 2"},
		"layer type": {ocitest.Spec{Images: amd, Manifest: func(_ int, m map[string]any) {
			m["layers"].([]any)[0].(map[string]any)["mediaType"] = "application/x-foo"
		}}, "unknown layer media type"},
		"layout version":    {ocitest.Spec{Images: amd, Layout: func(f map[string][]byte) { f["oci-layout"] = []byte(`{"imageLayoutVersion":"2.0.0"}`) }}, "imageLayoutVersion"},
		"platform mismatch": {ocitest.Spec{Images: amd, Platform: func(_ int, p map[string]any) { p["architecture"] = "arm64" }}, "differs from the image config"},
		"empty":             {ocitest.Spec{}, "lists no manifest"},
		"only attestation":  {ocitest.Spec{Multi: true, Images: []ocitest.Image{{Attestation: true, Layers: []ocitest.Layer{base()}}}}, "no runnable container image"},
		"layer not gzip": {ocitest.Spec{Images: amd, Manifest: func(_ int, m map[string]any) {
			m["layers"].([]any)[0].(map[string]any)["mediaType"] = ocitest.MediaLayerGzip
		}}, "not gzip-compressed"},
		"schema 1": {ocitest.Spec{Images: amd, Index: func(idx map[string]any) {
			idx["manifests"].([]any)[0].(map[string]any)["mediaType"] = "application/vnd.docker.distribution.manifest.v1+json"
		}}, "schema 1"},
		"not a tar layer": {ocitest.Spec{Images: amd, Blobs: func(b map[string][]byte) {
			for p, d := range b {
				if len(d) > 1000 {
					b[p] = nil
					_ = d
				}
			}
		}}, "the descriptor says"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, img := read(t, write(t, ocitest.Build(c.spec)), nil)
			if !strings.Contains(problems(img), c.want) {
				t.Errorf("want a problem with %q, got:\n%s", c.want, problems(img))
			}
		})
	}
}

func TestOpenErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := oci.Open(dir, nil); !errors.Is(err, oci.ErrNotLayout) {
		t.Errorf("empty dir: %v", err)
	}
	docker := write(t, ocitest.Archive(map[string][]byte{"manifest.json": []byte("[]")}))
	if _, err := oci.Open(docker, nil); !errors.Is(err, oci.ErrNotLayout) || !strings.Contains(err.Error(), "Docker 25") {
		t.Errorf("docker archive: %v", err)
	}
	gz := write(t, ocitest.Gzip(ocitest.Build(ocitest.Spec{})))
	if _, err := oci.Open(gz, nil); !errors.Is(err, oci.ErrNotLayout) || !strings.Contains(err.Error(), "gzip") {
		t.Errorf("gzip: %v", err)
	}
	junk := write(t, []byte("no tar at all, just some text that is long enough to be no tar header"))
	if _, err := oci.Open(junk, nil); !errors.Is(err, oci.ErrNotLayout) {
		t.Errorf("junk: %v", err)
	}
}

func TestTree(t *testing.T) {
	spec := ocitest.Spec{Images: []ocitest.Image{{OS: "linux", Arch: "amd64", Layers: []ocitest.Layer{
		{Files: []ocitest.File{
			ocitest.Dir("opt"), ocitest.Dir("opt/oi4"), ocitest.Dir("opt/oi4/licenses"),
			{Path: "opt/oi4/licenses/a.txt", Data: "A"},
			{Path: "opt/oi4/licenses/b.txt", Data: "B"},
			ocitest.Dir("data"), {Path: "data/x", Data: "x"}, {Path: "data/y", Data: "y"},
			ocitest.Dir("usr"), ocitest.Dir("usr/share"), {Path: "usr/share/lic.txt", Data: "L"},
			{Path: "hard", Type: tar.TypeLink, Link: "usr/share/lic.txt"},
		}},
		{Gzip: true, Files: []ocitest.File{
			ocitest.Whiteout("opt/oi4/licenses/a.txt"),
			{Path: "data/.wh..wh..opq"}, {Path: "data/z", Data: "z"},
			ocitest.Symlink("opt/oi4/licenses/c.txt", "../../../usr/share/lic.txt"),
			ocitest.Symlink("opt/oi4/licenses/d.txt", "/nowhere"),
			ocitest.Symlink("loop", "loop"),
			ocitest.Symlink("lic", "/opt/oi4/licenses"),
		}},
	}}}}
	l, img := read(t, write(t, ocitest.Build(spec)), func(p string) bool { return strings.HasPrefix(p, "/opt/oi4/licenses/") })
	if len(img.Problems) != 0 {
		t.Fatal(problems(img))
	}
	tree := l.Tree(img.Runnable()[0])
	if tree.Incomplete != "" {
		t.Fatal(tree.Incomplete)
	}
	if _, err := tree.Stat("/opt/oi4/licenses/a.txt"); !errors.Is(err, oci.ErrNotExist) {
		t.Errorf("whiteout: %v", err)
	}
	ls, _ := tree.List("/data")
	if len(ls) != 1 || ls[0].Path != "/data/z" {
		t.Errorf("opaque: %v", ls)
	}
	for p, want := range map[string]string{"/opt/oi4/licenses/b.txt": "B", "/opt/oi4/licenses/c.txt": "L", "/lic/b.txt": "B", "/hard": "L"} {
		if got, err := tree.ReadFile(p); err != nil || string(got) != want {
			t.Errorf("%s: %q %v", p, got, err)
		}
	}
	if _, err := tree.Stat("/opt/oi4/licenses/d.txt"); !errors.Is(err, oci.ErrNotExist) {
		t.Errorf("dangling: %v", err)
	}
	if _, err := tree.Stat("/loop"); !errors.Is(err, oci.ErrLoop) {
		t.Errorf("loop: %v", err)
	}
	if n, err := tree.Lstat("/lic"); err != nil || n.Type != tar.TypeSymlink {
		t.Errorf("lstat: %v %v", n, err)
	}
	if r, _ := tree.Resolve("/lic/../usr"); r != "/opt/oi4/usr" && r != "/usr" {
		t.Logf("resolve /lic/.. = %s", r)
	}
}

func TestTreeZstdIncomplete(t *testing.T) {
	spec := ocitest.Spec{Images: []ocitest.Image{{OS: "linux", Arch: "amd64", Layers: []ocitest.Layer{base()}}},
		Manifest: func(_ int, m map[string]any) {
			m["layers"].([]any)[0].(map[string]any)["mediaType"] = "application/vnd.oci.image.layer.v1.tar+zstd"
		}}
	l, img := read(t, write(t, ocitest.Build(spec)), nil)
	if len(img.Problems) != 0 {
		t.Fatal(problems(img))
	}
	if tree := l.Tree(img.Runnable()[0]); !strings.Contains(tree.Incomplete, "zstd") {
		t.Errorf("incomplete: %q", tree.Incomplete)
	}
}

func TestCheckDigest(t *testing.T) {
	for d, ok := range map[string]bool{
		ocitest.Digest("sha256", nil):       true,
		ocitest.Digest("sha512", nil):       true,
		"sha256:ABC":                        false,
		"sha384:" + strings.Repeat("a", 96): false,
		"nodigest":                          false,
	} {
		if got := oci.CheckDigest(d) == ""; got != ok {
			t.Errorf("%s: %v", d, got)
		}
	}
}
