# C-053 – P4 increment 6a: provider side in `oec` (sources, `Get` answers, Profile, PublicationList, pagination)

- **Date:** 2026-10-27
- **Phase:** P4 – developer SDK (D2), increment 6 (PO order of the C-045 review: increments 6–8 and 10 after the P5 interleave; ROADMAP P4 "Order and cycle cut")
- **Baseline:** guideline `9044929` (branch head `b94e803`, only `findings/`), unchanged; schema `2.0.1-dev` @ `9044929`
- **Start:** workspace after C-052 addendum 2 (`e63f96c`), new agent session (G-3.10)

## Intent

STATUS "Next step" 1 and ROADMAP P4 increment 6 asked for:
1. the cycle cut first (6 in one cycle or split), and a check of the order 6 → 7 → 8 → 10;
2. a resource registry in `oec` (sans-IO parts in `oec-core`, ADR-0026): sources (the application and its subordinate assets), MAM/Health/Profile/PublicationList per `Source`, `set_health` (Health = most severe detail, A-28), store keys of assets (open since C-048);
3. automatic `Get` answers for every `Mode` except `OFF_0` (REQ-RES-PUBLIST-027) with correlation via `ReplyTo` (C-047);
4. Profile and PublicationList with the combined `ON_REQUEST_…` defaults (REQ-RES-PUBLIST-026), stable DataSetWriterIds;
5. publish modes, intervals/Precisions, pagination, `MaxPacketSize` (D7 T7-MON-18 must accept the pagination); the offline behavior (risk of C-050);
6. tests without network in the gate, runtime tests against the in-process TLS test broker, optional interop / D7 capture (data only, Q-009);
7. dependencies only within ADR-0023;
8. findings SF-107 ff.

Baseline check at the start:
- guideline `feature/spec-findings`, head `b94e803` (only `findings/`) over text `9044929`; SF-001 … 106 resolved, none open;
- 1402 inventory items, 440 REQs, 158 `schema`/`d7-validate` REQs with 0 gaps, no drafts; 29 of 29 entry schemas, 245 payload and 18 `broker.json` vectors; the 18 capture fixtures `reviewed`;
- schema 2.0.0 frozen (`oec-schema-2.0.0`), working state `2.0.1-dev` with `schema.Baseline = 9044929`;
- `oec/check.sh`: all checks passed.

## Cycle cut (agent decision G-3.4 b, A-63)

**Increment 6 is split.**
- **6a (this cycle):** the source registry, MAM, Health and Profile per source, the PublicationList with persisted DataSetWriterIds, `Get` answers for these four resources, pagination and `MaxPacketSize`, the offline policy, the asset store keys.
- **6b (C-054):** Data DataSets (`add_data`, `DataSet::set`), their Metadata, the PublicationList entries of Data/Metadata with the publish modes (`APPLICATION`, `FILTER`, `SOURCE`, `ON_REQUEST`), intervals (cyclic publication), Precisions as a dead band, and the UUIDv5 DataSetClassId of ADR-0020 §6 (needs the uuid `v5` feature, ADR-0023).

Reasons:
- 6a alone is about 3200 added lines (the `oec-core` provider with pagination and the writer table about 1900, the `oec` layer about 1200, with tests). The whole increment would not be reviewable as one cycle.
- 6a is a closed unit: every built-in resource of an L2 application without DataSets is served, testable end to end (runtime tests, interop, D7 `monitor`).
- 6b builds on the PublicationList, the writer table and the page plan of 6a without changing them; publish modes and intervals only matter once there is application data.

**Order 6 → 7 → 8 → 10 stays.** 7 (Config, `Set`/`Del`, events, Status `BadNotFound`) needs the PublicationList and `Get` path of 6; 8 (consumer) reads Profile/PublicationList as 6 produces them; 10 (examples, docs, release) needs the complete API. ROADMAP P4 is updated (increment 6 split into 6a ✅ C-053 / 6b C-054, table rows).

## Work

### 1. `oec-core`: sans-IO provider (`35a53eb`)

New module `oec_core::provider` (ADR-0026 §12):
- `Provider` owns the `Lifecycle` of C-049 and the registry of sources. The runtime calls `on_connected` after CONNACK, `poll` at `poll_timeout` and after every change, `handle_message` for every message of `subscriptions`, and publishes what they return in order.
- `add_asset` / `remove_asset` / `set_mam` / `set_health` per Identification Link; the AppId addresses the application itself. Errors: `ProviderError` (unknown or duplicate asset, the AppId as an asset, a MAM whose ProductInstanceUri is not the asset's IL, writer ids exhausted, topic errors).
- **Offline: state, no queue** (ADR-0020 §7, A-65). Changes set dirty flags; `on_connected` publishes the current state of every source.
- **Order** (A-66): Birth → app Health → per asset MAM, Health, (L2) Profile → app Profile → PublicationList. Only the application has the 60 s heartbeat (SF-106).
- **Profile** (L2+): `[MAM, Health, Profile, PublicationList]` per source. **PublicationList**: per source MAM, Health, Profile with `ON_REQUEST_SOURCE_10` (no `Interval`, `Precisions` or `Config`); one DataSetWriterId for all its DataSetMessages (D7 T7-MON-08) (A-67).
- `provider::writers`: the table of DataSetWriterIds keyed by (resource, Source, Filter); 10/11 for the app MAM/Health of the lifecycle; never reused; JSON `{"Version":1,"Entries":[…]}`; `from_json` refuses bad tables (A-68).
- `provider::pagination`: the page plan as a pure function of the DataSetMessage sizes with upper bounds of the overhead; honors PaginationRequest (REQ-PAG-002/004), else one message or all pages with the largest PerPage that fits (REQ-PAG-007); a DataSetMessage larger than the limit gets a page of its own (REQ-PAG-009 is the publisher's duty) (A-70).
- `Get` answers (REQ-RES-PUBLIST-027, REQ-CMP-041/042, REQ-MSG-015): without Source all sources on `Pub/<Resource>`, with Source on `Pub/<Resource>/<Source>[/<Filter>]`; `ReplyTo` of the answer = MessageId of the `Get` (REQ-MSG-015). Unknown Source → no answer (REQ-CMP-047 MUST); the Status event `BadNotFound` (SHOULD) follows in increment 7 (A-71).
- Tests `oec-core/tests/provider.rs` (10, incl. 2 property tests: page-plan invariants for random sizes, limits and requests; every generated payload passes the strict schema with `strict-validation`), unit tests in `pagination` (5) and `writers` (3).

### 2. `oec`: asset API, registry and runtime (`e17a6eb`)

- **New module `oec::asset`:** `AssetDef` (`new`, `with_oi4_identifier`, `revision_counter`), `Asset` (`identification_link`, `mam`, `is_this`, `set_health`, `set_health_details`, `bump_revision_counter`, `remove`), `HealthDetail` (+ `new`), `HealthState` (re-export). In the prelude.
- **`AppHandle`:** `this`, `add_asset`, `asset`, `assets`; `mam` and `identification_link` moved from `App` (which derefs to `AppHandle`); `mam()` returns an owned value (A-72).
- **`build()`:** creates the provider with the persisted writer table (`app/writer-ids`); a corrupt table is `Error::Io` (`InvalidData`), never a silent renumbering (A-68).
- **Asset MAM defaults:** HardwareRevision `"undefined"` (REQ-RES-MAM-008: physical asset with unknown revision), DeviceClass `""` unless set; Health NORMAL_0 / 100 (A-69).
- **Asset RevisionCounter:** SDK-managed (persisted under `assets/<FNV-1a 64 of the IL>/revision-counter`, starting at 0; resolves the C-048 key length problem: an IL has up to 255 characters outside the store-key alphabet) or taken from the VendorNameplate (fixed; `bump` logs and is a no-op; −1 = no counter) (A-69).
- **`set_health`** clears the Details; **`set_health_details`** aggregates to the most severe detail (A-28) without HealthScore; invalid values are logged and ignored (sketch signature `-> ()`).
- **Runtime:** drives the provider (subscriptions handed to the session before the first connection and restored after every CONNACK; Will from the provider; `Get` handling; poll deadlines; wake-up on every change; Close). Limits: `Get` answers against the bus `MaxPacketSize`, unsolicited publications against `min(publish_limit, MaxPacketSize)` (REQ-PAG-005 "own limit") (A-70).
- `From<ProviderError> for Error` → `Error::Invalid`; crate docs updated ("State of P4 … increment 6a").
- **Tests:** `tests/app.rs` 21 tests (new: `assets`, `revision_counters_and_writer_ids_are_persisted`); `tests/runtime.rs` 15 tests (new, against the in-process TLS test broker that can now inject PUBLISH packets: `level_2_publishes_its_sources_and_answers_get` – `Get` subscriptions, order of the 7 publications, writer ids equal to the PublicationList, `Get MAM`/`Get Profile` answers with `ReplyTo`, a `Get Data` not served; `asset_changes_are_published_at_once`; `offline_changes_are_published_as_state_after_reconnect`).
- The sketch doc of `Asset::set_health` is aligned with SF-106 (heartbeat only for `this()`).

### 3. Interop test (`4d8ca70`) and a one-off D7 run

- New opt-in test `level_2_assets_and_get_answers` in `oec/tests/interop.rs`: an L2 application (explicit broker, `reference_designation(false)`) with one asset against Mosquitto 2.1.2; the probe checks the 7 topics of the connect in order (Birth, Health, asset MAM/Health/Profile, app Profile, PublicationList), a `Get PublicationList` answered with `ReplyTo` = its MessageId and 6 entries, and the asset Health `MAINTENANCE_REQUIRED` / 70 after `set_health`.
- `oec/dev/interop.sh` (Docker): **9 passed, 1 ignored** (`child_app`, the helper of the Will test), about 23 s; the broker and its volume removed afterwards.
- **One-off D7 check (outside the gate, Q-009: external call only):** `oec-check monitor` (built from `oec/conformance`) observed the dev broker live (`--level 2 --max-packet-size 262144 --duration 30s`) while the interop test ran:
  - first run: T7-MON-01 failed on the test's own `Get` request (`DataSetClassId ""`), not on the SDK; the request now uses the PublicationList DataSetClassId `217434d6-6e1e-4230-b907-f52bc9ffe152`;
  - rerun: **10 pass, 0 fail, 3 skip, exit 4 (INCONCLUSIVE)**; T7-MON-18 (pagination and MaxPacketSize), T7-MON-04 (MAM and Health for the component and its subordinate asset), T7-MON-08 (DataSetWriterIds) pass; the SDK app showed 10 messages, 0 invalid, 1 Birth, 1 Close, 1 subordinate source;
  - skips: T7-MON-03 concerns only the probe acting as a consumer (`Aggregation/vendor.com/Client/C-1/1`, connected before the observation, no Birth by design); T7-MON-05 needs ≥ 125 s; T7-MON-07 saw no RequestIds;
  - the capture is **not committed**: the monitor fixtures of `oec/conformance/testdata/captures/` are generated from scenarios (`fixtures_test.go -update`), and a recorded SDK capture with timestamps of a real run would need a new fixture kind in D7 (scope of P5, not of 6a). So no new test data with a verdict and no R-x (A-73).

## Agent decisions (G-3.4 b, for information: REVIEW A-63 … A-74)

- **A-63** cycle cut 6a/6b, order 6 → 7 → 8 → 10 kept (above).
- **A-64** sans-IO provider in `oec-core` (`provider`, `provider::pagination`, `provider::writers`); `oec` holds it behind the shared state and persists the writer table.
- **A-65** offline: state, no queue; full state on every connect.
- **A-66** order on connect and on change; heartbeat only for the application (SF-106).
- **A-67** Profile and PublicationList content: four resources, `ON_REQUEST_SOURCE_10`, the PublicationList not listing itself, one DataSetWriterId for the PublicationList.
- **A-68** DataSetWriterIds persisted under `app/writer-ids`, never reused; a corrupt table is an error.
- **A-69** asset defaults (HardwareRevision `"undefined"`, DeviceClass `""`, Health NORMAL_0/100), RevisionCounter SDK-managed or from the nameplate, FNV-1a hashed store keys.
- **A-70** limits and pagination: `Get` answers against the bus `MaxPacketSize`, unsolicited publications against the own lower limit; page plan rules.
- **A-71** `Get` for an unknown Source: silence now, Status `BadNotFound` with increment 7.
- **A-72** API (ADR-0020 amendment C-053): additions `AppHandle::assets`, `Asset::{mam, is_this}`, `HealthDetail::new`; `bump_revision_counter` returns `Result<()>`; `mam`/`identification_link` on `AppHandle`, `mam()` owned; no asset heartbeat.
- **A-73** tests: interop L2 test; one-off D7 `monitor` run 10/0/3, capture not committed.
- **A-74** no new dependency (ADR-0023 amendment C-053); the uuid `v5` feature is expected in 6b.

Recorded in: ADR-0020 amendment C-053 (API), ADR-0026 amendment C-053 §12 (provider semantics), ADR-0023 amendment C-053 (dependencies).

## REQ tracing (main ones)

| Area | REQs |
|---|---|
| Sources, MAM, Health per Source | REQ-CMP-006/007/045, REQ-RES-MAM-006/008/016/018/019, REQ-RES-HEALTH-001/002/004, REQ-TOP-017 (SF-106) |
| Lifecycle order | REQ-LIFE-001/015 |
| `Get` | REQ-RES-PUBLIST-027, REQ-CMP-041/042/047, REQ-MSG-015 |
| Profile, PublicationList | REQ-RES-PROFILE-*, REQ-RES-PUBLIST-006/012/017/026, REQ-MSG-020/021 |
| Pagination, packet size | REQ-PAG-002/004/005/007/009, SF-105 |

## Defects found

- **SDK:** none outside the work in progress. While building, clippy (pedantic, `-D warnings`) flagged a match in `tests/runtime.rs` and rustdoc a broken intra-doc link in `identity.rs` (`App::mam` after the move); both fixed before the commit.
- **Test:** the first D7 run found an invalid `Get` request in the new interop test (empty DataSetClassId); fixed in the test (above).

## Changes to reviewed items

None: no REQ, vector, matrix entry, catalog entry or fixture changed; schema unchanged (`2.0.1-dev` @ `9044929`). The accepted ADR-0020 sketch contract is refined by amendment C-053 (A-72): `bump_revision_counter` returns `Result<()>`, `mam()` is owned and on `AppHandle`, and no heartbeat for represented assets (follows SF-106). The sketch doc of `Asset::set_health` is aligned.

## Spec findings

None. SF-107 stays the next free id. The unknown-Source SHOULD (REQ-CMP-047) and the asset heartbeat (SF-106) are implemented as the guideline says; nothing contradicts it.

## Risks and open technical points

- **Asset store key collision:** two ILs with the same FNV-1a 64 hash would share a RevisionCounter. Probability is negligible (about n²/2⁶⁵); accepted, documented in ADR-0026 §12.8.
- **Unknown Source:** no Status event until increment 7 (SHOULD of REQ-CMP-047).
- **Data DataSets** (6b) will grow the PublicationList and the Profile; the page plan and the writer table are ready for it, the Profile resource list is extended per increment.
- **Large asset counts:** every change of the asset set republishes the whole PublicationList (REQ-RES-PUBLIST-017), paginated; fine for tens or hundreds of assets, to be measured with the demos (P6).
- **D7 capture of SDK runs:** not part of the conformance test data yet (A-73); a recorded-capture fixture kind could be added in P5 (`test`).

## Commits

Workspace (never pushed):
- `35a53eb` `oec-core` provider (provider, pagination, writers, tests);
- `e17a6eb` `oec` layer (asset API, registry, runtime, identity, tests; `publish_limit` in the provider config);
- `4d8ca70` interop test `level_2_assets_and_get_answers`;
- this log, ADR-0020/0023/0026 amendments C-053, sketch doc, STATUS, ROADMAP, REVIEW.md/REVIEW_DE.md.

Size of the code commits: 14 files, +3185/−133 lines.

## Checks (final)

`oec/check.sh` at the end of the cycle (2026-10-27): **all checks passed** (spec gate, schema, crosscheck, conformance, SDK gate incl. clippy pedantic, rustdoc, `cargo deny` without exception, `oec/dev`).

`oec/dev/interop.sh` against the local dev broker (Docker, Mosquitto 2.1.2): **9 passed, 1 ignored**, about 23 s.

## For the PO (review)

- **To decide:** nothing (no finding, no scope question).
- **To review:** nothing (no new test data with a verdict, no reviewed item changed).
- **For information:** `REVIEW.md` A-63 … A-74. Worth a look:
  - A-63 (cycle cut 6a/6b);
  - A-65 (offline: state instead of a queue);
  - A-71 (unknown Source: Status event only with increment 7);
  - A-72 (sketch refinements: `bump_revision_counter` returns `Result`, no asset heartbeat).

## Next

**C-054 – P4 increment 6b** (ROADMAP P4): Data DataSets (`add_data`, `DataSet::set`, field types), Metadata (REQ-RES-META-*, REQ-CMP-044), PublicationList entries of Data/Metadata with the publish modes (`ON_REQUEST_FILTER_9` default, Metadata fixed), intervals (cyclic publication, REQ-RES-PUBLIST-013/014, REQ-MQTT-019), Precisions as a dead band (REQ-RES-PUBLIST-015), the UUIDv5 DataSetClassId (REQ-MSG-013; uuid `v5` feature, ADR-0023 amendment), Profile extended by Data/Metadata. Then 7, 8, 10.

Next free ids (unchanged): SF-107, REQ-CMP-048, REQ-RES-PUBLIST-028, REQ-TOP-029, V-PAY-246, V-TOP-138 (valid) / V-TOP-255 (invalid), V-BRK-019, P-129, ADR-0027, Q-031, A-75.

**Agent for C-054 (G-3.10): new agent.**
- The context of this session was compacted several times during C-053.
- 6b is a new increment with its own design questions (DataSet API, dead band, UUIDv5); a fresh session reads the state from the documents instead of a compacted summary.
- The state is handover-ready (G-3.7): this log, ADR-0020/0023/0026 amendments C-053, STATUS "Next step" 1, ROADMAP P4, `REVIEW.md`, the crate docs of `oec`/`oec-core`.

Start prompt: `prompt.md` (German, not committed).
