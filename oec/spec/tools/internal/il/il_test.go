package il

import (
	"strings"
	"testing"
)

func TestEncodeDecodeGuidelineExamples(t *testing.T) {
	// MG 4.2 Examples table.
	for il, enc := range map[string]string{
		"provider.com/FieldDataService/FDS-001/1200-0345": "cHJvdmlkZXIuY29tL0ZpZWxkRGF0YVNlcnZpY2UvRkRTLTAwMS8xMjAwLTAzNDU",
		"https://id.consumer.com/MEService/Inst07322":     "aHR0cHM6Ly9pZC5jb25zdW1lci5jb20vTUVTZXJ2aWNlL0luc3QwNzMyMg",
		"company.id/A87BAD34H":                            "Y29tcGFueS5pZC9BODdCQUQzNEg",
		"https://company.id/A87BAD34H":                    "aHR0cHM6Ly9jb21wYW55LmlkL0E4N0JBRDM0SA",
		"https://example.com/~sn?S=1000":                  "aHR0cHM6Ly9leGFtcGxlLmNvbS9-c24_Uz0xMDAw",
	} {
		if got := Encode(il); got != enc {
			t.Errorf("Encode(%q) = %q, want %q", il, got, enc)
		}
		got, v, err := CheckTopicElement(enc)
		if err != nil || got != il || v != nil {
			t.Errorf("CheckTopicElement(%q) = %q, %v, %v", enc, got, v, err)
		}
	}
}

func TestDecodeRejectsNonCanonical(t *testing.T) {
	for _, enc := range []string{
		"", "Y29tcGFueS5pZC9BODdCQUQzNEg=", "aHR0cHM6Ly9leGFtcGxlLmNvbS9+c24/Uz0xMDAw",
		"Y29tcGFueS5pZC9BODdCQUQzNEh", "Y29tcGFueS5pZC9BODdCQUQzNE", "Y29tcGFueS5p\nZC9BODdCQUQzNEg",
	} {
		if _, err := Decode(enc); err == nil {
			t.Errorf("Decode(%q): expected error", enc)
		}
	}
}

func TestValidate(t *testing.T) {
	cases := map[string]string{
		"company.id/A87BAD34H": "",
		"https://www.domain-abc.com/sd09fqw4?1P=productcode&S=serialnumber": "",
		"company.id:8080/x":                      "",
		"HTTPS://company.id/x":                   "",
		"":                                       Empty,
		"Company.id/A87BAD34H":                   UppercaseAuthority,
		"https://Company.id/x":                   UppercaseAuthority,
		"/abc":                                   EmptyAuthority,
		"https://?q":                             EmptyAuthority,
		"company.id/A%20B":                       ForbiddenChar,
		"company.id/A B":                         ForbiddenChar,
		"company.id/\u00e4":                      ForbiddenChar,
		"company.id/" + strings.Repeat("a", 244): "",
		"company.id/" + strings.Repeat("a", 245): TooLong,
	}
	for in, want := range cases {
		v := Validate(in)
		got := ""
		if len(v) > 0 {
			got = v[0].Code
		}
		if got != want {
			t.Errorf("Validate(%q) = %v, want %s", in, v, want)
		}
	}
}

func TestDnpGuidelineExamples(t *testing.T) {
	// MG A3 Table 5 and the URI example.
	for _, c := range [][5]string{
		{"example.com", "MyModel", "MyProductCode", "MySerial", "example.com/MyModel/MyProductCode/MySerial"},
		{"example.com", "ABC DEF", "123.456/3&8", "123456#33", "example.com/ABC,20DEF/123.456,2F3,268/123456,2333"},
		{"Example.com", "ABC@home", "ABC*33<4", "20123.4", "example.com/ABC,40home/ABC,2A33,3C4/20123.4"},
		{"provider.com", "FieldDataService", "FDS-001", "1200-0345", "provider.com/FieldDataService/FDS-001/1200-0345"},
		{"example.com", "", "", "SN1", "example.com/nd/nd/SN1"},
	} {
		got, v := Oi4Identifier(c[0], c[1], c[2], c[3])
		if got != c[4] || v != nil {
			t.Errorf("Oi4Identifier(%q) = %q, %v, want %q", c[:4], got, v, c[4])
		}
	}
	if got := DnpEncode("Edge Gateway >4GB"); got != "Edge,20Gateway,20,3E4GB" {
		t.Errorf("DnpEncode = %q", got)
	}
	if got := DnpEncode("#20815!2205+1432"); got != ",2320815,212205,2B1432" {
		t.Errorf("DnpEncode = %q", got)
	}
	if got := DnpEncode("a,b\u00e4\t"); got != "a,2Cb,C3,A4,09" {
		t.Errorf("DnpEncode = %q", got)
	}
}

func TestManufacturerURI(t *testing.T) {
	for in, ok := range map[string]bool{
		"example.com": true, "id.example.com": true, "my-company.co.uk": true,
		"www.example.com": false, "https://example.com": false, "example": false,
		"Example.com": false, "example.com/x": false, "-a.com": false,
	} {
		if got := CheckManufacturerURI(in) == nil; got != ok {
			t.Errorf("CheckManufacturerURI(%q) ok=%v, want %v", in, got, ok)
		}
	}
}
