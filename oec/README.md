# oec – OI4 Open Edge Computing v2.0 monorepo

All deployable artifacts of the OEC v2.0 project live here (ADR-0012). Planning, decisions and cycle logs are in [`../project/`](../project/); the entry point for agents is [`../AGENTS.md`](../AGENTS.md).

| Folder | Deliverable | Content | State |
|--------|-------------|---------|-------|
| `schema/` | D1 | JSON Schema 2020-12 for all OEC messages + schema test vectors | planned (P2) |
| `sdk/rust/` | D2 | Rust SDK (Cargo workspace): `oec-core` (sans-IO protocol logic), `oec` (runtime: MQTT/TLS, timers, `/etc/oi4` bootstrap), `oec-ffi` (stable C ABI + `oec.h`) | planned (P4) |
| `sdk/micro/` | D9 | Micro SDK for microcontroller-class smart devices (Messaging L1), reuses the `no_std` part of `oec-core` (ADR-0016) | planned (P4b) |
| `conformance/` | D7, D8 | One Go module (ADR-0013/0014), two CLIs: D7 application (Messaging) compliance, D8 infrastructure (Service Container + platform) compliance; Windows/macOS/Linux. **Must not depend on anything in `sdk/`** – shares only `schema/` (ADR-0006) | planned (P2 `validate`, P5, P5b) |
| `demos/weather/` | D3 | Weather service (OTConnector, Messaging L3), MET Norway provider + offline simulation (ADR-0010) | planned (P6) |
| `demos/logger/` | D4 | Logger service (Utility, Messaging L1) | planned (P6) |
| `registry/` | D5 | OEC Registry, read-only in step 1 (ADR-0008) | planned (P7) |
| `docs/` | D6 | Developer documentation – standalone Docusaurus site (ADR-0012) | planned (P8, continuous) |
| `dev/` | – | Local development environment: docker compose, TLS broker, dev PKI, sample `/etc/oi4` layout | planned (P4) |

Folders are created when their phase starts. Further language bindings (C#, Go, TypeScript, …) are added under `sdk/<language>/` once decided (ADR-0011).

License: MIT (see `LICENSE`, ADR-0004).
