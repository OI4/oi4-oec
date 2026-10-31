// Package simapp is the simulated OEC application of the oec-infra run
// self-test (C-074): it reads the storages of RIG 4.1 like a containerized
// application, connects to the Message Bus with TLS and publishes its Birth
// message. A Fault makes it deviate in one point. It runs in process behind
// the fake Engine API (enginetest) and, built as the static binary
// oec-run-sim, in an image for the opt-in run with Docker.
package simapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/OI4/oec/conformance/internal/mqttwire"
)

// Fault is one deviation of the simulated application.
type Fault string

// The faults. Good and URIIL conform.
const (
	Good           Fault = "good"             // conforms
	URIIL          Fault = "uri-il"           // conforms, its Identification Link is a URI (no Oi4Identifier)
	StrictJSON     Fault = "strict-json"      // refuses a broker.json with an unknown element and exits
	Port8883       Fault = "port-8883"        // ignores SecurePort and connects to 8883
	CAOnly         Fault = "ca-only"          // verifies the broker with ca.pem even if broker_ca.pem exists
	NoVerify       Fault = "no-verify"        // does not verify the broker certificate
	Oversized      Fault = "oversized"        // publishes a payload larger than MaxPacketSize
	LastColon      Fault = "last-colon"       // splits the credentials at the last ":"
	PassphraseRaw  Fault = "passphrase-raw"   // uses the passphrase file without base64 decoding
	NoCert         Fault = "no-cert"          // ignores the client certificate
	NoCredentials  Fault = "no-credentials"   // ignores the credentials
	NoAuth         Fault = "no-auth"          // ignores both the client certificate and the credentials
	ClientIDRandom Fault = "client-id-random" // connects with a random ClientId
	SerialOther    Fault = "serial-other"     // uses another SerialNumber than its container name
	Exit           Fault = "exit"             // exits at once with 1
)

// Faults are all faults.
var Faults = []Fault{Good, URIIL, StrictJSON, Port8883, CAOnly, NoVerify, Oversized, LastColon, PassphraseRaw, NoCert, NoCredentials, NoAuth, ClientIDRandom, SerialOther, Exit}

// Known reports whether f is a fault of the package.
func Known(f Fault) bool {
	for _, k := range Faults {
		if k == f {
			return true
		}
	}
	return false
}

// The storage paths in the container (RIG 4.1.1 … 4.1.3).
const (
	BrokerJSON  = "/etc/oi4/mqtt/broker.json"
	BrokerCA    = "/etc/oi4/mqtt/broker_ca.pem"
	CA          = "/etc/oi4/certs/ca.pem"
	CertDir     = "/etc/oi4/certs"
	PrivateKey  = "/run/secrets/oi4/private_key.pem"
	Passphrase  = "/run/secrets/oi4/passphrase"
	Credentials = "/run/secrets/oi4/credentials"
)

// Identity of the simulated application: its Identification Link is
// ManufacturerUri/Model/ProductCode/<SerialNumber>.
const (
	ServiceType     = "Utility"
	ManufacturerURI = "oi4.org"
	Model           = "oec-d8-sim"
	ProductCode     = "SimApp"
	OtherSerial     = "SN-0001"
)

// Options configure one run.
type Options struct {
	Fault    Fault
	Hostname string // the container name; "": os.Hostname
	// Path maps a path of the container to the file system the application
	// reads; nil: the path itself.
	Path func(p string) string
	// Dialer opens the TCP connection (the fake engine maps the broker host
	// to loopback); nil: net.Dialer.
	Dialer func(ctx context.Context, network, addr string) (net.Conn, error)
	Retry  time.Duration // pause between connection attempts; default 1 s
	Log    func(format string, args ...any)
}

// Run runs the application until ctx ends (exit code 0) or it gives up
// (exit code 1).
func Run(ctx context.Context, o Options) int {
	if o.Path == nil {
		o.Path = func(p string) string { return p }
	}
	if o.Retry == 0 {
		o.Retry = time.Second
	}
	if o.Log == nil {
		o.Log = func(string, ...any) {}
	}
	if o.Hostname == "" {
		h, err := os.Hostname()
		if err != nil {
			o.Log("hostname: %v", err)
			return 1
		}
		o.Hostname = h
	}
	if o.Fault == Exit {
		o.Log("exit (fault)")
		return 1
	}
	for {
		cfg, maxPacket, err := read(o)
		if err != nil {
			o.Log("%v", err)
			return 1
		}
		cl, err := mqttwire.Dial(ctx, cfg)
		if err == nil {
			err = serve(ctx, o, cl, maxPacket)
			cl.Kill()
		}
		if ctx.Err() != nil {
			return 0
		}
		o.Log("%v", err)
		select {
		case <-ctx.Done():
			return 0
		case <-time.After(o.Retry):
		}
	}
}

type brokerJSON struct {
	Address       string `json:"Address"`
	SecurePort    int    `json:"SecurePort"`
	MaxPacketSize int    `json:"MaxPacketSize"`
}

// read takes the connection from the storages.
func read(o Options) (mqttwire.Config, int, error) {
	f := o.Fault
	file := func(p string) ([]byte, error) { return os.ReadFile(o.Path(p)) }
	exists := func(p string) bool {
		st, err := os.Stat(o.Path(p))
		return err == nil && !st.IsDir()
	}
	var c mqttwire.Config
	raw, err := file(BrokerJSON)
	if err != nil {
		return c, 0, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if f == StrictJSON {
		dec.DisallowUnknownFields()
	}
	var b brokerJSON
	if err := dec.Decode(&b); err != nil {
		return c, 0, fmt.Errorf("broker.json: %w", err)
	}
	if b.Address == "" || b.SecurePort < 1 || b.SecurePort > 65535 {
		return c, 0, errors.New("broker.json needs Address and SecurePort")
	}
	port := b.SecurePort
	if f == Port8883 {
		port = mqttwire.DefaultPort
	}
	c.Broker = "mqtts://" + net.JoinHostPort(b.Address, strconv.Itoa(port))
	c.CAFile = o.Path(CA)
	if f != CAOnly && exists(BrokerCA) {
		c.CAFile = o.Path(BrokerCA)
	}
	c.Insecure = f == NoVerify
	cert := path.Join(CertDir, o.Hostname+".pem")
	if f != NoCert && f != NoAuth && exists(cert) && exists(PrivateKey) {
		c.CertFile, c.KeyFile = o.Path(cert), o.Path(PrivateKey)
		if pp, err := file(Passphrase); err == nil {
			if f == PassphraseRaw {
				c.KeyPassphrase = bytes.TrimSpace(pp)
			} else if c.KeyPassphrase, err = base64.StdEncoding.DecodeString(strings.TrimSpace(string(pp))); err != nil {
				return c, 0, fmt.Errorf("passphrase: %w", err)
			}
		}
		if _, err := mqttwire.TLSConfig(c, b.Address); err != nil {
			// A key the application cannot decrypt: it goes on without it.
			o.Log("client certificate: %v", err)
			c.CertFile, c.KeyFile, c.KeyPassphrase = "", "", nil
		}
	}
	if cr, err := file(Credentials); err == nil && f != NoCredentials && f != NoAuth {
		dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(cr)))
		if err != nil {
			return c, 0, fmt.Errorf("credentials: %w", err)
		}
		s := string(dec)
		i := strings.Index(s, ":")
		if f == LastColon {
			i = strings.LastIndex(s, ":")
		}
		if i < 1 {
			return c, 0, errors.New("credentials are not <username>:<password>")
		}
		c.Username, c.Password = s[:i], []byte(s[i+1:])
	}
	c.ClientID = o.Hostname
	if f == ClientIDRandom {
		c.ClientID = mqttwire.ClientID("sim-")
	}
	c.Dialer = o.Dialer
	c.ConnectTimeout = 5 * time.Second
	c.KeepAlive = 30 * time.Second
	il := IdentificationLink(f, o.Hostname)
	c.Will = &mqttwire.Message{Topic: Topic("Health", il), Payload: health(il, "FAILURE_1", 0)}
	limit := b.MaxPacketSize
	if limit == 0 {
		limit = 262144
	}
	return c, limit, nil
}

// IdentificationLink returns the Identification Link of the application.
func IdentificationLink(f Fault, hostname string) string {
	switch f {
	case URIIL:
		return "https://" + ManufacturerURI + "/" + Model + "/" + ProductCode + "/" + hostname
	case SerialOther:
		return ManufacturerURI + "/" + Model + "/" + ProductCode + "/" + OtherSerial
	}
	return ManufacturerURI + "/" + Model + "/" + ProductCode + "/" + hostname
}

// Topic is the Pub topic of a resource of the application about itself.
func Topic(resource, il string) string {
	e := base64.RawURLEncoding.EncodeToString([]byte(il))
	return "Oi4v2/" + ServiceType + "/" + e + "/Pub/" + resource + "/" + e
}

func message(il, class string, writer int, payload map[string]any) []byte {
	doc := map[string]any{
		"MessageId":      strconv.FormatInt(time.Now().UnixMilli(), 10) + "-" + ServiceType + "/" + il,
		"MessageType":    "ua-data",
		"PublisherId":    ServiceType + "/" + il,
		"DataSetClassId": class,
		"ReplyTo":        "",
		"Messages": []any{map[string]any{
			"DataSetWriterId": writer, "WriterGroupName": "", "DataSetWriterName": il, "Payload": payload,
		}},
	}
	b, _ := json.Marshal(doc)
	return b
}

func health(il, state string, score int) []byte {
	return message(il, "d8e7b6df-42ba-448a-975a-199f59e8ffeb", 2, map[string]any{"Health": state, "HealthScore": score})
}

// MAM is the Birth message of the application (exported for the tests of
// the endpoint).
func MAM(f Fault, hostname string) []byte {
	il := IdentificationLink(f, hostname)
	serial := hostname
	if f == SerialOther {
		serial = OtherSerial
	}
	text := func(s string) map[string]any { return map[string]any{"Locale": "en-US", "Text": s} }
	return message(il, "360ca8f3-5e66-42a2-8f10-9cdf45f4bf58", 1, map[string]any{
		"Manufacturer": text("OI4"), "ManufacturerUri": ManufacturerURI, "Model": text(Model), "ProductCode": ProductCode,
		"HardwareRevision": "", "SoftwareRevision": "2.0.0", "DeviceRevision": "", "DeviceManual": "",
		"DeviceClass": "Oi4." + ServiceType, "SerialNumber": serial, "ProductInstanceUri": il,
		"RevisionCounter": 0, "Description": text("Simulated application of oec-infra run"),
	})
}

// serve publishes the Birth message and the Health and stays connected.
func serve(ctx context.Context, o Options, cl *mqttwire.Client, maxPacket int) error {
	il := IdentificationLink(o.Fault, o.Hostname)
	if err := cl.Publish(ctx, mqttwire.Message{Topic: Topic("MAM", il), Payload: MAM(o.Fault, o.Hostname), QoS: 1}); err != nil {
		return err
	}
	if err := cl.Publish(ctx, mqttwire.Message{Topic: Topic("Health", il), Payload: health(il, "NORMAL_0", 100), QoS: 1}); err != nil {
		return err
	}
	if o.Fault == Oversized {
		big := bytes.Repeat([]byte{' '}, maxPacket+1)
		copy(big, health(il, "NORMAL_0", 100))
		if err := cl.Publish(ctx, mqttwire.Message{Topic: Topic("Health", il), Payload: big}); err != nil {
			return err
		}
	}
	select {
	case <-ctx.Done():
		cl.Close()
		return nil
	case err := <-cl.Lost():
		return fmt.Errorf("connection lost: %w", err)
	}
}
