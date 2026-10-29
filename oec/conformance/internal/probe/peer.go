package probe

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/OI4/oec/conformance/internal/mqttwire"
)

// Roles of the announced components (DESIGN.md §3.4, Q-029 (a)).
const (
	RoleTester = "tester" // D7 itself with --announce
	RolePeer   = "peer"   // the simulated second component of --peer
)

// AnnouncedServiceType is the ServiceType of the announced components: D7
// is a test tool (MG 4.2).
const AnnouncedServiceType = "Utility"

// Heartbeat is the interval of the Health of an announced component: at
// least every 60 s (MG 7.5.1, SF-106).
const Heartbeat = 30 * time.Second

// DataSetWriterIds of the announced components (MG 8.3.1: unique per
// publisher, ≥ 10).
const (
	writerMAM     = 10
	writerHealth  = 11
	writerProfile = 12
	writerStatus  = 13
)

// statusEventClass is the DataSetClassId of the Event resource (MG 8.3).
const statusEventClass = "543ae05e-b6d9-4161-a0a3-350a0fac5976"

// Announced is one component D7 announced during the run (report).
type Announced struct {
	Role        string `json:"role"`
	PublisherID string `json:"publisher_id"`
	Requests    int    `json:"requests"` // requests received on its Get topics
	Answered    int    `json:"answered"` // of these answered (Pub or Event Status)
}

// PeerIL returns the Identification Link of the simulated peer for one run:
// an Oi4Identifier with a random serial number.
func PeerIL() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "oi4.org/oec-check/Peer/" + hex.EncodeToString(b[:])
}

// component is a Level 1 OEC application D7 announces (Birth, heartbeat,
// Close message, Will) and whose Get MAM, Health and Profile it answers.
type component struct {
	role string
	il   string
	pid  string
	pub  func(ctx context.Context, topic string, payload []byte) error
	in   chan Message

	mu       sync.Mutex
	last     int64
	requests int
	answered int
}

func newComponent(role, il string) *component {
	return &component{role: role, il: il, pid: AnnouncedServiceType + "/" + il, in: make(chan Message, 256)}
}

// identity splits the Oi4Identifier into the MAM elements (MG 4.3).
func (c *component) identity() (manufacturerURI, model, productCode, serial string) {
	lv := strings.SplitN(c.il, "/", 4)
	for len(lv) < 4 {
		lv = append(lv, "")
	}
	return lv[0], lv[1], lv[2], lv[3]
}

func (c *component) mam() map[string]any {
	uri, model, code, serial := c.identity()
	desc := "OEC conformance tool D7 (oec-check)"
	if c.role == RolePeer {
		desc = "Simulated peer component of the OEC conformance tool D7 (oec-check --peer)"
	}
	return map[string]any{
		"Manufacturer":       map[string]any{"Locale": "en-US", "Text": "OI4"},
		"ManufacturerUri":    uri,
		"Model":              map[string]any{"Locale": "en-US", "Text": model},
		"ProductCode":        code,
		"HardwareRevision":   "",
		"SoftwareRevision":   "2.0.0",
		"DeviceRevision":     "",
		"DeviceManual":       "https://" + uri + "/oec",
		"DeviceClass":        "Oi4." + AnnouncedServiceType,
		"SerialNumber":       serial,
		"ProductInstanceUri": c.il,
		"RevisionCounter":    1,
		"Description":        map[string]any{"Locale": "en-US", "Text": desc},
	}
}

func (c *component) dsm(writer int, group string, payload any) map[string]any {
	return map[string]any{"DataSetWriterId": writer, "WriterGroupName": group, "DataSetWriterName": c.il, "Payload": payload}
}

func (c *component) healthDSM(state string, score int) map[string]any {
	return c.dsm(writerHealth, "", map[string]any{"Health": state, "HealthScore": score})
}

func (c *component) nextID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	ms := time.Now().UnixMilli()
	if ms <= c.last {
		ms = c.last + 1
	}
	c.last = ms
	return fmt.Sprintf("%d-%s", ms, c.pid)
}

func (c *component) payload(class, replyTo string, dsms ...map[string]any) ([]byte, error) {
	list := []any{}
	for _, d := range dsms {
		list = append(list, d)
	}
	doc := map[string]any{"MessageId": c.nextID(), "MessageType": "ua-data", "PublisherId": c.pid, "DataSetClassId": class,
		"ReplyTo": replyTo, "Messages": list}
	return json.Marshal(doc)
}

func (c *component) topic(method, resource, source, filter string) string {
	return Topic(AnnouncedServiceType, c.il, method, resource, source, filter)
}

func (c *component) publish(ctx context.Context, resource, source, filter, replyTo string, dsms ...map[string]any) error {
	class := Classes[resource]
	if resource == "Event" {
		class = statusEventClass
	}
	p, err := c.payload(class, replyTo, dsms...)
	if err != nil {
		return err
	}
	return c.pub(ctx, c.topic("Pub", resource, source, filter), p)
}

// will is the Will of the connection: the Health of the component with
// FAILURE_1/0, created at connect (MG 6.3.5).
func (c *component) will() (*mqttwire.Message, error) {
	p, err := c.payload(Classes["Health"], "", c.healthDSM("FAILURE_1", 0))
	if err != nil {
		return nil, err
	}
	return &mqttwire.Message{Topic: c.topic("Pub", "Health", c.il, ""), Payload: p}, nil
}

// birth publishes the Birth message and the Health (MG 6.3.3).
func (c *component) birth(ctx context.Context) error {
	if err := c.publish(ctx, "MAM", c.il, "", "", c.dsm(writerMAM, "", c.mam())); err != nil {
		return err
	}
	return c.health(ctx, "NORMAL_0", 100)
}

func (c *component) health(ctx context.Context, state string, score int) error {
	return c.publish(ctx, "Health", c.il, "", "", c.healthDSM(state, score))
}

// close publishes the Close message (MG 6.3.4); the caller disconnects.
func (c *component) close(ctx context.Context) error { return c.health(ctx, "NORMAL_0", 0) }

// owns reports whether a topic is a request to the component.
func (c *component) owns(topic string) bool {
	return strings.HasPrefix(topic, "Oi4v2/"+AnnouncedServiceType+"/"+enc(c.il)+"/Get/")
}

// deliver queues a request; a full queue drops it.
func (c *component) deliver(m Message) {
	select {
	case c.in <- m:
	default:
	}
}

// run sends the heartbeat and answers the requests until ctx ends.
func (c *component) run(ctx context.Context) {
	tick := time.NewTicker(Heartbeat)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			_ = c.health(ctx, "NORMAL_0", 100)
		case m := <-c.in:
			c.answer(ctx, m)
		}
	}
}

// answer reacts to a Get: MAM, Health and Profile of the component, with
// or without its AppId as Source; an unknown Source gets Event Status Bad
// BadNotFound (MG 8.3.6.1, SF-099); other resources are not supported.
func (c *component) answer(ctx context.Context, m Message) {
	lv := strings.Split(m.Topic, "/")
	if len(lv) < 5 {
		return
	}
	var doc map[string]any
	if json.Unmarshal(m.Payload, &doc) != nil {
		return
	}
	c.mu.Lock()
	c.requests++
	c.mu.Unlock()
	id, _ := doc["MessageId"].(string)
	res, source, filter := lv[4], "", ""
	if len(lv) > 5 {
		source = dec(lv[5])
		filter = strings.Join(lv[6:], "/")
	}
	var d map[string]any
	switch res {
	case "MAM":
		d = c.dsm(writerMAM, "", c.mam())
	case "Health":
		d = c.healthDSM("NORMAL_0", 100)
	case "Profile":
		d = c.dsm(writerProfile, "", map[string]any{"Resources": []any{"MAM", "Health", "Profile"}})
	default:
		return
	}
	var err error
	switch {
	case len(lv) > 5 && source != c.il:
		ev := c.dsm(writerStatus, "Status/Bad", map[string]any{"Number": BadNotFound, "Description": "The requested asset is unknown.",
			"Category": "CAT_STATUS_1", "Details": map[string]any{"SymbolicId": "BadNotFound"}})
		err = c.publish(ctx, "Event", c.il, "Status/Bad", id, ev)
	case filter != "":
		err = c.publish(ctx, res, source, filter, id)
	default:
		err = c.publish(ctx, res, source, "", id, d)
	}
	if err == nil {
		c.mu.Lock()
		c.answered++
		c.mu.Unlock()
	}
}

func (c *component) report() Announced {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Announced{Role: c.role, PublisherID: c.pid, Requests: c.requests, Answered: c.answered}
}
