// Package registrytest is a simulated OEC Registry (MG A1) for the self-tests
// of oec-infra bus (T8-BUS-03): it collects the MAM and Health of the assets
// on the Message Bus and answers Get MAM and Get Health with and without
// Source on its own topics (REQ-REG-001/002). A fault makes it deviate.
package registrytest

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/probe"
)

// Faults of the simulated Registry.
const (
	None         = ""
	NotCollected = "not-collected" // ignores the MAM and Health of the other assets
	NoAnswer     = "no-answer"     // answers no Get
	Publisher    = "publisher"     // answers with a PublisherId that is not its own
	WriterName   = "writer-name"   // DataSetWriterName is its own Identification Link, not the one of the asset
	StaleHealth  = "stale-health"  // keeps the first Health of an asset, not the last
	KeepsClosed  = "keeps-closed"  // ignores the Close message
	KeepsWill    = "keeps-will"    // ignores the Will
)

// StatusEvent is a conforming variant (no fault): the Registry answers a
// Get for an unknown Source with the Status Event of SF-099 under its own
// AppId (SF-108) instead of no answer.
const StatusEvent = "status-event"

// Faults lists every fault.
var Faults = []string{NotCollected, NoAnswer, Publisher, WriterName, StaleHealth, KeepsClosed, KeepsWill}

type asset struct {
	mam, health json.RawMessage
	online      bool
	writer      int
}

// Registry is a running simulated Registry.
type Registry struct {
	IL    string
	fault string
	pid   string
	cl    *mqttwire.Client

	mu     sync.Mutex
	assets map[string]*asset
	last   int64
}

// Start connects the Registry with c (its Birth and a heartbeat every hb)
// and stops it at the end of the test.
func Start(t testing.TB, c mqttwire.Config, il, fault string, hb time.Duration) *Registry {
	t.Helper()
	if fault != None && fault != StatusEvent && !slices.Contains(Faults, fault) {
		t.Fatalf("unknown registry fault %q", fault)
	}
	r := &Registry{IL: il, fault: fault, pid: "Registry/" + il, assets: map[string]*asset{}}
	ctx, cancel := context.WithCancel(context.Background())
	c.ClientID = mqttwire.ClientID("registry-")
	will := r.message("Pub", "Health", il, "", r.dsm(11, il, json.RawMessage(`{"Health":"FAILURE_1","HealthScore":0}`)))
	c.Will = &will
	cl, err := mqttwire.Dial(ctx, c)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	r.cl = cl
	for _, f := range []string{"Oi4v2/+/+/Pub/MAM/#", "Oi4v2/+/+/Pub/Health/#"} {
		if err := cl.Subscribe(ctx, f, 1, r.collect); err != nil {
			t.Fatal(err)
		}
	}
	if err := cl.Subscribe(ctx, "Oi4v2/Registry/"+enc(il)+"/Get/#", 1, func(m mqttwire.Message) {
		// Not in the handler: a Publish there waits for its PUBACK, which
		// the client reads only after the handler returned.
		go r.answer(ctx, m)
	}); err != nil {
		t.Fatal(err)
	}
	mam := fmt.Sprintf(`{"Manufacturer":{"Locale":"en-US","Text":"OI4"},"ManufacturerUri":"oi4.org","Model":{"Locale":"en-US","Text":"Registry"},"ProductCode":"SIM","HardwareRevision":"","SoftwareRevision":"2.0.0","DeviceRevision":"","DeviceManual":"https://oi4.org/oec","DeviceClass":"Oi4.Registry","SerialNumber":"1","ProductInstanceUri":%q,"RevisionCounter":1,"Description":{"Locale":"en-US","Text":"Simulated OEC Registry"}}`, il)
	birth := []mqttwire.Message{r.message("Pub", "MAM", il, "", r.dsm(10, il, json.RawMessage(mam))), r.healthMessage()}
	for _, m := range birth {
		if err := cl.Publish(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(hb)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				_ = cl.Publish(ctx, r.healthMessage())
			}
		}
	}()
	t.Cleanup(func() {
		cancel()
		<-done
		cl.Kill()
	})
	return r
}

func enc(il string) string { return base64.RawURLEncoding.EncodeToString([]byte(il)) }

func (r *Registry) healthMessage() mqttwire.Message {
	return r.message("Pub", "Health", r.IL, "", r.dsm(11, r.IL, json.RawMessage(`{"Health":"NORMAL_0","HealthScore":100}`)))
}

func (r *Registry) nextID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	ms := time.Now().UnixMilli()
	if ms <= r.last {
		ms = r.last + 1
	}
	r.last = ms
	return fmt.Sprintf("%d-%s", ms, r.pid)
}

func (r *Registry) dsm(writer int, name string, payload json.RawMessage) map[string]any {
	return map[string]any{"DataSetWriterId": writer, "WriterGroupName": "", "DataSetWriterName": name, "Payload": payload}
}

func (r *Registry) message(method, resource, source, replyTo string, dsms ...map[string]any) mqttwire.Message {
	pid := r.pid
	if r.fault == Publisher && replyTo != "" {
		pid = "Utility/" + r.IL
	}
	list := []any{}
	for _, d := range dsms {
		list = append(list, d)
	}
	raw, _ := json.Marshal(map[string]any{"MessageId": r.nextID(), "MessageType": "ua-data", "PublisherId": pid, "DataSetClassId": probe.Classes[resource], "ReplyTo": replyTo, "Messages": list})
	return mqttwire.Message{Topic: probe.Topic("Registry", r.IL, method, resource, source, ""), Payload: raw, QoS: 1}
}

type networkMessage struct {
	MessageId string
	Messages  []struct {
		DataSetWriterName string
		Payload           json.RawMessage
	}
}

func decode(m mqttwire.Message) (networkMessage, bool) {
	var nm networkMessage
	return nm, json.NewDecoder(bytes.NewReader(m.Payload)).Decode(&nm) == nil
}

// collect keeps the last MAM and Health of every asset; the Close message
// (Health NORMAL_0/0) and the Will (FAILURE_1/0) make it unavailable.
func (r *Registry) collect(m mqttwire.Message) {
	lv := strings.Split(m.Topic, "/")
	if len(lv) < 5 || r.fault == NotCollected && lv[2] != enc(r.IL) {
		return
	}
	nm, ok := decode(m)
	if !ok {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, d := range nm.Messages {
		a := r.assets[d.DataSetWriterName]
		if a == nil {
			a = &asset{writer: 100 + 2*len(r.assets)}
			r.assets[d.DataSetWriterName] = a
		}
		if lv[4] == "MAM" {
			a.mam, a.online = d.Payload, true
			continue
		}
		var h struct {
			Health      string
			HealthScore int
		}
		_ = json.Unmarshal(d.Payload, &h)
		switch {
		case h.HealthScore == 0 && h.Health == "NORMAL_0":
			a.online = a.online && r.fault == KeepsClosed
		case h.HealthScore == 0 && h.Health == "FAILURE_1":
			a.online = a.online && r.fault == KeepsWill
		}
		if a.health == nil || r.fault != StaleHealth {
			a.health = d.Payload
		}
	}
}

// answer publishes the MAM or Health of the available assets (all or the
// one of the Source) with the MessageId of the Get as ReplyTo; nothing for
// an unknown Source.
func (r *Registry) answer(ctx context.Context, m mqttwire.Message) {
	if r.fault == NoAnswer {
		return
	}
	lv := strings.Split(m.Topic, "/")
	nm, ok := decode(m)
	if len(lv) < 5 || !ok || lv[4] != "MAM" && lv[4] != "Health" {
		return
	}
	resource, source := lv[4], ""
	if len(lv) > 5 {
		raw, err := base64.RawURLEncoding.DecodeString(lv[5])
		if err != nil {
			return
		}
		source = string(raw)
	}
	r.mu.Lock()
	var dsms []map[string]any
	names := make([]string, 0, len(r.assets))
	for il := range r.assets {
		names = append(names, il)
	}
	slices.Sort(names)
	for _, il := range names {
		a := r.assets[il]
		if !a.online || source != "" && il != source {
			continue
		}
		payload, writer := a.mam, a.writer
		if resource == "Health" {
			payload, writer = a.health, a.writer+1
		}
		if payload == nil {
			continue
		}
		name := il
		if r.fault == WriterName {
			name = r.IL
		}
		dsms = append(dsms, r.dsm(writer, name, payload))
	}
	r.mu.Unlock()
	if source != "" && len(dsms) == 0 {
		if r.fault == StatusEvent {
			_ = r.cl.Publish(ctx, StatusBad(r.IL, r.nextID(), nm.MessageId))
		}
		return
	}
	_ = r.cl.Publish(ctx, r.message("Pub", resource, source, nm.MessageId, dsms...))
}

// eventClass is the DataSetClassId of Event (MG A2).
const eventClass = "543ae05e-b6d9-4161-a0a3-350a0fac5976"

// StatusBad is the Status Event of SF-099 (MG 8.3.6.1, Status Bad) for
// the Get with the MessageId replyTo of the Registry il, published under
// its AppId (SF-108) with the MessageId id.
func StatusBad(il, id, replyTo string) mqttwire.Message {
	payload := map[string]any{"Number": 2147483648, "Category": "CAT_STATUS_1", "Description": "Bad", "Details": map[string]any{"SymbolicId": "Bad"}}
	dsm := map[string]any{"DataSetWriterId": 12, "WriterGroupName": "Status/Bad", "DataSetWriterName": il, "Payload": payload}
	raw, _ := json.Marshal(map[string]any{"MessageId": id, "MessageType": "ua-data", "PublisherId": "Registry/" + il, "DataSetClassId": eventClass, "ReplyTo": replyTo, "Messages": []any{dsm}})
	return mqttwire.Message{Topic: probe.Topic("Registry", il, "Pub", "Event", il, "Status/Bad"), Payload: raw, QoS: 1}
}
