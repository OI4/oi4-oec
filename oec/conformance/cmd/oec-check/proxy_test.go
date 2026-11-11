package main

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/OI4/oec/conformance/internal/mqttwire"
	"github.com/OI4/oec/conformance/internal/mqttwire/mqtttest"
	"github.com/OI4/oec/conformance/internal/proxy"
)

// TestProxyUpstreamIdentity (C-113 RT-1): D7 connects with its own client
// certificate (--cert/--key) to a broker that maps the certificate to the
// user. A client of the proxy without credentials does not get D7's
// identity; with --proxy-upstream-cert/--proxy-upstream-key the proxy
// presents that identity (of the component under test) for it.
func TestProxyUpstreamIdentity(t *testing.T) {
	pki := mqtttest.NewPKI(t)
	b := mqtttest.NewBroker(t, pki, "", "", true)
	for _, c := range []struct {
		upstream []string
		relayed  bool
	}{
		{nil, false},
		{[]string{"--proxy-upstream-cert", pki.ClientCert, "--proxy-upstream-key", pki.ClientKey}, true},
	} {
		f := newMonitorFlags("test", io.Discard)
		args := append([]string{"--broker", b.URL(), "--ca", pki.CAFile, "--cert", pki.ClientCert, "--key", pki.ClientKey,
			"--proxy-listen", "127.0.0.1:0", "--proxy-ca", pki.CAFile, "--proxy-ca-key", pki.CAKeyFile}, c.upstream...)
		if err := f.fl.Parse(args); err != nil {
			t.Fatal(err)
		}
		if ok, err := f.usesProxy(); !ok || err != nil {
			t.Fatal(ok, err)
		}
		cfg, _, err := f.connection()
		if err != nil {
			t.Fatal(err)
		}
		px, err := f.newProxy(cfg)
		if err != nil {
			t.Fatal(err)
		}
		px.Serve()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cl, err := mqttwire.Dial(ctx, mqttwire.Config{Broker: "mqtts://localhost:" + px.Addr()[len("127.0.0.1:"):], CAFile: pki.CAFile, ConnectTimeout: 3 * time.Second})
		cancel()
		if err == nil {
			cl.Close()
		}
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		px.Wait(ctx, func(cs []proxy.Conn) bool { return len(cs) == 1 && !cs[0].Closed.IsZero() })
		cancel()
		cs := px.Conns()
		px.Close()
		if len(cs) != 1 || cs[0].Relayed() != c.relayed || (err == nil) != c.relayed {
			t.Errorf("%v: relayed %v, want %v: %v %+v", c.upstream, len(cs) == 1 && cs[0].Relayed(), c.relayed, err, cs)
		}
	}
}

// TestLoopback: a proxy reachable from other hosts is reported.
func TestLoopback(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:18883": true, "[::1]:18883": true, "127.1.2.3:1": true,
		"[::]:18883": false, "0.0.0.0:18883": false, "192.168.1.10:18883": false, "nonsense": false,
	} {
		if got := loopback(addr); got != want {
			t.Errorf("%s: %v", addr, got)
		}
	}
}
