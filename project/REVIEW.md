# Review and decisions – open items for the PO

_Last updated: 2026-10-26, after C-046 (guideline `e0262dd`, register `2453eff`); P4 in progress._

This is the single list of everything that waits for the PO:
- items to **review**: drafts, code, and changes to reviewed items;
- items to **decide**: spec findings, open questions, ADRs, cycle plan.

Each cycle updates it. Items leave the list once the PO has answered and the decision record has been applied. Details are in the cycle logs; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md), with the same numbering.

**How to answer:** tick the box or write the decision into the "PO" field. You can also answer in the chat, e.g. "D-1 … D-5 wie empfohlen, R-1 … R-4 akzeptiert".

**Current state:** your C-045 review was applied in C-045 addendum 1: three REQs `reviewed`, SF-103 in the guideline (`e0262dd`), re-baseline, P4 first. C-046 started P4:
- P4 plan and cycle cut in ROADMAP P4;
- JSON export of the vectors and the matrix (`oec/spec/export/`, `3f6e0a0`);
- Rust workspace `oec/sdk/rust` with crate `oec-core`, increments 1 (identity) and 2 (topic model), with no runtime dependency (`bfb4a03`);
- all IL, dnp, Oi4Identifier and 151 topic vectors, the payload topics and the 66 matrix rows pass;
- the gate gained clippy pedantic and `cargo deny`;
- ADR-0026 `proposed`.

No reviewed item changed, coverage stays at 0 gaps, and no spec finding is open.

Log: [C-046](log/2026-10-26_C-046_p4-start.md).

## Overview

| # | Item | Kind | Source | Recommendation | PO |
|---|------|------|--------|----------------|----|
| D-1 | ADR-0026: Rust workspace layout and crate roles (G-3.4) | decide | C-046 | accept | ☐ |
| D-2 | JSON export `oec/spec/export/` as the SDK test-vector contract | decide | C-046 | accept | ☐ |
| D-3 | P4 cycle cut and order (increment 3 in its own cycle; increment 9 before 6–8) | decide | C-046 | accept | ☐ |
| D-4 | `Oi4Identifier`: dnp-encoding opt-in, `/` without it is an error (`slash-in-element`) | decide | C-046 | accept | ☐ |
| D-5 | White space and control characters in Filter values: warning, not error | decide | C-046 | accept | ☐ |
| R-1 | `oec-core` code and tests (increments 1–2) | review | C-046 | accept | ☐ |
| R-2 | spectool JSON export (format, `omitempty`) | review | C-046 | accept | ☐ |
| R-3 | cargo-deny gate (`deny.toml`, offline fallback) | review | C-046 | accept | ☐ |
| R-4 | Lint profile: clippy `pedantic`, `doc_markdown` allowed | review | C-046 | accept | ☐ |

## Decisions

### D-1 – ADR-0026: Rust workspace layout and crate roles

- **Where:** `decisions/ADR-0026-rust-sdk-workspace.md` (`proposed`). The repository layout is a PO decision (G-3.4).
- **Content:**
  - one Cargo workspace `oec/sdk/rust` with flat crates `oec-core`, `oec` (increment 4) and `oec-ffi` (increment 10);
  - sketch and spikes excluded until `oec` replaces them;
  - `Cargo.lock` committed;
  - `publish = false` until P9;
  - edition 2024, MSRV 1.89.
- **`oec-core`:**
  - sans-IO and internal, with no stability promise of its own;
  - no runtime dependency in increments 1–2; from increment 3, only the I/O-free crates of ADR-0023;
  - structured errors with stable codes, which `oec` wraps into `oec::Error::Invalid` as the accepted sketch says.
- **Alternative:** one crate with modules. It would mix the sans-IO core with tokio/rustls and make `oec-ffi` and fuzzing harder. ADR-0011 already names the three crates.
- **PO:** ☐ accept ☐ change: …

### D-2 – JSON export as the test-vector contract

- **What:** `spectool render` writes `oec/spec/export/vectors/*.json` and `export/matrix/method-resource.json`; `render -check` keeps them fresh. The SDKs read only the export (ADR-0026 §3).
- **Why:**
  - The Rust and C SDKs need no YAML parser: `serde_yaml` is archived/deprecated, and D9 would need a C YAML library.
  - JSON is easy to turn into C arrays for the D9 host tests.
  - The YAML stays the single reviewed source, and the export is generated.
- **Alternative:** each SDK parses the YAML (`serde_yaml` or a fork for Rust, a C YAML library for D9).
- **PO:** ☐ accept ☐ change: …

### D-3 – P4 cycle cut and order

- **Where:** ROADMAP P4 "Order and cycle cut".
- **Order:**
  - C-046: increments 1–2;
  - C-047: increment 3 (message model, schema embedding, first runtime dependencies);
  - C-048: increment 4 (`oec` crate: App, Level, Bootstrap);
  - then increment 5 (MQTT codec/session, transport);
  - then **increment 9 (dev environment) directly after 5**, because the transport needs the TLS broker for its interop tests (ADR-0023 Consequences);
  - P5 interleave (`internal/mqttwire`, `monitor --replay`);
  - then 6–8 (provider, write, consumer side; sketch deleted);
  - then 10 (`oec-ffi`).
- **Concrete exit criteria (6):** all ADR-0020/0021 items implemented; all D2-relevant vectors pass; integration tests against the dev broker incl. D7 `test`; C examples under ASan; gate green on the platform matrix; example apps documented.
- **Prerequisite from increment 5/9:** a running container engine (Docker Desktop).
- **PO:** ☐ accept ☐ change: …

### D-4 – `Oi4Identifier`: dnp-encoding opt-in

- **Rule (MG 4.3.1, MG A3, SF-043 e):** the elements are plain strings; the dnp-encoding *may* be used for characters that are not permitted or for `/`.
- **SDK behaviour:**
  - `Oi4Identifier::new(uri, model, product_code, serial).build()` does not mask. A `/` in an element is the SDK-only violation `slash-in-element` (it would shift the four-part structure), and other characters that are not permitted are reported as IL violations.
  - `.dnp_encoded()` masks every element first.
- **Alternative:** always mask, as the spectool vectors do. Masking changes the IL of every asset with such characters, so the user should choose it explicitly. The sketch already had `.dnp_encoded()`.
- **PO:** ☐ accept ☐ change: …

### D-5 – White space in Filter values: warning

- **Rule:** REQ-TOP-020 (MG 7.7) – Filter values SHOULD NOT contain white space or control characters.
- **SDK behaviour:**
  - `Topic::parse`/`Topic::new` accept such values (the topic is valid) and report them via `Topic::warnings()`. D7 `validate` does the same (warning, exit code 0).
  - The publishing side of `oec` (increment 6) will log a warning and publish anyway.
- **Alternative:** reject on publish. That would make a SHOULD into a MUST.
- **PO:** ☐ accept ☐ change: …

## Review items

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
- **PO:** ☐ accepted ☐ remarks: …

### R-2 – spectool JSON export

- **Where:** `oec/spec/tools` (render), `oec/spec/export/`, `oec/spec/README.md`.
- **Format:**
  - `$comment` ("generated, do not edit"), `source` (the YAML path), then the fields of the Go model;
  - Go zero values are omitted, so a missing `encoded` means "" (e.g. V-IL-116).
- **PO:** ☐ accepted ☐ remarks: …

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
- **PO:** ☐ accepted ☐ remarks: …

### R-4 – Lint profile

- **Where:** `oec/sdk/rust/Cargo.toml` `[workspace.lints]`.
- **Profile:**
  - `unsafe_code = forbid` (`oec-ffi` will opt out per module);
  - `missing_docs`, `unreachable_pub`;
  - clippy `all` + `pedantic` with `-D warnings` in the gate;
  - allowed: `module_name_repetitions`, `must_use_candidate`, and `doc_markdown`, because the docs use guideline terms like ManufacturerUri or PublicationList as prose.
- **PO:** ☐ accepted ☐ remarks: …

## After your answers (C-047)

1. ADR-0026 → `accepted` (or changed as you say).
2. C-047: P4 increment 3 – message model, schema embedding (`build.rs`, `strict-validation`), payload vectors; first runtime dependencies under `cargo deny`.
