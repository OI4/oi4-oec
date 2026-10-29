package image

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/OI4/oec/conformance"
	"github.com/OI4/oec/conformance/catalog"
	"github.com/OI4/oec/conformance/internal/oci"
	"github.com/OI4/oec/conformance/internal/oci/ocitest"
	"github.com/OI4/oec/conformance/internal/verdict"
)

func setup(t *testing.T) ([]verdict.Test, verdict.Requirements) {
	t.Helper()
	cat, err := catalog.Load(catalog.D8File)
	if err != nil {
		t.Fatal(err)
	}
	reqs, err := verdict.LoadRequirements()
	if err != nil {
		t.Fatal(err)
	}
	return Tests(cat), reqs
}

func run(t *testing.T, tests []verdict.Test, reqs verdict.Requirements, src string, level int) []verdict.TestResult {
	t.Helper()
	res, err := Check(src)
	if err != nil {
		t.Fatal(err)
	}
	return verdict.Build(tests, res.Observations, reqs, level)
}

func TestCatalogStatus(t *testing.T) {
	tests, _ := setup(t)
	for _, tc := range tests {
		if (tc.Status == "implemented") != Implemented[tc.ID] {
			t.Errorf("%s: catalog status %s, implemented here %v", tc.ID, tc.Status, Implemented[tc.ID])
		}
	}
	if len(tests) != 4 {
		t.Errorf("%d image tests in the catalog", len(tests))
	}
}

// TestFixtures checks every image fixture against its expectation at
// level 3 (testdata/images/index.yaml).
func TestFixtures(t *testing.T) {
	tests, reqs := setup(t)
	fixtures, err := conformance.ImageFixtures()
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, f := range fixtures {
		listed[f.File] = true
		t.Run(f.File, func(t *testing.T) {
			if f.Status != "draft" && f.Status != "reviewed" {
				t.Errorf("status %q", f.Status)
			}
			for id := range f.Expect {
				if !Implemented[id] {
					t.Errorf("expectation for %s, which is no implemented image test", id)
				}
			}
			for _, tr := range run(t, tests, reqs, filepath.Join(fixtureDir, f.File), 3) {
				want, ok := f.Expect[tr.ID]
				if !ok {
					want.Outcome = string(verdict.Pass)
				}
				if string(tr.Outcome) != want.Outcome {
					t.Errorf("%s: outcome %s, want %s (%+v)", tr.ID, tr.Outcome, want.Outcome, tr.Reqs)
					continue
				}
				var with []string
				for _, r := range tr.Reqs {
					switch {
					case string(r.Outcome) == want.Outcome:
						with = append(with, r.Req)
					case r.Outcome != verdict.Skip && !verdict.Worse(tr.Outcome, r.Outcome):
						t.Errorf("%s: %s has %s, not better than the test outcome %s", tr.ID, r.Req, r.Outcome, tr.Outcome)
					}
				}
				if want.Outcome != string(verdict.Pass) {
					w := slices.Clone(want.Reqs)
					sort.Strings(w)
					sort.Strings(with)
					if !slices.Equal(with, w) {
						t.Errorf("%s: requirements with %s %v, want %v", tr.ID, want.Outcome, with, w)
					}
				}
				if f.Positive && tr.Outcome != verdict.Pass {
					t.Errorf("%s: a positive fixture must pass", tr.ID)
				}
			}
		})
	}
	files, _ := filepath.Glob(filepath.Join(fixtureDir, "*.tar"))
	for _, p := range files {
		if !listed[filepath.Base(p)] {
			t.Errorf("%s is not listed in index.yaml", p)
		}
	}
	if len(files) != len(fixtures) {
		t.Errorf("%d files, %d index entries", len(files), len(fixtures))
	}
}

// TestFixtureCoverage: every implemented image test has a positive fixture
// (it passes) and a negative fixture (an expectation worse than pass).
func TestFixtureCoverage(t *testing.T) {
	fixtures, err := conformance.ImageFixtures()
	if err != nil {
		t.Fatal(err)
	}
	for id := range Implemented {
		pos, neg := false, false
		for _, f := range fixtures {
			e, ok := f.Expect[id]
			pos = pos || f.Positive && !ok
			neg = neg || ok && e.Outcome != string(verdict.Pass)
		}
		if !pos || !neg {
			t.Errorf("%s: positive fixture %v, negative fixture %v", id, pos, neg)
		}
	}
}

// TestLevel1: at level 1 only T8-IMG-01 is judged.
func TestLevel1(t *testing.T) {
	tests, reqs := setup(t)
	for _, f := range []string{"no-sbom.tar", "unsupported-arch.tar", "sha512.tar"} {
		res := run(t, tests, reqs, filepath.Join(fixtureDir, f), 1)
		for _, tr := range res {
			want := verdict.Skip
			if tr.ID == TOCI {
				want = verdict.Pass
			}
			if tr.Outcome != want {
				t.Errorf("%s %s: %s at level 1", f, tr.ID, tr.Outcome)
			}
		}
		if code := verdict.Exit(res, false); code != verdict.ExitPass {
			t.Errorf("%s: exit %d at level 1", f, code)
		}
	}
}

// TestLayoutDirectory: an extracted layout gives the verdict of its archive.
func TestLayoutDirectory(t *testing.T) {
	tests, reqs := setup(t)
	for _, f := range []string{"good.tar", "bad-digest.tar", "dangling-symlink.tar"} {
		raw, err := os.ReadFile(filepath.Join(fixtureDir, f))
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		tr := tar.NewReader(bytes.NewReader(raw))
		for {
			h, err := tr.Next()
			if err != nil {
				break
			}
			p := filepath.Join(dir, filepath.FromSlash(h.Name))
			if h.Typeflag == tar.TypeDir {
				_ = os.MkdirAll(p, 0o755)
				continue
			}
			var b bytes.Buffer
			_, _ = b.ReadFrom(tr)
			if err := os.WriteFile(p, b.Bytes(), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		a := run(t, tests, reqs, filepath.Join(fixtureDir, f), 3)
		d := run(t, tests, reqs, dir, 3)
		for i := range a {
			if a[i].Outcome != d[i].Outcome {
				t.Errorf("%s %s: archive %s, directory %s", f, a[i].ID, a[i].Outcome, d[i].Outcome)
			}
		}
	}
}

// TestZstdInconclusive: zstd layers cannot be read; T8-IMG-03 is skipped
// inconclusive (exit 4), the rest is judged.
func TestZstdInconclusive(t *testing.T) {
	tests, reqs := setup(t)
	spec := ocitest.Spec{Multi: true, Images: multi(good), Manifest: func(_ int, m map[string]any) {
		for _, l := range m["layers"].([]any) {
			l.(map[string]any)["mediaType"] = "application/vnd.oci.image.layer.v1.tar+zstd"
		}
	}}
	p := filepath.Join(t.TempDir(), "zstd.tar")
	if err := os.WriteFile(p, ocitest.Build(spec), 0o600); err != nil {
		t.Fatal(err)
	}
	res := run(t, tests, reqs, p, 3)
	for _, tr := range res {
		want := verdict.Pass
		if tr.ID == TSBOM {
			want = verdict.Skip
		}
		if tr.Outcome != want {
			t.Errorf("%s: %s", tr.ID, tr.Outcome)
		}
		if tr.ID == TSBOM && (len(tr.Reqs) == 0 || !tr.Reqs[0].Inconclusive || !strings.Contains(tr.Reqs[0].Reason, "zstd")) {
			t.Errorf("%s: %+v", tr.ID, tr.Reqs)
		}
	}
	if code := verdict.Exit(res, false); code != verdict.ExitInconclusive {
		t.Errorf("exit %d", code)
	}
}

func TestSupported(t *testing.T) {
	for _, c := range []struct {
		os, arch, variant string
		ok                bool
	}{
		{"linux", "amd64", "", true}, {"linux", "arm64", "", true}, {"linux", "arm64", "v8", true},
		{"linux", "arm", "v7", true}, {"linux", "arm", "", true}, {"linux", "arm", "v6", false},
		{"windows", "amd64", "", false}, {"linux", "386", "", false}, {"linux", "amd64", "v1", true}, {"linux", "amd64", "v3", false},
	} {
		_, ok := Supported(ociPlatform(c.os, c.arch, c.variant))
		if ok != c.ok {
			t.Errorf("%s/%s/%s: %v", c.os, c.arch, c.variant, ok)
		}
	}
}

func ociPlatform(os, arch, variant string) oci.Platform {
	return oci.Platform{OS: os, Architecture: arch, Variant: variant}
}
