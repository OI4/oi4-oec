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

## Amendment C-065 (2026-10-28) – P5 increment 4: the Action and the deferred monitor rules (agent, G-3.4 b)

Decided by the agent within this ADR; the PO can overrule. Details: `log/2026-10-28_C-065_d7-action-monitor.md`, REVIEW A-191 … A-201; the "As built (C-065)" notes in `oec/conformance/DESIGN.md`.

1. **Delivered.** T7-ACT-01 … 04 and T7-MON-09, 13, 15, 16, 17 are `implemented`, each with a positive and a negative fixture (5 new sim faults, 6 new capture fixtures, `draft`, G-3.9). **T7-MON-14 is cut to C-066** (increment 5): its PublicationList Mode/Interval/Precisions semantics needs a timing model of its own.
2. **The Action tests run inside the write window.** A DataSetWriterId the CUT provides is only reserved; D7 never uses it and creates nothing with it, so the restore check is unaffected. Responses are correlated by `RequestorId` and `RequestId` on any Reply topic of the CUT; the CorrelationData is judged, not used for correlation.
3. **Whether the CUT must answer** `NewDataSetWriterId`: it offers the service in its Profile, or REQ-CMP-040 is MUST at the declared level. Otherwise an unanswered request is a skip.
4. **Monitor rules on a state per participant** that survives reconnects (DataSet structures, Profile, RevisionCounters, Pub Configs merged per object, pending changes). A change counts only after Status Good; the republication (MON-15) and the larger RevisionCounter (MON-16) are expected within the tolerance (`--tolerance`, 5 s); missing evidence at the end is a skip, a plain one where the guideline allows the absence (Del SubscriptionList, REQ-RES-SUBLIST-015; no Profile and no publication for MON-13).
5. **MON-16 judges growth, not the exact +1**, and only for value changes of known, non-sensitive Config objects; a ReferenceDesignation change is not counted (A-113 stays as it is).


## Amendment C-066 (2026-10-28) – P5 increment 5: proxy attachment, connection and lifecycle tests (agent, G-3.4 b)

Decided by the agent within this ADR; the PO can overrule. Details: `log/2026-10-28_C-066_d7-proxy-lifecycle.md`, REVIEW A-202 … A-214; the "As built (C-066)" notes in `oec/conformance/DESIGN.md` §3.3, §3.4 and §10.

1. **Delivered.** The proxy attachment (§3.4, Q-028 (a)) and T7-CON-01 … 04, T7-LIFE-01 … 03 are `implemented`, each with a positive and a negative sim fixture (15 new sim faults, `draft`, G-3.9). **T7-MON-14 is cut again, to C-067** (increment 6): the proxy took the cycle, and the timing model of the PublicationList is independent of it.
2. **Own relay on the Go standard library** (`internal/proxy`), no new dependency: paho has only a client side and its decoder is lenient, while D7 must see the CONNECT and PUBLISH packets exactly as the CUT sent them. The relay decodes CONNECT and PUBLISH strictly and forwards every frame unchanged; downward it reads frame by frame for the CONNACK code and the first SUBACK.
3. **Certificates from a CA the CUT trusts.** `--proxy-ca`/`--proxy-ca-key` are given by the tester (test material, generated in a temporary directory, never committed); D7 issues the relay and the wrong-host certificates from it and an untrusted CA of its own. The upstream connection uses D7's TLS settings and the CUT's CONNECT; a client certificate of the CUT is seen, not relayed.
4. **Order of a proxy run.** D7 waits for the CUT's relayed connection with a PUBLISH and the first SUBACK (bounded by CONNECT + `--timeout`) before the discovery Get – a conforming CUT may subscribe after its Birth. The connection phase runs after all other tests: TLS 1.1, untrusted CA, wrong host, then the drop (Will) and the reconnection with a Birth. Without the proxy CON/LIFE are inconclusive skips (exit 4).
5. **Monitor corrections found by the new tests:** T7-MON-03 reports a missing Birth after a Will only when the connection or observation ends without one; T7-MON-10 does not count a recovery or late Birth as a second Birth.
6. **Self-test through the proxy.** Every sim fixture runs through the proxy and the sim starts independently of D7; an index entry may select its tests (`tests`), and an expected test without a result is a harness error.

## Amendment C-067 (2026-10-29) – P5 increment 6a: restart hook, T7-LIFE-04, T7-SET-10 and T7-MON-14 (agent, G-3.4 b)

Decided by the agent within this ADR; the PO can overrule. Details: `log/2026-10-29_C-067_d7-restart-publist.md`, REVIEW A-215 … A-221; the "As built (C-067)" notes in `oec/conformance/DESIGN.md` §3.3, §3.4 and §10.

1. **Delivered.** The restart hook (§3.4) with T7-LIFE-04 and T7-SET-10, and the monitor rule T7-MON-14, are `implemented`, each with a positive and a negative fixture (4 new sim faults, 5 new capture fixtures, `draft`, G-3.9). **Increment 6 is split** (A-215): the peer simulation, `--announce` (Q-029 (a)) and T7-ADR-01 follow in C-068 (6b); `list`/`explain`, the report schema and the P5 exit run move to C-069.
2. **The hook is a tester-provided command,** `--restart-cmd` or `--stop-cmd` + `--start-cmd`, run by the shell, bounded by `--restart-wait`. D7 does not stop processes or containers itself: it cannot know how the CUT is deployed. Without the hook the restart tests are inconclusive skips (exit 4), as are the proxy tests without the proxy.
3. **The restart phase runs in the write window,** after the other write tests and before the restore check: T7-SET-10 changes the ReferenceDesignation before the restart and restores it after the read-back, so the restore check also covers the restart. The Close message is the last Pub Health of the CUT before its DISCONNECT (proxy) or before its new Birth (bus); a stop without DISCONNECT, or the Will, makes T7-LIFE-04 inconclusive, because D7 cannot tell a faulty hook from a faulty CUT.
4. **T7-MON-14 judges against the last observed PublicationList** with its own time model (A-191): per entry and topic form the time of the last unsolicited publication; changes of the list (Set/Del addressed to the component, a new entry content, a connect) start a tolerance window in which nothing is judged; Get answers with a significant deviation create an expectation of an unsolicited publication within the tolerance (REQ-RES-PUBLIST-013, REQ-MQTT-019). A component that publishes without an observed PublicationList is an inconclusive skip. Not judged: OFF_0 (T7-SET-08), Event intervals, the completeness of the forms of a Mode, localized Get answers.

## Amendment C-068 (2026-10-29) – P5 increment 6b: peer simulation, `--announce` and T7-ADR-01 (agent, G-3.4 b)

Decided by the agent within this ADR; the PO can overrule. Details: `log/2026-10-29_C-068_d7-peer-announce.md`, REVIEW A-222 … A-228; the "As built (C-068)" notes in `oec/conformance/DESIGN.md` §3.3, §3.4, §3.5 and §10.

1. **Delivered.** Increment 6b is complete (A-222): `--peer`, `--announce` (Q-029 (a)) and T7-ADR-01 `implemented` with a positive and two negative sim fixtures (3 new faults, `draft`, G-3.9); the fixture `none` runs with peer and announce and is `draft` again. All 54 D7 tests are `implemented`. C-069 stays as planned (increments 7/8).
2. **One announced-component type serves the tester and the peer** (A-223, A-224): a Level-1 `Utility` with Will, Birth (MAM, Health `NORMAL_0`/100), a 30 s heartbeat and the Close message, answering Get MAM, Health and Profile. The tester (`--announce`) lives on D7's main connection under the tester IL, which must then be an Oi4Identifier; the peer (`--peer`) has its own connection and the IL `oi4.org/oec-check/Peer/<random>`. An unknown Source gets Event Status Bad `BadNotFound` (REQ-CMP-047), a Filter an empty list, other resources are ignored; a lost peer connection ends the run. Default stays unannounced (Q-029 (a)).
3. **"Present" Health** (A-225): once the CUT is discovered, each announced component publishes its Health once more, so a CUT that connected after their Births (always through the proxy) can discover them without retained messages.
4. **T7-ADR-01 judges each request of the CUT to the peer** (A-226): the CUT is the PublisherId and the topic carries the peer's AppId or the peer as Source; the topic must be the peer's request topic (ServiceType REQ-TOP-006, AppId REQ-TOP-010, both REQ-BUS-002), one violation per distinct topic, judged at the end of the run. Without `--peer` an inconclusive skip (D7 cannot provoke the request), with the peer but without a request a plain skip (the REQs bind a requester only). Requests to the announced tester are counted, not judged.
5. **The simulated CUT is a consumer** like the SDK directory (A-227): it asks every newly seen application for its MAM; its consumer subscriptions follow its request subscriptions because D7 takes the first SUBACK as ready.
6. **Report** (A-228): key `announced` (role, PublisherId, requests received, answered); the report version stays 1, the key is only added.

## Amendment C-069 (2026-10-29) – P5 increments 7/8: `list`/`explain`, report schema, coverage report, exit run (agent, G-3.4 b)

Decided by the agent within this ADR; the PO can overrule. Details: `log/2026-10-29_C-069_d7-list-explain-report-schema-p5-exit.md`, REVIEW A-229 … A-238; the "As built (C-069)" notes in `oec/conformance/DESIGN.md` §2, §6, §7 and §10.

1. **Delivered, not split** (A-229): `oec-check list`, `explain` and `report-schema`; `report.schema.json` (report version 1 frozen); the coverage report `catalog/COVERAGE.md`; the P5 exit run against `demo.rs` (level 2, proxy, restart hook, peer, announce: 49 pass, 0 fail, 6 skip, exit 0, restored). No new test, fault or fixture. The exit assessment is in ROADMAP P5; the PO confirms it (REVIEW D-1).
2. **Module root package `conformance`** (A-230): `report.schema.json` with `ValidateReport` and the loaders of the fixture indexes live in the module root, because `go:embed` cannot reach `testdata/` from a sub-package and `internal/report` imports `probe` (so `probe`'s sim tests cannot import `report`). It imports no tool package.
3. **Report schema** (A-231, A-232): JSON Schema 2020-12 of report version 1 of `monitor` and `test` with closed objects and the conditional keys of the modes and attachments; `validate`'s report is not covered. Every report the gate produces is validated: the CLI tests, every capture replay at levels 1–3, and the report document of every sim fixture run (assembled with the keys of `report.Monitor`). A request's `test` is a test id or the phase `discovery`, `state` or `restart`; the preparatory Profile read is labelled `discovery` (it had an empty label – found by the schema). A changed key or meaning is a new report version.
4. **`list` and `explain` read only embedded data** (A-233): the catalog, `spec.FS` and the fixture indexes; `explain` shows the fixtures statically with the outcome their index states (the self-tests check them dynamically); a positive sim fixture without the peer is not shown for a test that needs the peer. `version` names the catalog baseline and the report version.
5. **Coverage report in the conformance module** (A-234), not in `spectool` as DESIGN §6 planned: the tools of `oec/spec` must not import `conformance/` (ADR-0018 §5). `catalog/COVERAGE.md` is generated by `internal/explain` and diffed by the gate; `TestFixtureCoverage` enforces a negative (fail, warn or info) and a positive fixture for every implemented test.
6. **Exit interpretation** (A-236, A-237): for a SHOULD or a heuristic rule (T7-GET-09, T7-SET-12, T7-MON-10) the negative fixture expects the outcome of its obligation (`warn`/`info`), not `fail`. CI on Windows/macOS/Linux and the container image come with P9; C-069 records the native macOS arm64 gate and `CGO_ENABLED=0` cross builds for windows/linux/darwin × amd64/arm64.

## Amendment C-070 (2026-10-29) – P5b plan and increment 1: `oec-infra image` (agent, G-3.4 b)

Decided by the agent within this ADR; the PO can overrule. Details: `log/2026-10-29_C-070_p5b-plan-d8-image.md`, REVIEW A-239 … A-248; ROADMAP P5b (plan, cycle cut, exit); the "As built (C-070)" notes in `oec/conformance/DESIGN.md` §2, §9 and §10.

1. **No go-containerregistry** (A-239, changes decision 7): the OCI reader `internal/oci` and the SBOM reader `internal/sbom` use only the standard library (`archive/tar`, `compress/gzip`, `crypto/sha256`/`sha512`, `encoding/json`). D8 reads local layouts only, so the registry client – the main reason for the library – is not needed; the reader needs digest verification per blob and a layer file system (whiteouts, symlinks inside the image root), which go-containerregistry does not give in the form the rules need. No new dependency.
2. **Inputs of `image`** (A-240): an OCI image layout directory or an uncompressed tar of it (oci-archive: `docker save` of Docker 25 or later, `skopeo copy … oci-archive:`, `podman save --format oci-archive`). A registry reference, a Docker archive with only `manifest.json` and a compressed tar are input errors (exit 2) with a hint; registry access stays possible later (P9) but is not planned.
3. **Reading rules** (A-241): Docker v2 schema 2 media types are accepted, schema 1 is a structure problem; attestation manifests (buildx) and `unknown` platforms are not runnable images; zstd layers make the file system unreadable (no decoder in the standard library) → the SBOM test is an inconclusive skip (exit 4).
4. **Platforms** (A-242): supported are linux/amd64, linux/arm64 (v8) and linux/arm (v7) (RIG 5.1); no supported runnable image fails REQ-APP-001 and REQ-CMP-032; REQ-APP-002 (multi-architecture) needs at least two of them.
5. **Integrity** (A-243): T8-IMG-04 warns and never fails (note of REQ-CMP-030, SF-091 (f)); a digest mismatch fails REQ-CMP-027 in T8-IMG-01; a digest algorithm other than SHA256 warns; signatures are not checked (RIG 4.3 requires none).
6. **Report** (A-244): own schema `infra-report.schema.json` (`$id` `…/oec-infra/report.schema.json`, report version 1, closed objects, test ids `T8-`), independent of the D7 report version. Version 1 may grow additively (new optional keys, new modes) until the P5b exit and is frozen then. `verdict.Evidence` gains `image` and `path` (D7 never sets them; the D7 schema is unchanged). The JUnit writer, the test listing and the summary line of `internal/report` are shared; the D7 output is byte-identical.
7. **SBOM rules** (A-245, SF-114 proposed): exactly one of `sbom.spdx.json` (SPDX 2.2/2.3 JSON) and `sbom.cdx.json` (CycloneDX 1.x JSON) per distinct layer set; every license id and `LicenseRef-` of an expression needs an embedded text or a non-empty `<id>.txt`; a name-only CycloneDX license needs an embedded text; symlinks under the folder must resolve inside the image (REQ-CNT-015); a `VOLUME` on the folder, packages without a license and references to other SPDX documents are notes (`info`).
8. **Image fixtures** (A-246): generated by `go test ./internal/image -run TestFixtureFiles -update` (`internal/oci/ocitest`, deterministic), byte-compared in the gate, expected verdicts in `testdata/images/index.yaml` with the semantics of the sim index; every implemented image test needs a positive and a negative fixture.
9. **P5b plan** (A-247): six increments, `image` → `bus` → `host` → Engine API + `container` → `run` → `list`/`explain`/exit; exit criteria precised (self-test against purpose-built good and faulty images; the demo and registry images follow with P6/P7; CI with P9).

## Amendment C-070 review (2026-10-29) – SBOM rules after SF-114 (agent, G-3.4 b)

Decided by the agent within this ADR after the PO decision on SF-114 (RIG 4.1.4 at `a55f475`); the PO can overrule. Details: C-070 log addendum 3, REVIEW A-249. Replaces item 7 of amendment C-070 (A-245).

1. **Formats:** exactly one of `sbom.spdx.json` and `sbom.cdx.json` per distinct layer set (unchanged). `sbom.spdx.json` is SPDX 2.x JSON (`spdxVersion` `SPDX-2.x`) or SPDX 3.x JSON-LD (`@context` `https://spdx.org/rdf/3.x/spdx-context.jsonld`); `sbom.cdx.json` is CycloneDX (`bomFormat`, `specVersion`). A document that is neither fails REQ-CMP-028/REQ-CNT-013; a version or construct D8 cannot read (SPDX other than 2.x/3.x, CycloneDX other than 1.x, an unknown SPDX 3 license type) is not a violation: REQ-CMP-028 passes and the content REQs are skipped inconclusive (exit 4).
2. **Components:** SPDX packages (2.x) and `software_Package` elements (3.x), CycloneDX `components` (nested); the CycloneDX `metadata.component` is the subject of the SBOM, its licenses need texts but it needs no identifier; SPDX files and snippets are no components. No name or no version → REQ-CNT-013 fails; no stated license (`NOASSERTION`, none) → REQ-CNT-026 fails, `NONE` is a statement; no identifier (purl, CPE, SWID, SPDX `PACKAGE-MANAGER`/`PERSISTENT-ID` reference, SPDX 3 `externalIdentifier` packageUrl/cpe22/cpe23/swid/gitoid/swhid, CycloneDX omniborId/swhid) → REQ-CNT-027 warns (SHOULD).
3. **Texts:** every license id, `LicenseRef-`, exception id after `WITH` and `AdditionRef-` needs an embedded text or a non-empty `<id>.txt` (REQ-CNT-014, REQ-LIC-002); a name-only CycloneDX license needs an embedded text. License ids are checked for syntax only, not against the SPDX license list (it changes over time).
4. **Notes:** references to other SPDX documents (`DocumentRef-`, SPDX 3 imports) and a `VOLUME` over `/opt/oi4` are notes (`info`); an empty component list is a note. Still standard library only (`encoding/json`, `regexp`); no SPDX or CycloneDX library.

## Amendment C-071 (2026-10-29) – P5b increment 2: `oec-infra bus` (agent, G-3.4 b)

Decided by the agent within this ADR; the PO can overrule. Details: C-071 log, REVIEW A-250 … A-258, DESIGN.md §9 "As built (C-071)".

1. **Command and connection (A-250):** `oec-infra bus` with the connection flags of `oec-check monitor` (`--broker` / `--bus-storage`, `--ca`, `--cert`/`--key`/`--key-passphrase-file`, `--username`/`--password-file`, `--insecure` only explicitly, `--config`), duplicated in `cmd/oec-infra` (the CLIs share no command code); no `--client-id` (D8 opens several connections with random ClientIds). Level 3, `input` = broker URL. D8 publishes only on the topics of its own probe AppId `Utility/oi4.org/oec-infra/D8/<12 hex>` and of its probe components (`…-1`, `…-2`).
2. **Second client (A-251):** `--credentials-2 FILE` (base64 `user:password`, the format of `run/secrets/oi4/credentials`, RIG 4.1.3) or `--bus-storage-2 ROOT` (mutually exclusive); it uses the broker, CA and `--insecure` of the main connection. Without it, or with the same identity as the main connection, T8-BUS-02 is inconclusive (not a usage error).
3. **T8-BUS-01 (A-252, A-253):** raw TLS handshakes (standard library) for exactly 1.2 and exactly 1.3 (one must succeed) and for 1.0 … 1.1 (must fail), all presenting the client certificate of the run (a listener that requires one fails TLS 1.2 without it – found in the live run, C-071); a plain-text MQTT listener on `--plain-port` (default 1883, repeatable, 0: none) fails only if it answers CONNECT with CONNACK, an open port without CONNACK is a note; the certificate chain is verified against the CA of the run, a self-signed certificate fails. The MaxPacketSize probe (SF-105) is one QoS 1 PUBLISH with a payload of exactly MaxPacketSize on a topic padded to 65535 octets, delivered back to D8 without a disconnect; MaxPacketSize from `--max-packet-size`, else `broker.json`, else 262144 with a note; above `--max-probe-size` (default 64 MiB) inconclusive.
4. **SF-115 (D-1):** a certificate that does not name the broker host or is outside its validity violates REQ-MQTT-007 "(SF-115, proposed)"; if the PO rejects the proposal these findings become notes. Decided as proposed in the C-071 review, see amendment C-071 review.
5. **T8-BUS-02 (A-254):** the unprivileged client publishes Set Config/Data/Metadata/PublicationList/ReferenceDesignation/SubscriptionList, Del ReferenceDesignation/SubscriptionList and Call NewDataSetWriterId to the probe AppId; a positive control (D8 receives the same requests of its own) comes first; a request that arrives violates REQ-GEN-001 and the requirement of the resource; a readable Set/Config (subscription granted and a message delivered) warns REQ-RES-CONFIG-026 (SHOULD). A broker may deny silently, with SUBACK 0x80 or by disconnecting.
6. **T8-BUS-03 (A-255):** the Registry is `--registry IL` or the first publisher on `Oi4v2/Registry/+/Pub/#` within `--registry-wait` (default 65 s: one heartbeat of SF-106 plus slack); two probe components announce themselves (Will, Birth, a second Health); Get MAM / Get Health with and without Source must list both with the last Health, under the Registry's own PublisherId and the asset's DataSetWriterName (an asset found only by payload fails the name); after the Close message of the first and the Will of the second neither may be listed (polled up to 3 × `--timeout`). No Registry → inconclusive skip; against `oec/dev` until P7.
7. **Self-test (A-256):** `mqtttest` gains options (additive, `NewBroker` unchanged, D7 gate unchanged): TLS version bounds, broker certificate variants, plain-text listener, ACL callback with optional disconnect, payload and packet limits, protocol levels; the test PKI is created at run time. `internal/bus/registrytest` simulates the Registry with seven faults. `testdata/bus/index.yaml`: 29 broker scenarios (6 positive), `draft` (G-3.9).
8. **Report (A-257):** `infra-report.schema.json` grows additively: `bus` in `command`/`mode`, key `broker` (url, probe, tls, certificate, max_packet_size, registry; required for bus, forbidden for image), evidence `topic`. Report version stays 1.
9. **Dev broker ACL (A-258):** `oec/dev/mosquitto/acl`: only `oec-probe` publishes Set, Del and Call; every client reads `Oi4v2/#` and publishes Pub, Get and Reply (Reply was missing at first; found by the D7 demo run, T7-ACT-01/02/04). Mosquitto cannot bind the AppId level to a user and applications read their own Set/Config, so T8-BUS-02 warns REQ-RES-CONFIG-026 against `oec/dev` (a limitation of the dev environment, not of the guideline).
10. No new dependency (paho v1.5.1 `packets`, standard library). `host` (increment 3) is not pulled forward.

## Amendment C-071 review (2026-10-29) – SF-115 decided (agent, G-3.4 b)

The PO decided D-1 as proposed ("D-1 wie vorgeschlagen"): MG 6.2.2 (guideline `b955783`) requires on the broker side that the broker certificate is valid for the broker address of the Message Bus storage (`Address` of `broker.json`; a host name as DNS name, an IP address as IP address of the Subject Alternative Name) and within its period of validity; REQ-MQTT-007 follows.

1. T8-BUS-01 keeps its verdicts: a certificate for another host or outside its validity violates REQ-MQTT-007. The findings cite "(MG 6.2.2)" instead of "(SF-115, proposed)" (`internal/bus/protocol.go`, test `TestCertificateRule`).
2. D8 checks the host of the broker URL of the run (`--broker`, or `Address` of `broker.json` with `--bus-storage`), as before; the report and its schema are unchanged.
