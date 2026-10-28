package main

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"

	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/proxy"
)

// usesProxy reports whether a --proxy-* flag is set, and checks that they
// come together.
func (f *monitorFlags) usesProxy() (bool, error) {
	if f.proxyListen == "" {
		if f.proxyCA != "" || f.proxyCAKey != "" || f.proxyNames != "" {
			return false, errors.New("--proxy-ca, --proxy-ca-key and --proxy-name need --proxy-listen")
		}
		return false, nil
	}
	if f.proxyCA == "" || f.proxyCAKey == "" {
		return false, errors.New("--proxy-listen needs --proxy-ca and --proxy-ca-key (the CA the component under test trusts)")
	}
	if f.proxyWait <= 0 {
		return false, errors.New("--proxy-wait must be positive")
	}
	return true, nil
}

// newProxy binds the proxy attachment (DESIGN.md §3.4): it relays to the
// broker of cfg with the TLS settings of D7 and issues its certificates
// with --proxy-ca.
func (f *monitorFlags) newProxy(cfg mqttwire.Config) (*proxy.Proxy, error) {
	pair, err := tls.LoadX509KeyPair(f.proxyCA, f.proxyCAKey)
	if err != nil {
		return nil, fmt.Errorf("--proxy-ca/--proxy-ca-key: %w", err)
	}
	ca := pair.Leaf
	if ca == nil {
		if ca, err = x509.ParseCertificate(pair.Certificate[0]); err != nil {
			return nil, fmt.Errorf("--proxy-ca: %w", err)
		}
	}
	if !ca.BasicConstraintsValid || !ca.IsCA {
		return nil, fmt.Errorf("--proxy-ca: %s is no CA certificate", ca.Subject)
	}
	key, ok := pair.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, errors.New("--proxy-ca-key: the key cannot sign")
	}
	host, addr, err := mqttwire.BrokerAddress(cfg.Broker)
	if err != nil {
		return nil, err
	}
	up, err := mqttwire.TLSConfig(cfg, host)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, n := range strings.Split(f.proxyNames, ",") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	return proxy.New(proxy.Config{Listen: f.proxyListen, Upstream: addr, UpstreamTLS: up, CA: ca, CAKey: key, Names: names, HandshakeTimeout: f.timeout})
}
