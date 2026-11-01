//go:build unix

package oec

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// buildExample builds examples/<name> into a temporary directory.
func buildExample(t *testing.T, name string) string {
	t.Helper()
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("no go command: %v", err)
	}
	exe := filepath.Join(t.TempDir(), name)
	cmd := exec.Command(gobin, "build", "-o", exe, "./examples/"+name)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./examples/%s: %v\n%s", name, err, out)
	}
	return exe
}

// examples/hello: Birth and Health, and on SIGTERM the Close message,
// DISCONNECT and exit code 0 (REQ-LIFE-001, REQ-LIFE-005). The signal
// belongs to Go: the SDK installs no handlers (ADR-0027 amendment C-076).
func TestExampleHello(t *testing.T) {
	b := startBroker(t)
	exe := buildExample(t, "hello")
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), "OEC_ROOT="+b.container(t))
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	lines := bufio.NewScanner(stdout)
	if !lines.Scan() || !strings.HasPrefix(lines.Text(), "running ") {
		t.Fatalf("first line %q", lines.Text())
	}

	connect, i := b.expect(t, 0, "CONNECT", func(s seen) bool { return s.Seen == "connect" })
	if connect.ClientID != "app-1" || connect.Will == nil {
		t.Fatalf("CONNECT %+v", connect)
	}
	mam, i := b.publish(t, i, "/Pub/MAM/")
	if !strings.Contains(mam.Payload, `"PublisherId":"Utility/example.com/Hello,20OEC,20Go/HELLO-GO-1/app-1"`) {
		t.Fatalf("MAM %s", mam.Payload)
	}
	health, i := b.publish(t, i, *connect.Will)
	if !strings.Contains(health.Payload, `"Health":"NORMAL_0","HealthScore":100`) {
		t.Fatalf("Health %s", health.Payload)
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	b.expect(t, i, "Close message", func(s seen) bool {
		return s.Seen == "publish" && s.Topic == *connect.Will && strings.Contains(s.Payload, `"HealthScore":0`)
	})
	b.expect(t, i, "DISCONNECT", func(s seen) bool { return s.Seen == "disconnect" })
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	select {
	case err := <-exited:
		if err != nil {
			t.Fatalf("hello: %v", err)
		}
	case <-time.After(wait):
		t.Fatal("hello did not exit")
	}
}
