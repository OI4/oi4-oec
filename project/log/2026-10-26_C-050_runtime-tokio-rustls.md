# C-050 – P4 increment 5b: runtime on tokio + rustls (`App::run`)

- **Date:** 2026-10-26
- **Phase:** P4 – SDK core implementation (D2), fifth cycle (cycle cut: ROADMAP P4)
- **Baseline:** guideline `734c560` (register `fce5cf5`, branch head, only `findings/`), unchanged; schema `2.0.1-dev` @ `734c560`
- **Start:** workspace after C-049 (`77406df`), new agent session (G-3.10)

## Intent

STATUS "Next step" 1 and ROADMAP P4 (increment 5, row C-050) asked for increment 5b, the runtime in the crate `oec`:
1. a **transport** on tokio `TcpStream` + tokio-rustls (ADR-0023): TLS 1.2/1.3 only (REQ-MQTT-005), trust only from the CA of the storage or the `Broker`, client certificate before credentials (REQ-MQTT-010), minimal features, `cargo deny` without exceptions or with a logged one;
2. the **runtime on the sans-IO core**: drive `oec_core::mqtt::Session` and `oec_core::lifecycle::Lifecycle` (ADR-0026 §9), map `ConnectionSettings` → `SessionConfig`, Birth then Health after CONNACK, Close before DISCONNECT, heartbeat through `Lifecycle::poll`;
3. the **API from the sketch** (ADR-0020 §1/§2/§5): `App::run` (own `current_thread` runtime, signals → Close + DISCONNECT), `App::run_async` (`Send` future, Q-021), `App::handle`/`AppHandle`, `ConnectionState`, `on_connection`, reconnect with backoff and a new Will per CONNECT (REQ-LIFE-016);
4. decisions on the open points of C-048/C-049: keys with `passphrase`, an explicit `Broker` without `ca_pem`, Birth first with Session Present;
5. **tests** against an in-process TLS test broker built on the `oec-core` codec, optionally a smoke test against a local broker that never breaks the gate; no outside network access in the gate;
6. a re-check of the P4 cycle cut.

Baseline check at the start:
- guideline head `fce5cf5` (only `findings/`) over text `734c560`;
- SF-001 … 104 resolved, none open;
- 1399 inventory items, 440 REQs, 158 `schema`/`d7-validate` REQs with 0 gaps, no drafts;
- entry schemas 29 of 29, 245 payload vectors, 18 `broker.json` vectors;
- `oec/check.sh` green.

The guideline did not change in this cycle, so no re-baseline was needed. No spec, schema or vector file changed, so `spectool render`/`coverage` had nothing new to process (both run in the gate).

## Cycle cut (re-checked, agent decision G-3.4 b)

**C-050: no split.** All six parts are delivered and green:
- product code in `oec/src`: `runtime.rs` 441 lines, `transport.rs` about 255, `app.rs` +212 (API, `AppHandle`, `ConnectionState`), `lib.rs` +34 (all with docs);
- `oec/tests/runtime.rs` 817 lines (test broker, PKI, 12 tests and the smoke test); small adaptations in `tests/app.rs` and `tests/common`.

It is one review unit: transport, runtime and API are one data path (settings → TLS → session → lifecycle → state), and the tests exercise them only together. A split (e.g. transport alone) would have produced a cycle without a testable result.

**Plan unchanged:** next is increment 9 (dev environment: compose, Mosquitto with TLS, dev PKI, sample `/etc/oi4`) with the interop tests of increment 5, then the P5 interleave, increments 6–8 and 10. Increment 9 is small in code but new in kind (containers, PKI scripts, interop against a real broker), so it stays a cycle of its own.

## Work

### 1. Transport (`oec/src/transport.rs`)

- `client_config`: rustls `ClientConfig` with the selected crypto provider, protocol versions TLS 1.3 and 1.2 only (REQ-MQTT-005), root store **only** from the CA file (all certificates in it; none usable → `Error::Bootstrap`, REQ-MQTT-008), client authentication with the certificate chain and key for `Auth::ClientCertificate` (REQ-MQTT-009/010), no client authentication for `Auth::Credentials`.
- `private_key`: PKCS#8, PKCS#1 or SEC1 through rustls-pki-types; an `ENCRYPTED PRIVATE KEY` is decrypted with the passphrase through pkcs8 (PBES2). The PEM is decoded to DER first, because the PEM reader of der 0.8 cannot read the PBES2 parameters. A legacy OpenSSL `Proc-Type: 4,ENCRYPTED` key is refused with a hint to convert it to PKCS#8; an encrypted key without passphrase is an error (REQ-CNT-008/009).
- `server_name` (DNS name or IP address of the configured host), `connect` (TCP + TLS handshake under one timeout), small `poll_fn` helpers `read`/`write_all`/`shutdown`, and `race`/`Either` (no `tokio::select!`, so tokio needs no `macros` feature).

### 2. Runtime (`oec/src/runtime.rs`)

- `run(app)`: loads the TLS configuration (fatal on error), builds the `Lifecycle` (MAM writer 10, Health writer 11) and the `Session`, then loops: connect → `drive` → backoff → reload TLS → connect. Shutdown is observed at every await.
- `drive`: CONNECT with `Lifecycle::will(now)`; on `Event::Connected` the backoff is reset and `Lifecycle::on_connected` queues **Birth, then Health** before the state becomes `Online`; while connected the deadline is the earlier of the session timeout and `Lifecycle::poll_timeout`, and `Lifecycle::poll` gives the heartbeat; writes are bounded by the keep-alive interval; EOF, I/O errors and timeouts end in `connection_lost`/`handle_timeout`, so no DISCONNECT is sent and the broker publishes the Will.
- `close`: Close (if connected), DISCONNECT, TLS `close_notify`, all within 5 s.
- `session_config`: the mapping of ADR-0026 §10.3.
- `Backoff`: 1 s doubling to 60 s, randomized to 50 … 100 % (xorshift seeded from ClientId and clock), reset on CONNACK; unit-tested.
- `signals`: SIGTERM/SIGINT on Unix, Ctrl-C/Close/Shutdown on Windows, Ctrl-C elsewhere → `AppHandle::shutdown`.

### 3. API in `oec` (`app.rs`, `lib.rs`)

- `App::run`, `App::run_async`, `App::handle`, `Deref<Target = AppHandle>`, `AppHandle::{state, shutdown}` (+ `Debug`), `ConnectionState`, `AppBuilder::on_connection` moved from the sketch; prelude exports `AppHandle`.
- `Broker::ca_pem` is required: `build()` fails with `LevelNotMet` citing REQ-CMP-002 and REQ-MQTT-008; `ConnectionSettings::ca()` returns `&Path`.
- Crate docs: state C-050, the cargo features, a `run()` example; `Error::Bootstrap` documents the TLS material.

### 4. Sketch, README, gate

- Sketch: `Broker::ca_pem` "required, no platform trust store"; `run_async` "installs no signal handlers"; `Asset::set_health_details` "equals" the most severe detail (REQ-RES-HEALTH-002, ADR-0020 amendment C-049). `set_health`/`set_health_details` themselves stay in the sketch until `Asset` moves (increment 6).
- `oec/sdk/rust/README.md`: runtime row, dependencies, crypto features, runtime tests and the smoke test.
- `oec/sdk/check.sh`: an extra `cargo clippy -p oec --all-targets --no-default-features --features=crypto-ring -D warnings`, so the ring path stays compiled.

### 5. Tests (`oec/tests/runtime.rs`)

- **Test PKI:** rcgen generates a CA, a server certificate for `localhost`/`127.0.0.1`, a client certificate and an encrypted PKCS#8 copy of a key (PBKDF2-SHA256, AES-256-CBC) per run; files under `CARGO_TARGET_TMPDIR`; nothing committed.
- **Test broker:** blocking std + rustls server on `127.0.0.1:0`, one thread per connection, decodes with the `oec-core` codec, answers CONNECT/PINGREQ/SUBSCRIBE as configured (`Behavior`: refused first CONNACK, no PINGRESP, mutual TLS, TLS 1.2 only) and reports `Seen::{Tls, Packet, Closed}` over a channel; `drop_connection` cuts the TCP stream to simulate a lost link.
- **Tests (all green, about 3 s, stable over repeated runs, with aws-lc-rs and with ring):**
  - `lifecycle_on_one_connection` – CONNECT (ClientId, Clean Session, Keep Alive, user/password, Will = Health FAILURE_1/0 on the Health topic), then Birth (MAM, writer 10, no Retain) as the first PUBLISH, Health NORMAL_0/100 (writer 11) second, then on shutdown Close (NORMAL_0/0) and DISCONNECT; states `Connecting → Online → Closed`;
  - `reconnect_after_broker_drop` – no DISCONNECT on loss, new CONNECT with a new Will, Birth and Health again; states include `Reconnecting`;
  - `keep_alive_timeout_closes_without_disconnect` – no PINGRESP → connection closed without DISCONNECT, reconnect;
  - `refused_connect_is_retried` – a refused CONNACK is retried, the second attempt gets `Online`;
  - `untrusted_broker_is_never_online` – a server certificate from another CA: TLS fails, never `Online`;
  - `client_certificate_over_tls12` – TLS 1.2-only broker with mutual TLS: certificate presented, no user name in CONNECT;
  - `encrypted_private_key_with_passphrase` – the encrypted key works with the passphrase;
  - `unusable_tls_material_fails_run` – through the container storage: a wrong passphrase, an encrypted key without passphrase, a legacy OpenSSL-encrypted key, a CA file without certificate: `run` returns `Error::Bootstrap` naming the cause;
  - `shutdown_before_run`, `run_async_on_a_spawned_task`, `unreachable_broker_keeps_connecting`, `handle_is_clone_send_sync`;
  - `smoke_against_a_local_broker` – only with `OEC_SMOKE_BROKER`, `OEC_SMOKE_CA`, `OEC_SMOKE_USER`, `OEC_SMOKE_PASSWORD`; otherwise it returns at once.
- **Manual smoke run (not in the gate):** `eclipse-mosquitto:2.0.18` in Docker with a TLS listener (port 18883, password file). The broker log showed CONNECT (ClientId, keep alive 5 s, user), the Birth on the MAM topic, the Health, the Close Health and DISCONNECT, in this order. Container and temporary files were removed. The real interop tests come with increment 9.
- **Encrypted keys from OpenSSL 3** (`openssl pkcs8 -topk8 -v2 aes-256-cbc` and `-scrypt`) were decrypted by the transport in a manual check; temporary files removed.

### 6. Measurements

- Tree of `oec` (unique crates, normal edges): 73 default (was 22), 122 with `strict-validation` (was 74), 73 with `crypto-ring`; `oec-core` unchanged (18); `Cargo.lock` 191 packages (was 106).
- `cargo deny check`: advisories, bans, licenses, sources ok; no exception, no `ignore`, `deny.toml` unchanged. Warnings: duplicates of getrandom, r-efi, syn, windows-sys only.

## Agent decisions (G-3.4 b, for information: REVIEW A-32 … A-41)

Recorded in ADR-0020 amendment C-050, ADR-0023 amendment C-050 and ADR-0026 §10 (`6cc0e52`).

1. **A-32 TLS transport.** TLS 1.3/1.2 only; trust only from the CA file; client certificate wins and then no MQTT credentials; TLS material loaded at `run` start (fatal) and reloaded before each reconnect (keep the last good one).
2. **A-33 Keys with passphrase (REQ-CNT-008/009).** Supported: PKCS#8 PBES2 (PBKDF2 with HMAC-SHA-2 or scrypt; AES) through the RustCrypto crate pkcs8. Refused with a clear `Error::Bootstrap`: legacy OpenSSL encryption, PBES1, PBKDF2-SHA1. RIG/MG do not name the key format; PKCS#8 is what current OpenSSL writes. No spec finding: the guideline leaves the format to the implementation, and a refusal with a hint is a documented limitation.
3. **A-34 Explicit `Broker` needs a CA.** No CA → `build()` fails with `LevelNotMet` (REQ-CMP-002, REQ-MQTT-008). No OS trust store, also not as a fallback (ADR-0023: trust from the storage only); an opt-in may come later as an addition. This changes the semantics of a sketch item (it accepted a `Broker` without CA), so it is an ADR-0020 amendment (C-050 C.1).
4. **A-35 Birth first.** Clean Session is always true and the core session refuses Session Present with it, so no resent message can precede the Birth (REQ-LIFE-015); no extra code. The Birth/Health order is asserted in the runtime tests.
5. **A-36 Reconnect and timeouts.** Backoff 1 s → 60 s, ×2, jitter 50 … 100 %, reset on CONNACK; the runtime never gives up; a refused CONNACK is logged as an error and retried (credentials can be fixed by the platform). Connect (TCP + TLS) and write timeouts = keep alive; close timeout 5 s.
6. **A-37 `run` vs. `run_async`.** `run`: own `current_thread` runtime plus a signal task (SIGINT/SIGTERM; Windows Ctrl-C/Close/Shutdown). `run_async`: the same loop without signal handlers; the application calls `AppHandle::shutdown`. This refines "same as `run`" (ADR-0020 amendment C-050 C.2).
7. **A-38 State and handle semantics.** `Closed` before and after `run`; `Connecting` until the first CONNACK, then `Online`/`Reconnecting`; `on_connection` called on every change, synchronously on the runtime thread; `shutdown` idempotent, before `run` it makes `run` return at once. Writer ids: MAM 10, Health 11. Received PUBLISH packets are dropped with a debug log until increments 6–8.
8. **A-39 Test broker and smoke test.** In-process std + rustls broker on the `oec-core` codec with a per-run rcgen PKI (rcgen as the only new dev dependency); smoke test only with `OEC_SMOKE_*`; no outside network access in the gate.
9. **A-40 Dependencies and crypto features.** tokio/rustls/tokio-rustls/rustls-pki-types with the smallest features of ADR-0023 §1; pkcs8 as a new crate within the rules; features `crypto-aws-lc-rs` (default, post-quantum key exchange preferred) and `crypto-ring` (ring wins if both; gate runs clippy with ring). Tree 73/122; deny without exceptions.
10. **A-41 Cycle cut and leftovers.** C-050 not split; next increment 9. `set_health`/`set_health_details` stay in the sketch (increment 6, with `Asset`); only the doc says "equals". ADR-0026 §9.6 wording of the MessageId counter fixed to the REQ-MSG-007 form `<ms><n>-<PublisherId>` (the code always did this; the C-049 log and A-29 said `-1`, `-2`).

## Changes to reviewed items

None in the sense of G-3.9 (no REQ, vector or matrix entry changed). Agent amendments within accepted decisions:
- ADR-0020 (PO-accepted): amendment C-050 – the API moved, plus two semantic refinements of sketch items: an explicit `Broker` needs a CA (A-34), `run_async` installs no signal handlers (A-37). Both follow the REQs/ADR-0023 and are listed in REVIEW for information; the PO can overrule.
- ADR-0023 (PO-accepted): amendment C-050 – the crates of §1 with their features, the crate pkcs8 within the rules, rcgen as a dev dependency.
- ADR-0026 (agent-accepted): §10; §9.6 wording fix (no semantic change; the code and its docs were right).
- Schema unchanged (`2.0.1-dev`), no CHANGELOG entry.

## Spec findings

None. RIG 4.1 (TLS, CA, client certificate or credentials), MG 6.3 (Birth/Close/Will) and MG 9.1.2 (Health) were sufficient. The format of an encrypted private key is not defined by the guideline (REQ-CNT-008/009 only name a passphrase); supporting PKCS#8 PBES2 is an implementation choice, not a gap. Next free id stays SF-105.

## Risks and open technical points

- **Windows signals:** the Windows path (`ctrl_c`/`ctrl_close`/`ctrl_shutdown`) is not compiled locally (no Windows target installed). It uses the documented tokio API; the CI with Windows runners (ADR-0023 §2 "Cross builds") will compile it.
- **Signal handlers stay registered** after `run` returns (a tokio property): SIGINT/SIGTERM then no longer terminate the process by default. Documented in ADR-0026 §10.7; typical applications end after `run`.
- **Interop:** the runtime is tested against the in-process broker and once manually against Mosquitto 2.0.18. Systematic interop (Mosquitto with the dev PKI, client certificates, broker restarts) comes with increment 9.
- **Heartbeat timing** (60 s) is covered by the core tests only; the runtime tests do not wait 60 s. The runtime uses the deadline of `Lifecycle::poll_timeout`, which the core tests pin.
- **Build of aws-lc-sys** on Windows (NASM) and ARMv7 (use `crypto-ring`) is not verified yet (CI).
- **No offline queue:** publications exist only while connected. For the lifecycle that is correct (Birth and Health are sent on every connect). Increment 6 publishes state (ADR-0020 §7), so a reconnect republishes the current state rather than a backlog.

## Commits

Workspace (never pushed):
- `d28987e` code: transport, runtime, API, tests, dependencies, gate (ring clippy);
- `ac065ee` README and sketch docs;
- `6cc0e52` ADR-0020/0023 amendments C-050, ADR-0026 §10 and §9.6 wording fix, one doc line in `transport.rs`;
- this log, STATUS, ROADMAP; then REVIEW.md/REVIEW_DE.md.

Guideline: none.

## Checks (final)

`oec/check.sh`: all checks passed. Included in it:
- spectool `lint`, `render -check`, `coverage -check -gate` (0 gaps), `topic -check`, `inventory -check -rev 734c560` (1399 items);
- `schema/check.sh`, release freeze, `spectool crosscheck`;
- `sdk/check.sh`:
  - the workspace (`oec-core`, `oec`) in both feature sets with fmt, clippy pedantic, tests (incl. proptests and the runtime tests over loopback TLS) and doc;
  - clippy of `oec` with `crypto-ring`;
  - cargo-deny;
  - the sketches and the spikes;
- the Go tests of `oec/spec/tools` and `oec/conformance`.

## For the PO (review)

Nothing to decide and no test data to review: no D-x or R-x. For information (`REVIEW.md` A-32 … A-41), worth a look because they refine the accepted sketch contract or the dependency set:
- A-34: an explicit `Broker` without CA is refused (no OS trust store);
- A-37: `run_async` installs no signal handlers;
- A-33: keys with passphrase only as PKCS#8 PBES2;
- A-40: the dependency tree of `oec` grows from 22 to 73 crates (tokio, rustls, aws-lc-rs, RustCrypto for pkcs8), `crypto-ring` as the alternative provider.

## Next

Increment 9 (ROADMAP P4): the dev environment:
- docker compose with Mosquitto (TLS listener, password file and client certificates), a generated dev PKI (script, nothing secret committed), a sample `/etc/oi4` layout (`broker.json`, `broker_ca.pem`, credentials/keys) for `Bootstrap::ContainerStorage { root }`;
- interop tests of increment 5 against it (connect with credentials and with a client certificate, Will on a killed client, broker restart → reconnect + Birth), opt-in so the gate stays without containers, or a separate check script;
- the smoke test of C-050 becomes part of that.

Next free ids (unchanged): SF-105, REQ-CMP-048, REQ-RES-PUBLIST-028, REQ-TOP-029, V-PAY-246, V-TOP-138 (valid) / V-TOP-255 (invalid), V-BRK-019, P-129, ADR-0027, Q-031, A-42.

**Agent for increment 9 (G-3.10): new agent.**
- The context of this session was compacted during C-050.
- Increment 9 is a new topic in other files: containers, PKI scripts, compose, interop tests; little Rust.
- The state is handover-ready (G-3.7): this log, ADR-0026 §10 (the runtime contract), STATUS "Next step" 1, ROADMAP P4 and `REVIEW.md`.

Start prompt: `prompt.md` (German, not committed).
