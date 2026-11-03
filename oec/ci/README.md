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
| `versions.sh` | the pins: Rust toolchain, cbindgen, cargo-deny, actionlint, the guideline branch, the Ubuntu image of `linux.sh` |
| `packages.sh` | the system packages on Debian/Ubuntu (apt): build tools, clang with `libclang-rt-dev` (sanitizer runtime), LLVM, CMake, GCC 14; nothing on macOS |
| `setup-tools.sh` | installs the toolchain and tools of `versions.sh` (rustup; `cargo install --locked` into `~/.oec-tools`) |
| `fetch.sh` | `cargo fetch --locked` for every `Cargo.lock` under `oec/` (the gate builds `--offline --locked`) |
| `conformance.sh` | gofmt, vet and tests of `oec/conformance` (D7/D8), as gate step 4 |
| `binaries.sh` | cross-builds `oec-check` and `oec-infra` for windows/linux/darwin × amd64/arm64 with `SHA256SUMS` (reproducible: `CGO_ENABLED=0`, `-trimpath`, no build id) |
| `lint.sh` | actionlint on the workflows (opt-in locally, a job of `gate.yml`) |
| `linux.sh` | the job `gate` of `gate.yml` in an Ubuntu 24.04 container (opt-in) |

## Workflows

| File | Jobs | Runners |
|---|---|---|
| `gate.yml` | `gate`: guideline checkout next to `oec/` (branch of `versions.sh`, full history for the inventory check), Go, cache of the cargo tools and crates, `setup-tools.sh`, `fetch.sh`, `oec/check.sh`; `actionlint`: `lint.sh` | `ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-15`; actionlint on `ubuntu-24.04` |
| `conformance.yml` | `test`: `conformance.sh`; `binaries`: `binaries.sh`, upload as the artifact `oec-conformance-binaries` (14 days) | `ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-15`, `macos-15-intel`, `windows-2025`, `windows-11-arm`; binaries on `ubuntu-24.04` |

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
```

`linux.sh` clones the committed state (HEAD of this repository and the
guideline branch), so commit first. It runs as a non-root user with
`CC=clang` as on the runner, removes its container on exit and removes the
Ubuntu image if it pulled it. The container has the architecture of the
Docker host (arm64 on Apple silicon, the counterpart of `ubuntu-24.04-arm`).
It is limited to `OEC_CI_CPUS` CPUs (default 4, as a hosted Linux runner):
cargo and Go follow the quota, the links of the Rust tests stay within the
memory of a small Docker VM (8 GB: 14 parallel links were killed), and the
run time estimates the runner's.

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
