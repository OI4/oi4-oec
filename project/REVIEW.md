# Review and decisions – open items for the PO

_Last updated: 2026-10-25, after C-038 (guideline `a42a132`); PO answers recorded 2026-10-25: D-1, D-2, R-1 … R-3 accepted._

This is the single list of everything that waits for the PO: items to **review** (drafts and changes to reviewed items) and items to **decide** (spec findings, open questions, cycle plan). Each cycle updates it; items leave the list once the PO has answered and `spectool review` (or the decision record) has been applied. Details are in the cycle log; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md) (same numbering).

**How to answer:** tick the box or write the decision into the "PO" field (or answer in the chat, e.g. "D-1, D-2 wie vorgeschlagen, R-1 … R-3 akzeptiert"). Anything not accepted gets a note; the agent reworks it in the next cycle.

Current state: **9 drafts** (payload vectors V-PAY-231 … 239), **2 decisions** (ADR amendment, oracle WriterGroupName on topics without Filter). No open spec finding (next SF-100); no open question (Q-011 parked). Coverage: 33 `schema`/`d7-validate` gaps now, **19 after this review** (projection); entry schemas 26 of 29. Details: [C-038 log](log/2026-10-25_C-038_data-event-sublist.md).

## Overview

| # | Item | Kind | Source | Recommendation | PO |
|---|------|------|--------|----------------|----|
| D-1 | ADR-0019 amendment C-038 – Oi4Pv by WriterGroupName, Data without DataSetClassId, Event relations in the schema, dispatch, REQ-RES-SUBLIST-002 split, entry schemas, umbrellas | decide | C-038 | accept || ☑ accepted |
| D-2 | Oracle: the WriterGroupName of Data/Config/Event on a topic without Filter is a Filter of the resource (REQ-MSG-024) | decide | C-038 | accept || ☑ accepted |
| R-1 | 3 Data vectors V-PAY-231 … 233 | review | C-038 | accept || ☑ accepted |
| R-2 | 3 Event vectors V-PAY-234 … 236 | review | C-038 | accept || ☑ accepted |
| R-3 | 3 SubscriptionList vectors V-PAY-237 … 239 | review | C-038 | accept || ☑ accepted |

## Decisions

### D-1 – ADR-0019 amendment C-038

- **Where:** [`ADR-0019`](decisions/ADR-0019-schema-architecture.md#amendment-c-038-2026-10-25--data-event-and-subscriptionlist-agent-proposals-within-this-adr-g-44-proposed-po-review-pending), log [§2](log/2026-10-25_C-038_data-event-sublist.md#2-schemas-eb7f6d3), [§4](log/2026-10-25_C-038_data-event-sublist.md#4-adr-0019-amendment-c-038-148d77b-proposed).
- **Change 1 – Oi4Pv (§4, changes an accepted sentence):** §4 said the Oi4Pv rules are selected by the Filter/Tag in the rule layer. The WriterGroupName of a Data DataSetMessage is its Tag, so the schema selects `Oi4Pv` (required `Pv`, keys `Pv`/`Sv1` … `Sv255`, REQ-RES-DATA-004) when the WriterGroupName is `Oi4Pv`. WriterGroupName = topic Filter stays R-2.
- **Change 2 – oracle (§3):** see D-2.
- **Change 3 – Data without DataSetClassId (§4):** no const, REQ-MSG-011 does not apply; `message.schema.json` checks Data with the generic DataSetMessage (no Oi4Pv check without the topic).
- **Change 4 – Event relations in the schema (§4 step 4):** Category ↔ EventCategory (REQ-RES-EVENT-002), Status Number severity ↔ EventLevel and SymbolicId prefix (REQ-RES-EVENT-004, class 3 = Bad), Syslog PRI mod 8 ↔ EventLevel (REQ-RES-EVENT-008), with the oracle's conditions so that one defect reports one REQ.
- **Change 5 – dispatch (§4):** Event and SubscriptionList by DataSetClassId; MessageType `ua-event` for Event, `ua-keyframe` for Data and SubscriptionList.
- **Change 6 – REQ-RES-SUBLIST-002 split (§3/§5):** the concrete resource level of an OEC TopicPath in the schema (other namespaces unchecked); the WriterGroupName derived from the TopicPath stays R-3 (expectation of the reviewed V-PAY-086).
- **Change 7 – entry schemas (§8):** Get Data/SubscriptionList only PaginationRequest/Locale; Set Data/SubscriptionList and Del SubscriptionList exactly one DataSetMessage via `contains`, no `SequenceNumber`; empty `Pub` valid (SF-099).
- **Change 8 – umbrellas (§9):** REQ-RES-DATA-001/003/008, REQ-RES-EVENT-009/011, REQ-RES-SUBLIST-009 in `coverage/implications.yaml` (table in the log §1).
- **Recommendation:** accept. Only change 1 replaces an accepted sentence (moves a check from the rule layer into the schema); nothing else is reversed.
- **PO:** ☑ accepted (2026-10-25)

### D-2 – Oracle: WriterGroupName on a topic without Filter

- **Where:** REQ view [`msg.md`](../oec/spec/requirements/view/msg.md#req-msg-024), log [§1](log/2026-10-25_C-038_data-event-sublist.md#1-oracle-and-umbrellas-d6d5010), commit `d6d5010`.
- **Reading:** MG 8.2.3 makes the WriterGroupName of Data, Config and Event their topic Filter (MG 7.7). On `Pub/Data/<Source>` (and the Config/Event forms without Filter) there is no Filter to compare with; until now the oracle accepted any string there. It now requires the form of a Filter of the resource: one topic-encoded level for Data/Config, `<EventCategory>/<EventLevel>` for Event; otherwise REQ-MSG-024.
- **Effect:** no reviewed vector changes its verdict (the guideline examples V-PAY-059/060 carry the Tag `oee`). The schema patterns restate the rule (probe P-67).
- **Alternative:** keep the WriterGroupName unchecked on these topics; the schema would then need a topic-dependent pattern, which option B cannot route (one entry schema per ABNF rule).
- **Recommendation:** accept.
- **PO:** ☑ accepted (2026-10-25)

## Reviews

### R-1 – Data vectors V-PAY-231 … 233 (`draft`)

- **Where:** [`payload-constructed.md`](../oec/spec/vectors/view/payload-constructed.md#v-pay-231) (rendered with payload), source `oec/spec/vectors/payload/constructed.yaml`.
- **What to check:** V-PAY-231 is the positive Oi4Pv vector (the negative V-PAY-084 had no positive yet); the negatives have exactly one defect, `violates` computed by the oracle.

| ✅ | Vector | Title | Violates / reqs | Base | Content |
|---|--------|-------|----------|------|--------|
| ☑ | [V-PAY-231](../oec/spec/vectors/view/payload-constructed.md#v-pay-231) | Pub Data Oi4Pv with a primary and a secondary value | valid: DATA-003, DATA-004 | V-PAY-084 | `Pv` 21.4 added to `Sv1` |
| ☑ | [V-PAY-232](../oec/spec/vectors/view/payload-constructed.md#v-pay-232) | Get Data that carries a Data DataSetMessage | DATA-006 | V-PAY-055 | Get Data with the DataSetMessage of V-PAY-061 |
| ☑ | [V-PAY-233](../oec/spec/vectors/view/payload-constructed.md#v-pay-233) | Set Data with two DataSetMessages | DATA-009 | V-PAY-061 | its DataSetMessage twice |

### R-2 – Event vectors V-PAY-234 … 236 (`draft`)

- **Where:** [`payload-constructed.md`](../oec/spec/vectors/view/payload-constructed.md#v-pay-234).
- **What to check:** one-defect negatives on the guideline Events V-PAY-063 (Status/Good) and V-PAY-064 (Syslog/Notice). No Generic Event vector: REQ-RES-EVENT-009 is an umbrella over EVENT-001/002.

| ✅ | Vector | Title | Violates | Base | Defect |
|---|--------|-------|----------|------|--------|
| ☑ | [V-PAY-234](../oec/spec/vectors/view/payload-constructed.md#v-pay-234) | Pub Event without Number | EVENT-001 | V-PAY-063 | no Number |
| ☑ | [V-PAY-235](../oec/spec/vectors/view/payload-constructed.md#v-pay-235) | Status Event with a Details key other than SymbolicId | EVENT-003 | V-PAY-063 | Details key `Code` next to `SymbolicId` |
| ☑ | [V-PAY-236](../oec/spec/vectors/view/payload-constructed.md#v-pay-236) | Syslog Event with a Description | EVENT-007 | V-PAY-064 | a Description |

### R-3 – SubscriptionList vectors V-PAY-237 … 239 (`draft`)

- **Where:** [`payload-constructed.md`](../oec/spec/vectors/view/payload-constructed.md#v-pay-237).
- **What to check:** one-defect negatives on V-PAY-066/069/072.

| ✅ | Vector | Title | Violates | Base | Defect |
|---|--------|-------|----------|------|--------|
| ☑ | [V-PAY-237](../oec/spec/vectors/view/payload-constructed.md#v-pay-237) | Pub SubscriptionList entry without TopicPath | SUBLIST-001 | V-PAY-069 | no TopicPath in the entry |
| ☑ | [V-PAY-238](../oec/spec/vectors/view/payload-constructed.md#v-pay-238) | Get SubscriptionList that carries a SubscriptionList DataSetMessage | SUBLIST-007 | V-PAY-066 | Get SubscriptionList with the entry of V-PAY-072 |
| ☑ | [V-PAY-239](../oec/spec/vectors/view/payload-constructed.md#v-pay-239) | Set SubscriptionList without entry | SUBLIST-010 | V-PAY-072 | empty Messages |

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
