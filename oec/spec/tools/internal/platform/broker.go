// Package platform holds the reference checks of the files a platform
// provides to the containers (RIG 4.1): so far broker.json of the Message
// Bus storage (RIG 4.1.1).
package platform

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/OI4/oec/spec/tools/internal/payload"
)

// Broker is the requirement of the content of broker.json.
const Broker = "REQ-CNT-003"

// Bounds of MaxPacketSize (RIG 4.1.1, SF-091 (b)).
const (
	MinPacketSize = 262144
	MaxPacketSize = 268435455
)

// Address syntax (SF-100 (b)): an IPv4 address in dotted-decimal
// form, an IPv6 address in text form (IPv6address of RFC 3986, without
// brackets and zone) or a host name of RFC 1123 whose last label is not all
// digits. The JSON Schema uses the same patterns.
var (
	IPv4Pattern     = `^` + ipv4 + `$`
	IPv6Pattern     = ipv6()
	HostnamePattern = `^(?:` + label + `\.)*` + label + `$`
	NumericTLD      = `(?:^|\.)[0-9]+$`

	ipv4RE     = regexp.MustCompile(IPv4Pattern)
	ipv6RE     = regexp.MustCompile(IPv6Pattern)
	hostnameRE = regexp.MustCompile(HostnamePattern)
	numericRE  = regexp.MustCompile(NumericTLD)
)

const (
	dec   = `(?:25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9]?[0-9])`
	ipv4  = `(?:` + dec + `\.){3}` + dec
	h16   = `[0-9A-Fa-f]{1,4}`
	label = `[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?`
)

// ipv6 translates the IPv6address rule of RFC 3986 3.2.2.
func ipv6() string {
	ls32 := `(?:` + h16 + `:` + h16 + `|` + ipv4 + `)`
	pre := func(n int) string { // [ *n( h16 ":" ) h16 ]
		if n == 0 {
			return `(?:` + h16 + `)?`
		}
		return fmt.Sprintf(`(?:(?:%s:){0,%d}%s)?`, h16, n, h16)
	}
	alts := []string{
		fmt.Sprintf(`(?:%s:){6}%s`, h16, ls32),
		fmt.Sprintf(`::(?:%s:){5}%s`, h16, ls32),
		fmt.Sprintf(`%s::(?:%s:){4}%s`, pre(0), h16, ls32),
		fmt.Sprintf(`%s::(?:%s:){3}%s`, pre(1), h16, ls32),
		fmt.Sprintf(`%s::(?:%s:){2}%s`, pre(2), h16, ls32),
		fmt.Sprintf(`%s::%s:%s`, pre(3), h16, ls32),
		pre(4) + `::` + ls32,
		pre(5) + `::` + h16,
		pre(6) + `::`,
	}
	return `^(?:` + strings.Join(alts, `|`) + `)$`
}

// CheckBroker checks a decoded broker.json (payload.Parse): a JSON object
// with Address, SecurePort and MaxPacketSize (RIG 4.1.1). Further members
// are allowed (SF-100 (a)).
func CheckBroker(doc any) []payload.Problem {
	var out []payload.Problem
	add := func(ptr, format string, a ...any) {
		out = append(out, payload.Problem{Pointer: ptr, Req: Broker, Msg: fmt.Sprintf(format, a...)})
	}
	m, ok := doc.(map[string]any)
	if !ok {
		add("", "broker.json must be a JSON object")
		return out
	}
	for _, k := range []string{"Address", "SecurePort", "MaxPacketSize"} {
		if _, ok := m[k]; !ok {
			add("", "missing mandatory element %s", k)
		}
	}
	if v, ok := m["Address"]; ok {
		s, isStr := v.(string)
		switch {
		case !isStr:
			add("/Address", "Address must be a string")
		case !ValidAddress(s):
			add("/Address", "Address %q is no IP address or host name", s)
		}
	}
	if v, ok := m["SecurePort"]; ok {
		if n, ok := integer(v); !ok || n < 1 || n > 65535 {
			add("/SecurePort", "SecurePort must be an integer 1 … 65535, got %v", v)
		}
	}
	if v, ok := m["MaxPacketSize"]; ok {
		if n, ok := integer(v); !ok || n < MinPacketSize || n > MaxPacketSize {
			add("/MaxPacketSize", "MaxPacketSize must be an integer %d … %d, got %v", MinPacketSize, MaxPacketSize, v)
		}
	}
	return out
}

// ValidAddress reports whether s is an IPv4 address, an IPv6 address or a
// host name (SF-100 (b)).
func ValidAddress(s string) bool {
	if ipv4RE.MatchString(s) || ipv6RE.MatchString(s) {
		return true
	}
	return len(s) <= 253 && hostnameRE.MatchString(s) && !numericRE.MatchString(s)
}

func integer(v any) (int64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	i, err := strconv.ParseInt(n.String(), 10, 64)
	return i, err == nil
}
