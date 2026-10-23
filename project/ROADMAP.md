# Roadmap – OI4 OEC v2.0

The roadmap is iterative: phases are ordered by dependency, each ends with explicit exit criteria confirmed by the product owner. Phases may be re-planned when the guideline changes. Detailed planning of a phase happens in its first cycle.

```
P0 Onboarding ──► P1 Spec baseline ──► P2 JSON Schema ─────────────┬─► P5 Conformance tool ──┐
                                   └─► P3 SDK + tool design ──────┴─► P4 SDK core ───────────┴─► P6 Demos ──► P7 Registry ──► P9 Release
                                                                  ├─► P4b Micro SDK (D9)
                                                                  └─► P5b Infrastructure compliance tool (D8)
                         P8 Documentation (starts with P2, grows with every phase) ───────────────────────────────────────────┘

P2 already ships the offline `validate` command of the conformance tool (D7).
P4 and P5 can run in parallel once P3 is decided; demos (P6) and registry (P7) are verified with D7 (messaging) and D8 (containers). P4b (C, independent of the Rust SDK) needs P1/P2 (REQ catalog, schemas, test vectors) and the API sketch from P3; P5b can start after P1 (RIG requirements).
```

---

## P0 – Onboarding & governance ✅ (C-001, C-002)

- Study guideline v2.0 draft, RFC MS-010, legacy code.
- Create guardrails, roadmap, logging structure, research digests.
- **Exit:** product owner accepts GUARDRAILS + ROADMAP.

## P1 – Spec baseline (requirements catalog) ✅ (C-012 … C-032)

Turn the prose guideline into a precise, testable baseline – the foundation for schema, SDK and conformity checks.

- Requirements catalog `REQ-<area>-NNN` as YAML in `oec/spec/` (ADR-0018; areas: CMP, IL, TOP, MSG, ACT, MQTT, LIFE, RES-<resource>, PAG, LOC, QRY, BUS, CNT, LIC, APP, PLT, REG, GEN – section assignment in `oec/spec/requirements/areas.yaml`) covering **both** guideline documents (MG + RIG) with guideline reference, normative keyword per MG 1.2 (MUST/SHOULD/MAY and negations, G-1.6), **compliance domain (Messaging / Service Container) and level (L1/L2/L3)** and testability (`schema` = D1, `D7` = messaging live test, `D8` = container check, `manual`). Plan: C-012 log, updated by C-013 (no specific services after GL-ADR-014).
- Topic grammar as formal definition (ABNF + regex) per Method × Resource combination (which `Source`/`Filter` levels are required/allowed/forbidden), incl. percent-encoding of `+ # / %` within a `Filter` level (MG 7.7) and Status event levels `Good|Uncertain|Bad`.
- Method × Resource matrix (Pub/Get/Set/Del/Call/Reply) incl. payload expectations (e.g. `Get` = empty `Messages` or only Pagination/Locale).
- Review and extend the spec-findings register `oi4-oec-documentation/findings/SPEC-FINDINGS.md`; propose interpretations; get decisions from product owner / work group.
- Golden examples: all Appendix B examples, checked against the guideline description and built as compliant, meaningful test data (ADR-0018 amendment C-013); deviations become spec findings; the PO reviews the test data per increment (G-3.9).
- **Cycle cut (PO-confirmed, C-013), "Level 1 first":** C-014 `oec/spec/` skeleton + catalog schema + tools + normative-sentence inventory + `CMP` · C-015 guideline update SF-036 … 038 (PO request) + re-baseline `5cb109e` · C-016 PO decisions (Status event levels, SF-039, Q-019) + SF-035 keyword pass + re-baseline `de852d6` · C-017 GL-ADR-017 (MG 7.2 removed, MG 7 flattened, PO request) + re-baseline `bc5d063` · C-018 `IL`, `TOP`, topic grammar + topic columns of the Method × Resource matrix + topic/IL vectors (done) · C-019 SF-043 (e) in the guideline (dnp-encoding only as optional hint at the `Oi4Identifier`, PO request) + re-baseline `ea80271` (done) · C-020 SF-040, SF-042, SF-043 (a)–(d) in the guideline, REQ-TOP-007 withdrawn, IL reviewed (PO request) + re-baseline `5dedfd5` (done) · C-021 SF-041 in the guideline, TOP reviewed (PO request) + re-baseline `2cf0d70` (done) · C-022 SF-044 decided and recorded in GL-ADR-018, implemented in the guideline (PO request) + re-baseline `fd9da6e` (done) · C-023 `MSG` + `ACT`, golden payload vectors started (done) · C-024 SF-045 … 051 + SF-052 (tolerant reader) + SF-053 (A2 reserved ids) in the guideline (PO review of C-023) + re-baseline `a7d18f9`; second PO review: SF-052 SHOULD + strict/tolerant validation, SF-054 (B2 rebuilt), MSG + ACT reviewed, re-baseline `4af61f3`; third PO review: complete B1.1.5 Metadata example, SF-055, payload vectors reviewed, re-baseline `2f00e9f` (done) · C-025 `MQTT` + `LIFE` (drafted, golden Birth/Close/Will vectors, SF-057 … 060) (done) · C-026 SF-057 … 060 in the guideline (PO review of C-025) + re-baseline `720d862`; PO review: MQTT + LIFE + V-PAY-023 … 025 reviewed, SF-061 + re-baseline `c6453c7` (done) · C-027 core resources (MAM, Health, Profile, PublicationList, ReferenceDesignation) + `PAG` + `LOC` + `QRY` (DataSet content checks, golden vectors V-PAY-026 … 054, SF-062 … 074, GL-ADR-019/020; reviewed by the PO 2026-10-20) · C-028 `RES-DATA`, `RES-META`, `RES-EVENT`, `RES-SUBLIST` + `BUS` + payload columns of the Method × Resource matrix (all rows except Config) + golden vectors of B1.1.4, B1.1.6, B1.1.9 and the Status events of B1.1.3 … B1.1.10 (C-027/C-028 also correct the B1 examples per SF-056) · C-029 `RES-CONFIG` (144 items, split off C-028 by the agent as allowed by the PO, see `log/2026-10-20_C-028_data-meta-event-sublist-bus.md`) + payload columns of the Config rows + golden vectors of the B1.1.3 Config examples (its Status event is covered in C-028) · C-030 RIG areas `CNT`, `LIC`, `APP`, `PLT` + `REG` + `GEN` (drafted 2026-10-21, no split; SF-089 … 093; PO review 2026-10-22: reviewed; SF-089/090/092/093 implemented, re-baseline `b942df3`; SF-091 decided; closed 2026-10-22 with six follow-up drafts for the PO) · C-031 golden vectors + coverage report (done 2026-10-22, no split: examples coverage of all 144 MG/RIG code blocks, V-PAY-117 … 126, consistency review, SF-094/095 open) · C-032 PO review → P1 exit (done 2026-10-22: SF-091/094/095 implemented, re-baseline `5fb0fd8`, all drafts reviewed, P1 closed; afterwards MG 7.7 headings and MG 8.2.3/9.1.8, RIG 4.1.3 and whole-MG links, list anchors, MG 2 Guardrails, MG 2.1 update, MG Content list links (PO requests) + re-baseline `2025207`; MG 6.1 Protocol Version + MG 6.4 rewrite + re-baseline `75da059`; Welcome/start pages + MG 1/1.1 + re-baseline `c1419dd`; shop links removed + re-baseline `4811e60`; landing page + PO list edits + re-baseline `69863d3`; Conventions as MG 1.2 + RIG 1.2, RIG compliance → 1.3 + re-baseline `21bf409`; PO text edits + Model en-US rule MG 5 → MG 4.3.1 + re-baseline `9b6b22f`; missing links + re-baseline `f951bcb`; MG 7 note cleanup + RFC 5424 links + re-baseline `c840cb5`; MG 7.7 subheaders + re-baseline `c67ec35`; MG 7.7 Event filter restructuring (PO) + interactive landing page + re-baseline `e14726c`; MG 8.2–8.4 heading levels + re-baseline `485d8db`; Community section + navigation + re-baseline `12c66fa`; data-structure cards + admonition colors + re-baseline `985cd3d`; NOTE boxes + MG 7.7 colon + re-baseline `f024823`; card layout for all data-structure pages + re-baseline `98b77ef`; heading level fixes + re-baseline `6cd6f0e`; cards for MAM/Oi4Identifier/broker.json + re-baseline `f1e0cef`; RIG 4.1.1 note + MG 8.1 JSON overlays + re-baseline `f30d664`; Figure 5 Payload link + re-baseline `a7e59a3`; history passages removed + re-baseline `b0eedbe`; MG 8.1.3 Payload links + re-baseline `adc9bbe`; V1.04 references removed + re-baseline `1b779a7`; MG 8.3.1 MAM link + re-baseline `fa9ef19`; MG 8.3.x DataSetClassIds + re-baseline `43d5eec`; SF-096 Health order of priority + re-baseline `a3e4677` · MG 8.3.13 Query REST analogy + re-baseline `b6341d8` · MG 3.1 table alignment + re-baseline `eed0104` · MG 8.2.1 WriterGroupName removed + re-baseline `e8c149e` · MG 8.3.13 REST GET example restored + re-baseline `69bc021` · MG 6.3.5 Will DataSetWriterId + re-baseline `223fb51` · MG 9.1.x card/box layout + re-baseline `fa315eb`; MG 9.1.x outer cards removed + re-baseline `f4826e3`; MG 9.1.3 `<Context.Name>` + re-baseline `6c2be2c`; MG 9.1.4/9.1.5 `<Tag>` + re-baseline `0810d79`; MG 9.1.x `Status/<StatusCode>` + re-baseline `f99a769`; MG 9.1.3 `StatusCode` `Bad` + re-baseline `8406a6a`; same in 9.1.8/9.1.9/8.3.3.x + re-baseline `56bbcdb`; MG 9.1.5 headings + re-baseline `cd4b4db`; payload placeholders `<AppId>`/`<Source>` + re-baseline `5b8fe59`). Golden vectors of an area may be built in its area cycle already.
- Guideline ADRs (GL-ADR-001 … 018) and their implementation notes are part of the baseline (G-1.1). The OEC Process Description (GL-ADR-015) is informative and not catalogued.
- **Exit:** catalog + matrix + golden vectors reviewed by the PO; all blocking spec findings have a decided interpretation (at `5b8fe59` SF-001 … 096 are resolved, no open finding – SF-056 and SF-062 … 068 with GL-ADR-019, SF-069 and SF-073 with GL-ADR-020 in C-027, SF-075 … 081 in the C-028 review, SF-082/083 editorial, SF-084 … 088 (Config) in the C-029 review with GL-ADR-021; GL-ADR-022 (secret storage `/run/secrets/oi4`, PO order 2026-10-22) resolves SF-091 (d), SF-089/090/092/093 in the C-030 closure, SF-091 and the editorial SF-094/095 in C-032 – and the guideline carries keywords for protocol behavior (SF-035); areas CMP, IL, TOP, the topic columns of the matrix and the 203 topic/IL vectors are reviewed since C-022; MSG and ACT are reviewed since C-024, MQTT and LIFE since C-026; RES-MAM, RES-HEALTH, RES-PROFILE, RES-PUBLIST, RES-REFDES, PAG and LOC drafted in C-027, re-baselined after the guideline correction and reviewed 2026-10-20 together with the payload vectors V-PAY-026 … 052; GL-ADR-020 (children of a ReferenceDesignation via the generic `Query`) added the area QRY; MSG, RES-REFDES and QRY reviewed together with V-PAY-053/054 on 2026-10-20; RES-DATA, RES-META, RES-EVENT, RES-SUBLIST, BUS, the payload columns of the matrix (except Config) and V-PAY-055 … 087 reviewed on 2026-10-21, then re-baselined on `b2f0fe7` after SF-075 … 081 on `c2be26b` after SF-082 and on `5bc2e85` after SF-083; RES-CONFIG, the payload columns of the Config rows and V-PAY-088 … 113 drafted in C-029 and reviewed 2026-10-21, then re-baselined on `a77d331` after SF-084 … 088 and GL-ADR-021; the follow-ups REQ-RES-CONFIG-024 … 026, the Pub/Set Config rows and V-PAY-114 … 116 reviewed as well; the RIG areas CNT, PLT, APP, LIC, REG and GEN drafted in C-030 and reviewed 2026-10-22, re-baselined on `b942df3` after SF-089/090/092/093; follow-up drafts REQ-CNT-019/020/021, REQ-REG-002/007 and REQ-MQTT-007 (SF-091 (g)) for the PO; C-031: every fenced code block of MG/RIG is covered by a golden vector or explained (`coverage/examples.yaml`), V-PAY-117 … 126 and REQ-RES-DATA-001/RES-META-002/003 drafted for the PO, SF-094/095 editorial; `schema`/`d7-validate` REQs without a vector are covered by the P2 exit). **Met 2026-10-22 (C-032):** PO review of C-031 and the C-030 follow-ups, SF-091/094/095 implemented, re-baseline on `5fb0fd8`, all 434 REQs and 329 vectors `reviewed`, the 21 REQs without a vector moved to the P2 exit (PO decision).

## P2 – JSON Schema (D1) – started C-033

- **Architecture:** ADR-0019 (`proposed` in C-033). JSON Schema 2020-12 with modular files: OPC UA base types, envelopes, MetaData, Actions, helpers, one module per resource. Discrimination via `if`/`then`/`else` on `MessageType` → `DataSetClassId` → `DataSetWriterId`. Strict flavor (closures `unevaluatedProperties` with a REQ comment) as the source; a tolerant flavor (unknown-key tolerance) is generated. REQ ids in `$comment`.
- **Topic-aware validation** (ADR-0019 §5, recommendation option B): one entry schema per topic form (29 = ABNF rules = matrix rows), plus a topic-agnostic `message.schema.json`, plus a small rule layer in D7 `validate` (equality with topic values, IL decoding, cross-field arithmetic).
- **Coverage** (OPC UA 1.05.04 JSON with the fixed content masks of MG 8.1.4):
  - NetworkMessage (`ua-data`, `ReplyTo`);
  - DataSetMessage (`DataSetWriterName`, `WriterGroupName`, `Status` object, `MetaDataVersion` xor `MinorVersion`, `MessageType`);
  - DataSetMetaData (`ua-metadata`, FieldMetaData, StructureDescription, EnumDescription, …);
  - Actions (`ua-action-request`/`ua-action-response`, ActionRequest/ActionResponse/ActionMetaData/ActionState);
  - all resources: MAM, Health incl. `Details[]`, Config Pub/Set, Data/Oi4Pv, Metadata, Event (Status/Syslog/Generic), Profile, PublicationList, SubscriptionList, ReferenceDesignation;
  - the helpers Pagination, PaginationRequest, Locale and Query;
  - the only common-service Action `NewDataSetWriterId` (GL-ADR-012);
  - `broker.json` (for D8);
  - specific rules: NodeIds as JSON strings (`i=`/`nsu=`, no `ns=`, GL-ADR-013), the `LocaleId` regex, `DataSetWriterId` ≠ 0, helper DataSets (reserved DataSetWriterIds 1–3 and 9) with their `WriterGroupName`.
- **Test suite:** the reviewed vectors of `oec/spec/vectors` are the input; nothing is copied. Each vector is routed by topic to its entry schema. An expectation list (`oec/schema/tests/rule-layer.yaml`) names the invalid vectors that only the rule layer detects. Probes check schema paths during an increment. Two independent 2020-12 validators must agree on every case: Go `santhosh-tekuri/jsonschema` (also inside D7) and Rust `jsonschema`. The local CI is `oec/schema/check.sh`, next to the `spectool` checks.
- **First increment of D7:** the offline `validate` command (topic + payload from file, stdin or directory), see P5.
- **Increments and cycle cut** (C-033; ordered by dependency, each cycle ends with a PO review of its drafted vectors, reclassifications and ADR/SF decisions):
  - **C-033 – plan + ADR-0019 + PoC** (done 2026-10-23): ADR-0019 `proposed`; PoC of the envelope, helpers and Health (`pub-health`, `get-health`) against 16 vectors and 17 probes with three validators (after an independent review of the ADR and PoC); SF-097. The full build-out waits for the acceptance of ADR-0019.
  - **C-034 – foundation + envelope:**
    - PO decisions on ADR-0019 (option, `$id` base, umbrella rule) and SF-097;
    - layout per ADR-0019;
    - expectation list;
    - `spectool coverage` per-REQ report: positive/negative vector per `schema`/`d7-validate` REQ, layer, entry schema per allowed matrix row;
    - lint check "every allowed matrix row has an entry schema" (once all entry schemas exist);
    - modules `opcua`, `envelope` (`ua-data`, `ua-metadata`), `metadata`, `helpers` (PAG, LOC, QRY);
    - areas MSG, LIFE, BUS, PAG, LOC, QRY;
    - negative vectors for them;
    - gap REQs REQ-MSG-004/017/042/043/045 and REQ-PAG-009;
    - the 8 bare MetaData vectors.
  - **C-035 – Actions + core resources:**
    - `actions`, NewDataSetWriterId (`call-`/`reply-newdatasetwriterid`);
    - MAM, Health, Profile, PublicationList and ReferenceDesignation with their 13 topic forms;
    - areas ACT, RES-MAM, RES-HEALTH, RES-PROFILE, RES-PUBLIST, RES-REFDES;
    - gap REQs REQ-ACT-013, REQ-RES-HEALTH-007, REQ-RES-MAM-006/014;
    - oracle extensions (e.g. REQ-RES-HEALTH-007).
  - **C-036 – Data, Metadata, Event, SubscriptionList:**
    - Data/Oi4Pv, Metadata (deep data type descriptions, SF-050), Event (Status/Syslog/Generic), SubscriptionList, with their 11 topic forms;
    - areas RES-DATA, RES-META, RES-EVENT, RES-SUBLIST;
    - gap REQ REQ-RES-DATA-003.
  - **C-037 – Config + `broker.json`:**
    - Config Pub/Get/Set (groups, configuration objects, Validation, masking);
    - area RES-CONFIG with gap REQ REQ-RES-CONFIG-015;
    - `platform/broker.schema.json` (REQ-CNT-003, verification `d8` unchanged) with RIG 4.1.1 vectors.
  - **C-038 – D7 `validate`:**
    - `oec/conformance` Go module skeleton (ADR-0013) with the `validate` command: topic grammar + IL + embedded entry schemas + rule layer R-1 … R-4, report with REQ ids, JSON output, exit codes;
    - every topic and payload vector passes as expected;
    - areas TOP, IL, CMP (rule-layer parts) with gap REQs REQ-IL-005 and REQ-TOP-018;
    - umbrella REQs REQ-CMP-004/005/025/026, REQ-TOP-002 and REQ-IL-001/008 (covered through their details, if the PO accepts ADR-0019 §9).
  - **C-039 – P2 exit review:**
    - consistency review;
    - `coverage -check` as gate;
    - `CHANGELOG` 2.0.0 of the schemas;
    - `$id`s frozen;
    - PO review → P2 exit.
  - The agent may split a cycle that grows too large (allowed by the PO), with the reason in the log. P2 sizing at `5b8fe59`: 159 `schema`/`d7-validate` REQs; 129 lack a positive and/or a negative vector (93 positive only, 15 negative only, 21 none); 29 topic forms with 126 payload vectors.
- **Exit:**
  1. ADR-0019 accepted.
  2. Every non-deprecated REQ with `schema` and/or `d7-validate` (159 at `5b8fe59`, including the 21 of the C-031 gap list) has at least one reviewed positive and one reviewed negative vector. Umbrella REQs are covered through their detail REQs, if the PO accepts ADR-0019 §9. `spectool coverage -check` gates this.
  3. Every allowed matrix row has an entry schema, and the bare MetaData and `broker.json` have schemas. All valid vectors pass. Every invalid vector is rejected by its entry schema with exactly the REQs of `violates`, or the expectation list partitions its `violates` into schema REQs (reported exactly) and rule-layer REQs, which D7 `validate` then reports.
  4. Both validators agree on every case.
  5. The oracle (`oec/spec/tools/internal/payload`) checks every rule that a schema enforces, and `spectool lint` is green.
  6. D7 `validate` (first increment) runs on Windows, macOS and Linux and reports REQ ids.
  7. Schemas are released with stable `$id`s (`2.0.0`, `$id` base decided). Hosting at the `$id` URLs follows the publication decision (Q-011).

## P3 – SDK and conformance tool design (D2, D7, D8, D9) – decision phase

Explicitly a discussion phase with the product owner (Q-001, Q-002, Q-008). Options: `research/sdk-architecture-comparison.md`, `research/sdk-options.md`. **Decided:** S4 Rust SDK + C API (ADR-0011), local monorepo `oec/` (ADR-0012). Remaining in P3: SDK API sketch (Rust + C), crate choices. D7 + D8 in Go (ADR-0013, ADR-0014). v2 only (ADR-0015). Level targets + micro SDK (ADR-0016); Micro SDK in C17 on ESP32/ESP-IDF (ADR-0017); its C API sketch is part of P3.

- Options paper: language(s) (TypeScript/Node, Go, Python, C#, ...), "SDK vs. sidecar/runtime vs. code generation", packaging, repo layout.
- Developer-experience sketches: "hello world" OEC app in ≤ 20 lines; how to add a device/asset source, publish data, react to `Set`, expose config.
- API design for: compliance level selection (L1 smart device without container … L3), identity (IL/Oi4Identifier), container storage bootstrap (optional), connection & lifecycle, resource registry (built-in MAM/Health/Profile/PublicationList/...), data publishing with PublicationList semantics (Mode/Interval/Precisions), `Get` handling + pagination, `Set`/`Del` with automatic Status events, Config resource, Event/Syslog logging, consumer side (typed subscriptions, request/response helpers).
- Conformance tool: CLI shape (commands, options, exit codes, report formats), test catalog structure (test case ↔ REQ ids), independence from the SDK (Q-009), distribution for Windows/macOS.
- **Exit:** accepted ADRs for language, repo layout, public SDK API sketch and conformance tool architecture.

## P4 – SDK core implementation (D2)

Incremental, each increment test-covered:

1. Identity: IL validation, base64url codec, Oi4Identifier builder (optional dnp-encoding of its elements, MG 4.3.1/A3).
2. Topic model: build/parse/validate, wildcard subscription builder.
3. Message model: typed NetworkMessage/DataSetMessage/MetaData/Action messages (OPC UA 1.05.04, fixed content masks), MessageId/`ReplyTo`/Action correlation (`RequestId`, `CorrelationData`, `ResponseAddress`), schema validation hook.
4. Bootstrap: explicit configuration (smart devices, Level 1) and optional container storage contract (`broker.json`, credentials/certs, ClientId from hostname).
5. Connection & lifecycle: TLS MQTT client, Birth/Will/Close, Health heartbeat incl. `Details[]`, reconnect; selectable compliance level (L1/L2/L3).
6. Provider side: resources registry, automatic answers to `Get`, `Profile`/`PublicationList` generation, publish modes, intervals/precisions, pagination, `MaxPacketSize`.
7. Write side: `Set`/`Del` handling with Status events, Config resource with validation.
8. Consumer side: typed subscriptions, discovery helpers, `Get` request with correlation + timeout, pagination reassembly.
9. Dev environment: docker compose with TLS broker (e.g. Mosquitto), generated dev PKI, sample `/etc/oi4` layout.
- **Exit:** conformity test vectors pass; integration tests against broker pass (incl. D7 `test` against an SDK example app, as soon as available); example app documented.

## P4b – Micro SDK (D9)

SDK for microcontroller-class smart devices without container (ADR-0016), written in **C17** with a platform abstraction layer; reference port ESP-IDF on **ESP32** (ADR-0017).

- Portable core (IL/base64url, topics incl. percent-encoding, JSON encoding of NetworkMessage/MAM/Health, lifecycle state machine) + PAL (MQTT 3.1.1/TLS, clock, persistence); ESP-IDF port with `esp-mqtt` + `esp-tls`/mbedTLS; afterwards (best effort) **Arduino library** packaging with an Arduino-ESP32 PAL port (ADR-0017 amendment C-010).
- C safety measures: static analysis, ASan/UBSan in host tests, fuzzing of topic/JSON parsers.
- Tests: host unit tests (CMake, Unity) against the shared D1 test vectors on macOS/Windows/Linux; integration in Espressif QEMU (`esp32`) against the dev broker with D7 (to be confirmed); final check on a real ESP32 board.
- Scope: designed for Messaging **L3** (all messages, ADR-0016 amendment C-016 / Q-019), delivered incrementally: first **L1** (Birth/Will, Pub MAM + Health, TLS + auth, topic and payload format), then L2 (Get, Set/Del where applicable) and L3 (all resources incl. `SubscriptionList`, `Call`/`Reply` + `NewDataSetWriterId`, empty-list behavior) in later increments.
- Explicit configuration (no `/etc/oi4`), static memory where possible, documented RAM/flash footprint.
- Example firmware for the reference board.
- Targets: ESP-IDF `esp32` + `esp32s3` supported, C3/C6 best effort; reference hardware: ESP32 dev board + M5Stack Core2/CoreS3. Example firmware generic + M5Stack variant (display/buttons via `M5Unified`, example only) (ADR-0017 amendment C-011).
- **Exit:** example firmware on the ESP32 reference board and on the M5Stack device (and in QEMU, if feasible) passes D7 `--level 1` against the dev broker (first increment); later increments pass `--level 2` and `--level 3`.

## P5 – Conformance test tool (D7)

Command-line tool to check the **OEC Messaging Compliance (Level 1/2/3)** of applications and smart devices against the guideline (no platform/broker/container checks – those are D8, ADR-0006/0014). Shares only the D1 JSON Schemas and the spec baseline `oec/spec/` (ADR-0018) with the SDK, no code. Runs on **Windows and macOS** (and Linux for CI) and doubles as the **integration test harness** for SDK, demos and registry.

- `validate` – offline: topic + payload (single file, stdin, directory or recorded capture) against D1 + topic grammar + cross-field rules (e.g. topic `<Source>` = payload `DataSetWriterName`, `<Filter>` = `WriterGroupName`, namespace `Oi4v2`). Delivered with P2.
- `monitor` – passive: connect to a broker, observe all `Oi4v2/#` traffic, validate every message live, summarize per participant.
- `test` – active: test one application under test (by AppId/IL or discovery), e.g.
  - lifecycle: Birth (MAM), Health on start + every 60 s, Close/Will behavior;
  - mandatory resources/methods per level (MG 1.3) answer `Get` correctly, with `ReplyTo`;
  - Profile consistency (announced resources actually answer), PublicationList consistency;
  - Data ⇒ Metadata; `Set`/`Del` ⇒ Status event with `ReplyTo`; `NewDataSetWriterId` Action (`Call`/`Reply`) correlation;
  - pagination/`MaxPacketSize`, Locale;
  - negative cases (invalid requests are rejected/ignored as specified).
- Test catalog: every test case references REQ ids and guideline sections; selectable by tag and by compliance profile `level1` / `level2` / `level3` (declared level passed as CLI parameter and includes lower levels, MG 1.3).
- Connection: CLI flags/config file and optionally the container storage contract (`/etc/oi4/mqtt`, …); TLS + user/password or client certificate; explicit `--insecure` dev mode only.
- Output: human-readable console report, machine-readable JSON and JUnit XML; meaningful exit codes for CI.
- Distribution: single self-contained executable per OS/arch (Windows x64/arm64, macOS arm64/x64, Linux) and container image.
- **Exit:** `test` passes against the SDK example app and fails on purpose-built faulty fixtures; runs in CI on Windows, macOS and Linux runners.

## P5b – Infrastructure compliance tool (D8)

Command-line tool (Go, ADR-0014) that checks the **Service Container Compliance (Level 1/2/3)** of application containers and, as far as testable, the **platform requirements** of the RIG (scope confirmed, Q-017). Same conventions as D7: Windows/macOS/Linux, test cases reference REQ ids, profiles `level1..3`, console + JSON + JUnit output, exit codes.

- `inspect-image` – offline: analyze an OCI image (registry reference or archive): OCI format, exactly one canonical SBOM (`sbom.spdx.json` / `sbom.cdx.json`) + license texts (`<License-ID>.txt` or embedded) in `/opt/oi4/licenses` (RIG 4.1.4), storage paths, user, labels, digest/integrity.
- `inspect-container` – a running app container: storage mounts (`/etc/oi4/...`, `/run/secrets/...`), hostname = container name rules, networks.
- `inspect-platform` – platform host/broker as far as testable (TLS-only broker, `broker.json` provisioning, registry ports 5798/5799).
- **Exit:** our demo and registry images pass `level3`; purpose-built faulty images fail.

## P6 – Demo services (D3, D4)

- **Weather service** (`OTConnector`): weather stations as assets (each with its own IL + MAM), `Data` (Oi4Pv: e.g. temperature as `Pv`, humidity/pressure/wind as `Sv<n>`) + `Metadata` with units, `Config` for locations and polling interval, `Health` reflecting API reachability. Data source via provider interface: default MET Norway (keyless, CC BY 4.0, honor User-Agent/caching/attribution rules) + offline simulation provider (ADR-0010, G-4.7).
- **Logger service** (`Utility`): subscribes to selectable topics/resources (default: `Event`, `Health`, optionally `Data`), logs structured output (stdout/JSON, optional file), configurable via the `Config` resource, shows the consumer side of the SDK.
- Both: Dockerfile (OCI, SBOM + license texts in `/opt/oi4/licenses`), compose integration, README. Service Container Level 3 (ADR-0016).
- **Compliance targets (ADR-0016):** weather = Messaging **L3** (full provider showcase); logger = Messaging **L1** (minimal participant; configured via file/environment instead of the `Config` resource).
- **Exit:** both demos run in the dev environment and pass D7 (`level3` resp. `level1` profile) and D8 (`level3`).

## P7 – OEC Registry (D5)

Step 1 is **read-only** (ADR-0008): no `Set`/`Del`/`Call` towards other participants.


- Discovery: listens to `Pub/MAM`, `Pub/Health`, `Pub/Profile`, `Pub/Event` (+ others as decided); actively queries new participants after Birth (`Get/MAM`, `Get/Profile`, `Get/PublicationList`, ...); handles Will/Close (availability: Close = offline graceful, Will = offline ungraceful, Birth = online, MG 6.3.4/6.3.5).
- Provides own Registry services per MG A1 (`Oi4v2/Registry/<AppId>/Get/MAM[/<Source>]`, `Get/Health[/<Source>]`).
- Model: applications vs. devices (assets) incl. "published by" relation, supported resources/methods, **observed (informative) Messaging compliance level** derived from `Profile` + `Get` answers (MG 1.3, A1), health incl. `Details[]` and history, last seen, event/audit trail; v2 only, v1 traffic ignored (ADR-0015). Target: Messaging + Service Container L3 (ADR-0016).
- Conformity view: validation of observed traffic per participant (reuses D1 and the D7 rule engine where sensible).
- Web UI (modern stack, decided in an ADR) + documented API (REST/OpenAPI and/or live updates via WebSocket/SSE); default ports 5798/5799 (A1).
- **Exit:** registry lists all dev-environment participants with capabilities, health and conformity status; registry passes D7 and D8 at `level3`.

## P8 – Documentation (D6) – continuous

- New standalone Docusaurus site in `oec/docs` (ADR-0012), no links to the guideline site.
- Sections: Getting started (dev environment in 10 minutes), Concepts (short; guideline referenced by section number, no links – G-4.11) incl. compliance levels, SDK guide + API reference, micro SDK guide, compliance tool guides (D7, D8), JSON Schema reference (generated where possible) + validation how-to, Demo walkthroughs, Registry guide, Migration notes 1.x → 2.0 (breaking changes GL-ADR-001 … 011: Identification Link, `Oi4v2`, `DataSetWriterName`/`WriterGroupName`, `ReplyTo`, OPC UA 1.05.04, Actions, removed resources, SBOM, compliance levels).
- Each phase delivers its doc pages in the same phase.
- **Exit:** docs build without broken links; a newcomer can run the demos by following the docs only.

## P9 – Hardening & release

- CI pipelines (build, test, schema vectors, lint, image build multi-arch, D7/D8 on Windows/macOS/Linux runners, micro SDK host tests + ESP-IDF builds for `esp32` + `esp32s3`), SBOM/license report, security scan.
- Versioned release of schema, SDK packages, conformance tool binaries, container images; release notes.
- Publication target: private GitHub repositories (working assumption, Q-011). CRA review before any *public* release (Q-011): SBOM, vulnerability handling process, security update policy, support period.
- Guideline feedback package: all spec findings with proposed text changes for the work group.

---

## Cross-cutting tracks

- **Guideline evolution:** every cycle checks whether `oi4-oec-documentation` changed; if so, diff against the recorded baseline and update REQs/schemas.
- **Spec feedback:** the spec-findings register `oi4-oec-documentation/findings/SPEC-FINDINGS.md` is curated continuously and periodically handed to the work group.
