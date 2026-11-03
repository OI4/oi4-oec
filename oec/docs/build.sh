#!/bin/sh
# Opt-in build of the documentation site D6 (ADR-0031 §6); not part of the
# gate. Needs node >= 22.12 with npm and, once, the npm registry (or a
# filled npm cache):
#   1. go test ./npmpolicy (tools/) – the npm policy of package.json and
#      package-lock.json, as in the gate, before anything is installed
#   2. npm ci --ignore-scripts – exactly package-lock.json, no install scripts
#   3. docusaurus build – the site into build/; every broken link or anchor
#      fails (docusaurus.config.mjs)
# Links to the files of oec/ point at OEC_DOCS_REPO_URL (default
# https://github.com/OI4/oi4-oec/blob/main). build/, .docusaurus/ and
# node_modules/ are never committed (.gitignore). The site is not published
# (Q-011). Serve the result locally with `npm run serve`.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
command -v npm >/dev/null || { echo "build.sh: npm missing (node >= 22.12)" >&2; exit 1; }
echo "docs: npm policy"
(cd "$here/tools" && GOTOOLCHAIN="${GOTOOLCHAIN:-auto}" go test -count=1 ./npmpolicy)
cd "$here"
echo "docs: npm ci --ignore-scripts"
npm ci --ignore-scripts --no-audit --no-fund
echo "docs: docusaurus build"
npm run build
echo "docs: ok ($here/build)"
