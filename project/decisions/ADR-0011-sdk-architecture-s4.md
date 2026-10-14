# ADR-0011 – SDK architecture: start with S4 (Rust SDK + C API)

- Status: accepted
- Date: 2026-10-12
- Deciders: product owner (Thomas), C-004
- Related: Q-001, Q-008, `research/sdk-architecture-comparison.md`, `research/sdk-options.md`

## Context

Primary audience: industrial edge developers (C-heavy, Rust rising). Go and TypeScript must not be excluded long-term. ARMv7 is best effort. Nine options (S1–S9) were compared on stability/performance, acceptance, ease of use and maintenance.

## Decision

Start with **S4**: one SDK written in **Rust**, offered as
- an idiomatic **Rust API**, and
- a stable **C API** (C ABI + generated header) for C/C++ developers.

The SDK is layered so that further languages can be added later without rework:

1. `oec-core` – sans-IO protocol logic (identity, topics, codec, resources, Get/Set/Del, pagination, PublicationList, lifecycle state machine); no sockets, no threads, no async runtime.
2. `oec` – runtime (MQTT 3.1.1 + TLS, timers, container storage bootstrap, reconnect).
3. Front ends – Rust API, C ABI (`oec-ffi`); later possibly a daemon (S9) or bindings (S1/S2).

## Consequences

- Further languages (C#, Go, TypeScript, …) are decided later via a spike between **S9** (core as co-process with thin wrappers) and **S1/S2** (in-process native/WASM bindings).
- The C ABI must be designed for foreign use from the start: opaque handles, plain structs, UTF-8 strings/JSON buffers, explicit free functions, callbacks with user-data pointers, no async or Rust types in the ABI.
- The async runtime of the `oec` runtime must not leak into the C API (C API = blocking calls + callbacks, background thread owned by the SDK).
- Exact crate choices (MQTT client, TLS, JSON Schema) are decided in P3/P4 under G-3.4/G-4.4.
- The conformance tool (D7) is written in Go (ADR-0013).

## Amendment C-006 (guideline baseline `5f070bb`)

The guideline changes (GL-ADR-001 … 011) do not change the decision, they strengthen it:

- **Compliance levels (GL-ADR-009):** Level 1 targets resource-constrained **smart devices** that are not containers. The C API is the natural path for them. Therefore the SDK must work **without the container storage contract** (explicit configuration of broker, credentials and identity) and must let the developer choose the compliance level (L1 = Pub MAM/Health + Birth/Will; L2/L3 enable Get/Set and the other resources). Bootstrap from `/etc/oi4` is an optional convenience for containerized apps.
- **OPC UA 1.05.04 alignment (GL-ADR-004/005/006/007):** fixed content masks, `ReplyTo`, `DataSetWriterName`/`WriterGroupName` and Actions are pure protocol logic in `oec-core`; the public API should hide them completely.
- Whether a `no_std`/small-footprint build of `oec-core` for microcontroller-class devices is needed is an open question (Q-015).

## Amendment C-007 (ADR-0016)

`oec-core` must be `no_std`-compatible (`alloc` optional, no `std`-only dependencies in the codec/topic/identity code) so that the micro SDK (D9) can reuse it. The micro SDK itself is decided separately (ADR-0016, Q-016).
