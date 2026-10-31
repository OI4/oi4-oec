package host

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/OI4/oec/conformance/internal/host/hosttest"
	"github.com/OI4/oec/conformance/internal/mqttwire/mqtttest"
	"github.com/OI4/oec/conformance/internal/testvec"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// TestBrokerVectors: the judgement of broker.json (REQ-CNT-003) agrees with
// every broker-json vector of oec/spec (V-BRK-…).
func TestBrokerVectors(t *testing.T) {
	vs, err := testvec.Load("platform")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, v := range vs {
		content, ok := v.Extra["content"].(string)
		if !strings.HasPrefix(v.ID, "V-BRK-") || !ok {
			continue
		}
		n++
		r := &runner{}
		if err := r.judgeBrokerJSON(TMQTT, "broker.json", []byte(content)); err != nil {
			t.Fatal(err)
		}
		var bad []string
		for _, o := range r.obs {
			if o.Kind == verdict.Violation && !slices.Contains(bad, o.Req) {
				bad = append(bad, o.Req)
			}
		}
		switch v.Expected {
		case "valid":
			if len(bad) > 0 {
				t.Errorf("%s: valid, judged %v (%+v)", v.ID, bad, r.obs)
			}
		default:
			for _, req := range v.Violates {
				if !slices.Contains(bad, req) {
					t.Errorf("%s: violates %v, judged %v (%+v)", v.ID, v.Violates, bad, r.obs)
				}
			}
		}
	}
	if n != 18 {
		t.Errorf("%d broker-json vectors", n)
	}
}

func TestParseMountinfo(t *testing.T) {
	const info = `22 1 8:2 / / rw,relatime shared:1 - ext4 /dev/sda2 rw
1503 1501 0:120 / /etc/oi4/mqtt ro,relatime - ext4 /dev/sda2 rw
1504 1501 0:120 /secrets/credentials /run/secrets/oi4/credentials rw,relatime - ext4 /dev/sda2 ro,errors=remount-ro
1505 1501 0:121 / /mnt/with\040space rw - tmpfs tmpfs rw
`
	ms, err := ParseMountinfo(strings.NewReader(info))
	if err != nil {
		t.Fatal(err)
	}
	want := []Mount{{"/", false}, {"/etc/oi4/mqtt", true}, {"/run/secrets/oi4/credentials", true}, {"/mnt/with space", false}}
	if !slices.Equal(ms, want) {
		t.Errorf("mounts %+v, want %+v", ms, want)
	}
	if _, err := ParseMountinfo(strings.NewReader("1 2 3\n")); err == nil {
		t.Error("short line accepted")
	}
	if _, err := ParseMountinfo(strings.NewReader("1 2 3 4 5 6 7 8 9 10\n")); err == nil {
		t.Error("line without separator accepted")
	}
}

func TestArchitecture(t *testing.T) {
	for m, want := range map[string]string{"x86_64": "amd64", "AMD64": "amd64", "aarch64": "ARMv8", "arm64": "ARMv8", "armv7l": "ARMv7", "i686": "", "riscv64": "", "armv6l": ""} {
		if got := Architecture(m); got != want {
			t.Errorf("%s: %q, want %q", m, got, want)
		}
	}
}

// TestLocal: the facts of the system the test runs on are measured where
// D8 supports it (Linux, macOS, Windows).
func TestLocal(t *testing.T) {
	f := Local()
	if f.OS != runtime.GOOS || !f.InterfacesRead {
		t.Errorf("facts %+v", f)
	}
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
		if f.Machine == "" || f.Memory == 0 || f.Storage == 0 || f.StorageSource == "" || len(f.Daemons) == 0 {
			t.Errorf("facts %+v", f)
		}
	}
	if runtime.GOOS != "windows" && f.Kernel == "" {
		t.Errorf("no kernel release: %+v", f)
	}
	t.Logf("%+v", f)
}

// TestUnreadable: a secret D8 may not read makes its requirement
// inconclusive, not failed (D8 runs as a user that may not read the
// storages).
func TestUnreadable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs file modes and a user other than root")
	}
	p := mqtttest.NewPKI(t)
	r := hosttest.New(t, p, hosttest.Options{Port: 8883, Credentials: "app:secret"})
	if err := os.Chmod(filepath.Join(r.Secrets, "credentials"), 0); err != nil {
		t.Fatal(err)
	}
	res := Run(context.Background(), Config{Root: r.Dir, Tests: []string{TSecrets}, Facts: conformingFacts(),
		Mounts: func() ([]Mount, error) { return nil, ErrNoMountTable }})
	incon := false
	for _, o := range res.Observations {
		if o.Req == reqSecrets && o.Kind == verdict.Violation {
			t.Errorf("violation %s", o.Detail)
		}
		incon = incon || o.Req == reqSecrets && o.Inconclusive && strings.Contains(o.Detail, "permission denied")
	}
	if !incon {
		t.Errorf("no inconclusive REQ-CNT-008: %+v", res.Observations)
	}
	for _, s := range res.Host.Storages {
		if s.Mount != "unknown" {
			t.Errorf("storage %s: mount %s without a mount table", s.Name, s.Mount)
		}
	}
}
