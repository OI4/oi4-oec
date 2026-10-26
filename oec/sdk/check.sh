#!/bin/sh
# Checks of the P3 SDK sketches (spikes, not product code; ADR-0020 … 0022):
#   1. rust/sketch: the public Rust API sketch of D2 compiles with all DX
#      examples; tests: hello world ≤ 20 lines, every REQ id cited under
#      oec/sdk/ exists and is not deprecated;
#   2. the C headers oec.h (D2 C ABI) and oecm.h (D9) and their examples
#      compile as C17 without warnings (syntax only – no implementation);
#   3. spikes (C-043, ADR-0023): rust/spike/schema-embed embeds oec/schema/v2
#      at compile time and reproduces every strict verdict of
#      oec/schema/dist/cases.json (needs schema/check.sh first, as in
#      oec/check.sh); rust/spike/mqtt-codec is the sans-IO MQTT 3.1.1 codec
#      of option (a) of Q-026, byte-exact against hand-encoded packets.
set -eu
root="$(cd "$(dirname "$0")" && pwd)"
cc="${CC:-cc}"
cflags="-std=c17 -Wall -Wextra -Wpedantic -Werror -fsyntax-only"

echo "rust/sketch: cargo test"
cd "$root/rust/sketch"
cargo test --offline --locked --quiet

echo "rust/sketch/c: oec.h + examples (C17)"
for f in "$root"/rust/sketch/c/examples/*.c; do
	# shellcheck disable=SC2086
	"$cc" $cflags -I "$root/rust/sketch/c/include" "$f"
done

echo "micro/sketch: oecm.h + examples (C17)"
for f in "$root"/micro/sketch/examples/*.c; do
	# shellcheck disable=SC2086
	"$cc" $cflags -I "$root/micro/sketch/include" "$f"
done

for spike in schema-embed mqtt-codec; do
	echo "rust/spike/$spike: cargo test"
	cd "$root/rust/spike/$spike"
	cargo test --offline --locked --quiet
done

echo "sdk sketches ok"
