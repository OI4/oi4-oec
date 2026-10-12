# C-003 – Decisions on open questions; SDK options paper

- Date: 2026-10-12
- Phase: P0 → P3 (discussion)
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Guideline baseline: `oi4-oec-documentation` @ `c85ce2a` (unchanged)

## Input from product owner

- Q-004: PO decides spec interpretations → ADR-0007.
- Q-005: "Open-Meteo if our use is non-commercial". Agent checked Open-Meteo terms (2026-10-12): free API is non-commercial only; "integrating into commercial products or promotional activities" is commercial → grey zone for an alliance demo. Asked PO with alternatives; PO chose **MET Norway as default behind a provider interface + offline simulation** → ADR-0010.
- Q-006: Registry step 1 read-only → ADR-0008.
- Q-007: Publish everything on GitHub once the other points are settled (Q-001/Q-002); afterwards each change committed individually → ADR-0009, G-3.8.
- Q-008: PO asked whether ARMv7 has disadvantages → answered in `research/sdk-options.md` §4 (key fact: Node.js 24+ ships no official linux-armv7l builds, checked against nodejs.org/dist/index.json). Remains open as input for Q-001.
- Q-009: Only the schemas are shared between D7 and SDK, so D7 really tests the code → ADR-0006 amendment, G-4.12.
- Q-010: D7 tests applications only → ADR-0006 amendment, roadmap P5.

## Execution

- Updated `OPEN-QUESTIONS.md`, `ROADMAP.md` (P3, P5, P6, P7), `GUARDRAILS.md` (G-3.4, G-3.8, G-4.12), `STATUS.md`, ADR-0006; new ADR-0007 … ADR-0010.
- Wrote `research/sdk-options.md`: complexity the SDK must hide, form options (library / sidecar / codegen), language comparison (TS, Go, .NET, Python), ARMv7 analysis, constellations K1–K4, repo layout options R1–R3, questions for PO.
- Facts checked online: Open-Meteo terms, MET Norway terms (User-Agent, caching, CC BY 4.0), Node.js release platforms, current Go (1.27.2) and .NET (10 LTS) versions, GitHub org of existing repos (`OI4`).

## Q-001 discussion (ongoing)

- Q-008 decided: ARMv7 best effort (does not exclude a language).
- Audience: industrial developers, C-heavy, Rust gaining popularity → added option K5 (Rust core + C API), `sdk-options.md` §8.
- PO: Rust is favorite; Go and TypeScript must not be excluded; asked whether ONE SDK can serve C, C#, Go, TypeScript → analysis `sdk-options.md` §9: sans-IO Rust core + Rust runtime + stable C ABI; per-language paths (P/Invoke, napi-rs/WASM, wazero-WASM/cgo); recommendation: one SDK, staged languages.

- PO chose "discuss further" and requested a comparison document with a direct comparison table focusing on (1) stability & performance, (2) acceptance among edge developers, (3) ease of use, (4) maintenance effort → `research/sdk-architecture-comparison.md` (solutions S1–S8, recommendation: start S4, grow into S1/S2 hybrid).

- PO proposed: Rust core communicating with language SDK wrappers via local HTTP. Evaluated as **S9** in `research/sdk-architecture-comparison.md`: feasible; must run as co-process in the *same* container (identity model); needs push channel (gRPC streaming or WebSocket/SSE); key issue = liveness/Will semantics (core must detect dead app); rating +/o/+/+; strongest point maintenance; C keeps C API. Recommendation updated: S4 first, then S9 or S1/S2 for further languages, decided by spike.

## Next

- PO reviews `research/sdk-architecture-comparison.md`; decide Q-001, then Q-002 (repo layout); then publish on GitHub (ADR-0009).
