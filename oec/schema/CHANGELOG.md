# Changelog of the OEC v2 schema bundle (D1)

Versions follow ADR-0019 §6 and amendment C-041:

- the bundle version is `2.MINOR.PATCH`;
- the `$id`s carry only the major version: `https://schemas.openindustry4.com/oec/v2/<path>` (strict flavor) and `…/oec/v2/tolerant/<path>` (generated tolerant flavor);
- each release names the guideline commit it is derived from (`schema.Baseline`);
- tools that embed the bundle (D7, SDK) report the version and the baseline with every result (`schema.Version`, `schema.Baseline`).

Within v2:

- **PATCH:** a fix of a constraint that contradicts the guideline baseline, or a re-baseline without a visible change.
- **MINOR:** additions (new files, new `$defs`, new optional fields) and new guideline baselines.
- No file and no `$defs` name of a release is removed or renamed. A removal is a breaking change and needs `v3`.

`release/<version>.json` freezes each release: the path, a hash of the canonical JSON and the `$defs` names of every source file. `schematest` fails in these cases:
- a released version differs from its manifest;
- a released `$id` or `$defs` name disappears;
- `schema.Baseline` differs from the spec catalog.

Work after a release uses a pre-release version (e.g. `2.1.0-dev`) and an `Unreleased` section here.

## Unreleased (2.0.1-dev)

- Re-baseline on guideline `ccb475a` (C-045): SF-101 (MG 8.3.8, `Get` with every `Mode` except `OFF_0`, combined `ON_REQUEST_…` values recommended) and SF-102 (MG 7.2, a directly connected smart device uses `OTConnector`). No schema content change: both `Mode` forms stay valid, and the `ServiceType` enumeration is unchanged. PATCH, because the re-baseline has no visible change.
- Re-baseline on guideline `e0262dd` (C-045 addendum 1): SF-103 (MG B1.1.8/B2.6 examples use `ON_REQUEST_APPLICATION_SOURCE_FILTER_15` for requestable resources). No schema content change; the golden test vectors V-PAY-037 … 040 follow the examples.
- Re-baseline on guideline `734c560` (C-046 review): SF-104 (MG 4.3.1 and MG A3: the elements `Model`, `ProductCode` and `SerialNumber` of the `Oi4Identifier` are always dnp-encoded, in line with the MG 4.1 masking rule). No schema content change: the rule cannot be expressed in JSON Schema and is checked by the rule layer R-3 (`ProductInstanceUri` of the MAM, REQ-RES-MAM-006); the new invalid vector V-PAY-245 is listed in `tests/rule-layer.yaml`.
- Re-baseline on guideline `59e3ae3` (C-051 review): SF-105 (RIG 4.1.1: the broker accepts every MQTT payload up to `MaxPacketSize`, a packet limit of the broker is larger by 65544 octets; payload + topic length + 4 must not exceed 268435455). No schema content change: `broker.json` keeps its form and range (V-BRK-001 … 018 unchanged); the new rules concern the broker and the MQTT packet and are checked by D8 (T8-BUS-01).
- Re-baseline on guideline `9044929` (C-052 review): SF-106 (MG 7.5.1: `Health` is published at initialization and on change; the `Health` of the application itself is additionally published at least every 60 seconds as a heartbeat, a periodic publication for subordinate assets is optional). No schema content change: the rule concerns the timing of publications and is checked by D7 `monitor` (T7-MON-05).
- Re-baseline on guideline `2e32a6b` (C-056 review): SF-107 (MG 9.1.9, B1.1.3, B1.1.4: a `Data` or `Config` DataSet that can be changed with `Set` is listed in the `SubscriptionList` with the subscription of its `Set` topic, `Config` `NONE_0`) and SF-108 (MG 9.1: the Status event for a request whose `Source` addresses an asset the entity neither is nor represents is published with the `Source` `AppId`). No schema content change: a `SubscriptionList` entry keeps its form (`TopicPath` is a topic string, `Config` `NONE_0` is already allowed) and the Status event is unchanged; the rules are checked by D7 (T7-SET-01, T7-GET-05, T7-SET-02). PATCH, because the re-baseline has no visible change.
- Re-baseline on guideline `57ebe91` (C-059 review): SF-109 (MG 9.1.9: subscriptions that only exist for the time of a request are not listed in the `SubscriptionList`), SF-110 (MG 9.1.9: only the subscriptions to request topics of the own `AppId` are excluded; B1.1.9 unchanged) and SF-111 (MG 9.1.9: an application that does not support the creation of entries rejects such a `Set` with the status `Event` `Bad`, e.g. `BadNotSupported`). No schema content change: the rules concern which subscriptions are listed and how a `Set` is answered, not the form of an entry; V-PAY-072 … 075 unchanged.
- Re-baseline on guideline `eb7889c` (C-060 review): SF-112 (MG 8.4.1: the `Set` that creates a `DataSet` carries the `DataSetWriterId` provided by `NewDataSetWriterId`; a `DataSet` already listed in the `PublicationList` keeps its listed id) and SF-113 (MG 9.2.1: a `NewDataSetWriterId` request for a `Source` the application neither is nor represents is answered with `BadNotFound` and an empty `Payload`). No schema content change: an `ActionResponse` with a Bad `Status` and an empty `Payload` is already valid (REQ-ACT-012).
- Re-baseline on guideline `a55f475` (C-070 review): SF-114 (RIG 4.1.4: the SBOM lists every software component with its name, its version and its licenses in SPDX or CycloneDX JSON, the version of the format is not restricted; every license is named unambiguously, a component without a stated license does not comply; the text of every named license and license exception is part of the SBOM or stored as `<License-ID>.txt`). No schema content change: the SBOM is no OI4 message, its rules are checked by D8 (T8-IMG-03). PATCH, because the re-baseline has no visible change.
- Re-baseline on guideline `b955783` (C-071 review): SF-115 (MG 6.2.2: the broker certificate is valid for the broker address of the Message Bus storage (`Address` of `broker.json`; a host name as DNS name, an IP address as IP address of the Subject Alternative Name) and within its period of validity). No schema content change: the rule concerns the TLS certificate of the broker, not an OI4 message or `broker.json`; it is checked by D8 (T8-BUS-01). PATCH, because the re-baseline has no visible change.

## 2.0.0

First release, P2 exit (C-041), accepted by the PO on 2026-10-26; local git tag `oec-schema-2.0.0`. Guideline baseline `168e1d1` (OEC Messaging Guideline and Runtime & Infrastructure Guideline, branch `feature/spec-findings`, GL-ADR-001 … 020, SF-001 … 100).

**Content:** 45 source files (strict flavor), plus the generated tolerant flavor.

| Path | Content |
|---|---|
| `opcua.schema.json` | OPC UA base types (Byte … DateTime, Guid, NodeId, QualifiedName, LocalizedText, ConfigurationVersion), `VerboseEnum` |
| `envelope.schema.json` | NetworkMessage/DataSetMessage (`ua-data`), DataSetMetaData NetworkMessage (`ua-metadata`), PublisherId, MessageId, ReplyTo, DataSetClassId, DataSetWriterId |
| `metadata.schema.json` | DataSetMetaDataType and its sub-objects (MG 8.2.4 … 8.2.15), predefined resource MetaData (MG B2) |
| `actions.schema.json` | Action messages (MG 8.2.16 … 8.2.18, 8.4.1), NewDataSetWriterId (MG 9.2.1) |
| `helpers.schema.json` | PaginationRequest, Pagination, Locale, Query |
| `resources/*.schema.json` | config, data (incl. Oi4Pv), event, health, mam, profile, publicationlist, referencedesignation, subscriptionlist |
| `topics/*.schema.json` | 29 entry schemas, one per allowed row of the method-resource matrix |
| `message.schema.json` | topic-agnostic schema of any OEC message |
| `platform/broker.schema.json` | platform file `broker.json` (RIG 4.1.1), for D8 |

**Verification at the release:**
- 244 payload vectors and 18 broker-json vectors of `oec/spec/vectors`, plus 128 probes;
- 634 cases, 0 disagreements between santhosh-tekuri/jsonschema (Go) and jsonschema-rs (Rust); the informative Python `jsonschema` check agrees;
- oracle/schema cross-check (`spectool crosscheck`): 0 disagreements;
- coverage gate: 158 `schema`/`d7-validate` REQs, 0 gaps.

**Limits:**
- Rules that need topic values, several fields or several messages are in the rule layer of D7 `validate`, not in the schemas (ADR-0019 §5, `tests/rule-layer.yaml`). Examples: equalities with the topic, uniqueness of keys, the Identification Link heuristic, calendar validity, the role of Birth/Close/Will.
- The `$id`s are not hosted yet (Q-011). Consumers resolve them from the embedded bundle (`schema.FS`) or from `dist/`.
