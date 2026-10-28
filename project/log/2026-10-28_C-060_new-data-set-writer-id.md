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
