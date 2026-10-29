package probe

import (
	"bytes"
	"context"
	"strings"
	"unicode"

	"github.com/OI4/oec/conformance/internal/verdict"
)

// adr01Reqs are the requirements of T7-ADR-01.
var adr01Reqs = []string{"REQ-BUS-002", "REQ-TOP-006", "REQ-TOP-010"}

// requestMethods are the methods of a request (MG 7.4).
var requestMethods = map[string]bool{"Get": true, "Set": true, "Del": true, "Call": true}

// requestToPeer reports a request of the CUT that addresses the simulated
// peer: its topic carries the AppId of the peer, or the peer as Source
// under another AppId (in the run only the peer represents its asset). A
// request under a wrongly encoded AppId cannot be attributed.
func (r *Runner) requestToPeer(n *netMsg) bool {
	return r.peer != "" && r.app != "" && requestMethods[n.method] && strings.HasSuffix(n.publisherID, "/"+r.app) &&
		(n.app == r.peer || n.source == r.peer)
}

// adr01 is T7-ADR-01 (REQ-BUS-002, REQ-TOP-006, REQ-TOP-010): the requests
// of the CUT address the destination. D7 announces a simulated peer
// (RunBus, --peer); the requirements apply if the CUT sends requests to it.
// The test waits up to the timeout for a first request and is judged at the
// end of the run (judgeADR01) over every request to the peer.
func (r *Runner) adr01(ctx context.Context) error {
	if r.peer == "" {
		return nil
	}
	return r.until(ctx, func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return len(r.toPeer) > 0
	})
}

func (r *Runner) judgeADR01() {
	const t = "T7-ADR-01"
	if r.peer == "" {
		for _, req := range adr01Reqs {
			r.add(verdict.Observation{Test: t, Req: req, Kind: verdict.Skipped, Inconclusive: true,
				Evidence: r.ev(nil, "needs a peer (--peer): D7 announces no second component the CUT could address")})
		}
		return
	}
	r.mu.Lock()
	reqs := append([]*netMsg(nil), r.toPeer...)
	r.mu.Unlock()
	if len(reqs) == 0 {
		for _, req := range adr01Reqs {
			r.skip(t, req, "the CUT sent no request to the peer %s/%s during the run (%s applies to a requester)", AnnouncedServiceType, r.peer, req)
		}
		return
	}
	checked, seen := map[string]bool{}, map[string]bool{}
	ok := func(req string, n *netMsg) {
		if !checked[req] {
			checked[req] = true
			r.add(verdict.Observation{Test: t, Req: req, Kind: verdict.Checked, Evidence: r.evTopic(n, "%s %s (MessageId %s) addresses the peer", n.method, n.resource, n.messageID)})
		}
	}
	bad := func(req string, n *netMsg, format string, a ...any) {
		r.add(verdict.Observation{Test: t, Req: req, Kind: verdict.Violation, Evidence: r.evTopic(n, format, a...)})
	}
	for _, n := range reqs {
		stOK, appOK := n.st == AnnouncedServiceType, n.app == r.peer
		if stOK && appOK {
			ok("REQ-TOP-006", n)
			ok("REQ-TOP-010", n)
			ok("REQ-BUS-002", n)
			continue
		}
		// One violation per topic: a consumer repeats its request.
		if seen[n.topic] {
			continue
		}
		seen[n.topic] = true
		if stOK {
			ok("REQ-TOP-006", n)
		} else {
			bad("REQ-TOP-006", n, "the %s (MessageId %s) to the peer carries the ServiceType %s instead of %s, the ServiceType of the peer", n.method, n.messageID, n.st, AnnouncedServiceType)
		}
		if appOK {
			ok("REQ-TOP-010", n)
		} else {
			bad("REQ-TOP-010", n, "the %s (MessageId %s) about the peer carries the AppId %s instead of %s, the AppId of the peer", n.method, n.messageID, n.app, r.peer)
		}
		bad("REQ-BUS-002", n, "the %s %s (MessageId %s) to the peer is not published on the %s topic of the peer (%s)", n.method, n.resource, n.messageID, n.method,
			Topic(AnnouncedServiceType, r.peer, n.method, n.resource, "", "")+"/…")
	}
}

// evTopic is the evidence of a message of the CUT.
func (r *Runner) evTopic(n *netMsg, format string, a ...any) verdict.Evidence {
	e := r.ev(nil, format, a...)
	e.Topic = n.topic
	return e
}

// adr02 is T7-ADR-02 (REQ-TOP-004): versions are not mixed. D7 sends its
// requests under Oi4v2 only; the requirement applies if the CUT also
// supports version 1 (it publishes under Oi4 during the run). A message
// under Oi4 that carries the MessageId of a request of D7 is an answer to
// an Oi4v2 request in the wrong version. The test sends one Get MAM of its
// own and is judged at the end of the run (judgeADR02).
func (r *Runner) adr02(ctx context.Context) error {
	x, err := r.get(ctx, "T7-ADR-02", r.st, "MAM", r.app, "", nil)
	if err != nil {
		return err
	}
	return r.wait(ctx, []*exchange{x}, nil)
}

// v1Publisher reports whether a topic under Oi4 belongs to the CUT: the
// AppId follows the ServiceType, unencoded (version 1) or encoded.
func (r *Runner) v1Publisher(topic string) bool {
	lv := strings.SplitN(topic, "/", 3)
	if len(lv) < 3 {
		return false
	}
	return strings.HasPrefix(lv[2], r.app+"/") || strings.HasPrefix(lv[2], enc(r.app)+"/")
}

func (r *Runner) judgeADR02() {
	const t, req = "T7-ADR-02", "REQ-TOP-004"
	r.mu.Lock()
	v1 := append([]Message(nil), r.v1...)
	var ids []string
	for _, x := range r.exchanges {
		ids = append(ids, x.id)
	}
	r.mu.Unlock()
	if r.v1Refused != nil {
		r.skip(t, req, "the namespace Oi4 cannot be watched: %v", r.v1Refused)
		return
	}
	own, mixed := 0, false
	for _, m := range v1 {
		if r.v1Publisher(m.Topic) {
			own++
		}
		for _, id := range ids {
			if bytes.Contains(m.Payload, []byte(id)) {
				r.violation(t, req, nil, "%s carries the MessageId %s of a request D7 sent under Oi4v2: a request received under Oi4v2 is answered under Oi4", m.Topic, id)
				mixed = true
				break
			}
		}
	}
	switch {
	case mixed:
	case own > 0:
		r.checked(t, req, nil)
	default:
		r.skip(t, req, "the CUT published nothing under the namespace Oi4 during the run (it supports one namespace version; condition of %s)", req)
	}
}

// caseVariant returns the IL with the case of its last letter swapped, or
// "" if it has no letter (T7-ADR-03).
func caseVariant(il string) string {
	b := []rune(il)
	for i := len(b) - 1; i >= 0; i-- {
		switch {
		case unicode.IsUpper(b[i]):
			b[i] = unicode.ToLower(b[i])
		case unicode.IsLower(b[i]):
			b[i] = unicode.ToUpper(b[i])
		default:
			continue
		}
		return string(b)
	}
	return ""
}

// adr03 is T7-ADR-03: a Get MAM with the exact Identification Link of each
// asset is answered on that Source with the DataSetMessage of exactly that
// IL (REQ-IL-001, REQ-IL-013, REQ-TOP-018); a Get with an IL that differs
// only in the case of one letter is not answered as the same asset
// (REQ-IL-015).
func (r *Runner) adr03(ctx context.Context) error {
	const t = "T7-ADR-03"
	all, err := r.get(ctx, t, r.st, "MAM", "", "", nil)
	if err != nil {
		return err
	}
	if err := r.wait(ctx, []*exchange{all}, nil); err != nil {
		return err
	}
	srcs := []string{r.app}
	for _, s := range sources(r.reply(t, all)) {
		if s != "" && !contains(srcs, s) && len(srcs) < maxSources {
			srcs = append(srcs, s)
		}
	}
	var each []*exchange
	for _, s := range srcs {
		x, err := r.get(ctx, t, r.st, "MAM", s, "", nil)
		if err != nil {
			return err
		}
		each = append(each, x)
	}
	// The case variant of a subordinate asset if there is one (its IL is
	// compared by the application), else of the AppId.
	target := srcs[len(srcs)-1]
	variant := caseVariant(target)
	var vx *exchange
	if variant != "" && !contains(srcs, variant) {
		if vx, err = r.get(ctx, t, r.st, "MAM", variant, "", nil); err != nil {
			return err
		}
	}
	var silent []*exchange
	if vx != nil {
		silent = append(silent, vx)
	}
	if err := r.wait(ctx, each, silent); err != nil {
		return err
	}
	for _, x := range each {
		ns := r.reply(t, x)
		if len(ns) == 0 {
			r.skip(t, "REQ-TOP-018", "Get MAM with Source %q is not answered (judged by T7-GET-01)", x.source)
			continue
		}
		names := sources(ns)
		if len(names) != 1 || names[0] != x.source {
			for _, req := range []string{"REQ-IL-001", "REQ-IL-013", "REQ-TOP-018"} {
				r.violation(t, req, x, "Get MAM with the Source %q is answered with the DataSetMessages of %q; want exactly one with the DataSetWriterName %q, character by character", x.source, names, x.source)
			}
			continue
		}
		for _, req := range []string{"REQ-IL-001", "REQ-IL-013", "REQ-TOP-018"} {
			r.checked(t, req, x)
		}
	}
	if vx == nil {
		r.skip(t, "REQ-IL-015", "the Identification Link %q has no letter whose case D7 can change", target)
		return nil
	}
	answers, _, others := r.snapshot(vx)
	ok := true
	for _, a := range append(answers, others...) {
		if names := sources([]*netMsg{a}); len(names) > 0 {
			r.violation(t, "REQ-IL-015", vx, "Get MAM for %q, which differs from the asset %q only in case, is answered on %s with the DataSetMessages of %q", variant, target, a.topic, names)
			ok = false
		}
	}
	if ok {
		r.checked(t, "REQ-IL-015", vx)
	}
	return nil
}
