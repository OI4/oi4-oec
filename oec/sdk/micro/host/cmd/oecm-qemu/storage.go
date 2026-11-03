package main

import (
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

// provisioning is the content of the NVS partition "oecprov" of the
// example firmware (examples/esp-idf/main/main.c), derived from the layout
// of the container storage below OEC_ROOT as the host runner reads it
// (REQ-CNT-004 … 009). A device has no container; the layout is the
// runner's convention.
type provisioning struct {
	serial    string // hostname of the storage
	uri       string // mqtts://host:port, loopback mapped to the slirp host address
	maxPacket int
	ca        []byte
	user      string
	password  string
	cert, key []byte // a client certificate wins (REQ-MQTT-010)
	level     int
}

func (p *provisioning) auth() string {
	if p.cert != nil {
		return "client certificate"
	}
	return "credentials"
}

type brokerJSON struct {
	Address       string
	SecurePort    *int
	MaxPacketSize *int
}

// loadProvisioning reads the storage below root; a loopback Address of
// broker.json becomes hostAddr, the address of the host in the QEMU user
// network (slirp: 10.0.2.2).
func loadProvisioning(root, hostAddr string) (*provisioning, error) {
	p := &provisioning{}
	name, err := hostname(root)
	if err != nil {
		return nil, err
	}
	p.serial = name

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
	addr := b.Address
	if ip := net.ParseIP(addr); addr == "localhost" || (ip != nil && ip.IsLoopback()) {
		addr = hostAddr
	}
	p.uri = "mqtts://" + net.JoinHostPort(addr, strconv.Itoa(*b.SecurePort))
	p.maxPacket = *b.MaxPacketSize

	for _, f := range []string{"etc/oi4/mqtt/broker_ca.pem", "etc/oi4/certs/ca.pem"} {
		if p.ca, err = os.ReadFile(filepath.Join(root, f)); err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("no CA certificate (REQ-CNT-004, REQ-CNT-005): %w", err)
	}
	if !strings.Contains(string(p.ca), "-----BEGIN CERTIFICATE-----") {
		return nil, errors.New("CA certificate: no PEM certificate")
	}

	secrets := filepath.Join(root, "run/secrets/oi4")
	key := filepath.Join(secrets, "private_key.pem")
	cert := filepath.Join(root, "etc/oi4/certs", name+".pem")
	if exists(key) && exists(cert) {
		if exists(filepath.Join(secrets, "passphrase")) {
			return nil, errors.New("an encrypted private_key.pem is not supported by the QEMU runner")
		}
		if p.cert, err = os.ReadFile(cert); err != nil {
			return nil, err
		}
		if p.key, err = os.ReadFile(key); err != nil {
			return nil, err
		}
		return p, nil
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
	p.user, p.password = user, pass
	return p, nil
}

type nvsString struct {
	key   string
	value []byte
}

// csv writes the input of the NVS partition generator of ESP-IDF
// (esp_idf_nvs_partition_gen) into dir: every string goes through a file,
// so that no value needs CSV quoting.
func (p *provisioning) csv(dir string) (string, error) {
	strs := []nvsString{{"uri", []byte(p.uri)}, {"ca", p.ca}, {"serial", []byte(p.serial)}}
	if p.cert != nil {
		strs = append(strs, nvsString{"cert", p.cert}, nvsString{"key", p.key})
	} else {
		strs = append(strs, nvsString{"user", []byte(p.user)}, nvsString{"password", []byte(p.password)})
	}
	var b strings.Builder
	b.WriteString("key,type,encoding,value\noec,namespace,,\n")
	for _, s := range strs {
		f := filepath.Join(dir, s.key+".txt")
		if err := os.WriteFile(f, s.value, 0o600); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "%s,file,string,%s\n", s.key, f)
	}
	fmt.Fprintf(&b, "level,data,u8,%d\nmax_packet,data,u32,%d\n", p.level, p.maxPacket)
	path := filepath.Join(dir, "oecprov.csv")
	return path, os.WriteFile(path, []byte(b.String()), 0o600)
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
