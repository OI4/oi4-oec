# shellcheck shell=sh disable=SC2034
# The pins of the CI (ADR-0032 §5), sourced by setup-tools.sh, lint.sh,
# linux.sh, micro-esp-idf.sh, micro-arduino.sh, sbom.sh and audit.sh. The
# tests of oec/ci/pipelines check them against sdk/check.sh (cbindgen),
# the rust-version of every Cargo package and the guideline ref of the
# workflows; the workflows key their tool cache on this file.
rust_version=1.89.0
# rustup of setup-tools.sh where the runner has none (C-113 RT-3):
# rustup-init of this rustup release with the SHA-256 of
# static.rust-lang.org/rustup/archive/<version>/<target>/rustup-init[.exe].sha256
# for the platform of every hosted runner (pipelines.Runners).
rustup_version=1.29.3
rustup_init_sha256_linux_amd64=b1c8cb14e30889e7e2999e6b69e62d9e9b9ffc0f4df8180564d306d9680e6c55
rustup_init_sha256_linux_arm64=9f608c142e0e38a78957d0661ba1dfaf0b6fb37184cfaa4b9a0aa57af6184282
rustup_init_sha256_darwin_arm64=63752a1919a3a4793173ec70f8f212621a7ce38ba6a246095984482f4de20f9f
rustup_init_sha256_darwin_amd64=bc107f1fb0228de7fa32413c89c1b305cea9cd256e94b01150cd054128145429
rustup_init_sha256_windows_amd64=38027d51d69a5d05493627a0634dbea52d6cd793c8f072d2373d3f4163eec7db
rustup_init_sha256_windows_arm64=407e0d72f81d26a01f9d032b8317908cbb2917ba22765474b682b41390792ad9
cbindgen_version=0.29.4
cargo_deny_version=0.20.2
actionlint_version=1.7.12
# govulncheck of audit.sh (C-108): a tool run with go run, not a dependency
govulncheck_version=1.9.0
guideline_ref=feature/spec-findings
ubuntu_image=ubuntu:24.04@sha256:fe81560990fae86dc4743202b2cb4d3df98d0bcf949173cecef15ce1127ef852
# The micro SDK workflow micro.yml (C-106): the Espressif ESP-IDF image of
# ADR-0030 §8 (the index digest, linux/amd64 and linux/arm64; Ubuntu 24.04
# with the Espressif QEMU), arduino-cli with the SHA-256 of its release
# archives, the Arduino-ESP32 cores (4.0: the newest release candidate until
# 4.0.0 is released) and the libraries of the M5Stack example.
espressif_idf_image=espressif/idf:v6.1@sha256:81893c71bb5e570088901f21def8684c25cd2a9020281bd01b843a7655edb18c
arduino_cli_version=1.5.1
arduino_cli_sha256_linux_amd64=28a8e119c498a25607821c36cb2dc49e8463941b261a0d99091baa7bc692dd2b
arduino_cli_sha256_linux_arm64=1e69e077479f300614d4551334e0a33f08ee40b04315d83b8e7e0e94f0d0ee62
arduino_cli_sha256_darwin_arm64=cb952e8c1621c95ef5f1d17831c945e3d0ec5973f89c557a7ec8feb9c4f7d4c9
arduino_esp32_3_3=3.3.12
arduino_esp32_4_0=4.0.0-RC1
m5unified_version=0.2.29
m5gfx_version=0.2.36
