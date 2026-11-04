# oec/ci – CI workflows (P9, ADR-0032)

The CI of the monorepo is GitHub Actions in the workspace repository. The
workflows are **generated** from a typed model in this Go module (standard
library only) and only set up tools and call the scripts of the local gate –
the same `oec/check.sh` that runs before every commit. Nothing here publishes
anything: the workflows become active when the repository is pushed to GitHub
(a decision of the PO, Q-011), and their binaries are workflow artifacts, not
releases.

## Layout

| Path | Content |
|---|---|
| `gha/` | the workflow model, the YAML renderer (plain scalars where unambiguous, double-quoted otherwise, literal blocks) and the CI policy `Check` |
| `pipelines/` | the pinned actions, the hosted runners and the workflows; tests against the repository (policy, generated files, tool pins, scripts, runners) |
| `cmd/cigen/` | renders the workflows into `../../.github/workflows`; `-check` fails if a file is outdated or not generated (gate step 11) |
| `versions.sh` | the pins: Rust toolchain, cbindgen, cargo-deny, actionlint, the guideline branch, the Ubuntu image of `linux.sh`; for `micro.yml` the Espressif ESP-IDF image (by digest), `arduino-cli` with the SHA-256 of its archives, the Arduino-ESP32 cores and the M5Stack libraries |
| `disk.sh` | frees disk space on a GitHub-hosted Linux runner (removes the preinstalled Android SDK, .NET, Haskell, Swift and CodeQL); nothing elsewhere (macOS, local, `linux.sh` container) |
| `packages.sh` | the system packages on Debian/Ubuntu (apt): build tools, clang with `libclang-rt-dev` (sanitizer runtime), LLVM, CMake, GCC 14; `--go` the Go of the distribution (`linux.sh`), `--armv7` the ARMv7 cross toolchain and `qemu-user` (`sdk-armv7.sh`); nothing on macOS |
| `setup-tools.sh` | installs the toolchain and tools of `versions.sh` (rustup; `cargo install --locked` into `~/.oec-tools`); `--toolchain` only the Rust toolchain (`sdk.yml`, `micro.yml`); `GITHUB_PATH` entries as Windows paths on Windows |
| `fetch.sh` | `cargo fetch --locked` for every `Cargo.lock` under `oec/` (the gate builds `--offline --locked`) |
| `conformance.sh` | gofmt, vet and tests of `oec/conformance` (D7/D8), as gate step 4 |
| `binaries.sh` | cross-builds `oec-check` and `oec-infra` for windows/linux/darwin × amd64/arm64 with `SHA256SUMS` (reproducible: `CGO_ENABLED=0`, `-trimpath`, no build id) |
| `sdk-rust.sh` | the Rust SDK on one runner of `sdk.yml`: clippy and tests of `oec/sdk/rust` for the host target (the C tests of `oec-ffi` with the sanitizers on Unix), `ffi-lib.sh`, and a C smoke test (`examples/hello.c` linked to the static library, run without a container storage: it must fail with the bootstrap error) |
| `sdk-go.sh` | the Go SDK on one runner of `sdk.yml`, as the Go part of `sdk/check.sh` (`oec-buildlib`, `go-testbroker`, `oec-check`, `go vet`, `go test -race`, examples); on Windows the library of the MinGW target `x86_64-pc-windows-gnu` (amd64 only), with `<dir>` also as prebuilt library |
| `sdk-armv7.sh` | ARMv7 Linux, best effort: cross build of the Rust SDK for `armv7-unknown-linux-gnueabihf` (aws-lc-rs; clippy also with `crypto-ring`), the Rust tests under `qemu-arm`, `ffi-lib.sh`, the C smoke test under `qemu-arm`, the Go SDK for `linux/arm` (`GOARM=7`: vet, examples, the example `hello` under `qemu-arm`) |
| `ffi-lib.sh` | the prebuilt library of one target: `<dir>/liboec_ffi-<target>/` with `include/oec.h`, `lib/liboec_ffi.a` (`lib/oec_ffi.lib` for MSVC), `native-static-libs.txt` (`rustc --print native-static-libs`, for MinGW plus the static winpthreads), `BUILDINFO`, `LICENSE`, `SHA256SUMS` |
| `micro-host.sh` | the micro SDK on one runner of `micro.yml`: on Linux/macOS `sdk/micro/check.sh` with the compiler `OEC_CI_CC` (GCC in the CI; the gate uses clang) and clang for the analysis (`OECM_ANALYZER`); on Windows (MinGW gcc, amd64) the portable part: gofmt, vet and `go test -race` of the portable packages, the commands built with cgo, `oecm-vecgen -check`, CMake (`MinGW Makefiles`, no sanitizers) and ctest, GCC `-O2` warnings, `oec-check validate` of the emitted payloads, `hello_device`, `arduino/package.sh`, the headers as C++17 |
| `micro-esp-idf.sh` | the ESP-IDF builds `esp32`, `esp32s3`, `qemu` in the Espressif container of `versions.sh` and D7 `--level 1/2/3` against the `qemu` firmware (`cmd/oecm-qemu`); `oec-check` and the test binary are built on the host for the container, `go-testbroker` natively (Linux) or from `OEC_TESTBROKER`; runs as the calling user with `--init`; removes its container and the image if it pulled it |
| `micro-arduino.sh` | the Arduino library on one core (`3.3` or `4.0`): `arduino-cli` of `versions.sh` (SHA-256 checked) with an own home `OECM_ARDUINO_HOME` (default `~/.oec-arduino`), the core and the M5Stack libraries, `arduino/package.sh`, the examples with `--warnings all` (a warning in the library or a sketch fails); ESP32-C3/C6 best effort |
| `lint.sh` | actionlint on the workflows (opt-in locally, a job of `gate.yml`) |
| `linux.sh` | the job `gate` of `gate.yml` in an Ubuntu 24.04 container (opt-in) |

## Workflows

| File | Jobs | Runners |
|---|---|---|
| `gate.yml` | `gate`: guideline checkout next to `oec/` (branch of `versions.sh`, full history for the inventory check), Go, cache of the cargo tools and crates, `disk.sh`, `packages.sh`, `setup-tools.sh`, `fetch.sh`, `oec/check.sh` (`CARGO_INCREMENTAL=0`); `actionlint`: `lint.sh` | `ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-15`; actionlint on `ubuntu-24.04` |
| `conformance.yml` | `test`: `conformance.sh`; `binaries`: `binaries.sh`, upload as the artifact `oec-conformance-binaries` (14 days) | `ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-15`, `macos-15-intel`, `windows-2025`, `windows-11-arm`; binaries on `ubuntu-24.04` |
| `sdk.yml` | `rust`: `setup-tools.sh --toolchain`, `sdk-rust.sh`, upload `liboec_ffi-<target>`; `go`: `sdk-go.sh`, on Windows upload `liboec_ffi-x86_64-pc-windows-gnu`; `armv7`: `packages.sh --armv7`, `sdk-armv7.sh`, upload `liboec_ffi-armv7-unknown-linux-gnueabihf` (all artifacts 14 days) | `rust`: all six; `go`: all but `windows-11-arm` (no cgo toolchain); `armv7`: `ubuntu-24.04` |
| `micro.yml` | `host`: `packages.sh`, `setup-tools.sh --toolchain` (not on Windows), `micro-host.sh` with `OEC_CI_CC=gcc`; `esp-idf`: `disk.sh`, `setup-tools.sh --toolchain`, `micro-esp-idf.sh`; `arduino`: `disk.sh`, `micro-arduino.sh` for the cores `3.3` and `4.0`; no artifacts (no firmware) | `host`: `ubuntu-24.04`, `ubuntu-24.04-arm`, `windows-2025`; `esp-idf` and `arduino`: `ubuntu-24.04` |

Triggers: push to `main`, pull requests, manual start. Every run step uses
`bash` (Git Bash on Windows). The repository is checked out with LF line
endings everywhere (`.gitattributes`).

## Policy (ADR-0032 §4)

`gha.Check` fails the generation and the gate if a workflow

- uses an action outside `actions/*` or not pinned to the full commit SHA of
  a release in the pin list (`pipelines.Checkout` …; the tag is a comment);
- grants more than `permissions: contents: read`, or has no concurrency group
  with `cancel-in-progress`;
- uses a runner outside the hosted runners of `pipelines.Runners`, or a job
  without `timeout-minutes` (at most 240);
- has an expression `${{ … }}` in a `run` (values go through `env`), or one
  that reads a context an attacker controls (`github.event`,
  `github.head_ref`, `inputs`, `steps`, `needs`);
- uses a secret other than `OEC_GUIDELINE_TOKEN`, or uses it anywhere but in
  the `token` of a checkout step; or checks out with persisted credentials.

The triggers `pull_request_target` and `workflow_run` and self-hosted runners
cannot be expressed in the model.

## Prebuilt libraries (`sdk.yml`)

Each run of `sdk.yml` leaves one artifact per target, built from the
commit of the run with the default features of `oec-ffi` (aws-lc-rs):

| Artifact | Built on | Library |
|---|---|---|
| `liboec_ffi-x86_64-unknown-linux-gnu`, `liboec_ffi-aarch64-unknown-linux-gnu` | `ubuntu-24.04`, `ubuntu-24.04-arm` | `lib/liboec_ffi.a` (glibc) |
| `liboec_ffi-aarch64-apple-darwin`, `liboec_ffi-x86_64-apple-darwin` | `macos-15`, `macos-15-intel` | `lib/liboec_ffi.a` |
| `liboec_ffi-x86_64-pc-windows-msvc`, `liboec_ffi-aarch64-pc-windows-msvc` | `windows-2025`, `windows-11-arm` | `lib/oec_ffi.lib` (DLL CRT) |
| `liboec_ffi-x86_64-pc-windows-gnu` | `windows-2025` (job `go`) | `lib/liboec_ffi.a` (MinGW; for cgo) |
| `liboec_ffi-armv7-unknown-linux-gnueabihf` | `ubuntu-24.04` (cross) | `lib/liboec_ffi.a` (glibc, best effort) |

Link a C program with the header of `include/`, the library and the system
libraries of `native-static-libs.txt` (`rustc --print native-static-libs`;
for MinGW `ffi-lib.sh` adds `-l:libpthread.a`: aws-lc uses winpthreads
there and `rustc` does not print it). For the Go SDK,
copy the library to `oec/sdk/go/lib/<GOOS>_<GOARCH>/liboec_ffi.a` instead of
running `oec-buildlib`. Static libraries only; the shared library, a
third-party license report (C-108) and releases (Q-011) are not part of the
artifacts.

## Activation (PO, Q-011)

The workflows do nothing until the PO pushes the workspace to its GitHub
repository. The gate job checks out `OI4/oi4-oec-documentation`; while that
repository is private it needs a repository secret `OEC_GUIDELINE_TOKEN` (a
fine-grained token with read access to its contents) – without it the
checkout falls back to `github.token`, which works only for a public
guideline repository. The branch `feature/spec-findings` must contain the
pages the gate checks (docscheck, step 10).

## Local checks

```sh
cd oec/ci && go run ./cmd/cigen          # regenerate .github/workflows after a change here
./check.sh                               # from oec/: step 11 runs gofmt, vet, tests and cigen -check
ci/lint.sh                               # opt-in: actionlint (network)
ci/linux.sh > /tmp/linux.log 2>&1        # opt-in: the gate job in Ubuntu 24.04 (Docker, network, 1–2 h)
ci/sdk-rust.sh /tmp/sdk                  # opt-in: the job rust of sdk.yml on this machine (network for crates)
ci/sdk-go.sh                             # opt-in: the job go of sdk.yml on this machine
ci/micro-host.sh                         # opt-in: the job host of micro.yml (OEC_CI_CC=gcc as in the CI)
ci/micro-esp-idf.sh                      # opt-in: the job esp-idf of micro.yml (Docker, ~9 GB image)
ci/micro-arduino.sh 4.0                  # opt-in: the job arduino of micro.yml (network, ~8 GB per core)
```

`micro-esp-idf.sh` mounts the workspace at the same absolute path in the
container (the test binary finds `oec/` by its source path) and runs as the
calling user (`HOME=/tmp/oec-home`); its build directories
`sdk/micro/target/esp-idf/` are deleted first (a build of another host does
not configure there). It needs a `go-testbroker` of the container's
architecture: on Linux it builds one with cargo, elsewhere pass
`OEC_TESTBROKER=<linux binary>` (on macOS build it in the Ubuntu image of
`versions.sh`). `OEC_CI_CA_CERTS` is passed on for the component download
of `espressif/mqtt` in networks with TLS interception.

`micro-arduino.sh` keeps one core at a time below `OECM_ARDUINO_HOME`
(`core install` replaces the other version) and deletes the downloads after
the install; the CI caches neither the cores (several GB each, beyond the
cache quota of the repository) nor the firmware.

`sdk-armv7.sh` needs the Debian/Ubuntu cross packages (`packages.sh
--armv7`); on macOS run it in an Ubuntu container.

`linux.sh` clones the committed state (HEAD of this repository and the
guideline branch), so commit first. It runs as a non-root user with
`CC=clang` and `CARGO_INCREMENTAL=0` as on the runner (smaller targets;
without it the Docker VM ran out of disk in C-105), removes its container on exit and removes the
Ubuntu image if it pulled it. The container has the architecture of the
Docker host (arm64 on Apple silicon, the counterpart of `ubuntu-24.04-arm`).
It is limited to `OEC_CI_CPUS` CPUs (default 4, as a hosted Linux runner):
cargo and Go follow the quota, the links of the Rust tests stay within the
memory of a small Docker VM (8 GB: 14 parallel links were killed), and the
run time estimates the runner's. `--init` reaps orphaned processes as
systemd does on the runner (the restart hooks of the D7 tests start the
application in the background; without an init it stays a zombie when
stopped and the stop hook waits forever).

Behind a TLS-intercepting proxy (the host trusts the proxy's root
certificate, the container does not; Go then fails with `x509: certificate
signed by unknown authority`), export that root certificate as PEM to a file
outside the repository and set `OEC_CI_CA_CERTS=/path/to/root.pem`; the
file is mounted read-only and added to the container's trust store. Never
commit such a file. On macOS:
`security find-certificate -a -c "<CA name>" -p /Library/Keychains/System.keychain > /tmp/root.pem`.

## Updating a pin

- **An action:** take the commit SHA of its latest release tag
  (`gh api repos/actions/<name>/commits/<tag> --jq .sha`), change it in
  `pipelines/pipelines.go`, run `cigen`.
- **A tool:** change `versions.sh`; the tests compare cbindgen with
  `sdk/check.sh` and the Rust toolchain with the `rust-version` of every
  Cargo package; the cache key of `gate.yml` changes with the file.
- **Go:** the workflows read the Go version from `go.mod`
  (`go-version-file`), `GOTOOLCHAIN=auto` fetches the `toolchain` of each
  module.
