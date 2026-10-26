# Review and decisions – open items for the PO

_Last updated: 2026-10-26, after C-040 (guideline `168e1d1`)._

This is the single list of everything that waits for the PO: items to **review** (drafts and changes to reviewed items) and items to **decide** (spec findings, open questions, cycle plan). Each cycle updates it; items leave the list once the PO has answered and `spectool review` (or the decision record) has been applied. Details are in the cycle log; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md) (same numbering).

**How to answer:** tick the box or write the decision into the "PO" field (or answer in the chat, e.g. "D-1 … D-4 wie vorgeschlagen, R-1 … R-4 akzeptiert"). Anything not accepted gets a note; the agent reworks it in the next cycle.

Current state: **4 decisions** (ADR-0019 amendment C-040 and three of its points) and **4 review items** (9 drafts + the umbrellas) from C-040 – D7 `validate`. No open spec finding (next SF-101); no open question for P2 (Q-011 parked). Coverage: 11 `schema`/`d7-validate` gaps now, **0 after this review** (projection; `coverage -check -gate` green on a copy with the drafts reviewed) – P2 exit criterion 2 is then met. Details: [C-040 log](log/2026-10-26_C-040_d7-validate.md).

## Overview

| # | Item | Kind | Source | Recommendation | PO |
|---|------|------|--------|----------------|----|
| D-1 | ADR-0019 amendment C-040 – D7 layers and finding model, dedupe, R-4 as mutation probes, CLI contract (flags, exit codes, binaries) | decide | C-040 §8 | accept | ☐ |
| D-2 | Parse findings (invalid JSON) report REQ-CMP-005 | decide | C-040 §7 | accept | ☐ |
| D-3 | REQ-IL-005 as a heuristic rule (encoded IL in the payload) in oracle and D7, not as an umbrella | decide | C-040 §3 | accept | ☐ |
| D-4 | A valid topic vector with the warning `whitespace` counts as a negative of REQ-TOP-020 | decide | C-040 §5 | accept | ☐ |
| R-1 | V-TOP-218 (reviewed → `draft`): violates + REQ-ACT-020 | review | C-040 §2 | accept | ☐ |
| R-2 | V-PAY-242 … 244 (REQ-IL-005) | review | C-040 §3 | accept | ☐ |
| R-3 | V-TOP-135 … 137, V-TOP-254 | review | C-040 §4 | accept | ☐ |
| R-4 | Umbrellas REQ-CMP-004/005/025/026, REQ-TOP-002, REQ-IL-001/008 | review | C-040 §6 | accept | ☐ |

## Decisions

### D-1 – ADR-0019 amendment C-040 (`proposed`)

- **Where:** [ADR-0019, amendment C-040](decisions/ADR-0019-schema-architecture.md#amendment-c-040-2026-10-26--d7-validate-agent-proposals-within-this-adr-g-44-proposed); code in [`oec/conformance`](../oec/conformance/README.md).
- **Content:**
  1. Four layers `topic`, `parse`, `schema`, `rule`; each finding has layer, REQ ids, pointer/topic level, message; schema errors map to the REQ of the nearest `$comment`; the rule layer runs even when the schema fails but skips values of the wrong type.
  2. Parse → REQ-CMP-005 (D-2).
  3. Dedupe: a rule finding with the same REQ and pointer as a schema finding is dropped (each violation reported once).
  4. R-4 (number/calendar semantics) as mutation probes against the schema; unit-tested, no vector needs it today.
  5. REQ-IL-005 heuristic (D-3).
  6. Warning layer of REQ-TOP-020 (D-4).
  7. CLI: `oec-check validate --topic/--object/--role/--tolerant/--format`, message files (JSON, JSONL, directories), exit codes 0 valid (warnings allowed) / 1 invalid / 2 usage or input / 3 internal; `version` names the baseline; data only from the embedded `spec.FS`/`schema.FS`; D8 binary `oec-infra`; pure Go (cross-compiles for Windows, Linux, macOS – P2 exit 6).
  8. Umbrellas (R-4).
- **PO:** ☐

### D-2 – Parse findings report REQ-CMP-005

- A payload that is no valid JSON (not UTF-8, two values, duplicate keys, syntax error) had no REQ. D7 now reports REQ-CMP-005 ("the payload is OPC UA-conform JSON"). REQ-CMP-005 is also an umbrella (R-4); its remainder (strict JSON syntax) is exactly what the parse layer checks.
- **Alternative:** a new REQ "the payload is JSON" via an SF-101 – it would only restate REQ-CMP-005.
- **PO:** ☐

### D-3 – REQ-IL-005 as a heuristic rule

- REQ-IL-005: "Identification Links in the payload are not encoded." Oracle and D7 (R-3) report a value that is **no valid IL but whose base64url decoding is one** – the typical mistake of copying the topic level into the payload. Checked fields: PublisherId, RequestorId, the sender in MessageId and ReplyTo, every DataSetWriterName, the DataSetWriterName of MetaData.
- **Risk:** a false positive for a value that is not a valid IL and happens to decode to one; none is known from MG/RIG.
- **Alternative:** REQ-IL-005 as an umbrella over MSG-006/007/023 and ACT-004/005 (the IL syntax of each field) – it would not detect the encoded form specifically.
- **PO:** ☐

### D-4 – Warning vectors count as negatives of REQ-TOP-020

- REQ-TOP-020 is a SHOULD (no whitespace in Filter levels). A topic with whitespace is valid; D7 prints `warning: whitespace (REQ-TOP-020)` and exits 0. The coverage report now counts such a valid vector as the **negative** of REQ-TOP-020 (layer `warning`), not as a positive.
- The 8 reviewed whitespace vectors (V-TOP-011/014/017/019/020/021/132/133) still list REQ-TOP-020 in `reqs`; they are unchanged (only the counting changed). The positive is V-TOP-137 (R-3).
- **PO:** ☐

## Reviews

### R-1 – V-TOP-218 (reviewed → `draft`)

- **Where:** [view](../oec/spec/vectors/view/topic-invalid.md#v-top-218).
- **Change:** "Call with Filter" on NewDataSetWriterId: `violates` REQ-TOP-013 → REQ-ACT-020 + REQ-TOP-013; reason + "the topic of NewDataSetWriterId has no Filter (MG 9.2.1, as V-TOP-253)". D7's level-by-level diagnosis found the missing REQ.
- **PO:** ☐

### R-2 – V-PAY-242 … 244 (`draft`)

- **Where:** [view](../oec/spec/vectors/view/payload-constructed.md#v-pay-242).
- V-PAY-242 (invalid): Pub Health whose ReplyTo carries the base64url form of the requestor's IL → REQ-IL-005.
- V-PAY-243 (invalid): Call whose RequestorId and MessageId carry the base64url form → REQ-IL-005.
- V-PAY-244 (valid): Pub Health with all ILs unencoded.
- Schema layer: none (JSON Schema cannot decode base64url); rule layer R-3 (`tests/rule-layer.yaml`).
- **PO:** ☐

### R-3 – V-TOP-135 … 137, V-TOP-254 (`draft`)

- **Where:** [valid](../oec/spec/vectors/view/topic-valid.md#v-top-135), [invalid](../oec/spec/vectors/view/topic-invalid.md#v-top-254).
- V-TOP-135: Pub Health of a device (with Source) – positive of TOP-001/009/011/012/013/018, IL-002/004.
- V-TOP-136: Call NewDataSetWriterId with Source – positive of TOP-001/011/012/013/018, IL-002/004.
- V-TOP-137: Pub Event with Filter `Status/Bad` – positive of TOP-001/013/020/021/022, IL-002.
- V-TOP-254 (invalid): `…/Pub/Health/` (empty Source level) – violates TOP-018, fails the grammar.
- **PO:** ☐

### R-4 – Umbrellas in `coverage/implications.yaml`

- **Where:** [`implications.yaml`](../oec/spec/coverage/implications.yaml) (end of file), rule of ADR-0019 §9.
- REQ-CMP-004 (topic conformity) → TOP-001/005/009/011/012/013/018/019/021/022/023, IL-002/003/004, ACT-020; remainder TOP-003/006/010/016 (`d7-monitor`/`d7-test`), TOP-020 warning.
- REQ-CMP-025/026 (fields) → the "Fields of …" REQs (MSG, ACT, BUS, LOC, PAG, RES-*); CMP-025 + MSG-001, CMP-026 + MSG-005/031/039, CONFIG-004/006/007, MAM-004/017, REFDES-012.
- REQ-CMP-005 (OPC UA JSON) → MSG-001/002/003/005/016/031/034, CMP-025/026; remainder strict JSON syntax (parse layer, D-2).
- REQ-TOP-002 → CMP-004, CMP-005.
- REQ-IL-001 → IL-002/004/009/016/018/019, TOP-009/018, MSG-006/023.
- REQ-IL-008 → IL-009/016/018/019.
- **PO:** ☐

## Last review – C-039 addendum 2 (answered 2026-10-26)

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
