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

## Amendment C-006 (2026-10-14, guideline baseline `5f070bb`)

The guideline was split (GL-ADR-011) and all guideline ADRs (GL-ADR-001 … 011, `oi4-oec-documentation/rfcs_adrs/`, formerly "MS-0xx" RFCs) were implemented. The source of truth is now:

- **OEC Messaging Guideline** – `oi4-oec-documentation/docs/guideline/` (cited "MG x.y")
- **OEC Runtime & Infrastructure Guideline** – `oi4-oec-documentation/docs/infrastructure-guideline/` (cited "RIG x.y")
- **Guideline ADRs** – `oi4-oec-documentation/rfcs_adrs/ADR-0xx_*.md` (cited "GL-ADR-0xx"); their *Implementation Notes* are binding interpretations.

To avoid confusion with our own project ADRs (`ADR-NNNN`), guideline ADRs are always prefixed `GL-`.

## Amendment C-008 (2026-10-14, guideline baseline `7bead4c`)

- The baseline is now `7bead4c` on branch `feature/spec-findings` of `oi4-oec-documentation`. It adds **GL-ADR-012** (only the common service `NewDataSetWriterId` remains) and **GL-ADR-013** (NodeIds as JSON strings, OPC UA V1.05), and resolves all spec findings SF-001 … 031.
- The guideline repo now has its own copy of our findings register, `findings/SPEC-FINDINGS.md`, with the same SF ids. Our `project/SPEC-FINDINGS.md` remains the place where new findings are first recorded.
- Until the branch is merged, we track the branch commit. When it is merged, the merge commit on the main branch becomes the baseline; the content is identical.

## Amendment C-013 (2026-10-14, guideline baseline `49b6fe2`)

- The baseline is now `49b6fe2` on branch `feature/spec-findings`. It resolves SF-032 … 034 and adds:
  - **MG 1.2 "Normative Keywords"** (BCP 14 / RFC 2119 + RFC 8174; RIG 1.1 applies it to the RIG): requirement strength is taken from the keyword, statements without keyword are informative (see G-1.6 and SF-035).
  - **GL-ADR-014** – the specific services (Read, Write, Subscribe, Unsubscribe, GenericMethod) are removed.
  - **GL-ADR-015** – the former MG chapter 6 (process description) moved to the separate **OEC Process Description** (`docs/process-description/`); MG chapters 7–11 were renumbered to 6–10.
  - **GL-ADR-016** – the empty AAS mapping chapter is removed.
- The **OEC Process Description is informative**. It is not part of the normative baseline; no requirements are derived from it.
- The external document "Requirements for Interoperability" is no longer referenced (SF-034). The normative baseline is MG + RIG + GL-ADRs only.
- **Section numbers:** ADRs, logs and spec findings written before C-013 cite the old MG numbering (old MG n = new MG n−1 for n = 7…10). They are not rewritten; living documents (GUARDRAILS, STATUS, ROADMAP, digest) use the current numbering.

## Amendment C-015 (2026-10-15, guideline baseline `5cb109e`)

- The baseline is now `5cb109e` on branch `feature/spec-findings`. The PO had the agent implement the decided findings directly in the guideline repo (local commits, never pushed):
  - `609cb4a` – **SF-038**: the OEC Messaging Compliance and the OEC Service Container Compliance are **independent** domains (MG 1.3, RIG 1.2, RIG 5.2, glossaries, GL-ADR-009 note). A microcontroller can reach Messaging Level 3 without any Service Container level; a container can reach Service Container Level 3 with Messaging Level 1.
  - `e0b1676` – **SF-037**: Messaging **Level 3 = all methods, resources and common services**; "Resources Without Content" (empty list on `Get`, no unsolicited `Pub`, `Event` Status `Bad` on `Set`/`Del` of unknown objects); every `Set`/`Del` is answered by `Event` Status (MG 7.1.4, 7.2.x, 9.1, GL-ADR-009 note).
  - `0106c5f` – **SF-036**: the set of resources and common services is closed (MG 7.1.5, GL-ADR-014 note).
  - `5cb109e` – the guideline copy `findings/SPEC-FINDINGS.md` is synced with our register.
- Changes the agent makes in the guideline repo on behalf of the PO are part of the baseline like any other guideline commit. Each one references its SF id; the PO reviews them before the branch is merged.

## Amendment C-016 (2026-10-15, guideline baseline `de852d6`)

- The baseline is now `de852d6` on branch `feature/spec-findings`. Again implemented by the agent on PO request (local commits):
  - `a6147e7` – SF-037 amendment (`Event` `Status` answer to `Set`/`Del`: must from Level 2 on, should at Level 1) and **SF-039** (`Get` of `Metadata` without `Data`: no `Metadata`, may answer with `Event` `Status`/`Bad`).
  - `0de223c`, `f830382` – **SF-035**: keywords added to all keyword-less protocol statements of the normative-sentence inventory (and `ReplyTo` in MG 8.2.1). The interim rule "keyword-less behavior = implicit MUST" is no longer needed for the guideline text; keyword-less behavior found later is a new spec finding.
  - `edf76e1`, `de852d6` – findings register sync. No open findings.

## Amendment C-017 (2026-10-15, guideline baseline `bc5d063`)

- The baseline is now `bc5d063` on branch `feature/spec-findings`. Implemented by the agent on PO request (local commits):
  - `f5d69d8` – **GL-ADR-017** "Adapt minimum set of topic elements to compliance levels": MG 7.2 (7.2.1 For Applications, 7.2.2 For Devices) removed. The requirements per level follow from MG 1.3; the rule for directly connected and represented entities is a new section of MG 1.3; "Resources Without Content" moved unchanged to MG 9.1. No requirement of a level changed.
  - `6e1b5c3` – findings register sync.
  - `7677161` – GL-ADR-017 extended (PO request): chapter MG 7 flattened – the content of former 7.1 Topic Namespace Elements is the section "Topic Elements" of MG 7; the topic elements MG 7.1.1 … 7.1.7 are now **MG 7.1 … 7.7** (7.1.5.1/7.1.5.2 → 7.5.1/7.5.2). Note: "MG 7.2" now denotes the ServiceType Element.
  - `9489d5e` – findings register sync.
  - `4d2f919` – MG 7 introduction: generic MQTT topic rules replaced by a link to MQTT v3.1.1 section 4.7 (OASIS), only OEC-specific rules kept; ADR-017 implementation note.
  - `4f5c548` – findings register sync.
  - `10491df` – MG 7 MQTT introduction removed entirely (topic fully defined by schema and 7.1 … 7.7); MQTT link moved to "Topic Elements"; white-space recommendation dropped.
  - `2d6dc59` – findings register sync.
  - `d7d8750` – PO: heading "Topic Elements" removed from the MG 7 index.
  - `edf9305` – dangling `#topic-elements` anchor in MG 1.3 fixed; ADR-017 aligned.
  - `2452494` – findings register sync.
  - `cb50c04` – PO: "White space is allowed, but should not be used." (same keyword as the former MG 7 intro) moved to MG 7.7 `<Topic encoded String>`.
  - `bc5d063` – findings register sync.
- References to MG 7.x in project artifacts written before C-017 (logs, findings, ADRs) use the old numbers and are historical; the REQ catalog, GUARDRAILS, digest, ROADMAP and STATUS were renumbered (C-017).

