package bus

import (
	"context"
	"crypto/tls"
	"net"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OI4/oec/conformance"
	"github.com/OI4/oec/conformance/catalog"
	"github.com/OI4/oec/conformance/internal/bus/registrytest"
	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/mqttwire/mqtttest"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// Users of the scenario broker.
const (
	userProbe    = "oec-probe" // privileged: D8
	userApp      = "app"       // unprivileged: --credentials-2
	userRegistry = "registry"
	registryIL   = "oi4.org/Registry/SIM/1"
	maxPacket    = MinMaxPacketSize
)

var passwords = map[string]string{userProbe: "probe-secret", userApp: "app-secret", userRegistry: "registry-secret"}

func setup(t *testing.T) ([]verdict.Test, verdict.Requirements) {
	t.Helper()
	cat, err := catalog.Load(catalog.D8File)
	if err != nil {
		t.Fatal(err)
	}
	reqs, err := verdict.LoadRequirements()
	if err != nil {
		t.Fatal(err)
	}
	return Tests(cat), reqs
}

func TestCatalogStatus(t *testing.T) {
	tests, _ := setup(t)
	for _, tc := range tests {
		if (tc.Status == "implemented") != Implemented[tc.ID] {
			t.Errorf("%s: catalog status %s, implemented here %v", tc.ID, tc.Status, Implemented[tc.ID])
		}
	}
	if len(tests) != 3 {
		t.Errorf("%d bus tests in the catalog", len(tests))
	}
}

// acl is the ACL of a scenario broker: the unprivileged user may publish
// only Pub and Get and may not read Set/Config; open lists the methods it
// may write in addition, readConfig allows subscribe (s) and read (r) of
// Set/Config.
type acl struct {
	open       []string
	subscribe  bool // Set/Config may be subscribed
	readConfig bool // Set/Config is delivered
}

func (a acl) allow(user, topic string, k mqtttest.Access) bool {
	if user != userApp {
		return true
	}
	lv := strings.Split(topic, "/")
	method, resource := "", ""
	if len(lv) > 4 {
		method, resource = lv[3], lv[4]
	}
	switch k {
	case mqtttest.Write:
		return method == "Pub" || method == "Get" || slices.Contains(a.open, method)
	case mqtttest.Subscribe:
		return !(method == "Set" && resource == "Config") || a.subscribe
	default:
		return !(method == "Set" && resource == "Config") || a.readConfig
	}
}

// closedPort returns a TCP port of 127.0.0.1 without a listener.
func closedPort(t *testing.T) int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// scenario starts the broker (and the Registry) of a bus fixture and
// returns the configuration of the run.
func scenario(t *testing.T, fault string) Config {
	t.Helper()
	p := mqtttest.NewPKI(t)
	users := map[string]string{userApp: passwords[userApp], userRegistry: passwords[userRegistry]}
	o := mqtttest.Options{Username: userProbe, Password: passwords[userProbe], Users: users, ACL: acl{}.allow,
		MaxPacket: maxPacket + 65544, MaxPayload: maxPacket}
	c := Config{MaxPacketSize: maxPacket, PlainPorts: []int{closedPort(t)}, RegistryWait: 2 * time.Second, Timeout: time.Second}
	regFault := registrytest.None
	test, variant, _ := strings.Cut(fault, "/")
	registry := test == "none" || test == TRegistry
	switch test {
	case "none":
	case TProtocol:
		switch variant {
		case "tls11":
			o.MinTLS = tls.VersionTLS11
		case "tls13-only":
			o.MinTLS = tls.VersionTLS13
		case "mtls":
			o.RequireCert = true
		case "mtls-tls11":
			o.RequireCert, o.MinTLS = true, tls.VersionTLS11
		case "plain-listener":
			o.Plain = true
		case mqtttest.CertSelfSigned, mqtttest.CertOtherCA, mqtttest.CertWrongHost, mqtttest.CertExpired:
			cert := p.ServerCert(t, variant)
			o.Cert = &cert
		case "no-311":
			o.Levels = []byte{3}
		case "packet-limit":
			// The probe PUBLISH has 1 + 3 + 2 + 65535 + 2 + 262144 octets
			// (a remaining length of 3 octets): a limit of one octet less
			// than MaxPacketSize + 65544 still accepts it.
			o.MaxPacket = maxPacket + 65544 - 2
		case "payload-limit":
			o.MaxPayload = maxPacket - 1
		default:
			t.Fatalf("unknown scenario %s", fault)
		}
	case TACL:
		switch variant {
		case "open-acl":
			o.ACL = nil
		case "call-open":
			o.ACL = acl{open: []string{"Call"}}.allow
		case "del-open":
			o.ACL = acl{open: []string{"Del"}}.allow
		case "config-readable":
			o.ACL = acl{subscribe: true, readConfig: true}.allow
		case "read-filtered":
			o.ACL = acl{subscribe: true}.allow
		case "disconnect":
			o.DisconnectDenied = true
		case "no-credentials-2":
		default:
			t.Fatalf("unknown scenario %s", fault)
		}
	case TRegistry:
		switch variant {
		case "no-registry":
			registry = false
			c.RegistryWait = 500 * time.Millisecond
		case "named":
			c.Registry = registryIL
		case registrytest.StatusEvent:
			regFault = variant
		default:
			if !slices.Contains(registrytest.Faults, variant) {
				t.Fatalf("unknown scenario %s", fault)
			}
			regFault = variant
		}
	default:
		t.Fatalf("unknown scenario %s", fault)
	}
	b := mqtttest.NewBrokerWith(t, p, o)
	if o.Plain {
		_, port, _ := net.SplitHostPort(b.PlainAddr)
		n, _ := strconv.Atoi(port)
		c.PlainPorts = []int{n}
	}
	conn := func(user string) mqttwire.Config {
		return mqttwire.Config{Broker: b.URL(), CAFile: p.CAFile, Username: user, Password: []byte(passwords[user])}
	}
	c.Main = conn(userProbe)
	if o.RequireCert {
		// D8 presents the client certificate of the PKI (CN
		// oec-test-client, privileged in the ACL).
		c.Main.Username, c.Main.Password, c.Main.CertFile, c.Main.KeyFile = "", nil, p.ClientCert, p.ClientKey
	}
	if fault != TACL+"/no-credentials-2" {
		second := conn(userApp)
		c.Second = &second
	}
	if registry && o.Cert == nil {
		registrytest.Start(t, conn(userRegistry), registryIL, regFault, 100*time.Millisecond)
	}
	return c
}

// TestFixtures runs every bus scenario of testdata/bus/index.yaml and checks
// the verdict of its selected tests against the expectation.
func TestFixtures(t *testing.T) {
	tests, reqs := setup(t)
	fixtures, err := conformance.BusFixtures()
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no bus fixtures")
	}
	for _, f := range fixtures {
		t.Run(f.Fault, func(t *testing.T) {
			t.Parallel()
			if f.Status != "draft" && f.Status != "reviewed" {
				t.Errorf("status %q", f.Status)
			}
			for id := range f.Expect {
				if !Implemented[id] || !f.Runs(id) {
					t.Errorf("expectation for %s, which is no implemented bus test of the fixture", id)
				}
			}
			c := scenario(t, f.Fault)
			c.Tests = f.Tests
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			res := Run(ctx, c)
			for _, tr := range verdict.Build(tests, res.Observations, reqs, 3) {
				if !f.Runs(tr.ID) {
					continue
				}
				want, ok := f.Expect[tr.ID]
				if !ok {
					want.Outcome = string(verdict.Pass)
				}
				if string(tr.Outcome) != want.Outcome {
					t.Errorf("%s: outcome %s, want %s (%+v)", tr.ID, tr.Outcome, want.Outcome, tr.Reqs)
					continue
				}
				var with []string
				for _, r := range tr.Reqs {
					switch {
					case string(r.Outcome) == want.Outcome:
						with = append(with, r.Req)
					case r.Outcome != verdict.Skip && !verdict.Worse(tr.Outcome, r.Outcome):
						t.Errorf("%s: %s has %s, not better than the test outcome %s", tr.ID, r.Req, r.Outcome, tr.Outcome)
					}
				}
				if want.Outcome != string(verdict.Pass) {
					w := slices.Clone(want.Reqs)
					sort.Strings(w)
					sort.Strings(with)
					if !slices.Equal(with, w) {
						t.Errorf("%s: requirements with %s %v, want %v (%+v)", tr.ID, want.Outcome, with, w, tr.Reqs)
					}
				}
				if f.Positive && tr.Outcome != verdict.Pass {
					t.Errorf("%s: a positive fixture must pass (%+v)", tr.ID, tr.Reqs)
				}
			}
		})
	}
}

// TestClientCertTLS: on a listener that requires a client certificate the
// TLS probes present the certificate of the run, so both TLS versions are
// recorded (C-071 live run against the dev broker, port 8884).
func TestClientCertTLS(t *testing.T) {
	t.Parallel()
	c := scenario(t, TProtocol+"/mtls")
	c.Tests = []string{TProtocol}
	res := Run(context.Background(), c)
	if strings.Join(res.Broker.TLS, ",") != "1.2,1.3" {
		t.Errorf("TLS %v, want 1.2 and 1.3", res.Broker.TLS)
	}
}

// TestCertificateRule: a broker certificate for a different host or outside
// its validity violates REQ-MQTT-007 under MG 6.2.2 (SF-115, decided in the
// C-071 review), so the finding cites MG 6.2.2 and is not marked proposed.
func TestCertificateRule(t *testing.T) {
	for _, variant := range []string{mqtttest.CertWrongHost, mqtttest.CertExpired} {
		t.Run(variant, func(t *testing.T) {
			t.Parallel()
			c := scenario(t, TProtocol+"/"+variant)
			c.Tests = []string{TProtocol}
			res := Run(context.Background(), c)
			found := false
			for _, o := range res.Observations {
				if o.Req != reqCert || o.Kind != verdict.Violation {
					continue
				}
				found = true
				if !strings.Contains(o.Detail, "(MG 6.2.2)") || strings.Contains(o.Detail, "proposed") {
					t.Errorf("finding %q: want the rule MG 6.2.2, not a proposed interpretation", o.Detail)
				}
			}
			if !found {
				t.Error("no violation of REQ-MQTT-007")
			}
		})
	}
}

// TestFixtureCoverage: every bus test has a positive fixture (it passes)
// and a negative fixture (an expectation worse than pass).
func TestFixtureCoverage(t *testing.T) {
	fixtures, err := conformance.BusFixtures()
	if err != nil {
		t.Fatal(err)
	}
	for id := range Implemented {
		pos, neg := false, false
		for _, f := range fixtures {
			e, ok := f.Expect[id]
			pos = pos || f.Positive && f.Runs(id) && !ok
			neg = neg || ok && e.Outcome != string(verdict.Pass)
		}
		if !pos || !neg {
			t.Errorf("%s: positive fixture %v, negative fixture %v", id, pos, neg)
		}
	}
	for _, fault := range registrytest.Faults {
		if !slices.ContainsFunc(fixtures, func(f conformance.Fixture) bool { return f.Fault == TRegistry+"/"+fault }) {
			t.Errorf("no fixture for the Registry fault %s", fault)
		}
	}
}
