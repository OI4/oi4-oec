# `oec/schema/` – D1 JSON Schemas of the OEC v2 messages

> **State: build-out (C-035).** [ADR-0019](../../project/decisions/ADR-0019-schema-architecture.md) is accepted (C-033 review). The files cover the ua-data envelope (NetworkMessage, DataSetMessage), the helper DataSets (PaginationRequest, Pagination, Locale, Query), the resource Health with its topic forms `pub-health` and `get-health`, and the DataSetMetaData (ua-metadata) with the DataSetMetaDataType and its sub-objects (MG 8.2.2 … 8.2.15), the predefined resource MetaData (MG B2) and the topic forms `pub-metadata`, `get-metadata` and `set-metadata`. The Actions and the other resources follow in C-036 … C-038 (ROADMAP P2).

The schemas are derived from the spec baseline in [`../spec/`](../spec/), which is derived only from the guideline (ADR-0018). The schemas never define requirements of their own. Each constraint names the requirement it enforces in a `$comment` that starts with the REQ id(s).

## Layout

| Path | Content |
|---|---|
| `v2/opcua.schema.json` | OPC UA base types used by the messages (Byte, UInt16, UInt32, Int32, DateTime, Guid, ConfigurationVersion, NodeId (REQ-MSG-039), QualifiedName, LocalizedText (REQ-MSG-040)) and `VerboseEnum` (REQ-MSG-031, in the `allOf` of every numbered enumeration of a DataSet) |
| `v2/envelope.schema.json` | NetworkMessage and DataSetMessage of MessageType `ua-data`, the DataSetMetaData NetworkMessage (`ua-metadata`), plus PublisherId, MessageId, ReplyTo, DataSetClassId, NotReservedClassId and DataSetWriterId |
| `v2/metadata.schema.json` | DataSetMetaDataType with FieldMetaData, KeyValuePair, Structure/Enum/SimpleType descriptions (MG 8.2.4 … 8.2.15), and `ResourceMetaData`: the predefined MetaData of a resource (MG B2) with its DataSetClassId (REQ-MSG-012/010) |
| `v2/helpers.schema.json` | helper DataSets PaginationRequest, Pagination, Locale and Query, with their DataSetMessages (reserved DataSetWriterIds 1/2/3/9); the Payload of a Query is refined by the resource module |
| `v2/resources/<resource>.schema.json` | one module per OI4 resource: the DataSet and its DataSetMessage (so far `health`) |
| `v2/topics/<rule>.schema.json` | **entry schema per topic form**. `<rule>` is the ABNF rule of `../spec/topic/topic.abnf` and of the matrix row (option B of ADR-0019) |
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

`schematest` first checks the REQ tracing (ADR-0019 §1) against the catalog `../spec/requirements/`: every REQ at the start of a `$comment` exists, is not deprecated and carries the verification method `schema`, and every reference in the parentheses after the ids (e.g. `(MG 8.2.3)`) is a `ref` of one of these REQs.

It then routes every payload vector of `../spec/vectors/payload/` through `topic-patterns.json`. A bare `object: DataSetMetaDataType` vector is the predefined MetaData of a resource (MG B2, SF-054) and is validated against `metadata.schema.json#/$defs/ResourceMetaData` (B) and `#/$defs/DataSetMetaData` (A). A valid vector with a topic whose rule has no entry schema yet is validated against `message.schema.json` (A) only. A vector is in scope when the matched rule has an entry schema. For each vector in scope, the harness:

1. validates the vector against the entry schema (B) and against `message.schema.json` (A), in the flavor of the vector `mode`;
2. maps every error leaf to the REQ ids of the deepest `$comment` on its evaluation path (an undefined key maps to the REQ of its closure);
3. checks that valid vectors pass. Invalid vectors must fail B with **exactly** the REQs of `violates`. A vector listed in `tests/rule-layer.yaml` must fail B with exactly its `schema` REQs (or pass B if they are empty); its `rule` REQs are left to the rule layer of D7 `validate`. A vector with a declared `role` (Birth, Close, Will; Q-020 (a)) is validated like any other message; the rules of its role are rule layer R-6 (ADR-0019 §5).

The **probes** (`probes.go`) are synthetic mutations of vectors. They exercise schema paths that no reviewed vector covers yet, and they are not catalog vectors. Every verdict is written to `dist/cases.json`, which the Rust validator re-checks.
