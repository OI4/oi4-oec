# Review and decisions – open items for the PO

_Last updated: 2026-10-31, C-072 review addendum 3: you accepted **R-2** ("R-2 akzeptiert"); REQ-PLT-001 and all 55 host scenarios are `reviewed`. D-1 (SF-116) and R-1 were answered and applied in addenda 1–2. **Nothing is open.** A-259 … A-270 for information._

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
- C-055 (P4 increment 7a; increment 7 is split into 7a, 7b and 7c, the SubscriptionList moves to 8):
  - `oec-core`: the write side of the provider – `Set`/`Del` subscribed from Level 2, every request answered with a Status event (`Pub/Event/<Source>/Status/<Level>`, `ReplyTo`), `BadNotFound` for requests to an unknown Source, `Set PublicationList` within `Publication::writable` (persisted), `Set Data` in two phases.
  - `oec`: `DataSet::on_set` returns `Result`, `SetData::{get, fields}`; the handler decides, the SDK answers (Pub Data + Status Good, or Status Bad with the handler's code); settings persisted under `app/publications`; the sketch is aligned (ADR-0020 amendment C-055).
  - Tests: core and runtime tests in the gate (incl. the guideline vectors V-PAY-061/062 of MG B1.1.4); opt-in interop 11 green; a one-off D7 `oec-check monitor` run against the SDK: 9 pass, 0 fail, 4 skip.
  - **Two findings to decide: D-1 = SF-107** (how a requester sees that a DataSet is writable) and **D-2 = SF-108** (which Source the Status event for a request to an unknown asset carries; this caused one of the D7 skips). No reviewed item changed, no new test data, no new dependency; A-87 … A-97 are for information. Worth a look: **A-87** (cycle cut), **A-91** (unknown Source), **A-95** (`Set Data` rules).
  - C-056 (increment 7b, Config) runs in a new agent; `prompt.md` is filled.
- C-056 (P4 increment 7b, not split):
  - `oec-core`: Config DataSets in the provider (`provider::config`) – the definition check, the value checks incl. Pattern (new dependency `regex`), masking of sensitive values in every publication, `Get Config` and `Set Config` from Level 2 with partial updates and the status codes of 7a, the table of set values.
  - `oec`: the API `oec::config` (`ConfigDef`, `ConfigValues`, `ConfigHandle` with `on_set` returning `Result`, `set`, `remove`) and `Asset::add_config`; values persisted under `app/config`; the RevisionCounter goes up with every change; the means of authentication under `/run/secrets/oi4` are read again before every reconnect (GL-ADR-022); the file store is owner-only (0700/0600). The sketch is aligned (ADR-0020 amendment C-056).
  - Tests: core and runtime tests in the gate (incl. the guideline vectors V-PAY-098 … 102/114 of MG B1.1.3); opt-in interop 12 green; a one-off D7 `oec-check monitor` run against the SDK: 10 pass, 0 fail, 3 skip, after fixing a D7 bug found by that run (T7-MON-10 took a MAM published on change for a second Birth; test first, reviewed fixtures unchanged).
  - No new finding, no reviewed item changed, no new test data, so there is nothing new to review. **D-1 (SF-107) and D-2 (SF-108) are still open.** A-98 … A-108 are for information. Worth a look: **A-103** (masking), **A-104** (sensitive values at rest), **A-105** (secret reload), **A-106** (regex dialect).
  - C-057 (increment 7c) runs in a new agent; `prompt.md` is filled.
- C-056 review: your answer ("D-1 und D-2 sind reviewed und akzeptiert") is applied.
  - D-1: MG 9.1.9 and B1.1.3/B1.1.4 in `580f1c6` as proposed, **SF-107 resolved**; D-2: MG 9.1 in `2e32a6b` as proposed, **SF-108 resolved** (register `d2dc9ca`, no open finding). Re-baseline on `2e32a6b` (1403 inventory items, 441 REQs).
  - REQ-CMP-047 and REQ-CMP-043 follow the new MG 9.1 text, the new REQ-RES-SUBLIST-016 states MG 9.1.9 (D7 T7-SET-01); all three are `draft` (**R-1**).
  - SDK (`8591929`, test first): the Status event for a request to an unknown Source is published under the `AppId` (A-91 replaced, **A-109**); the interop test `level_2_config` no longer races the SUBSCRIBE (`6afd879`); `oec/dev/interop.sh` 12 passed, 1 ignored; one-off D7 `monitor` 10 pass, 0 fail, 3 skip – T7-MON-04 now passes with an unknown Source asked.
  - SF-107 changes the SDK only with the SubscriptionList (increment 8). C-057 (7c) still runs in a new agent; `prompt.md` is updated.
- C-056 review addendum 2: you accepted R-1 ("R-1 reviewed"); REQ-CMP-043, REQ-CMP-047 and REQ-RES-SUBLIST-016 are `reviewed` (`ee9f0e9`). No drafts, no open finding, nothing open.
- C-057 (P4 increment 7c, not split; order ReferenceDesignation → Generic events → Syslog bridge):
  - `oec-core`: `provider::refdes` – `Get`/`Set`/`Del ReferenceDesignation` from Level 2 on the write path of 7a (A-92 ends for it), `Get` with the Query of the direct children, the stored table; `provider::events` – Generic events (`CAT_GENERIC_99`) and Syslog events of the application with persisted DataSetWriterIds and PublicationList entries, Syslog Levels switchable by `Set PublicationList`.
  - `oec`: `Asset::on_reference_designation`/`reference_designation`, `AppBuilder::reference_designation`, persisted under `app/reference-designations`, RevisionCounter + 1 per change; `GenericEvent` + `Asset::emit_event`; `AppBuilder::syslog` and the `log` bridge (`event::install_bridge`): SDK and transport records excluded, no feedback loop, rate limit, MSG ≤ 1024 bytes. The sketch is aligned (ADR-0020 amendment C-057).
  - Tests: core and runtime tests in the gate (incl. the guideline vectors V-PAY-041 … 046/053/054 of MG B1.1.10 and the event shapes of V-PAY-064/065), the Syslog test in its own binary; opt-in interop 13 green; a one-off D7 `oec-check monitor` run against the SDK: 9 pass, 0 fail, 4 skip.
  - No new finding, no reviewed item changed, no new test data, no new dependency: **nothing open.** A-110 … A-119 are for information. Worth a look: **A-113** (a ReferenceDesignation change increments the RevisionCounter – if you read MG 5 differently, it becomes SF-109), **A-115** (events are not queued offline), **A-117** (the `log` bridge; applications must not log secrets).
  - C-058 (increment 8, consumer side and SubscriptionList) runs in a new agent; `prompt.md` is filled.
- C-058 (P4 increment 8a; increment 8 is split into 8a and 8b):
  - `oec-core`: a sans-IO consumer (`consumer`) – subscriptions with reference-counted MQTT filters, requests published only after the broker acknowledged the subscriptions their answers arrive on, correlation over ReplyTo, paginated answers reassembled in page order, the Parent check of a Query answer (REQ-RES-REFDES-015), the Status event of `Set`/`Del`, and a directory of the applications on the bus (Birth, Close, Will, heartbeat).
  - `oec`: the API `oec::consumer` – `Consumer::{subscribe, on_presence, apps, get, set, del}` with `_blocking` twins, `Target`, `Received`, `Presence`, `GetOptions`, `WriteOptions`; new errors `Error::Status` (a Bad answer) and `Error::NotConnected` (offline, nothing queued). The sketch is not aligned, because 8b deletes it; ADR-0020 amendment C-058 records the differences.
  - Tests: core tests with the guideline request vectors and a proptest; runtime tests with two applications over a routing mode of the in-process test broker; opt-in interop 14 green (new `consumer_asks_a_provider`); a one-off D7 `oec-check monitor` run: 11 pass, 0 fail, 2 skip, the consumer's requests all schema-valid.
  - No finding, no reviewed item changed, no new test data, no new dependency: **nothing open.** A-120 … A-127 are for information. Worth a look: **A-120** (cycle cut), **A-124** (no queue while offline), **A-126** (API differences against the sketch).
  - C-059 (increment 8b: SubscriptionList, the Action `NewDataSetWriterId`, deletion of the sketch, example app) runs in a new agent; `prompt.md` is filled.
- C-059 (P4 increment 8b; split once more: the Action `NewDataSetWriterId` and the farewell of the sketch move to 8c):
  - `oec-core`: the SubscriptionList of the provider (`provider::sublist`) – one entry per lasting subscription of the consumer side and one `NONE_0` entry per DataSet that accepts `Set` (SF-107); `Get`, `Set` (Interval) and `Del` from Level 2 with Status events (**A-92 ends**); published again after every change; the changes are persisted and reach the consumer (Interval throttle, a deleted subscription is unsubscribed).
  - `oec`: `Subscription::interval` and `Subscription::configurable` (as in the sketch); the store key `app/subscriptions`; the example app `oec/sdk/rust/oec/examples/demo.rs` (provider + consumer, the application under test of D7 `test`).
  - A defect of C-058 found by the gate under load and fixed (test first): a request that reused an answer subscription just released waited until its timeout.
  - Tests: core and runtime tests in the gate (incl. the guideline vectors V-PAY-072 … 075 of MG B1.1.9); opt-in interop 14 green; a one-off D7 `oec-check monitor` run against the demo app and the interop consumer: 12 pass, 0 fail, 1 skip (no Action yet), 0 invalid of 95 messages.
  - **Three findings to decide** (register `b35b41e`): **D-1 = SF-109** (are the short-lived answer subscriptions of requests listed?), **D-2 = SF-110** (MG 9.1.9 "other subscriptions to topics of the own AppId are not listed" vs. the example B1.1.9), **D-3 = SF-111** (must an application create a new entry on `Set`?). The SDK already follows the proposals. No reviewed item changed, no new test data, no new dependency. A-128 … A-136 are for information.
  - C-060 (increment 8c) runs in a new agent; `prompt.md` is filled.
- C-059 review: your answer ("D-1 … entspricht deiner Empfehlung", "D-2, D-3 Dein Vorschlag ist akzeptiert") is applied.
  - MG 9.1.9 in `57ebe91` as proposed: transient answer subscriptions are not listed (SF-109); only subscriptions to request topics of the own `AppId` are excluded, B1.1.9 stays (SF-110); an application that does not support the creation of entries rejects such a `Set` with `Bad`, e.g. `BadNotSupported` (SF-111). **SF-109 … SF-111 resolved** (register `7a2bda2`, no open finding). Re-baseline on `57ebe91` (1405 inventory items, 441 REQs, 0 gaps).
  - REQ-RES-SUBLIST-004, -010 and -016 follow the new text and are `draft` (**R-1**). Schema `2.0.1-dev` @ `57ebe91` without content change; D7/D8 catalogs on `57ebe91` (T7-SET-11 note: a rejected creation is no failure).
  - The SDK already did all three; nothing changes there. C-060 (8c) still runs in a new agent; `prompt.md` is updated.
- C-060 (P4 increment 8c, not split):
  - `oec-core`: the responder of the Action `NewDataSetWriterId` (`provider::action`) – from Level 2 every `ActionRequest` gets one final `ActionResponse` at once; the new id is reserved for 60 s (`Ttl`) in the persisted writer table and never handed out again; at most 256 open reservations; `NewDataSetWriterId` in the Profile. The requester in the consumer core (CorrelationData, RequestId, TimeoutHint, only the final response counts).
  - `oec`: `Consumer::new_data_set_writer_id(resource, &Target, &ActionOptions) -> ProvidedWriterId` and `_blocking` (ADR-0020 amendment C-060: differs from the sketch's signature).
  - **The sketch is gone:** its examples are in `oec/sdk/rust/oec/examples/` (next to `demo.rs`), the C contract `oec.h` and the C examples in `oec/sdk/rust/oec-ffi/`, the REQ walker in `oec/tests/citations.rs`; the gates are adapted.
  - Tests: core and runtime tests in the gate (guideline vectors V-PAY-008/009, 185, 186, 197, 199, 200, 203); opt-in interop 14 green (after a fix of the test setup); a one-off D7 `oec-check monitor` run: 13 pass, 0 fail, **T7-MON-07 now passes** (the Action), 0 invalid of 101 messages.
  - **Two findings to decide** (register `19c1a95`): **D-1 = SF-112** (what is a provided DataSetWriterId used for, and may it be handed out again?), **D-2 = SF-113** (does the application answer a `Call` for an unknown asset?). No reviewed item changed, no new test data, no new dependency. A-137 … A-144 are for information.
  - Increment 10 (`oec-ffi`) runs in a new agent; `prompt.md` is filled.
- C-060 review (addendum 1): you accepted D-1, D-2 and R-1 of C-059.
  - R-1 of C-059: REQ-RES-SUBLIST-004, -010, -016 `reviewed`.
  - Guideline `eb7889c`: MG 8.4.1 (the `Set` that creates a `DataSet` carries the provided id; a listed id stays) and MG 9.2.1 (unknown Source → `BadNotFound`, empty Payload); register `4fac7d2`, SF-001 … 113 resolved, no open finding.
  - Re-baseline: 1406 items, 442 REQs, 0 gaps; **new REQ-ACT-024**, notes on REQ-ACT-017/023 → **R-1**. Schema `2.0.1-dev` @ `eb7889c` (no content change), D7 test T7-ACT-04 (`planned`) for REQ-ACT-024.
  - SDK: no behaviour change (it already did both); cites REQ-ACT-024.
- C-060 review addendum 2: you accepted R-1 → REQ-ACT-017, -023, -024 `reviewed`; 0 drafts, nothing open.
- C-061 (P4 increment 10, not split):
  - **The C ABI `oec-ffi`** (`oec/sdk/rust/oec-ffi`): a shared and a static library over the whole Rust API (provider, Config, events, consumer, `Get`/`Set`/`Del`, the Action), 46 C functions. The header `include/oec.h` is generated by cbindgen 0.29.4 (pinned) and compared with a fresh output in the gate. It keeps the contract of P3 with a few documented changes (ADR-0021 amendment C-061, A-147 … A-151).
  - Small additions to the Rust API for JSON values (ADR-0020 amendment C-061, A-146).
  - Tests: `hello.c`, `logger.c` and an ABI test program in C run against the in-process TLS test broker, against the shared library and against the static library under AddressSanitizer + UBSan (required in the gate), incl. a stress phase with 6 threads; once on Linux with LeakSanitizer and cross builds for Linux amd64, Windows (GNU) and ARMv7. Opt-in interop 14 green; the C examples once against Mosquitto.
  - **P4 exit criteria checked** (log): 1, 2, 4, 6 met; 3 met except D7 `test` (P5); 5: gate green, but Windows MSVC and native amd64/Windows runs need the CI (P9) → **D-1**.
  - No finding, no reviewed item changed, no new test data, no new dependency. A-145 … A-152 are for information.
  - The next cycle runs in a new agent; `prompt.md` is filled.
- C-061 review: your answer ("D-1 (a)") is applied: **P4 is closed** (ROADMAP P4 heading and exit assessment, STATUS). Criterion 3 (D7 `test` against the SDK example app) is part of the P5 exit, the platform part of criterion 5 (Windows MSVC, native amd64/Windows runs) moves to the P9 CI. **Next phase: P5** (D7 `test`, proxy, `list`/`explain`), started by a new agent in C-062.
- C-062 (P5 start and increment 1):
  - **P5 plan and cycle cut** in the ROADMAP: 8 increments in C-062 … C-068 (the rest of `Get`, the write side with the state model, the Action and the deferred monitor rules, proxy, restart hook/peer, `list`/`explain` + report schema, exit run).
  - **`oec-check test`** (bus attachment): D7 finds the component under test, sends `Get` requests as a not-announced tester, correlates the answers by `ReplyTo` and judges them; the monitor rules run on the same subscription, a recorded run replays with `monitor --replay`. Engine `internal/probe`, simulated CUT `internal/sim` (MG B1.1 OT connector from reviewed vectors, with fault injection).
  - **T7-GET-01, 02, 05, 09 `implemented`**: Get MAM/Health for all Sources; the Profile lists exactly the served resources of the level; empty list and unknown Source (Event Status Bad); unknown keys ignored. Each with a positive and a negative sim fixture → **R-1**.
  - Found test-first and fixed: D7's own requests used QoS 1 instead of the default QoS 0 (REQ-MQTT-017).
  - Against the SDK example app `demo.rs` (level 2, copy of a dev root, dev broker): **14 pass, 0 fail, 3 skip, exit 0**, 18 requests all answered.
  - ADR-0025 amendment C-062; no SF, no reviewed item changed, no new dependency. A-153 … A-163 for information.
- C-062 review: you accepted R-1; the 8 sim fixtures in `testdata/sim/index.yaml` are `reviewed`. Nothing is open.
- C-063 (P5 increment 2):
  - **T7-GET-03, 04, 06, 07, 08 and T7-ADR-02, 03 `implemented`** – every `Get` test of the catalog: every resource with and without Source and Filter, PublicationList entries and Data ⇒ Metadata; every represented entity fulfills the level; pagination on request (by size passively, T7-MON-18); the Query of ReferenceDesignation; the Locale; no answers in the namespace `Oi4`; exact Identification Links.
  - 11 new faults of the simulated component (2 conforming variants, 9 faults); with the 3 fixtures that got outcomes of the new tests, 14 sim fixtures are `draft` → **R-1**.
  - Found test-first and fixed: two false positives of the monitor engine (REQ-LIFE-003 on an answer to a Get with a Locale, REQ-PAG-005 on the answer to a Get for one page). The capture fixture `pagination-broken` relied on the second one; it is corrected and `draft` again → **R-2**.
  - Against `demo.rs` (level 2): first 20 pass, 1 fail – a defect of D7 (T7-GET-07 expected a content where "ignored" means "answered like without"), fixed with a new positive fixture; then **21 pass, 0 fail, 3 skip, exit 0**, 82 requests all answered. The SDK was not changed.
  - ADR-0025 amendment C-063; no SF, no new dependency. A-164 … A-175 for information.
- C-063 review: you accepted R-1 and R-2; the 14 sim fixtures in `testdata/sim/index.yaml` and the capture fixture `pagination-broken` are `reviewed`. Nothing is open.
- C-064 (P5 increment 3):
  - **T7-SET-01 … 09, 11, 12 `implemented`** – the write side: Set and Del of Config, Data, Metadata, PublicationList, ReferenceDesignation and SubscriptionList, each answered with a Status event with the `ReplyTo` and, after a successful Set, the publication; rejected changes (unknown object, read-only property, `NONE_0` entries); Del of a whole object and of a part; the log level. T7-SET-10 (restart) stays in increment 6.
  - **D7 restores the component:** it writes back what the component published, deletes only what it created and undoes every change it provoked. Before the first and after the last write test it reads Config, PublicationList, ReferenceDesignation and SubscriptionList and compares them; the result is reported (`restore`), it is not a verdict.
  - The simulated component is writable; 13 new faults (2 conforming variants, 11 faults); with `none`, 14 sim fixtures went to you as R-1 (accepted in the C-064 review).
  - Found test-first and fixed: an invalid Status DataSetWriterId of the sim, invalid stimuli of D7, destructive requests without a known state, subscriptions left behind; after a code review three more (no write-back of a masked value at level 1, a ReferenceDesignation is deleted only after its write-back was accepted, a restore note when a run ends during the write tests).
  - Against `demo.rs` (level 2): **30 pass, 0 fail, 5 skip, exit 0**, 136 requests all answered, state restored (three runs). The SDK was not changed.
  - ADR-0025 amendment C-064; no SF, no new dependency. A-176 … A-190 for information.
- C-064 review: you accepted R-1; the 14 sim fixtures in `testdata/sim/index.yaml` are `reviewed`. Nothing is open.
- C-065 (P5 increment 4):
  - **T7-ACT-01 … 04 `implemented`** – D7 calls `NewDataSetWriterId`: one final response on the ResponseAddress with the CorrelationData, a new id ≥ 10 with a Ttl that the component does not use; no id again while it is reserved; no answer to unaddressable or undecodable requests; BadNotFound with an empty Payload for an unknown Source. The calls run inside the write window; a provided id is never used, so the restore check is unaffected.
  - **T7-MON-09, 13, 15, 16, 17 `implemented`** (18 of 19 monitor rules): one DataSetClassId per structure, Data/Event support of components with Sources, the list published again after a change, the RevisionCounter, the mandatory Config objects in a Set Config. **T7-MON-14 moves to C-066** (A-191).
  - 5 new faults of the simulated component (1 conforming variant, 4 faults) → **R-1**; 6 new capture fixtures (1 positive, 5 negative) → **R-2** (both accepted in the C-067 review). No reviewed fixture changed its verdict.
  - Found test-first and fixed: the report's list of requests left out the Action calls; besides, sim and fixture details (A-201).
  - Against `demo.rs` (level 2): **38 pass, 0 fail, 6 skip, exit 0**, 145 requests (142 answered; the 3 others are the deliberately unanswerable calls of T7-ACT-03), state restored. The SDK was not changed.
  - ADR-0025 amendment C-065; no SF, no new dependency. A-191 … A-201 for information.
- C-066 (P5 increment 5):
  - **Proxy attachment** `--proxy-listen`: the component connects to D7, D7 relays to the broker and sees the CONNECT, the Will, TLS and the login. An own small MQTT relay on the Go standard library (paho has no server side); no new dependency.
  - **T7-CON-01 … 04 and T7-LIFE-01 … 03 `implemented`**: MQTT 3.1.1 and a stable ClientId; TLS ≥ 1.2 and certificate verification (D7 offers TLS 1.1, an untrusted CA and a wrong host name – the component must refuse); login; no Retain; the Will on every connection (created anew); the Birth first after every connect and after a recovery; the Will published on an abnormal disconnect. Without the proxy these tests are skipped and the run is inconclusive (exit 4). **T7-MON-14 moves again, to C-067** (A-202).
  - 15 new faults of the simulated component (2 conforming variants, 13 faults) and `none` (now also judges CON/LIFE) → **R-3** (accepted in the C-067 review). Every sim fixture runs through the proxy; no other reviewed fixture changed.
  - Found test-first and fixed: two false findings of the monitor (T7-MON-03 on a late Birth, T7-MON-10 on a Birth after recovery); the demo run found a D7 race – the discovery Get overtook the component's subscriptions; D7 now waits for them (A-205).
  - Against `demo.rs` through the proxy (level 2): **46 pass, 0 fail, 5 skip, exit 0**; the demo refused all three bad TLS offers, reconnected after the drop with its Birth first; 145 requests (142 answered, the 3 others are the deliberate T7-ACT-03 calls), state restored. The SDK was not changed.
  - ADR-0025 amendment C-066; no SF, no new dependency. A-202 … A-214 for information.
- C-067 (P5 increment 6a):
  - **Restart hook** `--restart-cmd`, or `--stop-cmd` + `--start-cmd` (`--restart-wait`, default 2 min): you give D7 a command that stops the component gracefully and starts it again; D7 does not manage processes or containers itself. Without the hook the two restart tests are skipped and the run is inconclusive (exit 4).
  - **T7-LIFE-04 and T7-SET-10 `implemented`**: the Close message before a graceful disconnect (the last Health before the DISCONNECT, `NORMAL_0`/0 on the application's Health topic with its DataSetWriterId); a changed ReferenceDesignation survives the restart (D7 sets it before, reads it after and restores it).
  - **T7-MON-14 `implemented`** (19 of 19 monitor rules): publications follow the PublicationList – the topic forms of the Mode, no unsolicited publication for `ON_REQUEST_1`, the Interval, the Precisions, and a change seen in a Get answer is published (A-220).
  - **Cut** (A-215): increment 6 is split; the peer simulation, `--announce` and T7-ADR-01 follow in C-068; `list`/`explain`, the report schema and the P5 exit run in C-069.
  - 4 new faults of the simulated component (1 conforming variant, 3 faults) → **R-4**; 5 new capture fixtures (1 positive, 4 negative) and `lifecycle-pass` back to draft → **R-5** (both accepted in the C-067 review). The other reviewed fixtures keep their verdicts.
  - Against `demo.rs` through the proxy with the restart hook (level 2): **48 pass, 0 fail, 6 skip, exit 0**; LIFE-04 and MON-14 pass, SET-10 is not applicable (the demo has no ReferenceDesignation object); 146 requests (143 answered, the 3 others are the deliberate T7-ACT-03 calls), state restored. The SDK was not changed.
  - ADR-0025 amendment C-067; no SF, no new dependency. A-215 … A-221 for information.
- C-067 review: you accepted R-1, R-2 (C-065), R-3 (C-066), R-4 and R-5 (C-067); all 56 sim fixtures in `testdata/sim/index.yaml` and all 29 capture fixtures in `testdata/captures/index.yaml` are `reviewed`. Nothing is open.
- C-068 (P5 increment 6b):
  - **Peer simulation** `--peer`: D7 announces a simulated second component, a Level-1 `Utility` `oi4.org/oec-check/Peer/<random>` on its own connection – Will, Birth (MAM, Health), a "present" Health once the component under test is found (A-225), a heartbeat every 30 s, the Close message at the end. It answers Get MAM, Health and Profile; an unknown Source gets Event Status Bad `BadNotFound`.
  - **`--announce`** (your answer (a) to Q-029): D7 itself is announced the same way under its tester IL (`--tester-il` must then be an Oi4Identifier). Without the flag D7 stays unannounced as before.
  - **T7-ADR-01 `implemented`** – all 54 D7 tests are implemented: every request of the component to the peer must use the peer's request topic (its ServiceType REQ-TOP-006, its AppId REQ-TOP-010, the Get/Set/… topic of the addressed component REQ-BUS-002). Without `--peer` the test is an inconclusive skip (exit 4 unless `--allow-skip`), with the peer but without a request a plain skip (A-226).
  - The demo asks every application it sees for its MAM (SDK directory), so T7-ADR-01 is judged on it. The simulated component now does the same; 3 new sim fixtures (1 conforming variant, 2 faults) → **R-7**; `none` runs with peer and `--announce` and is back to draft → **R-6** (both accepted in the C-068 review). The other reviewed fixtures keep their verdicts.
  - Against `demo.rs` through the proxy with the restart hook, `--peer` and `--announce` (level 2): **49 pass, 0 fail, 6 skip, exit 0**; T7-ADR-01 passes (the demo asked the peer and D7 for their MAM on the right topics, both answered); state restored. The SDK was not changed.
  - ADR-0025 amendment C-068; no SF, no new dependency. A-222 … A-228 for information.
- C-068 review: you accepted R-6 and R-7; all 59 sim fixtures and all 29 capture fixtures are `reviewed`. Nothing is open.
- C-069 (P5 increments 7/8, not split, A-229):
  - **`oec-check list`** (the catalog: id, mode, level, status, needs, title, REQs; filters `--mode`, `--level`, `--req`, `--needs`, `--status`, id patterns; text or JSON), **`oec-check explain <test|REQ>`** (a test with its REQs, their keywords, levels and MG/RIG sources, and its fixtures with the expected outcome; a REQ with its statement, sources, quotes, verification, tests and fixtures) and **`oec-check report-schema`** – all from the data embedded in the binary (A-233).
  - **`report.schema.json`** freezes report version 1 of `monitor` and `test` (incl. `requests`, `restore`, `connections`, `announced`). The gate validates every report it produces: CLI tests, every capture replay, every sim fixture run (A-231). It found one D7 defect: the preparatory Profile read had an empty request label, now `discovery` (A-232).
  - **Coverage report** `oec/conformance/catalog/COVERAGE.md` (generated, checked by the gate): 55 tests, 0 uncovered `d7-monitor`/`d7-test` REQs, per test its fixtures by outcome; the gate now also enforces a negative and a positive fixture for every implemented test (A-234).
  - **D7 has 55 tests, not 54**: T7-ACT-04 came with the `eb7889c` re-baseline in the C-060 review; the docs since C-062 counted 54. Corrected in STATUS, ROADMAP, README, DESIGN (A-235).
  - **P5 exit run** against `demo.rs` (level 2, proxy, restart hook, `--peer`, `--announce`): **49 pass, 0 fail, 6 skip, exit 0**, state restored, the report valid against the schema; the demo asked the peer and D7 for their MAM. Cross builds for Windows/Linux/macOS × amd64/arm64 succeed. The SDK was not changed.
  - **P5 exit** (ROADMAP P5 "Exit assessment"): criteria 1–3 met, 4 (CI on Windows/macOS/Linux) with P9 → **D-1**.
  - No new or changed fixture (nothing to review), no SF, no new dependency (the schema uses the validator of `validate`). ADR-0025 amendment C-069; A-229 … A-238 for information.
- C-069 review: your answer ("D-1 (a)") is applied: **P5 is closed** (ROADMAP P5 heading and "Decision" paragraph, STATUS). Criterion 4 (CI on Windows/macOS/Linux runners) and the D7 container image move to the P9 CI (new P9 bullet). **Next phase: P5b** (D8 `oec-infra`), planned by a new agent in C-070.
- C-070 (P5b plan and increment 1, A-247):
  - **P5b plan** in ROADMAP P5b: six increments in C-070 … C-075 – `image`, `bus`, `host`, Engine API + `container`, `run` with purpose-built test images, `list`/`explain`/coverage + frozen report + exit run. **Exit made precise:** every test implemented with reviewed positive and negative fixtures, offline gate; `image` and `run` pass on a purpose-built good image and fail with the expected REQs on purpose-built faulty images (`run` opt-in with Docker); `bus`/`host` opt-in against `oec/dev`; frozen report schema, `list`/`explain`, coverage, cross builds. "Our demo and registry images pass `level3`" moves into the P6/P7 exits, CI into P9.
  - **Design check** (DESIGN §9, ADR-0025 against the as-built D7): `catalog`, `verdict` and `report` shared; D8 gets its own `infra-report.schema.json` instead of a new version of the frozen D7 schema; `explain`/coverage for D8 come with increment 6; OCI read with the standard library instead of `go-containerregistry`, no registry access (ADR-0025 amendment C-070).
  - **`oec-infra image`** (T8-IMG-01 … 04 `implemented`, 4 of 17 D8 tests): reads an OCI layout directory or an oci-archive offline (no Docker daemon, no network), checks OCI format and digests, the platforms (linux amd64/arm64/arm, multi-architecture), exactly one SBOM in `/opt/oi4/licenses` with a text for every license, SHA256 digests; text/JSON/JUnit; `report-schema`, `version`. D7 output stays byte-identical.
  - **13 image fixtures** generated by a test (2 positive, 11 negative, `draft`) → **R-1**.
  - **One finding: SF-114** (RIG 4.1.4: SBOM versions and license texts) → **D-1**; D8 implements the proposal until you decide.
  - Opt-in try: `docker save` of Mosquitto on Docker 28 gives an oci-archive that `oec-infra image` reads (fails IMG-03, as expected for a non-OEC image). Cross builds ok. No new dependency, no reviewed item changed, D7 gate unchanged. A-239 … A-248 for information.
- C-070 review (log addendum 1): **R-1 accepted**, the 13 image fixtures are `reviewed`. Your answer to **D-1** ("alle Lizenzen lizenzkonform genannt", "CRA und RED Ready", versions not restricted, texts "rechtskonform") changes the proposal: SF-114 has a revised proposal in the register (`6d53556`) with the exact RIG 4.1.4 wording → **D-1 (revised)** to confirm. D8 keeps the C-070 behaviour until then.
- C-070 review addendum 2/3: your answer ("D-1 überarbeitet ok, einarbeiten") is applied.
  - RIG 4.1.4 in `a55f475` with the revised wording, **SF-114 resolved** (`3a86b5e`), no open finding. Re-baseline on `a55f475`: 1409 inventory items, 444 REQs, 0 gaps; schema `2.0.1-dev` @ `a55f475` (no content change).
  - REQ-CNT-013/014 follow the new text; new **REQ-CNT-026** (every license named unambiguously, MUST) and **REQ-CNT-027** (unique identifier per component, SHOULD); all four `draft`.
  - D8 T8-IMG-03 follows the decision (A-249): any SPDX/CycloneDX version, SPDX 3 JSON-LD readable, unreadable versions inconclusive, name/version/license per component, texts of license exceptions, identifier warns.
  - Image fixtures: 12 of the 13 reviewed ones changed bytes (versions and purls added) and are `draft` again, `invalid-sbom` has new content; 6 new ones → **R-2** (numbered on in the C-070 review because R-1 is answered).
- C-070 review addendum 4: you accepted R-2 ("R-2 akzeptiert"); REQ-CNT-013/014/026/027 and all 19 image fixtures are `reviewed`. Nothing is open.
- C-071 (P5b increment 2):
  - **`oec-infra bus`** (T8-BUS-01 … 03 `implemented`, 7 of 17 D8 tests): connects like `oec-check monitor` (`--broker`/`--bus-storage`, CA, client certificate, user/password, `--insecure` only explicitly). T8-BUS-01 probes TLS 1.2/1.3 (one must work) and 1.0/1.1 (must fail), a plain-text listener, the certificate chain against the CA, MQTT 3.1.1 and the MaxPacketSize of `broker.json` (SF-105). T8-BUS-02 lets a second, unprivileged client (`--credentials-2`/`--bus-storage-2`) send Set, Del and Call to D8's own probe AppId – none may arrive; reading Set/Config warns. T8-BUS-03 announces two probe components and asks the OEC Registry for their MAM and Health, after Close and Will they must be gone.
  - **29 broker scenarios** (6 positive) against the in-process test broker `mqtttest` (new options, additive, D7 gate unchanged) and a simulated Registry `registrytest` → **R-1**.
  - **One finding: SF-115** (MG 6.2.2: broker certificate for the broker address and within its validity) → **D-1**; D8 implements the proposal until you decide.
  - **Opt-in run against `oec/dev`** (now in `live.sh`, `TestBusDevBroker`): BUS-01 pass on 8883 and 8884 (TLS 1.2/1.3, MaxPacketSize 262144), BUS-02 warn REQ-RES-CONFIG-026 (a limitation of the dev broker, A-258), BUS-03 inconclusive (no Registry before P7). The dev broker got an ACL (`oec/dev/mosquitto/acl`).
  - Two defects found by the live runs and fixed (test first): the TLS probes did not present the client certificate (TLS 1.2 looked refused on 8884); the dev ACL lacked `Reply` (D7 T7-ACT-01/02/04 failed against the demo). Interop and the D7 demo run are green again.
  - No new dependency, no reviewed item changed, `COVERAGE.md` unchanged. `host` is not pulled forward (C-072). A-250 … A-258 for information.
- C-071 review (log addendum 1): **R-1 accepted** ("R-1 akzeptiert"), the 29 broker scenarios are `reviewed`. The C-071 docs counted 28 scenarios; `index.yaml` and the R-1 list had 29 (6 positive, 23 negative) – the count is corrected everywhere. D-1 is still open.
- C-071 review addenda 2–3: you decided **D-1 as proposed** ("D-1 wie vorgeschlagen"). Applied:
  - MG 6.2.2 in `b955783` (broker side: the broker certificate is valid for the broker address of `broker.json` and within its period of validity), **SF-115 resolved** (`db6a477`), no open finding.
  - Re-baseline on `b955783`: 1410 inventory items (+1), 444 REQs, 0 gaps; schema `2.0.1-dev` @ `b955783` (no content change).
  - REQ-MQTT-007 extended by the new sentence → `draft`.
  - D8 T8-BUS-01: same verdicts, the findings cite "(MG 6.2.2)" instead of "(SF-115, proposed)"; the notes of the scenarios `T8-BUS-01/wrong-host` and `/expired` follow → `draft`.
  - Together → **R-2** (numbered on in the C-071 review because R-1 is answered).
- C-071 review addendum 4: you accepted **R-2** ("R-2 akzeptiert"); REQ-MQTT-007 and the scenarios `T8-BUS-01/wrong-host`, `/expired` are `reviewed` – all 29 broker scenarios `reviewed`. No draft, no open finding. Nothing is open.
- C-072 (P5b increment 3):
  - **`oec-infra host`** (T8-HST-01, 02, 04 `implemented`, 10 of 17 D8 tests): reads the storages of RIG 4.1 below `--root` (default `/`) and judges content, certificates, secrets and – on Linux, from the mount table – read-only mounts (T8-HST-01 Message Bus storage and `broker.json` against the D1 schema; T8-HST-02 certificate and secret storages). T8-HST-04 measures the host D8 runs on: architecture, RAM, non-volatile memory, network, Linux and patch level (reported for a manual verdict), container daemon, broker and OEC Registry. Read-only; no secret or file content in the report.
  - **55 host scenarios** (7 positive) on generated roots with fake platform facts, mount tables and daemons, against `mqtttest` and `registrytest` → **R-1**.
  - **One finding: SF-116** (RIG 5.1: is "1 GB RAM / 4 GB non-volatile memory" 10^9 or 2^30 octets, nominal or usable?) → **D-1**; D8 implements the proposal with a manual band until you decide.
  - New verdict outcome on a requirement: `manual` (a measured value a human judges, e.g. the patch level); it does not make the test worse (A-263).
  - **Opt-in runs** (now in `live.sh`): both sample roots of `oec/dev` on this Mac and in a Linux container next to the dev broker: T8-HST-01/02 pass; T8-HST-04 fails for REQ-PLT-008 (no OEC Registry before P7) and, on macOS, REQ-PLT-003 (not Linux). No deviation in `oec/dev`.
  - A pre-existing defect fixed: `oec-check`/`oec-infra` did not compile for 32-bit ARM (ARMv7), A-269.
  - No new dependency, no reviewed item changed, `COVERAGE.md` unchanged, D7 gate unchanged. Increment 4 (Engine API) not pulled forward. A-259 … A-270 for information.
- C-072 review (log addendum 1): you accepted **D-1** and **R-1** ("D-1 und R-1 reviewed und akzeptiert"). The 55 host scenarios are `reviewed`; SF-116 is decided as proposed and applied in addendum 2 (RIG 5.1 text, re-baseline, D8 messages).
- C-072 review addendum 2: D-1 applied:
  - RIG 5.1 in `7427a4f` (the minimum RAM and non-volatile memory are the nominal sizes of the installed memory, 1 GB = 10^9 octets), **SF-116 resolved** (`5a6f0b1`), no open finding.
  - Re-baseline on `7427a4f`: 1410 inventory items (2 remapped), 444 REQs, 0 gaps; schema `2.0.1-dev` @ `7427a4f` (no content change).
  - REQ-PLT-001 follows the text → `draft`.
  - D8 T8-HST-04: same thresholds and verdicts, the two manual memory findings cite "(RIG 5.1)" and the nominal size of the installed memory instead of "(RIG 5.1, SF-116)"; the notes of the scenarios `T8-HST-04/ram-reserved` and `/storage-nominal` follow → `draft`.
  - Together → **R-2** (numbered on because R-1 is answered).
- C-072 review addendum 3: you accepted **R-2** ("R-2 akzeptiert"); REQ-PLT-001 and the scenarios `T8-HST-04/ram-reserved`, `/storage-nominal` are `reviewed` – all 55 host scenarios `reviewed`. No draft, no open finding. Nothing is open.

Coverage stays at 0 gaps; schema `2.0.1-dev` @ `7427a4f` (re-baseline, no content change).

Logs: [C-072](log/2026-10-30_C-072_d8-host.md), [C-071](log/2026-10-29_C-071_d8-bus.md), [C-070](log/2026-10-29_C-070_p5b-plan-d8-image.md), [C-069](log/2026-10-29_C-069_d7-list-explain-report-schema-p5-exit.md), [C-068](log/2026-10-29_C-068_d7-peer-announce.md), [C-067](log/2026-10-29_C-067_d7-restart-publist.md), [C-066](log/2026-10-28_C-066_d7-proxy-lifecycle.md), [C-065](log/2026-10-28_C-065_d7-action-monitor.md), [C-064](log/2026-10-28_C-064_d7-set-tests.md), [C-063](log/2026-10-28_C-063_d7-get-tests.md), [C-062](log/2026-10-28_C-062_d7-test-engine.md), [C-061](log/2026-10-28_C-061_oec-ffi.md), [C-060](log/2026-10-28_C-060_new-data-set-writer-id.md), [C-059](log/2026-10-28_C-059_subscription-list.md), [C-058](log/2026-10-27_C-058_consumer-core.md), [C-057](log/2026-10-27_C-057_refdes-events.md), [C-056](log/2026-10-27_C-056_config.md), [C-055](log/2026-10-27_C-055_write-side.md), [C-054](log/2026-10-27_C-054_data-sets-metadata.md), [C-053](log/2026-10-27_C-053_provider-registry-get.md), [C-052](log/2026-10-26_C-052_d7-monitor.md), [C-051](log/2026-10-26_C-051_dev-environment-interop.md), [C-050](log/2026-10-26_C-050_runtime-tokio-rustls.md), [C-049](log/2026-10-26_C-049_mqtt-session-lifecycle.md), [C-048](log/2026-10-26_C-048_oec-crate-bootstrap.md), [C-047](log/2026-10-26_C-047_message-model.md), [C-046 incl. addenda 1–3](log/2026-10-26_C-046_p4-start.md).

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
| A-21 | Config definition and string-keyed getters without runtime; Pattern checked from increment 7 (done in C-056, A-106) | info (agent) | C-048 | – | – |
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
| A-87 | Cycle cut: 7 split into 7a (this cycle), 7b (Config) and 7c (ReferenceDesignation, Syslog, Generic events); SubscriptionList → 8; order 7a → 7b → 7c → 8 → 10 | info (agent) | C-055 | – | – |
| A-88 | Modules `status`, `provider::write`, `provider::overrides`; `Handled::{Publish, SetData}` with a two-phase `Set Data` | info (agent) | C-055 | – | – |
| A-89 | Requests and levels: `Set`/`Del` from Level 2 only; invalid payloads ignored; exactly one resource DataSetMessage | info (agent) | C-055 | – | – |
| A-90 | Status events: topic, envelope, ids, Description/SymbolicId; Event in the Profile, Status entries in the PublicationList | info (agent) | C-055 | – | – |
| A-91 | Unknown Source: `BadNotFound` under the requested Source with bounded transient ids – replaced by A-109 after SF-108 | info (agent) | C-055 | – | – |
| A-92 | Writes not served yet (Config, SubscriptionList, ReferenceDesignation, every `Del`) → `BadNotFound`; `Set Metadata` → `BadNotWritable` (Config served since C-056) | info (agent) | C-055 | – | – |
| A-93 | `Set PublicationList`: identification, checks and status codes, answers | info (agent) | C-055 | – | – |
| A-94 | Publication settings persisted under `app/publications`, applied only within the current `Writable` | info (agent) | C-055 | – | – |
| A-95 | `Set Data`: checks, the handler decides, unknown fields ignored, panic → `BadInternalError` | info (agent) | C-055 | – | – |
| A-96 | API (ADR-0020 amendment C-055): `on_set` returns `Result`, `SetData::fields`; sketch aligned | info (agent) | C-055 | – | – |
| A-97 | Tests (core, runtime, app, interop 11 green, D7 9/0/4); interop probes send schema-valid requests; no new dependency | info (agent) | C-055 | – | – |
| A-98 | Cycle cut: 7b not split; order 7b → 7c → 8 → 10 unchanged | info (agent) | C-056 | – | – |
| A-99 | Module `provider::config`; `Handled::SetConfig` with a two-phase `Set Config`, handler outside the lock | info (agent) | C-056 | – | – |
| A-100 | Config published masked at add/connect/change; `Get`/`Set Config` and Profile entry from Level 2; PublicationList entry `ON_REQUEST_FILTER_9` | info (agent) | C-056 | – | – |
| A-101 | `Set Config`: partial update, checks and status codes, empty `Value` unsets, unknown members ignored | info (agent) | C-056 | – | – |
| A-102 | Answer order Pub Config → Status Good (MG B1.1.3, informative, shows the reverse) | info (agent) | C-056 | – | – |
| A-103 | Masking: `********` in every publication, the mask in a `Set` keeps the value, no values in `Debug`/errors, the handler sees clear values | info (agent) | C-056 | – | – |
| A-104 | Persistence of set values under `app/config`, applied only while valid; RevisionCounter + 1 per change; `FileStore` 0700/0600 | info (agent) | C-056 | – | – |
| A-105 | Secrets: the means of authentication re-read before every reconnect, last good values on error; Config values not in `/run/secrets/oi4` | info (agent) | C-056 | – | – |
| A-106 | Pattern with Rust `regex` (Unicode, unanchored); new dependency `regex`, `cargo deny` without exception; A-21 resolved | info (agent) | C-056 | – | – |
| A-107 | API (ADR-0020 amendment C-056): `ConfigHandle::on_set` returns `Result`, `ConfigDef::{label, context}`, `ConfigHandle::{context, source, remove}`; sketch aligned | info (agent) | C-056 | – | – |
| A-108 | Tests (core, runtime, app, interop 12 green); D7 `monitor` 10/0/3 after a D7 fix of T7-MON-10 | info (agent) | C-056 | – | – |
| A-109 | SF-108 in the SDK: the Status event for an unknown Source under the `AppId`, transient ids dropped; interop race fixed | info (agent) | C-056 review | – | – |
| A-110 | Cycle cut: 7c not split; order ReferenceDesignation → Generic events → Syslog bridge; 7c → 8 → 10 | info (agent) | C-057 | – | – |
| A-111 | ReferenceDesignation from Level 2: `Get` with Query, `Set`/`Del` decided by the handler, identical `Set` without a write, `Del` of nothing → `BadNotFound`; not published on connect | info (agent) | C-057 | – | – |
| A-112 | ReferenceDesignation persisted under `app/reference-designations`; corrupt → `Error::Io`; kept while its asset is removed | info (agent) | C-057 | – | – |
| A-113 | A ReferenceDesignation change increments the RevisionCounter (MG 5, REQ-RES-MAM-018 – interpretation) | info (agent) | C-057 | – | – |
| A-114 | A request completed after its asset was removed is answered under the `AppId` (RD, Config, Data) | info (agent) | C-057 | – | – |
| A-115 | Generic events: ≤ 16 Levels per source, persisted writer ids and `FILTER_4` entries, Details a JSON object, dropped offline, allowed at Level 1 | info (agent) | C-057 | – | – |
| A-116 | Syslog events: PRI as Number, `MSG`/`HEADER`; Informational/Debug `OFF_0` by default; switchable by `Set PublicationList`, persisted | info (agent) | C-057 | – | – |
| A-117 | `log` bridge: installed at `build()`, SDK/transport targets excluded, reentrancy guard, rate limit 50 + 10/s, MSG ≤ 1024 bytes; never log secrets | info (agent) | C-057 | – | – |
| A-118 | API (ADR-0020 amendment C-057): `Result` returns, `GenericEvent` builder, `Asset::reference_designation`, `install_bridge`; sketch aligned | info (agent) | C-057 | – | – |
| A-119 | Tests (core, runtime, own Syslog binary, interop 13 green); D7 `monitor` 9/0/4; no new dependency | info (agent) | C-057 | – | – |
| A-120 | Cycle cut: increment 8 split into 8a (consumer core, C-058) and 8b (SubscriptionList, Action `NewDataSetWriterId`, sketch farewell, example app; C-059); 8a → 8b → 10 | info (agent) | C-058 | – | – |
| A-121 | Requests published only after the SUBACK of their answer filters; a refused filter ends the request and is not resubscribed while in use | info (agent) | C-058 | – | – |
| A-122 | Correlation: ReplyTo = MessageId on the responder's `Pub/` prefix only; requests without `MessageType` (as the guideline vectors) | info (agent) | C-058 | – | – |
| A-123 | Presence: Birth/Close/Will/heartbeat rules; an application first seen by its heartbeat is asked for its MAM | info (agent) | C-058 | – | – |
| A-124 | Nothing queued offline: `Error::NotConnected`; a lost connection ends pending requests | info (agent) | C-058 | – | – |
| A-125 | Writer ids of `Set`/`Del` resolved (Data/Config by `Get`, ReferenceDesignation by the PublicationList), otherwise explicit | info (agent) | C-058 | – | – |
| A-126 | API (ADR-0020 amendment C-058): option builders, `&Target`, typed `Received`, `Error::Status`/`NotConnected`, `get` needs an application; sketch not aligned | info (agent) | C-058 | – | – |
| A-127 | Tests (core 14 incl. vectors and proptest, runtime 4 with a routing test broker, interop 14 green); D7 `monitor` 11/0/2; no new dependency | info (agent) | C-058 | – | – |
| A-128 | Cycle cut: 8b (SubscriptionList, example app; C-059) and 8c (Action `NewDataSetWriterId`, sketch farewell; C-060); 8b → 8c → 10 | info (agent) | C-059 | – | – |
| A-129 | SubscriptionList entries: consumer filters (Interval = minimum, `CONF_1` only if all configurable), then the `NONE_0` `Set` entries (SF-107); one writer id; Level 2+; Profile of the application only; not in the PublicationList | info (agent) | C-059 | – | – |
| A-130 | Republication of the whole list on every change; after a connect only if not empty | info (agent) | C-059 | – | – |
| A-131 | `Set`/`Del SubscriptionList`: status codes; `Set` changes the Interval of a configurable entry; a TopicPath not subscribed → `BadNotSupported` (SF-111) | info (agent) | C-059 | – | – |
| A-132 | Consumer: Interval throttle per subscription and topic (no trailing delivery); `Del` unsubscribes the filter; settings only on filters whose subscriptions are all configurable | info (agent) | C-059 | – | – |
| A-133 | Persistence under `app/subscriptions`; corrupt → `Error::Io` at `build()` | info (agent) | C-059 | – | – |
| A-134 | API (ADR-0020 amendment C-059): `Subscription::interval`/`configurable` as in the sketch; writer id of `Set`/`Del SubscriptionList` resolved by a `Get` | info (agent) | C-059 | – | – |
| A-135 | Example app `oec/examples/demo.rs` (provider + consumer, Level 2, `OEC_ROOT`, `OEC_DEMO_SECONDS`) – the application under test of D7 `test` | info (agent) | C-059 | – | – |
| A-136 | Tests (core 7 + 3 + 1, runtime 2 + 1), answer-filter race of C-058 fixed (test first), interop 14 green, D7 `monitor` 12/0/1; no new dependency | info (agent) | C-059 | – | – |
| A-137 | Cycle cut: 8c (Action `NewDataSetWriterId` + farewell of the sketch) in one cycle; 8c → 10 | info (agent) | C-060 | – | – |
| A-138 | Responder: both Call filters from Level 2, one final response per ActionRequest at once, no response where REQ-ACT-005/006/017 say so, per-request Payload, `BadNotFound` for an unknown Source (SF-113), packed to the MaxPacketSize | info (agent) | C-060 | – | – |
| A-139 | Reservations: Ttl 60 s, ids above every id handed out, never reused (SF-112), cap 256 → `BadResourceUnavailable`, persisted in the writer table; a `Set` keeps using the listed id | info (agent) | C-060 | – | – |
| A-140 | Profile: `NewDataSetWriterId` from Level 2 for the application and every asset | info (agent) | C-060 | – | – |
| A-141 | Requester: CorrelationData = RequestId (8 bytes), unique RequestIds, TimeoutHint = remaining time, only the final response of the own request; Good without fields → `BadDecodingError` | info (agent) | C-060 | – | – |
| A-142 | API (ADR-0020 amendment C-060): `new_data_set_writer_id(resource, &Target, &ActionOptions) -> ProvidedWriterId`, default timeout 10 s | info (agent) | C-060 | – | – |
| A-143 | Farewell of the sketch: examples ported (no broadcast `Get`), `oec.h` + C examples in `oec-ffi/` (C Action with increment 10), walker `oec/tests/citations.rs`, gates adapted | info (agent) | C-060 | – | – |
| A-144 | Tests (core 10 + 3, runtime + 1), interop 14 green after the `Dev::root` fix, stress 25/25, D7 `monitor` 13/0 with T7-MON-07; no new dependency | info (agent) | C-060 | – | – |
| A-145 | Cycle cut: increment 10 in one cycle; the additive `oec` API first in its own commit | info (agent) | C-061 | – | – |
| A-146 | Additive `oec` API (ADR-0020 amendment C-061): `ConfigDef::from_json`, `ConfigValues::to_json`, `DataSet::update_json`, `SetData::to_json`, `AppHandle::emit_syslog` | info (agent) | C-061 | – | – |
| A-147 | Generated header: cbindgen 0.29.4 pinned, committed, diffed in the gate; constants as `int32_t` + `#define` instead of C enums | info (agent) | C-061 | – | – |
| A-148 | Signatures: `service_type` in subscriptions and requests (`OEC_ST_ANY`), `level` for Generic events, Precision in `fields_json` (no `oec_dataset_set_precision`) | info (agent) | C-061 | – | – |
| A-149 | Errors: last error only on failure, `OEC_ERR_NOT_CONNECTED`, `OEC_ERR_STATUS` + `oec_last_status_code`, panic → `OEC_ERR_PANIC` + app unusable | info (agent) | C-061 | – | – |
| A-150 | Threads and handles: callback thread, no blocking calls/`run`/`free` inside callbacks, child handles valid until the app is freed, `oec_app_start` errors not reported | info (agent) | C-061 | – | – |
| A-151 | Additions (`oec_set`, `oec_del`, `oec_new_data_set_writer_id`, …), what C does not get yet, packaging | info (agent) | C-061 | – | – |
| A-152 | Tests: C programs against the in-process broker, shared + static under ASan/UBSan, stress, LSan/cross builds on Linux once; ADR-0026 §20 | info (agent) | C-061 | – | – |
| A-153 | One subscription feeds the monitor engine (and recorder), then the runner; a test run is replayable | info (agent) | C-062 | – | – |
| A-154 | D7 requests with QoS 0 (REQ-MQTT-017); valid except the deliberate unknown key of T7-GET-09 | info (agent) | C-062 | – | – |
| A-155 | Tester `Utility/oi4.org/oec-check/D7/<12 hex>`, new per run, not announced, `--tester-il` | info (agent) | C-062 | – | – |
| A-156 | Discovery of the CUT (ServiceType given or all seven, fallback publication, not found → exit 4); CUT treated as an application | info (agent) | C-062 | – | – |
| A-157 | Waiting: answered + 250 ms; "no answer" waits `--timeout` (10 s); sequential tests | info (agent) | C-062 | – | – |
| A-158 | An active test judges only the REQs of its catalog entry | info (agent) | C-062 | – | – |
| A-159 | Passive REQs a test run cannot judge are plain skips ("test run:"); a level claim needs a `monitor` run too | info (agent) | C-062 | – | – |
| A-160 | Report version 1 with mode `test`, `tester`, `requests`; report schema in increment 7 | info (agent) | C-062 | – | – |
| A-161 | `--tests` with ids or globs; non-selected tests skip | info (agent) | C-062 | – | – |
| A-162 | `--app` checked as IL (dnp-encoded, hint in the error); no `--replay`/`--duration` for `test` | info (agent) | C-062 | – | – |
| A-163 | Probes of GET-05 (empty-list filter, unknown Source), GET-01 max. 20 Sources, level-1 skips; the P5 cycle cut | info (agent) | C-062 | – | – |
| A-164 | Bounded stimuli: max. 5 Filters per resource, 50 PublicationList entries, 20 subordinate Sources | info (agent) | C-063 | – | – |
| A-165 | Get Metadata only for the Data announced in the PublicationList; REQ-CMP-044 (MAY) not stimulated | info (agent) | C-063 | – | – |
| A-166 | Pagination by size judged passively (T7-MON-18); GET-06 stimulates pagination on request only | info (agent) | C-063 | – | – |
| A-167 | T7-ADR-02 listens to `Oi4/#` best effort, skip without `Oi4` traffic | info (agent) | C-063 | – | – |
| A-168 | "Ignored" = answered like the same Get without it; Query not judged without ReferenceDesignation objects; choice of the Parent | info (agent) | C-063 | – | – |
| A-169 | GET-08 accepts `de-DE`, `en-US` and the invariant locale | info (agent) | C-063 | – | – |
| A-170 | Monitor: answers to a Get with a Locale are not compared with the Birth (REQ-LIFE-003) | info (agent) | C-063 | – | – |
| A-171 | Monitor: the answer to a Get for one page is a series of its own (REQ-PAG-002/005); capture `pagination-broken` corrected (R-2) | info (agent) | C-063 | – | – |
| A-172 | Sim content: V-PAY-021/047/054, Config Filter without blanks, ua-metadata keys | info (agent) | C-063 | – | – |
| A-173 | Tester-violation contract: exactly the deliberate stimuli without a fault, a part of them with one | info (agent) | C-063 | – | – |
| A-174 | Fixture answer timeout 3 s, run context 2 min | info (agent) | C-063 | – | – |
| A-175 | Increment 2 not split; no SF raised (Config Filter vs. `Context.Name` checked) | info (agent) | C-063 | – | – |
| A-176 | Increment 3 not split; SET-10 stays in increment 6 | info (agent) | C-064 | – | – |
| A-177 | Restore model: Get without Source of Config, PublicationList, ReferenceDesignation, SubscriptionList before/after; Data and disk not compared | info (agent) | C-064 | – | – |
| A-178 | The restore result is reported (report key `restore`, text line, stderr warning), not judged | info (agent) | C-064 | – | – |
| A-179 | Writes are write-backs; Config with the mask; Data only where a Set subscription is listed | info (agent) | C-064 | – | – |
| A-180 | Bounded destructive requests: only own subscriptions deleted, ReferenceDesignation only with known state, `NONE_0` changes undone, Modes set back | info (agent) | C-064 | – | – |
| A-181 | "Nothing published" (REQ-BUS-007) = nothing until the Status event plus the grace time | info (agent) | C-064 | – | – |
| A-182 | SET-03 runs last and reuses the reactions of the other write tests | info (agent) | C-064 | – | – |
| A-183 | SET-01 judges REQ-RES-SUBLIST-016/CMP-016 after the catalog note | info (agent) | C-064 | – | – |
| A-184 | Stimuli: prefix `d7`/`D7`, RefDes DataSetWriterId from the PublicationList, one deliberately invalid Number | info (agent) | C-064 | – | – |
| A-185 | Status event recognition: category Status, `ReplyTo`, level from the severity bits, Source per SF-108 | info (agent) | C-064 | – | – |
| A-186 | Sim: write side, `Changed()`, one Status DataSetWriterId per Source and level, content additions | info (agent) | C-064 | – | – |
| A-187 | Fixture contract checks the restore (D7 against `sim.Changed()`) | info (agent) | C-064 | – | – |
| A-188 | No write-back of a masked sensitive Config value at level 1 (REQ-RES-CONFIG-025 only SHOULD) | info (agent) | C-064 | – | – |
| A-189 | A ReferenceDesignation is deleted only after its write-back Set got Status Good | info (agent) | C-064 | – | – |
| A-190 | A run that ends during the write tests reports the restore as not checked (warning) | info (agent) | C-064 | – | – |
| A-191 | Cut: T7-MON-14 moves to C-066 (increment 5) | info (agent) | C-065 | – | – |
| A-192 | Action stimuli; responses matched by `RequestorId` and `RequestId` on any Reply topic | info (agent) | C-065 | – | – |
| A-193 | The Action tests run inside the write window; provided ids are never used | info (agent) | C-065 | – | – |
| A-194 | `NewDataSetWriterId` must be answered if the Profile offers it or REQ-CMP-040 is MUST at the level | info (agent) | C-065 | – | – |
| A-195 | ACT-02 initial use stays manual; ACT-03 deliberate invalid requests and its reference call | info (agent) | C-065 | – | – |
| A-196 | MON-09: structure = sorted field names per DataSet | info (agent) | C-065 | – | – |
| A-197 | MON-13: without Profile and publication a plain skip | info (agent) | C-065 | – | – |
| A-198 | MON-15: republication criteria; Del SubscriptionList not judged | info (agent) | C-065 | – | – |
| A-199 | MON-16: growth not exactly +1; only value changes of non-sensitive Config objects; monotonicity only in `test` | info (agent) | C-065 | – | – |
| A-200 | MON-17 judges the requester against the merged Pub Configs; in `test` only the CUT | info (agent) | C-065 | – | – |
| A-201 | Fixture details (`retag`, sim progress, `answers-unknown`) and the Action calls in the report | info (agent) | C-065 | – | – |
| A-202 | Cut: T7-MON-14 moves again, to C-067 (increment 6) | info (agent) | C-066 | – | – |
| A-203 | Own MQTT relay on the Go standard library (no new dependency) | info (agent) | C-066 | – | – |
| A-204 | Proxy certificates from a CA the component trusts; client certificates are not relayed | info (agent) | C-066 | – | – |
| A-205 | D7 waits for the component's Birth and first SUBACK before its first Get | info (agent) | C-066 | – | – |
| A-206 | Connection phase last: TLS 1.1, untrusted CA, wrong host, drop, reconnection | info (agent) | C-066 | – | – |
| A-207 | CON-01: keep alive and clean session are notes; a changing ClientId is a violation | info (agent) | C-066 | – | – |
| A-208 | LIFE-01: "created anew" = MessageId time within the tolerance of the CONNECT, no repeat | info (agent) | C-066 | – | – |
| A-209 | LIFE-02: Birth first on every connection; recovery after FAILURE_1/0 | info (agent) | C-066 | – | – |
| A-210 | Without proxy: CON/LIFE inconclusive skips (exit 4) | info (agent) | C-066 | – | – |
| A-211 | Monitor: MON-03 late Birth, MON-10 recovery Birth | info (agent) | C-066 | – | – |
| A-212 | Sim: independent start, reconnect loop, Will QoS 0 per connection, `late-subscribe` | info (agent) | C-066 | – | – |
| A-213 | Harness: all fixtures through the proxy, `tests` selection, missing result is an error, `none` back to draft | info (agent) | C-066 | – | – |
| A-214 | Report: `attachment` "proxy", `proxy_listen`, `connections` | info (agent) | C-066 | – | – |
| A-215 | Cut: increment 6 split – peer, `--announce`, T7-ADR-01 to C-068; increments 7/8 to C-069 | info (agent) | C-067 | – | – |
| A-216 | Restart hook: a command given by the tester, run by the shell, bounded by `--restart-wait` | info (agent) | C-067 | – | – |
| A-217 | Restart phase in the write window, before the restore check | info (agent) | C-067 | – | – |
| A-218 | LIFE-04: which message is the Close message; inconclusive cases | info (agent) | C-067 | – | – |
| A-219 | SET-10: changed object, not applicable without ReferenceDesignation or with Status Bad | info (agent) | C-067 | – | – |
| A-220 | MON-14: time model, what it judges and what not | info (agent) | C-067 | – | – |
| A-221 | Fixtures: restart for every sim fixture, LIFE-02 selections narrowed, `lifecycle-pass` with B's PublicationList | info (agent) | C-067 | – | – |
| A-222 | Cut: increment 6b complete in C-068; C-069 unchanged | info (agent) | C-068 | – | – |
| A-223 | `--announce`: D7 as a Level-1 Utility under its tester IL, on its main connection | info (agent) | C-068 | – | – |
| A-224 | Peer: identity, connection, lifecycle and answers of the simulated second component | info (agent) | C-068 | – | – |
| A-225 | "Present" Health after the component under test is found | info (agent) | C-068 | – | – |
| A-226 | T7-ADR-01: which requests are judged, per-request checks, skips | info (agent) | C-068 | – | – |
| A-227 | Sim as a consumer, 3 faults, harness checks, `none` with peer and announce | info (agent) | C-068 | – | – |
| A-228 | Report: key `announced`, report version 1 | info (agent) | C-068 | – | – |
| A-229 | Cut: increments 7 and 8 in one cycle | info (agent) | C-069 | – | – |
| A-230 | Module root package `conformance` for the report schema and the fixture indexes | info (agent) | C-069 | – | – |
| A-231 | Report schema: scope, closed objects, validated in the gate | info (agent) | C-069 | – | – |
| A-232 | Request labels: test id or phase; the Profile read of the discovery labelled `discovery` | info (agent) | C-069 | – | – |
| A-233 | `list`/`explain`/`report-schema`: embedded data only, fixtures as stated, `version` extended | info (agent) | C-069 | – | – |
| A-234 | Coverage report `catalog/COVERAGE.md` in the conformance module, not in `spectool` | info (agent) | C-069 | – | – |
| A-235 | Test count corrected: 55 D7 tests | info (agent) | C-069 | – | – |
| A-236 | Exit criterion 2: negative fixtures of SHOULD/heuristic rules expect `warn`/`info` | info (agent) | C-069 | – | – |
| A-237 | Exit criterion 4 and distribution: local runs now, CI and container image with P9 | info (agent) | C-069 | – | – |
| A-238 | No new dependency, no schema change, SDK unchanged | info (agent) | C-069 | – | – |
| A-239 | Standard-library OCI and SBOM readers, no `go-containerregistry` | info (agent) | C-070 | – | – |
| A-240 | Input forms: OCI layout directory and oci-archive; no registry access | info (agent) | C-070 | – | – |
| A-241 | Reading rules: Docker schema 2, attestations, zstd inconclusive | info (agent) | C-070 | – | – |
| A-242 | Supported platforms linux/amd64, arm64(/v8), arm(/v7); fewer than two → warn | info (agent) | C-070 | – | – |
| A-243 | T8-IMG-04 warns only; a digest mismatch fails T8-IMG-01 | info (agent) | C-070 | – | – |
| A-244 | Own `infra-report.schema.json` (report version 1 of `oec-infra`), shared report helpers | info (agent) | C-070 | – | – |
| A-245 | SBOM rules as proposed in SF-114; notes for VOLUME, unlicensed packages, external DocumentRef | info (agent) | C-070 | – | – |
| A-246 | Image fixtures generated and byte-compared; coverage rule | info (agent) | C-070 | – | – |
| A-247 | P5b plan, cycle cut C-070 … C-075, precise exit | info (agent) | C-070 | – | – |
| A-248 | DESIGN §9 corrections; `explain`/coverage for D8 in increment 6 | info (agent) | C-070 | – | – |
| A-249 | SBOM rules after SF-114 (replace A-245): formats, components, texts, notes | info (agent) | C-070 review | – | – |
| A-250 | Command `oec-infra bus`: connection flags as `oec-check monitor`, no `--client-id`, own probe AppId | info (agent) | C-071 | – | – |
| A-251 | Second client `--credentials-2`/`--bus-storage-2`; without it T8-BUS-02 inconclusive | info (agent) | C-071 | – | – |
| A-252 | T8-BUS-01 probes: TLS versions with the client certificate, plain-text listener, certificate chain | info (agent) | C-071 | – | – |
| A-253 | MaxPacketSize probe (SF-105): one PUBLISH of MaxPacketSize on a 65535-octet topic | info (agent) | C-071 | – | – |
| A-254 | T8-BUS-02 ACL probes with a positive control; readable Set/Config warns | info (agent) | C-071 | – | – |
| A-255 | T8-BUS-03 Registry: discovery (65 s), two probe components, Close/Will; no Registry → inconclusive | info (agent) | C-071 | – | – |
| A-256 | Self-test: `mqtttest` options (additive), `registrytest` (7 faults), 29 broker scenarios | info (agent) | C-071 | – | – |
| A-257 | Report key `broker` (additive, report version 1 of `oec-infra`) | info (agent) | C-071 | – | – |
| A-258 | Dev broker ACL (incl. `Reply`); REQ-RES-CONFIG-026 warns against `oec/dev` | info (agent) | C-071 | – | – |
| A-259 | Command `oec-infra host`: `--root`, storage overrides, `--container`, read-only, no secret in the report | info (agent) | C-072 | – | – |
| A-260 | T8-HST-01: CA choice, `broker.json` schema and Address, expiry left to T8-BUS-01 | info (agent) | C-072 | – | – |
| A-261 | T8-HST-02: closed secret storage, key/passphrase/certificate rules, SHOULDs warn | info (agent) | C-072 | – | – |
| A-262 | Mounts only from the Linux mount table; no own mount fails only inside a container | info (agent) | C-072 | – | – |
| A-263 | Verdict kind `Reported` → outcome `manual` on a requirement, test unchanged | info (agent) | C-072 | – | – |
| A-264 | Hardware thresholds and manual bands (SF-116), network, facts per OS on the standard library | info (agent) | C-072 | – | – |
| A-265 | Linux check; patch level reported, not judged | info (agent) | C-072 | – | – |
| A-266 | Container daemon: endpoint order, connect only | info (agent) | C-072 | – | – |
| A-267 | Broker and Registry with the login of the storages; no Registry fails T8-HST-04 | info (agent) | C-072 | – | – |
| A-268 | Self-test `hosttest`, 55 scenarios, report key `host` | info (agent) | C-072 | – | – |
| A-269 | 32-bit build fix in `internal/probe` | info (agent) | C-072 | – | – |
| A-270 | Live runs on the computer and in a Linux container | info (agent) | C-072 | – | – |

## Decisions and review items

No open item.

### Answered earlier

Answered after the C-072 review (2026-10-31) and applied in C-072 addendum 3:
- R-2 – REQ-PLT-001 and the host scenarios `T8-HST-04/ram-reserved`, `/storage-nominal` after SF-116: ☑ accepted ("R-2 akzeptiert") → `reviewed`; all 55 host scenarios `reviewed`.

Answered in the C-072 review (2026-10-31) and applied in C-072 addenda 1–2:
- D-1 – SF-116 (RIG 5.1: "1 GB RAM / 4 GB non-volatile memory" = 10^9 octets, nominal size of the installed memory): ☑ as proposed ("D-1 … reviewed und akzeptiert") → RIG 5.1 text, SF-116 resolved, re-baseline; D8 T8-HST-04 keeps the manual bands, its messages cite RIG 5.1.
- R-1 – 55 host scenarios of `oec-infra host` (T8-HST-01, 02, 04, `oec/conformance/testdata/hosts/index.yaml`): ☑ accepted → `reviewed`.

Answered after the C-071 review (2026-10-30) and applied in C-071 addendum 4:
- R-2 – REQ-MQTT-007 and the broker scenarios `T8-BUS-01/wrong-host`, `/expired` after SF-115: ☑ accepted ("R-2 akzeptiert") → `reviewed`; all 29 broker scenarios `reviewed`.

Answered in the C-071 review (2026-10-29) and applied in C-071 addenda 2–3:
- D-1 – SF-115 (MG 6.2.2: broker certificate for the broker address and within its validity): ☑ as proposed ("D-1 wie vorgeschlagen") → guideline `b955783`, SF-115 resolved (`db6a477`), re-baseline, REQ-MQTT-007 and the scenarios `wrong-host`/`expired` → R-2; D8 findings cite MG 6.2.2 (ADR-0025 amendment C-071 review).

Answered after C-071 (2026-10-29) and applied in C-071 addendum 1:
- R-1 – 29 broker scenarios of `oec-infra bus` (T8-BUS-01 … 03, `oec/conformance/testdata/bus/index.yaml`): ☑ accepted → `reviewed`.

Answered after the C-070 review (2026-10-29) and applied in C-070 addendum 4:
- R-2 – REQ-CNT-013/014/026/027 after SF-114 and 18 image fixtures of `oec-infra image`: ☑ accepted → `reviewed`; all 19 image fixtures `reviewed`.

Answered in the C-070 review (2026-10-29) and applied in C-070 addenda 1–3:
- R-1 – 13 image fixtures of `oec-infra image` (T8-IMG-01 … 04): ☑ accepted → `reviewed` (addendum 1); 12 of them changed bytes with D-1 and are in R-2.
- D-1 – SF-114 (RIG 4.1.4: SBOM versions, naming of licenses, texts, CRA/RED): first answer changed the proposal (versions not restricted, licenses named license-conformant, "CRA und RED Ready", texts legally compliant) → revised proposal (`6d53556`); ☑ revised wording ("D-1 überarbeitet ok, einarbeiten") → guideline `a55f475` (RIG 4.1.4), SF-114 resolved (`3a86b5e`), re-baseline, REQ-CNT-013/014/026/027 and the image fixtures → R-2; D8 rules A-249.

Answered in the C-069 review (2026-10-29) and applied in C-069 addendum 1:
- D-1 – P5 exit and the next phase: ☑ (a) → P5 closed; criteria 1–3 met, criterion 4 (CI on Windows/macOS/Linux runners) and the D7 container image carried into the P9 CI; next phase **P5b** (D8). The criteria and the evidence are in the C-069 log (P5 exit assessment). Caveat noted: the P5b exit on the demo and registry images follows with P6/P7.

Answered in the C-068 review (2026-10-29) and applied in C-068 addendum 1:
- R-6 – the changed sim fixture `none` (runs with peer and `--announce`, T7-ADR-01 pass): ☑ accepted → `reviewed`.
- R-7 – 3 sim fixtures of T7-ADR-01 (`own-app`, `service-type`, the conforming variant `silent`): ☑ accepted → `reviewed`.

Answered in the C-067 review (2026-10-29) and applied in the addenda of the C-065, C-066 and C-067 logs:
- R-1 of C-065 – 5 sim fixtures of `oec-check test` (`testdata/sim/index.yaml`: faults and the conforming variant of T7-ACT-01 … 04): ☑ accepted → `reviewed`.
- R-2 of C-065 – 6 capture fixtures of the monitor rules T7-MON-09/13/15/16/17 (`configuration-pass` and 5 negatives): ☑ accepted → `reviewed`.
- R-3 of C-066 – 16 sim fixtures of T7-CON-01 … 04 and T7-LIFE-01 … 03 incl. `none`: ☑ accepted → `reviewed`.
- R-4 of C-067 – 4 sim fixtures of T7-LIFE-04 and T7-SET-10 (incl. the conforming variant `T7-LIFE-04/abort`): ☑ accepted → `reviewed`.
- R-5 of C-067 – 6 capture fixtures of T7-MON-14 (`publist-pass` and 4 negatives, the changed `lifecycle-pass`): ☑ accepted → `reviewed`.

Answered in the C-064 review (2026-10-28) and applied in C-064 addendum 1:
- R-1 – 14 sim fixtures of `oec-check test` (`testdata/sim/index.yaml`: 13 new faults and conforming variants of T7-SET-01 … 09, 11, 12, `none` with the write outcomes): ☑ accepted → `reviewed`.

Answered in the C-063 review (2026-10-28) and applied in C-063 addendum 1:
- R-1 – 14 sim fixtures of `oec-check test` (`testdata/sim/index.yaml`: 11 new faults and conforming variants of T7-GET-03/04/06/07/08 and T7-ADR-02/03, 3 entries with added outcomes): ☑ accepted → `reviewed`.
- R-2 – capture fixture `pagination-broken` (stimulus corrected to `Page` 0, same verdict): ☑ accepted → `reviewed`.

Answered in the C-062 review (2026-10-28) and applied in C-062 addendum 1:
- R-1 – 8 sim fixtures of `oec-check test` (`testdata/sim/index.yaml`: the fault-free run and 7 faults of T7-GET-01/02/05/09): ☑ accepted → `reviewed`.

Answered in the C-061 review (2026-10-28) and applied in C-061 addendum 1:
- D-1 – P4 exit and the next phase: ☑ (a) → P4 closed; criterion 3 (D7 `test` against `oec/sdk/rust/oec/examples/demo.rs`) carried into the P5 exit, criterion 5 (platform matrix: Windows MSVC, native amd64/Windows runs) into the P9 CI; next phase **P5**. The criteria and the evidence are in the C-061 log (P4 exit table).

Answered after the C-060 review (2026-10-28) and applied in C-060 addendum 2:
- R-1 – REQ-ACT-017, -023 and the new REQ-ACT-024 after SF-112/SF-113: ☑ accepted → `reviewed`.

Answered in the C-060 review (2026-10-28) and applied in C-060 addendum 1:
- D-1 – SF-112 (what a provided `DataSetWriterId` is for; reuse): ☑ as proposed → guideline `eb7889c` (MG 8.4.1), SF-112 resolved (`4fac7d2`), re-baseline, REQ-ACT-023 → R-1.
- D-2 – SF-113 (`NewDataSetWriterId` for an unknown Source): ☑ as proposed (`BadNotFound`, empty Payload) → guideline `eb7889c` (MG 9.2.1), SF-113 resolved, re-baseline, new REQ-ACT-024 and REQ-ACT-017 → R-1.
- R-1 of the C-059 review – REQ-RES-SUBLIST-004, -010, -016 after SF-109 … SF-111: ☑ accepted → `reviewed`.

Answered in the C-059 review (2026-10-28) and applied in C-059 addendum 1:
- D-1 – SF-109 (transient answer subscriptions of a requester): ☑ as proposed (not listed) → guideline `57ebe91` (MG 9.1.9), SF-109 resolved (`7a2bda2`), re-baseline, REQ-RES-SUBLIST-016 → R-1. Your note on the purpose (service A follows only selected values of OT service B, e.g. 1 of 100 devices) is recorded in the log.
- D-2 – SF-110 (own `AppId` vs. B1.1.9): ☑ as proposed (only request subscriptions to the own `AppId` excluded, B1.1.9 stays) → guideline `57ebe91`, SF-110 resolved, REQ-RES-SUBLIST-016 → R-1.
- D-3 – SF-111 (creation of entries via `Set`): ☑ as proposed (optional, otherwise `Bad`, e.g. `BadNotSupported`) → guideline `57ebe91`, SF-111 resolved, REQ-RES-SUBLIST-004/010 → R-1.

Answered after the C-056 review (2026-10-27) and applied in C-056 addendum 2:
- R-1 – REQ-CMP-043, REQ-CMP-047 (MG 9.1 after SF-108) and the new REQ-RES-SUBLIST-016 (MG 9.1.9 after SF-107): ☑ accepted → `reviewed` (`ee9f0e9`).

Answered in the C-056 review (2026-10-27) and applied in C-056 addendum 1:
- D-1 – SF-107 (how a requester finds out whether a DataSet is writable): ☑ as proposed → guideline `580f1c6` (MG 9.1.9, B1.1.3/B1.1.4), SF-107 resolved (`d2dc9ca`), re-baseline, new REQ-RES-SUBLIST-016 → R-1.
- D-2 – SF-108 (`Source` of the Status event for a request to an unknown asset): ☑ as proposed → guideline `2e32a6b` (MG 9.1), SF-108 resolved (`d2dc9ca`), re-baseline, REQ-CMP-043/047 → R-1; SDK changed (A-109).

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

### A-87 … A-97 – P4 increment 7a (C-055, agent)

- **Where:** `oec/sdk/rust/oec-core/src/{status.rs, provider.rs}`, `provider/{write.rs, overrides.rs, writers.rs, sets.rs}`, `oec/sdk/rust/oec/src/{data.rs, app.rs, runtime.rs, status.rs}`, the tests, the sketch (`sketch/src/data.rs`, `examples/device_source.rs`); ADR-0020 amendment C-055, ADR-0026 amendment C-055 §14, ADR-0023 amendment C-055; [C-055 log](log/2026-10-27_C-055_write-side.md).
- **A-87 Cycle cut:** increment 7 is split. 7a (this cycle: the request path, Status events, `BadNotFound` for unknown Sources, `Set PublicationList`, `Set Data`) is one mechanism – check a request, answer with a Status event, correlate with `ReplyTo` – and already about 1500 lines of code and 1330 lines of tests. 7b (Config with masking and secrets) is its own security-relevant topic on the unchanged write path; 7c (ReferenceDesignation, Syslog bridge, Generic events) are small additions on the Event writer of 7a. The SubscriptionList moves to 8, because it lists the subscriptions of the consumer. Order 7a → 7b → 7c → 8 → 10.
- **A-88 Modules:** `oec_core::status` (status codes of MG 8.3.6.1 with level, Description and SymbolicId), `provider::write` (request parsing, Status events, `Set PublicationList`, `Set Data`), `provider::overrides` (stored publication settings). `Provider::handle_message` returns `Handled::Publish` or `Handled::SetData`; for the latter the runtime asks the handler and calls `complete_set_data`, so the sans-IO core never runs user code.
- **A-89 Requests and levels:** from Level 2 the provider subscribes `Set/<R>/#` for Config, Data, Metadata, PublicationList, SubscriptionList and ReferenceDesignation and `Del/<R>/#` for SubscriptionList and ReferenceDesignation (14 request filters with the `Get` filters). Level 1 subscribes no request topic and offers nothing writable (REQ-CMP-043 applies to an entity that offers `Set`/`Del`); `on_set` returns an error there. A payload that is not JSON or has no valid MessageId cannot be answered (`ReplyTo`) and is ignored. A request carries exactly one resource DataSetMessage (helper DataSets 1 … 9 aside): none or several → `BadInvalidArgument`, not an object or no `Messages` → `BadDecodingError`.
- **A-90 Status events:** `ua-data` on `Pub/Event/<Source>/Status/<Level>` with `ReplyTo`, DataSetWriterName = Source, WriterGroupName `Status/<Level>`; the payload carries the `Number` and, for known codes, Description and SymbolicId. Each own source gets persisted DataSetWriterIds for its three Status levels; the PublicationList lists them (`FILTER_4`, `Config` `NONE_0`) and the Profile lists `Event`.
- **A-91 Unknown Source:** a `Get`, `Set` or `Del` to a Source the application neither is nor represents is answered with `BadNotFound` on `Pub/Event/<requested Source>/Status/Bad` (literal reading of MG 9.1; REQ-CMP-047). These events get transient DataSetWriterIds from 65535 downwards, at most 64 foreign Sources per run (beyond that no answer), not persisted and not in the PublicationList. **SF-108 (D-2)** proposes the `AppId` instead; if decided so, this changes in the next cycle.
- **A-92 Writes not served yet:** `Set Config`, `Set SubscriptionList`, `Set ReferenceDesignation` and every `Del` → `BadNotFound` until 7b/7c/8 (the object is not there); `Set Metadata` → `BadNotWritable` for an announced DataSet (Metadata follows the definition), otherwise `BadNotFound`.
- **A-93 `Set PublicationList`:** the entry is identified by Resource, Source and Filter (REQ-RES-PUBLIST-003); `Mode` is normalized before comparison. Unknown entry → `BadNotFound`; not decodable → `BadDecodingError`; a change the entry's `Config` forbids, or a change of a read-only property → `BadNotWritable`; an invalid Precision (unknown or non-numeric field, not a number ≥ 0) → `BadInvalidArgument`; unchanged → `Good` with the entry echoed. On success the changed entry is published on `Pub/PublicationList/<Source>/<Filter>` with `ReplyTo`, then the Status `Good`; the full list again only on an on/off toggle (then also the Metadata); the publication state restarts (next due at once).
- **A-94 Persistence:** the applied settings are stored under `app/publications` (`{"Version":1,"Entries":[…]}`), next to the writer ids and Metadata versions; a corrupt table → `Error::Io` (`InvalidData`) at `build()`. At `add_data` a stored setting is applied only if it is still allowed by the current `Writable` and valid for the definition, so a changed program never inherits a forbidden setting.
- **A-95 `Set Data`:** unknown Tag or `OFF_0` → `BadNotFound`; no handler → `BadNotWritable`; payload not an object → `BadDecodingError`; a field missing → `BadInvalidArgument`; a wrong type → `BadTypeMismatch`; unknown fields are ignored (tolerant reader, REQ-MSG-046). The handler gets `SetData` and decides: `Ok` → the new state is published as Pub Data with `ReplyTo`, then Status `Good`; `Err(code)` → only the Status event with that code (an `Err` with a Good code is sent as `Bad`, an Uncertain code as it is); a panic → `BadInternalError`. The handler runs on the runtime task outside the registry lock (a blocking handler delays the bus loop, documented).
- **A-96 API (ADR-0020 amendment C-055):** `DataSet::on_set` returns `Result<()>` (refused for a removed DataSet and at Level 1; the sketch returned `()`); `SetData::{get, fields}`; `Writable` documents the applied `Set PublicationList`. The sketch is aligned.
- **A-97 Tests and dependencies:**
  - `oec-core`: 12 write tests (`tests/provider_write.rs`, incl. the guideline vectors V-PAY-061/062 of MG B1.1.4 and a strict-schema test of every answer), unit tests in `status`, `overrides` and `writers`;
  - `oec`: runtime tests `set_publication_list_is_answered_and_persisted` and `set_data_is_decided_by_the_handler` against the in-process TLS broker, app test `set_handlers_and_publication_settings`;
  - opt-in interop `level_2_set_data_and_publication_list` against Mosquitto: `interop.sh` 11 passed, 1 ignored;
  - one-off D7 `oec-check monitor` (external call, Q-009): the first run found 5 schema-invalid requests of the test probe itself (fixed: interop probes send schema-valid requests); rerun 9 pass, 0 fail, 4 skip (T7-MON-03 connect not seen, 05 needs ≥ 125 s, 07 no RequestIds, 04 because of SF-108); capture not committed;
  - no new dependency; `Cargo.lock` and `deny.toml` unchanged, `cargo deny` without exception.

### A-98 … A-108 – P4 increment 7b (C-056, agent)

- **Where:** `oec/sdk/rust/oec-core/src/provider/config.rs`, `provider.rs`, `provider/write.rs`, `mqtt/session.rs`; `oec/sdk/rust/oec/src/{config.rs, asset.rs, app.rs, runtime.rs, store.rs, container.rs}`; the tests; the sketch (`sketch/src/config.rs`, `examples/config.rs`); the D7 monitor (`oec/conformance/internal/monitor`); ADR-0020 amendment C-056, ADR-0026 amendment C-056 §15, ADR-0023 amendment C-056; [C-056 log](log/2026-10-27_C-056_config.md).
- **A-98 Cycle cut:** 7b is not split. Config, its masking and the secret reload use one mechanism (the write path of 7a) and one persistence pattern (the tables of 6a/7a); splitting off the masking would leave a Config that leaks sensitive values. About 2000 lines of code and 1450 lines of tests. Order 7b → 7c → 8 → 10 unchanged.
- **A-99 Module and phases:** `oec_core::provider::config` holds the definition and value checks, the masking, the `Set Config` check and the table of set values. A valid `Set Config` returns `Handled::SetConfig`; the runtime asks the `on_set` handler outside every lock and calls `complete_set_config` (as `Set Data` in 7a).
- **A-100 Publication and levels:** every Config DataSet is published on `Pub/Config/<Source>/<Filter>` (Filter = context name) when added, at every connect and after every change, always masked. At Level 1 it is published, but `Get`/`Set Config` are not served and `on_set` is refused. From Level 2 `Get Config` (selection by Source and Filter, paginated) and `Set Config` are served and the Profile lists `Config`. The PublicationList gets one entry per Config (`ON_REQUEST_FILTER_9`, Filter = context, `Config` `NONE_0`, its own DataSetWriterId).
- **A-101 `Set Config`:** a partial update (at least one object; the mandatory objects of every group sent must be present); read-only properties may be repeated unchanged. The first violation in the order of the JSON members is answered: unknown context, group or object → `BadNotFound`; no or several DataSetMessages, `Value` missing, mandatory object missing → `BadInvalidArgument`; not a JSON object → `BadDecodingError`; `Value` no string, other `Type`, `Value` not of its Type → `BadTypeMismatch`; another read-only property changed → `BadNotWritable`; outside the Validation or the Config beyond the MaxPacketSize → `BadOutOfRange`. An empty `Value` unsets the object (MG 9.1.3); unknown members are ignored (REQ-MSG-046); `Del Config` is not subscribed (MG has no `Del` for Config). Then the handler decides (`Ok`, or `Err(code)` → Status with that code; a panic → `BadInternalError`).
- **A-102 Answer order:** after an accepted `Set` the masked Config with `ReplyTo`, then the Status event Good, as for `Set Data`. The informative MG B1.1.3 shows the Status first; REQ-RES-CONFIG-022 orders nothing, so this is no finding.
- **A-103 Masking (GL-ADR-021):** every publication replaces a non-empty Value/DefaultValue of a sensitive object with `********` (an empty one stays empty). The mask in a `Set` keeps the value and is no change; `ConfigHandle::set` refuses the mask as a value. `Debug` output and error messages of the SDK never contain values. The `on_set` handler sees the proposed values in clear (the application needs them).
- **A-104 Persistence:** only values that were set (by `Set Config` or `ConfigHandle::set`) are stored, under `app/config` (`{"Version":1,"Entries":[…]}`); a corrupt table → `Error::Io` (`InvalidData`) at `build()`. At `add_config` a stored value is used only if its object still exists and the value still passes the checks. Every effective change increments and persists the RevisionCounter of the source and republishes its MAM (REQ-RES-MAM-018; a counter from the VendorNameplate is not changed). The `FileStore` creates directories 0700 and files 0600 on Unix: sensitive values are stored in clear, protected by the file modes (the guideline asks for no encryption at rest; a platform may encrypt the data volume).
- **A-105 Secrets (GL-ADR-022):** with a container storage the runtime reads `/run/secrets/oi4` (credentials, private key, passphrase) and the client certificate again before every reconnect, because the platform may renew them while the application runs. Changed → used for the next CONNECT and TLS; unreadable or refused → the last good values stay, with a warning that names no value. An explicit `Broker` is not re-read. Sensitive Config values are not written to `/run/secrets/oi4`: GL-ADR-022 reserves it for the platform's means of authentication.
- **A-106 Pattern:** checked with the Rust crate `regex` (Unicode classes, unanchored match, linear time); a pattern that does not compile is refused when the Config is added. D7 uses Go RE2 (`\d` there means ASCII digits only); MG 8.3.3.1 itself says regex dialects differ, so this is no finding. `regex` (+ regex-automata, regex-syntax, memchr; MIT/Apache-2.0, memchr Unlicense/MIT) is a new runtime dependency of `oec-core`, already in `Cargo.lock` through jsonschema; `cargo deny` without exception. A-21 is resolved.
- **A-107 API (ADR-0020 amendment C-056):** `ConfigHandle::on_set` returns `Result<()>` (refused at Level 1 and for a removed Config; the sketch returned `()`), additions `ConfigDef::{label, context}` and `ConfigHandle::{context, source, remove}`; the prelude is unchanged; the sketch is aligned.
- **A-108 Tests:**
  - `oec-core`: 16 Config tests (`tests/provider_config.rs`, incl. the guideline vectors V-PAY-098 … 102/114 of MG B1.1.3, every status code and a strict-schema test of every answer), 8 unit tests in `provider::config`, `Session::set_credentials`;
  - `oec`: `config_is_got_set_and_persisted` and `renewed_secrets_apply_on_reconnect` against the in-process TLS broker, app tests `config_is_declared_set_and_persisted` and `invalid_configs_and_set_handlers`, `file_store_is_owner_only`;
  - opt-in interop `level_2_config` against Mosquitto: `interop.sh` 12 passed, 1 ignored;
  - one-off D7 `oec-check monitor` (external call, Q-009): run 1 9 pass, 1 info, 3 skip – T7-MON-10 took the MAM republished after the Config change for a second Birth; fixed in D7 with a test first (`TestMAMOnChange`), the reviewed fixtures unchanged; run 2 10 pass, 0 fail, 3 skip (T7-MON-03 the probe has no Birth, 05 needs ≥ 125 s, 07 no RequestIds); capture not committed.

### A-109 – SF-108 in the SDK (C-056 review, agent)

- **Where:** `oec/sdk/rust/oec-core/src/provider.rs`, `provider/write.rs`, `provider/writers.rs`; the tests `oec-core/tests/{provider, provider_write}.rs`, `oec/tests/{runtime, interop}.rs`; ADR-0026 amendment C-056 review (replaces §14.6).
- **What:** a `Get`, `Set` or `Del` to a Source the application neither is nor represents is answered with `BadNotFound` on `Pub/Event/<AppId>/Status/Bad` (DataSetWriterName = AppId, the persisted Status id of the application, `ReplyTo`). The bounded transient ids (65535 downwards, `FOREIGN_SOURCES`) and `Ignored::UnknownSource` are gone: no publication carries a foreign Source, so there is nothing to bound. Test first: the changed tests failed before the change.
- **Interop:** in one full `interop.sh` run, `level_2_config` sent its first `Get` before the broker knew the application's SUBSCRIBE (`Online` follows CONNACK); the test now waits for the first `Pub/Config` (`6afd879`). A test defect, not an SDK defect. Then 12 passed, 1 ignored (twice).
- **D7:** one-off `oec-check monitor` (external call, Q-009) during `level_2_set_data_and_publication_list`: 10 pass, 0 fail, 3 skip (T7-MON-03, 05, 07 as before); T7-MON-04 passes although an unknown Source was asked (in C-055 it was skipped because of SF-108).

### A-110 … A-119 – P4 increment 7c (C-057, agent)

- **Where:** `oec/sdk/rust/oec-core/src/provider/{refdes.rs, events.rs}`, `provider.rs`, `provider/{write.rs, config.rs}`; `oec/sdk/rust/oec/src/{event.rs, asset.rs, app.rs, runtime.rs, lib.rs}`; the tests `oec-core/tests/{provider_refdes, provider_event, provider_write}.rs`, `oec/tests/{app, runtime, syslog, interop}.rs`, `oec/tests/testbroker/mod.rs`; the sketch (`sketch/src/{asset, event, app}.rs`); ADR-0020/0023 amendment C-057, ADR-0026 amendment C-057 §16; [C-057 log](log/2026-10-27_C-057_refdes-events.md).
- **A-110 Cycle cut:** 7c is not split. ReferenceDesignation, Generic and Syslog events are small additions on two existing mechanisms (the write path of 7a and the Event writer with persisted ids). Built in the order ReferenceDesignation → Generic events → Syslog bridge, because the bridge uses the event path of the Generic events. About 1700 lines of code and 1850 lines of tests (incl. the shared test broker). Order 7c → 8 → 10 unchanged.
- **A-111 ReferenceDesignation:**
  - offered from Level 2 (`AppBuilder::reference_designation(true)` at Level 1 → `Error::Invalid` at `build()`); the Profile lists it after Metadata; one PublicationList entry per source (`ON_REQUEST_SOURCE_10`, WriterGroupName `ReferenceDesignation`); not published on connect (it is on request only);
  - `Get` with or without Source, and with the Query of the direct children: the Parent IdentificationLink is compared character by character (REQ-RES-REFDES-014);
  - `Set` replaces the object as a whole, `Del` removes it; both in two phases: the SDK checks, the `on_reference_designation` handler decides outside every lock (a panic → `BadInternalError`), the SDK answers (Pub with `ReplyTo` + Status Good, or Status Bad with the handler's code);
  - a `Set` of the identical object is Good without a write; a `Del` without a stored object → `BadNotFound` without asking the handler; an invalid payload → `BadDecodingError`.
  - A-92 ends for ReferenceDesignation; `Set`/`Del SubscriptionList` stay `BadNotFound` until increment 8.
- **A-112 Persistence:** the table `{"Version":1,"Entries":[{"Source","ReferenceDesignation"}]}` under `app/reference-designations`; a table that cannot be read → `Error::Io` (`InvalidData`) at `build()`; a failing write is logged (the change stays in memory). An object stays stored while its asset is removed, so it is back when the asset is added again.
- **A-113 RevisionCounter:** an accepted ReferenceDesignation change increments and persists the RevisionCounter of the source and republishes its MAM, as a Config change does (A-104). MG 5 counts the "configuration data of the asset as defined in OPC UA Part 100"; a reference designation set over the bus is engineering data of the asset, so the SDK counts it. The guideline does not name it explicitly. No finding, because the reading follows the text. If you read it differently, I make it SF-109.
- **A-114 Late answers:** a `Set`/`Del` whose handler finishes after its asset was removed is answered under the `AppId` (as SF-108 does for unknown Sources). This holds for ReferenceDesignation, Config and Data.
- **A-115 Generic events:**
  - `Asset::emit_event(GenericEvent)`: category `CAT_GENERIC_99`, free Level (a valid topic level, at most 16 per source), Number, optional Description, Details as a JSON object;
  - every Level gets a persisted DataSetWriterId and a PublicationList entry (`FILTER_4`) at first use, even offline;
  - while offline, or with 256 events waiting, the event is dropped and the call still returns `Ok` (no queue for later: an event is about now);
  - allowed at every Level; `ua-event` in the DataSetMessage, the NetworkMessage stays `ua-data` (REQ-MSG-029).
- **A-116 Syslog events:** published on `Pub/Event/<AppId>/Syslog/<Level>`; Number = PRI (Facility 1 "user" × 8 + severity), no Description, Details `MSG` and `HEADER` (RFC 3164 header with HOSTNAME from the MAM and TAG = the application's name), MG B1.1.6. Eight PublicationList entries with Config `MODE_1`: Emergency … Notice on (`FILTER_4`), Informational and Debug `OFF_0` by default (REQ-TOP-024). `Set PublicationList` switches a Level between `FILTER_4` and `OFF_0` only (others → `BadNotWritable`, REQ-TOP-027); the result is persisted in `app/publications`.
- **A-117 `log` bridge:**
  - `AppBuilder::syslog(level)` installs one process-global `log::Log` at the first `build()` if no logger is set yet; `event::install_bridge(Some(local))` keeps a local logger as well. A later `env_logger::init()` panics (documented).
  - Level mapping: Error → Error, Warn → Warning, Info → Informational, Debug/Trace → Debug. `event::syslog` publishes the other Syslog levels directly.
  - No feedback loop: records of `oec`, `oec_core`, `rustls`, `tokio` and `mio` are not bridged, and a thread-local guard is held while a record is published. `log::max_level` follows the switched Levels.
  - Rate limit: a burst of 50, then 10 per second; the number dropped is reported once as a Warning. MSG is `TAG[pid]: text`, cut to 1024 bytes on a character boundary.
  - Secrets: the SDK cannot tell a secret in a log text. The docs say: never log secrets (GL-ADR-021); the SDK's own records are not bridged.
- **A-118 API (ADR-0020 amendment C-057):** `Asset::on_reference_designation` and `Asset::emit_event` return `Result`; new `Asset::reference_designation`, `AppBuilder::reference_designation`, `GenericEvent::new(level, number).description(..).details_json(..)`, `event::{syslog, install_bridge, MAX_MESSAGE_BYTES}`; the prelude has `ReferenceDesignation` and `GenericEvent`. The sketch is aligned.
- **A-119 Tests:**
  - `oec-core`: `tests/provider_refdes.rs` (5 tests, incl. the guideline vectors V-PAY-041 … 046/053/054 and a strict-schema check of every answer), `tests/provider_event.rs` (5 tests, the shapes of V-PAY-064/065), `provider_write.rs` (late `Set Data` answer);
  - `oec`: `reference_designation_is_set_got_deleted_and_persisted` and `generic_events_are_published_while_connected` against the in-process TLS broker, now in `tests/testbroker/mod.rs`; `tests/syslog.rs` as its own binary (the logger is process-global); unit tests in `event`;
  - opt-in interop `level_2_reference_designation_and_generic_events` against Mosquitto: `interop.sh` 13 passed, 1 ignored (the Syslog bridge is not in the interop binary, because its logger is process-global);
  - one-off D7 `oec-check monitor` (external call, Q-009): 9 pass, 0 fail, 4 skip (T7-MON-03 connected before observation, 05 needs ≥ 125 s, 07 no RequestIds, 18 MaxPacketSize not given); capture not committed;
  - no new dependency; `Cargo.lock` and `deny.toml` unchanged, `cargo deny` without exception.

### A-120 … A-127 – P4 increment 8a (C-058, agent)

- **Where:** `oec/sdk/rust/oec-core/src/consumer.rs`, `consumer/{received, request, directory}.rs`; `oec/sdk/rust/oec/src/{consumer.rs, app.rs, runtime.rs, lib.rs, status.rs}`; the tests `oec-core/tests/consumer.rs`, `oec/tests/{consumer, interop}.rs`, `oec/tests/testbroker/mod.rs`; ADR-0020/0023 amendment C-058, ADR-0026 amendment C-058 §17; [C-058 log](log/2026-10-27_C-058_consumer-core.md).
- **A-120 Cycle cut:** increment 8 holds three review topics: the consumer core (a new protocol role with its own state machine), the SubscriptionList (a provider resource that lists the consumer's subscriptions, so it needs the core) and the farewell of the sketch (possible only once every sketch item exists in `oec`). The core alone is about 1500 lines of code and 1430 lines of tests. So: 8a = the consumer core (C-058), 8b = SubscriptionList with `Subscription::configurable`/`interval`, the Action `NewDataSetWriterId`, the deletion of the sketch with its REQ walker moved into `oec`, the gate scripts and the example app (C-059; its agent may cut the Action out as 8c). Order 8a → 8b → 10.
- **A-121 SUBACK gating:** a request is published only after the broker acknowledged the subscriptions its answers arrive on (the responder's `Pub/<Resource>/#`; for `Set`/`Del` and Bad answers its `Pub/Event/+/Status/+`). Without it the first answer could be lost: there are no retained messages on the bus. A filter the broker refuses ends the waiting requests with `Error::Io(PermissionDenied)` and is not subscribed again while it is in use.
- **A-122 Correlation:** an answer counts only with ReplyTo = the MessageId of the request (REQ-MSG-015), on the responder's own `Pub/` prefix and after the request was published; anything else is not an answer. The DataSetMessages of requests carry no `MessageType`, exactly like the guideline request vectors (REQ-MSG-029, MG B1); the core tests compare the requests with V-PAY-002/007/020/046/053/055/061/100.
- **A-123 Presence:** Birth = a MAM of the application about itself without ReplyTo → online; Close = Health `NORMAL_0` with HealthScore 0 → offline (graceful); Will = Health `FAILURE_1` with HealthScore 0 → offline (ungraceful); any other Health of an application not yet known (the 60 s heartbeat, REQ-TOP-017) → online, and the SDK asks it once for its MAM. `Consumer::apps()` returns state, MAM and last Health per application.
- **A-124 Offline:** requests are not queued (as publications are not, A-65): a request while offline fails with `Error::NotConnected`; a lost connection ends the pending requests with it; after `shutdown` → `Error::Closed`. Subscriptions survive a reconnect (the session subscribes them again).
- **A-125 Writer ids:** a `Set`/`Del` carries one DataSetMessage with the DataSetWriterId of the addressed DataSet (REQ-BUS-010). The SDK finds it: Data/Config by a `Get` (exactly one answer – none → `BadNotFound`, several → `Error::Invalid`), ReferenceDesignation by the PublicationList entry of the Source (the object may not exist yet – found by a test); other list resources need `WriteOptions::data_set_writer_id`. A Metadata `Get` without id first asks the Data.
- **A-126 API (ADR-0020 amendment C-058):** `GetOptions`/`WriteOptions` are builders; requests take `&Target` and options by reference; `set` takes `&WriteOptions` instead of a timeout, `del` an optional payload (SF-098); `get` needs a concrete application (`Target::any()` is for `subscribe`; discovery is `on_presence`/`apps()`); `Received` has typed accessors (`payload()` → `Payload`) instead of public fields; `Presence` carries the ServiceType and a typed MAM; new `Consumer::apps`, `set_blocking`/`del_blocking`, `Error::Status`, `Error::NotConnected`, `StatusCode::from_code`/`is_good`. The `_blocking` forms refuse to run on a tokio runtime thread (they would deadlock). The sketch is not aligned because 8b deletes it.
- **A-127 Tests:**
  - `oec-core/tests/consumer.rs` (14 tests): the request vectors, the request matrix, correlation, `BadNotFound`, Status events, pagination incl. a proptest over the arrival order of pages, the Query check, SUBACK gating and timeouts, refusal and close, presence, deliveries; the answers come from the real `Provider` and are checked against the strict schema;
  - `oec/tests/consumer.rs` (4 tests) with two applications over the new routing mode of the in-process TLS test broker (forwards PUBLISH and the Will to matching subscriptions): `Get` incl. pages and Metadata, `Set Data`, ReferenceDesignation `Set`/Query/`Del`, presence incl. the Will, `NotConnected`/`Timeout`/`Invalid`/`Closed`; 10 runs in a row green, about 1 s each;
  - opt-in interop `consumer_asks_a_provider` against Mosquitto: `interop.sh` 14 passed, 1 ignored;
  - one-off D7 `oec-check monitor` (external call, Q-009): 11 pass, 0 fail, 2 skip (T7-MON-05 observation shorter than 125 s, T7-MON-07 no Action RequestIds), 0 invalid of 37 messages; capture not committed;
  - no new dependency; `Cargo.lock` and `deny.toml` unchanged, `cargo deny` without exception.

### A-128 … A-136 – P4 increment 8b (C-059, agent)

- **Where:** `oec/sdk/rust/oec-core/src/provider/sublist.rs`, `provider.rs`, `provider/{write, writers}.rs`, `consumer.rs`; `oec/sdk/rust/oec/src/{consumer.rs, app.rs, runtime.rs}`, `oec/examples/demo.rs`; the tests `oec-core/tests/{provider_sublist, consumer_sublist, consumer}.rs`, `oec/tests/{sublist, app, interop}.rs`; ADR-0020/0023 amendment C-059, ADR-0026 amendment C-059 §18; [C-059 log](log/2026-10-28_C-059_subscription-list.md).
- **A-128 Cycle cut:** 8b held two review topics of about the same size: the SubscriptionList (a provider resource on the write path that is fed by the consumer and feeds back into it) and the Action `NewDataSetWriterId` (a different message family: CompactEncoding `ActionNetworkMessage`, `Call`/`Reply`, `CorrelationData`, `TimeoutHint`, the reservation of ids in the writer table). The sketch can only go after the Action. So: 8b = SubscriptionList + example app (C-059), 8c = Action + farewell of the sketch (C-060). Order 8b → 8c → 10.
- **A-129 Entries:** first one entry per topic filter of the lasting consumer subscriptions (sorted; Interval = the smallest of the subscriptions on the filter, omitted when 0; `CONF_1` only if every subscription on the filter is configurable), then one `NONE_0` entry per writable Config context and per Data DataSet that accepts `Set`, with its `Set` topic (SF-107). All entries share one DataSetWriterId (MG 9.1.9 note), DataSetWriterName = the AppId. The list exists from Level 2 (REQ-CMP-039): the Profile of the application lists it, those of the assets do not; the PublicationList does not list it. A `Get` with the Source of an asset gets an empty list. Answer subscriptions of requests are not listed (SF-109); a consumer subscription to a topic of the own AppId is (SF-110).
- **A-130 Republication:** the whole list is published unsolicited whenever it differs from the last published one (REQ-RES-SUBLIST-005); after a connect only if not empty; a list that became empty is published once as empty.
- **A-131 `Set`/`Del`:** no ListFilter or a Source other than the AppId → `BadNotFound`; undecodable → `BadDecodingError`; TopicPath outside the addressed group → `BadInvalidArgument`; `NONE_0` entry or a changed `Config` → `BadNotWritable`; **a TopicPath the application has not subscribed → `BadNotSupported`** (the SDK does not create subscriptions from outside; SF-111). `Set` stores the Interval (absent = 0) and publishes the entry with the `ReplyTo`; `Del` (with the entry as payload, V-PAY-074) unsubscribes the filter and publishes the remaining entries of the group; then Status Good. A deleted entry can be restored by a `Set` with its TopicPath.
- **A-132 Consumer:** an Interval drops a publication of a topic that arrives within the Interval after the last delivered one of the same topic for the same subscription (no delivery of the last value at the end of the Interval; REQ-RES-SUBLIST-003 only says "processing interval"). A deleted filter is unsubscribed and delivers nothing, but stays subscribed while a pending request waits for its answer on it. `Set`/`Del` apply only to filters whose subscriptions are all configurable. Presence subscriptions are `NONE_0`.
- **A-133 Persistence:** the changes by `Set`/`Del` are stored under `app/subscriptions` (only the changed entries) and survive a restart; a corrupt table makes `build()` fail with `Error::Io(InvalidData)`, as for the other tables.
- **A-134 API:** `Subscription::interval(Duration)` and `Subscription::configurable()` with the sketch signatures (no own `#[must_use]`: the type has it). `Consumer::set`/`del` of `Resource::SubscriptionList` find the DataSetWriterId by a `Get` of the addressed entries; restoring a deleted entry needs `WriteOptions::data_set_writer_id`. The sketch stays the contract only for `Consumer::new_data_set_writer_id`.
- **A-135 Example app** `oec/sdk/rust/oec/examples/demo.rs`: one Level 2 OTConnector that is provider (pump asset with a writable `oee` DataSet and a range check, Config `service`/`Period`, ReferenceDesignation) and consumer (presence, configurable `oee` subscription with 1 s Interval); container storage under `OEC_ROOT` (default `/`), `OEC_DEMO_SECONDS` for a graceful end; built by the gate (`clippy --all-targets`). It is the application under test of D7 `test`.
- **A-136 Tests:**
  - `oec-core/tests/provider_sublist.rs` (7): entries, filter, Profile, republication, the guideline vectors V-PAY-072 … 075 answered and schema-checked, every rejection, persistence; `consumer_sublist.rs` (3); `consumer.rs` (+1, see below);
  - `oec/tests/sublist.rs` (2) with two applications over the routing test broker; `oec/tests/app.rs` (+1, corrupt table);
  - **defect of C-058 fixed (test first):** a released answer filter lost its SUBACK state at once; a request that reused it before the UNSUBSCRIBE waited for a SUBACK that never came and timed out (4 of 20 runs under load). Now the state is kept until the session has unsubscribed the filter; 25 runs in a row green under load;
  - opt-in interop `consumer_asks_a_provider` with `Get`/`Set`/`Del SubscriptionList` against Mosquitto: `interop.sh` 14 passed, 1 ignored;
  - one-off D7 `oec-check monitor` (external call, Q-009; 140 s against the demo app and the interop consumer): 12 pass, 0 fail, 1 skip (T7-MON-07 no Action RequestIds), 0 invalid of 95 messages; capture not committed;
  - no new dependency; `Cargo.lock` and `deny.toml` unchanged, `cargo deny` without exception.

### A-137 … A-144 – P4 increment 8c (C-060, agent)

- **Where:** `oec/sdk/rust/oec-core/src/provider/{action, writers}.rs`, `provider.rs`, `status.rs`, `consumer.rs`; `oec/sdk/rust/oec/src/consumer.rs`, `oec/examples/`, `oec/tests/{consumer, interop, citations}.rs`; `oec/sdk/rust/oec-ffi/`; the tests `oec-core/tests/{provider_action, consumer}.rs`; ADR-0020/0021/0023 amendments C-060, ADR-0026 §19; [C-060 log](log/2026-10-28_C-060_new-data-set-writer-id.md).
- **A-137 Cycle cut:** the typed Action model exists since C-047, the persisted writer table since C-053, the request machinery since C-058; new are one responder path, one reservation table and one request kind – one review topic. The farewell of the sketch is mechanical. Not split; 8c → 10.
- **A-138 Responder:** from Level 2 the application subscribes `Call/NewDataSetWriterId` of its AppId with and without Source and answers every `ActionRequest` at once with one final response (no intermediate responses, so the `TimeoutHint` is always kept). No response if the request cannot be decoded or addressed, its PublisherId is not the application or its ResponseAddress is not the matching Reply topic (REQ-ACT-005/006/017). Each request's Payload is decoded on its own, so an invalid `Resource` fails only that request (`BadInvalidArgument`). Unknown Source → `BadNotFound` (SF-113 proposal). Several responses share one message up to the MaxPacketSize.
- **A-139 Reservations:** the provided id is higher than every id handed out so far (after 65535 the lowest free one), reserved for 60 s, persisted in the writer table (`app/writer-ids`) so that a restart does not hand it out again, and never assigned or provided again – stricter than "only within the `Ttl`" (SF-112 (3)). At most 256 open reservations; then `BadResourceUnavailable`. The SDK does not bind a provided id to a new DataSet: it lists an id for everything it accepts a `Set` for, and a `Set` keeps using that id (SF-112 (1)/(2)).
- **A-140 Profile:** `NewDataSetWriterId` is listed from Level 2 for the application and every asset (it answers for each Source it is or represents); REQ-CMP-040: L2 SHOULD, L3 MUST.
- **A-141 Requester:** CorrelationData = the consumer's RequestId as 8 bytes; RequestId unique among the pending actions; TimeoutHint = the remaining time (≥ 1 ms); only the final response of the addressed Responder with the own RequestId and CorrelationData counts; a Good response without `DataSetWriterId`/`Ttl` → `BadDecodingError`.
- **A-142 API:** `Consumer::new_data_set_writer_id(resource, &Target, &ActionOptions) -> Result<ProvidedWriterId>` instead of the sketch's `(service_type, app, resource, timeout) -> (u16, Duration)` – the same `Target` as for `get`; ServiceType and AppId required, no Filter (`Error::Invalid`); Bad → `Error::Status`; no response → `Error::Timeout`; default timeout 10 s.
- **A-143 Farewell of the sketch:** examples `hello` (16 lines), `config`, `device_source`, `consumer` ported to `oec/examples/`. The sketch's consumer example sent a broadcast `Get` to `Target::any()`; since C-058 `get` needs a concrete application, so the ported example targets the OTConnector named on the command line. `oec.h` + `hello.c`/`logger.c` moved to `oec/sdk/rust/oec-ffi/` (still the contract; the C Action follows with increment 10). The REQ walker is `oec/tests/citations.rs`. The workspace excludes only the spikes; `oec/sdk/check.sh` has no sketch step.
- **A-144 Tests and runs:** `oec-core/tests/provider_action.rs` (10, incl. the guideline vectors V-PAY-008/009, 185, 186, 197, 199, 200, 203), `oec-core/tests/consumer.rs` (+3), `oec/tests/consumer.rs` (+1, two applications; a Level 1 application does not answer); 25 runs in a row green under load; interop `consumer_asks_a_provider` with the Action – the first `interop.sh` run failed because the demo had written its store into the generated dev root, fixed in the test setup (`Dev::root`), then 14 passed, 1 ignored; one-off D7 `oec-check monitor` against the demo and the interop consumer: 13 pass, 0 fail, T7-MON-07 passes, 0 invalid of 101; no new dependency, `cargo deny` without exception.

### A-145 … A-152 – P4 increment 10 (C-061, agent)

- **Where:** `oec/sdk/rust/oec-ffi/` (`src/{abi, types, app, asset, consumer}.rs`, `cbindgen.toml`, `include/oec.h`, `examples/{hello, logger}.c`, `tests/c.rs`, `tests/c/abi.c`); `oec/sdk/rust/oec/src/{config, data, app, event}.rs`, `oec-core/src/provider.rs`; `oec/sdk/check.sh`; ADR-0021 amendment C-061 (the ABI and its deviations), ADR-0020/0023 amendments C-061, ADR-0026 §20; log `log/2026-10-28_C-061_oec-ffi.md`.
- **A-145 Cycle cut:** the Rust API is complete since C-060; the ABI is one translation layer (handles, strings, JSON values, callbacks, errors) – crate, header, libraries, C tests and gate are one review topic. Not split; the additive `oec` API went first in its own commit (`fb857cb`).
- **A-146 Additive `oec` API:** `ConfigDef::from_json` (definition in the form of the published Config DataSet), `ConfigValues::to_json`, `DataSet::update_json`/`SetData::to_json` (OPC UA JSON, REQ-MSG-031), `AppHandle::emit_syslog` (Syslog event of one application without the process-wide `log` bridge); core `Provider::data_fields`. Nothing existing changed.
- **A-147 Generated header:** cbindgen 0.29.4 (CLI, pinned; the gate fails on another version or any difference to the committed header). The contract text (memory, strings, JSON, callbacks, threads, panics) is the `header` of `cbindgen.toml`; each function comment comes from the rustdoc incl. `# Safety`. The protocol enums are `int32_t` typedefs with `#define` constants of the same names and values: a C enum value out of range would be undefined behaviour in Rust, an integer is checked (`OEC_ERR_INVALID_ARGUMENT`). C code that uses the names compiles unchanged.
- **A-148 Signatures:** `oec_subscribe`/`oec_get` (and the new requests) take `service_type` before `app_il` – a request needs a concrete ServiceType + AppId since C-058; `OEC_ST_ANY` (−1) only in subscriptions. `oec_asset_emit_event` takes the `level` of the Generic event topic. `oec_dataset_set_precision` is replaced by `"Precision"` in the field JSON of `oec_asset_add_data` (as in Rust; a later change is a `Set PublicationList`). The `oec_set_fn` parameter is `json`.
- **A-149 Errors:** `oec_last_error()` is set only by a failing call (thread-local, valid until the next failure on that thread); new `OEC_ERR_NOT_CONNECTED` (8) and `OEC_ERR_STATUS` (9) with `oec_last_status_code()`; a panic is caught at the boundary → `OEC_ERR_PANIC`, and the app answers every later call with `OEC_ERR_PANIC`.
- **A-150 Threads and handles:** callbacks run on the thread that runs the app (`oec_app_run` caller or the SDK thread of `oec_app_start`), one at a time; `oec_get` callbacks on the caller's thread. Inside a callback the blocking functions, `oec_app_run` and `oec_app_free` return `OEC_ERR_INVALID_STATE` (they would wait for the thread running the callback); everything else is allowed, incl. freeing the own subscription. Child handles stay valid until their app is freed, even after remove (then `OEC_ERR_INVALID_ARGUMENT`). An error of the `oec_app_start` thread is not reported (risk; `oec_app_run` reports it).
- **A-151 Additions and the rest:** `oec_set`, `oec_del`, `oec_new_data_set_writer_id` (ADR-0021 amendment C-060), `oec_dataset_remove`, `oec_config_set`, `oec_builder_free`, `oec_last_status_code`, three StatusCode constants. Not in C yet (additive later; the logger demo needs none): presence, configurable/interval subscriptions, HealthDetail, ReferenceDesignation, `bump_revision_counter`, ClassId/description, connection state, ClientId/keep alive/MaxPacketSize, `run_async`. Packaging: `liboec_ffi.{so,dylib}`/`oec_ffi.dll`, `liboec_ffi.a`, `include/oec.h`; static linking needs the system libraries of `rustc --print native-static-libs`; release archives and pkg-config in P9. `OEC_ABI_VERSION` 1.
- **A-152 Tests and platform checks:**
  - 8 unit tests of the FFI layer (no network);
  - `tests/c.rs` (3 tests, Unix only): `hello.c`, `logger.c` and `tests/c/abi.c` built as C17 with `-Werror` against the shared library and against the static library with ASan + UBSan, run against the in-process TLS test broker; `abi.c` checks argument/state errors of every function, a provider + consumer in one process (`Get`/`Set`/`Del`, Config, the Action, the callback rules) and a stress phase of 6 threads × 150 rounds; sanitizers required in the gate (`OEC_FFI_REQUIRE_SANITIZERS=1`), LeakSanitizer on Linux;
  - repeated 5 × and under CPU load (up to 34 s, green); ASan caught a deliberate double free, LSan a deliberate leak;
  - once in Docker (`rust:1.89-bookworm`, arm64): C tests with gcc 12 + LSan, full workspace 340 tests; cross builds for Linux amd64, Windows amd64 (GNU) and ARMv7 with both crypto backends;
  - interop 14 green; the C logger and C hello once against Mosquitto;
  - no new dependency (`Cargo.lock` +`oec-ffi` only), `cargo deny` without exception.

### A-153 … A-163 – P5 increment 1 (C-062, agent)

- **Where:** `oec/conformance/internal/probe/` (`probe.go`, `session.go`, `judge.go`, tests), `internal/sim/sim.go`, `testdata/sim/index.yaml`, `cmd/oec-check/{test, monitor, main}.go`, `internal/report/monitor.go`, `internal/monitor/monitor.go` (`ForApp`/`NotFound` moved there); ADR-0025 amendment C-062; DESIGN.md §3.3/§10 "As built (C-062)"; ROADMAP P5 plan; log `log/2026-10-28_C-062_d7-test-engine.md`.
- **A-153 One subscription:** a test run subscribes once to `Oi4v2/#` (QoS 1). Every message goes to the monitor engine and the recorder, then to the runner. So the passive checks run during `test` and `monitor --replay` replays a recorded test run.
- **A-154 QoS 0 requests:** the new tester-violation check found that D7 published its requests with QoS 1. REQ-MQTT-017 makes QoS 0 the default, so this was fixed test-first. The requests are valid messages, except the deliberate unknown key of T7-GET-09 (REQ-MSG-001).
- **A-155 Tester:** `Utility/oi4.org/oec-check/D7/<12 hex digits>`, new per run, not announced (no Birth/MAM; announcing comes with `--announce`, increment 6, Q-029); `--tester-il` overrides.
- **A-156 Discovery:** `Get MAM/<AppId>` on the given ServiceType or on all seven; the first answer fixes the ServiceType. Fallback: a publication of the AppId seen on the bus. Not found: every REQ inconclusive (exit 4). The CUT is treated as an application (it answers for itself and its subordinate assets).
- **A-157 Waiting:** sequential tests; an answered request waits 250 ms more for duplicates; "no answer expected" waits the full `--timeout` (default 10 s).
- **A-158 Scope of a test:** an active test judges only the REQs its catalog entry cites; other observations come from the monitor rules.
- **A-159 Verdict of a test run:** passive REQs that a short run cannot judge (connect not seen, shorter than the heartbeat, MaxPacketSize unknown) are plain skips with "test run: …", not inconclusive, so `test` can exit 0. A level claim therefore needs a `monitor` run of at least 125 s as well (DESIGN.md §6).
- **A-160 Report:** the monitor report version 1 with `"mode": "test"` plus `tester` and `requests` (topic, MessageId, sent, replies). `report.schema.json` freezes it in increment 7.
- **A-161 Selection:** `--tests` takes ids or `path.Match` globs (comma-separated or repeated); a pattern that matches nothing is a usage error; non-selected tests are skip "not selected (--tests)".
- **A-162 CLI:** `--app [ServiceType/]AppId` is required and checked as an Identification Link, so the Oi4Identifier elements must be dnp-encoded (`OEC,20Demo`); the usage error says so. `--replay`/`--duration` are not flags of `test`; `--timeout`, `--tests`, `--tester-il` are new.
- **A-163 Probes and the cut:** T7-GET-05 asks Data (else Config) with the filter `oecCheckNone<hex>` for the empty list and the unknown Source `oi4.org/oec-check/D7/unknown-<hex>`; REQ-CMP-044 (MAY) is not stimulated. T7-GET-01 asks at most 20 subordinate Sources. At level 1 an unanswered optional resource is a skip. Cycle cut: increment 1 carries the infrastructure and only four tests; the plan and reasons are in ROADMAP P5 and the log.

### A-164 … A-175 – P5 increment 2 (C-063, agent)

- **Where:** `oec/conformance/internal/probe/{resources, helpers, adr, probe, session}.go`, `internal/sim/sim.go`, `internal/monitor/monitor.go`, `testdata/sim/index.yaml`, `testdata/captures/`, `catalog/d7.yaml`; ADR-0025 amendment C-063; DESIGN.md §3.3/§10 "As built (C-063)"; log `log/2026-10-28_C-063_d7-get-tests.md`.
- **A-164 Bounded stimuli:** T7-GET-03 requests at most 5 Filters per resource (taken from the first answer) and 50 PublicationList entries; T7-GET-01/04 and ADR-03 at most 20 subordinate Sources. A large CUT is sampled, not exhausted.
- **A-165 Metadata:** Get Metadata is requested for the Data the PublicationList announces (REQ-CMP-008/037, RES-META-002). Metadata without Data (REQ-CMP-044) is a MAY and not stimulated.
- **A-166 Pagination by size:** D7 cannot make the CUT's content larger than `MaxPacketSize`; T7-GET-06 stimulates pagination on request (PerPage 1: page 1, a page beyond the end, page 0) and T7-MON-18 judges the size of every NetworkMessage passively.
- **A-167 ADR-02:** D7 speaks only `Oi4v2` and listens to `Oi4/#` best effort. An `Oi4` message with the MessageId of a D7 request fails REQ-TOP-004; without any `Oi4` traffic the REQ is skipped (the CUT supports one version, condition of REQ-TOP-004). A dual-version CUT that stays silent under `Oi4` during the run passes.
- **A-168 "Ignored":** a Query with Source, a Query on Get MAM, an undefined query parameter and an unknown key are judged against the answer to the same Get without them, not against an expected content. Without any ReferenceDesignation object the Query REQs (RES-REFDES-011/014, QRY-002/003) are skipped. The Parent is the first one with fewer children than objects, so a supported and an ignored Query differ.
- **A-169 Locale:** the answer to a Get with `de-DE` may carry `de-DE`, the default language `en-US` or the invariant locale; whether the CUT has German texts is not observable.
- **A-170 Monitor, Locale:** an answer to a Get with a Locale differs from the Birth by design; the monitor engine records the MessageIds of such Gets and does not compare their answers with the Birth (REQ-LIFE-003).
- **A-171 Monitor, one page:** for a Get with `Page` > 0 only that page is published (REQ-PAG-002); the engine records the requested page and does not require such an answer to end the series (REQ-PAG-005). The capture `pagination-broken` relied on the old behaviour and is corrected (R-2).
- **A-172 Sim content:** added from reviewed vectors V-PAY-021 (Metadata), V-PAY-047 (MAM in German), V-PAY-054 (ReferenceDesignation of the asset). The Config WriterGroupName "Network settings" (V-PAY-092) is used as "Networksettings", because a Filter with white space is a REQ-TOP-020 warning (A-4). The unknown-key fault knows the keys of `ua-metadata`.
- **A-173 Contract:** the requests of D7 that strict validation rejects are exactly the deliberate stimuli without a fault (`REQ-MSG-001` ×2, `REQ-QRY-001`, `REQ-RES-REFDES-013`); with a fault a part of them, because a test may stop before its stimuli.
- **A-174 Timeouts:** the fixture runs wait 3 s for an answer (was 2 s) and 2 min per run (was 1 min); 19 fixtures run in parallel, about 43 s.
- **A-175 Cut and findings:** increment 2 was done in one cycle. No SF: the Filter of Config (WriterGroupName, REQ-TOP-022) and `Context.Name` (REQ-RES-CONFIG-015) were checked and are consistent after SF-085.

### A-176 … A-190 – P5 increment 3 (C-064, agent)

- **Where:** `oec/conformance/internal/probe/{set, probe}.go`, `internal/sim/{sim, write}.go`, `internal/report/monitor.go`, `cmd/oec-check/test.go`, `testdata/sim/index.yaml`, `catalog/d7.yaml`; ADR-0025 amendment C-064; DESIGN.md §3.3/§10 "As built (C-064)"; log `log/2026-10-28_C-064_d7-set-tests.md`.
- **A-176 Cut:** increment 3 was done in one cycle; T7-SET-10 (survives a restart) needs the restart hook and stays in increment 6.
- **A-177 Restore model:** before the first and after the last write test D7 reads Config, PublicationList, ReferenceDesignation and SubscriptionList with Get without Source and compares them. Data is excluded (its values change by themselves); what the component stores on disk is not compared.
- **A-178 Reported, not judged:** the restore result is a statement about D7, not about the component: report key `restore` (report version 1 unchanged), a "restore:" line in the text report, and a warning on stderr with the changed lines.
- **A-179 Write-backs:** a Set sends what the component published; Config as the full DataSet including the mask of sensitive values; Data only where a Set subscription is listed.
- **A-180 Bounded destructive requests:** only SubscriptionList entries D7 created are deleted; a ReferenceDesignation only with a known state (and see A-189) and restored with Set; `NONE_0` entries get only a change that must be rejected, undone if accepted; changed Modes are set back.
- **A-181 "Nothing published":** REQ-BUS-007 is judged until the Status event plus the grace time.
- **A-182 SET-03:** runs after the other write tests, reuses their reactions and stimulates only missing combinations; supported = any reaction with the `ReplyTo` except BadNotSupported.
- **A-183 SET-01 and REQ-RES-SUBLIST-016/CMP-016:** Good → checked; BadNotWritable/BadNotSupported → violation; other outcomes → skip (catalog note).
- **A-184 Stimuli:** unknown objects carry `d7`/`D7`; a created ReferenceDesignation takes its DataSetWriterId from the PublicationList (else 10); SET-05 sends one deliberately invalid Number (tester violation REQ-RES-CONFIG-007).
- **A-185 Status event:** the Event of the category Status whose `ReplyTo` is the MessageId of the request; level from the severity bits of the StatusCode; Source = Source of the request, or the AppId for an asset the entity neither is nor represents (SF-108).
- **A-186 Sim:** write side and `Changed()`; one Status DataSetWriterId per Source and level (REQ-MSG-021; the Get-path Bad event moves from 623 to 625); a sensitive Config object, two Syslog entries and the Set subscriptions added; only the application's Data is writable.
- **A-187 Contract:** every fixture run has a restore result that agrees with `sim.Changed()`; without a fault the state is restored; `Changed()` is empty after every run.
- **A-188 Masked values at level 1:** REQ-RES-CONFIG-025 (the mask in a Set leaves the secret unchanged) is only SHOULD at level 1. There D7 does not write a Config DataSet with a masked value at all (the component could store the mask, and the restore check cannot see it) and skips CONFIG-025. The restore note says that sensitive values cannot be compared.
- **A-189 ReferenceDesignation Del:** only after a write-back Set of the object got Status Good; otherwise SET-09 skips REQ-RES-REFDES-010, SET-04 uses its own subscription, SET-03 skips the Del (new conforming fixture `T7-SET-09/set-rejected`).
- **A-190 Interrupted run:** if the run ends during the write tests (error, broker lost, interrupt), the restore is reported as not checked with a note naming the test, and `oec-check test` warns; no recovery attempt.

### A-191 … A-201 – P5 increment 4 (C-065, agent)

- **Where:** `oec/conformance/internal/probe/{action, probe}.go`, `internal/sim/action.go`, `internal/monitor/{config, monitor}.go`, `testdata/sim/index.yaml`, `testdata/captures/`, `catalog/d7.yaml`; ADR-0025 amendment C-065; DESIGN.md §3.3/§10 "As built (C-065)"; log `log/2026-10-28_C-065_d7-action-monitor.md`.
- **A-191 Cut:** T7-MON-14 (Mode, Interval and Precisions of the PublicationList, judged over the whole observation) needs a timing model none of the other rules needs; it moves to C-066 (increment 5). The rest of increment 4 in one cycle.
- **A-192 Action stimuli:** `ua-action-request` on `Call/NewDataSetWriterId[/<Source>]` with ResponseAddress = the matching `Reply` topic, `RequestorId` = the tester, `TimeoutHint`, optional CorrelationData; DataSetWriterId 8, ActionTargetId 1, Payload `{"Resource": "Data"}`. Responses are matched by `RequestorId` and `RequestId` on any Reply topic of the component, so a wrong topic is a finding, not a timeout.
- **A-193 Write window:** the Action tests run after the write tests, inside the restore window. A provided id is only reserved; D7 never uses it and creates nothing, so nothing must be restored (demo: restored).
- **A-194 Must answer:** if the Profile lists `NewDataSetWriterId` or REQ-CMP-040 is MUST at the declared level; otherwise an unanswered call is a skip.
- **A-195 Limits:** ACT-02 cannot see the initial use of a provided id within its Ttl (D7 creates no DataSet; manual part, catalog note). ACT-03 sends two schema-invalid requests on purpose (tester violation REQ-ACT-019) and judges silence only if a valid call beside them is answered.
- **A-196 MON-09:** the structure of a DataSet is the sorted list of its field names (Metadata `Fields`, Data payload keys) per resource, DataSetWriterName and WriterGroupName; field types are not compared.
- **A-197 MON-13:** without Profile and without publication the rule skips as "not observable", not inconclusive (`test` requests the Profile). Otherwise the reviewed `lifecycle-pass` would have become INCONCLUSIVE.
- **A-198 MON-15:** after Status Good the list counts as published again on a Pub with an empty `ReplyTo` or the `ReplyTo` of the Set (the latter shows the MAY REQs PUBLIST-018/SUBLIST-006), within the tolerance (5 s). After a Del SubscriptionList the absence is not judged (REQ-RES-SUBLIST-015, remaining entries not observable).
- **A-199 MON-16:** the RevisionCounter never decreases and does not change between -1 and a counter; after a Set Config with Status Good that changes a known non-sensitive Value, a later MAM must carry a larger counter – not exactly +1 (several changes may lie between). ReferenceDesignation changes are not counted (A-113 unchanged). D7 makes no value-changing Set Config, so a `test` run judges only monotonicity.
- **A-200 MON-17:** the requester must include every mandatory object of each group it sends, judged against the Pub Configs of the addressed Source and WriterGroupName, merged per object. In `test` only the observations about the component count, so the tester's own Sets are not judged.
- **A-201 Details:** `retag` uses the Config tag of MG B1.1.3 without blanks in three captures (REQ-TOP-020); the progress variant of the sim carries the response fields in the intermediate response (REQ-ACT-012); `answers-unknown` also expects T7-MON-01 REQ-ACT-019; the report's `requests` list the Action calls in the order sent, with their ActionResponses as `replies` (fixed test-first).

### A-202 … A-214 – P5 increment 5 (C-066, agent)

- **Where:** `oec/conformance/internal/proxy/`, `internal/probe/{lifecycle, probe, session, sim_test}.go`, `internal/sim/sim.go`, `internal/monitor/monitor.go`, `cmd/oec-check/{proxy, test}.go`, `internal/report/monitor.go`, `testdata/sim/index.yaml`, `catalog/d7.yaml`; ADR-0025 amendment C-066; DESIGN.md §3.3, §3.4, §10 "As built (C-066)"; log `log/2026-10-28_C-066_d7-proxy-lifecycle.md`.
- **A-202 Cut:** T7-MON-14 moves again, to C-067. The proxy filled the cycle; MON-14 (timing model of the PublicationList) is independent of it.
- **A-203 Own relay:** `internal/proxy` on the Go standard library. paho has only a client side and decodes leniently; D7 must see the packets exactly. CONNECT and PUBLISH are decoded strictly, every frame is forwarded unchanged. No new dependency.
- **A-204 Certificates:** the tester gives a CA the component trusts (`--proxy-ca`, `--proxy-ca-key`; test material in a temporary directory, never committed). D7 issues the relay and the wrong-host certificates from it and an untrusted CA of its own. Upstream the proxy uses D7's TLS settings and the component's CONNECT (its credentials). A client certificate of the component is seen, but cannot be relayed – a component that authenticates only with a client certificate cannot be tested through the proxy yet.
- **A-205 Wait before discovery:** through the proxy D7 waits for the component's Birth and the broker's first SUBACK (at most CONNECT + `--timeout`) before its first Get. A conforming component may subscribe after its Birth (the SDK does); without the wait the discovery Get was lost (demo run 1).
- **A-206 Connection phase last:** after all other tests (outside the write window): TLS 1.1 only, untrusted CA, wrong host name, then the drop without DISCONNECT and the wait for the reconnection with a Birth (`--proxy-wait`, default 2 min). A phase the component never reaches is an inconclusive skip.
- **A-207 CON-01:** keep alive ≠ 60 s and clean session false are notes (MG 6.2.3 names defaults; REQ-CMP-009 grades them); an empty or changing ClientId is a violation.
- **A-208 Will created anew (REQ-LIFE-016):** the time in the Will's MessageId lies within `--tolerance` (5 s) of the CONNECT, and no MessageId repeats; a MessageId without a time is a violation.
- **A-209 LIFE-02:** a Birth is the Pub MAM about itself without `ReplyTo`; it must be the first PUBLISH of every connection (REQ-LIFE-015) and come within `--timeout` (REQ-LIFE-001). After a Health `FAILURE_1`/0 about itself the next other Health must follow a new Birth (REQ-LIFE-017); without such a failure a plain skip.
- **A-210 Without proxy:** every CON/LIFE requirement is an inconclusive skip, the run ends with exit 4 (unless `--allow-skip`); a retained message of the component on D7's subscription is still a CON-04 finding.
- **A-211 Monitor:** MON-03 reports a missing Birth after a Will only when the connection or the observation ends without one (a late Birth is REQ-LIFE-015, judged by LIFE-02); MON-10 does not count a Birth after recovery or a late Birth as a second Birth. Unit tests first; the reviewed captures keep their verdicts.
- **A-212 Sim:** through the proxy the simulated component starts independently of D7 (like a real one), reconnects in a loop after a drop, creates its Will per connection with QoS 0 (it was QoS 1; REQ-MQTT-017 QoS 0 as default); variant `late-subscribe`.
- **A-213 Harness:** every sim fixture runs through the proxy; an entry may select its tests (`tests`); an expected test without a result is an error; `none` judges CON/LIFE and is back to `draft`.
- **A-214 Report:** `attachment` `"proxy"`, `proxy_listen` and `connections` (every attempt: phase, TLS version, CONNECT without password, CONNACK, PUBLISH topics and times, first SUBACK, drop); one text line per attempt; report version 1 unchanged (additive).

### A-215 … A-221 – P5 increment 6a (C-067, agent)

- **A-215 Cut:** increment 6 is split. C-067 (6a): the restart hook, T7-LIFE-04, T7-SET-10 and T7-MON-14. C-068 (6b): the peer simulation, `--announce` (Q-029 (a)) and T7-ADR-01 – a second simulated component with its own lifecycle is new infrastructure like the proxy. `list`/`explain`, the report schema and the P5 exit run move to C-069. ROADMAP table adapted.
- **A-216 Restart hook:** `--restart-cmd CMD`, or `--stop-cmd CMD` + `--start-cmd CMD`, run by `sh -c` (Windows `cmd /C`), output to stderr; a start in the background must redirect its output (else D7 waits up to 10 s for it). `--restart-wait` (default 2 min) bounds the wait for the component to come back. D7 does not stop processes or containers itself: it cannot know how the component is deployed (systemd, Docker, a script).
- **A-217 Restart phase:** inside the write window, after the other write tests and before the restore check: SET-10 sets its change, the hook runs, D7 waits for the new Birth (through the proxy also the new connection and its first SUBACK) and an answer to Get Health, then LIFE-04 and SET-10 judge, and SET-10 restores. The restore check therefore also covers the restart.
- **A-218 LIFE-04:** the Close message is the last Pub Health of the component (no answer) after the hook started and before its DISCONNECT (proxy) or before its new Birth (bus). Judged: present (REQ-LIFE-005), on the Health topic of the application (006), Health `NORMAL_0`/0 with the DataSetWriterId of its Health from Get Health after the restart (007), graded by REQ-CMP-046. PUBLISH packets after the Close message before the DISCONNECT are a note. Inconclusive: no hook, the connection did not end, no DISCONNECT (proxy), the Will on the bus, no new Birth within `--restart-wait` (bus).
- **A-219 SET-10:** D7 changes the ReferenceDesignation of the application (else the first object) by appending "D7" to Value and Local of its first aspect. Not applicable (plain skip) without a ReferenceDesignation object or with Status Bad on the Set: the condition of REQ-RES-REFDES-002 (the component offers Set ReferenceDesignation) is not met.
- **A-220 MON-14:** the rules are the entries of the last observed Pub PublicationList of the component (an absent Mode is `ON_REQUEST_1`). A Set/Del PublicationList addressed to it makes them uncertain until its next Pub PublicationList; within the tolerance after a changed entry or a connect nothing is judged; Birth, Will and Close are never judged. Mode: no unsolicited publication for `ON_REQUEST_1` (008, not MAM/Health); every publication uses a form its Mode enables (009 … 011). Interval > 0: the gap per form ≤ Interval + tolerance (013, plus 014 with Precisions), also at the end. Interval 0 with Precisions: a change only below a Precision is a SHOULD finding (015). Interval 0: a Get answer that shows a significant change must be followed by its unsolicited publication within the tolerance (013, REQ-MQTT-019). Not judged: `OFF_0` (T7-SET-08), Event intervals, forms that are never used (completeness), several DataSetMessages in the filter form, Get answers with a Locale. A component that publishes without an observed PublicationList is an inconclusive skip (`test` requests the list). No SF: the reading of "enables" matches the reviewed REQ-RES-PUBLIST-009 … 011.
- **A-221 Fixtures:** the sim harness wires the restart hook to every sim fixture (LIFE-04 and SET-10 pass on the reviewed ones); `T7-LIFE-02/no-birth` and `/health-first` select LIFE-01 … 03 (the restart would move their MON-03 finding); `lifecycle-pass` gets B's PublicationList and is back to `draft` (R-5); the capture generator gives Get PublicationList its DataSetClassId (schema).

### A-222 … A-228 – P5 increment 6b (C-068, agent)

- **A-222 Cut:** increment 6b is delivered completely in C-068 (`--announce`, `--peer`, T7-ADR-01); C-069 stays as planned (increments 7/8: `list`/`explain`, the report schema, the P5 exit run). ROADMAP table: C-068 ✅.
- **A-223 `--announce`:** D7 is a Level-1 `Utility` under its tester IL on its main connection; its Will is set in D7's CONNECT. Birth (MAM, Health `NORMAL_0`/100), "present" Health, heartbeat every 30 s, Close message (`NORMAL_0`/0) at the end; it answers Get MAM, Health and Profile. `--tester-il` must then be an Oi4Identifier (4 elements): the MAM takes its elements from it. Default unchanged: unannounced (Q-029 (a)).
- **A-224 Peer:** `Utility/oi4.org/oec-check/Peer/<12 hex>` (new per run) on its own MQTT connection (ClientId `oec-check-peer-…`), with the same lifecycle and answers as the announced tester. A Get for an unknown Source gets Event Status Bad `BadNotFound` and no Pub (REQ-CMP-047), a Get with a Filter an empty list, other resources are ignored. A lost peer connection ends the run with an error ("peer connection lost").
- **A-225 "Present" Health:** once D7 has found the component under test, every announced component publishes its Health once more. A component that connected after their Births – always through the proxy, because it can connect only when D7 listens – sees them by this Health, without retained messages (which REQ-MQTT-018 forbids).
- **A-226 T7-ADR-01:** a request to the peer is a Get/Set/Del/Call of the component under test (its PublisherId) whose topic carries the peer's AppId or the peer as Source. Each is checked against `Oi4v2/Utility/<peer>/<Method>/…`: ServiceType (REQ-TOP-006), AppId (REQ-TOP-010), both (REQ-BUS-002); one violation per distinct topic; judged at the end of the run (requests after the restart count). Without `--peer` an inconclusive skip (D7 cannot provoke the request; plain runs therefore end with exit 4 unless `--peer` or `--allow-skip`), with the peer but without requests a plain skip (the REQs bind requesters only). Requests to the announced tester are counted in the report, not judged.
- **A-227 Sim and harness:** the simulated component is a consumer like the SDK directory: it subscribes to `Pub/MAM` and `Pub/Health` of all applications – after its request subscriptions, because D7 takes the first SUBACK as "ready" (found by `TestSimLateSubscribe`) – and asks every new application for its MAM. 3 faults (58 in total), index fields `peer` and `announce`; the harness checks the announced components (count, requests answered, no finding of the peer). `none` runs with both flags and is back to `draft` (R-6).
- **A-228 Report:** key `announced` (per component: role `tester`/`peer`, `publisher_id`, `requests` received, `answered`) and one text line per component; report version stays 1 (only added).

### A-229 … A-238 – P5 increments 7/8 (C-069, agent)

- **A-229 Cut:** increments 7 and 8 in one cycle as planned; 7 adds no test and no fixture, 8 is one run with the C-068 recipe. ROADMAP table row C-069 ✅, exit assessment in ROADMAP P5.
- **A-230 Root package:** `report.schema.json`, `ValidateReport` and the loaders of `testdata/sim/index.yaml` and `testdata/captures/index.yaml` live in the module root package `conformance` – `go:embed` cannot reach `testdata/` from a sub-package, and `internal/report` imports `probe`, so the sim tests in `probe` cannot use `report`. The root package imports no tool package.
- **A-231 Report schema:** JSON Schema 2020-12, report version 1 of `monitor` and `test`, closed objects; conditional keys per mode (`test` needs `app` and `tester`; `replay` needs `capture` and has no `broker`; live/test need `broker`) and attachment (`proxy` needs `proxy_listen`, only then `connections`). `validate`'s own report (P2) is not covered. Validated in the gate: the CLI tests (`--format json`, `--report json=`), every capture replay at levels 1–3, and every sim fixture run (its report document is assembled in the test with the keys of `report.Monitor`, because of the import cycle of A-230; the CLI tests cover the real `report.Monitor`). A changed key or meaning is a new report version.
- **A-232 Request labels:** the `test` of a request in the report is a test id or one of the phases `discovery`, `state` (snapshot for the restore check), `restart` (after the hook). The schema found that the Profile read during discovery had an empty label; it is now `discovery` (`internal/probe/probe.go`).
- **A-233 `list`/`explain`/`report-schema`:** only embedded data (catalog, `spec.FS`, the fixture indexes); no network. `explain` shows the fixtures statically with the outcome their index states; a positive fixture is listed as "pass or not applicable"; a sim fixture without the peer is not shown for a test that needs the peer. Usage errors (unknown id, more than one id, unknown filter value) exit 2. `version` now also names the catalog baseline and the report version. JSON tags on the catalog types for `--format json`.
- **A-234 Coverage report:** `oec/conformance/catalog/COVERAGE.md`, generated by `internal/explain` (`go test ./internal/explain -run TestCoverageReport -update`) and diffed by the gate. DESIGN §6 planned it in `spectool`; but the tools of `oec/spec` must not import `conformance/` (ADR-0018 §5), so it lives in D7. `TestFixtureCoverage` enforces a negative (fail, warn or info) and a positive fixture for every implemented test.
- **A-235 Test count:** D7 has 55 tests (19 monitor, 36 test). T7-ACT-04 (unknown Source → `BadNotFound`, REQ-ACT-024) came with the `eb7889c` re-baseline in the C-060 review; the docs since then said 54. Corrected in the current-state texts (STATUS, ROADMAP, README, DESIGN); older logs stay as written.
- **A-236 Exit criterion 2:** "every negative fixture fails" is read as "fails at the obligation of its rule": for a SHOULD (T7-GET-09 REQ-MSG-046, T7-SET-12 REQ-TOP-027) the negative fixture expects `warn`, for the heuristic T7-MON-10 (uniqueness of the IL is not provable by observation) `info`. All other tests have a `fail` fixture.
- **A-237 Exit criterion 4 and distribution:** CI on Windows/macOS/Linux and the multi-arch container image come with P9 (release); C-069 records the native macOS arm64 gate and `CGO_ENABLED=0` cross builds for windows/linux/darwin × amd64/arm64 (go1.27.3, about 14–16 MB each).
- **A-238 Dependencies and other parts:** no new dependency (the report schema uses `santhosh-tekuri/jsonschema/v6` of `validate`), no schema change (D1 stays `2.0.1-dev` @ `eb7889c`), the SDK and `demo.rs` unchanged, no SF.

### A-239 … A-248 – P5b plan and increment 1 (C-070, agent)

- **A-239 Readers:** OCI and SBOM read with the Go standard library (`archive/tar`, `compress/gzip`, `crypto/sha256`, `encoding/json`) instead of `go-containerregistry` (planned in DESIGN §9) or SPDX/CycloneDX libraries; D8 needs no registry, auth or remote code and only the license parts of an SBOM. No new dependency.
- **A-240 Input forms:** an OCI image layout directory or an uncompressed oci-archive. A registry reference, a Docker archive with only `manifest.json` and a gzip tar are input errors (exit 2) with a hint: `docker save` (Docker 25+ writes an OCI layout), `skopeo copy docker://REF oci-archive:FILE`, `podman save --format oci-archive`.
- **A-241 Reading rules:** Docker schema-2 media types accepted, schema 1 is a structure problem; attestation manifests and `unknown` platforms are not runnable images; zstd layers are not read (no decoder in the standard library) and make T8-IMG-03 inconclusive (exit 4).
- **A-242 Platforms:** supported (RIG 5.1 ARMv7, ARMv8, amd64) are linux/amd64 (variant `""`/`v1`), linux/arm64 (`""`/`v8`), linux/arm (`""`/`v7`). No supported platform → REQ-APP-001 and REQ-CMP-032 fail; fewer than two → REQ-APP-002 warns.
- **A-243 Digests:** T8-IMG-04 only warns (REQ-CMP-030 is a SHOULD); a digest mismatch fails T8-IMG-01 (REQ-CMP-027); no signature check (no REQ).
- **A-244 Report:** own `infra-report.schema.json` (report version 1 of `oec-infra`), grows additively until the P5b exit and is frozen then; `Evidence` gains `image` and `path`; the JUnit writer and the test/summary text are shared with D7, whose output stays byte-identical.
- **A-245 SBOM rules:** as the SF-114 proposal; two SBOMs → REQ-CNT-013 fail; notes (`info`) for a `VOLUME` over `/opt/oi4`, packages without a license and an external `DocumentRef`.
- **A-246 Fixtures:** generated by `go test ./internal/image -run TestFixtureFiles -update` and byte-compared in the gate; index semantics as the sim fixtures; every implemented test has a positive and a negative fixture; about 260 KB.
- **A-247 Plan:** P5b in six increments C-070 … C-075, exit as in ROADMAP P5b; demo/registry images → P6/P7 exits, CI → P9.
- **A-248 Design:** DESIGN §9 corrected and "As built (C-070)" added; `internal/explain` and `COVERAGE.md` stay D7-only until increment 6, D8 then gets its own coverage file so the D7 `COVERAGE.md` stays unchanged.

### A-249 – SBOM rules after SF-114 (C-070 review, agent)

Replaces A-245; ADR-0025 amendment "C-070 review".
- **Formats:** `sbom.spdx.json` is SPDX 2.x JSON or SPDX 3.x JSON-LD, `sbom.cdx.json` is CycloneDX. A version or construct D8 cannot read (SPDX other than 2.x/3.x, CycloneDX other than 1.x, unknown SPDX 3 license types) is no violation: REQ-CMP-028 passes, the content REQs are skipped inconclusive (exit 4). A file that is no SBOM of its format fails REQ-CMP-028/REQ-CNT-013.
- **Components:** SPDX packages / `software_Package`, CycloneDX `components`; the CycloneDX `metadata.component` is the subject of the SBOM (its licenses need texts, but it needs no identifier); SPDX files and snippets are no components. Missing name or version → REQ-CNT-013 fail; `NOASSERTION` or no license → REQ-CNT-026 fail, `NONE` passes; no purl/CPE/SWID/other persistent id → REQ-CNT-027 warn.
- **Texts:** every license id, `LicenseRef-`, exception after `WITH` and `AdditionRef-` needs a text (embedded or `<id>.txt`); a name-only license needs an embedded text.
- **License ids:** checked for syntax, not against the SPDX license list (it changes over time; the guideline does not restrict versions).
- **Notes (`info`):** references to other SPDX documents and SPDX 3 imports, a `VOLUME` over `/opt/oi4`, an empty component list.
- **No new dependency:** SPDX 3 JSON-LD is read with `encoding/json`; the context is matched by URL, not expanded.

### A-250 … A-258 – P5b increment 2 `bus` (C-071, agent)

ADR-0025 amendment C-071; DESIGN §9 "As built (C-071)".
- **A-250 Command:** `oec-infra bus` takes the connection flags of `oec-check monitor` (`--broker`/`--bus-storage`, `--ca`, `--cert`/`--key`, `--username`/`--password-file`, `--insecure` only explicitly, `--config`); no `--client-id`, because D8 opens several connections. D8 publishes only on its own probe AppId `Utility/oi4.org/oec-infra/D8/<12 hex>` and its two probe components, so a run never writes to an application of the platform.
- **A-251 Second client:** `--credentials-2 FILE` (format of `run/secrets/oi4/credentials`) or `--bus-storage-2 ROOT`; without it, or with the same identity, T8-BUS-02 is inconclusive (exit 4 without `--allow-skip`), not a usage error.
- **A-252 T8-BUS-01:** raw TLS handshakes with the standard library for exactly 1.2 and exactly 1.3 (one must work) and 1.0 … 1.1 (must fail), all with the client certificate of the run; plain-text listener on `--plain-port` (default 1883) fails only with a CONNACK; chain against the CA of the run, self-signed fails.
- **A-253 MaxPacketSize (SF-105):** one QoS 1 PUBLISH with a payload of exactly MaxPacketSize on a topic padded to 65535 octets must come back without disconnect; MaxPacketSize from `--max-packet-size`, `broker.json` or 262144 (note); above `--max-probe-size` (64 MiB) inconclusive.
- **A-254 T8-BUS-02:** Set of six resources, Del of two, Call NewDataSetWriterId by the unprivileged client; a positive control (D8 receives its own requests) first; a denial may be silent, SUBACK 0x80 or a disconnect.
- **A-255 T8-BUS-03:** Registry from `--registry` or discovered on `Oi4v2/Registry/+/Pub/#` within `--registry-wait` (65 s, one heartbeat of SF-106 plus slack); two probe components (Will, Birth, second Health), Get MAM/Health with and without Source, then Close and Will; no Registry → inconclusive, against `oec/dev` until P7.
- **A-256 Self-test:** `mqtttest` gains options (TLS bounds, certificate variants, plain listener, ACL callback, payload/packet limits, protocol levels), `NewBroker` and the D7 gate unchanged; `registrytest` with seven faults; 29 scenarios, `draft`.
- **A-257 Report:** `infra-report.schema.json` additive: `bus` in `command`/`mode`, key `broker` (url, probe, tls, certificate, max_packet_size, registry), evidence `topic`; report version stays 1.
- **A-258 Dev broker ACL:** only `oec-probe` may publish Set, Del and Call; everyone reads `Oi4v2/#` and publishes Pub, Get and Reply. Mosquitto cannot bind the AppId level to a user and applications read their own Set/Config, so T8-BUS-02 warns REQ-RES-CONFIG-026 against `oec/dev` – a dev limitation, no SF.

### A-259 … A-270 – P5b increment 3 `host` (C-072, agent)

ADR-0025 amendment C-072; DESIGN §9 "As built (C-072)"; log C-072.
- **A-259 Command:** `oec-infra host` reads the storages of RIG 4.1 below `--root DIR` (default `/`); `--mqtt-storage`, `--certs-storage`, `--secrets-storage` override one storage, `--container` the name of `ROOT/etc/hostname`; `--daemon`, `--offline`, `--registry-wait` (65 s), `--timeout`, `--tests`, `--format`, `--report`, `--allow-skip`, `--config` (the YAML loader of `bus`, now shared). Read-only, at most 1 MiB per file; the report lists names, modes and sizes, never a content or secret. Level 3.
- **A-260 T8-HST-01:** `broker.pem` is verified against `broker_ca.pem`, else `ca.pem`; a `broker_ca.pem` equal to the CA is a note; an expired chain is verified at the middle of its validity and noted (validity and host name are T8-BUS-01, MG 6.2.2). `broker.json` against `platform/broker.schema.json` with the D1 validator (a unit test runs the 18 V-BRK vectors) and its `Address` (REQ-MQTT-003).
- **A-261 T8-HST-02:** `ca.pem` holds a CA certificate; the secret storage is a closed set (`credentials`, `private_key.pem`, `passphrase`) with at least one login; an encrypted key if and only if a passphrase, which must open it; the key needs its certificate `<containername>.pem` (another name: warn, REQ-CNT-007 is a SHOULD); a private key in the other storages: warn (REQ-CNT-002, SHOULD); username characters per REQ-MQTT-011.
- **A-262 Mounts:** judged only from the Linux mount table (`/proc/self/mountinfo`): `ro`/`rw` when the storage or a file in it is a mount point (Docker mounts secrets per file) or a read-only mount covers it; `rw` fails; no own mount fails only if D8 runs in a container (`/.dockerenv`, `/run/.containerenv`); elsewhere `unknown`, not judged. World-writable entries are a note.
- **A-263 Verdict kind `Reported`:** a measured value a human judges gets the outcome `manual` on its requirement (already in both report schemas); rank 0, so the test keeps its outcome. D7 never emits it.
- **A-264 Hardware:** architecture from the kernel (ARMv7, ARMv8, amd64); RAM ≥ 10^9 octets pass, 768 MiB … 10^9 manual, below fail; storage ≥ 4 · 10^9 pass, 3.5 · 10^9 … 4 · 10^9 manual, below fail (SF-116); a physical interface that is up passes, only virtual or down interfaces are manual, none fails. Facts per OS on the standard library (`syscall`; Windows `kernel32` via `NewLazyDLL`), no `golang.org/x/sys`.
- **A-265 Linux and patch level:** another OS fails REQ-PLT-003; on Linux the kernel, the os-release and `reboot-required` are reported for a manual verdict. Package managers are not queried (distribution-specific, needs network).
- **A-266 Container daemon:** `--daemon`, else `DOCKER_HOST`/`CONTAINER_HOST`, else the default sockets of the OS (Docker and Podman); a connect within 2 s is enough (the Engine API comes with C-073); permission denied → manual; an unsupported scheme → inconclusive.
- **A-267 Broker and Registry:** D8 connects to the broker of `broker.json` with the login of the storages (REQ-PLT-007) and waits `--registry-wait` for a publication on `Oi4v2/Registry/+/Pub/#` (REQ-PLT-008). No Registry fails here, because the platform must provide one (T8-BUS-03 says inconclusive, since it checks the broker). `--offline` → inconclusive.
- **A-268 Self-test:** `internal/host/hosttest` writes the roots with the `mqtttest` PKI at run time; 55 scenarios with fake facts, mount tables and daemon dialers, T8-HST-04 against `mqtttest` + `registrytest`; no root, no network beyond loopback, no Docker. `HostFixtures()` is not part of `Fixtures()`, `COVERAGE.md` unchanged (A-248). Report key `host`, additive, report version 1.
- **A-269 32-bit build:** `internal/probe` `BadNotFound` typed `int64`; before, `oec-check`/`oec-infra` did not compile for `GOARCH=arm`/`386` (ARMv7 is a RIG 5.1 platform). D7 behaviour unchanged.
- **A-270 Live runs:** `live.sh` runs `TestHostDevRoots` (both sample roots on this computer) and `TestHostDevContainer` (the Linux binary in a container in the network namespace of the dev broker, storages `ro`, once the secret storage `rw`). Result in the C-072 log; no deviation in `oec/dev`.

## Next (C-073)

1. C-073: P5b increment 4 – Engine API client `internal/engine` (read-only, Unix socket / named pipe), `oec-infra container` (T8-CTR-01 … 03) and T8-HST-03 (application storages not shared), self-test against a fake Engine API; in a new session (start prompt in `prompt.md`).
