# Review and decisions – open items for the PO

_Last updated: 2026-10-26, after C-046 addendum 2 (guideline `e0262dd`, register `726f2a3`); P4 in progress._

This is the single list of everything that waits for the PO:
- items to **decide**: spec findings (contradictions, gaps or missing definitions in the guideline), open questions, scope;
- items to **review**: test data (G-3.9) and changes to reviewed items.

Engineering decisions (layout, crates, API details within accepted ADRs, libraries within the dependency policy, build and test setup, cycle cuts) are made by the agent since 2026-10-26 (GUARDRAILS G-3.4 (b)). They are listed here **for information**; you can overrule any of them, but no answer is needed.

Each cycle updates this file. Items leave the list once the PO has answered and the decision record has been applied. Details are in the cycle logs; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md), with the same numbering.

**How to answer:** tick the box or write the decision into the "PO" field, or answer in the chat, e.g. "D-1 wie empfohlen".

**Current state:** C-046 started P4 (Rust workspace `oec/sdk/rust`, `oec-core` increments 1–2, JSON export, gate with clippy pedantic and `cargo deny`).
- Addendum 1: on your instruction, GUARDRAILS G-3.4 splits PO decisions (a) from agent decisions (b); the former C-046 D-1, D-2, D-3, D-5 are decided by the agent (A-1 … A-4), ADR-0026 is `accepted (agent, G-3.4 b)`; the former R-1 … R-4 are for information (now A-5 … A-8). Your question on the former D-4 led to **SF-104**.
- Addendum 2: your answer "MG 4.1 is leading and unambiguous" decided **SF-104** (register `726f2a3`): the `Oi4Identifier` elements are always dnp-encoded. Implemented on our side: REQ-IL-022/033 (`draft`), new V-PAY-245 (`draft`), oracle and D7, `oec-core` (A-9).

Coverage stays at 0 gaps; schema unchanged (`2.0.1-dev`).

Log: [C-046 incl. addenda 1–2](log/2026-10-26_C-046_p4-start.md).

## Overview

| # | Item | Kind | Source | Recommendation | PO |
|---|------|------|--------|----------------|----|
| D-1 | Order: align the guideline text MG 4.3.1/A3 with SF-104 ("may" → "must") | decide | C-046 add. 2 | yes | ☐ |
| R-1 | REQ-IL-033 (MAY → MUST) and REQ-IL-022 (`draft`) | review | C-046 add. 2 | accept | ☐ |
| R-2 | V-PAY-245 (`draft`): MAM with unmasked `ABC@home` | review | C-046 add. 2 | accept | ☐ |
| A-1 | ADR-0026: Rust workspace layout and crate roles | info (agent) | C-046 D-1 | accepted | – |
| A-2 | JSON export `oec/spec/export/` as the SDK test-vector contract | info (agent) | C-046 D-2 | accepted | – |
| A-3 | P4 cycle cut and order (increment 3 own cycle; increment 9 before 6–8) | info (agent) | C-046 D-3 | accepted | – |
| A-4 | White space and control characters in Filter values: warning, not error | info (agent) | C-046 D-5 | accepted | – |
| A-5 | `oec-core` code and tests (increments 1–2) | info | C-046 R-1 | – | – |
| A-6 | spectool JSON export (format, `omitempty`) | info | C-046 R-2 | – | – |
| A-7 | cargo-deny gate (`deny.toml`, offline fallback) | info | C-046 R-3 | – | – |
| A-8 | Lint profile: clippy `pedantic`, `doc_markdown` allowed | info | C-046 R-4 | – | – |
| A-9 | SF-104 consequences: `oec-core` always masks, D7/oracle R-3 only dnp-encoded | info (agent) | C-046 add. 2 | – | – |

## Decisions

### D-1 – Order: align MG 4.3.1 and MG A3 with SF-104

- **Your decision (2026-10-26):** "MG 4.1 is leading and unambiguous" – values with characters that are not permitted or have a special function in the URL syntax must be masked.
- **Consequence:** MG 4.3.1 applies all of MG 4.1 to the Oi4Identifier and names the dnp-encoding as the means. The dnp-encoding keeps exactly the unreserved characters `A–Z a–z 0–9 - . _ ~`; every other character is not permitted or reserved. So masking the offending characters equals dnp-encoding the whole element: one Oi4Identifier per nameplate, reversible (option (a) of SF-104). Elements of unreserved characters only do not change.
- **Open:** MG 4.3.1 ("the dnp-encoding … may be used to mask these characters") and MG A3 ("may be built in a way that masks", "may be dnp-encoded") still read as optional. Changing guideline text needs your order (G-3.4 a). I asked in the session; you were not available.
- **Proposed text (register SF-104):** MG 4.3.1 note: "If a value of `Model`, `ProductCode` or `SerialNumber` contains other characters than `A`–`Z`, `a`–`z`, `0`–`9`, `-`, `.`, `_` and `~`, the value must be dnp-encoded (A3), so that the `Oi4Identifier` can be regenerated from the same values." MG A3: "may be built" → "are built", "may be dnp-encoded" → "are dnp-encoded".
- **With your order:** guideline commit, SF-104 `resolved`, re-baseline (the schema baseline follows; version stays `2.0.1-dev`), the MG quotes of REQ-IL-033 follow.
- **Without:** SF-104 stays `decided`; our implementation already follows your decision.
- **PO:** ☐ yes, align the text ☐ no, keep the text ☐ other: …

## Review items

### R-1 – REQ-IL-033 and REQ-IL-022 (`draft`)

- **Where:** `oec/spec/requirements/il.yaml`, view `requirements/view/il.md`.
- **REQ-IL-033** "Oi4Identifier elements are dnp-encoded", **MUST** (was MAY, "optional"): the generator writes Model, ProductCode and SerialNumber dnp-encoded; values of unreserved characters only stay unchanged; every other character is not permitted, reserved (REQ-IL-017) or the separating `/` and is masked; one Oi4Identifier per nameplate, every `,` in an element is a mask sign. New MG 4.1 quote; refs + SF-104. Verification stays `manual` (D7 checks it heuristically through REQ-RES-MAM-006, R-2).
- **REQ-IL-022:** "may be masked" → "are masked" (REQ-IL-017, REQ-IL-033).
- **Mechanical:** REQ-IL-032 notes point to the new keyword (stays `reviewed`).
- **PO:** ☐ accepted ☐ remarks: …

### R-2 – V-PAY-245 (`draft`)

- **Where:** `oec/spec/vectors/payload/constructed.yaml`, view `vectors/view/payload-constructed.md`.
- **Content:** V-PAY-210 as the MAM of an asset with the SerialNumber `ABC@home`; ProductInstanceUri, DataSetWriterName and the topic carry `provider.com/FieldDataService/FDS-001/ABC@home` unmasked → invalid, REQ-RES-MAM-006 (R-3 heuristic). Valid would be `…/ABC,40home`. `@` is permitted in an IL, so no other REQ fires. Listed in `schema/tests/rule-layer.yaml` (the schema cannot see it).
- **PO:** ☐ accepted ☐ remarks: …

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

## Next (C-047)

1. P4 increment 3 – message model, schema embedding (`build.rs`, `strict-validation`), payload vectors; first runtime dependencies under `cargo deny`.
2. After your answers: D-1 guideline text and re-baseline; R-1/R-2 `reviewed`.
