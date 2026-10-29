package sbom

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func terms(ts []Term) string {
	var s []string
	for _, t := range ts {
		if t.Exception {
			s = append(s, "+"+t.ID)
		} else {
			s = append(s, t.ID)
		}
	}
	return strings.Join(s, ",")
}

func TestExpression(t *testing.T) {
	for e, want := range map[string]string{
		"MIT":                                  "MIT",
		"NOASSERTION":                          "",
		"NONE":                                 "",
		"(MIT OR Apache-2.0) AND BSD-3-Clause": "MIT,Apache-2.0,BSD-3-Clause",
		"GPL-2.0-or-later WITH Classpath-exception-2.0":   "GPL-2.0-or-later,+Classpath-exception-2.0",
		"GPL-2.0+ and LicenseRef-acme":                    "GPL-2.0,LicenseRef-acme",
		"((MIT))":                                         "MIT",
		"Apache-2.0 WITH AdditionRef-acme-exception OR X": "Apache-2.0,+AdditionRef-acme-exception,X",
	} {
		ts, err := Expression(e)
		if err != nil || terms(ts) != want {
			t.Errorf("%q: %v %v", e, terms(ts), err)
		}
	}
	for _, e := range []string{"MIT AND", "(MIT", "MIT)", "MIT Apache-2.0", "AND MIT", "MIT/X11", "WITH x", "MIT WITH", "MIT WITH (x)"} {
		if _, err := Expression(e); err == nil {
			t.Errorf("%q: no error", e)
		}
	}
}

func licenses(d *Doc) string {
	var got []string
	for _, l := range d.Licenses {
		s := l.ID + l.Name
		if l.Exception {
			s = "+" + s
		}
		if l.Embedded {
			s += "*"
		}
		got = append(got, s)
	}
	return strings.Join(got, ",")
}

func components(d *Doc) string {
	var got []string
	for _, c := range d.Components {
		s := c.Name + "@" + c.Version
		if !c.Licensed {
			s += "!lic"
		}
		if !c.Identified {
			s += "!id"
		}
		got = append(got, s)
	}
	return strings.Join(got, ",")
}

const spdxHead = `"spdxVersion":"SPDX-2.3","dataLicense":"CC0-1.0","SPDXID":"SPDXRef-DOCUMENT","name":"app","documentNamespace":"https://example.com/app","creationInfo":{"created":"2026-01-01T00:00:00Z","creators":["Tool: test"]}`

const purl = `"externalRefs":[{"referenceCategory":"PACKAGE-MANAGER","referenceType":"purl","referenceLocator":"pkg:generic/a@1"}]`

func TestSPDX(t *testing.T) {
	doc, err := Parse(SPDXFile, []byte(`{`+spdxHead+`,
	 "packages":[
	  {"name":"a","SPDXID":"SPDXRef-a","versionInfo":"1.0","licenseConcluded":"MIT","licenseDeclared":"MIT OR LicenseRef-x",`+purl+`},
	  {"name":"b","SPDXID":"SPDXRef-b","versionInfo":"2","licenseConcluded":"NOASSERTION","licenseDeclared":"NOASSERTION",
	   "externalRefs":[{"referenceCategory":"SECURITY","referenceType":"cpe23Type","referenceLocator":"cpe:2.3:a:b:b:2:*:*:*:*:*:*:*"}]},
	  {"name":"c","SPDXID":"SPDXRef-c","licenseDeclared":"DocumentRef-o:LicenseRef-y",
	   "externalRefs":[{"referenceCategory":"OTHER","referenceType":"x","referenceLocator":"y"}]},
	  {"name":"d","SPDXID":"SPDXRef-d","versionInfo":"3","licenseConcluded":"NONE","licenseDeclared":"GPL-2.0-only WITH Classpath-exception-2.0",`+purl+`},
	  {"name":"e","SPDXID":"SPDXRef-e","versionInfo":"4",`+purl+`}],
	 "files":[{"fileName":"f","licenseConcluded":"Apache-2.0","licenseInfoInFiles":["BSD-2-Clause"]}],
	 "hasExtractedLicensingInfos":[{"licenseId":"LicenseRef-x","extractedText":"text of x"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := licenses(doc); got != "MIT,LicenseRef-x*,GPL-2.0-only,+Classpath-exception-2.0,Apache-2.0,BSD-2-Clause" {
		t.Errorf("licenses %s", got)
	}
	if got := components(doc); got != "a@1.0,b@2!lic,c@!id,d@3,e@4!lic" {
		t.Errorf("components %s", got)
	}
	if doc.Format != "SPDX" || doc.Version != "SPDX-2.3" || strings.Join(doc.External, ",") != "DocumentRef-o:LicenseRef-y" {
		t.Errorf("format %s %s external %v", doc.Format, doc.Version, doc.External)
	}
}

// Every SPDX 2.x version is read (SF-114: the version is not restricted).
func TestSPDXVersions(t *testing.T) {
	for _, v := range []string{"SPDX-2.1", "SPDX-2.2", "SPDX-2.3", "SPDX-2.4"} {
		head := strings.Replace(spdxHead, "SPDX-2.3", v, 1)
		doc, err := Parse(SPDXFile, []byte(`{`+head+`,"packages":[{"name":"a","SPDXID":"SPDXRef-a","versionInfo":"1","licenseDeclared":"MIT"}]}`))
		if err != nil || doc.Version != v {
			t.Errorf("%s: %v", v, err)
		}
	}
}

const spdx3 = `{"@context":"https://spdx.org/rdf/3.0.1/spdx-context.jsonld","@graph":[
 {"type":"CreationInfo","@id":"_:ci","specVersion":"3.0.1","created":"2026-01-01T00:00:00Z","createdBy":["urn:t:tool"]},
 {"type":"SpdxDocument","spdxId":"urn:t:doc","creationInfo":"_:ci","rootElement":["urn:t:a"]},
 {"type":"software_Package","spdxId":"urn:t:a","creationInfo":"_:ci","name":"a","software_packageVersion":"1.0","software_packageUrl":"pkg:generic/a@1.0"},
 {"type":"software_Package","spdxId":"urn:t:b","creationInfo":"_:ci","name":"b","software_packageVersion":"2",
  "externalIdentifier":[{"type":"ExternalIdentifier","externalIdentifierType":"cpe23","identifier":"cpe:2.3:a:b:b:2:*:*:*:*:*:*:*"}]},
 {"type":"software_Package","spdxId":"urn:t:c","creationInfo":"_:ci","name":"c"},
 {"type":"software_Package","spdxId":"urn:t:d","creationInfo":"_:ci","name":"d","software_packageVersion":"4","software_packageUrl":"pkg:generic/d@4"},
 {"type":"software_Package","spdxId":"urn:t:e","creationInfo":"_:ci","name":"e","software_packageVersion":"5","software_packageUrl":"pkg:generic/e@5"},
 {"type":"simplelicensing_LicenseExpression","spdxId":"urn:t:l1","creationInfo":"_:ci","simplelicensing_licenseExpression":"MIT OR LicenseRef-x",
  "simplelicensing_customIdToUri":[{"type":"DictionaryEntry","key":"LicenseRef-x","value":"urn:t:x"}]},
 {"type":"simplelicensing_SimpleLicensingText","spdxId":"urn:t:x","creationInfo":"_:ci","simplelicensing_licenseText":"text of x"},
 {"type":"simplelicensing_LicenseExpression","spdxId":"urn:t:l2","creationInfo":"_:ci","simplelicensing_licenseExpression":"NOASSERTION"},
 {"type":"expandedlicensing_WithAdditionOperator","spdxId":"urn:t:w","creationInfo":"_:ci",
  "expandedlicensing_subjectLicense":"https://spdx.org/licenses/GPL-2.0-only","expandedlicensing_subjectAddition":"https://spdx.org/licenses/Classpath-exception-2.0"},
 {"type":"expandedlicensing_ListedLicense","spdxId":"https://spdx.org/licenses/GPL-2.0-only","creationInfo":"_:ci","simplelicensing_licenseText":"GPL text"},
 {"type":"expandedlicensing_ListedLicenseException","spdxId":"https://spdx.org/licenses/Classpath-exception-2.0","creationInfo":"_:ci"},
 {"type":"expandedlicensing_CustomLicense","spdxId":"urn:t:LicenseRef-acme","creationInfo":"_:ci","simplelicensing_licenseText":"acme text"},
 {"type":"Relationship","spdxId":"urn:t:r1","creationInfo":"_:ci","from":"urn:t:a","relationshipType":"hasConcludedLicense","to":["urn:t:l1"]},
 {"type":"Relationship","spdxId":"urn:t:r2","creationInfo":"_:ci","from":"urn:t:b","relationshipType":"hasDeclaredLicense","to":["urn:t:l2"]},
 {"type":"Relationship","spdxId":"urn:t:r3","creationInfo":"_:ci","from":"urn:t:c","relationshipType":"hasDeclaredLicense","to":["https://spdx.org/rdf/3.0.1/terms/ExpandedLicensing/NoneLicense"]},
 {"type":"Relationship","spdxId":"urn:t:r4","creationInfo":"_:ci","from":"urn:t:d","relationshipType":"hasConcludedLicense","to":["urn:t:w"]},
 {"type":"Relationship","spdxId":"urn:t:r5","creationInfo":"_:ci","from":"urn:t:e","relationshipType":"hasDeclaredLicense","to":["urn:t:LicenseRef-acme"]},
 {"type":"Relationship","spdxId":"urn:t:r6","creationInfo":"_:ci","from":"urn:t:a","relationshipType":"dependsOn","to":["urn:t:b"]}
]}`

func TestSPDX3(t *testing.T) {
	doc, err := Parse(SPDXFile, []byte(spdx3))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Format != "SPDX" || doc.Version != "SPDX-3.0.1" {
		t.Errorf("format %s %s", doc.Format, doc.Version)
	}
	if got := licenses(doc); got != "MIT,LicenseRef-x*,GPL-2.0-only*,+Classpath-exception-2.0,LicenseRef-acme*" {
		t.Errorf("licenses %s", got)
	}
	if got := components(doc); got != "a@1.0,b@2!lic,c@!id,d@4,e@5" {
		t.Errorf("components %s", got)
	}
}

func TestCycloneDX(t *testing.T) {
	doc, err := Parse(CycloneDXFile, []byte(`{"bomFormat":"CycloneDX","specVersion":"1.6",
	 "metadata":{"component":{"name":"app","licenses":[{"license":{"id":"Apache-2.0"}}]}},
	 "components":[
	  {"name":"a","version":"1","purl":"pkg:generic/a@1","licenses":[{"license":{"name":"Acme EULA","text":{"content":"EULA"}}}],
	   "components":[{"name":"a1","version":"1.1","cpe":"cpe:2.3:a:x:a1:1.1:*:*:*:*:*:*:*","licenses":[{"expression":"MIT OR ISC WITH LLVM-exception"}]}]},
	  {"name":"b","version":"2","purl":"pkg:generic/b@2","licenses":[{"license":{"name":"Other"}}]},
	  {"name":"c"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := licenses(doc); got != "Apache-2.0,Acme EULA*,MIT,ISC,+LLVM-exception,Other" {
		t.Errorf("licenses %s", got)
	}
	if got := components(doc); got != "a@1,a1@1.1,b@2,c@!lic!id" {
		t.Errorf("components %s", got)
	}
}

func TestCycloneDXVersions(t *testing.T) {
	for _, v := range []string{"1.0", "1.4", "1.6", "1.7", "1.12"} {
		doc, err := Parse(CycloneDXFile, []byte(`{"bomFormat":"CycloneDX","specVersion":"`+v+`"}`))
		if err != nil || doc.Version != v {
			t.Errorf("%s: %v", v, err)
		}
	}
}

// A version this tool cannot read is no violation: Parse returns
// ErrUnsupported, oec-infra reports it as inconclusive (SF-114).
func TestUnsupported(t *testing.T) {
	for name, c := range map[string]struct{ file, data string }{
		"spdx 4 context":  {SPDXFile, `{"@context":"https://spdx.org/rdf/4.0/spdx-context.jsonld","@graph":[]}`},
		"spdx 1.2":        {SPDXFile, `{"spdxVersion":"SPDX-1.2"}`},
		"spdx 3 tag":      {SPDXFile, `{"spdxVersion":"SPDX-3.0"}`},
		"cdx 2":           {CycloneDXFile, `{"bomFormat":"CycloneDX","specVersion":"2.0"}`},
		"spdx 3 expanded": {SPDXFile, strings.Replace(spdx3, `"type":"expandedlicensing_WithAdditionOperator"`, `"type":"expandedlicensing_FutureOperator"`, 1)},
	} {
		_, err := Parse(c.file, []byte(c.data))
		if !errors.Is(err, ErrUnsupported) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for name, c := range map[string]struct{ file, data, want string }{
		"no json":       {SPDXFile, `nope`, "no JSON object"},
		"no version":    {SPDXFile, `{"name":"x"}`, "neither spdxVersion nor an SPDX 3 @context"},
		"spdx fields":   {SPDXFile, `{"spdxVersion":"SPDX-2.3","SPDXID":"SPDXRef-DOCUMENT"}`, "creationInfo, dataLicense, documentNamespace, name"},
		"spdx expr":     {SPDXFile, `{` + spdxHead + `,"packages":[{"name":"a","licenseDeclared":"MIT AND"}]}`, "/packages/0/licenseDeclared"},
		"spdx3 graph":   {SPDXFile, `{"@context":"https://spdx.org/rdf/3.0.1/spdx-context.jsonld"}`, "@graph"},
		"spdx3 doc":     {SPDXFile, `{"@context":"https://spdx.org/rdf/3.0.1/spdx-context.jsonld","@graph":[]}`, "no SpdxDocument"},
		"spdx3 expr":    {SPDXFile, strings.Replace(spdx3, `"MIT OR LicenseRef-x"`, `"MIT OR"`, 1), "urn:t:l1"},
		"spdx3 to":      {SPDXFile, strings.Replace(spdx3, `"to":["urn:t:l2"]`, `"to":["urn:t:nothing"]`, 1), "urn:t:nothing"},
		"spdx3 context": {SPDXFile, `{"@context":"https://example.com/ctx.jsonld","@graph":[]}`, "@context"},
		"cdx format":    {CycloneDXFile, `{"bomFormat":"SPDX","specVersion":"1.5"}`, "bomFormat"},
		"cdx version":   {CycloneDXFile, `{"bomFormat":"CycloneDX"}`, "specVersion"},
		"cdx empty lic": {CycloneDXFile, `{"bomFormat":"CycloneDX","specVersion":"1.5","components":[{"name":"a","licenses":[{}]}]}`, "neither license nor expression"},
		"spdx as cdx":   {CycloneDXFile, `{` + spdxHead + `}`, "bomFormat"},
		"cdx as spdx":   {SPDXFile, `{"bomFormat":"CycloneDX","specVersion":"1.6"}`, "neither spdxVersion"},
		"array":         {SPDXFile, `[]`, "no JSON object"},
	} {
		_, err := Parse(c.file, []byte(c.data))
		if err == nil || errors.Is(err, ErrUnsupported) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func ExampleExpression() {
	ts, _ := Expression("GPL-2.0-or-later WITH Classpath-exception-2.0 OR MIT")
	fmt.Println(terms(ts))
	// Output: GPL-2.0-or-later,+Classpath-exception-2.0,MIT
}
