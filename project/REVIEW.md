# Review and decisions – open items for the PO

_Last updated: 2026-10-25, after the PO review of C-036 addendum 2 (guideline `a42a132`)._

This is the single list of everything that waits for the PO: items to **review** (drafts and changes to reviewed items) and items to **decide** (spec findings, open questions, cycle plan). Each cycle updates it; items leave the list once the PO has answered and `spectool review` (or the decision record) has been applied. Details are in the cycle log; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md) (same numbering).

**How to answer:** tick the box or write the decision into the "PO" field (or answer in the chat, e.g. "D-1: wie vorgeschlagen, R-1 … R-4 akzeptiert"). Anything not accepted gets a note; the agent reworks it in the next cycle.

Current state: **nothing open.** All items in `oec/spec` are `reviewed`; no open spec finding (next SF-100); no open question for P2 (Q-011 parked). Coverage: 48 `schema`/`d7-validate` gaps; entry schemas 11 of 29. C-037 (PublicationList + ReferenceDesignation) fills this list again.

## Last review – C-036 addendum 2 (answered 2026-10-25)

The filled-in sheet is kept in git (`5be6f29`); details in the [C-036 log, addendum 3](log/2026-10-25_C-036_actions-mam-profile.md#addendum-3--po-review-of-addendum-2-2026-10-25).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| R-1 | New REQ-CMP-047 – `Get` for an unknown asset: no `Pub`, should `Event` Status Bad | accepted | `spectool review` (`737cb14`) |
| R-2 | REQ-CMP-041 (reviewed) – statement extended by the `Filter` case | accepted | no change (stays `reviewed`) |
| R-3 | Guideline wording of SF-099 (`a42a132`) | accepted | no change |

## Earlier review – C-036 (answered 2026-10-25)

The filled-in sheet is kept in git (`a9aae9c`); details in the [C-036 log, addendum 1](log/2026-10-25_C-036_actions-mam-profile.md#addendum-1--po-review-2026-10-25).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| D-1 | SF-099 – empty `Pub` as the answer to a `Get` without content | decided differently: no content → empty list; unknown asset → no `Pub`, should `Event` Status Bad | register `5491830` (`decided`); on PO order in the guideline `a42a132` (addendum 2, review items R-1 … R-3 above); schemas unchanged |
| D-2 | ADR-0019 amendment C-036 | accepted | ADR-0019 status, amendment heading and point 1 (`be648c9`) |
| D-3 | Cycle split – PublicationList/ReferenceDesignation → C-037, exit C-041 | accepted | ROADMAP P2 unchanged since C-036 |
| D-4 | REQ-RES-MAM-006 heuristic | accepted | oracle unchanged (`298e709`) |
| R-1 … R-4 | V-PAY-176 … 217, V-TOP-253, `reqs` of 8 guideline vectors | all accepted | `spectool review` (`879e5e9`), gaps 81 → 48 |

**Follow-up:** the PO confirmed on 2026-10-25 that a `Filter` on a known asset that matches nothing gets the empty list, and ordered SF-099 into the guideline (addendum 2).
