package catalog

import (
	"os"
	"regexp"
	"testing"
)

// TestIndependence guards Q-009/ADR-0006: the compliance tools share only the
// data modules oec/spec and oec/schema with the SDKs, never SDK code. The
// check sits here because the catalog is the one package both tools share.
func TestIndependence(t *testing.T) {
	raw, err := os.ReadFile("../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"github.com/OI4/oec/spec": true, "github.com/OI4/oec/schema": true}
	for _, m := range regexp.MustCompile(`github\.com/OI4/oec/[A-Za-z0-9_./-]+`).FindAllString(string(raw), -1) {
		if m == "github.com/OI4/oec/conformance" {
			continue
		}
		if !allowed[m] {
			t.Errorf("go.mod references %s; D7/D8 may depend only on oec/spec and oec/schema", m)
		}
	}
}
