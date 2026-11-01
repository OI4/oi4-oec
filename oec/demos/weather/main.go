// Command oec-weather is the OEC demo weather service D3 (ADR-0028): an
// OTConnector on Messaging Level 3 on the Go SDK (ADR-0027). Its weather
// stations are represented assets with the Data Oi4Pv (temperature,
// humidity, pressure, wind) and a Config with their location; the
// application Config selects the provider, the number of stations, the
// poll interval and the thresholds of the warnings Frost and Storm
// (Generic events). The provider simulation is offline and deterministic,
// the provider met-no reads the Locationforecast of MET Norway (ADR-0010;
// data under CC BY 4.0). A station without data has the Health
// FAILURE_1/0, the application then OFF_SPEC_3. OEC_ROOT (optional) is
// the root of the container storage, "/" in a container (REQ-CNT-004);
// the SDK keeps its state in /etc/oi4/app.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	oec "github.com/OI4/oec/sdk/go"
)

// version is the SoftwareRevision of the service and its stations.
const version = "0.1.0"

// product is the MAM of the application (A-341).
var product = oec.Product{
	ManufacturerURI:  "oi4.org",
	Model:            "OEC Weather",
	ProductCode:      "OEC-WEATHER",
	Manufacturer:     "Open Industry 4.0 Alliance",
	Description:      "Demo weather service of OI4 Open Edge Computing",
	SoftwareRevision: version,
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "oec-weather:", err)
		os.Exit(1)
	}
}

func run() error {
	root := os.Getenv("OEC_ROOT")
	app, err := oec.New(oec.Options{Level: oec.Level3, ServiceType: oec.OTConnector, Product: product, ContainerRoot: root})
	if err != nil {
		return err
	}
	defer app.Close()
	if err := app.This().SetHealth(oec.Normal, 100); err != nil {
		return err
	}
	// OEC_WEATHER_METNO_URL replaces the endpoint of met-no for tests
	// (a fake server); it is no part of the Config (A-350).
	metnoURL := os.Getenv("OEC_WEATHER_METNO_URL")
	if metnoURL != "" {
		fmt.Fprintln(os.Stderr, "oec-weather: met-no endpoint", metnoURL, "(OEC_WEATHER_METNO_URL, for tests)")
	}
	svc := newService(app, hostname(root), metnoURL, os.Stdout)
	if err := svc.start(); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	fmt.Println("running", oec.Version())
	done := make(chan error, 1)
	go func() {
		err := svc.run(ctx)
		if err != nil {
			stop()
		}
		done <- err
	}()
	err = app.Run(ctx)
	stop()
	if perr := <-done; perr != nil && err == nil {
		err = perr
	}
	return err
}
