# SDK options paper (Q-001, Q-002, Q-008) – for discussion in P3

Status: draft for product-owner discussion (C-003). Nothing here is decided.
Facts were checked on 2026-10-12 and are marked as such.

## 1. What the SDK must take off the developer's plate

This is what an app developer should *not* have to know in detail. The guideline is the reference for each item.

| # | Complexity | Guideline |
|---|------------|-----------|
| 1 | Bootstrap from the container storage contract: broker address/port, CA, client cert/key/passphrase, base64 credentials, ClientId = hostname | 6, 7.2 |
| 2 | TLS MQTT 3.1.1 connection, reconnect, keep-alive, QoS 0, no retain, MaxPacketSize | 7 |
| 3 | Identity: Identification Link validation, Oi4Identifier/dnp generation, base64url topic encoding | 3, A3 |
| 4 | Lifecycle: Birth (MAM), Will, Close, Health heartbeat every 60 s and on change | 7.3 |
| 5 | Topic building/parsing per Method × Resource, subscriptions with wildcards | 8 |
| 6 | Envelope details: MessageId, PublisherId, CorrelationId, DataSetClassId, DataSetWriterId allocation, SequenceNumber, Timestamp | 9.2 |
| 7 | Mandatory resources answered automatically: MAM, Health, Profile, PublicationList, ReferenceDesignation (+ Metadata if Data) | 8.2 |
| 8 | Get handling incl. Source/Filter matching, pagination, Locale, MaxPacketSize splitting | 10, 9.3.15 |
| 9 | PublicationList semantics: Mode, Interval, Precisions (deadband), Config flags | 9.3.11 |
| 10 | Set/Del handling with automatic Status event + CorrelationId; Config resource with validation | 10.1.x, 9.3.3 |
| 11 | Events: Syslog/Status/Generic with correct topic and payload, ideally bridged from the normal logger | 9.3.9 |
| 12 | Consumer side: discover participants, typed subscriptions, Get with correlation/timeout, pagination reassembly | 10 |
| 13 | Optional payload validation against D1 (dev mode / strict mode) | – |

Target developer experience: a minimal compliant app in about 20 lines. The developer provides the MAM data and the data points; everything else uses defaults.

## 2. Form of the SDK

| Option | Description | Pros | Cons |
|--------|-------------|------|------|
| **A. Native library** (per language) | App links the SDK and is itself the MQTT client | Matches the guideline model (each app container = one MQTT identity with its own cert/ClientId); no extra moving parts; best performance | One implementation per language |
| B. Sidecar / OEC agent | A separate process/container speaks OEC; the app uses a simple local API (HTTP/gRPC) | Language-agnostic; maximum hiding | Guideline identity is per container (hostname, `<containername>.pem`), which makes a sidecar awkward in plain Docker (no pods); extra deployment and failure modes; latency; Get/Set callbacks into the app become an RPC protocol of its own |
| C. Code generation only | Types/validators generated from D1 for many languages | Cheap multi-language reach | Hides almost none of the behavioral complexity (items 1–12) |
| **A + C hybrid** | One full-featured reference SDK; D1-generated types for other languages later | Focus plus a path to more languages | Second-language SDKs remain thin until built |

**Recommendation:** A (native library), designed so that C (generated types) can be added later. Do not pursue B for now; revisit if many languages are requested.

## 3. Language candidates

Criteria: developer reach in the OT/edge community, ease of use, container footprint, ARMv7, maturity of MQTT 3.1.1 + TLS libraries, JSON Schema 2020-12 support, synergy with the registry and D7.

| Criterion | TypeScript / Node | Go | C# / .NET | Python |
|-----------|-------------------|----|-----------|--------|
| Developer reach / ease | ++ JSON-native, large community | + simple, growing in infra/edge | + strong in industrial/Windows shops | ++ popular with automation/data people |
| MQTT 3.1.1 lib | mqtt.js 5 (mature) | eclipse paho.mqtt.golang (mature) | MQTTnet 5 (mature) | paho-mqtt 2 (mature) |
| JSON Schema 2020-12 | Ajv (2020 mode) | santhosh-tekuri/jsonschema v6 | JsonSchema.Net | python-jsonschema |
| Container footprint | ~60–150 MB | ~10–20 MB (static binary) | ~80–120 MB (AOT smaller) | ~60–150 MB |
| **ARMv7** (checked 2026-10-12) | ⚠ **Node 24+ no longer ships official linux-armv7l builds**; only Node 22 (LTS until ~04/2027) has them | ✅ first class (`GOARM=7`) | ✅ .NET 10 LTS supports linux-arm (until 11/2028) | ✅ (interpreter ok, some native wheels missing) |
| Single binary for D7 on Windows/macOS | possible (Node SEA / Bun), more effort | ✅ trivial cross-compile from one CI runner | ✅ self-contained/AOT per OS | ⚠ (PyInstaller, fragile) |
| Registry synergy (web UI is TS/React anyway) | ++ same language end to end | + backend Go, UI TS | + backend .NET, UI TS | o |

Current toolchain versions (checked 2026-10-12): Go 1.27.2; .NET 10 LTS (11 preview); Node 24 LTS (22 in maintenance).

## 4. ARMv7 – answer to the PO's question (Q-008)

Disadvantages of keeping ARMv7 (32-bit ARM):

1. **It narrows the language choice.** Current Node.js (24+) dropped official ARMv7 builds, so a TypeScript SDK would be stuck on Node 22 (EOL ~04/2027) or depend on unofficial builds. Bun and Deno do not support ARMv7 at all.
2. **Native dependencies** often lack prebuilt 32-bit ARM binaries, so builds become slower or fail (e.g. npm/Python packages with C extensions).
3. **CI and multi-arch images** need QEMU emulation for ARMv7: builds and tests are slow, and some base images no longer publish `arm/v7`.
4. **32-bit limits:** ≤ 4 GB address space and lower performance, which matters little for OEC apps. Time handling (2038 problem) is solved in modern runtimes, though not necessarily on old devices.
5. **Market:** many current edge devices are ARMv8 (64-bit), but installed ARMv7 hardware (older IPCs, Raspberry Pi 2/3 with 32-bit OS) still exists in OI4 member products.

Advantage of keeping it: the guideline (11.1) names ARMv7, and existing devices of members stay supported.

→ If ARMv7 stays a hard requirement, **Go or .NET** are the safe SDK choices; with TypeScript, ARMv7 would be "best effort".

## 5. Candidate constellations

| | SDK (D2) + demos | Registry (D5) | Conformance tool (D7) | Comment |
|---|---|---|---|---|
| **K1** | Go | Go backend (on SDK) + React/TS UI embedded in the binary | Go, separate module, **no import of SDK code** (CI check) | Smallest images, ARMv7 safe, one CI toolchain; Go less familiar to some app developers |
| **K2** | TypeScript (Node) | Node backend (on SDK) + React UI | Go | Best reach and full-stack synergy; ARMv7 only best effort; different languages make D7 independence natural |
| **K3** | C# / .NET | ASP.NET backend + React UI | Go or .NET | Good fit for Windows-centric industrial developers; heavier images |
| K4 | Go **and** TypeScript | as K1 | Go | Twice the SDK effort; only after one SDK is stable |

**Agent recommendation:** **K1** if ARMv7 remains required (it does per guideline 11.1), with a TypeScript SDK as a possible second SDK later (K4) based on D1-generated types. **K2** if reach among web/JS developers matters more than ARMv7.

## 6. Repository layout (Q-002)

The existing GitHub organization is `OI4`. Legacy repos `oi4-oec-service`, `oi4-oec-service-go`, `oi4-oec-demo` and `oi4-oec-registry` exist there, and the docs are in `OI4/oi4-oec-documentation`.

| Option | Description | Pros | Cons |
|--------|-------------|------|------|
| **R1 Monorepo** (new repo, e.g. `OI4/oi4-oec`) | `/project`, `/schema`, `/sdk-<lang>`, `/conformance`, `/demos/weather`, `/demos/logger`, `/registry`, `/dev` (compose, dev PKI) | Atomic changes across schema/SDK/tool, one CI, simple for agents, one issue tracker, consistent versioning | Larger repo; per-artifact release tagging needed (`sdk-go/v2.0.0` etc.) |
| R2 Polyrepo (new repo per artifact) | `oi4-oec-schema`, `oi4-oec-sdk-go`, `oi4-oec-conformance`, ... | Independent release cycles; clear ownership | Cross-repo changes are painful early on; `project/` would need its own home |
| R3 Reuse legacy repos with a v2 branch | v2 in `oi4-oec-service-go` etc. | Keeps names and stars | Mixes histories (against the clean-room idea); legacy structure does not fit the new deliverables (schema, D7) |

The docs could either stay in `oi4-oec-documentation`, which already has the guideline and a deploy pipeline, or move into the monorepo.

**Agent recommendation:**
- R1 monorepo for all code plus `project/`.
- Docs stay in `oi4-oec-documentation`, so the guideline and the user docs live in one Docusaurus site.
- Schema `$id`s point to a stable published URL (e.g. GitHub Pages of the docs site or of the monorepo).
- Legacy repos get a README note pointing to v2 when v2 is published.

## 7. Questions for the PO

1. Does ARMv7 remain a hard requirement for SDK-based apps (Q-008)?
2. Which developer group is the primary audience for the SDK (web/JS, Go/infra, .NET/industrial, Python/data)?
3. One SDK first (recommended), or two in parallel?
4. Repo layout: R1/R2/R3? Name of the new repo? Do the docs stay in `oi4-oec-documentation`?

## 8. PO input (C-003) and revised recommendation

PO answers:
- **ARMv7:** best effort. Support it where the toolchain allows, but it does not rule out a language (Q-008 decided).
- **Primary audience:** developers in the industrial environment. They are C-heavy, and Rust is gaining popularity.

This shifts the picture towards **Rust** and a **C-compatible API**.

| Aspect | Rust core + C API ("K5") |
|--------|--------------------------|
| Idea | SDK written in Rust (idiomatic Rust API) plus a stable **C ABI** (`liboec` + header generated with cbindgen), so C/C++ developers can use it directly. Later bindings (Python via PyO3, .NET via P/Invoke, Node via napi-rs, Go via cgo) build on the same core. |
| MQTT 3.1.1 + TLS | `rumqttc` (3.1.1 and 5, rustls) |
| JSON / Schema 2020-12 | `serde`/`serde_json`; `jsonschema` crate (2020-12 support) for optional validation |
| ARMv7 | `armv7-unknown-linux-gnueabihf` is a Tier 2 target with host tools → fine |
| Footprint | static binaries, a few MB; small images (distroless/scratch) |
| Memory safety | big advantage over a pure C/C++ SDK (network-facing parsing code) |
| Risks | Rust learning curve for C developers (mitigated by the C API); async runtime choice (tokio) must not leak into the C API (C API = blocking calls + callbacks + own background thread) |

Not recommended: a **pure C SDK**. It is the hardest to make easy and safe (manual memory management in JSON/MQTT parsing), it is slow to develop, and it would compete with the Rust option.

Conformance tool (D7) under K5:
- **D7-a: Rust, separate workspace/crate with no dependency on SDK crates** (enforced by a CI check on the dependency tree). One toolchain for the team; cross-builds for Windows/macOS/Linux in the CI matrix.
- D7-b: Go. A second language guarantees independence (shares only the schema files, ADR-0006) and gives trivial cross-compilation, but adds a second toolchain.

Demos and registry under K5:
- Weather service in Rust (idiomatic API).
- Logger service **in C using the C API**. This proves and documents the C path for the main audience.
- Registry backend in Rust (on the SDK, e.g. `axum`), UI in React/TypeScript and embedded into the binary.

**Revised agent recommendation:** K5 (Rust core + C API), D7-a, monorepo R1.

## 9. One SDK for many languages? (PO question C-003)

PO: "Rust is my favorite. I don't want to exclude Go and TypeScript. Do we build ONE classic SDK or one per language? Can ONE SDK be used from C, C#, Go and TypeScript?"

### 9.1 Short answer

Yes, technically one protocol core can serve C, C#, Go and TypeScript. The cost is in the bindings and in distributing native binaries, not in the protocol logic. Most of the complexity (items 1–12 in §1) would be implemented exactly once.

### 9.2 Key design idea: a "sans-IO" core

The core does the protocol logic but **no I/O**. It does not open sockets or own threads; it only uses the clock that the host passes in.
- Input: received MQTT messages (topic + payload), timer ticks, API calls (e.g. "publish data point").
- Output: actions (publish topic + payload, subscribe, unsubscribe, schedule a timer).

This makes the core deterministic and easy to test. It can be compiled natively **or to WebAssembly**, and each host language can choose:
- the bundled Rust I/O driver (MQTT/TLS/timers/container bootstrap), or
- its own native MQTT client (mqtt.js, paho, MQTTnet).

### 9.3 Layering

```
            ┌──────────────────────────── oec-core (Rust, sans-IO) ───────────────────────────┐
            │ identity/IL · topics · codec · resources · Get/Set/Del · pagination · lifecycle │
            └──────────────────────────────────────────────────────────────────────────────────┘
                 │                               │                                  │
     oec (Rust runtime: MQTT+TLS,      WASM build of core               (protocol logic only)
     timers, /etc/oi4 bootstrap)            │                                  
                 │                    ┌─────┴─────────┐
        C ABI: liboec + oec.h         │               │
      ┌──────────┼───────────┐   TypeScript       Go (wazero,
      C/C++   C# (P/Invoke)  Go (cgo)  (WASM + mqtt.js)  pure Go, no cgo)
```

### 9.4 Per language

| Language | Native C ABI path | WASM core path | Native re-implementation |
|----------|-------------------|----------------|--------------------------|
| Rust | – (direct) | – | – |
| C / C++ | ✅ natural (cbindgen header) | – | – |
| C# / .NET | ✅ good: P/Invoke, NuGet with `runtimes/<rid>/native` libs | possible (wasmtime-dotnet), not needed | possible |
| Go | ⚠ cgo: needs a C cross-toolchain, no pure static cross-compile, callbacks from Rust threads need care; Go developers often avoid cgo | ✅ interesting: wazero is pure Go, no cgo; MQTT via paho | possible; independent implementation verified by D7 |
| TypeScript / Node | ✅ napi-rs with prebuilt binaries per OS/arch (incl. armv7) | ✅ no native prebuilds; MQTT via mqtt.js; also usable in Deno/Bun | possible |

Tooling notes: UniFFI (Mozilla) plus the community generators `uniffi-bindgen-cs` and `uniffi-bindgen-go` can generate C# and Go bindings automatically. UniFFI does not produce a public C API, though, so C still needs a hand-designed `cbindgen` API. Recommendation: **one hand-designed, stable C ABI** as the single foreign interface, with thin idiomatic wrappers on top of it.

### 9.5 Trade-offs: one core vs. one SDK per language

| | One core + bindings | One SDK per language |
|-|---------------------|----------------------|
| Protocol logic | once → consistent behavior, one place to fix bugs | N times → drift risk (mitigated by D1 + D7) |
| Idiomatic feel | needs a thin wrapper per language | natural |
| Distribution | native libs per OS/arch (or WASM) → more complex CI/release | simple (pure packages) |
| Debugging | across the FFI boundary harder | simple |
| Threading/callbacks | must be designed carefully in the C ABI | language-native |
| Effort for language #2..n | small to medium (wrapper) | large (full SDK) |

### 9.6 Agent recommendation

1. Build **one SDK** = Rust `oec-core` (sans-IO) + Rust runtime + **stable C ABI** from day one. Design the C ABI so that C# and Go wrappers are easy (opaque handles, plain structs, UTF-8 strings/JSON buffers, explicit free, callbacks with a user-data pointer, no Rust types or async in the ABI).
2. Deliver in this order:
   - Rust API
   - C API, proven by the logger demo in C
   - C# wrapper
   - TypeScript and Go: decide per language between the binding (napi-rs or WASM; for Go, wazero WASM rather than cgo) and a native implementation, based on a small spike.
3. D7 (conformance tool) stays independent (ADR-0006) and checks every language binding the same way.
