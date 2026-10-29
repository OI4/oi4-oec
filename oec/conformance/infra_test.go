package conformance

import (
	"strings"
	"testing"
)

// infraMinimal is a valid oec-infra image report of version 1.
const infraMinimal = `{
  "report": 1, "tool": "oec-infra", "version": "dev", "schemas": "2.0.1-dev", "baseline": "eb7889c",
  "catalog": "eb7889c", "command": ["oec-infra", "image", "image.tar"], "mode": "image",
  "input": "image.tar", "form": "oci-archive", "level": 3,
  "started": "2026-10-29T10:00:00Z", "ended": "2026-10-29T10:00:01Z",
  "image": {"digest": "sha256:aa", "media_type": "application/vnd.oci.image.index.v1+json", "index": true,
    "manifests": [{"digest": "sha256:bb", "media_type": "application/vnd.oci.image.manifest.v1+json", "platform": "linux/amd64", "runnable": true, "supported": true, "layers": 2}],
    "algorithms": ["sha256"], "problems": 1},
  "tests": [{"id": "T8-IMG-03", "title": "SBOM", "level": 2, "status": "implemented", "outcome": "fail",
    "reqs": [{"req": "REQ-CNT-014", "obligation": "MUST", "outcome": "fail",
      "evidence": [{"image": "linux/amd64", "path": "/opt/oi4/licenses/MIT.txt", "detail": "no text"}]}]}],
  "summary": {"fail": 1}, "exit": 1
}`

func TestInfraReportSchema(t *testing.T) {
	if !strings.Contains(string(InfraReportSchema), `"const": 1`) {
		t.Error("the infra report schema does not pin report version 1")
	}
	if err := ValidateInfraReport([]byte(infraMinimal)); err != nil {
		t.Fatal(err)
	}
	first := func(m map[string]any, key string) map[string]any { return m[key].([]any)[0].(map[string]any) }
	img := func(m map[string]any) map[string]any { return m["image"].(map[string]any) }
	for name, bad := range map[string][]byte{
		"oec-check":          patch(t, infraMinimal, func(m map[string]any) { m["tool"] = "oec-check" }),
		"version 2":          patch(t, infraMinimal, func(m map[string]any) { m["report"] = 2 }),
		"unknown key":        patch(t, infraMinimal, func(m map[string]any) { m["participants"] = []any{} }),
		"no input":           patch(t, infraMinimal, func(m map[string]any) { delete(m, "input") }),
		"image without form": patch(t, infraMinimal, func(m map[string]any) { delete(m, "form") }),
		"image without data": patch(t, infraMinimal, func(m map[string]any) { delete(m, "image") }),
		"form":               patch(t, infraMinimal, func(m map[string]any) { m["form"] = "docker-archive" }),
		"mode":               patch(t, infraMinimal, func(m map[string]any) { m["mode"] = "replay" }),
		"command":            patch(t, infraMinimal, func(m map[string]any) { m["command"] = []any{"oec-infra", "monitor"} }),
		"exit 2":             patch(t, infraMinimal, func(m map[string]any) { m["exit"] = 2 }),
		"D7 test id":         patch(t, infraMinimal, func(m map[string]any) { first(m, "tests")["id"] = "T7-MON-01" }),
		"evidence topic":     patch(t, infraMinimal, func(m map[string]any) { first(first(first(m, "tests"), "reqs"), "evidence")["topic"] = "Oi4v2/x" }),
		"file content":       patch(t, infraMinimal, func(m map[string]any) { first(img(m), "manifests")["content"] = "x" }),
		"manifest digest":    patch(t, infraMinimal, func(m map[string]any) { first(img(m), "manifests")["digest"] = "abc" }),
		"no runnable":        patch(t, infraMinimal, func(m map[string]any) { delete(first(img(m), "manifests"), "runnable") }),
	} {
		if err := ValidateInfraReport(bad); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := ValidateReport([]byte(infraMinimal)); err == nil {
		t.Error("the oec-check schema accepts an oec-infra report")
	}
}

func TestImageFixtureIndex(t *testing.T) {
	list, err := ImageFixtures()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatal("no image fixture")
	}
	for _, f := range list {
		if f.Kind != "image" || f.Name == "" || !strings.HasSuffix(f.File, ".tar") || f.Status != "draft" && f.Status != "reviewed" {
			t.Errorf("fixture %+v", f)
		}
		for id, e := range f.Expect {
			if !strings.HasPrefix(id, "T8-IMG-") || e.Outcome == "" {
				t.Errorf("%s: expect %s %+v", f.Name, id, e)
			}
		}
	}
}
