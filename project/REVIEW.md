# Review and decisions – open items for the PO

_Last updated: 2026-10-25, after C-039 (guideline `23f6cfd`)._

This is the single list of everything that waits for the PO: items to **review** (drafts and changes to reviewed items) and items to **decide** (spec findings, open questions, cycle plan). Each cycle updates it; items leave the list once the PO has answered and `spectool review` (or the decision record) has been applied. Details are in the cycle log; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md) (same numbering).

**How to answer:** tick the box or write the decision into the "PO" field (or answer in the chat, e.g. "D-1, D-2 wie vorgeschlagen, R-1 … R-3 akzeptiert"). Anything not accepted gets a note; the agent reworks it in the next cycle.

Current state: **20 drafts** (payload vectors V-PAY-240/241, broker.json vectors V-BRK-001 … 018) and **2 changed REQs** (REQ-RES-CONFIG-011, REQ-RES-CONFIG-024), **2 decisions** (ADR amendment, spec finding SF-100). No open question (Q-011 parked). Coverage: 16 `schema`/`d7-validate` gaps now, **15 after this review** (projection); entry schemas **29 of 29**. Details: [C-039 log](log/2026-10-25_C-039_config-broker.md).

## Overview

| # | Item | Kind | Source | Recommendation | PO |
|---|------|------|--------|----------------|----|
| D-1 | ADR-0019 amendment C-039 – Config def families, representation and masking, dispatch with the Set family, rule layer, entry schemas, umbrellas; platform module `broker.json`, `d8` tracing exception, vector kind `broker-json` | decide | C-039 | accept | ☐ |
| D-2 | SF-100 – content of `broker.json` (RIG 4.1.1): (a) further elements, (b) Address syntax, (c) SecurePort 0 | decide | C-039 | as proposed | ☐ |
| R-1 | REQ-RES-CONFIG-011 verification `d7-validate` → `d7-monitor` | review | C-039 | accept | ☐ |
| R-2 | REQ-RES-CONFIG-024 verification +`schema` | review | C-039 | accept | ☐ |
| R-3 | 2 Config vectors V-PAY-240/241 | review | C-039 | accept | ☐ |
| R-4 | 18 `broker.json` vectors V-BRK-001 … 018 | review | C-039 | accept | ☐ |
| Info | Re-baseline on `23f6cfd` (MG A3 formatting fix `30fea57`, ids only) | info | C-039 | – | – |

## Decisions

### D-1 – ADR-0019 amendment C-039

- **Where:** [`ADR-0019`](decisions/ADR-0019-schema-architecture.md#amendment-c-039-2026-10-25--config-and-brokerjson-agent-proposals-within-this-adr-g-44-proposed-po-review-pending), log [§2](log/2026-10-25_C-039_config-broker.md#2-config-schemas-8a26340), [§5](log/2026-10-25_C-039_config-broker.md#5-brokerjson-1161973-3f40059), [§6](log/2026-10-25_C-039_config-broker.md#6-adr-0019-amendment-c-039-a1b68c1-proposed).
- **Change 1 – Config def families (§2):** a Pub family (complete objects, REQ-RES-CONFIG-002) and a Set family (partial update, only `Value` mandatory, REQ-RES-CONFIG-003); shared GroupName pattern, Type enumeration, representation; both closed.
- **Change 2 – representation and masking (§3):** `Value`/`DefaultValue` by `Type` (Boolean, Number, DateTime RFC 3339, ByteString Base64 without CR/LF, String); `""` valid for every Type; the mask `********` only with `Sensitive` = true; in a Pub a Sensitive value is only `""` or the mask (REQ-RES-CONFIG-024, see R-2). DateTime calendar validity is R-4. Oracle tweak: ByteString with CR/LF rejected like in the schema (`10a7d8d`).
- **Change 3 – dispatch (§4):** `message.schema.json` checks a Config DataSetMessage against the Set family (superset; Pub and Set are indistinguishable without the topic).
- **Change 4 – rule layer (§5/§8):** REQ-RES-CONFIG-008 (Value satisfies its own Validation, V-PAY-108) and REQ-RES-CONFIG-015 (unique Filter per Source, V-PAY-241) are R-3; oracle reports REQ-RES-CONFIG-015 (`4e52e5c`).
- **Change 5 – entry schemas (§8):** Get Config only PaginationRequest/Locale; Set Config exactly one DataSetMessage, no helper, no `SequenceNumber`; empty `Pub` valid (SF-099). **29 of 29** topic forms have an entry schema.
- **Change 6 – umbrellas (§9):** REQ-RES-CONFIG-010 and 018 (table in the log §1).
- **Change 7 – platform module (§7):** `platform/broker.schema.json` for REQ-CNT-003 (D8), implementing the proposal of SF-100 (D-2); patterns identical to the new oracle `internal/platform`.
- **Change 8 – tracing exception (§1):** a REQ in a `$comment` of a `platform/` file may have `d8` instead of `schema`. REQ-CNT-003 keeps `[d8]` and stays out of the `schema`/`d7-validate` coverage.
- **Change 9 – vector kind `broker-json` (§8):** `V-BRK-xxx` with the file content in `content`; lint runs the oracle, `schematest` the schema; probes P-109 … P-128.
- **Recommendation:** accept. No accepted sentence of ADR-0019 is replaced; changes 7 … 9 extend the bundle beyond messages (the platform file of RIG 4.1.1).
- **PO:** ☐ accepted ☐ other: …

### D-2 – SF-100: content of `broker.json`

- **Where:** register [`SPEC-FINDINGS.md`](../oi4-oec-documentation/findings/SPEC-FINDINGS.md) (SF-100, `68141cb`), RIG 4.1.1, log [§5](log/2026-10-25_C-039_config-broker.md#5-brokerjson-1161973-3f40059).
- **(a) Further elements:** RIG 4.1.1 lists three mandatory elements and says nothing about others. **Proposed:** allowed; clients ignore unknown elements (forward compatibility). Alternative: forbidden (closed object).
- **(b) Address syntax:** "IP address or host name". **Proposed:** IPv4 dotted-decimal (no leading zeros), IPv6 text form without brackets and zone index, or an RFC 1123 host name (labels 1 … 63 characters of letters, digits, `-`; ≤ 253 characters; no `_`, no trailing `.`) whose last label is not all digits; no port, scheme or path. Alternative: any non-empty string.
- **(c) SecurePort 0:** `<UINT16>` includes 0, which is no listening port. **Proposed:** 1 … 65535.
- **Proposed RIG text:** Address value range "`<IPv4 address, IPv6 address (without brackets) or host name (RFC 1123)>`"; SecurePort value range "`1 … 65535`"; after the element list: "Clients ignore elements of `broker.json` they do not know."
- **Effect:** schema, oracle and V-BRK vectors already implement the proposal (V-BRK-004 depends on (a), V-BRK-013 on (c), V-BRK-014 … 016 on (b)). On a different decision they are adapted. REQ-CNT-003 is reworded only after the guideline text changes (ADR-0018).
- **Recommendation:** decide as proposed and order the RIG text (or let me implement it).
- **PO:** ☐ as proposed ☐ as proposed, implement the RIG text ☐ other: …

## Reviews

### R-1 – REQ-RES-CONFIG-011 → `d7-monitor` (`draft`)

- **Where:** [`res-config.md`](../oec/spec/requirements/view/res-config.md#req-res-config-011), log [§1](log/2026-10-25_C-039_config-broker.md#1-oracle-verification-changes-umbrellas-4e52e5c-10a7d8d).
- **Change:** verification `d7-validate` → `d7-monitor`. "A configuration object with Mandatory = true is contained in every Set of its group" – the Set itself carries no Mandatory flags; only the last Pub Config of the addressed Source and Filter shows them. One message is not enough, a monitor that saw that Pub is.
- **Effect:** the REQ leaves the `schema`/`d7-validate` count (159 → 158) and its gap closes.
- **PO:** ☐ accepted ☐ other: …

### R-2 – REQ-RES-CONFIG-024 +`schema` (`draft`)

- **Where:** [`res-config.md`](../oec/spec/requirements/view/res-config.md#req-res-config-024).
- **Change:** verification `[d7-validate, d7-test]` → `[schema, d7-validate, d7-test]`. `pub-config` restricts `Value`/`DefaultValue` of a Sensitive object to `""` or `********` (probes P-93/P-94, reviewed negative V-PAY-115). `d7-test` stays for the publications after a Set.
- **PO:** ☐ accepted ☐ other: …

### R-3 – Config vectors V-PAY-240/241 (`draft`)

- **Where:** [`payload-constructed.md`](../oec/spec/vectors/view/payload-constructed.md#v-pay-240), source `oec/spec/vectors/payload/constructed.yaml`.
- **What to check:** the positive and the negative vector of REQ-RES-CONFIG-015 (unique context per Source); the negative is rule layer R-3 (expectation in `oec/schema/tests/rule-layer.yaml`).

| ✅ | Vector | Title | Violates / reqs | Base | Content |
|---|--------|-------|----------|------|--------|
| ☐ | [V-PAY-240](../oec/spec/vectors/view/payload-constructed.md#v-pay-240) | Pub all Config DataSets of an asset with two contexts | valid: CONFIG-015, CONFIG-024, … | – | `Pub/Config/<Source>` without Filter; contexts "Network settings" (DataSetWriterId 42) and "Overall device settings" (43); masked password |
| ☐ | [V-PAY-241](../oec/spec/vectors/view/payload-constructed.md#v-pay-241) | Pub Config with the same context twice for one Source | CONFIG-015 (rule) | V-PAY-240 | second context renamed to "Network settings" |

### R-4 – `broker.json` vectors V-BRK-001 … 018 (`draft`)

- **Where:** [`platform-broker-json.md`](../oec/spec/vectors/view/platform-broker-json.md) (rendered with content), source `oec/spec/vectors/platform/broker-json.yaml`.
- **What to check:** all constructed from RIG 4.1.1 (the RIG has no example); every invalid vector violates REQ-CNT-003 with exactly one defect. Vectors marked SF-100 depend on D-2.

| ✅ | Vector | Title | Result |
|---|--------|-------|--------|
| ☐ | [V-BRK-001](../oec/spec/vectors/view/platform-broker-json.md#v-brk-001) | Minimal broker.json with host name and the smallest MaxPacketSize | valid |
| ☐ | [V-BRK-002](../oec/spec/vectors/view/platform-broker-json.md#v-brk-002) | broker.json with IPv4 address and the largest MaxPacketSize | valid |
| ☐ | [V-BRK-003](../oec/spec/vectors/view/platform-broker-json.md#v-brk-003) | broker.json with IPv6 address | valid |
| ☐ | [V-BRK-004](../oec/spec/vectors/view/platform-broker-json.md#v-brk-004) | broker.json with a fully qualified host name and a further element | valid (SF-100 (a)) |
| ☐ | [V-BRK-005](../oec/spec/vectors/view/platform-broker-json.md#v-brk-005) | broker.json without Address | REQ-CNT-003 |
| ☐ | [V-BRK-006](../oec/spec/vectors/view/platform-broker-json.md#v-brk-006) | broker.json without SecurePort | REQ-CNT-003 |
| ☐ | [V-BRK-007](../oec/spec/vectors/view/platform-broker-json.md#v-brk-007) | broker.json without MaxPacketSize | REQ-CNT-003 |
| ☐ | [V-BRK-008](../oec/spec/vectors/view/platform-broker-json.md#v-brk-008) | MaxPacketSize below the minimum | REQ-CNT-003 |
| ☐ | [V-BRK-009](../oec/spec/vectors/view/platform-broker-json.md#v-brk-009) | MaxPacketSize above the maximum | REQ-CNT-003 |
| ☐ | [V-BRK-010](../oec/spec/vectors/view/platform-broker-json.md#v-brk-010) | MaxPacketSize as a string | REQ-CNT-003 |
| ☐ | [V-BRK-011](../oec/spec/vectors/view/platform-broker-json.md#v-brk-011) | SecurePort above the UINT16 range | REQ-CNT-003 |
| ☐ | [V-BRK-012](../oec/spec/vectors/view/platform-broker-json.md#v-brk-012) | SecurePort as a string | REQ-CNT-003 |
| ☐ | [V-BRK-013](../oec/spec/vectors/view/platform-broker-json.md#v-brk-013) | SecurePort 0 | REQ-CNT-003 (SF-100 (c)) |
| ☐ | [V-BRK-014](../oec/spec/vectors/view/platform-broker-json.md#v-brk-014) | Address with port | REQ-CNT-003 (SF-100 (b)) |
| ☐ | [V-BRK-015](../oec/spec/vectors/view/platform-broker-json.md#v-brk-015) | Address as a URL | REQ-CNT-003 (SF-100 (b)) |
| ☐ | [V-BRK-016](../oec/spec/vectors/view/platform-broker-json.md#v-brk-016) | Empty Address | REQ-CNT-003 (SF-100 (b)) |
| ☐ | [V-BRK-017](../oec/spec/vectors/view/platform-broker-json.md#v-brk-017) | Element names not in PascalCase | REQ-CNT-003 |
| ☐ | [V-BRK-018](../oec/spec/vectors/view/platform-broker-json.md#v-brk-018) | broker.json is no JSON object | REQ-CNT-003 |

## Information

- **Re-baseline on `23f6cfd`** (log [§4](log/2026-10-25_C-039_config-broker.md#4-re-baseline-on-23f6cfd-889c569-dd5405d)): the guideline branch head is your merge of origin with Konrad Heidrich's `30fea57` (MG A3 table, `<sub>` formatting). Inventory unchanged (1398 items), only the `baseline` ids changed (`889c569`); ADR-0001 amendment C-039 (`dd5405d`). Nothing to decide.

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
