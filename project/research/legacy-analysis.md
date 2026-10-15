# Legacy analysis (history only – see GUARDRAILS G-1.3)

Analyzed in C-001. Revisions: `oi4-oec-service` `2d6e6f2`, `oi4-oec-service-go` `9b96844`, `oi4-oec-demo` `d686c84`, `oi4-oec-registry` `fb77c3e`. All implement guideline **1.x** (Oi4Identifier with dnp-encoded multi-level topics) – incompatible with 2.0 (GL-ADR-010, formerly MS-010; additionally GL-ADR-003 … 007 change namespace, payload keys and services).

## oi4-oec-service (TypeScript, Lerna monorepo)

- Packages: `oi4-oec-service-node` (application runtime), `-model` (types, enums, topic parser), `-json-schemas` (≈48 draft-07 schemas, Ajv), `-conformity-validator`, `-logger` (Winston + Syslog events), `-asyncapi`, `-dnp-encoding`.
- API: `OI4ApplicationFactory` / builder; MAM read from `/etc/oi4/config/mam.json`; async-mqtt client; resources held in a central mutable object.
- Problems: topic parser based on segment counts (fragile), metadata stubbed, config handling partial, pagination ad hoc, tight coupling between transport and resource handling, many `any`s, outdated dependencies.
- Ideas worth keeping (re-implement, not copy): typed topic model; lifecycle hooks; separate schema package consumable by other tools; conformity validator as a reusable library; logger that maps to Syslog events.

## oi4-oec-service-go (Go)

- Packages split by concern (api, application, broker, dnp, ...); paho.mqtt, zap logging, functional options pattern.
- Problems: data races (shared maps without locks), pagination not implemented, Get handling incomplete, `TODO.md` lists unresolved spec questions (many of which are still open in 2.0 → cross-check with the spec-findings register `oi4-oec-documentation/findings/SPEC-FINDINGS.md`).
- Ideas worth keeping: functional options; clean package boundaries; small static binary is attractive for edge.

## oi4-oec-demo

- Weather demo used OpenWeather API **with API key** → the product owner wants a demo without such a subscription (→ keyless source + simulation, G-4.7).
- PV/logger demos show consumer side but are tightly bound to legacy SDK internals.

## oi4-oec-registry

- Express backend + React 18 / MUI frontend; REST under `/registry/*`, Swagger, conformity endpoints, config via OI4 Config resource, file logging.
- Problems: too many responsibilities in one process/class, inconsistent topic casing, polling UI, legacy identifier model.
- Ideas worth keeping: application/device distinction with "published by" relation; conformity check per participant; audit trail of events; health history.

## Lessons learned for v2.0

1. Contract first (schema + test vectors) before SDK code.
2. Topic handling must be grammar-based (per method × resource), not count-based.
3. Clear separation: transport ↔ protocol codec ↔ resource model ↔ application logic.
4. Concurrency-safe state; explicit lifecycle state machine.
5. Pagination and `MaxPacketSize` must be first-class, not an afterthought.
6. Live updates (push) for the registry UI instead of polling.
