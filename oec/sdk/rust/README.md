# Rust SDK D2 – Cargo workspace (P4, from C-046)

Workspace of the Rust SDK D2 (ADR-0011, ADR-0023, ADR-0026). The public API is the one decided in ADR-0020 (sketch in `sketch/`); the crates below grow with the P4 increments of `project/ROADMAP.md`.

| Path | Content | State |
|---|---|---|
| `oec-core/` | sans-IO protocol logic: Identification Link, dnp-encoding, Oi4Identifier (`il`), topic elements, allowed forms, Filter, topic parsing/building, subscription filters (`topic`); typed messages with fixed content masks and Action correlation (`message`), resource payloads (`resource`), embedded D1 schemas with optional validation (`schema`, feature `strict-validation`); MQTT 3.1.1 codec and client session as a sans-IO state machine (`mqtt`); Birth/Will/Close and Health heartbeat (`lifecycle`); the provider (`provider`): sources, MAM/Health/Profile/PublicationList, DataSetWriterIds, `Get` answers, pagination, Data DataSets with Metadata, UUIDv5 DataSetClassId, MetaDataVersions, publish modes, intervals and dead band | increments 1–2 (C-046), 3 (C-047), 5a (C-049), 6a (C-053), 6b (C-054) |
| `oec/` | public API of ADR-0020, wraps or re-exports `oec-core`: `Level`, `Error`, `App`/`AppBuilder` with `Bootstrap` (container storage contract with `root` prefix, or explicit `Broker`), identity and MAM (`identity`), `Store` (`FileStore`, `MemoryStore`), Config definition and typed getters (`config`), `StatusCode`; `build()` validates identity, bootstrap and level without connecting. Runtime: TLS transport (tokio + rustls, trust only from the CA of the storage or the `Broker`, client certificate or credentials, PKCS#8 keys with passphrase), `App::run` (own `current_thread` runtime, SIGINT/SIGTERM), `App::run_async`, `AppHandle` (shutdown, `ConnectionState`), `on_connection`, reconnect with backoff; drives `oec_core::mqtt::Session` and `oec_core::lifecycle::Lifecycle` (Birth, Health, heartbeat, Close, Will); assets (`asset`) and Data DataSets (`data`) on the `oec_core::provider`, writer ids and MetaDataVersions persisted | increments 4 (C-048), 5b (C-050), 6a (C-053), 6b (C-054) |
| `oec-ffi/` | C ABI (ADR-0021) | P4 increment 10 |
| `sketch/` | P3 API sketch (not product code); the items not yet in `oec` are still the contract; deleted when the last piece has moved | frozen, shrinking |
| `spike/` | P3 spikes `schema-embed`, `mqtt-codec` (not product code) | frozen |

`sketch/` and `spike/` are stand-alone crates outside the workspace (`exclude`), with their own lock files.

## Rules

- Edition 2024, MSRV `rust-version = 1.89`, license MIT, `publish = false`.
- Lints for all members: `unsafe_code = "forbid"` (`oec-ffi` will opt out with a documented exception), `missing_docs`, clippy `all` + `pedantic` (warnings are errors in the check).
- `oec-core` does no I/O and has no stability promise of its own; `oec` re-exports or wraps its types. Its runtime dependencies are the I/O-free crates of ADR-0023: serde, serde_json, time, uuid (feature `v5` for the DataSetClassId, ADR-0023 amendment C-054), and jsonschema `=0.60.1` behind the feature `strict-validation` (same pin as the schema gate).
- `oec-core` has one dev-dependency: proptest (property tests of the MQTT codec and session, default features off; ADR-0023 amendment C-049). No cargo-fuzz: it needs nightly.
- `oec` adds thiserror, log, tokio (`rt`, `net`, `time`, `sync`, `signal`), rustls 0.23 (`std`, `tls12`), tokio-rustls 0.26, rustls-pki-types and pkcs8 (decryption of PKCS#8 keys with passphrase) (ADR-0023, amendment C-050) and forwards the feature `strict-validation` to `oec-core`. The MQTT codec of Q-026 is `oec_core::mqtt` (sans-IO, C-049); `oec` drives it over TLS (C-050).
- Crypto provider of rustls: cargo feature `crypto-aws-lc-rs` (default, aws-lc-rs with post-quantum key exchange preferred) or `crypto-ring` (ring, e.g. for ARMv7: `--no-default-features --features crypto-ring`). With both, ring wins; with neither, the build fails with a `compile_error!`.
- `oec` has one dev-dependency: rcgen (test PKI of the runtime tests). `oec/tests/runtime.rs` runs the runtime against an in-process TLS test broker built on the `oec-core` codec (lifecycle order, Will on drop, reconnect, keep-alive timeout, refused CONNECT, untrusted broker, client certificate over TLS 1.2, encrypted key, shutdown, `run_async`). No network access beyond loopback.
- Interop tests against Mosquitto 2.1.2 of the dev environment (`oec/tests/interop.rs`, C-051, ADR-0026 amendment C-051): credentials and client certificate, Birth → Health → Close → DISCONNECT, the Will of a killed child process, reconnect after a broker restart, refused logins, wrong CA, TLS 1.2/1.3, packet limits (SF-105), explicit bootstrap, L2 assets and `Get` answers (C-053), Data DataSets with Metadata and an Interval (C-054). Opt-in: run `oec/dev/interop.sh` (needs a container engine, pulls the pinned image); without `OEC_INTEROP=1` the tests pass at once, so the gate needs neither Docker nor network. See [`oec/dev/README.md`](../../dev/README.md). They replace the `OEC_SMOKE_*` smoke test of C-050.
- `oec-core/build.rs` embeds the schema tree `oec/schema/v2` (strict sources, with `strict-validation`) and reports `Version`/`Baseline` of `oec/schema/schema.go` as `oec_core::schema::{VERSION, BASELINE}` (ADR-0023 amendment C-047).
- Every rule cites its REQ ids in a doc comment; the walker in `sketch/tests/sketch.rs` checks that every cited id exists and is not deprecated.
- Acceptance test: the shared test vectors of the spec baseline, read from the JSON export `oec/spec/export/` (generated by `spectool render`), see `oec-core/tests/vectors.rs` (IL, dnp, Oi4Identifier, topic, matrix), `oec-core/tests/payload.rs` (all payload vectors; with `strict-validation` the verdicts equal the schema gate) `oec-core/tests/lifecycle.rs` (Will, Birth and Close equal V-PAY-025/023/024; with `strict-validation` every generated lifecycle payload passes the strict schema) and `oec/tests/broker.rs` (all `broker.json` vectors end to end through the container bootstrap; with `strict-validation` the verdicts equal `platform/broker.schema.json`).
- Supply chain: `Cargo.lock` is committed; `deny.toml` is the cargo-deny policy of ADR-0023 §2.

## Check

```sh
oec/sdk/check.sh   # part of oec/check.sh (step 5)
```

It runs `cargo fmt --check`, then `cargo clippy --all-targets -D warnings`, `cargo test` and `cargo doc` (warnings are errors) once with default features and once with `--features strict-validation`, `cargo clippy -p oec --no-default-features --features crypto-ring`, then `cargo deny check`, all `--offline --locked` except the advisory DB fetch of cargo-deny (fallback: cached DB). cargo-deny is installed with `cargo install cargo-deny --locked`.
