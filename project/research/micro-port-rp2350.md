# Micro SDK on the Raspberry Pi Pico 2 / Pico 2 W – assessment

- Status: **assessment** (2026-11-06, PO question "wird es wohl auch auf einem
  Raspberry Pi Pico 2 laufen? Wenn nicht können wir es aufnehmen?"). A new
  port is scope (GUARDRAILS G-3.4 a): the PO decides whether and when; this
  note is the input for that decision.
- Related: ADR-0017 (C17 core + PAL, reference port ESP32, "further ports
  possible later through the PAL"), ADR-0022 (API `oecm.h`), ADR-0024
  (micro SDK libraries), ADR-0030 (implementation), `oec/sdk/micro/README.md`
  ("Limits": ESP32 family only as a ready port).

## 1. Short answer

- **Pico 2 (RP2350, no radio): no** – the board has no network interface.
  It would need an external Ethernet or Wi-Fi module (e.g. WIZnet W5500) and
  a PAL for it; not proposed.
- **Pico 2 W (RP2350 + Infineon CYW43439 Wi-Fi): not today, but feasible.**
  Today the SDK has only the ESP-IDF PAL (ESP32 family, also through
  Arduino-ESP32). The C17 core is portable and has no ESP dependency; a Pico
  2 W needs a second PAL (`port/pico-sdk/`) and an example firmware.

## 2. Fit of the core

| | needed (ESP32 build, Level 3) | Pico 2 W |
|---|---|---|
| flash | core + PAL ≈ 34 KB, with MQTT/TLS/Wi-Fi ≈ 0.9 MB app image | 4 MB |
| RAM | device 1 KB, transmit 8 KB, ReferenceDesignation 2 KB, PAL receive 8 KB + ring 16 KB, TLS session ≈ 40–60 KB | 520 KB SRAM |
| C17, no heap in the core | yes | `arm-none-eabi-gcc` (Cortex-M33) or RISC-V (Hazard3) |
| wall clock | SNTP | lwIP SNTP |
| randomness for TLS | ESP RNG | RP2350 TRNG (`pico_rand`) |

The core fits with a large margin. Nothing in the core needs to change.

## 3. What the PAL needs (`oecm_pal`, `oecm.h`)

| PAL function | Pico SDK (2.3.1, 2026-09) |
|---|---|
| `mqtt_connect` (TLS ≥ 1.2, MQTT 3.1.1, new Will per connect) | lwIP `altcp_tls` + mbedTLS for TLS; **MQTT client: see §4** |
| `mqtt_publish` (QoS 0), `mqtt_subscribe`, `mqtt_disconnect` | MQTT client (§4) |
| `unix_time_ms`, `monotonic_ms` | lwIP SNTP + `time_us_64()` |
| `persist_load`/`persist_save` (Level 3: `oecm.refdes`, `oecm.wid`) | no NVS: a small record store in the last flash sectors (`hardware_flash`, `flash_safe_execute` because the Wi-Fi driver and XIP share the flash) |
| broker settings, CA, client certificate or credentials | compiled in from a git-ignored header, as `arduino_secrets.h` (never committed) |

## 4. Finding: the lwIP MQTT client cannot send our Will

The Pico SDK brings the lwIP MQTT client (`lwip/apps/mqtt.h`). It limits the
Will message to 255 octets (`u8_t will_msg_len`, `LWIP_ERROR(… len <= 0xFF)`
in `mqtt_client_connect`). The Will of the core is a Health message with
`FAILURE_1` and score 0 (REQ-LIFE-008/016); for the host example
(`examples/host/hello_device.c`, Identification Link of 47 octets) a Health
message is **436 octets**, longer with longer identifiers. Its default
buffers (`MQTT_OUTPUT_RINGBUF_SIZE` 256, `MQTT_VAR_HEADER_BUFFER_LEN` 128) are
configurable; the Will limit is a type, not a setting.

Options for the MQTT client of the port:
1. **coreMQTT** (FreeRTOS, MIT): MQTT 3.1.1, no Will limit beyond MQTT,
   transport interface over lwIP `altcp_tls`/mbedTLS. Recommended.
2. A local patch of lwIP MQTT (`u16_t` Will length) – a fork of a platform
   library; not proposed.
3. Own minimal MQTT 3.1.1 client in C (the host runner has one in Go) – more
   code to own; only if 1 fails.

## 5. Licenses (G-3.4 a: the PO decides)

| Component | License |
|---|---|
| Pico SDK, picotool | BSD-3-Clause |
| lwIP | BSD-3-Clause |
| mbedTLS | Apache-2.0 (or GPL-2.0-or-later) |
| coreMQTT | MIT |
| cyw43-driver (Wi-Fi) | **v1.1.1 in Pico SDK 2.3.1:** own terms – free for use with Raspberry Pi semiconductor devices (`LICENSE.RP`), otherwise non-commercial; **v2.0.0 (2026-09):** MIT. No Pico SDK release brings v2.0.0 yet; a port uses v1.1.1 under `LICENSE.RP` (on a Pico 2 W that is a Raspberry Pi device) or pins the driver submodule at v2.0.0. |

No copyleft in the firmware path (mbedTLS is taken under Apache-2.0).

## 6. Verification without hardware

- Upstream QEMU has no RP2350 machine (open feature request); the community
  fork found emulates the CPU only, no CYW43439. Unlike the ESP32 (Espressif QEMU
  with OpenCores Ethernet, D7 Level 1–3 in CI) there is **no emulated D7
  run** for the Pico 2 W.
- Possible in CI and the gate: the build of the port and the example
  (`arm-none-eabi-gcc` and Pico SDK in a container pinned by digest), host
  unit tests of the PAL parts that do not need the radio (record store with
  a flash fake, Will/connect mapping, reassembly), the core tests as today.
- Runtime evidence (D7 `--level 1/2/3` against the device) only on a real
  Pico 2 W – an opt-in step by the PO, as for the ESP32 and M5Stack today.
  Until then the port is "build-tested, best effort".

## 7. Effort and placement

- About one cycle: PAL with coreMQTT, record store, example firmware
  `examples/pico-sdk/` (Sensor), `build.sh`, host tests, `micro.yml` job,
  README section, Community page, SBOM entry (supply chain, C-108), ADR-0030
  amendment, ADR-0024 amendment (coreMQTT, Pico SDK). A second about half a
  cycle for the hardware run and fixes once a board is on the PO's desk.
- Arduino on the Pico 2 W (Earle Philhower's `arduino-pico` core) would need
  yet another PAL (no esp-mqtt there; ADR-0024 fallback arduino-mqtt) – not
  part of the proposal.
- All phases are closed (P9 after C-111); the first release waits for the
  PO's notice (C-110 D-2). A port is new scope after P9: either before the
  first release (delays it by about a cycle) or as the first increment after
  it (proposed), with the status "best effort" in the README until the
  hardware run.
