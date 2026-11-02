package sim

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The write side of the simulated CUT (C-064, MG 9.1): Set and Del change
// the content model of the run; every request gets a Status event with
// the MessageId as ReplyTo (REQ-RES-EVENT-005), a successful Set the
// publication of the changed object (REQ-BUS-006). Changed reports what a
// run left changed.

var symbols = map[int64]string{Good: "Good", BadNotWritable: "BadNotWritable", BadOutOfRange: "BadOutOfRange",
	BadNotSupported: "BadNotSupported", BadNotFound: "BadNotFound", BadTypeMismatch: "BadTypeMismatch", BadInvalidArgument: "BadInvalidArgument"}

// status publishes the Status event about a request on
// Pub/Event/<Source>/Status/<Level>.
func (s *Sim) status(ctx context.Context, source, replyTo string, code int64, text string) {
	level := "Good"
	if code != Good {
		level = "Bad"
	}
	ev := map[string]any{"DataSetWriterId": statusWriter(source, level), "WriterGroupName": "Status/" + level, "DataSetWriterName": source,
		"Payload": map[string]any{"Number": float64(code), "Description": text, "Category": "CAT_STATUS_1",
			"Details": map[string]any{"SymbolicId": symbols[code]}}}
	_ = s.publish(ctx, "Event", source, "Status/"+level, replyTo, []map[string]any{ev})
}

// statusWriter is the DataSetWriterId of the Status events of a Source
// and level: one DataSet per Source and Filter (REQ-MSG-021); 623 is the
// Status/Good of V-PAY-062.
func statusWriter(source, level string) float64 {
	id := 623.0
	switch source {
	case AppID:
	case Asset:
		id = 4802
	default: // an unknown Source (fault T7-SET-02/unknown-source)
		id = 4900
	}
	if level == "Bad" {
		id += 2
	}
	return id
}

// off reports a DataSet whose PublicationList entry has the Mode OFF_0: it
// is not accessible via Get (REQ-RES-PUBLIST-007).
func (s *Sim) off(res, source string, d map[string]any) bool {
	if res == "PublicationList" || res == "SubscriptionList" {
		return false
	}
	id, _ := d["DataSetWriterId"].(float64)
	for _, e := range s.m.dsms["PublicationList"][source] {
		p, _ := e["Payload"].(map[string]any)
		if r, _ := p["Resource"].(string); r != res {
			continue
		}
		if w, _ := p["DataSetWriterId"].(float64); w == id {
			return p["Mode"] == "OFF_0"
		}
	}
	return false
}

// messages returns the DataSetMessages of a request without helpers.
func messages(doc map[string]any) []map[string]any {
	var out []map[string]any
	list, _ := doc["Messages"].([]any)
	for _, x := range list {
		d, _ := x.(map[string]any)
		if id, _ := d["DataSetWriterId"].(float64); d != nil && id > 9 {
			out = append(out, d)
		}
	}
	return out
}

func clone(v any) any {
	b, _ := json.Marshal(v)
	var c any
	_ = json.Unmarshal(b, &c)
	return c
}

// write handles one Set or Del; lv are the topic levels.
func (s *Sim) write(lv []string, doc map[string]any) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	method, res := lv[3], lv[4]
	replyTo, _ := doc["MessageId"].(string)
	source := ""
	if len(lv) > 5 {
		src, ok := dec(lv[5])
		if !ok {
			src = lv[5]
		}
		source = src
	}
	filter := ""
	if len(lv) > 6 {
		filter = strings.Join(lv[6:], "/")
	}
	if source != AppID && source != Asset || s.listed(source, res) {
		at := AppID
		if s.fault == SetUnknown && source != "" {
			at = source
		}
		s.status(ctx, at, replyTo, BadNotFound, "The addressed asset is unknown.")
		return
	}
	var code int64
	var text string
	switch {
	case method == "Set" && res == "Config":
		code, text = s.setConfig(ctx, source, filter, replyTo, messages(doc))
	case method == "Set" && res == "Data":
		code, text = s.setData(ctx, source, filter, replyTo, messages(doc))
	case method == "Set" && res == "Metadata":
		if s.fault == SetMetadata {
			return
		}
		code, text = s.setMetadata(ctx, source, filter, replyTo, doc)
		if s.fault == MetaNoEvent {
			return
		}
	case method == "Set" && res == "PublicationList" && source == AppID:
		code, text = s.setPublication(ctx, source, filter, replyTo, messages(doc))
	case res == "SubscriptionList" && source == AppID:
		code, text = s.subscription(ctx, method, source, filter, replyTo, messages(doc))
	case method == "Set" && res == "ReferenceDesignation" && filter == "":
		code, text = s.setRefDes(ctx, source, replyTo, messages(doc))
	case method == "Del" && res == "ReferenceDesignation" && filter == "":
		code, text = s.delRefDes(ctx, source, replyTo)
		if s.fault == DelSilent {
			return
		}
	default:
		code, text = BadNotWritable, fmt.Sprintf("%s %s is not supported for this Source.", method, res)
	}
	s.status(ctx, source, replyTo, code, text)
}

// find returns the index of the DataSet of the Source with the
// WriterGroupName, or -1.
func (s *Sim) find(res, source, group string) int {
	for i, d := range s.m.dsms[res][source] {
		if g, _ := d["WriterGroupName"].(string); g == group {
			return i
		}
	}
	return -1
}

// setConfig applies a Set Config completely or not at all
// (REQ-RES-CONFIG-023, -025).
func (s *Sim) setConfig(ctx context.Context, source, filter, replyTo string, ms []map[string]any) (int64, string) {
	i := s.find("Config", source, filter)
	if i < 0 || len(ms) != 1 {
		return BadNotFound, "The Config DataSet is unknown."
	}
	cur := s.m.dsms["Config"][source][i]
	have, _ := cur["Payload"].(map[string]any)
	want, _ := ms[0]["Payload"].(map[string]any)
	if want == nil {
		return BadInvalidArgument, "The Set carries no Config DataSet."
	}
	next, _ := clone(have).(map[string]any)
	for gk, gv := range want {
		g, _ := gv.(map[string]any)
		hg, ok := have[gk].(map[string]any)
		if !ok || g == nil {
			return BadNotFound, fmt.Sprintf("The group %q is unknown.", gk)
		}
		if gk == "Context" {
			if !reflect.DeepEqual(g, hg) {
				return BadNotWritable, "The Context is read only."
			}
			continue
		}
		for on, ov := range g {
			o, _ := ov.(map[string]any)
			ho, known := hg[on].(map[string]any)
			if !known {
				return BadNotFound, fmt.Sprintf("The configuration object %q is unknown.", on)
			}
			if _, object := ho["Type"]; !object {
				if !reflect.DeepEqual(ov, hg[on]) {
					return BadNotWritable, fmt.Sprintf("The property %q of the group %q is read only.", on, gk)
				}
				continue
			}
			for pk, pv := range o {
				if pk == "Value" || (pk == "Name" && s.fault == ConfigName) {
					continue
				}
				if !reflect.DeepEqual(pv, ho[pk]) {
					return BadNotWritable, fmt.Sprintf("The property %s of %q is read only.", pk, on)
				}
			}
			v, isString := o["Value"].(string)
			if !isString {
				return BadTypeMismatch, fmt.Sprintf("The Value of %q is no string.", on)
			}
			no, _ := next[gk].(map[string]any)[on].(map[string]any)
			if ho["Sensitive"] == true && v == Mask {
				continue
			}
			if code, text := check(on, ho, v); code != Good {
				return code, text
			}
			no["Value"] = v
			if ho["Sensitive"] == true && v != "" {
				no["Value"] = Mask
			}
			if s.fault == ConfigName && o["Name"] != nil {
				no["Name"] = o["Name"]
			}
		}
		for ok, ov := range hg {
			if o, _ := ov.(map[string]any); o != nil && o["Mandatory"] == true {
				if _, there := g[ok]; !there {
					return BadInvalidArgument, fmt.Sprintf("The mandatory configuration object %q is missing.", ok)
				}
			}
		}
	}
	cur["Payload"] = next
	_ = s.publish(ctx, "Config", source, filter, replyTo, []map[string]any{cur})
	return Good, "The Config was changed."
}

// check validates a Value against Type and Validation; an empty Value
// unsets the object.
func check(name string, o map[string]any, v string) (int64, string) {
	if v == "" {
		return Good, ""
	}
	val, _ := o["Validation"].(map[string]any)
	switch o["Type"] {
	case "Number":
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return BadTypeMismatch, fmt.Sprintf("The Value of %q is no Number.", name)
		}
		if lo, ok := val["Min"].(float64); ok && f < lo {
			return BadOutOfRange, fmt.Sprintf("The Value of %q is below Min.", name)
		}
		if hi, ok := val["Max"].(float64); ok && f > hi {
			return BadOutOfRange, fmt.Sprintf("The Value of %q is above Max.", name)
		}
	case "Boolean":
		if v != "true" && v != "false" {
			return BadTypeMismatch, fmt.Sprintf("The Value of %q is no Boolean.", name)
		}
	}
	if p, ok := val["Pattern"].(string); ok {
		if re, err := regexp.Compile(p); err == nil && !re.MatchString(v) {
			return BadOutOfRange, fmt.Sprintf("The Value of %q does not match the Pattern.", name)
		}
	}
	return Good, ""
}

// setData changes the fields of a writable Data DataSet: the Data of the
// application (its Set subscription is listed).
func (s *Sim) setData(ctx context.Context, source, tag, replyTo string, ms []map[string]any) (int64, string) {
	i := s.find("Data", source, tag)
	if i < 0 {
		return BadNotFound, "The Data DataSet is unknown."
	}
	if source != AppID || len(ms) != 1 {
		return BadNotWritable, "The Data DataSet is not writable."
	}
	cur := s.m.dsms["Data"][source][i]
	have, _ := cur["Payload"].(map[string]any)
	want, _ := ms[0]["Payload"].(map[string]any)
	if len(want) == 0 {
		return BadInvalidArgument, "The Set carries no fields."
	}
	for k, v := range want {
		hv, ok := have[k]
		if !ok {
			return BadNotFound, fmt.Sprintf("The field %q is unknown.", k)
		}
		if reflect.TypeOf(hv) != reflect.TypeOf(v) {
			return BadTypeMismatch, fmt.Sprintf("The field %q has another type.", k)
		}
	}
	next, _ := clone(have).(map[string]any)
	for k, v := range want {
		next[k] = v
	}
	cur["Payload"] = next
	if s.fault != DataNoPub {
		_ = s.publish(ctx, "Data", source, tag, replyTo, []map[string]any{cur})
	}
	return Good, "The Data was changed."
}

// setMetadata replaces the DataSetMetaData of a Data DataSet.
func (s *Sim) setMetadata(ctx context.Context, source, tag, replyTo string, doc map[string]any) (int64, string) {
	cur, ok := s.m.meta[source+"|"+tag]
	if !ok {
		return BadNotFound, "The Metadata is unknown."
	}
	md, _ := doc["MetaData"].(map[string]any)
	if md == nil {
		return BadInvalidArgument, "The Set carries no MetaData."
	}
	cur["MetaData"] = clone(md)
	s.metadata(ctx, source, tag, replyTo)
	return Good, "The Metadata was changed."
}

// entryFields of a PublicationList entry and the Config that makes them
// writable (REQ-RES-PUBLIST-016).
var entryFields = map[string][]string{
	"Mode": {"MODE_1", "MODE_AND_INTERVAL_3"}, "Interval": {"INTERVAL_2", "MODE_AND_INTERVAL_3"},
	"Precisions": {"INTERVAL_2", "MODE_AND_INTERVAL_3"},
}

// setPublication changes one entry of the PublicationList.
func (s *Sim) setPublication(ctx context.Context, source, filter, replyTo string, ms []map[string]any) (int64, string) {
	i := s.find("PublicationList", source, filter)
	if i < 0 {
		return BadNotFound, "The PublicationList entry is unknown."
	}
	if len(ms) != 1 {
		return BadInvalidArgument, "A Set carries one entry."
	}
	cur := s.m.dsms["PublicationList"][source][i]
	have, _ := cur["Payload"].(map[string]any)
	want, _ := ms[0]["Payload"].(map[string]any)
	if want == nil {
		return BadInvalidArgument, "The Set carries no entry."
	}
	cfg, _ := have["Config"].(string)
	for k, v := range want {
		if reflect.DeepEqual(v, have[k]) {
			continue
		}
		allowed, writable := entryFields[k]
		if !writable {
			return BadNotWritable, fmt.Sprintf("%s is read only.", k)
		}
		if !contains(allowed, cfg) && !(k == "Mode" && s.fault == PublistMode) {
			return BadNotWritable, fmt.Sprintf("Config %s does not allow to change %s.", cfg, k)
		}
	}
	next, _ := clone(have).(map[string]any)
	for k, v := range want {
		next[k] = v
	}
	cur["Payload"] = next
	_ = s.publish(ctx, "PublicationList", source, filter, replyTo, []map[string]any{cur})
	return Good, "The PublicationList entry was changed."
}

// subscription handles Set and Del SubscriptionList (MG 9.1.9).
func (s *Sim) subscription(ctx context.Context, method, source, filter, replyTo string, ms []map[string]any) (int64, string) {
	if filter == "" || len(ms) != 1 {
		return BadInvalidArgument, "The request carries no entry."
	}
	want, _ := ms[0]["Payload"].(map[string]any)
	path, _ := want["TopicPath"].(string)
	if path == "" {
		return BadInvalidArgument, "The entry has no TopicPath."
	}
	list := s.m.dsms["SubscriptionList"][source]
	at := -1
	for i, d := range list {
		p, _ := d["Payload"].(map[string]any)
		if g, _ := d["WriterGroupName"].(string); g == filter && p["TopicPath"] == path {
			at = i
		}
	}
	if method == "Del" {
		if at < 0 {
			return BadNotFound, "The subscription is unknown."
		}
		if p, _ := list[at]["Payload"].(map[string]any); p["Config"] != "CONF_1" {
			return BadNotWritable, "The subscription cannot be deleted."
		}
		list = append(list[:at:at], list[at+1:]...)
		s.m.dsms["SubscriptionList"][source] = list
		var rest []map[string]any
		for _, d := range list {
			if g, _ := d["WriterGroupName"].(string); g == filter {
				rest = append(rest, d)
			}
		}
		if len(rest) > 0 {
			_ = s.publish(ctx, "SubscriptionList", source, filter, replyTo, rest)
		}
		return Good, "The subscription was deleted."
	}
	if at < 0 {
		if s.fault == NoCreate {
			return BadNotSupported, "Subscriptions cannot be created."
		}
		if c, ok := want["Config"]; ok && c != "CONF_1" {
			return BadNotWritable, "A created subscription is CONF_1."
		}
		e := map[string]any{"DataSetWriterId": 227.0, "WriterGroupName": filter, "DataSetWriterName": source,
			"Payload": map[string]any{"TopicPath": path, "Interval": want["Interval"], "Config": "CONF_1"}}
		if e["Payload"].(map[string]any)["Interval"] == nil {
			e["Payload"].(map[string]any)["Interval"] = 0.0
		}
		s.m.dsms["SubscriptionList"][source] = append(list, e)
		_ = s.publish(ctx, "SubscriptionList", source, filter, replyTo, []map[string]any{e})
		return Good, "The subscription was created."
	}
	cur := list[at]
	have, _ := cur["Payload"].(map[string]any)
	if have["Config"] != "CONF_1" && s.fault != SublistNone {
		return BadNotWritable, "The subscription cannot be changed."
	}
	if c, ok := want["Config"]; ok && c != have["Config"] {
		return BadNotWritable, "Config is read only."
	}
	if iv, ok := want["Interval"]; ok {
		have["Interval"] = iv
	}
	_ = s.publish(ctx, "SubscriptionList", source, filter, replyTo, []map[string]any{cur})
	return Good, "The subscription was changed."
}

// setRefDes sets (or creates) the ReferenceDesignation of an asset.
func (s *Sim) setRefDes(ctx context.Context, source, replyTo string, ms []map[string]any) (int64, string) {
	if s.fault == RefDesFixed {
		return BadNotWritable, "The ReferenceDesignation is set by the engineering tool."
	}
	if len(ms) != 1 {
		return BadInvalidArgument, "A Set carries one ReferenceDesignation."
	}
	p, _ := ms[0]["Payload"].(map[string]any)
	if p == nil {
		return BadInvalidArgument, "The Set carries no ReferenceDesignation."
	}
	// The CUT owns the DataSetWriterId (REQ-MSG-021): the one of the
	// object, else the one its PublicationList entry names, else the one
	// of the request.
	id, _ := ms[0]["DataSetWriterId"].(float64)
	for _, e := range s.m.dsms["PublicationList"][AppID] {
		ep, _ := e["Payload"].(map[string]any)
		if ep["Resource"] == "ReferenceDesignation" && ep["Source"] == source {
			id, _ = ep["DataSetWriterId"].(float64)
		}
	}
	if cur := s.m.dsms["ReferenceDesignation"][source]; len(cur) > 0 {
		id, _ = cur[0]["DataSetWriterId"].(float64)
	}
	d := map[string]any{"DataSetWriterId": id, "WriterGroupName": "", "DataSetWriterName": source, "Payload": clone(p)}
	if s.m.dsms["ReferenceDesignation"] == nil {
		s.m.dsms["ReferenceDesignation"] = map[string][]map[string]any{}
	}
	s.m.dsms["ReferenceDesignation"][source] = []map[string]any{d}
	_ = s.publish(ctx, "ReferenceDesignation", source, "", replyTo, []map[string]any{d})
	return Good, "The ReferenceDesignation was changed."
}

// delRefDes deletes the ReferenceDesignation of an asset: afterwards
// nothing is published (REQ-BUS-007).
func (s *Sim) delRefDes(ctx context.Context, source, replyTo string) (int64, string) {
	if len(s.m.dsms["ReferenceDesignation"][source]) == 0 {
		return BadNotFound, "The asset has no ReferenceDesignation."
	}
	delete(s.m.dsms["ReferenceDesignation"], source)
	if s.fault == DelPublishes {
		_ = s.publish(ctx, "ReferenceDesignation", source, "", replyTo, nil)
	}
	return Good, "The ReferenceDesignation was deleted."
}

// canonical returns the content model as sorted lines: resource, Source
// and DataSetMessage (or Metadata) as JSON; empty lists are omitted.
func canonical(m *model) []string {
	var out []string
	for res, bySource := range m.dsms {
		for src, ds := range bySource {
			for _, d := range ds {
				b, _ := json.Marshal(d)
				out = append(out, res+" "+src+" "+string(b))
			}
		}
	}
	for k, d := range m.meta {
		b, _ := json.Marshal(d["MetaData"])
		out = append(out, "Metadata "+k+" "+string(b))
	}
	sort.Strings(out)
	return out
}

// Changed compares the content model with the one the run started with:
// the lines that differ ("-" removed or changed, "+" added or changed).
// After a test run that restored the CUT, it is empty.
func (s *Sim) Changed() ([]string, error) {
	start, err := build(s.fault)
	if err != nil {
		return nil, err
	}
	s.state.Lock()
	now := canonical(s.m)
	s.state.Unlock()
	was := canonical(start)
	var out []string
	in := func(l []string, x string) bool {
		i := sort.SearchStrings(l, x)
		return i < len(l) && l[i] == x
	}
	for _, x := range was {
		if !in(now, x) {
			out = append(out, "- "+x)
		}
	}
	for _, x := range now {
		if !in(was, x) {
			out = append(out, "+ "+x)
		}
	}
	return out, nil
}
