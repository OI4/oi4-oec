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

_(in progress)_
