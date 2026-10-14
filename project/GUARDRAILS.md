# Guardrails – OI4 OEC v2.0

These rules apply to every human and agent working on this project. A deviation requires an accepted ADR or explicit approval of the product owner, recorded in the cycle log.

---

## G-1 Sources and authority

| ID | Rule |
|----|------|
| G-1.1 | **Source of truth** (ADR-0001) are the two v2.0 guideline documents in `oi4-oec-documentation`: the **OEC Messaging Guideline** (`docs/guideline/`, cited as "MG x.y") and the **OEC Runtime & Infrastructure Guideline** (`docs/infrastructure-guideline/`, cited as "RIG x.y"). The guideline ADRs in `oi4-oec-documentation/rfcs_adrs/` (ADR-001 … ADR-011, cited as "GL-ADR-0xx" to distinguish them from our project ADRs) explain the rationale and their *Implementation Notes* are binding interpretations. The Confluence export under `oi4-oec-documentation/guideline/` and the PDFs (v1.1.0) are **historical reference only**. |
| G-1.2 | The guideline is **not final** (version string "x.x.x – ongoing / in discussion"). Always record the guideline revision (git commit of `oi4-oec-documentation`) you worked against in the cycle log. Current baseline: `5f070bb` (2026-10-14, all GL-ADRs implemented). Previous baseline: `c85ce2a` (2026-10-06). |
| G-1.3 | `legacy/` (oi4-oec-service, oi4-oec-service-go, oi4-oec-demo, oi4-oec-registry) is **history only**: read-only, never modified, **no code copied**. It may be consulted for ideas and lessons learned; if an idea is reused, name the source in the log. Legacy implements guideline 1.x (namespace `Oi4`), which is incompatible with 2.0. |
| G-1.4 | Do not edit guideline text unless the product owner explicitly asks for it. Problems in the guideline go to `SPEC-FINDINGS.md`. |
| G-1.5 | External standards are referenced in their **latest published version** (e.g. JSON Schema 2020-12 (ADR-0003), OPC UA **1.05.04** Part 6/14 as required by GL-ADR-006, RFC 4648, RFC 3986, IEC 61406-1). Where the guideline still links older versions (e.g. OPC UA v1.04 pages), note it as a spec finding instead of silently deviating. |

## G-2 Protocol invariants (derived from the guideline – must hold in every artifact)

Section numbers refer to the Messaging Guideline (MG) or the Runtime & Infrastructure Guideline (RIG). If the guideline changes, update this list in the same cycle.

| ID | Invariant | Guideline |
|----|-----------|-----------|
| G-2.1 | Topic schema: `Oi4v2/<ServiceType>/<AppId>/<Method>/<Resource>[/<Source>[/<Filter>]]`. Namespace is exactly **`Oi4v2`** for this version; `Oi4` denotes v1. A request is always answered in the namespace in which it was received; versions are never mixed. All deliverables are **v2 only**; v1 traffic is ignored (ADR-0015). | MG 8.1, 8.1.1, GL-ADR-003 |
| G-2.2 | `AppId` and `Source` are the **base64url (RFC 4648 §5) encoded Identification Link, without padding**, forming exactly **one** topic level (max. 340 chars). There is exactly one valid encoding per IL. | MG 4.2, 8.1.3, 8.1.6 |
| G-2.3 | In payloads (`PublisherId`, `MessageId`, `ReplyTo`, `DataSetWriterName`, `RequestorId`, `ProductInstanceUri`, ...) the Identification Link is **always unencoded** and used exactly as assigned (no normalization, case-sensitive, char-by-char comparison). | MG 4.1, 4.2 |
| G-2.4 | Identification Link: ASCII subset of IEC 61406-1, max. 255 chars, lowercase authority, optional scheme (`https` recommended; with/without scheme = different ILs), no `%`. The `Oi4Identifier` (`ManufacturerUri/dnp(Model)/dnp(ProductCode)/dnp(SerialNumber)`) is one optional way to build an IL. | MG 4.1, 4.3, A3 |
| G-2.5 | ServiceTypes: `Registry`, `OTConnector`, `Utility`, `Persistence`, `Aggregation`, `OOCConnector`, `ITConnector`. Methods: `Pub`, `Get`, `Set`, `Del`, `Call`, `Reply`. | MG 8.1.2, 8.1.4 |
| G-2.6 | Payloads follow **OPC UA Part 14/6 V1.05.04** JSON with the **fixed content masks of MG 9.1.4** (no PubSub configuration is exchanged): `ua-data` (NetworkMessage), `ua-metadata` (DataSetMetaData), `ua-action-request` / `ua-action-response` (ActionNetworkMessage, replaces `MSG`). `Messages` is always an array. DataSet payloads use VerboseEncoding + RawData; Actions use CompactEncoding. All keys PascalCase; OI4-defined objects **must not be extended**. `ReplyTo` in `ua-metadata` is the only OI4-specific key. | MG 9.1, 9.1.4, 9.2, GL-ADR-006/007 |
| G-2.7 | `PublisherId` = `<ServiceType>/<IdentificationLink>`; `MessageId` = `<unixTimestampInMs>-<PublisherId>`, globally unique. **`ReplyTo`** (replaces `CorrelationId`) is mandatory in `ua-data`/`ua-metadata`: `""` if not caused by another message, otherwise the `MessageId` of the causing message (passed on along chains). Actions correlate via `CorrelationData`, `RequestId`, `RequestorId`, `ResponseAddress`; `PublisherId` of request and response is the Responder. | MG 9.2.1, 9.2.2, 9.2.16–9.2.18, GL-ADR-005/007 |
| G-2.8 | Every `DataSetMessage` carries **`DataSetWriterName`** (= unencoded IL of the asset, corresponds to topic `<Source>`; replaces `Source`) and **`WriterGroupName`** (= topic `<Filter>`, `""` if the resource has no filter; replaces `Filter`). Not used in Action messages. `Status` is a StatusCode object, sent only if not Good. `MetaDataVersion` and `MinorVersion` are mutually exclusive. `MessageType` optional (`ua-keyframe` default, `ua-event` for Event). | MG 9.2.3, GL-ADR-004/006 |
| G-2.9 | Predefined `DataSetClassId`s (MG A2) are used verbatim; IDs of removed resources are reserved and never reused. | MG 9.2.1, A2 |
| G-2.10 | `DataSetWriterId` is UInt16, unique per publisher; 0–9 reserved: 1 PaginationRequest, 2 Pagination, 3 Locale, 4–8 common-service Actions (`ActionTargetId` 1); applications start at 10 (see SF-001). | MG 9.2.3, 9.2.17 |
| G-2.11 | MQTT: v3.1.1, TLS ≥ 1.2 only (default port 8883, no plain 1883), auth via username/password or client certificate (cert wins), QoS 0 default, **no retained messages**, clean session `true`, keep-alive 60 s, ClientId = container name/hostname. TLS/auth mandatory at all levels; remaining client behavior mandatory at Level 3, recommended at Levels 1–2. | MG 7.1–7.4, 1.3 |
| G-2.12 | Lifecycle: Birth = `MAM` of the application on every connect; Will = `Health` `FAILURE_1`/0; Close = `Health` `NORMAL_0`/0 before graceful disconnect; Birth/Will/Close without `Timestamp`/`SequenceNumber`. Birth + Will mandatory from Level 1. `Health` (incl. optional `Details[]`) is published on start, on any change (also of a `DetailObject`) and every 60 s; `Health` = most severe status. | MG 7.3.3–7.3.5, 9.3.2, GL-ADR-001 |
| G-2.13 | **Compliance levels** (GL-ADR-009) per domain. Messaging: **L1** `Pub` of `MAM` + `Health` (self + subordinate assets), Birth/Will, TLS/auth, topic + payload format; **L2** + `Get` (MAM/Health via Get) and `Set` where applicable, `Profile`/`PublicationList`/`ReferenceDesignation` recommended; **L3** full guideline incl. `Profile`, `PublicationList`, `ReferenceDesignation` mandatory. `Data` ⇒ `Metadata` (L2 recommended, L3 mandatory). `Set`/`Del` ⇒ `Event` category `Status` mandatory. "OI4 compliant" without level = L3. Removed in 2.0: License, LicenseText, RtLicense, Interfaces, SBoM, NE107 (ADR-0005, GL-ADR-008). | MG 1.3, 8.2, RIG 1.2 |
| G-2.14 | `Set`/`Del` only on fully qualified topics; every `Set`/`Del` is answered by a `Status` event whose `ReplyTo` is the request's `MessageId`. | MG 8.1.4, 10.1.x |
| G-2.15 | Messages must respect the broker's `MaxPacketSize`; larger result sets use Pagination (`PaginationRequest` on `Get`, `Pagination` on `Pub`, linked by `PaginationId`). | RIG 4.1.1, MG 9.3.11 |
| G-2.16 | Container storage contract (Service Container Compliance, mandatory at L3, recommended at L2): `/etc/oi4/mqtt` (`broker.json`, `broker.pem`, `broker_ca.pem`), `/etc/oi4/certs` (`ca.pem`, `<containername>.pem`), `/run/secrets/mqtt_credentials` (base64 `user:pass`), `/run/secrets/mqtt_private_key.pem`, `/run/secrets/mqtt_passphrase`, `/etc/oi4/app` (config), `/opt/oi4/app` (data), **`/opt/oi4/licenses`** (SBOM in SPDX or CycloneDX + license texts, shipped in the image; mandatory from L2). OCI image mandatory at all levels for containerized components. | RIG 1.2, 4.1, 4.2, GL-ADR-002 |
| G-2.17 | Filter elements must not contain MQTT special characters `+`, `#` (and `/` except where the guideline defines multi-level filters, see SF-005). | MG 8.1.7 |

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
| G-4.6 | **Developer experience is a primary goal of D2:** usable at every compliance level (L1 smart device without container contract … L3 full container app); microcontrollers are served by the micro SDK D9, which reuses the `no_std` part of `oec-core` (ADR-0016), sensible defaults from the container storage contract, minimal boilerplate, helpful error messages that reference the guideline section. |
| G-4.7 | **Demos must run without paid or registration-bound external services** (weather demo: keyless API, plus an offline simulation fallback). |
| G-4.8 | **Containers:** OCI images, multi-arch (amd64, arm64; armv7 if feasible), non-root user, container name = hostname convention honored, **SBOM (SPDX or CycloneDX) + license texts generated at build time into `/opt/oi4/licenses`** (RIG 4.1.4; our own containers target Service Container Compliance Level 3). |
| G-4.9 | **Versioning:** SemVer for all artifacts; JSON Schema `$id`s carry the guideline major version (2.x). |
| G-4.10 | **License:** MIT for all new artifacts (ADR-0004). New dependencies must have MIT-compatible licenses. |
| G-4.11 | Docs are written for newcomers: start with a working example, explain concepts after. The docs site (`oec/docs`) is **standalone – no hyperlinks to the guideline site**; guideline sections may be referenced by number (ADR-0012). |
| G-4.12 | **Verifiable by D7/D8:** every runtime artifact (SDK example app, micro SDK example, demos, registry) must pass D7 at its declared Messaging level and – if containerized – D8 at its Service Container level (ADR-0016) in CI. The tools' verdict is based on the guideline (REQ ids), never on SDK behavior. D7/D8 share **only the D1 JSON Schemas** with the SDK – no code, no SDK-derived fixtures (ADR-0006). |
| G-4.13 | **Cross-platform tooling:** D7, D8 (and any developer CLI) must run on Windows and macOS (plus Linux for CI); no shell-specific scripts as the only entry point, paths and line endings handled portably. |
