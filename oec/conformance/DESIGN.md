# D7/D8 compliance tools – architecture (P3, C-044)

Status: **design sketch**, proposed with [ADR-0025](../../project/decisions/ADR-0025-compliance-tool-architecture.md).
Built so far: `oec-check validate` (P2), the test catalog (`catalog/`, C-044) and – since **C-052** – `internal/mqttwire`, `internal/capture`, `internal/verdict`, `internal/monitor` and `oec-check monitor` (live and `--replay`, 13 of the 19 monitor tests; the "As built" notes below are amendment C-052 of ADR-0025). Everything else is the plan for P5 (D7) and P5b (D8).
Inputs: ADR-0006 (independence, modes), ADR-0013/0014 (Go, one module, two binaries), ADR-0016 (levels), ADR-0019 §5 and amendment C-040 item 7 (`validate` contract), RIG (D8), Q-009.

## 1. Scope and principles

- **Two binaries, one module** `github.com/OI4/oec/conformance`: `oec-check` (D7, application/messaging) and `oec-infra` (D8, Service Container and platform). They may share `internal/` packages (report, catalog, MQTT wire code).
- **Independent of the SDKs** (Q-009, ADR-0006): the only shared inputs are the data modules `oec/spec` (requirements, topic grammar, matrix, vectors) and `oec/schema` (JSON Schemas). `catalog/independence_test.go` fails if `go.mod` references any other `oec/` module.
- **Black box**: the tools see only what crosses the wire (MQTT, TLS) or the container/host boundary (OCI image, Engine API, file system). Requirements that cannot be observed that way are `manual` in the catalog with a reason.
- **Every verdict names REQ ids.** A test exists only as a catalog entry (§6); the catalog covers every `d7-monitor`, `d7-test` and `d8` REQ, enforced by `catalog_test.go`.
- **Pure Go, no cgo**, static binaries for Windows/macOS/Linux × amd64/arm64 (ARMv7 best effort, Q-008).

## 2. Package layout (target)

| Package | Content | Phase |
|---|---|---|
| `cmd/oec-check` | D7 CLI: `validate`, `monitor`, `test`, `list`, `explain`, `version` | `validate` P2; `monitor` **C-052**; rest P5 |
| `cmd/oec-infra` | D8 CLI: `image`, `run`, `container`, `host`, `bus`, `list`, `explain`, `version` | P5b |
| `catalog` | test catalogs `d7.yaml`/`d8.yaml` (embedded), loader, coverage gate | **C-044** |
| `internal/topic`, `ilink`, `schemas`, `rules`, `validate` | one-message layers of `validate` (unchanged) | P2 |
| `internal/mqttwire` | MQTT 3.1.1 client + packet codec + proxy endpoint (§4) on top of `paho.mqtt.golang/packets`; container storage reader | client **C-052**; proxy P5 |
| `internal/mqttwire/mqtttest` | test-only: in-process TLS broker on the paho `packets` codec and a throw-away PKI (§4) | **C-052** |
| `internal/capture` | recording format (JSONL), recorder, replayer | **C-052** |
| `internal/monitor` | stateful monitor engine: per-CUT state, rules T7-MON-xx | **C-052** (13 of 19) |
| `internal/probe` | active test runner: stimuli, correlation (CorrelationId/RequestId), timeouts, verdicts T7-xxx | P5 |
| `internal/verdict` | verdict model, severity from REQ keyword × level, applicability (§5) | **C-052** |
| `internal/report` | text, JSON (`report.schema.json`), JUnit XML | monitor report **C-052**; schema file P5 |
| `internal/sim` | simulated CUT with fault injection (§10) – test-only | P5 (for `test`; monitor fixtures are captures, §10) |
| `internal/oci`, `internal/engine`, `internal/sbom` | D8: image reader, read-only Docker/Podman Engine API client, minimal SPDX/CycloneDX reader | P5b |
| `internal/testvec` | vector helper (unchanged) | P2 |

## 3. D7 `oec-check`

### 3.1 Commands

| Command | Purpose | Network |
|---|---|---|
| `validate` | one message or a message file (exists; contract ADR-0019 amendment C-040 item 7) | none |
| `monitor` | passive observation of the bus; runs `validate` on every message plus the stateful rules (T7-MON-01 … 19) | subscriber |
| `test` | active test against one running CUT (T7-CON/LIFE/ADR/GET/SET/ACT) while the monitor rules keep running | client or proxy |
| `list` | catalog listing, filterable (`--mode`, `--level`, `--req`, `--needs`) | none |
| `explain <test or REQ id>` | test description, REQs with statements and source anchors | none |
| `version` | tool, schema release, spec baseline, catalog versions | none |

### 3.2 `monitor`

```
oec-check monitor --broker mqtts://host:8883 [--ca … --cert … --key …] \
                  [--app <AppId>] [--level 1|2|3] [--duration 10m] \
                  [--record capture.jsonl] [--report json=r.json --report junit=r.xml]
oec-check monitor --replay capture.jsonl [--app <AppId>] [--level N]
```

- Subscribes to `Oi4v2/#` with **QoS 1**: the delivered QoS is then the publisher's QoS (min of both), so REQ-MQTT-xxx about publish QoS is observable; retain flags of delivered messages show retained publications.
- Per message: the `validate` layers (topic, parse, schema, rules R-2/3/4/6). Per CUT (keyed by ServiceType + AppId): stateful rules – Birth/Health timing, uniqueness of MessageIds/RequestIds/IL, DataSetWriterId/DataSetClassId consistency, PublicationList semantics (Mode, Interval, Precisions), pagination chains (R-5 multi-message rules).
- `--app` restricts verdicts to one CUT; without it every component seen is judged (useful for a whole Service Container). Messages of other components are still used as context (e.g. requests addressed to the CUT).
- `--record` writes the capture; `--replay` runs the same engine offline and deterministically (rule regression tests, bug reports, CI). The recording is a superset of the `validate` message file: `{"topic", "payload", "received", "qos", "retain"}` per line plus a header line with tool/schema/baseline version.
- Time-based rules (e.g. Health every 60 s, PublicationList Interval) need a minimum duration; if `--duration` is too short the affected tests end as `skip` with the reason "observation too short".

**As built (C-052, ADR-0025 amendment C-052):**

- Flags: `--replay FILE` or a live connection (§8); `--record FILE`, `--duration` (default 5 min > the 2 × 60 s + 5 s the heartbeat needs), `--app`, `--level` (default 3, REQ-CMP-011), `--allow-skip`, `--heartbeat` (60 s), `--tolerance` (5 s), `--max-packet-size`, `--format text|json|junit`, `--report FORMAT=PATH`, `--config FILE.yaml`. Live-only flags are usage errors with `--replay`. A live run stops at the end of `--duration`, on SIGINT/SIGTERM or when the connection is lost (the reason is in the report and the capture trailer); the verdicts of the observed part still count.
- **Capture format v1** (`internal/capture`): line 1 is the header `{"format": "oec-capture", "version": 1, "tool", "tool_version", "schemas", "baseline", "broker", "insecure", "subscription", "subscribe_qos", "max_packet_size", "started", "note"}`; then one record per message `{"topic", "payload" | "payload_base64", "size", "received", "qos", "retain"}` (`payload` = the JSON as received, compacted; a payload that is not JSON is kept in `payload_base64`; `size` = bytes as received); an optional last line is the trailer `{"ended", "messages", "error"}`. Records are in reception order with non-decreasing `received`; the reader is strict (unknown keys fail). `validate` accepts a capture as a message file. Replay takes `subscribe_qos` and (unless `--max-packet-size` is set) `max_packet_size` from the header; the observation ends at the trailer (else at the last record).
- **Participants** are keyed by the PublisherId (`<ServiceType>/<AppId>`) and summarized in the report (messages, invalid, Births, Wills, Closes, Sources, first/last). `--app` accepts the AppId or the PublisherId; the other components are still observed as context, but only the CUT's verdicts count. A CUT that is never seen makes every implemented test `skip` (inconclusive, exit 4).
- **Implemented** (catalog status `implemented`): T7-MON-01 … 08, 10, 11, 12, 18, 19. **Deferred** (`planned`): T7-MON-09 (DataSetClassId per structure across reconnects), 13 (Data/Event publications), 14 and 15 (PublicationList Mode/Interval/Precisions and republication), 16 (RevisionCounter), 17 (Set Config objects) – they need the PublicationList/Config state model of `test`.
- Rule decisions: a Will is recognized as a Health/MAM message of the component whose MessageId timestamp is not newer than the component's last Birth (REQ-LIFE-016: the Will is registered at CONNECT, before the Birth); the heartbeat (REQ-TOP-017) is judged for the Health of the component itself (`Source` = `AppId`, SF-106) and restarted by any own Health or Birth; REQ-RES-HEALTH-004 (a change of a DetailObject) is not observable and ends as a not-applicable `skip`; IL uniqueness (T7-MON-10) is a heuristic (`info`).

### 3.3 `test`

```
oec-check test --broker … --app <AppId> [--level 3] [--attach bus|proxy] \
               [--listen :8883 --server-cert … --server-key …] \
               [--restart-cmd "docker restart cut"] [--peer] [--tests T7-GET-*] \
               [--announce] [--report …]
```

- `--level` default **3** (REQ-CMP-011: a component labeled compliant without a stated level fulfills level 3). Tests whose `level` is above the declared level are not run; each REQ of a run test is judged by its own obligation at the declared level (§5).
- Discovery: the CUT's MAM/Health/Profile (retained or by `Get`) decide applicability: which resources, subordinate assets, Config groups, PublicationList entries exist.
- Stimuli: `Get` (with paging, query, locale, unknown keys), `Set`/`Del` on every modifiable resource (with restore of the original content where possible), `Call NewDataSetWriterId`, malformed and unaddressable requests (T7-ACT-03). Responses are correlated by CorrelationId/RequestId with a timeout (`--timeout`, default 10 s).
- **Side effects**: `test` changes the CUT's configuration. The tool restores the content it changed and lists every change it could not restore in the report. It refuses to run without `--app` (never against "all").
- The monitor engine runs during `test`; its findings are part of the same report.

### 3.4 Attachment modes (`needs` in the catalog)

| Mode | How | What it adds | Catalog `needs` |
|---|---|---|---|
| bus (default) | D7 is an ordinary client of the broker (credentials with read on `Oi4v2/#`, write on the CUT's Get/Set/Del/Call topics) | everything visible to a subscriber/requester | – |
| **proxy** | D7 is the CUT's MQTT endpoint (TLS listener, `--listen`) and relays to the real broker; the CUT is configured to connect to D7 (Message Bus storage `broker.json` pointing to D7) | CONNECT details (protocol level, Will, clean session, keep alive, ClientId, credentials), TLS version/cipher offered by the CUT, certificate verification (D7 offers an untrusted CA / wrong host name / TLS 1.1 and expects the CUT to refuse), connection drops to provoke Will and recovery | `proxy` |
| restart hook | `--restart-cmd` / `--stop-cmd` + `--start-cmd`: graceful stop and start | Close message on graceful disconnect, persistence across restarts | `restart-hook` |
| peer | D7 simulates a second OEC component (own MAM/Health) that the CUT is expected to address | requests sent by the CUT | `peer` |

A test whose `needs` are not available ends as `skip` ("needs proxy"). For a level claim this makes the result **inconclusive** (§5.3). Proxy mode is the only way to check REQ-MQTT/REQ-SEC connection requirements of the CUT without broker log access ([Q-028](../../project/OPEN-QUESTIONS.md)).

### 3.5 Tester identity

D7 needs an Oi4Identifier for its own requests (Source of Get/Set). Default: a tester IL generated per run (valid against the IL grammar, configurable with `--tester-il`) **without** publishing MAM/Health, so it does not appear as a component in the Registry. `--announce` makes D7 publish MAM/Health/Birth like a component (needed in `peer` mode and when a CUT only answers known components). Open: [Q-029](../../project/OPEN-QUESTIONS.md).

## 4. MQTT layer

- Client and proxy share one codec: `github.com/eclipse/paho.mqtt.golang` v1.5.x (EPL-2.0 **or** EDL-1.0; taken under **EDL-1.0**, BSD-3-Clause, ADR-0023 rule on dual licenses) – the client for bus mode, its public `packets` package for the proxy endpoint and for raw stimuli (e.g. a PUBLISH with retain or an invalid UTF-8 topic that a normal client refuses).
- `eclipse/paho.golang` is MQTT 5 only → not usable (OEC uses MQTT 3.1.1).
- Broker for tests: the dev-env Mosquitto (P4 integration) or `mochi-mqtt/server` (MIT) embedded **in tests only**; decided in P5 by a spike, not part of the binaries. **As built (C-052):** neither spike nor mochi – `internal/mqttwire/mqtttest` is a small in-process TLS broker on the same paho `packets` codec (CONNECT with credentials or client certificate, SUBSCRIBE, PUBLISH QoS 0/1, Will on a lost connection; no retained messages, no QoS 2), so the gate needs no new dependency and no network beyond loopback. The dev-env Mosquitto is used by the opt-in `live.sh`.
- **As built (C-052), client:** `paho.mqtt.golang` **v1.5.1** (pinned in `go.mod`/`go.sum`, indirect `golang.org/x/net`, `golang.org/x/sync`, `gorilla/websocket` – BSD); `mqtts://` (also `ssl://`, `tls://`) only, TLS 1.2/1.3, CA from a PEM file (else system roots), credentials and/or a client certificate whose key may be an **encrypted PKCS#8** (PBES2: PBKDF2 with HMAC-SHA1/-256/-384/-512, AES-128/192/256-CBC – the format of the dev PKI and the RIG secret storage; other encryptions are refused with a clear error); clean session, no auto-reconnect (a lost connection is reported, not hidden), keep alive 60 s; `Kill()` drops the connection without DISCONNECT so that the broker publishes the Will (tests). `--insecure` skips only the broker certificate verification, prints a warning on stderr and is recorded in the capture header and the report.
- `--max-packet-size` (default: `MaxPacketSize` of the Message Bus storage, else the RIG minimum 262144 bytes) for pagination checks (REQ-PAG-xxx, MaxPacketSize).

## 5. Verdict model

### 5.1 Outcomes

| Outcome | Meaning |
|---|---|
| `pass` | all checked REQs of the test fulfilled |
| `fail` | a REQ whose obligation at the declared level is MUST/MUST NOT is violated |
| `warn` | a SHOULD/SHOULD NOT REQ is not followed (also: `validate` warnings) |
| `info` | a MAY observation, or a check that only gives hints (heuristics) |
| `skip` | not run: level above the declared level, not applicable (resource not supported), missing `needs`, observation too short – always with a reason |
| `manual` | listed for completeness: REQs of the catalog's `manual:` list and REQs with verification `manual` that the report should mention |

### 5.2 Severity

The severity comes from the REQ, not from the test: `levels.L<n>` of the REQ at the declared level (MUST → fail, SHOULD → warn, MAY → info, none → not judged). Ungraded REQs (no `levels`, e.g. platform REQs of D8) use their `keyword`. A test with several REQs reports one result per REQ; the test outcome is the worst of them. **As built (C-052):** a REQ that is `skip` because nothing applicable was observed (e.g. no Call sent, so no RequestId to check) does not make the test worse than `pass` ("all checked REQs"); a test is `skip` only if all its REQs are, and an *inconclusive* skip (a MUST that could not be judged: observation too short, CUT not seen) keeps the test `skip` and leads to exit 4.

### 5.3 Exit codes

| Code | Meaning |
|---|---|
| 0 | all run tests pass (warnings allowed) and no required test was skipped |
| 1 | at least one `fail` (the same as a `validate` violation) |
| 2 | usage error (flags, unreadable config) |
| 3 | internal error (tool bug, broker unreachable before any verdict) |
| **4** | **inconclusive**: no `fail`, but at least one test with a MUST REQ at the declared level was skipped (missing `needs`, CUT not found, observation too short) |

`validate` keeps 0–3 (it never skips). `--allow-skip` turns 4 into 0 for CI pipelines that run, e.g., only bus mode. Code 4 is new: [Q-030](../../project/OPEN-QUESTIONS.md).

## 6. Test catalog (`catalog/`)

- `d7.yaml` and `d8.yaml`, embedded with `go:embed`, loaded strictly (unknown keys fail). Fields: `id` (`T7-<GROUP>-NN` / `T8-<GROUP>-NN`), `title`, `mode`, `level`, `reqs`, `needs`, `status` (`planned` → `implemented` in P5/P5b), `note`; plus `manual:` (REQ + reason) for live REQs that a black-box tool cannot observe.
- Rules, enforced by `catalog_test.go` against `oec/spec`:
  1. tool name and spec baseline match;
  2. ids unique and with the tool's prefix;
  3. `mode` matches the REQ verification kind (monitor → `d7-monitor`, test → `d7-test`, D8 modes → `d8`);
  4. each REQ exists, is not deprecated and has that verification kind;
  5. `level` = the lowest level at which one of its REQs applies (absent only if all REQs are ungraded);
  6. `needs` from the tool's vocabulary;
  7. `manual:` entries have verification `manual` and a reason;
  8. **every** active `d7-monitor`, `d7-test` (D7) and `d8` (D8) REQ is covered by a test or a `manual:` entry (P3 exit criterion 4).
- Coverage at C-044:

| Catalog | Tests | By mode | `needs` | `manual:` | REQs covered |
|---|---|---|---|---|---|
| D7 | 54 | 19 monitor, 35 test | proxy 7, restart-hook 2, peer 1 | 4 (REQ-CMP-011 tool rule; REQ-LIFE-011/012/013 subscriber side) | 47 d7-monitor + 119 d7-test |
| D8 | 17 | image 4, run 3, container 3, host 4, bus 3 | docker 7, host-access 4, credentials-2 1 | 1 (REQ-CMP-012) | 50 d8 |

- The REQ ↔ test relation is many-to-many: one test can check several REQs; one REQ can be checked by a monitor rule and by an active stimulus. `explain` shows both directions; P5 adds the catalog to the coverage report of `spectool` (P5 exit).
- REQs with verification `schema`/`d7-validate` are covered by the `validate` layers and the vector gate (P2); they are not repeated in the catalog.

## 7. Reports

- `--format text|json` for stdout (as `validate`), plus any number of `--report <format>=<path>` (formats `text`, `json`, `junit`).
- **JSON**: versioned by `report.schema.json` (published with the tool; `validate`'s existing keys `version`/`schemas`/`baseline` stay). Content: tool, schema release, spec baseline, catalog version, command line (secrets removed), declared level, CUT (AppId, IL, ServiceType, MAM excerpt), attachment mode, per test: outcome, duration, per REQ: outcome, severity, evidence (message references into the capture, expected vs. actual), and a summary per outcome.
- **JUnit XML**: one `testsuite` per catalog group, one `testcase` per test, REQ ids as `properties`; `fail` → `failure`, `skip`/`manual` → `skipped`; warnings as `system-out`. Purpose: CI integration (GitHub/GitLab/Jenkins).
- **As built (C-052), `monitor`:** JSON report version 1 (`"report": 1`; keys `tool`, `version`, `schemas`, `baseline`, `catalog`, `command`, `mode` live/replay, `attachment`, `capture`, `broker`, `insecure`, `level`, `app`, `self_assessment`, `started`/`ended`/`observed`, `messages`/`namespace`, `participants`, `tests` with per-REQ obligation, outcome, `inconclusive`, reason and evidence (participant, capture line, topic, JSON pointer, detail), `summary`, `error`, `exit`). The text report lists the participants, one line per test and the evidence of every REQ that is not `pass`. `report.schema.json` follows with `test` (the format may still grow). No secret is ever part of a report or capture (passwords and passphrases are only read from files).
- A compliance claim (OI4 certification, Q-007) needs the JSON report plus the capture; the report states that it is a self-assessment.

## 8. Configuration

- `--config oec-check.yaml` (or `oec-infra.yaml`); CLI flags override the file. All flags have file keys with the same name.
- Connection: `--broker`, `--ca`, `--cert`, `--key`, `--username`, `--password-file` (no password on the command line), or `--bus-storage DIR` that reads a Message Bus storage in the RIG layout (`broker.json` + certificates) – the same files the CUT uses.
- `--insecure` only for lab setups; the report marks such runs.
- **As built (C-052):** `--config` for `monitor`: a YAML map whose keys are flag names (lists for repeatable flags); unknown keys are usage errors, flags on the command line win. Additional connection flags `--key-passphrase-file` and `--client-id`. `--bus-storage ROOT` reads `etc/oi4/mqtt/broker.json` (Address, SecurePort, MaxPacketSize), the CA `etc/oi4/mqtt/broker_ca.pem` or `etc/oi4/certs/ca.pem`, the client certificate `etc/oi4/certs/<etc/hostname>.pem` with `run/secrets/oi4/private_key.pem` and the base64 `run/secrets/oi4/passphrase`, and the base64 `run/secrets/oi4/credentials` – the layout of ADR-0026 amendment C-048 and the sample roots of `oec/dev`; explicit flags override single values.

## 9. D8 `oec-infra`

| Command | Checks | Needs | Catalog |
|---|---|---|---|
| `image <ref or OCI layout or tar>` | OCI conformance, architectures/multi-arch index, SBOM and license texts in `/opt/oi4/licenses`, signature/digest | offline (registry access optional) | T8-IMG-01 … 04 |
| `run <image>` | starts the image in a sandbox with generated storages, hostname and a D8 MQTT test endpoint; checks that the application uses Message Bus storage, certificates, secrets and its container name (REQ-CNT-004/009/021, REQ-MQTT-013) | docker | T8-RUN-01 … 03 |
| `container <name>` | running container: networks, name/hostname, mounts (licenses not overlaid) | docker | T8-CTR-01 … 03 |
| `host` | Service Container host: storage layout and permissions, isolation of application storages, platform properties | host-access (+ docker) | T8-HST-01 … 04 |
| `bus --broker …` | broker protocol, TLS, certificates; ACLs with a second credential set; OEC Registry present | credentials-2 for ACL | T8-BUS-01 … 03 |

- Libraries: `github.com/google/go-containerregistry` (Apache-2.0) for images; an own minimal read-only Engine API client over the Unix socket / named pipe (Docker and Podman share the endpoints needed; avoids the large Docker SDK); a minimal SPDX-JSON/CycloneDX-JSON reader (only presence and license fields; `spdx/tools-golang` has no stable release and an unclear license, `cyclonedx-go` is a fallback).
- `run` and `container` never modify containers other than the ones they create; `host` is read-only.
- D8 grades Service Container REQs by level like D7; platform REQs are not graded (MG 1.3) and use their keyword.

## 10. Self-test (`internal/sim`)

- A simulated CUT in Go (MQTT client of the same codec) that implements the level 1–3 behavior from the spec and can inject one fault per catalog test (`sim.Fault("T7-SET-04")`).
- Each implemented catalog test needs a positive run (sim without fault → pass) and a negative run (sim with fault → fail with the expected REQ ids). `catalog_test.go` gains this check when tests move to `implemented` (P5 exit: "every catalog test has a positive and a negative fixture").
- Captures from the demo services (D3/D4) and the SDK interop tests (P4) are replayed as additional regression data – they enter as data files, not as code dependencies (Q-009).
- **As built (C-052), monitor tests:** the positive and negative fixtures of the monitor rules are **captures** (`testdata/captures/*.jsonl`) built from reviewed payload vectors by a generator test (`go test ./internal/monitor -run TestFixtureFiles -update`; without `-update` the test fails if a file differs from its scenario), with the expected verdicts in `testdata/captures/index.yaml` (test → outcome and the exact REQ ids; status `draft` until the PO review, G-3.9). `replay_test.go` replays every fixture, checks the expected verdicts and that every implemented monitor test has a positive and a negative fixture, and that `monitor.Implemented` equals the catalog status. `internal/sim` stays the plan for `test` (active stimuli need a responding CUT). The live path is covered by a loopback test against `mqtttest` in the gate and by `oec/conformance/live.sh` (opt-in) against the dev-env Mosquitto.

## 11. Distribution

- `CGO_ENABLED=0` cross builds: windows/darwin/linux × amd64/arm64; linux/arm/v7 best effort (Q-008).
- Multi-arch container image `FROM scratch` (+ CA certificates) for CI use; `oec-infra container/host` need the Docker socket mounted and are documented as host-binary first.
- Release tooling (GoReleaser, MIT, or plain `go build` + scripts), signing and SBOM of the tools themselves decided in P9 together with Q-011 (CRA).
- Versioning: the tools carry their own SemVer; each report and `version` names the schema release and the spec baseline they embed.

## 12. Out of scope for P3

No MQTT, monitor, test or D8 code. P5 starts with `internal/mqttwire` + `internal/capture` + `monitor --replay`, P5b with `image`. (C-052 delivered the P5 start including the live `monitor`; see the "As built" notes.)
