# ADR-0023 – Dependencies of the Rust SDK (D2)

- Status: **accepted** (PO, C-045 review 2026-10-26; proposed in C-043). Amendment C-047 (feature sets, version source `schema.go`, tolerant flavor in memory), C-048 (thiserror, log in `oec`) C-049 (codec of Q-026 in `oec-core`, proptest as dev-dependency, no cargo-fuzz) and C-050 (transport: tokio, rustls, tokio-rustls, rustls-pki-types; pkcs8 for keys with passphrase; crypto features; rcgen as dev-dependency) and C-086 (package `oec-registry`: no new crate in increment 1, hyper stack decided for increment 3) by the agent (G-3.4 b).
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

## Amendment C-049 (2026-10-26) – MQTT codec of Q-026 and the property-test dev dependency (agent, G-3.4 b)

Applied in P4 increment 5a within this ADR (libraries within the dependency rules, G-3.4 b). Details: `log/2026-10-26_C-049_mqtt-session-lifecycle.md`.

- **Q-026 option (a) realized:** the MQTT 3.1.1 codec and the client session are `oec_core::mqtt` (packets, canonical codec, session state machine), written new from the spike `mqtt-codec` after review (the spike stays as the C-043 evidence). Product size: about 2.1k lines in `oec-core/src/mqtt/` (with docs) plus about 1.6k lines of codec, session and property tests. No new runtime dependency: the default tree of `oec-core` stays at 20 crates (81 with `strict-validation`).
- **Fuzzing = proptest, no cargo-fuzz (decides the "cargo-fuzz or proptest" of the Consequences):** proptest 1.12 (MIT OR Apache-2.0) is a **dev-dependency** of `oec-core` only, workspace-managed, `default-features = false` with `std` (no `fork`/`timeout`, no `bit-set` persistence of failures). It runs on stable in the normal `cargo test` of the gate (codec round trips, garbage and mutation inputs, chunked streams, random session runs). cargo-fuzz needs nightly and libFuzzer (a C++ build); a fuzz target would sit outside the gate, so it is not set up. It can be added later as an optional `fuzz/` crate outside the workspace without touching the gate.
- **Dev tree:** proptest adds 10 crates to `Cargo.lock` (chacha20, core_detect, cpufeatures 0.3, getrandom 0.4, r-efi 6, rand 0.10, rand_core 0.10, rand_xorshift, unarray, proptest), 33 crates in `oec-core` with dev edges. None of them is linked into the SDK. r-efi is `MIT OR Apache-2.0 OR LGPL-2.1-or-later` and is used under MIT/Apache-2.0 (G-4.4). `cargo deny check`: advisories, bans, licenses and sources ok; no exception and no `ignore` entry. The only warning is the pre-existing duplicate of `syn` (not a TLS or crypto crate).
- **Still not yet:** tokio, rustls/tokio-rustls and rustls-pki-types come with the transport in increment 5b (C-050).

## Amendment C-050 (2026-10-26) – transport dependencies of `oec` (agent, G-3.4 b)

Applied in P4 increment 5b within this ADR (libraries within the dependency rules, G-3.4 b). Details: `log/2026-10-26_C-050_runtime-tokio-rustls.md`, REVIEW A-32/A-33/A-40.

- **Crates of §1, smallest feature sets (workspace dependencies):**
  - tokio 1.54 (1.54.1), `default-features = false`, features `rt`, `net`, `time`, `sync`, `signal` exactly as §1 (no `macros`, no `io-util`, no `rt-multi-thread`: the runtime uses `current_thread`, and the read/write helpers are small `poll_fn`s);
  - rustls 0.23 (0.23.46), `default-features = false`, features `std`, `tls12`, `logging` (TLS 1.2 is needed for brokers without TLS 1.3, RIG 4.1; TLS 1.0/1.1 do not exist in rustls, REQ-MQTT-005);
  - tokio-rustls 0.26 (0.26.6), `default-features = false` (the provider comes from rustls);
  - rustls-pki-types 1.15 (`std`) with `PemObject` for the CA, the client certificate and the key; rustls-webpki is 0.103.16 (the fixed line of the advisories in the Context).
- **Crypto provider as cargo features of `oec` (§1 "Crypto provider"):** `crypto-aws-lc-rs` (default: `rustls/aws_lc_rs` + `rustls/prefer-post-quantum`) and `crypto-ring` (`rustls/ring`). With both enabled, ring wins (a build with `--features crypto-ring` on top of the default stays valid); with neither, a `compile_error!` names the features. The gate runs clippy with `--no-default-features --features crypto-ring` so the ring path keeps compiling; the runtime tests pass with both providers (run locally).
- **New crate within the rules: pkcs8 0.11** (RustCrypto, `Apache-2.0 OR MIT`), features `std`, `pem`, `encryption`. It decrypts PKCS#8 `ENCRYPTED PRIVATE KEY` files with the `passphrase` of the container storage (REQ-CNT-008/009): PBES2 with PBKDF2 (HMAC-SHA-224/256/384/512) or scrypt and AES-CBC/GCM, as written by OpenSSL ≥ 1.1 (PRF HMAC-SHA-256) (`openssl pkcs8 -topk8 -v2 aes-256-cbc`, and with `-scrypt`; checked). rustls-pki-types reads only unencrypted keys. Not supported, with a clear `Error::Bootstrap`: the legacy OpenSSL encryption (`Proc-Type: 4,ENCRYPTED` in a PKCS#1/SEC1 PEM) PBES1 (MD5/DES) and PBKDF2 with HMAC-SHA-1 (the pkcs5 feature `sha1-insecure` stays off); OpenSSL converts all of them with `openssl pkcs8 -topk8 -v2 aes-256-cbc`. pkcs8 brings pkcs5, pbkdf2, scrypt, salsa20, aes, cbc, sha2, hmac, der, spki and their RustCrypto helpers (all `MIT OR Apache-2.0`).
- **Dev dependency: rcgen 0.14** (`MIT OR Apache-2.0`), features `pem`, `aws_lc_rs`, for the test PKI of `oec/tests/runtime.rs` (CA, server and client certificates generated per test run; nothing is committed). It is not linked into the SDK. The test broker uses rustls directly (no tokio), so no other dev dependency is needed.
- **Tree of `oec`** (unique crates, normal edges): **73** in the default build (was 22), **122** with `strict-validation` (was 74), 73 with `crypto-ring` (aws-lc-rs and aws-lc-sys replaced by ring and getrandom). `oec-core` is unchanged (18). `Cargo.lock`: 191 packages (was 106). Direct dependencies of `oec`: 9 (oec-core, thiserror, log, serde_json, tokio, rustls, tokio-rustls, rustls-pki-types, pkcs8), within the "about 11" of the Consequences. Most of the growth is tokio (mio, socket2, signal-hook-registry, libc) and the RustCrypto stack of pkcs8.
- **Licenses and policy:** aws-lc-sys (`ISC AND (Apache-2.0 OR ISC) AND Apache-2.0 AND MIT AND BSD-3-Clause AND …`), aws-lc-rs (`ISC AND (Apache-2.0 OR ISC)`), ring (`Apache-2.0 AND ISC`), rustls-webpki and untrusted (ISC), subtle (BSD-3-Clause): all within the allowlist of §2; `deny.toml` is unchanged. `cargo deny check`: advisories, bans, licenses and sources ok, **no exception and no `ignore` entry**. The remaining warnings are duplicates of non-TLS crates only (getrandom, r-efi, syn, windows-sys); rustls, webpki and the crypto provider exist once per build.
- **Build:** aws-lc-sys builds its C code with `cc` (no CMake needed on macOS/Linux; checked on macOS arm64). Windows x86-64 needs NASM or the `prebuilt-nasm` feature of aws-lc-sys; ARMv7 uses `crypto-ring` (Q-008). Decided with the CI (§2 "Cross builds").
- **Not used:** `rustls-native-certs`/`webpki-roots` (trust only from the storage, ADR-0020 amendment C-050), `rustls-pemfile` (unmaintained), `signal-hook`, `tokio-util`, `futures`.


## Amendment C-053 (2026-10-27) – no new dependency in increment 6a (agent, G-3.4 b)

P4 increment 6a (provider side, ADR-0026 §12) adds no crate and no feature: the provider, the pagination and the writer-id table use `serde_json` and the standard library; the store-key hash is the FNV-1a function already in `oec`. `Cargo.lock` and `deny.toml` are unchanged; `cargo deny check` passes without exception. Expected in increment 6b: the `v5` feature of `uuid` for the publisher-defined DataSetClassId (ADR-0020 §6); it will be recorded here when it is added.

## Amendment C-054 (2026-10-27) – uuid feature `v5` (agent, G-3.4 b)

P4 increment 6b enables the feature `v5` of `uuid` (workspace dependency, used by `oec-core` and `oec`) for the publisher-defined DataSetClassId (ADR-0020 §6, REQ-MSG-013; ADR-0026 §13.3). It pulls in **sha1_smol 1.0.1** (BSD-3-Clause, no dependencies of its own), which is within the license allowlist of §2. SHA-1 is used only as the name-based UUID function of RFC 9562 §5.5, not for security. `Cargo.lock`: 192 packages (was 191); `deny.toml` unchanged; `cargo deny check` passes with **no exception and no `ignore` entry**. No other crate or feature was added in 6b: the dead band, the cyclic publication and the MetaDataVersions use `serde_json` and the standard library.

## Amendment C-055 (2026-10-27) – no new dependency in increment 7a (agent, G-3.4 b)

P4 increment 7a (write side, ADR-0026 §14) adds no crate and no feature: Status codes, request parsing, the `Set PublicationList` overrides and `Set Data` use `serde`, `serde_json` and the standard library. `Cargo.lock` and `deny.toml` are unchanged; `cargo deny check` passes without exception. Increment 7b (Config with secrets under `/run/secrets/oi4`) is expected to need none either; if it does, it will be recorded here.

## Amendment C-056 (2026-10-27) – regex for `Validation.Pattern` (agent, G-3.4 b)

P4 increment 7b checks the `Pattern` of a Config object (REQ-RES-CONFIG-008; MG 8.3.3.1) and adds **regex 1.13.1** as a runtime dependency of `oec-core` (workspace dependency `regex = { version = "1", default-features = false, features = ["std", "unicode"] }`). It brings regex-automata 0.4.18, regex-syntax 0.8.11 and memchr 2.8.3 (regex*: MIT OR Apache-2.0; memchr: Unlicense OR MIT), all within the allowlist of §2. The `perf` features (aho-corasick, SIMD literal search) are off: patterns are short and only checked on a `Set`.
- **Why regex:** I/O-free, the de-facto standard, linear-time matching (no backtracking, so a pattern cannot blow up), syntax close to the RE2 of D7 (`regexp`); it was already in `Cargo.lock` via jsonschema under `strict-validation`. Patterns come only from the owner's definition (Validation is read-only on the bus), never from a request.
- **Tree:** `oec-core` 22 unique crates in the default build (was 18), `oec` 77 (was 73). `Cargo.lock`: 192 packages, unchanged. `deny.toml` unchanged; `cargo deny check` passes with **no exception and no `ignore` entry**.
- Masking, persistence and the secret reload use `serde_json` and the standard library (`std::os::unix::fs` for file modes); no other crate or feature was added.

## Amendment C-057 (2026-10-27) – no new dependency in increment 7c (agent, G-3.4 b)

P4 increment 7c (ReferenceDesignation, Syslog bridge, Generic events) adds **no crate and no feature**. The Syslog bridge implements `log::Log` of **log** (already a dependency of `oec`, ADR-0023 amendment C-050); the rate limit, the reentrancy guard and the HOSTNAME/TAG use the standard library; the ReferenceDesignation table and the Generic `Details` use `serde`/`serde_json`. `Cargo.lock` unchanged (192 packages); `cargo deny check` passes with **no exception and no `ignore` entry**.

## Amendment C-058 (2026-10-27) – no new dependency in increment 8a (agent, G-3.4 b)

P4 increment 8a (the consumer core) adds **no crate and no feature**. The sans-IO consumer in `oec-core` uses `serde_json`, `time` and the standard library; the waiting in `oec` uses `tokio::sync::oneshot` (feature `sync` already enabled) and a thread park for the `_blocking` forms; the routing mode of the test broker uses the existing dev dependencies. `Cargo.lock` unchanged (192 packages); `cargo deny check` passes with **no exception and no `ignore` entry**.

## Amendment C-059 (2026-10-28) – no new dependency in increment 8b (agent, G-3.4 b)

P4 increment 8b (SubscriptionList, `Subscription::configurable`/`interval`, example app `oec/examples/demo.rs`) adds **no crate and no feature**: the list, its overrides and their persistence use `serde`/`serde_json` and the standard library; the example uses only `oec` and the standard library (no `[dev-dependencies]` change). `Cargo.lock` unchanged (192 packages); `cargo deny check` passes with **no exception and no `ignore` entry**.

## Amendment C-060 (2026-10-28) – no new dependency in increment 8c (agent, G-3.4 b)

P4 increment 8c (the Action `NewDataSetWriterId`, the ported examples, the walker in `oec/tests/citations.rs`) adds **no crate and no feature**: responder, reservations and requester use `serde`/`serde_json`, `time` and the standard library; the example `consumer.rs` uses tokio (`rt`, `sync`, `time`), a normal dependency of `oec` with these features enabled since C-050. The deleted sketch had no dependencies; its lock file is gone with it. `Cargo.lock` unchanged (192 packages); `cargo deny check` passes with **no exception and no `ignore` entry**.

## Amendment C-061 (2026-10-28) – increment 10: `oec-ffi` adds no crate; cbindgen pinned (agent, G-3.4 b)

P4 increment 10 (the C ABI `oec-ffi`) adds **no crate and no feature**. Runtime dependencies of `oec-ffi`: `oec` (its crypto and `strict-validation` features are forwarded), `serde`, `serde_json` and `tokio` (the runtime of `oec_app_start`; the workspace features `rt`, `net`, `time`, `sync`, `signal` that `oec` already uses). Dev dependencies: `oec-core`, `rcgen`, `rustls`, `rustls-pki-types`, `pkcs8` – the ones of the in-process TLS test broker of `oec/tests`, which `oec-ffi/tests/c.rs` includes as a `#[path]` module. `Cargo.lock` grows by the package `oec-ffi` only (192 → 193 packages); `cargo deny check` passes with **no exception and no `ignore` entry**.

**cbindgen** (decided as a CLI tool, table above): pinned to **0.29.4** (`cbindgen_version` in `oec/sdk/check.sh`); the gate fails if it is missing or reports another version, regenerates the header into a temporary file and fails on any difference to the committed `oec-ffi/include/oec.h`. Install: `cargo install cbindgen --version 0.29.4 --locked`. It is neither a build dependency nor linked; its MPL-2.0 license does not reach the generated header (our MIT code).

**C compiler and sanitizers** (test tools, not dependencies): the system `cc` (or `$CC`) builds the C examples and the ABI test program; AddressSanitizer and UBSan come with the toolchain (Apple clang, gcc, clang); `OEC_FFI_REQUIRE_SANITIZERS=1` (default in `oec/sdk/check.sh`) makes a toolchain without them fail the gate instead of skipping.

## Amendment C-086 (2026-11-02) – the package `oec-registry` of P7 (agent, G-3.4 b)

The OEC Registry D5 (ADR-0029) is the Cargo package `oec/registry` (`oec-registry`), outside the SDK workspace like `oec-config` (ADR-0028 §7), with its own `Cargo.lock` and the `deny.toml` of the SDK. Details: `log/2026-11-02_C-086_p7-start-registry.md`, REVIEW A-380, A-383, A-385.

- **Increment 1 (C-086) adds no new crate.** Runtime: `oec` (by path) and `log`; dev: `serde_json` and **jsonschema `=0.60.1` with `default-features = false`** – the pin of ADR-0019 §8, already in the SDK lock through `strict-validation`; it validates `api/asyncapi.json` (later `api/openapi.json`) against the vendored official meta-schemas offline (ADR-0029 §11). `Cargo.lock`: 169 packages, all of them in the SDK lock (the SDK lock pruned to the registry's tree); 78 unique crates in the normal tree, as `oec-config`. `cargo deny check`: advisories, bans, licenses and sources ok, **no exception and no `ignore` entry**.
- **Decided for increment 3 (A-383), only for `oec-registry`:** the HTTP server stack `hyper` 1 (features `server`, `http1`), `hyper-util` (`tokio`) and `http-body-util`, on the `tokio` and `tokio-rustls` of the SDK (all MIT or `MIT OR Apache-2.0`). Reason: REST, Server-Sent Events and TLS on the ports 5798/5799 need a robust HTTP/1.1 parser and streaming; the "small blocking server on `std::net`" of ADR-0028 §3 fits a demo, not a product service with long-lived SSE streams. The exact versions, the tree growth and the self-signed certificate crate (`rcgen` is already a dev dependency of the SDK) are recorded when increment 3 adds them, with `cargo deny` without exception.
- **Not for the SDK:** none of these crates becomes a dependency of `oec-core`, `oec` or `oec-ffi`.
