# P3 spikes for D2 (C-043)

These are not product code. Each spike backs one decision of ADR-0023 and runs offline in `../../check.sh` (step 5 of `oec/check.sh`).

| Spike | Shows | Result (C-043, macOS arm64, cargo 1.89) |
|---|---|---|
| `schema-embed/` | ADR-0019 §7: D2 embeds `oec/schema/v2` at compile time. `build.rs` writes one `include_str!` per file and takes version and baseline from the newest manifest in `oec/schema/release/`. The registry is built from memory (jsonschema-rs, no default features, so there is no file or HTTP resolution), and validators are built lazily per entry schema. | 45 schemas (272 KB) for release `2.0.0`. All 630 strict cases of `oec/schema/dist/cases.json` give the same verdicts. Registry 2.5 ms; all 630 cases including the lazy validator builds 38 ms (release build). Cost of validation in a stripped LTO binary: +3.1 MB (0.35 → 3.45 MB). So validation is the cargo feature `strict-validation` (ADR-0020 §13). |
| `mqtt-codec/` | Q-026 option (a): the MQTT 3.1.1 client subset of OEC as a sans-IO codec with no dependencies. It covers CONNECT with a new Will per connect, PUBLISH QoS 0/1 without retain, SUBSCRIBE/UNSUBSCRIBE, PUBACK, PING, DISCONNECT, an incremental decoder with a maximum packet size, and keep alive as a pure timer. | 307 lines of code and 8 byte-exact tests (hand-encoded packets, remaining-length boundaries of table 2.4, any chunking, malformed input). A product version adds session state (packet ids, QoS 1 inflight), fuzzing and broker interop tests. The estimate is 1–1.5k lines with tests. |

The Cargo lock files pin the crate versions that are cached locally. `schema-embed` uses the same versions as `oec/schema/tools/rust-check`, so `cargo test --offline --locked` needs no network.
