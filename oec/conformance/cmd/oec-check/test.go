package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/OI4/oec/conformance/catalog"
	"github.com/OI4/oec/conformance/internal/ilink"
	"github.com/OI4/oec/conformance/internal/monitor"
	"github.com/OI4/oec/conformance/internal/probe"
	"github.com/OI4/oec/conformance/internal/proxy"
	"github.com/OI4/oec/conformance/internal/report"
	"github.com/OI4/oec/conformance/internal/validate"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// runTest runs oec-check test (DESIGN.md §3.3, ADR-0025 amendment C-062):
// D7 sends requests to the component under test and judges the answers;
// the passive monitor checks run on the same subscription.
func runTest(args []string, stdout, stderr io.Writer) int {
	f := newMonitorFlags("test", stderr)
	if err := f.fl.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitValid
		}
		return exitUsage
	}
	usageErr := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "%s: "+format+"\n", append([]any{tool}, a...)...)
		return exitUsage
	}
	if f.fl.NArg() > 0 {
		return usageErr("test takes no arguments, got %q", f.fl.Args())
	}
	if err := f.applyConfig(); err != nil {
		return usageErr("%v", err)
	}
	if f.app == "" {
		return usageErr("test needs --app (AppId or <ServiceType>/<AppId>)")
	}
	st, app := probe.SplitApp(f.app)
	if st != "" && !slices.Contains(probe.ServiceTypes, st) {
		return usageErr("--app: unknown ServiceType %q", st)
	}
	if v := ilink.Validate(app); len(v) > 0 {
		return usageErr("--app: %s is no valid AppId: %s (the elements of an Oi4Identifier are dnp-encoded, e.g. ' ' as ',20')", app, v[0].Detail)
	}
	if f.testerIL != "" {
		if v := ilink.Validate(f.testerIL); len(v) > 0 {
			return usageErr("--tester-il: %s", v[0].Detail)
		}
		// The MAM of the announced tester takes the elements of the
		// Oi4Identifier (MG 4.3).
		if lv := strings.SplitN(f.testerIL, "/", 4); f.announce && (len(lv) < 4 || slices.Contains(lv, "")) {
			return usageErr("--announce needs --tester-il as an Oi4Identifier <ManufacturerUri>/<Model>/<ProductCode>/<SerialNumber>")
		}
	}
	if f.level < 1 || f.level > 3 {
		return usageErr("--level must be 1, 2 or 3")
	}
	if f.format != "text" && f.format != "json" && f.format != "junit" {
		return usageErr("unknown format %q (text, json, junit)", f.format)
	}
	if f.timeout <= 0 || f.heartbeat <= 0 || f.tolerance < 0 || f.maxPacketSize < 0 {
		return usageErr("--timeout and --heartbeat must be positive, --tolerance and --max-packet-size not negative")
	}
	viaProxy, err := f.usesProxy()
	if err != nil {
		return usageErr("%v", err)
	}
	restart, err := f.restartHook(stderr)
	if err != nil {
		return usageErr("%v", err)
	}
	all, err := probe.AllTests()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	for _, p := range f.tests {
		if !slices.ContainsFunc(all, func(t verdict.Test) bool { return probe.Selected([]string{p}, t.ID) }) {
			return usageErr("--tests: %q matches no test of the catalog", p)
		}
	}
	cfg, mps, err := f.connection()
	if err != nil {
		return usageErr("%v", err)
	}
	var px *proxy.Proxy
	if viaProxy {
		if px, err = f.newProxy(cfg); err != nil {
			return usageErr("%v", err)
		}
		defer px.Close()
		f.warnProxy(px, cfg.Broker, stderr)
	}
	v, err := validate.New()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	cat, err := catalog.Load(catalog.D7File)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	reqs, err := verdict.LoadRequirements()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	opt := monitor.DefaultOptions()
	opt.Heartbeat, opt.Tolerance, opt.MaxPacketSize = f.heartbeat, f.tolerance, f.maxPacketSize
	if opt.MaxPacketSize == 0 {
		opt.MaxPacketSize = mps
	}
	if cfg.Insecure {
		fmt.Fprintf(stderr, "%s: WARNING: --insecure: the certificate of %s is NOT verified; dev only, the capture and the report are marked\n", tool, cfg.Broker)
	}
	attachment := "bus"
	if px != nil {
		attachment = "proxy"
	}
	rep := &report.Monitor{Report: report.MonitorVersion, Tool: tool, Version: version, Schemas: v.SchemaVersion(), Baseline: v.Baseline(),
		Catalog: cat.Baseline, Command: command("test", args), Mode: "test", Attachment: attachment, Level: f.level, App: f.app,
		SelfAssessment: true, Broker: cfg.Broker, Insecure: cfg.Insecure, Capture: f.record}
	if px != nil {
		rep.ProxyListen = px.Addr()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	o, code := newObserver(v, cfg, opt, f.record, rep, time.Now().UTC(), stderr)
	if o == nil {
		return code
	}
	defer o.file.Close()
	fmt.Fprintf(stderr, "%s: testing %s on %s (timeout %s)\n", tool, f.app, cfg.Broker, f.timeout)
	if px != nil {
		fmt.Fprintf(stderr, "%s: proxy attachment on %s: the component under test connects there (waiting up to %s)\n", tool, px.Addr(), f.proxyWait)
	}
	s, err := probe.RunBus(ctx, cfg, probe.Options{App: f.app, Level: f.level, Timeout: f.timeout, TesterIL: f.testerIL, Tests: f.tests,
		Proxy: px, ProxyWait: f.proxyWait, Tolerance: f.tolerance, Restart: restart, RestartWait: f.restartWait, Peer: f.peer, Announce: f.announce},
		reqs, o.observe)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	ended, code := o.finish(s.Ended, s.Cause, rep, stderr)
	if code != 0 {
		return code
	}
	res := o.e.Finish(ended)
	rep.Observed = rep.Ended.Sub(rep.Started).Round(time.Millisecond).String()
	rep.Messages, rep.Namespace = res.Messages, res.Namespace
	rep.Tester, rep.Requests, rep.Restore, rep.Announced = s.Probe.Tester, s.Probe.Requests, s.Probe.Restore, s.Probe.Announced
	if px != nil {
		rep.Connections = px.Conns()
	}
	if rs := s.Probe.Restore; rs != nil && !rs.Restored {
		fmt.Fprintf(stderr, "%s: WARNING: %s\n", tool, rs.Summary())
		for _, l := range rs.Changed {
			fmt.Fprintf(stderr, "%s:   %s\n", tool, l)
		}
	}
	if rep.Tests, rep.Participants, err = probe.Judge(f.app, f.level, f.tests, res, s, reqs); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	if rep.Participants == nil {
		rep.Participants = []monitor.Participant{}
	}
	rep.Summary = verdict.Summary(rep.Tests)
	rep.Exit = verdict.Exit(rep.Tests, f.allowSkip)
	return writeReports(rep, f, stdout, stderr)
}
