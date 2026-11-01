// Command hello is Hello World on the Go SDK (ADR-0027): a Level 1 Utility
// that publishes its MAM and Health until SIGINT/SIGTERM. OEC_ROOT
// (optional) is the root of the container storage, "/" in a container
// (REQ-CNT-004).
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	oec "github.com/OI4/oec/sdk/go"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "hello:", err)
		os.Exit(1)
	}
}

func run() error {
	app, err := oec.New(oec.Options{
		Level:       oec.Level1,
		ServiceType: oec.Utility,
		Product: oec.Product{
			ManufacturerURI:  "example.com",
			Model:            "Hello OEC Go",
			ProductCode:      "HELLO-GO-1",
			Manufacturer:     "Example Inc.",
			SoftwareRevision: "1.0.0",
			DeviceManual:     "https://example.com/hello/manual.pdf",
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Println("running", oec.Version())
	return app.Run(ctx) // until SIGINT/SIGTERM
}
