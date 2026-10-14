# C-008 – Re-baseline on `7bead4c` (spec findings resolved)

- Date: 2026-10-14
- Phase: P3 (cross-cutting)
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Guideline baseline: `oi4-oec-documentation` `5f070bb` → **`7bead4c`** (branch `feature/spec-findings`)

## Input from product owner

The PO had our findings and some new ones fixed in the guideline. He asked us to go through everything again and adapt it.

## Analysis

- Upstream ran 28 commits `5f070bb..7bead4c`, one per finding.
  - Added `findings/SPEC-FINDINGS.md`, a copy of our register with every finding resolved.
  - New guideline ADRs:
    - **GL-ADR-012:** only the common service `NewDataSetWriterId` remains.
    - **GL-ADR-013:** `NodeId` is a JSON string per OPC UA V1.05.
  - New findings SF-028 … 031, all resolved: outdated example topics, link targets, Confluence links, NodeId encoding.
- Lesson from C-006: never record "resolved" without checking. **Every upstream resolution was spot-checked by grep** for the previously reported defects:
  - v1.04 links: 0 left
  - `https://groe.mca`: 0
  - DSWID 0 in templates: 0
  - `en-EN`: 0
  - `/Syslog/…`: 0
  - "in KiB": 0
  - "still being observed": 0
  - `APPLCATION`: 0
  - NodeId object form: only in the normative sentence that forbids it
  - removed common services: 0
  - B1.1.11 pagination ids: correct (1/2)
  - Set topics in MG 10.1.3/10.1.5: correct
- Also scanned for new inconsistencies (`CorrelationId`, `MSG`, `oi4/`, `Source`/`Filter` keys, `ns=`). The remaining hits are historical remarks or PublicationList keys. **No new findings.**

## Impact on project decisions

| Decision | Result |
|----------|--------|
| ADR-0001 | amended: baseline `7bead4c`, GL-ADR-012/013, upstream findings mirror, branch tracking |
| ADR-0006 (D7) | amended: SF-025 resolved → declared level is a normative test-run parameter (`--level`, includes lower levels); only `NewDataSetWriterId` as common-service Action |
| ADR-0008 (registry) | amended: availability from Birth/Close/Will (MG 7.3.4/7.3.5); observed level (A1) |
| ADR-0014 (D8) | amended: canonical SBOM names `sbom.spdx.json`/`sbom.cdx.json`, `<License-ID>.txt` (SF-027) |
| all others (0002–0005, 0007, 0009–0013, 0015, 0016) | unaffected |
| Open questions Q-011, Q-016, Q-017 | unaffected |

## Changes

- **GUARDRAILS:**
  - G-1.1/1.2/1.4/1.5: baseline, GL-ADR range, findings mirror with shared SF ids, OPC UA links.
  - G-2.6 NodeId strings; G-2.7 opaque MessageId; G-2.8 Status rules; G-2.9 DataSetClassId rules; G-2.10 DSWID rules, only `NewDataSetWriterId`; G-2.11 MQTT 3.1.1 minimum; G-2.12 Birth/Close/Will topics and semantics; G-2.13 level declaration; G-2.15 bytes; G-2.16 canonical SBOM; G-2.17 percent-encoding.
  - G-4.8 canonical SBOM names.
- **SPEC-FINDINGS:** rebuilt from the verified upstream register. All 31 findings are closed, the Open table is empty, the next free id is SF-032.
- **ROADMAP:** P1 (exit state, percent-encoding in the topic grammar), P2 coverage (NodeId strings, LocaleId, DSWID ≠ 0, helpers, only NewDataSetWriterId), P5 (`--level`), P5b (canonical SBOM), P7 (availability, observed level).
- **research/guideline-digest.md:** updated to `7bead4c`.
- **AGENTS.md, STATUS:** baseline updated.

## Notes

- The baseline is a **branch** commit (`feature/spec-findings`). When it is merged, switch the baseline to the merge commit; the content is identical (ADR-0001 amendment).

## Next

- PO answers Q-016 (micro SDK language/hardware) and confirms Q-017 (D8 scope).
- Start P1: requirements catalog for MG + RIG.
