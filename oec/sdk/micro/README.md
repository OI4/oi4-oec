# OEC micro SDK (D9)

C17 SDK for smart devices on microcontrollers: a platform-independent core
and a platform abstraction layer (PAL), the reference port for ESP32 on
ESP-IDF. Decisions: API in ADR-0022 (with the amendments), implementation in
ADR-0030, ESP32 reference platform in ADR-0024.

State (C-094): **Messaging Level 2** in the core (Level 1 since C-092), host
tests, payload check and fuzz targets; the Go host runner runs the core under
D7 `--level 1` and `--level 2` in the gate. Level 3, the ESP-IDF port and the
Arduino library follow in C-095 … C-097 (ADR-0030 §10).

| Path | Content |
|---|---|
| `include/oecm.h` | the API (the only public header) |
| `src/` | the core: Identification Link, base64url, dnp-encoding, Oi4Identifier (`oecm_il.c`), output buffer and JSON writer (`oecm_buf.c`), messages and lifecycle (`oecm_device.c`); Level 2: numbers, time, SHA-1/UUIDv5 (`oecm_num.c`), JSON pull reader (`oecm_json.c`), topic parser (`oecm_topic.c`), resources and pagination (`oecm_res.c`), requests (`oecm_req.c`) |
| `tests/` | Unity tests, the recording PAL `fake_pal.c`, the payload emitter `emit.c`, the generated vector header `vectors_gen.h` |
| `fuzz/` | libFuzzer targets `fuzz_json.c`, `fuzz_topic.c`, `fuzz_message.c`, the corpus replayer `replay.c`, seed corpora, `run.sh` |
| `examples/host/hello_device.c` | a Level 1 device with a stub PAL that prints, the shape of the firmware |
| `host/` | Go module (standard library only): `cmd/oecm-vecgen` generates `tests/vectors_gen.h` from `oec/spec/export/vectors`; `cmd/oecm-host` is the host runner (the core with a PAL in Go over cgo), `mqtt/` its MQTT 3.1.1 client, `internal/d7run` the D7 harness |
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
   Identification Link unless given (REQ-MQTT-013). Level 3 returns
   `OECM_E_LEVEL` until it is implemented (C-095).
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

## Level 2 behavior

The application describes its Data DataSets (`oecm_data`: Tag, typed fields
pointing to its own variables, unit, EURange, decimals, interval, writable)
and its Config contexts (`oecm_cfg_context` → groups → objects with a text
buffer, Type, Validation Length/Min/Max/Values, mandatory, sensitive) in
`oecm_config`. The core decides every request as the Rust provider does
(`oec-core`):

- after CONNACK it subscribes to its `Get`/`Set`/`Del` topics, then sends
  Birth and Health, then Profile, PublicationList, SubscriptionList, Config,
  Metadata and Data (the order of `oec-core` `provider.rs`);
- `Get` of MAM, Health, Profile, Data, Metadata, Config, PublicationList and
  SubscriptionList, paginated when the answer does not fit the transmit
  buffer or `MaxPacketSize` (REQ-PAG-002 … 008), with `PerPage`/`Page` of a
  PaginationRequest;
- `Set Data` (writable DataSets, every field), `Set Config` (the whole `Set`
  is validated before any buffer is written; sensitive values are published
  masked), `Set PublicationList`, `Set`/`Del SubscriptionList`, each answered
  with a Status event `Pub/Event/<AppId>/Status/Good|Bad`;
- after an accepted `Set` the core calls `on_data`/`on_config`; the
  application calls `oecm_data_changed` when it changed a value itself;
  DataSets with `interval_ms` are published periodically.

DataSetWriterIds: MAM 10, Health 11, Profile 12, PublicationList 13, Status
14/15/16 (Good/Uncertain/Bad), SubscriptionList 17, then the Config contexts
and the Data DataSets from 18 in definition order (Metadata uses the id of its
Data). Differences to the Rust SDK (ADR-0030 amendment C-094): the first
violation in document order, no Validation `Pattern`, no Syslog, `double`
output not correctly rounded beyond 15 … 17 significant digits.

## Sizes

`OECM_DEVICE_SIZE` is 1024 octets; Level 1 uses 368 and Level 2 568 on a
64-bit host (544 on Cortex-M4). The Level 2 core compiled for Cortex-M4 with
`-Os` has about 35 KB of code and constants, no data and no bss.
`OECM_TX_MIN` is 2048 octets: enough for every Health, Will and Close, also
for an Identification Link of 255 octets (1804 octets). The Birth grows with
the MAM texts; a typical Birth needs about 1000 octets, a 255-octet
Identification Link with empty texts 2315. A Level 2 device needs a larger
transmit buffer for its Config and Metadata answers (the host demo uses 8192);
larger answers are paginated.

## Host runner

`host/cmd/oecm-host` links the core with cgo and implements the PAL in Go
(`crypto/tls`, TLS 1.2+, and a minimal MQTT 3.1.1 client). It is a test
harness for D7, not a product (ADR-0030 §7, amendment C-093). It reads a
container storage below `OEC_ROOT` (default `/`, RIG 4.1):

| File | Use |
|---|---|
| `proc/sys/kernel/hostname`, else `etc/hostname` | default SerialNumber, name of the client certificate |
| `etc/oi4/mqtt/broker.json` | Address, SecurePort, MaxPacketSize (all required) |
| `etc/oi4/mqtt/broker_ca.pem`, else `etc/oi4/certs/ca.pem` | CA of the broker certificate |
| `run/secrets/oi4/private_key.pem` + `etc/oi4/certs/<hostname>.pem` | client certificate (used when present; no passphrase) |
| `run/secrets/oi4/credentials` | `base64(user:password)` otherwise |

Flags: `-level` (1; 2 runs the simulated temperature sensor of `demo.c`), `-il`, `-manufacturer`, `-manufacturer-uri`, `-model`,
`-product-code`, `-serial`, `-software-revision`, `-client-id`. SIGTERM or
SIGINT stops it gracefully (Close message). Against the dev broker
(`oec/dev`, broker running):

```sh
cd oec/sdk/micro/host
OEC_ROOT=../../../dev/generated/roots/oec-dev-app go run ./cmd/oecm-host
```

## Check

`./check.sh` (step 9 of `oec/check.sh`, about 4 min): host Go module with
the MQTT client under `-race` and D7 `oec-check test --level 1` (21 pinned
passes) and `--level 2` (the demo sensor, 38 pinned passes) against the host
runner on the Rust SDK's `go-testbroker` (about 3 min), vector header
freshness, CMake build in `target/host`, the Unity tests and the replay of
the fuzz seed corpora under AddressSanitizer + UBSan, `clang --analyze`, the
symbol allowlist, GCC 14/15 if installed, the emitted payloads validated by
D7 `oec-check validate`, the host example. Needs `cmake` and `cargo`.

## Fuzzing

`fuzz/run.sh [SECONDS] [TARGET...]` (opt-in, default 60 s each for `json`,
`topic`, `message`) builds the targets with a clang that has libFuzzer
(Homebrew LLVM first), fuzzes a copy of the seed corpus in
`target/fuzz/<target>` and replays the grown corpus under ASan + UBSan.
On macOS the fuzzing itself runs with UBSan only, because the ASan runtime
of Homebrew LLVM 21 hangs at start-up on macOS 27; `OECM_FUZZ_SAN` overrides
the sanitizers. Findings are written as `target/fuzz/<target>-crash-*`; add
a minimized finding to `fuzz/corpus/<target>/` together with the fix.

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
