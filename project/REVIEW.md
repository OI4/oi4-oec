# Review and decisions – open items for the PO

_Last updated: 2026-10-27, C-054 (P4 increment 6b: provider side, part 2; A-75 … A-86 for information; guideline `9044929`, register `b94e803`). **Nothing is open for the PO.**_

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
- C-051 (P4 increment 9): dev environment `oec/dev` – Mosquitto 2.1.2 pinned by digest (TLS 1.2/1.3, a password-file listener and a client-certificate listener), a dev PKI generator in Go (standard library), two sample container storages; opt-in interop tests of increment 5 against it via `oec/dev/interop.sh` (8 green, the gate stays without Docker); the C-050 smoke test is replaced. No SDK defect, no new dependency, no reviewed item changed, no new vector. **One finding: SF-105** (`MaxPacketSize` is a payload limit, brokers limit whole packets) → **D-1** to decide. A-42 … A-49 are for information. The P5 interleave (C-052) runs in a new agent; `prompt.md` is filled.
- C-051 review: your answer to D-1 ("Vorschlag komplett", after a first reply "option b" that I asked you to clarify) is applied: RIG 4.1.1 in `59e3ae3`, **SF-105 resolved** (`d220c48`), re-baseline on `59e3ae3` (1401 inventory items). REQ-CNT-003 follows the new text and is back to `draft` (R-1). D8 T8-BUS-01 also covers REQ-CNT-003 (A-50).
- C-051 addendum 2: you accepted R-1; REQ-CNT-003 is `reviewed` (`41c9f44`). No drafts, no open finding.
- C-052 (P5 interleave, one cycle): D7 `oec-check monitor` in `oec/conformance`.
  - Live over MQTT 3.1.1/TLS (`internal/mqttwire` on `eclipse/paho.mqtt.golang` v1.5.1, EDL-1.0) with `--record`, and offline with `--replay` of JSONL captures (`internal/capture`).
  - 13 of 19 monitor tests are `implemented`; text/JSON/JUnit reports; exit code 4 when a MUST cannot be judged.
  - The opt-in `oec/conformance/live.sh` is green against the dev broker (Mosquitto 2.1.2, client certificate and credentials, Birth/Will/reconnect).
  - No reviewed item changed, schema unchanged.
  - **One finding: SF-106** (scope of the 60 s Health heartbeat) → **D-1** to decide.
  - **R-1:** 18 capture fixtures with expected verdicts (`draft`) to review.
  - A-51 … A-62 are for information.
  - C-053 (P4 increment 6) runs in a new agent; `prompt.md` is filled.
- C-052 review: your answer ("Ich stimme D-1 und R-1 zu") is applied.
  - D-1: MG 7.5.1 in `9044929` as proposed, **SF-106 resolved** (`b94e803`), re-baseline on `9044929` (1402 inventory items). REQ-TOP-017 follows the new text and is back to `draft` (**R-2**). D7 T7-MON-05 and the SDK already match, nothing changes there.
  - R-1: the 18 capture fixtures are `reviewed`.
- C-052 addendum 2: you accepted R-2; REQ-TOP-017 is `reviewed` (`8d7538b`). No drafts, no open finding.
- C-053 (P4 increment 6a; increment 6 is split into 6a and 6b):
  - `oec-core`: a sans-IO provider for the sources of an application (itself and its represented assets): MAM, Health and Profile per Source, the PublicationList with persisted DataSetWriterIds, `Get` answers for these four resources from Level 2, pagination against `MaxPacketSize`.
  - `oec`: the asset API (`AssetDef`, `Asset`, `HealthDetail`, `AppHandle::{this, add_asset, asset, assets}`) on the runtime; offline changes are published as the current state after the reconnect, nothing is queued.
  - Tests: runtime tests against the in-process TLS broker; opt-in interop 9 green; a one-off D7 `oec-check monitor` run against the SDK: 10 pass, 0 fail, 3 skip (T7-MON-18 pagination passes).
  - No finding, no new dependency, no reviewed item changed, no new test data, so there is nothing to decide or review; A-63 … A-74 are for information. Worth a look: **A-65** (offline: state instead of a queue), **A-71** (unknown Source: Status event only with increment 7), **A-72** (sketch refinements).
  - C-054 (increment 6b) runs in a new agent; `prompt.md` is filled.
- C-054 (P4 increment 6b, not split):
  - `oec-core`: Data DataSets on the provider of 6a – Metadata derived from the field definitions with persisted ConfigurationVersions, the UUIDv5 DataSetClassId of Tag and field structure, the publish forms APPLICATION/SOURCE/FILTER, `Interval` > 0 cyclic, `Interval` 0 with the Precisions as a dead band, `Get Data`/`Get Metadata` (no Metadata for Data that is not announced, REQ-CMP-044), Profile and PublicationList with Data and Metadata.
  - `oec`: the API `oec::data` (`DataSetDef`, `Field`, `Publication`, `Mode`, `Value`, `DataSet`) and `Asset::add_data`; the sketch is aligned (ADR-0020 amendment C-054).
  - Tests: core and runtime tests in the gate (intervals without sleeping); opt-in interop 10 green; a one-off D7 `oec-check monitor` run against the SDK: 10 pass, 0 fail, 3 skip, after fixing a D7 bug found by that run (T7-MON-11 treated an unsolicited Metadata as "without content").
  - New: uuid feature `v5` (+ sha1_smol, BSD-3-Clause) under `cargo deny` without exception.
  - No finding, no reviewed item changed, no new test data, so there is nothing to decide or review; A-75 … A-86 are for information. Worth a look: **A-80** (a DataSet goes out only once every field has a value), **A-84** (sketch changes: `update` returns `Result`, no `Value::Json`), **A-85** (D7 fix).
  - C-055 (increment 7) runs in a new agent; `prompt.md` is filled.

Coverage stays at 0 gaps; schema `2.0.1-dev` @ `9044929` (re-baseline, no content change).

Logs: [C-054](log/2026-10-27_C-054_data-sets-metadata.md), [C-053](log/2026-10-27_C-053_provider-registry-get.md), [C-052](log/2026-10-26_C-052_d7-monitor.md), [C-051](log/2026-10-26_C-051_dev-environment-interop.md), [C-050](log/2026-10-26_C-050_runtime-tokio-rustls.md), [C-049](log/2026-10-26_C-049_mqtt-session-lifecycle.md), [C-048](log/2026-10-26_C-048_oec-crate-bootstrap.md), [C-047](log/2026-10-26_C-047_message-model.md), [C-046 incl. addenda 1–3](log/2026-10-26_C-046_p4-start.md).

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
| A-42 | Dev environment `oec/dev`; dev PKI generator in Go (standard library), its tests and a config lint in the gate | info (agent) | C-051 | – | – |
| A-43 | Mosquitto 2.1.2 pinned by tag + digest; listeners 8883 (password file) and 8884 (client certificate, CN = user); TLS 1.2 minimum; loopback ports; persistence | info (agent) | C-051 | – | – |
| A-44 | Broker packet limits: payload limit = `MaxPacketSize`, packet limit = `MaxPacketSize` + 65544 (follows the SF-105 proposal) | info (agent) | C-051 | – | – |
| A-45 | Sample container storages: two roots (credentials / certificate with encrypted key), `Address` `localhost`, no `broker_ca.pem` | info (agent) | C-051 | – | – |
| A-46 | Opt-in interop: `OEC_INTEROP=1` via `oec/dev/interop.sh`, skip with exit 0 without an engine, `OEC_COMPOSE` for Podman, pulls only there | info (agent) | C-051 | – | – |
| A-47 | Interop test design: probe on the `oec-core` codec, killed child process for the Will, persistent probe + MessageId filter for the restart | info (agent) | C-051 | – | – |
| A-48 | Smoke test (`OEC_SMOKE_*`) replaced by `explicit_broker`; no new dependency, ADR-0023 unchanged | info (agent) | C-051 | – | – |
| A-49 | Cycle cut: C-051 not split; next the P5 interleave (C-052), then increments 6–8 and 10 | info (agent) | C-051 | – | – |
| A-50 | D8 T8-BUS-01 also covers REQ-CNT-003: a payload of exactly `MaxPacketSize` on a long topic is accepted | info (agent) | C-051 review | – | – |
| A-51 | Cycle cut: C-052 not split (capture, mqttwire, `monitor --replay` and live in one cycle) | info (agent) | C-052 | – | – |
| A-52 | Dependency paho v1.5.1 (EDL-1.0) pinned; indirect x/net, x/sync, gorilla/websocket (BSD); go1.27.3; govulncheck clean, run by hand | info (agent) | C-052 | – | – |
| A-53 | Own in-process TLS test broker `mqtttest` on the paho codec instead of mochi | info (agent) | C-052 | – | – |
| A-54 | TLS: `mqtts://` only, TLS 1.2/1.3, encrypted PKCS#8 (PBES2), no auto-reconnect, `--insecure` explicit and recorded, secrets only from files | info (agent) | C-052 | – | – |
| A-55 | Capture format v1: header, records, trailer; `payload_base64` for non-JSON; strict reader | info (agent) | C-052 | – | – |
| A-56 | Verdicts: a not-applicable skip does not worsen a test; a MUST that cannot be judged → exit 4 | info (agent) | C-052 | – | – |
| A-57 | Rule decisions: Will detection by MessageId (REQ-LIFE-016), heartbeat as SF-106 proposes, HEALTH-004 not observable, T7-MON-10 info, QoS from the delivered QoS | info (agent) | C-052 | – | – |
| A-58 | T7-MON-09, 13 … 17 deferred to `test` (PublicationList/Config state model) | info (agent) | C-052 | – | – |
| A-59 | Monitor fixtures as generated captures + `index.yaml`; positive and negative per implemented test enforced | info (agent) | C-052 | – | – |
| A-60 | CLI: `--level` default 3, `--duration` 5 min, `--app`, `--config` YAML, MaxPacketSize precedence, `--key-passphrase-file`, `--client-id` | info (agent) | C-052 | – | – |
| A-61 | Monitor report JSON version 1, text, JUnit; `report.schema.json` deferred to `test` | info (agent) | C-052 | – | – |
| A-62 | Opt-in live test `oec/conformance/live.sh` (`OEC_LIVE=1`); the gate needs no Docker or network; run once green | info (agent) | C-052 | – | – |
| A-63 | Cycle cut: increment 6 split into 6a (C-053: sources, built-in resources, `Get`, pagination) and 6b (C-054: DataSets, publish modes, intervals, dead band); order 6 → 7 → 8 → 10 kept | info (agent) | C-053 | – | – |
| A-64 | Sans-IO provider in `oec-core` (`provider`, `pagination`, `writers`), held by `oec` behind the shared state | info (agent) | C-053 | – | – |
| A-65 | Offline: state, no queue; the full state of every source on every connect | info (agent) | C-053 | – | – |
| A-66 | Order on connect: Birth → app Health → per asset MAM/Health/Profile → app Profile → PublicationList; heartbeat only for the application (SF-106) | info (agent) | C-053 | – | – |
| A-67 | Profile `[MAM, Health, Profile, PublicationList]`; PublicationList entries MAM/Health/Profile per source with `ON_REQUEST_SOURCE_10`, not listing itself | info (agent) | C-053 | – | – |
| A-68 | DataSetWriterIds persisted under `app/writer-ids`, never reused; a corrupt table is an error, no silent renumbering | info (agent) | C-053 | – | – |
| A-69 | Asset defaults (HardwareRevision `"undefined"`, DeviceClass `""`, Health NORMAL_0/100); RevisionCounter SDK-managed or from the nameplate; FNV-hashed store keys | info (agent) | C-053 | – | – |
| A-70 | Limits: `Get` answers against the bus `MaxPacketSize`, unsolicited publications against the own lower limit; page-plan rules | info (agent) | C-053 | – | – |
| A-71 | `Get` for an unknown Source: no answer now (MUST of REQ-CMP-047), Status `BadNotFound` (SHOULD) with increment 7 | info (agent) | C-053 | – | – |
| A-72 | API: `AppHandle::assets`, `Asset::{mam, is_this}`, `HealthDetail::new`; `bump_revision_counter` returns `Result`; `mam()` owned on `AppHandle`; no asset heartbeat | info (agent) | C-053 | – | – |
| A-73 | Interop test `level_2_assets_and_get_answers`; one-off D7 `monitor` run 10/0/3, capture not committed | info (agent) | C-053 | – | – |
| A-74 | No new dependency; uuid `v5` expected in 6b | info (agent) | C-053 | – | – |
| A-75 | Cycle cut: 6b in one cycle; `Set PublicationList` stays in increment 7; order 7 → 8 → 10 kept | info (agent) | C-054 | – | – |
| A-76 | Modules `provider::{data, sets, versions}` in `oec-core`, `oec::data`; versions persisted under `app/metadata-versions` | info (agent) | C-054 | – | – |
| A-77 | DataSetClassId = UUIDv5 of Tag and field structure (names, types, ValueRank) in an SDK namespace | info (agent) | C-054 | – | – |
| A-78 | Metadata from the fields (EngineeringUnits with UnitId −1, EURange); ConfigurationVersion as VersionTime, Major with the structure, Minor with any change | info (agent) | C-054 | – | – |
| A-79 | Metadata on its Filter topic at add, connect and version change, before the Data; `Get Metadata` only for announced Data (REQ-CMP-044) | info (agent) | C-054 | – | – |
| A-80 | A DataSet is published and answered once every field has a value; values coerced to the field type | info (agent) | C-054 | – | – |
| A-81 | Publish forms: FILTER alone, SOURCE/APPLICATION carry every member (state of the scope); `OFF_0` listed but never sent | info (agent) | C-054 | – | – |
| A-82 | `Interval` > 0 cyclic without catch-up, `Interval` 0 with Precisions as dead band; injected time | info (agent) | C-054 | – | – |
| A-83 | `Get Data` from the state (unknown Tag → empty list); Profile/PublicationList with Data and Metadata; `remove` keeps the ids reserved | info (agent) | C-054 | – | – |
| A-84 | API: `Field::range`, `DataSet::{tag, source, values}`, `Mode::union`; `update` returns `Result`; `Value::Json` dropped | info (agent) | C-054 | – | – |
| A-85 | Tests; interop `level_2_data_sets_and_metadata`; D7 `monitor` 10/0/3 after a D7 fix of T7-MON-11 | info (agent) | C-054 | – | – |
| A-86 | uuid feature `v5` (+ sha1_smol, BSD-3-Clause), `cargo deny` without exception | info (agent) | C-054 | – | – |

## Decisions and review items

Nothing is open for the PO.

### Answered earlier

Answered after the C-052 review (2026-10-27) and applied in C-052 addendum 2:
- R-2 – REQ-TOP-017 after SF-106: ☑ accepted → `reviewed` (`8d7538b`).

Answered in the C-052 review (2026-10-27) and applied in C-052 addendum 1:
- D-1 – SF-106 (scope of the 60 s Health heartbeat): ☑ as proposed → guideline `9044929` (MG 7.5.1), SF-106 resolved (`b94e803`), re-baseline, REQ-TOP-017 → R-2.
- R-1 – 18 capture fixtures of `oec-check monitor`: ☑ accepted → `reviewed` in `testdata/captures/index.yaml`.

Answered after the C-051 review (2026-10-26) and applied in C-051 addendum 2:
- R-1 – REQ-CNT-003 after SF-105: ☑ accepted → `reviewed` (`41c9f44`).

Answered in the C-051 review (2026-10-26) and applied in C-051 addendum 1:
- D-1 – SF-105 (`MaxPacketSize` vs. the packet limit of the broker): first reply "option b", which was ambiguous ((b) is a part of the proposal) and was clarified; then ☑ "Vorschlag komplett" = (a) + (b) + (c) → guideline `59e3ae3` (RIG 4.1.1), SF-105 resolved (`d220c48`), re-baseline, REQ-CNT-003 → R-1.

Answered in the C-046 review (2026-10-26) and applied in C-046 addendum 3:
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

### A-42 … A-49 – P4 increment 9 (C-051, agent)

- **Where:**
  - `oec/dev/` (`compose.yaml`, `mosquitto/mosquitto.conf`, `pki/*.go`, `interop.sh`, `README.md`);
  - `oec/sdk/rust/oec/tests/interop.rs`;
  - `oec/check.sh` step 6;
  - ADR-0026 §11;
  - log [C-051](log/2026-10-26_C-051_dev-environment-interop.md).
- **A-42 Dev environment and PKI generator:**
  - `go run ./pki` (Go module `oec/dev`, standard library only) writes everything into the git-ignored `oec/dev/generated/`:
    - ECDSA P-256 CA and certificates;
    - the Mosquitto `$7$` password file;
    - a PKCS#8 PBES2 key with passphrase;
    - two sample storages.
  - `-force` replaces only its own output (marker file).
  - Go instead of an `openssl` script: portable to Windows/macOS/Linux, no LibreSSL/OpenSSL differences, no `mosquitto_passwd` needed.
  - Its tests also lint `mosquitto.conf` and `compose.yaml` and run in the gate (step 6) without Docker.
- **A-43 Broker:**
  - `eclipse-mosquitto:2.1.2-alpine` pinned by tag and multi-arch digest.
  - `8883`: TLS + password file (REQ-MQTT-009).
  - `8884`: TLS + client certificate required, CN = user name (REQ-MQTT-010).
  - `tls_version tlsv1.2` is the minimum (1.3 negotiated); TLS 1.0/1.1 are refused, checked with OpenSSL.
  - No plain or anonymous access.
  - Ports on `127.0.0.1`; persistence on (needed for the restart test).
  - The entrypoint copies the TLS files to a broker-owned directory with mode 0600, so host permissions do not matter.
- **A-44 Packet limits:**
  - `message_size_limit 262144` (= `MaxPacketSize` of the sample `broker.json`: larger payloads are dropped, the connection stays);
  - `max_packet_size 327688` (= 262144 + 65544: a packet with a full payload and the longest topic passes).
  - This follows the proposal of SF-105 (D-1). The test `max_packet_size_limits_the_payload` pins it.
- **A-45 Sample storages:**
  - `generated/roots/oec-dev-app` (credentials, 8883) and `generated/roots/oec-dev-cert` (certificate + encrypted key + passphrase, 8884), laid out per RIG 4.1, read by `Bootstrap::ContainerStorage { root }`.
  - `broker.json` has exactly the three keys of V-BRK-001 … 018, no new format.
  - `Address` is `localhost` (apps run on the host); in a container on the compose network use `mosquitto`.
  - No `broker_ca.pem`, so the fallback to `ca.pem` is used.
- **A-46 Opt-in contract:**
  - The interop tests are no-ops unless `OEC_INTEROP=1`. The gate compiles and lints them but needs no engine and no network.
  - `oec/dev/interop.sh`:
    - checks the container engine (none → note, exit 0);
    - generates the PKI if missing;
    - starts the broker, runs the tests single-threaded, removes the broker (`OEC_INTEROP_KEEP=1` keeps it).
  - `OEC_COMPOSE` selects the compose command (Podman).
  - Image pulls happen only here.
- **A-47 Test design:**
  - A probe client (on the `oec-core` codec) watches `Oi4v2/#` like a second application.
  - For the Will, the test binary starts itself as a child process and kills it (no Close, no DISCONNECT possible).
  - For the broker restart the probe keeps a persistent session and ignores redelivered messages older than the restart (by MessageId time), so a stale Will cannot pass the test.
- **A-48 Smoke test and dependencies:**
  - `explicit_broker` (explicit `Broker` with CA and credentials against the dev broker) replaces the env-gated smoke test of C-050 (`OEC_SMOKE_*` removed).
  - No new Rust or Go dependency; ADR-0023 unchanged.
  - No SDK defect was found. Two test defects were fixed before the commit: the AppId in the topic is base64url-encoded, and QoS 1 redelivery after the restart.
- **A-49 Cycle cut:**
  - C-051 not split.
  - Next: the P5 interleave (C-052: D7 `internal/mqttwire`, capture, `monitor --replay`, then `monitor` live against this broker), then increments 6–8 and 10.
  - C-050 risks closed: interop with a real broker, reconnect after a broker restart. Open as before: Windows/ARMv7 builds (CI), heartbeat timing (core tests), no offline queue (increment 6).

### A-50 – D8 T8-BUS-01 covers REQ-CNT-003 (C-051 review, agent)

- **Where:** `oec/conformance/catalog/d8.yaml`, test T8-BUS-01 (broker protocol, TLS and certificate).
- Before, D8 checked REQ-CNT-003 only on the host (T8-HST-01: the content of `broker.json`). SF-105 adds a rule for the broker itself.
- T8-BUS-01 now also publishes a payload of exactly `MaxPacketSize` on a long topic. It expects that the message is delivered and the connection stays open.
- Larger payloads are not checked (SF-105 (c)). The test is `planned` (P5b).

### A-51 … A-62 – P5 interleave (C-052, agent)

- **Where:** `oec/conformance` (`internal/capture`, `internal/mqttwire`, `internal/mqttwire/mqtttest`, `internal/verdict`, `internal/monitor`, `internal/report/monitor.go`, `cmd/oec-check/monitor.go`, `live.sh`), ADR-0025 amendment C-052, `oec/conformance/DESIGN.md` "As built (C-052)"; details in the [C-052 log](log/2026-10-26_C-052_d7-monitor.md).
- **A-51 Cycle cut:** one cycle, not split. Capture and the monitor engine are small next to `validate`; the live part needed only the client and the dev broker of C-051.
- **A-52 Dependency:**
  - `github.com/eclipse/paho.mqtt.golang` v1.5.1 (EDL-1.0, as ADR-0025 allows) pinned in `go.mod`/`go.sum`; indirect `golang.org/x/net` v0.61.0, `golang.org/x/sync` v0.24.0, `github.com/gorilla/websocket` v1.5.3 (BSD).
  - Toolchain go1.27.3 (required by x/net).
  - govulncheck: no vulnerabilities. It is run by hand before a dependency change (like `cargo deny` updates), because it needs the vulnerability database and the gate is offline.
- **A-53 Test broker:** an own in-process TLS broker `mqtttest` (about 400 lines of test code on the paho codec) instead of `mochi-mqtt/server`: no new dependency, Will on a dropped connection, `Subscribers()` for waits without sleeps.
- **A-54 TLS and keys:**
  - only `mqtts://`, TLS 1.2/1.3; the broker is verified against the CA file (`--ca` or the storage), without one against the system roots;
  - client certificate and/or credentials; encrypted PKCS#8 keys (PBES2, PBKDF2 with SHA-1/256/384/512, AES-CBC) decrypted in the module;
  - no auto-reconnect: a lost connection ends the run with a report error instead of hiding a gap;
  - `--insecure` only explicit, warned on stderr and recorded in the report (G-4.5);
  - secrets only from files (`--password-file`, `--key-passphrase-file`, container storage).
- **A-55 Capture format v1:** a header (format version, tool, schemas and baseline, broker, `insecure`, subscription, MaxPacketSize, start), one record per message `{"topic","payload"|"payload_base64","size","received","qos","retain"}`, a trailer; the reader is strict (no time going back, nothing after the trailer). `validate` reads captures, too.
- **A-56 Verdicts:** severity from the REQ keyword × level; a not-applicable skip does not worsen a test; a MUST that cannot be judged (inconclusive) gives exit 4 unless `--allow-skip`.
- **A-57 Rule decisions:**
  - Will detection: a Health about the component whose MessageId time is not newer than its last Birth (REQ-LIFE-016: the Will is prepared at connect).
  - Heartbeat only for the Health about the component itself, restarted by any own Health or the Birth; 60 s + 5 s; a run shorter than 125 s is inconclusive (as SF-106 decided in the C-052 review).
  - REQ-RES-HEALTH-004 cannot be observed on the bus → not-applicable skip.
  - T7-MON-10 (IL uniqueness) is a heuristic → info.
  - T7-MON-12 derives the publisher QoS from the delivered QoS (D7 subscribes with QoS 1).
- **A-58 Deferred:** T7-MON-09, 13 … 17 need the PublicationList/Config state model that `test` builds anyway; they stay `planned`.
- **A-59 Fixtures:** generated captures + `index.yaml` instead of `internal/sim`; a test requires a positive and a negative fixture per implemented monitor test and that the catalog status equals the engine (→ R-1).
- **A-60 CLI:** `--level` default 3 (REQ-CMP-011: a full claim is L3), `--duration` default 5 min, `--app` by AppId or PublisherId, `--config` YAML (the command line wins), MaxPacketSize from flag > storage/capture header > 262144, `--bus-storage` as in DESIGN.md, new `--key-passphrase-file` and `--client-id`.
- **A-61 Reports:** monitor JSON report version 1, text, JUnit (one testcase per monitor test, planned ones `skipped`); `report.schema.json` is deferred to `test`, so one schema covers both commands.
- **A-62 Live tests:** opt-in `oec/conformance/live.sh` (`OEC_LIVE=1`, pattern of `oec/dev/interop.sh`), in the module that owns the test; the gate needs no Docker or network. Run once green on 2026-10-26: both roots (client certificate with encrypted key on 8884, credentials on 8883) saw 5 records, 2 Births and 1 Will, exit 4 (6 s cannot judge the heartbeat), 6.09 s.

### A-63 … A-74 – P4 increment 6a (C-053, agent)

- **Where:** `oec/sdk/rust/oec-core/src/provider.rs`, `provider/pagination.rs`, `provider/writers.rs`, `oec/sdk/rust/oec/src/{asset.rs, app.rs, runtime.rs, identity.rs}`, the tests; ADR-0020 amendment C-053, ADR-0026 §12, ADR-0023 amendment C-053; details in the [C-053 log](log/2026-10-27_C-053_provider-registry-get.md).
- **A-63 Cycle cut:** increment 6 is split. 6a (this cycle) is about 3200 lines with tests and complete for an application without DataSets; 6b (C-054) adds Data DataSets, Metadata, publish modes, intervals, Precisions and the UUIDv5 DataSetClassId on top, without changing 6a. The order 6 → 7 → 8 → 10 stays: 7 and 8 need the PublicationList and `Get` path of 6.
- **A-64 Sans-IO provider:** `oec_core::provider::Provider` owns the lifecycle of C-049 and the registry of sources; the runtime only feeds it (connect, timer, messages) and publishes what it returns. Pagination and the writer table are pure modules with their own tests, so the gate tests all of it without network.
- **A-65 Offline:** nothing is queued (ADR-0020 §7 "state, not messages"). A change while offline only marks the source; after the reconnect the current state of every source is published. A consumer never gets a backlog of outdated states, and memory does not grow while offline. This closes the offline-queue risk of C-050.
- **A-66 Order:** Birth → Health of the application → per asset (in the order of registration) MAM, Health and, from Level 2, Profile → Profile of the application → PublicationList. Only the application has the 60 s heartbeat (SF-106); assets publish on change and on connect.
- **A-67 Profile and PublicationList** (Level 2+): the Profile lists `MAM, Health, Profile, PublicationList`; the PublicationList has per source the entries MAM, Health and Profile with `ON_REQUEST_SOURCE_10` (REQ-RES-PUBLIST-026: combined default), without Interval/Precisions/Config; it does not list itself. All its DataSetMessages share one DataSetWriterId (D7 T7-MON-08). Level 1 serves neither and subscribes no `Get`.
- **A-68 DataSetWriterIds:** MAM 10 and Health 11 of the application (lifecycle); every other publication (resource, Source, Filter) gets the next free id up to 65535. The table is stored under `app/writer-ids` after every change and an id is never reused, also after `remove` (REQ-RES-PUBLIST-006). A corrupt table stops `build()` with `Error::Io` instead of renumbering, because consumers rely on the ids.
- **A-69 Assets:**
  - MAM defaults HardwareRevision `"undefined"` (REQ-RES-MAM-008: a physical asset with unknown revision) and DeviceClass `""`, unless the `Product` sets them; Health NORMAL_0 / 100 until set.
  - RevisionCounter: managed by the SDK (persisted, starting at 0, `bump_revision_counter`) or taken from the VendorNameplate (`AssetDef::revision_counter`, fixed; −1 = none).
  - Store key `assets/<FNV-1a 64 of the IL>/revision-counter`: an IL has up to 255 characters, some outside the key alphabet (open since C-048). A hash collision is negligible and accepted.
- **A-70 Limits and pagination:** `Get` answers are paginated against the `MaxPacketSize` of the Message Bus, unsolicited publications against the application's own lower limit if set (REQ-PAG-005). Without PaginationRequest: one message if it fits, else all pages with the largest PerPage that fits (REQ-PAG-007); a requested PerPage is honored or lowered (REQ-PAG-004); Page 0 = all pages (REQ-PAG-002). D7 T7-MON-18 accepts the result.
- **A-71 Unknown Source:** a `Get` for a Source the application does not represent gets no answer on the Pub topic (MUST of REQ-CMP-047). The recommended Status event `BadNotFound` (SHOULD) needs the event machinery of increment 7 and comes there (ADR-0020 §6 stays the target).
- **A-72 API (ADR-0020 amendment C-053):**
  - additions `AppHandle::assets`, `Asset::mam`, `Asset::is_this`, `HealthDetail::new`;
  - `bump_revision_counter` returns `Result<()>` (sketch: `()`), because the persisted counter can fail;
  - `mam()`/`identification_link()` moved from `App` to `AppHandle` (`App` derefs to it), `mam()` returns an owned value;
  - no heartbeat for represented assets (SF-106; the sketch said "automatic" for every asset).
- **A-73 Tests:**
  - `oec-core`: 10 provider tests (2 property tests: page plan; every generated payload passes the strict schema) and 8 unit tests;
  - `oec`: asset and persistence tests; 3 runtime tests against the in-process TLS broker (Level 2 sources and `Get` answers, changes at once, offline changes as state after a reconnect);
  - opt-in interop `level_2_assets_and_get_answers` against Mosquitto: `interop.sh` 9 passed;
  - one-off D7 `oec-check monitor` (external call, Q-009) during that test: 10 pass, 0 fail, 3 skip (exit 4: the heartbeat needs ≥ 125 s; T7-MON-03/07 concern only the probe). The capture is not committed: D7 fixtures are generated from scenarios, a recorded capture would need a new fixture kind (P5). So there is no new test data to review.
- **A-74 Dependencies:** none new; `Cargo.lock` and `deny.toml` unchanged, `cargo deny` without exception. 6b will add the uuid feature `v5` (ADR-0023 amendment then).

### A-75 … A-86 – P4 increment 6b (C-054, agent)

- **Where:** `oec/sdk/rust/oec-core/src/provider.rs`, `provider/{data.rs, sets.rs, versions.rs, pagination.rs}`, `oec/sdk/rust/oec/src/{data.rs, app.rs, asset.rs, runtime.rs}`, the tests, the sketch (`sketch/src/data.rs`), the D7 monitor (`oec/conformance/internal/monitor/monitor.go`); ADR-0020 amendment C-054, ADR-0026 §13, ADR-0023 amendment C-054; details in the [C-054 log](log/2026-10-27_C-054_data-sets-metadata.md).
- **A-75 Cycle cut:** 6b fits one cycle (about 2400 lines of code, 1500 lines of tests; one topic: the Data path). `Set PublicationList` (REQ-RES-PUBLIST-016 …) stays in increment 7, because it needs `Set` handling and Status events; `Publication::writable` is already listed as the entry's `Config`. Order 7 → 8 → 10 kept; 7 is re-cut at its start (possibly 7a events/Status + `Set PublicationList`, 7b Config).
- **A-76 Modules:** `provider::data` (types, DataSetClassId, Metadata, coercion, encoding, dead band), `provider::sets` (state of one DataSet), `provider::versions` (ConfigurationVersions) in `oec-core`; `oec::data` wraps them. The version table is stored under `app/metadata-versions` after every change; a corrupt table stops `build()` (as the writer table, A-68).
- **A-77 DataSetClassId:** `UUIDv5(namespace, JSON [tag, [[name, builtInTypeId, valueRank], …]])`, namespace `UUIDv5(URL, "https://schemas.openindustry4.com/oec/v2/sdk/data-set-class")`. Stable over restarts and SDK versions, changes with the structure (REQ-MSG-013); unit, description and range do not change it. `DataSetDef::class_id` sets an explicit one.
- **A-78 Metadata content and versions:** one FieldMetaData per field (BuiltInType, DataType, ValueRank −1, Description, FieldId = UUIDv5 of DataSetClassId and name), properties `EngineeringUnits` (UNECE namespace, **UnitId −1** = not given, DisplayName = the unit) and `EURange`. The guideline example uses a real UNECE UnitId (20529); a `Field::unit_id` can be added later without breaking the API. ConfigurationVersion: VersionTime (seconds since 2000); both parts change with the structure, Minor with any other change; the Timestamp of the Metadata is the first sending of its version. Data messages carry the full version as MetaDataVersion.
- **A-79 Metadata publication:** on its Filter topic (fixed `ON_REQUEST_FILTER_9`) at `add_data`, after every connect and on a version change, always before the Data (REQ-RES-META-003), at every level. `Get Metadata` is answered only with Source and Tag of a DataSet that is not `OFF_0`; otherwise silence (REQ-CMP-044: Data not announced in the PublicationList). The Metadata size is checked against `MaxPacketSize` at `add_data`.
- **A-80 Complete value sets:** a Data DataSetMessage carries all fields of its Metadata, so a DataSet is published and answered only once every field has a value; `update` is all or none. Values are converted to the field type (an integer fits every numeric type whose range holds it; Guid and LocalizedText from strings); Int64/UInt64 and NaN/±Infinity are JSON strings (OPC UA JSON). DataSetWriterName = Source, WriterGroupName = Tag, Timestamp = last value change.
- **A-81 Publish forms:** FILTER publishes the DataSet alone on `Pub/Data/<Source>/<Tag>`; SOURCE and APPLICATION publish, when one member is due, every member with values on `Pub/Data/<Source>` or `Pub/Data` (the state of the scope), and the other members restart their dead band from these values. Mixed DataSetClassIds → none in the NetworkMessage. `OFF_0`: never sent or answered; the Data entry stays listed with `OFF_0`, the Metadata entry is omitted.
- **A-82 Interval and dead band:** `Interval` > 0 publishes the state cyclically and ignores Precisions (REQ-RES-PUBLIST-014); missed cycles are not caught up; the first complete set and every connect publish at once. `Interval` 0 publishes a change when a numeric field moves by more than its Precision (no Precision: any change), non-numeric or non-finite changes always. The runtime injects monotonic time, so tests drive intervals without sleeping.
- **A-83 `Get Data`, Profile, PublicationList:** `Get Data` answers the current state of the addressed scope, paginated, with `ReplyTo`; an unknown Tag gives an empty list (SF-099). The Profile lists Data and Metadata; Level 2 subscribes `Get/Data/#` and `Get/Metadata/#`. The PublicationList lists per DataSet Data (Mode, Interval, Precisions, Config) and Metadata (same DataSetWriterId). `remove` republishes the PublicationList and keeps the id and the version reserved.
- **A-84 API (ADR-0020 amendment C-054):**
  - additions `Field::range`, `DataSet::{tag, source, values}`, `Mode::union`, typed `Value` variants for every `BuiltinType`;
  - `DataSet::update` returns `Result<()>` (sketch: `()`): an unknown field or a wrong type must not be dropped silently;
  - `Value::Json` (escape hatch) dropped, `Value` is `non_exhaustive`: pre-encoded JSON cannot be checked against the field type;
  - `oi4pv(pv)` names the field `Pv`; `class_id` and `interval` are checked at `add_data`;
  - `on_set`/`SetData` stay in the sketch (increment 7).
- **A-85 Tests:**
  - `oec-core`: 12 data tests (modes, groups, interval, dead band, `Get`, versions, the pinned DataSetClassId, a strict-schema test of every generated payload) and 9 unit tests;
  - `oec`: declaration, persistence and refused definitions (`tests/app.rs`); `data_sets_publish_their_state` against the in-process TLS broker;
  - opt-in interop `level_2_data_sets_and_metadata` against Mosquitto: `interop.sh` 10 passed;
  - one-off D7 `oec-check monitor` (external call, Q-009) during that test: first 2 failures, both not in the SDK: the test reused a MessageId for two `Get`s (test fixed), and **D7 T7-MON-11 reported an unsolicited Metadata as "without content"**, because it looked for DataSetMessages, while a ua-metadata message carries its content in `MetaData`. Fixed in D7 with a test first (`TestUnsolicitedMetadataHasContent`); the 18 reviewed fixtures and their verdicts are unchanged (they only had solicited Metadata). Rerun: 10 pass, 0 fail, 3 skip (exit 4 as in C-053). Capture not committed (A-73).
- **A-86 Dependencies:** the uuid feature `v5` (SHA-1 name-based UUIDs) adds sha1_smol 1.0.1 (BSD-3-Clause, no dependencies); `Cargo.lock` 192 packages; `deny.toml` unchanged; `cargo deny` without exception.

## Next (C-055)

1. P4 increment 7 – write side: `Set`/`Del` with Status events, the Config resource, `Set PublicationList` (`Publication::writable`), Status `BadNotFound` for a `Get` of an unknown Source; cycle cut first. Runs in a new agent (start prompt in `prompt.md`).
