// Package hosttest builds the root directories of the host tests of
// oec-infra host: the storages of RIG 4.1 of a conforming application
// container with the test PKI of mqtttest (created at run time, never
// committed). The tests change the files of a root to inject a fault.
package hosttest

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/OI4/oec/conformance/internal/mqttwire/mqtttest"
)

// Container is the container name of a root (etc/hostname).
const Container = "oec-host-app"

// Options of a root.
type Options struct {
	Address       string // broker.json Address; "": 127.0.0.1
	Port          int    // broker.json SecurePort
	MaxPacketSize int    // broker.json MaxPacketSize; 0: 262144
	// Broker is written to broker.pem (the leaf and its chain); nil: the
	// broker certificate of the PKI.
	Broker *tls.Certificate
	// BrokerCA is copied to broker_ca.pem; "": no broker_ca.pem.
	BrokerCA string
	// Credentials "user:password" are written base64 encoded to
	// credentials; "": no credentials.
	Credentials string
	// Certificate writes the client certificate of the PKI as
	// <container>.pem, its encrypted key as private_key.pem and the
	// passphrase.
	Certificate bool
}

// Root is a root directory with the storages of RIG 4.1.
type Root struct {
	Dir, MQTT, Certs, Secrets string
}

// New writes a root to a temporary directory.
func New(t testing.TB, p *mqtttest.PKI, o Options) Root {
	t.Helper()
	dir := t.TempDir()
	r := Root{Dir: dir, MQTT: filepath.Join(dir, "etc", "oi4", "mqtt"), Certs: filepath.Join(dir, "etc", "oi4", "certs"), Secrets: filepath.Join(dir, "run", "secrets", "oi4")}
	for _, d := range []string{r.MQTT, r.Certs, r.Secrets} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	Write(t, filepath.Join(dir, "etc", "hostname"), []byte(Container+"\n"))
	if o.Address == "" {
		o.Address = "127.0.0.1"
	}
	if o.MaxPacketSize == 0 {
		o.MaxPacketSize = 262144
	}
	b, err := json.MarshalIndent(map[string]any{"Address": o.Address, "SecurePort": o.Port, "MaxPacketSize": o.MaxPacketSize}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	Write(t, filepath.Join(r.MQTT, "broker.json"), b)
	cert := o.Broker
	if cert == nil {
		cert = &p.BrokerCert
	}
	var chain []byte
	for _, der := range cert.Certificate {
		chain = append(chain, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
	}
	Write(t, filepath.Join(r.MQTT, "broker.pem"), chain)
	if o.BrokerCA != "" {
		Write(t, filepath.Join(r.MQTT, "broker_ca.pem"), Read(t, o.BrokerCA))
	}
	Write(t, filepath.Join(r.Certs, "ca.pem"), Read(t, p.CAFile))
	if o.Credentials != "" {
		Secret(t, filepath.Join(r.Secrets, "credentials"), []byte(base64.StdEncoding.EncodeToString([]byte(o.Credentials))))
	}
	if o.Certificate {
		Write(t, filepath.Join(r.Certs, Container+".pem"), Read(t, p.ClientCert))
		Secret(t, filepath.Join(r.Secrets, "private_key.pem"), Read(t, p.ClientKeyEncrypted))
		Secret(t, filepath.Join(r.Secrets, "passphrase"), []byte(base64.StdEncoding.EncodeToString([]byte(p.Passphrase))))
	}
	return r
}

// Read returns the content of a file.
func Read(t testing.TB, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Write writes a file readable for everyone (0644).
func Write(t testing.TB, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Secret writes a file readable for its owner only (0600).
func Secret(t testing.TB, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Remove removes a file or directory tree.
func Remove(t testing.TB, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
}
