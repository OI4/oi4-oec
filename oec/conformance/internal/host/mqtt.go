package host

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	schema "github.com/OI4/oec/schema"
)

// brokerSchema is the D1 schema of broker.json (REQ-CNT-003).
const brokerSchema = schema.BaseURI + "platform/broker.schema.json"

var mqttReqs = []string{reqMQTTStorage, reqBrokerJSON, reqAddress}

// messageBusStorage judges T8-HST-01: the Message Bus storage with
// broker.pem, broker.json and – if the broker certificate was signed by a
// CA other than ca.pem – broker_ca.pem, mounted read-only (RIG 4.1.1).
func (r *runner) messageBusStorage() {
	s := r.mqtt
	switch {
	case s.err != nil:
		r.inconclusive(TMQTT, mqttReqs, s.dir, "%s", unreadable(s.dir, s.err))
		return
	case !s.exists:
		r.violation(TMQTT, reqMQTTStorage, s.dir, "the Message Bus storage %s does not exist (RIG 4.1.1)", s.dir)
		r.violation(TMQTT, reqAddress, s.dir, "no broker.json: the address of the broker is not provided (RIG 4.1.1)")
		return
	}
	r.checked(TMQTT, reqMQTTStorage, s.dir, "Message Bus storage %s", s.dir)
	r.judgeMount(TMQTT, reqMQTTStorage, s)
	r.worldWritable(TMQTT, reqMQTTStorage, s)
	r.brokerJSON()
	r.brokerCertificate()
}

// BrokerJSON judges the content of broker.json against the D1 schema
// platform/broker.schema.json (REQ-CNT-003) and its Address (REQ-MQTT-003);
// it returns the findings as observations of test.
func (r *runner) brokerJSON() {
	s := r.mqtt
	file := s.path("broker.json")
	data, ok, err := s.read("broker.json")
	switch {
	case err != nil:
		r.inconclusive(TMQTT, []string{reqBrokerJSON, reqAddress}, file, "%s", unreadable(file, err))
		return
	case !ok:
		r.violation(TMQTT, reqMQTTStorage, file, "no broker.json in the Message Bus storage (RIG 4.1.1)")
		r.violation(TMQTT, reqAddress, file, "no broker.json: the address of the broker is not provided (RIG 4.1.1)")
		return
	}
	if err := r.judgeBrokerJSON(TMQTT, file, data); err != nil {
		r.inconclusive(TMQTT, []string{reqBrokerJSON, reqAddress}, file, "%v", err)
	}
}

// judgeBrokerJSON judges one content of broker.json (also used for the
// V-BRK vectors). An error is an internal one (the schemas).
func (r *runner) judgeBrokerJSON(test, file string, data []byte) error {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		r.violation(test, reqBrokerJSON, file, "broker.json is no JSON document: %v", err)
		r.violation(test, reqAddress, file, "broker.json is no JSON document: the address of the broker is not provided")
		return nil
	}
	set, err := r.loadSchemas()
	if err != nil {
		return err
	}
	fs, err := set.Validate(brokerSchema, inst)
	if err != nil {
		return err
	}
	for _, f := range fs {
		req := f.Req
		if req == "" {
			req = reqBrokerJSON
		}
		where := f.Pointer
		if where == "" {
			where = "/"
		}
		r.violation(test, req, file, "broker.json %s: %s (platform/broker.schema.json)", where, f.Detail)
	}
	if len(fs) == 0 {
		r.checked(test, reqBrokerJSON, file, "broker.json is valid (platform/broker.schema.json)")
	}
	obj, _ := inst.(map[string]any)
	addr, isString := obj["Address"].(string)
	if !isString {
		r.violation(test, reqAddress, file, "broker.json has no Address (String): the address of the broker is not provided")
		return nil
	}
	probe := map[string]any{"Address": addr, "SecurePort": json.Number("8883"), "MaxPacketSize": json.Number("262144")}
	af, err := set.Validate(brokerSchema, probe)
	if err != nil {
		return err
	}
	if len(af) > 0 {
		r.violation(test, reqAddress, file, "broker.json Address %q is no IP address or host name (RIG 4.1.1)", addr)
		return nil
	}
	r.checked(test, reqAddress, file, "broker.json Address %s", addr)
	return nil
}

// brokerCertificate judges broker.pem and the CA that verifies it:
// broker_ca.pem if present, else ca.pem of the OI4 certificate storage
// (REQ-CNT-001). The validity period and the host name of the broker
// certificate are judged by T8-BUS-01 (REQ-MQTT-007, MG 6.2.2).
func (r *runner) brokerCertificate() {
	s := r.mqtt
	file := s.path("broker.pem")
	data, ok, err := s.read("broker.pem")
	switch {
	case err != nil:
		r.inconclusive(TMQTT, []string{reqMQTTStorage}, file, "%s", unreadable(file, err))
		return
	case !ok:
		r.violation(TMQTT, reqMQTTStorage, file, "no broker.pem (public certificate of the broker) in the Message Bus storage (RIG 4.1.1)")
		return
	}
	chain, err := pemCertificates(data)
	if err != nil {
		r.violation(TMQTT, reqMQTTStorage, file, "broker.pem: %v", err)
		return
	}
	caFile, caName := s.path("broker_ca.pem"), "broker_ca.pem"
	caData, ok, err := s.read("broker_ca.pem")
	if err == nil && !ok {
		caFile, caName = r.certs.path("ca.pem"), "ca.pem"
		caData, ok, err = r.certs.read("ca.pem")
	}
	switch {
	case err != nil:
		r.inconclusive(TMQTT, []string{reqMQTTStorage}, caFile, "%s", unreadable(caFile, err))
		return
	case !ok:
		r.violation(TMQTT, reqMQTTStorage, file, "neither broker_ca.pem nor ca.pem of the OI4 certificate storage: the broker certificate cannot be verified (RIG 4.1.1)")
		return
	}
	cas, err := pemCertificates(caData)
	if err != nil {
		r.violation(TMQTT, reqMQTTStorage, caFile, "%s: %v", caName, err)
		return
	}
	if caName == "broker_ca.pem" {
		if own, _, _ := r.certs.read("ca.pem"); own != nil {
			if same, err := pemCertificates(own); err == nil && sameCertificate(cas, same) {
				r.note(TMQTT, reqMQTTStorage, caFile, "broker_ca.pem holds the CA of ca.pem; RIG 4.1.1 provides it only if the broker certificate was signed by a different CA")
			}
		}
	}
	valid, err := verify(chain, cas)
	if err != nil {
		if caName == "broker_ca.pem" {
			r.violation(TMQTT, reqMQTTStorage, file, "the broker certificate %q is not signed by the CA of broker_ca.pem: %v (RIG 4.1.1)", chain[0].Subject, err)
		} else {
			r.violation(TMQTT, reqMQTTStorage, file, "the broker certificate %q is not signed by the CA of ca.pem and there is no broker_ca.pem: %v (RIG 4.1.1)", chain[0].Subject, err)
		}
		return
	}
	if !valid {
		r.note(TMQTT, reqMQTTStorage, file, "the broker certificate is valid %s … %s, not now; T8-BUS-01 judges it (REQ-MQTT-007, MG 6.2.2)",
			chain[0].NotBefore.UTC().Format(time.DateOnly), chain[0].NotAfter.UTC().Format(time.DateOnly))
	}
	r.checked(TMQTT, reqMQTTStorage, file, "broker.pem %q is verified by %s", chain[0].Subject, caName)
}

// verify verifies the chain of broker.pem (leaf first) against the CAs. A
// chain that is valid only within the period of validity of its leaf is
// verified with valid = false.
func verify(chain, cas []*x509.Certificate) (valid bool, err error) {
	roots, inter := x509.NewCertPool(), x509.NewCertPool()
	for _, c := range cas {
		roots.AddCert(c)
	}
	for _, c := range chain[1:] {
		inter.AddCert(c)
	}
	opts := x509.VerifyOptions{Roots: roots, Intermediates: inter, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}}
	_, err = chain[0].Verify(opts)
	var inv x509.CertificateInvalidError
	if errors.As(err, &inv) && inv.Reason == x509.Expired {
		leaf := chain[0]
		opts.CurrentTime = leaf.NotBefore.Add(leaf.NotAfter.Sub(leaf.NotBefore) / 2)
		if _, err2 := leaf.Verify(opts); err2 == nil {
			return false, nil
		}
	}
	return err == nil, err
}

// pemCertificates parses every CERTIFICATE block of a PEM file.
func pemCertificates(data []byte) ([]*x509.Certificate, error) {
	var out []*x509.Certificate
	for {
		var b *pem.Block
		b, data = pem.Decode(data)
		if b == nil {
			break
		}
		if b.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(b.Bytes)
		if err != nil {
			return nil, fmt.Errorf("no valid certificate: %v", err)
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, errors.New("no PEM certificate")
	}
	return out, nil
}

func sameCertificate(a, b []*x509.Certificate) bool {
	for _, x := range a {
		for _, y := range b {
			if x.Equal(y) {
				return true
			}
		}
	}
	return false
}
