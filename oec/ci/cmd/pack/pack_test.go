package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// tree writes a small directory with an executable, a nested file and an
// empty directory, with modification times and permissions that must not
// reach the archive.
func tree(t *testing.T, mtime time.Time, perm os.FileMode) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "in")
	files := map[string]string{"b.txt": "b\n", "sub/a.txt": "a\n", "bin/tool": "#!/bin/sh\n"}
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		mode := perm
		if strings.HasPrefix(name, "bin/") {
			mode = 0o755
		}
		if err := os.WriteFile(p, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "empty"), 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func sum(t *testing.T, path string) [32]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(data)
}

// TestReproducible: two trees with the same content but other times and
// permissions give the same bytes, for tar.gz and zip.
func TestReproducible(t *testing.T) {
	a := tree(t, time.Unix(1, 0), 0o600)
	b := tree(t, time.Now(), 0o664)
	for _, ext := range []string{".tar.gz", ".zip"} {
		out := t.TempDir()
		for i, dir := range []string{a, b} {
			if err := pack(filepath.Join(out, string(rune('a'+i))+ext), dir, "pkg-1.0.0/", 1700000000); err != nil {
				t.Fatal(err)
			}
		}
		if sum(t, filepath.Join(out, "a"+ext)) != sum(t, filepath.Join(out, "b"+ext)) {
			t.Errorf("%s: different bytes for the same content", ext)
		}
	}
}

// TestTarGz: sorted entries under the prefix, owner 0, fixed time, 0644 or
// 0755, a gzip header without name and time.
func TestTarGz(t *testing.T) {
	out := filepath.Join(t.TempDir(), "x.tar.gz")
	if err := pack(out, tree(t, time.Now(), 0o600), "pkg/", 1700000000); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	if zr.Name != "" || !zr.ModTime.IsZero() {
		t.Errorf("gzip header: name %q, time %v", zr.Name, zr.ModTime)
	}
	tr := tar.NewReader(zr)
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
		if h.Uid != 0 || h.Gid != 0 || h.Uname != "" || h.Gname != "" {
			t.Errorf("%s: owner %d/%d %q/%q", h.Name, h.Uid, h.Gid, h.Uname, h.Gname)
		}
		if h.ModTime.Unix() != 1700000000 {
			t.Errorf("%s: time %v", h.Name, h.ModTime)
		}
		want := int64(0o644)
		if h.Typeflag == tar.TypeDir || h.Name == "pkg/bin/tool" {
			want = 0o755
		}
		if h.Mode != want {
			t.Errorf("%s: mode %o, want %o", h.Name, h.Mode, want)
		}
	}
	want := "pkg/ pkg/b.txt pkg/bin/ pkg/bin/tool pkg/empty/ pkg/sub/ pkg/sub/a.txt"
	if got := strings.Join(names, " "); got != want {
		t.Errorf("entries %s, want %s", got, want)
	}
}

// TestZip: the same entries in a zip with the fixed time and modes.
func TestZip(t *testing.T) {
	out := filepath.Join(t.TempDir(), "x.zip")
	if err := pack(out, tree(t, time.Now(), 0o600), "pkg/", 1700000000); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
		if !f.Modified.Equal(time.Unix(1700000000, 0)) {
			t.Errorf("%s: time %v", f.Name, f.Modified)
		}
		want := os.FileMode(0o644)
		if f.Name == "pkg/bin/tool" {
			want = 0o755
		}
		if f.FileInfo().IsDir() {
			want = os.ModeDir | 0o755
		}
		if f.Mode() != want {
			t.Errorf("%s: mode %v, want %v", f.Name, f.Mode(), want)
		}
	}
	want := "pkg/ pkg/b.txt pkg/bin/ pkg/bin/tool pkg/empty/ pkg/sub/ pkg/sub/a.txt"
	if got := strings.Join(names, " "); got != want {
		t.Errorf("entries %s, want %s", got, want)
	}
}

// TestReject: an unknown extension, a bad prefix and a symbolic link fail,
// and a failed run leaves no archive.
func TestReject(t *testing.T) {
	dir := tree(t, time.Now(), 0o644)
	out := t.TempDir()
	if err := pack(filepath.Join(out, "x.tar"), dir, "pkg/", 0); err == nil {
		t.Error("x.tar accepted")
	}
	for _, prefix := range []string{"", "/abs/", "../up/", "noslash"} {
		if err := pack(filepath.Join(out, "x.zip"), dir, prefix, 0); err == nil {
			t.Errorf("prefix %q accepted", prefix)
		}
	}
	if err := os.Symlink("b.txt", filepath.Join(dir, "link")); err != nil {
		t.Skip("no symbolic links:", err)
	}
	if err := pack(filepath.Join(out, "y.zip"), dir, "pkg/", 0); err == nil {
		t.Error("symbolic link accepted")
	}
	if _, err := os.Stat(filepath.Join(out, "y.zip")); err == nil {
		t.Error("a failed run left y.zip")
	}
}

// TestGzipHeader: the gzip header does not depend on the host or the time
// (no name, no time, best compression, OS byte 255).
func TestGzipHeader(t *testing.T) {
	out := filepath.Join(t.TempDir(), "x.tar.gz")
	if err := pack(out, tree(t, time.Now(), 0o644), "p/", 0); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data[:10], []byte{0x1f, 0x8b, 8, 0, 0, 0, 0, 0, 2, 255}) {
		t.Errorf("gzip header % x", data[:10])
	}
}
