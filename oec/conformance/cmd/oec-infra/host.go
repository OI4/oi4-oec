package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/OI4/oec/conformance/internal/host"
	"github.com/OI4/oec/conformance/internal/report"
	"github.com/OI4/oec/conformance/internal/verdict"
	schema "github.com/OI4/oec/schema"
)

// hostLevel is the level the host tests are judged at: the platform has no
// declared level of its own (DESIGN.md §9).
const hostLevel = 3

type hostFlags struct {
	fl                         *flag.FlagSet
	config                     string
	root, mqtt, certs, secrets string
	container, daemon, self    string
	offline                    bool
	registryWait, timeout      time.Duration
	tests                      list
	format                     string
	reports                    reports
	allowSkip                  bool
}

func newHostFlags(stderr io.Writer) *hostFlags {
	f := &hostFlags{fl: flag.NewFlagSet("host", flag.ContinueOnError)}
	fl := f.fl
	fl.SetOutput(stderr)
	fl.Usage = func() { fmt.Fprint(stderr, usage) }
	fl.StringVar(&f.config, "config", "", "YAML file with flag values (keys = flag names); flags override it")
	fl.StringVar(&f.root, "root", string(filepath.Separator), "root directory with the storages of RIG 4.1 (the file system of an application container)")
	fl.StringVar(&f.mqtt, "mqtt-storage", "", "Message Bus storage (default ROOT/etc/oi4/mqtt)")
	fl.StringVar(&f.certs, "certs-storage", "", "OI4 certificate storage (default ROOT/etc/oi4/certs)")
	fl.StringVar(&f.secrets, "secrets-storage", "", "secret storage (default ROOT/run/secrets/oi4)")
	fl.StringVar(&f.container, "container", "", "container name of the application (default: ROOT/etc/hostname)")
	fl.StringVar(&f.daemon, "daemon", "", "endpoint of the container daemon: unix://PATH, npipe://PATH, tcp://HOST:PORT or a socket path (default: DOCKER_HOST, CONTAINER_HOST, the default sockets)")
	fl.StringVar(&f.self, "self", "", "the container D8 runs in (name, id or unique id prefix); T8-HST-03 neither judges it nor compares its mounts")
	fl.BoolVar(&f.offline, "offline", false, "do not contact the Message Bus (REQ-PLT-007/008 inconclusive)")
	fl.DurationVar(&f.registryWait, "registry-wait", host.DefaultRegistryWait, "discovery time of the OEC Registry")
	fl.DurationVar(&f.timeout, "timeout", host.DefaultTimeout, "waiting time for one connection")
	fl.Var(&f.tests, "tests", "test ids or patterns (path.Match) to run, comma separated; repeatable (default: all)")
	fl.StringVar(&f.format, "format", "text", "output on standard output: text, json, junit")
	fl.Var(&f.reports, "report", "write a report file FORMAT=PATH (repeatable)")
	fl.BoolVar(&f.allowSkip, "allow-skip", false, "exit 0 instead of 4 if a mandatory requirement could not be judged")
	return f
}

func runHost(args []string, stdout, stderr io.Writer) int {
	usageErr := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "%s: %s\n", tool, fmt.Sprintf(format, a...))
		return exitUsage
	}
	f := newHostFlags(stderr)
	if err := f.fl.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitPass
		}
		return exitUsage
	}
	if f.fl.NArg() != 0 {
		return usageErr("host takes no arguments (the root is --root DIR)")
	}
	if err := applyConfig(f.fl, f.config); err != nil {
		return usageErr("%v", err)
	}
	if !validFormat(f.format) {
		return usageErr("unknown --format %q (text, json, junit)", f.format)
	}
	if f.timeout <= 0 || f.registryWait <= 0 {
		return usageErr("--timeout and --registry-wait must be positive")
	}
	root, err := filepath.Abs(f.root)
	if err != nil {
		return usageErr("--root: %v", err)
	}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return usageErr("--root %s is no directory", f.root)
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
	var tests []verdict.Test
	for _, t := range host.Tests(cat) {
		if (host.Config{Tests: f.tests}).Runs(t.ID) {
			tests = append(tests, t)
		}
	}
	if len(tests) == 0 {
		return usageErr("--tests selects no host test")
	}
	abs := func(p string) string {
		if p == "" {
			return ""
		}
		if a, err := filepath.Abs(p); err == nil {
			return a
		}
		return p
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	started := time.Now().UTC()
	res := host.Run(ctx, host.Config{Root: root, MQTT: abs(f.mqtt), Certs: abs(f.certs), Secrets: abs(f.secrets),
		Container: f.container, Daemon: f.daemon, Self: f.self, Offline: f.offline, RegistryWait: f.registryWait, Timeout: f.timeout, Tests: f.tests})
	h := res.Host
	rep := &report.Infra{Report: report.InfraVersion, Tool: tool, Version: version, Schemas: schema.Version, Baseline: base,
		Catalog: cat.Baseline, Command: append([]string{tool, "host"}, args...), Mode: "host", Input: root,
		Level: hostLevel, Started: started, Host: &h}
	rep.Tests = verdict.Build(tests, res.Observations, reqs, hostLevel)
	rep.Summary = verdict.Summary(rep.Tests)
	rep.Exit = verdict.Exit(rep.Tests, f.allowSkip)
	rep.Ended = time.Now().UTC()
	return writeReports(rep, f.format, f.reports, stdout, stderr)
}
