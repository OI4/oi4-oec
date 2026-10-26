# `oec/schema/` – D1 JSON Schemas of the OEC v2 messages

> **State: build-out (C-039).** [ADR-0019](../../project/decisions/ADR-0019-schema-architecture.md) is accepted (C-033 review). The files cover the ua-data envelope (NetworkMessage, DataSetMessage), the helper DataSets (PaginationRequest, Pagination, Locale, Query), the resource Health with its topic forms `pub-health` and `get-health`, and the DataSetMetaData (ua-metadata) with the DataSetMetaDataType and its sub-objects (MG 8.2.2 … 8.2.15), the predefined resource MetaData (MG B2) and the topic forms `pub-metadata`, `get-metadata` and `set-metadata`, the Action messages (MG 8.2.16 … 8.2.18, 8.4.1) with the common service NewDataSetWriterId (MG 9.2.1) and its topic forms `call-newdatasetwriterid` and `reply-newdatasetwriterid`, the resources MAM and Profile with `pub-`/`get-mam` and `pub-`/`get-profile` (C-036), the resources PublicationList and ReferenceDesignation with `pub-`/`get-`/`set-publicationlist` and `pub-`/`get-`/`set-`/`del-referencedesignation` (C-037), and the resources Data (with the predefined DataSet Oi4Pv), Event and SubscriptionList with `pub-`/`get-`/`set-data`, `pub-event` and `pub-`/`get-`/`set-`/`del-subscriptionlist` (C-038), and the resource Config with `pub-`/`get-`/`set-config` (C-039). With Config, every allowed row of the method-resource matrix has its entry schema (29 of 29). Outside the messages, `platform/broker.schema.json` describes the platform file `broker.json` (RIG 4.1.1, C-039) for the infrastructure compliance CLI (D8).

The schemas are derived from the spec baseline in [`../spec/`](../spec/), which is derived only from the guideline (ADR-0018). The schemas never define requirements of their own. Each constraint names the requirement it enforces in a `$comment` that starts with the REQ id(s).

## Layout

| Path | Content |
|---|---|
| `v2/opcua.schema.json` | OPC UA base types used by the messages (Byte, UInt16, UInt32, Int32, DateTime, Guid, ConfigurationVersion, NodeId (REQ-MSG-039), QualifiedName, LocalizedText (REQ-MSG-040)) and `VerboseEnum` (REQ-MSG-031, in the `allOf` of every numbered enumeration of a DataSet) |
| `v2/envelope.schema.json` | NetworkMessage and DataSetMessage of MessageType `ua-data`, the DataSetMetaData NetworkMessage (`ua-metadata`), plus PublisherId, MessageId, ReplyTo, DataSetClassId, NotReservedClassId and DataSetWriterId |
| `v2/metadata.schema.json` | DataSetMetaDataType with FieldMetaData, KeyValuePair, Structure/Enum/SimpleType descriptions (MG 8.2.4 … 8.2.15), and `ResourceMetaData`: the predefined MetaData of a resource (MG B2) with its DataSetClassId (REQ-MSG-012/010) |
| `v2/actions.schema.json` | Action NetworkMessage (`ua-action-request`/`-response`), ActionRequest, ActionResponse with `Status`, and the request/response DataSets of NewDataSetWriterId (reserved DataSetWriterId 8); a Bad `Status` forbids Payload fields (REQ-ACT-012) |
| `v2/helpers.schema.json` | helper DataSets PaginationRequest, Pagination, Locale and Query, with their DataSetMessages (reserved DataSetWriterIds 1/2/3/9); the Payload of a Query is refined by the resource module |
| `v2/resources/<resource>.schema.json` | one module per OI4 resource: the DataSet and its DataSetMessage (`config`, `data`, `event`, `health`, `mam`, `profile`, `publicationlist`, `referencedesignation`, `subscriptionlist`) |
| `v2/topics/<rule>.schema.json` | **entry schema per topic form**. `<rule>` is the ABNF rule of `../spec/topic/topic.abnf` and of the matrix row (option B of ADR-0019) |
| `v2/platform/broker.schema.json` | content of the platform file `broker.json` of the Message Bus storage (RIG 4.1.1, REQ-CNT-003): Address (IPv4, IPv6 or host name), SecurePort, MaxPacketSize; no closure, further elements are allowed (SF-100); used by D8 |
| `v2/message.schema.json` | topic-agnostic schema of any OEC message, used for monitoring when the topic is unknown (option A, for comparison) |
| `schema.go`, `go.mod` | Go module `github.com/OI4/oec/schema`: embeds `v2/` (`schema.FS`) for D7 |
| `tools/cmd/schematest` | harness: builds the bundle, checks the `$comment` tracing against the catalog, validates the payload vectors and probes, maps errors to REQ ids, writes `dist/` |
| `tests/rule-layer.yaml` | expectation list: invalid vectors whose violated REQs are partitioned into `schema` and `rule` (rule layer of D7 `validate`) |
| `tools/rust-check` | second, independent validator (Rust `jsonschema` crate) over `dist/cases.json` |
| `check.sh` | local CI: both validators |

Every file uses `$schema` 2020-12, with `$id` = `https://schemas.openindustry4.com/oec/v2/<path>`. The base was decided by the PO (ADR-0019 §6: major-only `$id`s, the tools report the bundle version). The `v2/` sources are the **strict** flavor: OI4-defined objects are closed with a closure `"unevaluatedProperties": {"$comment": "REQ-MSG-001 …", "not": {}}`, so an undefined key maps to the REQ of its closure. The **tolerant** flavor (unknown-key tolerance of REQ-MSG-046, like the oracle mode `tolerant`) is generated, never edited. It is the same tree without the closures, with its `$id`s under `…/oec/v2/tolerant/`. `schematest` writes both flavors to `dist/<host>/oec/v2/…`.

## Running

Requirements:
- Go, with the toolchain pinned by the module (`GOTOOLCHAIN=auto`)
- Rust/cargo

```sh
./check.sh                                    # gofmt, vet, Go validator, Rust validator
cd tools && go run ./cmd/schematest -v        # table + error details per rejected vector
```

`schematest` first checks the REQ tracing (ADR-0019 §1) against the catalog `../spec/requirements/`: every REQ at the start of a `$comment` exists, is not deprecated and carries the verification method `schema` (in `platform/` files also `d8`: these files are checked by the infrastructure compliance CLI; ADR-0019 amendment C-039), and every reference in the parentheses after the ids (e.g. `(MG 8.2.3)`) is a `ref` of one of these REQs.

It then routes every payload vector of `../spec/vectors/payload/` through `topic-patterns.json`. A bare `object: DataSetMetaDataType` vector is the predefined MetaData of a resource (MG B2, SF-054) and is validated against `metadata.schema.json#/$defs/ResourceMetaData` (B) and `#/$defs/DataSetMetaData` (A). A valid vector with a topic whose rule has no entry schema yet is validated against `message.schema.json` (A) only. A vector is in scope when the matched rule has an entry schema. For each vector in scope, the harness:

1. validates the vector against the entry schema (B) and against `message.schema.json` (A), in the flavor of the vector `mode`;
2. maps every error leaf to the REQ ids of the deepest `$comment` on its evaluation path (an undefined key maps to the REQ of its closure);
3. checks that valid vectors pass. Invalid vectors must fail B with **exactly** the REQs of `violates`. A vector listed in `tests/rule-layer.yaml` must fail B with exactly its `schema` REQs (or pass B if they are empty); its `rule` REQs are left to the rule layer of D7 `validate`. A vector with a declared `role` (Birth, Close, Will; Q-020 (a)) is validated like any other message; the rules of its role are rule layer R-6 (ADR-0019 §5).

The broker-json vectors of `../spec/vectors/platform/` (`V-BRK-`) are validated against `platform/broker.schema.json`: valid ones must pass, invalid ones must fail with exactly their `violates` REQs (there is no rule layer for this file). Both flavors are equal here, because the file has no closure.

The **probes** (`probes.go`) are synthetic mutations of vectors. They exercise schema paths that no reviewed vector covers yet, and they are not catalog vectors. Every verdict is written to `dist/cases.json`, which the Rust validator re-checks.
