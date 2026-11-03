# shellcheck shell=sh disable=SC2034
# The pins of the CI (ADR-0032 §5), sourced by setup-tools.sh, lint.sh and
# linux.sh. The tests of oec/ci/pipelines check them against sdk/check.sh
# (cbindgen), the rust-version of every Cargo package and the guideline ref
# of the workflows; the workflows key their tool cache on this file.
rust_version=1.89.0
cbindgen_version=0.29.4
cargo_deny_version=0.20.2
actionlint_version=1.7.12
guideline_ref=feature/spec-findings
ubuntu_image=ubuntu:24.04@sha256:fe81560990fae86dc4743202b2cb4d3df98d0bcf949173cecef15ce1127ef852
