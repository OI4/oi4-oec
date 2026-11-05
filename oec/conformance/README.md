# `oec/conformance/` – D7/D8 compliance CLIs

> **State: D7 `validate` (C-040); test catalog and architecture design (C-044, [`DESIGN.md`](DESIGN.md), ADR-0025); `monitor` live and `--replay` with 13 of 19 monitor tests (C-052, ADR-0025 amendment C-052); `test` with the bus attachment and T7-GET-01/02/05/09 (C-062, ADR-0025 amendment C-062), every `Get` test and T7-ADR-02/03 (C-063, amendment C-063), the write tests T7-SET-01 … 09, 11, 12 with restore (C-064, amendment C-064), the Action tests T7-ACT-01 … 04 and the monitor rules T7-MON-09, 13, 15, 16, 17 (18 of 19, C-065, amendment C-065), the proxy attachment with T7-CON-01 … 04 and T7-LIFE-01 … 03 (C-066, amendment C-066), the restart hook with T7-LIFE-04 and T7-SET-10 and the monitor rule T7-MON-14 (19 of 19, C-067, amendment C-067), the peer simulation, `--announce` and T7-ADR-01 (C-068, amendment C-068) – all 55 D7 tests implemented; `list`, `explain`, `report-schema`, the frozen report schema version 1 and the catalog coverage report (C-069, amendment C-069). D8: `oec-infra image` with T8-IMG-01 … 04 and its report schema (C-070, amendment C-070; P5b plan in the ROADMAP), `oec-infra bus` with T8-BUS-01 … 03 (C-071, amendment C-071), `oec-infra host` with T8-HST-01, 02, 04 (C-072, amendment C-072), the read-only Engine API client, `oec-infra container` with T8-CTR-01 … 03 and T8-HST-03 (C-073, amendment C-073), `oec-infra run` with T8-RUN-01 … 03 (C-074, amendment C-074) – all 17 D8 tests implemented; `oec-infra list`, `explain`, the D8 part of the coverage report and the frozen D8 report schema version 1 (C-075, amendment C-075; P5b exit run in the C-075 log).** One Go module (ADR-0013/0014) for the application and messaging compliance CLI **`oec-check`** (D7) and the infrastructure compliance CLI **`oec-infra`** (D8, P5b). It depends only on [`../spec/`](../spec/) (data files via `spec.FS`) and [`../schema/`](../schema/) (entry schemas via `schema.FS`), never on `sdk/` or on the internals of `spec/tools` (ADR-0006/0018/0019 §5).

User guides (documentation D6): [Compliance tools](https://oi4.github.io/oi4-oec-documentation/docs/community/compliance), [`oec-check`](https://oi4.github.io/oi4-oec-documentation/docs/community/compliance/oec-check), [`oec-infra`](https://oi4.github.io/oi4-oec-documentation/docs/community/compliance/oec-infra), [Validate a message](https://oi4.github.io/oi4-oec-documentation/docs/community/compliance/validate).

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
- Run: `--duration` (default 5m; the heartbeat needs at least 2 × `--heartbeat` + `--tolerance` = 125 s), `--app` (AppId or `ServiceType/AppId` of the component under test; without it every participant is judged), `--level` (1–3, default 3), `--allow-skip`, `--max-packet-size`, `--config FILE.yaml` (keys = flag names except `config` and `replay`, command line wins).
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

- **Restart hook** (DESIGN.md §3.4): `--restart-cmd CMD`, or `--stop-cmd CMD --start-cmd CMD`, and `[--restart-wait 2m]`. Shell commands (`sh -c`, Windows `cmd /S /C` with the command line as written, so a quoted path works) that stop the CUT gracefully (SIGTERM, `docker stop`, …) and start it again; a command that starts the CUT in the background must redirect its output, else D7 waits up to 10 s for it. In the write window, after the other write tests, T7-SET-10 sets a changed ReferenceDesignation, D7 runs the hook and waits up to `--restart-wait` for the new Birth (through the proxy also the first SUBACK) and an answer to Get Health; then T7-LIFE-04 judges the last Pub Health before the DISCONNECT (proxy) or before the new Birth (bus) as the Close message, and T7-SET-10 reads the ReferenceDesignation again and restores it. Without the hook both tests are skipped and the run is inconclusive (exit 4); a stop without DISCONNECT (Will) is inconclusive as well.

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
- [`catalog/COVERAGE.md`](catalog/COVERAGE.md) is the generated coverage report, a D7 section and (since C-075) a D8 section (tests by status and need, REQs per verification kind and how they are checked, fixtures per test); regenerate with `go test ./internal/explain -run TestCoverageReport -update`.

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
- The JSON report has its own schema [`infra-report.schema.json`](infra-report.schema.json) (report version 1, frozen since the P5b exit, C-075: `freeze_test.go` pins its hash like that of `report.schema.json`; a change of the format is a new report version and a new schema).

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
- Against `oec/dev`: T8-BUS-01 passes; T8-BUS-02 warns REQ-RES-CONFIG-026 (the dev ACL cannot bind the AppId level to a user and the applications read their own Set/Config, see [`../dev/README.md`](../dev/README.md)); T8-BUS-03 is inconclusive there without an OEC Registry and passes with the registry of the compose profile `registry` (C-090, `OEC_LIVE_REGISTRY=1 live.sh`; again in the P7 exit run C-091); against the registry of `oec/registry` on the test broker it passes (C-086, its gate step). `oec-infra bus` sends deliberately invalid messages on its probe AppIds (`oi4.org/oec-infra/D8/…`): an `oec-check monitor` on the same bus at the same time fails T7-MON-01/05 for them – do not overlap the two, or replay the record without the probe topics.

## `oec-infra host`

Reads the storages of RIG 4.1 below a root directory, measures the platform host it runs on and compares the mounts of the containers of the engine (D8, T8-HST-01 … 04; read-only, never prints a secret):
- **T8-HST-01** Message Bus storage `etc/oi4/mqtt`: `broker.pem` verifies against `broker_ca.pem` or `ca.pem`, `broker.json` against `platform/broker.schema.json` (REQ-CNT-001/003, REQ-MQTT-003).
- **T8-HST-02** OI4 certificate storage `etc/oi4/certs` (`ca.pem` a CA, `<containername>.pem` the certificate of `private_key.pem`) and secret storage `run/secrets/oi4` (only `credentials`, `private_key.pem`, `passphrase`; the passphrase opens the key; at least one login), no private key elsewhere (REQ-CNT-002/005 … 008, REQ-MQTT-011).
- Every storage mounted read-only – judged on Linux from `/proc/self/mountinfo`, so run it inside the application container (or a container with the same mounts); with `--root` on a copy the mounts are `unknown`.
- **T8-HST-04** platform host (RIG 5.1): architecture, RAM, non-volatile memory, a network interface (REQ-PLT-001; RIG 5.1 counts the nominal size of the installed memory with 1 GB = 10^9 octets, so values just below 1 GB / 4 GB are `manual`, as are virtual-only interfaces), Linux (REQ-PLT-003; the patch level is reported, the verdict is `manual`), a container daemon (REQ-PLT-006), the broker of `broker.json` with the login of the storages and the OEC Registry (REQ-PLT-007/008).
- **T8-HST-03** (C-073) application storages not shared, through the Engine API: every OI4 container of the engine (a mount on or above `/etc/oi4`, `/opt/oi4` or `/run/secrets/oi4`; running or stopped) – no other container mounts the same named volume or a nesting host path of its secret storage (REQ-CNT-024) or, if any OI4 container persists `/etc/oi4/app` or `/opt/oi4/app` with a bind or volume, of its application storages (REQ-CNT-011; a missing application mount is `manual`, a tmpfs or image mount fails). Sources are compared as the engine reports them; the evidence names the other container and its destination, never a source. `--self CONTAINER` excludes the container D8 runs in; no engine: inconclusive.

```sh
G=../dev/generated
# the sample roots of oec/dev on this computer (macOS fails REQ-PLT-003; without the dev broker use --offline)
go run ./cmd/oec-infra host --root $G/roots/oec-dev-app --offline
# inside a Linux container with the storages mounted read-only, in the network of the dev broker
GOOS=linux CGO_ENABLED=0 go build -o /tmp/oec-infra ./cmd/oec-infra
docker run --rm --network container:oec-dev-mosquitto-1 --entrypoint /oec-infra -v /tmp/oec-infra:/oec-infra:ro \
  -v $PWD/$G/roots/oec-dev-app/etc/oi4/mqtt:/etc/oi4/mqtt:ro -v $PWD/$G/roots/oec-dev-app/etc/oi4/certs:/etc/oi4/certs:ro \
  -v $PWD/$G/roots/oec-dev-app/run/secrets/oi4:/run/secrets/oi4:ro -v /var/run/docker.sock:/var/run/docker.sock \
  eclipse-mosquitto:2.1.2-alpine host --container oec-dev-app --registry-wait 5s
```

- `--root DIR` (default `/`), `--mqtt-storage`, `--certs-storage`, `--secrets-storage` (one storage elsewhere), `--container NAME` (default `ROOT/etc/hostname`), `--daemon ENDPOINT` (`unix://`, `npipe://`, `tcp://` or a socket path; default `DOCKER_HOST`, `CONTAINER_HOST`, the default sockets of the OS), `--self CONTAINER`, `--offline` (no Message Bus: REQ-PLT-007/008 inconclusive), `--registry-wait` (65 s), `--timeout` (5 s), `--tests`, `--allow-skip`, `--format`, `--report`, `--config`. Level 3; exit codes as for `image`. A `manual` requirement does not change the outcome of its test; the report lists the measured value for a human verdict.
- Against `oec/dev` (C-072): T8-HST-01/02 pass on both sample roots; in a Linux container the mounts are `ro`, a read-write secret storage fails REQ-CNT-008; T8-HST-04 fails REQ-PLT-008 until the OEC Registry exists (P7). C-073: T8-HST-03 fails when a second container mounts the application volume of another (live test), passes without it.

## `oec-infra container`

Reads one or more containers (name, id or unique id prefix, running or created) through the Docker/Podman Engine API (D8, T8-CTR-01 … 03; only `GET`, never the environment, labels or mount sources in a report):
- **T8-CTR-01** networks (RIG 4.4): a bridge network passes; the host network fails REQ-CNT-023 (SHOULD → warn) and warns REQ-CMP-031 (the exception for low-level Ethernet communication is judged manually); `none`, user-mode networks (`pasta`, `slirp4netns`) and other drivers (`macvlan`, `ipvlan`, overlay) are notes; `container:<id>` is followed one step.
- **T8-CTR-02** name and hostname (RIG 4.2): the container name is a host name label (RFC 1123, at most 63 characters), unique within the engine (case-insensitive) and the hostname (REQ-CNT-019); it ends in `-<uuid>` with a default name of at most 26 characters (REQ-CNT-020, SHOULD).
- **T8-CTR-03** `/opt/oi4/licenses` not overlaid by the host (REQ-CNT-017): a bind mount or tmpfs on it, covering it or below it fails; a named volume is `manual` (allowed if populated from the image, REQ-CNT-018); an image mount is a note.

```sh
# a container of the local engine (DOCKER_HOST, CONTAINER_HOST or the default sockets)
go run ./cmd/oec-infra container oec-dev-app-0b7c…
# Podman, remote engine over TCP (plain only)
go run ./cmd/oec-infra container --daemon unix:///run/user/1000/podman/podman.sock my-app-…
go run ./cmd/oec-infra container --daemon tcp://edge-host:2375 my-app-…
```

- `--daemon ENDPOINT` (as for `host`), `--timeout` (10 s), `--level` (default 3), `--tests`, `--allow-skip`, `--format`, `--report`, `--config`. An unknown container is a usage error (exit 2); no engine makes every test inconclusive (exit 4).
- Against `oec/dev` (C-073, live test): a container named `<name>-<uuid>` with that hostname, on the bridge network of the broker and with own volumes passes; host network, a name with `_` and a bind on `/opt/oi4/licenses` fail. The dev broker itself fails REQ-CNT-019 (hostname `mosquitto`, compose name `oec-dev-mosquitto-1`) – Mosquitto is no OI4 Alliance-compliant container, so this is not a deviation of `oec/dev`.

## `oec-infra run`

Starts an application image in a sandbox of its own through the local Docker/Podman Engine API and checks that the application uses what the platform gives it (D8, T8-RUN-01 … 03, level 3 by default):
- D8 generates the three storages of RIG 4.1 per container: `broker.json` (a non-default `SecurePort`, `MaxPacketSize` and an element the application does not know), `broker.pem`, `ca.pem`, `broker_ca.pem`, the client certificate `<containername>.pem`, the secret storage (`credentials` or `private_key.pem` encrypted + `passphrase`), and an empty application storage.
- Each container is named `oec-d8-run-<uuid>` with that name as hostname (RIG 4.2). An MQTT test endpoint of D8 (TLS, credentials or client certificate, MaxPacketSize) answers like a broker without forwarding.
- Three phases, one container each: **credentials** (only the credentials, broker certificate signed by the CA of `ca.pem`), **certificate** (only the client certificate, broker certificate signed by the CA of `broker_ca.pem`) and **untrusted** (the endpoint shows a certificate of an unknown CA; the application must not connect – only with T8-RUN-01).
- **T8-RUN-01** (REQ-CMP-029, REQ-CNT-004): connection to `Address`/`SecurePort` of `broker.json`, broker verification, no payload above `MaxPacketSize`; REQ-CMP-029 repeats the observations of REQ-CNT-004/009 and REQ-CNT-021. **T8-RUN-02** (REQ-CNT-009): login with the generated credentials and with the client certificate; only one of the two is a violation (RIG 4.1.3, SF-117). **T8-RUN-03** (REQ-CNT-021, REQ-MQTT-013): ClientId and the SerialNumber of the Birth message (MAM of the application) are the container name; a URI as Identification Link is a note.

```sh
# a local image (never pulled); Docker Desktop on macOS: the endpoint listens on 127.0.0.1
go run ./cmd/oec-infra run my-app:1.0
# Linux: the endpoint listens on the bridge gateway; an IP address as broker host needs no host-gateway
go run ./cmd/oec-infra run --broker-host 172.17.0.1 --wait 60s --report json=run.json my-app:1.0
# rootless Podman (C-107): the gateway of its bridge is no address of the host – name one
go run ./cmd/oec-infra run --daemon unix:///run/user/1000/podman/podman.sock --listen 192.0.2.10 --broker-host 192.0.2.10 my-app:1.0
```

- `--daemon ENDPOINT` (local only: `unix://`, `npipe://`, loopback `tcp://`; a remote engine is a usage error, exit 2), `--timeout` (10 s), `--wait` (30 s, until the Birth message), `--settle` (2 s), `--listen ADDR`, `--broker-host NAME|IP` (`oec-d8-broker` via `host-gateway`), `--max-packet-size` (262144), `--level`, `--tests`, `--allow-skip`, `--format`, `--report`, `--config`. An image the engine does not have is a usage error – D8 never pulls.
- D8 creates, starts, stops and removes only containers named `oec-d8-run-…` with its labels `org.oi4.oec-infra=run` and `org.oi4.oec-infra.run=<run id>` (anonymous volumes of those containers go with them); it never creates or removes volumes, networks or images. The report never contains a secret, a username, a log, the environment or a host path.
- Live (C-074, `live.sh`, Docker Desktop 4.47 / Engine 28.4): the 15 faults of the test image `oec-d8-sim` give the same verdicts as against the fake Engine API. C-107 (Ubuntu 24.04): the same with Docker Engine 29.1 and with rootless Podman 4.9.3 (`oec/ci/podman.sh`; `OEC_LIVE_RUN_LISTEN` passes `--listen`/`--broker-host` to `TestRunDev`).

## `oec-infra list`, `explain`, `report-schema`

Views of the embedded D8 catalog ([`catalog/d8.yaml`](catalog/d8.yaml)), the requirements of `../spec/` and the D8 fixture indexes (image, bus, host, container, run), like those of `oec-check`; no network, no Docker.

```sh
go run ./cmd/oec-infra list                              # 17 tests: id, mode, level, status, needs, title, REQs
go run ./cmd/oec-infra list --mode host --needs host-access
go run ./cmd/oec-infra list --req REQ-CNT-009 'T8-RUN-*' --format json
go run ./cmd/oec-infra explain T8-RUN-02                 # notes, REQs with keyword, levels and RIG/MG sources, fixtures
go run ./cmd/oec-infra explain REQ-CMP-012               # statement, sources, verification, tests (here: manual)
go run ./cmd/oec-infra report-schema > infra-report.schema.json
```

- `list` filters: `--mode image|run|container|host|bus`, `--level N`, `--req`, `--needs docker|host-access|credentials-2|none`, `--status`, id patterns (`T8-HST-*`); flags may follow the patterns. A test that checks only platform requirements is not graded (level `-`, MG 1.3) and is listed at every `--level`. Unfiltered it ends with the totals and the `manual` requirement.
- `explain` takes one test or REQ id (case-insensitive); an unknown id is a usage error (exit 2). The fixtures are those of the five D8 indexes with their stated outcome, and the requirements a fixture expects as manual. A requirement checked only by D7 points to `oec-check explain` (JSON `oec_check`); a `T7-…` id is a usage error with the same pointer.

## Image `oi4/oec-conformance` (C-107)

Both tools in one multi-arch image (linux/amd64, linux/arm64; ADR-0025 §10, amendment C-107): `oec-check` and `oec-infra` in `/usr/local/bin`, static (`CGO_ENABLED=0`, `-trimpath`, no build id), on `gcr.io/distroless/static-debian13:nonroot` (CA certificates, `/tmp`, user 65532; no shell), the SBOM and license texts in `/opt/oi4/licenses` by `oec-sbom` with the Go license map [`images/go-licenses.json`](images/go-licenses.json) (one entry `module@version` → SPDX expression per required module of `go.mod`; a test keeps it complete). Built locally only (opt-in, Docker with buildx; never pushed – publication is the PO's, Q-011):

```sh
conformance/images/build.sh /tmp/oec-images     # from oec/: build both platforms, D8 image, load the native one
docker run --rm oi4/oec-conformance:0.1.0 oec-check list
docker run --rm -v "$PWD/app.oci.tar:/work/app.oci.tar:ro" oi4/oec-conformance:0.1.0 \
  oec-infra image --level 3 /work/app.oci.tar
conformance/images/build.sh --remove            # remove the loaded image
```

`build.sh` judges the OCI archive with D8 `image --level 3` (`T8-IMG-01 … 04`) and runs both tools in the loaded image (`oec-check list`; `oec-infra image` on its own archive, without network). D8 `run` does not apply: the tools are no OEC application. The command is `oec-check` by default (no entrypoint: name the tool). `bus`, `monitor` and `test` need the network of the broker (`--network`) and the storages mounted read-only; `run`, `container` and `host` need the engine socket mounted – use the binaries on the host for those (`conformance.yml` artifacts or `go run`).

## Layout

| Path | Content |
|---|---|
| `cmd/oec-check` | CLI (D7) |
| `cmd/oec-infra` | CLI (D8): `image`, `bus`, `host`, `container`, `run`, `list`, `explain`, `report-schema`, `version` |
| `.` (`conformance`) | `report.schema.json` with `ValidateReport`, `infra-report.schema.json` with `ValidateInfraReport` and the loaders of the fixture indexes (data outside the package directories, embedded) |
| `internal/explain` | catalog view of `list`/`explain` of both CLIs (`explain.D7`, `explain.D8`), coverage report `catalog/COVERAGE.md` |
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
| `internal/host` | tests T8-HST-01 … 04 of `oec-infra host`: storages below a root, mount table (Linux), platform facts per OS (`facts_<os>.go`), container daemon, Message Bus and Registry, application storages of the containers of the engine (`storages.go`); `hosttest`: test-only builder of host roots |
| `internal/engine` | Docker/Podman Engine API client on the standard library (Unix socket, Windows named pipe, TCP; `_ping`, `version`, container list and inspect, network inspect, image inspect), default endpoints per OS; writes (create, start, stop, remove) only for its own containers (`write.go`, name prefix `oec-d8-` and owner label); `enginetest`: test-only fake Engine API serving recorded inspect documents, since C-074 with a run mode that starts a simulated application in process |
| `internal/container` | tests T8-CTR-01 … 03 of `oec-infra container` |
| `internal/run` | tests T8-RUN-01 … 03 of `oec-infra run`: phases, generated storages, run PKI, MQTT test endpoint, judging; `simapp`: simulated OEC application with 15 faults (and the binary `oec-run-sim` for the live run); `runtest`: test images `oec-d8-sim:<fault>` for the fake engine and as OCI/Docker archive for `docker load` |
| `internal/testvec` | test helper: reads the vectors of `../spec/vectors/` |
| `testdata/sim` | `index.yaml`: expected verdicts of `test` against the simulated CUT per fault (C-062 … C-064 reviewed in the C-062, C-063 and C-064 reviews, C-065 … C-067 in the C-067 review, C-068 in the C-068 review; `T7-GET-04/registry` of C-087 `reviewed` since the C-087 addendum 1; new entries start as `draft`, G-3.9) |
| `testdata/captures` | capture fixtures of the monitor rules + `index.yaml` with expected verdicts (C-052 reviewed in the C-052 review, C-065 and C-067 in the C-067 review; new entries start as `draft`, G-3.9) |
| `testdata/images` | generated OCI archives of the image rules + `index.yaml` with expected verdicts (C-070, reviewed in the C-070 review); regenerate with `go test ./internal/image -run TestFixtureFiles -update` |
| `testdata/bus` | `index.yaml`: expected verdicts of `oec-infra bus` per broker scenario against `mqtttest` and the simulated Registry (C-071, `reviewed` since the C-071 review, `wrong-host` and `expired` re-reviewed after SF-115; `T8-BUS-03/status-event` added in C-086, `reviewed` since the C-086 addendum 1; new or changed entries are `draft`, G-3.9) |
| `testdata/hosts` | `index.yaml`: expected verdicts of `oec-infra host` per host scenario (generated root, fake facts, mount table and daemon, `mqtttest` broker with the simulated Registry; C-072, `reviewed` since the C-072 review, `ram-reserved` and `storage-nominal` re-reviewed after SF-116; the T8-HST-03 scenarios of C-073 and the changed `none` and `T8-HST-02/certificate` `reviewed` since the C-073 review) |
| `testdata/containers` | `engine/`: inspect documents recorded from Docker Engine 28.4 (API 1.51), sanitized; `index.yaml`: expected verdicts of `oec-infra container` per scenario against the fake Engine API (C-073, `reviewed` since the C-073 review) |
| `testdata/run` | `index.yaml`: expected verdicts of `oec-infra run` per fault of the simulated application and per environment case against the fake Engine API (C-074, `reviewed` since the C-074 review; `passphrase-raw`, `no-cert` and `no-credentials` re-reviewed after SF-117) |
| `Dockerfile`, `images/` | the image `oi4/oec-conformance` (C-107): `images/build.sh` (opt-in), `images/go-licenses.json` (the Go license map of its SBOM); `image_test.go` checks them offline |
| `catalog` | test catalogs of `monitor`/`test` (D7, `d7.yaml`) and `oec-infra` (D8, `d8.yaml`), embedded; `COVERAGE.md` (generated); `catalog_test.go` enforces mode ↔ verification, levels and full coverage of every `d7-monitor`/`d7-test`/`d8` REQ; `independence_test.go` keeps the module free of `sdk/` (Q-009) |

## Tests

`go test ./...` checks D7 against the vectors of the baseline: every topic, IL and dnp vector; every payload vector (valid → no finding, invalid → exactly its `violates`); the split between schema layer and rule layer of [`../schema/tests/rule-layer.yaml`](../schema/tests/rule-layer.yaml); R-4 probes; CLI exit codes; the test catalogs against the requirements catalog (`catalog/`); the MQTT client against the loopback test broker (passwords, client certificates, Will, lost connection); every monitor rule with a positive and a negative capture fixture; `monitor` live against the loopback broker and from captures; every implemented `test` test with a positive and a negative run against the simulated CUT, and `oec-check test` against it; every report produced against `report.schema.json`; D8: the OCI reader and the SBOM reader, every image rule with a positive and a negative image fixture, `oec-infra image` on every fixture with each JSON report against `infra-report.schema.json`; every bus test with positive and negative broker scenarios against `mqtttest` (TLS/certificate/ACL/size faults) and the simulated Registry, `oec-infra bus` through the CLI; every host test with positive and negative host scenarios on generated roots (T8-HST-03 against the fake Engine API), `oec-infra host` through the CLI; the Engine API client and every container test with positive and negative scenarios against the fake Engine API, `oec-infra container` through the CLI; every run test with positive and negative fixtures against the fake Engine API and the simulated application (no Docker, loopback only), the refusal of foreign containers, no secret in a report, `oec-infra run` through the CLI; `list`/`explain` of both CLIs and the coverage report (positive and negative fixture for every implemented D7 and D8 test); the hashes of both frozen report schemas (`freeze_test.go`). The Go toolchain is pinned in `go.mod` (`GOTOOLCHAIN=auto` with an older local Go).

The module is pure Go without cgo; `GOOS=windows|linux|darwin go build ./cmd/oec-check` (and `./cmd/oec-infra`) cross-compiles (checked in C-040 for windows/amd64, linux/amd64, linux/arm64 and darwin/arm64, in C-069 for windows/linux/darwin × amd64/arm64, `oec-infra` in C-070, both in C-075 for windows/linux/darwin × amd64/arm64 and linux arm/386 with `go vet` for windows amd64/arm64 and linux arm/386; tests run on macOS so far, CI on Windows/macOS/Linux comes with P9, D-1 (a)).

`./live.sh` (opt-in, needs Docker; not part of `oec/check.sh`) starts the dev broker of [`../dev/`](../dev/) and runs `TestMonitorDevBroker`: a simulated component publishes Birth, gets killed (the broker sends its Will) and reconnects, while `oec-check monitor` observes once with the client certificate of `oec-dev-cert` (port 8884) and once with the credentials of `oec-dev-app` (port 8883). Since C-071 it also runs `TestBusDevBroker`: `oec-infra bus` on port 8883 (probe credentials, second client `oec-dev-app`) and 8884 (probe certificate, second client `oec-dev-cert`). Since C-072 also `TestHostDevRoots` (`oec-infra host` on both sample roots on this computer) and `TestHostDevContainer` (the Linux binary in a container in the network of the broker with the storages mounted read-only, once with a read-write secret storage). Since C-073 also `TestContainerDev`: two throw-away containers next to the broker (one conforming, one with host network, an invalid name, a bind on the license folder and the application volume of the first), `oec-infra container` on both and on the broker, `oec-infra host --tests T8-HST-03` with and without the second container; both are removed afterwards with their anonymous volumes (`rm -f -v`; since C-075 the test fails if a dangling volume is left). Since C-074 also `TestRunDev`: it builds the simulated application for Linux, loads the test images `oec-d8-sim:<fault>` with `docker load`, runs `oec-infra run` for each of the 15 faults, compares with the expectations of `testdata/run/index.yaml`, removes the images by tag and checks that no `oec-d8-*` container is left. `OEC_LIVE_OUT=DIR` keeps the captures, `OEC_LIVE_KEEP=1` the broker.
