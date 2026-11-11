package proxy_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/mqttwire/mqtttest"
	"github.com/OI4/oec/conformance/internal/proxy"
)

type env struct {
	pki *mqtttest.PKI
	b   *mqtttest.Broker
	p   *proxy.Proxy
}

func setup(t *testing.T) *env {
	t.Helper()
	pki := mqtttest.NewPKI(t)
	b := mqtttest.NewBroker(t, pki, "user", "secret", false)
	up, err := mqttwire.TLSConfig(mqttwire.Config{CAFile: pki.CAFile}, "localhost")
	if err != nil {
		t.Fatal(err)
	}
	_, addr, _ := mqttwire.BrokerAddress(b.URL())
	p, err := proxy.New(proxy.Config{Listen: "127.0.0.1:0", Upstream: addr, UpstreamTLS: up, CA: pki.CA, CAKey: pki.CAKey, HandshakeTimeout: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	p.Serve()
	return &env{pki, b, p}
}

func (e *env) url() string {
	return "mqtts://localhost:" + e.p.Addr()[strings.LastIndex(e.p.Addr(), ":")+1:]
}

func (e *env) dial(t *testing.T, c mqttwire.Config) (*mqttwire.Client, error) {
	t.Helper()
	c.Broker, c.CAFile = e.url(), e.pki.CAFile
	if c.Username == "" && !c.Insecure {
		c.Username, c.Password = "user", []byte("secret")
	}
	c.ConnectTimeout = 3 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return mqttwire.Dial(ctx, c)
}

func wait(t *testing.T, p *proxy.Proxy, done func([]proxy.Conn) bool) []proxy.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if !p.Wait(ctx, done) {
		t.Fatalf("timeout; connections %+v", p.Conns())
	}
	return p.Conns()
}

// TestRelay: a client connects through the proxy; the proxy records the
// CONNECT, the Will, the PUBLISH packets with their retain flag, and the
// broker delivers them to a direct subscriber. Drop ends the connection
// without DISCONNECT, and the broker publishes the Will.
func TestRelay(t *testing.T) {
	e := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sub, err := mqttwire.Dial(ctx, mqttwire.Config{Broker: e.b.URL(), CAFile: e.pki.CAFile, Username: "user", Password: []byte("secret")})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	got := make(chan mqttwire.Message, 10)
	if err := sub.Subscribe(ctx, "Oi4v2/#", 1, func(m mqttwire.Message) { got <- m }); err != nil {
		t.Fatal(err)
	}
	will := &mqttwire.Message{Topic: "Oi4v2/will", Payload: []byte(`{"will":true}`)}
	cl, err := e.dial(t, mqttwire.Config{ClientID: "cut-1", Will: will})
	if err != nil {
		t.Fatal(err)
	}
	if err := cl.Publish(ctx, mqttwire.Message{Topic: "Oi4v2/a", Payload: []byte("1"), QoS: 1, Retain: true}); err != nil {
		t.Fatal(err)
	}
	if m := <-got; m.Topic != "Oi4v2/a" || string(m.Payload) != "1" {
		t.Fatalf("relayed %+v", m)
	}
	cs := wait(t, e.p, func(cs []proxy.Conn) bool { return len(cs) == 1 && len(cs[0].Publishes) == 1 })
	if !cs[0].Subscribed.IsZero() {
		t.Errorf("subscribed before any SUBSCRIBE: %v", cs[0].Subscribed)
	}
	if err := cl.Subscribe(ctx, "Oi4v2/x/#", 0, func(mqttwire.Message) {}); err != nil {
		t.Fatal(err)
	}
	cs = wait(t, e.p, func(cs []proxy.Conn) bool { return !cs[0].Subscribed.IsZero() })
	c := cs[0]
	if !c.Relayed() || c.TLSVersion == "" || c.ClientCert || c.Connect.ClientID != "cut-1" || c.Connect.Level != 4 ||
		c.Connect.ProtocolName != "MQTT" || !c.Connect.CleanSession || c.Connect.KeepAlive != 60 ||
		!c.Connect.HasUsername || !c.Connect.HasPassword || c.Connect.Username != "user" {
		t.Errorf("connection %+v connect %+v", c, c.Connect)
	}
	if w := c.Connect.Will; w == nil || w.Topic != "Oi4v2/will" || string(w.Payload) != `{"will":true}` || w.Retain {
		t.Errorf("will %+v", w)
	}
	if p := c.Publishes[0]; p.Topic != "Oi4v2/a" || string(p.Payload) != "1" || !p.Retain || p.QoS != 1 || p.Time.IsZero() {
		t.Errorf("publish %+v", p)
	}
	if n := e.p.Drop(); n != 1 {
		t.Errorf("dropped %d", n)
	}
	select {
	case m := <-got:
		if m.Topic != "Oi4v2/will" {
			t.Errorf("after drop %+v", m)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no Will after Drop")
	}
	select {
	case <-cl.Lost():
	case <-time.After(5 * time.Second):
		t.Fatal("the client did not lose the connection")
	}
	cs = wait(t, e.p, func(cs []proxy.Conn) bool { return !cs[0].Closed.IsZero() })
	if !cs[0].Dropped || cs[0].Disconnect {
		t.Errorf("after drop %+v", cs[0])
	}
}

// TestDisconnect: a regular DISCONNECT is recorded, the broker publishes no
// Will.
func TestDisconnect(t *testing.T) {
	e := setup(t)
	cl, err := e.dial(t, mqttwire.Config{ProtocolVersion: 3})
	if err != nil {
		t.Fatal(err)
	}
	cl.Close()
	cs := wait(t, e.p, func(cs []proxy.Conn) bool { return len(cs) == 1 && !cs[0].Closed.IsZero() })
	if !cs[0].Disconnect || cs[0].Connect.Level != 3 || cs[0].Connect.ProtocolName != "MQIsdp" || cs[0].Connect.Will != nil {
		t.Errorf("%+v %+v", cs[0], cs[0].Connect)
	}
}

// TestRejectPhases: a conforming client refuses TLS 1.1, the untrusted CA
// and the wrong host name (no CONNECT), a deviating client accepts them
// (a CONNECT is recorded, never relayed).
func TestRejectPhases(t *testing.T) {
	e := setup(t)
	e.p.Reject(proxy.TLS11, proxy.UntrustedCA, proxy.WrongHost)
	for range 3 {
		if _, err := e.dial(t, mqttwire.Config{}); err == nil {
			t.Error("a conforming client connected")
		}
	}
	cs := wait(t, e.p, func(cs []proxy.Conn) bool { return len(cs) == 3 && !cs[2].Closed.IsZero() })
	for i, want := range []proxy.Phase{proxy.TLS11, proxy.UntrustedCA, proxy.WrongHost} {
		if c := cs[i]; c.Phase != want || c.Connect != nil || c.HandshakeErr == "" || c.Connack != -1 {
			t.Errorf("%d: %+v", i, c)
		}
	}
	e.p.Reject(proxy.TLS11)
	if _, err := e.dial(t, mqttwire.Config{MinTLSVersion: tls.VersionTLS10}); err == nil {
		t.Error("the TLS 1.1 phase relayed")
	}
	e.p.Reject(proxy.UntrustedCA, proxy.WrongHost)
	for range 2 {
		if _, err := e.dial(t, mqttwire.Config{Insecure: true, Username: "user", Password: []byte("secret")}); err == nil {
			t.Error("a reject phase relayed")
		}
	}
	cs = wait(t, e.p, func(cs []proxy.Conn) bool { return len(cs) == 6 && !cs[5].Closed.IsZero() })
	for i, want := range []proxy.Phase{proxy.TLS11, proxy.UntrustedCA, proxy.WrongHost} {
		c := cs[3+i]
		if c.Phase != want || c.Connect == nil || c.Connack != -1 || c.Relayed() {
			t.Errorf("%d: %+v", 3+i, c)
		}
	}
	if cs[3].TLSVersion != "TLS 1.1" {
		t.Errorf("TLS version %q", cs[3].TLSVersion)
	}
	if q := e.p.Clear(); len(q) != 0 {
		t.Errorf("queue %v", q)
	}
	e.p.Reject(proxy.TLS11)
	if q := e.p.Clear(); len(q) != 1 {
		t.Errorf("queue %v", q)
	}
	cl, err := e.dial(t, mqttwire.Config{})
	if err != nil {
		t.Fatal(err)
	}
	cl.Close()
}

// TestClientCertificate: a client certificate is seen; the broker refuses
// the connection without credentials (CONNACK 5 is recorded).
func TestClientCertificate(t *testing.T) {
	e := setup(t)
	_, err := e.dial(t, mqttwire.Config{CertFile: e.pki.ClientCert, KeyFile: e.pki.ClientKey, Username: "x", Password: []byte("y")})
	if err == nil {
		t.Fatal("wrong password connected")
	}
	cs := wait(t, e.p, func(cs []proxy.Conn) bool { return len(cs) == 1 && cs[0].Connack >= 0 })
	if !cs[0].ClientCert || cs[0].Connack == 0 || cs[0].Relayed() {
		t.Errorf("%+v", cs[0])
	}
}

// TestUpstreamIdentity (C-113 RT-1): the client certificate in UpstreamTLS
// is D7's own identity; a relayed client never gets it. Against a broker
// that maps the client certificate to the user, a client without
// credentials is not relayed; only the explicit UpstreamCert (the identity
// of the CUT) is presented upstream.
func TestUpstreamIdentity(t *testing.T) {
	pki := mqtttest.NewPKI(t)
	b := mqtttest.NewBroker(t, pki, "", "", true)
	probe := mqttwire.Config{CAFile: pki.CAFile, CertFile: pki.ClientCert, KeyFile: pki.ClientKey}
	up, err := mqttwire.TLSConfig(probe, "localhost")
	if err != nil {
		t.Fatal(err)
	}
	if len(up.Certificates) != 1 {
		t.Fatal("the upstream TLS settings carry no client certificate")
	}
	_, addr, _ := mqttwire.BrokerAddress(b.URL())
	dial := func(cut *tls.Certificate) (proxy.Conn, error) {
		t.Helper()
		p, err := proxy.New(proxy.Config{Listen: "127.0.0.1:0", Upstream: addr, UpstreamTLS: up, UpstreamCert: cut, CA: pki.CA, CAKey: pki.CAKey, HandshakeTimeout: 3 * time.Second})
		if err != nil {
			t.Fatal(err)
		}
		defer p.Close()
		p.Serve()
		e := &env{pki, b, p}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cl, err := mqttwire.Dial(ctx, mqttwire.Config{Broker: e.url(), CAFile: pki.CAFile, ConnectTimeout: 3 * time.Second})
		if err == nil {
			cl.Close()
		}
		return wait(t, p, func(cs []proxy.Conn) bool { return len(cs) == 1 && !cs[0].Closed.IsZero() })[0], err
	}
	c, err := dial(nil)
	if err == nil || c.Relayed() || c.Connect == nil || c.Connect.HasUsername {
		t.Errorf("a client without credentials was relayed with D7's identity: %v %+v", err, c)
	}
	cut := up.Certificates[0]
	if c, err := dial(&cut); err != nil || !c.Relayed() {
		t.Errorf("with UpstreamCert: %v %+v", err, c)
	}
	if len(up.Certificates) != 1 {
		t.Error("proxy.New changed the caller's UpstreamTLS")
	}
}

func str(s string) []byte {
	b := binary.BigEndian.AppendUint16(nil, uint16(len(s)))
	return append(b, s...)
}

// raw writes one packet to the proxy and returns the recorded connection.
func raw(t *testing.T, e *env, header byte, body []byte) proxy.Conn {
	t.Helper()
	n := len(e.p.Conns())
	c, err := tls.Dial("tcp", e.p.Addr(), &tls.Config{ServerName: "localhost", RootCAs: rootCAs(e)})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	pkt := []byte{header}
	l := len(body)
	for {
		b := byte(l % 128)
		l /= 128
		if l > 0 {
			b |= 0x80
		}
		pkt = append(pkt, b)
		if l == 0 {
			break
		}
	}
	if _, err := c.Write(append(pkt, body...)); err != nil {
		t.Fatal(err)
	}
	return wait(t, e.p, func(cs []proxy.Conn) bool {
		return len(cs) > n && (cs[n].Err != "" || cs[n].Connack >= 0 || !cs[n].Closed.IsZero())
	})[n]
}

func rootCAs(e *env) *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(e.pki.CA)
	return pool
}

// TestParseConnect: MQTT 5.0 CONNECT with properties; malformed packets.
func TestParseConnect(t *testing.T) {
	e := setup(t)
	// MQTT 5.0: properties (session expiry), Will with properties, user
	// name and password.
	body := append(str("MQTT"), 5, 0x02|0x04|0x80|0x40, 0, 30)
	body = append(body, 5, 0x11, 0, 0, 0, 0)
	body = append(body, str("id5")...)
	body = append(body, 0)
	body = append(body, str("Oi4v2/w")...)
	body = append(body, str("{}")...)
	body = append(body, str("user")...)
	body = append(body, str("secret")...)
	c := raw(t, e, 0x10, body)
	if c.Connect == nil || c.Connect.Level != 5 || c.Connect.ClientID != "id5" || c.Connect.KeepAlive != 30 ||
		c.Connect.Will == nil || c.Connect.Will.Topic != "Oi4v2/w" || !c.Connect.HasPassword {
		t.Errorf("v5 %+v %+v", c, c.Connect)
	}
	for name, b := range map[string]struct {
		h    byte
		body []byte
	}{
		"not CONNECT":    {0x30, append(str("t"), 'x')},
		"reserved flags": {0x11, append(str("MQTT"), 4, 0x02, 0, 60, 0, 0)},
		"protocol name":  {0x10, append(str("MQTX"), 4, 0x02, 0, 60, 0, 0)},
		"MQTT level 3":   {0x10, append(str("MQTT"), 3, 0x02, 0, 60, 0, 0)},
		"reserved bit":   {0x10, append(str("MQTT"), 4, 0x03, 0, 60, 0, 0)},
		"will retain":    {0x10, append(str("MQTT"), 4, 0x22, 0, 60, 0, 0)},
		"truncated":      {0x10, append(str("MQTT"), 4, 0x82, 0, 60, 0, 0)},
	} {
		if c := raw(t, e, b.h, b.body); c.Err == "" || c.Connack != -1 {
			t.Errorf("%s: %+v", name, c)
		}
	}
}
