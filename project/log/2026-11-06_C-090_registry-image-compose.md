# C-090 – P7 increment 5: Registry image, compose profile `registry`, D8 opt-in

- Date: 2026-11-06
- Phase: P7 (OEC Registry D5), increment 5 of 6 (ROADMAP P7 cycle cut, A-381)
- Agent session: new (handover of C-089, `prompt.md`)
- Baseline: guideline `52d8470` (branch `feature/spec-findings`, head `eb821e9`), SF-001 … 117 resolved, next SF-118; workspace head `d92deaa`

## Intent

1. **Image** of the registry as the demo images (ADR-0029 §12, ADR-0028 §4): multi-stage, linux/amd64 + arm64, non-root, `/etc/oi4/app` as volume, ports 5798/5799 (TLS with the application certificate or self-signed), SBOM and license texts – **including the npm packages of the bundle** (react, react-dom, scheduler, MIT) next to the Rust crates. The image does not rebuild `ui/dist/` (embedded, stamped); no node in the image.
2. Opt-in `images/build.sh` with D8 `image`/`run` at `--level 3`.
3. **Compose profile `registry`** in `oec/dev` (opt-in, as `demos`): ports on `127.0.0.1` only, `OEC_REGISTRY_HOSTS` fitting; README. D8 `bus` T8-BUS-03 against `oec/dev` (opt-in; inconclusive there so far). The registry lists all demos in the profile (a preview of the P7 exit in C-091).

## Plan

1. Baseline and gate; Docker before-list (own objects).
2. SBOM tool: read the npm packages of a `package-lock.json` (test first).
3. `oec/registry/Dockerfile`, `.dockerignore`, `images/build.sh`, offline `image_test.go` (test first).
4. Opt-in build with D8 `image`/`run`.
5. `oec/dev`: registry storage and certificate in `go run ./pki`, service `oec-registry` in `compose.yaml` (tests first), README.
6. `oec/conformance/live.sh` with the registry: D8 `bus` T8-BUS-03 and `host` REQ-PLT-008 against `oec/dev`.
7. Compose run with both profiles: the registry lists every demo.
8. Clean-up, after-list, docs, gate.

## Work

### 1. Baseline

Guideline `52d8470` on `feature/spec-findings` (head `eb821e9`, only `findings/` after it); SF-001 … 117 resolved, next SF-118; inventory 1411 items, 444 REQs, 0 gaps, no REQ draft; 55 D7 and 17 D8 tests `implemented`; schema 2.0.0 frozen, working state `2.0.1-dev` @ `52d8470`. `oec/check.sh` on the C-089 head: "all checks passed", rc 0. Docker 28.4 running; cbindgen 0.29.4 installed.

### 2. SBOM of the npm bundle (A-419)

`oec/demos/internal/sbom/npm.go` (`Npm`, standard library): reads `package-lock.json` v2/v3 (`packages`), takes every package below `node_modules/` that is not `dev` (the root package is the application itself), purl `pkg:npm/<name>@<version>` (scope as `%40scope`), the stated license normalized as the other kinds; refused: lockfile < 2 or without `packages`, a link, an entry outside `node_modules/`, a package without a string license, no shipped package. New component kind `npm`; `cmd/oec-sbom` gains `-npm <file>`. Tests first (`npm_test.go`): `TestNpm` (synthetic lockfile, dev and root left out, scoped purl), `TestNpmRegistryUI` (the committed lockfile of the registry = react 19.3.1, react-dom 19.3.1, scheduler 0.28.0, MIT – the set `internal/uipolicy` pins), `TestNpmOrder`, `TestNpmRefused` (each refusal). The demos do not pass `-npm`; their golden SBOM (R-1 of C-084) is unchanged.

### 3. Image (A-420)

`oec/registry/Dockerfile`, build context `oec/`:

- **Stages:** `go` (`golang:1.27.3-trixie@sha256:c65ecfe6…`, the toolchain for the SBOM tool), `base` (`gcr.io/distroless/cc-debian13:nonroot@sha256:08476c24…` of the target platform; its dpkg database feeds the SBOM), `build` (`rust:1.89-trixie@sha256:57407b37…` on `$BUILDPLATFORM`; the Debian cross toolchain of the target, `rustup target add`; `cargo build --locked --release --target <cpu>-unknown-linux-gnu`, stripped; `cargo tree` (normal edges, no proc-macro, per target) → `oec-sbom -cargo … -rust <rustc version> -npm registry/ui/package-lock.json -base /base -debian … -texts …`; license texts copied to `/out/licenses`), final stage on `base` **without `RUN`**: `/usr/local/bin/oec-registry`, `/opt/oi4/licenses` (read only), `/etc/oi4/app` and `/opt/oi4/app` owned by 65532, `VOLUME`, `USER 65532:65532`, `EXPOSE 5798 5799`, `OEC_REGISTRY_WEB=0.0.0.0:5798`, `OEC_REGISTRY_API=0.0.0.0:5799`, OCI labels. The bases are those of the demos (ADR-0028 amendment C-084), pinned by tag and digest.
- **No node:** the image embeds `ui/dist/` as committed through `src/ui_assets.rs` (`include_bytes!`); `ui/build.stamp` and the policy are checked in the gate, the image build does not touch them.
- **`oec/.dockerignore`:** allowlist extended by the registry crate (`Cargo.toml`, `Cargo.lock`, `src/`), `api/openapi.json`, `api/asyncapi.json`, `ui/dist/`, `ui/package-lock.json`; `**/node_modules` and `**/target` excluded.
- **`images/build.sh`** (opt-in, as `oec/demos/images/build.sh`): own buildx builder `oec-registry-<pid>` (driver docker-container, BuildKit `moby/buildkit:v0.34.1` pinned by tag and digest), both platforms into one OCI archive, D8 `oec-infra image --level 3`, load of the native platform as `oi4/oec-registry:0.1.0` (replacing an earlier image by ID), D8 `oec-infra run --level 3`; a verdict other than pass exits 1; `--remove` removes the image; builder and pulled BuildKit image removed; nothing pruned.
- **`image_test.go`** (gate, offline; tests first): `TestDockerfile` (pinned images = the demos', final stage without `RUN`, user, volumes, ports, env, no `node`/`npm` anywhere, SBOM with `-npm`), `TestDockerignore`, `TestBuildScript` (levels, clean-up, no prune).

**Opt-in result** (`images/build.sh /tmp/oec-reg-img`):

| | Value |
|---|---|
| Build, both platforms | 119 s (cargo release ~45 s arm64 native, ~41 s cross to amd64) |
| OCI archive | 32 MB (33,085,440 B); index `sha256:afedc252…`, amd64 `sha256:1d5a7e1c…`, arm64 `sha256:011baed7…` |
| D8 `image --level 3` | T8-IMG-01 … 04 pass, exit 0 |
| Loaded `oi4/oec-registry:0.1.0` (arm64) | 52,041,185 B, image `sha256:1428179b…` |
| D8 `run --level 3` | T8-RUN-01, 02, 03 pass, exit 0 – credentials phase TLS 1.3, 6 PUBLISH, largest payload 6611 octets; certificate phase client certificate verified; untrusted phase TLS handshake refused (unknown CA) |
| Image config | User `65532:65532`, ports 5798/5799, volumes `/etc/oi4/app`, `/opt/oi4/app` |

**SBOM** (SPDX 2.3, `/opt/oi4/licenses/sbom.spdx.json`): 145 packages – cargo 124, Debian 14 (base-files, ca-certificates, gcc-14-base, libc6, libgcc-s1, libgomp1, libssl3t64, libstdc++6, libzstd1, media-types, netbase, tzdata, tzdata-legacy, zlib1g), generic 4 (`oec-registry`, `oec`, `oec-core` MIT; `rust-std` 1.89.0 MIT OR Apache-2.0), **npm 3** (react 19.3.1, react-dom 19.3.1, scheduler 0.28.0 – MIT, `pkg:npm` purls); 42 license texts. Licenses of the cargo and npm components: MIT OR Apache-2.0 70, MIT 24, Apache-2.0 OR MIT 20, Unlicense OR MIT 2, ISC 2, BSD-3-Clause 2, MIT-0 1, Zlib 1, Apache-2.0 1 and a few compound expressions (ring, aws-lc style) – all within ADR-0023; no new text was needed for the npm three.

Demo images rebuilt for the compose run (unchanged code): D8 `image` 4 pass and `run` 3 pass at L3 each – logger 89 s / 30 MB / 49,249,595 B, weather 37 s / 36 MB / 55,729,080 B, config 32 s / 27 MB / 45,969,917 B (build / OCI archive / loaded).

### 4. Compose profile `registry` (A-416, A-417)

- **`go run ./pki`:** `generated/roots/oec-registry/` – `broker.json` (Address `mosquitto`, SecurePort **8884**, the certificate listener), `etc/oi4/certs/<name>.pem` (application certificate of the dev CA: CN = SAN = container name `oec-registry-<uuid>`, extra SANs `oec-registry`, `localhost`, `127.0.0.1`, `::1`, EKU client and server), `run/secrets/oi4/private_key.pem` (encrypted PKCS#8) and `passphrase` (file modes as the demos); `generated/demos.env` gains `OEC_REGISTRY_NAME`. `newLeaf` takes the key usages; the storage writer is shared with the demos. Tests first: `TestRegistryStorage` (certificate, SANs, EKU, key decrypts with the passphrase, `broker.json`), `TestComposeRegistry` (profile, local image `pull_policy: never`, container name = hostname from the env file, ports `127.0.0.1:5798`/`5799`, `OEC_REGISTRY_HOSTS`, read-only, `cap_drop: [ALL]`, `no-new-privileges`, named volumes, storage mounts read only).
- **`compose.yaml`:** service `oec-registry` in the profile `registry`; `OEC_REGISTRY_HOSTS=oec-registry` (the service name) – the container name is added by the registry itself; TLS on with the application certificate. No new ACL line: the broker takes the CN as user name, the pattern lines allow reading and `Pub`/`Get`.
- **Checks:** `go test ./pki` ok; `docker compose config -q` ok without profile, with `demos`, with `demos` + `registry`.

### 5. D8 against `oec/dev` (A-418)

`oec/conformance/live.sh` with `OEC_LIVE_REGISTRY=1` (requires the local image and a registry storage; `go run ./pki -force` for an older `generated/`) starts the profile `registry` **after** `TestMonitorDevBroker` (the registry's `Get`s would change its recording); `compose down -v` covers the profile. `live_test.go`: `TestBusDevBroker` on 8883 expects T8-BUS-03 pass via discovery on `Oi4v2/Registry/+/Pub/#` with the registry IL from `demos.env`; `TestHostDevRoots` waits 65 s (one heartbeat) and expects REQ-PLT-008 pass.

Result (`OEC_LIVE_REGISTRY=1 live.sh`, rc 0, 5 min 25 s):

- `TestMonitorDevBroker` pass.
- `TestBusDevBroker/8883`: T8-BUS-01 pass, T8-BUS-02 warn (REQ-RES-CONFIG-026 – the known ACL limitation of the dev broker, A-258), **T8-BUS-03 pass** (REQ-REG-001, REQ-REG-002), exit 0; the 8884 run pass.
- `TestHostDevRoots`: REQ-PLT-008 **pass** (the registry is found); T8-HST-04 fails only on REQ-PLT-003 (macOS is not Linux – as before).
- `TestHostDevContainer`, `TestContainerDev`, `TestRunDev` pass.
- **Cause of the earlier "inconclusive":** there was no OEC Registry on the bus before P7 – nothing else; with the registry, discovery finds it and the probes pass.

### 6. Compose run with both profiles (A-421)

First run of `--profile demos --profile registry up -d --wait`: the registry listed **5 of 6** demo assets (logger, weather service, three stations) – **the config service was missing**. After its first heartbeat (60 s) it appeared online, without MAM, `available: false`, observed level 0.

Analysis: a bus capture (probe user, `mosquitto_sub` in the broker container) shows the registry's `Get/MAM` to the config service unanswered, while the logger and the weather service answered theirs; a probe `Get MAM` 3 s after the start of the config service was not answered either. The config service is **Messaging Level 1** and answers no `Get` (REQ-CMP-017 applies from Level 2; documented in `oec/demos/config/config_test.go`); its MAM is published only with its Birth. An asset is available only after its Birth (MG A1). The registry had subscribed after that Birth. **The guideline is consistent and the registry conform – no SF**; it is the start order of the deployment.

Fix (test first): `TestComposeDemos` requires every demo to `depends_on` `oec-registry` with `condition: service_started` and `required: false`; `compose.yaml` adds it (plus a header comment). `required: false` keeps `--profile demos` alone working. Result: three of three cycles `down -v` → `up -d --wait` with both profiles – **6 assets, all available** (config service L1; logger, weather service and the three stations observed L3), 0 ignored; `--profile demos` alone still starts. Headless Chrome (`--timeout=4000`) on `https://localhost:5798/` shows all six. A registry started after a Level 1 application lists it after the application's next Birth (`docker restart`).

Side observations, left unchanged (A-422): the consumer presence directory of the SDK core (`oec-core/src/consumer/directory.rs`) treats the application's own heartbeat as a new application, so the config service sends `Get/MAM` to its own AppId (unanswered at L1) – harmless; the registry repeats "N of M assets available" at INFO with heartbeats – cosmetic.

### 7. Docs

`oec/registry/README.md` (state, section *Image*, layout, checks), `oec/dev/README.md` (section *Compose profile `registry` (C-090)*, file table, generated tree, start order, live run), `oec/README.md`, `oec/docs/docs/demos/index.md` (*With the OEC Registry*), ADR-0029 amendment C-090, ADR-0028 amendment C-090 (SBOM `-npm`), STATUS, ROADMAP, REVIEW, REVIEW_DE.

## Agent decisions (G-3.4 b, for information: REVIEW A-416 … A-423)

A-416 client certificate of the dev CA for the registry, HTTPS with it, both means of authentication across the profiles; A-417 ports on `127.0.0.1`, `OEC_REGISTRY_HOSTS=oec-registry`, `OEC_REGISTRY_NAME` in `demos.env`; A-418 `live.sh` `OEC_LIVE_REGISTRY` after the monitor test; A-419 SBOM tool `-npm` (shipped packages only); A-420 Dockerfile on the demos' bases, stamped UI embedded, no node, `images/build.sh`; A-421 demos start after the registry (Level 1 config service), no SF; A-422 observations left unchanged; A-423 SBOM extraction with `docker rm -v`.

## Test data for review (G-3.9)

- None. No D7/D8 fixture, scenario or verdict was added or changed; the npm test lockfile is input data of a unit test without a verdict of the guideline.

## Changes to reviewed items

- None. The golden SBOM of the demos (R-1 of C-084) is unchanged.

## Spec findings

- None. The missing config service (§6) follows from MG A1 and REQ-CMP-017 as written; the deployment orders the start.

## Dependencies

- Go: standard library only. Rust: none new (ADR-0023 unchanged; the registry crates as in C-088). npm: none new (the image reads the committed lockfile). C: none.
- Base and builder images: those of the demos (no new image): `rust:1.89-trixie@sha256:57407b37…`, `golang:1.27.3-trixie@sha256:c65ecfe6…`, `gcr.io/distroless/cc-debian13:nonroot@sha256:08476c24…`; opt-in BuildKit `moby/buildkit:v0.34.1` (Apache-2.0), pulled and removed by `build.sh`.

## D7, SDKs, demos

- Unchanged: D7, D8, Rust SDK, C ABI, Go SDK, demo code (pins 46/49/22), HTTP API, OpenAPI, Web UI (stamp unchanged). The SBOM tool of the demos module gained the additive `-npm` (ADR-0028 amendment C-090). Report schemas frozen, `COVERAGE.md` unchanged. A-113 untouched.

## Docker objects (own only)

- **Before** (`docker image ls`/`ps -a`/`volume ls`/`network ls`/`buildx ls`, full list in the session): no own object – no `oi4/oec-*` image, no `oec-dev*` container, volume or network, no `oec-*` builder. Objects of other projects were not touched.
- **Created and removed in C-090:** images `oi4/oec-registry:0.1.0`, `oi4/oec-logger:0.1.0`, `oi4/oec-weather:0.1.0`, `oi4/oec-config:0.1.0` (removed with `build.sh --remove`); builders `oec-registry-<pid>`, `oec-demos-<pid>` and BuildKit `moby/buildkit:v0.34.1` (removed by the scripts); D8 sandbox containers `oec-d8-run-…` (removed by D8); compose project `oec-dev` – containers, network `oec-dev_default`, volumes `oec-dev_data`, `oec-dev_oec-*-etc`/`-opt` incl. `oec-registry-etc`/`-opt` (`down -v`); live-test containers (`rm -v`); a container of the SBOM extraction – removed first without `-v`, its **two anonymous volumes** (owner 65532) were found in the after-list and removed (A-423).
- **After:** identical to the before-list (only the uptime of foreign containers differs). No prune.

## Commits

- `ced8a99` SBOM tool: `oec/demos/internal/sbom/{npm.go,npm_test.go,sbom.go,cmd/oec-sbom/main.go,testdata/npm/package-lock.json}`.
- `14d979e` Image: `oec/registry/Dockerfile`, `oec/registry/images/build.sh`, `oec/registry/image_test.go`, `oec/.dockerignore`.
- `e22eb1d` Profile: `oec/dev/pki/{generate.go,generate_test.go,config_test.go}`, `oec/dev/compose.yaml`, `oec/dev/README.md`.
- `cc0b60c` Live run: `oec/conformance/live.sh`, `oec/conformance/cmd/oec-infra/live_test.go`.
- This commit: ADR-0029 and ADR-0028 amendments C-090, this log, REVIEW, REVIEW_DE, STATUS, ROADMAP, `oec/README.md`, `oec/registry/README.md`, `oec/docs/docs/demos/index.md`.

## Checks (final)

- `caffeinate -i oec/check.sh` → **all checks passed**, rc 0 (~11 min, load 3.7 … 9.2, no flake, no rerun). Step 6 `sdk/go` ok (180.6 s); step 7 demos: `demos` 1.3 s, `config` 114.0 s, `internal/sbom` 1.7 s (incl. the `-npm` reader of A-419), `logger` 44.5 s, `weather` 59.7 s – D7 pins logger L2 46, weather L3 49, config L1 22 unchanged; step 8 registry: `registry` 49.3 s (D7 L3 49 pass, D8 T8-BUS-03, HTTP/SSE/TLS, Web UI, `image_test.go` Dockerfile/build.sh/SBOM policy), `internal/regtest` 1.3 s, `internal/uipolicy` 5.6 s (stamp unchanged).
- After the gate only documentation changed (log, STATUS, ROADMAP, REVIEW, REVIEW_DE, ADR-0028/0029 amendments, READMEs, demo guide page).

## For the PO

- Nothing to review (no R-x), nothing to decide (no D-x), no SF.
- For information: A-416 … A-423 – especially A-421 (the demos start after the registry, because a Level 1 application answers no `Get MAM` and the registry must see its Birth).

## Handover (G-3.10)

**A new session is recommended** for C-091: this session's context was compacted. `prompt.md` (workspace root, German, not committed) holds the start prompt of C-091.

## Next

C-091 – P7 exit run (ROADMAP P7 exit): the registry image with D8 `image`, `container` and `run` at `level3`; a compose run of both profiles in `oec/dev` in which the registry lists every participant with MAM, Health, availability, Profile, observed level and conformity; D8 `bus`/`host` against `oec/dev`; the Registry guide in `oec/docs` (input for P8); then the PO decides on closing P7.
