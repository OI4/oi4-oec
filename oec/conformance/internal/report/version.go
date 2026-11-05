package report

import (
	"regexp"
	"runtime/debug"
	"strings"
)

var (
	// semVer: a Go module version vMAJOR.MINOR.PATCH[-pre-release], without
	// build metadata (+dirty, +incompatible).
	semVer = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$`)
	// pseudo: the end of a Go pseudo-version (time and commit).
	pseudo = regexp.MustCompile(`[-.][0-9]{14}-[0-9a-f]{12}$`)
)

// ToolVersion is the version a tool reports (key version of a report): the
// version stamped by the release build (-X main.version, ADR-0033), or else
// the module version of a binary installed with
// go install …/cmd/<tool>@v<version> (C-110), or else stamped ("dev"). A
// pseudo-version (a local build in a git checkout) counts as no version.
func ToolVersion(stamped string) string {
	return toolVersion(stamped, debug.ReadBuildInfo)
}

func toolVersion(stamped string, read func() (*debug.BuildInfo, bool)) string {
	if stamped != "dev" {
		return stamped
	}
	info, ok := read()
	if !ok || info == nil {
		return stamped
	}
	v := info.Main.Version
	if !semVer.MatchString(v) || pseudo.MatchString(v) {
		return stamped
	}
	return strings.TrimPrefix(v, "v")
}
