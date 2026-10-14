# Status

_Last updated: 2026-10-14, cycle C-009_

## Current phase

**P3 – SDK and conformance tool design** – architecture decided: S4 Rust SDK + C API (ADR-0011); local monorepo `oec/` (ADR-0012). Workspace is now a local git repo.

C-006 re-baselined all project artifacts on guideline `5f070bb` (guideline ADRs GL-ADR-001 … 011 implemented; guideline split into MG + RIG). All project decisions remain valid; ADR-0001/0005/0006/0011 amended.

C-007 decided Q-013–Q-015: second tool D8 for infrastructure compliance (ADR-0014), v2 only (ADR-0015), level targets + micro SDK D9 (ADR-0016).

C-008 re-baselined on guideline `7bead4c` (PO resolved all spec findings SF-001 … 031; new GL-ADR-012 only `NewDataSetWriterId` as common service, GL-ADR-013 NodeIds as JSON strings). No project decision changed; ADR-0001/0006/0008/0014 amended. **No open spec findings.**

C-009: micro SDK D9 in C17 with ESP32/ESP-IDF as reference (ADR-0017, Q-016); D8 scope incl. platform checks confirmed (Q-017).

## Next step

1. Product owner confirms the order of the next phases: **P1 spec baseline** (requirements catalog, topic grammar) → **P2 JSON Schema** (+ D7 `validate`), in parallel finishing P3 (Rust/C API sketch, crate choices; D7 = Go per ADR-0013).
2. Then plan P1 in detail (first cycle of P1).

## Deliverables

| ID | Deliverable | State |
|----|-------------|-------|
| D1 | JSON Schema | not started |
| D2 | SDK | not started – S4 Rust SDK + C API decided (ADR-0011); location `oec/sdk/rust` |
| D3 | Weather demo | not started – target Messaging L3 (ADR-0016) |
| D4 | Logger demo | not started – target Messaging L1 (ADR-0016) |
| D5 | Registry | not started – target L3, v2 only |
| D6 | Docusaurus docs | not started – new standalone site in `oec/docs` (ADR-0012) |
| D7 | Application compliance tool (CLI) | not started – Go (ADR-0013), Messaging L1–L3 of apps/devices, shares only schemas; location `oec/conformance` |
| D8 | Infrastructure compliance tool (CLI) | not started – Go, Service Container + platform (ADR-0014); location `oec/conformance` |
| D9 | Micro SDK | not started – C17, reference ESP32/ESP-IDF, Messaging L1 (ADR-0016/0017); location `oec/sdk/micro` |

## Baselines

- Guideline: `oi4-oec-documentation` @ `7bead4c`, branch `feature/spec-findings` (MG + RIG + GL-ADR-001 … 013, SF-001 … 031 resolved); previous `5f070bb`, `c85ce2a`
- Toolchain seen: node 22.13, npm 11, yarn 4.13, go 1.24.2, Docker 28.4, Python 3.12.9 (macOS arm64)

## Blocking open questions

None. Q-011 (publication/CRA) deliberately parked.

## Decided so far

ADR-0001 source of truth · ADR-0002 languages · ADR-0003 JSON Schema 2020-12 · ADR-0004 MIT · ADR-0005 drop `_ToDelete_` · ADR-0006 conformance tool D7 · ADR-0007 PO decides spec interpretations · ADR-0008 registry read-only (step 1) · ADR-0009 GitHub + single commits · ADR-0010 MET Norway weather provider · ADR-0011 SDK = S4 Rust + C API · ADR-0012 local git repo + `oec/` monorepo, standalone docs · ADR-0013 D7 in Go · ADR-0014 two compliance tools D7/D8 · ADR-0015 v2 only · ADR-0016 level targets + micro SDK · ADR-0017 micro SDK in C17, ESP32 reference

## Cycle history

| Cycle | Date | Topic | Log |
|-------|------|-------|-----|
| C-001 | 2026-10-12 | Onboarding, guardrails, roadmap | `log/2026-10-12_C-001_onboarding.md` |
| C-002 | 2026-10-12 | Scope: conformance tool D7; decisions schema/license/ToDelete | `log/2026-10-12_C-002_scope-conformance-tool.md` |
| C-003 | 2026-10-12 | Decisions Q-004–Q-010; SDK options paper | `log/2026-10-12_C-003_decisions-sdk-options.md` |
| C-004 | 2026-10-12 | SDK = S4, monorepo `oec/`, local git repo | `log/2026-10-12_C-004_sdk-s4-monorepo.md` |
| C-005 | 2026-10-12 | D7 in Go; OPEN-QUESTIONS split into open/decided | `log/2026-10-12_C-005_d7-go-open-questions.md` |
| C-006 | 2026-10-14 | Re-baseline on guideline `5f070bb` (GL-ADRs, MG/RIG split) | `log/2026-10-14_C-006_guideline-rebaseline.md` |
| C-007 | 2026-10-14 | Decisions Q-013–Q-015: D8, v2 only, level targets, micro SDK | `log/2026-10-14_C-007_compliance-tools-levels.md` |
| C-008 | 2026-10-14 | Re-baseline on guideline `7bead4c` (all SF resolved, GL-ADR-012/013) | `log/2026-10-14_C-008_rebaseline-findings-resolved.md` |
| C-009 | 2026-10-14 | Micro SDK in C / ESP32 (Q-016); D8 scope confirmed (Q-017) | `log/2026-10-14_C-009_micro-sdk-c-esp32.md` |
