package run

import (
	"fmt"
	"strings"
	"time"

	"github.com/OI4/oec/conformance/internal/ilink"
	"github.com/OI4/oec/conformance/internal/verdict"
)

// judge turns the phases into observations.
type judge struct {
	tests []string
	obs   []verdict.Observation
	items []item
}

// item is one observation of a requirement before it is assigned to the
// selected tests.
type item struct {
	req, ctr, path, detail string
	kind                   verdict.Kind
	inconclusive           bool
}

func (j *judge) add(req string, k verdict.Kind, ctr, p, format string, a ...any) {
	j.items = append(j.items, item{req: req, kind: k, ctr: ctr, path: p, detail: fmt.Sprintf(format, a...)})
}

func (j *judge) unknown(req, ctr, format string, a ...any) {
	j.items = append(j.items, item{req: req, kind: verdict.Skipped, inconclusive: true, ctr: ctr, detail: fmt.Sprintf(format, a...)})
}

// inconclusive marks every requirement of the selected tests.
func (j *judge) inconclusive(format string, a ...any) {
	for _, t := range j.tests {
		for _, req := range reqsOf[t] {
			j.obs = append(j.obs, verdict.Observation{Test: t, Req: req, Kind: verdict.Skipped, Inconclusive: true, Evidence: verdict.Evidence{Detail: fmt.Sprintf(format, a...)}})
		}
	}
}

// emit assigns the items to the selected tests; REQ-CMP-029 (the container
// environment requirements of RIG 4.1 and 4.2) repeats the items of
// REQ-CNT-004, REQ-CNT-009 and REQ-CNT-021.
func (j *judge) emit() {
	for _, t := range j.tests {
		for _, req := range reqsOf[t] {
			n := 0
			for _, it := range j.items {
				detail := it.detail
				switch {
				case it.req == req:
				case req == reqEnv && (it.req == reqBus || it.req == reqSecrets || it.req == reqName):
					detail = it.req + ": " + detail
				default:
					continue
				}
				n++
				j.obs = append(j.obs, verdict.Observation{Test: t, Req: req, Kind: it.kind, Inconclusive: it.inconclusive,
					Evidence: verdict.Evidence{Container: it.ctr, Path: it.path, Detail: detail}})
			}
			if n == 0 {
				j.obs = append(j.obs, verdict.Observation{Test: t, Req: req, Kind: verdict.Skipped, Inconclusive: true,
					Evidence: verdict.Evidence{Detail: "no phase of the run could judge it"}})
			}
		}
	}
}

func (j *judge) judge(results []phaseResult, untrusted bool) {
	by := map[string]*phaseResult{}
	for i := range results {
		by[results[i].rep.Name] = &results[i]
	}
	j.bus(by, untrusted)
	j.secrets(by)
	j.name(by)
	j.clientID(results)
	j.emit()
}

// ended describes how a phase without a connection ended.
func ended(p *phaseResult) string {
	s := fmt.Sprintf("within %s", (time.Duration(p.rep.ObservedMS) * time.Millisecond).Round(100*time.Millisecond))
	if p.rep.ExitCode != nil {
		s += fmt.Sprintf("; the container exited with %d", *p.rep.ExitCode)
	}
	return s
}

// handshakes returns whether a connection completed or failed the TLS
// handshake, and the first error.
func handshakes(p *phaseResult) (ok, failed bool, msg string) {
	for _, c := range p.conns {
		switch c.Handshake {
		case "ok":
			ok = true
		case "failed":
			if !failed {
				msg = c.HandshakeError
			}
			failed = true
		}
	}
	return ok, failed, msg
}

const pathBrokerJSON = DirMQTT + "/broker.json"

// bus judges REQ-CNT-004: Address, SecurePort and MaxPacketSize of
// broker.json, the broker certificate verified with broker_ca.pem or
// ca.pem, unknown elements ignored.
func (j *judge) bus(by map[string]*phaseResult, untrusted bool) {
	if p := by[PhaseCredentials]; p != nil {
		ctr := p.rep.Container
		ok, failed, msg := handshakes(p)
		switch {
		case p.err:
			j.unknown(reqBus, ctr, "phase credentials: %s", p.rep.Error)
		case ok:
			j.add(reqBus, verdict.Checked, ctr, pathBrokerJSON, "connected to Address and SecurePort %d of broker.json (with an element the application does not know) and verified the broker certificate with ca.pem", p.rep.Port)
		case failed:
			j.add(reqBus, verdict.Violation, ctr, pathBrokerJSON, "the TLS handshake failed although the broker certificate is signed by the CA of ca.pem and broker_ca.pem is absent: %s", msg)
		default:
			j.add(reqBus, verdict.Violation, ctr, pathBrokerJSON, "no connection to Address and SecurePort %d of broker.json (with an element the application does not know) %s", p.rep.Port, ended(p))
		}
	}
	if p := by[PhaseCertificate]; p != nil {
		ctr := p.rep.Container
		ok, failed, msg := handshakes(p)
		switch {
		case p.err:
			j.unknown(reqBus, ctr, "phase certificate: %s", p.rep.Error)
		case ok:
			j.add(reqBus, verdict.Checked, ctr, DirMQTT+"/broker_ca.pem", "verified the broker certificate with broker_ca.pem")
		case failed:
			j.add(reqBus, verdict.Violation, ctr, DirMQTT+"/broker_ca.pem", "the TLS handshake failed although the broker certificate is signed by the CA of broker_ca.pem (ca.pem names another CA): %s", msg)
		default:
			j.unknown(reqBus, ctr, "phase certificate: no connection to SecurePort %d %s", p.rep.Port, ended(p))
		}
	}
	if p := by[PhaseUntrusted]; untrusted && p != nil {
		ctr := p.rep.Container
		ok, failed, _ := handshakes(p)
		switch {
		case p.err:
			j.unknown(reqBus, ctr, "phase untrusted: %s", p.rep.Error)
		case ok:
			j.add(reqBus, verdict.Violation, ctr, DirCerts+"/ca.pem", "accepted a broker certificate of a CA that neither ca.pem nor broker_ca.pem names")
		case failed:
			j.add(reqBus, verdict.Checked, ctr, DirCerts+"/ca.pem", "refused a broker certificate of a CA that neither ca.pem nor broker_ca.pem names")
		default:
			j.unknown(reqBus, ctr, "phase untrusted: no connection to SecurePort %d %s", p.rep.Port, ended(p))
		}
	}
	for _, name := range []string{PhaseCredentials, PhaseCertificate, PhaseUntrusted} {
		p := by[name]
		if p == nil {
			continue
		}
		for _, c := range p.conns {
			if c.Oversized > 0 {
				j.add(reqBus, verdict.Violation, p.rep.Container, pathBrokerJSON, "published a payload of %d octets, larger than MaxPacketSize of broker.json", c.LargestPayload)
				break
			}
		}
	}
}

// The state of one means of authentication.
const (
	authUnknown = iota // no CONNECT: it cannot be judged
	authOK
	authUnused
	authWrong
)

// connects returns the connections with a CONNECT.
func connects(p *phaseResult) []conn {
	var out []conn
	for _, c := range p.conns {
		if c.Auth != "" {
			out = append(out, c)
		}
	}
	return out
}

// secrets judges REQ-CNT-009: the credentials (phase credentials) and the
// client certificate with its encrypted key (phase certificate).
func (j *judge) secrets(by map[string]*phaseResult) {
	cred, cert := authUnknown, authUnknown
	var credCtr, certCtr string
	if p := by[PhaseCredentials]; p != nil && !p.err {
		credCtr = p.rep.Container
		for _, c := range connects(p) {
			switch {
			case c.Auth == "credentials-wrong":
				cred = authWrong
			case c.Auth == "credentials" && cred != authWrong:
				cred = authOK
			case cred == authUnknown:
				cred = authUnused
			}
		}
	}
	if p := by[PhaseCertificate]; p != nil && !p.err {
		certCtr = p.rep.Container
		for _, c := range connects(p) {
			switch {
			case c.ClientCertificate == "untrusted" || c.Auth == "credentials-wrong":
				cert = authWrong
			case c.Auth == "certificate" && cert != authWrong:
				cert = authOK
			case cert == authUnknown:
				cert = authUnused
			}
		}
	}
	const sf = " (SF-117: whether an application must support both means is open)"
	switch {
	case cred == authWrong || cert == authWrong:
		if cred == authWrong {
			j.add(reqSecrets, verdict.Violation, credCtr, DirSecrets+"/credentials", "the CONNECT carried other credentials than the base64-decoded credentials split at the first \":\"")
		}
		if cert == authWrong {
			j.add(reqSecrets, verdict.Violation, certCtr, DirCerts, "the CONNECT authenticated with something else than the certificate <containername>.pem and private_key.pem with the base64-decoded passphrase")
		}
	case cred == authOK && cert == authOK:
		j.add(reqSecrets, verdict.Checked, credCtr, DirSecrets+"/credentials", "authenticated with the credentials")
		j.add(reqSecrets, verdict.Checked, certCtr, DirCerts, "authenticated with the client certificate and the private key decrypted with the passphrase")
	case cred == authOK && cert == authUnused:
		j.add(reqSecrets, verdict.Warning, certCtr, DirCerts, "authenticated with the credentials, but not with the client certificate and the encrypted private key"+sf)
	case cred == authUnused && cert == authOK:
		j.add(reqSecrets, verdict.Warning, credCtr, DirSecrets+"/credentials", "authenticated with the client certificate, but not with the credentials"+sf)
	case cred == authUnused && cert == authUnused:
		j.add(reqSecrets, verdict.Violation, credCtr, DirSecrets, "authenticated neither with the credentials nor with the client certificate")
	default:
		ctr := credCtr
		if cred != authUnknown {
			ctr = certCtr
		}
		j.unknown(reqSecrets, ctr, "no CONNECT in the phase %s", map[bool]string{true: PhaseCredentials, false: PhaseCertificate}[cred == authUnknown])
	}
}

// name judges REQ-CNT-021: the container name from the hostname as name
// of the certificate and, with an Oi4Identifier, as SerialNumber.
func (j *judge) name(by map[string]*phaseResult) {
	n := len(j.items)
	for _, ph := range []string{PhaseCredentials, PhaseCertificate} {
		p := by[ph]
		if p == nil || p.rep.Birth == nil {
			continue
		}
		ctr, b := p.rep.Container, p.rep.Birth
		if ilink.Scheme(b.IdentificationLink) != "" {
			j.add(reqName, verdict.Note, ctr, "", "the Identification Link %s is a URI, not an Oi4Identifier: the SerialNumber rule does not apply", b.IdentificationLink)
			break
		}
		segs := strings.Split(b.IdentificationLink, "/")
		switch {
		case len(segs) != 4:
			j.add(reqName, verdict.Violation, ctr, "", "the Identification Link %s of the Birth message is no Oi4Identifier of four elements", b.IdentificationLink)
		case segs[3] != ilink.DnpEncode(ctr):
			j.add(reqName, verdict.Violation, ctr, "", "the SerialNumber %s of the Oi4Identifier is not the container name", segs[3])
		case b.SerialNumber != "" && b.SerialNumber != ctr:
			j.add(reqName, verdict.Violation, ctr, "", "the SerialNumber %s of the MAM is not the container name", b.SerialNumber)
		default:
			j.add(reqName, verdict.Checked, ctr, "", "the SerialNumber of the Oi4Identifier is the container name")
		}
		break
	}
	if p := by[PhaseCertificate]; p != nil {
		for _, c := range p.conns {
			if c.Auth == "certificate" {
				j.add(reqName, verdict.Checked, p.rep.Container, DirCerts+"/"+p.rep.Container+".pem", "used the certificate named after the container name")
				break
			}
		}
	}
	if len(j.items) == n {
		j.unknown(reqName, "", "no Birth message and no CONNECT with the client certificate")
	}
}

// clientID judges REQ-MQTT-013: the container name as MQTT ClientId.
func (j *judge) clientID(results []phaseResult) {
	n := len(j.items)
	for i := range results {
		p := &results[i]
		for _, c := range connects(p) {
			if c.ClientID == p.rep.Container {
				j.add(reqClientID, verdict.Checked, p.rep.Container, "", "the ClientId is the container name")
			} else {
				j.add(reqClientID, verdict.Violation, p.rep.Container, "", "the ClientId %q is not the container name", c.ClientID)
			}
			break
		}
	}
	if len(j.items) == n {
		j.unknown(reqClientID, "", "no CONNECT in any phase")
	}
}
