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

## 2.0.0

First release, P2 exit (C-041). Guideline baseline `168e1d1` (OEC Messaging Guideline and Runtime & Infrastructure Guideline, branch `feature/spec-findings`, GL-ADR-001 … 020, SF-001 … 100).

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
