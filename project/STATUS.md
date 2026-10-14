# Status

_Last updated: 2026-10-14, cycle C-006_

## Current phase

**P3 – SDK and conformance tool design** – architecture decided: S4 Rust SDK + C API (ADR-0011); local monorepo `oec/` (ADR-0012). Workspace is now a local git repo.

C-006 re-baselined all project artifacts on guideline `5f070bb` (guideline ADRs GL-ADR-001 … 011 implemented; guideline split into MG + RIG). All project decisions remain valid; ADR-0001/0005/0006/0011 amended.

## Next step

1. Product owner answers the new questions **Q-013** (D7 container checks?), **Q-014** (v1 coexistence), **Q-015** (target compliance levels / `no_std`).
2. Product owner confirms the order of the next phases: **P1 spec baseline** (requirements catalog, topic grammar) → **P2 JSON Schema** (+ D7 `validate`), in parallel finishing P3 (Rust/C API sketch, crate choices; D7 = Go per ADR-0013).
3. Then plan P1 in detail (first cycle of P1).

## Deliverables

| ID | Deliverable | State |
|----|-------------|-------|
| D1 | JSON Schema | not started |
| D2 | SDK | not started – S4 Rust SDK + C API decided (ADR-0011); location `oec/sdk/rust` |
| D3 | Weather demo | not started |
| D4 | Logger demo | not started |
| D5 | Registry | not started |
| D6 | Docusaurus docs | not started – new standalone site in `oec/docs` (ADR-0012) |
| D7 | Conformance test tool (CLI) | not started – Go (ADR-0013), apps only, shares only schemas; location `oec/conformance` |

## Baselines

- Guideline: `oi4-oec-documentation` @ `5f070bb` (MG + RIG + GL-ADR-001 … 011); previous `c85ce2a`
- Toolchain seen: node 22.13, npm 11, yarn 4.13, go 1.24.2, Docker 28.4, Python 3.12.9 (macOS arm64)

## Blocking open questions

None blocking yet. Q-013 must be answered before P5, Q-014/Q-015 before P3 closes. Q-011 (publication/CRA) deliberately parked.

## Decided so far

ADR-0001 source of truth · ADR-0002 languages · ADR-0003 JSON Schema 2020-12 · ADR-0004 MIT · ADR-0005 drop `_ToDelete_` · ADR-0006 conformance tool D7 · ADR-0007 PO decides spec interpretations · ADR-0008 registry read-only (step 1) · ADR-0009 GitHub + single commits · ADR-0010 MET Norway weather provider · ADR-0011 SDK = S4 Rust + C API · ADR-0012 local git repo + `oec/` monorepo, standalone docs · ADR-0013 D7 in Go

## Cycle history

| Cycle | Date | Topic | Log |
|-------|------|-------|-----|
| C-001 | 2026-10-12 | Onboarding, guardrails, roadmap | `log/2026-10-12_C-001_onboarding.md` |
| C-002 | 2026-10-12 | Scope: conformance tool D7; decisions schema/license/ToDelete | `log/2026-10-12_C-002_scope-conformance-tool.md` |
| C-003 | 2026-10-12 | Decisions Q-004–Q-010; SDK options paper | `log/2026-10-12_C-003_decisions-sdk-options.md` |
| C-004 | 2026-10-12 | SDK = S4, monorepo `oec/`, local git repo | `log/2026-10-12_C-004_sdk-s4-monorepo.md` |
| C-005 | 2026-10-12 | D7 in Go; OPEN-QUESTIONS split into open/decided | `log/2026-10-12_C-005_d7-go-open-questions.md` |
| C-006 | 2026-10-14 | Re-baseline on guideline `5f070bb` (GL-ADRs, MG/RIG split) | `log/2026-10-14_C-006_guideline-rebaseline.md` |
