package bus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/OI4/oec/conformance/internal/ilink"
	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/probe"
)

// errLost marks a connection the broker closed during an exchange.
var errLost = errors.New("the broker closed the connection")

// aclProbe is one publication of the unprivileged client under the probe
// AppId of D8.
type aclProbe struct {
	method, resource, filter string
	reqs                     []string // besides REQ-GEN-001
	// own: a publication only the participant with the AppId should make
	// (Pub, Reply: REQ-GEN-002, SHOULD, SF-119) instead of a write request
	// (REQ-GEN-001 and reqs, MUST).
	own bool
}

var aclProbes = []aclProbe{
	{"Set", "Config", "", []string{reqSetConfig}, false},
	{"Set", "Data", "", []string{reqSetData}, false},
	{"Set", "Metadata", "probe", []string{reqSetMeta}, false},
	{"Set", "PublicationList", "", []string{reqSetPubList}, false},
	{"Set", "ReferenceDesignation", "", []string{reqRefDes}, false},
	{"Del", "ReferenceDesignation", "", []string{reqRefDes}, false},
	{"Set", "SubscriptionList", "", []string{reqSubList}, false},
	{"Del", "SubscriptionList", "", []string{reqSubList}, false},
	{"Call", "NewDataSetWriterId", "", nil, false},
	{"Pub", "Health", "", nil, true},
	{"Reply", "NewDataSetWriterId", "", nil, true},
}

// verdictReqs are the requirements a probe is judged against.
func (p aclProbe) verdictReqs() []string {
	if p.own {
		return []string{reqOwnAppID}
	}
	return append([]string{reqACL}, p.reqs...)
}

func (p aclProbe) name() string { return p.method + " " + p.resource }

// marker is the MessageId of a probe message; the receiver identifies the
// message by it.
type marker struct {
	mu  sync.Mutex
	pid string
	n   int64
}

func (m *marker) next() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ms := time.Now().UnixMilli()
	if ms <= m.n {
		ms = m.n + 1
	}
	m.n = ms
	return fmt.Sprintf("%d-%s", ms, m.pid)
}

// request is a request NetworkMessage of a probe (empty Messages: the
// probe AppId has no component that would process it).
func request(id, pid, resource string) []byte {
	doc := map[string]any{"MessageId": id, "MessageType": "ua-data", "PublisherId": pid, "ReplyTo": "", "Messages": []any{}}
	if c, ok := probe.Classes[resource]; ok {
		doc["DataSetClassId"] = c
	}
	raw, _ := json.Marshal(doc)
	return raw
}

// seen collects the MessageIds a client received.
type seen struct {
	mu  sync.Mutex
	ids map[string]bool
}

func (s *seen) add(m mqttwire.Message) {
	var doc struct{ MessageId string }
	if json.NewDecoder(bytes.NewReader(m.Payload)).Decode(&doc) != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ids[doc.MessageId] = true
}

func (s *seen) has(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ids[id]
}

// wait waits until every id was seen or the timeout passed.
func (s *seen) wait(ctx context.Context, d time.Duration, ids ...string) {
	end := time.Now().Add(d)
	for time.Now().Before(end) && ctx.Err() == nil {
		all := true
		for _, id := range ids {
			all = all && s.has(id)
		}
		if all {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// acl is T8-BUS-02: the unprivileged client publishes Set, Del and Call to
// the probe AppId of D8, which D8 must not receive (REQ-GEN-001 and the
// resource requirements), and must not read Set/Config (REQ-RES-CONFIG-026).
// It also publishes Pub and Reply under the probe AppId, which D8 should not
// receive (REQ-GEN-002, SHOULD: a warning). A positive control shows that D8
// itself receives the same topics.
func (r *runner) acl(ctx context.Context, main *mqttwire.Client) {
	if r.c.Second == nil {
		r.inconclusive(TACL, aclReqs, "no unprivileged client: --credentials-2 or --bus-storage-2 names one")
		return
	}
	if same(r.c.Main, *r.c.Second) {
		r.inconclusive(TACL, aclReqs, "the second client has the identity of D8: --credentials-2 names an unprivileged user")
		return
	}
	pid := ServiceType + "/" + r.probe
	mk := &marker{pid: pid}
	got := &seen{ids: map[string]bool{}}
	all := "Oi4v2/" + ServiceType + "/" + ilink.Encode(r.probe) + "/#"
	if err := main.Subscribe(ctx, all, 1, got.add); err != nil {
		r.inconclusive(TACL, aclReqs, "D8 cannot subscribe to %s: %v", all, err)
		return
	}
	topics := make([]string, len(aclProbes))
	controls := make([]string, len(aclProbes))
	for i, p := range aclProbes {
		topics[i] = probe.Topic(ServiceType, r.probe, p.method, p.resource, r.probe, p.filter)
		controls[i] = mk.next()
		if err := main.Publish(ctx, mqttwire.Message{Topic: topics[i], Payload: request(controls[i], pid, p.resource), QoS: 1}); err != nil {
			r.inconclusive(TACL, aclReqs, "D8 cannot publish %s: %v", p.name(), err)
			return
		}
	}
	got.wait(ctx, r.c.Timeout, controls...)

	u, err := r.dial(ctx, *r.c.Second, "oec-infra-u-", nil)
	if err != nil {
		r.inconclusive(TACL, aclReqs, "the unprivileged client cannot connect: %v", err)
		return
	}
	defer func() {
		if u != nil {
			u.Kill()
		}
	}()

	// Read access to Set/Config (REQ-RES-CONFIG-026).
	readTopic := topics[0]
	read := &seen{ids: map[string]bool{}}
	readID := ""
	if err := u.Subscribe(ctx, readTopic, 1, read.add); err != nil {
		r.checked(TACL, reqReadConfig, "the broker refuses the subscription of the unprivileged client to %s: %v", readTopic, err)
		u.Kill()
		u = nil
	} else {
		readID = mk.next()
		if err := main.Publish(ctx, mqttwire.Message{Topic: readTopic, Payload: request(readID, pid, "Config"), QoS: 1}); err != nil {
			r.inconclusive(TACL, []string{reqReadConfig}, "D8 cannot publish Set Config: %v", err)
			readID = ""
		}
	}

	ids := make([]string, len(aclProbes))
	for i, p := range aclProbes {
		if u == nil {
			if u, err = r.dial(ctx, *r.c.Second, "oec-infra-u-", nil); err != nil {
				r.inconclusive(TACL, p.verdictReqs(), "the unprivileged client cannot reconnect: %v", err)
				continue
			}
		}
		ids[i] = mk.next()
		pctx, cancel := context.WithTimeout(ctx, r.c.Timeout)
		err := u.Publish(pctx, mqttwire.Message{Topic: topics[i], Payload: request(ids[i], pid, p.resource), QoS: 1})
		cancel()
		select {
		case <-u.Lost():
			err = errLost
		default:
		}
		if err != nil {
			// The broker refused the message by closing the connection
			// (or did not acknowledge it).
			u.Kill()
			u = nil
		}
	}
	waitIDs := []string{}
	if readID != "" {
		waitIDs = append(waitIDs, readID)
	}
	for _, id := range ids {
		if id != "" {
			waitIDs = append(waitIDs, id)
		}
	}
	// Positive results end the wait early; a conforming broker makes D8 wait
	// the whole timeout.
	got.wait(ctx, r.c.Timeout, waitIDs...)

	if readID != "" {
		if read.has(readID) {
			r.warning(TACL, reqReadConfig, readTopic, "the unprivileged client may subscribe to and receives Set Config")
		} else {
			r.checked(TACL, reqReadConfig, "the unprivileged client does not receive Set Config")
		}
	}
	for i, p := range aclProbes {
		reqs := p.verdictReqs()
		switch {
		case !got.has(controls[i]):
			r.inconclusive(TACL, reqs, "D8 does not receive its own %s on %s: the probe cannot observe it", p.name(), topics[i])
		case ids[i] == "":
		case got.has(ids[i]) && p.own:
			r.warning(TACL, reqOwnAppID, topics[i], "the unprivileged client published %s under the AppId of D8 and the broker delivered it", p.name())
		case got.has(ids[i]):
			for _, req := range reqs {
				r.violation(TACL, req, topics[i], "the unprivileged client published %s and the broker delivered it", p.name())
			}
		default:
			for _, req := range reqs {
				r.checked(TACL, req, "the broker did not deliver %s of the unprivileged client", p.name())
			}
		}
	}
}

// same reports whether two connection configurations have the same
// identity at the broker.
func same(a, b mqttwire.Config) bool {
	if a.Username != "" && b.Username != "" {
		return a.Username == b.Username
	}
	return a.Username == b.Username && a.CertFile == b.CertFile
}
