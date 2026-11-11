package main

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/proxy"
)

// usesProxy reports whether a --proxy-* flag is set, and checks that they
// come together.
func (f *monitorFlags) usesProxy() (bool, error) {
	if f.proxyListen == "" {
		if f.proxyCA != "" || f.proxyCAKey != "" || f.proxyNames != "" || f.proxyUpCert != "" || f.proxyUpKey != "" {
			return false, errors.New("--proxy-ca, --proxy-ca-key, --proxy-name, --proxy-upstream-cert and --proxy-upstream-key need --proxy-listen")
		}
		return false, nil
	}
	if f.proxyCA == "" || f.proxyCAKey == "" {
		return false, errors.New("--proxy-listen needs --proxy-ca and --proxy-ca-key (the CA the component under test trusts)")
	}
	if (f.proxyUpCert == "") != (f.proxyUpKey == "") {
		return false, errors.New("--proxy-upstream-cert and --proxy-upstream-key go together")
	}
	if f.proxyWait <= 0 {
		return false, errors.New("--proxy-wait must be positive")
	}
	return true, nil
}

// newProxy binds the proxy attachment (DESIGN.md §3.4): it relays to the
// broker of cfg with the TLS settings of D7 without its client certificate
// (C-113 RT-1) – for the relayed client only --proxy-upstream-cert, the
// identity of the component under test – and issues its certificates with
// --proxy-ca.
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
	var cut *tls.Certificate
	if f.proxyUpCert != "" {
		tc, err := mqttwire.TLSConfig(mqttwire.Config{CertFile: f.proxyUpCert, KeyFile: f.proxyUpKey}, host)
		if err != nil {
			return nil, fmt.Errorf("--proxy-upstream-cert/--proxy-upstream-key: %w", err)
		}
		cut = &tc.Certificates[0]
	}
	var names []string
	for _, n := range strings.Split(f.proxyNames, ",") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	return proxy.New(proxy.Config{Listen: f.proxyListen, Upstream: addr, UpstreamTLS: up, UpstreamCert: cut, CA: ca, CAKey: key, Names: names, HandshakeTimeout: f.timeout})
}

// loopback reports whether the proxy address host:port is a loopback
// address, reachable from this host only.
func loopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// warnProxy warns when the proxy is reachable from other hosts: every
// client that reaches it is relayed to the broker (with its own
// credentials, or with --proxy-upstream-cert).
func (f *monitorFlags) warnProxy(px *proxy.Proxy, broker string, stderr io.Writer) {
	if loopback(px.Addr()) {
		return
	}
	fmt.Fprintf(stderr, "%s: WARNING: the proxy listens on %s, reachable from other hosts: every client that reaches it is relayed to %s", tool, px.Addr(), broker)
	if f.proxyUpCert != "" {
		fmt.Fprintf(stderr, " and authenticated with the client certificate of --proxy-upstream-cert (%s)", f.proxyUpCert)
	}
	fmt.Fprintln(stderr, "; listen on a loopback address unless the component under test runs on another host, and only in a trusted network")
}
