package conformance

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The tool image oi4/oec-conformance (ADR-0025 §10, amendment C-107),
// checked offline like the images of the demos and the registry.

var pinnedImage = regexp.MustCompile(`^[a-z0-9.-]+(?:/[a-z0-9._-]+)+:[A-Za-z0-9._-]+@sha256:[0-9a-f]{64}$`)

type instruction struct{ cmd, args string }

func dockerfileInstructions(t *testing.T, path string) []instruction {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []instruction
	var cur strings.Builder
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if cur.Len() == 0 && (line == "" || strings.HasPrefix(line, "#")) {
			continue
		}
		if strings.HasSuffix(line, "\\") {
			cur.WriteString(strings.TrimSuffix(line, "\\") + " ")
			continue
		}
		cur.WriteString(line)
		cmd, args, _ := strings.Cut(cur.String(), " ")
		out = append(out, instruction{strings.ToUpper(cmd), strings.TrimSpace(args)})
		cur.Reset()
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestImageDockerfile: the builder and base images pinned by tag and
// digest, static Go builds (no cgo) on the build platform, the SBOM by
// oec-sbom with the reviewed Go license map, a final stage of the base
// image without RUN as the non-root user 65532 with the license folder.
func TestImageDockerfile(t *testing.T) {
	ins := dockerfileInstructions(t, "Dockerfile")
	args := map[string]string{}
	stages := map[string]bool{}
	var final []instruction
	var build strings.Builder
	for _, in := range ins {
		switch in.cmd {
		case "ARG":
			if k, v, ok := strings.Cut(in.args, "="); ok && strings.HasSuffix(k, "_IMAGE") {
				if !pinnedImage.MatchString(v) {
					t.Errorf("ARG %s=%s: not pinned by tag and digest", k, v)
				}
				args[k] = v
			}
		case "FROM":
			f := strings.Fields(in.args)
			img := f[0]
			if strings.HasPrefix(img, "--platform=") {
				img = f[1]
			}
			name := strings.Trim(img, "${}")
			if _, ok := args[name]; !ok && !stages[img] {
				t.Errorf("FROM %s: neither a pinned ARG image nor an own stage", img)
			}
			if i := slices.Index(f, "AS"); i >= 0 && i+1 < len(f) {
				stages[f[i+1]] = true
			}
			final = nil
		case "RUN":
			build.WriteString(in.args + "\n")
		}
		final = append(final, in)
	}
	if !strings.HasPrefix(args["BASE_IMAGE"], "gcr.io/distroless/static-debian13:nonroot@") {
		t.Errorf("BASE_IMAGE %s, want the distroless static nonroot image", args["BASE_IMAGE"])
	}
	if final[0].args != "${BASE_IMAGE}" {
		t.Errorf("final stage FROM %s, want the base image", final[0].args)
	}
	var user, licenses bool
	for _, in := range final[1:] {
		switch {
		case in.cmd == "RUN":
			t.Errorf("RUN in the final stage: %s", in.args)
		case in.cmd == "USER":
			user = in.args == "65532:65532"
		case in.cmd == "COPY" && strings.HasSuffix(in.args, " /opt/oi4/licenses/"):
			licenses = true
		case in.cmd == "VOLUME" && strings.Contains(in.args, "/opt/oi4/licenses"):
			t.Error("a VOLUME on /opt/oi4/licenses (REQ-CNT-017)")
		}
	}
	if !user || !licenses {
		t.Errorf("final stage: USER 65532:65532 %v, licenses copied %v", user, licenses)
	}
	b := build.String()
	for _, want := range []string{"CGO_ENABLED=0", "-trimpath", "-buildid=", "-X main.version=$VERSION", "oec-check oec-infra",
		"oec-sbom -name oec-conformance", "-golicenses ../conformance/images/go-licenses.json", "-texts images/licenses"} {
		if !strings.Contains(b, want) {
			t.Errorf("build stage lacks %s", want)
		}
	}
}

// TestImageDockerignore: oec/.dockerignore lets in D7/D8 and the spec data
// they embed, not the spec tools.
func TestImageDockerignore(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", ".dockerignore"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	for _, l := range []string{"!conformance", "!spec", "spec/tools", "!schema", "!demos", "!sdk/go"} {
		if !slices.Contains(lines, l) {
			t.Errorf(".dockerignore lacks %s", l)
		}
	}
}

// TestImageBuildScript: images/build.sh runs the pinned BuildKit of the
// demos in a builder of its own, builds both platforms, judges the OCI
// archive with D8 image at level 3 and fails on any other verdict, never
// prunes; D8 run does not apply (the tools are no OEC application).
func TestImageBuildScript(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("images", "build.sh"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := os.ReadFile(filepath.Join("..", "demos", "images", "build.sh"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	re := regexp.MustCompile(`(?m)^buildkit="([^"]+)"$`)
	m, dm := re.FindStringSubmatch(s), re.FindStringSubmatch(string(d))
	if m == nil || dm == nil || m[1] != dm[1] {
		t.Fatalf("BuildKit %v: not the pinned one of the demos %v", m, dm)
	}
	for _, want := range []string{
		`--driver-opt "image=$buildkit"`, `docker buildx rm "$builder"`, `-f "$oec/conformance/Dockerfile"`,
		`--platform linux/amd64,linux/arm64`, `oec-infra" image --level 3 `, `docker image rm "$old"`, "--remove",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("build.sh lacks %s", want)
		}
	}
	if strings.Contains(s, `oec-infra" run `) {
		t.Error("build.sh runs D8 run: the tool image is no OEC application")
	}
	if regexp.MustCompile(`(?m)^[^#]*\bprune\b`).MatchString(s) {
		t.Error("build.sh must not prune")
	}
	if !regexp.MustCompile(`(?m)^\[ "\$d8" = 0 \] \|\| \{`).MatchString(s) {
		t.Error("build.sh must exit non-zero when D8 did not pass")
	}
}
