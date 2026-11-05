# C-109 – P9 increment 6: versioning and the release workflow

- Date: 2026-11-25
- Phase: P9 – Hardening & release, increment 6 (ADR-0032 §9, §10)
- Baseline: guideline `52d8470` (branch `feature/spec-findings`, head `40204d7` before the cycle: the Community index of C-108 addendum 5 after `064aa39`, no guideline chapter changed); SF-001 … 117 resolved, next SF-118
- Workspace before the cycle: `c068234` (local; `origin/main` = `42f6784`; the CI is paused, C-105 D-2 (b))

## Goal

Increment 6 as handed over: one versioning scheme (SemVer) for every deliverable – Rust crates, Go modules with tag prefixes per module path, micro SDK, images, D7/D8, the schema with its own version – with its reasons (A-x) and, where a first public version number is a promise to the outside, a decision for the PO (D-x); a generated workflow `release.yml` (test first in `oec/ci/pipelines`, logic in scripts under `oec/ci/`), started by a tag or by hand, that builds every deliverable from one commit with the existing scripts (`binaries.sh`, `ffi-lib.sh`, `arduino/package.sh`, `sbom.sh`, the image `build.sh` as OCI archives), reproducibly where possible, with `SHA256SUMS`, the SBOMs and release notes – **as workflow artifacts only**: no registry push, no GitHub Release, no crates.io, Go proxy or ESP registry (Q-011, C-104 D-2).

## Plan

1. **Versioning (new ADR-0033):** SemVer 2.0.0; one release version for all code deliverables in lockstep (release train) with a single source `oec/VERSION`, enforced by a gate test over every manifest and constant that states it; the schema bundle (`2.MINOR.PATCH`, ADR-0019 §6), the report formats of D7/D8 and the micro SDK API version keep their own versions; tags for the train and for the Go modules (prefix = module directory) in the release repository `OI4/oec` (C-104 D-3 (a)); D7/D8 report the release version (`-X main.version`). First public version number and the Go module of the schema → D-x.
2. **Model extension of `oec/ci/gha`** (tests first): tag trigger (`on.push.tags`) and job dependencies (`needs:`) – the policy keeps `needs.` out of expressions; `actions/download-artifact` pinned by SHA.
3. **`oec/ci/release.sh <dir> [part…]`** (logic) and **`release.yml`** (test `TestRelease` first): parts `source` (`git archive` of `oec/`), `binaries` (D7/D8), `ffi` (prebuilt C ABI library per target on its native runner), `micro` (Arduino library ZIP), `schema` (bundle), `sbom`, `images` (OCI archives, judged by D8), `assemble` (deterministic archives, release notes from `oec/CHANGELOG.md` and the schema changelog, one `SHA256SUMS`); the release job collects the parts into one artifact. No cache in release jobs.
4. **Local verification:** `lint.sh`; `release.sh` twice on the Mac (same checksums); in a fresh Ubuntu 24.04 container (image of `versions.sh`, shellcheck); the images part in a container with an engine of its own (never on the host engine: the tags `oi4/oec-*:0.1.0` belong to the `oec/dev` stack of another session); `linux.sh`; the gate. Hosted-only items open for C-111.
5. **Docs:** ADR-0033, ADR-0032 amendment C-109, `oec/ci/README.md`, `oec/README.md`, Community pages of changed tools, ROADMAP, STATUS, REVIEW/REVIEW_DE (A-602 ff., D-1 ff.).
