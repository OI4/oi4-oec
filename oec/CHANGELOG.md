# Changelog of the OEC deliverables

All code deliverables of `oec/` share one version, the release train in [`VERSION`](VERSION) (ADR-0033, [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html)): the Rust SDK (`oec-core`, `oec`), the C ABI (`oec-ffi`), the Go SDK, the micro SDK, the compliance tools D7 `oec-check` and D8 `oec-infra`, the demos (logger, weather, config service), the OEC Registry and their images. Before 1.0.0 a MINOR version may change the public API.

Separate versions:

- the JSON Schema bundle: `2.MINOR.PATCH`, its own [CHANGELOG](schema/CHANGELOG.md) (ADR-0019 §6);
- the report formats of D7 and D8: version 1, frozen (`oec-check report-schema`, `oec-infra report-schema`);
- the C API of the micro SDK: `OECM_API_VERSION` in `sdk/micro/include/oecm.h`.

`oec/ci/release.sh` takes the section of the version it builds as the release notes; a tag build fails while the heading still says "unreleased". Every change of a deliverable adds a line to the section of the next version.

## 0.1.0 – unreleased

First release of OEC v2.0 for the OEC Messaging Guideline and the OEC Runtime & Infrastructure Guideline at the baseline `52d8470`.

- **JSON Schema bundle** 2.0.x (D1): strict and tolerant flavor of every message, resource, payload and `broker.json`.
- **SDK** (D2): Rust SDK `oec` with `oec-core` (Messaging Level 1–3, provider and consumer, own MQTT 3.1.1 client over TLS), the C ABI `oec-ffi` with the header `oec.h` and prebuilt static libraries for Linux (x86-64, AArch64, ARMv7), macOS (x86-64, Apple silicon) and Windows (x64 and Arm64 with MSVC, x64 with MinGW), the Go SDK over the C ABI.
- **Micro SDK** (D9): C17 for microcontrollers, Messaging Level 1–3, ESP-IDF component and Arduino library `OECMicro` (ESP32, ESP32-S3).
- **Application compliance tool** D7 `oec-check`: `validate`, `monitor`, `test` (55 tests), `list`, `explain`, `report-schema`.
- **Infrastructure compliance tool** D8 `oec-infra`: `image`, `run`, `container`, `host`, `bus` (17 tests), `list`, `explain`, `report-schema`.
- **Demos**: logger (C on the C ABI, Level 2), weather (Go SDK, Level 3, MET Norway provider), config service (Rust, edits the `Config` of the other applications); each with a small Web UI and an image.
- **OEC Registry** (D5): Messaging Level 3, REST and SSE API (OpenAPI 3.2, AsyncAPI 3.1), Web UI, image.
- **Images** for linux/amd64 and linux/arm64: `oi4/oec-logger`, `oi4/oec-weather`, `oi4/oec-config`, `oi4/oec-registry`, `oi4/oec-conformance`, each with its SBOM and license texts.
- **Supply chain**: an SPDX 2.3 SBOM, the license texts and a license report for every deliverable; the security policy in `SECURITY.md`.
