package bus

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/OI4/oec/conformance/internal/ilink"
	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/probe"
)

// DataSetWriterIds of the probe components (MG 8.3.1: ≥ 10).
const (
	writerMAM    = 10
	writerHealth = 11
)

// component is a probe component of T8-BUS-03: an OEC application of D8
// with Birth, Health, Close message and Will on its own connection.
type component struct {
	il  string
	pid string
	mk  *marker
	cl  *mqttwire.Client
}

func newComponent(il string) *component {
	pid := ServiceType + "/" + il
	return &component{il: il, pid: pid, mk: &marker{pid: pid}}
}

func (c *component) mam() map[string]any {
	lv := strings.SplitN(c.il, "/", 4)
	return map[string]any{
		"Manufacturer":       map[string]any{"Locale": "en-US", "Text": "OI4"},
		"ManufacturerUri":    lv[0],
		"Model":              map[string]any{"Locale": "en-US", "Text": lv[1]},
		"ProductCode":        lv[2],
		"HardwareRevision":   "",
		"SoftwareRevision":   "2.0.0",
		"DeviceRevision":     "",
		"DeviceManual":       "https://" + lv[0] + "/oec",
		"DeviceClass":        "Oi4." + ServiceType,
		"SerialNumber":       lv[3],
		"ProductInstanceUri": c.il,
		"RevisionCounter":    1,
		"Description":        map[string]any{"Locale": "en-US", "Text": "Probe component of the OEC conformance tool D8 (oec-infra bus)"},
	}
}

func health(state string, score int) map[string]any {
	return map[string]any{"Health": state, "HealthScore": score}
}

func (c *component) message(resource string, writer int, payload any) mqttwire.Message {
	doc := map[string]any{"MessageId": c.mk.next(), "MessageType": "ua-data", "PublisherId": c.pid, "DataSetClassId": probe.Classes[resource], "ReplyTo": "",
		"Messages": []any{map[string]any{"DataSetWriterId": writer, "WriterGroupName": "", "DataSetWriterName": c.il, "Payload": payload}}}
	raw, _ := json.Marshal(doc)
	return mqttwire.Message{Topic: probe.Topic(ServiceType, c.il, "Pub", resource, c.il, ""), Payload: raw, QoS: 1}
}

// start connects with the Will (Health FAILURE_1/0, MG 6.3.5) and publishes
// the Birth (MAM, Health NORMAL_0/100, MG 6.3.3) and a second Health
// NORMAL_0/99, the last Health the Registry must return.
func (c *component) start(ctx context.Context, r *runner) error {
	will := c.message("Health", writerHealth, health("FAILURE_1", 0))
	cl, err := r.dial(ctx, r.c.Main, "oec-infra-reg-", &will)
	if err != nil {
		return err
	}
	c.cl = cl
	for _, m := range []mqttwire.Message{c.message("MAM", writerMAM, c.mam()), c.message("Health", writerHealth, health("NORMAL_0", 100)), c.message("Health", writerHealth, health("NORMAL_0", 99))} {
		if err := cl.Publish(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

// close publishes the Close message (Health NORMAL_0/0, MG 6.3.4) and
// disconnects.
func (c *component) close(ctx context.Context) error {
	err := c.cl.Publish(ctx, c.message("Health", writerHealth, health("NORMAL_0", 0)))
	c.cl.Close()
	return err
}

// answer is one publication of the Registry.
type answer struct {
	topic       string
	publisherID string
	dsms        []dsm
}

type dsm struct {
	name    string
	payload any
}

// registryProbe correlates the answers of the Registry with the Gets of D8
// by their ReplyTo.
type registryProbe struct {
	r    *runner
	main *mqttwire.Client
	il   string
	mk   *marker
	pid  string
	mu   sync.Mutex
	got  map[string][]answer
	any  bool // the Registry answered at least one Get
}

func (p *registryProbe) receive(m mqttwire.Message) {
	var doc struct {
		PublisherId string
		ReplyTo     string
		Messages    []struct {
			DataSetWriterName string
			Payload           json.RawMessage
		}
	}
	if json.NewDecoder(bytes.NewReader(m.Payload)).Decode(&doc) != nil || doc.ReplyTo == "" {
		return
	}
	a := answer{topic: m.Topic, publisherID: doc.PublisherId}
	for _, d := range doc.Messages {
		var v any
		_ = json.Unmarshal(d.Payload, &v)
		a.dsms = append(a.dsms, dsm{name: d.DataSetWriterName, payload: v})
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.got[doc.ReplyTo]; ok {
		p.got[doc.ReplyTo] = append(p.got[doc.ReplyTo], a)
		p.any = true
	}
}

func (p *registryProbe) topic(method, resource, source string) string {
	return probe.Topic("Registry", p.il, method, resource, source, "")
}

// get sends one Get to the Registry and returns its MessageId.
func (p *registryProbe) get(ctx context.Context, resource, source string) (string, error) {
	id := p.mk.next()
	p.mu.Lock()
	p.got[id] = nil
	p.mu.Unlock()
	return id, p.main.Publish(ctx, mqttwire.Message{Topic: p.topic("Get", resource, source), Payload: request(id, p.pid, resource), QoS: 1})
}

func (p *registryProbe) answers(id string) []answer {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]answer(nil), p.got[id]...)
}

// problem is one finding of a round.
type problem struct {
	req, topic, detail string
}

// query is one Get of a round and what it expects.
type query struct {
	resource, source string // source "": all assets
	present          map[string]any
	absent           []string
}

// round sends the queries, waits and returns the problems and whether any
// query was answered.
func (p *registryProbe) round(ctx context.Context, wait time.Duration, qs []query, mams map[string]any) ([]problem, bool, error) {
	ids := make([]string, len(qs))
	for i, q := range qs {
		id, err := p.get(ctx, q.resource, q.source)
		if err != nil {
			return nil, false, err
		}
		ids[i] = id
	}
	select {
	case <-time.After(wait):
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
	type result struct {
		as   []answer
		dsms []dsm
	}
	res := make([]result, len(qs))
	answered, misnamed := false, false
	listed := map[string]map[string]bool{} // resource → assets in the answer to the Get without Source
	for i, q := range qs {
		res[i].as = p.answers(ids[i])
		for _, a := range res[i].as {
			res[i].dsms = append(res[i].dsms, a.dsms...)
		}
		answered = answered || len(res[i].as) > 0
		for il := range q.present {
			_, how := find(res[i].dsms, il, mams[il])
			misnamed = misnamed || how == "payload"
			if q.source == "" && how != "" {
				if listed[q.resource] == nil {
					listed[q.resource] = map[string]bool{}
				}
				listed[q.resource][il] = true
			}
		}
	}
	var out []problem
	status := p.topic("Pub", "Event", p.il) + "/Status/"
	for i, q := range qs {
		get, want := "Get "+q.resource, p.topic("Pub", q.resource, q.source)
		if q.source != "" {
			get += "/<" + q.source + ">"
		}
		if len(res[i].as) == 0 {
			switch {
			case q.source == "":
				out = append(out, problem{reqAnswer, p.topic("Get", q.resource, q.source), "no answer to " + get})
			case len(q.present) == 0:
			case answered && !listed[q.resource][q.source]:
				out = append(out, problem{reqCollect, p.topic("Get", q.resource, q.source), fmt.Sprintf("the Registry does not return the available probe component %s (no answer to %s, not in Get %s)", q.source, get, q.resource)})
			default:
				out = append(out, problem{reqAnswer, p.topic("Get", q.resource, q.source), "no answer to " + get})
			}
			continue
		}
		for _, a := range res[i].as {
			// An absent Source is an unknown asset: the Status Event of
			// SF-099 under the AppId (SF-108) is a conforming answer.
			if a.topic != want && !(len(q.present) == 0 && strings.HasPrefix(a.topic, status)) {
				out = append(out, problem{reqAnswer, a.topic, fmt.Sprintf("the answer to %s is published on %s, not on %s", get, a.topic, want)})
			}
			if a.publisherID != "Registry/"+p.il {
				out = append(out, problem{reqAnswer, a.topic, fmt.Sprintf("the answer to %s has the PublisherId %q, not the one of the Registry %q", get, a.publisherID, "Registry/"+p.il)})
			}
		}
		for _, il := range sortedKeys(q.present) {
			d, how := find(res[i].dsms, il, mams[il])
			switch {
			case how == "name" && !reflect.DeepEqual(d.payload, q.present[il]):
				out = append(out, problem{reqAnswer, want, fmt.Sprintf("the answer to %s contains for %s the payload %s, not the last one received %s", get, il, js(d.payload), js(q.present[il]))})
			case how == "name":
			case how == "payload":
				out = append(out, problem{reqAnswer, want, fmt.Sprintf("the answer to %s contains the MAM of %s with the DataSetWriterName %q, not the Identification Link of the asset", get, il, d.name)})
			case misnamed:
				out = append(out, problem{reqAnswer, want, fmt.Sprintf("the answer to %s has no DataSetMessage with the DataSetWriterName %s", get, il)})
			default:
				out = append(out, problem{reqCollect, want, fmt.Sprintf("the answer to %s does not contain the available probe component %s", get, il)})
			}
		}
		for _, il := range q.absent {
			if _, how := find(res[i].dsms, il, mams[il]); how != "" {
				out = append(out, problem{reqAnswer, want, fmt.Sprintf("the answer to %s still contains %s after its Close message or Will", get, il)})
			}
		}
	}
	return out, answered, nil
}

// find looks for the DataSetMessage of an asset: by its DataSetWriterName
// ("name") or – for a Registry with another DataSetWriterName – by the MAM
// payload ("payload").
func find(ds []dsm, il string, mam any) (dsm, string) {
	for _, d := range ds {
		if d.name == il {
			return d, "name"
		}
	}
	for _, d := range ds {
		if mam != nil && reflect.DeepEqual(d.payload, mam) {
			return d, "payload"
		}
	}
	return dsm{}, ""
}

func js(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func sortedKeys(m map[string]any) []string {
	return slices.Sorted(maps.Keys(m))
}

// normalize returns v as decoded JSON (the form of the answers).
func normalize(v any) any {
	raw, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}

// registry is T8-BUS-03: two probe components announce themselves; the
// Registry returns their MAM and last Health on Get MAM and Get Health
// (with and without Source) and no longer returns them after the Close
// message of the first and the Will of the second.
func (r *runner) registry(ctx context.Context, main *mqttwire.Client) {
	il := r.c.Registry
	if il == "" {
		il = r.discover(ctx, main)
		if il == "" {
			r.inconclusive(TRegistry, registryReqs, "no OEC Registry published on Oi4v2/Registry/+/Pub/# within %s (--registry names one)", r.c.RegistryWait)
			return
		}
	}
	r.info.Registry = il
	p := &registryProbe{r: r, main: main, il: il, pid: ServiceType + "/" + r.probe, mk: &marker{pid: ServiceType + "/" + r.probe}, got: map[string][]answer{}}
	if err := main.Subscribe(ctx, "Oi4v2/Registry/"+ilink.Encode(il)+"/Pub/#", 1, p.receive); err != nil {
		r.inconclusive(TRegistry, registryReqs, "D8 cannot subscribe to the answers of the Registry: %v", err)
		return
	}
	closed, killed := newComponent(r.probe+"-1"), newComponent(r.probe+"-2")
	for _, c := range []*component{closed, killed} {
		if err := c.start(ctx, r); err != nil {
			r.inconclusive(TRegistry, registryReqs, "probe component %s: %v", c.il, err)
			return
		}
	}
	defer killed.cl.Kill()
	mams := map[string]any{closed.il: normalize(closed.mam()), killed.il: normalize(killed.mam())}
	last := normalize(health("NORMAL_0", 99))
	available := []query{
		{resource: "MAM", source: closed.il, present: map[string]any{closed.il: mams[closed.il]}},
		{resource: "Health", source: closed.il, present: map[string]any{closed.il: last}},
		{resource: "MAM", present: map[string]any{closed.il: mams[closed.il], killed.il: mams[killed.il]}},
		{resource: "Health", present: map[string]any{closed.il: last, killed.il: last}},
	}
	probs, answered, err := r.poll(ctx, p, available, mams)
	if err != nil {
		r.inconclusive(TRegistry, registryReqs, "%v", err)
		return
	}
	if !answered {
		r.violation(TRegistry, reqAnswer, p.topic("Get", "MAM", ""), "the Registry %s answers no Get MAM or Get Health within %s", il, 3*r.c.Timeout)
		r.inconclusive(TRegistry, []string{reqCollect}, "the Registry does not answer")
		return
	}
	if err := closed.close(ctx); err != nil {
		r.inconclusive(TRegistry, registryReqs, "Close message of %s: %v", closed.il, err)
		return
	}
	killed.cl.Kill()
	gone := []query{
		{resource: "MAM", absent: []string{closed.il, killed.il}},
		{resource: "Health", absent: []string{closed.il, killed.il}},
		{resource: "MAM", source: closed.il, absent: []string{closed.il}},
		{resource: "MAM", source: killed.il, absent: []string{killed.il}},
	}
	probs2, _, err := r.poll(ctx, p, gone, mams)
	if err != nil {
		r.inconclusive(TRegistry, registryReqs, "%v", err)
		return
	}
	probs = append(probs, probs2...)
	bad := map[string]bool{}
	for _, pr := range probs {
		bad[pr.req] = true
		r.violation(TRegistry, pr.req, pr.topic, "%s", pr.detail)
	}
	if !bad[reqCollect] {
		r.checked(TRegistry, reqCollect, "the Registry returns both probe components while they are available")
	}
	if !bad[reqAnswer] {
		r.checked(TRegistry, reqAnswer, "the Registry answers Get MAM and Get Health with and without Source")
	}
}

// poll repeats a round until it finds no problem or the time (3 timeouts)
// is over; it returns the problems of the last round.
func (r *runner) poll(ctx context.Context, p *registryProbe, qs []query, mams map[string]any) ([]problem, bool, error) {
	end := time.Now().Add(3 * r.c.Timeout)
	wait := max(r.c.Timeout/5, 200*time.Millisecond)
	answered := false
	for {
		probs, ans, err := p.round(ctx, wait, qs, mams)
		if err != nil {
			return nil, false, err
		}
		answered = answered || ans
		if len(probs) == 0 || time.Now().After(end) {
			return dedupe(probs), answered, nil
		}
	}
}

func dedupe(ps []problem) []problem {
	seen := map[problem]bool{}
	var out []problem
	for _, p := range ps {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// discover waits for the first publication of a Registry.
func (r *runner) discover(ctx context.Context, main *mqttwire.Client) string {
	found := make(chan string, 1)
	if err := main.Subscribe(ctx, "Oi4v2/Registry/+/Pub/#", 0, func(m mqttwire.Message) {
		lv := strings.Split(m.Topic, "/")
		if len(lv) < 3 {
			return
		}
		il, err := decode(lv[2])
		if err != nil {
			return
		}
		select {
		case found <- il:
		default:
		}
	}); err != nil {
		return ""
	}
	select {
	case il := <-found:
		return il
	case <-time.After(r.c.RegistryWait):
	case <-ctx.Done():
	}
	return ""
}

func decode(level string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(level)
	return string(raw), err
}
