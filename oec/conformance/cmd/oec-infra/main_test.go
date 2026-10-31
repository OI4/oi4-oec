package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OI4/oec/conformance"
	"github.com/OI4/oec/conformance/internal/oci/ocitest"
)

const fixtures = "../../testdata/images/"

func call(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	if code == exitPass || code == 1 || code == exitInconclusive {
		checkReports(t, args, out.Bytes())
	}
	return code, out.String(), errOut.String()
}

// checkReports validates every JSON report of a run – standard output with
// --format json and each --report json=PATH – against
// infra-report.schema.json.
func checkReports(t *testing.T, args []string, stdout []byte) {
	t.Helper()
	if len(args) == 0 || args[0] != "image" && args[0] != "bus" && args[0] != "host" && args[0] != "container" && args[0] != "run" {
		return
	}
	for i := 0; i < len(args)-1; i++ {
		var doc []byte
		switch {
		case args[i] == "--format" && args[i+1] == "json":
			doc = stdout
		case args[i] == "--report" && strings.HasPrefix(args[i+1], "json="):
			var err error
			if doc, err = os.ReadFile(strings.TrimPrefix(args[i+1], "json=")); err != nil {
				t.Errorf("%v: %v", args, err)
				continue
			}
		default:
			continue
		}
		if err := conformance.ValidateInfraReport(doc); err != nil {
			t.Errorf("%v: report does not match infra-report.schema.json: %v\n%s", args, err, doc)
		}
	}
}

type jsonReport struct {
	Report int    `json:"report"`
	Mode   string `json:"mode"`
	Form   string `json:"form"`
	Exit   int    `json:"exit"`
	Image  struct {
		Index     bool `json:"index"`
		Manifests []struct {
			Platform  string `json:"platform"`
			Runnable  bool   `json:"runnable"`
			Supported bool   `json:"supported"`
		} `json:"manifests"`
	} `json:"image"`
	Tests []struct {
		ID      string `json:"id"`
		Outcome string `json:"outcome"`
	} `json:"tests"`
}

func decode(t *testing.T, s string) jsonReport {
	t.Helper()
	var r jsonReport
	if err := json.Unmarshal([]byte(s), &r); err != nil {
		t.Fatalf("%v\n%s", err, s)
	}
	return r
}

func TestUsage(t *testing.T) {
	dir := t.TempDir()
	docker := filepath.Join(dir, "docker.tar")
	junk := filepath.Join(dir, "junk.tar")
	if err := os.WriteFile(docker, ocitest.Archive(map[string][]byte{"manifest.json": []byte("[]"), "repositories": []byte("{}")}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(junk, []byte("no tar"), 0o600); err != nil {
		t.Fatal(err)
	}
	good := fixtures + "good.tar"
	for _, c := range []struct {
		args []string
		msg  string
	}{
		{nil, "usage"},
		{[]string{"inspect"}, "unknown command"},
		{[]string{"image"}, "exactly one"},
		{[]string{"image", good, good}, "exactly one"},
		{[]string{"image", "--level", "4", good}, "--level"},
		{[]string{"image", "--format", "pdf", good}, "--format"},
		{[]string{"image", "--report", "pdf=x.pdf", good}, "unknown report format"},
		{[]string{"image", "--report", "json", good}, "want <format>=<path>"},
		{[]string{"image", filepath.Join(dir, "missing.tar")}, "no such file"},
		{[]string{"image", docker}, "Docker"},
		{[]string{"image", junk}, "not an OCI image layout"},
		{[]string{"image", dir}, "not an OCI image layout"},
		{[]string{"report-schema", "x"}, "no arguments"},
	} {
		code, out, errOut := call(t, c.args...)
		if code != exitUsage || !strings.Contains(out+errOut, c.msg) {
			t.Errorf("%v: exit %d, want %d with %q\n%s%s", c.args, code, exitUsage, c.msg, out, errOut)
		}
	}
}

func TestVersionAndSchema(t *testing.T) {
	code, out, _ := call(t, "version")
	if code != exitPass || !strings.HasPrefix(out, "oec-infra dev (schemas ") || !strings.Contains(out, "report 1)") {
		t.Errorf("version: %d %q", code, out)
	}
	code, out, _ = call(t, "report-schema")
	if code != exitPass || out != string(conformance.InfraReportSchema) {
		t.Errorf("report-schema: %d", code)
	}
	var s struct {
		ID string `json:"$id"`
	}
	if err := json.Unmarshal([]byte(out), &s); err != nil || s.ID != conformance.InfraReportSchemaID {
		t.Errorf("report-schema: $id %q %v", s.ID, err)
	}
	if code, out, _ = call(t, "help"); code != exitPass || !strings.Contains(out, "oec-infra image") {
		t.Errorf("help: %d", code)
	}
}

// TestImageFixtures runs every image fixture through the CLI: the exit code
// follows the expectation of testdata/images/index.yaml, every JSON report
// is valid, the text and JUnit files are written.
func TestImageFixtures(t *testing.T) {
	list, err := conformance.ImageFixtures()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatal("no image fixture")
	}
	for _, f := range list {
		t.Run(f.File, func(t *testing.T) {
			dir := t.TempDir()
			js, junit, text := filepath.Join(dir, "r.json"), filepath.Join(dir, "r.xml"), filepath.Join(dir, "r.txt")
			want := exitPass
			for _, e := range f.Expect {
				switch {
				case e.Outcome == "fail":
					want = 1
				case e.Outcome == "skip" && want == exitPass:
					want = exitInconclusive
				}
			}
			code, out, errOut := call(t, "image", "--format", "json", "--report", "json="+js, "--report", "junit="+junit, "--report", "text="+text, fixtures+f.File)
			if code != want {
				t.Fatalf("exit %d, want %d\n%s%s", code, want, out, errOut)
			}
			r := decode(t, out)
			if r.Report != 1 || r.Mode != "image" || r.Form != "oci-archive" || r.Exit != code || len(r.Tests) != 4 {
				t.Errorf("report: %+v", r)
			}
			for _, tr := range r.Tests {
				w := "pass"
				if e, ok := f.Expect[tr.ID]; ok {
					w = e.Outcome
				}
				if tr.Outcome != w {
					t.Errorf("%s: %s, want %s", tr.ID, tr.Outcome, w)
				}
			}
			raw, err := os.ReadFile(junit)
			if err != nil {
				t.Fatal(err)
			}
			var suites struct {
				Name  string `xml:"name,attr"`
				Tests int    `xml:"tests,attr"`
			}
			if err := xml.Unmarshal(raw, &suites); err != nil || suites.Name != "oec-infra" || suites.Tests != 4 {
				t.Errorf("junit: %+v %v", suites, err)
			}
			raw, err = os.ReadFile(text)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(raw), "oec-infra dev image (oci-archive ") || !strings.Contains(string(raw), "summary: ") {
				t.Errorf("text:\n%s", raw)
			}
		})
	}
}

// TestLayoutDirectory: an extracted layout gives the same verdicts as the
// archive.
func TestLayoutDirectory(t *testing.T) {
	raw, err := os.ReadFile(fixtures + "good.tar")
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
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		var b bytes.Buffer
		if _, err := b.ReadFrom(tr); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, b.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	code, out, errOut := call(t, "image", "--format", "json", dir)
	if code != exitPass {
		t.Fatalf("exit %d\n%s%s", code, out, errOut)
	}
	r := decode(t, out)
	if r.Form != "oci-layout" || !r.Image.Index || len(r.Image.Manifests) != 3 {
		t.Errorf("report: %+v", r)
	}
	supported, runnable := 0, 0
	for _, m := range r.Image.Manifests {
		if m.Supported {
			supported++
		}
		if m.Runnable {
			runnable++
		}
	}
	if supported != 2 || runnable != 2 {
		t.Errorf("manifests: %+v", r.Image.Manifests)
	}
}

// TestInconclusive: a zstd layer cannot be read (exit 4); --allow-skip
// turns it into 0; level 1 judges only T8-IMG-01.
func TestInconclusive(t *testing.T) {
	var imgs []ocitest.Image
	for _, a := range []string{"amd64", "arm64"} {
		imgs = append(imgs, ocitest.Image{OS: "linux", Arch: a, Layers: []ocitest.Layer{{Files: []ocitest.File{{Path: "app/run", Data: a}}}}})
	}
	spec := ocitest.Spec{Multi: true, Images: imgs, Manifest: func(_ int, m map[string]any) {
		for _, l := range m["layers"].([]any) {
			l.(map[string]any)["mediaType"] = "application/vnd.oci.image.layer.v1.tar+zstd"
		}
	}}
	p := filepath.Join(t.TempDir(), "zstd.tar")
	if err := os.WriteFile(p, ocitest.Build(spec), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, out, errOut := call(t, "image", "--format", "json", p); code != exitInconclusive {
		t.Errorf("exit %d\n%s%s", code, out, errOut)
	}
	code, out, _ := call(t, "image", "--allow-skip", p)
	if code != exitPass || !strings.Contains(out, "inconclusive") || !strings.Contains(out, "zstd") {
		t.Errorf("--allow-skip: exit %d\n%s", code, out)
	}
	code, out, _ = call(t, "image", "--level", "1", "--format", "json", fixtures+"no-sbom.tar")
	if code != exitPass {
		t.Errorf("--level 1: exit %d\n%s", code, out)
	}
	for _, tr := range decode(t, out).Tests {
		if tr.ID != "T8-IMG-01" && tr.Outcome != "skip" {
			t.Errorf("--level 1: %s %s", tr.ID, tr.Outcome)
		}
	}
}
