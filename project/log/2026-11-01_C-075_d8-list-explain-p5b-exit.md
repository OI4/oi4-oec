# C-075 – P5b increment 6: `oec-infra list`/`explain`, D8 coverage, report freeze, P5b exit run

- **Date:** 2026-11-01
- **Phase:** P5b – Infrastructure compliance tool (D8), increment 6 (the last of the P5b plan)
- **Baseline:** guideline `52d8470` (branch head `eb821e9`, findings only), SF-001 … 117 resolved, no open finding; 1411 inventory items, 444 REQs, 158 `schema`/`d7-validate` REQs, 0 gaps, no REQ drafts; 29/29 entry schemas, 245 payload vectors, 18 `broker.json` vectors; schema 2.0.0 frozen, `2.0.1-dev` @ `52d8470`; fixtures: 29 captures, 59 sim (58 faults), 19 images, 29 bus scenarios, 69 host scenarios, 26 container scenarios, 20 run fixtures, all reviewed; catalog: 55 D7 tests and 17 of 17 D8 tests `implemented`; D7 report version 1 frozen, D8 report version 1 additive until this exit
- **Agent:** new agent session (start prompt of the C-074 handover, G-3.10)

## Intent

PO order (start prompt): C-075 = P5b increment 6, test first:
- `oec-infra list`/`explain` (pattern of `oec-check`, C-069);
- coverage report for `d8.yaml` (`internal/explain` and `COVERAGE.md` beyond D7-only, lift A-248; include the indexes of image, bus, host, container and run; every implemented D8 test needs a positive and a negative fixture);
- freeze D8 report version 1 (release freeze as for the D7 report);
- README/DESIGN "As built" pass;
- P5b exit run and exit assessment (criteria of the ROADMAP; justify what moves to P6/P7/P9).

D7 is closed: only shared packages may change, the D7 gate and the D7 part of `COVERAGE.md` stay unchanged in content.

## Baseline check

Guideline `52d8470` on `feature/spec-findings` (head `eb821e9`); SF-001 … 117 resolved, next SF-118; the numbers of the start prompt confirmed (1411 items, 444 REQs, 0 gaps; all fixture counts; 55 D7 and 17 D8 tests `implemented`); cbindgen 0.29.4; `oec/check.sh` "all checks passed" before the first change.

## Work

### 1. Coverage report for D7 and D8 (`a2c62c9`)

- `internal/explain` had one hard-wired view (D7 catalog, `d7-monitor`/`d7-test`/`validate`, sim and capture fixtures). It now has **one view per tool**: `explain.D7` and `explain.D8` (catalog file, modes, needs, verification kinds, fixture kinds); `Load`/`LoadInfra`/`LoadTool` build an index for one view. The D7 view is the former behavior.
- `conformance.InfraFixtures()` loads the five D8 indexes in the order image, bus, host, container, run (the run index was outside `Fixtures()` by A-248; `Fixtures()` of D7 stays as it is). `FixtureRef.Manual` lists the requirements a fixture expects as manual (host and container scenarios).
- **D8 rules** of the view: one verification kind `d8`; no `validate` column; level `–` for a test that checks only platform requirements (not graded, MG 1.3; such a test runs at every `--level`); a D8 fixture runs only the tests of its own mode; a positive fixture is an index entry `positive: true` that runs the test and states no outcome or `pass` for it. Several host scenarios state `pass` for a test together with a manual verdict; on a non-positive scenario that counts in the `manual` column only, not as positive.
- `COVERAGE.md`: the D8 section is **appended**; the D7 section is byte-identical with the file before C-075 (in the gate: `TestCoverageReport` requires that the combined report starts with the D7-only report). Format change justified: additive, the D7 reader sees the same section; the new section answers the P5b exit criterion "coverage report of `d8.yaml`".
- Tests first: `TestInfraIndex` (counts, level `–`, manual, positive rule, the `oec-check` pointer), `TestFixtureCoverage` over both indexes (red until the D8 index existed), `TestCoverageReport` (D7 prefix), `TestFixtureIndexes` in the module root (now also over `InfraFixtures`).

D8 numbers in `COVERAGE.md` at C-075:

| | |
|---|---|
| Tests | 17 `implemented`, 0 `planned`; image 4, run 3, container 3, host 4, bus 3 |
| `needs` | docker 7, host-access 4, credentials-2 1, none 6 |
| `d8` REQs | 52: 51 checked by tests, 1 `manual` (REQ-CMP-012), 0 uncovered (the C-044 design table counted 50; the number grew with the re-baselines since) |
| Fixtures | 163: 19 image, 29 bus, 69 host, 26 container, 20 run; 27 positive; 163 reviewed |
| Positive and negative per test | 17 of 17 |

### 2. `oec-infra list`/`explain` (`db3e4ed`)

- Test first: `cmd/oec-infra/list_test.go` (table and JSON output, filters `--mode`/`--level`/`--req`/`--needs`/`--status`, patterns, the closing line, `explain` of a test, of a REQ checked by D8, of a REQ checked only by D7, of the manual REQ, a `T7-…` id, unknown ids and usage errors), then `list.go` and the wiring in `main.go` (switch, usage, package doc).
- Output as `oec-check`: level `-` in `list`, "not graded" in `explain`; the closing line "17 tests (17 implemented, 0 planned), 1 manual requirement (oec-infra explain REQ-…)"; fixture lines add ", manual …" with the requirements a fixture expects as manual. A requirement without a D8 test but with D7 kinds prints "not checked by oec-infra (oec-check explain REQ-…)" (JSON `oec_check`; no `validate` field in D8). A `T7-…` id is a usage error that points to `oec-check explain` (exit 2).
- The helpers (`parseInterleaved`, table writer, JSON views) are **copied** from `cmd/oec-check/list.go` and adapted rather than moved into a shared package: D7 is closed, and a move would change `cmd/oec-check` (A-292).

### 3. D8 report version 1 frozen (`eb21a0d`)

- `infra-report.schema.json`: the description says "frozen since the P5b exit (C-075)"; no key changed. `internal/report/infra.go`: the comment of `InfraVersion` likewise.
- New `freeze_test.go` in the module root (`TestReportFreeze`): the SHA-256 of the LF-normalized `report.schema.json` and `infra-report.schema.json` is pinned. The D7 hash equals the file of C-069 (`4ce2f57`), so the D7 schema is unchanged; the D7 freeze of C-069 was a declaration, now both are enforced.
- A format change after this point needs a new report version, a new schema file and a new pin; version 1 stays readable (A-296).

### 4. Docs pass (this commit)

DESIGN §2 (rows of `cmd/oec-infra`, `catalog`, module root, `internal/explain`), §6 "As built (C-075), D8 coverage", §7 "As built (C-075), freeze", §9 "As built (C-075), `list`/`explain`", §10 "D8 (C-075)", header; README (state line, `oec-infra list`/`explain`/`report-schema` section, `COVERAGE.md`, report freeze, layout, tests, cross builds, `live.sh`); ADR-0025 amendment C-075; ROADMAP P5b (heading, item 6, C-075 row, exit assessment); STATUS; REVIEW/REVIEW_DE.

## Defects found and fixed (test first)

- **Live test left anonymous volumes** (`513e0c9`, A-297). `TestContainerDev` (`cmd/oec-infra/live_test.go`, C-073) creates two containers from the dev broker image (Mosquitto) and removed them with `docker rm -f` without `-v`. The image declares two `VOLUME`s (data and log), so every live run since C-073 left 4 anonymous, empty volumes. Found in the exit run by comparing the volume list before and after.
  - Test first: `TestContainerDev` records the dangling volumes before and compares after its cleanup; red with `rm -f` (4 new dangling volumes reported), green with `rm -f -v`. `live.sh` header notes the rule.
  - `oec-infra run` was not affected: it removes its containers with `v=1` (`internal/engine/write.go`, amendment C-074 item 1); `TestRunDev` already checked that no `oec-d8-*` object is left.
  - Cleanup without `prune`: the 8 volumes of this cycle's two red/green runs and 8 older ones were removed **by name**. The older ones were identified as leftovers of the C-073 and C-074 live runs by their creation time (the times of those runs), the owner of their mount point (the uid/gid of the broker image) and their empty content, checked with an own throw-away `--rm` container; nothing else was touched. Afterwards the Docker state equals the snapshot before the cycle minus those 8 old volumes; no `oec-live-*` or `oec-d8-*` container, volume, network or image is left. No volume id or foreign object is named in a committed file.

## Runs

- **Gate** (`oec/check.sh`): "all checks passed" after each step and before the final docs commit.
- **Cross builds** (`CGO_ENABLED=0`): `oec-check` and `oec-infra` for windows/linux/darwin × amd64/arm64 and linux arm/386 – 16 binaries built; `go vet ./...` clean for windows amd64/arm64 and linux arm/386.
- **Live** (`live.sh`, opt-in, Docker Desktop with Engine 28.4, linux/arm64, macOS arm64 host), exit 0:

| Test | Against | Result |
|---|---|---|
| `TestMonitorDevBroker` | dev broker, ports 8883/8884 | pass |
| `TestBusDevBroker` (`oec-infra bus`) | dev broker, credentials and client certificate | T8-BUS-01 pass; T8-BUS-02 warn REQ-RES-CONFIG-026; T8-BUS-03 skip (no Registry); exit 4 |
| `TestHostDevRoots` (`oec-infra host`) | both sample roots of `oec/dev` on macOS | T8-HST-01/02 pass; T8-HST-03 skip (not asked); T8-HST-04 fail REQ-PLT-008 (no Registry), REQ-PLT-003 (not Linux) |
| `TestHostDevContainer` | the Linux binary in a container in the broker network, storages read-only | T8-HST-01/02 pass; T8-HST-04 fail REQ-PLT-008, REQ-PLT-006 (no engine socket in the container); REQ-PLT-001/003 manual |
| `TestContainerDev` (`oec-infra container`, `host --tests T8-HST-03`) | a conforming and a faulty throw-away container, the dev broker | conforming: T8-CTR-01 … 03 pass; faulty: CTR-01 warn, CTR-02 fail, CTR-03 fail; dev broker: CTR-02 fail (hostname ≠ name, not an OI4 container, A-279); T8-HST-03 with the shared volume fails REQ-CNT-011, without passes |
| `TestRunDev` (`oec-infra run`) | 15 test images `oec-d8-sim:<fault>` | 15 of 15 faults as against the fake Engine API |

- **Cleanup check** after the live runs: `docker compose down -v` ran; no `oec-live-*` or `oec-d8-*` container, volume, network or image is left; `oec/dev/generated/` is clean.

## P5b exit assessment

Criteria of ROADMAP P5b "Exit" (precised in C-070):

1. *Every `d8.yaml` test `implemented` or a documented `manual`; every implemented test has a positive and a negative fixture, all reviewed by the PO; the self-test runs in the gate offline – no network, no Docker daemon.* – **Met.** 17 of 17 `implemented`; 1 `manual` requirement with reason (REQ-CMP-012: the tester chooses the level); 52 `d8` REQs, 0 uncovered. 163 fixtures, all `reviewed` (R-1/R-2 of C-070 … C-074); `TestFixtureCoverage` checks a positive and a negative fixture for all 17 tests over the five indexes, besides the per-package checks; `COVERAGE.md` shows them per test. The self-test uses generated OCI archives, generated host roots, the fake Engine API (`enginetest`), `mqtttest`, the simulated Registry and the simulated application; loopback only.
2. *Self-test against purpose-built images: `image` and `run` pass at level 3 on a good image and fail with the expected REQ ids on each faulty image – `image` in the gate, `run` in the opt-in run with Docker (`live-infra.sh`), recorded in the exit log.* – **Met.** `image`: the 3 positive fixtures pass at level 3, each of the 16 others gives its expected REQ ids (gate, also through the CLI with every JSON report validated). `run`: `TestRunDev` in `live.sh` (as built instead of a separate `live-infra.sh`, A-290) – `oec-d8-sim:good` passes, each of the 14 faulty images gives its expected verdicts and REQ ids; 15 of 15 (table above).
3. *`bus` and `host` run against the dev environment (`oec/dev`, opt-in); the result is recorded in the exit log, deviations are fixed in the dev environment or become an SF.* – **Met with recorded deviations, no SF** (A-298). Recorded above. Each deviation comes from what `oec/dev` is – a dev broker with sample storages on a developer machine, not a Service Container platform:
   - BUS-02 warn REQ-RES-CONFIG-026 (SHOULD): Mosquitto ACL patterns cannot bind the AppId level of a topic to a user, so every authenticated client can read `Set/Config` (C-071, A-258). Not fixable in Mosquitto's ACL syntax; the guideline is consistent.
   - BUS-03 inconclusive and HST-04 REQ-PLT-008 fail: there is no OEC Registry before P7. Fixed by P7, not by `oec/dev` now.
   - HST-04 REQ-PLT-003 on macOS (not Linux) and REQ-PLT-006 in the container (no engine socket mounted): properties of the test host, correct verdicts.
   None of them is a contradiction or a gap in the guideline, so no SF.
4. *`infra-report.schema.json` frozen (report version 1) and every report of the gate validated; `list`, `explain`, `report-schema`; coverage report of `d8.yaml`; `CGO_ENABLED=0` cross builds windows/linux/darwin × amd64/arm64.* – **Met.** Frozen and hash-pinned (with the D7 schema); the gate validates every D8 JSON report (CLI tests of all five commands, every fixture through the CLI); `list`/`explain` (this cycle), `report-schema` (C-070); `COVERAGE.md` D8 section; cross builds as above, plus linux arm/386.

**Carried to other phases** (as planned in C-070, A-299), with the reason:
- "Our demo and Registry images pass `level3`" → P6 exit (demo images) and P7 exit (Registry image): the images do not exist before those phases. With the Registry, BUS-03 and REQ-PLT-008 can also pass against `oec/dev`.
- CI on Windows/macOS/Linux runners, the D8 binaries and the tool container image → P9 (as D7, D-1 (a) of the C-069 review): they are release artifacts and depend on the publication decision (Q-011).
- Live runs on Podman and on a Windows engine (named pipe; A-271, A-289) → P9 CI: no Podman and no Windows engine on this machine; the code paths have unit tests and cross builds.

**Proposal (REVIEW D-1):** close P5b with the carried items; next phase (a) P6 demo services (recommended), (b) P4b micro SDK, (c) P7 Registry, (d) keep P5b open.

## Agent decisions (G-3.4 b, for information: REVIEW A-292 … A-300)

- **A-292** `oec-infra list`/`explain` as `oec-check`; helpers copied, not moved (D7 closed).
- **A-293** `internal/explain` with one view per tool; `InfraFixtures` (image, bus, host, container, run); A-248 lifted.
- **A-294** `COVERAGE.md`: D8 section appended, D7 section byte-identical; D8 columns (`d8`, no `validate`, level `–`, `manual`).
- **A-295** Positive D8 fixture rule; 52 instead of 50 `d8` REQs.
- **A-296** Freeze by hash pins of both report schemas.
- **A-297** Live test volume leak fixed; own leftovers removed by name, no prune.
- **A-298** Exit criterion 3: deviations of the dev environment recorded, no SF.
- **A-299** Carried items to P6/P7/P9.
- **A-300** No new dependency, no schema change, no new or changed test data, D7 unchanged.

## Changes to reviewed items

None. No fixture, REQ, vector or catalog entry changed (the catalog `d8.yaml` is read, not written). `COVERAGE.md` gains a section; its D7 part is unchanged.

## Spec findings

None (next SF-118).

## Dependencies

None. `list`/`explain` and the freeze test use the standard library and the existing module packages; `cmd/oec-infra/live_test.go` gained the import of `slices` (standard library).

## D7 unchanged

`cmd/oec-check`, `internal/monitor`, `internal/probe`, `internal/sim`, the sim and capture indexes and `report.schema.json` (hash equals C-069) are untouched. Shared packages changed: `internal/explain` (views; D7 output identical), module root (`InfraFixtures`, `freeze_test.go`), `internal/report/infra.go` (comment only). The D7 gate (55 tests, sim fixtures, replays, report schema, D7 part of `COVERAGE.md`) passes unchanged.

## Risks and open technical points

- **CLI duplication** (A-292): `cmd/oec-infra/list.go` and `cmd/oec-check/list.go` share about 100 lines of helpers. If D7 opens again (a new report version, a new catalog test), the helpers can move to `internal/explain` in one step; both CLI tests pin the output.
- **Podman and Windows engine** not run live (carried to P9).
- **The freeze test** pins bytes, not meaning: a pure reformatting of a schema also fails it. That is intended – the published file is the contract.

## Commits

| Commit | Content |
|---|---|
| `a2c62c9` | `internal/explain` views D7/D8, `InfraFixtures`, D8 section of `COVERAGE.md` |
| `db3e4ed` | `oec-infra list`/`explain` |
| `eb21a0d` | D8 report version 1 frozen, `freeze_test.go` (both report schemas) |
| `513e0c9` | live test: `rm -f -v` and the dangling-volume check |
| (this commit) | docs: DESIGN §2/§6/§7/§9/§10, README, ADR-0025 amendment C-075, ROADMAP, STATUS, REVIEW/REVIEW_DE, this log |

## Checks (final)

- `oec/check.sh`: "all checks passed" (final run before the docs commit).
- D7: 55 tests `implemented`, D7 part of `COVERAGE.md` byte-identical; D8: 17 of 17 `implemented`, 17 of 17 with a positive and a negative fixture.
- Fixtures: 29 captures, 59 sim, 19 images, 29 bus, 69 host, 26 container, 20 run – all `reviewed`.
- No key, certificate or `prompt.md` committed; `oec/dev/generated/` clean; `legacy/` untouched; no foreign container or volume of this Mac named in a committed file.

## For the PO

- **D-1:** P5b exit and the next phase. Recommendation: (a) close P5b, next P6.
- No R-x (no new or changed test data), no SF.
- A-292 … A-300 for information.

## Handover (G-3.10)

New agent session for C-076. The next cycle starts a new phase (plan and first increment) chosen by D-1; the D8 context of C-070 … C-075 is in DESIGN §9/§10, the README and ADR-0025 and is not needed beyond them. The start prompt is in `prompt.md` (not committed).

## Next

PO answer to D-1. Then C-076: with (a) the P6 plan (weather demo in Rust on `oec`, logger demo in C on `oec-ffi`, Dockerfiles with SBOM and license texts, compose integration; cycle cut; exit with D7 and D8) and its first increment.

## Addendum 1 – C-075 review (2026-11-01)

PO answer (chat, 2026-11-01): "D-1 (a), zu den Demo-Diensten habe ich aber noch Ideen:
- Service 1 Wetterservice wie gehabt
- Service 2 Config Service, der die Configs von Service 1 und 3 anzeigt und bearbeiten kann
- Service 3 Logger, vielleicht noch Grafana (wenn machbar) zur Historienaufzeichnung

Alle 3 Service sollen eine kleine Web UI bekommen (gehört nicht zum OEC Standard) bei dem man sieht was passiert. Bei Service 1 kann über die Config die Koordinaten für das Wetter eingestellt werden, ansonsten noch was Sinn macht. Überleg dir was."

### Applied

- **D-1 (a): P5b is closed** with the carried items of the exit assessment (demo images → P6 exit, Registry image → P7 exit, CI/binaries/tool image/Podman/Windows engine → P9). ROADMAP P5b heading ✅, STATUS, REVIEW/REVIEW_DE. **Next phase: P6.**
- **P6 scope (PO input, G-3.4 a: scope and deliverables):** three demo services instead of two, each with a small Web UI that is not part of the OEC standard:
  - **D3 weather service** (`OTConnector`, Rust on `oec`) as planned; its Config sets the coordinates of the weather locations and further sensible settings (proposal A-302).
  - **D10 config service** (new deliverable): shows and edits the `Config` of the weather service and of the logger (proposal A-304).
  - **D4 logger service** (C on `oec-ffi`, Q-024): now with a `Config` resource (so that the config service can edit it) and a history; Grafana for the history "if feasible" (D-4).
  ROADMAP P6 rewritten as the planning input of C-076; project README deliverables (D10), AGENTS.md quick facts.
- A first attempt to ask the PO the license question of Grafana in the chat got no answer (the PO reviews later). The points that are the PO's by G-3.4 a are therefore recorded as open questions with a recommendation, the engineering consequences are decided by the agent:
  - **D-2 = Q-031** language/runtime of the config service D10 (recommended: Rust on `oec`).
  - **D-3 = Q-032** technique of the three Web UIs (recommended: served by each service itself, static HTML/CSS/JavaScript without a build step and without a framework, live updates by Server-Sent Events).
  - **D-4 = Q-033** Grafana (AGPL-3.0) for the history of the logger (recommended: an optional compose profile with the unchanged official Grafana and Prometheus images, not built or shipped by us; the logger shows a short history in its own UI without them).
  - **A-301** Messaging levels: weather L3, logger **L2** (it now offers `Get`/`Set Config`; the `Event` Status after `Set` is a must from L2), config service **L1** (it offers nothing to modify; it only requests); all three Service Container L3. The PO asked in C-007 for one demo at L1 and one at L3 – that holds, the logger adds L2. ADR-0016 amendment C-075 review. Language C of the logger stays (Q-024); the part "configured by file/environment, no `Config` resource" of Q-024 is superseded by the PO input.
  - **A-302 … A-306** proposals for the Config contents, the config service, the Web UIs and the provisional cycle cut (REVIEW.md); final with the P6 plan and ADR-0027 in C-076.
- No code, no test data, no reviewed item changed; no SF (next SF-118). `oec/check.sh` unchanged since the docs commit.
- `prompt.md` (not committed) is rewritten for C-076: P6 plan, ADR-0027, increment 1 (weather core) – which does not depend on D-2 … D-4.
