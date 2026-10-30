package host

import (
	"context"
	"encoding/base64"
	"encoding/pem"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/OI4/oec/conformance"
	"github.com/OI4/oec/conformance/catalog"
	"github.com/OI4/oec/conformance/internal/bus/registrytest"
	"github.com/OI4/oec/conformance/internal/host/hosttest"
	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/mqttwire/mqtttest"
	"github.com/OI4/oec/conformance/internal/verdict"
)

const (
	registryIL  = "oi4.org/Registry/SIM/1"
	userApp     = "app"
	appPassword = "app-secret"
	dockerSock  = "unix:///var/run/docker.sock"
)

func setup(t *testing.T) ([]verdict.Test, verdict.Requirements) {
	t.Helper()
	cat, err := catalog.Load(catalog.D8File)
	if err != nil {
		t.Fatal(err)
	}
	reqs, err := verdict.LoadRequirements()
	if err != nil {
		t.Fatal(err)
	}
	return Tests(cat), reqs
}

func TestCatalogStatus(t *testing.T) {
	tests, _ := setup(t)
	for _, tc := range tests {
		if (tc.Status == "implemented") != Implemented[tc.ID] {
			t.Errorf("%s: catalog status %s, implemented here %v", tc.ID, tc.Status, Implemented[tc.ID])
		}
	}
	if len(tests) != 4 {
		t.Errorf("%d host tests in the catalog", len(tests))
	}
}

// conformingFacts are the facts of the conforming platform host.
func conformingFacts() *Facts {
	return &Facts{OS: "linux", Kernel: "6.1.0-test", Distribution: "Debian GNU/Linux 12 (bookworm)", Machine: "x86_64",
		Memory: 2_000_000_000, Storage: 16_000_000_000, StorageSource: "block devices mmcblk0",
		Interfaces:     []Interface{{Name: "lo", Up: true, Loopback: true}, {Name: "eth0", Up: true, Physical: true}},
		InterfacesRead: true, Daemons: []string{dockerSock}}
}

// closedPort returns a TCP port of 127.0.0.1 without a listener.
func closedPort(t *testing.T) int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func port(t *testing.T, addr string) int {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := strconv.Atoi(p)
	return n
}

func pemCert(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func b64(s string) []byte { return []byte(base64.StdEncoding.EncodeToString([]byte(s))) }

// scenario builds the root, the host and – for T8-HST-04 – the broker with
// the Registry of a host fixture and returns the configuration of the run.
func scenario(t *testing.T, f conformance.Fixture) Config {
	t.Helper()
	p := mqtttest.NewPKI(t)
	test, variant, _ := strings.Cut(f.Fault, "/")
	certificate := test == TSecrets && slices.Contains([]string{"certificate", "unencrypted-key", "key-without-cert", "cert-misnamed", "key-mismatch", "no-passphrase", "wrong-passphrase", "passphrase-unencrypted"}, variant)
	o := mqtttest.Options{Users: map[string]string{userApp: appPassword, "registry": "registry-secret"}}
	ho := hosttest.Options{}
	if certificate {
		o.RequireCert = true
		ho.Certificate = true
	} else {
		ho.Credentials = userApp + ":" + appPassword
	}
	if f.Fault == TMQTT+"/broker-ca" {
		cert := p.ServerCert(t, mqtttest.CertOtherCA)
		o.Cert, ho.Broker, ho.BrokerCA = &cert, &cert, p.OtherCAFile
	}
	registry := true
	if variant == "no-registry" {
		registry = false
	}
	broker := mqtttest.NewBrokerWith(t, p, o)
	ho.Port = port(t, broker.Addr)
	if f.Runs(TPlatform) && registry {
		rc := mqttwire.Config{Broker: broker.URL(), CAFile: p.CAFile, Username: "registry", Password: []byte("registry-secret")}
		if o.RequireCert {
			rc = mqttwire.Config{Broker: broker.URL(), CAFile: p.CAFile, CertFile: p.ClientCert, KeyFile: p.ClientKey}
		}
		if ho.BrokerCA != "" {
			rc.CAFile = ho.BrokerCA
		}
		registrytest.Start(t, rc, registryIL, registrytest.None, 100*time.Millisecond)
	}
	if variant == "no-broker" {
		ho.Port = closedPort(t)
	}
	if variant == "wrong-credentials" {
		ho.Credentials = userApp + ":wrong"
	}
	r := hosttest.New(t, p, ho)
	facts := conformingFacts()
	mounts := []Mount{{Point: "/"}, {Point: r.MQTT, ReadOnly: true}, {Point: r.Certs, ReadOnly: true}, {Point: r.Secrets, ReadOnly: true}}
	daemons := map[string]error{dockerSock: nil}
	env := map[string]string{}
	c := Config{Root: r.Dir, RegistryWait: 2 * time.Second, Timeout: time.Second, Tests: f.Tests}
	file := func(dir, name string) string { return filepath.Join(dir, name) }
	switch test {
	case "none":
	case TMQTT:
		switch variant {
		case "broker-ca":
		case "no-mqtt-storage":
			hosttest.Remove(t, r.MQTT)
		case "no-broker-pem":
			hosttest.Remove(t, file(r.MQTT, "broker.pem"))
		case "broker-pem-garbage":
			hosttest.Write(t, file(r.MQTT, "broker.pem"), []byte("not a certificate\n"))
		case "no-broker-json":
			hosttest.Remove(t, file(r.MQTT, "broker.json"))
		case "not-json":
			hosttest.Write(t, file(r.MQTT, "broker.json"), []byte("Address=127.0.0.1\n"))
		case "broker-json-invalid":
			hosttest.Write(t, file(r.MQTT, "broker.json"), []byte(`{"Address": "127.0.0.1", "SecurePort": 8883, "MaxPacketSize": 1024}`))
		case "address-with-port":
			hosttest.Write(t, file(r.MQTT, "broker.json"), []byte(`{"Address": "127.0.0.1:8883", "SecurePort": 8883, "MaxPacketSize": 262144}`))
		case "untrusted-broker":
			hosttest.Write(t, file(r.MQTT, "broker.pem"), pemCert(p.ServerCert(t, mqtttest.CertOtherCA).Certificate[0]))
		case "wrong-broker-ca":
			hosttest.Write(t, file(r.MQTT, "broker_ca.pem"), hosttest.Read(t, p.OtherCAFile))
		case "broker-ca-same":
			hosttest.Write(t, file(r.MQTT, "broker_ca.pem"), hosttest.Read(t, p.CAFile))
		case "mqtt-rw":
			mounts[1].ReadOnly = false
		case "mqtt-not-mounted":
			mounts = slices.Delete(mounts, 1, 2)
			facts.InContainer = true
		case "world-writable":
			if err := os.Chmod(file(r.MQTT, "broker.json"), 0o666); err != nil {
				t.Fatal(err)
			}
		case "broker-expired":
			hosttest.Write(t, file(r.MQTT, "broker.pem"), pemCert(p.ServerCert(t, mqtttest.CertExpired).Certificate[0]))
		default:
			t.Fatalf("unknown fixture %s", f.Fault)
		}
	case TSecrets:
		switch variant {
		case "certificate":
		case "unencrypted-key":
			hosttest.Secret(t, file(r.Secrets, "private_key.pem"), hosttest.Read(t, p.ClientKey))
			hosttest.Remove(t, file(r.Secrets, "passphrase"))
		case "secret-files-mounted":
			mounts = append(mounts[:3], Mount{Point: file(r.Secrets, "credentials"), ReadOnly: true})
		case "no-certs-storage":
			hosttest.Remove(t, r.Certs)
		case "no-ca":
			hosttest.Remove(t, file(r.Certs, "ca.pem"))
		case "ca-not-ca":
			hosttest.Write(t, file(r.Certs, "ca.pem"), hosttest.Read(t, p.ClientCert))
		case "key-in-mqtt":
			hosttest.Write(t, file(r.MQTT, "extra.pem"), hosttest.Read(t, p.ClientKey))
		case "key-in-certs":
			hosttest.Write(t, file(r.Certs, hosttest.Container+".key"), []byte("key material\n"))
		case "no-secrets":
			hosttest.Remove(t, r.Secrets)
		case "empty-secrets":
			hosttest.Remove(t, file(r.Secrets, "credentials"))
		case "extra-secret":
			hosttest.Secret(t, file(r.Secrets, "token"), []byte("secret\n"))
		case "bad-credentials":
			hosttest.Secret(t, file(r.Secrets, "credentials"), b64(userApp))
		case "username-reserved":
			hosttest.Secret(t, file(r.Secrets, "credentials"), b64("app/1:"+appPassword))
		case "key-without-cert":
			hosttest.Remove(t, file(r.Certs, hosttest.Container+".pem"))
		case "cert-misnamed":
			if err := os.Rename(file(r.Certs, hosttest.Container+".pem"), file(r.Certs, "app.pem")); err != nil {
				t.Fatal(err)
			}
		case "key-mismatch":
			hosttest.Write(t, file(r.Certs, hosttest.Container+".pem"), pemCert(p.BrokerCert.Certificate[0]))
		case "no-passphrase":
			hosttest.Remove(t, file(r.Secrets, "passphrase"))
		case "wrong-passphrase":
			hosttest.Secret(t, file(r.Secrets, "passphrase"), b64("wrong"))
		case "passphrase-unencrypted":
			hosttest.Secret(t, file(r.Secrets, "private_key.pem"), hosttest.Read(t, p.ClientKey))
		case "secrets-rw":
			mounts[3].ReadOnly = false
		case "certs-rw":
			mounts[2].ReadOnly = false
		default:
			t.Fatalf("unknown fixture %s", f.Fault)
		}
	case TPlatform:
		switch variant {
		case "armv7":
			facts.Machine = "armv7l"
		case "daemon-env":
			env["DOCKER_HOST"] = "tcp://127.0.0.1:2375"
			daemons = map[string]error{"tcp://127.0.0.1:2375": nil}
		case "arch-386":
			facts.Machine = "i686"
		case "ram-low":
			facts.Memory = 512 << 20
		case "ram-reserved":
			facts.Memory = 950_000_000
		case "storage-low":
			facts.Storage = 2_000_000_000
		case "storage-nominal":
			facts.Storage = 3_900_000_000
		case "no-network":
			facts.Interfaces = facts.Interfaces[:1]
		case "virtual-network":
			facts.Interfaces[1].Physical = false
		case "not-linux":
			facts.OS = "darwin"
		case "no-daemon":
			daemons = map[string]error{}
		case "daemon-denied":
			daemons[dockerSock] = &net.OpError{Op: "dial", Net: "unix", Err: syscall.EACCES}
		case "daemon-stale":
			daemons[dockerSock] = &net.OpError{Op: "dial", Net: "unix", Err: syscall.ECONNREFUSED}
		case "no-broker", "wrong-credentials":
		case "no-registry":
			c.RegistryWait = 500 * time.Millisecond
		case "unknown-facts":
			facts = &Facts{Daemons: []string{dockerSock}}
		case "offline":
			c.Offline = true
		default:
			t.Fatalf("unknown fixture %s", f.Fault)
		}
	default:
		t.Fatalf("unknown fixture %s", f.Fault)
	}
	c.Facts = facts
	c.Mounts = func() ([]Mount, error) { return mounts, nil }
	c.DialDaemon = func(_ context.Context, ep string) error {
		err, ok := daemons[ep]
		if !ok {
			return &net.OpError{Op: "dial", Net: "unix", Err: fs.ErrNotExist}
		}
		return err
	}
	c.Getenv = func(k string) string { return env[k] }
	return c
}

// TestFixtures runs every host fixture of testdata/hosts/index.yaml and
// checks the verdict of its selected tests against the expectation.
func TestFixtures(t *testing.T) {
	tests, reqs := setup(t)
	fixtures, err := conformance.HostFixtures()
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no host fixtures")
	}
	for _, f := range fixtures {
		t.Run(f.Fault, func(t *testing.T) {
			t.Parallel()
			if runtime.GOOS == "windows" && f.Fault == TMQTT+"/world-writable" {
				t.Skip("file modes are not reported on Windows")
			}
			if f.Status != "draft" && f.Status != "reviewed" {
				t.Errorf("status %q", f.Status)
			}
			for id := range f.Expect {
				if !Implemented[id] || !f.Runs(id) {
					t.Errorf("expectation for %s, which is no implemented host test of the fixture", id)
				}
			}
			c := scenario(t, f)
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			res := Run(ctx, c)
			for _, tr := range verdict.Build(tests, res.Observations, reqs, 3) {
				if !f.Runs(tr.ID) || !Implemented[tr.ID] {
					continue
				}
				want, ok := f.Expect[tr.ID]
				if !ok {
					want.Outcome = string(verdict.Pass)
				}
				if string(tr.Outcome) != want.Outcome {
					t.Errorf("%s: outcome %s, want %s (%+v)", tr.ID, tr.Outcome, want.Outcome, tr.Reqs)
					continue
				}
				var with, manual []string
				for _, r := range tr.Reqs {
					switch {
					case r.Outcome == verdict.Manual:
						manual = append(manual, r.Req)
					case string(r.Outcome) == want.Outcome:
						with = append(with, r.Req)
					case r.Outcome != verdict.Skip && !verdict.Worse(tr.Outcome, r.Outcome):
						t.Errorf("%s: %s has %s, not better than the test outcome %s", tr.ID, r.Req, r.Outcome, tr.Outcome)
					}
				}
				if want.Outcome != string(verdict.Pass) && !equalSet(with, want.Reqs) {
					t.Errorf("%s: requirements with %s %v, want %v (%+v)", tr.ID, want.Outcome, with, want.Reqs, tr.Reqs)
				}
				if !equalSet(manual, want.Manual) {
					t.Errorf("%s: requirements with a manual verdict %v, want %v (%+v)", tr.ID, manual, want.Manual, tr.Reqs)
				}
				if f.Positive && tr.Outcome != verdict.Pass {
					t.Errorf("%s: a positive fixture must pass (%+v)", tr.ID, tr.Reqs)
				}
			}
			if len(res.Host.Storages) != 3 || res.Host.Root != c.Root {
				t.Errorf("host %+v", res.Host)
			}
		})
	}
}

func equalSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	sort.Strings(a)
	sort.Strings(b)
	return slices.Equal(a, b)
}

// TestFixtureCoverage: every host test has a positive fixture (it passes)
// and a negative fixture (an expectation worse than pass).
func TestFixtureCoverage(t *testing.T) {
	fixtures, err := conformance.HostFixtures()
	if err != nil {
		t.Fatal(err)
	}
	for id := range Implemented {
		pos, neg := false, false
		for _, f := range fixtures {
			e, ok := f.Expect[id]
			pos = pos || f.Positive && f.Runs(id) && (!ok || e.Outcome == string(verdict.Pass))
			neg = neg || ok && e.Outcome != string(verdict.Pass)
		}
		if !pos || !neg {
			t.Errorf("%s: positive fixture %v, negative fixture %v", id, pos, neg)
		}
	}
}
