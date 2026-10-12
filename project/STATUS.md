# Status

_Last updated: 2026-10-12, cycle C-004_

## Current phase

**P3 – SDK and conformance tool design** – architecture decided: S4 Rust SDK + C API (ADR-0011); local monorepo `oec/` (ADR-0012). Workspace is now a local git repo.

## Next step

1. Product owner confirms the order of the next phases: **P1 spec baseline** (requirements catalog, topic grammar) → **P2 JSON Schema** (+ D7 `validate`), in parallel finishing P3 (Rust/C API sketch, crate choices, D7 language Q-012).
2. Then plan P1 in detail (first cycle of P1).

## Deliverables

| ID | Deliverable | State |
|----|-------------|-------|
| D1 | JSON Schema | not started |
| D2 | SDK | not started – S4 Rust SDK + C API decided (ADR-0011); location `oec/sdk/rust` |
| D3 | Weather demo | not started |
| D4 | Logger demo | not started |
| D5 | Registry | not started |
| D6 | Docusaurus docs | not started – new standalone site in `oec/docs` (ADR-0012) |
| D7 | Conformance test tool (CLI) | not started – scope decided (apps only, shares only schemas); language open (Q-012) |

## Baselines

- Guideline: `oi4-oec-documentation` @ `c85ce2a` (2026-10-06)
- Toolchain seen: node 22.13, npm 11, yarn 4.13, go 1.24.2, Docker 28.4, Python 3.12.9 (macOS arm64)

## Blocking open questions

None for P1. Open: Q-011 (publication/CRA), Q-012 (D7 language, needed before P2 `validate`).

## Decided so far

ADR-0001 source of truth · ADR-0002 languages · ADR-0003 JSON Schema 2020-12 · ADR-0004 MIT · ADR-0005 drop `_ToDelete_` · ADR-0006 conformance tool D7 · ADR-0007 PO decides spec interpretations · ADR-0008 registry read-only (step 1) · ADR-0009 GitHub + single commits · ADR-0010 MET Norway weather provider · ADR-0011 SDK = S4 Rust + C API · ADR-0012 local git repo + `oec/` monorepo, standalone docs

## Cycle history

| Cycle | Date | Topic | Log |
|-------|------|-------|-----|
| C-001 | 2026-10-12 | Onboarding, guardrails, roadmap | `log/2026-10-12_C-001_onboarding.md` |
| C-002 | 2026-10-12 | Scope: conformance tool D7; decisions schema/license/ToDelete | `log/2026-10-12_C-002_scope-conformance-tool.md` |
| C-003 | 2026-10-12 | Decisions Q-004–Q-010; SDK options paper | `log/2026-10-12_C-003_decisions-sdk-options.md` |
| C-004 | 2026-10-12 | SDK = S4, monorepo `oec/`, local git repo | `log/2026-10-12_C-004_sdk-s4-monorepo.md` |
