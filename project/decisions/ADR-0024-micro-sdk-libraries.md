# ADR-0024 – Libraries and toolchain of the micro SDK (D9)

- Status: **accepted** (PO, C-045 review 2026-10-26; proposed in C-043)
- Date: 2026-10-26
- Deciders: product owner (Thomas) – proposed by the agent in C-043
- Related: ADR-0016 (micro SDK, levels), ADR-0017 (C17, PAL, ESP32/ESP-IDF, Arduino, M5Stack; candidates), ADR-0022 (D9 API, PAL, Will per connect), ADR-0004 (MIT), GUARDRAILS G-3.4/G-4.4, Q-027

## PO decision (C-045 review, 2026-10-26)

Accepted as proposed. Q-027 (a): own bounded JSON pull reader in the core; cJSON is not used. ADR-0017 received a matching amendment (C-045).

## Context

ADR-0017 named the D9 candidates and left the final choice for P3/P4b:
- esp-mqtt and esp-tls/mbedTLS;
- a hand-written JSON encoder, decoding with cJSON ("bundled with ESP-IDF") or jsmn;
- Unity;
- CMake.

ADR-0022 fixed the API: no heap in the core, one task with `oecm_poll`, a PAL with TLS duty, and a new Will per connect. It asked C-043 to check that the Arduino MQTT client supports TLS and a Will per connect.

The agent checked the facts on 2026-10-26 (GitHub and the esp-mqtt sources):
- **ESP-IDF:** current releases are v6.1 (2026-08), v6.0.3 and v5.5.6. Since IDF 6 the components `mqtt` and `json` (cJSON) are no longer part of IDF; they are managed components (`espressif/mqtt`, `espressif/cjson`). `esp-tls`, `mbedtls` and `unity` stay in IDF.
- **esp-mqtt** (Apache-2.0, v1.1.0, 2026-07):
  - With `network.disable_auto_reconnect = true`, a broken connection moves the client to `WAIT_RECONNECT` and sends `MQTT_EVENT_DISCONNECTED`.
  - `esp_mqtt_set_config()` copies a new `session.last_will` (malloc/strdup).
  - `esp_mqtt_client_reconnect()` then connects with it; it is allowed only in `WAIT_RECONNECT`.
  - So a new Will per connect (REQ-LIFE-016) is possible without destroying the client. Checked in `mqtt_client.c` on `master`.
- **jsmn:** last commit 2021-10. Under G-4.4 ("no abandoned packages") it is out.
- **cJSON** (MIT, v1.7.19, 2025-09) allocates on the heap (hooks are possible) and is no longer bundled with IDF 6.
- **Arduino:**
  - Arduino-ESP32 (3.3.12; 4.0.0-RC1 since 2026-09) is built on ESP-IDF. Third-party libraries such as `ESP32MQTTClient` (MIT) include `<mqtt_client.h>` in Arduino CI for core v2/v3, so esp-mqtt is usable in Arduino sketches.
  - `arduino-mqtt` (256dpi, MIT, v2.5.3, 2026-04) supports `setWill()` before every `connect()` and TLS via any `Client` (`NetworkClientSecure`).
  - `PubSubClient` (MIT) had its last release in 2020 (v2.8).
  - `ArduinoMqttClient` is LGPL-2.1.

## Decision

| Role | Choice | License | Notes |
|---|---|---|---|
| Language/build | C17; **CMake** ≥ 3.16 for the host build and the ESP-IDF component | – | As ADR-0017. The core uses no CMake-only features (Arduino build, ADR-0017). |
| Platform SDK (reference port) | **ESP-IDF**, latest stable at the start of P4b (today v6.1). CI also builds against the IDF of the current Arduino-ESP32 release | Apache-2.0 | ESP32 and ESP32-S3 including M5Stack (ADR-0017). |
| MQTT (ESP-IDF port) | **esp-mqtt** (`espressif/mqtt` managed component from IDF 6; built-in in IDF 5.x), MQTT 3.1.1 | Apache-2.0 | `disable_auto_reconnect = true`. The core decides when to reconnect (backoff) and calls `pal.mqtt_connect(will)`; the port does `esp_mqtt_set_config` (new Will) + `esp_mqtt_client_reconnect`. The first connect uses `esp_mqtt_client_start`. Events from the MQTT task go through a FreeRTOS queue to the application task (ADR-0022). |
| TLS | **esp-tls / mbedTLS** (in IDF) | Apache-2.0 | TLS ≥ 1.2 only. CA from the device configuration, not from the certificate bundle. Client certificate or username/password (REQ-MQTT-005/008/009/010). |
| JSON writing | **own encoder** in the core | MIT (ours) | Deterministic, into the caller's transmit buffer (ADR-0017/0022). |
| JSON reading | **own bounded pull reader** in the core | MIT (ours) | No heap. Limits on depth, token length and number of members. Strings are unescaped into caller buffers. It reads only what the core needs (envelopes and requests). |
| Wall clock | SNTP (`esp_netif_sntp`) in the ESP-IDF port | Apache-2.0 | Required (ADR-0022). |
| Persistence | NVS (ESP-IDF) / Preferences (Arduino, on top of NVS) | Apache-2.0 / LGPL-2.1 (Arduino-ESP32 core, as a platform) | Behind the PAL. |
| Unit tests | **Unity** (v2.7) on the host via CMake/CTest; the same tests as an ESP-IDF test app | MIT | Host jobs with ASan/UBSan. A libFuzzer target for the JSON reader and the topic parser (clang, host). |
| Arduino port | Arduino library (`library.properties`, `src/`) generated from the core and the **ESP-IDF PAL**, which uses esp-mqtt from the Arduino-ESP32 core. **Fallback:** a PAL on **arduino-mqtt** (256dpi) + `NetworkClientSecure`, if a core release no longer ships esp-mqtt | MIT | No second MQTT client while esp-mqtt is available. Whether 4.x (IDF 6) still ships it is checked at the start of P4b. |
| M5Stack demo | **M5Unified** (MIT, active) only in the demo application, not in the core or PAL | MIT | Display and buttons for P5b. |

**Not chosen:**
- **cJSON:** heap (a static arena via hooks adds complexity); since IDF 6 an extra component; parsing a full tree costs RAM on L1 devices.
- **jsmn:** abandoned (G-4.4); it would also only yield tokens, so unescaping and number parsing would remain our code.
- **PubSubClient:** no release since 2020; QoS 0 publish only.
- **ArduinoMqttClient:** LGPL-2.1, a problem for statically linked firmware.
- **ESP-IDF certificate bundle as default trust:** OEC trust comes from the broker's CA (RIG 4.1.2), not from public roots.

**License note for firmware:**
- The core and PAL are MIT (ADR-0004); ESP-IDF, esp-mqtt and mbedTLS are Apache-2.0.
- The Arduino-ESP32 core is LGPL-2.1. That is the user's platform choice, as with every Arduino sketch; our library does not change it.
- The README of the Arduino library will say so (P4b).

## Consequences

- The core needs its own JSON reader. This is about 400–600 lines with tests, on top of the planned encoder. It is the attack surface for broker input, hence the fuzz target and the limits.
- P4b starts with a short check: does esp-mqtt behave as above in the IDF of the current Arduino-ESP32 release, and does Arduino-ESP32 4.x still ship it? If not, the arduino-mqtt fallback applies.
- The open choice for the PO is Q-027: an own JSON reader (recommended) or cJSON with a static arena.
