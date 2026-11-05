# ADR-0033 – Versioning and releases (P9)

- Status: **accepted (agent, G-3.4 b)** for the versioning scheme, the single version source, the snapshot versions, the release build `release.sh`/`release.yml` and its artifacts; **decisions of the PO** (G-3.4 a, Q-011, C-104 D-2): the first public version number (C-109 D-1), the Go module of the schema bundle (C-109 D-2), the release channels and their visibility (C-109 D-3, a proposal) and the publication itself.
- Date: 2026-11-25
- Deciders: agent (scheme, sources, build, artifacts); product owner (Thomas) – first public version, publication, channels
- Related: ADR-0019 §6 and amendment C-041 (schema bundle `2.MINOR.PATCH`, release manifests), ADR-0026 (Rust SDK), ADR-0027 (Go SDK), ADR-0028 amendment C-108 (SBOM tool), ADR-0030 (micro SDK), ADR-0032 §8 … §10 and its amendments (CI, local verification, releases), C-104 D-2/D-3 (channels pending; release repository `OI4/oec`), C-108 D-3 (a) (weekly supply-chain run and Dependabot from the first release), Q-011 (publication, CRA), ROADMAP P9 increment 6

## Context

Up to C-108 every deliverable carried a version of its own making: `0.1.0` in the Rust manifests, `library.properties`, the Registry Web UI and API documents, the image tags and the SBOM defaults – set by hand when each was built, with nothing that kept them equal; the JSON Schema bundle `2.0.0` (frozen) / `2.0.1-dev` with release manifests (ADR-0019 §6); D7/D8 printed `dev` (no version stamped in their builds) next to the bundle version and report format `1`; the micro SDK has `OECM_API_VERSION 1`. Go modules have no version in a file, only tags. Nothing had been released; the first public release decides what a version number promises (Q-011, C-104 D-2/D-3).

The deliverables are built together, tested together by one gate and depend on each other along fixed paths (the Go SDK and the logger on the C ABI, the demos and the registry on the SDKs, D7/D8 on the schema bundle and the test suite). Every image contains one deliverable and its SBOM names its version.

## Decision

### 1. Semantic Versioning 2.0.0 (A-602)

Every version is a SemVer 2.0.0 version `MAJOR.MINOR.PATCH`. Before 1.0.0 a MINOR version may change the public API (SemVer §4); from 1.0.0 an incompatible change of a public API (the Rust and Go APIs, the C ABI and `oec.h`, the micro SDK API, the command lines and exit codes of D7/D8, the REST/SSE API of the registry) needs a new MAJOR version.

### 2. One release train for the code deliverables, one source (A-603)

The Rust SDK (`oec-core`, `oec`), the C ABI `oec-ffi`, the Go SDK, the micro SDK (ESP-IDF component and Arduino library `OECMicro`), D7 `oec-check`, D8 `oec-infra`, the demos (logger, weather, config service), the OEC Registry (with its Web UI, OpenAPI and AsyncAPI documents) and their images share **one version**, the release train, in the file **`oec/VERSION`**. Every manifest and constant that states a version of the train states it: `[workspace.package] version` of `sdk/rust`, the packages `oec-config` and `oec-registry`, `version=` of `library.properties`, `LOGGER_VERSION`, `version` of the weather demo, `version` of the Web UI `package.json`/`package-lock.json`, `info.version` of the API documents, and the defaults `ARG VERSION=`, `${VERSION:-…}`, `${OEC_DEMOS_VERSION:-…}`, `${OEC_REGISTRY_VERSION:-…}` of the Dockerfiles, image and SBOM scripts, compose files and tests. **`TestReleaseVersion`** (`oec/ci/pipelines`, gate step 11) fails on a difference; a release bumps `VERSION` and these places in one commit.

Reasons: the deliverables are tested only together (one gate, one commit) – separate versions would claim compatibility combinations nobody tested; a user of the Go SDK needs the `liboec_ffi` of the same version, an image the D7/D8 that judged it; one number is what a user can report. Cost: a deliverable gets a new version without a change of its own (the changelog says so).

### 3. Own versions (A-604)

- **JSON Schema bundle**: `2.MINOR.PATCH` (ADR-0019 §6) – its MAJOR is the guideline's; `schema.Version`, release manifest `schema/release/<version>.json`, `schema/CHANGELOG.md`; a pre-release `2.MINOR.PATCH-dev` between releases. Tag `oec-schema-<version>` (ADR-0019 amendment C-041).
- **Report formats** of D7 and D8: version `1`, frozen (`report.MonitorVersion`, `report.InfraVersion`; a new field is a new format version).
- **Micro SDK C API**: `OECM_API_VERSION` in `oecm.h` (ADR-0030; an application checks it at compile time).

D7/D8 print all of them: `oec-check version` → `oec-check <train> (schemas <bundle>, baseline <guideline commit>, …, report 1)`.

### 4. Next version in the sources, snapshots in between (A-605)

The sources always hold the **next** version of the train (now `0.1.0`); `oec/CHANGELOG.md` collects its changes under `## <version> – unreleased`. A build from a commit without a release tag is a **snapshot** `<version>-snapshot.g<12 hex digits of the commit>` – a SemVer pre-release, so it sorts below the release; allowed in Docker tags and file names (no `+`). The release scripts pass it to the builds (`VERSION`): D7/D8 report it (`-X main.version`, `ci/binaries.sh`, `conformance/Dockerfile`; without it they report `dev`), the image tags, OCI labels and SBOMs carry it. The applications report the version of their sources in the MAM (`SoftwareRevision`; compiled in, no build parameter) – a snapshot image of the logger says `0.1.0` inside and `0.1.0-snapshot.g…` on its tag.

### 5. Tags (A-606)

- In the workspace repository: **`oec-v<version>`** on the release commit starts `release.yml` (§6); `oec-schema-<version>` for the bundle (ADR-0019).
- In the release repository `OI4/oec` (C-104 D-3 (a), `oec/` becomes its root) at publication: `v<version>` for the train and, for the Go modules, `<module directory>/v<version>` (`conformance/v0.1.0`, `sdk/go/v0.1.0`, `demos/v0.1.0`, `registry/v0.1.0`, …; Go's rule for modules in subdirectories). Before that the `replace` directives to sibling modules (`conformance`, `demos`, `schema/tools`) are replaced by the tagged versions – otherwise `go install …/cmd/oec-check@v0.1.0` fails. The module of the schema bundle is D-2.

### 6. The release build: `oec/ci/release.sh` and `release.yml` (A-607 … A-610)

`release.sh <dir> [part…]` builds the parts of the release from the checked-out commit with the existing scripts into the flat directory `<dir>`:

| part | file(s) | built by |
|---|---|---|
| `source` | `oec-<v>-source.tar.gz` – `oec/` at the commit | `git archive --mtime` |
| `binaries` | `oec-check-<v>-<os>-<arch>[.exe]`, `oec-infra-…` (6 platforms) | `ci/binaries.sh` |
| `micro` | `OECMicro-<v>.zip` (Arduino library) | `sdk/micro/arduino/package.sh` |
| `schema` | `oec-schema-<bundle>.tar.gz` (both flavors, CHANGELOG, LICENSE) | `cmd/schematest -dist` |
| `sbom` | `oec-<v>-sbom.tar.gz` (11 deliverables) | `ci/sbom.sh` |
| `ffi` | `liboec_ffi-<v>-<target>.tar.gz`/`.zip` (8 targets, `.zip` for Windows) | `ci/ffi-lib.sh` |
| `images` | `oec-<image>-<v>.oci.tar` (5 images, linux/amd64 + linux/arm64), `oec-<v>-image-reports.tar.gz` (D8 reports, logs) | the three `images/build.sh` |
| `assemble` | `RELEASE-NOTES.md`, `SHA256SUMS` | – |

- **One commit:** every part writes a record (commit, version); `assemble` refuses parts of another commit or version and, on a tag build, a missing part or target.
- **Tag build** (`GITHUB_REF_TYPE=tag`): the tag must be `oec-v<VERSION>`, the tree clean, the CHANGELOG section of the version without "unreleased", the schema bundle released (no `-dev`, manifest present) – a release ships only a frozen bundle. Otherwise a snapshot (§4); a dirty tree only with `OEC_RELEASE_ALLOW_DIRTY=1` (local tries).
- **Reproducible where the toolchain allows** (A-608): `SOURCE_DATE_EPOCH` = commit time; archives by `oec/ci/cmd/pack` (Go standard library: sorted entries, fixed modes, owner 0, mtime = commit time, PAX tar + gzip/zip without host fields); D7/D8 `-trimpath -buildid=`; the C ABI library with the same toolchain and paths (`ffi-lib.sh`); the SBOMs (C-108). Checked: two runs give the same `SHA256SUMS` on macOS and in Ubuntu 24.04 (C-109 log). **Not** claimed for the images (`apt-get` in the builder stages, layer times); their OCI archives are covered by `SHA256SUMS` and judged by D8.
- **Release notes** (A-609): facts (commit, guideline baseline, bundle, report formats, `OECM_API_VERSION`, Go and Rust toolchain, `rustc` of each library), the section of the version from `oec/CHANGELOG.md`, the section of the bundle from `schema/CHANGELOG.md` (its `Unreleased` for a `-dev` bundle), the file list. Not generated from the cycle logs: they are the project's working record, not a user's change list; the CHANGELOG is written for users.
- **No crate archives, no ESP-IDF component archive** (A-610): the crates are `publish = false` with path dependencies (`cargo package` refuses them; a crates.io release needs the PO's channel decision, D-3, and versioned dependencies); the ESP-IDF component is not self-contained (`port/esp-idf` refers to `src/` and `include/` of the SDK) – it ships in the source archive.

`release.yml` (generated, ADR-0032): on a tag `oec-v*` or by hand; `contents: read`; no caches (a release builds from clean toolchains); jobs `ffi` (matrix of the 8 targets on the runners of `sdk.yml`), `dist` (`source binaries micro schema sbom`), `images` and `release` (needs all; downloads the parts `oec-release-*` of the same run, `assemble`), uploads **one workflow artifact `oec-release`** (30 days). **Nothing is published** – no GitHub Release, no registry push, no crates.io, Go proxy, ESP registry or Arduino Library Manager (Q-011, C-104 D-2); `TestRelease` rejects `push`, `login`, `publish` and `upload` in the script.

### 7. Channels and the first public version are the PO's

What a public version number promises (D-1: 0.1.0 as an early version, or 1.0.0/2.0.0 as stable), where releases go and from which repository (D-3) and the schema module path (D-2) are decided by the PO. The weekly `supply-chain.yml` run and Dependabot come with the first release (C-108 D-3 (a)), not with `release.yml`.

## Consequences

- One number per release; the gate keeps the sources consistent; a release commit changes `VERSION`, the listed manifests and the CHANGELOG heading, and needs a released schema bundle (for 0.1.0: freeze `2.0.1` first, ADR-0019 amendment C-041).
- Every snapshot build is identifiable (commit in the version); D7/D8 reports name it.
- The images are not byte-reproducible; their reproducibility (layer timestamps, pinned package versions) is an open improvement.
- The Go SDK from a module proxy needs the prebuilt `liboec_ffi` of the same version (cgo); the release ships it per target – how a user obtains both together is part of the channel decision.

## Alternatives considered

- **Independent versions per deliverable:** each change bumps only its deliverable – rejected: untested combinations, 12 Go modules × tags, and a user cannot name "the" version.
- **CalVer:** says when, not what changed – rejected for libraries with an API promise.
- **Version from the tag only** (`git describe`, no file): the Cargo, Arduino, npm and OpenAPI manifests need a literal version – one file plus the gate test is simpler than rewriting manifests at build time.
- **SemVer build metadata for snapshots** (`0.1.0+g…`): ignored for precedence and not allowed in Docker tags – rejected for a pre-release.
- **Release notes generated from the logs or commit messages:** the logs are internal and long, the commit messages informal – rejected for a written CHANGELOG.
