# C-058 – P4 increment 8a: consumer core

- **Date:** 2026-10-27
- **Phase:** P4 – SDK core implementation (D2), increment 8a (increment 8 split into 8a and 8b)
- **Baseline:** guideline `2e32a6b` (branch head `d2dc9ca`, findings only), SF-001 … 108 resolved; 1403 inventory items, 441 REQs, 0 gaps; schema `2.0.1-dev` @ `2e32a6b` – all unchanged
- **Agent:** new agent (start prompt from C-057, `prompt.md`)

## Intent

PO order (start prompt): P4 increment 8 as in STATUS "Next step" 1 and ROADMAP P4: the consumer side after the sketch `consumer.rs` (typed subscriptions, discovery helpers, `Get` with correlation over ReplyTo and timeout, async and blocking, reassembly of paginated answers, the Parent check of a Query answer), the SubscriptionList (MG 9.1.9, incl. the `Set` subscriptions of writable DataSets, SF-107), the deletion of the sketch with the move of its REQ walker, and a small SDK example app for D7 `test`. Cut the cycle first.

Baseline checked at the start: `oec/check.sh` "all checks passed"; findings register `d2dc9ca`, no open finding, no drafts.

## Cycle cut (agent decision G-3.4 b, A-120)

Increment 8 does **not** fit one cycle. It holds three review topics:

1. the consumer core – a new protocol role (requester) with its own state machine: subscriptions with SUBACK gating, correlation, pagination, presence, timeouts, the runtime integration;
2. the SubscriptionList – a provider resource on the write path of 7a, fed by the consumer's subscriptions (it needs 1), plus the `Set` entries of the writable DataSets (SF-107);
3. the farewell of the sketch – the REQ walker moves into `oec`, the gate scripts change, the examples become SDK examples; it is only possible once **every** sketch item exists in `oec` (ADR-0020 §1), i.e. after 2 and after `Subscription::configurable`/`interval` and `Consumer::new_data_set_writer_id` (Actions).

The consumer core alone is about 1500 lines of code in `oec-core`/`oec` and 1430 lines of tests plus the routing mode of the test broker. Split:

- **8a (C-058, this cycle):** the consumer core in `oec-core` and `oec`: typed subscriptions, presence/discovery, `Get` (async/blocking) with ReplyTo correlation, timeout, page reassembly and the Query check, `Set`/`Del` with their Status event.
- **8b (C-059, plan):** SubscriptionList (`Get`/`Set`/`Del`, A-92 ends; SF-107 entries, REQ-RES-SUBLIST-016; Profile/PublicationList; persistence, corrupt → `Error::Io`), `Subscription::configurable`/`interval`, the Action `NewDataSetWriterId` (`Consumer::new_data_set_writer_id`), the deletion of the sketch with the move of its REQ walker into `oec` (ADR-0026 §1), `oec/sdk/check.sh` and `oec/check.sh`, the SDK example app (provider + consumer) for D7 `test`. The C-059 agent re-cuts it (e.g. Actions as 8c) if it does not fit.

Order 8a → 8b → 10 (ROADMAP table updated).

## Work

### 1. `oec-core`: sans-IO consumer (`5923aa7`)

Module `oec_core::consumer` (with `received`, `request`, `directory`):

- **`Consumer`** state machine owned by the runtime: `subscribe(filters)`/`subscribe_presence()`/`unsubscribe(id)` with reference-counted MQTT filters; `take_filter_changes()` (SUBSCRIBE/UNSUBSCRIBE for the session), `on_subscribed(results)` (SUBACK), `get(..)`/`write(..)` (requests), `take_outbound()`, `take_completions()`, `next_deadline()`/`expire(now)`, `cancel(id)`, `close()`, `handle_message(topic, payload, now, next_id)` → `Incoming` (deliveries per subscription, presence changes, a decode error).
- **Requests** (`request.rs`): `Address` (ServiceType, AppId, optional Source and Filter, checked against the topic matrix, REQ-TOP-013), `GetParams` (PaginationRequest `PerPage` with Page 0 = all pages, Locale, Query only without Source, DataSetWriterId of the Data for a Metadata `Get`), `WriteEntry` (the one DataSetMessage of a `Set`/`Del`, REQ-BUS-010). The encoders produce exactly the guideline request vectors (V-PAY-002/007/020/046/053/055/061/100); requests carry no `MessageType` in their DataSetMessages, as the vectors (REQ-MSG-029, MG B1).
- **SUBACK gating:** a request is published only after the broker acknowledged the filters its answers arrive on (`Pub/<Resource>/#` of the responder; for `Set`/`Del` and Bad answers its `Pub/Event/+/Status/+`), so no answer is lost (no retained messages on the bus). A refused filter ends the waiting requests with `Outcome::Refused` and is not subscribed again while it is in use.
- **Correlation** (REQ-MSG-015): an answer counts only with ReplyTo = the request's MessageId **and** on the responder's own `Pub/` prefix, and only after the request was published.
- **Pagination** (REQ-PAG-002 … 006, MG 8.3.8): pages are collected by their `Page` and reassembled in page order once `ceil(TotalCount / PerPage)` pages are there (any arrival order, proptest); an answer without Pagination is complete.
- **Query** (REQ-RES-REFDES-015, REQ-QRY-004): answer items whose Parent does not match the Query are dropped; `ReferenceDesignationQuery::matches` is now shared by provider and consumer.
- **Status events** (REQ-RES-EVENT-005): `Set`/`Del` end with the Status of the responder; a `Get` ends early with a Bad Status (e.g. `BadNotFound` under the AppId, REQ-CMP-047, SF-108).
- **Directory** (`directory.rs`, MG 6.3, REQ-LIFE-*): Birth = MAM without ReplyTo → online; Close = Health `NORMAL_0` with HealthScore 0 → offline graceful; Will = Health `FAILURE_1` with HealthScore 0 → offline ungraceful; a Health heartbeat of an unknown application (REQ-TOP-017, SF-106) marks it online and triggers one discovery `Get MAM`; MAM answers fill in the MAM.
- **Typed decoding** (`received.rs`): `Payload` per resource (MAM, Health, Profile, PublicationList, SubscriptionList, Config, Data, Metadata, Event, ReferenceDesignation, raw JSON otherwise); unknown keys ignored (REQ-MSG-046).
- **Time** in Unix milliseconds handed in by the runtime; no clock in the core.

Small additions: `DataNetworkMessage::into_messages`, `Filter::of_kind`.

### 2. `oec`: the consumer API and the runtime (`af68af8`)

Module `oec::consumer` (ADR-0020 amendment C-058):

- `AppHandle::consumer()` → `Consumer` (clonable, also in the prelude with `Resource` and `Target`).
- `Consumer::subscribe(resource, &target, f)` → `Subscription` (unsubscribes when dropped); `on_presence(f)`; `apps()` → `Vec<AppInfo>`.
- `Consumer::get(resource, &target, &GetOptions)` (async) and `get_blocking`; `set(resource, &target, payload_json, &WriteOptions)`, `del(resource, &target, Option<payload_json>, &WriteOptions)` and their `_blocking` forms → the `StatusCode` of the Status event.
- `Target::{any, app, source, filter}`; `GetOptions::{new, timeout, per_page, locale, query, data_set_writer_id}`; `WriteOptions::{new, timeout, data_set_writer_id}`; `DEFAULT_TIMEOUT` 10 s.
- `Received` (accessors `topic`, `service_type`, `app`, `resource`, `source`, `filter`, `data_set_writer_id`, `data_set_writer_name`, `payload`, `payload_json`, `status`, `reply_to`); `Presence::{Online, OfflineGraceful, OfflineUngraceful}` (`#[non_exhaustive]`), `AppInfo`.
- **Writer ids** of a `Set`/`Del`: given by `WriteOptions::data_set_writer_id`, or resolved: Data/Config by a `Get` (exactly one answer → its id; none → `Error::Status(BadNotFound)`; more → `Error::Invalid`); ReferenceDesignation by `Get PublicationList` of the Source with Filter `ReferenceDesignation` (the object may not exist yet); other list resources need the explicit id. A Metadata `Get` without id first asks the Data.
- **Errors:** new `Error::Status(StatusCode)` (Bad answer) and `Error::NotConnected` (offline: nothing is queued, ADR-0020 §7; a lost connection ends pending requests with it); `Error::Timeout`, `Error::Closed` after shutdown, `Error::Invalid` for an incomplete target or a request outside the matrix, `Error::Io(PermissionDenied)` for a refused subscription.
- **Runtime** (`runtime.rs`): `drive_consumer` after each step (SUBSCRIBE/UNSUBSCRIBE, publish requests, the next deadline in the timer); SUBACKs and every received PUBLISH go to the consumer after the provider; `on_connected` after the Birth, `on_disconnected` on loss and shutdown. Callbacks run on the runtime task outside every lock, inside `catch_unwind`. Lock order: consumer state → registry (ADR-0026 §17).
- `block_on` for the `_blocking` forms parks the calling thread; on a tokio runtime thread it returns `Error::Invalid` instead of deadlocking.
- `StatusCode::from_code`, `StatusCode::is_good`.

### 3. Test broker routing mode (`af68af8`)

`tests/testbroker/mod.rs` gets `TestBroker::routing(pki)`: it forwards each PUBLISH (QoS 0) to every connection with a matching subscription (MQTT wildcard match), sends the Will of a connection closed without DISCONNECT, removes the subscriptions of a closed connection and answers UNSUBSCRIBE. The existing (non-routing) mode is unchanged; `route` is a constructor, not a `Behavior` flag (clippy `struct_excessive_bools`).

### 4. Interop test (`7528c52`) and D7 run

`consumer_asks_a_provider` in `oec/tests/interop.rs`: a Level 2 provider (`refdes_app` + child asset) and a Level 1 consumer over Mosquitto 2.1.2 (TLS, client certificates): presence (Birth, Close), `Get MAM` (3 answers, also with `per_page(1)` = 3 pages reassembled), `Get Health`, `Set ReferenceDesignation` of the child (Good), Query of the children (1 answer), a refused Parent (`BadOutOfRange` from the provider), `Del` (Good).

- `oec/dev/interop.sh`: **14 passed, 0 failed, 1 ignored** (`child_app`, as before), about 25 s.
- One-off D7 `oec-check monitor` (external call, Q-009; `--level 2 --max-packet-size 262144 --duration 25s`) during `consumer_asks_a_provider`: **11 pass, 0 fail, 2 skip** (T7-MON-05 observation shorter than 125 s, T7-MON-07 no Action RequestIds), exit 4 (inconclusive by the skips). 37 messages, **0 invalid**: the consumer's `Get`/`Set`/`Del` requests (12 messages) are schema-valid (A-97). Capture not committed (A-73).

### 5. Sketch

Not aligned in this cycle: its consumer items are replaced by `oec::consumer` now and the sketch is deleted in 8b. The differences are recorded as ADR-0020 amendment C-058 C; the sketch still compiles and its REQ walker still runs in the gate.

## Agent decisions (G-3.4 b, for information: REVIEW A-120 … A-127)

- **A-120** cycle cut 8a/8b (above).
- **A-121** requests only after SUBACK of their answer filters; a refused filter ends the request (`Io(PermissionDenied)`) and is not resubscribed while used.
- **A-122** correlation: ReplyTo = MessageId on the responder's `Pub/` prefix only; requests without `MessageType` in their DataSetMessages (as the guideline vectors).
- **A-123** presence rules (Birth/Close/Will/heartbeat, discovery `Get MAM`).
- **A-124** nothing queued offline: `NotConnected`; a lost connection ends pending requests.
- **A-125** writer-id resolution for `Set`/`Del` and the Metadata `Get`.
- **A-126** API (ADR-0020 amendment C-058): builders, `&Target`, typed `Received`, `Error::Status`/`NotConnected`, `get` needs a concrete application.
- **A-127** tests: core with guideline vectors and proptest, runtime with the routing test broker, interop 14 green, D7 11/0/2; no new dependency.

## REQ tracing (main ones)

| REQ | Where |
|---|---|
| REQ-MSG-015 (ReplyTo) | `oec_core::consumer::Consumer::answer`; tests `get_is_answered_and_correlated_over_the_reply_to` |
| REQ-MSG-029, MG B1 (no MessageType in requests) | `consumer/request.rs`; `get_requests_are_the_guideline_vectors`, `write_requests_are_the_guideline_vectors` |
| REQ-TOP-013 (request matrix) | `Address::topic`; `requests_outside_the_matrix_are_refused` |
| REQ-PAG-001 … 006 | `GetParams::per_page`, `add_page`; `paginated_answers_are_reassembled_in_page_order`, proptest `pages_in_any_order_are_reassembled` |
| REQ-RES-REFDES-012/013/015, REQ-QRY-001/004 | `GetParams::query`, `Consumer::finish`; `a_query_answer_is_checked_against_the_parent` |
| REQ-RES-EVENT-005, REQ-CMP-047 | Status handling in `answer`; `set_and_del_end_with_their_status_event`, `unknown_source_ends_the_get_with_bad_not_found` |
| REQ-LIFE-*, REQ-TOP-017 | `consumer/directory.rs`; `the_directory_follows_birth_close_and_will`, `an_application_first_seen_by_its_heartbeat_is_asked_for_its_mam` |
| REQ-BUS-010 (one DataSetMessage per `Set`/`Del`) | `WriteEntry` |
| REQ-MSG-046 (unknown keys) | `received::decode` |

## Defects found

1. **Writer id of a new ReferenceDesignation (own new code, test first):** `Consumer::set` of a ReferenceDesignation resolved the writer id by a `Get`, which is an empty list while the object does not exist → `BadNotFound`. Found by `one_app_writes_to_another`; fixed with `resolve_refdes_writer` (the PublicationList entry of the Source). In `af68af8`.
2. **REQ citations (own new code):** three cited ids did not exist (`REQ-RES-EVENT-012/013`, `REQ-RES-HEALTH-010`); the sketch REQ walker in `oec/check.sh` stopped the gate. Remapped to REQ-RES-EVENT-005, REQ-CMP-047, REQ-TOP-017 (`7528c52`).

No defect in the provider, in D7 or in the guideline.

## Changes to reviewed items

None (no REQ, vector, matrix, fixture or schema changed).

## Spec findings

None. The consumer follows MG 7/8/9 as written; the Query check of REQ-RES-REFDES-015 and the Status event under the AppId (SF-108) worked unchanged against the provider and Mosquitto.

## Dependencies

None (ADR-0023 amendment C-058): the consumer uses `serde_json`, `time`, `tokio` (`sync::oneshot`) and the standard library. `Cargo.lock` unchanged, `cargo deny check` without exception.

## Risks and open technical points

- **Sketch items still missing in `oec`:** `Subscription::configurable`/`interval` (SubscriptionList) and `Consumer::new_data_set_writer_id` (Action `NewDataSetWriterId`, REQ-ACT-*) – 8b; the sketch can only be deleted after them.
- The consumer's subscriptions are not yet listed in a SubscriptionList (8b).
- The callbacks run on the runtime task; a callback that blocks stalls the application (documented).
- A responder that answers a paginated `Get` with an inconsistent `TotalCount` ends in a timeout, not an error.

## Commits

| Commit | Content |
|---|---|
| `5923aa7` | `oec-core` consumer (sans-IO, 14 tests incl. proptest) |
| `af68af8` | `oec::consumer`, runtime, `Error::Status`/`NotConnected`, `StatusCode::from_code`, test broker routing, 4 runtime tests |
| `7528c52` | interop test `consumer_asks_a_provider`; REQ citations remapped |
| (this) | log, STATUS, ROADMAP, ADR-0020/0023/0026 amendments, REVIEW |

## Checks (final)

- `oec/check.sh`: **all checks passed** (spec, schema, SDK gate with clippy pedantic, rustdoc, tests, `cargo deny`; Go tests of `oec/spec/tools`, `oec/conformance`, `oec/dev`).
- Runtime consumer tests: 10 runs in a row green, about 1 s each.
- `oec/dev/interop.sh`: 14 passed, 0 failed, 1 ignored.
- D7 `monitor`: 11 pass, 0 fail, 2 skip, 0 invalid of 37.

## For the PO (review)

Nothing to decide or review: no finding, no new test data with a verdict, no reviewed item changed. A-120 … A-127 are for information; worth a look: **A-120** (cycle cut), **A-124** (no queue offline), **A-126** (API changes against the sketch).

## Handover (G-3.10)

**New agent** for C-059: 8b is a different topic (a provider resource plus the sketch farewell and the example app), this session's context is largely used, and the hand-over is complete in this log, ADR-0026 §17 and ADR-0020 amendment C-058. `prompt.md` is filled with the start prompt (not committed).

## Next

C-059 – P4 increment 8b: SubscriptionList, `Subscription::configurable`/`interval`, Action `NewDataSetWriterId`, deletion of the sketch with the REQ walker moved into `oec`, gate scripts, SDK example app for D7 `test`. Then 10 (`oec-ffi`).
