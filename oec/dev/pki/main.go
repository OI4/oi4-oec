// Command pki generates the dev PKI, the Mosquitto password file and two
// sample container storages for the OEC dev environment (oec/dev, C-051,
// ADR-0026 amendment C-051). Go standard library only, so it runs wherever
// the gate runs (Linux, macOS, Windows).
//
//	go run ./pki [-out generated] [-force] [-port 8883] [-cert-port 8884] [-san name,...]
//
// Everything below -out is a development secret: never commit it, never use
// it in production (GUARDRAILS G-4.5).
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	o := defaults()
	flag.StringVar(&o.out, "out", o.out, "output directory (replaced with -force)")
	flag.BoolVar(&o.force, "force", false, "replace an existing output directory")
	flag.IntVar(&o.port, "port", o.port, "host port of the credentials listener (SecurePort of oec-dev-app)")
	flag.IntVar(&o.certPort, "cert-port", o.certPort, "host port of the client certificate listener (SecurePort of oec-dev-cert)")
	flag.StringVar(&o.address, "address", o.address, "Address in broker.json")
	sans := flag.String("san", "", "additional DNS names or IP addresses of the broker certificate, comma separated")
	flag.Parse()
	if *sans != "" {
		o.sans = strings.Split(*sans, ",")
	}
	if err := generate(o); err != nil {
		fmt.Fprintln(os.Stderr, "pki:", err)
		os.Exit(1)
	}
	fmt.Printf("dev PKI, password file and sample container storages written to %s\n", o.out)
}
