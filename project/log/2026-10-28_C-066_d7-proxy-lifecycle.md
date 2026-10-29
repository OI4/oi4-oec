# C-066 – P5 increment 5: proxy attachment, connection and lifecycle tests

- **Date:** 2026-10-28
- **Phase:** P5 – Conformance test tool (D7 `test`), increment 5 (T7-MON-14 cut to increment 6)
- **Baseline:** guideline `eb7889c` (branch head `4fac7d2`, findings only), SF-001 … 113 resolved; 1406 inventory items, 442 REQs, 158 `schema`/`d7-validate` REQs, 0 gaps, no REQ drafts; 29/29 entry schemas, 245 payload vectors, 18 `broker.json` vectors; schema `2.0.1-dev` @ `eb7889c`; capture fixtures 24 (18 reviewed, 6 draft), sim fixtures 37 (32 reviewed, 5 draft). All verified at the start (gate green, cbindgen 0.29.4); the guideline did not change.
- **Agent:** new agent (start prompt from C-065, `prompt.md`)

## Intent

PO order (start prompt): P5 increment 5 after the ROADMAP plan, test-first.

- The proxy attachment `--proxy-listen` (DESIGN §3.4, Q-028 (a)): the CUT connects to D7, D7 relays to the broker and sees CONNECT, Will, TLS and authentication. An own minimal MQTT relay on the standard library. Without the proxy the connection tests stay `skip`, the run is inconclusive (exit 4).
- T7-CON-01 … 04, T7-LIFE-01 … 03 and T7-MON-14.
- Every test gets a sim fault `<test>/<what>` (or a capture fixture), `draft` → R-x. `planned` → `implemented` only with a positive and a negative fixture.
- A run against `demo.rs` through the proxy on a copy of a dev root (level 2), the result including `restore` into this log.

Open for the PO at the start: R-1/R-2 of C-065. **No answer came during the cycle; they stay open.**

## Cut (agent, G-3.4 b)

Increment 5 is done except **T7-MON-14**, which moves again, to C-067 (increment 6; A-202).

- The proxy is new infrastructure (TLS listener, certificate issuing, relay, connection phases) and changed how every sim fixture runs. It filled the cycle.
- T7-MON-14 is independent of it: a passive rule with its own timing model (expected publication times per PublicationList entry, tolerances, changes of an entry during the run).
- ROADMAP: plan item 5 is marked done without MON-14; MON-14 is added to item 6 and to the table row C-067.

## Work

### 1. The proxy (`internal/proxy`)

**Decision: an own relay on the Go standard library, no new dependency (A-203).**
- paho.mqtt.golang (ADR-0025) has only a client side.
- Its decoder is lenient, while D7 must see the CONNECT and PUBLISH packets exactly as the CUT sent them.
- A relay needs only a small part of MQTT: frame boundaries (fixed header and Remaining Length), a strict decoder for CONNECT and PUBLISH, and the CONNACK and SUBACK types downward. Everything else is forwarded byte for byte.
- An embedded broker library would have been a new, larger dependency and would hide the CUT's packets behind its own session handling.

How it works:
- **Certificates.** `--proxy-ca`/`--proxy-ca-key` name a CA the CUT trusts (the copy of its root has it as `etc/oi4/certs/ca.pem`). D7 issues three ECDSA P-256 certificates at start:
  - the relay certificate for `--proxy-name` (default `localhost`, `127.0.0.1`, `::1`);
  - a certificate of the same CA for `wrong-host.invalid`;
  - a certificate of a fresh CA the CUT does not know.

  The proxy CA is test material: generated in a temporary directory, never committed (A-204).
- **Phases per connection attempt:**
  - `relay`: TLS ≥ 1.2, then relayed to `--broker`;
  - `tls1.1`: TLS 1.1 only;
  - `untrusted-ca`;
  - `wrong-host`.

  `Reject(phases…)` queues the next attempts; `Drop()` closes the relayed connections without DISCONNECT, so the broker publishes the Will.
- **Upstream.** The proxy connects to `--broker` with D7's TLS settings and forwards the CUT's CONNECT, including its credentials. A client certificate of the CUT is seen (T7-CON-03) but cannot be relayed (A-204).
- **What it records per attempt (`proxy.Conn`):**
  - phase, TLS version, client certificate presence, handshake error;
  - the decoded CONNECT (protocol name/level, ClientId, keep alive, clean session, Will with topic/QoS/Retain/payload, user name, password presence);
  - CONNECT time and CONNACK code;
  - every PUBLISH of the client with its time;
  - the time of the first SUBACK;
  - DISCONNECT, dropped, closed.
- **Tests.** `proxy_test.go` covers the codec (malformed packets, Will flags) and every phase against `mqtttest`.

### 2. The tests (`internal/probe/lifecycle.go`)

**Order of a proxy run (A-205, A-206).**
1. D7 subscribes, serves the proxy and waits up to `--proxy-wait` (default 2 min) for a relayed connection of the CUT with a PUBLISH and the broker's first SUBACK, bounded by CONNECT + `--timeout`.
2. The other tests run.
3. The connection phase: the next three attempts get TLS 1.1, untrusted CA and wrong host; the relayed connection is dropped; D7 waits for the reconnection with a Birth (or `--timeout` after its CONNECT) and the Will on the bus (or `--timeout` after the drop).
4. Every connection of the run is judged.

**Judgments:**
- **T7-CON-01** (REQ-MQTT-001, REQ-CMP-001, REQ-CMP-009):
  - protocol level ≥ 4;
  - ClientId not empty and the same on every connection;
  - keep alive ≠ 60 s and clean session false are notes, because MG 6.2.3 allows configured values (A-207).
- **T7-CON-02** (REQ-MQTT-005, REQ-MQTT-008, REQ-CMP-002):
  - relayed connections use TLS ≥ 1.2;
  - a CONNECT in a reject phase is a violation;
  - a phase not reached because the CUT did not reconnect is an inconclusive skip.
- **T7-CON-03** (REQ-MQTT-009): a client certificate, or user name and password.
- **T7-CON-04** (REQ-MQTT-018): no Will Retain, no PUBLISH with Retain.
- **T7-LIFE-01** (REQ-LIFE-008, REQ-CMP-003, REQ-LIFE-010, REQ-LIFE-016):
  - every CONNECT has the Will on `Pub/Health/<AppId>` without Retain;
  - the Will has no Timestamp and no SequenceNumber;
  - it is created anew per connection: the time in its MessageId is within `--tolerance` of the CONNECT, and no MessageId repeats (A-208).
- **T7-LIFE-02** (REQ-LIFE-001, REQ-LIFE-015, REQ-LIFE-017; A-209):
  - a Birth on every relayed connection within `--timeout`, as the first PUBLISH;
  - after a self Health `FAILURE_1`/0, the next other Health follows a new Birth;
  - REQ-LIFE-017 is a skip if the CUT reports no such failure;
  - no reconnection after the drop is an inconclusive skip.
- **T7-LIFE-03** (REQ-LIFE-009): the Will is the CUT's Health about itself with `FAILURE_1`/0 and the DataSetWriterId of its own Health.

**Without the proxy (A-210)** every CON/LIFE requirement is an inconclusive skip "needs proxy" (exit 4 unless `--allow-skip`). A retained delivery of the CUT on D7's subscription is still a T7-CON-04 finding (`TestSimNoProxy`).

### 3. Simulated CUT and fixtures

**Sim (`internal/sim`; A-212):**
- reconnects in a loop after a drop and publishes its Birth again;
- creates its Will anew per connection, with QoS 0 (REQ-MQTT-017, QoS 0 as default; it was QoS 1);
- through the proxy, it starts independently of D7 in a goroutine, like a real CUT. The old synchronous start hid the race of defect 6.

**15 new faults:**

| Fault | Kind | Expected |
|---|---|---|
| `T7-CON-01/mqtt31` | negative | CON-01 fail REQ-CMP-001, REQ-MQTT-001 |
| `T7-CON-01/client-id` | negative | CON-01 fail REQ-CMP-009 |
| `T7-CON-02/tls11` | negative | CON-02 fail REQ-CMP-002, REQ-MQTT-005 |
| `T7-CON-02/no-verify` | negative | CON-02 fail REQ-CMP-002, REQ-MQTT-008 |
| `T7-CON-03/anonymous` | negative | CON-03 fail REQ-MQTT-009 |
| `T7-CON-04/retain` | negative | CON-04 fail REQ-MQTT-018 |
| `T7-LIFE-01/no-will` | negative | LIFE-01 fail REQ-CMP-003, REQ-LIFE-008; MON-10 info REQ-IL-012; LIFE-03 skip |
| `T7-LIFE-01/stale-will` | negative | LIFE-01 fail REQ-LIFE-016 |
| `T7-LIFE-01/timestamp` | negative | LIFE-01 warn REQ-LIFE-010 |
| `T7-LIFE-02/no-birth` | negative | LIFE-02 fail REQ-LIFE-001; MON-03 fail REQ-CMP-003, REQ-LIFE-001 |
| `T7-LIFE-02/health-first` | negative | LIFE-02 warn REQ-LIFE-015 |
| `T7-LIFE-02/no-rebirth` | negative | LIFE-02 fail REQ-LIFE-017 |
| `T7-LIFE-02/recovery` | conforming variant | pass |
| `T7-LIFE-02/late-subscribe` | conforming variant | pass (Birth first, subscriptions 300 ms later) |
| `T7-LIFE-03/writer-id` | negative | LIFE-03 fail REQ-LIFE-009; MON-08 fail REQ-MSG-021 |

- That makes 51 faults and **52 sim fixtures**: 31 reviewed, 21 draft (5 from C-065, the 15 new ones, and `none`).
- **Harness (A-213):**
  - every fixture runs through the proxy;
  - an index entry may select its tests (`tests`); the new entries select `T7-MON-*`, `T7-CON-*`, `T7-LIFE-*` to keep the gate short;
  - the other entries run every test, so CON/LIFE run against them too and pass (their faults are not connection faults);
  - an expected test without a result is a harness error.
- New single tests: `TestSimNoProxy` and `TestSimLateSubscribe`. The probe tests take about 129 s.
- No capture fixture is new: the CON/LIFE tests need the proxy, and the corrected MON-03/MON-10 behavior is covered by monitor unit tests.

### 4. CLI and report

- **Flags** of `oec-check test`: `--proxy-listen`, `--proxy-ca`, `--proxy-ca-key`, `--proxy-name`, `--proxy-wait`. The CA must be a CA certificate, and the key a signer.
- **Report** (A-214, report version 1 unchanged):
  - `attachment` is `"proxy"`;
  - `proxy_listen`;
  - `connections`: every attempt without payloads and without password;
  - the text report prints one line per attempt.
- The CLI tests cover the proxy run and the missing CA key.

### 5. Catalog

- T7-CON-01 … 04 and T7-LIFE-01 … 03 are `implemented`.
- The notes of MON-03/10 are adapted.
- T7-MON-14 is still `planned`; its note now says "cut to C-067".
- `probe.Implemented` follows the catalog order.

## Defects found and fixed (test first)

1. **T7-MON-03 false failure.** A Will followed by a late Birth on the next connection was reported as a missing Birth. Now a missing Birth is reported only when the connection or the observation ends without one; a late Birth is REQ-LIFE-015 (T7-LIFE-02). Monitor unit test first.
2. **T7-MON-10 false note.** The Birth after a recovery (REQ-LIFE-017) or a late Birth was counted as a second Birth. Now it is not. Unit test first.
3. **Harness gap.** A fixture expectation for a test without a result passed silently (e.g. after a `tests` selection). Now it is an error (A-213).
4. **`TestImplemented` order.** `probe.Implemented` is now in catalog order.
5. **Sim Will with QoS 1.** The sim used QoS 1 for the Will, against the QoS 0 default (REQ-MQTT-017) that the SDK and the guideline examples use. It is QoS 0 now, created per connection.
6. **Discovery overtook the CUT's subscriptions (found by demo run 1).**
   - Symptom: 141 of 145 requests answered, one more unanswered than in C-065: the discovery `Get MAM`.
   - Cause: D7 sent the Get in the same millisecond as the demo's Birth. The SDK subscribes after its Birth, which is conforming (no rule orders Birth and SUBSCRIBE). `awaitProxy` waited only for the first PUBLISH.
   - Fix: the proxy records the first SUBACK, and D7 waits for it.
   - Test first: the sim starts asynchronously, and the variant `T7-LIFE-02/late-subscribe` plus `TestSimLateSubscribe` failed before the fix and pass after it; the race detector is clean.
   - A D7 defect, **not an SDK defect**. Commit `ff5b79b`.

## Runs

**Demo** (`oec/sdk/rust/target/debug/examples/demo`, dev broker Mosquitto from `oec/dev`, port 8883):
- `OEC_ROOT` = a copy of the `oec-dev-app` root under `/tmp/c066-demo`;
- in the copy, `etc/oi4/certs/ca.pem` is the proxy CA (openssl EC P-256, generated there), and `etc/oi4/mqtt/broker.json` points to `localhost`, SecurePort 18883 (MaxPacketSize 262144).

The command is the C-065 base plus the proxy:

```
oec-check test --broker mqtts://localhost:8883 --ca ../dev/generated/broker/ca.pem \
    --username oec-probe --password-file ../dev/generated/probe/password \
    --app 'OTConnector/example.com/OEC,20Demo/DEMO-1/oec-dev-app' --level 2 \
    --proxy-listen 127.0.0.1:18883 --proxy-ca /tmp/c066-demo/ca.pem --proxy-ca-key /tmp/c066-demo/ca8.key \
    --record test.jsonl --report json=report.json
```

Then the demo is started on the copy.

**Run 1** (after `bcab7c0`): 46 pass, 0 fail, 5 skip, exit 0, restore OK. 141 of 145 requests answered → defect 6.

**Run 2** (after `ff5b79b`, final code): **46 pass, 0 fail, 5 skip, result PASS, exit 0.**
- 355 messages in 67.6 s, 210 of them from the CUT, 0 invalid; 2 Births, 1 Will.
- 145 requests, 142 answered. The 3 unanswered are the deliberate T7-ACT-03 stimuli, as in C-065.
- **restore: `{checked: true, restored: true}`** – "the state of the CUT after the write tests equals the state before them".
- Proxy, 5 connection attempts:
  - #0 relay: TLS 1.3, MQTT/4, ClientId `oec-dev-app`, keep alive 60, clean session, Will QoS 0 without Retain on `Pub/Health/<AppId>`, user name and password; CONNACK 0; 198 PUBLISH; SUBACK about 65 ms after the CONNECT; dropped.
  - #1 tls1.1: refused ("client offered only unsupported versions").
  - #2 untrusted-ca: refused ("unknown certificate authority").
  - #3 wrong-host: refused ("bad certificate").
  - #4 relay: reconnect about 6 s after #3 (SDK backoff); TLS 1.3, CONNACK 0, 11 PUBLISH, Birth first.
- T7-CON-01 … 04 pass. T7-LIFE-01 and LIFE-03 pass. T7-LIFE-02 passes REQ-LIFE-001/015; REQ-LIFE-017 is skipped, because the demo never reports `FAILURE_1`/0.
- T7-MON-03 now passes; in C-065 it was skipped ("connected before the run"). Compared with C-065 (38 pass, 6 skip) the run has 7 CON/LIFE passes plus MON-03.

Skipped tests:
- T7-MON-07: the demo sends no Calls.
- T7-MON-17: no Set Config from the CUT.
- T7-ADR-02: no `Oi4` traffic.
- T7-SET-03: level 3 is above the declared level 2.
- T7-SET-04: no whole object to delete (as in C-065).

The `planned` tests (MON-14, LIFE-04, ADR-01, SET-10) are listed as "not implemented yet" and do not count.

**The SDK is unchanged:** `demo.rs` passes every implemented test.

**Housekeeping:**
- The demo was stopped and the dev broker shut down with `docker compose down`.
- `oec/dev/generated/` is untouched.
- The proxy CA and the root copy stayed under `/tmp` and were deleted afterwards.

## Agent decisions (G-3.4 b, for information: REVIEW A-202 … A-214)

- **A-202 Cut:** T7-MON-14 moves again, to C-067 (increment 6). The proxy filled the cycle; MON-14 is independent of it.
- **A-203 Own relay:** `internal/proxy` on the standard library, no new dependency. paho has no server side and decodes leniently. The relay decodes CONNECT and PUBLISH strictly and forwards every frame unchanged.
- **A-204 Certificates:**
  - the tester gives a CA the CUT trusts (`--proxy-ca`/`--proxy-ca-key`), and D7 issues the relay and wrong-host certificates from it, plus an untrusted CA of its own;
  - upstream the proxy uses D7's TLS settings and the CUT's CONNECT;
  - a client certificate of the CUT is seen but not relayed.
- **A-205 Wait before discovery:** D7 waits for the CUT's Birth and its first SUBACK (bounded by CONNECT + `--timeout`) before the first Get. Found by demo run 1 (defect 6).
- **A-206 Connection phase last:** TLS 1.1 → untrusted CA → wrong host → drop → reconnection. It runs after all other tests, so the write/restore window is not disturbed. A phase the CUT never reaches is an inconclusive skip.
- **A-207 CON-01 notes:** keep alive ≠ 60 s and clean session false are notes, not violations: MG 6.2.3 names defaults, and REQ-CMP-009 grades them. A ClientId that changes between connections is a violation.
- **A-208 "Created anew" (REQ-LIFE-016):**
  - the time in the Will's MessageId must be within `--tolerance` of the CONNECT;
  - no MessageId may repeat;
  - a Will without such a MessageId is a violation.
- **A-209 LIFE-02:**
  - a Birth is the Pub MAM about itself without `ReplyTo`, and it must be the first PUBLISH of each connection;
  - recovery means: after a self Health `FAILURE_1`/0 the next other self Health must follow a new Birth;
  - REQ-LIFE-017 is a plain skip without such a failure.
- **A-210 Without proxy:** every CON/LIFE requirement is an inconclusive skip (exit 4); the retained finding of CON-04 stays visible on the bus.
- **A-211 Monitor:**
  - MON-03 reports a missing Birth after a Will only when the connection or observation ends without one;
  - MON-10 does not count a recovery or late Birth as a second Birth.
- **A-212 Sim:**
  - through the proxy, it starts independently of D7;
  - it reconnects in a loop;
  - its Will has QoS 0 and is created per connection;
  - the variant `late-subscribe` subscribes 300 ms after the Birth.
- **A-213 Harness:**
  - every sim fixture runs through the proxy;
  - an entry may select its tests (`tests`) to keep the gate short; the reviewed entries select nothing, run every test and keep their expectations (CON/LIFE pass on them);
  - an expected test without a result is an error;
  - `none` judges the CON/LIFE tests and is therefore back to `draft`.
- **A-214 Report:**
  - `attachment` `"proxy"`, `proxy_listen` and `connections`, with no payloads and no password;
  - the text report gets one line per attempt;
  - the report version stays 1 (additive).

## Changes to reviewed items

- **Sim fixture `none`** (reviewed in C-062 … C-065) now also expects CON-01 … 04 and LIFE-01 … 03 to pass, with REQ-LIFE-017 skipped. It is back to **`draft`** (R-3).
- **The other 31 reviewed sim fixtures** are unchanged in the index. They now run through the proxy and CON/LIFE run against them as well; these tests pass on them (none of their faults is a connection fault), so their expected verdicts still hold ("every implemented test not listed passes or is skipped"). They stay `reviewed`.
- **The reviewed capture fixtures** keep their verdicts after the MON-03/MON-10 corrections, e.g. `will-then-publish` still fails REQ-CMP-003/REQ-LIFE-001 (the observation ends without a Birth). Their index entries and `.jsonl` files are unchanged.
- **Catalog:** the MON-03/MON-10 notes are adapted to the corrected behavior; the cited REQs are unchanged.
- No REQ, vector, schema or guideline change.

## Spec findings

None. The cited REQs of T7-CON-01 … 04 and T7-LIFE-01 … 03 were read against MG 6.2.3, 6.3.1, 6.3.5, RIG 4.1 and found consistent:
- keep alive and clean session are defaults, graded by REQ-CMP-009;
- the Will is created at connect (REQ-LIFE-016);
- Birth first (REQ-LIFE-015), and a Birth after recovery (REQ-LIFE-017);
- the order of Birth and SUBSCRIBE is not specified. D7 adapted to that (A-205); the guideline needs no change.

Next free: SF-114.

## Dependencies

None. `go.mod` is unchanged. The relay uses only `crypto/tls`, `crypto/x509`, `crypto/ecdsa` and `net` (A-203).

## Risks and open technical points

- **Client certificates** are not relayed: a CUT that authenticates only with a client certificate cannot reach the broker through the proxy. Such a CUT needs a broker that accepts D7's credentials for it; today this is a limitation documented in the README.
- **`--proxy-wait`** (2 min) must cover the CUT's reconnect backoff. The demo took about 6 s after the last reject; a CUT with a long backoff ends inconclusive.
- **The connection phase** needs the CUT to try three times and refuse each attempt. A CUT that gives up after one refused handshake leaves the later phases unjudged (inconclusive skip).
- **T7-MON-14** (C-067) still needs the timing model of the PublicationList.
- **Gate time:** the probe tests take about 129 s.

## Commits

- `bcab7c0`: proxy, CON/LIFE tests, sim faults, CLI flags, report, monitor fixes, catalog, sim index.
- `ff5b79b`: SUBACK wait before discovery (defect 6), `late-subscribe` variant and test.
- `6ea05ce`: README, DESIGN, ADR-0025 amendment C-066, log, STATUS, ROADMAP, REVIEW, REVIEW_DE.

## Checks (final)

`oec/check.sh` reports "all checks passed":
- spec gate;
- schema;
- SDK gate incl. `cargo deny`, cbindgen diff, C tests under ASan/UBSan;
- Go tests of `spec/tools`, `conformance` (gofmt, vet, `TestSimFixtures` with 52 sim fixtures through the proxy, 24 monitor captures, `internal/proxy`) and `dev`.

## For the PO

- **R-1/R-2 of C-065** (5 sim fixtures, 6 capture fixtures) are still open.
- **R-3:** please review the 16 draft sim fixtures of C-066 in `oec/conformance/testdata/sim/index.yaml` (G-3.9): the 15 new ones (CON/LIFE) and `none`, which is back to draft because it now judges CON/LIFE as well.
- A-202 … A-214 for information. Worth a look:
  - A-202: MON-14 moved again;
  - A-204: client certificates are not relayed;
  - A-205: D7 waits for the subscriptions;
  - A-207: keep alive and clean session are notes;
  - A-208: what "created anew" means for the Will.
- No SF, no decision needed.

## Handover (G-3.10)

**The same agent role continues in a new session** for C-067 (increment 6: restart hook, peer/`--announce`, T7-LIFE-04, T7-SET-10, T7-ADR-01, plus T7-MON-14).

- Reason: the context of this session has been compacted several times, and increment 6 brings new attachments.
- Everything needed is in the code, DESIGN.md §3.3, §3.4 and §10, the ADR-0025 amendments C-062 … C-066 and this log.
- `prompt.md` holds the start prompt (not committed).

## Next

1. PO review of R-1/R-2 (C-065) and R-3 (C-066).
2. C-067, P5 increment 6 after the ROADMAP plan, plus T7-MON-14.

Next free ids:
- SF-114;
- REQ-CMP-048, REQ-RES-PUBLIST-028, REQ-RES-SUBLIST-017, REQ-TOP-029, REQ-ACT-025;
- V-PAY-246, V-TOP-138 / V-TOP-255, V-BRK-019, P-129;
- ADR-0027, Q-031, A-215.

## Addendum 1 – PO review: R-3 (2026-10-29)

The PO answered "R-1, R-2, R-3, R-4 und R-5 – 6 akzeptiert" after C-067 (G-3.9).

- R-3: the 16 sim fixtures of T7-CON-01 … 04 and T7-LIFE-01 … 03 (incl. `none`) in `oec/conformance/testdata/sim/index.yaml` → `reviewed`; the entries changed again in C-067 (`none`, `T7-LIFE-02/no-birth`, `/health-first`) are reviewed in their C-067 form.
- Recorded together with R-1/R-2 of C-065 and R-4/R-5 of C-067; the list of changed files is in addendum 1 of `2026-10-29_C-067_d7-restart-publist.md`. No code, spec, schema or catalog change.
