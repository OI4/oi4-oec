# C-109 – P9 increment 6: versioning and the release workflow

- Date: 2026-11-25
- Phase: P9 – Hardening & release, increment 6 (ADR-0032 §9, §10)
- Baseline: guideline `52d8470` (branch `feature/spec-findings`, head `40204d7` before the cycle: the Community index of C-108 addendum 5 after `064aa39`, no guideline chapter changed); SF-001 … 117 resolved, next SF-118
- Workspace before the cycle: `c068234` (local; `origin/main` = `42f6784`; the CI is paused, C-105 D-2 (b))

## Goal

Increment 6 as handed over: one versioning scheme (SemVer) for every deliverable – Rust crates, Go modules with tag prefixes per module path, micro SDK, images, D7/D8, the schema with its own version – with its reasons (A-x) and, where a first public version number is a promise to the outside, a decision for the PO (D-x); a generated workflow `release.yml` (test first in `oec/ci/pipelines`, logic in scripts under `oec/ci/`), started by a tag or by hand, that builds every deliverable from one commit with the existing scripts (`binaries.sh`, `ffi-lib.sh`, `arduino/package.sh`, `sbom.sh`, the image `build.sh` as OCI archives), reproducibly where possible, with `SHA256SUMS`, the SBOMs and release notes – **as workflow artifacts only**: no registry push, no GitHub Release, no crates.io, Go proxy or ESP registry (Q-011, C-104 D-2).

## Plan

1. **Versioning (new ADR-0033):** SemVer 2.0.0; one release version for all code deliverables in lockstep (release train) with a single source `oec/VERSION`, enforced by a gate test over every manifest and constant that states it; the schema bundle (`2.MINOR.PATCH`, ADR-0019 §6), the report formats of D7/D8 and the micro SDK API version keep their own versions; tags for the train and for the Go modules (prefix = module directory) in the release repository `OI4/oec` (C-104 D-3 (a)); D7/D8 report the release version (`-X main.version`). First public version number and the Go module of the schema → D-x.
2. **Model extension of `oec/ci/gha`** (tests first): tag trigger (`on.push.tags`) and job dependencies (`needs:`) – the policy keeps `needs.` out of expressions; `actions/download-artifact` pinned by SHA.
3. **`oec/ci/release.sh <dir> [part…]`** (logic) and **`release.yml`** (test `TestRelease` first): parts `source` (`git archive` of `oec/`), `binaries` (D7/D8), `ffi` (prebuilt C ABI library per target on its native runner), `micro` (Arduino library ZIP), `schema` (bundle), `sbom`, `images` (OCI archives, judged by D8), `assemble` (deterministic archives, release notes from `oec/CHANGELOG.md` and the schema changelog, one `SHA256SUMS`); the release job collects the parts into one artifact. No cache in release jobs.
4. **Local verification:** `lint.sh`; `release.sh` twice on the Mac (same checksums); in a fresh Ubuntu 24.04 container (image of `versions.sh`, shellcheck); the images part in a container with an engine of its own (never on the host engine: the tags `oi4/oec-*:0.1.0` belong to the `oec/dev` stack of another session); `linux.sh`; the gate. Hosted-only items open for C-111.
5. **Docs:** ADR-0033, ADR-0032 amendment C-109, `oec/ci/README.md`, `oec/README.md`, Community pages of changed tools, ROADMAP, STATUS, REVIEW/REVIEW_DE (A-602 ff., D-1 ff.).

## Result

Increment 6 is built and verified locally. ADR-0033 fixes the versioning: SemVer, one release train `oec/VERSION` (`0.1.0`, the next version) for every code deliverable, checked by `TestReleaseVersion` over every manifest and default that states it; the schema bundle, the D7/D8 report formats and `OECM_API_VERSION` keep their own versions; a build without a release tag is a snapshot `<v>-snapshot.g<12 hex>`; D7/D8 report the build version. The generated workflow `release.yml` (tag `oec-v*` or by hand) builds every deliverable from one commit with the existing scripts through `oec/ci/release.sh` – source archive, D7/D8 for six platforms, `liboec_ffi` for eight targets, the `OECMicro` library, the schema bundle, the SBOMs, the five images as OCI archives with their D8 reports, `RELEASE-NOTES.md` from the new `oec/CHANGELOG.md` and one `SHA256SUMS` – as the single workflow artifact `oec-release`; nothing is published. Two runs give the same `SHA256SUMS` on macOS and in Ubuntu 24.04; macOS and Ubuntu give the same archives and D7/D8 files. Two defects of the new script were found and fixed on the way. Open for the PO: **D-1** the first public version, **D-2** the version of the schema Go module, **D-3** the channels at the first release. Exit criterion 7 is met locally; the hosted run follows in C-111.

## What was done

### 1. Versioning (ADR-0033, A-602 … A-606)

- `oec/VERSION` = `0.1.0`; `TestReleaseVersion` (`oec/ci/pipelines`, gate step 11) – test first – reads it and checks the Cargo workspace and packages (`sdk/rust`, `oec-config`, `oec-registry`), `library.properties`, `LOGGER_VERSION`, the weather version, the Web UI `package.json`/`package-lock.json`, `info.version` of the OpenAPI/AsyncAPI documents and the defaults of the Dockerfiles, image/SBOM scripts, compose files and tests (`7b5e768`). Every place already said `0.1.0`: no manifest changed.
- D7/D8 report the build version: their existing `var version = "dev"` is now set by `-X main.version=$VERSION` in `ci/binaries.sh` and the `conformance/Dockerfile` (`ARG VERSION`; the image said `dev` while its label said `0.1.0`); `image_test.go` pins the flag; `oec-check version` → `oec-check 0.1.0-snapshot.g… (schemas 2.0.1-dev, baseline 52d8470, …)`. Additive; the report formats are unchanged (the tool version was already the report field `version`). Pages: `oec/conformance/README.md`, Community `compliance/index.md`.
- `oec/CHANGELOG.md` (new): the changes of the train for users, `## 0.1.0 – unreleased` (the first release lists the deliverables).
- Tags (ADR-0033 §5): `oec-v<v>` in the workspace; in `OI4/oec` at publication `v<v>` and `<module dir>/v<v>`; the sibling `replace` directives go before that. The schema module is D-2.

### 2. `oec/ci/gha` and `cmd/pack` (A-611, A-608)

- The model got `on.push.tags` and job `needs` (tests first; the policy still rejects the `needs` context in expressions and every `${{` in `run`); `actions/download-artifact` v8.0.2 pinned by SHA (`0e35c0f`).
- `oec/ci/cmd/pack` (standard library; `f53b041`): `pack -format tar.gz|zip -mtime <unix> -prefix <dir/> -o <file> <dir>` – sorted entries, modes 0644/0755, owner 0, PAX tar, gzip/zip without host fields. Tests: two packs byte-identical, modes, prefix, no symlink escape.

### 3. `release.sh` and `release.yml` (A-607 … A-610)

- `oec/ci/release.sh <dir> [part…]` (`bf9faf7`): parts `source` (`git archive --mtime`), `binaries` (`binaries.sh`), `micro` (`arduino/package.sh` → `OECMicro-<v>.zip`), `schema` (`cmd/schematest -dist`), `sbom` (`sbom.sh`, packed), `ffi` (`ffi-lib.sh` of the host or `OEC_FFI_TARGET`, packed `.tar.gz`/`.zip`), `images` (the three `images/build.sh` with `VERSION`, OCI archives renamed with the version, reports packed; the build tags removed again) and `assemble` (checks the part records, writes `RELEASE-NOTES.md` and `SHA256SUMS`). Default: every part but `images`. Tag mode: tag `oec-v<VERSION>`, clean `oec/` tree, dated CHANGELOG section, released bundle with manifest; `assemble` then needs every part and all eight library targets.
- `release.yml` (`Release()` in `pipelines.go`, test first `TestRelease`): jobs `ffi` (matrix of the eight targets on the runners of `sdk.yml`), `dist`, `images` (`ubuntu-24.04`, buildx/QEMU as in `images.yml`) and `release` (needs all three; `download-artifact` pattern `oec-release-*`, merged; `assemble`; artifact `oec-release`, 30 days); `contents: read`, no caches. `TestRelease` also rejects `push`, `login`, `publish` and `upload` in the script. `cigen: 7 workflows current`.

### 4. Found and fixed (A-612)

- `git archive … HEAD:oec` started in `oec/` treats the tree path relative to the cwd – the first Mac run produced a 132-byte empty source archive. Now run from the top level (`git -C <toplevel> archive … <commit>:oec`), with a comment. Found before the commit.
- After the first runs, D7/D8 differed between the Mac and the Ubuntu container only by the build setting `vcs.modified=true`: an unrelated uncommitted file outside `oec/` on the Mac (`prompt.md`). `binaries.sh` now builds with `-buildvcs=false` – the release version names the commit (test first in `TestRelease`; `a102cdc`). The image build had no `.git` anyway.
- Observation, unchanged since C-107: an `.oci.tar` (buildx `type=oci`) carries no image name, and the host's classic Docker store refuses `docker load` of an OCI layout (`blobs/json: no such file`; tried with the logger archive, nothing was imported). Podman and Docker with the containerd image store load it; it is the input for a registry push (D-3) and for D8 `image`. Documented in `oec/ci/README.md` instead of changing the image scripts.

### 5. Local verification

- `cd oec/ci && go test ./...`: ok; `oec/ci/lint.sh` (actionlint 1.7.12): `.github/workflows ok`.
- Tag-mode refusals, one at a time: a wrong tag, a dirty tree, an "unreleased" CHANGELOG heading; in a scratch clone the bundle `2.0.1-dev` ("not released") and `2.0.1` without a manifest.
- Mac (arm64), `release.sh` (every part but `images`, host library `aarch64-apple-darwin`) twice at `bf9faf7` (`OEC_RELEASE_ALLOW_DIRTY=1`, `prompt.md`): **identical `SHA256SUMS`**, ~7 min per run.
- Ubuntu container (own script `/tmp/oec-c109-ct/run.sh`, container `oec-c109-ct`, `ubuntu_image` of `versions.sh` arm64, non-root, `HEAD` cloned from a read-only mount; `packages.sh --go --armv7`, shellcheck): `shellcheck -x oec/ci/release.sh` clean; `setup-tools.sh --toolchain`, `fetch.sh`; two runs (`source binaries micro schema sbom ffi`, then `ffi` for `armv7-unknown-linux-gnueabihf`, then `assemble`) → **identical `SHA256SUMS`** (19 files each, incl. the aarch64 and armv7 libraries). Against the Mac: the source archive (git 2.50 vs 2.43), the SBOM archive, the schema bundle and the `OECMicro` ZIP identical; D7/D8 identical after the `-buildvcs=false` fix (Mac `binaries` at `a102cdc` against the container below); the release notes differ by design (libraries, `rustc`).
- Images part in an engine of its own (own script `/tmp/oec-c109-img/run.sh`, privileged container `oec-c109-img` from `ubuntu_image` with `docker.io`, `docker-buildx`, its own `dockerd` and an anonymous volume, non-root user in group `docker`): `release.sh /out/i binaries images assemble` at `a102cdc` – five OCI archives (multi-arch index), D8 `image` 4 pass on all five, D8 `run` 3 pass on the four application images, the tool image's smoke runs report `oec-infra 0.1.0-snapshot.ga102cdc83149`, no `oi4/` image left, `SHA256SUMS` verified (`shasum -c`); ~20 min. Never on the host engine (its `oi4/oec-*:0.1.0` tags belong to another session's `oec/dev` stack).
- `linux.sh` and the gate: below.

### 6. Hosted runners – **open for C-111**

The hosted run of `release.yml`: the `ffi` targets on Windows (MSVC x64/arm64, MinGW) and macOS (arm64, amd64), the merge of the parts by `download-artifact`, the time and disk of the `images` job with five OCI archives on `ubuntu-24.04`, amd64 against the arm64 bytes of the local runs, the artifact size (~400 MB). Also from earlier cycles: `images.yml`, `supply-chain.yml`, Arduino-ESP32 4.0.0 final.

### 7. Documentation

ADR-0033 (new), ADR-0032 amendment C-109; `oec/ci/README.md` (layout, workflow row, section "Releases", local command), `oec/README.md` (`ci/` row, "Versions"), `oec/conformance/README.md` (tool version), `oec/CHANGELOG.md`; Community page `compliance/index.md` (prebuilt binaries from the release build, the tool version) – guideline commit `ae64fc3` on `feature/spec-findings`. ROADMAP P9, STATUS, REVIEW/REVIEW_DE.

## Commits

Workspace (local, not pushed): `5a8d55d` (plan), `0e35c0f` (gha tags and needs), `7b5e768` (`VERSION`, `TestReleaseVersion`, D7/D8 version stamp), `f53b041` (`cmd/pack`), `bf9faf7` (`release.yml`, `release.sh`, `CHANGELOG.md`), `a102cdc` (`-buildvcs=false`), `b9a9993` (ADRs, READMEs), then the documentation and this log. Guideline (local, not pushed): `ae64fc3`.

## Agent decisions (G-3.4 b)

A-602 … A-612 in `REVIEW.md`: SemVer (A-602), one release train (A-603), own versions (A-604), snapshots and version stamping (A-605), tags (A-606), `release.yml` (A-607), `release.sh` and reproducibility (A-608), release notes from the CHANGELOG (A-609), no crate/ESP-IDF component archives (A-610), the gha model (A-611), findings of the local verification (A-612).

## Decisions for the PO (G-3.4 a)

**D-1** first public version number (proposal (a) `0.1.0`), **D-2** version of the schema Go module (proposal (a) it follows the train, the bundle keeps `schema.Version`), **D-3** channels at the first release (proposal (a) GitHub Release on `OI4/oec`, images on GHCR, Go modules by tag; crates.io, ESP Component Registry, Arduino Library Manager later) – REVIEW "Open for you"; Q-011, C-104 D-2. None blocks C-110.

## Docker

Own objects only (no `prune`): containers `oec-c109-ct` and `oec-c109-img` (`--rm`, the latter's anonymous engine volume removed with it), the image `ubuntu:24.04@sha256:fe8156…` of `versions.sh` (pulled in this cycle, absent before; removed at the end with `docker image rm`), the container of `linux.sh` (removed by the script). The failed `docker load` of an OCI archive imported nothing. Foreign objects untouched.

## Temporary files

Created and deleted: `/tmp/oec-c109-rel*/` and their logs (Mac runs), `/tmp/oec-c109-macbin/` and `/tmp/oec-c109-macbin.sums`, `/tmp/oec-c109-ct/` and `/tmp/oec-c109-ct.log` (container runs), `/tmp/oec-c109-img/` and `/tmp/oec-c109-img.log` (images run), `/tmp/oec-c109-rep/` (extracted reports), the scratch clone of the tag-mode trial, `/tmp/oec-c109-linux.log`, `/tmp/gate.log`.

## linux.sh

`caffeinate -dims oec/ci/linux.sh` (async, without `OEC_CI_CA_CERTS`) cloned the committed head `b9a9993` (guideline `ae64fc3`) into a fresh Ubuntu container (`ubuntu:24.04` of `versions.sh`, aarch64): all 11 steps green, `cigen: 7 workflows current (conformance.yml, gate.yml, images.yml, micro.yml, release.yml, sdk.yml, supply-chain.yml)`, `docscheck: 19 pages ok, 30 README links to them ok`, "all checks passed", exit 0, ~25 min (load up to 12). The ADR-0032 amendment note (release notes from the CHANGELOG, `-buildvcs=false`) and the project documentation came after the clone; they are checked by the gate below.

## Gate

`caffeinate -dims oec/check.sh` on the Mac at `b9a9993` plus the uncommitted documentation (ADR-0032 amendment note, ROADMAP, STATUS, REVIEW, REVIEW_DE, this log; load 10 at the start): all 11 steps green on the first run, no repeats, `docscheck: 19 pages ok, 30 README links to them ok`, `cigen: 7 workflows current (conformance.yml, gate.yml, images.yml, micro.yml, release.yml, sdk.yml, supply-chain.yml)`, "all checks passed", ~17 min.

## Next

C-110 – P9 increment 7: the feedback package for the working group (ADR-0032 §10) – every spec finding SF-001 … 117 with finding, decision and text, and the guideline diff up to `52d8470`; how it is handed on is the PO's (Q-011). The CI stays paused until the exit run C-111 (C-105 D-2 (b)); C-109 D-1 … D-3 do not block C-110.

Handover (G-3.10): **new agent session** – the context of this session was compacted; C-110 works on the guideline register and diff, not on the release files. `prompt.md` written for C-110 (not committed).

## Addendum 1 (2026-11-25) – PO answers D-1 … D-3

The PO answered "D-1, D-2 und D-3 (a)".

- **D-1 (a):** the first public version number is **`0.1.0`** – an early version; SemVer allows API changes in a 0.x minor version; 1.0.0 follows when the PO calls the APIs stable. The CRA obligations (C-108) apply all the same. Nothing to change: `oec/VERSION` and every manifest already say `0.1.0` (`TestReleaseVersion`).
- **D-2 (a):** the Go module `github.com/OI4/oec/schema` **follows the release train** (`schema/v0.1.0` in `OI4/oec`); the bundle version `2.MINOR.PATCH` stays inside (`schema.Version`, release manifest, `oec-schema-<bundle>.tar.gz`, `oec-check version`). No path change.
- **D-3 (a):** at the first release, a **GitHub Release on `OI4/oec`** with the files of `oec-release` as assets, the five images on the **GitHub Container Registry** (`ghcr.io/oi4/…`, multi-arch) and the **Go modules through their tags** in `OI4/oec`. crates.io, the ESP Component Registry and the Arduino Library Manager come later, each on the PO's order.

Unchanged: nothing is published before the PO's go for the first release; whether `OI4/oec` is public stays Q-011 (CRA review); the weekly `supply-chain.yml` run and Dependabot come with the first release (C-108 D-3 (a)); before the first tag the schema bundle is frozen as `2.0.1` and the CHANGELOG section is dated.

Agent decision (G-3.4 b), **A-613:** the publishing step promised in D-3 (a) ("built in the cycle after your answer, test first") is built in **C-110** next to the feedback package, so that the exit run C-111 checks the whole release path: a publishing job behind the release build, tests first in `oec/ci/pipelines`, the logic in a script, inert until the PO's go (dry run without a tag, nothing published). What it needs from the PO – the repository `OI4/oec`, a write secret for it, the GHCR namespace – comes as a D-x of C-110 when the design is clear. The `replace` directives between the Go modules are replaced by tagged versions for the Go tags (ADR-0033 §5).

Applied: ADR-0033 (status, amendment C-109 addendum 1), REVIEW.md/REVIEW_DE.md (answered, A-613), STATUS, ROADMAP P9.
