# C-087 – P7 increment 2: Registry Level 3

- Date: 2026-11-03
- Phase: P7 (OEC Registry D5), increment 2 of 6 (ROADMAP P7 cycle cut, A-381)
- Agent session: new (handover of C-086, `prompt.md`)
- Baseline: guideline `52d8470` (branch `feature/spec-findings`, head `eb821e9`), SF-001 … 117 resolved, next SF-118; workspace head `5d8f3b0`

## Intent

1. Registry at Messaging **Level 3**: own `Config` (settings), persistent `ReferenceDesignation`; collect `Profile`, `Event`, `PublicationList`, `SubscriptionList` with their `Get`s; publisher relation and devices; observed Messaging level (informative) and conformity view (`strict-validation`, independent of D7) – ADR-0029 §3/§4/§6.
2. D7 refinement for registries (A-388): T7-GET-04/SET-03/MON-04 treat only the application itself as an entity for ServiceType `Registry` (ADR-0025 amendment, test first) – or an SF if the guideline text is ambiguous.
3. D7 `level3` pinned against the registry in gate step 8, D8 T8-BUS-03 still pass; AsyncAPI extended (channels = topics of the code, read-only).
4. Read-only stays hard (ADR-0008); the source scan of the bus side stays.

## Plan

1. Baseline and gate.
2. D7 refinement, test first (sim variant + monitor test), ADR-0025 amendment.
3. SDK: what the conformity view needs from the bus (a typed subscription only delivers what decodes) – smallest additive extension, test first.
4. Directory L3 (sans-IO, unit tests), bus side (Config, raw subscriptions, conformity check, `Get`s), `main.rs` at L3.
5. AsyncAPI regenerated from the topic forms; Rust and Go tests; D7 `level3` pins; RD persistence test.
6. Docs, gate, commits.

## Work

### 1. Baseline

Guideline `52d8470` checked out, branch head `eb821e9`; 1411 inventory items, 444 REQs, 0 gaps, no REQ draft; D7 55 / D8 17 tests `implemented`; schema `2.0.1-dev` @ `52d8470`; cbindgen 0.29.4. `oec/check.sh` at the start → **all checks passed**.

### 2. D7 refinement for registries (A-391, test first, ADR-0025 amendment C-087)

MG 1.3 defines a represented entity as one without an MQTT client of its own, represented by one application; MG A1 says a registry publishes the `MAM` and `Health` of the assets it lists "as publisher of its own". The text is consistent – **no SF**; D7 only had to tell the two apart.

- Test first: monitor `TestListedAssets` (a Registry and a non-registry contrast) and the new conforming sim variant **`T7-GET-04/registry`** (fault `sim.Listed`: ServiceType `Registry`, the asset has only `MAM` and `Health`, every other request about it answered as for an unknown asset). Both failed before the change (T7-GET-04 REQ-CMP-010/045, monitor note REQ-CMP-045).
- `probe`: for ServiceType `Registry` T7-GET-04 judges the application only (REQ-CMP-045 skipped: "listed assets (MG A1), not represented entities"); T7-SET-03 has no asset target and expects the Status event of a request about a listed asset under the AppId (SF-108). `monitor`: T7-MON-04 judges REQ-CMP-006/007 for the registry's own AppId only and skips the REQ-CMP-045 heuristic. T7-GET-01 still asks for `MAM` and `Health` of every listed Source (the Registry service).
- Catalog notes of T7-GET-04/SET-03/MON-04, `DESIGN.md` bullet; `COVERAGE.md`: only the fixture counts (60 sim, 15 positive, 88 of 89 reviewed). D7 report version 1 unchanged.
- The fixture is `draft` → **R-1**.

### 3. SDK – raw subscriptions (A-392, test first, ADR-0026 amendment C-087 §23)

A typed subscription delivers only what decodes; the conformity view must also see what does not. `oec-core::Consumer::subscribe_raw(&[TopicFilter])` reports a matching publication in `Incoming::raw` with its topic, payload, decoded items and the decoding error; `oec::Consumer::subscribe_raw(Resource, &Target, f)` calls `f(&RawPublication)`. MQTT subscriptions, reference counting, SubscriptionList entries as for typed subscriptions. Tests written first: `oec-core/tests/consumer.rs::raw_subscriptions_get_the_publication_as_received`, `oec/tests/consumer.rs::raw_subscriptions_get_every_publication`. **No C-ABI and no Go change** (ABI version 1).

### 4. Registry Level 3 (`oec/registry`)

- `src/main.rs`: `LEVEL = L3`; the registry's `Config` is added to the application before the start; the `ReferenceDesignation` is persisted by the SDK file store of the container storage.
- `src/bus.rs` (rewritten):
  - **Config `registry`, group `directory` (A-393):** `GetInterval` 10 s (1 … 3600), `GetTimeout` 5 s (1 … 60), `HealthHistory` 20 (0 … 1000), `EventTrail` 200 (0 … 10000), `ViolationTrail` 10 (0 … 1000); read by the worker at least once a second, clamped, defaults when unset.
  - **Raw subscriptions** `Oi4v2/+/+/Pub/<R>/#` for `MAM`, `Health`, `Profile`, `Event`, `PublicationList`, `SubscriptionList`.
  - **Conformity check (A-394):** strict D1 schema validation by `oec_core::schema::Validator` (feature `strict-validation`); attribution as D7: topic outside the grammar → REQ-TOP-003, no JSON → REQ-CMP-005, schema violation → its requirements, schema-valid but not decodable → a violation without requirement; text ≤ 512 characters; counted per `AppId` of the topic.
  - **`Get`s (A-395):** `MAM`, `Health`, `Profile`, `PublicationList`, `SubscriptionList` without Source, only what is missing, once per online period and at most once per `GetInterval`, 500 ms after the sighting; one probe thread per application (≤ 16); a batch stops when `Get MAM`/`Get Health` times out. Complete list answers replace the application's lists.
  - **Read-only scan:** now over all files under `src/` (test modules stripped), forbidding `.set(`, `.set_blocking(`, `.del(`, `.del_blocking(`, `.new_data_set_writer_id`; `test_modules_only_are_skipped` proves the stripping.
- `src/directory.rs` (L3): Profile, Health history, Event trail (Source, Filter, publisher), PublicationList/SubscriptionList, answers per wanted resource (`Answered`/`Timeout`/`Refused`), devices of an application (publisher relation), **observed level** (A-395: L1 REQ-CMP-014; L2 REQ-CMP-015/017; L3 REQ-CMP-018/020/034 … 039, with the missing requirements of the next level), **conformity** (messages, violations, last violations), `Limits` from the Config. Unit tests for every rule; the HTTP side (C-088) reads `get`, `observed`, `conformity`, `limits` – until then `#[allow(dead_code, reason = …)]`.
- `Cargo.toml`: `oec-core` (by path, `strict-validation`) and `serde_json` as normal dependencies (ADR-0023 amendment C-087; no new crate).
- `api/asyncapi.json` (A-396): every topic form of the own AppId by `oec_core::topic::Form::of` (55 channels) plus 13 `Pub` forms of the six collected resources and 5 `Get`s to others: **73 channels, 73 operations, 29 messages**; payloads by the `$id` of the D1 schemas. `src/api_tests.rs`: meta-schema, channels = the forms of the SDK core and the collected resources, every channel has an operation, towards others only `receive` of `Pub` and `send` of `Get`, replies `Get`→`Pub` and `Call`→`Reply`.
- Go (`registry_test.go`, `internal/regtest`): `wire.go` turns the AsyncAPI addresses into patterns (`CheckWire`: every published topic and every subscribed filter of the registry is a channel); `wire_test.go` `TestAPI`. `TestD8Registry` checks the six collecting filters and the wire. **`TestD7Level3` replaces `TestD7Level2`** (A-397): `oec-check test --level 3`, **49 pinned passes, 6 skip, 0 fail** (skips: T7-MON-07/09/17 nothing observed to check, T7-ADR-02 one namespace, T7-SET-04/10 no `Del`-able or RD object on a fresh storage – as the demos); the wire of all registry connections against the AsyncAPI. **`TestReferenceDesignationPersists`**: `Set` of the RD (DataSetWriterId from the registry's PublicationList; ids 1 … 9 are helper DataSets, REQ-MSG-019) → Status Good → restart on the same storage → `Get` returns it.
- `check.sh` header/echo (D7 level 3), `README.md` (state, Config table, layout, checks).

### 5. Gate

See "Checks (final)".

## Agent decisions (G-3.4 b, for information: REVIEW A-391 … A-397)

A-391 D7 refinement for registries; A-392 SDK raw subscriptions; A-393 the registry's Config; A-394 conformity view and attribution; A-395 `Get` policy, devices and observed level; A-396 AsyncAPI scope and wire check; A-397 gate step 8 at D7 level 3 and the RD persistence test.

## Test data for review (G-3.9)

- **R-1:** `oec/conformance/testdata/sim/index.yaml`, variant `T7-GET-04/registry` (positive, `draft`): a conforming Registry that lists an asset (MAM and Health only; every other request about it answered as for an unknown asset) → every D7 test passes. Basis: MG 1.3 (represented entity), MG A1 (listed assets published by the registry), SF-099/SF-108 (unknown asset → Status `Bad…` under the AppId).

## Changes to reviewed items

- None. The reviewed fixtures keep their verdicts (the whole sim suite and the monitor captures pass unchanged); the new variant is `draft`.

## Spec findings

- None (MG 1.3 and A1 are consistent; see §2).

## Dependencies

- Rust: no new crate. `oec-registry` uses `oec-core` (`strict-validation`) and `serde_json` at run time (ADR-0023 amendment C-087): `Cargo.lock` 169 packages (unchanged, all in the SDK lock), normal tree 124 unique crates (was 78). `cargo deny`: ok, no exception.
- Go: standard library only. C: none. npm: none (C-089).

## D7, SDKs, demos

- D7: additive refinement for registries (A-391, ADR-0025 amendment C-087), report version 1 unchanged. `COVERAGE.md`: content unchanged, only the fixture counts.
- Rust SDK: additive raw subscriptions (ADR-0026 §23). C ABI, Go SDK, D8 and the demos unchanged; the demo pins hold (46/49/22).

## Docker objects (own only)

- No Docker was used in C-087 (no image, container, volume or network created). Before = after: none.

## Commits

- `53b5d72` D7 refinement for registries (probe, monitor, sim variant `T7-GET-04/registry`, catalog/design, ADR-0025 amendment C-087).
- `da9d944` SDK raw subscriptions (oec-core + oec, tests, ADR-0026 §23).
- `d40d478` Registry Level 3 (Config, persistent RD, directory L3, conformity view, AsyncAPI 73 channels, gate step 8 with D7 `level3`, ADR-0023/0029 amendments C-087, READMEs).
- This commit: log, REVIEW, REVIEW_DE, STATUS, ROADMAP.

## Checks (final)

- `oec/check.sh` → "all checks passed", rc 0, after the last code change (load ~7 at start, ~4 at the end). Step 8: TestD8Registry (T8-BUS-03 pass), TestD7Level3 (49 pins, 0 fail), TestReferenceDesignationPersists, wire check, Rust API tests. Demo pins unchanged (Logger L2 46, Weather L3 49, Config L1 22).
- `oec/registry/check.sh` alone green before the full gate.
- No Docker objects created or removed in C-087.

## For the PO

- **R-1** to review: the sim variant `T7-GET-04/registry` (draft).
- No D-x (nothing under G-3.4 a), no SF.
- For information: A-391 … A-397 – especially A-391 (D7 refinement), A-392 (additive SDK raw subscriptions) and A-394 (conformity view).

## Handover (G-3.10)

**A new session is recommended** for C-088: this session's context was compacted several times; `prompt.md` (workspace root, German, not committed) holds the start prompt of C-088.

## Next

C-088 – P7 increment 3: HTTP – REST read-only + `api/openapi.json` (OpenAPI 3.2.0 against the vendored meta-schema, paths = router), SSE `/api/stream`, TLS with the application certificate and a self-signed fallback, ports 5798/5799, the hardening contract of ADR-0028 amendment C-083; `hyper` 1 (ADR-0023 amendment C-086); the HTTP side reads the directory of C-087 (entries, devices, observed level, conformity, events).
