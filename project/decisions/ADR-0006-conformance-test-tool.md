# ADR-0006 – Conformance test tool as deliverable D7

- Status: accepted (scope C-002, independence + scope C-003); language pending (Q-001)
- Date: 2026-10-12
- Deciders: product owner (Thomas), C-002

## Context

The product owner wants an additional outcome: a test tool that runs from the command line on Windows and macOS and later also serves as integration test.

## Decision

- New deliverable **D7 – Conformance test tool** (CLI) with three modes: `validate` (offline messages), `monitor` (passive live validation), `test` (active test of one application under test).
- Roadmap: offline `validate` ships with the JSON Schema (P2); design together with the SDK in P3; implementation in new phase **P5**; demos (P6) and registry (P7) must pass it. Phases after P4 were renumbered (Demos P6, Registry P7, Docs P8, Release P9).
- Test cases reference requirement IDs (REQ-…) from P1; machine-readable reports (JSON, JUnit XML) and exit codes for CI.
- Guardrails G-4.12 (artifacts must pass D7) and G-4.13 (cross-platform) added.

## Amendment C-003 (Q-009, Q-010)

- **Independence:** D7 shares only the D1 JSON Schemas with the SDK. No shared code (no topic/IL/codec libraries from the SDK), so D7 genuinely tests SDK-based code. D7 maintains its own fixtures.
- **Scope:** D7 tests **applications only** (no broker, platform or container storage checks). The application under test must still be reachable via a broker that D7 connects to.

## Open

- Language/runtime (part of Q-001).

## Amendment C-006 (guideline baseline `5f070bb`)

The guideline now defines compliance **domains** (Messaging, Service Container) and **levels** 1–3 (MG 1.3, 8.2, RIG 1.2; GL-ADR-009):

- D7 tests the **OEC Messaging Compliance** of a component under test – applications *and* smart devices (which are treated as applications, MG 8.2) – **per level**: test profiles `level1`, `level2`, `level3` instead of `minimum`/`full`. The level is a CLI option because the guideline does not yet define how a level is declared or discovered (SF-025).
- Whether D7 also checks the **Service Container Compliance** of an application container (OCI image, SBOM in `/opt/oi4/licenses`, storage conventions) is open: Q-013. Platform/broker checks remain out of scope (Q-010).
- D7 supports namespace `Oi4v2` only; see Q-014 for `Oi4` (v1) traffic.
