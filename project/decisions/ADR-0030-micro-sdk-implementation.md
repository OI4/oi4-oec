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
