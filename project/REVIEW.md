# Review and decisions – open items for the PO

_Last updated: 2026-10-26, after C-046 addendum 1 (guideline `e0262dd`, register `4b4a5c8`); P4 in progress._

This is the single list of everything that waits for the PO:
- items to **decide**: spec findings (contradictions, gaps or missing definitions in the guideline), open questions, scope;
- items to **review**: test data (G-3.9) and changes to reviewed items.

Engineering decisions (layout, crates, API details within accepted ADRs, libraries within the dependency policy, build and test setup, cycle cuts) are made by the agent since 2026-10-26 (GUARDRAILS G-3.4 (b)). They are listed here **for information**; you can overrule any of them, but no answer is needed.

Each cycle updates this file. Items leave the list once the PO has answered and the decision record has been applied. Details are in the cycle logs; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md), with the same numbering.

**How to answer:** tick the box or write the decision into the "PO" field, or answer in the chat, e.g. "D-1 wie empfohlen".

**Current state:** C-046 started P4 (Rust workspace `oec/sdk/rust`, `oec-core` increments 1–2, JSON export, gate with clippy pedantic and `cargo deny`). In C-046 addendum 1:
- your question on the former D-4 ("can a receiver tell whether a value is masked?") is answered in D-1 below and led to **SF-104** (register `c41b59a`);
- on your instruction, GUARDRAILS G-3.4 now splits PO decisions (a) from agent decisions (b); `project/README.md` and AGENTS.md follow;
- the former C-046 D-1, D-2, D-3 and D-5 are decided by the agent as recommended; ADR-0026 is `accepted (agent, G-3.4 b)`;
- the former R-1 … R-4 (code, export, cargo-deny, lints) are engineering items and now for information only.

No reviewed item changed, coverage stays at 0 gaps. SF-104 is the only open finding.

Log: [C-046 incl. addendum 1](log/2026-10-26_C-046_p4-start.md).

## Overview

| # | Item | Kind | Source | Recommendation | PO |
|---|------|------|--------|----------------|----|
| D-1 | SF-104: optional dnp-encoding makes the `Oi4Identifier` ambiguous | decide | C-046 add. 1 | (a) always mask | ☐ |
| A-1 | ADR-0026: Rust workspace layout and crate roles | info (agent) | C-046 D-1 | accepted | – |
| A-2 | JSON export `oec/spec/export/` as the SDK test-vector contract | info (agent) | C-046 D-2 | accepted | – |
| A-3 | P4 cycle cut and order (increment 3 own cycle; increment 9 before 6–8) | info (agent) | C-046 D-3 | accepted | – |
| A-4 | White space and control characters in Filter values: warning, not error | info (agent) | C-046 D-5 | accepted | – |
| R-1 | `oec-core` code and tests (increments 1–2) | info | C-046 | – | – |
| R-2 | spectool JSON export (format, `omitempty`) | info | C-046 | – | – |
| R-3 | cargo-deny gate (`deny.toml`, offline fallback) | info | C-046 | – | – |
| R-4 | Lint profile: clippy `pedantic`, `doc_markdown` allowed | info | C-046 | – | – |

## Decisions

### D-1 – SF-104: optional dnp-encoding of the `Oi4Identifier`

- **Your question:** can a receiver tell whether a value is masked or natively contains the characters?
- **Answer: no.** There is no marker. The mask sign `,` is itself permitted in an Identification Link, so `A,2F3` is either the native value `A,2F3` or the masked `A/3`.
- **Receivers are not affected:** MG 4.1 forbids decoding an IL; receivers compare it as an opaque string.
- **The real problem is on the generator side.** Because the masking is optional ("may", MG 4.3.1, MG A3, REQ-IL-033, SF-043 e), two generators can derive two different Oi4Identifiers from the same nameplate (e.g. `ABC@home` or `ABC,40home`). MG 4.3 promises that the identifier can be regenerated from the nameplate with the creation rules. In addition, the note in MG 4.3.1 says "mask these characters", while MG A3 Table 19 masks the whole element, including permitted characters such as `@`.
- **Follow-up (topic or payload?):** both alike. base64url in topics and JSON escaping in payloads are lossless; the ambiguity arises before, when the IL string is built, so the two variants differ in the topic (Source, Filter) and in the payload (`ProductInstanceUri`, registry). `"` is not permitted in an IL (MG 4.1), so it must be masked anyway; the free choice exists only for the permitted reserved characters other than `/`. MG 4.1 also says such values "must be masked" – a second contradiction with the "may" of MG 4.3.1/A3, which supports (a).
- **Options (register, see SF-104):**
  - **(a) recommended – always dnp-encode** Model, ProductCode and SerialNumber (ManufacturerUri stays as it is). Elements with only unreserved characters (`A–Z a–z 0–9 - . _ ~`) stay unchanged. Deterministic and reversible; matches Table 19 and the spectool vectors. MG 4.3.1/A3 "may" → "must", REQ-IL-033 MAY → MUST (→ `draft`), SF-043 (e) amended. Existing identifiers whose elements are all unreserved do not change.
  - (b) encode a whole element only when needed (it contains a character not permitted in an IL, or `/`). Deterministic, but not reversible: a native `,40` cannot be told apart.
  - (c) keep "may" and accept the ambiguity; the SDK keeps the opt-in.
- **Impact of (a):** guideline text (MG 4.3.1, A3) by your order, re-baseline, schema version step, REQ-IL-022/033 → `draft`; in `oec-core` `build` always masks and `.dnp_encoded()`/`slash-in-element` are removed. Not blocking C-047.
- **PO:** ☐ (a) ☐ (b) ☐ (c) ☐ other: …

## Agent decisions (for information, G-3.4 b)

### A-1 – ADR-0026: Rust workspace layout and crate roles

- **Where:** `decisions/ADR-0026-rust-sdk-workspace.md`, now `accepted (agent, G-3.4 b)`.
- One Cargo workspace `oec/sdk/rust`, flat crates `oec-core` (sans-IO, internal), `oec` (increment 4) and `oec-ffi` (increment 10); sketch and spikes excluded until `oec` replaces them; `Cargo.lock` committed; `publish = false` until P9; edition 2024, MSRV 1.89.
- `oec-core` errors carry stable codes; `oec` wraps them into `oec::Error::Invalid`.
- §5.2 (`Oi4Identifier` dnp opt-in) stays as it is until D-1 is decided.

### A-2 – JSON export as the test-vector contract

- `spectool render` writes `oec/spec/export/vectors/*.json` and `export/matrix/method-resource.json`; `render -check` keeps them fresh. The SDKs read only the export, so neither Rust nor C needs a YAML parser. The YAML stays the reviewed source.

### A-3 – P4 cycle cut and order

- C-046 increments 1–2 · C-047 increment 3 · C-048 increment 4 · then 5 · then 9 (dev environment with TLS broker) · P5 interleave · then 6–8 · then 10. Six concrete exit criteria in ROADMAP P4. Increment 5/9 needs a container engine (Docker Desktop).

### A-4 – White space in Filter values: warning

- REQ-TOP-020 is a SHOULD NOT. `Topic::parse`/`Topic::new` accept such values and report them via `Topic::warnings()`, like D7 `validate` (warning, exit 0). The publishing side will log a warning and publish anyway.

## Review items (for information)

These are engineering items (G-3.4 b). Remarks are welcome, no answer is needed. Test data stays a PO review (G-3.9); none changed in C-046.

### R-1 – `oec-core` code and tests (increments 1–2)

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

### R-2 – spectool JSON export

- **Where:** `oec/spec/tools` (render), `oec/spec/export/`, `oec/spec/README.md`.
- **Format:**
  - `$comment` ("generated, do not edit"), `source` (the YAML path), then the fields of the Go model;
  - Go zero values are omitted, so a missing `encoded` means "" (e.g. V-IL-116).

### R-3 – cargo-deny gate

- **Where:** `oec/sdk/rust/deny.toml`, `oec/sdk/check.sh` step 0.
- **Policy (ADR-0023 §2):**
  - license allowlist;
  - advisories, unmaintained and yanked crates are errors;
  - one version of each rustls/ring/aws-lc crate;
  - `openssl`, `native-tls`, `rustls-pemfile` banned;
  - crates.io only.
- **Offline:** if the advisory DB cannot be fetched, the gate repeats with the cached DB (`--frozen`). A real violation fails both runs.
- **New prerequisite:** `cargo install cargo-deny --locked` (0.20.2 installed).

### R-4 – Lint profile

- **Where:** `oec/sdk/rust/Cargo.toml` `[workspace.lints]`.
- **Profile:**
  - `unsafe_code = forbid` (`oec-ffi` will opt out per module);
  - `missing_docs`, `unreachable_pub`;
  - clippy `all` + `pedantic` with `-D warnings` in the gate;
  - allowed: `module_name_repetitions`, `must_use_candidate`, and `doc_markdown`, because the docs use guideline terms like ManufacturerUri or PublicationList as prose.

## Next (C-047)

1. P4 increment 3 – message model, schema embedding (`build.rs`, `strict-validation`), payload vectors; first runtime dependencies under `cargo deny`.
2. After your answer on D-1: for (a)/(b) a guideline edit by your order, re-baseline, REQ-IL-033 → `draft`, `oec-core` adjusted.
