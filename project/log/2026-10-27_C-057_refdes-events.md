# C-057 – P4 increment 7c: ReferenceDesignation, Syslog bridge, Generic events

- **Date:** 2026-10-27
- **Phase:** P4 – developer SDK (D2), increment 7c (not split, A-110)
- **Baseline:** guideline `2e32a6b` (branch head `d2dc9ca`, only `findings/`), unchanged; schema `2.0.1-dev` @ `2e32a6b`
- **Start:** workspace after the C-056 review addendum 2 (`3a3ef99`), new agent session (G-3.10)

## Intent

STATUS "Next step" 1 and ROADMAP P4 increment 7c asked for:
1. the cycle cut first (does 7c fit one cycle; order 7c → 8 → 10);
2. ReferenceDesignation per sketch `asset.rs` (`Asset::on_reference_designation`): `Get`/`Set`/`Del ReferenceDesignation` on the write path of 7a (check, the handler decides, the SDK answers with Pub + `ReplyTo` or a Status event), persisted in the store (corrupt → `Error::Io` at `build()`), A-92 replaced for ReferenceDesignation, Profile/PublicationList (ADR-0026 §12–§15);
3. the Syslog bridge (`AppBuilder::syslog`, `log` → `Pub/Event/<Source>/Syslog/<Level>`): level mapping per MG 8.3.6, limits/throttling, no feedback loop, nothing secret in events (GL-ADR-021, A-103);
4. Generic events (`Asset::emit_event`, `CAT_GENERIC_99`) on the Event writer of 7a, with their PublicationList entries;
5. SF-107 only in increment 8, SF-108 done;
6. tests without network in the gate, runtime tests against the in-process TLS test broker, the interop test extended and run once, optionally D7 `monitor`;
7. dependencies only within ADR-0023; findings SF-109 ff.

Baseline check at the start:
- guideline `feature/spec-findings`, head `d2dc9ca` (only `findings/`) over text `2e32a6b`; SF-001 … 108 resolved, no open finding;
- 1403 inventory items, 441 REQs, 158 `schema`/`d7-validate` REQs with 0 gaps, no drafts; 29 of 29 entry schemas, 245 payload and 18 `broker.json` vectors; the 18 capture fixtures `reviewed`;
- schema 2.0.0 frozen (`oec-schema-2.0.0`), working state `2.0.1-dev` with `schema.Baseline = 2e32a6b`;
- `oec/check.sh`: all checks passed.

## Cycle cut (agent decision G-3.4 b, A-110)

**7c is not split.** ReferenceDesignation, Generic events and Syslog events are three small additions on two existing mechanisms: the two-phase write path of 7a (`Handled::*`, completion outside the lock, Status events with `ReplyTo`) and the Event writer with its persisted DataSetWriterIds and PublicationList entries.
- **Order inside the cycle:** ReferenceDesignation → Generic events → Syslog bridge. The bridge uses the event path (queue, writer ids, entries) built for the Generic events.
- **Size at the end:** source +1716/−45 lines (incl. unit tests in the modules), integration tests +1860/−327 lines (incl. the test broker moved to its own module); within one cycle.
- **Order 7c → 8 → 10 unchanged.** 8 (consumer side, SubscriptionList, SF-107/REQ-RES-SUBLIST-016) is a new topic and needs the complete provider; 10 (`oec-ffi`) needs the complete API.

## Work

### 1. `oec-core`: ReferenceDesignation (`5e5e93d`)

- New module `provider::refdes`:
  - `StoredReferenceDesignations`: the persisted table (`{"Version":1,"Entries":[{"Source","ReferenceDesignation"}]}`, duplicate Source refused);
  - `ReferenceDesignationRequest`: the decided `Set`/`Del` that waits for the application;
  - `complete_reference_designation`;
  - the `Get` answer with and without Source, and with the Query of the direct children.
- `Provider::with_reference_designations` offers the resource from Level 2:
  - subscriptions `Get`/`Set`/`Del ReferenceDesignation/#`;
  - the Profile lists `ReferenceDesignation` after Metadata;
  - one PublicationList entry per source: `ON_REQUEST_SOURCE_10`, Config `NONE_0`, WriterGroupName `ReferenceDesignation`.
- `provider::write` routes `Set`/`Del ReferenceDesignation` to `Handled::SetReferenceDesignation` when the resource is offered. This replaces A-92 for RD. `Set`/`Del SubscriptionList` stay `BadNotFound` until 8.
- New `answer()`: a request completed after its asset was removed meanwhile is answered under the `AppId` (consistent with SF-108). The Config and Data completions use it too.
- Tests `tests/provider_refdes.rs` (5):
  - the guideline vectors V-PAY-041 … 046 and V-PAY-053/054 replayed;
  - the application decides;
  - every status code of an invalid write;
  - Profile/PublicationList;
  - a strict-schema check of every answer.
- `provider_write.rs`: `Set Data` completed after the asset was removed.

### 2. `oec-core`: Syslog and Generic events (`074a7db`)

- New module `provider::events`.
- `Provider::with_syslog`:
  - threshold;
  - DataSetWriterIds for the eight Syslog levels of the `AppId`;
  - `emit_syslog`: Number = PRI = Facility 1 × 8 + severity, no Description, Details `MSG`/`HEADER` per MG B1.1.6, RFC 3164 header with HOSTNAME from the MAM and TAG = the application's name.
- `emit_generic`:
  - `CAT_GENERIC_99`, free `<Level>`, at most 16 Levels per source;
  - every Level gets a persisted DataSetWriterId and a PublicationList entry (`FILTER_4`, Config `NONE_0`).
- The queue holds at most 256 events and is drained at `poll`. Offline nothing is queued. Events are removed with their asset.
- Syslog PublicationList entries:
  - Config `MODE_1`, Mode `FILTER_4` or `OFF_0`;
  - Debug and Informational are off by default (REQ-TOP-024);
  - `Set PublicationList` switches a Level between the two Modes (REQ-TOP-027); other Modes → `BadNotWritable`;
  - the result is persisted with the overrides (`app/publications`).
- Tests `tests/provider_event.rs` (5): the shapes of V-PAY-064/065, the checks, offline and Level 1, switching and persistence.

### 3. `oec`: API, runtime and the `log` bridge (`80a8023`)

- **API** (ADR-0020 amendment C-057):
  - `Asset::on_reference_designation` returns `Result`;
  - `Asset::reference_designation`;
  - `AppBuilder::reference_designation(true)` at Level 1 → `Error::Invalid` at `build()`;
  - `ReferenceDesignation`/`Designation`/`DesignationParent` re-exported;
  - `GenericEvent::new(level, number).description(..).details_json(..)`;
  - `Asset::emit_event(GenericEvent) -> Result`;
  - `AppBuilder::syslog`, `event::syslog`, `event::install_bridge`, `event::MAX_MESSAGE_BYTES`.
  - The prelude has `ReferenceDesignation` and `GenericEvent`.
- **Persistence:**
  - store key `app/reference-designations`; corrupt → `Error::Io` (`InvalidData`) at `build()`; a failing write is logged.
  - An accepted change increments and persists the RevisionCounter of the source and republishes its MAM (A-113).
- **Runtime:** completes `Handled::SetReferenceDesignation` (handler outside every lock, `catch_unwind` → `BadInternalError`) and syncs `log::max_level` after every handled message.
- **Bridge** (`oec::event`):
  - **Install and target:** one process-global `log::Log`, installed at the first `build()` if no logger is set. The first live application is the target.
  - **Level mapping:** Error→Error, Warn→Warning, Info→Informational, Debug/Trace→Debug.
  - **No feedback loop:**
    - records of `oec`, `oec_core`, `rustls`, `tokio` and `mio` are excluded;
    - a thread-local guard is held while a record is being published.
  - **Rate limit:** burst 50, then 10/s; the number dropped is reported as one Warning.
  - **MSG:** `TAG[pid]: text`, cut to 1024 bytes on a character boundary.
- **Tests:**
  - `tests/runtime.rs`:
    - `reference_designation_is_set_got_deleted_and_persisted`;
    - `generic_events_are_published_while_connected`.
  - `tests/syslog.rs`: its own binary, because the logger is process-global. `log_records_become_syslog_events` checks the mapping, the exclusions, Set PublicationList switching, `event::syslog` and the rate limit.
  - Unit tests in `event`.
  - The in-process TLS test broker moved from `tests/runtime.rs` to `tests/testbroker/mod.rs`, shared by both binaries.

### 4. Interop test (`89fa268`) and D7 run

- New opt-in test `level_2_reference_designation_and_generic_events` against Mosquitto:
  - `Set` RD → Pub with `ReplyTo` + Status Good;
  - `Get` with and without Source;
  - `Del` → Status Good;
  - `Del` again → `BadNotFound`;
  - a Generic event with its PublicationList entry.
- `level_2_assets_and_get_answers` expects 20 PublicationList entries instead of 12 (the 8 Syslog entries of the application).
- Two test defects fixed on the way, both in the test, not in the SDK:
  - **Wrong DataSetClassId:** the probe sent the SubscriptionList DataSetClassId (`e5d68c47…`) in its RD requests instead of `27a75019-164a-496d-a38b-90e8a55c2cfa`.
  - **Missing `WriterGroupName`:** the `Set` requests lacked `"WriterGroupName": ""` (MG 9.1.10 example; A-97: interop probes send schema-valid requests). The D7 run showed it.
- **`oec/dev/interop.sh`: 13 passed, 0 failed, 1 ignored** (`child_app`), about 24 s; run three times, the last after the final refactor.
- **One-off D7 run** (external call, Q-009): `oec-check monitor --level 2 --duration 20s` during the new interop test.
  - Result: **9 pass, 0 fail, 4 skip**, exit 4 (inconclusive), 0 invalid messages in 23.
  - Skips:
    - T7-MON-03: the SDK was connected before observation began;
    - T7-MON-05: needs ≥ 125 s;
    - T7-MON-07: no RequestIds;
    - T7-MON-18: MaxPacketSize unknown, because no `--max-packet-size` was given.
  - T7-MON-04 and T7-MON-19 ("no Syslog Debug by default") pass.
  - The capture had the RD Set/Pub/Get/Del, the Status Good/Bad events and `Generic/Maintenance`. Not committed (A-73).

### 5. Sketch alignment (`531066e`)

- `sketch/src/asset.rs`:
  - `ReferenceDesignation` with `Designation`/`DesignationParent`;
  - `on_reference_designation` and `emit_event` return `Result`;
  - new `Asset::reference_designation`.
- `sketch/src/event.rs`: `GenericEvent` as a builder with Level; docs name the `log` bridge. `install_bridge` is described in the docs, not sketched, because the sketch has no dependencies.
- `sketch/src/app.rs`: `AppBuilder::reference_designation`; the `syslog` docs warn about secrets in logs.

### 6. ADRs (`e6386a1`)

- ADR-0020 amendment C-057 (API).
- ADR-0023 amendment C-057 (no new dependency).
- ADR-0026 amendment C-057 §16 (semantics).

## Agent decisions (G-3.4 b, for information: REVIEW A-110 … A-119)

- **A-110:** cycle cut (above).
- **A-111 ReferenceDesignation semantics:**
  - offered from Level 2, not published on connect;
  - `Get` with the Query of the direct children (character-by-character IL compare);
  - `Set` replaces the object as a whole; a `Set` of the identical object is `Good` without a write;
  - `Del` without a stored object → `BadNotFound` without asking the application;
  - payload invalid → `BadDecodingError`.
- **A-112 Persistence of RD:** `app/reference-designations`; corrupt → `Error::Io`; the object stays stored while its asset is removed; a failing write only logged.
- **A-113 RD change → RevisionCounter + 1:** interpretation of MG 5 (REQ-RES-MAM-018).
  - MG 5: "configuration data of the asset as defined in OPC UA Part 100". An RD set over the bus is engineering data of the asset, so the SDK treats it as configuration.
  - The guideline does not name the RD explicitly. No SF: the reading follows the text, and the PO can overrule (see "For the PO").
- **A-114 `answer()`:** a request completed after its asset was removed is answered under the `AppId` (SF-108 consistent); applies to RD, Config and Data.
- **A-115 Generic events:**
  - Level rules: non-empty, a valid topic level, at most 16 per source;
  - Details must be a JSON object;
  - a Level is registered at first use even offline;
  - offline or with 256 waiting → dropped with `Ok` (no queueing for later);
  - allowed at Level 1;
  - `ua-event` in the DataSetMessage, `ua-data` in the NetworkMessage (REQ-MSG-029).
- **A-116 Syslog events:** PRI as Number, `MSG`/`HEADER`, Notice and more severe on by default, Informational/Debug `OFF_0`, switchable via `Set PublicationList` (only `FILTER_4`/`OFF_0`), persisted.
- **A-117 `log` bridge:**
  - installed at `build()` if no logger is set; `install_bridge` keeps a local logger;
  - SDK/transport targets excluded, reentrancy guard;
  - `max_level` follows the switched levels;
  - rate limit 50/10 per s, MSG cut to 1024 bytes;
  - documented: never log secrets, and `env_logger::init()` after `build()` panics (use `install_bridge`).
- **A-118 API** (ADR-0020 amendment C-057): see 3. The sketch is aligned.
- **A-119 Tests:** see 1–4. No new dependency.

A-92 is now replaced for ReferenceDesignation. Still `BadNotFound` until 8: `Set SubscriptionList` and `Del SubscriptionList`.

## REQ tracing (main ones)

| Area | REQs |
|---|---|
| RD content, persistence | REQ-RES-REFDES-001, 002 (016: the Parent content is the application's data) |
| `Get` RD | REQ-RES-REFDES-003 … 005, 011 … 014, REQ-PAG-* |
| `Set`/`Del` RD | REQ-RES-REFDES-006, 007, 009, 010; REQ-RES-EVENT-005; REQ-CMP-020, REQ-CMP-043 (Del without object → `BadNotFound`) |
| Event content | REQ-RES-EVENT-001, 002, 011; REQ-MSG-029 |
| Syslog | REQ-RES-EVENT-007, 008; REQ-TOP-024, 027 |
| Generic | REQ-RES-EVENT-009 (010 is a documentation duty of the application) |
| RevisionCounter | REQ-RES-MAM-018 (A-113) |
| Profile, PublicationList | REQ-RES-PROFILE-002, REQ-RES-PUBLIST-003/006/011/012/016/017 |

Not by the SDK:
- REQ-RES-REFDES-008: ACL, broker side (D8).
- REQ-RES-REFDES-015: the requester's duty, consumer side (increment 8).
- REQ-RES-EVENT-006: Status events for application-specific states. Possible later with an API; not asked for in 7c.

## Defects found

- **SDK:** none in the code of earlier cycles. One test expectation of earlier cycles changed: the interop test now expects 20 instead of 12 PublicationList entries (the 8 Syslog entries of the application).
- **Tests:**
  - the Generic runtime test waited for a PublicationList republish that never came, because the Level was already in the first list; rewritten;
  - the Syslog test built a `Set PublicationList` entry with the wrong DataSetWriterId;
  - the two interop probe defects (above).
- **D7:** none.
- Smaller issues fixed before the commits:
  - clippy pedantic: `needless_pass_by_value` → `GenericEvent::into_core(self)`; `struct_field_names`; `too_many_lines` in tests → helpers.
- **Observation, not a defect:** the SDK accepts requests with a wrong DataSetClassId. The guideline binds the sender to the right DataSetClassId but asks no receiver check; the SDK routes by topic and decodes the payload strictly. Kept as is; noted as a risk for D7 (a sender check belongs there).

## Changes to reviewed items

None:
- no REQ, vector, matrix entry, D7 catalog entry or fixture changed;
- schema unchanged (`2.0.1-dev` @ `2e32a6b`).

The accepted ADR-0020 sketch contract is refined by amendment C-057 (A-118), and the sketch is aligned.

ROADMAP: the dependency column of the C-056 row named the log instead of the dependency; corrected to `regex` (a slip in the C-056 closure).

## Spec findings

None new. Next free id: SF-109.
- A-113 (RD and the RevisionCounter) is an interpretation of MG 5 that follows its text. If the PO sees it as a gap, it becomes SF-109.
- SF-107 is decided and comes with increment 8.

## Risks and open technical points

- **Global logger:**
  - The bridge is the process-global `log` logger. An application that installs its own logger after `build()` gets an error, or a panic with `env_logger::init()`. This is documented; `install_bridge(Some(local))` is the way to keep both.
  - With several `App`s in one process only the first live one gets the Syslog events. That is the normal case of one application per container.
- **`tracing`:** not bridged directly. `tracing` users can forward through `tracing-log`; there is no new dependency for it.
- **Secrets in logs:** the SDK cannot tell a secret in a log text. The API docs warn (GL-ADR-021); the SDK's own records are not bridged at all.
- **Handler on the runtime task:** as for `Set Data`/`Set Config`, a blocking `on_reference_designation` delays the bus loop (documented).
- **D7:** T7-MON-09/13 … 17 still `planned`. The 20 s run cannot see T7-MON-05.

## Commits

Workspace (never pushed):
- `5e5e93d` `oec-core` ReferenceDesignation;
- `074a7db` `oec-core` Syslog and Generic events;
- `80a8023` `oec` API, runtime, `log` bridge, tests, test broker module;
- `89fa268` interop test `level_2_reference_designation_and_generic_events`;
- `531066e` sketch alignment;
- `e6386a1` ADR-0020/0023/0026 amendments C-057;
- closing commit: this log, STATUS, ROADMAP, REVIEW.md/REVIEW_DE.md.

Guideline repo: no commit (no finding, no re-baseline).

## Checks (final)

`oec/check.sh` at the end of the cycle (2026-10-27): **all checks passed**. It covers:
- the spec gate, schema and crosscheck;
- conformance;
- the SDK gate: clippy pedantic, rustdoc, `cargo deny` without exception, crypto-ring pass;
- `oec/dev`.

`oec/dev/interop.sh` against the local dev broker (Docker, Mosquitto 2.1.2): **13 passed, 0 failed, 1 ignored**, about 24 s.

## For the PO (review)

- **To decide:** nothing. No finding, no open question.
- **To review:** nothing. No new test data with a verdict, no reviewed item changed.
- **For information:** `REVIEW.md` A-110 … A-119. Worth a look:
  - **A-113:** a `Set`/`Del ReferenceDesignation` increments the RevisionCounter, as a Config change does. If you read MG 5 differently, I will make SF-109 from it.
  - **A-117:** the `log` bridge – what becomes a Syslog event, what is excluded, the rate limit, and that applications must not log secrets.
  - **A-115:** events are not queued while offline.

## Next

**C-058 – P4 increment 8** (ROADMAP P4): the consumer side.
- Typed subscriptions, discovery helpers, `Get` with correlation and timeout, pagination reassembly.
- The SubscriptionList with `Set`/`Del SubscriptionList` (A-92 ends there). It lists the `Set` subscriptions of the writable Data and Config DataSets (SF-107, REQ-RES-SUBLIST-016).
- At the end of 8 the sketch is deleted and its REQ walker moves into `oec` (ADR-0026 §1).
- Then 10 (`oec-ffi`).

Next free ids:
- SF-109;
- REQ-CMP-048, REQ-RES-PUBLIST-028, REQ-RES-SUBLIST-017, REQ-TOP-029;
- V-PAY-246, V-TOP-138 (valid) / V-TOP-255 (invalid), V-BRK-019, P-129;
- ADR-0027, Q-031, A-120.

**Agent for C-058 (G-3.10): new agent.**
- The context of this session was compacted several times during C-057.
- Increment 8 is a new topic, the consumer side, with its own reading: MG 9.1.9 SubscriptionList, the `Get`/pagination rules from the requester's view, REQ-RES-REFDES-015. It also ends with the deletion of the sketch.
- A fresh session reads the state from the documents instead of a compacted summary.
- The state is handover-ready (G-3.7): this log, ADR-0020/0023/0026 amendments C-057, STATUS "Next step" 1, ROADMAP P4, `REVIEW.md` (nothing open), the crate docs.

Start prompt: `prompt.md` (German, not committed).
