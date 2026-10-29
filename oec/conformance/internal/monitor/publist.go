package monitor

// T7-MON-14: the publication rules of the PublicationList (MG 8.3.8). The
// rules of a component are the entries of its last observed Pub
// PublicationList (unsolicited or the answer to a Get). A Set or Del
// PublicationList addressed to the component makes them uncertain until the
// next Pub PublicationList; publications within the tolerance after a
// changed entry or a connect are not judged.

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/OI4/oec/conformance/internal/topic"
	"github.com/OI4/oec/conformance/internal/verdict"
)

const testPublist = "T7-MON-14"

// publistReqs are the requirements of T7-MON-14.
var publistReqs = []string{"REQ-MQTT-019", "REQ-RES-PUBLIST-008", "REQ-RES-PUBLIST-009", "REQ-RES-PUBLIST-010",
	"REQ-RES-PUBLIST-011", "REQ-RES-PUBLIST-013", "REQ-RES-PUBLIST-014", "REQ-RES-PUBLIST-015"}

// form is a set of publication forms (MG 8.3.8).
type form int

const (
	formApp    form = 1 << iota // APPLICATION_2: Pub/<Resource>
	formSrc                     // SOURCE_3: Pub/<Resource>/<Source>
	formFilter                  // FILTER_4: Pub/<Resource>/<Source>/<Filter>
)

var formOrder = []form{formApp, formSrc, formFilter}

var formName = map[form]string{formApp: "APPLICATION_2", formSrc: "SOURCE_3", formFilter: "FILTER_4"}

var formReq = map[form]string{formApp: "REQ-RES-PUBLIST-009", formSrc: "REQ-RES-PUBLIST-010", formFilter: "REQ-RES-PUBLIST-011"}

// modeForms are the forms that a Mode value enables (MG 8.3.8; OFF_0 and
// ON_REQUEST_1 enable none).
var modeForms = map[int64]form{
	2: formApp, 3: formSrc, 4: formFilter, 5: formApp | formSrc, 6: formApp | formFilter, 7: formSrc | formFilter,
	8: formApp | formSrc | formFilter, 9: formFilter, 10: formSrc, 11: formApp, 12: formSrc | formFilter,
	13: formApp | formFilter, 14: formApp | formSrc, 15: formApp | formSrc | formFilter,
}

// noFilter are the resources without Filter: FILTER_4 publishes them on
// Pub/<Resource>/<Source> like SOURCE_3 (MG 8.3.8, SF-067 (c)).
var noFilter = map[string]bool{"MAM": true, "Health": true, "Profile": true, "ReferenceDesignation": true,
	"License": true, "LicenseText": true, "RtLicense": true}

// notJudged are the resources whose publications T7-MON-14 does not judge:
// the lists themselves (T7-MON-15) and Metadata (no DataSetMessage).
var notJudged = map[string]bool{"PublicationList": true, "SubscriptionList": true, "Metadata": true}

func (f form) String() string {
	var s []string
	for _, x := range formOrder {
		if f&x != 0 {
			s = append(s, formName[x])
		}
	}
	return strings.Join(s, " and ")
}

// topicForm is the form of a Pub topic; for a resource without Filter the
// topic with Source is both SOURCE_3 and FILTER_4.
func topicForm(t *topic.Parsed) form {
	switch {
	case t.Source == "":
		return formApp
	case t.Filter != "":
		return formFilter
	case noFilter[t.Resource]:
		return formSrc | formFilter
	}
	return formSrc
}

// slot is the form under which a publication is tracked.
func slot(f form) form {
	if f&formSrc != 0 {
		return formSrc
	}
	return f
}

// rule is one entry of the PublicationList with its observed publications.
type rule struct {
	k         key // Resource, Source, Filter
	id        int64
	idOK      bool
	mode      int64
	modeName  string
	interval  int64              // ms
	prec      map[string]float64 // Precisions > 0
	line      int
	since     time.Time               // entry learned or changed, or the last connect
	last      map[form]time.Time      // last unsolicited publication per form
	lastLine  map[form]int            // its line
	payload   map[form]map[string]any // its payload
	latest    map[string]any          // last unsolicited payload in any form
	latestLin int
}

// expect is a changed content seen in the answer to a Get that the
// component publishes within the tolerance (REQ-RES-PUBLIST-013,
// REQ-MQTT-019).
type expect struct {
	r        *rule
	payload  map[string]any
	deadline time.Time
	x        verdict.Evidence
	prevLine int
}

type publist struct {
	rules     map[key]*rule
	pubs      bool // the component published
	uncertain bool // a Set or Del PublicationList since the last Pub PublicationList
	offline   bool // after a Will or Close, until the next Birth
	expects   []*expect
}

func (e *Engine) pcheck(p *part, req string) { e.check(p, testPublist, req) }

func (e *Engine) pviolation(p *part, req string, x verdict.Evidence) {
	e.add(testPublist, req, verdict.Violation, x)
}

// parseMode returns the value of a Mode; an absent Mode is ON_REQUEST_1.
func parseMode(v any) (int64, string, bool) {
	switch m := v.(type) {
	case nil:
		return 1, "ON_REQUEST_1 (no Mode)", true
	case string:
		i := strings.LastIndex(m, "_")
		n, err := strconv.ParseInt(m[i+1:], 10, 64)
		return n, m, err == nil && i > 0
	case json.Number:
		n, err := m.Int64()
		return n, m.String(), err == nil
	}
	return 0, "", false
}

func number(v any) (float64, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case float64:
		return n, true
	}
	return 0, false
}

// learn reads the entries of a Pub PublicationList of the component.
func (e *Engine) learn(p *part, m Message, ds []dsm) {
	p.pl.uncertain = false
	for _, d := range e.resourceDSMs(ds) {
		res := str(d.payload, "Resource")
		mode, name, ok := parseMode(d.payload["Mode"])
		if res == "" || !ok {
			continue
		}
		r := &rule{k: key{res, str(d.payload, "Source"), str(d.payload, "Filter")}, mode: mode, modeName: name, line: m.Line,
			since: m.Received, prec: map[string]float64{}}
		r.id, r.idOK = num(d.payload, "DataSetWriterId")
		if iv, ok := num(d.payload, "Interval"); ok && iv > 0 {
			r.interval = iv
		}
		if pr, ok := d.payload["Precisions"].(map[string]any); ok {
			for f, v := range pr {
				if x, ok := number(v); ok && x > 0 {
					r.prec[f] = x
				}
			}
		}
		if mode == 1 && res != "MAM" && res != "Health" {
			e.pcheck(p, "REQ-RES-PUBLIST-008")
		}
		old := p.pl.rules[r.k]
		if old != nil && old.mode == r.mode && old.interval == r.interval && fmt.Sprint(old.prec) == fmt.Sprint(r.prec) && old.id == r.id {
			old.line = m.Line
			continue
		}
		r.last, r.lastLine, r.payload = map[form]time.Time{}, map[form]int{}, map[form]map[string]any{}
		if p.pl.rules == nil {
			p.pl.rules = map[key]*rule{}
		}
		p.pl.rules[r.k] = r
	}
}

// publistChange notes a Set or Del PublicationList addressed to q.
func (e *Engine) publistChange(q *part, t *topic.Parsed) {
	if t.Resource == "PublicationList" {
		q.pl.uncertain = true
		q.pl.expects = nil
	}
}

// publistConnect restarts the observation of the rules at a Birth.
func (e *Engine) publistConnect(p *part, at time.Time) {
	p.pl.offline = false
	for _, r := range p.pl.rules {
		r.since, r.latest = at, nil
		r.last, r.lastLine, r.payload = map[form]time.Time{}, map[form]int{}, map[form]map[string]any{}
	}
}

// publistEnd stops the observation at a Will or Close.
func (e *Engine) publistEnd(p *part) {
	p.pl.offline = true
	p.pl.expects = nil
}

// find returns the entry of a published DataSetMessage: by Resource and
// Source, then by the Filter of the topic or the DataSetWriterId.
func (p *part) find(t *topic.Parsed, d dsm) *rule {
	var cands []*rule
	for _, r := range p.pl.rules {
		if r.k.resource == t.Resource && r.k.name == d.name {
			cands = append(cands, r)
		}
	}
	if len(cands) > 1 && t.Filter != "" {
		var c []*rule
		for _, r := range cands {
			if r.k.group == t.Filter {
				c = append(c, r)
			}
		}
		cands = c
	}
	if len(cands) > 1 && d.idOK {
		var c []*rule
		for _, r := range cands {
			if r.idOK && r.id == d.id {
				c = append(c, r)
			}
		}
		cands = c
	}
	if len(cands) != 1 {
		return nil
	}
	return cands[0]
}

// deviation compares two payloads of a DataSet: changed reports any
// difference; significant reports a difference that is not a numeric
// field below its Precision (MG 8.3.8); small reports a numeric field that
// changed below its Precision.
func deviation(old, cur map[string]any, prec map[string]float64) (changed, significant, small bool) {
	names := map[string]bool{}
	for f := range old {
		names[f] = true
	}
	for f := range cur {
		names[f] = true
	}
	for f := range names {
		if same(old[f], cur[f]) {
			continue
		}
		changed = true
		a, ok1 := number(old[f])
		b, ok2 := number(cur[f])
		if pr := prec[f]; pr > 0 && ok1 && ok2 && math.Abs(b-a) < pr {
			small = true
			continue
		}
		significant = true
	}
	return changed, significant, small
}

// same compares two JSON values; numbers by value.
func same(a, b any) bool {
	if x, ok := number(a); ok {
		y, ok := number(b)
		return ok && x == y
	}
	switch x := a.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for k, v := range x {
			if w, ok := y[k]; !ok || !same(v, w) {
				return false
			}
		}
		return true
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !same(x[i], y[i]) {
				return false
			}
		}
		return true
	}
	return a == b
}

// publistPub judges a Pub of the component against its PublicationList.
func (e *Engine) publistPub(p *part, m Message, t *topic.Parsed, msg map[string]any, ds []dsm, birth bool) {
	p.pl.pubs = true
	e.expire(p, m.Received)
	if t.Resource == "PublicationList" {
		e.learn(p, m, ds)
		return
	}
	if notJudged[t.Resource] || len(p.pl.rules) == 0 {
		return
	}
	if role, _ := selfHealth(p, t, ds); role != "" {
		return
	}
	unsolicited := str(msg, "ReplyTo") == ""
	tf := topicForm(t)
	for _, d := range e.resourceDSMs(ds) {
		r := p.find(t, d)
		if r == nil || d.payload == nil {
			continue
		}
		if !unsolicited {
			e.answer(p, m, d, r, e.locales[str(msg, "ReplyTo")])
			continue
		}
		judge := !birth && !p.pl.uncertain && !p.pl.offline && !m.Received.Before(r.since.Add(e.opt.Tolerance))
		forms := modeForms[r.mode]
		if judge {
			e.mode(p, m, t, d, r, tf, forms)
		}
		s := slot(tf & forms)
		if tf&forms == 0 {
			s = slot(tf)
		}
		if judge && forms&tf != 0 && t.Resource != "Event" {
			e.interval(p, m, d, r, s)
		}
		e.resolve(p, m, r, d.payload)
		r.last[s], r.lastLine[s], r.payload[s] = m.Received, m.Line, d.payload
		r.latest, r.latestLin = d.payload, m.Line
	}
}

// mode is REQ-RES-PUBLIST-008 … 011: the form of an unsolicited
// publication is enabled by the Mode of its entry.
func (e *Engine) mode(p *part, m Message, t *topic.Parsed, d dsm, r *rule, tf, forms form) {
	switch {
	case r.mode == 1:
		if t.Resource == "MAM" || t.Resource == "Health" {
			return // the Mode itself violates REQ-RES-PUBLIST-012 (T7-MON-01)
		}
		e.pviolation(p, "REQ-RES-PUBLIST-008", ev(p, m, d.ptr, "unsolicited publication of %s, its PublicationList entry (line %d) has the Mode %s: only as answer to a Get", r.k, r.line, r.modeName))
	case forms == 0:
		// OFF_0 (REQ-RES-PUBLIST-007, T7-SET-08) or an unknown value
	case tf&forms == 0:
		for _, f := range formOrder {
			if forms&f != 0 {
				e.pviolation(p, formReq[f], ev(p, m, d.ptr, "%s is published in the form %s, its PublicationList entry (line %d) has the Mode %s (%s)", r.k, tf, r.line, r.modeName, forms))
			}
		}
	default:
		for _, f := range formOrder {
			if tf&forms&f != 0 {
				e.pcheck(p, formReq[f])
			}
		}
	}
}

// interval is REQ-RES-PUBLIST-013 … 015 between two unsolicited
// publications in the same form.
func (e *Engine) interval(p *part, m Message, d dsm, r *rule, s form) {
	if r.interval > 0 {
		e.gap(p, r, s, m.Received, ev(p, m, d.ptr, ""), "")
		return
	}
	prev := r.payload[s]
	if len(r.prec) == 0 || prev == nil {
		return
	}
	changed, significant, small := deviation(prev, d.payload, r.prec)
	switch {
	case small && !significant:
		e.pviolation(p, "REQ-RES-PUBLIST-015", ev(p, m, d.ptr+"/Payload", "%s is published again (previous line %d) although no field deviates by its Precision %v", r.k, r.lastLine[s], r.prec))
	case changed:
		e.pcheck(p, "REQ-RES-PUBLIST-015")
	}
}

// gap judges the time from the previous publication in a form (or the
// start of the rule) to at against the Interval of the entry.
func (e *Engine) gap(p *part, r *rule, s form, at time.Time, x verdict.Evidence, end string) {
	ref, line := r.since, r.line
	if l, ok := r.last[s]; ok && l.After(ref) {
		ref, line = l, r.lastLine[s]
	}
	iv := time.Duration(r.interval) * time.Millisecond
	reqs := []string{"REQ-RES-PUBLIST-013"}
	if len(r.prec) > 0 {
		reqs = append(reqs, "REQ-RES-PUBLIST-014")
	}
	g := at.Sub(ref)
	if g <= iv+e.opt.Tolerance {
		for _, req := range reqs {
			e.pcheck(p, req)
		}
		return
	}
	what := "the previous publication"
	if _, ok := r.last[s]; !ok {
		what = "the entry or the connect"
	}
	x.Detail = fmt.Sprintf("%s%s: %s after %s (line %d) in the form %s, the Interval is %d ms (+%s tolerance)", end, r.k, g.Round(time.Millisecond), what, line, s, r.interval, e.opt.Tolerance)
	for _, req := range reqs {
		e.pviolation(p, req, x)
	}
}

// answer compares the answer to a Get with the last unsolicited
// publication: a changed content is published on change (Interval 0,
// REQ-RES-PUBLIST-013, REQ-MQTT-019).
func (e *Engine) answer(p *part, m Message, d dsm, r *rule, localized bool) {
	if localized || r.interval > 0 || modeForms[r.mode] == 0 || r.latest == nil || p.pl.uncertain || p.pl.offline || r.k.resource == "Event" {
		return
	}
	if _, significant, _ := deviation(r.latest, d.payload, r.prec); !significant {
		return
	}
	p.pl.expects = append(p.pl.expects, &expect{r: r, payload: d.payload, deadline: m.Received.Add(e.opt.Tolerance),
		x: ev(p, m, d.ptr+"/Payload", ""), prevLine: r.latestLin})
}

// resolve marks the expected publications of a changed content as seen.
func (e *Engine) resolve(p *part, m Message, r *rule, payload map[string]any) {
	keep := p.pl.expects[:0]
	for _, x := range p.pl.expects {
		if _, significant, _ := deviation(x.payload, payload, r.prec); x.r == r && !significant {
			e.pcheck(p, "REQ-RES-PUBLIST-013")
			e.pcheck(p, "REQ-MQTT-019")
			continue
		}
		keep = append(keep, x)
	}
	p.pl.expects = keep
}

// expire reports the expected publications whose time has passed.
func (e *Engine) expire(p *part, now time.Time) {
	keep := p.pl.expects[:0]
	for _, x := range p.pl.expects {
		if now.After(x.deadline) {
			x.x.Detail = fmt.Sprintf("the answer to a Get shows a changed content of %s (last unsolicited publication line %d), but no publication of it followed within %s", x.r.k, x.prevLine, e.opt.Tolerance)
			e.pviolation(p, "REQ-RES-PUBLIST-013", x.x)
			e.pviolation(p, "REQ-MQTT-019", x.x)
			continue
		}
		keep = append(keep, x)
	}
	p.pl.expects = keep
}

// finishPublist judges the Intervals until the end of the observation.
func (e *Engine) finishPublist(p *part, end time.Time) {
	if !p.pl.pubs {
		return
	}
	if len(p.pl.rules) == 0 {
		for _, req := range publistReqs {
			e.skip(p, testPublist, req, true, "no PublicationList of the component observed (test mode requests it with Get PublicationList)")
		}
		return
	}
	e.expire(p, end)
	if p.pl.uncertain || p.pl.offline || (p.state != "connected" && p.state != "unknown") {
		return
	}
	keys := make([]key, 0, len(p.pl.rules))
	for k := range p.pl.rules {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	for _, k := range keys {
		r := p.pl.rules[k]
		forms := modeForms[r.mode]
		if r.interval == 0 || forms == 0 || k.resource == "Event" || end.Before(r.since.Add(e.opt.Tolerance)) {
			continue
		}
		seen := false
		for _, f := range formOrder {
			if _, ok := r.last[f]; ok {
				seen = true
				e.gap(p, r, f, end, verdict.Evidence{Participant: p.ID, Line: r.lastLine[f]}, "until the end of the observation ")
			}
		}
		if !seen {
			e.gap(p, r, slot(forms), end, verdict.Evidence{Participant: p.ID, Line: r.line}, "no unsolicited publication of ")
		}
	}
}
