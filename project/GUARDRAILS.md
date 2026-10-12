# Guardrails – OI4 OEC v2.0

These rules apply to every human and agent working on this project. A deviation requires an accepted ADR or explicit approval of the product owner, recorded in the cycle log.

---

## G-1 Sources and authority

| ID | Rule |
|----|------|
| G-1.1 | **Source of truth** for the protocol is the v2.0 draft guideline in `oi4-oec-documentation/docs/guideline/` (ADR-0001). Accepted RFCs in `oi4-oec-documentation/rfcs_adrs/` (e.g. MS-010) are part of it. The Confluence export under `oi4-oec-documentation/guideline/` and the PDFs (v1.1.0) are **historical reference only**. |
| G-1.2 | The guideline is **not final**. Always record the guideline revision (git commit of `oi4-oec-documentation`) you worked against in the cycle log. Onboarding baseline: `c85ce2a` (2026-10-06). |
| G-1.3 | `legacy/` (oi4-oec-service, oi4-oec-service-go, oi4-oec-demo, oi4-oec-registry) is **history only**: read-only, never modified, **no code copied**. It may be consulted for ideas and lessons learned; if an idea is reused, name the source in the log. |
| G-1.4 | Do not edit guideline text unless the product owner explicitly asks for it. Problems in the guideline go to `SPEC-FINDINGS.md`. |
| G-1.5 | External standards are referenced in their **latest published version** (e.g. JSON Schema 2020-12 (ADR-0003), OPC UA 1.05 reference pages, RFC 4648, RFC 3986, IEC 61406-1). If the guideline references an older version (e.g. OPC UA v1.04), note the gap as a spec finding instead of silently deviating. |

## G-2 Protocol invariants (derived from the guideline – must hold in every artifact)

The section numbers refer to the guideline. If the guideline changes, update this list in the same cycle.

| ID | Invariant | Guideline |
|----|-----------|-----------|
| G-2.1 | Topic schema: `Oi4/<ServiceType>/<AppId>/<Method>/<Resource>[/<Source>[/<Filter>]]`. Namespace is exactly `Oi4`. | 8.1, 8.1.1 |
| G-2.2 | `AppId` and `Source` are the **base64url (RFC 4648 §5) encoded Identification Link, without padding**, forming exactly **one** topic level. There is exactly one valid encoding per IL. | 3.2, 8.1.3, 8.1.6 |
| G-2.3 | In payloads (`PublisherId`, `MessageId`, `CorrelationId`, `Source`, `ProductInstanceUri`, ...) the Identification Link is **always unencoded** and used exactly as assigned (no normalization, case-sensitive, char-by-char comparison). | 3.1, 3.2 |
| G-2.4 | Identification Link: ASCII subset of IEC 61406-1, max. 255 chars, lowercase authority, optional scheme (with/without scheme = different ILs), no `%`. The `Oi4Identifier` (`ManufacturerUri/dnp(Model)/dnp(ProductCode)/dnp(SerialNumber)`) is one optional way to build an IL. | 3.1, 3.3, A3 |
| G-2.5 | ServiceTypes: `Registry`, `OTConnector`, `Utility`, `Persistence`, `Aggregation`, `OOCConnector`, `ITConnector`. Methods: `Pub`, `Get`, `Set`, `Del`, `Call`, `Reply`. | 8.1.2, 8.1.4 |
| G-2.6 | Payloads are OPC UA PubSub JSON: `NetworkMessage` (`ua-data`), `DataSetMetaData` (`ua-metadata`), `ServiceNetworkMessage` (`MSG`). All OI4/OPC UA keys are **PascalCase**. OI4-defined objects **must not be extended** with user-defined keys. | 1, 9 |
| G-2.7 | `PublisherId` = `<ServiceType>/<IdentificationLink>`; `MessageId` = `<unixTimestampInMs>-<PublisherId>` and unique (add a counter if needed). `CorrelationId` = `MessageId` of the causing message. | 9.2.1 |
| G-2.8 | Predefined `DataSetClassId`s (Appendix A2) are used verbatim and are present on every OI4-defined resource message. | 9.2.1, A2 |
| G-2.9 | `DataSetWriterId` is UInt16, unique per publisher; 0–9 reserved (1 PaginationRequest, 2 Pagination, 3 Locale – see SF-001); applications start at 10. | 9.2.3 |
| G-2.10 | MQTT: v3.1.1, TLS ≥ 1.2 only (default port 8883, no plain 1883), auth via username/password or client certificate (cert wins), QoS 0 default, **no retained messages**, clean session `true`, keep-alive 60 s, ClientId = container name/hostname. | 7.1–7.3 |
| G-2.11 | Lifecycle: Birth = `MAM` of the application on connect; Will = `Health` `FAILURE_1`/0; Close = `Health` `NORMAL_0`/0 before graceful disconnect; Birth/Will without `Timestamp`/`SequenceNumber`. `Health` is published on start, on change and every 60 s. | 7.3.3–7.3.5, 8.1.5.1 |
| G-2.12 | Minimum compliance for applications: methods `Pub`, `Get`; resources `MAM`, `Health`, `Profile`, `PublicationList`, `ReferenceDesignation`. `_ToDelete_` items (License, LicenseText, RtLicense, Interfaces, SBoM, NE107) are **not** part of 2.0 (ADR-0005). `Data` ⇒ `Metadata` mandatory. `Set`/`Del` ⇒ `Event` with category `Status` mandatory. | 8.2 |
| G-2.13 | `Set`/`Del` only on fully qualified topics (no multi-target writes); every `Set`/`Del` is answered by a `Status` event carrying the request's `MessageId` as `CorrelationId`. | 8.1.4, 10.1.x |
| G-2.14 | Messages must respect the broker's `MaxPacketSize`; larger result sets use Pagination (`PaginationRequest` on `Get`, `Pagination` on `Pub`). | 6.1.1, 9.3.15 |
| G-2.15 | Container storage contract: `/etc/oi4/mqtt` (`broker.json`, `broker.pem`, `broker_ca.pem`), `/etc/oi4/certs` (`ca.pem`, `<containername>.pem`), `/run/secrets/mqtt_credentials` (base64 `user:pass`), `/run/secrets/mqtt_private_key.pem`, `/run/secrets/mqtt_passphrase`, `/etc/oi4/app` (config), `/opt/oi4/app` (data). | 6.1 |
| G-2.16 | Filter elements must not contain MQTT special characters `+`, `#` (and `/` except where the guideline defines multi-level filters, see SF-005). | 8.1.7 |

## G-3 Process

| ID | Rule |
|----|------|
| G-3.1 | Work happens in **cycles** (see `README.md`). No cycle without a log file; no cycle is done without updated `STATUS.md`. |
| G-3.2 | **Small steps.** One phase at a time; a new phase starts only after the product owner confirmed the exit criteria of the previous one (or explicitly allowed overlap). |
| G-3.3 | **Traceability:** every schema rule, SDK behavior and test references the guideline section and, from Phase 1 on, a requirement ID (`REQ-…`). |
| G-3.4 | **Ask before deciding** (product owner; the PO also decides spec interpretations, ADR-0007) for: choice of language/runtime/framework, repository layout, public API shape of the SDK, interpretation of ambiguous spec sections with protocol impact, licenses, anything changing scope. Record the answer as an ADR. |
| G-3.5 | Spec ambiguities are never silently resolved. Log them as `SF-xxx`, propose an interpretation, mark it in code/schema with the SF id, and keep the decision reversible. |
| G-3.6 | Language: communication with the product owner in German; all artifacts (code, comments, docs, logs, ADRs, commit messages) in English. |
| G-3.7 | Handover-ready at all times: another agent must be able to continue using only `AGENTS.md`, `STATUS.md` and the latest log. |
| G-3.8 | **Version control:** the workspace is a local git repo (ADR-0012); every change is committed individually, referencing the cycle and REQ/SF/ADR ids. Nothing is pushed/published without PO approval (CRA review, Q-011). Deployables live only in `oec/`. |

## G-4 Engineering

| ID | Rule |
|----|------|
| G-4.1 | **Clean room, clean code:** small modules, clear responsibilities, no global mutable state, typed APIs, no dead code, no TODOs without a tracking entry. |
| G-4.2 | **Schema first / contract first:** the JSON Schema and shared test vectors (valid + invalid messages incl. topic) are the contract. The SDK, demos and registry must pass them. Test vectors are language-neutral so a second SDK can reuse them. |
| G-4.3 | **Tests are part of the deliverable:** unit tests for all protocol logic (IL encoding, topic build/parse, message build/validate, pagination, lifecycle); integration tests against a real broker (containerized, TLS). |
| G-4.4 | **Current stable tooling and dependencies** only, few dependencies, no abandoned packages; lock files committed; reproducible builds. |
| G-4.5 | **Secure by default:** TLS mandatory; any insecure mode must be explicit, dev-only and loudly logged. No secrets, keys or real credentials in the repo. Sensitive config values (`Sensitive=true`) never logged. |
| G-4.6 | **Developer experience is a primary goal of D2:** sensible defaults from the container storage contract, minimal boilerplate, helpful error messages that reference the guideline section. |
| G-4.7 | **Demos must run without paid or registration-bound external services** (weather demo: keyless API, plus an offline simulation fallback). |
| G-4.8 | **Containers:** multi-arch images (amd64, arm64; armv7 if feasible), non-root user, container name = hostname convention honored. |
| G-4.9 | **Versioning:** SemVer for all artifacts; JSON Schema `$id`s carry the guideline major version (2.x). |
| G-4.10 | **License:** MIT for all new artifacts (ADR-0004). New dependencies must have MIT-compatible licenses. |
| G-4.11 | Docs are written for newcomers: start with a working example, explain concepts after. The docs site (`oec/docs`) is **standalone – no hyperlinks to the guideline site**; guideline sections may be referenced by number (ADR-0012). |
| G-4.12 | **Verifiable by D7:** every runtime artifact (SDK example app, demos, registry) must pass the conformance tool `test` suite in CI. The tool's verdict is based on the guideline (REQ ids), never on SDK behavior. D7 shares **only the D1 JSON Schemas** with the SDK – no code, no SDK-derived fixtures (ADR-0006). |
| G-4.13 | **Cross-platform tooling:** D7 (and any developer CLI) must run on Windows and macOS (plus Linux for CI); no shell-specific scripts as the only entry point, paths and line endings handled portably. |
