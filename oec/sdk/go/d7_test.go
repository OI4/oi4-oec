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
	check := os.Getenv("OEC_GO_D7")
	if check == "" {
		t.Skip("OEC_GO_D7 (path of oec-check) is not set")
	}
	b := startBroker(t)
	exe := buildExample(t, "hello")
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
	pid := filepath.Join(work, "hello.pid")
	start := filepath.Join(work, "start.sh")
	stop := filepath.Join(work, "stop.sh")
	write(t, start, fmt.Sprintf("#!/bin/sh\nOEC_ROOT=%q %q >> %q 2>&1 &\necho $! > %q\n",
		root, exe, filepath.Join(work, "hello.log"), pid))
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
		if log, err := os.ReadFile(filepath.Join(work, "hello.log")); err == nil && t.Failed() {
			t.Logf("hello.log:\n%s", log)
		}
	})

	report := filepath.Join(work, "d7.json")
	cmd := exec.Command(check, "test",
		"--broker", fmt.Sprintf("mqtts://localhost:%d", b.Port), "--ca", b.CA,
		"--username", "user", "--password-file", password,
		"--app", "Utility/example.com/Hello,20OEC,20Go/HELLO-GO-1/app-1", "--level", "1",
		"--timeout", "3s",
		"--proxy-listen", fmt.Sprintf("127.0.0.1:%d", proxyPort),
		"--proxy-ca", proxyCA, "--proxy-ca-key", proxyKey,
		// A Level 1 app answers no Get Health: the restart wait always ends
		// by its timeout.
		"--stop-cmd", stop, "--start-cmd", start, "--restart-wait", "15s", "--peer", "--announce",
		"--format", "text", "--report", "json="+report)
	began := time.Now()
	out, err := cmd.CombinedOutput()
	t.Logf("oec-check test: %v after %v", err, time.Since(began).Round(time.Second))
	if err != nil {
		t.Fatalf("oec-check test: %v\n%s", err, out)
	}
	raw, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	if keep := os.Getenv("OEC_GO_D7_REPORT"); keep != "" {
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
	outcome := map[string]string{}
	for _, x := range r.Tests {
		outcome[x.ID] = x.Outcome
		if x.Outcome == "fail" {
			t.Errorf("%s: fail", x.ID)
		}
	}
	// The lifecycle of the Go SDK at Level 1: connection, Birth, Will,
	// Close message on SIGTERM, restart (REQ-LIFE-*, T7-CON-*, T7-MON-*).
	for _, id := range []string{
		"T7-MON-01", "T7-MON-02", "T7-MON-03", "T7-MON-04", "T7-MON-05", "T7-MON-06",
		"T7-CON-01", "T7-CON-02", "T7-CON-03", "T7-CON-04",
		"T7-LIFE-01", "T7-LIFE-02", "T7-LIFE-03", "T7-LIFE-04",
	} {
		if outcome[id] != "pass" {
			t.Errorf("%s: %q, want pass\n%s", id, outcome[id], out)
		}
	}
	if r.Exit != 0 {
		t.Errorf("exit %d", r.Exit)
	}
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
