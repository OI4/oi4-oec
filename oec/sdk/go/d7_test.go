//go:build unix

package oec

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// D7 `oec-check test --level 1` against examples/hello on the test broker
// (ADR-0027 §9, C-076), with the proxy attachment, the restart hook, the
// peer and D7 announced: no Docker, loopback only. OEC_GO_D7 is the path
// of a built oec-check (oec/conformance/cmd/oec-check; run, not imported –
// D7 stays independent, ADR-0006). Without it the test is skipped;
// oec/sdk/check.sh sets it. OEC_GO_D7_REPORT keeps the JSON report.
func TestD7Level1Hello(t *testing.T) {
	// The lifecycle of the Go SDK at Level 1: connection, Birth, Will,
	// Close message on SIGTERM, restart (REQ-LIFE-*, T7-CON-*, T7-MON-*).
	// A Level 1 app answers no Get Health: the restart wait always ends by
	// its timeout.
	runD7(t, "hello", "Utility/example.com/Hello,20OEC,20Go/HELLO-GO-1/app-1", 1, []string{
		"T7-MON-01", "T7-MON-02", "T7-MON-03", "T7-MON-04", "T7-MON-05", "T7-MON-06",
		"T7-CON-01", "T7-CON-02", "T7-CON-03", "T7-CON-04",
		"T7-LIFE-01", "T7-LIFE-02", "T7-LIFE-03", "T7-LIFE-04",
	})
}

// D7 `oec-check test --level 3` against examples/source (C-077): the
// provider part of the Go SDK – Get/Set of every resource, the represented
// asset, writable Data and Config, PublicationList and SubscriptionList,
// ReferenceDesignation. OEC_GO_D7_L3_REPORT keeps the JSON report.
func TestD7Level3Source(t *testing.T) {
	runD7(t, "source", "OTConnector/example.com/Demo,20Connector,20Go/CONN-GO-1/app-1", 3, d7Level3Pass)
}

// runD7 runs oec-check test at the level against the example and fails on
// every fail, on a test of want that does not pass and on exit ≠ 0.
func runD7(t *testing.T, example, app string, level int, want []string) {
	check := os.Getenv("OEC_GO_D7")
	if check == "" {
		t.Skip("OEC_GO_D7 (path of oec-check) is not set")
	}
	b := startBroker(t)
	exe := buildExample(t, example)
	work := t.TempDir()
	proxyCA, proxyKey := proxyCA(t, work)
	proxyPort := freePort(t)

	root := b.container(t)
	write(t, filepath.Join(root, "etc", "oi4", "mqtt", "broker.json"),
		fmt.Sprintf(`{"Address": "localhost", "SecurePort": %d, "MaxPacketSize": 262144}`, proxyPort))
	ca, err := os.ReadFile(proxyCA)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "etc", "oi4", "certs", "ca.pem"), string(ca))
	password := filepath.Join(work, "password")
	write(t, password, "pa:ss\n")

	// The restart hook: SIGTERM and wait; start again in the background.
	pid := filepath.Join(work, example+".pid")
	logf := filepath.Join(work, example+".log")
	start := filepath.Join(work, "start.sh")
	stop := filepath.Join(work, "stop.sh")
	write(t, start, fmt.Sprintf("#!/bin/sh\nOEC_ROOT=%q %q >> %q 2>&1 &\necho $! > %q\n", root, exe, logf, pid))
	write(t, stop, fmt.Sprintf("#!/bin/sh\nP=$(cat %q)\nkill -TERM \"$P\"\nwhile kill -0 \"$P\" 2>/dev/null; do sleep 0.1; done\n", pid))
	for _, f := range []string{start, stop} {
		if err := os.Chmod(f, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command(start).CombinedOutput(); err != nil {
		t.Fatalf("start.sh: %v %s", err, out)
	}
	t.Cleanup(func() {
		if s, err := os.ReadFile(pid); err == nil {
			if p, err := strconv.Atoi(strings.TrimSpace(string(s))); err == nil {
				_ = syscall.Kill(p, syscall.SIGKILL)
			}
		}
		if log, err := os.ReadFile(logf); err == nil && t.Failed() {
			t.Logf("%s:\n%s", filepath.Base(logf), log)
		}
	})

	report := filepath.Join(work, "d7.json")
	cmd := exec.Command(check, "test",
		"--broker", fmt.Sprintf("mqtts://localhost:%d", b.Port), "--ca", b.CA,
		"--username", "user", "--password-file", password,
		"--app", app, "--level", strconv.Itoa(level),
		"--timeout", "3s",
		"--proxy-listen", fmt.Sprintf("127.0.0.1:%d", proxyPort),
		"--proxy-ca", proxyCA, "--proxy-ca-key", proxyKey,
		"--stop-cmd", stop, "--start-cmd", start, "--restart-wait", "15s", "--peer", "--announce",
		"--format", "text", "--report", "json="+report)
	began := time.Now()
	out, err := cmd.CombinedOutput()
	t.Logf("oec-check test --level %d: %v after %v", level, err, time.Since(began).Round(time.Second))
	if err != nil {
		t.Fatalf("oec-check test: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	keepEnv := "OEC_GO_D7_REPORT"
	if level == 3 {
		keepEnv = "OEC_GO_D7_L3_REPORT"
	}
	if keep := os.Getenv(keepEnv); keep != "" {
		if err := os.WriteFile(keep, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var r struct {
		Exit  int `json:"exit"`
		Tests []struct {
			ID      string `json:"id"`
			Outcome string `json:"outcome"`
		} `json:"tests"`
		Summary map[string]int `json:"summary"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	t.Logf("summary %v, exit %d", r.Summary, r.Exit)
	if testing.Verbose() {
		t.Logf("oec-check output:\n%s", out)
	}
	outcome := map[string]string{}
	for _, x := range r.Tests {
		outcome[x.ID] = x.Outcome
		if x.Outcome == "fail" {
			t.Errorf("%s: fail", x.ID)
		}
	}
	for _, id := range want {
		if outcome[id] != "pass" {
			t.Errorf("%s: %q, want pass", id, outcome[id])
		}
	}
	if r.Exit != 0 || t.Failed() {
		t.Errorf("exit %d\n%s", r.Exit, out)
	}
}

// d7Level3Pass: the 49 D7 tests that pass against examples/source (C-077).
// D7 skips the other six, as for the Rust demo since C-067: T7-MON-07,
// T7-MON-17, T7-ADR-01 and T7-ADR-02 judge requests of the CUT (none sent);
// T7-SET-04 and T7-SET-10 need a ReferenceDesignation object, which the
// fresh store of the run does not have.
var d7Level3Pass = []string{
	"T7-MON-01", "T7-MON-02", "T7-MON-03", "T7-MON-04", "T7-MON-05", "T7-MON-06", "T7-MON-08", "T7-MON-09", "T7-MON-10", "T7-MON-11", "T7-MON-12", "T7-MON-13", "T7-MON-14", "T7-MON-15", "T7-MON-16", "T7-MON-18", "T7-MON-19",
	"T7-CON-01", "T7-CON-02", "T7-CON-03", "T7-CON-04",
	"T7-LIFE-01", "T7-LIFE-02", "T7-LIFE-03", "T7-LIFE-04",
	"T7-ADR-03",
	"T7-GET-01", "T7-GET-02", "T7-GET-03", "T7-GET-04", "T7-GET-05", "T7-GET-06", "T7-GET-07", "T7-GET-08", "T7-GET-09",
	"T7-SET-01", "T7-SET-02", "T7-SET-03", "T7-SET-05", "T7-SET-06", "T7-SET-07", "T7-SET-08", "T7-SET-09", "T7-SET-11", "T7-SET-12",
	"T7-ACT-01", "T7-ACT-02", "T7-ACT-03", "T7-ACT-04",
}

// proxyCA writes a CA for the proxy attachment of D7 (test material only).
func proxyCA(t *testing.T, dir string) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "OEC Go test proxy CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cert, k := filepath.Join(dir, "proxy-ca.pem"), filepath.Join(dir, "proxy-ca.key")
	write(t, cert, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})))
	write(t, k, string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})))
	return cert, k
}

// freePort is a free loopback port (closed again, so another process could
// take it first – unlikely in a test).
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
