// Package npmpolicy is the offline npm policy of the documentation site
// (ADR-0031 §5, §6; the rules of ADR-0029 §10 with A-493): package.json
// private and MIT with exact versions, the allowed dependencies and scripts
// only; package-lock.json v3 with every package resolved from
// registry.npmjs.org with a sha512 integrity; every license in the
// allowlist of ADR-0023 or in the site-only list of the PO (C-098 D-1 (a)),
// an OR expression with one allowed choice, a package without a license
// field only with a reviewed table entry; install scripts and npm aliases
// only as reviewed; .npmrc with the registry and ignore-scripts; and the
// build rules of docusaurus.config.mjs. It needs neither node nor the
// network; standard library only.
package npmpolicy

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Registry is the only source of packages.
const Registry = "https://registry.npmjs.org/"

// Allowlist is the license allowlist of ADR-0023 §2.
var Allowlist = []string{"MIT", "MIT-0", "Apache-2.0", "ISC", "BSD-2-Clause", "BSD-3-Clause", "Zlib", "Unicode-3.0"}

// SiteOnly are the licenses the PO allowed for the npm tree of the
// documentation site only (C-098 D-1 (a), ADR-0031 §5); not for the SDKs,
// the registry or the demos.
var SiteOnly = []string{"0BSD", "CC0-1.0", "BlueOak-1.0.0", "Python-2.0", "CC-BY-4.0"}

// Dependencies are the direct dependencies of package.json (ADR-0031 §4,
// A-492); the site has no devDependencies.
var Dependencies = []string{
	"@docusaurus/core", "@docusaurus/preset-classic", "@mdx-js/react", "clsx",
	"prism-react-renderer", "react", "react-dom",
}

// Scripts are the scripts of package.json (no lifecycle script).
var Scripts = map[string]string{
	"build": "docusaurus build",
	"serve": "docusaurus serve",
	"start": "docusaurus start",
}

// Reviewed is a license stated outside the license field.
type Reviewed struct{ License, Where string }

// Unlicensed is the reviewed license of a package without a license field
// in the lockfile (A-493), by name@version.
var Unlicensed = map[string]Reviewed{
	"eval@0.1.8":         {"MIT", "legacy licenses array of package.json; LICENSE"},
	"format@0.2.2":       {"MIT", "legacy licenses array of package.json; Readme.md"},
	"require-like@0.1.2": {"MIT", "License file (the MIT text)"},
}

// InstallScripts are the packages with an install script and why the site
// needs none of them: npm never runs it (.npmrc, build.sh: ignore-scripts).
var InstallScripts = map[string]string{
	"core-js":  "postinstall prints a funding message",
	"fsevents": "binding.gyp (node-gyp rebuild); optional, darwin only, ships fsevents.node",
}

// Aliases are the npm aliases of the tree: the name in node_modules and
// the package actually installed.
var Aliases = map[string]string{
	"react-helmet-async": "@slorber/react-helmet-async",
	"react-loadable":     "@docusaurus/react-loadable",
}

// Npmrc are the settings .npmrc must hold.
var Npmrc = map[string]string{"registry": Registry, "ignore-scripts": "true"}

// ConfigFile is the Docusaurus configuration.
const ConfigFile = "docusaurus.config.mjs"

// ConfigRequired must be in ConfigFile (ADR-0031 §3, §6: every broken link
// fails the build, no blog); ConfigForbidden must not (analytics, a search
// service, the guideline site – G-4.11).
var (
	ConfigRequired  = []string{"onBrokenLinks: 'throw'", "onBrokenAnchors: 'throw'", "onBrokenMarkdownLinks: 'throw'", "blog: false"}
	ConfigForbidden = []string{"gtag", "googleAnalytics", "googleTagManager", "algolia", "oi4.github.io", "oi4-oec-documentation"}
)

const sha512Size = 64

var (
	release = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	semver  = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$`)
)

type manifest struct {
	Name                 string            `json:"name"`
	Version              string            `json:"version"`
	Private              bool              `json:"private"`
	License              string            `json:"license"`
	Scripts              map[string]string `json:"scripts"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies json.RawMessage   `json:"optionalDependencies"`
	PeerDependencies     json.RawMessage   `json:"peerDependencies"`
	BundleDependencies   json.RawMessage   `json:"bundleDependencies"`
	BundledDependencies  json.RawMessage   `json:"bundledDependencies"`
	Overrides            json.RawMessage   `json:"overrides"`
	Workspaces           json.RawMessage   `json:"workspaces"`
}

type lockEntry struct {
	Name             string            `json:"name"`
	Version          string            `json:"version"`
	Resolved         string            `json:"resolved"`
	Integrity        string            `json:"integrity"`
	License          json.RawMessage   `json:"license"`
	Dev              bool              `json:"dev"`
	Link             bool              `json:"link"`
	InBundle         bool              `json:"inBundle"`
	HasInstallScript bool              `json:"hasInstallScript"`
	Dependencies     map[string]string `json:"dependencies"`
	DevDependencies  map[string]string `json:"devDependencies"`
}

type lockfile struct {
	Name            string               `json:"name"`
	Version         string               `json:"version"`
	LockfileVersion int                  `json:"lockfileVersion"`
	Packages        map[string]lockEntry `json:"packages"`
}

// Package is one package of the lockfile.
type Package struct {
	// Name is the name in node_modules (the alias of an aliased package).
	Name, Version string
	// License is the SPDX expression of the lockfile or of Unlicensed.
	License string
	// Licenses are the identifiers of the allowed choice.
	Licenses []string
}

func readJSON(file string, v any) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(file), err)
	}
	return nil
}

func allowed(id string) bool { return slices.Contains(Allowlist, id) || slices.Contains(SiteOnly, id) }

// Allowed evaluates an SPDX license expression (A-493): an identifier is
// allowed if it is in Allowlist or SiteOnly; AND needs both sides, OR one
// of them; an exception (WITH) is not allowed (a question to the PO). It
// returns the identifiers of the allowed choice.
func Allowed(expr string) ([]string, bool) {
	p := &parser{toks: strings.Fields(strings.NewReplacer("(", " ( ", ")", " ) ").Replace(expr))}
	ids, ok := p.or()
	if p.bad || p.i != len(p.toks) || !ok {
		return nil, false
	}
	return ids, true
}

type parser struct {
	toks []string
	i    int
	bad  bool
}

func (p *parser) peek() string {
	if p.i < len(p.toks) {
		return p.toks[p.i]
	}
	return ""
}

func (p *parser) or() ([]string, bool) {
	ids, ok := p.and()
	for p.peek() == "OR" {
		p.i++
		alt, altOK := p.and()
		if !ok && altOK {
			ids, ok = alt, true
		}
	}
	return ids, ok
}

func (p *parser) and() ([]string, bool) {
	ids, ok := p.term()
	for p.peek() == "AND" {
		p.i++
		more, moreOK := p.term()
		ids, ok = append(ids, more...), ok && moreOK
	}
	return ids, ok
}

func (p *parser) term() ([]string, bool) {
	t := p.peek()
	switch t {
	case "(":
		p.i++
		ids, ok := p.or()
		if p.peek() != ")" {
			p.bad = true
			return nil, false
		}
		p.i++
		return ids, ok
	case "", ")", "OR", "AND", "WITH":
		p.bad = true
		return nil, false
	}
	p.i++
	if p.peek() == "WITH" {
		p.i += 2
		p.bad = p.bad || p.i > len(p.toks)
		return nil, false
	}
	return []string{t}, allowed(t)
}

// Check checks package.json, package-lock.json, .npmrc and the Docusaurus
// configuration in the site directory dir and returns the packages of the
// lockfile (sorted by path) and every violation.
func Check(dir string) ([]Package, []error) {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	var m manifest
	if err := readJSON(filepath.Join(dir, "package.json"), &m); err != nil {
		return nil, []error{err}
	}
	var l lockfile
	if err := readJSON(filepath.Join(dir, "package-lock.json"), &l); err != nil {
		return nil, []error{err}
	}

	if !m.Private {
		fail("package.json: not private")
	}
	if m.License != "MIT" {
		fail("package.json: license %q, want MIT (G-4.10)", m.License)
	}
	if !maps.Equal(m.Scripts, Scripts) {
		fail("package.json: scripts %v, want exactly %v (no lifecycle scripts)", m.Scripts, Scripts)
	}
	for name, raw := range map[string]json.RawMessage{
		"optionalDependencies": m.OptionalDependencies, "peerDependencies": m.PeerDependencies,
		"bundleDependencies": m.BundleDependencies, "bundledDependencies": m.BundledDependencies,
		"overrides": m.Overrides, "workspaces": m.Workspaces,
	} {
		if raw != nil {
			fail("package.json: %s is not allowed", name)
		}
	}
	if len(m.DevDependencies) != 0 {
		fail("package.json: devDependencies are not allowed (the site has runtime dependencies only)")
	}
	deps := slices.Sorted(maps.Keys(m.Dependencies))
	if !slices.Equal(deps, Dependencies) {
		fail("package.json: dependencies %v, want exactly %v (ADR-0031 §4)", deps, Dependencies)
	}
	for _, name := range deps {
		if v := m.Dependencies[name]; !release.MatchString(v) {
			fail("package.json: %s %q is not an exact release version", name, v)
		}
	}

	if l.LockfileVersion != 3 {
		fail("package-lock.json: lockfileVersion %d, want 3", l.LockfileVersion)
	}
	root, ok := l.Packages[""]
	switch {
	case !ok:
		fail("package-lock.json: no root entry")
	case l.Name != m.Name || root.Name != m.Name || l.Version != m.Version || root.Version != m.Version ||
		!maps.Equal(root.Dependencies, m.Dependencies) || len(root.DevDependencies) != 0:
		fail("package-lock.json: the root entry differs from package.json (run npm install --package-lock-only)")
	}
	for _, name := range deps {
		if e, ok := l.Packages["node_modules/"+name]; !ok || e.Version != m.Dependencies[name] {
			fail("package-lock.json: %s is not locked at %s", name, m.Dependencies[name])
		}
	}

	var pkgs []Package
	seen := map[string]bool{}
	for _, k := range slices.Sorted(maps.Keys(l.Packages)) {
		if k == "" {
			continue
		}
		e := l.Packages[k]
		if !strings.HasPrefix(k, "node_modules/") {
			fail("package-lock.json: %s is not a package of node_modules", k)
			continue
		}
		name := k[strings.LastIndex(k, "node_modules/")+len("node_modules/"):]
		key := name + "@" + e.Version
		seen[name], seen[key] = true, true
		switch {
		case e.Link:
			fail("package-lock.json: %s is a link", k)
			continue
		case e.InBundle:
			fail("package-lock.json: %s is bundled in another package", k)
			continue
		case !semver.MatchString(e.Version):
			fail("package-lock.json: %s has no exact version (%q)", k, e.Version)
			continue
		}
		if e.Dev {
			fail("package-lock.json: %s is a dev package (the site has runtime dependencies only)", k)
		}
		installed := name
		if e.Name != "" && e.Name != name {
			if Aliases[name] != e.Name {
				fail("package-lock.json: %s is an alias of %s that is not reviewed (Aliases)", k, e.Name)
			}
			installed = e.Name
		}
		want := Registry + installed + "/-/" + path.Base(installed) + "-" + e.Version + ".tgz"
		if e.Resolved != want {
			fail("package-lock.json: %s is resolved from %q, want %q", k, e.Resolved, want)
		}
		sum, ok := strings.CutPrefix(e.Integrity, "sha512-")
		if b, err := base64.StdEncoding.DecodeString(sum); !ok || err != nil || len(b) != sha512Size {
			fail("package-lock.json: %s has no sha512 integrity (%q)", k, e.Integrity)
		}
		if _, ok := InstallScripts[name]; e.HasInstallScript && !ok {
			fail("package-lock.json: %s has an install script that is not reviewed (InstallScripts)", k)
		}
		var lic string
		reviewed, inTable := Unlicensed[key]
		switch {
		case e.License != nil && json.Unmarshal(e.License, &lic) == nil && lic != "":
			if inTable {
				fail("npmpolicy: Unlicensed lists %s, which has a license field (%s)", key, lic)
			}
		case inTable:
			lic = reviewed.License
		default:
			fail("package-lock.json: %s: no license field and no reviewed entry in Unlicensed (A-493)", key)
			continue
		}
		ids, ok := Allowed(lic)
		if !ok {
			fail("package-lock.json: %s: license %s is neither in the allowlist of ADR-0023 nor site-only (ADR-0031 §5) – a question to the PO", key, lic)
		}
		pkgs = append(pkgs, Package{Name: name, Version: e.Version, License: lic, Licenses: ids})
	}
	for _, key := range slices.Sorted(maps.Keys(Unlicensed)) {
		if !seen[key] {
			fail("npmpolicy: Unlicensed lists %s, which is not in the lockfile", key)
		}
	}
	for _, name := range append(slices.Sorted(maps.Keys(InstallScripts)), slices.Sorted(maps.Keys(Aliases))...) {
		if !seen[name] {
			fail("npmpolicy: %s is reviewed but not in the lockfile", name)
		}
	}

	errs = append(errs, checkNpmrc(filepath.Join(dir, ".npmrc"))...)
	errs = append(errs, checkConfig(filepath.Join(dir, ConfigFile))...)
	return pkgs, errs
}

// checkNpmrc checks .npmrc: the settings of Npmrc, no other registry (also
// not per scope) and no credentials.
func checkNpmrc(file string) []error {
	data, err := os.ReadFile(file)
	if err != nil {
		return []error{err}
	}
	var errs []error
	got := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		got[k] = v
		if strings.HasSuffix(k, ":registry") || strings.Contains(k, "_auth") || strings.HasPrefix(k, "//") {
			errs = append(errs, fmt.Errorf(".npmrc: %s is not allowed", k))
		}
	}
	for _, k := range slices.Sorted(maps.Keys(Npmrc)) {
		if got[k] != Npmrc[k] {
			errs = append(errs, fmt.Errorf(".npmrc: %s=%q, want %q", k, got[k], Npmrc[k]))
		}
	}
	return errs
}

// checkConfig checks the text of ConfigFile for ConfigRequired and
// ConfigForbidden.
func checkConfig(file string) []error {
	data, err := os.ReadFile(file)
	if err != nil {
		return []error{err}
	}
	var errs []error
	s := string(data)
	for _, want := range ConfigRequired {
		if !strings.Contains(s, want) {
			errs = append(errs, fmt.Errorf("%s: %q is missing", ConfigFile, want))
		}
	}
	for _, bad := range ConfigForbidden {
		if strings.Contains(s, bad) {
			errs = append(errs, fmt.Errorf("%s: %q is not allowed (no analytics, no search service, G-4.11)", ConfigFile, bad))
		}
	}
	return errs
}
