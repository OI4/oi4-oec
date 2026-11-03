#!/bin/sh
# Checks of the documentation D6 of P8 (ADR-0031), offline, no node. Since
# C-100 addendum 2 the pages live in the Community section of the guideline
# site (../../oi4-oec-documentation/docs/community, next to this
# repository, as for the inventory check of step 1):
#   tools (module oec/docs/tools, standard library only): gofmt, go vet,
#   go test – the pages (front matter with title and description, no slug,
#   index.md, a level-1 heading), every Markdown link and anchor between the
#   pages and, as repository URLs, into the files of oec/ (GitHub/Docusaurus
#   anchors), no relative link out of the section, no hyperlink to the
#   guideline (GUARDRAILS G-4.11: sections by number), MDX-safe text, and
#   the links of the READMEs of oec/ to the pages; then docscheck. The build
#   of the guideline site (build.sh, yarn) is opt-in.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
export GOTOOLCHAIN=auto

cd "$here/tools"
test -z "$(gofmt -l .)" || { gofmt -l .; echo "gofmt: files need formatting in $(pwd)" >&2; exit 1; }
go vet ./...
go test ./...
go run ./cmd/docscheck -docs "$here/../../oi4-oec-documentation/docs/community" -oec "$here/.."
