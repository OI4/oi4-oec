# ADR-0030 – Micro SDK implementation (P4b, D9)

- Status: **accepted (agent, G-3.4 b)** – the PO decided the language, the reference port and the scope (ADR-0016, ADR-0017, Q-019, Q-025, Q-027); the agent decided the implementation below (A-429 ff.)
- Date: 2026-11-08
- Deciders: agent (layout, build, test harness, tools, cycle cut); product owner (Thomas) – scope, languages, licenses, phase order
- Related: ADR-0006 (D7 independent), ADR-0012 (monorepo `oec/`), ADR-0016 (levels), ADR-0017 (C17 + PAL, ESP32), ADR-0022 (API contract `oecm.h`), ADR-0024 (libraries: esp-mqtt, esp-tls/mbedTLS, Unity, own JSON), ADR-0025 (D7), ADR-0028 (D7 in the demo gate), MG 6, MG 8, MG 9, ROADMAP P4b

## Context

ADR-0022 fixed the API of D9 (`oecm.h`, sketch of C-042) and ADR-0024 the libraries. P4b now builds the core, the ESP-IDF reference port, the example firmware and the Arduino packaging. It is delivered by Messaging level: L1, then L2, then L3. Each level must be shown by D7 `test` against a running device.

The work happens on a Mac without a connected ESP32. The real-hardware run therefore cannot be part of the gate. Espressif's QEMU fork emulates an `esp32` with an OpenCores Ethernet MAC and can run the same firmware against a broker on the host.

## Decision

1. **Layout** (`oec/sdk/micro/`; the sketch folder is removed, its header becomes the real one):

   | Path | Content |
   |---|---|
   | `include/oecm.h` | public API (ADR-0022 contract plus additive L2/L3 functions) |
   | `src/` | portable core, C17, only `<stdint.h>`, `<stddef.h>`, `<stdbool.h>`, `<string.h>` and `<limits.h>` from the C library |
   | `tests/` | Unity host tests, generated vector header, the message emitter for `oec-check validate` |
   | `third_party/unity/` | ThrowTheSwitch Unity **v2.7.0** (MIT, `src/` and `LICENSE.txt` only, unchanged) |
   | `fuzz/` | libFuzzer targets and their seed corpora |
   | `host/` | Go module `github.com/OI4/oec/sdk/micro/host`: the vector generator and the cgo host runner (PAL in Go) |
   | `port/esp-idf/` | ESP-IDF component `oecm` (core + ESP-IDF PAL) |
   | `examples/` | `esp-idf/` (generic firmware), `m5stack/` (M5Unified variant) |
   | `arduino/` | Arduino library packaging (best effort, ADR-0017 amendment C-010) |
   | `check.sh` | the micro gate, step 9 of `oec/check.sh` |

   Build output goes to `oec/sdk/micro/target/` (ignored like the Rust `target/`).

2. **Core rules** (ADR-0022, checked in the gate):
   - no heap and no `stdio`: the gate checks the undefined symbols of the core objects against an allowlist (`memcpy`, `memmove`, `memset`, `memcmp`, `strlen`, `strchr`; checked on a build without sanitizers and stack protector, `-ffreestanding -fno-builtin`); `malloc`, `free`, `printf`, `snprintf` and floating-point formatting from the C library are not allowed;
   - the device state is a private struct; a static assertion checks `sizeof` and `alignof` against `oecm_device`. `OECM_DEVICE_SIZE` and `OECM_TX_MIN` are measured per level and fixed in the header (ADR-0022 §3);
   - numbers (also `double` at L2) are formatted by the core itself;
   - the core is compiled with `-std=c17 -Wall -Wextra -Wpedantic -Wconversion -Wsign-conversion -Wshadow -Wcast-qual -Wstrict-prototypes -Wmissing-prototypes -Wvla -Wformat=2 -Werror`, with clang and, if installed, GCC 14/15.
3. **Host build:** CMake ≥ 3.20 builds the static library `oecm`, the Unity tests and the emitter into `target/host`, with `-fsanitize=address,undefined -fno-sanitize-recover=all`; `ctest` runs them. Apple clang on macOS, any C17 compiler with ASan/UBSan elsewhere.
4. **Test vectors:** the C tests use the shared vectors of `oec/spec/export/vectors/` (G-1.4). A stdlib Go generator (`host/cmd/oecm-vecgen`) writes `tests/vectors_gen.h` with the fields the tests need; the gate regenerates it and fails on a difference (`-check`). The C core gets no JSON file reader for its tests.
5. **Payload check:** the emitter (`tests/emit.c`) drives the core with a recording PAL and writes every message it produces as JSONL with the `role` (`birth`, `will`, `close`). The gate validates it with D7 `oec-check validate` (D1 schemas and the D7 payload rules), so the C encoder is judged by the same tool as the SDKs, not by its own expectations.
6. **Static analysis and fuzzing:**
   - `clang --analyze` over the core with `-analyzer-werror` in the gate;
   - libFuzzer targets for the topic parser and the JSON reader (from L2). The gate replays the seed corpora under ASan/UBSan with a plain driver; longer fuzz runs are opt-in with Homebrew/LLVM `clang -fsanitize=fuzzer` (`fuzz/run.sh`), results in the log.
7. **Host runner for D7 (from C-093):** the Go module `host` links the core with cgo and implements the PAL in Go: `crypto/tls` (TLS 1.2+, CA verification) and a small own MQTT 3.1.1 client (stdlib only; CONNECT with Will, PUBLISH QoS 0, SUBSCRIBE, PINGREQ, DISCONNECT). All core calls run on one OS-locked goroutine; network events reach it through a channel, as the ESP-IDF port does with a FreeRTOS queue. The runner is configured like a container application (`OEC_ROOT` with `etc/oi4/mqtt/broker.json`, CA and password file), so the D7 harness of the demos is reused as a copy (`RunD7`, test broker, proxy, restart hooks). D7 at the level of the increment runs in the gate with pinned pass IDs.
8. **ESP-IDF reference port and QEMU (opt-in):**
   - ESP-IDF **v6.1** (latest stable at the start of P4b, ADR-0024) is installed outside the workspace in `~/esp/esp-idf-v6.1`, tools in `~/.espressif` (opt-in, not in the gate); Espressif QEMU from `idf_tools.py install qemu-xtensa`;
   - the port component maps the PAL to esp-mqtt (`disable_auto_reconnect`, new Will per connect via `esp_mqtt_set_config`), esp-tls/mbedTLS, SNTP and NVS (ADR-0022 §7, ADR-0024);
   - `examples/esp-idf` builds for `esp32` and `esp32s3`; a QEMU build (`CONFIG_ETH_USE_OPENETH`, `-nic user,model=open_eth`) runs the firmware against the D7 proxy on the host (10.0.2.2), D7 at the level of the increment; the restart hook kills and restarts QEMU;
   - the run on a real ESP32 board and an M5Stack is an opt-in step with instructions (Wi-Fi and broker settings are never committed); without hardware in the session it goes to the PO at the P4b exit.
9. **DataSetWriterIds:** the core assigns the ids of its publications in definition order from 10 (MAM 10, Health 11, then the L2/L3 definitions), so they are stable across restarts of the same firmware (REQ-MSG-020/021). Ids of `NewDataSetWriterId` (L3) are taken above the definitions.
10. **Cycle cut (one increment per cycle, test first, gate green at the end of each):**
    1. **C-092** – this ADR; core L1: IL validation, base64url, dnp, Oi4Identifier, topic building, JSON writer, NetworkMessage/MAM/Health, lifecycle (Birth, Will per connect, Close, heartbeat, recovery Birth, reconnect backoff); host tests against the vectors under ASan/UBSan; emitter + `oec-check validate`; `clang --analyze`; symbol allowlist; `micro/check.sh` as gate step 9.
    2. **C-093** – Go host runner (cgo PAL, own MQTT 3.1.1 client on `crypto/tls`); D7 `--level 1` in the gate with pinned passes.
    3. **C-094** – L2: bounded JSON pull reader + topic parser (fuzz targets), `Get` of MAM/Health/Profile/Data/Metadata/Config/ReferenceDesignation with empty lists and pagination, `Set` with Status event, Data and Config definitions with typed storage, own number formatting; D7 `--level 2` in the gate.
    4. **C-095** – L3: all resources and methods of REQ-CMP-010/033 for the device (lists, `Set`/`Del` ReferenceDesignation with persistence, SubscriptionList, PublicationList, `Call`/`Reply` `NewDataSetWriterId`); D7 `--level 3` in the gate.
    5. **C-096** – ESP-IDF port, example firmware for `esp32`/`esp32s3`, QEMU run with D7 (opt-in, in the log), footprint (RAM/flash) documented.
    6. **C-097** – Arduino library and M5Stack example (build checks, opt-in), docs page `oec/docs/docs/micro-sdk/`, exit assessment; the real-hardware run as D-x to the PO.

    A cycle may take two increments when they are small; a split is recorded in the log.

## Consequences

- The gate grows by step 9 (host build with sanitizers, tests, vector freshness, payload validation, analyzer, symbol check; from C-093 a D7 run). It needs `cmake` and a C compiler with ASan/UBSan; ESP-IDF, QEMU and libFuzzer stay opt-in.
- The core can be judged on the host by D7 without an ESP32. The ESP-IDF port adds only the PAL, so the QEMU and hardware runs check the port, not the protocol logic again.
- The Go host runner is a test harness, not a product. Its MQTT client is minimal and covers only what the PAL needs.

## Alternatives considered

- **Host PAL in C on an existing MQTT library (Paho C, mosquitto client):** a new C dependency with OpenSSL in the gate. Rejected; Go stdlib covers TLS and the small MQTT subset.
- **Parse the vector JSON in the C tests:** needs a JSON file reader before L2 and mixes test and product code. Rejected for a generated header.
- **Validate the C output only with own C assertions:** would test the encoder against itself. Rejected; D7 `validate` is the independent judge (ADR-0006).
- **QEMU in the gate:** needs ESP-IDF (~2 GB) and long builds. Rejected; opt-in like the Docker runs.

## Amendment C-093 (2026-11-09) – increment 2: host runner and D7 Level 1 (agent, G-3.4 b)

Refines §7 (A-439 … A-445):

- **Layout:** `host/mqtt` (MQTT 3.1.1 client, stdlib), `host/cmd/oecm-host` (the runner: cgo PAL, storage, event loop), `host/internal/d7run` (copy of the D7 part of `oec/demos/internal/demotest`; the demos module is not imported, so the micro SDK keeps its own module and the copy carries only what a device needs). Each core source is compiled as its own cgo translation unit (`core_*.c` include `src/oecm_*.c`), so the static names of the core do not clash.
- **MQTT subset:** CONNECT (clean session, Will QoS 0 without retain, user/password, the keepalive of the core, 60 s by default), CONNACK, PUBLISH QoS 0 out, PUBLISH QoS 0/1 in (PUBACK for QoS 1, D7 sends with QoS 1), SUBSCRIBE QoS 0 (SUBACK refusal = error), PINGREQ every keepalive/2 (an unanswered ping closes the connection), DISCONNECT. Incoming packets are bounded by `MaxPacketSize` + 65544 (SF-105).
- **Events by generation:** every connect gets a generation number; events of an older connection are dropped, a late CONNACK of an older connection is answered with DISCONNECT (no Will). This mirrors what the ESP-IDF port must do with esp-mqtt events after `esp_mqtt_client_stop`.
- **Storage (RIG 4.1):** `OEC_ROOT` (default `/`): hostname from `proc/sys/kernel/hostname`, else `etc/hostname`; `etc/oi4/mqtt/broker.json` (all three elements required); CA `etc/oi4/mqtt/broker_ca.pem`, else `etc/oi4/certs/ca.pem`; a client certificate (`run/secrets/oi4/private_key.pem` + `etc/oi4/certs/<hostname>.pem`) is used when present, else `run/secrets/oi4/credentials` (SF-117: the means the platform provides). An encrypted key (passphrase file) is rejected: the runner is a test harness, the ESP32 port has its own key storage.
- **The test binary is the device:** `TestMain` runs the runner when `OECM_HOST_MAIN=1`, so D7 starts the test binary itself (no separate build step in the gate); the restart hooks stop it with SIGTERM (Close) and start it again.
- **Gate:** `micro/check.sh` builds `oec-check` and `go-testbroker` first, runs `go test -race ./mqtt` and `go test ./...` with `OECM_REQUIRE=1` (a missing tool fails). The D7 Level 1 run takes about 2 min. Pinned: 21 pass (the list of the config service without T7-ADR-01, the device sends no requests), 5 warn for the SHOULD answers a Level 1 device does not give, 0 fail.

## Amendment C-094 (2026-11-10) – increment 3: Level 2, D7 Level 2, fuzzing (agent, G-3.4 b)

Refines §4, §6, §9 and §10 (A-446 … A-457; details in `log/2026-11-10_C-094_micro-level2.md`):

- **Scope** (A-446): `Get` of MAM, Health, Profile, Data, Metadata, Config, PublicationList, SubscriptionList (pagination REQ-PAG-002 … 008); `Set` of Data, Config, PublicationList, SubscriptionList; `Del` of SubscriptionList; Status events. PublicationList and SubscriptionList moved from C-095 to C-094; ReferenceDesignation, `NewDataSetWriterId` and persistence stay in C-095.
- **Sources:** `oecm_num.c` (numbers, time, SHA-1, UUIDv5), `oecm_json.c` (pull reader, no recursion, depth ≤ 16), `oecm_topic.c`, `oecm_res.c` (encoders, pagination, connect publications), `oecm_req.c` (requests). The host runner compiles each as its own `core_*.c` unit.
- **DataSetWriterIds** (A-448, §9): MAM 10, Health 11, Profile 12, PublicationList 13, Status 14/15/16 (Good/Uncertain/Bad), SubscriptionList 17, Config contexts and Data DataSets from 18 in definition order (Metadata shares the Data id). Inbound helper DataSets 1 … 9 (REQ-MSG-019): PaginationRequest 1 used, Locale 3 and Query 9 ignored.
- **Deviations from the Rust provider** (A-449 … A-452): the first violation in document order of the request; the MessageId pattern checked on the raw token; duplicate keys all checked, the last applies; a value larger than its buffer → `BadOutOfRange`; a TopicPath that does not fit the transmit buffer → `BadInvalidArgument`; list entries `NONE_0` configurable; no Validation `Pattern`; `double` output not correctly rounded beyond 15 … 17 significant digits; subscriptions before the Birth (as Rust); no Syslog (REQ-TOP-027 SHOULD, T7-SET-12 skipped).
- **Fuzzing** (A-454, §6): `fuzz/fuzz_json.c`, `fuzz_topic.c`, `fuzz_message.c` (a connected Level 2 device; traps on a publication outside `Oi4v2/` or with invalid JSON), `replay.c`, seed corpora; ctest replays them under ASan + UBSan in the gate; `fuzz/run.sh` (opt-in) fuzzes with libFuzzer and replays the grown corpus. On macOS the fuzzing runs with UBSan only (the ASan runtime of Homebrew LLVM 21 hangs at start-up on macOS 27), ASan comes from the replay.
- **D7 in the gate** (A-455, revises A-445): `TestD7Level1` (21 pinned passes) and `TestD7Level2` (the simulated sensor `cmd/oecm-host/demo.c`, `-level 2`, 38 pinned passes, 3 warns for Level 3 features); step 9 takes about 4 min.
- **Symbol check** (A-457): the plain build sets `-U_FORTIFY_SOURCE -D_FORTIFY_SOURCE=0`, so the checked `memset` of the macOS headers is not lowered to `bzero` (not on the allowlist).

## Amendment C-095 (2026-11-11) – increment 4: Level 3, D7 Level 3 (agent, G-3.4 b)

Refines §4, §6, §7, §9 and §10 (A-458 … A-466; details in `log/2026-11-11_C-095_micro-level3.md`):

- **Scope:** `Get` (with Query), `Set`, `Del` of the device's ReferenceDesignation with Status events, publication on change and persistence (REQ-RES-REFDES-002); `Call`/`Reply NewDataSetWriterId` (MG 9.2.1, REQ-ACT-023) with persisted reservations. The lists (PublicationList, SubscriptionList) were already done in C-094 (A-446). Persisted Config values are not part of the core (A-463).
- **Source:** `oecm_l3.c`; hooks in `oecm_res.c` (Profile, lists, publication) and `oecm_req.c` (dispatch); the host runner compiles it as `core_l3.c`.
- **DataSetWriterIds** (§9): the ReferenceDesignation takes the id after the Config contexts and Data DataSets; ids of `NewDataSetWriterId` start after it, are reserved in persisted blocks of 256, wrap after 65535 to the first id and are limited to 256 per 60 s (A-460).
- **Deviations from the Rust provider** (A-461, A-462): `NewDataSetWriterId` in the Profile from Level 3 only; requests to the ReferenceDesignation and `Call` topics with a Filter level are ignored at every level; duplicate fields make a `Call` ignored, an array-form `Payload` is rejected.
- **Host runner** (§7, A-464): `-level 3`, `-state DIR` (default `OEC_ROOT/var/lib/oecm-host`, one file per record, atomic rename); `internal/d7run` `Files` seeds the storage.
- **D7 in the gate** (A-464): `TestD7Level3` with a stored ReferenceDesignation, 50 pinned passes, 0 warn; the Level 1 (21) and Level 2 (38) runs stay; step 9 takes about 5 min.
- **Payload check and fuzzing** (§5, §6, A-465): the emitter adds a Level 3 lifecycle (69 messages valid); `fuzz_message` drives a Level 3 device with an in-memory store reset per input, 6 new seeds.

## Amendment C-096 (2026-11-12) – increment 5: ESP-IDF port, example firmware, QEMU (agent, G-3.4 b)

Refines §1, §7, §8 and §10 (A-467 … A-478; details in `log/2026-11-12_C-096_esp-idf-port.md`):

- **Layout** (§1, A-467): the component is `port/esp-idf/oecm/` (ESP-IDF names a component after its directory), with `idf_component.yml` (`espressif/mqtt ^1.1.0`); the simulated sensor moved from the host runner to `examples/sensor/` and is shared by the host runner and the firmware (A-471); `examples/esp-idf/` is the example firmware with `build.sh` (variants `esp32`, `esp32s3`, `qemu`) and the committed `dependencies.lock`.
- **PAL** (§8, A-468): esp-mqtt events go through a FreeRTOS ring buffer to `oecm_esp_dispatch` on the application task (tagged with a connection generation; fragmented messages reassembled up to `rx_size`); no auto-reconnect; the Will is set with `esp_mqtt_set_config` before every connect; `persist_*` in NVS blobs (namespace `oecm`); no certificate bundle, the CA comes from the provisioning (A-476).
- **Provisioning** (A-469): the firmware reads broker, CA, credentials or client certificate, SerialNumber, level, MaxPacketSize, SNTP server and Wi-Fi from the NVS partition `oecprov` (namespace `oec`); nothing of a site is compiled in or committed (§8). The serial console takes `stop` and `time <unix ms>` (A-470).
- **QEMU** (§7, §8, A-472 … A-475): the Go command `host/cmd/oecm-qemu` runs the `qemu` build like `oecm-host` (same container storage below `OEC_ROOT`, loopback → 10.0.2.2, flash image with NVS seeding, clock over the console, SIGTERM → `stop`); the pinned D7 passes moved to `internal/d7run/pins.go` and `d7run.D7` gained `ProxyNames`, `Timeout` and `RestartWait`; D7 against QEMU is opt-in (`OECM_QEMU_BUILD`, `OECM_QEMU`), never in the gate.
- **Footprint** (A-477): see the log and the README.
