# C-049 – P4 increment 5a: MQTT 3.1.1 codec, session and lifecycle (sans-IO)

- **Date:** 2026-10-26
- **Phase:** P4 – SDK core implementation (D2), fourth cycle (cycle cut: ROADMAP P4)
- **Baseline:** guideline `734c560` (register `fce5cf5`, branch head, only `findings/`), unchanged; schema `2.0.1-dev` @ `734c560`
- **Start:** workspace after C-048 (`31d4783`), new agent session (G-3.10)

## Intent

STATUS "Next step" 1 and ROADMAP P4 (increment 5, row C-049) asked for increment 5a, everything sans-IO in `oec-core`:
1. an **MQTT 3.1.1 codec** (ADR-0023 Q-026 option a, from the spike `mqtt-codec`): CONNECT (Will, User Name/Password, Clean Session, Keep Alive), CONNACK, PUBLISH QoS 0/1 without Retain, PUBACK, SUBSCRIBE/SUBACK, UNSUBSCRIBE/UNSUBACK, PINGREQ/PINGRESP, DISCONNECT; limits as structured errors; every decision traced to a REQ or ADR section;
2. the **session as a state machine** (bytes in, bytes/events out, time as a parameter): connect, keep alive and ping timeout, packet ids and QoS 1 acknowledgements, subscriptions, a new Will per CONNECT (REQ-LIFE-016), a clean close;
3. **lifecycle** (MG 6, REQ-LIFE-*, REQ-CMP-001 … 007, REQ-TOP-017) and **Health** (MG 9.1.2) as pure functions/state machines on the message model of increment 3: Birth, Will, Close, the Health heartbeat with `Details[]`; the payloads pass the embedded schema (test with `strict-validation`);
4. **fuzzing/property tests** for codec and session with one dev dependency under ADR-0023 and `cargo deny`;
5. no network, TLS or runtime dependency (those come in C-050);
6. a re-check of the P4 cycle cut.

Baseline check at the start:
- guideline head `fce5cf5` (only `findings/`) over text `734c560`;
- SF-001 … 104 resolved, none open;
- 1399 inventory items, 440 REQs, 158 `schema`/`d7-validate` REQs with 0 gaps, no drafts;
- entry schemas 29 of 29, 245 payload vectors, 18 `broker.json` vectors;
- `oec/check.sh` green.

The guideline did not change in this cycle, so no re-baseline was needed. No spec, schema or vector file changed, so `spectool render`/`coverage` had nothing new to process (both run in the gate).

## Cycle cut (re-checked, agent decision G-3.4 b)

**C-049: no split.** All six parts are delivered and green:
- about 2,700 lines of product code in `oec-core/src`, with docs: `mqtt/` about 2,170 lines, `lifecycle.rs` 480, `MessageIds` about 50;
- about 2,030 lines of integration tests.

It is one review unit: the lifecycle produces the Will that the session sends with each CONNECT, and the session tests and the proptests share the packet model.

**Plan unchanged:**
- **C-050 – 5b (runtime, `oec`):** transport tokio + rustls; `App::run`/`run_async`/`handle`, `AppHandle`, `ConnectionState`, `on_connection`, signals, reconnect/backoff; the Health heartbeat runs on the `Lifecycle` of 5a.
- Then increment 9 (dev broker, interop tests), the P5 interleave, increments 6–8 and 10.

5b is still a cycle of its own: it brings the largest dependency step of P4 and the first network code.

## Work

### Codec: `oec-core::mqtt` (`3daebf6`)

Written new after review of the spike `mqtt-codec` (366 lines, 8 tests). It was not copied: the spike has no streaming decoder, no limits, no rule attribution and no SUBSCRIBE/UNSUBSCRIBE refusals.

| Part | Content | REQ / rule |
|---|---|---|
| `packet.rs` | `Packet` with `Connect` (+ `LastWill`, no Retain field), `ConnAck`, `Publish` (`Delivery::AtMostOnce` / `AtLeastOnce { id, dup }`), `PubAck`, `Subscribe`/`SubAck`, `Unsubscribe`/`UnsubAck`, `PingReq`/`PingResp`, `Disconnect`; `PacketId` = `NonZeroU16`; the password is redacted in `Debug` | REQ-MQTT-001, REQ-MQTT-017/018, G-4.5 |
| `codec.rs` `encode`/`decode`/`encoded_len` | symmetric and canonical: encode Ok ⇔ decode Ok; accepted bytes re-encode identically; nothing is appended on error | MQTT 2.2, 3.x |
| `codec.rs` `Decoder` | incremental decoder for a stream; the limit is checked at the fixed header (before the body arrives); after an error it stays poisoned | MQTT 2.2.3, RIG 4.1.1 |
| `CodecError` | `Incomplete`, `TooLarge`, `Malformed`, `Unsupported`, `ProtocolViolation`, …; `rule()` names `MQTT-x.y.z-n`, an MQTT section or the REQ | ADR-0026 §9.2 |
| `check_topic_name`/`check_topic_filter` | no wildcards in names; `#` last and alone in its level; `+` alone in its level; not empty; UTF-8 without U+0000 | MQTT 4.7, MQTT-1.5.3-1/2 |

Refused (each with its rule):
- QoS 2 and PUBREC/PUBREL/PUBCOMP (REQ-MQTT-017);
- a Will with Retain (REQ-MQTT-018);
- a password without user name (MQTT-3.1.2-22);
- Session Present with a refusal code (MQTT-3.2.2-4);
- reserved packet types, flags and bits;
- a non-minimal Remaining Length, trailing bytes, packet id 0;
- empty SUBSCRIBE/UNSUBSCRIBE;
- a Remaining Length over 268435455 or over the decoder limit.

Retain on a *received* PUBLISH is decoded (the broker may deliver retained messages of others).

### Session: `oec-core::mqtt::Session` (`3daebf6`)

- **I/O-free API:** `connect(will, now)`, `handle_input(bytes, now)`, `handle_timeout(now)`/`poll_timeout()`, `publish`, `subscribe`, `unsubscribe`, `disconnect`, `connection_lost`, `take_transmit`, `poll_event`. Time is a monotonic `Duration`.
- **`SessionConfig`:**
  - ClientId;
  - `clean_session` (default true, REQ-MQTT-015);
  - `keep_alive: NonZeroU16` (default 60 s, REQ-MQTT-014);
  - `credentials` (REQ-MQTT-010);
  - `max_packet_size` (default and minimum 262144, RIG 4.1.1);
  - `max_in_flight` (default 64).
  - `validate()`: an empty ClientId needs Clean Session (REQ-MQTT-016).
- **Events:**
  - `Connected { session_present }`;
  - `Message(Publish)` (an incoming QoS 1 is acknowledged at once);
  - `Published(id)`, `Subscribed { packet_id, results, restored }`, `Unsubscribed { packet_id, filters }`, `Dropped(ids)`;
  - `Closed(CloseReason)`, with `CloseReason` one of `Graceful`, `Refused(code)`, `ConnectTimeout`, `KeepAliveTimeout`, `Protocol(..)` or `TransportClosed`.
- Semantics in ADR-0026 §9.3:
  - timers: CONNACK, PING, PINGRESP;
  - publish limits: `PayloadTooLarge` per REQ-PAG-005, the in-flight limit, packet ids that wrap;
  - restore or resend after a reconnect;
  - close without DISCONNECT on errors, so the broker publishes the Will (REQ-LIFE-008).

### Lifecycle: `oec-core::lifecycle` (`3daebf6`)

| Item | Content | REQ |
|---|---|---|
| `Lifecycle::new(service_type, app_id, mam, mam_writer, health_writer)` | the application with its MAM (ProductInstanceUri = AppId) and two different DataSetWriterIds for MAM and Health | REQ-LIFE-003, REQ-MSG-020 |
| `will(unix_ms)` | `LastWill` on `Pub/Health/<AppId>`, Health FAILURE_1/0, QoS 0, a new MessageId per call | REQ-LIFE-008 … 010, REQ-LIFE-016 |
| `birth`, `close` | MAM on `Pub/MAM/<AppId>`; Health NORMAL_0/0 | REQ-LIFE-001 … 003, REQ-LIFE-005 … 007 |
| `on_connected(now, unix_ms)` | Birth first, then the Health | REQ-LIFE-001, REQ-LIFE-015, REQ-TOP-017 |
| `set_health`, `poll`/`poll_timeout` | Health on change (incl. `Details`) and every 60 s; Birth again before the first non-failure Health after FAILURE_1/0 | REQ-TOP-017, REQ-RES-HEALTH-004, REQ-LIFE-012/017 |
| `check_health` | HealthScore ≤ 100; Health equals the most severe `Details` entry | REQ-RES-HEALTH-001/002 |
| `asset_health` + `HealthBeat` | Health of another asset on `Pub/Health/<asset>`; the AppId is refused | REQ-CMP-007 |
| `message::MessageIds` | `<unix-ms>-<PublisherId>[-n]`, unique and monotonic per publisher | REQ-MSG-007/008 |

The messages have the shape of the MG 6.3 examples and of the vectors V-PAY-023/024/025 (A-27).

### Dependencies (`3daebf6`)

| Crate | Version | Features | Role |
|---|---|---|---|
| proptest | 1.12.0 (dev only) | `default-features = false`, `std` | property tests of codec and session |

- The dev tree adds 10 crates to `Cargo.lock`: chacha20, core_detect, cpufeatures 0.3, getrandom 0.4, r-efi 6, rand 0.10, rand_core 0.10, rand_xorshift, unarray and proptest itself. That makes 33 crates in `oec-core` with dev edges.
- The default tree of `oec-core` is unchanged at 20 crates (81 with `strict-validation`).
- Licenses:
  - proptest and the rand family are MIT OR Apache-2.0;
  - r-efi is MIT OR Apache-2.0 OR LGPL-2.1-or-later, used under MIT/Apache-2.0 and only on UEFI targets.
- `cargo deny check`: ok, **no exception and no `ignore` entry**. The only warning is the pre-existing duplicate `syn`.
- ADR-0023 amendment C-049 records this.

### Docs, gate comment (`6dff6a4`)

- `oec/sdk/rust/README.md`: the `oec-core` row lists `mqtt`/`lifecycle`; the dependency rule is proptest, no cargo-fuzz; the lifecycle vector test.
- `oec/src/{app,lib}.rs`: "increment 5" → "increment 5b".
- `oec/sdk/check.sh` header: lifecycle vectors and proptests; the spike is superseded by `oec-core::mqtt`.
- The sketch is unchanged. `run`/`run_async`/`AppHandle`/`ConnectionState`/`on_connection` stay there and compile.

### Tests

| Test | Build | What it checks |
|---|---|---|
| `tests/mqtt_codec.rs` (18) | both | byte-exact encodings (hand-encoded per MQTT 3.1.1) of every packet incl. Will + credentials and an empty password; Remaining Length boundaries 127/128, 16383/16384 and 2097151/2097152, and the 4-byte maximum; the limit at the header; incomplete input; refusals of fixed header, PUBLISH, CONNECT, acks, SUBSCRIBE with their `rule()`; valid topic filters; the encoder refuses what the decoder refuses; streaming and poisoning; error display |
| `tests/session.rs` (17) | both | CONNECT with the MG 6.3.1 defaults; config validation; every CONNECT carries the Will it is given; graceful close (Close, then DISCONNECT); publish without Retain and with limits; QoS 1 acknowledgement; incoming QoS 1 acked at once; in-flight limit and id reuse; ping and keep-alive timeout; traffic defers the ping; connect timeout and refusal; protocol violations close without DISCONNECT; subscriptions held, sent and restored; pending requests sent again; SUBACK with the wrong count; resend with DUP (session present); `Dropped` (no session) |
| `tests/lifecycle.rs` (10, strict 11) | both | Will/Birth/Close equal V-PAY-025/023/024 as JSON values, topic and role included (MessageId too, with the vector's timestamps); topics and PublisherId of the application; a new MessageId per Will; Birth before Health on connect; a change is published at once and the heartbeat after 60 s; Health checks (score, equals-aggregation, NORMAL_0/0); Birth after recovery from FAILURE_1/0; asset Health; `HealthBeat`; MessageIds unique and monotonic; strict: every generated payload (Birth, Will, Close, Health with `Details`, asset Health) passes the strict schema |
| `tests/proptest_mqtt.rs` (6) | both | 1024 cases each: encode ⇔ decode round trip and every proper prefix is `Incomplete`; valid packets always encode; arbitrary bytes never panic and decode canonically; mutated packets decode canonically or fail; any chunking decodes the same; random session runs (operations, broker packets, time) never panic and keep their invariants (`connect` only from `Disconnected`, in-flight ≤ limit, no Retain, Clean Session as configured, no server packet sent, `Closed` ⇒ `Disconnected`, the next timeout within two keep-alive periods) |

Results:
- default build: lib 27, codec 18, session 17, lifecycle 10, proptest 6 and the earlier suites;
- strict build: lifecycle 11, plus the earlier strict suites.

All pass; the proptests take about 0.7 s. While writing the lifecycle test, DataSetWriterIds 7/9 were refused by the model (reserved, REQ-MSG-020: application ids start at 10), so the test uses 17/19. The model rule worked as intended.

## Agent decisions (G-3.4 b, for information: REVIEW A-25 … A-31)

Recorded in ADR-0026 amendment C-049 §9, ADR-0023 amendment C-049 and ADR-0020 amendment C-049 (`e6bf7bf`).

1. **A-25 Codec scope and canonical rules.**
   - Only what a client needs, without QoS 2.
   - Symmetric and canonical: the decoder refuses what the encoder would never write, e.g. a non-minimal Remaining Length or trailing bytes.
   - Every refusal names its rule. The receiver is stricter than the "be lenient" habit, because the SDK talks to a broker that must speak correct MQTT 3.1.1. Every refusal closes the connection (MQTT 4.8).
   - The decoder limit is MaxPacketSize plus the largest header (65539), so a PUBLISH whose *payload* fits is never refused for its topic.
2. **A-26 Session model.**
   - Sans-IO with monotonic `Duration` time.
   - Keep alive: PINGREQ one keep-alive period after the older of the last sent and the last received packet. The connect and PINGRESP timeouts are one keep-alive period each.
   - Errors, timeouts and connection loss close **without** DISCONNECT, so the broker publishes the Will.
   - Subscriptions are owned by the session and restored after a reconnect without session. Unacknowledged QoS 1 messages are reported as `Dropped` (the runtime decides). With Session Present they are resent with DUP.
   - `publish` only while connected (no offline queue in the core; that is a runtime policy for 5b).
3. **A-27 Lifecycle message shape.**
   - Will, Birth, Close and the self-Health follow the MG 6.3 examples and the vectors V-PAY-023/024/025: no DataSetMessage `MessageType` (REQ-MSG-029 assumes `ua-keyframe`), no Timestamp, no SequenceNumber, empty WriterGroupName.
   - This is an exception to ADR-0026 §7.3 ("always sends `MessageType`") for these messages only. Both forms pass the schema; the SDK sends what the guideline shows.
4. **A-28 Health rules.**
   - Health **equals** the most severe `Details` entry (REQ-RES-HEALTH-002). The sketch's "at least" is read as "equals" (ADR-0020 amendment C-049; the sketch doc is fixed in C-050).
   - `set_health` refuses NORMAL_0/0, which means Close (REQ-LIFE-014).
   - Initial Health NORMAL_0/100.
   - After FAILURE_1/0 about itself, the next non-failure Health is preceded by a Birth (REQ-LIFE-012/017).
   - The heartbeat restarts on every connect and on every change.
5. **A-29 MessageIds.** One generator per application. The first id of a millisecond has no counter, then `-1`, `-2`, …. A wall clock that steps back holds the last millisecond, so uniqueness wins over clock precision (REQ-MSG-007/008).
6. **A-30 proptest, no cargo-fuzz.**
   - proptest runs on stable in the normal gate.
   - cargo-fuzz needs nightly and libFuzzer, so its targets would not be in the gate; not set up. It can come later as an optional crate outside the workspace.
   - `cargo deny` stays without exceptions.
7. **A-31 `ConnectionSettings` → `SessionConfig` deferred, cycle cut.**
   - `oec-core` defines its own `SessionConfig` (the core must not depend on `oec`, ADR-0026 §2).
   - The mapping is written in C-050 with the runtime. It is mechanical except two points (see Risks).
   - C-049 not split; C-050 = 5b as planned.

## Changes to reviewed items

None:
- no REQ, vector, matrix entry or PO-accepted decision content changed;
- ADR-0020 and ADR-0023 (PO-accepted) got agent amendments within their decisions:
  - ADR-0020: no API change, one wording clarification that follows the REQ;
  - ADR-0023: the dev dependency chosen from the "cargo-fuzz or proptest" of its Consequences;
- ADR-0026 is agent-accepted and got §9;
- schema unchanged (`2.0.1-dev`), no CHANGELOG entry.

## Spec findings

None. MG 6.3 (Birth/Close/Will), MG 9.1.2 (Health), RIG 4.1 and the REQs were sufficient, and the generated Will/Birth/Close equal the role vectors. The absent DataSetMessage `MessageType` in the MG 6.3 examples is covered by REQ-MSG-029 (default `ua-keyframe`), so it is not a gap. Next free id stays SF-105.

## Risks and open technical points (for C-050)

- **Mapping `ConnectionSettings` → `SessionConfig`:**
  - `keep_alive: Duration` → `NonZeroU16` seconds (`build()` already ensures 1 … 65535);
  - `Auth::Credentials` → `Credentials`;
  - `Auth::ClientCertificate` → TLS only, no MQTT user name;
  - `publish_limit`/`max_packet_size` → `SessionConfig::max_packet_size`;
  - Clean Session stays true (REQ-MQTT-015).
- **Persistent session and Birth first:** with Clean Session false and Session Present, the session resends unacknowledged QoS 1 messages before the runtime can publish the Birth (REQ-LIFE-015). With the default Clean Session this cannot happen; 5b keeps Clean Session or holds the resend.
- **`set_health` while `Details` are set** (sketch `Asset`): to be decided in C-050 within "equals" (ADR-0020 amendment C-049).
- **Unchanged from C-048:** encrypted private keys with `passphrase` (REQ-CNT-008), and the CA of an explicit `Broker` without `ca_pem`. Both belong to 5b; nothing in C-049 changed them.
- **Interop:** the codec is tested byte-exact against the standard and by property tests, not yet against a real broker. Increment 9 (dev broker) adds that; 5b may already run a local smoke test.

## Commits

Workspace (never pushed):
- `3daebf6` `oec-core`: `mqtt` (packet, codec, session), `lifecycle`, `MessageIds`, tests, proptest dev dependency, `Cargo.lock`;
- `6dff6a4` READMEs, `oec` doc references (5 → 5b), gate comment;
- `e6bf7bf` ADR-0020/0023 amendments C-049, ADR-0026 §9;
- this log, STATUS, ROADMAP; then REVIEW.md/REVIEW_DE.md.

Guideline: none.

## Checks (final)

`oec/check.sh`: all checks passed. Included in it:
- spectool `lint`, `render -check`, `coverage -check -gate` (0 gaps), `topic -check`, `inventory -check -rev 734c560` (1399 items);
- `schema/check.sh`, release freeze, `spectool crosscheck`;
- `sdk/check.sh`:
  - the workspace (`oec-core`, `oec`) in both feature sets with fmt, clippy pedantic, tests (incl. proptests) and doc;
  - cargo-deny;
  - the sketches (REQ walker: every REQ id cited in the new files exists);
  - the spikes;
- the Go tests of `oec/spec/tools` and `oec/conformance`.

## For the PO (review)

Nothing to decide and no test data to review: no D-x or R-x. For information only (`REVIEW.md` A-25 … A-31):
- the agent decisions above, notably the lifecycle messages without `MessageType` (A-27);
- "equals" for Health and Details (A-28);
- proptest instead of cargo-fuzz (A-30).

## Next

C-050 – P4 increment 5b (ROADMAP P4), the runtime in `oec`:
- transport tokio + rustls (TLS 1.2/1.3, CA from the storage, client certificate or credentials);
- `App::run`/`run_async`/`handle`, `AppHandle`, `ConnectionState`, `on_connection`, signals, reconnect with backoff;
- the `Session` and `Lifecycle` of 5a, the mapping `ConnectionSettings` → `SessionConfig`;
- the open points: passphrase and the CA of an explicit `Broker`.

Next free ids (unchanged): SF-105, REQ-CMP-048, REQ-RES-PUBLIST-028, REQ-TOP-029, V-PAY-246, V-TOP-138 (valid) / V-TOP-255 (invalid), V-BRK-019, P-129, ADR-0027, Q-031.

**Agent for C-050 (G-3.10): new agent.**
- The context of this session was compacted during C-049.
- C-050 is a new topic in other files and with new dependencies: the crate `oec` (runtime, public API from the sketch), tokio/rustls under ADR-0023, network tests.
- The state is handover-ready (G-3.7): this log, ADR-0026 §9 (the contract of the sans-IO parts), STATUS "Next step" 1, ROADMAP P4 and `REVIEW.md`.

Start prompt: `prompt.md` (German, not committed).
