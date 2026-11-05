package sbom

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Go reads the output of "go version -m <binary>": the Go standard
// library (pkg:golang/stdlib, BSD-3-Clause) and every module dependency.
// The build information names no licenses: a module replaced by a local
// directory belongs to this workspace and gets localLicense; any other
// module is refused until its license is known (fail closed). The main
// module is the application itself and is left out.
func Go(r io.Reader, localLicense string) ([]Component, error) {
	return GoKnown(r, localLicense, nil)
}

// GoKnown is Go with the reviewed licenses of third-party modules (C-107,
// the tool image of oec/conformance): known maps "<module>@<version>" to an
// SPDX license expression (ParseGoLicenses). A module of another version
// or not in the map is refused as in Go.
func GoKnown(r io.Reader, localLicense string, known map[string]string) ([]Component, error) {
	var out []Component
	scan := bufio.NewScanner(r)
	first := true
	var dep *Component
	flush := func() error {
		if dep == nil {
			return nil
		}
		if dep.License == "" {
			dep.License = known[dep.Name+"@"+dep.Version]
		}
		if dep.License == "" {
			return fmt.Errorf("Go module %s %s: license unknown (neither a module of this workspace nor in the Go license map)", dep.Name, dep.Version)
		}
		out = append(out, *dep)
		dep = nil
		return nil
	}
	for scan.Scan() {
		line := scan.Text()
		if first {
			first = false
			i := strings.LastIndex(line, ": go")
			if i < 0 {
				return nil, fmt.Errorf("go version -m: no Go version in %q", line)
			}
			v := strings.TrimPrefix(strings.TrimSpace(line[i+2:]), "go")
			out = append(out, Component{Kind: KindGo, Name: "stdlib", Version: "go" + v,
				License: "BSD-3-Clause", Purl: "pkg:golang/stdlib@" + v})
			continue
		}
		f := strings.Split(strings.TrimPrefix(line, "\t"), "\t")
		switch f[0] {
		case "dep":
			if err := flush(); err != nil {
				return nil, err
			}
			if len(f) < 3 {
				return nil, fmt.Errorf("go version -m: bad dep line %q", line)
			}
			dep = &Component{Kind: KindGo, Name: f[1], Version: f[2], Purl: "pkg:golang/" + f[1] + "@" + f[2]}
		case "=>":
			if dep == nil || len(f) < 2 {
				return nil, fmt.Errorf("go version -m: replacement without dep: %q", line)
			}
			if strings.HasPrefix(f[1], ".") || strings.HasPrefix(f[1], "/") {
				dep.License = localLicense
			} else if len(f) >= 3 {
				dep.Name, dep.Version, dep.Purl = f[1], f[2], "pkg:golang/"+f[1]+"@"+f[2]
			}
		}
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}
	if first {
		return nil, fmt.Errorf("go version -m: empty")
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return out, nil
}

// ParseGoLicenses reads a Go license map (e.g. oec/conformance/images/
// go-licenses.json): {"comment": "…", "modules": {"<module>@<version>":
// "<SPDX expression>"}}. Every key names a version and every expression
// parses (IDs); NOASSERTION and NONE are refused.
func ParseGoLicenses(data []byte) (map[string]string, error) {
	var doc struct {
		Modules map[string]string `json:"modules"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("Go license map: %v", err)
	}
	if len(doc.Modules) == 0 {
		return nil, fmt.Errorf("Go license map: no modules")
	}
	for k, e := range doc.Modules {
		if i := strings.LastIndex(k, "@v"); i <= 0 {
			return nil, fmt.Errorf("Go license map: %q is not <module>@<version>", k)
		}
		if _, err := IDs(e); err != nil {
			return nil, fmt.Errorf("Go license map: %s: %v", k, err)
		}
	}
	return doc.Modules, nil
}
