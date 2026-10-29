package bus

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"

	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/probe"
	"github.com/OI4/oec/conformance/internal/report"
)

var tlsNames = map[uint16]string{tls.VersionTLS10: "1.0", tls.VersionTLS11: "1.1", tls.VersionTLS12: "1.2", tls.VersionTLS13: "1.3"}

// transport is the part of T8-BUS-01 without credentials: the TLS versions
// of the secure port (REQ-MQTT-006), plain-text listeners (REQ-MQTT-006)
// and the broker certificate (REQ-MQTT-007).
func (r *runner) transport(ctx context.Context, host, addr string) {
	d := &net.Dialer{Timeout: r.c.Timeout}
	nc, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		r.inconclusive(TProtocol, []string{reqTLSOnly, reqCert}, "the broker %s is not reachable: %v", addr, err)
		return
	}
	nc.Close()
	// The probes present the client certificate of the run: a listener
	// that requires one fails a TLS 1.2 (or 1.1) handshake without it.
	var client []tls.Certificate
	if r.c.Main.CertFile != "" {
		tc, err := mqttwire.TLSConfig(r.c.Main, host)
		if err != nil {
			r.inconclusive(TProtocol, []string{reqTLSOnly, reqCert}, "client certificate: %v", err)
			return
		}
		client = tc.Certificates
	}
	var certs []*x509.Certificate
	var errs []string
	for _, v := range []uint16{tls.VersionTLS12, tls.VersionTLS13} {
		cs, err := handshake(ctx, d, addr, host, client, v, v)
		if err != nil {
			errs = append(errs, fmt.Sprintf("TLS %s: %v", tlsNames[v], err))
			continue
		}
		r.info.TLS = append(r.info.TLS, tlsNames[v])
		if certs == nil {
			certs = cs.PeerCertificates
		}
	}
	if len(r.info.TLS) == 0 {
		r.violation(TProtocol, reqTLSOnly, "", "%s accepts no TLS 1.2 or 1.3 connection (%s)", addr, strings.Join(errs, "; "))
	} else {
		r.checked(TProtocol, reqTLSOnly, "%s accepts TLS %s", addr, strings.Join(r.info.TLS, ", "))
	}
	if cs, err := handshake(ctx, d, addr, host, client, tls.VersionTLS10, tls.VersionTLS11); err == nil {
		r.violation(TProtocol, reqTLSOnly, "", "%s accepts a TLS %s connection (TLS 1.2 or higher only)", addr, tlsNames[cs.Version])
	} else {
		r.checked(TProtocol, reqTLSOnly, "%s refuses TLS 1.0 and 1.1", addr)
	}
	for _, p := range r.c.PlainPorts {
		r.plain(ctx, d, net.JoinHostPort(host, strconv.Itoa(p)))
	}
	if len(certs) == 0 {
		r.inconclusive(TProtocol, []string{reqCert}, "no TLS handshake with %s: the broker certificate is unknown", addr)
		return
	}
	r.certificate(host, certs)
}

// handshake opens a TLS connection without verifying the broker (the
// certificate is judged separately), with the client certificate if given.
func handshake(ctx context.Context, d *net.Dialer, addr, host string, client []tls.Certificate, minV, maxV uint16) (tls.ConnectionState, error) {
	td := tls.Dialer{NetDialer: d, Config: &tls.Config{ServerName: host, MinVersion: minV, MaxVersion: maxV, InsecureSkipVerify: true, Certificates: client}} //nolint:gosec // the certificate is checked in certificate()
	c, err := td.DialContext(ctx, "tcp", addr)
	if err != nil {
		return tls.ConnectionState{}, err
	}
	defer c.Close()
	return c.(*tls.Conn).ConnectionState(), nil
}

// plain probes one port for a plain-text MQTT listener: an anonymous
// CONNECT without credentials; any CONNACK is a listener D8 must not find.
func (r *runner) plain(ctx context.Context, d *net.Dialer, addr string) {
	nc, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		r.checked(TProtocol, reqTLSOnly, "no listener on %s", addr)
		return
	}
	defer nc.Close()
	_ = nc.SetDeadline(time.Now().Add(r.c.Timeout))
	cp := packets.NewControlPacket(packets.Connect).(*packets.ConnectPacket)
	cp.ProtocolName, cp.ProtocolVersion, cp.CleanSession, cp.Keepalive = "MQTT", 4, true, 10
	cp.ClientIdentifier = mqttwire.ClientID("oec-infra-plain-")
	if err := cp.Write(nc); err != nil {
		r.checked(TProtocol, reqTLSOnly, "%s closed the connection: %v", addr, err)
		return
	}
	p, err := packets.ReadPacket(nc)
	ack, ok := p.(*packets.ConnackPacket)
	if err != nil || !ok {
		r.note(TProtocol, reqTLSOnly, "%s is open but answers an MQTT CONNECT without CONNACK", addr)
		return
	}
	if ack.ReturnCode == packets.Accepted {
		_ = packets.NewControlPacket(packets.Disconnect).Write(nc)
	}
	r.violation(TProtocol, reqTLSOnly, "", "plain-text MQTT listener on %s (CONNACK return code %d to an anonymous CONNECT)", addr, ack.ReturnCode)
}

// certificate judges the broker certificate (REQ-MQTT-007): not
// self-signed, a chain to the CA of the client (--ca, broker_ca.pem or
// ca.pem), and – proposed interpretation of SF-115 – valid for the broker
// address and at the time of the run.
func (r *runner) certificate(host string, certs []*x509.Certificate) {
	leaf := certs[0]
	c := &report.Certificate{Subject: leaf.Subject.String(), Issuer: leaf.Issuer.String(), NotBefore: leaf.NotBefore.UTC(), NotAfter: leaf.NotAfter.UTC()}
	c.Names = append(c.Names, leaf.DNSNames...)
	for _, ip := range leaf.IPAddresses {
		c.Names = append(c.Names, ip.String())
	}
	r.info.Certificate = c
	if bytes.Equal(leaf.RawIssuer, leaf.RawSubject) && leaf.CheckSignatureFrom(leaf) == nil {
		r.violation(TProtocol, reqCert, "", "the broker certificate %q is self-signed", c.Subject)
		return
	}
	now := time.Now()
	at := now
	switch {
	case now.Before(leaf.NotBefore):
		r.violation(TProtocol, reqCert, "", "the broker certificate is not valid before %s (SF-115, proposed)", c.NotBefore.Format(time.RFC3339))
		at = leaf.NotBefore.Add(time.Second)
	case now.After(leaf.NotAfter):
		r.violation(TProtocol, reqCert, "", "the broker certificate expired at %s (SF-115, proposed)", c.NotAfter.Format(time.RFC3339))
		at = leaf.NotAfter.Add(-time.Second)
	default:
		r.checked(TProtocol, reqCert, "the broker certificate is valid until %s", c.NotAfter.Format(time.RFC3339))
	}
	if err := leaf.VerifyHostname(host); err != nil {
		r.violation(TProtocol, reqCert, "", "the broker certificate is not valid for the broker address %s (SF-115, proposed): %v", host, err)
	} else {
		r.checked(TProtocol, reqCert, "the broker certificate is valid for %s", host)
	}
	var roots *x509.CertPool
	caName := r.c.Main.CAFile
	switch {
	case caName != "":
		raw, err := os.ReadFile(caName)
		if err != nil {
			r.inconclusive(TProtocol, []string{reqCert}, "CA %s: %v", caName, err)
			return
		}
		roots = x509.NewCertPool()
		if !roots.AppendCertsFromPEM(raw) {
			r.inconclusive(TProtocol, []string{reqCert}, "CA %s: no PEM certificate", caName)
			return
		}
	case r.c.Main.Insecure:
		r.inconclusive(TProtocol, []string{reqCert}, "--insecure without --ca: the chain of the broker certificate is not checked")
		return
	default:
		caName = "the system roots"
	}
	inter := x509.NewCertPool()
	for _, ic := range certs[1:] {
		inter.AddCert(ic)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: roots, Intermediates: inter, CurrentTime: at, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
		r.violation(TProtocol, reqCert, "", "the broker certificate %q does not chain to %s: %v", c.Subject, caName, err)
		return
	}
	r.checked(TProtocol, reqCert, "the broker certificate chains to %s", caName)
}

// protocol judges the MQTT 3.1.1 CONNECT of D8 (REQ-MQTT-002).
func (r *runner) protocol(err error) {
	switch {
	case err == nil:
		r.checked(TProtocol, reqMQTT311, "the broker accepts an MQTT 3.1.1 connection")
	case errors.Is(err, packets.ErrorRefusedBadProtocolVersion):
		r.violation(TProtocol, reqMQTT311, "", "the broker refuses MQTT 3.1.1: %v", err)
	default:
		r.inconclusive(TProtocol, []string{reqMQTT311}, "D8 cannot connect to the broker: %v", err)
	}
}

// packet sends one PUBLISH whose payload is exactly MaxPacketSize on the
// longest topic MQTT allows (REQ-CNT-003, RIG 4.1.1, SF-105): the broker
// accepts and delivers it without closing the connection. It uses its own
// connection, so a disconnect does not affect the other tests.
func (r *runner) packet(ctx context.Context) {
	size := r.c.MaxPacketSize
	if size == 0 {
		size = MinMaxPacketSize
		r.note(TProtocol, reqPacket, "no MaxPacketSize (broker.json or --max-packet-size): probed with the minimum %d of RIG 4.1.1", size)
	}
	r.info.MaxPacketSize = size
	if size > r.c.MaxProbeSize {
		r.inconclusive(TProtocol, []string{reqPacket}, "MaxPacketSize %d is larger than --max-probe-size %d: not probed", size, r.c.MaxProbeSize)
		return
	}
	topic := probe.Topic(ServiceType, r.probe, "Pub", "Data", r.probe, "MaxPacketSize-")
	if n := min(topicMax, remainingMax-publishOverhead-size); len(topic) < n {
		topic += strings.Repeat("x", n-len(topic))
	}
	payload := padded(size, ServiceType+"/"+r.probe)
	got := make(chan int, 1)
	wait := r.c.Timeout + time.Duration(size>>20)*time.Second
	cctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	cl, err := r.dial(cctx, r.c.Main, "oec-infra-size-", nil)
	if err != nil {
		r.inconclusive(TProtocol, []string{reqPacket}, "second connection of D8: %v", err)
		return
	}
	defer cl.Kill()
	filter := probe.Topic(ServiceType, r.probe, "Pub", "Data", "", "") + "/#"
	if err := cl.Subscribe(cctx, filter, 1, func(m mqttwire.Message) {
		if m.Topic == topic {
			select {
			case got <- len(m.Payload):
			default:
			}
		}
	}); err != nil {
		r.inconclusive(TProtocol, []string{reqPacket}, "%v", err)
		return
	}
	short := topic[:min(len(topic), 120)] + "…"
	pubErr := make(chan error, 1)
	go func() { pubErr <- cl.Publish(cctx, mqttwire.Message{Topic: topic, Payload: payload, QoS: 1}) }()
	for {
		select {
		case n := <-got:
			if n != size {
				r.violation(TProtocol, reqPacket, short, "the broker delivered %d of %d payload bytes", n, size)
				return
			}
			r.checked(TProtocol, reqPacket, "a payload of MaxPacketSize %d bytes on a topic of %d bytes was delivered", size, len(topic))
			return
		case err := <-cl.Lost():
			r.violation(TProtocol, reqPacket, short, "the broker closed the connection after a PUBLISH with a payload of MaxPacketSize %d bytes on a topic of %d bytes: %v", size, len(topic), err)
			return
		case err := <-pubErr:
			if err != nil && cctx.Err() == nil {
				r.violation(TProtocol, reqPacket, short, "a PUBLISH with a payload of MaxPacketSize %d bytes on a topic of %d bytes failed: %v", size, len(topic), err)
				return
			}
		case <-cctx.Done():
			if ctx.Err() != nil {
				r.inconclusive(TProtocol, []string{reqPacket}, "the run was cancelled")
				return
			}
			r.violation(TProtocol, reqPacket, short, "a PUBLISH with a payload of MaxPacketSize %d bytes on a topic of %d bytes was not delivered within %s", size, len(topic), wait)
			return
		}
	}
}

// padded returns a Data NetworkMessage of exactly size bytes (a probe
// DataSet of D8 with a padding value).
func padded(size int, pid string) []byte {
	doc := map[string]any{"MessageId": fmt.Sprintf("%d-%s", time.Now().UnixMilli(), pid), "MessageType": "ua-data", "PublisherId": pid,
		"DataSetClassId": "0a8d7d3e-5c39-4b8c-9d1e-0ec1f0a0d800", "Messages": []any{map[string]any{"DataSetWriterId": 20, "DataSetWriterName": strings.TrimPrefix(pid, ServiceType+"/"), "Payload": map[string]any{"Padding": "@"}}}}
	raw, _ := json.Marshal(doc)
	if len(raw) > size {
		return bytes.Repeat([]byte("x"), size)
	}
	return bytes.Replace(raw, []byte(`"@"`), []byte(`"`+strings.Repeat("x", size-len(raw)+1)+`"`), 1)
}
