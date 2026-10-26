# Review and decisions – open items for the PO

_Last updated: 2026-10-26, after C-039 addendum 2 (guideline `168e1d1`)._

This is the single list of everything that waits for the PO: items to **review** (drafts and changes to reviewed items) and items to **decide** (spec findings, open questions, cycle plan). Each cycle updates it; items leave the list once the PO has answered and `spectool review` (or the decision record) has been applied. Details are in the cycle log; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md) (same numbering).

**How to answer:** tick the box or write the decision into the "PO" field (or answer in the chat, e.g. "D-1, D-2 wie vorgeschlagen, R-1 … R-3 akzeptiert"). Anything not accepted gets a note; the agent reworks it in the next cycle.

Current state: **3 review items** from implementing SF-100 (two changed reviewed REQs back on `draft`, the guideline wording); no open spec finding (next SF-101); no open question for P2 (Q-011 parked). Coverage: 15 `schema`/`d7-validate` gaps (unchanged – both REQs are `d8`); entry schemas 29 of 29. Details: [C-039 log, addendum 2](log/2026-10-25_C-039_config-broker.md#addendum-2--sf-100-in-the-guideline-re-baseline-on-168e1d1-2026-10-26).

## Overview

| # | Item | Kind | Source | Recommendation | PO |
|---|------|------|--------|----------------|----|
| R-1 | REQ-CNT-003 (reviewed → `draft`) – content of `broker.json`: Address forms, SecurePort 1 … 65535, further elements | review | C-039 add. 2 | accept | ☑ accepted |
| R-2 | REQ-CNT-004 (reviewed → `draft`) – the application ignores unknown elements of `broker.json` | review | C-039 add. 2 | accept | ☑ accepted |
| R-3 | Guideline wording of SF-100 (`168e1d1`: RIG 4.1.1) | review | C-039 add. 2 | accept | ☑ accepted |

## Reviews

### R-1 – REQ-CNT-003 reworded (`draft`)

- **Where:** [`requirements/cnt.yaml`](../oec/spec/requirements/cnt.yaml) / [view](../oec/spec/requirements/view/cnt.md#req-cnt-003).
- **Statement (new parts in bold):** "broker.json in the Message Bus storage is a JSON object with the mandatory fields Address (String, IP address or host name of the broker**: an IPv4 address in dotted-decimal form without leading zeros, an IPv6 address in the text form of RFC 4291 without brackets and without zone index, or a host name of RFC 1123 – labels of 1 to 63 letters, digits and "-" that do not start or end with "-", the last label not all digits, at most 253 characters, no trailing "." –; no port, scheme or path**), SecurePort (UINT16, **1 … 65535,** port on which the broker listens for TLS connections) and MaxPacketSize (INT32, …, at least 262144 and at most 268435455). **It may contain further elements.** The broker accepts payloads up to MaxPacketSize."
- **Also:** quotes + "Address contains no port, no scheme and no path." and "Clients ignore elements of broker.json they do not know."; inventory + RIG-4.1.1-77f0d05d; refs + SF-100; the note names `platform/broker.schema.json`, the oracle `internal/platform` and V-BRK-001 … 018. Verification stays `[d8]`.
- **PO:** ☑ accepted (2026-10-26)

### R-2 – REQ-CNT-004 extended (`draft`)

- **Where:** [view](../oec/spec/requirements/view/cnt.md#req-cnt-004).
- **Change:** statement + "It ignores elements of broker.json it does not know."; quote of the new sentence; inventory + RIG-4.1.1-77f0d05d (shared with REQ-CNT-003: the platform may add elements, the application ignores them); refs + SF-100. Levels (L2 SHOULD, L3 MUST) and verification `[d8, manual]` unchanged – D8 can offer a `broker.json` with an extra element to the container.
- **PO:** ☑ accepted (2026-10-26)

### R-3 – Guideline wording (`168e1d1`)

- RIG 4.1.1, `Address`: value range "`<IPv4 address, IPv6 address (without brackets) or host name (RFC 1123)>`"; description + "An IPv4 address is written in dotted-decimal form without leading zeros, an IPv6 address in the text form of RFC 4291 without brackets and without zone index. A host name consists of labels of 1 to 63 letters, digits and `-`, separated by `.`; a label does not start or end with `-`, the last label is not all digits, and the host name has at most 253 characters and does not end with `.`. `Address` contains no port, no scheme and no path."
- RIG 4.1.1, `SecurePort`: value range "`1 … 65535`".
- After the element list: "Clients ignore elements of `broker.json` they do not know."
- **Beyond the register text:** the register proposed only the new value range for `Address`; the description sentences spell out the decided rules of SF-100 (b) so that the guideline, not only the register, carries them. Please object if you prefer the short form.
- No GL-ADR (like SF-099). `yarn build` green. Register `7808df7`: SF-100 resolved.
- **PO:** ☑ accepted (2026-10-26)

## Last review – C-039 (answered 2026-10-26)

The filled-in sheet is kept in git (`4ad2f96`); details in the [C-039 log, addendum 1](log/2026-10-25_C-039_config-broker.md#addendum-1--po-review-2026-10-26).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| D-1 | ADR-0019 amendment C-039 – Config def families, representation and masking, dispatch, rule layer, entry schemas, umbrellas; platform module `broker.json`, `d8` tracing exception, vector kind `broker-json` | accepted | ADR-0019 status, amendment heading, item 7 (`d7520ba`) |
| D-2 | SF-100 – content of `broker.json` | as proposed, implement the RIG text | RIG 4.1.1 `168e1d1`, register `7808df7`, re-baseline `b70eb87` (R-1 … R-3 above) |
| R-1, R-2 | REQ-RES-CONFIG-011 → `d7-monitor`, REQ-RES-CONFIG-024 +`schema` | accepted | `spectool review` (`dd5893d`) |
| R-3, R-4 | V-PAY-240/241, V-BRK-001 … 018 | all accepted | `spectool review` (`dd5893d`), gaps 16 → 15 |

## Earlier review – C-038 (answered 2026-10-25)

The filled-in sheet is kept in git (`cc198a5`); details in the [C-038 log, addendum 1](log/2026-10-25_C-038_data-event-sublist.md#addendum-1--po-review-2026-10-25).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| D-1 | ADR-0019 amendment C-038 – Oi4Pv by WriterGroupName, Data without DataSetClassId, Event relations in the schema, dispatch, REQ-RES-SUBLIST-002 split, entry schemas, umbrellas | accepted | ADR-0019 status, amendment heading and §4 pointer (`574b8e0`) |
| D-2 | Oracle: the WriterGroupName of Data/Config/Event on a topic without Filter is a Filter of the resource (REQ-MSG-024) | accepted | oracle unchanged (`d6d5010`) |
| R-1 … R-3 | V-PAY-231 … 239 (Data, Event, SubscriptionList) | all accepted | `spectool review` (`dc80c8a`), gaps 33 → 19 |
