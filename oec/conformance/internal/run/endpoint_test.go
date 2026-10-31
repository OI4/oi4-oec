package run

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/run/simapp"
	"github.com/OI4/oec/conformance/internal/topic"
)

// testEndpoint starts an endpoint for the broker name "broker" with the
// credentials user/pass and a MaxPacketSize of 200.
func testEndpoint(t *testing.T) (*endpoint, *pki) {
	t.Helper()
	p, err := newPKI()
	if err != nil {
		t.Fatal(err)
	}
	_, cert, err := p.b.broker("broker")
	if err != nil {
		t.Fatal(err)
	}
	spec, err := topic.Load()
	if err != nil {
		t.Fatal(err)
	}
	ep, err := listen("127.0.0.1", cert, p.a.pool(), "user", []byte("pass"), 200, spec)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ep.Close)
	return ep, p
}

// clientCert returns a TLS client certificate of the authority.
func clientCert(t *testing.T, a *authority) tls.Certificate {
	t.Helper()
	certPEM, keyPEM, err := a.client("oec-d8-run-test", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	blk, _ := pem.Decode(keyPEM)
	der, err := mqttwire.DecryptPKCS8(blk.Bytes, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := key.(*ecdsa.PrivateKey); !ok {
		t.Fatalf("key %T", key)
	}
	plain, _ := x509.MarshalPKCS8PrivateKey(key)
	c, err := tls.X509KeyPair(certPEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: plain}))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func dialTLS(t *testing.T, ep *endpoint, p *pki, certs ...tls.Certificate) *tls.Conn {
	t.Helper()
	c, err := tls.Dial("tcp", "127.0.0.1:"+strconv.Itoa(ep.Port()), &tls.Config{ServerName: "broker", RootCAs: p.b.pool(), Certificates: certs, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	_ = c.SetDeadline(time.Now().Add(10 * time.Second))
	return c
}

func str(s string) []byte { return append([]byte{byte(len(s) >> 8), byte(len(s))}, s...) }

func packet(kind byte, body []byte) []byte {
	out := []byte{kind}
	n := len(body)
	for {
		b := byte(n % 128)
		n /= 128
		if n > 0 {
			b |= 0x80
		}
		out = append(out, b)
		if n == 0 {
			break
		}
	}
	return append(out, body...)
}

// connectPacket builds a CONNECT; a nil user or password is left out.
func connectPacket(level byte, clientID string, user, pass *string) []byte {
	name := "MQTT"
	if level == 3 {
		name = "MQIsdp"
	}
	var flags byte = 0x02
	if user != nil {
		flags |= 0x80
	}
	if pass != nil {
		flags |= 0x40
	}
	body := append(str(name), level, flags, 0, 60)
	if level == 5 {
		body = append(body, 0)
	}
	body = append(body, str(clientID)...)
	if user != nil {
		body = append(body, str(*user)...)
	}
	if pass != nil {
		body = append(body, str(*pass)...)
	}
	return packet(0x10, body)
}

func expect(t *testing.T, r io.Reader, want []byte) {
	t.Helper()
	got := make([]byte, len(want))
	if _, err := io.ReadFull(r, got); err != nil {
		t.Fatalf("want %x: %v", want, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}
}

func last(t *testing.T, ep *endpoint, n int) conn {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		s := ep.Snapshot()
		if len(s) >= n || time.Now().After(deadline) {
			if len(s) < n {
				t.Fatalf("%d connections, want %d", len(s), n)
			}
			return s[n-1]
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func ptr(s string) *string { return &s }

// TestEndpointCredentials: MQTT 5.0 with the credentials – CONNACK,
// SUBACK, PUBACK, PINGRESP, the Birth and the payload sizes.
func TestEndpointCredentials(t *testing.T) {
	ep, p := testEndpoint(t)
	c := dialTLS(t, ep, p)
	if _, err := c.Write(connectPacket(5, "oec-d8-run-x", ptr("user"), ptr("pass"))); err != nil {
		t.Fatal(err)
	}
	expect(t, c, []byte{0x20, 3, 0, 0, 0})
	sub := append([]byte{0, 7, 0}, str("Oi4v2/#")...)
	_, _ = c.Write(packet(0x82, append(sub, 1)))
	expect(t, c, []byte{0x90, 4, 0, 7, 0, 0})
	il := simapp.IdentificationLink(simapp.Good, "oec-d8-run-x")
	mam := simapp.MAM(simapp.Good, "oec-d8-run-x")
	pub := append(append(str(simapp.Topic("MAM", il)), 0, 9, 0), mam...)
	_, _ = c.Write(packet(0x32, pub))
	expect(t, c, []byte{0x40, 2, 0, 9})
	big := append(append(str("Oi4v2/x"), 0), bytes.Repeat([]byte("x"), 1000)...)
	_, _ = c.Write(packet(0x30, big))
	_, _ = c.Write([]byte{0xC0, 0})
	expect(t, c, []byte{0xD0, 0})
	got := last(t, ep, 1)
	if got.Handshake != "ok" || got.ClientCertificate != "none" || got.Auth != "credentials" || !got.Accepted || got.ProtocolLevel != 5 || got.ClientID != "oec-d8-run-x" {
		t.Errorf("connection %+v", got.RunConnection)
	}
	if got.Publishes != 2 || got.LargestPayload != 1000 || got.Oversized != 2 || len(mam) <= 200 || len(mam) >= 1000 {
		t.Errorf("publishes %+v", got.RunConnection)
	}
	if got.birth == nil || got.birth.IdentificationLink != il || got.birth.SerialNumber != "oec-d8-run-x" {
		t.Errorf("birth %+v", got.birth)
	}
}

// TestEndpointRefused: wrong password (MQTT 3.1), no credentials (3.1.1),
// a certificate of an unknown CA (5.0) and a verified certificate.
func TestEndpointRefused(t *testing.T) {
	ep, p := testEndpoint(t)
	for i, tc := range []struct {
		level      byte
		user, pass *string
		cert       *authority
		ack        []byte
		auth, cc   string
	}{
		{3, ptr("user"), ptr("wrong"), nil, []byte{0x20, 2, 0, 4}, "credentials-wrong", "none"},
		{4, nil, nil, nil, []byte{0x20, 2, 0, 5}, "none", "none"},
		{5, nil, nil, p.x, []byte{0x20, 3, 0, 0x87, 0}, "none", "untrusted"},
		{5, ptr("user"), nil, nil, []byte{0x20, 3, 0, 0x86, 0}, "credentials-wrong", "none"},
		{4, nil, nil, p.a, []byte{0x20, 2, 0, 0}, "certificate", "verified"},
	} {
		var certs []tls.Certificate
		if tc.cert != nil {
			certs = append(certs, clientCert(t, tc.cert))
		}
		c := dialTLS(t, ep, p, certs...)
		_, _ = c.Write(connectPacket(tc.level, "c"+strconv.Itoa(i), tc.user, tc.pass))
		expect(t, c, tc.ack)
		got := last(t, ep, i+1)
		if got.Auth != tc.auth || got.ClientCertificate != tc.cc || got.Accepted != (tc.auth == "certificate") || got.ProtocolLevel != int(tc.level) {
			t.Errorf("%d: %+v", i, got.RunConnection)
		}
		c.Close()
	}
}

// TestEndpointLimits: a packet above the packet limit of the broker ends
// the connection; a client that does not trust the broker certificate
// fails the handshake.
func TestEndpointLimits(t *testing.T) {
	ep, p := testEndpoint(t)
	c := dialTLS(t, ep, p)
	_, _ = c.Write(connectPacket(4, "c", ptr("user"), ptr("pass")))
	expect(t, c, []byte{0x20, 2, 0, 0})
	n := 200 + BrokerOverhead + 1
	_, _ = c.Write(packet(0x30, append(str("t"), make([]byte, n-3)...)))
	if _, err := c.Read(make([]byte, 1)); err == nil {
		t.Error("the connection is still open")
	}
	got := last(t, ep, 1)
	if got.Oversized != 1 || got.Publishes != 0 {
		t.Errorf("oversized %+v", got.RunConnection)
	}

	raw, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(ep.Port()))
	if err != nil {
		t.Fatal(err)
	}
	tc := tls.Client(raw, &tls.Config{ServerName: "broker", RootCAs: p.x.pool(), MinVersion: tls.VersionTLS12})
	if err := tc.Handshake(); err == nil {
		t.Fatal("handshake with an unknown CA")
	}
	raw.Close()
	got = last(t, ep, 2)
	if got.Handshake != "failed" || got.HandshakeError == "" || got.Accepted {
		t.Errorf("handshake %+v", got.RunConnection)
	}
}

func TestParseConnect(t *testing.T) {
	for _, b := range [][]byte{
		connectPacket(4, "c", nil, nil)[:6],
		packet(0x10, append(str("MQTT"), 6, 0, 0, 60)),
		packet(0x10, append(str("MQIsdp"), 4, 0, 0, 60)),
	} {
		if _, err := parseConnect(frame{header: b[0], body: b[2:]}); err == nil {
			t.Errorf("%x parsed", b)
		}
	}
	b := connectPacket(5, "id", ptr("u"), ptr("p"))
	c, err := parseConnect(frame{header: b[0], body: b[2:]})
	if err != nil || c.level != 5 || c.clientID != "id" || c.user != "u" || string(c.pass) != "p" {
		t.Errorf("%+v %v", c, err)
	}
}
