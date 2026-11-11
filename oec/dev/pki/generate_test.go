package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func generated(t *testing.T) (string, options) {
	t.Helper()
	o := defaults()
	o.out = filepath.Join(t.TempDir(), "generated")
	o.sans = []string{"broker.example", "192.0.2.1"}
	if err := generate(o); err != nil {
		t.Fatal(err)
	}
	return o.out, o
}

func read(t *testing.T, root, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func cert(t *testing.T, root, rel string) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(read(t, root, rel))
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatalf("%s: no PEM certificate", rel)
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func decode64(t *testing.T, b []byte) string {
	t.Helper()
	s, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	return string(s)
}

// The broker certificate verifies against the CA for every name, the client
// certificate for client authentication with the container name as CN.
func TestChain(t *testing.T) {
	out, _ := generated(t)
	roots := x509.NewCertPool()
	roots.AddCert(cert(t, out, "broker/ca.pem"))
	broker := cert(t, out, "broker/broker.pem")
	for _, name := range []string{"localhost", "mosquitto", "127.0.0.1", "::1", "broker.example", "192.0.2.1"} {
		if _, err := broker.Verify(x509.VerifyOptions{DNSName: name, Roots: roots}); err != nil {
			t.Errorf("broker certificate for %s: %v", name, err)
		}
	}
	probe := cert(t, out, "probe/probe.pem")
	if _, err := probe.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil || probe.Subject.CommonName != probeUser {
		t.Errorf("probe certificate: %v", err)
	}
	client := cert(t, out, "roots/oec-dev-cert/etc/oi4/certs/oec-dev-cert.pem")
	if _, err := client.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		t.Errorf("client certificate: %v", err)
	}
	if client.Subject.CommonName != certName {
		t.Errorf("client CN %q", client.Subject.CommonName)
	}
	if _, err := client.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err == nil {
		t.Error("the client certificate must not be usable as server certificate")
	}
	for _, root := range []string{appName, certName} {
		for _, rel := range []string{"etc/oi4/certs/ca.pem", "etc/oi4/mqtt/broker.pem"} {
			if !bytes.Equal(read(t, out, "roots/"+root+"/"+rel), read(t, out, "broker/"+filepath.Base(rel))) {
				t.Errorf("%s/%s differs from the broker side", root, rel)
			}
		}
	}
}

// broker.json follows RIG 4.1.1 / V-BRK (integers, PascalCase); the
// container name is the hostname; no broker_ca.pem with a single CA.
func TestContainerStorages(t *testing.T) {
	out, o := generated(t)
	for root, port := range map[string]int{appName: o.port, certName: o.certPort} {
		var b map[string]any
		if err := json.Unmarshal(read(t, out, "roots/"+root+"/etc/oi4/mqtt/broker.json"), &b); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"Address": "localhost", "SecurePort": float64(port), "MaxPacketSize": float64(maxPacketSize)}
		if len(b) != len(want) {
			t.Errorf("%s broker.json: %v", root, b)
		}
		for k, v := range want {
			if b[k] != v {
				t.Errorf("%s broker.json %s = %v, want %v", root, k, b[k], v)
			}
		}
		if got := strings.TrimSpace(string(read(t, out, "roots/"+root+"/etc/hostname"))); got != root {
			t.Errorf("hostname %q", got)
		}
		if _, err := os.Stat(filepath.Join(out, "roots", root, "etc/oi4/mqtt/broker_ca.pem")); err == nil {
			t.Errorf("%s: unexpected broker_ca.pem", root)
		}
		if fi, err := os.Stat(filepath.Join(out, "roots", root, "etc/oi4/app")); err != nil || !fi.IsDir() {
			t.Errorf("%s: no application specific volume", root)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "roots", certName, "run/secrets/oi4/credentials")); err == nil {
		t.Error("oec-dev-cert must authenticate with its certificate only")
	}
	if runtime.GOOS != "windows" {
		for _, rel := range []string{"broker/broker.key", "broker/passwd", "probe/password", "probe/probe.key",
			"roots/oec-dev-app/run/secrets/oi4/credentials", "roots/oec-dev-cert/run/secrets/oi4/private_key.pem",
			"roots/oec-dev-cert/run/secrets/oi4/passphrase"} {
			fi, err := os.Stat(filepath.Join(out, rel))
			if err != nil {
				t.Fatal(err)
			}
			if fi.Mode().Perm() != 0o600 {
				t.Errorf("%s: mode %v", rel, fi.Mode().Perm())
			}
		}
	}
}

// verify checks a password file line of mosquitto_passwd (`$7$`).
func verify(t *testing.T, line, user, password string) bool {
	t.Helper()
	u, hash, ok := strings.Cut(line, ":")
	parts := strings.Split(hash, "$")
	if !ok || u != user || len(parts) != 5 || parts[1] != "7" {
		t.Fatalf("not a $7$ line: %q", line)
	}
	iterations, err := strconv.Atoi(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	salt, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil {
		t.Fatal(err)
	}
	want, err := base64.StdEncoding.DecodeString(parts[4])
	if err != nil {
		t.Fatal(err)
	}
	got, err := pbkdf2.Key(sha512.New, password, salt, iterations, len(want))
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Equal(got, want)
}

// A line written by `mosquitto_passwd -b p alice secret` of
// eclipse-mosquitto:2.1.2-alpine: the format the generator imitates.
func TestMosquittoPasswdVector(t *testing.T) {
	const line = "alice:$7$1000$/2AwS4emc12eL/URFa3e1JpwSY1wGF9X6bY8EdfH3dNnOUYaWieh2yyDHmfkUxsHkEaO7Wn8SjBxZYMUTPY1vw==$vyhtQgHArUZ4DuG7eR8gKU7GP1qVW+3d33oS2NAPK8qi7YvUOM8PMFQbdNqHN9YuXY/NsSAxHQQpBym1MgyVuA=="
	if !verify(t, line, "alice", "secret") || verify(t, line, "alice", "Secret") {
		t.Fatal("PBKDF2-SHA512 of mosquitto_passwd not reproduced")
	}
	mine, err := mosquittoHash(defaults().rand, "alice", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if len(mine) != len(line) || !verify(t, mine, "alice", "secret") {
		t.Fatalf("generated line differs in shape: %q", mine)
	}
}

// The password file holds the users of the credentials files: oec-dev-app,
// the probe, then the demos (TestDemoStorages).
func TestPasswordFile(t *testing.T) {
	out, _ := generated(t)
	lines := strings.Split(strings.TrimSpace(string(read(t, out, "broker/passwd"))), "\n")
	if len(lines) != 2+len(demos) {
		t.Fatalf("passwd: %d lines", len(lines))
	}
	user, password, _ := strings.Cut(decode64(t, read(t, out, "roots/oec-dev-app/run/secrets/oi4/credentials")), ":")
	if user != appName || len(password) < 32 || !verify(t, lines[0], user, password) {
		t.Error("the credentials of oec-dev-app do not match passwd line 1")
	}
	user = strings.TrimSpace(string(read(t, out, "probe/username")))
	password = strings.TrimSpace(string(read(t, out, "probe/password")))
	if user != probeUser || len(password) < 32 || !verify(t, lines[1], user, password) {
		t.Error("the probe credentials do not match passwd line 2")
	}
}

// The private key of oec-dev-cert is PKCS#8 PBES2 (PBKDF2-HMAC-SHA256,
// AES-256-CBC) with the base64 passphrase of the secret storage, and it
// belongs to the client certificate.
func TestEncryptedPrivateKey(t *testing.T) {
	out, _ := generated(t)
	checkEncryptedKey(t, out, "roots/oec-dev-cert/", "roots/oec-dev-cert/etc/oi4/certs/oec-dev-cert.pem")
}

// checkEncryptedKey decrypts run/secrets/oi4/private_key.pem of the storage
// root with its passphrase and compares it with the certificate certRel.
func checkEncryptedKey(t *testing.T, out, root, certRel string) {
	t.Helper()
	block, _ := pem.Decode(read(t, out, root+"run/secrets/oi4/private_key.pem"))
	if block == nil || block.Type != "ENCRYPTED PRIVATE KEY" {
		t.Fatal("no ENCRYPTED PRIVATE KEY")
	}
	var info encryptedPrivateKeyInfo
	if rest, err := asn1.Unmarshal(block.Bytes, &info); err != nil || len(rest) > 0 {
		t.Fatal(err)
	}
	var p pbes2Params
	if _, err := asn1.Unmarshal(info.EncryptionAlgorithm.Parameters.FullBytes, &p); err != nil {
		t.Fatal(err)
	}
	var kdf pbkdf2Params
	if _, err := asn1.Unmarshal(p.KeyDerivationFunc.Parameters.FullBytes, &kdf); err != nil {
		t.Fatal(err)
	}
	var iv []byte
	if _, err := asn1.Unmarshal(p.EncryptionScheme.Parameters.FullBytes, &iv); err != nil {
		t.Fatal(err)
	}
	if !info.EncryptionAlgorithm.Algorithm.Equal(oidPBES2) || !p.KeyDerivationFunc.Algorithm.Equal(oidPBKDF2) ||
		!kdf.PRF.Algorithm.Equal(oidHMACWithSHA256) || !p.EncryptionScheme.Algorithm.Equal(oidAES256CBC) ||
		kdf.IterationCount != pbes2Iterations || len(kdf.Salt) != 16 || len(iv) != aes.BlockSize {
		t.Fatalf("unexpected algorithms: %+v %+v", p, kdf)
	}
	passphrase := decode64(t, read(t, out, root+"run/secrets/oi4/passphrase"))
	dk, err := pbkdf2.Key(sha256.New, passphrase, kdf.Salt, kdf.IterationCount, 32)
	if err != nil {
		t.Fatal(err)
	}
	c, err := aes.NewCipher(dk)
	if err != nil {
		t.Fatal(err)
	}
	plain := make([]byte, len(info.EncryptedData))
	cipher.NewCBCDecrypter(c, iv).CryptBlocks(plain, info.EncryptedData)
	pad := int(plain[len(plain)-1])
	key, err := x509.ParsePKCS8PrivateKey(plain[:len(plain)-pad])
	if err != nil {
		t.Fatal(err)
	}
	pub := cert(t, out, certRel).PublicKey.(*ecdsa.PublicKey)
	if !key.(*ecdsa.PrivateKey).PublicKey.Equal(pub) {
		t.Fatal("the private key does not belong to the client certificate")
	}
}

// An existing output is replaced only with -force and only if this tool
// wrote it.
func TestPrepare(t *testing.T) {
	out, o := generated(t)
	if err := generate(o); err == nil || !strings.Contains(err.Error(), "-force") {
		t.Fatalf("existing output overwritten: %v", err)
	}
	before := read(t, out, "broker/ca.pem")
	o.force = true
	if err := generate(o); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, read(t, out, "broker/ca.pem")) {
		t.Fatal("-force kept the old CA")
	}
	foreign := filepath.Join(t.TempDir(), "foreign")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	o.out = foreign
	if err := generate(o); err == nil || !strings.Contains(err.Error(), "not written by this tool") {
		t.Fatalf("foreign directory replaced: %v", err)
	}
}

// demoEnv reads generated/demos.env, the container names of the compose
// profile demos.
func demoEnv(t *testing.T, out string) map[string]string {
	t.Helper()
	env := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(read(t, out, demosEnv))), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("%s: %q", demosEnv, line)
		}
		env[k] = v
	}
	return env
}

var containerName = regexp.MustCompile(`^([a-z0-9-]{1,26})-[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// The demo containers of the compose profile demos (ADR-0028 §5): a
// <DefaultContainerName>-<uuid> name per installation (REQ-CNT-019,
// REQ-CNT-020), a container storage per demo with the broker as seen in the
// compose network and own credentials that the non-root container user may
// read (the run directory keeps other users of the host out), the
// applications the config service asks at every connect.
func TestDemoStorages(t *testing.T) {
	out, _ := generated(t)
	env := demoEnv(t, out)
	if len(env) != len(demos)+1 {
		t.Fatalf("%s: %v", demosEnv, env)
	}
	lines := strings.Split(strings.TrimSpace(string(read(t, out, "broker/passwd"))), "\n")
	names := map[string]string{}
	for i, d := range demos {
		name := env[d.env]
		m := containerName.FindStringSubmatch(name)
		if m == nil || m[1] != d.user || len(name) > 63 {
			t.Fatalf("%s=%q", d.env, name)
		}
		names[d.user] = name
		root := "roots/" + d.user + "/"
		var b map[string]any
		if err := json.Unmarshal(read(t, out, root+"etc/oi4/mqtt/broker.json"), &b); err != nil {
			t.Fatal(err)
		}
		if len(b) != 3 || b["Address"] != "mosquitto" || b["SecurePort"] != float64(8883) || b["MaxPacketSize"] != float64(maxPacketSize) {
			t.Errorf("%s broker.json: %v", d.user, b)
		}
		for _, rel := range []string{"etc/oi4/certs/ca.pem", "etc/oi4/mqtt/broker.pem"} {
			if !bytes.Equal(read(t, out, root+rel), read(t, out, "broker/"+filepath.Base(rel))) {
				t.Errorf("%s/%s differs from the broker side", d.user, rel)
			}
		}
		user, password, _ := strings.Cut(decode64(t, read(t, out, root+"run/secrets/oi4/credentials")), ":")
		if user != d.user || len(password) < 32 || len(lines) != 2+len(demos) || !verify(t, lines[2+i], user, password) {
			t.Errorf("the credentials of %s do not match passwd line %d", d.user, 3+i)
		}
		for _, rel := range []string{"etc/hostname", "etc/oi4/app", "run/secrets/oi4/private_key.pem"} {
			if _, err := os.Stat(filepath.Join(out, root+rel)); err == nil {
				t.Errorf("%s: unexpected %s (the engine sets the hostname, the volumes are named)", d.user, rel)
			}
		}
		if runtime.GOOS != "windows" {
			for rel, want := range map[string]os.FileMode{"run": 0o700, "run/secrets/oi4": 0o755, "run/secrets/oi4/credentials": 0o644} {
				fi, err := os.Stat(filepath.Join(out, root+rel))
				if err != nil {
					t.Fatal(err)
				}
				if fi.Mode().Perm() != want {
					t.Errorf("%s/%s: mode %v, want %v", d.user, rel, fi.Mode().Perm(), want)
				}
			}
		}
	}
	var settings struct {
		Apps []string `json:"apps"`
	}
	dec := json.NewDecoder(bytes.NewReader(read(t, out, "roots/oec-config/"+configServiceFile)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&settings); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Utility/oi4.org/OEC,20Logger/OEC-LOGGER/" + names["oec-logger"],
		"OTConnector/oi4.org/OEC,20Weather/OEC-WEATHER/" + names["oec-weather"],
	}
	if strings.Join(settings.Apps, " ") != strings.Join(want, " ") {
		t.Errorf("config-service.json apps %v, want %v", settings.Apps, want)
	}
	// a new installation gets new names
	out2, _ := generated(t)
	if env2 := demoEnv(t, out2); env2["OEC_LOGGER_NAME"] == env["OEC_LOGGER_NAME"] {
		t.Error("the container names are not per installation")
	}
}

// The OEC Registry of the compose profile registry (ADR-0029 §12, C-090,
// A-416): a <DefaultContainerName>-<uuid> name (REQ-CNT-019, REQ-CNT-020),
// the broker as seen in the compose network on the client certificate
// listener, an application certificate of the dev CA with the container
// name as CN (use_identity_as_username) that also serves the HTTPS ports
// for the container name, the compose service name and loopback, and its
// encrypted key with the passphrase – readable by the non-root container
// user, the run directory keeps other users of the host out – and the
// password of the first user admin of the Web UI and API (ADR-0035). No
// credentials, no password file line.
func TestRegistryStorage(t *testing.T) {
	out, _ := generated(t)
	name := demoEnv(t, out)[registryEnv]
	if m := containerName.FindStringSubmatch(name); m == nil || m[1] != registryUser || len(name) > 63 {
		t.Fatalf("%s=%q", registryEnv, name)
	}
	root := "roots/" + registryUser + "/"
	var b map[string]any
	if err := json.Unmarshal(read(t, out, root+"etc/oi4/mqtt/broker.json"), &b); err != nil {
		t.Fatal(err)
	}
	if len(b) != 3 || b["Address"] != "mosquitto" || b["SecurePort"] != float64(8884) || b["MaxPacketSize"] != float64(maxPacketSize) {
		t.Errorf("broker.json: %v", b)
	}
	for _, rel := range []string{"etc/oi4/certs/ca.pem", "etc/oi4/mqtt/broker.pem"} {
		if !bytes.Equal(read(t, out, root+rel), read(t, out, "broker/"+filepath.Base(rel))) {
			t.Errorf("%s differs from the broker side", rel)
		}
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert(t, out, "broker/ca.pem"))
	certRel := root + "etc/oi4/certs/" + name + ".pem"
	c := cert(t, out, certRel)
	if c.Subject.CommonName != name {
		t.Errorf("CN %q, want the container name", c.Subject.CommonName)
	}
	if _, err := c.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		t.Errorf("client authentication: %v", err)
	}
	for _, host := range []string{name, registryUser, "localhost", "127.0.0.1", "::1"} {
		if _, err := c.Verify(x509.VerifyOptions{DNSName: host, Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
			t.Errorf("HTTPS for %s: %v", host, err)
		}
	}
	if _, err := c.Verify(x509.VerifyOptions{DNSName: "mosquitto", Roots: roots}); err == nil {
		t.Error("the registry certificate is valid for the broker name")
	}
	checkEncryptedKey(t, out, root, certRel)
	admin := strings.TrimSuffix(string(read(t, out, root+"run/secrets/oi4/registry_admin_password")), "\n")
	if len(admin) < 12 || len(admin) > 256 || strings.ContainsAny(admin, " \t\r\n") {
		t.Errorf("registry_admin_password: %d characters", len(admin))
	}
	for _, rel := range []string{"etc/hostname", "etc/oi4/app", "run/secrets/oi4/credentials"} {
		if _, err := os.Stat(filepath.Join(out, root+rel)); err == nil {
			t.Errorf("unexpected %s", rel)
		}
	}
	if strings.Contains(string(read(t, out, "broker/passwd")), registryUser) {
		t.Error("the registry has a password file line")
	}
	if strings.Contains(string(read(t, out, "roots/oec-config/"+configServiceFile)), registryUser) {
		t.Error("the config service asks the registry")
	}
	if runtime.GOOS != "windows" {
		for rel, want := range map[string]os.FileMode{"run": 0o700, "run/secrets/oi4": 0o755,
			"run/secrets/oi4/private_key.pem": 0o644, "run/secrets/oi4/passphrase": 0o644,
			"run/secrets/oi4/registry_admin_password": 0o644} {
			fi, err := os.Stat(filepath.Join(out, root+rel))
			if err != nil {
				t.Fatal(err)
			}
			if fi.Mode().Perm() != want {
				t.Errorf("%s: mode %v, want %v", rel, fi.Mode().Perm(), want)
			}
		}
	}
}
