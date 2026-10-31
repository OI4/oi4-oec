package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/OI4/oec/conformance/internal/container"
	"github.com/OI4/oec/conformance/internal/report"
	"github.com/OI4/oec/conformance/internal/verdict"
	schema "github.com/OI4/oec/schema"
)

type containerFlags struct {
	fl        *flag.FlagSet
	config    string
	daemon    string
	level     int
	timeout   time.Duration
	tests     list
	format    string
	reports   reports
	allowSkip bool
}

func newContainerFlags(stderr io.Writer) *containerFlags {
	f := &containerFlags{fl: flag.NewFlagSet("container", flag.ContinueOnError)}
	fl := f.fl
	fl.SetOutput(stderr)
	fl.Usage = func() { fmt.Fprint(stderr, usage) }
	fl.StringVar(&f.config, "config", "", "YAML file with flag values (keys = flag names); flags override it")
	fl.StringVar(&f.daemon, "daemon", "", "Engine API endpoint: unix://PATH, npipe://PATH, tcp://HOST:PORT or a socket path (default: DOCKER_HOST, CONTAINER_HOST, the default sockets)")
	fl.IntVar(&f.level, "level", 3, "declared compliance level of the application (1, 2, 3)")
	fl.DurationVar(&f.timeout, "timeout", container.DefaultTimeout, "waiting time for one request to the Engine API")
	fl.Var(&f.tests, "tests", "test ids or patterns (path.Match) to run, comma separated; repeatable (default: all)")
	fl.StringVar(&f.format, "format", "text", "output on standard output: text, json, junit")
	fl.Var(&f.reports, "report", "write a report file FORMAT=PATH (repeatable)")
	fl.BoolVar(&f.allowSkip, "allow-skip", false, "exit 0 instead of 4 if a mandatory requirement could not be judged")
	return f
}

func runContainer(args []string, stdout, stderr io.Writer) int {
	usageErr := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "%s: %s\n", tool, fmt.Sprintf(format, a...))
		return exitUsage
	}
	f := newContainerFlags(stderr)
	if err := f.fl.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitPass
		}
		return exitUsage
	}
	if f.fl.NArg() == 0 {
		return usageErr("container takes one or more container names or ids")
	}
	if err := applyConfig(f.fl, f.config); err != nil {
		return usageErr("%v", err)
	}
	if f.level < 1 || f.level > 3 {
		return usageErr("--level must be 1, 2 or 3")
	}
	if !validFormat(f.format) {
		return usageErr("unknown --format %q (text, json, junit)", f.format)
	}
	if f.timeout <= 0 {
		return usageErr("--timeout must be positive")
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
	for _, t := range container.Tests(cat) {
		if (container.Config{Tests: f.tests}).Runs(t.ID) {
			tests = append(tests, t)
		}
	}
	if len(tests) == 0 {
		return usageErr("--tests selects no container test")
	}
	names := f.fl.Args()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	started := time.Now().UTC()
	res, err := container.Run(ctx, container.Config{Containers: names, Daemon: f.daemon, Tests: f.tests, Timeout: f.timeout})
	if err != nil {
		if errors.Is(err, container.ErrNoContainer) {
			return usageErr("%v", err)
		}
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	run := res.Run
	rep := &report.Infra{Report: report.InfraVersion, Tool: tool, Version: version, Schemas: schema.Version, Baseline: base,
		Catalog: cat.Baseline, Command: append([]string{tool, "container"}, args...), Mode: "container",
		Input: strings.Join(names, ", "), Level: f.level, Started: started, Container: &run}
	rep.Tests = verdict.Build(tests, res.Observations, reqs, f.level)
	rep.Summary = verdict.Summary(rep.Tests)
	rep.Exit = verdict.Exit(rep.Tests, f.allowSkip)
	rep.Ended = time.Now().UTC()
	return writeReports(rep, f.format, f.reports, stdout, stderr)
}
