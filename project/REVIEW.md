# Review and decisions – open items for the PO

_Last updated: 2026-10-26, after C-047 addendum 1 (guideline `734c560`, register `fce5cf5`); P4 in progress. **Nothing open for the PO.**_

This is the single list of everything that waits for the PO:
- items to **decide**: spec findings (contradictions, gaps or missing definitions in the guideline), open questions, scope;
- items to **review**: test data (G-3.9) and changes to reviewed items.

Engineering decisions (layout, crates, API details within accepted ADRs, libraries within the dependency policy, build and test setup, cycle cuts) are made by the agent since 2026-10-26 (GUARDRAILS G-3.4 (b)). They are listed here **for information**; you can overrule any of them, but no answer is needed.

Each cycle updates this file. Items leave the list once the PO has answered and the decision record has been applied. Details are in the cycle logs; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md), with the same numbering.

**How to answer:** tick the box or write the decision into the "PO" field, or answer in the chat, e.g. "D-1 wie empfohlen".

**Current state:** C-046 started P4 (Rust workspace `oec/sdk/rust`, `oec-core` increments 1–2, JSON export, gate with clippy pedantic and `cargo deny`).
- Addendum 1: on your instruction, GUARDRAILS G-3.4 splits PO decisions (a) from agent decisions (b); the former C-046 D-1, D-2, D-3, D-5 are decided by the agent (A-1 … A-4), ADR-0026 is `accepted (agent, G-3.4 b)`; the former R-1 … R-4 are for information (now A-5 … A-8). Your question on the former D-4 led to **SF-104**.
- Addendum 2: your answer "MG 4.1 is leading and unambiguous" decided **SF-104** (register `726f2a3`): the `Oi4Identifier` elements are always dnp-encoded. Implemented on our side: REQ-IL-022/033, new V-PAY-245, oracle and D7, `oec-core` (A-9).
- Addendum 3: your review ("D-1 reviewed und akzeptiert -> übernehmen, R-1 und R-2 akzeptiert") is applied: MG 4.3.1/A3 aligned in `734c560`, **SF-104 resolved** (`fce5cf5`), re-baseline on `734c560` (1399 inventory items, REQ-IL-033 quotes remapped), REQ-IL-022/033 and V-PAY-245 `reviewed`. No drafts, no open finding.

- C-047 (P4 increment 3): `oec-core` message model (all envelopes, helper DataSets, MetaData, Actions with correlation, all resource payloads), schema embedding via `build.rs` with `schema::Validator` behind `strict-validation`, all 245 payload vectors as tests with the same verdicts and REQ ids as the schema gate, first runtime dependencies (serde, serde_json, time, uuid, jsonschema) under `cargo deny` without exceptions. No reviewed item changed, no new vector, no spec finding, so there is nothing to decide or review; A-10 … A-16 are for information.
- C-047 addendum 1 (PO instruction): new GUARDRAILS G-3.10. At every cycle end the agent decides between a new agent session and the same one, and for a new one writes the start prompt into `prompt.md`. Decision for C-048: new agent; `prompt.md` is filled.

Coverage stays at 0 gaps; schema unchanged (`2.0.1-dev` @ `734c560`).

Logs: [C-047](log/2026-10-26_C-047_message-model.md), [C-046 incl. addenda 1–3](log/2026-10-26_C-046_p4-start.md).

## Overview

| # | Item | Kind | Source | Recommendation | PO |
|---|------|------|--------|----------------|----|
| A-1 | ADR-0026: Rust workspace layout and crate roles | info (agent) | C-046 D-1 | accepted | – |
| A-2 | JSON export `oec/spec/export/` as the SDK test-vector contract | info (agent) | C-046 D-2 | accepted | – |
| A-3 | P4 cycle cut and order (increment 3 own cycle; increment 9 before 6–8) | info (agent) | C-046 D-3 | accepted | – |
| A-4 | White space and control characters in Filter values: warning, not error | info (agent) | C-046 D-5 | accepted | – |
| A-5 | `oec-core` code and tests (increments 1–2) | info | C-046 R-1 | – | – |
| A-6 | spectool JSON export (format, `omitempty`) | info | C-046 R-2 | – | – |
| A-7 | cargo-deny gate (`deny.toml`, offline fallback) | info | C-046 R-3 | – | – |
| A-8 | Lint profile: clippy `pedantic`, `doc_markdown` allowed | info | C-046 R-4 | – | – |
| A-9 | SF-104 consequences: `oec-core` always masks, D7/oracle R-3 only dnp-encoded | info (agent) | C-046 add. 2 | – | – |
| A-10 | Tolerant typed decoder: unknown keys ignored, types and invariants enforced, patterns left to the schema | info (agent) | C-047 | – | – |
| A-11 | Encoder with the fixed content masks (MessageType always, `ReplyTo` `""`, DataSetClassId from the resource) | info (agent) | C-047 | – | – |
| A-12 | Lossless values: DateTime/ByteString keep their text, `Number` for int-or-float, lowercase Guids, sorted maps | info (agent) | C-047 | – | – |
| A-13 | Action API: correlation derived from the Call topic, `response`/`done`/`failed` | info (agent) | C-047 | – | – |
| A-14 | Shapes of Config (flattened groups), Data (raw object + Oi4Pv view), SubscriptionList `TopicPath` (string) | info (agent) | C-047 | – | – |
| A-15 | Schema embedding (`schema.go` version, tolerant flavor in memory) and dependency feature sets; gate runs both feature sets | info (agent) | C-047 | – | – |
| A-16 | C-047 cycle cut re-checked: no split | info (agent) | C-047 | – | – |

## Decisions and review items

None open. Answered in the C-046 review (2026-10-26) and applied in C-046 addendum 3:
- D-1 – align MG 4.3.1/A3 with SF-104: ☑ yes → guideline `734c560`, SF-104 resolved (`fce5cf5`), re-baseline.
- R-1 – REQ-IL-033 (MUST) and REQ-IL-022: ☑ accepted → `reviewed` (REQ-IL-033 quotes remapped to the aligned text, mechanical).
- R-2 – V-PAY-245: ☑ accepted → `reviewed`.

## Agent decisions (for information, G-3.4 b)

### A-1 – ADR-0026: Rust workspace layout and crate roles

- **Where:** `decisions/ADR-0026-rust-sdk-workspace.md`, `accepted (agent, G-3.4 b)`.
- One Cargo workspace `oec/sdk/rust`, flat crates `oec-core` (sans-IO, internal), `oec` (increment 4) and `oec-ffi` (increment 10); sketch and spikes excluded until `oec` replaces them; `Cargo.lock` committed; `publish = false` until P9; edition 2024, MSRV 1.89.
- `oec-core` errors carry stable codes; `oec` wraps them into `oec::Error::Invalid`.
- §5.2 (`Oi4Identifier`) follows SF-104 since addendum 2 (A-9).

### A-2 – JSON export as the test-vector contract

- `spectool render` writes `oec/spec/export/vectors/*.json` and `export/matrix/method-resource.json`; `render -check` keeps them fresh. The SDKs read only the export, so neither Rust nor C needs a YAML parser. The YAML stays the reviewed source.

### A-3 – P4 cycle cut and order

- C-046 increments 1–2 · C-047 increment 3 · C-048 increment 4 · then 5 · then 9 (dev environment with TLS broker) · P5 interleave · then 6–8 · then 10. Six concrete exit criteria in ROADMAP P4. Increment 5/9 needs a container engine (Docker Desktop).

### A-4 – White space in Filter values: warning

- REQ-TOP-020 is a SHOULD NOT. `Topic::parse`/`Topic::new` accept such values and report them via `Topic::warnings()`, like D7 `validate` (warning, exit 0). The publishing side will log a warning and publish anyway.

## Engineering items (for information)

These are engineering items (G-3.4 b). Remarks are welcome, no answer is needed.

### A-5 – `oec-core` code and tests (increments 1–2)

- **Where:** `oec/sdk/rust/oec-core/src/` (about 2,100 lines incl. unit tests) and `tests/vectors.rs`.
- **Increment 1:**
  - base64url (strict, canonical);
  - `IdentificationLink` (all violations at once, stable codes);
  - `dnp_encode`;
  - `Oi4Identifier`.
- **Increment 2:**
  - `ServiceType`/`Method`/`Resource`/`Service`/`Subject` (closed sets);
  - `Form` (29 allowed combinations);
  - `Filter` (Tag, Context.Name, Event, list filter, level encoding);
  - `Topic` (parse/build, rule, reply, warnings);
  - `TopicFilter` (builder, MQTT matching).
- **Tests:** 22 unit tests and 6 vector tests: 29 IL, 15 dnp, 13 Oi4Identifier, 151 topic vectors, the payload topics and the 66 matrix rows. All pass. The error classification is the same as D7 (`grammar` before `identification-link`).
- **Not yet:** coverage numbers (`cargo-llvm-cov` needs `llvm-tools`, which the Homebrew Rust lacks).

### A-6 – spectool JSON export

- **Where:** `oec/spec/tools` (render), `oec/spec/export/`, `oec/spec/README.md`.
- **Format:**
  - `$comment` ("generated, do not edit"), `source` (the YAML path), then the fields of the Go model;
  - Go zero values are omitted, so a missing `encoded` means "" (e.g. V-IL-116).

### A-7 – cargo-deny gate

- **Where:** `oec/sdk/rust/deny.toml`, `oec/sdk/check.sh` step 0.
- **Policy (ADR-0023 §2):**
  - license allowlist;
  - advisories, unmaintained and yanked crates are errors;
  - one version of each rustls/ring/aws-lc crate;
  - `openssl`, `native-tls`, `rustls-pemfile` banned;
  - crates.io only.
- **Offline:** if the advisory DB cannot be fetched, the gate repeats with the cached DB (`--frozen`). A real violation fails both runs.
- **New prerequisite:** `cargo install cargo-deny --locked` (0.20.2 installed).

### A-8 – Lint profile

- **Where:** `oec/sdk/rust/Cargo.toml` `[workspace.lints]`.
- **Profile:**
  - `unsafe_code = forbid` (`oec-ffi` will opt out per module);
  - `missing_docs`, `unreachable_pub`;
  - clippy `all` + `pedantic` with `-D warnings` in the gate;
  - allowed: `module_name_repetitions`, `must_use_candidate`, and `doc_markdown`, because the docs use guideline terms like ManufacturerUri or PublicationList as prose.

### A-9 – SF-104 consequences (agent)

- `oec-core`: `Oi4Identifier::build` always dnp-encodes; `dnp_encoded()` and the SDK-only violation `slash-in-element` are removed (also from the P3 sketch); ADR-0026 §5.2 rewritten.
- Oracle and D7 `validate` (rule R-3 of REQ-RES-MAM-006): a generated Oi4Identifier in the MAM must carry the dnp-encoded elements; before, the plain value was accepted too. No reviewed vector changed its result. ADR-0019 R-3 got a note.

### A-10 … A-16 – P4 increment 3 (C-047, agent)

- **Where:**
  - `oec/sdk/rust/oec-core/src/{message,resource,schema}`, `build.rs`, `tests/payload.rs`, `tests/model.rs`;
  - ADR-0026 amendment C-047 §7, ADR-0023 amendment C-047;
  - log [C-047](log/2026-10-26_C-047_message-model.md) with the REQ tracing tables (96 REQ ids cited).
- **A-10 Tolerant typed decoder:**
  - Decoding ignores keys beyond the guideline (REQ-MSG-046).
  - It refuses wrong JSON types, missing required fields, values outside closed sets and broken model invariants: DataSetClassId, MessageType, reserved DataSetWriterIds, MetaDataVersion vs. Major/MinorVersion, Good `Status`, Action fields.
  - Patterns and cross-field rules stay with the schema and the D7 rule layer.
  - On the vectors, 63 of 143 invalid payloads are refused by the decoder; all 143 are caught by decoder, schema or rule layer.
- **A-11 Encoder:**
  - The SDK writes only the keys of the fixed content masks (REQ-MSG-001/003/004/016/017).
  - It always sends the DataSetMessage `MessageType` and `ReplyTo` (`""` without a trigger).
  - It sets the DataSetClassId of a predefined resource itself; publisher-defined ids refuse reserved and predefined values (REQ-MSG-014).
- **A-12 Values:**
  - `DateTime` and `ByteString` keep their text, so CorrelationData is echoed byte-identical (REQ-ACT-007).
  - Int-or-float fields use `serde_json::Number`.
  - Guids only lowercase hyphenated.
  - Sorted maps (JSON key order carries no meaning).
- **A-13 Actions:**
  - `ActionRequestMessage::new` takes the Responder and the `ResponseAddress` from the Call topic (REQ-ACT-005/006).
  - `response()` copies PublisherId, RequestorId and CorrelationData.
  - `done()`/`failed()` build the matching ActionResponse; a Bad Status gets an empty Payload (REQ-ACT-012/014).
- **A-14 Shapes:** Config groups are flattened maps; Data is a raw object with an `Oi4Pv` view; SubscriptionList `TopicPath` stays a string.
- **A-15 Embedding and dependencies:**
  - `schema::VERSION`/`BASELINE` come from `oec/schema/schema.go` (`2.0.1-dev` @ `734c560`), as ordered for this cycle.
  - The 45 strict sources are embedded; the tolerant flavor is derived in memory.
  - Minimal features: uuid without `v4`/`v5` until ids are generated.
  - 20 crates (81 with `strict-validation`), `cargo deny` without exceptions.
  - The gate runs clippy, tests and doc in both feature sets.
- **A-16 Cycle cut:** no split; the model, schema hook and vector parity form one unit. The plan after C-047 is unchanged.
- **Tests:** default 27 unit + 13 integration tests, strict 30 + 15; the parity with the schema gate held at the first run.

## Next (C-048)

1. P4 increment 4 – crate `oec`: `App`/`AppBuilder`, `Level`, `Bootstrap` (explicit + container storage contract), `Store`, `Error`; the sketch API moves into `oec`; new runtime dependencies thiserror and log.
