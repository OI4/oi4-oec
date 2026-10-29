// Package bus implements the broker tests of oec-infra bus (DESIGN.md §9,
// catalog d8.yaml, mode bus): T8-BUS-01 protocol, TLS, certificate and
// MaxPacketSize of the Message Bus, T8-BUS-02 the ACL against a second,
// unprivileged client and T8-BUS-03 the OEC Registry. The tests connect to
// the broker like an OEC application (mqttwire) and add raw TLS and TCP
// probes; they publish only on the topics of their own probe AppId.
package bus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path"
	"time"

	"github.com/OI4/oec/conformance/catalog"
	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/report"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// The tests of the package.
const (
	TProtocol = "T8-BUS-01"
	TACL      = "T8-BUS-02"
	TRegistry = "T8-BUS-03"
)

// Implemented lists the tests this package implements; the catalog status
// must agree (bus_test.go).
var Implemented = map[string]bool{TProtocol: true, TACL: true, TRegistry: true}

// Defaults and limits.
const (
	MinMaxPacketSize    = 262144           // RIG 4.1.1: MaxPacketSize is at least 256 KiB
	DefaultMaxProbeSize = 64 << 20         // a larger MaxPacketSize is not probed
	DefaultTimeout      = 5 * time.Second  // one exchange
	DefaultRegistryWait = 65 * time.Second // a heartbeat of the Registry (60 s, MG 7.5.1, SF-106) and some slack
	ServiceType         = "Utility"        // of the probe components (MG 4.2: a test tool)
	topicMax            = 65535            // MQTT: the longest topic name
	remainingMax        = 268435455        // MQTT: the largest remaining length
	publishOverhead     = 4                // topic length and packet identifier of a QoS 1 PUBLISH
	probePrefix         = "oi4.org/oec-infra/D8/"
)

// Config of a run.
type Config struct {
	Main          mqttwire.Config  // the privileged client: D8 itself
	Second        *mqttwire.Config // the unprivileged client of --credentials-2; nil: T8-BUS-02 is inconclusive
	MaxPacketSize int              // broker.json or --max-packet-size; 0: the minimum of RIG 4.1.1 (note)
	MaxProbeSize  int              // a larger MaxPacketSize is not probed (inconclusive); 0: DefaultMaxProbeSize
	PlainPorts    []int            // ports of the broker host probed for a plain-text MQTT listener
	Registry      string           // Identification Link of the Registry; "": discovered on Oi4v2/Registry/+/Pub/#
	RegistryWait  time.Duration    // discovery; 0: DefaultRegistryWait
	Timeout       time.Duration    // one exchange; 0: DefaultTimeout
	Tests         []string         // the selected tests (patterns); empty: every test
}

// Runs reports whether the configuration selects the test.
func (c Config) Runs(id string) bool {
	if len(c.Tests) == 0 {
		return true
	}
	for _, p := range c.Tests {
		if ok, _ := path.Match(p, id); ok {
			return true
		}
	}
	return false
}

// Tests returns the bus tests of the D8 catalog for verdict.Build.
func Tests(cat *catalog.Catalog) []verdict.Test {
	var out []verdict.Test
	for _, t := range cat.Tests {
		if t.Mode == "bus" {
			out = append(out, verdict.Test{ID: t.ID, Title: t.Title, Status: t.Status, Level: t.Level, Reqs: t.Reqs})
		}
	}
	return out
}

// Result of a run.
type Result struct {
	Broker       report.Broker
	Observations []verdict.Observation
}

// ProbeIL returns the Identification Link of a probe component: an
// Oi4Identifier of D8 with a random serial number.
func ProbeIL() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return probePrefix + hex.EncodeToString(b[:])
}

const (
	reqMQTT311    = "REQ-MQTT-002"
	reqTLSOnly    = "REQ-MQTT-006"
	reqCert       = "REQ-MQTT-007"
	reqPacket     = "REQ-CNT-003"
	reqACL        = "REQ-GEN-001"
	reqSetConfig  = "REQ-RES-CONFIG-020"
	reqReadConfig = "REQ-RES-CONFIG-026"
	reqSetData    = "REQ-RES-DATA-010"
	reqSetMeta    = "REQ-RES-META-005"
	reqSetPubList = "REQ-RES-PUBLIST-023"
	reqRefDes     = "REQ-RES-REFDES-008"
	reqSubList    = "REQ-RES-SUBLIST-012"
	reqCollect    = "REQ-REG-001"
	reqAnswer     = "REQ-REG-002"
)

// Requirements of the tests (catalog d8.yaml).
var (
	protocolReqs = []string{reqMQTT311, reqTLSOnly, reqCert, reqPacket}
	aclReqs      = []string{reqACL, reqSetConfig, reqReadConfig, reqSetData, reqSetMeta, reqSetPubList, reqRefDes, reqSubList}
	registryReqs = []string{reqCollect, reqAnswer}
)

type runner struct {
	c     Config
	probe string // the probe AppId of the run
	obs   []verdict.Observation
	info  report.Broker
}

func (r *runner) add(test, req string, k verdict.Kind, topic, format string, a ...any) {
	r.obs = append(r.obs, verdict.Observation{Test: test, Req: req, Kind: k, Evidence: verdict.Evidence{Topic: topic, Detail: fmt.Sprintf(format, a...)}})
}

func (r *runner) checked(test, req, format string, a ...any) {
	r.add(test, req, verdict.Checked, "", format, a...)
}

func (r *runner) violation(test, req, topic, format string, a ...any) {
	r.add(test, req, verdict.Violation, topic, format, a...)
}

func (r *runner) warning(test, req, topic, format string, a ...any) {
	r.add(test, req, verdict.Warning, topic, format, a...)
}

func (r *runner) note(test, req, format string, a ...any) {
	r.add(test, req, verdict.Note, "", format, a...)
}

// inconclusive marks requirements the run could not judge.
func (r *runner) inconclusive(test string, reqs []string, format string, a ...any) {
	for _, req := range reqs {
		r.obs = append(r.obs, verdict.Observation{Test: test, Req: req, Kind: verdict.Skipped, Inconclusive: true, Evidence: verdict.Evidence{Detail: fmt.Sprintf(format, a...)}})
	}
}

// Run executes the selected tests against the broker of c.Main.
func Run(ctx context.Context, c Config) Result {
	if c.MaxProbeSize == 0 {
		c.MaxProbeSize = DefaultMaxProbeSize
	}
	if c.RegistryWait == 0 {
		c.RegistryWait = DefaultRegistryWait
	}
	if c.Timeout == 0 {
		c.Timeout = DefaultTimeout
	}
	r := &runner{c: c, probe: ProbeIL()}
	r.info.URL, r.info.Probe = c.Main.Broker, r.probe
	all := []struct {
		id   string
		reqs []string
	}{{TProtocol, protocolReqs}, {TACL, aclReqs}, {TRegistry, registryReqs}}
	host, addr, err := mqttwire.BrokerAddress(c.Main.Broker)
	if err != nil {
		for _, t := range all {
			if c.Runs(t.id) {
				r.inconclusive(t.id, t.reqs, "%v", err)
			}
		}
		return r.result()
	}
	if c.Runs(TProtocol) {
		r.transport(ctx, host, addr)
	}
	main, err := r.dial(ctx, c.Main, "oec-infra-", nil)
	if c.Runs(TProtocol) {
		r.protocol(err)
	}
	if err != nil {
		reason := "D8 cannot connect to the broker: " + err.Error()
		for _, t := range all {
			reqs := t.reqs
			if t.id == TProtocol {
				reqs = []string{reqPacket}
			}
			if c.Runs(t.id) {
				r.inconclusive(t.id, reqs, "%s", reason)
			}
		}
		return r.result()
	}
	defer main.Close()
	if c.Runs(TProtocol) {
		r.packet(ctx)
	}
	if c.Runs(TACL) {
		r.acl(ctx, main)
	}
	if c.Runs(TRegistry) {
		r.registry(ctx, main)
	}
	return r.result()
}

func (r *runner) result() Result {
	return Result{Broker: r.info, Observations: r.obs}
}

func (r *runner) dial(ctx context.Context, c mqttwire.Config, prefix string, will *mqttwire.Message) (*mqttwire.Client, error) {
	if c.ClientID == "" {
		c.ClientID = mqttwire.ClientID(prefix)
	}
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = r.c.Timeout
	}
	c.Will = will
	return mqttwire.Dial(ctx, c)
}
