# Review and decisions – open items for the PO

_Last updated: 2026-10-24, after C-034 (workspace `f8eedd5`, guideline `cc2fb4b`, register `72dcd60`)._

This is the single list of everything that waits for the PO: items to **review** (drafts and changes to reviewed items) and items to **decide** (spec findings, open questions, cycle plan). Each cycle updates it; items leave the list once the PO has answered and `spectool review` (or the decision record) has been applied. Details are in the cycle log; this file links to them.

**How to answer:** tick the box or write the decision into the "PO" field (or answer in the chat, e.g. "D-1: wie vorgeschlagen, R-1 … R-4 akzeptiert"). Anything not accepted gets a note; the agent reworks it in the next cycle.

Current state: **32 drafts** (3 REQs, 29 payload vectors), **1 open spec finding** (SF-098), **1 open question** for P2 (Q-020), 1 plan change. Nothing else is open in `oec/spec` (all other items `reviewed`).

## Overview

| # | Item | Kind | Source | Recommendation | PO |
|---|------|------|--------|----------------|----|
| D-1 | SF-098 – `Del` with zero or one DataSetMessage | decide | C-034 | "at most one, as defined for the resource" | |
| D-2 | Q-020 – Birth/Close/Will roles in D7 `validate` | decide | C-034 | option (a): vector `role` + `validate --role` | |
| D-3 | Cycle split – MetaData as own cycle C-035, P2 exit C-040 | decide | C-034 | accept | |
| R-1 | 23 new negative vectors V-PAY-129 … 151 | review | C-034 | accept | |
| R-2 | 6 reviewed vectors with extended `reqs` (back to `draft`) | review | C-034 | accept | |
| R-3 | Reclassifications REQ-MSG-026, REQ-PAG-009, REQ-BUS-002 (back to `draft`) | review | C-034 | accept | |
| R-4 | Umbrella implication map `implications.yaml` | review | C-034 | accept | |
| R-5 | ADR-0019 editorial clarifications (§1, §2) | review | C-034 | accept (no decision changed) | |

## Decisions

### D-1 – SF-098: number of DataSetMessages in a `Del`

- **Where:** `oi4-oec-documentation/findings/SPEC-FINDINGS.md` (SF-098, open, register `72dcd60`).
- **Finding:** MG 8.3.11: "… a `Set` contains a single `DataSetMessage` … and a `Del` contains no `DataSetMessage`." MG 9.1.9 (`Del SubscriptionList`): "the payload contains a single `DataSetMessages` of type `SubscriptionList`", the example carries one entry. MG 9.1.10 (`Del ReferenceDesignation`) has `"Messages": []`. MG 8.2.3 speaks of "the `DataSetMessage`" of a `Del`.
- **Proposed interpretation (recommended):** the resource rule wins. Text for MG 8.3.11: "… and a `Del` contains at most one `DataSetMessage`, as defined for the resource in 9.1." Consequence: only the statement of REQ-PAG-009 changes; REQ-RES-SUBLIST-011, REQ-RES-REFDES-007, oracle and vectors stay.
- **Alternative:** `Del SubscriptionList` without DataSetMessage (topic `Source`/`Filter` address the entry). Changes MG 9.1.9, REQ-RES-SUBLIST-011, the oracle and the reviewed `Del SubscriptionList` vectors.
- **Also decide:** shall the agent implement it in the guideline (then re-baseline), or only record the decision?
- **PO:** ☐ as proposed ☐ alternative ☐ other: … · implement in the guideline: ☐ yes ☐ later

### D-2 – Q-020: how D7 `validate` checks Birth, Close and Will (REQ-LIFE-003/007/009)

- **Where:** `project/OPEN-QUESTIONS.md` (Q-020); REQ view [`life.md`](../oec/spec/requirements/view/life.md).
- **Problem:** a Birth is a normal `Pub MAM`, a Close/Will a normal `Pub Health`. From one message the validator cannot tell the role, so REQ-LIFE-003 (Birth = own MAM), REQ-LIFE-007 (Close = `NORMAL_0`, HealthScore 0) and REQ-LIFE-009 (Will = `FAILURE_1`, HealthScore 0) cannot be checked offline. ADR-0019 §8 had listed "REQ-LIFE-003 + `d7-validate`" as a candidate; C-034 deferred it.
- **Option (a) – recommended:** payload vectors get an optional field `role` (`birth`/`close`/`will`); D7 `validate --role <role>` applies the role rules; the three REQs get `d7-validate` in addition to `d7-test`, with positive/negative vectors in C-035. Pro: offline checkable, testable with vectors, SDK developers can check their Will message before deployment. Con: one more vector field and CLI option; the role is declared by the caller, not proven.
- **Option (b):** keep REQ-LIFE-003/007/009 with `d7-test` only (live: connect, disconnect, kill). Pro: no new concepts. Con: no offline check, no vector coverage before P5.
- **PO:** ☐ (a) ☐ (b) ☐ other: …

### D-3 – Cycle split (MetaData → C-035)

- **Where:** `project/ROADMAP.md` P2, log [C-034 "Split"](log/2026-10-23_C-034_foundation-envelope.md#split-agent-decision-allowed-by-the-po).
- **Change:** C-034 delivered foundation + `ua-data` envelope. The bare MetaData (`ua-metadata`, module `metadata`, REQ-MSG-012, REQ-MSG-032 … 045, gap REQs REQ-MSG-042/043/045, 8 bare MetaData vectors) becomes **C-035**. Later cycles shift by one: C-036 Actions + core resources · C-037 Data/Metadata/Event/SubscriptionList · C-038 Config + `broker.json` · C-039 D7 `validate` · **C-040 P2 exit review**.
- **Reason:** MetaData is its own type system and would roughly double the schema and vector volume of C-034 (already 29 vector drafts to review).
- **PO:** ☐ accepted ☐ other: …

## Reviews

### R-1 – New negative vectors V-PAY-129 … 151 (`draft`)

- **Where:** [`payload-constructed.md`](../oec/spec/vectors/view/payload-constructed.md#v-pay-129) (rendered with payload), source `oec/spec/vectors/payload/constructed.yaml`.
- **What to check:** each vector is a reviewed valid vector with exactly one defect; `violates` is computed by the oracle; title, reason and notes are correct and the defect is a realistic mistake.
- Abbreviations: topic `…/` = `Oi4v2/OTConnector/<AppId>/`; base = the valid vector it is derived from.

| ☐ | Vector | Title | Violates | Base | Defect (notes) |
|---|--------|-------|----------|------|----------------|
| ☐ | [V-PAY-129](../oec/spec/vectors/view/payload-constructed.md#v-pay-129) | Pub Health with the MessageType ua-metadata | MSG-002 | V-PAY-003 | NetworkMessage `MessageType` `ua-metadata` |
| ☐ | [V-PAY-130](../oec/spec/vectors/view/payload-constructed.md#v-pay-130) | Pub Health without ReplyTo | MSG-003 | V-PAY-003 | field `ReplyTo` removed |
| ☐ | [V-PAY-131](../oec/spec/vectors/view/payload-constructed.md#v-pay-131) | Pub Health with Messages as an object | MSG-005 | V-PAY-003 | `Messages` is the DataSetMessage itself, not an array |
| ☐ | [V-PAY-132](../oec/spec/vectors/view/payload-constructed.md#v-pay-132) | Pub Health with a PublisherId without ServiceType | MSG-006 | V-PAY-003 | `PublisherId` `provider.com/FieldDataService/FDS-001/1200-0345` (MessageId unchanged; suffix compared only for a well-formed PublisherId) |
| ☐ | [V-PAY-133](../oec/spec/vectors/view/payload-constructed.md#v-pay-133) | Pub Health with a MessageId without timestamp | MSG-007 | V-PAY-003 | `MessageId` = PublisherId only |
| ☐ | [V-PAY-134](../oec/spec/vectors/view/payload-constructed.md#v-pay-134) | Pub Health without DataSetClassId | MSG-011 | V-PAY-003 | `DataSetClassId` removed |
| ☐ | [V-PAY-135](../oec/spec/vectors/view/payload-constructed.md#v-pay-135) | Pub Health with a ReplyTo that is no MessageId | MSG-015 | V-PAY-003 | `ReplyTo` `"1639693510000"` (timestamp without PublisherId) |
| ☐ | [V-PAY-136](../oec/spec/vectors/view/payload-constructed.md#v-pay-136) | Pub Health with a PublisherId in the DataSetMessage | MSG-016 | V-PAY-003 | DataSetMessage repeats the `PublisherId` |
| ☐ | [V-PAY-137](../oec/spec/vectors/view/payload-constructed.md#v-pay-137) | Pub Health with MetaDataVersion and MinorVersion | MSG-018 | V-PAY-118 | `MinorVersion` 0 next to `MetaDataVersion` 1.0 |
| ☐ | [V-PAY-138](../oec/spec/vectors/view/payload-constructed.md#v-pay-138) | Pub Health with a Pagination under a wrong WriterGroupName | MSG-019 | V-PAY-003 | Pagination (DSWId 2) with `WriterGroupName` `Paging` |
| ☐ | [V-PAY-139](../oec/spec/vectors/view/payload-constructed.md#v-pay-139) | Pub Health with the reserved DataSetWriterId 5 | MSG-020 | V-PAY-003 | Health `DataSetWriterId` 5 (4 … 8 reserved) |
| ☐ | [V-PAY-140](../oec/spec/vectors/view/payload-constructed.md#v-pay-140) | Pub Health with a WriterGroupName | MSG-024 | V-PAY-003 | `WriterGroupName` `Health` (Health has no Filter → `""`) |
| ☐ | [V-PAY-141](../oec/spec/vectors/view/payload-constructed.md#v-pay-141) | Pub Health with a Good Status | MSG-026 | V-PAY-118 | `Status` `{Code: 0, Symbol: Good}` |
| ☐ | [V-PAY-142](../oec/spec/vectors/view/payload-constructed.md#v-pay-142) | Pub Health with a Status without Code | MSG-027 | V-PAY-118 | `Status` with `Symbol` only |
| ☐ | [V-PAY-143](../oec/spec/vectors/view/payload-constructed.md#v-pay-143) | Pub Health with the DataSetMessage MessageType ua-event | MSG-029 | V-PAY-118 | `MessageType` `ua-event` (only for Event) |
| ☐ | [V-PAY-144](../oec/spec/vectors/view/payload-constructed.md#v-pay-144) | Pub Health as a delta frame | MSG-029, MSG-030 | V-PAY-118 | `MessageType` `ua-deltaframe` |
| ☐ | [V-PAY-145](../oec/spec/vectors/view/payload-constructed.md#v-pay-145) | Pub Health with a numeric enumeration value | MSG-031, RES-HEALTH-001 | V-PAY-003 | `"Health": 0` instead of `"NORMAL_0"` |
| ☐ | [V-PAY-146](../oec/spec/vectors/view/payload-constructed.md#v-pay-146) | Get Health with an invalid Locale | LOC-001 | V-PAY-001 | Locale (DSWId 3) `German` |
| ☐ | [V-PAY-147](../oec/spec/vectors/view/payload-constructed.md#v-pay-147) | Get Health with a Query | QRY-001 | V-PAY-001 | Query (DSWId 9) with `Parent` of ReferenceDesignation |
| ☐ | [V-PAY-148](../oec/spec/vectors/view/payload-constructed.md#v-pay-148) | Pub Health whose DataSetWriterName differs from the Source | MSG-023 | V-PAY-003 | `DataSetWriterName` names another asset; **rule layer only** (R-2, expectation list `oec/schema/tests/rule-layer.yaml`) |
| ☐ | [V-PAY-149](../oec/spec/vectors/view/payload-constructed.md#v-pay-149) | Set Data with a Pagination | PAG-008, PAG-009 | V-PAY-061 | single-page Pagination (DSWId 2) after the Data DataSetMessage |
| ☐ | [V-PAY-150](../oec/spec/vectors/view/payload-constructed.md#v-pay-150) | Set Data with a SequenceNumber | BUS-010 | V-PAY-061 | `SequenceNumber` 12 (removed from the guideline example by SF-079) |
| ☐ | [V-PAY-151](../oec/spec/vectors/view/payload-constructed.md#v-pay-151) | Pub PublicationList entry with the Filter as WriterGroupName | MSG-025, RES-PUBLIST-002 | V-PAY-037 | entry `WriterGroupName` `oee` instead of `Data/oee` |

**PO:** ☐ all accepted ☐ accepted except: …

### R-2 – Reviewed vectors with extended `reqs` (back to `draft`)

- **Where:** [`payload-guideline.md`](../oec/spec/vectors/view/payload-guideline.md#v-pay-003), source `oec/spec/vectors/payload/guideline.yaml`.
- **Change:** only the list `reqs` (the REQs the valid vector demonstrates) grew; topic, payload and expectation are unchanged. Reason: these REQs had only negative vectors; the vectors already satisfy them, so they provide the positive vector for the P2 exit.

| ☐ | Vector | Title | Added to `reqs` |
|---|--------|-------|-----------------|
| ☐ | [V-PAY-003](../oec/spec/vectors/view/payload-guideline.md#v-pay-003) | B1.1.2: Pub Health of an explicit asset, reply to a Get | REQ-MSG-001, REQ-MSG-014 |
| ☐ | [V-PAY-005](../oec/spec/vectors/view/payload-guideline.md#v-pay-005) | B1.1.11: Get MAM of all assets with PaginationRequest | REQ-PAG-001, REQ-PAG-008 |
| ☐ | [V-PAY-006](../oec/spec/vectors/view/payload-guideline.md#v-pay-006) | B1.1.11: Pub MAM of all assets with Pagination | REQ-PAG-003, REQ-PAG-004, REQ-PAG-008 |
| ☐ | [V-PAY-007](../oec/spec/vectors/view/payload-guideline.md#v-pay-007) | B1.1.12: Get MAM of all assets with Locale | REQ-LOC-003 |
| ☐ | [V-PAY-046](../oec/spec/vectors/view/payload-guideline.md#v-pay-046) | B1.1.10: Del ReferenceDesignation of an explicit asset | REQ-PAG-009 |
| ☐ | [V-PAY-061](../oec/spec/vectors/view/payload-guideline.md#v-pay-061) | B1.1.4: Set explicit Data tag of an explicit asset | REQ-PAG-009 |

**PO:** ☐ all accepted ☐ accepted except: …

### R-3 – Reclassified requirements (back to `draft`)

| ☐ | REQ | Title | Verification before → after | Reason |
|---|-----|-------|-----------------------------|--------|
| ☐ | [REQ-MSG-026](../oec/spec/requirements/view/msg.md#req-msg-026) | Status is only sent if not Good | `d7-validate` → `schema, d7-validate` | `Status.Code` 0 (Good) is visible in the single payload; the envelope schema rejects it (ADR-0019 §8 candidate). Negative vector V-PAY-141. |
| ☐ | [REQ-PAG-009](../oec/spec/requirements/view/pag.md#req-pag-009) | Set and Del are not paginated | `d7-validate` → `schema, d7-validate` | A PaginationRequest/Pagination in a `Set`/`Del` is visible from payload + topic method; the entry schemas reject DSWId 1 and 2 (ADR-0019 §8 candidate). Note points to SF-098 (D-1); `MaxPacketSize` not checked. Negative vector V-PAY-149. |
| ☐ | [REQ-BUS-002](../oec/spec/requirements/view/bus.md#req-bus-002) | A requester publishes on the request topic of the addressed component | `d7-validate` → `d7-test` | One message shows the topic's ServiceType/AppId, but not whether they belong to the addressed component; D7 `test` checks it against the component under test. No negative vector is possible offline. |

**PO:** ☐ all accepted ☐ accepted except: …

### R-4 – Umbrella implication map

- **Where:** `oec/spec/coverage/implications.yaml`, report [`coverage/vectors.md`](../oec/spec/coverage/vectors.md). Rule: ADR-0019 §9 (accepted) – an umbrella REQ counts as covered by vectors when all its details are covered; the remainder stays with the other verification methods.

| ☐ | Umbrella | Details | Remainder |
|---|----------|---------|-----------|
| ☐ | REQ-MSG-004 (NetworkMessage content mask) | MSG-003, MSG-005, MSG-011 | none – the mask is not on the wire; its effect is the field presence |
| ☐ | REQ-MSG-017 (DataSetMessage content mask) | MSG-016, MSG-018, MSG-024, MSG-029, MSG-031 | none – field presence and VerboseEncoding |
| ☐ | REQ-BUS-004 (`Set` content) | BUS-010, MSG-030, TOP-013, RES-CONFIG-003, RES-CONFIG-019, RES-DATA-009, RES-META-004, RES-PUBLIST-022, RES-REFDES-006, RES-SUBLIST-010 | object belongs to the addressed component, complete updated state → `d7-test` (REQ-BUS-002) |
| ☐ | REQ-BUS-005 (`Del` content) | BUS-010, TOP-013, RES-REFDES-007, RES-SUBLIST-011 | addressed object exists at the component → `d7-test` (REQ-BUS-002) |

The maps of REQ-CMP-004/005/025/026, REQ-TOP-002 and REQ-IL-001/008 follow in C-039 (D7 `validate`).

**PO:** ☐ accepted ☐ other: …

### R-5 – ADR-0019 editorial clarifications

- **Where:** [`decisions/ADR-0019-schema-architecture.md`](decisions/ADR-0019-schema-architecture.md) §1, §2, status line. No decision changed.
- §1 **Type guards:** only *predicates* with `contains` state `"type": "array"`; an asserting `contains`/`maxContains` must not (otherwise a non-array `Messages` reports a secondary REQ-PAG-008/LOC-003 next to REQ-MSG-005).
- §1 **One REQ per constraint object (new):** santhosh-tekuri/jsonschema stops evaluating a schema object after a failed `type`/`const`/`enum`. When one value can violate two REQs (`ua-deltaframe`: MSG-029 + MSG-030; `"Health": 0`: RES-HEALTH-001 + MSG-031), each constraint is its own `allOf` item.
- §1 **Reference lists (new):** the MG refs in parentheses of a `$comment` are refs of the named REQs; the harness checks REQ ids and refs against the catalog.
- §2: Health moved to `v2/resources/` (done).

**PO:** ☐ accepted

## Not open, for information

- **Coverage after this review** (projection): the `schema`/`d7-validate` gaps fall from 129 to 96; MSG (except MetaData), PAG, LOC, QRY and REQ-BUS-010 are covered, REQ-MSG-004/017 derived.
- **Checks:** all `spectool` checks green; santhosh, jsonschema-rs and Python `jsonschema` agree on 172 schema cases.
- **Parked:** Q-011 (publication/CRA) – working assumption private GitHub repos; not needed for P2.

## After the review

The agent runs `spectool review <accepted ids>`, records the answers in the C-034 log (addendum), applies D-1 … D-3, updates this file and starts C-035 (MetaData).
