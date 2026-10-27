# C-054 – P4 increment 6b: provider side, part 2 (Data DataSets, Metadata, publish modes, intervals, Precisions, UUIDv5 DataSetClassId)

- **Date:** 2026-10-27
- **Phase:** P4 – developer SDK (D2), increment 6b (increment 6 split in C-053, A-63; PO order 6b → 7 → 8 → 10)
- **Baseline:** guideline `9044929` (branch head `b94e803`, only `findings/`), unchanged; schema `2.0.1-dev` @ `9044929`
- **Start:** workspace after C-053 (`2e02ea6`), new agent session (G-3.10)

## Intent

STATUS "Next step" 1 and ROADMAP P4 increment 6b asked for:
1. the cycle cut first (6b in one cycle or split), and a check of the order 6b → 7 → 8 → 10;
2. Data DataSets (`Asset::add_data`, `DataSetDef`, field types, a `DataSet` handle that sets state; ADR-0020 §6/§7, sketch `data.rs`) for the application and for assets, sans-IO in `oec-core` on the provider of 6a (ADR-0026 §12), DataSetWriterIds from the existing writer table;
3. Metadata from the field definitions (REQ-RES-META-*), `Get Data`/`Get Metadata` answers (REQ-RES-DATA-007, REQ-CMP-044), the Profile extended by Data/Metadata;
4. PublicationList entries for Data/Metadata with the publish modes (default `ON_REQUEST_FILTER_9`, Metadata fixed, REQ-RES-PUBLIST-007 … 012/026), intervals (REQ-RES-PUBLIST-013/014, REQ-MQTT-019) and Precisions as a dead band (REQ-RES-PUBLIST-015), keeping "state, not messages" (ADR-0020 §7, ADR-0026 §12.3);
5. the DataSetClassId as UUIDv5 of Tag and field structure (ADR-0020 §6, REQ-MSG-013) with the uuid feature `v5` (ADR-0023); `Set PublicationList` stays in increment 7 unless pulled in;
6. tests without network in the gate, runtime tests against the in-process TLS test broker with testable time, vectors where they fit, interop and optionally a D7 `monitor` run (Q-009);
7. dependencies only within ADR-0023;
8. findings SF-107 ff.

Baseline check at the start:
- guideline `feature/spec-findings`, head `b94e803` (only `findings/`) over text `9044929`; SF-001 … 106 resolved, none open;
- 1402 inventory items, 440 REQs, 158 `schema`/`d7-validate` REQs with 0 gaps, no drafts; 29 of 29 entry schemas, 245 payload and 18 `broker.json` vectors; the 18 capture fixtures `reviewed`;
- schema 2.0.0 frozen (`oec-schema-2.0.0`), working state `2.0.1-dev` with `schema.Baseline = 9044929`;
- `oec/check.sh`: all checks passed.

## Cycle cut (agent decision G-3.4 b, A-75)

**6b is not split.**
- Estimated and delivered size: about 2400 lines of source and 1500 lines of tests (`oec-core` + `oec`), one topic (the path of a value from `update` to the bus and to a `Get` answer). Splitting Metadata from Data would leave neither testable end to end: a Data DataSetMessage carries the ConfigurationVersion of its Metadata, and `Get Metadata` depends on the Data entry (REQ-CMP-044).
- **`Set PublicationList` (REQ-RES-PUBLIST-016 …) stays in increment 7.** It needs the `Set` handling, Status events and the `Config` semantics of 7; 6b already lists the entry's `Config` from `Publication::writable`, so 7 only adds the write path.
- **Order 7 → 8 → 10 stays.** 7 (Config, `Set`/`Del`, events, Status `BadNotFound`, `Set PublicationList`, `on_set`) builds on the Data path of 6b; 8 (consumer) reads Profile, PublicationList, Metadata and Data as 6 produces them; 10 needs the complete API. 7 is re-cut at its start (possibly 7a events/Status + `Set PublicationList`, 7b Config).

ROADMAP P4 is updated (6b ✅ C-054, row C-055 (plan) increment 7, exit criterion 3).

## Work

### 1. uuid feature `v5` (`56569ad`)

`uuid` gets the feature `v5` (ADR-0023 amendment C-054, A-86); it adds `sha1_smol` 1.0.1 (BSD-3-Clause, no dependencies). `Cargo.lock` 191 → 192 packages; `deny.toml` unchanged; `cargo deny check` without exception. SHA-1 is only the name-based UUID function of RFC 9562, no security use.

### 2. `oec-core`: Data DataSets on the provider (`7fca3cb`, `b8e227c`)

New modules under `oec_core::provider` (ADR-0026 §13):
- **`provider::data`:** `DataSetSpec`, `FieldSpec`, `BuiltinType`, `Value`, `PublicationSpec`, `Mode`, `Writable`; the DataSetClassId (A-77), the Metadata of a spec (A-78), coercion of values to the field type (A-80), the JSON encoding of values (Int64/UInt64 and NaN/±Infinity as strings), the dead band (A-82). `b8e227c`: integer values keep their natural variant (Int32/UInt32 for types up to 32 bit) so a `Value` read back equals the one set.
- **`provider::sets`:** the state of one DataSet (values, last sent values, due flag, interval deadline, version), its PublicationList entries (Data with Mode/Interval/Precisions/Config, Metadata with `ON_REQUEST_FILTER_9` and the same DataSetWriterId; Metadata omitted for `OFF_0`).
- **`provider::versions`:** the table of ConfigurationVersions keyed by (Source, Tag) with the Metadata fingerprint and the first-sent Timestamp; JSON `{"Version":1,"Entries":[…]}`, `from_json` refuses bad tables.
- **`Provider`:** `add_data`, `update_data`, `data_values`, `remove_data`, `meta_data`, `meta_data_versions`; `on_connected` publishes per source after the 6a resources the Metadata of every DataSet not `OFF_0`, then the complete Data; `poll` publishes changed Metadata before due Data (REQ-RES-META-003) and drives intervals; `poll_timeout` includes the next interval deadline; `handle_message` answers `Get Data`/`Get Metadata` (A-79, A-83). the Profile of every source lists `[MAM, Health, Profile, PublicationList, Data, Metadata]` (the resources the SDK serves); subscriptions add `Get/Data` and `Get/Metadata` (Level 2+).
- Publish forms (A-81): FILTER on `Pub/Data/<Source>/<Tag>`, SOURCE on `Pub/Data/<Source>`, APPLICATION on `Pub/Data`; a group message carries every member with complete values, a mixed group has no DataSetClassId; paginated against the own limit (6a page plan).
- **Tests:** `oec-core/tests/provider_data.rs` 12 tests:
  - `guideline_get_vectors_are_answered_like_the_guideline` – a DataSet built from the Metadata of V-PAY-021 answers the `Get` vectors V-PAY-055 (Data) and V-PAY-020 (Metadata) like V-PAY-058 and V-PAY-021, up to generated ids and times;
  - `every_mode_publishes_its_forms`, `get_data_and_get_metadata_follow_the_modes`, `the_publication_list_describes_data_and_metadata`, `precisions_are_a_dead_band`, `an_interval_publishes_cyclically` (injected time, no sleep), `level_1_publishes_metadata_and_data_without_get`, `metadata_versions_survive_restarts`, `definitions_and_values_are_checked`, `metadata_larger_than_the_bus_is_refused`, `many_data_sets_are_paginated_with_their_class`;
  - `generated_data_payloads_pass_the_strict_schema` (`strict-validation`: every type, every mode).
  - Unit tests: `data` 6 (incl. the pinned DataSetClassId `7d60caaf-6142-5766-b5c2-68226528ed79` of `Oi4Pv`/`Pv` Double), `versions` 3; the 6a tests (`provider.rs` 10, `pagination` 5, `writers` 3) unchanged and green.

### 3. `oec`: public data API, persistence, runtime (`5859857`)

- **New module `oec::data`:** `Field` (`new`, `double`, `unit`, `description`, `range`), `DataSetDef` (`new`, `oi4pv`, `field`, `class_id`, `description`, `publication`), `Publication` (`default_data`, `mode`, `interval`, `precision`, `writable`), `Mode` (with `union`), `Writable`, `Value`, `BuiltinType`, `DataSet` (`tag`, `source`, `update`, `values`, `remove`). In the prelude. `Asset::add_data` for the application (`this()`) and for every asset.
- **`build()`:** loads the persisted version table (`app/metadata-versions`); a corrupt table is `Error::Io` (`InvalidData`), as the writer table (A-76). Both tables are stored after every change.
- **Runtime:** injects monotonic time into the provider (intervals), wakes on every `update`; nothing else changes (the 6a loop already publishes what the provider returns).
- **Tests:** `tests/app.rs` 23 (new: `data_sets_are_declared_and_persisted`, `invalid_data_sets_are_refused`); `tests/runtime.rs` 16 (new: `data_sets_publish_their_state` against the in-process TLS test broker: Metadata before Data, the PublicationList entries, a 50 ms interval DataSet, the dead band, `Get Data`/`Get Metadata` with `ReplyTo`, `remove`).
- Crate docs ("State of P4 … 6b") and `oec/sdk/rust/README.md` updated with this log.

### 4. Interop test (`bc8594c`, `41e66b2`) and a one-off D7 run (`640ffae`)

- New opt-in test `level_2_data_sets_and_metadata` in `oec/tests/interop.rs`: an L2 application with an asset against Mosquitto 2.1.2: the asset's FILTER DataSet `oee` (Precision 0.5) and the application's `Oi4Pv` with `ON_REQUEST | SOURCE` and a 200 ms interval; the probe checks Metadata before Data, the PublicationList entries, `Get Data` and `Get Metadata` answers with `ReplyTo`, and a published change.
- `oec/dev/interop.sh` (Docker): **10 passed, 1 ignored** (`child_app`), about 23 s; broker and volume removed afterwards.
- **One-off D7 check (outside the gate, Q-009: external call only):** `oec-check monitor --bus-storage oec/dev/generated/roots/oec-dev-app --level 2 --max-packet-size 262144 --duration 25s` (built from `oec/conformance`) while that interop test ran:
  - first run: 8 pass, **2 fail**, 3 skip – neither in the SDK:
    - T7-MON-06 (MessageId uniqueness): the test sent `Get Data` and `Get Metadata` with the same MessageId → test fixed (`41e66b2`, also the observed DataSetClassId in `Get Data` instead of an empty one);
    - **T7-MON-11 (REQ-CMP-042) – D7 defect:** the monitor judged an unsolicited `Pub Metadata` as "without content", because it looked for `Messages`; a ua-metadata NetworkMessage carries its content in `MetaData`. The 18 reviewed fixtures only contain solicited Metadata, so this was never exercised. Fixed test first (`TestUnsolicitedMetadataHasContent` replays V-PAY-021 unsolicited; red before, green after): `content()` takes a `meta` flag, `pub()` sets it when the Metadata message has a `MetaData` object (`640ffae`). All conformance tests green, fixtures and their verdicts unchanged.
  - rerun: **10 pass, 0 fail, 3 skip, exit 4 (INCONCLUSIVE)**; the SDK app sent 16 messages, 0 invalid, 1 Birth, 1 Close, 1 subordinate source; skips T7-MON-03 (probe), T7-MON-05 (≥ 125 s), T7-MON-07 (no RequestIds); T7-MON-09/13 … 17 are `planned` in D7.
  - the capture is **not committed** (as A-73): no new test data with a verdict, no R-x.

### 5. Sketch alignment (`db4bc3d`)

`oec/sdk/rust/sketch` follows the implemented API (ADR-0020 amendment C-054, A-84): `Value` without `Json`, `#[non_exhaustive]`, with UInt64/Bytes/LocalizedText; `Field::range`, `Mode::union`, `DataSet::{tag, source, values}`; `DataSet::update` returns `Result<()>`; `LocalizedText` derives `PartialEq`/`Eq`; `examples/device_source.rs` sets Pv and Sv1 first and loops on `update(..).is_ok()`. The sketch tests (incl. the REQ walker) pass.

## Agent decisions (G-3.4 b, for information: REVIEW A-75 … A-86)

- **A-75** cycle cut: 6b in one cycle; `Set PublicationList` in 7; order 7 → 8 → 10.
- **A-76** modules `provider::{data, sets, versions}`, `oec::data`; versions persisted under `app/metadata-versions`, a corrupt table is an error.
- **A-77** DataSetClassId = v5(namespace, JSON `[tag, [[name, builtInTypeId, -1], …]]`), namespace = v5(URL, `https://schemas.openindustry4.com/oec/v2/sdk/data-set-class`); FieldIds = v5(DataSetClassId, name); unit, description, range do not change it; `DataSetDef::class_id` overrides it.
- **A-78** Metadata: EngineeringUnits (UNECE namespace, UnitId −1, DisplayName = unit), EURange; ConfigurationVersion as VersionTime (Major and Minor on a structure change, Minor only otherwise); Timestamp = first sending of the version; Data carries the full MetaDataVersion; Metadata size checked at `add_data`.
- **A-79** Metadata publication on add, connect and version change, before Data, every level, unless `OFF_0`; `Get Metadata` only with Source and Tag of an announced DataSet, else silence (REQ-CMP-044).
- **A-80** Data published and answered only with a complete value set; `update` all or none; coercion to the field type; DataSetWriterName = Source, WriterGroupName = Tag, Timestamp = last change, `ua-keyframe`, no SequenceNumber.
- **A-81** publish forms FILTER/SOURCE/APPLICATION; a group carries every member with complete values and restarts the dead band of the members not due; mixed classes → no DataSetClassId; `OFF_0` listed, never sent, Metadata entry omitted.
- **A-82** `Interval` > 0 cyclic, Precisions ignored, no catch-up, first complete set and connect at once; `Interval` 0 dead band `|new − last| > precision`, non-numeric/non-finite changes always; injected time.
- **A-83** `Get Data` scopes (all / Source / Source+Tag), unknown Tag or incomplete values → empty list (SF-099), unknown Source → silence (A-71); Profile and subscriptions extended; `remove` republishes the PublicationList, the DataSetWriterId and the version stay reserved.
- **A-84** API (ADR-0020 amendment C-054): `update` returns `Result`; `Value::Json` dropped, typed variants for every `BuiltinType`; `Field::range`, `DataSet::{tag, source, values}`, `Mode::union`; `oi4pv` names the field `Pv`; `class_id`/`interval` validated at add; `on_set`/`SetData` in 7.
- **A-85** tests (above); interop 10/1; D7 10/0/3 after the D7 T7-MON-11 fix.
- **A-86** uuid `v5` + sha1_smol (BSD-3-Clause), lock 192, no deny exception.

Recorded in: ADR-0020 amendment C-054 (API), ADR-0026 amendment C-054 §13 (provider semantics), ADR-0023 amendment C-054 (dependencies).

## REQ tracing (main ones)

| Area | REQs |
|---|---|
| Data | REQ-RES-DATA-* (incl. 007 `Get Data`), REQ-MSG-018 (DataSetWriterName = Source), REQ-MSG-032/033/036 (payload encoding) |
| Metadata | REQ-RES-META-* (incl. 003 before Data), REQ-CMP-044 |
| DataSetClassId | REQ-MSG-013 |
| PublicationList modes, intervals, Precisions | REQ-RES-PUBLIST-007 … 015, 017, 026, 027; REQ-MQTT-019 |
| Pagination | REQ-PAG-002/004/005/007 (6a page plan, now with Data) |
| Profile | REQ-RES-PROFILE-* (Data, Metadata) |

## Defects found

- **D7 (`oec/conformance`):** T7-MON-11 treated an unsolicited Metadata as a publication without content (above); fixed test first in `640ffae`. No reviewed fixture changed.
- **Test:** the new interop test reused a MessageId and sent an empty DataSetClassId in `Get Data`; fixed in `41e66b2`.
- **SDK:** none outside the work in progress (clippy pedantic and rustdoc findings, a borrow error and wrong expected values in new tests were fixed before the commits).

## Changes to reviewed items

None: no REQ, vector, matrix entry, catalog entry or fixture changed; schema unchanged (`2.0.1-dev` @ `9044929`). The D7 fix changes code, not test data. The accepted ADR-0020 sketch contract is refined by amendment C-054 (A-84) and the sketch is aligned.

## Spec findings

None. SF-107 stays the next free id. Note without contradiction: the guideline examples carry a real UNECE UnitId (20529 for `%`); the SDK sends −1 (= not given) because `Field::unit` takes only a display text; a `unit_id` can be added later (A-78).

## Risks and open technical points

- **`Set PublicationList` and `on_set`:** `Publication::writable` is only listed as `Config` until increment 7.
- **UnitId:** −1 until an optional `Field::unit_id` exists (no compatibility break).
- **Large groups:** a SOURCE/APPLICATION group sends all members at each due member; fine for tens of DataSets, to be measured with the demos (P6). Pagination keeps every message under the limit.
- **Persisted tables:** writer ids and versions grow with every Tag ever used (never reused by design); bounded by the 65535 DataSetWriterIds.
- **D7:** T7-MON-09/13 … 17 are still `planned`; a recorded-capture fixture kind for SDK runs remains a P5 topic.

## Commits

Workspace (never pushed):
- `56569ad` uuid feature `v5` (Cargo, ADR-0023 scope);
- `7fca3cb` `oec-core` provider data (data, sets, versions, provider, tests);
- `b8e227c` `oec-core` coercion keeps natural integer variants;
- `5859857` `oec` data API, persistence, runtime, tests;
- `bc8594c` interop test `level_2_data_sets_and_metadata`;
- `640ffae` D7 monitor: unsolicited Metadata has content (T7-MON-11);
- `41e66b2` interop test: distinct MessageIds, observed DataSetClassId;
- `db4bc3d` sketch alignment;
- closing commits: crate docs and README; ADR-0020/0023/0026 amendments C-054; this log, STATUS, ROADMAP, REVIEW.md/REVIEW_DE.md.

Size of the code commits (`oec-core`, `oec`, Cargo files): 20 files, +3957/−42 lines (source +2411/−35, tests +1536/−6).

## Checks (final)

`oec/check.sh` at the end of the cycle (2026-10-27): **all checks passed** (spec gate, schema, crosscheck, conformance incl. the new D7 test, SDK gate incl. clippy pedantic, rustdoc, `cargo deny` without exception, `oec/dev`).

`oec/dev/interop.sh` against the local dev broker (Docker, Mosquitto 2.1.2): **10 passed, 1 ignored**, about 23 s.

## For the PO (review)

- **To decide:** nothing (no finding, no scope question).
- **To review:** nothing (no new test data with a verdict, no reviewed item changed).
- **For information:** `REVIEW.md` A-75 … A-86. Worth a look:
  - A-80 (a DataSet goes out only with a value for every field);
  - A-84 (sketch refinements: `update` returns `Result`, no `Value::Json`);
  - A-85 (the D7 defect in T7-MON-11, found by the SDK run and fixed).

## Next

**C-055 – P4 increment 7** (ROADMAP P4): write side – `Set`/`Del` with Status events, the Config resource (incl. masking of sensitive values, GL-ADR-021, and secrets under `/run/secrets/oi4`, GL-ADR-022), `Set PublicationList` (REQ-RES-PUBLIST-016 …, `Publication::writable`), `on_set`/`SetData`, Status `BadNotFound` for a `Get` of an unknown Source (REQ-CMP-047 SHOULD, A-71). Cycle cut first (possibly 7a/7b). Then 8, 10.

Next free ids (unchanged): SF-107, REQ-CMP-048, REQ-RES-PUBLIST-028, REQ-TOP-029, V-PAY-246, V-TOP-138 (valid) / V-TOP-255 (invalid), V-BRK-019, P-129, ADR-0027, Q-031, A-87.

**Agent for C-055 (G-3.10): new agent.**
- The context of this session was compacted several times during C-054.
- 7 is a new increment with its own design questions (write path, Status events, Config with secrets); a fresh session reads the state from the documents instead of a compacted summary.
- The state is handover-ready (G-3.7): this log, ADR-0020/0023/0026 amendments C-054, STATUS "Next step" 1, ROADMAP P4, `REVIEW.md`, the crate docs of `oec`/`oec-core`.

Start prompt: `prompt.md` (German, not committed).
