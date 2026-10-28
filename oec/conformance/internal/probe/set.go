package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/OI4/oec/conformance/internal/verdict"
)

// The write side of the runner (P5 increment 3, C-064, DESIGN.md §3.3):
// T7-SET-01 … 09, 11 and 12. Every test restores the state of the CUT:
// it writes back what it read, deletes only what it created and undoes a
// change it provoked. The state before the first and after the last write
// test is compared (Restore).

// eventReq and pubReq are the requirements of a resource on the Status
// event after a Set or Del and on the publication after a successful Set.
var eventReq = map[string]string{
	"Config": "REQ-RES-CONFIG-021", "Data": "REQ-RES-DATA-011", "Metadata": "REQ-RES-META-006",
	"PublicationList": "REQ-RES-PUBLIST-024", "ReferenceDesignation": "REQ-RES-REFDES-009", "SubscriptionList": "REQ-RES-SUBLIST-013",
}

var pubReq = map[string]string{
	"Config": "REQ-RES-CONFIG-022", "Data": "REQ-RES-DATA-012", "Metadata": "REQ-RES-META-007",
	"PublicationList": "REQ-RES-PUBLIST-025", "ReferenceDesignation": "REQ-RES-REFDES-010", "SubscriptionList": "REQ-RES-SUBLIST-014",
}

// StatusCodes the runner distinguishes (MG 8.3.6.1).
const (
	badNotWritable  = 0x803B0000
	badNotSupported = 0x803D0000
)

// Mask is the published value of a sensitive configuration object
// (REQ-RES-CONFIG-024).
const Mask = "********"

// stateResources are compared before and after the write tests; Data is
// not (its values change by themselves).
var stateResources = []string{"Config", "PublicationList", "ReferenceDesignation", "SubscriptionList"}

// Restore is the result of the restore check: the answers to Get without
// Source of stateResources before the first and after the last write test.
type Restore struct {
	Checked  bool     `json:"checked"`
	Restored bool     `json:"restored"`
	Changed  []string `json:"changed,omitempty"` // "- " before, "+ " after: resource, Source, WriterGroupName, DataSetWriterId, Payload
	Note     string   `json:"note,omitempty"`
}

// Summary is the one-line result of the restore check.
func (rs Restore) Summary() string {
	note := ""
	if rs.Note != "" {
		note = " (" + rs.Note + ")"
	}
	switch {
	case !rs.Checked:
		return "restore not checked: " + rs.Note
	case rs.Restored:
		return "restore: the state of the CUT after the write tests equals the state before them" + note
	}
	return fmt.Sprintf("restore: the state of the CUT after the write tests differs from the state before them (%d line(s))%s", len(rs.Changed), note)
}

type state struct {
	dsms     map[string][]dsm
	answered map[string]bool
	lines    []string
}

// readState reads stateResources with Get without Source; with the label
// as test of the requests.
func (r *Runner) readState(ctx context.Context, label string) (*state, error) {
	xs := map[string]*exchange{}
	var all []*exchange
	for _, res := range stateResources {
		if r.hasProfile && !contains(r.profile, res) {
			continue
		}
		x, err := r.get(ctx, label, r.st, res, "", "", nil)
		if err != nil {
			return nil, err
		}
		xs[res] = x
		all = append(all, x)
	}
	if err := r.until(ctx, r.completeAll(all...)); err != nil {
		return nil, err
	}
	st := &state{dsms: map[string][]dsm{}, answered: map[string]bool{}}
	for res, x := range xs {
		answers, _, _ := r.snapshot(x)
		var ns []*netMsg
		for _, a := range answers {
			if a.topic == x.answerTopic {
				ns = append(ns, a)
			}
		}
		ds, why := series(ns)
		if why != "" {
			continue
		}
		st.answered[res] = true
		st.dsms[res] = ds
		for _, d := range ds {
			b, _ := json.Marshal(d.payload)
			st.lines = append(st.lines, fmt.Sprintf("%s %s %s %d %s", res, d.name, d.group, d.id, b))
		}
	}
	sort.Strings(st.lines)
	return st, nil
}

// prepareWrite reads what the write tests need: the Profile, the assets
// (Get MAM without Source) and the state before the first write.
func (r *Runner) prepareWrite(ctx context.Context) error {
	if !r.hasProfile {
		if err := r.readProfile(ctx, ""); err != nil {
			return err
		}
	}
	if len(r.mamAll) == 0 {
		x, err := r.get(ctx, "state", r.st, "MAM", "", "", nil)
		if err != nil {
			return err
		}
		if err := r.until(ctx, r.completeAll(x)); err != nil {
			return err
		}
		answers, _, _ := r.snapshot(x)
		ds, _ := series(answers)
		for _, d := range ds {
			if !contains(r.mamAll, d.name) {
				r.mamAll = append(r.mamAll, d.name)
			}
		}
	}
	st, err := r.readState(ctx, "state")
	if err != nil {
		return err
	}
	r.before = st
	return nil
}

// checkRestore compares the state after the write tests with the state
// before them.
func (r *Runner) checkRestore(ctx context.Context) error {
	after, err := r.readState(ctx, "state")
	if err != nil {
		return err
	}
	res := &Restore{Checked: len(r.before.answered) > 0}
	if !res.Checked {
		res.Note = "the CUT answered no Get without Source of " + strings.Join(stateResources, ", ")
	}
	for _, d := range r.before.dsms["Config"] {
		if masked(payloadOf(d)) {
			res.Note = "sensitive configuration values are published masked: the restore check cannot compare them"
			break
		}
	}
	for _, res2 := range stateResources {
		if r.before.answered[res2] != after.answered[res2] {
			res.Changed = append(res.Changed, fmt.Sprintf("%s answered before %t, after %t", res2, r.before.answered[res2], after.answered[res2]))
		}
	}
	in := func(l []string, x string) bool {
		i := sort.SearchStrings(l, x)
		return i < len(l) && l[i] == x
	}
	for _, x := range r.before.lines {
		if !in(after.lines, x) && after.answered[strings.SplitN(x, " ", 2)[0]] {
			res.Changed = append(res.Changed, "- "+x)
		}
	}
	for _, x := range after.lines {
		if !in(r.before.lines, x) && r.before.answered[strings.SplitN(x, " ", 2)[0]] {
			res.Changed = append(res.Changed, "+ "+x)
		}
	}
	res.Restored = res.Checked && len(res.Changed) == 0
	r.restore = res
	return nil
}

// msg is the DataSetMessage of a request built from a received one.
func msg(d dsm) map[string]any {
	return map[string]any{"DataSetWriterId": d.id, "WriterGroupName": d.group, "DataSetWriterName": d.name, "Payload": d.payload}
}

// write sends a Set or Del (ua-data) with the DataSetMessages.
func (r *Runner) write(ctx context.Context, test, method, resource, source, filter string, msgs ...map[string]any) (*exchange, error) {
	id := r.nextID()
	list := []any{}
	for _, m := range msgs {
		list = append(list, m)
	}
	doc := map[string]any{"MessageId": id, "MessageType": "ua-data", "PublisherId": r.tester, "ReplyTo": "", "Messages": list}
	if c, ok := Classes[resource]; ok {
		doc["DataSetClassId"] = c
	}
	return r.post(ctx, test, method, resource, source, filter, id, doc)
}

// setMeta sends a Set Metadata (ua-metadata, MG 9.1.5).
func (r *Runner) setMeta(ctx context.Context, test, source, tag string, writer int64, md any) (*exchange, error) {
	id := r.nextID()
	doc := map[string]any{"MessageId": id, "MessageType": "ua-metadata", "PublisherId": r.tester, "DataSetWriterId": writer,
		"WriterGroupName": tag, "DataSetWriterName": source, "Timestamp": time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"ReplyTo": "", "MetaData": md}
	return r.post(ctx, test, "Set", "Metadata", source, tag, id, doc)
}

func (r *Runner) post(ctx context.Context, test, method, resource, source, filter, id string, doc map[string]any) (*exchange, error) {
	payload, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	x := &exchange{test: test, id: id, method: method, resource: resource, source: source, filter: filter, sent: time.Now(),
		topic: Topic(r.st, r.app, method, resource, source, filter), answerTopic: Topic(r.st, r.app, "Pub", resource, source, filter)}
	r.mu.Lock()
	r.exchanges = append(r.exchanges, x)
	r.mu.Unlock()
	if err := r.pub(ctx, x.topic, payload); err != nil {
		return nil, fmt.Errorf("publish %s: %w", x.topic, err)
	}
	return x, nil
}

// settle waits until every exchange has an Event with its ReplyTo (plus
// the grace time) or the timeout ended.
func (r *Runner) settle(ctx context.Context, xs ...*exchange) error {
	return r.until(ctx, func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, x := range xs {
			if len(x.events()) == 0 {
				return false
			}
		}
		return true
	})
}

// quiet waits until x has its Status event and then the grace time again:
// the time in which nothing may be published (REQ-BUS-007).
func (r *Runner) quiet(ctx context.Context, x *exchange) error {
	if err := r.settle(ctx, x); err != nil {
		return err
	}
	return sleep(ctx, r.opt.Grace)
}

// level is the EventLevel of a StatusCode (its severity bits).
func level(code int64) string {
	switch code >> 30 & 3 {
	case 0:
		return "Good"
	case 1:
		return "Uncertain"
	}
	return "Bad"
}

// status is the Status event about a request.
type status struct {
	got    bool
	code   int64
	source string // decoded Source of the event topic
	level  string // last level of the event topic
	n      *netMsg
}

func (s status) good() bool { return s.got && level(s.code) == "Good" }
func (s status) bad() bool  { return s.got && level(s.code) == "Bad" }

func (s status) String() string {
	if !s.got {
		return "no Status event"
	}
	return fmt.Sprintf("Status %s (Number 0x%08X)", s.level, s.code)
}

// statusOf returns the first Event of the category Status with the
// MessageId of x as ReplyTo.
func (r *Runner) statusOf(x *exchange) status {
	_, events, _ := r.snapshot(x)
	for _, e := range events {
		lv := strings.Split(e.topic, "/")
		if len(lv) != 8 || lv[6] != "Status" {
			continue
		}
		for _, d := range e.content() {
			p, _ := d.payload.(map[string]any)
			if f, ok := p["Number"].(float64); ok {
				return status{got: true, code: int64(f), source: dec(lv[5]), level: lv[7], n: e}
			}
		}
	}
	return status{}
}

// eventSource is the Source of the Status event about a request: the
// Source of the request, for an asset the entity neither is nor
// represents its AppId (REQ-CMP-043, SF-108).
func (r *Runner) eventSource(source string) string {
	if source == r.app || contains(r.mamAll, source) {
		return source
	}
	return r.app
}

func (r *Runner) checkedf(test, req string, x *exchange, format string, a ...any) {
	r.add(verdict.Observation{Test: test, Req: req, Kind: verdict.Checked, Evidence: r.ev(x, format, a...)})
}

func what(x *exchange) string {
	return fmt.Sprintf("%s %s (MessageId %s)", x.method, x.resource, x.id)
}

// judge judges the reaction to a Set or Del: the Status event
// (REQ-TOP-014/015, REQ-CMP-023, REQ-CMP-038, REQ-RES-EVENT-005 and the
// event requirement of the resource) and, after a successful Set, the
// publication of the changed object (REQ-BUS-006, REQ-MQTT-019,
// REQ-MSG-015 and the publication requirement of the resource). The
// catalog decides which of them the test judges (add).
func (r *Runner) judge(test string, x *exchange) status {
	s := r.statusOf(x)
	top := "REQ-TOP-014"
	if x.method == "Del" {
		top = "REQ-TOP-015"
	}
	exact := []string{"REQ-RES-EVENT-005"}
	if q := eventReq[x.resource]; q != "" {
		exact = append(exact, q)
	}
	if !s.got {
		for _, q := range append([]string{top, "REQ-CMP-023"}, exact...) {
			r.violation(test, q, x, "%s: no Event of the category Status with this ReplyTo within %s", what(x), r.opt.Timeout)
		}
	} else {
		for _, q := range []string{top, "REQ-CMP-023", "REQ-CMP-038"} {
			r.checkedf(test, q, x, "%s: %s", what(x), s)
		}
		var problems []string
		if want := r.eventSource(x.source); s.source != want {
			problems = append(problems, fmt.Sprintf("the Status event is published under the Source %q instead of %q", s.source, want))
		}
		if s.level != level(s.code) {
			problems = append(problems, fmt.Sprintf("the event topic has the level %s, but Number 0x%08X is %s", s.level, s.code, level(s.code)))
		}
		if want := r.st + "/" + r.app; s.n.publisherID != want {
			problems = append(problems, fmt.Sprintf("the Status event has the PublisherId %q instead of %q", s.n.publisherID, want))
		}
		for _, q := range exact {
			if len(problems) == 0 {
				r.checkedf(test, q, x, "%s: %s on %s", what(x), s, s.n.topic)
			}
			for _, p := range problems {
				r.violation(test, q, x, "%s: %s", what(x), p)
			}
		}
	}
	if x.method == "Set" && (s.good() || !s.got) {
		r.judgePub(test, x, s)
	}
	return s
}

// judgePub judges the publication of the changed object after a Set: on
// the Pub topic with Source and Filter of the Set and its MessageId as
// ReplyTo. Without Status event, only a publication is judged.
func (r *Runner) judgePub(test string, x *exchange, s status) {
	answers, _, others := r.snapshot(x)
	reqs := []string{"REQ-BUS-006", "REQ-MQTT-019"}
	if q := pubReq[x.resource]; q != "" {
		reqs = append(reqs, q)
	}
	var ok, wrong []*netMsg
	for _, a := range answers {
		if a.topic == x.answerTopic {
			ok = append(ok, a)
		} else {
			wrong = append(wrong, a)
		}
	}
	switch {
	case len(ok) > 0:
		for _, q := range append(reqs, "REQ-MSG-015") {
			r.checkedf(test, q, x, "%s: published on %s with this ReplyTo", what(x), x.answerTopic)
		}
	case len(wrong) > 0:
		for _, q := range reqs {
			r.violation(test, q, x, "%s: the changed object is published on %s instead of %s", what(x), wrong[0].topic, x.answerTopic)
		}
	case !s.got:
	default:
		if len(others) > 0 {
			r.violation(test, "REQ-MSG-015", x, "%s: %d publication(s) on %s after the Set, with ReplyTo %q instead of the MessageId of the Set", what(x), len(others), x.answerTopic, others[0].replyTo)
		}
		for _, q := range reqs {
			r.violation(test, q, x, "%s: answered with %s, but nothing is published on %s with this ReplyTo within %s", what(x), s, x.answerTopic, r.opt.Timeout)
		}
	}
}

// published returns what is published on the Pub topic of x after the
// request (with any ReplyTo; Events excluded).
func (r *Runner) published(x *exchange) []*netMsg {
	answers, _, others := r.snapshot(x)
	var out []*netMsg
	for _, a := range answers {
		if a.topic == x.answerTopic {
			out = append(out, a)
		}
	}
	return append(out, others...)
}

// pubEntry is a PublicationList entry with its Config.
type pubEntry struct {
	entry
	config string
	d      dsm
}

func (r *Runner) pubEntries() []pubEntry {
	var out []pubEntry
	for _, d := range r.before.dsms["PublicationList"] {
		es := entries([]dsm{d})
		if len(es) == 0 {
			continue
		}
		p, _ := d.payload.(map[string]any)
		c, _ := p["Config"].(string)
		if c == "" {
			c = "NONE_0"
		}
		out = append(out, pubEntry{entry: es[0], config: c, d: d})
	}
	return out
}

// subConfig is the Config of a SubscriptionList entry (default NONE_0).
func subConfig(d dsm) string {
	p, _ := d.payload.(map[string]any)
	if c, _ := p["Config"].(string); c != "" {
		return c
	}
	return "NONE_0"
}

// setSubs returns the Set subscriptions the CUT lists for its Data and
// Config DataSets (REQ-RES-SUBLIST-016) as "<Resource>|<Source>|<Tag>".
func (r *Runner) setSubs() map[string]bool {
	out := map[string]bool{}
	prefix := "Oi4v2/" + r.st + "/" + enc(r.app) + "/Set/"
	for _, d := range r.before.dsms["SubscriptionList"] {
		p, _ := d.payload.(map[string]any)
		tp, _ := p["TopicPath"].(string)
		rest, ok := strings.CutPrefix(tp, prefix)
		if !ok {
			continue
		}
		lv := strings.SplitN(rest, "/", 3)
		if len(lv) == 3 && (lv[0] == "Data" || lv[0] == "Config") {
			out[lv[0]+"|"+dec(lv[1])+"|"+lv[2]] = true
		}
	}
	return out
}

// currentData reads the Data DataSets with a listed Set subscription.
func (r *Runner) currentData(ctx context.Context, test string) ([]dsm, error) {
	var keys []string
	for k := range r.setSubs() {
		if strings.HasPrefix(k, "Data|") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var xs []*exchange
	for _, k := range keys {
		lv := strings.SplitN(k, "|", 3)
		x, err := r.get(ctx, test, r.st, "Data", lv[1], lv[2], nil)
		if err != nil {
			return nil, err
		}
		xs = append(xs, x)
	}
	if len(xs) == 0 {
		return nil, nil
	}
	if err := r.wait(ctx, xs, nil); err != nil {
		return nil, err
	}
	var out []dsm
	for _, x := range xs {
		answers, _, _ := r.snapshot(x)
		for _, a := range answers {
			for _, d := range a.content() {
				if a.topic == x.answerTopic && d.name == x.source && d.group == x.filter {
					out = append(out, d)
					break
				}
			}
		}
	}
	return out, nil
}

// subEntry is a SubscriptionList entry D7 creates: a subscription to a
// Data topic of the tester (n distinguishes the Source).
func (r *Runner) subEntry(filter string, n int) map[string]any {
	var id int64 = 10
	if ds := r.before.dsms["SubscriptionList"]; len(ds) > 0 {
		id = ds[0].id
	}
	tag := strings.TrimPrefix(filter, "Data/")
	src := fmt.Sprintf("%s-%d", r.opt.TesterIL, n)
	return map[string]any{"DataSetWriterId": id, "WriterGroupName": filter, "DataSetWriterName": r.app,
		"Payload": map[string]any{"TopicPath": Topic("Utility", r.opt.TesterIL, "Pub", "Data", src, tag), "Interval": 0, "Config": "CONF_1"}}
}

func payloadOf(d dsm) map[string]any {
	c, _ := clone(d.payload).(map[string]any)
	if c == nil {
		c = map[string]any{}
	}
	return c
}

func clone(v any) any {
	b, _ := json.Marshal(v)
	var c any
	_ = json.Unmarshal(b, &c)
	return c
}

func with(d dsm, payload map[string]any) map[string]any {
	m := msg(d)
	m["Payload"] = payload
	return m
}

// absent is the reason of a skip for want missing in the state of res.
func (r *Runner) absent(res, want string) string {
	if !r.before.answered[res] {
		return "the CUT does not answer Get " + res + " without Source"
	}
	return "the CUT has no " + want
}

// set01 is T7-SET-01: every object the CUT offers for modification answers
// a Set with a Status event and the publication of the object; the Set
// writes back what the CUT published. A Data or Config DataSet that
// accepts the Set is listed with its Set subscription, a listed one
// accepts it (REQ-RES-SUBLIST-016). A Del of an unknown subscription
// stimulates REQ-TOP-015.
func (r *Runner) set01(ctx context.Context) error {
	const t = "T7-SET-01"
	subs := r.setSubs()
	listing := r.before.answered["SubscriptionList"]
	var xs []*exchange
	listed := map[*exchange]bool{}
	for _, d := range r.before.dsms["Config"] {
		if r.unsafeMask(d) {
			continue
		}
		x, err := r.write(ctx, t, "Set", "Config", d.name, d.group, msg(d))
		if err != nil {
			return err
		}
		listed[x] = subs["Config|"+d.name+"|"+d.group]
		xs = append(xs, x)
	}
	data, err := r.currentData(ctx, t)
	if err != nil {
		return err
	}
	for _, d := range data {
		x, err := r.write(ctx, t, "Set", "Data", d.name, d.group, msg(d))
		if err != nil {
			return err
		}
		listed[x] = true
		xs = append(xs, x)
	}
	for _, e := range r.pubEntries() {
		if e.config != "NONE_0" {
			x, err := r.write(ctx, t, "Set", "PublicationList", e.d.name, e.d.group, msg(e.d))
			if err != nil {
				return err
			}
			xs = append(xs, x)
		}
	}
	for _, d := range r.before.dsms["SubscriptionList"] {
		if subConfig(d) == "CONF_1" {
			x, err := r.write(ctx, t, "Set", "SubscriptionList", d.name, d.group, msg(d))
			if err != nil {
				return err
			}
			xs = append(xs, x)
		}
	}
	for _, d := range r.before.dsms["ReferenceDesignation"] {
		x, err := r.write(ctx, t, "Set", "ReferenceDesignation", d.name, "", msg(d))
		if err != nil {
			return err
		}
		xs = append(xs, x)
	}
	filter := "Data/d7-" + random()
	del, err := r.write(ctx, t, "Del", "SubscriptionList", r.app, filter, r.subEntry(filter, 1))
	if err != nil {
		return err
	}
	xs = append(xs, del)
	if err := r.settle(ctx, xs...); err != nil {
		return err
	}
	events, offered := 0, 0
	for _, x := range xs {
		s := r.judge(t, x)
		if s.got {
			events++
		}
		if x.resource != "Config" && x.resource != "Data" {
			continue
		}
		obj := fmt.Sprintf("%s/%s/%s", x.resource, x.source, x.filter)
		switch {
		case listed[x]:
			offered++
			switch {
			case s.good():
				r.checkedf(t, "REQ-RES-SUBLIST-016", x, "%s is listed with its Set subscription and accepts the Set", obj)
				r.checkedf(t, "REQ-CMP-016", x, "%s accepts the Set", obj)
			case s.got && (s.code == badNotWritable || s.code == badNotSupported):
				r.violation(t, "REQ-RES-SUBLIST-016", x, "%s is listed with its Set subscription, but the Set (MessageId %s) is answered with %s", obj, x.id, s)
				r.violation(t, "REQ-CMP-016", x, "%s is offered for modification (Set subscription listed), but the Set (MessageId %s) is answered with %s", obj, x.id, s)
			default:
				r.skip(t, "REQ-RES-SUBLIST-016", "the write-back of %s is answered with %s", obj, s)
				r.skip(t, "REQ-CMP-016", "the write-back of %s is answered with %s", obj, s)
			}
		case !listing:
			r.skip(t, "REQ-RES-SUBLIST-016", "the CUT does not answer Get SubscriptionList")
		case s.good():
			r.violation(t, "REQ-RES-SUBLIST-016", x, "%s accepts the Set (MessageId %s), but the SubscriptionList does not list its Set subscription", obj, x.id)
		default:
			r.checkedf(t, "REQ-RES-SUBLIST-016", x, "%s is not listed and does not accept the Set (%s)", obj, s)
		}
	}
	if offered == 0 {
		r.skip(t, "REQ-CMP-016", "%s", r.absent("SubscriptionList", "Set subscription of a Data or Config DataSet in its SubscriptionList (offers no object for modification)"))
	}
	if events == 0 {
		r.violation(t, "REQ-CMP-038", nil, "none of %d Set and Del requests is answered with an Event", len(xs))
	}
	return nil
}

// set02 is T7-SET-02: a Set or Del of an object the CUT does not have is
// answered with Status Bad; for an unknown asset under the AppId
// (REQ-CMP-043, SF-108).
func (r *Runner) set02(ctx context.Context) error {
	const t = "T7-SET-02"
	unknown := "oi4.org/oec-check/Unknown/" + random()
	tag := "D7Unknown-" + random()
	a, err := r.write(ctx, t, "Set", "Data", unknown, tag, map[string]any{"DataSetWriterId": 10, "WriterGroupName": tag, "DataSetWriterName": unknown, "Payload": map[string]any{"D7": 1}})
	if err != nil {
		return err
	}
	b, err := r.write(ctx, t, "Set", "Data", r.app, tag, map[string]any{"DataSetWriterId": 10, "WriterGroupName": tag, "DataSetWriterName": r.app, "Payload": map[string]any{"D7": 1}})
	if err != nil {
		return err
	}
	filter := "Data/d7-" + random()
	c, err := r.write(ctx, t, "Del", "SubscriptionList", r.app, filter, r.subEntry(filter, 1))
	if err != nil {
		return err
	}
	if err := r.settle(ctx, a, b, c); err != nil {
		return err
	}
	for _, x := range []*exchange{a, b, c} {
		s := r.statusOf(x)
		obj := fmt.Sprintf("%s %s/%s/%s (MessageId %s)", x.method, x.resource, x.source, x.filter, x.id)
		switch {
		case !s.got:
			r.violation(t, "REQ-CMP-043", x, "%s of an unknown object: no Status event with this ReplyTo within %s", obj, r.opt.Timeout)
		case level(s.code) != "Bad" || s.level != "Bad":
			r.violation(t, "REQ-CMP-043", x, "%s of an unknown object is answered with %s instead of Bad", obj, s)
		case s.source != r.app:
			r.violation(t, "REQ-CMP-043", x, "%s of an unknown object: the Status event is published under the Source %q instead of the AppId (SF-108)", obj, s.source)
		default:
			r.checkedf(t, "REQ-CMP-043", x, "%s of an unknown object: %s", obj, s)
		}
	}
	return nil
}

// set04 is T7-SET-04: after the Del of a whole object nothing is
// published (REQ-BUS-007), after a partial Del the rest (REQ-BUS-008).
// The whole object is a ReferenceDesignation (restored afterwards) and
// the last of two subscriptions D7 created; the partial Del deletes the
// first one.
func (r *Runner) set04(ctx context.Context) error {
	const t = "T7-SET-04"
	judged := false
	defer func() {
		if !judged {
			r.skip(t, "REQ-BUS-007", "no Del of a whole object was accepted (no ReferenceDesignation object D7 can restore, no SubscriptionList entry created)")
		}
	}()
	whole := func(x *exchange, obj string) {
		if s := r.statusOf(x); s.good() {
			judged = true
			if ps := r.published(x); len(ps) > 0 {
				r.violation(t, "REQ-BUS-007", x, "after the Del of %s (MessageId %s), %d publication(s) on %s", obj, x.id, len(ps), x.answerTopic)
			} else {
				r.checkedf(t, "REQ-BUS-007", x, "nothing published on %s after the Del of %s", x.answerTopic, obj)
			}
		}
	}
	if ds := r.before.dsms["ReferenceDesignation"]; len(ds) > 0 {
		d := ds[0]
		ok, err := r.ensureRestorable(ctx, t, d)
		if err != nil {
			return err
		}
		if ok {
			x, err := r.write(ctx, t, "Del", "ReferenceDesignation", d.name, "")
			if err != nil {
				return err
			}
			if err := r.quiet(ctx, x); err != nil {
				return err
			}
			whole(x, "the ReferenceDesignation of "+d.name)
			if s := r.statusOf(x); !s.bad() {
				y, err := r.write(ctx, t, "Set", "ReferenceDesignation", d.name, "", msg(d))
				if err != nil {
					return err
				}
				if err := r.settle(ctx, y); err != nil {
					return err
				}
			}
		}
	}
	filter := "Data/d7-" + random()
	e1, e2 := r.subEntry(filter, 1), r.subEntry(filter, 2)
	x1, err := r.write(ctx, t, "Set", "SubscriptionList", r.app, filter, e1)
	if err != nil {
		return err
	}
	if err := r.settle(ctx, x1); err != nil {
		return err
	}
	if s := r.statusOf(x1); !s.good() {
		r.skip(t, "REQ-BUS-008", "the CUT does not create a SubscriptionList entry (Set answered with %s, SF-111)", s)
		if s.bad() {
			return nil
		}
		return r.remove(ctx, t, filter, e1)
	}
	x2, err := r.write(ctx, t, "Set", "SubscriptionList", r.app, filter, e2)
	if err != nil {
		return err
	}
	if err := r.settle(ctx, x2); err != nil {
		return err
	}
	if s := r.statusOf(x2); !s.good() {
		r.skip(t, "REQ-BUS-008", "the CUT does not create a second SubscriptionList entry (Set answered with %s)", s)
		if !s.bad() {
			if err := r.remove(ctx, t, filter, e2); err != nil {
				return err
			}
		}
		return r.remove(ctx, t, filter, e1)
	}
	d1, err := r.write(ctx, t, "Del", "SubscriptionList", r.app, filter, e1)
	if err != nil {
		return err
	}
	if err := r.settle(ctx, d1); err != nil {
		return err
	}
	if s := r.statusOf(d1); s.good() {
		answers, _, _ := r.snapshot(d1)
		rest := false
		path := e2["Payload"].(map[string]any)["TopicPath"]
		for _, a := range answers {
			for _, d := range a.content() {
				if p, _ := d.payload.(map[string]any); a.topic == d1.answerTopic && p["TopicPath"] == path {
					rest = true
				}
			}
		}
		if rest {
			r.checkedf(t, "REQ-BUS-008", d1, "the remaining subscription is published on %s with the ReplyTo of the Del", d1.answerTopic)
		} else {
			r.violation(t, "REQ-BUS-008", d1, "after the Del of one of two subscriptions (MessageId %s), the remaining one is not published on %s with this ReplyTo", d1.id, d1.answerTopic)
		}
	} else {
		r.skip(t, "REQ-BUS-008", "the Del of a subscription D7 created is answered with %s", s)
	}
	d2, err := r.write(ctx, t, "Del", "SubscriptionList", r.app, filter, e2)
	if err != nil {
		return err
	}
	if err := r.quiet(ctx, d2); err != nil {
		return err
	}
	whole(d2, "the last subscription of "+filter)
	return nil
}

// remove deletes a SubscriptionList entry D7 created (or may have created:
// a Set without Status event).
func (r *Runner) remove(ctx context.Context, t, filter string, e map[string]any) error {
	d, err := r.write(ctx, t, "Del", "SubscriptionList", r.app, filter, e)
	if err != nil {
		return err
	}
	return r.settle(ctx, d)
}

// configObjects returns the names "<group>/<object>" of the configuration
// objects of a Config DataSet, sorted.
func configObjects(p map[string]any) []string {
	var out []string
	for gk, gv := range p {
		g, _ := gv.(map[string]any)
		if gk == "Context" || g == nil {
			continue
		}
		for ok, ov := range g {
			if o, _ := ov.(map[string]any); o != nil && o["Type"] != nil {
				out = append(out, gk+"/"+ok)
			}
		}
	}
	sort.Strings(out)
	return out
}

func object(p map[string]any, name string) map[string]any {
	gk, ok, _ := strings.Cut(name, "/")
	g, _ := p[gk].(map[string]any)
	o, _ := g[ok].(map[string]any)
	return o
}

// masked reports whether the Config has a sensitive object published with
// the mask.
func masked(p map[string]any) bool {
	for _, n := range configObjects(p) {
		if o := object(p, n); o["Sensitive"] == true && o["Value"] == Mask {
			return true
		}
	}
	return false
}

// unsafeMask reports whether the Config DataSet has a masked sensitive
// value and the mask in a Set is not MUST at the declared level
// (REQ-RES-CONFIG-025 is SHOULD at level 1): the CUT could store the mask
// instead of the secret, and the restore check could not see it (the
// mask is published either way). D7 does not write such a DataSet.
func (r *Runner) unsafeMask(d dsm) bool {
	return masked(payloadOf(d)) && !r.mandatory("REQ-RES-CONFIG-025")
}

// set05 is T7-SET-05: Set Config writes back every Config DataSet
// (including the mask of sensitive values) and is rejected for an
// unknown object, a changed read-only property and a Value that does not
// represent its Type.
func (r *Runner) set05(ctx context.Context) error {
	const t = "T7-SET-05"
	ds := r.before.dsms["Config"]
	if len(ds) == 0 {
		for _, q := range []string{"REQ-RES-CONFIG-005", "REQ-RES-CONFIG-021", "REQ-RES-CONFIG-022", "REQ-RES-CONFIG-023", "REQ-RES-CONFIG-024", "REQ-RES-CONFIG-025"} {
			r.skip(t, q, "%s", r.absent("Config", "Config DataSet"))
		}
		return nil
	}
	negatives := false
	for _, d := range ds {
		p := payloadOf(d)
		if r.unsafeMask(d) {
			r.skip(t, "REQ-RES-CONFIG-025", "Config/%s/%s has a masked sensitive value: D7 does not write it back at level %d (REQ-RES-CONFIG-025 is not MUST, the CUT could store the mask)", d.name, d.group, r.opt.Level)
			continue
		}
		x, err := r.write(ctx, t, "Set", "Config", d.name, d.group, msg(d))
		if err != nil {
			return err
		}
		if err := r.settle(ctx, x); err != nil {
			return err
		}
		s := r.judge(t, x)
		obj := "Config/" + d.name + "/" + d.group
		if s.good() {
			for _, a := range r.published(x) {
				for _, c := range a.content() {
					cp, _ := c.payload.(map[string]any)
					for _, n := range configObjects(cp) {
						if o := object(cp, n); o["Sensitive"] == true {
							if v, _ := o["Value"].(string); v != "" && v != Mask {
								r.violation(t, "REQ-RES-CONFIG-024", x, "%s: the sensitive object %s is published after the Set with an unmasked Value", obj, n)
							} else {
								r.checkedf(t, "REQ-RES-CONFIG-024", x, "%s: the sensitive object %s is masked after the Set", obj, n)
							}
						}
					}
				}
			}
		}
		if masked(p) {
			switch {
			case s.good():
				r.checkedf(t, "REQ-RES-CONFIG-025", x, "%s: the write-back with the mask of a sensitive object is accepted", obj)
			case s.got && s.code != badNotWritable:
				r.violation(t, "REQ-RES-CONFIG-025", x, "%s: the write-back with the mask of a sensitive object is rejected with %s", obj, s)
			}
		} else if s.got && !s.good() && s.code != badNotWritable {
			r.violation(t, "REQ-RES-CONFIG-023", x, "%s: the unchanged Config DataSet (write-back) is rejected with %s", obj, s)
		}
		if !s.good() || negatives {
			continue
		}
		negatives = true
		objs := configObjects(p)
		if len(objs) == 0 {
			continue
		}
		type neg struct {
			what string
			reqs []string
			p    map[string]any
		}
		var ns []neg
		unknown := payloadOf(d)
		g, _, _ := strings.Cut(objs[0], "/")
		unknown[g].(map[string]any)["d7Unknown"] = map[string]any{"Name": map[string]any{"Locale": "en-US", "Text": "D7"}, "Type": "String", "Value": "D7"}
		ns = append(ns, neg{"an unknown configuration object " + g + "/d7Unknown", []string{"REQ-RES-CONFIG-005", "REQ-RES-CONFIG-023"}, unknown})
		renamed := payloadOf(d)
		object(renamed, objs[0])["Name"] = map[string]any{"Locale": "en-US", "Text": "D7 changed"}
		ns = append(ns, neg{"a changed Name of " + objs[0], []string{"REQ-RES-CONFIG-023"}, renamed})
		for _, n := range objs {
			if object(p, n)["Type"] == "Number" {
				wrong := payloadOf(d)
				object(wrong, n)["Value"] = "D7-not-a-number"
				ns = append(ns, neg{"the Value \"D7-not-a-number\" of the Number " + n, []string{"REQ-RES-CONFIG-023"}, wrong})
				break
			}
		}
		for _, n := range ns {
			y, err := r.write(ctx, t, "Set", "Config", d.name, d.group, with(d, n.p))
			if err != nil {
				return err
			}
			if err := r.settle(ctx, y); err != nil {
				return err
			}
			s := r.judge(t, y)
			for _, q := range n.reqs {
				switch {
				case s.bad():
					r.checkedf(t, q, y, "%s: a Set with %s is rejected with %s", obj, n.what, s)
				case s.got:
					r.violation(t, q, y, "%s: a Set with %s is answered with %s instead of Bad", obj, n.what, s)
				}
			}
			if !s.bad() {
				z, err := r.write(ctx, t, "Set", "Config", d.name, d.group, msg(d))
				if err != nil {
					return err
				}
				if err := r.settle(ctx, z); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// set06 is T7-SET-06: Set Data writes back the Data DataSets the CUT
// offers for Set (listed Set subscription); without one, a Set of an
// unknown tag stimulates the Status event.
func (r *Runner) set06(ctx context.Context) error {
	const t = "T7-SET-06"
	data, err := r.currentData(ctx, t)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		tag := "D7Unknown-" + random()
		x, err := r.write(ctx, t, "Set", "Data", r.app, tag, map[string]any{"DataSetWriterId": 10, "WriterGroupName": tag, "DataSetWriterName": r.app, "Payload": map[string]any{"D7": 1}})
		if err != nil {
			return err
		}
		if err := r.settle(ctx, x); err != nil {
			return err
		}
		r.judge(t, x)
		r.skip(t, "REQ-RES-DATA-012", "%s", r.absent("SubscriptionList", "Set subscription of a Data DataSet in its SubscriptionList (offers no Data DataSet for Set)"))
		return nil
	}
	var xs []*exchange
	for _, d := range data {
		x, err := r.write(ctx, t, "Set", "Data", d.name, d.group, msg(d))
		if err != nil {
			return err
		}
		xs = append(xs, x)
	}
	if err := r.settle(ctx, xs...); err != nil {
		return err
	}
	for _, x := range xs {
		if s := r.judge(t, x); s.got && !s.good() {
			r.skip(t, "REQ-RES-DATA-012", "Set Data/%s/%s is answered with %s", x.source, x.filter, s)
		}
	}
	return nil
}

// set07 is T7-SET-07: Set Metadata writes back the Metadata of a Data
// DataSet of the PublicationList; without one, a Set of an unknown tag
// stimulates the Status event.
func (r *Runner) set07(ctx context.Context) error {
	const t = "T7-SET-07"
	var x *exchange
	n := 0
	for _, e := range r.pubEntries() {
		if e.resource != "Data" || e.mode == "OFF_0" || e.filter == "" || n == maxFilters {
			continue
		}
		n++
		g, err := r.getMeta(ctx, t, e.source, e.filter, e.writer)
		if err != nil {
			return err
		}
		if err := r.wait(ctx, []*exchange{g}, nil); err != nil {
			return err
		}
		answers, _, _ := r.snapshot(g)
		for _, a := range answers {
			if a.topic == g.answerTopic && a.msgType == "ua-metadata" && len(a.msgs) == 1 && a.msgs[0].payload != nil {
				x, err = r.setMeta(ctx, t, e.source, e.filter, e.writer, a.msgs[0].payload)
				if err != nil {
					return err
				}
				break
			}
		}
		if x != nil {
			break
		}
	}
	if x == nil {
		var err error
		tag := "D7Unknown-" + random()
		x, err = r.setMeta(ctx, t, r.app, tag, 10, unknownMeta(tag))
		if err != nil {
			return err
		}
		r.skip(t, "REQ-RES-META-007", "no Data DataSet of the PublicationList answers Get Metadata")
	}
	if err := r.settle(ctx, x); err != nil {
		return err
	}
	if s := r.judge(t, x); s.got && !s.good() {
		r.skip(t, "REQ-RES-META-007", "Set Metadata/%s/%s is answered with %s", x.source, x.filter, s)
	}
	return nil
}

// otherMode is a Mode different from the entry's (Event entries know only
// OFF_0 and FILTER_4, REQ-RES-PUBLIST-012).
func otherMode(e pubEntry) string {
	switch {
	case e.resource == "Event" && e.mode == "OFF_0":
		return "FILTER_4"
	case e.resource == "Event":
		return "OFF_0"
	case e.mode != "ON_REQUEST_1":
		return "ON_REQUEST_1"
	}
	return "ON_REQUEST_APPLICATION_SOURCE_FILTER_15"
}

// setEntry sends a Set PublicationList of the entry with another Mode.
func (r *Runner) setEntry(ctx context.Context, t string, e pubEntry, mode string) (*exchange, error) {
	p := payloadOf(e.d)
	p["Mode"] = mode
	x, err := r.write(ctx, t, "Set", "PublicationList", e.d.name, e.d.group, with(e.d, p))
	if err != nil {
		return nil, err
	}
	return x, r.settle(ctx, x)
}

// restoreEntry writes the entry back after an accepted change.
func (r *Runner) restoreEntry(ctx context.Context, t string, e pubEntry) error {
	x, err := r.write(ctx, t, "Set", "PublicationList", e.d.name, e.d.group, msg(e.d))
	if err != nil {
		return err
	}
	return r.settle(ctx, x)
}

// set08 is T7-SET-08: Set PublicationList writes back the configurable
// entries, is rejected for a Mode change that Config NONE_0 does not
// allow, and hides a Data DataSet whose Mode is set to OFF_0.
func (r *Runner) set08(ctx context.Context) error {
	const t = "T7-SET-08"
	es := r.pubEntries()
	if len(es) == 0 {
		for _, q := range []string{"REQ-RES-PUBLIST-007", "REQ-RES-PUBLIST-016", "REQ-RES-PUBLIST-024", "REQ-RES-PUBLIST-025"} {
			r.skip(t, q, "%s", r.absent("PublicationList", "PublicationList entry"))
		}
		return nil
	}
	var back []pubEntry
	for _, e := range es {
		if e.config != "NONE_0" {
			back = append(back, e)
		}
	}
	if len(back) == 0 {
		back = es[:1]
	}
	var xs []*exchange
	for _, e := range back {
		x, err := r.write(ctx, t, "Set", "PublicationList", e.d.name, e.d.group, msg(e.d))
		if err != nil {
			return err
		}
		xs = append(xs, x)
	}
	if err := r.settle(ctx, xs...); err != nil {
		return err
	}
	for _, x := range xs {
		r.judge(t, x)
	}
	var fixed *pubEntry
	for i, e := range es {
		if e.config == "NONE_0" && (fixed == nil || fixed.resource == "MAM" || fixed.resource == "Health") {
			fixed = &es[i]
		}
	}
	if fixed == nil {
		r.skip(t, "REQ-RES-PUBLIST-016", "every PublicationList entry has a Config other than NONE_0")
	} else {
		mode := otherMode(*fixed)
		x, err := r.setEntry(ctx, t, *fixed, mode)
		if err != nil {
			return err
		}
		s := r.judge(t, x)
		switch {
		case s.bad():
			r.checkedf(t, "REQ-RES-PUBLIST-016", x, "the Mode change of the NONE_0 entry %s is rejected with %s", fixed.d.group, s)
		case s.got:
			r.violation(t, "REQ-RES-PUBLIST-016", x, "the entry %s has Config NONE_0, but the change of its Mode to %s (MessageId %s) is answered with %s", fixed.d.group, mode, x.id, s)
		}
		if !s.bad() {
			if err := r.restoreEntry(ctx, t, *fixed); err != nil {
				return err
			}
		}
	}
	var data *pubEntry
	for i, e := range es {
		if e.resource == "Data" && e.mode != "OFF_0" && e.filter != "" && (e.config == "MODE_1" || e.config == "MODE_AND_INTERVAL_3") {
			data = &es[i]
			break
		}
	}
	if data == nil {
		r.skip(t, "REQ-RES-PUBLIST-007", "no Data entry of the PublicationList whose Mode can be set")
		return nil
	}
	x, err := r.setEntry(ctx, t, *data, "OFF_0")
	if err != nil {
		return err
	}
	s := r.judge(t, x)
	if !s.good() {
		r.skip(t, "REQ-RES-PUBLIST-007", "the Mode OFF_0 of the entry %s is answered with %s", data.d.group, s)
	} else {
		g, err := r.get(ctx, t, r.st, "Data", data.source, data.filter, nil)
		if err != nil {
			return err
		}
		if err := r.wait(ctx, []*exchange{g}, nil); err != nil {
			return err
		}
		answers, _, _ := r.snapshot(g)
		seen := false
		for _, a := range answers {
			for _, d := range a.content() {
				if d.name == data.source && d.group == data.filter {
					seen = true
				}
			}
		}
		if seen {
			r.violation(t, "REQ-RES-PUBLIST-007", g, "the entry %s has the Mode OFF_0, but Get Data/%s/%s (MessageId %s) returns the DataSet", data.d.group, data.source, data.filter, g.id)
		} else {
			r.checkedf(t, "REQ-RES-PUBLIST-007", g, "with Mode OFF_0, Get Data/%s/%s does not return the DataSet", data.source, data.filter)
		}
	}
	if !s.bad() {
		return r.restoreEntry(ctx, t, *data)
	}
	return nil
}

// set09 is T7-SET-09: Set and Del ReferenceDesignation: a write-back, a
// Del and the restoring Set; without an object, a Del stimulates the
// Status event.
func (r *Runner) set09(ctx context.Context) error {
	const t = "T7-SET-09"
	ds := r.before.dsms["ReferenceDesignation"]
	if !r.before.answered["ReferenceDesignation"] {
		for _, q := range []string{"REQ-RES-REFDES-009", "REQ-RES-REFDES-010"} {
			r.skip(t, q, "the CUT does not answer Get ReferenceDesignation: D7 could not restore a deleted object")
		}
		return nil
	}
	if len(ds) == 0 {
		x, err := r.write(ctx, t, "Del", "ReferenceDesignation", r.app, "")
		if err != nil {
			return err
		}
		if err := r.settle(ctx, x); err != nil {
			return err
		}
		r.judge(t, x)
		r.skip(t, "REQ-RES-REFDES-010", "the CUT has no ReferenceDesignation object")
		return nil
	}
	d := ds[0]
	for _, method := range []string{"Set", "Del", "Set"} {
		var x *exchange
		var err error
		if method == "Set" {
			x, err = r.write(ctx, t, "Set", "ReferenceDesignation", d.name, "", msg(d))
		} else {
			x, err = r.write(ctx, t, "Del", "ReferenceDesignation", d.name, "")
		}
		if err != nil {
			return err
		}
		if err := r.settle(ctx, x); err != nil {
			return err
		}
		s := r.judge(t, x)
		if method == "Del" && s.bad() {
			break
		}
		if method == "Set" && !s.good() {
			// The restoring Set would fail as well.
			r.skip(t, "REQ-RES-REFDES-010", "the write-back Set ReferenceDesignation of %s got %s: D7 does not delete an object it could not restore", d.name, s)
			break
		}
	}
	return nil
}

// restorable reports whether a write-back Set ReferenceDesignation of the
// source got Status Good: only then D7 can restore its object after a Del.
func (r *Runner) restorable(source string) bool {
	r.mu.Lock()
	xs := append([]*exchange(nil), r.exchanges...)
	r.mu.Unlock()
	for _, x := range xs {
		if x.method == "Set" && x.resource == "ReferenceDesignation" && x.source == source && r.statusOf(x).good() {
			return true
		}
	}
	return false
}

// ensureRestorable sends the write-back Set ReferenceDesignation of the
// object unless an earlier one got Status Good, and reports whether D7
// can restore the object after a Del.
func (r *Runner) ensureRestorable(ctx context.Context, test string, d dsm) (bool, error) {
	if r.restorable(d.name) {
		return true, nil
	}
	x, err := r.write(ctx, test, "Set", "ReferenceDesignation", d.name, "", msg(d))
	if err != nil {
		return false, err
	}
	if err := r.settle(ctx, x); err != nil {
		return false, err
	}
	return r.statusOf(x).good(), nil
}

// set11 is T7-SET-11: Set SubscriptionList writes back the CONF_1
// entries, is rejected for a NONE_0 entry, and creates (optional, SF-111)
// and deletes an entry of D7.
func (r *Runner) set11(ctx context.Context) error {
	const t = "T7-SET-11"
	if !r.before.answered["SubscriptionList"] {
		for _, q := range []string{"REQ-RES-SUBLIST-004", "REQ-RES-SUBLIST-013", "REQ-RES-SUBLIST-014", "REQ-RES-SUBLIST-015"} {
			r.skip(t, q, "the CUT does not answer Get SubscriptionList")
		}
		return nil
	}
	var xs []*exchange
	var fixed *dsm
	for i, d := range r.before.dsms["SubscriptionList"] {
		switch subConfig(d) {
		case "CONF_1":
			x, err := r.write(ctx, t, "Set", "SubscriptionList", d.name, d.group, msg(d))
			if err != nil {
				return err
			}
			xs = append(xs, x)
		case "NONE_0":
			if fixed == nil {
				fixed = &r.before.dsms["SubscriptionList"][i]
			}
		}
	}
	if err := r.settle(ctx, xs...); err != nil {
		return err
	}
	for _, x := range xs {
		r.judge(t, x)
	}
	if fixed != nil {
		p := payloadOf(*fixed)
		iv, _ := p["Interval"].(float64)
		p["Interval"] = iv + 1000
		x, err := r.write(ctx, t, "Set", "SubscriptionList", fixed.name, fixed.group, with(*fixed, p))
		if err != nil {
			return err
		}
		if err := r.settle(ctx, x); err != nil {
			return err
		}
		s := r.judge(t, x)
		switch {
		case s.bad():
			r.checkedf(t, "REQ-RES-SUBLIST-004", x, "the Interval change of the NONE_0 entry %s is rejected with %s", fixed.group, s)
		case s.got:
			r.violation(t, "REQ-RES-SUBLIST-004", x, "the entry %s has Config NONE_0, but the change of its Interval (MessageId %s) is answered with %s", fixed.group, x.id, s)
		}
		if !s.bad() {
			y, err := r.write(ctx, t, "Set", "SubscriptionList", fixed.name, fixed.group, msg(*fixed))
			if err != nil {
				return err
			}
			if err := r.settle(ctx, y); err != nil {
				return err
			}
		}
	}
	filter := "Data/d7-" + random()
	e := r.subEntry(filter, 1)
	x, err := r.write(ctx, t, "Set", "SubscriptionList", r.app, filter, e)
	if err != nil {
		return err
	}
	if err := r.settle(ctx, x); err != nil {
		return err
	}
	s := r.judge(t, x)
	switch {
	case s.bad():
		r.checkedf(t, "REQ-RES-SUBLIST-004", x, "the creation of an entry is rejected with %s (SF-111)", s)
		r.skip(t, "REQ-RES-SUBLIST-015", "the CUT does not create SubscriptionList entries (SF-111)")
		return nil
	case !s.good():
		return r.remove(ctx, t, filter, e)
	}
	for _, a := range r.published(x) {
		for _, d := range a.content() {
			if p, _ := d.payload.(map[string]any); p["Config"] == "CONF_1" {
				r.checkedf(t, "REQ-RES-SUBLIST-004", x, "the created entry is published with Config CONF_1")
			} else {
				r.violation(t, "REQ-RES-SUBLIST-004", x, "the created entry is published with Config %v instead of CONF_1", p["Config"])
			}
		}
	}
	d, err := r.write(ctx, t, "Del", "SubscriptionList", r.app, filter, e)
	if err != nil {
		return err
	}
	if err := r.quiet(ctx, d); err != nil {
		return err
	}
	if s := r.judge(t, d); s.good() {
		if ps := r.published(d); len(ps) > 0 {
			r.violation(t, "REQ-RES-SUBLIST-015", d, "no entry of %s remains after the Del (MessageId %s), but %d publication(s) follow on %s", filter, d.id, len(ps), d.answerTopic)
		} else {
			r.checkedf(t, "REQ-RES-SUBLIST-015", d, "no entry of %s remains after the Del, nothing is published", filter)
		}
	}
	return nil
}

// set12 is T7-SET-12: a Syslog level of the PublicationList can be
// switched (REQ-TOP-027); the Mode is set back afterwards.
func (r *Runner) set12(ctx context.Context) error {
	const t = "T7-SET-12"
	var syslog []pubEntry
	for _, e := range r.pubEntries() {
		if e.resource == "Event" && strings.HasPrefix(e.filter, "Syslog/") {
			syslog = append(syslog, e)
		}
	}
	if len(syslog) == 0 {
		r.skip(t, "REQ-TOP-027", "%s", r.absent("PublicationList", "Syslog entry in its PublicationList"))
		return nil
	}
	var e *pubEntry
	for i := range syslog {
		if syslog[i].config == "MODE_1" || syslog[i].config == "MODE_AND_INTERVAL_3" {
			e = &syslog[i]
			break
		}
	}
	if e == nil {
		r.violation(t, "REQ-TOP-027", nil, "every Syslog entry of the PublicationList (%d) has a Config without Mode: the log level is not configurable", len(syslog))
		return nil
	}
	x, err := r.setEntry(ctx, t, *e, otherMode(*e))
	if err != nil {
		return err
	}
	s := r.statusOf(x)
	if s.good() {
		r.checkedf(t, "REQ-TOP-027", x, "the Mode of the Syslog entry %s is switched to %s", e.d.group, otherMode(*e))
	} else {
		r.violation(t, "REQ-TOP-027", x, "the Syslog entry %s has Config %s, but the change of its Mode (MessageId %s) is answered with %s", e.d.group, e.config, x.id, s)
	}
	if !s.bad() {
		return r.restoreEntry(ctx, t, *e)
	}
	return nil
}

// setMatrix are the Set and Del an application supports at level 3, and
// those a represented entity supports (REQ-CMP-033).
var setMatrix = []struct {
	method, resource string
	asset            bool
}{
	{"Set", "Config", true}, {"Set", "Data", true}, {"Set", "Metadata", true}, {"Set", "PublicationList", false},
	{"Set", "SubscriptionList", false}, {"Set", "ReferenceDesignation", true}, {"Del", "SubscriptionList", false},
	{"Del", "ReferenceDesignation", true},
}

// reaction returns the first earlier exchange with the method, resource and
// Source that got a reaction (Status event or publication with its ReplyTo).
func (r *Runner) reaction(method, resource, source string) (*exchange, status) {
	r.mu.Lock()
	xs := append([]*exchange(nil), r.exchanges...)
	r.mu.Unlock()
	var first *exchange
	for _, x := range xs {
		if x.method != method || x.resource != resource || x.source != source {
			continue
		}
		s := r.statusOf(x)
		answers, _, _ := r.snapshot(x)
		if s.got || len(answers) > 0 {
			if s.code != badNotSupported {
				return x, s
			}
			first = x
		}
	}
	if first != nil {
		return first, r.statusOf(first)
	}
	return nil, status{}
}

// stimulus sends a harmless Set or Del for set03: an unknown object, or
// the write-back of an existing one.
func (r *Runner) stimulus(ctx context.Context, method, resource, source string) ([]*exchange, error) {
	const t = "T7-SET-03"
	tag := "D7Unknown-" + random()
	one := func(x *exchange, err error) ([]*exchange, error) {
		if err != nil {
			return nil, err
		}
		return []*exchange{x}, nil
	}
	switch method + " " + resource {
	case "Set Config":
		// A complete Config DataSet (REQ-RES-CONFIG-003) under an unknown
		// Filter.
		p := map[string]any{"d7": map[string]any{"d7Unknown": map[string]any{"Value": "D7"}}}
		return one(r.write(ctx, t, "Set", resource, source, tag, map[string]any{"DataSetWriterId": 10, "WriterGroupName": tag, "DataSetWriterName": source, "Payload": p}))
	case "Set Data":
		return one(r.write(ctx, t, "Set", resource, source, tag, map[string]any{"DataSetWriterId": 10, "WriterGroupName": tag, "DataSetWriterName": source, "Payload": map[string]any{"D7": 1}}))
	case "Set Metadata":
		return one(r.setMeta(ctx, t, source, tag, 10, unknownMeta(tag)))
	case "Set PublicationList":
		return one(r.write(ctx, t, "Set", resource, source, "Data/"+tag, map[string]any{"DataSetWriterId": 10, "WriterGroupName": "Data/" + tag, "DataSetWriterName": source,
			"Payload": map[string]any{"Resource": "Data", "Source": source, "Filter": tag, "DataSetWriterId": 10, "Mode": "OFF_0", "Config": "NONE_0"}}))
	case "Set SubscriptionList":
		return one(r.write(ctx, t, "Set", resource, source, "Data/"+tag))
	case "Del SubscriptionList":
		return one(r.write(ctx, t, "Del", resource, source, "Data/"+tag, r.subEntry("Data/"+tag, 1)))
	}
	if !r.before.answered["ReferenceDesignation"] {
		// D7 does not know the object: it could not restore it.
		return nil, nil
	}
	obj := r.object(source)
	if method == "Set" && obj != nil {
		return one(r.write(ctx, t, "Set", resource, source, "", msg(*obj)))
	}
	if method == "Del" && obj == nil {
		return one(r.write(ctx, t, "Del", resource, source, ""))
	}
	if method == "Del" {
		ok, err := r.ensureRestorable(ctx, t, *obj)
		if err != nil || !ok {
			return nil, err
		}
	}
	// Set without object: create and delete; Del with object: delete and
	// restore.
	var first *exchange
	var err error
	if method == "Set" {
		var id int64 = 10
		for _, e := range r.pubEntries() {
			if e.resource == resource && e.source == source {
				id = e.writer
			}
		}
		first, err = r.write(ctx, t, "Set", resource, source, "", map[string]any{"DataSetWriterId": id, "WriterGroupName": "", "DataSetWriterName": source,
			"Payload": map[string]any{"Function": map[string]any{"Value": "=D7", "Local": "=D7"}}})
	} else {
		first, err = r.write(ctx, t, "Del", resource, source, "")
	}
	if err != nil {
		return nil, err
	}
	if err := r.settle(ctx, first); err != nil {
		return nil, err
	}
	var undo *exchange
	if s := r.statusOf(first); !s.bad() {
		if method == "Set" {
			undo, err = r.write(ctx, t, "Del", resource, source, "")
		} else {
			undo, err = r.write(ctx, t, "Set", resource, source, "", msg(*obj))
		}
		if err != nil {
			return nil, err
		}
		return []*exchange{first, undo}, nil
	}
	return []*exchange{first}, nil
}

// object returns the ReferenceDesignation object of the source before the
// write tests, or nil.
func (r *Runner) object(source string) *dsm {
	for i, d := range r.before.dsms["ReferenceDesignation"] {
		if d.name == source {
			return &r.before.dsms["ReferenceDesignation"][i]
		}
	}
	return nil
}

// unknownMeta is the MetaData of a DataSet the CUT does not have
// (REQ-MSG-035).
func unknownMeta(name string) map[string]any {
	return map[string]any{"Name": name, "Namespaces": []any{}, "StructureDataTypes": []any{}, "EnumDataTypes": []any{}, "SimpleDataTypes": []any{}}
}

// set03 is T7-SET-03 (level 3): the application supports Set and Del for
// every resource MG 9.1 defines them for, a represented asset Set for
// Config, Data, Metadata and ReferenceDesignation and Del for
// ReferenceDesignation (REQ-CMP-033). It judges the reactions of the
// earlier write tests and stimulates the missing combinations; supported
// means any reaction with the ReplyTo except BadNotSupported. It runs
// after the other write tests.
func (r *Runner) set03(ctx context.Context) error {
	const t = "T7-SET-03"
	if !r.applies("REQ-CMP-033") {
		r.skip(t, "REQ-CMP-033", "REQ-CMP-033 does not apply at level %d", r.opt.Level)
		return nil
	}
	targets := []string{r.app}
	for _, s := range r.mamAll {
		if s != r.app {
			targets = append(targets, s)
			break
		}
	}
	type combo struct{ method, resource, source string }
	var missing []combo
	for _, src := range targets {
		for _, m := range setMatrix {
			if src != r.app && !m.asset {
				continue
			}
			if x, _ := r.reaction(m.method, m.resource, src); x == nil {
				missing = append(missing, combo{m.method, m.resource, src})
			}
		}
	}
	var sent []*exchange
	for _, c := range missing {
		xs, err := r.stimulus(ctx, c.method, c.resource, c.source)
		if err != nil {
			return err
		}
		sent = append(sent, xs...)
	}
	if err := r.settle(ctx, sent...); err != nil {
		return err
	}
	for _, src := range targets {
		who := "the application"
		if src != r.app {
			who = "the represented asset " + src
		}
		for _, m := range setMatrix {
			if src != r.app && !m.asset {
				continue
			}
			x, s := r.reaction(m.method, m.resource, src)
			switch {
			case x == nil && m.resource == "ReferenceDesignation" && !r.before.answered[m.resource]:
				r.skip(t, "REQ-CMP-033", "the CUT does not answer Get ReferenceDesignation: D7 sends no %s ReferenceDesignation it could not undo", m.method)
			case x == nil && m.method == "Del" && m.resource == "ReferenceDesignation" && r.object(src) != nil && !r.restorable(src):
				r.skip(t, "REQ-CMP-033", "%s rejects the write-back Set ReferenceDesignation: D7 does not delete an object it could not restore", who)
			case x == nil:
				r.violation(t, "REQ-CMP-033", nil, "%s does not support %s %s: no reaction with the ReplyTo within %s", who, m.method, m.resource, r.opt.Timeout)
			case s.got && s.code == badNotSupported:
				r.violation(t, "REQ-CMP-033", x, "%s does not support %s %s: %s", who, m.method, m.resource, s)
			default:
				r.checkedf(t, "REQ-CMP-033", x, "%s supports %s %s (%s)", who, m.method, m.resource, s)
			}
		}
	}
	return nil
}
