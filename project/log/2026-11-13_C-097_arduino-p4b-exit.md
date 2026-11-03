# C-097 – P4b: Arduino library, M5Stack example, docs page, P4b exit

- Date: 2026-11-13
- Phase: P4b – Micro SDK (D9), increment 6 of 6 (ADR-0030 §10)
- PO input: none new (C-091 addendum 1: P4b in one session; ask only when something really does not work, is unclear or needs a guideline change).
- Baseline: guideline `52d8470` (unchanged since C-078), SF-001 … 117 resolved, next SF-118.

## Goal

The last P4b increment: the micro SDK as an Arduino library (ADR-0024: generated from the core and the ESP-IDF PAL on the esp-mqtt of the Arduino-ESP32 core, fallback arduino-mqtt), the M5Stack example (M5Unified, display and buttons), build checks (opt-in), the docs page `oec/docs/docs/micro-sdk/` (P8 input) and the P4b exit assessment with a D-x for the run on real hardware.

## What was done

1. **esp-mqtt in the Arduino cores (ADR-0024 check, A-479):** only the `esp32-libs` archives of the Arduino-ESP32 releases were inspected first (checksums of the package index verified):
   - **3.3.12** (ESP-IDF 5.5.5) ships `libmqtt.a` with `mqtt_client.h`; it differs from esp-mqtt 1.1.0 only in fields the PAL does not use (connection-state enum, `reason_code`, `ciphersuites_list`).
   - **4.0.0-RC1** (ESP-IDF 6.1) ships the managed component `libespressif__mqtt.a`; its `mqtt_client.h` is identical to the esp-mqtt 1.1.0 of the ESP-IDF build of C-096.
   - So the PAL `port/esp-idf/oecm/oecm_esp.c` is used unchanged; no arduino-mqtt fallback, no second MQTT client.
2. **API addition, test first (A-483):** `bool oecm_connected(const oecm_device *dev)` (`include/oecm.h`, `src/oecm_device.c`): true from `oecm_on_connected` until the disconnect or `oecm_stop`; for a status display. Unity test `test_connected_state` (`tests/test_device.c`) failed first, then passed; 88 Unity tests, ctest 11/11. `examples/sensor/oecm_sensor.{h,c}`: C++ guards (`extern "C"`) and `oecm_sensor_value()`/`oecm_sensor_setpoint()` for the display. ADR-0022 amendment C-097.
3. **Arduino library `oec/sdk/micro/arduino/`** (A-480 … A-482):
   - `library.properties` (name **OECMicro**, version 0.1.0, `architectures=esp32`, `includes=OECMicro.h`, `category=Communication`, url `https://oi4.org`), `OECMicro.h` (16 lines: `oecm.h` + `oecm_esp.h`), `README.md` (71 lines: install, boards, examples, provisioning, use in an own sketch, licenses).
   - `package.sh [out]` (34 lines): an Arduino library cannot refer to files outside its folder, so it copies `src/*.{c,h}`, `include/oecm.h`, `port/esp-idf/oecm/oecm_esp.{c,h}` (from `include/`) and `OECMicro.h` flat into `OECMicro/src/`, `examples/sensor/oecm_sensor.{c,h}` into each example, `oec/LICENSE` (MIT); output `target/arduino/OECMicro/` and `OECMicro-0.1.0.zip` (zip only if `zip` exists); `target/` is git-ignored.
   - `examples/Sensor/Sensor.ino` (143 lines): Wi-Fi station, `configTime` and wait for a plausible time (≥ 2023-01-01), MAM (Manufacturer OI4, Model "OEC Micro Arduino", ProductCode `OECM-ARDUINO`, SerialNumber = base MAC, SoftwareRevision 0.1.0, DeviceClass "Sensor"), the shared sensor at `LEVEL` 3, loop `oecm_esp_dispatch` (≤ 200 ms) + `oecm_poll` + the sensor tick; the serial monitor takes `stop`.
   - `examples/M5Stack/M5Stack.ino` (166 lines): the same device (Model "OEC Micro M5Stack", `OECM-M5STACK`) with M5Unified: the display shows Identification Link, Level, connected/connecting (`oecm_connected`), Health and Value/Setpoint and is redrawn only on change; button A or a tap cycles the Health NORMAL/100 → MAINTENANCE_REQUIRED/70 → OFF_SPEC/50 → CHECK_FUNCTION/80 → FAILURE/0 (`oecm_set_health`); dispatch wait ≤ 50 ms for the buttons.
   - `examples/*/arduino_secrets.h` (14 lines each): the provisioning as compile-time placeholders (Arduino convention; Wi-Fi, broker URI, CA, username/password or client certificate/key, SNTP server), all secret values empty in the repository.
4. **Gate step** in `micro/check.sh` (A-484): "micro: arduino/package.sh (library layout, C-097), headers as C++17" – packages into a temporary directory; `cmp` checks every copied core/port/header source against its origin; checks the required `library.properties` fields and each example (`.ino`, `arduino_secrets.h`, `oecm_sensor.{c,h}`); fails if a secret placeholder in `arduino_secrets.h` is not empty (tested with a non-empty value: fails); compiles `oecm.h` and `oecm_sensor.h` as C++17 with `-fsyntax-only -Werror -Wall -Wextra`. No Arduino toolchain needed in the gate. Header comment of `check.sh` extended.
5. **Harness defect fixed (A-487):** while checking the load before the gate, six `qemu-system-xtensa` processes of the C-096 opt-in runs (`TestQEMUD7Level1/2/3`, two runs each) were still running (about 2 h, parent 1). Cause: the cleanup of `internal/d7run` ended the device process with SIGKILL; the QEMU runner cannot forward a SIGKILL, so its QEMU child survived. Now the cleanup sends SIGTERM (the runner sends `stop` and ends QEMU), waits up to 20 s and only then sends SIGKILL. The six processes were stopped (`kill <pid>`). Re-run `TestQEMUD7Level1` (opt-in): pass in 245 s, **no QEMU process left**. The host runner (no child process) behaves as before.
6. **Micro README:** state, path table (`arduino/`), `oecm_connected`, Sizes with the Arduino table, Check paragraph with the Arduino step, new sections "Run on real hardware (opt-in)" (ADR-0030: instructions for the run without a board in the session – broker reachable with `-san <ip>` + `OEC_DEV_BIND`, flash, provisioning, D7 with the probe user, proxy/restart hook, Arduino upload) and "Arduino library (C-097)".
7. **Docs page** `oec/docs/docs/micro-sdk/index.md` (137 lines, `sidebar_position` 3, like the demo and registry pages, A-379): purpose, levels, start (host runner, ESP-IDF, QEMU, Arduino), other platforms (PAL), checks, limits, links to the READMEs; relative links checked.
8. ADR-0030, ADR-0024 and ADR-0022 amendments C-097.

## Results

**Tools (opt-in, outside the workspace, A-488):** Homebrew `arduino-cli` 1.5.1 (GPL-3.0, a tool, not linked); an own config `~/.arduino-oecm/arduino-cli.yaml` (data, downloads and sketchbook below `~/.arduino-oecm`, not in the default locations); Arduino-ESP32 **3.3.12** (install 4 min 11 s), then **4.0.0-RC1** (Espressif development index; 5 min 15 s; replaces 3.3.12); M5Unified 0.2.29 + M5GFX 0.2.36 (MIT); `arduino-lint` 1.3.0 (GitHub release, checksum verified) in `/tmp`.

**Builds** (`arduino-cli compile --warnings all`, library from `package.sh`): all compile on both cores; **no warning from the library or the sketches** (the only warning on 4.0.0-RC1 is in the core: `esp32-hal-ledc.c:262` `intr_type` deprecated). Core + PAL = the linked `oecm_*.c.o` objects of the map without the example sensor (string constants merged by the linker count with the sketch; C-096 `liboecm.a` 33.6 KB):

| Sketch, board (FQBN `esp32:esp32:…`) | 3.3.12 flash | globals | core + PAL | 4.0.0-RC1 flash | globals | core + PAL |
|---|---|---|---|---|---|---|
| `Sensor`, `esp32` | 1,054,306 (80 %) | 59,332 | 35,438 | 1,123,293 (85 %) | 61,784 | 35,188 |
| `Sensor`, `esp32s3` | 1,018,964 (77 %) | 57,968 | 35,146 | 1,089,310 (83 %) | 59,024 | 34,898 |
| `M5Stack`, `m5stack_core2` | 1,240,659 (18 % of 6.5 MB) | 63,492 | 35,390 | 1,308,899 (19 %) | 65,460 | 35,141 |
| `M5Stack`, `m5stack_cores3` | 1,212,719 (38 % of 3 MB) | 61,804 | 35,102 | 1,285,215 (40 %) | 63,136 | 34,849 |
| `M5Stack`, `m5stack_core` | 1,228,623 (93 %) | 63,444 | 35,390 | 1,296,907 (98 %) | 65,396 | 35,141 |

- Core + PAL use **no static RAM** in any build (the 69 octets of `.dram0` belong to the example sensor).
- Best effort (ROADMAP P4b targets): `Sensor` on 4.0.0-RC1 for **`esp32c3`** 1,191,971 (90 %), globals 50,460, core + PAL 38,340; **`esp32c6`** 1,217,871 (92 %), globals 56,140, core + PAL 38,390 (RISC-V code is larger than Xtensa); no warning from the library.
- The M5Core (4 MB flash, default partition scheme with a 1.25 MB app) is nearly full with M5Unified (93 %/98 %): the README recommends a larger partition scheme there.
- First compile 24 s, later ones a few seconds.

**arduino-lint** (`--compliance strict`): the sketches have no finding. The library has one error left: **LP042** – the `url` `https://oi4.org` answers the HEAD request with 405. The repository `OI4/oi4-oec` is private, so it is not a usable url either. The url stays; it is a publication matter (D-1, carried to P9). The name was changed from "OEC Micro" to **OECMicro** after LP015/LS008 (spaces in the name).

**No QEMU run of the sketches (A-485):** the Arduino-ESP32 core is built without the OpenCores Ethernet driver (`CONFIG_ETH_USE_OPENETH` absent in its `sdkconfig.h`), and the QEMU does not emulate Wi-Fi. The evidence for the Arduino library is the compile on both cores plus the identical sources (the gate's `cmp`) that pass D7 Level 1/2/3 in the QEMU (C-096) and on the host (gate).

**Micro check** (`oec/sdk/micro/check.sh` alone, 4 min 18 s): green, including the new Arduino step.

## Agent decisions (G-3.4 b)

- **A-479 esp-mqtt in the Arduino cores:** 3.3.12 and 4.0.0-RC1 ship esp-mqtt with the API of the PAL; the ESP-IDF PAL is reused unchanged (ADR-0024 main path), no arduino-mqtt fallback; persistence over NVS directly, not `Preferences` (ADR-0024 amendment C-097).
- **A-480 Library layout:** sources in `arduino/` (properties, umbrella header, README, sketches, `package.sh`); the installable library is assembled by `package.sh` into `target/arduino/` (flat copy; nothing generated committed). Name `OECMicro` (arduino-lint), version 0.1.0, `architectures=esp32`.
- **A-481 Provisioning in sketches:** `arduino_secrets.h` with compile-time placeholders (Arduino convention) instead of the NVS partition `oecprov` of the ESP-IDF firmware (Arduino users have no partition-generator step); the committed placeholders stay empty (gate). The Level 3 records stay in NVS namespace `oecm`; the Arduino core initializes NVS.
- **A-482 M5Stack example as an Arduino sketch:** `arduino/examples/M5Stack` (M5Unified) instead of an ESP-IDF variant `examples/m5stack/` (ADR-0030 §1 amended): M5Unified is an Arduino library first; one M5Stack variant suffices for the example (ADR-0017 amendment C-011: example only). Health cycle by button A/tap shows the Health path to D7 `monitor` and the registry.
- **A-483 `oecm_connected`:** additive read-only API for status displays (ADR-0022 amendment C-097); the sensor accessors and C++ guards make the shared sensor usable from a `.ino`.
- **A-484 Gate step:** the gate checks the package (copy unchanged, examples complete, secrets empty, headers valid C++17) without an Arduino toolchain; compiling the sketches stays opt-in (like ESP-IDF and QEMU, ADR-0030 §8).
- **A-485 Arduino evidence = compile only:** no QEMU network in the Arduino core; runtime behavior is the identical core + PAL checked in QEMU and on the host.
- **A-486 arduino-lint LP042:** the `url` stays `https://oi4.org` until publication; a reachable public url is a precondition for the Arduino Library Manager (P9, G-3.4 a: publication).
- **A-487 Harness defect:** `internal/d7run` stops the device with SIGTERM, SIGKILL only after 20 s (orphaned QEMU processes of C-096).
- **A-488 Tools and disk:** `arduino-cli` from Homebrew with an own config below `~/.arduino-oecm`; the download cache (`staging`, 3.8 GB) removed, the installed 4.0.0-RC1 core (7.3 GB) kept for re-checks (`rm -rf ~/.arduino-oecm` frees it); `/tmp` build directories, `arduino-lint` and the index files removed.

## P4b exit assessment

ROADMAP P4b, item by item (evidence: gate = `oec/check.sh` step 9, opt-in = log):

| # | Item | State | Evidence |
|---|---|---|---|
| 1 | Portable core (IL, topics, JSON, MAM/Health/NetworkMessage, lifecycle) + PAL; ESP-IDF port on esp-mqtt + esp-tls/mbedTLS; own JSON encoder + bounded pull reader; `oecm_`; `OTConnector` | ✅ | C-092 … C-096; gate: 88 Unity tests, payload check (69 messages valid) |
| 2 | Arduino library (best effort) | ✅ | C-097: library + 2 examples, gate step (package), opt-in builds on 3.3.12 and 4.0.0-RC1 |
| 3 | C safety: static analysis, ASan/UBSan, fuzzing | ✅ | gate: clang/GCC `-Werror`, `clang --analyze`, ASan/UBSan, fuzz seed replay, symbol allowlist; opt-in libFuzzer 0 findings (C-094/C-095) |
| 4 | Host unit tests on macOS/Windows/Linux | ◐ | macOS in the gate; Windows/Linux need CI → **P9** (like P4/P5) |
| 5 | Integration in Espressif QEMU against the broker with D7 | ✅ | opt-in C-096: Level 1 21, Level 2 38, Level 3 50 pass, 0 fail (test broker); `oec/dev` Level 3 38 pass, 0 fail; C-097 Level 1 re-run pass |
| 6 | Final check on a real ESP32 board | ✗ | no board in the session; instructions in the micro README → **D-1** |
| 7 | Scope Level 3, delivered L1 → L2 → L3 | ✅ | gate: D7 `test` on the host runner Level 1 (21), 2 (38), 3 (50 pass), 0 fail |
| 8 | Explicit configuration, static memory, documented footprint | ✅ | no heap in the core; README Sizes (host, Cortex-M4, ESP-IDF, Arduino) |
| 9 | Example firmware for the reference board | ✅ | `examples/esp-idf` (`esp32`, `esp32s3`, `qemu`), Arduino `Sensor` |
| 10 | Targets `esp32` + `esp32s3`; C3/C6 best effort; M5Stack Core2/CoreS3 variant | ✅ (compile) | ESP-IDF builds (C-096) and Arduino builds; C3/C6 Arduino compile (C-097); M5Stack sketch for Core2/CoreS3/Core |
| **Exit** | Firmware on the ESP32 board and the M5Stack (and in QEMU, if feasible) passes D7 `--level 1/2/3` | ◐ | **QEMU: met** (all three levels). **Real ESP32 board and M5Stack: not run** (no hardware; the M5Stack sketch is compile-only) |

Deviation: the exit criterion names the real reference hardware; in the session only QEMU (and the identical code on the host) is possible – this was foreseen in ADR-0030 §10 and in the C-092 plan ("D-x at the exit").

**D-1 to the PO – close P4b?** (G-3.4 a: phase order, publication). Proposal (a): close P4b with these carried items:
- **Run on real hardware** (ESP32 board, M5Stack Core2/CoreS3): an opt-in step with the instructions in `oec/sdk/micro/README.md` ("Run on real hardware"), as soon as a board is available – by the PO or in a later cycle; a defect found there is fixed as a P4b follow-up.
- **CI** (host tests on Linux/Windows, ESP-IDF and Arduino builds, C3/C6) → P9.
- **Publication** (Arduino Library Manager – needs a public repository and a reachable `url`, LP042; ESP Component Registry) → P9.
- **Arduino-ESP32 4.0 final:** re-check the builds when 4.0.0 is released (RC1 today) → P9.

Next phase: **P8 – Documentation** (recommended; the docs pages of the demos, the registry and the micro SDK exist, the Docusaurus site is the next step) or **P9 – Hardening & release**. Alternatives: (b) close P4b, next P9; (c) keep P4b open until the hardware run.

## Open / next

- D-1 (PO).
- A-113 and A-422 unchanged (info).
- After D-1: P8 or P9 per the PO.

## Docker

None in C-097 (no container, volume, network or image created).

## Gate

`caffeinate -i oec/check.sh` – **all checks passed** (82 min 36 s wall clock, mostly waiting under background load (Spotlight indexing); no repeat needed). The new micro step `arduino/package.sh (library layout, C-097), headers as C++17` passed; D7 `validate` of the micro examples: 69 messages, 0 invalid.

## Addendum 1 – PO decision on the P4b exit (2026-11-13)

PO answer (chat): "D-1 (a) aber, noch nicht im Arduino Library veröffentlichen".

### Applied

- **D-1 (a): P4b is closed.** ROADMAP P4b heading ✅ and "Decision" paragraph; STATUS (current phase P8, previous phase P4b, D9 done, next step, cycle row); REVIEW/REVIEW_DE (D-1 answered).
- **Carried items:**
  - run on real hardware (ESP32 board, M5Stack Core2/CoreS3): opt-in step per `oec/sdk/micro/README.md` "Run on real hardware" when a board is available; a defect found there is a P4b follow-up;
  - CI (host tests on Linux/Windows, ESP-IDF and Arduino builds, C3/C6) → P9;
  - ESP Component Registry → P9 (publication, the PO decides, Q-011);
  - re-check on Arduino-ESP32 4.0 final → P9.
- **Restriction (PO):** the Arduino Library Manager **not yet** (PO: "noch nicht im Arduino Library veröffentlichen") – only on a later explicit PO order, not part of P9 by itself. The library stays a ZIP built by `arduino/package.sh`; `library.properties` and arduino-lint LP042 (A-486) stay as they are. Recorded in ROADMAP P9 and in Q-011 (OPEN-QUESTIONS).
- **Next phase: P8 – Documentation (D6)**, starting with C-098.
