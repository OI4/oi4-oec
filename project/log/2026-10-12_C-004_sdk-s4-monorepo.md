# C-004 – SDK architecture S4, local monorepo

- Date: 2026-10-12
- Phase: P3 (design decisions)
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Guideline baseline: `oi4-oec-documentation` @ `c85ce2a` (unchanged)

## Input from product owner

- Q-001: "Start with S4" → ADR-0011 (Rust SDK + C API, layered core → runtime → front ends; further languages later via spike S9 vs S1/S2).
- Q-002: this repo exists only locally and stays local; parts may be published later; introduce a sensible subfolder holding all deployables (monorepo).
- Asked: git for the workspace? → PO chose local git repo at the workspace root, `legacy/` and `oi4-oec-documentation/` git-ignored.
- Asked: where do the new docs live? → new standalone docs **without links** to the guideline site; publication must be reconsidered because of the EU Cyber Resilience Act.

## Execution

- ADR-0011 (SDK = S4), ADR-0012 (local repo, `oec/` monorepo, standalone docs, publication after CRA review); ADR-0009 amended.
- OPEN-QUESTIONS: Q-001, Q-002 decided; Q-007 amended; new Q-011 (publication/CRA), Q-012 (D7 language).
- GUARDRAILS G-3.8 (local repo, single commits, no push without approval), G-4.11 (standalone docs); ROADMAP P3/P8/P9; README; AGENTS.md; STATUS.
- Created `oec/README.md` (layout: `schema/`, `sdk/rust/`, `conformance/`, `demos/weather/`, `demos/logger/`, `registry/`, `docs/`, `dev/`) and `oec/LICENSE` (MIT).
- `git init` (branch `main`, no remote) and commits:
  1. `.gitignore` (repo setup)
  2. governance baseline (AGENTS.md, prompt.txt, project/ incl. C-001…C-003 logs and ADR-0001…0012)
  3. `oec/` skeleton
  4. this log + STATUS

## Rationale

- Folder name `oec/`: short, matches the product name, works as future publication root (`git subtree split --prefix oec`).
- Subfolders are created only when their phase starts (no empty placeholders).

## Verification

- `git status` clean after commit 4; nested repos not tracked; no changes in `legacy/` or `oi4-oec-documentation/`.

## Next

- PO confirms next phase order (P1 → P2, P3 remainder in parallel); then detailed planning of P1.
