package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/OI4/oec/conformance/catalog"
	"github.com/OI4/oec/conformance/internal/capture"
	"github.com/OI4/oec/conformance/internal/monitor"
	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/report"
	"github.com/OI4/oec/conformance/internal/topic"
	"github.com/OI4/oec/conformance/internal/validate"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// Subscription is the topic filter of a monitor run (DESIGN.md §3.2).
const subscription = topic.Namespace + "/#"

// DefaultDuration of a live run: longer than monitor.MinDuration, so the
// heartbeat rule is judged.
const defaultDuration = 5 * time.Minute

// reports is the repeatable flag --report <format>=<path>.
type reports []string

func (r *reports) String() string { return strings.Join(*r, ",") }

func (r *reports) Set(v string) error {
	f, p, ok := strings.Cut(v, "=")
	if !ok || p == "" {
		return fmt.Errorf("want <format>=<path>, got %q", v)
	}
	if f != "text" && f != "json" && f != "junit" {
		return fmt.Errorf("unknown report format %q (text, json, junit)", f)
	}
	*r = append(*r, v)
	return nil
}

type monitorFlags struct {
	fl                                  *flag.FlagSet
	replay, record, app, format, config string
	broker, ca, cert, key, keyPassFile  string
	username, passwordFile, busStorage  string
	clientID                            string
	insecure, allowSkip                 bool
	level, maxPacketSize                int
	duration, heartbeat, tolerance      time.Duration
	reports                             reports
}

func newMonitorFlags(stderr io.Writer) *monitorFlags {
	f := &monitorFlags{fl: flag.NewFlagSet("monitor", flag.ContinueOnError)}
	fl := f.fl
	fl.SetOutput(stderr)
	fl.Usage = func() { fmt.Fprint(stderr, usage) }
	fl.StringVar(&f.config, "config", "", "YAML file with flag values (keys = flag names); flags override it")
	fl.StringVar(&f.replay, "replay", "", "judge a capture offline instead of a broker")
	fl.StringVar(&f.record, "record", "", "write the observed messages to a capture (JSONL)")
	fl.StringVar(&f.app, "app", "", "component under test: AppId (Identification Link) or <ServiceType>/<AppId>")
	fl.IntVar(&f.level, "level", 3, "declared level 1, 2 or 3 (REQ-CMP-011: default 3)")
	fl.StringVar(&f.format, "format", "text", "report format on standard output: text, json or junit")
	fl.Var(&f.reports, "report", "additional report <format>=<path> (text, json, junit); repeatable")
	fl.BoolVar(&f.allowSkip, "allow-skip", false, "exit 0 instead of 4 if a mandatory requirement could not be judged")
	fl.DurationVar(&f.duration, "duration", defaultDuration, "observation time of a live run")
	fl.DurationVar(&f.heartbeat, "heartbeat", 60*time.Second, "Health heartbeat (REQ-TOP-017)")
	fl.DurationVar(&f.tolerance, "tolerance", 5*time.Second, "tolerance of the time-based rules")
	fl.IntVar(&f.maxPacketSize, "max-packet-size", 0, "MaxPacketSize of the Message Bus (default: broker.json of --bus-storage or the capture header)")
	fl.StringVar(&f.broker, "broker", "", "broker URL mqtts://host[:port]")
	fl.StringVar(&f.ca, "ca", "", "PEM CA bundle that verifies the broker (default: system roots)")
	fl.StringVar(&f.cert, "cert", "", "client certificate (PEM)")
	fl.StringVar(&f.key, "key", "", "private key of the client certificate (PEM, also encrypted PKCS#8)")
	fl.StringVar(&f.keyPassFile, "key-passphrase-file", "", "file with the passphrase of an encrypted key")
	fl.StringVar(&f.username, "username", "", "MQTT user name")
	fl.StringVar(&f.passwordFile, "password-file", "", "file with the MQTT password (no password on the command line)")
	fl.StringVar(&f.busStorage, "bus-storage", "", "container storage root (RIG 4.1: etc/oi4/mqtt/broker.json, certificates, run/secrets/oi4)")
	fl.StringVar(&f.clientID, "client-id", "", "MQTT client identifier (default: oec-check-<random>)")
	fl.BoolVar(&f.insecure, "insecure", false, "do not verify the broker certificate (dev only; marked in capture and report)")
	return f
}

// applyConfig sets the flags that the command line did not set from the
// YAML file (DESIGN.md §8). Unknown keys are usage errors.
func (f *monitorFlags) applyConfig() error {
	if f.config == "" {
		return nil
	}
	raw, err := os.ReadFile(f.config)
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("%s: %w", f.config, err)
	}
	set := map[string]bool{}
	f.fl.Visit(func(fl *flag.Flag) { set[fl.Name] = true })
	for k, v := range doc {
		if k == "config" || k == "replay" || f.fl.Lookup(k) == nil {
			return fmt.Errorf("%s: unknown key %q", f.config, k)
		}
		if set[k] {
			continue
		}
		vals := []any{v}
		if l, ok := v.([]any); ok {
			vals = l
		}
		for _, x := range vals {
			if err := f.fl.Set(k, fmt.Sprint(x)); err != nil {
				return fmt.Errorf("%s: %s: %w", f.config, k, err)
			}
		}
	}
	return nil
}

func secretFile(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return bytes.TrimRight(b, "\r\n"), nil
}

// connection builds the client configuration: the container storage first,
// the explicit flags override it.
func (f *monitorFlags) connection() (mqttwire.Config, int, error) {
	var c mqttwire.Config
	mps := 0
	if f.busStorage != "" {
		s, err := mqttwire.ReadStorage(f.busStorage)
		if err != nil {
			return c, 0, err
		}
		c, mps = s.Config, s.MaxPacketSize
	}
	if f.broker != "" {
		c.Broker = f.broker
	}
	if f.ca != "" {
		c.CAFile = f.ca
	}
	if f.cert != "" || f.key != "" {
		if f.cert == "" || f.key == "" {
			return c, 0, errors.New("--cert and --key go together")
		}
		c.CertFile, c.KeyFile, c.KeyPassphrase = f.cert, f.key, nil
	}
	if f.keyPassFile != "" {
		p, err := secretFile(f.keyPassFile)
		if err != nil {
			return c, 0, err
		}
		c.KeyPassphrase = p
	}
	if f.username != "" {
		c.Username = f.username
	}
	if f.passwordFile != "" {
		p, err := secretFile(f.passwordFile)
		if err != nil {
			return c, 0, err
		}
		c.Password = p
	}
	if c.Password != nil && c.Username == "" {
		return c, 0, errors.New("--password-file needs --username")
	}
	if c.Broker == "" {
		return c, 0, errors.New("monitor needs --broker, --bus-storage or --replay")
	}
	if _, _, err := mqttwire.BrokerAddress(c.Broker); err != nil {
		return c, 0, err
	}
	c.Insecure = f.insecure
	c.ClientID = f.clientID
	return c, mps, nil
}

// command returns the command line without secrets (no secret is taken on
// the command line; file paths stay).
func command(args []string) []string {
	return append([]string{tool, "monitor"}, args...)
}

func runMonitor(args []string, stdout, stderr io.Writer) int {
	f := newMonitorFlags(stderr)
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
		return usageErr("monitor takes no arguments, got %q", f.fl.Args())
	}
	if err := f.applyConfig(); err != nil {
		return usageErr("%v", err)
	}
	if f.level < 1 || f.level > 3 {
		return usageErr("--level must be 1, 2 or 3")
	}
	if f.format != "text" && f.format != "json" && f.format != "junit" {
		return usageErr("unknown format %q (text, json, junit)", f.format)
	}
	if f.heartbeat <= 0 || f.tolerance < 0 || f.duration <= 0 || f.maxPacketSize < 0 {
		return usageErr("--heartbeat and --duration must be positive, --tolerance and --max-packet-size not negative")
	}
	if f.replay != "" {
		for _, n := range []string{"record", "broker", "ca", "cert", "key", "key-passphrase-file", "username", "password-file", "bus-storage", "client-id", "insecure", "duration"} {
			if fl := f.fl.Lookup(n); fl.Value.String() != fl.DefValue {
				return usageErr("--%s is a live option, not allowed with --replay", n)
			}
		}
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
	opt := monitor.DefaultOptions()
	opt.Heartbeat, opt.Tolerance, opt.MaxPacketSize = f.heartbeat, f.tolerance, f.maxPacketSize
	rep := &report.Monitor{Report: report.MonitorVersion, Tool: tool, Version: version, Schemas: v.SchemaVersion(), Baseline: v.Baseline(),
		Catalog: cat.Baseline, Command: command(args), Attachment: "bus", Level: f.level, App: f.app, SelfAssessment: true}

	var res monitor.Result
	if f.replay != "" {
		rep.Mode, rep.Capture = "replay", f.replay
		data, err := os.ReadFile(f.replay)
		if err != nil {
			return usageErr("%v", err)
		}
		c, err := capture.Read(bytes.NewReader(data))
		if err != nil {
			return usageErr("%s: %v", f.replay, err)
		}
		rep.Broker, rep.Insecure, rep.Started, rep.Ended = c.Header.Broker, c.Header.Insecure, c.Header.Started, c.End()
		if c.Trailer != nil {
			rep.Error = c.Trailer.Error
		}
		if res, err = monitor.Replay(v, c, opt); err != nil {
			fmt.Fprintf(stderr, "%s: %s: internal error: %v\n", tool, f.replay, err)
			return exitInternal
		}
	} else {
		cfg, mps, err := f.connection()
		if err != nil {
			return usageErr("%v", err)
		}
		if opt.MaxPacketSize == 0 {
			opt.MaxPacketSize = mps
		}
		if cfg.Insecure {
			fmt.Fprintf(stderr, "%s: WARNING: --insecure: the certificate of %s is NOT verified; dev only, the capture and the report are marked\n", tool, cfg.Broker)
		}
		rep.Mode, rep.Broker, rep.Insecure, rep.Capture = "live", cfg.Broker, cfg.Insecure, f.record
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		var code int
		res, code = live(ctx, v, cfg, opt, f, rep, stderr)
		if code != 0 {
			return code
		}
	}
	rep.Observed = rep.Ended.Sub(rep.Started).Round(time.Millisecond).String()
	rep.Messages, rep.Namespace = res.Messages, res.Namespace
	obs, parts := res.Observations, res.Participants
	if f.app != "" {
		obs, parts = forApp(f.app, obs, parts)
	}
	rep.Participants = parts
	if rep.Participants == nil {
		rep.Participants = []monitor.Participant{}
	}
	reqs, err := verdict.LoadRequirements()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	tests, err := monitor.Tests()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	if f.app != "" && len(parts) == 0 {
		obs = notFound(tests, f.app)
	}
	rep.Tests = verdict.Build(tests, obs, reqs, f.level)
	rep.Summary = verdict.Summary(rep.Tests)
	rep.Exit = verdict.Exit(rep.Tests, f.allowSkip)

	write := func(format string, w io.Writer) error {
		switch format {
		case "json":
			return rep.JSON(w)
		case "junit":
			return rep.JUnit(w)
		}
		return rep.Text(w)
	}
	for _, r := range f.reports {
		format, path, _ := strings.Cut(r, "=")
		var buf bytes.Buffer
		if err := write(format, &buf); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", tool, err)
			return exitInternal
		}
		if err := os.WriteFile(filepath.Clean(path), buf.Bytes(), 0o644); err != nil {
			fmt.Fprintf(stderr, "%s: report: %v\n", tool, err)
			return exitInternal
		}
	}
	if err := write(f.format, stdout); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	return rep.Exit
}

// live observes the broker until the duration ends, the run is interrupted
// or the connection is lost. A connection that cannot be set up is exit 3
// (DESIGN.md §5.3).
func live(ctx context.Context, v *validate.Validator, cfg mqttwire.Config, opt monitor.Options, f *monitorFlags, rep *report.Monitor, stderr io.Writer) (monitor.Result, int) {
	var rec *capture.Writer
	var recFile *os.File
	started := time.Now().UTC()
	if f.record != "" {
		var err error
		if recFile, err = os.Create(f.record); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", tool, err)
			return monitor.Result{}, exitUsage
		}
		defer recFile.Close()
		h := capture.Header{Tool: tool, ToolVersion: version, Schemas: rep.Schemas, Baseline: rep.Baseline, Broker: cfg.Broker,
			Insecure: cfg.Insecure, Subscription: subscription, SubscribeQoS: opt.SubscribeQoS, MaxPacketSize: opt.MaxPacketSize, Started: started}
		if rec, err = capture.NewWriter(recFile, h); err != nil {
			fmt.Fprintf(stderr, "%s: record: %v\n", tool, err)
			return monitor.Result{}, exitInternal
		}
	}
	dctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	cl, err := mqttwire.Dial(dctx, cfg)
	cancel()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return monitor.Result{}, exitInternal
	}
	defer cl.Close()
	msgs := make(chan mqttwire.Message, 4096)
	e := monitor.New(v, opt, started)
	if err := cl.Subscribe(ctx, subscription, byte(opt.SubscribeQoS), func(m mqttwire.Message) { msgs <- m }); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return monitor.Result{}, exitInternal
	}
	fmt.Fprintf(stderr, "%s: monitoring %s on %s for %s\n", tool, subscription, cfg.Broker, f.duration)
	timer := time.NewTimer(f.duration)
	defer timer.Stop()
	var cause error
	last, line := started, 1
	observe := func(m mqttwire.Message) error {
		if m.Received.Before(last) {
			m.Received = last
		}
		last = m.Received.UTC()
		r := capture.Record{Topic: m.Topic, Payload: m.Payload, Size: len(m.Payload), Received: last, QoS: m.QoS, Retain: m.Retain}
		if rec != nil {
			if err := rec.Write(r); err != nil {
				return fmt.Errorf("record: %w", err)
			}
			line++
			r.Line = line
		}
		return e.Observe(monitor.FromRecord(r))
	}
loop:
	for {
		select {
		case m := <-msgs:
			if err := observe(m); err != nil {
				fmt.Fprintf(stderr, "%s: %v\n", tool, err)
				return monitor.Result{}, exitInternal
			}
		case <-timer.C:
			break loop
		case <-ctx.Done():
			cause = errors.New("interrupted")
			break loop
		case err := <-cl.Lost():
			cause = fmt.Errorf("connection lost: %v", err)
			break loop
		}
	}
	for drained := false; !drained; {
		select {
		case m := <-msgs:
			if err := observe(m); err != nil {
				fmt.Fprintf(stderr, "%s: %v\n", tool, err)
				return monitor.Result{}, exitInternal
			}
		default:
			drained = true
		}
	}
	ended := time.Now().UTC()
	if ended.Before(last) {
		ended = last
	}
	if rec != nil {
		if err := rec.Close(ended, cause); err != nil {
			fmt.Fprintf(stderr, "%s: record: %v\n", tool, err)
			return monitor.Result{}, exitInternal
		}
	}
	if cause != nil {
		rep.Error = cause.Error()
		fmt.Fprintf(stderr, "%s: observation stopped early: %v\n", tool, cause)
	}
	rep.Started, rep.Ended = started, ended
	return e.Finish(ended), 0
}

// forApp keeps the participants and observations of the component under
// test (DESIGN.md §3.2 --app): its AppId or its <ServiceType>/<AppId>.
func forApp(app string, obs []verdict.Observation, parts []monitor.Participant) ([]verdict.Observation, []monitor.Participant) {
	ids := map[string]bool{}
	var keep []monitor.Participant
	for _, p := range parts {
		if p.AppID == app || p.ID == app {
			ids[p.ID] = true
			keep = append(keep, p)
		}
	}
	var out []verdict.Observation
	for _, o := range obs {
		if ids[o.Participant] {
			out = append(out, o)
		}
	}
	return out, keep
}

// notFound makes every requirement of the implemented tests inconclusive:
// the component under test did not appear (DESIGN.md §5.3, exit 4).
func notFound(tests []verdict.Test, app string) []verdict.Observation {
	var out []verdict.Observation
	for _, t := range tests {
		for _, r := range t.Reqs {
			out = append(out, verdict.Observation{Test: t.ID, Req: r, Kind: verdict.Skipped, Inconclusive: true,
				Evidence: verdict.Evidence{Detail: fmt.Sprintf("the component under test %q was not observed", app)}})
		}
	}
	return out
}

// captureMessages converts a capture for validate: one message per record.
func captureMessages(name string, data []byte, tolerant bool) ([]source, error) {
	c, err := capture.Read(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if len(c.Records) == 0 {
		return nil, fmt.Errorf("%s: no message", name)
	}
	var out []source
	for _, r := range c.Records {
		p := r.Payload
		if p == nil {
			p = []byte{}
		}
		out = append(out, source{name: fmt.Sprintf("%s:%d", name, r.Line), msg: validate.Message{Topic: r.Topic, Payload: json.RawMessage(p), Tolerant: tolerant}})
	}
	return out, nil
}
