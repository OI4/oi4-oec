# oec/ci – CI workflows (P9, ADR-0032)

The CI of the monorepo is GitHub Actions in the workspace repository. The
workflows are **generated** from a typed model in this Go module (standard
library only) and only set up tools and call the scripts of the local gate –
the same `oec/check.sh` that runs before every commit. Nothing here publishes
anything: the workflows become active when the repository is pushed to GitHub
(a decision of the PO, Q-011), and their binaries – also those of
`release.yml` – are workflow artifacts, not published releases.

## Layout

| Path | Content |
|---|---|
| `gha/` | the workflow model, the YAML renderer (plain scalars where unambiguous, double-quoted otherwise, literal blocks) and the CI policy `Check` |
| `pipelines/` | the pinned actions, the hosted runners and the workflows; tests against the repository (policy, generated files, tool pins, scripts, runners) |
| `cmd/pack/` | deterministic `.tar.gz`/`.zip` of a directory for `release.sh` (sorted entries, fixed modes and owner, one mtime, no host fields) |
| `cmd/cigen/` | renders the workflows into `../../.github/workflows`; `-check` fails if a file is outdated or not generated (gate step 11) |
| `versions.sh` | the pins: Rust toolchain, cbindgen, cargo-deny, actionlint, govulncheck, the guideline branch, the Ubuntu image of `linux.sh`; for `micro.yml` the Espressif ESP-IDF image (by digest), `arduino-cli` with the SHA-256 of its archives, the Arduino-ESP32 cores and the M5Stack libraries |
| `disk.sh` | frees disk space on a GitHub-hosted Linux runner (removes the preinstalled Android SDK, .NET, Haskell, Swift and CodeQL); nothing elsewhere (macOS, local, `linux.sh` container) |
| `packages.sh` | the system packages on Debian/Ubuntu (apt): build tools, clang with `libclang-rt-dev` (sanitizer runtime), LLVM, CMake, GCC 14; `--go` the Go of the distribution (`linux.sh`), `--armv7` the ARMv7 cross toolchain and `qemu-user` (`sdk-armv7.sh`); nothing on macOS |
| `setup-tools.sh` | installs the toolchain and tools of `versions.sh` (rustup; `cargo install --locked` into `~/.oec-tools`); `--toolchain` only the Rust toolchain (`sdk.yml`, `micro.yml`); `GITHUB_PATH` entries as Windows paths on Windows |
| `fetch.sh` | `cargo fetch --locked` for every `Cargo.lock` under `oec/` (the gate builds `--offline --locked`) |
| `conformance.sh` | gofmt, vet and tests of `oec/conformance` (D7/D8), as gate step 4 |
| `binaries.sh` | cross-builds `oec-check` and `oec-infra` for windows/linux/darwin × amd64/arm64 with `SHA256SUMS` (reproducible: `CGO_ENABLED=0`, `-trimpath`, `-buildvcs=false`, no build id); `VERSION` (default `dev`) is the version they report (`-X main.version`) |
| `sdk-rust.sh` | the Rust SDK on one runner of `sdk.yml`: clippy and tests of `oec/sdk/rust` for the host target (the C tests of `oec-ffi` with the sanitizers on Unix), `ffi-lib.sh`, and a C smoke test (`examples/hello.c` linked to the static library, run without a container storage: it must fail with the bootstrap error) |
| `sdk-go.sh` | the Go SDK on one runner of `sdk.yml`, as the Go part of `sdk/check.sh` (`oec-buildlib`, `go-testbroker`, `oec-check`, `go vet`, `go test -race`, examples); on Windows the library of the MinGW target `x86_64-pc-windows-gnu` (amd64 only), with `<dir>` also as prebuilt library |
| `sdk-armv7.sh` | ARMv7 Linux, best effort: cross build of the Rust SDK for `armv7-unknown-linux-gnueabihf` (aws-lc-rs; clippy also with `crypto-ring`), the Rust tests under `qemu-arm`, `ffi-lib.sh`, the C smoke test under `qemu-arm`, the Go SDK for `linux/arm` (`GOARM=7`: vet, examples, the example `hello` under `qemu-arm`) |
| `ffi-lib.sh` | the prebuilt library of one target: `<dir>/liboec_ffi-<target>/` with `include/oec.h`, `lib/liboec_ffi.a` (`lib/oec_ffi.lib` for MSVC), `native-static-libs.txt` (`rustc --print native-static-libs`, for MinGW plus the static winpthreads), `BUILDINFO`, `LICENSE`, `SHA256SUMS` |
| `micro-host.sh` | the micro SDK on one runner of `micro.yml`: on Linux/macOS `sdk/micro/check.sh` with the compiler `OEC_CI_CC` (GCC in the CI; the gate uses clang) and clang for the analysis (`OECM_ANALYZER`); on Windows (MinGW gcc, amd64) the portable part: gofmt, vet and `go test -race` of the portable packages, the commands built with cgo, `oecm-vecgen -check`, CMake (`MinGW Makefiles`, no sanitizers) and ctest, GCC `-O2` warnings, `oec-check validate` of the emitted payloads, `hello_device`, `arduino/package.sh`, the headers as C++17 |
| `micro-esp-idf.sh` | the ESP-IDF builds `esp32`, `esp32s3`, `qemu` and those of the smart sensor demo `demos-smart-sensors/esp32-dht22` (`esp32`, `esp32s3`, build only) in the Espressif container of `versions.sh` and D7 `--level 1/2/3` against the `qemu` firmware (`cmd/oecm-qemu`); `oec-check` and the test binary are built on the host for the container, `go-testbroker` natively (Linux) or from `OEC_TESTBROKER`; runs as the calling user with `--init`; removes its container and the image if it pulled it |
| `micro-arduino.sh` | the Arduino library on one core (`3.3` or `4.0`): `arduino-cli` of `versions.sh` (SHA-256 checked) with an own home `OECM_ARDUINO_HOME` (default `~/.oec-arduino`), the core and the M5Stack libraries, `arduino/package.sh`, the examples with `--warnings all` (a warning in the library or a sketch fails); ESP32-C3/C6 best effort |
| `images.sh` | the images of `images.yml`: `demos/images/build.sh`, `registry/images/build.sh` and `conformance/images/build.sh` into one directory – both platforms, D8 `image` on each OCI archive, D8 `run` on the native image of each application, the reports and logs kept, the archives deleted; the native images stay loaded as `oi4/oec-*:$VERSION` (default `0.1.0`) |
| `getting-started.sh` | the newcomer run: "See OEC at work" of the Community pages with curl and jq – `go run ./pki` (if `oec/dev/generated` is missing, as `live.sh`), compose up of the profiles `demos` and `registry`, the four Web UIs, the registry lists the demos and the stations, the config service sets the stations of the weather service to 4 and the registry lists station 4; always `down -v`; `OEC_COMPOSE` as in `live.sh` |
| `live-infra.sh` | `oec/conformance/live.sh` with `OEC_LIVE_REGISTRY=1` (D7 `monitor`, D8 `bus`/`host`/`container`/`run` incl. `TestRunDev`); fails without a container engine or the registry image |
| `podman.sh` | Podman on Linux (rootless, the user of the runner): the DNS plugin of the network backend (installed with apt on a runner), the API socket, `docker save` → `podman load`, D8 `run` through the Podman socket with `--listen`/`--broker-host` on the host address, `getting-started.sh` and `live-infra.sh` with `podman compose` plus `oec/dev/compose.podman.yaml` and a health-check ticker (no systemd timers needed); a table of what works; `OEC_PODMAN_LIMITS` (default empty) lists known limits (reported, not fatal). C-107 locally: all four steps work with Podman 4.9.3 (`oec/dev/README.md` "Platforms and Podman") |
| `linux-images.sh` | the job `images` of `images.yml` on a fresh Ubuntu 24.04 in a privileged container with a Docker engine of its own (opt-in; `OEC_CI_PODMAN=1` adds `podman.sh`, with storage driver `vfs` – nested in Docker Desktop the overlay driver failed) |
| `sbom.sh` | the SBOM (SPDX 2.3 JSON), the license texts and the license report `LICENSES.md` of every deliverable into `<dir>/<deliverable>/` with `oec-sbom` (no build: the crates from `cargo tree` over the eight targets of the SDK matrix, the Go modules from `go list -deps` over the six platforms of the binaries; needs the crates fetched); the image SBOMs stay in the images |
| `audit.sh` | the vulnerability check with network: `govulncheck` (pinned, `go run`) on every Go module, `cargo deny check advisories` with the fresh RustSec database on every `Cargo.lock` outside the spikes, `npm audit` of the Registry Web UI; every check runs, the outputs go to `<dir>`, it fails at the end on any finding |
| `release.sh` | the release of `release.yml` (ADR-0033): `release.sh <dir> [source binaries micro schema sbom ffi images assemble]` builds the parts from the checked-out commit with the scripts above, `assemble` packs them and writes `RELEASE-NOTES.md` and `SHA256SUMS`; a tag `oec-v<VERSION>` makes a release, every other build a snapshot `<VERSION>-snapshot.g<commit>` |
| `publish.sh` | the publication of a release (ADR-0034): `publish.sh <dir> [--publish]` checks the release of `release.sh`, prepares the release repository `OI4/oec` (commit, Go module tags, `go.sum`) and checks it with `go install`, checks the images and writes the body of the GitHub Release; a dry run without `--publish` (every run of `release.yml`); with `--publish` only on a release tag in the job `publish` |
| `lint.sh` | actionlint on the workflows (opt-in locally, a job of `gate.yml`) |
| `linux.sh` | the job `gate` of `gate.yml` in an Ubuntu 24.04 container (opt-in) |

## Workflows

| File | Jobs | Runners |
|---|---|---|
| `gate.yml` | `gate`: guideline checkout next to `oec/` (branch of `versions.sh`, full history for the inventory check), Go, cache of the cargo tools and crates, `disk.sh`, `packages.sh`, `setup-tools.sh`, `fetch.sh`, `oec/check.sh` (`CARGO_INCREMENTAL=0`); `actionlint`: `lint.sh` | `ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-15`; actionlint on `ubuntu-24.04` |
| `conformance.yml` | `test`: `conformance.sh`; `binaries`: `binaries.sh`, upload as the artifact `oec-conformance-binaries` (14 days) | `ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-15`, `macos-15-intel`, `windows-2025`, `windows-11-arm`; binaries on `ubuntu-24.04` |
| `sdk.yml` | `rust`: `setup-tools.sh --toolchain`, `sdk-rust.sh`, upload `liboec_ffi-<target>`; `go`: `sdk-go.sh`, on Windows upload `liboec_ffi-x86_64-pc-windows-gnu`; `armv7`: `packages.sh --armv7`, `sdk-armv7.sh`, upload `liboec_ffi-armv7-unknown-linux-gnueabihf` (all artifacts 14 days) | `rust`: all six; `go`: all but `windows-11-arm` (no cgo toolchain); `armv7`: `ubuntu-24.04` |
| `micro.yml` | `host`: `packages.sh`, `setup-tools.sh --toolchain` (not on Windows), `micro-host.sh` with `OEC_CI_CC=gcc`; `esp-idf`: `disk.sh`, `setup-tools.sh --toolchain`, `micro-esp-idf.sh`; `arduino`: `disk.sh`, `micro-arduino.sh` for the cores `3.3` and `4.0`; no artifacts (no firmware) | `host`: `ubuntu-24.04`, `ubuntu-24.04-arm`, `windows-2025`; `esp-idf` and `arduino`: `ubuntu-24.04` |
| `supply-chain.yml` | `sbom`: `setup-tools.sh --toolchain`, `fetch.sh`, `sbom.sh`, upload `oec-sbom` (14 days); `audit`: `setup-tools.sh`, `audit.sh`, upload `oec-audit` (14 days, also after a failure); no release | `ubuntu-24.04` |
| `release.yml` | `ffi`: `packages.sh` (armv7: `--armv7`), `setup-tools.sh --toolchain`, `release.sh … ffi`; `dist`: `setup-tools.sh --toolchain`, `fetch.sh`, `release.sh … source binaries micro schema sbom`; `images`: `release.sh … images`; `release` (after all three): the parts of the run, `release.sh … assemble`, the dry run `publish.sh`, upload `oec-release` (30 days); `publish` (after `release`, only on a tag, environment `release`): the artifact `oec-release`, `publish.sh --publish`; no caches | `ffi`: the runner of each target in `sdk.yml` (armv7 cross on `ubuntu-24.04`); the others `ubuntu-24.04` |
| `images.yml` | `images`: `disk.sh`, `images.sh` (the five images, D8 `image`/`run`), `getting-started.sh`, `live-infra.sh`, on `ubuntu-24.04` also `podman.sh`; upload of the D8 reports and logs as `oec-images-reports-<runner>` (14 days, also after a failure) – never an image or OCI archive | `ubuntu-24.04`, `ubuntu-24.04-arm` |

Triggers: push to `main`, pull requests, manual start; `release.yml` only a
tag `oec-v*` or a manual start. Pushes and pull requests that change only
`project/**` (logs, status, reviews, ADRs) start no workflow (`paths-ignore`,
C-111 D-3 (a)); the gate never reads `project/` (`TestPathsIgnore`). Every run step uses
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
- uses a secret other than `OEC_GUIDELINE_TOKEN` and `OEC_RELEASE_TOKEN`,
  the first anywhere but in the `token` of a checkout step, the second
  anywhere but in the `env` of a run step of a job in the environment
  `release`; or checks out with persisted credentials;
- gives a job an environment while the workflow runs on pull requests, or
  has an `if:` condition that reads a context an attacker controls.

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
there and `rustc` does not print it; for MSVC with clang add
`-fms-runtime-lib=dll -Wl,-nodefaultlib:libcmt`, as `sdk-rust.sh`). For the Go SDK,
copy the library to `oec/sdk/go/lib/<GOOS>_<GOARCH>/liboec_ffi.a` instead of
running `oec-buildlib`. Static libraries only; the shared library is not
built. `release.yml` packs the same libraries with the SBOM of `oec-ffi`
(C-109).

## Activation (PO, Q-011)

The workflows do nothing until the PO pushes the workspace to its GitHub
repository. The gate job checks out `OI4/oi4-oec-documentation`; while that
repository is private it needs a repository secret `OEC_GUIDELINE_TOKEN` –
without it the checkout falls back to `github.token`, which works only for a
public guideline repository. The branch `feature/spec-findings` must contain
the pages the gate checks (docscheck, step 10).

The token is used only as the `token` of that checkout (`gate.yml`,
`persist-credentials: false`). Use a **fine-grained** personal access token,
not a classic one (a classic token cannot be limited to one repository or to
read access on private repositories):

- resource owner `OI4`; repository access "Only select repositories" →
  `OI4/oi4-oec-documentation`;
- repository permission **Contents: Read-only** (Metadata: Read-only is
  added automatically), nothing else; an expiry date;
- stored as the Actions secret `OEC_GUIDELINE_TOKEN` of `OI4/oi4-oec`
  (`gh secret set OEC_GUIDELINE_TOKEN -R OI4/oi4-oec`).

If the organisation requires approval of fine-grained tokens, an owner of
`OI4` approves it.

**Publishing** (`release.yml` job `publish`, ADR-0034) needs, before the
first release tag:

- the repository `OI4/oec` (its visibility is Q-011); it may be empty;
- in `OI4/oi4-oec` the environment **`release`** with the PO as required
  reviewer and the deployment tag rule `oec-v*` (GitHub creates a missing
  environment without protection when a job names it – create it first);
- in that environment the secret **`OEC_RELEASE_TOKEN`** and the variable
  **`OEC_RELEASE_USER`** (the account of the token). The GitHub Container
  Registry accepts only a classic token: a **classic** personal access
  token of an account that may write `OI4/oec` and create packages of
  `OI4`, scopes `repo` (`public_repo` once `OI4/oec` is public) and
  `write:packages`, with an expiry date – best of a machine account with
  access to `OI4/oec` only;
- after the first push: link the packages `oec-*` to `OI4/oec` (the label
  `org.opencontainers.image.source` does it) and set their visibility; GHCR
  creates them private.

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
ci/images.sh /tmp/oec-images             # opt-in: the five images with D8 (Docker with buildx, network)
ci/getting-started.sh                    # opt-in: the newcomer run (needs the images, curl, jq; not next to a running oec/dev stack)
ci/live-infra.sh                         # opt-in: live.sh with the registry
ci/linux-images.sh /tmp/reports          # opt-in: the job images in Ubuntu 24.04 with its own dockerd (Docker, network, 30–60 min)
ci/sbom.sh /tmp/oec-sbom                 # opt-in: the SBOMs and license reports of every deliverable (offline after ci/fetch.sh)
ci/audit.sh /tmp/oec-audit               # opt-in: govulncheck, cargo deny check advisories, npm audit (network, needs npm)
ci/release.sh /tmp/oec-release           # opt-in: a snapshot release for this machine's library target (after ci/fetch.sh; ~10 min)
```

## Releases (`release.yml`, C-109)

One version for every code deliverable: [`../VERSION`](../VERSION), with
its changes in [`../CHANGELOG.md`](../CHANGELOG.md) (ADR-0033;
`TestReleaseVersion` checks every manifest and default that states it). The
JSON Schema bundle (`schema.Version`, `2.MINOR.PATCH`), the D7/D8 report
formats and `OECM_API_VERSION` keep their own versions.

| File | Content |
|---|---|
| `oec-<v>-source.tar.gz` | `oec/` at the commit (`git archive`) |
| `oec-check-<v>-<os>-<arch>`, `oec-infra-<v>-<os>-<arch>` | D7/D8 for six platforms, reporting `<v>` |
| `liboec_ffi-<v>-<target>.tar.gz` (`.zip` for Windows) | the prebuilt C ABI library of each of the eight targets of `sdk.yml` |
| `OECMicro-<v>.zip` | the Arduino library (the ESP-IDF component is in the source archive) |
| `oec-schema-<bundle>.tar.gz` | the JSON Schema bundle, both flavors |
| `oec-<v>-sbom.tar.gz` | `sbom.sh`: SBOM, license texts and report of every deliverable |
| `oec-<image>-<v>.oci.tar`, `oec-<v>-image-reports.tar.gz` | the five images (linux/amd64 + linux/arm64) and their D8 reports |
| `RELEASE-NOTES.md`, `SHA256SUMS` | facts and changes of the version; the checksums of every file |

`<v>` is `VERSION` on a tag build: the tag must be `oec-v<VERSION>`, the
tree clean, the CHANGELOG heading of the version without "unreleased" and
the schema bundle released (no `-dev`, a manifest in `schema/release/`).
Every other build is a snapshot `<VERSION>-snapshot.g<12 hex digits>`
(`OEC_RELEASE_ALLOW_DIRTY=1` allows uncommitted changes for a try).
`SOURCE_DATE_EPOCH` is the commit time: the archives (`cmd/pack`, `git
archive`), D7/D8, the libraries (same toolchain and paths) and the SBOMs
are reproducible – two runs give the same `SHA256SUMS` (C-109: macOS and
Ubuntu 24.04); the images are not (package installs in the builder
stages). `assemble` refuses parts of another commit or version and, on a
tag build, a missing part or library target. The images part needs Docker
with buildx and tags `oi4/oec-*:<v>` only while it runs: never run it next
to an `oec/dev` stack of the same version. An `.oci.tar` is an OCI image
layout with a multi-arch index and no image name – the input for an image
registry (`skopeo copy --all oci-archive:<file> docker://<name>:<v>`) and
for D8 `oec-infra image`; Podman and Docker with the containerd image store
load it, the classic Docker store does not.

A release commit sets `VERSION` and the places `TestReleaseVersion` lists,
and turns `## <v> – unreleased` into the release date; then the tag
`oec-v<v>` starts `release.yml`.

### Publishing (`publish.sh`, C-110)

The channels of C-109 D-3 (a), in this order (ADR-0034):

1. **The release repository `OI4/oec`:** one commit on `main` whose tree is
   `oec/` of the source archive, with the Go modules made usable from
   outside – every `replace` to a sibling module becomes a requirement of
   `v<v>`, and `go.sum` is completed by `go mod tidy` against the prepared
   repository itself (until it no longer changes). Tags `v<v>` and, for each
   of the twelve Go modules, `<module directory>/v<v>` (`conformance/v0.1.0`,
   `sdk/go/v0.1.0`, …). The commit has the time of the source commit and a
   fixed author: the same release gives the same tree and, in a dry run, the
   same commit (C-110: macOS and Ubuntu 24.04). Before anything leaves the
   runner, each module is checked with an empty module cache
   (`go list -mod=readonly -deps -test ./...`, `go mod verify`), and
   `go install github.com/OI4/oec/conformance/cmd/oec-check@v<v>` (and
   `oec-infra`) must work and report `<v>`.
2. **The images** `ghcr.io/oi4/oec-<image>:<v>`: `skopeo copy --all
   --preserve-digests` of each OCI archive (both platforms, the digest of
   the index is kept); the release body lists the digests.
3. **The GitHub Release `v<v>`** of `OI4/oec` with every file of
   `oec-release` as an asset (`gh release create --verify-tag`); its body is
   `RELEASE-NOTES.md` plus where to get the release.

Every run of `release.yml` runs `publish.sh` as a **dry run** in the job
`release`: it prepares and checks all of it and prints the commands that
would write (`publish: dry run, would run: …`); nothing leaves the runner,
no token is used. Only on a tag build does the job **`publish`** follow; it
runs in the GitHub environment **`release`** (the PO approves each run as
its required reviewer) and gets the environment secret `OEC_RELEASE_TOKEN`
in the `env` of its one run step. `publish.sh --publish` refuses anything
but the tag build `oec-v<v>` of a release version. The push is atomic; a
tag that already exists with the same tree is kept, with another tree the
run fails; an existing GitHub Release fails the run (a rerun after a failed
image push skips the repository part). The token goes to git as an HTTP
header from the environment (never in a URL or an argument), to skopeo in
an authfile with mode 600 in a temporary directory, to gh as `GH_TOKEN`;
the script never traces (`set -x`).

Locally: `release.sh <dir> source assemble` and `publish.sh <dir>`
(`OEC_PUBLISH_KEEP=<new dir>` keeps the prepared repository); needs git and
Go, skopeo for image archives. crates.io, the ESP Component Registry and
the Arduino Library Manager are not channels (C-109 D-3 (a); later, each
on the PO's order).

## Supply chain (`supply-chain.yml`, C-108)

`sbom.sh` writes one directory per deliverable – `oec-rust-sdk`, `oec-ffi`,
`oec-go-sdk`, `oec-micro-sdk`, `oec-check`, `oec-infra`, `oec-logger`,
`oec-weather`, `oec-config`, `oec-registry` (with the npm packages of the
Web UI), `oec-schema` – with `sbom.spdx.json`, the text of every license as
`<id>.txt` and `LICENSES.md` (the licenses with the number of components
and the components with their purl). The creation time is the commit time
(`SOURCE_DATE_EPOCH`), so the same commit gives the same files. A library
lists what a program that uses it links (the Go SDK: the crates of
`oec-ffi` and the Rust standard library); a binary also its language's
standard library. The micro SDK and the schema have no third-party
component (Unity is used by the tests only; ESP-IDF, esp-mqtt and the
Arduino core are the platform's). The images keep their own SBOM with the
Debian base in `/opt/oi4/licenses`, checked by D8 in `images.yml`.

`audit.sh` needs network and is never part of the gate (the gate's
`cargo deny check` falls back to the cached database). `govulncheck` fails
only on a vulnerability in called code; its output also lists the
module-level findings (`-show verbose`). It is built with the toolchain of
the checked module (an older Go cannot load a newer standard library).
Handling a finding: [`../SECURITY.md`](../SECURITY.md) and the dependency
rules (ADR-0023, ADR-0029 §10, ADR-0030).

`micro-esp-idf.sh` mounts the workspace at the same absolute path in the
container (the test binary finds `oec/` by its source path) and runs as the
calling user (`HOME=/tmp/oec-home`); its build directories
`sdk/micro/target/esp-idf/` and `demos-smart-sensors/esp32-dht22/target/`
are deleted first (a build of another host does
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
- **A tool:** change `versions.sh` (cbindgen, cargo-deny, actionlint and
  govulncheck must be release versions); the tests compare cbindgen with
  `sdk/check.sh` and the Rust toolchain with the `rust-version` of every
  Cargo package; the cache key of `gate.yml` changes with the file.
- **Go:** the workflows read the Go version from `go.mod`
  (`go-version-file`), `GOTOOLCHAIN=auto` fetches the `toolchain` of each
  module.
