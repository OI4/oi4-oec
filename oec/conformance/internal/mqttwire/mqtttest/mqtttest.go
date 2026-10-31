// Package mqtttest is an in-process MQTT 3.1.1 test broker over TLS on the
// paho packet codec, with a throw-away PKI. It is used by tests only (the
// gate runs without network and container engine); the dev broker of
// oec/dev is the real counterpart in the opt-in live tests.
package mqtttest

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"

	"github.com/OI4/oec/conformance/internal/mqttwire"
)

// PKI holds the files of a throw-away CA, a broker certificate for
// localhost/127.0.0.1 and a client certificate.
type PKI struct {
	Dir                   string
	CAFile, OtherCAFile   string
	BrokerCert            tls.Certificate
	ClientCert, ClientKey string // unencrypted PKCS#8
	ClientKeyEncrypted    string // PKCS#8 PBES2 (PBKDF2-SHA256, AES-256-CBC)
	Passphrase            string
	// CA and CAKey issue the certificates; the proxy attachment of
	// oec-check test signs its server certificates with them in tests.
	CA        *x509.Certificate
	CAKey     *ecdsa.PrivateKey
	CAKeyFile string // PKCS#8 key of CA (PEM)
	// OtherCA (OtherCAFile) and its key issue certificates no client of
	// the PKI trusts (ServerCert).
	OtherCA    *x509.Certificate
	OtherCAKey *ecdsa.PrivateKey
}

func key(t testing.TB) *ecdsa.PrivateKey {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func write(t testing.TB, path, typ string, der []byte) {
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
}

// NewPKI creates the PKI in a temporary directory.
func NewPKI(t testing.TB) *PKI {
	dir := t.TempDir()
	p := &PKI{Dir: dir, Passphrase: "test-passphrase"}
	now := time.Now()
	mk := func(serial int64, cn string, ca bool, pub *ecdsa.PublicKey, parent *x509.Certificate, signer *ecdsa.PrivateKey, mod func(*x509.Certificate)) (*x509.Certificate, []byte) {
		tpl := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: cn}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour)}
		if ca {
			tpl.IsCA, tpl.BasicConstraintsValid, tpl.KeyUsage = true, true, x509.KeyUsageCertSign
		}
		if mod != nil {
			mod(tpl)
		}
		if parent == nil {
			parent = tpl
		}
		der, err := x509.CreateCertificate(rand.Reader, tpl, parent, pub, signer)
		if err != nil {
			t.Fatal(err)
		}
		c, _ := x509.ParseCertificate(der)
		return c, der
	}
	caKey := key(t)
	var caDER []byte
	p.CA, caDER = mk(1, "oec test CA", true, &caKey.PublicKey, nil, caKey, nil)
	p.CAKey = caKey
	p.CAFile = filepath.Join(dir, "ca.pem")
	write(t, p.CAFile, "CERTIFICATE", caDER)
	caKeyDER, err := x509.MarshalPKCS8PrivateKey(caKey)
	if err != nil {
		t.Fatal(err)
	}
	p.CAKeyFile = filepath.Join(dir, "ca.key")
	write(t, p.CAKeyFile, "PRIVATE KEY", caKeyDER)
	otherKey := key(t)
	var otherDER []byte
	p.OtherCA, otherDER = mk(2, "other CA", true, &otherKey.PublicKey, nil, otherKey, nil)
	p.OtherCAKey = otherKey
	p.OtherCAFile = filepath.Join(dir, "other-ca.pem")
	write(t, p.OtherCAFile, "CERTIFICATE", otherDER)

	bk := key(t)
	_, bDER := mk(3, "localhost", false, &bk.PublicKey, p.CA, caKey, func(c *x509.Certificate) {
		c.DNSNames, c.IPAddresses = []string{"localhost"}, []net.IP{net.IPv4(127, 0, 0, 1)}
		c.ExtKeyUsage, c.KeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, x509.KeyUsageDigitalSignature
	})
	p.BrokerCert = tls.Certificate{Certificate: [][]byte{bDER}, PrivateKey: bk}

	ck := key(t)
	_, cDER := mk(4, "oec-test-client", false, &ck.PublicKey, p.CA, caKey, func(c *x509.Certificate) {
		c.ExtKeyUsage, c.KeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, x509.KeyUsageDigitalSignature
	})
	p.ClientCert = filepath.Join(dir, "client.pem")
	write(t, p.ClientCert, "CERTIFICATE", cDER)
	ckDER, err := x509.MarshalPKCS8PrivateKey(ck)
	if err != nil {
		t.Fatal(err)
	}
	p.ClientKey = filepath.Join(dir, "client.key")
	write(t, p.ClientKey, "PRIVATE KEY", ckDER)
	p.ClientKeyEncrypted = filepath.Join(dir, "client.enc.key")
	write(t, p.ClientKeyEncrypted, "ENCRYPTED PRIVATE KEY", EncryptPKCS8(t, ckDER, p.Passphrase))
	return p
}

// Server certificate variants of ServerCert.
const (
	CertSelfSigned = "self-signed" // the broker certificate signs itself
	CertOtherCA    = "other-ca"    // signed by OtherCA, which no client trusts
	CertWrongHost  = "wrong-host"  // signed by CA for another host name only
	CertExpired    = "expired"     // signed by CA for localhost, expired
)

// ServerCert returns a broker certificate of a variant (Cert…); "" returns
// BrokerCert.
func (p *PKI) ServerCert(t testing.TB, kind string) tls.Certificate {
	if kind == "" {
		return p.BrokerCert
	}
	k := key(t)
	now := time.Now()
	tpl := &x509.Certificate{SerialNumber: big.NewInt(100), Subject: pkix.Name{CommonName: "localhost"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
		DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, KeyUsage: x509.KeyUsageDigitalSignature}
	parent, signer := p.CA, p.CAKey
	switch kind {
	case CertSelfSigned:
		parent, signer = tpl, k
	case CertOtherCA:
		parent, signer = p.OtherCA, p.OtherCAKey
	case CertWrongHost:
		tpl.Subject.CommonName, tpl.DNSNames, tpl.IPAddresses = "broker.invalid", []string{"broker.invalid"}, nil
	case CertExpired:
		tpl.NotBefore, tpl.NotAfter = now.Add(-30*time.Minute), now.Add(-time.Minute)
	default:
		t.Fatalf("unknown certificate variant %q", kind)
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, parent, &k.PublicKey, signer)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: k}
}

// EncryptPKCS8 encrypts a PKCS#8 key with PBES2 (PBKDF2-HMAC-SHA256, 2048
// iterations, AES-256-CBC) like `openssl pkcs8 -topk8 -v2 aes-256-cbc`
// (mqttwire.EncryptPKCS8).
func EncryptPKCS8(t testing.TB, der []byte, passphrase string) []byte {
	out, err := mqttwire.EncryptPKCS8(der, []byte(passphrase))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Broker is a minimal MQTT 3.1.1 broker (it also accepts the protocol level
// 3 of MQTT 3.1, as common brokers do): CONNECT with a password or a client
// certificate (or without credentials after AllowAnonymous), SUBSCRIBE (QoS 0/1, filters with + and #), PUBLISH (QoS 0/1,
// delivered in order with min(publish, subscribe) QoS and the retain flag
// 0), PING, DISCONNECT and the Will on a lost connection. No retained
// messages, no QoS 2, no sessions. Options adds the broker properties the
// infrastructure checks of oec-infra bus judge (users, ACL, TLS versions,
// certificate, a plain listener, size limits).
type Broker struct {
	Addr      string // 127.0.0.1:port
	PlainAddr string // 127.0.0.1:port of the plain listener (Options.Plain)
	Username  string
	Password  string
	ln, plain net.Listener
	opt       Options
	mu        sync.Mutex
	conns     map[*conn]bool
	anon      bool
	wg        sync.WaitGroup
}

// Access is the kind of an ACL check.
type Access int

// Kinds of ACL checks.
const (
	Read      Access = iota // delivery of a message to a subscriber
	Write                   // a PUBLISH of the client
	Subscribe               // a SUBSCRIBE filter
)

// Options of NewBrokerWith. The zero value of every field keeps the
// behavior of NewBroker.
type Options struct {
	Username, Password string
	Users              map[string]string // further user → password pairs
	RequireCert        bool              // only clients with a certificate of the PKI; the user is its CN
	MinTLS, MaxTLS     uint16            // TLS versions of the listener (default 1.2 … 1.3)
	Cert               *tls.Certificate  // broker certificate (default PKI.BrokerCert)
	Plain              bool              // also a plain-text listener (PlainAddr)
	// ACL decides an access of a user (nil: everything is allowed). A
	// denied PUBLISH is dropped (QoS 1: acknowledged) or, with
	// DisconnectDenied, ends the connection; a denied filter gets the
	// SUBACK return code 0x80; a denied Read is not delivered.
	ACL              func(user, topic string, a Access) bool
	DisconnectDenied bool
	MaxPayload       int    // > 0: drops (acknowledges) a PUBLISH with a larger payload
	MaxPacket        int    // > 0: ends the connection on a larger control packet (fixed header included)
	Levels           []byte // accepted protocol levels (default 3 and 4)
}

type conn struct {
	c      net.Conn
	out    chan packets.ControlPacket
	subs   map[string]byte
	will   *packets.PublishPacket
	nextID uint16
	user   string
}

// NewBroker starts a broker with TLS 1.2+ that accepts the user/password
// pair or – if requireCert – only clients with a certificate of the PKI.
func NewBroker(t testing.TB, p *PKI, user, password string, requireCert bool) *Broker {
	return NewBrokerWith(t, p, Options{Username: user, Password: password, RequireCert: requireCert})
}

// NewBrokerWith starts a broker with the options.
func NewBrokerWith(t testing.TB, p *PKI, o Options) *Broker {
	pool := x509.NewCertPool()
	pool.AddCert(p.CA)
	cert := p.BrokerCert
	if o.Cert != nil {
		cert = *o.Cert
	}
	tc := &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientCAs: pool}
	if o.MinTLS != 0 {
		tc.MinVersion = o.MinTLS
	}
	if o.MaxTLS != 0 {
		tc.MaxVersion = o.MaxTLS
	}
	if o.RequireCert {
		tc.ClientAuth = tls.RequireAndVerifyClientCert
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", tc)
	if err != nil {
		t.Fatal(err)
	}
	b := &Broker{Addr: ln.Addr().String(), Username: o.Username, Password: o.Password, ln: ln, opt: o, conns: map[*conn]bool{}}
	if o.Plain {
		if b.plain, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
			ln.Close()
			t.Fatal(err)
		}
		b.PlainAddr = b.plain.Addr().String()
		b.wg.Add(1)
		go b.accept(b.plain, false)
	}
	b.wg.Add(1)
	go b.accept(ln, o.RequireCert)
	t.Cleanup(b.Close)
	return b
}

// AllowAnonymous makes the broker also accept a CONNECT without user name
// and password (a component that does not authenticate, T7-CON-03).
func (b *Broker) AllowAnonymous() {
	b.mu.Lock()
	b.anon = true
	b.mu.Unlock()
}

// URL returns mqtts://localhost:<port> (the broker certificate names
// localhost).
func (b *Broker) URL() string {
	_, port, _ := net.SplitHostPort(b.Addr)
	return "mqtts://localhost:" + port
}

// Close stops the broker and drops all connections without Will.
func (b *Broker) Close() {
	b.ln.Close()
	if b.plain != nil {
		b.plain.Close()
	}
	b.mu.Lock()
	for c := range b.conns {
		c.will = nil
		c.c.Close()
	}
	b.mu.Unlock()
	b.wg.Wait()
}

func (b *Broker) accept(ln net.Listener, requireCert bool) {
	defer b.wg.Done()
	for {
		nc, err := ln.Accept()
		if err != nil {
			return
		}
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			b.serve(&conn{c: nc, out: make(chan packets.ControlPacket, 4096), subs: map[string]byte{}}, requireCert)
		}()
	}
}

// writer sends the queued packets in order.
func (c *conn) writer(done chan struct{}) {
	defer close(done)
	for p := range c.out {
		c.c.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if p.Write(c.c) != nil {
			c.c.Close()
		}
	}
}

func (b *Broker) serve(c *conn, requireCert bool) {
	done := make(chan struct{})
	go c.writer(done)
	defer func() {
		close(c.out)
		<-done
		c.c.Close()
	}()
	c.c.SetReadDeadline(time.Now().Add(10 * time.Second))
	p, err := packets.ReadPacket(c.c)
	if err != nil {
		return
	}
	cp, ok := p.(*packets.ConnectPacket)
	if !ok {
		return
	}
	ack := packets.NewControlPacket(packets.Connack).(*packets.ConnackPacket)
	b.mu.Lock()
	anon := b.anon && !cp.UsernameFlag && !cp.PasswordFlag
	b.mu.Unlock()
	switch {
	case !b.level(cp.ProtocolVersion):
		ack.ReturnCode = packets.ErrRefusedBadProtocolVersion
	case requireCert:
		if tc, ok := c.c.(*tls.Conn); ok && len(tc.ConnectionState().PeerCertificates) > 0 {
			c.user = tc.ConnectionState().PeerCertificates[0].Subject.CommonName
		}
	case anon:
	case cp.Username == b.Username && string(cp.Password) == b.Password:
		c.user = cp.Username
	case b.opt.Users != nil && b.opt.Users[cp.Username] == string(cp.Password) && cp.Username != "" && cp.PasswordFlag:
		c.user = cp.Username
	default:
		ack.ReturnCode = packets.ErrRefusedNotAuthorised
	}
	c.out <- ack
	if ack.ReturnCode != packets.Accepted {
		return
	}
	if cp.WillFlag {
		w := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
		w.TopicName, w.Payload, w.Qos, w.Retain = cp.WillTopic, cp.WillMessage, cp.WillQos, cp.WillRetain
		c.will = w
	}
	b.mu.Lock()
	b.conns[c] = true
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.conns, c)
		will := c.will
		b.mu.Unlock()
		if will != nil {
			b.route(will)
		}
	}()
	for {
		c.c.SetReadDeadline(time.Time{})
		if cp.Keepalive > 0 {
			c.c.SetReadDeadline(time.Now().Add(3 * time.Duration(cp.Keepalive) * time.Second / 2))
		}
		p, err := packets.ReadPacket(c.c)
		if err != nil {
			return
		}
		if b.opt.MaxPacket > 0 && packetSize(p) > b.opt.MaxPacket {
			return
		}
		switch p := p.(type) {
		case *packets.SubscribePacket:
			ack := packets.NewControlPacket(packets.Suback).(*packets.SubackPacket)
			ack.MessageID = p.MessageID
			b.mu.Lock()
			for i, f := range p.Topics {
				if !b.allowed(c.user, f, Subscribe) {
					ack.ReturnCodes = append(ack.ReturnCodes, 0x80)
					continue
				}
				q := min(p.Qoss[i], 1)
				c.subs[f] = q
				ack.ReturnCodes = append(ack.ReturnCodes, q)
			}
			b.mu.Unlock()
			c.out <- ack
		case *packets.PublishPacket:
			denied := !b.allowed(c.user, p.TopicName, Write)
			if denied && b.opt.DisconnectDenied {
				return
			}
			if p.Qos == 1 {
				ack := packets.NewControlPacket(packets.Puback).(*packets.PubackPacket)
				ack.MessageID = p.MessageID
				c.out <- ack
			}
			if denied || b.opt.MaxPayload > 0 && len(p.Payload) > b.opt.MaxPayload {
				continue
			}
			b.route(p)
		case *packets.PingreqPacket:
			c.out <- packets.NewControlPacket(packets.Pingresp)
		case *packets.DisconnectPacket:
			b.mu.Lock()
			c.will = nil
			b.mu.Unlock()
			return
		}
	}
}

func (b *Broker) route(p *packets.PublishPacket) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for c := range b.conns {
		best, found := byte(0), false
		for f, q := range c.subs {
			if Match(f, p.TopicName) {
				best, found = max(best, q), true
			}
		}
		if !found || !b.allowed(c.user, p.TopicName, Read) {
			continue
		}
		out := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
		out.TopicName, out.Payload, out.Qos = p.TopicName, p.Payload, min(p.Qos, best)
		if out.Qos > 0 {
			c.nextID++
			if c.nextID == 0 {
				c.nextID = 1
			}
			out.MessageID = c.nextID
		}
		c.out <- out
	}
}

func (b *Broker) level(v byte) bool {
	if len(b.opt.Levels) == 0 {
		return v == 3 || v == 4
	}
	for _, l := range b.opt.Levels {
		if l == v {
			return true
		}
	}
	return false
}

func (b *Broker) allowed(user, topic string, a Access) bool {
	return b.opt.ACL == nil || b.opt.ACL(user, topic, a)
}

// packetSize is the size of a control packet on the wire (fixed header
// included).
func packetSize(p packets.ControlPacket) int {
	var buf bytes.Buffer
	if p.Write(&buf) != nil {
		return 0
	}
	return buf.Len()
}

// Subscribers counts the connections subscribed with exactly this filter.
func (b *Broker) Subscribers(filter string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := 0
	for c := range b.conns {
		if _, ok := c.subs[filter]; ok {
			n++
		}
	}
	return n
}

// Match reports whether an MQTT topic filter matches a topic.
func Match(filter, topic string) bool {
	f, t := strings.Split(filter, "/"), strings.Split(topic, "/")
	for i, l := range f {
		if l == "#" {
			return true
		}
		if i >= len(t) || l != "+" && l != t[i] {
			return false
		}
	}
	return len(f) == len(t)
}
