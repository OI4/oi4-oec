package probe

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// series reads the NetworkMessages of one answer (MG 8.3.11): a single
// NetworkMessage without Pagination, or pages 1 … k with HasNext false on
// the last one. It returns the resource DataSetMessages and, if the
// answer is no complete series, the reason.
func series(ns []*netMsg) ([]dsm, string) {
	if len(ns) == 0 {
		return nil, "no answer"
	}
	if len(ns) == 1 && ns[0].pag == nil {
		return ns[0].content(), ""
	}
	pages := append([]*netMsg(nil), ns...)
	for _, n := range pages {
		if n.pag == nil {
			return nil, fmt.Sprintf("%d NetworkMessages, not every one with a Pagination", len(ns))
		}
	}
	sort.SliceStable(pages, func(i, j int) bool { return pages[i].pag.page < pages[j].pag.page })
	var out []dsm
	for i, n := range pages {
		if n.pag.page != int64(i+1) {
			return nil, fmt.Sprintf("the pages are not numbered 1 … %d (page %d at position %d)", len(pages), n.pag.page, i+1)
		}
		out = append(out, n.content()...)
	}
	last := pages[len(pages)-1].pag
	if last.hasNext {
		return nil, fmt.Sprintf("page %d has HasNext true, but no further page follows within the timeout", last.page)
	}
	if int64(len(out)) != last.total {
		return nil, fmt.Sprintf("the pages carry %d resource DataSetMessages, TotalCount is %d", len(out), last.total)
	}
	return out, ""
}

// complete reports whether the answer of x is a complete series: without
// Pagination or with a page that has HasNext false.
func (r *Runner) complete(x *exchange) bool {
	answers, _, _ := r.snapshot(x)
	for _, a := range answers {
		if a.pag == nil || !a.pag.hasNext {
			return true
		}
	}
	return false
}

func (r *Runner) completeAll(xs ...*exchange) func() bool {
	return func() bool {
		for _, x := range xs {
			if !r.complete(x) {
				return false
			}
		}
		return true
	}
}

func names(ds []dsm) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.name)
	}
	sort.Strings(out)
	return out
}

func same(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (r *Runner) paginationRequest(perPage, page int) map[string]any {
	return r.helper(1, "PaginationRequest", map[string]any{"PerPage": perPage, "Page": page})
}

// get06 is T7-GET-06: Get MAM without Source and without PaginationRequest
// is answered with the complete result (REQ-PAG-005); with a
// PaginationRequest of PerPage 1 for page 1, a page beyond the end and
// page 0 (all pages) it is answered page by page (REQ-PAG-002). Pagination
// by size (MaxPacketSize) cannot be stimulated; T7-MON-18 checks the size
// of every NetworkMessage.
func (r *Runner) get06(ctx context.Context) error {
	const t = "T7-GET-06"
	full, err := r.get(ctx, t, r.st, "MAM", "", "", nil)
	if err != nil {
		return err
	}
	if err := r.until(ctx, r.completeAll(full)); err != nil {
		return err
	}
	ns := r.reply(t, full)
	if len(ns) == 0 {
		why := "Get MAM without Source is not answered (T7-GET-01)"
		r.skip(t, "REQ-PAG-005", "%s", why)
		r.skip(t, "REQ-PAG-002", "%s", why)
		return nil
	}
	all, why := series(ns)
	if why != "" {
		r.violation(t, "REQ-PAG-005", full, "the answer to Get MAM without Source is incomplete: %s", why)
		r.skip(t, "REQ-PAG-002", "the complete result is unknown")
		return nil
	}
	r.checked(t, "REQ-PAG-005", full)
	n := len(all)
	p1, err := r.getWith(ctx, t, "MAM", "", "", r.paginationRequest(1, 1))
	if err != nil {
		return err
	}
	beyond, err := r.getWith(ctx, t, "MAM", "", "", r.paginationRequest(1, n+1))
	if err != nil {
		return err
	}
	p0, err := r.getWith(ctx, t, "MAM", "", "", r.paginationRequest(1, 0))
	if err != nil {
		return err
	}
	if err := r.until(ctx, r.completeAll(p1, beyond, p0)); err != nil {
		return err
	}
	ok := true
	bad := func(x *exchange, format string, a ...any) {
		r.violation(t, "REQ-PAG-002", x, format, a...)
		ok = false
	}
	single := func(x *exchange, page, want int) {
		ns := r.reply(t, x)
		switch {
		case len(ns) == 0:
			bad(x, "Get MAM with the PaginationRequest PerPage 1, Page %d (MessageId %s) is not answered within %s", page, x.id, r.opt.Timeout)
		case len(ns) > 1:
			bad(x, "Get MAM with the PaginationRequest PerPage 1, Page %d is answered with %d NetworkMessages; with Page > 0 only the requested page is published", page, len(ns))
		case ns[0].pag == nil:
			bad(x, "the answer to Get MAM with the PaginationRequest PerPage 1, Page %d has no Pagination", page)
			if len(ns[0].content()) > 1 {
				r.violation(t, "REQ-PAG-005", x, "the answer to Get MAM with the PaginationRequest PerPage 1 carries %d resource DataSetMessages in one NetworkMessage", len(ns[0].content()))
			}
		case len(ns[0].content()) != want:
			bad(x, "page %d of PerPage 1 carries %d resource DataSetMessages instead of %d", page, len(ns[0].content()), want)
		case ns[0].pag.page != int64(page):
			bad(x, "the answer to the request of page %d has Page %d", page, ns[0].pag.page)
		case page > n && ns[0].pag.hasNext:
			bad(x, "the page beyond the last page (%d of %d) has HasNext true", page, n)
		}
	}
	single(p1, 1, 1)
	single(beyond, n+1, 0)
	if ns := r.reply(t, p0); len(ns) == 0 {
		bad(p0, "Get MAM with the PaginationRequest PerPage 1, Page 0 (MessageId %s) is not answered within %s", p0.id, r.opt.Timeout)
	} else if got, why := series(ns); why != "" {
		bad(p0, "the answer to Get MAM with the PaginationRequest PerPage 1, Page 0 is no complete series of pages: %s", why)
	} else {
		for _, a := range ns {
			c := len(a.content())
			switch {
			case c > 1 && a.pag == nil:
				bad(p0, "the answer to Get MAM with the PaginationRequest PerPage 1, Page 0 has no Pagination and carries %d resource DataSetMessages", c)
				r.violation(t, "REQ-PAG-005", p0, "the answer to Get MAM with the PaginationRequest PerPage 1 carries %d resource DataSetMessages in one NetworkMessage", c)
			case c > 1:
				bad(p0, "page %d of PerPage 1 carries %d resource DataSetMessages", a.pag.page, c)
				r.violation(t, "REQ-PAG-005", p0, "page %d of PerPage 1 carries %d resource DataSetMessages", a.pag.page, c)
			}
		}
		if len(ns) != len(got) {
			bad(p0, "with Page 0 and PerPage 1 the %d resource DataSetMessages are published in %d NetworkMessages", len(got), len(ns))
		}
	}
	if ok {
		r.checked(t, "REQ-PAG-002", p1)
	}
	return nil
}

// Aspects of ReferenceDesignation (MG 9.1.10).
var aspects = []string{"Function", "Product", "Location"}

// children maps the IdentificationLink of every Parent to the assets
// whose ReferenceDesignation names it (in any aspect).
func children(ds []dsm) map[string][]string {
	out := map[string][]string{}
	for _, d := range ds {
		p, _ := d.payload.(map[string]any)
		for _, a := range aspects {
			o, _ := p[a].(map[string]any)
			parent, _ := o["Parent"].(map[string]any)
			il, _ := parent["IdentificationLink"].(string)
			if il != "" && !contains(out[il], d.name) {
				out[il] = append(out[il], d.name)
			}
		}
	}
	for il := range out {
		sort.Strings(out[il])
	}
	return out
}

func (r *Runner) query(parent string, extra map[string]any) map[string]any {
	p := map[string]any{"Parent": parent}
	for k, v := range extra {
		p[k] = v
	}
	return r.helper(9, "Query", p)
}

var get07Reqs = []string{"REQ-QRY-001", "REQ-QRY-002", "REQ-QRY-003", "REQ-RES-REFDES-011", "REQ-RES-REFDES-013", "REQ-RES-REFDES-014"}

// get07 is T7-GET-07: the Query of ReferenceDesignation (MG 8.3.13.1)
// returns the direct children of the Parent or – if not supported – is
// ignored; undefined query parameters, a Query with Source and a Query on
// another resource are ignored.
func (r *Runner) get07(ctx context.Context) error {
	const t = "T7-GET-07"
	if !r.hasProfile || !contains(r.profile, "ReferenceDesignation") {
		for _, req := range get07Reqs {
			r.skip(t, req, "the Profile does not list ReferenceDesignation")
		}
		return nil
	}
	fullX, err := r.get(ctx, t, r.st, "ReferenceDesignation", "", "", nil)
	if err != nil {
		return err
	}
	if err := r.until(ctx, r.completeAll(fullX)); err != nil {
		return err
	}
	full, why := series(r.reply(t, fullX))
	if why != "" {
		for _, req := range get07Reqs {
			r.skip(t, req, "the answer to Get ReferenceDesignation without Source is not usable (%s; T7-GET-03)", why)
		}
		return nil
	}
	kids := children(full)
	var parents []string
	for il := range kids {
		parents = append(parents, il)
	}
	sort.Strings(parents)
	parent := ""
	for _, il := range parents {
		if len(kids[il]) < len(full) {
			parent = il
			break
		}
	}
	if parent == "" && len(parents) > 0 {
		parent = parents[0]
	}
	if parent == "" {
		parent = "oi4.org/oec-check/D7/parent-" + random()
	}
	expected, all := kids[parent], names(full)
	q1, err := r.getWith(ctx, t, "ReferenceDesignation", "", "", r.query(parent, nil))
	if err != nil {
		return err
	}
	q2, err := r.getWith(ctx, t, "ReferenceDesignation", "", "", r.query(parent, map[string]any{UnknownKey: "D7 checks that undefined query parameters are ignored"}))
	if err != nil {
		return err
	}
	q3, err := r.getWith(ctx, t, "ReferenceDesignation", r.app, "", r.query("oi4.org/oec-check/D7/parent-"+random(), nil))
	if err != nil {
		return err
	}
	own, err := r.get(ctx, t, r.st, "ReferenceDesignation", r.app, "", nil)
	if err != nil {
		return err
	}
	mam, err := r.get(ctx, t, r.st, "MAM", "", "", nil)
	if err != nil {
		return err
	}
	mamQ, err := r.getWith(ctx, t, "MAM", "", "", r.query(parent, nil))
	if err != nil {
		return err
	}
	if err := r.until(ctx, r.completeAll(q1, q2, q3, own, mam, mamQ)); err != nil {
		return err
	}
	read := func(x *exchange) ([]string, bool) {
		ds, why := series(r.reply(t, x))
		if why != "" {
			return nil, false
		}
		return names(ds), true
	}
	got1, ok1 := read(q1)
	supported := false
	switch {
	case len(full) == 0:
		for _, req := range []string{"REQ-RES-REFDES-011", "REQ-RES-REFDES-014", "REQ-QRY-002", "REQ-QRY-003"} {
			r.skip(t, req, "the application has no object of ReferenceDesignation: the answer to a Query cannot be judged")
		}
	case !ok1:
		r.violation(t, "REQ-RES-REFDES-014", q1, "Get ReferenceDesignation with the Query Parent %q (MessageId %s) is not answered completely within %s", parent, q1.id, r.opt.Timeout)
		r.violation(t, "REQ-QRY-003", q1, "Get ReferenceDesignation with a Query is not answered (completely), the same Get without Query is")
		r.skip(t, "REQ-RES-REFDES-011", "the Get with Query is not answered")
		r.skip(t, "REQ-QRY-002", "the Get with Query is not answered")
	case same(got1, expected):
		supported = true
		r.checked(t, "REQ-RES-REFDES-011", q1)
		r.checked(t, "REQ-RES-REFDES-014", q1)
		if same(expected, all) {
			r.skip(t, "REQ-QRY-003", "every asset is a child of %q: an ignored Query cannot be told apart", parent)
		} else {
			r.skip(t, "REQ-QRY-003", "the application supports the Query of ReferenceDesignation")
		}
	case same(got1, all):
		r.checked(t, "REQ-QRY-003", q1)
		if r.mandatory("REQ-RES-REFDES-011") {
			r.violation(t, "REQ-RES-REFDES-011", q1, "the Query Parent %q is ignored (answer: all %d objects instead of the children %q); level %d requires the Query", parent, len(all), expected, r.opt.Level)
		} else {
			r.skip(t, "REQ-RES-REFDES-011", "the application does not support the Query of ReferenceDesignation (optional at level %d)", r.opt.Level)
		}
		r.skip(t, "REQ-RES-REFDES-014", "the application does not support the Query of ReferenceDesignation")
		r.skip(t, "REQ-QRY-002", "the application does not support the Query of ReferenceDesignation")
	default:
		r.violation(t, "REQ-RES-REFDES-014", q1, "the Query Parent %q is answered with %q; the direct children are %q", parent, got1, expected)
		r.violation(t, "REQ-QRY-003", q1, "the Query Parent %q is answered with %q, neither the children %q nor every object (Query ignored)", parent, got1, expected)
		r.skip(t, "REQ-RES-REFDES-011", "the answer to the Query is wrong (REQ-RES-REFDES-014)")
	}
	if supported {
		if got2, ok := read(q2); ok && same(got2, got1) {
			r.checked(t, "REQ-QRY-002", q2)
		} else {
			r.violation(t, "REQ-QRY-002", q2, "the Query Parent %q with the undefined parameter %s is answered with %q instead of %q", parent, UnknownKey, got2, got1)
		}
	}
	got3, ok3 := read(q3)
	switch gotO, okO := read(own); {
	case !okO:
		r.skip(t, "REQ-RES-REFDES-013", "Get ReferenceDesignation/<AppId> without Query is not answered (T7-GET-03)")
	case ok3 && same(got3, gotO):
		r.checked(t, "REQ-RES-REFDES-013", q3)
	default:
		r.violation(t, "REQ-RES-REFDES-013", q3, "Get ReferenceDesignation/<AppId> with a Query (MessageId %s) is answered with %q, the same Get without Query with %q (the Query must be ignored)", q3.id, got3, gotO)
	}
	gotM, okM := read(mam)
	switch gotQ, okQ := read(mamQ); {
	case !okM:
		r.skip(t, "REQ-QRY-001", "Get MAM without Source is not answered (T7-GET-01)")
	case okQ && same(gotQ, gotM):
		r.checked(t, "REQ-QRY-001", mamQ)
	default:
		r.violation(t, "REQ-QRY-001", mamQ, "Get MAM without Source with a Query (MessageId %s) is answered with %q instead of %q (a Query on MAM must be ignored)", mamQ.id, gotQ, gotM)
	}
	return nil
}

// Locales D7 accepts in an answer to a Get with the Locale de-DE: the
// requested one, the default language and the invariant locale.
var locales = map[string]bool{"de-DE": true, "en-US": true, "": true}

// localized collects the Locale of every LocalizedText (an object with the
// String fields Locale and Text) in v.
func localized(v any, path string, out map[string]string) {
	switch o := v.(type) {
	case map[string]any:
		l, okL := o["Locale"].(string)
		_, okT := o["Text"].(string)
		if okL && okT {
			out[path] = l
			return
		}
		for k, x := range o {
			localized(x, path+"/"+k, out)
		}
	case []any:
		for i, x := range o {
			localized(x, fmt.Sprintf("%s/%d", path, i), out)
		}
	}
}

// get08 is T7-GET-08: a Get with the Locale de-DE is answered with every
// LocalizedText in de-DE or in the default language en-US (REQ-LOC-002).
// Whether the application supports German cannot be observed; both are
// accepted.
func (r *Runner) get08(ctx context.Context) error {
	const t = "T7-GET-08"
	locale := r.helper(3, "Locale", map[string]any{"Locale": "de-DE"})
	mam, err := r.getWith(ctx, t, "MAM", "", "", locale)
	if err != nil {
		return err
	}
	xs := []*exchange{mam}
	if r.hasProfile && contains(r.profile, "Config") {
		x, err := r.getWith(ctx, t, "Config", r.app, "", locale)
		if err != nil {
			return err
		}
		xs = append(xs, x)
	}
	if err := r.until(ctx, r.completeAll(xs...)); err != nil {
		return err
	}
	ok, judged := true, false
	for _, x := range xs {
		ns := r.reply(t, x)
		if len(ns) == 0 {
			if x == mam && !r.mamAnswered {
				continue
			}
			r.violation(t, "REQ-LOC-002", x, "Get %s with the Locale de-DE (MessageId %s) is not answered within %s, the same Get without Locale is", x.resource, x.id, r.opt.Timeout)
			ok = false
			continue
		}
		judged = true
		for _, n := range ns {
			for _, d := range n.content() {
				found := map[string]string{}
				localized(d.payload, "", found)
				var bad []string
				for p, l := range found {
					if !locales[l] {
						bad = append(bad, fmt.Sprintf("%s=%q", strings.TrimPrefix(p, "/"), l))
					}
				}
				if len(bad) > 0 {
					sort.Strings(bad)
					r.violation(t, "REQ-LOC-002", x, "the %s of %q answers the Locale de-DE with LocalizedText in another language: %s", x.resource, d.name, strings.Join(bad, ", "))
					ok = false
				}
			}
		}
	}
	switch {
	case ok && judged:
		r.checked(t, "REQ-LOC-002", mam)
	case ok:
		r.skip(t, "REQ-LOC-002", "Get MAM/<AppId> is not answered either (discovery)")
	}
	return nil
}
