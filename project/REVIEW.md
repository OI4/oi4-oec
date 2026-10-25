# Review and decisions – open items for the PO

_Last updated: 2026-10-25, after the PO review of C-035 (guideline `68d878d`)._

This is the single list of everything that waits for the PO: items to **review** (drafts and changes to reviewed items) and items to **decide** (spec findings, open questions, cycle plan). Each cycle updates it; items leave the list once the PO has answered and `spectool review` (or the decision record) has been applied. Details are in the cycle log; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md) (same numbering).

**How to answer:** tick the box or write the decision into the "PO" field (or answer in the chat, e.g. "D-1: wie vorgeschlagen, R-1 … R-4 akzeptiert"). Anything not accepted gets a note; the agent reworks it in the next cycle.

Current state: **nothing open.** All items in `oec/spec` are `reviewed`; no open spec finding (next SF-099); no open question for P2 (Q-011 parked). Coverage: 77 `schema`/`d7-validate` gaps. C-036 (Actions + core resources) fills this list again.

## Last review – C-035 (answered 2026-10-25)

The filled-in sheet is kept in git (`fe28054`); details in the [C-035 log, addendum 1](log/2026-10-24_C-035_metadata.md#addendum-1--po-review-2026-10-25).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| D-1 | ADR-0019 amendment C-035 (`ResourceMetaData`, R-3 name uniqueness, R-6 declared role) | accepted | ADR-0019 status and amendment (`705edd4`) |
| D-2 | Scope change – Metadata entry schemas in C-035 | accepted | ROADMAP P2 unchanged since C-035 |
| D-3 | Topic REQs of the roles (REQ-LIFE-002/006/008) | option (a): keep `d7-monitor`/`d7-test` | `life.yaml` header, ADR-0019 amendment point 3 |
| R-1 … R-4 | V-PAY-152 … 175, V-PAY-023 … 025 with `role`, REQ-LIFE-003/007/009 and REQ-MSG-038 reclassified | all accepted | `spectool review` (`8f24ecb`), gaps 96 → 77 |

## Earlier review – C-034 (answered 2026-10-24)

The filled-in sheet is kept in git (`fec3030`); details in the [C-034 log, addendum 1](log/2026-10-23_C-034_foundation-envelope.md#addendum-1--po-review-sf-098-in-the-guideline-re-baseline-on-68d878d-2026-10-24).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| D-1 | SF-098 – `Del` with zero or one DataSetMessage | as proposed; implement in the guideline | MG 8.3.11 `68d878d`, register `ff0d842`, re-baseline `4037103`; REQ-PAG-009 statement |
| D-2 | Q-020 – Birth/Close/Will roles in D7 `validate` | option (a) | OPEN-QUESTIONS (Decided), ADR-0019 §9 note; implementation in C-035 |
| D-3 | Cycle split – MetaData C-035, exit C-040 | accepted | ROADMAP P2 unchanged |
| R-1 … R-5 | vectors V-PAY-129 … 151, six re-drafted vectors, REQ-MSG-026/PAG-009/BUS-002, `implications.yaml`, ADR-0019 editorial | all accepted | `spectool review` (`8447ec5`), gaps 129 → 96 |
