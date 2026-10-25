# Review and decisions – open items for the PO

_Last updated: 2026-10-25, PO answers for C-036 addendum 2 recorded (R-1 … R-3 accepted); guideline `a42a132`._

This is the single list of everything that waits for the PO: items to **review** (drafts and changes to reviewed items) and items to **decide** (spec findings, open questions, cycle plan). Each cycle updates it; items leave the list once the PO has answered and `spectool review` (or the decision record) has been applied. Details are in the cycle log; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md) (same numbering).

**How to answer:** tick the box or write the decision into the "PO" field (or answer in the chat, e.g. "D-1: wie vorgeschlagen, R-1 … R-4 akzeptiert"). Anything not accepted gets a note; the agent reworks it in the next cycle.

Current state: **3 review items** from implementing SF-099 (one draft REQ, one changed reviewed REQ, the guideline wording); no open spec finding (next SF-100); no open question for P2 (Q-011 parked). Coverage: 48 `schema`/`d7-validate` gaps (unchanged – both REQs are `d7-test`). Details: [C-036 log, addendum 2](log/2026-10-25_C-036_actions-mam-profile.md#addendum-2--sf-099-in-the-guideline-re-baseline-on-a42a132-2026-10-25).

## Overview

| # | Item | Kind | Source | Recommendation | PO |
|---|------|------|--------|----------------|----|
| R-1 | New REQ-CMP-047 – `Get` for an unknown asset: no `Pub`, should `Event` Status Bad | review | C-036 add. 2 | accept | ✅ accepted |
| R-2 | REQ-CMP-041 (reviewed) – statement extended by the `Filter` case | review | C-036 add. 2 | accept | ✅ accepted |
| R-3 | Guideline wording of SF-099 (`a42a132`: MG 9.1, 9.1.x `Pub` bullets, 8.3.6.1) | review | C-036 add. 2 | accept | ✅ accepted |

## Reviews

### R-1 – REQ-CMP-047 (new, `draft`)

- **Where:** [`requirements/cmp.yaml`](../oec/spec/requirements/cmp.yaml) / [view](../oec/spec/requirements/view/cmp.md).
- **Statement:** "An entity does not answer a Get whose Source addresses an asset that the entity neither is nor represents on the related Pub topic. It should answer such a Get with an Event of the category Status and the level Bad (BadNotFound, MG 8.3.6.1)." Keyword SHOULD at all levels (the PO said "should"; the Set/Del rule REQ-CMP-043 is MUST from Level 2), `d7-test`, inventory MG-9.1-144b2f9a.
- **PO:** X accepted ☐ change: … (2026-10-25)

### R-2 – REQ-CMP-041 statement extended (reviewed item)

- **Change:** appended "The same applies to a Get whose Filter matches no content of a Source the entity provides; a Get for a Source the entity does not provide is REQ-CMP-047." Quote and inventory key follow the guideline; refs +SF-099. Reason: the PO confirmed this reading on 2026-10-25. The item stays `reviewed` (SF-097/SF-098 precedent); please object if not.
- **PO:** X accepted ☐ change: … (2026-10-25)

### R-3 – Guideline wording (`a42a132`)

- MG 9.1 "Resources Without Content", first bullet + "This also applies to a `Get` whose `Filter` matches no content of a `Source` the entity provides."; new second bullet: "A `Get` whose `Source` addresses an asset that the entity neither is nor represents is not answered on the related `Pub` topic. The entity should answer such a `Get` with an `Event` of the category `Status` and the level `Bad` (`BadNotFound`, see 8.3.6.1 Status)."
- MG 9.1.1/9.1.2/9.1.3/9.1.4/9.1.7/9.1.8/9.1.9/9.1.10, `Pub` bullet: "… `DataSetMessages` of type X – none in the answer to a `Get` without content (Resources Without Content) – and optional `Pagination` …".
- MG 8.3.6.1: Status events also report "a `Get` that is not answered – a `Get` of `Metadata` without `Data` or a `Get` for an unknown asset".
- "Neither is nor represents" uses the terms of MG 1.3 (directly connected / represented entities). No GL-ADR (GL-ADR-017 only moved the section). The 8 `Pub`-content REQs and REQ-RES-EVENT-005 only got new quotes/keys (mechanical, statements unchanged).
- **PO:** X accepted ☐ change: … (2026-10-25)


## Last review – C-036 (answered 2026-10-25)

The filled-in sheet is kept in git (`a9aae9c`); details in the [C-036 log, addendum 1](log/2026-10-25_C-036_actions-mam-profile.md#addendum-1--po-review-2026-10-25).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| D-1 | SF-099 – empty `Pub` as the answer to a `Get` without content | decided differently: no content → empty list; unknown asset → no `Pub`, should `Event` Status Bad | register `5491830` (`decided`); on PO order in the guideline `a42a132` (addendum 2, review items R-1 … R-3 above); schemas unchanged |
| D-2 | ADR-0019 amendment C-036 | accepted | ADR-0019 status, amendment heading and point 1 (`be648c9`) |
| D-3 | Cycle split – PublicationList/ReferenceDesignation → C-037, exit C-041 | accepted | ROADMAP P2 unchanged since C-036 |
| D-4 | REQ-RES-MAM-006 heuristic | accepted | oracle unchanged (`298e709`) |
| R-1 … R-4 | V-PAY-176 … 217, V-TOP-253, `reqs` of 8 guideline vectors | all accepted | `spectool review` (`879e5e9`), gaps 81 → 48 |

**Follow-up:** the PO confirmed on 2026-10-25 that a `Filter` on a known asset that matches nothing gets the empty list, and ordered SF-099 into the guideline (addendum 2).

## Earlier review – C-035 (answered 2026-10-25)


The filled-in sheet is kept in git (`fe28054`); details in the [C-035 log, addendum 1](log/2026-10-24_C-035_metadata.md#addendum-1--po-review-2026-10-25).

| # | Item | PO answer | Applied |
|---|------|-----------|---------|
| D-1 | ADR-0019 amendment C-035 (`ResourceMetaData`, R-3 name uniqueness, R-6 declared role) | accepted | ADR-0019 status and amendment (`705edd4`) |
| D-2 | Scope change – Metadata entry schemas in C-035 | accepted | ROADMAP P2 unchanged since C-035 |
| D-3 | Topic REQs of the roles (REQ-LIFE-002/006/008) | option (a): keep `d7-monitor`/`d7-test` | `life.yaml` header, ADR-0019 amendment point 3 |
| R-1 … R-4 | V-PAY-152 … 175, V-PAY-023 … 025 with `role`, REQ-LIFE-003/007/009 and REQ-MSG-038 reclassified | all accepted | `spectool review` (`8f24ecb`), gaps 96 → 77 |
