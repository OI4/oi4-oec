# Review and decisions – open items for the PO

_Last updated: 2026-10-25, PO answers for C-036 recorded (D-1 decided differently, everything else accepted); guideline `68d878d`._

This is the single list of everything that waits for the PO: items to **review** (drafts and changes to reviewed items) and items to **decide** (spec findings, open questions, cycle plan). Each cycle updates it; items leave the list once the PO has answered and `spectool review` (or the decision record) has been applied. Details are in the cycle log; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md) (same numbering).

**How to answer:** tick the box or write the decision into the "PO" field (or answer in the chat, e.g. "D-1: wie vorgeschlagen, R-1 … R-4 akzeptiert"). Anything not accepted gets a note; the agent reworks it in the next cycle.

Current state: **51 drafts** (42 payload vectors, 1 topic vector, 8 reviewed guideline vectors with extended `reqs`), **4 decisions** (one spec finding, ADR amendment, cycle split, one heuristic). Open spec finding: SF-099 (next SF-100); no open question (Q-011 parked). Coverage: 81 `schema`/`d7-validate` gaps now, **48 after this review** (projection). Details: [C-036 log](log/2026-10-25_C-036_actions-mam-profile.md).

## Overview

| # | Item | Kind | Source | Recommendation | PO |
|---|------|------|--------|----------------|----|
| D-1 | SF-099 – empty `Pub` as the answer to a `Get` without content | decide | C-036 | as proposed; implement in the guideline on PO order | ✅ decided differently: no content → empty list; unknown asset → no answer, SHOULD Event Status Bad |
| D-2 | ADR-0019 amendment C-036 – empty `Pub`, umbrellas, Action dispatch and closures, rule-layer additions | decide | C-036 | accept | ✅ accepted |
| D-3 | Cycle split – PublicationList/ReferenceDesignation → new C-037, exit C-041 | decide | C-036 | accept | ✅ accepted |
| D-4 | REQ-RES-MAM-006 checked heuristically (false-positive risk) | decide | C-036 | accept | ✅ accepted |
| R-1 | 28 Action vectors V-PAY-176 … 203 | review | C-036 | accept | ✅ accepted |
| R-2 | 14 MAM/Profile vectors V-PAY-204 … 217 | review | C-036 | accept | ✅ accepted |
| R-3 | Topic vector V-TOP-253 (REQ-ACT-020) | review | C-036 | accept | ✅ accepted |
| R-4 | `reqs` of 8 reviewed guideline vectors extended (back to `draft`) | review | C-036 | accept | ✅ accepted |

## Decisions

### D-1 – SF-099: empty `Pub` as the answer to a `Get` without content

- **Where:** register [`SPEC-FINDINGS.md`](../oi4-oec-documentation/findings/SPEC-FINDINGS.md) (SF-099, `29682b0`), log [§2](log/2026-10-25_C-036_actions-mam-profile.md#2-sf-099--empty-pub-of-a-resource-register-29682b0-open).
- **Problem:** MG 9.1.x says for every resource "the `Pub` payload contains DataSetMessages of type `<Resource>` and optional Pagination". MG 9.1 "Resources Without Content" demands `"Messages": []` as the answer to a `Get` without content (REQ-CMP-041). Read literally, 9.1.x forbids that answer. Also unstated: how a `Get` for an unknown `Source`/`Filter` is answered.
- **Proposed interpretation:** "contains DataSetMessages of type X" names the kind, not a minimum. An empty `Pub` is valid as the answer to a `Get` without content, also for an unknown `Source`/`Filter`. An unsolicited empty `Pub` stays forbidden (REQ-CMP-042). Proposed guideline text: in 9.1 "Resources Without Content", first bullet: "This also applies to a `Get` whose `Source` or `Filter` addresses no content of the entity."; in 9.1.x for `Pub`: "… contains `DataSetMessages` of type `<Resource>` – none in the answer to a `Get` without content (see Resources Without Content) – and optional `Pagination`".
- **Alternative:** an unknown `Source` is not answered (the requester times out). The schemas are the same; only the register/guideline text differs.
- **Already applied (reversible):** the `pub-<resource>` entry schemas accept `"Messages": []`; the C-033 `anyOf` of `pub-health` is removed. No REQ statement changes.
- **Recommendation:** as proposed; on PO order implement it in the guideline (MG 9.1, 9.1.x) and re-baseline.
- **PO:** ☐ as proposed ☐ as proposed + implement in the guideline ☐ alternative X other: "Wenn Get angefragt wird und es gibt nichts, dann leere Liste. Wenn es das Asset nicht gibt, kommt keine Antwort und should für ein Event mit Fehler. Das Event-Verhalten sollte bereits beschrieben sein." (2026-10-25)

### D-2 – ADR-0019 amendment C-036

- **Where:** [`ADR-0019`](decisions/ADR-0019-schema-architecture.md#amendment-c-036-2026-10-25--actions-and-core-resources-agent-proposals-within-this-adr-g-44-proposed-po-review-pending), log [§7](log/2026-10-25_C-036_actions-mam-profile.md#7-adr-0019-amendment-c-036-agent-proposals-po-review).
- **Change 1 – empty `Pub`:** see D-1.
- **Change 2 – umbrellas (§9):** REQ-RES-MAM-022, REQ-RES-HEALTH-007, REQ-RES-PROFILE-005, REQ-RES-PUBLIST-021, REQ-RES-REFDES-005 ("a `Pub` contains DataSetMessages of type X and optional Pagination") become umbrellas of REQ-MSG-010 + the content REQ + REQ-PAG-008 + REQ-LOC-003 + REQ-QRY-001 in `coverage/implications.yaml` (`e686d3a`). Remainder: REQ-CMP-042 (`d7-monitor`). The same pattern for REQ-RES-CONFIG-018/DATA-008/SUBLIST-009 is decided in their cycles.
- **Change 3 – Action dispatch (§4):** `message.schema.json` selects Action messages by `MessageType`, then the NewDataSetWriterId fields by the reserved DataSetWriterId 8. A publisher-defined Action is checked structurally only.
- **Change 4 – Action closures (§3):** an unknown key in any Action object incl. `Status` → REQ-ACT-003; an undefined field, wrong type or out-of-range value in the NewDataSetWriterId Payload → REQ-ACT-013; missing/semantic → REQ-ACT-021/022. Technical note on `$ref` + `unevaluatedProperties`.
- **Change 5 – rule layer (§5):** R-2 REQ-ACT-005/006, REQ-RES-MAM-014; R-3 REQ-ACT-004, REQ-RES-MAM-016, REQ-RES-MAM-006 (heuristic, D-4), expectation V-PAY-050 (REQ-PAG-004).
- **Change 6 – cycle numbers** after the split (D-3).
- **Recommendation:** accept. No earlier decision is reversed; the C-033 `anyOf` of `pub-health` was part of the PoC, not of the decision.
- **PO:** X accepted ☐ other: …

### D-3 – Cycle split: PublicationList/ReferenceDesignation → new C-037

- **Where:** `ROADMAP.md` P2, log ["Split"](log/2026-10-25_C-036_actions-mam-profile.md#split-agent-decision-allowed-by-the-po).
- **Change:** C-036 = Actions + MAM, Health, Profile. New C-037 = PublicationList + ReferenceDesignation (8 topic forms). Then C-038 Data/Event/SubscriptionList, C-039 Config + `broker.json`, C-040 D7 `validate`, C-041 P2 exit review.
- **Reason:** ACT alone needed 28 vectors and a new envelope branch; PublicationList and ReferenceDesignation are each about as large as MAM + Profile. Without the split the review sheet would have held about 100 drafts.
- **Recommendation:** accept.
- **PO:** X accepted ☐ other: …

### D-4 – REQ-RES-MAM-006 checked heuristically

- **Where:** REQ view [`res-mam.md`](../oec/spec/requirements/view/res-mam.md#req-res-mam-006), log [§1](log/2026-10-25_C-036_actions-mam-profile.md#1-oracle-298e709).
- **Problem:** REQ-RES-MAM-006: when the ProductInstanceUri is a *generated* Oi4Identifier, its elements are the MAM's Model (en-US), ProductCode and SerialNumber. One message does not say whether the identifier was generated.
- **Heuristic (oracle and rule layer R-3):** a ProductInstanceUri without scheme, with four segments and the ManufacturerUri as first segment counts as generated. Then segments 2 … 4 must be Model, ProductCode, SerialNumber (plain or dnp-encoded, `nd` for empty). Anything else is not checked.
- **Risk:** a vendor identifier of exactly this shape that was not generated from the MAM is reported (false positive). The REQ is a MUST, so D7 would report it as a failure; D7 `validate` (C-040) names the heuristic in its message.
- **Alternative:** keep REQ-RES-MAM-006 out of `d7-validate` (`d7-test` only); then it stays a gap REQ for the P2 exit and V-PAY-210 is dropped.
- **Recommendation:** accept the heuristic.
- **PO:** X heuristic accepted ☐ alternative ☐ other: …

## Reviews

### R-1 – Action vectors V-PAY-176 … 203 (`draft`)

- **Where:** [`payload-constructed.md`](../oec/spec/vectors/view/payload-constructed.md#v-pay-176) (rendered with payload), source `oec/spec/vectors/payload/constructed.yaml`.
- **What to check:** one-defect negatives on the guideline's NewDataSetWriterId Call (V-PAY-008) and Reply (V-PAY-009); `violates` is computed by the oracle. V-PAY-182/184/186 are caught by the rule layer, not by the schema. V-PAY-203 is a positive: a Reply rejected with `BadInvalidArgument` (Code 2158690304 = 0x80AB0000) and an empty Payload.

| ✅ | Vector | Title | Violates | Base | Defect |
|---|--------|-------|----------|------|--------|
| ✅ | [V-PAY-176](../oec/spec/vectors/view/payload-constructed.md#v-pay-176) | Call with the MessageType ua-action-response | ACT-001 | V-PAY-008 | MessageType `ua-action-response` in a Call |
| ✅ | [V-PAY-177](../oec/spec/vectors/view/payload-constructed.md#v-pay-177) | Call without TimeoutHint | ACT-002 | V-PAY-008 | no TimeoutHint |
| ✅ | [V-PAY-178](../oec/spec/vectors/view/payload-constructed.md#v-pay-178) | Reply with a ResponseAddress | ACT-002 | V-PAY-009 | ResponseAddress in a Reply |
| ✅ | [V-PAY-179](../oec/spec/vectors/view/payload-constructed.md#v-pay-179) | Call with a DataSetClassId | ACT-003 | V-PAY-008 | DataSetClassId in a Call |
| ✅ | [V-PAY-180](../oec/spec/vectors/view/payload-constructed.md#v-pay-180) | Reply with a DataSetWriterName in the ActionResponse | ACT-003 | V-PAY-009 | DataSetWriterName in an ActionResponse item |
| ✅ | [V-PAY-181](../oec/spec/vectors/view/payload-constructed.md#v-pay-181) | Call with a MessageId without timestamp | ACT-004 | V-PAY-008 | MessageId without timestamp |
| ✅ | [V-PAY-182](../oec/spec/vectors/view/payload-constructed.md#v-pay-182) | Call with a MessageId of the Responder | ACT-004 | V-PAY-008 | sender in the MessageId = Responder (rule R-3) |
| ✅ | [V-PAY-183](../oec/spec/vectors/view/payload-constructed.md#v-pay-183) | Call with a RequestorId without ServiceType | ACT-005 | V-PAY-008 | RequestorId without ServiceType |
| ✅ | [V-PAY-184](../oec/spec/vectors/view/payload-constructed.md#v-pay-184) | Reply from another application than the Responder of the topic | ACT-005 | V-PAY-009 | PublisherId …1200-0346 ≠ Responder of the topic (rule R-2) |
| ✅ | [V-PAY-185](../oec/spec/vectors/view/payload-constructed.md#v-pay-185) | Call with a ResponseAddress of the Method Pub | ACT-006 | V-PAY-008 | ResponseAddress with Method `Pub` |
| ✅ | [V-PAY-186](../oec/spec/vectors/view/payload-constructed.md#v-pay-186) | Call with the Reply topic of another application as ResponseAddress | ACT-006 | V-PAY-008 | ResponseAddress = Reply topic of another application (rule R-2) |
| ✅ | [V-PAY-187](../oec/spec/vectors/view/payload-constructed.md#v-pay-187) | Call with ActionState 2 | ACT-010 | V-PAY-008 | ActionState 2 in a request |
| ✅ | [V-PAY-188](../oec/spec/vectors/view/payload-constructed.md#v-pay-188) | Call without RequestId | ACT-010 | V-PAY-008 | no RequestId |
| ✅ | [V-PAY-189](../oec/spec/vectors/view/payload-constructed.md#v-pay-189) | Reply with ActionState 0 | ACT-012 | V-PAY-009 | ActionState 0 |
| ✅ | [V-PAY-190](../oec/spec/vectors/view/payload-constructed.md#v-pay-190) | Reply with a Bad Status and response fields | ACT-012 | V-PAY-009 | Bad Status with Payload fields |
| ✅ | [V-PAY-191](../oec/spec/vectors/view/payload-constructed.md#v-pay-191) | Call NewDataSetWriterId with a numeric Resource | ACT-013 | V-PAY-008 | Resource `3` |
| ✅ | [V-PAY-192](../oec/spec/vectors/view/payload-constructed.md#v-pay-192) | Reply NewDataSetWriterId with Ttl as JSON string | ACT-013 | V-PAY-009 | Ttl `"10"` |
| ✅ | [V-PAY-193](../oec/spec/vectors/view/payload-constructed.md#v-pay-193) | Call NewDataSetWriterId with an unknown request field | ACT-013 | V-PAY-008 | request field `Name` |
| ✅ | [V-PAY-194](../oec/spec/vectors/view/payload-constructed.md#v-pay-194) | Reply NewDataSetWriterId with a DataSetWriterId above UInt16 | ACT-013 | V-PAY-009 | DataSetWriterId 70000 |
| ✅ | [V-PAY-195](../oec/spec/vectors/view/payload-constructed.md#v-pay-195) | Reply with the Good Status as Code 0 | ACT-014 | V-PAY-009 | Status `{"Code": 0}` |
| ✅ | [V-PAY-196](../oec/spec/vectors/view/payload-constructed.md#v-pay-196) | Reply with a Status Symbol | ACT-014 | V-PAY-009 | Status with Symbol |
| ✅ | [V-PAY-197](../oec/spec/vectors/view/payload-constructed.md#v-pay-197) | Call NewDataSetWriterId with the DataSetWriterId 7 | ACT-019 | V-PAY-008 | DataSetWriterId 7 |
| ✅ | [V-PAY-198](../oec/spec/vectors/view/payload-constructed.md#v-pay-198) | Reply NewDataSetWriterId with the ActionTargetId 2 | ACT-019 | V-PAY-009 | ActionTargetId 2 |
| ✅ | [V-PAY-199](../oec/spec/vectors/view/payload-constructed.md#v-pay-199) | Call NewDataSetWriterId for an unknown Resource | ACT-021 | V-PAY-008 | Resource `Status` |
| ✅ | [V-PAY-200](../oec/spec/vectors/view/payload-constructed.md#v-pay-200) | Call NewDataSetWriterId without Resource | ACT-021 | V-PAY-008 | Payload `{}` |
| ✅ | [V-PAY-201](../oec/spec/vectors/view/payload-constructed.md#v-pay-201) | Reply NewDataSetWriterId with the reserved DataSetWriterId 8 | ACT-022 | V-PAY-009 | DataSetWriterId 8 |
| ✅ | [V-PAY-202](../oec/spec/vectors/view/payload-constructed.md#v-pay-202) | Reply NewDataSetWriterId without Ttl | ACT-022 | V-PAY-009 | no Ttl |
| ✅ | [V-PAY-203](../oec/spec/vectors/view/payload-constructed.md#v-pay-203) | Reply NewDataSetWriterId rejected with BadInvalidArgument | – (valid) | V-PAY-009 | – |

### R-2 – MAM and Profile vectors V-PAY-204 … 217 (`draft`)

- **Where:** [`payload-constructed.md`](../oec/spec/vectors/view/payload-constructed.md#v-pay-204).
- **What to check:** bases are the guideline MAM (V-PAY-028; V-PAY-029 without `Source` for V-PAY-212), Profile (V-PAY-032) and Get Profile (V-PAY-030). V-PAY-210/211/213 are rule-layer cases. V-PAY-214 keeps `Health` in the list because the oracle requires MAM, Health and Profile (REQ-RES-PROFILE-002, `d7-test`).

| ✅ | Vector | Title | Violates | Base | Defect |
|---|--------|-------|----------|------|--------|
| ✅ | [V-PAY-204](../oec/spec/vectors/view/payload-constructed.md#v-pay-204) | Pub MAM without SerialNumber | RES-MAM-001 | V-PAY-028 | no SerialNumber |
| ✅ | [V-PAY-205](../oec/spec/vectors/view/payload-constructed.md#v-pay-205) | Pub MAM with a RevisionCounter above Int32 | RES-MAM-001 | V-PAY-028 | RevisionCounter 2147483648 |
| ✅ | [V-PAY-206](../oec/spec/vectors/view/payload-constructed.md#v-pay-206) | Pub MAM with a ManufacturerUri with scheme | RES-MAM-004 | V-PAY-028 | `https://provider.com` |
| ✅ | [V-PAY-207](../oec/spec/vectors/view/payload-constructed.md#v-pay-207) | Pub MAM with an upper-case ManufacturerUri | RES-MAM-004 | V-PAY-028 | `Provider.com` |
| ✅ | [V-PAY-208](../oec/spec/vectors/view/payload-constructed.md#v-pay-208) | Pub MAM with a DeviceManual without scheme | RES-MAM-012 | V-PAY-028 | `www…` without scheme |
| ✅ | [V-PAY-209](../oec/spec/vectors/view/payload-constructed.md#v-pay-209) | Pub MAM with a DeviceManual without host | RES-MAM-012 | V-PAY-028 | `file:///…` without host |
| ✅ | [V-PAY-210](../oec/spec/vectors/view/payload-constructed.md#v-pay-210) | Pub MAM whose ProductInstanceUri does not match its ProductCode | RES-MAM-006 | V-PAY-028 | ProductCode `FDS-002` ≠ element of the Oi4Identifier (rule R-3) |
| ✅ | [V-PAY-211](../oec/spec/vectors/view/payload-constructed.md#v-pay-211) | Pub MAM whose ProductInstanceUri differs from the DataSetWriterName | RES-MAM-016 | V-PAY-028 | ProductInstanceUri `https://provider.com/FDS-001/1200-0345` (rule R-3) |
| ✅ | [V-PAY-212](../oec/spec/vectors/view/payload-constructed.md#v-pay-212) | Pub MAM with a ProductInstanceUri of 256 characters | RES-MAM-017 | V-PAY-029 | ProductInstanceUri and DataSetWriterName with 256 characters, topic without `Source` |
| ✅ | [V-PAY-213](../oec/spec/vectors/view/payload-constructed.md#v-pay-213) | Pub MAM of an OT connector with the DeviceClass Oi4.Utility | RES-MAM-014 | V-PAY-028 | DeviceClass `Oi4.Utility` on an `OTConnector` topic (rule R-2) |
| ✅ | [V-PAY-214](../oec/spec/vectors/view/payload-constructed.md#v-pay-214) | Pub Profile with an unknown resource | RES-PROFILE-001 | V-PAY-032 | extra entry `Status` (12th entry) |
| ✅ | [V-PAY-215](../oec/spec/vectors/view/payload-constructed.md#v-pay-215) | Pub Profile with a resource listed twice | RES-PROFILE-001 | V-PAY-032 | `Health` twice |
| ✅ | [V-PAY-216](../oec/spec/vectors/view/payload-constructed.md#v-pay-216) | Pub Profile without Resources | RES-PROFILE-001 | V-PAY-032 | Payload `{}` |
| ✅ | [V-PAY-217](../oec/spec/vectors/view/payload-constructed.md#v-pay-217) | Get Profile that carries a Profile DataSetMessage | RES-PROFILE-003 | V-PAY-030 | Profile DataSetMessage in a `Get` |

### R-3 – Topic vector V-TOP-253 (`draft`)

- **Where:** [`topic-invalid.md`](../oec/spec/vectors/view/topic-invalid.md#v-top-253).

| ✅ | Vector | Title | Violates | Topic |
|---|--------|-------|----------|-------|
| ✅ | [V-TOP-253](../oec/spec/vectors/view/topic-invalid.md#v-top-253) | Reply NewDataSetWriterId with Filter | ACT-020, TOP-013 | `…/Reply/NewDataSetWriterId/<Source>/ReferenceDesignation` – NewDataSetWriterId is answered without a Filter (MG 9.2.1, SF-041 c) |

### R-4 – `reqs` of 8 reviewed guideline vectors extended (back to `draft`)

- **Where:** [`payload-guideline.md`](../oec/spec/vectors/view/payload-guideline.md), source `oec/spec/vectors/payload/guideline.yaml`; commit `25ea9ab`.
- **Change:** topic and payload unchanged. Each vector already satisfies the added REQs; listing them gives the positive coverage (ADR-0019 §8, as in C-034). While `draft`, these vectors do not count, so REQ-MSG-001/014/017/031 fall back temporarily.

| ✅ | Vector | Title | Added to `reqs` |
|---|--------|-------|-----------------|
| ✅ | [V-PAY-001](../oec/spec/vectors/view/payload-guideline.md#v-pay-001) | B1.1.2: Get Health of an explicit asset | REQ-RES-HEALTH-005 |
| ✅ | [V-PAY-003](../oec/spec/vectors/view/payload-guideline.md#v-pay-003) | B1.1.2: Pub Health of an explicit asset, reply to a Get | REQ-RES-HEALTH-001 |
| ✅ | [V-PAY-004](../oec/spec/vectors/view/payload-guideline.md#v-pay-004) | B1.1.2: Pub Health of all assets, self-initialized | REQ-RES-HEALTH-001/002 |
| ✅ | [V-PAY-008](../oec/spec/vectors/view/payload-guideline.md#v-pay-008) | B1.2.1: Call NewDataSetWriterId | REQ-ACT-013 |
| ✅ | [V-PAY-009](../oec/spec/vectors/view/payload-guideline.md#v-pay-009) | B1.2.1: Reply NewDataSetWriterId | REQ-ACT-013 |
| ✅ | [V-PAY-028](../oec/spec/vectors/view/payload-guideline.md#v-pay-028) | B1.1.1: Pub MAM of an explicit asset, reply to a Get | REQ-RES-MAM-006/014 |
| ✅ | [V-PAY-119](../oec/spec/vectors/view/payload-guideline.md#v-pay-119) | MG 8.1.3: Call NewDataSetWriterId with CorrelationData (Figure 7) | REQ-ACT-013 |
| ✅ | [V-PAY-120](../oec/spec/vectors/view/payload-guideline.md#v-pay-120) | MG 8.1.3: Reply NewDataSetWriterId with CorrelationData (Figure 8) | REQ-ACT-013 |

## Last review – C-035 (answered 2026-10-25)

The filled-in sheet is kept in git (`fe28054`); details in the [C-035 log, addendum 1](log/2026-10-24_C-035_metadata.md#addendum-1--po-review-2026-10-25).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| D-1 | ADR-0019 amendment C-035 (`ResourceMetaData`, R-3 name uniqueness, R-6 declared role) | accepted | ADR-0019 status and amendment (`705edd4`) |
| D-2 | Scope change – Metadata entry schemas in C-035 | accepted | ROADMAP P2 unchanged since C-035 || ✅ accepted |
| D-3 | Topic REQs of the roles (REQ-LIFE-002/006/008) | option (a): keep `d7-monitor`/`d7-test` | `life.yaml` header, ADR-0019 amendment point 3 || ✅ accepted |
| R-1 … R-4 | V-PAY-152 … 175, V-PAY-023 … 025 with `role`, REQ-LIFE-003/007/009 and REQ-MSG-038 reclassified | all accepted | `spectool review` (`8f24ecb`), gaps 96 → 77 |
