package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OI4/oec/conformance"
	"github.com/OI4/oec/conformance/internal/testvec"
	schema "github.com/OI4/oec/schema"
)

func vectors(t *testing.T) map[string]testvec.Vector {
	t.Helper()
	vs, err := testvec.Load("payload")
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]testvec.Vector{}
	for _, v := range vs {
		m[v.ID] = v
	}
	return m
}

func exec(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errb)
	checkReports(t, args, code, out.Bytes())
	return code, out.String(), errb.String()
}

// checkReports validates every JSON report of a monitor or test run –
// standard output with --format json and each --report json=PATH – against
// report.schema.json (C-069).
func checkReports(t *testing.T, args []string, code int, stdout []byte) {
	t.Helper()
	if len(args) == 0 || args[0] != "monitor" && args[0] != "test" || code != exitValid && code != exitInvalid && code != exitInconclusive {
		return
	}
	for i := 1; i+1 < len(args); i++ {
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
		if err := conformance.ValidateReport(doc); err != nil {
			t.Errorf("%v: the JSON report does not conform to report.schema.json: %v", args, err)
		}
	}
}

func message(v testvec.Vector) string {
	b, _ := json.Marshal(map[string]any{"topic": v.Topic, "payload": json.RawMessage(v.Payload)})
	return string(b)
}

func TestExitCodes(t *testing.T) {
	vs := vectors(t)
	valid, invalid := vs["V-PAY-003"], vs["V-PAY-148"]
	dir := t.TempDir()
	write := func(name, data string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	payload := write("payload.json", valid.Payload)
	sub := filepath.Join(dir, "msgs")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	write("msgs/a.json", message(valid))
	write("msgs/b.jsonl", message(valid)+"\n"+message(invalid)+"\n")
	write("bad.json", `{"topic": "x", "payload": {}, "unknown": 1}`)
	for _, c := range []struct {
		name  string
		stdin string
		args  []string
		code  int
		out   string
	}{
		{"no args", "", nil, exitUsage, ""},
		{"unknown command", "", []string{"lint"}, exitUsage, ""},
		{"version", "", []string{"version"}, exitValid, "(schemas " + schema.Version + ", baseline "},
		{"valid file", "", []string{"validate", "--topic", valid.Topic, payload}, exitValid, "VALID"},
		{"valid stdin", valid.Payload, []string{"validate", "--topic", valid.Topic}, exitValid, "VALID"},
		{"invalid", invalid.Payload, []string{"validate", "--topic", invalid.Topic, "-"}, exitInvalid, "REQ-MSG-023"},
		{"invalid topic", "{}", []string{"validate", "--topic", "Oi4v2/Foo"}, exitInvalid, "REQ-TOP-001"},
		{"strict json", `{"a":1,"a":2}`, []string{"validate", "--topic", valid.Topic}, exitInvalid, "[parse] REQ-CMP-005"},
		{"message stdin", message(valid) + "\n" + message(invalid), []string{"validate"}, exitInvalid, "-#2: INVALID"},
		{"pretty message stdin", "{\n \"topic\": " + strTopic(valid) + ",\n \"payload\": " + valid.Payload + "\n}", []string{"validate"}, exitValid, "-: VALID"},
		{"directory", "", []string{"validate", sub}, exitInvalid, "3 message(s), 1 invalid"},
		{"unknown key", "", []string{"validate", filepath.Join(dir, "bad.json")}, exitUsage, ""},
		{"role without topic", "", []string{"validate", "--role", "birth", sub}, exitUsage, ""},
		{"unknown role", valid.Payload, []string{"validate", "--topic", valid.Topic, "--role", "x"}, exitUsage, ""},
		{"object and topic", "{}", []string{"validate", "--topic", valid.Topic, "--object", "DataSetMetaDataType"}, exitUsage, ""},
		{"bad format", "{}", []string{"validate", "--format", "xml", "--topic", valid.Topic}, exitUsage, ""},
		{"missing file", "", []string{"validate", filepath.Join(dir, "none.json")}, exitUsage, ""},
		{"object", vs["V-PAY-011"].Payload, []string{"validate", "--object", "DataSetMetaDataType"}, exitValid, "VALID"},
		{"role", vs["V-PAY-003"].Payload, []string{"validate", "--topic", valid.Topic, "--role", "close"}, exitInvalid, "REQ-LIFE-007"},
	} {
		t.Run(c.name, func(t *testing.T) {
			code, out, errs := exec(t, c.stdin, c.args...)
			if code != c.code {
				t.Errorf("exit %d, want %d\nstdout: %s\nstderr: %s", code, c.code, out, errs)
			}
			if !strings.Contains(out, c.out) {
				t.Errorf("stdout lacks %q:\n%s", c.out, out)
			}
		})
	}
}

func strTopic(v testvec.Vector) string {
	b, _ := json.Marshal(v.Topic)
	return string(b)
}

func TestJSONReport(t *testing.T) {
	inv := vectors(t)["V-PAY-148"]
	code, out, _ := exec(t, inv.Payload, "validate", "--format", "json", "--topic", inv.Topic)
	if code != exitInvalid {
		t.Fatalf("exit %d", code)
	}
	var r struct {
		Tool, Schemas, Baseline string
		Valid                   bool
		Results                 []struct {
			Source   string
			Valid    bool
			Reqs     []string
			Findings []struct{ Req, Layer, Pointer, Detail string }
			Topic    struct{ Rule string }
		}
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	if r.Tool != tool || r.Schemas != schema.Version || r.Baseline != schema.Baseline || r.Valid || len(r.Results) != 1 {
		t.Fatalf("report %+v", r)
	}
	res := r.Results[0]
	if res.Valid || strings.Join(res.Reqs, ",") != "REQ-MSG-023" || res.Topic.Rule == "" || res.Findings[0].Layer != "rule" {
		t.Errorf("result %+v", res)
	}
}
