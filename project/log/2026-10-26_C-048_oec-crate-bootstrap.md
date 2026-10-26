# C-048 – P4 increment 4: crate `oec`, bootstrap, store, errors

- **Date:** 2026-10-26
- **Phase:** P4 – SDK core implementation (D2), third cycle (cycle cut: ROADMAP P4)
- **Baseline:** guideline `734c560` (register `fce5cf5`, branch head, only `findings/`), unchanged; schema `2.0.1-dev` @ `734c560`
- **Start:** workspace after C-047 addendum 1 (`cc50c07`), new agent session (G-3.10)

## Intent

STATUS "Next step" 1 and ROADMAP P4 asked for increment 4:
1. the crate `oec` in `oec/sdk/rust` (ADR-0026 §2: the public API of ADR-0020; it re-exports or wraps `oec-core`, which stays sans-IO); the sketch API moves into `oec` piece by piece, and what is not done yet stays in the sketch and keeps compiling;
2. `App`/`AppBuilder` and `Level` (ADR-0020 §2/§3): `build()` rejects configurations that cannot keep the level or have an invalid identity; `run_async` (Q-021) only as a form if no runtime is needed; connection, lifecycle and MQTT belong to increment 5;
3. `Bootstrap` (ADR-0020 §5): `Bootstrap::container()` with the container storage contract and a `root` prefix, `Bootstrap::Explicit(Broker)`; the `broker.json` vectors as tests, with `strict-validation` the same verdicts as `platform/broker.schema.json`;
4. `Store` (persistent and `MemoryStore`) and `Error` (ADR-0020 §14), string-keyed Config getters (Q-022) where no runtime is needed;
5. thiserror and log under `cargo deny`;
6. a re-check of the P4 cycle cut.

Baseline check at the start:
- guideline head `fce5cf5` (only `findings/`) over text `734c560`;
- SF-001 … 104 resolved, none open;
- 1399 inventory items, 440 REQs, 158 `schema`/`d7-validate` REQs with 0 gaps, no drafts;
- entry schemas 29 of 29, 245 payload vectors, 18 `broker.json` vectors;
- `oec/check.sh` green.

The guideline did not change in this cycle, so no re-baseline was needed.

## Cycle cut (re-checked, agent decision G-3.4 b)

**C-048: no split.** All six parts are delivered and green. About 2,550 lines of product code and 600 lines of integration tests in `oec`, plus four small additions to `oec-core`. It is one review unit: the builder, the bootstrap and the store meet in `build()`, and the `broker.json` vectors test them together.

**Plan re-cut: increment 5 is split into two cycles** (ROADMAP P4 table):
- **C-049 – 5a (sans-IO, `oec-core`):** MQTT 3.1.1 codec and session from the spike `mqtt-codec`, with fuzzing/property tests; the lifecycle state machine and the Birth/Will/Close/Health messages as pure functions. No runtime dependency.
- **C-050 – 5b (runtime, `oec`):** tokio + rustls transport, `App::run`/`run_async`/`handle`, `AppHandle`, `ConnectionState`, `on_connection`, signals, reconnect, Health heartbeat.

Reason: 5a is a large, dependency-free unit with its own fuzzing; 5b brings the largest dependency step of P4 (async runtime, TLS) and the first network code. One cycle for both would mix two review topics, as the C-046 cut avoided for increments 1–3. Increment 9 (dev broker) follows 5b, then the P5 interleave, 6–8 and 10, as before.

## Work

### Dependencies (`7ae45dc`)

| Crate | Version | Features | Role |
|---|---|---|---|
| thiserror | 2 (2.0.21, + thiserror-impl) | default | `oec::Error` |
| log | 0.4 (0.4.34) | default | SDK diagnostics (no secrets, G-4.5) |
| serde_json | 1 (workspace) | default | reading `broker.json` |

- Tree of `oec`: 22 crates in the default build, 74 with `strict-validation` (unique crates, normal edges).
- `cargo deny check`: advisories, bans, licenses and sources ok. **No exception and no `ignore` entry**; both new crates are MIT OR Apache-2.0 (G-4.4).
- Base64 for the container secrets is a small decoder in `oec` (standard alphabet, decoding only) rather than one more crate.
- ADR-0023 amendment C-048 records this.

### `oec-core` additions (`5f00217`)

Small and I/O-free:
- `LocalizedText: From<&str>/From<String>` (locale `en-US`, REQ-RES-MAM-024), as the sketch promised;
- `il::valid_manufacturer_uri` public: the MAM ManufacturerUri uses the rule of the Oi4Identifier (REQ-RES-MAM-004);
- `Filter::context_name`: a Config context name is a topic Filter element (REQ-RES-CONFIG-015, REQ-TOP-019);
- `Validator::schema(path)`: any embedded schema, used by the `broker.json` parity test.

### Crate `oec` (`7ae45dc`)

| Module | Content | REQs (main) |
|---|---|---|
| `lib` | `Level` (L1 < L2 < L3), `Error` (`Invalid`, `LevelNotMet`, `Bootstrap`, `Io`, `Timeout`, `Closed`; `IlError`/`TopicError` → `Invalid` with message and codes), `Result`, `prelude`, `ServiceType` | ADR-0020 §3/§14 |
| `app` | `Bootstrap` (`container()` = `ContainerStorage { root: "/" }`, `Explicit(Broker)`), `Broker` (port 8883, MaxPacketSize 262144 by default), `AppBuilder` (+ `reference_designation`), `App` with the resolved `ConnectionSettings`, `Auth`, `Secret` | REQ-CMP-001/002/020, REQ-MQTT-004/005/008 … 015, REQ-RES-REFDES-002, REQ-PAG-005, REQ-LIFE-008 |
| `container` | the container storage contract below `root` (details below) | REQ-CNT-003/004/005/008/009/010/011/019/021, SF-100 |
| `address` | Address rule of `broker.json` (IPv4, IPv6, host name), the same as `platform/broker.schema.json` and the Go oracle | REQ-CNT-003 |
| `identity` | `IdentificationLink` and `Oi4Identifier` (wrappers), `Product` → MAM, `LocalizedText`, `Mam` | REQ-IL-*, REQ-RES-MAM-001 … 018, 024 |
| `store` | `Store` (+ `is_persistent`), `FileStore` (atomic per key), `MemoryStore` | REQ-CNT-010, REQ-RES-MAM-018, REQ-RES-REFDES-002 |
| `config` | `ConfigType`, `ConfigObject`, `ConfigDef` (+ `context`, `values`), `ConfigValues` (`number`/`string`/`boolean`/`changed`) | REQ-RES-CONFIG-001 … 015, 024/025 (Q-022) |
| `status`, `event` | `StatusCode` (+ `code()`), `SyslogLevel` | MG 8.3.6.1, REQ-TOP-024/027 |

**`build()`** validates without connecting:
1. keep alive (whole seconds 1 … 65535);
2. the bootstrap: container storage or explicit `Broker`;
3. the identity: an explicit IL is kept (REQ-IL-006/031), otherwise the Oi4Identifier is built from the `Product` and the serial number (REQ-IL-007, REQ-RES-MAM-006);
4. the level: ReferenceDesignation is offered by default at L2/L3, is required at L3 (REQ-CMP-020) and needs a persistent store (REQ-RES-REFDES-002), otherwise `LevelNotMet`;
5. the RevisionCounter from the store (REQ-RES-MAM-018);
6. the MAM (REQ-RES-MAM-*): defaults Manufacturer = ManufacturerUri, SoftwareRevision `undefined`, DeviceClass `Oi4.<ServiceType>`, ProductInstanceUri = IL, ManufacturerUri and DeviceManual checked;
7. the own publish limit (≤ MaxPacketSize of the bus, REQ-PAG-005) and the ClientId (container name, explicit, or derived from the IL).

**Container storage contract** (all paths below `root`; ADR-0026 §8.2):
- container name = hostname (`proc/sys/kernel/hostname`, else `etc/hostname`), an RFC 1123 label: SerialNumber and ClientId (REQ-CNT-019/021, REQ-MQTT-013);
- `etc/oi4/mqtt/broker.json`: `Address`, `SecurePort`, `MaxPacketSize` checked as by the schema, integers only, unknown elements ignored (REQ-CNT-003/004);
- CA: `broker_ca.pem`, else `etc/oi4/certs/ca.pem` (REQ-CNT-004/005, SF-100);
- authentication: client certificate (`etc/oi4/certs/<name>.pem` + `run/secrets/oi4/private_key.pem`, optional `passphrase`) wins over `credentials` (REQ-CNT-008/009, REQ-MQTT-010);
- store: `FileStore` in `etc/oi4/app/oec` if the application volume exists (REQ-CNT-010/011), else `MemoryStore` with a warning.

**REQ tracing:** `oec` cites 89 distinct REQ ids; the REQ walker of the sketch checks them (green).

### Sketch, READMEs, gate comment (`c118359`)

- `sketch/src/store.rs`: `MemoryStore::new()`/`Default` (the only changed sketch item); the sketch compiles with all examples.
- `sketch/README.md`: which items moved into `oec` and which stay until increments 5–10; the ClientId default row now says what the builder doc already said (derived from the IL).
- `oec/sdk/rust/README.md`, `oec/sdk/check.sh` header: crate `oec`, its dependencies, the `broker.json` vector test.

### Tests

| Test | Build | What it checks |
|---|---|---|
| `tests/broker.rs` `broker_json_vectors` | both | all 18 `broker.json` vectors (4 valid, 14 invalid) end to end through `Bootstrap::ContainerStorage` with a temporary root: valid ones build an `App` with the vector's host, port and MaxPacketSize; invalid ones fail with `Error::Bootstrap` citing REQ-CNT-003; the export baseline equals `oec_core::schema::BASELINE` |
| `tests/broker.rs` `same_verdicts_as_the_schema` | strict | the same 18 vectors against the embedded `platform/broker.schema.json`: verdicts and REQ ids equal the vectors |
| `tests/app.rs` (18, strict 19) | both | container defaults; `broker_ca.pem` wins; no CA; missing `broker.json`; hostname rules; client certificate wins; key without certificate; invalid secrets; no application volume; container identity rules; RevisionCounter from the store; explicit broker and client certificate; explicit rejections; levels (incl. ReferenceDesignation × store); invalid identity; `App: Send + Sync`; error display; strict: the MAM passes the strict schema |
| unit tests | both | 17 (default) / 19 (strict): store keys and atomic save, base64, `broker.json` parsing, address corpus (strict: the same corpus against the schema), identity, Config (strict: a Config DataSet passes the schema), `StatusCode::code` |
| doc test | both | the crate example compiles |

Results: default build 17 + 18 + 1 + 1 tests, strict build 19 + 19 + 2 + 1. All pass. Every vector verdict agreed at the first full run; no vector was changed.

## Agent decisions (G-3.4 b, for information: REVIEW A-17 … A-24)

Recorded in ADR-0020 amendment C-048, ADR-0026 amendment C-048 §8 and ADR-0023 amendment C-048 (`baecb34`).

1. **A-17 Crate structure and errors.**
   - `oec` re-exports the `oec-core` types that already have the right shape (`ServiceType`, `SyslogLevel`, `LocalizedText`, `Mam`, `ConfigType`) and wraps `IdentificationLink`/`Oi4Identifier`, whose errors are part of the API.
   - `Error` is the sketch enum with thiserror; core errors become `Invalid` with message and violation codes. `StatusCode` stays separate (§14).
   - All file access is in `oec`; `oec-core` stays sans-IO.
2. **A-18 Container storage contract details.**
   - Hostname from `/proc/sys/kernel/hostname`, else `/etc/hostname`; must be an RFC 1123 label.
   - `SecurePort`/`MaxPacketSize` must be JSON integers: `8883.0` is refused, as by the Go oracle, although JSON Schema would accept it as an integer value. No vector has this case, so no verdict differs.
   - The CA is required in a container (TLS only, REQ-MQTT-005/008); without one the bootstrap fails instead of falling back to the OS trust store (ADR-0023: trust comes from the storage).
   - A client certificate wins over credentials. A key without its certificate falls back to the credentials with a warning. Malformed base64 is always an error. Base64 tolerates missing padding and surrounding white space.
3. **A-19 Explicit bootstrap and ClientId.**
   - Outside a container a serial number is required (REQ-RES-MAM-015).
   - The default ClientId is `oec` + the FNV-1a 64 hash of the IL (19 characters, stable across restarts, REQ-MQTT-013).
   - A `Broker` without authentication → `LevelNotMet` (every level authenticates, REQ-CMP-002, REQ-MQTT-009); a missing file → `Io`.
   - In a container, `serial_number` needs an explicit IL and `client_id` is refused (the container name is both, REQ-CNT-021, REQ-MQTT-013).
4. **A-20 Level, ReferenceDesignation and store.**
   - New `AppBuilder::reference_designation(bool)`: default off at L1, on at L2/L3; off at L3 → `LevelNotMet` (REQ-CMP-020).
   - New `Store::is_persistent()`; offered with a non-persistent store → `LevelNotMet` (REQ-RES-REFDES-002). An explicit bootstrap defaults to `MemoryStore`, so a device at L2/L3 sets a `FileStore` (or, at L2, switches the ReferenceDesignation off).
   - `MemoryStore::new()` instead of a unit struct (ADR-0020 amendment, the sketch follows).
5. **A-21 Config without runtime.**
   - `ConfigDef::values()` validates the definition (context as a Filter, names, uniqueness, defaults against Type and Validation) and returns the initial `ConfigValues` with the string-keyed getters (Q-022).
   - **Pattern** is stored but not checked yet: that needs a regex crate in `oec` (ADR-0023 decision in increment 7, together with `Set Config`).
6. **A-22 Store and connection settings.**
   - Store keys: `/`-separated segments of `[A-Za-z0-9._-]`, no leading `.`; atomic save (temp file, sync, rename, directory fsync on Unix). RevisionCounter under `app/revision-counter`.
   - Keep alive: whole seconds 1 … 65535 (0 would disable the Will trigger, REQ-LIFE-008).
   - Own MaxPacketSize: clamped to the bus value with a warning, 0 refused (REQ-PAG-005).
   - `Secret` never shows its bytes in `Debug` (G-4.5).
7. **A-23 Dependencies.** thiserror 2 and log 0.4 only; `cargo deny` without exceptions; own base64 decoder.
8. **A-24 Cycle cut and Q-021.**
   - C-048 not split; increment 5 split into 5a (C-049, sans-IO) and 5b (C-050, runtime).
   - `run`/`run_async`/`handle`/`AppHandle`/`ConnectionState`/`on_connection` stay in the sketch until 5b: they need the runtime. No placeholder `run_async`; `App: Send + Sync` is tested instead.

## Changes to reviewed items

None:
- no REQ, vector, matrix entry or PO-accepted decision content changed;
- ADR-0020 and ADR-0023 (PO-accepted) got agent amendments within their decisions (additions allowed by ADR-0020 §1; one construction change, `MemoryStore::new()`; libraries listed in ADR-0023 §1);
- ADR-0026 is agent-accepted and got §8;
- schema unchanged (`2.0.1-dev`), no CHANGELOG entry.

## Spec findings

None. The container storage contract (RIG 4.1, REQ-CNT-*) and the vectors were sufficient, and every vector verdict agreed with `platform/broker.schema.json`. How the hostname is read and that JSON integers are required are SDK choices (A-18), not gaps. Next free id stays SF-105.

## Risks and open technical points (for the next cycles)

- **Encrypted private keys** (`passphrase`, REQ-CNT-008): rustls/rustls-pki-types cannot decrypt PKCS#8. 5b needs a decision (e.g. RustCrypto `pkcs8` with `encryption`) under ADR-0023, or a documented limitation.
- **Explicit `Broker` without `ca_pem`:** 5b decides whether this is an error or an opt-in OS trust store (ADR-0023: storage only by default).
- **Regex for Config Pattern** (increment 7, A-21).
- **Store key length** for represented assets (increment 6): keys will need a hashed or encoded IL.

## Commits

Workspace (never pushed):
- `5f00217` `oec-core` additions;
- `7ae45dc` crate `oec`, workspace dependencies, `Cargo.lock`, tests;
- `c118359` sketch `MemoryStore::new()`, READMEs, gate comment;
- `baecb34` ADR-0020 amendment C-048, ADR-0026 §8, ADR-0023 amendment C-048;
- this log, STATUS, ROADMAP; then REVIEW.md/REVIEW_DE.md.

Guideline: none.

## Checks (final)

`oec/check.sh`: all checks passed. Included in it:
- spectool `lint`, `render -check`, `coverage -check -gate` (0 gaps), `topic -check`, `inventory -check -rev 734c560` (1399 items);
- `schema/check.sh`, release freeze, `spectool crosscheck`;
- `sdk/check.sh`: the workspace (`oec-core`, `oec`) in both feature sets with fmt, clippy pedantic, tests and doc; cargo-deny; the sketches (REQ walker) and the spikes;
- the Go tests of `oec/spec/tools` and `oec/conformance`.

## For the PO (review)

Nothing to decide and no test data to review: no D-x or R-x. For information only (`REVIEW.md` A-17 … A-24): the agent decisions above, notably the new builder switch `reference_designation(bool)`, `MemoryStore::new()` and the split of increment 5.

## Next

C-049 – P4 increment 5a (ROADMAP P4): sans-IO MQTT 3.1.1 codec and session in `oec-core` (from the spike `mqtt-codec`) with fuzzing/property tests, lifecycle state machine and the Birth/Will/Close/Health messages as pure functions; no runtime dependency.

Next free ids (unchanged): SF-105, REQ-CMP-048, REQ-RES-PUBLIST-028, REQ-TOP-029, V-PAY-246, V-TOP-138 (valid) / V-TOP-255 (invalid), V-BRK-019, P-129, ADR-0027, Q-031.

**Agent for C-049 (G-3.10): new agent.**
- The context of this session was compacted during C-048.
- C-049 starts a new increment (5a) in other files: `oec-core` (new modules for MQTT and lifecycle), the spike `mqtt-codec`, ADR-0023 (MQTT codec, fuzzing), MG 6 and MG 9.2 (REQ-MQTT-*, REQ-LIFE-*).
- The state is handover-ready (G-3.7): this log, STATUS "Next step" 1, ROADMAP P4 and `REVIEW.md`.

Start prompt: `prompt.md` (German, not committed).
