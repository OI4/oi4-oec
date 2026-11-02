# C-093 – P4b: Go host runner, D7 Level 1 in the gate

- Date: 2026-11-09
- Phase: P4b – Micro SDK (D9), increment 2 of 6 (ADR-0030 §10)
- PO input: none new (C-091 addendum 1: P4b in one session; ask only when something really does not work, is unclear or needs a guideline change).
- Baseline: guideline `52d8470` (unchanged since C-078), SF-001 … 117 resolved, next SF-118.

## Goal

Run the C17 core of C-092 as a real MQTT client on the host and judge it with D7 `oec-check test --level 1` in the gate. That checks the protocol logic of the core before the ESP-IDF port (C-096) adds only a PAL.

## What was done

1. **MQTT 3.1.1 client** `oec/sdk/micro/host/mqtt` (Go standard library):
   - packets: CONNECT with Will and user/password, CONNACK, PUBLISH QoS 0 out, PUBLISH QoS 0/1 in with PUBACK, SUBSCRIBE/SUBACK, PINGREQ/PINGRESP, DISCONNECT;
   - incoming packets bounded by a maximum size; writes serialized with a 10 s deadline; a write error closes the connection;
   - 7 tests over `net.Pipe`: exact CONNECT bytes, remaining-length encoding (1 … 4 octets; five length octets and a packet above the limit rejected), PUBLISH parsing, a refused CONNACK, a session (subscribe, publish both ways, QoS 1 PUBACK, disconnect), `Run` errors (SUBACK refusal, SUBACK of an unknown packet id, oversized packet, unexpected packet type, malformed PUBLISH), keepalive (ping sent, an unanswered ping closes). Green with `-race -count=3`.
2. **Host runner** `oec/sdk/micro/host/cmd/oecm-host` (cgo):
   - the three core sources, each as its own translation unit (`core_*.c`);
   - the PAL in Go (`pal.go`, trampolines in `pal.c`): connect = TLS dial (20 s) + MQTT CONNECT in a goroutine, `MaxPacketSize` + 65544 as the packet limit (SF-105); publish, subscribe, disconnect, Unix and monotonic time;
   - one OS-locked loop for all core calls: signals (SIGTERM/SIGINT → `oecm_stop`, exit 0), network events, the poll timer (`oecm_poll_interval_ms`);
   - events carry a connection generation; stale events are dropped, a stale CONNACK gets DISCONNECT;
   - container storage below `OEC_ROOT` (RIG 4.1): hostname, `broker.json`, CA, client certificate or credentials;
   - flags for the MAM elements, level, IL and ClientId.
3. **D7 harness** `oec/sdk/micro/host/internal/d7run`: a copy of the D7 part of `oec/demos/internal/demotest` (test broker `go-testbroker`, storage with the proxy CA, start/stop scripts, `oec-check test` with proxy, peer, announce and restart hooks, pinned passes, exact pass count).
4. **Tests** `cmd/oecm-host/main_test.go`:
   - `TestMain` makes the test binary the device (`OECM_HOST_MAIN=1`);
   - `TestStorage`: missing `broker.json`, SecurePort 0, no CA, no authentication, credentials without a colon, valid credentials (password with a colon), client certificate wins over credentials;
   - `TestD7Level1`: D7 `test --level 1`, 21 pinned passes.
5. **`oec/sdk/micro/check.sh`:** builds `oec-check` and `go-testbroker` first, then `go test -race ./mqtt` and `go test ./...` with `OECM_REQUIRE=1`, `OEC_D7`, `OEC_TESTBROKER`; header of `oec/check.sh` step 9 updated.
6. `oec/sdk/micro/README.md` (host runner section, check), ADR-0030 amendment C-093.

## Results

| Check | Result |
|---|---|
| `go test -race ./mqtt` | 7 of 7 |
| `TestStorage` | pass |
| **D7 `oec-check test --level 1`** (go-testbroker, proxy, peer, restart hooks) | **21 pass, 5 warn, 29 skip, 0 fail** (exit 0) |
| `oec/sdk/micro/check.sh` | green, about 2 min 20 s (was 6 s) |
| opt-in: runner against `oec/dev` Mosquitto (credentials, 8883), D7 `monitor --level 1` 85 s | 12 pass, 0 fail, 7 skip; 2 Births, 1 Close, 1 Will |
| `oec/check.sh` | all checks passed |

D7 Level 1 in detail: the same verdicts as the config service (C-082) except T7-ADR-01, which is skipped because the device sends no requests (no `--peer` discovery at Level 1). Warns: T7-GET-05, T7-SET-01, T7-SET-02, T7-SET-06, T7-SET-07 – the SHOULD answers (Status events) that a Level 1 component does not give (REQ-CMP-017 applies from Level 2). They become passes with C-094. The device log of the D7 run shows the lifecycle tests: graceful stop with Close (`stopped (ok)`), restart, a connection lost when D7 cuts it (Will), then three refused TLS handshakes (TLS 1.1 offered by D7, an unknown CA, a wrong host name – T7-CON-02) and a reconnect.

Opt-in against `oec/dev` (`docker compose up -d --wait`, only the broker): the runner with `OEC_ROOT=generated/roots/oec-dev-app` connected over TLS with credentials. A script started D7 `monitor` first, then the runner, stopped it with SIGTERM (Close), started it again (Birth) and killed it with SIGKILL after 66 s (Will by the broker). Monitor: 7 messages, 0 invalid, T7-MON-01 … 04, 06, 08, 10 … 12, 16, 18, 19 pass. T7-MON-05 was skipped: an observation of 85 s is shorter than the 2 min 5 s the test needs. It passes in the gate run. Exit 4 (inconclusive) only because of the skips.

No defect was found in the core: the first D7 run had 0 fail. The only failures during the cycle were in my own tests: a wrong expected remaining length (32, not 30) and an empty pin list before the first run.

## Agent decisions (G-3.4 b)

- **A-439 Own MQTT 3.1.1 client** (ADR-0030 §7, amendment C-093): the subset the PAL needs, standard library only, in its own package with byte-exact tests. QoS 1 is accepted inbound (PUBACK), because D7 publishes its requests with QoS 1. The core always publishes QoS 0.
- **A-440 Runner layout:** each core source is one cgo translation unit (static names of the core stay private); config, device and transmit buffer are allocated in C, so no Go pointer is kept by C (cgo rules); all core calls on one OS-locked goroutine, events through a channel – the model of the ESP-IDF port (FreeRTOS queue).
- **A-441 Connection generations:** every `mqtt_connect` starts a new generation; events of older connections are dropped and a late CONNACK of an older connection is answered with DISCONNECT. That keeps a slow dial after a connect timeout of the core from reaching the core as a second connection. The ESP-IDF port needs the same rule for esp-mqtt events (noted for C-096).
- **A-442 Runner storage:** like a container application (RIG 4.1, `OEC_ROOT`); a client certificate is used when present, else the credentials (SF-117); `broker.json` needs all three elements; an encrypted key is rejected (the runner is a test harness; `oec-dev-cert` uses an encrypted key, so the opt-in run used `oec-dev-app`).
- **A-443 D7 harness as a copy:** `internal/d7run` copies the D7 part of `demotest` instead of importing the demos module: the micro SDK keeps its own Go module without a dependency on the demos, and the copy carries only what a device needs. A change to the D7 options must be made in both.
- **A-444 The test binary is the device:** `TestMain` runs the runner with `OECM_HOST_MAIN=1`, so D7 starts the test binary (no separate build step, the same cgo build as the tests).
- **A-445 Gate time:** step 9 now takes about 2 min 20 s (D7 Level 1 about 2 min, mostly the fixed waits of the restart and Health tests). Accepted; from C-094 the D7 run moves to `--level 2` instead of adding a second run.

## Open / next

- No SF, no D-x, no R-x (no new test data with a verdict; the D7 pins are results of the reviewed D7 catalog).
- **C-094:** Level 2 – bounded JSON pull reader and topic parser with libFuzzer targets (`fuzz/`), `Get`/`Set`/Status, Data and Config definitions, own number formatting; D7 `--level 2` in the gate.

## Docker

Before (own objects): none. Foreign containers `vdi2770-validator`, `aas-web-ui-twinsphere-*` and the network `oi4-oec-ifm-iolink-demo_default` were not touched.
Created for the opt-in run: `oec-dev-mosquitto-1`, network `oec-dev_default`, volume `oec-dev_data` (`docker compose up -d --wait` in `oec/dev`, no profile). Removed with `docker compose down -v`. After: no own object left; the runner process was stopped (SIGKILL as part of the run).

## Gate

`caffeinate -i oec/check.sh` → **all checks passed** (exit 0; step 9 with the D7 Level 1 run included). Only documentation (log, STATUS, ROADMAP, REVIEW, REVIEW_DE) was edited after the start of the gate run.
