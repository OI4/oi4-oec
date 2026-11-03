#!/bin/sh
# Fetches the crates of every Cargo.lock under oec/ (cargo fetch --locked,
# every target platform), so that the gate builds with --offline --locked
# (ADR-0032 §5). Needs network.
set -eu
here="$(cd "$(dirname "$0")" && pwd)"
oec="$(cd "$here/.." && pwd)"
locks="$(find "$oec" -name Cargo.lock -not -path '*/target/*' -not -path '*/node_modules/*' | sort)"
test -n "$locks" || { echo "fetch: no Cargo.lock under $oec" >&2; exit 1; }
printf '%s\n' "$locks" | while IFS= read -r lock; do
	dir="$(dirname "$lock")"
	echo "fetch: ${dir#"$oec"/}"
	cargo fetch --locked --quiet --manifest-path "$dir/Cargo.toml"
done
