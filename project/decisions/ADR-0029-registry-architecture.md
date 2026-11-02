# ADR-0029 – OEC Registry architecture (P7, D5)

- Status: **accepted (agent, G-3.4 b)** – the PO decided the read-only scope (ADR-0008) and the technology (Q-035, C-085 addendum 2); the agent decided the architecture below (A-380 … A-389)
- Date: 2026-11-02
- Deciders: agent (architecture, versions, crates, gates); product owner (Thomas) – scope, technology, licenses
- Related: ADR-0006 (D7 independent), ADR-0008 (read-only step 1, availability, observed level), ADR-0012 (monorepo `oec/`), ADR-0015 (v2 only), ADR-0016 (level targets: Registry Messaging L3, Service Container L3), ADR-0019 (D1 schemas, jsonschema-rs), ADR-0020 (Rust API), ADR-0023 (Rust dependencies), ADR-0026 (Rust workspace), ADR-0028 (demo architecture, hardening contract of the Web UIs), Q-006, Q-035, MG A1, REQ-REG-001 … 007, REQ-CMP-013, REQ-LIFE-011 … 013

## Context

D5 is the OEC Registry of MG A1: it collects the `MAM` and `Health` of the assets on the Message Bus, answers the Registry services `Get/MAM[/<Source>]` and `Get/Health[/<Source>]` for all *available* assets with its own `PublisherId` and `DataSetWriterIds` (REQ-REG-002), and shows applications and devices with MAM, latest Health and availability in a web front-end (REQ-REG-003). Optional: Profile, publisher relation, Event audit trail, its log file, schema conformity, observed Messaging level (REQ-REG-004/005, REQ-CMP-013). The web front-end and its REST API should use TLS with the application certificate and a self-signed fallback; the default ports are 5798 and 5799 (REQ-REG-007).

Step 1 is read-only (ADR-0008): the registry sends `Get` only, never `Set`, `Del` or `Call` to other participants. The PO fixed the technology in Q-035: backend in Rust on the SDK `oec`, Web UI React + TypeScript + Vite (npm pinned, licenses checked, build embedded), REST with OpenAPI, live updates by SSE, the MQTT interface in AsyncAPI. Targets (ADR-0016): Messaging L3, Service Container L3; D7 and D8 at `level3` are the P7 exit.

D8 already judges a registry: T8-BUS-03 (`oec-infra bus`) discovers it on `Oi4v2/Registry/+/Pub/#`, starts two probe components with Birth, a second Health and a Will, and expects both with their last Health in the answers to `Get MAM`/`Get Health` with and without Source, and neither after Close or Will (simulated registry `oec/conformance/internal/bus/registrytest`).

## Decision

### 1. Layout (A-380)

```text
oec/registry/
├── README.md            what it does, how to run and test it
├── check.sh             gate step of the registry (called by oec/check.sh)
├── Cargo.toml/.lock     Rust package `oec-registry` (own lockfile, outside the SDK workspace)
├── src/                 main.rs (wiring), directory.rs (sans-IO model), bus.rs (SDK side),
│                        later: http/ (REST, SSE, TLS), the embedded UI table
├── api/                 asyncapi.json (MQTT interface), later openapi.json (REST),
│                        meta/ (vendored official meta-schemas with their licenses)
├── ui/                  later: React + TypeScript + Vite sources, package(-lock).json, dist/
├── go.mod               Go module github.com/OI4/oec/registry (standard library only)
└── internal/regtest/    Go integration-test harness; *_test.go: D7, D8, HTTP
```

- **Rust package like the config service** (ADR-0028 §7): `oec` by path, own `Cargo.lock`, `[workspace]` empty, edition 2024, MSRV 1.89, the SDK lints (`clippy::pedantic`, `unsafe_code = forbid`), `cargo deny` with `oec/sdk/rust/deny.toml`.
- **Go integration tests in their own module** next to the crate. The harness of the demos (`oec/demos/internal/demotest`) cannot be imported across the `internal` boundary and P6 is closed; `internal/regtest` takes over the parts it needs (test broker, container storage, process start, D7 and D8 runs), standard library only. A later consolidation of both harnesses is possible but not planned.
- Binary `oec-registry`; build output in `target/` (git-ignored).

### 2. Identity and levels

- ServiceType `Registry`; product `oi4.org` / Model `OEC Registry` / ProductCode `OEC-REGISTRY`; SerialNumber from the container (hostname); container storage contract of RIG 4.1 below `OEC_ROOT` (like the demos).
- Messaging **L3** and Service Container **L3** at the P7 exit. Increment 1 runs at **L2** (A-389): the SDK serves `Get` only from level 2 (ADR-0020 §3), and the Registry services are `Get`s; L2 needs no persistent `ReferenceDesignation`. Increment 2 raises it to L3 with its own `Config` (registry settings) and the D7 handling of listed assets (§5).
- The registry represents no entities. Its own assets are only itself.

### 3. Directory model (sans-IO, `directory.rs`)

One entry per Identification Link (the `Source`), kept in memory (no persistence in step 1; a restart rediscovers within one heartbeat interval):

| Field | Meaning |
|---|---|
| `il` | Identification Link (`Source`, `DataSetWriterName`) |
| `kind` | `application` (its `Source` is the `AppId` of its topics, own MQTT client) or `device` (represented: `Source` ≠ `AppId`) |
| `publisher` | ServiceType and AppId whose topics carried it (for a device: the related publisher application, A1 optional) |
| `mam`, `health` | the last received payloads (typed by the SDK) |
| `availability` | `online`, `offline-graceful` (Close), `offline-ungraceful` (Will or FAILURE_1/0 about itself) |
| `first_seen`, `last_seen`, `online_since` | times of the registry clock |
| later | `profile`, health history (bounded), events (bounded trail), observed level, conformity counters |

Rules (MG 6.3, A1, ADR-0008 amendment C-008, REQ-LIFE-011 … 013):

1. A `Pub/MAM` of an application about itself without `ReplyTo` is its **Birth**: the application is online.
2. An application **not yet known** (the registry started later) is online with its first `Pub/MAM` or `Pub/Health` about itself (heartbeat, ≤ 60 s, SF-106); the registry then asks for what it lacks with `Get` (§4).
3. A `Health` of an application about itself with `NORMAL_0`/0 is its **Close** (offline graceful), with `FAILURE_1`/0 its **Will** or a broken state (offline ungraceful). It is online again **only after its next Birth** (REQ-LIFE-013); answers and heartbeats do not bring it back.
4. A **device** is online while its publisher application is online and its `MAM` was received since the publisher came online; when the publisher goes offline, its devices go offline with the same kind.
5. **Available** (A1) = online with both `MAM` and `Health` known. Only available assets are listed in the Registry services; the web front-end shows all entries with their availability.
6. v1 traffic (`oi4/…`) is not subscribed (ADR-0015). Messages that do not parse as v2 are counted, not stored.

### 4. Discovery and active `Get`s (read-only)

- Subscriptions: `Oi4v2/+/+/Pub/MAM/#`, `Oi4v2/+/+/Pub/Health/#` (increment 1); `Pub/Profile/#`, `Pub/Event/#`, `Pub/PublicationList/#`, `Pub/SubscriptionList/#` (increment 2).
- After a Birth or the first sighting the registry sends `Get MAM`, `Get Health` (whatever is missing) and, from increment 2, `Get Profile`, `Get PublicationList`, `Get SubscriptionList` to that application (MG A1 note: "a number of `Get` messages … request information from the application"). Rate-limited per application; answers arrive on the normal `Pub` topics and feed the same rules.
- **Hard rule:** the registry code calls only `Consumer::get`. A unit test of the bus side asserts that no `set`, `del` or `call` of the consumer is used (source scan), and the D8/D7 integration runs see no `Set`/`Del`/`Call` from the registry towards others.

### 5. Registry services (MG A1) and the SDK extension "listed assets"

The Registry services are the regular `Get/MAM[/<Source>]` and `Get/Health[/<Source>]` topics of the registry's own AppId. The SDK answers them for the application and its represented assets; for the registry it must also answer them for the available assets of others – without making them represented assets (no heartbeat, no PublicationList entry, no Profile, no other resources). An additive SDK extension does this (ADR-0026 amendment C-086 §22, A-386):

- `AppHandle::list_asset(il, mam, health)` puts or replaces a **listed asset**, `AppHandle::unlist_asset(il)` removes it; allowed only for ServiceType `Registry` (`Error::Invalid` otherwise), never for the application itself or one of its represented assets.
- `Get MAM`/`Get Health` without Source: the application, its represented assets, then the listed assets (sorted by IL), each its own `DataSetMessage` of one `NetworkMessage`, paginated by the SDK if too large. With the Source of a listed asset: exactly its `DataSetMessage`. Every other resource with the Source of a listed asset is answered as an unknown Source (no `Pub`, Status event `BadNotFound` under the AppId, REQ-CMP-047, SF-099/108).
- `DataSetWriterIds` of listed assets are the registry's own (A1): assigned per (resource, IL) by the SDK's writer table, persisted like the others (stable across restarts, REQ-MSG-021). `DataSetWriterName` = the asset's IL; the payload is the last received `MAM`/`Health` (re-serialized from the typed value).
- Listed assets are never published on their own initiative.
- **D7 refinement for registries** (increment 2, A-388): T7-GET-04 and T7-SET-03 treat every further Source of the answer to `Get MAM` as a represented entity, T7-MON-04 reports a Source that publishes on the bus itself (REQ-CMP-045, info). For ServiceType `Registry` these Sources are listed assets (A1), so D7 gets a small additive refinement (ADR-0025 amendment, test first): for a registry, only the application itself is an entity of T7-GET-04/SET-03/MON-04. If the run shows that the guideline text itself is ambiguous, an SF is opened instead. Seen already at L2 in increment 1 (C-086): T7-GET-04 warns REQ-CMP-010 (D7 and its peer, listed by the registry, do not answer `Get Profile` with their Source) and T7-MON-04 reports REQ-CMP-045 as info; no test fails, 44 pass.

### 6. Observed level and conformity view (increment 2, informative)

- **Observed Messaging level** (REQ-CMP-013, ADR-0008): derived from the `Profile` and the answers to the registry's `Get`s, with the level tables of `oec/spec` as source; shown as "observed, informative", never as a compliance claim.
- **Conformity view**: every received message is validated with the D1 schemas through the SDK feature `strict-validation` (jsonschema-rs embedded, ADR-0019/0023), independent of D7 (ADR-0006: the registry does not link or run D7). Per participant: number of messages, of schema violations, and the last violations (bounded).

### 7. HTTP: REST, SSE, TLS, ports (increment 3, A-383)

- **Server crate:** `hyper` 1 (`server`, `http1`) with `http-body-util` and `hyper-util` (`tokio`), on the SDK's `tokio` and `tokio-rustls`; recorded in ADR-0023 amendment C-086 for the package `oec-registry` only. Reason: a product-grade HTTP/1.1 parser and SSE streaming over TLS without writing a parser of our own; all crates MIT or MIT/Apache-2.0.
- **Ports:** `5798` web front-end (UI and the same REST API under `/api`, same origin, no CORS) and `5799` REST API only (A1 default ports); configurable by environment (`OEC_REGISTRY_WEB`, `OEC_REGISTRY_API`, empty = off).
- **TLS** (REQ-REG-007): the application certificate of the container storage (RIG 4.1.2/4.1.3) if present, else a self-signed certificate generated at start (crate chosen in increment 3 within ADR-0023); plain HTTP only when explicitly configured (dev).
- **REST:** read-only (`GET` only, `405` otherwise): `/api/assets`, `/api/assets/{il}`, `/api/events`, `/api/stream` (SSE), `/api/openapi.json`, `/api/asyncapi.json`. Hardening contract of ADR-0028 amendment C-083 (limits, timeouts, guards, keep-alive).
- **OpenAPI 3.2.0** (A-382): `api/openapi.json`; the SSE stream is described with `text/event-stream` and `itemSchema` (new in 3.2).

### 8. AsyncAPI (A-382, A-385)

- **AsyncAPI 3.1.0**, `api/asyncapi.json`: the MQTT interface of the registry – the Registry services (receive `Get/MAM[/<Source>]`, `Get/Health[/<Source>]`, send the answers), its own Birth/Health/Close/Will, the subscriptions and the read-only `Get`s it uses to collect (increment 1), and from increment 2 its further own resources. Payloads reference the D1 schemas by their `$id`.
- Increment 1 describes the Registry services, the lifecycle, the collecting subscriptions and `Get MAM`/`Get Health`; every later increment extends the document with what it adds.

### 9. Web UI (increment 4, A-382, A-384)

- React **19.3.1** + react-dom 19.3.1, TypeScript **7.0.2**, Vite **7.3.x** (the maintained 7 line) – not Vite 8, whose hard dependency `lightningcss` is MPL-2.0, outside the license policy (ADR-0023). JSX by Vite's esbuild (`jsx: automatic`), no `@vitejs/plugin-react` (Babel tree). Exact versions are re-checked and pinned when increment 4 starts.
- Shipped code (the bundle) is React, react-dom and scheduler only (MIT). Node ≥ 22.12 is a development tool only.
- **Embedded:** `ui/dist/` (built files) is committed, like the generated `ui_assets.c` of the logger; the binary embeds it with `include_bytes!` through a generated table; no npm at run time or in the image build.

### 10. npm policy and its gate (A-384)

- `package.json` pins exact versions (no `^`/`~`); `package-lock.json` (lockfileVersion 3) is committed; every package is `resolved` from `https://registry.npmjs.org/` with an `integrity` (sha512).
- Licenses: every package of the lockfile (shipped and build-time) is in the allowlist of ADR-0023 (MIT, Apache-2.0, ISC, BSD-2-Clause, BSD-3-Clause, Zlib, Unicode-3.0, MIT-0); anything else is a question to the PO.
- `npm ci --ignore-scripts` (no install scripts), then `npm run build` (`tsc --noEmit` + `vite build`): an opt-in script `ui/build.sh` that needs the npm cache or network once; it writes `ui/dist/` and a stamp (sha256 over the UI sources, `package.json` and `package-lock.json`).
- The gate needs no node and no network: a Go test checks the lockfile rules and the licenses, and recomputes the stamp, so changed UI sources without a rebuilt `dist/` fail.

### 11. Tests and gate (A-385, A-387)

- `oec/registry/check.sh`, a new step of `oec/check.sh` after the demos: `cargo fmt --check`, `clippy -D warnings`, `cargo test`, `cargo deny`, build; `gofmt`, `go vet`, `go test -race` with the test broker, `oec-check` and `oec-infra` (built into a temporary directory like the demos).
- Rust unit tests: the directory rules (§3), the bus side, the documents (§8).
- Go integration tests: **D8 `oec-infra bus --tests T8-BUS-03`** against the registry on the test broker (pass); **D7 `oec-check test`** at the registry's current level with pinned passes; later HTTP/SSE/TLS and the UI files.
- **API documents in the gate:** `api/openapi.json` and `api/asyncapi.json` are validated in the Rust tests against the official meta-schemas (OpenAPI 3.2 schema `2025-09-17`, AsyncAPI 3.1.0 JSON Schema), vendored under `api/meta/` with source URL and license (Apache-2.0), with jsonschema-rs `=0.60.1` as a dev-dependency (the pin of ADR-0019 §8); plus semantic tests: the AsyncAPI channels match the topics the registry subscribes to and answers, the OpenAPI paths match the router.

### 12. Image (increment 5)

As the demos (ADR-0028 §4): multi-stage, linux/amd64 and arm64, non-root, `/etc/oi4/app` as volume, SBOM and license texts, D8 `image`/`run` at `--level 3` in `build.sh`; compose profile `registry` in `oec/dev` (opt-in). Publication and signing stay P9.

## Consequences

- One small additive SDK extension (listed assets) instead of registry-specific hacks; other applications cannot use it (ServiceType check).
- The registry is the first product with npm. The pinned lockfile, the license allowlist without exception and the committed `dist/` keep the gate offline and node-free.
- D7 needs one additive refinement for registries at level 3 (increment 2).
- The P7 cycle cut is in the ROADMAP (A-381).
