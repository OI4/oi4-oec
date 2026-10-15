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

## P1 – Spec baseline (requirements catalog)

Turn the prose guideline into a precise, testable baseline – the foundation for schema, SDK and conformity checks.

- Requirements catalog `REQ-<area>-NNN` as YAML in `oec/spec/` (ADR-0018; areas: CMP, IL, TOP, MSG, ACT, MQTT, LIFE, RES-<resource>, PAG, LOC, BUS, CNT, LIC, APP, PLT, REG, GEN – section assignment in `oec/spec/requirements/areas.yaml`) covering **both** guideline documents (MG + RIG) with guideline reference, normative keyword per MG 1.2 (MUST/SHOULD/MAY and negations, G-1.6), **compliance domain (Messaging / Service Container) and level (L1/L2/L3)** and testability (`schema` = D1, `D7` = messaging live test, `D8` = container check, `manual`). Plan: C-012 log, updated by C-013 (no specific services after GL-ADR-014).
- Topic grammar as formal definition (ABNF + regex) per Method × Resource combination (which `Source`/`Filter` levels are required/allowed/forbidden), incl. percent-encoding of `+ # / %` within a `Filter` level (MG 7.7) and Status event levels `Good|Uncertain|Bad`.
- Method × Resource matrix (Pub/Get/Set/Del/Call/Reply) incl. payload expectations (e.g. `Get` = empty `Messages` or only Pagination/Locale).
- Review and extend `SPEC-FINDINGS.md`; propose interpretations; get decisions from product owner / work group.
- Golden examples: all Appendix B examples, checked against the guideline description and built as compliant, meaningful test data (ADR-0018 amendment C-013); deviations become spec findings; the PO reviews the test data per increment (G-3.9).
- **Cycle cut (PO-confirmed, C-013), "Level 1 first":** C-014 `oec/spec/` skeleton + catalog schema + tools + normative-sentence inventory + `CMP` · C-015 guideline update SF-036 … 038 (PO request) + re-baseline `5cb109e` · C-016 PO decisions (Status event levels, SF-039, Q-019) + SF-035 keyword pass + re-baseline `de852d6` · C-017 GL-ADR-017 (MG 7.2 removed, MG 7 flattened, PO request) + re-baseline `4f5c548` · C-018 `IL`, `TOP`, topic grammar + topic vectors · C-019 `MSG` + `ACT` · C-020 `MQTT` + `LIFE` · C-021 core resources (MAM, Health, Profile, PublicationList, ReferenceDesignation) + `PAG` + `LOC` · C-022 remaining resources + `BUS` + Method × Resource matrix · C-023 RIG areas `CNT`, `LIC`, `APP`, `PLT` + `REG` + `GEN` · C-024 golden vectors + coverage report · C-025 PO review → P1 exit. Golden vectors of an area may be built in its area cycle already.
- Guideline ADRs (GL-ADR-001 … 016) and their implementation notes are part of the baseline (G-1.1). The OEC Process Description (GL-ADR-015) is informative and not catalogued.
- **Exit:** catalog + matrix + golden vectors reviewed by the PO; all blocking spec findings have a decided interpretation (at `4f5c548` SF-001 … 039 are resolved and the guideline carries keywords for protocol behavior (SF-035); no open findings – new findings go to SF-040 ff.).

## P2 – JSON Schema (D1)

- Decide schema architecture (ADR): JSON Schema 2020-12; modular `$defs` (OPC UA base types, envelopes, OI4 resources, services); discriminating by `DataSetClassId`/`MessageType`; strict `unevaluatedProperties: false` for OI4-defined objects.
- **Topic-aware validation:** payload rules depend on the topic (e.g. `Get` vs `Pub`, `Set` needs single message). Define a "message = topic + payload" validation model (schema + small rule layer, or schema per topic pattern) – decided in ADR.
- Coverage (OPC UA 1.05.04 JSON with the fixed content masks of MG 8.1.4): NetworkMessage (`ua-data`, `ReplyTo`), DataSetMessage (`DataSetWriterName`, `WriterGroupName`, `Status` object, `MetaDataVersion` xor `MinorVersion`, `MessageType`), DataSetMetaData (`ua-metadata`, + FieldMetaData, StructureDescription, EnumDescription, ...), Actions (`ua-action-request`/`ua-action-response`, ActionRequest/ActionResponse/ActionMetaData/ActionState), all resources (MAM, Health incl. `Details[]`, Config Pub/Set, Data/Oi4Pv, Metadata, Event (Status/Syslog/Generic), Profile, PublicationList, SubscriptionList, ReferenceDesignation, Pagination, PaginationRequest, Locale), the only common-service Action `NewDataSetWriterId` (GL-ADR-012), `broker.json`. NodeIds as JSON strings (`i=`/`nsu=`, no `ns=`, GL-ADR-013); `LocaleId` regex; `DataSetWriterId` ≠ 0; helper DataSets (reserved DSWIDs 1–3) with their `WriterGroupName`.
- Test suite: valid/invalid vectors per rule (language-neutral JSON files), CI-run with at least two independent 2020-12 validators.
- First increment of D7: offline `validate` command (topic + payload from file/stdin/directory), see P5.
- **Exit:** every REQ marked "schema-testable" is covered by a positive and negative vector; schemas published with stable `$id`s.

## P3 – SDK and conformance tool design (D2, D7, D8, D9) – decision phase

Explicitly a discussion phase with the product owner (Q-001, Q-002, Q-008). Options: `research/sdk-architecture-comparison.md`, `research/sdk-options.md`. **Decided:** S4 Rust SDK + C API (ADR-0011), local monorepo `oec/` (ADR-0012). Remaining in P3: SDK API sketch (Rust + C), crate choices. D7 + D8 in Go (ADR-0013, ADR-0014). v2 only (ADR-0015). Level targets + micro SDK (ADR-0016); Micro SDK in C17 on ESP32/ESP-IDF (ADR-0017); its C API sketch is part of P3.

- Options paper: language(s) (TypeScript/Node, Go, Python, C#, ...), "SDK vs. sidecar/runtime vs. code generation", packaging, repo layout.
- Developer-experience sketches: "hello world" OEC app in ≤ 20 lines; how to add a device/asset source, publish data, react to `Set`, expose config.
- API design for: compliance level selection (L1 smart device without container … L3), identity (IL/Oi4Identifier), container storage bootstrap (optional), connection & lifecycle, resource registry (built-in MAM/Health/Profile/PublicationList/...), data publishing with PublicationList semantics (Mode/Interval/Precisions), `Get` handling + pagination, `Set`/`Del` with automatic Status events, Config resource, Event/Syslog logging, consumer side (typed subscriptions, request/response helpers).
- Conformance tool: CLI shape (commands, options, exit codes, report formats), test catalog structure (test case ↔ REQ ids), independence from the SDK (Q-009), distribution for Windows/macOS.
- **Exit:** accepted ADRs for language, repo layout, public SDK API sketch and conformance tool architecture.

## P4 – SDK core implementation (D2)

Incremental, each increment test-covered:

1. Identity: IL validation, base64url codec, Oi4Identifier + dnp-encoding.
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
- **Spec feedback:** `SPEC-FINDINGS.md` is curated continuously and periodically handed to the work group.
