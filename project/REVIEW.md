# Review and decisions – open items for the PO

_Last updated: 2026-10-24, after C-035 (MetaData + LIFE roles; guideline `68d878d`)._

This is the single list of everything that waits for the PO: items to **review** (drafts and changes to reviewed items) and items to **decide** (spec findings, open questions, cycle plan). Each cycle updates it; items leave the list once the PO has answered and `spectool review` (or the decision record) has been applied. Details are in the cycle log; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md) (same numbering).

**How to answer:** tick the box or write the decision into the "PO" field (or answer in the chat, e.g. "D-1: wie vorgeschlagen, R-1 … R-4 akzeptiert"). Anything not accepted gets a note; the agent reworks it in the next cycle.

Current state: **31 drafts** (4 REQs, 27 payload vectors), **3 decisions** (ADR amendment, plan change, one method question). No open spec finding (next SF-099), no open question (Q-011 parked). Coverage: 96 `schema`/`d7-validate` gaps now, **77 after this review** (projection). Details: [C-035 log](log/2026-10-24_C-035_metadata.md).

## Overview

| # | Item | Kind | Source | Recommendation | PO |
|---|------|------|--------|----------------|----|
| D-1 | ADR-0019 amendment C-035 – `ResourceMetaData`, R-3 name uniqueness, R-6 declared role | decide | C-035 | accept | |
| D-2 | Scope change – Metadata entry schemas in C-035 instead of C-037 | decide | C-035 | accept | |
| D-3 | Topic REQs of the roles (REQ-LIFE-002/006/008) – also `d7-validate`? | decide | C-035 | (a) keep `d7-monitor`/`d7-test` | |
| R-1 | 19 MetaData vectors V-PAY-152 … 170 | review | C-035 | accept | |
| R-2 | 5 role negatives V-PAY-171 … 175 | review | C-035 | accept | |
| R-3 | V-PAY-023 … 025 with field `role` (back to `draft`) | review | C-035 | accept | |
| R-4 | Reclassifications REQ-LIFE-003/007/009 (+`d7-validate`), REQ-MSG-038 (−`schema`) (back to `draft`) | review | C-035 | accept | |

## Decisions

### D-1 – ADR-0019 amendment C-035

- **Where:** [`ADR-0019`](decisions/ADR-0019-schema-architecture.md#amendment-c-035-2026-10-24--metadata-and-declared-roles), log [§4](log/2026-10-24_C-035_metadata.md#4-adr-0019-amendment-c-035-agent-proposals-po-review).
- **Change 1 – bare MetaData → `ResourceMetaData` (§8):** §8 routes a bare MetaData vector to the generic `DataSetMetaData`. The oracle also checks that a *predefined* resource MetaData (MG B2) carries the A2 DataSetClassId of its resource (REQ-MSG-012/010). For the exact REQ comparison a schema must see this: `ResourceMetaData` = `DataSetMetaData` + one `if Name = <resource> then DataSetClassId = <A2 id>` per resource. `DataSetMetaData` stays the generic schema for vendor DataSets.
- **Change 2 – R-3 covers name uniqueness:** "names are unique" (fields REQ-MSG-038, StructureFields REQ-MSG-043, EnumFields REQ-MSG-044) is not expressible in JSON Schema (`uniqueItems` compares whole items, not one key). The rule layer R-3 checks it; expectations V-PAY-159/165/167.
- **Change 3 – new rule R-6 "declared role":** implements Q-020 (a). The caller declares `--role birth|close|will`; R-6 checks the role content (REQ-LIFE-003/007/009). The schemas cannot see the role.
- **Change 4 – §9 applied** to REQ-LIFE-003/007/009 (see R-4).
- **Recommendation:** accept. No earlier decision is reversed; all four follow from the accepted ADR and Q-020.
- **PO:** ☐ accepted ☐ other: …

### D-2 – Scope change: Metadata entry schemas in C-035

- **Where:** `ROADMAP.md` P2 (C-035, C-037), log ["Scope change"](log/2026-10-24_C-035_metadata.md#scope-change-agent-decision-allowed-by-the-po).
- **Change:** the entry schemas `Pub`/`Get`/`Set Metadata` (3 topic forms, area RES-META) were pulled from C-037 into C-035. C-037 keeps Data/Oi4Pv, Event and SubscriptionList (8 topic forms).
- **Reason:** these entry schemas only wrap the MetaData objects built in C-035 (plus the `{}` rule of REQ-RES-META-001/003/004); building them later would mean re-opening the same module. Cost in C-035: 3 schema files, V-PAY-169/170.
- **Recommendation:** accept.
- **PO:** ☐ accepted ☐ other: …

### D-3 – Topic REQs of the roles (REQ-LIFE-002/006/008)

- **Where:** REQ view [`life.md`](../oec/spec/requirements/view/life.md), log [§5](log/2026-10-24_C-035_metadata.md#5-life-roles-q-020-a-ffd45ac).
- **Problem:** the Q-020 decision named the *content* REQs REQ-LIFE-003/007/009. Each role also has a *topic* rule: the Birth/Close/Will is published under the application's own `Source` (= AppId) – REQ-LIFE-002 (`d7-monitor`), REQ-LIFE-006/008 (`d7-test`). With `--role` given, D7 `validate` could check these offline as well.
- **Option (a) – recommended:** keep REQ-LIFE-002/006/008 with `d7-monitor`/`d7-test`. Pro: no further reclassification; for a valid `Pub MAM` "Source = AppId" already follows from REQ-RES-MAM-016 + R-2 + REQ-LIFE-003, so R-6 catches the Birth case anyway; the Close/Will topic is best proven live (the broker delivers the Will). Con: the `validate --role` report does not name REQ-LIFE-006/008.
- **Option (b):** add `d7-validate` to REQ-LIFE-002/006/008; R-6 also reports them; new negatives with a foreign `Source`. Pro: complete offline report. Con: three more reclassified reviewed REQs; REQ-LIFE-008 is only partly offline-checkable (registration in every CONNECT and the Retain flag stay `d7-test`), so it would be split or carry both methods.
- **PO:** ☐ (a) ☐ (b) ☐ other: …

## Reviews

### R-1 – MetaData vectors V-PAY-152 … 170 (`draft`)

- **Where:** [`payload-constructed.md`](../oec/spec/vectors/view/payload-constructed.md#v-pay-152) (rendered with payload), source `oec/spec/vectors/payload/constructed.yaml`.
- **What to check:** V-PAY-152 is a realistic vendor `Pub Metadata` (DataSet `axis`, DataSetWriterId 95) that fills StructureDataTypes (`Range`), EnumDataTypes (`AxisMode`) and SimpleDataTypes (`Percent`) – the guideline has no such example. V-PAY-153 … 170 are one-defect negatives; `violates` is computed by the oracle. V-PAY-155 violates two REQs on purpose (a numeric StructureType is both no VerboseEncoding, REQ-MSG-034, and no EnumStructureType value, REQ-MSG-042). V-PAY-159/165/167 are caught by the rule layer (R-3), not by the schema.

| ☐ | Vector | Title | Violates | Base | Defect |
|---|--------|-------|----------|------|--------|
| ☐ | [V-PAY-152](../oec/spec/vectors/view/payload-constructed.md#v-pay-152) | Pub Metadata with structure, enumeration and simple data types | – (valid) | – | – |
| ☐ | [V-PAY-153](../oec/spec/vectors/view/payload-constructed.md#v-pay-153) | Predefined MetaData of ReferenceDesignation without DataSetClassId | MSG-012 | V-PAY-017 | bare ReferenceDesignation MetaData without DataSetClassId |
| ☐ | [V-PAY-154](../oec/spec/vectors/view/payload-constructed.md#v-pay-154) | Pub Metadata without Timestamp | MSG-032 | V-PAY-152 | DataSetMetaData without Timestamp |
| ☐ | [V-PAY-155](../oec/spec/vectors/view/payload-constructed.md#v-pay-155) | Pub Metadata with a numeric StructureType | MSG-034, MSG-042 | V-PAY-152 | StructureType as a number |
| ☐ | [V-PAY-156](../oec/spec/vectors/view/payload-constructed.md#v-pay-156) | Pub Metadata without the empty EnumDataTypes | MSG-035 | V-PAY-121 | without EnumDataTypes |
| ☐ | [V-PAY-157](../oec/spec/vectors/view/payload-constructed.md#v-pay-157) | Pub Metadata with an incomplete ConfigurationVersion | MSG-036 | V-PAY-152 | ConfigurationVersion without MinorVersion |
| ☐ | [V-PAY-158](../oec/spec/vectors/view/payload-constructed.md#v-pay-158) | Pub Metadata with the BuiltInType as name | MSG-037 | V-PAY-152 | BuiltInType `"Double"` |
| ☐ | [V-PAY-159](../oec/spec/vectors/view/payload-constructed.md#v-pay-159) | Pub Metadata with two fields of the same name | MSG-038 | V-PAY-152 | duplicate field name |
| ☐ | [V-PAY-160](../oec/spec/vectors/view/payload-constructed.md#v-pay-160) | Pub Metadata with a NodeId in the namespace index form | MSG-039 | V-PAY-152 | DataType `ns=1;i=3001` |
| ☐ | [V-PAY-161](../oec/spec/vectors/view/payload-constructed.md#v-pay-161) | Pub Metadata with a Description without Locale | MSG-040 | V-PAY-152 | Description without Locale |
| ☐ | [V-PAY-162](../oec/spec/vectors/view/payload-constructed.md#v-pay-162) | Pub Metadata with a property value that is no Variant | MSG-041 | V-PAY-152 | property Value without Variant wrapper |
| ☐ | [V-PAY-163](../oec/spec/vectors/view/payload-constructed.md#v-pay-163) | Pub Metadata with an unknown StructureType | MSG-042 | V-PAY-152 | StructureType `"Structure"` |
| ☐ | [V-PAY-164](../oec/spec/vectors/view/payload-constructed.md#v-pay-164) | Pub Metadata with a StructureField of ValueRank 0 | MSG-043 | V-PAY-152 | StructureField ValueRank 0 |
| ☐ | [V-PAY-165](../oec/spec/vectors/view/payload-constructed.md#v-pay-165) | Pub Metadata with two structure fields of the same name | MSG-043 | V-PAY-152 | duplicate StructureField name |
| ☐ | [V-PAY-166](../oec/spec/vectors/view/payload-constructed.md#v-pay-166) | Pub Metadata with an enumeration value as JSON number | MSG-044 | V-PAY-152 | EnumField Value as a number |
| ☐ | [V-PAY-167](../oec/spec/vectors/view/payload-constructed.md#v-pay-167) | Pub Metadata with two enumeration fields of the same name | MSG-044 | V-PAY-152 | duplicate EnumField name |
| ☐ | [V-PAY-168](../oec/spec/vectors/view/payload-constructed.md#v-pay-168) | Pub Metadata with a SimpleTypeDescription without BaseDataType | MSG-045 | V-PAY-152 | SimpleType without BaseDataType |
| ☐ | [V-PAY-169](../oec/spec/vectors/view/payload-constructed.md#v-pay-169) | Pub Metadata with an empty MetaData | RES-META-003 | V-PAY-152 | Pub Metadata with `MetaData` `{}` |
| ☐ | [V-PAY-170](../oec/spec/vectors/view/payload-constructed.md#v-pay-170) | Set Metadata with an empty MetaData | RES-META-004 | V-PAY-022 | Set Metadata with `MetaData` `{}` |

### R-2 – Role negatives V-PAY-171 … 175 (`draft`)

- **Where:** [`payload-constructed.md`](../oec/spec/vectors/view/payload-constructed.md#v-pay-171).
- **What to check:** each vector is a *valid* regular message (schema and oracle without role accept it) that fails its declared `role`. Each one violates only the content REQ; a Birth about another asset would also break the topic REQ REQ-LIFE-002 and is therefore covered by unit tests only (see D-3).

| ☐ | Vector | Role | Title | Violates | Base | Defect |
|---|--------|------|-------|----------|------|--------|
| ☐ | [V-PAY-171](../oec/spec/vectors/view/payload-constructed.md#v-pay-171) | birth | Birth message with two DataSetMessages | LIFE-003 | V-PAY-023 | second DataSetMessage (DataSetWriterId 56) |
| ☐ | [V-PAY-172](../oec/spec/vectors/view/payload-constructed.md#v-pay-172) | close | Close message without HealthScore | LIFE-007 | V-PAY-024 | without HealthScore |
| ☐ | [V-PAY-173](../oec/spec/vectors/view/payload-constructed.md#v-pay-173) | close | Close message with Health FAILURE_1 | LIFE-007 | V-PAY-024 | Health `FAILURE_1` |
| ☐ | [V-PAY-174](../oec/spec/vectors/view/payload-constructed.md#v-pay-174) | will | Will message with Health NORMAL_0 | LIFE-009 | V-PAY-025 | Health `NORMAL_0` |
| ☐ | [V-PAY-175](../oec/spec/vectors/view/payload-constructed.md#v-pay-175) | will | Will message with HealthScore 10 | LIFE-009 | V-PAY-025 | HealthScore 10 |

### R-3 – V-PAY-023 … 025 with field `role` (back to `draft`)

- **Where:** [`payload-guideline.md`](../oec/spec/vectors/view/payload-guideline.md#v-pay-023), source `oec/spec/vectors/payload/guideline.yaml`; commits `ffd45ac`, `9d56e45`.
- **Change:** the reviewed golden vectors of MG B1.1.1/B1.1.2 get `role: birth|close|will` and one sentence in the notes; topic and payload are unchanged. They are now the positive role vectors of REQ-LIFE-003/007/009. Lint requires the role for every vector that names one of these REQs.

| ☐ | Vector | Role | Title |
|---|--------|------|-------|
| ☐ | [V-PAY-023](../oec/spec/vectors/view/payload-guideline.md#v-pay-023) | birth | B1.1.1: Birth message of an application |
| ☐ | [V-PAY-024](../oec/spec/vectors/view/payload-guideline.md#v-pay-024) | close | B1.1.2: Close message of an application |
| ☐ | [V-PAY-025](../oec/spec/vectors/view/payload-guideline.md#v-pay-025) | will | B1.1.2: Will message of an application |

### R-4 – Reclassifications (back to `draft`)

- **Where:** REQ views [`life.md`](../oec/spec/requirements/view/life.md), [`msg.md`](../oec/spec/requirements/view/msg.md#req-msg-038).

| ☐ | REQ | Verification before → after | Reason |
|---|-----|-----------------------------|--------|
| ☐ | REQ-LIFE-003 (Birth = own MAM) | `[d7-monitor, schema]` → `[schema, d7-validate, d7-monitor]` | Q-020 (a): checkable offline with `--role birth` (R-6) |
| ☐ | REQ-LIFE-007 (Close = `NORMAL_0`, HealthScore 0) | `[d7-test, schema]` → `[schema, d7-validate, d7-test]` | Q-020 (a): `--role close` |
| ☐ | REQ-LIFE-009 (Will = `FAILURE_1`, HealthScore 0) | `[d7-test, schema]` → `[schema, d7-validate, d7-test]` | Q-020 (a): `--role will` |
| ☐ | REQ-MSG-038 (field names unique) | `[schema, d7-validate]` → `[d7-validate]` | uniqueness by key is not expressible in JSON Schema; rule layer R-3 (D-1 change 2). REQ-MSG-043/044 keep `schema` – only their uniqueness part is R-3 |

## Previous review – C-034 (answered 2026-10-24)

The filled-in sheet is kept in git (`fec3030`); details in the [C-034 log, addendum 1](log/2026-10-23_C-034_foundation-envelope.md#addendum-1--po-review-sf-098-in-the-guideline-re-baseline-on-68d878d-2026-10-24).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| D-1 | SF-098 – `Del` with zero or one DataSetMessage | as proposed; implement in the guideline | MG 8.3.11 `68d878d`, register `ff0d842`, re-baseline `4037103`; REQ-PAG-009 statement |
| D-2 | Q-020 – Birth/Close/Will roles in D7 `validate` | option (a) | OPEN-QUESTIONS (Decided), ADR-0019 §9 note; implementation in C-035 |
| D-3 | Cycle split – MetaData C-035, exit C-040 | accepted | ROADMAP P2 unchanged |
| R-1 … R-5 | vectors V-PAY-129 … 151, six re-drafted vectors, REQ-MSG-026/PAG-009/BUS-002, `implications.yaml`, ADR-0019 editorial | all accepted | `spectool review` (`8447ec5`), gaps 129 → 96 |
