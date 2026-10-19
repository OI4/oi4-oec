# C-027 – Core resources (MAM, Health, Profile, PublicationList, ReferenceDesignation), Pagination and Locale

- Date: 2026-10-19
- Phase: P1
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Status: done (waiting for PO review)
- Guideline baseline: `oi4-oec-documentation` @ `c6453c7`, branch `feature/spec-findings` (GL-ADR-001 … 018, SF-001 … 055 and SF-057 … 061 resolved, SF-056 open (B1 examples, C-027/C-028, not blocking), next free id SF-062) – checked at cycle start (`git log`, register header, `spectool inventory -check -rev c6453c7` ok, 1279 items; `lint`, `render -check`, `coverage -check`, `topic -check` ok)

## Intent

PO request (chat, 2026-10-19, hand-over prompt `prompt.md`): execute C-027 as planned in STATUS "Next step":

- Areas `RES-MAM` (MG 5, 8.3.1, 9.1.1), `RES-HEALTH` (MG 8.3.2, 9.1.2), `RES-PROFILE` (MG 8.3.7, 9.1.7), `RES-PUBLIST` (MG 8.3.8, 9.1.8), `RES-REFDES` (MG 8.3.10, 9.1.10), `PAG` (MG 8.3.11) and `LOC` (MG 8.3.12): requirements as `draft`, every inventory item of these sections mapped or listed in `coverage/informative.yaml` with a reason, `lint`/`render`/`coverage`/`topic` green, areas `drafted`.
- Respect the PO decisions SF-035 … SF-061 and GL-ADR-018, in particular SF-052: sender strict (REQ-MSG-001), receiver tolerant (REQ-MSG-046, SHOULD), validation modes strict/tolerant (`payload.Mode`, vector field `mode`). Areas CMP, IL, TOP, MSG, ACT, MQTT, LIFE and all existing vectors (203 topic/IL, V-PAY-001 … 025) are reviewed – changes only with a reason in this log.
- Golden payload vectors (V-PAY-026 ff., `status: draft`, `edits` per ADR-0018 amendments C-013/C-023, G-3.9) for the B1 examples of the resources of this cycle (MG B1.1.1, B1.1.2, B1.1.7, B1.1.8, B1.1.10, B1.1.11, B1.1.12 – as far as not yet curated); the DataSet content is checked against the resource requirements. B1 defects of SF-056 are recorded as `edits`; the guideline correction of SF-056 only on PO request. Payload columns of the matrix follow in C-028.
- New ambiguities become SF-062 ff. in `oi4-oec-documentation/findings/SPEC-FINDINGS.md` (Closed table kept ascending) with a proposed interpretation. Guideline text unchanged, no new GL-ADRs.
- At the end: log, STATUS (ROADMAP/AGENTS/GUARDRAILS only if the baseline changes), views (`spectool render`), short summary with the points for the PO.

## Scope / acceptance criteria

- `oec/spec/requirements/res-mam.yaml`, `res-health.yaml`, `res-profile.yaml`, `res-publist.yaml`, `res-refdes.yaml`, `pag.yaml`, `loc.yaml` (`status: draft`); `areas.yaml` – the seven areas `drafted`.
- Inventory items: RES-MAM 64, RES-HEALTH 27, RES-PROFILE 18, RES-PUBLIST 61, RES-REFDES 43, PAG 28, LOC 3 (244 in total) – 0 open in `coverage/report.md`.
- Reference checks (`tools/internal/payload`) extended by the DataSet content of MAM, Health, Profile, PublicationList, ReferenceDesignation and the helper DataSets PaginationRequest, Pagination, Locale (strict/tolerant), tagged with the new REQ ids; tests.
- Golden payload vectors V-PAY-026 ff. (`draft`) for the not yet curated B1 examples of these resources. The Status events of B1.1.8/B1.1.10 belong to RES-EVENT and follow in C-028.
- New spec findings registered (register-only commit in the guideline repo, no re-baseline – ADR-0001 amendment C-018).
- `go vet`, `gofmt -l`, `go test ./...`, `spectool lint`, `render -check`, `coverage -check`, `topic -check`, `inventory -check -rev c6453c7` pass.

## Inputs (sources read, with revisions)

- Guideline `c6453c7`: MG 5, MG 7.4, MG 7.5 (index, 7.5.1), MG 8.2.3, MG 8.3 index, 8.3.1, 8.3.2, 8.3.7, 8.3.8, 8.3.10, 8.3.11 (+ 8.3.11.1, 8.3.11.2), 8.3.12, MG 9 index, 9.1 index, 9.1.1, 9.1.2, 9.1.7, 9.1.8, 9.1.10, MG B1.1.1, B1.1.2, B1.1.7, B1.1.8, B1.1.10, B1.1.11, B1.1.12, GL-ADR-018; register SF-056.
- Project: AGENTS, README, GUARDRAILS, STATUS, ROADMAP, OPEN-QUESTIONS, C-026 log incl. all addenda, `oec/spec/README.md`, `requirements/` (CMP, TOP, MSG, IL, LIFE), `matrix/method-resource.yaml`, `tools/internal/payload`, `vectors/payload/`.

## Work done

Guideline repo (`oi4-oec-documentation`, branch `feature/spec-findings`, local only):

- `5ada306`: register – new **SF-062 … SF-068** (`open (C-027)`), each with a proposed interpretation; header "SF-062 … SF-068 found in C-027, open", next free id **SF-069**. Register-only commit: guideline text unchanged, **no re-baseline** (`oec/spec` stays on `c6453c7`, ADR-0001 amendment C-018). The pre-existing local change `.yarn/install-state.gz` was left untouched.

Workspace:

- `5d85b77`: this log (plan).
- `8a96356`, `d64fb31`, `fedf077`: requirements (all `draft`, 79 in total):
  - `res-mam.yaml` – **REQ-RES-MAM-001 … 022**: MAM fields (001), equal to the OPC UA VendorNameplate (002), uniform manufacturer name (003, SHOULD), ManufacturerUri domain without scheme (004) and matching the IL domain (005, SHOULD), generated Oi4Identifier uses the MAM values (006), Model (007), HardwareRevision incl. "undefined"/"" (008), four-number revisions (009, SHOULD), SoftwareRevision (010) and "" without software (011, SHOULD), DeviceManual URL or "" (012) as PDF (013, SHOULD), DeviceClass (014, SHOULD), SerialNumber (015), ProductInstanceUri = Identification Link (016) ≤ 255 characters (017, MUST NOT), RevisionCounter (018), detected components get IL + MAM (019, SHOULD), Get/Pub payloads (020 … 022).
  - `res-health.yaml` – **REQ-RES-HEALTH-001 … 007**: fields (001), Health = most severe Detail (002, implicit, SF-065), NE107 symbols/colors (003, SHOULD), a change of Details triggers a publication (004), Get/Pub payloads (005 … 007).
  - `res-profile.yaml` – **REQ-RES-PROFILE-001 … 005**: field Resources (001), lists exactly the served resources incl. MAM, Health, Profile (002), Get/Pub payloads (003 … 005).
  - `res-publist.yaml` – **REQ-RES-PUBLIST-001 … 025**: entry fields (001), entry ↔ DataSetMessage/topic (002, implicit), uniqueness of Resource/Source/Filter (003), speaking Filter names (004, SHOULD), entry DataSetWriterId = the one of the publication (005, implicit), the publisher assigns the ids (006, SHOULD), the Mode values (007 … 011), Mode fits the publication forms of the resource (012, implicit, SF-067 (c)), Interval/Precisions (013 … 015), Config (016, implicit), republication on change (017) and partial republication (018, MAY), Get/Pub/Set payloads (019 … 022), Set protected by ACL (023, platform), status Event after Set (024, `{SHOULD, MUST, MUST}` like REQ-TOP-014), republication after Set with ReplyTo (025).
  - `res-refdes.yaml` – **REQ-RES-REFDES-001 … 010**: Function/Product/Location objects (001), persisted (002), Get/Pub/Set/Del payloads (003 … 007; Set replaces, Del deletes the complete object – SF-068 (e)/(f)), ACL (008, platform), status Event after Set/Del (009, `{SHOULD, MUST, MUST}`), republication on `Pub/ReferenceDesignation/<Source>` (010, SF-068 (b)).
  - `pag.yaml` – **REQ-PAG-001 … 008**: PaginationRequest fields (001), answered page by page, page 0 = all pages, page beyond the end = empty page (002), Pagination fields (003) and consistent values (004, implicit, SF-063), pagination of too many DataSetMessages (005), re-trigger with PaginationRequest (006), consistent pages (007), PaginationRequest only in Get, Pagination only in Pub (008, implicit, SF-062 (b)).
  - `loc.yaml` – **REQ-LOC-001/002**: Locale field (001), a Get with Locale is answered in the requested language, otherwise English; Locale ignored in other methods (002, implicit, SF-062 (c)/(e)).
  - Grading: the resource requirements apply to the served resources; which resource is mandatory at which level stays in CMP (REQ-CMP-006/007/018/020). Set/Del status Events follow REQ-TOP-014/015 (`{SHOULD, MUST, MUST}`); ACL rules are `domain: platform`.
- `356cdd8`: **REQ-CMP-018 (reviewed)** gets the inventory key `MG-9.1.7-6f3f54c3` ("`Profile` itself is mandatory for … Level 3 and recommended for Level 2"). Reason: the 9.1.7 sentence restates exactly the reviewed grading of REQ-CMP-018 (MG 1.3); a second requirement would duplicate it. Statement, levels and status unchanged – inventory mapping only.
- `b062c17`: `coverage/informative.yaml` – 36 entries (RES-MAM 8, RES-HEALTH 6, RES-PROFILE 5, RES-PUBLIST 9, RES-REFDES 4, PAG 4; introductions, DataSetClassId pointers, examples, motivation, each with a reason); `areas.yaml` – the seven areas `drafted`; `coverage/report.md`.
- `7c1f6bf`: reference checks `tools/internal/payload/content.go` (new) – DataSet content per resource and method, MUST rules only, tagged with the new REQ ids:
  - helpers: PaginationRequest (REQ-PAG-001), Pagination fields and consistency within one NetworkMessage (REQ-PAG-003/004: Page ≥ 1, count ≤ PerPage and equal to the expected page size, HasNext, PaginationId a MessageId of the publisher and on page 1 the own MessageId), placement (REQ-PAG-008), Locale (REQ-LOC-001);
  - Get without resource DataSetMessage (REQ-RES-MAM-020, HEALTH-005, PROFILE-003, PUBLIST-019, REFDES-003), Set with exactly one (PUBLIST-022, REFDES-006), Del without (REFDES-007);
  - MAM (001/004/012/016/017), Health (001/002), Profile (001/002), PublicationList entries (001/002/003/012, reserved entry ids REQ-MSG-020), ReferenceDesignation (001); undefined keys REQ-MSG-001 in strict mode only (SF-052).
  - `payload.go` passes the DataSetMessages and PublisherId/MessageId to the content checks; `payload_test.go` new `TestContent`; `README.md` paragraph.
  - `vectors/vectors_test.go`: the synthetic Health fixture of `TestCheckPayload` gets `"Health": "NORMAL_0"` (an empty Health payload is now a content error); test-only change, no vector touched.
  - All 25 reviewed payload vectors (V-PAY-001 … 025) stay valid unchanged; their notes "checked in C-027/C-028" (V-PAY-006) are now partly fulfilled – no change to reviewed vectors.
- `296c4c0`: golden payload vectors **V-PAY-026 … 047** (`draft`, `vectors/payload/guideline.yaml`, header/description updated):
  - B1.1.1 MAM: 026/027 Get (explicit, all), 028 Pub explicit, 029 Pub all (comment-only objects removed, SF-056 (b)).
  - B1.1.7 Profile: 030/031 Get, 032 Pub explicit, 033 Pub all (comment objects removed). Kept with the incomplete resource list (SF-066 (b), noted).
  - B1.1.8 PublicationList: 034 … 036 Get (entry, asset, full), 037 Pub explicit entry, 038 Pub all of the asset, 039 Pub full (comment objects removed; Event entry `Mode` `APPLICATION_2` → `FILTER_4`, `corrected`, SF-067 (c)), 040 Set entry.
  - B1.1.10 ReferenceDesignation: 041/042 Get, 043 Pub explicit, 044 Pub all, 045 Set (placeholders `instantiated` with IEC 81346 example values – Function `=P1.L2.FDS1`, Product `-P1.L2.FDS1`, Location `+P1.H3.R2`, parents and parent ILs – incl. the missing `>` of one placeholder; `DataSetWriterId` 93 → 94, SF-056 (d)/(f); in 044 also comment objects removed), 046 Del (envelope of the requester: `MessageId`, `PublisherId`, `ReplyTo` `corrected`, SF-068 (a)).
  - B1.1.12: 047 Pub MAM with Locale (verbatim; answer to V-PAY-007).
  - Every vector with a JSON-Pointer edit keeps the verbatim example in `original`; the Status events of B1.1.8/B1.1.10 are left for RES-EVENT (C-028). B1.1.2 (V-PAY-001 … 004), B1.1.11 (005/006) and the B1.1.12 Get (007) were already curated.
- `04300bc`: constructed invalid vectors **V-PAY-048 … 052** (`draft`, `vectors/payload/constructed.yaml`): HealthScore 101 (REQ-RES-HEALTH-001), Health NORMAL_0 with a FAILURE_1 Detail (REQ-RES-HEALTH-002), HasNext true on the only page (REQ-PAG-004), Get MAM with a MAM DataSetMessage (REQ-RES-MAM-020), Event entry with Mode APPLICATION_2 (REQ-RES-PUBLIST-012) – each derived from a golden vector, each violating exactly one rule.
- `251fa8a`: Health check – CHECK_FUNCTION_2 and MAINTENANCE_REQUIRED_4 share one severity rank (the order of the two "low severity" states is open, SF-065 (a)); aligns the check with the note of REQ-RES-HEALTH-002; three new test cases.
- `b8c9da0`: views (`spectool render`): seven new area pages, README index, `cmp.md`, vector views.

No reviewed area other than the inventory key of REQ-CMP-018 (above) and no reviewed vector was changed.

## Verification

- `go vet ./...`, `gofmt -l .` (empty), `go test ./...` ok.
- `spectool lint` ok: 14 area files, 291 requirements, 138 informative items; matrix 66 combinations; vectors topic=146, identification-link=29, dnp-encoding=15, oi4identifier=13, payload=52.
- `spectool render -check`, `coverage -check`, `topic -check` ok; `inventory -check -rev c6453c7` ok (1279 items).
- Coverage: **RES-MAM 64 (56 covered, 8 informative)**, **RES-HEALTH 27 (21/6)**, **RES-PROFILE 18 (13/5)**, **RES-PUBLIST 61 (52/9)**, **RES-REFDES 43 (39/4)**, **PAG 28 (24/4)**, **LOC 3 (3/0)** – 0 open each.
- All quotes verbatim (lint); `original` of every edited vector equals its B1 block (vector check: verbatim in the origin section); every edit target resolves. Negative check: V-PAY-039 with the original Event `Mode` `APPLICATION_2` is reported as REQ-RES-PUBLIST-012 (temporary change, reverted).

## Decisions / assumptions (agent, G-4.4)

- Get/Pub/Set/Del payload rules are written per resource (REQ-RES-*-Get/Pub/Set/Del), the helper placement once in PAG/LOC (SF-062 (a)/(b)).
- Keyword-less definitions that the checks need are implicit MUSTs with refs SF-035 + the specific SF: REQ-RES-HEALTH-002 (SF-065 (b)), REQ-RES-PUBLIST-002/005/012/016 (SF-067), REQ-PAG-004/008 (SF-063/SF-062), REQ-LOC-002 (SF-062).
- Health severity: proposal F > S > C > M > N in the statement (SF-065 (a)); the checks treat C and M as equal until decided.
- PublicationList: all entries share the DataSetWriterId of the PublicationList (note of MG 9.1.8, B1.1.8: 161) – accepted by the checks, contradiction to REQ-MSG-021 in SF-067 (a). Entry `DataSetWriterName` = `Source`, `WriterGroupName` = `<ResourceType>[/<Tag>]` (REQ-MSG-025, GL-ADR-018).
- Mode vs. forms (REQ-RES-PUBLIST-012): Event only `OFF_0`/`FILTER_4`, Metadata only `ON_REQUEST_FILTER_9`, MAM/Health must enable a periodic form – proposal SF-067 (c); the B1.1.8 Event entry is corrected in the vectors.
- Profile lists the services too (`NewDataSetWriterId`, SF-066 (a)); the check requires MAM, Health and Profile in the list.
- DeviceManual `""` when no manual exists (SF-064 (a), used by B1.1.11/B1.1.12).
- B1.1.10: placeholders replaced by plausible IEC 81346 values (functional `=`, product `-`, location `+`); the Del example is read as a request of the IT connector (SF-068 (a)).
- B1.1.12 Pub (V-PAY-047): the device keeps `en-US` while the application answers in `de-DE` – read as the fallback of REQ-LOC-002.
- Pagination: one NetworkMessage is checked in isolation; ordering across a series is a D7 monitor task (REQ-PAG-004 note).

## Findings

| Id | Topic | Proposed interpretation (short) | Blocks |
|---|---|---|---|
| SF-062 | Helper DataSets per method; "Pagination for every Method"; Locale in non-Get methods, Optional only field; Get with Source/fallback; typos | Get: only PaginationRequest/Locale; Pub: resource DSMs + Pagination; Locale ignored outside Get; Locale field Mandatory; fallback English | no |
| SF-063 | Pagination semantics (PerPage, Page start/0, TotalCount, HasNext, PaginationId, page beyond the end) | page size, pages from 1, TotalCount without helpers, HasNext = Page·PerPage < TotalCount, PaginationId = MessageId of page 1, empty page | no |
| SF-064 | MAM: DeviceManual without manual, "undefined" vs "", GlobalAssetId "fused?", Model text for the Oi4Identifier | `""`; as written; read only; the Text | no |
| SF-065 | Health severity order of the two low states; keyword-less rule | F > S > C > M > N; MUST | no (checks tolerant) |
| SF-066 | Profile: services listed?, incomplete B1.1.7, Profile of subordinate assets | yes; editorial; what the application serves for it | no |
| SF-067 | PublicationList: shared DataSetWriterId vs REQ-MSG-021, DSWName template, Mode vs topic forms, 3.4·1038, Interval, Set payload, Event topic `Status/<StatusCode>`, ReplyTo of the republication | exception for list entries; Source; allowed Mode per resource; 10^38; …; `<EventLevel>`; ReplyTo of the Set | no (vectors carry the edit) |
| SF-068 | ReferenceDesignation: B1.1.10 Del envelope, republication topic, wrong links, Location copy-paste, extent of Set/Del | requester envelope; `Pub/ReferenceDesignation/<Source>`; editorial; Set replaces, Del deletes all | no (vector carries the edit) |

SF-056 is applied as edits in V-PAY-029, 033, 038, 039, 043 … 045 ((b), (d), (f)); its remaining parts concern B1.1.3 … B1.1.6 and B1.1.9 (C-028).

## Points for the PO

1. Decide **SF-062 … SF-068** – in particular **SF-067 (a)** (shared DataSetWriterId of PublicationList entries vs REQ-MSG-021), **SF-067 (c)** (allowed Mode per resource; B1.1.8 Event `APPLICATION_2`), **SF-065 (a)** (order CHECK_FUNCTION_2 vs MAINTENANCE_REQUIRED_4), **SF-066 (a)** (Profile lists services), **SF-064 (a)** (DeviceManual `""`), **SF-068 (a)** (B1.1.10 Del envelope).
2. Review the views `oec/spec/requirements/view/res-mam.md`, `res-health.md`, `res-profile.md`, `res-publist.md`, `res-refdes.md`, `pag.md`, `loc.md` (implicit MUSTs, grading, informative classification) and the inventory-only change of REQ-CMP-018.
3. Review the vectors V-PAY-026 … 047 (`payload-guideline.md`, esp. the edits of 038/039, 043 … 046) and V-PAY-048 … 052 (`payload-constructed.md`).
4. Optionally: order the guideline correction of SF-056 (for B1.1.1, B1.1.7, B1.1.8, B1.1.10) and of SF-062 … 068, then re-baseline.

## Next

C-028 (RES-DATA, RES-META, RES-EVENT, RES-CONFIG, RES-SUBLIST; payload columns of the matrix; Status events of B1.1.3 … B1.1.10; remaining parts of SF-056), as planned in STATUS. The C-027 areas move to `reviewed` after the PO review.

## Addendum 1 – PO review of the findings (2026-10-19)

PO input (chat): **SF-056**, **SF-065**, **SF-066**, **SF-067**, **SF-068** reviewed and accepted. **SF-062**: pagination is a helper for outputs that are too large and should behave like the paging of typical REST APIs – mainly for `Pub` (consequence of a too large list) and `Get` (which page, e.g. the next); little sense for `Set`/`Del`, unless e.g. a `Config` is too large ("change the value on page 5") – to be checked; hint: there is (was) a message size limit, hard to calculate. **SF-063**: does it match typical REST behavior? **SF-064**: proposal fits, check it against the OPC UA specification. Views and vectors follow later.

Work done:

- Guideline repo `c170b05` (register only, no re-baseline): SF-056, SF-065 … 068 → `decided (PO, C-027 review 2026-10-19): proposed interpretation accepted; guideline text not changed yet`. SF-062 … 064 stay `open` with revised proposals:
  - **SF-062**: (h) no pagination for `Set`/`Del` – a `Set` already addresses exactly one DataSetMessage by `Source`/`Filter` (`Config`: `Context.Name`); page numbers are no stable addresses (they depend on `PerPage` and order; REST changes a resource by its id, not by page); a `Set` split over several messages needs transaction semantics (all mandatory config elements in one `Set`, MG 9.1.3; all-or-nothing; one Status event); a single DataSetMessage larger than the limit is a provider design error (split the `Config` into contexts). (g) new: the size limit exists as `MaxPacketSize` in RIG 4.1.1 (Message Bus storage, ≥ 262144 bytes, "A NetworkMessage … shall not exceed this size") but MG 8.3.11 does not reference it – "too many DataSetMessages" = the NetworkMessage would exceed `MaxPacketSize` or the requested `PerPage`.
  - **SF-063**: mapped to page-based REST paging (GitHub `page`/`per_page`, JSON:API `page[number]`/`page[size]`, Stripe `has_more`, OData `@odata.count`/`nextLink`): pages from 1, page size, last page may be smaller, total count without helpers, `HasNext` ≙ `has_more`, empty page beyond the end – all typical. Differences: `Page` 0 (all pages streamed) and the complete answer to a `Get` without `PaginationRequest` are publish/subscribe extensions; answers are published to all subscribers (requester finds its answer via `ReplyTo`); `PaginationId` orders a series, it is no cursor. New (h): the publisher may use a smaller page size than requested (REST servers cap `per_page`), e.g. for `MaxPacketSize`, announced in `PerPage`; (i) series = NetworkMessages for one `Get`, so a single-page answer carries its own `MessageId` as `PaginationId`.
  - **SF-064** checked against OPC 10000-100 (DI) 4.5.2 `IVendorNameplateType` and 4.7 `DeviceType`: (a) confirmed – `DeviceType` defaults for unsupported mandatory properties are the empty string (and empty LocalizedText, `RevisionCounter` -1); OPC UA also allows a file pathname for `DeviceManual`, not usable over the bus. New: (e) OPC UA recommends `SemanticVersionString` for revisions vs. MG "four numbers" (B1 uses "1.0.1"); (f) `RevisionCounter` counts configuration-data changes in OPC UA (MG: changes of the MAM) and -1 means "not supported"; (g) OPC UA allows a GUID as `ManufacturerUri`, MG requires a domain name; (h) OPC UA default "" vs MG "undefined". (c) `GlobalAssetId` is the AAS `globalAssetId` (IDTA-01001), not OPC UA.
- Requirements, checks and vectors are **not** changed yet: they already implement the accepted proposals (SF-065 … 068); SF-062 … 064 changes (e.g. REQ-PAG-004 "PerPage ≤ requested", REQ-RES-MAM-009/018, MaxPacketSize in REQ-PAG-005) follow after the PO decision. The guideline correction for the decided findings waits for the PO order.

Points for the PO:

1. Decide the revised **SF-062** (no pagination for `Set`/`Del`; `MaxPacketSize` as trigger), **SF-063** (REST mapping; smaller page size allowed) and **SF-064** (in particular (b) keep "undefined", (e) SemVer allowed, (f) OPC UA semantics and -1 for `RevisionCounter`).
2. Order (or not) the guideline correction of SF-056 and SF-062 … 068.
