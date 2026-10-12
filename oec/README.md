# oec – OI4 Open Edge Computing v2.0 monorepo

All deployable artifacts of the OEC v2.0 project live here (ADR-0012). Planning, decisions and cycle logs are in [`../project/`](../project/); the entry point for agents is [`../AGENTS.md`](../AGENTS.md).

| Folder | Deliverable | Content | State |
|--------|-------------|---------|-------|
| `schema/` | D1 | JSON Schema 2020-12 for all OEC messages + schema test vectors | planned (P2) |
| `sdk/rust/` | D2 | Rust SDK (Cargo workspace): `oec-core` (sans-IO protocol logic), `oec` (runtime: MQTT/TLS, timers, `/etc/oi4` bootstrap), `oec-ffi` (stable C ABI + `oec.h`) | planned (P4) |
| `conformance/` | D7 | Conformance test CLI in **Go** (ADR-0013), Windows/macOS/Linux. **Must not depend on anything in `sdk/`** – shares only `schema/` (ADR-0006) | planned (P2 `validate`, P5) |
| `demos/weather/` | D3 | Weather service (OTConnector), MET Norway provider + offline simulation (ADR-0010) | planned (P6) |
| `demos/logger/` | D4 | Logger service (Utility) | planned (P6) |
| `registry/` | D5 | OEC Registry, read-only in step 1 (ADR-0008) | planned (P7) |
| `docs/` | D6 | Developer documentation – standalone Docusaurus site (ADR-0012) | planned (P8, continuous) |
| `dev/` | – | Local development environment: docker compose, TLS broker, dev PKI, sample `/etc/oi4` layout | planned (P4) |

Folders are created when their phase starts. Further language bindings (C#, Go, TypeScript, …) are added under `sdk/<language>/` once decided (ADR-0011).

License: MIT (see `LICENSE`, ADR-0004).
