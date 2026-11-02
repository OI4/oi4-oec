#!/bin/sh
# Fuzzes the micro SDK core with libFuzzer (ADR-0030 §6, C-094; opt-in, not
# in the gate): the JSON reader, the topic parser and a received message at
# a connected Level 2 device. Each target runs SECONDS (default 60) on a
# copy of its seed corpus in target/fuzz/<target>; findings are written
# there as crash-*. Afterwards the grown corpus is replayed under ASan and
# UBSan with fuzz/replay.c, built by the system compiler (cc).
# Needs a clang with libFuzzer (Apple clang has none:
# CC=/opt/homebrew/opt/llvm/bin/clang is tried first). The ASan runtime of
# Homebrew LLVM 21 hangs at start-up on macOS 27 (even for an empty target),
# so on macOS the fuzzing itself runs with UBSan only and ASan comes from the
# replay; OECM_FUZZ_SAN overrides the sanitizers of the fuzzing run.
#
#   fuzz/run.sh [SECONDS] [TARGET...]
set -eu
root="$(cd "$(dirname "$0")/.." && pwd)"
seconds="${1:-60}"
[ $# -gt 0 ] && shift
targets="${*:-json topic message}"
cc="${CC:-}"
if [ -z "$cc" ]; then
	for c in /opt/homebrew/opt/llvm/bin/clang /usr/local/opt/llvm/bin/clang clang; do
		command -v "$c" >/dev/null 2>&1 && { cc="$c"; break; }
	done
fi
if [ -z "${OECM_FUZZ_SAN:-}" ]; then
	case "$(uname -s)" in
	Darwin) OECM_FUZZ_SAN=fuzzer,undefined ;;
	*) OECM_FUZZ_SAN=fuzzer,address,undefined ;;
	esac
fi
out="$root/target/fuzz"
mkdir -p "$out"
for t in $targets; do
	echo "fuzz: $t, ${seconds}s ($cc, $OECM_FUZZ_SAN)"
	"$cc" -std=c17 -g -O1 -fsanitize="$OECM_FUZZ_SAN" -fno-sanitize-recover=all \
		-I"$root/include" -I"$root/src" -o "$out/fuzz_$t" "$root/fuzz/fuzz_$t.c" "$root"/src/*.c
	mkdir -p "$out/$t"
	cp "$root/fuzz/corpus/$t/"* "$out/$t/"
	(cd "$out" && "./fuzz_$t" -max_total_time="$seconds" -max_len=4096 -artifact_prefix="$out/$t-" \
		"$out/$t")
	cc -std=c17 -g -O1 -fsanitize=address,undefined -fno-sanitize-recover=all \
		-I"$root/include" -I"$root/src" -o "$out/replay_$t" "$root/fuzz/fuzz_$t.c" \
		"$root/fuzz/replay.c" "$root"/src/*.c
	"$out/replay_$t" "$out/$t"
done
