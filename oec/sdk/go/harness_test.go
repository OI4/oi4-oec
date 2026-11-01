package oec

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The Go tests run against the in-process TLS test broker of the Rust SDK
// as a program (oec-ffi/tests/go/testbroker.rs, ADR-0027 §9, C-076):
// OEC_GO_TESTBROKER is its path (default: the debug build in the Cargo
// target directory). Without it the broker tests are skipped, unless
// OEC_GO_REQUIRE_BROKER=1 (set by oec/sdk/check.sh).

// wait is the upper bound for every wait of a test.
const wait = 15 * time.Second

// seen is one line of the test broker: what it saw on a connection.
type seen struct {
	Seen       string   `json:"seen"`
	Conn       int      `json:"conn"`
	ClientCert bool     `json:"client_cert"`
	ClientID   string   `json:"client_id"`
	Username   *string  `json:"username"`
	Will       *string  `json:"will"`
	Topic      string   `json:"topic"`
	Payload    string   `json:"payload"`
	Retain     bool     `json:"retain"`
	Filters    []string `json:"filters"`
}

func (s seen) String() string {
	switch s.Seen {
	case "publish":
		return fmt.Sprintf("%d publish %s %s", s.Conn, s.Topic, s.Payload)
	case "subscribe", "unsubscribe":
		return fmt.Sprintf("%d %s %v", s.Conn, s.Seen, s.Filters)
	default:
		return fmt.Sprintf("%d %s", s.Conn, s.Seen)
	}
}

// broker is a running test broker.
type broker struct {
	Port       int    `json:"port"`
	CA         string `json:"ca"`
	ClientCert string `json:"client_cert"`
	ClientKey  string `json:"client_key"`

	stdin io.WriteCloser

	mu     sync.Mutex
	cond   *sync.Cond
	events []seen
	ended  bool
}

func testBrokerPath() string {
	if p := os.Getenv("OEC_GO_TESTBROKER"); p != "" {
		return p
	}
	return filepath.Join("..", "rust", "target", "debug", "examples", "go-testbroker")
}

// startBroker starts a test broker for t; it ends with t.
func startBroker(t *testing.T) *broker {
	t.Helper()
	path := testBrokerPath()
	if _, err := os.Stat(path); err != nil {
		if os.Getenv("OEC_GO_REQUIRE_BROKER") == "1" {
			t.Fatalf("test broker %s: %v", path, err)
		}
		t.Skipf("no test broker (%v): cargo build -p oec-ffi --example go-testbroker", err)
	}
	cmd := exec.Command(path, t.TempDir())
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	lines := bufio.NewScanner(stdout)
	lines.Buffer(make([]byte, 1<<20), 1<<24)
	b := &broker{stdin: stdin}
	b.cond = sync.NewCond(&b.mu)
	if !lines.Scan() {
		t.Fatalf("test broker: no first line: %v", lines.Err())
	}
	if err := json.Unmarshal(lines.Bytes(), b); err != nil {
		t.Fatalf("test broker: %v: %s", err, lines.Text())
	}
	read := make(chan struct{})
	go func() {
		defer close(read)
		for lines.Scan() {
			var s seen
			if err := json.Unmarshal(lines.Bytes(), &s); err != nil {
				panic(fmt.Sprintf("test broker: %v: %s", err, lines.Text()))
			}
			b.mu.Lock()
			b.events = append(b.events, s)
			b.cond.Broadcast()
			b.mu.Unlock()
		}
		b.mu.Lock()
		b.ended = true
		b.cond.Broadcast()
		b.mu.Unlock()
	}()
	t.Cleanup(func() {
		stdin.Close()
		<-read // Wait closes stdout
		_ = cmd.Wait()
	})
	return b
}

// expect returns the first event from index from on that matches, with its
// index; it fails t after wait.
func (b *broker) expect(t *testing.T, from int, what string, match func(seen) bool) (seen, int) {
	t.Helper()
	deadline := time.Now().Add(wait)
	timer := time.AfterFunc(wait, func() {
		b.mu.Lock()
		b.cond.Broadcast()
		b.mu.Unlock()
	})
	defer timer.Stop()
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := from; ; {
		for ; i < len(b.events); i++ {
			if match(b.events[i]) {
				return b.events[i], i
			}
		}
		if b.ended || time.Now().After(deadline) {
			var all []string
			for _, e := range b.events {
				all = append(all, e.String())
			}
			t.Fatalf("test broker: no %s within %v; seen:\n%s", what, wait, strings.Join(all, "\n"))
		}
		b.cond.Wait()
	}
}

// publish waits for a PUBLISH whose topic contains part.
func (b *broker) publish(t *testing.T, from int, part string) (seen, int) {
	t.Helper()
	return b.expect(t, from, "PUBLISH on *"+part+"*", func(s seen) bool {
		return s.Seen == "publish" && strings.Contains(s.Topic, part)
	})
}

// send sends a PUBLISH to the client of connection conn.
func (b *broker) send(t *testing.T, conn int, topic, payload string) {
	t.Helper()
	line, _ := json.Marshal(map[string]any{"cmd": "send", "conn": conn, "topic": topic, "payload": payload})
	if _, err := b.stdin.Write(append(line, '\n')); err != nil {
		t.Fatal(err)
	}
}

// container writes a container storage (REQ-CNT-004) for the broker:
// hostname app-1, broker.json, the CA and the credentials user/pa:ss.
func (b *broker) container(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	ca, err := os.ReadFile(b.CA)
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"etc/hostname":                "app-1\n",
		"etc/oi4/mqtt/broker.json":    fmt.Sprintf(`{"Address": "localhost", "SecurePort": %d, "MaxPacketSize": 262144}`, b.Port),
		"etc/oi4/certs/ca.pem":        string(ca),
		"run/secrets/oi4/credentials": "dXNlcjpwYTpzcw==\n", // base64("user:pa:ss")
	} {
		write(t, filepath.Join(root, name), content)
	}
	if err := os.MkdirAll(filepath.Join(root, "etc", "oi4", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// eventually polls cond until it holds; it fails t after wait.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(wait)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("not within %v: %s", wait, what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
