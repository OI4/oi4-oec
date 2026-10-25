# C-036 – Actions, MAM, Health and Profile schemas and vectors

- **Date:** 2026-10-25
- **Phase:** P2 – JSON Schema (D1)
- **Baseline:** guideline `68d878d` (branch `feature/spec-findings`); register `29682b0` (SF-099 opened in this cycle, register only, no re-baseline)
- **Start:** workspace `fa9061d` (C-035 reviewed, all items `reviewed`)

## Intent

PO (chat, 2026-10-25): "Mache weiter mit C-036."

ROADMAP P2 planned C-036 as "Actions + core resources":
- the `actions` module and NewDataSetWriterId (`call-`/`reply-newdatasetwriterid`);
- the entry schemas of MAM, Health, Profile, PublicationList and ReferenceDesignation (13 topic forms);
- the areas ACT, RES-MAM, RES-HEALTH, RES-PROFILE, RES-PUBLIST, RES-REFDES;
- the gap REQs REQ-ACT-013, REQ-RES-HEALTH-007, REQ-RES-MAM-006/014.

## Split (agent decision, allowed by the PO)

**C-036 = Actions + MAM, Health, Profile. PublicationList and ReferenceDesignation → new C-037.** The later cycles shift by one:
- C-038 Data/Event/SubscriptionList;
- C-039 Config + `broker.json`;
- C-040 D7 `validate`;
- C-041 P2 exit review.

Reasons:

- The ACT area alone has 14 `schema`/`d7-validate` REQs. The Action messages have their own envelope (`ua-action-request`/`-response`), a closed `Status` object and a request/response pair per service. They needed 28 vectors (V-PAY-176 … 203) and a new dispatch branch in `message.schema.json`.
- MAM and Profile added 14 more vectors. They also brought one spec finding (SF-099, empty `Pub`) and an umbrella rule for the `Pub` content REQs that touches all nine resources.
- PublicationList (Pub/Get/Set/Del with per-entry rules) and ReferenceDesignation (Pub/Get/Set/Del with the `Query` children of GL-ADR-020) are each about as large as MAM + Profile together. Together they hold 8 of the 13 planned topic forms. With them, the cycle would have doubled again and the review sheet would have held about 100 drafts.
- The split follows the module boundary: C-037 needs no `actions` code, only the envelope, the helpers and the umbrella rule of this cycle.

## Work

### 1. Oracle (`298e709`)

The oracle has to report every REQ that the schemas enforce, so that lint keeps `violates` = oracle (ADR-0019 Consequences). New checks:

- **REQ-ACT-013 – payload encoding of NewDataSetWriterId.** Before C-036 every defect of the request/response Payload was REQ-ACT-021/022. Now:
  - a wrong JSON type, an out-of-range value (e.g. DataSetWriterId 70000 > UInt16) or an undefined field → REQ-ACT-013 (the Payload is encoded per the ActionMetaData, MG 8.2.18/9.2.1);
  - a missing field or a semantically wrong value (e.g. DataSetWriterId 8 = reserved, Resource `Status`) → REQ-ACT-021/022 as before.
- **REQ-ACT-003 for the `Status`.** An unknown key in the `Status` of an ActionResponse now reports REQ-ACT-003, like an unknown key in every other Action object.
- **REQ-RES-MAM-006 (heuristic).** The REQ: when the ProductInstanceUri is a generated Oi4Identifier, its elements are Model (en-US), ProductCode and SerialNumber of the MAM. A message does not say whether the identifier was generated. The oracle therefore only treats a ProductInstanceUri as generated when it has no scheme, has four segments and its first segment is the ManufacturerUri. Then the remaining segments must be Model, ProductCode and SerialNumber, plain or dnp-encoded, `nd` for an empty value.
  - **Risk:** a vendor ProductInstanceUri of exactly this shape that was not generated from the MAM would be reported (false positive). The PO decides whether this is acceptable (D-4).
- **REQ-RES-MAM-014.** The MAM of the application itself (`Source` = AppId) carries the DeviceClass `Oi4.<ServiceType>` of the topic. `Oi4.Utility` on an `OTConnector` topic is reported.
- Unit tests for all new checks and for the cases that must not be reported.

### 2. SF-099 – empty `Pub` of a resource (register `29682b0`, open)

Found while writing `pub-mam`/`pub-profile` and revising `pub-health`.

- **The conflict:** every resource section of MG 9.1.x says for `Pub`: "the payload contains `DataSetMessages` of type `<Resource>` and optional `Pagination`". MG 9.1 "Resources Without Content" requires that a `Get` for which the entity has no content is answered with `"Messages": []` (REQ-CMP-041). Read literally, 9.1.x forbids the answer that 9.1 demands.
- **Also open:** it is not stated how a `Get` whose `Source`/`Filter` addresses nothing the entity knows is answered: with an empty `Pub`, or not at all.
- **The C-033 PoC** of `pub-health` required a Health DataSetMessage or a Pagination (an `anyOf`, tagged REQ-RES-HEALTH-007). That rejects the answer REQ-CMP-041 demands.
- **Proposed interpretation:** "contains DataSetMessages of type X" names the kind, not a minimum. An empty `Pub` is valid as the answer to a `Get` without content, also for an unknown `Source`/`Filter`. An unsolicited empty `Pub` stays forbidden (REQ-CMP-042). The register entry has the proposed guideline text for 9.1 and 9.1.x.
- **Alternative:** an unknown `Source` is not answered (the requester times out). The schemas are the same in both cases.
- **Consequences here:** no REQ statement changes. The `pub-<resource>` entry schemas accept `"Messages": []`. The `anyOf` of `pub-health` is removed (`884b38c`), and probe P-11 (empty `Pub` Health) now expects valid.

### 3. `Pub` content REQs as umbrellas (`e686d3a`)

REQ-RES-MAM-022, REQ-RES-HEALTH-007, REQ-RES-PROFILE-005, REQ-RES-PUBLIST-021 and REQ-RES-REFDES-005 say "a `Pub` contains DataSetMessages of type X and optional Pagination". This is not one constraint. The oracle reports each violation as a detail REQ:

| Defect | Detail REQ |
|---|---|
| DataSetMessage of another resource | REQ-MSG-010 (DataSetClassId) |
| DataSetMessage of the resource with a wrong content | content REQ (e.g. REQ-RES-MAM-001, REQ-RES-PROFILE-001) |
| PaginationRequest in a `Pub` | REQ-PAG-008 |
| Locale in a `Pub` | REQ-LOC-003 |
| Query in a `Pub` | REQ-QRY-001 |

Lint requires `violates` = oracle, so no vector can name the umbrella. Per ADR-0019 §9 they become umbrellas in `coverage/implications.yaml`. Each umbrella counts as covered when all its details are covered. The remainder ("an empty `Pub` only as the answer to a `Get`") is named in the map and stays `d7-monitor` via REQ-CMP-042.

REQ-RES-CONFIG-018, REQ-RES-DATA-008 and REQ-RES-SUBLIST-009 have the same shape. They are decided in their cycles (C-038/C-039), because their details are not built yet.

### 4. Schemas (`884b38c`)

| File | Content |
|------|---------|
| `actions` (new) | Action NetworkMessage `ua-action-request`/`-response` (REQ-ACT-001/002/003), MessageId with timestamp (REQ-ACT-004), RequestorId (REQ-ACT-005 structural part), ResponseAddress as a Reply topic (REQ-ACT-006 structural part), ActionRequest (REQ-ACT-010: fields, ActionState 1), ActionResponse (REQ-ACT-012: fields, ActionState 1 or 2), `Status` in CompactEncoding (REQ-ACT-014: `{}` for Good, otherwise a Code ≠ 0, no Symbol), Bad `Status` → Payload without fields (REQ-ACT-012: Code 2147483648 … 4294967295 → `maxProperties 0`), DataSetWriterId/ActionTargetId of the service (REQ-ACT-019), NewDataSetWriterId request/response (REQ-ACT-021/022, encoding REQ-ACT-013) |
| `resources/mam` (new) | MAM DataSet (REQ-RES-MAM-001: required fields, RevisionCounter Int32; REQ-RES-MAM-004 ManufacturerUri; REQ-RES-MAM-012 URIs; REQ-RES-MAM-017 lengths) and its DataSetMessage |
| `resources/profile` (new) | Profile DataSet (REQ-RES-PROFILE-001: `Resources`, each a resource of MG 7.5.1 or a common service of MG 7.5.2, listed once) and its DataSetMessage; that the list is complete (REQ-RES-PROFILE-002) is `d7-test` |
| `topics/pub-mam`, `topics/pub-profile` (new) | `Pub`: DataSetMessages of the resource, optional Pagination, empty list allowed (SF-099) |
| `topics/get-mam`, `topics/get-profile` (new) | `Get`: empty or PaginationRequest/Locale; a Profile DataSetMessage in a `Get` → REQ-RES-PROFILE-003 |
| `topics/call-newdatasetwriterid`, `topics/reply-newdatasetwriterid` (new) | the NewDataSetWriterId item is required (DataSetWriterId 8) |
| `topics/pub-health` | `anyOf` removed (SF-099) |
| `message` | MAM and Profile dispatched by DataSetClassId; Action messages by `MessageType`, then the NewDataSetWriterId fields only for an item with DataSetWriterId 8 |
| `envelope` | description points to `actions` |

Technical note (ADR-0019 amendment C-036 point 4): `unevaluatedProperties` inside a `$ref`'d schema does not see properties that the referencing schema declares next to the `$ref`. The base `ActionNetworkMessage` therefore declares every key of both message types (`ResponseAddress`/`TimeoutHint` as `{}`). The request and response schemas then restrict them.

Entry schemas: **11 of 29** allowed matrix rows (5 before).

### 5. Harness `schematest` (`884b38c`)

- Probe P-11 (empty `Pub` Health) now expects valid (SF-099).
- New probes P-35 … P-44:
  - P-35/36: empty `Pub` MAM/Profile is valid;
  - P-37: an Uncertain `Status` keeps its Payload (valid);
  - P-38: lowest Bad code with a Payload field → REQ-ACT-012;
  - P-39 … P-44 in a new form `(message)`: topic-agnostic, against `message.schema.json` only. They cover REQ-ACT-021, REQ-ACT-022, REQ-ACT-012, a valid publisher-defined Action (structural check only), REQ-RES-MAM-001 and REQ-RES-PROFILE-001.
- Helpers `item()`/`payload()` for the probe mutations.
- 136 payload vectors routed to an entry schema (C-035: 73), 59 further valid vectors checked with the message schema only, 22 skipped (topic form without entry schema and invalid).

### 6. Vectors (`25ea9ab`, all `draft`)

Each negative has exactly one defect; `violates` is computed by the oracle. Bases: Call = V-PAY-008, Reply = V-PAY-009, MAM = V-PAY-028, Profile = V-PAY-032, Get Profile = V-PAY-030.

| Vectors | Violates | Defect |
|---------|----------|--------|
| V-PAY-176 | ACT-001 | Call with MessageType `ua-action-response` |
| V-PAY-177, 178 | ACT-002 | Call without TimeoutHint; Reply with ResponseAddress |
| V-PAY-179, 180 | ACT-003 | Call with DataSetClassId; Reply item with DataSetWriterName |
| V-PAY-181, 182 | ACT-004 | MessageId without timestamp; MessageId with the Responder as sender (rule R-3) |
| V-PAY-183, 184 | ACT-005 | RequestorId without ServiceType; Reply from another application (PublisherId …1200-0346, rule R-2) |
| V-PAY-185, 186 | ACT-006 | ResponseAddress with `Pub`; Reply topic of another application (rule R-2) |
| V-PAY-187, 188 | ACT-010 | ActionState 2; no RequestId |
| V-PAY-189, 190 | ACT-012 | ActionState 0; Bad `Status` with Payload fields |
| V-PAY-191 … 194 | ACT-013 | Resource 3 (number); Ttl `"10"` (string); undefined field `Name`; DataSetWriterId 70000 |
| V-PAY-195, 196 | ACT-014 | `Status` Code 0; `Status` with Symbol |
| V-PAY-197, 198 | ACT-019 | DataSetWriterId 7; ActionTargetId 2 |
| V-PAY-199, 200 | ACT-021 | Resource `Status`; Payload `{}` |
| V-PAY-201, 202 | ACT-022 | DataSetWriterId 8 (reserved); no Ttl |
| V-PAY-203 | – (positive) | Reply with BadInvalidArgument (Code 2158690304 = 0x80AB0000) and an empty Payload; reqs REQ-ACT-012/013/014 |
| V-PAY-204, 205 | RES-MAM-001 | no SerialNumber; RevisionCounter 2147483648 |
| V-PAY-206, 207 | RES-MAM-004 | ManufacturerUri `https://provider.com`; `Provider.com` |
| V-PAY-208, 209 | RES-MAM-012 | DeviceManual `www…` without scheme; DeviceManual `file:///…` without host |
| V-PAY-210 | RES-MAM-006 | ProductCode `FDS-002` ≠ the Oi4Identifier element (rule R-3) |
| V-PAY-211 | RES-MAM-016 | ProductInstanceUri `https://provider.com/FDS-001/1200-0345` ≠ DataSetWriterName (rule R-3) |
| V-PAY-212 | RES-MAM-017 | ProductInstanceUri and DataSetWriterName with 256 characters (on the V-PAY-029 topic without `Source`, so that R-2 does not fire) |
| V-PAY-213 | RES-MAM-014 | own MAM with DeviceClass `Oi4.Utility` on an `OTConnector` topic (rule R-2) |
| V-PAY-214 … 216 | RES-PROFILE-001 | extra entry `Status`; `Health` twice; Payload `{}` |
| V-PAY-217 | RES-PROFILE-003 | `Get` Profile carrying a Profile DataSetMessage |
| V-TOP-253 | ACT-020, TOP-013 | Reply NewDataSetWriterId with a Filter (topic vector, layer R-1) |

Notes:

- V-PAY-214 first replaced `Health` by `Status` in the list of V-PAY-032. Lint then also reported REQ-RES-PROFILE-002, because the oracle requires MAM, Health and Profile in every Profile. Fixed: the list keeps `Health` and gets `Status` as a 12th entry.
- REQ-RES-PROFILE-002 ("the Profile lists exactly the supported resources") is `d7-test` only. The schema does not check that MAM/Health/Profile are listed; only the oracle does, so the vectors keep them.
- V-TOP-253 names two REQs: REQ-ACT-020 (a Reply topic has no Filter) and REQ-TOP-013 (topic grammar of the Reply form). The topic grammar rejects it, so both are reported by the same check (R-1).

Rule-layer expectations (`oec/schema/tests/rule-layer.yaml`): V-PAY-182 (ACT-004, R-3), V-PAY-184 (ACT-005, R-2), V-PAY-186 (ACT-006, R-2), V-PAY-210 (MAM-006, R-3), V-PAY-211 (MAM-016, R-3), V-PAY-213 (MAM-014, R-2), and V-PAY-050 (PAG-004 Pagination arithmetic, R-3). V-PAY-050 is a reviewed vector from C-027 that is now routed (topic `Pub/MAM`) and needed its expectation; the vector itself is unchanged.

### 7. ADR-0019 amendment C-036 (agent proposals, PO review)

Appended to [ADR-0019](../decisions/ADR-0019-schema-architecture.md#amendment-c-036-2026-10-25--actions-and-core-resources-agent-proposals-within-this-adr-g-44-proposed-po-review-pending), status `proposed`:

1. Empty `Pub` (SF-099): `pub-<resource>` accepts `"Messages": []`, the `pub-health` `anyOf` is removed.
2. §9 umbrella rule applied to the `Pub` content REQs (§3 above).
3. §4 discrimination of Actions in `message.schema.json` (MessageType, then DataSetWriterId 8); publisher-defined Actions structural only.
4. §3 closures of the Action objects (REQ-ACT-003 incl. `Status`, REQ-ACT-013 for the NewDataSetWriterId Payload) and the `$ref`/`unevaluatedProperties` note.
5. §5 rule-layer additions: R-2 (ACT-005, ACT-006, MAM-014), R-3 (ACT-004, MAM-016, the MAM-006 heuristic, PAG-004 expectation V-PAY-050).
6. Cycle numbers after the split.

## Changes to reviewed items

| Item | Change | Reason |
|------|--------|--------|
| V-PAY-001 | `reqs` + REQ-RES-HEALTH-005; `draft` | The vector already satisfies the REQ; positive coverage (ADR-0019 §8), as in C-034. Topic and payload unchanged. |
| V-PAY-003 | `reqs` + REQ-RES-HEALTH-001; `draft` | as above |
| V-PAY-004 | `reqs` + REQ-RES-HEALTH-001/002; `draft` | as above |
| V-PAY-008, 009, 119, 120 | `reqs` + REQ-ACT-013; `draft` | as above; the NewDataSetWriterId Call/Reply examples of the guideline are the positives of the encoding REQ. |
| V-PAY-028 | `reqs` + REQ-RES-MAM-006/014; `draft` | as above; the guideline MAM example is a generated Oi4Identifier and the DeviceClass of its own application. |
| `coverage/implications.yaml` (reviewed in C-034) | five umbrellas added | §3; ADR-0019 §9 rule; reviewed as part of D-2. |
| `pub-health` (PoC, accepted with ADR-0019) | `anyOf` removed | SF-099 (§2); D-1. |
| ADR-0019 (accepted) | amendment C-036, status line; the original cycle cut (decision 6) and the §9 cycle numbers stay as decided, point 6 of the amendment maps them | §7; D-2. |

While the 8 guideline vectors are `draft`, they do not count for coverage. Some MSG REQs therefore fall back temporarily (REQ-MSG-001/014/031 to "negative only", REQ-MSG-017 to "details open"). They return after the review.

## Coverage

- **Now** (drafts not counted): 159 `schema`/`d7-validate` REQs, **81 gaps** (77 before the cycle; the rise comes from the 8 re-drafted guideline vectors). Entry schemas 11 of 29. 217 payload vectors, 147 topic vectors.
- **After the PO review of this cycle** (projection on a temporary copy with every draft set to `reviewed`): **48 gaps**.
  - ACT: 14 of 14 covered (incl. the gap REQ REQ-ACT-013).
  - RES-MAM: 8 covered + REQ-RES-MAM-022 derived (incl. the gap REQs REQ-RES-MAM-006/014).
  - RES-HEALTH: 3 covered + REQ-RES-HEALTH-007 derived (gap REQ).
  - RES-PROFILE: 2 covered + REQ-RES-PROFILE-005 derived.
  - MSG: 36 covered + 2 derived.
- Left for C-037 … C-039: RES-PUBLIST, RES-REFDES, RES-DATA, RES-EVENT, RES-SUBLIST, RES-CONFIG, BUS and the remaining TOP/IL/CMP umbrellas.

## Commits

Workspace:

| Commit | Content |
|--------|---------|
| `298e709` | oracle: ACT-013 payload encoding, ACT-003 for the `Status`, MAM-006 heuristic, MAM-014; tests |
| `e686d3a` | `implications.yaml`: five `Pub` content umbrellas; coverage view |
| `884b38c` | schemas `actions`, `resources/mam`, `resources/profile`, six entry schemas, `pub-health` without `anyOf`, `message` dispatch; probes P-11, P-35 … P-44 |
| `25ea9ab` | V-PAY-176 … 217, V-TOP-253, reqs of 8 guideline vectors; expectations; views |
| (docs, two commits after `25ea9ab`) | log, ADR-0019 amendment, `oec/schema/README.md`; ROADMAP, STATUS, REVIEW, REVIEW_DE |

Guideline repo `oi4-oec-documentation`: `29682b0` register SF-099 (open).

## Checks (final)

- `spectool lint` (436 REQs, 217 payload vectors, 147 topic vectors), `render -check`, `coverage -check`, `topic -check`, `inventory -check -rev 68d878d` (1397 items), `go test ./...`: green.
- `oec/schema/check.sh` (gofmt, vet, `schematest`, jsonschema-rs): 42 schema files (21 strict + 21 tolerant), 375 cases, 0 disagreements between santhosh and jsonschema-rs. Python `jsonschema` (third, informative): 0 disagreements.

## For the PO (review)

Collected in [`REVIEW.md`](../REVIEW.md) (German explanation: [`REVIEW_DE.md`](../REVIEW_DE.md)):

1. D-1 SF-099: empty `Pub` as the answer to a `Get` without content (proposed interpretation; on PO order into the guideline + re-baseline).
2. D-2 ADR-0019 amendment C-036.
3. D-3 Cycle split: PublicationList/ReferenceDesignation → C-037, exit C-041.
4. D-4 REQ-RES-MAM-006 heuristic (false-positive risk).
5. R-1 V-PAY-176 … 203 (Actions), R-2 V-PAY-204 … 217 (MAM, Profile), R-3 V-TOP-253, R-4 `reqs` of the 8 reviewed guideline vectors.

Next free ids: SF-100, V-PAY-218, V-TOP-254, ADR-0020, Q-021.
