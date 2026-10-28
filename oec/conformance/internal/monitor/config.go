package monitor

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/OI4/oec/conformance/internal/topic"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// This file holds the rules on the structure and the configuration of a
// component: T7-MON-09 (DataSetClassId), T7-MON-13 (Data and Event),
// T7-MON-15 (republication of the lists), T7-MON-16 (RevisionCounter) and
// T7-MON-17 (mandatory Config objects in a Set).

// classIDs is T7-MON-09: a publisher-defined DataSetClassId identifies one
// structure (the field names of the DataSet), also across reconnects, and
// a DataSet with an unchanged structure keeps its DataSetClassId
// (REQ-MSG-013). Publisher-defined are the DataSetClassIds of Data and of
// the DataSetMetaData; the other resources use the predefined ones.
func (e *Engine) classIDs(p *part, m Message, t *topic.Parsed, msg map[string]any, ds []dsm) {
	switch t.Resource {
	case "Metadata":
		md, _ := msg["MetaData"].(map[string]any)
		class := str(md, "DataSetClassId")
		fields, ok := md["Fields"].([]any)
		if class == "" || !ok {
			return
		}
		var names []string
		for _, f := range fields {
			fm, _ := f.(map[string]any)
			names = append(names, str(fm, "Name"))
		}
		e.classID(p, m, "/MetaData/DataSetClassId", class, names, key{"Data", str(msg, "DataSetWriterName"), str(msg, "WriterGroupName")})
	case "Data":
		class := str(msg, "DataSetClassId")
		if class == "" {
			return
		}
		for _, d := range e.resourceDSMs(ds) {
			if d.payload == nil {
				continue
			}
			names := make([]string, 0, len(d.payload))
			for n := range d.payload {
				names = append(names, n)
			}
			e.classID(p, m, d.ptr+"/Payload", class, names, key{"Data", d.name, d.group})
		}
	}
}

func (e *Engine) classID(p *part, m Message, ptr, class string, names []string, k key) {
	const test, req = "T7-MON-09", "REQ-MSG-013"
	sort.Strings(names)
	structure := strings.Join(names, ", ")
	e.check(p, test, req)
	if c, ok := p.classes[class]; !ok {
		p.classes[class] = classRef{class: class, structure: structure, line: m.Line}
	} else if c.structure != structure {
		e.add(test, req, verdict.Violation, ev(p, m, ptr, "DataSetClassId %s identifies the fields [%s] (line %d), here the fields [%s]: a changed structure needs a new DataSetClassId", class, c.structure, c.line, structure))
	}
	if c, ok := p.dataSets[k]; ok && c.structure == structure && c.class != class {
		e.add(test, req, verdict.Violation, ev(p, m, ptr, "the DataSet %s keeps its fields [%s] but changes its DataSetClassId from %s (line %d) to %s", k, structure, c.class, c.line, class))
	}
	p.dataSets[k] = classRef{class: class, structure: structure, line: m.Line}
}

// supported notes for T7-MON-13 a Pub Data, a Pub Event and the Resources
// of the Profile of the component about itself.
func (e *Engine) supported(p *part, t *topic.Parsed, ds []dsm) {
	rs := e.resourceDSMs(ds)
	switch t.Resource {
	case "Data":
		p.data = p.data || len(rs) > 0
	case "Event":
		p.event = p.event || len(rs) > 0
	case "Profile":
		for _, d := range rs {
			arr, ok := d.payload["Resources"].([]any)
			if d.name != p.AppID || !ok {
				continue
			}
			p.profile = p.profile[:0]
			for _, r := range arr {
				if s, ok := r.(string); ok {
					p.profile = append(p.profile, s)
				}
			}
			p.profileSeen = true
		}
	}
}

// finishSupported is T7-MON-13: the component supports Data (REQ-CMP-036)
// and Event (REQ-CMP-038), which are part of the complete Message Bus
// behavior (REQ-CMP-010). A resource is supported when the component
// publishes it or lists it in its Profile (REQ-CMP-021; also without
// content, REQ-CMP-041 … 043). Judged for components that publish about a
// Source; a pure requester is not judged. Without a publication and a
// Profile the support is not observable (skip, not inconclusive: absence
// on the bus is no evidence, the probe requests the Profile).
func (e *Engine) finishSupported(p *part) {
	const test = "T7-MON-13"
	if len(p.sources) == 0 {
		return
	}
	listed := func(r string) bool {
		for _, x := range p.profile {
			if x == r {
				return true
			}
		}
		return false
	}
	var missing []string
	for _, r := range []struct {
		res, req  string
		published bool
	}{{"Data", "REQ-CMP-036", p.data}, {"Event", "REQ-CMP-038", p.event}} {
		switch {
		case r.published || listed(r.res):
			e.check(p, test, r.req)
		case p.profileSeen:
			missing = append(missing, r.res)
			e.add(test, r.req, verdict.Violation, verdict.Evidence{Participant: p.ID, Detail: fmt.Sprintf("the Profile of the component about itself does not list %s and no Pub %s was observed", r.res, r.res)})
		default:
			e.skip(p, test, r.req, false, "not observable: no Pub %s and no Profile of the component about itself (test mode requests the Profile with Get)", r.res)
		}
	}
	switch {
	case len(missing) > 0:
		e.add(test, "REQ-CMP-010", verdict.Violation, verdict.Evidence{Participant: p.ID, Detail: fmt.Sprintf("the component does not support %s", strings.Join(missing, " and "))})
	case p.profileSeen || p.data && p.event:
		e.check(p, test, "REQ-CMP-010")
	default:
		e.skip(p, test, "REQ-CMP-010", false, "not observable: no Profile of the component about itself (test mode requests the Profile with Get)")
	}
}

// change notes a Set or Del addressed to the component q: for
// T7-MON-15 a change of its PublicationList or SubscriptionList, for
// T7-MON-17 the mandatory objects of a Set Config sent by p, for
// T7-MON-16 a Set Config that changes a value q published.
func (e *Engine) change(p, q *part, m Message, t *topic.Parsed, msg map[string]any) {
	id := str(msg, "MessageId")
	if id == "" {
		return
	}
	switch t.Resource {
	case "PublicationList", "SubscriptionList":
		q.changes = append(q.changes, &change{id: id, method: t.Method, resource: t.Resource, line: m.Line})
	case "Config":
		if t.Method != "Set" {
			return
		}
		for _, d := range e.resourceDSMs(dsms(msg)) {
			src := d.name
			if src == "" {
				src = t.SourceIL
			}
			pub := q.configs[src+"|"+d.group]
			e.mandatory(p, q, m, d, src, pub)
			if pub != nil && e.modifies(d, pub) {
				if r, ok := q.revs[src]; ok && r.rev != -1 {
					q.changes = append(q.changes, &change{id: id, method: t.Method, resource: t.Resource, line: m.Line, source: src, prevRev: r.rev})
					return
				}
			}
		}
	}
}

// mandatory is T7-MON-17: a Set Config contains, for every group it
// contains, each object the addressed component published with
// Mandatory = true, with its Value (REQ-RES-CONFIG-011). The Mandatory
// flags are known only from the last Pub Config of the same Source and
// WriterGroupName.
func (e *Engine) mandatory(p, q *part, m Message, d dsm, src string, pub *configRef) {
	const test, req = "T7-MON-17", "REQ-RES-CONFIG-011"
	if pub == nil {
		e.skip(p, test, req, true, "no Pub Config of %s for the Source %q and the WriterGroupName %q observed before the Set: the Mandatory flags are unknown", q.ID, src, d.group)
		return
	}
	e.check(p, test, req)
	for _, g := range sortedKeys(d.payload) {
		set, ok := d.payload[g].(map[string]any)
		group := pub.groups[g]
		if !ok || group == nil {
			continue
		}
		for _, o := range sortedKeys(group) {
			obj, _ := group[o].(map[string]any)
			if obj["Mandatory"] != true {
				continue
			}
			so, _ := set[o].(map[string]any)
			if _, ok := so["Value"]; !ok {
				e.add(test, req, verdict.Violation, ev(p, m, d.ptr+"/Payload/"+pointer(g), "the Set contains the group %q without the mandatory object %q and its Value (Pub Config of %s, line %d)", g, o, q.ID, pub.line))
			}
		}
	}
}

// modifies reports whether a Set Config gives a known, non-sensitive
// object a Value other than the published one (T7-MON-16). Sensitive
// values are masked in the Pub and therefore unknown.
func (e *Engine) modifies(d dsm, pub *configRef) bool {
	for g, gv := range d.payload {
		set, _ := gv.(map[string]any)
		for o, ov := range set {
			so, _ := ov.(map[string]any)
			po, _ := pub.groups[g][o].(map[string]any)
			v, ok := so["Value"]
			if po == nil || !ok || po["Sensitive"] == true {
				continue
			}
			if !reflect.DeepEqual(v, po["Value"]) {
				return true
			}
		}
	}
	return false
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func pointer(s string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(s)
}

// changed follows the publications of a component after a change: the
// status event answering a Set or Del, the republication of a list
// (T7-MON-15), its Config (T7-MON-16/17) and the RevisionCounter of its
// MAM (T7-MON-16).
func (e *Engine) changed(p *part, m Message, t *topic.Parsed, msg map[string]any, ds []dsm) {
	replyTo := str(msg, "ReplyTo")
	switch t.Resource {
	case "Event":
		if !strings.HasPrefix(t.Filter, "Status/") || replyTo == "" {
			return
		}
		for _, c := range p.changes {
			if c.id == replyTo && c.status == "" {
				c.status = t.Filter
				c.goodAt = m.Received
			}
		}
	case "PublicationList", "SubscriptionList":
		may := map[string]string{"PublicationList": "REQ-RES-PUBLIST-018", "SubscriptionList": "REQ-RES-SUBLIST-006"}[t.Resource]
		for _, c := range p.changes {
			if c.resource != t.Resource || c.republished {
				continue
			}
			switch replyTo {
			case c.id:
				c.republished = true
				e.check(p, "T7-MON-15", may)
			case "":
				c.republished = true
			}
		}
	case "Config":
		for _, d := range e.resourceDSMs(ds) {
			k := d.name + "|" + d.group
			c := p.configs[k]
			if c == nil {
				c = &configRef{groups: map[string]map[string]any{}}
				p.configs[k] = c
			}
			c.line = m.Line
			for g, gv := range d.payload {
				group, ok := gv.(map[string]any)
				if !ok {
					continue
				}
				if c.groups[g] == nil {
					c.groups[g] = map[string]any{}
				}
				for o, ov := range group {
					c.groups[g][o] = ov
				}
			}
		}
	case "MAM":
		for _, d := range e.resourceDSMs(ds) {
			e.revision(p, m, d)
		}
	}
}

// revision is T7-MON-16: the RevisionCounter of an asset does not
// decrease, does not change between -1 (no revision counter) and a
// counter, and grows after a successful Set Config that changed a value of
// the asset (REQ-RES-MAM-018).
func (e *Engine) revision(p *part, m Message, d dsm) {
	const test, req = "T7-MON-16", "REQ-RES-MAM-018"
	rev, ok := num(d.payload, "RevisionCounter")
	if !ok || d.name == "" {
		return
	}
	e.check(p, test, req)
	at := d.ptr + "/Payload/RevisionCounter"
	if r, ok := p.revs[d.name]; ok {
		switch {
		case (r.rev == -1) != (rev == -1):
			e.add(test, req, verdict.Violation, ev(p, m, at, "the RevisionCounter of %q changes from %d (line %d) to %d: an asset without revision counter uses -1", d.name, r.rev, r.line, rev))
		case rev < r.rev:
			e.add(test, req, verdict.Violation, ev(p, m, at, "the RevisionCounter of %q decreases from %d (line %d) to %d", d.name, r.rev, r.line, rev))
		}
	}
	p.revs[d.name] = revRef{rev: rev, line: m.Line}
	for _, c := range p.changes {
		if c.resource != "Config" || c.resolved || c.source != d.name {
			continue
		}
		switch {
		case rev > c.prevRev:
			c.resolved = true
		case c.good() && m.Received.Sub(c.goodAt) > e.opt.Tolerance:
			c.resolved = true
			e.add(test, req, verdict.Violation, ev(p, m, at, "the successful Set Config of line %d changed a value of %q, the MAM still carries the RevisionCounter %d", c.line, d.name, rev))
		}
	}
}

// finishChanges is the end of T7-MON-15 and T7-MON-16 for the changes
// the component confirmed with Status/Good.
func (e *Engine) finishChanges(p *part, end time.Time) {
	const test = "T7-MON-15"
	for _, c := range p.changes {
		if !c.good() {
			continue
		}
		if c.resource == "Config" {
			if !c.resolved {
				e.skip(p, "T7-MON-16", "REQ-RES-MAM-018", true, "no Pub MAM of %q observed after the successful Set Config of line %d (the RevisionCounter is visible in the MAM)", c.source, c.line)
			}
			continue
		}
		req := map[string]string{"PublicationList": "REQ-RES-PUBLIST-017", "SubscriptionList": "REQ-RES-SUBLIST-005"}[c.resource]
		switch {
		case c.republished:
			e.check(p, test, req)
		case c.method == "Del":
			e.skip(p, test, req, false, "no publication after the successful Del SubscriptionList of line %d: without remaining entries nothing is published (REQ-RES-SUBLIST-015), whether entries remain is not observable", c.line)
		case end.Sub(c.goodAt) > e.opt.Tolerance:
			e.add(test, req, verdict.Violation, verdict.Evidence{Participant: p.ID, Line: c.line, Detail: fmt.Sprintf("the %s %s of line %d was successful, the component did not publish its %s again (within %s)", c.method, c.resource, c.line, c.resource, e.opt.Tolerance)})
		default:
			e.skip(p, test, req, true, "observation ended within the tolerance after the successful %s %s of line %d", c.method, c.resource, c.line)
		}
	}
}
