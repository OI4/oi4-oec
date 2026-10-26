# Review and decisions – open items for the PO

_Last updated: 2026-10-26, after C-041 (guideline `168e1d1`)._

This is the single list of everything that waits for the PO: items to **review** (drafts and changes to reviewed items) and items to **decide** (spec findings, open questions, cycle plan). Each cycle updates it; items leave the list once the PO has answered and `spectool review` (or the decision record) has been applied. Details are in the cycle log; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md) (same numbering).

**How to answer:** tick the box or write the decision into the "PO" field (or answer in the chat, e.g. "D-1 … D-3 wie vorgeschlagen, R-1 akzeptiert"). Anything not accepted gets a note; the agent reworks it in the next cycle.

Current state: **3 decisions** (ADR-0019 amendment C-041, the P2 exit, the release tag) and **1 review item** (7 reclassified REQs) from C-041 – P2 exit review. No open spec finding (next SF-101); no open question for P2 (Q-011 parked). Coverage: 158 `schema`/`d7-validate` REQs, **0 gaps** before and after this review. Details: [C-041 log](log/2026-10-26_C-041_p2-exit.md).

## Overview

| # | Item | Kind | Source | Recommendation | PO |
|---|------|------|--------|----------------|----|
| D-1 | ADR-0019 amendment C-041 – crosscheck gate, oracle ReplyTo fix, `oec/check.sh`, release/freeze rules, D7 bundle reporting, enforced meaning of `schema` | decide | C-041 §8 | accept | ☐ |
| D-2 | P2 exit: all 7 exit criteria met (6 without a Windows/Linux test run, 7 without hosting) → close P2 | decide | C-041 "P2 exit assessment" | accept | ☐ |
| D-3 | Local git tag `oec-schema-2.0.0` on the release commit | decide | C-041 §6 | accept | ☐ |
| R-1 | REQ-LIFE-003/007/009, REQ-MSG-023, REQ-RES-MAM-016, REQ-RES-PUBLIST-003, REQ-RES-REFDES-013 (reviewed → `draft`): verification `schema` removed | review | C-041 §4 | accept | ☐ |

## Decisions

### D-1 – ADR-0019 amendment C-041 (`proposed`)

- **Where:** [ADR-0019, amendment C-041](decisions/ADR-0019-schema-architecture.md#amendment-c-041-2026-10-26--p2-exit-crosscheck-release-200-ci-gate-agent-proposals-within-this-adr-g-44-proposed-po-review-pending).
- **Content:**
  1. `spectool crosscheck` is a gate: on every schema case (vectors and probes, 634) the oracle must agree – entry schema rejects with S → oracle rejects with ⊇ S; generic schema rejects → oracle rejects; schema accepts → oracle reports only `d7-validate` REQs.
  2. Oracle fix found by it: the ReplyTo sender must be a PublisherId (probe P-16).
  3. `oec/check.sh` is the one local gate (spec checks incl. `coverage -gate`, both validators, release freeze, crosscheck, conformance tests).
  4. Release rules: `schema.Version`/`Baseline`, `CHANGELOG.md`, manifest `release/<version>.json`; a release is immutable; no `$id`/`$defs` name of a release disappears within v2; after a release the next change is a pre-release `2.x.y-dev`; `Baseline` = spec baseline (a rebaseline forces a version bump); `dist/` is cleaned before writing.
  5. D7 reports `schemas` (bundle version) and `baseline` and refuses a mismatch.
  6. `schema` only when a schema reports the REQ (R-1).
- **PO:** ☐

### D-2 – P2 exit

| # | Criterion | State |
|---|---|---|
| 1 | ADR-0019 accepted | met (C-041 amendment = D-1) |
| 2 | 0 coverage gaps | met – 158 REQs, 0 gaps |
| 3 | Entry schemas for all matrix rows, vectors as expected | met – 29 of 29, 25 rule-layer expectations |
| 4 | Both validators agree | met – 634 cases, 0 disagreements |
| 5 | Oracle checks every schema rule | met – crosscheck 0 disagreements |
| 6 | D7 on Windows, macOS, Linux | met with a limit – cross-compiles for all three; tests run on macOS only |
| 7 | Schemas released, stable `$id`s | met, hosting open – 2.0.0 frozen; hosting waits for Q-011 |

- **Recommendation:** close P2 after D-1 and R-1. The two limits depend on the publication (Q-011, hosted CI), not on P2 work; they are named in `oec/schema/CHANGELOG.md`.
- **Next:** ROADMAP order P3 (Rust SDK + C API sketch, crate choices; D8 CLI shape on the D7 base) → P4 SDK core. The PO picks the order.
- **PO:** ☐

### D-3 – Tag `oec-schema-2.0.0`

- After acceptance, the commit that holds the accepted release (2.0.0 manifest + ADR status) gets the local tag `oec-schema-2.0.0` (no push, ADR-0009).
- Not `oec/schema/v2.0.0`: Go reads `<module dir>/vX.Y.Z` tags as module versions, and the module path `github.com/OI4/oec/schema` has no `/v2` suffix, so `go get` would reject it.
- **Alternative:** no tag; the manifest alone identifies the release.
- **PO:** ☐

## Review items

### R-1 – 7 REQs lose `schema`

| REQ | After | Who checks it |
|---|---|---|
| REQ-LIFE-003, 007, 009 | `d7-validate` + `d7-monitor`/`d7-test` | rule R-6 with `--role` (Birth/Close/Will is not visible in one message) |
| REQ-MSG-023 | `d7-validate` | rule R-2 (DataSetWriterName = Source of the topic) |
| REQ-RES-MAM-016 | `d7-validate` | rule R-3 (ProductInstanceUri = DataSetWriterName) |
| REQ-RES-PUBLIST-003 | `d7-validate` | rule R-3 (unique keys, as REQ-MSG-038 in C-035) |
| REQ-RES-REFDES-013 | `d7-validate` + `d7-test` | rule R-2 (`Query` only without Source) |

- No schema `$comment` and no schema case reports these REQs; every negative vector of them is a rule-layer expectation. Statements unchanged; only the verification and a note. Coverage stays 0 gaps.
- **PO:** ☐

## Last review – C-040 (answered 2026-10-26)

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
