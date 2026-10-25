# Review and decisions – open items for the PO

_Last updated: 2026-10-25, after the PO review of C-037 (guideline `a42a132`)._

This is the single list of everything that waits for the PO: items to **review** (drafts and changes to reviewed items) and items to **decide** (spec findings, open questions, cycle plan). Each cycle updates it; items leave the list once the PO has answered and `spectool review` (or the decision record) has been applied. Details are in the cycle log; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md) (same numbering).

**How to answer:** tick the box or write the decision into the "PO" field (or answer in the chat, e.g. "D-1: wie vorgeschlagen, R-1 … R-4 akzeptiert"). Anything not accepted gets a note; the agent reworks it in the next cycle.

Current state: **nothing open.** All items in `oec/spec` are `reviewed`; no open spec finding (next SF-100); no open question for P2 (Q-011 parked). Coverage: 35 `schema`/`d7-validate` gaps; entry schemas 18 of 29. C-038 (Data, Event, SubscriptionList) fills this list again.

## Last review – C-037 (answered 2026-10-25)

The filled-in sheet is kept in git (`be07dcc`); details in the [C-037 log, addendum 1](log/2026-10-25_C-037_publist-refdes.md#addendum-1--po-review-2026-10-25).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| D-1 | ADR-0019 amendment C-037 – entry Filter checks, Mode forms, Set/Del counting, rule-layer additions, PL/RD dispatch | accepted | ADR-0019 status and amendment heading (`097b812`) |
| D-2 | Oracle reading: the PublicationList entry Filter is topic-encoded and has the Tag structure of its resource | accepted | oracle unchanged (`f36c357`) |
| R-1, R-2 | V-PAY-218 … 230 (PublicationList, ReferenceDesignation) | all accepted | `spectool review` (`69a6bb4`), gaps 48 → 35 |

## Earlier review – C-036 addendum 2 (answered 2026-10-25)

The filled-in sheet is kept in git (`5be6f29`); details in the [C-036 log, addendum 3](log/2026-10-25_C-036_actions-mam-profile.md#addendum-3--po-review-of-addendum-2-2026-10-25).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| R-1 | New REQ-CMP-047 – `Get` for an unknown asset: no `Pub`, should `Event` Status Bad | accepted | `spectool review` (`737cb14`) |
| R-2 | REQ-CMP-041 (reviewed) – statement extended by the `Filter` case | accepted | no change (stays `reviewed`) |
| R-3 | Guideline wording of SF-099 (`a42a132`) | accepted | no change |
