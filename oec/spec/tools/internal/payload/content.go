package payload

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OI4/oec/spec/tools/internal/il"
)

// Content of the DataSets (MG 8.3): the helper DataSets PaginationRequest,
// Pagination, Locale and Query (areas PAG, LOC, QRY, RES-REFDES) for
// every resource and the DataSets of MAM, Health, Profile, PublicationList
// and ReferenceDesignation (areas RES-*, C-027) and of Data (Oi4Pv), Event
// and SubscriptionList (C-028) and Config (C-029). The application-defined
// Data DataSets themselves are not checked. Recommendations (SHOULD) are
// only reported where a violation is unambiguous (REQ-MSG-026 for a Good
// Status, REQ-RES-MAM-014 for an Oi4.<ServiceType> DeviceClass of another
// ServiceType, C-036).

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
// ("" = no rule in this area). A Del carries no resource DataSetMessage
// unless delOne is set (exactly one, the entry to delete).
type resourceRule struct {
	get, set, del string
	delOne        bool
	content       func(c *checker, d dsm)
}

var resourceRules = map[string]resourceRule{
	"MAM":                  {get: "REQ-RES-MAM-020", content: (*checker).mam},
	"Health":               {get: "REQ-RES-HEALTH-005", content: (*checker).health},
	"Profile":              {get: "REQ-RES-PROFILE-003", content: (*checker).profile},
	"PublicationList":      {get: "REQ-RES-PUBLIST-019", set: "REQ-RES-PUBLIST-022", content: (*checker).publicationEntry},
	"ReferenceDesignation": {get: "REQ-RES-REFDES-003", set: "REQ-RES-REFDES-006", del: "REQ-RES-REFDES-007", content: (*checker).referenceDesignation},
	"Data":                 {get: "REQ-RES-DATA-006", set: "REQ-RES-DATA-009", content: (*checker).dataSet},
	"Event":                {content: (*checker).event},
	"Config":               {get: "REQ-RES-CONFIG-016", set: "REQ-RES-CONFIG-019", content: (*checker).config},
	"SubscriptionList":     {get: "REQ-RES-SUBLIST-007", set: "REQ-RES-SUBLIST-010", del: "REQ-RES-SUBLIST-011", delOne: true, content: (*checker).subscriptionEntry},
}

// commonServices are the services of MG 7.5.2 a Profile may list (MG 8.3.7).
var commonServices = map[string]bool{"NewDataSetWriterId": true}

var (
	domainPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
	// Order of priority of MG 8.3.2: FAILURE_1 > CHECK_FUNCTION_2 > OFF_SPEC_3 > MAINTENANCE_REQUIRED_4 > NORMAL_0.
	healthRank     = map[string]int{"NORMAL_0": 0, "MAINTENANCE_REQUIRED_4": 1, "OFF_SPEC_3": 2, "CHECK_FUNCTION_2": 3, "FAILURE_1": 4}
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

// publishingModes are the values of the enumeration Mode of a
// PublicationList entry (MG 8.3.8).
var publishingModes = func() map[string]bool {
	out := map[string]bool{}
	for k := range publishingMode {
		out[k] = true
	}
	return out
}()

var (
	// filterLevelPattern matches one level of a topic-encoded String
	// (filter-level of topic.abnf, MG 7.7, SF-040).
	filterLevelPattern = regexp.MustCompile(`^(?:[^\x00#%+/]|%2B|%23|%2F|%25)+$`)
	// eventFilterPattern is the level structure of an Event Filter
	// (event-filter of topic.abnf, SF-042); the characters of the levels are
	// checked by filterLevelPattern.
	eventFilterPattern = regexp.MustCompile(`^(?:Syslog/(?:Debug|Informational|Notice|Warning|Error|Critical|Alert|Emergency)|Status/(?:Good|Uncertain|Bad)|Generic/[^/]*)$`)
)

// isTagOf reports whether the levels of a Filter have the structure of a Tag
// of the resource (list-filter of topic.abnf, REQ-TOP-022): one level for
// Data, Metadata and Config, <EventCategory>/<EventLevel> for Event.
func isTagOf(resource, filter string) bool {
	if resource == "Event" {
		return eventFilterPattern.MatchString(filter)
	}
	return !strings.Contains(filter, "/")
}

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
			if c.t.Method == "Set" || c.t.Method == "Del" {
				c.add(d.ptr, "REQ-PAG-009", "a %s is not paginated", c.t.Method)
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
			if c.t.Method != "Get" {
				c.add(d.ptr, "REQ-LOC-003", "a Locale is only sent with the method Get")
			}
			if prev, dup := seen[d.id]; dup {
				c.add(d.ptr, "REQ-LOC-003", "a second Locale (first at %s)", prev)
			}
			seen[d.id] = d.ptr
			if d.payload != nil {
				c.locale(d)
			}
		case 9:
			query, defined := queries[c.t.Resource]
			if c.t.Method != "Get" || !defined {
				c.add(d.ptr, "REQ-QRY-001", "a Query is only sent with the method Get of a resource with a defined Query, not with %s/%s", c.t.Method, c.t.Resource)
			}
			if prev, dup := seen[d.id]; dup {
				c.add(d.ptr, "REQ-QRY-001", "a second Query (first at %s)", prev)
			}
			seen[d.id] = d.ptr
			if defined && c.t.Method == "Get" {
				query(c, d)
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
		if rule.del != "" && rule.delOne {
			if len(res) != 1 {
				c.add("/Messages", rule.del, "a Del %s carries exactly one %s DataSetMessage, got %d", c.t.Resource, c.t.Resource, len(res))
			}
		} else if rule.del != "" {
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

// numberedEnum matches the values of an OPC UA enumeration in
// VerboseEncoding ("<Name>_<Value>", REQ-MSG-031).
var numberedEnum = regexp.MustCompile(`_[0-9]+$`)

func (c *checker) enum(m map[string]any, ptr, key, req string, values map[string]bool) (string, bool) {
	if n, isNum := m[key].(json.Number); isNum {
		numbered := len(values) > 0
		for v := range values {
			numbered = numbered && numberedEnum.MatchString(v)
		}
		if numbered {
			c.add(join(ptr, key), "REQ-MSG-031", "%s %s is the numeric value of an enumeration; VerboseEncoding writes \"<Name>_<Value>\"", key, n)
		}
	}
	s, ok := c.str(m, ptr, key, req)
	if ok && !values[s] {
		c.add(join(ptr, key), req, "%s %q is no value of the enumeration", key, s)
		return "", false
	}
	return s, ok
}

// --- helper DataSets (MG 8.3.11, 8.3.12, 8.3.13) ----------------------------

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

// queries are the resources with a defined Query (MG 8.3.13) and the check of
// their query parameters.
var queries = map[string]func(*checker, dsm){
	"ReferenceDesignation": (*checker).refDesQuery,
}

// refDesAspects are the values of the query parameter Aspect of
// ReferenceDesignation (MG 8.3.13.1).
var refDesAspects = map[string]bool{"Function": true, "Product": true, "Location": true}

// refDesQuery checks the Query of ReferenceDesignation (MG 8.3.13.1): only
// without Source, query parameters Parent and Aspect.
func (c *checker) refDesQuery(d dsm) {
	if c.t.SourceIL != "" {
		c.add(d.ptr, "REQ-RES-REFDES-013", "a Query of ReferenceDesignation is only sent with Get/ReferenceDesignation without Source")
	}
	if d.payload == nil {
		return
	}
	const req = "REQ-RES-REFDES-012"
	p := join(d.ptr, "Payload")
	m := c.object(p, d.payload, shape{required: []string{"Parent"}, optional: []string{"Aspect"}, missing: req})
	if s, ok := c.str(m, p, "Parent", req); ok && s == "" {
		c.add(join(p, "Parent"), req, "Parent must contain an Identification Link")
	}
	c.enum(m, p, "Aspect", req, refDesAspects)
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
		c.mamIdentifier(p, m, s)
	}
	c.int32(m, p, "RevisionCounter", req)
	if dc, ok := m["DeviceClass"].(string); ok && d.name != "" && d.name == c.t.AppIL {
		if st, found := strings.CutPrefix(dc, "Oi4."); found && serviceTypes[st] && st != c.t.ServiceType {
			c.add(join(p, "DeviceClass"), "REQ-RES-MAM-014", "the DeviceClass %q of the application names another ServiceType than %s of its topic", dc, c.t.ServiceType)
		}
	}
}

// mamIdentifier checks REQ-RES-MAM-006 heuristically (C-036): a
// ProductInstanceUri without scheme of four segments whose first segment is
// the ManufacturerUri is taken for a generated Oi4Identifier (MG 4.3.1), so
// its elements are the Model (only if its Locale is en-US, REQ-IL-027),
// ProductCode and SerialNumber of the MAM, plain or dnp-encoded (MG A3), "nd"
// for an empty value.
func (c *checker) mamIdentifier(p string, m map[string]any, piu string) {
	mu, ok := m["ManufacturerUri"].(string)
	seg := strings.Split(piu, "/")
	if !ok || mu == "" || il.Scheme(piu) != "" || len(seg) != 4 || seg[0] != strings.ToLower(mu) {
		return
	}
	type element struct{ name, value string }
	var elems [3]*element
	if lt, ok := m["Model"].(map[string]any); ok && lt["Locale"] == "en-US" {
		if t, ok := lt["Text"].(string); ok {
			elems[0] = &element{"Model", t}
		}
	}
	for i, k := range []string{"ProductCode", "SerialNumber"} {
		if v, ok := m[k].(string); ok {
			elems[i+1] = &element{k, v}
		}
	}
	for i, e := range elems {
		if e == nil {
			continue
		}
		s := seg[i+1]
		if e.value == "" && s == il.NoData || e.value != "" && (s == e.value || s == il.DnpEncode(e.value)) {
			continue
		}
		c.add(join(p, "ProductInstanceUri"), "REQ-RES-MAM-006", "ProductInstanceUri %q is an Oi4Identifier of the ManufacturerUri, but its element %q is not the %s %q", piu, s, e.name, e.value)
	}
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
	if healthOK && allOK && worst != "" && healthRank[health] != healthRank[worst] {
		c.add(join(p, "Health"), "REQ-RES-HEALTH-002", "Health %s is not the Health with the highest priority of the Details (%s at %s)", health, worst, worstPtr)
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
	if filterOK {
		for _, l := range strings.Split(filter, "/") {
			if !filterLevelPattern.MatchString(l) {
				c.add(join(p, "Filter"), req, "Filter %q is no topic-encoded String (MG 7.7)", filter)
				break
			}
		}
	}
	mode, modeOK := c.enum(m, p, "Mode", req, publishingModes)
	flags := publishingMode[mode]
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
		case filterOK && !isTagOf(resource, filter):
			c.add(join(p, "Filter"), match, "Filter %q is no Tag of %s", filter, resource)
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

// HasQuery reports whether a Query (MG 8.3.13) is defined for the resource.
func HasQuery(resource string) bool {
	_, ok := queries[resource]
	return ok
}

// --- Data (MG 8.3.4) ---------------------------------------------------------

// svPattern matches the secondary values Sv1 … Sv255 of Oi4Pv.
var svPattern = regexp.MustCompile(`^Sv([1-9][0-9]?|1[0-9][0-9]|2[0-4][0-9]|25[0-5])$`)

// dataSet checks a Data DataSet; only the predefined DataSet Oi4Pv has a
// defined structure (REQ-RES-DATA-004).
func (c *checker) dataSet(d dsm) {
	if d.group != "Oi4Pv" {
		return
	}
	const req = "REQ-RES-DATA-004"
	p := join(d.ptr, "Payload")
	if _, ok := d.payload["Pv"]; !ok {
		c.add(p, req, "missing Pv")
	}
	for k := range d.payload {
		if k != "Pv" && !svPattern.MatchString(k) {
			c.add(join(p, k), req, "%s is no field of Oi4Pv (Pv, Sv1 … Sv255)", k)
		}
	}
}

// --- Event (MG 8.3.6) --------------------------------------------------------

var (
	eventCategories = map[string]string{"Syslog": "CAT_SYSLOG_0", "Status": "CAT_STATUS_1", "Generic": "CAT_GENERIC_99"}
	categoryValues  = map[string]bool{"CAT_SYSLOG_0": true, "CAT_STATUS_1": true, "CAT_GENERIC_99": true}
	// Severity names of RFC 5424 6.2.1 by PRI modulo 8 (MG 8.3.6.2).
	syslogLevels = []string{"Emergency", "Alert", "Critical", "Error", "Warning", "Notice", "Informational", "Debug"}
	// Severity classes of a StatusCode by bits 30–31 (OPC UA Part 4 7.39);
	// the reserved class 3 is treated as Bad (MG 8.3.6.1).
	statusClasses = []string{"Good", "Uncertain", "Bad", "Bad"}
)

func (c *checker) event(d dsm) {
	p := join(d.ptr, "Payload")
	m := c.object(p, d.payload, shape{required: []string{"Number", "Category"}, optional: []string{"Description", "Details"}, missing: "REQ-RES-EVENT-001"})
	if m == nil {
		return
	}
	number, numberOK := c.uint(m, p, "Number", "REQ-RES-EVENT-001", maxUint32)
	category, categoryOK := c.enum(m, p, "Category", "REQ-RES-EVENT-001", categoryValues)
	c.str(m, p, "Description", "REQ-RES-EVENT-001")
	var details map[string]any
	if v, ok := m["Details"]; ok {
		if details, ok = v.(map[string]any); !ok {
			c.add(join(p, "Details"), "REQ-RES-EVENT-001", "Details must be a JSON object")
		}
	}
	groupCat, level, _ := strings.Cut(d.group, "/")
	if want, known := eventCategories[groupCat]; known && categoryOK && category != want {
		c.add(join(p, "Category"), "REQ-RES-EVENT-002", "Category %s does not correspond to the EventCategory %s of the WriterGroupName", category, groupCat)
	}
	dp := join(p, "Details")
	switch category {
	case "CAT_STATUS_1":
		const req = "REQ-RES-EVENT-003"
		for k := range details {
			if k != "SymbolicId" {
				c.add(join(dp, k), req, "%s is no Details key of a Status Event (SymbolicId)", k)
			}
		}
		symbol, symbolOK := c.str(details, dp, "SymbolicId", req)
		if !numberOK || groupCat != "Status" {
			return
		}
		class := statusClasses[number>>30]
		if level != class {
			c.add(join(d.ptr, "WriterGroupName"), "REQ-RES-EVENT-004", "EventLevel %q is not the severity class %s of the StatusCode %d", level, class, number)
		}
		if symbolOK && !strings.HasPrefix(symbol, class) {
			c.add(join(dp, "SymbolicId"), "REQ-RES-EVENT-004", "SymbolicId %q does not start with the severity class %s of the StatusCode", symbol, class)
		}
	case "CAT_SYSLOG_0":
		const req = "REQ-RES-EVENT-007"
		if _, ok := m["Description"]; ok {
			c.add(join(p, "Description"), req, "a Syslog Event has no Description")
		}
		for k := range details {
			if k != "MSG" && k != "HEADER" {
				c.add(join(dp, k), req, "%s is no Details key of a Syslog Event (MSG, HEADER)", k)
			}
		}
		c.str(details, dp, "MSG", req)
		c.str(details, dp, "HEADER", req)
		if !numberOK {
			return
		}
		if number > 191 {
			c.add(join(p, "Number"), req, "Number %d is no syslog PRI (0 … 191)", number)
			return
		}
		if want := syslogLevels[number%8]; groupCat == "Syslog" && level != want {
			c.add(join(d.ptr, "WriterGroupName"), "REQ-RES-EVENT-008", "EventLevel %q is not the severity %s of the PRI %d", level, want, number)
		}
	}
}

// --- SubscriptionList (MG 8.3.9) ---------------------------------------------

var subscriptionConfigs = map[string]bool{"NONE_0": true, "CONF_1": true}

// subscriptionEntry checks an entry of the SubscriptionList: fields
// (REQ-RES-SUBLIST-001) and the WriterGroupName derived from the TopicPath
// (REQ-RES-SUBLIST-002): the resource level of an OEC TopicPath must be
// concrete (MG 8.3.9); topic filters outside the OEC namespace are not checked.
func (c *checker) subscriptionEntry(d dsm) {
	const req = "REQ-RES-SUBLIST-001"
	p := join(d.ptr, "Payload")
	m := c.object(p, d.payload, shape{required: []string{"TopicPath"}, optional: []string{"Interval", "Config"}, missing: req})
	c.uint(m, p, "Interval", req, maxUint32)
	c.enum(m, p, "Config", req, subscriptionConfigs)
	path, ok := c.str(m, p, "TopicPath", req)
	if !ok {
		return
	}
	levels := strings.Split(path, "/")
	if levels[0] != "Oi4v2" && levels[0] != "+" && levels[0] != "#" {
		return
	}
	if len(levels) < 5 || slices.Contains(levels[:5], "#") || strings.ContainsAny(levels[4], "+#") {
		c.add(join(p, "TopicPath"), "REQ-RES-SUBLIST-002", "the resource level of TopicPath %q is not concrete", path)
		return
	}
	if d.group == "" {
		return
	}
	want, filter := levels[4], levels[min(len(levels), 6):]
	wild := len(levels) > 5 && levels[5] == "#"
	for _, l := range filter {
		wild = wild || strings.ContainsAny(l, "+#")
	}
	if wild {
		if rt, _, _ := strings.Cut(d.group, "/"); rt != want {
			c.add(join(d.ptr, "WriterGroupName"), "REQ-RES-SUBLIST-002", "WriterGroupName %q does not start with the resource %s of the TopicPath", d.group, want)
		}
		return
	}
	if len(filter) > 0 {
		want += "/" + strings.Join(filter, "/")
	}
	if d.group != want {
		c.add(join(d.ptr, "WriterGroupName"), "REQ-RES-SUBLIST-002", "WriterGroupName %q is not %q derived from the TopicPath", d.group, want)
	}
}

// --- Config (MG 8.3.3) -------------------------------------------------------

var (
	configTypes = map[string]bool{"Boolean": true, "ByteString": true, "DateTime": true, "Number": true, "String": true}
	// GroupName and ConfigName (MG 8.3.3.1/8.3.3.2, REQ-RES-CONFIG-004).
	configNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
	// Number as decimal text in JSON number syntax (REQ-RES-CONFIG-007, SF-084 (d)).
	configNumberPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)
)

// configMask replaces the value of a sensitive configuration object in every
// Pub and keeps it in a Set (GL-ADR-021, REQ-RES-CONFIG-024/025).
const configMask = "********"

// config checks a Config DataSet (REQ-RES-CONFIG-002 for Pub,
// REQ-RES-CONFIG-003 for Set): the keys of the Payload other than Context are
// groups, the keys of a Group other than Name and Description are
// configuration objects (SF-084 (a)). A non-empty Value or DefaultValue is
// checked against Type (REQ-RES-CONFIG-007) and a non-empty Value against the
// Validation of the same object (REQ-RES-CONFIG-008), except the mask of an
// object with Sensitive = true. In a Pub, a non-empty Value or DefaultValue of
// such an object must be the mask (REQ-RES-CONFIG-024). Whether the Set
// matches the published Config (names, Mandatory, read-only properties,
// Sensitive) is not decidable per message (REQ-RES-CONFIG-005/011/023/025).
func (c *checker) config(d dsm) {
	set := c.t.Method == "Set"
	req := "REQ-RES-CONFIG-002"
	if set {
		req = "REQ-RES-CONFIG-003"
	}
	p := join(d.ptr, "Payload")
	groups := 0
	for _, k := range sortedKeys(d.payload) {
		v := d.payload[k]
		if k == "Context" {
			m := c.object(join(p, k), v, shape{required: []string{"Name"}, optional: []string{"Description"}, missing: req})
			c.configTexts(m, join(p, k))
			continue
		}
		groups++
		c.configName(p, k, "GroupName")
		c.configGroup(join(p, k), v, req, set)
	}
	if groups == 0 {
		c.add(p, req, "a Config DataSet contains at least one Group")
	}
}

func (c *checker) configName(ptr, name, kind string) {
	if !configNamePattern.MatchString(name) {
		c.add(join(ptr, name), "REQ-RES-CONFIG-004", "%s %q contains characters other than a-z, A-Z, 0-9, - and _ or starts with a digit or -", kind, name)
	}
}

// configTexts checks the LocalizedTexts Name and Description of a Group or
// the Context (REQ-MSG-040).
func (c *checker) configTexts(m map[string]any, ptr string) {
	for _, k := range []string{"Name", "Description"} {
		if v, ok := m[k]; ok {
			c.localizedText(join(ptr, k), v)
		}
	}
}

func (c *checker) configGroup(ptr string, v any, req string, set bool) {
	m, ok := v.(map[string]any)
	if !ok {
		c.add(ptr, req, "a Group must be a JSON object")
		return
	}
	if _, ok := m["Name"]; !ok && !set {
		c.add(ptr, req, "missing field Name")
	}
	c.configTexts(m, ptr)
	configs := 0
	for _, k := range sortedKeys(m) {
		if k == "Name" || k == "Description" {
			continue
		}
		configs++
		c.configName(ptr, k, "ConfigName")
		c.configObject(join(ptr, k), m[k], req, set)
	}
	if configs == 0 {
		c.add(ptr, req, "a Group contains at least one configuration object")
	}
}

func (c *checker) configObject(ptr string, v any, req string, set bool) {
	s := shape{required: []string{"Type", "Value", "Name"}, optional: []string{"Unit", "DefaultValue", "Mandatory", "Sensitive", "Description", "Validation"}, missing: req}
	if set {
		s.required, s.optional = []string{"Value"}, append(s.optional, "Type", "Name")
	}
	m := c.object(ptr, v, s)
	if m == nil {
		return
	}
	typ, typeOK := c.enum(m, ptr, "Type", "REQ-RES-CONFIG-006", configTypes)
	value, valueOK := c.str(m, ptr, "Value", "REQ-RES-CONFIG-007")
	def, defOK := c.str(m, ptr, "DefaultValue", "REQ-RES-CONFIG-007")
	c.str(m, ptr, "Unit", req)
	for _, k := range []string{"Mandatory", "Sensitive"} {
		if b, ok := m[k]; ok {
			if _, isBool := b.(bool); !isBool {
				c.add(join(ptr, k), req, "%s must be a Boolean", k)
			}
		}
	}
	c.configTexts(m, ptr)
	sensitive := m["Sensitive"] == true
	if sensitive && !set {
		for _, f := range []struct {
			key, s string
			ok     bool
		}{{"Value", value, valueOK}, {"DefaultValue", def, defOK}} {
			if f.ok && f.s != "" && f.s != configMask {
				c.add(join(ptr, f.key), "REQ-RES-CONFIG-024", "%s of a sensitive configuration object is published unmasked (expected %q)", f.key, configMask)
			}
		}
	}
	masked := func(s string) bool { return sensitive && s == configMask }
	valueOK = valueOK && !masked(value)
	defOK = defOK && !masked(def)
	if typeOK {
		if valueOK {
			c.configValue(join(ptr, "Value"), typ, value)
		}
		if defOK {
			c.configValue(join(ptr, "DefaultValue"), typ, def)
		}
	}
	val, ok := m["Validation"]
	if !ok {
		return
	}
	vp := join(ptr, "Validation")
	vm := c.object(vp, val, shape{optional: []string{"Length", "Min", "Max", "Pattern", "Values"}, missing: req})
	if vm == nil {
		return
	}
	length, lengthOK := c.uint(vm, vp, "Length", req, maxUint32)
	limits := map[string]float64{}
	for _, k := range []string{"Min", "Max"} {
		if x, ok := vm[k]; ok {
			n, isNum := x.(json.Number)
			f, err := n.Float64()
			if !isNum || err != nil {
				c.add(join(vp, k), req, "%s must be a number", k)
				continue
			}
			limits[k] = f
		}
	}
	pattern, patternOK := c.str(vm, vp, "Pattern", req)
	var values []string
	valuesOK := false
	if x, ok := vm["Values"]; ok {
		arr, isArr := x.([]any)
		valuesOK = isArr
		for i, e := range arr {
			s, isStr := e.(string)
			if !isStr {
				c.add(join(join(vp, "Values"), strconv.Itoa(i)), req, "Values must be an array of strings")
				valuesOK = false
				continue
			}
			values = append(values, s)
		}
		if !isArr {
			c.add(join(vp, "Values"), req, "Values must be an array of strings")
		}
	}
	if !valueOK || value == "" {
		return
	}
	const vreq = "REQ-RES-CONFIG-008"
	at := join(ptr, "Value")
	if lengthOK && int64(utf8.RuneCountInString(value)) > length {
		c.add(at, vreq, "Value %q is longer than Length %d", value, length)
	}
	if typeOK && typ == "Number" && configNumberPattern.MatchString(value) {
		f, err := strconv.ParseFloat(value, 64)
		if min, ok := limits["Min"]; ok && err == nil && f < min {
			c.add(at, vreq, "Value %s is below Min %v", value, min)
		}
		if max, ok := limits["Max"]; ok && err == nil && f > max {
			c.add(at, vreq, "Value %s is above Max %v", value, max)
		}
	}
	if patternOK {
		// Patterns RE2 cannot compile (e.g. look-arounds) are not evaluated.
		if re, err := regexp.Compile(pattern); err == nil && !re.MatchString(value) {
			c.add(at, vreq, "Value %q does not match Pattern %q", value, pattern)
		}
	}
	if valuesOK && !slices.Contains(values, value) {
		c.add(at, vreq, "Value %q is not one of Values %q", value, values)
	}
}

// configValue checks the string representation of a non-empty Value or
// DefaultValue of the given Type (REQ-RES-CONFIG-007, SF-084 (d)).
func (c *checker) configValue(ptr, typ, s string) {
	if s == "" {
		return
	}
	ok := true
	switch typ {
	case "Boolean":
		ok = s == "true" || s == "false"
	case "Number":
		ok = configNumberPattern.MatchString(s)
	case "DateTime":
		_, err := time.Parse(time.RFC3339Nano, s)
		ok = err == nil
	case "ByteString":
		_, err := base64.StdEncoding.DecodeString(s)
		ok = err == nil
	}
	if !ok {
		c.add(ptr, "REQ-RES-CONFIG-007", "%q is no %s in its string representation", s, typ)
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
