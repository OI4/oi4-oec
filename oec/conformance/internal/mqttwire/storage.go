package mqttwire

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Storage is what D7 takes from a container storage (RIG 4.1): the broker of
// the Message Bus and the login of the container.
type Storage struct {
	Config        Config
	MaxPacketSize int // broker.json, 0 if absent
}

type brokerJSON struct {
	Address       string `json:"Address"`
	SecurePort    int    `json:"SecurePort"`
	MaxPacketSize int    `json:"MaxPacketSize"`
}

// ReadStorage reads the container storage below root – the file system of
// an application container, e.g. a sample root of oec/dev (RIG 4.1.1–4.1.3):
//
//	etc/oi4/mqtt/broker.json            Address, SecurePort, MaxPacketSize
//	etc/oi4/mqtt/broker_ca.pem          CA of the broker, else etc/oi4/certs/ca.pem
//	etc/hostname                        container name
//	etc/oi4/certs/<container>.pem       client certificate
//	run/secrets/oi4/private_key.pem     its key (PKCS#8, encrypted)
//	run/secrets/oi4/passphrase          base64 passphrase of the key
//	run/secrets/oi4/credentials         base64 "<username>:<password>"
//
// The client certificate is used if it and its key exist, the credentials if
// they exist; both together are allowed.
func ReadStorage(root string) (*Storage, error) {
	p := func(rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }
	raw, err := os.ReadFile(p("etc/oi4/mqtt/broker.json"))
	if err != nil {
		return nil, fmt.Errorf("container storage: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	var b brokerJSON
	if err := dec.Decode(&b); err != nil {
		return nil, fmt.Errorf("container storage: broker.json: %w", err)
	}
	if b.Address == "" || b.SecurePort < 1 || b.SecurePort > 65535 {
		return nil, errors.New("container storage: broker.json needs Address and SecurePort")
	}
	s := &Storage{MaxPacketSize: b.MaxPacketSize}
	s.Config.Broker = "mqtts://" + net.JoinHostPort(b.Address, strconv.Itoa(b.SecurePort))
	for _, ca := range []string{"etc/oi4/mqtt/broker_ca.pem", "etc/oi4/certs/ca.pem"} {
		if exists(p(ca)) {
			s.Config.CAFile = p(ca)
			break
		}
	}
	if s.Config.CAFile == "" {
		return nil, errors.New("container storage: neither etc/oi4/mqtt/broker_ca.pem nor etc/oi4/certs/ca.pem")
	}
	if host, err := os.ReadFile(p("etc/hostname")); err == nil {
		name := strings.TrimSpace(string(host))
		cert, key := p("etc/oi4/certs/"+name+".pem"), p("run/secrets/oi4/private_key.pem")
		if name != "" && exists(cert) && exists(key) {
			s.Config.CertFile, s.Config.KeyFile = cert, key
			if pp, err := os.ReadFile(p("run/secrets/oi4/passphrase")); err == nil {
				dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(pp)))
				if err != nil {
					return nil, fmt.Errorf("container storage: passphrase: %w", err)
				}
				s.Config.KeyPassphrase = dec
			}
		}
	}
	if cr, err := os.ReadFile(p("run/secrets/oi4/credentials")); err == nil {
		dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(cr)))
		if err != nil {
			return nil, fmt.Errorf("container storage: credentials: %w", err)
		}
		user, pass, ok := strings.Cut(string(dec), ":")
		if !ok || user == "" {
			return nil, errors.New("container storage: credentials are not <username>:<password>")
		}
		s.Config.Username, s.Config.Password = user, []byte(pass)
	}
	if s.Config.CertFile == "" && s.Config.Username == "" {
		return nil, errors.New("container storage: neither a client certificate with key nor credentials")
	}
	return s, nil
}

func exists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
