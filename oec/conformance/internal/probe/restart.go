package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/OI4/oec/conformance/internal/proxy"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// The restart tests (C-067, DESIGN.md §3.4) use the restart hook: a
// command that stops the CUT gracefully and starts it again. They run in
// the write window, after the other write tests:
//
//  1. T7-SET-10 sets a changed ReferenceDesignation;
//  2. the hook restarts the CUT; D7 waits for its new Birth (through the
//     proxy also for its first SUBACK) and its answer to Get Health;
//  3. T7-LIFE-04 judges the end of the old connection: the last Pub
//     Health of the CUT before its DISCONNECT (proxy) or before its new
//     Birth (bus) is the Close message;
//  4. T7-SET-10 reads the ReferenceDesignation again and restores it.
var restartTests = []string{"T7-LIFE-04", "T7-SET-10"}

func isRestartTest(id string) bool { return contains(restartTests, id) }

// NoRestart is the reason of the inconclusive skips without restart hook.
const NoRestart = "needs the restart hook (--restart-cmd, or --stop-cmd and --start-cmd): D7 cannot stop and start the CUT"

func (r *Runner) restartSelected() bool {
	for _, id := range restartTests {
		if r.Selected(id) {
			return true
		}
	}
	return false
}

// refdesChange is the changed ReferenceDesignation of T7-SET-10.
type refdesChange struct {
	orig dsm
	want map[string]any
}

// restart runs the restart phase.
func (r *Runner) restart(ctx context.Context) error {
	if r.opt.Restart == nil {
		for _, id := range restartTests {
			if r.Selected(id) {
				r.lifecycleSkipTest(id, "%s", NoRestart)
			}
		}
		return nil
	}
	var change *refdesChange
	if r.Selected("T7-SET-10") {
		var err error
		if change, err = r.set10Change(ctx); err != nil {
			return err
		}
	}
	old := -1
	if r.opt.Proxy != nil {
		for _, c := range r.opt.Proxy.Conns() {
			if c.Relayed() && c.Closed.IsZero() {
				old = c.ID
			}
		}
	}
	at := time.Now()
	r.mu.Lock()
	r.restartAt = at
	r.mu.Unlock()
	if err := r.opt.Restart(ctx); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		for _, id := range restartTests {
			if r.Selected(id) {
				r.lifecycleSkipTest(id, "the restart hook failed: %v", err)
			}
		}
		if change != nil {
			return r.set10Restore(ctx, change)
		}
		return nil
	}
	back, err := r.awaitRestart(ctx, old)
	if err != nil {
		return err
	}
	// The Get Health shows that the CUT answers again and gives the
	// DataSetWriterId of its Health; a CUT that subscribes after its Birth
	// gets a second chance.
	var own []int64
	for try := 0; try < 2 && len(own) == 0; try++ {
		x, err := r.get(ctx, "restart", r.st, "Health", r.app, "", nil)
		if err != nil {
			return err
		}
		if err := r.until(ctx, r.completeAll(x)); err != nil {
			return err
		}
		answers, _, _ := r.snapshot(x)
		for _, a := range answers {
			for _, d := range a.content() {
				if d.name == r.app {
					own = append(own, d.id)
				}
			}
		}
	}
	if r.Selected("T7-LIFE-04") {
		r.life04(at, old, back, own)
	}
	if change != nil {
		if err := r.set10Judge(ctx, change, back); err != nil {
			return err
		}
		return r.set10Restore(ctx, change)
	}
	return nil
}

// awaitRestart waits until the CUT is back: its new Birth on the bus or,
// through the proxy, a new relayed connection with its Birth and first
// SUBACK. It reports whether the CUT came back within RestartWait.
func (r *Runner) awaitRestart(ctx context.Context, old int) (bool, error) {
	deadline := time.Now().Add(r.opt.RestartWait)
	for time.Now().Before(deadline) {
		if r.opt.Proxy != nil {
			for _, c := range r.opt.Proxy.Conns() {
				if c.ID > old && c.Relayed() && r.birth(c) >= 0 && !c.Subscribed.IsZero() {
					return true, sleep(ctx, r.opt.Grace)
				}
			}
		} else {
			r.mu.Lock()
			ms := append([]*netMsg(nil), r.restarted...)
			r.mu.Unlock()
			if rebirth(ms, r.app) >= 0 {
				return true, sleep(ctx, r.opt.Grace)
			}
		}
		if err := sleep(ctx, 50*time.Millisecond); err != nil {
			return false, err
		}
	}
	return false, nil
}

// rebirth returns the index of the first Birth (Pub MAM of the CUT about
// itself, no answer) among the publications, or -1.
func rebirth(ms []*netMsg, app string) int {
	for i, n := range ms {
		if n.method == "Pub" && n.resource == "MAM" && n.replyTo == "" && strings.HasSuffix(n.topic, "/MAM/"+enc(app)) {
			return i
		}
	}
	return -1
}

// healthPub reports a Pub Health of the CUT that answers no request.
func (r *Runner) healthPub(n *netMsg) bool {
	return n != nil && n.method == "Pub" && n.resource == "Health" && n.app == r.app && n.replyTo == ""
}

// life04 is T7-LIFE-04: the Close message before a graceful disconnect
// (REQ-LIFE-005 … 007, graded by REQ-CMP-046).
func (r *Runner) life04(at time.Time, old int, back bool, own []int64) {
	const t = "T7-LIFE-04"
	var close *netMsg
	var where string
	if r.opt.Proxy != nil {
		var c *proxy.Conn
		cs := r.opt.Proxy.Conns()
		for i := range cs {
			if cs[i].ID == old {
				c = &cs[i]
			}
		}
		switch {
		case c == nil:
			r.lifecycleSkipTest(t, "the CUT was not connected through the proxy when the restart hook started")
			return
		case c.Closed.IsZero():
			r.lifecycleSkipTest(t, "connection %d of the CUT did not end after the restart hook", c.ID)
			return
		case !c.Disconnect:
			r.lifecycleSkipTest(t, "connection %d of the CUT ended without DISCONNECT: no graceful disconnect (the restart hook does not stop the CUT gracefully, or the CUT does not disconnect)", c.ID)
			return
		}
		after := 0
		for _, p := range c.Publishes {
			if p.Time.Before(at) {
				continue
			}
			n := parse(Message{Topic: p.Topic, Payload: p.Payload, Received: p.Time})
			if r.healthPub(n) {
				close, after = n, 0
			} else if close != nil {
				after++
			}
		}
		where = fmt.Sprintf("connection %d, DISCONNECT at %s", c.ID, c.Closed.UTC().Format(time.RFC3339Nano))
		if close != nil && after > 0 {
			r.add(verdict.Observation{Test: t, Req: "REQ-LIFE-005", Kind: verdict.Note,
				Evidence: r.ev(nil, "%d PUBLISH packet(s) follow the Close message before the DISCONNECT (%s)", after, where)})
		}
	} else {
		r.mu.Lock()
		ms := append([]*netMsg(nil), r.restarted...)
		r.mu.Unlock()
		end := rebirth(ms, r.app)
		if end < 0 {
			end = len(ms)
		}
		for _, n := range ms[:end] {
			if !r.healthPub(n) {
				continue
			}
			if state, _, _, ok := health(n); ok && state == "FAILURE_1" && strings.HasSuffix(n.topic, "/"+enc(r.app)) {
				r.lifecycleSkipTest(t, "the broker published the Will of the CUT (%s): no graceful disconnect", n.messageID)
				return
			}
			close = n
		}
		if !back {
			r.lifecycleSkipTest(t, "no new Birth of the CUT within %s after the restart hook: D7 cannot tell whether the CUT was stopped", r.opt.RestartWait)
			return
		}
		where = "before its new Birth on the bus"
	}
	if close == nil {
		for _, req := range []string{"REQ-LIFE-005", "REQ-CMP-046"} {
			r.add(verdict.Observation{Test: t, Req: req, Kind: verdict.Violation,
				Evidence: r.ev(nil, "no Close message (Pub Health of the CUT) after the start of the restart hook (%s)", where)})
		}
		for _, req := range []string{"REQ-LIFE-006", "REQ-LIFE-007"} {
			r.skip(t, req, "the CUT published no Close message")
		}
		return
	}
	ev := func(format string, a ...any) verdict.Evidence {
		e := r.ev(nil, format, a...)
		e.Topic = close.topic
		return e
	}
	obs := func(req string, problems []string, ok string) {
		if len(problems) == 0 {
			r.add(verdict.Observation{Test: t, Req: req, Kind: verdict.Checked, Evidence: ev("%s", ok)})
		}
		for _, p := range problems {
			r.add(verdict.Observation{Test: t, Req: req, Kind: verdict.Violation, Evidence: ev("Close message (MessageId %s, %s): %s", close.messageID, where, p)})
		}
	}
	bad := false
	obs("REQ-LIFE-005", nil, fmt.Sprintf("Close message (MessageId %s, %s)", close.messageID, where))
	var topic []string
	if want := Topic(r.st, r.app, "Pub", "Health", r.app, ""); close.topic != want {
		topic = append(topic, fmt.Sprintf("published on %s instead of %s", close.topic, want))
	}
	obs("REQ-LIFE-006", topic, "published on "+close.topic)
	var content []string
	state, score, id, ok := health(close)
	switch {
	case !ok:
		content = append(content, fmt.Sprintf("%d DataSetMessages; one Health DataSetMessage is required", len(close.msgs)))
	case state != "NORMAL_0" || score != 0:
		content = append(content, fmt.Sprintf("Health %s, HealthScore %v instead of NORMAL_0/0", state, score))
	}
	if ok && len(own) > 0 && !containsID(own, id) {
		var ids []string
		for _, x := range own {
			ids = append(ids, strconv.FormatInt(x, 10))
		}
		content = append(content, fmt.Sprintf("DataSetWriterId %d, the Health of the CUT about itself uses %s", id, strings.Join(ids, ", ")))
	}
	idNote := ""
	if len(own) == 0 {
		idNote = "; the DataSetWriterId is not compared: the CUT did not answer Get Health after the restart"
	}
	obs("REQ-LIFE-007", content, fmt.Sprintf("Health NORMAL_0/0 with DataSetWriterId %d%s", id, idNote))
	bad = len(topic) > 0 || len(content) > 0
	if bad {
		r.add(verdict.Observation{Test: t, Req: "REQ-CMP-046", Kind: verdict.Violation, Evidence: ev("the Close message is not as defined in MG 6.3.4 (see REQ-LIFE-006/007)")})
	} else {
		r.add(verdict.Observation{Test: t, Req: "REQ-CMP-046", Kind: verdict.Checked, Evidence: ev("Close message as defined in MG 6.3.4")})
	}
}

func containsID(ids []int64, id int64) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// set10Change sets a changed ReferenceDesignation before the restart
// (T7-SET-10): the object of the application (else the first one) with
// "D7" appended to Value and Local of its first aspect.
func (r *Runner) set10Change(ctx context.Context) (*refdesChange, error) {
	const t = "T7-SET-10"
	const req = "REQ-RES-REFDES-002"
	if !r.before.answered["ReferenceDesignation"] {
		r.skip(t, req, "the CUT does not answer Get ReferenceDesignation")
		return nil, nil
	}
	ds := r.before.dsms["ReferenceDesignation"]
	if len(ds) == 0 {
		r.skip(t, req, "the CUT has no ReferenceDesignation object")
		return nil, nil
	}
	d := ds[0]
	for _, x := range ds {
		if x.name == r.app {
			d = x
			break
		}
	}
	want := changedRefDes(payloadOf(d))
	x, err := r.write(ctx, t, "Set", "ReferenceDesignation", d.name, "", with(d, want))
	if err != nil {
		return nil, err
	}
	if err := r.settle(ctx, x); err != nil {
		return nil, err
	}
	s := r.judge(t, x)
	switch {
	case s.bad():
		r.skip(t, req, "the CUT rejects Set ReferenceDesignation of %s with %s: it does not accept the ReferenceDesignation from outside", d.name, s)
		return nil, nil
	case !s.good():
		r.lifecycleSkipTest(t, "Set ReferenceDesignation of %s got %s", d.name, s)
		return &refdesChange{orig: d}, nil
	}
	return &refdesChange{orig: d, want: want}, nil
}

// changedRefDes appends "D7" to Value and Local of the first aspect (a
// Location +D7 if there is none).
func changedRefDes(p map[string]any) map[string]any {
	for _, aspect := range []string{"Function", "Product", "Location"} {
		a, ok := p[aspect].(map[string]any)
		if !ok {
			continue
		}
		for _, k := range []string{"Value", "Local"} {
			if v, ok := a[k].(string); ok {
				a[k] = v + "D7"
			}
		}
		return p
	}
	p["Location"] = map[string]any{"Value": "+D7"}
	return p
}

// set10Judge reads the ReferenceDesignation after the restart: it is the
// changed one (REQ-RES-REFDES-002).
func (r *Runner) set10Judge(ctx context.Context, c *refdesChange, back bool) error {
	const t = "T7-SET-10"
	const req = "REQ-RES-REFDES-002"
	if c.want == nil {
		return nil
	}
	if !back {
		r.lifecycleSkipTest(t, "the CUT did not come back within %s after the restart hook", r.opt.RestartWait)
		return nil
	}
	x, err := r.get(ctx, t, r.st, "ReferenceDesignation", c.orig.name, "", nil)
	if err != nil {
		return err
	}
	if err := r.until(ctx, r.completeAll(x)); err != nil {
		return err
	}
	answers, _, _ := r.snapshot(x)
	var got []dsm
	for _, a := range answers {
		if a.topic == x.answerTopic {
			got = append(got, a.content()...)
		}
	}
	if len(answers) == 0 {
		r.lifecycleSkipTest(t, "Get ReferenceDesignation of %s (MessageId %s) is not answered after the restart", c.orig.name, x.id)
		return nil
	}
	want, _ := json.Marshal(c.want)
	orig, _ := json.Marshal(payloadOf(c.orig))
	switch {
	case len(got) != 1:
		r.violation(t, req, x, "after the restart Get ReferenceDesignation of %s returns %d objects instead of the one D7 set", c.orig.name, len(got))
	default:
		b, _ := json.Marshal(got[0].payload)
		switch string(b) {
		case string(want):
			r.checkedf(t, req, x, "the ReferenceDesignation D7 set for %s survived the restart", c.orig.name)
		case string(orig):
			r.violation(t, req, x, "after the restart the ReferenceDesignation of %s is the one before the Set: the Set (accepted with Status Good) was not persisted", c.orig.name)
		default:
			r.violation(t, req, x, "after the restart the ReferenceDesignation of %s is %s instead of %s", c.orig.name, b, want)
		}
	}
	return nil
}

// set10Restore writes the original ReferenceDesignation back.
func (r *Runner) set10Restore(ctx context.Context, c *refdesChange) error {
	x, err := r.write(ctx, "T7-SET-10", "Set", "ReferenceDesignation", c.orig.name, "", msg(c.orig))
	if err != nil {
		return err
	}
	return r.settle(ctx, x)
}
