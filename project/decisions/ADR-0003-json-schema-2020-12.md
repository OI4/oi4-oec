# ADR-0003 – JSON Schema draft 2020-12

- Status: accepted
- Date: 2026-10-12
- Deciders: product owner (Thomas), C-002

## Context

D1 must validate all OEC messages as precisely as possible, based on the latest schema specification. Legacy schemas used draft-07.

## Decision

All schemas use JSON Schema **draft 2020-12** (`"$schema": "https://json-schema.org/draft/2020-12/schema"`).

## Consequences

- Allows `$defs`, `unevaluatedProperties`, `prefixItems`, `$dynamicRef` for strict, modular schemas.
- Validators must support 2020-12 (e.g. Ajv 2020, santhosh-tekuri/jsonschema for Go, JsonSchema.Net, python-jsonschema); checked in P2/P3.
- Schema architecture details (modules, `$id` scheme, topic-aware validation) follow in a P2 ADR → ADR-0019 (C-033, accepted; validator pair santhosh-tekuri/jsonschema + jsonschema-rs, checked in the PoC).
