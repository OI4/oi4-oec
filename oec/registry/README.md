# OEC Registry – D5 (Rust on `oec`)

The OEC Registry of P7 (ADR-0029, Q-035): it collects the `MAM` and
`Health` of every participant on the OI4 Message Bus and offers the
available assets with the Registry services of MG A1 – `Get/MAM[/<Source>]`
and `Get/Health[/<Source>]` on its own AppId. Step 1 is **read-only**
(ADR-0008): towards others it only subscribes and sends `Get`, never `Set`,
`Del` or `Call`. ServiceType `Registry`, product `oi4.org` / `OEC Registry`
/ `OEC-REGISTRY`, SerialNumber = the hostname of the container.

State (C-087, increment 2 of the P7 cycle cut in `project/ROADMAP.md`):
Messaging Level 3. The registry collects `MAM`, `Health`, `Profile`,
`Event`, `PublicationList` and `SubscriptionList` of every participant
(raw subscriptions of the SDK, ADR-0026 §23), asks with `Get` for what is
missing (once per online period and `GetInterval`), keeps the publisher
relation and the devices, an observed Messaging level (informative) and a
conformity view (every received message validated with the strict D1
schemas, independent of D7). It has its own `Config` (below) and a
persistent `ReferenceDesignation`; D7 `oec-check test --level 3` passes.
Next: REST/OpenAPI/SSE/TLS on 5798/5799 (increment 3), the
React/TypeScript/Vite UI (increment 4), the image (increment 5).

### Config `registry`

Group `directory`, read by the worker at least once a second; values out
of range are clamped:

| Setting | Default | Range | Meaning |
|---|---|---|---|
| `GetInterval` | 10 s | 1 … 3600 | least time between two `Get` of a resource of a participant |
| `GetTimeout` | 5 s | 1 … 60 | time an answer may take |
| `HealthHistory` | 20 | 0 … 1000 | kept `Health` changes per asset |
| `EventTrail` | 200 | 0 … 10000 | kept `Event`s of all participants |
| `ViolationTrail` | 10 | 0 … 1000 | kept schema violations per participant |

## Build and run

An own Cargo package outside the SDK workspace with its own `Cargo.lock`;
`oec` is a path dependency (`../sdk/rust/oec`):

```sh
cd oec/registry
cargo build --offline --locked          # target/debug/oec-registry
OEC_ROOT=<container storage root> target/debug/oec-registry
```

The broker, the credentials and the identity come from the container
storage under `OEC_ROOT` (default `/`) through the SDK: `broker.json`,
`/etc/oi4/mqtt/…`, `/run/secrets/oi4` (REQ-CNT-003, GL-ADR-022). Run it on
a **copy** of an example storage, never on `oec/dev/generated/` itself.
`OEC_REGISTRY_LOG` = `error`, `warn`, `info` (default) or `debug` sets the
log on stderr; `info` reports "N of M assets available" after every change.

## Layout

| Path | Content |
|---|---|
| `src/main.rs` | wiring: identity, level, log, the SDK application |
| `src/directory.rs` | the directory: sans-IO rules of ADR-0029 §3 with unit tests |
| `src/bus.rs` | the `Config`, the raw subscriptions, the conformity check, the worker that feeds the directory, `Get` of what is missing, listing in the SDK; a source scan asserts that no `Set`/`Del`/`Call` is sent |
| `src/api_tests.rs` | the AsyncAPI document against its meta-schema, its channels against the topic forms of the SDK core (`Form::of`) and the collected resources, its operations read-only |
| `api/asyncapi.json` | AsyncAPI 3.1.0: the MQTT interface (Registry services, lifecycle, collecting subscriptions, `Get`s to others); payloads reference the D1 schemas by `$id` |
| `api/meta/` | the AsyncAPI 3.1.0 JSON Schema, vendored from `asyncapi/spec-json-schemas` (commit `eee8d22`, `schemas/3.1.0.json`, Apache-2.0, license file beside it) |
| `registry_test.go`, `internal/regtest/` | Go integration tests (module `github.com/OI4/oec/registry`, standard library only): the registry on the in-process TLS test broker, judged by D8 and D7 |
| `check.sh` | step 8 of `oec/check.sh` |

## Checks

`./check.sh` (offline, no Docker): `cargo fmt --check`, clippy (pedantic,
warnings are errors), `cargo test`, `cargo deny` with the `deny.toml` of
the SDK, the build; then `gofmt`, `go vet` and `go test -race` with
`go-testbroker` of `oec/sdk/rust` and `oec-check`/`oec-infra` of
`oec/conformance` built into a temporary directory (run, not imported:
ADR-0006):

- `TestD8Registry`: `oec-infra bus --tests T8-BUS-03` passes (REQ-REG-001,
  REQ-REG-002); the registry subscribes to `Oi4v2/+/+/Pub/<R>/#` for the
  six collected resources, and its topics and subscriptions on the wire
  are channels of `api/asyncapi.json`.
- `TestD7Level3`: `oec-check test --level 3` – no fail, 49 pinned passes,
  the wire again against `api/asyncapi.json`; T7-GET-04, T7-SET-03 and
  T7-MON-04 treat only the registry itself as an entity (A-388).
- `TestReferenceDesignationPersists`: a `Set` of the ReferenceDesignation
  survives a restart on the same storage.
- `internal/regtest` `TestAPI`: the channel patterns of the AsyncAPI.

`OEC_REGISTRY_D7_REPORT=<file>` keeps the D7 report of the run.
