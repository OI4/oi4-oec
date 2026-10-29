// Package probe is the active test runner of oec-check test (DESIGN.md
// §3.3): it discovers the component under test (CUT), sends the stimuli of
// the catalog tests of mode test, correlates the answers by their ReplyTo
// and turns them into verdict observations. The monitor engine runs beside
// it on the same subscription (session.go).
package probe

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/OI4/oec/conformance/catalog"
	"github.com/OI4/oec/conformance/internal/proxy"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// Implemented lists the test-mode catalog tests the runner judges (d7.yaml
// status implemented), in run order.
var Implemented = []string{"T7-CON-01", "T7-CON-02", "T7-CON-03", "T7-CON-04", "T7-LIFE-01", "T7-LIFE-02", "T7-LIFE-03", "T7-LIFE-04",
	"T7-ADR-01", "T7-ADR-02", "T7-ADR-03", "T7-GET-01", "T7-GET-02", "T7-GET-03", "T7-GET-04",
	"T7-GET-05", "T7-GET-06", "T7-GET-07", "T7-GET-08", "T7-GET-09", "T7-SET-01", "T7-SET-02", "T7-SET-03", "T7-SET-04",
	"T7-SET-05", "T7-SET-06", "T7-SET-07", "T7-SET-08", "T7-SET-09", "T7-SET-10", "T7-SET-11", "T7-SET-12",
	"T7-ACT-01", "T7-ACT-02", "T7-ACT-03", "T7-ACT-04"}

// Tests returns the test-mode tests of the D7 catalog for verdict.Build.
func Tests() ([]verdict.Test, error) {
	c, err := catalog.Load(catalog.D7File)
	if err != nil {
		return nil, err
	}
	var out []verdict.Test
	for _, t := range c.Tests {
		if t.Mode == "test" {
			out = append(out, verdict.Test{ID: t.ID, Title: t.Title, Status: t.Status, Level: t.Level, Reqs: t.Reqs})
		}
	}
	return out, nil
}

// Classes are the predefined DataSetClassIds of the resources D7 requests
// (MG 8.3; probe_test.go checks them against the entry schemas).
var Classes = map[string]string{
	"MAM":                  "360ca8f3-5e66-42a2-8f10-9cdf45f4bf58",
	"Health":               "d8e7b6df-42ba-448a-975a-199f59e8ffeb",
	"Config":               "9d5983db-440d-4474-9fd7-1cd7a6c8b6c2",
	"Profile":              "48017c6a-05c8-48d7-9d85-4b08bbb707f3",
	"PublicationList":      "217434d6-6e1e-4230-b907-f52bc9ffe152",
	"ReferenceDesignation": "27a75019-164a-496d-a38b-90e8a55c2cfa",
	"SubscriptionList":     "e5d68c47-c276-4929-8ab9-4c1090cac785",
}

// ServiceTypes of MG 4.2, in the order discovery tries them.
var ServiceTypes = []string{"OTConnector", "ITConnector", "Utility", "Persistence", "Aggregation", "OOCConnector", "Registry"}

// Gettable are the resources a Get may address with a Source and without
// Filter (Method × Resource matrix; Metadata needs a Filter, Event and
// NewDataSetWriterId are not requested with Get).
var Gettable = []string{"MAM", "Health", "Config", "Data", "Profile", "PublicationList", "ReferenceDesignation", "SubscriptionList"}

// levelResources maps the resources of the compliance levels to the CMP
// requirement that makes them mandatory (MG 1.3); MAM and Health are
// mandatory at every level.
var levelResources = []struct{ resource, req string }{
	{"Profile", "REQ-CMP-018"}, {"PublicationList", "REQ-CMP-019"}, {"ReferenceDesignation", "REQ-CMP-020"},
	{"Config", "REQ-CMP-035"}, {"Data", "REQ-CMP-036"}, {"Metadata", "REQ-CMP-037"}, {"Event", "REQ-CMP-038"},
	{"SubscriptionList", "REQ-CMP-039"},
}

// BadNotFound is the StatusCode for an unknown asset (MG 8.3.6.1).
const BadNotFound = 0x803E0000

// UnknownKey is the key T7-GET-09 adds to a Get (REQ-MSG-046).
const UnknownKey = "OecCheckUnknownKey"

// Options of a run.
type Options struct {
	App      string        // AppId or <ServiceType>/<AppId>
	Level    int           // declared level 1 … 3
	Timeout  time.Duration // waiting time for an answer; also the silence that counts as "no answer"
	Grace    time.Duration // collection time after the expected answers (default 250 ms, at most Timeout)
	TesterIL string        // Identification Link of D7 (default: generated per run)
	Tests    []string      // test ids or path.Match patterns; empty: every implemented test

	// Subscribed is called by RunBus after the subscription and before the
	// first request (tests start the simulated CUT here, so the monitor
	// sees its Birth message).
	Subscribed func(context.Context) error

	// Proxy is the proxy attachment (DESIGN.md §3.4); nil: bus attachment,
	// the connection and life cycle tests are inconclusive. RunBus starts
	// it after the subscription.
	Proxy *proxy.Proxy
	// ProxyWait is the time the CUT has to connect through the proxy, at
	// the start and after the drop (default 2 min).
	ProxyWait time.Duration
	// Tolerance is the allowed difference between the time in the
	// MessageId of a Will and its CONNECT (REQ-LIFE-016, default 5 s).
	Tolerance time.Duration

	// Restart is the restart hook (DESIGN.md §3.4): it stops the CUT
	// gracefully and starts it again; nil: T7-LIFE-04 and T7-SET-10 are
	// inconclusive.
	Restart func(context.Context) error
	// RestartWait is the time the CUT has to come back after the restart
	// hook returned (default 2 min).
	RestartWait time.Duration

	// Announce makes D7 a component on the bus (DESIGN.md §3.5, Q-029
	// (a)): RunBus publishes Birth, Health and Close message under the
	// tester IL and registers its Will; false: D7 is not announced.
	Announce bool
	// Peer is the peer attachment (DESIGN.md §3.4): RunBus announces a
	// simulated second component PeerIL on its own connection; T7-ADR-01
	// judges the requests of the CUT to it. false: T7-ADR-01 is
	// inconclusive.
	Peer   bool
	PeerIL string // Identification Link of the peer (default: generated per run)
}

// Request is one stimulus of the run (report).
type Request struct {
	Test      string    `json:"test"`
	Topic     string    `json:"topic"`
	MessageID string    `json:"message_id"`
	Sent      time.Time `json:"sent"`
	Replies   int       `json:"replies"` // messages with the MessageId as ReplyTo; for a Call the ActionResponses with its RequestId
}

// Result of a run.
type Result struct {
	ServiceType  string
	AppID        string
	Found        bool   // the CUT answered or was seen on the bus
	Tester       string // PublisherId of D7
	Requests     []Request
	Observations []verdict.Observation
	Restore      *Restore    // nil if no write test ran
	Announced    []Announced // the components D7 announced (--announce, --peer)
}

// PublishFunc sends one request.
type PublishFunc func(ctx context.Context, topic string, payload []byte) error

// Runner runs the tests against one CUT. Deliver may be called from another
// goroutine while Run is active.
type Runner struct {
	opt  Options
	reqs verdict.Requirements
	pub  PublishFunc

	mu        sync.Mutex
	exchanges []*exchange
	seen      map[string]bool // ServiceTypes under which the AppId was seen publishing
	last      int64

	tester      string
	st, app     string
	found       bool
	mamAnswered bool // Get MAM/<AppId> was answered (discovery)
	profile     []string
	hasProfile  bool
	obs         []verdict.Observation
	cites       map[string][]string // catalog REQs of the tests
	v1          []Message           // received under the namespace Oi4 (T7-ADR-02)
	v1Refused   error               // the subscription to Oi4/# was refused
	mamAll      []string            // Sources of the answer to Get MAM without Source
	before      *state              // state before the first write test
	restore     *Restore
	used        map[int64]bool // DataSetWriterIds the CUT publishes or lists in its PublicationList
	rid         int64          // last RequestId of D7
	calls       []*call
	actions     []*actMsg // received on a Reply topic of the CUT
	provided    []provided
	retained    []*netMsg // delivered with Retain, from the CUT (T7-CON-04 without proxy)
	dropped     time.Time // the proxy dropped the connection of the CUT
	busWill     *netMsg   // Health FAILURE_1 of the CUT about itself after dropped
	restartAt   time.Time // the restart hook was started
	restarted   []*netMsg // publications of the CUT after restartAt
	peer        string    // AppId of the simulated peer ("" without --peer)
	toPeer      []*netMsg // requests of the CUT that address the peer (T7-ADR-01)
	// present lets the announced components publish their Health once
	// the CUT is found (RunBus): a CUT that connected after their Birth
	// sees them by their heartbeat.
	present func(context.Context) error
}

// New creates a runner; the tester identity is fixed here.
func New(opt Options, reqs verdict.Requirements, pub PublishFunc) *Runner {
	if opt.Timeout <= 0 {
		opt.Timeout = 10 * time.Second
	}
	if opt.Grace <= 0 {
		opt.Grace = 250 * time.Millisecond
	}
	if opt.Grace > opt.Timeout {
		opt.Grace = opt.Timeout
	}
	if opt.TesterIL == "" {
		opt.TesterIL = TesterIL()
	}
	if opt.ProxyWait <= 0 {
		opt.ProxyWait = 2 * time.Minute
	}
	if opt.Tolerance <= 0 {
		opt.Tolerance = 5 * time.Second
	}
	if opt.RestartWait <= 0 {
		opt.RestartWait = 2 * time.Minute
	}
	r := &Runner{opt: opt, reqs: reqs, pub: pub, seen: map[string]bool{}, tester: "Utility/" + opt.TesterIL, cites: map[string][]string{},
		used: map[int64]bool{}}
	r.st, r.app = SplitApp(opt.App)
	if opt.Peer {
		if opt.PeerIL == "" {
			r.opt.PeerIL = PeerIL()
		}
		r.peer = r.opt.PeerIL
	}
	if tests, err := Tests(); err == nil {
		for _, t := range tests {
			r.cites[t.ID] = t.Reqs
		}
	}
	return r
}

// Tester returns the PublisherId of D7.
func (r *Runner) Tester() string { return r.tester }

// TesterIL returns a tester Identification Link for one run (DESIGN.md
// §3.5): an Oi4Identifier with a random serial number.
func TesterIL() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "oi4.org/oec-check/D7/" + hex.EncodeToString(b[:])
}

// SplitApp splits --app into ServiceType ("" if not given) and AppId.
func SplitApp(app string) (string, string) {
	if st, il, ok := strings.Cut(app, "/"); ok {
		for _, s := range ServiceTypes {
			if s == st {
				return st, il
			}
		}
	}
	return "", app
}

func enc(il string) string { return base64.RawURLEncoding.EncodeToString([]byte(il)) }

func dec(s string) string {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return ""
	}
	return string(b)
}

// Message is one message received from the bus.
type Message struct {
	Topic    string
	Payload  []byte
	Received time.Time
	Retain   bool // delivered as a retained message
}

type dsm struct {
	id      int64
	name    string // DataSetWriterName
	group   string // WriterGroupName
	payload any
}

// pagination is the Pagination helper DataSet of a NetworkMessage
// (MG 8.3.11.2).
type pagination struct {
	total, perPage, page int64
	hasNext              bool
	id                   string
}

// helper reports a helper DataSet (DataSetWriterId 0 … 9, MG 8.3.11).
func (d dsm) helper() bool { return d.id >= 0 && d.id <= 9 }

type netMsg struct {
	topic, method, resource         string
	st, app, source                 string // source: the Source of the topic, decoded ("" if absent)
	messageID, replyTo, publisherID string
	msgType                         string
	msgs                            []dsm // for ua-metadata: one entry with the MetaData as payload
	paged                           bool
	pag                             *pagination
	size                            int
	received                        time.Time
}

// parse decodes the parts the runner needs; nil if the message is no OEC
// NetworkMessage.
func parse(m Message) *netMsg {
	lv := strings.Split(m.Topic, "/")
	if len(lv) < 5 || lv[0] != "Oi4v2" {
		return nil
	}
	var doc map[string]any
	if json.Unmarshal(m.Payload, &doc) != nil {
		return nil
	}
	n := &netMsg{topic: m.Topic, st: lv[1], app: dec(lv[2]), method: lv[3], resource: lv[4], received: m.Received, size: len(m.Payload)}
	if len(lv) > 5 {
		n.source = dec(lv[5])
	}
	n.messageID, _ = doc["MessageId"].(string)
	n.replyTo, _ = doc["ReplyTo"].(string)
	n.publisherID, _ = doc["PublisherId"].(string)
	n.msgType, _ = doc["MessageType"].(string)
	if n.msgType == "ua-metadata" {
		d := dsm{id: -1, payload: doc["MetaData"]}
		if f, ok := doc["DataSetWriterId"].(float64); ok {
			d.id = int64(f)
		}
		d.name, _ = doc["DataSetWriterName"].(string)
		d.group, _ = doc["WriterGroupName"].(string)
		n.msgs = append(n.msgs, d)
		return n
	}
	list, _ := doc["Messages"].([]any)
	for _, x := range list {
		o, _ := x.(map[string]any)
		d := dsm{id: -1, payload: o["Payload"]}
		if f, ok := o["DataSetWriterId"].(float64); ok {
			d.id = int64(f)
		}
		d.name, _ = o["DataSetWriterName"].(string)
		d.group, _ = o["WriterGroupName"].(string)
		if d.id == 2 {
			n.paged = true
			p, _ := d.payload.(map[string]any)
			pg := &pagination{}
			pg.total, pg.perPage, pg.page = num(p["TotalCount"]), num(p["PerPage"]), num(p["Page"])
			pg.hasNext, _ = p["HasNext"].(bool)
			pg.id, _ = p["PaginationId"].(string)
			n.pag = pg
		}
		n.msgs = append(n.msgs, d)
	}
	return n
}

func num(v any) int64 {
	f, _ := v.(float64)
	return int64(f)
}

// content returns the DataSetMessages without helpers.
func (n *netMsg) content() []dsm {
	var out []dsm
	for _, d := range n.msgs {
		if !d.helper() {
			out = append(out, d)
		}
	}
	return out
}

type exchange struct {
	test        string
	id          string
	method      string // Get, Set or Del
	topic       string
	answerTopic string
	resource    string
	source      string
	filter      string
	sent        time.Time
	replies     []*netMsg // ReplyTo = MessageId of the request
	others      []*netMsg // on the answer topic with another ReplyTo, after the request
}

func isEvent(n *netMsg) bool { return n.method == "Pub" && n.resource == "Event" }

// answers returns the replies that are no Event (an Event Status is the
// reaction to a failed request, not its answer).
func (x *exchange) answers() []*netMsg {
	var out []*netMsg
	for _, n := range x.replies {
		if !isEvent(n) {
			out = append(out, n)
		}
	}
	return out
}

func (x *exchange) events() []*netMsg {
	var out []*netMsg
	for _, n := range x.replies {
		if isEvent(n) {
			out = append(out, n)
		}
	}
	return out
}

// cyclic resources are published without request as well; a publication
// on their topic with an empty ReplyTo is no evidence of a wrong answer.
var cyclic = map[string]bool{"Health": true, "Data": true, "Event": true}

// Deliver hands a received message to the runner.
func (r *Runner) Deliver(m Message) {
	if strings.HasPrefix(m.Topic, V1Namespace+"/") {
		r.mu.Lock()
		r.v1 = append(r.v1, m)
		r.mu.Unlock()
		return
	}
	n := parse(m)
	if n != nil && n.method == "Reply" && r.app != "" && n.app == r.app {
		if a := parseAction(m); a != nil {
			r.mu.Lock()
			r.actions = append(r.actions, a)
			r.mu.Unlock()
		}
		return
	}
	if n != nil && r.requestToPeer(n) {
		r.mu.Lock()
		r.toPeer = append(r.toPeer, n)
		r.mu.Unlock()
		return
	}
	if n == nil || n.method != "Pub" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.app != "" && n.app == r.app && strings.HasSuffix(n.publisherID, "/"+r.app) {
		r.seen[n.st] = true
		r.noteUsed(n)
		if m.Retain {
			r.retained = append(r.retained, n)
		}
		if !r.restartAt.IsZero() {
			r.restarted = append(r.restarted, n)
		}
		if !r.dropped.IsZero() && r.busWill == nil && n.resource == "Health" && strings.HasSuffix(n.topic, "/"+enc(r.app)) {
			if state, _, _, ok := health(n); ok && state == "FAILURE_1" {
				r.busWill = n
			}
		}
	}
	for _, x := range r.exchanges {
		switch {
		case n.replyTo != "" && n.replyTo == x.id:
			x.replies = append(x.replies, n)
		case n.topic == x.answerTopic && !n.received.Before(x.sent) && (n.replyTo != "" || !cyclic[n.resource]):
			x.others = append(x.others, n)
		}
	}
}

func (r *Runner) nextID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	ms := time.Now().UnixMilli()
	if ms <= r.last {
		ms = r.last + 1
	}
	r.last = ms
	return fmt.Sprintf("%d-%s", ms, r.tester)
}

// Topic builds an OEC topic; the filter is used only with a source.
func Topic(st, app, method, resource, source, filter string) string {
	t := "Oi4v2/" + st + "/" + enc(app) + "/" + method + "/" + resource
	if source != "" {
		t += "/" + enc(source)
		if filter != "" {
			t += "/" + filter
		}
	}
	return t
}

// get sends a Get; extra keys are added to the NetworkMessage.
func (r *Runner) get(ctx context.Context, test, st, resource, source, filter string, extra map[string]any) (*exchange, error) {
	id := r.nextID()
	doc := map[string]any{"MessageId": id, "MessageType": "ua-data", "PublisherId": r.tester, "ReplyTo": "", "Messages": []any{}}
	if c, ok := Classes[resource]; ok {
		doc["DataSetClassId"] = c
	}
	for k, v := range extra {
		doc[k] = v
	}
	payload, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	x := &exchange{test: test, id: id, method: "Get", resource: resource, source: source, filter: filter, sent: time.Now(),
		topic: Topic(st, r.app, "Get", resource, source, filter), answerTopic: Topic(st, r.app, "Pub", resource, source, filter)}
	r.mu.Lock()
	r.exchanges = append(r.exchanges, x)
	r.mu.Unlock()
	if err := r.pub(ctx, x.topic, payload); err != nil {
		return nil, fmt.Errorf("publish %s: %w", x.topic, err)
	}
	return x, nil
}

// helper builds a helper DataSetMessage of D7 (MG 8.3.11 … 8.3.13): the
// DataSetWriterName of a request helper is the requester (REQ-MSG-019).
func (r *Runner) helper(id int, group string, payload map[string]any) map[string]any {
	return map[string]any{"DataSetWriterId": id, "WriterGroupName": group, "DataSetWriterName": r.opt.TesterIL, "Payload": payload}
}

// getWith sends a Get whose Messages are the given helper DataSets.
func (r *Runner) getWith(ctx context.Context, test, resource, source, filter string, helpers ...map[string]any) (*exchange, error) {
	list := []any{}
	for _, h := range helpers {
		list = append(list, h)
	}
	return r.get(ctx, test, r.st, resource, source, filter, map[string]any{"Messages": list})
}

// getMeta sends a Get Metadata for one Data DataSet (MG 9.1.5): a
// ua-metadata message with the empty MetaData (REQ-RES-META-001).
func (r *Runner) getMeta(ctx context.Context, test, source, tag string, writer int64) (*exchange, error) {
	id := r.nextID()
	doc := map[string]any{"MessageId": id, "MessageType": "ua-metadata", "PublisherId": r.tester, "DataSetWriterId": writer,
		"WriterGroupName": tag, "DataSetWriterName": source, "Timestamp": time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"ReplyTo": "", "MetaData": map[string]any{}}
	payload, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	x := &exchange{test: test, id: id, method: "Get", resource: "Metadata", source: source, filter: tag, sent: time.Now(),
		topic: Topic(r.st, r.app, "Get", "Metadata", source, tag), answerTopic: Topic(r.st, r.app, "Pub", "Metadata", source, tag)}
	r.mu.Lock()
	r.exchanges = append(r.exchanges, x)
	r.mu.Unlock()
	if err := r.pub(ctx, x.topic, payload); err != nil {
		return nil, fmt.Errorf("publish %s: %w", x.topic, err)
	}
	return x, nil
}

// until returns when done reports true (plus the grace time) or the
// timeout ended.
func (r *Runner) until(ctx context.Context, done func() bool) error {
	deadline := time.Now().Add(r.opt.Timeout)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for time.Now().Before(deadline) {
		if done() {
			return sleep(ctx, r.opt.Grace)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
	return nil
}

// wait returns when every exchange of want has an answer (plus the grace
// time) or the timeout ended; with silent exchanges (no answer expected) it
// always waits for the timeout.
func (r *Runner) wait(ctx context.Context, want, silent []*exchange) error {
	deadline := time.Now().Add(r.opt.Timeout)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		if len(silent) == 0 && r.answered(want) {
			return sleep(ctx, r.opt.Grace)
		}
		if !time.Now().Before(deadline) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

// waitAny returns when one of the exchanges has an answer (plus the grace
// time) or the timeout ended.
func (r *Runner) waitAny(ctx context.Context, xs []*exchange) error {
	deadline := time.Now().Add(r.opt.Timeout)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for time.Now().Before(deadline) {
		for _, x := range xs {
			if r.answered([]*exchange{x}) {
				return sleep(ctx, r.opt.Grace)
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
	return nil
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (r *Runner) answered(xs []*exchange) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range xs {
		if len(x.answers()) == 0 {
			return false
		}
	}
	return true
}

// snapshot copies the received messages of an exchange.
func (r *Runner) snapshot(x *exchange) (answers, events, others []*netMsg) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return x.answers(), x.events(), append([]*netMsg(nil), x.others...)
}

func (r *Runner) ev(x *exchange, format string, a ...any) verdict.Evidence {
	e := verdict.Evidence{Participant: r.st + "/" + r.app, Detail: fmt.Sprintf(format, a...)}
	if x != nil {
		e.Topic = x.topic
	}
	return e
}

// add records an observation if the catalog lists the requirement for the
// test: the shared checks (reply) judge REQ-BUS-003 and REQ-MSG-015 only
// in the test that cites them.
func (r *Runner) add(o verdict.Observation) {
	if reqs, ok := r.cites[o.Test]; ok && !contains(reqs, o.Req) {
		return
	}
	r.obs = append(r.obs, o)
}

func (r *Runner) checked(test, req string, x *exchange) {
	detail := "checked"
	if x != nil {
		detail = "answer to Get (MessageId " + x.id + ")"
	}
	r.add(verdict.Observation{Test: test, Req: req, Kind: verdict.Checked, Evidence: r.ev(x, "%s", detail)})
}

func (r *Runner) violation(test, req string, x *exchange, format string, a ...any) {
	r.add(verdict.Observation{Test: test, Req: req, Kind: verdict.Violation, Evidence: r.ev(x, format, a...)})
}

func (r *Runner) skip(test, req string, format string, a ...any) {
	r.add(verdict.Observation{Test: test, Req: req, Kind: verdict.Skipped, Evidence: r.ev(nil, format, a...)})
}

// applies reports whether the requirement applies at the declared level.
func (r *Runner) applies(req string) bool {
	o := r.reqs.Obligation(req, r.opt.Level)
	return o != "" && o != "none"
}

// mandatory reports whether the requirement is MUST at the declared level.
func (r *Runner) mandatory(req string) bool {
	return verdict.Mandatory(r.reqs.Obligation(req, r.opt.Level))
}

// reply judges the answer of a Get: on the Pub topic of the request with
// its MessageId as ReplyTo (REQ-BUS-003, REQ-MSG-015). Without an answer
// every requirement of missing is violated. It returns the answers on the
// right topic.
func (r *Runner) reply(test string, x *exchange, missing ...string) []*netMsg {
	answers, _, others := r.snapshot(x)
	var ok []*netMsg
	for _, a := range answers {
		if a.topic != x.answerTopic {
			r.violation(test, "REQ-BUS-003", x, "the answer to Get (MessageId %s) is published on %s instead of %s", x.id, a.topic, x.answerTopic)
			continue
		}
		ok = append(ok, a)
	}
	if len(ok) > 0 {
		r.checked(test, "REQ-BUS-003", x)
		r.checked(test, "REQ-MSG-015", x)
		return ok
	}
	if len(answers) > 0 {
		return nil
	}
	if len(others) > 0 {
		r.violation(test, "REQ-MSG-015", x, "%d publication(s) on %s after the Get, with ReplyTo %q instead of the Get's MessageId %s", len(others), x.answerTopic, others[0].replyTo, x.id)
	}
	for _, req := range missing {
		r.violation(test, req, x, "Get (MessageId %s) not answered within %s", x.id, r.opt.Timeout)
	}
	return nil
}

// Run discovers the CUT and runs the selected tests.
func (r *Runner) Run(ctx context.Context) (Result, error) {
	if r.opt.Proxy != nil {
		if err := r.awaitProxy(ctx); err != nil {
			return r.result(), err
		}
	}
	if err := r.discover(ctx); err != nil {
		return r.result(), err
	}
	if !r.found {
		return r.result(), nil
	}
	if r.present != nil {
		if err := r.present(ctx); err != nil {
			return r.result(), err
		}
	}
	tests := map[string]func(context.Context) error{
		"T7-ADR-01": r.adr01, "T7-ADR-02": r.adr02, "T7-ADR-03": r.adr03,
		"T7-GET-01": r.get01, "T7-GET-02": r.get02, "T7-GET-03": r.get03, "T7-GET-04": r.get04, "T7-GET-05": r.get05,
		"T7-GET-06": r.get06, "T7-GET-07": r.get07, "T7-GET-08": r.get08, "T7-GET-09": r.get09,
		"T7-SET-01": r.set01, "T7-SET-02": r.set02, "T7-SET-03": r.set03, "T7-SET-04": r.set04, "T7-SET-05": r.set05,
		"T7-SET-06": r.set06, "T7-SET-07": r.set07, "T7-SET-08": r.set08, "T7-SET-09": r.set09, "T7-SET-11": r.set11,
		"T7-SET-12": r.set12,
		"T7-ACT-01": r.act01, "T7-ACT-02": r.act02, "T7-ACT-03": r.act03, "T7-ACT-04": r.act04,
	}
	// The resource tests need the Profile T7-GET-02 reads.
	if !r.Selected("T7-GET-02") {
		for _, id := range []string{"T7-GET-03", "T7-GET-04", "T7-GET-05", "T7-GET-07", "T7-GET-08"} {
			if r.Selected(id) {
				if err := r.readProfile(ctx, ""); err != nil {
					return r.result(), err
				}
				break
			}
		}
	}
	writing := false
	// interrupted records that the run ended after the first write: the
	// CUT may be left modified.
	interrupted := func(id string, err error) (Result, error) {
		if r.before != nil && r.restore == nil {
			r.mu.Lock()
			r.restore = &Restore{Note: "the run ended during the write tests (" + id + "): the CUT may be left modified"}
			r.mu.Unlock()
		}
		return r.result(), err
	}
	for _, id := range Implemented {
		if !r.Selected(id) || id == "T7-SET-03" || isLifecycleTest(id) || isRestartTest(id) {
			continue
		}
		// The Action tests run in the write window: the ids the CUT
		// reserves must not change the compared state.
		if (strings.HasPrefix(id, "T7-SET-") || isActionTest(id)) && !writing {
			writing = true
			if err := r.prepareWrite(ctx); err != nil {
				return r.result(), err
			}
		}
		if err := tests[id](ctx); err != nil {
			return interrupted(id, err)
		}
	}
	// T7-SET-03 judges the reactions of the other write tests.
	if r.Selected("T7-SET-03") {
		if !writing {
			writing = true
			if err := r.prepareWrite(ctx); err != nil {
				return r.result(), err
			}
		}
		if err := r.set03(ctx); err != nil {
			return interrupted("T7-SET-03", err)
		}
	}
	// The restart tests run in the write window: T7-SET-10 restores the
	// ReferenceDesignation it changed before the restore check.
	if r.restartSelected() {
		if r.Selected("T7-SET-10") && !writing {
			writing = true
			if err := r.prepareWrite(ctx); err != nil {
				return r.result(), err
			}
		}
		if err := r.restart(ctx); err != nil {
			return interrupted("restart", err)
		}
	}
	if writing {
		if err := r.checkRestore(ctx); err != nil {
			return interrupted("restore check", err)
		}
	}
	// The connection and life cycle tests drop the connection of the CUT:
	// after every other test.
	if r.lifecycleSelected() {
		if err := r.lifecycle(ctx); err != nil {
			return r.result(), err
		}
	}
	// T7-ADR-02 watches the namespace Oi4 during the whole run.
	if r.Selected("T7-ADR-02") {
		if err := sleep(ctx, r.opt.Grace); err != nil {
			return r.result(), err
		}
		r.judgeADR02()
	}
	// T7-ADR-01 judges the requests to the peer of the whole run.
	if r.Selected("T7-ADR-01") {
		r.judgeADR01()
	}
	return r.result(), nil
}

// Selected reports whether --tests selects the test.
func (r *Runner) Selected(id string) bool { return Selected(r.opt.Tests, id) }

// Selected reports whether one of the patterns (test ids or path.Match
// patterns) selects the test; no pattern selects every test.
func Selected(patterns []string, id string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, p := range patterns {
		if ok, _ := path.Match(p, id); ok {
			return true
		}
	}
	return false
}

func (r *Runner) result() Result {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := Result{ServiceType: r.st, AppID: r.app, Found: r.found, Tester: r.tester, Observations: r.obs, Restore: r.restore}
	for _, x := range r.exchanges {
		res.Requests = append(res.Requests, Request{Test: x.test, Topic: x.topic, MessageID: x.id, Sent: x.sent.UTC(), Replies: len(x.replies)})
	}
	for _, c := range r.calls {
		res.Requests = append(res.Requests, Request{Test: c.test, Topic: c.topic, MessageID: c.id, Sent: c.sent.UTC(), Replies: len(r.responsesLocked(c))})
	}
	sort.SliceStable(res.Requests, func(i, j int) bool { return res.Requests[i].Sent.Before(res.Requests[j].Sent) })
	return res
}

// discover finds the ServiceType of the CUT: Get MAM/<AppId> on the given
// or on every ServiceType; else a publication of the CUT seen meanwhile.
func (r *Runner) discover(ctx context.Context) error {
	sts := ServiceTypes
	if r.st != "" {
		sts = []string{r.st}
	}
	var xs []*exchange
	for _, st := range sts {
		x, err := r.get(ctx, "discovery", st, "MAM", r.app, "", nil)
		if err != nil {
			return err
		}
		xs = append(xs, x)
	}
	if err := r.waitAny(ctx, xs); err != nil {
		return err
	}
	for _, x := range xs {
		answers, _, _ := r.snapshot(x)
		for _, a := range answers {
			if a.topic == x.answerTopic {
				r.st, r.found, r.mamAnswered = a.st, true, true
				return nil
			}
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.st != "" {
		r.found = r.seen[r.st]
		return nil
	}
	var seen []string
	for st := range r.seen {
		seen = append(seen, st)
	}
	sort.Strings(seen)
	if len(seen) > 0 {
		r.st, r.found = seen[0], true
	}
	return nil
}

func sources(ns []*netMsg) []string {
	var out []string
	for _, n := range ns {
		for _, d := range n.content() {
			out = append(out, d.name)
		}
	}
	return out
}

func contains(s []string, x string) bool {
	for _, e := range s {
		if e == x {
			return true
		}
	}
	return false
}

// maxSources limits the assets T7-GET-01 requests one by one.
const maxSources = 20

var assetReq = map[string]string{"MAM": "REQ-RES-MAM-021", "Health": "REQ-RES-HEALTH-006"}

// get01 is T7-GET-01: Get MAM and Health without and with Source, for the
// application and every subordinate asset of the answers.
func (r *Runner) get01(ctx context.Context) error {
	const t = "T7-GET-01"
	var missing []string
	if r.applies("REQ-CMP-017") {
		missing = []string{"REQ-BUS-003", "REQ-CMP-015", "REQ-CMP-017"}
	}
	mamAll, err := r.get(ctx, t, r.st, "MAM", "", "", nil)
	if err != nil {
		return err
	}
	healthAll, err := r.get(ctx, t, r.st, "Health", "", "", nil)
	if err != nil {
		return err
	}
	if err := r.wait(ctx, []*exchange{mamAll, healthAll}, nil); err != nil {
		return err
	}
	answers := map[string][]*netMsg{"MAM": r.reply(t, mamAll, missing...), "Health": r.reply(t, healthAll, missing...)}
	all := map[string][]string{"MAM": sources(answers["MAM"]), "Health": sources(answers["Health"])}
	xs := map[string]*exchange{"MAM": mamAll, "Health": healthAll}
	for _, res := range []string{"MAM", "Health"} {
		ns, x := answers[res], xs[res]
		if len(ns) == 0 {
			continue
		}
		bad := false
		if len(ns) > 1 && !ns[0].paged {
			r.violation(t, assetReq[res], x, "Get %s without Source is answered with %d NetworkMessages instead of one", res, len(ns))
			bad = true
		}
		if !contains(all[res], r.app) {
			r.violation(t, assetReq[res], x, "the answer to Get %s without Source lacks the DataSetMessage of the application itself", res)
			bad = true
		}
		if !bad {
			r.checked(t, assetReq[res], x)
		}
	}
	if len(answers["MAM"]) > 0 && len(answers["Health"]) > 0 {
		for _, s := range all["MAM"] {
			if !contains(all["Health"], s) {
				r.violation(t, "REQ-RES-HEALTH-006", healthAll, "the answer to Get Health without Source lacks the Health of %q, whose MAM is in the answer to Get MAM", s)
			}
		}
		for _, s := range all["Health"] {
			if !contains(all["MAM"], s) {
				r.violation(t, "REQ-RES-MAM-021", mamAll, "the answer to Get MAM without Source lacks the MAM of %q, whose Health is in the answer to Get Health", s)
			}
		}
	}
	srcs := []string{r.app}
	for _, s := range append(all["MAM"], all["Health"]...) {
		if s != "" && !contains(srcs, s) && len(srcs) < maxSources {
			srcs = append(srcs, s)
		}
	}
	var each []*exchange
	for _, s := range srcs {
		for _, res := range []string{"MAM", "Health"} {
			x, err := r.get(ctx, t, r.st, res, s, "", nil)
			if err != nil {
				return err
			}
			each = append(each, x)
		}
	}
	if err := r.wait(ctx, each, nil); err != nil {
		return err
	}
	for _, x := range each {
		ns := r.reply(t, x, missing...)
		if len(ns) == 0 {
			continue
		}
		names := sources(ns)
		if len(names) != 1 || names[0] != x.source {
			r.violation(t, assetReq[x.resource], x, "Get %s with Source %q is answered with the DataSetMessages of %q; want exactly the one of the Source", x.resource, x.source, names)
			continue
		}
		r.checked(t, assetReq[x.resource], x)
		if len(missing) > 0 {
			r.checked(t, "REQ-CMP-015", x)
			r.checked(t, "REQ-CMP-017", x)
		}
	}
	if len(missing) == 0 {
		r.skip(t, "REQ-BUS-003", "Get of MAM and Health is not required at level %d (REQ-CMP-017); an answer, if any, is judged", r.opt.Level)
	}
	return nil
}

// readProfile requests the Profile of the application; with a test it
// judges the answer (T7-GET-02), without it only reads it.
func (r *Runner) readProfile(ctx context.Context, test string) error {
	x, err := r.get(ctx, test, r.st, "Profile", r.app, "", nil)
	if err != nil {
		return err
	}
	if err := r.wait(ctx, []*exchange{x}, nil); err != nil {
		return err
	}
	if test == "" {
		answers, _, _ := r.snapshot(x)
		for _, a := range answers {
			if a.topic == x.answerTopic && r.setProfile(a) {
				break
			}
		}
		return nil
	}
	var missing []string
	if r.applies("REQ-CMP-018") {
		missing = []string{"REQ-CMP-018"}
	}
	ns := r.reply(test, x, missing...)
	if len(ns) == 0 {
		return nil
	}
	r.checked(test, "REQ-CMP-018", x)
	if !r.setProfile(ns[0]) {
		r.violation(test, "REQ-RES-PROFILE-004", x, "the answer to Get Profile/<AppId> has no DataSetMessage of %q with a Resources list", r.app)
		return nil
	}
	r.checked(test, "REQ-RES-PROFILE-004", x)
	return nil
}

func (r *Runner) setProfile(n *netMsg) bool {
	for _, d := range n.content() {
		if d.name != r.app {
			continue
		}
		p, _ := d.payload.(map[string]any)
		list, ok := p["Resources"].([]any)
		if !ok {
			return false
		}
		r.profile, r.hasProfile = nil, true
		for _, x := range list {
			if s, ok := x.(string); ok {
				r.profile = append(r.profile, s)
			}
		}
		return true
	}
	return false
}

// get02 is T7-GET-02: the Profile lists every resource the level requires,
// every listed resource answers a Get and no other one does.
func (r *Runner) get02(ctx context.Context) error {
	const t = "T7-GET-02"
	if err := r.readProfile(ctx, t); err != nil {
		return err
	}
	if !r.hasProfile {
		r.skip(t, "REQ-CMP-021", "no Profile of the application")
		r.skip(t, "REQ-RES-PROFILE-002", "no Profile of the application")
		return nil
	}
	required := []string{"MAM", "Health"}
	for _, lr := range levelResources {
		if r.mandatory(lr.req) {
			required = append(required, lr.resource)
		}
	}
	complete := true
	for _, res := range required {
		if !contains(r.profile, res) {
			r.violation(t, "REQ-CMP-021", nil, "the Profile does not list %s, which level %d requires", res, r.opt.Level)
			complete = false
		}
	}
	var listed, unlisted []*exchange
	for _, res := range Gettable {
		if res == "Profile" {
			continue
		}
		x, err := r.get(ctx, t, r.st, res, r.app, "", nil)
		if err != nil {
			return err
		}
		if contains(r.profile, res) {
			listed = append(listed, x)
		} else {
			unlisted = append(unlisted, x)
		}
	}
	if err := r.wait(ctx, listed, unlisted); err != nil {
		return err
	}
	exact := true
	for _, x := range listed {
		if len(r.reply(t, x)) == 0 {
			r.violation(t, "REQ-RES-PROFILE-002", x, "the Profile lists %s, but Get %s/<AppId> (MessageId %s) is not answered within %s", x.resource, x.resource, x.id, r.opt.Timeout)
			exact = false
		}
	}
	for _, x := range unlisted {
		if answers, _, _ := r.snapshot(x); len(answers) > 0 {
			r.violation(t, "REQ-RES-PROFILE-002", x, "the Profile does not list %s, but Get %s/<AppId> (MessageId %s) is answered", x.resource, x.resource, x.id)
			r.violation(t, "REQ-CMP-021", x, "%s is supported (its Get is answered) but not listed in the Profile", x.resource)
			exact, complete = false, false
		}
	}
	if complete {
		r.checked(t, "REQ-CMP-021", nil)
	}
	if exact {
		r.checked(t, "REQ-RES-PROFILE-002", nil)
	}
	return nil
}

func random() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// get05 is T7-GET-05: a Get without content is answered with an empty list
// (REQ-CMP-041); a Get for an unknown asset gets no Pub, but an Event
// Status Bad under the AppId (REQ-CMP-047).
func (r *Runner) get05(ctx context.Context) error {
	const t = "T7-GET-05"
	res := ""
	for _, c := range []string{"Data", "Config"} {
		if r.hasProfile && contains(r.profile, c) {
			res = c
			break
		}
	}
	var want []*exchange
	var empty *exchange
	filter := "oecCheckNone" + random()
	if res != "" {
		x, err := r.get(ctx, t, r.st, res, r.app, filter, nil)
		if err != nil {
			return err
		}
		empty, want = x, append(want, x)
	}
	unknown := "oi4.org/oec-check/D7/unknown-" + random()
	ux, err := r.get(ctx, t, r.st, "MAM", unknown, "", nil)
	if err != nil {
		return err
	}
	if err := r.wait(ctx, want, []*exchange{ux}); err != nil {
		return err
	}
	if empty == nil {
		r.skip(t, "REQ-CMP-041", "the Profile lists neither Data nor Config (no resource with a Filter to request without content)")
	} else if ns := r.reply(t, empty, "REQ-CMP-041"); len(ns) > 0 {
		n := len(sources(ns))
		if n > 0 {
			r.violation(t, "REQ-CMP-041", empty, "Get %s with the Filter %q, which matches nothing, is answered with %d DataSetMessage(s) instead of an empty list", res, filter, n)
		} else {
			r.checked(t, "REQ-CMP-041", empty)
		}
	}
	r.skip(t, "REQ-CMP-044", "not stimulated: D7 requests Metadata only for Data announced in the PublicationList (REQ-CMP-044 is a MAY)")
	answers, events, others := r.snapshot(ux)
	ok := true
	for _, a := range append(answers, others...) {
		r.violation(t, "REQ-CMP-047", ux, "Get MAM for the unknown Source %q is answered on %s", unknown, a.topic)
		ok = false
	}
	statusBad := Topic(r.st, r.app, "Pub", "Event", r.app, "Status/Bad")
	found := false
	for _, e := range events {
		if e.topic != statusBad {
			r.violation(t, "REQ-CMP-047", ux, "the Event for the unknown Source is published on %s instead of %s", e.topic, statusBad)
			ok = false
			continue
		}
		found = true
		for _, d := range e.content() {
			p, _ := d.payload.(map[string]any)
			if n, _ := p["Number"].(float64); int64(n) != BadNotFound {
				r.violation(t, "REQ-CMP-047", ux, "the Event Status Bad for the unknown Source has Number %v instead of BadNotFound (%d)", p["Number"], BadNotFound)
				ok = false
			}
		}
	}
	if !found {
		r.violation(t, "REQ-CMP-047", ux, "no Event Status Bad with the ReplyTo %s on %s within %s", ux.id, statusBad, r.opt.Timeout)
		ok = false
	}
	if ok {
		r.checked(t, "REQ-CMP-047", ux)
	}
	return nil
}

// get09 is T7-GET-09: a Get MAM with an unknown key is answered like one
// without (REQ-MSG-046).
func (r *Runner) get09(ctx context.Context) error {
	const t = "T7-GET-09"
	x, err := r.get(ctx, t, r.st, "MAM", r.app, "", map[string]any{UnknownKey: "D7 checks that receivers ignore unknown keys"})
	if err != nil {
		return err
	}
	if err := r.wait(ctx, []*exchange{x}, nil); err != nil {
		return err
	}
	if len(r.reply(t, x)) > 0 {
		r.checked(t, "REQ-MSG-046", x)
		return nil
	}
	if !r.mamAnswered {
		r.skip(t, "REQ-MSG-046", "the Get MAM/<AppId> without unknown key is not answered either")
		return nil
	}
	r.violation(t, "REQ-MSG-046", x, "Get MAM/<AppId> with the unknown key %s (MessageId %s) is not answered within %s, the same Get without it is", UnknownKey, x.id, r.opt.Timeout)
	return nil
}
