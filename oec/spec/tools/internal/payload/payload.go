// Package payload holds the reference checks of the Message Bus payload
// (areas MSG and ACT, MG 8): the envelope of ua-data, ua-metadata,
// ua-action-request and ua-action-response against the topic it is published
// on. It is spec tooling (ADR-0018), not an SDK: it recomputes the expected
// result of payload test vectors and reports every finding with the REQ id
// it violates. Two validation modes exist (MG 8, SF-052): Strict reports
// every key that is not defined for an object (REQ-MSG-001, REQ-ACT-003 for
// Action messages; conformance of the sender), Tolerant ignores such keys and
// checks only the defined keys (REQ-MSG-046, view of a tolerant receiver).
// CheckMetaDataType checks a bare DataSetMetaDataType, the predefined
// MetaData of a resource (MG B2). The content of the DataSets (MG 8.3, content.go) is
// checked for the helper DataSets PaginationRequest, Pagination, Locale and
// Query (areas PAG, LOC, QRY, RES-REFDES), the resources MAM, Health,
// Profile, PublicationList and ReferenceDesignation (areas RES-*, C-027) and
// Data (Oi4Pv), Metadata (empty Get), Event and SubscriptionList (C-028)
// and Config (C-029). The data type descriptions of the MetaData
// (MG 8.2.9 … 8.2.15) are checked since C-035.
package payload

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Topic is the decomposition of the topic a message is published on.
type Topic struct {
	ServiceType string
	AppIL       string // decoded AppId
	Method      string
	Resource    string // resource or service
	SourceIL    string // decoded Source, "" if absent
	Filter      string // wire form of the Filter, "" if absent
}

// Problem is one deviation of a message from the requirements.
type Problem struct {
	Pointer string // JSON Pointer (RFC 6901) of the offending value, "" for the message
	Req     string // violated requirement
	Msg     string
}

func (p Problem) String() string {
	at := p.Pointer
	if at == "" {
		at = "/"
	}
	return fmt.Sprintf("%s: %s (%s)", at, p.Msg, p.Req)
}

// ClassIDs are the predefined DataSetClassIds of the resources (MG A2).
var ClassIDs = map[string]string{
	"MAM":                  "360ca8f3-5e66-42a2-8f10-9cdf45f4bf58",
	"Health":               "d8e7b6df-42ba-448a-975a-199f59e8ffeb",
	"Config":               "9d5983db-440d-4474-9fd7-1cd7a6c8b6c2",
	"Event":                "543ae05e-b6d9-4161-a0a3-350a0fac5976",
	"Profile":              "48017c6a-05c8-48d7-9d85-4b08bbb707f3",
	"PublicationList":      "217434d6-6e1e-4230-b907-f52bc9ffe152",
	"SubscriptionList":     "e5d68c47-c276-4929-8ab9-4c1090cac785",
	"ReferenceDesignation": "27a75019-164a-496d-a38b-90e8a55c2cfa",
}

// ReservedClassIDs must never be used (MG A2 Reserved IDs).
var ReservedClassIDs = map[string]bool{
	"2ae0505e-2830-4980-b65e-0bbdf08e2d45": true, "a6e6c727-4057-419f-b2ea-3fe9173e71cf": true,
	"ebd12d4b-da1c-4671-ab86-db102fecc603": true, "96d22d73-bce6-42d3-9949-45e0d04e4d54": true,
	"32c0a57a-24fd-4378-8eb3-4a42a481bf56": true, "0721c712-f7b5-4ad0-b2a0-58f0a5744963": true,
	"d08ade63-777b-464a-8f58-73649dc3ec1c": true, "965ab782-4669-480e-bab4-c6a3b14ee4cf": true,
	"d2ee7674-cad5-4b59-a37c-3631bb0cd409": true, "a9031581-4c03-4c6a-9c12-dd7fe0123ef5": true,
	"04cac81f-9056-4af3-a8a0-686fc499b5f9": true, "a04f4f5f-b2cc-40d7-9a6a-0884233ccce7": true,
	"2b967128-5b7e-4c0f-a6eb-c59af1118ce6": true, "54b8a5c2-812d-442e-8a91-257af7304161": true,
	"2b4e35a2-437a-4a3d-b007-9b1c27b08127": true, "97439c90-7d10-47b8-8ada-6913a466107d": true,
	"4292bf9f-b9d0-4de1-a33e-f4736356d4c1": true, "0b3b07af-9cf9-4b40-977f-75b5b02bb0d5": true,
	"31c6e1c0-88c7-453b-ae9d-85fb6bc50275": true, "3b4a62ba-026f-4ee8-bc99-3a5f85fc9f3b": true,
	"760abda2-ba40-4e6e-863a-eea8c002b4e4": true, "414e26f6-341b-43b7-90fc-bb9e0b1b0866": true,
	"3b423a40-a676-4ba0-8017-f0b2cd65bc26": true, "cfa96d87-e65c-42f4-9d9e-026797c37f4c": true,
	"0c959a0f-ec83-43c1-9ccc-20ab20a0e171": true, "bb725543-9de1-4f22-883f-d32c2de5dea1": true,
}

// actionDataSetWriterIDs are the reserved DataSetWriterIds of the common
// services (MG 8.4).
var actionDataSetWriterIDs = map[string]int64{"NewDataSetWriterId": 8}

// noFilter are the resources whose topic never has a Filter (MG 7.7,
// matrix/method-resource.yaml); their WriterGroupName is "".
var noFilter = map[string]bool{"MAM": true, "Health": true, "Profile": true, "ReferenceDesignation": true}

var resources = map[string]bool{
	"MAM": true, "Health": true, "Config": true, "Data": true, "Metadata": true, "Event": true,
	"Profile": true, "PublicationList": true, "SubscriptionList": true, "ReferenceDesignation": true,
}

var serviceTypes = map[string]bool{
	"Registry": true, "OTConnector": true, "Utility": true, "Persistence": true,
	"Aggregation": true, "OOCConnector": true, "ITConnector": true,
}

var helperNames = map[int64]string{1: "PaginationRequest", 2: "Pagination", 3: "Locale", 9: "Query"}

var (
	guidPattern      = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	messageIDPattern = regexp.MustCompile(`^[0-9]+-(.+)$`)
	localePattern    = regexp.MustCompile(`^[a-z]{2,3}(-[A-Z]{2,3})?$`)
	indexFormPattern = regexp.MustCompile(`^[0-9]+:`) // QualifiedName "<index>:<name>" (Part 6 5.1.12)
)

const maxUint32 = 1<<32 - 1

// Parse decodes a JSON message strictly: exactly one JSON value, no
// duplicate object keys, numbers kept as json.Number.
func Parse(data []byte) (any, error) {
	if err := checkDuplicates(data); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing data after the JSON value")
	}
	return v, nil
}

func checkDuplicates(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	type frame struct {
		obj  bool
		keys map[string]bool
		key  bool // the next token of an object is a key
	}
	var stack []*frame
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		var top *frame
		if len(stack) > 0 {
			top = stack[len(stack)-1]
		}
		if d, ok := tok.(json.Delim); ok && (d == '}' || d == ']') {
			stack = stack[:len(stack)-1]
			continue
		}
		if top != nil && top.obj {
			if top.key {
				k := tok.(string)
				if top.keys[k] {
					return fmt.Errorf("duplicate key %q", k)
				}
				top.keys[k] = true
				top.key = false
			} else {
				top.key = true
			}
		}
		if d, ok := tok.(json.Delim); ok {
			stack = append(stack, &frame{obj: d == '{', keys: map[string]bool{}, key: d == '{'})
		}
	}
}

// Resolve returns the value at a JSON Pointer (RFC 6901).
func Resolve(doc any, pointer string) (any, bool) {
	if pointer == "" {
		return doc, true
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, false
	}
	cur := doc
	for _, tok := range strings.Split(pointer[1:], "/") {
		tok = strings.NewReplacer("~1", "/", "~0", "~").Replace(tok)
		switch c := cur.(type) {
		case map[string]any:
			v, ok := c[tok]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(tok)
			if err != nil || i < 0 || i >= len(c) || (len(tok) > 1 && tok[0] == '0') {
				return nil, false
			}
			cur = c[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

// Mode selects how keys that are not defined for an object are treated
// (MG 8, SF-052).
type Mode int

const (
	// Strict reports every undefined key (conformance of the sender,
	// REQ-MSG-001).
	Strict Mode = iota
	// Tolerant ignores undefined keys and checks only the defined keys (view
	// of a tolerant receiver, REQ-MSG-046).
	Tolerant
)

// ParseMode maps "strict" (or "") and "tolerant" to a Mode.
func ParseMode(s string) (Mode, error) {
	switch s {
	case "", "strict":
		return Strict, nil
	case "tolerant":
		return Tolerant, nil
	}
	return Strict, fmt.Errorf("unknown validation mode %q", s)
}

// Check validates a parsed message against the topic it is published on
// (strict mode).
func Check(t Topic, msg any) []Problem {
	return CheckMode(t, msg, Strict)
}

// CheckMetaDataType validates a bare DataSetMetaDataType (MG 8.2.4): the
// predefined MetaData of a resource (MG B2, the only bare DataSetMetaDataType
// of the guideline). The resource is named by its Name; for a resource with
// a predefined DataSetClassId (MG A2) the MetaData carries exactly that id
// (REQ-MSG-012, REQ-MSG-010).
func CheckMetaDataType(v any, mode Mode) []Problem {
	c := &checker{mode: mode}
	c.metaDataType("", v)
	c.resourceMetaData(v)
	return c.sorted()
}

// CheckMode validates a parsed message against the topic it is published on
// in the given mode.
func CheckMode(t Topic, msg any, mode Mode) []Problem {
	c := &checker{t: t, mode: mode}
	switch t.Method {
	case "Pub", "Get", "Set", "Del":
		if t.Resource == "Metadata" {
			c.metadata(msg)
		} else {
			c.data(msg)
		}
	case "Call":
		c.action(msg, true)
	case "Reply":
		c.action(msg, false)
	default:
		c.add("", "REQ-MSG-002", "no payload rules for Method %q", t.Method)
	}
	return c.sorted()
}

type checker struct {
	t     Topic
	mode  Mode
	out   []Problem
	pub   string // PublisherId of the NetworkMessage, "" if malformed
	msgID string // MessageId of the NetworkMessage, "" if malformed
}

func (c *checker) sorted() []Problem {
	sort.SliceStable(c.out, func(i, j int) bool { return c.out[i].Pointer < c.out[j].Pointer })
	return c.out
}

func (c *checker) add(ptr, req, format string, a ...any) {
	c.out = append(c.out, Problem{Pointer: ptr, Req: req, Msg: fmt.Sprintf(format, a...)})
}

func join(ptr, key string) string {
	return ptr + "/" + strings.NewReplacer("~", "~0", "/", "~1").Replace(key)
}

// shape lists the keys of one payload object and the requirements violated
// by a missing (missing) or an undefined (unknown) key.
type shape struct {
	required, optional []string
	missing, unknown   string
}

func (c *checker) object(ptr string, v any, s shape) map[string]any {
	m, ok := v.(map[string]any)
	if !ok {
		c.add(ptr, s.missing, "must be a JSON object")
		return nil
	}
	allowed := map[string]bool{}
	for _, k := range s.required {
		allowed[k] = true
		if _, ok := m[k]; !ok {
			c.add(ptr, s.missing, "missing field %s", k)
		}
	}
	for _, k := range s.optional {
		allowed[k] = true
	}
	if c.mode == Tolerant {
		return m
	}
	unknown := s.unknown
	if unknown == "" {
		unknown = "REQ-MSG-001"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !allowed[k] {
			c.add(join(ptr, k), unknown, "field %s is not defined for this object", k)
		}
	}
	return m
}

func (c *checker) str(m map[string]any, ptr, key, req string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		c.add(join(ptr, key), req, "%s must be a string", key)
		return "", false
	}
	return s, true
}

func (c *checker) uint(m map[string]any, ptr, key, req string, max uint64) (int64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	n, ok := v.(json.Number)
	if !ok {
		c.add(join(ptr, key), req, "%s must be a number", key)
		return 0, false
	}
	u, err := strconv.ParseUint(n.String(), 10, 64)
	if err != nil || u > max {
		c.add(join(ptr, key), req, "%s must be an unsigned integer <= %d, got %s", key, max, n)
		return 0, false
	}
	return int64(u), true
}

func (c *checker) dateTime(m map[string]any, ptr, key, req string) {
	s, ok := c.str(m, ptr, key, req)
	if !ok {
		return
	}
	if _, err := time.Parse(time.RFC3339Nano, s); err != nil {
		c.add(join(ptr, key), req, "%s %q is no ISO 8601 date-time", key, s)
	}
}

// publisherID checks "<ServiceType>/<IdentificationLink>" and returns the
// value and its Identification Link.
func (c *checker) publisherID(m map[string]any, key, req string) (string, string) {
	s, ok := c.str(m, "", key, req)
	if !ok {
		return "", ""
	}
	st, il, found := strings.Cut(s, "/")
	if !found || !serviceTypes[st] || il == "" {
		c.add(join("", key), req, "%s %q is not <ServiceType>/<IdentificationLink>", key, s)
		return "", ""
	}
	return s, il
}

// topicPublisher checks that the PublisherId of a Pub message is the
// application of the topic; Get, Set and Del address the receiving
// application (MG 7.3), so their PublisherId is not related to the topic.
func (c *checker) topicPublisher(pub string) {
	if pub != "" && c.t.Method == "Pub" && pub != c.t.ServiceType+"/"+c.t.AppIL {
		c.add("/PublisherId", "REQ-MSG-006", "PublisherId %q differs from the topic (%s/%s)", pub, c.t.ServiceType, c.t.AppIL)
	}
}

func (c *checker) messageID(m map[string]any, sender, req string) {
	s, ok := c.str(m, "", "MessageId", req)
	if !ok || sender == "" {
		return
	}
	if g := messageIDPattern.FindStringSubmatch(s); g == nil || g[1] != sender {
		c.add("/MessageId", req, "MessageId %q is not <unixTimestampInMs>-%s", s, sender)
	}
}

func (c *checker) replyTo(m map[string]any) {
	s, ok := c.str(m, "", "ReplyTo", "REQ-MSG-015")
	if ok && s != "" && !messageIDPattern.MatchString(s) {
		c.add("/ReplyTo", "REQ-MSG-015", "ReplyTo %q is neither empty nor a MessageId", s)
	}
}

func (c *checker) classID(m map[string]any, ptr, key, req string) (string, bool) {
	s, ok := c.str(m, ptr, key, req)
	if !ok {
		return "", false
	}
	if !guidPattern.MatchString(s) {
		c.add(join(ptr, key), req, "DataSetClassId %q is no lower-case GUID string", s)
	}
	if ReservedClassIDs[s] {
		c.add(join(ptr, key), "REQ-MSG-014", "DataSetClassId %q is reserved", s)
	}
	return s, true
}

func (c *checker) configVersion(ptr string, v any) {
	const req = "REQ-MSG-036"
	m := c.object(ptr, v, shape{required: []string{"MajorVersion", "MinorVersion"}, missing: req})
	if m == nil {
		return
	}
	c.uint(m, ptr, "MajorVersion", req, maxUint32)
	c.uint(m, ptr, "MinorVersion", req, maxUint32)
}

// --- ua-data -----------------------------------------------------------------

func (c *checker) data(msg any) {
	m := c.object("", msg, shape{
		required: []string{"MessageId", "MessageType", "PublisherId", "ReplyTo", "Messages"},
		optional: []string{"DataSetClassId", "WriterGroupName"},
		missing:  "REQ-MSG-003",
	})
	if m == nil {
		return
	}
	if _, ok := m["WriterGroupName"]; ok {
		c.add("/WriterGroupName", "REQ-MSG-003", "the NetworkMessage has no WriterGroupName")
	}
	if s, ok := c.str(m, "", "MessageType", "REQ-MSG-002"); ok && s != "ua-data" {
		c.add("/MessageType", "REQ-MSG-002", "MessageType must be ua-data, got %q", s)
	}
	pub, pubIL := c.publisherID(m, "PublisherId", "REQ-MSG-006")
	c.topicPublisher(pub)
	c.messageID(m, pub, "REQ-MSG-007")
	c.pub = pub
	c.msgID, _ = m["MessageId"].(string)
	c.replyTo(m)
	want, predefined := ClassIDs[c.t.Resource]
	if id, ok := c.classID(m, "", "DataSetClassId", "REQ-MSG-003"); ok {
		if predefined && id != want {
			c.add("/DataSetClassId", "REQ-MSG-010", "DataSetClassId of %s must be %s, got %s", c.t.Resource, want, id)
		}
	} else if predefined {
		c.add("", "REQ-MSG-011", "missing DataSetClassId %s of %s", want, c.t.Resource)
	}
	arr, ok := m["Messages"].([]any)
	if _, present := m["Messages"]; present && !ok {
		c.add("/Messages", "REQ-MSG-005", "Messages must be a JSON array")
	}
	var dsms []dsm
	for i, v := range arr {
		if d, ok := c.dataSetMessage(fmt.Sprintf("/Messages/%d", i), v, pubIL); ok {
			dsms = append(dsms, d)
		}
	}
	c.contents(dsms)
}

// dataSetMessage checks the envelope of a DataSetMessage and returns it for
// the content checks if its DataSetWriterId is valid.
func (c *checker) dataSetMessage(ptr string, v any, pubIL string) (dsm, bool) {
	m := c.object(ptr, v, shape{
		required: []string{"DataSetWriterId", "DataSetWriterName", "WriterGroupName", "Payload"},
		optional: []string{"SequenceNumber", "MetaDataVersion", "MinorVersion", "Timestamp", "Status", "MessageType", "PublisherId"},
		missing:  "REQ-MSG-016",
	})
	if m == nil {
		return dsm{}, false
	}
	if _, ok := m["PublisherId"]; ok {
		c.add(join(ptr, "PublisherId"), "REQ-MSG-016", "the DataSetMessage has no PublisherId")
	}
	id, idOK := c.uint(m, ptr, "DataSetWriterId", "REQ-MSG-016", 65535)
	name, nameOK := c.str(m, ptr, "DataSetWriterName", "REQ-MSG-016")
	group, groupOK := c.str(m, ptr, "WriterGroupName", "REQ-MSG-016")
	switch {
	case !idOK:
	case id == 0 || (id >= 4 && id <= 8):
		c.add(join(ptr, "DataSetWriterId"), "REQ-MSG-020", "DataSetWriterId %d is invalid or reserved", id)
	case id <= 3 || id == 9:
		if groupOK && group != helperNames[id] {
			c.add(join(ptr, "WriterGroupName"), "REQ-MSG-019", "WriterGroupName of DataSetWriterId %d must be %s, got %q", id, helperNames[id], group)
		}
		if nameOK && pubIL != "" && name != pubIL {
			c.add(join(ptr, "DataSetWriterName"), "REQ-MSG-019", "DataSetWriterName of a helper DataSet must be the sender %q, got %q", pubIL, name)
		}
	default:
		if nameOK && c.t.SourceIL != "" && name != c.t.SourceIL {
			c.add(join(ptr, "DataSetWriterName"), "REQ-MSG-023", "DataSetWriterName %q differs from the topic Source %q", name, c.t.SourceIL)
		}
		if groupOK {
			c.writerGroupName(join(ptr, "WriterGroupName"), group)
		}
	}
	payload, isObj := m["Payload"].(map[string]any)
	if _, ok := m["Payload"]; ok && !isObj {
		c.add(join(ptr, "Payload"), "REQ-MSG-016", "Payload must be a JSON object")
	}
	c.uint(m, ptr, "SequenceNumber", "REQ-MSG-016", maxUint32)
	if _, ok := m["SequenceNumber"]; ok && (c.t.Method == "Set" || c.t.Method == "Del") {
		c.add(join(ptr, "SequenceNumber"), "REQ-BUS-010", "a DataSetMessage in a %s request carries no SequenceNumber", c.t.Method)
	}
	c.dateTime(m, ptr, "Timestamp", "REQ-MSG-016")
	_, hasMDV := m["MetaDataVersion"]
	_, hasMinor := m["MinorVersion"]
	if hasMDV && hasMinor {
		c.add(ptr, "REQ-MSG-018", "MetaDataVersion and MinorVersion must not be combined")
	}
	if hasMDV {
		c.configVersion(join(ptr, "MetaDataVersion"), m["MetaDataVersion"])
	}
	c.uint(m, ptr, "MinorVersion", "REQ-MSG-016", maxUint32)
	if st, ok := m["Status"]; ok {
		sp := join(ptr, "Status")
		if sm := c.object(sp, st, shape{required: []string{"Code"}, optional: []string{"Symbol"}, missing: "REQ-MSG-027"}); sm != nil {
			if code, ok := c.uint(sm, sp, "Code", "REQ-MSG-027", maxUint32); ok && code == 0 {
				c.add(join(sp, "Code"), "REQ-MSG-026", "a Good Status is not sent")
			}
			c.str(sm, sp, "Symbol", "REQ-MSG-027")
		}
	}
	if s, ok := c.str(m, ptr, "MessageType", "REQ-MSG-029"); ok {
		want := "ua-keyframe"
		if c.t.Resource == "Event" {
			want = "ua-event"
		}
		if s != want {
			c.add(join(ptr, "MessageType"), "REQ-MSG-029", "MessageType of %s must be %s, got %q", c.t.Resource, want, s)
		}
		if s == "ua-deltaframe" || s == "ua-keepalive" {
			c.add(join(ptr, "MessageType"), "REQ-MSG-030", "a DataSetMessage is a complete key frame, %s is not used", s)
		}
	}
	if !idOK || id == 0 || (id >= 4 && id <= 8) {
		return dsm{}, false
	}
	return dsm{ptr: ptr, id: id, name: name, group: group, payload: payload}, true
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
	case c.t.Filter == "" && filteredLists[c.t.Resource] && !isFilterOf(c.t.Resource, group):
		// published without Filter: the WriterGroupName is the Filter that
		// addresses this DataSet (MG 8.2.3), a topic-encoded Tag of the resource
		c.add(ptr, "REQ-MSG-024", "WriterGroupName %q is no Filter of %s", group, c.t.Resource)
	}
}

// isFilterOf reports whether filter is a topic-encoded Filter of the resource
// (list-filter of topic.abnf): every level a filter-level, the levels a Tag.
func isFilterOf(resource, filter string) bool {
	for _, l := range strings.Split(filter, "/") {
		if !filterLevelPattern.MatchString(l) {
			return false
		}
	}
	return isTagOf(resource, filter)
}

// --- ua-metadata -------------------------------------------------------------

func (c *checker) metadata(msg any) {
	m := c.object("", msg, shape{
		required: []string{"MessageId", "MessageType", "PublisherId", "DataSetWriterId",
			"WriterGroupName", "DataSetWriterName", "Timestamp", "ReplyTo", "MetaData"},
		missing: "REQ-MSG-032",
	})
	if m == nil {
		return
	}
	if s, ok := c.str(m, "", "MessageType", "REQ-MSG-002"); ok && s != "ua-metadata" {
		c.add("/MessageType", "REQ-MSG-002", "MessageType must be ua-metadata, got %q", s)
	}
	pub, _ := c.publisherID(m, "PublisherId", "REQ-MSG-006")
	c.topicPublisher(pub)
	c.messageID(m, pub, "REQ-MSG-007")
	c.replyTo(m)
	c.dateTime(m, "", "Timestamp", "REQ-MSG-032")
	if id, ok := c.uint(m, "", "DataSetWriterId", "REQ-MSG-032", 65535); ok && id < 10 {
		c.add("/DataSetWriterId", "REQ-MSG-020", "DataSetWriterId %d is invalid or reserved", id)
	}
	if name, ok := c.str(m, "", "DataSetWriterName", "REQ-MSG-032"); ok && c.t.SourceIL != "" && name != c.t.SourceIL {
		c.add("/DataSetWriterName", "REQ-MSG-023", "DataSetWriterName %q differs from the topic Source %q", name, c.t.SourceIL)
	}
	if group, ok := c.str(m, "", "WriterGroupName", "REQ-MSG-032"); ok && c.t.Filter != "" && group != c.t.Filter {
		c.add("/WriterGroupName", "REQ-MSG-024", "WriterGroupName %q differs from the topic Filter %q", group, c.t.Filter)
	}
	if md, ok := m["MetaData"]; ok {
		obj, isObj := md.(map[string]any)
		empty := isObj && len(obj) == 0
		switch {
		case c.t.Method == "Get":
			// A Get carries an empty DataSetMetaData (MG 9.1.5).
			if !empty {
				c.add("/MetaData", "REQ-RES-META-001", "MetaData of a Get must be the empty object {}")
			}
		case empty && c.t.Method == "Pub":
			c.add("/MetaData", "REQ-RES-META-003", "MetaData of a Pub must be the filled DataSetMetaData, got {}")
		case empty && c.t.Method == "Set":
			c.add("/MetaData", "REQ-RES-META-004", "MetaData of a Set must be the filled DataSetMetaData, got {}")
		default:
			c.metaDataType("/MetaData", md)
		}
	}
}

func (c *checker) metaDataType(ptr string, v any) {
	const req = "REQ-MSG-035"
	m := c.object(ptr, v, shape{
		required: []string{"Name", "Namespaces", "StructureDataTypes", "EnumDataTypes", "SimpleDataTypes"},
		optional: []string{"Description", "Fields", "DataSetClassId", "ConfigurationVersion"},
		missing:  req,
	})
	if m == nil {
		return
	}
	c.str(m, ptr, "Name", req)
	if d, ok := m["Description"]; ok {
		c.localizedText(join(ptr, "Description"), d)
	}
	c.classID(m, ptr, "DataSetClassId", req)
	if cv, ok := m["ConfigurationVersion"]; ok {
		c.configVersion(join(ptr, "ConfigurationVersion"), cv)
	}
	for _, k := range []string{"Namespaces", "StructureDataTypes", "EnumDataTypes", "SimpleDataTypes"} {
		v, ok := m[k]
		if !ok {
			continue
		}
		arr, isArr := v.([]any)
		if !isArr {
			c.add(join(ptr, k), req, "%s must be a JSON array", k)
		}
		for i, e := range arr {
			ep := fmt.Sprintf("%s/%d", join(ptr, k), i)
			switch k {
			case "Namespaces":
				if _, ok := e.(string); !ok {
					c.add(ep, req, "a namespace is a string")
				}
			case "StructureDataTypes":
				c.structureDescription(ep, e)
			case "EnumDataTypes":
				c.enumDescription(ep, e)
			case "SimpleDataTypes":
				c.simpleTypeDescription(ep, e)
			}
		}
	}
	f, ok := m["Fields"]
	if !ok {
		return
	}
	arr, isArr := f.([]any)
	if !isArr {
		c.add(join(ptr, "Fields"), req, "Fields must be a JSON array")
	}
	names := map[string]bool{}
	for i, e := range arr {
		fp := fmt.Sprintf("%s/Fields/%d", ptr, i)
		if name := c.fieldMetaData(fp, e); name != "" {
			if names[name] {
				c.add(join(fp, "Name"), "REQ-MSG-038", "field name %q is not unique", name)
			}
			names[name] = true
		}
	}
}

func (c *checker) fieldMetaData(ptr string, v any) string {
	const req = "REQ-MSG-037"
	m := c.object(ptr, v, shape{
		required: []string{"Name", "Description", "FieldFlags", "BuiltInType", "DataType", "ValueRank",
			"ArrayDimensions", "MaxStringLength", "DataSetFieldId", "Properties"},
		missing: req,
	})
	if m == nil {
		return ""
	}
	name, _ := c.str(m, ptr, "Name", req)
	if d, ok := m["Description"]; ok {
		c.localizedText(join(ptr, "Description"), d)
	}
	c.uint(m, ptr, "FieldFlags", req, 65535)
	c.uint(m, ptr, "BuiltInType", req, 25)
	c.nodeIDField(m, ptr, "DataType", req)
	if v, ok := m["ValueRank"]; ok {
		n, isNum := v.(json.Number)
		if _, err := strconv.ParseInt(string(n), 10, 32); !isNum || err != nil {
			c.add(join(ptr, "ValueRank"), req, "ValueRank must be an Int32")
		}
	}
	c.arrayDimensions(m, ptr, req)
	c.uint(m, ptr, "MaxStringLength", req, maxUint32)
	if s, ok := c.str(m, ptr, "DataSetFieldId", req); ok && !guidPattern.MatchString(s) {
		c.add(join(ptr, "DataSetFieldId"), req, "DataSetFieldId %q is no GUID string", s)
	}
	if v, ok := m["Properties"]; ok {
		arr, isArr := v.([]any)
		if !isArr {
			c.add(join(ptr, "Properties"), req, "Properties must be a JSON array")
		}
		for i, p := range arr {
			c.keyValuePair(fmt.Sprintf("%s/Properties/%d", ptr, i), p)
		}
	}
	return name
}

func (c *checker) arrayDimensions(m map[string]any, ptr, req string) {
	v, ok := m["ArrayDimensions"]
	if !ok {
		return
	}
	arr, isArr := v.([]any)
	if !isArr {
		c.add(join(ptr, "ArrayDimensions"), req, "ArrayDimensions must be a JSON array")
	}
	for i, d := range arr {
		n, isNum := d.(json.Number)
		if _, err := strconv.ParseUint(string(n), 10, 32); !isNum || err != nil {
			c.add(fmt.Sprintf("%s/ArrayDimensions/%d", ptr, i), req, "a dimension is a UInt32")
		}
	}
}

// nodeIDField checks a NodeId key: a JSON string (req of the object) in the
// format of REQ-MSG-039.
func (c *checker) nodeIDField(m map[string]any, ptr, key, req string) {
	if s, ok := c.str(m, ptr, key, req); ok {
		c.nodeID(join(ptr, key), s)
	}
}

// qualifiedName checks a QualifiedName key: a JSON string, not empty and not
// in the namespace index form "<index>:<name>" (Part 6 5.1.12, REQ-MSG-041).
func (c *checker) qualifiedName(m map[string]any, ptr, key, req string) {
	if s, ok := c.str(m, ptr, key, req); ok && (s == "" || indexFormPattern.MatchString(s)) {
		c.add(join(ptr, key), req, "%s %q is no QualifiedName string (plain name or nsu=<uri>;<name>)", key, s)
	}
}

// structureTypes are the values of EnumStructureType (MG 8.2.10).
var structureTypes = map[string]bool{
	"Structure_0": true, "StructureWithOptionalFields_1": true, "Union_2": true,
	"StructureWithSubtypedValues_3": true, "UnionWithSubtypedValues_4": true,
}

// enumBuiltInTypes are the BuiltInTypes of an EnumDescription (MG 8.2.12):
// Int32 for enumerations, unsigned integers, ByteString or ExtensionObject
// for OptionSets.
var enumBuiltInTypes = map[int64]bool{6: true, 3: true, 5: true, 7: true, 9: true, 15: true, 22: true}

// int64Text is an Int64 encoded as JSON string (Part 6 5.4.2.3).
var int64Text = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)$`)

func (c *checker) structureDescription(ptr string, v any) {
	const req = "REQ-MSG-042"
	m := c.object(ptr, v, shape{required: []string{"DataTypeId", "Name", "StructureDefinition"}, missing: req})
	if m == nil {
		return
	}
	c.nodeIDField(m, ptr, "DataTypeId", req)
	c.qualifiedName(m, ptr, "Name", req)
	if d, ok := m["StructureDefinition"]; ok {
		c.structureDefinition(join(ptr, "StructureDefinition"), d)
	}
}

func (c *checker) structureDefinition(ptr string, v any) {
	const req = "REQ-MSG-042"
	m := c.object(ptr, v, shape{required: []string{"DefaultEncodingId", "BaseDataType", "StructureType", "Fields"}, missing: req})
	if m == nil {
		return
	}
	c.nodeIDField(m, ptr, "DefaultEncodingId", req)
	c.nodeIDField(m, ptr, "BaseDataType", req)
	if n, isNum := m["StructureType"].(json.Number); isNum {
		c.add(join(ptr, "StructureType"), "REQ-MSG-034", "StructureType %s is the numeric value of an enumeration; VerboseEncoding writes \"<Name>_<Value>\"", n)
	}
	if s, ok := c.str(m, ptr, "StructureType", req); ok && !structureTypes[s] {
		c.add(join(ptr, "StructureType"), req, "StructureType %q is no value of EnumStructureType", s)
	}
	f, ok := m["Fields"]
	if !ok {
		return
	}
	arr, isArr := f.([]any)
	if !isArr {
		c.add(join(ptr, "Fields"), req, "Fields must be a JSON array")
	}
	names := map[string]bool{}
	for i, e := range arr {
		fp := fmt.Sprintf("%s/Fields/%d", ptr, i)
		if name := c.structureField(fp, e); name != "" {
			if names[name] {
				c.add(join(fp, "Name"), "REQ-MSG-043", "field name %q is not unique in the StructureDefinition", name)
			}
			names[name] = true
		}
	}
}

func (c *checker) structureField(ptr string, v any) string {
	const req = "REQ-MSG-043"
	m := c.object(ptr, v, shape{
		required: []string{"Name", "Description", "DataType", "ValueRank", "ArrayDimensions", "MaxStringLength", "IsOptional"},
		missing:  req,
	})
	if m == nil {
		return ""
	}
	name, _ := c.str(m, ptr, "Name", req)
	if d, ok := m["Description"]; ok {
		c.localizedText(join(ptr, "Description"), d)
	}
	c.nodeIDField(m, ptr, "DataType", req)
	if v, ok := m["ValueRank"]; ok {
		n, isNum := v.(json.Number)
		r, err := strconv.ParseInt(string(n), 10, 32)
		if !isNum || err != nil || (r != -1 && r < 1) {
			c.add(join(ptr, "ValueRank"), req, "ValueRank must be -1 (scalar) or >= 1 (fixed rank array), got %v", v)
		}
	}
	c.arrayDimensions(m, ptr, req)
	c.uint(m, ptr, "MaxStringLength", req, maxUint32)
	if v, ok := m["IsOptional"]; ok {
		if _, isBool := v.(bool); !isBool {
			c.add(join(ptr, "IsOptional"), req, "IsOptional must be a Boolean")
		}
	}
	return name
}

func (c *checker) enumDescription(ptr string, v any) {
	const req = "REQ-MSG-044"
	m := c.object(ptr, v, shape{required: []string{"DataTypeId", "Name", "EnumDefinition", "BuiltInType"}, missing: req})
	if m == nil {
		return
	}
	c.nodeIDField(m, ptr, "DataTypeId", req)
	c.qualifiedName(m, ptr, "Name", req)
	if t, ok := c.uint(m, ptr, "BuiltInType", req, 255); ok && !enumBuiltInTypes[t] {
		c.add(join(ptr, "BuiltInType"), req, "BuiltInType %d is neither 6 (enumeration) nor 3, 5, 7, 9, 15, 22 (OptionSet)", t)
	}
	d, ok := m["EnumDefinition"]
	if !ok {
		return
	}
	dp := join(ptr, "EnumDefinition")
	def := c.object(dp, d, shape{required: []string{"Fields"}, missing: req})
	if def == nil {
		return
	}
	f, ok := def["Fields"]
	if !ok {
		return
	}
	arr, isArr := f.([]any)
	if !isArr {
		c.add(join(dp, "Fields"), req, "Fields must be a JSON array")
	}
	names := map[string]bool{}
	for i, e := range arr {
		fp := fmt.Sprintf("%s/Fields/%d", dp, i)
		fm := c.object(fp, e, shape{required: []string{"Value", "Name"}, optional: []string{"DisplayName", "Description"}, missing: req})
		if fm == nil {
			continue
		}
		if s, ok := c.str(fm, fp, "Value", req); ok {
			if _, err := strconv.ParseInt(s, 10, 64); err != nil || !int64Text.MatchString(s) {
				c.add(join(fp, "Value"), req, "Value %q is no Int64 as JSON string", s)
			}
		}
		for _, k := range []string{"DisplayName", "Description"} {
			if t, ok := fm[k]; ok {
				c.localizedText(join(fp, k), t)
			}
		}
		if name, ok := c.str(fm, fp, "Name", req); ok {
			if names[name] {
				c.add(join(fp, "Name"), req, "field name %q is not unique in the EnumDefinition", name)
			}
			names[name] = true
		}
	}
}

func (c *checker) simpleTypeDescription(ptr string, v any) {
	const req = "REQ-MSG-045"
	m := c.object(ptr, v, shape{required: []string{"BaseDataType", "BuiltInType", "DataTypeId", "Name"}, missing: req})
	if m == nil {
		return
	}
	c.nodeIDField(m, ptr, "BaseDataType", req)
	c.uint(m, ptr, "BuiltInType", req, 255)
	c.nodeIDField(m, ptr, "DataTypeId", req)
	c.qualifiedName(m, ptr, "Name", req)
}

// resourceMetaData checks the DataSetClassId of the predefined MetaData of a
// resource (MG B2, MG A2): present (REQ-MSG-012) and the one of the resource
// named by Name (REQ-MSG-010).
func (c *checker) resourceMetaData(v any) {
	m, _ := v.(map[string]any)
	name, _ := m["Name"].(string)
	want, ok := ClassIDs[name]
	if !ok {
		return
	}
	id, present := m["DataSetClassId"]
	switch {
	case !present:
		c.add("", "REQ-MSG-012", "the MetaData of %s lacks its DataSetClassId %s", name, want)
	case id != want:
		c.add("/DataSetClassId", "REQ-MSG-010", "DataSetClassId of %s must be %s, got %v", name, want, id)
	}
}

func (c *checker) keyValuePair(ptr string, v any) {
	const req = "REQ-MSG-041"
	kv := c.object(ptr, v, shape{required: []string{"Key", "Value"}, missing: req})
	if kv == nil {
		return
	}
	if k, ok := c.str(kv, ptr, "Key", req); ok && (k == "" || indexFormPattern.MatchString(k)) {
		c.add(join(ptr, "Key"), req, "Key %q is no QualifiedName string (plain name or nsu=<uri>;<name>)", k)
	}
	val, ok := kv["Value"]
	if !ok {
		return
	}
	vm, isObj := val.(map[string]any)
	if !isObj || vm["UaType"] == nil {
		c.add(join(ptr, "Value"), req, "Value must be a Variant object with UaType")
		return
	}
	if t, ok := vm["UaType"].(json.Number); ok && t.String() == "22" {
		if eo, ok := vm["Value"].(map[string]any); !ok || eo["UaTypeId"] == nil {
			c.add(join(ptr, "Value"), req, "an ExtensionObject Value must contain UaTypeId")
		}
	}
}

func (c *checker) localizedText(ptr string, v any) {
	const req = "REQ-MSG-040"
	m := c.object(ptr, v, shape{required: []string{"Locale", "Text"}, missing: req})
	if m == nil {
		return
	}
	if s, ok := c.str(m, ptr, "Locale", req); ok && !localePattern.MatchString(s) {
		c.add(join(ptr, "Locale"), req, "Locale %q is no LocaleId", s)
	}
	c.str(m, ptr, "Text", req)
}

func (c *checker) nodeID(ptr, s string) {
	switch {
	case strings.HasPrefix(s, "ns="):
		c.add(ptr, "REQ-MSG-039", "NodeId %q uses a namespace index", s)
	case strings.HasPrefix(s, "nsu="):
		if _, rest, ok := strings.Cut(s, ";"); !ok || !validIdentifier(rest) {
			c.add(ptr, "REQ-MSG-039", "NodeId %q is no NodeId string", s)
		}
	case !validIdentifier(s):
		c.add(ptr, "REQ-MSG-039", "NodeId %q is no NodeId string", s)
	}
}

func validIdentifier(s string) bool {
	t, id, ok := strings.Cut(s, "=")
	if !ok || id == "" {
		return false
	}
	switch t {
	case "i":
		_, err := strconv.ParseUint(id, 10, 32)
		return err == nil
	case "s", "g", "b":
		return true
	}
	return false
}

// --- Actions -----------------------------------------------------------------

func (c *checker) action(msg any, request bool) {
	s := shape{
		required: []string{"MessageId", "MessageType", "PublisherId", "Timestamp", "RequestorId", "Messages"},
		optional: []string{"CorrelationData", "ResponseAddress", "TimeoutHint"},
		missing:  "REQ-ACT-002",
		unknown:  "REQ-ACT-003",
	}
	if request {
		s.required = append(s.required, "ResponseAddress", "TimeoutHint")
	}
	m := c.object("", msg, s)
	if m == nil {
		return
	}
	wantType := "ua-action-response"
	if request {
		wantType = "ua-action-request"
	}
	if s, ok := c.str(m, "", "MessageType", "REQ-ACT-001"); ok && s != wantType {
		c.add("/MessageType", "REQ-ACT-001", "MessageType of %s must be %s, got %q", c.t.Method, wantType, s)
	}
	pub, _ := c.publisherID(m, "PublisherId", "REQ-ACT-005")
	if pub != "" && pub != c.t.ServiceType+"/"+c.t.AppIL {
		c.add("/PublisherId", "REQ-ACT-005", "PublisherId %q is not the Responder of the topic (%s/%s)", pub, c.t.ServiceType, c.t.AppIL)
	}
	requestor, _ := c.publisherID(m, "RequestorId", "REQ-ACT-005")
	sender := pub
	if request {
		sender = requestor
	}
	c.messageID(m, sender, "REQ-ACT-004")
	c.dateTime(m, "", "Timestamp", "REQ-ACT-002")
	if s, ok := c.str(m, "", "CorrelationData", "REQ-ACT-002"); ok {
		if _, err := base64.StdEncoding.DecodeString(s); err != nil {
			c.add("/CorrelationData", "REQ-ACT-002", "CorrelationData is no Base64 ByteString")
		}
	}
	if request {
		if s, ok := c.str(m, "", "ResponseAddress", "REQ-ACT-006"); ok {
			if want := c.replyTopic(); s != want {
				c.add("/ResponseAddress", "REQ-ACT-006", "ResponseAddress %q is not the Reply topic %q", s, want)
			}
		}
		if v, ok := m["TimeoutHint"]; ok {
			n, isNum := v.(json.Number)
			if f, err := n.Float64(); !isNum || err != nil || f <= 0 {
				c.add("/TimeoutHint", "REQ-ACT-002", "TimeoutHint must be a positive Duration")
			}
		}
	} else {
		for _, k := range []string{"ResponseAddress", "TimeoutHint"} {
			if _, ok := m[k]; ok {
				c.add(join("", k), "REQ-ACT-002", "a ua-action-response has no %s", k)
			}
		}
	}
	arr, ok := m["Messages"].([]any)
	if _, present := m["Messages"]; present && (!ok || len(arr) == 0) {
		c.add("/Messages", "REQ-ACT-002", "Messages must be a non-empty JSON array")
	}
	ids := map[int64]bool{}
	for i, e := range arr {
		p := fmt.Sprintf("/Messages/%d", i)
		if id, ok := c.actionMessage(p, e, request); ok && request {
			if ids[id] {
				c.add(join(p, "RequestId"), "REQ-ACT-011", "RequestId %d is not unique in the request", id)
			}
			ids[id] = true
		}
	}
}

// replyTopic is the Reply topic corresponding to the Call topic (REQ-ACT-006).
func (c *checker) replyTopic() string {
	t := "Oi4v2/" + c.t.ServiceType + "/" + encodeIL(c.t.AppIL) + "/Reply/" + c.t.Resource
	if c.t.SourceIL != "" {
		t += "/" + encodeIL(c.t.SourceIL)
	}
	return t
}

func encodeIL(il string) string { return base64.RawURLEncoding.EncodeToString([]byte(il)) }

func (c *checker) actionMessage(ptr string, v any, request bool) (int64, bool) {
	s := shape{
		required: []string{"DataSetWriterId", "ActionTargetId", "RequestId", "ActionState", "Payload"},
		optional: []string{"MetaDataVersion", "Timestamp"},
		missing:  "REQ-ACT-010",
		unknown:  "REQ-ACT-003",
	}
	if !request {
		s.required = append(s.required, "Status")
		s.missing = "REQ-ACT-012"
	}
	m := c.object(ptr, v, s)
	if m == nil {
		return 0, false
	}
	req := s.missing
	id, idOK := c.uint(m, ptr, "DataSetWriterId", req, 65535)
	target, targetOK := c.uint(m, ptr, "ActionTargetId", req, 65535)
	if want, ok := actionDataSetWriterIDs[c.t.Resource]; ok {
		if idOK && id != want {
			c.add(join(ptr, "DataSetWriterId"), "REQ-ACT-019", "DataSetWriterId of %s must be %d, got %d", c.t.Resource, want, id)
		}
		if targetOK && target != 1 {
			c.add(join(ptr, "ActionTargetId"), "REQ-ACT-019", "ActionTargetId of a common service must be 1, got %d", target)
		}
	}
	rid, ridOK := c.uint(m, ptr, "RequestId", req, 65535)
	if state, ok := c.uint(m, ptr, "ActionState", req, 2); ok {
		if request && state != 1 {
			c.add(join(ptr, "ActionState"), req, "ActionState of a request must be 1, got %d", state)
		}
		if !request && state == 0 {
			c.add(join(ptr, "ActionState"), req, "ActionState of a response must be 1 or 2, got 0")
		}
	}
	c.dateTime(m, ptr, "Timestamp", req)
	if mv, ok := m["MetaDataVersion"]; ok {
		c.configVersion(join(ptr, "MetaDataVersion"), mv)
	}
	payload, isObj := m["Payload"].(map[string]any)
	if _, present := m["Payload"]; present && !isObj {
		c.add(join(ptr, "Payload"), req, "Payload must be a JSON object")
	}
	bad := false
	if !request {
		if st, ok := m["Status"]; ok {
			sp := join(ptr, "Status")
			if sm := c.object(sp, st, shape{optional: []string{"Code", "Symbol"}, missing: "REQ-ACT-014", unknown: "REQ-ACT-003"}); sm != nil {
				if _, ok := sm["Symbol"]; ok {
					c.add(join(sp, "Symbol"), "REQ-ACT-014", "the Status of an ActionResponse has no Symbol")
				}
				if code, ok := c.uint(sm, sp, "Code", "REQ-ACT-014", maxUint32); ok {
					if code == 0 {
						c.add(join(sp, "Code"), "REQ-ACT-014", "Good is encoded as {}")
					}
					bad = code&0x80000000 != 0
				}
			}
		}
		if bad && len(payload) > 0 {
			c.add(join(ptr, "Payload"), "REQ-ACT-012", "the Payload of a Bad response is empty")
		}
	}
	if payload != nil && !bad && c.t.Resource == "NewDataSetWriterId" {
		c.newDataSetWriterID(join(ptr, "Payload"), payload, request)
	}
	return rid, ridOK
}

// newDataSetWriterID checks the Payload of NewDataSetWriterId (MG 8.4.1): a
// missing field or a value outside the semantics of the field violates
// REQ-ACT-021/022, a field with a wrong JSON type, a value out of range of
// its data type or a field not defined in the ActionMetaData violates the
// encoding of REQ-ACT-013 (C-036).
func (c *checker) newDataSetWriterID(ptr string, p map[string]any, request bool) {
	const enc = "REQ-ACT-013"
	if request {
		const req = "REQ-ACT-021"
		pm := c.object(ptr, p, shape{required: []string{"Resource"}, missing: req, unknown: enc})
		if s, ok := c.str(pm, ptr, "Resource", enc); ok && !resources[s] {
			c.add(join(ptr, "Resource"), req, "Resource %q is no ResourceType", s)
		}
		return
	}
	const req = "REQ-ACT-022"
	pm := c.object(ptr, p, shape{required: []string{"DataSetWriterId", "Ttl"}, missing: req, unknown: enc})
	if id, ok := c.uint(pm, ptr, "DataSetWriterId", enc, 65535); ok && id < 10 {
		c.add(join(ptr, "DataSetWriterId"), req, "the provided DataSetWriterId %d is reserved", id)
	}
	c.uint(pm, ptr, "Ttl", enc, maxUint32)
}
