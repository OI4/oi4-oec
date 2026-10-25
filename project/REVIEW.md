# Review and decisions – open items for the PO

_Last updated: 2026-10-25, after the PO review of C-036 (guideline `68d878d`)._

This is the single list of everything that waits for the PO: items to **review** (drafts and changes to reviewed items) and items to **decide** (spec findings, open questions, cycle plan). Each cycle updates it; items leave the list once the PO has answered and `spectool review` (or the decision record) has been applied. Details are in the cycle log; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md) (same numbering).

**How to answer:** tick the box or write the decision into the "PO" field (or answer in the chat, e.g. "D-1: wie vorgeschlagen, R-1 … R-4 akzeptiert"). Anything not accepted gets a note; the agent reworks it in the next cycle.

Current state: **nothing open.** All items in `oec/spec` are `reviewed`; SF-099 is decided and waits only for a PO order to go into the guideline (next SF-100); no open question for P2 (Q-011 parked). Coverage: 48 `schema`/`d7-validate` gaps. C-037 (PublicationList + ReferenceDesignation) fills this list again.

## Last review – C-036 (answered 2026-10-25)

The filled-in sheet is kept in git (`a9aae9c`); details in the [C-036 log, addendum 1](log/2026-10-25_C-036_actions-mam-profile.md#addendum-1--po-review-2026-10-25).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| D-1 | SF-099 – empty `Pub` as the answer to a `Get` without content | decided differently: no content → empty list; unknown asset → no `Pub`, should `Event` Status Bad | register `5491830` (`decided`); guideline text on PO order; schemas unchanged |
| D-2 | ADR-0019 amendment C-036 | accepted | ADR-0019 status, amendment heading and point 1 (`be648c9`) |
| D-3 | Cycle split – PublicationList/ReferenceDesignation → C-037, exit C-041 | accepted | ROADMAP P2 unchanged since C-036 |
| D-4 | REQ-RES-MAM-006 heuristic | accepted | oracle unchanged (`298e709`) |
| R-1 … R-4 | V-PAY-176 … 217, V-TOP-253, `reqs` of 8 guideline vectors | all accepted | `spectool review` (`879e5e9`), gaps 81 → 48 |

**Open point for the PO (not a review item):** the agent reads "es gibt nichts" to include a `Filter` on a known asset that matches nothing (→ empty list); please confirm when SF-099 goes into the guideline.

## Earlier review – C-035 (answered 2026-10-25)


The filled-in sheet is kept in git (`fe28054`); details in the [C-035 log, addendum 1](log/2026-10-24_C-035_metadata.md#addendum-1--po-review-2026-10-25).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| D-1 | ADR-0019 amendment C-035 (`ResourceMetaData`, R-3 name uniqueness, R-6 declared role) | accepted | ADR-0019 status and amendment (`705edd4`) |
| D-2 | Scope change – Metadata entry schemas in C-035 | accepted | ROADMAP P2 unchanged since C-035 |
| D-3 | Topic REQs of the roles (REQ-LIFE-002/006/008) | option (a): keep `d7-monitor`/`d7-test` | `life.yaml` header, ADR-0019 amendment point 3 |
| R-1 … R-4 | V-PAY-152 … 175, V-PAY-023 … 025 with `role`, REQ-LIFE-003/007/009 and REQ-MSG-038 reclassified | all accepted | `spectool review` (`8f24ecb`), gaps 96 → 77 |
