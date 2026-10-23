# `oec/schema/` – D1 JSON Schemas of the OEC v2 messages

> **State: proof of concept (C-033).** [ADR-0019](../../project/decisions/ADR-0019-schema-architecture.md) is accepted (C-033 review). The files cover the envelope (NetworkMessage, DataSetMessage), the helper DataSets and the resource Health with its topic forms `pub-health` and `get-health`. The full build-out starts in C-034 (ROADMAP P2).

The schemas are derived from the spec baseline in [`../spec/`](../spec/), which is derived only from the guideline (ADR-0018). The schemas never define requirements of their own. Each constraint names the requirement it enforces in a `$comment` that starts with the REQ id(s).

## Layout

| Path | Content |
|---|---|
| `v2/opcua.schema.json` | OPC UA base types used by the messages (Byte, UInt16, UInt32, DateTime, Guid, ConfigurationVersion) |
| `v2/envelope.schema.json` | NetworkMessage and DataSetMessage of MessageType `ua-data`, plus PublisherId, MessageId, ReplyTo, DataSetClassId and DataSetWriterId |
| `v2/helpers.schema.json` | helper DataSets PaginationRequest, Pagination and Locale, with their DataSetMessages (reserved DataSetWriterIds 1/2/3) |
| `v2/health.schema.json` | resource module Health: the DataSet and its DataSetMessage |
| `v2/topics/<rule>.schema.json` | **entry schema per topic form**. `<rule>` is the ABNF rule of `../spec/topic/topic.abnf` and of the matrix row (option B of ADR-0019) |
| `v2/message.schema.json` | topic-agnostic schema of any OEC message, used for monitoring when the topic is unknown (option A, for comparison) |
| `schema.go`, `go.mod` | Go module `github.com/OI4/oec/schema`: embeds `v2/` (`schema.FS`) for D7 |
| `tools/cmd/schematest` | PoC harness: builds the bundle, validates the payload vectors and probes, maps errors to REQ ids, writes `dist/` |
| `tests/rule-layer.yaml` | expectation list: invalid vectors whose violated REQs are partitioned into `schema` and `rule` (rule layer of D7 `validate`); empty in the PoC |
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

`schematest` routes every payload vector of `../spec/vectors/payload/` through `topic-patterns.json`. A vector is in scope when the matched rule has an entry schema. For each vector in scope, the harness:

1. validates the vector against the entry schema (B) and against `message.schema.json` (A), in the flavor of the vector `mode`;
2. maps every error leaf to the REQ ids of the deepest `$comment` on its evaluation path (an undefined key maps to the REQ of its closure);
3. checks that valid vectors pass. Invalid vectors must fail B with **exactly** the REQs of `violates`. A vector listed in `tests/rule-layer.yaml` must fail B with exactly its `schema` REQs (or pass B if they are empty); its `rule` REQs are left to the rule layer of D7 `validate`.

The **probes** (`probes.go`) are synthetic mutations of vectors. They exercise schema paths that no reviewed vector covers yet, and they are not catalog vectors. Every verdict is written to `dist/cases.json`, which the Rust validator re-checks.
