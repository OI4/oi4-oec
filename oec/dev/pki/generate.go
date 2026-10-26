package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Names of the sample containers (container name = hostname = client
// certificate CN, REQ-CNT-019, REQ-CNT-021) and of the probe user of the
// interoperability tests.
const (
	appName   = "oec-dev-app"
	certName  = "oec-dev-cert"
	probeUser = "oec-probe"
)

// MaxPacketSize of the sample broker.json: the minimum of RIG 4.1.1. The
// Mosquitto configuration limits the payload to it (message_size_limit) and
// the whole packet to it plus the largest PUBLISH header (SF-105).
const maxPacketSize = 262144

// pbes2Iterations is the PBKDF2 iteration count of the encrypted private key,
// the default of `openssl pkcs8 -topk8 -v2 aes-256-cbc` (OpenSSL 3).
const pbes2Iterations = 2048

// mosquittoIterations is the PBKDF2-SHA512 iteration count that
// mosquitto_passwd 2.1 writes for `$7$` hashes.
const mosquittoIterations = 1000

// marker identifies an output directory written by this tool; -force only
// replaces such a directory.
const marker = ".oec-dev-pki"

type options struct {
	out      string
	force    bool
	port     int
	certPort int
	address  string
	sans     []string
	now      time.Time
	rand     io.Reader
}

func defaults() options {
	return options{
		out:      "generated",
		port:     8883,
		certPort: 8884,
		address:  "localhost",
		now:      time.Now(),
		rand:     rand.Reader,
	}
}

// issued is a certificate with its key.
type issued struct {
	cert *x509.Certificate
	der  []byte
	key  *ecdsa.PrivateKey
}

func (i issued) pem() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: i.der})
}

func (i issued) keyPEM() ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(i.key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

func generate(o options) error {
	if err := prepare(o.out, o.force); err != nil {
		return err
	}
	ca, err := newCA(o)
	if err != nil {
		return err
	}
	broker, err := newLeaf(o, ca, "mosquitto", x509.ExtKeyUsageServerAuth, append([]string{"localhost", "mosquitto", "127.0.0.1", "::1"}, o.sans...))
	if err != nil {
		return err
	}
	client, err := newLeaf(o, ca, certName, x509.ExtKeyUsageClientAuth, nil)
	if err != nil {
		return err
	}
	probe, err := newLeaf(o, ca, probeUser, x509.ExtKeyUsageClientAuth, nil)
	if err != nil {
		return err
	}
	probeKey, err := probe.keyPEM()
	if err != nil {
		return err
	}
	appPassword, err := secret(o.rand)
	if err != nil {
		return err
	}
	probePassword, err := secret(o.rand)
	if err != nil {
		return err
	}
	passphrase, err := secret(o.rand)
	if err != nil {
		return err
	}
	var passwd []byte
	for _, u := range [][2]string{{appName, appPassword}, {probeUser, probePassword}} {
		line, err := mosquittoHash(o.rand, u[0], u[1])
		if err != nil {
			return err
		}
		passwd = append(passwd, line+"\n"...)
	}
	brokerKey, err := broker.keyPEM()
	if err != nil {
		return err
	}
	encrypted, err := encryptPKCS8(o.rand, client.key, passphrase)
	if err != nil {
		return err
	}

	w := writer{root: o.out}
	w.file(marker, []byte("written by `go run ./pki` (oec/dev); development secrets, never commit\n"), 0o644)
	// the broker side, mounted read-only into the container
	w.file("broker/ca.pem", ca.pem(), 0o644)
	w.file("broker/broker.pem", broker.pem(), 0o644)
	w.file("broker/broker.key", brokerKey, 0o600)
	w.file("broker/passwd", passwd, 0o600)
	// the probe client of the interoperability tests: plain files, so the
	// tests need neither base64 nor key decryption
	w.file("probe/username", []byte(probeUser+"\n"), 0o644)
	w.file("probe/password", []byte(probePassword+"\n"), 0o600)
	w.file("probe/probe.pem", probe.pem(), 0o644)
	w.file("probe/probe.key", probeKey, 0o600)
	// container storage with credentials (REQ-CNT-003 … 009)
	w.storage(appName, o.address, o.port, ca, broker)
	w.file("roots/"+appName+"/run/secrets/oi4/credentials", credentials(appName, appPassword), 0o600)
	// container storage with client certificate and encrypted key
	w.storage(certName, o.address, o.certPort, ca, broker)
	w.file("roots/"+certName+"/etc/oi4/certs/"+certName+".pem", client.pem(), 0o644)
	w.file("roots/"+certName+"/run/secrets/oi4/private_key.pem", encrypted, 0o600)
	w.file("roots/"+certName+"/run/secrets/oi4/passphrase", []byte(base64.StdEncoding.EncodeToString([]byte(passphrase))+"\n"), 0o600)
	return w.err
}

// prepare creates an empty output directory; an existing one is replaced
// only with force and only if this tool wrote it.
func prepare(out string, force bool) error {
	if _, err := os.Stat(out); err == nil {
		if !force {
			return fmt.Errorf("%s exists; use -force to replace it", out)
		}
		if _, err := os.Stat(filepath.Join(out, marker)); err != nil {
			return fmt.Errorf("%s was not written by this tool (no %s); not replaced", out, marker)
		}
		if err := os.RemoveAll(out); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.MkdirAll(out, 0o755)
}

type writer struct {
	root string
	err  error
}

func (w *writer) file(rel string, content []byte, mode os.FileMode) {
	if w.err != nil {
		return
	}
	path := filepath.Join(w.root, filepath.FromSlash(rel))
	if w.err = os.MkdirAll(filepath.Dir(path), 0o755); w.err != nil {
		return
	}
	w.err = os.WriteFile(path, content, mode)
}

// storage writes the parts of a container storage common to both samples:
// hostname, Message Bus storage, OI4 certificate storage and the
// application specific volume (RIG 4.1.1, 4.1.2, 4.1.4).
func (w *writer) storage(name, address string, port int, ca, broker issued) {
	if w.err != nil {
		return
	}
	brokerJSON, err := json.MarshalIndent(struct {
		Address       string
		SecurePort    int
		MaxPacketSize int
	}{address, port, maxPacketSize}, "", "  ")
	if err != nil {
		w.err = err
		return
	}
	base := "roots/" + name + "/"
	w.file(base+"etc/hostname", []byte(name+"\n"), 0o644)
	w.file(base+"etc/oi4/mqtt/broker.json", append(brokerJSON, '\n'), 0o644)
	w.file(base+"etc/oi4/mqtt/broker.pem", broker.pem(), 0o644)
	w.file(base+"etc/oi4/certs/ca.pem", ca.pem(), 0o644)
	if w.err == nil {
		w.err = os.MkdirAll(filepath.Join(w.root, filepath.FromSlash(base+"etc/oi4/app")), 0o755)
	}
}

func serial(r io.Reader) (*big.Int, error) {
	return rand.Int(r, new(big.Int).Lsh(big.NewInt(1), 127))
}

func newCA(o options) (issued, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), o.rand)
	if err != nil {
		return issued{}, err
	}
	sn, err := serial(o.rand)
	if err != nil {
		return issued{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          sn,
		Subject:               pkix.Name{CommonName: "OEC dev CA (never use in production)", Organization: []string{"OEC dev"}},
		NotBefore:             o.now.Add(-time.Hour),
		NotAfter:              o.now.AddDate(3, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(o.rand, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return issued{}, err
	}
	cert, err := x509.ParseCertificate(der)
	return issued{cert, der, key}, err
}

// newLeaf issues a server or client certificate; names are DNS names or IP
// addresses of the subject alternative name.
func newLeaf(o options, ca issued, cn string, usage x509.ExtKeyUsage, names []string) (issued, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), o.rand)
	if err != nil {
		return issued{}, err
	}
	sn, err := serial(o.rand)
	if err != nil {
		return issued{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: sn,
		Subject:      pkix.Name{CommonName: cn, Organization: []string{"OEC dev"}},
		NotBefore:    o.now.Add(-time.Hour),
		NotAfter:     o.now.AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
	}
	for _, n := range names {
		if ip := net.ParseIP(n); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else if n != "" {
			tmpl.DNSNames = append(tmpl.DNSNames, n)
		}
	}
	der, err := x509.CreateCertificate(o.rand, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		return issued{}, err
	}
	cert, err := x509.ParseCertificate(der)
	return issued{cert, der, key}, err
}

// secret is a random password or passphrase (24 random bytes, base64url).
func secret(r io.Reader) (string, error) {
	b := make([]byte, 24)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// credentials is the secret storage file: base64(<username>:<password>)
// (RIG 4.1.3, REQ-CNT-009).
func credentials(user, password string) []byte {
	return []byte(base64.StdEncoding.EncodeToString([]byte(user+":"+password)) + "\n")
}

// mosquittoHash is a password file line as mosquitto_passwd writes it with
// `-H sha512-pbkdf2`: user:$7$<iterations>$<base64 salt>$<base64 hash>.
func mosquittoHash(r io.Reader, user, password string) (string, error) {
	salt := make([]byte, 64)
	if _, err := io.ReadFull(r, salt); err != nil {
		return "", err
	}
	hash, err := pbkdf2.Key(sha512.New, password, salt, mosquittoIterations, sha512.Size)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:$7$%d$%s$%s", user, mosquittoIterations,
		base64.StdEncoding.EncodeToString(salt), base64.StdEncoding.EncodeToString(hash)), nil
}

// OIDs of PKCS#5 v2.1 (RFC 8018) and NIST AES.
var (
	oidPBES2          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}
	oidPBKDF2         = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 12}
	oidHMACWithSHA256 = asn1.ObjectIdentifier{1, 2, 840, 113549, 2, 9}
	oidAES256CBC      = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 42}
)

type algorithm struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"`
}

type pbkdf2Params struct {
	Salt           []byte
	IterationCount int
	PRF            algorithm
}

type pbes2Params struct {
	KeyDerivationFunc algorithm
	EncryptionScheme  algorithm
}

type encryptedPrivateKeyInfo struct {
	EncryptionAlgorithm algorithm
	EncryptedData       []byte
}

func raw(v any) (asn1.RawValue, error) {
	b, err := asn1.Marshal(v)
	return asn1.RawValue{FullBytes: b}, err
}

// encryptPKCS8 writes key as PKCS#8 `ENCRYPTED PRIVATE KEY` with PBES2
// (PBKDF2-HMAC-SHA256, AES-256-CBC), as `openssl pkcs8 -topk8 -v2
// aes-256-cbc` does (REQ-CNT-008, ADR-0023 amendment C-050).
func encryptPKCS8(r io.Reader, key *ecdsa.PrivateKey, passphrase string) ([]byte, error) {
	plain, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	salt, iv := make([]byte, 16), make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(r, salt); err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(r, iv); err != nil {
		return nil, err
	}
	dk, err := pbkdf2.Key(sha256.New, passphrase, salt, pbes2Iterations, 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(dk)
	if err != nil {
		return nil, err
	}
	pad := aes.BlockSize - len(plain)%aes.BlockSize
	padded := append(plain, make([]byte, pad)...)
	for i := len(plain); i < len(padded); i++ {
		padded[i] = byte(pad)
	}
	data := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(data, padded)

	null, err := raw(asn1.NullRawValue)
	if err != nil {
		return nil, err
	}
	kdf, err := raw(pbkdf2Params{salt, pbes2Iterations, algorithm{oidHMACWithSHA256, null}})
	if err != nil {
		return nil, err
	}
	ivRaw, err := raw(iv)
	if err != nil {
		return nil, err
	}
	params, err := raw(pbes2Params{algorithm{oidPBKDF2, kdf}, algorithm{oidAES256CBC, ivRaw}})
	if err != nil {
		return nil, err
	}
	der, err := asn1.Marshal(encryptedPrivateKeyInfo{algorithm{oidPBES2, params}, data})
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: der}), nil
}
