# D2 API sketch (P3, C-042) – not product code

Compile-checked sketch of the public API of the Rust SDK (D2), its C ABI and the protocol flows behind it. The decisions are in ADR-0020 (Rust API) and ADR-0021 (C ABI). This folder is deleted when the P4 crate exists.

| Path | Content |
|---|---|
| `src/` | public items of the future crate `oec`; every body is `unimplemented!()`; doc comments cite REQ ids |
| `examples/hello.rs` | hello world, Level 1, 16 lines (test: ≤ 20) |
| `examples/device_source.rs` | Level 3 OTConnector with a represented device, Data + `Set` |
| `examples/config.rs` | Config resource with validation and a sensitive value |
| `examples/consumer.rs` | subscriptions, presence, `Get` (blocking and async) |
| `tests/sketch.rs` | hello world line limit; every REQ id cited under `oec/sdk/` exists and is not deprecated |
| `c/include/oec.h`, `c/examples/` | C ABI (`oec-ffi`) contract, hello world and logger demo in C |

Check: `oec/sdk/check.sh` (part of `oec/check.sh`). It needs no network and has no dependencies.

## Module map

| Module | Main items | Topic of the guideline |
|---|---|---|
| `app` | `App`, `AppBuilder`, `AppHandle`, `Bootstrap`, `Broker`, `ConnectionState` | MG 6 MQTT, MG 9.2 lifecycle, RIG 4 storage |
| `identity` | `IdentificationLink`, `Oi4Identifier`, `Product`, `LocalizedText` | MG 4, MG 5 |
| `asset` | `AssetDef`, `Asset`, `HealthState`, `HealthDetail`, `ReferenceDesignation` | MG 1.3 represented entities, MG 8.3.2, MG 8.3.10 |
| `data` | `DataSetDef`, `Field`, `Value`, `Publication`, `Mode`, `Writable`, `DataSet` | MG 8.2, MG 8.3.8, MG 9.1.4/9.1.5/9.1.8 |
| `config` | `ConfigDef`, `ConfigObject`, `ConfigValues`, `ConfigHandle` | MG 8.3.3, MG 9.1.3 |
| `event` | `SyslogLevel`, `GenericEvent`, `syslog` | MG 8.3.6, MG 9.1.6 |
| `consumer` | `Consumer`, `Target`, `Received`, `Presence`, `GetOptions`, `Subscription` | MG 7, MG 8.3.9, MG 9.1.9, MG 9.3 |
| `status` | `StatusCode` | MG 8.3.6.1 |
| `store` | `Store`, `FileStore`, `MemoryStore` | RIG 4.1 application storage |

## Flows (what the SDK does without application code)

**Start (`App::run`)**
1. `build()`: validate the identity (REQ-IL-008), the level and the store (ADR-0020 §3). In a container, read `broker.json`, CA and credentials (REQ-CNT-004, REQ-CNT-009).
2. Connect with TLS (REQ-MQTT-005) and a new Will with the current MessageId (REQ-LIFE-008, REQ-LIFE-016).
3. Subscribe to the own `Get`/`Set`/`Del`/`Call` topics of the level (REQ-CMP-008).
4. Birth first (REQ-LIFE-001, REQ-LIFE-015), then Health and the MAM/Health of all assets (REQ-CMP-006, REQ-CMP-007), then the publishing entries of the PublicationList.
5. Health heartbeat every 60 s (REQ-TOP-017); cyclic publications per Interval (REQ-RES-PUBLIST-013).

**`Get` with pagination**
1. Parse topic and payload (tolerant reader, REQ-MSG-046) and resolve Source/Filter.
2. Unknown Source: no `Pub`, Status event `BadNotFound` (REQ-CMP-047). Empty resource: empty list (REQ-CMP-041).
3. Take a snapshot of the addressed DataSets (REQ-PAG-007), apply Locale/Query and cut pages by MaxPacketSize and PerPage (REQ-PAG-002, REQ-PAG-005).
4. Publish each page with `ReplyTo` = MessageId of the request (REQ-MSG-015).

**`Set` of a DataSet**
1. Address check: unknown → `BadNotFound`, not writable → `BadNotWritable` (REQ-CMP-043).
2. Type check against the field definitions → `BadTypeMismatch`.
3. Application handler `on_set` → `Ok` or a `StatusCode`.
4. On `Good`: store the new values and publish the changed DataSet (REQ-RES-DATA-012).
5. Always: Status event with `ReplyTo` (REQ-RES-EVENT-005, REQ-TOP-014).

**`Set` of the PublicationList** – the same pipeline:
1. Only fields allowed by the entry `Config` change (REQ-RES-PUBLIST-016).
2. The new entry is persisted and republished (REQ-RES-PUBLIST-025).
3. The publication schedule changes immediately.

**Connection loss**
1. `ConnectionState::Reconnecting`.
2. Reconnect with backoff and a new Will (REQ-LIFE-016).
3. Birth again, then the current state (REQ-LIFE-001).

Updates in between only change the state store; the newest state is published after the reconnect.

**Shutdown (SIGTERM, `AppHandle::shutdown`)**
1. Close message (REQ-LIFE-005).
2. Disconnect.
3. `run()` returns `Ok`.

## Defaults

| Item | Default | Source |
|---|---|---|
| Bootstrap | container storage under `/` | ADR-0020 §5 |
| Store | `FileStore` in `/etc/oi4/app/oec` (container), required when ReferenceDesignation is offered | REQ-CNT-010, REQ-RES-REFDES-002 |
| Keep alive | 60 s | REQ-MQTT-014 |
| ClientId | container name / serial number | REQ-MQTT-013 |
| Data Mode / Interval | `ON_REQUEST_FILTER_9` / 0 (on change) | ADR-0020 §7 |
| MAM, Health, Profile, ReferenceDesignation Mode | `ON_REQUEST_SOURCE_10` | ADR-0020 §7, SF-101 |
| Lowest published Syslog level | `Notice` | REQ-TOP-024 |
| DataSetWriterIds | from 10 | REQ-MSG-020 |
| SoftwareRevision / HardwareRevision / DeviceRevision of an app | `"undefined"` / `""` / `""` | REQ-RES-MAM-010, REQ-RES-MAM-008 (examples of MG 5) |
| DeviceClass of an app | `Oi4.<ServiceType>` | REQ-RES-MAM-014 |
