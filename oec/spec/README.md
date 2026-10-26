# `oec/spec/` – OEC v2 spec baseline

Machine-readable baseline derived **only** from the OEC guideline (Messaging Guideline "MG", Runtime & Infrastructure Guideline "RIG", guideline ADRs "GL-ADR"). It is the shared contract of the JSON Schemas (D1), the SDKs (D2, D9), the compliance CLIs (D7, D8) and the documentation (D6). See ADR-0018.

| Path | Content | Edited by |
|---|---|---|
| `requirements/areas.yaml` | area registry: code, title, guideline sections, cycle, status | hand |
| `requirements/<area>.yaml` | requirements `REQ-<AREA>-NNN` of one area | hand |
| `requirements/*.schema.json` | JSON Schema 2020-12 of the files above | hand |
| `requirements/view/*.md` | Markdown view per area – the PO review artifact | `spectool render` |
| `coverage/inventory.yaml` | all candidates of normative statements in MG + RIG (keyword, field, admonition, implicit, table) | `spectool inventory` |
| `coverage/informative.yaml` | inventory items that are no requirement, with reason (MG 1.2, RIG 1.2) | hand |
| `coverage/examples.yaml` (+ schema) | fenced code blocks of MG + RIG without a golden vector (templates, code in other languages, deployment snippets, examples with a spec finding), with reason (C-031) | hand |
| `coverage/report.md` | coverage of the inventory per area and of the guideline examples by golden vectors | `spectool coverage` |
| `coverage/implications.yaml` (+ schema) | implication map of umbrella requirements: umbrella → detail REQs + remainder not covered by vectors (ADR-0019 §9, C-034) | hand |
| `coverage/vectors.md` | vector coverage of the `schema`/`d7-validate` requirements (reviewed positive + negative vector, umbrellas derived, layer of each negative vector), entry schema per allowed matrix row (P2 exit gate) | `spectool coverage` |
| `topic/topic.abnf` | formal topic grammar (RFC 5234 + RFC 7405), one rule per allowed Method × Resource form | hand |
| `topic/topic-patterns.json` | anchored regexes generated from `topic.abnf` (portable dialect, see below) | `spectool topic` |
| `matrix/method-resource.yaml` (+ schema) | Method × Resource matrix: allowed, `Source`/`Filter` required/optional/forbidden, ABNF rule, guideline refs; payload columns of the allowed rows (MessageType, DataSetClassId/DataSetWriterId rule, content, helpers, answer, requirements; C-028) | hand |
| `matrix/view/method-resource.md` | Markdown view of the matrix: overview grid, topic form and payload columns per combination, linked topic and payload vectors – PO review artifact | `spectool render` |
| `vectors/` (+ `vectors.schema.json`) | language-neutral test vectors: `topic/` (golden from the guideline, constructed valid, invalid), `il/` (Identification Link, dnp-encoding, `Oi4Identifier`), `payload/` (topic + JSON message, golden from Appendix B and the concrete examples of the MG body), `platform/` (content of the platform file `broker.json`, RIG 4.1.1; C-039) | hand, checked by `spectool lint` |
| `vectors/view/*.md` | Markdown view per vector file (overview table + one section per vector with decoded Identification Links and linked REQs) – PO review artifact | `spectool render` |
| `tools/` | Go module with `spectool` | – |
| `go.mod`, `spec.go` | Go module `github.com/OI4/oec/spec`: embeds the data files as `spec.FS` for D7 (`oec/conformance`, C-040) | hand |

## Citing requirements

Use the id, e.g. `REQ-CMP-003`. Ids are never reused or renumbered; a replaced entry gets `status: deprecated` and `replaced_by`. Entries in `draft` are not yet confirmed by the PO (GUARDRAILS G-3.9).

Fields of an entry (`requirements/catalog.schema.json`):

- `statement` – one atomic requirement in our words, written in present tense; the obligation is carried by `keyword` and `levels`, the guideline wording by `quotes`.
- `keyword` – MUST / MUST NOT / SHOULD / SHOULD NOT / MAY as mapped by MG 1.2 / RIG 1.2; for graded domains it equals the Level 3 obligation.
- `implicit: true` – the guideline states the behavior without keyword; treated as MUST by the interim rule of SF-035. Since C-016 the guideline carries the keywords (SF-035 resolved, `0de223c`), so new entries should not need this flag; a remaining keyword-less behavior is a new spec finding.
- `domain` – `messaging` or `service-container` (graded, `levels` L1–L3 required) or `platform` (not graded, no `levels`).
- `levels` – obligation per compliance level; `none` = not required at this level.
- `scope` – for umbrella entries (e.g. the level-table rows of MG 1.3): the sections whose detailed requirements they grade.
- `refs`, `quotes` – guideline sections and verbatim text (checked against the guideline by `spectool lint`).
- `verification` – `schema`, `d7-validate`, `d7-monitor`, `d7-test`, `d8` or `manual`.
- `inventory` – keys of the inventory items the entry covers.

## YAML subset

All YAML files are JSON-compatible: one document, no anchors/aliases, no tags, no merge keys, string keys only, decimal numbers only, `true`/`false`/`null` only; values that YAML 1.1 parsers read differently (`yes`, `no`, `on`, `off`, dates) must be quoted. `spectool` rejects everything else.

## spectool

Requires Go (the module pins its toolchain; `GOTOOLCHAIN=auto` downloads it). Run from `oec/spec/tools/`:

```sh
go run ./cmd/spectool inventory -spec ..   # regenerate coverage/inventory.yaml from the guideline HEAD (-rev to set the baseline)
go run ./cmd/spectool lint -spec ..        # schema + rules; checks refs and quotes against the guideline
go run ./cmd/spectool render -spec ..      # regenerate requirements/view/*.md, matrix/view/*.md, vectors/view/*.md
go run ./cmd/spectool coverage -spec ..    # regenerate coverage/report.md and coverage/vectors.md (-gate: P2 exit gate)
go run ./cmd/spectool topic -spec ..       # regenerate topic/topic-patterns.json
go run ./cmd/spectool topic -spec .. 'Oi4v2/…'   # analyze topics (grammar rule, Source/Filter, IL check, warnings)
go run ./cmd/spectool review -spec .. REQ-QRY-001..004 QRY V-PAY-053,V-PAY-054 Pub/MAM   # PO accepted: draft -> reviewed, then render + coverage + lint
go run ./cmd/spectool rebaseline -spec .. -rev <commit>   # move to a new guideline commit (see "After a guideline change")
go run ./cmd/spectool crosscheck -spec ..  # oracle vs schema on every case of ../schema/dist/cases.json (run ../schema/check.sh first)
go test ./...
```

`lint` also loads `topic.abnf` and the matrix and checks that every allowed matrix row has a grammar rule and – unless the area of its resource is still `planned` – consistent payload columns with known requirements (valid payload vectors must match them), that grammar, interpreter and generated regexes agree (random samples) and that every vector still yields its expected result with the reference code (`internal/topic`, `internal/il`, `internal/payload`); golden vectors must appear verbatim in the cited guideline section.

`review` takes requirement and vector ids, area codes (`drafted` → `reviewed`), matrix rows `Method/Resource` (payload column) or `matrix` (all payload columns); ranges `REQ-QRY-001..004` and comma lists are expanded. It refuses unknown ids, unexpected states (e.g. a `planned` area) and an area that would keep `draft` requirements, and writes nothing then; `-to draft` reopens entries, `-n` is a dry run. Edits are line based – comments and formatting stay as they are; header comments that describe the review state are still updated by hand.

Every command accepts `-check` (where it generates files) to fail instead of writing, for CI. The guideline repository defaults to `<spec>/../../oi4-oec-documentation`; override with `-guideline`.

## Topic grammar and regexes

`topic.abnf` is the source; the regexes are generated, never edited. They use the common subset of ECMA-262 (compile with the `u` flag), Go RE2, Python `re` and Rust `regex`: anchored with `^…$`, non-capturing groups, `\xHH` escapes, no look-around or back-references. In Python use `re.fullmatch` (`$` also matches before a final line break). The regexes check the syntax only – `AppId` and `Source` additionally need the Identification Link check (MG 4.1: decode base64url, check characters, authority, length).

## Vectors

One file per kind, `kind` + `baseline` + `description` + `vectors[]` (`vectors.schema.json`). Every vector has an id (`V-TOP-`, `V-IL-`, `V-DNP-`, `V-OID-`, `V-PAY-`), `expected: valid|invalid`, `reqs` (illustrated) or `violates` (broken) REQ ids, `origin` (`guideline` with `ref`, or `constructed`) and `status` (`draft` until the PO confirms it, G-3.9). A golden vector whose guideline text had to be corrected keeps the original in `origin.original` and names the spec finding in `origin.sf`.

Payload vectors (`V-PAY-`, kind `payload`) hold the `topic` and the message as JSON text in `payload` (exactly one JSON value, no duplicate keys; numbers are compared as written). `internal/payload` checks the envelope of MG 8 against the topic – `ua-data`, `ua-metadata`, `ua-action-request`, `ua-action-response`: keys, `MessageType`, `PublisherId`/`MessageId`/`RequestorId`, `DataSetClassId` (MG A2), `DataSetWriterId`/`DataSetWriterName`/`WriterGroupName` (helper DataSets, list entries per GL-ADR-018), `Status`, `ResponseAddress`, NewDataSetWriterId – and reports every problem with the REQ id it violates. A valid vector must have no problem; an invalid vector lists exactly the reported REQ ids in `violates`. Optional `mode` selects the validation mode of MG 8 (SF-052): `strict` (default) reports every undefined key (REQ-MSG-001/REQ-ACT-003), `tolerant` ignores undefined keys (REQ-MSG-046). A vector with `object: DataSetMetaDataType` instead of `topic` holds a bare MetaData object (the predefined MetaData of MG B2). Since C-027 it also checks the DataSet content (only MUST rules): the helper DataSets PaginationRequest, Pagination and Locale with their placement per Method (areas PAG, LOC) for every resource, the generic helper Query (`DataSetWriterId` 9, only with `Get` of a resource with a defined Query, at most once, area QRY; the Query of ReferenceDesignation only without Source with `Parent`/`Aspect`, area RES-REFDES; GL-ADR-020), and the DataSets of MAM, Health, Profile, PublicationList and ReferenceDesignation with the payload rules of their Methods (areas RES-*); since C-028 also the Oi4Pv DataSet and the Get/Set rules of Data, the empty MetaData of a Get Metadata, the Event categories Status/Syslog/Generic with Category and EventLevel against the WriterGroupName, and the SubscriptionList entries (fields, WriterGroupName derived from the TopicPath, Get/Set/Del with exactly one entry); Config follows in C-029. Valid payload vectors with a topic must also match the payload columns of their matrix row (MessageType, number of resource DataSetMessages, helper DataSets). A golden payload vector that differs from its example keeps the verbatim example in `original` and lists every change in `edits` (`target` = `topic` or a JSON Pointer, `kind` = `removed`/`completed`/`instantiated`/`corrected`, `reason`, optional `sf`); `removed` targets point into the original, all others into the payload. Without `original`, topic and payload must appear verbatim (whitespace-normalized) in the cited guideline section; a topic the example does not name is added with an edit `{target: topic, kind: completed}` (e.g. the MG 1.2 and MG 8.1.x examples, C-031).

broker.json vectors (`V-BRK-`, kind `broker-json`, C-039) hold the content of the file `broker.json` of the Message Bus storage (RIG 4.1.1) as JSON text in `content`. `internal/platform` checks REQ-CNT-003: a JSON object with `Address` (IPv4 address, IPv6 address without brackets/zone or RFC 1123 host name whose last label is not all digits), `SecurePort` (integer 1 … 65535) and `MaxPacketSize` (integer 262144 … 268435455); further elements are allowed (SF-100, RIG 4.1.1 since guideline `168e1d1`). All of them are constructed (RIG 4.1.1 has no example); REQ-CNT-003 keeps the verification `d8`, so they do not count in the vector coverage of `schema`/`d7-validate` REQs.

Guideline examples (C-031): `spectool coverage` extracts every fenced code block of MG and RIG (including Appendix B; key `<Doc>-<Section>-<hash>` from the normalized content, like the inventory keys). A block is covered if the payload (or `original`) of a golden payload vector of the same section appears in it, or if it is a JSON object/array – or a list of object members such as `"Payload": {…}` – contained in a golden payload vector. Every other block needs an entry in `coverage/examples.yaml` (keys + reason). `coverage -check` fails on open blocks, stale keys and explanations of covered blocks.

Vector coverage (C-034, ADR-0019 §8/§9): `spectool coverage` also writes `coverage/vectors.md` for every non-deprecated REQ whose verification contains `schema` or `d7-validate`. A REQ is **covered** if a reviewed valid vector names it in `reqs` and a reviewed invalid vector in `violates`; an umbrella of `coverage/implications.yaml` is **derived** if all its details are covered or derived. The layer of a negative payload vector is `schema`, or `rule` if the expectation list `../schema/tests/rule-layer.yaml` puts the REQ into its rule part; topic and IL vectors are `R-1`. A valid topic vector with the warning `whitespace` is a negative of REQ-TOP-020 (layer `warning`, a SHOULD that D7 reports as a warning; ADR-0019 amendment C-040) and not a positive of it. The report also lists the entry schema `../schema/v2/topics/<rule>.schema.json` of every allowed matrix row. `coverage -check` fails on structural problems (unknown or deprecated umbrella/detail, an umbrella without `schema`/`d7-validate`, cycles; an expectation whose `schema` + `rule` differs from `violates`, a rule REQ without `d7-validate`, a schema REQ without `schema`). `-gate` (P2 exit) also fails on every REQ that is neither covered nor derived and on every allowed row without an entry schema.

## Workflow per area

1. `areas.yaml` assigns guideline sections to the area (longest prefix wins).
2. Every inventory item of these sections is either mapped by a requirement (`inventory`) or listed in `coverage/informative.yaml` with a reason.
3. `lint`, `render`, `coverage` pass (including the guideline examples of the sections); the area status becomes `drafted`.
4. The PO reviews `requirements/view/<area>.md` (and, for areas with a grammar or vectors, `matrix/view/method-resource.md` and `vectors/view/*.md`); confirmed entries and the area become `reviewed` (`spectool review`).

After a guideline change: `spectool rebaseline -rev <commit>` (the commit must be checked out in the guideline repository, without uncommitted changes in `docs/` and `rfcs_adrs/`). It regenerates the inventory with the new baseline, updates `baseline` in all files (always quoted, e.g. `baseline: "2452494"` – an all-digit short hash is otherwise parsed as a YAML number) and the header of `topic.abnf`, regenerates topic patterns, views and coverage report and runs `lint`. Remap the keys that `lint` reports as missing, sync golden vectors whose example changed and update `coverage/examples.yaml` for the blocks that `coverage` reports; the command lists the living project documents that still name the old baseline. If the guideline changed nothing in `docs/` or `rfcs_adrs/` since the baseline (e.g. a register-only commit), it stops: no re-baseline is needed (ADR-0001 amendment C-018; `-force` overrides).
