---
title: OEC Micro SDK
sidebar_position: 4
description: The OEC Micro SDK (D9) makes a microcontroller an OEC smart device – a C17 core without heap, an ESP-IDF port for the ESP32 and an Arduino library, messaging levels 1 to 3.
---

# OEC Micro SDK

The OEC Micro SDK turns a microcontroller into a smart device on the OI4
Message Bus. A small C17 core speaks the messages of the Messaging
Guideline; a platform abstraction layer (PAL) connects it to the MQTT
client, the clock and the persistent memory of a platform. The reference
platform is the ESP32 with ESP-IDF; the same code is also an Arduino
library. The device connects over TLS to the broker of an OEC platform,
announces itself with its Master Asset Model (`MAM`) and keeps its `Health`
up to date – from Level 2 it also serves its Data and Config.

| | |
|---|---|
| ServiceType | `OTConnector` (a directly connected smart device) |
| Messaging level | 1, 2 or 3 (set by the application) |
| Language | C17, no heap, no stdio in the core |
| Platforms | ESP32 and ESP32-S3 with ESP-IDF 6.1; Arduino-ESP32 3.3 and later (also M5Stack Core2/CoreS3); a Go host runner for tests |
| MQTT | 3.1.1 over TLS 1.2+, username/password or client certificate |
| Size on the ESP32 | about 34–35 KB of flash for core and port, no static RAM |

## What a device does

- **Level 1** – the device connects with a Will (`FAILURE_1`), publishes
  its Birth (`MAM`) and `Health`, repeats the `Health` at least every 60 s,
  reconnects with backoff and sends the Close when it stops gracefully.
- **Level 2** – it answers `Get` of `MAM`, `Health`, `Profile`, Data,
  Metadata, Config and the publication and subscription lists (paginated
  when needed), accepts `Set` of writable Data and of Config values, and
  publishes its Data on change or periodically. Every `Set` is answered with
  a Status event.
- **Level 3** – it adds its `ReferenceDesignation` (`Get`/`Set`/`Del`,
  persisted) and the method `NewDataSetWriterId`.

The application describes its MAM, Data and Config statically; the core
decides every request. The application calls the core from one task; the
PAL hands the MQTT events over to it.

## Start

### On the host, against the dev environment

The host runner links the core with a PAL in Go and reads the same
container storage as the other OEC applications. With the broker of
`oec/dev` running:

```sh
cd oec/sdk/micro/host
OEC_ROOT=../../../dev/generated/roots/oec-dev-app go run ./cmd/oecm-host -level 3
```

At Level 2 and 3 it runs the simulated temperature sensor of the examples:
Data `Temperature` with a writable `Setpoint` and the Config context
`Sensor`. The OEC Registry and the config service of the
[demos](../demos/index.md) see it like any other participant.

### On an ESP32 with ESP-IDF

`examples/esp-idf/` is the same sensor as firmware. Build it with ESP-IDF
6.1 for `esp32` or `esp32s3`, write the settings (broker, CA, credentials,
Wi-Fi, level) into the NVS partition `oecprov` and flash both. Nothing of a
site is compiled in.

```sh
. $IDF_PATH/export.sh
oec/sdk/micro/examples/esp-idf/build.sh esp32
```

The serial console accepts `stop` (graceful Close) and `time <unix ms>`.

### In QEMU, without hardware

The `qemu` variant of the firmware runs in the Espressif QEMU with
Ethernet. The QEMU runner `oecm-qemu` starts it with the settings of a
container storage, like the host runner – this is how the project checks
the firmware without a board.

### With Arduino

`oec/sdk/micro/arduino/package.sh` assembles the library `OECMicro` (core
and ESP-IDF port) as a folder and a zip for *Add .ZIP Library*. It has two
examples:

- `Sensor` – the sensor on any ESP32 with Wi-Fi;
- `M5Stack` – the same device on an M5Stack Core2 or CoreS3 with a status
  display (Identification Link, connection, Health, Value/Setpoint); button
  A or a tap cycles the Health. It needs M5Unified.

The settings are in `arduino_secrets.h` next to the sketch. The Arduino-ESP32
core brings esp-mqtt with it; no other MQTT library is needed.

## Other platforms

A port implements the PAL of `oecm.h`: connect (with the Will of the
core), publish, subscribe, disconnect, wall clock, monotonic clock and – for
Level 3 – loading and saving two small records. The ESP-IDF port
(`oecm_esp.c`) and the Go PAL of the host runner are the examples.

## Checks

The gate of the project builds the core with warnings as errors (clang and
GCC), runs its unit tests and the fuzz corpora under AddressSanitizer and
UBSan, checks that the core uses no library function beyond a short
allowlist, validates the emitted messages with `oec-check validate` and runs
the host runner through `oec-check test` at Level 1, 2 and 3. Opt-in, the
firmware passes the same `oec-check test` runs in QEMU and against the broker
of `oec/dev`; the Arduino examples are built with `arduino-cli`.

## Limits

- **Tested without hardware.** The firmware is checked in QEMU and the
  Arduino sketches are only compiled; a run on a real ESP32 board and on an
  M5Stack is still open.
- **ESP32 family only** as a ready port. Other microcontrollers need their
  own PAL.
- **TLS only** (`mqtts://`); MQTT 5 is not used.
- The core does not persist Config values – the application does it if it
  needs to.
- Compared with the Rust SDK: no Validation `Pattern`, no Syslog; a `Set`
  with several violations reports the first one.
- Not yet published to the Arduino Library Manager or the ESP Component
  Registry.

## Details

- The SDK – API, behavior per level, sizes, host runner, ESP-IDF port,
  provisioning, QEMU runner, checks:
  [`oec/sdk/micro/README.md`](../../../sdk/micro/README.md)
- The Arduino library – install, examples, provisioning, licenses:
  [`oec/sdk/micro/arduino/README.md`](../../../sdk/micro/arduino/README.md)
- The dev environment with broker and PKI:
  [`oec/dev/README.md`](../../../dev/README.md)
