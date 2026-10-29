package sbom

import (
	"strings"
	"testing"
)

func TestExpression(t *testing.T) {
	for e, want := range map[string]string{
		"MIT":                                  "MIT",
		"NOASSERTION":                          "",
		"NONE":                                 "",
		"(MIT OR Apache-2.0) AND BSD-3-Clause": "MIT,Apache-2.0,BSD-3-Clause",
		"GPL-2.0-or-later WITH Classpath-exception-2.0": "GPL-2.0-or-later",
		"GPL-2.0+ and LicenseRef-acme":                  "GPL-2.0,LicenseRef-acme",
		"((MIT))":                                       "MIT",
	} {
		ids, err := Expression(e)
		if err != nil || strings.Join(ids, ",") != want {
			t.Errorf("%q: %v %v", e, ids, err)
		}
	}
	for _, e := range []string{"MIT AND", "(MIT", "MIT)", "MIT Apache-2.0", "AND MIT", "MIT/X11", "WITH x"} {
		if _, err := Expression(e); err == nil {
			t.Errorf("%q: no error", e)
		}
	}
}

const spdxHead = `"spdxVersion":"SPDX-2.3","dataLicense":"CC0-1.0","SPDXID":"SPDXRef-DOCUMENT","name":"app","documentNamespace":"https://example.com/app","creationInfo":{"created":"2026-01-01T00:00:00Z","creators":["Tool: test"]}`

func TestSPDX(t *testing.T) {
	doc, err := Parse(SPDXFile, []byte(`{`+spdxHead+`,
	 "packages":[
	  {"name":"a","SPDXID":"SPDXRef-a","licenseConcluded":"MIT","licenseDeclared":"MIT OR LicenseRef-x"},
	  {"name":"b","SPDXID":"SPDXRef-b","licenseConcluded":"NOASSERTION","licenseDeclared":"NOASSERTION"},
	  {"name":"c","SPDXID":"SPDXRef-c","licenseDeclared":"DocumentRef-o:LicenseRef-y"}],
	 "files":[{"fileName":"f","licenseConcluded":"Apache-2.0","licenseInfoInFiles":["BSD-2-Clause"]}],
	 "hasExtractedLicensingInfos":[{"licenseId":"LicenseRef-x","extractedText":"text of x"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range doc.Licenses {
		s := l.ID
		if l.Embedded {
			s += "*"
		}
		got = append(got, s)
	}
	if strings.Join(got, ",") != "MIT,LicenseRef-x*,Apache-2.0,BSD-2-Clause" {
		t.Errorf("licenses %v", got)
	}
	if doc.Packages != 3 || strings.Join(doc.Unlicensed, ",") != "b" || strings.Join(doc.External, ",") != "DocumentRef-o:LicenseRef-y" {
		t.Errorf("packages %d unlicensed %v external %v", doc.Packages, doc.Unlicensed, doc.External)
	}
}

func TestCycloneDX(t *testing.T) {
	doc, err := Parse(CycloneDXFile, []byte(`{"bomFormat":"CycloneDX","specVersion":"1.6",
	 "metadata":{"component":{"name":"app","licenses":[{"license":{"id":"Apache-2.0"}}]}},
	 "components":[
	  {"name":"a","licenses":[{"license":{"name":"Acme EULA","text":{"content":"EULA"}}}],
	   "components":[{"name":"a1","licenses":[{"expression":"MIT OR ISC"}]}]},
	  {"name":"b","licenses":[{"license":{"name":"Other"}}]},
	  {"name":"c"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range doc.Licenses {
		s := l.ID + l.Name
		if l.Embedded {
			s += "*"
		}
		got = append(got, s)
	}
	if strings.Join(got, ",") != "Apache-2.0,Acme EULA*,MIT,ISC,Other" {
		t.Errorf("licenses %v", got)
	}
	if doc.Packages != 4 || strings.Join(doc.Unlicensed, ",") != "c" {
		t.Errorf("packages %d unlicensed %v", doc.Packages, doc.Unlicensed)
	}
}

func TestParseErrors(t *testing.T) {
	for name, c := range map[string]struct{ file, data, want string }{
		"no json":       {SPDXFile, `nope`, "no JSON object"},
		"spdx 3":        {SPDXFile, `{"@context":"https://spdx.org/rdf/3.0.1/spdx-context.jsonld","@graph":[]}`, "no spdxVersion"},
		"spdx 2.1":      {SPDXFile, `{"spdxVersion":"SPDX-2.1"}`, "not accepted"},
		"spdx fields":   {SPDXFile, `{"spdxVersion":"SPDX-2.3","SPDXID":"SPDXRef-DOCUMENT"}`, "creationInfo, dataLicense, documentNamespace, name"},
		"spdx expr":     {SPDXFile, `{` + spdxHead + `,"packages":[{"name":"a","licenseDeclared":"MIT AND"}]}`, "/packages/0/licenseDeclared"},
		"cdx format":    {CycloneDXFile, `{"bomFormat":"SPDX","specVersion":"1.5"}`, "bomFormat"},
		"cdx version":   {CycloneDXFile, `{"bomFormat":"CycloneDX","specVersion":"2.0"}`, "specVersion"},
		"cdx empty lic": {CycloneDXFile, `{"bomFormat":"CycloneDX","specVersion":"1.5","components":[{"name":"a","licenses":[{}]}]}`, "neither license nor expression"},
		"spdx as cdx":   {CycloneDXFile, `{` + spdxHead + `}`, "bomFormat"},
		"array":         {SPDXFile, `[]`, "no JSON object"},
	} {
		if _, err := Parse(c.file, []byte(c.data)); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
