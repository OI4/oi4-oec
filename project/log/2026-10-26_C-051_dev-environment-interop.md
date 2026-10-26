# C-051 – P4 increment 9: dev environment (Mosquitto with TLS, dev PKI) and interop tests

- **Date:** 2026-10-26
- **Phase:** P4 – SDK core implementation (D2), sixth cycle (cycle cut: ROADMAP P4)
- **Baseline:** guideline `734c560` (register `fce5cf5` at the start, `a286511` at the end, both only `findings/`), unchanged; schema `2.0.1-dev` @ `734c560`
- **Start:** workspace after C-050 (`f228c82`), new agent session (G-3.10)

## Intent

STATUS "Next step" 1 and ROADMAP P4 (increment 9, row C-051) asked for:
1. a **dev environment `oec/dev/`** (ADR-0012):
   - compose with Mosquitto: TLS 1.2/1.3 only, a password file and client certificates (REQ-MQTT-009/010), the image pinned;
   - a dev-PKI script; nothing generated is committed;
   - a sample `/etc/oi4` layout that `Bootstrap::ContainerStorage { root }` reads and that matches the `broker.json` vectors;
   - a README;
2. **interop tests of increment 5** against that broker (exit criterion 3):
   - credentials and client certificate;
   - Birth → Health → Close → DISCONNECT;
   - the Will of a killed client;
   - broker restart → reconnect with a new Will and Birth;
   - refused login and wrong CA;
   - the tests are opt-in: the default gate needs no container engine and no outside network;
   - they absorb the C-050 smoke test;
3. findings from the comparison with a real broker (SF-105 ff.);
4. a re-check of the C-050 risks;
5. a re-check of the cycle cut.

Baseline check at the start:
- guideline head `fce5cf5` (only `findings/`) over text `734c560`;
- SF-001 … 104 resolved, none open;
- 1399 inventory items, 440 REQs, 158 `schema`/`d7-validate` REQs with 0 gaps, no drafts;
- entry schemas 29 of 29, 245 payload vectors, 18 `broker.json` vectors;
- `oec/check.sh` green.

The guideline text did not change. The only guideline commit is the register entry for SF-105 (`a286511`, only `findings/`), so no re-baseline was needed (ADR-0001 amendment C-018). No spec, schema or vector file changed.

## Cycle cut (re-checked, agent decision G-3.4 b)

**C-051: no split.** All parts are delivered and green:
- `oec/dev`:
  - `compose.yaml` 42 lines and `mosquitto/mosquitto.conf` 58;
  - the Go PKI generator: `pki/main.go` 36, `generate.go` 414, tests 396;
  - `interop.sh` 52 and the README 211;
- `oec/sdk/rust/oec/tests/interop.rs` 846 lines (probe client, child process, 8 tests + helper);
- the smoke test removed from `tests/runtime.rs`.

It is one review unit: the broker configuration, the PKI and the sample storages exist to be used by the interop tests, and the tests are the only proof that they fit together. A split would have left a broker without tests or tests without a broker.

**Plan:** next is the P5 interleave as planned:
- D7 `internal/mqttwire`, `internal/capture` and `monitor --replay`;
- then `monitor` against this dev broker (ADR-0025, DESIGN.md §11).

After it come increments 6–8 and 10. Details under "Next".

## Work

### 1. Broker (`oec/dev/compose.yaml`, `oec/dev/mosquitto/mosquitto.conf`)

- **Image:** `eclipse-mosquitto:2.1.2-alpine@sha256:860aeb5d…bb0f`.
  - 2.1.2 is the current stable release; C-050 had used 2.0.18 manually.
  - It is pinned by tag and by the multi-arch index digest, which covers amd64, arm64 and others.
  - Pulled only by `compose up` on the opt-in path.
- **Listeners**, both with the broker certificate of the dev CA:
  - `8883`: user name and password from the password file (REQ-MQTT-009);
  - `8884`: `require_certificate true` and `use_identity_as_username true`, so the CN is the user name (REQ-MQTT-010).
  - Both use `tls_version tlsv1.2`. In Mosquitto this is the minimum, so TLS 1.3 is negotiated when the client offers it (REQ-MQTT-005).
  - There is no plain listener and no anonymous access (`allow_anonymous false`, REQ-MQTT-006).
- **Packet size (see SF-105):**
  - `message_size_limit 262144` – payloads over the `MaxPacketSize` of the sample `broker.json` are dropped and the connection stays;
  - `max_packet_size 327688` = 262144 + 65544 (fixed header 5 + topic length 2 + topic 65535 + packet id 2).
  - So every payload up to `MaxPacketSize` is accepted with any topic.
- **Persistence:** on, in the named volume `data` (`oec-dev_data`), together with `queue_qos0_messages`. A persistent session (Clean Session 0) survives a restart; the restart test needs that for its probe.
- **Ports:** published on `127.0.0.1` only (`OEC_DEV_BIND` overrides).
- **Healthcheck:** a TCP check with `nc` (every 1 s during the start period, then every 60 s). Each check makes the broker log "unexpected eof", which is harmless and documented.
- **Entrypoint:**
  - copies the read-only mounted `generated/broker/*` into `/mosquitto/tls` (owner `mosquitto`, mode 0600);
  - so host file ownership and permissions do not matter (Docker, Podman, rootless). On SELinux hosts the bind mounts may need `:z` (README).
  - The broker runs as the `mosquitto` user of the image.

### 2. Dev PKI (`oec/dev/pki`, Go module `oec/dev`, standard library only)

`go run ./pki` writes `generated/` (git-ignored, marker `.oec-dev-pki`; `-force` only replaces a directory with that marker; `-san` adds names to the broker certificate):
- **CA:** ECDSA P-256, 3 years, path length 0.
- **Broker certificate:** serverAuth; SAN `localhost`, `mosquitto`, `127.0.0.1`, `::1`.
- **Client certificates** (clientAuth): `oec-dev-cert` (the sample container) and `oec-probe` (the observer of the tests).
- **Passwords and passphrase:** random. The Mosquitto password file uses the `$7$` format of `mosquitto_passwd` 2.1 (PBKDF2-SHA512, 1000 iterations, 64-byte salt). A vector captured from `mosquitto_passwd` pins the format.
- **Encrypted key:** the key of `oec-dev-cert` is written as PKCS#8 PBES2 (PBKDF2-HMAC-SHA256, 2048 iterations as the OpenSSL 3 default of `openssl pkcs8 -topk8 -v2 aes-256-cbc`, AES-256-CBC) with its passphrase. The SDK decrypts it (A-33). A test decrypts it again with the same algorithm.
- **Sample container storages:** two complete RIG 4.1 roots (item 3).
- **Tests (in the gate):**
  - `generate_test.go`: the chain verifies, the key usages and SANs are right, the password file format matches the vector, the encrypted key round-trips, the marker logic works, `broker.json` has exactly three keys;
  - `config_test.go` lints `mosquitto.conf` and `compose.yaml`: both listeners, the TLS floor, `require_certificate`, no anonymous access, the packet limits, the image pinned by digest, ports on loopback, persistence.

Why Go and not a shell script with `openssl`:
- `openssl` differs between LibreSSL (macOS), OpenSSL 1.1/3 and Windows, and PBES2 options and SAN syntax are not portable;
- `mosquitto_passwd` would need the image or a local Mosquitto;
- Go is already a project toolchain (ADR-0013), runs on Windows, macOS and Linux, and the standard library has everything needed (x509, ecdsa, pbkdf2 since Go 1.24, aes/cipher).
- No new dependency (`go.mod` without requirements).

### 3. Sample `/etc/oi4` layout (`generated/roots/<container>`)

| Path (below the root) | `oec-dev-app` (credentials) | `oec-dev-cert` (certificate) |
|---|---|---|
| `etc/hostname` | `oec-dev-app` | `oec-dev-cert` (REQ-CNT-019/021) |
| `etc/oi4/mqtt/broker.json` | `{"Address":"localhost","SecurePort":8883,"MaxPacketSize":262144}` | same with `8884` (REQ-CNT-003, shape of V-BRK-001) |
| `etc/oi4/mqtt/broker.pem` | broker certificate | same |
| `etc/oi4/certs/ca.pem` | dev CA (REQ-CNT-005) | same |
| `etc/oi4/certs/oec-dev-cert.pem` | – | client certificate (REQ-CNT-006/007) |
| `run/secrets/oi4/credentials` | `base64(user:password)` (RIG 4.1.3, REQ-CNT-008, GL-ADR-022) | – |
| `run/secrets/oi4/private_key.pem`, `passphrase` | – | encrypted key + base64 passphrase (RIG 4.1.3, REQ-CNT-008) |
| `etc/oi4/app/` | empty (REQ-CNT-010) | empty |

`Address` is `localhost` because the application runs on the host (the tests, or `cargo run` of an example).
- A containerized application on the compose network would use `mosquitto`; the broker certificate covers both names.
- There is no `etc/oi4/mqtt/broker_ca.pem`, so the documented fallback to `etc/oi4/certs/ca.pem` is used.
- Secret files have mode 0600.
- No new format: `broker.json` has exactly the keys of the vectors. The Go test `TestContainerStorages` checks the layout (keys and values of `broker.json`, hostname, no `broker_ca.pem`, `etc/oi4/app/`, no credentials next to the certificate, file modes). The Rust helper `dev()` refuses a `broker.json` whose `Address` is not `localhost`, and every interop test bootstraps a copy of a sample root with `Bootstrap::ContainerStorage { root }`.

### 4. Interop tests (`oec/sdk/rust/oec/tests/interop.rs`)

- **Gating:**
  - each test returns at once unless `OEC_INTEROP=1`, so the default gate compiles and lints the file (clippy pedantic, three feature variants) but needs no engine;
  - `OEC_DEV_DIR` (default `../../../dev` of the crate) locates `generated/`;
  - `OEC_COMPOSE` (default `docker compose`) is the compose command for the restart.
  - A static mutex serializes the tests, and `interop.sh` also passes `--test-threads=1`.
- **Probe:** a blocking MQTT client on the `oec-core` codec over rustls. It:
  - logs in with the probe credentials (or the probe certificate);
  - subscribes to `Oi4v2/#` with QoS 1;
  - decodes publications and selects them by the base64url AppId element of the topic (`Oi4v2/Utility/<AppId>/Pub/<Resource>/<Source>`);
  - parses the payload as JSON for the assertions (resource, `Health`, `HealthScore`).
- **Child process:** the Will needs a client that dies without DISCONNECT.
  - The test binary starts itself again as the ignored test `child_app`, with the storage root in `OEC_INTEROP_CHILD_ROOT`.
  - The child runs `App::run` until it is killed (`Child::kill` = SIGKILL / TerminateProcess).
  - A child that is not killed by the test is killed when the test fails (drop guard).
- **Tests** (8 + the helper):

| Test | What it shows | REQs |
|---|---|---|
| `credentials_birth_health_close_disconnect` | credentials on 8883; Birth (MAM) → Health NORMAL_0/100 → shutdown → Close Health NORMAL_0/0; no Will afterwards (DISCONNECT discarded it) | REQ-MQTT-009, REQ-LIFE-001/005/015, REQ-TOP-017 |
| `client_certificate_will_of_a_killed_process` | client certificate with encrypted key on 8884; Birth, Health; the killed child's Will FAILURE_1/0 arrives | REQ-MQTT-010, REQ-CNT-008, REQ-LIFE-008 … 010 |
| `broker_restart_new_will_and_birth` | the broker restarts under a running app; the app reconnects with a new Birth and Health; then the app is killed and its new Will arrives | REQ-LIFE-016, ADR-0026 §10.5 |
| `explicit_broker` | `Bootstrap::Explicit` with `Broker::new("localhost").port(…).ca_pem(…).credentials(…)` (probe credentials); Birth → Health → Close | ADR-0020 §3 (replaces the C-050 smoke test) |
| `refused_logins` | wrong password, unknown user, credentials on the certificate listener, a certificate of a foreign CA: never `Online`, no Birth | REQ-MQTT-009/010 |
| `wrong_ca` | the app trusts a foreign CA: TLS fails, never `Online` | REQ-MQTT-007/008 |
| `tls_versions` | the probe connects with TLS 1.2 only and with TLS 1.3 only, on both listeners | REQ-MQTT-005 |
| `max_packet_size_limits_the_payload` | a payload of exactly 262144 octets on a 1000-character topic is delivered; 262145 octets are dropped and the publisher stays connected (a PINGREQ is answered) | RIG 4.1.1, REQ-CNT-003, SF-105 |

- **Restart test detail:**
  - The probe uses a persistent session (Clean Session 0, QoS 1), so it keeps what is published around the restart.
  - After the restart Mosquitto may redeliver QoS 1 messages from before it. In one run the first message seen after the restart was an old Health.
  - The test therefore ignores publications whose MessageId timestamp (Unix ms, REQ-MSG-007) is older than the restart.
  - A stale Will or Birth can no longer satisfy it, and a missing new one still times out.
- **`interop.sh`** (POSIX sh):
  - derives the engine from `OEC_COMPOSE` and checks `<engine> info`; without a running engine it prints a note and **exits 0**;
  - runs `go run ./pki` if `generated/` is missing;
  - `compose up -d --wait`;
  - `cargo test --locked -p oec --test interop -- --test-threads=1` with `OEC_INTEROP=1`;
  - `compose down -v` in a trap (`OEC_INTEROP_KEEP=1` keeps the broker).
- **Smoke test removed:** `smoke_against_a_local_broker` and the `OEC_SMOKE_*` variables are gone from `runtime.rs` and the SDK README; `explicit_broker` covers the same path against the dev broker.

### 5. Documentation and gate

- `oec/dev/README.md`:
  - prerequisites;
  - generating the PKI;
  - start/stop (Docker, Podman via `OEC_COMPOSE`);
  - ports and listeners;
  - the generated layout;
  - running an app against the broker (container storage root or explicit);
  - `mosquitto_sub` examples;
  - the interop tests;
  - troubleshooting;
  - what is never committed.
- `oec/sdk/rust/README.md`: the smoke-test paragraph replaced by the interop paragraph.
- `oec/README.md`: the row `dev`.
- `oec/check.sh` step 6 runs `go vet` + `go test` of `oec/dev` (PKI generator and config lint) – no engine, no network.
- `oec/sdk/check.sh`: header comment (interop tests compiled but not run).

### 6. Measurements

- **`interop.sh`** from scratch (`down -v` before; PKI present; image cached): 8 passed, 1 ignored (`child_app`), about 27 s.
  - The restart test takes the longest (broker restart about 1 s plus the backoff of the app).
  - Green three times in a row before the final run.
- **TLS floor checked by hand** with OpenSSL 3.0.15 `s_client` against both listeners:
  - `-tls1` and `-tls1_1`: the server answers "alert protocol version" (70);
  - `-tls1_2`: ECDHE-ECDSA-AES256-GCM-SHA384;
  - `-tls1_3`: TLS_AES_256_GCM_SHA384.
- **Without engine:** `interop.sh` with an engine that is not running (simulated) and with `OEC_COMPOSE=podman-compose` (no podman installed) prints the skip note and exits 0.
- The gate stays offline: `cargo test` runs the interop tests as no-ops (0.00 s).

## Agent decisions (G-3.4 b, for information: REVIEW A-42 … A-49)

- **A-42** Dev environment layout and Go PKI generator. The generator uses the standard library only and its tests (PKI and config lint) run in the gate as step 6.
- **A-43** Mosquitto 2.1.2:
  - pinned by tag and digest;
  - two TLS listeners (8883 credentials, 8884 client certificate with CN as user name), TLS 1.2 minimum and 1.3 allowed;
  - ports on loopback, persistence on, entrypoint copy of the TLS material.
- **A-44** Packet limits: a payload limit equal to `MaxPacketSize` plus a packet limit with header room (pending SF-105).
- **A-45** Sample container storages:
  - two roots (credentials and certificate with an encrypted key);
  - `Address` `localhost`;
  - no `broker_ca.pem`, so the fallback to `ca.pem` is used;
  - `broker.json` in exactly the vector shape.
- **A-46** Opt-in contract:
  - `OEC_INTEROP=1` and `interop.sh`;
  - the skip with exit 0 without an engine;
  - `OEC_COMPOSE` for Podman;
  - image pulls only on this path.
- **A-47** Test design: probe on the `oec-core` codec, killed child process for the Will, persistent probe with a MessageId filter for the restart, serialized tests.
- **A-48** The smoke test is replaced by `explicit_broker`. No new dependency (Rust or Go), ADR-0023 unchanged.
- **A-49** Cycle cut: C-051 not split, next the P5 interleave.

## Defects found

- **SDK:** none. The runtime of C-050 behaved against Mosquitto 2.1.2 exactly as against the in-process broker:
  - Will per CONNECT, Birth → Health, Close → DISCONNECT;
  - reconnect after a broker restart, refused CONNACK retried;
  - client certificate with an encrypted key.
- **Tests (fixed during the cycle, before the first commit):**
  1. The probe first matched publications by the container name in the topic. The AppId element is base64url-encoded (MG 4.3.1, SF-104), so the "nothing arrives" checks passed vacuously. The probe now matches the encoded AppId (`Oi4Identifier::…encode()`), so the negative checks use the same filter as the positive checks that do see the publications.
  2. The restart redelivery described in §4 (one flaky run). It is fixed by the MessageId filter.

## Changes to reviewed items

None in the sense of G-3.9: no REQ, vector or matrix entry changed, no new vector, schema unchanged (`2.0.1-dev`, no CHANGELOG entry). Agent amendment: ADR-0026 §11 (agent-accepted). ADR-0020 and ADR-0023 unchanged: no API change, no new dependency.

## Spec findings

**SF-105 (open, for the PO): RIG 4.1.1 – `MaxPacketSize` and the packet limit of the broker.** Register entry in the guideline repo, `a286511` (only `findings/`).
- **What the guideline says:** RIG 4.1.1 defines `MaxPacketSize` as the maximum MQTT **payload** (SF-091 (b)). It does not say how the platform configures the broker.
- **What brokers do:** they usually limit the whole **packet** (Mosquitto `max_packet_size`, the MQTT 5 Maximum Packet Size).
- **Measured with Mosquitto 2.1.2:** with `max_packet_size 262144`, a publisher with a payload of exactly 262144 octets is disconnected ("oversize packet"), and the broker publishes its Will.
- **Also:**
  - the maximum 268435455 is the MQTT remaining length, so a payload can never be that large;
  - what happens to a larger payload (dropped or disconnected) is open.
- **Proposed interpretation:**
  - keep the payload semantics;
  - the broker accepts every payload up to `MaxPacketSize` (a payload limit, or a packet limit ≥ `MaxPacketSize` + 65544), with a proposed sentence for RIG 4.1.1;
  - add a note on the remaining length;
  - no rule for larger payloads; D8 checks only that payloads up to `MaxPacketSize` are accepted.
- **Consequences until the decision:**
  - REQ-CNT-003 already says "the broker accepts payloads up to MaxPacketSize" (derived) and stays unchanged;
  - the dev broker and the interop test follow the proposal.

Next free id SF-106.

## Risks and open technical points (C-050 list re-checked)

- **Interop (C-050):** **closed.** Systematic interop with Mosquitto 2.1.2 is covered by `interop.sh`: dev PKI, both authentication modes, encrypted key, Will, restart, refusals, wrong CA, TLS versions and the packet limit.
- **Reconnect after a real broker restart (C-050):** **closed** (`broker_restart_new_will_and_birth`).
  - Mosquitto does not publish the Wills of its clients when it is stopped with SIGTERM, so the Will seen after the restart is the new one.
  - The test still filters out redeliveries.
- **Windows signals, aws-lc-sys on Windows/ARMv7 (C-050):** unchanged, CI. `interop.sh` itself is POSIX sh. On Windows use Git Bash, WSL or the manual steps of the README; the Rust tests and the Go generator are portable.
- **Signal handlers stay registered (C-050):** unchanged (documented, ADR-0026 §10.7).
- **Heartbeat timing (C-050):** unchanged. The interop tests do not wait 60 s; the core tests pin the deadline. D7 `monitor` (P5) will observe heartbeats on the dev broker.
- **No offline queue (C-050):** unchanged, increment 6.
- **New: Podman.** `podman compose` with Docker Compose as the provider works through `OEC_COMPOSE`. `podman-compose` 1.x has no `up --wait`, as documented in the README. Not tested locally (no Podman installed).
- **New: the image digest.** It pins the multi-arch index. A rebuilt upstream tag is not picked up until the digest is changed deliberately (intended). The digest is checked by the config lint, not against the registry (offline gate).
- **New: SF-105.** Until the PO decides, `max_packet_size` of the dev broker follows the proposed interpretation.

## Commits

Workspace (never pushed):
- `394d731` dev environment: `.gitignore`, Go module, PKI generator with tests, `mosquitto.conf`, `compose.yaml`;
- `2ae3a74` interop tests, `interop.sh`, `oec/dev/README.md`, smoke test removed, SDK README, `sdk/check.sh` header;
- `e007e55` gate step 6 (`oec/dev` Go tests), `oec/README.md`;
- ADR-0026 §11, this log, STATUS, ROADMAP, AGENTS.md;
- REVIEW.md/REVIEW_DE.md.

Guideline: `a286511` (SF-105 register entry, only `findings/`; `.yarn/install-state.gz` stays uncommitted).

## Checks (final)

`oec/check.sh`: all checks passed. Included in it:
- spectool `lint`, `render -check`, `coverage -check -gate` (0 gaps), `topic -check`, `inventory -check -rev 734c560` (1399 items);
- `schema/check.sh`, release freeze, `spectool crosscheck`;
- `sdk/check.sh`:
  - the workspace in both feature sets (fmt, clippy pedantic incl. `tests/interop.rs`, tests, doc);
  - clippy with `crypto-ring`;
  - cargo-deny;
  - sketches and spikes;
- the Go tests of `oec/spec/tools`, `oec/conformance` and (new) `oec/dev`.

Counts: 440 REQs, 158 `schema`/`d7-validate` REQs with 0 gaps, entry schemas 29 of 29, 245 payload vectors, 18 `broker.json` vectors.

`oec/dev/interop.sh` against the local broker (Docker 28.4, Compose 2.39): 8 passed, 1 ignored, about 27 s.

## For the PO (review)

- **To decide: D-1 = SF-105** (`MaxPacketSize` vs. the packet limit of the broker), with the proposed interpretation and text for RIG 4.1.1.
- **No test data to review:** no new vector, no R-x.
- **For information:** `REVIEW.md` A-42 … A-49. Worth a look:
  - A-43 (Mosquitto 2.1.2, two listeners);
  - A-44 (packet limits, tied to SF-105);
  - A-46 (how the interop tests are started and why the gate stays without Docker).

## Next

**P5 interleave** (ROADMAP P4 "Order and cycle cut", ADR-0025, `oec/conformance/DESIGN.md`):
- D7 `internal/mqttwire` (MQTT 3.1.1 client and codec on `paho.mqtt.golang/packets`, ADR-0025);
- `internal/capture` (JSONL recording, recorder, replayer);
- `oec-check monitor --replay` (offline, deterministic);
- then `monitor` live against the dev broker of this cycle (TLS, credentials or client certificate).

The SDK interop tests can provide captures as regression data (DESIGN.md: data files, not code, Q-009). After that come increments 6–8 (provider, write and consumer side) and 10 (`oec-ffi`). The P5 interleave gets its own cut at the start of the next cycle.

Next free ids: SF-106, REQ-CMP-048, REQ-RES-PUBLIST-028, REQ-TOP-029, V-PAY-246, V-TOP-138 (valid) / V-TOP-255 (invalid), V-BRK-019, P-129, ADR-0027, Q-031, A-50.

**Agent for the P5 interleave (G-3.10): new agent.**
- The context of this session was compacted several times during C-051.
- The next cycle is a different deliverable (D7, Go), with other files and other ADRs (ADR-0013/0025, DESIGN.md); none of the Rust context of P4 is needed.
- The state is handover-ready (G-3.7): this log, ADR-0026 §11, `oec/dev/README.md`, STATUS "Next step" 1, ROADMAP P4/P5 and `REVIEW.md`.

Start prompt: `prompt.md` (German, not committed).

## Addendum 1 – PO review: D-1 (2026-10-26)

### PO input

1. "Für D-1 wähle ich option b da sie präzise ist".
   - This was ambiguous: (b) is one part of the proposal (the note on 268435455), not one of the choices (as proposed, (d), (e), other); on its own it does not solve problem (a).
   - The PO was not available for a question, so the guideline was not changed (G-3.4 a).
   - The reply and the question were recorded in REVIEW D-1 (`9ee19db`).
2. Clarification: "Vorschlag komplett" → (a) + (b) + (c) as proposed.

### Guideline (G-3.4 a, PO order)

- `59e3ae3` (branch `feature/spec-findings`), RIG 4.1.1 card `MaxPacketSize`, after the sentence on the value range:
  - (b) "A payload is limited further by the MQTT packet: payload + topic length + 4 must not exceed 268435455."
  - (a) "The broker accepts every MQTT payload up to `MaxPacketSize`. If the broker limits the size of the whole MQTT packet, this limit includes the fixed header, the topic and the packet identifier and must therefore be larger than `MaxPacketSize` (by 65544 octets for any topic)."
  - (c) no text: what the broker does with a larger payload stays open on purpose.
  - (b) is placed before (a) so that it directly follows the sentence on the maximum it refines; the wording is the one of the proposal.
- Register `d220c48`: SF-105 `resolved (59e3ae3)`, moved to Closed; no open finding; next SF-106.

### Re-baseline on `59e3ae3`

- Branch head was the text commit, so no detached checkout was needed; `spectool rebaseline -rev 59e3ae3`.
- Inventory 1399 → **1401**: RIG-4.1.1-fb78a5f5 (MUST NOT, (b)) and RIG-4.1.1-674b5bb8 (MUST, packet limit of (a)); the sentence "The broker accepts …" has no keyword and is quoted only. No other key changed.
- Both items are mapped to **REQ-CNT-003**:
  - the statement now says that the broker accepts every payload up to MaxPacketSize, that a packet limit is larger by 65544 octets, and that payload + topic length + 4 does not exceed 268435455;
  - three new quotes;
  - the notes record SF-105 (c) and that (b) restates the MQTT remaining-length limit (no own check);
  - status → `draft`, **R-1** for the PO.
  - Not a mechanical remap, because the statement changes.
- `spectool lint`, `render`, `coverage`: CNT 0 open, 158 REQs with 0 gaps.
- Schema: `Baseline` `59e3ae3`, `Version` stays `2.0.1-dev` (no content change, `broker.json` keeps its form and range, V-BRK-001 … 018 unchanged), CHANGELOG `Unreleased` entry; D7/D8 catalog baselines `59e3ae3`.
- **A-50 (agent, G-3.4 b):** D8 T8-BUS-01 additionally lists REQ-CNT-003. It publishes a payload of exactly `MaxPacketSize` on a long topic and expects delivery without a disconnect. Larger payloads are not checked (SF-105 (c)). Until now, D8 checked REQ-CNT-003 only on the host side (T8-HST-01, `broker.json`).
- Unchanged, because they already follow the decision: `oec/dev/mosquitto/mosquitto.conf` (`message_size_limit 262144`, `max_packet_size 327688`), the SDK (`oec` limits the outgoing payload, accepts packets up to `MaxPacketSize` + 65539), the interop test `max_packet_size_limits_the_payload`. ADR-0026 §11.3 notes that SF-105 is resolved.
- Project docs: AGENTS.md, GUARDRAILS G-1.2, ADR-0001 amendment C-051 review, STATUS, this log, review sheets; `prompt.md` (not committed) updated for C-052.

### Checks

- `oec/check.sh` → "all checks passed" (incl. `inventory -check -rev 59e3ae3` with 1401 items, coverage gate 158 REQs / 0 gaps, schema baseline = spec baseline, D7/D8 catalog coverage).
- The interop run was not repeated: neither the broker configuration nor the SDK changed.

### Next ids

SF-106, REQ-CMP-048, REQ-RES-PUBLIST-028, REQ-TOP-029, V-PAY-246, V-TOP-138 (valid) / V-TOP-255 (invalid), V-BRK-019, P-129, ADR-0027, Q-031, A-51.

The agent decision for C-052 is unchanged: a **new agent**, start prompt in `prompt.md`.
