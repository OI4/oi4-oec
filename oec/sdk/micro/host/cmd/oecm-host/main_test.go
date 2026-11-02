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

// d7Level1Pass: the 21 D7 tests that pass against the micro core in the
// host runner at Level 1 (C-093) – the config service's list (C-082)
// without T7-ADR-01: the device sends no requests. T7-ADR-03 judges the
// Identification Links of its publications. A Level 1 device answers no
// Get, Set or Call (REQ-CMP-017 applies from Level 2): D7 warns on five
// tests for the missing SHOULD answers (T7-GET-05, T7-SET-01, T7-SET-02,
// T7-SET-06, T7-SET-07) and skips the other 29 – the Level 2 and 3 tests
// (T7-MON-13, T7-SET-03), the request tests without an answer to judge
// (T7-GET-01 … 04, 06 … 09, T7-SET-04, 05, 08 … 12, T7-ACT-*), T7-ADR-01
// (no request sent), and the monitor tests whose trigger the run does not
// contain (T7-MON-07, 09, 14, 15, 17, 18, T7-ADR-02).
var d7Level1Pass = []string{
	"T7-MON-01", "T7-MON-02", "T7-MON-03", "T7-MON-04", "T7-MON-05", "T7-MON-06", "T7-MON-08", "T7-MON-10", "T7-MON-11", "T7-MON-12", "T7-MON-16", "T7-MON-19",
	"T7-CON-01", "T7-CON-02", "T7-CON-03", "T7-CON-04",
	"T7-LIFE-01", "T7-LIFE-02", "T7-LIFE-03", "T7-LIFE-04",
	"T7-ADR-03",
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
		Pass:     d7Level1Pass,
		Report:   "OECM_D7_REPORT",
	})
}

// d7Level2Pass: the 38 D7 tests that pass against the micro core in the
// host runner at Level 2 (C-094), the simulated temperature sensor of
// demo.c. D7 warns on three SHOULDs of features of Level 3 of the micro
// SDK (C-095): ReferenceDesignation (T7-GET-03, T7-GET-04) and
// NewDataSetWriterId (T7-ACT-01). It skips 14: T7-GET-07, T7-SET-04,
// T7-SET-09 and T7-SET-10 need ReferenceDesignation; T7-ACT-02 … 04 need
// NewDataSetWriterId; T7-SET-12 needs a Syslog publication (the micro SDK
// has none, ADR-0030); T7-MON-07, T7-ADR-01 and T7-ADR-02 judge requests
// of the CUT (none sent); T7-MON-14 and T7-MON-17 observed nothing to
// judge; T7-SET-03 is Level 3.
var d7Level2Pass = []string{
	"T7-MON-01", "T7-MON-02", "T7-MON-03", "T7-MON-04", "T7-MON-05", "T7-MON-06", "T7-MON-08", "T7-MON-09", "T7-MON-10", "T7-MON-11", "T7-MON-12", "T7-MON-13", "T7-MON-15", "T7-MON-16", "T7-MON-18", "T7-MON-19",
	"T7-CON-01", "T7-CON-02", "T7-CON-03", "T7-CON-04",
	"T7-LIFE-01", "T7-LIFE-02", "T7-LIFE-03", "T7-LIFE-04",
	"T7-ADR-03",
	"T7-GET-01", "T7-GET-02", "T7-GET-05", "T7-GET-06", "T7-GET-08", "T7-GET-09",
	"T7-SET-01", "T7-SET-02", "T7-SET-05", "T7-SET-06", "T7-SET-07", "T7-SET-08", "T7-SET-11",
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
		Pass:     d7Level2Pass,
		Report:   "OECM_D7_REPORT",
	})
}

// d7Level3Pass: the 50 D7 tests that pass against the micro core in the
// host runner at Level 3 (C-095): the Level 2 list, T7-SET-03, the
// ReferenceDesignation tests (T7-GET-03, T7-GET-04, T7-GET-07, T7-SET-04,
// T7-SET-09, T7-SET-10; the run starts with a stored ReferenceDesignation,
// refdesSeed), T7-MON-14 (its publication on change) and the
// NewDataSetWriterId tests (T7-ACT-01 … 04). D7 skips five: T7-SET-12
// needs a Syslog publication (none in the micro SDK); T7-MON-07, T7-ADR-01
// and T7-ADR-02 judge requests of the CUT (none sent); T7-MON-17 observed
// no Set Config of a group with mandatory objects.
var d7Level3Pass = append(append([]string{}, d7Level2Pass...),
	"T7-MON-14", "T7-GET-03", "T7-GET-04", "T7-GET-07", "T7-SET-03", "T7-SET-04", "T7-SET-09", "T7-SET-10",
	"T7-ACT-01", "T7-ACT-02", "T7-ACT-03", "T7-ACT-04")

// refdesSeed is a stored ReferenceDesignation in the form the core stores
// (canonical member order, oecm_l3.c), as after a Set at commissioning.
const refdesSeed = `{"Function":{"Value":"=OEC.MICRO","Parent":{"Value":"=OEC","IdentificationLink":"oi4.org/OEC/OEC/1"}},"Location":{"Value":"+HOST"}}`

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
		Pass:     d7Level3Pass,
		Report:   "OECM_D7_REPORT",
		Files:    map[string]string{"var/lib/oecm-host/oecm.refdes": refdesSeed},
	})
}
