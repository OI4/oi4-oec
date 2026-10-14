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
