package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// storage is the broker configuration of the host runner, read from the
// layout of the container storage (REQ-CNT-004 … 009) below a root, so
// that the runner works with oec/dev and the D7 harness of the demos. A
// device has no container; the layout is the runner's convention.
type storage struct {
	name      string // hostname: default serial number
	addr      string // host:port of the broker
	maxPacket int
	tls       *tls.Config
	username  *string
	password  *string
	auth      string // "client certificate" or "credentials"
}

type brokerJSON struct {
	Address       string
	SecurePort    *int
	MaxPacketSize *int
}

func loadStorage(root string) (*storage, error) {
	st := &storage{}
	name, err := hostname(root)
	if err != nil {
		return nil, err
	}
	st.name = name

	path := filepath.Join(root, "etc/oi4/mqtt/broker.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("broker.json (REQ-CNT-004): %w", err)
	}
	var b brokerJSON
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if b.Address == "" || b.SecurePort == nil || *b.SecurePort < 1 || *b.SecurePort > 65535 ||
		b.MaxPacketSize == nil || *b.MaxPacketSize < 1 {
		return nil, fmt.Errorf("%s: Address, SecurePort 1 … 65535 and MaxPacketSize are required (REQ-CNT-003)", path)
	}
	st.addr = net.JoinHostPort(b.Address, strconv.Itoa(*b.SecurePort))
	st.maxPacket = *b.MaxPacketSize

	pool := x509.NewCertPool()
	var ca []byte
	for _, f := range []string{"etc/oi4/mqtt/broker_ca.pem", "etc/oi4/certs/ca.pem"} {
		if ca, err = os.ReadFile(filepath.Join(root, f)); err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("no CA certificate (REQ-CNT-004, REQ-CNT-005): %w", err)
	}
	if !pool.AppendCertsFromPEM(ca) {
		return nil, errors.New("CA certificate: no PEM certificate")
	}
	// Verification of the broker certificate (REQ-MQTT-008), TLS 1.2 or
	// higher (REQ-MQTT-005).
	st.tls = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}

	secrets := filepath.Join(root, "run/secrets/oi4")
	key := filepath.Join(secrets, "private_key.pem")
	cert := filepath.Join(root, "etc/oi4/certs", name+".pem")
	if exists(key) && exists(cert) {
		// A client certificate wins over credentials (REQ-MQTT-010).
		if exists(filepath.Join(secrets, "passphrase")) {
			return nil, errors.New("an encrypted private_key.pem is not supported by the host runner")
		}
		pair, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			return nil, fmt.Errorf("client certificate: %w", err)
		}
		st.tls.Certificates = []tls.Certificate{pair}
		st.auth = "client certificate"
		return st, nil
	}
	raw, err = os.ReadFile(filepath.Join(secrets, "credentials"))
	if err != nil {
		return nil, fmt.Errorf("no means of authentication (REQ-MQTT-009, REQ-CNT-008): %w", err)
	}
	dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("credentials are not base64 (REQ-CNT-009): %w", err)
	}
	user, pass, ok := strings.Cut(string(dec), ":")
	if !ok || user == "" {
		return nil, errors.New("credentials: expected base64(<username>:<password>) (REQ-CNT-009)")
	}
	st.username, st.password = &user, &pass
	st.auth = "credentials"
	return st, nil
}

func hostname(root string) (string, error) {
	for _, f := range []string{"proc/sys/kernel/hostname", "etc/hostname"} {
		if raw, err := os.ReadFile(filepath.Join(root, f)); err == nil {
			if name := strings.TrimSpace(string(raw)); name != "" {
				return name, nil
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}
	return os.Hostname()
}

func exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
