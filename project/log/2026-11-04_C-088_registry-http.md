# C-088 – P7 increment 3: Registry HTTP (REST/OpenAPI, SSE, TLS)

- Date: 2026-11-04
- Phase: P7 (OEC Registry D5), increment 3 of 6 (ROADMAP P7 cycle cut, A-381)
- Agent session: new (handover of C-087, `prompt.md`)
- Baseline: guideline `52d8470` (branch `feature/spec-findings`, head `eb821e9`), SF-001 … 117 resolved, next SF-118; workspace head `694fcff`

## Intent

1. **HTTP side of the registry** (ADR-0029 §7): read-only REST (`GET` only, `405` otherwise) `/api/assets`, `/api/assets/{il}`, `/api/events`, `/api/stream` (SSE), `/api/openapi.json`, `/api/asyncapi.json` on the directory of C-087; the `#[allow(dead_code, …)]` of the directory go.
2. **OpenAPI 3.2.0** `api/openapi.json` (SSE with `itemSchema`), validated against the vendored official meta-schema; paths = router.
3. **TLS** with the application certificate of the container storage, otherwise self-signed at start; plain HTTP only explicitly (dev). Ports 5798 (UI + `/api`) and 5799 (API only), switchable by environment; the hardening contract of ADR-0028 amendment C-083.
4. `hyper` 1 stack for `oec-registry` only (ADR-0023 amendment C-086); versions, tree growth, `cargo deny` recorded.
5. Go integration tests HTTP/SSE/TLS (standard library); D7 `level3` and D8 T8-BUS-03 stay green; read-only stays hard.

## Plan

1. Baseline and gate.
2. Dependencies (hyper, hyper-util, http-body-util, rcgen; SDK versions of tokio, tokio-rustls, rustls, rustls-pki-types, pkcs8), `cargo deny`.
3. `src/http.rs` (listeners, TLS, connection limits, guards, keep-alive), `src/tls.rs` (certificate), `src/api.rs` (router, views, SSE); directory revision and event sequence for the stream; unit tests.
4. `api/openapi.json` + meta-schema + Rust tests (meta-schema, paths = router, views against the response schemas).
5. Go tests: TLS (self-signed, application certificate, plain only explicit), REST, SSE, contract (guards, limits, slow, keep-alive), ports; D7/D8 runs with HTTP off.
6. Docs, gate, commits.

## Work

### 1. Baseline

Guideline `52d8470` checked out, branch head `eb821e9`; cbindgen 0.29.4. Counts as in the handover (1411 inventory items, 444 REQs, 158 `schema`/`d7-validate` REQs without gap, 55 D7 + 17 D8 tests `implemented`, 89/89 D7 and 164/164 D8 fixtures reviewed). `oec/check.sh` at the start (workspace head `694fcff`) → "all checks passed".

### 2. Dependencies (A-398, ADR-0023 amendment C-088)

`hyper` 1.12 (`server`, `http1`), `hyper-util` 0.1.21 (`tokio`) and – at the versions and features of the SDK – `tokio`, `rustls` (`aws_lc_rs`), `tokio-rustls`, `rustls-pki-types`, `pkcs8`, plus `rcgen` 0.14 for the self-signed certificate. `http-body-util` (named in ADR-0029 §7) is not needed: the registry has one small body type and reads no request body. `Cargo.lock` 169 → 195 packages (8 new to the workspace), normal tree 124 → 137 crates; `cargo deny check` ok without exception.

### 3. HTTP side (`oec/registry/src`)

- `api.rs` (sans-IO): router (`/`, `/api/assets`, `/api/assets/{il}`, `/api/events`, `/api/stream`, `/api/openapi.json`, `/api/asyncapi.json`), the guards of ADR-0028 amendment C-083 in the registry variant (Host, Origin, Transfer-Encoding, Content-Length, `GET` only → `405` with `Allow: GET`), the JSON views of the directory (camelCase, MG payloads as received) and the stream diff (`snapshot`, `asset`, `event`, `directory`).
- `http.rs`: two listeners (5798 web: `/` + `/api`; 5799: `/api` only), TLS by `tokio-rustls`, a current-thread tokio runtime on its own thread, limits (16 connections, 8 streams, `503` beyond), timeouts (5 s handshake, 5 s head, 10 s response), keep-alive comments, the common security headers, `Connection: close`.
- `tls.rs`: the application certificate of the container storage (also an encrypted PKCS#8 key) or a self-signed one by rcgen (ECDSA P-256, SANs localhost, container name, `OEC_REGISTRY_HOSTS`, `127.0.0.1`, `::1`).
- `main.rs`: settings parsed and ports bound before the SDK application starts (errors exit ≠ 0 before any MQTT connect); logs which certificate is used and the URLs, a warning for plain HTTP.
- `directory.rs`: `EventRecord::seq` (counted since the start) for the stream; `me()`; every `#[allow(dead_code, …)]` removed (all read by the HTTP side). `bus.rs`: the directory is shared as `Arc<Mutex<…>>`; the read-only source scan also covers the HTTP side (A-404).
- `index.html`: a static placeholder page on `/` until the Web UI (C-089).

### 4. OpenAPI (A-403)

`api/openapi.json` (OpenAPI 3.2.0; the stream as `text/event-stream` with `itemSchema`; MG payloads by `$ref` to the strict D1 schemas), generated once by a script of the session and then maintained by hand. The official schema iteration `2026-08-30` of `https://spec.openapis.org/oas/3.2/schema/2026-08-30` is vendored in `api/meta/` with the Apache-2.0 license; the download was checked byte-identical (sha256 `ec53db6e114b768ff2d273b232bda210f51b70c5e57d57ac8daf7e8496a6e073`). ADR-0029 §11 named the first iteration `2025-09-17`; the newest published one is used (A-403). Rust tests: valid against the meta-schema (and a broken copy is rejected), paths = router with `GET` only, every `$ref` resolves, the views satisfy the response schemas.

### 5. Go integration tests (A-405)

`internal/regtest/http.go` (harness: HTTPS client, SSE reader, the contract checks, certificates, OpenAPI loader) and `http_test.go`:

- `TestHTTPSelfSigned`: SANs and `serverAuth`, the same certificate on both ports, no plain HTTP on a TLS port; with a participant announced on the bus (MAM, Health, Event): the stream snapshot and the `asset`/`event` messages, `/api/assets`, `/api/assets/{il}`, `404` for an unknown asset, `/api/events`, the served documents equal the files, every OpenAPI path answers `200`, `/` only on the web port; subtests guards (every non-`GET` method incl. `HEAD`/`OPTIONS` → `405`, Host/Origin/TE/size), slow client (handshake and head closed after ≈ 5 s), keep-alive, limits (8 streams, 16 connections, `503` beyond, freed slots usable again).
- `TestHTTPApplicationCertificate`: own CA and leaf in the storage; the registry presents the leaf, verified with the pinned CA and the container name; MQTT with the same certificate works.
- `TestHTTPPlain`: `OEC_REGISTRY_TLS=off`, warning logged, guards and stream.
- `TestHTTPSettings`: equal addresses, invalid address, `TLS=maybe`, `KEEPALIVE_MS=0`, a taken port → exit ≠ 0 with a message and no MQTT connect; both ports off → "HTTP: off".

`StartRegistry` (D8/D7/RD runs) sets HTTP off (`regtest.HTTPOff`), so the bus tests do not depend on free ports. Debugging notes: the fake participant first used DataSetWriterId 1 (the pagination helper, REQ-MSG-019) and moved to 10; the rcgen leaf is not a CA, so the self-signed check verifies the signature directly. `go test -race -count=3 -run TestHTTP` stable.

### 6. Gate

See "Checks (final)".

## Agent decisions (G-3.4 b, for information: REVIEW A-398 … A-406)

A-398 HTTP stack without `http-body-util`; A-399 TLS identity (application certificate, else rcgen self-signed; read at start); A-400 ports, settings, bind `0.0.0.0`, start fails early; A-401 hardening contract in the registry variant (`405` for every non-`GET`, no `408` on timeout); A-402 resources and stream messages; A-403 OpenAPI 3.2.0 and the meta-schema iteration `2026-08-30`; A-404 read-only scan of the HTTP side; A-405 Go HTTP tests and HTTP off in the bus runs; A-406 placeholder page on `/` until C-089.

## Test data for review (G-3.9)

- None. No D7/D8 fixture, scenario or verdict was added or changed (the Go HTTP tests check the registry's own behaviour, no conformance verdicts).

## Changes to reviewed items

- None.

## Spec findings

- None. MG A1 (TLS, application certificate or self-signed, ports 5798/5799) is implemented as written.

## Dependencies

- Rust (`oec-registry` only, ADR-0023 amendment C-088): `hyper` 1.12.0, `hyper-util` 0.1.21, `rcgen` 0.14.10 and the SDK versions of `tokio`, `rustls`, `tokio-rustls`, `rustls-pki-types`, `pkcs8`. Lock 169 → 195, normal tree 124 → 137; licenses MIT / MIT OR Apache-2.0; `cargo deny` ok, no exception.
- Vendored data: the OpenAPI 3.2 schema `2026-08-30` (Apache-2.0, license file beside it).
- Go: standard library only. C: none. npm: none (C-089).

## D7, SDKs, demos

- Unchanged: D7, D8, Rust SDK, C ABI, Go SDK, demos (pins 46/49/22). Report schemas frozen, `COVERAGE.md` unchanged. A-113 untouched.

## Docker objects (own only)

- No Docker was used in C-088 (no image, container, volume or network created). Before = after: none.

## Commits

- `893ddd5` Registry HTTP: `api.rs`, `http.rs`, `tls.rs`, `index.html`, `main.rs`/`directory.rs`/`bus.rs`, `api/openapi.json`, vendored OpenAPI 3.2 schema, dependencies, Rust and Go tests, `check.sh`, README.
- This commit: ADR-0023/ADR-0029 amendments C-088, log, REVIEW, REVIEW_DE, STATUS, ROADMAP.

## Checks (final)

- `oec/check.sh` → "all checks passed", rc 0, on the final code (load ~19 at the peak, ~4 at the end; no flake). Step 8: `registry/rust` fmt, clippy, 31 tests, deny, build; `registry/go` `ok github.com/OI4/oec/registry 50.7s` (TestD8Registry T8-BUS-03 pass, TestD7Level3 49 pins/0 fail, TestReferenceDesignationPersists, the four TestHTTP*), `internal/regtest` ok. Demo pins unchanged (Logger L2 46, Weather L3 49, Config L1 22).
- Before: `oec/registry/check.sh` alone green; `go test -race -count=3 -run TestHTTP` stable.
- Only documentation changed after the gate run (amendments, log, REVIEW, STATUS, ROADMAP) – nothing the gate checks.
- No Docker objects created or removed in C-088.

## For the PO

- Nothing to review (no R-x), nothing to decide (no D-x), no SF.
- For information: A-398 … A-406 – especially A-399 (TLS identity), A-400 (bind to all interfaces inside the container) and A-403 (newer meta-schema iteration than named in ADR-0029 §11).

## Handover (G-3.10)

**A new session is recommended** for C-089: this session's context was compacted. `prompt.md` (workspace root, German, not committed) holds the start prompt of C-089.

## Next

C-089 – P7 increment 4: Web UI (React + TypeScript + Vite, ADR-0029 §9/§10): applications and devices, MAM, latest Health and history, availability, Profile, events, observed level, conformity on the REST/SSE API of C-088; npm policy and its offline Go check (A-384); `ui/dist/` embedded and served on `/` of the web port instead of the placeholder.
