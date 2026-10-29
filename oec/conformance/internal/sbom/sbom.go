// Package sbom reads the software bill of materials of an OEC container
// image (RIG 4.1.4): SPDX 2.2/2.3 JSON or CycloneDX 1.x JSON. It returns
// the licenses the SBOM names and whether it embeds their texts, which is
// what oec-infra image needs for REQ-CNT-013/014 and REQ-LIC-002. The
// accepted versions and the text rules follow the proposed interpretation
// of SF-114 until the product owner decides it.
package sbom

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// File names of the SBOM in /opt/oi4/licenses (RIG 4.1.4).
const (
	SPDXFile      = "sbom.spdx.json"
	CycloneDXFile = "sbom.cdx.json"
)

// License is one license the SBOM names.
type License struct {
	ID string // SPDX license id or LicenseRef-…; "" for a license given only by name
	// Name of a CycloneDX license given only by name.
	Name string
	// Embedded: the SBOM carries the license text.
	Embedded bool
	// Where the license is named first (JSON pointer).
	Where string
}

// Doc is a parsed SBOM.
type Doc struct {
	Format  string // SPDX | CycloneDX
	Version string // SPDX-2.3, 1.6, …
	// Licenses by id or name, in the order of their first appearance.
	Licenses []License
	// Packages counts the packages/components; Unlicensed lists those that
	// state no license (informative).
	Packages   int
	Unlicensed []string
	// External lists license references to other SPDX documents
	// (DocumentRef-…:LicenseRef-…); their texts are not checked.
	External []string
}

type collector struct {
	doc  *Doc
	seen map[string]int
}

func (c *collector) add(l License) {
	key := l.ID
	if key == "" {
		key = "name:" + l.Name
	}
	if i, ok := c.seen[key]; ok {
		if l.Embedded {
			c.doc.Licenses[i].Embedded = true
		}
		return
	}
	c.seen[key] = len(c.doc.Licenses)
	c.doc.Licenses = append(c.doc.Licenses, l)
}

// Parse parses the SBOM file name with the content data. An error means the
// file is no SBOM of the format its name requires, or a version the
// guideline does not accept.
func Parse(name string, data []byte) (*Doc, error) {
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("%s is no JSON object: %v", name, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("%s: trailing data after the JSON object", name)
	}
	c := &collector{doc: &Doc{}, seen: map[string]int{}}
	var err error
	switch name {
	case SPDXFile:
		err = c.spdx(data)
	case CycloneDXFile:
		err = c.cyclonedx(data)
	default:
		err = fmt.Errorf("%s is no SBOM file name of RIG 4.1.4", name)
	}
	if err != nil {
		return nil, err
	}
	sort.Strings(c.doc.Unlicensed)
	return c.doc, nil
}

// ---- SPDX 2.2 / 2.3 ----

type spdxDoc struct {
	SPDXVersion       string `json:"spdxVersion"`
	DataLicense       string `json:"dataLicense"`
	SPDXID            string `json:"SPDXID"`
	Name              string `json:"name"`
	DocumentNamespace string `json:"documentNamespace"`
	CreationInfo      *struct {
		Created  string   `json:"created"`
		Creators []string `json:"creators"`
	} `json:"creationInfo"`
	Packages []struct {
		Name                 string   `json:"name"`
		SPDXID               string   `json:"SPDXID"`
		LicenseConcluded     string   `json:"licenseConcluded"`
		LicenseDeclared      string   `json:"licenseDeclared"`
		LicenseInfoFromFiles []string `json:"licenseInfoFromFiles"`
	} `json:"packages"`
	Files []struct {
		FileName           string   `json:"fileName"`
		LicenseConcluded   string   `json:"licenseConcluded"`
		LicenseInfoInFiles []string `json:"licenseInfoInFiles"`
	} `json:"files"`
	Snippets []struct {
		LicenseConcluded    string   `json:"licenseConcluded"`
		LicenseInfoInSnippe []string `json:"licenseInfoInSnippets"`
	} `json:"snippets"`
	Extracted []struct {
		LicenseID     string `json:"licenseId"`
		ExtractedText string `json:"extractedText"`
	} `json:"hasExtractedLicensingInfos"`
}

// SPDXVersions are the accepted SPDX versions (SF-114 (a), proposed).
var SPDXVersions = []string{"SPDX-2.2", "SPDX-2.3"}

func (c *collector) spdx(data []byte) error {
	var d spdxDoc
	if err := json.Unmarshal(data, &d); err != nil {
		return fmt.Errorf("%s is no SPDX JSON document: %v", SPDXFile, err)
	}
	c.doc.Format, c.doc.Version = "SPDX", d.SPDXVersion
	switch {
	case d.SPDXVersion == "":
		return fmt.Errorf("%s has no spdxVersion: no SPDX JSON document", SPDXFile)
	case !contains(SPDXVersions, d.SPDXVersion):
		return fmt.Errorf("%s: spdxVersion %q is not accepted (%s)", SPDXFile, d.SPDXVersion, strings.Join(SPDXVersions, ", "))
	}
	var missing []string
	for f, ok := range map[string]bool{
		"dataLicense": d.DataLicense != "", "SPDXID": d.SPDXID != "", "name": d.Name != "",
		"documentNamespace": d.DocumentNamespace != "",
		"creationInfo":      d.CreationInfo != nil && d.CreationInfo.Created != "" && len(d.CreationInfo.Creators) > 0,
	} {
		if !ok {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("%s lacks the SPDX document fields %s", SPDXFile, strings.Join(missing, ", "))
	}
	embedded := map[string]bool{}
	for _, e := range d.Extracted {
		if e.LicenseID != "" && strings.TrimSpace(e.ExtractedText) != "" && !strings.EqualFold(strings.TrimSpace(e.ExtractedText), "NOASSERTION") {
			embedded[e.LicenseID] = true
		}
	}
	expr := func(e, where string) error {
		ids, err := Expression(e)
		if err != nil {
			return fmt.Errorf("%s %s: %v", SPDXFile, where, err)
		}
		for _, id := range ids {
			if strings.HasPrefix(id, "DocumentRef-") {
				c.doc.External = append(c.doc.External, id)
				continue
			}
			c.add(License{ID: id, Embedded: embedded[id], Where: where})
		}
		return nil
	}
	c.doc.Packages = len(d.Packages)
	for i, p := range d.Packages {
		stated := false
		for _, f := range []struct{ name, v string }{{"licenseConcluded", p.LicenseConcluded}, {"licenseDeclared", p.LicenseDeclared}} {
			if f.v == "" {
				continue
			}
			if !isNone(f.v) {
				stated = true
			}
			if err := expr(f.v, fmt.Sprintf("/packages/%d/%s", i, f.name)); err != nil {
				return err
			}
		}
		for j, v := range p.LicenseInfoFromFiles {
			if err := expr(v, fmt.Sprintf("/packages/%d/licenseInfoFromFiles/%d", i, j)); err != nil {
				return err
			}
		}
		if !stated {
			c.doc.Unlicensed = append(c.doc.Unlicensed, nonEmpty(p.Name, p.SPDXID))
		}
	}
	for i, f := range d.Files {
		if f.LicenseConcluded != "" {
			if err := expr(f.LicenseConcluded, fmt.Sprintf("/files/%d/licenseConcluded", i)); err != nil {
				return err
			}
		}
		for j, v := range f.LicenseInfoInFiles {
			if err := expr(v, fmt.Sprintf("/files/%d/licenseInfoInFiles/%d", i, j)); err != nil {
				return err
			}
		}
	}
	for i, s := range d.Snippets {
		if s.LicenseConcluded != "" {
			if err := expr(s.LicenseConcluded, fmt.Sprintf("/snippets/%d/licenseConcluded", i)); err != nil {
				return err
			}
		}
		for j, v := range s.LicenseInfoInSnippe {
			if err := expr(v, fmt.Sprintf("/snippets/%d/licenseInfoInSnippets/%d", i, j)); err != nil {
				return err
			}
		}
	}
	return nil
}

// ---- CycloneDX 1.x ----

type cdxLicense struct {
	License *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Text *struct {
			Content string `json:"content"`
		} `json:"text"`
	} `json:"license"`
	Expression string `json:"expression"`
}

type cdxComponent struct {
	Name       string         `json:"name"`
	BOMRef     string         `json:"bom-ref"`
	Licenses   []cdxLicense   `json:"licenses"`
	Components []cdxComponent `json:"components"`
}

type cdxService struct {
	Name     string       `json:"name"`
	Licenses []cdxLicense `json:"licenses"`
	Services []cdxService `json:"services"`
}

type cdxDoc struct {
	BOMFormat   string `json:"bomFormat"`
	SpecVersion string `json:"specVersion"`
	Metadata    *struct {
		Component *cdxComponent `json:"component"`
		Licenses  []cdxLicense  `json:"licenses"`
	} `json:"metadata"`
	Components []cdxComponent `json:"components"`
	Services   []cdxService   `json:"services"`
}

var cdxVersion = regexp.MustCompile(`^1\.[0-9]+$`)

func (c *collector) cyclonedx(data []byte) error {
	var d cdxDoc
	if err := json.Unmarshal(data, &d); err != nil {
		return fmt.Errorf("%s is no CycloneDX JSON document: %v", CycloneDXFile, err)
	}
	c.doc.Format, c.doc.Version = "CycloneDX", d.SpecVersion
	if d.BOMFormat != "CycloneDX" {
		return fmt.Errorf("%s: bomFormat %q, expected \"CycloneDX\"", CycloneDXFile, d.BOMFormat)
	}
	if !cdxVersion.MatchString(d.SpecVersion) {
		return fmt.Errorf("%s: specVersion %q is not accepted (1.x)", CycloneDXFile, d.SpecVersion)
	}
	if d.Metadata != nil {
		if err := c.cdxLicenses(d.Metadata.Licenses, "/metadata/licenses"); err != nil {
			return err
		}
		if d.Metadata.Component != nil {
			if err := c.cdxComponent(*d.Metadata.Component, "/metadata/component", false); err != nil {
				return err
			}
		}
	}
	for i, comp := range d.Components {
		if err := c.cdxComponent(comp, fmt.Sprintf("/components/%d", i), true); err != nil {
			return err
		}
	}
	var services func([]cdxService, string) error
	services = func(ss []cdxService, at string) error {
		for i, s := range ss {
			w := fmt.Sprintf("%s/%d", at, i)
			if err := c.cdxLicenses(s.Licenses, w+"/licenses"); err != nil {
				return err
			}
			if err := services(s.Services, w+"/services"); err != nil {
				return err
			}
		}
		return nil
	}
	return services(d.Services, "/services")
}

func (c *collector) cdxComponent(comp cdxComponent, at string, count bool) error {
	if count {
		c.doc.Packages++
		if len(comp.Licenses) == 0 {
			c.doc.Unlicensed = append(c.doc.Unlicensed, nonEmpty(comp.Name, comp.BOMRef))
		}
	}
	if err := c.cdxLicenses(comp.Licenses, at+"/licenses"); err != nil {
		return err
	}
	for i, sub := range comp.Components {
		if err := c.cdxComponent(sub, fmt.Sprintf("%s/components/%d", at, i), true); err != nil {
			return err
		}
	}
	return nil
}

func (c *collector) cdxLicenses(ls []cdxLicense, at string) error {
	for i, l := range ls {
		w := fmt.Sprintf("%s/%d", at, i)
		switch {
		case l.License != nil:
			embedded := l.License.Text != nil && strings.TrimSpace(l.License.Text.Content) != ""
			switch {
			case l.License.ID != "":
				if !idRe.MatchString(l.License.ID) {
					return fmt.Errorf("%s %s/license/id: %q is no license id", CycloneDXFile, w, l.License.ID)
				}
				c.add(License{ID: l.License.ID, Embedded: embedded, Where: w + "/license/id"})
			case l.License.Name != "":
				c.add(License{Name: l.License.Name, Embedded: embedded, Where: w + "/license/name"})
			default:
				return fmt.Errorf("%s %s/license: neither id nor name", CycloneDXFile, w)
			}
		case l.Expression != "":
			ids, err := Expression(l.Expression)
			if err != nil {
				return fmt.Errorf("%s %s/expression: %v", CycloneDXFile, w, err)
			}
			for _, id := range ids {
				c.add(License{ID: id, Where: w + "/expression"})
			}
		default:
			return fmt.Errorf("%s %s: neither license nor expression", CycloneDXFile, w)
		}
	}
	return nil
}

// ---- license expressions (SPDX 2.3 Annex D) ----

var idRe = regexp.MustCompile(`^(?:DocumentRef-[A-Za-z0-9.-]+:)?(?:LicenseRef-)?[A-Za-z0-9.-]+$`)

func isNone(e string) bool {
	e = strings.TrimSpace(e)
	return e == "NONE" || e == "NOASSERTION"
}

// Expression returns the license ids of an SPDX license expression that
// need a license text (SF-114 (b), proposed): every license id and
// LicenseRef- id, without the "+" suffix; exception ids after WITH and
// NONE/NOASSERTION need none.
func Expression(e string) ([]string, error) {
	if isNone(e) {
		return nil, nil
	}
	tokens := strings.Fields(strings.NewReplacer("(", " ( ", ")", " ) ").Replace(e))
	var ids []string
	depth, expectOperand, afterWith := 0, true, false
	for _, t := range tokens {
		switch {
		case t == "(":
			if !expectOperand {
				return nil, fmt.Errorf("license expression %q: unexpected \"(\"", e)
			}
			depth++
		case t == ")":
			if expectOperand || depth == 0 {
				return nil, fmt.Errorf("license expression %q: unexpected \")\"", e)
			}
			depth--
		case isOp(t, "AND"), isOp(t, "OR"), isOp(t, "WITH"):
			if expectOperand {
				return nil, fmt.Errorf("license expression %q: unexpected %s", e, t)
			}
			expectOperand, afterWith = true, isOp(t, "WITH")
			continue
		default:
			if !expectOperand {
				return nil, fmt.Errorf("license expression %q: missing operator before %q", e, t)
			}
			id := strings.TrimSuffix(t, "+")
			if !idRe.MatchString(id) {
				return nil, fmt.Errorf("license expression %q: %q is no license id", e, t)
			}
			if !afterWith && !isNone(id) {
				ids = append(ids, id)
			}
			expectOperand, afterWith = false, false
		}
	}
	if depth != 0 || expectOperand {
		return nil, errors.New("license expression " + fmt.Sprintf("%q", e) + " is incomplete")
	}
	return ids, nil
}

func isOp(t, op string) bool { return t == op || t == strings.ToLower(op) }

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func nonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
