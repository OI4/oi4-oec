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
		"evidence line":      patch(t, infraMinimal, func(m map[string]any) { first(first(first(m, "tests"), "reqs"), "evidence")["line"] = 3 }),
		"image with broker":  patch(t, infraMinimal, func(m map[string]any) { m["broker"] = map[string]any{"url": "mqtts://b:8883", "probe": "x"} }),
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

// infraBus is a valid oec-infra bus report of version 1.
const infraBus = `{
  "report": 1, "tool": "oec-infra", "version": "dev", "schemas": "2.0.1-dev", "baseline": "a55f475",
  "catalog": "a55f475", "command": ["oec-infra", "bus", "--broker", "mqtts://broker:8883"], "mode": "bus",
  "input": "mqtts://broker:8883", "level": 3,
  "started": "2026-10-29T10:00:00Z", "ended": "2026-10-29T10:00:09Z",
  "broker": {"url": "mqtts://broker:8883", "probe": "oi4.org/oec-infra/D8/1a2b", "tls": ["1.2", "1.3"],
    "certificate": {"subject": "CN=broker", "issuer": "CN=OEC CA", "not_before": "2026-01-01T00:00:00Z", "not_after": "2027-01-01T00:00:00Z", "names": ["broker", "10.0.0.1"]},
    "max_packet_size": 262144, "registry": "oi4.org/Registry/SIM/1"},
  "tests": [{"id": "T8-BUS-02", "title": "ACL", "level": 1, "status": "implemented", "outcome": "fail",
    "reqs": [{"req": "REQ-GEN-001", "obligation": "MUST", "outcome": "fail",
      "evidence": [{"topic": "Oi4v2/Utility/x/Set/Config/x", "detail": "delivered"}]}]}],
  "summary": {"fail": 1}, "exit": 1
}`

func TestInfraBusReport(t *testing.T) {
	if err := ValidateInfraReport([]byte(infraBus)); err != nil {
		t.Fatal(err)
	}
	br := func(m map[string]any) map[string]any { return m["broker"].(map[string]any) }
	for name, bad := range map[string][]byte{
		"no broker":            patch(t, infraBus, func(m map[string]any) { delete(m, "broker") }),
		"form":                 patch(t, infraBus, func(m map[string]any) { m["form"] = "oci-layout" }),
		"image":                patch(t, infraBus, func(m map[string]any) { m["image"] = map[string]any{"index": false, "manifests": []any{}} }),
		"command":              patch(t, infraBus, func(m map[string]any) { m["command"] = []any{"oec-infra", "image"} }),
		"no probe":             patch(t, infraBus, func(m map[string]any) { delete(br(m), "probe") }),
		"password":             patch(t, infraBus, func(m map[string]any) { br(m)["password"] = "x" }),
		"tls 1.1":              patch(t, infraBus, func(m map[string]any) { br(m)["tls"] = []any{"1.1"} }),
		"url":                  patch(t, infraBus, func(m map[string]any) { br(m)["url"] = "tcp://b:1883" }),
		"certificate pem":      patch(t, infraBus, func(m map[string]any) { br(m)["certificate"].(map[string]any)["pem"] = "x" }),
		"no certificate dates": patch(t, infraBus, func(m map[string]any) { delete(br(m)["certificate"].(map[string]any), "not_after") }),
	} {
		if err := ValidateInfraReport(bad); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestBusFixtureIndex(t *testing.T) {
	list, err := BusFixtures()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatal("no bus fixture")
	}
	seen := map[string]bool{}
	for _, f := range list {
		if f.Kind != "bus" || f.Fault == "" || f.File != "" || seen[f.Fault] || f.Status != "draft" && f.Status != "reviewed" {
			t.Errorf("fixture %+v", f)
		}
		seen[f.Fault] = true
		for id, e := range f.Expect {
			if !strings.HasPrefix(id, "T8-BUS-") || e.Outcome == "" {
				t.Errorf("%s: expect %s %+v", f.Fault, id, e)
			}
		}
	}
}

// infraHost is a valid oec-infra host report of version 1.
const infraHost = `{
  "report": 1, "tool": "oec-infra", "version": "dev", "schemas": "2.0.1-dev", "baseline": "b955783",
  "catalog": "b955783", "command": ["oec-infra", "host", "--root", "/"], "mode": "host",
  "input": "/", "level": 3,
  "started": "2026-10-30T10:00:00Z", "ended": "2026-10-30T10:00:02Z",
  "host": {"root": "/", "container": "oec-app",
    "storages": [{"name": "mqtt", "path": "/etc/oi4/mqtt", "exists": true, "mount": "ro",
      "files": [{"name": "broker.json", "mode": "-r--r--r--", "size": 61}]},
      {"name": "secrets", "path": "/run/secrets/oi4", "exists": false, "mount": "none"}],
    "platform": {"os": "linux", "kernel": "6.6.31", "machine": "aarch64", "memory": 4000000000,
      "storage": 32000000000, "interfaces": ["eth0"], "daemon": "unix:///var/run/docker.sock"}},
  "tests": [{"id": "T8-HST-04", "title": "Platform host", "level": 1, "status": "implemented", "outcome": "pass",
    "reqs": [{"req": "REQ-PLT-003", "obligation": "MUST", "outcome": "manual",
      "evidence": [{"path": "/etc/os-release", "detail": "patch level: Debian 12"}]}]}],
  "summary": {"pass": 1}, "exit": 0
}`

func TestInfraHostReport(t *testing.T) {
	if err := ValidateInfraReport([]byte(infraHost)); err != nil {
		t.Fatal(err)
	}
	h := func(m map[string]any) map[string]any { return m["host"].(map[string]any) }
	st := func(m map[string]any) map[string]any { return h(m)["storages"].([]any)[0].(map[string]any) }
	for name, bad := range map[string][]byte{
		"no host":         patch(t, infraHost, func(m map[string]any) { delete(m, "host") }),
		"broker":          patch(t, infraHost, func(m map[string]any) { m["broker"] = map[string]any{} }),
		"form":            patch(t, infraHost, func(m map[string]any) { m["form"] = "oci-layout" }),
		"command":         patch(t, infraHost, func(m map[string]any) { m["command"] = []any{"oec-infra", "bus"} }),
		"no root":         patch(t, infraHost, func(m map[string]any) { delete(h(m), "root") }),
		"storage kind":    patch(t, infraHost, func(m map[string]any) { st(m)["name"] = "data" }),
		"mount":           patch(t, infraHost, func(m map[string]any) { st(m)["mount"] = "readonly" }),
		"file content":    patch(t, infraHost, func(m map[string]any) { st(m)["files"].([]any)[0].(map[string]any)["content"] = "x" }),
		"file mode":       patch(t, infraHost, func(m map[string]any) { st(m)["files"].([]any)[0].(map[string]any)["mode"] = "0644" }),
		"platform secret": patch(t, infraHost, func(m map[string]any) { h(m)["platform"].(map[string]any)["password"] = "x" }),
	} {
		if err := ValidateInfraReport(bad); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestHostFixtureIndex(t *testing.T) {
	list, err := HostFixtures()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatal("no host fixture")
	}
	seen := map[string]bool{}
	for _, f := range list {
		if f.Kind != "host" || f.Fault == "" || f.File != "" || seen[f.Fault] || f.Status != "draft" && f.Status != "reviewed" {
			t.Errorf("fixture %+v", f)
		}
		seen[f.Fault] = true
		if len(f.Expect) == 0 {
			t.Errorf("%s: no expectation", f.Fault)
		}
		for id, e := range f.Expect {
			if !strings.HasPrefix(id, "T8-HST-") || e.Outcome == "" {
				t.Errorf("%s: expect %s %+v", f.Fault, id, e)
			}
		}
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
