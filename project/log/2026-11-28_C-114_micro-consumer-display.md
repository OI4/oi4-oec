# C-114 – Micro SDK hardware demo: DHT22 sensor and Core2 climate display

- Date: 2026-11-28 (wall clock of the session: 2026-11-13)
- Phase: none open (all phases closed after C-111); PO order (G-3.4 a: scope)
- Agent/Author: GitHub Copilot (Claude Opus 5.5)
- Status: done
- Baseline: guideline `980cde8` (unchanged); workspace before the cycle: `647c2a9`

## Intent

The PO connected two boards by USB and ordered two smart device services on the Micro SDK, both on the broker of the dev environment on the PO's computer:

1. an **ESP32-S3 with a DHT22 at GPIO 4** as the OT connector (the sensor);
2. an **M5Stack Core2** that visualises the data of the sensor.

## PO answers (session 2026-11-13)

- **ServiceType of the Core2:** `OTConnector` (asked: `OTConnector`, `ITConnector`, `Utility`; a directly connected smart device, MG 7.2, SF-102).
- **Dev PKI:** re-key with the LAN address of the Mac (`go run ./pki -force -san <ip>`), so the boards reach the broker over Wi-Fi with a certificate that names it.
- **Gate failure caused by Go 1.27.4 (found in this cycle):** fix it in C-114 as its own commit (option (a) of three: fix now, demo only, revert the bump).

## What was built

### Micro SDK: consumer subscriptions (ADR-0022 amendment C-114, A-671, A-672)

A display needs to receive other applications' publications; the Micro SDK only subscribed to its own request topics. Additive to the API of C-092 … C-097 (zero-initialised configurations keep their behaviour):

- `oecm_config.subscriptions`/`subscription_count` (up to `OECM_SUBS_MAX` 4 topic filters, borrowed) and `on_message`; checked by `oecm_init` (form `Oi4v2/<ServiceType>/<AppId>/Pub/<Resource>…`, wildcards as whole levels, Level 2 and up);
- subscribed after the request topics at every connect; a matching publication is handed to `on_message` with the index of the first matching filter;
- listed first in the `SubscriptionList` (`NONE_0`, WriterGroupName as `oec-core` `sublist.rs`), selectable by `Get`, `Set`/`Del` answered with `BadNotWritable`;
- stateless read functions: `oecm_topic_app`, `oecm_topic_source`, `oecm_msg_count`, `oecm_msg_number`, `oecm_msg_text`.

Unit tests first (`test_topic.c`, `test_reader.c`, `test_level2.c`); the emitter adds the consumer messages to the corpus validated by `oec-check validate`. Its first run found an invalid request of the emitter itself: `/Get/SubscriptionList/` with the Filter `/Health/Any` is not a valid topic (REQ-TOP-022); it now asks for `/Data/Climate`.

### Sensor: `esp32-dht22` on the ESP32-S3

The existing DHT22 demo (log 2026-11-28_HW_esp32-dht22, ADR-0012 amendment "smart sensor demos") unchanged, built for `esp32s3`, flashed and provisioned with `provision.sh`. First run of the S3 build on a board: readings around 24 °C / 54 %, online in the registry.

### Display: `oec/demos-smart-sensors/m5core2-display/` (new)

An Arduino sketch on the Arduino library `OECMicro` and M5Unified (the Arduino path of the Micro SDK, ADR-0017, `micro.yml` since C-106), Level 3:

- MAM `OEC Micro M5Stack Core2 Display` / `OECM-CORE2-DISPLAY` / base MAC, DeviceClass `Display`;
- subscriptions `Oi4v2/OTConnector/+/Pub/Data/+/Climate` and `Oi4v2/OTConnector/+/Pub/Health/+`; up to six sensors, a sensor that only sent Health gives way to one with data, its own publications ignored;
- Data `View` (Sensors, Shown), Config `Display`/`General` (Title, StaleAfter 10 … 3600 s);
- display: label, temperature, humidity, Health and age of one sensor, grey and "stale" after `StaleAfter`; button A or a tap shows the next sensor;
- provisioning from the NVS partition `oecprov` at 0x10000 written by the DHT22 demo's `provision.sh` (same keys), so one script provisions both boards; `partitions.csv` keeps `otadata` at 0xe000 because `arduino-cli upload` writes `boot_app0` there;
- `build.sh` with the pinned toolchain of `oec/ci/micro-arduino.sh`, warnings in the library or the sketch fail the build; `target/` with `flash_args`.

CI: `micro-arduino.sh` also compiles the sketch for `esp32:esp32:m5stack_core2` (required, `--warnings all`); `TestReleaseVersion` checks its `VERSION` against `oec/VERSION`.

### Go SDK: the library without LLVM bitcode (ADR-0027 amendment C-114, A-673)

The gate failed in `sdk/check.sh` at `go vet` of the Go SDK: "disallowed LTO section __LLVM,__bitcode in #cgo LDFLAGS: …/liboec_ffi.a". Not caused by the demo: go1.27.4 (the toolchain of every module since commit `e2188fb`, not pushed) rejects LTO sections in libraries of a `-L` directory of the module, and the precompiled Rust standard library embeds bitcode in its objects. `oec-buildlib` and `ffi-lib.sh` now remove the bitcode sections with the `llvm-objcopy` of the rustup component `llvm-tools`; `setup-tools.sh` and the weather image install it. Without the fix the hosted `gate` and `sdk.yml` would fail on the next push.

## Verification

- **Dev environment:** PKI re-keyed with `-san 192.168.178.53`, stack with `OEC_DEV_BIND=0.0.0.0` and the profiles demos and registry, all containers healthy.
- **Boards:**

| Board | Port | Identification Link | D7 `test` Level 3 (no proxy) |
|---|---|---|---|
| ESP32-S3 (QFN56 rev v0.2), DHT22 GPIO 4 | `/dev/cu.usbmodem101` | `oi4.org/OEC,20Micro,20ESP32-S3,20DHT22/OECM-ESP32S3-DHT22/44BD8D791E34` | 39 pass, 0 fail, 16 skip |
| M5Stack Core2 (ESP32-D0WDQ6-V3 rev v3.1) | `/dev/cu.usbserial-5C9A0604311` | `oi4.org/OEC,20Micro,20M5Stack,20Core2,20Display/OECM-CORE2-DISPLAY/8C4F0099CA60` | 38 pass, 0 fail, 17 skip |

  Both INCONCLUSIVE only because the connection and restart tests need the proxy and the restart hooks; the extra skip of the Core2 is T7-MON-14 (no periodic `View` publication in the window – its interval is 60 s).
- **Consumer path end to end:** the Core2's `View` on the bus reports `Sensors` 1 and `Shown` = the S3's Identification Link; the PO confirmed the display (title, temperature, humidity, Health, age).
- **Micro SDK:** `oec/sdk/micro/check.sh` green (emitter: 71 messages, 0 invalid); `micro-arduino.sh 3.3` green incl. the Core2 sketch.
- **Go SDK strip:** `go vet` with go1.27.4 on darwin/arm64 and amd64 after `oec-buildlib` (the first gate run after the fix still failed on a stale `lib/darwin_amd64/liboec_ffi.a` of 2026-11-01, rebuilt; README note); an ELF archive in the pinned Rust image: `.llvmbc` 1 → 0 sections.
- **Gate:** `oec/check.sh` "all checks passed" on the union of all C-114 changes (a run before failed in `demos/check.sh` at load average 62 – Spotlight indexing: a 4-minute Health gap in the config demo D7 run and a port race of the weather test, A-660; rerun green). `oec/ci/micro-arduino.sh 3.3` ok with the Core2 sketch (1 274 859 bytes, no warning).

## Agent decisions (G-3.4 b)

- **A-671 Consumer subscriptions in the Micro SDK:** filters borrowed from the configuration (no state growth), at most four, Level 2 and up; `NONE_0` entries only, definition order (differences to the Rust SDK recorded in the README).
- **A-672 Read functions instead of a parser API:** stateless lookups by path (`Payload.Temperature`) on the received buffer; no allocation, the core's JSON scanner reused.
- **A-673 Bitcode strip for the Go SDK:** `llvm-objcopy` of rustup's `llvm-tools` (one tool for Mach-O, ELF and COFF, same version as `rustc`); not chosen: go1.27.3 (keeps GO-2026-6859), `CGO_LDFLAGS` from the environment for every user, fat LTO (`compiler_builtins` keeps its bitcode). The ids A-669/A-670 were already used by ADR-0032 amendment C-113 addendum 1 (commit `e2188fb`) without a REVIEW entry; REVIEW now lists them.
- **A-674 Core2 demo as an Arduino sketch** (not ESP-IDF): the Core2's display stack is M5Unified, which the Micro SDK's Arduino path already uses for the `M5Stack` example; the sketch reuses the DHT22 demo's provisioning partition and script.

## Open for the PO

Nothing to decide. Optional: push (the workspace is ahead of `origin/main` by the commits of `e2188fb` onwards; the push brings the Go 1.27.4 bump together with its fix).

## Hand-over (G-3.10)

Same session: the cycle is closed. Both boards keep running against the dev stack; before passing a board on, erase it (`esptool erase-flash`: it holds the Wi-Fi and broker credentials).
