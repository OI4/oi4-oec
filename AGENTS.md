# AGENTS.md – Entry point for humans and AI agents

This workspace builds **OI4 Open Edge Computing (OEC) v2.0** artifacts from scratch:
JSON Schema, developer SDK/library, two demo services (weather, logger), an OEC Registry, a micro SDK in C for microcontrollers (reference ESP32), two cross-platform compliance CLIs (application/messaging D7, infrastructure D8) and a Docusaurus documentation.

**Before doing anything, read in this order:**

1. [`project/README.md`](project/README.md) – how this project is organized and how we work (cycles, logging)
2. [`project/GUARDRAILS.md`](project/GUARDRAILS.md) – non-negotiable rules
3. [`project/STATUS.md`](project/STATUS.md) – where we are right now and what is next
4. [`project/ROADMAP.md`](project/ROADMAP.md) – phases, deliverables, exit criteria
5. [`project/OPEN-QUESTIONS.md`](project/OPEN-QUESTIONS.md) – decisions still pending with the product owner
6. The latest file in [`project/log/`](project/log/) – what happened last and why

Quick facts:

- Source of truth: OEC Messaging Guideline `oi4-oec-documentation/docs/guideline/` ("MG"), OEC Runtime & Infrastructure Guideline `oi4-oec-documentation/docs/infrastructure-guideline/` ("RIG") and the guideline ADRs `oi4-oec-documentation/rfcs_adrs/` ("GL-ADR-0xx") – see ADR-0001, GUARDRAILS G-1. Current baseline: `ff1562f` (branch `feature/spec-findings`, GL-ADR-001 … 020; SF-001 … 074 resolved (SF-040 … 044 in C-019 … C-022, SF-045 … 055 in C-024, SF-057 … 061 in C-026, SF-056 and SF-062 … 068 in C-027 with GL-ADR-019, SF-069 and SF-073 with GL-ADR-020, SF-070/071 figures, SF-072 and SF-074 in the C-027 review); SF-075 … 081 found in C-028 and open (register-only commit `594aaa9`, no re-baseline), next free SF-082; keywords added to the guideline in C-016). The OEC Process Description (`docs/process-description/`) is informative. MG chapters were renumbered by GL-ADR-015 – older ADRs/logs use the old numbers (see ADR-0001 amendment C-013).
- All deployables live in the monorepo folder `oec/` (ADR-0012). The workspace root is a local git repo – commit every change individually, never push (ADR-0009/0012).
- Spec findings (`SF-xxx`) live only in `oi4-oec-documentation/findings/SPEC-FINDINGS.md` (since C-018; next free id there).
- `legacy/` is history only. Do not modify, do not copy code from it (see GUARDRAILS G-1.3).
- Communication with the product owner: German. All project artifacts (logs, plans, code, docs): English.
