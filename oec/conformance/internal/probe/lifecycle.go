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

// The connection and life cycle tests (C-066) judge what the proxy
// attachment saw (DESIGN.md §3.4): the CONNECT packets, the TLS handshakes
// and the PUBLISH packets of the CUT. They run after the other tests:
//
//  1. the proxy answers the next three connection attempts with TLS 1.1
//     only, a certificate of an untrusted CA and a certificate for a wrong
//     host name (T7-CON-02) and drops the relayed connection without
//     DISCONNECT, so the broker publishes the Will (T7-LIFE-03);
//  2. the CUT reconnects (it must refuse the three attempts) and publishes
//     its Birth (T7-LIFE-02);
//  3. every connection of the run is judged.
var lifecycleTests = []string{"T7-CON-01", "T7-CON-02", "T7-CON-03", "T7-CON-04", "T7-LIFE-01", "T7-LIFE-02", "T7-LIFE-03"}

func isLifecycleTest(id string) bool { return contains(lifecycleTests, id) }

// NoProxy is the reason of the inconclusive skips without the proxy.
const NoProxy = "needs the proxy attachment (--proxy-listen): the CONNECT, the TLS handshake and the PUBLISH packets of the CUT are not visible on the bus"

// lifecycleSelected reports whether a connection or life cycle test runs.
func (r *Runner) lifecycleSelected() bool {
	for _, id := range lifecycleTests {
		if r.Selected(id) {
			return true
		}
	}
	return false
}

// awaitProxy waits until the CUT is connected through the proxy and has
// published (its Birth), before the discovery.
func (r *Runner) awaitProxy(ctx context.Context) error {
	p := r.opt.Proxy
	wctx, cancel := context.WithTimeout(ctx, r.opt.ProxyWait)
	defer cancel()
	var at time.Time
	ok := p.Wait(wctx, func(cs []proxy.Conn) bool {
		for _, c := range cs {
			if c.Relayed() {
				at = c.ConnectTime
				return true
			}
		}
		return false
	})
	if !ok {
		return ctx.Err()
	}
	wctx, cancel = context.WithDeadline(ctx, at.Add(r.opt.Timeout))
	defer cancel()
	p.Wait(wctx, func(cs []proxy.Conn) bool {
		for _, c := range cs {
			if c.Relayed() && len(c.Publishes) > 0 {
				return true
			}
		}
		return false
	})
	return ctx.Err()
}

// lifecycle runs the connection phase and judges the connections.
func (r *Runner) lifecycle(ctx context.Context) error {
	if r.opt.Proxy == nil {
		r.noProxy()
		return nil
	}
	p := r.opt.Proxy
	before := p.Conns()
	last := -1
	for _, c := range before {
		if c.Relayed() {
			last = c.ID
		}
	}
	if last < 0 {
		r.lifecycleSkip(true, "the CUT did not connect through the proxy within %s", r.opt.ProxyWait)
		return nil
	}
	rejects := []proxy.Phase{}
	if r.Selected("T7-CON-02") {
		rejects = []proxy.Phase{proxy.TLS11, proxy.UntrustedCA, proxy.WrongHost}
	}
	p.Reject(rejects...)
	dropped := time.Now()
	r.mu.Lock()
	r.dropped = dropped
	r.mu.Unlock()
	p.Drop()
	// The reconnection: a new relayed connection with its Birth (or
	// Timeout after its CONNECT), and the Will on the bus (or Timeout
	// after the drop).
	deadline := dropped.Add(r.opt.ProxyWait)
	for {
		cs := p.Conns()
		var next *proxy.Conn
		for i := range cs {
			if cs[i].ID > last && cs[i].Relayed() {
				next = &cs[i]
				break
			}
		}
		now := time.Now()
		r.mu.Lock()
		will := r.busWill != nil
		r.mu.Unlock()
		reconnected := next != nil && (r.birth(*next) >= 0 || now.Sub(next.ConnectTime) >= r.opt.Timeout || !next.Closed.IsZero())
		if reconnected && (will || now.Sub(dropped) >= r.opt.Timeout) || now.After(deadline) {
			break
		}
		if err := sleep(ctx, 50*time.Millisecond); err != nil {
			return err
		}
	}
	if err := sleep(ctx, r.opt.Grace); err != nil {
		return err
	}
	left := p.Clear()
	r.judgeLifecycle(p.Conns(), last, left)
	return nil
}

// lifecycleSkip skips every requirement of the selected tests.
func (r *Runner) lifecycleSkip(inconclusive bool, format string, a ...any) {
	for _, id := range lifecycleTests {
		if !r.Selected(id) {
			continue
		}
		for _, req := range r.cites[id] {
			r.add(verdict.Observation{Test: id, Req: req, Kind: verdict.Skipped, Inconclusive: inconclusive, Evidence: r.ev(nil, format, a...)})
		}
	}
}

// noProxy: without the proxy the tests are inconclusive; T7-CON-04 reports
// a retained delivery of the CUT on D7's subscription as a finding.
func (r *Runner) noProxy() {
	r.mu.Lock()
	retained := append([]*netMsg(nil), r.retained...)
	r.mu.Unlock()
	if r.Selected("T7-CON-04") {
		for _, n := range retained {
			r.add(verdict.Observation{Test: "T7-CON-04", Req: "REQ-MQTT-018", Kind: verdict.Violation,
				Evidence: verdict.Evidence{Participant: r.st + "/" + r.app, Topic: n.topic, Detail: "delivered to D7 as a retained message: the CUT published it with Retain"}})
		}
	}
	r.lifecycleSkip(true, "%s", NoProxy)
}

// cev is the evidence of a connection.
func (r *Runner) cev(c proxy.Conn, format string, a ...any) verdict.Evidence {
	return verdict.Evidence{Participant: r.st + "/" + r.app, Detail: fmt.Sprintf("connection %d (%s, %s): ", c.ID, c.Phase, c.Accepted.Format(time.RFC3339Nano)) + fmt.Sprintf(format, a...)}
}

func (r *Runner) cobs(test, req string, kind verdict.Kind, c proxy.Conn, format string, a ...any) {
	r.add(verdict.Observation{Test: test, Req: req, Kind: kind, Evidence: r.cev(c, format, a...)})
}

// self reports whether a PUBLISH of the CUT is about itself on the
// resource (topic Oi4v2/<ST>/<AppId>/Pub/<resource>/<AppId>), and returns
// the parsed message.
func (r *Runner) self(p proxy.Pub, resource string) (*netMsg, bool) {
	lv := strings.Split(p.Topic, "/")
	if len(lv) != 6 || lv[0] != "Oi4v2" || lv[3] != "Pub" || lv[4] != resource || dec(lv[2]) != r.app || dec(lv[5]) != r.app {
		return nil, false
	}
	n := parse(Message{Topic: p.Topic, Payload: p.Payload})
	return n, n != nil
}

// birth returns the index of the Birth among the PUBLISH packets of the
// connection (Pub MAM of the CUT about itself, no answer), or -1.
func (r *Runner) birth(c proxy.Conn) int {
	for i, p := range c.Publishes {
		if n, ok := r.self(p, "MAM"); ok && n.replyTo == "" {
			return i
		}
	}
	return -1
}

// health returns the Health state and score of a self Health message.
func health(n *netMsg) (state string, score float64, id int64, ok bool) {
	if len(n.msgs) != 1 {
		return "", 0, 0, false
	}
	p, _ := n.msgs[0].payload.(map[string]any)
	state, _ = p["Health"].(string)
	score, _ = p["HealthScore"].(float64)
	return state, score, n.msgs[0].id, state != ""
}

func (r *Runner) judgeLifecycle(cs []proxy.Conn, firstOld int, left []proxy.Phase) {
	var relay []proxy.Conn // relay phase with a CONNECT
	for _, c := range cs {
		if c.Phase == proxy.Relay && c.Connect != nil {
			relay = append(relay, c)
		}
	}
	if r.Selected("T7-CON-01") {
		r.con01(relay)
	}
	if r.Selected("T7-CON-02") {
		r.con02(cs, left)
	}
	if r.Selected("T7-CON-03") {
		r.con03(relay)
	}
	if r.Selected("T7-CON-04") {
		r.con04(relay)
	}
	if r.Selected("T7-LIFE-01") {
		r.life01(relay)
	}
	if r.Selected("T7-LIFE-02") {
		r.life02(cs, firstOld)
	}
	if r.Selected("T7-LIFE-03") {
		r.life03(relay)
	}
}

// con01: protocol level ≥ 4 (MQTT 3.1.1 or later) and the client settings
// of MG 6.2.3 (ClientId, keep alive 60 s, clean session).
func (r *Runner) con01(relay []proxy.Conn) {
	const t = "T7-CON-01"
	ids := map[string]bool{}
	for _, c := range relay {
		k := c.Connect
		if k.Level < 4 {
			for _, req := range []string{"REQ-MQTT-001", "REQ-CMP-001"} {
				r.cobs(t, req, verdict.Violation, c, "CONNECT with protocol %q level %d (MQTT 3.1); MQTT 3.1.1 (level 4) or later is required", k.ProtocolName, k.Level)
			}
		} else {
			for _, req := range []string{"REQ-MQTT-001", "REQ-CMP-001"} {
				r.cobs(t, req, verdict.Checked, c, "CONNECT with protocol %q level %d", k.ProtocolName, k.Level)
			}
		}
		ok := true
		switch {
		case k.ClientID == "" && !k.CleanSession:
			ok = false
			r.cobs(t, "REQ-CMP-009", verdict.Violation, c, "CONNECT with an empty ClientId and clean session false: a persistent session needs a ClientId (MG 6.2.3)")
		case k.ClientID == "":
			ok = false
			r.cobs(t, "REQ-CMP-009", verdict.Violation, c, "CONNECT with an empty ClientId: the ClientId is the container name or a stable unique id (MG 6.2.3)")
		}
		ids[k.ClientID] = true
		if k.KeepAlive != 60 {
			r.cobs(t, "REQ-CMP-009", verdict.Note, c, "keep alive %d s; the default of MG 6.2.3 is 60 s (a configured value is allowed)", k.KeepAlive)
		}
		if !k.CleanSession {
			r.cobs(t, "REQ-CMP-009", verdict.Note, c, "clean session false; the default of MG 6.2.3 is true (a configured value is allowed)")
		}
		if ok {
			r.cobs(t, "REQ-CMP-009", verdict.Checked, c, "ClientId %q, keep alive %d s, clean session %t", k.ClientID, k.KeepAlive, k.CleanSession)
		}
	}
	if len(ids) > 1 && len(relay) > 1 {
		var list []string
		for _, c := range relay {
			list = append(list, strconv.Quote(c.Connect.ClientID))
		}
		r.cobs(t, "REQ-CMP-009", verdict.Violation, relay[len(relay)-1], "the ClientId changes between the connections (%s): it is the container name or a stable unique id (MG 6.2.3)", strings.Join(list, ", "))
	}
	if len(relay) == 0 {
		r.lifecycleSkipTest(t, "no CONNECT of the CUT was relayed")
	}
}

func (r *Runner) lifecycleSkipTest(test, format string, a ...any) {
	for _, req := range r.cites[test] {
		r.add(verdict.Observation{Test: test, Req: req, Kind: verdict.Skipped, Inconclusive: true, Evidence: r.ev(nil, format, a...)})
	}
}

// con02: TLS 1.2 or higher only, the certificate of the broker verified
// (chain and host name): the CUT must not send a CONNECT in the reject
// phases.
func (r *Runner) con02(cs []proxy.Conn, left []proxy.Phase) {
	const t = "T7-CON-02"
	for _, c := range cs {
		if c.Phase == proxy.Relay && c.Connect != nil && c.TLSVersion != "" {
			r.cobs(t, "REQ-MQTT-005", verdict.Checked, c, "relayed over %s", c.TLSVersion)
		}
	}
	for _, c := range cs {
		var reqs []string
		var what string
		switch c.Phase {
		case proxy.TLS11:
			reqs, what = []string{"REQ-MQTT-005", "REQ-CMP-002"}, "the proxy offered TLS 1.1 only"
		case proxy.UntrustedCA:
			reqs, what = []string{"REQ-MQTT-008", "REQ-CMP-002"}, "the proxy presented a certificate of an untrusted CA"
		case proxy.WrongHost:
			reqs, what = []string{"REQ-MQTT-008", "REQ-CMP-002"}, "the proxy presented a certificate for the host name "+proxy.WrongHostName
		default:
			continue
		}
		for _, req := range reqs {
			switch {
			case c.Connect != nil:
				r.cobs(t, req, verdict.Violation, c, "%s; the CUT completed the handshake (%s) and sent its CONNECT", what, c.TLSVersion)
			case c.HandshakeErr != "":
				r.cobs(t, req, verdict.Checked, c, "%s; the CUT refused the handshake (%s)", what, c.HandshakeErr)
			default:
				r.cobs(t, req, verdict.Checked, c, "%s; the CUT sent no CONNECT (%s)", what, c.Err)
			}
		}
	}
	for _, ph := range left {
		reqs := []string{"REQ-MQTT-008", "REQ-CMP-002"}
		if ph == proxy.TLS11 {
			reqs = []string{"REQ-MQTT-005", "REQ-CMP-002"}
		}
		for _, req := range reqs {
			r.add(verdict.Observation{Test: t, Req: req, Kind: verdict.Skipped, Inconclusive: true,
				Evidence: r.ev(nil, "the phase %s was not used: the CUT did not connect again within %s after the drop", ph, r.opt.ProxyWait)})
		}
	}
}

// con03: user name and password, or a client certificate.
func (r *Runner) con03(relay []proxy.Conn) {
	const t = "T7-CON-03"
	for _, c := range relay {
		k := c.Connect
		switch {
		case c.ClientCert:
			r.cobs(t, "REQ-MQTT-009", verdict.Checked, c, "client certificate presented")
		case k.HasUsername && k.HasPassword:
			r.cobs(t, "REQ-MQTT-009", verdict.Checked, c, "user name %q and password", k.Username)
		default:
			r.cobs(t, "REQ-MQTT-009", verdict.Violation, c, "CONNECT without client certificate and without user name and password (user name %t, password %t)", k.HasUsername, k.HasPassword)
		}
	}
	if len(relay) == 0 {
		r.lifecycleSkipTest(t, "no CONNECT of the CUT was relayed")
	}
}

// con04: no message with Retain, the Will included.
func (r *Runner) con04(relay []proxy.Conn) {
	const t = "T7-CON-04"
	bad := false
	for _, c := range relay {
		if w := c.Connect.Will; w != nil && w.Retain {
			bad = true
			r.cobs(t, "REQ-MQTT-018", verdict.Violation, c, "the Will (%s) is registered with Will Retain", w.Topic)
		}
		for _, p := range c.Publishes {
			if p.Retain {
				bad = true
				r.cobs(t, "REQ-MQTT-018", verdict.Violation, c, "PUBLISH %s with Retain", p.Topic)
			}
		}
	}
	if !bad && len(relay) > 0 {
		n := 0
		for _, c := range relay {
			n += len(c.Publishes)
		}
		r.cobs(t, "REQ-MQTT-018", verdict.Checked, relay[len(relay)-1], "%d connection(s), %d PUBLISH packet(s), none with Retain", len(relay), n)
	}
	if len(relay) == 0 {
		r.lifecycleSkipTest(t, "no CONNECT of the CUT was relayed")
	}
}

// willDoc decodes the Will payload.
type willDoc struct {
	MessageID string           `json:"MessageId"`
	Messages  []map[string]any `json:"Messages"`
}

// life01: the Will of every CONNECT (topic, Retain, no Timestamp and
// SequenceNumber, created at connect).
func (r *Runner) life01(relay []proxy.Conn) {
	const t = "T7-LIFE-01"
	topic := Topic(r.st, r.app, "Pub", "Health", r.app, "")
	ids := map[string]int{}
	for _, c := range relay {
		w := c.Connect.Will
		if w == nil {
			for _, req := range []string{"REQ-LIFE-008", "REQ-CMP-003"} {
				r.cobs(t, req, verdict.Violation, c, "CONNECT without Will")
			}
			continue
		}
		ok := true
		if w.Topic != topic {
			ok = false
			r.cobs(t, "REQ-LIFE-008", verdict.Violation, c, "the Will topic is %s instead of %s", w.Topic, topic)
		}
		if w.Retain {
			ok = false
			r.cobs(t, "REQ-LIFE-008", verdict.Violation, c, "the Will is registered with Will Retain")
		}
		if ok {
			r.cobs(t, "REQ-LIFE-008", verdict.Checked, c, "Will on %s, Will Retain false", w.Topic)
		}
		r.cobs(t, "REQ-CMP-003", verdict.Checked, c, "Will registered in the CONNECT")
		var doc willDoc
		if err := json.Unmarshal(w.Payload, &doc); err != nil {
			r.cobs(t, "REQ-LIFE-016", verdict.Violation, c, "the Will payload is no JSON NetworkMessage: %v", err)
			continue
		}
		stamped := false
		for _, d := range doc.Messages {
			for _, k := range []string{"Timestamp", "SequenceNumber"} {
				if _, has := d[k]; has {
					stamped = true
					r.cobs(t, "REQ-LIFE-010", verdict.Violation, c, "the DataSetMessage of the Will carries %s", k)
				}
			}
		}
		if !stamped {
			r.cobs(t, "REQ-LIFE-010", verdict.Checked, c, "the Will carries no Timestamp and no SequenceNumber")
		}
		ms, err := strconv.ParseInt(strings.SplitN(doc.MessageID, "-", 2)[0], 10, 64)
		if err != nil {
			r.cobs(t, "REQ-LIFE-016", verdict.Violation, c, "the MessageId %q of the Will carries no time", doc.MessageID)
			continue
		}
		created := time.UnixMilli(ms).UTC()
		if d := created.Sub(c.ConnectTime); d > r.opt.Tolerance || d < -r.opt.Tolerance {
			r.cobs(t, "REQ-LIFE-016", verdict.Violation, c, "the MessageId %s of the Will was created at %s, %s from the CONNECT at %s (tolerance %s): the Will is created anew for every connection",
				doc.MessageID, created.Format(time.RFC3339Nano), d.Round(time.Millisecond), c.ConnectTime.Format(time.RFC3339Nano), r.opt.Tolerance)
		} else if prev, dup := ids[doc.MessageID]; dup {
			r.cobs(t, "REQ-LIFE-016", verdict.Violation, c, "the Will repeats the MessageId %s of connection %d", doc.MessageID, prev)
		} else {
			r.cobs(t, "REQ-LIFE-016", verdict.Checked, c, "the Will (MessageId %s) was created %s from the CONNECT", doc.MessageID, d.Round(time.Millisecond))
		}
		ids[doc.MessageID] = c.ID
	}
	if len(relay) == 0 {
		r.lifecycleSkipTest(t, "no CONNECT of the CUT was relayed")
	}
}

// life02: the Birth first after every connect (REQ-LIFE-001,
// REQ-LIFE-015) and after a recovery from FAILURE_1/0 (REQ-LIFE-017).
func (r *Runner) life02(cs []proxy.Conn, firstOld int) {
	const t = "T7-LIFE-02"
	reconnected := false
	failures := 0
	for _, c := range cs {
		if !c.Relayed() {
			continue
		}
		if c.ID > firstOld {
			reconnected = true
		}
		b := r.birth(c)
		end := c.Closed
		if end.IsZero() {
			end = time.Now()
		}
		switch {
		case b >= 0:
			r.cobs(t, "REQ-LIFE-001", verdict.Checked, c, "Birth %s after %s", c.Publishes[b].Topic, c.Publishes[b].Time.Sub(c.ConnectTime).Round(time.Millisecond))
			if b == 0 {
				r.cobs(t, "REQ-LIFE-015", verdict.Checked, c, "the Birth is the first message")
			} else {
				r.cobs(t, "REQ-LIFE-015", verdict.Violation, c, "the first message after the CONNECT is %s, the Birth follows as message %d", c.Publishes[0].Topic, b+1)
			}
		case end.Sub(c.ConnectTime) >= r.opt.Timeout:
			r.cobs(t, "REQ-LIFE-001", verdict.Violation, c, "no Birth (Pub MAM of the CUT about itself) within %s after the CONNECT (%d messages)", r.opt.Timeout, len(c.Publishes))
		default:
			r.cobs(t, "REQ-LIFE-001", verdict.Skipped, c, "the connection ended %s after the CONNECT without a Birth", end.Sub(c.ConnectTime).Round(time.Millisecond))
		}
		// REQ-LIFE-017: after a self Health FAILURE_1/0 the next Health
		// that reports recovery follows a new Birth.
		failing, rebirth := false, false
		for _, p := range c.Publishes {
			if n, ok := r.self(p, "MAM"); ok && n.replyTo == "" {
				rebirth = true
				continue
			}
			n, ok := r.self(p, "Health")
			if !ok || n.replyTo != "" {
				continue
			}
			state, score, _, ok := health(n)
			if !ok {
				continue
			}
			switch {
			case state == "FAILURE_1" && score == 0:
				failing, rebirth = true, false
				failures++
			case failing && rebirth:
				failing = false
				r.cobs(t, "REQ-LIFE-017", verdict.Checked, c, "after FAILURE_1/0 a new Birth, then Health %s", state)
			case failing:
				failing = false
				r.cobs(t, "REQ-LIFE-017", verdict.Violation, c, "after FAILURE_1/0 the Health %s (%s) follows without a new Birth", state, p.Time.Format(time.RFC3339Nano))
			}
		}
	}
	if !reconnected {
		r.add(verdict.Observation{Test: t, Req: "REQ-LIFE-001", Kind: verdict.Skipped, Inconclusive: true,
			Evidence: r.ev(nil, "the CUT did not reconnect through the proxy within %s after the drop", r.opt.ProxyWait)})
	}
	if failures == 0 {
		r.skip(t, "REQ-LIFE-017", "the CUT reported no FAILURE_1/0 about itself while connected")
	}
}

// life03: the Will is the Health of the CUT with FAILURE_1/0 and the
// DataSetWriterId of its own Health.
func (r *Runner) life03(relay []proxy.Conn) {
	const t = "T7-LIFE-03"
	own := map[int64]bool{}
	for _, c := range relay {
		for _, p := range c.Publishes {
			if n, ok := r.self(p, "Health"); ok {
				if _, _, id, ok := health(n); ok {
					own[id] = true
				}
			}
		}
	}
	var ids []string
	for id := range own {
		ids = append(ids, strconv.FormatInt(id, 10))
	}
	r.mu.Lock()
	bus := r.busWill
	r.mu.Unlock()
	seen := "the broker published it after the drop"
	if bus == nil {
		seen = "the bus showed no Will after the drop"
	}
	for _, c := range relay {
		w := c.Connect.Will
		if w == nil {
			continue
		}
		n := parse(Message{Topic: w.Topic, Payload: w.Payload})
		if n == nil {
			r.cobs(t, "REQ-LIFE-009", verdict.Violation, c, "the Will is no NetworkMessage")
			continue
		}
		state, score, id, ok := health(n)
		switch {
		case n.method != "Pub" || n.resource != "Health":
			r.cobs(t, "REQ-LIFE-009", verdict.Violation, c, "the Will is published on %s, not as Pub Health", w.Topic)
		case !ok:
			r.cobs(t, "REQ-LIFE-009", verdict.Violation, c, "the Will carries %d DataSetMessages; one Health DataSetMessage is required", len(n.msgs))
		case state != "FAILURE_1" || score != 0:
			r.cobs(t, "REQ-LIFE-009", verdict.Violation, c, "the Will reports Health %s, HealthScore %v instead of FAILURE_1/0", state, score)
		case len(own) > 0 && !own[id]:
			r.cobs(t, "REQ-LIFE-009", verdict.Violation, c, "the Will uses DataSetWriterId %d, the Health of the CUT %s", id, strings.Join(ids, ", "))
		case len(own) > 1:
			r.cobs(t, "REQ-LIFE-009", verdict.Violation, c, "the Health of the CUT uses several DataSetWriterIds (%s); the Will and all later Health messages use one", strings.Join(ids, ", "))
		default:
			r.cobs(t, "REQ-LIFE-009", verdict.Checked, c, "Will Health FAILURE_1/0 with DataSetWriterId %d; %s", id, seen)
		}
	}
}
