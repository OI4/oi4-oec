# `oec/conformance/` – D7/D8 compliance CLIs

> **State: D7 `validate` (C-040); test catalog and architecture design (C-044, [`DESIGN.md`](DESIGN.md), ADR-0025); `monitor` live and `--replay` with 13 of 19 monitor tests (C-052, ADR-0025 amendment C-052); `test` with the bus attachment and T7-GET-01/02/05/09 (C-062, ADR-0025 amendment C-062), every `Get` test and T7-ADR-02/03 (C-063, amendment C-063), the write tests T7-SET-01 … 09, 11, 12 with restore (C-064, amendment C-064), the Action tests T7-ACT-01 … 04 and the monitor rules T7-MON-09, 13, 15, 16, 17 (18 of 19, C-065, amendment C-065), the proxy attachment with T7-CON-01 … 04 and T7-LIFE-01 … 03 (C-066, amendment C-066), the restart hook with T7-LIFE-04 and T7-SET-10 and the monitor rule T7-MON-14 (19 of 19, C-067, amendment C-067), the peer simulation, `--announce` and T7-ADR-01 (C-068, amendment C-068) – all 54 D7 tests implemented.** One Go module (ADR-0013/0014) for the application and messaging compliance CLI **`oec-check`** (D7) and, from P5b, the infrastructure compliance CLI `oec-infra` (D8). It depends only on [`../spec/`](../spec/) (data files via `spec.FS`) and [`../schema/`](../schema/) (entry schemas via `schema.FS`), never on `sdk/` or on the internals of `spec/tools` (ADR-0006/0018/0019 §5).

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

Observes the `Oi4v2/#` namespace passively (DESIGN.md §3.2) – live over MQTT 3.1.1 with TLS 1.2/1.3, or offline from a capture – and judges every message with the four layers of `validate` plus the stateful monitor rules of [`catalog/d7.yaml`](catalog/d7.yaml) (`T7-MON-01 … 19`: origin, Birth/Will/Close order, Birth = MAM, Sources, heartbeat, Call/Response correlation, DataSetClassId per structure, Data/Event support, republication after a change, RevisionCounter, mandatory Config objects in a Set, MaxPacketSize, Debug before Set, the publication rules of the PublicationList (Mode, Interval, Precisions; since C-067), …).

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

## `oec-check test`

Tests one running component (CUT) actively (DESIGN.md §3.3): D7 sends `Get`, `Set` and `Del` requests as the not-announced tester `Utility/oi4.org/oec-check/D7/<random>` and judges the answers; the monitor rules run on the same subscription. Implemented: T7-GET-01 (Get MAM/Health with `ReplyTo`, all Sources), T7-GET-02 (Profile lists exactly the served resources of the level), T7-GET-05 (empty list, unknown Source → Event Status Bad), T7-GET-09 (unknown keys ignored); since C-063 also T7-GET-03 (every resource with and without Source and Filter, PublicationList entries, Data ⇒ Metadata), T7-GET-04 (every represented entity fulfills the level), T7-GET-06 (pagination on request; by size passively, T7-MON-18), T7-GET-07 (Query of ReferenceDesignation), T7-GET-08 (Locale), T7-ADR-02 (no answer in the namespace `Oi4`) and T7-ADR-03 (exact Identification Links); since C-064 the write tests T7-SET-01 … 09, 11, 12 (`Set`/`Del` ⇒ Status event with `ReplyTo` and publication; Config, Data, Metadata, PublicationList, ReferenceDesignation, SubscriptionList, log level); since C-065 the Action tests T7-ACT-01 … 04 (`Call NewDataSetWriterId` ⇒ one final `ua-action-response` on the ResponseAddress with CorrelationData, new ids while reserved (Ttl), no answer to unaddressable or undecodable requests, BadNotFound for an unknown Source); since C-066 with the proxy attachment the connection and lifecycle tests T7-CON-01 … 04 (MQTT 3.1.1 CONNECT, ClientId, TLS ≥ 1.2 and certificate verification, authentication, no Retain) and T7-LIFE-01 … 03 (Will on every connection, Birth first after every connect and after recovery, Will published on an abnormal disconnect); since C-067 with the restart hook T7-LIFE-04 (Close message before a graceful disconnect) and T7-SET-10 (a changed ReferenceDesignation survives the restart). The other active tests follow the P5 plan (ROADMAP).

```sh
# against the SDK example app (oec/sdk/rust/oec/examples/demo.rs) on the dev broker;
# the elements of the Oi4Identifier are dnp-encoded ("OEC Demo" → "OEC,20Demo")
go run ./cmd/oec-check test --broker mqtts://localhost:8883 --ca ../dev/generated/broker/ca.pem \
    --username oec-probe --password-file ../dev/generated/probe/password \
    --app 'OTConnector/example.com/OEC,20Demo/DEMO-1/oec-dev-app' --level 2 --record test.jsonl
```

- `--app [ServiceType/]AppId` is required; with a bare AppId, D7 asks on every ServiceType. The connection, verdict and output flags are those of `monitor`; `--record` writes a capture that `monitor --replay` reads.
- Run: `--timeout` (default 10s: waiting time for an answer, and the silence that counts as "no answer"), `--tests ID|GLOB,…` (repeatable; the others are skipped as "not selected"), `--tester-il IL`.
- A test run is short: passive requirements that need a long observation (heartbeat, connect, MaxPacketSize) are skipped with the reason "test run: …"; a level claim needs a `monitor` run of at least 125 s as well.
- **`test` changes the CUT.** The write tests write back what the CUT published, delete only what D7 created and undo every change they provoke. Before the first and after the last write test D7 reads Config, PublicationList, ReferenceDesignation and SubscriptionList; if they differ, `test` warns on stderr and lists the differences (report key `restore`); it also warns if the run ended during the write tests. A ReferenceDesignation is deleted only after its write-back was accepted; at level 1 a Config DataSet with a masked sensitive value is not written. Run it only against a component whose configuration may be written.
- **Proxy attachment** (DESIGN.md §3.4, Q-028): `--proxy-listen HOST:PORT --proxy-ca FILE --proxy-ca-key FILE [--proxy-name NAME,…] [--proxy-wait 2m]`. The CUT connects to D7 instead of the broker (its `broker.json` points to the proxy, its `ca.pem` is the proxy CA); D7 issues its TLS certificates from that CA (names `localhost`, `127.0.0.1`, `::1` or `--proxy-name`) and relays to `--broker` with its own TLS settings and the CUT's CONNECT (credentials included). D7 waits up to `--proxy-wait` for the CUT's connection, Birth and first subscription, runs the other tests, then answers the next three connection attempts with TLS 1.1 only, an untrusted CA and a wrong host name (the CUT must refuse them), drops the relayed connection (Will) and waits for the reconnection with a Birth. Without the proxy T7-CON-* and T7-LIFE-* are skipped and the run is inconclusive (exit 4). A client certificate of the CUT is seen but not relayed. The proxy CA and key are test material only – generate them in a temporary directory.

```sh
# demo through the proxy: the copy of the dev root trusts proxy-ca.pem and its broker.json
# points to localhost:18883
go run ./cmd/oec-check test --broker mqtts://localhost:8883 --ca ../dev/generated/broker/ca.pem \
    --username oec-probe --password-file ../dev/generated/probe/password \
    --app 'OTConnector/example.com/OEC,20Demo/DEMO-1/oec-dev-app' --level 2 \
    --proxy-listen 127.0.0.1:18883 --proxy-ca /tmp/x/proxy-ca.pem --proxy-ca-key /tmp/x/proxy-ca.key
```

- **Restart hook** (DESIGN.md §3.4): `--restart-cmd CMD`, or `--stop-cmd CMD --start-cmd CMD`, and `[--restart-wait 2m]`. Shell commands (`sh -c`, Windows `cmd /C`) that stop the CUT gracefully (SIGTERM, `docker stop`, …) and start it again; a command that starts the CUT in the background must redirect its output, else D7 waits up to 10 s for it. In the write window, after the other write tests, T7-SET-10 sets a changed ReferenceDesignation, D7 runs the hook and waits up to `--restart-wait` for the new Birth (through the proxy also the first SUBACK) and an answer to Get Health; then T7-LIFE-04 judges the last Pub Health before the DISCONNECT (proxy) or before the new Birth (bus) as the Close message, and T7-SET-10 reads the ReferenceDesignation again and restores it. Without the hook both tests are skipped and the run is inconclusive (exit 4); a stop without DISCONNECT (Will) is inconclusive as well.

```sh
# demo through the proxy with the restart hook (stop.sh: SIGTERM to the demo and wait for its end;
# start.sh: start it again in the background with its output redirected)
go run ./cmd/oec-check test … --proxy-listen 127.0.0.1:18883 --proxy-ca /tmp/x/proxy-ca.pem --proxy-ca-key /tmp/x/proxy-ca.key \
    --stop-cmd /tmp/x/stop.sh --start-cmd /tmp/x/start.sh
```

- **Peer and `--announce`** (DESIGN.md §3.4, §3.5, Q-029): `--peer` announces a simulated second component `Utility/oi4.org/oec-check/Peer/<random>` on its own connection (Birth, a Health once the CUT is found, heartbeat every 30 s, Close message, Will) and answers its Get MAM, Health and Profile. T7-ADR-01 checks every request of the CUT to the peer: on the peer's request topic, with its ServiceType and AppId (REQ-BUS-002, REQ-TOP-006, REQ-TOP-010). If the CUT sends no request to the peer, the test is skipped; **without `--peer` it is skipped as inconclusive (exit 4)**. `--announce` announces D7 itself the same way under the tester IL (`--tester-il` must then be an Oi4Identifier); by default D7 is not announced.

```sh
# the full run against the demo: proxy, restart hook, peer, D7 announced
go run ./cmd/oec-check test … --proxy-listen 127.0.0.1:18883 --proxy-ca /tmp/x/proxy-ca.pem --proxy-ca-key /tmp/x/proxy-ca.key \
    --stop-cmd /tmp/x/stop.sh --start-cmd /tmp/x/start.sh --peer --announce
```

- The report is the monitor report with `"mode": "test"` plus `tester`, `requests`, `restore` and `announced` (each announced component: role, PublisherId, requests received and answered); with the proxy `attachment` is `"proxy"`, plus `proxy_listen` and `connections` (each attempt: phase, TLS version, CONNECT without password, CONNACK, PUBLISH topics and times, first SUBACK, drop). Exit codes as for `monitor`.

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
| `internal/probe` | active test runner of `test`: discovery, requests with `ReplyTo` correlation, tests T7-GET-xx/ADR-xx/SET-xx/ACT-xx/CON-xx/LIFE-xx, state snapshot and restore check, restart phase, announced components (peer, `--announce`), verdict of a test run |
| `internal/proxy` | proxy attachment: TLS listener with certificates from the proxy CA, minimal MQTT 3.1.1 relay (strict CONNECT/PUBLISH decoder), connection phases |
| `internal/sim` | test-only simulated CUT with fault injection (DESIGN.md §10) |
| `internal/verdict` | verdict model: severity from REQ keyword × level, test aggregation, exit code |
| `internal/report` | text and JSON report of `validate`; text, JSON and JUnit report of `monitor` and `test` |
| `internal/testvec` | test helper: reads the vectors of `../spec/vectors/` |
| `testdata/sim` | `index.yaml`: expected verdicts of `test` against the simulated CUT per fault (C-062 … C-064 reviewed in the C-062, C-063 and C-064 reviews, C-065 … C-067 in the C-067 review; C-068: 3 new entries and the changed `none` are `draft`; new entries start as `draft`, G-3.9) |
| `testdata/captures` | capture fixtures of the monitor rules + `index.yaml` with expected verdicts (C-052 reviewed in the C-052 review, C-065 and C-067 in the C-067 review; new entries start as `draft`, G-3.9) |
| `catalog` | test catalogs of `monitor`/`test` (D7, `d7.yaml`) and `oec-infra` (D8, `d8.yaml`), embedded; `catalog_test.go` enforces mode ↔ verification, levels and full coverage of every `d7-monitor`/`d7-test`/`d8` REQ; `independence_test.go` keeps the module free of `sdk/` (Q-009) |

## Tests

`go test ./...` checks D7 against the vectors of the baseline: every topic, IL and dnp vector; every payload vector (valid → no finding, invalid → exactly its `violates`); the split between schema layer and rule layer of [`../schema/tests/rule-layer.yaml`](../schema/tests/rule-layer.yaml); R-4 probes; CLI exit codes; the test catalogs against the requirements catalog (`catalog/`); the MQTT client against the loopback test broker (passwords, client certificates, Will, lost connection); every monitor rule with a positive and a negative capture fixture; `monitor` live against the loopback broker and from captures; every implemented `test` test with a positive and a negative run against the simulated CUT, and `oec-check test` against it. The Go toolchain is pinned in `go.mod` (`GOTOOLCHAIN=auto` with an older local Go).

The module is pure Go without cgo; `GOOS=windows|linux|darwin go build ./cmd/oec-check` cross-compiles (checked in C-040 for windows/amd64, linux/amd64, linux/arm64 and darwin/arm64; tests run on macOS so far, CI on Linux/Windows comes with P4).

`./live.sh` (opt-in, needs Docker; not part of `oec/check.sh`) starts the dev broker of [`../dev/`](../dev/) and runs `TestMonitorDevBroker`: a simulated component publishes Birth, gets killed (the broker sends its Will) and reconnects, while `oec-check monitor` observes once with the client certificate of `oec-dev-cert` (port 8884) and once with the credentials of `oec-dev-app` (port 8883). `OEC_LIVE_OUT=DIR` keeps the captures, `OEC_LIVE_KEEP=1` the broker.
