# Review and decisions – open items for the PO

_Last updated: 2026-10-26, after C-050 (guideline `734c560`, register `fce5cf5`); P4 in progress. **Nothing open for the PO.**_

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
- C-048 (P4 increment 4): crate `oec` with `Level`, `Error`, `App`/`AppBuilder`, `Bootstrap` (container storage contract and explicit `Broker`), `Store`, identity/MAM and string-keyed Config getters; all 18 `broker.json` vectors as tests with the same verdicts as `platform/broker.schema.json`; new runtime dependencies thiserror and log under `cargo deny` without exceptions; increment 5 split into 5a (C-049) and 5b (C-050). No reviewed item changed, no new vector, no spec finding, so there is nothing to decide or review; A-17 … A-24 are for information. C-049 runs in a new agent; `prompt.md` is filled.
- C-049 (P4 increment 5a): sans-IO in `oec-core` – own MQTT 3.1.1 codec (canonical, every refusal names its MQTT rule or REQ) and client `Session` as a state machine (new Will per CONNECT, keep alive, QoS 1, subscriptions, close → Will), `lifecycle` (Will/Birth/Close equal the role vectors V-PAY-025/023/024, Health heartbeat, Health checks), `MessageIds`; proptest as the only new (dev) dependency, `cargo deny` without exceptions. No reviewed item changed, no new vector, no spec finding, so there is nothing to decide or review; A-25 … A-31 are for information. C-050 runs in a new agent; `prompt.md` is filled.
- C-050 (P4 increment 5b): runtime in `oec` – TLS transport on tokio + rustls (CA-only trust, client certificate or credentials, PKCS#8 keys with passphrase), `App::run`/`run_async`/`handle`, `AppHandle`, `ConnectionState`, `on_connection`, reconnect with backoff, driving the `Session` and `Lifecycle` of C-049; runtime tests against an in-process TLS test broker; new dependencies tokio, rustls, tokio-rustls, rustls-pki-types, pkcs8 (dev: rcgen) under `cargo deny` without exceptions. No reviewed item changed, no new vector, no spec finding, so there is nothing to decide or review; A-32 … A-41 are for information. Two of them refine the accepted sketch contract (ADR-0020 amendment C-050) and are worth a look: **A-34** (an explicit `Broker` needs a CA) and **A-37** (`run_async` installs no signal handlers). Increment 9 (C-051) runs in a new agent; `prompt.md` is filled.

Coverage stays at 0 gaps; schema unchanged (`2.0.1-dev` @ `734c560`).

Logs: [C-050](log/2026-10-26_C-050_runtime-tokio-rustls.md), [C-049](log/2026-10-26_C-049_mqtt-session-lifecycle.md), [C-048](log/2026-10-26_C-048_oec-crate-bootstrap.md), [C-047](log/2026-10-26_C-047_message-model.md), [C-046 incl. addenda 1–3](log/2026-10-26_C-046_p4-start.md).

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
| A-17 | Crate `oec`: re-export vs. wrap, `Error` wraps core errors as `Invalid` with codes, file I/O only in `oec` | info (agent) | C-048 | – | – |
| A-18 | Container storage contract details (hostname, integer-only `broker.json` numbers, CA required, certificate before credentials) | info (agent) | C-048 | – | – |
| A-19 | Explicit bootstrap: serial number required, default ClientId from the IL hash, no authentication → `LevelNotMet` | info (agent) | C-048 | – | – |
| A-20 | New `reference_designation(bool)`, `Store::is_persistent()`, `MemoryStore::new()` | info (agent) | C-048 | – | – |
| A-21 | Config definition and string-keyed getters without runtime; Pattern checked from increment 7 | info (agent) | C-048 | – | – |
| A-22 | Store keys and atomic save; keep alive 1 … 65535 s; publish limit clamped; `Secret` redacted | info (agent) | C-048 | – | – |
| A-23 | Dependencies thiserror 2 and log 0.4 only, own base64 decoder, no deny exception | info (agent) | C-048 | – | – |
| A-24 | Cycle cut: C-048 not split, increment 5 split into 5a (C-049) and 5b (C-050); `run`/`run_async` stay in the sketch | info (agent) | C-048 | – | – |
| A-25 | MQTT codec: client subset without QoS 2, symmetric and canonical, every refusal names its rule | info (agent) | C-049 | – | – |
| A-26 | Session model: sans-IO, keep-alive timers, close without DISCONNECT on errors (→ Will), subscriptions restored, unacknowledged QoS 1 `Dropped` without session | info (agent) | C-049 | – | – |
| A-27 | Lifecycle messages in the MG 6.3 shape: no DataSetMessage `MessageType` (exception to A-11) | info (agent) | C-049 | – | – |
| A-28 | Health rules: Health equals the most severe Detail, NORMAL_0/0 only for Close, Birth after recovery from FAILURE_1/0 | info (agent) | C-049 | – | – |
| A-29 | MessageIds: unique and monotonic per publisher, counter within a millisecond (appended to the timestamp, wording fixed in C-050) | info (agent) | C-049 | – | – |
| A-30 | proptest (dev only) instead of cargo-fuzz; no deny exception | info (agent) | C-049 | – | – |
| A-31 | Own `SessionConfig` in `oec-core`, mapping from `ConnectionSettings` in C-050; C-049 not split | info (agent) | C-049 | – | – |
| A-32 | TLS transport: TLS 1.3/1.2 only, trust only from the CA file, client certificate before credentials, TLS reload per reconnect | info (agent) | C-050 | – | – |
| A-33 | Keys with passphrase: PKCS#8 PBES2 via pkcs8; legacy OpenSSL, PBES1, PBKDF2-SHA1 refused | info (agent) | C-050 | – | – |
| A-34 | Explicit `Broker` without CA → `LevelNotMet`; no OS trust store (ADR-0020 amendment C-050) | info (agent) | C-050 | – | – |
| A-35 | Birth first: Clean Session always, Session Present refused, so nothing precedes the Birth | info (agent) | C-050 | – | – |
| A-36 | Reconnect: backoff 1 s → 60 s with jitter, never gives up, refused CONNACK retried; timeouts = keep alive, close 5 s | info (agent) | C-050 | – | – |
| A-37 | `run`: own runtime + signals; `run_async`: no signal handlers (ADR-0020 amendment C-050) | info (agent) | C-050 | – | – |
| A-38 | `ConnectionState`/`on_connection`/`shutdown` semantics; writer ids MAM 10, Health 11 | info (agent) | C-050 | – | – |
| A-39 | In-process TLS test broker with rcgen PKI; env-gated smoke test; no outside network in the gate | info (agent) | C-050 | – | – |
| A-40 | Dependencies tokio, rustls, tokio-rustls, rustls-pki-types, pkcs8; features `crypto-aws-lc-rs`/`crypto-ring`; tree 22 → 73; no deny exception | info (agent) | C-050 | – | – |
| A-41 | Cycle cut: C-050 not split, next increment 9; `set_health` stays in the sketch; ADR-0026 §9.6 wording fix | info (agent) | C-050 | – | – |

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

### A-17 … A-24 – P4 increment 4 (C-048, agent)

- **Where:**
  - `oec/sdk/rust/oec/src` (`lib` with `Level`/`Error`, `app` with `Bootstrap`/`Broker`/`AppBuilder`/`App`, `container`, `store`, `config`, `identity`, `status`, `address`, `event`), `oec/sdk/rust/oec/tests/{app,broker}.rs`;
  - ADR-0020 amendment C-048, ADR-0026 §8, ADR-0023 amendment C-048;
  - log [C-048](log/2026-10-26_C-048_oec-crate-bootstrap.md) (89 REQ ids cited in `oec`).
- **A-17 Crate structure and errors:**
  - `oec` re-exports the `oec-core` types that already have the right shape (`ServiceType`, `SyslogLevel`, `LocalizedText`, `Mam`, `ConfigType`) and wraps `IdentificationLink`/`Oi4Identifier`.
  - `Error` (ADR-0020 §14, thiserror): core errors become `Invalid` with message and violation codes; `StatusCode` stays separate.
  - All file access is in `oec`; `oec-core` stays sans-IO.
- **A-18 Container storage contract:**
  - Hostname from `/proc/sys/kernel/hostname`, else `/etc/hostname`; must be an RFC 1123 label (REQ-CNT-021).
  - `SecurePort`/`MaxPacketSize` must be JSON integers (`8883.0` refused, like the Go oracle); no vector has this case.
  - The CA is required in a container (`broker_ca.pem`, else `ca.pem`); no fallback to the OS trust store.
  - A client certificate wins over credentials; a key without its certificate falls back to the credentials with a warning; malformed base64 is an error.
- **A-19 Explicit bootstrap:** serial number required (REQ-RES-MAM-015); default ClientId `oec` + FNV-1a 64 hash of the IL (REQ-MQTT-013); no authentication → `LevelNotMet` (REQ-CMP-002, REQ-MQTT-009); in a container `client_id` is refused and `serial_number` needs an explicit IL.
- **A-20 Level and store:** `reference_designation(bool)` – off at L1, on at L2/L3, off at L3 → `LevelNotMet` (REQ-CMP-020); a non-persistent store with ReferenceDesignation → `LevelNotMet` (REQ-RES-REFDES-002); explicit bootstrap defaults to `MemoryStore`; `MemoryStore::new()` replaces the unit struct (ADR-0020 amendment).
- **A-21 Config:** `ConfigDef::values()` validates the definition and returns `ConfigValues` with string-keyed getters (Q-022); Pattern is checked from increment 7 (needs a regex crate).
- **A-22 Settings:** store keys `/`-separated `[A-Za-z0-9._-]` segments, atomic save; keep alive whole seconds 1 … 65535 (REQ-LIFE-008); own MaxPacketSize clamped to the bus value, 0 refused (REQ-PAG-005); `Secret` redacted in `Debug` (G-4.5).
- **A-23 Dependencies:** thiserror 2 and log 0.4 only (22 crates, 74 with `strict-validation`); `cargo deny` without exceptions; own base64 decoder instead of a crate.
- **A-24 Cycle cut:** C-048 not split; increment 5 split into 5a (C-049: sans-IO MQTT codec, session, lifecycle, fuzzing) and 5b (C-050: tokio/rustls transport, `run`/`run_async`/`AppHandle`); these stay in the sketch until 5b; `App: Send + Sync` is tested.
- **Tests:** default 17 unit + 18 App + 1 `broker.json` test, strict 19 + 19 + 2; all 18 `broker.json` vectors agree with the schema gate.
- **Open technical points (later cycles):** regex for Config Pattern in increment 7; store key length for represented assets and `set_health` while `Details` are set in increment 6; an opt-in OS trust store only if asked for. (Encrypted keys and the CA of an explicit `Broker` were decided in C-050: A-33, A-34.)

### A-25 … A-31 – P4 increment 5a (C-049, agent)

- **Where:**
  - `oec/sdk/rust/oec-core/src/mqtt/{packet,codec,session}.rs`, `src/lifecycle.rs`, `src/message/envelope.rs` (`MessageIds`);
  - `oec/sdk/rust/oec-core/tests/{mqtt_codec,session,lifecycle,proptest_mqtt}.rs`;
  - ADR-0026 §9, ADR-0023 amendment C-049, ADR-0020 amendment C-049;
  - log [C-049](log/2026-10-26_C-049_mqtt-session-lifecycle.md) with the REQ tracing tables.
- **A-25 Codec:**
  - All packets a client needs (CONNECT with Will and credentials, CONNACK, PUBLISH QoS 0/1, PUBACK, SUBSCRIBE/SUBACK, UNSUBSCRIBE/UNSUBACK, PING, DISCONNECT); QoS 2 and Will Retain are refused (REQ-MQTT-017/018).
  - Symmetric and canonical: what decodes re-encodes byte-identically; a non-minimal Remaining Length, reserved bits or trailing bytes are refused.
  - Every refusal is a `CodecError` whose `rule()` names the MQTT statement (`MQTT-x.y.z-n`), the section or the REQ.
  - Written new after reviewing the spike; nothing was copied.
- **A-26 Session:**
  - Sans-IO: bytes and monotonic time in, bytes and events out.
  - Every `connect` takes the Will of this connection (REQ-LIFE-016).
  - Keep alive (default 60 s, REQ-MQTT-014): a PING after one idle period, a timeout after one more.
  - Errors, timeouts and connection loss close **without** DISCONNECT, so the broker publishes the Will (REQ-LIFE-008).
  - Subscriptions are restored after a reconnect without session. Unacknowledged QoS 1 messages are reported as `Dropped` (the runtime decides). A payload over MaxPacketSize is refused (REQ-PAG-005).
- **A-27 Lifecycle message shape:**
  - Will, Birth, Close and the self-Health are exactly the MG 6.3 examples (equal to V-PAY-025/023/024): one DataSetMessage, no Timestamp/SequenceNumber, empty WriterGroupName and **no DataSetMessage `MessageType`** (REQ-MSG-029 assumes `ua-keyframe`).
  - This is an exception to A-11 ("always `MessageType`") for these messages only; both forms pass the schema.
- **A-28 Health:**
  - Health **equals** the most severe `Details` entry (REQ-RES-HEALTH-002). The sketch says "at least"; ADR-0020 amendment C-049 reads it as "equals", and C-050 fixes the sketch doc.
  - NORMAL_0/0 is refused in `set_health` (it means Close, REQ-LIFE-014); HealthScore ≤ 100.
  - Health on change (incl. `Details`) and every 60 s (REQ-TOP-017, REQ-RES-HEALTH-004).
  - A Birth before the first non-failure Health after FAILURE_1/0 (REQ-LIFE-012/017).
  - Asset Health refuses the AppId (REQ-CMP-007).
- **A-29 MessageIds:** `<unix-ms>-<PublisherId>`, then `<unix-ms>1-<PublisherId>`, `<unix-ms>2-<PublisherId>`, … within the same millisecond (the counter is appended to the timestamp as in REQ-MSG-007; the C-049 text said `-1`, `-2`, which was a wording error, fixed in C-050); a wall clock that steps back keeps the last millisecond (REQ-MSG-007/008).
- **A-30 Property tests:**
  - proptest 1.12, dev-only, without default features. It adds 10 crates to the lock file, none linked into the SDK.
  - Licenses: MIT OR Apache-2.0; r-efi also offers LGPL and is used under MIT/Apache-2.0.
  - `cargo deny` without exceptions.
  - cargo-fuzz is not set up: it needs nightly, so it could not run in the gate.
- **A-31 Mapping and cycle cut:**
  - `oec-core` has its own `SessionConfig` (the core does not depend on `oec`); `ConnectionSettings` → `SessionConfig` is mapped in C-050 with the runtime.
  - C-049 not split; C-050 = 5b as planned.
- **Tests:** codec 18, session 17, lifecycle 10 (strict 11: every generated payload passes the strict schema), proptest 6 × 1024 cases (about 0.7 s). All green.
- **Open technical points (C-050):**
  - encrypted private keys (`passphrase`) and the CA of an explicit `Broker` (unchanged from C-048);
  - Birth first also with a persistent session (A-26 resends before the Birth; the default Clean Session avoids it);
  - `set_health` while `Details` are set.

### A-32 … A-41 – P4 increment 5b (C-050, agent)

- **Where:**
  - `oec/sdk/rust/oec/src/{transport,runtime,app,lib}.rs`;
  - `oec/sdk/rust/oec/tests/runtime.rs` (test broker, PKI, runtime tests, smoke test);
  - ADR-0026 §10, ADR-0023 amendment C-050, ADR-0020 amendment C-050;
  - log [C-050](log/2026-10-26_C-050_runtime-tokio-rustls.md).
- **A-32 TLS transport:**
  - rustls with TLS 1.3 and 1.2 only (REQ-MQTT-005); the server name is the configured host.
  - Trust anchors only from the CA file of the storage or `Broker::ca_pem` (REQ-MQTT-008); never the OS trust store.
  - Client certificate and key when present, and then no MQTT user name/password (REQ-MQTT-009/010).
  - TLS material loaded at `run` start (an error there is fatal: `Error::Bootstrap`) and reloaded before every reconnect, so renewed certificates are used; on a failed reload the last good one stays.
- **A-33 Keys with passphrase (REQ-CNT-008/009):**
  - `ENCRYPTED PRIVATE KEY` (PKCS#8 PBES2: PBKDF2 with HMAC-SHA-2 or scrypt, AES) is decrypted with the crate pkcs8; checked with keys written by OpenSSL 3 (`openssl pkcs8 -topk8 -v2 aes-256-cbc`, also `-scrypt`).
  - Refused with a clear `Error::Bootstrap` and a conversion hint: legacy OpenSSL encryption (`Proc-Type: 4,ENCRYPTED`), PBES1, PBKDF2-SHA1.
  - The guideline does not define the key format; this is an implementation choice, not a finding.
- **A-34 Explicit `Broker` needs a CA:**
  - `build()` refuses a `Broker` without `ca_pem` with `LevelNotMet` (REQ-CMP-002, REQ-MQTT-008); `ConnectionSettings::ca()` returns `&Path`.
  - No fallback to the OS trust store (ADR-0023: trust from the storage only). An explicit opt-in could be added later without breaking anything.
  - This changes the semantics of a sketch item (the sketch accepted a `Broker` without CA): ADR-0020 amendment C-050 C.1.
- **A-35 Birth first:** Clean Session is always true and the core session refuses Session Present with it, so no resent message can precede the Birth (REQ-LIFE-015). The runtime tests assert Birth first, then Health.
- **A-36 Reconnect and timeouts:**
  - Backoff 1 s, ×2 per failed attempt, at most 60 s, each delay randomized to 50 … 100 %; reset by a CONNACK. The runtime never gives up.
  - A refused CONNACK (e.g. wrong credentials) is logged as an error and retried: the platform can fix the credentials without restarting the application.
  - TCP + TLS connect and every write are bounded by the keep-alive interval; Close + DISCONNECT on shutdown get 5 s.
- **A-37 `run` vs. `run_async`:**
  - `run` builds its own tokio `current_thread` runtime and handles SIGINT/SIGTERM (Windows: Ctrl-C, Close, Shutdown) → Close, DISCONNECT.
  - `run_async` is the same loop **without** signal handlers: in the application's runtime the signals belong to the application, which calls `AppHandle::shutdown`. ADR-0020 §2 said "same as `run`"; amendment C-050 C.2.
  - tokio keeps signal handlers registered after `run` returns (noted in ADR-0026 §10.7).
- **A-38 State and handle:**
  - `ConnectionState`: `Closed` before and after `run`; `Connecting` until the first CONNACK; `Online`; `Reconnecting` after a loss.
  - `on_connection` is called on each change, on the runtime thread (must not block).
  - `shutdown` is idempotent; before `run`, `run` returns at once.
  - DataSetWriterIds of the application: MAM 10, Health 11 (REQ-MSG-020).
  - Received messages are dropped (debug log) until the consumer and request side (increments 6–8).
- **A-39 Tests:**
  - An in-process test broker (std + rustls on loopback, decoding with the `oec-core` codec) and a PKI generated per run with rcgen (dev only).
  - 12 runtime tests: lifecycle order with payloads, reconnect with new Will/Birth, keep-alive timeout, refused CONNECT, untrusted broker, client certificate over TLS 1.2, encrypted key, unusable TLS material, shutdown before run, `run_async` spawned, unreachable broker, handle traits. About 3 s, stable, green with both crypto providers.
  - Smoke test against a local broker only with `OEC_SMOKE_*`; one manual run against Mosquitto 2.0.18 in Docker showed CONNECT, Birth, Health, Close, DISCONNECT in order.
- **A-40 Dependencies:**
  - tokio 1.54 (`rt`, `net`, `time`, `sync`, `signal`), rustls 0.23 (`std`, `tls12`, `logging`), tokio-rustls 0.26, rustls-pki-types 1.15, pkcs8 0.11 (`pem`, `encryption`); dev: rcgen 0.14.
  - Features `crypto-aws-lc-rs` (default, post-quantum key exchange preferred) and `crypto-ring` (ring wins if both are on; the gate runs clippy with ring).
  - Tree of `oec`: 73 crates (was 22), 122 with `strict-validation`; lock file 191 packages. Licenses all in the allowlist (aws-lc-sys and ring incl.); `cargo deny` without exceptions, `deny.toml` unchanged.
- **A-41 Cycle cut and leftovers:**
  - C-050 not split; next is increment 9 (C-051, dev environment), then the P5 interleave and increments 6–8.
  - `set_health`/`set_health_details` stay in the sketch until `Asset` moves (increment 6); the sketch doc says "equals" now.
  - ADR-0026 §9.6 and A-29 described the MessageId counter as `-1`, `-2`; the code (and REQ-MSG-007) append it to the timestamp. Wording fixed, code unchanged.

## Next (C-051)

1. P4 increment 9 – dev environment `oec/dev`: docker compose with Mosquitto (TLS, password file, client certificates), a generated dev PKI, a sample `/etc/oi4` layout; interop tests of increment 5 against it (opt-in, so the default gate needs no container engine). Runs in a new agent (start prompt in `prompt.md`). No new runtime dependency expected.
