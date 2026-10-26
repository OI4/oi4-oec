# ADR-0017 – Micro SDK (D9) in C with ESP32 as reference board

- Status: accepted
- Date: 2026-10-14
- Deciders: product owner (Thomas), C-009
- Related: Q-016, ADR-0016, ADR-0011, ADR-0006, G-4.2, G-4.6

## Context

ADR-0016 introduced a micro SDK for microcontroller-class smart devices (Messaging Level 1, L2 if feasible). The agent had proposed Rust `no_std`. The PO's answer to Q-016: something from the **C spectrum** makes sense, with an **ESP32** as reference board.

## Decision

- **Language:** portable **C17**. The headers can also be used from C++ (`extern "C"`). The core uses no compiler-specific extensions; heap use is optional (static buffers are possible).
- **Structure** in `oec/sdk/micro/`:
  - **Portable core**:
    - Identification Link and base64url
    - topic build/parse incl. percent-encoding
    - JSON encoding of the NetworkMessage/DataSetMessage and of MAM/Health
    - lifecycle state machine for Birth, Will, Close and the Health heartbeat
    - L2: `Get` handling
  - **Platform abstraction layer (PAL)** – the interface the core calls for:
    - MQTT 3.1.1 client with TLS
    - clock
    - optional persistence
  - **Reference port for ESP-IDF** (Espressif IoT Development Framework):
    - MQTT via `esp-mqtt`
    - TLS via `esp-tls`/mbedTLS
    - FreeRTOS
- **Reference board:** **ESP32**, ESP-IDF target `esp32`, e.g. an ESP32-DevKitC. Other ESP32 variants (S3, C3, C6) are build-tested on a best-effort basis. Further ports (e.g. Zephyr) are possible later through the PAL.
- **API style:** follows the C API of D2 (`oec.h`, ADR-0011) where sensible, so that developers see a familiar shape. There is no shared code.
- **Independence:** the micro SDK and the Rust SDK are separate implementations. Consistency comes from the language-neutral D1 schemas and test vectors (G-4.2) and from D7. The micro SDK example device must pass D7 `--level 1`.

## Rationale

- C is the native language of ESP-IDF and of practically every MCU vendor SDK. It brings no extra toolchain; Rust on Xtensa needs the separate `espup` toolchain fork.
- MQTT, TLS, Wi-Fi and certificate handling are already maintained in ESP-IDF, so the micro SDK only adds OEC protocol logic.
- A small portable core plus a PAL keeps it open for other MCUs.

## Consequences

- **ADR-0016 and ADR-0011 are amended:** `oec-core` (Rust) **no longer needs to be `no_std`-compatible**, because the micro SDK does not reuse it. It is still welcome if it costs nothing.
- **Roadmap:** P4b no longer depends on P4. It needs P1/P2 (REQ catalog, schemas, test vectors) and the API sketch from P3.
- **Toolchains in the monorepo:** Rust, Go, C (ESP-IDF, CMake). Host builds of the portable core use CMake + a host C compiler, so unit tests run on macOS, Windows and Linux without hardware.
- **Testing without hardware:**
  - Host unit tests against the shared test vectors.
  - Integration in Espressif's QEMU (supports `esp32` incl. emulated Ethernet) against the dev broker with D7 – to be confirmed in P4b.
  - Final check on the real board.
- **Candidates (decided in P3/P4b, G-4.4):**
  - JSON: hand-written encoder (small, deterministic), decoding with `cJSON` (bundled with ESP-IDF) or `jsmn`.
  - Unit tests: Unity (also used by ESP-IDF).
- **Licenses:** ESP-IDF is Apache-2.0 and cJSON is MIT, both compatible with our MIT license (ADR-0004).

## Amendment C-010 – considered options and Arduino

The PO asked (C-010) whether C matches the agent's recommendation, given that Rust might be technically better while C has the most libraries for Arduino and ESP32.

| Criterion | Rust (`no_std` / esp-rs) | C17 (chosen) |
|-----------|--------------------------|--------------|
| Memory safety (network-exposed device, CRA) | ✅ strong | ⚠️ needs discipline: static analysis, fuzzing of the parsers, sanitizers in host tests |
| Reuse of the Rust `oec-core` (one protocol implementation) | ✅ possible | ❌ second implementation; mitigated by shared test vectors + D7 |
| ESP32 toolchain | ⚠️ classic ESP32/S3 (Xtensa) need the `espup` toolchain fork; C3/C6 (RISC-V) work with upstream Rust | ✅ the ESP-IDF default |
| Integration into existing firmware (ESP-IDF, Arduino, Zephyr, vendor SDKs) | ⚠️ only as a precompiled static library per target | ✅ plain source integration |
| Arduino ecosystem | ❌ practically not usable as an Arduino library | ✅ can be packaged as an Arduino library |
| Target audience (device makers) | small Rust share | C/C++ is the de-facto standard |

**Assessment:** The agent's earlier Rust proposal was driven mainly by reusing `oec-core`. The micro SDK scope is small (L1, L2 if feasible: IL/base64url, topics, JSON for MAM/Health, lifecycle), so a second implementation is cheap, and the shared test vectors plus D7 keep both SDKs aligned. For a micro SDK, adoption by device makers weighs more than reuse. **C17 is the recommendation.**

Rust stays the better choice for D2 (container apps), where its strengths count fully.

**Mitigations for C** (part of P4b):
- No dynamic parsing of untrusted input in L1 (L1 only publishes). L2 `Get` parsing is size-bounded.
- Static analysis (e.g. `clang-tidy`, `cppcheck`).
- AddressSanitizer and UndefinedBehaviorSanitizer in the host tests.
- Fuzzing of the topic and JSON parsers.

**Arduino:** In addition to the ESP-IDF reference port, the core is also packaged as an **Arduino library** with an Arduino port of the PAL (Arduino-ESP32 core, which is based on ESP-IDF). This is planned in P4b, best effort after the ESP-IDF port. The core must therefore stay compatible with the Arduino build (no CMake-only features in the core, C17 subset supported by the Arduino ESP32 toolchain). Boards without TLS-capable hardware or memory (e.g. AVR) are out of scope, because TLS is mandatory at every level.

## Amendment C-011 – M5Stack

The PO asked to include **M5Stack** devices. M5Stack controllers use the ESP32 family:
- Core/Core2: ESP32
- CoreS3 and AtomS3: ESP32-S3

They are programmed with Arduino (library `M5Unified`, MIT), ESP-IDF or UIFlow/MicroPython. The architecture therefore does not change. Changes:

- **Supported targets:** ESP-IDF targets **`esp32` and `esp32s3`** are now both *supported* (CI build + host tests). Previously `esp32s3` was best effort. C3/C6 remain best effort.
- **Reference hardware:** generic ESP32 dev board (e.g. ESP32-DevKitC) **plus an M5Stack device** as a second reference: M5Stack **Core2** (ESP32) or **CoreS3** (ESP32-S3), whichever is available to the PO.
- **Example firmware for M5Stack** in addition to the generic example. It uses the display and buttons via `M5Unified`, e.g. to show the Identification Link, connection state and Health, or to switch Health with a button. This is part of the **example only**: the SDK core and the PALs do not depend on `M5Unified`.
- **Ports:** the Arduino port (best effort, see C-010) covers M5Stack directly. The ESP-IDF port covers M5Stack too, since it is plain ESP32/ESP32-S3.
- **Out of scope:** UIFlow/MicroPython bindings.

## Amendment C-045 – JSON library

ADR-0024 (accepted in the C-045 review, Q-027) settles the JSON candidates named under "Consequences": the core uses its **own encoder** and its **own bounded pull reader** (no heap, limits on depth, token length and members). `jsmn` is abandoned (G-4.4) and `cJSON` needs a heap or an arena and is an extra component since ESP-IDF 6, so neither is used. The license note on cJSON no longer applies. The rest of this ADR is unchanged.
