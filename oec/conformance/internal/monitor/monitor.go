// Package monitor is the engine of oec-check monitor (DESIGN.md §3.2): it
// runs every observed message through the validate layers and keeps the
// state of each participant for the stateful d7-monitor rules. The same
// engine serves the live run and the replay of a capture; it depends only
// on the order and the receive times of the messages, so a replay is
// deterministic.
package monitor

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/OI4/oec/conformance/catalog"
	"github.com/OI4/oec/conformance/internal/capture"
	"github.com/OI4/oec/conformance/internal/topic"
	"github.com/OI4/oec/conformance/internal/validate"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// Implemented lists the catalog tests the engine judges (d7.yaml status
// implemented).
var Implemented = []string{
	"T7-MON-01", "T7-MON-02", "T7-MON-03", "T7-MON-04", "T7-MON-05", "T7-MON-06",
	"T7-MON-07", "T7-MON-08", "T7-MON-09", "T7-MON-10", "T7-MON-11", "T7-MON-12", "T7-MON-13",
	"T7-MON-14", "T7-MON-15", "T7-MON-16", "T7-MON-17", "T7-MON-18", "T7-MON-19",
}

// Tests returns the monitor tests of the D7 catalog for verdict.Build.
func Tests() ([]verdict.Test, error) {
	c, err := catalog.Load(catalog.D7File)
	if err != nil {
		return nil, err
	}
	var out []verdict.Test
	for _, t := range c.Tests {
		if t.Mode == "monitor" {
			out = append(out, verdict.Test{ID: t.ID, Title: t.Title, Status: t.Status, Level: t.Level, Reqs: t.Reqs})
		}
	}
	return out, nil
}

// Options are the parameters of a run.
type Options struct {
	Heartbeat     time.Duration // Health heartbeat (REQ-TOP-017): 60 s
	Tolerance     time.Duration // tolerance of time-based rules: 5 s
	MaxPacketSize int           // MaxPacketSize of the Message Bus, 0 if unknown
	SubscribeQoS  int           // QoS of D7's subscription; the delivered QoS shows the publisher's QoS from 1 on
}

// DefaultOptions are the defaults of DESIGN.md §3.2.
func DefaultOptions() Options {
	return Options{Heartbeat: 60 * time.Second, Tolerance: 5 * time.Second, SubscribeQoS: 1}
}

// MinDuration is the observation time the heartbeat rule needs
// (2 × heartbeat + tolerance).
func (o Options) MinDuration() time.Duration { return 2*o.Heartbeat + o.Tolerance }

// ForCapture completes the options from the capture header: the QoS of the
// recording subscription and, unless set, the MaxPacketSize.
func (o Options) ForCapture(h capture.Header) Options {
	o.SubscribeQoS = h.SubscribeQoS
	if o.MaxPacketSize <= 0 {
		o.MaxPacketSize = h.MaxPacketSize
	}
	return o
}

// Replay runs a capture through a new engine (monitor --replay). The
// options are completed from the header (ForCapture).
func Replay(v *validate.Validator, c *capture.Capture, opt Options) (Result, error) {
	e := New(v, opt.ForCapture(c.Header), c.Header.Started)
	for _, r := range c.Records {
		if err := e.Observe(FromRecord(r)); err != nil {
			return Result{}, err
		}
	}
	return e.Finish(c.End()), nil
}

// FromRecord converts a capture record.
func FromRecord(r capture.Record) Message {
	return Message{Line: r.Line, Topic: r.Topic, Payload: r.Payload, Size: r.Size, Received: r.Received, QoS: r.QoS, Retain: r.Retain}
}

// Message is one observed MQTT message.
type Message struct {
	Line     int // line in the capture, 0 in a live run without recording
	Topic    string
	Payload  []byte
	Size     int // byte length as received
	Received time.Time
	QoS      byte
	Retain   bool
}

// Participant summarizes one component seen on the bus.
type Participant struct {
	ID          string    `json:"id"` // <ServiceType>/<Identification Link>
	ServiceType string    `json:"service_type"`
	AppID       string    `json:"app_id"` // Identification Link (unencoded)
	Messages    int       `json:"messages"`
	Invalid     int       `json:"invalid"`
	Births      int       `json:"births"`
	Wills       int       `json:"wills"`
	Closes      int       `json:"closes"`
	Sources     []string  `json:"sources,omitempty"` // subordinate assets (Sources other than the AppId)
	First       time.Time `json:"first"`
	Last        time.Time `json:"last"`
}

// Result of a run.
type Result struct {
	Observations []verdict.Observation
	Participants []Participant
	Messages     int // all observed messages
	Namespace    int // messages in the Oi4v2 namespace
}

type key struct{ resource, name, group string }

type source struct {
	line        int
	mam, health bool
}

type pending struct {
	line     int
	deadline time.Time
}

type metaRef struct {
	line  int
	topic string
	id    int64
	k     key
}

type entryRef struct {
	line  int
	topic string
	id    int64
	k     key
}

type part struct {
	Participant
	seen       bool
	state      string // "", "connected", "will", "close"
	endLine    int
	birthTS    int64             // MessageId timestamp of the current Birth
	late       *verdict.Evidence // first message after a Will without a Birth before it
	recovering bool              // a Health FAILURE_1/0 about itself while connected
	birthID    int64
	birthMAM   any
	birthRev   string
	lastHealth time.Time
	healthLine int
	healthSelf bool
	sources    map[string]*source
	k2id       map[key]int64
	id2k       map[int64]key
	ids        map[key]map[int64]bool // all ids per key over the run
	metas      []metaRef
	entries    map[key]entryRef
	pending    map[int64]pending
	setSeen    bool
	// kept across reconnects (endConnection does not reset them)
	classes     map[string]classRef // T7-MON-09: structure per DataSetClassId
	dataSets    map[key]classRef    // T7-MON-09: DataSetClassId per Data DataSet
	data, event bool                // T7-MON-13: Pub Data / Pub Event observed
	profile     []string            // T7-MON-13: Resources of the last Profile about itself
	profileSeen bool
	revs        map[string]revRef     // T7-MON-16: last RevisionCounter per Source
	configs     map[string]*configRef // T7-MON-16/17: last Pub Config per Source and WriterGroupName
	changes     []*change             // T7-MON-15/16: Set and Del addressed to the component
	pl          publist               // T7-MON-14: rules of the PublicationList
}

type classRef struct {
	class, structure string
	line             int
}

type revRef struct {
	rev  int64
	line int
}

type configRef struct {
	groups map[string]map[string]any
	line   int
}

// change is a Set or Del addressed to a component that changes its
// PublicationList or SubscriptionList (T7-MON-15) or a value of its Config
// (T7-MON-16), identified by the MessageId of the request.
type change struct {
	id, method, resource string
	line                 int
	source               string // Config: the Source whose value changes
	prevRev              int64  // Config: RevisionCounter before the change
	status               string // Filter of the status event, e.g. Status/Good
	goodAt               time.Time
	republished          bool // list: published again after the request
	resolved             bool // Config: judged with a later Pub MAM
}

func (c *change) good() bool { return c.status == "Status/Good" }

type chain struct {
	sender, topic  string
	total, perPage int64
	pages          map[int64]int
	last           int64
	lastNext       bool
	lastAt         time.Time
	lastLine       int
	single         bool // the answer to a Get for one page (REQ-PAG-002)
}

// Engine is the monitor engine. It is not safe for concurrent use.
type Engine struct {
	v        *validate.Validator
	opt      Options
	start    time.Time
	parts    map[string]*part
	obs      []verdict.Observation
	checked  map[string]bool
	msgIDs   map[string]int
	chains   map[string]*chain
	mams     map[string]mamRef
	appTypes map[string]map[string]bool
	locales  map[string]bool // MessageIds of the Gets with a Locale (REQ-LOC-002)
	pages    map[string]bool // MessageIds of the Gets for one page (REQ-PAG-002)
	messages int
	ns       int
}

type mamRef struct {
	line     int
	identity string
}

// New returns an engine for an observation that started at start.
func New(v *validate.Validator, opt Options, start time.Time) *Engine {
	return &Engine{v: v, opt: opt, start: start, parts: map[string]*part{}, checked: map[string]bool{},
		msgIDs: map[string]int{}, chains: map[string]*chain{}, mams: map[string]mamRef{}, appTypes: map[string]map[string]bool{},
		locales: map[string]bool{}, pages: map[string]bool{}}
}

var messageTS = regexp.MustCompile(`^([0-9]+)-`)

func ts(id string) int64 {
	g := messageTS.FindStringSubmatch(id)
	if g == nil {
		return -1
	}
	n, err := strconv.ParseInt(g[1], 10, 64)
	if err != nil {
		return -1
	}
	return n
}

func (e *Engine) part(id string) *part {
	p, ok := e.parts[id]
	if !ok {
		st, il, _ := strings.Cut(id, "/")
		p = &part{Participant: Participant{ID: id, ServiceType: st, AppID: il}, sources: map[string]*source{},
			k2id: map[key]int64{}, id2k: map[int64]key{}, ids: map[key]map[int64]bool{}, entries: map[key]entryRef{}, pending: map[int64]pending{},
			classes: map[string]classRef{}, dataSets: map[key]classRef{}, revs: map[string]revRef{}, configs: map[string]*configRef{}}
		e.parts[id] = p
	}
	return p
}

func ev(p *part, m Message, ptr, format string, a ...any) verdict.Evidence {
	id := ""
	if p != nil {
		id = p.ID
	}
	return verdict.Evidence{Participant: id, Line: m.Line, Topic: m.Topic, Pointer: ptr, Detail: fmt.Sprintf(format, a...)}
}

func (e *Engine) check(p *part, test, req string) {
	id := ""
	if p != nil {
		id = p.ID
	}
	k := test + "|" + req + "|" + id
	if !e.checked[k] {
		e.checked[k] = true
		e.obs = append(e.obs, verdict.Observation{Test: test, Req: req, Kind: verdict.Checked, Evidence: verdict.Evidence{Participant: id}})
	}
}

func (e *Engine) add(test, req string, kind verdict.Kind, x verdict.Evidence) {
	e.obs = append(e.obs, verdict.Observation{Test: test, Req: req, Kind: kind, Evidence: x})
}

func (e *Engine) skip(p *part, test, req string, inconclusive bool, format string, a ...any) {
	e.obs = append(e.obs, verdict.Observation{Test: test, Req: req, Kind: verdict.Skipped, Inconclusive: inconclusive,
		Evidence: verdict.Evidence{Participant: p.ID, Detail: fmt.Sprintf(format, a...)}})
}

type dsm struct {
	ptr         string
	id          int64
	idOK        bool
	name, group string
	payload     map[string]any
}

func str(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func num(m map[string]any, k string) (int64, bool) {
	n, ok := m[k].(json.Number)
	if !ok {
		return 0, false
	}
	i, err := strconv.ParseInt(string(n), 10, 64)
	return i, err == nil
}

func dsms(msg map[string]any) []dsm {
	arr, _ := msg["Messages"].([]any)
	var out []dsm
	for i, v := range arr {
		dm, ok := v.(map[string]any)
		if !ok {
			continue
		}
		d := dsm{ptr: fmt.Sprintf("/Messages/%d", i), name: str(dm, "DataSetWriterName"), group: str(dm, "WriterGroupName")}
		d.id, d.idOK = num(dm, "DataSetWriterId")
		d.payload, _ = dm["Payload"].(map[string]any)
		out = append(out, d)
	}
	return out
}

// helper reports a helper DataSet (PaginationRequest, Pagination, Locale,
// Query) or a reserved DataSetWriterId (MG 8.2.3).
func (d dsm) helper() bool { return d.id >= 0 && d.id <= 9 }

// Observe judges one message.
func (e *Engine) Observe(m Message) error {
	e.messages++
	if m.Topic != topic.Namespace && !strings.HasPrefix(m.Topic, topic.Namespace+"/") {
		return nil
	}
	e.ns++
	payload := m.Payload
	if payload == nil {
		payload = []byte{}
	}
	res, err := e.v.Validate(validate.Message{Topic: m.Topic, Payload: json.RawMessage(payload)})
	if err != nil {
		return fmt.Errorf("line %d: %w", m.Line, err)
	}
	var msg map[string]any
	if inst, err := validate.Parse(payload); err == nil {
		msg, _ = inst.(map[string]any)
	}
	t := res.Topic
	p := e.sender(t, msg, m.Topic)
	if p != nil {
		if !p.seen {
			p.seen, p.First = true, m.Received
		}
		p.Messages++
		p.Last = m.Received
		if !res.Valid {
			p.Invalid++
		}
	}
	e.valid(p, m, res)
	if p == nil || t == nil || msg == nil {
		return nil
	}
	e.client(p, m)
	e.packet(p, m)
	e.unique(p, m, t, msg)
	switch t.Method {
	case "Pub":
		e.pub(p, m, t, msg)
	case "Set", "Del":
		q := e.part(t.ServiceType + "/" + t.AppIDIL)
		if t.Method == "Set" {
			q.setSeen = true
		}
		e.change(p, q, m, t, msg)
		e.publistChange(q, t)
		e.lifecycle(p, m, t, msg, nil)
	case "Call":
		e.call(p, m, msg)
		e.lifecycle(p, m, t, msg, nil)
	case "Reply":
		e.reply(p, m, t, msg)
		e.lifecycle(p, m, t, msg, nil)
	default:
		if t.Method == "Get" {
			e.request(msg)
		}
		e.lifecycle(p, m, t, msg, nil)
	}
	return nil
}

// request notes the helper DataSets of a Get that change its answer: a
// Locale (DataSetWriterId 3, MG 8.3.12) and a PaginationRequest for one
// page (DataSetWriterId 1 with Page > 0, MG 8.3.11).
func (e *Engine) request(msg map[string]any) {
	id := str(msg, "MessageId")
	for _, d := range dsms(msg) {
		switch {
		case !d.idOK:
		case d.id == 3:
			e.locales[id] = true
		case d.id == 1:
			if page, ok := num(d.payload, "Page"); ok && page > 0 {
				e.pages[id] = true
			}
		}
	}
}

// sender attributes a message to the component that sent it: the topic
// names the publisher for Pub and the Responder for Reply; a Call names
// the Requestor in RequestorId; Get, Set and Del name the sender in
// PublisherId.
func (e *Engine) sender(t *topic.Parsed, msg map[string]any, raw string) *part {
	id := ""
	if t != nil {
		switch t.Method {
		case "Pub", "Reply":
			id = t.ServiceType + "/" + t.AppIDIL
		case "Call":
			id = str(msg, "RequestorId")
		default:
			id = str(msg, "PublisherId")
		}
	}
	if id == "" {
		id = str(msg, "PublisherId")
	}
	if st, il, ok := strings.Cut(id, "/"); !ok || st == "" || il == "" {
		return nil
	}
	return e.part(id)
}

// valid is T7-MON-01: every message of the namespace passes the four
// layers of validate (REQ-TOP-002); traffic that does not even follow the
// topic grammar violates REQ-TOP-003.
func (e *Engine) valid(p *part, m Message, res validate.Result) {
	const test = "T7-MON-01"
	e.check(p, test, "REQ-TOP-002")
	e.check(p, test, "REQ-TOP-003")
	if res.Topic == nil {
		e.add(test, "REQ-TOP-003", verdict.Violation, ev(p, m, "", "traffic in the Oi4v2 namespace that does not follow the topic grammar"))
	}
	for _, f := range res.Findings {
		req := f.Req
		if req == "" {
			req = "REQ-TOP-002"
		}
		e.add(test, req, verdict.Violation, ev(p, m, f.Pointer, "[%s] %s", f.Layer, f.Detail))
	}
	for _, w := range res.Warnings {
		req := topic.WarningReqs[w]
		if req == "" {
			req = "REQ-TOP-002"
		}
		e.add(test, req, verdict.Warning, ev(p, m, "", "%s", w))
	}
}

// client is T7-MON-12: the delivered QoS and retain flag (MQTT 3.1.1
// delivers min(publish QoS, subscribe QoS); a retained message is
// delivered with retain set when D7 subscribes).
func (e *Engine) client(p *part, m Message) {
	const test = "T7-MON-12"
	e.check(p, test, "REQ-CMP-009")
	if m.Retain {
		e.add(test, "REQ-CMP-009", verdict.Violation, ev(p, m, "", "the message was retained by the broker: the publisher set the Retain flag (REQ-MQTT-018)"))
	}
	if e.opt.SubscribeQoS < 1 {
		e.skip(p, test, "REQ-MQTT-017", false, "D7 subscribed with QoS 0: the publisher's QoS is not visible")
		return
	}
	e.check(p, test, "REQ-MQTT-017")
	if m.QoS > 0 {
		e.add(test, "REQ-MQTT-017", verdict.Violation, ev(p, m, "", "published with QoS %d, the default is QoS 0", m.QoS))
	}
}

// packet is T7-MON-18 for REQ-PAG-005: no NetworkMessage exceeds the
// MaxPacketSize of the Message Bus.
func (e *Engine) packet(p *part, m Message) {
	const test = "T7-MON-18"
	if e.opt.MaxPacketSize <= 0 {
		e.skip(p, test, "REQ-PAG-005", true, "MaxPacketSize of the Message Bus unknown (--max-packet-size or --bus-storage)")
		return
	}
	e.check(p, test, "REQ-PAG-005")
	if m.Size > e.opt.MaxPacketSize {
		e.add(test, "REQ-PAG-005", verdict.Violation, ev(p, m, "", "the NetworkMessage has %d bytes, the MaxPacketSize is %d", m.Size, e.opt.MaxPacketSize))
	}
}

// unique is T7-MON-06: MessageIds are globally unique.
func (e *Engine) unique(p *part, m Message, t *topic.Parsed, msg map[string]any) {
	const test = "T7-MON-06"
	id := str(msg, "MessageId")
	if id == "" {
		return
	}
	req := "REQ-MSG-008"
	if t.Method == "Call" || t.Method == "Reply" {
		req = "REQ-ACT-004"
	}
	e.check(p, test, req)
	if line, ok := e.msgIDs[id]; ok {
		e.add(test, req, verdict.Violation, ev(p, m, "/MessageId", "MessageId %q was already used (line %d)", id, line))
		return
	}
	e.msgIDs[id] = m.Line
}

// call is T7-MON-07: a Requestor keeps the RequestId unique among its
// pending requests (until the final response or the TimeoutHint).
func (e *Engine) call(p *part, m Message, msg map[string]any) {
	const test, req = "T7-MON-07", "REQ-ACT-011"
	timeout, ok := num(msg, "TimeoutHint")
	if !ok || timeout <= 0 {
		timeout = 0
	}
	for k, id := range p.pending {
		if !m.Received.Before(id.deadline) {
			delete(p.pending, k)
		}
	}
	arr, _ := msg["Messages"].([]any)
	for i, v := range arr {
		am, _ := v.(map[string]any)
		rid, ok := num(am, "RequestId")
		if !ok {
			continue
		}
		e.check(p, test, req)
		if prev, ok := p.pending[rid]; ok {
			e.add(test, req, verdict.Violation, ev(p, m, fmt.Sprintf("/Messages/%d/RequestId", i), "RequestId %d is still pending from line %d", rid, prev.line))
		}
		p.pending[rid] = pending{line: m.Line, deadline: m.Received.Add(time.Duration(timeout) * time.Millisecond)}
	}
}

func (e *Engine) reply(p *part, m Message, t *topic.Parsed, msg map[string]any) {
	e.publisher(p, m, t, msg)
	q := e.parts[str(msg, "RequestorId")]
	if q == nil {
		return
	}
	arr, _ := msg["Messages"].([]any)
	for _, v := range arr {
		am, _ := v.(map[string]any)
		rid, ok := num(am, "RequestId")
		state, _ := num(am, "ActionState")
		if ok && state == 2 {
			delete(q.pending, rid)
		}
	}
}

// publisher is T7-MON-02: the ServiceType/AppId of a Pub or Reply topic are
// those of the component that sends it (its PublisherId).
func (e *Engine) publisher(p *part, m Message, t *topic.Parsed, msg map[string]any) {
	const test = "T7-MON-02"
	pub := str(msg, "PublisherId")
	if pub == "" {
		return
	}
	e.check(p, test, "REQ-TOP-006")
	e.check(p, test, "REQ-TOP-010")
	st, il, _ := strings.Cut(pub, "/")
	if st != t.ServiceType {
		e.add(test, "REQ-TOP-006", verdict.Violation, ev(p, m, "/PublisherId", "the topic carries the ServiceType %s, the publisher is %s", t.ServiceType, pub))
	}
	if il != t.AppIDIL {
		e.add(test, "REQ-TOP-010", verdict.Violation, ev(p, m, "/PublisherId", "the topic carries the AppId %q, the publisher is %s", t.AppIDIL, pub))
	}
	if e.appTypes[il] == nil {
		e.appTypes[il] = map[string]bool{}
	}
	e.appTypes[il][st] = true
}

func (e *Engine) pub(p *part, m Message, t *topic.Parsed, msg map[string]any) {
	e.publisher(p, m, t, msg)
	ds := dsms(msg)
	unsolicited := str(msg, "ReplyTo") == ""
	e.check(p, "T7-MON-04", "REQ-CMP-014")
	birth := e.lifecycle(p, m, t, msg, ds)
	_, meta := msg["MetaData"].(map[string]any)
	e.content(p, m, t, ds, unsolicited, birth, t.Resource == "Metadata" && meta, e.locales[str(msg, "ReplyTo")])
	e.writerIDs(p, m, t, msg, ds)
	e.paginate(p, m, ds, e.pages[str(msg, "ReplyTo")])
	e.identity(p, m, t, ds)
	e.debug(p, m, t, unsolicited)
	e.classIDs(p, m, t, msg, ds)
	e.supported(p, t, ds)
	e.changed(p, m, t, msg, ds)
	e.publistPub(p, m, t, msg, ds, birth)
}

func (e *Engine) resourceDSMs(ds []dsm) []dsm {
	var out []dsm
	for _, d := range ds {
		if !d.idOK || !d.helper() {
			out = append(out, d)
		}
	}
	return out
}

// selfHealth returns the role of a Pub Health about the component itself
// with HealthScore 0: "close" for NORMAL_0, "will" for FAILURE_1.
func selfHealth(p *part, t *topic.Parsed, ds []dsm) (string, dsm) {
	if t.Method != "Pub" || t.Resource != "Health" || len(ds) != 1 || ds[0].name != p.AppID {
		return "", dsm{}
	}
	score, ok := ds[0].payload["HealthScore"].(json.Number)
	if f, err := score.Float64(); !ok || err != nil || f != 0 {
		return "", ds[0]
	}
	switch str(ds[0].payload, "Health") {
	case "NORMAL_0":
		return "close", ds[0]
	case "FAILURE_1":
		return "will", ds[0]
	}
	return "", ds[0]
}

func isBirth(p *part, t *topic.Parsed, msg map[string]any, ds []dsm) bool {
	if t.Method != "Pub" || t.Resource != "MAM" || str(msg, "ReplyTo") != "" {
		return false
	}
	rs := 0
	self := false
	for _, d := range ds {
		if !d.idOK || !d.helper() {
			rs++
			self = self || d.name == p.AppID
		}
	}
	return rs == 1 && self
}

// onChange: while connected, an unsolicited Pub MAM about itself with
// another RevisionCounter than the current MAM is the MAM published on
// change (REQ-RES-MAM-018; MAM is published without request, REQ-RES-PUBLIST),
// not a new Birth. A repeated MAM with the same RevisionCounter stays a
// Birth (T7-MON-10).
func onChange(p *part, ds []dsm) bool {
	if p.state != "connected" || p.birthMAM == nil {
		return false
	}
	for _, d := range ds {
		if d.name == p.AppID {
			return revision(d.payload) != p.birthRev
		}
	}
	return false
}

// lifecycle is T7-MON-03 and the Close part of T7-MON-05: the Birth is the
// first message after every connect; after a Close the component publishes
// nothing but a new Birth (REQ-LIFE-014). A connect is visible after a Will
// or Close of the component, or as the first message of a component that
// was not seen before.
func (e *Engine) lifecycle(p *part, m Message, t *topic.Parsed, msg map[string]any, ds []dsm) bool {
	const test = "T7-MON-03"
	first := p.Messages == 1
	birth := ds != nil && isBirth(p, t, msg, ds) && !onChange(p, ds)
	if birth {
		if p.state == "connected" && p.late == nil && !p.recovering {
			e.add("T7-MON-10", "REQ-IL-012", verdict.Note, ev(p, m, "", "a second Birth of %s without Will or Close in between: a second instance with the same Identification Link, or a reconnect whose Will was not published", p.ID))
		}
		e.birth(p, m, t, msg, ds)
		return true
	}
	switch p.state {
	case "will", "close":
		req, what := "REQ-LIFE-001", "Will"
		test := test
		if p.state == "close" {
			req, what, test = "REQ-LIFE-014", "Close", "T7-MON-05"
		}
		detail := fmt.Sprintf("after its %s message (line %d) the component published %s %s without a Birth", what, p.endLine, t.Method, t.Resource)
		if t.Method == "Pub" && t.Resource == "MAM" {
			e.add(test, "REQ-LIFE-002", verdict.Violation, ev(p, m, "", "%s; a Birth is the Pub MAM about the component itself (Source = AppId)", detail))
		}
		if req == "REQ-LIFE-001" {
			// A late Birth still fulfils REQ-LIFE-001; that it comes first is
			// REQ-LIFE-015 (SHOULD, T7-LIFE-02). The violation is reported when
			// the connection ends or the observation ends without a Birth.
			x := ev(p, m, "", "%s", detail)
			p.late = &x
		} else {
			e.add(test, req, verdict.Violation, ev(p, m, "", "%s", detail))
		}
		p.state = "connected"
	case "":
		if first {
			p.state = "unknown"
		}
	}
	if ds == nil {
		return false
	}
	role, d := selfHealth(p, t, ds)
	switch role {
	case "close":
		e.noBirth(p)
		p.Closes++
		p.state, p.endLine = "close", m.Line
		e.check(p, "T7-MON-05", "REQ-LIFE-014")
		e.endConnection(p)
	case "will":
		at := ts(str(msg, "MessageId"))
		switch {
		case p.birthTS > 0 && at >= 0 && at <= p.birthTS:
			e.noBirth(p)
			p.Wills++
			p.state, p.endLine = "will", m.Line
			e.endConnection(p)
		case p.birthTS > 0:
			p.recovering = true
		case p.late != nil:
			e.noBirth(p)
			p.Wills++
			p.state, p.endLine = "will", m.Line
			e.endConnection(p)
		case p.birthTS == 0:
			e.add(test, "REQ-LIFE-001", verdict.Note, ev(p, m, d.ptr, "Health FAILURE_1 with HealthScore 0 about itself without an observed Birth: a Will message or a regular Health (the Will carries the time of the connect, REQ-LIFE-016)"))
		}
	}
	return false
}

// noBirth reports a connection after a Will that ended, or an observation
// that ended, without a Birth (REQ-LIFE-001, REQ-CMP-003).
func (e *Engine) noBirth(p *part) {
	if p.late == nil {
		return
	}
	x := *p.late
	x.Detail += "; no Birth followed on this connection"
	e.add("T7-MON-03", "REQ-LIFE-001", verdict.Violation, x)
	e.add("T7-MON-03", "REQ-CMP-003", verdict.Violation, x)
	p.late = nil
}

func (e *Engine) endConnection(p *part) {
	p.birthTS, p.birthMAM, p.birthRev = 0, nil, ""
	p.lastHealth = time.Time{}
	p.k2id, p.id2k = map[key]int64{}, map[int64]key{}
	p.pending = map[int64]pending{}
	p.setSeen = false
	e.publistEnd(p)
}

func (e *Engine) birth(p *part, m Message, t *topic.Parsed, msg map[string]any, ds []dsm) {
	const test = "T7-MON-03"
	p.late, p.recovering = nil, false
	p.Births++
	p.state = "connected"
	p.setSeen = false
	for _, r := range []string{"REQ-CMP-003", "REQ-LIFE-001", "REQ-LIFE-002", "REQ-LIFE-003"} {
		e.check(p, test, r)
	}
	res, err := e.v.Validate(validate.Message{Topic: m.Topic, Payload: json.RawMessage(m.Payload), Role: "birth"})
	if err == nil {
		for _, f := range res.Findings {
			if f.Req == "REQ-LIFE-003" {
				e.add(test, f.Req, verdict.Violation, ev(p, m, f.Pointer, "%s", f.Detail))
			}
		}
	}
	p.birthTS = ts(str(msg, "MessageId"))
	for _, d := range ds {
		if d.name == p.AppID {
			p.birthID, p.birthMAM, p.birthRev = d.id, d.payload, revision(d.payload)
		}
	}
	p.lastHealth, p.healthLine = m.Received, m.Line
	e.publistConnect(p, m.Received)
}

func revision(m map[string]any) string {
	n, _ := m["RevisionCounter"].(json.Number)
	return string(n)
}

// content is T7-MON-04, T7-MON-05 (heartbeat), T7-MON-11 and the regular
// MAM part of T7-MON-03. A Pub Metadata carries its content in MetaData
// (meta), not in DataSetMessages. The answer to a Get with a Locale
// (localized) is no regular Pub MAM: its LocalizedText may be in the
// requested language (REQ-LOC-002).
func (e *Engine) content(p *part, m Message, t *topic.Parsed, ds []dsm, unsolicited, birth, meta, localized bool) {
	rs := e.resourceDSMs(ds)
	if unsolicited {
		e.check(p, "T7-MON-11", "REQ-CMP-042")
		if len(rs) == 0 && !meta {
			e.add("T7-MON-11", "REQ-CMP-042", verdict.Violation, ev(p, m, "/Messages", "an unsolicited Pub %s without content (no DataSetMessage of the resource)", t.Resource))
		}
	}
	if t.Resource == "PublicationList" || t.Resource == "SubscriptionList" {
		return
	}
	for _, d := range rs {
		if d.name == "" {
			continue
		}
		s := p.sources[d.name]
		if s == nil {
			s = &source{line: m.Line}
			p.sources[d.name] = s
		}
		switch t.Resource {
		case "MAM":
			s.mam = true
			if d.name == p.AppID && p.birthMAM != nil && !birth && !localized {
				e.regularMAM(p, m, d)
			}
		case "Health":
			s.health = true
			if d.name == p.AppID {
				e.heartbeat(p, m)
			}
		}
	}
}

func (e *Engine) regularMAM(p *part, m Message, d dsm) {
	const test, req = "T7-MON-03", "REQ-LIFE-003"
	if d.idOK && d.id != p.birthID {
		e.add(test, req, verdict.Violation, ev(p, m, d.ptr+"/DataSetWriterId", "the Pub MAM about itself uses DataSetWriterId %d, the Birth used %d", d.id, p.birthID))
	}
	switch rev := revision(d.payload); {
	case rev == p.birthRev && !reflect.DeepEqual(d.payload, p.birthMAM):
		e.add(test, req, verdict.Violation, ev(p, m, d.ptr+"/Payload", "the Pub MAM about itself differs from the Birth with the same RevisionCounter %s", p.birthRev))
	case rev != p.birthRev:
		// a changed configuration (REQ-RES-MAM-018): later MAMs are
		// compared with this one
		p.birthMAM, p.birthRev = d.payload, rev
	}
}

// heartbeat is REQ-TOP-017 for the Health of the component itself: the
// gap between two publications (or the Birth and the first Health) is at
// most heartbeat + tolerance (MG 7.5.1 since SF-106: the heartbeat applies
// to the component's own Health; it is optional for subordinate assets).
func (e *Engine) heartbeat(p *part, m Message) {
	const test, req = "T7-MON-05", "REQ-TOP-017"
	p.healthSelf = true
	limit := e.opt.Heartbeat + e.opt.Tolerance
	if !p.lastHealth.IsZero() {
		if gap := m.Received.Sub(p.lastHealth); gap > limit {
			e.add(test, req, verdict.Violation, ev(p, m, "", "%s between the Health about itself and the previous Health or Birth (line %d), the heartbeat is %s (+%s tolerance)", gap.Round(time.Millisecond), p.healthLine, e.opt.Heartbeat, e.opt.Tolerance))
		} else {
			e.check(p, test, req)
		}
	}
	p.lastHealth, p.healthLine = m.Received, m.Line
}

// writerIDs is T7-MON-08: one DataSetWriterId per resource, DataSetWriterName
// and WriterGroupName of a publisher (REQ-MSG-021), the same id in the
// DataSetMetaData (REQ-MSG-033) and in the PublicationList entry
// (REQ-RES-PUBLIST-005).
func (e *Engine) writerIDs(p *part, m Message, t *topic.Parsed, msg map[string]any, ds []dsm) {
	const test, req = "T7-MON-08", "REQ-MSG-021"
	if t.Resource == "Metadata" {
		id, ok := num(msg, "DataSetWriterId")
		if ok {
			p.metas = append(p.metas, metaRef{line: m.Line, topic: m.Topic, id: id, k: key{"Data", str(msg, "DataSetWriterName"), str(msg, "WriterGroupName")}})
		}
		return
	}
	for _, d := range ds {
		if !d.idOK || d.helper() {
			continue
		}
		k := key{t.Resource, d.name, d.group}
		if t.Resource == "PublicationList" || t.Resource == "SubscriptionList" {
			k = key{resource: t.Resource}
			if t.Resource == "PublicationList" {
				e.entry(p, m, d)
			}
		}
		e.check(p, test, req)
		if p.ids[k] == nil {
			p.ids[k] = map[int64]bool{}
		}
		p.ids[k][d.id] = true
		if id, ok := p.k2id[k]; ok && id != d.id {
			e.add(test, req, verdict.Violation, ev(p, m, d.ptr+"/DataSetWriterId", "%s uses DataSetWriterId %d, before %d", k, d.id, id))
		}
		if k0, ok := p.id2k[d.id]; ok && k0 != k {
			e.add(test, req, verdict.Violation, ev(p, m, d.ptr+"/DataSetWriterId", "DataSetWriterId %d is used for %s and %s", d.id, k0, k))
		}
		p.k2id[k], p.id2k[d.id] = d.id, k
	}
}

func (k key) String() string {
	s := k.resource
	if k.name != "" || k.group != "" {
		s += fmt.Sprintf(" (DataSetWriterName %q, WriterGroupName %q)", k.name, k.group)
	}
	return s
}

func (e *Engine) entry(p *part, m Message, d dsm) {
	id, ok := num(d.payload, "DataSetWriterId")
	res := str(d.payload, "Resource")
	if !ok || res == "" {
		return
	}
	k := key{res, str(d.payload, "Source"), str(d.payload, "Filter")}
	p.entries[k] = entryRef{line: m.Line, topic: m.Topic, id: id, k: k}
}

// paginate is T7-MON-18 for REQ-PAG-004: the pages of one series (one
// PaginationId) go to the same topic, keep TotalCount and PerPage, count
// from 1 without repetition and end with HasNext false.
func (e *Engine) paginate(p *part, m Message, ds []dsm, single bool) {
	const test, req = "T7-MON-18", "REQ-PAG-004"
	for _, d := range ds {
		if !d.idOK || d.id != 2 || d.payload == nil {
			continue
		}
		pid := str(d.payload, "PaginationId")
		total, ok1 := num(d.payload, "TotalCount")
		per, ok2 := num(d.payload, "PerPage")
		page, ok3 := num(d.payload, "Page")
		next, _ := d.payload["HasNext"].(bool)
		if pid == "" || !ok1 || !ok2 || !ok3 {
			continue
		}
		e.check(p, test, req)
		k := p.ID + "|" + pid
		c := e.chains[k]
		if c == nil {
			c = &chain{sender: p.ID, topic: m.Topic, total: total, perPage: per, pages: map[int64]int{}, single: single}
			e.chains[k] = c
		}
		at := d.ptr + "/Payload"
		switch {
		case c.topic != m.Topic:
			e.add(test, req, verdict.Violation, ev(p, m, at, "page %d of the series %q goes to another topic than line %d (%s)", page, pid, c.lastLine, c.topic))
		case c.total != total || c.perPage != per:
			e.add(test, req, verdict.Violation, ev(p, m, at, "page %d of the series %q has TotalCount %d and PerPage %d, the series started with %d and %d", page, pid, total, per, c.total, c.perPage))
		}
		if line, ok := c.pages[page]; ok {
			e.add(test, req, verdict.Violation, ev(p, m, at+"/Page", "page %d of the series %q was already published (line %d)", page, pid, line))
		} else if c.last > 0 && page != c.last+1 {
			e.add(test, req, verdict.Violation, ev(p, m, at+"/Page", "page %d of the series %q follows page %d", page, pid, c.last))
		}
		c.pages[page] = m.Line
		c.last, c.lastNext, c.lastAt, c.lastLine = page, next, m.Received, m.Line
	}
}

// identity is T7-MON-10 (heuristic): one Identification Link names one
// asset.
func (e *Engine) identity(p *part, m Message, t *topic.Parsed, ds []dsm) {
	const test = "T7-MON-10"
	if t.Resource != "MAM" {
		return
	}
	for _, d := range ds {
		piu := str(d.payload, "ProductInstanceUri")
		if piu == "" {
			continue
		}
		e.check(p, test, "REQ-IL-010")
		e.check(p, test, "REQ-IL-012")
		id := str(d.payload, "ManufacturerUri") + "|" + str(d.payload, "ProductCode") + "|" + str(d.payload, "SerialNumber")
		if prev, ok := e.mams[piu]; ok && prev.identity != id {
			e.add(test, "REQ-IL-010", verdict.Note, ev(p, m, d.ptr+"/Payload", "the Identification Link %q names two assets: ManufacturerUri|ProductCode|SerialNumber %q here and %q in line %d", piu, id, prev.identity, prev.line))
		}
		e.mams[piu] = mamRef{line: m.Line, identity: id}
	}
}

// debug is T7-MON-19: no Syslog Debug event from the Birth until the
// first Set addressed to the component (which may change its log level).
func (e *Engine) debug(p *part, m Message, t *topic.Parsed, unsolicited bool) {
	const test, req = "T7-MON-19", "REQ-TOP-024"
	if t.Resource != "Event" || !unsolicited || !(t.Filter == "Syslog/Debug" || strings.HasPrefix(t.Filter, "Syslog/Debug/")) {
		return
	}
	switch {
	case p.setSeen:
		e.add(test, req, verdict.Note, ev(p, m, "", "Syslog Debug event after a Set addressed to the component (the log level may have been changed)"))
	case p.state == "connected":
		e.add(test, req, verdict.Violation, ev(p, m, "", "Syslog Debug event without a Set addressed to the component since its Birth: Debug is not the default log level"))
	default:
		e.add(test, req, verdict.Note, ev(p, m, "", "Syslog Debug event; the start of the component was not observed, so the default log level cannot be judged"))
	}
}

// Finish ends the observation at end and returns the result.
func (e *Engine) Finish(end time.Time) Result {
	ids := make([]string, 0, len(e.parts))
	for id, p := range e.parts {
		if p.seen {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	observed := end.Sub(e.start)
	var parts []Participant
	for _, id := range ids {
		p := e.parts[id]
		e.finishLifecycle(p)
		e.finishSources(p, end, observed)
		e.finishHeartbeat(p, end, observed)
		e.finishWriterIDs(p)
		e.finishSupported(p)
		e.finishChanges(p, end)
		e.finishPublist(p, end)
		e.check(p, "T7-MON-19", "REQ-TOP-024")
		if len(e.appTypes[p.AppID]) > 1 {
			var sts []string
			for st := range e.appTypes[p.AppID] {
				sts = append(sts, st)
			}
			sort.Strings(sts)
			e.add("T7-MON-10", "REQ-IL-012", verdict.Note, verdict.Evidence{Participant: p.ID, Detail: fmt.Sprintf("the Identification Link %q publishes as %s", p.AppID, strings.Join(sts, " and "))})
		}
		for il := range p.sources {
			if il != p.AppID {
				p.Sources = append(p.Sources, il)
			}
		}
		sort.Strings(p.Sources)
		parts = append(parts, p.Participant)
	}
	e.finishChains(end)
	e.finishRepresented(ids)
	return Result{Observations: e.obs, Participants: parts, Messages: e.messages, Namespace: e.ns}
}

func (e *Engine) finishLifecycle(p *part) {
	const test = "T7-MON-03"
	e.noBirth(p)
	if p.Births > 0 || p.Messages == 0 {
		return
	}
	for _, r := range []string{"REQ-CMP-003", "REQ-LIFE-001", "REQ-LIFE-002", "REQ-LIFE-003"} {
		e.skip(p, test, r, true, "no connect of the component was observed (it was connected before the observation started)")
	}
}

func (e *Engine) finishSources(p *part, end time.Time, observed time.Duration) {
	const test = "T7-MON-04"
	if len(p.sources) == 0 {
		return
	}
	names := make([]string, 0, len(p.sources))
	for il := range p.sources {
		names = append(names, il)
	}
	sort.Strings(names)
	for _, il := range names {
		if p.ServiceType == registryST && il != p.AppID {
			continue // a listed asset (MG A1): only the Registry itself is judged
		}
		s := p.sources[il]
		if s.mam {
			e.check(p, test, "REQ-CMP-006")
		} else {
			e.skip(p, test, "REQ-CMP-006", true, "no Pub MAM observed for a Source of the component (test mode requests it with Get MAM)")
		}
		switch {
		case s.health:
			e.check(p, test, "REQ-CMP-007")
		case il == p.AppID && p.Births > 0 && observed >= e.opt.Heartbeat+e.opt.Tolerance:
			e.add(test, "REQ-CMP-007", verdict.Violation, verdict.Evidence{Participant: p.ID, Line: s.line, Detail: "no Pub Health of the component about itself was observed"})
		default:
			e.skip(p, test, "REQ-CMP-007", true, "no Pub Health observed for a Source of the component (test mode requests it with Get Health)")
		}
	}
}

func (e *Engine) finishHeartbeat(p *part, end time.Time, observed time.Duration) {
	const test, req = "T7-MON-05", "REQ-TOP-017"
	limit := e.opt.Heartbeat + e.opt.Tolerance
	e.skip(p, test, "REQ-RES-HEALTH-004", false, "a change of a DetailObject is not observable from the bus (internal state of the asset)")
	connected := p.state == "connected" || p.state == "unknown"
	if connected && !p.lastHealth.IsZero() {
		if gap := end.Sub(p.lastHealth); gap > limit {
			e.add(test, req, verdict.Violation, verdict.Evidence{Participant: p.ID, Line: p.healthLine, Detail: fmt.Sprintf("no Health about itself for %s until the end of the observation, the heartbeat is %s (+%s tolerance)", gap.Round(time.Millisecond), e.opt.Heartbeat, e.opt.Tolerance)})
			return
		}
	}
	if observed < e.opt.MinDuration() {
		e.skip(p, test, req, true, "observation too short: %s < %s", observed.Round(time.Second), e.opt.MinDuration())
		return
	}
	if !p.healthSelf && connected {
		e.add(test, req, verdict.Violation, verdict.Evidence{Participant: p.ID, Detail: fmt.Sprintf("no Health about itself during %s", observed.Round(time.Second))})
		return
	}
	e.check(p, test, req)
}

func (e *Engine) finishWriterIDs(p *part) {
	const test = "T7-MON-08"
	for _, md := range p.metas {
		ids := p.ids[md.k]
		if len(ids) == 0 {
			continue
		}
		e.check(p, test, "REQ-MSG-033")
		if !ids[md.id] {
			e.add(test, "REQ-MSG-033", verdict.Violation, verdict.Evidence{Participant: p.ID, Line: md.line, Topic: md.topic, Pointer: "/DataSetWriterId", Detail: fmt.Sprintf("the DataSetMetaData of %s carries DataSetWriterId %d, the DataSetMessages use %s", md.k, md.id, idList(ids))})
		}
	}
	keys := make([]key, 0, len(p.entries))
	for k := range p.entries {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	for _, k := range keys {
		en := p.entries[k]
		ids := p.ids[k]
		if len(ids) == 0 {
			continue
		}
		e.check(p, test, "REQ-RES-PUBLIST-005")
		if !ids[en.id] {
			e.add(test, "REQ-RES-PUBLIST-005", verdict.Violation, verdict.Evidence{Participant: p.ID, Line: en.line, Topic: en.topic, Detail: fmt.Sprintf("the PublicationList entry of %s names DataSetWriterId %d, the publications use %s", k, en.id, idList(ids))})
		}
	}
}

func idList(ids map[int64]bool) string {
	var s []string
	for id := range ids {
		s = append(s, strconv.FormatInt(id, 10))
	}
	sort.Strings(s)
	return strings.Join(s, ", ")
}

func (e *Engine) finishChains(end time.Time) {
	const test, req = "T7-MON-18", "REQ-PAG-005"
	keys := make([]string, 0, len(e.chains))
	for k := range e.chains {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		c := e.chains[k]
		if !c.lastNext || c.single {
			continue
		}
		p := e.parts[c.sender]
		if end.Sub(c.lastAt) > e.opt.Tolerance {
			e.add(test, req, verdict.Violation, verdict.Evidence{Participant: c.sender, Line: c.lastLine, Topic: c.topic, Detail: fmt.Sprintf("the series ends with page %d and HasNext true; no further page within %s", c.last, e.opt.Tolerance)})
		} else {
			e.skip(p, test, req, true, "observation ended within the tolerance after a page with HasNext true")
		}
	}
}

// registryST is the ServiceType of a Registry (MG A1): the Sources it
// publishes MAM and Health for are listed assets, not represented ones
// (ADR-0025 amendment C-087).
const registryST = "Registry"

// finishRepresented is the heuristic of REQ-CMP-045: a represented entity
// (a Source of another component) does not communicate on the bus itself.
func (e *Engine) finishRepresented(ids []string) {
	const test, req = "T7-MON-04", "REQ-CMP-045"
	byIL := map[string]string{}
	for _, id := range ids {
		byIL[e.parts[id].AppID] = id
	}
	for _, id := range ids {
		p := e.parts[id]
		if p.ServiceType == registryST && len(p.Sources) > 0 {
			e.skip(p, test, req, false, "the participant is a Registry: its Sources are listed assets (MG A1), not represented entities")
			continue
		}
		for _, il := range p.Sources {
			e.check(p, test, req)
			if q, ok := byIL[il]; ok && q != id {
				e.add(test, req, verdict.Note, verdict.Evidence{Participant: id, Detail: fmt.Sprintf("the Source %q of %s publishes on the bus itself as %s", il, id, q)})
			}
		}
	}
}

// ForApp keeps the participants and observations of the component under
// test (DESIGN.md §3.2 --app): its AppId or its <ServiceType>/<AppId>.
func ForApp(app string, obs []verdict.Observation, parts []Participant) ([]verdict.Observation, []Participant) {
	ids := map[string]bool{}
	var keep []Participant
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

// NotFound makes every requirement of the implemented tests inconclusive:
// the component under test did not appear (DESIGN.md §5.3, exit 4).
func NotFound(tests []verdict.Test, app string) []verdict.Observation {
	var out []verdict.Observation
	for _, t := range tests {
		for _, r := range t.Reqs {
			out = append(out, verdict.Observation{Test: t.ID, Req: r, Kind: verdict.Skipped, Inconclusive: true,
				Evidence: verdict.Evidence{Detail: fmt.Sprintf("the component under test %q was not observed", app)}})
		}
	}
	return out
}
