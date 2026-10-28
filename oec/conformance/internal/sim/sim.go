// Package sim is the simulated component under test of oec-check test
// (DESIGN.md §10): an OEC application that answers Get requests from a
// content model built from the reviewed payload vectors, and that can
// carry one fault per catalog test to prove that the test fails. It is
// used by tests only.
package sim

import (
	"context"
	"crypto/tls"
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
	DualVersion   Fault = "T7-ADR-02/dual-version"   // variant, conforming: also publishes its Birth under the namespace Oi4
	Mixed         Fault = "T7-ADR-02/mixed"          // DualVersion, and answers to Get MAM are repeated under Oi4
	Case          Fault = "T7-ADR-03/case"           // the Source of a Get is compared case-insensitively
	NoMetadata    Fault = "T7-GET-03/metadata"       // Get Metadata is not answered
	FilterIgnored Fault = "T7-GET-03/filter"         // the Filter of Get PublicationList is ignored
	AssetLevel    Fault = "T7-GET-04/asset-level"    // the subordinate asset does not provide Config
	PerPage       Fault = "T7-GET-06/per-page"       // a PaginationRequest is ignored
	QueryIgnored  Fault = "T7-GET-07/ignored"        // the Query of ReferenceDesignation is not supported (ignored)
	QuerySource   Fault = "T7-GET-07/with-source"    // the Query is evaluated on a Get with Source too
	NoRefDes      Fault = "T7-GET-07/empty"          // variant, conforming: ReferenceDesignation is supported, but has no object
	WrongLocale   Fault = "T7-GET-08/locale"         // a Get with a Locale is answered with LocalizedText in fr-FR
	NotListed     Fault = "T7-SET-01/sublist"        // the Set subscription of the writable Config is not listed
	SetUnknown    Fault = "T7-SET-02/unknown-source" // a Set or Del for an unknown asset gets its Status event under that Source
	SetMetadata   Fault = "T7-SET-03/metadata"       // a Set Metadata is dropped
	DelPublishes  Fault = "T7-SET-04/del-publishes"  // the Del of a ReferenceDesignation publishes an empty list
	ConfigName    Fault = "T7-SET-05/read-only"      // a Set Config may change the Name of a configuration object
	DataNoPub     Fault = "T7-SET-06/no-pub"         // a successful Set Data is not followed by the publication
	MetaNoEvent   Fault = "T7-SET-07/no-event"       // a Set Metadata gets no Status event
	PublistMode   Fault = "T7-SET-08/read-only"      // a Set PublicationList may change the Mode of a NONE_0 entry
	DelSilent     Fault = "T7-SET-09/del-silent"     // a Del ReferenceDesignation gets no Status event
	RefDesFixed   Fault = "T7-SET-09/set-rejected"   // variant, conforming: a Set ReferenceDesignation is rejected (BadNotWritable), a Del is accepted
	SublistNone   Fault = "T7-SET-11/none-writable"  // a Set SubscriptionList may change a NONE_0 entry
	NoCreate      Fault = "T7-SET-11/no-create"      // variant, conforming: a Set SubscriptionList does not create entries (BadNotSupported)
	FixedLevel    Fault = "T7-SET-12/fixed-level"    // the Syslog levels of the PublicationList are NONE_0

	// Connection and life cycle (C-066): through the proxy attachment.
	MQTT31       Fault = "T7-CON-01/mqtt31"        // CONNECT with the protocol level 3 (MQTT 3.1)
	ClientIDs    Fault = "T7-CON-01/client-id"     // a new random ClientId on every connection
	TLS11        Fault = "T7-CON-02/tls11"         // also offers TLS 1.0 and 1.1
	NoVerify     Fault = "T7-CON-02/no-verify"     // does not verify the certificate of the broker
	Anonymous    Fault = "T7-CON-03/anonymous"     // connects without user name and password (and without a client certificate)
	Retained     Fault = "T7-CON-04/retain"        // publishes its Birth with Retain
	NoWill       Fault = "T7-LIFE-01/no-will"      // registers no Will
	StaleWill    Fault = "T7-LIFE-01/stale-will"   // registers the fixed Will of MG B1.1.2 (MessageId of 2021) on every connection
	WillTime     Fault = "T7-LIFE-01/timestamp"    // the Will carries a Timestamp
	NoRebirth    Fault = "T7-LIFE-02/no-birth"     // no Birth after a reconnect
	HealthFirst  Fault = "T7-LIFE-02/health-first" // the first message of a connection is its Health, the MAM follows
	FailNoBirth  Fault = "T7-LIFE-02/no-rebirth"   // reports FAILURE_1/0 and recovers to NORMAL_0 without a new Birth
	FailRecovery Fault = "T7-LIFE-02/recovery"     // variant, conforming: reports FAILURE_1/0 and recovers with a new Birth
	WillWriter   Fault = "T7-LIFE-03/writer-id"    // the Will uses another DataSetWriterId than the Health
)

// Faults lists every fault.
var Faults = []Fault{ReplyTo, AllAssets, Unanswered, Missing, EmptyList, UnknownSource, UnknownKey,
	DualVersion, Mixed, Case, NoMetadata, FilterIgnored, AssetLevel, PerPage, QueryIgnored, QuerySource, NoRefDes, WrongLocale,
	NotListed, SetUnknown, SetMetadata, DelPublishes, ConfigName, DataNoPub, MetaNoEvent, PublistMode, DelSilent, RefDesFixed, SublistNone, NoCreate, FixedLevel,
	NoCorrelation, Progress, Reissue, AnswersUnknown, NoReply,
	MQTT31, ClientIDs, TLS11, NoVerify, Anonymous, Retained, NoWill, StaleWill, WillTime, NoRebirth, HealthFirst, FailNoBirth, FailRecovery, WillWriter}

// ServiceType, AppID and Asset identify the simulated CUT: the OT connector
// of the guideline examples (MG B1.1) with one subordinate asset.
const (
	ServiceType = "OTConnector"
	AppID       = "provider.com/FieldDataService/FDS-001/1200-0345"
	Asset       = "groe.mca/DeviceModelB/AC33.447/20443"
)

// StatusCodes of the Status events (MG 8.3.6.1).
const (
	Good               = 0
	BadNotWritable     = 0x803B0000
	BadOutOfRange      = 0x803C0000
	BadNotSupported    = 0x803D0000
	BadNotFound        = 0x803E0000
	BadTypeMismatch    = 0x80740000
	BadInvalidArgument = 0x80AB0000
)

// Mask replaces the value of a sensitive configuration object in every
// publication (REQ-RES-CONFIG-024).
const Mask = "********"

type model struct {
	classes  map[string]string                      // resource → DataSetClassId
	dsms     map[string]map[string][]map[string]any // resource → Source → DataSetMessages; "MAM/de-DE": MAM in German
	profiles map[string][]string                    // Source → Resources
	meta     map[string]map[string]any              // Source "|" Tag → Metadata NetworkMessage
	will     []byte                                 // the Will of MG B1.1.2 (V-PAY-025)
	willDoc  map[string]any                         // the same, decoded: a fresh MessageId per connection
}

// known top-level keys of a NetworkMessage (MG 8.2.1).
var known = map[string]bool{"MessageId": true, "MessageType": true, "PublisherId": true, "DataSetClassId": true, "ReplyTo": true, "Messages": true,
	// the DataSetMetaData of a Get Metadata (MessageType ua-metadata)
	"DataSetWriterId": true, "WriterGroupName": true, "DataSetWriterName": true, "Timestamp": true, "MetaData": true}

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
	m := &model{classes: map[string]string{}, dsms: map[string]map[string][]map[string]any{}, profiles: map[string][]string{}, meta: map[string]map[string]any{}}
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
		{"ReferenceDesignation", "V-PAY-043", 0}, {"ReferenceDesignation", "V-PAY-054", 4799}, {"SubscriptionList", "V-PAY-070", 0},
		{"MAM/de-DE", "V-PAY-047", 0},
	} {
		if err := add(s.res, s.vid, s.writer); err != nil {
			return nil, err
		}
	}
	// The Filter of the Config example (B1.1.3) contains a space, which
	// REQ-TOP-020 advises against; the simulated CUT uses it without the
	// space, so the answer to Get Config with Filter (T7-GET-03) has no
	// warning.
	for _, ds := range m.dsms["Config"] {
		for _, d := range ds {
			g, _ := d["WriterGroupName"].(string)
			d["WriterGroupName"] = strings.ReplaceAll(g, " ", "")
		}
	}
	// Write side (C-064): a sensitive configuration object (published
	// masked, GL-ADR-021), the configurable Syslog levels Informational and
	// Debug (REQ-TOP-027) and the Set subscriptions of the writable Data and
	// Config DataSets (REQ-RES-SUBLIST-016).
	for _, d := range m.dsms["Config"][AppID] {
		p, _ := d["Payload"].(map[string]any)
		g, _ := p["network"].(map[string]any)
		if g == nil {
			return nil, fmt.Errorf("sim: Config group network not found")
		}
		g["password"] = map[string]any{"Name": map[string]any{"Locale": "en-US", "Text": "Password"}, "Sensitive": true, "Type": "String", "Value": Mask}
	}
	for i, level := range []string{"Informational", "Debug"} {
		m.dsms["PublicationList"][AppID] = append(m.dsms["PublicationList"][AppID], map[string]any{"DataSetWriterId": 161.0,
			"WriterGroupName": "Event/Syslog/" + level, "DataSetWriterName": AppID, "Payload": map[string]any{"Resource": "Event",
				"Source": AppID, "Filter": "Syslog/" + level, "DataSetWriterId": float64(626 + i), "Mode": "OFF_0", "Interval": 0.0, "Config": "MODE_1"}})
	}
	for _, sub := range []struct{ res, tag string }{{"Data", "oee"}, {"Config", "Networksettings"}} {
		m.dsms["SubscriptionList"][AppID] = append(m.dsms["SubscriptionList"][AppID], map[string]any{"DataSetWriterId": 227.0,
			"WriterGroupName": sub.res + "/" + sub.tag, "DataSetWriterName": AppID, "Payload": map[string]any{
				"TopicPath": topic("Set", sub.res, AppID, sub.tag), "Interval": 0.0, "Config": "NONE_0"}})
	}
	if len(m.dsms["MAM"][Asset]) != 1 || len(m.dsms["Profile"][AppID]) != 1 {
		return nil, fmt.Errorf("sim: content model incomplete")
	}
	m.dsms["Health"][Asset] = []map[string]any{{"DataSetWriterId": 3457.0, "WriterGroupName": "", "DataSetWriterName": Asset,
		"Payload": map[string]any{"Health": "NORMAL_0", "HealthScore": 100.0}}}
	m.dsms["Profile"][Asset] = []map[string]any{{"DataSetWriterId": 75.0, "WriterGroupName": "", "DataSetWriterName": Asset,
		"Payload": map[string]any{"Resources": []any{"MAM", "Health", "Profile", "Config", "Data", "Metadata", "Event", "ReferenceDesignation"}}}}
	for src, ds := range m.dsms["Profile"] {
		p, _ := ds[0]["Payload"].(map[string]any)
		list, _ := p["Resources"].([]any)
		for _, r := range list {
			s, _ := r.(string)
			m.profiles[src] = append(m.profiles[src], s)
		}
	}
	meta, ok := byID["V-PAY-021"]
	if !ok {
		return nil, fmt.Errorf("sim: vector V-PAY-021 (Metadata) not found")
	}
	m.meta[AppID+"|oee"] = meta
	will, ok := byID["V-PAY-025"]
	if !ok {
		return nil, fmt.Errorf("sim: vector V-PAY-025 (Will) not found")
	}
	if m.will, err = json.Marshal(will); err != nil {
		return nil, err
	}
	m.willDoc = will
	return m, nil
}

// Sim is a running simulated CUT.
type Sim struct {
	cfg   mqttwire.Config
	clMu  sync.Mutex
	cl    *mqttwire.Client // the current connection; client() reads it
	conns int              // connections so far
	loop  chan struct{}    // closed when the reconnect loop ends
	m     *model
	fault Fault
	state sync.Mutex // the content model: written by Set and Del, read by Changed
	mu    sync.Mutex
	last  int64
	// reserved counts the DataSetWriterIds NewDataSetWriterId provided
	reserved int
	gets     chan mqttwire.Message
	stop     chan struct{}
	done     chan struct{}
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

// build loads the content model of a run with the changes of the fault.
func build(fault Fault) (*model, error) {
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
	if fault == AssetLevel {
		m.dsms["Profile"][Asset][0]["Payload"] = map[string]any{"Resources": []any{"MAM", "Health", "Profile", "Data", "Metadata", "Event", "ReferenceDesignation"}}
		m.profiles[Asset] = []string{"MAM", "Health", "Profile", "Data", "Metadata", "Event", "ReferenceDesignation"}
	}
	if fault == NoRefDes {
		m.dsms["ReferenceDesignation"] = map[string][]map[string]any{}
	}
	if fault == FixedLevel {
		for _, d := range m.dsms["PublicationList"][AppID] {
			p, _ := d["Payload"].(map[string]any)
			if f, _ := p["Filter"].(string); strings.HasPrefix(f, "Syslog/") {
				p["Config"] = "NONE_0"
			}
		}
	}
	if fault == NotListed {
		var keep []map[string]any
		for _, d := range m.dsms["SubscriptionList"][AppID] {
			if g, _ := d["WriterGroupName"].(string); !strings.HasPrefix(g, "Config/") {
				keep = append(keep, d)
			}
		}
		m.dsms["SubscriptionList"][AppID] = keep
	}
	return m, nil
}

// Start connects the simulated CUT with its Will, subscribes to its Get,
// Set, Del and Call topics and publishes the Birth message and its Health.
// When the connection is lost, it reconnects with a fresh Will and
// publishes its Birth again (MG 6.3.5) until Close.
func Start(ctx context.Context, cfg mqttwire.Config, fault Fault) (*Sim, error) {
	m, err := build(fault)
	if err != nil {
		return nil, err
	}
	s := &Sim{m: m, fault: fault, gets: make(chan mqttwire.Message, 256), stop: make(chan struct{}), done: make(chan struct{}), loop: make(chan struct{})}
	if cfg.ClientID == "" {
		cfg.ClientID = mqttwire.ClientID("oec-sim")
	}
	switch fault {
	case MQTT31:
		cfg.ProtocolVersion = 3
	case TLS11:
		cfg.MinTLSVersion = tls.VersionTLS10
	case NoVerify:
		cfg.Insecure = true
	case Anonymous:
		cfg.Username, cfg.Password, cfg.CertFile, cfg.KeyFile = "", nil, "", ""
	}
	s.cfg = cfg
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
	if err := s.connect(ctx); err != nil {
		close(s.stop)
		close(s.loop)
		<-s.done
		return nil, err
	}
	if fault == FailNoBirth || fault == FailRecovery {
		if err := s.fail(ctx); err != nil {
			s.Close()
			return nil, err
		}
	}
	go s.keep()
	return s, nil
}

func (s *Sim) client() *mqttwire.Client {
	s.clMu.Lock()
	defer s.clMu.Unlock()
	return s.cl
}

// willPayload is the Will of the connection: the Health of the application
// with FAILURE_1/0, created at connect (MG 6.3.5, REQ-LIFE-016).
func (s *Sim) willPayload() ([]byte, error) {
	if s.fault == StaleWill {
		return s.m.will, nil
	}
	doc := map[string]any{}
	for k, v := range s.m.willDoc {
		doc[k] = v
	}
	doc["MessageId"] = s.nextID()
	list, _ := doc["Messages"].([]any)
	d := map[string]any{}
	if len(list) == 1 {
		src, _ := list[0].(map[string]any)
		for k, v := range src {
			d[k] = v
		}
	}
	switch s.fault {
	case WillTime:
		d["Timestamp"] = time.Now().UTC().Format(time.RFC3339Nano)
	case WillWriter:
		d["DataSetWriterId"] = 3999.0
	}
	doc["Messages"] = []any{d}
	return json.Marshal(doc)
}

// connect establishes one connection: Will, subscriptions and Birth.
func (s *Sim) connect(ctx context.Context) error {
	cfg := s.cfg
	if s.fault != NoWill {
		will, err := s.willPayload()
		if err != nil {
			return err
		}
		cfg.Will = &mqttwire.Message{Topic: topic("Pub", "Health", AppID, ""), Payload: will}
	}
	if s.fault == ClientIDs {
		cfg.ClientID = mqttwire.ClientID("oec-sim")
	}
	cl, err := mqttwire.Dial(ctx, cfg)
	if err != nil {
		return err
	}
	s.clMu.Lock()
	s.cl = cl
	n := s.conns
	s.conns++
	s.clMu.Unlock()
	receive := func(m mqttwire.Message) {
		select {
		case s.gets <- m:
		case <-s.stop:
		}
	}
	for _, method := range []string{"Get", "Set", "Del", "Call"} {
		if err := cl.Subscribe(ctx, topic(method, "#", "", ""), 1, receive); err != nil {
			cl.Kill()
			return err
		}
	}
	birth := []string{"MAM", "Health"}
	switch {
	case s.fault == NoRebirth && n > 0:
		birth = []string{"Health"}
	case s.fault == HealthFirst:
		birth = []string{"Health", "MAM"}
	}
	for _, res := range birth {
		if err := s.publish(ctx, res, AppID, "", "", s.m.dsms[res][AppID]); err != nil {
			cl.Kill()
			return err
		}
	}
	if s.fault == DualVersion || s.fault == Mixed {
		if err := s.publishV1(ctx, ""); err != nil {
			cl.Kill()
			return err
		}
	}
	return nil
}

// health publishes the Health of the application with the given state.
func (s *Sim) health(ctx context.Context, state string, score float64) error {
	h := s.m.dsms["Health"][AppID][0]
	d := map[string]any{}
	for k, v := range h {
		d[k] = v
	}
	d["Payload"] = map[string]any{"Health": state, "HealthScore": score}
	return s.publish(ctx, "Health", AppID, "", "", []map[string]any{d})
}

// fail reports FAILURE_1/0 and recovers (FailNoBirth, FailRecovery): with
// a new Birth only for FailRecovery (REQ-LIFE-017).
func (s *Sim) fail(ctx context.Context) error {
	if err := s.health(ctx, "FAILURE_1", 0); err != nil {
		return err
	}
	if s.fault == FailRecovery {
		if err := s.publish(ctx, "MAM", AppID, "", "", s.m.dsms["MAM"][AppID]); err != nil {
			return err
		}
	}
	return s.publish(ctx, "Health", AppID, "", "", s.m.dsms["Health"][AppID])
}

// keep reconnects after a lost connection until Close.
func (s *Sim) keep() {
	defer close(s.loop)
	for {
		select {
		case <-s.client().Lost():
		case <-s.stop:
			return
		}
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := s.connect(ctx)
			cancel()
			if err == nil {
				break
			}
			select {
			case <-time.After(50 * time.Millisecond):
			case <-s.stop:
				return
			}
		}
	}
}

// Connections returns the number of connections so far.
func (s *Sim) Connections() int {
	s.clMu.Lock()
	defer s.clMu.Unlock()
	return s.conns
}

// Close stops answering, publishes the Close message and disconnects.
func (s *Sim) Close() {
	close(s.stop)
	<-s.loop
	<-s.done
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.health(ctx, "NORMAL_0", 0)
	s.client().Close()
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

// V1Topic is the topic of the version 1 Birth of the dual-version
// variants (namespace Oi4, MG 7.1): unencoded AppId, lowercase method and
// resource as in version 1.
var V1Topic = "Oi4/" + ServiceType + "/" + AppID + "/pub/mam/" + AppID

// publishV1 publishes the MAM of the application under the namespace Oi4;
// with a correlation id (Mixed) as the answer to that request.
func (s *Sim) publishV1(ctx context.Context, correlation string) error {
	doc := map[string]any{"MessageId": s.nextID(), "MessageType": "ua-data", "PublisherId": ServiceType + "/" + AppID,
		"DataSetClassId": s.m.classes["MAM"], "CorrelationId": correlation, "Messages": []any{s.m.dsms["MAM"][AppID][0]}}
	payload, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	return s.client().Publish(ctx, mqttwire.Message{Topic: V1Topic, Payload: payload})
}

func (s *Sim) publish(ctx context.Context, res, source, filter, replyTo string, dsms []map[string]any) error {
	return s.publishID(ctx, s.nextID(), res, source, filter, replyTo, dsms)
}

func (s *Sim) publishID(ctx context.Context, id, res, source, filter, replyTo string, dsms []map[string]any) error {
	doc := map[string]any{"MessageId": id, "MessageType": "ua-data", "PublisherId": ServiceType + "/" + AppID, "ReplyTo": replyTo}
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
	// Retained: the Birth (MAM of the application, no answer) is retained.
	retain := s.fault == Retained && res == "MAM" && source == AppID && replyTo == ""
	return s.client().Publish(ctx, mqttwire.Message{Topic: topic("Pub", res, source, filter), Payload: payload, Retain: retain})
}

func contains(s []string, x string) bool {
	for _, e := range s {
		if e == x {
			return true
		}
	}
	return false
}

// request are the helper DataSets of a Get (MG 8.3.11 … 8.3.13).
type request struct {
	perPage, page int
	paged         bool
	locale        string
	parent        string
	aspect        string
	query         bool
}

func helpers(doc map[string]any) request {
	var q request
	list, _ := doc["Messages"].([]any)
	for _, x := range list {
		d, _ := x.(map[string]any)
		p, _ := d["Payload"].(map[string]any)
		id, _ := d["DataSetWriterId"].(float64)
		switch id {
		case 1:
			pp, _ := p["PerPage"].(float64)
			pg, _ := p["Page"].(float64)
			q.perPage, q.page, q.paged = int(pp), int(pg), pp > 0
		case 3:
			q.locale, _ = p["Locale"].(string)
		case 9:
			q.parent, _ = p["Parent"].(string)
			q.aspect, _ = p["Aspect"].(string)
			q.query = true
		}
	}
	return q
}

var aspects = []string{"Function", "Product", "Location"}

// children selects the ReferenceDesignation objects whose Parent (of the
// Aspect, or of any aspect) is the requested one (MG 8.3.13.1).
func children(dsms []map[string]any, parent, aspect string) []map[string]any {
	var out []map[string]any
	for _, d := range dsms {
		p, _ := d["Payload"].(map[string]any)
		for _, a := range aspects {
			if aspect != "" && a != aspect {
				continue
			}
			o, _ := p[a].(map[string]any)
			pa, _ := o["Parent"].(map[string]any)
			if il, _ := pa["IdentificationLink"].(string); il == parent {
				out = append(out, d)
				break
			}
		}
	}
	return out
}

// relabel returns a copy of v with the Locale of every LocalizedText set to
// locale (fault WrongLocale).
func relabel(v any, locale string) any {
	switch o := v.(type) {
	case map[string]any:
		c := map[string]any{}
		_, l := o["Locale"].(string)
		_, t := o["Text"].(string)
		for k, x := range o {
			c[k] = relabel(x, locale)
		}
		if l && t {
			c["Locale"] = locale
		}
		return c
	case []any:
		c := make([]any, len(o))
		for i, x := range o {
			c[i] = relabel(x, locale)
		}
		return c
	}
	return v
}

// answer publishes the answer to a Get, page by page if it carries a
// PaginationRequest (MG 8.3.11.1); with Page 0 every page.
func (s *Sim) answer(ctx context.Context, res, source, filter, replyTo string, q request, dsms []map[string]any) {
	if !q.paged || s.fault == PerPage {
		_ = s.publish(ctx, res, source, filter, replyTo, dsms)
		return
	}
	total := len(dsms)
	last := (total + q.perPage - 1) / q.perPage
	if last == 0 {
		last = 1
	}
	from, to := q.page, q.page
	if q.page == 0 {
		from, to = 1, last
	}
	first := ""
	for page := from; page <= to; page++ {
		id := s.nextID()
		if first == "" {
			first = id
		}
		var part []map[string]any
		for i := (page - 1) * q.perPage; i < page*q.perPage && i < total; i++ {
			part = append(part, dsms[i])
		}
		part = append(part, map[string]any{"DataSetWriterId": 2.0, "WriterGroupName": "Pagination", "DataSetWriterName": AppID,
			"Payload": map[string]any{"TotalCount": total, "PerPage": q.perPage, "Page": page, "HasNext": page*q.perPage < total, "PaginationId": first}})
		_ = s.publishID(ctx, id, res, source, filter, replyTo, part)
	}
}

// metadata answers a Get Metadata/<Source>/<Tag> (MG 9.1.5).
func (s *Sim) metadata(ctx context.Context, source, tag, replyTo string) {
	doc, ok := s.m.meta[source+"|"+tag]
	if !ok || s.fault == NoMetadata {
		return
	}
	c := map[string]any{}
	for k, v := range doc {
		c[k] = v
	}
	c["MessageId"], c["ReplyTo"] = s.nextID(), replyTo
	c["Timestamp"] = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	payload, err := json.Marshal(c)
	if err != nil {
		return
	}
	_ = s.client().Publish(ctx, mqttwire.Message{Topic: topic("Pub", "Metadata", source, tag), Payload: payload})
}

// handle answers one Get, Set, Del or Call.
func (s *Sim) handle(msg mqttwire.Message) {
	lv := strings.Split(msg.Topic, "/")
	if len(lv) < 5 {
		return
	}
	var doc map[string]any
	if json.Unmarshal(msg.Payload, &doc) != nil {
		return
	}
	s.state.Lock()
	defer s.state.Unlock()
	if lv[3] == "Set" || lv[3] == "Del" {
		s.write(lv, doc)
		return
	}
	if lv[3] == "Call" {
		s.call(lv, doc)
		return
	}
	if lv[3] != "Get" {
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if s.fault == Mixed && res == "MAM" {
		_ = s.publishV1(ctx, id)
	}
	q := helpers(doc)
	if res == "Metadata" && len(lv) >= 7 {
		if source, ok := dec(lv[5]); ok {
			s.metadata(ctx, source, strings.Join(lv[6:], "/"), replyTo)
		}
		return
	}
	bySource, ok := s.m.dsms[res]
	if !ok {
		return
	}
	if res == "MAM" && q.locale == "de-DE" {
		bySource = s.m.dsms["MAM/de-DE"]
	}
	localize := func(ds []map[string]any) []map[string]any {
		if s.fault != WrongLocale || q.locale == "" {
			return ds
		}
		out := make([]map[string]any, len(ds))
		for i, d := range ds {
			out[i], _ = relabel(d, "fr-FR").(map[string]any)
		}
		return out
	}
	if len(lv) == 5 {
		var all []map[string]any
		for _, src := range []string{AppID, Asset} {
			if s.fault == AllAssets && res == "Health" && src == Asset {
				continue
			}
			for _, d := range bySource[src] {
				if !s.off(res, src, d) {
					all = append(all, d)
				}
			}
		}
		if res == "ReferenceDesignation" && q.query && s.fault != QueryIgnored {
			all = children(all, q.parent, q.aspect)
		}
		s.answer(ctx, res, "", "", replyTo, q, localize(all))
		return
	}
	source, ok := dec(lv[5])
	filter := strings.Join(lv[6:], "/")
	requested := source
	if ok && s.fault == Case {
		for _, il := range []string{AppID, Asset} {
			if strings.EqualFold(source, il) {
				source = il
			}
		}
	}
	if !ok || (source != AppID && source != Asset) {
		if s.fault == UnknownSource {
			_ = s.publish(ctx, res, source, filter, replyTo, nil)
			return
		}
		ev := map[string]any{"DataSetWriterId": statusWriter(AppID, "Bad"), "WriterGroupName": "Status/Bad", "DataSetWriterName": AppID,
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
		g, _ := d["WriterGroupName"].(string)
		if s.off(res, source, d) {
			continue
		}
		if filter == "" || g == filter || (s.fault == FilterIgnored && res == "PublicationList") {
			out = append(out, d)
		}
	}
	if res == "ReferenceDesignation" && q.query && s.fault == QuerySource {
		out = children(out, q.parent, q.aspect)
	}
	if len(out) == 0 && filter != "" && s.fault == EmptyList {
		return
	}
	// The answer is published on the topic of the request (the Source as
	// requested, also with the fault Case).
	s.answer(ctx, res, requested, filter, replyTo, q, localize(out))
}
