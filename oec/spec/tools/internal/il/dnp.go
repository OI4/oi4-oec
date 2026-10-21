package il

import (
	"fmt"
	"regexp"
	"strings"
)

// DnpEncode applies the dnp-encoding (MG A3): the unreserved characters
// A-Z a-z 0-9 - . _ ~ are kept, every other octet of the UTF-8 encoding is
// written as "," followed by two upper-case hex digits. Table 18 is the ASCII
// excerpt of this rule (SF-043 a).
func DnpEncode(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if c < 0x80 && unreserved(rune(c)) {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, ",%02X", c)
	}
	return b.String()
}

func unreserved(c rune) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.ContainsRune("-._~", c)
}

// NoData represents a missing Oi4Identifier element (MG 4.3.1).
const NoData = "nd"

// Violation codes of ManufacturerURI.
const (
	BadManufacturerURI = "manufacturer-uri"
)

var domainRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// CheckManufacturerURI checks a ManufacturerUri (MG 4.3.1): lower case,
// a domain name without scheme, path and "www." (subdomains are allowed,
// SF-043 c).
func CheckManufacturerURI(s string) []Violation {
	switch {
	case s != strings.ToLower(s):
		return []Violation{{UppercaseAuthority, fmt.Sprintf("ManufacturerUri %q contains upper-case characters", s)}}
	case strings.HasPrefix(s, "www."):
		return []Violation{{BadManufacturerURI, "the third level domain www. is excluded"}}
	case !domainRE.MatchString(s):
		return []Violation{{BadManufacturerURI, fmt.Sprintf("%q is not a domain name of the form <manufacturer>.<tld>", s)}}
	}
	return nil
}

// Oi4Identifier builds ManufacturerUri/Model/ProductCode/SerialNumber
// (MG 4.3.1) and masks the elements with the dnp-encoding (MG A3). The
// guideline only permits the masking (SF-043 e); it is applied here so that
// every nameplate value yields a valid Identification Link, and it leaves
// values without such characters unchanged. The ManufacturerUri is converted
// to lower case (MG A3 Table 19); empty elements become "nd". The result is
// validated as an Identification Link.
func Oi4Identifier(manufacturerURI, model, productCode, serialNumber string) (string, []Violation) {
	m := strings.ToLower(manufacturerURI)
	if v := CheckManufacturerURI(m); v != nil {
		return "", v
	}
	part := func(s string) string {
		if s == "" {
			return NoData
		}
		return DnpEncode(s)
	}
	id := m + "/" + part(model) + "/" + part(productCode) + "/" + part(serialNumber)
	return id, Validate(id)
}
