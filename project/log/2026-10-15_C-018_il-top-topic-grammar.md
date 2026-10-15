# C-018 – Areas IL and TOP, formal topic grammar, topic vectors

- Date: 2026-10-15
- Phase: P1
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Status: in progress
- Guideline baseline: `oi4-oec-documentation` @ `bc5d063`, branch `feature/spec-findings` (checked at cycle start: HEAD = `bc5d063`, working tree clean)

## Intent

PO request (chat, 2026-10-15): execute C-018 as planned in STATUS "Next step" and ROADMAP P1 – areas `IL` (MG 4, MG A3) and `TOP` (MG 7), the formal topic grammar (`oec/spec/topic/topic.abnf` + derived `topic-patterns.json`) and topic test vectors including their lint; golden vectors for IL/TOP may start (ADR-0018 amendment C-013).

## Scope / acceptance criteria

- `requirements/il.yaml` and `requirements/top.yaml` (`status: draft`); every inventory item of MG 4, MG A3 and MG 7 is covered by a requirement or classified informative; areas `IL` and `TOP` become `drafted`.
- `topic/topic.abnf` (RFC 5234 + RFC 7405) defines the topic names below `Oi4v2`: generic elements and one production per allowed Method × Resource/service combination with required/optional/forbidden `Source`/`Filter` and the resource-specific filter structures (MG 7.7, MG 9.1.x, MG 9.2.1).
- The regexes in `topic/topic-patterns.json` are **generated** from the ABNF (common subset of ECMA-262, RE2 and Rust `regex`; anchored, no look-around, no back-references) – `-check` mode for CI.
- Topic columns of the Method × Resource matrix (`matrix/method-resource.yaml`): allowed yes/no, `Source`/`Filter` required/optional/forbidden, ABNF rule; a tool check keeps matrix and ABNF consistent (C-012 §4 step 1).
- Language-neutral vectors (`vectors/`): topic vectors (valid with expected parse result, invalid with violated REQ ids), Identification Link vectors (validity + base64url encoding) and dnp-encoding/`Oi4Identifier` vectors; all guideline topic examples (MG 4, MG 7, Appendix B) are included as golden topic vectors; `status: draft` (G-3.9).
- `spectool lint` checks the vectors against the ABNF (interpreter) **and** the generated regexes (Go RE2) and against a reference check of the Identification Link rules; an independent second engine (ECMA-262 and an external ABNF implementation) is run once and recorded here.
- New spec findings (SF-040 ff.) for ambiguities found on the way, with proposed interpretations used in grammar, catalog and vectors.

## Inputs

- STATUS, ROADMAP P1, C-012 plan §3–§5, C-013/C-014/C-017 logs, ADR-0018 (+ amendments), GUARDRAILS G-1.6, G-2.1, G-2.2, G-2.4, G-2.5, G-2.17, G-3.5, G-3.9.
- Guideline @ `bc5d063`: MG 1.2, MG 4 (4, 4.1, 4.2, 4.3, 4.3.1, 4.3.2), MG 7 (7, 7.1 … 7.7, 7.5.1, 7.5.2), MG 8.2.16 (`ResponseAddress`), MG 8.3.6.x, MG 8.3.8, MG 9.1.1 … 9.1.10, MG 9.2.1, MG A3, MG B1.1.x/B1.2.1 topics, RIG 4.2.

## Plan

1. Spec findings for the topic/IL ambiguities (SF-040 ff.).
2. Go package `internal/abnf` (parser RFC 5234 + RFC 7405, matcher, regex compiler, random generator) with tests.
3. `topic/topic.abnf`, `matrix/method-resource.yaml` (+ schema), `spectool topic` (generate `topic-patterns.json`, check matrix ↔ ABNF via generated samples).
4. Reference checks `internal/il` (Identification Link rules, canonical base64url, dnp-encoding, `Oi4Identifier`).
5. Vector schema + vectors (topic, IL, dnp/`Oi4Identifier`), lint integration.
6. Catalog `IL` and `TOP` + informative classification; render, coverage.
7. Verify; record (log, STATUS, README, ADR-0018 amendment, GUARDRAILS).
