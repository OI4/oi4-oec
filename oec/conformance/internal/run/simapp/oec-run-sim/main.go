// Command oec-run-sim is the simulated application of oec-infra run as a
// static binary for an image of the opt-in run with Docker (C-074).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/OI4/oec/conformance/internal/run/simapp"
)

func main() {
	fault := flag.String("fault", string(simapp.Good), "deviation of the simulated application")
	flag.Parse()
	f := simapp.Fault(*fault)
	if !simapp.Known(f) {
		fmt.Fprintf(os.Stderr, "oec-run-sim: unknown fault %q\n", *fault)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := simapp.Run(ctx, simapp.Options{Fault: f, Log: func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "oec-run-sim: "+format+"\n", args...)
	}})
	stop()
	os.Exit(code)
}
