# `oec/spec/` – OEC v2 spec baseline

Machine-readable baseline derived **only** from the OEC guideline (Messaging Guideline "MG", Runtime & Infrastructure Guideline "RIG", guideline ADRs "GL-ADR"). It is the shared contract of the JSON Schemas (D1), the SDKs (D2, D9), the compliance CLIs (D7, D8) and the documentation (D6). See ADR-0018.

| Path | Content | Edited by |
|---|---|---|
| `requirements/areas.yaml` | area registry: code, title, guideline sections, cycle, status | hand |
| `requirements/<area>.yaml` | requirements `REQ-<AREA>-NNN` of one area | hand |
| `requirements/*.schema.json` | JSON Schema 2020-12 of the files above | hand |
| `requirements/view/*.md` | Markdown view per area – the PO review artifact | `spectool render` |
| `coverage/inventory.yaml` | all candidates of normative statements in MG + RIG (keyword, field, admonition, implicit, table) | `spectool inventory` |
| `coverage/informative.yaml` | inventory items that are no requirement, with reason (MG 1.2) | hand |
| `coverage/report.md` | coverage of the inventory per area | `spectool coverage` |
| `topic/topic.abnf` | formal topic grammar (RFC 5234 + RFC 7405), one rule per allowed Method × Resource form | hand |
| `topic/topic-patterns.json` | anchored regexes generated from `topic.abnf` (portable dialect, see below) | `spectool topic` |
| `matrix/method-resource.yaml` (+ schema) | Method × Resource matrix: allowed, `Source`/`Filter` required/optional/forbidden, ABNF rule, guideline refs | hand |
| `matrix/view/method-resource.md` | Markdown view of the matrix: overview grid, topic form per combination, linked vectors – PO review artifact | `spectool render` |
| `vectors/` (+ `vectors.schema.json`) | language-neutral test vectors: `topic/` (golden from the guideline, constructed valid, invalid), `il/` (Identification Link, dnp-encoding, `Oi4Identifier`), `payload/` (topic + JSON message, golden from Appendix B) | hand, checked by `spectool lint` |
| `vectors/view/*.md` | Markdown view per vector file (overview table + one section per vector with decoded Identification Links and linked REQs) – PO review artifact | `spectool render` |
| `tools/` | Go module with `spectool` | – |

## Citing requirements

Use the id, e.g. `REQ-CMP-003`. Ids are never reused or renumbered; a replaced entry gets `status: deprecated` and `replaced_by`. Entries in `draft` are not yet confirmed by the PO (GUARDRAILS G-3.9).

Fields of an entry (`requirements/catalog.schema.json`):

- `statement` – one atomic requirement in our words, written in present tense; the obligation is carried by `keyword` and `levels`, the guideline wording by `quotes`.
- `keyword` – MUST / MUST NOT / SHOULD / SHOULD NOT / MAY as mapped by MG 1.2; for graded domains it equals the Level 3 obligation.
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
go run ./cmd/spectool coverage -spec ..    # regenerate coverage/report.md
go run ./cmd/spectool topic -spec ..       # regenerate topic/topic-patterns.json
go run ./cmd/spectool topic -spec .. 'Oi4v2/…'   # analyze topics (grammar rule, Source/Filter, IL check, warnings)
go test ./...
```

`lint` also loads `topic.abnf` and the matrix and checks that every allowed matrix row has a grammar rule, that grammar, interpreter and generated regexes agree (random samples) and that every vector still yields its expected result with the reference code (`internal/topic`, `internal/il`, `internal/payload`); golden vectors must appear verbatim in the cited guideline section.

Every command accepts `-check` (where it generates files) to fail instead of writing, for CI. The guideline repository defaults to `<spec>/../../oi4-oec-documentation`; override with `-guideline`.

## Topic grammar and regexes

`topic.abnf` is the source; the regexes are generated, never edited. They use the common subset of ECMA-262 (compile with the `u` flag), Go RE2, Python `re` and Rust `regex`: anchored with `^…$`, non-capturing groups, `\xHH` escapes, no look-around or back-references. In Python use `re.fullmatch` (`$` also matches before a final line break). The regexes check the syntax only – `AppId` and `Source` additionally need the Identification Link check (MG 4.1: decode base64url, check characters, authority, length).

## Vectors

One file per kind, `kind` + `baseline` + `description` + `vectors[]` (`vectors.schema.json`). Every vector has an id (`V-TOP-`, `V-IL-`, `V-DNP-`, `V-OID-`, `V-PAY-`), `expected: valid|invalid`, `reqs` (illustrated) or `violates` (broken) REQ ids, `origin` (`guideline` with `ref`, or `constructed`) and `status` (`draft` until the PO confirms it, G-3.9). A golden vector whose guideline text had to be corrected keeps the original in `origin.original` and names the spec finding in `origin.sf`.

Payload vectors (`V-PAY-`, kind `payload`) hold the `topic` and the message as JSON text in `payload` (exactly one JSON value, no duplicate keys; numbers are compared as written). `internal/payload` checks the envelope of MG 8 against the topic – `ua-data`, `ua-metadata`, `ua-action-request`, `ua-action-response`: keys, `MessageType`, `PublisherId`/`MessageId`/`RequestorId`, `DataSetClassId` (MG A2), `DataSetWriterId`/`DataSetWriterName`/`WriterGroupName` (helper DataSets, list entries per GL-ADR-018), `Status`, `ResponseAddress`, NewDataSetWriterId – and reports every problem with the REQ id it violates. A valid vector must have no problem; an invalid vector lists exactly the reported REQ ids in `violates`. The DataSet content of the resources (MG 8.3) is not checked yet (C-026/C-027). A golden payload vector that differs from its example keeps the verbatim example in `original` and lists every change in `edits` (`target` = `topic` or a JSON Pointer, `kind` = `removed`/`completed`/`instantiated`/`corrected`, `reason`, optional `sf`); `removed` targets point into the original, all others into the payload. Without `original`, topic and payload must appear verbatim (whitespace-normalized) in the cited Appendix B section.

## Workflow per area

1. `areas.yaml` assigns guideline sections to the area (longest prefix wins).
2. Every inventory item of these sections is either mapped by a requirement (`inventory`) or listed in `coverage/informative.yaml` with a reason.
3. `lint`, `render`, `coverage` pass; the area status becomes `drafted`.
4. The PO reviews `requirements/view/<area>.md` (and, for areas with a grammar or vectors, `matrix/view/method-resource.md` and `vectors/view/*.md`); confirmed entries and the area become `reviewed`.

After a guideline change: regenerate the inventory with the new baseline, update `baseline` in all files (always quoted, e.g. `baseline: "2452494"` – an all-digit short hash is otherwise parsed as a YAML number), and remap the keys that `lint` reports as missing.
