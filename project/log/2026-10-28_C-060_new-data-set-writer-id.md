# C-060 – P4 increment 8c: Action `NewDataSetWriterId`, farewell of the sketch

- **Date:** 2026-10-28
- **Phase:** P4 – SDK core implementation (D2), increment 8c (not split)
- **Baseline:** guideline `57ebe91` (branch head `7a2bda2`, findings only), SF-001 … 111 resolved; 1405 inventory items, 441 REQs, 0 gaps; schema `2.0.1-dev` @ `57ebe91` – all unchanged
- **Agent:** new agent (start prompt from C-059, `prompt.md`)

## Intent

PO order (start prompt): P4 increment 8c as in STATUS "Next step" 1 and ROADMAP P4: the Action `NewDataSetWriterId` (MG 8.2.16 … 8.2.20, 8.4, 9.2; REQ-ACT-*) with a responder in the provider (CompactEncoding, `Call`/`Reply`, `CorrelationData`, `TimeoutHint`, `ActionState`, reservation with `Ttl` in the persisted writer table, Profile) and `Consumer::new_data_set_writer_id` after the sketch; then the farewell of the sketch (ADR-0026 §1, §18.1): examples ported to `oec`, C header and C examples moved to where `oec-ffi` (10) keeps the contract, the REQ walker moved into `oec`, the sketch deleted, the gates adapted. Cut the cycle first.

Baseline checked at the start: `oec/check.sh` "all checks passed" (49 s); guideline repo `feature/spec-findings` at `7a2bda2` (text `57ebe91`), no open finding; drafts REQ-RES-SUBLIST-004/010/016 (R-1, not answered yet – left open).

## Cycle cut (agent decision G-3.4 b, A-137)

8c fits **one** cycle, not split. The typed Action model (`oec_core::message::action`: `ActionRequestMessage`/`ActionResponseMessage`, `NewDataSetWriterIdRequest`/`Response`, `TimeoutHint`, `ActionState`, the CompactEncoding checks) exists since C-047, the persisted writer table since C-053 and the SUBACK-gated request machinery of the consumer since C-058. What is new is one responder path, one reservation table, one request kind of the consumer and its public method – one review topic. The farewell of the sketch is mechanical (port, move, delete, gate) and depends on the Action only because the sketch stays the API contract until `Consumer::new_data_set_writer_id` exists (ADR-0020 §1). Order 8c → 10 (ROADMAP table updated).

## Work

### 1. `oec-core`: the responder and the reservations (`3a9e2cc`)

- `provider::action` (new): a `Call` on `Call/NewDataSetWriterId[/<Source>]` of the own AppId (from Level 2; the provider subscribes both filters, REQ-CMP-040, REQ-ACT-020) is answered at once with one final `ActionResponse` (`ActionState` 2) per `ActionRequest` on its ResponseAddress (REQ-ACT-015), with the `RequestId` (REQ-ACT-012) and the `CorrelationData` (REQ-ACT-007) of the request; answering at once also keeps any `TimeoutHint` (REQ-ACT-008). A request that cannot be decoded or addressed (DataSetWriterId 8 / ActionTargetId 1, REQ-ACT-019) or whose ResponseAddress is not the Reply topic of its Call topic gets no response (REQ-ACT-017). The Payload of each `ActionRequest` is decoded on its own (`Value` as `ActionPayload`), so a broken field fails only its request with `BadInvalidArgument` (REQ-ACT-014). A Source the application neither is nor represents → `BadNotFound`, empty Payload (SF-113, open). As many responses per message as fit the MaxPacketSize.
- `provider::writers`: reservations in the persisted writer table (`app/writer-ids`, `Version` 1 with the new optional members `Reserved` and `Last`): a provided id is above every id handed out so far, after 65535 the lowest free one (as every id); reserved for the `Ttl` of 60 s (`WRITER_ID_TTL_SECONDS`, REQ-ACT-023) and never assigned or provided again (SF-112, open); at most 256 open reservations (`MAX_RESERVATIONS`), beyond → `BadResourceUnavailable` (new in `status`). A table written before C-060 is read unchanged.
- Profile: `NewDataSetWriterId` (`Subject::Service`) is listed from Level 2 for the application and every asset (MG 8.3.7).
- Tests `oec-core/tests/provider_action.rs` (10): the guideline Call of MG B1.2.1 (V-PAY-008) answered like V-PAY-009, CorrelationData/RequestId copied, unique reserved ids, reservations survive a restart, unknown Source, invalid resource like the vector, the cap, several requests split by the MaxPacketSize, unanswerable requests ignored, every answer schema-valid (strict); `provider.rs`, `provider_refdes.rs`, `oec/tests/runtime.rs` follow the new Profile and subscriptions.

### 2. Requester: `oec-core` consumer and the public API (`d19c36c`)

- `oec_core::consumer::Consumer::new_data_set_writer_id(resource, address, deadline_ms, now_ms, next_id)`: the `Call` of MG B1.2.1 with `CorrelationData` = the consumer's RequestId as 8 big-endian bytes (REQ-ACT-007), a `RequestId` unique among the pending actions (REQ-ACT-011), `TimeoutHint` = deadline − now, at least 1 ms (REQ-ACT-009); published after the SUBACK of the Reply filter like every request of 8a. Only the final `ActionResponse` of the addressed Responder with the own RequestId and CorrelationData ends it (`Outcome::WriterId` or `Outcome::Status`); intermediate responses are ignored (REQ-ACT-016). A Good response without `DataSetWriterId`/`Ttl` → `BadDecodingError`.
- `oec::consumer`: `Consumer::new_data_set_writer_id(resource, &Target, &ActionOptions) -> Result<ProvidedWriterId>` and `_blocking`; `ActionOptions::{new, timeout}` (default 10 s); `ProvidedWriterId { data_set_writer_id: u16, ttl: Duration }`. The Target needs ServiceType and AppId and no Filter (`Error::Invalid`, REQ-ACT-020); a Bad response → `Error::Status`; no response → `Error::Timeout`.
- Tests: `oec-core/tests/consumer.rs` (+3: the request equals V-PAY-008 and passes the strict schema; end-to-end against the real provider incl. `BadNotFound`; only the final response of the own request ends it, timeout, concurrent RequestIds), `oec/tests/consumer.rs` (+1 `one_app_gets_a_new_writer_id_from_another`: Ttl 60 s, distinct ids ≥ 10, `BadNotFound`, `Invalid` with a Filter, a Level 1 application does not answer → `Timeout`).

### 3. Interop (`4e93aed`)

`consumer_asks_a_provider` also asks the provider for two new ids and for an unknown asset (SF-113) against Mosquitto. The first `oec/dev/interop.sh` run failed four tests with a DataSetWriterId conflict: an earlier run of `examples/demo.rs` with `OEC_ROOT=oec/dev/generated/roots/oec-dev-app` had written its store into the generated root, which the interop tests copy. `Dev::root` now empties `etc/oi4/app` of the copy; the polluted generated root was cleaned. A test-setup defect, not an SDK defect.

### 4. Farewell of the sketch (`f7fe415`)

- `oec/sdk/rust/sketch/` deleted (ADR-0026 §1): every public item of the sketch exists in `oec`.
- Examples ported to `oec/sdk/rust/oec/examples/`: `hello.rs` (16 lines, DX ≤ 20), `config.rs` (L3 OTConnector, Config with `on_set`), `device_source.rs` (L3, `DataSetDef::oi4pv`, Precision, `on_set`, `update`), `consumer.rs` (tokio current-thread runtime with `run_async`, waits for Online, `Get MAM` and the Action against the OTConnector named in argv[1]; `OEC_ROOT`, `OEC_DEMO_SECONDS`). Adaptations to the real API: `&Target`, the `GetOptions` builder, explicit `Bootstrap`, `Presence` arms with `..`, **no broadcast `Get`** (since C-058 `get` needs ServiceType + AppId; discovery is `on_presence`). `demo.rs` stays. All are built by clippy `--all-targets` in the gate.
- C contract moved: `sketch/c/include/oec.h` → `oec-ffi/include/oec.h`, `sketch/c/examples/{hello,logger}.c` → `oec-ffi/examples/` (ADR-0021); `oec/sdk/check.sh` compiles them there as C17. The C ABI has no Action yet (only `oec_get`/`oec_subscribe` on the consumer side) – added with increment 10.
- REQ walker moved to `oec/sdk/rust/oec/tests/citations.rs` (hello world ≤ 20 lines, complete REQ ids only, every REQ id cited under `oec/sdk/` exists and is active; > 1000 citations). Negative check: a planted `REQ-ACT-999` fails it.
- Gates: workspace `exclude = ["spike"]`; `oec/sdk/check.sh` without the sketch step (C header loop on `oec-ffi`); `oec/check.sh` labels; `cargo fmt --all` now covers everything the sketch had excluded. READMEs (`oec/`, `oec/sdk/rust/`, `oec/sdk/micro/sketch/`) and the crate doc of `oec` updated; `oec/sdk/rust/README.md` keeps the API overview of the sketch README (module map, flows incl. the Action, defaults).

### 5. Runs

- Stress: the new runtime tests 25 times in a row while a separate build/test runs as load: 25/25 green. (Parallel processes of the same test binary are no valid method: the `Root` paths are fixed at compile time.)
- Interop: `oec/dev/interop.sh` after the root fix: **14 passed, 1 ignored**.
- D7 `oec-check monitor` (external call, Q-009; built binary, `--level 2 --max-packet-size 262144 --duration 140s`, against `examples/demo.rs` on a copied root and the interop consumer): **13 pass, 0 fail**, no skip of an implemented test (the 6 skips are the tests deferred to `test`, A-58); **T7-MON-07 passes** (3 Calls, 3 Replies); 101 messages, 0 invalid. Capture not committed.
- Smoke test of the ported consumer example against the dev broker next to the demo: `Get MAM` Ok (2 MAMs), new DataSetWriterId Ok (36), both exit 0.

## Agent decisions (G-3.4 b, for information: REVIEW A-137 … A-144)

- **A-137 Cycle cut:** 8c not split (above).
- **A-138 Responder:** both Call filters from Level 2; immediate final response per ActionRequest, no intermediate responses (the TimeoutHint never matters); no response for what REQ-ACT-017 lists or a ResponseAddress that is not the matching Reply topic; per-request Payload decoding; `BadNotFound` for an unknown Source (SF-113 proposal); responses packed up to the MaxPacketSize.
- **A-139 Reservations:** Ttl 60 s; ids above every id handed out (`Last`), wrap to the lowest free; reserved ids persisted and never reused (SF-112 proposal (3) – stricter than "after the Ttl"); cap 256 → `BadResourceUnavailable`; new optional members `Reserved`/`Last` of the writer table (`Version` 1), older tables still read. A `Set` keeps being addressed by its topic and the listed id; the SDK lists an id for everything it accepts a `Set` for, so it never needs a provided id (SF-112 proposal (1)/(2)); no "claim" API.
- **A-140 Profile:** `NewDataSetWriterId` from Level 2 for the application and every asset (it answers for each Source it is or represents).
- **A-141 Requester:** CorrelationData = RequestId big-endian; RequestId unique among pending actions; TimeoutHint = remaining time ≥ 1 ms; only the final response of the own request; Good without fields → `BadDecodingError`.
- **A-142 API (ADR-0020 amendment C-060):** `new_data_set_writer_id(resource, &Target, &ActionOptions) -> ProvidedWriterId` instead of the sketch's `(service_type, app, resource, timeout) -> (u16, Duration)`; default timeout 10 s.
- **A-143 Farewell of the sketch:** examples ported with the adaptations above, `oec.h` + C examples in `oec-ffi` (Action in C with increment 10), walker in `oec/tests/citations.rs`, gates adapted.
- **A-144 Tests and runs:** core 10 + 3, runtime + 1, interop extended (+ `Dev::root` fix), stress 25/25, interop 14/1, D7 `monitor` 13/0 with T7-MON-07; no new dependency.

## REQ tracing (main ones)

REQ-CMP-040 (Call/Reply + NewDataSetWriterId: L2 SHOULD, L3 MUST → from Level 2), REQ-ACT-006 (ResponseAddress), REQ-ACT-007 (CorrelationData), REQ-ACT-008/009 (TimeoutHint), REQ-ACT-011 (RequestId), REQ-ACT-012 (fields of the ActionResponse, empty Payload with Bad), REQ-ACT-014 (Status, per-request failure), REQ-ACT-015 (one final response), REQ-ACT-016 (intermediate responses), REQ-ACT-017 (no response), REQ-ACT-019 (DataSetWriterId 8, ActionTargetId 1), REQ-ACT-020 (no Filter), REQ-ACT-022 (new id), REQ-ACT-023 (Ttl reservation); vectors V-PAY-008/009 (MG B1.2.1).

## Defects found

- Wrong citation (fixed in the final commit): the doc of `provider_action.rs::correlation_data_and_request_id_are_copied` named REQ-ACT-004/013 instead of REQ-ACT-007/012 (the walker checks existence, not meaning).
- Test setup (interop): the demo had polluted the generated dev root; `Dev::root` now starts from an empty application store (`4e93aed`). No SDK defect, no D7 defect.

## Changes to reviewed items

None. No REQ, vector, matrix row or fixture changed; the schema is unchanged. The notes on REQ-ACT-017 and REQ-ACT-023 named in SF-112/113 follow only when the PO decides them (then `draft`, R-x).

## Spec findings (guideline repo `feature/spec-findings`, register `19c1a95`, both `open`, to decide: REVIEW D-1, D-2)

- **SF-112** – what a provided `DataSetWriterId` does in the "upcoming set request" (MG 8.4.1/9.2.1 vs. 8.2.3/9.1.8): which id a creating `Set` carries, what happens if the Responder already lists an id, whether an unused id may be handed out again. Proposed: a `Set` carries the listed id, a provided id only for a combination not listed yet (it becomes the id of the created DataSet); unused ids are not handed out again while never-used ids are left; one sentence for MG 8.4.1.
- **SF-113** – the answer to `Call/NewDataSetWriterId/<Source>` for an unknown Source: no response (MG 8.1.3 "cannot address") or a final `ActionResponse` with `BadNotFound`? Proposed: `BadNotFound` with an empty Payload; "cannot address" means DataSetWriterId/ActionTargetId; one sentence for MG 9.2.1.

The guideline text is unchanged (`57ebe91`), so no re-baseline. Next free id SF-114.

## Dependencies

None new (ADR-0023 amendment C-060): the examples use tokio (`rt`, `sync`, `time`), already a normal dependency of `oec` with these features. `Cargo.lock` unchanged (192 packages); `cargo deny check` without exception.

## Risks and open technical points

- SF-112/113 may be decided differently: (2)/(3) of SF-112 change only the reservation rules in `writers.rs`; a "no response" for SF-113 is one branch in `action.rs` and two test expectations.
- The C ABI (`oec.h`) does not have the Action yet; increment 10 adds it with the generated header.
- R-1 (REQ-RES-SUBLIST-004/010/016) still waits for the PO.

## Commits

- `1784362` log skeleton, cycle cut (A-137), ROADMAP.
- `3a9e2cc` responder + reservations (`oec-core`).
- `d19c36c` requester core + `oec` API + tests.
- `4e93aed` interop with the Action, `Dev::root` fix.
- `f7fe415` farewell of the sketch.
- Guideline repo `19c1a95` (register only): SF-112, SF-113 open.
- This log, STATUS, ROADMAP, ADR-0020/0021/0023/0026 amendments, REVIEW/REVIEW_DE: final commit of the cycle.

## Checks (final)

`oec/check.sh`: "all checks passed" (spec gate, schema, SDK gate incl. clippy pedantic for both feature sets and `crypto-ring`, tests, rustdoc, `cargo deny`, C17 headers, Go tests of `spec/tools`, `conformance`, `dev`).

## For the PO

- **Decide D-1 (SF-112)** and **D-2 (SF-113)** – the SDK follows both proposals already.
- **R-1** (REQ-RES-SUBLIST-004/010/016, C-059 review) is still open.
- A-137 … A-144 for information; worth a look: A-139 (reservations), A-142 (API), A-143 (no broadcast `Get` in the examples, C Action deferred).

## Handover (G-3.10)

**New agent** for increment 10 (`oec-ffi`, the C ABI): a different language boundary (C ABI, cbindgen or hand-written header, ownership across FFI, callbacks into C, the C examples as tests) with its own review topics; this session's context has been compacted several times, and the hand-over is complete in this log, ADR-0021 (amendment C-060), ADR-0026 §19 and ADR-0020 amendment C-060. `prompt.md` is filled with the start prompt (not committed).

## Next

1. P4 increment 10: `oec-ffi` – the C ABI after ADR-0021 on top of `oec` (incl. `new_data_set_writer_id`), the header `oec-ffi/include/oec.h` generated or checked against the contract, the C examples built and run against the test broker. Cut the cycle first.
2. Apply the PO's answers to D-1/D-2 (guideline text, register, re-baseline, REQ-ACT-017/023 notes → R-x) and R-1.

Next free ids: SF-114; REQ-CMP-048, REQ-RES-PUBLIST-028, REQ-RES-SUBLIST-017, REQ-TOP-029; V-PAY-246, V-TOP-138 / V-TOP-255, V-BRK-019, P-129; ADR-0027, Q-031, A-145.
