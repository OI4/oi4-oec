# C-067 – P5 increment 6a: restart hook, T7-LIFE-04, T7-SET-10 and T7-MON-14

- **Date:** 2026-10-29
- **Phase:** P5 – Conformance test tool (D7 `test`), increment 6a (increment 6 split, A-215)
- **Baseline:** guideline `eb7889c` (branch head `4fac7d2`, findings only), SF-001 … 113 resolved; 1406 inventory items, 442 REQs, 158 `schema`/`d7-validate` REQs, 0 gaps, no REQ drafts; 29/29 entry schemas, 245 payload vectors, 18 `broker.json` vectors; schema `2.0.1-dev` @ `eb7889c`; at the start capture fixtures 24 (18 reviewed, 6 draft), sim fixtures 52 (31 reviewed, 21 draft). The guideline did not change.
- **Agent:** the C-066 agent, continued on the PO's "continue" after the C-066 handover

## Intent

PO order: "continue" after C-066, i.e. the next cycle of the ROADMAP P5 plan: increment 6 (restart hook, peer simulation and `--announce` (Q-029), T7-LIFE-04, T7-SET-10, T7-ADR-01) plus T7-MON-14 (cut from C-065 and C-066), test-first, every test with a positive and a negative fixture (`draft` → R-x, G-3.9), then a run against `demo.rs`.

Open for the PO at the start: R-1/R-2 of C-065 and R-3 of C-066. **No answer came during the cycle; they stay open.**

## Cut (agent, G-3.4 b)

Increment 6 is split (A-215):

- **C-067 (6a):** the restart hook, T7-LIFE-04, T7-SET-10 and T7-MON-14.
- **C-068 (6b):** the peer simulation, `--announce` (Q-029 (a)) and T7-ADR-01. A second simulated component with its own lifecycle, which the CUT discovers and addresses, is new infrastructure of the size of the proxy; T7-ADR-01 needs it.
- **C-069:** increments 7/8 (`list`/`explain`, report schema, the P5 exit run).

Reasons: T7-MON-14 was cut twice and comes first; the restart hook is small and closes two tests. ROADMAP: plan item 6 carries the split note, the table has rows C-067 ✅, C-068 and C-069.

## Work

### 1. Restart hook (`cmd/oec-check/restart.go`, `internal/probe/restart.go`)

**Decision: the restart is a command given by the tester (A-216).**
- `--restart-cmd CMD`, or `--stop-cmd CMD` and `--start-cmd CMD`; run by `sh -c` (Windows `cmd /C`), output to stderr, a 10 s `WaitDelay` for output of a background start.
- `--restart-wait` (default 2 min) bounds the wait for the CUT to come back.
- D7 does not stop processes or containers itself: it cannot know how the CUT is deployed (systemd, Docker, a script, a device).
- Without the hook T7-LIFE-04 and T7-SET-10 are inconclusive skips ("needs the restart hook"), the run ends with exit 4 unless `--allow-skip` (like the proxy, A-210).

**Restart phase (A-217).** It runs inside the write window, after the other write and Action tests and before the restore check:
1. T7-SET-10 sets a changed ReferenceDesignation (A-219).
2. D7 starts the hook and records every publication of the CUT from then on.
3. D7 waits for the CUT's new Birth – through the proxy also for its new connection and first SUBACK – and an answer to Get Health, bounded by `--restart-wait`.
4. T7-LIFE-04 and T7-SET-10 judge; T7-SET-10 restores the original object.

The restore check that follows therefore also covers the restart.

**T7-LIFE-04** (REQ-CMP-046, REQ-LIFE-005/006/007; A-218):
- The Close message is the last Pub Health of the CUT (not an answer) after the hook started and before its DISCONNECT (proxy) or before its new Birth (bus attachment).
- REQ-LIFE-005: present. REQ-LIFE-006: on the Health topic of the application. REQ-LIFE-007: Health `NORMAL_0`, HealthScore 0, with the DataSetWriterId of the application's Health, which D7 reads with Get Health after the restart. REQ-CMP-046 grades them.
- PUBLISH packets after the Close message and before the DISCONNECT are a note.
- Inconclusive: no hook, the connection did not end, no DISCONNECT (proxy: the Will instead), the Will on the bus, no new Birth within `--restart-wait`.

**T7-SET-10** (REQ-RES-REFDES-002; A-219):
- D7 changes the ReferenceDesignation of the application (or else the first object): "D7" is appended to Value and Local of its first aspect (Function, Product, Location; a Location `+D7` if none).
- After the restart Get ReferenceDesignation must return exactly the set object.
- Plain skip (not applicable) without a ReferenceDesignation object or with Status Bad on the Set: the condition of the REQ (the CUT offers Set ReferenceDesignation) is not met.

### 2. Simulated CUT and sim fixtures

- `Sim.Restart`: Close message, DISCONNECT, reconnect with a new Will and Birth; serialized with the reconnect loop of the proxy attachment.
- Faults (all `draft`, R-4):
  - `T7-LIFE-04/no-close` – DISCONNECT without a Close message;
  - `T7-LIFE-04/writer-id` – Close message with DataSetWriterId 3999;
  - `T7-LIFE-04/abort` (positive variant) – the connection breaks without DISCONNECT; LIFE-04 is an inconclusive skip, not a failure;
  - `T7-SET-10/not-persisted` – the set ReferenceDesignation is lost on the restart.
- The harness wires the restart hook to every sim fixture (the 31 reviewed ones pass LIFE-04 and SET-10); `TestSimNoRestart` checks the skip without the hook.
- `T7-LIFE-02/no-birth` and `/health-first` (draft, R-3) select T7-LIFE-01 … 03 instead of all LIFE tests (A-221): with the restart the monitor would judge the reconnect after the Close message, and their MON-03 outcome would move. Their expected verdicts are unchanged.
- `none` (draft, R-3): the note says LIFE-04 and SET-10 pass with every requirement judged.
- `cmd/oec-check/test_test.go`: `TestRestartHook` runs `--stop-cmd` and `--start-cmd` in this order.

### 3. T7-MON-14 (`internal/monitor/publist.go`)

**Time model (A-220).**
- The rules of a component are the entries of its last observed Pub PublicationList (Mode, Interval, Precisions; an absent Mode is `ON_REQUEST_1`).
- A Set or Del PublicationList addressed to it makes the rules uncertain until its next Pub PublicationList.
- Within the tolerance (`--tolerance`, 5 s) after a changed entry or a connect nothing is judged; Birth, Will and Close are never judged.

**Judgments** (REQ-RES-PUBLIST-008 … 015, REQ-MQTT-019):
- Mode `ON_REQUEST_1`: no unsolicited publication (008; not MAM/Health, which have their own rules).
- Every unsolicited publication uses a topic form its Mode enables (009 … 011): application, Source, Filter, and their combinations.
- Interval > 0: the gap between publications of an entry per form ≤ Interval + tolerance, also up to the end of the observation (013; with Precisions also 014, because Precisions do not suspend the Interval).
- Interval 0 with Precisions: a publication after a change below every Precision is a SHOULD finding (015).
- Interval 0: if a Get answer shows a significant change against the last unsolicited publication, the change must be published unsolicited within the tolerance (013, REQ-MQTT-019).

**Not judged (limits, A-220):** `OFF_0` (T7-SET-08 does), Event intervals, completeness of the enabled forms (a form that is never used), several DataSetMessages in the filter form, Get answers with a Locale. A component that publishes without an observed PublicationList is an inconclusive skip; `oec-check test` requests the list in discovery, `monitor` needs it on the bus.

"A Mode enables a form" is read as "an unsolicited publication must use an enabled form". This matches the reviewed REQ-RES-PUBLIST-009 … 011; no SF.

**Capture fixtures** (all `draft`, R-5), built in `fixtures_test.go` from reviewed payload vectors with the new helpers `publistOf` and `getAt`:
- `publist-pass` (positive, 32 s);
- `publist-mode` (fail 008, 011), `publist-interval` (fail 013, 014), `publist-precision` (warn 015), `publist-change` (fail REQ-MQTT-019, 013);
- `lifecycle-pass` changed (see "Changes to reviewed items").

### 4. Catalog, CLI, docs

- `catalog/d7.yaml`: T7-LIFE-04, T7-SET-10 and T7-MON-14 `implemented`; notes describe the judgments above. Only T7-ADR-01 is still `planned`.
- `oec/conformance/README.md`: state line, the restart hook with an example, the monitor list (19 of 19), the layout.
- `DESIGN.md`: "As built (C-067)" in §3.3, the restart hook in §3.4, §10.
- ADR-0025 amendment C-067.

## Defects found and fixed (test first)

1. **`lifecycle-pass` became inconclusive** (`TestMonitorReplayExitCodes`, exit 4 instead of 0): the peer B of the reviewed capture publishes without a known PublicationList, so T7-MON-14 cannot judge it. Fix in the fixture: A sends Get PublicationList to B at 11 s, B answers at 11.1 s with MAM and Health (`ON_REQUEST_APPLICATION_SOURCE_FILTER_15`, Interval 0).
2. **The first version of that Get failed T7-MON-01** (REQ-MSG-011): a Get of a predefined resource must carry its DataSetClassId. The test helper `getAt` now sets the DataSetClassId for `PublicationList/` paths.

Both were defects of the test data, not of D7 or the SDK. The demo run found nothing.

## Runs

**Demo** (`oec/sdk/rust/target/debug/examples/demo`, dev broker Mosquitto from `oec/dev`, port 8883), as in C-066 plus the restart hook:
- `OEC_ROOT` = a copy of the `oec-dev-app` root under `/tmp/c067-demo`; in the copy `etc/oi4/certs/ca.pem` is the proxy CA (openssl EC P-256, key as `pkcs8 -nocrypt`, generated there) and `etc/oi4/mqtt/broker.json` points to `localhost`, SecurePort 18883.
- `start.sh`: starts the demo with `nohup` and `OEC_ROOT` in the background, output to `demo.log`, its PID to `demo.pid`.
- `stop.sh`: SIGTERM to the PID, waits up to 10 s for the process to end.

```
oec-check test --broker mqtts://localhost:8883 --ca ../dev/generated/broker/ca.pem \
    --username oec-probe --password-file ../dev/generated/probe/password \
    --app 'OTConnector/example.com/OEC,20Demo/DEMO-1/oec-dev-app' --level 2 \
    --proxy-listen 127.0.0.1:18883 --proxy-ca /tmp/c067-demo/ca.pem --proxy-ca-key /tmp/c067-demo/ca8.key \
    --stop-cmd /tmp/c067-demo/stop.sh --start-cmd /tmp/c067-demo/start.sh \
    --record test.jsonl --report json=report.json
```

Then `start.sh` once.

**Run** (after `d1addc1`, final code): **48 pass, 0 fail, 6 skip, result PASS, exit 0.**
- 372 messages in 64.4 s, 226 of them from the CUT, 0 invalid; 3 Births, 1 Will, 1 Close message.
- 146 requests, 143 answered. The 3 unanswered are the deliberate T7-ACT-03 stimuli, as in C-065/C-066.
- **restore: `{checked: true, restored: true}`** – "the state of the CUT after the write tests equals the state before them".
- Proxy, 6 connection attempts:
  - #0 relay: TLS 1.3, MQTT/4, ClientId `oec-dev-app`, CONNACK 0, 195 PUBLISH; ended by the restart with the Close message and DISCONNECT (the demo handles SIGTERM in `runtime::signals`: Close, then DISCONNECT).
  - #1 relay: the restarted demo, CONNACK 0, 18 PUBLISH, Birth first; dropped in the connection phase.
  - #2 tls1.1, #3 untrusted-ca, #4 wrong-host: refused.
  - #5 relay: reconnect, CONNACK 0, 12 PUBLISH, Birth first.
- **T7-LIFE-04 passes:** REQ-CMP-046, REQ-LIFE-005, 006 and 007 all judged.
- **T7-MON-14 passes:** REQ-RES-PUBLIST-011 judged (the demo's Data and the MAM/Health use enabled forms); the others are not applicable ("nothing to check was observed": the demo has no Interval > 0, no Precisions and no `ON_REQUEST_1` entry).
- **T7-SET-10** is a plain skip: the demo has no ReferenceDesignation object (T7-SET-09 creates and deletes one within its test).
- T7-LIFE-02 skips REQ-LIFE-017 (no `FAILURE_1`/0), as in C-066.

Skipped tests:
- T7-MON-07: the demo sends no Calls.
- T7-MON-17: no Set Config from the CUT.
- T7-ADR-01: `planned` ("not implemented yet").
- T7-ADR-02: no `Oi4` traffic.
- T7-SET-03: level 3 is above the declared level 2.
- T7-SET-04: no whole object to delete (as in C-065/C-066).
- T7-SET-10: see above.

Compared with C-066 (46 pass, 5 skip): + LIFE-04, + MON-14 (pass), + SET-10 (skip). T7-ADR-01 is listed as "not implemented yet" and does not count, as the planned tests in C-066.

**The SDK is unchanged:** `demo.rs` passes every implemented test.

**Housekeeping:** the demo was stopped, the dev broker shut down with `docker compose down`; `oec/dev/generated/` is untouched; the root copy and the proxy CA under `/tmp/c067-demo` were deleted.

**Also verified (temporary test, not committed):** the sim fixture `none` with the restart hook passes T7-LIFE-04 (all 4 REQs) and T7-SET-10 (REQ-RES-REFDES-002) – i.e. SET-10 is exercised by the gate, even though the demo has no ReferenceDesignation.

## Agent decisions (G-3.4 b, for information: REVIEW A-215 … A-221)

- **A-215 Cut:** see "Cut".
- **A-216 Restart hook:** a tester command (`--restart-cmd`, or `--stop-cmd` + `--start-cmd`), run by the shell, `--restart-wait` 2 min; D7 manages no processes or containers.
- **A-217 Restart phase:** in the write window, before the restore check; SET-10 changes, restarts, judges and restores.
- **A-218 LIFE-04:** the last Pub Health before DISCONNECT (proxy) or before the new Birth (bus) is the Close message; the inconclusive cases.
- **A-219 SET-10:** the change ("D7" appended to the first aspect); not applicable without a ReferenceDesignation object or with Status Bad.
- **A-220 MON-14:** time model, judgments, limits; no SF for "enables".
- **A-221 Harness and fixtures:** the restart for every sim fixture, `TestSimNoRestart`, LIFE-02 selections narrowed, `lifecycle-pass` with B's PublicationList, `getAt` with the DataSetClassId.

## Changes to reviewed items

- **Capture fixture `lifecycle-pass`** (reviewed in C-052): B now answers A's Get PublicationList (defect 1), so T7-MON-14 can judge B. All other verdicts are unchanged and the capture still passes. Because its content changed, it is back to **`draft`** (R-5).
- **The 31 reviewed sim fixtures** are unchanged in the index. The restart hook now runs on them too; LIFE-04 and SET-10 pass on them (none of their faults concerns the restart), so their expected verdicts still hold. They stay `reviewed`.
- **The other 17 reviewed capture fixtures** are unchanged; T7-MON-14 passes or is skipped on them.
- **Draft sim fixtures of R-3:** `none` (note extended), `T7-LIFE-02/no-birth` and `/health-first` (`tests` narrowed, verdicts unchanged).
- **Catalog:** LIFE-04, SET-10 and MON-14 `planned` → `implemented`; the cited REQs are unchanged.
- No REQ, vector, schema or guideline change.

## Spec findings

None. The cited REQs were read against the guideline and found consistent:
- MG 6.3.4 / REQ-LIFE-005 … 007: the Close message is a Health `NORMAL_0`/0 on the application's Health topic with its DataSetWriterId, before a graceful disconnect;
- MG 8.3.10 / REQ-RES-REFDES-002: a set ReferenceDesignation persists over a restart;
- MG 8.3.8 (PublicationList, SF-067) / REQ-RES-PUBLIST-008 … 015, REQ-MQTT-019: Mode, Interval and Precisions; the reading of "enables" (A-220) matches the reviewed REQ wording.

Next free: SF-114.

## Dependencies

None. `go.mod` is unchanged; the hook uses `os/exec`.

## Risks and open technical points

- **The hook depends on the tester:** a stop command that kills the CUT (SIGKILL, `docker kill`) gives no graceful disconnect; LIFE-04 is then inconclusive, not a failure (`T7-LIFE-04/abort`).
- **SET-10 against the demo** is not applicable (no ReferenceDesignation object); it is exercised only by the sim fixtures.
- **MON-14 against the demo** judged only REQ-RES-PUBLIST-011; Interval, Precisions and `ON_REQUEST_1` are exercised only by the captures. The limits of A-220 (OFF_0, Event intervals, completeness, Locale) remain.
- **Tolerance:** MON-14 uses `--tolerance` (5 s); a CUT with a heavily loaded bus may need a larger one.
- **Gate time:** the sim fixtures now also restart the simulated CUT; the probe package takes a little longer.

## Commits

- `0f9d3ad`: restart hook, T7-LIFE-04, T7-SET-10, 4 sim faults, `Sim.Restart`, LIFE-02 selections, catalog, sim index.
- `d1addc1`: T7-MON-14 (`publist.go`, monitor hooks), 5 capture fixtures, `lifecycle-pass` with B's PublicationList, catalog.
- `0c195ff`: README, DESIGN, ADR-0025 amendment C-067, log, STATUS, ROADMAP, REVIEW, REVIEW_DE, sim index note.

## Checks (final)

`oec/check.sh` reports "all checks passed":
- spec gate;
- schema;
- SDK gate incl. `cargo deny`, cbindgen diff, C tests under ASan/UBSan;
- Go tests of `spec/tools`, `conformance` (gofmt, vet, `TestSimFixtures` with 56 sim fixtures through the proxy and with the restart hook, 29 monitor captures, `internal/proxy`) and `dev`.

## For the PO

- **R-1/R-2 of C-065** and **R-3 of C-066** are still open.
- **R-4:** please review the 4 draft sim fixtures of C-067 in `oec/conformance/testdata/sim/index.yaml` (G-3.9), in particular `T7-LIFE-04/abort` (inconclusive instead of a failure).
- **R-5:** please review the 5 new capture fixtures of T7-MON-14 and the changed `lifecycle-pass` in `oec/conformance/testdata/captures/index.yaml` (G-3.9), in particular the reading of "enables" (`publist-mode`) and `publist-change`.
- A-215 … A-221 for information. Worth a look: A-215 (cut), A-216 (the hook is a tester command), A-218 (what counts as the Close message), A-220 (what MON-14 judges and what not).
- No SF, no decision needed.

## Handover (G-3.10)

**A new agent session** continues with C-068 (increment 6b: peer simulation, `--announce`, T7-ADR-01).

- Reason: the context of this session has been compacted several times, and 6b brings new infrastructure (a second simulated component).
- Everything needed is in the code, DESIGN.md §3.3, §3.4 and §10, the ADR-0025 amendments C-062 … C-067 and this log.
- `prompt.md` holds the start prompt (not committed).

## Next

1. PO review of R-1/R-2 (C-065), R-3 (C-066), R-4/R-5 (C-067).
2. C-068, P5 increment 6b after the ROADMAP plan.
3. C-069, increments 7/8 and the P5 exit run.

Next free ids:
- SF-114;
- REQ-CMP-048, REQ-RES-PUBLIST-028, REQ-RES-SUBLIST-017, REQ-TOP-029, REQ-ACT-025;
- V-PAY-246, V-TOP-138 / V-TOP-255, V-BRK-019, P-129;
- ADR-0027, Q-031, A-222.
