// Package image holds the rules of oec-infra image (catalog d8.yaml,
// T8-IMG-01 … 04; DESIGN.md §9): it reads an OCI image offline with
// package oci and turns what it finds into observations for the verdict
// package.
package image

import (
	"archive/tar"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/OI4/oec/conformance/catalog"
	"github.com/OI4/oec/conformance/internal/oci"
	"github.com/OI4/oec/conformance/internal/sbom"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// Test ids.
const (
	TOCI       = "T8-IMG-01"
	TPlatform  = "T8-IMG-02"
	TSBOM      = "T8-IMG-03"
	TIntegrity = "T8-IMG-04"
)

// Implemented lists the tests this package implements; the catalog status
// must match (TestCatalogStatus).
var Implemented = map[string]bool{TOCI: true, TPlatform: true, TSBOM: true, TIntegrity: true}

// LicenseDir is the SBOM folder of RIG 4.1.4.
const LicenseDir = "/opt/oi4/licenses"

// Requirements of the rules.
const (
	reqOCI       = "REQ-CMP-027"
	reqApp001    = "REQ-APP-001"
	reqApp002    = "REQ-APP-002"
	reqCmp032    = "REQ-CMP-032"
	reqCmp028    = "REQ-CMP-028"
	reqCnt013    = "REQ-CNT-013"
	reqCnt014    = "REQ-CNT-014"
	reqCnt015    = "REQ-CNT-015"
	reqLic002    = "REQ-LIC-002"
	reqIntegrity = "REQ-CMP-030"
)

// Supported reports whether a platform is one of the processor
// architectures of RIG 5.1 (ARMv7, ARMv8, amd64) with the operating system
// Linux, and its canonical name.
func Supported(p oci.Platform) (string, bool) {
	if p.OS != "linux" {
		return "", false
	}
	switch {
	case p.Architecture == "amd64" && (p.Variant == "" || p.Variant == "v1"):
		return "linux/amd64", true
	case p.Architecture == "arm64" && (p.Variant == "" || p.Variant == "v8"):
		return "linux/arm64/v8", true
	case p.Architecture == "arm" && (p.Variant == "" || p.Variant == "v7"):
		return "linux/arm/v7", true
	}
	return "", false
}

// Tests returns the image tests of the D8 catalog for verdict.Build.
func Tests(cat *catalog.Catalog) []verdict.Test {
	var out []verdict.Test
	for _, t := range cat.Tests {
		if t.Mode == "image" {
			out = append(out, verdict.Test{ID: t.ID, Title: t.Title, Status: t.Status, Level: t.Level, Reqs: t.Reqs})
		}
	}
	return out
}

// Result of a check.
type Result struct {
	Image        *oci.Image
	Observations []verdict.Observation
}

type checker struct {
	l   *oci.Layout
	img *oci.Image
	obs []verdict.Observation
}

func (c *checker) add(test, req string, kind verdict.Kind, image, where, format string, args ...any) {
	c.obs = append(c.obs, verdict.Observation{Test: test, Req: req, Kind: kind,
		Evidence: verdict.Evidence{Image: image, Path: where, Detail: fmt.Sprintf(format, args...)}})
}

func (c *checker) skip(test string, reqs []string, inconclusive bool, image, detail string) {
	for _, r := range reqs {
		c.obs = append(c.obs, verdict.Observation{Test: test, Req: r, Kind: verdict.Skipped, Inconclusive: inconclusive,
			Evidence: verdict.Evidence{Image: image, Detail: detail}})
	}
}

// Check reads the image at src (OCI layout directory or oci-archive) and
// returns the observations of T8-IMG-01 … 04. An error means src is no
// readable image layout (exit code 2).
func Check(src string) (*Result, error) {
	l, err := oci.Open(src, func(p string) bool { return strings.HasPrefix(p, LicenseDir+"/") })
	if err != nil {
		return nil, err
	}
	defer l.Close()
	img, err := l.Read()
	if err != nil {
		return nil, err
	}
	c := &checker{l: l, img: img}
	c.oci()
	c.integrity()
	c.platforms()
	c.sbom()
	return &Result{Image: img, Observations: c.obs}, nil
}

// T8-IMG-01: the layout, its documents and blobs follow the OCI image
// specification (REQ-CMP-027).
func (c *checker) oci() {
	if len(c.img.Problems) == 0 {
		c.add(TOCI, reqOCI, verdict.Checked, "", "", "OCI image layout, index, manifests, configs and layers are valid")
		return
	}
	for _, p := range c.img.Problems {
		c.add(TOCI, reqOCI, verdict.Violation, p.Image, p.Where, "%s", p.Detail)
	}
}

// T8-IMG-04: digests (RIG 4.3 recommends SHA256 digests; REQ-CMP-030 keeps
// the recommendation a recommendation at level 3, so a finding only warns).
func (c *checker) integrity() {
	bad := false
	for _, p := range c.img.Problems {
		if p.Kind == oci.Integrity {
			bad = true
			c.add(TIntegrity, reqIntegrity, verdict.Warning, p.Image, p.Where, "integrity: %s", p.Detail)
		}
	}
	for _, a := range c.img.Algorithms {
		if a != "sha256" {
			bad = true
			c.add(TIntegrity, reqIntegrity, verdict.Warning, "", "", "digests use %s; RIG 4.3 recommends SHA256", a)
		}
	}
	if !bad {
		c.add(TIntegrity, reqIntegrity, verdict.Checked, "", "", "every blob is addressed by a verified SHA256 digest")
	}
}

// T8-IMG-02: processor architectures (RIG 5.1).
func (c *checker) platforms() {
	reqs := []string{reqApp001, reqCmp032, reqApp002}
	run := c.img.Runnable()
	if len(run) == 0 {
		c.skip(TPlatform, reqs, true, "", "the image holds no readable platform image (see T8-IMG-01)")
		return
	}
	supported := map[string]bool{}
	var all []string
	for _, m := range run {
		all = append(all, m.Platform.String())
		if name, ok := Supported(m.Platform); ok {
			supported[name] = true
		}
	}
	sort.Strings(all)
	var names []string
	for n := range supported {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) == 0 {
		for _, r := range []string{reqApp001, reqCmp032} {
			c.add(TPlatform, r, verdict.Violation, "", "", "no image for a supported platform (linux on ARMv7, ARMv8 or amd64); the image has %s", strings.Join(all, ", "))
		}
	} else {
		for _, r := range []string{reqApp001, reqCmp032} {
			c.add(TPlatform, r, verdict.Checked, "", "", "supported platforms: %s", strings.Join(names, ", "))
		}
	}
	if len(names) < 2 {
		c.add(TPlatform, reqApp002, verdict.Violation, "", "", "no multi-architecture image: %d supported platform(s) (%s)", len(names), strings.Join(all, ", "))
	} else {
		c.add(TPlatform, reqApp002, verdict.Checked, "", "", "multi-architecture image: %s", strings.Join(names, ", "))
	}
}

// T8-IMG-03: SBOM and license texts (RIG 4.1.4).
func (c *checker) sbom() {
	all := []string{reqCmp028, reqCnt013, reqCnt014, reqCnt015, reqLic002}
	run := c.img.Runnable()
	if len(run) == 0 {
		c.skip(TSBOM, all, true, "", "the image holds no readable platform image (see T8-IMG-01)")
		return
	}
	done := map[string]bool{}
	for _, m := range run {
		key := layersKey(m)
		if done[key] { // the same layers: the same file system
			continue
		}
		done[key] = true
		c.sbomOf(m, all)
	}
}

func layersKey(m *oci.Manifest) string {
	var b strings.Builder
	for _, l := range m.Layers {
		b.WriteString(l.Digest + ",")
	}
	return b.String()
}

func (c *checker) sbomOf(m *oci.Manifest, all []string) {
	plat := m.Platform.String()
	tree := c.l.Tree(m)
	if tree.Incomplete != "" {
		c.skip(TSBOM, all, true, plat, "the file system cannot be read: "+tree.Incomplete)
		return
	}
	notChecked := func(detail string) { c.skip(TSBOM, []string{reqCnt014, reqLic002}, false, plat, detail) }
	dir, err := tree.Resolve(LicenseDir)
	if err != nil {
		if !errors.Is(err, oci.ErrNotExist) || isLink(tree, LicenseDir) {
			c.add(TSBOM, reqCnt015, verdict.Violation, plat, LicenseDir, "%s cannot be resolved inside the image: %v", LicenseDir, err)
		}
		for _, r := range []string{reqCmp028, reqCnt013} {
			c.add(TSBOM, r, verdict.Violation, plat, LicenseDir, "no SBOM: %s does not exist in the image", LicenseDir)
		}
		notChecked("no SBOM in the image")
		return
	}
	if n, _ := tree.Stat(dir); n == nil || !n.IsDir() {
		for _, r := range []string{reqCmp028, reqCnt013} {
			c.add(TSBOM, r, verdict.Violation, plat, LicenseDir, "no SBOM: %s is no directory", LicenseDir)
		}
		notChecked("no SBOM in the image")
		return
	}
	c.volumes(m, plat)
	if c.links(tree, dir, plat) {
		c.add(TSBOM, reqCnt015, verdict.Checked, plat, LicenseDir, "the content of %s is part of the image", LicenseDir)
	}
	var found []string
	for _, name := range []string{sbom.SPDXFile, sbom.CycloneDXFile} {
		p := path.Join(LicenseDir, name)
		if _, err := tree.Lstat(p); err == nil {
			found = append(found, name)
		}
	}
	switch len(found) {
	case 0:
		for _, r := range []string{reqCmp028, reqCnt013} {
			c.add(TSBOM, r, verdict.Violation, plat, LicenseDir, "no SBOM: neither %s nor %s in %s", sbom.SPDXFile, sbom.CycloneDXFile, LicenseDir)
		}
		notChecked("no SBOM in the image")
		return
	case 2:
		c.add(TSBOM, reqCmp028, verdict.Checked, plat, LicenseDir, "the image has an SBOM")
		c.add(TSBOM, reqCnt013, verdict.Violation, plat, LicenseDir, "both %s and %s: exactly one SBOM is allowed", sbom.SPDXFile, sbom.CycloneDXFile)
		notChecked("two SBOMs: the license texts are not checked")
		return
	}
	file := path.Join(LicenseDir, found[0])
	data, err := tree.ReadFile(file)
	if err != nil {
		for _, r := range []string{reqCmp028, reqCnt013} {
			c.add(TSBOM, r, verdict.Violation, plat, file, "the SBOM cannot be read from the image: %v", err)
		}
		notChecked("the SBOM cannot be read")
		return
	}
	doc, err := sbom.Parse(found[0], data)
	if err != nil {
		for _, r := range []string{reqCmp028, reqCnt013} {
			c.add(TSBOM, r, verdict.Violation, plat, file, "%v", err)
		}
		notChecked("the SBOM is invalid")
		return
	}
	for _, r := range []string{reqCmp028, reqCnt013} {
		c.add(TSBOM, r, verdict.Checked, plat, file, "%s %s SBOM with %d packages and %d licenses", doc.Format, doc.Version, doc.Packages, len(doc.Licenses))
	}
	c.texts(tree, doc, plat, file)
}

func isLink(t *oci.Tree, p string) bool {
	n, err := t.Lstat(p)
	return err == nil && n.Type == tar.TypeSymlink
}

// volumes: a VOLUME on the folder or above makes it a mount point that a
// host mount could replace (RIG 4.1.4 forbids overlaying it from the host).
func (c *checker) volumes(m *oci.Manifest, plat string) {
	if m.Config == nil {
		return
	}
	var vols []string
	for v := range m.Config.Config.Volumes {
		v = path.Clean("/" + v)
		if v == LicenseDir || strings.HasPrefix(LicenseDir, v+"/") || v == "/" || strings.HasPrefix(v, LicenseDir+"/") {
			vols = append(vols, v)
		}
	}
	sort.Strings(vols)
	for _, v := range vols {
		c.add(TSBOM, reqCnt015, verdict.Note, plat, v, "the image declares VOLUME %s: the platform must not overlay %s with a host mount", v, LicenseDir)
	}
}

// links: every symlink in the folder resolves inside the image.
func (c *checker) links(t *oci.Tree, dir, plat string) bool {
	ok := true
	for _, n := range t.Walk(dir) {
		if n.Type != tar.TypeSymlink {
			continue
		}
		if _, err := t.Stat(n.Path); err != nil {
			ok = false
			c.add(TSBOM, reqCnt015, verdict.Violation, plat, n.Path, "symlink to %s does not resolve inside the image: %v", n.Link, err)
		}
	}
	return ok
}

func (c *checker) texts(t *oci.Tree, doc *sbom.Doc, plat, file string) {
	missing := 0
	for _, l := range doc.Licenses {
		if l.Embedded {
			continue
		}
		if l.ID == "" {
			missing++
			for _, r := range []string{reqCnt014, reqLic002} {
				c.add(TSBOM, r, verdict.Violation, plat, file, "license %q (%s) is given only by name and has no embedded text", l.Name, l.Where)
			}
			continue
		}
		p := path.Join(LicenseDir, l.ID+".txt")
		data, err := t.ReadFile(p)
		switch {
		case err != nil:
			missing++
			for _, r := range []string{reqCnt014, reqLic002} {
				c.add(TSBOM, r, verdict.Violation, plat, p, "no text for license %s (%s): not embedded in the SBOM and no readable %s (%v)", l.ID, l.Where, path.Base(p), err)
			}
		case len(strings.TrimSpace(string(data))) == 0:
			missing++
			for _, r := range []string{reqCnt014, reqLic002} {
				c.add(TSBOM, r, verdict.Violation, plat, p, "the text of license %s is empty", l.ID)
			}
		}
	}
	if missing == 0 {
		for _, r := range []string{reqCnt014, reqLic002} {
			c.add(TSBOM, r, verdict.Checked, plat, file, "every license of the SBOM has its text (%d licenses)", len(doc.Licenses))
		}
	}
	if len(doc.Licenses) == 0 {
		c.add(TSBOM, reqLic002, verdict.Note, plat, file, "the SBOM names no license")
	}
	if n := len(doc.Unlicensed); n > 0 {
		c.add(TSBOM, reqLic002, verdict.Note, plat, file, "%d of %d packages state no license (completeness is checked manually): %s", n, doc.Packages, list(doc.Unlicensed, 5))
	}
	if len(doc.External) > 0 {
		c.add(TSBOM, reqCnt014, verdict.Note, plat, file, "license references to other SPDX documents are not checked: %s", list(doc.External, 5))
	}
}

func list(s []string, max int) string {
	if len(s) <= max {
		return strings.Join(s, ", ")
	}
	return strings.Join(s[:max], ", ") + fmt.Sprintf(", … %d more", len(s)-max)
}
