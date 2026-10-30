package host

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/OI4/oec/conformance/internal/mqttwire"
)

// The files of the secret storage (RIG 4.1.3).
const (
	fileCredentials = "credentials"
	fileKey         = "private_key.pem"
	filePassphrase  = "passphrase"
)

// username: the unreserved characters of RFC 3986 and "@" (REQ-MQTT-011).
var username = regexp.MustCompile(`^[A-Za-z0-9._~@-]+$`)

// identity is what the storages provide for the login of the application.
type identity struct {
	user        string
	password    []byte
	credentials bool // valid credentials
	key         crypto.PublicKey
	keyFile     string // private_key.pem that D8 could read and decrypt
	passphrase  []byte
	certFile    string // the certificate of the key in the OI4 certificate storage
}

// certificateStorages judges T8-HST-02: the OI4 certificate storage with
// ca.pem and the certificate of the application, the secret storage with
// credentials, private_key.pem and passphrase, both mounted read-only, and
// no private keys outside the secret storage (RIG 4.1.2, 4.1.3).
func (r *runner) certificateStorages() {
	r.noPrivateKeys()
	r.certificateStorage()
	id := r.secretStorage(TSecrets)
	id = r.applicationCertificate(TSecrets, id)
	r.authentication(id)
}

// noPrivateKeys judges REQ-CNT-002: no private key in the Message Bus and
// the OI4 certificate storage.
func (r *runner) noPrivateKeys() {
	scanned := false
	for _, s := range []*store{r.mqtt, r.certs} {
		if !s.exists {
			continue
		}
		scanned = true
		names := sortedNames(s)
		for _, n := range names {
			lower := strings.ToLower(n)
			if strings.HasSuffix(lower, ".key") || strings.HasSuffix(lower, ".p12") || strings.HasSuffix(lower, ".pfx") {
				r.violation(TSecrets, reqNoKeys, s.path(n), "%s: the name of a private key in %s; private keys belong to the secret storage (RIG 4.1.3)", n, s.dir)
				continue
			}
			data, ok, err := s.read(n)
			if err != nil || !ok {
				continue
			}
			for rest := data; ; {
				var b *pem.Block
				b, rest = pem.Decode(rest)
				if b == nil {
					break
				}
				if strings.HasSuffix(b.Type, "PRIVATE KEY") {
					r.violation(TSecrets, reqNoKeys, s.path(n), "%s holds a private key (%s) in %s; private keys belong to the secret storage (RIG 4.1.3)", n, b.Type, s.dir)
					break
				}
			}
		}
	}
	if scanned {
		r.checked(TSecrets, reqNoKeys, "", "Message Bus and OI4 certificate storage scanned for private keys")
	}
}

func sortedNames(s *store) []string {
	out := make([]string, 0, len(s.entries))
	for n, fi := range s.entries {
		if fi.Mode().IsRegular() {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// certificateStorage judges REQ-CNT-005/006: the OI4 certificate storage
// exists, is mounted read-only and has ca.pem with a CA certificate.
func (r *runner) certificateStorage() {
	s := r.certs
	switch {
	case s.err != nil:
		r.inconclusive(TSecrets, []string{reqCA, reqCertStorage}, s.dir, "%s", unreadable(s.dir, s.err))
		return
	case !s.exists:
		r.violation(TSecrets, reqCertStorage, s.dir, "the OI4 certificate storage %s does not exist (RIG 4.1.2)", s.dir)
		r.violation(TSecrets, reqCA, s.dir, "no ca.pem: the OI4 certificate storage %s does not exist (RIG 4.1.2)", s.dir)
		return
	}
	r.checked(TSecrets, reqCertStorage, s.dir, "OI4 certificate storage %s", s.dir)
	r.judgeMount(TSecrets, reqCertStorage, s)
	r.worldWritable(TSecrets, reqCertStorage, s)
	file := s.path("ca.pem")
	data, ok, err := s.read("ca.pem")
	switch {
	case err != nil:
		r.inconclusive(TSecrets, []string{reqCA}, file, "%s", unreadable(file, err))
		return
	case !ok:
		r.violation(TSecrets, reqCertStorage, file, "no ca.pem in the OI4 certificate storage (RIG 4.1.2)")
		r.violation(TSecrets, reqCA, file, "no ca.pem in the OI4 certificate storage (RIG 4.1.2)")
		return
	}
	cas, err := pemCertificates(data)
	if err != nil {
		r.violation(TSecrets, reqCA, file, "ca.pem: %v", err)
		return
	}
	for _, c := range cas {
		if c.IsCA && c.BasicConstraintsValid {
			r.checked(TSecrets, reqCA, file, "ca.pem holds the CA certificate %q", c.Subject)
			return
		}
	}
	r.violation(TSecrets, reqCA, file, "ca.pem holds no CA certificate (basic constraints CA:TRUE): %q", cas[0].Subject)
}

// secretStorage judges REQ-CNT-008 and REQ-MQTT-011 (test "" only reads
// the identity for the connection of T8-HST-04).
func (r *runner) secretStorage(test string) identity {
	var id identity
	s := r.secrets
	judge := test != ""
	violation := func(file, format string, a ...any) {
		if judge {
			r.violation(test, reqSecrets, file, format, a...)
		}
	}
	switch {
	case s.err != nil:
		if judge {
			r.inconclusive(test, []string{reqSecrets, reqUsername}, s.dir, "%s", unreadable(s.dir, s.err))
		}
		return id
	case !s.exists:
		return id
	}
	if judge {
		r.judgeMount(test, reqSecrets, s)
		r.worldWritable(test, reqSecrets, s)
		for _, n := range sortedEntries(s) {
			if n != fileCredentials && n != fileKey && n != filePassphrase {
				violation(s.path(n), "%s: the secret storage contains only credentials, private_key.pem and passphrase; further secrets are provided outside of it (RIG 4.1.3)", n)
			}
		}
	}
	file := s.path(fileCredentials)
	if data, ok, err := s.read(fileCredentials); err != nil {
		if judge {
			r.inconclusive(test, []string{reqSecrets, reqUsername}, file, "%s", unreadable(file, err))
		}
	} else if ok {
		dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
		user, pass, found := strings.Cut(string(dec), ":")
		switch {
		case err != nil:
			violation(file, "credentials are not base64 encoded: %v (RIG 4.1.3)", err)
		case !found || user == "":
			violation(file, "credentials are not base64(<username>:<password>) (RIG 4.1.3)")
		default:
			id.user, id.password, id.credentials = user, []byte(pass), true
			if judge && username.MatchString(user) {
				r.checked(test, reqUsername, file, "username %q", user)
			} else if judge {
				r.violation(test, reqUsername, file, "username %q has characters other than the unreserved characters of RFC 3986 and \"@\"", user)
			}
		}
	}
	r.privateKey(test, &id)
	return id
}

func sortedEntries(s *store) []string {
	out := make([]string, 0, len(s.entries))
	for n := range s.entries {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// privateKey reads private_key.pem and passphrase (REQ-CNT-008).
func (r *runner) privateKey(test string, id *identity) {
	s := r.secrets
	judge := test != ""
	violation := func(file, format string, a ...any) {
		if judge {
			r.violation(test, reqSecrets, file, format, a...)
		}
	}
	keyFile, ppFile := s.path(fileKey), s.path(filePassphrase)
	data, hasKey, err := s.read(fileKey)
	if err != nil {
		if judge {
			r.inconclusive(test, []string{reqSecrets}, keyFile, "%s", unreadable(keyFile, err))
		}
		return
	}
	pp, hasPP, err := s.read(filePassphrase)
	if err != nil {
		if judge {
			r.inconclusive(test, []string{reqSecrets}, ppFile, "%s", unreadable(ppFile, err))
		}
		return
	}
	if !hasKey {
		if hasPP {
			violation(ppFile, "passphrase without private_key.pem; each file is present only if used (RIG 4.1.3)")
		}
		return
	}
	var block *pem.Block
	for rest := data; ; {
		var b *pem.Block
		b, rest = pem.Decode(rest)
		if b == nil {
			break
		}
		if strings.HasSuffix(b.Type, "PRIVATE KEY") {
			block = b
			break
		}
	}
	if block == nil {
		violation(keyFile, "private_key.pem holds no PEM private key (RIG 4.1.3)")
		return
	}
	der := block.Bytes
	var passphrase []byte
	if hasPP {
		passphrase, err = base64.StdEncoding.DecodeString(strings.TrimSpace(string(pp)))
		if err != nil {
			violation(ppFile, "passphrase is not base64 encoded: %v (RIG 4.1.3)", err)
			return
		}
	}
	switch {
	case block.Headers["DEK-Info"] != "":
		if judge {
			r.inconclusive(test, []string{reqSecrets}, keyFile, "private_key.pem is encrypted with the legacy PEM encryption (DEK-Info), which D8 does not read; it reads PKCS#8 (ENCRYPTED PRIVATE KEY)")
		}
		return
	case block.Type == "ENCRYPTED PRIVATE KEY":
		if !hasPP {
			violation(keyFile, "private_key.pem is encrypted and there is no passphrase (RIG 4.1.3)")
			return
		}
		der, err = mqttwire.DecryptPKCS8(der, passphrase)
		if err != nil {
			violation(ppFile, "the passphrase does not decrypt private_key.pem: %v (RIG 4.1.3)", err)
			return
		}
	case hasPP:
		violation(ppFile, "passphrase although private_key.pem is not encrypted; the passphrase is present only if the key is encrypted (RIG 4.1.3)")
	}
	key, err := parseKey(block.Type, der)
	if err != nil {
		violation(keyFile, "private_key.pem: %v", err)
		return
	}
	id.key, id.keyFile, id.passphrase = key, keyFile, passphrase
}

// parseKey parses a decrypted private key and returns its public key.
func parseKey(typ string, der []byte) (crypto.PublicKey, error) {
	var k any
	var err error
	switch typ {
	case "EC PRIVATE KEY":
		k, err = x509.ParseECPrivateKey(der)
	case "RSA PRIVATE KEY":
		k, err = x509.ParsePKCS1PrivateKey(der)
	default:
		k, err = x509.ParsePKCS8PrivateKey(der)
	}
	if err != nil {
		return nil, fmt.Errorf("no valid private key: %v", err)
	}
	if s, ok := k.(crypto.Signer); ok {
		return s.Public(), nil
	}
	return nil, errors.New("no valid private key")
}

func samePublicKey(a, b crypto.PublicKey) bool {
	switch k := a.(type) {
	case *ecdsa.PublicKey:
		return k.Equal(b)
	case *rsa.PublicKey:
		return k.Equal(b)
	case ed25519.PublicKey:
		return k.Equal(b)
	}
	return false
}

// applicationCertificate finds the certificate of private_key.pem in the
// OI4 certificate storage: <containername>.pem (REQ-CNT-006/007/008).
func (r *runner) applicationCertificate(test string, id identity) identity {
	if id.key == nil || !r.certs.exists {
		return id
	}
	judge := test != ""
	c := r.certs
	name := r.c.Container + ".pem"
	if r.c.Container != "" && c.has(name) {
		file := c.path(name)
		data, _, err := c.read(name)
		if err != nil {
			if judge {
				r.inconclusive(test, []string{reqCertStorage, reqSecrets}, file, "%s", unreadable(file, err))
			}
			return id
		}
		chain, err := pemCertificates(data)
		switch {
		case err != nil:
			if judge {
				r.violation(test, reqCertStorage, file, "%s: %v", name, err)
			}
		case !samePublicKey(id.key, chain[0].PublicKey):
			if judge {
				r.violation(test, reqSecrets, id.keyFile, "private_key.pem is not the key of %s (RIG 4.1.3)", name)
				r.checked(test, reqAppCert, file, "certificate of the application %s", name)
			}
		default:
			id.certFile = file
			if judge {
				r.checked(test, reqCertStorage, file, "certificate of the application %s", name)
				r.checked(test, reqAppCert, file, "certificate of the application %s", name)
				r.checked(test, reqSecrets, id.keyFile, "private_key.pem is the key of %s", name)
			}
		}
		return id
	}
	for _, n := range sortedNames(c) {
		if n == "ca.pem" || !strings.HasSuffix(n, ".pem") {
			continue
		}
		data, _, err := c.read(n)
		if err != nil {
			continue
		}
		chain, err := pemCertificates(data)
		if err != nil || !samePublicKey(id.key, chain[0].PublicKey) {
			continue
		}
		id.certFile = c.path(n)
		if judge {
			r.checked(test, reqCertStorage, id.certFile, "certificate of the application %s", n)
			if r.c.Container == "" {
				r.inconclusive(test, []string{reqAppCert}, id.certFile, "the container name is unknown (no etc/hostname below the root; --container)")
			} else {
				r.violation(test, reqAppCert, id.certFile, "the certificate of the application is %s, not %s (the container name, RIG 4.1.2)", n, name)
			}
		}
		return id
	}
	if judge {
		want := name
		if r.c.Container == "" {
			want = "<containername>.pem"
		}
		r.violation(test, reqCertStorage, c.dir, "no certificate of the application for private_key.pem (%s) in the OI4 certificate storage (RIG 4.1.2)", want)
	}
	return id
}

// authentication judges whether the secret storage provides at least one
// means of authentication (REQ-CNT-008).
func (r *runner) authentication(id identity) {
	s := r.secrets
	if s.err != nil {
		return
	}
	if !s.exists {
		r.violation(TSecrets, reqSecrets, s.dir, "the secret storage %s does not exist: no means of authentication (RIG 4.1.3)", s.dir)
		return
	}
	var means []string
	if id.credentials {
		means = append(means, "credentials")
	}
	if id.certFile != "" {
		means = append(means, "private_key.pem with the certificate of the application")
	}
	if len(means) == 0 {
		if !hasInconclusive(r, TSecrets, reqSecrets) {
			r.violation(TSecrets, reqSecrets, s.dir, "no means of authentication: neither credentials nor private_key.pem with the certificate of the application (RIG 4.1.3)")
		}
		return
	}
	r.checked(TSecrets, reqSecrets, s.dir, "means of authentication: %s", strings.Join(means, ", "))
}

func hasInconclusive(r *runner, test, req string) bool {
	for _, o := range r.obs {
		if o.Test == test && o.Req == req && o.Inconclusive {
			return true
		}
	}
	return false
}

// connection is the login of the application of the root for T8-HST-04.
func (r *runner) connection() (mqttwire.Config, error) {
	var c mqttwire.Config
	data, ok, err := r.mqtt.read("broker.json")
	if err != nil || !ok {
		return c, fmt.Errorf("no readable broker.json in %s", r.mqtt.dir)
	}
	b, err := parseBroker(data)
	if err != nil {
		return c, err
	}
	c.Broker = b
	switch {
	case r.mqtt.has("broker_ca.pem"):
		c.CAFile = r.mqtt.path("broker_ca.pem")
	case r.certs.has("ca.pem"):
		c.CAFile = r.certs.path("ca.pem")
	default:
		return c, errors.New("neither broker_ca.pem nor ca.pem")
	}
	id := r.applicationCertificate("", r.secretStorage(""))
	if id.credentials {
		c.Username, c.Password = id.user, id.password
	}
	if id.certFile != "" {
		c.CertFile, c.KeyFile, c.KeyPassphrase = id.certFile, id.keyFile, id.passphrase
	}
	if c.Username == "" && c.CertFile == "" {
		return c, fmt.Errorf("no usable means of authentication in %s", r.secrets.dir)
	}
	return c, nil
}

// parseBroker returns the broker URL of broker.json.
func parseBroker(data []byte) (string, error) {
	var b struct {
		Address    string
		SecurePort int
	}
	dec := jsonDecoder(bytes.NewReader(data))
	if err := dec.Decode(&b); err != nil {
		return "", fmt.Errorf("broker.json: %v", err)
	}
	if b.Address == "" || b.SecurePort < 1 || b.SecurePort > 65535 {
		return "", errors.New("broker.json has no usable Address and SecurePort")
	}
	return brokerURL(b.Address, b.SecurePort), nil
}
