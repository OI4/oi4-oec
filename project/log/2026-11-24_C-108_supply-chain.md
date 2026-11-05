# C-108 – P9 increment 5: supply chain (SBOM, license reports, vulnerability checks, `SECURITY.md`)

- Date: 2026-11-24
- Phase: P9 – Hardening & release, increment 5 (ADR-0032 §10)
- Baseline: guideline `52d8470` (branch `feature/spec-findings`, head `989eff5` before the cycle, unchanged chapters); SF-001 … 117 resolved, next SF-118
- Workspace before the cycle: `efbc825` (local; `origin/main` = `42f6784`; the CI is paused, C-105 D-2 (b))

## Goal

Increment 5 as handed over: an SBOM and a license report for every deliverable (Rust SDK and C ABI `oec-ffi`, Go SDK, micro SDK, the D7/D8 binaries, the three demos, the registry with its Web UI, the schema), with the existing tools (`oec-sbom`, `cargo deny`), generated in a workflow (test first in `oec/ci/pipelines`, logic in scripts under `oec/ci/`, at most a workflow artifact); the vulnerability checks `govulncheck` (Go modules), `cargo deny check advisories` (Rust) and `npm audit` (registry Web UI) with every finding handled, network opt-in only (not in the gate); `SECURITY.md` and a draft of the vulnerability handling (reporting channel, time frames, support period as a proposal) as CRA input – contacts and commitments are the PO's (Q-011): placeholders and D-x.

## Plan

1. **Baseline and first scans (network, opt-in):** `govulncheck` on every Go module, `cargo deny check advisories` on every `Cargo.lock`, `npm audit` of `oec/registry/ui`; handle each finding (update within the dependency rules or a justified exception).
2. **`oec-sbom` for deliverables without an image** (ADR-0028 amendment C-108, additive, tests first): `-base`/`-debian` optional (only images have a base), the Go modules of a binary or library from `go list -deps` without building it (`-golist`, the license map of C-107 for third-party modules, fail closed), the Go standard library of a binary (`-gostd`), a license report in Markdown (`-report`). The image SBOMs stay unchanged.
3. **`oec/ci/sbom.sh <dir>`:** one SPDX 2.3 SBOM, the license texts and the license report per deliverable; **`oec/ci/audit.sh <dir>`:** the three vulnerability checks, all run, the job fails on any finding; `govulncheck` as a tool pinned in `versions.sh` (`go run …@version`, no module dependency – like actionlint).
4. **Workflow `supply-chain.yml`** (test first, `TestSupplyChain`): job `sbom` (artifact with the SBOMs and reports, 14 days, no release) and job `audit` (artifact with the outputs).
5. **`SECURITY.md`** (`oec/SECURITY.md`, travels with the release repository `OI4/oec`; a pointer at the workspace root) and the draft `project/research/vulnerability-handling.md` (intake, triage, time frames, coordinated disclosure, CRA reporting duties, support period – proposals with placeholders) → D-x for the PO.
6. **Local verification:** `lint.sh`, `sbom.sh` and `audit.sh` on the Mac, both in a fresh Ubuntu 24.04 container (image of `versions.sh`), `linux.sh`, the gate. The hosted runs stay open for C-111 (C-105 D-2 (b)).
7. **Docs:** ADR-0032/0028 amendments C-108, `oec/ci/README.md`, `oec/README.md`, Community pages where a tool changes, ROADMAP, STATUS, REVIEW/REVIEW_DE (A-595 ff., D-1 ff.).
