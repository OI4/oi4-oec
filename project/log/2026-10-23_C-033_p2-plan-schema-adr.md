# C-033 – P2 start: P2 plan, schema architecture ADR-0019, JSON Schema proof of concept

- Date: 2026-10-23
- Phase: P2 (started in this cycle)
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Status: done; waiting for PO review (ADR-0019 `proposed`, SF-097 open)
- Guideline baseline: `oi4-oec-documentation` @ `5b8fe59`, branch `feature/spec-findings` (GL-ADR-001 … 022, SF-001 … 096 resolved, next free id SF-097). During the cycle, the register-only commit `decc2da` added SF-097 (open). The guideline text did not change, so no re-baseline was needed (STATUS "Next step" 3).

## Intent

PO order (chat, 2026-10-23): run C-033 as described in STATUS "Next step" 1 and ROADMAP P2. P1 is closed, and P2 (JSON Schema, D1) starts. The order has three parts:

1. Plan P2 with increments and cycles up to the P2 exit. The exit includes the 21 `schema`/`d7-validate` REQs without a vector from the C-031 gap list.
2. Write the schema architecture ADR (ADR-0019, `proposed`) with options and a recommendation.
3. Optionally build a small PoC that backs the ADR. The full build-out waits until the PO accepts the ADR.

## Plan

**No split.** The cycle covers only planning, the ADR and a PoC. These parts depend on each other: the PoC measures the options of the ADR, and the ADR sets the cut of the increments. The schema build-out is explicitly not part of C-033; it starts in C-034, after the ADR is accepted.

## Scope / acceptance criteria (C-033)

- The baseline at `5b8fe59` is verified, and all five `spectool` checks are green.
- ROADMAP P2 holds the architecture summary, the increments, the cycle cut up to the exit, and refined exit criteria including the 21 gap REQs.
- ADR-0019 is written with status `proposed`. It covers dialect, modules, `unevaluatedProperties`, discrimination, topic-aware validation (options with pros and cons and a recommendation), `$id`/versioning, location, test suite with two validators, and the verification split.
- The PoC covers the envelope, helpers and Health against the existing vectors, uses two independent 2020-12 validators, and has a local CI script.
- Spec ambiguities go to the register (SF-097 ff.).
- STATUS, ROADMAP, views and this log are updated, and every change is committed individually.

## Inputs

- ROADMAP P2 (before C-033) and STATUS "Next step" 1.
- ADR-0003 (JSON Schema 2020-12), ADR-0006/0013/0014 (D7/D8 in Go, sharing only `schema/` and `spec/`), ADR-0012 (monorepo `oec/`), ADR-0018 (REQ catalog, shared contract).
- `oec/spec/`: catalog, `topic-patterns.json`, matrix, vectors V-PAY-001 … 126, oracle `tools/internal/payload`.
- MG 8 (NetworkMessage, DataSetMessage, helpers), MG 9.1.x (resources), OPC UA Part 14 JSON encoding (via the MG).
- C-031 log, "Gap list for P2/D7".

## Work

### 1. Baseline check and fix (`fa96538`)

- The guideline repo is at `5b8fe59`. The register has no open finding, and the next free id was SF-097.
- `spectool coverage -check` **failed**: 29 keys in `oec/spec/coverage/examples.yaml` pointed to code-block keys that C-032 addendum 53 had re-keyed. Addendum 53 re-baselined the inventory but did not remap the examples file.
- The 29 keys were remapped 1:1 (same code block, new key) and the report was regenerated. Commit `fa96538`. No reviewed REQ, vector or matrix entry changed; only the derived examples mapping did.
- After the fix, all five checks are green:
  - `lint`
  - `render -check`
  - `coverage -check`
  - `topic -check`
  - `inventory -check -rev 5b8fe59` (1396 items)

### 2. P2 sizing (at `5b8fe59`)

- 159 non-deprecated REQs carry `schema` and/or `d7-validate`:

  | Vector coverage | REQs |
  |---|---|
  | positive and negative | 30 |
  | positive only | 93 |
  | negative only | 15 |
  | none | 21 (= the C-031 gap list) |

- Per area (total / pos+neg / pos only / neg only / none):

  | Area | tot | pn | p | n | none |
  |---|---|---|---|---|---|
  | ACT | 14 | 0 | 13 | 0 | 1 |
  | BUS | 4 | 0 | 4 | 0 | 0 |
  | CMP | 4 | 0 | 0 | 0 | 4 |
  | IL | 10 | 6 | 0 | 1 | 3 |
  | LIFE | 3 | 0 | 3 | 0 | 0 |
  | LOC | 1 | 0 | 1 | 0 | 0 |
  | MSG | 38 | 1 | 30 | 2 | 5 |
  | PAG | 5 | 0 | 0 | 4 | 1 |
  | QRY | 1 | 0 | 1 | 0 | 0 |
  | RES-CONFIG | 13 | 9 | 3 | 0 | 1 |
  | RES-DATA | 6 | 0 | 4 | 1 | 1 |
  | RES-EVENT | 8 | 3 | 5 | 0 | 0 |
  | RES-HEALTH | 4 | 0 | 0 | 3 | 1 |
  | RES-MAM | 9 | 1 | 6 | 0 | 2 |
  | RES-META | 3 | 1 | 2 | 0 | 0 |
  | RES-PROFILE | 3 | 0 | 3 | 0 | 0 |
  | RES-PUBLIST | 7 | 1 | 6 | 0 | 0 |
  | RES-REFDES | 7 | 0 | 7 | 0 | 0 |
  | RES-SUBLIST | 6 | 2 | 4 | 0 | 0 |
  | TOP | 13 | 6 | 1 | 4 | 2 |

- 29 topic forms (ABNF rules = allowed matrix rows), each with at least one payload vector (the largest: `pub-event` 14, `pub-config` 12, `set-config` 12, `pub-health` 11). Eight vectors are bare `DataSetMetaData` objects.
- The 21 gap REQs:
  - umbrella REQs: REQ-CMP-004/005/025/026, REQ-TOP-002, REQ-IL-001/008;
  - REQ-ACT-013, REQ-IL-005;
  - REQ-MSG-004/017/042/043/045, REQ-PAG-009;
  - REQ-RES-CONFIG-015, REQ-RES-DATA-003, REQ-RES-HEALTH-007, REQ-RES-MAM-006/014, REQ-TOP-018.

  Each one is assigned to a P2 cycle in ROADMAP P2.
- `broker.json` (REQ-CNT-003, verification `d8`): its note says a JSON Schema can be added with D8. P2 plans `platform/broker.schema.json` in C-037 and leaves the REQ unchanged.

### 3. P2 plan (ROADMAP P2)

ROADMAP P2 was rewritten. It now holds:

- the architecture summary (ADR-0019);
- the coverage list, now also naming `NewDataSetWriterId`, the reserved helper DataSetWriterIds and `broker.json`;
- the test-suite description;
- the increments and cycle cut;
- refined exit criteria.

The increments are cut by dependency: modules before resources, resources before D7 `validate`. Each step is sized to about one area group per cycle, like the P1 area cycles:

| Cycle | Content | Gap REQs |
|---|---|---|
| C-033 | plan + ADR-0019 + PoC (this cycle) | – |
| C-034 | PO decisions ADR-0019/SF-097; layout, expectation list, per-REQ coverage report; modules `opcua`/`envelope`/`metadata`/`helpers`; MSG, LIFE, BUS, PAG, LOC, QRY; the 8 bare MetaData vectors | MSG-004/017/042/043/045, PAG-009 |
| C-035 | Actions + `NewDataSetWriterId`; MAM, Health, Profile, PublicationList, ReferenceDesignation (13 topic forms) | ACT-013, RES-HEALTH-007, RES-MAM-006/014 |
| C-036 | Data/Oi4Pv, Metadata (SF-050 data types), Event, SubscriptionList (11 topic forms) | RES-DATA-003 |
| C-037 | Config Pub/Get/Set + `broker.json` | RES-CONFIG-015 |
| C-038 | D7 `validate` (`oec/conformance` skeleton, rule layer R-1 … R-4); TOP, IL, CMP rule-layer parts; umbrella REQs | IL-005, TOP-018, umbrellas |
| C-039 | P2 exit review (coverage gate, CHANGELOG 2.0.0, `$id`s frozen) | – |

- The exit is refined to seven measurable criteria (see ROADMAP P2). The main gate: every one of the 159 REQs has a reviewed positive and negative vector, and umbrella REQs count through their details if the PO agrees. The other criteria: entry schemas for all allowed matrix rows, both validators agree, the oracle covers all schema-enforced rules, D7 `validate` is green on all vectors, and `$id`s are stable.
- New vectors get V-PAY-127 ff. with status `draft`, and the PO reviews them per cycle.

### 4. ADR-0019 (`proposed`)

`project/decisions/ADR-0019-schema-architecture.md`. Summary:

1. **Dialect:** 2020-12. Every OI4-defined object is closed with an `unevaluatedProperties` closure that carries its REQ (`{"$comment": "REQ-MSG-001 …", "not": {}}`; semantically `false`). Discrimination uses `if`/`then`/`else` instead of `oneOf` for precise errors, with type guards in every predicate. REQ ids go in `$comment` (`REQ-…: …`). `format` is an annotation; the patterns are normative.
2. **Modules:** `opcua`, `envelope`, `metadata`, `actions`, `helpers`, one module per resource, `topics/<rule>.schema.json` (entry schemas), `message.schema.json` (topic-agnostic) and `platform/broker.schema.json`.
3. **Flavors:** strict is normative. Envelope definitions are closed and only refined by wrappers; DataSets are closed at their own definition. The tolerant flavor (unknown-key tolerance of REQ-MSG-046, like the oracle mode `tolerant`; not the full receiver behavior) is generated by removing the closures and rewriting the `$id` prefix (`…/oec/v2/tolerant/`). The alternative is `$dynamicRef`; it is rejected because of weaker validator support and readability.
4. **Discrimination:** `MessageType` (`ua-data`/`ua-metadata`/`ua-action-*`) → `DataSetClassId` (resource) → `DataSetWriterId` (reserved helper ids 1–3, 9) → resource-specific discriminators.
5. **Topic-aware validation:** options A (payload schema + code), **B (entry schema per topic form + small rule layer, recommended)** and C (one schema over `{topic, payload}`), with a comparison table and PoC results. The rule layer R-1 … R-5 is the part that JSON Schema cannot express.
6. **`$id`:** `<base>/oec/v2/<path>` with only the major version and relative `$ref`s; minor and patch versions go in the CHANGELOG, and tools report the bundle version. Versioning options: (i) major-only `$id`s (recommended) or (ii) immutable release `$id`s plus a `v2` alias. Base options: (a) `schemas.openindustry4.com` (recommended), (b) `oi4.github.io/oi4-oec-schema`, (c) a placeholder until publication.
7. **Location:** `oec/schema/` as the Go embed module `github.com/OI4/oec/schema`. Rust embeds by path.
8. **Test suite:** the reviewed vectors are the input; the schema must report exactly the REQs of `violates`, and an expectation list partitions rule-layer cases into `schema`/`rule` REQs; probes cover schema paths during an increment. The gate is santhosh (Go) + jsonschema-rs (Rust), which must agree. Python is optional, and Ajv is checked best-effort. `spectool coverage` gets a per-REQ report.
9. **Verification split:**

   | `verification` | REQs |
   |---|---|
   | `schema` only | 4 |
   | `schema` + `d7-validate` | 116 |
   | `d7-validate` only | 39 |

   Reclassification candidates (decided per increment): REQ-MSG-026 and REQ-PAG-009 (+`schema`), REQ-LIFE-003 (+`d7-validate`). Umbrella REQs get derived coverage through an explicit implication map; a remainder that no detail implies stays `manual`/`d7-test`.
10. **PoC results.**

### 5. Proof of concept (`oec/schema/`)

- **Schemas** (`v2/`):
  - `opcua` (integers, DateTime with bounded fields, Guid, ConfigurationVersion);
  - `envelope` (NetworkMessage `ua-data`/`ua-metadata`, DataSetMessage with fixed content masks, `MetaDataVersion` xor `MinorVersion`);
  - `helpers` (Pagination, PaginationRequest, Locale, Query with the reserved DataSetWriterIds and WriterGroupName);
  - `health` (Health incl. `Details[]` and the REQ-RES-HEALTH-002 priority chain);
  - `topics/pub-health` also enforces REQ-RES-HEALTH-007 (at least one resource DataSetMessage, or a Pagination-only page);
  - `topics/pub-health` and `topics/get-health` (entry schemas, option B);
  - `message` (option A).
- **Go module** `oec/schema`: `schema.go` with `//go:embed v2`, `BaseURI` and `TolerantBaseURI`.
- **Harness** `tools/cmd/schematest`:
  - loads `topic-patterns.json` and the payload vectors of `oec/spec`;
  - routes each vector by topic to its entry schema, or marks it out of scope;
  - generates the tolerant flavor into `dist/`;
  - validates every vector under B and A;
  - maps errors to REQ ids via the deepest `$comment` (closures carry their own REQ);
  - compares the result **exactly** with `valid`/`violates`, using the expectation list `tests/rule-layer.yaml` (empty in the PoC) for rule-layer partitions;
  - runs 17 probes (`probes.go`);
  - writes `cases.json` for the second validator.
- **Second validator:** `tools/rust-check` uses jsonschema 0.60.1 with default features off. It meta-validates every file, builds a registry and validates every case, then reports disagreements.
- **Local CI:** `check.sh` runs gofmt, vet, schematest and `cargo run --locked`. It passes.
- **Results:**

  | Measure | Result |
  |---|---|
  | vectors routed | 16 (110 out of scope) |
  | cases | 49 (32 vector validations + 17 probes) |
  | schema files | 14 (7 strict + 7 tolerant), all meta-valid |
  | invalid vectors under B | all with the exact oracle REQs |
  | invalid vectors under A | misses V-PAY-125 and V-PAY-126; only REQ-MSG-014 on V-PAY-122 |
  | probes | 17/17 (exact REQ sets) |
  | santhosh v6.0.3 vs jsonschema-rs 0.60.1 | 0 disagreements |
  | Python jsonschema 4.26.0 (session venv, not committed) | 0 disagreements |

- **Lessons** (in ADR-0019 §10): closures need their own REQ comment; the REQ set must be compared exactly; predicates need type guards; `contains`/`maxContains` need their own `allOf` item with a REQ comment; the oracle lacks REQ-RES-HEALTH-007 (an oracle gap that C-035 closes); a Locale in a Pub is accepted until SF-097 is decided.
- **Review:** an independent review of ADR-0019 and the PoC ran in this cycle; its findings were applied before the commits (§7).

### 6. Spec finding (register `decc2da`, open)

**SF-097 – MG 8.3.12, MG 8.3.11, MG 9.1.x: Locale outside `Get`.**

- MG 8.3.11 has both a sender and a receiver rule for `PaginationRequest`/`Pagination` (REQ-PAG-008: "must only be used with …, each at most once per NetworkMessage … ignored by the receiver").
- MG 8.3.12 has only the receiver rule for `Locale` ("only meaningful in combination with the method Get; in a NetworkMessage of another method it is ignored by the receiver").
- The `Pub` bullets of MG 9.1.x and the reviewed matrix list only Pagination as a helper of `Pub`. But no REQ forbids a `Locale` in a `Pub`, and the oracle accepts one.
- There is also no "at most once" rule for `Locale` in a `Get`.
- **Proposed interpretation:** like PaginationRequest, a `Locale` is used only with `Get`, at most once per NetworkMessage; the receiver rule is unchanged. Proposed sentence for MG 8.3.12 (in the register).
- Consequences: a LOC REQ (new or extended), the oracle, the entry schemas, and new negative vectors.
- The finding is needed before the C-034/C-035 entry schemas of the `Pub`/`Set`/`Del`/`Call`/`Reply` forms.

### 7. Rubber-duck review of ADR-0019 and the PoC

An independent review (rubber-duck agent) of ADR-0019 and the PoC found ten points. All were accepted and applied before the commits:

| # | Finding | Applied |
|---|---|---|
| 1 | The harness accepted an invalid vector when the schema reported only a subset of `violates`; the expectation list did not exist. | Exact REQ-set comparison. `tests/rule-layer.yaml` (partition `schema`/`rule`, rules, reason; empty in the PoC) is loaded and validated by `schematest`. ADR §8 and ROADMAP exit 3 updated. |
| 2 | The tolerant flavor was presented as the receiver view, but it keeps the sender-only placement rules (e.g. REQ-PAG-008). | ADR §3: the tolerant flavor is unknown-key tolerance only, like the oracle mode `tolerant`; ignoring misplaced helpers is receiver behavior for `d7-test`. |
| 3 | `pub-health` accepted an empty `Messages`, so REQ-RES-HEALTH-007 was not fully enforced as claimed. | New `anyOf`: at least one resource (non-helper) DataSetMessage, or a Pagination (page beyond the last page, REQ-PAG-002). Probes P-11 (empty → REQ-RES-HEALTH-007) and P-12 (Pagination only → valid). A Locale in a Pub stays accepted: REQ-RES-HEALTH-007 does not name it, and SF-097 is open (probe P-17). |
| 4 | Every undefined key was mapped to REQ-MSG-001 by hard-coded harness logic, which would be wrong for Actions (REQ-ACT-003). | Closures are now schemas with their own REQ comment (`{"$comment": "REQ-MSG-001 …", "not": {}}`); the special case in the harness is removed, and the tolerant generator drops every `unevaluatedProperties`. |
| 5 | `if`/`contains`/`not` predicates were vacuously true for non-objects/non-arrays, giving secondary REQs. | Type guards added (`type: object` in predicates, `type: array` with `contains`); ADR §1 convention. Probes P-13 (scalar DataSetMessage → only REQ-MSG-016) and P-14 (`Details` string → only REQ-RES-HEALTH-001). |
| 6 | The DateTime pattern accepted month 99 and similar values that the oracle rejects. | Bounded pattern (month, day, hour, minute, second, offset); calendar validity stays with the rule layer (R-4). Probe P-15. |
| 7 | ADR §3 said base definitions stay open and concrete wrappers close, but the envelope definitions are closed themselves. | ADR §3 now describes the real design: envelope definitions are closed and only refined by wrappers; DataSets close themselves; discriminating keys are declared unconditionally. |
| 8 | `ReplyTo` and `PaginationId` accepted any `<digits>-<text>`; REQ-MSG-015 is listed as "schema only" although its trigger part is cross-message. | Both use the MessageId structure (ServiceType/AppId). ADR §9 explains "schema without d7-validate" (the trigger part of REQ-MSG-015 is `d7-test`). Probe P-16. |
| 9 | Major-only `$id`s prevent two releases in one registry and make results less reproducible. | ADR §6: trade-off (i) major-only + reported bundle version (recommended) vs (ii) immutable release `$id`s + `v2` alias, for the PO. |
| 10 | Umbrella REQs should not count as covered just because the oracle cannot name them. | ADR §9: derived coverage with an explicit implication map; any remainder that no detail implies stays `manual`/`d7-test`. |

The REQ attribution is checked by the Go harness only; the Rust validator gates the validity verdict (ADR §8).

### 8. Project documents

- ROADMAP P2 is rewritten (§3).
- STATUS is updated: header, current phase, C-033 paragraph, next step (C-034), D1/D7 rows, baseline line (SF-097 open, next SF-098), blocking questions (ADR-0019 decision, SF-097), decided list (ADR-0019 `proposed`), cycle history.
- `oec/README.md`: `spec/` is done (P1), and `schema/` is a PoC (P2, ADR-0019 proposed).
- ADR-0003: the pointer to the P2 ADR now names ADR-0019 and the validator pair.

## Changes to reviewed items

**None.** No REQ, vector, matrix entry or topic pattern was changed. `fa96538` only remaps derived keys in `coverage/examples.yaml` (§1). The reclassification candidates of ADR-0019 §9 are proposals only, decided per increment.

## Commits

Workspace (`oi4-oec`, local, not pushed):

- `fa96538` – coverage: 29 example keys remapped to `5b8fe59` (baseline fix, §1)
- `efcf274` – `oec/schema` PoC (schemas, Go module, harness, probes, expectation list, Rust validator, `check.sh`, README)
- `b3bf639` – ADR-0019 (`proposed`) + ADR-0003 pointer
- `1ab6436` – ROADMAP P2, STATUS, `oec/README.md`
- this log

Guideline repo (`oi4-oec-documentation`, register only):

- `decc2da` – SF-097 added (open), next free id SF-098

## Checks (final)

- `spectool lint`, `render -check`, `coverage -check`, `topic -check` and `inventory -check -rev 5b8fe59` (1396 items): all green.
- `oec/schema/check.sh`: gofmt and vet clean; schematest reports 16 vectors and 17 probes, all as expected (exact REQ sets); rust-check meta-validates 14 schemas and finds 0 disagreements over 49 cases. Python jsonschema 4.26.0 (session venv): 0 disagreements over 49 cases.

## For the PO (C-034 – decisions before the schema build-out)

1. **ADR-0019 §5, topic-aware validation:** option **B** (entry schema per topic form + small D7 rule layer, recommended), A (payload schema only + code) or C (one schema over `{topic, payload}`)? A stays as the topic-agnostic fallback under B.
2. **ADR-0019 §3, tolerant flavor:** generated from strict by rewriting `unevaluatedProperties` (recommended), or switched by `$dynamicRef`?
3. **ADR-0019 §6, `$id`:** base (a) `https://schemas.openindustry4.com/oec/v2/` (recommended), (b) `https://oi4.github.io/oi4-oec-schema/oec/v2/`, or (c) a placeholder until publication (Q-011)? Versioning (i) major-only `$id`s, tools report the bundle version (recommended), or (ii) immutable release `$id`s (`…/oec/2.1.0/…`) plus a `v2` alias?
4. **ADR-0019 §8, validator gate:** Go santhosh-tekuri/jsonschema + Rust jsonschema-rs (recommended); Python optional; Ajv best-effort.
5. **ADR-0019 §9, verification split:**
   - the meaning of `schema` / `schema + d7-validate` / `d7-validate`;
   - the reclassification candidates REQ-MSG-026 and REQ-PAG-009 (+`schema`) and REQ-LIFE-003 (+`d7-validate`);
   - the **umbrella rule**: REQ-CMP-004/005/025/026, REQ-TOP-002 and REQ-IL-001/008 get derived coverage through an explicit implication map to their detail REQs; a remainder that no detail implies stays `manual`/`d7-test`.
6. **P2 cycle cut** C-034 … C-039 and the refined exit (ROADMAP P2).
7. **SF-097** (Locale only with `Get`, at most once): accept the proposed interpretation, or allow `Locale` everywhere?
8. **Baseline fix `fa96538`** (C-032 addendum 53 slip, derived keys only): for information.
