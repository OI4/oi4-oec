# ADR-0026 – Rust SDK workspace, crate roles and test-vector contract (P4)

- Status: **accepted (agent, G-3.4 b)** – proposed in C-046, accepted by the agent in C-046 addendum 1 after the PO delegated engineering decisions (GUARDRAILS G-3.4 amended 2026-10-26). Amendment C-047 (§7, increment 3), C-048 (§8, increment 4), C-049 (§9, increment 5a) and C-050 (§10, increment 5b; wording fix of §9.6) and C-051 (§11, increment 9: dev environment, interop tests) by the agent (G-3.4 b).
- Date: 2026-10-26
- Deciders: agent (delegated by the product owner, G-3.4 b); the PO can overrule
- Related: ADR-0011 (S4: `oec-core` sans-IO, `oec` runtime, `oec-ffi`), ADR-0012 (monorepo), ADR-0018 (spec baseline), ADR-0020 (public Rust API), ADR-0021 (C ABI), ADR-0023 (dependencies, supply-chain rules), GUARDRAILS G-3.4 (b), G-4.4, SF-104

## Context

P4 starts with C-046. ADR-0011 named the three crates `oec-core`, `oec` and `oec-ffi`, and ADR-0020/0021/0023 fixed the public API, the C ABI and the dependencies. Four things were still open:
- the layout of the workspace and how it relates to the P3 sketch and spikes;
- the role and stability of `oec-core`;
- how the SDKs read the shared test vectors of `oec/spec/vectors/` (YAML);
- the lints and gates of product code.

G-3.4 first made the repository layout a PO decision, so C-046 proposed this ADR. On 2026-10-26 the PO delegated engineering decisions such as the layout to the agent (G-3.4 b), so the agent accepted it in C-046 addendum 1. Point §5.2 depended on the spec finding SF-104, which stayed with the PO; the PO decided it in C-046 addendum 2 (§5.2 follows), and the guideline text says so since `734c560`.

## Decision

### 1. Workspace layout

- `oec/sdk/rust/` is one Cargo workspace (resolver 3). Its members are flat folders named after the crates: `oec-core` (from C-046), `oec` (from increment 4) and `oec-ffi` (increment 10).
- `sketch/` and `spike/` stay stand-alone crates outside the workspace (`exclude`), with their own lock files. They are frozen.
  - The sketch is deleted when `oec` implements its API (ADR-0020 §1).
  - At that point its REQ-tracing walker (`sketch/tests/sketch.rs`) moves into the workspace as a test of `oec`. Until then it also checks the REQ ids cited in the workspace.
- `Cargo.lock` is committed (G-4.4). Every crate has `publish = false` until a publication decision (P9).
- Shared settings live in `[workspace.package]`: edition 2024, `rust-version = "1.89"` (ADR-0023 §2), license MIT. Shared dependency versions live in `[workspace.dependencies]`.

### 2. Crate roles

- **`oec-core`** holds the sans-IO protocol logic. It does no I/O, has no runtime and no global state.
  - Increments 1–2 have no runtime dependencies at all; base64url and the percent-encoding are about 100 lines each.
  - From increment 3, it may use only the I/O-free crates of ADR-0023: serde, serde_json, jsonschema behind `strict-validation`, time and uuid.
  - It is an internal crate with no stability promise of its own.
- **`oec`** is the public API of ADR-0020. It re-exports or wraps `oec-core` types.
  - Where the accepted sketch signature differs, `oec` adapts and the sketch wins. For example, `oec::Result` with `Error::Invalid(String)` wraps the structured `oec-core` errors.
- **`oec-core` errors are structured:**
  - `IlError` lists all violations, each with a stable code that is also the vocabulary of the IL vectors: `empty`, `too-long`, `forbidden-character`, `empty-authority`, `uppercase-authority`, `encoding`, `manufacturer-uri`. There is no SDK-only code (the former `slash-in-element` went away with SF-104, §5.2).
  - `TopicError` has the kinds `grammar` and `identification-link`, which are the `fails` values of the topic vectors.
- **Names follow the sketch:** `Resource::Mam`, `ServiceType::OTConnector`, and `SyslogLevel` ordered from Emergency to Debug.
  - `oec-core` adds two internal names: `Subject` (resource or service in the fifth topic level) and `Form` (the allowed Method × Resource combination with Source/Filter presence).

### 3. Test-vector contract: JSON export of the spec baseline

- The SDKs read the shared vectors and the Method × Resource matrix as JSON from `oec/spec/export/`, not from the YAML sources.
  - `spectool render` writes the export, and `render -check` (step 1 of `oec/check.sh`) fails when it is stale.
  - `rebaseline` and `review` render it as well.
- **Format:** `$comment`, `source`, then exactly the fields of the Go model `vectors.File` (`kind`, `baseline`, `description`, `vectors[]`) or of the matrix (`baseline`, `combinations[]`).
  - Go zero values are omitted. For example, a missing `encoded` is the empty string (V-IL-116), and a missing `warnings` is "none".
- **Reason:**
  - Without the export, the Rust and C SDKs would need a YAML parser just for their tests. The de-facto Rust crate `serde_yaml` is archived and published as `+deprecated`, and D9 would need a C YAML library.
  - JSON is already a dev dependency (serde_json) and is easy to turn into C arrays for D9 host tests (P4b).
  - The YAML stays the reviewed source; the export is generated and never edited.
- **Use:** every SDK has one acceptance test that runs all vectors relevant to its increments. Today that is `oec-core/tests/vectors.rs`:
  - IL, dnp and Oi4Identifier vectors;
  - all 151 topic vectors (parse fields, rule, warnings, exact round trip, failure kind);
  - the topics of all valid payload vectors;
  - the matrix, row by row against the `Form` table.

### 4. Lints and gates

- **Workspace lints:**
  - `unsafe_code = "forbid"`. `oec-ffi` will opt out with `deny` plus a documented `allow` per FFI module (ADR-0021).
  - `missing_docs` and `unreachable_pub`.
  - clippy `all` + `pedantic`, without `module_name_repetitions` and `must_use_candidate`.
  - `doc_markdown` is allowed because the doc comments use the CamelCase terms of the guideline (ManufacturerUri, PublicationList, …) as prose.
- **Gate:** `oec/sdk/check.sh` (step 5 of `oec/check.sh`) runs:
  - `cargo fmt --check`;
  - `cargo clippy --all-targets -D warnings`;
  - `cargo test`;
  - `cargo doc` with warnings as errors;
  - `cargo deny check`.

  Everything except the advisory DB fetch runs `--offline --locked`.
- **`deny.toml` encodes ADR-0023 §2:**
  - license allowlist MIT, MIT-0, Apache-2.0, ISC, BSD-2/3-Clause, Zlib, Unicode-3.0;
  - advisories, unmaintained and yanked crates are errors;
  - one version each of `rustls`, `rustls-webpki`, `rustls-pki-types`, `tokio-rustls`, `ring`, `aws-lc-rs`, `aws-lc-sys`;
  - `rustls-pemfile`, `openssl`, `openssl-sys` and `native-tls` are banned;
  - only crates.io as a source.

  Without network, the gate falls back to the cached advisory DB (`--frozen`). A real violation fails both runs. An accepted advisory needs an `ignore` entry with a reason that is logged in the cycle log.
- **REQ tracing:** every rule cites its REQ ids in a doc comment. The walker fails on unknown or deprecated ids; in C-046 it caught a citation of the withdrawn REQ-TOP-007, which became REQ-TOP-028.

### 5. Semantics fixed in increments 1–2 (SDK side, no spec change)

1. **Identification Link:**
   - It is kept exactly as given and compared character by character (REQ-IL-013/015).
   - `decode` accepts only the canonical unpadded base64url form (REQ-IL-004).
   - Octets that are not UTF-8 count as forbidden characters.
2. **Oi4Identifier:**
   - `build` always dnp-encodes Model, ProductCode and SerialNumber (REQ-IL-033, SF-104). Values made only of unreserved characters stay unchanged, so typical identifiers are not affected; the result is one Oi4Identifier per nameplate, as the spec tools and the vectors already build it.
   - History: C-046 made the dnp-encoding opt-in (`.dnp_encoded()`, as in the sketch), with the SDK-only violation `slash-in-element` for a `/` without it, because MG 4.3.1/A3 said "may" (SF-043 e). The PO question on it led to SF-104; the PO decided on 2026-10-26 that MG 4.1 governs ("must be masked by other means"), so C-046 addendum 2 removed the opt-in and the violation (also from the sketch).
3. **Topic parsing classifies like `spectool` and D7:**
   - First all structural rules, including the canonical `encoded-il` of AppId and Source → `grammar`.
   - Then the decoded AppId, then the Source, as Identification Links → `identification-link`.
   - `Display` is the exact inverse of `parse`.
4. **White space and control characters in Filter values** (REQ-TOP-020, SHOULD NOT) are valid but are reported by `Topic::warnings()`. The publishing side of `oec` decides how to surface them (increment 6).
5. **`TopicFilter`:**
   - The builder defaults to `Oi4v2/+/+/<Method>/<Resource>/#`.
   - A selection that can never match a valid topic is an error, e.g. "without Source" where the Source is required, or a Filter without a Source selection.
   - `matches` implements MQTT 3.1.1 §4.7.

### 6. P4 cycle cut

The increments of ROADMAP P4 are cut into reviewable cycles (ROADMAP P4 "Cycle cut"):
- C-046: increments 1–2, no runtime dependencies;
- C-047: increment 3, the message model with the first runtime dependencies and schema embedding.

P5 interleaves after increment 5 (`internal/mqttwire`, `monitor --replay`) because P5 does not depend on P4.

## Consequences

- `oec/check.sh` needs `cargo-deny` installed (`cargo install cargo-deny --locked`). The `oec/sdk/check.sh` header documents it, and the gate fails with that hint when it is missing.
- D9 (P4b) reuses the JSON export for its host tests. The C vocabulary of violation codes should match §2.
- Every change to `oec/spec/vectors` shows up in `oec/spec/export` in the same commit (`render -check`).
- The sketch and the workspace exist side by side until `oec` replaces the sketch. Both are checked.

## Amendment C-047 (2026-10-26) – increment 3: message model and schema embedding (agent, G-3.4 b)

Decided by the agent within ADR-0020/0023 and this ADR; the PO can overrule. Details and REQ tracing: `log/2026-10-26_C-047_message-model.md`.

### 7. Semantics fixed in increment 3 (SDK side, no spec change)

1. **Modules:** `oec-core::message` (envelopes `ua-data`, `ua-metadata`, `ua-action-request`/`-response`, helper DataSets, DataSetMetaData, value types) and `oec-core::resource` (the payloads of MAM, Health, Profile, PublicationList, SubscriptionList, ReferenceDesignation, Config/ConfigSet, Data with the Oi4Pv view, Event). `Message::decode(&Topic, &[u8])` picks the type from the topic form (e.g. `Set/Config` → `ConfigSet`, `Call/NewDataSetWriterId` → Action request).
2. **Tolerant decoder, typed by construction:** decoding ignores unknown keys (REQ-MSG-046; no `deny_unknown_fields`) and refuses what the type cannot hold: wrong JSON types, missing required fields, values outside closed sets, and model invariants (DataSetClassId of the resource, MessageType, reserved DataSetWriterIds, MetaDataVersion vs. Major/MinorVersion, Good `Status`, Action correlation fields). String patterns and cross-field rules that the type does not need are left to the schema (`strict-validation`) and the D7 rule layer. Measured on the vectors: 63 of 143 invalid payloads are refused by the decoder, the rest by the schema or rule layer.
3. **Encoder = fixed content masks:** the SDK always writes the fields of the fixed content masks (REQ-MSG-004, REQ-MSG-016/017) in the guideline order, always sends the DataSetMessage `MessageType`, never a DataSetMessage `PublisherId`, and `ReplyTo` as `""` when there is no trigger. `DataSetClassId` is set from the resource (REQ-MSG-010/011); a publisher-defined one (Data, Metadata) refuses reserved and predefined ids (REQ-MSG-014).
4. **Lossless values:** `DateTime` and `ByteString` keep their text (a decoded `CorrelationData` is echoed byte-identical, REQ-ACT-007); numbers that are int or float in the schema (`TimeoutHint`, Min/Max, Precisions) are `serde_json::Number`. Guids only in the lowercase hyphenated form. Object keys are kept in sorted maps (no `preserve_order`): key order is not significant in JSON.
5. **Action correlation:** `ActionRequestMessage::new` takes the PublisherId from the Call topic (the Responder, REQ-ACT-005) and sets `ResponseAddress` to its Reply topic (REQ-ACT-006); `response()` copies PublisherId, RequestorId and CorrelationData; `ActionRequest::done`/`failed` build the matching ActionResponse (same DataSetWriterId, ActionTargetId, RequestId; a Bad Status has an empty Payload, REQ-ACT-012/014). `ActionResponse::payload` is an `Option` (`None` ↔ `{}`).
6. **Config and Data:** Config groups are flattened maps (the group names are application-defined); Data is a raw JSON object with a typed `Oi4Pv` view (`Pv`, `Sv<n>`). SubscriptionList `TopicPath` stays a string (it may be any topic filter).
7. **Schema embedding:** `build.rs` reads `Version` and `Baseline` from `oec/schema/schema.go` (the tree that is embedded, e.g. `2.0.1-dev` @ `734c560`), not from `oec/schema/release/` as the C-043 spike did; with `strict-validation` it embeds the 45 strict sources by `include_str!`. `schema::Validator` derives the tolerant flavor in memory and routes and attributes errors to REQ ids like the schema gate (ADR-0019 §8).
8. **Tests as contract:** `tests/payload.rs` runs all 245 payload vectors: valid ones decode and re-encode to an equal JSON value (tolerant V-PAY-019/113: unknown keys dropped), invalid ones never panic, and in the `strict-validation` build the verdicts and REQ ids equal the schema gate, with the rule-layer cases of `oec/schema/tests/rule-layer.yaml` as the only valid-to-schema negatives; every re-encoding passes the strict schema. `tests/model.rs` covers the sender side (builders → strict schema → decode) and the Action correlation. The gate runs clippy, tests and rustdoc in both feature sets.

## Amendment C-048 (2026-10-26) – increment 4: crate `oec`, bootstrap and store (agent, G-3.4 b)

Decided by the agent within ADR-0020/0023 and this ADR; the PO can overrule. Details and REQ tracing: `log/2026-10-26_C-048_oec-crate-bootstrap.md`.

### 8. Semantics fixed in increment 4 (SDK side, no spec change)

1. **Crate `oec` (role of §2 made concrete):** a workspace member with the public API of ADR-0020 (amendment C-048 lists what moved). It re-exports value types of `oec-core` that are already in the right shape (`ServiceType`, `SyslogLevel`, `LocalizedText`, `Mam`, `ConfigType`) and wraps the ones whose error type or invariants are part of the API (`IdentificationLink`, `Oi4Identifier`). `oec-core` stays sans-IO; all file access (container storage, `FileStore`) is in `oec`. The errors of `oec-core` become `oec::Error::Invalid` with their message and codes (e.g. `[too-long]`), so the cause stays visible without exposing the core types.
2. **Container storage contract** (`Bootstrap::ContainerStorage { root }`, every path below `root`):
   - container name = hostname, read from `proc/sys/kernel/hostname`, else `etc/hostname`; it must be an RFC 1123 label (REQ-CNT-019/021). It is the SerialNumber of the Oi4Identifier and the MQTT ClientId.
   - `etc/oi4/mqtt/broker.json` (REQ-CNT-003/004): `Address` (IPv4, IPv6 or host name, the rule of `platform/broker.schema.json`), `SecurePort` 1 … 65535 and `MaxPacketSize` 262144 … 268435455 as JSON **integers** (a float such as `8883.0` is refused, as by the Go oracle of the D8 checks); unknown elements are ignored and logged at `debug`; all problems are reported at once with REQ-CNT-003.
   - CA: `etc/oi4/mqtt/broker_ca.pem`, else `etc/oi4/certs/ca.pem` (REQ-CNT-004/005, SF-100); neither → `Bootstrap`.
   - Authentication (REQ-CNT-008/009, REQ-MQTT-010): `run/secrets/oi4/private_key.pem` + `etc/oi4/certs/<name>.pem` (+ optional base64 `passphrase`) win over `run/secrets/oi4/credentials` (`base64(<username>:<password>)`, split at the first `:`). A key without certificate falls back to the credentials with a warning, otherwise → `Bootstrap`. Malformed base64 is always an error. Base64 is the standard alphabet with optional padding; surrounding white space (a final line break) is ignored.
   - Store: `FileStore` in `etc/oi4/app/oec` if the application specific volume `etc/oi4/app` exists (REQ-CNT-010/011), otherwise `MemoryStore` with a warning.
3. **`Store`:** keys are `/`-separated segments of `[A-Za-z0-9._-]`, not empty and not starting with `.` (no path traversal in the `FileStore`). `FileStore::save` is atomic per key (temporary file, `sync`, `rename`, directory `fsync` on Unix). The RevisionCounter lives under `app/revision-counter` (REQ-RES-MAM-018).
4. **Tests as contract:** `oec/tests/broker.rs` runs all 18 `broker.json` vectors of the export (V-BRK-001 … 018) end to end through `Bootstrap::ContainerStorage` with a temporary root: valid ones build an `App` with the expected connection settings, invalid ones fail with `Error::Bootstrap` citing REQ-CNT-003. With `strict-validation` the same vectors are validated against the embedded `platform/broker.schema.json` and the verdicts and REQ ids equal the vectors. `oec/tests/app.rs` covers the rest of the storage contract, the explicit bootstrap, identity, levels and the MAM (strict: the MAM passes the strict schema).
5. **Cycle cut:** C-048 delivers increment 4 as one cycle; the plan of §6 continues with increment 5 in C-049.

## Amendment C-049 (2026-10-26) – increment 5a: MQTT codec, session and lifecycle, sans-IO (agent, G-3.4 b)

Decided by the agent within ADR-0020/0023 and this ADR; the PO can overrule. Details and REQ tracing: `log/2026-10-26_C-049_mqtt-session-lifecycle.md`.

### 9. Semantics fixed in increment 5a (SDK side, no spec change)

1. **Modules (roles of §2):** `oec-core::mqtt` (packets, codec, `Session`) and `oec-core::lifecycle` (`Lifecycle`, `HealthBeat`, `check_health`), plus `message::MessageIds`. Both are sans-IO: no socket, no timer, no clock and no new dependency. The caller passes monotonic time (`Duration` since an origin it chooses) and, for MessageIds, the wall clock in Unix milliseconds. Transport, timers, reconnect/backoff and the public API (`App::run`, `AppHandle`, `ConnectionState`) stay in `oec` (increment 5b, C-050).
2. **Codec (REQ-MQTT-001, MQTT 3.1.1):** all packets a client sends or receives except the QoS 2 flow: CONNECT (Will, User Name/Password, Clean Session, Keep Alive), CONNACK, PUBLISH QoS 0/1, PUBACK, SUBSCRIBE/SUBACK, UNSUBSCRIBE/UNSUBACK, PINGREQ/PINGRESP, DISCONNECT. The codec is **symmetric and canonical**: `encode` and `decode` apply the same rules, and every accepted byte sequence re-encodes byte-identically (minimal Remaining Length, reserved bits/flags 0, no trailing bytes). Refused, each as a `CodecError` with the rule it enforces (`rule()`: `MQTT-x.y.z-n`, an MQTT section or a REQ id): QoS 2 and PUBREC/PUBREL/PUBCOMP (REQ-MQTT-017), a Will with Retain (REQ-MQTT-018), a password without user name (MQTT-3.1.2-22), Session Present with a refusal (MQTT-3.2.2-4), strings over 65535 bytes, invalid UTF-8 or U+0000, topic names with wildcards, malformed topic filters, empty SUBSCRIBE/UNSUBSCRIBE, packet id 0, a Remaining Length over 268435455 or over the decoder limit. Retain on a *received* PUBLISH is decoded (a broker may deliver retained messages of others); the session never sets it.
3. **Session:** one `Session` per client; `connect(will, now)` takes the Will of this connection, so every CONNECT carries a new Will (REQ-LIFE-016). `SessionConfig` = ClientId, Clean Session (default true, REQ-MQTT-015), Keep Alive (default 60 s, REQ-MQTT-014), credentials, MaxPacketSize (≥ 262144, RIG 4.1.1) and the in-flight limit (default 64). An empty ClientId needs Clean Session (REQ-MQTT-016).
   - **Timers:** CONNACK is awaited for one keep-alive period; PINGREQ is due one keep-alive period after the older of the last sent and the last received packet; PINGRESP is awaited one keep-alive period. Keep Alive is a `NonZeroU16` in seconds: the SDK never connects with Keep Alive 0 (no keep alive), so a dead broker is always detected.
   - **Publishing:** only while connected; Retain is never set; a payload over MaxPacketSize is refused (`PayloadTooLarge`, RIG 4.1.1, REQ-PAG-005), as is a QoS 1 publish beyond the in-flight limit. Packet ids run 1 … 65535 and wrap, skipping ids still in use.
   - **Subscriptions** may be requested in any state and are sent when connected. After a CONNACK without Session Present the session restores all granted subscriptions (`Event::Subscribed { restored: true }`), and in-flight QoS 1 messages are reported as `Event::Dropped` (the runtime decides whether to republish). With Session Present they are resent with DUP (MQTT-4.4.0-1). Session Present on a Clean Session is a protocol error.
   - **Close:** `disconnect()` writes DISCONNECT, so the broker discards the Will. Every protocol error, timeout and `connection_lost()` closes **without** DISCONNECT, so the broker publishes the Will (REQ-LIFE-008). `Event::Closed` carries a `CloseReason`.
4. **Lifecycle messages:** the Will, Birth and Close have the shape of the MG 6.3 examples and of the vectors V-PAY-023/024/025: one DataSetMessage, an empty WriterGroupName, no Timestamp, no SequenceNumber and **no DataSetMessage `MessageType`** (REQ-MSG-029 assumes `ua-keyframe`). This is a deliberate exception to the "always sends `MessageType`" rule of §7.3 for these three messages and the Health, so the SDK sends what the guideline shows. Data and other resources keep §7.3. Will = Health FAILURE_1/0, QoS 0 (REQ-LIFE-008 … 010); Birth = MAM, QoS 0 (REQ-LIFE-001 … 003); Close = Health NORMAL_0/0 (REQ-LIFE-005 … 007).
5. **Order and Health:** `on_connected` gives the Birth first, then the Health (REQ-LIFE-001, REQ-LIFE-015, REQ-TOP-017). The Health is published on every change, including `Details` (REQ-RES-HEALTH-004), and every 60 s (`HEALTH_HEARTBEAT`, REQ-TOP-017). `check_health` refuses a HealthScore over 100 (REQ-RES-HEALTH-001) and a Health that is not **equal** to the most severe `Details` entry (REQ-RES-HEALTH-002). `set_health` also refuses NORMAL_0/0, which means Close only (REQ-LIFE-014). After the application reported FAILURE_1/0 about itself, the next non-failure Health is preceded by a Birth (REQ-LIFE-012, REQ-LIFE-017). `asset_health` refuses the AppId (REQ-CMP-007: the asset Health is for other assets).
6. **MessageIds (REQ-MSG-007/008):** `MessageIds` gives `<unix-ms>-<PublisherId>` for the first message in a millisecond and `<unix-ms><n>-<PublisherId>` (n = 1, 2, … appended to the timestamp, the form of REQ-MSG-007) for the next ones (wording fixed in C-050; the code always did this); a clock that goes backwards keeps the last millisecond, so ids stay unique and ordered per publisher.
7. **Tests as contract:** `tests/mqtt_codec.rs` (byte-exact packets, every refusal with its rule), `tests/session.rs` (scenarios against a scripted broker: connect, ping, timeouts, QoS 1, restore, resend, close), `tests/lifecycle.rs` (Will/Birth/Close equal V-PAY-025/023/024 as JSON values; Health rules; with `strict-validation` every generated payload passes the strict schema) and `tests/proptest_mqtt.rs` (round trip of random packets, random and mutated bytes never panic, any chunking decodes the same, random session runs keep the invariants). The proptests run in the normal gate on stable (ADR-0023 amendment C-049).
8. **Cycle cut:** C-049 delivers increment 5a as one cycle; C-050 is increment 5b (runtime: tokio + rustls, `run`/`run_async`/`AppHandle`/`ConnectionState`/`on_connection`, mapping `ConnectionSettings` → `SessionConfig`).

## Amendment C-050 (2026-10-26) – increment 5b: runtime on the sans-IO core (agent, G-3.4 b)

Decided by the agent within ADR-0020/0023 and this ADR; the PO can overrule. Details and REQ tracing: `log/2026-10-26_C-050_runtime-tokio-rustls.md`, REVIEW A-32 … A-41. Public API: ADR-0020 amendment C-050; dependencies: ADR-0023 amendment C-050.

### 10. Semantics fixed in increment 5b (SDK side, no spec change)

1. **Modules (roles of §2):** `oec` gets two internal modules: `transport` (TLS client configuration, TCP + TLS connect, small `poll_fn` read/write helpers) and `runtime` (the loop that drives `oec_core::mqtt::Session` and `oec_core::lifecycle::Lifecycle`). The public part is in `app` (ADR-0020 amendment C-050). `oec-core` is unchanged and stays sans-IO.
2. **TLS (REQ-MQTT-005/007/008/010, RIG 4.1):**
   - TLS 1.3 and 1.2 only (rustls has nothing older); the server name is the configured host (DNS name or IP address) and is verified by rustls/webpki;
   - trust anchors only from the CA file (`broker_ca.pem` or `ca.pem` of the storage, or `Broker::ca_pem`); every certificate in the file is added, a file without a usable certificate is `Error::Bootstrap`; never the OS trust store;
   - client certificate (chain from the certificate file) and key (PKCS#8, PKCS#1, SEC1, or encrypted PKCS#8 with the passphrase, ADR-0023 amendment C-050) when `Auth::ClientCertificate`; then no User Name/Password in CONNECT (REQ-MQTT-010);
   - the TLS material is loaded when `run` starts (fatal: `Error::Bootstrap`) and reloaded before every reconnect, so renewed certificates are used; if the reload fails, the last good configuration is kept with a warning.
3. **`ConnectionSettings` → `SessionConfig`:** ClientId (REQ-MQTT-013); Keep Alive in whole seconds as `NonZeroU16` (`build` already checked 1 … 65535, REQ-MQTT-014); Clean Session always true (REQ-MQTT-015); `Auth::Credentials` → User Name + Password, `Auth::ClientCertificate` → none; `max_packet_size` = the resolved limit (the MaxPacketSize of the Message Bus or the smaller own limit, RIG 4.1.1, REQ-PAG-005); in-flight limit default (64). Lifecycle DataSetWriterIds: MAM **10**, Health **11** (the first application ids, REQ-MSG-020); assets get theirs with increment 6.
4. **One connection:**
   - TCP connect and TLS handshake together are bounded by the keep-alive interval;
   - CONNECT carries a new Will built at that moment (REQ-LIFE-008, REQ-LIFE-016);
   - on CONNACK: backoff reset, then `Lifecycle::on_connected` → **Birth, then Health** are queued before anything else (REQ-LIFE-001, REQ-LIFE-015, REQ-TOP-017), then the state becomes `Online`. Because Clean Session is always true and the session refuses Session Present (§9.3), no resent message can precede the Birth: "Birth first" holds without extra code (C-048/C-049 open point decided);
   - while connected, the wake-up deadline is the earlier of `Session::poll_timeout` (keep alive, PINGRESP) and `Lifecycle::poll_timeout` (Health heartbeat 60 s, REQ-TOP-017); `Lifecycle::poll` gives the heartbeat;
   - writes are bounded by the keep-alive interval (no progress → connection lost); reads use a 16 KiB buffer; EOF, I/O errors and session timeouts call `connection_lost`/`handle_timeout`, so the session closes **without** DISCONNECT and the broker publishes the Will (REQ-LIFE-008);
   - received PUBLISH packets are logged at `debug` and dropped until the consumer side (increment 8) and `Get`/`Set` (increments 6/7) exist.
5. **Reconnect:** every lost, failed or refused connection is retried; the runtime never gives up on its own. A refused CONNACK (e.g. wrong credentials) is logged as an error and retried too, because the platform can fix the credentials without restarting the application. Backoff: 1 s, doubled per failed attempt up to 60 s, each delay randomized to 50 … 100 % (xorshift seeded from ClientId and clock, no `rand` crate) so that many applications do not reconnect in lockstep; reset by a CONNACK. After the first CONNACK a lost connection reports `Reconnecting`; before it the state stays `Connecting`.
6. **Shutdown:** `AppHandle::shutdown` sets a `tokio::sync::watch` flag. During connect or backoff the runtime stops at once. While connected it queues the Close (Health NORMAL_0/0, REQ-LIFE-005 … 007), then DISCONNECT (so the broker discards the Will, MG 6.3.4), then the TLS `close_notify`, all within 5 s; on timeout it logs a warning and stops. Then the state is `Closed` and `run` returns `Ok(())`.
7. **Runtimes and signals:** `App::run` builds a tokio `current_thread` runtime (`enable_io`, `enable_time`) on the calling thread, spawns the signal task and blocks on the loop. The signal task waits for SIGTERM/SIGINT (Unix) or Ctrl-C/Ctrl-Close/Ctrl-Shutdown (Windows; other targets: Ctrl-C) and calls `shutdown`. tokio keeps a signal registered for the life of the process, so after `run` returns, SIGINT/SIGTERM no longer terminate the process by default; an application that runs on after `run` handles them itself. `App::run_async` is the same loop without the signal task (ADR-0020 amendment C-050).
8. **Clocks:** the session and the lifecycle get monotonic time from a tokio `Instant` taken at the start of `run`; MessageIds use the wall clock (`SystemTime`, Unix ms, §9.6).
9. **Tests as contract:** `oec/tests/runtime.rs` runs the real runtime against an **in-process test broker**: a blocking std + rustls server on loopback (a thread per connection) that decodes with the `oec-core` codec and reports what it saw (`Tls` with whether a client certificate was presented, `Packet`, `Closed`). The PKI (CA, server, client certificates; an encrypted PKCS#8 key) is generated per run with rcgen; nothing is committed and nothing leaves loopback. Covered: CONNECT/Will/Birth/Health/Close/DISCONNECT order and payloads on one connection; the Will survives a dropped connection and the reconnect sends a new Will, Birth and Health; keep-alive timeout without PINGRESP closes without DISCONNECT; a refused CONNACK is retried; an untrusted broker never gets `Online`; client certificate over TLS 1.2 without credentials; encrypted key with passphrase; unusable TLS material fails `run`; shutdown before `run`; `run_async` on a spawned task; an unreachable broker keeps `Connecting`; `AppHandle: Clone + Send + Sync`. An optional smoke test runs against a local TLS broker when `OEC_SMOKE_BROKER` is set (README); without it, it does nothing, so the gate never depends on a broker. The 60 s heartbeat itself is covered by the core tests (`tests/lifecycle.rs`), not by waiting in the runtime tests.
10. **Cycle cut:** C-050 delivers increment 5b as one cycle (not split). Next is increment 9 (dev environment with a TLS broker and the interop tests), then increments 6–8, as planned in §6/ROADMAP P4.


## Amendment C-051 (2026-10-26) – increment 9: dev environment and interop tests (agent, G-3.4 b)

Decided by the agent within ADR-0012, ADR-0023 and this ADR; the PO can overrule. Details: `log/2026-10-26_C-051_dev-environment-interop.md`, REVIEW A-42 … A-49. Spec finding: SF-105 (open).

### 11. Dev environment `oec/dev` and the interop test contract

1. **Layout (ADR-0012):** `oec/dev/` holds `compose.yaml`, `mosquitto/mosquitto.conf`, the Go module `oec/dev` with the command `pki` (dev PKI generator, standard library only), `interop.sh` and a README. Everything the generator writes goes to `oec/dev/generated/` (git-ignored, marker file `.oec-dev-pki`; `-force` replaces only a directory with this marker). No key, password or certificate is committed (GUARDRAILS G-4.5).
2. **Broker:** Mosquitto **2.1.2** (`eclipse-mosquitto:2.1.2-alpine`), pinned by tag **and** multi-arch index digest (the practice of REQ-CNT-022 for an immutable image). Two TLS listeners, both TLS 1.2 minimum (`tls_version tlsv1.2`; TLS 1.3 is negotiated when offered), no plain and no anonymous listener (REQ-MQTT-005/006):
   - `8883`: user name and password from a Mosquitto password file (REQ-MQTT-009);
   - `8884`: client certificate required, issued by the dev CA, CN = user name (`use_identity_as_username`, REQ-MQTT-010).
   Ports are published on `127.0.0.1` only (overridable). Persistence is on (named volume), so sessions with Clean Session 0 survive a broker restart. The entrypoint copies the read-only TLS material to a directory owned by the broker user (mode 0600), so host ownership does not matter (Docker, Podman, rootless).
3. **Packet size (RIG 4.1.1, SF-091 (b), SF-105):** `MaxPacketSize` of the sample `broker.json` is 262144 and limits the **payload**. The broker drops larger payloads (`message_size_limit 262144`); its whole-packet limit `max_packet_size` is 262144 + 65544 (fixed header 5 + topic length 2 + topic 65535 + packet id 2), so every payload up to `MaxPacketSize` is accepted with any topic. A whole-packet limit equal to `MaxPacketSize` disconnects compliant publishers (measured), which is SF-105 (resolved after the C-051 review: RIG 4.1.1 since `59e3ae3` requires exactly this configuration).
4. **Dev PKI (`go run ./pki`):** ECDSA P-256 CA (3 years, path length 0) and leaf certificates (1 year): broker certificate (serverAuth; SAN `localhost`, `mosquitto`, `127.0.0.1`, `::1` plus `-san`); client certificates (clientAuth) for the container `oec-dev-cert` and for the probe client `oec-probe`; random passwords and passphrase; a Mosquitto `$7$` password file (PBKDF2-SHA512, 1000 iterations, the format of `mosquitto_passwd` 2.1, pinned by a captured vector); the key of `oec-dev-cert` as PKCS#8 PBES2 (PBKDF2-HMAC-SHA256 with the OpenSSL 3 default of 2048 iterations, AES-256-CBC) with a base64 passphrase; secret files with mode 0600. Its tests also lint `mosquitto.conf` and `compose.yaml` (listeners, TLS floor, authentication, packet limits, image pin, loopback ports) and run in the gate (`oec/check.sh` step 6) without a container engine.
5. **Sample container storages:** `generated/roots/oec-dev-app` (credentials, port 8883) and `generated/roots/oec-dev-cert` (client certificate with encrypted key and passphrase, port 8884) are complete roots of RIG 4.1 (`etc/hostname`, `etc/oi4/mqtt/broker.json` + `broker.pem`, `etc/oi4/certs/ca.pem` [+ `<container>.pem`], `run/secrets/oi4/credentials` or `run/secrets/oi4/private_key.pem` + `passphrase`, `etc/oi4/app/`) that `Bootstrap::ContainerStorage { root }` reads unchanged. `broker.json` has exactly the three keys of the vectors V-BRK-001 … 018; `Address` is `localhost` (the tests and examples run on the host; containers on the compose network use `mosquitto`, which the broker certificate also covers); there is no `broker_ca.pem`, so the fallback to `ca.pem` is exercised.
6. **Interop tests (`oec/tests/interop.rs`), opt-in:** they run only with `OEC_INTEROP=1` and otherwise return at once, so the gate compiles and lints them but needs neither a container engine nor the network. `oec/dev/interop.sh` generates the PKI if missing, starts the broker (`compose up --wait`), runs the tests single-threaded and removes the broker and its volume (`OEC_INTEROP_KEEP=1` keeps it); without a running container engine it prints a note and exits 0. Image pulls happen only on this path. `OEC_COMPOSE` selects the compose command (default `docker compose`).
   - **Observer:** a blocking probe client on the `oec-core` codec over rustls subscribes to `Oi4v2/#` and matches publications by the base64url AppId element, like a second application on the bus.
   - **Killed process:** the Will is tested with a child process (the test binary re-executes itself into an ignored test `child_app` with the storage root in `OEC_INTEROP_CHILD_ROOT`) that is killed (SIGKILL / TerminateProcess), so the SDK can send neither Close nor DISCONNECT.
   - **Broker restart:** a persistent probe (Clean Session 0, QoS 1) keeps what is published while the broker restarts (`$OEC_COMPOSE restart mosquitto`); publications whose MessageId time precedes the restart (QoS 1 redeliveries) are skipped, so a stale Will cannot satisfy the test.
   - **Covered:** credentials (Birth → Health NORMAL_0/100 → Close NORMAL_0/0 → DISCONNECT, no Will afterwards); client certificate with encrypted key; Will FAILURE_1/0 of a killed process; reconnect after a broker restart with a new Birth and a new Will; refused logins (wrong password, unknown user, credentials on the certificate listener, certificate of a foreign CA) and a wrong CA never get `Online`; TLS 1.2 and 1.3 on both listeners; payload = `MaxPacketSize` delivered with a 1000-character topic, payload + 1 dropped without losing the connection; the explicit bootstrap (`Bootstrap::Explicit`).
   - It **replaces the smoke test** of §10.9 (`OEC_SMOKE_*`, removed from `tests/runtime.rs`).
7. **Dependencies:** none new. The Rust tests use crates already in the tree (rustls, rustls-pki-types, serde_json; dev: rcgen); the generator uses the Go standard library only. ADR-0023 is unchanged.
8. **Platforms:** the generator and the tests are portable; `interop.sh` is POSIX sh (Linux, macOS; on Windows Git Bash/WSL or the manual steps of the README). Podman works through `OEC_COMPOSE`; `podman-compose` 1.x lacks `up --wait`.
9. **Cycle cut:** C-051 delivers increment 9 as one cycle (not split). Next is the P5 interleave (D7 `internal/mqttwire`, capture, `monitor --replay`, then `monitor` against this broker), then increments 6–8 and 10.
