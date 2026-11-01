package oec

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt-in interoperability test against the Mosquitto of oec/dev (C-078):
// OEC_GO_DEV is the directory oec/dev with its generated PKI and the broker
// running (docker compose -f oec/dev/compose.yaml up -d --wait). Without it
// the test is skipped; the gate never sets it.
//
// The provider runs on a copy of the sample storage oec-dev-cert (client
// certificate), the consumer on a copy of oec-dev-app (user name and
// password) with the credentials of the probe user: the ACL of oec/dev
// lets only oec-probe send Set (MG 9.1, REQ-GEN-001).
func TestInteropDev(t *testing.T) {
	dev := os.Getenv("OEC_GO_DEV")
	if dev == "" {
		t.Skip("OEC_GO_DEV not set: interop test against oec/dev skipped")
	}
	gen := filepath.Join(dev, "generated")
	provRoot := copyRoot(t, filepath.Join(gen, "roots", "oec-dev-cert"))
	consRoot := copyRoot(t, filepath.Join(gen, "roots", "oec-dev-app"))
	write(t, filepath.Join(consRoot, "etc", "hostname"), "oec-go-consumer\n")
	user := readTrim(t, filepath.Join(gen, "probe", "username"))
	pass := readTrim(t, filepath.Join(gen, "probe", "password"))
	write(t, filepath.Join(consRoot, "run", "secrets", "oi4", "credentials"),
		base64.StdEncoding.EncodeToString([]byte(user+":"+pass))+"\n")

	provider, err := New(Options{Level: Level3, ServiceType: OTConnector, Product: testProduct,
		ContainerRoot: provRoot, StoreDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = provider.Close() })
	pump, err := provider.AddAsset(AssetOptions{
		Product:      Product{ManufacturerURI: "vendor.com", Model: "Pump", ProductCode: "P-3"},
		SerialNumber: "0815",
	})
	if err != nil {
		t.Fatal(err)
	}
	speed, err := pump.AddData(DataDef{Tag: "speed", Fields: []Field{
		{Name: "Setpoint", Type: TypeDouble, Unit: "rpm", Range: &Range{0, 3000}},
	}, Mode: ModeOnRequest | ModeSource})
	if err != nil {
		t.Fatal(err)
	}
	if err := speed.Update(map[string]any{"Setpoint": 1000.0}); err != nil {
		t.Fatal(err)
	}
	if err := speed.OnSet(func(json.RawMessage) error { return nil }); err != nil {
		t.Fatal(err)
	}
	consumer, err := New(Options{Level: Level2, ServiceType: Aggregation, Product: consumerProduct,
		ContainerRoot: consRoot, StoreDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = consumer.Close() })
	for _, a := range []*App{provider, consumer} {
		if err := a.Start(); err != nil {
			t.Fatal(err)
		}
	}
	app := strings.TrimSuffix(providerIL, "app-1") + "oec-dev-cert" // serial number = hostname

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	eventually(t, "the provider answers on oec/dev", func() bool {
		c, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		got, err := consumer.Get(c, ResourceMAM, Address{ServiceType: OTConnector, App: app})
		return err == nil && len(got) == 2
	})

	got := make(chan Received, 16)
	sub, err := consumer.Subscribe(ResourceData, Address{ServiceType: OTConnector, App: app, Source: pumpIL},
		func(r Received) { got <- r })
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	at := Address{ServiceType: OTConnector, App: app, Source: pumpIL, Filter: "speed"}
	if err := consumer.Set(ctx, ResourceData, at, map[string]any{"Setpoint": 2500.0}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	data, err := consumer.Get(ctx, ResourceData, at)
	if err != nil || len(data) != 1 || !strings.Contains(string(data[0].Payload), `"Setpoint":2500.0`) {
		t.Fatalf("Get after Set: %v %v", data, err)
	}
	if err := speed.Update(map[string]any{"Setpoint": 1500.0}); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case r := <-got:
			if r.App == app && r.Source == pumpIL && strings.Contains(string(r.Payload), `"Setpoint":1500.0`) {
				return
			}
		case <-ctx.Done():
			t.Fatal("no Pub/Data of the update on oec/dev")
		}
	}
}

// copyRoot copies a sample container storage into a temporary directory
// (modes kept: the private key and the secrets stay 0600).
func copyRoot(t *testing.T, from string) string {
	t.Helper()
	to := t.TempDir()
	err := filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), info.Mode().Perm()|0o700)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(to, rel), b, info.Mode().Perm())
	})
	if err != nil {
		t.Fatal(err)
	}
	return to
}

func readTrim(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}
