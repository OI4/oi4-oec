# C-068 – P5 increment 6b: peer simulation, `--announce` and T7-ADR-01

- **Date:** 2026-10-29
- **Phase:** P5 – Conformance test tool (D7 `test`), increment 6b (increment 6 split in C-067, A-215)
- **Baseline:** guideline `eb7889c` (branch head `4fac7d2`, findings only), SF-001 … 113 resolved, no open finding; 1406 inventory items, 442 REQs, 158 `schema`/`d7-validate` REQs, 0 gaps, no REQ drafts; 29/29 entry schemas, 245 payload vectors, 18 `broker.json` vectors; schema `2.0.1-dev` @ `eb7889c`; at the start capture fixtures 29 (29 reviewed), sim fixtures 56 (56 reviewed). The guideline did not change.
- **Agent:** new agent session (start prompt of the C-067 handover, G-3.10)

## Intent

PO order (start prompt): C-068 = P5 increment 6b after the ROADMAP plan, test-first:
- `--announce` (Q-029 (a)): D7 publishes MAM, Health and Birth under its tester IL like a component; without the flag D7 stays unannounced, as before.
- Peer simulation (`--peer`, DESIGN §3.4): D7 announces a simulated second component and answers its requests as far as needed.
- T7-ADR-01 (requests of the application to the peer address the destination; REQ-BUS-002, REQ-TOP-006, REQ-TOP-010); applies only if the application sends requests to the peer, else a skip – check whether the demo does.
- Every test with sim faults (`draft` → R-x, G-3.9); `planned` → `implemented` only with a positive and a negative fixture; then a run against `demo.rs` through the proxy with the restart hook.

Open for the PO at the start: nothing. No answer or new order came during the cycle.

## Cut (agent, G-3.4 b)

**C-068 delivers increment 6b completely** (A-222): `--announce`, `--peer` and T7-ADR-01. ROADMAP: plan item 6 ✅, table row C-068 ✅. C-069 stays as planned (increments 7/8: `list`/`explain`, report schema, P5 exit run).

## Work

### 1. Announced components (`internal/probe/peer.go`, `session.go`)

One type `component` serves both roles (A-223, A-224):

| | tester (`--announce`) | peer (`--peer`) |
|---|---|---|
| Identification Link | the tester IL (`--tester-il`, default `oi4.org/oec-check/D7/<12 hex>`) | `oi4.org/oec-check/Peer/<12 hex>`, new per run |
| ServiceType | `Utility` | `Utility` |
| MQTT connection | the main connection of D7 (its Will is set before the CONNECT) | an own connection (ClientId `oec-check-peer-…`, own Will) |

Lifecycle of each component (MG 6.3, as the SDK does it):
- Will: Health `FAILURE_1`/0 on its Health topic, set in the CONNECT.
- Birth after D7's subscriptions and before the CUT is waited for: Pub MAM, then Pub Health `NORMAL_0`/100.
- **"Present" Health** (A-225): once the CUT is discovered, every component publishes its Health once more. A CUT that started after the Births (always the case through the proxy: the CUT connects only once D7 listens) sees the components by this Health; the SDK directory and the simulated CUT then ask with Get MAM.
- Heartbeat: Health every 30 s (MG 7.5.1: at least every 60 s).
- At the end (run context still valid): Close message (Health `NORMAL_0`/0), then DISCONNECT of the peer connection; D7 waits the grace time.
- MAM: Level 1 (a Utility that only identifies itself), DeviceClass `Oi4.Utility`; ManufacturerUri, Model, ProductCode and SerialNumber are the elements of the IL (Model `oec-check`), ProductInstanceUri the IL. Writer ids: MAM 10, Health 11, Profile 12, Status 13.

Answers (MG 8.3, as far as a requester needs them):
- Get MAM, Health, Profile without Source or with the component's own IL as Source: the resource, with `ReplyTo` correlation (Profile lists MAM, Health, Profile).
- Unknown Source: no Pub, Event Status Bad (`BadNotFound`), as REQ-CMP-047 (SF-099, SF-108) recommends.
- A Filter: an empty list (SF-099).
- Other resources and methods: ignored (a Level-1 component need not support them).

The report has a new key `announced` (role, `publisher_id`, requests received, answered) and a text line per component (A-228). `--announce` needs a `--tester-il` that is an Oi4Identifier (4 elements): the MAM takes its elements from it.

Without `--peer` and `--announce` nothing changes: D7 is not announced (Q-029 (a)).

### 2. T7-ADR-01 (`internal/probe/adr.go`)

**Attribution (A-226).** A request of the CUT to the peer is a message on the bus with
- the method Get, Set, Del or Call,
- the CUT as publisher (`PublisherId` = `<ServiceType>/<AppId>` of the CUT),
- the peer's AppId in the topic **or** the peer as Source (topic level 5, decoded).

The second criterion catches requests on a wrong AppId with the peer as Source; within a run only the peer represents its asset. A request whose AppId and Source are both wrong (or wrongly encoded) cannot be attributed – a limit of a black-box test. Only the peer counts; requests of the CUT to the announced tester are not judged (the tester is not part of the catalog entry).

**Judgment.** Every attributed request is checked against the request topic of the peer `Oi4v2/Utility/<peer>/<Method>/<Resource>/…`:
- REQ-TOP-006: ServiceType = `Utility`;
- REQ-TOP-010: AppId = the peer's;
- REQ-BUS-002: both, i.e. on the request topic of the addressed component (the detail names the expected topic).

One violation per distinct topic, one check per REQ. In its slot the test waits up to `--timeout` for a first request; it judges at the end of the run, so every request of the whole run counts (also after the restart).

**Skips:**
- no `--peer`: inconclusive skip ("needs a peer (--peer)"), exit 4 without `--allow-skip` – like the proxy (A-210) and the restart hook (A-216);
- peer announced, but no request to it: plain skip (not applicable: the three REQs apply to a requester).

So **a plain `oec-check test` now ends with exit 4** unless `--peer` (or `--allow-skip`) is given; `TestTestLive` and the demo run use `--peer --announce`.

### 3. Simulated CUT and sim fixtures

The simulated CUT (`internal/sim`) is now a consumer (A-227):
- it subscribes to `Oi4v2/+/+/Pub/MAM/+` and `…/Pub/Health/+` (after its request subscriptions, see "Defects");
- a MAM about an application marks it known; a Health about an unknown application that is neither a Will (`FAILURE_1`/0) nor a Close message (`NORMAL_0`/0) triggers Get MAM on `Oi4v2/<ST>/<AppId>/Get/MAM/<AppId>` (MessageId, PublisherId, the MAM DataSetClassId, `ReplyTo` correlation) – the behaviour of the SDK directory (`oec-core/src/consumer.rs`, `discover`).

New faults (58 in total; all `draft`, R-7):
- `T7-ADR-01/own-app` – Get MAM on the CUT's own Get topic with the peer as Source (`Oi4v2/OTConnector/<CUT>/Get/MAM/<peer>`): fail REQ-BUS-002, REQ-TOP-006, REQ-TOP-010;
- `T7-ADR-01/service-type` – the peer's AppId under the CUT's ServiceType `OTConnector`: fail REQ-BUS-002, REQ-TOP-006 (REQ-TOP-010 passes);
- `T7-ADR-01/silent` (positive variant) – the CUT watches nothing and requests nothing: plain skip of all three REQs.

`none` runs with `peer: true` and `announce: true`: the CUT asks the peer and D7 for their MAM on the right topics, T7-ADR-01 passes; back to `draft` (R-6, see "Changes to reviewed items").

Harness (`internal/probe/sim_test.go`):
- index fields `peer` and `announce` per entry (default false);
- every fixture: the number of announced components equals peer + announce; no violation or warning of the peer, no warning of the tester (the tester's deliberate stimuli stay as before);
- `none`: every announced component received requests and answered them all;
- a T7-ADR-01 skip is inconclusive exactly when the fixture has no peer.

59 sim fixtures (55 reviewed, 4 draft).

### 4. CLI, catalog, docs

- `cmd/oec-check`: `--peer` and `--announce` (test only; `monitor --peer` is a usage error), usage text, `announced` in the report; `test_test.go`: the live test runs with `--peer --announce` and expects `pass  T7-ADR-01` and the report entries; usage errors for `monitor --peer` and `--announce` with a tester IL of 3 elements.
- `catalog/d7.yaml`: T7-ADR-01 `implemented`; **all 54 D7 tests are implemented**.
- README, DESIGN.md "As built (C-068)" in §3.3, §3.4, §3.5 and §10, ADR-0025 amendment C-068.

## Defects found and fixed (test first)

1. **`TestSimLateSubscribe` failed** after the sim became a consumer: the discovery Get was not answered. D7 takes the CUT's first SUBACK through the proxy as "ready for requests" (`lifecycle.go`, A-205); the new consumer subscription came before the delayed request subscriptions of `T7-LIFE-02/late-subscribe`, so D7 asked too early. Fix in the sim: the consumer subscriptions follow the request subscriptions (a defect of the test double, not of D7; a real CUT that subscribes to presence first and to its requests later can hit the same race – the 10 s timeout and the one retry of discovery cover a short delay).
2. **The first demo run ended with exit 4** (48 pass, 7 skip): `stop.sh` was not executable (`chmod` ran in parallel with the creation of the file), so the restart hook failed with exit 126 and T7-LIFE-04/T7-SET-10 were inconclusive. A mistake of the run setup, not of D7; D7 reported it correctly. Rerun below.

The SDK needed no change.

## Runs

**Demo** (`oec/sdk/rust/target/debug/examples/demo`, dev broker Mosquitto from `oec/dev`, port 8883), as in C-067 plus `--peer --announce`:
- `OEC_ROOT` = a copy of the `oec-dev-app` root under `/tmp/c068-demo`; in the copy `etc/oi4/certs/ca.pem` is the proxy CA (openssl EC P-256, key as `pkcs8 -nocrypt`, generated there) and `etc/oi4/mqtt/broker.json` points to `localhost`, SecurePort 18883.
- `start.sh`/`stop.sh` as in C-067 (written with the create tool; `chmod +x` afterwards, not in parallel).

```
oec-check test --broker mqtts://localhost:8883 --ca ../dev/generated/broker/ca.pem \
    --username oec-probe --password-file ../dev/generated/probe/password \
    --app 'OTConnector/example.com/OEC,20Demo/DEMO-1/oec-dev-app' --level 2 \
    --proxy-listen 127.0.0.1:18883 --proxy-ca /tmp/c068-demo/ca.pem --proxy-ca-key /tmp/c068-demo/ca8.key \
    --stop-cmd /tmp/c068-demo/stop.sh --start-cmd /tmp/c068-demo/start.sh \
    --peer --announce --record test.jsonl --report json=report.json
```

Then `start.sh` once.

**Run** (after `d5a6e70`, final code): **49 pass, 0 fail, 6 skip, result PASS, exit 0.**
- 387 messages in 68.7 s, 227 of them from the CUT, 0 invalid; 3 Births, 1 Will, 1 Close message.
- 146 requests of D7, 143 answered. The 3 unanswered are the deliberate T7-ACT-03 stimuli, as in C-065 … C-067.
- **announced:** tester `Utility/oi4.org/oec-check/D7/4ef8001e9a8b` 1 request received, 1 answered; peer `Utility/oi4.org/oec-check/Peer/54f80781f4fe` 1 request received, 1 answered.
- **restore: `{checked: true, restored: true}`**.
- Proxy, 6 connection attempts: relay (197 PUBLISH, ended by the restart with Close message and DISCONNECT), relay (the restarted demo, 18 PUBLISH), `tls1.1`, `untrusted-ca`, `wrong-host` refused, relay (reconnect, 11 PUBLISH).
- **T7-ADR-01 passes.** The demo started after D7's Births, saw the peer and D7 first by their "present" Health (demo log `online  oi4.org/oec-check/Peer/… None`) and 2.4 s after the start sent `Get MAM` to both: `Oi4v2/Utility/<peer>/Get/MAM/<peer>` (PublisherId `OTConnector/example.com/OEC,20Demo/DEMO-1/oec-dev-app`) and the same for D7. Both were answered (demo log `online … Some("oec-check")`). REQ-BUS-002, REQ-TOP-006 and REQ-TOP-010 checked on the request to the peer. The restarted demo did not ask again during the run: it came back after the peer's last heartbeat before the end (heartbeat 30 s).

Skipped tests:
- T7-MON-07: the demo sends no Calls.
- T7-MON-17: no Set Config from the CUT.
- T7-ADR-02: no `Oi4` traffic.
- T7-SET-03: level 3 is above the declared level 2.
- T7-SET-04: no whole object to delete (as in C-065 … C-067).
- T7-SET-10: the demo has no ReferenceDesignation object (as in C-067).

Compared with C-067 (48 pass, 6 skip): + T7-ADR-01 (pass). Every D7 test is now implemented; the 6 skips are not applicable to the demo or above its level.

**The SDK is unchanged:** `demo.rs` passes every implemented test.

**Housekeeping:** the demo was stopped, the dev broker shut down with `docker compose down`; `oec/dev/generated/` is untouched; the root copy, the proxy CA, the captures and reports under `/tmp/c068-demo` were deleted.

## Agent decisions (G-3.4 b, for information: REVIEW A-222 … A-228)

- **A-222 Cut:** increment 6b complete in C-068; C-069 unchanged.
- **A-223 `--announce`:** the tester is a Level-1 `Utility` on D7's main connection (Will set before the CONNECT); Birth, "present" Health, 30 s heartbeat, Close message; answers Get MAM/Health/Profile; `--tester-il` must be an Oi4Identifier.
- **A-224 Peer:** `Utility/oi4.org/oec-check/Peer/<random>` on an own connection with the same lifecycle and answers; unknown Source → Status Bad `BadNotFound`, Filter → empty list, other resources ignored; a lost peer connection ends the run with an error.
- **A-225 "Present" Health:** after the discovery of the CUT every announced component publishes its Health once, so a CUT started after their Births finds them.
- **A-226 T7-ADR-01:** attribution (CUT as publisher; the peer's AppId or the peer as Source), the three REQs per request, skips (inconclusive without `--peer`, plain without requests); only the peer counts.
- **A-227 Sim and harness:** the sim is a consumer like the SDK directory, subscribing after its request subscriptions; 3 faults; index fields `peer`/`announce`; harness checks of the announced components; `none` with peer and announce, back to `draft`.
- **A-228 Report:** key `announced` and a text line per component; report version stays 1 (only added).

## Changes to reviewed items

- **Sim fixture `none`** (reviewed in the C-067 review): runs with `peer: true` and `announce: true`; its expected verdicts gain T7-ADR-01 pass (all other verdicts unchanged); note extended. Back to **`draft`** (R-6).
- **The other 55 reviewed sim fixtures** are unchanged in the index. The simulated CUT now also subscribes to MAM/Health and sends Get MAM to applications it sees by a Health; without `--peer` and `--announce` there is no other application in these runs (D7 is unannounced), so nothing changes for them; T7-ADR-01 is an inconclusive skip that the harness accepts for fixtures without a peer. They stay `reviewed`.
- **Capture fixtures:** unchanged (T7-ADR-01 is a `test` test).
- **Catalog:** T7-ADR-01 `planned` → `implemented`; the cited REQs are unchanged.
- No REQ, vector, schema or guideline change.

## Spec findings

None. The cited REQs were read against the guideline and found consistent:
- REQ-BUS-002 (MG 9.1): "Service B must publish on the Get topic of one of Service A's resources" (and on its Del topic);
- REQ-TOP-006 (MG 7.2): "For the methods Get, Set, and Del, the ServiceType of the destination that has to react to these methods must be chosen";
- REQ-TOP-010 (MG 7.3): the same for the AppId.

All three are MUST at L1–L3 for a requester; the catalog entry (`needs: [peer]`, "applies when the CUT sends requests to it") matches. Q-029 (a) is implemented as answered. Next free: SF-114.

## Dependencies

None. `go.mod` is unchanged.

## Risks and open technical points

- **Plain runs are inconclusive now** without `--peer` (or `--allow-skip`): intended (A-226), the README and the usage text say so.
- **T7-ADR-01 only sees what the CUT does by itself.** A CUT that requests other components only on a user action, or only components of a certain ServiceType, gives a plain skip. D7 cannot provoke requests of a CUT.
- **Attribution limit:** a request with a wrong AppId and without the peer as Source is not attributed (it may still be a topic or IL finding of other tests).
- **The tester is not judged as a destination;** the CUT's requests to D7 are counted in the report only.
- **Gate time:** 3 more sim fixtures (only T7-ADR-01, short); the probe package takes about 2 min 20 s.

## Commits

- `10660b7`: probe (peer, announce, T7-ADR-01), sim consumer and 3 faults, harness, sim index, catalog; also the report field `announced` (`internal/report/monitor.go`).
- `d5a6e70`: CLI flags `--peer`/`--announce`, report wiring, usage text, CLI tests.
- docs commit: README, DESIGN, ADR-0025 amendment C-068, log, STATUS, ROADMAP, REVIEW, REVIEW_DE.

## Checks (final)

`oec/check.sh` reports "all checks passed":
- spec gate;
- schema;
- SDK gate incl. `cargo deny`, cbindgen diff, C tests under ASan/UBSan;
- Go tests of `spec/tools`, `conformance` (gofmt, vet, `TestSimFixtures` with 59 sim fixtures through the proxy and with the restart hook, 29 monitor captures, `internal/proxy`) and `dev`.

## For the PO

- **R-6:** please review the changed sim fixture `none` in `oec/conformance/testdata/sim/index.yaml` (G-3.9): it now runs with the peer and `--announce` and expects T7-ADR-01 pass.
- **R-7:** please review the 3 new sim fixtures of T7-ADR-01 (`own-app`, `service-type`, the conforming variant `silent`) in the same file.
- A-222 … A-228 for information. Worth a look: A-226 (when T7-ADR-01 judges and that plain runs without `--peer` are inconclusive), A-225 ("present" Health).
- No SF, no decision needed.

## Handover (G-3.10)

**A new agent session** continues with C-069 (increments 7/8: `list`/`explain`, report schema, P5 exit run).

- Reason: the context of this session was compacted, and C-069 is a different kind of work (CLI commands, a JSON schema of the report, the exit run against the P5 criteria).
- Everything needed is in the code, DESIGN.md §3 and §10, the ADR-0025 amendments C-062 … C-068 and this log.
- `prompt.md` holds the start prompt (not committed).

## Next

1. PO review of R-6/R-7.
2. C-069, increments 7/8 and the P5 exit run.

Next free ids:
- SF-114;
- REQ-CMP-048, REQ-RES-PUBLIST-028, REQ-RES-SUBLIST-017, REQ-TOP-029, REQ-ACT-025;
- V-PAY-246, V-TOP-138 / V-TOP-255, V-BRK-019, P-129;
- ADR-0027, Q-031, A-229; review items from R-8.
