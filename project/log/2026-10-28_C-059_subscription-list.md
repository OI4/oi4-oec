# C-059 – P4 increment 8b: SubscriptionList

- **Date:** 2026-10-28
- **Phase:** P4 – SDK core implementation (D2), increment 8b (8b split into 8b and 8c)
- **Baseline:** guideline `2e32a6b` (branch head `d2dc9ca`, findings only), SF-001 … 108 resolved; 1403 inventory items, 441 REQs, 0 gaps; schema `2.0.1-dev` @ `2e32a6b` – all unchanged
- **Agent:** new agent (start prompt from C-058, `prompt.md`)

## Intent

PO order (start prompt): P4 increment 8b as in STATUS "Next step" 1 and ROADMAP P4: the SubscriptionList (MG 9.1.9) with `Get`/`Set`/`Del` on the write path of 7a (A-92 ends), the `Set` entries of the writable Data and Config DataSets (SF-107, REQ-RES-SUBLIST-016), `Subscription::configurable`/`interval`, Profile/PublicationList, persistence; the Action `NewDataSetWriterId` (may be split off as 8c); at the end of 8 the deletion of the sketch with the move of its REQ walker into `oec`; a small SDK example app (provider + consumer) for D7 `test`. Cut the cycle first.

Baseline checked at the start: `oec/check.sh` "all checks passed"; findings register `d2dc9ca`, no open finding, no drafts.

## Cycle cut (agent decision G-3.4 b, A-128)

8b does **not** fit one cycle. It holds two review topics of about the same size:

1. the SubscriptionList – a provider resource on the write path of 7a (`Get`/`Set`/`Del`, Status events, `ReplyTo`, republication, persistence) that is fed by the consumer core of 8a and feeds back into it (deleted filters, processing interval), plus the `Set` entries of the writable DataSets (SF-107);
2. the Action `NewDataSetWriterId` – a different message family (CompactEncoding `ActionNetworkMessage`, `Call`/`Reply` with `CorrelationData`, `TimeoutHint`, `ActionState`, the `Ttl` reservation of ids in the persisted writer table) with a responder in the provider and a requester in the consumer.

The farewell of the sketch can only follow 2, because the sketch is the API contract until `Consumer::new_data_set_writer_id` exists (ADR-0020 §1). The example app needs only 8a and 8b. Split:

- **8b (C-059, this cycle):** SubscriptionList, `Subscription::configurable`/`interval`, the SDK example app under `oec/sdk/rust/oec/examples/`.
- **8c (C-060, plan):** the Action `NewDataSetWriterId` on both sides; then the farewell of the sketch: its examples (hello, device source, config, consumer) ported to `oec`, its C header `sketch/c/oec.h` (the contract of ADR-0021) moved next to the future `oec-ffi`, its REQ walker moved into `oec`, the sketch deleted, `oec/sdk/check.sh` and `oec/check.sh` adapted.

Order 8b → 8c → 10 (ROADMAP table updated).

## Work

### 1. `oec-core`: the SubscriptionList of the provider (`65408d7`)

Module `oec_core::provider::sublist` (ADR-0026 §18):

- **Entries** (MG 9.1.9, REQ-RES-SUBLIST-001/016): one per topic filter of the lasting consumer subscriptions (handed in by the runtime, `Provider::set_consumer_subscriptions`; sorted by filter; Interval = the smallest on the filter, omitted when 0; `CONF_1` only if every subscription on the filter is configurable), then one `NONE_0` entry per writable Config context and per Data DataSet accepted by `Set`, with its `Set` topic (SF-107). One DataSetWriterId for the whole list (MG 9.1.9 note), assigned from Level 2; DataSetWriterName = the AppId.
- **Get** (REQ-RES-SUBLIST-008) with the ListFilter (`<ResourceType>[/<Tag>]`), pagination and Locale as the other lists; a `Get` with the Source of an asset → empty list. The Profile of the application lists the SubscriptionList from Level 2 (REQ-CMP-039), the PublicationList does not.
- **Republication** (REQ-RES-SUBLIST-005): the whole list unsolicited whenever it changed; after a connect only if not empty.
- **Set/Del** on the write path of 7a – **A-92 ends**: every `Set` and `Del` of the SubscriptionList is answered (Status event with `ReplyTo`). Rejections: `BadNotFound` (no ListFilter, Source ≠ AppId), `BadDecodingError`, `BadInvalidArgument` (TopicPath outside the addressed group), `BadNotWritable` (`NONE_0` entry or a changed `Config`, REQ-RES-SUBLIST-004/010), `BadNotSupported` (a TopicPath the application has not subscribed – the SDK does not create subscriptions from outside; **SF-111**). Success: `Set` stores the Interval and publishes the entry with the `ReplyTo`; `Del` marks the entry deleted and publishes the remaining entries of the group.
- **Overrides** (`SubscriptionOverrides`, JSON): what `Set`/`Del` changed (TopicPath, Interval, Deleted); handed back to the consumer side (`take_filter_overrides`) and persisted by `oec`.
- Tests `oec-core/tests/provider_sublist.rs` (7): the entries incl. SF-107, filter addressing, Profile, republication, the guideline vectors V-PAY-072 … 075 answered and schema-checked (the Config of V-PAY-099 as a writable Config DataSet), every rejection, the persisted overrides.

### 2. Consumer side, `oec` glue, persistence (`1665748`)

- `oec_core::consumer`: `configure(id, interval_ms, configurable)`, `settings`, `take_subscriptions` (the lasting subscriptions when changed; the answer subscriptions of requests are not part of it – **SF-109**), `set_filter_overrides`: a deleted filter is unsubscribed (but stays subscribed while a pending request waits on it) and delivers nothing; an Interval drops publications of a topic within the Interval after the last delivered one (per subscription and topic, no trailing delivery). Presence subscriptions stay `NONE_0` without Interval. A consumer filter on a topic of the own AppId is listed (**SF-110**).
- `oec::consumer`: `Subscription::interval(Duration)` and `Subscription::configurable()` (sketch signatures, ADR-0020 amendment C-059); `sync()` exchanges subscriptions and overrides between consumer and provider on connect and after every runtime step (lock order consumer → registry); `Set`/`Del SubscriptionList` resolve the DataSetWriterId by a `Get` of the addressed entries (a deleted entry needs the explicit id).
- `oec::app`: store key `app/subscriptions`; a corrupt table → `Error::Io(InvalidData)` from `build()`.
- Tests: `oec-core/tests/consumer_sublist.rs` (3), `oec/tests/sublist.rs` (2, two applications on the routing test broker: list incl. the provider's `Set` entry, `Set` Interval 60000 throttles, `BadNotWritable` for a non-configurable subscription, `Del` unsubscribes, restore by `Set`; a deleted subscription stays deleted after a restart), `oec/tests/app.rs` (+1, corrupt table). The consumer app in `tests/sublist.rs` is Level 2: a Level 1 application has no SubscriptionList (REQ-CMP-039).

### 3. Example app and interop (`e75d4d8`)

- `oec/sdk/rust/oec/examples/demo.rs`: one Level 2 OTConnector, provider and consumer at once – pump asset with a writable `oee` DataSet (`Set Data` checked: Count ≤ 1,000,000, else `BadOutOfRange`), Config `service`/`Period` (default 5 s) pacing the updates, ReferenceDesignation; presence printout and a configurable `oee` subscription with 1 s Interval. Container storage under `OEC_ROOT` (default `/`), `OEC_DEMO_SECONDS` ends it gracefully (Close). Built in the gate by `cargo clippy --all-targets`. It is the application under test of D7 `test` (ROADMAP P4 dependency).
- Interop `consumer_asks_a_provider` (opt-in): the provider app now also has a configurable subscription; new step `subscription_list()`: `Get`, `Set` Interval 1000, `Del` against Mosquitto 2.1.2.

### 4. Defect found by the gate: answer filter reused before its UNSUBSCRIBE (`55ad42a`, test first)

The first full gate run failed in `oec/tests/consumer.rs::one_app_writes_to_another` (timeout); 4 of 20 runs failed under load. Cause (C-058 code): when a request completed, its answer filters were released and their entries removed; a next request on the same filters, started before the runtime took the filter changes, created new entries in state `Pending`, but the session still held the filters, so no SUBSCRIBE and no SUBACK followed and the request waited until its timeout. Fix: a released filter keeps its entry and SUBACK state until `take_filter_changes` has unsubscribed it; a refused filter used again after its release is subscribed anew; a filter unsubscribed while in use (deleted) waits for a new SUBACK. New core test `an_answer_filter_used_again_before_its_unsubscribe_releases_the_request` (red before, green after); the runtime tests `consumer` and `sublist` 25 × in a row green under parallel load.

### 5. Runs

- `oec/dev/interop.sh` (`OEC_INTEROP_KEEP=1`): **14 passed, 0 failed, 1 ignored** (`child_app`, as before).
- One-off D7 `oec-check monitor` (external call, Q-009; `--level 2 --max-packet-size 262144 --duration 140s`, cert root of the dev environment) while the demo app ran 150 s and the interop consumer test ran against it: **12 pass, 0 fail, 1 skip** (T7-MON-07: no Action RequestIds – 8c), verdict PASS, exit 0; 95 messages, **0 invalid** – the `Get`/`Set`/`Del`/`Pub SubscriptionList` traffic is schema-valid; T7-MON-05 (heartbeat, ≥ 125 s) passes now. The demo ended with exit 0 (Close). Capture not committed (A-73).
- Dev broker stopped afterwards (`docker compose -f oec/dev/compose.yaml down -v`).

### 6. Sketch

Unchanged and still in the gate (REQ walker). `Subscription::{interval, configurable}` now exist in `oec` with the sketch signatures; the sketch remains the contract only for `Consumer::new_data_set_writer_id` and is deleted in 8c.

## Agent decisions (G-3.4 b, for information: REVIEW A-128 … A-136)

- **A-128** cycle cut 8b/8c (above).
- **A-129** the entries of the SubscriptionList (order, Interval, `CONF_1` rule, SF-107 entries, one writer id, Level 2+, Profile/PublicationList).
- **A-130** republication rules.
- **A-131** `Set`/`Del SubscriptionList` semantics and status codes, incl. `BadNotSupported` for a TopicPath not subscribed (SF-111).
- **A-132** consumer side: throttle per subscription and topic without trailing delivery, delete = unsubscribe, overrides only on all-configurable filters, presence `NONE_0`.
- **A-133** persistence `app/subscriptions`, corrupt → `Error::Io`.
- **A-134** API (ADR-0020 amendment C-059): `Subscription::interval`/`configurable`, writer-id resolution for the SubscriptionList.
- **A-135** the example app `examples/demo.rs`.
- **A-136** tests, the answer-filter fix, interop 14 green, D7 12/0/1; no new dependency.

## REQ tracing (main ones)

| REQ | Where |
|---|---|
| REQ-RES-SUBLIST-001, -016 (entries, SF-107) | `provider::sublist`; `get_lists_consumer_subscriptions_and_writable_data_sets` |
| REQ-RES-SUBLIST-003 (Interval) | `Consumer::interval`, `Subscription::interval`; `set_and_del_subscription_list_reach_the_subscription`, `another_app_lists_sets_and_deletes_subscriptions` |
| REQ-RES-SUBLIST-004 (`Config`) | `provider::sublist` checks; `writes_are_checked` |
| REQ-RES-SUBLIST-005 (republication) | `changes_publish_the_list_again` |
| REQ-RES-SUBLIST-008 (Get, filter) | `get_with_filter_addresses_matching_entries` |
| REQ-RES-SUBLIST-010 … 015 (Set/Del) | `guideline_set_and_del_vectors_are_answered` (V-PAY-072 … 075), `writes_are_checked` |
| REQ-CMP-039 (Level 2) | `only_the_application_profile_lists_the_subscription_list` |
| REQ-RES-EVENT-005, REQ-MSG-015 (Status, ReplyTo) | write path of 7a; `oec/tests/sublist.rs` |

## Defects found

1. **Answer filter reused before its UNSUBSCRIBE** (C-058 code in `oec-core/src/consumer.rs`, found by the gate under load; test first, `55ad42a`) – above.
2. **Own new code during the cycle (before commit):** Level 1 consumer apps in the first `tests/sublist.rs` had no SubscriptionList (REQ-CMP-039) – the test apps are Level 2 now; a race of the test helper that rewrote `ca.pem` while another app read it – written only if missing; clippy `double_must_use` on `Subscription::interval`/`configurable`.

No defect in the provider of 6–7, in D7 or in the schema.

## Changes to reviewed items

None (no REQ, vector, matrix, fixture or schema changed). If the PO decides SF-110 as proposed, REQ-RES-SUBLIST-016 follows the text (→ `draft`, R-x).

## Spec findings (register `b35b41e`, all `open`, to decide: REVIEW D-1 … D-3)

- **SF-109** – are the transient answer subscriptions of a requester `SubscriptionList` entries? MG 9.1.9 does not say. Proposed: not listed (one sentence added to MG 9.1.9). The SDK does not list them.
- **SF-110** – MG 9.1.9 (from SF-107): "Other subscriptions to topics of the own `AppId` are not listed", but B1.1.9/V-PAY-072 list the FDS app's subscription to its own `Pub/Data/<FDS>/oee`. The SF-107 proposal meant the request subscriptions (`Get`, `Del`, …). Proposed: "Other subscriptions of the application to request topics of its own `AppId` (e.g. `Get` or `Del`) are not listed." The SDK implements the proposal.
- **SF-111** – must an application create a new entry on `Set` of a TopicPath it has not subscribed (MG 9.1.9 "creates a new entry") when MG 8.3.9 says "depending on implementation and/or technical needs"? Proposed: optional; rejected with Status `Bad` (e.g. `BadNotSupported`). The SDK answers `BadNotSupported`.

A-113 (ReferenceDesignation change increments the RevisionCounter) unchanged, as instructed.

## Dependencies

None (ADR-0023 amendment C-059). `Cargo.lock` unchanged (192 packages), `cargo deny check` without exception.

## Risks and open technical points

- The SDK cannot create subscriptions on request (SF-111); if the PO decides that creation is mandatory, the API needs a handler for unknown TopicPaths (the application decides) – a small addition in 8c.
- The throttle drops publications within the Interval; a "latest value at the end of the Interval" delivery would need a timer per topic (not specified; REQ-RES-SUBLIST-003 only says "processing interval").
- The SubscriptionList writer id is resolved by an extra `Get`; restoring a deleted entry needs the explicit id.
- Still missing from the sketch: `Consumer::new_data_set_writer_id` (Action `NewDataSetWriterId`, 8c).

## Commits

| Commit | Content |
|---|---|
| `021f195` | cycle cut (ROADMAP), log skeleton |
| `65408d7` | `oec-core` provider SubscriptionList (7 tests) |
| `1665748` | consumer core settings and overrides, `oec` glue (`Subscription::interval`/`configurable`, `sync`, writer-id resolution), store key, runtime tests |
| `e75d4d8` | example `oec/examples/demo.rs`, interop extension |
| `55ad42a` | fix: answer filter reused before its UNSUBSCRIBE (test first) |
| guideline `b35b41e` | register: SF-109 … SF-111 (open) |
| (this) | log, STATUS, ROADMAP, ADR-0020/0023/0026 amendments, REVIEW |

## Checks (final)

- `oec/check.sh`: **all checks passed** (spec, schema, SDK gate with clippy pedantic, rustdoc, tests, `cargo deny`; Go tests of `oec/spec/tools`, `oec/conformance`, `oec/dev`).
- Runtime tests `consumer` + `sublist`: 25 runs in a row green under parallel load.
- `oec/dev/interop.sh`: 14 passed, 0 failed, 1 ignored.
- D7 `monitor`: 12 pass, 0 fail, 1 skip, 0 invalid of 95, exit 0.

## For the PO

- **Decide:** D-1 = SF-109, D-2 = SF-110, D-3 = SF-111 (REVIEW).
- **Review:** nothing (no new test data with a verdict, no reviewed item changed).
- For information: A-128 … A-136; worth a look: **A-128** (cut 8b/8c), **A-131** (`BadNotSupported` instead of creating an entry), **A-132** (throttle without trailing delivery).

## Handover (G-3.10)

**New agent** for C-060 (8c): the Action `NewDataSetWriterId` is a different message family (CompactEncoding `ActionNetworkMessage`, `Call`/`Reply`, `CorrelationData`, `TimeoutHint`, the `Ttl` reservation in the writer table), followed by the sketch farewell; this session's context has been compacted several times, and the hand-over is complete in this log, ADR-0026 §18 and ADR-0020 amendment C-059. `prompt.md` is filled with the start prompt (not committed).

## Next

C-060 – P4 increment 8c: Action `NewDataSetWriterId` (responder in the provider, `Consumer::new_data_set_writer_id`), then the farewell of the sketch (examples ported, `sketch/c/oec.h` moved next to the future `oec-ffi`, REQ walker into `oec`, sketch deleted, `oec/sdk/check.sh`/`oec/check.sh` adapted); apply the PO decisions on SF-109 … SF-111. Then 10 (`oec-ffi`).
