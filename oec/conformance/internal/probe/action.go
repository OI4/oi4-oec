package probe

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/OI4/oec/conformance/internal/verdict"
)

// The Action tests (P5 increment 4, C-065, DESIGN.md §3.3): T7-ACT-01 … 04
// call the common service NewDataSetWriterId (MG 8.4.1, MG 9.2.1) of the
// CUT. They run inside the write window: a provided DataSetWriterId is only
// reserved by the CUT, D7 never uses it, so the restore check compares
// the same state.

// Action addresses of NewDataSetWriterId (REQ-ACT-019).
const (
	newWriterService = "NewDataSetWriterId"
	newWriterID      = 8
	newWriterTarget  = 1
)

// actResp is one ActionResponse (MG 8.2.6).
type actResp struct {
	writer, target, requestID, state int64
	status                           map[string]any // nil if absent or no object
	payload                          map[string]any
	payloadSet                       bool
}

// actMsg is one message received on a Reply topic of the CUT.
type actMsg struct {
	topic, msgType, requestor string
	correlation               any
	responses                 []actResp
	received                  time.Time
}

// call is one ua-action-request of D7.
type call struct {
	test        string
	id          string
	topic       string
	reply       string // the ResponseAddress
	requestID   int64
	correlation string // Base64; "" if the request carries none
	sent        time.Time
}

// provided is a DataSetWriterId the CUT provided.
type provided struct {
	id, ttl int64
	at      time.Time
	test    string
}

func intOf(v any) (int64, bool) {
	f, ok := v.(float64)
	if !ok || f != float64(int64(f)) {
		return 0, false
	}
	return int64(f), true
}

// parseAction decodes a message on a Reply topic; nil if it is no JSON
// object.
func parseAction(m Message) *actMsg {
	var doc map[string]any
	if json.Unmarshal(m.Payload, &doc) != nil {
		return nil
	}
	a := &actMsg{topic: m.Topic, received: m.Received, correlation: doc["CorrelationData"]}
	a.msgType, _ = doc["MessageType"].(string)
	a.requestor, _ = doc["RequestorId"].(string)
	list, _ := doc["Messages"].([]any)
	for _, x := range list {
		o, _ := x.(map[string]any)
		if o == nil {
			continue
		}
		var rs actResp
		rs.writer, _ = intOf(o["DataSetWriterId"])
		rs.target, _ = intOf(o["ActionTargetId"])
		rs.state, _ = intOf(o["ActionState"])
		var ok bool
		if rs.requestID, ok = intOf(o["RequestId"]); !ok {
			rs.requestID = -1
		}
		rs.status, _ = o["Status"].(map[string]any)
		rs.payload, rs.payloadSet = o["Payload"].(map[string]any)
		if _, there := o["Payload"]; !there {
			rs.payloadSet = false
		}
		a.responses = append(a.responses, rs)
	}
	return a
}

// noteUsed records the DataSetWriterIds a publication of the CUT uses:
// those of its DataSetMessages and those its PublicationList names.
// Caller holds r.mu.
func (r *Runner) noteUsed(n *netMsg) {
	for _, d := range n.msgs {
		if d.id > 9 {
			r.used[d.id] = true
		}
		if n.resource == "PublicationList" && !d.helper() {
			if p, _ := d.payload.(map[string]any); p != nil {
				if id, ok := intOf(p["DataSetWriterId"]); ok {
					r.used[id] = true
				}
			}
		}
	}
}

// actionTopics returns the Call topic and the corresponding Reply topic
// (REQ-TOP-016).
func (r *Runner) actionTopics(source string) (string, string) {
	return Topic(r.st, r.app, "Call", newWriterService, source, ""), Topic(r.st, r.app, "Reply", newWriterService, source, "")
}

// callNew sends a NewDataSetWriterId request for the Source ("" for the
// application without Source element); edit may change the
// ActionRequest.
func (r *Runner) callNew(ctx context.Context, test, source string, correlation bool, edit func(ar map[string]any)) (*call, error) {
	callTopic, reply := r.actionTopics(source)
	r.mu.Lock()
	r.rid++
	rid := r.rid
	r.mu.Unlock()
	c := &call{test: test, id: r.nextID(), topic: callTopic, reply: reply, requestID: rid}
	ar := map[string]any{"DataSetWriterId": newWriterID, "ActionTargetId": newWriterTarget, "RequestId": rid, "ActionState": 1,
		"Payload": map[string]any{"Resource": "Data"}}
	if edit != nil {
		edit(ar)
	}
	doc := map[string]any{"MessageId": c.id, "MessageType": "ua-action-request", "PublisherId": r.st + "/" + r.app,
		"Timestamp": time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), "ResponseAddress": reply, "RequestorId": r.tester,
		"TimeoutHint": r.opt.Timeout.Milliseconds(), "Messages": []any{ar}}
	if correlation {
		c.correlation = base64.StdEncoding.EncodeToString([]byte(c.id))
		doc["CorrelationData"] = c.correlation
	}
	payload, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return c, r.send(ctx, c, payload)
}

func (r *Runner) send(ctx context.Context, c *call, payload []byte) error {
	c.sent = time.Now()
	r.mu.Lock()
	r.calls = append(r.calls, c)
	r.mu.Unlock()
	if err := r.pub(ctx, c.topic, payload); err != nil {
		return fmt.Errorf("publish %s: %w", c.topic, err)
	}
	return nil
}

// hit is an ActionResponse to a call and the message that carried it.
type hit struct {
	msg  *actMsg
	resp actResp
}

// responses returns the ActionResponses with the RequestId of the call,
// addressed to D7 and received after the request, on any Reply topic of
// the CUT.
func (r *Runner) responses(c *call) []hit {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []hit
	for _, a := range r.actions {
		if a.requestor != r.tester || a.received.Before(c.sent) {
			continue
		}
		for _, rs := range a.responses {
			if rs.requestID == c.requestID {
				out = append(out, hit{a, rs})
			}
		}
	}
	return out
}

// final returns the final ActionResponses (ActionState 2) of a call.
func final(hs []hit) []hit {
	var out []hit
	for _, h := range hs {
		if h.resp.state == 2 {
			out = append(out, h)
		}
	}
	return out
}

// awaitCalls waits until every call has a final response (plus the grace
// time) or the timeout ended.
func (r *Runner) awaitCalls(ctx context.Context, cs ...*call) error {
	return r.until(ctx, func() bool {
		for _, c := range cs {
			if len(final(r.responses(c))) == 0 {
				return false
			}
		}
		return true
	})
}

// offered reports whether the CUT lists NewDataSetWriterId in the Profile
// of the application (MG 8.3.7).
func (r *Runner) offered() bool { return r.hasProfile && contains(r.profile, newWriterService) }

// expected reports whether the CUT has to answer NewDataSetWriterId: it
// offers the service, or the service is mandatory at the declared level
// (REQ-CMP-040).
func (r *Runner) expected() bool { return r.offered() || r.mandatory("REQ-CMP-040") }

func (r *Runner) actEv(c *call, format string, a ...any) string {
	return fmt.Sprintf("%s (request MessageId %s, RequestId %d): ", c.topic, c.id, c.requestID) + fmt.Sprintf(format, a...)
}

// unanswered judges a valid call without final response: a violation of
// the CMP requirements where they apply, of the ACT requirements if the
// CUT has to answer, else a skip.
func (r *Runner) unanswered(test string, c *call, reqs ...string) {
	for _, req := range []string{"REQ-CMP-034", "REQ-CMP-040"} {
		if r.applies(req) {
			r.violation(test, req, nil, "%s", r.actEv(c, "no final response within %s", r.opt.Timeout))
		}
	}
	for _, req := range reqs {
		if r.expected() {
			r.violation(test, req, nil, "%s", r.actEv(c, "no final response within %s", r.opt.Timeout))
		} else {
			r.skip(test, req, "the CUT does not offer %s (not in its Profile, REQ-CMP-040 is %s at level %d) and did not answer",
				newWriterService, r.reqs.Obligation("REQ-CMP-040", r.opt.Level), r.opt.Level)
		}
	}
}

// actJudge collects the violations of one test and records every other
// requirement as checked.
type actJudge struct {
	r    *Runner
	test string
	bad  map[string]bool
}

func (j *actJudge) fail(req string, c *call, format string, a ...any) {
	j.bad[req] = true
	j.r.violation(j.test, req, nil, "%s", j.r.actEv(c, format, a...))
}

func (j *actJudge) done(detail string, reqs ...string) {
	for _, req := range reqs {
		if !j.bad[req] {
			j.r.add(verdict.Observation{Test: j.test, Req: req, Kind: verdict.Checked, Evidence: j.r.ev(nil, "%s", detail)})
		}
	}
}

// statusCode returns the Code of a CompactEncoding Status ({} is Good).
func statusCode(s map[string]any) (int64, bool) {
	if s == nil {
		return 0, false
	}
	v, there := s["Code"]
	if !there {
		return 0, true
	}
	c, ok := intOf(v)
	return c, ok
}

// act01Reqs are the requirements T7-ACT-01 judges on every answered call.
var act01Reqs = []string{"REQ-ACT-001", "REQ-ACT-006", "REQ-ACT-007", "REQ-ACT-012", "REQ-ACT-014", "REQ-ACT-015", "REQ-ACT-016",
	"REQ-ACT-020", "REQ-ACT-022", "REQ-TOP-016"}

// act01 is T7-ACT-01: NewDataSetWriterId for the application, without
// Source and with CorrelationData, and with the AppId as Source and
// without CorrelationData.
func (r *Runner) act01(ctx context.Context) error {
	const test = "T7-ACT-01"
	a, err := r.callNew(ctx, test, "", true, nil)
	if err != nil {
		return err
	}
	b, err := r.callNew(ctx, test, r.app, false, nil)
	if err != nil {
		return err
	}
	if err := r.awaitCalls(ctx, a, b); err != nil {
		return err
	}
	j := &actJudge{r: r, test: test, bad: map[string]bool{}}
	answered := 0
	for _, c := range []*call{a, b} {
		hs := r.responses(c)
		if len(final(hs)) == 0 && len(hs) == 0 {
			r.unanswered(test, c, "REQ-ACT-001", "REQ-ACT-015")
			continue
		}
		answered++
		r.judgeResponses(j, c, hs)
	}
	if answered == 0 {
		return nil
	}
	for _, req := range []string{"REQ-CMP-034", "REQ-CMP-040"} {
		if r.applies(req) {
			j.done("NewDataSetWriterId answered", req)
		}
	}
	j.done(fmt.Sprintf("%d of 2 NewDataSetWriterId request(s) answered", answered), act01Reqs...)
	return nil
}

// judgeResponses judges the responses of one valid call for the
// application (T7-ACT-01, T7-ACT-02) and records a provided id.
func (r *Runner) judgeResponses(j *actJudge, c *call, hs []hit) {
	finals := 0
	for _, h := range hs {
		m, rs := h.msg, h.resp
		if m.msgType != "ua-action-response" {
			j.fail("REQ-ACT-001", c, "the response has the MessageType %q instead of ua-action-response", m.msgType)
		}
		if m.topic != c.reply {
			for _, req := range []string{"REQ-ACT-006", "REQ-ACT-020", "REQ-TOP-016"} {
				j.fail(req, c, "the response is published on %s instead of the ResponseAddress %s", m.topic, c.reply)
			}
		}
		got, _ := m.correlation.(string)
		switch {
		case c.correlation != "" && got != c.correlation:
			j.fail("REQ-ACT-007", c, "the response carries the CorrelationData %v instead of %q", m.correlation, c.correlation)
		case c.correlation == "" && m.correlation != nil:
			j.fail("REQ-ACT-007", c, "the request carries no CorrelationData, the response %v", m.correlation)
		}
		if rs.writer != newWriterID || rs.target != newWriterTarget {
			j.fail("REQ-ACT-015", c, "the response has DataSetWriterId %d and ActionTargetId %d instead of %d and %d", rs.writer, rs.target, newWriterID, newWriterTarget)
		}
		code, ok := statusCode(rs.status)
		if !ok {
			j.fail("REQ-ACT-014", c, "the response has no CompactEncoding Status ({} or {\"Code\": <UInt32>})")
		} else if _, sym := rs.status["Symbol"]; sym {
			j.fail("REQ-ACT-014", c, "the Status carries a Symbol")
		}
		switch rs.state {
		case 1:
			continue
		case 2:
			finals++
		default:
			j.fail("REQ-ACT-012", c, "the response has the ActionState %d (1 Executing or 2 Done)", rs.state)
			continue
		}
		if !rs.payloadSet {
			j.fail("REQ-ACT-012", c, "the final response has no Payload object")
		}
		if ok && code != 0 {
			j.fail("REQ-ACT-022", c, "the request for the application was rejected with the Status 0x%08X: no DataSetWriterId provided", code)
			continue
		}
		r.judgeProvided(j, c, rs.payload)
	}
	if finals != 1 {
		j.fail("REQ-ACT-015", c, "%d final response(s) (ActionState 2) instead of exactly one", finals)
	}
}

// judgeProvided checks the Payload of a successful NewDataSetWriterId
// response (REQ-ACT-022) and records the id for T7-ACT-02.
func (r *Runner) judgeProvided(j *actJudge, c *call, p map[string]any) {
	id, okID := intOf(p["DataSetWriterId"])
	ttl, okTTL := intOf(p["Ttl"])
	switch {
	case !okID || id < 10 || id > 65535:
		j.fail("REQ-ACT-022", c, "the Payload has the DataSetWriterId %v instead of a UInt16 >= 10", p["DataSetWriterId"])
		return
	case !okTTL || ttl < 0 || ttl > 4294967295:
		j.fail("REQ-ACT-022", c, "the Payload has the Ttl %v instead of a UInt32", p["Ttl"])
		return
	}
	r.mu.Lock()
	used := r.used[id]
	var again *provided
	for i := range r.provided {
		if r.provided[i].id == id {
			again = &r.provided[i]
		}
	}
	r.provided = append(r.provided, provided{id: id, ttl: ttl, at: time.Now(), test: j.test})
	r.mu.Unlock()
	switch {
	case used:
		j.fail("REQ-ACT-022", c, "the provided DataSetWriterId %d is already used by the CUT", id)
	case again != nil && j.test == "T7-ACT-01":
		j.fail("REQ-ACT-022", c, "the DataSetWriterId %d was already provided (%s): it is not new", id, again.test)
	}
}

// act02 is T7-ACT-02: two further requests while the ids provided before
// are reserved get new ids. The acceptance of the initial use (e.g. in a
// Set) within Ttl is not tested: D7 does not create objects with them.
func (r *Runner) act02(ctx context.Context) error {
	const test, req = "T7-ACT-02", "REQ-ACT-023"
	var cs []*call
	for range 2 {
		c, err := r.callNew(ctx, test, "", false, nil)
		if err != nil {
			return err
		}
		if err := r.awaitCalls(ctx, c); err != nil {
			return err
		}
		cs = append(cs, c)
	}
	j := &actJudge{r: r, test: test, bad: map[string]bool{}}
	n := 0
	for _, c := range cs {
		r.mu.Lock()
		before := append([]provided(nil), r.provided...)
		r.mu.Unlock()
		for _, h := range final(r.responses(c)) {
			code, ok := statusCode(h.resp.status)
			if !ok || code != 0 {
				continue
			}
			id, okID := intOf(h.resp.payload["DataSetWriterId"])
			if !okID {
				continue
			}
			n++
			for _, p := range before {
				if p.id == id && time.Since(p.at) < time.Duration(p.ttl)*time.Second {
					j.fail(req, c, "the DataSetWriterId %d was provided again although it is reserved for %d s since %s (%s)",
						id, p.ttl, p.at.UTC().Format(time.RFC3339Nano), p.test)
				}
			}
			r.judgeProvided(j, c, h.resp.payload)
		}
	}
	if n == 0 {
		if r.expected() {
			r.violation(test, req, nil, "no DataSetWriterId provided by %d request(s)", len(cs))
		} else {
			r.skip(test, req, "the CUT provided no DataSetWriterId")
		}
		return nil
	}
	j.done(fmt.Sprintf("%d DataSetWriterId(s) provided while the earlier ones were reserved; their initial use is not tested", n), req)
	return nil
}

// act03 is T7-ACT-03: requests the CUT cannot address or decode get no
// response: an unknown DataSetWriterId (4, reserved for future common
// services), an unknown ActionTargetId and an undecodable message. A valid
// request beside them shows that the CUT answers at all.
func (r *Runner) act03(ctx context.Context) error {
	const test, req = "T7-ACT-03", "REQ-ACT-017"
	control, err := r.callNew(ctx, test, "", false, nil)
	if err != nil {
		return err
	}
	writer, err := r.callNew(ctx, test, "", false, func(ar map[string]any) { ar["DataSetWriterId"] = 4 })
	if err != nil {
		return err
	}
	target, err := r.callNew(ctx, test, "", false, func(ar map[string]any) { ar["ActionTargetId"] = 2 })
	if err != nil {
		return err
	}
	// The undecodable request goes to the Call topic with the AppId as
	// Source: any response on its Reply topic is a response to it.
	callTopic, reply := r.actionTopics(r.app)
	broken := &call{test: test, id: r.nextID(), topic: callTopic, reply: reply, requestID: -1}
	if err := r.send(ctx, broken, []byte(`{"MessageId": "`+broken.id+`", "MessageType": "ua-action-request", "Messages": [`)); err != nil {
		return err
	}
	if err := sleep(ctx, r.opt.Timeout); err != nil {
		return err
	}
	if len(final(r.responses(control))) == 0 {
		r.unanswered(test, control)
		r.skip(test, req, "the CUT did not answer the valid NewDataSetWriterId request: its silence to the others proves nothing")
		return nil
	}
	j := &actJudge{r: r, test: test, bad: map[string]bool{}}
	for _, x := range []struct {
		c    *call
		what string
	}{{writer, "the unknown DataSetWriterId 4"}, {target, "the unknown ActionTargetId 2"}} {
		if hs := r.responses(x.c); len(hs) > 0 {
			code, _ := statusCode(hs[0].resp.status)
			j.fail(req, x.c, "the request with %s was answered (%d response(s), Status 0x%08X)", x.what, len(hs), code)
		}
	}
	r.mu.Lock()
	var late []*actMsg
	for _, a := range r.actions {
		if a.topic == reply && !a.received.Before(broken.sent) {
			late = append(late, a)
		}
	}
	r.mu.Unlock()
	if len(late) > 0 {
		j.fail(req, broken, "the undecodable request was answered on %s (%d message(s))", reply, len(late))
	}
	j.done("no response to the unknown DataSetWriterId, the unknown ActionTargetId and the undecodable request", req)
	return nil
}

// act04 is T7-ACT-04: NewDataSetWriterId for a Source the CUT neither is
// nor represents is answered with BadNotFound and an empty Payload.
func (r *Runner) act04(ctx context.Context) error {
	const test, req = "T7-ACT-04", "REQ-ACT-024"
	unknown := "oi4.org/oec-check/Unknown/" + random()
	c, err := r.callNew(ctx, test, unknown, false, nil)
	if err != nil {
		return err
	}
	if err := r.awaitCalls(ctx, c); err != nil {
		return err
	}
	fs := final(r.responses(c))
	if len(fs) == 0 {
		if r.expected() {
			r.violation(test, req, nil, "%s", r.actEv(c, "the request for the unknown Source %s got no final response within %s", unknown, r.opt.Timeout))
		} else {
			r.skip(test, req, "the CUT does not offer %s (not in its Profile) and did not answer", newWriterService)
		}
		return nil
	}
	j := &actJudge{r: r, test: test, bad: map[string]bool{}}
	for _, h := range fs {
		code, ok := statusCode(h.resp.status)
		if !ok || code != BadNotFound {
			j.fail(req, c, "the request for the unknown Source %s was answered with the Status %v instead of {\"Code\": %d} (BadNotFound)", unknown, h.resp.status, BadNotFound)
		}
		if len(h.resp.payload) > 0 {
			j.fail(req, c, "the response for the unknown Source carries the Payload %v instead of an empty one", h.resp.payload)
		}
	}
	j.done("the request for the unknown Source "+unknown+" was answered with BadNotFound", req)
	return nil
}

// isActionTest reports a test of the Action group.
func isActionTest(id string) bool { return strings.HasPrefix(id, "T7-ACT-") }
