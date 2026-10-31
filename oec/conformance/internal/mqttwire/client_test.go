package mqttwire_test

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/mqttwire/mqtttest"
)

func TestBrokerAddress(t *testing.T) {
	for in, want := range map[string]string{
		"mqtts://localhost":       "localhost:8883",
		"mqtts://localhost:8884":  "localhost:8884",
		"ssl://10.0.0.1:1234":     "10.0.0.1:1234",
		"tls://[::1]:8883":        "[::1]:8883",
		"mqtts://broker.example/": "broker.example:8883",
	} {
		if _, got, err := mqttwire.BrokerAddress(in); err != nil || got != want {
			t.Errorf("%s: %q %v, want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"mqtt://localhost:1883", "tcp://localhost:1883", "wss://x", "localhost:8883", "mqtts://u:p@h", "mqtts://h/x", "http://h"} {
		if _, _, err := mqttwire.BrokerAddress(in); err == nil {
			t.Errorf("%s: no error", in)
		}
	}
}

func TestDecryptPKCS8(t *testing.T) {
	p := mqtttest.NewPKI(t)
	raw, _ := os.ReadFile(p.ClientKeyEncrypted)
	b, _ := pem.Decode(raw)
	der, err := mqttwire.DecryptPKCS8(b.Bytes, []byte(p.Passphrase))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := x509.ParsePKCS8PrivateKey(der); err != nil {
		t.Fatal(err)
	}
	if _, err := mqttwire.DecryptPKCS8(b.Bytes, []byte("wrong")); err == nil || !strings.Contains(err.Error(), "passphrase") {
		t.Errorf("wrong passphrase: %v", err)
	}
	if _, err := mqttwire.DecryptPKCS8([]byte{0x30, 0x00}, nil); err == nil {
		t.Error("garbage: no error")
	}
}

func TestTLSConfig(t *testing.T) {
	p := mqtttest.NewPKI(t)
	tc, err := mqttwire.TLSConfig(mqttwire.Config{CAFile: p.CAFile, CertFile: p.ClientCert, KeyFile: p.ClientKeyEncrypted, KeyPassphrase: []byte(p.Passphrase)}, "localhost")
	if err != nil {
		t.Fatal(err)
	}
	if tc.MinVersion != 0x0303 || tc.MaxVersion != 0x0304 || len(tc.Certificates) != 1 || tc.InsecureSkipVerify || tc.RootCAs == nil {
		t.Errorf("tls config %+v", tc)
	}
	for name, c := range map[string]mqttwire.Config{
		"no passphrase": {CertFile: p.ClientCert, KeyFile: p.ClientKeyEncrypted},
		"cert only":     {CertFile: p.ClientCert},
		"no CA":         {CAFile: filepath.Join(p.Dir, "missing.pem")},
		"CA is a key":   {CAFile: p.ClientKey},
		"key mismatch":  {CertFile: p.CAFile, KeyFile: p.ClientKey},
	} {
		if _, err := mqttwire.TLSConfig(c, "localhost"); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func dial(t *testing.T, c mqttwire.Config) (*mqttwire.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return mqttwire.Dial(ctx, c)
}

func TestClientPassword(t *testing.T) {
	p := mqtttest.NewPKI(t)
	b := mqtttest.NewBroker(t, p, "user", "secret", false)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sub, err := dial(t, mqttwire.Config{Broker: b.URL(), CAFile: p.CAFile, Username: "user", Password: []byte("secret")})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	got := make(chan mqttwire.Message, 10)
	if err := sub.Subscribe(ctx, "Oi4v2/#", 1, func(m mqttwire.Message) { got <- m }); err != nil {
		t.Fatal(err)
	}
	will := &mqttwire.Message{Topic: "Oi4v2/will", Payload: []byte(`{"will":true}`)}
	pub, err := dial(t, mqttwire.Config{Broker: b.URL(), CAFile: p.CAFile, Username: "user", Password: []byte("secret"), Will: will})
	if err != nil {
		t.Fatal(err)
	}
	for i, m := range []mqttwire.Message{{Topic: "Oi4v2/a", Payload: []byte("1")}, {Topic: "Oi4v2/b", Payload: []byte("2"), QoS: 1}, {Topic: "other", Payload: []byte("3")}} {
		if err := pub.Publish(ctx, m); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}
	pub.Kill()
	want := []mqttwire.Message{{Topic: "Oi4v2/a", Payload: []byte("1")}, {Topic: "Oi4v2/b", Payload: []byte("2"), QoS: 1}, *will}
	for _, w := range want {
		select {
		case m := <-got:
			if m.Topic != w.Topic || string(m.Payload) != string(w.Payload) || m.QoS != w.QoS || m.Retain || m.Received.IsZero() {
				t.Errorf("got %+v, want %+v", m, w)
			}
		case <-ctx.Done():
			t.Fatalf("timeout waiting for %s", w.Topic)
		}
	}

	if _, err := dial(t, mqttwire.Config{Broker: b.URL(), CAFile: p.CAFile, Username: "user", Password: []byte("wrong")}); err == nil {
		t.Error("wrong password: connected")
	}
	if _, err := dial(t, mqttwire.Config{Broker: b.URL(), CAFile: p.OtherCAFile, Username: "user", Password: []byte("secret")}); err == nil {
		t.Error("wrong CA: connected")
	}
	ins, err := dial(t, mqttwire.Config{Broker: b.URL(), CAFile: p.OtherCAFile, Insecure: true, Username: "user", Password: []byte("secret")})
	if err != nil {
		t.Fatalf("insecure: %v", err)
	}
	ins.Close()

	b.Close()
	select {
	case <-sub.Lost():
	case <-ctx.Done():
		t.Fatal("lost connection not reported")
	}
}

func TestClientCertificate(t *testing.T) {
	p := mqtttest.NewPKI(t)
	b := mqtttest.NewBroker(t, p, "", "", true)
	c, err := dial(t, mqttwire.Config{Broker: b.URL(), CAFile: p.CAFile, CertFile: p.ClientCert, KeyFile: p.ClientKeyEncrypted, KeyPassphrase: []byte(p.Passphrase)})
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	if _, err := dial(t, mqttwire.Config{Broker: b.URL(), CAFile: p.CAFile}); err == nil {
		t.Error("no client certificate: connected")
	}
}

func TestClientDialer(t *testing.T) {
	p := mqtttest.NewPKI(t)
	b := mqtttest.NewBroker(t, p, "", "", true)
	var got string
	cfg := mqttwire.Config{Broker: "mqtts://localhost:1", CAFile: p.CAFile, CertFile: p.ClientCert, KeyFile: p.ClientKeyEncrypted, KeyPassphrase: []byte(p.Passphrase),
		Dialer: func(ctx context.Context, network, addr string) (net.Conn, error) {
			got = addr
			var d net.Dialer
			return d.DialContext(ctx, network, b.Addr)
		}}
	c, err := dial(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	if got != "localhost:1" {
		t.Errorf("dialer address %q", got)
	}
	cfg.CAFile = p.OtherCAFile
	if _, err := dial(t, cfg); err == nil {
		t.Error("untrusted broker certificate: connected")
	}
}

func TestEncryptPKCS8(t *testing.T) {
	p := mqtttest.NewPKI(t)
	raw, _ := os.ReadFile(p.ClientKeyEncrypted)
	b, _ := pem.Decode(raw)
	der, err := mqttwire.DecryptPKCS8(b.Bytes, []byte(p.Passphrase))
	if err != nil {
		t.Fatal(err)
	}
	enc, err := mqttwire.EncryptPKCS8(der, []byte("a:b"))
	if err != nil {
		t.Fatal(err)
	}
	back, err := mqttwire.DecryptPKCS8(enc, []byte("a:b"))
	if err != nil || !bytes.Equal(back, der) {
		t.Fatalf("round trip: %v", err)
	}
}

func TestReadStorage(t *testing.T) {
	p := mqtttest.NewPKI(t)
	root := t.TempDir()
	put := func(rel, content string) {
		path := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path string) string { b, _ := os.ReadFile(path); return string(b) }
	if _, err := mqttwire.ReadStorage(root); err == nil {
		t.Error("empty storage: no error")
	}
	put("etc/oi4/mqtt/broker.json", `{"Address":"localhost","SecurePort":8884,"MaxPacketSize":262144}`)
	put("etc/oi4/certs/ca.pem", read(p.CAFile))
	if _, err := mqttwire.ReadStorage(root); err == nil {
		t.Error("storage without login: no error")
	}
	put("etc/hostname", "cut\n")
	put("etc/oi4/certs/cut.pem", read(p.ClientCert))
	put("run/secrets/oi4/private_key.pem", read(p.ClientKeyEncrypted))
	put("run/secrets/oi4/passphrase", base64.StdEncoding.EncodeToString([]byte(p.Passphrase))+"\n")
	put("run/secrets/oi4/credentials", base64.StdEncoding.EncodeToString([]byte("user:pa:ss"))+"\n")
	s, err := mqttwire.ReadStorage(root)
	if err != nil {
		t.Fatal(err)
	}
	c := s.Config
	if c.Broker != "mqtts://localhost:8884" || s.MaxPacketSize != 262144 || !strings.HasSuffix(c.CAFile, "certs/ca.pem") ||
		!strings.HasSuffix(c.CertFile, "cut.pem") || string(c.KeyPassphrase) != p.Passphrase || c.Username != "user" || string(c.Password) != "pa:ss" {
		t.Errorf("storage %+v", s)
	}
	if _, err := mqttwire.TLSConfig(c, "localhost"); err != nil {
		t.Errorf("tls from storage: %v", err)
	}
	put("etc/oi4/mqtt/broker_ca.pem", read(p.OtherCAFile))
	if s, _ := mqttwire.ReadStorage(root); s == nil || !strings.HasSuffix(s.Config.CAFile, "broker_ca.pem") {
		t.Errorf("broker_ca.pem not preferred: %+v", s)
	}
}
