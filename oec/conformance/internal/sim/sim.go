// Package sim is the simulated component under test of oec-check test
// (DESIGN.md §10): an OEC application that answers Get requests from a
// content model built from the reviewed payload vectors, and that can
// carry one fault per catalog test to prove that the test fails. It is
// used by tests only.
package sim

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/testvec"
)

// Fault is a deliberate deviation of the simulated CUT; the name starts
// with the catalog test that must detect it.
type Fault string

// Faults of the simulated CUT.
const (
	None          Fault = ""
	ReplyTo       Fault = "T7-GET-01/reply-to"       // answers carry an empty ReplyTo
	AllAssets     Fault = "T7-GET-01/all-assets"     // Get Health without Source omits the subordinate asset
	Unanswered    Fault = "T7-GET-02/unanswered"     // SubscriptionList is listed in the Profile but not answered
	Missing       Fault = "T7-GET-02/missing"        // Config is answered but not listed in the Profile
	EmptyList     Fault = "T7-GET-05/empty-list"     // a Get without content is not answered
	UnknownSource Fault = "T7-GET-05/unknown-source" // a Get for an unknown asset gets an empty Pub instead of an Event
	UnknownKey    Fault = "T7-GET-09/unknown-key"    // a Get with an unknown key is dropped
)

// Faults lists every fault.
var Faults = []Fault{ReplyTo, AllAssets, Unanswered, Missing, EmptyList, UnknownSource, UnknownKey}

// ServiceType, AppID and Asset identify the simulated CUT: the OT connector
// of the guideline examples (MG B1.1) with one subordinate asset.
const (
	ServiceType = "OTConnector"
	AppID       = "provider.com/FieldDataService/FDS-001/1200-0345"
	Asset       = "groe.mca/DeviceModelB/AC33.447/20443"
)

// BadNotFound is the StatusCode for an unknown asset (MG 8.3.6.1).
const BadNotFound = 0x803E0000

type model struct {
	classes  map[string]string                      // resource → DataSetClassId
	dsms     map[string]map[string][]map[string]any // resource → Source → DataSetMessages
	profiles map[string][]string                    // Source → Resources
	will     []byte
}

// known top-level keys of a NetworkMessage (MG 8.2.1).
var known = map[string]bool{"MessageId": true, "MessageType": true, "PublisherId": true, "DataSetClassId": true, "ReplyTo": true, "Messages": true}

func load() (*model, error) {
	vs, err := testvec.Load("payload")
	if err != nil {
		return nil, err
	}
	byID := map[string]map[string]any{}
	for _, v := range vs {
		var doc map[string]any
		if v.Payload != "" && json.Unmarshal([]byte(v.Payload), &doc) == nil {
			byID[v.ID] = doc
		}
	}
	m := &model{classes: map[string]string{}, dsms: map[string]map[string][]map[string]any{}, profiles: map[string][]string{}}
	add := func(res, vid string, writer float64) error {
		doc, ok := byID[vid]
		if !ok {
			return fmt.Errorf("sim: vector %s not found", vid)
		}
		if c, ok := doc["DataSetClassId"].(string); ok {
			m.classes[res] = c
		}
		list, _ := doc["Messages"].([]any)
		n := 0
		for _, x := range list {
			d, _ := x.(map[string]any)
			if id, _ := d["DataSetWriterId"].(float64); writer != 0 && id != writer {
				continue
			}
			c := map[string]any{}
			for k, v := range d {
				if k != "SequenceNumber" && k != "Timestamp" {
					c[k] = v
				}
			}
			src, _ := c["DataSetWriterName"].(string)
			if m.dsms[res] == nil {
				m.dsms[res] = map[string][]map[string]any{}
			}
			m.dsms[res][src] = append(m.dsms[res][src], c)
			n++
		}
		if n == 0 {
			return fmt.Errorf("sim: vector %s has no DataSetMessage %v", vid, writer)
		}
		return nil
	}
	for _, s := range []struct {
		res, vid string
		writer   float64
	}{
		{"MAM", "V-PAY-023", 0}, {"MAM", "V-PAY-006", 4798}, {"Health", "V-PAY-003", 0}, {"Profile", "V-PAY-032", 0},
		{"Config", "V-PAY-092", 0}, {"Data", "V-PAY-059", 0}, {"PublicationList", "V-PAY-038", 0},
		{"ReferenceDesignation", "V-PAY-043", 0}, {"SubscriptionList", "V-PAY-070", 0},
	} {
		if err := add(s.res, s.vid, s.writer); err != nil {
			return nil, err
		}
	}
	if len(m.dsms["MAM"][Asset]) != 1 || len(m.dsms["Profile"][AppID]) != 1 {
		return nil, fmt.Errorf("sim: content model incomplete")
	}
	m.dsms["Health"][Asset] = []map[string]any{{"DataSetWriterId": 3457.0, "WriterGroupName": "", "DataSetWriterName": Asset,
		"Payload": map[string]any{"Health": "NORMAL_0", "HealthScore": 100.0}}}
	m.dsms["Profile"][Asset] = []map[string]any{{"DataSetWriterId": 75.0, "WriterGroupName": "", "DataSetWriterName": Asset,
		"Payload": map[string]any{"Resources": []any{"MAM", "Health", "Profile"}}}}
	for src, ds := range m.dsms["Profile"] {
		p, _ := ds[0]["Payload"].(map[string]any)
		list, _ := p["Resources"].([]any)
		for _, r := range list {
			s, _ := r.(string)
			m.profiles[src] = append(m.profiles[src], s)
		}
	}
	will, ok := byID["V-PAY-025"]
	if !ok {
		return nil, fmt.Errorf("sim: vector V-PAY-025 (Will) not found")
	}
	if m.will, err = json.Marshal(will); err != nil {
		return nil, err
	}
	return m, nil
}

// Sim is a running simulated CUT.
type Sim struct {
	cl    *mqttwire.Client
	m     *model
	fault Fault
	mu    sync.Mutex
	last  int64
	gets  chan mqttwire.Message
	stop  chan struct{}
	done  chan struct{}
}

func enc(il string) string { return base64.RawURLEncoding.EncodeToString([]byte(il)) }

func dec(s string) (string, bool) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	return string(b), err == nil
}

func topic(method, resource, source, filter string) string {
	t := "Oi4v2/" + ServiceType + "/" + enc(AppID) + "/" + method + "/" + resource
	if source != "" {
		t += "/" + enc(source)
		if filter != "" {
			t += "/" + filter
		}
	}
	return t
}

// Start connects the simulated CUT with its Will, subscribes to its Get
// topics and publishes the Birth message and its Health.
func Start(ctx context.Context, cfg mqttwire.Config, fault Fault) (*Sim, error) {
	m, err := load()
	if err != nil {
		return nil, err
	}
	if fault == Missing {
		p, _ := m.dsms["Profile"][AppID][0]["Payload"].(map[string]any)
		var list []any
		for _, r := range p["Resources"].([]any) {
			if r != "Config" {
				list = append(list, r)
			}
		}
		m.dsms["Profile"][AppID][0]["Payload"] = map[string]any{"Resources": list}
	}
	s := &Sim{m: m, fault: fault, gets: make(chan mqttwire.Message, 256), stop: make(chan struct{}), done: make(chan struct{})}
	cfg.Will = &mqttwire.Message{Topic: topic("Pub", "Health", AppID, ""), Payload: m.will, QoS: 1}
	if cfg.ClientID == "" {
		cfg.ClientID = mqttwire.ClientID("oec-sim")
	}
	if s.cl, err = mqttwire.Dial(ctx, cfg); err != nil {
		return nil, err
	}
	// Answers are published from a worker, not from the receive callback
	// (paho delivers in order and must not block on own publications).
	go func() {
		defer close(s.done)
		for {
			select {
			case m := <-s.gets:
				s.handle(m)
			case <-s.stop:
				return
			}
		}
	}()
	receive := func(m mqttwire.Message) {
		select {
		case s.gets <- m:
		case <-s.stop:
		}
	}
	if err := s.cl.Subscribe(ctx, topic("Get", "#", "", ""), 1, receive); err != nil {
		s.Close()
		return nil, err
	}
	for _, res := range []string{"MAM", "Health"} {
		if err := s.publish(ctx, res, AppID, "", "", s.m.dsms[res][AppID]); err != nil {
			s.Close()
			return nil, err
		}
	}
	return s, nil
}

// Close stops answering, publishes the Close message and disconnects.
func (s *Sim) Close() {
	close(s.stop)
	<-s.done
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h := s.m.dsms["Health"][AppID][0]
	closing := map[string]any{}
	for k, v := range h {
		closing[k] = v
	}
	closing["Payload"] = map[string]any{"Health": "NORMAL_0", "HealthScore": 0.0}
	_ = s.publish(ctx, "Health", AppID, "", "", []map[string]any{closing})
	s.cl.Close()
}

func (s *Sim) nextID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ms := time.Now().UnixMilli()
	if ms <= s.last {
		ms = s.last + 1
	}
	s.last = ms
	return fmt.Sprintf("%d-%s/%s", ms, ServiceType, AppID)
}

func (s *Sim) publish(ctx context.Context, res, source, filter, replyTo string, dsms []map[string]any) error {
	doc := map[string]any{"MessageId": s.nextID(), "MessageType": "ua-data", "PublisherId": ServiceType + "/" + AppID, "ReplyTo": replyTo}
	if c, ok := s.m.classes[res]; ok {
		doc["DataSetClassId"] = c
	}
	if res == "Event" {
		doc["DataSetClassId"] = "543ae05e-b6d9-4161-a0a3-350a0fac5976"
	}
	list := []any{}
	for _, d := range dsms {
		list = append(list, d)
	}
	doc["Messages"] = list
	payload, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	return s.cl.Publish(ctx, mqttwire.Message{Topic: topic("Pub", res, source, filter), Payload: payload})
}

func contains(s []string, x string) bool {
	for _, e := range s {
		if e == x {
			return true
		}
	}
	return false
}

// handle answers one Get.
func (s *Sim) handle(msg mqttwire.Message) {
	lv := strings.Split(msg.Topic, "/")
	if len(lv) < 5 || lv[3] != "Get" {
		return
	}
	var doc map[string]any
	if json.Unmarshal(msg.Payload, &doc) != nil {
		return
	}
	if s.fault == UnknownKey {
		for k := range doc {
			if !known[k] {
				return
			}
		}
	}
	id, _ := doc["MessageId"].(string)
	replyTo := id
	if s.fault == ReplyTo {
		replyTo = ""
	}
	res := lv[4]
	if s.fault == Unanswered && res == "SubscriptionList" {
		return
	}
	bySource, ok := s.m.dsms[res]
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if len(lv) == 5 {
		var all []map[string]any
		for _, src := range []string{AppID, Asset} {
			if s.fault == AllAssets && res == "Health" && src == Asset {
				continue
			}
			all = append(all, bySource[src]...)
		}
		_ = s.publish(ctx, res, "", "", replyTo, all)
		return
	}
	source, ok := dec(lv[5])
	filter := strings.Join(lv[6:], "/")
	if !ok || (source != AppID && source != Asset) {
		if s.fault == UnknownSource {
			_ = s.publish(ctx, res, source, filter, replyTo, nil)
			return
		}
		ev := map[string]any{"DataSetWriterId": 623.0, "WriterGroupName": "Status/Bad", "DataSetWriterName": AppID,
			"Payload": map[string]any{"Number": float64(BadNotFound), "Description": "The requested asset is unknown.",
				"Category": "CAT_STATUS_1", "Details": map[string]any{"SymbolicId": "BadNotFound"}}}
		_ = s.publish(ctx, "Event", AppID, "Status/Bad", replyTo, []map[string]any{ev})
		return
	}
	if len(bySource[source]) == 0 && !contains(s.m.profiles[source], res) {
		return
	}
	var out []map[string]any
	for _, d := range bySource[source] {
		if g, _ := d["WriterGroupName"].(string); filter == "" || g == filter {
			out = append(out, d)
		}
	}
	if len(out) == 0 && filter != "" && s.fault == EmptyList {
		return
	}
	_ = s.publish(ctx, res, source, filter, replyTo, out)
}
