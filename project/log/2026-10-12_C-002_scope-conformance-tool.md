# C-002 – Scope extension (conformance tool) and first decisions

- Date: 2026-10-12
- Phase: P0
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Guideline baseline: `oi4-oec-documentation` @ `c85ce2a` (unchanged)

## Input from product owner

1. Additional outcome: a test tool, runnable from the command line on Windows and macOS, that later also serves as integration test.
2. JSON Schema 2020-12 → confirmed.
3. MIT license → confirmed.
4. `_ToDelete_` items → ignore, they are dropped in v2.0.

## Execution

- Added deliverable **D7 – Conformance test tool** (`README.md`, `AGENTS.md`, `STATUS.md`).
- Roadmap: offline `validate` with P2; P3 now covers SDK + tool design; new **P5 Conformance test tool**; renumbered Demos → P6, Registry → P7, Docs → P8, Release → P9; demos/registry exit criteria require passing D7; requirement catalog (P1) tags testability (`schema` / `conformance` / `manual`).
- Guardrails: G-2.12 updated (no License/LicenseText), G-4.10 MIT confirmed, new G-4.12 (artifacts must pass D7, verdict based on guideline) and G-4.13 (cross-platform).
- ADRs: 0003 (JSON Schema 2020-12), 0004 (MIT), 0005 (drop `_ToDelete_`: License, LicenseText, RtLicense, Interfaces, SBoM, NE107), 0006 (D7 scope).
- SPEC-FINDINGS SF-007 → decided; OPEN-QUESTIONS Q-003 → decided; new Q-009 (tool language + independence from SDK), Q-010 (test platforms/brokers too?); phase references updated.
- Guideline digest updated (NE107/License removed).

## Rationale

- The tool is placed before demos/registry so that it can serve as their integration test.
- Proposed (not yet decided): the tool should not share protocol logic with the SDK, so it acts as an independent oracle; sharing schemas and test vectors is fine (Q-009).

## Verification

- Cross-references (phase numbers, ADR ids, Q ids) checked by grep.
- No changes in `legacy/` or `oi4-oec-documentation/`.

## Next

- Product-owner review of guardrails/roadmap; then P1 and the P3 discussion (Q-001, Q-009).
