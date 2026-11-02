package probe

import (
	"context"
	"fmt"
)

// resourceRows are the resources T7-GET-03 requests with and without
// Source: the CMP requirement that makes them mandatory (MG 1.3) and the
// requirement on the answer (MG 9.1).
var resourceRows = []struct{ resource, cmp, answer string }{
	{"Config", "REQ-CMP-035", "REQ-RES-CONFIG-017"},
	{"Data", "REQ-CMP-036", "REQ-RES-DATA-007"},
	{"PublicationList", "REQ-CMP-019", "REQ-RES-PUBLIST-020"},
	{"ReferenceDesignation", "REQ-CMP-020", "REQ-RES-REFDES-004"},
	{"SubscriptionList", "REQ-CMP-039", "REQ-RES-SUBLIST-008"},
}

// filtered are the resources whose Get with Source takes a Filter: the
// WriterGroupName of their DataSetMessages (Config group, Data tag,
// <ResourceType>[/<Tag>] of the lists).
var filtered = map[string]bool{"Config": true, "Data": true, "PublicationList": true, "SubscriptionList": true}

// maxFilters limits the Filters T7-GET-03 requests per resource, maxEntries
// the PublicationList entries it requests.
const (
	maxFilters = 5
	maxEntries = 50
)

func key(d dsm) string { return fmt.Sprintf("%d|%s|%s", d.id, d.group, d.name) }

func contents(ns []*netMsg) []dsm {
	var out []dsm
	for _, n := range ns {
		out = append(out, n.content()...)
	}
	return out
}

// entry is a PublicationList entry (MG 8.3.8).
type entry struct {
	resource, source, filter, mode string
	writer                         int64
}

func entries(ds []dsm) []entry {
	var out []entry
	for _, d := range ds {
		p, _ := d.payload.(map[string]any)
		e := entry{writer: num(p["DataSetWriterId"])}
		e.resource, _ = p["Resource"].(string)
		e.source, _ = p["Source"].(string)
		e.filter, _ = p["Filter"].(string)
		e.mode, _ = p["Mode"].(string)
		if e.resource != "" && e.source != "" {
			out = append(out, e)
		}
	}
	return out
}

// get03 is T7-GET-03: every supported resource answers Get without Source,
// with Source and with Filter; every PublicationList entry of a
// requestable resource with a Mode other than OFF_0 answers Get; the Data
// announced in the PublicationList answers Get Metadata.
func (r *Runner) get03(ctx context.Context) error {
	const t = "T7-GET-03"
	type pair struct{ with, without *exchange }
	xs := map[string]pair{}
	var want, silent []*exchange
	for _, row := range resourceRows {
		w, err := r.get(ctx, t, r.st, row.resource, r.app, "", nil)
		if err != nil {
			return err
		}
		wo, err := r.get(ctx, t, r.st, row.resource, "", "", nil)
		if err != nil {
			return err
		}
		xs[row.resource] = pair{w, wo}
		if r.hasProfile && contains(r.profile, row.resource) {
			want = append(want, w, wo)
		} else {
			silent = append(silent, w, wo)
		}
	}
	if err := r.wait(ctx, want, silent); err != nil {
		return err
	}
	getOK := true // REQ-CMP-008: every Get of a listed resource is answered
	var lists []dsm
	var filters []*exchange
	type fx struct {
		row  int
		want []dsm
	}
	expect := map[*exchange]fx{}
	for i, row := range resourceRows {
		p := xs[row.resource]
		listed := r.hasProfile && contains(r.profile, row.resource)
		with, without := r.reply(t, p.with), r.reply(t, p.without)
		if len(with) == 0 && len(without) == 0 {
			if r.applies(row.cmp) {
				r.violation(t, row.cmp, p.with, "%s is not supported: neither Get %s/<AppId> nor Get %s without Source is answered within %s", row.resource, row.resource, row.resource, r.opt.Timeout)
			}
			if listed {
				r.violation(t, row.answer, p.with, "the Profile lists %s, but Get %s is not answered within %s", row.resource, row.resource, r.opt.Timeout)
				getOK = false
			}
			continue
		}
		r.checked(t, row.cmp, p.with)
		ok := true
		for _, x := range []*exchange{p.with, p.without} {
			ns := map[*exchange][]*netMsg{p.with: with, p.without: without}[x]
			if len(ns) == 0 {
				r.violation(t, row.answer, x, "Get %s (MessageId %s) is not answered within %s, the same Get with%s Source is", row.resource, x.id, r.opt.Timeout, map[bool]string{true: "out", false: ""}[x == p.with])
				ok, getOK = false, false
				continue
			}
			if len(ns) > 1 && !ns[0].paged {
				r.violation(t, row.answer, x, "Get %s is answered with %d NetworkMessages without Pagination instead of one", row.resource, len(ns))
				ok = false
			}
		}
		if len(with) > 0 && len(without) > 0 {
			all := map[string]bool{}
			for _, d := range contents(without) {
				all[key(d)] = true
			}
			for _, d := range contents(with) {
				if !all[key(d)] {
					r.violation(t, row.answer, p.without, "the answer to Get %s without Source lacks the DataSetMessage %q/%q (DataSetWriterId %d) of the answer with Source", row.resource, d.name, d.group, d.id)
					ok = false
				}
			}
		}
		if row.resource == "Config" || row.resource == "Data" || row.resource == "ReferenceDesignation" {
			for _, d := range contents(with) {
				if d.name != r.app {
					r.violation(t, row.answer, p.with, "Get %s/<AppId> is answered with a DataSetMessage of %q", row.resource, d.name)
					ok = false
				}
			}
		}
		if ok {
			r.checked(t, row.answer, p.with)
		}
		if row.resource == "PublicationList" {
			lists = contents(without)
			if len(lists) == 0 {
				lists = contents(with)
			}
		}
		if !filtered[row.resource] {
			continue
		}
		var groups []string
		for _, d := range contents(with) {
			if d.group != "" && !contains(groups, d.group) && len(groups) < maxFilters {
				groups = append(groups, d.group)
			}
		}
		for _, g := range groups {
			x, err := r.get(ctx, t, r.st, row.resource, r.app, g, nil)
			if err != nil {
				return err
			}
			var w []dsm
			for _, d := range contents(with) {
				if d.group == g {
					w = append(w, d)
				}
			}
			filters = append(filters, x)
			expect[x] = fx{i, w}
		}
	}

	// PublicationList entries (REQ-RES-PUBLIST-027) and Metadata of the
	// announced Data (REQ-RES-META-002, REQ-CMP-044: D7 requests Metadata
	// only for Data announced in the PublicationList).
	var announced, metas []*exchange
	requested := map[string]bool{}
	for _, e := range entries(lists) {
		if len(announced) >= maxEntries {
			break
		}
		if e.mode == "OFF_0" {
			continue
		}
		switch {
		case e.resource == "Metadata":
			if e.filter == "" || requested[Topic(r.st, r.app, "Get", "Metadata", e.source, e.filter)] {
				continue
			}
			x, err := r.getMeta(ctx, t, e.source, e.filter, e.writer)
			if err != nil {
				return err
			}
			requested[x.topic] = true
			metas = append(metas, x)
		case contains(Gettable, e.resource):
			topic := Topic(r.st, r.app, "Get", e.resource, e.source, e.filter)
			if requested[topic] {
				continue
			}
			x, err := r.get(ctx, t, r.st, e.resource, e.source, e.filter, nil)
			if err != nil {
				return err
			}
			requested[topic] = true
			announced = append(announced, x)
			if e.resource == "Data" && e.filter != "" {
				mt := Topic(r.st, r.app, "Get", "Metadata", e.source, e.filter)
				if requested[mt] {
					continue
				}
				mx, err := r.getMeta(ctx, t, e.source, e.filter, e.writer)
				if err != nil {
					return err
				}
				requested[mt] = true
				metas = append(metas, mx)
			}
		}
	}
	if err := r.wait(ctx, append(append(append([]*exchange(nil), filters...), announced...), metas...), nil); err != nil {
		return err
	}
	for _, x := range filters {
		f := expect[x]
		row := resourceRows[f.row]
		ns := r.reply(t, x, row.answer)
		if len(ns) == 0 {
			getOK = false
			continue
		}
		got := contents(ns)
		have := map[string]bool{}
		for _, d := range got {
			have[key(d)] = true
		}
		bad := false
		for _, d := range f.want {
			if !have[key(d)] {
				r.violation(t, row.answer, x, "Get %s with the Filter %q lacks the DataSetMessage %q/%q (DataSetWriterId %d)", row.resource, f.want[0].group, d.name, d.group, d.id)
				bad = true
			}
		}
		// The SubscriptionList evaluates wildcards: it may add entries.
		if row.resource != "SubscriptionList" && len(got) != len(f.want) {
			r.violation(t, row.answer, x, "Get %s with the Filter %q is answered with %d DataSetMessages; the answer with Source has %d with this WriterGroupName", row.resource, f.want[0].group, len(got), len(f.want))
			bad = true
		}
		if !bad {
			r.checked(t, row.answer, x)
		}
	}
	if lists == nil {
		r.skip(t, "REQ-RES-PUBLIST-027", "no PublicationList entries to request")
		r.skip(t, "REQ-RES-META-002", "no PublicationList: D7 requests Metadata only for Data announced in the PublicationList (REQ-CMP-044)")
		r.skip(t, "REQ-CMP-037", "no PublicationList: D7 requests Metadata only for Data announced in the PublicationList (REQ-CMP-044)")
	}
	for _, x := range announced {
		if len(r.reply(t, x)) > 0 {
			r.checked(t, "REQ-RES-PUBLIST-027", x)
		} else {
			r.violation(t, "REQ-RES-PUBLIST-027", x, "the PublicationList announces %s with a Mode other than OFF_0, but Get %s (MessageId %s) is not answered within %s", x.topic, x.topic, x.id, r.opt.Timeout)
		}
	}
	// An unanswered Get Metadata violates REQ-RES-META-002 only if the
	// application claims Metadata (Profile or level); else Metadata is
	// judged as not supported (REQ-CMP-037).
	claims := (r.hasProfile && contains(r.profile, "Metadata")) || r.mandatory("REQ-CMP-037")
	metaOK, metaBad := 0, 0
	for _, x := range metas {
		ns := r.reply(t, x)
		if len(ns) == 0 {
			if claims {
				r.violation(t, "REQ-RES-META-002", x, "Get Metadata/%s (MessageId %s) of Data announced in the PublicationList is not answered within %s", x.topic[len(Topic(r.st, r.app, "Get", "Metadata", "", "")):], x.id, r.opt.Timeout)
				metaBad++
			}
			continue
		}
		a := ns[0]
		md, _ := a.msgs[0].payload.(map[string]any)
		tag := x.topic[len(Topic(r.st, r.app, "Get", "Metadata", x.source, ""))+1:]
		switch {
		case a.msgType != "ua-metadata":
			r.violation(t, "REQ-RES-META-002", x, "Get Metadata is answered with a %s message instead of ua-metadata", a.msgType)
		case len(md) == 0:
			r.violation(t, "REQ-RES-META-002", x, "Get Metadata is answered without filled DataSetMetaData")
		case a.msgs[0].name != x.source || a.msgs[0].group != tag:
			r.violation(t, "REQ-RES-META-002", x, "the answer to Get Metadata/%s/%s carries DataSetWriterName %q and WriterGroupName %q", x.source, tag, a.msgs[0].name, a.msgs[0].group)
		default:
			r.checked(t, "REQ-RES-META-002", x)
			metaOK++
			continue
		}
		metaBad++
	}
	if lists != nil {
		switch {
		case metaOK > 0:
			r.checked(t, "REQ-CMP-037", nil)
		case len(metas) > 0:
			if metaBad == 0 {
				r.skip(t, "REQ-RES-META-002", "Metadata is not supported (the Profile does not list it)")
			}
			if r.applies("REQ-CMP-037") {
				r.violation(t, "REQ-CMP-037", metas[0], "Metadata is not supported: no Get Metadata of the Data announced in the PublicationList is answered")
			}
			if r.hasProfile && contains(r.profile, "Metadata") {
				getOK = false
			}
		default:
			r.skip(t, "REQ-CMP-037", "the PublicationList announces no Data (D7 requests Metadata only for announced Data, REQ-CMP-044)")
			r.skip(t, "REQ-RES-META-002", "the PublicationList announces no Data")
		}
		if len(announced) == 0 {
			r.skip(t, "REQ-RES-PUBLIST-027", "the PublicationList has no entry of a requestable resource with a Mode other than OFF_0")
		}
	}
	if r.applies("REQ-CMP-008") {
		if getOK {
			r.checked(t, "REQ-CMP-008", nil)
		} else {
			r.violation(t, "REQ-CMP-008", nil, "a Get of a resource the Profile lists is not answered (the Set and Del patterns are judged by T7-SET-*)")
		}
	}
	return nil
}

// entityResources returns the resources T7-GET-04 requests with Source:
// required (MUST at the level) and recommended (SHOULD) ones.
func (r *Runner) entityResources() (required, recommended []string) {
	if r.mandatory("REQ-CMP-017") {
		required = append(required, "MAM", "Health")
	} else if r.applies("REQ-CMP-017") {
		recommended = append(recommended, "MAM", "Health")
	}
	for _, lr := range levelResources {
		if !contains(Gettable, lr.resource) {
			continue
		}
		switch {
		case r.mandatory(lr.req):
			required = append(required, lr.resource)
		case r.applies(lr.req):
			recommended = append(recommended, lr.resource)
		}
	}
	return required, recommended
}

// appOnly are provided once by the representing application (REQ-CMP-045).
var appOnly = map[string]bool{"PublicationList": true, "SubscriptionList": true}

// get04 is T7-GET-04: the application itself (REQ-CMP-024) and every entity
// it represents (REQ-CMP-045) answer Get with their Source for every
// resource of the level; at level 2 the recommended resources are
// REQ-CMP-010. A Registry represents no entity: the other Sources of its
// answer to Get MAM are listed assets (MG A1, ADR-0025 amendment C-087).
func (r *Runner) get04(ctx context.Context) error {
	const t = "T7-GET-04"
	required, recommended := r.entityResources()
	all, err := r.get(ctx, t, r.st, "MAM", "", "", nil)
	if err != nil {
		return err
	}
	if err := r.wait(ctx, []*exchange{all}, nil); err != nil {
		return err
	}
	srcs := []string{r.app}
	for _, s := range sources(r.reply(t, all)) {
		if s != "" && !contains(srcs, s) && len(srcs) < maxSources && !r.registry() {
			srcs = append(srcs, s)
		}
	}
	type ask struct {
		source, resource string
		required         bool
		x                *exchange
	}
	var asks []ask
	var xs []*exchange
	for _, s := range srcs {
		for i, list := range [][]string{required, recommended} {
			for _, res := range list {
				if s != r.app && appOnly[res] {
					continue
				}
				x, err := r.get(ctx, t, r.st, res, s, "", nil)
				if err != nil {
					return err
				}
				asks = append(asks, ask{s, res, i == 0, x})
				xs = append(xs, x)
			}
		}
	}
	if len(asks) == 0 {
		r.skip(t, "REQ-CMP-024", "level %d requires no resource on request (Get)", r.opt.Level)
		r.skip(t, "REQ-CMP-045", "level %d requires no resource on request (Get)", r.opt.Level)
		return nil
	}
	if err := r.wait(ctx, xs, nil); err != nil {
		return err
	}
	bad := map[string]bool{}
	for _, a := range asks {
		if len(r.reply(t, a.x)) > 0 {
			continue
		}
		who, req := "the application itself", "REQ-CMP-024"
		if a.source != r.app {
			who, req = fmt.Sprintf("the represented entity %q", a.source), "REQ-CMP-045"
		}
		if a.required {
			r.violation(t, req, a.x, "%s does not answer Get %s with its Source within %s; level %d requires %s", who, a.resource, r.opt.Timeout, r.opt.Level, a.resource)
			bad[req] = true
		}
		if r.applies("REQ-CMP-010") {
			r.violation(t, "REQ-CMP-010", a.x, "%s does not answer Get %s with its Source within %s", who, a.resource, r.opt.Timeout)
			bad["REQ-CMP-010"] = true
		}
	}
	if !bad["REQ-CMP-024"] {
		r.checked(t, "REQ-CMP-024", nil)
	}
	if r.registry() {
		r.skip(t, "REQ-CMP-045", "the application is a Registry: the other Sources of its answer to Get MAM are listed assets (MG A1), not represented entities")
	} else if len(srcs) == 1 {
		r.skip(t, "REQ-CMP-045", "the application represents no entity (the answer to Get MAM without Source has only its own MAM)")
	} else if !bad["REQ-CMP-045"] {
		r.checked(t, "REQ-CMP-045", nil)
	}
	if r.applies("REQ-CMP-010") && !bad["REQ-CMP-010"] {
		r.checked(t, "REQ-CMP-010", nil)
	}
	return nil
}
