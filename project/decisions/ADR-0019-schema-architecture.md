# ADR-0019 – Schema architecture of D1 (modules, topic-aware validation, `$id`, test suite)

- Status: **accepted** (PO, C-033 review 2026-10-23; proposed in C-033). The PO accepted every recommendation, see "PO decision" below. Editorial clarifications in C-034 (§1 type guards of asserting schemas, one REQ per constraint object, reference lists; §2 Health module moved); no decision changed. Amendment C-035 (bare MetaData routing, R-3 name uniqueness, rule R-6 declared role) accepted by the PO (C-035 review 2026-10-25). Amendment C-036 (empty `Pub`, Pub-content umbrellas, Action dispatch, rule-layer additions) accepted by the PO (C-036 review 2026-10-25; SF-099 decided revised, point 1). Amendment C-037 (PublicationList and ReferenceDesignation: entry Filter checks, Mode forms, Set/Del counting, rule-layer additions) accepted by the PO (C-037 review 2026-10-25, incl. the oracle reading of the entry Filter). Amendment C-038 (Data, Event and SubscriptionList: Oi4Pv by WriterGroupName, WriterGroupName without topic Filter, Event relations in the schema, dispatch, REQ-RES-SUBLIST-002 split, umbrellas) accepted by the PO (C-038 review 2026-10-25, incl. the oracle reading of the WriterGroupName without topic Filter). Amendment C-039 (Config: def families, representation and masking, dispatch with the Set structure, rule layer; platform module `broker.json` with the `d8` tracing exception and the vector kind `broker-json`) proposed, PO review pending.
- Date: 2026-10-23
- Deciders: product owner (Thomas); proposal by the agent in C-033
- Related: ADR-0003 (JSON Schema 2020-12), ADR-0006/0013/0014 (D7/D8 share only schemas and spec), ADR-0009 (no push), ADR-0011 (Rust SDK), ADR-0012 (monorepo `oec/`), ADR-0018 (spec baseline `oec/spec/`), GL-ADR-013 (NodeIds), SF-052 (strict/tolerant), SF-097 (Locale outside Get), C-033 log

## Context

P2 delivers D1, the JSON Schemas of all OEC v2 messages, plus the offline `validate` command of D7. ADR-0003 fixed the dialect (2020-12) and deferred the architecture to this ADR.

The P1 spec baseline in `oec/spec/` (guideline `5b8fe59`) is the input:

- **159 non-deprecated REQs** carry `schema` and/or `d7-validate`: 116 carry both, 39 only `d7-validate` and 4 only `schema`.
  - 30 of them have a positive and a negative vector, 93 only a positive one, 15 only a negative one and 21 none (the gap list of C-031).
  - The P2 exit requires a positive and a negative vector for each of them.
- **126 payload vectors** (`topic` + JSON `payload`) cover all 29 topic forms of the matrix (`topic.abnf` rule = matrix row). 8 more vectors are bare `DataSetMetaDataType` objects (MG B2).
- `oec/spec/tools/internal/payload` is the **reference checker (oracle)**. `spectool lint` requires that an invalid vector's `violates` equals the set of REQs the oracle reports.

The guideline constrains the schema design in five ways:

- **Strict sender, tolerant receiver** (REQ-MSG-001/046, SF-052): undefined keys are an error for a sender but are ignored by a receiver.
- **The topic determines the payload.** Examples:
  - `Get` carries no resource DataSet;
  - `Pub` Health carries Health DataSets and an optional `Pagination`;
  - `Set` carries exactly one DataSet;
  - helper DataSets are placed per Method (PAG-008, QRY-001).

  Some rules even compare payload values with topic values: PublisherId, DataSetWriterName = Source, WriterGroupName = Filter.
- **Reserved values:** DataSetClassIds (MG A2, REQ-MSG-014) and DataSetWriterIds 0 and 1 … 9 (MSG-019/020).
- **OPC UA JSON encoding** (OPC UA 1.05.04 with the fixed content masks of MG 8.1.4), NodeIds as strings (GL-ADR-013).
- **Cross-message rules** (pagination series, Set → Status event, Call → Reply) that no single-message schema can check.

## PO decision (C-033 review, 2026-10-23)

The PO accepted the agent's recommendation on every open point:

1. §5 topic-aware validation: **option B** (entry schema per topic form + small rule layer in D7 `validate`), with `message.schema.json` (option A) as topic-agnostic fallback.
2. §3 tolerant flavor: **generated** from the strict source by removing the closures (no `$dynamicRef`).
3. §6 `$id`: base **(a) `https://schemas.openindustry4.com`** and versioning **(i) major-only `$id`s**, the tools report the bundle version (`2.MINOR.PATCH` + guideline commit). Hosting still waits for Q-011.
4. §8 validator gate: **Go santhosh-tekuri/jsonschema + Rust jsonschema-rs**; Python optional, Ajv best-effort.
5. §9 verification split as described, including the reclassification candidates (REQ-MSG-026 and REQ-PAG-009 +`schema`, REQ-LIFE-003 +`d7-validate`; applied in the increment that covers them, C-034) and the umbrella rule (derived coverage through an explicit implication map).
6. The P2 cycle cut C-034 … C-039 and the refined exit (ROADMAP P2).

SF-097 (Locale outside `Get`) was decided as proposed in the same review: a `Locale` is only used with `Get`, at most once per NetworkMessage. The entry schemas of the other methods therefore forbid `DataSetWriterId` 3 once the guideline and the catalog implement SF-097. Implemented in C-033 addendum 2 (guideline `cc2fb4b`, REQ-LOC-003); the PoC entry schema `pub-health` forbids `DataSetWriterId` 3 and `get-health` allows at most one Locale (probes P-17/P-18, 52 cases) – the PoC notes below describe the state at acceptance.

## Decision

### 1. Dialect and conventions

- Every file declares `"$schema": "https://json-schema.org/draft/2020-12/schema"` (ADR-0003). There is no `$vocabulary` and no custom keyword.
- **`format` is an annotation only.** Format assertion is optional in 2020-12 and differs between validators, so every normative constraint is a `pattern`, `enum`, `const` or numeric bound. Examples: lower-case Guid, `LocaleId`, and DateTime per OPC UA/RFC 3339 with every field bounded (month 01–12, hour 00–23, offset ±23:59, …). Calendar validity (e.g. `2026-02-30`) cannot be expressed portably; it is a rule-layer check (R-4), like in the oracle (`time.Parse`).
- **Regexes** use the portable dialect of `topic-patterns.json`: the common subset of ECMA-262 (with `u`), RE2, Python `re` and Rust `regex`; anchored, no look-around and no back-references.
- **OPC UA integers** are written as `type: integer` plus `minimum`/`maximum` (Byte, UInt16, UInt32, Int32, …).
- **Known divergences:** JSON Schema treats `1.0` as an integer, while the oracle compares numbers as written and rejects it; and calendar validity of DateTime (above). Such a vector stays an oracle/rule-layer case (see §8).
- **Type guards:** `required`, `properties` and `const` are vacuously true for values of another type, and `contains` for non-arrays. Every predicate (`if`, `contains`, `not`) that tests object keys therefore states `"type": "object"`, and every predicate with `contains` states `"type": "array"`. Otherwise a malformed value would select a wrong branch and report a secondary REQ (found in the C-033 review, probes P-13/P-14). An **asserting** schema (e.g. `contains` + `maxContains` in an `allOf`) states no type of its own: the type error is reported once by the schema that owns the type (e.g. REQ-MSG-005 for `Messages`), and a second `"type"` would add a secondary REQ (editorial clarification C-034).
- **One REQ per constraint object:** some validators stop evaluating a schema object after a failed `type`, `const` or `enum` (santhosh-tekuri/jsonschema does, found in C-034 with V-PAY-144/145). When one value can violate two REQs (e.g. `ua-deltaframe`: REQ-MSG-029 and REQ-MSG-030; `"Health": 0`: REQ-RES-HEALTH-001 and REQ-MSG-031), each constraint therefore sits in its own `allOf` item and is never a sibling of `type`/`const`/`enum` (editorial clarification C-034).
- **Reference lists:** the parentheses after the REQ ids of a `$comment` name only refs of these REQs (e.g. `REQ-MSG-029 (MG 8.2.3)`); the harness checks this and the REQ ids against the catalog (C-034).
- **REQ tracing:** every constraint that enforces a requirement carries a `"$comment"` that **starts with the REQ id(s)**, e.g. `"REQ-PAG-008 (MG 8.3.11): at most one Pagination per NetworkMessage"`.
  - The test harness maps every error to the REQs of the deepest such comment on its evaluation path.
  - Forbidden keys or branches are written as `{"$comment": "REQ-…", "not": {}}` instead of `false`, so that the comment is reachable. This includes the **closures**: `"unevaluatedProperties": {"$comment": "REQ-MSG-001 (MG 8): …", "not": {}}`, so an undefined key maps to the REQ of its closure (REQ-MSG-001; Action objects will use REQ-ACT-003). Nothing is hard-coded in the harness.
  - An error that maps to no REQ fails the test suite.
- **Discriminating chains use `if`/`then`/`else`, not `oneOf`/`anyOf`.**
  - `oneOf` reports the errors of every branch, which makes error messages and the REQ mapping ambiguous.
  - An `if` chain reports only the errors of the selected branch.

### 2. Modules and layout

```
oec/schema/
  v2/                                  source = strict flavor; $id = <base>/oec/v2/<path>
    opcua.schema.json                  OPC UA base types (Byte … UInt32, Int32, DateTime, Guid, LocaleId,
                                       LocalizedText, NodeId (GL-ADR-013), StatusCode, ConfigurationVersion, …)
    envelope.schema.json               NetworkMessage (ua-data, ua-metadata), DataSetMessage, ids
                                       (PublisherId, MessageId, ReplyTo, DataSetClassId, DataSetWriterId)
    metadata.schema.json               DataSetMetaData, FieldMetaData, StructureDescription, EnumDescription, …
    actions.schema.json                ua-action-request/-response envelopes, ActionRequest/-Response,
                                       ActionMetaData, ActionState; NewDataSetWriterId (GL-ADR-012)
    helpers.schema.json                PaginationRequest, Pagination, Locale, Query + their DataSetMessages
    resources/<resource>.schema.json   one module per OI4 resource: the DataSet (`$defs/<Resource>`) and its
                                       DataSetMessage (`$defs/<Resource>DataSetMessage`)
    topics/<rule>.schema.json          29 entry schemas, one per topic form (= ABNF rule = matrix row)
    message.schema.json                topic-agnostic schema of any OEC message
    platform/broker.schema.json        broker.json (RIG 4.1.1, REQ-CNT-003; used by D8)
  schema.go, go.mod                    Go module `github.com/OI4/oec/schema`, go:embed of v2/ (D7, D8)
  tools/                               harness (Go) + second validator (Rust); check.sh = local CI
  CHANGELOG.md                         release history 2.MINOR.PATCH with guideline baseline
```

The PoC of C-033 kept `health.schema.json` at the top level; it moved to `resources/` in C-034.

### 3. Strict and tolerant flavors

- Every OI4-defined object is **closed** with an `unevaluatedProperties` closure (`{"$comment": "REQ-…", "not": {}}`, §1); `unevaluatedProperties` is used for nothing else.
- **Where the closure sits:**
  - The **envelope** definitions (`DataNetworkMessage`, `DataSetMessage`, `ConfigurationVersion`, the `Status` object) declare every key of MG 8.1 and are closed themselves. Wrappers such as `HealthDataSetMessage` or an entry schema only **refine** these keys (`const`, `Payload` → `$ref`) and never add a key.
  - **DataSets** (the `Payload` objects of the resources and helpers, e.g. `Health`, `Pagination`) are closed at their own definition.
  - Keys used for discrimination and common keys are declared unconditionally, never only inside an `if` branch: annotations of a failing `if` do not count as evaluated, so a branch-local declaration would make the closure reject the key.
- The Payload of Data (publisher-defined names, REQ-MSG-001) and the vendor-extensible places named by the guideline stay open.
- **The strict flavor is the source and the normative sender view.** The **tolerant flavor** is **generated mechanically** by `schematest`: it is the same tree with every closure removed and the `$id`s rewritten under `<base>/oec/v2/tolerant/`. Nobody edits it.
- **Scope of the tolerant flavor:** unknown-key tolerance (REQ-MSG-046), exactly like the oracle mode `tolerant` of the vectors. It is **not** the complete receiver behavior: what a receiver ignores beyond unknown keys (a misplaced Pagination/PaginationRequest, REQ-PAG-008; a Locale outside `Get`, REQ-LOC-002) stays a sender violation in both flavors and is receiver behavior tested by `d7-test`.
- **Reason:** `unevaluatedProperties` only sees the properties evaluated in its own dynamic scope, so a single source cannot switch strictness with a parameter.
- **Alternative:** the 2020-12 way is `"unevaluatedProperties": {"$dynamicRef": "#closed"}`, where a tolerant wrapper redefines the anchor `closed` as `true`. We do not take it as the base design, for three reasons:
  - `$dynamicRef` support differs between validators;
  - code generators for the SDK (D2: typify, quicktype, …) do not understand it;
  - the error output becomes indirect.

### 4. Discrimination

The schemas discriminate in this order:

1. **`MessageType`**:
   - `ua-data` → data NetworkMessage;
   - `ua-metadata` → MetaData NetworkMessage;
   - `ua-action-request`/`-response` → Action messages;
   - any other value → REQ-MSG-002.
2. **`DataSetClassId`**: a predefined resource (MG A2) selects its resource module. A publisher-defined class id (Data, Metadata of Data) selects the generic DataSetMessage. The Oi4Pv rules are selected by the Filter/Tag, which is a rule-layer case (REQ-RES-DATA-003). _Replaced by amendment C-038 item 1: the schema selects Oi4Pv by the WriterGroupName._
3. **`DataSetWriterId`** inside `Messages[]`:
   - 1/2/3/9 → helper DataSets (PaginationRequest, Pagination, Locale, Query) with their placement rules per Method;
   - 0 and 4 … 8 → REQ-MSG-020;
   - otherwise → the resource DataSetMessage.
4. **Within a resource:** its own discriminators, e.g. the Event category via `WriterGroupName`, and the Config groups.

### 5. Topic-aware validation ("message = topic + payload")

A message is the pair (topic, payload). Three options were evaluated. Each is measured against the reviewed vectors in the PoC (§10).

| | Option A – payload schema + rule layer | **Option B – entry schema per topic form + rule layer** (recommended) | Option C – one schema over `{topic, payload}` |
|---|---|---|---|
| Idea | One topic-agnostic `message.schema.json`. Everything the topic decides (Method, Resource, Source, Filter) lives in code. | The topic is parsed with `topic-patterns.json`. The matched rule selects `topics/<rule>.schema.json`, which `$ref`s the shared modules and adds the topic form's rules (allowed DataSets and helpers, number of DataSetMessages, DataSetClassId const). Only value comparisons with the topic and multi-message rules remain in code. | A wrapper schema validates `{"topic": "...", "payload": {...}}`. The topic regexes and the payload branches sit in one schema; `if` on the topic pattern selects the payload branch. |
| Topic-dependent structure (Get vs Pub, Set single, helper placement) | in code, so every consumer (D7, SDK tests, third parties) re-implements it | **in the schema**: declarative, the same for every validator | in the schema |
| Equality with topic values (PublisherId, Source = DataSetWriterName, Filter = WriterGroupName) | code | code (small rule layer) | still code; JSON Schema cannot compare two instance values |
| Fit with the spec baseline | weak | 1:1 with the ABNF rules and the matrix rows; lint can check that every allowed row has an entry schema | the 29 regexes are duplicated into the schema |
| Error quality | good, but topic-dependent errors appear only in D7 | good: the `if` chain shows only the selected branch; the PoC maps every error to the right REQ | poor: regex `if` conditions on long topic strings; errors are hard to read |
| Usable without a topic (payload file, capture without topic) | yes | through `message.schema.json`, which is kept as fallback | no |
| Size | smallest schema, largest code | 29 small entry files (≈ 30–80 lines each) + modules | one large schema |
| PoC result (16 Health vectors) | misses V-PAY-125 (Pagination in a Get) and V-PAY-126 (Health DataSet in a Get); reports only REQ-MSG-014 for V-PAY-122 | all 16 as expected, exact REQs | not built (the structural part equals B) |

**Recommendation: option B, with `message.schema.json` (option A) as the topic-agnostic fallback.** Most "topic + payload" rules are structural per topic form, so they belong in the shared, declarative contract that D7, the SDKs and third parties use alike. Only what JSON Schema cannot express stays in code.

**Rule layer of D7 `validate`** (Go, in `oec/conformance`; every rule names its REQ ids). It covers what JSON Schema cannot express:

- **R-1 topic:** grammar (`topic-patterns.json`), Identification Link decoding and checks (IL, TOP areas), routing to the entry schema.
- **R-2 equality with topic values:**
  - PublisherId (ServiceType/AppId),
  - DataSetWriterName = Source,
  - WriterGroupName = Filter (Config context name, Data Tag, Event category/level, list TopicPath),
  - the MessageId prefix and PaginationId.
- **R-3 values across fields of one message:**
  - unique DataSetWriterIds,
  - Pagination arithmetic (REQ-PAG-004),
  - the Oi4Identifier derived from the MAM (REQ-RES-MAM-006),
  - an unencoded IL in the payload (REQ-IL-005).
- **R-4 rules the oracle checks by number semantics** (e.g. integer text `1.0`).
- **R-5 multi-message rules** (series, request → answer): these belong to `monitor`/`test` (P5), not to `validate`.

The rule layer and the oracle implement the same rules independently: `conformance/` may import the data files of `oec/spec/` but not the internals of its tools (ADR-0018 §5). The vectors keep them consistent. The Go package `github.com/OI4/oec/schema` holds data only: the embedded files and two `$id` constants.

### 6. `$id` and versioning

- `$id` = `<base>/oec/v2/<path>`, absolute `https`, with only the **major** version in the path. `$ref`s between files are relative (`../envelope.schema.json#/$defs/…`), so the bundle can be moved to another base by rewriting one prefix.
- Within `v2`, a file's `$id` stays stable. Fixes and additions are released as `2.MINOR.PATCH` in `CHANGELOG.md`, together with the guideline baseline; the version is not part of the `$id`. Breaking changes need a new major version, i.e. a new guideline major.
- **Trade-off (review finding; PO decided (i)):** with major-only `$id`s, two releases `2.x` cannot be loaded into one registry side by side, and a validation result names its contract only through the bundle version. The alternative is **immutable release `$id`s** (`<base>/oec/2.1.0/<path>`) plus a moving alias `<base>/oec/v2/` (a catalog or redirects). The relative `$ref`s make both possible with the same sources.
  - (i) **major-only `$id`s** (PoC): simple and stable for consumers; D7 and the SDK embed exactly one bundle and **report its version** (`2.MINOR.PATCH` + guideline commit) with every result.
  - (ii) **release `$id`s + `v2` alias**: exact identity and coexistence; every release changes every `$id`, and consumers must pin or follow the alias.

  The agent recommends (i) for the P2 releases: every OEC tool embeds one bundle, and coexistence is not a use case yet. (ii) can be added later without changing the sources, because only the generated `$id` prefix differs.
- Validators never fetch `$id`s: D7, the tests and the SDK load the embedded bundle (offline registry). Publishing the files at the `$id` URLs is a convenience for third parties, not a runtime dependency.
- **`<base>` (PO decided (a)):**
  - (a) **`https://schemas.openindustry4.com`** (used in the PoC): neutral and permanent, but OI4 must run the host;
  - (b) `https://oi4.github.io/oi4-oec-schema` (GitHub Pages, like the guideline at `oi4.github.io/oi4-oec-documentation`): no own infrastructure, but bound to GitHub;
  - (c) a non-resolvable base (e.g. `https://schemas.oi4.example/…`) until publication: decouples from Q-011, but means a breaking `$id` change later.

  The agent recommends (a). Hosting itself waits for Q-011 (publication), which is parked.

### 7. Location and consumption

- `oec/schema/` (ADR-0012) holds the sources, tools and tests.
- **D7/D8 (Go):** module `github.com/OI4/oec/schema` with `//go:embed v2`, consumed through a `replace ../schema` directive in the monorepo. D7/D8 share only `schema/` and `spec/` (ADR-0006).
- **D2 (Rust):** `include_str!`/`build.rs` over `oec/schema/v2`, decided with the SDK API in P3/P4.
- npm/PyPI packages, if any, come in P9.

### 8. Test suite and validators

- **Inputs:** the reviewed vectors in `oec/spec/vectors/payload/` (single source; not copied).
  - Each vector is routed by its topic and validated against its entry schema (B) and against `message.schema.json` (A), in the flavor of its `mode`.
  - Bare MetaData vectors are validated against `metadata.schema.json#/$defs/DataSetMetaData`.
- **Expectations (exact):**
  - A valid vector passes.
  - An invalid vector fails its entry schema, and the schema reports **exactly** the REQs of `violates`.
  - The exception is an invalid vector whose violation the schema cannot see completely. The expectation list `oec/schema/tests/rule-layer.yaml` partitions its `violates` into `schema` (REQs the entry schema reports exactly; empty = the schema must accept the vector) and `rule` (REQs only the rule layer detects), with the rules (R-x) and a reason. `schema` + `rule` must equal `violates`.

  The reviewed vector files are not changed. The harness enforces the partition (C-033: the list exists and is empty, since all 7 invalid Health vectors are fully schema-detected). Lint will cross-check it: a `rule` REQ must carry `d7-validate`.
- **Probes:** synthetic mutations of vectors (`tools/cmd/schematest/probes.go`). They exercise schema paths that no reviewed vector covers yet, during an increment. A probe that covers a REQ gap is promoted to a reviewed constructed vector (V-PAY-127 ff.). Probes are not catalog vectors and never count for coverage.
- **Validators** (CI gate = both must agree on the verdict of every case, including meta-validation of every file; the REQ attribution is checked by the Go harness):
  1. **Go `santhosh-tekuri/jsonschema/v6`**: also the validator inside D7.
  2. **Rust `jsonschema` crate (jsonschema-rs)**: the candidate for D2, an independent implementation.
  - **Python `jsonschema`** 4.26 serves as an optional third cross-check (0 disagreements in C-033).
  - **Ajv** is not a gate (weaker on `unevaluated*`/`$dynamicRef` and needs its strict-mode settings). Its compatibility is checked best-effort before a release.
- **CI:** `oec/schema/check.sh` runs gofmt/vet, the Go harness and the Rust runner. Under ADR-0009 (local repo, no push) it is a local gate that every cycle runs next to the `spectool` checks. A hosted workflow (Windows/macOS/Linux) follows when the repository is published.
- **Coverage:** `spectool coverage` is extended (first build-out increment) by a per-REQ report: positive/negative vector per `schema`/`d7-validate` REQ, which layer catches each negative vector, and entry schema per allowed matrix row. `-check` becomes the P2 exit gate.

### 9. Verification split: `schema` vs `d7-validate`

| `verification` | Meaning from P2 on | Count at `5b8fe59` |
|---|---|---|
| `schema` without `d7-validate` | The single-payload part is fully decided by the entry schema. Parts that need several messages stay with the other methods of the REQ (e.g. REQ-MSG-015 `schema` + `d7-test`: ReplyTo is empty or a MessageId in the schema; that it is the MessageId of the trigger is a `d7-test` rule). | 4 (REQ-LIFE-003/007/009, REQ-MSG-015) |
| `schema` + `d7-validate` | Structural part in the schema, remainder (§5 R-1 … R-4) in the rule layer. The expectation list says which layer catches each negative vector. | 116 |
| `d7-validate` only | Needs topic values, decoding, arithmetic or several fields → rule layer | 39 (TOP, IL, BUS-002/004/005, CMP-004, PAG-004/009, MSG-026, RES-CONFIG-008/010/011/015/024, RES-DATA-001, RES-MAM-006/014, ACT-020) |

- Reclassifications are not made in this ADR. Each one is made in the increment that covers the REQ, with a reason in the log and PO review (reviewed items).
- Candidates found so far:
  - REQ-MSG-026 (Status Code ≠ Good) and REQ-PAG-009 (no Pagination in Set/Del) are schema-expressible with entry schemas → add `schema`;
  - REQ-LIFE-003 ("MAM of the component itself") needs Source vs AppId → add `d7-validate`.
  - Note (C-034 review, Q-020 option (a), 2026-10-24): a Birth, Close or Will is not recognizable from one message. Payload vectors get an optional field `role` (`birth`/`close`/`will`) and D7 gets `validate --role <role>`; with a declared role REQ-LIFE-003/007/009 are checked offline and get `d7-validate` in addition to `d7-test` (C-035; the CLI option in C-039).
- **Umbrella REQs** of the gap list (REQ-CMP-004/005/025/026, REQ-TOP-002, REQ-IL-001/008): the oracle reports the detail REQ, and lint requires `violates` = oracle result, so no vector can name the umbrella in `violates`. Proposal: the coverage report shows an umbrella as **derived** coverage, with an explicit implication map (umbrella → detail REQs) in the catalog data. An umbrella counts as covered for the P2 exit when all its mapped details are covered. A remainder that no detail implies (e.g. identity or external-standard claims in REQ-IL-001/008 and REQ-TOP-002) is named in the map and stays with `manual`/`d7-test`; it is not reported as covered by vectors.

### 10. Proof of concept (C-033)

`oec/schema/v2/` holds `opcua`, `envelope`, `helpers`, `health`, `topics/pub-health`, `topics/get-health` and `message`. The PoC harness and the second validator are under `oec/schema/tools`.

Results against the 16 payload vectors on the two Health topic forms:

| Vector | Form | Expected (oracle) | B entry schema | A generic |
|---|---|---|---|---|
| V-PAY-001/002/003/004/024/025/117/118 | get/pub-health | valid | valid | valid |
| V-PAY-019 | pub-health (tolerant) | valid | valid | valid |
| V-PAY-018 | pub-health | REQ-MSG-001 | REQ-MSG-001 | REQ-MSG-001 |
| V-PAY-048 | pub-health | REQ-RES-HEALTH-001 | REQ-RES-HEALTH-001 | REQ-RES-HEALTH-001 |
| V-PAY-049 | pub-health | REQ-RES-HEALTH-002 | REQ-RES-HEALTH-002 | REQ-RES-HEALTH-002 |
| V-PAY-122 | get-health | REQ-MSG-010, REQ-MSG-014 | REQ-MSG-010, REQ-MSG-014 | REQ-MSG-014 only |
| V-PAY-124 | pub-health | REQ-PAG-003 | REQ-PAG-003 | REQ-PAG-003 |
| V-PAY-125 | get-health | REQ-PAG-008 | REQ-PAG-008 | **valid (miss)** |
| V-PAY-126 | get-health | REQ-RES-HEALTH-005 | REQ-RES-HEALTH-005 | **valid (miss)** |

- Seventeen probes all give exactly the intended REQ set:
  - two Paginations, a PaginationRequest in a Pub, two PaginationRequests (each REQ-PAG-008);
  - a Query in a Get Health (QRY-001);
  - DataSetWriterId 5 (MSG-020);
  - `ua-metadata` with `Messages` (MSG-002);
  - an unknown key in Health (MSG-001);
  - PerPage 0 (PAG-001);
  - a helper with a wrong WriterGroupName (MSG-019);
  - an empty Pub Health (RES-HEALTH-007);
  - a scalar DataSetMessage (MSG-016 only, no secondary PAG-008);
  - `Details` as a string (RES-HEALTH-001 only, no secondary RES-HEALTH-002);
  - a Timestamp with month 13 (MSG-016);
  - a ReplyTo without PublisherId (MSG-015);
  - valid: a fixed Pagination, a Pagination-only page beyond the last page, a Locale in a Pub (until SF-097).
- Go santhosh v6.0.3, Rust jsonschema 0.60.1 and Python jsonschema 4.26.0 agree on all 49 cases (32 vector validations + 17 probes) and meta-validate all 14 files (7 strict + 7 tolerant).
- Lessons for the build-out:
  - Closures need their own REQ comment (§1); a bare `false` gives no REQ.
  - The harness compares the REQ set exactly; a subset check would hide a REQ the schema misses.
  - Predicates need type guards (§1).
  - `contains`/`maxContains` belong in an `allOf` item with their own REQ comment.
  - The Health priority rule (REQ-RES-HEALTH-002) works as an ordered `if`/`contains` chain.
  - REQ-RES-HEALTH-007 is enforced by the entry schema as far as its baseline text goes: at least one resource DataSetMessage or a Pagination-only page (REQ-PAG-002), no PaginationRequest or Query, no other resource. The oracle does not check it yet; this is an oracle gap, which C-035 closes together with vectors.
  - A Locale in a Pub is accepted: REQ-RES-HEALTH-007 does not name it, and SF-097 is open.

## Consequences

- The topic-form structure becomes part of D1. Every allowed matrix row gets an entry schema, and lint will check that.
- D7 `validate` = topic grammar + IL + entry schema + a small rule layer (R-1 … R-4; R-6 since amendment C-035). The SDK can reuse the same entry schemas for its own tests.
- The oracle stays the reference for the vectors. Where it lacks a rule that a schema enforces (e.g. REQ-RES-HEALTH-007), the oracle is extended in the same increment so that lint keeps `violates` = oracle result.
- Strict is normative. The tolerant flavor is generated; consumers choose it by `$id` prefix.
- `$comment` REQ prefixes are mandatory for constraints that enforce a REQ; the harness fails on unmapped errors.
- Two validators are a permanent gate; disagreements are analysed, and a schema construct that splits validators is replaced.
- Reviewed REQs/vectors are not changed by this ADR. Reclassifications and new vectors follow per increment with reasons.

## Alternatives considered

- **Option A only** and **option C**: see §5.
- **`$dynamicRef`-switched strictness:** see §3.
- **`oneOf` discrimination:** ambiguous errors (§1).
- **One monolithic schema file:** poor review granularity, no 1:1 mapping to matrix rows.
- **Authoring in a DSL that generates JSON Schema** (CUE, TypeSpec, …): an extra toolchain, weak or uneven 2020-12 output (`unevaluatedProperties`, `if` chains), and the PO reviews the generated files anyway. Hand-written JSON with a strong test harness is simpler.
- **Ajv as a gate validator:** see §8.

## Amendment C-035 (2026-10-24) – MetaData and declared roles (agent proposals within this ADR, G-4.4; accepted by the PO in the C-035 review 2026-10-25)

1. **§8 bare MetaData vectors → `ResourceMetaData`.** A bare `object: DataSetMetaDataType` vector is the predefined MetaData of a resource (MG B2, SF-054). Lint (the oracle `CheckMetaDataType`) also checks the DataSetClassId of the predefined resources against MG A2 (REQ-MSG-012/010), which `#/$defs/DataSetMetaData` alone cannot see. The harness therefore validates such a vector against `metadata.schema.json#/$defs/ResourceMetaData` (B: `DataSetMetaData` + one `if Name = <resource> then DataSetClassId required and = <A2 id>` per resource with a predefined id) and against `#/$defs/DataSetMetaData` (A, generic). §8 originally named only `#/$defs/DataSetMetaData`.
2. **§5 R-3 also covers name uniqueness by key:** unique field names of a DataSetMetaData (REQ-MSG-038), unique StructureField names (REQ-MSG-043) and unique EnumField names (REQ-MSG-044). `uniqueItems` compares whole array items, not one key; these are rule-layer checks (expectations V-PAY-159/165/167).
3. **§5 new rule R-6 – declared role** (Q-020 (a)): with a declared role (`birth`/`close`/`will`; vector field `role`, D7 `validate --role`) the rule layer checks the single-message rules of the role: Birth = Pub MAM with exactly one DataSetMessage whose ProductInstanceUri is the AppId (REQ-LIFE-003); Close = Pub Health with exactly one DataSetMessage, Health NORMAL_0, HealthScore 0 (REQ-LIFE-007); Will = the same with FAILURE_1 (REQ-LIFE-009). The topic rules of the roles (Source = AppId, REQ-LIFE-002/006/008) and the DataSetWriterId shared with the regular messages stay with D7 `monitor`/`test` (PO decision D-3 (a), C-035 review). The role is not visible in the message, so the schemas do not see it (expectations V-PAY-171 … 175). R-5 (multi-message) is unchanged; D7 `validate` = R-1 … R-4 + R-6.
4. **§9 applied:** REQ-LIFE-003/007/009 carry `schema` + `d7-validate` (C-035; REQ-MSG-038 loses `schema`, uniqueness is R-3 only; reviewed by the PO 2026-10-25).

## Amendment C-036 (2026-10-25) – Actions and core resources (agent proposals within this ADR, G-4.4; accepted by the PO in the C-036 review 2026-10-25)

1. **§5/§8 empty `Pub` of a resource (SF-099, decided by the PO in the C-036 review, revised).** The C-033 PoC of `pub-health` required a Health DataSetMessage or a Pagination (an `anyOf` tagged REQ-RES-HEALTH-007). MG 9.1 "Resources Without Content" requires `"Messages": []` as the answer to a `Get` without content (REQ-CMP-041), so the entry schemas `pub-<resource>` accept an empty `Messages` array; the `anyOf` is removed from `pub-health` (probe P-11 now expects valid, P-35/P-36 for MAM and Profile). That an empty `Pub` only answers a `Get` needs the request and stays with `monitor` (REQ-CMP-042). The PO decided SF-099 revised: a `Get` for an asset the entity does not know is not answered with a `Pub`; the entity should publish an `Event` Status `Bad` (e.g. `BadNotFound`). The schemas are unaffected (an empty `Pub` stays valid for a known asset without content); the `Get`-to-answer relation, including "no `Pub` for an unknown asset", is runtime behaviour (`monitor`). Implemented in the guideline at `a42a132` on PO order (REQ-CMP-041 extended, new REQ-CMP-047, both `d7-test`; C-036 addendum 2).
2. **§9 umbrella rule applied to the `Pub` content REQs.** REQ-RES-MAM-022, REQ-RES-HEALTH-007, REQ-RES-PROFILE-005, REQ-RES-PUBLIST-021 and REQ-RES-REFDES-005 ("a `Pub` contains DataSetMessages of type X and optional Pagination") are not one schema constraint: the oracle reports their violations as the detail REQs REQ-MSG-010 (DataSetClassId), the content REQ of the resource (e.g. REQ-RES-MAM-001), REQ-PAG-008, REQ-LOC-003 and REQ-QRY-001 (helper placement). They become umbrellas in `coverage/implications.yaml`; the remainder (empty `Pub` only as a `Get` answer) stays `d7-monitor` (REQ-CMP-042). The same pattern for REQ-RES-CONFIG-018, REQ-RES-DATA-008 and REQ-RES-SUBLIST-009 is decided in their cycles.
3. **§4 Actions in the topic-agnostic schema.** `message.schema.json` selects the Action NetworkMessage by `MessageType` (`ua-action-request`/`-response`) and the common service by the reserved `DataSetWriterId` 8 of an item (NewDataSetWriterId, GL-ADR-012): only then are the request/response fields checked. A publisher-defined Action (any other DataSetWriterId) is checked structurally only (ActionRequest/ActionResponse; probe P-42). The entry schemas `call-`/`reply-newdatasetwriterid` require the NewDataSetWriterId item.
4. **§3 closures of the Action objects.** An unknown key in an Action NetworkMessage, ActionRequest, ActionResponse or its `Status` reports REQ-ACT-003 (the oracle was aligned in C-036); an undefined request/response field in the `Payload` of NewDataSetWriterId reports REQ-ACT-013 (encoding per ActionMetaData), like a wrong JSON type or an out-of-range value of a field. Missing fields and semantic values stay REQ-ACT-021/022. Technical note: `unevaluatedProperties` in a `$ref`'d base does not see properties that the referencing schema declares next to the `$ref`, so the base `ActionNetworkMessage` declares every key of both message types (`ResponseAddress`/`TimeoutHint` as `{}`), and the request/response schemas restrict them.
5. **§5 rule layer additions** (expectations in `tests/rule-layer.yaml`):
   - R-2: the PublisherId of a Call/Reply is the Responder of the topic (REQ-ACT-005, V-PAY-184); the `ResponseAddress` is the Reply topic of the Call topic (REQ-ACT-006, V-PAY-186); the DeviceClass `Oi4.<ServiceType>` of the MAM of the application itself names the ServiceType of the topic (REQ-RES-MAM-014, V-PAY-213).
   - R-3: the sender in the MessageId of a Call is the RequestorId (REQ-ACT-004, V-PAY-182); ProductInstanceUri = DataSetWriterName (REQ-RES-MAM-016, V-PAY-211); the generated Oi4Identifier of the MAM (REQ-RES-MAM-006, V-PAY-210) is checked **heuristically**: only a ProductInstanceUri without scheme with four segments whose first segment is the ManufacturerUri is taken for a generated Oi4Identifier; then its elements must be Model (en-US), ProductCode and SerialNumber, plain or dnp-encoded, `nd` for empty. A vendor identifier of the same shape that is not generated would be a false positive.
   - The Pagination arithmetic (REQ-PAG-004, already R-3) gets its expectation (V-PAY-050), because `pub-mam` now routes the vector.
6. **Cycle numbers.** C-036 was split (ROADMAP P2): PublicationList and ReferenceDesignation are the new C-037, so the later cycles shift by one (D7 `validate` incl. `--role` in C-040, P2 exit review C-041). The cycle numbers in §9 and in the PO decision are historical.

## Amendment C-037 (2026-10-25) – PublicationList and ReferenceDesignation (agent proposals within this ADR, G-4.4; accepted by the PO in the C-037 review 2026-10-25)

1. **§3 PublicationList entry: Filter checks in oracle and schema.** The Filter of an entry is a topic-encoded String (MG 8.3.8 with MG 7.7): every `/`-separated level is a `filter-level` of `topic.abnf` (no `+`, `#`, `/` or bare `%`, only the escapes `%2B`, `%23`, `%2F`, `%25`). A violation reports REQ-RES-PUBLIST-001. The Filter is present exactly for Data, Metadata, Config and Event; for Data/Metadata/Config it is one level (Tag or Context.Name), for Event it is `<EventCategory>/<EventLevel>` (`event-filter` of `topic.abnf`). Both report REQ-RES-PUBLIST-002. The oracle was extended accordingly (`f36c357`), so the schema and the oracle report the same REQs; a numeric `Mode` reports REQ-MSG-031 and REQ-RES-PUBLIST-001, like every verbose enumeration. The Event pattern restates `event-filter`; a change of the topic grammar must change both.
2. **§3 Mode forms (REQ-RES-PUBLIST-012).** Per Resource the schema forbids the Modes the guideline rules out, as `required: [Mode]` plus `not: {enum: …}`: Event only `OFF_0`/`FILTER_4`, Metadata only `ON_REQUEST_FILTER_9`, MAM and Health not `OFF_0`/`ON_REQUEST_1`. A missing Mode means the default `ON_REQUEST_1` and therefore violates for these resources (as in the oracle).
3. **§8 Set and Del counting.** `set-publicationlist` and `set-referencedesignation` require exactly one resource DataSetMessage by `contains` (an item with a DataSetWriterId 10 … 65535) with `minContains`/`maxContains` 1 (REQ-RES-PUBLIST-022, REQ-RES-REFDES-006). The helper items are rejected by the `items` chain (REQ-PAG-008/009, REQ-LOC-003, REQ-QRY-001), so the count does not depend on them. `del-referencedesignation` rejects every non-helper item (REQ-RES-REFDES-007, SF-098: no DataSetMessage). Set and Del entries forbid `SequenceNumber` in every item (REQ-BUS-010).
4. **§5 rule layer additions** (expectations in `tests/rule-layer.yaml`):
   - R-2: a Query of ReferenceDesignation only on the topic without Source (REQ-RES-REFDES-013, V-PAY-230). `topic.abnf` has one rule `get-referencedesignation` for both topics, so one entry schema routes both; splitting the entry schema by Source would need a second routing key next to the ABNF rule (option B of §2), which this ADR does not have.
   - R-3: unique entries by Resource, Source and Filter (REQ-RES-PUBLIST-003, V-PAY-220); `uniqueItems` compares whole items, and two entries with the same key may differ in Mode or Interval.
   - R-3: WriterGroupName = `<Resource>[/<Filter>]` and DataSetWriterName = Source of the entry (REQ-RES-PUBLIST-002). The schema checks only that the WriterGroupName starts with a ResourceType (REQ-MSG-025, GL-ADR-018); the expectation of V-PAY-151 records the split (schema REQ-MSG-025, rule REQ-RES-PUBLIST-002).
5. **§4 topic-agnostic dispatch.** `message.schema.json` selects PublicationList and ReferenceDesignation by `DataSetClassId` like MAM, Health and Profile: helper items by their reserved DataSetWriterId (the ReferenceDesignation Query with its parameters, the generic Query otherwise), every other item as the resource DataSetMessage. The method (Pub/Get/Set/Del) is not visible without the topic, so the method-dependent counts stay in the entry schemas (probes P-62/P-63).

## Amendment C-038 (2026-10-25) – Data, Event and SubscriptionList (agent proposals within this ADR, G-4.4; accepted by the PO in the C-038 review 2026-10-25)

1. **§4 Oi4Pv selected by the WriterGroupName, not by the rule layer.** §4 step 2 says the Oi4Pv rules are selected by the Filter/Tag as a rule-layer case (REQ-RES-DATA-003). The WriterGroupName of a Data DataSetMessage is its Tag (REQ-MSG-024), so the selection is visible in the DataSetMessage itself: `resources/data.schema.json#/$defs/DataDataSetMessage` applies `#/$defs/Oi4Pv` (required `Pv`, key names `Pv`/`Sv1` … `Sv255` by `propertyNames`, REQ-RES-DATA-004) when `WriterGroupName` is `Oi4Pv`. That WriterGroupName and topic Filter are equal stays R-2 (REQ-MSG-024). An Oi4Pv DataSet published under another Tag is not recognizable from one message (REQ-RES-DATA-002, manual).
2. **§3 oracle: WriterGroupName on a Data/Config/Event topic without Filter.** On `Pub/Data/<Source>` (and the Config/Event forms without Filter) there is no topic Filter to compare with. The oracle (`payload.go`, `d6d5010`) now requires there a WriterGroupName that is a Filter of the resource (one topic-encoded level for Data and Config, `event-filter` for Event); otherwise REQ-MSG-024. The schema patterns of `DataDataSetMessage` and `EventDataSetMessage` restate the same rule, so schema and oracle agree (probe P-67).
3. **§4 Data without dispatch by DataSetClassId.** Data has no predefined DataSetClassId (MG A2), so the entry schemas `pub-`/`get-`/`set-data` neither fix nor require one (REQ-MSG-011 does not apply), and `message.schema.json` checks a Data DataSetMessage with the generic DataSetMessage (no Oi4Pv check without the topic). The content of an application-defined DataSet is any JSON object; that its fields match the Metadata of the same Source and Tag needs that Metadata (REQ-RES-DATA-001, d7-test).
4. **§4 step 4 Event relations within the DataSetMessage.** `resources/event.schema.json#/$defs/EventDataSetMessage` checks the relations between the WriterGroupName (`<EventCategory>/<EventLevel>`) and the Payload in the schema, not in the rule layer:
   - REQ-RES-EVENT-002: the Category corresponds to the EventCategory prefix (`Syslog`, `Status`, `Generic`);
   - REQ-RES-EVENT-004: the Number of a Status Event (StatusCode, UInt32) selects the EventLevel by its severity bits (three ranges; class 3 counts as Bad) and the SymbolicId starts with the class name;
   - REQ-RES-EVENT-008: the PRI of a Syslog Event (Number ≤ 191) selects the EventLevel by PRI mod 8 (eight `if`/`then` pairs).
   The Payload rules by Category stay in `#/$defs/Event` (REQ-RES-EVENT-003 Status Details = SymbolicId only, REQ-RES-EVENT-007 Syslog Number ≤ 191, no Description, Details only MSG/HEADER). The oracle conditions are restated: EVENT-004/008 apply only if Category, WriterGroupName category and Number agree, so one defect reports one REQ. Equality of WriterGroupName and topic Filter stays R-2.
5. **§4 topic-agnostic dispatch.** `message.schema.json` selects Event and SubscriptionList by their DataSetClassId, like the other predefined resources (probes P-81/P-82). Event DataSetMessages carry `MessageType` `ua-event`, Data and SubscriptionList `ua-keyframe` (REQ-MSG-029).
6. **§3/§5 REQ-RES-SUBLIST-002 split.** The schema checks that the resource level of a TopicPath in the OEC namespace (`Oi4v2`, or a leading `+`/`#`) is concrete, i.e. neither `+` nor covered by `#`; topic filters outside the OEC namespace are not checked (probe P-77). The derivation of the WriterGroupName from the TopicPath of the same entry stays R-3 (expectation of V-PAY-086: schema none, rule REQ-RES-SUBLIST-002). The schema checks only that the WriterGroupName starts with a ResourceType (REQ-MSG-025, as for PublicationList in C-037).
7. **§8 entry schemas.** `get-data` and `get-subscriptionlist` allow only PaginationRequest and Locale; a Query reports REQ-QRY-001, a resource DataSetMessage REQ-RES-DATA-006/REQ-RES-SUBLIST-007. `set-data`, `set-subscriptionlist` and `del-subscriptionlist` require exactly one resource DataSetMessage by `contains` with `minContains`/`maxContains` 1 (REQ-RES-DATA-009, REQ-RES-SUBLIST-010/011) and forbid `SequenceNumber` (REQ-BUS-010), as in C-037. `pub-data`, `pub-event` and `pub-subscriptionlist` accept an empty `Messages` (SF-099) and a Pagination.
8. **§9 umbrellas.** REQ-RES-DATA-001/003/008, REQ-RES-EVENT-009/011 and REQ-RES-SUBLIST-009 are umbrellas in `coverage/implications.yaml` (oracle detail REQs as in C-036 item 2); the remainders are `d7-test` (Metadata of the DataSet, REQ-RES-DATA-001), manual (Oi4Pv under another Tag, Generic Event content) or `d7-monitor` (empty `Pub` only as a `Get` answer, REQ-CMP-042).

## Amendment C-039 (2026-10-25) – Config and broker.json (agent proposals within this ADR, G-4.4; proposed, PO review pending)

1. **§2 two def families for Config.** `resources/config.schema.json` has a Pub family (`Config`, `Context`, `Group`, `ConfigObject`, `Validation`, `ConfigDataSetMessage`; REQ-RES-CONFIG-002) and a Set family (`ConfigSet…`; REQ-RES-CONFIG-003). A Pub carries the complete object (Name, Type, Value, Mandatory, …); a Set is a partial update in which only `Value` is mandatory (MG 8.3.3.2). Both families share the GroupName pattern (REQ-RES-CONFIG-004), the Type enumeration (REQ-RES-CONFIG-006) and the representation per Type, and are closed for REQ-MSG-001.
2. **§3 Value representation and masking in the schema.** REQ-RES-CONFIG-007: `Value`/`DefaultValue` is selected by `Type` (`if`/`then`) – Boolean `true`/`false`, Number in JSON number syntax, DateTime RFC 3339 syntax, ByteString Base64 (no CR/LF), String any; the empty string (unset, REQ-RES-CONFIG-010) is valid for every Type, and the mask `********` is valid only with `Sensitive` = true (GL-ADR-021). REQ-RES-CONFIG-024 (verification +schema, R-2 of C-039): in a Pub a Sensitive object carries `Value`/`DefaultValue` only as `""` or the mask; a Set may carry the clear value (probe P-105). The calendar validity of a DateTime (e.g. 30 February) is rule layer R-4, like the other DateTime fields. Oracle tweak `10a7d8d`: Go's Base64 decoder skips CR/LF, so the oracle now rejects them like the schema does.
3. **§4 topic-agnostic dispatch with the Set structure.** Pub and Set Config are indistinguishable without the topic, so `message.schema.json` checks a Config DataSetMessage (selected by its DataSetClassId) against the Set family, the superset (option A accepts every valid Pub and Set; probes P-107/P-108). The Pub-only rules (all mandatory fields, masking) stay in `pub-config`.
4. **§5/§8 rule layer of Config.** REQ-RES-CONFIG-008 (a Value satisfies Length, Min, Max, Pattern and Values of its own Validation, V-PAY-108) and REQ-RES-CONFIG-015 (unique Filter per Source in a Pub, V-PAY-241) are R-3: JSON Schema can neither use a sibling value as a constraint nor express uniqueness across array items by a pair of keys. WriterGroupName = topic Filter and DataSetWriterName = topic Source stay R-2. The C-039 oracle reports REQ-RES-CONFIG-015 (`4e52e5c`). REQ-RES-CONFIG-011 (Mandatory objects) needs the published Config and moves to `d7-monitor` (R-1 of C-039).
5. **§8 entry schemas of Config.** `get-config` allows only PaginationRequest and Locale (REQ-RES-CONFIG-016; a Query reports REQ-QRY-001, P-102); `set-config` requires exactly one Config DataSetMessage (REQ-RES-CONFIG-019) without helper DataSets and without SequenceNumber (REQ-BUS-010, P-103); `pub-config` accepts an empty `Messages` (SF-099) and a Pagination. With Config every allowed matrix row has its entry schema (29 of 29).
6. **§9 umbrellas.** REQ-RES-CONFIG-010 (unset by empty string) and REQ-RES-CONFIG-018 (Pub content) are umbrellas in `coverage/implications.yaml`, as in C-036 item 2.
7. **§7 platform module `broker.json`.** `v2/platform/broker.schema.json` describes the platform file `broker.json` of RIG 4.1.1 (REQ-CNT-003) for the infrastructure compliance CLI (D8). It is part of the schema bundle (same `$id` base, both flavors; equal, because the file has no closure). Its constraints implement the proposed interpretation of SF-100: further elements are allowed; `Address` is an IPv4 address, an IPv6 address without brackets/zone or an RFC 1123 host name whose last label is not all digits; `SecurePort` 1 … 65535; `MaxPacketSize` 262144 … 268435455. The patterns are identical to those of the oracle `oec/spec/tools/internal/platform` (compared in C-039; oracle and schema agree on all 38 broker cases of `dist/cases.json`).
8. **§1 tracing exception for platform files.** A REQ in a `$comment` of a `platform/` file may carry the verification `d8` instead of `schema` (`schematest` `checkComments`). REQ-CNT-003 keeps `[d8]`: D8 uses the schema as its implementation, and the vector coverage report of `schema`/`d7-validate` REQs (§8) does not count it.
9. **§8 vector kind `broker-json`.** The catalog gets the vector kind `broker-json` (`V-BRK-xxx`, field `content` = file content, `oec/spec/vectors/platform/`). `spectool lint` checks every vector with the oracle; `schematest` validates the content against `platform/broker.schema.json` (valid passes, invalid fails with exactly its `violates`), writes the cases to `dist/cases.json` for the second validator and uses the vectors as bases of probes P-109 … P-128.
