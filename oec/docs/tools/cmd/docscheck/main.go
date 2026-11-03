// Command docscheck checks the pages of the OEC developer documentation
// offline (ADR-0031): docscheck [-docs dir] [-oec dir]. The pages live in
// the Community section of the guideline site (C-100 addendum 2); the
// READMEs of oec/ that link them are checked too. It exits 1 on any problem.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/OI4/oec/docs/tools/docscheck"
)

func main() {
	docs := flag.String("docs", "../../../oi4-oec-documentation/docs/community", "the Community folder of the guideline site")
	oec := flag.String("oec", "../..", "the folder oec/ of this repository")
	flag.Parse()
	pages, err := docscheck.Pages(*docs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "docscheck:", err)
		os.Exit(2)
	}
	problems, err := docscheck.Check(*docs, *oec)
	if err != nil {
		fmt.Fprintln(os.Stderr, "docscheck:", err)
		os.Exit(2)
	}
	more, links, err := docscheck.CheckReadmes(*oec, *docs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "docscheck:", err)
		os.Exit(2)
	}
	problems = append(problems, more...)
	for _, p := range problems {
		fmt.Fprintln(os.Stderr, p)
	}
	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "docscheck: %d problem(s) in %d page(s) and the READMEs\n", len(problems), len(pages))
		os.Exit(1)
	}
	fmt.Printf("docscheck: %d pages ok, %d README links to them ok\n", len(pages), links)
}
