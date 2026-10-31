package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/OI4/oec/conformance/internal/report"
	runpkg "github.com/OI4/oec/conformance/internal/run"
	"github.com/OI4/oec/conformance/internal/verdict"
	schema "github.com/OI4/oec/schema"
)

type runFlags struct {
	fl         *flag.FlagSet
	config     string
	daemon     string
	level      int
	timeout    time.Duration
	tests      list
	format     string
	reports    reports
	allowSkip  bool
	wait       time.Duration
	settle     time.Duration
	listen     string
	brokerHost string
	maxPacket  int
}

func newRunFlags(stderr io.Writer) *runFlags {
	f := &runFlags{fl: flag.NewFlagSet("run", flag.ContinueOnError)}
	fl := f.fl
	fl.SetOutput(stderr)
	fl.Usage = func() { fmt.Fprint(stderr, usage) }
	fl.StringVar(&f.config, "config", "", "YAML file with flag values (keys = flag names); flags override it")
	fl.StringVar(&f.daemon, "daemon", "", "Engine API endpoint on this machine: unix://PATH, npipe://PATH, tcp://LOOPBACK:PORT or a socket path (default: DOCKER_HOST, CONTAINER_HOST, the default sockets)")
	fl.IntVar(&f.level, "level", 3, "declared compliance level of the application (1, 2, 3)")
	fl.DurationVar(&f.timeout, "timeout", runpkg.DefaultTimeout, "waiting time for one request to the Engine API")
	fl.Var(&f.tests, "tests", "test ids or patterns (path.Match) to run, comma separated; repeatable (default: all)")
	fl.StringVar(&f.format, "format", "text", "output on standard output: text, json, junit")
	fl.Var(&f.reports, "report", "write a report file FORMAT=PATH (repeatable)")
	fl.BoolVar(&f.allowSkip, "allow-skip", false, "exit 0 instead of 4 if a mandatory requirement could not be judged")
	fl.DurationVar(&f.wait, "wait", runpkg.DefaultWait, "longest observation of one container (until its Birth message)")
	fl.DurationVar(&f.settle, "settle", runpkg.DefaultSettle, "observation after the Birth message")
	fl.StringVar(&f.listen, "listen", "", "address of the MQTT test endpoint (default: 127.0.0.1 on macOS and Windows, the gateway of the bridge network elsewhere)")
	fl.StringVar(&f.brokerHost, "broker-host", runpkg.DefaultBrokerHost, "Address of the generated broker.json: a host name (resolved to host-gateway in the container) or an IP address of the endpoint")
	fl.IntVar(&f.maxPacket, "max-packet-size", runpkg.DefaultMaxPacketSize, "MaxPacketSize of the generated broker.json")
	return f
}

func runRun(args []string, stdout, stderr io.Writer) int {
	usageErr := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "%s: %s\n", tool, fmt.Sprintf(format, a...))
		return exitUsage
	}
	f := newRunFlags(stderr)
	if err := f.fl.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitPass
		}
		return exitUsage
	}
	if f.fl.NArg() != 1 {
		return usageErr("run takes exactly one image (name, id or unique id prefix of an image the engine has)")
	}
	if err := applyConfig(f.fl, f.config); err != nil {
		return usageErr("%v", err)
	}
	switch {
	case f.level < 1 || f.level > 3:
		return usageErr("--level must be 1, 2 or 3")
	case !validFormat(f.format):
		return usageErr("unknown --format %q (text, json, junit)", f.format)
	case f.timeout <= 0 || f.wait <= 0 || f.settle <= 0:
		return usageErr("--timeout, --wait and --settle must be positive")
	case f.maxPacket < 1 || f.maxPacket > 268435455-runpkg.BrokerOverhead:
		return usageErr("--max-packet-size must be 1 … %d", 268435455-runpkg.BrokerOverhead)
	case f.brokerHost == "":
		return usageErr("--broker-host must not be empty")
	}
	cat, base, err := load()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	reqs, err := verdict.LoadRequirements()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	cfg := runpkg.Config{
		Image: f.fl.Arg(0), Daemon: f.daemon, Tests: f.tests, Timeout: f.timeout, Wait: f.wait, Settle: f.settle,
		Listen: f.listen, BrokerHost: f.brokerHost, MaxPacketSize: f.maxPacket,
	}
	var tests []verdict.Test
	for _, t := range runpkg.Tests(cat) {
		if cfg.Runs(t.ID) {
			tests = append(tests, t)
		}
	}
	if len(tests) == 0 {
		return usageErr("--tests selects no run test")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	started := time.Now().UTC()
	res, err := runpkg.Run(ctx, cfg)
	if err != nil {
		if errors.Is(err, runpkg.ErrNoImage) || errors.Is(err, runpkg.ErrRemote) {
			return usageErr("%v", err)
		}
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	if res.Run.Cleanup != "" {
		fmt.Fprintf(stderr, "%s: cleanup: %s\n", tool, res.Run.Cleanup)
	}
	run := res.Run
	rep := &report.Infra{Report: report.InfraVersion, Tool: tool, Version: version, Schemas: schema.Version, Baseline: base,
		Catalog: cat.Baseline, Command: append([]string{tool, "run"}, args...), Mode: "run",
		Input: f.fl.Arg(0), Level: f.level, Started: started, Run: &run}
	rep.Tests = verdict.Build(tests, res.Observations, reqs, f.level)
	rep.Summary = verdict.Summary(rep.Tests)
	rep.Exit = verdict.Exit(rep.Tests, f.allowSkip)
	rep.Ended = time.Now().UTC()
	return writeReports(rep, f.format, f.reports, stdout, stderr)
}
