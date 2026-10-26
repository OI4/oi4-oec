# ADR-0023 – Dependencies of the Rust SDK (D2)

- Status: **accepted** (PO, C-045 review 2026-10-26; proposed in C-043). Amendment C-047 (feature sets, version source `schema.go`, tolerant flavor in memory) and C-048 (thiserror, log in `oec`) by the agent (G-3.4 b).
- Date: 2026-10-26
- Deciders: product owner (Thomas) – proposed by the agent in C-043
- Related: ADR-0004 (MIT), ADR-0011 (S4: `oec-core` sans-IO, `oec` runtime, `oec-ffi`), ADR-0019 §7/§8 (embedded schemas, jsonschema-rs), ADR-0020 (public API, execution model, `strict-validation`), ADR-0021 (C ABI), GUARDRAILS G-3.4/G-4.4, Q-008 (platforms), Q-026

## PO decision (C-045 review, 2026-10-26)

Accepted as proposed. Q-026 (a): own sans-IO MQTT 3.1.1 codec and session in `oec-core`; `rumqttc-v4-next` and `paho-mqtt` are not used.

## Context

ADR-0011 left the exact crates open: "MQTT client, TLS, JSON Schema are decided in P3/P4 under G-3.4/G-4.4". ADR-0020 fixed the execution model:
- a blocking `run()` on an SDK-owned runtime;
- `run_async()` for tokio users;
- synchronous, non-blocking updates;
- async consumer requests.

The choice follows G-4.4 (current stable, few dependencies, no abandoned packages, lock files) and G-3.4 (licenses compatible with MIT, ADR-0004). It must cover amd64 and arm64 (Linux, macOS, Windows), with ARMv7 Linux as best effort (Q-008).

The agent checked the facts on 2026-10-26 (crates.io, GitHub, RustSec advisory DB). Two spikes back the decision (`oec/sdk/rust/spike/`, run by `oec/check.sh`).

**Finding that changes the expected choice:** `rumqttc`, the obvious MQTT client and the assumption of the research (`research/sdk-options.md`), is effectively unmaintained:
- The last release is 0.25.1 (2025-11). Its rustls feature pins `rustls-webpki ^0.102.8`. That line has five open advisories without a fix (RUSTSEC-2026-0049, -0098, -0099, -0104, -0382; fixed only in 0.103.x). It also pins `rustls-pemfile`, which is unmaintained (RUSTSEC-2025-0134).
- The bump PRs were closed unmerged (#1072/#1073). The advisory issues stay open (#1046 since 2026-04, #1067 since 2026-07).
- The maintainer said in #1029 (2026-03) that the company has deprioritized the project; a one-person fork `rumqttc-next` exists.
- The `EventLoop` cannot take a TLS stream of our own without skipping its own CONNECT, so the vulnerable path cannot be avoided by configuration.

## Decision

### 1. Crates

| Role | Choice | License | Why / how |
|---|---|---|---|
| Async runtime | **tokio** 1.x (1.54) | MIT | De-facto standard, needed by `run_async` (ADR-0020). Features `rt`, `net`, `time`, `sync`, `signal` (no `full`). `run()` uses a `current_thread` runtime on the SDK thread. |
| MQTT 3.1.1 client | **own sans-IO codec and session in `oec-core`** (Q-026 option a) | MIT (ours) | OEC needs a small, frozen subset of MQTT 3.1.1: CONNECT with a new Will per connect (REQ-LIFE-016), PUBLISH QoS 0/1 without retain (REQ-MQTT-017/018), SUBSCRIBE, keep alive (REQ-MQTT-014), clean session (REQ-MQTT-015). The SDK has to own Will, reconnect and backoff anyway. Spike `mqtt-codec`: 307 lines, 8 byte-exact tests. The product version is ~1–1.5k lines with session state, fuzzing and broker interop tests. The runtime connects it through tokio `TcpStream` + `tokio-rustls`. |
| TLS | **rustls** 0.23 + **tokio-rustls** 0.26 | Apache-2.0/ISC/MIT, MIT/Apache-2.0 | rustls only speaks TLS 1.2/1.3, so REQ-MQTT-005 holds by construction. No OpenSSL, so cross builds are simple. |
| Crypto provider | **aws-lc-rs** (rustls default); cargo feature `crypto-ring` switches to **ring** 0.17 | ISC/Apache-2.0/MIT/BSD-3 (aws-lc-sys); Apache-2.0 AND ISC (ring) | aws-lc-rs is the actively developed default of rustls (hybrid post-quantum key exchange). It needs no CMake, only a C compiler; Windows x86-64 needs NASM or the `prebuilt-nasm` feature. ring 0.17 is maintained (RUSTSEC-2025-0010 covers only versions before 0.17) and builds more easily for ARMv7. That is the fallback for Q-008. |
| Certificates | **rustls-pki-types** 1.x (`PemObject`) | MIT/Apache-2.0 | Reads CA, client certificate and key from the Message Bus storage (RIG 4.1.1/4.1.2; REQ-MQTT-007/008/009/010). By default, trust comes only from the storage, not from the OS trust store. |
| JSON | **serde** 1 + **serde_json** 1 | MIT/Apache-2.0 | Typed messages (REQ-MSG-001). Reading ignores unknown keys (tolerant receiver, REQ-MSG-046). |
| Schema validation | **jsonschema** (jsonschema-rs, `=`-pinned, today 0.60.1), `default-features = false`, behind the feature `strict-validation` | MIT | Same version as the schema gate `oec/schema/tools/rust-check` (ADR-0019 §8); both are raised together. Embedding as in spike `schema-embed`: `build.rs` + `include_str!` over `oec/schema/v2`, version and baseline from `oec/schema/release/`. The registry is built from memory; no file or HTTP resolution. It costs +3.1 MB per binary, so it is a feature. It is on in the SDK's own tests and examples and recommended for application CI (ADR-0020 §13). Only the strict flavor is embedded. |
| Errors | **thiserror** 2 | MIT/Apache-2.0 | `oec::Error` (ADR-0020). |
| Logging | **log** 0.4 facade; optional feature `tracing` (**tracing** 0.1 + **tracing-subscriber** 0.3 `Layer`) | MIT/Apache-2.0, MIT | SDK diagnostics go through `log`, and the Syslog bridge is a `log::Log` (ADR-0020 §11). `tracing` users get a layer instead of a second global logger. |
| Time | **time** 0.3 (`formatting`, `parsing`) | MIT/Apache-2.0 | RFC 3339 timestamps with offset and milliseconds. |
| UUID | **uuid** 1 (`v4`, `v5`) | Apache-2.0/MIT | Default DataSetClassId as UUIDv5 (sketch `data.rs`). |
| C header | **cbindgen** 0.29 as a CLI tool, not a build dependency | MPL-2.0 (tool only) | `oec.h` is generated and committed; the check compares it with a fresh output. The tool is not linked, and the generated header is our MIT code. |
| Signals | `tokio::signal` | – | SIGTERM/SIGINT and Windows Ctrl-C/Close for a clean shutdown; no `signal-hook`. |

### 2. Supply-chain rules (from P4 on)

- `Cargo.lock` of the SDK workspace is committed (G-4.4).
- **cargo-deny** (MIT/Apache-2.0) runs in `oec/check.sh` with:
  - a license allowlist for linked code: MIT, Apache-2.0, ISC, BSD-2/3-Clause, Zlib, Unicode-3.0, MIT-0;
  - RustSec advisories as errors (an unmaintained-advisory needs a logged exception);
  - bans on duplicate major versions of TLS and crypto crates;
  - only crates.io as a source.
- Not allowed in linked code: EPL, MPL, (L)GPL and AGPL. Build-only tools may be MPL-2.0.
- **MSRV** `rust-version = 1.89` (the highest MSRV of the chosen crates: uuid 1.29; the local toolchain). It is raised on purpose with a log entry; edition 2024 for the new crates.
- **Cross builds:** the SDK uses only pure Rust and C via `cc`, so native builds on the CI runners of each OS work. For Linux arm64/ARMv7, `cargo-zigbuild` (MIT, active) is the candidate. `cross` (last release 2023) is not used (G-4.4). Decided in P4 with the CI.

### 3. Not chosen

| Candidate | Reason |
|---|---|
| `rumqttc` 0.25 | Unmaintained, vulnerable `rustls-webpki` 0.102 in the tree, `rustls-pemfile` (see Context). |
| `rumqttc-v4-next` 0.34 (fork) | Active, current webpki, Apache-2.0, ~300k downloads in 90 days. But it has one maintainer and a young, changing API (0.33 → 0.34 in three months). It is option (b) of Q-026 and would sit behind an internal transport interface. |
| `paho-mqtt` 0.14 | Eclipse-backed and active, but it wraps the C library paho.mqtt.c with OpenSSL: a C build, OpenSSL for cross builds and Windows, and its own threads next to tokio. License EPL-2.0/EDL-1.0. Option (c) of Q-026. |
| `ntex-mqtt`, `mqtt-protocol-core`/`mqtt-endpoint-tokio` | ntex brings its own runtime (not tokio) and MSRV 1.97. mqtt-protocol-core is sans-IO and fits the architecture, but has one maintainer and low adoption (8k downloads). |
| `native-tls`/OpenSSL | System OpenSSL makes cross builds and reproducibility harder; rustls covers TLS 1.2/1.3. |
| `rustls-pemfile`, `webpki-roots`, `rustls-native-certs` (default) | Unmaintained (RUSTSEC-2025-0134), or not needed: trust comes from the Message Bus storage. The OS store may become an opt-in later. |
| async-std, smol | async-std is discontinued; tokio is required by `run_async` users anyway. |
| `chrono` | Fine, but `time` covers RFC 3339 with fewer features; one date crate is enough. |

## Consequences

- P4 increment 5 (connection & lifecycle) includes the MQTT codec and session in `oec-core`, with fuzzing (cargo-fuzz or proptest). Increment 9 (dev environment with a TLS broker) carries the interop tests. Mosquitto in a container is test infrastructure only, not linked.
- The dependency set of `oec` is about 11 direct crates, plus jsonschema with `strict-validation`.
- D9 does not share any of this (ADR-0017); its libraries are in ADR-0024.
- If Q-026 is answered with (b) or (c), only the transport and session part of `oec` changes. The public API (ADR-0020) and the C ABI (ADR-0021) stay the same.

## Amendment C-047 (2026-10-26) – first runtime dependencies in `oec-core` (agent, G-3.4 b)

Applied in P4 increment 3 within this ADR (libraries within the dependency rules, G-3.4 b; the version source follows the PO order of C-047 and ROADMAP P4 increment 3):

- **Features, smallest set:** serde 1 (`derive`), serde_json 1, time 0.3 (`std`, `formatting`, `parsing`; no `macros`), uuid 1 (`std`, `serde`). `v4`/`v5` of uuid are added by the increment that generates ids (the default DataSetClassId of `oec`, increment 6), not before. jsonschema `=0.60.1` with `default-features = false`, optional, behind `strict-validation`.
- **Tree:** 20 crates in the default build of `oec-core`, 81 with `strict-validation` (jsonschema's regex, URI and number crates). `cargo deny check`: advisories, bans, licenses and sources ok; no exception and no `ignore` entry was needed.
- **Version and baseline source (changes §1 "Schema validation"):** `oec-core/build.rs` reads `Version` and `Baseline` from `oec/schema/schema.go`, i.e. the tree that is actually embedded (today `2.0.1-dev` @ `734c560`), not from `oec/schema/release/`. The C-043 spike reported the newest release manifest (`2.0.0`), which is wrong during a pre-release. The build script itself stays dependency-free.
- **Flavors:** only the strict sources are embedded (45 files). The tolerant flavor is derived in memory at `Validator::new()` exactly as the schema gate derives it, so a receiver can check "tolerant" without a second copy in the binary.

## Amendment C-048 (2026-10-26) – first dependencies of `oec` (agent, G-3.4 b)

Applied in P4 increment 4 within §1 (no new crate):

- **thiserror** 2 (2.0.21, with `thiserror-impl`) for `oec::Error`; **log** 0.4 (0.4.34), default features, for SDK diagnostics. Both are workspace dependencies. `oec` also uses serde_json directly (container storage) and forwards `strict-validation` to `oec-core`.
- **Tree of `oec`:** 22 crates in the default build, 74 with `strict-validation` (unique crates, normal edges). `cargo deny check`: advisories, bans, licenses and sources ok; no exception and no `ignore` entry (both crates are MIT OR Apache-2.0, G-4.4).
- **Not yet:** tokio, rustls/tokio-rustls and rustls-pki-types come with the transport (increment 5), uuid `v4`/`v5` with the default DataSetClassId (increment 6), the `tracing` feature with the Syslog bridge (ADR-0020 §11). Base64 for the container secrets is a 30-line decoder in `oec` instead of a crate (only decoding, standard alphabet).
