# ADR-0013 – Conformance test tool (D7) is written in Go

- Status: accepted
- Date: 2026-10-12
- Deciders: product owner (Thomas), C-005
- Related: Q-012, ADR-0006, ADR-0011, G-4.13

## Decision

D7 is implemented in **Go** in `oec/conformance/` as its own Go module.

## Rationale

- A different language from the Rust SDK makes independence natural (ADR-0006): only the D1 JSON Schema files are shared, no code.
- Go cross-compiles trivially to single static executables for Windows, macOS and Linux (amd64/arm64) from one CI runner (G-4.13).

## Consequences

- Two toolchains in the monorepo (Rust for SDK/demos/registry backend, Go for D7).
- D7 needs a JSON Schema 2020-12 validator in Go (candidate: `santhosh-tekuri/jsonschema` v6) and an MQTT 3.1.1 client with TLS (candidate: `eclipse/paho.mqtt.golang`) – final choice in P2/P5 (G-4.4).
- The schema files are consumed from `oec/schema/` (e.g. embedded at build time via `go:embed` from a copied/generated directory); the mechanism is decided in P2.

## Amendment C-007 (ADR-0014)

The infrastructure compliance tool **D8** is also written in Go. It lives in the same Go module `oec/conformance/` as a separate binary and may share internal packages (reporting, test catalog) with D7.
