package oec

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestLinkFlags: link.go links the library of oec-buildlib and the system
// libraries of every platform the SDK supports (ADR-0027 §2; Windows amd64
// with MinGW from C-105).
func TestLinkFlags(t *testing.T) {
	src, err := os.ReadFile("link.go")
	if err != nil {
		t.Fatal(err)
	}
	flags := map[string]string{}
	for _, m := range regexp.MustCompile(`(?m)^#cgo !oec_external,([a-z0-9,]+) LDFLAGS: (.*)$`).FindAllStringSubmatch(string(src), -1) {
		flags[m[1]] = m[2]
	}
	for _, p := range []string{"darwin,arm64", "darwin,amd64", "linux,amd64", "linux,arm64", "linux,arm", "windows,amd64"} {
		want := "-L${SRCDIR}/lib/" + strings.ReplaceAll(p, ",", "_") + " -loec_ffi"
		if flags[p] != want {
			t.Errorf("%s: LDFLAGS %q, want %q", p, flags[p], want)
		}
	}
	// The system libraries: rustc --print native-static-libs; on Windows
	// (MinGW) also winpthreads, which aws-lc uses there and rustc does not
	// print (static, no libwinpthread-1.dll at run time).
	system := map[string][]string{
		"darwin":  {"-liconv", "-lc", "-lm"},
		"linux":   {"-lgcc_s", "-lutil", "-lrt", "-lpthread", "-lm", "-ldl", "-lc"},
		"windows": {"-lbcrypt", "-lkernel32", "-lntdll", "-luserenv", "-lws2_32", "-ldbghelp", "-l:libpthread.a"},
	}
	for goos, libs := range system {
		if got := strings.Fields(flags[goos]); strings.Join(got, " ") != strings.Join(libs, " ") {
			t.Errorf("%s: system libraries %q, want %q", goos, got, libs)
		}
	}
}
