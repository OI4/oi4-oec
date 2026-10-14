# SDK architecture comparison (Q-001)

> **Note (C-006):** written against guideline baseline `c85ce2a`. Section numbers and terms (e.g. `CorrelationId`, `MSG`, namespace `Oi4`) predate baseline `5f070bb` (GL-ADR-001 … 011). The conclusions still hold; for current protocol facts see `guideline-digest.md`.

Status: decision input for the product owner (C-003). Details and background: `sdk-options.md`.
Focus aspects (PO, C-003):

1. **Stability & performance**
2. **Acceptance among edge developers** (C-heavy industrial developers, Rust rising, Go and TypeScript not to be excluded)
3. **Ease of use** (how much protocol complexity is hidden, how easy is installation)
4. **Maintenance effort on our side** (initial build plus keeping up with guideline changes)

Rating: `++` very good · `+` good · `o` neutral · `-` weak · `--` very weak. For maintenance, `++` means the *least* effort.

## Overview

| # | Solution | Languages covered | 1. Stability & performance | 2. Acceptance edge devs | 3. Ease of use | 4. Maintenance (our side) |
|---|----------|-------------------|:--:|:--:|:--:|:--:|
| S1 | Rust core + C ABI + native bindings (P/Invoke, napi-rs, cgo) | Rust, C/C++, C#, TS, Go | ++ | + | + | o |
| S2 | Rust sans-IO core; C ABI for C/C#, **WASM** for TS/Go | Rust, C/C++, C#, TS, Go | + | + | + | o |
| S3 | Separate native SDK per language | any | o | ++ | ++ | -- |
| S4 | Rust SDK + C API only | Rust, C/C++ | ++ | o | + | ++ |
| S5 | C core + bindings | C/C++, (C#, TS, Go) | o | + | o | - |
| S6 | Sidecar / local agent (HTTP/gRPC) | any | o | - | o | + |
| S7 | Code generation from D1 only (types + validators) | any | + | o | -- | + |
| S8 | Go core exported as C library (c-shared) | Go, (C, C#) | o | o | o | o |
| S9 | Rust core as **local co-process** in the app container; thin language wrappers talk to it via local IPC (HTTP/gRPC); C/Rust in-process via S4 | any (+ C/Rust native) | + | o | + | + |

**Agent recommendation (updated after S9):** start with **S4**, the Rust SDK plus the C API. It is the common first stage of S1, S2 and S9.

For further languages, the main candidates are:
- **S9**: same core as a co-process. Least maintenance, easy to add new languages.
- **S1/S2**: in-process bindings. Best performance, no second process.

Decide this with a small spike before the second language. Neither choice causes rework in the Rust core, as long as the core is layered `core → runtime → front ends (Rust API, C ABI, daemon)`.

---

## S1 – Rust core + C ABI + native bindings

One Rust implementation of all protocol logic and runtime (MQTT/TLS, timers, `/etc/oi4` bootstrap). It exposes a stable C ABI (`oec.h` + `liboec`). Thin idiomatic wrappers sit on top: C# via P/Invoke, TypeScript via napi-rs (native addon), Go via cgo.

- **Pros**
  - Protocol logic implemented once, so behavior is the same in all languages and there is one place to fix bugs.
  - Rust gives memory safety and high performance with a small footprint.
  - The C API serves the main audience directly.
  - Supports ARMv7 (best effort OK).
- **Cons**
  - Native binaries per OS/arch for every binding make the build and release matrix large.
  - Bugs at the FFI boundary (threads, callbacks, memory ownership) are harder to debug.
  - Go via cgo is unpopular: it needs a C cross-toolchain and gives no pure static cross-compile.
  - Wrappers are still needed per language.

## S2 – Rust sans-IO core, C ABI for C/C#, WASM for TypeScript/Go

Same core as S1, but the core does no I/O. For TypeScript and Go, the core is compiled once to **WebAssembly**. The host language does the I/O with its native MQTT client (mqtt.js, paho) and runs the WASM core (Node: built-in; Go: wazero, pure Go without cgo).

- **Pros**
  - Logic implemented once.
  - TypeScript and Go packages are pure (no native prebuilds, no cgo), so installation is trivial and Go cross-compiles normally.
  - One WASM artifact runs on every OS/arch, including ARMv7.
  - The sans-IO core is deterministic and very easy to test.
- **Cons**
  - Two binding technologies to maintain (C ABI and WASM interface).
  - The I/O layer (connect, TLS, timers, bootstrap) must be written thinly in each host language.
  - Slight WASM overhead and JSON copies across the boundary; irrelevant at typical edge message rates.
  - Fewer people know the WASM tooling.

## S3 – Separate native SDK per language

Independent, idiomatic SDKs (Rust, C, C#, Go, TypeScript). They share only the D1 schemas and are verified by D7.

- **Pros**
  - Most natural for each community: pure packages, native debugging, idiomatic APIs.
  - Highest acceptance.
- **Cons**
  - All of the protocol logic (lifecycle, Get/Set, pagination, PublicationList, ...) is built and maintained up to 5 times.
  - Every guideline change means 5 changes, so implementations drift apart.
  - Quality varies per language.
  - Needs maintainers with deep knowledge of each language.

## S4 – Rust SDK + C API only

The S1 core without further bindings: a Rust API plus a C API for C/C++.

- **Pros**
  - Least maintenance and the highest stability and performance.
  - Covers the main audience: industrial C developers and the rising Rust community.
  - A clean basis that can later grow into S1/S2 without rework.
- **Cons**
  - Excludes Go, TypeScript and C# developers for now; they could only use the C API by hand.

## S5 – C core + bindings

The core is written in C (or C++) with a C ABI, and bindings are added like in S1.

- **Pros**
  - Maximum acceptance among C developers, who can read and debug the core themselves.
  - Same binding paths as S1.
  - Very high performance.
- **Cons**
  - Network-facing JSON/MQTT parsing in C is a constant source of memory-safety and security bugs.
  - Slow development and high test effort.
  - Bindings have the same cons as S1.
  - It competes with Rust, which the PO prefers.

## S6 – Sidecar / local agent

One OEC agent (any implementation language) per app speaks MQTT/OEC. The app talks to it via a local HTTP/gRPC API, with generated clients per language.

- **Pros**
  - Language-agnostic: any language with HTTP/gRPC works.
  - One implementation.
- **Cons**
  - The guideline model has one MQTT identity per container (hostname = ClientId, `<containername>.pem`). A separate agent container breaks this or forces awkward container setups.
  - An extra process means more failure modes and latency.
  - Get/Set callbacks into the app need a second protocol.
  - Deployment becomes more complex.
  - Low acceptance on the edge.

## S7 – Code generation only

Types and validators per language are generated from the D1 JSON Schemas. The rest is left to the app developer.

- **Pros**
  - Cheap and broad language coverage.
  - Always consistent with the schema.
- **Cons**
  - Hides almost none of the real complexity: lifecycle, topics, Get/Set, pagination, PublicationList, bootstrap.
  - Misses the main goal of D2.
  - Useful only as a *supplement*, e.g. for consumers.

## S8 – Go core exported as C library

The SDK is written in Go and exported via `-buildmode=c-shared` for C and C#.

- **Pros**
  - Go developers get a native SDK.
  - Easy cross-compiling for the Go side.
- **Cons**
  - Every library embeds a full Go runtime (size, GC, signal handling), and only one Go runtime per process is supported.
  - Callbacks from C into Go are awkward.
  - TypeScript would still need another binding.
  - Weaker fit for the C/Rust-oriented audience.

---

## S9 – Rust core as local co-process with thin language wrappers (PO proposal, C-003)

The Rust core runs as its own **process inside the app container**. It holds the MQTT connection, the identity, the lifecycle and the resource logic. Each language has a thin wrapper that only does the language binding and abstraction. The wrapper talks to the core over a local interface:
- gRPC with bidirectional streams, or
- HTTP/JSON plus a push channel (WebSocket/SSE),

over a Unix domain socket or named pipe, alternatively loopback plus a token. This is similar to the Dapr sidecar pattern, but inside the *same* container.

### Feasibility: yes, technically straightforward

- The core is the same Rust code as S4. The daemon is just another front end on top of the library (`oec-core` → `oec` runtime → `oecd` daemon).
- **Same container is essential.** It keeps the guideline identity model intact: same hostname/ClientId, same mounted `/etc/oi4` and `/run/secrets`. A *separate* sidecar container (S6) would break this.
- **Push direction is required.** Incoming `Get`/`Set`/`Del`, Config changes and subscribed data must reach the app. Plain request/response HTTP is not enough; this needs gRPC streaming or WebSocket/SSE.
- **Interface definition** in Protobuf (gRPC) or OpenAPI/AsyncAPI. Clients for C#, Go, TypeScript, Python, Java and C++ can be **generated**. Generated code is still a raw client, so a small idiomatic layer on top remains useful.
- **C:** gRPC/HTTP streaming from plain C is cumbersome. C (and Rust) therefore use the core in-process via the C API (S4). For C, S9 is a complement, not a replacement.

### Points to solve (not blockers)

| Topic | Issue | Mitigation |
|-------|-------|------------|
| **Liveness / Will semantics** | In-process, an app crash drops the MQTT connection and the broker sends the Will (`FAILURE_1`). With a co-process, the core keeps the connection, so a hung or crashed app would still look healthy. | The core watches the wrapper session (stream + heartbeat). If it is lost, the core drops the MQTT connection uncleanly so the broker sends the Will, or it publishes a Health degradation. The app health score comes from the app. |
| Process supervision | Two processes in one container (multi-process containers are frowned upon). | The wrapper starts the core as a child process (or tini/s6 does), and the container exits if one of them dies. |
| Distribution | The core binary per OS/arch must reach the container. | Built **once** per OS/arch for all languages: OEC base images (`FROM oec-runtime`) or bundled/downloaded by the wrapper package. |
| Versioning | Wrapper and core are separate artifacts. | Versioned interface (`v1`), compatibility check in the handshake. |
| Security | Local API must not be reachable by others. | UDS/named pipe (file permissions) or loopback + random token. Never bind to `0.0.0.0`. |
| Performance | Extra serialization hop (app → IPC → core → MQTT). | Sub-millisecond on UDS, thousands of messages/s. Uncritical for typical OEC rates; less suited for very high-frequency data. |
| Debugging | Two processes. | Easier than FFI: separate logs, the interface can be traced, the core can be tested on its own. |

### Rating against the criteria

- **Stability & performance (`+`):** The Rust core is stable and crash-isolated from the app; an app memory bug cannot corrupt the core. The cost is an extra hop and the liveness topic above, so it is slightly behind S1/S4 (`++`).
- **Acceptance among edge developers (`o`):** The pattern is known from cloud-native (Dapr, Envoy), and wrappers feel native and install without a C toolchain or cgo. But "a second process in my container" and an extra local API meet skepticism in classic OT/edge circles. For C developers this is irrelevant because they use the C API.
- **Ease of use (`+`):** The wrappers can offer exactly the same idiomatic API as S1. For app developers it adds a little complexity (base image or binary, one more process to understand); with an OEC base image this is almost invisible.
- **Maintenance (`+`):** This is the strongest argument. Logic exists once. There is **one** native build matrix (core × OS/arch) instead of language × OS/arch, no FFI or ABI per language, wrappers are mostly generated, and new languages are cheap. The interface definition and its versioning must be maintained.

### Classification

S9 is the "good" variant of the sidecar idea (S6): the same container fixes the identity problem, and the Rust core keeps the logic in one place. Compared with S1/S2 it trades a little performance and a second process for **noticeably less maintenance** and easier support of new languages. It combines well with S4:

**Rust + C use the core in-process (S4); C#, Go, TypeScript, ... use the same core as a co-process (S9).**

## Notes common to all options

- D1 (JSON Schema) and D7 (conformance tool) are the same in every option. D7 verifies every language binding in the same way (ADR-0006).
- Supporting more languages later is cheapest with S1/S2 (only a wrapper) and most expensive with S3 (a full SDK).
- ARMv7 is best effort (Q-008). Rust, C, Go and WASM handle it well. For Node 24+ native addons, ARMv7 is not officially available, which favors WASM for TypeScript.
