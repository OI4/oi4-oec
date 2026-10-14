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

## Amendment C-007 (ADR-0014, ADR-0015)

- Service Container and platform checks are done by a **separate tool D8** (ADR-0014). D7 remains limited to Messaging compliance of applications and smart devices.
- D7 handles `Oi4v2` only and ignores v1 traffic (ADR-0015).

## Amendment C-008 (guideline baseline `7bead4c`)

SF-025 is resolved (MG 1.3):
- The compliance level is **declared by the manufacturer** for each compliance domain. There is no Message Bus message that declares it.
- Tests are grouped by level. The declared level is a parameter of the test run, and passing a level includes passing all lower levels.

D7 therefore takes the declared level as the CLI option `--level 1|2|3`; this is now normative, not a workaround. `level3` includes the `level1` and `level2` tests.

The active tests no longer cover FileUpload, FileDownload, FirmwareUpdate or Blink, because GL-ADR-012 removed them. The only common-service Action that D7 tests is `NewDataSetWriterId`.

## Amendment C-013 (ADR-0018)

- The independence rule is extended: D7 (and D8) share the D1 JSON Schemas **and the spec baseline `oec/spec/`** (REQ catalog, topic grammar, Method × Resource matrix, guideline-derived test vectors) with the SDKs. Both are derived only from the guideline, never from SDK code. Sharing code stays forbidden.
- D7 test cases reference REQ ids from `oec/spec/requirements/`; D7 may keep additional own fixtures, but the golden vectors in `oec/spec/vectors/` are the common positive baseline.
