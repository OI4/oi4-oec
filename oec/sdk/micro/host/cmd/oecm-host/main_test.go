package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OI4/oec/sdk/micro/host/internal/d7run"
)

// TestMain runs the test binary as the device program when the D7
// harness starts it (OECM_HOST_MAIN=1): the runner under test is the code
// of this package, without a second build.
func TestMain(m *testing.M) {
	if os.Getenv("OECM_HOST_MAIN") == "1" {
		os.Exit(run(os.Args[1:], os.Getenv("OEC_ROOT")))
	}
	os.Exit(m.Run())
}

func TestStorage(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		d7run.Write(t, filepath.Join(root, name), content)
	}
	if _, err := loadStorage(root); err == nil || !strings.Contains(err.Error(), "broker.json") {
		t.Fatalf("empty storage: %v", err)
	}
	write("etc/hostname", "dev-1\n")
	write("etc/oi4/mqtt/broker.json", `{"Address": "broker.local", "SecurePort": 0, "MaxPacketSize": 1000}`)
	if _, err := loadStorage(root); err == nil || !strings.Contains(err.Error(), "SecurePort") {
		t.Fatalf("SecurePort 0: %v", err)
	}
	write("etc/oi4/mqtt/broker.json", `{"Address": "broker.local", "SecurePort": 8883, "MaxPacketSize": 1000, "Extra": 1}`)
	if _, err := loadStorage(root); err == nil || !strings.Contains(err.Error(), "CA") {
		t.Fatalf("no CA: %v", err)
	}
	write("etc/oi4/certs/ca.pem", selfSigned(t))
	if _, err := loadStorage(root); err == nil || !strings.Contains(err.Error(), "authentication") {
		t.Fatalf("no credentials: %v", err)
	}
	write("run/secrets/oi4/credentials", "bm9jb2xvbg==\n") // "nocolon"
	if _, err := loadStorage(root); err == nil || !strings.Contains(err.Error(), "REQ-CNT-009") {
		t.Fatalf("credentials without colon: %v", err)
	}
	write("run/secrets/oi4/credentials", "dXNlcjpwYTpzcw==\n") // "user:pa:ss"
	st, err := loadStorage(root)
	if err != nil {
		t.Fatal(err)
	}
	if st.name != "dev-1" || st.addr != "broker.local:8883" || st.maxPacket != 1000 ||
		*st.username != "user" || *st.password != "pa:ss" || st.auth != "credentials" ||
		st.tls.MinVersion == 0 || st.tls.RootCAs == nil {
		t.Fatalf("storage %+v", st)
	}
	write("run/secrets/oi4/private_key.pem", "x")
	write("etc/oi4/certs/dev-1.pem", "x")
	if _, err := loadStorage(root); err == nil || !strings.Contains(err.Error(), "client certificate") {
		t.Fatalf("client certificate wins, invalid: %v", err)
	}
}

func selfSigned(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestD7Level1(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	d7run.Run(t, d7run.D7{
		Exe:      exe,
		Env:      []string{"OECM_HOST_MAIN=1"},
		Hostname: "dev-1",
		App:      "OTConnector/oi4.org/OEC,20Micro,20Host/OECM-HOST/dev-1",
		Level:    1,
		Pass:     d7run.Level1Pass,
		Report:   "OECM_D7_REPORT",
	})
}

func TestD7Level2(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	d7run.Run(t, d7run.D7{
		Exe:      exe,
		Args:     []string{"-level", "2"},
		Env:      []string{"OECM_HOST_MAIN=1"},
		Hostname: "dev-1",
		App:      "OTConnector/oi4.org/OEC,20Micro,20Host/OECM-HOST/dev-1",
		Level:    2,
		Pass:     d7run.Level2Pass,
		Report:   "OECM_D7_REPORT",
	})
}

func TestD7Level3(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	d7run.Run(t, d7run.D7{
		Exe:      exe,
		Args:     []string{"-level", "3"},
		Env:      []string{"OECM_HOST_MAIN=1"},
		Hostname: "dev-1",
		App:      "OTConnector/oi4.org/OEC,20Micro,20Host/OECM-HOST/dev-1",
		Level:    3,
		Pass:     d7run.Level3Pass,
		Report:   "OECM_D7_REPORT",
		Files:    map[string]string{"var/lib/oecm-host/oecm.refdes": d7run.RefdesSeed},
	})
}
