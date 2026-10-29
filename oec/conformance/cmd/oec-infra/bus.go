package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/OI4/oec/conformance/internal/bus"
	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/report"
	"github.com/OI4/oec/conformance/internal/verdict"
	schema "github.com/OI4/oec/schema"
)

// busLevel is the level the bus tests are judged at: the broker has no
// declared level of its own (DESIGN.md §9).
const busLevel = 3

// list is a repeatable, comma separated flag.
type list []string

func (l *list) String() string { return strings.Join(*l, ",") }

func (l *list) Set(v string) error {
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			*l = append(*l, s)
		}
	}
	return nil
}

// ports is the repeatable flag --plain-port.
type ports []int

func (p *ports) String() string {
	s := make([]string, len(*p))
	for i, n := range *p {
		s[i] = strconv.Itoa(n)
	}
	return strings.Join(s, ",")
}

func (p *ports) Set(v string) error {
	for _, s := range strings.Split(v, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || n < 0 || n > 65535 {
			return fmt.Errorf("not a port: %q", s)
		}
		if n > 0 {
			*p = append(*p, n)
		}
	}
	return nil
}

type busFlags struct {
	fl                                             *flag.FlagSet
	config                                         string
	broker, busStorage, ca, cert, key, keyPassFile string
	username, passwordFile                         string
	insecure                                       bool
	credentials2, busStorage2                      string
	plainPorts                                     ports
	maxPacketSize, maxProbeSize                    int
	registry                                       string
	registryWait, timeout                          time.Duration
	tests                                          list
	format                                         string
	reports                                        reports
	allowSkip                                      bool
}

func newBusFlags(stderr io.Writer) *busFlags {
	f := &busFlags{fl: flag.NewFlagSet("bus", flag.ContinueOnError)}
	fl := f.fl
	fl.SetOutput(stderr)
	fl.Usage = func() { fmt.Fprint(stderr, usage) }
	fl.StringVar(&f.config, "config", "", "YAML file with flag values (keys = flag names); flags override it")
	fl.StringVar(&f.broker, "broker", "", "broker URL mqtts://host[:port]")
	fl.StringVar(&f.busStorage, "bus-storage", "", "container storage root of a privileged client (RIG 4.1: etc/oi4/mqtt/broker.json, certificates, run/secrets/oi4)")
	fl.StringVar(&f.ca, "ca", "", "PEM CA bundle that verifies the broker (default: system roots)")
	fl.StringVar(&f.cert, "cert", "", "client certificate (PEM)")
	fl.StringVar(&f.key, "key", "", "private key of the client certificate (PEM, also encrypted PKCS#8)")
	fl.StringVar(&f.keyPassFile, "key-passphrase-file", "", "file with the passphrase of an encrypted key")
	fl.StringVar(&f.username, "username", "", "MQTT user name")
	fl.StringVar(&f.passwordFile, "password-file", "", "file with the MQTT password (no password on the command line)")
	fl.BoolVar(&f.insecure, "insecure", false, "do not verify the broker certificate when connecting (dev only; T8-BUS-01 still judges it)")
	fl.StringVar(&f.credentials2, "credentials-2", "", "credentials file (base64 of <username>:<password>, RIG 4.1.3) of an unprivileged client for the ACL probe")
	fl.StringVar(&f.busStorage2, "bus-storage-2", "", "container storage root of an unprivileged client for the ACL probe")
	fl.Var(&f.plainPorts, "plain-port", "port of the broker host probed for a plain-text MQTT listener; repeatable (default 1883; 0: none)")
	fl.IntVar(&f.maxPacketSize, "max-packet-size", 0, "MaxPacketSize of the Message Bus (default: broker.json of --bus-storage, else 262144)")
	fl.IntVar(&f.maxProbeSize, "max-probe-size", bus.DefaultMaxProbeSize, "a larger MaxPacketSize is not probed (inconclusive)")
	fl.StringVar(&f.registry, "registry", "", "Identification Link of the OEC Registry (default: discovered on Oi4v2/Registry/+/Pub/#)")
	fl.DurationVar(&f.registryWait, "registry-wait", bus.DefaultRegistryWait, "discovery time of the OEC Registry")
	fl.DurationVar(&f.timeout, "timeout", bus.DefaultTimeout, "waiting time for one exchange with the broker or the Registry")
	fl.Var(&f.tests, "tests", "test ids or patterns (path.Match) to run, comma separated; repeatable (default: all)")
	fl.StringVar(&f.format, "format", "text", "output on standard output: text, json, junit")
	fl.Var(&f.reports, "report", "write a report file FORMAT=PATH (repeatable)")
	fl.BoolVar(&f.allowSkip, "allow-skip", false, "exit 0 instead of 4 if a mandatory requirement could not be judged")
	return f
}

// applyConfig sets the flags that the command line did not set from the
// YAML file (as oec-check, DESIGN.md §8). Unknown keys are usage errors.
func (f *busFlags) applyConfig() error {
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
		if k == "config" || f.fl.Lookup(k) == nil {
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

// connection builds the configuration of the privileged client: the
// container storage first, the explicit flags override it (as oec-check).
func (f *busFlags) connection() (mqttwire.Config, int, error) {
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
		return c, 0, errors.New("bus needs --broker or --bus-storage")
	}
	if _, _, err := mqttwire.BrokerAddress(c.Broker); err != nil {
		return c, 0, err
	}
	c.Insecure = f.insecure
	return c, mps, nil
}

// second builds the configuration of the unprivileged client: the broker,
// its CA and --insecure of the privileged client, the identity of
// --credentials-2 or --bus-storage-2.
func (f *busFlags) second(main mqttwire.Config) (*mqttwire.Config, error) {
	switch {
	case f.credentials2 != "" && f.busStorage2 != "":
		return nil, errors.New("--credentials-2 and --bus-storage-2 exclude each other")
	case f.credentials2 != "":
		raw, err := os.ReadFile(f.credentials2)
		if err != nil {
			return nil, err
		}
		dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
		if err != nil {
			return nil, fmt.Errorf("--credentials-2: %w", err)
		}
		user, pass, ok := strings.Cut(string(dec), ":")
		if !ok || user == "" {
			return nil, errors.New("--credentials-2: not the base64 of <username>:<password>")
		}
		return &mqttwire.Config{Broker: main.Broker, CAFile: main.CAFile, Insecure: main.Insecure, Username: user, Password: []byte(pass)}, nil
	case f.busStorage2 != "":
		s, err := mqttwire.ReadStorage(f.busStorage2)
		if err != nil {
			return nil, fmt.Errorf("--bus-storage-2: %w", err)
		}
		c := s.Config
		c.Broker, c.CAFile, c.Insecure = main.Broker, main.CAFile, main.Insecure
		return &c, nil
	}
	return nil, nil
}

func runBus(args []string, stdout, stderr io.Writer) int {
	usageErr := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "%s: %s\n", tool, fmt.Sprintf(format, a...))
		return exitUsage
	}
	f := newBusFlags(stderr)
	if err := f.fl.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitPass
		}
		return exitUsage
	}
	if f.fl.NArg() != 0 {
		return usageErr("bus takes no arguments")
	}
	if err := f.applyConfig(); err != nil {
		return usageErr("%v", err)
	}
	if !validFormat(f.format) {
		return usageErr("unknown --format %q (text, json, junit)", f.format)
	}
	if f.timeout <= 0 || f.registryWait <= 0 || f.maxProbeSize <= 0 || f.maxPacketSize < 0 {
		return usageErr("--timeout, --registry-wait and --max-probe-size must be positive, --max-packet-size not negative")
	}
	main, mps, err := f.connection()
	if err != nil {
		return usageErr("%v", err)
	}
	second, err := f.second(main)
	if err != nil {
		return usageErr("%v", err)
	}
	if f.maxPacketSize > 0 {
		mps = f.maxPacketSize
	}
	plain := []int(f.plainPorts)
	if !isSet(f.fl, "plain-port") {
		plain = []int{1883}
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
	for _, t := range bus.Tests(cat) {
		if (bus.Config{Tests: f.tests}).Runs(t.ID) {
			tests = append(tests, t)
		}
	}
	if len(tests) == 0 {
		return usageErr("--tests selects no bus test")
	}
	if main.Insecure {
		fmt.Fprintf(stderr, "%s: WARNING: --insecure: D8 does not verify the broker certificate when connecting (dev only)\n", tool)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	started := time.Now().UTC()
	res := bus.Run(ctx, bus.Config{Main: main, Second: second, MaxPacketSize: mps, MaxProbeSize: f.maxProbeSize,
		PlainPorts: plain, Registry: f.registry, RegistryWait: f.registryWait, Timeout: f.timeout, Tests: f.tests})
	br := res.Broker
	rep := &report.Infra{Report: report.InfraVersion, Tool: tool, Version: version, Schemas: schema.Version, Baseline: base,
		Catalog: cat.Baseline, Command: append([]string{tool, "bus"}, args...), Mode: "bus", Input: main.Broker,
		Level: busLevel, Started: started, Broker: &br}
	rep.Tests = verdict.Build(tests, res.Observations, reqs, busLevel)
	rep.Summary = verdict.Summary(rep.Tests)
	rep.Exit = verdict.Exit(rep.Tests, f.allowSkip)
	rep.Ended = time.Now().UTC()
	return writeReports(rep, f.format, f.reports, stdout, stderr)
}

func isSet(fl *flag.FlagSet, name string) bool {
	set := false
	fl.Visit(func(f *flag.Flag) { set = set || f.Name == name })
	return set
}
