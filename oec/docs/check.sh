#!/bin/sh
# Checks of the documentation site D6 of P8 (ADR-0031), offline, no node:
#   tools (module oec/docs/tools, standard library only): gofmt, go vet,
#   go test – the pages of docs/ (front matter with title and description,
#   sidebar_position, exactly one root page slug /, a level-1 heading),
#   every Markdown link and anchor into the site and into the files of oec/
#   (GitHub/Docusaurus anchors), no link out of oec/ and no hyperlink to the
#   guideline site (GUARDRAILS G-4.11); the npm policy of the site
#   (tools/npmpolicy, C-099: package.json, package-lock.json, licenses,
#   .npmrc and the build rules of docusaurus.config.mjs); then docscheck
#   over docs/. The site build (build.sh, npm) is opt-in.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
export GOTOOLCHAIN=auto

cd "$here/tools"
test -z "$(gofmt -l .)" || { gofmt -l .; echo "gofmt: files need formatting in $(pwd)" >&2; exit 1; }
go vet ./...
go test ./...
go run ./cmd/docscheck -docs "$here/docs"
