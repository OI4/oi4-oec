# `oec/conformance/` – D7/D8 compliance CLIs

> **State: D7 `validate` (C-040); test catalog and architecture design (C-044, [`DESIGN.md`](DESIGN.md), ADR-0025); `monitor` live and `--replay` with 13 of 19 monitor tests (C-052, ADR-0025 amendment C-052); `test` with the bus attachment and T7-GET-01/02/05/09 (C-062, ADR-0025 amendment C-062), every `Get` test and T7-ADR-02/03 (C-063, amendment C-063), the write tests T7-SET-01 … 09, 11, 12 with restore (C-064, amendment C-064), the Action tests T7-ACT-01 … 04 and the monitor rules T7-MON-09, 13, 15, 16, 17 (18 of 19, C-065, amendment C-065), the proxy attachment with T7-CON-01 … 04 and T7-LIFE-01 … 03 (C-066, amendment C-066), the restart hook with T7-LIFE-04 and T7-SET-10 and the monitor rule T7-MON-14 (19 of 19, C-067, amendment C-067), the peer simulation, `--announce` and T7-ADR-01 (C-068, amendment C-068) – all 55 D7 tests implemented; `list`, `explain`, `report-schema`, the frozen report schema version 1 and the catalog coverage report (C-069, amendment C-069). D8: `oec-infra image` with T8-IMG-01 … 04 and its report schema (C-070, amendment C-070; P5b plan in the ROADMAP), `oec-infra bus` with T8-BUS-01 … 03 (C-071, amendment C-071).** One Go module (ADR-0013/0014) for the application and messaging compliance CLI **`oec-check`** (D7) and the infrastructure compliance CLI **`oec-infra`** (D8, P5b). It depends only on [`../spec/`](../spec/) (data files via `spec.FS`) and [`../schema/`](../schema/) (entry schemas via `schema.FS`), never on `sdk/` or on the internals of `spec/tools` (ADR-0006/0018/0019 §5).

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

## `oec-check list`, `explain`, `report-schema`

Views of the embedded test catalog ([`catalog/d7.yaml`](catalog/d7.yaml)), the requirements of `../spec/` and the fixture indexes; no network.

```sh
go run ./cmd/oec-check list                              # 55 tests: id, mode, level, status, needs, title, REQs
go run ./cmd/oec-check list --mode test --level 1 --needs none
go run ./cmd/oec-check list --req REQ-CMP-047 'T7-GET-*' --format json
go run ./cmd/oec-check explain T7-ADR-01                 # notes, REQs with keyword, levels and MG/RIG sources, fixtures
go run ./cmd/oec-check explain REQ-RES-CONFIG-025         # statement, sources, quotes, verification, tests, fixtures
go run ./cmd/oec-check report-schema > report.schema.json
```

- `list` filters: `--mode monitor|test`, `--level N` (tests that apply at level N, i.e. level ≤ N), `--req`, `--needs proxy|restart-hook|peer|none`, `--status`, id patterns (`T7-SET-*`); flags may follow the patterns. Unfiltered it ends with the totals and the `manual` requirements.
- `explain` takes one test or REQ id (case-insensitive); an unknown id is a usage error (exit 2). The fixtures are those of `testdata/sim/index.yaml` and `testdata/captures/index.yaml` with their stated outcome; positive fixtures are listed as "pass or not applicable".
- `report-schema` prints [`report.schema.json`](report.schema.json) (JSON Schema 2020-12): report version 1 of `monitor` and `test`. Every report the gate produces (CLI tests, capture replays, sim fixture runs) is validated against it.
- [`catalog/COVERAGE.md`](catalog/COVERAGE.md) is the generated coverage report (tests by status and need, REQs per verification kind and how they are checked, fixtures per test); regenerate with `go test ./internal/explain -run TestCoverageReport -update`.

## `oec-infra image`

Offline inspection of an OCI image (D8, T8-IMG-01 … 04): OCI image format, supported platforms (linux on amd64, arm64/v8, arm/v7; multi-architecture), exactly one SBOM in `/opt/oi4/licenses` (`sbom.spdx.json` SPDX 2.x JSON or 3.x JSON-LD, `sbom.cdx.json` CycloneDX 1.x JSON; another version is inconclusive, RIG 4.1.4 does not restrict it) that lists every component with name, version, a stated license (and a purl or CPE, recommended), and a text for every license and license exception, SHA256 digests. No Docker daemon, no network.

```sh
docker save IMAGE -o image.tar                     # Docker 25 or later writes an OCI layout
skopeo copy docker://REF oci-archive:image.tar     # or: podman save --format oci-archive IMAGE -o image.tar
go run ./cmd/oec-infra image image.tar             # also an extracted OCI layout directory
go run ./cmd/oec-infra image --level 2 --format json --report junit=infra.xml image.tar
go run ./cmd/oec-infra report-schema > infra-report.schema.json
```

- The layers of every runnable platform image are applied (whiteouts, symlinks resolved inside the image); attestation manifests are skipped. A registry reference, a Docker archive with only `manifest.json` and a compressed tar are input errors (exit 2).
- Flags: `--level 1|2|3` (default 3; only T8-IMG-01 applies at level 1), `--format text|json|junit`, `--report FORMAT=PATH` (repeatable), `--allow-skip`. Exit codes as for `monitor`; a zstd layer cannot be read (exit 4, inconclusive). A digest finding of T8-IMG-04 only warns (REQ-CMP-030); a digest mismatch fails T8-IMG-01.
- The JSON report has its own schema [`infra-report.schema.json`](infra-report.schema.json) (report version 1, grows additively until the P5b exit).

## `oec-infra bus`

Probes a running Message Bus like an OEC application (D8, T8-BUS-01 … 03): MQTT 3.1.1 over TLS 1.2 or higher (raw handshakes for TLS 1.2, 1.3 and the refused 1.0/1.1, with the client certificate of the run), no plain-text MQTT listener (`--plain-port`, default 1883), the broker certificate (chain to `ca.pem` / `broker_ca.pem`, not self-signed, host name, validity – MG 6.2.2, SF-115), a PUBLISH of exactly MaxPacketSize (SF-105); the ACL with a second, unprivileged client (Set, Del and Call to the probe AppId of D8 must not arrive, Set/Config should not be readable); the OEC Registry (two probe components announce themselves; Get MAM / Get Health with and without Source; Close message and Will). D8 publishes only on the topics of its own probe AppId `Utility/oi4.org/oec-infra/D8/<random>`.

```sh
G=../dev/generated
# dev broker, port 8883: probe credentials (privileged in the dev ACL), unprivileged second client oec-dev-app
go run ./cmd/oec-infra bus --bus-storage $G/roots/oec-dev-app --username oec-probe --password-file $G/probe/password \
  --credentials-2 $G/roots/oec-dev-app/run/secrets/oi4/credentials --registry-wait 10s
# port 8884 (client certificate required): the probe certificate, second client oec-dev-cert
go run ./cmd/oec-infra bus --broker mqtts://localhost:8884 --ca $G/broker/ca.pem --cert $G/probe/probe.pem --key $G/probe/probe.key \
  --bus-storage-2 $G/roots/oec-dev-cert --max-packet-size 262144 --tests 'T8-BUS-0[12]'
```

- Connection flags as for `oec-check monitor` (`--broker`, `--bus-storage`, `--ca`, `--cert`/`--key`/`--key-passphrase-file`, `--username`/`--password-file`, `--insecure` only explicitly, `--config`), no `--client-id` (D8 opens several connections). The second client: `--credentials-2 FILE` (base64 `user:password`, the format of `run/secrets/oi4/credentials`) or `--bus-storage-2 ROOT`; it uses the broker, CA and `--insecure` of the main connection. Without it, or with the same identity, T8-BUS-02 is inconclusive.
- `--max-packet-size` (default: `broker.json`, else 262144 with a note), `--max-probe-size` (default 64 MiB; a larger MaxPacketSize is inconclusive), `--registry IL` (default: the first Registry seen on `Oi4v2/Registry/+/Pub/#` within `--registry-wait`, default 65 s), `--timeout` (one exchange, default 5 s), `--tests`, `--allow-skip`, `--format`, `--report`. Level 3; exit codes as for `image`.
- Against `oec/dev`: T8-BUS-01 passes; T8-BUS-02 warns REQ-RES-CONFIG-026 (the dev ACL cannot bind the AppId level to a user and the applications read their own Set/Config, see [`../dev/README.md`](../dev/README.md)); T8-BUS-03 is inconclusive until the OEC Registry exists (P7).

## Layout

| Path | Content |
|---|---|
| `cmd/oec-check` | CLI (D7) |
| `cmd/oec-infra` | CLI (D8): `image`, `bus`, `report-schema`, `version` |
| `.` (`conformance`) | `report.schema.json` with `ValidateReport`, `infra-report.schema.json` with `ValidateInfraReport` and the loaders of the fixture indexes (data outside the package directories, embedded) |
| `internal/explain` | catalog view of `list`/`explain`, coverage report `catalog/COVERAGE.md` |
| `internal/topic` | staged topic diagnosis over the generated patterns and the matrix |
| `internal/ilink` | Identification Link checks, base64url elements, dnp encoding |
| `internal/schemas` | schema set (strict + tolerant), REQ mapping of schema errors, R-4 probes |
| `internal/rules` | rule layer R-2, R-3, R-6 |
| `internal/validate` | one message: topic → parse → schema → rules |
| `internal/capture` | capture format (JSONL), recorder, reader |
| `internal/mqttwire` | MQTT 3.1.1 client over TLS on `paho.mqtt.golang` v1.5.1 (EDL-1.0), PKCS#8 PBES2 keys, container storage reader |
| `internal/mqttwire/mqtttest` | test-only in-process TLS broker and throw-away PKI (gate without network); since C-071 with options for D8 (TLS versions, broker certificate variants, plain-text listener, ACL, size limits, protocol levels) |
| `internal/monitor` | stateful monitor engine: participants, rules T7-MON-xx |
| `internal/probe` | active test runner of `test`: discovery, requests with `ReplyTo` correlation, tests T7-GET-xx/ADR-xx/SET-xx/ACT-xx/CON-xx/LIFE-xx, state snapshot and restore check, restart phase, announced components (peer, `--announce`), verdict of a test run |
| `internal/proxy` | proxy attachment: TLS listener with certificates from the proxy CA, minimal MQTT 3.1.1 relay (strict CONNECT/PUBLISH decoder), connection phases |
| `internal/sim` | test-only simulated CUT with fault injection (DESIGN.md §10) |
| `internal/verdict` | verdict model: severity from REQ keyword × level, test aggregation, exit code |
| `internal/report` | text and JSON report of `validate`; text, JSON and JUnit report of `monitor` and `test` and of `oec-infra` (shared JUnit writer) |
| `internal/oci` | OCI image reader on the standard library: layout directory and oci-archive, digest verification, layer file system (D8); `ocitest`: test-only image archive builder |
| `internal/sbom` | minimal SPDX 2.x JSON, SPDX 3.x JSON-LD and CycloneDX 1.x JSON reader (D8) |
| `internal/image` | rules T8-IMG-01 … 04 of `oec-infra image` |
| `internal/bus` | tests T8-BUS-01 … 03 of `oec-infra bus`: TLS, certificate, plain-text and packet probes, ACL probe, Registry probe; `registrytest`: test-only simulated OEC Registry with faults |
| `internal/testvec` | test helper: reads the vectors of `../spec/vectors/` |
| `testdata/sim` | `index.yaml`: expected verdicts of `test` against the simulated CUT per fault (C-062 … C-064 reviewed in the C-062, C-063 and C-064 reviews, C-065 … C-067 in the C-067 review, C-068 in the C-068 review; new entries start as `draft`, G-3.9) |
| `testdata/captures` | capture fixtures of the monitor rules + `index.yaml` with expected verdicts (C-052 reviewed in the C-052 review, C-065 and C-067 in the C-067 review; new entries start as `draft`, G-3.9) |
| `testdata/images` | generated OCI archives of the image rules + `index.yaml` with expected verdicts (C-070, reviewed in the C-070 review); regenerate with `go test ./internal/image -run TestFixtureFiles -update` |
| `testdata/bus` | `index.yaml`: expected verdicts of `oec-infra bus` per broker scenario against `mqtttest` and the simulated Registry (C-071, `reviewed` since the C-071 review, `wrong-host` and `expired` `draft` again after SF-115; new or changed entries are `draft`, G-3.9) |
| `catalog` | test catalogs of `monitor`/`test` (D7, `d7.yaml`) and `oec-infra` (D8, `d8.yaml`), embedded; `COVERAGE.md` (generated); `catalog_test.go` enforces mode ↔ verification, levels and full coverage of every `d7-monitor`/`d7-test`/`d8` REQ; `independence_test.go` keeps the module free of `sdk/` (Q-009) |

## Tests

`go test ./...` checks D7 against the vectors of the baseline: every topic, IL and dnp vector; every payload vector (valid → no finding, invalid → exactly its `violates`); the split between schema layer and rule layer of [`../schema/tests/rule-layer.yaml`](../schema/tests/rule-layer.yaml); R-4 probes; CLI exit codes; the test catalogs against the requirements catalog (`catalog/`); the MQTT client against the loopback test broker (passwords, client certificates, Will, lost connection); every monitor rule with a positive and a negative capture fixture; `monitor` live against the loopback broker and from captures; every implemented `test` test with a positive and a negative run against the simulated CUT, and `oec-check test` against it; every report produced against `report.schema.json`; D8: the OCI reader and the SBOM reader, every image rule with a positive and a negative image fixture, `oec-infra image` on every fixture with each JSON report against `infra-report.schema.json`; every bus test with positive and negative broker scenarios against `mqtttest` (TLS/certificate/ACL/size faults) and the simulated Registry, `oec-infra bus` through the CLI; `list`/`explain` and the coverage report (positive and negative fixture for every implemented test). The Go toolchain is pinned in `go.mod` (`GOTOOLCHAIN=auto` with an older local Go).

The module is pure Go without cgo; `GOOS=windows|linux|darwin go build ./cmd/oec-check` (and `./cmd/oec-infra`) cross-compiles (checked in C-040 for windows/amd64, linux/amd64, linux/arm64 and darwin/arm64, in C-069 for windows/linux/darwin × amd64/arm64, `oec-infra` in C-070; tests run on macOS so far, CI on Windows/macOS/Linux comes with P9, D-1 (a)).

`./live.sh` (opt-in, needs Docker; not part of `oec/check.sh`) starts the dev broker of [`../dev/`](../dev/) and runs `TestMonitorDevBroker`: a simulated component publishes Birth, gets killed (the broker sends its Will) and reconnects, while `oec-check monitor` observes once with the client certificate of `oec-dev-cert` (port 8884) and once with the credentials of `oec-dev-app` (port 8883). Since C-071 it also runs `TestBusDevBroker`: `oec-infra bus` on port 8883 (probe credentials, second client `oec-dev-app`) and 8884 (probe certificate, second client `oec-dev-cert`). `OEC_LIVE_OUT=DIR` keeps the captures, `OEC_LIVE_KEEP=1` the broker.
