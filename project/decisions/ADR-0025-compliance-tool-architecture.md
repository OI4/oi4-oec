# ADR-0025 – Architecture of the compliance tools D7 `oec-check` and D8 `oec-infra`

- Status: **accepted** (PO, C-045 review 2026-10-26; proposed in C-044)
- Date: 2026-10-26
- Deciders: product owner (Thomas) – proposed by the agent in C-044
- Related: ADR-0006 (independence of D7 from the SDKs, modes), ADR-0013/0014 (Go, one module, two binaries), ADR-0016 (levels), ADR-0018 (spec baseline), ADR-0019 §5 and amendment C-040 item 7 (`validate` contract), ADR-0023 (license rules, dual licenses), Q-008, Q-009, Q-011, Q-028, Q-029, Q-030; design: [`oec/conformance/DESIGN.md`](../../oec/conformance/DESIGN.md)

## PO decision (C-045 review, 2026-10-26)

Accepted as proposed. Q-028 (a): proxy mode is required for a full level claim of `oec-check test`; a bus-only run ends as inconclusive. Q-029 (a): the tester is not announced on the bus by default (`--announce` optional). Q-030 (a): exit code 4 = inconclusive, `--allow-skip` maps 4 → 0 for partial CI runs; `validate` keeps 0–3.

## Context

P2 delivered `oec-check validate`: a static checker of single messages, with exit codes 0–3 and a JSON report. P3 must decide how D7 grows into a live tester and how D8 is built. P5/P5b then implement the result.

The inputs:
- 434 active REQs. Their verification kinds:
  - 119 `d7-test`: needs a stimulus;
  - 47 `d7-monitor`: observable passively, sometimes over many messages;
  - 50 `d8`: container, image, host or broker;
  - 117 `manual`.
- ROADMAP P3 exit criterion 4: every `d7-test`/`d8-*` REQ maps to a planned test case.
- Q-009: the tools share only the schemas and the spec baseline with the SDKs.

## Decision

1. **Commands.**
   - `oec-check` has `validate` (unchanged), `monitor` (passive; `--record`/`--replay`), `test` (active against one CUT; default `--level 3` per REQ-CMP-011), `list`, `explain` and `version`.
   - `oec-infra` has `image`, `run`, `container`, `host`, `bus`, `list`, `explain` and `version`.
   - Both binaries live in the one module `oec/conformance` and may share `internal/` packages.
2. **Attachment modes for D7.**
   - Bus client is the default.
   - **Proxy:** D7 is the CUT's TLS MQTT endpoint and relays to the broker. This is the only black-box way to observe CONNECT, Will, TLS version, certificate verification and recovery.
   - Restart hook: a user-supplied command.
   - Peer: D7 plays a second component.
   - A catalog test declares what it `needs`. A missing need gives `skip` with a reason.
3. **Test catalog as data** (`oec/conformance/catalog/d7.yaml`, `d8.yaml`, embedded in the binaries).
   - Each test has an id, mode, level, REQs, needs and status.
   - `manual:` entries list live REQs that a black-box tool cannot observe, each with a reason.
   - `catalog_test.go` enforces, against `oec/spec`:
     - id rules;
     - mode ↔ verification kind;
     - REQ existence;
     - level = the lowest level at which one of the test's REQs applies;
     - **full coverage of every `d7-monitor`, `d7-test` and `d8` REQ**.
   - At C-044:
     - D7: 54 tests (19 monitor, 35 test) and 4 manual entries.
     - D8: 17 tests and 1 manual entry.
4. **Verdict model.**
   - Outcomes: pass/fail/warn/info/skip/manual.
   - The severity of each REQ comes from its obligation at the declared level (MUST → fail, SHOULD → warn, MAY → info). Ungraded REQs use their keyword.
   - Applicability is taken at run time from Profile, Config and the PublicationList.
5. **Exit codes.**
   - Codes 0–3 as in `validate`.
   - New: **4 = inconclusive.** No failure occurred, but a test with a MUST REQ at the declared level was skipped. `--allow-skip` maps 4 to 0.
6. **Reports.**
   - Formats: text; JSON (versioned by `report.schema.json`); JUnit XML. Several at once via `--report fmt=path`.
   - Each report names the tool, schema release, spec baseline, catalog version, declared level, CUT and attachment mode, and gives evidence per REQ.
7. **Libraries.**
   - `eclipse/paho.mqtt.golang` (client plus `packets` codec for the proxy). It is dual EPL-2.0/EDL-1.0; taken under **EDL-1.0** (BSD-3-Clause).
   - `google/go-containerregistry` (Apache-2.0).
   - Own minimal read-only Engine API client and a minimal SBOM reader.
   - An embedded test broker (`mochi-mqtt/server`, MIT) is allowed in tests only.
   - No cgo.
8. **Independence.**
   - `catalog/independence_test.go` fails if `go.mod` references an `oec/` module other than `spec` or `schema`.
   - Captures from SDK-based apps can serve as regression data, never as code.
9. **Self-test.**
   - A simulated CUT (`internal/sim`) with one injectable fault per catalog test.
   - An `implemented` test needs a positive and a negative fixture.
10. **Distribution.**
    - Static cross builds for windows/darwin/linux × amd64/arm64; linux/arm/v7 best effort.
    - A multi-arch `FROM scratch` image.
    - Release tooling and signing are decided in P9 together with Q-011.

## Consequences

- Without proxy mode, D7 cannot judge connection-level REQs (T7-CON-01…04, T7-LIFE-01…03). A bus-only run of a full level claim therefore ends as inconclusive (exit code 4), not as pass (Q-028, Q-030).
- `test` changes the CUT's state. D7 restores what it changed, reports what it could not restore, and refuses to run without `--app`.
- The catalog is the single list of D7/D8 tests. New REQs, or REQs whose verification changes, fail `oec/check.sh` until the catalog is updated. This keeps the spec and the tools in step at every re-baseline.
- The catalog levels are derived from the REQs. A test runs from the first level at which any of its REQs applies, and each REQ is judged at its own obligation. One test can therefore pass at L1 and fail at L2 for the same CUT behavior; this is intended.
- P5 starts with `internal/mqttwire` + `internal/capture` + `monitor --replay`. P5b starts with `image`.

## Alternatives considered

- **One binary with subcommands for D7 and D8.** Rejected: ADR-0014 decided two tools for two audiences (app vendors vs. platform/operations). The shared module already gives code reuse.
- **Tests in Go code only, no data catalog.** Rejected: coverage would not be checkable against the spec without parsing code, and `list`/`explain`/the reports would need a second registry.
- **Broker-side observation** (a plugin or log access for Mosquitto) instead of proxy mode. Rejected as the default: it ties D7 to one broker product and needs operator access. It remains possible later as an additional mode.
- **Exit code 1 for skipped MUST tests.** Rejected: it mixes "violates the spec" with "could not be checked". **Exit code 0 with warnings** was also rejected: a CI run would claim compliance it did not check.
- **`eclipse/paho.golang`.** Rejected: MQTT 5 only. **An own Go MQTT codec**: not needed, because the `packets` package of paho.mqtt.golang already exposes the codec.
- **The Docker SDK (`github.com/docker/docker/client`).** Rejected: a large dependency tree for a few read-only endpoints, and Podman compatibility is no better.

## Amendment C-052 (2026-10-26) – P5 interleave: mqttwire, capture, monitor (agent, G-3.4 b)

Decided by the agent within this ADR; the PO can overrule. Details: `log/2026-10-26_C-052_d7-monitor.md`, REVIEW A-51 … A-62; the "As built (C-052)" notes in `oec/conformance/DESIGN.md`.

1. **Delivered.** `internal/mqttwire` (client), `internal/capture`, `internal/verdict`, `internal/monitor`, the monitor report and `oec-check monitor`, live and `--replay`. 13 of the 19 monitor tests are `implemented` (T7-MON-01 … 08, 10, 11, 12, 18, 19). T7-MON-09 and 13 … 17 stay `planned`; they need the PublicationList/Config state model that `test` builds anyway. The proxy endpoint, `test`, `list`, `explain` and `internal/sim` remain P5.
2. **Library (decision 7).** `eclipse/paho.mqtt.golang` **v1.5.1** under EDL-1.0, pinned in `go.mod`/`go.sum`. Indirect: `golang.org/x/net` v0.61.0, `golang.org/x/sync`, `gorilla/websocket` (all BSD-3-Clause). Toolchain go1.27.3. `govulncheck ./...` is clean.
3. **Test broker.** Instead of `mochi-mqtt/server` there is a test-only in-process TLS broker, `internal/mqttwire/mqtttest`, on the same paho `packets` codec. It needs no new dependency and no network beyond loopback. The dev-env Mosquitto (ADR-0026 §11) is used opt-in by `oec/conformance/live.sh`.
4. **TLS and keys.** Only `mqtts://` is accepted, with TLS 1.2/1.3 only. Credentials and/or a client certificate can be used. Encrypted PKCS#8 keys (PBES2, PBKDF2 and AES-CBC) are decrypted in the module (no new dependency). Connection settings can come from the RIG container storage (`--bus-storage`). `--insecure` is explicit, warned about, and recorded in the capture and the report (G-4.5). There is no auto-reconnect: a lost connection ends the observation and is reported.
5. **Capture format v1.** JSONL file: a header, one record per message, and an optional trailer (DESIGN.md §3.2). It is a superset of the `validate` message file, and `validate` reads captures.
6. **Verdicts (decision 4).** A REQ skipped as not applicable does not make a test worse than `pass`. A test is `skip` only when all its REQs are skipped. An inconclusive skip of a MUST leads to exit code 4.
7. **Self-test (decision 9).** For `monitor`, the positive and negative fixtures are generated **captures** with expected verdicts (`testdata/captures/index.yaml`, test data under G-3.9). They are not `internal/sim` faults. `internal/sim` stays the self-test of `test`.
8. **JSON report.** The monitor report has version 1 (`"report": 1`). `report.schema.json` (decision 6) follows with `test`, once the format covers both commands.

## Amendment C-062 (2026-10-28) – P5 increment 1: `test` engine, simulated CUT (agent, G-3.4 b)

Decided by the agent within this ADR; the PO can overrule. Details: `log/2026-10-28_C-062_d7-test-engine.md`, REVIEW A-153 … A-163; the "As built (C-062)" notes in `oec/conformance/DESIGN.md`.

1. **Delivered.** `internal/probe` (runner, bus session, verdict of a test run), `internal/sim` (simulated CUT with fault injection, DESIGN.md §10), `oec-check test` with the bus attachment. T7-GET-01, 02, 05 and 09 are `implemented`, each with a positive and at least one negative sim fixture (`testdata/sim/index.yaml`, test data under G-3.9). The other active tests follow the P5 plan in the ROADMAP.
2. **One engine per subscription.** A test run subscribes once to `Oi4v2/#` with QoS 1. Every message goes first to the monitor engine (and with `--record` to a capture), then to the runner. So the passive checks run during `test` (DESIGN.md §3.3) and a recorded test run can be replayed with `monitor --replay`.
3. **Requests.** D7 publishes its requests with QoS 0 (the default of REQ-MQTT-017) and a `ReplyTo`-able MessageId per request. The tester PublisherId is `Utility/oi4.org/oec-check/D7/<12 hex digits>`, new per run and not announced (Q-029); `--tester-il` sets the Identification Link. The requests are valid messages, except the deliberately unknown key of T7-GET-09.
4. **Discovery.** With `--app <ServiceType>/<AppId>` D7 sends `Get MAM/<AppId>` on that ServiceType, with a bare AppId on all seven. The first answer fixes the ServiceType. Without an answer, a publication of the AppId seen on the bus is enough. Not found: every REQ is inconclusive (exit 4), as in `monitor`. The CUT is treated as an application (it answers for itself and its subordinate assets).
5. **Waiting.** Tests run one after the other. A request that expects an answer waits until it is answered, then a grace of 250 ms for duplicates; a request that expects no answer waits the full `--timeout` (default 10 s).
6. **Verdict of a test run.** Each active test judges only the REQs its catalog entry cites. Passive requirements that a short test run cannot judge (connection not observed, observation shorter than the heartbeat, MaxPacketSize unknown) become plain skips with the reason prefix "test run:", not inconclusive ones. A level claim therefore needs a `monitor` run of at least the minimum duration in addition to `test` (DESIGN.md §6).
7. **Report.** `test` writes the monitor report (version 1) with `"mode": "test"` and the additive members `tester` and `requests`. `report.schema.json` follows in increment 7.
8. **Selection.** `--tests` takes test ids or `path.Match` patterns. Non-selected tests are reported as skip "not selected (--tests)".

## Amendment C-063 (2026-10-28) – P5 increment 2: the rest of `Get` (agent, G-3.4 b)

Decided by the agent within this ADR; the PO can overrule. Details: `log/2026-10-28_C-063_d7-get-tests.md`, REVIEW A-164 … A-175; the "As built (C-063)" notes in `oec/conformance/DESIGN.md`.

1. **Delivered.** T7-GET-03, 04, 06, 07, 08 and T7-ADR-02, 03 are `implemented`, each with a positive and at least one negative sim fixture (11 new faults, `draft`, G-3.9). Every `Get` test of the catalog is now implemented.
2. **Bounded stimuli.** T7-GET-03 requests at most 5 Filters per resource and 50 PublicationList entries; T7-GET-01/04 and ADR-03 at most 20 subordinate Sources. Get Metadata is requested only for the Data the PublicationList announces (REQ-CMP-044 is a MAY).
3. **What a test run cannot stimulate is judged passively.** Pagination by size (`MaxPacketSize`) is left to T7-MON-18; T7-ADR-02 listens to `Oi4/#` best effort and skips without `Oi4` traffic (it cannot make the CUT speak version 1).
4. **"Ignored" means "answered like without".** A Query on Get ReferenceDesignation/<AppId>, a Query on Get MAM and an undefined query parameter are judged against the answer to the same Get without it, not against an expected content.
5. **Locale.** An answer to a Get with the Locale `de-DE` may carry `de-DE`, the default language `en-US` or the invariant locale; whether the CUT supports German is not observable.
6. **The monitor engine reads the observed `Get` requests** (of D7 and of any other participant). An answer to a Get with a Locale is not compared with the Birth (REQ-LIFE-003), and an answer to a Get for one page (`Page` > 0, REQ-PAG-002) is not required to end the series (REQ-PAG-005). Both were false positives of the monitor during `test`; the capture fixture `pagination-broken` was corrected accordingly.

## Amendment C-064 (2026-10-28) – P5 increment 3: the write side (agent, G-3.4 b)

Decided by the agent within this ADR; the PO can overrule. Details: `log/2026-10-28_C-064_d7-set-tests.md`, REVIEW A-176 … A-190; the "As built (C-064)" notes in `oec/conformance/DESIGN.md`.

1. **Delivered.** T7-SET-01 … 09, 11 and 12 are `implemented`, each with a positive and at least one negative sim fixture (13 new faults, `draft`, G-3.9). T7-SET-10 (restart) stays with increment 6.
2. **Restore is the contract of every write test.** D7 writes back what the CUT published, deletes only what it created and undoes every change it provoked. Before the first and after the last write test it reads Config, PublicationList, ReferenceDesignation and SubscriptionList with Get without Source and compares them (Data is excluded: its values change by themselves; the disk of the CUT is not compared). The result is reported (`restore` in the JSON report, version 1 unchanged; a text line; a stderr warning with the changed lines), it is not a verdict on the CUT. A run that ends during the write tests reports the restore as not checked, with a note naming the test.
3. **Bounded destructive requests.** Only SubscriptionList entries D7 created are deleted; a ReferenceDesignation is deleted only with a known state and after a write-back Set of it got Status Good, and restored with Set; a Config DataSet with a masked sensitive value is written only where REQ-RES-CONFIG-025 is MUST (not at level 1); entries with Config `NONE_0` receive only a change that must be rejected (undone if it is accepted); Data is written only where the CUT offers it (listed Set subscription).
4. **The reaction to a Set or Del** is the Event of the category Status whose `ReplyTo` is the MessageId of the request (its level from the severity bits of the StatusCode, its Source the Source of the request or, for an asset the entity neither is nor represents, the AppId, SF-108) and, after a successful Set, the publication of the changed object with the same `ReplyTo`. "Nothing published" (REQ-BUS-007) is judged until the Status event plus the grace time.
5. **T7-SET-03 reuses the other write tests.** It runs after them, takes their reactions and stimulates only the missing combinations of REQ-CMP-033; supported means any reaction with the `ReplyTo` except BadNotSupported.
