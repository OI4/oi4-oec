// Package rules is the rule layer of D7 validate (ADR-0019 §5): the
// relations JSON Schema cannot express – topic values in the payload (R-2),
// relations between fields and array items (R-3) and the declared message
// role (R-6). It checks the well-formed parts of a message only; the shape
// of the message is the task of the schema layer, so a value of the wrong
// type is skipped here.
package rules

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/OI4/oec/conformance/internal/finding"
	"github.com/OI4/oec/conformance/internal/ilink"
)

// Topic holds the topic values the rules relate to.
type Topic struct {
	ServiceType string
	AppIL       string
	Method      string
	Resource    string
	SourceIL    string
	Filter      string // wire form of the Filter levels
}

// Patterns gives access to the topic patterns of the baseline.
type Patterns interface {
	Match(rule, s string) bool
}

// Roles maps a declared message role to its requirement (R-6).
var Roles = map[string]string{"birth": "REQ-LIFE-003", "close": "REQ-LIFE-007", "will": "REQ-LIFE-009"}

var (
	serviceTypes = set("Registry", "OTConnector", "Utility", "Persistence", "Aggregation", "OOCConnector", "ITConnector")
	resources    = set("MAM", "Health", "Config", "Data", "Metadata", "Event", "Profile", "PublicationList", "SubscriptionList", "ReferenceDesignation")
	// resources whose topic has no Filter: their WriterGroupName is ""
	noFilter = set("MAM", "Health", "Profile", "ReferenceDesignation")
	// resources whose list entries and WriterGroupName carry a Filter
	filteredLists = set("Data", "Metadata", "Config", "Event")
	// resources with content rules
	contentRules = set("MAM", "Health", "Profile", "PublicationList", "ReferenceDesignation", "Data", "Event", "Config", "SubscriptionList")
	helperNames  = map[int64]string{1: "PaginationRequest", 2: "Pagination", 3: "Locale", 9: "Query"}
	configTypes  = set("Boolean", "ByteString", "DateTime", "Number", "String")

	messageID    = regexp.MustCompile(`^[0-9]+-(.+)$`)
	configNumber = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)
)

const configMask = "********"

func set(s ...string) map[string]bool {
	m := map[string]bool{}
	for _, x := range s {
		m[x] = true
	}
	return m
}

type checker struct {
	t     Topic
	pat   Patterns
	out   []finding.Finding
	pub   string
	msgID string
}

func (c *checker) add(ptr, req, format string, a ...any) {
	c.out = append(c.out, finding.Finding{Req: req, Layer: finding.LayerRule, Pointer: ptr, Detail: fmt.Sprintf(format, a...)})
}

func join(ptr string, keys ...string) string {
	r := strings.NewReplacer("~", "~0", "/", "~1")
	for _, k := range keys {
		ptr += "/" + r.Replace(k)
	}
	return ptr
}

func str(m map[string]any, k string) (string, bool) {
	s, ok := m[k].(string)
	return s, ok
}

// uint returns an unsigned integer in integer syntax up to max.
func uint(m map[string]any, k string, max uint64) (int64, bool) {
	n, ok := m[k].(json.Number)
	if !ok {
		return 0, false
	}
	u, err := strconv.ParseUint(string(n), 10, 64)
	if err != nil || u > max {
		return 0, false
	}
	return int64(u), true
}

// publisherID returns "<ServiceType>/<IL>" or "" if it is malformed.
func publisherID(m map[string]any, k string) string {
	s, _ := str(m, k)
	st, il, found := strings.Cut(s, "/")
	if !found || !serviceTypes[st] || il == "" {
		return ""
	}
	return s
}

// Check runs the rule layer for a message on a topic.
func Check(t Topic, msg any, role string, pat Patterns) []finding.Finding {
	c := &checker{t: t, pat: pat}
	m, ok := msg.(map[string]any)
	if !ok {
		return nil
	}
	switch t.Method {
	case "Pub", "Get", "Set", "Del":
		if t.Resource == "Metadata" {
			c.metadata(m)
		} else {
			c.data(m)
		}
	case "Call":
		c.action(m, true)
	case "Reply":
		c.action(m, false)
	}
	if role != "" {
		c.role(m, role)
	}
	return c.out
}

// CheckMetaDataType runs the rule layer for a bare DataSetMetaDataType.
func CheckMetaDataType(v any) []finding.Finding {
	c := &checker{}
	c.metaDataType("", v)
	return c.out
}

func (c *checker) messageIDOf(m map[string]any, sender, req string) {
	s, ok := str(m, "MessageId")
	if !ok || sender == "" {
		return
	}
	if g := messageID.FindStringSubmatch(s); g == nil || g[1] != sender {
		c.add("/MessageId", req, "MessageId %q is not <unixTimestampInMs>-%s", s, sender)
	}
}

func (c *checker) topicPublisher(pub string) {
	if pub != "" && c.t.Method == "Pub" && pub != c.t.ServiceType+"/"+c.t.AppIL {
		c.add("/PublisherId", "REQ-MSG-006", "PublisherId %q differs from the topic (%s/%s)", pub, c.t.ServiceType, c.t.AppIL)
	}
}

type dsm struct {
	ptr         string
	id          int64
	name, group string
	payload     map[string]any
}

func (c *checker) data(m map[string]any) {
	c.pub = publisherID(m, "PublisherId")
	c.topicPublisher(c.pub)
	c.messageIDOf(m, c.pub, "REQ-MSG-007")
	c.msgID, _ = str(m, "MessageId")
	pubIL := ""
	if c.pub != "" {
		_, pubIL, _ = strings.Cut(c.pub, "/")
	}
	arr, _ := m["Messages"].([]any)
	var dsms []dsm
	for i, v := range arr {
		dm, ok := v.(map[string]any)
		if !ok {
			continue
		}
		d := dsm{ptr: fmt.Sprintf("/Messages/%d", i)}
		id, idOK := uint(dm, "DataSetWriterId", 65535)
		name, nameOK := str(dm, "DataSetWriterName")
		group, groupOK := str(dm, "WriterGroupName")
		d.id, d.name, d.group = id, name, group
		d.payload, _ = dm["Payload"].(map[string]any)
		switch {
		case !idOK || id == 0 || id >= 4 && id <= 8:
			continue
		case id <= 3 || id == 9:
			if groupOK && group != helperNames[id] {
				c.add(join(d.ptr, "WriterGroupName"), "REQ-MSG-019", "WriterGroupName of DataSetWriterId %d must be %s, got %q", id, helperNames[id], group)
			}
			if nameOK && pubIL != "" && name != pubIL {
				c.add(join(d.ptr, "DataSetWriterName"), "REQ-MSG-019", "DataSetWriterName of a helper DataSet must be the sender %q, got %q", pubIL, name)
			}
		default:
			if nameOK && c.t.SourceIL != "" && name != c.t.SourceIL {
				c.add(join(d.ptr, "DataSetWriterName"), "REQ-MSG-023", "DataSetWriterName %q differs from the topic Source %q", name, c.t.SourceIL)
			}
			if groupOK {
				c.writerGroupName(join(d.ptr, "WriterGroupName"), group)
			}
		}
		dsms = append(dsms, d)
	}
	c.contents(dsms)
}

func (c *checker) writerGroupName(ptr, group string) {
	switch {
	case c.t.Resource == "PublicationList" || c.t.Resource == "SubscriptionList":
		if rt, _, _ := strings.Cut(group, "/"); !resources[rt] {
			c.add(ptr, "REQ-MSG-025", "WriterGroupName %q of a list entry is not <ResourceType>[/<Tag>]", group)
		} else if c.t.Filter != "" && group != c.t.Filter {
			c.add(ptr, "REQ-MSG-025", "WriterGroupName %q differs from the topic Filter %q", group, c.t.Filter)
		}
	case noFilter[c.t.Resource]:
		if group != "" {
			c.add(ptr, "REQ-MSG-024", "WriterGroupName of %s must be empty, got %q", c.t.Resource, group)
		}
	case c.t.Filter != "" && group != c.t.Filter:
		c.add(ptr, "REQ-MSG-024", "WriterGroupName %q differs from the topic Filter %q", group, c.t.Filter)
	case c.t.Filter == "" && filteredLists[c.t.Resource] && !c.isFilterOf(c.t.Resource, group):
		c.add(ptr, "REQ-MSG-024", "WriterGroupName %q is no Filter of %s", group, c.t.Resource)
	}
}

func (c *checker) isFilterOf(resource, filter string) bool {
	for _, l := range strings.Split(filter, "/") {
		if !c.pat.Match("filter-level", l) {
			return false
		}
	}
	return c.isTagOf(resource, filter)
}

func (c *checker) isTagOf(resource, filter string) bool {
	if resource == "Event" {
		return c.pat.Match("event-filter", filter)
	}
	return !strings.Contains(filter, "/")
}

func (c *checker) contents(dsms []dsm) {
	var res []dsm
	for _, d := range dsms {
		switch d.id {
		case 1, 3:
		case 2:
		case 9:
			if c.t.Method == "Get" && c.t.Resource == "ReferenceDesignation" && c.t.SourceIL != "" {
				c.add(d.ptr, "REQ-RES-REFDES-013", "a Query of ReferenceDesignation is only sent with Get/ReferenceDesignation without Source")
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
	if !contentRules[c.t.Resource] || c.t.Method == "Get" {
		return
	}
	if c.t.Method == "Del" && c.t.Resource == "ReferenceDesignation" {
		return
	}
	entries := map[string]string{}
	contexts := map[string]string{}
	for _, d := range res {
		if c.t.Resource == "Config" && c.t.Method == "Pub" && d.name != "" && d.group != "" {
			key := d.name + "\x00" + d.group
			if prev, dup := contexts[key]; dup {
				c.add(join(d.ptr, "WriterGroupName"), "REQ-RES-CONFIG-015", "Filter %q repeats the Config DataSet of the same Source at %s", d.group, prev)
			}
			contexts[key] = d.ptr
		}
		if d.payload == nil {
			continue
		}
		switch c.t.Resource {
		case "MAM":
			c.mam(d)
		case "PublicationList":
			c.publicationEntry(d)
			if _, ok := d.payload["Resource"].(string); ok {
				key := fmt.Sprintf("%v|%v|%v", d.payload["Resource"], d.payload["Source"], d.payload["Filter"])
				if prev, dup := entries[key]; dup {
					c.add(d.ptr, "REQ-RES-PUBLIST-003", "Resource, Source and Filter repeat the entry at %s", prev)
				}
				entries[key] = d.ptr
			}
		case "SubscriptionList":
			c.subscriptionEntry(d)
		case "Config":
			c.config(d)
		}
	}
}

func (c *checker) pagination(d dsm, count int) {
	const req, cons = "REQ-PAG-003", "REQ-PAG-004"
	p := join(d.ptr, "Payload")
	m := d.payload
	total, totalOK := uint(m, "TotalCount", 1<<32-1)
	perPage, perPageOK := uint(m, "PerPage", 1<<32-1)
	page, pageOK := uint(m, "Page", 1<<32-1)
	hasNext, hasNextOK := m["HasNext"].(bool)
	id, idOK := str(m, "PaginationId")
	if idOK && c.pub != "" {
		if g := messageID.FindStringSubmatch(id); g == nil || g[1] != c.pub {
			c.add(join(p, "PaginationId"), req, "PaginationId %q is no MessageId of the sender %s", id, c.pub)
		}
	}
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

func (c *checker) mam(d dsm) {
	p := join(d.ptr, "Payload")
	m := d.payload
	if s, ok := str(m, "ProductInstanceUri"); ok {
		if d.name != "" && s != d.name {
			c.add(join(p, "ProductInstanceUri"), "REQ-RES-MAM-016", "ProductInstanceUri %q differs from the DataSetWriterName %q", s, d.name)
		}
		c.mamIdentifier(p, m, s)
	}
	if dc, ok := str(m, "DeviceClass"); ok && d.name != "" && d.name == c.t.AppIL {
		if st, found := strings.CutPrefix(dc, "Oi4."); found && serviceTypes[st] && st != c.t.ServiceType {
			c.add(join(p, "DeviceClass"), "REQ-RES-MAM-014", "the DeviceClass %q of the application names another ServiceType than %s of its topic", dc, c.t.ServiceType)
		}
	}
}

// mamIdentifier checks a ProductInstanceUri that is an Oi4Identifier of the
// ManufacturerUri against the MAM elements it is built from (MG 4.3.1).
func (c *checker) mamIdentifier(p string, m map[string]any, piu string) {
	mu, ok := str(m, "ManufacturerUri")
	seg := strings.Split(piu, "/")
	if !ok || mu == "" || ilink.Scheme(piu) != "" || len(seg) != 4 || seg[0] != strings.ToLower(mu) {
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
		if v, ok := str(m, k); ok {
			elems[i+1] = &element{k, v}
		}
	}
	for i, e := range elems {
		if e == nil {
			continue
		}
		s := seg[i+1]
		if e.value == "" && s == ilink.NoData || e.value != "" && (s == e.value || s == ilink.DnpEncode(e.value)) {
			continue
		}
		c.add(join(p, "ProductInstanceUri"), "REQ-RES-MAM-006", "ProductInstanceUri %q is an Oi4Identifier of the ManufacturerUri, but its element %q is not the %s %q", piu, s, e.name, e.value)
	}
}

func (c *checker) publicationEntry(d dsm) {
	const match = "REQ-RES-PUBLIST-002"
	p := join(d.ptr, "Payload")
	m := d.payload
	resource, _ := str(m, "Resource")
	source, sourceOK := str(m, "Source")
	filter, filterOK := str(m, "Filter")
	if resources[resource] {
		_, hasFilter := m["Filter"]
		switch {
		case filteredLists[resource] && !hasFilter:
			c.add(p, match, "an entry of %s needs a Filter (its Tag)", resource)
		case !filteredLists[resource] && hasFilter:
			c.add(join(p, "Filter"), match, "an entry of %s has no Filter", resource)
		case filterOK && !c.isTagOf(resource, filter):
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
}

func (c *checker) subscriptionEntry(d dsm) {
	const req = "REQ-RES-SUBLIST-002"
	p := join(d.ptr, "Payload", "TopicPath")
	path, ok := str(d.payload, "TopicPath")
	if !ok {
		return
	}
	lv := strings.Split(path, "/")
	if lv[0] != "Oi4v2" && lv[0] != "+" && lv[0] != "#" {
		return
	}
	if len(lv) < 5 || slices.Contains(lv[:5], "#") || strings.ContainsAny(lv[4], "+#") {
		c.add(p, req, "the resource level of TopicPath %q is not concrete", path)
		return
	}
	if d.group == "" {
		return
	}
	want, filter := lv[4], lv[min(len(lv), 6):]
	wild := len(lv) > 5 && lv[5] == "#"
	for _, l := range filter {
		wild = wild || strings.ContainsAny(l, "+#")
	}
	gp := join(d.ptr, "WriterGroupName")
	if wild {
		if rt, _, _ := strings.Cut(d.group, "/"); rt != want {
			c.add(gp, req, "WriterGroupName %q does not start with the resource %s of the TopicPath", d.group, want)
		}
		return
	}
	if len(filter) > 0 {
		want += "/" + strings.Join(filter, "/")
	}
	if d.group != want {
		c.add(gp, req, "WriterGroupName %q is not %q derived from the TopicPath", d.group, want)
	}
}

func (c *checker) config(d dsm) {
	p := join(d.ptr, "Payload")
	for k, g := range d.payload {
		gm, ok := g.(map[string]any)
		if k == "Context" || !ok {
			continue
		}
		for n, o := range gm {
			om, ok := o.(map[string]any)
			if n == "Name" || n == "Description" || !ok {
				continue
			}
			c.configValidation(join(p, k, n), om)
		}
	}
}

// configValidation checks the Value of a configuration object against its
// Validation (REQ-RES-CONFIG-008).
func (c *checker) configValidation(ptr string, m map[string]any) {
	const req = "REQ-RES-CONFIG-008"
	vm, ok := m["Validation"].(map[string]any)
	if !ok {
		return
	}
	value, ok := str(m, "Value")
	if !ok || value == "" || m["Sensitive"] == true && value == configMask {
		return
	}
	typ, _ := str(m, "Type")
	at := join(ptr, "Value")
	if length, ok := uint(vm, "Length", 1<<32-1); ok && int64(utf8.RuneCountInString(value)) > length {
		c.add(at, req, "Value %q is longer than Length %d", value, length)
	}
	if configTypes[typ] && typ == "Number" && configNumber.MatchString(value) {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			if n, ok := vm["Min"].(json.Number); ok {
				if min, err := n.Float64(); err == nil && f < min {
					c.add(at, req, "Value %s is below Min %v", value, min)
				}
			}
			if n, ok := vm["Max"].(json.Number); ok {
				if max, err := n.Float64(); err == nil && f > max {
					c.add(at, req, "Value %s is above Max %v", value, max)
				}
			}
		}
	}
	if pattern, ok := str(vm, "Pattern"); ok {
		if re, err := regexp.Compile(pattern); err == nil && !re.MatchString(value) {
			c.add(at, req, "Value %q does not match Pattern %q", value, pattern)
		}
	}
	if arr, ok := vm["Values"].([]any); ok {
		var values []string
		for _, e := range arr {
			s, ok := e.(string)
			if !ok {
				return
			}
			values = append(values, s)
		}
		if !slices.Contains(values, value) {
			c.add(at, req, "Value %q is not one of Values %q", value, values)
		}
	}
}

func (c *checker) metadata(m map[string]any) {
	pub := publisherID(m, "PublisherId")
	c.topicPublisher(pub)
	c.messageIDOf(m, pub, "REQ-MSG-007")
	if name, ok := str(m, "DataSetWriterName"); ok && c.t.SourceIL != "" && name != c.t.SourceIL {
		c.add("/DataSetWriterName", "REQ-MSG-023", "DataSetWriterName %q differs from the topic Source %q", name, c.t.SourceIL)
	}
	if group, ok := str(m, "WriterGroupName"); ok && c.t.Filter != "" && group != c.t.Filter {
		c.add("/WriterGroupName", "REQ-MSG-024", "WriterGroupName %q differs from the topic Filter %q", group, c.t.Filter)
	}
	md, ok := m["MetaData"].(map[string]any)
	if !ok || c.t.Method == "Get" || len(md) == 0 {
		return
	}
	c.metaDataType("/MetaData", md)
}

// metaDataType checks the unique names of the fields of a
// DataSetMetaDataType and of its structure and enumeration types.
func (c *checker) metaDataType(ptr string, v any) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	c.uniqueNames(join(ptr, "Fields"), m["Fields"], "REQ-MSG-038", false)
	for i, e := range list(m["StructureDataTypes"]) {
		sd, _ := e.(map[string]any)
		def, _ := sd["StructureDefinition"].(map[string]any)
		c.uniqueNames(join(ptr, "StructureDataTypes", strconv.Itoa(i), "StructureDefinition", "Fields"), def["Fields"], "REQ-MSG-043", false)
	}
	for i, e := range list(m["EnumDataTypes"]) {
		ed, _ := e.(map[string]any)
		def, _ := ed["EnumDefinition"].(map[string]any)
		c.uniqueNames(join(ptr, "EnumDataTypes", strconv.Itoa(i), "EnumDefinition", "Fields"), def["Fields"], "REQ-MSG-044", true)
	}
}

func list(v any) []any {
	a, _ := v.([]any)
	return a
}

func (c *checker) uniqueNames(ptr string, v any, req string, withEmpty bool) {
	names := map[string]bool{}
	for i, e := range list(v) {
		f, _ := e.(map[string]any)
		name, ok := str(f, "Name")
		if !ok || name == "" && !withEmpty {
			continue
		}
		if names[name] {
			c.add(join(ptr, strconv.Itoa(i), "Name"), req, "field name %q is not unique", name)
		}
		names[name] = true
	}
}

func (c *checker) action(m map[string]any, request bool) {
	pub := publisherID(m, "PublisherId")
	if pub != "" && pub != c.t.ServiceType+"/"+c.t.AppIL {
		c.add("/PublisherId", "REQ-ACT-005", "PublisherId %q is not the Responder of the topic (%s/%s)", pub, c.t.ServiceType, c.t.AppIL)
	}
	sender := pub
	if request {
		sender = publisherID(m, "RequestorId")
	}
	c.messageIDOf(m, sender, "REQ-ACT-004")
	if !request {
		return
	}
	if s, ok := str(m, "ResponseAddress"); ok {
		want := "Oi4v2/" + c.t.ServiceType + "/" + ilink.Encode(c.t.AppIL) + "/Reply/" + c.t.Resource
		if c.t.SourceIL != "" {
			want += "/" + ilink.Encode(c.t.SourceIL)
		}
		if s != want {
			c.add("/ResponseAddress", "REQ-ACT-006", "ResponseAddress %q is not the Reply topic %q", s, want)
		}
	}
	ids := map[int64]bool{}
	for i, e := range list(m["Messages"]) {
		am, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if id, ok := uint(am, "RequestId", 65535); ok {
			if ids[id] {
				c.add(fmt.Sprintf("/Messages/%d/RequestId", i), "REQ-ACT-011", "RequestId %d is not unique in the request", id)
			}
			ids[id] = true
		}
	}
}

// role checks the single-message rules of a declared role (R-6).
func (c *checker) role(m map[string]any, role string) {
	req := Roles[role]
	resource := "Health"
	if role == "birth" {
		resource = "MAM"
	}
	if c.t.Method != "Pub" || c.t.Resource != resource {
		c.add("", req, "a %s message is a Pub %s, not a %s %s", role, resource, c.t.Method, c.t.Resource)
		return
	}
	dsms, ok := m["Messages"].([]any)
	if !ok {
		return
	}
	if len(dsms) != 1 {
		c.add("/Messages", req, "a %s message has exactly one DataSetMessage, not %d", role, len(dsms))
		return
	}
	dm, _ := dsms[0].(map[string]any)
	p, ok := dm["Payload"].(map[string]any)
	if !ok {
		return
	}
	const ptr = "/Messages/0/Payload"
	switch role {
	case "birth":
		if s, ok := str(p, "ProductInstanceUri"); ok && s != c.t.AppIL {
			c.add(join(ptr, "ProductInstanceUri"), req, "the Birth message carries the MAM of %q, not of the component itself %q", s, c.t.AppIL)
		}
	case "close":
		c.roleHealth(ptr, p, req, "NORMAL_0")
	case "will":
		c.roleHealth(ptr, p, req, "FAILURE_1")
	}
}

func (c *checker) roleHealth(ptr string, p map[string]any, req, health string) {
	if h, _ := str(p, "Health"); h != health {
		c.add(join(ptr, "Health"), req, "Health is %v, the message role requires %s", p["Health"], health)
	}
	n, ok := p["HealthScore"].(json.Number)
	if !ok {
		c.add(join(ptr, "HealthScore"), req, "HealthScore is missing or no number, the message role requires 0")
		return
	}
	if f, err := n.Float64(); err != nil || f != 0 {
		c.add(join(ptr, "HealthScore"), req, "HealthScore is %s, the message role requires 0", n)
	}
}
