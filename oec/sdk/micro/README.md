# OEC micro SDK (D9)

C17 SDK for smart devices on microcontrollers: a platform-independent core
and a platform abstraction layer (PAL), the reference port for ESP32 on
ESP-IDF. Decisions: API in ADR-0022 (with the amendments), implementation in
ADR-0030, ESP32 reference platform in ADR-0024. Guide:
[OEC Micro SDK](https://oi4.github.io/oi4-oec-documentation/docs/community/micro-sdk) in the Community section of the
guideline site.

State (C-097): **Messaging Level 3** in the core (Level 1 since C-092,
Level 2 since C-094), host tests, payload check and fuzz targets; the Go host
runner runs the core under D7 `--level 1`, `--level 2` and `--level 3` in the
gate. The ESP-IDF port and the example firmware (C-096) build for `esp32`,
`esp32s3` and the Espressif QEMU, where the firmware passes the same D7 runs
(opt-in). The Arduino library `OECMicro` (C-097) packages the core and the
ESP-IDF port with the examples `Sensor` and `M5Stack`. Not yet run on real
hardware.

| Path | Content |
|---|---|
| `include/oecm.h` | the API (the only public header) |
| `src/` | the core: Identification Link, base64url, dnp-encoding, Oi4Identifier (`oecm_il.c`), output buffer and JSON writer (`oecm_buf.c`), messages and lifecycle (`oecm_device.c`); Level 2: numbers, time, SHA-1/UUIDv5 (`oecm_num.c`), JSON pull reader (`oecm_json.c`), topic parser (`oecm_topic.c`), resources and pagination (`oecm_res.c`), requests (`oecm_req.c`); Level 3: ReferenceDesignation and `NewDataSetWriterId` (`oecm_l3.c`) |
| `tests/` | Unity tests, the recording PAL `fake_pal.c`, the payload emitter `emit.c`, the generated vector header `vectors_gen.h` |
| `fuzz/` | libFuzzer targets `fuzz_json.c`, `fuzz_topic.c`, `fuzz_message.c`, the corpus replayer `replay.c`, seed corpora, `run.sh` |
| `examples/host/hello_device.c` | a Level 1 device with a stub PAL that prints, the shape of the firmware |
| `examples/sensor/` | the simulated temperature sensor (Data, Config) of the host runner and the firmware |
| `examples/esp-idf/` | the example firmware (ESP-IDF project), `build.sh` |
| `port/esp-idf/oecm/` | the ESP-IDF component `oecm`: the core + the PAL `oecm_esp.c` |
| `arduino/` | the Arduino library `OECMicro`: `library.properties`, `OECMicro.h`, the sketches `examples/Sensor` and `examples/M5Stack`, `package.sh`, its [README](arduino/README.md) |
| `host/` | Go module (standard library only): `cmd/oecm-vecgen` generates `tests/vectors_gen.h` from `oec/spec/export/vectors`; `cmd/oecm-host` is the host runner (the core with a PAL in Go over cgo), `mqtt/` its MQTT 3.1.1 client; `cmd/oecm-qemu` runs the firmware in QEMU; `internal/d7run` the D7 harness and the pinned passes |
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
   Identification Link unless given (REQ-MQTT-013). Level 3 needs
   `persist_load`/`persist_save` and a ReferenceDesignation buffer
   (`OECM_E_ARG` otherwise, see below).
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

`oecm_connected` is true from `oecm_on_connected` until the disconnect or
`oecm_stop` (for a status display, C-097).

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

## Level 3 behavior

Level 3 adds the resource and the method of REQ-CMP-010/033 that a device
serves beyond Level 2 (as `oec-core` `provider/refdes.rs` and
`provider/actions.rs`):

- **ReferenceDesignation** of the device (MG 8.3.10): `Get` (with a Query
  DataSetMessage, id 9, for the direct children of a `Parent`), `Set` and
  `Del`, each `Set`/`Del` answered with a Status event; an accepted change
  is published with `ReplyTo` and persisted. The object is kept in the
  application's buffer `refdes_buf` (at least `OECM_REFDES_MIN` 128 octets)
  in its canonical form; a larger object than fits the buffer or half the
  transmit buffer is answered with `BadOutOfRange`. As in Rust it is
  published on change, not at connect. Entry in the PublicationList:
  `ON_REQUEST_SOURCE_10`.
- **`Call NewDataSetWriterId`** (MG 9.2.1): `Reply` with new ids above the
  own definitions (Ttl 60 s), `CorrelationData` echoed, requests for another
  Source answered with `BadNotFound`, at most 256 new ids per 60 s
  (`BadResourceUnavailable`); the ids are reserved in blocks of 256 in the
  persisted record and are not reused after a restart; replies that do not
  fit one message are split.
- Persistence over the PAL (`persist_load`/`persist_save`, small records):
  `oecm.refdes` (the ReferenceDesignation) and `oecm.wid` (the reserved
  DataSetWriterIds). `oecm_init` loads both; a load error is `OECM_E_PAL`, a
  stored record that is not valid is ignored. Config values live in the
  application, which persists them itself in `on_config` if it needs to.
- Requests to the ReferenceDesignation and `Call` topics with a Filter level
  are ignored (Rust: Filter forbidden).

The Profile lists `ReferenceDesignation` and `NewDataSetWriterId`, the
SubscriptionList adds `Call/NewDataSetWriterId`. The DataSetWriterId of the
ReferenceDesignation follows the Config contexts and Data DataSets.

## Sizes

`OECM_DEVICE_SIZE` is 1024 octets; the state of Level 1 … 3 uses 600 on a
64-bit host (568 on Cortex-M4). The Level 3 core compiled for Cortex-M4 with
`-Os` has about 42 KB of code and constants (6.3 KB of it Level 3), no data
and no bss.
`OECM_TX_MIN` is 2048 octets: enough for every Health, Will and Close, also
for an Identification Link of 255 octets (1804 octets). The Birth grows with
the MAM texts; a typical Birth needs about 1000 octets, a 255-octet
Identification Link with empty texts 2315. A Level 2 device needs a larger
transmit buffer for its Config and Metadata answers (the host demo uses 8192);
larger answers are paginated.

On the ESP32 (ESP-IDF v6.1, `-Os`, example firmware at Level 3, C-096):

| | `esp32` (Wi-Fi) | `esp32s3` (Wi-Fi) | `qemu` (Ethernet) |
|---|---|---|---|
| app image | 884 KB | 879 KB | 536 KB |
| `liboecm.a` (core + PAL, flash) | 33.6 KB | 33.7 KB | 33.0 KB |
| esp-mqtt / mbedTLS (flash) | 11.2 / 24.3 KB | 11.8 / 24.2 KB | 10.8 / 24.3 KB |
| static RAM (DRAM / IRAM) | 49.9 / 84.4 KB | 110.7 KB DIRAM | 29.4 / 45.6 KB |

The core and the PAL have no static data; the firmware holds the device
state (1024), the transmit buffer (8192) and the ReferenceDesignation buffer
(2048) statically, the PAL allocates its receive buffer (8192) and a ring
buffer of twice that size once at `oecm_esp_init`. In QEMU at Level 3 the
lowest free heap was about 167 KB after the D7 runs (TLS sessions included)
and the 8 KB main task used at most about 2.4 KB of its stack.

With Arduino (`arduino-cli` 1.5.1, Level 3 sketches, C-097) the core and the
PAL take 34.8 … 35.4 KB of flash in every build (the linked objects; string
constants merged by the linker are counted with the sketch) and no static
RAM:

| Sketch, board | Arduino-ESP32 3.3.12 (IDF 5.5) | 4.0.0-RC1 (IDF 6.1) |
|---|---|---|
| `Sensor`, ESP32 Dev Module | 1054 KB, globals 59.3 KB | 1123 KB, globals 61.8 KB |
| `Sensor`, ESP32-S3 Dev Module | 1019 KB, globals 58.0 KB | 1089 KB, globals 59.0 KB |
| `M5Stack`, M5Core2 | 1241 KB, globals 63.5 KB | 1309 KB, globals 65.5 KB |
| `M5Stack`, M5CoreS3 | 1213 KB, globals 61.8 KB | 1285 KB, globals 63.1 KB |
| `M5Stack`, M5Core | 1229 KB (93 %), globals 63.4 KB | 1297 KB (98 %), globals 65.4 KB |

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
| `var/lib/oecm-host/` (or `-state`) | Level 3: one file per persisted record (`oecm.refdes`, `oecm.wid`) |

Flags: `-level` (1; 2 runs the simulated temperature sensor of `examples/sensor`; 3
adds the ReferenceDesignation and `NewDataSetWriterId`), `-state`, `-il`, `-manufacturer`, `-manufacturer-uri`, `-model`,
`-product-code`, `-serial`, `-software-revision`, `-client-id`. SIGTERM or
SIGINT stops it gracefully (Close message). Against the dev broker
(`oec/dev`, broker running):

```sh
cd oec/sdk/micro/host
OEC_ROOT=../../../dev/generated/roots/oec-dev-app go run ./cmd/oecm-host
```

## Check

`./check.sh` (step 9 of `oec/check.sh`, about 5 min): host Go module with
the MQTT client under `-race` and D7 `oec-check test --level 1` (21 pinned
passes), `--level 2` (the demo sensor, 38 pinned passes) and `--level 3`
(with a stored ReferenceDesignation, 50 pinned passes) against the host
runner on the Rust SDK's `go-testbroker`, vector header
freshness, CMake build in `target/host`, the Unity tests and the replay of
the fuzz seed corpora under AddressSanitizer + UBSan, `clang --analyze`, the
symbol allowlist, GCC 14/15 if installed (in the CI GCC 14 on Linux amd64
and arm64 – C-104 addendum 3), the emitted payloads validated by
D7 `oec-check validate`, the host example, `arduino/package.sh` into a
temporary directory (the core and the port packaged unchanged, every example
complete, the placeholders of `arduino_secrets.h` empty) and `oecm.h` and
`oecm_sensor.h` as C++17. Needs `cmake` and `cargo`.

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

## ESP-IDF port (C-096)

`port/esp-idf/oecm/` is the ESP-IDF component `oecm` (ESP-IDF ≥ 6.0,
reference v6.1; ADR-0024): the core of `src/` and the PAL
`oecm_esp.c` (`include/oecm_esp.h`). It depends on `espressif/mqtt` (esp-mqtt
1.1.0, Apache-2.0) from the ESP Component Registry; TLS 1.2+ via
esp-tls/mbedTLS with the broker CA only (no certificate bundle).

| PAL function | ESP-IDF |
|---|---|
| `mqtt_connect` | `esp_mqtt_client_init` once, then `esp_mqtt_set_config` with the new Will before every connect (`disable_auto_reconnect`, REQ-LIFE-008/016) + `esp_mqtt_client_reconnect`, or `esp_mqtt_client_start` after a stop |
| `mqtt_publish`, `mqtt_subscribe` | `esp_mqtt_client_publish` (QoS 0, retain 0), `esp_mqtt_client_subscribe_single` (QoS 0) |
| `mqtt_disconnect` | `esp_mqtt_client_stop` (DISCONNECT, no Will) |
| events | esp-mqtt task → FreeRTOS ring buffer (fragmented messages reassembled up to `rx_size`, default 8192) → `oecm_esp_dispatch` on the application task → `oecm_on_*`; events of an earlier connection attempt are dropped |
| `unix_time_ms`, `monotonic_ms` | `gettimeofday` (SNTP is the application's duty), `esp_timer_get_time` |
| `persist_load`, `persist_save` | NVS blobs in the namespace `oecm` of the default `nvs` partition |

```c
oecm_esp_config pc = {.uri = "mqtts://broker:8883", .ca_pem = ca, .username = u, .password = p};
oecm_esp *port; oecm_pal pal;
oecm_esp_init(&pc, &port, &pal);         /* after nvs_flash_init, network and SNTP */
oecm_init(&dev, &cfg, &pal, tx, sizeof tx);
oecm_start(&dev);
for (;;) {
    oecm_esp_dispatch(port, &dev, oecm_poll_interval_ms(&dev));
    oecm_poll(&dev);
}
```

### Example firmware

`examples/esp-idf/` is the simulated temperature sensor of `examples/sensor`
(the same code as the host runner's) as a smart device, Level 3 by default
(Kconfig `OECM_EXAMPLE_LEVEL`). `examples/esp-idf/build.sh [esp32|esp32s3|qemu]`
(opt-in, after `. $IDF_PATH/export.sh`) builds into `target/esp-idf/<variant>/`
and prints the sizes; `qemu` is an `esp32` with OpenCores Ethernet for the
Espressif QEMU (`sdkconfig.qemu`), the others use Wi-Fi.

Nothing of a site is compiled in. The firmware reads its settings from the NVS
partition `oecprov` (`partitions.csv`), namespace `oec`:

| Key | Type | Content |
|---|---|---|
| `uri` | string | `mqtts://host:port` of the broker (TLS only) |
| `ca` | string | PEM of the CA of the broker certificate |
| `user`, `password` | string | credentials, or: |
| `cert`, `key` | string | PEM client certificate and key (used when present) |
| `serial` | string | MAM SerialNumber; default: the base MAC address |
| `level` | u8 | 1 … 3; default Kconfig |
| `max_packet` | u32 | MaxPacketSize of the broker; default 262144 |
| `ntp` | string | SNTP server; default `pool.ntp.org` |
| `wifi_ssid`, `wifi_pass` | string | Wi-Fi station (not in the `qemu` variant) |

Write a CSV in the format of the ESP-IDF NVS partition generator (strings as
`file,string,<path>`), generate the image and flash it at the offset of
`oecprov` (0x10000):

```sh
python -m esp_idf_nvs_partition_gen generate oecprov.csv oecprov.bin 0x6000
python -m esptool --chip esp32 write-flash 0x10000 oecprov.bin
```

Keep the CSV and the image outside the repository (credentials, G-4.5). The
serial console accepts `stop` (Close message, DISCONNECT, prints
`oecm: stopped`) and `time <unix ms>` (sets the clock without SNTP). The
firmware logs the free heap and the unused stack of its task every 60 s.

### QEMU runner

`host/cmd/oecm-qemu` runs the `qemu` build in the Espressif QEMU
(`idf_tools.py install qemu-xtensa`; the `qemu-system-xtensa` of Homebrew has
no `esp32` machine) like `oecm-host` runs the core: it reads the same
container storage below `OEC_ROOT`, writes it into `oecprov` on every start
(a loopback broker address becomes 10.0.2.2, the host in the QEMU user
network), keeps the flash image in `OEC_ROOT/var/lib/oecm-qemu/flash.bin`
(seeded with the files `oecm.refdes`/`oecm.wid` found there when it is
created), sets the clock over the console and turns SIGTERM into `stop`.

```sh
cd oec/sdk/micro/host
OEC_ROOT=… go run ./cmd/oecm-qemu -build ../target/esp-idf/qemu \
    -qemu ~/.espressif/tools/qemu-xtensa/<version>/qemu/bin/qemu-system-xtensa -level 3
```

D7 against the firmware in QEMU (opt-in, about 7 min for the three levels;
the same pinned passes as the host runner, `internal/d7run`):

```sh
OECM_QEMU_BUILD=$PWD/../target/esp-idf/qemu OECM_QEMU=…/qemu-system-xtensa \
OEC_D7=<oec-check> go test -run 'TestQEMUD7' -v ./cmd/oecm-qemu/
```

Without `OECM_QEMU_BUILD` and `OECM_QEMU` these tests are skipped, also in
the gate.

### Run on real hardware (opt-in)

Not yet done in the project (no board in the sessions; P4b exit). Against
the broker of `oec/dev`, from a Mac or PC in the same network as the board
(`<ip>` = its LAN address):

1. Broker reachable from the network: in `oec/dev`,
   `go run ./pki -force -san <ip>` and
   `OEC_DEV_BIND=<ip> docker compose up -d --wait` (broker only).
2. Firmware: `examples/esp-idf/build.sh esp32` (or `esp32s3`), then in
   `target/esp-idf/esp32/` `python -m esptool --chip esp32 -p <port>
   write-flash @flash_args`.
3. Provisioning (above): `uri` `mqtts://<ip>:8883`, `ca` =
   `oec/dev/generated/broker/ca.pem`, `user`/`password` of
   `generated/roots/oec-dev-app/run/secrets/oi4/credentials` (base64 of
   `user:password`), `wifi_ssid`/`wifi_pass`; flash `oecprov`. The serial
   log (`python -m esp_idf_monitor -p <port>`) shows the Identification
   Link.
4. D7 from the same machine, with the probe user (the dev ACL lets only it
   send `Set`/`Del`/`Call`):

   ```sh
   oec-check test --broker mqtts://localhost:8883 --ca generated/broker/ca.pem \
       --username oec-probe --password-file generated/probe/password \
       --app <Identification Link> --level 3
   ```

   Without the proxy and a restart hook the connection and restart tests
   are skipped (the C-096 run of the QEMU firmware on `oec/dev`: 38 pass,
   0 fail, 17 skip). For them, point `uri` at D7's `--proxy-listen` address
   (with `--proxy-name <ip>`) and give `--stop-cmd`/`--start-cmd` scripts
   that send `stop` over the serial port and reset the board
   (`python -m esptool -p <port> run`).
5. Arduino: the same values in `arduino_secrets.h` of `Sensor` or
   `M5Stack`, `arduino-cli upload -p <port> --fqbn esp32:esp32:m5stack_core2
   …`, then step 4; on the M5Stack the display shows the connection and
   button A changes the Health that D7 `monitor` and the OEC Registry see.

## Arduino library (C-097)

`arduino/` is the SDK as the Arduino library `OECMicro` (ADR-0024): the core
of `src/` and the ESP-IDF port `oecm_esp.c` unchanged, on the esp-mqtt and NVS
that the Arduino-ESP32 core (3.3 and later) brings with it – no other MQTT
library. An Arduino library cannot refer to files outside its folder, so
`arduino/package.sh [out]` assembles it into `target/arduino/OECMicro/` (and
`OECMicro-<version>.zip`); each example gets `examples/sensor`.

| Example | Board | |
|---|---|---|
| `Sensor` | any ESP32 with Wi-Fi | the firmware of `examples/esp-idf` as a sketch; serial monitor `stop` |
| `M5Stack` | M5Stack Core2, CoreS3 (M5Unified, MIT) | the same device with a status display (Identification Link, `oecm_connected`, Health, Value/Setpoint); button A or a tap cycles the Health through `NORMAL_0`, `MAINTENANCE_REQUIRED_4`, `OFF_SPEC_3`, `CHECK_FUNCTION_2`, `FAILURE_1` |

The settings are compiled in from `arduino_secrets.h` next to the sketch
(placeholders in the repository, never real values); the persisted records of
Level 3 are in the NVS namespace `oecm`. Install, provisioning and licenses:
[`arduino/README.md`](arduino/README.md). Build check (opt-in, with
`arduino-cli`, an own config file keeps the core out of the default
location):

```sh
arduino/package.sh
arduino-cli core install esp32:esp32        # with the Espressif board index
arduino-cli lib install M5Unified
arduino-cli compile --fqbn esp32:esp32:esp32 --warnings all \
    --library target/arduino/OECMicro target/arduino/OECMicro/examples/Sensor
```

The Arduino-ESP32 core has no OpenCores Ethernet, so the sketches do not run
in QEMU; the code they run is the one the QEMU runner checks.
