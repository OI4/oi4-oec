// Package proxy is the proxy attachment of oec-check test (DESIGN.md §3.4,
// Q-028 (a), ADR-0025 amendment C-066): the component under test (CUT)
// connects to D7 instead of the broker, and D7 relays every MQTT packet
// unchanged to the broker. On the way D7 sees what the bus cannot show: the
// TLS version, the CONNECT (protocol level, ClientId, keep alive, clean
// session, Will, login) and the PUBLISH packets with their retain flag.
//
// The relay is built on the standard library (crypto/tls, net) with an own
// strict MQTT frame codec: paho has no server side, and the packet decoder
// of paho is lenient (it reads a CONNECT of any protocol name).
//
// D7 terminates the TLS connection of the CUT with a server certificate
// that it issues itself from a CA the CUT trusts (--proxy-ca,
// --proxy-ca-key). For the negative phases of T7-CON-02 it can answer the
// next connection attempts with TLS 1.1 only, with a certificate of an
// untrusted CA or with a certificate for a wrong host name instead of
// relaying (Reject); Drop ends the relayed connections without DISCONNECT,
// so the broker publishes the Will of the CUT (T7-LIFE-02, T7-LIFE-03).
//
// Limitation: a client certificate of the CUT is seen (T7-CON-03) but not
// relayed; the upstream connection uses the TLS settings of D7 (its CA)
// but never D7's own client certificate (C-113 RT-1): the CUT
// authenticates with the credentials of its CONNECT or, if given, with
// Config.UpstreamCert, the identity of the CUT.
package proxy

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"net"
	"sync"
	"time"
)

// Phase is the way the proxy answers one connection attempt.
type Phase string

// Phases of a connection attempt.
const (
	Relay       Phase = "relay"        // TLS 1.2+ with the proxy certificate, relayed to the broker
	TLS11       Phase = "tls1.1"       // TLS 1.1 only (REQ-MQTT-005)
	UntrustedCA Phase = "untrusted-ca" // a certificate of a CA the CUT does not know (REQ-MQTT-008)
	WrongHost   Phase = "wrong-host"   // a certificate of the trusted CA for another host name (REQ-MQTT-008)
)

// WrongHostName is the only name of the WrongHost certificate.
const WrongHostName = "wrong-host.invalid"

// Config of a proxy.
type Config struct {
	Listen      string      // host:port of the proxy ("127.0.0.1:0" in tests)
	Upstream    string      // host:port of the broker
	UpstreamTLS *tls.Config // TLS of the connection to the broker (D7's CA, ServerName); its client certificate is ignored
	// UpstreamCert is the client certificate presented to the broker for
	// relayed clients: the identity of the CUT. Nil: none, the CUT
	// authenticates with the credentials of its CONNECT (C-113 RT-1).
	UpstreamCert *tls.Certificate
	CA           *x509.Certificate
	CAKey        crypto.Signer
	// Names of the proxy certificate (DNS names or IP addresses); default
	// localhost, 127.0.0.1 and ::1.
	Names            []string
	HandshakeTimeout time.Duration // TLS handshake and first packet; default 10 s
}

// Conn is one connection attempt of a client.
type Conn struct {
	ID           int       `json:"id"`
	Phase        Phase     `json:"phase"`
	Remote       string    `json:"remote"`
	Accepted     time.Time `json:"accepted"`
	TLSVersion   string    `json:"tls_version,omitempty"` // negotiated version; empty: no handshake
	ClientCert   bool      `json:"client_certificate"`
	HandshakeErr string    `json:"handshake_error,omitempty"`
	Connect      *Connect  `json:"connect,omitempty"`
	ConnectTime  time.Time `json:"connect_time,omitzero"`
	Err          string    `json:"error,omitempty"`     // malformed CONNECT, broker not reachable
	Connack      int       `json:"connack"`             // CONNACK code of the broker; -1: none
	Publishes    []Pub     `json:"publishes,omitzero"`  // PUBLISH packets of the client
	Subscribed   time.Time `json:"subscribed,omitzero"` // first SUBACK of the broker to the client
	Disconnect   bool      `json:"disconnect"`          // the client sent DISCONNECT
	Dropped      bool      `json:"dropped"`             // ended by Drop
	Closed       time.Time `json:"closed,omitzero"`
}

// Pub is one PUBLISH of the client with its time of reception.
type Pub struct {
	Publish
	Time time.Time `json:"time"`
}

// Relayed reports whether the broker accepted the connection.
func (c Conn) Relayed() bool { return c.Phase == Relay && c.Connect != nil && c.Connack == 0 }

// Proxy is a running proxy attachment.
type Proxy struct {
	cfg    Config
	ln     net.Listener
	phases map[Phase]*tls.Config

	mu      sync.Mutex
	conns   []*Conn
	queue   []Phase
	live    map[int][2]net.Conn
	changed chan struct{}
	serving bool
	closed  bool
	wg      sync.WaitGroup
}

// New creates the certificates and binds the listener; Serve starts
// accepting.
func New(cfg Config) (*Proxy, error) {
	if cfg.CA == nil || cfg.CAKey == nil {
		return nil, errors.New("proxy: a CA certificate and its key are needed")
	}
	if !cfg.CA.IsCA {
		return nil, errors.New("proxy: the CA certificate is no CA (basic constraints)")
	}
	if len(cfg.Names) == 0 {
		cfg.Names = []string{"localhost", "127.0.0.1", "::1"}
	}
	if cfg.HandshakeTimeout <= 0 {
		cfg.HandshakeTimeout = 10 * time.Second
	}
	// A relayed client must not authenticate as D7 (C-113 RT-1).
	up := &tls.Config{}
	if cfg.UpstreamTLS != nil {
		up = cfg.UpstreamTLS.Clone()
	}
	up.Certificates, up.GetClientCertificate = nil, nil
	if cfg.UpstreamCert != nil {
		up.Certificates = []tls.Certificate{*cfg.UpstreamCert}
	}
	cfg.UpstreamTLS = up
	good, err := issue(cfg.CA, cfg.CAKey, cfg.Names)
	if err != nil {
		return nil, err
	}
	wrong, err := issue(cfg.CA, cfg.CAKey, []string{WrongHostName})
	if err != nil {
		return nil, err
	}
	otherKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	other, err := selfSigned(otherKey)
	if err != nil {
		return nil, err
	}
	untrusted, err := issue(other, otherKey, cfg.Names)
	if err != nil {
		return nil, err
	}
	p := &Proxy{cfg: cfg, live: map[int][2]net.Conn{}, changed: make(chan struct{}), phases: map[Phase]*tls.Config{
		Relay:       {MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{good}, ClientAuth: tls.RequestClientCert},
		TLS11:       {MinVersion: tls.VersionTLS11, MaxVersion: tls.VersionTLS11, Certificates: []tls.Certificate{good}, ClientAuth: tls.RequestClientCert},
		UntrustedCA: {MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{untrusted}, ClientAuth: tls.RequestClientCert},
		WrongHost:   {MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{wrong}, ClientAuth: tls.RequestClientCert},
	}}
	if p.ln, err = net.Listen("tcp", cfg.Listen); err != nil {
		return nil, fmt.Errorf("proxy: %w", err)
	}
	return p, nil
}

var (
	serialMu sync.Mutex
	serial   = time.Now().UnixNano()
)

func template(cn string) *x509.Certificate {
	serialMu.Lock()
	serial++
	s := serial
	serialMu.Unlock()
	now := time.Now()
	return &x509.Certificate{SerialNumber: big.NewInt(s), Subject: pkix.Name{CommonName: cn},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour)}
}

func selfSigned(k *ecdsa.PrivateKey) (*x509.Certificate, error) {
	tpl := template("oec-check proxy untrusted CA")
	tpl.IsCA, tpl.BasicConstraintsValid, tpl.KeyUsage = true, true, x509.KeyUsageCertSign
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &k.PublicKey, k)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(der)
}

// issue creates a server certificate for the names, signed by the CA.
func issue(ca *x509.Certificate, caKey crypto.Signer, names []string) (tls.Certificate, error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	tpl := template(names[0])
	for _, n := range names {
		if ip := net.ParseIP(n); ip != nil {
			tpl.IPAddresses = append(tpl.IPAddresses, ip)
		} else {
			tpl.DNSNames = append(tpl.DNSNames, n)
		}
	}
	tpl.ExtKeyUsage, tpl.KeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, x509.KeyUsageDigitalSignature
	der, err := x509.CreateCertificate(rand.Reader, tpl, ca, &k.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("proxy: issue the certificate for %v: %w", names, err)
	}
	return tls.Certificate{Certificate: [][]byte{der, ca.Raw}, PrivateKey: k}, nil
}

// Addr is the address the proxy listens on.
func (p *Proxy) Addr() string { return p.ln.Addr().String() }

// Serve starts accepting connections (once). Clients that connect earlier
// wait in the backlog of the listener.
func (p *Proxy) Serve() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.serving || p.closed {
		return
	}
	p.serving = true
	p.wg.Add(1)
	go p.accept()
}

// Close stops the proxy and ends every connection.
func (p *Proxy) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.ln.Close()
	for _, c := range p.live {
		closeAll(c)
	}
	p.mu.Unlock()
	p.wg.Wait()
}

func closeAll(c [2]net.Conn) {
	for _, x := range c {
		if x == nil {
			continue
		}
		if t, ok := x.(*tls.Conn); ok {
			t.NetConn().Close()
		} else {
			x.Close()
		}
	}
}

// Reject makes the next connection attempts use the phases in order
// instead of being relayed; Clear removes the phases not yet used.
func (p *Proxy) Reject(phases ...Phase) {
	p.mu.Lock()
	p.queue = append(p.queue, phases...)
	p.mu.Unlock()
}

// Clear removes the phases Reject queued and that no connection used.
func (p *Proxy) Clear() []Phase {
	p.mu.Lock()
	defer p.mu.Unlock()
	q := p.queue
	p.queue = nil
	return q
}

// Drop ends every relayed connection on both sides without DISCONNECT (the
// broker publishes the Will of the client) and returns their number.
func (p *Proxy) Drop() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for id, c := range p.live {
		if p.conns[id].Phase != Relay {
			continue
		}
		p.conns[id].Dropped = true
		closeAll(c)
		n++
	}
	p.notify()
	return n
}

// Conns returns a copy of the connection attempts so far.
func (p *Proxy) Conns() []Conn {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Conn, len(p.conns))
	for i, c := range p.conns {
		out[i] = *c
		out[i].Publishes = append([]Pub(nil), c.Publishes...)
	}
	return out
}

// Wait returns when done reports true for the connections (true) or ctx
// ends (false).
func (p *Proxy) Wait(ctx context.Context, done func([]Conn) bool) bool {
	for {
		p.mu.Lock()
		ch := p.changed
		p.mu.Unlock()
		if done(p.Conns()) {
			return true
		}
		select {
		case <-ch:
		case <-ctx.Done():
			return done(p.Conns())
		}
	}
}

// notify wakes the waiters; p.mu is held.
func (p *Proxy) notify() {
	close(p.changed)
	p.changed = make(chan struct{})
}

func (p *Proxy) update(fn func()) {
	p.mu.Lock()
	fn()
	p.notify()
	p.mu.Unlock()
}

func (p *Proxy) accept() {
	defer p.wg.Done()
	for {
		nc, err := p.ln.Accept()
		if err != nil {
			return
		}
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			nc.Close()
			return
		}
		phase := Relay
		if len(p.queue) > 0 {
			phase, p.queue = p.queue[0], p.queue[1:]
		}
		c := &Conn{ID: len(p.conns), Phase: phase, Remote: nc.RemoteAddr().String(), Accepted: time.Now().UTC(), Connack: -1}
		p.conns = append(p.conns, c)
		p.live[c.ID] = [2]net.Conn{nc, nil}
		p.notify()
		p.mu.Unlock()
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.serve(c, nc)
		}()
	}
}

func tlsVersion(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	}
	return fmt.Sprintf("0x%04x", v)
}

// serve handles one connection attempt.
func (p *Proxy) serve(c *Conn, nc net.Conn) {
	var up net.Conn
	defer func() {
		p.update(func() {
			delete(p.live, c.ID)
			c.Closed = time.Now().UTC()
		})
		nc.Close()
		if up != nil {
			up.Close()
		}
	}()
	tc := tls.Server(nc, p.phases[c.Phase])
	nc.SetDeadline(time.Now().Add(p.cfg.HandshakeTimeout))
	if err := tc.Handshake(); err != nil {
		p.update(func() { c.HandshakeErr = err.Error() })
		return
	}
	st := tc.ConnectionState()
	p.update(func() {
		c.TLSVersion, c.ClientCert = tlsVersion(st.Version), len(st.PeerCertificates) > 0
		p.live[c.ID] = [2]net.Conn{tc, nil}
	})
	first, err := readFrame(tc)
	if err != nil {
		p.update(func() { c.Err = "no CONNECT: " + err.Error() })
		return
	}
	cp, err := parseConnect(first)
	now := time.Now().UTC()
	p.update(func() {
		c.Connect, c.ConnectTime = cp, now
		if err != nil {
			c.Err = err.Error()
		}
	})
	if err != nil || c.Phase != Relay {
		return
	}
	nc.SetDeadline(time.Time{})
	d := tls.Dialer{NetDialer: &net.Dialer{Timeout: p.cfg.HandshakeTimeout}, Config: p.cfg.UpstreamTLS}
	upc, err := d.Dial("tcp", p.cfg.Upstream)
	if err != nil {
		p.update(func() { c.Err = "broker: " + err.Error() })
		return
	}
	up = upc
	p.mu.Lock()
	dropped := c.Dropped || p.closed
	if !dropped {
		p.live[c.ID] = [2]net.Conn{tc, up}
	}
	p.mu.Unlock()
	if dropped {
		return
	}
	if _, err := up.Write(first.bytes()); err != nil {
		return
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.down(c, up, tc)
	}()
	p.upward(c, tc, up, cp.Level)
	closeAll([2]net.Conn{tc, up})
	<-done
}

// upward relays the packets of the client to the broker and records the
// PUBLISH and DISCONNECT packets.
func (p *Proxy) upward(c *Conn, from, to net.Conn, level byte) {
	for {
		f, err := readFrame(from)
		if err != nil {
			return
		}
		switch f.kind() {
		case typePublish:
			pub, err := parsePublish(f, level)
			now := time.Now().UTC()
			p.update(func() {
				if err != nil {
					c.Err = err.Error()
					return
				}
				c.Publishes = append(c.Publishes, Pub{Publish: pub, Time: now})
			})
		case typeDisconnect:
			p.update(func() { c.Disconnect = true })
		}
		if _, err := to.Write(f.bytes()); err != nil {
			return
		}
	}
}

// down relays the packets of the broker to the client; it records the
// CONNACK code and the time of the first SUBACK.
func (p *Proxy) down(c *Conn, from, to net.Conn) {
	defer closeAll([2]net.Conn{from, to})
	f, err := readFrame(from)
	if err != nil {
		return
	}
	code, err := connackCode(f)
	p.update(func() {
		if err != nil {
			c.Err = err.Error()
			return
		}
		c.Connack = code
	})
	if _, err := to.Write(f.bytes()); err != nil {
		return
	}
	for {
		f, err := readFrame(from)
		if err != nil {
			return
		}
		if f.kind() == typeSuback {
			now := time.Now().UTC()
			p.update(func() {
				if c.Subscribed.IsZero() {
					c.Subscribed = now
				}
			})
		}
		if _, err := to.Write(f.bytes()); err != nil {
			return
		}
	}
}
