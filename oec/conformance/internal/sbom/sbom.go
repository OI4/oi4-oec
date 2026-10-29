// Package sbom reads the software bill of materials of an OEC container
// image (RIG 4.1.4, SF-114): SPDX in the JSON serialization of its version
// (SPDX 2.x JSON, SPDX 3.x JSON-LD) or CycloneDX 1.x JSON. RIG 4.1.4 does
// not restrict the version of the format; a version this package cannot
// read is reported as ErrUnsupported, not as an invalid SBOM. It returns
// the components with name, version, license statement and identifier and
// the licenses and license exceptions the SBOM names with whether it embeds
// their texts, which is what oec-infra image needs for REQ-CNT-013/014/026/
// 027 and REQ-LIC-002.
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

// ErrUnsupported marks an SBOM in a version (or with a construct) this
// package cannot read. RIG 4.1.4 does not restrict the version of the
// format (SF-114), so such an SBOM is not judged.
var ErrUnsupported = errors.New("not readable by this tool")

func unsupported(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrUnsupported, fmt.Sprintf(format, args...))
}

// License is one license or license exception the SBOM names.
type License struct {
	ID string // SPDX license (exception) id or LicenseRef-/AdditionRef-…; "" for a license given only by name
	// Name of a CycloneDX license given only by name.
	Name string
	// Exception: a license exception (after WITH).
	Exception bool
	// Embedded: the SBOM carries the text.
	Embedded bool
	// Where the license is named first (JSON pointer or element id).
	Where string
}

// Component is one software component the SBOM lists: an SPDX package or a
// CycloneDX component (nested ones included; the described application,
// CycloneDX metadata.component, is the subject of the SBOM, not one of its
// components).
type Component struct {
	Name, Version string
	Where         string
	// Licensed: the component states its license (SPDX NONE is a
	// statement, NOASSERTION is none).
	Licensed bool
	// Identified: the component has a unique identifier such as a purl or
	// a CPE (RIG 4.1.4 recommendation).
	Identified bool
}

// Label names the component in a finding.
func (c Component) Label() string {
	if c.Name == "" {
		return c.Where
	}
	return c.Name
}

// Doc is a parsed SBOM.
type Doc struct {
	Format  string // SPDX | CycloneDX
	Version string // SPDX-2.3, SPDX-3.0.1, 1.6, …
	// Licenses by id or name, in the order of their first appearance.
	Licenses []License
	// Components in document order.
	Components []Component
	// External lists license references to other SPDX documents
	// (DocumentRef-…:LicenseRef-…, imported SPDX 3 elements); their texts
	// are not checked.
	External []string
}

// Missing returns the labels of the components for which has is false.
func (d *Doc) Missing(has func(Component) bool) []string {
	var out []string
	for _, c := range d.Components {
		if !has(c) {
			out = append(out, c.Label())
		}
	}
	return out
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
	if l.Exception {
		key = "exception:" + key
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

// terms adds the terms of a license expression; embedded tells whether the
// SBOM carries the text of an id.
func (c *collector) terms(ts []Term, where string, embedded func(string) bool) {
	for _, t := range ts {
		if strings.HasPrefix(t.ID, "DocumentRef-") {
			c.doc.External = append(c.doc.External, t.ID)
			continue
		}
		c.add(License{ID: t.ID, Exception: t.Exception, Embedded: embedded(t.ID), Where: where})
	}
}

// Parse parses the SBOM file name with the content data. An error wrapping
// ErrUnsupported means a version this package cannot read; another error
// means the file is no SBOM of the format its name requires.
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
	return c.doc, nil
}

// ---- SPDX: 2.x JSON or 3.x JSON-LD ----

var spdx2Version = regexp.MustCompile(`^SPDX-2\.[0-9]+$`)

func (c *collector) spdx(data []byte) error {
	var probe struct {
		SPDXVersion string          `json:"spdxVersion"`
		Context     json.RawMessage `json:"@context"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("%s is no SPDX JSON document: %v", SPDXFile, err)
	}
	switch {
	case len(probe.Context) > 0:
		return c.spdx3(data, probe.Context)
	case probe.SPDXVersion == "":
		return fmt.Errorf("%s has neither spdxVersion nor an SPDX 3 @context: no SPDX JSON document", SPDXFile)
	case !spdx2Version.MatchString(probe.SPDXVersion):
		c.doc.Format, c.doc.Version = "SPDX", probe.SPDXVersion
		return unsupported("%s: spdxVersion %q (SPDX 2.x JSON and SPDX 3.x JSON-LD are read)", SPDXFile, probe.SPDXVersion)
	}
	return c.spdx2(data)
}

// ---- SPDX 2.x ----

type spdxRef struct {
	Category string `json:"referenceCategory"`
	Type     string `json:"referenceType"`
	Locator  string `json:"referenceLocator"`
}

// identifies: a purl, a CPE, a SWID or another package-manager or
// persistent id (SPDX 2.3 Annex F).
func (r spdxRef) identifies() bool {
	if strings.TrimSpace(r.Locator) == "" {
		return false
	}
	switch strings.ReplaceAll(strings.ToUpper(r.Category), "_", "-") {
	case "PACKAGE-MANAGER", "PERSISTENT-ID":
		return true
	case "SECURITY":
		return r.Type == "cpe22Type" || r.Type == "cpe23Type" || r.Type == "swid"
	}
	return false
}

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
		Name                 string    `json:"name"`
		SPDXID               string    `json:"SPDXID"`
		VersionInfo          string    `json:"versionInfo"`
		ExternalRefs         []spdxRef `json:"externalRefs"`
		LicenseConcluded     string    `json:"licenseConcluded"`
		LicenseDeclared      string    `json:"licenseDeclared"`
		LicenseInfoFromFiles []string  `json:"licenseInfoFromFiles"`
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

func (c *collector) spdx2(data []byte) error {
	var d spdxDoc
	if err := json.Unmarshal(data, &d); err != nil {
		return fmt.Errorf("%s is no SPDX JSON document: %v", SPDXFile, err)
	}
	c.doc.Format, c.doc.Version = "SPDX", d.SPDXVersion
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
		ts, err := Expression(e)
		if err != nil {
			return fmt.Errorf("%s %s: %v", SPDXFile, where, err)
		}
		c.terms(ts, where, func(id string) bool { return embedded[id] })
		return nil
	}
	for i, p := range d.Packages {
		comp := Component{Name: p.Name, Version: p.VersionInfo, Where: fmt.Sprintf("/packages/%d", i)}
		for _, r := range p.ExternalRefs {
			comp.Identified = comp.Identified || r.identifies()
		}
		for _, f := range []struct{ name, v string }{{"licenseConcluded", p.LicenseConcluded}, {"licenseDeclared", p.LicenseDeclared}} {
			if f.v == "" {
				continue
			}
			if stated(f.v) {
				comp.Licensed = true
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
		c.doc.Components = append(c.doc.Components, comp)
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
	Version    string         `json:"version"`
	BOMRef     string         `json:"bom-ref"`
	Purl       string         `json:"purl"`
	CPE        string         `json:"cpe"`
	SWID       *struct{}      `json:"swid"`
	OmniborID  []string       `json:"omniborId"`
	SWHID      []string       `json:"swhid"`
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
	switch {
	case d.BOMFormat != "CycloneDX":
		return fmt.Errorf("%s: bomFormat %q, expected \"CycloneDX\"", CycloneDXFile, d.BOMFormat)
	case d.SpecVersion == "":
		return fmt.Errorf("%s has no specVersion: no CycloneDX JSON document", CycloneDXFile)
	case !cdxVersion.MatchString(d.SpecVersion):
		return unsupported("%s: specVersion %q (CycloneDX 1.x is read)", CycloneDXFile, d.SpecVersion)
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
		c.doc.Components = append(c.doc.Components, Component{
			Name: comp.Name, Version: comp.Version, Where: at, Licensed: len(comp.Licenses) > 0,
			Identified: strings.TrimSpace(comp.Purl) != "" || strings.TrimSpace(comp.CPE) != "" || comp.SWID != nil ||
				len(comp.OmniborID) > 0 || len(comp.SWHID) > 0,
		})
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
			ts, err := Expression(l.Expression)
			if err != nil {
				return fmt.Errorf("%s %s/expression: %v", CycloneDXFile, w, err)
			}
			c.terms(ts, w+"/expression", func(string) bool { return false })
		default:
			return fmt.Errorf("%s %s: neither license nor expression", CycloneDXFile, w)
		}
	}
	return nil
}

// ---- license expressions (SPDX 2.3 Annex D) ----

var idRe = regexp.MustCompile(`^(?:DocumentRef-[A-Za-z0-9.-]+:)?(?:LicenseRef-|AdditionRef-)?[A-Za-z0-9.-]+$`)

func isNone(e string) bool {
	e = strings.TrimSpace(e)
	return e == "NONE" || e == "NOASSERTION"
}

// stated: a license field states the license; NOASSERTION does not, NONE
// (no license applies) does (SF-114).
func stated(e string) bool {
	e = strings.TrimSpace(e)
	return e != "" && e != "NOASSERTION"
}

// Term is a license or license exception of an expression.
type Term struct {
	ID        string
	Exception bool // after WITH
}

// Expression returns the terms of an SPDX license expression that need a
// text (RIG 4.1.4, SF-114): every license id and LicenseRef- id, without
// the "+" suffix, and every license exception after WITH; NONE and
// NOASSERTION name no license.
func Expression(e string) ([]Term, error) {
	if isNone(e) {
		return nil, nil
	}
	tokens := strings.Fields(strings.NewReplacer("(", " ( ", ")", " ) ").Replace(e))
	var ids []Term
	depth, expectOperand, afterWith := 0, true, false
	for _, t := range tokens {
		switch {
		case t == "(":
			if !expectOperand || afterWith {
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
			if !isNone(id) {
				ids = append(ids, Term{ID: id, Exception: afterWith})
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
