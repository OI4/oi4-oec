package payload

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"unicode/utf8"
)

// Content of the DataSets (MG 8.3): the helper DataSets PaginationRequest,
// Pagination and Locale (areas PAG, LOC) for every resource and the DataSets
// of MAM, Health, Profile, PublicationList and ReferenceDesignation (areas
// RES-*, C-027). Only MUST rules are reported; recommendations (SHOULD) are
// not checked.

// dsm is a DataSetMessage of a ua-data NetworkMessage with a valid
// DataSetWriterId.
type dsm struct {
	ptr     string
	id      int64
	name    string // DataSetWriterName, "" if absent or malformed
	group   string // WriterGroupName, "" if absent or malformed
	payload map[string]any
}

// resourceRule names the requirements of the method payloads of a resource
// ("" = no rule in this area).
type resourceRule struct {
	get, set, del string
	content       func(c *checker, d dsm)
}

var resourceRules = map[string]resourceRule{
	"MAM":                  {get: "REQ-RES-MAM-020", content: (*checker).mam},
	"Health":               {get: "REQ-RES-HEALTH-005", content: (*checker).health},
	"Profile":              {get: "REQ-RES-PROFILE-003", content: (*checker).profile},
	"PublicationList":      {get: "REQ-RES-PUBLIST-019", set: "REQ-RES-PUBLIST-022", content: (*checker).publicationEntry},
	"ReferenceDesignation": {get: "REQ-RES-REFDES-003", set: "REQ-RES-REFDES-006", del: "REQ-RES-REFDES-007", content: (*checker).referenceDesignation},
}

// commonServices are the services of MG 7.5.2 a Profile may list (SF-066 (a)).
var commonServices = map[string]bool{"NewDataSetWriterId": true}

var (
	domainPattern  = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
	healthRank     = map[string]int{"NORMAL_0": 0, "MAINTENANCE_REQUIRED_4": 1, "CHECK_FUNCTION_2": 2, "OFF_SPEC_3": 3, "FAILURE_1": 4} // SF-065 (a)
	configValues   = map[string]bool{"NONE_0": true, "MODE_1": true, "INTERVAL_2": true, "MODE_AND_INTERVAL_3": true}
	filteredLists  = map[string]bool{"Data": true, "Metadata": true, "Config": true, "Event": true}
	publishingMode = map[string]modeFlags{
		"OFF_0": {}, "ON_REQUEST_1": {request: true},
		"APPLICATION_2": {app: true}, "SOURCE_3": {source: true}, "FILTER_4": {filter: true},
		"APPLICATION_SOURCE_5": {app: true, source: true}, "APPLICATION_FILTER_6": {app: true, filter: true},
		"SOURCE_FILTER_7": {source: true, filter: true}, "APPLICATION_SOURCE_FILTER_8": {app: true, source: true, filter: true},
		"ON_REQUEST_FILTER_9": {request: true, filter: true}, "ON_REQUEST_SOURCE_10": {request: true, source: true},
		"ON_REQUEST_APPLICATION_11": {request: true, app: true}, "ON_REQUEST_SOURCE_FILTER_12": {request: true, source: true, filter: true},
		"ON_REQUEST_APPLICATION_FILTER_13":        {request: true, app: true, filter: true},
		"ON_REQUEST_APPLICATION_SOURCE_14":        {request: true, app: true, source: true},
		"ON_REQUEST_APPLICATION_SOURCE_FILTER_15": {request: true, app: true, source: true, filter: true},
	}
)

// modeFlags are the publication forms a Mode of a PublicationList entry
// enables (MG 8.3.8).
type modeFlags struct{ request, app, source, filter bool }

func (f modeFlags) publishes() bool { return f.app || f.source || f.filter }

// contents checks the DataSets of a ua-data NetworkMessage and their
// combination per Method.
func (c *checker) contents(dsms []dsm) {
	var res []dsm
	seen := map[int64]string{}
	for _, d := range dsms {
		switch d.id {
		case 1, 2:
			want := map[int64]string{1: "Get", 2: "Pub"}[d.id]
			if c.t.Method != want {
				c.add(d.ptr, "REQ-PAG-008", "a %s is only sent with the method %s", helperNames[d.id], want)
			}
			if prev, dup := seen[d.id]; dup {
				c.add(d.ptr, "REQ-PAG-008", "a second %s (first at %s)", helperNames[d.id], prev)
			}
			seen[d.id] = d.ptr
			if d.payload == nil {
				continue
			}
			if d.id == 1 {
				c.paginationRequest(d)
			}
		case 3:
			if d.payload != nil {
				c.locale(d)
			}
		default:
			res = append(res, d)
		}
	}
	for _, d := range dsms {
		if d.id == 2 && d.payload != nil && c.t.Method == "Pub" {
			c.pagination(d, len(res))
		}
	}
	rule, ok := resourceRules[c.t.Resource]
	if !ok {
		return
	}
	switch c.t.Method {
	case "Get":
		for _, d := range res {
			c.add(d.ptr, rule.get, "a Get %s carries no %s DataSetMessage", c.t.Resource, c.t.Resource)
		}
		return
	case "Set":
		if rule.set != "" && len(res) != 1 {
			c.add("/Messages", rule.set, "a Set %s carries exactly one %s DataSetMessage, got %d", c.t.Resource, c.t.Resource, len(res))
		}
	case "Del":
		if rule.del != "" {
			for _, d := range res {
				c.add(d.ptr, rule.del, "a Del %s carries no %s DataSetMessage", c.t.Resource, c.t.Resource)
			}
			return
		}
	}
	entries := map[string]string{}
	for _, d := range res {
		if d.payload == nil {
			continue
		}
		rule.content(c, d)
		if _, ok := d.payload["Resource"].(string); ok && c.t.Resource == "PublicationList" {
			key := fmt.Sprintf("%v|%v|%v", d.payload["Resource"], d.payload["Source"], d.payload["Filter"])
			if prev, dup := entries[key]; dup {
				c.add(d.ptr, "REQ-RES-PUBLIST-003", "Resource, Source and Filter repeat the entry at %s", prev)
			}
			entries[key] = d.ptr
		}
	}
}

func (c *checker) int32(m map[string]any, ptr, key, req string) {
	v, ok := m[key]
	if !ok {
		return
	}
	n, isNum := v.(json.Number)
	if _, err := strconv.ParseInt(n.String(), 10, 32); !isNum || err != nil {
		c.add(join(ptr, key), req, "%s must be an Int32", key)
	}
}

func (c *checker) enum(m map[string]any, ptr, key, req string, values map[string]bool) (string, bool) {
	s, ok := c.str(m, ptr, key, req)
	if ok && !values[s] {
		c.add(join(ptr, key), req, "%s %q is no value of the enumeration", key, s)
		return "", false
	}
	return s, ok
}

// --- helper DataSets (MG 8.3.11, 8.3.12) ------------------------------------

func (c *checker) paginationRequest(d dsm) {
	const req = "REQ-PAG-001"
	p := join(d.ptr, "Payload")
	m := c.object(p, d.payload, shape{required: []string{"PerPage", "Page"}, missing: req})
	if n, ok := c.uint(m, p, "PerPage", req, maxUint32); ok && n == 0 {
		c.add(join(p, "PerPage"), req, "PerPage must be > 0")
	}
	c.uint(m, p, "Page", req, maxUint32)
}

func (c *checker) pagination(d dsm, count int) {
	const req = "REQ-PAG-003"
	p := join(d.ptr, "Payload")
	m := c.object(p, d.payload, shape{required: []string{"TotalCount", "PerPage", "Page", "HasNext", "PaginationId"}, missing: req})
	total, totalOK := c.uint(m, p, "TotalCount", req, maxUint32)
	perPage, perPageOK := c.uint(m, p, "PerPage", req, maxUint32)
	page, pageOK := c.uint(m, p, "Page", req, maxUint32)
	hasNext, hasNextOK := m["HasNext"].(bool)
	if _, present := m["HasNext"]; present && !hasNextOK {
		c.add(join(p, "HasNext"), req, "HasNext must be a Boolean")
	}
	id, idOK := c.str(m, p, "PaginationId", req)
	if idOK && c.pub != "" {
		if g := messageIDPattern.FindStringSubmatch(id); g == nil || g[1] != c.pub {
			c.add(join(p, "PaginationId"), req, "PaginationId %q is no MessageId of the sender %s", id, c.pub)
		}
	}
	const cons = "REQ-PAG-004"
	if pageOK && page == 0 {
		c.add(join(p, "Page"), cons, "the published Page starts at 1")
		return
	}
	if perPageOK && perPage == 0 {
		c.add(join(p, "PerPage"), cons, "PerPage must be > 0")
		return
	}
	if !totalOK || !perPageOK || !pageOK {
		return
	}
	want := int64(0)
	if rest := total - (page-1)*perPage; rest > 0 {
		want = min(rest, perPage)
	}
	if int64(count) != want {
		c.add(p, cons, "page %d of %d DataSetMessages with PerPage %d contains %d DataSetMessages, got %d", page, total, perPage, want, count)
	}
	if hasNextOK && hasNext != (page*perPage < total) {
		c.add(join(p, "HasNext"), cons, "HasNext must be %v on page %d of %d DataSetMessages with PerPage %d", !hasNext, page, total, perPage)
	}
	if idOK && page == 1 && c.msgID != "" && id != c.msgID {
		c.add(join(p, "PaginationId"), cons, "PaginationId of page 1 must be its own MessageId %q", c.msgID)
	}
}

func (c *checker) locale(d dsm) {
	const req = "REQ-LOC-001"
	p := join(d.ptr, "Payload")
	m := c.object(p, d.payload, shape{required: []string{"Locale"}, missing: req})
	if s, ok := c.str(m, p, "Locale", req); ok && !localePattern.MatchString(s) {
		c.add(join(p, "Locale"), req, "Locale %q is no LocaleId", s)
	}
}

// --- MAM (MG 8.3.1) ----------------------------------------------------------

func (c *checker) mam(d dsm) {
	const req = "REQ-RES-MAM-001"
	p := join(d.ptr, "Payload")
	m := c.object(p, d.payload, shape{
		required: []string{"Manufacturer", "ManufacturerUri", "Model", "ProductCode", "HardwareRevision",
			"SoftwareRevision", "DeviceRevision", "DeviceManual", "DeviceClass", "SerialNumber",
			"ProductInstanceUri", "RevisionCounter"},
		optional: []string{"Description", "GlobalAssetId"},
		missing:  req,
	})
	for _, k := range []string{"Manufacturer", "Model", "Description"} {
		if v, ok := m[k]; ok {
			c.localizedText(join(p, k), v)
		}
	}
	for _, k := range []string{"ProductCode", "HardwareRevision", "SoftwareRevision", "DeviceRevision", "DeviceClass", "SerialNumber", "GlobalAssetId"} {
		c.str(m, p, k, req)
	}
	if s, ok := c.str(m, p, "ManufacturerUri", req); ok && !domainPattern.MatchString(s) {
		c.add(join(p, "ManufacturerUri"), "REQ-RES-MAM-004", "ManufacturerUri %q is no lower-case domain name without scheme", s)
	}
	if s, ok := c.str(m, p, "DeviceManual", req); ok && s != "" {
		if u, err := url.Parse(s); err != nil || u.Scheme == "" || u.Host == "" {
			c.add(join(p, "DeviceManual"), "REQ-RES-MAM-012", "DeviceManual %q is neither empty nor a URL", s)
		}
	}
	if s, ok := c.str(m, p, "ProductInstanceUri", req); ok {
		if d.name != "" && s != d.name {
			c.add(join(p, "ProductInstanceUri"), "REQ-RES-MAM-016", "ProductInstanceUri %q differs from the DataSetWriterName %q", s, d.name)
		}
		if n := utf8.RuneCountInString(s); n > 255 {
			c.add(join(p, "ProductInstanceUri"), "REQ-RES-MAM-017", "ProductInstanceUri has %d > 255 characters", n)
		}
	}
	c.int32(m, p, "RevisionCounter", req)
}

// --- Health (MG 8.3.2) -------------------------------------------------------

func (c *checker) health(d dsm) {
	const req = "REQ-RES-HEALTH-001"
	p := join(d.ptr, "Payload")
	m := c.object(p, d.payload, shape{required: []string{"Health"}, optional: []string{"HealthScore", "Details"}, missing: req})
	values := map[string]bool{}
	for k := range healthRank {
		values[k] = true
	}
	health, healthOK := c.enum(m, p, "Health", req, values)
	c.uint(m, p, "HealthScore", req, 100)
	v, ok := m["Details"]
	if !ok {
		return
	}
	arr, isArr := v.([]any)
	if !isArr {
		c.add(join(p, "Details"), req, "Details must be a JSON array")
		return
	}
	worst, worstPtr, allOK := "", "", true
	for i, e := range arr {
		ep := fmt.Sprintf("%s/Details/%d", p, i)
		dm := c.object(ep, e, shape{required: []string{"Health"}, optional: []string{"DiagnosticCode", "Location", "Description"}, missing: req})
		if dm == nil {
			allOK = false
			continue
		}
		h, ok := c.enum(dm, ep, "Health", req, values)
		if !ok {
			allOK = false
		} else if worst == "" || healthRank[h] > healthRank[worst] {
			worst, worstPtr = h, ep
		}
		for _, k := range []string{"DiagnosticCode", "Location", "Description"} {
			c.str(dm, ep, k, req)
		}
	}
	if healthOK && allOK && worst != "" && health != worst {
		c.add(join(p, "Health"), "REQ-RES-HEALTH-002", "Health %s is not the most severe Health of the Details (%s at %s)", health, worst, worstPtr)
	}
}

// --- Profile (MG 8.3.7) ------------------------------------------------------

func (c *checker) profile(d dsm) {
	const req = "REQ-RES-PROFILE-001"
	p := join(d.ptr, "Payload")
	m := c.object(p, d.payload, shape{required: []string{"Resources"}, missing: req})
	v, ok := m["Resources"]
	if !ok {
		return
	}
	arr, isArr := v.([]any)
	if !isArr {
		c.add(join(p, "Resources"), req, "Resources must be a JSON array")
		return
	}
	listed := map[string]bool{}
	for i, e := range arr {
		ep := fmt.Sprintf("%s/Resources/%d", p, i)
		s, isStr := e.(string)
		switch {
		case !isStr:
			c.add(ep, req, "a resource name is a string")
		case !resources[s] && !commonServices[s]:
			c.add(ep, req, "%q is no resource (MG 7.5.1) or common service (MG 7.5.2)", s)
		case listed[s]:
			c.add(ep, req, "resource %s is listed twice", s)
		}
		listed[s] = true
	}
	for _, r := range []string{"MAM", "Health", "Profile"} {
		if !listed[r] {
			c.add(join(p, "Resources"), "REQ-RES-PROFILE-002", "the Profile does not list the supported resource %s", r)
		}
	}
}

// --- PublicationList (MG 8.3.8) ----------------------------------------------

func (c *checker) publicationEntry(d dsm) {
	const req = "REQ-RES-PUBLIST-001"
	p := join(d.ptr, "Payload")
	m := c.object(p, d.payload, shape{
		required: []string{"Resource", "Source", "DataSetWriterId"},
		optional: []string{"Filter", "Mode", "Interval", "Precisions", "Config"},
		missing:  req,
	})
	resource, resourceOK := c.enum(m, p, "Resource", req, resources)
	source, sourceOK := c.str(m, p, "Source", req)
	filter, filterOK := c.str(m, p, "Filter", req)
	if id, ok := c.uint(m, p, "DataSetWriterId", req, 65535); ok && id < 10 {
		c.add(join(p, "DataSetWriterId"), "REQ-MSG-020", "DataSetWriterId %d of the publication is invalid or reserved", id)
	}
	mode, modeOK := c.str(m, p, "Mode", req)
	flags, known := publishingMode[mode]
	if modeOK && !known {
		c.add(join(p, "Mode"), req, "Mode %q is no value of the enumeration", mode)
		modeOK = false
	}
	if _, present := m["Mode"]; !present {
		mode, modeOK = "ON_REQUEST_1", true // default (MG 8.3.8)
		flags = publishingMode[mode]
	}
	c.uint(m, p, "Interval", req, maxUint32)
	c.enum(m, p, "Config", req, configValues)
	if v, ok := m["Precisions"]; ok {
		pp := join(p, "Precisions")
		pm, isObj := v.(map[string]any)
		if !isObj {
			c.add(pp, req, "Precisions must be a JSON object")
		}
		for k, e := range pm {
			n, isNum := e.(json.Number)
			f, err := n.Float64()
			if !isNum || err != nil || f < 0 || f > 3.4e38 {
				c.add(join(pp, k), req, "a Precision is a Real in 0.0 … 3.4·10^38")
			}
		}
	}
	const match = "REQ-RES-PUBLIST-002"
	if resourceOK {
		_, hasFilter := m["Filter"]
		switch {
		case filteredLists[resource] && !hasFilter:
			c.add(p, match, "an entry of %s needs a Filter (its Tag)", resource)
		case !filteredLists[resource] && hasFilter:
			c.add(join(p, "Filter"), match, "an entry of %s has no Filter", resource)
		}
		want := resource
		if filterOK {
			want += "/" + filter
		}
		if d.group != "" && d.group != want {
			c.add(join(d.ptr, "WriterGroupName"), match, "WriterGroupName %q of the entry is not %q", d.group, want)
		}
	}
	if sourceOK && d.name != "" && d.name != source {
		c.add(join(d.ptr, "DataSetWriterName"), match, "DataSetWriterName %q of the entry differs from its Source %q", d.name, source)
	}
	if !resourceOK || !modeOK {
		return
	}
	const forms = "REQ-RES-PUBLIST-012"
	switch resource {
	case "Event":
		if flags != (modeFlags{}) && flags != (modeFlags{filter: true}) {
			c.add(join(p, "Mode"), forms, "an Event entry has the Mode OFF_0 or FILTER_4, got %s", mode)
		}
	case "Metadata":
		if mode != "ON_REQUEST_FILTER_9" {
			c.add(join(p, "Mode"), forms, "a Metadata entry has the Mode ON_REQUEST_FILTER_9, got %q", mode)
		}
	case "MAM", "Health":
		if !flags.publishes() {
			c.add(join(p, "Mode"), forms, "the %s entry must enable a publication (APPLICATION, SOURCE or FILTER)", resource)
		}
	}
}

// --- ReferenceDesignation (MG 8.3.10) ----------------------------------------

func (c *checker) referenceDesignation(d dsm) {
	const req = "REQ-RES-REFDES-001"
	p := join(d.ptr, "Payload")
	m := c.object(p, d.payload, shape{optional: []string{"Function", "Product", "Location"}, missing: req})
	for _, k := range []string{"Function", "Product", "Location"} {
		v, ok := m[k]
		if !ok {
			continue
		}
		op := join(p, k)
		o := c.object(op, v, shape{required: []string{"Value"}, optional: []string{"Local", "Parent"}, missing: req})
		c.str(o, op, "Value", req)
		c.str(o, op, "Local", req)
		if pv, ok := o["Parent"]; ok {
			pp := join(op, "Parent")
			po := c.object(pp, pv, shape{required: []string{"Value"}, optional: []string{"Local", "IdentificationLink"}, missing: req})
			for _, f := range []string{"Value", "Local", "IdentificationLink"} {
				c.str(po, pp, f, req)
			}
		}
	}
}
