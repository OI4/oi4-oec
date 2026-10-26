# oec – OI4 Open Edge Computing v2.0 monorepo

All deployable artifacts of the OEC v2.0 project live here (ADR-0012). Planning, decisions and cycle logs are in [`../project/`](../project/); the entry point for agents is [`../AGENTS.md`](../AGENTS.md).

| Folder | Deliverable | Content | State |
|--------|-------------|---------|-------|
| `spec/` | – | Spec baseline (ADR-0018): requirements catalog `REQ-<AREA>-NNN` (YAML + catalog JSON Schema), topic grammar (ABNF + derived regexes), Method × Resource matrix, topic and golden test vectors, Go tools (lint, render, coverage, extraction). Derived only from the guideline; shared contract for `schema/`, `sdk/`, `conformance/`, `docs/` | P1 done (C-032); payload oracle, vectors and coverage gate (P2, C-033 … C-041) |
| `schema/` | D1 | JSON Schema 2020-12 for all OEC messages (ADR-0019): modules, one entry schema per topic form, Go embed module, test harness with two validators (`check.sh`); test input = the vectors of `spec/` | release 2.0.0: 29 of 29 entry schemas (P2 closed in C-041, tag `oec-schema-2.0.0`) |
| `sdk/rust/` | D2 | Rust SDK (Cargo workspace): `oec-core` (sans-IO protocol logic), `oec` (runtime: MQTT/TLS, timers, `/etc/oi4` bootstrap), `oec-ffi` (stable C ABI + `oec.h`) | P4 since C-046: workspace (ADR-0026) with `oec-core` increments 1–2 (identity, topic model); P3 API sketch `sdk/rust/sketch` (ADR-0020) + C ABI sketch `sdk/rust/sketch/c` (ADR-0021) until `oec` replaces them |
| `sdk/micro/` | D9 | Micro SDK for microcontroller-class smart devices (Messaging L1) in **C17**: portable core + platform abstraction layer, reference port ESP-IDF on ESP32/ESP32-S3 incl. M5Stack, Arduino library as second port (ADR-0016/0017); independent of `sdk/rust/` | P3 design: API sketch `sdk/micro/sketch` (ADR-0022); product P4b |
| `conformance/` | D7, D8 | One Go module (ADR-0013/0014), two CLIs: D7 application (Messaging) compliance, D8 infrastructure (Service Container + platform) compliance; Windows/macOS/Linux. **Must not depend on anything in `sdk/`** – shares only `schema/` and `spec/` (ADR-0006/0018) | D7 `validate` (P2, C-040); test catalog D7/D8 and design `DESIGN.md` (P3, C-044, ADR-0025); `monitor`/`test` P5, D8 P5b |
| `demos/weather/` | D3 | Weather service (OTConnector, Messaging L3), MET Norway provider + offline simulation (ADR-0010) | planned (P6) |
| `demos/logger/` | D4 | Logger service (Utility, Messaging L1) | planned (P6) |
| `registry/` | D5 | OEC Registry, read-only in step 1 (ADR-0008) | planned (P7) |
| `docs/` | D6 | Developer documentation – standalone Docusaurus site (ADR-0012) | planned (P8, continuous) |
| `dev/` | – | Local development environment: docker compose, TLS broker, dev PKI, sample `/etc/oi4` layout | planned (P4) |

`./check.sh` is the local CI of the monorepo (spec checks with the coverage gate, schema bundle and release freeze, oracle/schema cross-check, conformance tests, Rust SDK workspace – fmt, clippy, tests against `spec/export/`, doc, `cargo deny` – and compile check of the P3 SDK sketches via `sdk/check.sh`; needs `cargo-deny`). Folders are created when their phase starts. Further language bindings (C#, Go, TypeScript, …) are added under `sdk/<language>/` once decided (ADR-0011).

License: MIT (see `LICENSE`, ADR-0004).
