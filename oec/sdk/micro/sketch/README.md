# D9 API sketch (P3, C-042) – not product code

Compile-checked sketch of the micro SDK API (D9, C17, ESP32 reference port). The decisions are in ADR-0022; the comparison with the C ABI of D2 (`oec/sdk/rust/sketch/c/include/oec.h`) is in ADR-0021. This folder is replaced by the real core in P4b.

| Path | Content |
|---|---|
| `include/oecm.h` | API contract of increment 1 (Messaging Level 1) |
| `examples/hello_device.c` | device with static memory, a stub PAL and the `oecm_poll` loop |

Check: `oec/sdk/check.sh` (part of `oec/check.sh`) compiles both with `-std=c17 -Wall -Wextra -Wpedantic -Werror`.

## Flow of a Level 1 device

1. `oecm_init(&dev, &cfg, &pal, tx, sizeof tx)`:
   - validate the IL (REQ-IL-008, REQ-IL-016, REQ-IL-019), or build the Oi4Identifier from the MAM (REQ-IL-007);
   - check the buffer sizes.
2. `oecm_start(&dev)`:
   - needs a wall clock (SNTP);
   - builds a new Will (REQ-LIFE-008, REQ-LIFE-016) and calls `pal.mqtt_connect` (TLS, REQ-MQTT-005).
3. The PAL reports the connection with `oecm_on_connected`. The core publishes:
   - the Birth (MAM) first (REQ-LIFE-001, REQ-LIFE-015);
   - then the Health.
4. Loop `oecm_poll`:
   - Health heartbeat every 60 s (REQ-TOP-017);
   - `oecm_set_health` publishes on change.
5. `oecm_on_disconnected` → reconnect with backoff and a new Will on the next `oecm_poll`.
6. `oecm_stop`: Close (REQ-LIFE-005), disconnect.

## PAL duties (ESP-IDF reference port, P4b)

| PAL function | ESP-IDF | Note |
|---|---|---|
| `mqtt_connect` | `esp_mqtt_client_init` / `esp_mqtt_set_config` + `esp_mqtt_client_start` | TLS via esp-tls/mbedTLS with the broker CA; `disable_auto_reconnect = true`, because esp-mqtt would reuse the old Will |
| `mqtt_publish` | `esp_mqtt_client_publish` (QoS 0, retain 0) | REQ-MQTT-017, REQ-MQTT-018 |
| `mqtt_subscribe` | `esp_mqtt_client_subscribe` | Level 2+ |
| events | `esp_mqtt_client_register_event` → FreeRTOS queue → app task → `oecm_on_*` | single-threaded core |
| `unix_time_ms` | `gettimeofday` after `esp_sntp` sync | REQ-MSG-007 |
| `monotonic_ms` | `esp_timer_get_time() / 1000` | |
| `persist_*` | NVS | Level 3 |

## Level 2/3 preview (ADR-0022 §9)

Data definitions are static arrays of typed fields with caller-provided storage. `Get`, pagination, `Set` with the Status event and the lists are handled in the core. The application only updates values and answers `Set` with a status code, as in D2.
