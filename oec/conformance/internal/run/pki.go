package run

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"time"

	"github.com/OI4/oec/conformance/internal/mqttwire"
)

// pki is the test PKI of one run, made at run time and never written
// outside the storages of the run (G-4.5): CA A is the CA of ca.pem and
// signs the client certificates, CA B the CA of broker_ca.pem, CA X a CA
// the container does not know.
type pki struct {
	a, b, x *authority
}

type authority struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pem  []byte
}

func newPKI() (*pki, error) {
	p := &pki{}
	var err error
	for _, c := range []struct {
		dst **authority
		cn  string
	}{{&p.a, "oec-d8 run CA"}, {&p.b, "oec-d8 run broker CA"}, {&p.x, "oec-d8 run untrusted CA"}} {
		if *c.dst, err = newAuthority(c.cn); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func serial() *big.Int {
	n, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	return n.Add(n, big.NewInt(1))
}

func newAuthority(cn string) (*authority, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	tpl := &x509.Certificate{
		SerialNumber: serial(), Subject: pkix.Name{CommonName: cn},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &authority{cert: cert, key: key, pem: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}, nil
}

// leaf is a certificate with its key.
type leaf struct {
	certPEM []byte
	key     *ecdsa.PrivateKey
}

func (a *authority) issue(cn string, host string, usage x509.ExtKeyUsage) (*leaf, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	tpl := &x509.Certificate{
		SerialNumber: serial(), Subject: pkix.Name{CommonName: cn},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage},
	}
	if host != "" {
		if ip := net.ParseIP(host); ip != nil {
			tpl.IPAddresses = []net.IP{ip}
		} else {
			tpl.DNSNames = []string{host}
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, a.cert, &key.PublicKey, a.key)
	if err != nil {
		return nil, err
	}
	return &leaf{certPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), key: key}, nil
}

// broker returns the server certificate of the endpoint for the broker
// address of broker.json (RIG 4.1.1, SF-115).
func (a *authority) broker(host string) (*leaf, tls.Certificate, error) {
	l, err := a.issue("oec-d8 run broker", host, x509.ExtKeyUsageServerAuth)
	if err != nil {
		return nil, tls.Certificate{}, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(l.key)
	if err != nil {
		return nil, tls.Certificate{}, err
	}
	c, err := tls.X509KeyPair(l.certPEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	return l, c, err
}

// client returns the certificate of the container (common name = container
// name) and its key as encrypted PKCS#8 (RIG 4.1.2, 4.1.3).
func (a *authority) client(name string, passphrase []byte) (certPEM, keyPEM []byte, err error) {
	l, err := a.issue(name, name, x509.ExtKeyUsageClientAuth)
	if err != nil {
		return nil, nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(l.key)
	if err != nil {
		return nil, nil, err
	}
	enc, err := mqttwire.EncryptPKCS8(der, passphrase)
	if err != nil {
		return nil, nil, err
	}
	return l.certPEM, pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: enc}), nil
}

func (a *authority) pool() *x509.CertPool {
	p := x509.NewCertPool()
	p.AddCert(a.cert)
	return p
}
