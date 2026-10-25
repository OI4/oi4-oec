# Review and decisions – open items for the PO

_Last updated: 2026-10-25, after C-037 (guideline `a42a132`); PO answers recorded 2026-10-25: D-1, D-2, R-1, R-2 accepted._

This is the single list of everything that waits for the PO: items to **review** (drafts and changes to reviewed items) and items to **decide** (spec findings, open questions, cycle plan). Each cycle updates it; items leave the list once the PO has answered and `spectool review` (or the decision record) has been applied. Details are in the cycle log; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md) (same numbering).

**How to answer:** tick the box or write the decision into the "PO" field (or answer in the chat, e.g. "D-1, D-2 wie vorgeschlagen, R-1 und R-2 akzeptiert"). Anything not accepted gets a note; the agent reworks it in the next cycle.

Current state: **13 drafts** (payload vectors V-PAY-218 … 230), **2 decisions** (ADR amendment, oracle reading of the entry Filter). No open spec finding (next SF-100); no open question (Q-011 parked). Coverage: 48 `schema`/`d7-validate` gaps now, **35 after this review** (projection); entry schemas 18 of 29. Details: [C-037 log](log/2026-10-25_C-037_publist-refdes.md).

## Overview

| # | Item | Kind | Source | Recommendation | PO |
|---|------|------|--------|----------------|----|
| D-1 | ADR-0019 amendment C-037 – entry Filter checks, Mode forms, Set/Del counting, rule-layer additions, PL/RD dispatch | decide | C-037 | accept | ☑ accepted |
| D-2 | Oracle reading: the Filter of a PublicationList entry is topic-encoded and has the Tag structure of its resource | decide | C-037 | accept | ☑ accepted |
| R-1 | 7 PublicationList vectors V-PAY-218 … 224 | review | C-037 | accept | ☑ accepted |
| R-2 | 6 ReferenceDesignation vectors V-PAY-225 … 230 | review | C-037 | accept | ☑ accepted |

For information (no answer needed): ROADMAP/STATUS said "8 topic forms" for C-037; `topic.abnf` and the matrix have 7 (no `Del PublicationList`). Corrected in ROADMAP/STATUS; no spec item changed.

## Decisions

### D-1 – ADR-0019 amendment C-037

- **Where:** [`ADR-0019`](decisions/ADR-0019-schema-architecture.md#amendment-c-037-2026-10-25--publicationlist-and-referencedesignation-agent-proposals-within-this-adr-g-44-proposed-po-review-pending), log [§4](log/2026-10-25_C-037_publist-refdes.md#4-adr-0019-amendment-c-037-be5f045-proposed).
- **Change 1 – entry Filter (§3):** encoding → REQ-RES-PUBLIST-001; presence (only Data/Metadata/Config/Event) and Tag structure (one level; Event `<EventCategory>/<EventLevel>`) → REQ-RES-PUBLIST-002. The Event pattern restates `event-filter` of `topic.abnf`. See D-2 for the reading itself.
- **Change 2 – Mode forms (§3):** Event only `OFF_0`/`FILTER_4`, Metadata only `ON_REQUEST_FILTER_9`, MAM/Health not `OFF_0`/`ON_REQUEST_1`; a missing Mode counts as the default `ON_REQUEST_1` (REQ-RES-PUBLIST-012, as in the oracle).
- **Change 3 – Set/Del counting (§8):** Set = exactly one resource DataSetMessage via `contains` (REQ-RES-PUBLIST-022, REQ-RES-REFDES-006); Del ReferenceDesignation = no DataSetMessage (REQ-RES-REFDES-007, SF-098); no `SequenceNumber` in Set/Del (REQ-BUS-010).
- **Change 4 – rule layer (§5):** R-2 REQ-RES-REFDES-013 (Query only without Source; one ABNF rule for both topics, V-PAY-230); R-3 REQ-RES-PUBLIST-003 (uniqueness by Resource/Source/Filter, V-PAY-220); R-3 the REQ-RES-PUBLIST-002 equalities WriterGroupName = `<Resource>[/<Filter>]`, DataSetWriterName = Source (V-PAY-151 split: REQ-MSG-025 in the schema).
- **Change 5 – dispatch (§4):** `message.schema.json` selects PublicationList/ReferenceDesignation by DataSetClassId; the method-dependent counts stay in the entry schemas.
- **Recommendation:** accept. No earlier decision is reversed.
- **PO:** ☑ accepted (2026-10-25)

### D-2 – Oracle reading of the PublicationList entry Filter

- **Where:** REQ view [`res-publist.md`](../oec/spec/requirements/view/res-publist.md#req-res-publist-001), log [§1](log/2026-10-25_C-037_publist-refdes.md#1-oracle-f36c357), commit `f36c357`.
- **Reading:** MG 8.3.8 calls the Filter of an entry "the Filter of the publication" and GL-ADR-018 makes `<ResourceType>/<Filter>` the WriterGroupName. The oracle therefore requires the entry Filter to be exactly a topic Filter of the resource: topic-encoded per MG 7.7 (`+` → `%2B`, `#` → `%23`, `/` in a Tag → `%2F`, `%` → `%25`), one level for Data/Metadata/Config, `<EventCategory>/<EventLevel>` for Event.
- **Effect:** a list entry with a raw `+` or an Event entry `Status/Fine` is reported (V-PAY-219, V-PAY-223). No reviewed vector changes its verdict.
- **Alternative:** the entry Filter is the decoded Tag (unencoded); then only the structure is checked and V-PAY-219 is dropped. That would conflict with GL-ADR-018 (WriterGroupName = complete topic Filter), so no SF was opened.
- **Recommendation:** accept.
- **PO:** ☑ accepted (2026-10-25)

## Reviews

### R-1 – PublicationList vectors V-PAY-218 … 224 (`draft`)

- **Where:** [`payload-constructed.md`](../oec/spec/vectors/view/payload-constructed.md#v-pay-218) (rendered with payload), source `oec/spec/vectors/payload/constructed.yaml`.
- **What to check:** one-defect negatives on the guideline vectors V-PAY-034/037/040; `violates` is computed by the oracle. V-PAY-220 is caught by the rule layer (R-3), not by the schema. V-PAY-219/223 use the Pub topic without list filter, because their Filter could not be a valid topic Filter.

| ✅ | Vector | Title | Violates | Base | Defect |
|---|--------|-------|----------|------|--------|
| ☑ | [V-PAY-218](../oec/spec/vectors/view/payload-constructed.md#v-pay-218) | Pub PublicationList entry without Source | PUBLIST-001 | V-PAY-037 | no Source in the entry |
| ☑ | [V-PAY-219](../oec/spec/vectors/view/payload-constructed.md#v-pay-219) | Pub PublicationList entry with an unencoded Filter | PUBLIST-001 | V-PAY-037 | Filter `o+e` unencoded (topic without list filter) |
| ☑ | [V-PAY-220](../oec/spec/vectors/view/payload-constructed.md#v-pay-220) | Pub PublicationList with the same entry twice | PUBLIST-003 | V-PAY-037 | the same entry twice (rule layer R-3) |
| ☑ | [V-PAY-221](../oec/spec/vectors/view/payload-constructed.md#v-pay-221) | Get PublicationList that carries a PublicationList DataSetMessage | PUBLIST-019 | V-PAY-034 | Get PublicationList with the entry of V-PAY-040 |
| ☑ | [V-PAY-222](../oec/spec/vectors/view/payload-constructed.md#v-pay-222) | Set PublicationList without entry | PUBLIST-022 | V-PAY-040 | Set PublicationList without entry |
| ☑ | [V-PAY-223](../oec/spec/vectors/view/payload-constructed.md#v-pay-223) | Pub PublicationList Event entry with an unknown EventLevel | PUBLIST-002 | V-PAY-037 | Event entry with Filter `Status/Fine` (topic without list filter) |
| ☑ | [V-PAY-224](../oec/spec/vectors/view/payload-constructed.md#v-pay-224) | Pub PublicationList entry with a numeric Mode | MSG-031, PUBLIST-001 | V-PAY-037 | Mode `8` instead of `APPLICATION_SOURCE_FILTER_8` |

### R-2 – ReferenceDesignation vectors V-PAY-225 … 230 (`draft`)

- **Where:** [`payload-constructed.md`](../oec/spec/vectors/view/payload-constructed.md#v-pay-225).
- **What to check:** one-defect negatives on V-PAY-041/043/045/046/053. V-PAY-230 is caught by the rule layer (R-2): the payload is the valid Query of V-PAY-053, only the topic has a Source.

| ✅ | Vector | Title | Violates | Base | Defect |
|---|--------|-------|----------|------|--------|
| ☑ | [V-PAY-225](../oec/spec/vectors/view/payload-constructed.md#v-pay-225) | Pub ReferenceDesignation aspect without Value | REFDES-001 | V-PAY-043 | aspect Function without Value |
| ☑ | [V-PAY-226](../oec/spec/vectors/view/payload-constructed.md#v-pay-226) | Get ReferenceDesignation that carries a ReferenceDesignation DataSetMessage | REFDES-003 | V-PAY-041 | Get ReferenceDesignation with the object of V-PAY-045 |
| ☑ | [V-PAY-227](../oec/spec/vectors/view/payload-constructed.md#v-pay-227) | Set ReferenceDesignation without DataSetMessage | REFDES-006 | V-PAY-045 | Set ReferenceDesignation without object |
| ☑ | [V-PAY-228](../oec/spec/vectors/view/payload-constructed.md#v-pay-228) | Del ReferenceDesignation that carries a ReferenceDesignation DataSetMessage | REFDES-007 | V-PAY-046 | Del ReferenceDesignation with the object of V-PAY-045 |
| ☑ | [V-PAY-229](../oec/spec/vectors/view/payload-constructed.md#v-pay-229) | Query of ReferenceDesignation without Parent | REFDES-012 | V-PAY-053 | Query without Parent |
| ☑ | [V-PAY-230](../oec/spec/vectors/view/payload-constructed.md#v-pay-230) | Query of ReferenceDesignation on a topic with Source | REFDES-013 | V-PAY-053 | the Query on the topic with Source (V-PAY-041) |

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
