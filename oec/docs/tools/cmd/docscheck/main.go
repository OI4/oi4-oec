// Command docscheck checks the pages of the OEC documentation site offline
// (ADR-0031): docscheck [-docs dir]. It exits 1 on any problem.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/OI4/oec/docs/tools/docscheck"
)

func main() {
	docs := flag.String("docs", "../docs", "the docs folder of the site (oec/docs/docs)")
	flag.Parse()
	pages, err := docscheck.Pages(*docs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "docscheck:", err)
		os.Exit(2)
	}
	problems, err := docscheck.Check(*docs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "docscheck:", err)
		os.Exit(2)
	}
	for _, p := range problems {
		fmt.Fprintln(os.Stderr, p)
	}
	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "docscheck: %d problem(s) in %d page(s)\n", len(problems), len(pages))
		os.Exit(1)
	}
	fmt.Printf("docscheck: %d pages ok\n", len(pages))
}
