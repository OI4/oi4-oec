#!/bin/sh
# Builds liboec_ffi.a and the Go SDK examples for linux amd64 and arm64
# inside a Debian container (run by run.sh, P4c exit criterion 4, C-078).
# /oec is the read-only monorepo folder oec/, /usr/local/go the Go
# toolchain, /out the output. Cross C toolchains of Debian; no emulation.
set -eu
export PATH=/usr/local/go/bin:$PATH GOTOOLCHAIN=local GOFLAGS=-buildvcs=false \
	GOCACHE=/tmp/gocache GOPATH=/tmp/gopath CARGO_TARGET_DIR=/tmp/target \
	CARGO_TARGET_X86_64_UNKNOWN_LINUX_GNU_LINKER=x86_64-linux-gnu-gcc \
	CARGO_TARGET_AARCH64_UNKNOWN_LINUX_GNU_LINKER=aarch64-linux-gnu-gcc \
	CC_x86_64_unknown_linux_gnu=x86_64-linux-gnu-gcc \
	CC_aarch64_unknown_linux_gnu=aarch64-linux-gnu-gcc

apt-get update -qq
apt-get install -y -qq --no-install-recommends gcc-x86-64-linux-gnu libc6-dev-amd64-cross \
	gcc-aarch64-linux-gnu libc6-dev-arm64-cross file >/dev/null
rustup target add x86_64-unknown-linux-gnu aarch64-unknown-linux-gnu

mkdir -p /work
tar -C /oec --exclude=./sdk/rust/target --exclude=./sdk/rust/spike --exclude=./sdk/go/lib \
	--exclude=./schema/tools -cf - ./schema ./sdk/rust ./sdk/go | tar -C /work -xf -
cd /work/sdk/go
rustc --version
go version
for t in x86_64:amd64 aarch64:arm64; do
	cpu="${t%%:*}"
	arch="${t##*:}"
	go run ./cmd/oec-buildlib -target "$cpu-unknown-linux-gnu"
	CGO_ENABLED=1 GOOS=linux GOARCH="$arch" CC="$cpu-linux-gnu-gcc" \
		go build -o "/out/linux_$arch/" ./examples/...
	CGO_ENABLED=1 GOOS=linux GOARCH="$arch" CC="$cpu-linux-gnu-gcc" go vet ./...
done
file /out/linux_*/*
