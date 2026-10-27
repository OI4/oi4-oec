# ADR-0020 – Public API of the Rust SDK (D2)

- Status: **accepted** (PO, C-045 review 2026-10-26; proposed in C-042). Amendment C-048 (first part of the crate `oec`: additions, `MemoryStore::new()`, rules of `build()`) C-049 (no API change; `set_health_details` reads "equals" per REQ-RES-HEALTH-002) and C-050 (runtime moved to `oec`; explicit `Broker` needs a CA; `run_async` installs no signal handlers) by the agent (G-3.4 b).
- Date: 2026-10-26
- Deciders: product owner (Thomas) – proposed by the agent in C-042
- Related: ADR-0011 (S4 architecture), ADR-0016 (level targets), ADR-0019 §7 (schemas in D2), ADR-0021 (C ABI), ADR-0022 (D9 API), Q-021, Q-022, SF-101

## PO decision (C-045 review, 2026-10-26)

Accepted as proposed. Q-021 (a): blocking API by default, `run_async` on demand. Q-022 (a): string-keyed `Config` getters; a derive macro stays a possible P4+ addition. SF-101 decided as proposed: every `Mode` except `OFF_0` allows `Get` (§7); the SDK behavior sketched for the open state is now the specified behavior.

## Context

P3 has to fix the public API of the Rust SDK (D2) before P4 implements it (ROADMAP P3, item 2). ADR-0011 decided the layering (`oec-core` sans-IO, `oec` runtime, front ends incl. the C ABI `oec-ffi`) and that the developer chooses the compliance level, but left the API itself open. The goal of D2 is to hide the protocol complexity (`research/sdk-options.md` §1: 14 complexity items such as Birth/Will, PublicationList semantics, pagination, Status events, DataSetWriterIds).

The API shape was worked out as a **compile-checked sketch**: `oec/sdk/rust/sketch/` (crate `oec-api-sketch`, library name `oec`, no dependencies, every body `unimplemented!()`), checked by `oec/sdk/check.sh` as part of `oec/check.sh`. Its doc comments carry the REQ ids; a test checks that every REQ id cited under `oec/sdk/` exists and is not deprecated, and that the hello world stays within 20 lines.

## Decision

### 1. The sketch is the API contract for P4

The public items of `oec/sdk/rust/sketch/src/` (modules `app`, `identity`, `asset`, `data`, `config`, `event`, `consumer`, `status`, `store`) are the public API of the crate `oec` in P4. P4 may add items. Renaming or removing items or changing the semantics described in their doc comments needs an amendment of this ADR. The sketch is deleted when the P4 crate exists. Its examples become the SDK examples.

### 2. Execution model: blocking by default, async on demand (Q-021)

- `App::run()` blocks, starts its own runtime and needs no async knowledge. `App::run_async()` returns a `Send` future for applications that already use tokio. The runtime crate is chosen in C-043 (ADR-0023: tokio).
- Handles (`AppHandle`, `Asset`, `DataSet`, `ConfigHandle`, `Consumer`) are `Clone + Send + Sync`.
- State updates (`DataSet::update`, `Asset::set_health`, …) are **synchronous and non-blocking**. They work from any thread or task, e.g. from a field-bus polling thread.
- Handlers (`on_set`, `on_presence`, subscriptions) are synchronous closures. They run on an SDK worker, one at a time per application.
- Only consumer requests that wait for an answer (`get`, `set`, `del`, `new_data_set_writer_id`) are `async`. A `_blocking` twin exists for each of them.
- The C ABI (ADR-0021) maps 1:1 to this model: blocking calls, callbacks and an SDK-owned thread (ADR-0011).

### 3. Level selection: the level is chosen, the protocol is automatic

`App::builder(level, service_type, product)` takes the Messaging level. The SDK enables every behavior that is **mandatory or recommended** at that level, so no protocol switch is left to the developer:

| Behavior (automatic) | L1 | L2 | L3 | REQs |
|---|:-:|:-:|:-:|---|
| TLS connect, auth, Will per connect, Birth first, Close, reconnect + Birth again | ✓ | ✓ | ✓ | REQ-CMP-001 … 003, REQ-LIFE-001/005/008/015/016/017, REQ-MQTT-005/008/009/010 |
| Pub MAM + Health (self and represented assets), Health heartbeat 60 s | ✓ | ✓ | ✓ | REQ-CMP-006/007, REQ-TOP-017 |
| ClientId, keep alive 60 s, clean session, QoS 0, no retain, publish on change | ✓ | ✓ | ✓ | REQ-MQTT-013 … 019 (recommended at L1/L2, the SDK does it always) |
| `Get` of every supported resource (MAM and Health included) incl. empty lists, pagination | – | ✓ | ✓ | REQ-CMP-008/015/017, REQ-CMP-041/042, REQ-PAG-002/005 |
| `Set`/`Del` where the application offers it, Status event after every `Set`/`Del` | ✓¹ | ✓ | ✓ | REQ-CMP-016, REQ-CMP-023, REQ-CMP-043, REQ-RES-EVENT-005 |
| Profile, PublicationList, ReferenceDesignation, Config, Data, Metadata, Event | –² | ✓ | ✓ | REQ-CMP-018 … 020, REQ-CMP-021, REQ-RES-PROFILE-002 |
| SubscriptionList, `Call`/`Reply` `NewDataSetWriterId` | – | ✓ | ✓ | REQ-CMP-039/040, REQ-ACT-023 |
| `Set`/`Del` subscribed for every resource that defines it; `BadNotWritable` for objects not offered | – | – | ✓ | REQ-CMP-033, REQ-CMP-043 |

¹ A Level 1 application that declares a writable object gets `Set` handling and the Status event (should at L1).
² At L1 the SDK publishes only what the level requires plus what the application declares. A declared DataSet is published, but there is no `Get` subscription.

- **Recommended behavior (R) is switched on.** The SDK offers the recommended L2 behavior (Profile, PublicationList, …) at L2 too. It costs the developer nothing and makes the *observed* level of the registry match.
- **Offering more than the level is allowed**, e.g. a writable DataSet at L1. **Offering less is impossible**: the SDK implements the resources itself and answers with empty lists (REQ-CMP-041, SF-037).
- **`build()` rejects configurations that cannot keep the level**, e.g. a non-persistent store while ReferenceDesignation is offered (Level 3 always, Level 2 by default; REQ-RES-REFDES-002), or an invalid identity. `MemoryStore` is for tests at Level 1.
- **Represented entities** (`add_asset`) get the same level as the application. The application provides the lists and common services once (REQ-CMP-024, REQ-CMP-045).
- The Service Container level is a property of the image (D8), not of the SDK.

### 4. Identity

- `IdentificationLink` is a validated newtype. It is never normalized and compares character by character (REQ-IL-008/009/013/015/016/018/019). Topic encoding is internal (REQ-IL-002 … 005).
- `Oi4Identifier` builds the IL: the ManufacturerUri is lower-cased, `nd` stands for a missing element, dnp-encoding is opt-in, length ≤ 255 (REQ-IL-022 … 033).
- `Product` describes the type level and becomes the MAM. Defaults for an application:
  - `HardwareRevision` `""` (REQ-RES-MAM-008)
  - `SoftwareRevision` `"undefined"` (REQ-RES-MAM-010)
  - `DeviceRevision` `""` (as in the MG examples)
  - `DeviceClass` `Oi4.<ServiceType>` (REQ-RES-MAM-014)
  - LocalizedText from `&str` in `en-US` (REQ-RES-MAM-024)
- The SDK fills the instance level of the MAM:
  - `SerialNumber`: the container name in a container (REQ-CNT-021), else set explicitly;
  - `ProductInstanceUri`: the IL (REQ-RES-MAM-016);
  - `RevisionCounter`: persisted and incremented on every configuration change made through the SDK (REQ-RES-MAM-018).
- An existing IL is used and never replaced (REQ-IL-006, REQ-IL-031). Otherwise the Oi4Identifier is generated (REQ-IL-007, REQ-RES-MAM-006).

### 5. Bootstrap, connection, lifecycle

- `Bootstrap::container()` is the default. It implements the container storage contract: `broker.json` with unknown keys ignored, `broker_ca.pem` falling back to `ca.pem`, `/run/secrets/oi4` and the container name from the hostname (REQ-CNT-004, REQ-CNT-009, REQ-CNT-021, SF-100). A `root` prefix serves tests and the dev environment.
- `Bootstrap::Explicit(Broker)` serves devices and tests (ADR-0011).
- **TLS only:** there is no API for a plain connection (REQ-MQTT-005). A client certificate wins over credentials (REQ-MQTT-010).
- MQTT keep alive is configurable with a default of 60 s (REQ-MQTT-014). Clean session, QoS 0 and no retain are fixed (REQ-MQTT-015/017/018).
- `run()` handles SIGINT/SIGTERM. It sends Close and then disconnects (REQ-LIFE-005). `ConnectionState` reports `Connecting` / `Online` / `Reconnecting` / `Closed`.

### 6. Resource registry and built-in resources

- **Entirely SDK-owned resources:** MAM, Health, Profile, PublicationList, SubscriptionList, ReferenceDesignation (with persistence), Metadata (derived from the field definitions), Status events and `NewDataSetWriterId` (Ttl reservation, REQ-ACT-023).
- **Application-declared resources:** Data (`add_data`), Config (`add_config`), Generic events (`emit_event`) and Syslog (via the logging bridge).
- The SDK assigns DataSetWriterIds from 10 (REQ-MSG-020, REQ-RES-PUBLIST-006).
- Publisher-defined DataSetClassIds default to a UUIDv5 of tag and field structure. They are stable across restarts and change with the structure (REQ-MSG-013).
- The SDK answers a `Get` for an unknown Source not on the Pub topic but with a Status event `BadNotFound` (REQ-CMP-047). A `Get` of Metadata without Data gets no Metadata (REQ-CMP-044).

### 7. Publishing: state, not messages (PublicationList semantics)

- Applications **update state**. The PublicationList entry decides when a message is sent:
  - `Mode` (REQ-RES-PUBLIST-007 … 012);
  - `Interval` – 0 publishes on change, more than 0 publishes cyclically and overrides Precisions (REQ-RES-PUBLIST-013/014, REQ-MQTT-019);
  - `Precisions` as a dead band (REQ-RES-PUBLIST-015).
- The last state answers `Get` (REQ-RES-DATA-007). A series of pages is built from one snapshot (REQ-PAG-007).
- `Mode` is a set of the four publication forms (`Mode::ON_REQUEST | Mode::FILTER`). The SDK maps it to the enum value. SDK defaults:
  - Data: `ON_REQUEST_FILTER_9`
  - Config: `ON_REQUEST_FILTER_9`
  - MAM, Health, Profile, ReferenceDesignation: `ON_REQUEST_SOURCE_10`
  - Metadata: `ON_REQUEST_FILTER_9`, fixed (REQ-RES-PUBLIST-012)
  - Event entries: `FILTER_4`; Syslog `Debug` and `Informational` are `OFF_0` (REQ-TOP-024/027)
  - The defaults of all resources that support `Get` contain `ON_REQUEST`, so they are correct under both readings of SF-101 (open: is `Get` allowed without `ON_REQUEST`?). Until SF-101 is decided, the SDK answers `Get` for every entry that is not `OFF_0` (proposed interpretation).
- `Publication::writable(...)` sets the entry `Config`. The SDK applies a `Set PublicationList` within these limits, persists it and republishes (REQ-RES-PUBLIST-016/017/018/024/025).
- The MaxPacketSize comes from `broker.json` or `Broker::max_packet_size`. The application can lower it for its own publications; the SDK paginates every list publication (REQ-PAG-005).

### 8. `Get`, pagination, Locale, Query – invisible to the application

The SDK answers `Get` from its state store. It honors PaginationRequest (REQ-PAG-002), Locale and the ReferenceDesignation Query (REQ-RES-REFDES-011 … 014). It ignores undefined or unsupported query parameters (REQ-QRY-002/003). The application never sees pagination.

### 9. `Set`/`Del`: handlers decide, the SDK answers

A `Set`/`Del` goes through a fixed pipeline:

1. decode (tolerant reader, REQ-MSG-046);
2. address check: unknown object → `BadNotFound`, not offered → `BadNotWritable` (REQ-CMP-043);
3. structural/type check against the definition;
4. application handler `Fn(&Request) -> Result<(), StatusCode>`;
5. apply and persist;
6. publish the changed state (REQ-RES-DATA-012, REQ-RES-CONFIG-022, REQ-RES-PUBLIST-025, …);
7. Status event with `ReplyTo` = request MessageId (REQ-RES-EVENT-005, REQ-MSG-015, REQ-TOP-014/015).

The SDK handles the lists and ReferenceDesignation completely. The application can observe or veto ReferenceDesignation changes.

### 10. Config

- `ConfigDef` → groups → `ConfigObject` with Type, default, unit, Mandatory, Sensitive and Validation.
- Names are checked when the definition is built (REQ-RES-CONFIG-004/006/007).
- A `Set Config` is validated completely by the SDK before the handler sees it (REQ-RES-CONFIG-005/008/011/023/025). It is applied atomically, persisted, the RevisionCounter is incremented and sensitive values are masked in every `Pub` (REQ-RES-CONFIG-024).
- Values are accessed by `(group, name)` with typed getters (`number`, `string`, `boolean`). A derive macro for typed config structs is a possible P4+ addition (Q-022).

### 11. Events

- Status events are automatic (§9).
- **Syslog:** the first `App` installs a `log`/`tracing` bridge, so `log::warn!(…)` becomes `Event/<AppId>/Syslog/Warning`. Each level has its own PublicationList entry. The lowest published level is configurable, default `Notice` (REQ-TOP-024/027, REQ-RES-EVENT-007/008).
- Generic events use `Asset::emit_event` (REQ-RES-EVENT-009).

### 12. Consumer side

`Consumer` provides:

- `subscribe(resource, target, f)`. Every subscription is an entry of the own SubscriptionList; `configurable()` sets `CONF_1` (REQ-RES-SUBLIST-001 … 005).
- `on_presence` for Birth/Close/Will.
- `get` with correlation by `ReplyTo`, page reassembly and timeout (REQ-MSG-015, REQ-PAG-002/006).
- `set`/`del`, waiting for the Status event.
- `new_data_set_writer_id` with TimeoutHint and RequestId handling (REQ-ACT-009/011).

`Target::any()` and absent parts become `+` wildcards (REQ-IL-021). Received messages are tolerant (REQ-MSG-046).

### 13. Strict sender with embedded schemas

Every outgoing message is built from typed values (REQ-MSG-001). With the cargo feature `strict-validation` (on in the SDK's own tests and examples), every outgoing message is also validated against the embedded D1 schemas (ADR-0019 §7). A violation is a test failure, or an error log in production.

### 14. Errors

`oec::Error` covers API and runtime errors: `Invalid`, `LevelNotMet`, `Bootstrap`, `Io`, `Timeout` and `Closed`. `StatusCode` is the protocol result of handlers and requests (MG 8.3.6.1). The two are never mixed.

### 15. Non-goals

- No custom Actions: the guideline has only common services (GL-ADR-012/014).
- No MQTT 5, no plain TCP, no v1 (ADR-0015).
- No direct access to raw topics for publishing. The only escape hatches are `Value::Json` and `Received::payload_json`.

## DX sketches (P3 goal)

| Example (`oec/sdk/rust/sketch/examples/`) | Shows |
|---|---|
| `hello.rs` (16 lines, test: ≤ 20) | Level 1 application in a container: product → MAM, Birth/Will/Close/heartbeat automatic |
| `device_source.rs` | Level 3 OTConnector with a represented field device (Oi4Identifier), Oi4Pv DataSet with Precision, writable PublicationList entry, `on_set` with own range check, update from a plain thread |
| `config.rs` | Config with Number + sensitive String, Validation, own rule in `on_set`, typed read access |
| `consumer.rs` | subscriptions, presence, blocking and async `Get` |

## Consequences

- P4 increments 5–8 (lifecycle, provider side, write side, consumer side) implement exactly this surface. The sans-IO core `oec-core` holds the state store, the PublicationList engine, the `Set` pipeline and the pagination. The runtime `oec` holds timers, MQTT/TLS, signals and the bootstrap.
- The SDK owns state, so memory grows with the number of DataSets and assets. This is acceptable on edge hardware (≥ 1 GB, RIG 5.1); D9 has its own API (ADR-0022).
- The C ABI (ADR-0021) and later bindings wrap the same concepts; the JSON boundary keeps them small.
- D7 `test` (P5) can use the SDK examples as applications under test. D7 shares no code with the SDK (Q-009).

## Alternatives considered

- **Async-first API (tokio required for every call).** Idiomatic for network code, but the simplest app would need `#[tokio::main]` and `.await` on every update, and the C ABI would need a second model. Rejected as default; `run_async` covers tokio users (Q-021).
- **Message-oriented API** (the app builds DataSetMessages and calls `publish(topic, msg)`). Maximum control, but every app would have to implement PublicationList, pagination and Status events. That contradicts the purpose of D2.
- **Code generation from a device description** (e.g. YAML → Rust). Useful later for large data models. Too early before the runtime API is stable.
- **One trait per resource** (`impl DataProvider for MyType`). Clean for static models, but awkward for assets discovered at runtime (connectors). Closures plus definitions cover both.

## Amendment C-048 (2026-10-26) – P4 increment 4: crate `oec`, first part (agent, G-3.4 b)

Decided by the agent within this ADR (APIs within an accepted ADR, G-3.4 b); the PO can overrule. Details and REQ tracing: `log/2026-10-26_C-048_oec-crate-bootstrap.md`, REVIEW A-17 … A-24.

### A. What moved from the sketch into `oec`

`Level`, `Error`, `Result`, `ServiceType`, `Bootstrap`, `Broker`, `AppBuilder` (all methods except `on_connection`), `App::builder`, the module `identity` (`IdentificationLink`, `Oi4Identifier`, `Product`, `LocalizedText`), `store`, `status::StatusCode`, `event::SyslogLevel` and the definition part of `config` (`ConfigType`, `ConfigObject`, `ConfigDef`, `ConfigValues`). For these items the crate is the contract from now on; the sketch keeps them only so that its remaining items and examples compile. Everything else (runtime, assets, data, `ConfigHandle`, events, consumer) stays in the sketch until its increment (ROADMAP P4: 5 … 8) and is still the contract.

### B. Additions (allowed by §1)

| Item | Why |
|---|---|
| `AppBuilder::reference_designation(bool)` | Makes "Level 2 by default" of §3 a switch: default off at L1, on at L2 (SHOULD) and L3 (MUST, REQ-CMP-020). `false` at L3 → `LevelNotMet`. Offered with a non-persistent store → `LevelNotMet` (REQ-RES-REFDES-002). |
| `Store::is_persistent()` (provided, default `true`; `MemoryStore` → `false`) | `build()` needs to know whether the store keeps the ReferenceDesignation (§3). |
| `App::{level, service_type, identification_link, mam, connection, reference_designation, syslog}` | Read access to what `build()` resolved; used by tests and by increment 5. |
| `app::ConnectionSettings`, `app::Auth`, `app::Secret` | The resolved connection: host, port, CA, credentials or client certificate, ClientId, keep alive, MaxPacketSize and own publish limit. `Secret` has a redacted `Debug` (G-4.5). |
| `IdentificationLink`: `Display`, `FromStr`, `AsRef<str>`; `identity::Mam` (re-export) | Usual conversions; the MAM is visible through `App::mam`. |
| `ConfigDef::{context, values}` | `values()` validates the definition (names, defaults against Type and Validation) and returns the initial values with the getters of §10 (Q-022). |
| `StatusCode::code()` | The numeric OPC UA code, needed by the Status events of increment 7. |
| `FileStore::dir()` | Where the state lives (diagnostics). |

### C. Changes of sketch items (need this amendment)

1. **`MemoryStore` is constructed with `MemoryStore::new()` (or `Default`)** instead of being a unit struct: it holds its map. The sketch mirrors the change.
2. **Builder rules made precise (no change of the described semantics):**
   - in a container, `serial_number` is only accepted together with an explicit `identification_link` (the container name is the SerialNumber of the Oi4Identifier, REQ-CNT-021), and `client_id` is rejected (the container name is the ClientId, REQ-MQTT-013) – the sketch says "outside a container" for both;
   - outside a container a serial number is required (REQ-RES-MAM-015) and the default ClientId is `oec` + the FNV-1a 64 hash of the Identification Link in hex (stable, "derived from the Identification Link" as in the sketch);
   - `keep_alive` takes whole seconds 1 … 65535 (the MQTT field; 0 would disable the Will trigger, REQ-LIFE-008);
   - `Broker` without credentials and client certificate → `LevelNotMet` (every level authenticates, REQ-CMP-002, REQ-MQTT-009); a missing CA, certificate or key file → `Io`;
   - `max_packet_size` above the MaxPacketSize of the Message Bus is clamped with a warning (REQ-PAG-005), 0 is `Invalid`.

### D. Q-021 (`run_async`)

`run`, `run_async`, `handle`, `AppHandle` and `ConnectionState` need the runtime and stay in the sketch until increment 5. No placeholder is added to `oec`; `App` is `Send + Sync` (tested), so the async form of §2 stays possible without an API change.

## Amendment C-049 (2026-10-26) – P4 increment 5a: no API change, one clarification (agent, G-3.4 b)

Increment 5a adds sans-IO building blocks to `oec-core` (ADR-0026 §9); the public API of `oec` and the sketch are unchanged. "Increment 5" in amendment C-048 D now means increment **5b** (C-050).

1. **`set_health_details` (sketch `asset.rs`):** the sketch says "`Health` is then **at least** the most severe detail". REQ-RES-HEALTH-002 says Health **equals** the Health with the highest priority of the Details. The SDK implements "equals" (the core check `oec_core::lifecycle::check_health` refuses anything else). This is a wording fix of the described semantics, not a change; C-050 corrects the sketch doc when it moves `Asset` to `oec`. How `set_health` behaves while `Details` are set is decided there, within this rule.
2. **Keep Alive never 0:** the core session takes Keep Alive as `NonZeroU16` seconds, matching C-048 C ("0 would disable the Will trigger").

## Amendment C-050 (2026-10-26) – P4 increment 5b: runtime in `oec` (agent, G-3.4 b)

Decided by the agent within this ADR (APIs within an accepted ADR, G-3.4 b); the PO can overrule. Details and REQ tracing: `log/2026-10-26_C-050_runtime-tokio-rustls.md`, REVIEW A-32 … A-41. Runtime semantics: ADR-0026 §10.

### A. What moved from the sketch into `oec`

`App::run`, `App::run_async`, `App::handle`, `impl Deref<Target = AppHandle> for App`, `AppHandle::{state, shutdown}`, `ConnectionState` and `AppBuilder::on_connection`. `AppHandle::{this, add_asset, asset, consumer}`, `Asset` (with `set_health`/`set_health_details`), DataSets, `ConfigHandle`, events and the consumer stay in the sketch until their increments (ROADMAP P4: 6 … 8) and are still the contract. Amendment C-048 D (Q-021) is realized: `run_async` returns a `Send` future (tested: it is spawned with `Runtime::spawn`, which requires `Send`).

### B. Additions (allowed by §1)

| Item | Why |
|---|---|
| `AppHandle: Debug`, `ConnectionState: Copy + Eq + Debug` | Usual derives; `AppHandle` is `Clone + Send + Sync` as §2 requires (tested). |
| `oec::prelude` exports `AppHandle` | It is named in application code that passes the handle to threads. |
| Cargo features `crypto-aws-lc-rs` (default) and `crypto-ring` | Choice of the rustls crypto provider (ADR-0023 §1, amendment C-050); not an API item. |

### C. Changes of sketch or C-048 items (need this amendment)

1. **An explicit `Broker` needs a CA.** The sketch said "`ca_pem`: CA of the broker". Without a CA, `build()` now fails with `LevelNotMet` (REQ-CMP-002, REQ-MQTT-008: the application verifies the broker with the CA of the Message Bus storage). There is no fallback to the platform trust store; ADR-0023 §1 says trust comes from the storage only, and an OS trust store may become an explicit opt-in later (then as an addition). Consequently `ConnectionSettings::ca()` returns `&Path` instead of `Option<&Path>` (a C-048 addition).
2. **`run_async` installs no signal handlers.** §2 and the sketch say "same as `App::run`". `run` owns its runtime and the process, so it handles SIGINT/SIGTERM (Windows: Ctrl-C, Close, Shutdown) and then sends Close and DISCONNECT (§5). `run_async` runs inside the application's tokio runtime; installing process-wide handlers there would steal the signals from the application. The application calls `AppHandle::shutdown` (e.g. from its own `tokio::signal::ctrl_c()`). Everything else is the same.
3. **Precision, no change of the described semantics:**
   - `ConnectionState` is `Closed` before `run` and after it returns; `Connecting` until the first CONNACK, `Online` while connected, `Reconnecting` after a connection was lost (until the next CONNACK). A refused or failed first attempt stays `Connecting`.
   - `on_connection` is called on every change of the state (no repeated equal value), synchronously on the runtime thread; it must not block.
   - `AppHandle::shutdown` is idempotent. Before `run` it makes `run` return `Ok(())` at once without connecting; during `run` it sends Close (if connected) and DISCONNECT, then `run` returns `Ok(())`.
   - `run`/`run_async` fail only before the first connection: `Error::Bootstrap` for unusable TLS material (CA, client certificate, key, passphrase), `Error::Invalid` for a host that is no TLS server name, `Error::Io` if the runtime cannot start. A lost or refused connection is not an error: the SDK reconnects with backoff (ADR-0026 §10).
   - `Error::Bootstrap` also covers the TLS material of the connection settings, not only the container storage (§14 lists the variant; its doc is widened).


## Amendment C-053 (2026-10-27) – P4 increment 6a: assets, `Get` answers, Profile and PublicationList (agent, G-3.4 b)

Decided by the agent within this ADR (APIs within an accepted ADR, G-3.4 b); the PO can overrule. Details and REQ tracing: `log/2026-10-27_C-053_provider-registry-get.md`, REVIEW A-63 … A-74. Provider semantics: ADR-0026 §12.

### A. What moved from the sketch into `oec`

`AppHandle::{this, add_asset, asset}`, `AssetDef` (`new`, `with_oi4_identifier`, `revision_counter`), `Asset::{identification_link, set_health, set_health_details, bump_revision_counter, remove}`, `HealthState` and `HealthDetail` (module `oec::asset`, also in the prelude). The SDK now serves MAM, Health and Profile of every source (the application and its represented assets) and the PublicationList, and answers `Get` of these four resources from Level 2 (§3, §6, §8). DataSets, Metadata, publish modes and intervals (§7 beyond the built-in resources), `ConfigHandle`, events, ReferenceDesignation and the consumer stay in the sketch until increments 6b, 7 and 8 and are still the contract.

### B. Additions (allowed by §1)

| Item | Why |
|---|---|
| `AppHandle::assets() -> Vec<Asset>` | The represented assets in the order of registration; needed to iterate without knowing the ILs. |
| `Asset::mam() -> Option<Mam>` | Read the published MAM (also the generated Oi4Identifier of `with_oi4_identifier`); `None` after `remove`. |
| `Asset::is_this() -> bool` | Tells the application itself from a represented asset (e.g. `remove` is refused for it). |
| `HealthDetail::new(HealthState)` | Short constructor; the fields stay public as in the sketch. |
| `HealthState::severity` (re-export of the `oec-core` enum) | The order of the aggregation (A-28) is visible and testable. |
| `impl From<ProviderError> for Error` (→ `Error::Invalid`) | Errors of the sans-IO provider keep their message (ADR-0026 §8.1). |

### C. Changes of sketch or C-048 items (need this amendment)

1. **`Asset::bump_revision_counter` returns `Result<()>`** (sketch: `()`). The counter is persisted (REQ-RES-MAM-018); a failing store must not be hidden, and a removed asset or a counter at `i32::MAX` is an error. A counter taken from the VendorNameplate (`AssetDef::revision_counter`) is not changed; the call logs a warning and returns `Ok(())`.
2. **`App::mam` and `App::identification_link` moved to `AppHandle`** (C-048 additions). `App` derefs to `AppHandle`, so `app.mam()` keeps compiling. `mam()` now returns an owned `Mam` (was `&Mam`), because the MAM lives behind the shared state of the handle and changes at runtime (RevisionCounter).
3. **No heartbeat for represented assets.** The sketch said "The heartbeat every 60 s is automatic" for every `Asset`. After SF-106 (MG 7.5.1, `9044929`) the 60 s heartbeat applies to the Health of the application itself only and is optional for subordinate assets; the SDK sends none for them (fewer messages, REQ-TOP-017). The sketch doc is updated.
4. **`Get` for an unknown Source** (§6): the SDK does not answer on the Pub topic (REQ-CMP-047, the MUST part). The Status event `BadNotFound` (the SHOULD part) comes with the event machinery of increment 7; until then the request is logged at `debug`. §6 stays the target.
5. **Precision, no change of the described semantics:**
   - `set_health` clears the `Details`; `set_health_details` sets Health to the most severe detail (A-28) and omits the HealthScore; no details means NORMAL_0 without `Details`. Invalid values (score above 100; NORMAL_0 with score 0 for the application itself, which is the Close of REQ-LIFE-014) and calls on a removed asset are logged and ignored, as the sketch signature `-> ()` implies.
   - A represented asset starts with Health NORMAL_0 / 100. Its MAM defaults differ from the application's (§4): `HardwareRevision` `"undefined"` (a physical asset without known revision, REQ-RES-MAM-008) and `DeviceClass` `""`, unless the `Product` sets them.
   - `add_asset` refuses the AppId, an IL registered twice and a `Product` whose MAM is invalid (`Error::Invalid`). It works before and during `run`; a change is published at once when connected, otherwise with the next connection (ADR-0026 §12: state, no queue).
   - `remove` keeps the DataSetWriterIds of the asset reserved (REQ-RES-PUBLIST-006), so a re-added asset gets its old ids.

## Amendment C-054 (2026-10-27) – P4 increment 6b: Data DataSets, Metadata, publish modes (agent, G-3.4 b)

Decided by the agent within this ADR (APIs within an accepted ADR, G-3.4 b); the PO can overrule. Details and REQ tracing: `log/2026-10-27_C-054_data-sets-metadata.md`, REVIEW A-75 … A-86. Provider semantics: ADR-0026 §13.

### A. What moved from the sketch into `oec`

Module `oec::data` (also in the prelude): `BuiltinType`, `Value`, `Field::{new, double, unit, description}`, `Mode` (`OFF`, `ON_REQUEST`, `APPLICATION`, `SOURCE`, `FILTER`, `|`), `Writable`, `Publication::{default_data, mode, interval, precision, writable}`, `DataSetDef::{new, oi4pv, field, class_id, description, publication}`, `DataSet::{update, remove}` and `Asset::add_data`. The SDK now publishes Data and Metadata per PublicationList entry and answers `Get Data` / `Get Metadata` (§6, §7). Still in the sketch and still the contract: `DataSet::on_set` and `SetData` (increment 7, write side), `Publication::writable` is stored and listed in the PublicationList entry `Config`, but a `Set PublicationList` is applied only with increment 7; `ConfigHandle`, events, ReferenceDesignation and the consumer (increments 7 and 8).

### B. Additions (allowed by §1)

| Item | Why |
|---|---|
| `Field::range(low, high)` | The `EURange` property of the Metadata (REQ-RES-META-003); without it a consumer cannot scale a value. |
| `DataSet::{tag, source, values}` | Read back the Tag, the `Source` and the current values (in field order, once every field has one); needed by applications that keep only the handle. |
| `Mode::union` (`const fn`) | `Mode::ON_REQUEST | Mode::FILTER` is not usable in `const` items; `union` is. |
| `Value::{UInt64, Bytes, LocalizedText}`, `From` for all integer widths, `f32`, `String`, `Vec<u8>`/`&[u8]`, `LocalizedText` | Every `BuiltinType` of the sketch gets a typed value. |

### C. Changes of sketch items (need this amendment)

1. **`DataSet::update` returns `Result<()>`** (sketch: `()`). An unknown field, a value that does not fit its field (type or range of the integer type) or a removed DataSet is `Error::Invalid`; the update is all or none. Silently dropping a wrong value would hide a programming error and publish stale state; the call stays synchronous and non-blocking.
2. **`Value::Json` (escape hatch) is dropped**; `Value` is `#[non_exhaustive]` with typed variants for every `BuiltinType` instead. Pre-encoded JSON could not be checked against the field type, and the strict sender rule (§5 of the design rules) would not hold. A later addition is possible without breaking the API.
3. **`DataSetDef::oi4pv(pv)` renames the field to `Pv`** (REQ-RES-DATA-002): the name of the given `Field` is ignored, its type, unit, description and range are kept. The sketch example `oi4pv(Field::double("Pv"))` is unchanged.
4. **A DataSet is published and answered once every field has a value.** A DataSetMessage of a Data DataSet carries all fields of its Metadata (REQ-RES-DATA-007 with REQ-RES-META-003); until then `Get Data` with its Tag returns an empty list. The sketch example `device_source.rs` sets `Pv` and `Sv1` before the polling loop.
5. **`DataSetDef::class_id` is checked at `add_data`** (not a UUID → `Error::Invalid`), as builders do not fail (ADR-0020 §1).
6. **`Publication::interval`** accepts whole milliseconds up to `u32::MAX` (the PublicationList `Interval` is a UInt32 in ms, REQ-RES-PUBLIST-013); other durations are `Error::Invalid` at `add_data`.

## Amendment C-055 (2026-10-27) – P4 increment 7a: writable DataSets and the write side (agent, G-3.4 b)

Decided by the agent within this ADR (APIs within an accepted ADR, G-3.4 b); the PO can overrule. Details and REQ tracing: `log/2026-10-27_C-055_write-side.md`, REVIEW A-87 … A-97. Provider semantics: ADR-0026 §14.

### A. What moved from the sketch into `oec`

`SetData::{get, fields}` and `DataSet::on_set` (module `oec::data`, in the prelude). The SDK now answers every `Set`/`Del` addressed to the application with a Status event (§9), applies `Set PublicationList` within `Publication::writable` and persists the result (§7), and answers a `Get` to an unknown Source with the Status event `BadNotFound` (REQ-CMP-047). Still in the sketch and still the contract: `ConfigDef`/`ConfigHandle`/`on_set` of Config (increment 7b), events of the application, ReferenceDesignation (increment 7c), the consumer and the SubscriptionList (increment 8).

### B. Additions (allowed by §1)

| Item | Why |
|---|---|
| `SetData::fields()` | Iterate all requested values in definition order; the sketch had only `get`. |
| `StatusCode` → core mapping (crate-internal) | The handler's code is sent as given; Description and SymbolicId only for the codes the SDK knows. |

### C. Changes of sketch items (need this amendment)

1. **`DataSet::on_set` returns `Result<()>`** (sketch: `()`): `Error::Invalid` for a removed DataSet and at Level 1, where the SDK does not subscribe `Set` (requests are served from Level 2 on, MG 9.1; REQ-CMP-043 applies to an entity that offers `Set`). A silently ignored handler would make the DataSet look writable to the developer. A further call replaces the handler.
2. **`SetData` is checked tolerantly:** a missing field is `BadInvalidArgument`, a value that does not fit its field `BadTypeMismatch`, both without calling the handler; **unknown fields are ignored** (tolerant reader, REQ-MSG-046; the sketch said unknown field → `BadTypeMismatch`).
3. **Handler outcome:** `Ok` → the SDK sets the values as state, publishes the DataSet with the `ReplyTo` of the `Set` (REQ-RES-DATA-012), then the Status event Good; `Err(code)` → only the Status event with that code, an `Err` with a Good code is sent as `Bad` (an Uncertain code is sent as it is); a panic → `BadInternalError`. The handler runs on the runtime task outside every SDK lock (documented: short and non-blocking).
4. **`Publication::writable`** is now applied: a `Set PublicationList` may change exactly what it allows (`Mode`, `Interval`, `Precisions`); the result survives restarts (store key `app/publications`) as long as the definition still allows it.

## Amendment C-056 (2026-10-27) – P4 increment 7b: the Config resource (agent, G-3.4 b)

Decided by the agent within this ADR (APIs within an accepted ADR, G-3.4 b); the PO can overrule. Details and REQ tracing: `log/2026-10-27_C-056_config.md`, REVIEW A-98 … A-108. Provider semantics: ADR-0026 §15.

### A. What moved from the sketch into `oec`

`ConfigObject`, `ConfigDef`, `ConfigValues` and `ConfigHandle::{values, on_set, set}` (module `oec::config`) and `Asset::add_config` (§10). The SDK publishes every Config DataSet, answers `Get Config` and `Set Config` from Level 2, validates a `Set` completely before the handler sees it, applies it atomically, persists it, increments the RevisionCounter, publishes the masked Config with `ReplyTo` and answers with a Status event (§9). Still in the sketch and still the contract: events of the application and ReferenceDesignation (increment 7c), the consumer and the SubscriptionList (increment 8).

### B. Additions (allowed by §1)

| Item | Why |
|---|---|
| `ConfigDef::label` | `Context.Name` (LocalizedText, REQ-RES-CONFIG-002) as display name; default: the context name (as in MG B1.1.3). |
| `ConfigHandle::{context, source, remove}` | Same shape as `DataSet::{tag, source, remove}`; `remove` withdraws the Config (REQ-RES-PUBLIST-017) and keeps its persisted values. |
| `ConfigDef::context` | The Filter of the DataSet, for diagnostics and tests. |

### C. Changes of sketch items (need this amendment)

1. **`ConfigHandle::on_set` returns `Result<()>`** (sketch: `()`), as `DataSet::on_set` (amendment C-055): `Error::Invalid` at Level 1 (no `Set` subscribed) and for a removed Config. A further call replaces the handler. Without handler a valid `Set` is applied (sketch contract).
2. **Handler outcome:** `Ok` → applied, persisted, published with `ReplyTo`, then Status Good; `Err(code)` → only the Status event with that code (a Good code is sent as `Bad`); a panic → `BadInternalError`. The handler runs on the runtime task outside every SDK lock and gets the proposed values in clear (the application needs them); `Debug` of `ConfigValues` masks sensitive values, and no error message of the SDK contains a value (G-4.5).
3. **`ConfigHandle::set`** refuses the mask `********` as a value of a sensitive object (it would be indistinguishable from "unchanged" on the bus, GL-ADR-021) and every value its definition refuses; it publishes and bumps the RevisionCounter like a `Set`, without Status event.
4. **`ConfigObject::pattern`** is checked from now on (A-21 resolved): Rust `regex` syntax with Unicode classes, unanchored match; a pattern that does not compile is refused when the definition is added.

## Amendment C-057 (2026-10-27) – P4 increment 7c: ReferenceDesignation, Syslog bridge, Generic events (agent, G-3.4 b)

Decided by the agent within this ADR (APIs within an accepted ADR, G-3.4 b); the PO can overrule. Details and REQ tracing: `log/2026-10-27_C-057_refdes-events.md`, REVIEW A-110 … A-119. Provider semantics: ADR-0026 §16. The sketch was aligned in the same cycle.

### A. What moved from the sketch into `oec`

`ReferenceDesignation` (with `Designation`, `DesignationParent`, re-exported from `oec_core`), `Asset::on_reference_designation`, `AppBuilder::reference_designation`, `Asset::emit_event`, `GenericEvent`, `AppBuilder::syslog`, `SyslogLevel` and `event::syslog` (§6, §11). The SDK serves `Get`/`Set`/`Del ReferenceDesignation` from Level 2, persists the objects and answers with a Status event; it publishes Generic events of the application and bridges the `log` crate to Syslog events. Still in the sketch and still the contract: the consumer and the SubscriptionList (increment 8).

### B. Additions (allowed by §1)

| Item | Why |
|---|---|
| `Asset::reference_designation()` | Read the stored object (e.g. for a local display); the sketch had only the handler. |
| `event::install_bridge(Option<Box<dyn log::Log>>)` | Keeps a local logger next to the bridge (one global logger per process); without it `build()` installs the bridge alone if no logger is set. |
| `event::MAX_MESSAGE_BYTES` (1024) | The MSG of a Syslog event is cut to this size (UTF-8 safe); documents the limit. |
| `GenericEvent::{level, number}` | Getters for diagnostics and tests. |

### C. Changes of sketch items (need this amendment)

1. **`ReferenceDesignation`** has the shape of MG 8.3.10 (`function`/`product`/`location` of `Designation { value, local, parent }`, `DesignationParent { value, local, identification_link }`); the sketch had flat strings.
2. **`Asset::on_reference_designation` returns `Result<()>`** (sketch: `()`), as `on_set` (amendments C-055/C-056): `Error::Invalid` if the application does not offer the resource. `AppBuilder::reference_designation(true)` at Level 1 is `Error::Invalid` at `build()` (Level 1 serves no requests, REQ-CMP-008). Without handler every valid `Set`/`Del` is applied. Handler outcome as for `on_set`: `Ok` → applied, persisted, published with `ReplyTo` (Set), Status Good; `Err(code)` → only the Status event (a Good code is sent as `Bad`); a panic → `BadInternalError`. The handler runs outside every SDK lock.
3. **`GenericEvent`** is a builder `new(level, number).description(..).details_json(..)` (sketch: public fields without Level): the `<Level>` of the topic is required (MG 8.3.6.3, free level). `details_json` must be a JSON object; it is checked at emit.
4. **`Asset::emit_event` returns `Result<()>`** (sketch: `()`): `Error::Invalid` for an empty Level, a Level with a character not allowed in a topic level, a 17th Level of a source, or Details that are no JSON object. A valid event is dropped silently (`Ok`) while offline or with 256 events waiting: events are not queued for later (MG 8.3.6, no retained or stored events). The first use of a Level registers its PublicationList entry even offline.
5. **Syslog bridge** (`log` only; `tracing` users bridge via `tracing-log`): records of the targets `oec`, `oec_core`, `rustls`, `tokio`, `mio` are not bridged, and a thread-local guard stops feedback; Level mapping Error→Error, Warn→Warning, Info→Informational, Debug/Trace→Debug; `log::max_level` follows the switched Syslog levels; at most 50 events in a burst and 10/s, the number dropped is reported once as a Warning. Never log secrets: a log record is a public event on the bus (GL-ADR-021). A later `env_logger::init()` (or any `set_logger`) fails because the bridge is the logger (documented; use `install_bridge`).
