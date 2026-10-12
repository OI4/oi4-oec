# ADR-0001 – Authoritative guideline source for v2.0

- Status: accepted
- Date: 2026-10-12
- Deciders: product owner (Thomas), confirmed in C-001

## Context

Two copies of the OEC Development Guideline exist in the workspace:

1. `oi4-oec-documentation/guideline/Members Space/.../OEC - Development Guideline.md` – Confluence export (plus PDFs v1.1.0).
2. `oi4-oec-documentation/docs/guideline/` – the Docusaurus version imported from the Confluence export (`run_import2.js`) and edited since then. It already contains RFC MS-010 (Identification Link replaces Oi4Identifier, base64url-encoded single topic level) – commit `c85ce2a` (2026-10-06).

## Decision

`oi4-oec-documentation/docs/guideline/` (Docusaurus) together with accepted RFCs in `oi4-oec-documentation/rfcs_adrs/` is the single source of truth for v2.0. The Confluence export and PDFs are historical references only.

## Consequences

- All requirements, schemas and code reference sections of the Docusaurus guideline.
- The guideline revision (git commit) used is recorded in each cycle log; changes are diffed against the recorded baseline.
- Contradictions inside the Docusaurus guideline are handled via `SPEC-FINDINGS.md`.

## Alternatives considered

- Confluence export as source – rejected: older, lacks MS-010.
- Both sources with case-by-case decision – rejected: unnecessary ambiguity.
