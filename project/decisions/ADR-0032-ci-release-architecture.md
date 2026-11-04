# ADR-0032 – CI and release architecture (P9)

- Status: **accepted (agent, G-3.4 b)** for the CI platform, the generated workflows, the CI policy, the tool pins, the runner matrix, the release build design and the cycle cut of P9; activation of the CI (push, secret, runner minutes), release channels and targets, visibility, the Go module path and the CRA review are **decisions of the PO** (Q-011, G-3.4 a; C-104 D-1 … D-3). C-104 addendum 1 (PO): D-1 (a) activate the CI now (secret first, then the pushes); D-2 channels and visibility decided before C-109, artifacts only until then; D-3 (a) a repository `OI4/oec` for the published deliverables at the first release (module paths unchanged).
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

`oec/ci/versions.sh` holds the pins: Rust 1.89.0, cbindgen 0.29.4, cargo-deny 0.20.2, actionlint 1.7.12, the guideline branch and the Ubuntu image (by digest) of `linux.sh`. The tests of `pipelines` compare them with `sdk/check.sh` (cbindgen), with the `rust-version` of every Cargo package and with the guideline ref of the workflows. `packages.sh` installs the system packages on the Linux runners and in `linux.sh` (apt: build tools, clang with its compiler-rt runtime `libclang-rt-dev` for the sanitizers, LLVM, CMake, GCC 14). `setup-tools.sh` installs the toolchain with rustup (rustup itself if missing), cbindgen and cargo-deny with `cargo install --locked` into `~/.oec-tools`; `fetch.sh` runs `cargo fetch --locked` for every `Cargo.lock` under `oec/`, so the gate keeps `--offline --locked`. Go comes from `actions/setup-go` with `go-version-file` and `GOTOOLCHAIN=auto`. The workflow caches `~/.oec-tools` and the cargo registry, keyed on `versions.sh` and the lock files.

### 6. Workflows and runners (A-541)

- **`gate.yml`** – job `gate`: the whole `oec/check.sh` on `ubuntu-24.04`, `ubuntu-24.04-arm` (`CC=clang`: `clang --analyze`, the same compiler for cgo, CMake and the sanitizers as on macOS) and `macos-15`, 180 minutes; job `actionlint` (`lint.sh`) on `ubuntu-24.04`.
- **`conformance.yml`** – job `test`: gofmt, vet and tests of `oec/conformance` (D7/D8, gate step 4) on `ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-15`, `macos-15-intel`, `windows-2025`, `windows-11-arm`; job `binaries`: `oec-check` and `oec-infra` for windows/linux/darwin × amd64/arm64 with `SHA256SUMS`, **reproducible** (`CGO_ENABLED=0`, `-trimpath`, empty build id; two builds give the same checksums), uploaded as a workflow artifact for 14 days – not a release.
- `.gitattributes` `* text=auto eol=lf` (A-542): LF in every checkout, also on Windows runners (gofmt, the hash pins of the schema release and of the vectors, G-4.13). The index was LF already; renormalising changes no file.

Later increments add `sdk.yml`, `micro.yml`, `images.yml` and `release.yml` the same way (§9).

### 7. The guideline in CI (A-543)

The gate job checks out `OI4/oi4-oec-documentation` at `feature/spec-findings` with full history into `oi4-oec-documentation/` next to `oec/`, as in the workspace. While that repository is private the checkout needs the secret `OEC_GUIDELINE_TOKEN` (fine-grained, read-only, contents of that one repository); without it the checkout falls back to `github.token`, which only reads a public repository. The pages of C-101 … C-103 must be pushed to that branch first, or docscheck (step 10) fails in CI (D-1).

### 8. Local verification without publication (A-544)

- `lint.sh` (opt-in locally, a job of `gate.yml`): actionlint at the pinned version with `go run …@v1.7.12` – a tool run like Docker or yarn, fetched into the Go module cache, not a dependency of any module (MIT).
- `linux.sh` (opt-in, Docker, network): the job `gate` in the pinned Ubuntu 24.04 container – clones the committed workspace and guideline branch, installs the build tools (apt), Go and rustup, runs `setup-tools.sh`, `fetch.sh` and `oec/check.sh` as a non-root user with `CC=clang`; removes its container and the image it pulled. On Apple silicon it is the counterpart of `ubuntu-24.04-arm`. Options found necessary in the C-104 runs (A-546): `--init` (an init process reaps the application the D7 restart hook starts in the background, as systemd does on the runner; without it the stopped application stays a zombie and the stop hook waits forever), `--cpus ${OEC_CI_CPUS:-4}` (the vCPUs of a hosted runner; with all host cores the parallel links of the Rust tests exhausted the memory of the Docker VM), and `OEC_CI_CA_CERTS` (a PEM mounted read-only and added to the trust store, for networks with TLS interception; never committed).
- What the Linux run found beyond the environment is fixed test first (A-547 … A-550): a test setup of D8 `host` that depended on the machine (`/.dockerenv`), the sanitizer runtime missing in a minimal install (`packages.sh` installs `libclang-rt-dev`), undefined behaviour in the micro SDK that only the UBSan of clang 18 reports (ADR-0030 amendment C-104), and D7 test runs of the Go SDK without a time limit. With these fixes the ninth run passed all eleven gate steps (C-104 log).
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

## Amendment C-105 (2026-11-21) – increment 2 as built: `sdk.yml` (agent, G-3.4 b)

Details: `log/2026-11-21_C-105_sdk-matrix.md`, REVIEW A-560 … A-570. Logic in scripts under `oec/ci/`, the workflow generated as before (gate step 11: `cigen: 3 workflows current`).

1. **Jobs (A-560):** `rust` on all six runners, each a **native** build of its host target (`OEC_CI_TARGET` is checked against the host of `rustc`): `x86_64`/`aarch64-unknown-linux-gnu`, `aarch64`/`x86_64-apple-darwin`, `x86_64`/`aarch64-pc-windows-msvc`; `sdk-rust.sh` runs clippy (`-D warnings`) and the tests of `oec/sdk/rust`, `ffi-lib.sh` and a C smoke test. `go` on five runners (not `windows-11-arm`): `sdk-go.sh` as the Go part of `sdk/check.sh` (`go test -race` with the go-testbroker and `oec-check`). `armv7` on `ubuntu-24.04`: `sdk-armv7.sh` (cross). Timeouts 120/90/90 minutes; cache prefix `oec-sdk-` (crates only, apart from the gate's tool cache).
2. **Prebuilt libraries (A-561):** `ffi-lib.sh` packs one target as `liboec_ffi-<target>/` (`include/oec.h`, the static library, `native-static-libs.txt`, `BUILDINFO` with target, crate version, commit and `rustc`, `LICENSE`, `SHA256SUMS`), uploaded as a directory artifact for 14 days – eight targets (six native, `x86_64-pc-windows-gnu` from the `go` job, ARMv7). Static library only (the C and Go users link statically; the shared library would add an import library and a run-time search path per OS). Default features (aws-lc-rs). Not a release (Q-011, D-2); the third-party license report of the linked crates is increment 5 (C-108).
3. **Windows (A-562):** MSVC targets with the DLL CRT. `tests/c.rs` of `oec-ffi` is Unix only; on Windows `sdk-rust.sh` compiles `examples/hello.c` with clang (`-fms-runtime-lib=dll`, the `.lib` items of `native-static-libs`) and runs it without a container storage: exit 1 with `build: bootstrap:` on stderr proves that the library links and runs (same check on every runner). x64 uses the prebuilt NASM objects of aws-lc-sys (`AWS_LC_SYS_PREBUILT_NASM=1`, no NASM install); arm64 compiles aws-lc with the `clang-cl` of Visual Studio, which aws-lc-sys finds itself – therefore the workflow sets no `CC`; the scripts take `OEC_CI_CC` (Unix: exported as `CC`; Windows: only the compiler of the smoke test).
4. **Go on Windows (A-563):** cgo needs a MinGW gcc; the Rust target is `x86_64-pc-windows-gnu` (ADR-0027 amendment C-105). `windows-11-arm` has no arm64 MinGW toolchain: not in the `go` job.
5. **ARMv7 (A-564):** best effort as a cross build on amd64 with the Ubuntu cross gcc and `qemu-user` (`packages.sh --armv7`): clippy and the tests of `oec-core`, `oec` (aws-lc-rs and `crypto-ring`) and the Rust tests of `oec-ffi` under `qemu-arm`; the C tests of `oec-ffi` (sanitizers, a native `cc`) stay with the native runners, a C smoke test runs under `qemu-arm`; the Go SDK for `linux/arm` (`GOARM=7`): vet, the examples, `hello` under `qemu-arm`. No hosted ARMv7 runner exists.
6. **Scripts (A-565):** `setup-tools.sh --toolchain` (the Rust toolchain only; Windows paths for `GITHUB_PATH` via `cygpath`), `packages.sh --armv7`; `sdk-rust.sh` and `sdk-go.sh` also run locally (macOS arm64 checked).
7. **Local verification (§8):** actionlint clean; on macOS arm64 `sdk-rust.sh` (the library is byte-identical in two builds) and `sdk-go.sh`; in an Ubuntu 24.04 arm64 container `sdk-armv7.sh` and the Windows MinGW side (`ffi-lib.sh` for `x86_64-pc-windows-gnu`, `hello.c` and the Go examples and test binary linked with MinGW gcc: no `libwinpthread-1.dll`); `linux.sh` (gate in Ubuntu). MSVC, `windows-11-arm`, `macos-15-intel` and the Go tests on Windows are verified only on the hosted runners (C-105 log).
8. **`linux.sh` (A-570):** the job exports `CARGO_INCREMENTAL=0` like `gate.yml` (A-558); without it the targets of the gate filled the disk of the Docker VM.
9. **Hosted runs (C-105 addendum 2, A-571 … A-575):** scripts take a relative `<dir>` and resolve it before any `cd`; the MSVC smoke test passes the system libraries of `native-static-libs` as `-l<name>` (the clang driver treats a bare `*.lib` as an input file); tests that connect to `localhost` must not assume IPv4 first (Windows tries `::1`), and tests that are not about the keep alive do not bound slow handshakes (qemu) by a 1 s keep alive. A run that GitHub does not start (billing) is no verification; the PO owns the account (D-2).

## Amendment C-106 (2026-11-22) – increment 3 as built: `micro.yml` (agent, G-3.4 b)

Details: `log/2026-11-22_C-106_micro-ci.md`, REVIEW A-576 … A-583. Logic in `oec/ci/micro-host.sh`, `micro-esp-idf.sh` and `micro-arduino.sh`; the workflow is generated as before (gate step 11: `cigen: 4 workflows current`). Test first: `TestMicro`, `TestRunners`.

1. **Jobs (A-576):** `host` on `ubuntu-24.04`, `ubuntu-24.04-arm` and `windows-2025` (60 min); `esp-idf` on `ubuntu-24.04` (120 min); `arduino` on `ubuntu-24.04` for the cores `3.3` and `4.0` (90 min). No artifacts: the CI keeps no firmware (the images would carry the placeholders only, but a firmware artifact is a release channel, Q-011).
2. **GCC on Linux (A-577):** the gate already runs `sdk/micro/check.sh` with clang (Linux and macOS); `micro.yml` runs it with `OEC_CI_CC=gcc` (CMake, sanitizers, cgo; the GCC of the runner image) and clang only for `clang --analyze`, selected by the new `OECM_ANALYZER` of `check.sh` (ADR-0030 amendment C-106). The GCC-only defect A-559 is the reason.
3. **Windows (A-578):** the MinGW gcc of the runner (the cgo compiler of the Go SDK, A-563) and `mingw32-make`; amd64 only. The D7 runs of the host runner (`internal/d7run`: `/bin/sh` restart hooks and POSIX signals; the A-550 area, unchanged), the sanitizers (MinGW has no ASan), the analysis and the symbol allowlist are Unix only; `micro-host.sh` runs the portable rest (gofmt, vet and `go test -race` of `./mqtt` and `./cmd/oecm-vecgen`, `go build ./cmd/...` with cgo, `oecm-vecgen -check`, CMake `MinGW Makefiles` with `-DOECM_SANITIZE=OFF` and ctest, GCC `-O2` with the warnings of the gate under LLP64, `oec-check validate` of the emitted payloads, `hello_device`, `arduino/package.sh`, the headers as C++17).
4. **ESP-IDF (A-579):** the image of ADR-0030 §8 at v6.1, pinned by its index digest in `versions.sh` (linux/amd64 and linux/arm64; Ubuntu 24.04 with the Espressif QEMU). `oec-check` and the `go test -c` binary of `cmd/oecm-qemu` are built on the host (`CGO_ENABLED=0`), `go-testbroker` natively on Linux or taken from `OEC_TESTBROKER`; the workspace is mounted at its own absolute path, the container runs as the calling user with `--init` (the restart hooks, A-546) and the `HOME` `/tmp/oec-home`. The build directories are recreated (a Mac build does not configure in Linux). `OEC_CI_CA_CERTS` is passed on for the component download.
5. **Arduino (A-580):** `arduino-cli` 1.5.1 from its GitHub release archive with the SHA-256 of `versions.sh` (no third-party action), an own home; Arduino-ESP32 3.3.12 and, as no 4.0.0 is released, its newest release candidate 4.0.0-RC1 from the development index; M5Unified/M5GFX pinned. Required: `Sensor` on ESP32 and ESP32-S3, `M5Stack` on M5Core2 and M5CoreS3, a warning in the library or a sketch fails; ESP32-C3/C6 best effort (an annotation, no failure). M5Core is not built (98 % of its app partition on 4.0).
6. **Caches (A-581):** only the crates (`~/.cargo/registry`, prefix `oec-micro-`, apart from `oec-sdk-` and the gate's tool cache). The cores (several GB each) and the Espressif image (≈ 8.5 GB) are not cached: they would exceed the 10 GB cache of the repository and evict the gate's caches; they are downloaded per run.
7. **Local verification (§8):** see the C-106 log – actionlint clean; `micro-host.sh` on macOS; in the Ubuntu 24.04 arm64 container the Linux GCC run and the Windows side as a MinGW cross build; `micro-esp-idf.sh` in the arm64 Espressif container; `micro-arduino.sh` for both cores on macOS; `linux.sh`. Open until the hosted runs (C-111): the amd64 image and runner disk, Windows MinGW native, the runner time of the jobs.
