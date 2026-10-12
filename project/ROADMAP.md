# Roadmap – OI4 OEC v2.0

The roadmap is iterative: phases are ordered by dependency, each ends with explicit exit criteria confirmed by the product owner. Phases may be re-planned when the guideline changes. Detailed planning of a phase happens in its first cycle.

```
P0 Onboarding ──► P1 Spec baseline ──► P2 JSON Schema ─────────────┬─► P5 Conformance tool ──┐
                                   └─► P3 SDK + tool design ──────┴─► P4 SDK core ───────────┴─► P6 Demos ──► P7 Registry ──► P9 Release
                         P8 Documentation (starts with P2, grows with every phase) ───────────────────────────────────────────┘

P2 already ships the offline `validate` command of the conformance tool (D7).
P4 and P5 can run in parallel once P3 is decided; demos (P6) and registry (P7) are verified with D7.
```

---

## P0 – Onboarding & governance ✅ (C-001, C-002)

- Study guideline v2.0 draft, RFC MS-010, legacy code.
- Create guardrails, roadmap, logging structure, research digests.
- **Exit:** product owner accepts GUARDRAILS + ROADMAP.

## P1 – Spec baseline (requirements catalog)

Turn the prose guideline into a precise, testable baseline – the foundation for schema, SDK and conformity checks.

- Requirements catalog `REQ-<area>-NNN` (areas: IL, TOP, MQTT, CNT, MSG, RES-<resource>, SVC, LIFE, PAG) with guideline reference, normative level (MUST/SHOULD/MAY) and testability (`schema` = D1, `conformance` = D7 live test, `manual`).
- Topic grammar as formal definition (ABNF + regex) per Method × Resource combination (which `Source`/`Filter` levels are required/allowed/forbidden).
- Method × Resource matrix (Pub/Get/Set/Del/Call/Reply) incl. payload expectations (e.g. `Get` = empty `Messages` or only Pagination/Locale).
- Review and extend `SPEC-FINDINGS.md`; propose interpretations; get decisions from product owner / work group.
- Golden examples: all Appendix B examples, corrected where needed, as the first test vectors.
- **Exit:** catalog + matrix reviewed; all blocking spec findings have a decided interpretation.

## P2 – JSON Schema (D1)

- Decide schema architecture (ADR): JSON Schema 2020-12; modular `$defs` (OPC UA base types, envelopes, OI4 resources, services); discriminating by `DataSetClassId`/`MessageType`; strict `unevaluatedProperties: false` for OI4-defined objects.
- **Topic-aware validation:** payload rules depend on the topic (e.g. `Get` vs `Pub`, `Set` needs single message). Define a "message = topic + payload" validation model (schema + small rule layer, or schema per topic pattern) – decided in ADR.
- Coverage: NetworkMessage, DataSetMessage, DataSetMetaData (+ FieldMetaData, StructureDescription, EnumDescription, ...), ServiceNetworkMessage, all resources (MAM, Health, Config Pub/Set, Data/Oi4Pv, Metadata, Event + categories, Profile, PublicationList, SubscriptionList, ReferenceDesignation, Pagination, PaginationRequest, Locale), common services (NewDataSetWriterId; others as far as specified), `broker.json`.
- Test suite: valid/invalid vectors per rule (language-neutral JSON files), CI-run with at least two independent 2020-12 validators.
- First increment of D7: offline `validate` command (topic + payload from file/stdin/directory), see P5.
- **Exit:** every REQ marked "schema-testable" is covered by a positive and negative vector; schemas published with stable `$id`s.

## P3 – SDK and conformance tool design (D2, D7) – decision phase

Explicitly a discussion phase with the product owner (Q-001, Q-002, Q-008). Options: `research/sdk-architecture-comparison.md`, `research/sdk-options.md`. **Decided:** S4 Rust SDK + C API (ADR-0011), local monorepo `oec/` (ADR-0012). Remaining in P3: SDK API sketch (Rust + C), crate choices. D7 in Go (ADR-0013).

- Options paper: language(s) (TypeScript/Node, Go, Python, C#, ...), "SDK vs. sidecar/runtime vs. code generation", packaging, repo layout.
- Developer-experience sketches: "hello world" OEC app in ≤ 20 lines; how to add a device/asset source, publish data, react to `Set`, expose config.
- API design for: identity (IL/Oi4Identifier), container storage bootstrap, connection & lifecycle, resource registry (built-in MAM/Health/Profile/PublicationList/...), data publishing with PublicationList semantics (Mode/Interval/Precisions), `Get` handling + pagination, `Set`/`Del` with automatic Status events, Config resource, Event/Syslog logging, consumer side (typed subscriptions, request/response helpers).
- Conformance tool: CLI shape (commands, options, exit codes, report formats), test catalog structure (test case ↔ REQ ids), independence from the SDK (Q-009), distribution for Windows/macOS.
- **Exit:** accepted ADRs for language, repo layout, public SDK API sketch and conformance tool architecture.

## P4 – SDK core implementation (D2)

Incremental, each increment test-covered:

1. Identity: IL validation, base64url codec, Oi4Identifier + dnp-encoding.
2. Topic model: build/parse/validate, wildcard subscription builder.
3. Message model: typed NetworkMessage/DataSetMessage/MetaData/MSG, MessageId/CorrelationId handling, schema validation hook.
4. Bootstrap: container storage contract, `broker.json`, credentials/certs, ClientId from hostname.
5. Connection & lifecycle: TLS MQTT client, Birth/Will/Close, Health heartbeat, reconnect.
6. Provider side: resources registry, automatic answers to `Get`, `Profile`/`PublicationList` generation, publish modes, intervals/precisions, pagination, `MaxPacketSize`.
7. Write side: `Set`/`Del` handling with Status events, Config resource with validation.
8. Consumer side: typed subscriptions, discovery helpers, `Get` request with correlation + timeout, pagination reassembly.
9. Dev environment: docker compose with TLS broker (e.g. Mosquitto), generated dev PKI, sample `/etc/oi4` layout.
- **Exit:** conformity test vectors pass; integration tests against broker pass (incl. D7 `test` against an SDK example app, as soon as available); example app documented.

## P5 – Conformance test tool (D7)

Command-line tool to check OEC **applications** against the guideline (no platform/broker checks, ADR-0006). Shares only the D1 JSON Schemas with the SDK, no code. Runs on **Windows and macOS** (and Linux for CI) and doubles as the **integration test harness** for SDK, demos and registry.

- `validate` – offline: topic + payload (single file, stdin, directory or recorded capture) against D1 + topic grammar + cross-field rules (e.g. topic `Source` = payload `Source`). Delivered with P2.
- `monitor` – passive: connect to a broker, observe all `Oi4/#` traffic, validate every message live, summarize per participant.
- `test` – active: test one application under test (by AppId/IL or discovery), e.g.
  - lifecycle: Birth (MAM), Health on start + every 60 s, Close/Will behavior;
  - mandatory resources/methods (8.2) answer `Get` correctly, with `CorrelationId`;
  - Profile consistency (announced resources actually answer), PublicationList consistency;
  - Data ⇒ Metadata; `Set`/`Del` ⇒ Status event with `CorrelationId`;
  - pagination/`MaxPacketSize`, Locale;
  - negative cases (invalid requests are rejected/ignored as specified).
- Test catalog: every test case references REQ ids and guideline sections; selectable by tag/profile (e.g. `minimum`, `full`).
- Connection: CLI flags/config file and optionally the container storage contract (`/etc/oi4/mqtt`, …); TLS + user/password or client certificate; explicit `--insecure` dev mode only.
- Output: human-readable console report, machine-readable JSON and JUnit XML; meaningful exit codes for CI.
- Distribution: single self-contained executable per OS/arch (Windows x64/arm64, macOS arm64/x64, Linux) and container image.
- **Exit:** `test` passes against the SDK example app and fails on purpose-built faulty fixtures; runs in CI on Windows, macOS and Linux runners.

## P6 – Demo services (D3, D4)

- **Weather service** (`OTConnector`): weather stations as assets (each with its own IL + MAM), `Data` (Oi4Pv: e.g. temperature as `Pv`, humidity/pressure/wind as `Sv<n>`) + `Metadata` with units, `Config` for locations and polling interval, `Health` reflecting API reachability. Data source via provider interface: default MET Norway (keyless, CC BY 4.0, honor User-Agent/caching/attribution rules) + offline simulation provider (ADR-0010, G-4.7).
- **Logger service** (`Utility`): subscribes to selectable topics/resources (default: `Event`, `Health`, optionally `Data`), logs structured output (stdout/JSON, optional file), configurable via the `Config` resource, shows the consumer side of the SDK.
- Both: Dockerfile, compose integration, README.
- **Exit:** both demos run in the dev environment and pass the D7 `test` suite.

## P7 – OEC Registry (D5)

Step 1 is **read-only** (ADR-0008): no `Set`/`Del`/`Call` towards other participants.


- Discovery: listens to `Pub/MAM`, `Pub/Health`, `Pub/Profile`, `Pub/Event` (+ others as decided); actively queries new participants after Birth (`Get/MAM`, `Get/Profile`, `Get/PublicationList`, ...); handles Will/Close (availability).
- Provides own Registry services per Appendix A1 (`Get/MAM[/<Source>]`, `Get/Health[/<Source>]`).
- Model: applications vs. devices (assets) incl. "published by" relation, supported resources/methods, last seen, health history, event/audit trail.
- Conformity view: validation of observed traffic per participant (reuses D1 and the D7 rule engine where sensible).
- Web UI (modern stack, decided in an ADR) + documented API (REST/OpenAPI and/or live updates via WebSocket/SSE); default ports 5798/5799 (A1).
- **Exit:** registry lists all dev-environment participants with capabilities, health and conformity status; registry passes the D7 `test` suite.

## P8 – Documentation (D6) – continuous

- New standalone Docusaurus site in `oec/docs` (ADR-0012), no links to the guideline site.
- Sections: Getting started (dev environment in 10 minutes), Concepts (short, links to guideline), SDK guide + API reference, conformance tool guide, JSON Schema reference (generated where possible) + validation how-to, Demo walkthroughs, Registry guide, Migration notes 1.x → 2.0 (breaking changes like MS-010).
- Each phase delivers its doc pages in the same phase.
- **Exit:** docs build without broken links; a newcomer can run the demos by following the docs only.

## P9 – Hardening & release

- CI pipelines (build, test, schema vectors, lint, image build multi-arch, D7 on Windows/macOS/Linux runners), SBOM/license report, security scan.
- Versioned release of schema, SDK packages, conformance tool binaries, container images; release notes.
- CRA review before any publication (Q-011): SBOM, vulnerability handling process, security update policy, support period.
- Guideline feedback package: all spec findings with proposed text changes for the work group.

---

## Cross-cutting tracks

- **Guideline evolution:** every cycle checks whether `oi4-oec-documentation` changed; if so, diff against the recorded baseline and update REQs/schemas.
- **Spec feedback:** `SPEC-FINDINGS.md` is curated continuously and periodically handed to the work group.
