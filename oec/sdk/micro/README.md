# OEC micro SDK (D9)

C17 SDK for smart devices on microcontrollers: a platform-independent core
and a platform abstraction layer (PAL), the reference port for ESP32 on
ESP-IDF. Decisions: API in ADR-0022 (with the amendments), implementation in
ADR-0030, ESP32 reference platform in ADR-0024.

State (C-092): **Messaging Level 1** in the core, host tests and payload
check. Levels 2 and 3, the host runner for D7, the ESP-IDF port and the
Arduino library follow in C-093 … C-097 (ADR-0030 §10).

| Path | Content |
|---|---|
| `include/oecm.h` | the API (the only public header) |
| `src/` | the core: Identification Link, base64url, dnp-encoding, Oi4Identifier (`oecm_il.c`), output buffer and JSON writer (`oecm_buf.c`), messages and lifecycle (`oecm_device.c`) |
| `tests/` | Unity tests, the recording PAL `fake_pal.c`, the payload emitter `emit.c`, the generated vector header `vectors_gen.h` |
| `examples/host/hello_device.c` | a Level 1 device with a stub PAL that prints, the shape of the firmware |
| `host/` | Go module (standard library only): `cmd/oecm-vecgen` generates `tests/vectors_gen.h` from `oec/spec/export/vectors` |
| `third_party/unity/` | Unity v2.7.0 (MIT), unchanged, test only |

## Rules of the core

- C17, no heap, no stdio, no global state: all state is in the caller's
  `oecm_device` (static assertion on size and alignment) and the transmit
  buffer. Allowed library symbols: `memcmp memcpy memmove memset strchr
  strlen` (checked with `nm`).
- Single-threaded: all `oecm_*` calls come from one task; the PAL hands
  network events over (FreeRTOS queue on ESP-IDF).
- No float, no `printf`: numbers are formatted by the core.
- Warnings are errors (`-Wall -Wextra -Wpedantic -Wconversion
  -Wsign-conversion -Wshadow -Wcast-qual -Wstrict-prototypes
  -Wmissing-prototypes -Wvla -Wformat=2`), with clang and GCC.

## Level 1 behavior

Same as the Rust SDK (`oec-core` `lifecycle.rs`):

1. `oecm_init` validates the configuration: UTF-8 of every text, the
   ManufacturerUri (REQ-RES-MAM-004), the given Identification Link
   (REQ-IL-008, REQ-IL-016, REQ-IL-019) or the Oi4Identifier built from the
   MAM (REQ-IL-007); it encodes the Birth once with the longest MessageId
   and fails with `OECM_E_NOSPACE` if the transmit buffer is too small.
   The ClientId is `oec` + 16 hex digits of the FNV-1a hash of the
   Identification Link unless given (REQ-MQTT-013). Levels 2 and 3 return
   `OECM_E_LEVEL` until they are implemented.
2. `oecm_start` needs a set wall clock (after 2020-01-01, e.g. SNTP) and
   connects with a new Will (Health `FAILURE_1`, score 0; REQ-LIFE-008,
   REQ-LIFE-016).
3. `oecm_on_connected`: the Birth (MAM, DataSetWriterId 10) first, then the
   Health (DataSetWriterId 11) (REQ-LIFE-001, REQ-LIFE-015).
4. `oecm_poll`: the Health on change and every 60 s (REQ-TOP-017); after an
   own `FAILURE_1`/0 the next other Health is preceded by a Birth
   (REQ-LIFE-017). A failed publication stays due.
5. `oecm_on_disconnected`: reconnect after 1 s, doubling up to 30 s, each
   with a new Will; a connect without answer is given up after 30 s.
6. `oecm_stop`: the Close (Health `NORMAL_0`, score 0; REQ-LIFE-005), then
   DISCONNECT. `NORMAL_0`/0 is reserved for the Close: `oecm_set_health`
   rejects it.

Every message has a MessageId of the Unix time in ms, with a counter while
the clock does not advance (REQ-MSG-007, REQ-MSG-008).

## Sizes

`OECM_DEVICE_SIZE` is 1024 octets; Level 1 uses 368 on a 64-bit host.
`OECM_TX_MIN` is 2048 octets: enough for every Health, Will and Close, also
for an Identification Link of 255 octets (1804 octets). The Birth grows with
the MAM texts; a typical Birth needs about 1000 octets, a 255-octet
Identification Link with empty texts 2315.

## Check

`./check.sh` (step 9 of `oec/check.sh`, about 6 s): host Go module, vector
header freshness, CMake build in `target/host`, the Unity tests under
AddressSanitizer + UBSan, `clang --analyze`, the symbol allowlist, GCC 14/15
if installed, the emitted payloads validated by D7 `oec-check validate`, the
host example. Needs `cmake`.

Regenerate the vectors after a change of `oec/spec/export/vectors`:

```sh
cd oec/sdk/micro/host && go run ./cmd/oecm-vecgen
```

## PAL duties of the ESP-IDF port (C-096)

| PAL function | ESP-IDF | Note |
|---|---|---|
| `mqtt_connect` | `esp_mqtt_client_init` / `esp_mqtt_set_config` + `esp_mqtt_client_start` | TLS via esp-tls/mbedTLS with the broker CA; `disable_auto_reconnect = true`, because esp-mqtt would reuse the old Will |
| `mqtt_publish` | `esp_mqtt_client_publish` (QoS 0, retain 0) | REQ-MQTT-017, REQ-MQTT-018 |
| `mqtt_subscribe` | `esp_mqtt_client_subscribe` | Level 2+ |
| events | `esp_mqtt_client_register_event` → FreeRTOS queue → app task → `oecm_on_*` | single-threaded core |
| `unix_time_ms` | `gettimeofday` after `esp_sntp` sync | REQ-MSG-007 |
| `monotonic_ms` | `esp_timer_get_time() / 1000` | |
| `persist_*` | NVS | Level 3 |
