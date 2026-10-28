// Package mqtttest is an in-process MQTT 3.1.1 test broker over TLS on the
// paho packet codec, with a throw-away PKI. It is used by tests only (the
// gate runs without network and container engine); the dev broker of
// oec/dev is the real counterpart in the opt-in live tests.
package mqtttest

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
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
	_, otherDER := mk(2, "other CA", true, &otherKey.PublicKey, nil, otherKey, nil)
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

// EncryptPKCS8 encrypts a PKCS#8 key with PBES2 (PBKDF2-HMAC-SHA256, 2048
// iterations, AES-256-CBC) like `openssl pkcs8 -topk8 -v2 aes-256-cbc`.
func EncryptPKCS8(t testing.TB, der []byte, passphrase string) []byte {
	type alg struct {
		Algorithm  asn1.ObjectIdentifier
		Parameters asn1.RawValue `asn1:"optional"`
	}
	raw := func(v any) asn1.RawValue {
		b, err := asn1.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return asn1.RawValue{FullBytes: b}
	}
	salt, iv := make([]byte, 16), make([]byte, aes.BlockSize)
	rand.Read(salt)
	rand.Read(iv)
	dk, err := pbkdf2.Key(sha256.New, passphrase, salt, 2048, 32)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := aes.NewCipher(dk)
	pad := aes.BlockSize - len(der)%aes.BlockSize
	plain := append(append([]byte{}, der...), bytes.Repeat([]byte{byte(pad)}, pad)...)
	enc := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(enc, plain)
	null := asn1.RawValue{FullBytes: []byte{5, 0}}
	kdf := raw(struct {
		Salt []byte
		Iter int
		PRF  alg
	}{salt, 2048, alg{asn1.ObjectIdentifier{1, 2, 840, 113549, 2, 9}, null}})
	params := raw(struct{ KDF, Enc alg }{
		alg{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 12}, kdf},
		alg{asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 42}, raw(iv)},
	})
	out, err := asn1.Marshal(struct {
		Alg  alg
		Data []byte
	}{alg{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}, params}, enc})
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
// messages, no QoS 2, no sessions.
type Broker struct {
	Addr     string // 127.0.0.1:port
	Username string
	Password string
	ln       net.Listener
	mu       sync.Mutex
	conns    map[*conn]bool
	anon     bool
	wg       sync.WaitGroup
}

type conn struct {
	c      net.Conn
	out    chan packets.ControlPacket
	subs   map[string]byte
	will   *packets.PublishPacket
	nextID uint16
}

// NewBroker starts a broker with TLS 1.2+ that accepts the user/password
// pair or – if requireCert – only clients with a certificate of the PKI.
func NewBroker(t testing.TB, p *PKI, user, password string, requireCert bool) *Broker {
	pool := x509.NewCertPool()
	pool.AddCert(p.CA)
	tc := &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{p.BrokerCert}, ClientCAs: pool}
	if requireCert {
		tc.ClientAuth = tls.RequireAndVerifyClientCert
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", tc)
	if err != nil {
		t.Fatal(err)
	}
	b := &Broker{Addr: ln.Addr().String(), Username: user, Password: password, ln: ln, conns: map[*conn]bool{}}
	b.wg.Add(1)
	go b.accept(requireCert)
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
	b.mu.Lock()
	for c := range b.conns {
		c.will = nil
		c.c.Close()
	}
	b.mu.Unlock()
	b.wg.Wait()
}

func (b *Broker) accept(requireCert bool) {
	defer b.wg.Done()
	for {
		nc, err := b.ln.Accept()
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
	case cp.ProtocolVersion != 4 && cp.ProtocolVersion != 3:
		ack.ReturnCode = packets.ErrRefusedBadProtocolVersion
	case !requireCert && !anon && (cp.Username != b.Username || string(cp.Password) != b.Password):
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
		switch p := p.(type) {
		case *packets.SubscribePacket:
			ack := packets.NewControlPacket(packets.Suback).(*packets.SubackPacket)
			ack.MessageID = p.MessageID
			b.mu.Lock()
			for i, f := range p.Topics {
				q := min(p.Qoss[i], 1)
				c.subs[f] = q
				ack.ReturnCodes = append(ack.ReturnCodes, q)
			}
			b.mu.Unlock()
			c.out <- ack
		case *packets.PublishPacket:
			if p.Qos == 1 {
				ack := packets.NewControlPacket(packets.Puback).(*packets.PubackPacket)
				ack.MessageID = p.MessageID
				c.out <- ack
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
		if !found {
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
