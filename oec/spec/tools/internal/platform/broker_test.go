package platform

import (
	"net"
	"sort"
	"strings"
	"testing"

	"github.com/OI4/oec/spec/tools/internal/payload"
)

func TestAddress(t *testing.T) {
	for _, c := range []struct {
		s  string
		ok bool
	}{
		{"192.168.1.10", true}, {"0.0.0.0", true}, {"255.255.255.255", true},
		{"256.1.1.1", false}, {"192.168.01.1", false}, {"1.2.3", false},
		{"::1", true}, {"::", true}, {"fe80::1", true}, {"2001:db8::8a2e:370:7334", true},
		{"::ffff:192.168.1.1", true}, {"1:2:3:4:5:6:7:8", true}, {"1:2:3:4:5:6:1.2.3.4", true},
		{"1:2:3:4:5:6:7:8:9", false}, {"1::2::3", false}, {":::", false}, {"12345::", false},
		{"[::1]", false}, {"fe80::1%eth0", false}, {"::256.1.1.1", false},
		{"localhost", true}, {"mqtt-broker", true}, {"broker.oi4.local", true}, {"Broker1.Example.COM", true},
		{"", false}, {"-broker", false}, {"broker-", false}, {"broker.", false}, {".broker", false},
		{"oi4_broker", false}, {"broker local", false}, {"999.1.1.1", false}, {"host.123", false},
		{"123host", true}, {strings.Repeat("a", 64), false}, {strings.Repeat("a", 63), true},
		{strings.Repeat("a.", 126) + "ab", false}, {strings.Repeat("a.", 125) + "abc", true},
	} {
		if got := ValidAddress(c.s); got != c.ok {
			t.Errorf("ValidAddress(%q) = %v, want %v", c.s, got, c.ok)
		}
	}
}

// The IPv6 pattern agrees with the Go parser on addresses without zone.
func TestIPv6AgreesWithNet(t *testing.T) {
	for _, s := range []string{
		"::1", "::", "1::", "1:2::", "::2:3", "1:2:3:4:5:6:7::", "::2:3:4:5:6:7:8", "1::8",
		"1:2:3:4:5::8", "1:2:3:4::7:8", "::ffff:1.2.3.4", "1::1.2.3.4", "1:2:3:4:5:6:7:8",
		"1:2:3:4:5:6:7:8:9", "1::2::3", "1:2:3:4:5:6:7::8", "::1.2.3", "1:2:3:4:5:6:7:1.2.3.4", "abcd::ABCD",
	} {
		want := net.ParseIP(s) != nil
		if got := ipv6RE.MatchString(s); got != want {
			t.Errorf("IPv6 pattern on %q = %v, net.ParseIP = %v", s, got, want)
		}
	}
}

func TestCheckBroker(t *testing.T) {
	for _, c := range []struct {
		name, doc string
		ptrs      []string
	}{
		{"valid", `{"Address":"mqtt-broker","SecurePort":8883,"MaxPacketSize":262144}`, nil},
		{"valid max and extra element", `{"Address":"::1","SecurePort":65535,"MaxPacketSize":268435455,"Comment":"x"}`, nil},
		{"no object", `[]`, []string{""}},
		{"missing all", `{}`, []string{"", "", ""}},
		{"address no string", `{"Address":1,"SecurePort":8883,"MaxPacketSize":262144}`, []string{"/Address"}},
		{"address with port", `{"Address":"broker:8883","SecurePort":8883,"MaxPacketSize":262144}`, []string{"/Address"}},
		{"port 0", `{"Address":"broker","SecurePort":0,"MaxPacketSize":262144}`, []string{"/SecurePort"}},
		{"port too big", `{"Address":"broker","SecurePort":65536,"MaxPacketSize":262144}`, []string{"/SecurePort"}},
		{"port string", `{"Address":"broker","SecurePort":"8883","MaxPacketSize":262144}`, []string{"/SecurePort"}},
		{"port fraction", `{"Address":"broker","SecurePort":8883.5,"MaxPacketSize":262144}`, []string{"/SecurePort"}},
		{"size too small", `{"Address":"broker","SecurePort":8883,"MaxPacketSize":262143}`, []string{"/MaxPacketSize"}},
		{"size too big", `{"Address":"broker","SecurePort":8883,"MaxPacketSize":268435456}`, []string{"/MaxPacketSize"}},
	} {
		doc, err := payload.Parse([]byte(c.doc))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		var got []string
		for _, p := range CheckBroker(doc) {
			if p.Req != Broker {
				t.Errorf("%s: requirement %s", c.name, p.Req)
			}
			got = append(got, p.Pointer)
		}
		sort.Strings(got)
		if strings.Join(got, ",") != strings.Join(c.ptrs, ",") {
			t.Errorf("%s: pointers %q, want %q", c.name, got, c.ptrs)
		}
	}
}
