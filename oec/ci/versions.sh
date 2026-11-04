# shellcheck shell=sh disable=SC2034
# The pins of the CI (ADR-0032 §5), sourced by setup-tools.sh, lint.sh,
# linux.sh, micro-esp-idf.sh and micro-arduino.sh. The tests of oec/ci/pipelines check them against sdk/check.sh
# (cbindgen), the rust-version of every Cargo package and the guideline ref
# of the workflows; the workflows key their tool cache on this file.
rust_version=1.89.0
cbindgen_version=0.29.4
cargo_deny_version=0.20.2
actionlint_version=1.7.12
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
