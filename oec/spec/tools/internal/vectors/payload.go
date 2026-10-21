package vectors

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/OI4/oec/spec/tools/internal/guideline"
	"github.com/OI4/oec/spec/tools/internal/payload"
	"github.com/OI4/oec/spec/tools/internal/topic"
)

// checkPayload recomputes a payload vector: the topic must be valid (a
// vector of a bare object has no topic), the payload strict JSON, and the
// reference checks in the vector's mode must report no problem (valid) or
// exactly the requirements in violates (invalid). A guideline vector must
// match its example verbatim or document every change in edits.
func checkPayload(ts *topic.Spec, v Vector, sections map[string]string, add func(string, ...any)) error {
	mode, err := payload.ParseMode(v.Mode)
	if err != nil {
		add("%v", err)
		return nil
	}
	var problems []payload.Problem
	msg, err := payload.Parse([]byte(v.Payload))
	switch {
	case v.Object != "":
		if err != nil {
			add("payload is no strict JSON: %v", err)
			return nil
		}
		problems = payload.CheckMetaDataType(msg, mode)
	default:
		res, terr := ts.Analyze(v.Topic)
		if terr != nil {
			return terr
		}
		if !res.Valid {
			add("topic is invalid (%s %s); payload vectors need a valid topic", res.Failure, res.Detail)
			return nil
		}
		if err != nil {
			add("payload is no strict JSON: %v", err)
			return nil
		}
		p := res.Parsed
		problems = payload.CheckMode(payload.Topic{
			ServiceType: p.ServiceType, AppIL: p.AppIDIL, Method: p.Method,
			Resource: p.Resource, SourceIL: p.SourceIL, Filter: p.Filter,
		}, msg, mode)
		if v.Expected == "valid" {
			checkPayloadColumns(ts, p.Method, p.Resource, msg, add)
		}
	}
	var got []string
	seen := map[string]bool{}
	for _, pr := range problems {
		if !seen[pr.Req] {
			seen[pr.Req] = true
			got = append(got, pr.Req)
		}
	}
	sort.Strings(got)
	switch {
	case v.Expected == "valid" && len(problems) > 0:
		add("payload violates %v:\n    %s", got, joinProblems(problems))
	case v.Expected == "invalid" && !equalSet(got, v.Violates):
		add("payload violates %v, vector says %v:\n    %s", got, v.Violates, joinProblems(problems))
	}
	if v.Origin.Kind == "guideline" {
		checkPayloadOrigin(v, msg, sections, add)
	}
	return nil
}

func joinProblems(ps []payload.Problem) string {
	var s []string
	for _, p := range ps {
		s = append(s, p.String())
	}
	return strings.Join(s, "\n    ")
}

func checkPayloadOrigin(v Vector, msg any, sections map[string]string, add func(string, ...any)) {
	topicEdited, pointerEdits := false, false
	for _, e := range v.Edits {
		if e.Target == "topic" {
			topicEdited = true
			if e.Kind == "removed" {
				add("edit of the topic cannot be %q", e.Kind)
			}
		} else {
			pointerEdits = true
		}
	}
	if pointerEdits != (v.Original != "") {
		add("original is required exactly if an edit has a JSON Pointer target")
	}
	var orig any
	if v.Original != "" {
		o, err := payload.Parse([]byte(relaxJSON(v.Original)))
		if err == nil {
			orig = o
			if reflect.DeepEqual(o, msg) {
				add("payload equals the original – drop original and the JSON Pointer edits")
			}
		}
	}
	for _, e := range v.Edits {
		if e.Target == "topic" {
			continue
		}
		doc, where := msg, "payload"
		if e.Kind == "removed" {
			doc, where = orig, "original"
		}
		if doc == nil {
			continue
		}
		if _, ok := payload.Resolve(doc, e.Target); !ok {
			add("edit target %s does not exist in the %s", e.Target, where)
		}
	}
	if sections == nil {
		return
	}
	sec, ok := sections[v.Origin.Ref]
	if !ok {
		add("origin %s has no guideline file", v.Origin.Ref)
		return
	}
	if v.Topic != "" && !topicEdited && !strings.Contains(sec, guideline.NormalizeForCompare(v.Topic)) {
		add("topic %q not found verbatim in %s", v.Topic, v.Origin.Ref)
	}
	text, what := v.Payload, "payload"
	if v.Original != "" {
		text, what = v.Original, "original"
	}
	if !strings.Contains(sec, guideline.NormalizeCodeForCompare(text)) {
		add("%s not found verbatim in %s", what, v.Origin.Ref)
	}
}

var trailingComma = regexp.MustCompile(`,(\s*[}\]])`)

// relaxJSON removes the illustrative parts of a guideline example that make
// it invalid JSON (// comments, trailing commas), so JSON Pointers of
// 'removed' edits can be resolved against it.
func relaxJSON(s string) string {
	var b strings.Builder
	inString, escaped := false, false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case inString:
			b.WriteByte(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
		case ch == '"':
			inString = true
			b.WriteByte(ch)
		case ch == '/' && i+1 < len(s) && s[i+1] == '/':
			for i < len(s) && s[i] != '\n' {
				i++
			}
			b.WriteByte('\n')
		default:
			b.WriteByte(ch)
		}
	}
	return trailingComma.ReplaceAllString(b.String(), "$1")
}

// helperIDs are the DataSetWriterIds of the helper DataSets (MG A3).
var helperIDs = map[string]string{"1": "PaginationRequest", "2": "Pagination", "3": "Locale", "9": "Query"}

// checkPayloadColumns cross-checks a valid payload vector against the
// payload columns of its matrix row (C-028): MessageType, number of resource
// DataSetMessages and the helper DataSets.
func checkPayloadColumns(ts *topic.Spec, method, resource string, msg any, add func(string, ...any)) {
	var cols *topic.Payload
	for _, c := range ts.Matrix.Combinations {
		if c.Method == method && c.Resource == resource && c.Allowed {
			cols = c.Payload
		}
	}
	m, ok := msg.(map[string]any)
	if cols == nil || !ok {
		return
	}
	if mt, _ := m["MessageType"].(string); mt != cols.MessageType {
		add("matrix %s %s: MessageType %q, payload column says %s", method, resource, mt, cols.MessageType)
	}
	if cols.MessageType == "ua-metadata" {
		md, _ := m["MetaData"].(map[string]any)
		if empty := md != nil && len(md) == 0; empty != (cols.Content == "metadata-empty") {
			add("matrix %s %s: MetaData empty=%v, payload column content is %s", method, resource, empty, cols.Content)
		}
		return
	}
	if cols.MessageType != "ua-data" {
		return
	}
	msgs, _ := m["Messages"].([]any)
	n := 0
	for _, e := range msgs {
		d, _ := e.(map[string]any)
		id := fmt.Sprint(d["DataSetWriterId"])
		h, isHelper := helperIDs[id]
		if !isHelper {
			n++
			continue
		}
		if !contains(cols.Helpers, h) {
			add("matrix %s %s: helper %s not in the payload column helpers %v", method, resource, h, cols.Helpers)
		}
	}
	switch {
	case cols.Content == "none" && n != 0:
		add("matrix %s %s: %d resource DataSetMessage(s), payload column content is none", method, resource, n)
	case cols.Content == "single" && n != 1:
		add("matrix %s %s: %d resource DataSetMessage(s), payload column content is single", method, resource, n)
	}
}

func contains(l []string, s string) bool {
	for _, e := range l {
		if e == s {
			return true
		}
	}
	return false
}
