package sim

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OI4/oec/conformance/internal/mqttwire"
)

// The Responder of the common service NewDataSetWriterId (C-065, MG 9.2.1,
// MG 8.4.1): the reserved DataSetWriterIds are kept outside the content
// model, so a run that only requests ids leaves Changed empty.

// Faults of the Action tests.
const (
	NoCorrelation  Fault = "T7-ACT-01/no-correlation"  // the response omits the CorrelationData of the request
	Progress       Fault = "T7-ACT-01/progress"        // variant, conforming: an intermediate response (ActionState 1) precedes the final one
	Reissue        Fault = "T7-ACT-02/reissue"         // every request gets the same DataSetWriterId although it is still reserved
	AnswersUnknown Fault = "T7-ACT-03/answers-unknown" // a request with an unknown DataSetWriterId or ActionTargetId is answered with BadNotFound
	NoReply        Fault = "T7-ACT-04/no-reply"        // a request for an unknown Source is not answered
)

// Action addresses of NewDataSetWriterId (REQ-ACT-019) and the reservation
// of the provided ids.
const (
	actionWriter = 8
	actionTarget = 1
	firstWriter  = 6000
	ttl          = 60
)

// call answers one ua-action-request on Call/NewDataSetWriterId[/<Source>].
func (s *Sim) call(lv []string, doc map[string]any) {
	if lv[4] != "NewDataSetWriterId" || len(lv) > 6 {
		return
	}
	source := AppID
	if len(lv) == 6 {
		src, ok := dec(lv[5])
		if !ok {
			return
		}
		source = src
	}
	reply := topic("Reply", "NewDataSetWriterId", "", "")
	if len(lv) == 6 {
		reply = topic("Reply", "NewDataSetWriterId", source, "")
	}
	if doc["MessageType"] != "ua-action-request" || doc["PublisherId"] != ServiceType+"/"+AppID || doc["ResponseAddress"] != reply {
		return
	}
	requestor, _ := doc["RequestorId"].(string)
	if requestor == "" {
		return
	}
	known := source == AppID || source == Asset
	list, _ := doc["Messages"].([]any)
	var final []any
	var progress []any
	for _, x := range list {
		ar, _ := x.(map[string]any)
		rid, ok := ar["RequestId"].(float64)
		if !ok {
			continue
		}
		addressed := ar["DataSetWriterId"] == float64(actionWriter) && ar["ActionTargetId"] == float64(actionTarget)
		resp := map[string]any{"DataSetWriterId": ar["DataSetWriterId"], "ActionTargetId": ar["ActionTargetId"], "RequestId": rid, "ActionState": 2.0}
		switch {
		case !addressed && s.fault != AnswersUnknown:
			continue
		case !addressed:
			resp["Status"], resp["Payload"] = map[string]any{"Code": float64(BadNotFound)}, map[string]any{}
		case !known && s.fault == NoReply:
			continue
		case !known:
			resp["Status"], resp["Payload"] = map[string]any{"Code": float64(BadNotFound)}, map[string]any{}
		default:
			p, _ := ar["Payload"].(map[string]any)
			if res, _ := p["Resource"].(string); res == "" {
				resp["Status"], resp["Payload"] = map[string]any{"Code": float64(BadInvalidArgument)}, map[string]any{}
				break
			}
			resp["Status"], resp["Payload"] = map[string]any{}, map[string]any{"DataSetWriterId": float64(s.reserve()), "Ttl": float64(ttl)}
		}
		if s.fault == Progress {
			// A Good response carries the response fields (REQ-ACT-012),
			// the intermediate one too.
			progress = append(progress, map[string]any{"DataSetWriterId": resp["DataSetWriterId"], "ActionTargetId": resp["ActionTargetId"],
				"RequestId": rid, "ActionState": 1.0, "Status": resp["Status"], "Payload": resp["Payload"]})
		}
		final = append(final, resp)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if len(progress) > 0 {
		s.respond(ctx, reply, requestor, doc["CorrelationData"], progress)
	}
	if len(final) > 0 {
		s.respond(ctx, reply, requestor, doc["CorrelationData"], final)
	}
}

// reserve returns a new DataSetWriterId, reserved for Ttl seconds
// (REQ-ACT-023); the fault Reissue returns the same one every time.
func (s *Sim) reserve() int {
	if s.fault == Reissue {
		return firstWriter
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id := firstWriter + s.reserved
	s.reserved++
	return id
}

// respond publishes a ua-action-response (MG 8.2.6) to the ResponseAddress.
func (s *Sim) respond(ctx context.Context, reply, requestor string, correlation any, responses []any) {
	doc := map[string]any{"MessageId": s.nextID(), "MessageType": "ua-action-response", "PublisherId": ServiceType + "/" + AppID,
		"Timestamp": time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), "RequestorId": requestor, "Messages": responses}
	if correlation != nil && s.fault != NoCorrelation {
		doc["CorrelationData"] = correlation
	}
	payload, err := json.Marshal(doc)
	if err != nil {
		return
	}
	_ = s.cl.Publish(ctx, mqttwire.Message{Topic: reply, Payload: payload})
}
