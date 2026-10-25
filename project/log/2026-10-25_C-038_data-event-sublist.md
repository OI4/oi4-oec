# C-038 – Data, Event and SubscriptionList schemas and vectors

- **Date:** 2026-10-25
- **Phase:** P2 – JSON Schema (D1)
- **Baseline:** guideline `a42a132` (branch `feature/spec-findings`, head `d4f79f3` = register only); no guideline change in this cycle, no re-baseline
- **Start:** workspace `1c0a9d9` (C-037 addendum 1 applied, all items `reviewed`)

## Intent

PO (chat, 2026-10-25): "D-1, D-2, R-1 und R-2 gereviewed und akzeptiert, bitte weitermachen" – apply the C-037 review (C-037 log, addendum 1), then run the next cycle of ROADMAP P2 / STATUS "Next step" 1: **C-038 – Data, Event, SubscriptionList** (areas RES-DATA, RES-EVENT, RES-SUBLIST), in the pattern of C-035 … C-037:

1. oracle checks for the payload REQs where missing; umbrellas per ADR-0019 §9;
2. resource modules, entry schemas, `message.schema.json` dispatch, probes from P-64; regenerate `dist/`;
3. vectors from V-PAY-231 (`draft`) so that every `schema`/`d7-validate` REQ of the three areas has a positive and a negative vector; projected gaps after the review;
4. architecture decisions as ADR-0019 amendment C-038 (`proposed`).

## Baseline check

All checks green at the start (`spectool lint`, `render -check`, `coverage -check`, `topic -check`, `inventory -check -rev a42a132`, `go test ./...`, `oec/schema/check.sh`): 437 REQs, 1398 inventory items, 230 payload and 147 topic vectors, 159 `schema`/`d7-validate` REQs with 35 gaps, entry schemas 18 of 29, 60 schema files, 439 cases, 0 disagreements.

## No split

The cycle stayed whole. 8 topic forms (as planned: Pub/Get/Set Data, Pub Event, Pub/Get/Set/Del SubscriptionList; `topic.abnf` and the matrix agree), three small modules, one oracle extension and 9 vectors. Data has no structure of its own (application-defined DataSets), Event and SubscriptionList are single objects.

## Work

### 1. Oracle and umbrellas (`d6d5010`)

**WriterGroupName on a topic without Filter (REQ-MSG-024).** `writerGroupName()` compared the WriterGroupName of a Data, Config or Event DataSetMessage with the topic Filter only when the topic had one. On `Pub/Data/<Source>` (and the Config/Event forms without Filter) any string passed. MG 8.2.3 and MG 7.7 make the WriterGroupName of these resources their Filter, so it must at least have the form of a Filter of the resource: one topic-encoded level for Data and Config (Tag, Context.Name), `event-filter` for Event. New helper `isFilterOf(resource, filter)` (per-level `filterLevelPattern` plus `isTagOf`); otherwise REQ-MSG-024. Three new cases in `payload_test.go` (fixture `tDataNoFilter`). No reviewed vector changes its verdict.

No other oracle check was missing: the Oi4Pv content (REQ-RES-DATA-004), the Event content and relations (REQ-RES-EVENT-001 … 008), the SubscriptionList entry (REQ-RES-SUBLIST-001/002) and the Get/Set/Del counts were already in the oracle.

**Umbrellas** (ADR-0019 §9, `coverage/implications.yaml`):

| Umbrella | Detail REQs | Remainder |
|---|---|---|
| REQ-RES-DATA-001 | REQ-MSG-016, 030, 031 | fields described by the Metadata of the same Source/Tag (d7-test, REQ-CMP-037) |
| REQ-RES-DATA-003 | REQ-MSG-024, REQ-RES-DATA-004 | an Oi4Pv DataSet under another Tag (manual, REQ-RES-DATA-002) |
| REQ-RES-DATA-008 | REQ-RES-DATA-001, 004, REQ-PAG-008, REQ-LOC-003, REQ-QRY-001 | empty `Pub` only as `Get` answer (REQ-CMP-042, d7-monitor) |
| REQ-RES-EVENT-009 | REQ-RES-EVENT-001, 002 | Number and Details of a Generic Event (REQ-RES-EVENT-010, manual) |
| REQ-RES-EVENT-011 | REQ-TOP-013, REQ-MSG-023, 024 | none |
| REQ-RES-SUBLIST-009 | REQ-MSG-010, REQ-RES-SUBLIST-001, REQ-PAG-008, REQ-LOC-003, REQ-QRY-001 | empty `Pub` only as `Get` answer (REQ-CMP-042) |

REQ-RES-DATA-001 (d7-validate only) and REQ-RES-EVENT-011 are "derived" at once (gaps 35 → 33); the other four follow when their details are covered.

### 2. Schemas (`eb7f6d3`)

New modules:

- `resources/data.schema.json`:
  - `Oi4Pv` (REQ-RES-DATA-004, SF-077): required `Pv`; key names `Pv` and `Sv1` … `Sv255` without leading zeros (`propertyNames`). The values are any JSON (described by the Metadata).
  - `DataDataSetMessage`: DataSetWriterId ≥ 10 (REQ-MSG-020); WriterGroupName = one topic-encoded level (REQ-MSG-024); MessageType `ua-keyframe` (REQ-MSG-029); the Payload of an application-defined DataSet is any object (envelope). If the WriterGroupName is `Oi4Pv`, the Payload is `Oi4Pv` (REQ-RES-DATA-003/004). This moves the Oi4Pv selection from the rule layer into the schema (ADR-0019 §4, see D-1).
- `resources/event.schema.json`:
  - `Event` (REQ-RES-EVENT-001, SF-075): required Number (UInt32) and Category (`CAT_SYSLOG_0`/`CAT_STATUS_1`/`CAT_GENERIC_99`, verbose enumeration); optional Description (String) and Details (object); closure REQ-MSG-001.
  - Category Status (REQ-RES-EVENT-003): Details only the String `SymbolicId`.
  - Category Syslog (REQ-RES-EVENT-007): Number ≤ 191, no Description, Details only the Strings `MSG` and `HEADER`.
  - `EventDataSetMessage`: WriterGroupName = `event-filter` (REQ-MSG-024), MessageType `ua-event` (REQ-MSG-029), and the relations between WriterGroupName and Payload: Category ↔ EventCategory (REQ-RES-EVENT-002), Status Number severity bits ↔ EventLevel and SymbolicId prefix (REQ-RES-EVENT-004; class 3 counts as Bad), Syslog PRI mod 8 ↔ EventLevel (REQ-RES-EVENT-008). The conditions follow the oracle, so one defect reports one REQ (e.g. EVENT-004 only with Category Status, a valid Number and a Status WriterGroupName).
- `resources/subscriptionlist.schema.json`:
  - `SubscriptionEntry` (REQ-RES-SUBLIST-001, SF-080): required TopicPath; optional Interval (UInt32) and Config (`NONE_0`/`CONF_1`); closure.
  - REQ-RES-SUBLIST-002: a TopicPath in the OEC namespace (`Oi4v2`, or starting with `+`/`#`) has a concrete resource level; topic filters of other namespaces are not checked (probe P-77). The derivation of the WriterGroupName from the TopicPath stays R-3 (expectation V-PAY-086).
  - `SubscriptionListDataSetMessage`: WriterGroupName starts with a ResourceType (REQ-MSG-025, GL-ADR-018), `ua-keyframe`.

Entry schemas (8, now **26 of 29**):

| Entry schema | Content |
|---|---|
| `pub-data` | Data DataSetMessages + at most one Pagination; `"Messages": []` valid (SF-099); no DataSetClassId const (Data has none) |
| `get-data` | only PaginationRequest and Locale; Query → REQ-QRY-001; a Data DataSetMessage → REQ-RES-DATA-006 |
| `set-data` | exactly one Data DataSetMessage (`contains`, REQ-RES-DATA-009); helpers rejected; REQ-BUS-010 |
| `pub-event` | Event DataSetMessages + at most one Pagination; empty valid; DataSetClassId const |
| `pub-subscriptionlist` | SubscriptionList DataSetMessages + at most one Pagination; empty valid |
| `get-subscriptionlist` | only PaginationRequest and Locale; a SubscriptionList DataSetMessage → REQ-RES-SUBLIST-007 |
| `set-subscriptionlist`, `del-subscriptionlist` | exactly one SubscriptionList DataSetMessage (REQ-RES-SUBLIST-010/011); helpers rejected; REQ-BUS-010 |

Further changes:

- `message.schema.json` dispatches Event and SubscriptionList by DataSetClassId; Data stays on the generic DataSetMessage (no predefined class id, no Oi4Pv check without the topic). Build-out state C-038.
- Probes **P-64 … P-82**: empty `Pub` Data/SubscriptionList (SF-099); Oi4Pv `Sv01`; Data WriterGroupName with two levels on a topic without Filter; Status Event with the SymbolicId of another class; Uncertain and class-3 Status Events (valid); Syslog PRI 192 and an extra Details key; Generic Event with free Details (valid); numeric Category; Event with `ua-keyframe`; SubscriptionList TopicPath with a wildcard resource level and outside the OEC namespace (valid); unknown Config; Del SubscriptionList with two entries; Query in Get Data; two `message` dispatch probes. Cross-check (temporary, not committed): for every probe the oracle on the base topic gives the schema verdict, except where the probe changed the WriterGroupName against the topic Filter (P-66, P-69, P-70) – an R-2 comparison the schema does not see.
- Expectation **V-PAY-086** (reviewed vector, unchanged, now routed by `pub-subscriptionlist`): schema none, rule REQ-RES-SUBLIST-002 (R-3) – the WriterGroupName is derived from the TopicPath of the same entry.
- The reviewed negatives V-PAY-081 … 085, 149 and 150 are now routed and report exactly their `violates` in the schema.
- `oec/schema/README.md`: state C-038, module list.

### 3. Vectors (`3558040`, reasons fixed in `a6bd841`)

9 constructed vectors **V-PAY-231 … 239** (`draft`); each negative mutates one reviewed vector with exactly one defect; `violates` = oracle (lint).

| Vector | Base | Content | violates / reqs | Layer |
|---|---|---|---|---|
| V-PAY-231 | 084 | valid Oi4Pv with `Pv` and `Sv1` | reqs DATA-003, DATA-004 | – |
| V-PAY-232 | 055 | Get Data with the DataSetMessage of V-PAY-061 | DATA-006 | schema |
| V-PAY-233 | 061 | Set Data with its DataSetMessage twice | DATA-009 | schema |
| V-PAY-234 | 063 | Status Event without Number | EVENT-001 | schema |
| V-PAY-235 | 063 | Status Event Details with an extra key `Code` | EVENT-003 | schema |
| V-PAY-236 | 064 | Syslog Event with a Description | EVENT-007 | schema |
| V-PAY-237 | 069 | SubscriptionList entry without TopicPath | SUBLIST-001 | schema |
| V-PAY-238 | 066 | Get SubscriptionList with the entry of V-PAY-072 | SUBLIST-007 | schema |
| V-PAY-239 | 072 | Set SubscriptionList without entry | SUBLIST-010 | schema |

The first draft of V-PAY-235 … 239 cited wrong MG subsections in `reason` (e.g. MG 8.3.11 for the SubscriptionList entry); `a6bd841` aligns them with the `refs` of their REQs (MG 8.3.6.1, 8.3.6.2, 8.3.9, 9.1.9). Both commits are in this cycle, the vectors were never reviewed.

No vector for the Generic Event: REQ-RES-EVENT-009 is an umbrella whose details EVENT-001/002 are covered by V-PAY-234 and V-PAY-083.

### 4. ADR-0019 amendment C-038 (`148d77b`, `proposed`)

1. §4 Oi4Pv selected by the WriterGroupName in the schema (was: rule layer by Filter/Tag).
2. §3 oracle: WriterGroupName on a Data/Config/Event topic without Filter must be a Filter of the resource (REQ-MSG-024).
3. §4 Data without DataSetClassId: no const, generic dispatch.
4. §4 step 4 Event relations (EVENT-002/004/008) in the schema; the oracle conditions are restated.
5. §4 Event/SubscriptionList dispatch in `message.schema.json`; MessageType per resource.
6. §3/§5 REQ-RES-SUBLIST-002 split: concreteness in the schema, derivation R-3.
7. §8 Get/Set/Del entry schemas as in C-037.
8. §9 the six umbrellas.

No new ADR, no new spec finding: every rule used here is decided (SF-062, SF-075 … 080, SF-098, SF-099, GL-ADR-018).

## Changes to reviewed items

| Item | Change | Reason |
|------|--------|--------|
| ADR-0019 (accepted) | amendment C-038 (`proposed`), changes the §4 sentence on Oi4Pv | §4; D-1. |
| Oracle `writerGroupName()` | WriterGroupName checked on topics without Filter | §1; D-2. No reviewed vector changes its verdict. |
| `coverage/implications.yaml` | six new umbrellas | §1; ADR-0019 §9 (accepted rule), part of D-1. |
| `tests/rule-layer.yaml` | new expectation for the reviewed V-PAY-086 | mechanical: the vector is routed for the first time; the split follows ADR-0019 §5 (R-3) and is part of D-1. The vector itself is unchanged. |

No REQ, vector, matrix row or reviewed implication changed.

## Coverage

- **Now** (drafts not counted): 159 `schema`/`d7-validate` REQs, **33 gaps** (35 → 33 by the umbrellas REQ-RES-DATA-001 and REQ-RES-EVENT-011). Entry schemas **26 of 29**. 239 payload vectors, 147 topic vectors.
- **After the PO review of this cycle** (projection on a temporary copy with every draft set to `reviewed`): **19 gaps** (−14):
  - RES-DATA: 004, 006, 009 covered; 003 and 008 derived.
  - RES-EVENT: 001, 003, 007 covered; 009 derived.
  - RES-SUBLIST: 001, 007, 010 covered; 009 derived.
  - BUS: REQ-BUS-004 derived (its last open details REQ-RES-DATA-009 and REQ-RES-SUBLIST-010).
- Left for C-039 … C-040 (19): CMP 4 (004, 005, 025, 026), IL 4 (001, 004, 005, 008), TOP 7 (002, 009, 011, 012, 018, 020, 021), RES-CONFIG 4 (010, 011, 015, 018).

## Commits

Workspace:

| Commit | Content |
|--------|---------|
| `d6d5010` | oracle: WriterGroupName without topic Filter; six umbrellas; coverage |
| `eb7f6d3` | schemas `resources/data`, `resources/event`, `resources/subscriptionlist`, 8 entry schemas, `message` dispatch; probes P-64 … P-82; expectation V-PAY-086; README |
| `3558040` | V-PAY-231 … 239; views, coverage |
| `148d77b` | ADR-0019 amendment C-038 (`proposed`) |
| `a6bd841` | V-PAY-235 … 239 reasons: MG subsections |
| (docs, after `a6bd841`) | this log; ROADMAP, STATUS, REVIEW, REVIEW_DE |

Guideline repo: no commit.

## Checks (final)

- `spectool lint` (437 REQs, 239 payload vectors, 147 topic vectors), `render -check`, `coverage -check`, `topic -check`, `inventory -check -rev a42a132` (1398 items), `go test ./...`: green.
- `oec/schema/check.sh` (gofmt, vet, `schematest`, jsonschema-rs): 82 schema files (41 strict + 41 tolerant), 518 cases (209 vectors routed to an entry schema), 0 disagreements between santhosh and jsonschema-rs.
- Python `jsonschema` + `referencing` (third validator, informative): 82 schemas, 518 cases, 0 disagreements.

## For the PO (review)

Collected in [`REVIEW.md`](../REVIEW.md) (German explanation: [`REVIEW_DE.md`](../REVIEW_DE.md)):

1. D-1 ADR-0019 amendment C-038.
2. D-2 Oracle: the WriterGroupName of Data/Config/Event on a topic without Filter.
3. R-1 V-PAY-231 … 233 (Data), R-2 V-PAY-234 … 236 (Event), R-3 V-PAY-237 … 239 (SubscriptionList).

Next free ids: SF-100, REQ-CMP-048, V-PAY-240, V-TOP-254, ADR-0020, Q-021, probe P-83. Next cycle: C-039 – Config + `broker.json` (ROADMAP P2).
