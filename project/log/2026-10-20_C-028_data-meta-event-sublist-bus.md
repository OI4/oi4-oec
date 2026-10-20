# C-028 – Resources Data, Metadata, Event, SubscriptionList, general Message Bus rules and payload columns of the matrix

- Date: 2026-10-20
- Phase: P1
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Status: in progress
- Guideline baseline: `oi4-oec-documentation` @ `ff1562f`, branch `feature/spec-findings` (GL-ADR-001 … 020, SF-001 … 074 resolved, no open findings, next free id SF-075) – checked at cycle start (`git log`, register header "No open findings", next free SF-075; `spectool lint` ok (303 requirements, 141 informative, 54 payload vectors), `render -check`, `coverage -check`, `topic -check`, `inventory -check -rev ff1562f` ok, 1329 items)

## Intent

PO request (chat, 2026-10-20): execute C-028 as planned in STATUS "Next step":

- Areas `RES-CONFIG` (MG 8.3.3 incl. subsections, 9.1.3), `RES-DATA` (MG 8.3.4, 9.1.4), `RES-META` (MG 8.3.5, 9.1.5), `RES-EVENT` (MG 8.3.6 incl. subsections, 9.1.6), `RES-SUBLIST` (MG 8.3.9, 9.1.9) and `BUS` (MG 9, general rules of the Message Bus communication): requirements as `draft`, every inventory item mapped or listed in `coverage/informative.yaml` with a reason, `lint`/`render`/`coverage`/`topic` green, areas `drafted`. The agent may split the cycle if it gets too large (e.g. RES-CONFIG in a follow-up cycle before the RIG cycle), plan the split in ROADMAP/STATUS and justify it here.
- PO decisions to respect: SF-052 (sender strict REQ-MSG-001, receiver tolerant REQ-MSG-046 SHOULD, validation modes strict/tolerant), SF-048/SF-072/SF-074 (publisher-defined `DataSetClassId` for Data/Metadata, REQ-MSG-010 … 013), GL-ADR-020 (generic `Query`, `DataSetWriterId` 9 – new query parameters for further resources only as an SF proposal), Status events after Set/Del belong to RES-EVENT resp. the resource.
- Reviewed areas (CMP, IL, TOP, MSG, ACT, MQTT, LIFE, RES-MAM, RES-HEALTH, RES-PROFILE, RES-PUBLIST, RES-REFDES, PAG, LOC, QRY) and vectors (203 topic/IL, V-PAY-001 … 054): changes only with a reason in this log.
- **Payload columns of the Method × Resource matrix** (`matrix/method-resource.yaml`; topic columns reviewed since C-022): per combination MessageType, DataSetWriterId and DataSetClassId rule, payload content (empty/DataSets/subset) and answer behavior (Pub as answer, Status event); schema, lint checks and view extended; status `draft`, own review.
- Golden payload vectors (V-PAY-055 ff., `draft`, G-3.9) for B1.1.3 Config, B1.1.4 Data, B1.1.6 Event, B1.1.9 SubscriptionList and the Status-event examples of B1.1.3 … B1.1.10 (incl. B1.1.5, B1.1.8, B1.1.10); DataSet content checked against the resource requirements (checker under `oec/spec/tools` extended, tests green). New defects become SF-075 ff. and vector `edits`; guideline correction only on PO order.
- New ambiguities → SF-075 ff. in `oi4-oec-documentation/findings/SPEC-FINDINGS.md` (Closed table ascending) with a proposed interpretation. Guideline text unchanged, no new GL-ADRs.
- At the end: log, STATUS (ROADMAP/AGENTS/GUARDRAILS if the baseline changes), views, short summary with the points for the PO.

## Split of the cycle (agent decision, PO mandate)

The planned scope is larger than C-027 (244 inventory items, 27 + 5 vectors): 280 open inventory items (RES-CONFIG 144, RES-DATA 30, RES-META 12, RES-EVENT 35, RES-SUBLIST 34, BUS 19 of 25), the new payload columns of all 29 allowed matrix rows, about 42 golden vectors (B1.1.3 alone has 17 examples, its `Set` examples sit in Markdown table cells) and DataSet checks for five resources. **RES-CONFIG is split off into the follow-up cycle C-029** (before the RIG cycle, which becomes C-030):

- RES-CONFIG is self-contained: two field tables of the same object with different obligations (8.3.3.1 `Pub`, 8.3.3.2 `Set`), the object structure `<GroupName>`/`<ConfigName>`/`Validation`/`Context` with name rules, `Set` semantics (mandatory elements, unset by empty string, no `Del`) and 17 B1.1.3 examples incl. two Status events – about half of the work of this cycle on its own.
- Its review is independent of the other areas; the Status-event content (RES-EVENT) it depends on is drafted first, in C-028.
- C-028 therefore delivers RES-DATA, RES-META, RES-EVENT, RES-SUBLIST, BUS, the payload columns of all allowed matrix rows except `Config` (lint requires payload columns only for rows whose resource area is no longer `planned`), and the vectors of B1.1.4, B1.1.6, B1.1.9 plus the Status events of B1.1.4, B1.1.5, B1.1.8, B1.1.9 and B1.1.10. C-029 adds RES-CONFIG, the `Config` payload columns and the B1.1.3 vectors (incl. its two Status events).

## Scope / acceptance criteria (C-028)

- `oec/spec/requirements/res-data.yaml`, `res-meta.yaml`, `res-event.yaml`, `res-sublist.yaml`, `bus.yaml` (`status: draft`); `areas.yaml` – these five areas `drafted`, RES-CONFIG moved to C-029.
- Inventory items RES-DATA 30, RES-META 12, RES-EVENT 35, RES-SUBLIST 34, BUS 25 (6 already mapped by CMP) – 0 open in `coverage/report.md`.
- Matrix payload columns (`payload` object per allowed row, `status: draft`), schema, lint checks (consistency with the topic columns, REQ references, payload vectors), view.
- Reference checks (`tools/internal/payload`) extended by the DataSet content of Data (`Oi4Pv`), Metadata, Event (Status, Syslog, Generic incl. consistency with the topic `Filter`) and SubscriptionList; tests.
- Golden payload vectors V-PAY-055 ff. (`draft`).
- New spec findings registered (register-only commit in the guideline repo, no re-baseline – ADR-0001 amendment C-018).
- `go vet`, `gofmt -l`, `go test ./...`, `spectool lint`, `render -check`, `coverage -check`, `topic -check`, `inventory -check -rev ff1562f` pass.

## Inputs (sources read, with revisions)

- Guideline `ff1562f`: MG 9 index, 9.1 index (incl. "Resources Without Content"), 8.3.3 (index, 8.3.3.1, 8.3.3.2), 8.3.4, 8.3.5, 8.3.6 (index, 8.3.6.1 … 8.3.6.3), 8.3.9, 9.1.3, 9.1.4, 9.1.5, 9.1.6, 9.1.9, 7.7 (Event levels), B1.1.3, B1.1.4, B1.1.6, B1.1.9 and the Status events of B1.1.5, B1.1.8, B1.1.10.
- Project: AGENTS, README, GUARDRAILS, STATUS, ROADMAP, OPEN-QUESTIONS, C-027 log incl. addenda 1 … 9, `oec/spec/README.md`, `requirements/` (CMP, MSG, TOP, RES-PUBLIST), `matrix/method-resource.yaml` (+ schema), `tools/internal/payload`, `tools/internal/vectors`, `vectors/payload/`.
