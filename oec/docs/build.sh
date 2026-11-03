#!/bin/sh
# Opt-in build of the documentation D6 (ADR-0031 amendment C-100 addendum 2);
# not part of the gate. The pages live in the Community section of the
# guideline site (../../oi4-oec-documentation/docs/community); this builds
# that site with its own tool chain (yarn 4 from .yarn/releases, Docusaurus
# 3) into its git-ignored build/:
#   1. docscheck – the offline check of the gate
#   2. yarn install --immutable – only if node_modules is missing (network)
#   3. yarn build – the whole guideline site
#   4. fails if a warning of the build (broken link, broken Markdown link)
#      names docs/community, or if a page has no HTML page in build/
# The guideline site keeps its own rules (onBrokenLinks: 'warn'); its
# configuration is not changed here. Nothing is published by this script.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
site="$(cd "$here/../../oi4-oec-documentation" && pwd)"
command -v node >/dev/null || { echo "build.sh: node missing (node >= 18)" >&2; exit 1; }
echo "docs: docscheck"
(cd "$here/tools" && GOTOOLCHAIN="${GOTOOLCHAIN:-auto}" go run ./cmd/docscheck -docs "$site/docs/community" -oec "$here/..")
cd "$site"
if [ ! -x node_modules/.bin/docusaurus ]; then
	echo "docs: yarn install --immutable"
	node .yarn/releases/yarn-*.cjs install --immutable
fi
log="$(mktemp -t oec-docs-build)"
trap 'rm -f "$log"' EXIT
echo "docs: docusaurus build of the guideline site"
node .yarn/releases/yarn-*.cjs build >"$log" 2>&1 || { cat "$log"; exit 1; }
if awk '/\[WARNING\]/{w=1} w && /docs\/community/{bad=1} END{exit !bad}' "$log"; then
	sed -n '/\[WARNING\]/,$p' "$log"
	echo "build.sh: the build warns about docs/community" >&2
	exit 1
fi
grep '\[WARNING\]' "$log" || true
missing=0
for p in $(cd docs/community && find . -name '*.md' -o -name '*.mdx' | sed 's|^\./||' | sort); do
	r="${p%.md*}"
	r="${r%/index}"
	[ "$r" = index ] && r=""
	h="build/docs/community${r:+/$r}.html"
	[ -f "$h" ] || { echo "build.sh: no $h for docs/community/$p" >&2; missing=1; }
done
[ "$missing" = 0 ] || exit 1
echo "docs: ok ($site/build/docs/community)"
