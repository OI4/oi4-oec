// Package il is the reference implementation of the Identification Link
// rules used to check the vectors: MG 4.1 (validity), MG 4.2 (base64url
// encoding in topics), MG A3 (dnp-encoding) and MG 4.3.1 (Oi4Identifier).
// It encodes the proposed interpretations of SF-043 where the guideline is
// not total. It is test tooling: SDK and D7 implement their own checks.
package il

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
)

// MaxLength is the maximum length of an Identification Link (MG 4.1).
const MaxLength = 255

// Violation codes returned by Validate.
const (
	Empty              = "empty"
	TooLong            = "too-long"
	ForbiddenChar      = "forbidden-character"
	EmptyAuthority     = "empty-authority"
	UppercaseAuthority = "uppercase-authority"
)

// Violation is one broken rule.
type Violation struct {
	Code   string
	Detail string
}

func (v Violation) String() string { return v.Code + ": " + v.Detail }

var schemeRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.\-]*://`)

// allowed reports whether c is one of the ASCII characters of MG 4.1
// "Allowed Characters" (letters, digits, unreserved and reserved characters).
func allowed(c rune) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' ||
		strings.ContainsRune("-._~:/?#[]@!$&'()*+,;=", c)
}

// Scheme returns the scheme including "://" or "" if the IL has none.
func Scheme(s string) string { return schemeRE.FindString(s) }

// Authority returns the authority: the text after the optional scheme up to
// the first "/", "?" or "#".
func Authority(s string) string {
	rest := strings.TrimPrefix(s, Scheme(s))
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		return rest[:i]
	}
	return rest
}

// Validate checks the machine-checkable rules of MG 4.1 and returns all
// violations; nil means valid. Not checkable: uniqueness, control of the
// domain, reserved characters "used according to their function" and the
// character combinations with special functions.
func Validate(s string) []Violation {
	var out []Violation
	if s == "" {
		return []Violation{{Empty, "the Identification Link is empty"}}
	}
	if n := len(s); n > MaxLength {
		out = append(out, Violation{TooLong, fmt.Sprintf("%d characters (octets), maximum %d", n, MaxLength)})
	}
	var bad []string
	seen := map[rune]bool{}
	for _, c := range s {
		if !allowed(c) && !seen[c] {
			seen[c] = true
			bad = append(bad, fmt.Sprintf("%q", c))
		}
	}
	if len(bad) > 0 {
		out = append(out, Violation{ForbiddenChar, "not permitted: " + strings.Join(bad, " ")})
	}
	a := Authority(s)
	switch {
	case a == "":
		out = append(out, Violation{EmptyAuthority, "the authority is mandatory"})
	case strings.ToLower(a) != a:
		out = append(out, Violation{UppercaseAuthority, fmt.Sprintf("authority %q contains upper-case characters", a)})
	}
	return out
}

// Encode returns the topic element of an Identification Link (MG 4.2).
func Encode(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

// Decode reverses Encode. It accepts exactly the canonical form: base64url
// alphabet, no padding, no other characters and zero unused trailing bits
// ("exactly one valid encoded representation", MG 4.2).
func Decode(enc string) (string, error) {
	if enc == "" {
		return "", fmt.Errorf("empty topic element")
	}
	// The Go decoder skips CR and LF; MG 4.2 step 4 forbids them.
	if strings.ContainsAny(enc, "\r\n") {
		return "", fmt.Errorf("line breaks are not allowed")
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(enc)
	if err != nil {
		return "", fmt.Errorf("not a canonical unpadded base64url string: %w", err)
	}
	return string(b), nil
}

// CheckTopicElement decodes an AppId/Source element and validates the
// resulting Identification Link.
func CheckTopicElement(enc string) (string, []Violation, error) {
	s, err := Decode(enc)
	if err != nil {
		return "", nil, err
	}
	return s, Validate(s), nil
}
