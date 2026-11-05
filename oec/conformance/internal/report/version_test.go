package report

import (
	"runtime/debug"
	"testing"
)

// TestToolVersion: a version stamped by the release build wins; without it
// the version of a module installed with go install …@v<version> (C-110,
// ADR-0034); otherwise dev – also for the pseudo-versions of a local build.
func TestToolVersion(t *testing.T) {
	info := func(v string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Path: "github.com/OI4/oec/conformance", Version: v}}, true
		}
	}
	none := func() (*debug.BuildInfo, bool) { return nil, false }
	for i, c := range []struct {
		stamped string
		read    func() (*debug.BuildInfo, bool)
		want    string
	}{
		{"0.1.0", info("v0.2.0"), "0.1.0"},
		{"0.1.0-snapshot.g0123456789ab", none, "0.1.0-snapshot.g0123456789ab"},
		{"dev", info("v0.1.0"), "0.1.0"},
		{"dev", info("v1.20.3"), "1.20.3"},
		{"dev", info("v0.1.0-snapshot.g0123456789ab"), "0.1.0-snapshot.g0123456789ab"},
		{"dev", info("v0.1.0-rc.1"), "0.1.0-rc.1"},
		{"dev", none, "dev"},
		{"dev", info(""), "dev"},
		{"dev", info("(devel)"), "dev"},
		{"dev", info("v0.0.0-20261105142444-690d42a0fd5a"), "dev"},
		{"dev", info("v0.0.0-20261105142444-690d42a0fd5a+dirty"), "dev"},
		{"dev", info("v0.1.1-0.20261105142444-690d42a0fd5a"), "dev"},
		{"dev", info("v0.1.0-rc.1.0.20261105142444-690d42a0fd5a"), "dev"},
		{"dev", info("v0.1.0+dirty"), "dev"},
		{"dev", info("v2.0.0+incompatible"), "dev"},
		{"dev", info("v01.0.0"), "dev"},
		{"dev", info("0.1.0"), "dev"},
	} {
		if got := toolVersion(c.stamped, c.read); got != c.want {
			t.Errorf("case %d: toolVersion(%q) = %q, want %q", i, c.stamped, got, c.want)
		}
	}
	if got := ToolVersion("1.2.3"); got != "1.2.3" {
		t.Errorf("ToolVersion(1.2.3) = %q", got)
	}
	// A test binary has no module version.
	if got := ToolVersion("dev"); got != "dev" {
		t.Errorf("ToolVersion(dev) = %q in a test", got)
	}
}
