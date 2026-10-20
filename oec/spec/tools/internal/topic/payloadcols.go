package topic

import (
	"fmt"

	"github.com/OI4/oec/spec/tools/internal/payload"
)

// ResourceAreas maps the resources and services of the matrix to the area
// whose requirements define their payload (C-028).
var ResourceAreas = map[string]string{
	"MAM": "RES-MAM", "Health": "RES-HEALTH", "Config": "RES-CONFIG", "Data": "RES-DATA",
	"Metadata": "RES-META", "Event": "RES-EVENT", "Profile": "RES-PROFILE",
	"PublicationList": "RES-PUBLIST", "SubscriptionList": "RES-SUBLIST",
	"ReferenceDesignation": "RES-REFDES", "NewDataSetWriterId": "ACT",
}

// PayloadOptions are the inputs of CheckPayload.
type PayloadOptions struct {
	Reqs       map[string]bool   // known requirement ids; nil skips the check
	AreaStatus map[string]string // area code -> status; nil skips the presence check
}

// CheckPayload checks the payload columns of the matrix (C-028): present on
// every allowed row unless the area of the resource is still planned, known
// requirements and consistency of message type, DataSetClassId rule, content,
// helpers and answer with the method and resource.
func (s *Spec) CheckPayload(opt PayloadOptions) []error {
	var errs []error
	for _, c := range s.Matrix.Combinations {
		where := fmt.Sprintf("%s: %s %s payload", MatrixFile, c.Method, c.Resource)
		add := func(format string, args ...any) {
			errs = append(errs, fmt.Errorf("%s: %s", where, fmt.Sprintf(format, args...)))
		}
		if !c.Allowed {
			continue
		}
		area, known := ResourceAreas[c.Resource]
		if !known {
			add("resource has no area in ResourceAreas")
			continue
		}
		p := c.Payload
		if p == nil {
			if st, ok := opt.AreaStatus[area]; opt.AreaStatus != nil && (!ok || st != "planned") {
				add("missing, but area %s is %q (not planned)", area, st)
			}
			continue
		}
		for _, r := range p.Reqs {
			if opt.Reqs != nil && !opt.Reqs[r] {
				add("unknown requirement %s", r)
			}
		}
		for _, e := range payloadConsistency(c) {
			add("%s", e)
		}
	}
	return errs
}

// payloadConsistency checks the payload columns of one allowed combination.
func payloadConsistency(c Combination) []string {
	p := c.Payload
	var out []string
	bad := func(format string, args ...any) { out = append(out, fmt.Sprintf(format, args...)) }
	switch {
	case c.Kind == "service":
		want := map[string]string{"Call": "ua-action-request", "Reply": "ua-action-response"}[c.Method]
		if p.MessageType != want {
			bad("message_type %s, want %s", p.MessageType, want)
		}
		if p.Content != "action-request" && p.Content != "action-response" || p.ClassID != "none" || p.WriterID != "action" {
			bad("an action message has content action-request/-response, class_id none and writer_id action")
		}
		if len(p.Helpers) > 0 {
			bad("an action message carries no helper DataSets")
		}
		return out
	case c.Resource == "Metadata":
		if p.MessageType != "ua-metadata" {
			bad("message_type %s, want ua-metadata", p.MessageType)
		}
		wantContent, wantClass := "metadata-filled", "in-metadata"
		if c.Method == "Get" {
			wantContent, wantClass = "metadata-empty", "none"
		}
		if p.Content != wantContent || p.ClassID != wantClass {
			bad("a %s Metadata has content %s and class_id %s", c.Method, wantContent, wantClass)
		}
		if len(p.Helpers) > 0 {
			bad("a ua-metadata message carries no helper DataSets")
		}
	default:
		if p.MessageType != "ua-data" {
			bad("message_type %s, want ua-data", p.MessageType)
		}
		_, predefined := payload.ClassIDs[c.Resource]
		switch {
		case predefined && p.ClassID != "predefined":
			bad("class_id %s, but %s has a predefined DataSetClassId (REQ-MSG-010)", p.ClassID, c.Resource)
		case !predefined && p.ClassID == "predefined":
			bad("class_id predefined, but %s has no predefined DataSetClassId (REQ-MSG-010)", c.Resource)
		}
	}
	if p.Content == "none" && c.Method != "Get" && c.Method != "Del" {
		bad("only a Get or Del carries no resource DataSet")
	}
	switch c.Method {
	case "Pub":
		if p.Content != "datasets" && p.Content != "metadata-filled" {
			bad("a Pub carries the resource DataSets, got content %s", p.Content)
		}
		if p.WriterID != "publisher" && p.WriterID != "list" {
			bad("a Pub uses the publisher's DataSetWriterIds (publisher or list), got %s", p.WriterID)
		}
		if p.Answer != "none" {
			bad("a Pub is not answered, got answer %s", p.Answer)
		}
	case "Get":
		if p.Content != "none" && p.Content != "metadata-empty" {
			bad("a Get carries no resource DataSet (content none or metadata-empty), got %s", p.Content)
		}
		if p.Answer != "pub" {
			bad("a Get is answered with a Pub, got answer %s", p.Answer)
		}
	case "Set":
		if p.Content != "single" && p.Content != "metadata-filled" {
			bad("a Set carries exactly one DataSet, got content %s", p.Content)
		}
		if p.Answer != "status-and-pub" {
			bad("a Set is answered with a Status event and a Pub, got answer %s", p.Answer)
		}
	case "Del":
		if p.Answer != "status" && p.Answer != "status-and-rest" {
			bad("a Del is answered with a Status event, got answer %s", p.Answer)
		}
		if (p.Content == "none") != (p.Answer == "status") {
			bad("a Del without DataSet deletes the whole object (answer status), a Del with one DataSet deletes an entry (answer status-and-rest)")
		}
	}
	if (p.Content == "none") != (p.WriterID == "none") && c.Kind != "service" {
		bad("writer_id none iff content none")
	}
	if p.WriterID == "list" && c.Resource != "PublicationList" && c.Resource != "SubscriptionList" {
		bad("writer_id list only for PublicationList and SubscriptionList (REQ-MSG-021)")
	}
	for _, h := range p.Helpers {
		switch h {
		case "PaginationRequest", "Locale":
			if c.Method != "Get" {
				bad("helper %s only with Get (REQ-PAG-008, REQ-LOC-002)", h)
			}
		case "Pagination":
			if c.Method != "Pub" {
				bad("helper Pagination only with Pub (REQ-PAG-008)")
			}
		case "Query":
			if c.Method != "Get" || !payload.HasQuery(c.Resource) {
				bad("helper Query only with Get of a resource with a defined Query (REQ-QRY-001)")
			}
		}
	}
	return out
}
