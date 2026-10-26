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
		"./generated/broker:/oec/broker:ro",
	} {
		if !strings.Contains(c, want) {
			t.Errorf("compose.yaml lacks %s", want)
		}
	}
}
