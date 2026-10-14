# C-006 – Re-baseline on guideline `5f070bb`

- Date: 2026-10-14
- Phase: P3 (cross-cutting)
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Guideline baseline: `oi4-oec-documentation` `c85ce2a` → **`5f070bb`**

## Input from product owner

The guideline changed: all ADRs from `rfcs_adrs` were incorporated. The PO asked us to check all our decisions, questions etc. and adapt them.

## Analysis

- Read all 11 guideline ADRs (`rfcs_adrs/ADR-001 … 011`), including their implementation notes.
- Built a fact extract of the new MG and RIG.
- Re-checked every spec finding (SF-001 … 022) against the new text.
  - An explore sub-agent did a first pass, but its results were partly wrong (e.g. it reported fixed items as still open and vice versa).
  - **Every SF status was therefore verified manually with grep before recording.** Lesson for other agents: never record agent findings without spot checks.
- Main changes in the guideline (details in `research/guideline-digest.md` §0):
  - **Split:** Messaging Guideline (MG) and Runtime & Infrastructure Guideline (RIG); chapters renumbered.
  - **Namespace:** `Oi4v2`; v1 `Oi4` may coexist.
  - **Payload keys:** DataSetMessage uses `DataSetWriterName`/`WriterGroupName`; NetworkMessage uses `ReplyTo` (replaces `CorrelationId`).
  - **OPC UA 1.05.04:** fixed content masks, `Status` object, `MinorVersion` xor `MetaDataVersion`.
  - **Services:** `MSG` replaced by OPC UA Actions (`ua-action-request`/`ua-action-response`).
  - **Health:** optional `Details[]`.
  - **Removed:** License resources, Interfaces, SBoM resource and NE107 events. The SBOM now lives in `/opt/oi4/licenses`.
  - **Compliance:** domains (Messaging, Service Container) × levels L1–L3.

## Impact on project decisions

| Decision | Result |
|----------|--------|
| ADR-0001 source of truth | amended: MG + RIG + GL-ADRs; cite GL-ADRs as `GL-ADR-0xx` |
| ADR-0002 … 0004, 0007 … 0010, 0012, 0013 | unaffected |
| ADR-0005 drop `_ToDelete_` | confirmed upstream (GL-ADR-008/002), note added |
| ADR-0006 D7 scope | amended: tests Messaging compliance L1/L2/L3 of apps + smart devices; profiles `level1..3`; container checks → Q-013 |
| ADR-0011 SDK S4 | amended: L1 smart devices make the C API more relevant; SDK must work without container contract, level selectable; `no_std` → Q-015 |

## Changes

- **GUARDRAILS:**
  - G-1 rewritten (two documents, GL-ADRs, citation scheme, baseline).
  - G-2 rewritten as G-2.1 … G-2.17 (`Oi4v2`, `ReplyTo`, writer name/group, content masks, Actions, reserved DSWIDs, compliance levels, SBOM).
  - G-4.6 and G-4.8 updated (SDK for all levels; SBOM in images).
- **SPEC-FINDINGS:**
  - Rewritten against `5f070bb`.
  - Closed: SF-007, 020, 022 resolved; SF-021 obsolete.
  - New: SF-023 (helper WriterGroupName), SF-024 (two StatusCode encodings), SF-025 (compliance level declaration), SF-026 (DetailObject NodeId), SF-027 (SBOM layout).
- **OPEN-QUESTIONS:**
  - New Q-013 (D7 container checks), Q-014 (v1 coexistence), Q-015 (target levels / `no_std`).
  - Annotated Q-003, Q-008, Q-010 and Q-011.
- **ROADMAP:**
  - P1: REQ catalog tagged by domain + level, covering MG + RIG.
  - P2: coverage updated (Actions, masks, new keys).
  - P3/P4: level selection, explicit config without container.
  - P5: level profiles, `Oi4v2`, `ReplyTo`.
  - P6: SBOM; optional L1 smart-device example.
  - P7: observed compliance level, v1 detection.
  - P8: migration notes per GL-ADR.
- **research:**
  - `guideline-digest.md` rewritten for `5f070bb`.
  - Pre-`5f070bb` notes added to the SDK research papers and the legacy analysis.
- **AGENTS.md, STATUS:** baseline and source-of-truth facts updated.

## Next

- PO answers Q-013, Q-014 and Q-015.
- Start P1: requirements catalog with domain/level tagging, and the topic grammar for `Oi4v2`.
