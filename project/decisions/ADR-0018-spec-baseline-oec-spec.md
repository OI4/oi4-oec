# ADR-0018 – Requirements catalog as YAML in `oec/spec/` (shared spec contract)

- Status: accepted
- Date: 2026-10-14
- Deciders: product owner (Thomas), C-013
- Related: Q-018, ADR-0001, ADR-0003, ADR-0006, ADR-0012, ADR-0013, ADR-0014, G-1.6, G-3.3, G-4.12, C-012 log §1

## Context

Phase P1 turns MG + RIG into a requirements catalog (`REQ-<AREA>-NNN`), a formal topic grammar, a Method × Resource matrix and test vectors (C-012 plan). Schema tests (D1), the Rust SDK (D2), the micro SDK (D9), D7 and D8 all have to reference REQ ids by machine. So far ADR-0006 allowed D7/D8 to share **only** the D1 JSON Schemas with the SDK, and ADR-0012 did not list a place for the spec baseline.

Options (Q-018): YAML files in a new `oec/spec/` folder, shared like D1 (proposal); JSON in `oec/spec/`; Markdown tables in `project/`.

## Decision

1. **Location:** new monorepo folder **`oec/spec/`** – the machine-readable spec baseline. Layout (details may be refined in the first execution cycle):

   ```
   oec/spec/
     README.md                    what is here, how to cite REQ ids
     requirements/
       catalog.schema.json        JSON Schema 2020-12 for the catalog files
       areas.yaml                 area registry (code, title, guideline sections)
       <area>.yaml                one file per area, e.g. top.yaml, res-health.yaml
     topic/
       topic.abnf                 formal topic grammar (RFC 5234 + RFC 7405)
       topic-patterns.json        derived regexes (generic + per Method × Resource)
     matrix/
       method-resource.yaml       Method × Resource matrix
     vectors/
       topics/{valid,invalid}/    topic test vectors
       golden/                    curated Appendix B examples (topic + payload)
     tools/                       catalog lint, Markdown rendering, coverage, extraction (Go)
   ```

2. **Format:** **YAML**, restricted to a JSON-compatible subset (no anchors/aliases, no custom tags, no merge keys; ambiguous scalars quoted), so that every YAML 1.2 parser in Go, Rust, C test harnesses, Python or JS reads it identically. Every file is validated against `catalog.schema.json` (JSON Schema 2020-12, ADR-0003) in CI. Generated Markdown views per area are the review artifact for the PO; the YAML is the source.

3. **Entry fields** (draft, finalized with `catalog.schema.json`): `id`, `title`, `statement`, `keyword` (MUST / MUST NOT / SHOULD / SHOULD NOT / MAY per MG 1.2, G-1.6), `implicit` (true if the guideline states the behavior without keyword – SF-035), `domain` (`messaging` / `service-container` / `platform`), `levels` (obligation per L1/L2/L3), `applies_to`, `condition`, `refs` (MG/RIG/GL-ADR/SF), `verification` (`schema` / `d7` with mode / `d8` / `manual`), `status` (`draft` → `reviewed` → `deprecated`), `notes`. Every file records the guideline revision (`baseline: <commit>`). REQ ids are never reused or renumbered. The C-012 field `maturity` is dropped: the guideline no longer has immature chapters (SF-033).

4. **Shared contract:** `oec/spec/` is shared by `schema/` (D1), `sdk/rust/` (D2), `sdk/micro/` (D9), `conformance/` (D7, D8) and `docs/` (D6). Its content is derived **only from the guideline** (ADR-0001) and PO decisions on spec findings – never from SDK code or SDK output. This extends the independence rule of ADR-0006: D7/D8 share **D1 schemas and `oec/spec/`** with the SDKs, still **no code**.

5. **Tools** in `oec/spec/tools/` are written in Go (same toolchain as D7/D8, ADR-0013), cross-platform, and must not import anything from `sdk/` or `conformance/`. `conformance/` may import the data files, not the tools' internals.

## Consequences

- ADR-0006 and ADR-0012 are amended (C-013); GUARDRAILS G-4.12, `oec/README.md` and `project/README.md` name `oec/spec/` as the second shared artifact.
- `oec/spec/` is created in the first P1 execution cycle (C-014), together with `catalog.schema.json` and the lint tool.
- The YAML parser for Go and Rust is chosen in C-014 (maintained libraries only; `gopkg.in/yaml.v3` and `serde_yaml` are no longer maintained and need a check against current alternatives).
- A guideline change can invalidate REQ entries: the extraction/coverage tools record the baseline per file, so a re-baseline shows which entries to review.

## Alternatives considered

- **JSON in `oec/spec/`:** no YAML dependency, but no comments and poorer to author and review for long normative text.
- **Markdown tables in `project/`:** easy to read, but not reliably machine-readable and outside the monorepo, so D7/D8/SDK tests could not reference it as a contract.

## Amendment C-013 – golden vectors and PO review (PO decision on the C-012 §5 rules)

The PO accepted the curation rules of the C-012 plan (§5) with an extension: the examples must be **checked against the guideline description** and built so that they are **compliant, meaningful and good test data**. The PO reviews the test data of every increment and confirms it or notes deviations. Binding rules for `oec/spec/vectors/golden/`:

1. **Extraction:** a tool in `oec/spec/tools/` reads Appendix B at the baseline revision and records for every example its source (`doc`, section, heading, index within the section, guideline commit) and the verbatim original. Re-running it after a guideline change shows which examples changed.
2. **Check against the description:** every example is compared with the normative text it illustrates (topic structure, MessageType, `DataSetClassId`, `DataSetWriterId`, field names/types/value ranges, filters, correlation). Each deviation is recorded in the vector and reported as a spec finding, so the guideline example can be fixed.
3. **Compliant and meaningful:** the vector is the **corrected, complete** form of the example:
   - illustrative non-JSON (`//` comments, ellipses, placeholder elements, trailing commas) is removed or completed;
   - template placeholders (`<…>`) are replaced by realistic, guideline-conformant values; values are consistent across all vectors (one coherent fictitious installation: same manufacturer, apps, assets, Identification Links);
   - deviations from the description are corrected.
   Every change against the original is listed in `edits` (JSON Pointer or topic, kind `removed` / `completed` / `instantiated` / `corrected`, reason, SF id where applicable). Nothing is changed silently.
4. **Validity:** golden vectors are **positive** vectors (`expected: valid`) and reference the REQ ids they illustrate. Negative vectors are derived per REQ in P2.
5. **Review status:** every vector has `status: draft` until the PO confirms it (`reviewed`). PO remarks are fixed in the vector or become spec findings. Vectors in `draft` may be used by tests, but a P1/P2 exit requires all golden vectors to be `reviewed`.

This replaces the C-012 rule "no content is invented" and the rule "a contradicting example keeps the guideline content and is marked invalid".

## Amendment C-014 – format details and tools (agent decisions within this ADR, G-4.4)

Refinements made while building `oec/spec/` (C-014 log); the decision above is unchanged.

1. **Libraries (Go, `oec/spec/tools`):** `go.yaml.in/yaml/v3` (maintained continuation of `gopkg.in/yaml.v3` by the YAML organization; node API needed to enforce the subset) and `github.com/santhosh-tekuri/jsonschema/v6` (JSON Schema 2020-12). Both MIT/Apache-2.0 (G-4.10). The Rust-side YAML library is chosen when the SDK first reads the catalog.
2. **Strict YAML subset, enforced by `spectool`:** one document; no anchors, aliases, tags, merge keys; string keys, no duplicates; decimal numbers only; only `true`/`false`/`null`; YAML 1.1 booleans (`yes`, `no`, `on`, `off`, …) and dates must be quoted.
3. **Additional entry fields:** `scope` (sections graded by an umbrella entry, e.g. a level-table row), `quotes` (`ref` + verbatim text, checked against the guideline), `inventory` (keys of the covered inventory items), `replaced_by` (with `status: deprecated`). `verification` values: `schema`, `d7-validate`, `d7-monitor`, `d7-test`, `d8`, `manual`. `keyword` equals the Level 3 obligation for graded domains; `levels` use `none` for "not required"; `platform` entries have no levels. `applies_to` values: `component`, `application`, `device`, `publisher`, `subscriber`, `requester`, `responder`, `container-image`, `manufacturer`, `platform`, `broker`, `registry`.
4. **Additional files:** `requirements/areas.schema.json`; `requirements/view/` (generated Markdown views); `coverage/inventory.yaml` (generated), `coverage/informative.yaml` + schema (hand-written), `coverage/report.md` (generated).
5. **Coverage model:** `spectool inventory` extracts every candidate of a normative statement from MG + RIG (without Appendix B, lists, authors, history): sentences/list items/table rows with an MG 1.2 keyword, field definitions with `Requirement`, tip/warning/info boxes, keyword-less behavior (SF-035 candidates) and the remaining rows of tables with keyword rows. Keys hash the content, not the line, so unrelated guideline edits keep the mapping. Each item is mapped by a requirement or listed as informative with a reason; an area may only be `drafted`/`reviewed` without open items.
6. **Areas:** in addition to the C-012/C-013 list, `BUS` (MG 9 general part) and `GEN` (introductory chapters, cyber security, glossaries) so every section has an owner; `areas.yaml` assigns sections by longest prefix.

## Amendment C-018 – topic grammar, matrix and vectors (agent decisions within this ADR, G-4.4)

Built in C-018 (log `2026-10-15_C-018_il-top-topic-grammar.md`); the decision above is unchanged.

1. **Topic grammar:** `topic/topic.abnf` (RFC 5234 + RFC 7405 case-sensitive strings) is the source of truth for topic syntax; it must stay regular (no recursion, no prose values). `spectool topic` generates `topic/topic-patterns.json` (one anchored regex per rule, portable subset of ECMA-262 u-mode, Go RE2, Python `re` with `fullmatch`, Rust `regex`). The ABNF toolchain (`tools/internal/abnf`) is hand-written so the regex dialect stays under our control and no new dependency is needed.
2. **Matrix:** `matrix/method-resource.yaml` (+ schema) lists every Method × Resource/service combination with `allowed`, `source`/`filter` = required/optional/forbidden, the ABNF rule and guideline refs. Lint checks matrix ↔ grammar (each allowed row has its rule, forbidden rows have none, random grammar samples are classified as the matrix says). Payload expectations are added in C-019/C-022.
3. **Vectors:** `vectors/<kind>/*.yaml` with `vectors/vectors.schema.json`; kinds `topic`, `identification-link`, `dnp-encoding`, `oi4identifier` (later kinds are added with their areas). Every expected result is recomputed by `spectool lint` with the reference code in `tools/internal/topic` and `tools/internal/il`; the reference code is spec tooling, not an SDK (G-1.3 and ADR-0013 independence are unaffected).
4. **Golden vectors of string kinds** (topics, Identification Links) live next to the constructed vectors of the same kind (`vectors/topic/guideline.yaml`) instead of `vectors/golden/`. Rule 1 of amendment C-013 (extraction with source) is met by `origin.ref` plus a lint check that the value appears verbatim in the cited section at the baseline; a corrected value keeps the original in `origin.original` and the reason in `origin.sf` (the `edits` list of amendment C-013 is used for payload vectors from the MSG cycle on – C-022 after the C-019, C-020 and C-021 shifts). Golden vectors may be `expected: valid` with warnings (e.g. white space in a Filter, MG 7.7 SHOULD NOT).
5. **Independent cross-check:** the generated regexes and the grammar are checked once per grammar change with a second engine (C-018: Node.js `RegExp` u-flag, Python `re`, Rust `regex` 1.x, Python `abnf` 2.9 package); results are recorded in the cycle log.
