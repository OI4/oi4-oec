# C-086 – P7 start: Registry architecture and increment 1 (core at Level 2)

- Date: 2026-11-02
- Phase: P7 (OEC Registry D5), increment 1 of 6 (ROADMAP P7 cycle cut, A-381)
- Agent session: new (handover of C-085, `prompt.md`)
- Baseline: guideline `52d8470` (branch `feature/spec-findings`, head `eb821e9`), SF-001 … 117 resolved, next SF-118; workspace head `fe2964b`

## Intent

1. Architecture of the OEC Registry D5 after Q-035 (Rust on `oec`, React + TypeScript + Vite, REST/OpenAPI, SSE, AsyncAPI for the MQTT interface): ADR-0029, with the engineering choices left to the agent (versions, HTTP server crate, npm policy and its gate, checks of the API documents).
2. P7 cycle cut in the ROADMAP.
3. Increment 1, test first: the registry core – read-only (ADR-0008), Registry services of MG A1, judged by D8 T8-BUS-03 and D7 in the gate.

## Plan

1. Baseline gate.
2. ADR-0029; ADR-0023 amendment for the HTTP crates and the dev-dependency of the document checks.
3. SDK: what is missing to answer the Registry services for the assets of others (additive, test first).
4. `oec/registry`: directory rules (sans-IO, unit tests), bus side, Registry services, AsyncAPI document with tests, Go integration tests with D8 and D7, `check.sh` as a gate step.
5. Docs: log, ROADMAP, STATUS, REVIEW/REVIEW_DE, READMEs; handover.

## Work

### 1. Baseline

Gate on the workspace head `fe2964b` → **all checks passed**. Counts unchanged: 1411 inventory items, 444 REQs, 158 `schema`/`d7-validate` REQs with 0 gaps, no REQ draft; 55 D7 and 17 D8 tests `implemented`; schema 2.0.0 frozen, work state `2.0.1-dev` @ `52d8470`; demos in the gate with their pins (logger 46, weather 49, config service 22). cbindgen 0.29.4 present.

### 2. Architecture – ADR-0029 (A-380 … A-389)

ADR-0029 `accepted (agent, G-3.4 b)`, inside Q-035 and ADR-0008. In short:

- **Layout (A-380):** `oec/registry/` is an own Cargo package `oec-registry` (own `Cargo.lock`, `oec` by path, like `oec/demos/config`), plus a Go module `github.com/OI4/oec/registry` for the integration tests (A-387) – the registry does not import or link D7/D8, it is judged by them as a black box (ADR-0006). Later: `api/openapi.json`, `ui/` (React), image files.
- **Identity and levels:** ServiceType `Registry`, `oi4.org` / `OEC Registry` / `OEC-REGISTRY`, SerialNumber = hostname; Messaging and Service Container L3 at the exit; increment 1 at **L2** (A-389) – the SDK serves `Get` only from L2, L2 needs no persistent ReferenceDesignation.
- **Directory model (§3):** one entry per IL; `application` vs. `device`; publisher; last `MAM`/`Health`; availability `online` / `offline-graceful` (Close) / `offline-ungraceful` (Will or `FAILURE_1`/0 about itself); online again only after the next Birth (REQ-LIFE-013); devices follow their publisher; "available" = online with `MAM` and `Health` (A1); v1 traffic not subscribed.
- **Discovery (§4):** subscriptions `Pub/MAM/#`, `Pub/Health/#` (more in increment 2); after a Birth or the first sighting `Get MAM`/`Get Health` of what is missing, rate-limited. **Hard rule:** only `Consumer::get` – a source scan in `bus.rs` asserts it, and D7/D8 see no `Set`/`Del`/`Call`.
- **Registry services (§5):** the regular `Get/MAM[/<Source>]`, `Get/Health[/<Source>]` of the registry's AppId, answered by the SDK through the new feature "listed assets" (A-386, §3 below).
- **Observed level and conformity view (§6, increment 2):** observed Messaging level from `Profile` and `Get` answers (informative); validation of every received message with the D1 schemas by the SDK feature `strict-validation` – independent of D7.
- **HTTP (§7, increment 3, A-383):** `hyper` 1 + `hyper-util` + `http-body-util` on the SDK's `tokio`/`tokio-rustls`; ports 5798 (UI + `/api`) and 5799 (API only); TLS with the application certificate, else self-signed; REST `GET` only; OpenAPI **3.2.0** (`itemSchema` for SSE).
- **AsyncAPI (§8, A-382):** **3.1.0**, `api/asyncapi.json`, payloads reference the D1 schemas by `$id`; extended by every increment.
- **Web UI (§9, increment 4, A-382):** React **19.3.1**, TypeScript **7.0.2**, Vite **7.3.x**; not Vite 8, because its hard dependency `lightningcss` is **MPL-2.0** (outside the license allowlist of ADR-0023) – for your information, no question as long as Vite 7 is maintained. Bundle = React, react-dom, scheduler (MIT); `ui/dist/` committed and embedded.
- **npm policy (§10, A-384):** exact pins, `package-lock.json` v3 committed, every package from `registry.npmjs.org` with sha512 integrity, licenses in the allowlist of ADR-0023 without exception, `npm ci --ignore-scripts`; the build is opt-in (`ui/build.sh`), the gate needs neither node nor network: a Go test checks lockfile, licenses and a source stamp.
- **Documents in the gate (§11, A-385):** `asyncapi.json` (and later `openapi.json`) validated against the vendored official meta-schemas with jsonschema-rs `=0.60.1` (dev-dependency, the pin of ADR-0019 §8), plus semantic tests (channels = topics of the code, operations read-only).
- **Image (§12, increment 5):** as the demos.

ADR-0023 amendment C-086: no new crate in increment 1 (the registry's `Cargo.lock` has 169 packages, every one already in the SDK lock; 78 unique normal crates); jsonschema `=0.60.1` as a dev-dependency; `hyper` 1 / `hyper-util` / `http-body-util` decided for increment 3, for `oec-registry` only (MIT).

### 3. SDK – listed assets (A-386, test first, commit `0f0c157`)

The SDK answered `Get MAM`/`Get Health` only for the application and its represented assets. A registry must answer them for the available assets of others without representing them (no heartbeat, no PublicationList entry, no Profile, no other resources). ADR-0026 amendment C-086 §22: `Provider::list_asset`/`unlist_asset`/`listed` in `oec-core`, `AppHandle::list_asset`/`unlist_asset`/`listed_assets` in `oec`; ServiceType `Registry` only; answers without Source in the order application, represented assets, listed assets (by IL), paginated; with the Source of a listed asset exactly its `DataSetMessage`; every other resource or method with that Source = unknown Source (Status event `BadNotFound` under the AppId, SF-099/108); own `DataSetWriterIds` per (resource, IL), persisted (REQ-MSG-021). Tests first: `oec-core/tests/provider_listed.rs` (new), `oec/tests/app.rs` `listed_assets`. No C-ABI or Go change (ABI version 1). No SF: A1 and REQ-REG-002 define the services; the SDK just had no API for them.

### 4. Registry increment 1 (`oec/registry`)

- `src/directory.rs` – the rules of ADR-0029 §3, sans-IO, 8 unit tests (Birth/Close/rebirth, Will, first sighting by heartbeat, devices follow their publisher, device of an unknown publisher, a MAM change is no Birth, ignored messages, Close of an unknown application).
- `src/bus.rs` – subscriptions, worker, `Get` of what is missing, listing/unlisting in the SDK; test `read_only` (source scan: only `Consumer::get`).
- `src/main.rs` – identity, level 2, log (`OEC_REGISTRY_LOG`), the SDK application.
- `api/asyncapi.json` (v0.1.0) – Registry services, lifecycle, collecting subscriptions, `Get MAM`/`Get Health` to others; `src/api_tests.rs`: valid against the vendored meta-schema, channels = the topics of the code, operations read-only.
- `registry_test.go`, `internal/regtest/` – the registry on the in-process TLS test broker (`go-testbroker` of the SDK), `oec-infra`/`oec-check` built into a temporary directory and run:
  - `TestD8Registry`: `oec-infra bus --tests T8-BUS-03` → **pass**, exit 0; the test asserts the subscriptions `Oi4v2/+/+/Pub/MAM/#` and `Oi4v2/+/+/Pub/Health/#` and logs the `Get` requests of the registry (0 towards others in this run – the probe components announce themselves).
  - `TestD7Level2`: `oec-check test --level 2` → **44 pass, 9 skip, 0 fail**, exit 0; the 44 passes are pinned. Skips: MON-07, MON-17, ADR-02 (no request by the CUT to others, no second namespace), MON-09, MON-14 (no `Data`, no cyclic publication), SET-05, SET-10 (no `Config` DataSet or RefDes object at L2), SET-04 (no `Del` D7 can restore), SET-03 (Level 3). T7-GET-04 warns REQ-CMP-010 and T7-MON-04 reports REQ-CMP-045 as info: D7 treats the assets the registry lists (D7 itself and its peer) as represented entities – the D7 refinement for registries (A-388) is planned for increment 2.
- `check.sh` – `cargo fmt --check`, clippy pedantic `-D warnings`, `cargo test`, `cargo deny --config ../sdk/rust/deny.toml check` (advisories, bans, licenses, sources ok), the build, `gofmt`, `go vet`, `go test -race`; **step 8 of `oec/check.sh`**.
- `README.md` – purpose, build/run, layout, checks.

### 5. D8 defects found by the first real registry (A-390, test first, commit `b7d5aa3`)

The first `TestD8Registry` run failed although the registry worked. Two defects of D8 T8-BUS-03 (D8 had only met its own simulated Registry so far):

1. **Missing `WriterGroupName`** in the Birth/Health/Close/Will of the two probe components and in the answers of the simulated Registry. The D1 schema requires it (REQ-MSG-016, MG 8.2.3); the SDK consumer of the registry dropped the messages ("missing field WriterGroupName"), so the components were never listed. Test first: `internal/bus/registry_test.go` `TestComponentMessages` validates every probe-component message, D8's `Get` and the Status event of the simulator with D7 `validate`; it failed, then `"WriterGroupName": ""` was added in `registry.go` and `registrytest.go`.
2. **Status event for a gone component judged as a wrong topic:** after Close and Will, `Get MAM/<component>` names an unknown Source; the registry (SDK) answers with the Status event `Bad` under its AppId and no `Pub` (SF-099, SF-108). D8 failed REQ-REG-002 with "published on the wrong topic". Test first: new conforming simulator variant `registrytest.StatusEvent` (`status-event`, not a fault) and the positive scenario **`T8-BUS-03/status-event`** in `testdata/bus/index.yaml` (status `draft`, G-3.9 → **R-1**); it failed, then `round()` accepts `…/Pub/Event/<il>/Status/…` when the component is absent; with the component available it is still a violation.

`COVERAGE.md` regenerated: only the D8 fixture counts changed (bus 29 → 30 scenarios, 6 → 7 positive, 163 of 164 reviewed; T8-BUS-03 positive 2 → 3); the pinned line in `internal/explain/explain_test.go` follows. D8 report version 1 unchanged. ADR-0025 amendment C-086; `DESIGN.md` note at T8-BUS-03. Conformance `go test ./...` green. Then `TestD8Registry` passed.

### 6. Gate

See "Checks (final)".

## Agent decisions (G-3.4 b, for information: REVIEW A-380 … A-390)

- A-380 layout `oec/registry` (Cargo package + Go test module); A-381 P7 cycle cut C-086 … C-091; A-382 versions (OpenAPI 3.2.0, AsyncAPI 3.1.0, React 19.3.1, TypeScript 7.0.2, Vite 7.3.x – not Vite 8/MPL-2.0); A-383 `hyper` 1 stack; A-384 npm policy and offline gate; A-385 meta-schemas vendored, jsonschema-rs dev-dependency; A-386 SDK listed assets; A-387 Go integration-test module, D7/D8 as black boxes; A-388 D7 refinement for registries in increment 2; A-389 increment 1 at L2; **A-390** D8 T8-BUS-03 defect fixes (§5).

## Test data for review (G-3.9)

- **R-1:** `oec/conformance/testdata/bus/index.yaml`, scenario `T8-BUS-03/status-event` (positive, `draft`): the simulated Registry answers `Get MAM` of a closed component with the Status event `Bad` under its AppId → T8-BUS-03 pass. Basis: SF-099 (unknown asset → no `Pub`, should `Event` Status Bad), SF-108 (event under the AppId).

## Changes to reviewed items

- None. The reviewed bus scenarios keep their verdicts; the new scenario is `draft`.

## Spec findings

- None. Both D8 defects and the SDK gap were covered by the guideline (REQ-MSG-016, SF-099, SF-108, A1).

## Dependencies

- Rust: no new crate (ADR-0023 amendment C-086); jsonschema `=0.60.1` as a dev-dependency of `oec-registry` (already in the SDK lock).
- Go: standard library only (the new module `oec/registry` has no `require`).
- Vendored file: `oec/registry/api/meta/asyncapi-3.1.0.schema.json` from `asyncapi/spec-json-schemas`, commit `eee8d2217d23b22c475090c0e12f20801f531f11`, `schemas/3.1.0.json`, sha256 `52cdfc5a54ac7edd0b7c00d320d3f7b85b72e05377479a22a163e56f6d3b896c`, license Apache-2.0 (`LICENSE-asyncapi-spec-json-schemas` beside it). Data, not code.
- npm: none yet (increment 4).

## D7, SDKs, demos

- D7 unchanged. D8: two defect fixes (A-390, ADR-0025 amendment C-086). Rust SDK: additive "listed assets" (ADR-0026 §22); C ABI, Go SDK and demos unchanged (their gate pins hold).
- `COVERAGE.md`: only the D8 fixture counts (above).

## Docker objects (own only)

- No Docker was used in C-086 (no image, container, volume or network created). No before/after list needed.

## Commits

- `0f0c157` SDK listed assets (A-386); `b7d5aa3` D8 T8-BUS-03 defect fixes (A-390); `8153ac2` ADR-0029 + ADR-0023/0025 amendments; `bcf37c0` `oec/registry` + gate step 8 + `oec/README.md`; `dd65943` conformance README; then the project files (log, ROADMAP, STATUS, REVIEW/REVIEW_DE).

## Checks (final)

- `oec/check.sh` → **all checks passed**, rc 0 (load average about 4.5 at the start), after the last code change; step 8 `registry/check.sh` included (`go test` of `oec/registry` ok in 41 s: `TestD8Registry` T8-BUS-03 pass, `TestD7Level2` 44 pinned passes); D7 pins of the demos unchanged (46/49/22).
- `oec/registry/check.sh` alone and `go test ./...` of `oec/conformance` green before the full gate.

## For the PO

- **R-1** to review: the positive bus scenario `T8-BUS-03/status-event` (draft).
- No D-x (nothing under G-3.4 a), no SF.
- For information: A-380 … A-390 – especially A-382 (Vite 7, not Vite 8, because of MPL-2.0 `lightningcss`), A-386 (additive SDK feature), A-390 (two D8 defects fixed).

## Handover (G-3.10)

**Same or new session both work** for C-087: the context of this session was compacted several times; `prompt.md` (workspace root, German, not committed) holds the start prompt of C-087, so a new session is recommended.

## Next

C-087 – P7 increment 2: Level 3 (own `Config`, persistent ReferenceDesignation), `Profile`/`Event`/`PublicationList`/`SubscriptionList`, publisher relation, observed level and conformity view, the D7 refinement for registries (A-388), D7 `level3` pinned, AsyncAPI extended.

## Addendum 1 – PO review of R-1 (2026-11-02)

PO answer (chat, 2026-11-02): "R-1 akzeptiert"

### Applied

- **R-1 accepted:** `oec/conformance/testdata/bus/index.yaml` scenario `T8-BUS-03/status-event` `draft` → `reviewed`. `COVERAGE.md` regenerated (`-update`): only the fixture line changes, 164 of 164 D8 fixtures reviewed. Conformance README `testdata/bus` row.
- REVIEW/REVIEW_DE (R-1 ☑ accepted, nothing open), STATUS (header, blocking line, cycle row C-086 addendum 1).
- No other change; the handover stays (new session for C-087, `prompt.md` updated: R-1 no longer open).
- Checks: `go test ./...` of `oec/conformance` green; `oec/check.sh` → all checks passed.
