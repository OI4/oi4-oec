package oec

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// notWrapped are the functions of the C ABI the Go package does not call,
// with the reason; each is named in README.md (P4c exit criterion 1).
var notWrapped = map[string]string{
	"oec_app_run": "installs SIGINT/SIGTERM handlers in the process; App.Run(ctx) uses oec_app_start, oec_app_wait and oec_app_shutdown instead",
}

// Every function of oec.h is called by the package or listed in
// notWrapped (ADR-0027 §2, exit criterion 1 of P4c).
func TestABICoverage(t *testing.T) {
	header, err := os.ReadFile(filepath.Join("..", "rust", "oec-ffi", "include", "oec.h"))
	if err != nil {
		t.Fatal(err)
	}
	decl := regexp.MustCompile(`(?m)^[a-z][^(\n]*\b(oec_[a-z0-9_]+)\(`)
	declared := map[string]bool{}
	for _, m := range decl.FindAllStringSubmatch(string(header), -1) {
		if !strings.HasPrefix(m[0], "typedef") {
			declared[m[1]] = true
		}
	}
	if len(declared) < 45 {
		t.Fatalf("only %d functions found in oec.h", len(declared))
	}

	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	use := regexp.MustCompile(`\bC\.(oec_[a-z0-9_]+)\(`)
	used := map[string]bool{}
	for _, f := range sources {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range use.FindAllStringSubmatch(string(src), -1) {
			used[m[1]] = true
		}
	}

	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for f := range declared {
		_, listed := notWrapped[f]
		switch {
		case used[f] && listed:
			t.Errorf("%s is called but listed in notWrapped", f)
		case !used[f] && !listed:
			missing = append(missing, f)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("functions of oec.h neither called nor in notWrapped: %v", missing)
	}
	calls := 0
	for f := range used {
		switch {
		case declared[f]:
			calls++
		case !strings.Contains(string(header), " "+f+";"): // C.oec_resource(r) converts to a typedef
			t.Errorf("%s is used but neither a function nor a type of oec.h", f)
		}
	}
	for f := range notWrapped {
		if !declared[f] {
			t.Errorf("notWrapped lists %s, which oec.h does not declare", f)
		}
		if !strings.Contains(string(readme), "`"+f+"`") {
			t.Errorf("README.md does not name the not wrapped %s", f)
		}
	}
	t.Logf("oec.h: %d functions, %d called, %d not wrapped", len(declared), calls, len(notWrapped))
}

// The ServiceType names are those of MG 7.2 (REQ-TOP-005; SF-118:
// CloudConnector) and the Go values follow the C ABI.
func TestServiceTypeNames(t *testing.T) {
	want := []string{"+", "Registry", "OTConnector", "Utility", "Persistence",
		"Aggregation", "CloudConnector", "ITConnector"}
	for i, name := range want {
		if got := ServiceType(i).String(); got != name {
			t.Errorf("ServiceType(%d) = %s, want %s", i, got, name)
		}
	}
	header, err := os.ReadFile(filepath.Join("..", "rust", "oec-ffi", "include", "oec.h"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`#define OEC_ST_CLOUD_CONNECTOR (\d+)`).FindSubmatch(header)
	if m == nil || string(m[1]) != fmt.Sprint(int(CloudConnector)-1) {
		t.Errorf("oec.h OEC_ST_CLOUD_CONNECTOR = %q, want %d", m, int(CloudConnector)-1)
	}
}
