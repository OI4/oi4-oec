# C-037 – PublicationList and ReferenceDesignation schemas and vectors

- **Date:** 2026-10-25
- **Phase:** P2 – JSON Schema (D1)
- **Baseline:** guideline `a42a132` (branch `feature/spec-findings`, head `d4f79f3` = register only); no guideline change in this cycle, no re-baseline
- **Start:** workspace `062aa13` (C-036 addendum 3 applied, all items `reviewed`)

## Intent

PO (hand-over prompt, 2026-10-25): apply the review of C-036 addendum 2 (R-1 … R-3 accepted), then run C-037 as in STATUS "Next step" 1 and ROADMAP P2: PublicationList and ReferenceDesignation (Pub/Get/Set/Del, areas RES-PUBLIST and RES-REFDES):

1. oracle checks for the payload REQs of both resources where missing; umbrellas per ADR-0019 §9;
2. resource modules, the entry schemas, the `message.schema.json` dispatch, harness routes and probes from P-45; regenerate `dist/`;
3. vectors from V-PAY-218 (`draft`) so that every `schema`/`d7-validate` REQ of both areas has a positive and a negative vector; projected gaps after the review;
4. architecture decisions as ADR-0019 amendment C-037 (`proposed`).

## Baseline check

All checks green at the start (`spectool lint`, `render -check`, `coverage -check`, `topic -check`, `inventory -check -rev a42a132`, `go test ./...`, `oec/schema/check.sh`): 437 REQs, 1398 inventory items, 217 payload and 147 topic vectors, 159 `schema`/`d7-validate` REQs with 48 gaps, entry schemas 11 of 29, 42 schema files, 375 cases, 0 disagreements.

## C-036 review applied (before the cycle)

See the [C-036 log, addendum 3](2026-10-25_C-036_actions-mam-profile.md#addendum-3--po-review-of-addendum-2-2026-10-25): `737cb14` (`spectool review REQ-CMP-047`), `5be6f29` (REVIEW.md answers), `062aa13` (log addendum, STATUS/ROADMAP, review sheets reset).

## No split

The cycle stayed whole. Both resources reuse the envelope, the helpers and the `Pub` umbrella rule of C-036 and need no new module beyond their own two. The open work was 10 negative vectors, 7 entry schemas and one oracle extension, about half of C-036.

**Topic forms: 7, not 8.** ROADMAP and STATUS said "8 topic forms". `topic.abnf` and the matrix (`matrix/method-resource.yaml`) allow 7:
- `pub-`, `get-` and `set-publicationlist`;
- `pub-`, `get-`, `set-` and `del-referencedesignation`.

There is no `Del PublicationList`: the matrix row is not allowed (`Del` only for ReferenceDesignation and SubscriptionList, MG 7.4, MG 9.1.x). The "8" counted Pub/Get/Set/Del for both resources. ROADMAP and STATUS are corrected; no spec item changes.

## Work

### 1. Oracle (`f36c357`)

The oracle reports every REQ the schemas enforce, so that lint keeps `violates` = oracle (ADR-0019 Consequences). Before C-037 the PublicationList entry check only knew the field set, the JSON types and the Mode forms. New checks in `publicationEntry`:

- **Filter encoding (REQ-RES-PUBLIST-001).** The Filter of an entry is a topic-encoded String (MG 8.3.8 with MG 7.7, SF-067). Every `/`-separated level must be a `filter-level` of `topic.abnf`: no `+`, `#`, `/`, NUL or bare `%`; only the escapes `%2B`, `%23`, `%2F`, `%25`.
- **Tag structure (REQ-RES-PUBLIST-002).** The Filter has the structure of the resource's topic Filter:
  - Data, Metadata, Config: one level (Tag or Context.Name);
  - Event: `<EventCategory>/<EventLevel>` (`event-filter`: Syslog with its 8 levels, Status Good/Uncertain/Bad, Generic with one free level).
- **Mode as a verbose enumeration.** A numeric Mode (e.g. `8`) now reports REQ-MSG-031 and REQ-RES-PUBLIST-001, like every other verbose enumeration.
- Seven new unit cases in `TestContent`. The existing vectors are unaffected.

ReferenceDesignation needed no new oracle check: the object, the Query (REQ-RES-REFDES-012/013) and the Get/Set/Del counts were already in the oracle.

**Umbrellas:** REQ-RES-PUBLIST-021 and REQ-RES-REFDES-005 (`Pub` content) were already umbrellas since C-036 (`e686d3a`). No new umbrella was needed. With the new negatives, both and REQ-BUS-005 become "derived" after the review.

### 2. Schemas (`9bf33f7`)

New modules:

- `resources/publicationlist.schema.json`:
  - `PublicationEntry` (REQ-RES-PUBLIST-001) has the fields Resource (enum of the 10 resources), Source, Filter, DataSetWriterId, Mode, Interval, Precisions and Config.
  - The Filter has the topic-encoded pattern.
  - The described DataSetWriterId is ≥ 10 (REQ-MSG-020).
  - Mode and Config are verbose enumerations.
  - The closure is REQ-MSG-001.
  - REQ-RES-PUBLIST-002:
    - The Filter is required for Data/Metadata/Config/Event and forbidden for the other six resources.
    - The Filter of Data/Metadata/Config is one level.
    - The Filter of an Event entry matches the `event-filter` pattern.
  - REQ-RES-PUBLIST-012, the Mode forms per resource:
    - Event: only `OFF_0` or `FILTER_4`.
    - Metadata: only `ON_REQUEST_FILTER_9`.
    - MAM and Health: not `OFF_0` or `ON_REQUEST_1`.
    - Each rule is written as `required: [Mode]` + `not enum`, because a missing Mode means the default `ON_REQUEST_1`.
  - `PublicationListDataSetMessage` checks:
    - DataSetWriterId ≥ 10;
    - the WriterGroupName starts with a ResourceType (REQ-MSG-025, GL-ADR-018: the complete Filter `<ResourceType>[/<Tag>]`);
    - `ua-keyframe` (REQ-MSG-029).
- `resources/referencedesignation.schema.json`:
  - `ReferenceDesignation` (REQ-RES-REFDES-001) has the optional aspects Function, Product and Location. Each aspect has a required Value, an optional Local and an optional Parent. The Parent has a required Value, an optional Local and an optional IdentificationLink. Every level is closed.
  - `ReferenceDesignationDataSetMessage` has WriterGroupName `""` (REQ-MSG-024).
  - `ReferenceDesignationQuery` (REQ-RES-REFDES-012, GL-ADR-020) has a required non-empty Parent and an optional Aspect enum, and is closed. It also has its Query DataSetMessage (helpers `QueryDataSetMessage` refined).

Entry schemas (7, now **18 of 29**). Every entry schema requires and fixes the DataSetClassId (REQ-MSG-010/011). Item chains by reserved DataSetWriterId as in C-036:

| Entry schema | Content |
|---|---|
| `pub-publicationlist`, `pub-referencedesignation` | resource DataSetMessages + at most one Pagination; PaginationRequest, Locale, Query rejected; `"Messages": []` valid (SF-099) |
| `get-publicationlist` | only PaginationRequest and Locale (each at most once); Query rejected (REQ-QRY-001: no Query defined for PublicationList); a resource DataSetMessage reports REQ-RES-PUBLIST-019 |
| `get-referencedesignation` | PaginationRequest, Locale and the ReferenceDesignation Query (each at most once); a resource DataSetMessage reports REQ-RES-REFDES-003 |
| `set-publicationlist`, `set-referencedesignation` | exactly one resource DataSetMessage (`contains` an item with DataSetWriterId 10 … 65535, `minContains`/`maxContains` 1; REQ-RES-PUBLIST-022, REQ-RES-REFDES-006); every helper rejected (PaginationRequest/Pagination report REQ-PAG-008 and REQ-PAG-009, as in the oracle); no `SequenceNumber` (REQ-BUS-010) |
| `del-referencedesignation` | no DataSetMessage (REQ-RES-REFDES-007, SF-098); helpers rejected; REQ-BUS-010 |

Further changes:

- `message.schema.json` dispatches PublicationList and ReferenceDesignation by DataSetClassId. Helper items are selected by reserved id, with the ReferenceDesignation Query parameters for RD. Every other item is checked as the resource DataSetMessage. The build-out state is now C-037.
- Harness: no code change for routing (`loadPatterns` picks every rule with an entry schema).
- Probes **P-45 … P-63**:
  - empty `Pub` of both resources (SF-099);
  - Filter encoding and Tag structure;
  - Event/Metadata Mode rules;
  - numeric Mode;
  - described DataSetWriterId 5;
  - Query in Get PublicationList;
  - SequenceNumber and PaginationRequest in Set;
  - two Queries and an empty Parent in Get ReferenceDesignation;
  - a Locale in Del;
  - a WriterGroupName in Pub ReferenceDesignation;
  - two `message` dispatch probes.
- Expectation **V-PAY-151** (reviewed vector, unchanged) is now routed by `pub-publicationlist`. Its WriterGroupName `oee` violates REQ-MSG-025, which the schema sees. That it differs from `Data/oee` of the entry (REQ-RES-PUBLIST-002) is an equality between two fields → `rule: [REQ-RES-PUBLIST-002]`, R-3.
- `oec/schema/README.md`: state C-037, module list.

### 3. Vectors (`991dc0b`)

13 constructed negative vectors **V-PAY-218 … 230** (`draft`). Each mutates one reviewed guideline vector with exactly one defect; `violates` = oracle (lint).

| Vector | Base | Defect | violates | Layer |
|---|---|---|---|---|
| V-PAY-218 | 037 | entry without Source | PUBLIST-001 | schema |
| V-PAY-219 | 037 | Filter `o+e` unencoded (topic without list filter, so only the encoding is wrong) | PUBLIST-001 | schema |
| V-PAY-220 | 037 | same entry twice | PUBLIST-003 | rule R-3 |
| V-PAY-221 | 034 | Get PublicationList with the entry of V-PAY-040 | PUBLIST-019 | schema |
| V-PAY-222 | 040 | Set PublicationList without entry | PUBLIST-022 | schema |
| V-PAY-223 | 037 | Event entry with Filter `Status/Fine` (topic without list filter: `Status/Fine` is no valid topic Filter) | PUBLIST-002 | schema |
| V-PAY-224 | 037 | Mode `8` | MSG-031, PUBLIST-001 | schema |
| V-PAY-225 | 043 | aspect Function without Value | REFDES-001 | schema |
| V-PAY-226 | 041 | Get ReferenceDesignation with the object of V-PAY-045 | REFDES-003 | schema |
| V-PAY-227 | 045 | Set ReferenceDesignation without object | REFDES-006 | schema |
| V-PAY-228 | 046 | Del ReferenceDesignation with the object of V-PAY-045 | REFDES-007 | schema |
| V-PAY-229 | 053 | Query without Parent | REFDES-012 | schema |
| V-PAY-230 | 053 | the Query on the topic with Source (V-PAY-041) | REFDES-013 | rule R-2 |

New expectations in `tests/rule-layer.yaml`:
- **V-PAY-220** (R-3): uniqueness by the compound key Resource/Source/Filter. `uniqueItems` compares whole items, and two entries with the same key may differ in Mode or Interval.
- **V-PAY-230** (R-2): `get-referencedesignation` is one ABNF rule for the topic with and without Source, so the payload alone does not show the Source.

No positive vector was added: every REQ of both areas already had reviewed positives (V-PAY-034 … 046, 053, 054).

### 4. ADR-0019 amendment C-037 (`be5f045`, `proposed`)

1. §3 Filter checks of the PublicationList entry in oracle and schema (encoding → PUBLIST-001, presence/Tag structure → PUBLIST-002; the Event pattern restates `event-filter` and must follow the grammar).
2. §3 Mode forms per resource (`required` + `not enum`, missing Mode = default).
3. §8 Set/Del counting with `contains`, helpers rejected separately, Del without DataSetMessage, REQ-BUS-010 in every Set/Del entry.
4. §5 rule-layer additions:
   - R-2: REFDES-013 (V-PAY-230). Splitting `get-referencedesignation` by Source would need a second routing key next to the ABNF rule.
   - R-3: PUBLIST-003 (V-PAY-220).
   - R-3: the PUBLIST-002 equalities (V-PAY-151 split).
5. §4 PublicationList/ReferenceDesignation dispatch in `message.schema.json`; the method-dependent counts stay in the entry schemas.

No new ADR, no new spec finding: every rule used here is decided (SF-067, SF-068, SF-073, SF-098, SF-099, GL-ADR-018, GL-ADR-020).

## Changes to reviewed items

| Item | Change | Reason |
|------|--------|--------|
| ADR-0019 (accepted) | amendment C-037 (`proposed`), status line | §4; D-1. |
| `tests/rule-layer.yaml` (expectations reviewed with C-034 … C-036) | new expectation for the reviewed V-PAY-151 | mechanical: the vector is routed for the first time; the split follows ADR-0019 §5 (R-3) and is part of D-1. The vector itself is unchanged. |
| Oracle `publicationEntry` | Filter encoding/Tag structure, Mode as verbose enumeration | §1; D-2. No reviewed vector changes its verdict. |
| ROADMAP/STATUS "8 topic forms" | → 7 | correction, see "No split". |

No REQ, vector, matrix row or implication changed.

## Coverage

- **Now** (drafts not counted): 159 `schema`/`d7-validate` REQs, **48 gaps** (unchanged; the new vectors are `draft`). Entry schemas **18 of 29**. 230 payload vectors, 147 topic vectors.
- **After the PO review of this cycle** (projection on a temporary copy with every draft set to `reviewed`): **35 gaps** (−13):
  - RES-PUBLIST: 6 covered + REQ-RES-PUBLIST-021 derived (001, 003, 019, 022 new; 021 umbrella).
  - RES-REFDES: 6 covered + REQ-RES-REFDES-005 derived (001, 003, 006, 007, 012, 013 new; 005 umbrella).
  - BUS: REQ-BUS-005 derived (needs REFDES-007, now covered). REQ-BUS-004 stays open (2 of 10: REQ-RES-DATA-009, REQ-RES-SUBLIST-010 → C-038).
- Left for C-038 … C-040 (35): BUS 1, CMP 4, IL 4, TOP 7, RES-CONFIG 4, RES-DATA 6, RES-EVENT 5, RES-SUBLIST 4.

## Commits

Workspace:

| Commit | Content |
|--------|---------|
| `737cb14`, `5be6f29`, `062aa13` | C-036 review (addendum 3) |
| `f36c357` | oracle: PublicationList entry Filter encoding and Tag structure, Mode as verbose enumeration; tests |
| `9bf33f7` | schemas `resources/publicationlist`, `resources/referencedesignation`, 7 entry schemas, `message` dispatch; probes P-45 … P-63; expectation V-PAY-151; README |
| `991dc0b` | V-PAY-218 … 230; expectations V-PAY-220/230; views, coverage |
| `be5f045` | ADR-0019 amendment C-037 (`proposed`) |
| (docs, after `be5f045`) | this log; ROADMAP, STATUS, REVIEW, REVIEW_DE |

Guideline repo: no commit.

## Checks (final)

- `spectool lint` (437 REQs, 230 payload vectors, 147 topic vectors), `render -check`, `coverage -check`, `topic -check`, `inventory -check -rev a42a132` (1398 items), `go test ./...`: green.
- `oec/schema/check.sh` (gofmt, vet, `schematest`, jsonschema-rs): 60 schema files (30 strict + 30 tolerant), 439 cases (166 vectors routed to an entry schema), 0 disagreements between santhosh and jsonschema-rs.
- Python `jsonschema` + `referencing` (third validator, informative, own venv; all `dist/` schemas in one registry, each case via `{"$ref": case.schema}`): 60 schemas, 439 cases, 0 disagreements.

## For the PO (review)

Collected in [`REVIEW.md`](../REVIEW.md) (German explanation: [`REVIEW_DE.md`](../REVIEW_DE.md)):

1. D-1 ADR-0019 amendment C-037.
2. D-2 Oracle reading of the entry Filter (topic-encoded, Tag structure of the resource).
3. R-1 V-PAY-218 … 224 (PublicationList), R-2 V-PAY-225 … 230 (ReferenceDesignation).

Next free ids: SF-100, REQ-CMP-048, V-PAY-231, V-TOP-254, ADR-0020, Q-021, probe P-64. Next cycle: C-038 – Data, Event, SubscriptionList (ROADMAP P2).

## Addendum 1 – PO review (2026-10-25)

PO (chat, 2026-10-25): "D-1, D-2, R-1 und R-2 gereviewed und akzeptiert, bitte weitermachen". Recorded in [`REVIEW.md`](../REVIEW.md) (commit `be07dcc`, kept as the review record): D-1 ADR-0019 amendment C-037 **accepted**; D-2 oracle reading of the PublicationList entry Filter (topic-encoded, Tag structure of the resource) **accepted**; R-1 V-PAY-218 … 224 and R-2 V-PAY-225 … 230 **accepted**.

Review applied (`69a6bb4`, `spectool review V-PAY-218..230`):

- 13 drafts → `reviewed`; views re-rendered.
- Coverage (`coverage/vectors.md`): schema/d7-validate gaps 48 → **35** (as projected); entry schemas 18 of 29. RES-PUBLIST and RES-REFDES have no gap left.

ADR-0019 (`097b812`): status line and amendment C-037 heading say accepted.

Date correction (mechanical): this log was first written as `2026-10-26_C-037_publist-refdes.md`, but the whole cycle ran on 2026-10-25 (see the commit dates). The file is renamed to `2026-10-25_C-037_publist-refdes.md`; the dates in this log, the ADR-0019 amendment heading, STATUS, ROADMAP and REVIEW follow. No content changed.

No reviewed item changed beyond the accepted proposals; guideline unchanged at `a42a132`.

`REVIEW.md` / `REVIEW_DE.md`: reset to "nothing open"; C-038 fills them again.

Checks: `spectool lint` (437 REQs, 230 payload vectors, 147 topic vectors), `render -check`, `coverage -check`, `topic -check`, `inventory -check -rev a42a132` (1398 items), `go test ./...`, `oec/schema/check.sh` (60 schema files, 439 cases, 0 disagreements): green.

Next free ids: SF-100, REQ-CMP-048, V-PAY-231, V-TOP-254, ADR-0020, Q-021, probe P-64. Next cycle: C-038 – Data, Event, SubscriptionList (ROADMAP P2).
