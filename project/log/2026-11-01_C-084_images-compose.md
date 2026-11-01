# C-084 – P6 increment 6: images, compose profile `demos`, ACL user

- **Date:** 2026-11-01
- **Phase:** P6 – Demo services (D3, D4, D10), increment 6 of 7 (ADR-0028, A-334)
- **Baseline:** unchanged since C-074 review addendum 2. Guideline `52d8470` (branch `feature/spec-findings`, head `eb821e9`); SF-001 … 117 resolved, no open finding. 1411 inventory items, 444 REQs, 0 gaps, no REQ drafts. Schema 2.0.0 frozen, `2.0.1-dev`. 55 D7 and 17 D8 tests `implemented`, report versions 1 frozen.
- **Agent:** new session (hand-over prompt of C-083); the context was compacted during the cycle

## Intent

Build increment 6 of P6 (ADR-0028 §4, §5):

- one `Dockerfile` per demo (`oi4/oec-logger`, `oi4/oec-weather`, `oi4/oec-config`) for linux/amd64 and linux/arm64, with a pinned multi-stage build, a minimal non-root runtime and no secrets;
- an SBOM (SPDX JSON) and the license texts in `/opt/oi4/licenses` (RIG 4.1.4, SF-114), produced by a Go tool of the demos module (standard library only) and tested offline in the gate;
- the compose profile `demos` in `oec/dev` with a container storage per demo from `go run ./pki` and the UI ports on `127.0.0.1` only; the broker alone stays the default;
- the MQTT user of the config service, allowed to publish `Set/Config` and nothing else;
- opt-in runs: image builds, D8 `oec-infra image`/`container`/`run`, a compose run with D7 and a look at the three UIs.

Nothing was open for the PO and no answer arrived. **No split:** images and compose/ACL both fit into C-084. P6 closes with the exit run in C-085.

## Plan

1. Check the baseline.
2. SBOM tool first (fixtures, golden SBOM, refusals, an offline D8 run on a synthetic OCI layout), then the Debian license map and the SPDX texts.
3. Dockerfiles, `.dockerignore`, `build.sh` and their offline rules (`images_test.go`); opt-in builds with D8 `image` and `run`.
4. pki demo storages, ACL and compose profile, tests first (`oec/dev/pki`); opt-in compose run with D8 `container`, D7 `monitor`/`test`, ACL probe and `curl`.
5. Docs, gate, hand-over.

## Work

### 1. Baseline

Unchanged: guideline `52d8470` (head `eb821e9`, only `findings/` beyond the baseline), the counts above, `oec/check.sh` green at the start (C-083 end). No re-baseline.

### 2. SBOM tool (commit `6cadda9`, A-369)

`oec/demos/internal/sbom` (package) and `internal/sbom/cmd/oec-sbom` (CLI), standard library only. Inputs:

- **Rust:** the output of `cargo tree --edges normal,no-proc-macro --prefix none --format '{p}|{l}' --target <triple>` – exactly the crates linked into the target binary (A-369). Only crates.io and path packages are accepted; a git source, another registry, a missing license or version is refused.
- **Go:** `go version -m` of the binary; the main module, `stdlib` (BSD-3-Clause) and the Go SDK of this workspace (`replace ../sdk/go`, MIT). Any other module is refused (the demos have none).
- **Base image:** `/var/lib/dpkg/status.d/*` and `/usr/share/doc/<pkg>/copyright` of the runtime base, copied from it into the build stage. DEP-5 copyright files give the license per package (all `License:` fields of all `Files:` paragraphs, joined by `AND`); short names are mapped to SPDX by `images/debian.json` (D-1). A name that is not SPDX becomes `LicenseRef-<package>-<name>` with the exact text of its stand-alone `License:` paragraph. Copyright files that are not DEP-5 need an entry under `packages` (the four GCC packages). Anything else fails.

Output: `sbom.spdx.json` (SPDX 2.3; one package per component with name, version, `licenseConcluded`/`licenseDeclared`, `copyrightText` NOASSERTION, purl `pkg:cargo`/`pkg:golang`/`pkg:deb`, a `DESCRIBES`/`CONTAINS` relationship tree, `hasExtractedLicensingInfos` for every `LicenseRef`), plus `<id>.txt` for every license and exception id in the document. SPDX ids need `images/licenses/<id>.txt` (texts of SPDX license-list-data v3.29.0). Every expression is parsed; `NOASSERTION`/`NONE` are refused. JSON without HTML escaping, sorted, stable (a fixed `created` from `SOURCE_DATE_EPOCH`).

Offline tests (gate step 7):

- `TestGolden`: the fixtures under `testdata/` (cargo tree, `go version -m`, a small Debian tree with `libc6`, `libgcc-s1`, `tzdata`) give exactly `testdata/expected/sbom.spdx.json` and the text list `texts.txt` (R-1).
- `TestRules`, `TestDebianFixture`, `TestCargo`, `TestGo`, `TestExpression`, `TestWrite`: structure and parsing.
- `TestRefused`: inputs that must fail (git source, other registry, no license, foreign Go module, not DEP-5, a name without text, Debian `or …, and`, `with … exception`, a license id without text, a `LicenseRef` without text …).
- `TestDebianMapTexts`: every SPDX id of `debian.json` has its text.
- `TestD8Image`: D8 `oec-infra image --level 2` (built from `oec/conformance`, `OEC_D8`) on a synthetic OCI layout with the generator output → T8-IMG-03 pass; the same without `MIT.txt` → fail.

### 3. Dockerfiles and `build.sh` (commit `cc37587`, A-367, A-368, A-370)

Per demo `oec/demos/<demo>/Dockerfile`, build context `oec/` (allowlist `oec/.dockerignore`: `schema`, `sdk/rust`, `sdk/go`, `demos`, never `target/`, `oec/dev`, `lib/`):

- `FROM --platform=$BUILDPLATFORM rust:1.89-trixie` (and the Go toolchain copied from `golang:1.27.3-trixie` for weather and the SBOM tool) cross-compiles with the Debian cross toolchains (`gcc-<cpu>-linux-gnu`, `libc6-dev-<arch>-cross`) and `rustup target add`: no emulation, the same on an amd64 or arm64 host. Logger: `oec-ffi` release staticlib + `make` (C17, `-Werror`). Weather: `oec-ffi` + `go build` with cgo. Config: `cargo build --locked --release`. Binaries stripped. Then `oec-sbom`.
- Final stage `gcr.io/distroless/cc-debian13:nonroot`: no `RUN`; the binary in `/usr/local/bin`, `/opt/oi4/licenses` (root-owned, read only), `/etc/oi4/app` and `/opt/oi4/app` owned by `65532:65532` and declared `VOLUME`; `USER 65532:65532`; `ENV OEC_UI_LISTEN=0.0.0.0:<port>`, `EXPOSE`; OCI labels.
- All three images pinned by tag and digest (multi-platform index digests).

`images/build.sh` (opt-in): an own buildx builder (driver `docker-container`, BuildKit `v0.34.1` pinned by tag and digest), both platforms into `/tmp/…/oec-<demo>.oci.tar`, D8 `oec-infra image` on each archive at the level of the demo, then the native platform loaded as `oi4/oec-<demo>:0.1.0`. At the end the builder (with its cache) is removed, and the BuildKit image if the script pulled it. `--remove` removes the three images by name. Nothing else is touched.

`images_test.go` (offline): the three Dockerfiles (pins, `USER 65532`, no `RUN`/`ADD`/secrets in the final stage, volumes, labels, port, `--platform=$BUILDPLATFORM`), the `.dockerignore` allowlist and `build.sh` (`TestBuildScript`: pinned BuildKit, cleanup, no prune command).

### 4. Opt-in image runs (Docker Desktop 4.47.0, engine 28.4.0, linux/arm64 host)

| Image | Build (cold cache) | OCI archive (2 platforms) | Loaded (arm64) | SBOM packages | D8 `image` | D8 `run` |
|---|---|---|---|---|---|---|
| `oi4/oec-logger:0.1.0` | 87 s | 30 MB | 49.2 MB | 88 | L2: 4 pass | L2: 3 pass |
| `oi4/oec-weather:0.1.0` | 34 s | 36 MB | 55.7 MB | 90 | L3: 4 pass | L3: 3 pass |
| `oi4/oec-config:0.1.0` | 28 s | 27 MB | 45.9 MB | 87 | L1: 1 pass, 3 skip | L1: 1 pass, 2 skip |

- The config service is Level 1; T8-IMG-02 … 04 and T8-RUN-01/02 are Level 2 tests and skip (A-371). For information only, at `--level 3`: `image` 4 pass, `run` 3 pass.
- D8 `run`: the phases credentials and certificate connect (TLS 1.3, Birth of the AppId with the container name); the phase "untrusted" fails its handshake as it should (T8-RUN-01).
- SBOM content (all three): 14 Debian packages of the base (`base-files`, `ca-certificates`, `gcc-14-base`, `libc6` 2.41, `libgcc-s1`, `libgomp1`, `libssl3t64`, `libstdc++6`, `libzstd1`, `media-types`, `netbase`, `tzdata`, `tzdata-legacy`, `zlib1g`), the Rust crates of `oec-ffi` resp. `oec-config`, the Go stdlib and the Go SDK (weather), the application. 21 `LicenseRef`s, all from the base image (D-1), 22 SPDX texts.
- Rebuild after the BuildKit pin (A-370): 91/33/29 s, the same sizes, three times D8 `image` exit 0; the pinned BuildKit image and the builder were removed by the script.

### 5. Compose profile `demos` and ACL (commit `0a46ff9`, A-373 … A-375)

Tests first (`oec/dev/pki`, offline): `TestDemoStorages` (files, contents, modes of the three storages, no `etc/hostname`, no `etc/oi4/app`), `TestPasswordFile` (5 users), `TestACL` (`oec-config` may write `Set/Config` only), `TestComposeDemos` (a small parser of `compose.yaml`: profile, image, `pull_policy: never`, names, mounts read only, named volumes, ports on `127.0.0.1`, hardening, broker without profile).

- `go run ./pki` also writes `generated/roots/oec-{logger,weather,config}/` (`broker.json` with Address `mosquitto`, SecurePort 8883, MaxPacketSize 262144; `broker.pem`; `ca.pem`; `run/secrets/oi4/credentials`), the users `oec-logger`, `oec-weather`, `oec-config` in the password file, `generated/demos.env` (`OEC_<DEMO>_NAME=oec-<demo>-<uuid>`) and `roots/oec-config/etc/oec-config/config-service.json` with the AppIds of logger and weather.
- `compose.yaml`: three services in profile `demos`, images local (`pull_policy: never`), `container_name` = `hostname` from `demos.env` (fallback `oec-<demo>`), storages mounted read only, two named volumes per demo, `127.0.0.1:878x:878x`, `read_only`, `cap_drop: [ALL]`, `no-new-privileges`, `depends_on` the healthy broker, `restart: "no"`.
- `mosquitto/acl`: `user oec-config` / `topic write Oi4v2/+/+/Set/Config/#`.
- A compose `${VAR:?}` fails also without the profile, because compose interpolates the whole file – hence the fallbacks (A-373).

### 6. Opt-in compose run (copy of `oec/dev` in a temporary directory, project `oec-c084`)

`docker compose -p oec-c084 --env-file generated/demos.env --profile demos up -d --wait` on a fresh `go run ./pki` of the copy – `oec/dev/generated` stayed untouched.

- All four containers up; the three demos with read-only root, no capabilities, no new privileges, UID 65532.
- **UIs:** `curl http://127.0.0.1:8781/`, `:8782/`, `:8783/` → 200 with the page and the hardening headers; `/api/status`, `/api/state`, `/api/apps` → JSON; a foreign `Host` → 403. Weather on `simulation` (attribution `null`, no `api.met.no`).
- **Config service:** found the logger and the weather service (both by `apps` and by Birth); `POST /api/set` with logger `historySize` 500 and weather `stations` 4 → `Status` Good, the values arrived.
- **ACL** (`mosquitto_pub`/`mosquitto_sub` in the broker container): `oec-config` → `Set/Config` delivered; `Set/Data`, `Del/Config`, `Call/…` denied (not delivered). `oec-logger` → `Set/Config` denied.
- **D8 `oec-infra container`** on the three containers: T8-CTR-01/02/03 pass each (bridge network, `<name>-<uuid>` = hostname, `/opt/oi4/licenses` not overlaid).
- **D7 `monitor`**, 3 min, with the three demos restarted during the observation: 15 pass, 4 skip, 0 fail (INCONCLUSIVE because of the skips).
- **D7 `test`** against the containers (restart hook `docker restart …`):
  - logger `--level 2`: 40 pass, 15 skip, 0 fail;
  - weather `--level 3`: 42 pass, 13 skip, 0 fail;
  - config service `--level 1` with `--peer --announce` and stop/start hooks: 16 pass, 5 warn (the Level 2 SHOULD answers T7-GET-05, T7-SET-01/02/06/07, as in the gate), 34 skip, 0 fail.
  - The skips need the D7 proxy or the start of the CUT during the observation, which compose does not offer; the gate runs these tests with the proxy (A-376).
- Observed, for information: after a restart of a demo, the config UI shows Health `None` until the next heartbeat (≤ 60 s, SF-106); one `Get MAM` of the config service timed out while the logger was restarting.
- `docker compose -p oec-c084 --profile demos down -v` removed containers, volumes and the network; `build.sh --remove` the images.

### 7. READMEs

`oec/dev/README.md` (profile `demos`, storages, ACL user, generated files), `oec/demos/README.md` (images, SBOM tool, layout), `logger/`, `weather/`, `config/README.md` (section "Image").

### 8. Gate

`caffeinate -i oec/check.sh` → see "Checks (final)".

## Defects found and fixed (test first)

- **Logger, path truncation (`main.c`):** GCC 14 of the Linux build reported `-Wformat-truncation` (an error with `-Werror`) for the `snprintf` of the file paths below `<root>/opt/oi4/app`: a path longer than the buffer was silently cut. Clang on macOS does not warn. Fixed by checking the result of `snprintf`: the directory is not created and opening the file fails with `ENAMETOOLONG`, which the logger reports. To find this class of problem before a Linux build, `make gcc-check` compiles the logger with GCC 14/15 when one is installed; `oec/demos/check.sh` runs it (A-372).
- **`build.sh`, shared BuildKit tag:** the first builder (unpinned) pulled a newer default BuildKit image and moved a tag that other builders of the host use. The previous state was restored by removing the newly pulled image by ID and re-tagging the old one. `build.sh` now pins BuildKit by tag and digest and removes only what it pulled (`TestBuildScript`, A-370).

## Agent decisions (G-3.4 b, for information: REVIEW A-367 … A-376)

- **A-367 Base image `gcr.io/distroless/cc-debian13:nonroot`:**
  - glibc 2.41, libgcc, libstdc++, OpenSSL libs, CA certificates, tzdata; no shell, no package manager; user `nonroot` 65532;
  - the dpkg database (`status.d`) and DEP-5 copyright files give a complete SBOM;
  - project Apache-2.0 (GoogleContainerTools/distroless), contents Debian 13 packages (licenses in the SBOM);
  - not chosen: `debian:trixie-slim` (several times larger, shell and apt at run time), Alpine (musl; the SDK links glibc), `scratch` (no glibc), Chainguard `glibc-dynamic` (free tier only `latest`, no pinned versions over time), UBI micro (larger, Red Hat terms).
- **A-368 Builders and cross-build:** `rust:1.89-trixie` (the gate toolchain 1.89) and `golang:1.27.3-trixie` (the Go of `go.mod`), Docker official images, pinned by tag and index digest; the build stage runs on the build platform and cross-compiles with the Debian cross toolchains – no QEMU, both architectures in about the same time.
- **A-369 SBOM tool:**
  - `cargo tree` (normal edges, no proc-macro, per target) instead of the `cargo metadata` of ADR-0028 §4: `cargo metadata` lists every platform and build dependency, which are not in the image; `cargo tree` gives the linked set with its license field;
  - SPDX 2.3 JSON with purls, namespace `https://github.com/OI4/oec/spdx/<name>-<16 hex digits of a SHA-256 over the content>`, `created` from `SOURCE_DATE_EPOCH`;
  - fails closed: every license named, every text present (SF-114), otherwise no image;
  - SPDX texts committed from license-list-data v3.29.0 (CC0 data, the texts are the licenses themselves).
- **A-370 `build.sh`:**
  - OCI archives of both platforms, D8 on the archives, only the native image loaded (the classic Docker image store cannot hold an index);
  - an own builder, BuildKit pinned (`moby/buildkit:v0.34.1`, Apache-2.0) and removed if pulled; no prune of any kind;
  - default buildx provenance attestations kept (`mode=min`, in the index as `unknown/unknown`; D8 `image` ignores them).
- **A-371 Levels in D8:** each image is checked at the level of its demo (logger L2, weather L3, config L1). The image tests are Level 2, so the config service skips them; an L3 run is for information only and not pinned anywhere.
- **A-372 Optional GCC check:** `make gcc-check` with GCC 14/15 if installed (`-O2 -Werror`), in `oec/demos/check.sh`; skipped without GCC, so the gate stays the same on a host without it.
- **A-373 Compose profile:**
  - names `oec-<demo>-<uuid>` per installation from `generated/demos.env` (REQ-CNT-019/020/021), passed with `--env-file`; fallbacks instead of `${VAR:?}`, so the default profile keeps working;
  - `pull_policy: never` (local images only; compose never fetches an image of that name);
  - `read_only`, `cap_drop: [ALL]`, `security_opt: no-new-privileges`, `restart: "no"`, `depends_on` broker healthy;
  - an existing `generated/` of an earlier cycle needs `go run ./pki -force`.
- **A-374 Demo storages:**
  - `broker.json` Address `mosquitto`, SecurePort 8883 (inside the compose network);
  - `credentials` mode 0644 below `run/` 0700 (UID 65532 must read the bind mount; `run/` keeps other host users out – as the D8 run storages);
  - no `etc/hostname` (the engine sets it), no `etc/oi4/app` (named volumes);
  - `config-service.json` lists the logger and weather AppIds, so discovery does not depend on catching a Birth.
- **A-375 ACL user `oec-config`:** `topic write Oi4v2/+/+/Set/Config/#`; it may set the `Config` of every application (Mosquitto cannot bind the AppId level, as in C-071); everything else unchanged (only `oec-probe` may `Set`/`Del`/`Call` in general).
- **A-376 D7 against compose:** opt-in only and INCONCLUSIVE by design (no proxy, no start of the CUT under observation); 0 fail is the result that counts. The full D7 runs with pins stay in the gate on the test broker.

## Changes to reviewed items

None. No fixture, vector or REQ changed, no schema change. New test data with a verdict: R-1 (golden SBOM).

## Spec findings

None (next SF-118). SF-114 (SBOM) and SF-117 (both authentication means) were enough; D8 found no violation in the images.

## Dependencies

- **Base image** `gcr.io/distroless/cc-debian13:nonroot@sha256:08476c24…` – GoogleContainerTools/distroless (Apache-2.0), Debian 13 packages; their licenses are in each SBOM (D-1).
- **Builder images** (build time only, nothing of them in the final image except the cross-compiled code): `docker.io/library/rust:1.89-trixie@sha256:57407b37…` and `docker.io/library/golang:1.27.3-trixie@sha256:c65ecfe6…` – Docker official images (Rust MIT/Apache-2.0, Go BSD-3-Clause, Debian packages), with `gcc-<cpu>-linux-gnu` and `libc6-dev-<arch>-cross` from Debian (GPL with the GCC runtime exception, LGPL – build tools; only libgcc/glibc of the base image are linked at run time).
- **BuildKit** `docker.io/moby/buildkit:v0.34.1@sha256:99368f3e…` (Apache-2.0), opt-in builder only.
- **SPDX license-list-data** v3.29.0 (texts under `images/licenses/`).
- Go: the standard library only. Rust: no new crate. C: none.

## D7 and D8 unchanged

No change under `oec/conformance`, `oec/sdk`, `oec-ffi` or the Go SDK. D7 pins unchanged: logger 46 pass (L2), weather 49 pass (L3), config service 22 pass (L1). D8 is used, not changed.

## Docker objects (own only)

- **Before:** none of the project (no `oi4/oec-*` image, no `oec-c084`/`oec-d8-*` container, volume or network, no `oec-demos-*` builder).
- **Created and removed:** the images `oi4/oec-{logger,weather,config}:0.1.0` (twice); the buildx builders `oec-demos-<pid>` with their BuildKit containers and cache; the pinned BuildKit image; the base and builder images inside the builders only; the D8 `run` containers/networks `oec-d8-*` (removed by D8); the compose project `oec-c084` (4 containers, 7 volumes, 1 network; `down -v`).
- **After:** the same image ids (127), containers, volumes and networks as before, including the tag of the default BuildKit image (see Defects).

## Risks and open technical points

- **Images local only:** publication, signing and a registry are P9 (PO).
- **Base image updates:** the digests are pinned; a security update of Debian needs a new digest and a rebuild (and a new SBOM). No automatic update in P6.
- **Provenance:** the default buildx attestation is in the archives; an SBOM attestation of BuildKit is not used (the SBOM is in the image, RIG 4.1.4).
- **ACL granularity:** `oec-config` may set any `Config` (A-375).

## Commits

- `6cadda9` – SBOM generator, Debian license map, SPDX texts, `OEC_D8` in the demos gate
- `cc37587` – Dockerfiles, `.dockerignore`, `build.sh`, `images_test.go`, logger truncation fix, `gcc-check`
- `0a46ff9` – compose profile `demos`, demo storages, ACL user `oec-config`, tests, `oec/dev/README.md`
- `b03dbc8` – `build.sh` BuildKit pin and `TestBuildScript`, demo READMEs
- this commit – docs: log, ADR-0028 amendment C-084, ROADMAP (C-084 ✅), STATUS, REVIEW/REVIEW_DE

## Checks (final)

- `oec/check.sh` → **all checks passed**, rc 0 (Go SDK 179 s; demos: config 112 s, logger 46 s, weather 59 s, SBOM 1.7 s, `images_test.go` 1.3 s; `oec/dev/pki` with the compose/ACL tests; GCC check with `gcc-15`). A first run was interrupted in the Go SDK step (no result) and repeated.
- Opt-in: image builds of both platforms, D8 `image`/`run`/`container`, compose run with D7 `monitor`/`test`, ACL probe, `curl` of the UIs – all as above, 0 fail.

## For the PO

- **R-1 – golden SBOM** `oec/demos/internal/sbom/testdata/expected/sbom.spdx.json` (+ `texts.txt`), from the fixtures under `testdata/` (cargo tree, `go version -m`, Debian `libc6`, `libgcc-s1`, `tzdata`). Verdict: this is the correct SBOM for these inputs (every component with name, version, license expression, purl; every license and exception text present), and D8 T8-IMG-03 passes on it and fails without `MIT.txt`.
- **D-1 – licenses of the base image** (G-3.4 a), `oec/demos/images/debian.json`:
  - Debian short names → SPDX: `Expat` → `MIT`, `Artistic` → `Artistic-1.0-Perl`, `GPL-2` → `GPL-2.0-only`, `GPL-2+`/`GPL-3+`/`LGPL-2+`/`LGPL-2.1+`/`LGPL-3+`/`GPL-1+` → `…-or-later`, the rest identical;
  - the four GCC runtime packages (copyright file not machine-readable) → `GPL-3.0-or-later WITH GCC-exception-3.1`;
  - 21 names without an SPDX id stay `LicenseRef-<package>-<name>` with the exact Debian text: 17 in `libc6` (e.g. `LicenseRef-libc6-Inner-Net`, `…-SunPro`, `…-public-domain`), `tzdata`/`tzdata-legacy` public domain, `media-types` ad hoc, `base-files` verbatim.
  - **Proposal / default:** accept as proposed. The images are local until P9; the question decides what the image declares when it is published.
- A-367 … A-376 for information. The most visible: distroless base (A-367), SBOM via `cargo tree` (A-369), BuildKit pinned and removed (A-370), compose names with UUID and `--env-file` (A-373), the `oec-config` ACL user (A-375).

## Handover (G-3.10)

**New agent session** for C-085. C-085 is the P6 exit run (all exit criteria over the three demos, opt-in images and compose, the P6 summary) – a review of the whole phase rather than more construction, and this context was compacted twice. A fresh context that reads AGENTS.md, STATUS and this log is better. `prompt.md` (workspace root, German, not committed) holds the start prompt of C-085.

## Next

C-085 – P6 exit run: the exit criteria of ROADMAP P6 over all three demos (gate, D7 at their levels, Web UIs, images with D8, compose profile `demos` with D7 `monitor`), the P6 summary, and the plan of P7.
