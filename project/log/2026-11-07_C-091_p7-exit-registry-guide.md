# C-091 – P7 exit run: the registry lists every participant; Registry guide

- Date: 2026-11-07
- Phase: P7 (OEC Registry D5), increment 6 of 6 (ROADMAP P7 cycle cut, A-381) = exit run
- Agent session: new (handover of C-090, `prompt.md`)
- Baseline: guideline `52d8470` (branch `feature/spec-findings`, head `eb821e9`), SF-001 … 117 resolved, next SF-118; workspace head `9f151fd`

## Intent

1. **Image:** `oec/registry/images/build.sh` again (opt-in), D8 `image` and `run` at `--level 3`; in addition D8 **`container`** at `--level 3` against the running registry container of the compose profile (exit (2)).
2. **Compose run** in `oec/dev` with `--profile demos --profile registry` (opt-in): the registry lists **every participant** with MAM, Health, availability, Profile, observed level and conformity – over REST and in the Web UI; a `Set` of the config service appears in the registry; `docker restart` and `docker kill` of a demo show Close and Will; D7 `monitor` against the run; D8 `bus` T8-BUS-03 and `host` REQ-PLT-008 against `oec/dev` (`OEC_LIVE_REGISTRY=1 live.sh`).
3. **Exit assessment** (1) … (8) with evidence; D-1 to the PO: close P7?
4. **Docs page** "OEC Registry" under `oec/docs/docs/` (P8 input), like the demo pages (A-379).

## Plan

1. Baseline gate; Docker before-list (own objects).
2. Docs page (independent of the runs; checked against the run afterwards).
3. Registry and demo images with D8 `image`/`run`.
4. Compose run in a copy of `oec/dev` (A-424): REST, Web UI, `Set`, restart, kill, registry restart, D8 `container`, D8 `bus`, D7 `monitor`.
5. Compose down, `live.sh` against the real `oec/dev`, images removed, after-list.
6. Log, amendments, READMEs, project files, gate, commits.

## Work

### 1. Baseline

Guideline `52d8470` on `feature/spec-findings` (head `eb821e9`, only `findings/` after it); SF-001 … 117 resolved, next SF-118; inventory 1411 items, 444 REQs (27 area files), 158 `schema`/`d7-validate` REQs with 0 gaps, no REQ draft; 55 D7 and 17 D8 tests `implemented`; schema 2.0.0 frozen, working state `2.0.1-dev` @ `52d8470`; crosscheck 636 cases, 0 disagreements. `caffeinate -i oec/check.sh` on `9f151fd`: **all checks passed**, rc 0 (demos and registry steps ok; registry 48.4 s, `internal/regtest` 1.4 s, `internal/uipolicy` 6.2 s). Docker 28.4 running; cbindgen 0.29.4 installed.

### 2. Images (exit (2), part 1)

Code unchanged since C-090; the images were built again from `9f151fd`.

| Image | Build (both platforms) | OCI archive | Loaded (arm64) | D8 `image --level 3` | D8 `run --level 3` |
|---|---|---|---|---|---|
| `oi4/oec-registry:0.1.0` | 122 s | 32 MB | 52,041,185 B | T8-IMG-01 … 04 pass | T8-RUN-01 … 03 pass |
| `oi4/oec-logger:0.1.0` | 99 s | 30 MB | 49,249,595 B | 4 pass | 3 pass |
| `oi4/oec-weather:0.1.0` | 41 s | 36 MB | 55,729,080 B | 4 pass | 3 pass |
| `oi4/oec-config:0.1.0` | 35 s | 27 MB | 45,969,917 B | 4 pass | 3 pass |

The registry image size equals C-090 to the byte (reproducible from the same sources and pinned bases).

### 3. Compose run with both profiles (exit (2) part 2, (3), (4))

A copy of `oec/dev` (`git ls-files` into a temporary directory, compose project `oec-c091`, own `generated/` from `go run ./pki`; A-424), then `docker compose -p oec-c091 --env-file generated/demos.env --profile demos --profile registry up -d --wait`: broker, logger, weather service (simulation, three stations), config service and registry healthy in 3 s.

**REST** (`curl --cacert generated/broker/ca.pem https://localhost:5799/api/assets`, ~60 s after the start): registry 0.1.0, limits Event trail 200 / Health history 20 / violation trail 10, 0 ignored, **6 assets, all online and available**, each with MAM and Health `NORMAL_0`/100:

| Asset | Kind, ServiceType | Observed level | Conformity (messages / violations) |
|---|---|---|---|
| OEC Config | application, `Utility` | L1 (missing for L2: REQ-CMP-015, REQ-CMP-017 – it is a Level 1 application) | 2 / 0 |
| OEC Logger | application, `Utility` | L3 | 12 / 0 |
| OEC Weather | application, `OTConnector`, 3 devices | L3 | 23 / 0 |
| OEC Weather Station 1 … 3 | devices, published by OEC Weather | L3 | – (counted at the publisher) |

The detail (`/api/assets/{il}`) of each application has the Profile, the answers to `Get` MAM/Health/Profile/PublicationList/SubscriptionList (L1: only what it publishes on its own), PublicationList and SubscriptionList, the Health history and no violation. The registry does not list itself (it ignores its own messages, ADR-0029); its `ignored` counter counts its own heartbeats and its answers to its own `Get`s (A-422, unchanged).

**Web UI** (`https://localhost:5798/`, headless Chrome through the DevTools protocol, A-426):

- *List view* (1600 × 1000): header "OEC Registry", the registry IL and v0.1.0, "3 applications · 3 devices · 6 online · 0 ignored", badge *Live*. Table *Applications and devices* with the columns Name/IL, ServiceType, Availability, Health, Level, Conformity, Last seen; OEC Config (Utility, Online, Normal 100, L1), OEC Logger (Utility, L3), OEC Weather (OTConnector, L3) and the three stations indented below the weather service (Device, L3, conformity "–"). Below: *Events* with the Frost event of a station "via OEC Weather", a Pause button and "n of at most 200 kept". Footer: "Read-only. API: /api/assets · /api/events · openapi.json · asyncapi.json".
- *Detail of the logger* (`#/asset/<IL>`): badges Application/Online/Normal 100/L3; *Availability* (Online, available "yes – online with MAM and Health", online since, first seen, last message); *MAM* (all elements); *Health* and a *Health history* line chart (Normal); *Observed Messaging level* "informative, not a verdict" – L3, nothing missing; *Schema conformity* n messages, 0 violations; *Profile and answers* "the registry reads with Get only" – Profile chips and Get MAM/Health/Profile/PublicationList/SubscriptionList *answered*; tables *PublicationList* and *SubscriptionList*; *Events* of the asset.
- *Detail of the weather service:* as above plus the PublicationList with the ids of the application and of the three stations and the section *Devices* with links to the stations.
- *Detail of station 1* (after the run of §5): Device/Online/Normal 100/L3, *Published by* OEC Weather, MAM with `HardwareRevision` "undefined" (the value REQ-RES-MAM-008 prescribes for an unknown one), Health history, observed L3, Get MAM/Health/Profile *answered*, "No Event received". The list shows the two D8 probes of §6 as well: `oec-infra` … `-1` *Offline* (Close) and `-2` *Offline (lost)* with *Failure 0* (its Will), header "5 applications · 3 devices · 6 online".

**`Set` through the config service** (`POST 127.0.0.1:8783/api/set`; the config service discovers the registry and its writable `Config` `registry`, DataSetWriterId 27):

| Target | Value | Status `Event` | In the registry |
|---|---|---|---|
| logger `common.historySize` | 2000 | `Good` | Event trail: Status/Good of OEC Logger |
| registry `directory.EventTrail` | 300 | `Good` | `/api/assets` limits `eventTrail` 300; the UI shows "of at most 300 kept" |
| registry `directory.GetTimeout` | 99 | `BadOutOfRange` (Validation max 60) | value unchanged |
| weather `common.frostThreshold` (§5) | 5 | `Good` | Status/Good of OEC Weather, then "Frost begins at Oslo" with `FrostThreshold` 5 |
| logger `common.data` (§5) | true | `Good` | Status/Good of OEC Logger |

The registry's own Status events do not appear in its trail (own messages ignored). The README said that values out of range "are clamped" – a `Set` is refused with `BadOutOfRange` by the SDK; the worker clamps what it reads. README corrected (A-428).

**Availability – `docker restart` of the logger** (SSE `/api/stream` recorded): `asset` message *offlineGraceful*, `available: false` (its Close) at restart + 0 s, *online*, `available: true` (Birth, MAM and Health) 1 s later.

**Availability – `docker kill` of the weather service:** the weather service *offlineUngraceful* with Health `FAILURE_1`/0 (its Will) and its three stations *offlineUngraceful*, `available: false` (they are published by it) within the second; after `docker start` all four *online* and available after 1 s, followed by a Frost event.

**Registry restart** (`docker restart` of the registry): after 5 s only the config service is known (its heartbeat) – online, **no MAM, not available**; after 70 s logger, weather service and stations are back (heartbeat → `Get`); the config service stays not available until its own restart (Birth), then available with MAM. This is the documented Level 1 limit of A-421 (no SF). The `Config` of the registry survived the restart (`eventTrail` 300 – persisted in `/etc/oi4/app`).

**D8 `container --level 3`** (`oec-infra container` with the engine socket): the registry container – T8-CTR-01, 02, 03 **pass** (bridge network `oec-c091_default`, hostname = container name, `/etc/oi4/certs`, `/etc/oi4/mqtt`, `/run/secrets/oi4` read only, `/etc/oi4/app` and `/opt/oi4/app` volumes, not shared); the three demo containers together: 3 pass.

### 4. D8 `bus` against the compose run (exit (3))

`oec-infra bus --bus-storage generated/roots/oec-dev-app --username oec-probe --password-file generated/probe/password --credentials-2 generated/roots/oec-dev-app/run/secrets/oi4/credentials --registry-wait 70s`, rc 0: T8-BUS-01 pass, T8-BUS-02 warn REQ-RES-CONFIG-026 (dev ACL, A-258), **T8-BUS-03 pass** (REQ-REG-001, REQ-REG-002; the registry of the profile found on `Oi4v2/Registry/+/Pub/#`).

### 5. D7 `monitor` (exit (3), A-425)

**Window 1** (8 min, certificate on 8884, recorded): 10 pass, 1 warn, 2 fail, 6 skip, exit 1. Every finding is on the probe AppIds of D8 `bus` (`Utility/oi4.org/oec-infra/D8/d550d01edeb1[-1|-2]`), which ran inside the window and sends **deliberately invalid** messages (10 invalid: T7-MON-01 REQ-TOP-003/013, REQ-MSG-001/002/003/016/032, REQ-REFDES-006, REQ-ACT-001/002/003), publishes with QoS 1 (T7-MON-12 warn) and has no Health heartbeat (T7-MON-05 "no Health about itself during 8m0s"). The participants: weather service 53 messages / 0 invalid, registry 64 / 0 (2 Births, 1 Close – its restart), config service 43 / 0, logger 34 / 0 (2 Births, 1 Close – its restart).
**Replay of window 1 without the probe topics** (47 of 225 lines removed – the probe's messages and the registry's `Get`s to it; trailer count adjusted): **14 pass, 1 info, 4 skip, 0 fail**, INCONCLUSIVE (exit 4). Info T7-MON-03: the Will of the killed weather service is seen without a Birth in the window – as intended.
**Window 2** (4 min, without D8, with `docker restart` of the weather service and two `Set`s): **13 pass, 6 skip, 0 fail**, INCONCLUSIVE (exit 4) – weather service 46 / 0 (1 Birth, 1 Close), registry 7 / 0, config service 11 / 0, logger 10 / 0. The skips are the expected ones of a short passive window (as C-084/C-085, A-376).

The registry's own traffic in both windows: `Pub` of its MAM, Health, Profile, Config, Event (its Status events of the `Set`s), PublicationList and SubscriptionList, and towards others **only `Get`** (MAM, Health, Profile, PublicationList, SubscriptionList) – no `Set`, `Del` or `Call` (exit (4)).

### 6. `live.sh` with the registry (exit (3))

After `down -v` of the compose copy: `OEC_LIVE_REGISTRY=1 caffeinate -i oec/conformance/live.sh` against the real `oec/dev` (its `generated/` of C-090 has the registry root), rc 0 in 5 min 26 s, **"live: all tests passed"**: `TestMonitorDevBroker` pass; `TestBusDevBroker/8883` T8-BUS-01 pass, T8-BUS-02 warn REQ-RES-CONFIG-026, **T8-BUS-03 pass** (REQ-REG-001/002), 8884 pass; `TestHostDevRoots` **REQ-PLT-008 pass** (the registry is found), T8-HST-04 fails only on REQ-PLT-003 (macOS is not Linux – as in C-090); `TestHostDevContainer`, `TestContainerDev`, `TestRunDev` pass. Identical to C-090.

### 7. Docs page (A-379 pattern)

`oec/docs/docs/registry/index.md` (new, sidebar position 2 after the demos): purpose and an overview table (ServiceType, levels, image, Web UI, REST); *What it shows* (availability rules, MAM/Health, Profile and lists, observed level and conformity as informative, Event trail, devices under their publisher, tools listed too, not itself); *Start* with the compose profiles, as an image (container storage, user, volumes, SBOM) and locally (`OEC_ROOT`); *Settings* (Config `directory` on the bus, environment at the start); *Web UI*; *REST and event stream* (four requests, `curl` example, OpenAPI/AsyncAPI, `405`); *Ports and TLS*; *Levels and checks*; *Limits* (read-only, no authentication, start order with Level 1 participants, in-memory directory, v2 only); *Details* links to `oec/registry/README.md` and `oec/dev/README.md`. Tables of the READMEs are not repeated. `oec/docs/docs/demos/index.md` links to it. Links and anchors checked with a small script (all resolve).

## Exit assessment (ROADMAP P7, criteria (1) … (8))

| # | Criterion | Result | Evidence |
|---|---|---|---|
| 1 | D7 `test --level 3` in the gate (exit 0, no fail, pinned) and D8 `bus` T8-BUS-03 in the gate | **met** | `oec/registry/registry_test.go` `TestD7Level3` (49 pinned passes, 0 fail) and `TestD8Registry` (T8-BUS-03 pass) – gate step 8 green in the baseline and the final gate |
| 2 | the image passes D8 `image`, `container` and `run` at `level3` (opt-in, log) | **met** | §2: `image` T8-IMG-01 … 04 pass, `run` T8-RUN-01 … 03 pass; §3: `container` T8-CTR-01 … 03 pass on the running registry container |
| 3 | compose run in `oec/dev`: the registry lists every participant with MAM, Health, availability, Profile, observed level and conformity in the UI and over REST; T8-BUS-03 against `oec/dev` | **met** | §3: all six demo assets (and the D8 probes) over REST and in the Web UI with every named item; `Set`, Close (restart) and Will (kill) visible; §4/§6: T8-BUS-03 pass against the compose run and against `oec/dev` (`live.sh`); §5: D7 `monitor` 0 fail without the deliberately invalid D8 probe traffic |
| 4 | read-only on the wire (tested) and over HTTP (`GET` only) | **met** | gate: `src/bus.rs` test `read_only` (no `set`/`del`/`call`/`new_data_set_writer_id` in the sources, only `GET` on the HTTP side), `src/api_tests.rs` `asyncapi_operations_are_read_only`, `internal/regtest/http.go` (every method but `GET` → `405`, run by `http_test.go`); §5: on the bus the registry sent only `Get` towards others |
| 5 | `openapi.json`/`asyncapi.json` valid against their meta-schemas and consistent with the code, in the gate | **met** | `src/api_tests.rs`: `openapi_is_valid`, `openapi_paths_are_the_router`, `openapi_schemas_are_the_views`, `asyncapi_is_valid`, `asyncapi_channels_are_the_interface` (vendored meta-schemas, ADR-0029 §8) |
| 6 | npm policy holds (exact pins, lockfile, license allowlist, offline gate); README and docs pages | **met** | `internal/uipolicy` in the gate (stamp unchanged since C-089); `oec/registry/README.md` (state, Config wording A-428), `oec/docs/docs/registry/index.md` (new) |
| 7 | no new dependency outside the policies; D7/D8/SDKs only defect fixes or additive with an amendment | **met** | C-091: no dependency, no code change (only docs); over P7: SDK "listed assets" and raw subscriptions additive (ADR-0026 §22/§23), D7 refinement for registries (ADR-0025 amendment C-087), D8 T8-BUS-03 defect fixes (C-086), SBOM `-npm` additive (ADR-0028 amendment C-090); Rust crates within ADR-0023 (amendment C-086: `hyper` 1 stack), npm per ADR-0029 §10 |
| 8 | `oec/check.sh` green | **met** | final gate below |

**Deviations (none blocking):**

- D7 `monitor` window 1 failed because D8 `bus` ran inside it – the fails are on the D8 probe AppIds; the replay without them and a clean second window have 0 fail (A-425). Not a defect of a participant.
- The Level 1 config service becomes available only after a Birth seen by the registry (A-421); after a registry restart it needs its own restart. Documented in the docs page and in `oec/dev/README.md`; follows from MG A1 and REQ-CMP-017 (no SF).
- `live.sh`: T8-HST-04 fails REQ-PLT-003 on macOS (not a Linux host) – the dev environment is not a Service Container platform (as recorded in the P5b exit, A-298).

**Proposal D-1 (G-3.4 a, phase order):** (a) **close P7**; next phase **P8 – Documentation** (the Docusaurus site around `oec/docs/docs/`: build, Getting started, concepts, SDK/tool guides, schema reference, migration notes; exit "docs build without broken links; a newcomer can run the demos by following the docs only"), then P9. Alternatives: (b) close P7, next P9 (hardening and release) with P8 alongside; (c) keep P7 open for a named gap.

**Carried, if P7 is closed:**

- → **P9:** publication and signing of the registry image (and the demo images), an image registry, CI (gate, image builds, D7/D8 runs on Linux/macOS/Windows runners), live runs on Podman and on a Windows engine, release notes/versioning of `oec-registry`, CRA review before any public release.
- → **P8:** the Docusaurus site around the docs pages (registry and demos), build and link check.
- **Later step of the registry (after the release scope, PO decision):** registry step 2 (writing: `Set`/`Del`/`Call` from the registry, ADR-0008), authentication of the HTTP API, persistence of the directory, expiry of offline entries.
- Unchanged and open for information: A-113, A-422.

## Defects

- None in code. One documentation imprecision fixed: `oec/registry/README.md` *Config `registry`* (A-428).

## Agent decisions (G-3.4 b, for information: REVIEW A-424 … A-428)

- **A-424** – Exit run in a copy of `oec/dev` (temporary directory, compose project `oec-c091`, own `generated/`): the working `oec/dev/generated/` stays as it is for `live.sh`, and the run's objects carry their own project name.
- **A-425** – D7 `monitor` overlapping D8 `bus`: the D8 probes send invalid messages on purpose; judged by a replay without the probe topics plus a second clean window. No tool change; a note in `oec/conformance/README.md` and `oec/dev/README.md` (do not overlap).
- **A-426** – Web UI screenshots through the Chrome DevTools protocol (session script, not committed). Headless Chrome with `--timeout`/`--dump-dom` often stops before the detail view's second fetch (after the first summary change) completes; in a normal page the detail is complete after ~1.1 s (network log). Not a registry defect; nothing changed.
- **A-427** – The directory lists every participant it saw, including short-lived tools (the D8 probes stay *offline* until the registry restarts), and never the registry itself. Unchanged for step 1 (in-memory directory, ADR-0029); stated in the docs page; expiry of offline entries is a candidate for a later step.
- **A-428** – `oec/registry/README.md`: a `Set` out of range is refused with `BadOutOfRange` (SDK validation of the Config definition); the worker's clamp is a second line of defence. Documentation only.

## Test data for review (G-3.9)

- None. No D7/D8 fixture, scenario, golden file or verdict was added or changed.

## Changes to reviewed items

- None.

## Spec findings

- None. Every observation of the run follows from the guideline as written (A-421 Level 1 start order, MG A1, REQ-CMP-017; REQ-RES-MAM-008 "undefined").

## Dependencies

- None new (Go standard library, Rust ADR-0023, npm ADR-0029 §10 unchanged). Opt-in tools only: Docker, BuildKit `moby/buildkit:v0.34.1` (pulled and removed by `build.sh`), Chrome and node of the machine for the screenshots.

## D7, D8, SDKs, demos, registry code

- Unchanged: D7, D8, Rust SDK, C ABI, Go SDK, demos (pins 46/49/22), registry code, HTTP API, OpenAPI/AsyncAPI, Web UI (stamp unchanged). Report schemas frozen, `COVERAGE.md` unchanged. A-113 and A-422 untouched.

## Docker objects (own only)

- **Before** (`docker image ls`, `ps -a`, `volume ls`, `network ls`, `buildx ls`): no own object – no `oi4/oec-*` image, no `oec-c091`/`oec-dev` container, volume or network, no `oec-*` builder. Objects of other projects were not touched.
- **Created and removed in C-091:** images `oi4/oec-registry:0.1.0`, `oi4/oec-logger:0.1.0`, `oi4/oec-weather:0.1.0`, `oi4/oec-config:0.1.0` (`build.sh --remove`); builders `oec-registry-<pid>`, `oec-demos-<pid>` and BuildKit (removed by the scripts); D8 sandbox containers (removed by D8); compose project `oec-c091` – containers `oec-c091-mosquitto-1`, `oec-logger-…`, `oec-weather-…`, `oec-config-…`, `oec-registry-…`, network `oec-c091_default`, volumes `oec-c091_data`, `oec-c091_oec-{logger,weather,config,registry}-{etc,opt}` (`down -v`); compose project `oec-dev` of `live.sh` (`down -v` by the script) and its test containers.
- **After:** images, volumes and networks identical to the before-list; containers only those of other projects. No prune.

## Commits

- Docs page: `oec/docs/docs/registry/index.md`, `oec/docs/docs/demos/index.md`.
- READMEs: `oec/registry/README.md`, `oec/dev/README.md`, `oec/conformance/README.md`.
- This log, ADR-0029 amendment C-091, ROADMAP, STATUS, REVIEW, REVIEW_DE.

## Checks (final)

- `caffeinate -i oec/check.sh` after the last change (docs, READMEs and project files only): **all checks passed**, 8 min 13 s, load 3.5 before and after; registry step incl. `internal/uipolicy` ok (no file under `ui/` changed).

## For the PO

- **D-1 – close P7?** Proposal (a): close P7, next phase P8 (Docusaurus site), then P9; carried items above.
- Nothing to review (no R-x), no SF.
- For information: A-424 … A-428.

## Handover (G-3.10)

**A new session is recommended** for the next cycle: this session's context was compacted. `prompt.md` (workspace root, German, not committed) holds the start prompt; it depends on D-1.

## Next

After D-1: with (a) C-092 = P8 start (plan and cycle cut of the Docusaurus site, ADR for the site); with (b) C-092 = P9 start; with (c) the named gap.
