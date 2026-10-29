package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBusDevBroker is the opt-in live run of `oec-infra bus` against the
// Mosquitto of oec/dev (oec/conformance/live.sh sets OEC_LIVE=1 and
// OEC_DEV_DIR). Port 8883: the probe logs in with the probe credentials, the
// unprivileged client with the credentials of the sample storage
// oec-dev-app. Port 8884 (client certificate required): the probe presents
// its client certificate, the unprivileged client is the sample storage
// oec-dev-cert. The dev ACL cannot bind the AppId level to a user and the
// applications read their own Set/Config, so REQ-RES-CONFIG-026 warns; there
// is no OEC Registry before P7, so T8-BUS-03 skips (C-071).
func TestBusDevBroker(t *testing.T) {
	if os.Getenv("OEC_LIVE") != "1" {
		t.Skip("opt-in: OEC_LIVE=1 (oec/conformance/live.sh)")
	}
	dev := os.Getenv("OEC_DEV_DIR")
	if dev == "" {
		t.Fatal("OEC_DEV_DIR not set")
	}
	gen := filepath.Join(dev, "generated")
	probe := filepath.Join(gen, "probe")
	runs := []struct {
		name string
		args []string
		exit int
		want map[string]string
	}{
		{"8883", []string{"--broker", "mqtts://localhost:8883", "--bus-storage", filepath.Join(gen, "roots", "oec-dev-app"),
			"--username", "oec-probe", "--password-file", filepath.Join(probe, "password"),
			"--credentials-2", filepath.Join(gen, "roots", "oec-dev-app", "run", "secrets", "oi4", "credentials"), "--registry-wait", "5s"},
			exitInconclusive, map[string]string{"T8-BUS-01": "pass", "T8-BUS-02": "warn", "T8-BUS-03": "skip"}},
		{"8884", []string{"--broker", "mqtts://localhost:8884", "--ca", filepath.Join(gen, "broker", "ca.pem"),
			"--cert", filepath.Join(probe, "probe.pem"), "--key", filepath.Join(probe, "probe.key"),
			"--bus-storage-2", filepath.Join(gen, "roots", "oec-dev-cert"), "--max-packet-size", "262144", "--tests", "T8-BUS-0[12]"},
			exitPass, map[string]string{"T8-BUS-01": "pass", "T8-BUS-02": "warn"}},
	}
	for _, r := range runs {
		t.Run(r.name, func(t *testing.T) {
			code, out, errOut := call(t, append(append([]string{"bus"}, r.args...), "--format", "json")...)
			t.Logf("%s%s", out, errOut)
			if code != r.exit {
				t.Errorf("exit %d, want %d", code, r.exit)
			}
			var rep busReport
			if err := json.Unmarshal([]byte(out), &rep); err != nil {
				t.Fatal(err)
			}
			if strings.Join(rep.Broker.TLS, ",") != "1.2,1.3" || rep.Broker.MaxPacketSize != 262144 {
				t.Errorf("broker %+v", rep.Broker)
			}
			got := map[string]string{}
			for _, tc := range rep.Tests {
				got[tc.ID] = tc.Outcome
			}
			for id, want := range r.want {
				if got[id] != want {
					t.Errorf("%s: %s, want %s", id, got[id], want)
				}
			}
			if len(got) != len(r.want) {
				t.Errorf("tests %v, want %v", got, r.want)
			}
		})
	}
}
