# C-065 – P5 increment 4: the Action and the deferred monitor rules

- **Date:** 2026-10-28
- **Phase:** P5 – Conformance test tool (D7 `test`), increment 4 (T7-MON-14 cut to increment 5)
- **Baseline:** guideline `eb7889c` (branch head `4fac7d2`, findings only), SF-001 … 113 resolved; 1406 inventory items, 442 REQs, 158 `schema`/`d7-validate` REQs, 0 gaps, no REQ drafts; 29/29 entry schemas, 245 payload vectors, 18 `broker.json` vectors; schema `2.0.1-dev` @ `eb7889c`; 18 capture fixtures and 32 sim fixtures reviewed. All verified at the start (gate green) and unchanged; no reviewed item changed (see "Changes to reviewed items").
- **Agent:** new agent (start prompt from C-064, `prompt.md`)

## Intent

PO order (start prompt): P5 increment 4 after the ROADMAP plan, test-first.

- T7-ACT-01 … 04: the Action `NewDataSetWriterId` (MG 8.4.1/9.2.1 after SF-112/SF-113, REQ-ACT-*), on the state model of C-064. A provided id or a created entry must not disturb the restore check.
- The remaining passive rules T7-MON-09 and T7-MON-13 … 17.
- Every test gets a sim fault `<test>/<what>` or, for monitor rules, a capture fixture (`draft` → R-x). `planned` → `implemented` only with a positive and a negative fixture.
- Then a run against `demo.rs` on a copy of a dev root (level 2), the result including `restore` into this log.

Nothing was open for the PO at the start.

## Cut (agent, G-3.4 b)

Increment 4 is done except **T7-MON-14**, which moves to C-066 (increment 5).

- T7-MON-14 judges the publication rules of the PublicationList: Mode, Interval and the Precisions as a dead band, per entry, over the whole observation. It needs a timing model of its own: expected publication times per entry, tolerances, changes of an entry during the run. None of the other rules of this increment needs it.
- The rest shares one state per participant (DataSet structures, Profile, RevisionCounters, Pub Configs, pending changes). It fit into one cycle.
- ROADMAP: plan item 4 is marked done; T7-MON-14 is added to item 5 and to the table row C-066.

## Work

### 1. The Action tests (`internal/probe/action.go`)

D7 calls `NewDataSetWriterId` on `Oi4v2/<ST>/<AppId>/Call/NewDataSetWriterId[/<Source>]`.

- **Request.** `ua-action-request` with:
  - ResponseAddress = the matching `Reply` topic (REQ-TOP-016);
  - `RequestorId` = the tester, and `TimeoutHint` = `--timeout`;
  - CorrelationData (Base64 of the MessageId) on the first call of ACT-01;
  - one ActionRequest: DataSetWriterId 8, ActionTargetId 1, a counting RequestId, ActionState 1, Payload `{"Resource": "Data"}`.
- **Correlation.** Responses are collected from every Reply topic of the CUT and matched by `RequestorId` and `RequestId`. A response on the wrong topic is therefore a finding, not a timeout. The CorrelationData is judged, not used to match.
- **Write window.** The Action tests run after the write tests, inside the write window: a provided id is only reserved by the CUT, D7 never uses it and creates nothing with it. The restore check before/after is unaffected (demo: restored).
- **Ids in use.** D7 records the DataSetWriterIds the CUT uses: those of its publications above the reserved range, and those in its PublicationList. A provided id must not be one of them.
- **Must the CUT answer?** Yes if its Profile lists `NewDataSetWriterId` or REQ-CMP-040 is MUST at the declared level. Otherwise an unanswered call is a skip.
- **The tests:**
  - **T7-ACT-01:** two calls, one without Source and with CorrelationData, one with the AppId as Source and without CorrelationData. Per answered call it checks:
    - MessageType; the topic equals the ResponseAddress; CorrelationData echoed or absent;
    - DataSetWriterId/ActionTargetId of the request;
    - a CompactEncoding Status without Symbol;
    - ActionState 1 (any number, REQ-ACT-016 MAY) or 2 (exactly one final); a Payload object;
    - a Good final response provides an id ≥ 10 (UInt16) with a UInt32 Ttl, not in use and not provided before.
  - **T7-ACT-02:** two more calls while the earlier ids are reserved; no id is provided again within its Ttl (REQ-ACT-023). The initial use within the Ttl needs a create, which D7 does not do (manual part, catalog note).
  - **T7-ACT-03:** an unknown DataSetWriterId (4), an unknown ActionTargetId (2) and an undecodable request get no response within `--timeout` (REQ-ACT-017). A valid call beside them shows that the CUT answers at all; without its answer the test skips. The two schema-invalid requests are deliberate tester violations of REQ-ACT-019 (whitelisted in `sim_test.go`, like the other deliberate stimuli).
  - **T7-ACT-04:** a call for the unknown Source `oi4.org/oec-check/Unknown/<random>` is answered with BadNotFound (0x803E0000) and an empty Payload (REQ-ACT-024, SF-113).
- **Report.** `requests` lists the calls with the Get/Set/Del requests, ordered by the time sent; `replies` of a call counts its ActionResponses (fix, see "Defects found").

### 2. The simulated CUT (`internal/sim/action.go`)

- It answers `NewDataSetWriterId` for the application and its assets. Ids start at 6000, each reserved for a Ttl of 60 s. An unknown Source gets BadNotFound; unknown writer/target ids and undecodable requests get no answer.
- **5 new faults** (36 in total):
  - `T7-ACT-01/no-correlation` – the CorrelationData is not echoed;
  - `T7-ACT-01/progress` – **conforming**: an intermediate response with ActionState 1 before the final one;
  - `T7-ACT-02/reissue` – always the id 6000;
  - `T7-ACT-03/answers-unknown` – unknown writer/target ids answered with BadNotFound;
  - `T7-ACT-04/no-reply` – an unknown Source is not answered.
- `testdata/sim/index.yaml`: 5 `draft` entries (→ R-1). With `none`, 37 sim fixtures; every run passes the restore contract (`sim.Changed()` empty).

### 3. The monitor rules (`internal/monitor/config.go`, hooks in `monitor.go`)

The state is kept per participant and survives reconnects.

- **T7-MON-09 (REQ-MSG-013).**
  - The structure of a DataSet is its sorted field names: Metadata `Fields[].Name`, or the keys of the Data payload. It is kept per resource, DataSetWriterName and WriterGroupName.
  - Violations: a DataSetClassId seen with another structure, or the same DataSet with the same structure under another DataSetClassId.
- **T7-MON-13 (REQ-CMP-010/036/038).** Only for a component with Sources.
  - Data and Event are supported if they are published or listed in the component's own Profile.
  - A Profile without them and no publication is a violation.
  - Without Profile and without publication the rule skips "not observable". This is a plain skip, not an inconclusive one: `test` asks for the Profile itself.
- **T7-MON-15 (REQ-RES-PUBLIST-017/018, REQ-RES-SUBLIST-005/006/015).**
  - Every Set/Del of the PublicationList/SubscriptionList records a pending change. The list counts as published again on a Pub with an empty `ReplyTo`, or with the MessageId of the request (then the MAY REQs 018/006 are checked).
  - At the end, after Status Good:
    - republished → the REQ is checked;
    - a Del of the SubscriptionList → plain skip, because nothing is published without remaining entries (SUBLIST-015) and the remaining entries are not observable;
    - nothing published for longer than the tolerance (5 s) → violation;
    - else an inconclusive skip.
- **T7-MON-16 (REQ-RES-MAM-018).** Every MAM with a DataSetWriterName is checked.
  - A decrease of the RevisionCounter, or a change between -1 and a counter, is a violation.
  - A Set Config answered with Status Good that changes the Value of a known, non-sensitive object (with a known counter ≠ -1) records a change. It is resolved by a MAM of that Source with a larger counter. A later MAM (beyond the tolerance) with a counter not larger is a violation; no later MAM gives a skip.
  - The exact +1 is not judged: other changes may count too.
- **T7-MON-17 (REQ-RES-CONFIG-011).** A Set Config contains, for every group it contains, each object the addressed component published with `Mandatory` = true, with a Value.
  - The flags come from the Pub Configs of the same Source and WriterGroupName, merged per object.
  - The requester is judged. Without a Pub Config before the Set the rule skips.
- **Catalog:** T7-MON-09/13/15/16/17 `implemented`; T7-MON-14 keeps `planned` with a note "moved to increment 5 (C-066)".

### 4. Capture fixtures (`internal/monitor/fixtures_test.go`, `testdata/captures/`)

6 new scenarios, `draft` (→ R-2), built from reviewed payload vectors as in C-052:

- `configuration-pass` (positive): MON-09/13/15/16/17 pass.
  - It follows MG B1.1.3: Set Config V-PAY-100, Pub Config V-PAY-099/114, MAM RevisionCounter 1 → 2.
  - It adds a PublicationList and a SubscriptionList Set, each republished with the `ReplyTo`.
- `class-id-unstable`: MON-09 fail REQ-MSG-013.
- `no-data-event`: MON-13 fail REQ-CMP-010/036/038.
- `list-not-republished`: MON-15 fail REQ-RES-PUBLIST-017.
- `revision-not-counted`: MON-16 fail REQ-RES-MAM-018.
- `set-config-mandatory-missing`: MON-17 fail REQ-RES-CONFIG-011.

New helpers `retag` and `mamRevision`. Unit tests `TestRevisionCounter`, `TestDelSubscriptionList`, `TestSetConfigUnchanged` in `monitor_test.go`. 24 capture fixtures in total.

## Defects found (test-first, fixed)

1. **Sim, progress variant:** the intermediate response first carried no Status/Payload. REQ-ACT-012 requires the response fields in every Good response, so it now carries the same Status and Payload as the final one.
2. **Sim, `answers-unknown`:** the BadNotFound responses echo DataSetWriterId 4 and ActionTargetId 2, which the schema of a NewDataSetWriterId response rejects. The fixture expects T7-MON-01 fail REQ-ACT-019 besides T7-ACT-03. This is a consequence of the fault, not a D7 defect.
3. **Fixtures, REQ-TOP-020:** the MG B1.1.3 Config tag "Overall device settings" has white space. As a topic Filter that is a REQ-TOP-020 warning (A-4), which made T7-MON-01 warn in the new captures. `retag` uses "device" instead in the topics and WriterGroupNames of `configuration-pass`, `revision-not-counted` and `set-config-mandatory-missing`.
4. **MON-13 skip kind:** "nothing observed" was first an inconclusive skip. That would have turned the reviewed `lifecycle-pass` from PASS into INCONCLUSIVE although nothing in it is wrong. It is now a plain skip (reason: `test` requests the Profile; `monitor` sees what it sees).
5. **Report, `requests`:** the stimuli list of the report left out D7's `NewDataSetWriterId` calls (found while preparing this log). Now they are listed in the order sent, with their ActionResponses as `replies`. Test first: `TestSimActionRequests` (`9958261`).

## Runs

**Demo** (`oec/sdk/rust/target/debug/examples/demo`, `OEC_ROOT` = a copy of the `oec-dev-app` root under `/tmp`, dev broker Mosquitto from `oec/dev`, port 8883), command as in C-062/C-064:

```
oec-check test --broker mqtts://localhost:8883 --ca ../dev/generated/broker/ca.pem \
    --username oec-probe --password-file ../dev/generated/probe/password \
    --app 'OTConnector/example.com/OEC,20Demo/DEMO-1/oec-dev-app' --level 2 \
    --record test.jsonl --report json=report.json
```

**Result: 38 pass, 0 fail, 6 skip, exit 0** (two runs: the first after `1e836cc`, the second with the final code after `9958261`; the same verdicts).

- Second run: 333 messages in 52.9 s, the CUT 188 of them, 0 invalid. 145 requests, 142 answered; the 3 unanswered are the deliberately unanswerable calls of T7-ACT-03 (the first run reported 136 requests, without the calls, defect 5).
- **restore: `{checked: true, restored: true}`** – "the state of the CUT after the write tests equals the state before them".
- T7-ACT-01 … 04 pass: the demo answers `NewDataSetWriterId` with new ids, echoes the CorrelationData, ignores the unaddressable and undecodable requests and answers an unknown Source with BadNotFound.
- T7-MON-09, 13, 15, 16 pass. T7-MON-17 skips: the demo sends no Set Config, and the tester's own Set Configs are not judged in `test` (only the CUT's observations count).
- The demo's RevisionCounter is 0 throughout and no MAM followed a Set Config. This is consistent: D7 sends no Set Config that changes a Value with Status Good (only write-backs and rejected invalid values), so MON-16 judged the monotonicity only.

Skipped tests:
- T7-MON-03: connected before the run.
- T7-MON-07: REQ-ACT-011 applies to Calls the CUT sends; the demo sends none. Earlier logs said "no Action"; since C-065 D7 itself calls, but MON-07 judges only the CUT's own Calls.
- T7-MON-14: `planned` (C-066).
- T7-ADR-02: no `Oi4` traffic.
- T7-SET-03: level 3 is above the declared level 2.
- T7-SET-04: no whole object to delete (no ReferenceDesignation object; the creation of a SubscriptionList entry is answered with BadNotSupported 0x803D0000, SF-111).

The `planned` tests (CON-*, LIFE-*, ADR-01, SET-10) are listed as skip "not implemented yet" and do not count.

A replay of the first run's capture with `monitor --replay` shows T7-MON-01 fail only for D7's deliberate invalid stimuli, as expected; MON-16/17 pass there (in `monitor` every participant is judged).

**Housekeeping:** `oec/dev/generated/` untouched; the demo store stayed in the `/tmp` copy (deleted afterwards); `docker compose down`. Interop and `live.sh` not rerun: no SDK or live-connection code changed.

## Agent decisions (G-3.4 b, for information: REVIEW A-191 … A-201)

- **A-191 Cut:** T7-MON-14 moves to C-066 (increment 5); the rest of increment 4 in one cycle.
- **A-192 Action stimuli and correlation:** the request as in "Work" 1; responses are matched by `RequestorId` and `RequestId` on any Reply topic of the CUT. The CorrelationData is judged, not used to match.
- **A-193 Write window:** the Action tests run after the write tests, inside the restore window; provided ids are never used, so nothing needs to be restored.
- **A-194 Must answer:** the CUT must answer `NewDataSetWriterId` if its Profile offers it or REQ-CMP-040 is MUST at the level; otherwise no answer is a skip.
- **A-195 ACT-02 and ACT-03 limits:** the initial use of a provided id within its Ttl stays manual (D7 creates no DataSet). ACT-03 sends two schema-invalid requests on purpose (tester violation REQ-ACT-019) and needs a valid answered call to judge silence.
- **A-196 MON-09 structure:** the sorted field names per DataSet (resource, DataSetWriterName, WriterGroupName); types are not compared.
- **A-197 MON-13 not observable:** without Profile and publication a plain skip, not inconclusive; the reviewed `lifecycle-pass` keeps PASS.
- **A-198 MON-15 republication:** a Pub with an empty `ReplyTo` or the `ReplyTo` of the Set counts, within the tolerance after Status Good; after a Del SubscriptionList the absence is a plain skip (SUBLIST-015).
- **A-199 MON-16 counter:** growth, not exactly +1; only value changes of known non-sensitive Config objects count (not ReferenceDesignation, A-113 unchanged). In `test` D7 makes no value-changing Set Config, so only monotonicity is judged there.
- **A-200 MON-17 judges the requester:** flags merged from the Pub Configs per Source and WriterGroupName. In `test` only the CUT's own Sets are judged.
- **A-201 Fixtures and report:** `retag` in three captures (REQ-TOP-020); the sim progress variant carries the response fields; `answers-unknown` expects T7-MON-01 REQ-ACT-019; the report lists the calls among the `requests`.

## Changes to reviewed items

None in content or verdict.
- The reviewed sim fixtures (`none` and the 31 faults) keep their expectations: the Action tests pass on them or are not affected. They stay `reviewed`.
- The reviewed capture fixtures keep their verdicts. `lifecycle-pass` gains passes of MON-09/13/16 and stays PASS (16 pass, 2 skip; was 13 pass); `reconnect-pass` stays INCONCLUSIVE as before. Their index entries and `.jsonl` files are unchanged.
- No REQ, vector, schema or guideline change.

## Spec findings

None. Checked during the work and found consistent:
- MG 8.4.1/9.2.1 after SF-112/SF-113 (Ttl, reuse, unknown Source);
- the response fields in intermediate responses (REQ-ACT-012/016);
- RevisionCounter (MG 5, REQ-RES-MAM-018, SF-064) vs. the Set Config of MG B1.1.3;
- republication of the lists after Set/Del (MG 9.1.8/9.1.9).

Next free: SF-114.

## Dependencies

None. `go.mod` unchanged.

## Risks and open technical points

- **MON-16 in `test`:** D7 judges only monotonicity, because it makes no value-changing Set Config. Idea for later (not planned): after a changing Set D7 could request Get MAM. That would need a Set that really changes a value and its restore.
- **MON-17 in `test`** judges only the CUT as a requester; a CUT that sends no Set Config gets a skip. In `monitor` every requester is judged.
- **T7-MON-14** (C-066) needs the timing model of the PublicationList.
- The probe tests now take about 122 s in the gate (37 sim fixtures plus the three single runs).

## Commits

- `b20ab3b`: Action tests, sim Action side, catalog, sim index (5 draft fixtures).
- `1e836cc`: monitor rules T7-MON-09/13/15/16/17, capture fixtures (6 drafts), unit tests, catalog.
- `933112a`: README, DESIGN, ADR-0025 amendment C-065.
- `9958261`: report `requests` with the Action calls (test first), DESIGN note.
- `e41b43a`: log, STATUS, ROADMAP, REVIEW, REVIEW_DE.
- This commit: commit list in the log.

## Checks (final)

`oec/check.sh` reports "all checks passed" (after `9958261`):
- spec gate;
- schema;
- SDK gate incl. `cargo deny`, cbindgen diff, C tests under ASan/UBSan;
- Go tests of `spec/tools`, `conformance` (gofmt, vet, `TestSimFixtures` with 37 sim fixtures, 24 monitor captures) and `dev`.

## For the PO

- **R-1:** please review the 5 draft sim fixtures of the Action tests in `oec/conformance/testdata/sim/index.yaml` (G-3.9).
- **R-2:** please review the 6 draft capture fixtures of the monitor rules in `oec/conformance/testdata/captures/index.yaml` (G-3.9).
- A-191 … A-201 for information. Worth a look:
  - A-191: T7-MON-14 moved to C-066;
  - A-195: what ACT-02/03 cannot judge;
  - A-199/A-200: what MON-16 and MON-17 can see in a `test` run.
- No SF, no decision needed.

## Handover (G-3.10)

**The same agent role continues in a new session** for C-066 (increment 5: proxy attachment, T7-CON-*, LIFE-01 … 03, T7-MON-14).

- Reason: the context of this session has been compacted, and increment 5 brings new infrastructure (a minimal MQTT relay).
- Everything needed is in the code, DESIGN.md §3.2/§3.3/§10, ADR-0025 amendments C-062 … C-065 and this log.
- `prompt.md` holds the start prompt (not committed).

## Next

1. PO review of R-1 (sim fixtures) and R-2 (capture fixtures).
2. C-066, P5 increment 5: proxy attachment (`--proxy-listen`, Q-028), T7-CON-01 … 04, T7-LIFE-01 … 03, T7-MON-14.

Next free ids:
- SF-114;
- REQ-CMP-048, REQ-RES-PUBLIST-028, REQ-RES-SUBLIST-017, REQ-TOP-029, REQ-ACT-025;
- V-PAY-246, V-TOP-138 / V-TOP-255, V-BRK-019, P-129;
- ADR-0027, Q-031, A-202.
