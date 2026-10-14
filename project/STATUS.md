# Status

_Last updated: 2026-10-14, cycle C-013_

## Current phase

**P1 – Spec baseline (planning done, awaiting PO confirmation)** – C-012 wrote the P1 plan (`log/2026-10-14_C-012_p1-plan.md`), C-013 re-baselined on guideline `49b6fe2` and updated the plan (`log/2026-10-14_C-013_rebaseline-49b6fe2.md`). P3 leftovers (Rust/C API sketches, crate choices) continue in parallel.

Previous phase: **P3 – SDK and conformance tool design** – architecture decided: S4 Rust SDK + C API (ADR-0011); local monorepo `oec/` (ADR-0012). Workspace is now a local git repo.

C-006 re-baselined all project artifacts on guideline `5f070bb` (guideline ADRs GL-ADR-001 … 011 implemented; guideline split into MG + RIG). All project decisions remain valid; ADR-0001/0005/0006/0011 amended.

C-007 decided Q-013–Q-015: second tool D8 for infrastructure compliance (ADR-0014), v2 only (ADR-0015), level targets + micro SDK D9 (ADR-0016).

C-008 re-baselined on guideline `7bead4c` (PO resolved all spec findings SF-001 … 031; new GL-ADR-012 only `NewDataSetWriterId` as common service, GL-ADR-013 NodeIds as JSON strings). No project decision changed; ADR-0001/0006/0008/0014 amended. No open spec findings at that point (new since C-012: SF-032 … 034).

C-009: micro SDK D9 in C17 with ESP32/ESP-IDF as reference (ADR-0017, Q-016); D8 scope incl. platform checks confirmed (Q-017).

C-010: C vs Rust for the micro SDK re-assessed – C17 confirmed as recommendation, Arduino library added as second port (ADR-0017 amendment). Q-011 parked with working assumption: private GitHub repos, no public availability.

C-011: M5Stack added to the micro SDK (`esp32s3` now supported, M5Stack Core2/CoreS3 as second reference device, M5Stack example firmware; ADR-0017 amendment).

C-012: P1 plan written – catalog `REQ-<AREA>-NNN` as YAML in a new `oec/spec/` folder with per-level obligations, "Level 1 first" area order (C-013 … C-022), ABNF + regex topic grammar, Method × Resource matrix in YAML, curated Appendix B golden vectors. New findings SF-032 (normative keywords), SF-033 (immature chapters/specific services), SF-034 (external "Requirements for Interoperability" doc); new question Q-018 (catalog format/location). **PO was not available – nothing confirmed yet, no catalog content written.**

C-013: re-baselined on guideline `49b6fe2` – PO resolved SF-032 (MG 1.2 normative keywords, BCP 14; keyword-less statements are informative), SF-033 (GL-ADR-014 removes the specific services, GL-ADR-015 moves former MG 6 to the informative OEC Process Description and renumbers MG 7–10 → 6–9) and SF-034 (no external document); GL-ADR-016 removes the empty AAS chapter. GUARDRAILS (new G-1.6 keywords), digest, ROADMAP, ADR-0001 (amendment), AGENTS.md updated; living documents use the new MG numbers, historical ADRs/logs keep the old ones. New findings **SF-035** (protocol behavior without keyword, e.g. Health 60 s heartbeat, Status event after Set/Del) and **SF-036** (closed vs. open set of resources/services). P1 plan: decisions 3–5 settled, area `SVC` and field `maturity` dropped, `ACT` folded into the `MSG` cycle, cycle cut C-014 … C-022. **PO decisions (C-013):** Q-018 → catalog as YAML in `oec/spec/`, shared guideline-derived contract (ADR-0018; ADR-0006/0012/0013 amended, G-4.12); SF-035 → PO adds keywords, until then implicit MUST (G-1.6); SF-036 → closed resource/service set (G-2.1).

## Next step

PO gave the go to start (2026-10-14, after C-011). Order: **P1 spec baseline** → **P2 JSON Schema** (+ D7 `validate`); remaining P3 work (Rust/C API sketches, crate choices) in parallel.

1. **Get the PO's confirmation of the remaining P1 points** (C-013 log "Follow-ups"): ask one question at a time via `ask_user`, with options and a recommendation – (1) overall plan/order/cycle cut C-014 … C-022, (2) golden-example curation rules. Q-018, SF-035 and SF-036 are decided. **Do not write catalog content before the plan is confirmed.**
2. After the answers: ROADMAP P1 update (cycle cut), STATUS. When the PO has updated the guideline for SF-035/036, re-baseline first (new guideline commit).
3. Then **C-014**: `oec/spec/` skeleton (ADR-0018), catalog JSON Schema, lint/render tool, YAML library choice (Go + Rust), normative-sentence inventory of MG + RIG, area `CMP`. Further cycles per the C-012 plan as updated in C-013.

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
| D9 | Micro SDK | not started – C17, ESP-IDF (`esp32`, `esp32s3`) + Arduino, reference ESP32 dev board + M5Stack, Messaging L1 (ADR-0016/0017); location `oec/sdk/micro` |

## Baselines

- Guideline: `oi4-oec-documentation` @ `49b6fe2`, branch `feature/spec-findings` (MG + RIG + GL-ADR-001 … 016, SF-001 … 034 resolved, SF-035/036 decided – guideline update by the PO pending; OEC Process Description informative); previous `7bead4c`, `5f070bb`, `c85ce2a`
- **Container engine:** Docker Desktop 28.4 is installed, but the daemon was **not running** on 2026-10-14. It is not needed for P1/P2, unit tests, host tests or the offline commands of D7/D8. It is **needed from P4 on** for:
  - the TLS dev broker (`oec/dev`, docker compose)
  - integration tests (SDK, D7 live tests, demos, registry)
  - building OCI images with SBOMs
  - D8 checks of running containers
  
  Any Docker-compatible engine works (Docker Desktop, OrbStack, Colima, Podman); CI uses Linux runners with Docker.
- Toolchain seen: node 22.13, npm 11, yarn 4.13, go 1.24.2, Docker 28.4, Python 3.12.9 (macOS arm64)

## Blocking open questions

- The P1 plan confirmation (order/cycle cut) and the golden-example curation rules block the start of the catalog (C-014).
- Q-011 (publication/CRA) parked – working assumption: private GitHub repos only.

## Decided so far

ADR-0001 source of truth · ADR-0002 languages · ADR-0003 JSON Schema 2020-12 · ADR-0004 MIT · ADR-0005 drop `_ToDelete_` · ADR-0006 conformance tool D7 · ADR-0007 PO decides spec interpretations · ADR-0008 registry read-only (step 1) · ADR-0009 GitHub + single commits · ADR-0010 MET Norway weather provider · ADR-0011 SDK = S4 Rust + C API · ADR-0012 local git repo + `oec/` monorepo, standalone docs · ADR-0013 D7 in Go · ADR-0014 two compliance tools D7/D8 · ADR-0015 v2 only · ADR-0016 level targets + micro SDK · ADR-0017 micro SDK in C17, ESP32 reference · ADR-0018 REQ catalog as YAML in `oec/spec/` (shared contract)

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
| C-010 | 2026-10-14 | Micro SDK C vs Rust assessment, Arduino port; Q-011 working assumption | `log/2026-10-14_C-010_micro-sdk-assessment-q011.md` |
| C-011 | 2026-10-14 | M5Stack support in the micro SDK | `log/2026-10-14_C-011_m5stack.md` |
| C-012 | 2026-10-14 | P1 plan (catalog format, area order, topic grammar, matrix, golden examples); SF-032 … 034, Q-018 | `log/2026-10-14_C-012_p1-plan.md` |
| C-013 | 2026-10-14 | Re-baseline on guideline `49b6fe2` (SF-032 … 034 resolved, GL-ADR-014 … 016, MG renumbering); SF-035/036 found and decided; Q-018 → ADR-0018; P1 plan updated | `log/2026-10-14_C-013_rebaseline-49b6fe2.md` |
