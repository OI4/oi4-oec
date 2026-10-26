# ADR-0020 – Public API of the Rust SDK (D2)

- Status: **accepted** (PO, C-045 review 2026-10-26; proposed in C-042). Amendment C-048 (first part of the crate `oec`: additions, `MemoryStore::new()`, rules of `build()`) and C-049 (no API change; `set_health_details` reads "equals" per REQ-RES-HEALTH-002) by the agent (G-3.4 b).
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
