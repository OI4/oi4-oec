# ADR-0032 – CI and release architecture (P9)

- Status: **accepted (agent, G-3.4 b)** for the CI platform, the generated workflows, the CI policy, the tool pins, the runner matrix, the release build design and the cycle cut of P9; activation of the CI (push, secret, runner minutes), release channels and targets, visibility, the Go module path and the CRA review are **decisions of the PO** (Q-011, G-3.4 a; C-104 D-1 … D-3).
- Date: 2026-11-20
- Deciders: agent (CI design, tools, versions, cycle cut); product owner (Thomas) – activation, publication, release channels, CRA
- Related: ADR-0009/0012 (local git repository, monorepo `oec/`, never push), ADR-0019 amendment C-041 (one local gate `oec/check.sh`), ADR-0023 (dependency policy), ADR-0026 (Rust SDK, toolchain 1.89), ADR-0027 (Go SDK), ADR-0030 (micro SDK), ADR-0031 (documentation), GUARDRAILS G-3.8 (publication), G-4.4 (dependencies), G-4.13 (cross-platform tooling), Q-011 (publication, CRA), ROADMAP P9

## Context

P9 (hardening and release) collects what P4 … P8 carried forward: the SDK platform matrix (Linux/macOS/Windows × amd64/arm64 incl. MSVC, ARMv7 best effort), D7 and D8 on Windows/macOS/Linux runners with their binaries for six targets and the multi-arch images, the micro SDK host tests on Linux/Windows and its ESP-IDF and Arduino builds, newcomer runs of Getting started on Linux and Windows, SBOM, license report, security scan, versioned releases and the guideline feedback package. Up to C-103 everything ran on one macOS arm64 machine through `oec/check.sh` (10 steps, 15 … 80 minutes); Linux and Windows were covered only by cross builds and the opt-in container runs of `oec/dev`.

The workspace is a local git repository (ADR-0009). Its GitHub remote `OI4/oi4-oec` and the guideline repository `OI4/oi4-oec-documentation` are **private** (checked read-only in C-104); the remote `main` is eight commits behind, the guideline branch `feature/spec-findings` three commits (the Community pages of C-101 … C-103). Nothing is pushed by the agent; whether, where and when anything is published is the PO's (G-3.8, Q-011).

What the gate needs (C-104 survey): Go (`GOTOOLCHAIN=auto`; the modules declare `go 1.26.0` with toolchain `go1.27.2`/`go1.27.3`; three modules have third-party Go dependencies from the module proxy), Rust 1.89 (`rust-version` of three Cargo packages) with clippy and rustfmt, cbindgen exactly 0.29.4 (`sdk/check.sh`), cargo-deny (advisory database over the network, falls back to `--frozen`), six `Cargo.lock` files built `--offline --locked`, a C17 compiler with ASan/UBSan (LSan on Linux), `clang --analyze` (`sdk/micro/check.sh`), cmake, make, nm, optionally GCC 14/15; the guideline repository next to the workspace with its history (`spectool inventory -check -rev 52d8470`) and its working tree (docscheck, step 10). The registry Web UI bundle is committed; no npm in the gate.

## Decision

### 1. Platform (A-536)

GitHub Actions in the workspace repository (`OI4/oi4-oec`), workflows in `.github/workflows/` at its root, **GitHub-hosted runners only** (no self-hosted runner: no machine of ours runs code of a pull request). The repository already lives on GitHub; Actions needs no further service. The workflows are inert files until the PO pushes (D-1).

### 2. One entry point (A-537)

A workflow only checks out, installs tools and calls the scripts of the local gate: the job `gate` runs `oec/check.sh` itself, the other jobs call scripts under `oec/ci/`. No build or test logic lives in YAML; what fails in CI can be reproduced locally with the same command, and the local gate stays the reference (ADR-0019 amendment C-041).

### 3. Generated workflows (A-538)

The workflows are **rendered** by the Go module `oec/ci` (`github.com/OI4/oec/ci`, standard library only) from a typed model (`gha`: workflow, job, step, action, matrix) and the definitions in `pipelines` – no YAML library (G-4.4): the renderer writes plain scalars where they read back as the same string in YAML 1.1 and 1.2, JSON strings (valid double-quoted YAML) otherwise, literal blocks for multi-line values, raw booleans and integers. `cmd/cigen` writes `.github/workflows/`; `cigen -check` fails if a file differs from its rendering or a file there is not generated. Gate **step 11** runs gofmt, vet, the tests of `oec/ci` and `cigen -check`.

### 4. CI policy (A-539)

`gha.Check` is part of the rendering and of the tests; a workflow that breaks it is not generated:

- actions only from `actions/*`, pinned to the full commit SHA of a release, the tag as a comment, from one pin list (checkout v7.0.1, setup-go v7.0.0, cache v6.1.0, upload-artifact v7.0.2 in C-104; SHAs verified against the tags);
- `permissions: contents: read` at the workflow level and nothing else; a concurrency group per workflow and ref with `cancel-in-progress`;
- triggers push (`main`), `pull_request`, `workflow_dispatch` – the model cannot express `pull_request_target`, `workflow_run` or self-hosted runners;
- every job has `timeout-minutes` (≤ 240) and runs on a listed hosted runner; with a matrix `runs-on: ${{ matrix.os }}`, `fail-fast: false`;
- no expression in a `run` (values go through `env`); no expression may read `github.event`, `github.head_ref`, `inputs`, `steps` or `needs` (script injection);
- one secret, `OEC_GUIDELINE_TOKEN`, only as the `token` of the guideline checkout; every checkout with `persist-credentials: false`;
- every run step in `bash` (Git Bash on Windows).

### 5. Tool pins (A-540)

`oec/ci/versions.sh` holds the pins: Rust 1.89.0, cbindgen 0.29.4, cargo-deny 0.20.2, actionlint 1.7.12, the guideline branch and the Ubuntu image (by digest) of `linux.sh`. The tests of `pipelines` compare them with `sdk/check.sh` (cbindgen), with the `rust-version` of every Cargo package and with the guideline ref of the workflows. `setup-tools.sh` installs the toolchain with rustup (rustup itself if missing), cbindgen and cargo-deny with `cargo install --locked` into `~/.oec-tools`; `fetch.sh` runs `cargo fetch --locked` for every `Cargo.lock` under `oec/`, so the gate keeps `--offline --locked`. Go comes from `actions/setup-go` with `go-version-file` and `GOTOOLCHAIN=auto`. The workflow caches `~/.oec-tools` and the cargo registry, keyed on `versions.sh` and the lock files.

### 6. Workflows and runners (A-541)

- **`gate.yml`** – job `gate`: the whole `oec/check.sh` on `ubuntu-24.04`, `ubuntu-24.04-arm` (`CC=clang`: `clang --analyze`, the same compiler for cgo, CMake and the sanitizers as on macOS) and `macos-15`, 180 minutes; job `actionlint` (`lint.sh`) on `ubuntu-24.04`.
- **`conformance.yml`** – job `test`: gofmt, vet and tests of `oec/conformance` (D7/D8, gate step 4) on `ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-15`, `macos-15-intel`, `windows-2025`, `windows-11-arm`; job `binaries`: `oec-check` and `oec-infra` for windows/linux/darwin × amd64/arm64 with `SHA256SUMS`, **reproducible** (`CGO_ENABLED=0`, `-trimpath`, empty build id; two builds give the same checksums), uploaded as a workflow artifact for 14 days – not a release.
- `.gitattributes` `* text=auto eol=lf` (A-542): LF in every checkout, also on Windows runners (gofmt, the hash pins of the schema release and of the vectors, G-4.13). The index was LF already; renormalising changes no file.

Later increments add `sdk.yml`, `micro.yml`, `images.yml` and `release.yml` the same way (§9).

### 7. The guideline in CI (A-543)

The gate job checks out `OI4/oi4-oec-documentation` at `feature/spec-findings` with full history into `oi4-oec-documentation/` next to `oec/`, as in the workspace. While that repository is private the checkout needs the secret `OEC_GUIDELINE_TOKEN` (fine-grained, read-only, contents of that one repository); without it the checkout falls back to `github.token`, which only reads a public repository. The pages of C-101 … C-103 must be pushed to that branch first, or docscheck (step 10) fails in CI (D-1).

### 8. Local verification without publication (A-544)

- `lint.sh` (opt-in locally, a job of `gate.yml`): actionlint at the pinned version with `go run …@v1.7.12` – a tool run like Docker or yarn, fetched into the Go module cache, not a dependency of any module (MIT).
- `linux.sh` (opt-in, Docker, network): the job `gate` in the pinned Ubuntu 24.04 container – clones the committed workspace and guideline branch, installs the build tools (apt), Go and rustup, runs `setup-tools.sh`, `fetch.sh` and `oec/check.sh` as a non-root user with `CC=clang`; removes its container and the image it pulled. On Apple silicon it is the counterpart of `ubuntu-24.04-arm`.
- The Windows side is verified locally by `GOOS=windows go vet` and `go test -c` of `oec/conformance` for amd64 and arm64, the macOS Intel side by the conformance tests as darwin/amd64 under Rosetta; the real runs need the activation (D-1).

### 9. Releases (design, built in increment 6; the channels are the PO's, D-2)

Versioning and release are a workflow `release.yml` started by a tag or by hand that builds every deliverable from one commit, reproducibly where the toolchain allows, with `SHA256SUMS`, SBOMs (SPDX for the Rust and Go artifacts, the image SBOMs of `images/build.sh`) and release notes from the logs – as **workflow artifacts**. Where they go (GitHub Releases/Packages, crates.io, the Go module proxy, the ESP Component Registry, an image registry), with which visibility and from which repository is decided by the PO (D-2, D-3, Q-011). The CRA review (SBOM, vulnerability handling, security update policy, support period) stays a precondition of any public release (Q-011).

### 10. Cycle cut of P9 (A-545)

| Increment | Cycle | Content |
|---|---|---|
| 1 | C-104 | plan, this ADR, `oec/ci` with `gate.yml` and `conformance.yml`, gate step 11, `.gitattributes`, local verification (actionlint, Linux container run, Windows cross checks) |
| 2 | C-105 | SDK platform matrix `sdk.yml`: `oec`/`oec-ffi` built and tested on Linux/macOS/Windows × amd64/arm64 incl. Windows MSVC, ARMv7 Linux best effort (cross); Go SDK incl. Windows (MinGW); prebuilt `liboec_ffi` archives as artifacts |
| 3 | C-106 | micro SDK `micro.yml`: host tests on Linux/Windows; ESP-IDF builds `esp32`, `esp32s3`, `qemu` in the pinned Espressif container; Arduino builds (Arduino-ESP32 3.3.x and 4.0 final; C3/C6 best effort) |
| 4 | C-107 | images and newcomer runs: multi-arch images of the demos, the registry and D7/D8 with D8 `image`/`run`; Getting started on Linux (compose) and the Windows limits; the opt-in `live-infra.sh` on a Linux runner; Podman on Linux |
| 5 | C-108 | supply chain: SBOM and license report of every deliverable, `govulncheck`, `cargo deny` advisories, npm audit of the Web UI, `SECURITY.md` and a draft of the vulnerability handling (CRA input, Q-011) |
| 6 | C-109 | versioning and `release.yml` (§9): artifacts, checksums, SBOMs, release notes; nothing published |
| 7 | C-110 | guideline feedback package: all spec findings SF-001 … with their proposed text and the guideline diff, for the work group |
| 8 | C-111 | P9 exit run (needs the activation, D-1) |

Increments 2 … 6 can only be verified locally (cross builds, containers) until the CI is active; the order may change with D-1.

## Consequences

- The local gate stays the single definition of "green"; CI repeats it on more platforms. Gate step 11 keeps the workflow files and their policy honest without network.
- A new workflow or job is a change in `oec/ci/pipelines` plus `cigen`; editing a file under `.github/workflows` by hand fails the gate.
- Action and tool updates are explicit pin changes (README "Updating a pin"); there is no automatic update (Dependabot is a question of increment 5).
- Nothing runs before the PO pushes; then each push to `main` and each pull request costs runner minutes on three gate runners and six conformance runners (macOS and Windows minutes count several times in private repositories) – part of D-1.
- The Go module paths `github.com/OI4/oec/...` do not match the repository `OI4/oi4-oec` (`oec/` inside it); this does not matter for CI (local `replace` and module roots) but does for `go get` by others (D-3).

## Alternatives considered

- **Hand-written YAML with a linter only:** simpler, but the policy (pins, permissions, expressions, secrets) would rest on review; the generator makes it a test and keeps eight future workflows consistent.
- **A YAML library for rendering:** not allowed for a new Go module (G-4.4, standard library only), and not needed for this fixed subset.
- **Reusable workflows or composite actions of our own:** more indirection for six to eight workflows; the generator already shares the steps.
- **Third-party actions** (Rust toolchain, caching, release upload): avoided; rustup is preinstalled on the hosted images, `cargo install --locked` is reproducible, the four first-party actions suffice.
- **Self-hosted runners** (e.g. the PO's Mac): would run code of pull requests on own hardware; excluded.
- **Another CI service** (GitLab CI, Azure Pipelines): the repositories are on GitHub; a second service would need a mirror.
