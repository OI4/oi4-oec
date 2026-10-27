package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OI4/oec/conformance/internal/capture"
	"github.com/OI4/oec/conformance/internal/mqttwire"
)

// signalWriter collects output and closes ready once it contains mark.
type signalWriter struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	mark  string
	ready chan struct{}
	done  bool
}

func (w *signalWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	if !w.done && strings.Contains(w.buf.String(), w.mark) {
		w.done = true
		close(w.ready)
	}
	return len(p), nil
}

func (w *signalWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// TestMonitorDevBroker is the opt-in live run against the Mosquitto of
// oec/dev (oec/conformance/live.sh sets OEC_LIVE=1 and OEC_DEV_DIR). Two
// monitors attach with the sample container storages – oec-dev-cert (client
// certificate, encrypted PKCS#8 key, port 8884) and oec-dev-app (credentials,
// port 8883). A simulated component logs in with the probe credentials,
// registers the Health Will of the fixture reconnect-pass, publishes its
// Birth, drops the connection (the broker publishes the Will) and reconnects
// with a new Birth.
func TestMonitorDevBroker(t *testing.T) {
	if os.Getenv("OEC_LIVE") != "1" {
		t.Skip("opt-in: OEC_LIVE=1 (oec/conformance/live.sh)")
	}
	dev := os.Getenv("OEC_DEV_DIR")
	if dev == "" {
		t.Fatal("OEC_DEV_DIR not set")
	}
	gen := filepath.Join(dev, "generated")
	read := func(rel string) []byte {
		b, err := os.ReadFile(filepath.Join(gen, rel))
		if err != nil {
			t.Fatal(err)
		}
		return bytes.TrimSpace(b)
	}
	data, err := os.ReadFile(captures + "reconnect-pass.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := capture.Read(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixture.Records) != 5 {
		t.Fatalf("reconnect-pass: %d records, want 5", len(fixture.Records))
	}
	out := t.TempDir()
	if d := os.Getenv("OEC_LIVE_OUT"); d != "" {
		out = d
	}

	type liveRun struct {
		root, rec string
		code      int
		stdout    bytes.Buffer
		stderr    *signalWriter
	}
	runs := []*liveRun{{root: "oec-dev-cert"}, {root: "oec-dev-app"}}
	var wg sync.WaitGroup
	for _, r := range runs {
		r.rec = filepath.Join(out, "live-"+r.root+".jsonl")
		r.stderr = &signalWriter{mark: "monitoring", ready: make(chan struct{})}
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.code = run([]string{"monitor", "--bus-storage", filepath.Join(gen, "roots", r.root), "--duration", "6s",
				"--record", r.rec, "--format", "json"}, strings.NewReader(""), &r.stdout, r.stderr)
		}()
	}
	for _, r := range runs {
		select {
		case <-r.stderr.ready:
		case <-time.After(20 * time.Second):
			t.Fatalf("%s: no subscription\n%s", r.root, r.stderr)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	probe := mqttwire.Config{Broker: "mqtts://localhost:8883", CAFile: filepath.Join(gen, "broker", "ca.pem"),
		Username: string(read("probe/username")), Password: read("probe/password")}
	rs := fixture.Records
	will := mqttwire.Message{Topic: rs[2].Topic, Payload: rs[2].Payload, QoS: rs[2].QoS}
	first := probe
	first.Will = &will
	pub, err := mqttwire.Dial(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rs[:2] {
		if err := pub.Publish(ctx, mqttwire.Message{Topic: r.Topic, Payload: r.Payload, QoS: r.QoS}); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(300 * time.Millisecond)
	pub.Kill()
	time.Sleep(500 * time.Millisecond)
	pub, err = mqttwire.Dial(ctx, probe)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rs[3:] {
		if err := pub.Publish(ctx, mqttwire.Message{Topic: r.Topic, Payload: r.Payload, QoS: r.QoS}); err != nil {
			t.Fatal(err)
		}
	}
	pub.Close()
	wg.Wait()

	for _, r := range runs {
		var rep struct {
			Participants []struct {
				ID                    string
				Births, Wills, Closes int
			}
			Tests []struct{ ID, Outcome string }
			Exit  int
		}
		if err := json.Unmarshal(r.stdout.Bytes(), &rep); err != nil {
			t.Fatalf("%s: exit %d: %v\n%s", r.root, r.code, err, r.stderr)
		}
		outcome := map[string]string{}
		for _, x := range rep.Tests {
			outcome[x.ID] = x.Outcome
		}
		// 6 s: the heartbeat (MUST) is inconclusive, nothing fails
		if r.code != exitInconclusive || len(rep.Participants) != 1 || rep.Participants[0].Births != 2 || rep.Participants[0].Wills != 1 ||
			outcome["T7-MON-03"] != "pass" || outcome["T7-MON-01"] != "pass" || outcome["T7-MON-12"] != "pass" || outcome["T7-MON-05"] != "skip" || outcome["T7-MON-04"] != "pass" {
			t.Errorf("%s: exit %d\n%s\n%s", r.root, r.code, r.stdout.String(), r.stderr)
			continue
		}
		got, err := os.ReadFile(r.rec)
		if err != nil {
			t.Fatal(err)
		}
		c, err := capture.Read(bytes.NewReader(got))
		if err != nil {
			t.Fatal(err)
		}
		if len(c.Records) != 5 || c.Header.MaxPacketSize != 262144 || c.Trailer == nil || c.Trailer.Error != "" {
			t.Errorf("%s: recording: %d records, header %+v, trailer %+v", r.root, len(c.Records), c.Header, c.Trailer)
		}
		for i, x := range c.Records {
			if x.Topic != rs[i].Topic || !bytes.Equal(x.Payload, rs[i].Payload) || x.QoS != rs[i].QoS || x.Retain {
				t.Errorf("%s: record %d differs: %s qos %d retain %v", r.root, i+1, x.Topic, x.QoS, x.Retain)
			}
		}
		t.Logf("%s: exit %d, %d records, participant %s births %d wills %d", r.root, r.code, len(c.Records),
			rep.Participants[0].ID, rep.Participants[0].Births, rep.Participants[0].Wills)
	}
}
