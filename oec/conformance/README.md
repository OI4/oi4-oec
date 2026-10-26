# `oec/conformance/` – D7/D8 compliance CLIs

> **State: D7 `validate` (C-040).** One Go module (ADR-0013/0014) for the application and messaging compliance CLI **`oec-check`** (D7) and, from P5b, the infrastructure compliance CLI `oec-infra` (D8). It depends only on [`../spec/`](../spec/) (data files via `spec.FS`) and [`../schema/`](../schema/) (entry schemas via `schema.FS`), never on `sdk/` or on the internals of `spec/tools` (ADR-0006/0018/0019 §5).

## `oec-check validate`

Checks messages offline against the embedded baseline (ADR-0019 §5 and amendment C-040):

| Layer | Checks |
|---|---|
| `topic` (R-1) | topic grammar of `topic-patterns.json` and the Method × Resource matrix, diagnosed level by level (REQ-TOP-…, REQ-ACT-020); Identification Link of AppId and Source (REQ-IL-…); warnings for control characters and whitespace in Filter levels (`whitespace` = recommendation of REQ-TOP-020 not followed; the text report names the REQ) |
| `parse` | strict JSON: UTF-8, one value, no duplicate keys; reported as REQ-CMP-005 (OPC UA-conform JSON, ADR-0019 amendment C-040) |
| `schema` | the entry schema of the topic form (`topics/<rule>.schema.json`), `message.schema.json` or the bare `metadata.schema.json#/$defs/ResourceMetaData`; strict or tolerant flavor; each error is mapped to the REQ ids of its `$comment` |
| `rule` | R-2 equality with topic values, R-3 relations inside one message (incl. the heuristic check for an encoded Identification Link in the payload, REQ-IL-005), R-4 number and calendar semantics (mutation probes against the schema), R-6 declared role (`--role`) |

```sh
cd oec/conformance
go run ./cmd/oec-check validate --topic 'Oi4v2/…/Pub/Health/…' payload.json
go run ./cmd/oec-check validate --object DataSetMetaDataType metadata.json
go run ./cmd/oec-check validate --format json messages.jsonl captures/
go run ./cmd/oec-check version
```

A message file holds one or more message objects `{"topic": "…", "payload": {…}}` with the optional keys `role` (`birth`, `close`, `will`), `object` and `tolerant` (one object, JSONL or concatenated objects); a directory is searched for `*.json` and `*.jsonl`. Without a file, standard input is read.

Every result names the tool version, the version of the embedded schema bundle (`schemas`, e.g. `2.0.0`) and the guideline baseline (ADR-0019 §6, amendment C-041). In the JSON report these are the keys `version`, `schemas` and `baseline`. D7 refuses to start when the bundle and the spec data come from different baselines.

Exit codes: `0` every message is valid (warnings allowed), `1` at least one message violates a requirement, `2` usage or input error, `3` internal error.

## Layout

| Path | Content |
|---|---|
| `cmd/oec-check` | CLI (D7) |
| `internal/topic` | staged topic diagnosis over the generated patterns and the matrix |
| `internal/ilink` | Identification Link checks, base64url elements, dnp encoding |
| `internal/schemas` | schema set (strict + tolerant), REQ mapping of schema errors, R-4 probes |
| `internal/rules` | rule layer R-2, R-3, R-6 |
| `internal/validate` | one message: topic → parse → schema → rules |
| `internal/report` | text and JSON report |
| `internal/testvec` | test helper: reads the vectors of `../spec/vectors/` |

## Tests

`go test ./...` checks D7 against the vectors of the baseline: every topic, IL and dnp vector; every payload vector (valid → no finding, invalid → exactly its `violates`); the split between schema layer and rule layer of [`../schema/tests/rule-layer.yaml`](../schema/tests/rule-layer.yaml); R-4 probes; CLI exit codes. The Go toolchain is pinned in `go.mod` (`GOTOOLCHAIN=auto` with an older local Go).

The module is pure Go without cgo; `GOOS=windows|linux|darwin go build ./cmd/oec-check` cross-compiles (checked in C-040 for windows/amd64, linux/amd64, linux/arm64 and darwin/arm64; tests run on macOS so far, CI on Linux/Windows comes with P4).
