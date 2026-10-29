package sbom

// SPDX 3.x in its JSON-LD serialization (SPDX 3.0 "Serialization"): a
// "@context" of spdx.org/rdf/3.x and a flat "@graph" of elements in the
// compact form of that context (types such as software_Package, properties
// such as software_packageVersion). Licenses are attached to a package by
// the relationships hasConcludedLicense / hasDeclaredLicense; their target
// is a LicenseExpression (Simple Licensing profile) or an element of the
// Expanded Licensing profile.

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var spdx3Context = regexp.MustCompile(`^https://spdx\.org/rdf/([0-9]+)\.([0-9]+)(?:\.[0-9]+)?/spdx-context\.jsonld$`)

type node map[string]json.RawMessage

func (n node) str(keys ...string) string {
	for _, k := range keys {
		var s string
		if json.Unmarshal(n[k], &s) == nil && s != "" {
			return s
		}
	}
	return ""
}

// strs reads a string or an array of strings.
func (n node) strs(key string) []string {
	var one string
	if json.Unmarshal(n[key], &one) == nil {
		if one == "" {
			return nil
		}
		return []string{one}
	}
	var many []string
	_ = json.Unmarshal(n[key], &many)
	return many
}

func (n node) nodes(key string) []node {
	var out []node
	_ = json.Unmarshal(n[key], &out)
	return out
}

func (n node) typ() string { return n.str("type", "@type") }
func (n node) id() string  { return n.str("spdxId", "@id") }

// lastSegment is the license id at the end of an element IRI
// (https://spdx.org/licenses/MIT, urn:…:LicenseRef-x, …#LicenseRef-x).
func lastSegment(iri string) string {
	if i := strings.LastIndexAny(iri, "/#:"); i >= 0 {
		return iri[i+1:]
	}
	return iri
}

type spdx3Reader struct {
	c        *collector
	byID     map[string]node
	imported map[string]bool
	texts    map[string]bool // license (exception) id → text embedded
}

func (c *collector) spdx3(data []byte, ctx json.RawMessage) error {
	c.doc.Format = "SPDX"
	var ctxs []string
	var one string
	if json.Unmarshal(ctx, &one) == nil {
		ctxs = []string{one}
	} else {
		var list []json.RawMessage
		_ = json.Unmarshal(ctx, &list)
		for _, r := range list {
			if json.Unmarshal(r, &one) == nil {
				ctxs = append(ctxs, one)
			}
		}
	}
	major := ""
	for _, s := range ctxs {
		if m := spdx3Context.FindStringSubmatch(s); m != nil {
			major, c.doc.Version = m[1], "SPDX-"+m[1]+"."+m[2]
			break
		}
	}
	switch major {
	case "":
		return fmt.Errorf("%s: the @context names no SPDX JSON-LD context (https://spdx.org/rdf/<version>/spdx-context.jsonld)", SPDXFile)
	case "3":
	default:
		return unsupported("%s: SPDX %s JSON-LD (SPDX 2.x JSON and SPDX 3.x JSON-LD are read)", SPDXFile, strings.TrimPrefix(c.doc.Version, "SPDX-"))
	}
	var doc struct {
		Graph []node `json:"@graph"`
	}
	if err := json.Unmarshal(data, &doc); err != nil || doc.Graph == nil {
		return fmt.Errorf("%s: SPDX 3 JSON-LD document without @graph array (%v)", SPDXFile, err)
	}
	r := &spdx3Reader{c: c, byID: map[string]node{}, imported: map[string]bool{}, texts: map[string]bool{}}
	hasDoc := false
	for _, n := range doc.Graph {
		if id := n.id(); id != "" {
			r.byID[id] = n
		}
		switch n.typ() {
		case "CreationInfo":
			if v := n.str("specVersion"); v != "" {
				c.doc.Version = "SPDX-" + v
			}
		case "SpdxDocument":
			hasDoc = true
			for _, m := range n.nodes("import") {
				r.imported[m.str("externalSpdxId")] = true
			}
		case "simplelicensing_SimpleLicensingText", "expandedlicensing_ListedLicense", "expandedlicensing_CustomLicense":
			if strings.TrimSpace(n.str("simplelicensing_licenseText")) != "" {
				r.texts[lastSegment(n.id())] = true
			}
		case "expandedlicensing_ListedLicenseException", "expandedlicensing_CustomLicenseAddition":
			if strings.TrimSpace(n.str("expandedlicensing_additionText")) != "" {
				r.texts[lastSegment(n.id())] = true
			}
		}
	}
	if !hasDoc {
		return fmt.Errorf("%s: the @graph has no SpdxDocument element", SPDXFile)
	}
	licenses := map[string][]string{}
	for _, n := range doc.Graph {
		if n.typ() != "Relationship" {
			continue
		}
		switch n.str("relationshipType") {
		case "hasConcludedLicense", "hasDeclaredLicense":
			licenses[n.str("from")] = append(licenses[n.str("from")], n.strs("to")...)
		}
	}
	for _, n := range doc.Graph {
		if n.typ() != "software_Package" {
			continue
		}
		comp := Component{Name: n.str("name"), Version: n.str("software_packageVersion"), Where: n.id(),
			Identified: n.str("software_packageUrl") != ""}
		for _, e := range n.nodes("externalIdentifier") {
			switch e.str("externalIdentifierType") {
			case "packageUrl", "cpe22", "cpe23", "swid", "gitoid", "swhid":
				comp.Identified = comp.Identified || e.str("identifier") != ""
			}
		}
		for _, to := range licenses[n.id()] {
			ok, err := r.license(to, 0)
			if err != nil {
				return err
			}
			comp.Licensed = comp.Licensed || ok
		}
		c.doc.Components = append(c.doc.Components, comp)
	}
	return nil
}

// license collects the licenses of the license element id and reports
// whether it states a license (NoAssertionLicense does not).
func (r *spdx3Reader) license(id string, depth int) (bool, error) {
	if depth > 32 {
		return false, fmt.Errorf("%s %s: license elements nested too deeply", SPDXFile, id)
	}
	switch {
	case strings.HasSuffix(id, "NoAssertionLicense"):
		return false, nil
	case strings.HasSuffix(id, "NoneLicense"):
		return true, nil
	}
	n, ok := r.byID[id]
	if !ok {
		if r.imported[id] {
			r.c.doc.External = append(r.c.doc.External, id)
			return true, nil
		}
		return false, fmt.Errorf("%s: license element %s is neither in the @graph nor imported", SPDXFile, id)
	}
	sub := func(key string) (bool, error) {
		to := n.str(key)
		if to == "" {
			return false, fmt.Errorf("%s %s: %s missing", SPDXFile, id, key)
		}
		return r.license(to, depth+1)
	}
	switch t := n.typ(); t {
	case "simplelicensing_LicenseExpression":
		e := n.str("simplelicensing_licenseExpression")
		ts, err := Expression(e)
		if err != nil {
			return false, fmt.Errorf("%s %s: %v", SPDXFile, id, err)
		}
		custom := map[string]string{}
		for _, m := range n.nodes("simplelicensing_customIdToUri") {
			custom[m.str("key")] = m.str("value")
		}
		r.c.terms(ts, id, func(lic string) bool {
			if uri, ok := custom[lic]; ok {
				return r.texts[lastSegment(uri)]
			}
			return r.texts[lic]
		})
		return stated(e), nil
	case "expandedlicensing_ListedLicense", "expandedlicensing_CustomLicense":
		lic := lastSegment(id)
		r.c.add(License{ID: lic, Embedded: r.texts[lic], Where: id})
		return true, nil
	case "expandedlicensing_ListedLicenseException", "expandedlicensing_CustomLicenseAddition":
		lic := lastSegment(id)
		r.c.add(License{ID: lic, Exception: true, Embedded: r.texts[lic], Where: id})
		return true, nil
	case "expandedlicensing_OrLaterOperator":
		return sub("expandedlicensing_subjectLicense")
	case "expandedlicensing_WithAdditionOperator":
		if _, err := sub("expandedlicensing_subjectLicense"); err != nil {
			return false, err
		}
		return sub("expandedlicensing_subjectAddition")
	case "expandedlicensing_ConjunctiveLicenseSet", "expandedlicensing_DisjunctiveLicenseSet":
		members := n.strs("expandedlicensing_member")
		if len(members) == 0 {
			return false, fmt.Errorf("%s %s: license set without members", SPDXFile, id)
		}
		for _, m := range members {
			if _, err := r.license(m, depth+1); err != nil {
				return false, err
			}
		}
		return true, nil
	default:
		return false, unsupported("%s %s: license element of type %q", SPDXFile, id, t)
	}
}
