package run

import (
	"bufio"
	"context"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/OI4/oec/conformance/internal/report"
	"github.com/OI4/oec/conformance/internal/topic"
)

// BrokerOverhead is the difference between the packet limit of the broker
// and MaxPacketSize: MaxPacketSize limits the payload, a packet may be
// larger by the topic and the headers (RIG 4.1.1, SF-105).
const BrokerOverhead = 65544

// MQTT control packet types.
const (
	pConnect     = 1
	pPublish     = 3
	pPuback      = 4
	pPubrec      = 5
	pPubrel      = 6
	pPubcomp     = 7
	pSubscribe   = 8
	pUnsubscribe = 10
	pPingreq     = 12
	pDisconnect  = 14
)

// endpoint is the MQTT test endpoint of one phase: a TLS listener that
// accepts the CONNECT of the container with the generated credentials or
// the generated client certificate, answers like a broker without
// forwarding and records what the container did. It keeps no password.
type endpoint struct {
	ln        net.Listener
	tls       *tls.Config
	clientCAs *x509.CertPool
	user      string
	pass      []byte
	maxPacket int
	spec      *topic.Spec

	mu      sync.Mutex
	conns   []*conn
	changed chan struct{}
	wg      sync.WaitGroup
	open    map[net.Conn]bool
	closed  bool
}

// conn is what the endpoint saw on one connection.
type conn struct {
	report.RunConnection
	birth *report.Birth
}

func listen(addr string, cert tls.Certificate, clientCAs *x509.CertPool, user string, pass []byte, maxPacket int, spec *topic.Spec) (*endpoint, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort(addr, "0"))
	if err != nil {
		return nil, err
	}
	e := &endpoint{
		ln: ln, clientCAs: clientCAs, user: user, pass: pass, maxPacket: maxPacket, spec: spec,
		changed: make(chan struct{}, 1), open: map[net.Conn]bool{},
		tls: &tls.Config{
			MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13,
			Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequestClientCert,
		},
	}
	e.wg.Add(1)
	go e.accept()
	return e, nil
}

// Port is the SecurePort of the endpoint.
func (e *endpoint) Port() int { return e.ln.Addr().(*net.TCPAddr).Port }

// Changed signals a new event (a connection, a CONNECT, a PUBLISH).
func (e *endpoint) Changed() <-chan struct{} { return e.changed }

func (e *endpoint) notify() {
	select {
	case e.changed <- struct{}{}:
	default:
	}
}

// Close stops the listener and every connection and waits for them.
func (e *endpoint) Close() {
	e.mu.Lock()
	e.closed = true
	for c := range e.open {
		c.Close()
	}
	e.mu.Unlock()
	e.ln.Close()
	e.wg.Wait()
}

// Snapshot returns copies of the recorded connections.
func (e *endpoint) Snapshot() []conn {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]conn, 0, len(e.conns))
	for _, c := range e.conns {
		cc := *c
		if c.birth != nil {
			b := *c.birth
			cc.birth = &b
		}
		out = append(out, cc)
	}
	return out
}

func (e *endpoint) accept() {
	defer e.wg.Done()
	for {
		raw, err := e.ln.Accept()
		if err != nil {
			return
		}
		e.mu.Lock()
		if e.closed {
			e.mu.Unlock()
			raw.Close()
			return
		}
		e.open[raw] = true
		e.mu.Unlock()
		e.wg.Add(1)
		go func() {
			defer e.wg.Done()
			e.serve(raw)
			e.mu.Lock()
			delete(e.open, raw)
			e.mu.Unlock()
			raw.Close()
		}()
	}
}

// update changes the record of a connection under the lock.
func (e *endpoint) update(c *conn, f func()) {
	e.mu.Lock()
	f()
	e.mu.Unlock()
	e.notify()
}

func (e *endpoint) serve(raw net.Conn) {
	c := &conn{RunConnection: report.RunConnection{ClientCertificate: "none"}}
	tc := tls.Server(raw, e.tls)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	err := tc.HandshakeContext(ctx)
	cancel()
	e.mu.Lock()
	e.conns = append(e.conns, c)
	e.mu.Unlock()
	if err != nil {
		e.update(c, func() { c.Handshake, c.HandshakeError = "failed", handshakeError(err) })
		return
	}
	st := tc.ConnectionState()
	e.update(c, func() {
		c.Handshake = "ok"
		c.TLS = map[uint16]string{tls.VersionTLS12: "1.2", tls.VersionTLS13: "1.3"}[st.Version]
		if len(st.PeerCertificates) > 0 {
			c.ClientCertificate = "untrusted"
			inter := x509.NewCertPool()
			for _, ic := range st.PeerCertificates[1:] {
				inter.AddCert(ic)
			}
			if _, err := st.PeerCertificates[0].Verify(x509.VerifyOptions{Roots: e.clientCAs, Intermediates: inter, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err == nil {
				c.ClientCertificate = "verified"
			}
		}
	})
	r := bufio.NewReader(tc)
	_ = tc.SetReadDeadline(time.Now().Add(15 * time.Second))
	f, err := e.read(r, c)
	if err != nil || f.kind() != pConnect {
		return
	}
	cn, err := parseConnect(f)
	if err != nil {
		return
	}
	auth, accepted := "none", false
	switch {
	case c.ClientCertificate == "verified":
		auth, accepted = "certificate", true
	case cn.hasUser:
		auth = "credentials-wrong"
		if subtle.ConstantTimeCompare([]byte(cn.user), []byte(e.user)) == 1 && subtle.ConstantTimeCompare(cn.pass, e.pass) == 1 && e.user != "" {
			auth, accepted = "credentials", true
		}
	}
	clear(cn.pass)
	e.update(c, func() { c.ProtocolLevel, c.ClientID, c.Auth, c.Accepted = int(cn.level), cn.clientID, auth, accepted })
	var ack []byte
	switch {
	case cn.level == 5 && accepted:
		ack = []byte{0x20, 3, 0, 0, 0}
	case cn.level == 5 && auth == "credentials-wrong":
		ack = []byte{0x20, 3, 0, 0x86, 0}
	case cn.level == 5:
		ack = []byte{0x20, 3, 0, 0x87, 0}
	case accepted:
		ack = []byte{0x20, 2, 0, 0}
	case auth == "credentials-wrong":
		ack = []byte{0x20, 2, 0, 4}
	default:
		ack = []byte{0x20, 2, 0, 5}
	}
	if _, err := tc.Write(ack); err != nil || !accepted {
		return
	}
	for {
		_ = tc.SetReadDeadline(time.Time{})
		f, err := e.read(r, c)
		if err != nil {
			return
		}
		var out []byte
		switch f.kind() {
		case pPublish:
			id, ok := e.publish(c, cn.level, f)
			if !ok {
				return
			}
			switch (f.header >> 1) & 3 {
			case 1:
				out = []byte{pPuback << 4, 2, byte(id >> 8), byte(id)}
			case 2:
				out = []byte{pPubrec << 4, 2, byte(id >> 8), byte(id)}
			}
		case pPubrel:
			if len(f.body) < 2 {
				return
			}
			out = []byte{pPubcomp << 4, 2, f.body[0], f.body[1]}
		case pSubscribe, pUnsubscribe:
			out = e.subAck(cn.level, f)
			if out == nil {
				return
			}
		case pPingreq:
			out = []byte{0xD0, 0}
		case pDisconnect:
			return
		}
		if out != nil {
			if _, err := tc.Write(out); err != nil {
				return
			}
		}
	}
}

// handshakeError shortens the error of a failed TLS handshake.
func handshakeError(err error) string {
	switch {
	case errors.Is(err, io.EOF):
		return "the client closed the connection during the handshake"
	case errors.Is(err, context.DeadlineExceeded):
		return "no handshake within 15 s"
	}
	s := err.Error()
	if i := strings.LastIndex(s, "tls: "); i >= 0 {
		s = s[i:]
	}
	return s
}

type frame struct {
	header byte
	body   []byte
}

func (f frame) kind() byte { return f.header >> 4 }

// read reads one control packet. A packet larger than the packet limit of
// the broker (MaxPacketSize + BrokerOverhead) is recorded as oversized and
// ends the connection without being read.
func (e *endpoint) read(r *bufio.Reader, c *conn) (frame, error) {
	h, err := r.ReadByte()
	if err != nil {
		return frame{}, err
	}
	n, mul := 0, 1
	for i := 0; ; i++ {
		if i == 4 {
			return frame{}, errors.New("malformed Remaining Length")
		}
		b, err := r.ReadByte()
		if err != nil {
			return frame{}, err
		}
		n += int(b&0x7f) * mul
		mul *= 128
		if b&0x80 == 0 {
			break
		}
	}
	if n > e.maxPacket+BrokerOverhead {
		e.update(c, func() {
			c.Oversized++
			if h>>4 == pPublish && n > c.LargestPayload {
				c.LargestPayload = n
			}
		})
		return frame{}, fmt.Errorf("packet of %d octets above the packet limit", n)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return frame{}, err
	}
	return frame{header: h, body: body}, nil
}

type connect struct {
	level    byte
	clientID string
	hasUser  bool
	user     string
	pass     []byte
}

type fields struct {
	b   []byte
	bad bool
}

func (r *fields) byte1() byte {
	if r.bad || len(r.b) < 1 {
		r.bad = true
		return 0
	}
	v := r.b[0]
	r.b = r.b[1:]
	return v
}

func (r *fields) uint16() uint16 {
	if r.bad || len(r.b) < 2 {
		r.bad = true
		return 0
	}
	v := binary.BigEndian.Uint16(r.b)
	r.b = r.b[2:]
	return v
}

func (r *fields) binary() []byte {
	n := int(r.uint16())
	if r.bad || len(r.b) < n {
		r.bad = true
		return nil
	}
	v := r.b[:n:n]
	r.b = r.b[n:]
	return v
}

// varint reads a Variable Byte Integer (MQTT 5.0 property length).
func (r *fields) varint() int {
	n, mul := 0, 1
	for i := 0; i < 4; i++ {
		b := r.byte1()
		if r.bad {
			return 0
		}
		n += int(b&0x7f) * mul
		mul *= 128
		if b&0x80 == 0 {
			return n
		}
	}
	r.bad = true
	return 0
}

func (r *fields) skip(n int) {
	if r.bad || len(r.b) < n {
		r.bad = true
		return
	}
	r.b = r.b[n:]
}

// parseConnect decodes a CONNECT of MQTT 3.1, 3.1.1 or 5.0.
func parseConnect(f frame) (*connect, error) {
	r := &fields{b: f.body}
	name := string(r.binary())
	c := &connect{level: r.byte1()}
	flags := r.byte1()
	r.uint16()
	if r.bad || !(name == "MQIsdp" && c.level == 3 || name == "MQTT" && (c.level == 4 || c.level == 5)) {
		return nil, errors.New("not a CONNECT of MQTT 3.1, 3.1.1 or 5.0")
	}
	if c.level == 5 {
		r.skip(r.varint())
	}
	c.clientID = string(r.binary())
	if flags&0x04 != 0 {
		if c.level == 5 {
			r.skip(r.varint())
		}
		r.binary()
		r.binary()
	}
	if flags&0x80 != 0 {
		c.hasUser, c.user = true, string(r.binary())
	}
	if flags&0x40 != 0 {
		c.pass = append([]byte(nil), r.binary()...)
	}
	if r.bad {
		return nil, errors.New("malformed CONNECT")
	}
	return c, nil
}

// publish records one PUBLISH and returns its packet identifier.
func (e *endpoint) publish(c *conn, level byte, f frame) (uint16, bool) {
	r := &fields{b: f.body}
	t := string(r.binary())
	var id uint16
	if (f.header>>1)&3 > 0 {
		id = r.uint16()
	}
	if level == 5 {
		r.skip(r.varint())
	}
	if r.bad {
		return 0, false
	}
	payload := r.b
	b := e.birth(t, payload)
	e.update(c, func() {
		c.Publishes++
		if len(payload) > c.LargestPayload {
			c.LargestPayload = len(payload)
		}
		if len(payload) > e.maxPacket {
			c.Oversized++
		}
		if c.birth == nil && b != nil {
			c.birth = b
		}
	})
	return id, true
}

// birth returns the Birth message of a PUBLISH: the Pub MAM of the
// application about itself (Source = AppId, MG 6.3.3).
func (e *endpoint) birth(t string, payload []byte) *report.Birth {
	res, err := e.spec.Check(t)
	if err != nil || !res.Valid || res.Parsed == nil {
		return nil
	}
	p := res.Parsed
	if p.Method != "Pub" || p.Resource != "MAM" || p.SourceIL == "" || p.SourceIL != p.AppIDIL {
		return nil
	}
	var doc struct {
		Messages []struct {
			Payload struct {
				SerialNumber string `json:"SerialNumber"`
			} `json:"Payload"`
		} `json:"Messages"`
	}
	b := &report.Birth{Topic: t, IdentificationLink: p.AppIDIL}
	if json.Unmarshal(payload, &doc) == nil && len(doc.Messages) > 0 {
		b.SerialNumber = doc.Messages[0].Payload.SerialNumber
	}
	return b
}

// subAck answers a SUBSCRIBE (granted QoS 0) or an UNSUBSCRIBE.
func (e *endpoint) subAck(level byte, f frame) []byte {
	r := &fields{b: f.body}
	id := r.uint16()
	if level == 5 {
		r.skip(r.varint())
	}
	n := 0
	for !r.bad && len(r.b) > 0 {
		r.binary()
		if f.kind() == pSubscribe {
			r.byte1()
		}
		n++
	}
	if r.bad {
		return nil
	}
	kind := byte(9)
	if f.kind() == pUnsubscribe {
		kind = 11
	}
	body := []byte{byte(id >> 8), byte(id)}
	if level == 5 {
		body = append(body, 0)
	}
	if kind == 9 || level == 5 {
		body = append(body, make([]byte, n)...)
	}
	out := []byte{kind << 4}
	for n := len(body); ; {
		b := byte(n % 128)
		n /= 128
		if n > 0 {
			b |= 0x80
		}
		out = append(out, b)
		if n == 0 {
			break
		}
	}
	return append(out, body...)
}
