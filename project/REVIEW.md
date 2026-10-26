# Review and decisions – open items for the PO

_Last updated: 2026-10-26, after the C-041 review (guideline `168e1d1`); P2 closed._

This is the single list of everything that waits for the PO: items to **review** (drafts and changes to reviewed items) and items to **decide** (spec findings, open questions, cycle plan). Each cycle updates it; items leave the list once the PO has answered and `spectool review` (or the decision record) has been applied. Details are in the cycle log; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md) (same numbering).

**How to answer:** tick the box or write the decision into the "PO" field (or answer in the chat, e.g. "D-1 … D-3 wie vorgeschlagen, R-1 akzeptiert"). Anything not accepted gets a note; the agent reworks it in the next cycle.

Current state: **nothing open.** P2 is closed (C-041 review, 2026-10-26); schema release 2.0.0 is tagged `oec-schema-2.0.0`. No draft REQ or vector, no open spec finding (next SF-101), no open question except the parked Q-011. Coverage: 158 `schema`/`d7-validate` REQs, **0 gaps**. Next: C-042 – P3 start.

## Last review – C-041 (answered 2026-10-26)

The filled-in sheet is kept in git (`077bdd5`); details in the [C-041 log, addendum 1](log/2026-10-26_C-041_p2-exit.md#addendum-1--po-review-2026-10-26).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| D-1 | ADR-0019 amendment C-041 – crosscheck gate, oracle ReplyTo fix, `oec/check.sh`, release/freeze rules, D7 bundle reporting, enforced meaning of `schema` | accepted | ADR-0019 status line, amendment heading and tag item (`03ec842`) |
| D-2 | P2 exit | accepted | P2 closed: ROADMAP P2 ✅, STATUS current phase P3, C-041 log addendum 1 |
| D-3 | Local git tag `oec-schema-2.0.0` | accepted | annotated tag on `03ec842` (records the acceptance; schema files unchanged since `5a2f16c`) |
| R-1 | REQ-LIFE-003/007/009, REQ-MSG-023, REQ-RES-MAM-016, REQ-RES-PUBLIST-003, REQ-RES-REFDES-013 without `schema` | accepted | `spectool review` (`98c0aae`), 0 drafts, gaps stay 0 |

## Earlier review – C-040 (answered 2026-10-26)

The filled-in sheet is kept in git (`8688f42`); details in the [C-040 log, addendum 1](log/2026-10-26_C-040_d7-validate.md#addendum-1--po-review-2026-10-26).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| D-1 | ADR-0019 amendment C-040 – D7 layers and finding model, dedupe, R-4 as mutation probes, CLI contract | accepted | ADR-0019 status line and amendment heading (`7a50660`) |
| D-2 | Parse findings report REQ-CMP-005 | accepted | already implemented (`7db7f73`) |
| D-3 | REQ-IL-005 as a heuristic rule in oracle and D7 | accepted | already implemented (`80a2b38`) |
| D-4 | Warning vectors count as negatives of REQ-TOP-020 | accepted | already implemented (`3409d9f`) |
| R-1 … R-3 | V-TOP-218; V-PAY-242 … 244; V-TOP-135 … 137, V-TOP-254 | all accepted | `spectool review` (`81b3521`), gaps 11 → 0 |
| R-4 | Umbrellas REQ-CMP-004/005/025/026, REQ-TOP-002, REQ-IL-001/008 | accepted | nothing to apply (`b677948`) |

## Earlier review – C-039 addendum 2 (answered 2026-10-26)

The filled-in sheet is kept in git (`2c7f76d`); details in the [C-039 log, addendum 3](log/2026-10-25_C-039_config-broker.md#addendum-3--po-review-of-addendum-2-2026-10-26).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| R-1 | REQ-CNT-003 reworded (Address forms, SecurePort 1 … 65535, further elements) | accepted | `spectool review` (`995258d`) |
| R-2 | REQ-CNT-004 extended (ignore unknown elements) | accepted | `spectool review` (`995258d`) |
| R-3 | Guideline wording of SF-100 (RIG 4.1.1, `168e1d1`) | accepted | nothing to apply |

## Earlier review – C-039 (answered 2026-10-26)

The filled-in sheet is kept in git (`4ad2f96`); details in the [C-039 log, addendum 1](log/2026-10-25_C-039_config-broker.md#addendum-1--po-review-2026-10-26).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| D-1 | ADR-0019 amendment C-039 – Config def families, representation and masking, dispatch, rule layer, entry schemas, umbrellas; platform module `broker.json`, `d8` tracing exception, vector kind `broker-json` | accepted | ADR-0019 status, amendment heading, item 7 (`d7520ba`) |
| D-2 | SF-100 – content of `broker.json` | as proposed, implement the RIG text | RIG 4.1.1 `168e1d1`, register `7808df7`, re-baseline `b70eb87` (follow-ups: last review above) |
| R-1, R-2 | REQ-RES-CONFIG-011 → `d7-monitor`, REQ-RES-CONFIG-024 +`schema` | accepted | `spectool review` (`dd5893d`) |
| R-3, R-4 | V-PAY-240/241, V-BRK-001 … 018 | all accepted | `spectool review` (`dd5893d`), gaps 16 → 15 |
