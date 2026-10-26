# C-047 – P4 increment 3: message model, schema embedding, payload vectors

- **Date:** 2026-10-26
- **Phase:** P4 – SDK core implementation (D2), second cycle (cycle cut: ROADMAP P4)
- **Baseline:** guideline `734c560` (register `fce5cf5`, branch head, only `findings/`), unchanged; schema `2.0.1-dev` @ `734c560`
- **Start:** workspace after C-046 addendum 3 (`4808d25`)

## Intent

STATUS "Next step" 1 and ROADMAP P4 asked for increment 3:
1. a typed message model in `oec-core`:
   - NetworkMessage/DataSetMessage (`ua-data`), DataSetMetaData (`ua-metadata`) and the Action messages;
   - all resource payloads and the helper DataSets;
   - fixed content masks;
   - MessageId/`ReplyTo` and Action correlation;
   - every decision traced to REQ ids;
2. schema embedding via `build.rs`, validation with `jsonschema =0.60.1` behind `strict-validation`, reporting `schema.Version`/`schema.Baseline` of the embedded tree (`oec/schema/schema.go`);
3. all valid and invalid payload vectors as tests, with the same verdicts as the schema gate in the `strict-validation` build;
4. the first runtime dependencies under `cargo deny`;
5. a re-check of the P4 cycle cut.

Baseline check at the start:
- guideline head `fce5cf5` (only `findings/`) over text `734c560`;
- SF-001 … 104 resolved, none open;
- 1399 inventory items, 440 REQs, 158 `schema`/`d7-validate` REQs with 0 gaps, no drafts;
- entry schemas 29 of 29, 245 payload vectors;
- `oec/check.sh` green.

The guideline did not change in this cycle, so no re-baseline was needed.

## Cycle cut (re-checked, agent decision G-3.4 b)

**No split.** All five parts are delivered and green.

The cycle is large: about 3,500 lines of product code and 650 lines of tests in `oec-core`. Still, it is one review unit:
- the model, the schema hook and the vector tests check each other;
- the parity test is the acceptance test of the model;
- a split such as "envelope now, resources later" would have left half the payload vectors without a decoder;
- the review load for the PO is small, because no reviewed item changed, there is no new vector and no finding (only A-x for information).

The plan for the following cycles stays as in ROADMAP P4: C-048 increment 4 (crate `oec`), then 5, 9, P5 interleave, 6–8, 10.

## Work

### Dependencies (`a5b16aa`)

| Crate | Version | Features | Role |
|---|---|---|---|
| serde | 1 (1.0.229) | `derive` | typed messages |
| serde_json | 1 (1.0.154) | default | JSON, `Value`, `Number` |
| time | 0.3 (0.3.55) | `std`, `formatting`, `parsing` | OPC UA DateTime (RFC 3339) |
| uuid | 1 (1.29.0) | `std`, `serde` | DataSetClassId, Guid |
| jsonschema | `=0.60.1` | none (`default-features = false`), optional | feature `strict-validation` |

- Tree: 20 crates in the default build, 81 with `strict-validation`. The extra crates come from jsonschema: regex, URI, number and email parsers, `ahash`, `once_cell`, …
- `cargo deny check`: advisories, bans, licenses and sources ok. **No exception and no `ignore` entry** was needed; all licenses are on the ADR-0023 §2 allowlist (G-4.4).
- uuid `v4`/`v5` are not enabled yet. They come with the increment that generates ids (default DataSetClassId of `oec`, increment 6).
- ADR-0023 amendment C-047 (agent, G-3.4 b) records the feature sets.

### Schema embedding (`c7fcfdc`)

`oec-core/build.rs` (dependency-free):
- **Version source:** reads `const Version` and `const Baseline` from `oec/schema/schema.go` and writes `schema::VERSION` (`2.0.1-dev`) and `schema::BASELINE` (`734c560`), plus the strict and tolerant base URIs. This is the tree that is embedded. The C-043 spike read the newest release manifest (`2.0.0`), which is wrong during a pre-release.
- **Embedding:** with `strict-validation` it writes one `include_str!` per strict source `oec/schema/v2/**/*.schema.json` (45 files), so the binary needs no schema file at run time.
- **Rebuilds:** `rerun-if-changed` for `schema.go`, the tree and every file.

`schema::Validator` (feature `strict-validation`):
- **Registry:** built in memory (no file or HTTP resolution). The tolerant flavor is derived at `Validator::new()` by dropping the closures, exactly as the gate does (REQ-MSG-046). Only the strict sources are embedded.
- **Routing:** `entry_for(&Topic)` gives the entry schema of the topic form, otherwise the generic message entry. This is the routing of the schema gate (ADR-0019 §8).
- **Attribution:** `SchemaViolation` lists per error the REQ ids of the deepest `$comment` on its evaluation path, as the gate attributes them.

### Message model `oec_core::message` (`c7fcfdc`)

| Item | Content | REQs (main) |
|---|---|---|
| `PublisherId`, `MessageId` | `<ServiceType>/<AppId>`; `<unixTimestampInMs>[<counter>]-<PublisherId>`, parsed and built | REQ-MSG-005 … 009 |
| `DataNetworkMessage<P>` | `ua-data` with the NetworkMessage fields and fixed mask, `DataSetClassId` from the resource (Data/Metadata: publisher-defined, reserved and predefined ids refused), `ReplyTo` (`""` or the trigger's MessageId) | REQ-MSG-002 … 004, 010 … 015 |
| `DataSetMessage<P>`, `DataSetEntry<P>` | fixed DataSetMessage mask, `DataSetWriterId` ≥ 10 for applications, MessageType per resource, `MetaDataVersion` vs. Major/MinorVersion, `Status` only if not Good, Timestamp, SequenceNumber | REQ-MSG-016 … 030 |
| helper DataSets | `PaginationRequest` (1), `Pagination` (2), `Locale` (3), `Query` (9) with their reserved DataSetWriterIds and WriterGroupNames | REQ-MSG-019, REQ-PAG-001/003, REQ-LOC-001, REQ-QRY-001 |
| `MetaDataNetworkMessage`, `DataSetMetaData` | `ua-metadata`, fields, structure/enum/simple type descriptions (Int64 as string) | REQ-MSG-031 … 045, REQ-RES-META-001/003 |
| `ActionRequestMessage<P>`, `ActionResponseMessage<P>` | `ua-action-request`/`-response`; PublisherId = Responder from the Call topic, RequestorId, `ResponseAddress` = Reply topic, `TimeoutHint` > 0, `CorrelationData` echoed unchanged; `response()` copies the correlation fields | REQ-ACT-001 … 014 |
| `ActionRequest<P>`, `ActionResponse<P>` | RequestId, ActionTargetId, ActionState; `done()`/`failed()` build the matching response, a Bad Status has an empty Payload; common service `NewDataSetWriterId` (DataSetWriterId 8, ActionTargetId 1) | REQ-ACT-010 … 014, 019, 021, 022 |
| `Message` | one enum over the 13 kinds; `decode(&Topic, &[u8])` picks the type from the topic form (`Set/Config` → `ConfigSet`, `Call/NewDataSetWriterId` → request, …); `encode()` writes only guideline keys | REQ-MSG-001, 046 |
| value types | `DateTime` (text kept, real calendar date), `LocalizedText`, `ConfigurationVersion`, `ByteString` (text kept) | REQ-MSG-016, REQ-ACT-007 |

### Resource payloads `oec_core::resource` (`c7fcfdc`)

| Type | Content | REQs (main) |
|---|---|---|
| `Mam` | MAM fields | REQ-RES-MAM-* |
| `Health`, `HealthState`, `DetailObject` | `Health::aggregate` takes the worst state by the MG 8.3.2 order | REQ-RES-HEALTH-002 |
| `Profile` | `Resources` | REQ-RES-PROFILE-* |
| `PublicationEntry`, `PublicationMode`, `PublicationConfig` | 16 modes, intervals, precisions | REQ-RES-PUBLIST-* |
| `SubscriptionEntry`, `SubscriptionConfig` | `TopicPath` as string | REQ-RES-SUBLIST-002 |
| `ReferenceDesignation`, `ReferenceDesignationQuery` | `Query` view (Parent, Aspect) | REQ-RES-REFDES-012 |
| `Config`, `ConfigObject`, `Validation`, `ConfigSet` | groups as flattened maps, `Type`, masking of sensitive values | REQ-RES-CONFIG-* (22 citations) |
| `Data`, `Oi4Pv` | raw object with a typed `Pv`/`Sv<n>` view | REQ-RES-DATA-004 |
| `Event`, `EventCategory` | Number, Category, Description, Details; MessageType `ua-event` | REQ-RES-EVENT-* |

**REQ tracing:** the new modules cite 96 distinct REQ ids, among them:
- REQ-MSG-001 … 046 except 022 (consumers do not store DataSetWriterIds: consumer side, increment 8); REQ-MSG-001 (no user-defined keys) and REQ-MSG-003 (NetworkMessage fields) were added to the docs during the log review;
- REQ-ACT-001 … 014, 019, 021, 022;
- 33 resource, pagination, locale and query REQs.

The REQ walker of the sketch (`oec/sdk/check.sh`) checks that every one exists and is not withdrawn: green.

### Tests (`c7fcfdc`)

| Test | Build | What it checks |
|---|---|---|
| `tests/payload.rs` `valid_vectors_round_trip` | both | all 102 valid payload vectors decode and re-encode to an equal JSON value; the two tolerant vectors V-PAY-019/113 drop their unknown keys and re-encode stably |
| `tests/payload.rs` `invalid_vectors_do_not_panic` | both | all 143 invalid vectors: 63 refused by the decoder, 80 left to the schema or rule layer; no panic |
| `tests/payload.rs` `verdicts_equal_the_schema_gate` | strict | all 245 vectors, with the 25 expectations of `oec/schema/tests/rule-layer.yaml`: 102 valid and 24 rule-only invalid vectors pass the schema (D7 rules catch the 24); the other 119 invalid vectors are rejected, each with exactly the REQ ids of the gate (for one of them only the schema part of its REQs, the rest is rule layer) |
| `tests/payload.rs` `encodings_pass_the_strict_schema` | strict | every SDK re-encoding of a valid vector passes the strict entry schema of its topic |
| `tests/model.rs` (5 tests) | both, schema check in strict | sender side:<br>• Get MAM with Pagination + Locale helpers, the Pub MAM reply with `ReplyTo` and Pagination;<br>• Health (aggregate), Event on `Status/Bad`;<br>• Data with a publisher DataSetClassId (reserved/predefined refused), ReferenceDesignation Query;<br>• NDWI Call/Reply correlation (`response`, `correlates`, `answers`, CorrelationData, `failed` → `Payload: {}`, Bad status refused in `with_status`, non-Call topic refused);<br>• decoder invariants with their REQ ids;<br>• tolerant reader |
| unit tests | both | 27 (default) / 30 (strict): base64url, IL, topic, Health aggregate, Oi4Pv, validator setup and routing |

Results:
- Default build: 27 + 5 + 2 + 6 tests. Strict build: 30 + 5 + 4 + 6 tests. All pass.
- The parity with the gate held at the first run.
- No vector was changed. No disagreement between SDK, schema and vectors was found, so there is no finding.

### Gate (`cb92fa5`)

- `oec/sdk/check.sh` runs clippy (pedantic, `-D warnings`), tests and `cargo doc -D warnings` twice: with default features and with `--features strict-validation`. Then `cargo deny check` runs, as before.
- `oec/sdk/rust/README.md`:
  - lists the new modules, the runtime dependencies and the two-pass gate;
  - fixes the increment of `oec-ffi` (10, not 8).

## Agent decisions (G-3.4 b, for information: REVIEW A-10 … A-15)

Recorded in ADR-0026 amendment C-047 §7 and ADR-0023 amendment C-047 (`8d43168`).

1. **A-10 Tolerant typed decoder.**
   - No `deny_unknown_fields` (REQ-MSG-046).
   - The decoder refuses only what the type cannot hold: JSON types, required fields, closed sets, model invariants.
   - Patterns and cross-field rules stay with the schema/rule layer. Reason: one source of truth for patterns (the schema), and a receiver must not fail on content it can still read.
2. **A-11 Encoder with fixed masks.**
   - The DataSetMessage `MessageType` is always sent.
   - `ReplyTo` is `""` without a trigger.
   - The DataSetClassId comes from the resource.
3. **A-12 Lossless values and representation.**
   - `DateTime`/`ByteString` keep their text (CorrelationData byte-identical, REQ-ACT-007).
   - `serde_json::Number` is used where the schema allows int or float.
   - Guid only in the lowercase hyphenated form.
   - Sorted maps (no `preserve_order`): key order carries no meaning in JSON.
   - `DataSetEntry` decodes via `Value` (one extra conversion, simpler dispatch over helper and payload DataSets).
4. **A-13 Action API.**
   - `ActionRequestMessage::new` derives the Responder and `ResponseAddress` from the Call topic.
   - `response()`/`done()`/`failed()` build the correlated answer.
   - `ActionResponse::payload` is `Option` (`None` ↔ `{}` for Bad).
   - `with_status` refuses Bad codes (use `failed`).
5. **A-14 Config/Data/SubscriptionList shapes.**
   - Config groups are flattened maps.
   - Data is a raw object with an `Oi4Pv` view.
   - SubscriptionList `TopicPath` is a string.
6. **A-15 Schema embedding and dependency features.**
   - Version/Baseline come from `schema.go` (PO order in the cycle request, ROADMAP P4 increment 3).
   - The tolerant flavor is derived in memory.
   - Minimal feature sets, no deny exceptions.
   - The gate runs both feature sets.

Cycle cut (no split, above) is A-16.

## Changes to reviewed items

None:
- no REQ, vector, matrix entry or PO-accepted decision content changed;
- ADR-0023 (PO-accepted) got an amendment that records the feature sets and applies the version source the PO ordered for this cycle (ROADMAP P4 increment 3); the decision table itself is unchanged;
- ADR-0026 is agent-accepted and got §7;
- schema unchanged (`2.0.1-dev`), no CHANGELOG entry.

## Spec findings

None. Every vector verdict agreed with the schema gate and the rule layer, and building the model did not reveal a contradiction or gap in MG/RIG. Next free id stays SF-105.

## Commits

Workspace (never pushed):
- `a5b16aa` dependencies (workspace + `oec-core` manifests, `Cargo.lock`);
- `c7fcfdc` `oec-core` increment 3 (`build.rs`, `schema`, `message`, `resource`, `lib.rs`, tests);
- `cb92fa5` gate both feature sets, README;
- `8d43168` ADR-0026 amendment C-047, ADR-0023 amendment C-047;
- `ba748af` REQ-MSG-001/003 citations in the message docs (found while writing this log);
- this log, STATUS, ROADMAP; then REVIEW.md/REVIEW_DE.md.

Guideline: none.

## Checks (final)

`oec/check.sh`: all checks passed. Included in it:
- spectool `lint`, `render -check` (incl. the export), `coverage -check -gate` (0 gaps), `topic -check`, `inventory -check -rev 734c560` (1399 items);
- `schema/check.sh`;
- release freeze;
- `spectool crosscheck`;
- `sdk/check.sh`:
  - the workspace in both feature sets: fmt, clippy, tests, doc;
  - cargo-deny;
  - the sketches (REQ walker) and the spikes;
- the Go tests of `oec/spec/tools` and `oec/conformance`.

## For the PO (review)

Nothing to decide and no test data to review: no D-x or R-x. For information only (`REVIEW.md` A-10 … A-16): the agent decisions above.

## Next

C-048 – P4 increment 4 (ROADMAP P4): crate `oec` with `App`/`AppBuilder`, `Level`, `Bootstrap` (explicit + container storage contract), `Store`, `Error`; the sketch API moves into `oec` piece by piece; new runtime dependencies thiserror and log.

Next free ids (unchanged): SF-105, REQ-CMP-048, REQ-RES-PUBLIST-028, REQ-TOP-029, V-PAY-246, V-TOP-138 (valid) / V-TOP-255 (invalid), V-BRK-019, P-129, ADR-0027, Q-031.

Agent for C-048 (G-3.10, decided in addendum 1): **new agent**, start prompt in `prompt.md`.

## Addendum 1 (2026-10-26) – PO instruction: agent hand-over rule

**PO instruction (German, paraphrased):** at the end of every cycle, decide whether a new agent starts or the same one continues. If a new one starts, also fill `prompt.md` with a start prompt.

**Done:**
- GUARDRAILS **G-3.10** (new): the decision at every cycle end and addendum, recorded in the log and in the German summary; default criteria for new vs. same agent; structure of the start prompt; `prompt.md` is never committed and stays unchanged when the same agent continues.
- G-3.6: `prompt.md` added to the German exceptions.
- `project/README.md`: cycle step 6 "Hand over", "done" only after step 6, and the log template's "Next" section names the agent decision.

**Decision for C-048: new agent.**
- The context of this session was compacted several times during C-047.
- C-048 starts a new increment in a new crate (`oec`) and works mostly in other files: ADR-0020 §2/§3/§5/§10/§14, the sketch, the storage contract (RIG) and the `broker.json` vectors.
- The state is handover-ready (G-3.7): this log, STATUS "Next step" 1, ROADMAP P4 and `REVIEW.md`.

**Start prompt:** `prompt.md` now holds the C-048 prompt. It is not committed.

**Checks:** only `project/` docs changed, so no gate run was needed (the last run after `500cdf0` was green).
