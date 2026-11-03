package npmpolicy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// site is the documentation site (oec/docs) seen from tools/npmpolicy.
const site = "../.."

// files are the files of the site the policy reads.
var files = []string{"package.json", "package-lock.json", ".npmrc", ConfigFile}

// The committed site keeps the npm policy (ADR-0031 §5, §6: the gate
// without node and network).
func TestSite(t *testing.T) {
	pkgs, errs := Check(site)
	for _, err := range errs {
		t.Error(err)
	}
	if len(pkgs) < 1000 {
		t.Fatalf("only %d packages in the lockfile", len(pkgs))
	}
	count := map[string]int{}
	var siteOnly []string
	for _, p := range pkgs {
		count[p.License]++
		if slices.ContainsFunc(p.Licenses, func(id string) bool { return slices.Contains(SiteOnly, id) }) {
			siteOnly = append(siteOnly, p.Name+"@"+p.Version+" "+p.License)
		}
	}
	t.Logf("%d packages, licenses %v", len(pkgs), count)
	t.Logf("site-only licenses (C-098 D-1 (a)): %v", siteOnly)
}

func TestAllowed(t *testing.T) {
	for expr, want := range map[string][]string{
		"MIT":                                     {"MIT"},
		"0BSD":                                    {"0BSD"},
		"CC-BY-4.0":                               {"CC-BY-4.0"},
		"(MIT OR CC0-1.0)":                        {"MIT"},
		"(WTFPL OR MIT)":                          {"MIT"},
		"(BSD-2-Clause OR MIT OR Apache-2.0)":     {"BSD-2-Clause"},
		"(MIT AND Zlib)":                          {"MIT", "Zlib"},
		"(GPL-3.0-only OR (MIT AND ISC))":         {"MIT", "ISC"},
		"MPL-2.0":                                 nil,
		"(MIT AND GPL-3.0-only)":                  nil,
		"(GPL-2.0-only OR LGPL-3.0-only)":         nil,
		"Apache-2.0 WITH LLVM-exception":          nil,
		"(MIT OR Apache-2.0 WITH LLVM-exception)": {"MIT"},
		"":                       nil,
		"(MIT":                   nil,
		"MIT)":                   nil,
		"MIT OR":                 nil,
		"UNLICENSED":             nil,
		"SEE LICENSE IN LICENSE": nil,
		"mit":                    nil,
	} {
		got, ok := Allowed(expr)
		if ok != (want != nil) || !slices.Equal(got, want) {
			t.Errorf("Allowed(%q) = %v, %v; want %v", expr, got, ok, want)
		}
	}
}

// copySite copies the files of the policy into a temporary directory.
func copySite(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(site, f))
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(dir, f), string(data))
	}
	return dir
}

func write(t *testing.T, file, data string) {
	t.Helper()
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func editJSON(t *testing.T, file string, f func(map[string]any)) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	f(v)
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	write(t, file, string(out))
}

func editText(t *testing.T, file string, f func(string) string) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	write(t, file, f(string(data)))
}

func entries(v map[string]any) map[string]any { return v["packages"].(map[string]any) }

func pkg(v map[string]any, key string) map[string]any { return entries(v)[key].(map[string]any) }

// Every rule of the policy fails on a violation, with its own message.
func TestViolations(t *testing.T) {
	type edit func(t *testing.T, dir string)
	lock := func(f func(map[string]any)) edit {
		return func(t *testing.T, dir string) { editJSON(t, filepath.Join(dir, "package-lock.json"), f) }
	}
	manifest := func(f func(map[string]any)) edit {
		return func(t *testing.T, dir string) { editJSON(t, filepath.Join(dir, "package.json"), f) }
	}
	text := func(file string, f func(string) string) edit {
		return func(t *testing.T, dir string) { editText(t, filepath.Join(dir, file), f) }
	}
	deps := func(v map[string]any) map[string]any { return v["dependencies"].(map[string]any) }
	cases := []struct {
		name string
		edit edit
		want string
	}{
		{"caret", manifest(func(v map[string]any) { deps(v)["clsx"] = "^2.1.1" }), `clsx "^2.1.1" is not an exact release version`},
		{"prerelease-direct", manifest(func(v map[string]any) { deps(v)["clsx"] = "2.1.1-rc.1" }), `clsx "2.1.1-rc.1" is not an exact release version`},
		{"extra-dependency", manifest(func(v map[string]any) { deps(v)["lodash"] = "4.18.1" }), "dependencies [@docusaurus/core"},
		{"dev-dependency", manifest(func(v map[string]any) { v["devDependencies"] = map[string]any{"typescript": "7.0.2"} }), "devDependencies are not allowed"},
		{"install-script", manifest(func(v map[string]any) { v["scripts"].(map[string]any)["postinstall"] = "node x.js" }), "no lifecycle scripts"},
		{"overrides", manifest(func(v map[string]any) { v["overrides"] = map[string]any{"tslib": "2.8.0"} }), "overrides is not allowed"},
		{"public", manifest(func(v map[string]any) { delete(v, "private") }), "not private"},
		{"license", manifest(func(v map[string]any) { v["license"] = "Apache-2.0" }), `license "Apache-2.0", want MIT`},
		{"lockfile-v2", lock(func(v map[string]any) { v["lockfileVersion"] = 2 }), "lockfileVersion 2, want 3"},
		{"root-differs", lock(func(v map[string]any) { deps(pkg(v, ""))["react"] = "19.3.0" }), "root entry differs"},
		{"not-locked", lock(func(v map[string]any) { pkg(v, "node_modules/react")["version"] = "19.3.0" }), "react is not locked at 19.3.1"},
		{"other-registry", lock(func(v map[string]any) {
			pkg(v, "node_modules/postcss")["resolved"] = "https://registry.example/postcss/-/postcss-8.5.32.tgz"
		}), "node_modules/postcss is resolved from"},
		{"git-dependency", lock(func(v map[string]any) {
			pkg(v, "node_modules/lodash")["resolved"] = "git+ssh://git@github.com/lodash/lodash.git#abc"
		}), "node_modules/lodash is resolved from"},
		{"no-integrity", lock(func(v map[string]any) { delete(pkg(v, "node_modules/lodash"), "integrity") }), "node_modules/lodash has no sha512 integrity"},
		{"sha1", lock(func(v map[string]any) {
			pkg(v, "node_modules/lodash")["integrity"] = "sha1-2jmj7l5rSw0yVb/vlWAYkK/YBwk="
		}), "node_modules/lodash has no sha512 integrity"},
		{"range", lock(func(v map[string]any) { pkg(v, "node_modules/lodash")["version"] = "^4.18.1" }), `node_modules/lodash has no exact version ("^4.18.1")`},
		{"link", lock(func(v map[string]any) {
			entries(v)["node_modules/local"] = map[string]any{"resolved": "../local", "link": true}
		}), "node_modules/local is a link"},
		{"outside", lock(func(v map[string]any) { entries(v)["../local"] = map[string]any{"version": "1.0.0"} }), "../local is not a package of node_modules"},
		{"bundled", lock(func(v map[string]any) { pkg(v, "node_modules/lodash")["inBundle"] = true }), "node_modules/lodash is bundled"},
		{"dev-package", lock(func(v map[string]any) { pkg(v, "node_modules/lodash")["dev"] = true }), "node_modules/lodash is a dev package"},
		{"mpl", lock(func(v map[string]any) { pkg(v, "node_modules/postcss")["license"] = "MPL-2.0" }), "postcss@8.5.32: license MPL-2.0 is neither"},
		{"or-without-choice", lock(func(v map[string]any) {
			pkg(v, "node_modules/postcss")["license"] = "(GPL-3.0-only OR MPL-2.0)"
		}), "license (GPL-3.0-only OR MPL-2.0) is neither"},
		{"and-with-gpl", lock(func(v map[string]any) {
			pkg(v, "node_modules/postcss")["license"] = "(MIT AND GPL-3.0-only)"
		}), "license (MIT AND GPL-3.0-only) is neither"},
		{"exception", lock(func(v map[string]any) {
			pkg(v, "node_modules/postcss")["license"] = "Apache-2.0 WITH LLVM-exception"
		}), "license Apache-2.0 WITH LLVM-exception is neither"},
		{"no-license", lock(func(v map[string]any) { delete(pkg(v, "node_modules/postcss"), "license") }), "postcss@8.5.32: no license field and no reviewed entry"},
		{"unlicensed-new-version", lock(func(v map[string]any) { pkg(v, "node_modules/eval")["version"] = "0.1.9" }), "eval@0.1.9: no license field and no reviewed entry"},
		{"unlicensed-stale", lock(func(v map[string]any) { pkg(v, "node_modules/eval")["license"] = "MIT" }), "Unlicensed lists eval@0.1.8, which has a license field"},
		{"unlicensed-gone", lock(func(v map[string]any) { delete(entries(v), "node_modules/format") }), "Unlicensed lists format@0.2.2, which is not in the lockfile"},
		{"install-script-new", lock(func(v map[string]any) { pkg(v, "node_modules/postcss")["hasInstallScript"] = true }), "node_modules/postcss has an install script that is not reviewed"},
		{"alias-new", lock(func(v map[string]any) { pkg(v, "node_modules/postcss")["name"] = "evil-postcss" }), "node_modules/postcss is an alias of evil-postcss"},
		{"alias-changed", lock(func(v map[string]any) {
			e := pkg(v, "node_modules/react-loadable")
			e["name"] = "@evil/react-loadable"
			e["resolved"] = "https://registry.npmjs.org/@evil/react-loadable/-/react-loadable-6.0.0.tgz"
		}), "node_modules/react-loadable is an alias of @evil/react-loadable"},
		{"npmrc-scripts", text(".npmrc", func(s string) string { return strings.Replace(s, "ignore-scripts=true", "ignore-scripts=false", 1) }), `.npmrc: ignore-scripts="false"`},
		{"npmrc-registry", text(".npmrc", func(s string) string {
			return strings.Replace(s, "registry=https://registry.npmjs.org/", "registry=https://registry.example/", 1)
		}), `.npmrc: registry="https://registry.example/"`},
		{"npmrc-scope", text(".npmrc", func(s string) string { return s + "@evil:registry=https://registry.example/\n" }), ".npmrc: @evil:registry is not allowed"},
		{"npmrc-token", text(".npmrc", func(s string) string { return s + "//registry.npmjs.org/:_authToken=x\n" }), ".npmrc: //registry.npmjs.org/:_authToken is not allowed"},
		{"config-warn", text(ConfigFile, func(s string) string {
			return strings.Replace(s, "onBrokenAnchors: 'throw'", "onBrokenAnchors: 'warn'", 1)
		}), `"onBrokenAnchors: 'throw'" is missing`},
		{"config-blog", text(ConfigFile, func(s string) string { return strings.Replace(s, "blog: false", "blog: {}", 1) }), `"blog: false" is missing`},
		{"config-analytics", text(ConfigFile, func(s string) string { return s + "// gtag: {trackingID: 'G-1'}\n" }), `"gtag" is not allowed`},
		{"config-guideline", text(ConfigFile, func(s string) string { return s + "// https://oi4.github.io/x\n" }), `"oi4.github.io" is not allowed`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := copySite(t)
			if _, errs := Check(dir); len(errs) != 0 {
				t.Fatalf("the copy fails already: %v", errs)
			}
			c.edit(t, dir)
			_, errs := Check(dir)
			for _, err := range errs {
				if strings.Contains(err.Error(), c.want) {
					return
				}
			}
			t.Errorf("no error with %q in %v", c.want, errs)
		})
	}
}

// A missing file of the policy is an error, not a pass.
func TestMissingFiles(t *testing.T) {
	for _, f := range files {
		t.Run(f, func(t *testing.T) {
			dir := copySite(t)
			if err := os.Remove(filepath.Join(dir, f)); err != nil {
				t.Fatal(err)
			}
			if _, errs := Check(dir); len(errs) == 0 {
				t.Errorf("no error without %s", f)
			}
		})
	}
}
