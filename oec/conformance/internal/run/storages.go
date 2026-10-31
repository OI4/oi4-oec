package run

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
)

// The storages of RIG 4.1 a phase mounts into the container: the Message
// Bus storage, the OI4 certificate storage and the secret storage read-only,
// the application storages empty and writable.
const (
	DirMQTT    = "/etc/oi4/mqtt"
	DirCerts   = "/etc/oi4/certs"
	DirSecrets = "/run/secrets/oi4"
	DirAppEtc  = "/etc/oi4/app"
	DirAppOpt  = "/opt/oi4/app"
)

// UnknownElement is the element of broker.json that no client knows; a
// client ignores it (RIG 4.1.1, REQ-CNT-004).
const UnknownElement = "OecD8Unknown"

// storages describe the content of one phase.
type storages struct {
	address       string
	port          int
	maxPacketSize int
	unknown       bool   // broker.json with UnknownElement
	brokerPEM     []byte // broker.pem: the public certificate of the broker
	caPEM         []byte // certs/ca.pem
	brokerCAPEM   []byte // mqtt/broker_ca.pem; nil: absent
	name          string // the container name
	certPEM       []byte // certs/<name>.pem; nil: no client certificate
	keyPEM        []byte // secrets/private_key.pem
	passphrase    []byte // secrets/passphrase (written base64)
	username      string // secrets/credentials (written base64 "<username>:<password>"); "": none
	password      []byte
}

// write creates the storages below dir (a fresh temporary directory) and
// returns the binds host:container[:ro].
func (s storages) write(dir string) ([]string, error) {
	bj := map[string]any{"Address": s.address, "SecurePort": s.port, "MaxPacketSize": s.maxPacketSize}
	if s.unknown {
		bj[UnknownElement] = map[string]any{"Note": "an element of a later version; clients ignore it"}
	}
	raw, err := json.MarshalIndent(bj, "", "  ")
	if err != nil {
		return nil, err
	}
	b64 := func(b []byte) []byte { return []byte(base64.StdEncoding.EncodeToString(b) + "\n") }
	files := map[string][]byte{
		"mqtt/broker.json": append(raw, '\n'),
		"mqtt/broker.pem":  s.brokerPEM,
		"certs/ca.pem":     s.caPEM,
	}
	if s.brokerCAPEM != nil {
		files["mqtt/broker_ca.pem"] = s.brokerCAPEM
	}
	if s.certPEM != nil {
		files["certs/"+s.name+".pem"] = s.certPEM
		files["secrets/private_key.pem"] = s.keyPEM
		files["secrets/passphrase"] = b64(s.passphrase)
	}
	if s.username != "" {
		files["secrets/credentials"] = b64(append([]byte(s.username+":"), s.password...))
	}
	dirs := []struct {
		rel, dst string
		mode     os.FileMode
		ro       bool
	}{
		{"mqtt", DirMQTT, 0o755, true},
		{"certs", DirCerts, 0o755, true},
		{"secrets", DirSecrets, 0o755, true},
		{"app-etc", DirAppEtc, 0o777, false},
		{"app-opt", DirAppOpt, 0o777, false},
	}
	var binds []string
	for _, d := range dirs {
		p := filepath.Join(dir, d.rel)
		if err := os.Mkdir(p, d.mode); err != nil {
			return nil, err
		}
		// The umask must not take the write permission of the application
		// storages: the container may run as another user.
		if err := os.Chmod(p, d.mode); err != nil {
			return nil, err
		}
		b := p + ":" + d.dst
		if d.ro {
			b += ":ro"
		}
		binds = append(binds, b)
	}
	for rel, data := range files {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), data, 0o644); err != nil {
			return nil, err
		}
	}
	return binds, nil
}
