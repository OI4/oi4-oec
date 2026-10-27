# `oec/conformance/` – D7/D8 compliance CLIs

> **State: D7 `validate` (C-040); test catalog and architecture design (C-044, [`DESIGN.md`](DESIGN.md), ADR-0025); `monitor` live and `--replay` with 13 of 19 monitor tests (C-052, ADR-0025 amendment C-052).** One Go module (ADR-0013/0014) for the application and messaging compliance CLI **`oec-check`** (D7) and, from P5b, the infrastructure compliance CLI `oec-infra` (D8). It depends only on [`../spec/`](../spec/) (data files via `spec.FS`) and [`../schema/`](../schema/) (entry schemas via `schema.FS`), never on `sdk/` or on the internals of `spec/tools` (ADR-0006/0018/0019 §5).

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

A message file holds a capture of `monitor --record` or one or more message objects `{"topic": "…", "payload": {…}}` with the optional keys `role` (`birth`, `close`, `will`), `object` and `tolerant` (one object, JSONL or concatenated objects); a directory is searched for `*.json` and `*.jsonl`. Without a file, standard input is read.

Every result names the tool version, the version of the embedded schema bundle (`schemas`, e.g. `2.0.0`) and the guideline baseline (ADR-0019 §6, amendment C-041). In the JSON report these are the keys `version`, `schemas` and `baseline`. D7 refuses to start when the bundle and the spec data come from different baselines.

Exit codes: `0` every message is valid (warnings allowed), `1` at least one message violates a requirement, `2` usage or input error, `3` internal error.

## `oec-check monitor`

Observes the `Oi4v2/#` namespace passively (DESIGN.md §3.2) – live over MQTT 3.1.1 with TLS 1.2/1.3, or offline from a capture – and judges every message with the four layers of `validate` plus the stateful monitor rules of [`catalog/d7.yaml`](catalog/d7.yaml) (`T7-MON-01 … 08, 10, 11, 12, 18, 19`: origin, Birth/Will/Close order, Birth = MAM, Sources, heartbeat, Call/Response correlation, MaxPacketSize, Debug before Set, …).

```sh
# live against the dev broker of oec/dev, with the container storage of a sample root
go run ./cmd/oec-check monitor --bus-storage ../dev/generated/roots/oec-dev-cert \
    --duration 3m --record run.jsonl --app 'provider.com/FieldDataService/FDS-001/1200-0345'
# live with explicit connection flags
go run ./cmd/oec-check monitor --broker mqtts://localhost:8883 --ca ca.pem \
    --username probe --password-file password.txt --duration 5m
# offline: replay a recording (deterministic)
go run ./cmd/oec-check monitor --replay run.jsonl --format json --report junit=monitor.xml
```

- Connection: `--broker mqtts://host:port`, `--ca`, `--username` + `--password-file`, `--cert` + `--key` (+ `--key-passphrase-file`, encrypted PKCS#8 supported), `--client-id`, or `--bus-storage ROOT` (the RIG container storage: `etc/oi4/mqtt/broker.json`, CA, client certificate, `run/secrets/oi4/…`); explicit flags override the storage. `--insecure` skips the broker certificate check – dev only, warned on stderr and marked in capture and report.
- Run: `--duration` (default 5m; the heartbeat needs at least 2 × `--heartbeat` + `--tolerance` = 125 s), `--app` (AppId or `ServiceType/AppId` of the component under test; without it every participant is judged), `--level` (1–3, default 3), `--allow-skip`, `--max-packet-size`, `--config FILE.yaml` (keys = flag names, command line wins).
- Output: `--format text|json|junit` on stdout, `--report FORMAT=PATH` (repeatable). The report lists the participants and per test the per-REQ results with evidence (capture line, topic, JSON pointer).
- Capture (`--record`, `--replay`): JSONL, header line + one record `{"topic", "payload", "size", "received", "qos", "retain"}` per message + trailer (DESIGN.md §3.2); `validate` reads it as a message file.

Exit codes: `0` no failure, `1` at least one `fail`, `2` usage error, `3` internal error or broker unreachable, `4` inconclusive – a MUST could not be judged (observation too short, CUT not seen); `--allow-skip` turns 4 into 0.

## Layout

| Path | Content |
|---|---|
| `cmd/oec-check` | CLI (D7) |
| `internal/topic` | staged topic diagnosis over the generated patterns and the matrix |
| `internal/ilink` | Identification Link checks, base64url elements, dnp encoding |
| `internal/schemas` | schema set (strict + tolerant), REQ mapping of schema errors, R-4 probes |
| `internal/rules` | rule layer R-2, R-3, R-6 |
| `internal/validate` | one message: topic → parse → schema → rules |
| `internal/capture` | capture format (JSONL), recorder, reader |
| `internal/mqttwire` | MQTT 3.1.1 client over TLS on `paho.mqtt.golang` v1.5.1 (EDL-1.0), PKCS#8 PBES2 keys, container storage reader |
| `internal/mqttwire/mqtttest` | test-only in-process TLS broker and throw-away PKI (gate without network) |
| `internal/monitor` | stateful monitor engine: participants, rules T7-MON-xx |
| `internal/verdict` | verdict model: severity from REQ keyword × level, test aggregation, exit code |
| `internal/report` | text and JSON report of `validate`; text, JSON and JUnit report of `monitor` |
| `internal/testvec` | test helper: reads the vectors of `../spec/vectors/` |
| `testdata/captures` | capture fixtures of the monitor rules + `index.yaml` with expected verdicts (reviewed in the C-052 review, G-3.9) |
| `catalog` | test catalogs of `monitor`/`test` (D7, `d7.yaml`) and `oec-infra` (D8, `d8.yaml`), embedded; `catalog_test.go` enforces mode ↔ verification, levels and full coverage of every `d7-monitor`/`d7-test`/`d8` REQ; `independence_test.go` keeps the module free of `sdk/` (Q-009) |

## Tests

`go test ./...` checks D7 against the vectors of the baseline: every topic, IL and dnp vector; every payload vector (valid → no finding, invalid → exactly its `violates`); the split between schema layer and rule layer of [`../schema/tests/rule-layer.yaml`](../schema/tests/rule-layer.yaml); R-4 probes; CLI exit codes; the test catalogs against the requirements catalog (`catalog/`); the MQTT client against the loopback test broker (passwords, client certificates, Will, lost connection); every monitor rule with a positive and a negative capture fixture; `monitor` live against the loopback broker and from captures. The Go toolchain is pinned in `go.mod` (`GOTOOLCHAIN=auto` with an older local Go).

The module is pure Go without cgo; `GOOS=windows|linux|darwin go build ./cmd/oec-check` cross-compiles (checked in C-040 for windows/amd64, linux/amd64, linux/arm64 and darwin/arm64; tests run on macOS so far, CI on Linux/Windows comes with P4).

`./live.sh` (opt-in, needs Docker; not part of `oec/check.sh`) starts the dev broker of [`../dev/`](../dev/) and runs `TestMonitorDevBroker`: a simulated component publishes Birth, gets killed (the broker sends its Will) and reconnects, while `oec-check monitor` observes once with the client certificate of `oec-dev-cert` (port 8884) and once with the credentials of `oec-dev-app` (port 8883). `OEC_LIVE_OUT=DIR` keeps the captures, `OEC_LIVE_KEEP=1` the broker.
