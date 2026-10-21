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

## Addendum – tooling: spectool review/rebaseline (2026-10-21)

PO request from the model-settings retro (point 3: automate the recurring bookkeeping). Logged here instead of a new cycle because C-029 … C-032 are already planned. No catalog content changed; nothing of C-028 was set to `reviewed`.

- New package `oec/spec/tools/internal/maintain` and two `spectool` commands (`e303ed8`):
  - `review [-to reviewed|draft] [-n] ID...` – sets requirements and vectors `draft` ↔ `reviewed`, areas `drafted` ↔ `reviewed`, matrix payload columns (`Method/Resource` or `matrix`); expands ranges (`REQ-QRY-001..004`) and comma lists. Refuses unknown ids, unexpected states and an area with remaining `draft` requirements (then writes nothing). Line-based edits keep comments. Then `render`, `coverage`, `lint`.
  - `rebaseline -rev <commit> [-n] [-force]` – requires the commit to be checked out in the guideline repository and `docs/`/`rfcs_adrs/` to be clean; stops if only the register changed since the baseline (ADR-0001 amendment C-018). Replaces `baseline` in all catalog files and the `topic.abnf` header, regenerates inventory, topic patterns, views and coverage, runs `lint`, and lists the lines of AGENTS/STATUS/GUARDRAILS/ROADMAP/`oec/spec/README.md` that still name the old baseline (prose stays manual).
- Verification: unit tests (`go test ./...` green, `vet`/`gofmt` clean). Dry runs on the workspace: `rebaseline -n -rev 594aaa9` → no re-baseline needed (register-only); `review -n matrix` → 26 payload columns; `review -n BUS` refused (draft requirements). In a temporary copy with a test guideline commit: full `rebaseline` (only baseline lines and generated files changed; lint reported V-PAY-021 as no longer verbatim, as expected) and a full `review` of all C-028 drafts (88 entries, 5 areas, 26 matrix rows; lint green; only status lines changed). Copy removed.
- Docs: `oec/spec/README.md` (`68dfbbf`), STATUS next step 3 (`737c320`).

## Addendum 1 – PO review (2026-10-21)

PO feedback: views `res-event.md`, `res-data.md`, `res-meta.md`, `res-sublist.md`, `bus.md` reviewed and accepted; matrix payload columns (`matrix/view/method-resource.md`) reviewed and accepted; V-PAY-020 … 022 and V-PAY-055 … 087 reviewed; REQ-PAG-005 reviewed; the MQTT note and the cycle pointers in `informative.yaml` are fine. SF-075 … SF-081 accepted as proposed, to be implemented.

- `spectool review` set the 55 requirements, the five areas (RES-EVENT, RES-DATA, RES-META, RES-SUBLIST, BUS → `reviewed`), 26 matrix payload columns and the 33 vectors V-PAY-055 … 087 to `reviewed`; V-PAY-020 … 022 and REQ-PAG-005 were already `reviewed` (the PO confirmed the C-028 changes). Views rendered (`87506ac`).

## Addendum 2 – SF-075 … SF-081 in the guideline, re-baseline on `b2f0fe7` (2026-10-21)

Guideline repository `oi4-oec-documentation`, branch `feature/spec-findings`, local commits (the PO ordered the guideline correction; no new GL-ADR – none was ordered):

- `dd064fe` SF-075 – `SymbolicId` (8.3.6.1) and `MSG`/`HEADER` (8.3.6.2) are keys under `Details`; a `Syslog` event must not contain `Description`; B1.1.6 note says `<EventLevel>`.
- `60d20f8` SF-076 – 8.3.6: `<EventCategory>` ↔ `Category`; 8.3.6.1: `<EventLevel>` = severity class of the StatusCode (bits 30–31; the reserved class 3 counts as `Bad`); 8.3.6.2: PRI 0 … 191, `<EventLevel>` = RFC 5424 severity name of PRI modulo 8; 9.1.6 aligned.
- `491a028` SF-077 – Part 14-7.2.5.4 instead of 7.2.3/Table 152; `Sv<n>` only if the asset provides it, no other keys than `Pv` and `Sv<n>`; description = `Description` of the field in the metadata; `Oi4Pv` also in the aggregated publications.
- `135ce3a` SF-078 – 9.1.4 `Pub` templates carry `DataSetClassId`; `Set` templates of 9.1.4/9.1.5 use the requester as `PublisherId`; `Oi4Pv` `Set` template without `correlationId`, `ReplyTo` like the other templates.
- `81d67d7` SF-079 – 8.2.3: a `DataSetMessage` in a `Set`/`Del` request carries the `DataSetWriterId` of the addressed `DataSet` as published by the addressed application (for `PublicationList`/`SubscriptionList` the common one), the `Source` as `DataSetWriterName`, the `Filter` as `WriterGroupName` (empty string for resources without `Filter`) and no `SequenceNumber`; B1.1.4 "Set explicit Data tag" without `SequenceNumber`.
- `2656353` SF-080 – 8.3.9/9.1.9/8.2.3/B1.1.9: `TopicPath` is the subscribed topic filter (wildcards allowed, concrete resource level), entry identity `Source` + `TopicPath`, `WriterGroupName` derived from the `TopicPath`, `Interval` is a processing interval (default 0, on arrival), `Config` read only (`NONE_0` neither changed nor deleted, `CONF_1` both), `Set`/`Del` rules and the republication after `Del`; B1.1.9 entries subscribe to `…/Pub/Data/<FDS>/oee`.
- `21c563a` SF-081 – 9.1: `Source` and – for resources with a `Filter` – the `Filter` are required in the `Set`/`Del` topic.
- `b2f0fe7` – register: SF-075 … 081 resolved (Closed table ascending), no open findings, next free id SF-082.
- Checks: all 91 JSON blocks of B1 parse; `yarn build` ok (`.yarn/install-state.gz` is a local install artifact, not committed).

Agent decisions while wording the corrections (G-4.4, for PO confirmation):

- SF-076: the mapping rules are placed in 8.3.6/8.3.6.1/8.3.6.2 (where `Number`/`Category` are defined) instead of the topic chapter 7.7.
- SF-077: added "Secondary values are only present if the asset provides them" next to the relaxed `Sv<n>` requirement.
- SF-078: placeholder `<(meta)data specific GUID>` reused from the `Set` templates; requester `PublisherId` written as `<IdentificationLink>`; `ReplyTo` of the `Oi4Pv` `Set` template aligned with the other `Set` templates.
- SF-079: the `Filter`/`WriterGroupName` sentence also covers resources without `Filter` (empty string).
- SF-080: the 9.1.9 `Pub` template keeps `<IdentificationLink>` as `DataSetWriterName` placeholder (consistent with the other templates) plus the explicit note "The `DataSetWriterName` of each entry is its `Source`"; added "The `Config` of an entry is set by the application; an entry created via `Set` is freely configurable (`CONF_1`)" (otherwise `Set` could create an entry nobody may delete); the `WriterGroupName` rule is repeated in 8.2.3 next to the other `WriterGroupName` rules; (d) needed no change of the 9.1 index – 9.1 and 9.1.9 are consistent after the 9.1.9 rewrite.

Workspace:

- `9ba9084` re-baseline: `spectool rebaseline -rev b2f0fe7` – baseline in all catalog files, inventory (1344 items) and coverage report regenerated; areas outside C-028 unchanged in content.
- `645679f` requirements: lint reported 33 stale or missing quotes/keys, all in the C-028 areas; remapped and requoted. Statements with new normative content went back to `draft` for PO review:
  - REQ-RES-EVENT-004 – StatusCode class 3 is treated as `Bad` (SF-076).
  - REQ-BUS-010 – empty `WriterGroupName` for resources without `Filter`, no `SequenceNumber` in `Set`/`Del` (SF-079); verification adds `schema`, `d7-validate`.
  - REQ-RES-SUBLIST-002 – the resource level of the `TopicPath` is concrete (SF-080).
  - REQ-RES-SUBLIST-004 – `Config` read only and set by the application, entries created via `Set` are `CONF_1`, a `Config` change is rejected (SF-080).
  All other remapped requirements only got new quotes/inventory keys/notes (wording identical in meaning) and stay `reviewed`. Areas stay `reviewed` (the review command refuses only on the way to `reviewed`; the four drafts are listed for the PO).
- `f23bfc5` vectors: V-PAY-061 without `SequenceNumber` (B1.1.4 "Set explicit Data tag" after SF-079); V-PAY-069 … 072, 074 without `original`/`edits` (B1.1.9 now carries the concrete `TopicPath`s, SF-080 (b)) – verbatim again, payload content identical to the reviewed edited form, so they stay `reviewed`; V-PAY-086 title/notes point to the former B1.1.9 `TopicPath` (guideline `ff1562f`).
- `b2bc1fb` checker: StatusCode class 3 on `Status/Bad` is valid (was an error); a `SequenceNumber` in a `Set`/`Del` `DataSetMessage` is reported as REQ-BUS-010; a SubscriptionList `TopicPath` in the OEC namespace (`Oi4v2`, `+`, `#` as first level) with a non-concrete resource level is reported as REQ-RES-SUBLIST-002 (topic filters outside the OEC namespace are not checked). Tests added.
- `3a3a85b` views.
- Checks: `go vet ./...`, `gofmt -l .` (empty), `go test ./...` ok; `spectool lint` ok (20 area files, 358 requirements, 171 informative items; 66 combinations; vectors topic 146, IL 29, dnp 15, oi4identifier 13, payload 87); `render -check`, `coverage -check` (C-028 areas 0 open), `topic -check`, `inventory -check -rev b2f0fe7` (1344 items) ok.

For the PO: review REQ-RES-EVENT-004, REQ-BUS-010, REQ-RES-SUBLIST-002, REQ-RES-SUBLIST-004 (views `res-event.md`, `bus.md`, `res-sublist.md`) and confirm the agent decisions above.

## Addendum 3 – PO acceptance, SF-082 lists of figures/tables, re-baseline on `c2be26b` (2026-10-21)

PO (2026-10-21): REQ-RES-EVENT-004, REQ-BUS-010, REQ-RES-SUBLIST-002 and REQ-RES-SUBLIST-004 reviewed and accepted – set to `reviewed` with `spectool review` (`1e32276`). The guideline wording of SF-076 … 080 including the agent decisions of addendum 2 is accepted. Nothing in the catalog is `draft` any more.

New PO order (no ADR, a spec-finding entry is enough, implement without PO review): the List of Figures and List of Tables of MG and RIG show chapter names instead of the captions, and not every entry is linked. Registered and resolved as **SF-082**.

Guideline (`oi4-oec-documentation`, branch `feature/spec-findings`):

- `2d9cfb5` – the four lists (MG/RIG figures/tables) now list every caption ("Figure n: …", "Table n: …") with a link to its page (MG: 21 figures, 11 tables; RIG: 2 figures, 10 tables). Found and fixed on the way:
  - MG Figure 1 (Reference Architecture) had a caption but no image since the Confluence export; the unreferenced source `static/img/guideline/3eb410fc-….drawio` (page "Official OI4 Reference Architecture") was exported with the draw.io CLI to `…drawio.png` and embedded (link to the `.drawio` like the other figures).
  - MG A2 lost its caption (originally "Table 14: Predefined DataSetClassIds …") in a format conversion; since SF-017/SF-053 A2 has three tables, now captioned Tables 4 (resources), 5 (Actions), 6 (reserved ids).
  - MG A5 lost its caption (originally "Table 18: Authors (in alphabetical order)") inside a broken 40-column conversion table; rebuilt as Author/Company table with caption Table 10 (same 17 authors).
  - Renumbering: MG A3 Tables 4/5 → 7/8 (incl. the A3 text reference and the MG 4.3.1 note), A4 Table 6 → 9, A6 Table 8 → 11. Figure 2 caption merged into one italic span; RIG Tables 7–9 captions got the colon.
  - `yarn build` without warnings.
- `c2be26b` – register: SF-082 closed/resolved, no open findings, next free id SF-083.

Agent decisions (G-4.4, editorial, no PO review requested by the PO):

- Links point to the page, not to the caption: in-page anchors (`<a id>` in MDX) are not registered by the Docusaurus 3 broken-anchor check and produced false warnings, so they were dropped.
- Not captioned: the unnumbered notation example in MG 1.2 and about 20 tables that never had a caption (definition/field tables in MG 1.2, 1.3, 4.1, 4.2, 7.1, 8.1.4, 8.2.20, 8.4, B1.1.3, B1.1.8 and RIG 1.2). Numbering them would shift Table 3 (NE 107), which requirements quote. Offered to the PO as a follow-up.
- The List of Figures of the OEC Process Description has the same defect; it was not part of the order (informative document) and is left unchanged.
- Historical records (GL-ADRs, ADR-0001 amendments, older logs) keep the old table numbers.

Workspace:

- `a464725` re-baseline: `spectool rebaseline -rev c2be26b` – inventory unchanged (1344 items, same keys; only line numbers in MG 2 shift because of the Figure 1 image); baseline in all catalog files.
- `2775093` references to MG A3 Table 4/5 updated to Table 7/8 – notes of two IL requirements, titles/description of the dnp-encoding and oi4identifier vectors and their views. Wording only; status stays `reviewed` (reason: SF-082 renumbering, content unchanged).
- `82471f3` the same in comments of `oec/spec/tools/internal/il` (no code change).
- Checks: `spectool lint` ok (20 area files, 358 requirements, 171 informative items; 66 combinations; vectors topic 146, IL 29, dnp 15, oi4identifier 13, payload 87); `render -check`, `coverage -check`, `topic -check`, `inventory -check -rev c2be26b` ok; `go test ./...` ok.

Next: C-029 (RES-CONFIG).

## Addendum 4 – SF-083 all figures/tables captioned and renumbered, re-baseline on `5bc2e85` (2026-10-21)

PO order (2026-10-21, follow-up of SF-082, implement without review): fix all defects, add all missing captions, check the numbering. Registered and resolved as **SF-083** (editorial, no ADR).

Guideline (`oi4-oec-documentation`, branch `feature/spec-findings`):

- `b81d98e` – every figure and table of MG, RIG and the Process Description (PD) has a caption directly below it, numbered in reading order (Docusaurus sidebar order). Result: MG 22 figures / 23 tables, RIG 2 figures / 11 tables, PD 8 figures. A scan (caption follows every image/table outside code blocks, numbers 1..n per document) and `yarn build` (no warnings) were run.
  - New captions: MG Figure 1 (1.2 UML sequence diagram example); MG Tables 1 (1.2 keywords), 2–3 (1.3 compliance functions, level examples), 6–7 (4.1 Identification Link components, examples), 8 (4.2 encoded examples), 9 (7.1 Namespace versions), 10–11 (8.1.4 content masks), 12 (8.2.20 ActionState usage), 14 (8.4 Actions), 23 (B1.1.8 precisions); RIG Table 1 (1.2 compliance functions).
  - Renumbering: MG Figures 1–21 → 2–22; MG Tables 1→4, 2→5, 3→13, 4→15, 5→16, 6→17, 7→18, 8→19, 9→20, 10→21, 11→22; RIG Tables 1–10 → 2–11. Text references updated (MG 4.3.1, 8.1.1, 8.1.2, 8.3.2, A3). The MG 9.1 index called the overview diagram "figure 16" (already wrong before) – now "Figure 9".
  - Conversion defects: literal `&lt;br/&gt;` (shown as text) → line break in MG 3.2, 8.3.2, A3, A6, B1.1.8 and RIG 4.1.1, 4.1.2; B1.1.8 precision table cells broken into lines; empty header rows replaced (MG A4 and RIG A1 glossaries `Term | Definition`; RIG 4.1.1–4.1.4 bold first row promoted to header); MG A6: 270 dead Confluence links and 4 links to a former Confluence (one broken) unlinked; MG B1.1.3: three JSON payloads the export had flattened into table cells are `json` code blocks again (content verbatim, `json.loads` valid); PD 2.2: the missing image exported from its drawio source (`d274072c-….drawio.png`) and embedded.
  - Lists of figures/tables of MG, RIG and PD regenerated from the captions (page links).
- `5bc2e85` – register: SF-083 closed/resolved, no open findings, next free id SF-084.

Agent decisions (G-4.4, editorial):

- Title pages (`index.md` logo and editor table of MG, RIG, PD) are front matter – no caption.
- The NE 107 icons inside the MG 8.3.2 table are table content, not figures; JSON listings other than MG Figures 5–8 stay uncaptioned (examples in text, not figures).
- B1.1.3 JSON blocks: only the format was repaired; whether the payloads are correct is checked in C-029 (RES-CONFIG golden vectors).
- External references to OPC UA tables ("Part 6 Table 1") and historical records (GL-ADRs, ADR-0001 amendments, older logs) are unchanged.

Workspace:

- `69e546b` re-baseline: `spectool rebaseline -rev 5bc2e85` – 1341 items (−3: RIG 4.1.2/4.1.4 empty header rows gone); changed keys in MG 8.3.2 (2), MG A4 (39), RIG A1 (16), RIG 4.1.x – only the two MG 8.3.2 keys were mapped: REQ-RES-HEALTH-002/003 remapped to `MG-8.3.2-d6f8e58c`/`-b75c4616`, quotes, statement and notes "Table 3" → "Table 13"; `coverage/informative.yaml` MG 8.1.3 reasons "Figure 6/7" → "Figure 7/8". Renumbering only, status stays `reviewed`.
- `dacd109` IL notes, dnp-encoding/oi4identifier vector titles and views: MG A3 Table 7/8 → 18/19 (status stays `reviewed`, same reason).
- `612cbdb` the same in comments of `oec/spec/tools/internal/il`.
- Checks: `spectool lint` ok (20 area files, 358 requirements, 171 informative items; 66 combinations; vectors topic 146, IL 29, dnp 15, oi4identifier 13, payload 87); `render -check`, `coverage -check`, `topic -check`, `inventory -check -rev 5bc2e85` ok; `go test ./...` ok.

Next: C-029 (RES-CONFIG).
