#!/bin/sh
# Installs the system packages of the gate on Debian/Ubuntu (ADR-0032 §5),
# on the Linux runners of gate.yml and in the container of linux.sh: the
# build tools, clang with its compiler-rt runtime (ASan/UBSan/LSan of
# sdk/check.sh; without libclang-rt-dev clang cannot link a sanitized
# program), LLVM tools, CMake and GCC 14 (sdk/micro/check.sh). --go also
# installs the Go of the distribution (linux.sh; GOTOOLCHAIN=auto then
# fetches the toolchain of go.mod; the runners use actions/setup-go). Uses
# sudo unless root; without apt-get (macOS) there is nothing to do. Needs
# network.
set -eu
if ! command -v apt-get >/dev/null; then
	echo "packages: no apt-get, nothing to install"
	exit 0
fi
packages=" ca-certificates curl git build-essential clang libclang-rt-dev llvm cmake gcc-14 "
if [ "${1:-}" = "--go" ]; then
	packages="$packages golang-go "
fi
sudo=""
if [ "$(id -u)" != 0 ]; then
	sudo="sudo"
fi
# shellcheck disable=SC2086 # sudo and packages are word lists
$sudo env DEBIAN_FRONTEND=noninteractive apt-get update -qq
# shellcheck disable=SC2086
$sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends $packages >/dev/null
echo "packages:$packages"
