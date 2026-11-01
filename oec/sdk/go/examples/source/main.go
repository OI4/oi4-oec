// Command source is a Level 3 OT connector on the Go SDK (ADR-0027): it
// represents a field device without an MQTT client of its own, publishes
// its process value, accepts a new setpoint by Set Data, reads its poll
// interval from its Config and reports a reached setpoint as Generic
// event. OEC_ROOT (optional) is the root of the container storage, "/" in
// a container (REQ-CNT-004); the SDK keeps its state in /etc/oi4/app.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	oec "github.com/OI4/oec/sdk/go"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "source:", err)
		os.Exit(1)
	}
}

// sensor is the simulated field device: it approaches its setpoint.
type sensor struct {
	mu      sync.Mutex
	pv, sv  float64
	reached bool
}

func run() error {
	app, err := oec.New(oec.Options{
		Level:       oec.Level3,
		ServiceType: oec.OTConnector,
		Product: oec.Product{
			ManufacturerURI:  "example.com",
			Model:            "Demo Connector Go",
			ProductCode:      "CONN-GO-1",
			Manufacturer:     "Example Inc.",
			SoftwareRevision: "1.0.0",
			DeviceManual:     "https://example.com/connector/manual.pdf",
		},
		ContainerRoot: os.Getenv("OEC_ROOT"),
	})
	if err != nil {
		return err
	}
	defer app.Close()
	if err := app.This().SetHealth(oec.Normal, 100); err != nil {
		return err
	}

	// The poll interval of the field bus, changeable by Set Config.
	interval := oec.NumberObject("interval", 1)
	interval.Min, interval.Max, interval.Unit, interval.Label = new(0.1), new(60.0), "s", "Poll interval"
	cfg, err := app.This().AddConfig(oec.ConfigDef{Context: "acquisition", Label: "Acquisition",
		Groups: []oec.ConfigGroup{{Name: "common", Label: "Common settings", Objects: []oec.ConfigObject{interval}}}})
	if err != nil {
		return err
	}

	// The field device: no Identification Link of its own → Oi4Identifier.
	device, err := app.AddAsset(oec.AssetOptions{
		Product:      oec.Product{ManufacturerURI: "sensor-maker.com", Model: "TS-100", ProductCode: "TS-100-A", HardwareRevision: "2"},
		SerialNumber: "4711",
	})
	if err != nil {
		return err
	}
	if err := device.SetHealth(oec.Normal, 100); err != nil {
		return err
	}
	// Oi4Pv: published on a change of at least 0.1 °C; consumers may change
	// Mode and Interval by Set PublicationList.
	temperature, err := device.AddData(oec.DataDef{
		Tag: oec.OI4PV,
		Fields: []oec.Field{
			{Name: "Pv", Type: oec.TypeDouble, Unit: "°C", Description: "Temperature", Precision: 0.1},
			{Name: "Sv1", Type: oec.TypeDouble, Unit: "°C", Description: "Setpoint", Range: &oec.Range{Low: -20, High: 80}},
		},
		Writable: oec.WritableModeAndInterval,
	})
	if err != nil {
		return err
	}
	s := &sensor{pv: 21, sv: 22}
	if err := temperature.Update(map[string]float64{"Pv": s.pv, "Sv1": s.sv}); err != nil {
		return err
	}
	// Writable setpoint: the SDK checks the types, answers with the Status
	// event and publishes the new state; the handler only decides. It runs
	// on the SDK thread and must not block.
	if err := temperature.OnSet(func(values json.RawMessage) error {
		var req struct{ Sv1 float64 }
		if err := json.Unmarshal(values, &req); err != nil || req.Sv1 < -20 || req.Sv1 > 80 {
			return oec.BadOutOfRange
		}
		s.mu.Lock()
		s.sv, s.reached = req.Sv1, false
		s.mu.Unlock()
		return nil
	}); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go poll(ctx, s, cfg, device, temperature)
	fmt.Println("running", oec.Version())
	return app.Run(ctx) // until SIGINT/SIGTERM
}

// poll is the field bus: every poll interval the value moves towards the
// setpoint; reaching it is reported once per setpoint.
func poll(ctx context.Context, s *sensor, cfg *oec.Config, device *oec.Asset, temperature *oec.DataSet) {
	for {
		every := time.Second
		if v, ok, err := cfg.Get("common", "interval"); err == nil && ok {
			if secs, err := strconv.ParseFloat(v, 64); err == nil {
				every = time.Duration(secs * float64(time.Second))
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
		s.mu.Lock()
		s.pv += (s.sv - s.pv) / 4
		pv, sv := math.Round(s.pv*100)/100, s.sv
		reached := !s.reached && math.Abs(pv-sv) < 0.1
		if reached {
			s.reached = true
		}
		s.mu.Unlock()
		if temperature.Update(map[string]float64{"Pv": pv}) != nil {
			return // app closed
		}
		if reached {
			_ = device.EmitEvent(oec.Event{Level: "Setpoint", Number: 1, Description: "setpoint reached",
				Details: map[string]float64{"Sv1": sv}})
		}
	}
}
