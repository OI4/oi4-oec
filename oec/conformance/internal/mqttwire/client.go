// Package mqttwire is the MQTT layer of D7 (DESIGN.md §4, ADR-0025): an
// MQTT 3.1.1 client on top of github.com/eclipse/paho.mqtt.golang (taken
// under EDL-1.0) with TLS 1.2/1.3 only – broker verification against a CA
// file, login with credentials or a client certificate (also an encrypted
// PKCS#8 key) – and the reader of a container storage (RIG 4.1). The proxy
// attachment of `test` (DESIGN.md §3.4) is internal/proxy.
package mqttwire

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

// DefaultPort is the MQTT over TLS port (IANA).
const DefaultPort = 8883

// Config describes one connection. There is no plain TCP: every connection
// uses TLS 1.2 or 1.3 (REQ-MQTT-009, G-4.5).
type Config struct {
	Broker        string // mqtts://host[:port] (also ssl:// and tls://)
	CAFile        string // PEM CA bundle that verifies the broker; empty: system roots
	CertFile      string // client certificate (PEM)
	KeyFile       string // private key (PEM; ENCRYPTED PRIVATE KEY needs KeyPassphrase)
	KeyPassphrase []byte
	Username      string
	Password      []byte
	// Insecure skips the verification of the broker certificate. Dev only;
	// the caller warns loudly and marks its report (G-4.5).
	Insecure       bool
	ClientID       string        // default: oec-check-<random>
	KeepAlive      time.Duration // default 60 s (REQ-MQTT-014)
	ConnectTimeout time.Duration // default 10 s
	Will           *Message      // registered in CONNECT (simulated components)
	// Deviations of simulated components (T7-CON-01, T7-CON-02): the
	// protocol level of the CONNECT (default 4, MQTT 3.1.1; 3 is MQTT 3.1)
	// and the lowest TLS version offered (default TLS 1.2).
	ProtocolVersion uint
	MinTLSVersion   uint16
	// Dialer opens the TCP connection to the broker address host:port
	// (TLS runs on top, verified for host); nil: net.Dialer. The simulated
	// application of the D8 self-test maps the broker host to loopback.
	Dialer func(ctx context.Context, network, addr string) (net.Conn, error)
}

// Message is one PUBLISH.
type Message struct {
	Topic    string
	Payload  []byte
	QoS      byte
	Retain   bool
	Received time.Time // set on reception
}

// BrokerAddress parses the broker URL into host:port. Plain MQTT schemes are
// refused.
func BrokerAddress(broker string) (host, addr string, err error) {
	u, err := url.Parse(broker)
	if err != nil || u.Host == "" {
		return "", "", fmt.Errorf("broker %q: not a URL mqtts://host[:port]", broker)
	}
	switch u.Scheme {
	case "mqtts", "ssl", "tls":
	case "mqtt", "tcp", "ws", "wss":
		return "", "", fmt.Errorf("broker %q: scheme %s is not supported; OEC uses MQTT over TLS (mqtts://)", broker, u.Scheme)
	default:
		return "", "", fmt.Errorf("broker %q: unknown scheme %q (mqtts://)", broker, u.Scheme)
	}
	if u.Path != "" && u.Path != "/" || u.User != nil || u.RawQuery != "" {
		return "", "", fmt.Errorf("broker %q: only mqtts://host[:port]", broker)
	}
	host, port := u.Hostname(), u.Port()
	if port == "" {
		port = strconv.Itoa(DefaultPort)
	}
	return host, net.JoinHostPort(host, port), nil
}

// TLSConfig builds the TLS configuration of a connection: TLS 1.2 or 1.3,
// the broker verified against CAFile (or the system roots), the client
// certificate if given.
func TLSConfig(c Config, serverName string) (*tls.Config, error) {
	tc := &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13, ServerName: serverName, InsecureSkipVerify: c.Insecure}
	if c.MinTLSVersion != 0 {
		tc.MinVersion = c.MinTLSVersion
	}
	if c.CAFile != "" {
		pemData, err := os.ReadFile(c.CAFile)
		if err != nil {
			return nil, err
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemData) {
			return nil, fmt.Errorf("%s: no PEM certificate", c.CAFile)
		}
		tc.RootCAs = pool
	}
	switch {
	case c.CertFile != "" && c.KeyFile != "":
		certPEM, err := os.ReadFile(c.CertFile)
		if err != nil {
			return nil, err
		}
		keyData, err := os.ReadFile(c.KeyFile)
		if err != nil {
			return nil, err
		}
		kp, err := keyPEM(keyData, c.KeyPassphrase)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", c.KeyFile, err)
		}
		cert, err := tls.X509KeyPair(certPEM, kp)
		if err != nil {
			return nil, fmt.Errorf("client certificate %s / key %s: %w", c.CertFile, c.KeyFile, err)
		}
		tc.Certificates = []tls.Certificate{cert}
	case c.CertFile != "" || c.KeyFile != "":
		return nil, errors.New("a client certificate needs both --cert and --key")
	}
	return tc, nil
}

// Client is a connected MQTT client.
type Client struct {
	pc   paho.Client
	lost chan error

	mu   sync.Mutex
	conn net.Conn
}

// ClientID returns a random client identifier with the given prefix.
func ClientID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// Dial connects (clean session, no automatic reconnect: a monitor reports a
// lost connection instead of hiding it).
func Dial(ctx context.Context, c Config) (*Client, error) {
	host, addr, err := BrokerAddress(c.Broker)
	if err != nil {
		return nil, err
	}
	tc, err := TLSConfig(c, host)
	if err != nil {
		return nil, err
	}
	if c.ClientID == "" {
		c.ClientID = ClientID("oec-check-")
	}
	if c.KeepAlive == 0 {
		c.KeepAlive = 60 * time.Second
	}
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = 10 * time.Second
	}
	if c.ProtocolVersion == 0 {
		c.ProtocolVersion = 4
	}
	cl := &Client{lost: make(chan error, 1)}
	o := paho.NewClientOptions().
		AddBroker("tls://" + addr).
		SetProtocolVersion(c.ProtocolVersion).
		SetClientID(c.ClientID).
		SetCleanSession(true).
		SetKeepAlive(c.KeepAlive).
		SetConnectTimeout(c.ConnectTimeout).
		SetAutoReconnect(false).
		SetConnectRetry(false).
		SetOrderMatters(true).
		SetCustomOpenConnectionFn(func(u *url.URL, _ paho.ClientOptions) (net.Conn, error) {
			var conn net.Conn
			if c.Dialer != nil {
				dctx, cancel := context.WithTimeout(ctx, c.ConnectTimeout)
				defer cancel()
				raw, err := c.Dialer(dctx, "tcp", u.Host)
				if err != nil {
					return nil, err
				}
				tlc := tls.Client(raw, tc)
				if err := tlc.HandshakeContext(dctx); err != nil {
					raw.Close()
					return nil, err
				}
				conn = tlc
			} else {
				d := tls.Dialer{NetDialer: &net.Dialer{Timeout: c.ConnectTimeout}, Config: tc}
				var err error
				if conn, err = d.DialContext(ctx, "tcp", u.Host); err != nil {
					return nil, err
				}
			}
			cl.mu.Lock()
			cl.conn = conn
			cl.mu.Unlock()
			return conn, nil
		}).
		SetConnectionLostHandler(func(_ paho.Client, err error) {
			select {
			case cl.lost <- err:
			default:
			}
		})
	if c.Username != "" {
		o.SetUsername(c.Username)
		o.SetPassword(string(c.Password))
	}
	if c.Will != nil {
		o.SetBinaryWill(c.Will.Topic, c.Will.Payload, c.Will.QoS, c.Will.Retain)
	}
	cl.pc = paho.NewClient(o)
	tok := cl.pc.Connect()
	select {
	case <-tok.Done():
	case <-ctx.Done():
		cl.Kill()
		return nil, ctx.Err()
	}
	if err := tok.Error(); err != nil {
		return nil, fmt.Errorf("connect %s: %w", c.Broker, err)
	}
	return cl, nil
}

func wait(ctx context.Context, tok paho.Token, what string) error {
	select {
	case <-tok.Done():
		if err := tok.Error(); err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Subscribe subscribes to a topic filter. fn is called in the order of
// reception, one message at a time.
func (cl *Client) Subscribe(ctx context.Context, filter string, qos byte, fn func(Message)) error {
	tok := cl.pc.Subscribe(filter, qos, func(_ paho.Client, m paho.Message) {
		fn(Message{Topic: m.Topic(), Payload: m.Payload(), QoS: m.Qos(), Retain: m.Retained(), Received: time.Now()})
	})
	if err := wait(ctx, tok, "subscribe "+filter); err != nil {
		return err
	}
	if st, ok := tok.(*paho.SubscribeToken); ok {
		for f, code := range st.Result() {
			if code == 0x80 {
				return fmt.Errorf("subscribe %s: refused by the broker", f)
			}
		}
	}
	return nil
}

// Publish sends one message and waits for its acknowledgement (QoS 1/2).
func (cl *Client) Publish(ctx context.Context, m Message) error {
	return wait(ctx, cl.pc.Publish(m.Topic, m.QoS, m.Retain, m.Payload), "publish "+m.Topic)
}

// Lost delivers the error of a lost connection.
func (cl *Client) Lost() <-chan error { return cl.lost }

// Close sends DISCONNECT and closes the connection (the broker discards the
// Will).
func (cl *Client) Close() { cl.pc.Disconnect(250) }

// Kill closes the network connection without DISCONNECT, so the broker
// publishes the Will (simulated components and tests).
func (cl *Client) Kill() {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	if cl.conn != nil {
		cl.conn.Close()
	}
}
