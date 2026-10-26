// Package ilink checks Identification Links (MG 4.1, IEC 61406-1) and their
// base64url encoding as topic elements (MG 4.2), and implements the
// dnp-encoding of the Oi4Identifier elements (MG A3).
//
// It is an independent implementation of the rules of the requirements
// catalog (oec/spec/requirements/il.yaml); the vectors of oec/spec keep it
// consistent with the reference checks of the spec baseline (ADR-0019 §5).
package ilink

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// MaxOctets is the maximum length of an Identification Link (REQ-IL-019).
const MaxOctets = 255

// MaxEncoded is the maximum length of an encoded topic element: the
// base64url encoding of MaxOctets octets without padding.
const MaxEncoded = (MaxOctets*8 + 5) / 6

// NoData is the Oi4Identifier element of a missing value (MG 4.3.1).
const NoData = "nd"

// Violation codes of an Identification Link and of its encoding.
const (
	CodeEmpty              = "empty"
	CodeTooLong            = "too-long"
	CodeForbiddenChar      = "forbidden-character"
	CodeEmptyAuthority     = "empty-authority"
	CodeUppercaseAuthority = "uppercase-authority"
	CodeEncoding           = "encoding"
)

// Violation is one finding of an Identification Link check.
type Violation struct {
	Code   string
	Req    string
	Detail string
}

var codeReq = map[string]string{
	CodeEmpty:              "REQ-IL-019",
	CodeTooLong:            "REQ-IL-019",
	CodeForbiddenChar:      "REQ-IL-016",
	CodeEmptyAuthority:     "REQ-IL-009",
	CodeUppercaseAuthority: "REQ-IL-018",
}

func violation(code, detail string) Violation {
	return Violation{Code: code, Req: codeReq[code], Detail: detail}
}

// permitted reports whether c may occur in an Identification Link: the
// unreserved and reserved characters of RFC 3986 (REQ-IL-016).
func permitted(c rune) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	}
	return strings.ContainsRune("-._~:/?#[]@!$&'()*+,;=", c)
}

// Scheme returns the "<scheme>://" prefix of s, or "".
func Scheme(s string) string {
	if s == "" || !isAlpha(s[0]) {
		return ""
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case isAlpha(c), c >= '0' && c <= '9', c == '+', c == '.', c == '-':
			continue
		case c == ':' && strings.HasPrefix(s[i:], "://"):
			return s[:i+3]
		}
		return ""
	}
	return ""
}

func isAlpha(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// Authority returns the authority of s: after the scheme (if any) up to the
// first "/", "?" or "#".
func Authority(s string) string {
	rest := s[len(Scheme(s)):]
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		return rest[:i]
	}
	return rest
}

// Validate checks an unencoded Identification Link and returns every
// violation (nil if it is valid).
func Validate(s string) []Violation {
	if s == "" {
		return []Violation{violation(CodeEmpty, "the Identification Link is empty")}
	}
	var out []Violation
	if len(s) > MaxOctets {
		out = append(out, violation(CodeTooLong, fmt.Sprintf("%d octets, maximum %d", len(s), MaxOctets)))
	}
	var bad []string
	seen := map[rune]bool{}
	for _, c := range s {
		if !permitted(c) && !seen[c] {
			seen[c] = true
			bad = append(bad, fmt.Sprintf("%q", c))
		}
	}
	if len(bad) > 0 {
		out = append(out, violation(CodeForbiddenChar, "not permitted: "+strings.Join(bad, " ")))
	}
	switch a := Authority(s); {
	case a == "":
		out = append(out, violation(CodeEmptyAuthority, "the authority is mandatory"))
	case strings.ToLower(a) != a:
		out = append(out, violation(CodeUppercaseAuthority, fmt.Sprintf("authority %q contains upper-case characters", a)))
	}
	return out
}

// Encode returns the topic element of an Identification Link: base64url
// without padding (REQ-IL-002).
func Encode(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

func isBase64URL(c rune) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_'
}

// CheckElement checks an encoded topic element (AppId, Source) as text:
// alphabet, padding, length and canonical encoding (MG 4.2). It returns
// the decoded Identification Link, or a violation with code "encoding" or
// "too-long". The decoded link is not validated (see Validate).
func CheckElement(enc string) (string, *Violation) {
	fail := func(req, detail string) (string, *Violation) {
		return "", &Violation{Code: CodeEncoding, Req: req, Detail: detail}
	}
	if enc == "" {
		return fail("REQ-IL-002", "the encoded Identification Link is empty")
	}
	switch {
	case strings.ContainsRune(enc, '+'):
		return fail("REQ-IL-003", "'+' of the standard base64 alphabet; base64url uses '-' and '_'")
	case strings.ContainsRune(enc, '='):
		return fail("REQ-IL-004", "padding '=' is not used")
	}
	for _, c := range enc {
		if !isBase64URL(c) {
			return fail("REQ-IL-002", fmt.Sprintf("%q is no base64url character", c))
		}
	}
	if len(enc) > MaxEncoded {
		return "", &Violation{Code: CodeTooLong, Req: "REQ-IL-019", Detail: fmt.Sprintf("%d characters encode more than %d octets", len(enc), MaxOctets)}
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(enc)
	if err != nil {
		return fail("REQ-IL-004", "not a canonical base64url string without padding")
	}
	return string(b), nil
}

// DnpEncode applies the dnp-encoding of MG A3: the unreserved characters
// A-Z a-z 0-9 - . _ ~ stay, every other octet becomes ",XX" (upper-case hex).
func DnpEncode(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if c < 0x80 && (isAlpha(c) || c >= '0' && c <= '9' || strings.IndexByte("-._~", c) >= 0) {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, ",%02X", c)
	}
	return b.String()
}
