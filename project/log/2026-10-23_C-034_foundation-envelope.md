# C-034 – P2 foundation: coverage tooling, reclassifications, `ua-data` envelope and helpers, negative vectors

- **Date:** 2026-10-23
- **Phase:** P2 – JSON Schema (D1)
- **Baseline:** guideline `cc2fb4b` (branch `feature/spec-findings`), register `72dcd60` (SF-098 open, register-only, no re-baseline)
- **Start:** workspace `0a2825c` (C-033 closed, all items reviewed)

## Intent

PO (chat, 2026-10-23): "MG 8.3.12, REQ-LOC-002, REQ-LOC-003, V-PAY-127/128 reviewed und akzeptiert. Setze mit C-034 fort."

C-034 is the first build-out cycle of P2 (ROADMAP P2, ADR-0019 accepted): foundation and envelope, i.e. reclassifications, umbrella implication map, per-REQ coverage report, expectation list, schema layout, the modules `opcua`, `envelope`, `helpers` and (planned) `metadata`, and the areas MSG, LIFE, BUS, PAG, LOC and QRY with negative vectors.

## Split (agent decision, allowed by the PO)

The planned scope did not fit one reviewable cycle. MetaData moves to a new cycle C-035:

- The bare MetaData (`ua-metadata`, MG 8.2.5 … 8.2.15) is its own type system (DataSetMetaData, FieldMetaData, StructureDescription, EnumDescription, ConfigurationVersion …). It has 15 REQs (REQ-MSG-012, REQ-MSG-032 … 045), three of them gap REQs without any vector (REQ-MSG-042/043/045), and 8 bare MetaData vectors that are not yet routed.
- Doing it in C-034 would roughly double the schema and vector volume of this cycle. The PO review of 23 new negative vectors plus six changed reviewed vectors is already large.
- The `ua-data` envelope, helpers and the tooling are what every later cycle needs; MetaData is needed only by `Metadata` (C-037) and the bare MetaData vectors.

New cycle cut (ROADMAP P2): C-034 foundation + `ua-data` envelope (this cycle) · **C-035 MetaData** (new) · C-036 Actions + core resources · C-037 Data/Metadata/Event/SubscriptionList · C-038 Config + `broker.json` · C-039 D7 `validate` · C-040 P2 exit review. The former C-035 … C-039 shift by one.

## Work

### 1. Triage of the C-034 areas (MSG, LIFE, BUS, PAG, LOC, QRY)

| Item | Decision | Reason |
|------|----------|--------|
| REQ-MSG-026 (`Status.Code` ≠ 0) | +`schema` (`a6d2480`, back to `draft`) | ADR-0019 §8 candidate; `Code` 0 is a plain value constraint. The meaning of the code stays with the rule layer. |
| REQ-PAG-009 (no Pagination in `Set`/`Del`) | +`schema` (`a6d2480`, back to `draft`) | ADR-0019 §8 candidate; a helper DataSet in a `Set`/`Del` is visible in the payload plus topic method (entry schema). The note points to SF-098. |
| REQ-BUS-002 (topic addresses the receiving component) | `d7-validate` → `d7-test` (`23dc344`, back to `draft`) | A single message cannot show whether the topic's `ServiceType`/`AppId` is the addressed component; that needs the component's publications. It was a `d7-validate` REQ without any possible negative vector. |
| REQ-LIFE-003/007/009 (Birth/Close/Will content) | **deferred** – no reclassification, Q-020 | A Birth, Close or Will message looks exactly like a regular `Pub MAM`/`Pub Health`; from one message the validator cannot tell its role. ADR-0019 §8 named "REQ-LIFE-003 + `d7-validate`" as a candidate, but without a role input this REQ cannot be validated offline. **PO question:** give payload vectors an optional `role` (`birth`/`close`/`will`) and D7 `validate` a `--role` option, or keep LIFE-003/007/009 with `d7-test` only? |
| REQ-MSG-004, REQ-MSG-017 (content masks) | umbrellas (`implications.yaml`) | The content mask is not on the wire; its effect is the field presence of the details (MSG-003/005/011, MSG-016/018/024/029/031). Remainder: none. |
| REQ-BUS-004, REQ-BUS-005 (`Set`/`Del` content) | umbrellas (`implications.yaml`) | Details are BUS-010, MSG-030, TOP-013 and the per-resource `Set`/`Del` REQs; the remainder (object exists at the addressed component, complete updated state) stays with `d7-test` (REQ-BUS-002). Resource details close in C-036 … C-038. |
| REQ-CMP-004/005/025/026, REQ-TOP-002, REQ-IL-001/008 | maps deferred to C-039 (D7 `validate`) | Their details are topic/IL rule-layer REQs; the map is written together with the rule layer. |

### 2. Oracle (`8e58358`)

The reference checker (`oec/spec/tools/internal/payload`) now reports:

- REQ-MSG-030 for a DataSetMessage `MessageType` `ua-deltaframe`/`ua-keepalive` (with REQ-MSG-029);
- REQ-PAG-009 for a pagination helper in a `Set`/`Del` (with REQ-PAG-008);
- REQ-MSG-031 for a numeric value of a numbered OPC UA enumeration (`<Name>_<Value>` expected).

Unit tests cover delta/keep-alive frames, a numeric Health enum and a non-numbered enum.

### 3. Coverage tooling (`ce67cdb`)

- `oec/spec/coverage/implications.yaml` (+ schema): umbrella → details + remainder (ADR-0019 §9) for REQ-MSG-004/017 and REQ-BUS-004/005.
- `internal/reqcov` + `spectool coverage` write `coverage/vectors.md`: per `schema`/`d7-validate` REQ the reviewed positive and negative vectors (drafts marked `*`, not counted), the layer of each negative vector (schema / rule / R-n from the expectation list), derived umbrellas, and an entry schema per allowed matrix row. `coverage -gate` is the P2 exit gate (no gap, every allowed row has an entry schema); `coverage -check` keeps the report current.

### 4. Spec finding SF-098 (register `72dcd60`, open)

MG 8.3.11 says a `Del` contains no `DataSetMessage`; MG 9.1.9 (`Del SubscriptionList`) carries exactly one, and MG 8.2.3 speaks of "the `DataSetMessage`" of a `Del`. Proposed interpretation: the resource rule wins – "a `Del` contains at most one `DataSetMessage`, as defined for the resource in 9.1"; only REQ-PAG-009's statement would follow. Until decided, REQ-PAG-009 and the schema check only the helper DataSets (which both readings forbid).

### 5. Schemas (`5f209ed`, `3801755`)

- Layout per ADR-0019 §2: `v2/resources/health.schema.json` (moved from `v2/`).
- `envelope`: `Status.Code` 0 rejected (REQ-MSG-026), `ua-deltaframe`/`ua-keepalive` rejected (REQ-MSG-030).
- `opcua`: `$defs/VerboseEnum` (REQ-MSG-031); `HealthState` uses it.
- `helpers`: `QueryDataSetMessage` (DataSetWriterId 9, REQ-MSG-019/QRY-001); `message.schema.json` routes it.
- MG refs of the PoC comments corrected to the refs of the REQs (NetworkMessage MG 8.2.1, DataSetMessage MG 8.2.3, ConfigurationVersion MG 8.2.5, Pagination MG 8.3.11.2, reserved ids MG A2).
- **Contains-guard fix:** asserting `contains`/`maxContains` items no longer state `"type": "array"`; a non-array `Messages` reported a secondary REQ-PAG-008/LOC-003 besides REQ-MSG-005.
- **santhosh fail-fast:** `santhosh-tekuri/jsonschema` v6 stops evaluating a schema object after a failed `type`/`const`/`enum` and skips its siblings. V-PAY-144 (`ua-deltaframe`) reported only REQ-MSG-029 instead of 029 + 030, V-PAY-145 (`"Health": 0`) only REQ-RES-HEALTH-001 instead of + MSG-031. DataSetMessage `MessageType` and `HealthState` are now `allOf` items with one REQ each. ADR-0019 §1 got the editorial convention "one REQ per constraint object" (no decision changed).

### 6. Harness `schematest`

- New `comments.go`: every leading REQ id in a `$comment` must exist, must not be deprecated and must have verification `schema`; every reference in the parentheses must be in the refs of those REQs. It found the wrong MG refs of item 5.
- Option A now runs over all valid vectors with a topic: 37 vectors routed to an entry schema (B), 80 more valid vectors checked with `message.schema.json` only, 34 without a schema check yet (invalid without entry schema, or no topic).
- Expectation list `oec/schema/tests/rule-layer.yaml`: first entry V-PAY-148 (REQ-MSG-023 `DataSetWriterName` ≠ topic `Source`, rule R-2, schema part empty).

### 7. Vectors (`3801755`, all `draft`)

New constructed negative vectors V-PAY-129 … 151 (`vectors/payload/constructed.yaml`), one violation each, `violates` computed by the oracle:

| Vector | Violates | Base |
|---|---|---|
| V-PAY-129 | MSG-002 (`ua-metadata` on `Pub Health`) | V-PAY-003 |
| V-PAY-130 | MSG-003 (no `ReplyTo`) | V-PAY-003 |
| V-PAY-131 | MSG-005 (`Messages` is an object) | V-PAY-003 |
| V-PAY-132 | MSG-006 (`PublisherId` without ServiceType) | V-PAY-003 |
| V-PAY-133 | MSG-007 (`MessageId` without timestamp) | V-PAY-003 |
| V-PAY-134 | MSG-011 (no `DataSetClassId`) | V-PAY-003 |
| V-PAY-135 | MSG-015 (`ReplyTo` `"1639693510000"`) | V-PAY-003 |
| V-PAY-136 | MSG-016 (`PublisherId` in the DataSetMessage) | V-PAY-003 |
| V-PAY-137 | MSG-018 (`MetaDataVersion` and `MinorVersion`) | V-PAY-118 |
| V-PAY-138 | MSG-019 (Pagination with `WriterGroupName` "Paging") | V-PAY-003 |
| V-PAY-139 | MSG-020 (Health with reserved `DataSetWriterId` 5) | V-PAY-003 |
| V-PAY-140 | MSG-024 (`WriterGroupName` "Health") | V-PAY-003 |
| V-PAY-141 | MSG-026 (`Status.Code` 0) | V-PAY-118 |
| V-PAY-142 | MSG-027 (`Status` without `Code`) | V-PAY-118 |
| V-PAY-143 | MSG-029 (`ua-event`) | V-PAY-118 |
| V-PAY-144 | MSG-029 + MSG-030 (`ua-deltaframe`) | V-PAY-118 |
| V-PAY-145 | MSG-031 + RES-HEALTH-001 (`"Health": 0`) | V-PAY-003 |
| V-PAY-146 | LOC-001 (`Locale` "German") | V-PAY-001 |
| V-PAY-147 | QRY-001 (`Query` in `Get Health`) | V-PAY-001 |
| V-PAY-148 | MSG-023 (`DataSetWriterName` ≠ `Source`; rule layer only) | V-PAY-003 |
| V-PAY-149 | PAG-008 + PAG-009 (`Set Data` with Pagination) | V-PAY-061 |
| V-PAY-150 | BUS-010 (`SequenceNumber` in a `Set`) | V-PAY-061 |
| V-PAY-151 | MSG-025 + RES-PUBLIST-002 | V-PAY-037 |

Two expectations were corrected against the oracle while building: V-PAY-132 violates only REQ-MSG-006 (the `MessageId` suffix is compared only when the `PublisherId` is well-formed), V-PAY-151 also violates REQ-RES-PUBLIST-002.

## Changes to reviewed items

| Item | Change | Reason |
|------|--------|--------|
| REQ-MSG-026, REQ-PAG-009 | verification +`schema`, notes; `draft` | Reclassification planned in ADR-0019 §8 (triage above). |
| REQ-BUS-002 | verification `d7-validate` → `d7-test`, note; `draft` | Not decidable from one message (triage above). |
| V-PAY-003 | `reqs` +REQ-MSG-001, MSG-014; `draft` | Positive coverage: the vector already satisfies these REQs; MSG-001/014 had only negative vectors. |
| V-PAY-005 | `reqs` +REQ-PAG-001, PAG-008; `draft` | Positive coverage (PaginationRequest). |
| V-PAY-006 | `reqs` +REQ-PAG-003, PAG-004, PAG-008; `draft` | Positive coverage (Pagination answer). |
| V-PAY-007 | `reqs` +REQ-LOC-003; `draft` | Positive coverage (one Locale in a `Get`). |
| V-PAY-046, V-PAY-061 | `reqs` +REQ-PAG-009; `draft` | Positive coverage (`Set`/`Del` without helper DataSets). |
| ADR-0019 (accepted) | §1 editorial bullets (type guards of asserting schemas, one REQ per constraint object, reference lists), §2 note (Health in `resources/`), status line | Findings of the build-out; no decision changed. |

Payload and topic content of the six vectors is unchanged; only `reqs` grew.

## Coverage

- Now (drafts not counted): 159 `schema`/`d7-validate` REQs, 129 gaps; entry schemas 2 of 29 allowed rows.
- **After the PO review of this cycle** (projection on a temporary copy with every draft set to `reviewed`): 96 gaps. All C-034 REQs of MSG, PAG, LOC, QRY and REQ-BUS-010 are covered; REQ-MSG-004/017 are derived. Left in these areas: the MetaData REQs REQ-MSG-012, 032 … 045 (C-035) and the resource details of REQ-BUS-004/005 (C-036 … C-038).

## Commits

Guideline repo: `72dcd60` (SF-098, register only).

Workspace:

| Commit | Content |
|--------|---------|
| `a6d2480` | REQ-MSG-026/PAG-009 +`schema` |
| `8e58358` | oracle MSG-030/031, PAG-009 |
| `23dc344` | REQ-BUS-002 → `d7-test` |
| `ce67cdb` | `implications.yaml`, `reqcov`, `coverage/vectors.md`, `coverage -gate` |
| `5f209ed` | schema layout, MSG-026/030/031, Query helper, MG refs, contains-guard fix, `$comment` check, option A over all valid vectors |
| `3801755` | V-PAY-129 … 151, six vectors `reqs` extended, `allOf` split (santhosh fail-fast), expectation list V-PAY-148, ADR-0019 §1 note |
| (this commit) | log, STATUS, ROADMAP, OPEN-QUESTIONS (Q-020) |

## Checks (final)

- `spectool lint` (436 REQs, 151 payload vectors), `render -check`, `coverage -check`, `topic -check`, `inventory -check -rev cc2fb4b` (1397 items), `go test ./...`: green.
- `oec/schema/check.sh` (gofmt, vet, `schematest`, jsonschema-rs): 16 schemas, 172 cases, 0 disagreements between santhosh and jsonschema-rs. Python `jsonschema` (third, informative) agrees on all 172 cases.

## For the PO (review)

1. **Vectors:** V-PAY-129 … 151 (`vectors/view/payload-constructed.md`) and the six re-drafted vectors V-PAY-003/005/006/007/046/061 (`payload-guideline.md`).
2. **Reclassifications:** REQ-MSG-026, REQ-PAG-009 (+`schema`), REQ-BUS-002 (→ `d7-test`).
3. **Umbrella map** `oec/spec/coverage/implications.yaml` (REQ-MSG-004/017, REQ-BUS-004/005).
4. **SF-098** (`Del` with zero or one `DataSetMessage`): proposed "at most one, as defined for the resource in 9.1".
5. **Cycle split:** MetaData → new C-035, exit review → C-040.
6. **Decision Q-020 – LIFE roles:** optional vector field `role` + D7 `validate --role` (recommended: makes REQ-LIFE-003/007/009 offline-checkable for a declared Birth/Close/Will) or LIFE-003/007/009 with `d7-test` only.
7. ADR-0019 editorial notes (§1, §2).
