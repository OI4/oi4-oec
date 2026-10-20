# C-028 – Resources Data, Metadata, Event, SubscriptionList, general Message Bus rules and payload columns of the matrix

- Date: 2026-10-20
- Phase: P1
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Status: done – awaiting PO review (all new artifacts `draft`)
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

## Results

### Requirements (all `draft`, areas `drafted`)

| Area | File | REQs | Inventory items | covered | informative | open |
|------|------|-----:|---:|---:|---:|---:|
| RES-EVENT | `requirements/res-event.yaml` | REQ-RES-EVENT-001 … 011 | 35 | 29 | 6 | 0 |
| RES-DATA | `requirements/res-data.yaml` | REQ-RES-DATA-001 … 012 | 30 | 23 | 7 | 0 |
| RES-META | `requirements/res-meta.yaml` | REQ-RES-META-001 … 007 | 12 | 8 | 4 | 0 |
| RES-SUBLIST | `requirements/res-sublist.yaml` | REQ-RES-SUBLIST-001 … 015 | 34 | 26 | 8 | 0 |
| BUS | `requirements/bus.yaml` | REQ-BUS-001 … 010 | 25 | 20 | 5 | 0 |

55 new REQs (catalog 303 → 358), 30 new informative items (141 → 171). MG 8.3.5 has no inventory item (no keyword, no field table); RES-META is derived from 9.1.5 and the B1.1.5 notes. Highlights:

- **RES-EVENT**: fields of the Event payload with `SymbolicId` (Status) and `MSG`/`HEADER` (Syslog) as keys of `Details` (SF-075 (a)); `Category` ↔ `<EventCategory>` (002, SF-076 (c)); Status `EventLevel` = severity class of the StatusCode (004, SF-076 (b)); Syslog PRI 0 … 191, no `Description`, `EventLevel` = RFC 5424 name of PRI % 8 (007/008, SF-076 (a)); Status event after every `Set`/`Del` (005, L1 SHOULD / L2+ MUST, C-016); Events only via `Pub` (011).
- **RES-DATA**: `Oi4Pv` with `Pv` mandatory, `Sv1` … `Sv255` optional, no other key (004, SF-077 (b)); `Get` without Data DSM (006), `Set` with exactly one (009); Status event and republication after `Set` (011/012). The publisher-defined `DataSetClassId` stays with REQ-MSG-011 (SF-048/072/074).
- **RES-META**: `Get` = `ua-metadata` with `MetaData: {}` (001); answer, `Pub`, `Set`, ACL, Status event, republication (002 … 007).
- **RES-SUBLIST**: entry fields (001), `WriterGroupName` derived from `TopicPath` (002, SF-080 (a)), `Interval` as processing interval of the subscriber (003, SF-080 (c)), `Config` `NONE_0`/`CONF_1` (004, SF-080 (e)), `Set` creates/reconfigures, `Del` carries exactly one entry (010/011), after `Del` the remaining entries or nothing (015, SF-080 (d)).
- **BUS**: base interactions of MG 9.1 (requester topic, `Get` → `Pub` with `ReplyTo`, `Set`/`Del` with `Source`/`Filter` (SF-081), publication after `Set` and partial `Del`, nothing after a whole `Del`) and the `DataSetMessage` header of `Set`/`Del` requests (010, SF-079).

### Matrix payload columns (`draft`)

`matrix/method-resource.yaml` + schema: new object `payload` on the allowed rows – `status`, `message_type`, `class_id` (`predefined`/`publisher-defined`/`if-known`/`in-metadata`/`none`), `writer_id` (`publisher`/`list`/`addressed`/`action`/`none`), `content` (`none`/`datasets`/`single`/`metadata-empty`/`metadata-filled`/`action-request`/`action-response`), `helpers`, `answer` (`none`/`pub`/`status-and-pub`/`status`/`status-and-rest`/`reply`), `reqs`, `notes`. 26 of 29 allowed rows have them; the three `Config` rows follow in C-029 (lint accepts missing columns only while the resource's area is `planned`). `spectool lint` checks presence, known REQs and consistency with method/resource (predefined class ids of MG A2, Get carries no resource DataSet, Set exactly one, Del answer, helper placement of PAG/LOC/QRY, list writer ids only for the two lists, action rows); valid payload vectors are cross-checked against their row (`MessageType`, number of resource DataSetMessages, helper DataSets, empty `MetaData`). The view `matrix/view/method-resource.md` shows a payload table per resource with REQ and vector links.

Design choices for the review: `Get` rows have `writer_id: none` (no resource DSM), `Get Metadata` uses `addressed` (the `ua-metadata` header carries the id of the addressed DataSet) and `class_id: none` (V-PAY-020); `Data` `Get`/`Set` use `if-known` (REQ-MSG-011), `Pub Data` `publisher-defined`; `Set`/`Del` of the two lists use `list` (REQ-BUS-010: common id of the list); `Pub Event` has no helpers (no paginated Event answers).

### Reference checks (`tools/internal/payload`)

- Data: `Get` without Data DSM (REQ-RES-DATA-006), `Set` exactly one (009), `Oi4Pv` fields (004; recognised by the `WriterGroupName` `Oi4Pv`). Application-defined Data DataSets are not checked.
- Metadata: `Get` requires `MetaData: {}` (REQ-RES-META-001; before only "JSON object", REQ-MSG-032).
- Event: fields/types (001), `Category` vs. `WriterGroupName` (002), Status `Details` keys and reserved severity bits (003), Status `EventLevel`/`SymbolicId` vs. StatusCode class (004), Syslog `Details` keys, PRI ≤ 191, no `Description` (007), `EventLevel` vs. PRI % 8 (008); Generic: `Details` any object.
- SubscriptionList: entry fields (001), `WriterGroupName` from `TopicPath` for OEC topic filters with concrete resource level (002), `Get` (007), `Set` exactly one (010), `Del` exactly one (011, new `delOne` rule – ReferenceDesignation `Del` stays "no DSM").
- Tests: `TestContentC028` (39 cases), `TestGetMetadata` extended, `TestCheckPayload`/`TestCheckPayloadDetectsDrift` for the matrix columns.

### Vectors (`draft`)

- Golden (`vectors/payload/guideline.yaml`), all valid: **V-PAY-055 … 062** B1.1.4 (Get ×3, Pub ×3, Set, Status), **V-PAY-063 … 065** B1.1.6 (Status, Syslog, Generic), **V-PAY-066 … 075** B1.1.9 (Get ×3, Pub ×3, Set, Status, Del, Status), Status events **V-PAY-076** (B1.1.3), **077** (B1.1.5), **078** (B1.1.8), **079/080** (B1.1.10 Set/Del). Verbatim except the B1.1.9 entries V-PAY-069 … 072 and 074: `original` + edit `corrected` at `/Messages/0/Payload/TopicPath` (SF-080 (b)). V-PAY-061 (Set Data) keeps `SequenceNumber` 3985 without edit (optional field; SF-079 only proposes its removal).
- Constructed invalid (`vectors/payload/constructed.yaml`): **V-PAY-081** Status level vs. StatusCode (REQ-RES-EVENT-004), **082** Syslog level vs. PRI (008), **083** Category vs. EventCategory (002), **084** Oi4Pv without Pv (REQ-RES-DATA-004), **085** Del SubscriptionList without entry (REQ-RES-SUBLIST-011), **086** unedited B1.1.9 `TopicPath` (REQ-RES-SUBLIST-002 – shows why the edit is needed), **087** Get Metadata with filled `MetaData` (REQ-RES-META-001).
- Payload vectors 54 → 87.

### Spec findings (register `594aaa9`, guideline text unchanged, no re-baseline)

SF-075 Event payload structure · SF-076 `Number`/`Category` of an Event vs. topic · SF-077 Data/`Oi4Pv` · SF-078 payload templates of 9.1.4/9.1.5 · SF-079 `DataSetMessage` header of `Set`/`Del` · SF-080 (a) … (f) SubscriptionList · SF-081 `Filter` in `Set`/`Del` of MG 9.1. All open with a proposed interpretation the drafts follow; none blocking. Next free id SF-082.

## Deviations from the plan

- **B1.1.3 Status event in C-028**: the PO request lists the Status events of B1.1.3 … B1.1.10; the plan above had moved the two B1.1.3 Status events to C-029. They are pure RES-EVENT content, so V-PAY-076 covers them now (both examples have identical text – one vector). C-029 adds only the Config examples.
- **SF-079 without vector edit**: the `SequenceNumber` in "Set Data" is optional and valid; removing it is only a proposal, so V-PAY-061 stays verbatim (no `original`/`edits`).
- **No SF for repeated `MessageId`s**: B1.1.9 `Del` and its Status event reuse the `MessageId`/`SequenceNumber` of the `Set` example (B1.1.10 uses new ones). All B1 examples are standalone illustrations and share `MessageId`s (e.g. all `Get`s), so this is not recorded as a defect.

## Changes to reviewed items (with reason)

- `requirements/pag.yaml`, REQ-PAG-005 (reviewed): inventory + MG-9.1.4-82b9abfd/4fa8ccde and MG-9.1.9-d64e37b0/d6d5f22d (the pagination sentences of 9.1.4/9.1.9 are the same rule as in 9.1.1/9.1.2/9.1.8), note updated; statement unchanged, status kept `reviewed` (`0a218d4`).
- `requirements/mqtt.yaml` note and `coverage/informative.yaml`: cycle pointers C-029 → C-030 for the RIG areas (`1174899`, cycle cut).
- Vectors V-PAY-020/021/022 (reviewed): `reqs` + REQ-RES-META-001 / 002, 003 / 004, REQ-BUS-010; payload, topic, status unchanged (`e5004cc`).
- Matrix: payload columns added to rows whose topic columns are reviewed; the topic columns are unchanged, the new columns are `draft`.
- Checker: `Get Metadata` with non-empty or non-object `MetaData` now reports REQ-RES-META-001 instead of REQ-MSG-032 (no reviewed vector affected).

## Checks

`go vet ./...`, `gofmt -l .` (empty), `go test ./...` ok; `spectool lint` ok (20 area files, 358 requirements, 171 informative items; 66 combinations; vectors topic 146, IL 29, dnp 15, oi4identifier 13, payload 87); `render` written; `coverage -check`, `topic -check`, `inventory -check -rev ff1562f` (1329 items) ok.

## Commits

Workspace: `5258438` plan · `3770a7c` RES-EVENT · `0a218d4` REQ-PAG-005 inventory · `a30e084` RES-DATA · `dba7cc6` RES-META · `d63f6a8` RES-SUBLIST · `5050a3b` BUS · `1174899` cycle cut (ROADMAP, areas) · `9d7f730` matrix payload columns · `2838e90` checker · `0df5cb2` vectors V-PAY-055 … 087 · `e5004cc` V-PAY-020 … 022 reqs · this log/STATUS. Guideline repo: `594aaa9` (register SF-075 … 081).

## For the PO (review / decisions)

1. Review the views `requirements/view/res-event.md`, `res-data.md`, `res-meta.md`, `res-sublist.md`, `bus.md` (55 REQs, 30 informative items).
2. Review the matrix payload columns (`matrix/view/method-resource.md`, payload tables) incl. the design choices above.
3. Review vectors V-PAY-055 … 087 (`vectors/view/payload-guideline.md`, `payload-constructed.md`).
4. Confirm the reviewed-item changes (REQ-PAG-005 inventory, V-PAY-020 … 022 reqs).
5. Decide SF-075 … SF-081 – especially SF-076 (level mappings Syslog/Status), SF-079 (`SequenceNumber` in `Set`), SF-080 (a) identification of an entry / `WriterGroupName` rule, (b) which `TopicPath` the B1.1.9 example should use, (d) republication after `Del`, (e) `NONE_0` vs. `Set`.

## Next

C-029 `RES-CONFIG` (MG 8.3.3 incl. 8.3.3.1/8.3.3.2, 9.1.3; 144 items), payload columns of the three `Config` rows, golden vectors of the B1.1.3 Config examples, Config content checks – after the PO review of C-028 or in parallel to it.
