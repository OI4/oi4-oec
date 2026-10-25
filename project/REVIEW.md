# Review and decisions – open items for the PO

_Last updated: 2026-10-25, after the PO review of C-038 (guideline `a42a132`)._

This is the single list of everything that waits for the PO: items to **review** (drafts and changes to reviewed items) and items to **decide** (spec findings, open questions, cycle plan). Each cycle updates it; items leave the list once the PO has answered and `spectool review` (or the decision record) has been applied. Details are in the cycle log; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md) (same numbering).

**How to answer:** tick the box or write the decision into the "PO" field (or answer in the chat, e.g. "D-1, D-2 wie vorgeschlagen, R-1 … R-3 akzeptiert"). Anything not accepted gets a note; the agent reworks it in the next cycle.

Current state: **nothing open.** All items in `oec/spec` are `reviewed`; no open spec finding (next SF-100); no open question for P2 (Q-011 parked). Coverage: 19 `schema`/`d7-validate` gaps; entry schemas 26 of 29. C-039 (Config + `broker.json`) fills this list again.

## Last review – C-038 (answered 2026-10-25)

The filled-in sheet is kept in git (`cc198a5`); details in the [C-038 log, addendum 1](log/2026-10-25_C-038_data-event-sublist.md#addendum-1--po-review-2026-10-25).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| D-1 | ADR-0019 amendment C-038 – Oi4Pv by WriterGroupName, Data without DataSetClassId, Event relations in the schema, dispatch, REQ-RES-SUBLIST-002 split, entry schemas, umbrellas | accepted | ADR-0019 status, amendment heading and §4 pointer (`574b8e0`) |
| D-2 | Oracle: the WriterGroupName of Data/Config/Event on a topic without Filter is a Filter of the resource (REQ-MSG-024) | accepted | oracle unchanged (`d6d5010`) |
| R-1 … R-3 | V-PAY-231 … 239 (Data, Event, SubscriptionList) | all accepted | `spectool review` (`dc80c8a`), gaps 33 → 19 |

## Earlier review – C-037 (answered 2026-10-25)

The filled-in sheet is kept in git (`be07dcc`); details in the [C-037 log, addendum 1](log/2026-10-25_C-037_publist-refdes.md#addendum-1--po-review-2026-10-25).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| D-1 | ADR-0019 amendment C-037 – entry Filter checks, Mode forms, Set/Del counting, rule-layer additions, PL/RD dispatch | accepted | ADR-0019 status and amendment heading (`097b812`) |
| D-2 | Oracle reading: the PublicationList entry Filter is topic-encoded and has the Tag structure of its resource | accepted | oracle unchanged (`f36c357`) |
| R-1, R-2 | V-PAY-218 … 230 (PublicationList, ReferenceDesignation) | all accepted | `spectool review` (`69a6bb4`), gaps 48 → 35 |
