package image

// Image fixtures of oec-infra image (DESIGN.md §10, C-070): small OCI
// image archives built by package ocitest from the scenarios below and
// written to testdata/images (`go test ./internal/image -run
// TestFixtureFiles -update`). The expected verdicts are in
// testdata/images/index.yaml, test data with verdict for the PO review
// (G-3.9). The license texts are placeholders: the checks only look for a
// text, not at its wording.

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/OI4/oec/conformance/internal/oci/ocitest"
)

var update = flag.Bool("update", false, "rewrite the image fixtures in testdata/images")

const fixtureDir = "../../testdata/images"

func text(id string) string {
	return "Placeholder for the text of the license " + id + " (oec-infra test data).\n"
}

func appLayer(arch string) ocitest.Layer {
	return ocitest.Layer{Files: []ocitest.File{
		ocitest.Dir("app"),
		{Path: "app/run", Data: "#!/bin/sh\n# demo application for " + arch + "\n"},
	}}
}

// licLayer is a layer with /opt/oi4/licenses and the given files (name →
// content; a value starting with "->" is a symlink target).
func licLayer(gzip bool, files map[string]string) ocitest.Layer {
	l := ocitest.Layer{Gzip: gzip, Files: []ocitest.File{ocitest.Dir("opt"), ocitest.Dir("opt/oi4"), ocitest.Dir("opt/oi4/licenses")}}
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		p := n
		if !strings.Contains(n, "/") {
			p = "opt/oi4/licenses/" + n
		}
		if target, ok := strings.CutPrefix(files[n], "->"); ok {
			l.Files = append(l.Files, ocitest.Symlink(p, target))
		} else {
			l.Files = append(l.Files, ocitest.File{Path: p, Data: files[n]})
		}
	}
	return l
}

type pkg struct{ name, concluded, declared string }

func spdx(pkgs []pkg, extracted map[string]string) string {
	var b strings.Builder
	b.WriteString(`{"spdxVersion":"SPDX-2.3","dataLicense":"CC0-1.0","SPDXID":"SPDXRef-DOCUMENT","name":"oec-demo-app",` +
		`"documentNamespace":"https://example.com/spdx/oec-demo-app-1.0.0","creationInfo":{"created":"2026-01-01T00:00:00Z","creators":["Tool: ocitest"]},"packages":[`)
	for i, p := range pkgs {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"name":%q,"SPDXID":"SPDXRef-Package-%d","downloadLocation":"NOASSERTION","licenseConcluded":%q,"licenseDeclared":%q}`, p.name, i, p.concluded, p.declared)
	}
	b.WriteString("]")
	if len(extracted) > 0 {
		b.WriteString(`,"hasExtractedLicensingInfos":[`)
		ids := make([]string, 0, len(extracted))
		for id := range extracted {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for i, id := range ids {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, `{"licenseId":%q,"extractedText":%q}`, id, extracted[id])
		}
		b.WriteString("]")
	}
	b.WriteString("}\n")
	return b.String()
}

var goodPkgs = []pkg{
	{"oec-demo-app", "Apache-2.0", "Apache-2.0"},
	{"libfoo", "MIT OR BSD-3-Clause", "MIT OR BSD-3-Clause"},
	{"acme-sdk", "LicenseRef-acme", "LicenseRef-acme"},
}

var goodExtracted = map[string]string{"LicenseRef-acme": text("LicenseRef-acme")}

// goodLicenses: Apache-2.0.txt is a symlink into /usr/share (resolved
// inside the image), LicenseRef-acme is embedded in the SBOM.
func goodLicenses() map[string]string {
	return map[string]string{
		"sbom.spdx.json":                       spdx(goodPkgs, goodExtracted),
		"MIT.txt":                              text("MIT"),
		"BSD-3-Clause.txt":                     text("BSD-3-Clause"),
		"Apache-2.0.txt":                       "->/usr/share/common-licenses/Apache-2.0",
		"usr/share/common-licenses/Apache-2.0": text("Apache-2.0"),
	}
}

func with(m map[string]string, kv ...string) map[string]string {
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] == "" {
			delete(m, kv[i])
		} else {
			m[kv[i]] = kv[i+1]
		}
	}
	return m
}

func images(lic func() ocitest.Layer, plats ...string) []ocitest.Image {
	var out []ocitest.Image
	for _, p := range plats {
		parts := strings.Split(p, "/")
		img := ocitest.Image{OS: parts[0], Arch: parts[1], Layers: []ocitest.Layer{appLayer(parts[1])}}
		if len(parts) > 2 {
			img.Variant = parts[2]
		}
		if lic != nil {
			img.Layers = append(img.Layers, lic())
		}
		out = append(out, img)
	}
	return out
}

func multi(lic func() ocitest.Layer) []ocitest.Image {
	return images(lic, "linux/amd64", "linux/arm64/v8")
}

func good() ocitest.Layer { return licLayer(true, goodLicenses()) }

func cdx() string {
	return `{"bomFormat":"CycloneDX","specVersion":"1.6","version":1,` +
		`"metadata":{"component":{"type":"application","name":"oec-demo-app","licenses":[{"license":{"id":"Apache-2.0","text":{"content":` + fmt.Sprintf("%q", text("Apache-2.0")) + `}}}]}},` +
		`"components":[` +
		`{"type":"library","name":"libbar","licenses":[{"expression":"MIT OR ISC"}]},` +
		`{"type":"library","name":"vendor-lib","licenses":[{"license":{"name":"Vendor EULA","text":{"content":` + fmt.Sprintf("%q", text("Vendor EULA")) + `}}}]}]}` + "\n"
}

// scenarios are the image fixtures by file name.
func scenarios() map[string]ocitest.Spec {
	return map[string]ocitest.Spec{
		// Positive: multi-architecture OCI index with an attestation
		// manifest, SPDX 2.3, texts as files, a symlink and an embedded
		// LicenseRef text.
		"good.tar": {Multi: true, Images: append(multi(good), ocitest.Image{Attestation: true, Layers: []ocitest.Layer{{Files: []ocitest.File{{Path: "sbom.spdx.json", Data: "{}"}}}}})},
		// Positive: Docker v2 schema 2 media types (manifest list), ARMv7 and
		// amd64, CycloneDX 1.6 with embedded texts and an expression.
		"cdx-docker.tar": {Multi: true, Docker: true, Images: images(func() ocitest.Layer {
			return licLayer(true, map[string]string{"sbom.cdx.json": cdx(), "MIT.txt": text("MIT"), "ISC.txt": text("ISC")})
		}, "linux/amd64", "linux/arm/v7")},
		// The application layer of arm64 does not match its digest.
		"bad-digest.tar": {Multi: true, Images: multi(good), Blobs: func(b map[string][]byte) {
			orig := ocitest.LayerTar(appLayer("arm64"))
			changed := ocitest.LayerTar(appLayer("ARM64"))
			p := ocitest.BlobPath(ocitest.Digest("sha256", orig))
			if _, ok := b[p]; !ok || len(orig) != len(changed) {
				panic("bad-digest: layer not found")
			}
			b[p] = changed
		}},
		// Every digest uses sha512.
		"sha512.tar": {Multi: true, Algorithm: "sha512", Images: multi(good)},
		// One platform, index.json lists the manifest directly.
		"single-arch.tar": {Images: images(good, "linux/amd64")},
		// Only platforms RIG 5.1 does not support.
		"unsupported-arch.tar": {Multi: true, Images: images(good, "linux/386", "linux/riscv64")},
		// No /opt/oi4/licenses.
		"no-sbom.tar": {Multi: true, Images: multi(nil)},
		// Both SBOM files.
		"two-sboms.tar": {Multi: true, Images: multi(func() ocitest.Layer {
			return licLayer(true, with(goodLicenses(), "sbom.cdx.json", cdx(), "ISC.txt", text("ISC")))
		})},
		// BSD-3-Clause.txt is missing.
		"missing-license-text.tar": {Multi: true, Images: multi(func() ocitest.Layer {
			return licLayer(true, with(goodLicenses(), "BSD-3-Clause.txt", ""))
		})},
		// A later layer deletes the SBOM (whiteout).
		"sbom-whiteout.tar": {Multi: true, Images: appendLayer(multi(good),
			ocitest.Layer{Files: []ocitest.File{ocitest.Whiteout("opt/oi4/licenses/sbom.spdx.json")}})},
		// MIT.txt is a symlink to a host path that is not in the image.
		"dangling-symlink.tar": {Multi: true, Images: multi(func() ocitest.Layer {
			return licLayer(true, with(goodLicenses(), "MIT.txt", "->/host/licenses/MIT.txt"))
		})},
		// sbom.spdx.json holds an SPDX 3.0 JSON-LD document (SF-114 (a)).
		"invalid-sbom.tar": {Multi: true, Images: multi(func() ocitest.Layer {
			return licLayer(true, with(goodLicenses(), "sbom.spdx.json",
				`{"@context":"https://spdx.org/rdf/3.0.1/spdx-context.jsonld","@graph":[{"type":"SpdxDocument","spdxId":"https://example.com/doc"}]}`+"\n"))
		})},
		// The image declares VOLUME /opt/oi4.
		"sbom-volume.tar": {Multi: true, Images: volumes(multi(good), "/opt/oi4")},
	}
}

func volumes(imgs []ocitest.Image, v ...string) []ocitest.Image {
	for i := range imgs {
		imgs[i].Volumes = v
	}
	return imgs
}

func appendLayer(imgs []ocitest.Image, l ocitest.Layer) []ocitest.Image {
	for i := range imgs {
		imgs[i].Layers = append(imgs[i].Layers, l)
	}
	return imgs
}

func TestFixtureFiles(t *testing.T) {
	sc := scenarios()
	if *update {
		old, _ := filepath.Glob(filepath.Join(fixtureDir, "*.tar"))
		for _, f := range old {
			if err := os.Remove(f); err != nil {
				t.Fatal(err)
			}
		}
	}
	for name, spec := range sc {
		data := ocitest.Build(spec)
		p := filepath.Join(fixtureDir, name)
		if *update {
			if err := os.WriteFile(p, data, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		have, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v (run with -update)", name, err)
		}
		if !bytes.Equal(have, data) {
			t.Errorf("%s differs from its scenario (run go test ./internal/image -run TestFixtureFiles -update)", name)
		}
	}
	files, _ := filepath.Glob(filepath.Join(fixtureDir, "*.tar"))
	for _, f := range files {
		if _, ok := sc[filepath.Base(f)]; !ok {
			t.Errorf("%s has no scenario", f)
		}
	}
}

// TestFixturesDeterministic: building a scenario twice gives the same bytes.
func TestFixturesDeterministic(t *testing.T) {
	for name, spec := range scenarios() {
		if !bytes.Equal(ocitest.Build(spec), ocitest.Build(spec)) {
			t.Errorf("%s is not deterministic", name)
		}
	}
}
