package main

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// listener is the settings of one listener of mosquitto.conf; global holds
// the settings before the first listener.
type listener map[string][]string

func parseConf(t *testing.T) (listener, map[string]listener) {
	t.Helper()
	b, err := os.ReadFile("../mosquitto/mosquitto.conf")
	if err != nil {
		t.Fatal(err)
	}
	global, listeners := listener{}, map[string]listener{}
	current := global
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		if key == "listener" {
			current = listener{}
			listeners[value] = current
			continue
		}
		current[key] = append(current[key], value)
	}
	return global, listeners
}

func one(t *testing.T, l listener, key string) string {
	t.Helper()
	if len(l[key]) != 1 {
		t.Fatalf("%s: %v", key, l[key])
	}
	return l[key][0]
}

// TLS 1.2/1.3 on every listener, no plain MQTT (REQ-MQTT-005, REQ-MQTT-006),
// no anonymous clients, credentials on 8883 and client certificates on
// 8884 (REQ-MQTT-009, REQ-MQTT-010), the packet size limits of SF-105.
func TestMosquittoConf(t *testing.T) {
	global, listeners := parseConf(t)
	if len(listeners) != 2 || listeners["8883"] == nil || listeners["8884"] == nil {
		t.Fatalf("listeners %v, want 8883 and 8884", listeners)
	}
	for port, l := range listeners {
		if one(t, l, "tls_version") != "tlsv1.2" || one(t, l, "protocol") != "mqtt" {
			t.Errorf("%s: tls_version/protocol", port)
		}
		for key, want := range map[string]string{"cafile": "/mosquitto/tls/ca.pem", "certfile": "/mosquitto/tls/broker.pem", "keyfile": "/mosquitto/tls/broker.key"} {
			if one(t, l, key) != want {
				t.Errorf("%s: %s", port, key)
			}
		}
		if _, ok := l["allow_anonymous"]; ok {
			t.Errorf("%s: per-listener allow_anonymous", port)
		}
	}
	if one(t, global, "allow_anonymous") != "false" || one(t, global, "password_file") != "/mosquitto/tls/passwd" {
		t.Error("global authentication")
	}
	if _, ok := global["per_listener_settings"]; ok {
		t.Error("per_listener_settings is deprecated in Mosquitto 2.1")
	}
	if _, ok := listeners["8883"]["require_certificate"]; ok {
		t.Error("8883 is the credentials listener")
	}
	if one(t, listeners["8884"], "require_certificate") != "true" || one(t, listeners["8884"], "use_identity_as_username") != "true" {
		t.Error("8884 needs a client certificate and uses its CN as user name")
	}
	if one(t, global, "message_size_limit") != strconv.Itoa(maxPacketSize) {
		t.Error("message_size_limit must be the MaxPacketSize of broker.json")
	}
	if one(t, global, "max_packet_size") != strconv.Itoa(maxPacketSize+5+2+65535+2) {
		t.Error("max_packet_size must be MaxPacketSize plus the largest PUBLISH header (SF-105)")
	}
	if one(t, global, "persistence") != "true" || one(t, global, "queue_qos0_messages") != "true" {
		t.Error("the restart test needs persistence and queued QoS 0 messages")
	}
	if one(t, global, "acl_file") != "/mosquitto/tls/acl" {
		t.Error("acl_file must be the ACL copied by the entrypoint")
	}
}

// The ACL lets only the probe user publish Set, Del and Call (REQ-GEN-001,
// oec-infra bus T8-BUS-02) and the user of the config service only
// Set/Config (ADR-0028 §5); every client reads the Message Bus and
// publishes Pub, Get and Reply.
func TestACL(t *testing.T) {
	b, err := os.ReadFile("../mosquitto/acl")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(string(b), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	want := []string{
		"user " + probeUser, "topic readwrite #",
		"user " + configUser, "topic write Oi4v2/+/+/Set/Config/#",
		"pattern read Oi4v2/#", "pattern write Oi4v2/+/+/Pub/#", "pattern write Oi4v2/+/+/Get/#",
		"pattern write Oi4v2/+/+/Reply/#",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("acl:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}

// The image is pinned by tag and digest; the ports default to loopback and
// to the ports of the sample broker.json.
func TestCompose(t *testing.T) {
	b, err := os.ReadFile("../compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c := string(b)
	if !regexp.MustCompile(`(?m)^\s+image: \S+:\d+\.\d+\.\d+\S*@sha256:[0-9a-f]{64}$`).MatchString(c) {
		t.Error("image not pinned by version tag and digest")
	}
	d := defaults()
	for _, want := range []string{
		`"${OEC_DEV_BIND:-127.0.0.1}:${OEC_DEV_MQTT_PORT:-` + strconv.Itoa(d.port) + `}:8883"`,
		`"${OEC_DEV_BIND:-127.0.0.1}:${OEC_DEV_CERT_PORT:-` + strconv.Itoa(d.certPort) + `}:8884"`,
		"./mosquitto/mosquitto.conf:/mosquitto/config/mosquitto.conf:ro",
		"./mosquitto/acl:/oec/acl/acl:ro",
		"/oec/acl/acl /mosquitto/tls/",
		"./generated/broker:/oec/broker:ro",
	} {
		if !strings.Contains(c, want) {
			t.Errorf("compose.yaml lacks %s", want)
		}
	}
}

// The Podman override (C-107): Docker Compose refuses
// healthcheck.start_interval against the API of Podman 4.x ("requires Docker
// Engine v25 or later"), so compose.podman.yaml resets it for the broker and
// probes every 5 s instead; it changes nothing else.
func TestComposePodman(t *testing.T) {
	b, err := os.ReadFile("../compose.podman.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s := services(t, string(b))
	if len(s) != 1 || s["mosquitto"] == "" {
		t.Fatalf("services %v, want mosquitto only", s)
	}
	var keys []string
	for _, line := range strings.Split(s["mosquitto"], "\n") {
		if l := strings.TrimSpace(line); l != "" && !strings.HasPrefix(l, "#") {
			keys = append(keys, l)
		}
	}
	want := []string{"healthcheck:", "start_interval: !reset null", "interval: 5s"}
	if strings.Join(keys, "\n") != strings.Join(want, "\n") {
		t.Errorf("mosquitto:\n%s\nwant:\n%s", strings.Join(keys, "\n"), strings.Join(want, "\n"))
	}
	c, err := os.ReadFile("../compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(services(t, string(c))["mosquitto"], "      start_interval: 1s\n") {
		t.Error("compose.yaml: the broker has no start_interval to reset")
	}
}

// service is the text of one service of compose.yaml (the lines below
// `  <name>:` up to the next service or top-level key).
func services(t *testing.T, c string) map[string]string {
	t.Helper()
	out, name, in := map[string]string{}, "", false
	for _, line := range strings.Split(c, "\n") {
		switch {
		case line == "services:":
			in = true
		case in && line != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "#"):
			in, name = false, ""
		case in && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") && strings.HasSuffix(line, ":") && !strings.HasPrefix(strings.TrimSpace(line), "#"):
			name = strings.TrimSuffix(strings.TrimSpace(line), ":")
		case in && name != "":
			out[name] += line + "\n"
		}
	}
	return out
}

// The profile demos (ADR-0028 §5): opt-in, so the broker alone stays the
// default; the local images only (never pulled); the container name of
// generated/demos.env as name and hostname (REQ-CNT-019); the container
// storage of `go run ./pki` read-only, named volumes for the application
// storages and nothing over /opt/oi4/licenses (REQ-CNT-017); the Web UI on
// loopback only (A-305); a bridge network (REQ-CNT-023). With the profile
// registry, a demo starts after the registry (A-421): the config service of
// Level 1 answers no Get MAM (REQ-CMP-017 from Level 2), so the registry
// sees its MAM only in its Birth (MG A1); without the profile, the demos
// start alone (required: false).
func TestComposeDemos(t *testing.T) {
	b, err := os.ReadFile("../compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s := services(t, string(b))
	if len(s) != 2+len(demos) || s["mosquitto"] == "" || strings.Contains(s["mosquitto"], "profiles:") {
		t.Fatalf("services %v: mosquitto without profile plus the demos and the registry", len(s))
	}
	for _, d := range demos {
		c := s[d.user]
		if c == "" {
			t.Errorf("no service %s", d.user)
			continue
		}
		for _, want := range []string{
			"profiles: [demos]",
			"image: oi4/oec-" + strings.TrimPrefix(d.user, "oec-") + ":${OEC_DEMOS_VERSION:-0.1.0}",
			"pull_policy: never",
			"container_name: ${" + d.env + ":-" + d.user + "}",
			"hostname: ${" + d.env + ":-" + d.user + "}",
			"./generated/roots/" + d.user + "/etc/oi4/mqtt:/etc/oi4/mqtt:ro",
			"./generated/roots/" + d.user + "/etc/oi4/certs:/etc/oi4/certs:ro",
			"./generated/roots/" + d.user + "/run/secrets/oi4:/run/secrets/oi4:ro",
			d.user + "-etc:/etc/oi4/app",
			d.user + "-opt:/opt/oi4/app",
			`"127.0.0.1:` + strconv.Itoa(d.port) + ":" + strconv.Itoa(d.port) + `"`,
			"condition: service_healthy",
			"      " + registryUser + ":\n        condition: service_started\n        required: false\n",
			"no-new-privileges:true",
			"cap_drop: [ALL]",
			"read_only: true",
		} {
			if !strings.Contains(c, want) {
				t.Errorf("%s lacks %s", d.user, want)
			}
		}
		for _, bad := range []string{"/opt/oi4/licenses", "network_mode", "privileged", "OEC_DEV_BIND", "0.0.0.0"} {
			if strings.Contains(c, bad) {
				t.Errorf("%s: %s", d.user, bad)
			}
		}
		if n := strings.Count(c, "127.0.0.1:"); n != 1 {
			t.Errorf("%s: %d published ports, want the Web UI only", d.user, n)
		}
		if !regexp.MustCompile(`(?m)^volumes:\n(?:  .*\n)*  ` + d.user + `-etc:\n(?:  .*\n)*  ` + d.user + `-opt:$`).MatchString(string(b)) {
			t.Errorf("%s: named volumes not declared", d.user)
		}
	}
	c := s["oec-config"]
	for _, want := range []string{
		"./generated/roots/oec-config/" + configServiceFile + ":/" + configServiceFile + ":ro",
		"OEC_CONFIG_FILE: /" + configServiceFile,
	} {
		if !strings.Contains(c, want) {
			t.Errorf("oec-config lacks %s", want)
		}
	}
}

// The profile registry (ADR-0029 §12, C-090, A-416/A-417): opt-in, the
// local image only; the container name of generated/demos.env; the
// container storage of `go run ./pki` read-only, named volumes for the
// application storages; both HTTPS ports on loopback only, TLS on; the
// compose service name as a further Host name.
func TestComposeRegistry(t *testing.T) {
	b, err := os.ReadFile("../compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c := services(t, string(b))[registryUser]
	if c == "" {
		t.Fatalf("no service %s", registryUser)
	}
	for _, want := range []string{
		"profiles: [registry]",
		"image: oi4/oec-registry:${OEC_REGISTRY_VERSION:-0.1.0}",
		"pull_policy: never",
		"container_name: ${" + registryEnv + ":-" + registryUser + "}",
		"hostname: ${" + registryEnv + ":-" + registryUser + "}",
		"OEC_REGISTRY_HOSTS: " + registryUser,
		"./generated/roots/" + registryUser + "/etc/oi4/mqtt:/etc/oi4/mqtt:ro",
		"./generated/roots/" + registryUser + "/etc/oi4/certs:/etc/oi4/certs:ro",
		"./generated/roots/" + registryUser + "/run/secrets/oi4:/run/secrets/oi4:ro",
		registryUser + "-etc:/etc/oi4/app",
		registryUser + "-opt:/opt/oi4/app",
		`"127.0.0.1:5798:5798"`,
		`"127.0.0.1:5799:5799"`,
		"condition: service_healthy",
		"no-new-privileges:true",
		"cap_drop: [ALL]",
		"read_only: true",
	} {
		if !strings.Contains(c, want) {
			t.Errorf("%s lacks %s", registryUser, want)
		}
	}
	for _, bad := range []string{"/opt/oi4/licenses", "network_mode", "privileged", "OEC_DEV_BIND", "0.0.0.0", "OEC_REGISTRY_TLS", "OEC_REGISTRY_WEB", "OEC_REGISTRY_API"} {
		if strings.Contains(c, bad) {
			t.Errorf("%s: %s", registryUser, bad)
		}
	}
	if n := strings.Count(c, "127.0.0.1:"); n != 2 {
		t.Errorf("%s: %d published ports, want 5798 and 5799", registryUser, n)
	}
	if !regexp.MustCompile(`(?m)^volumes:\n(?:  .*\n)*  ` + registryUser + `-etc:\n(?:  .*\n)*  ` + registryUser + `-opt:$`).MatchString(string(b)) {
		t.Errorf("%s: named volumes not declared", registryUser)
	}
}
