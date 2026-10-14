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
