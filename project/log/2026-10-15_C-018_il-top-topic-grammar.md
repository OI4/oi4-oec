# C-018 – Areas IL and TOP, formal topic grammar, topic vectors

- Date: 2026-10-15
- Phase: P1
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Status: done (PO decisions on SF-040 … 043 pending)
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

## Results

Commits (workspace repo, not pushed): `a8315ca` log plan · `8e798cf` SF-040 … 043 · `e325ea8` `internal/abnf` · `adae5cd` `internal/il` · `bbb4b1a` topic grammar + patterns + matrix + `internal/topic` · `4ed3133` vectors schema + `internal/vectors` + `spectool topic`/lint · `f868968` IL + TOP catalogs · `a452633` vectors · `a738c59` spec README · `4b15a76` ADR-0018 amendment C-018 · `20f0a82` GUARDRAILS G-1.2 · `76a3948` ROADMAP · this log + STATUS.

- **Topic grammar** `oec/spec/topic/topic.abnf`: 59 rules (RFC 5234 + RFC 7405), one rule per allowed Method × Resource/service form, closed `ServiceType`/`EventCategory` sets, Status levels `Good|Uncertain|Bad`, Syslog levels, `encoded-il` (base64url, ≤ 340 characters = 255 octets), `filter-level` (no NUL, `#`, `+`, `/`; `%` only in `%2B`/`%23`/`%2F`/`%25`). Open points are marked in comments with SF-040 … 043.
- **Regexes** `oec/spec/topic/topic-patterns.json`: 59 generated anchored patterns (largest: `topic`, 12,692 characters); `spectool topic -check` fails when outdated.
- **Matrix** `oec/spec/matrix/method-resource.yaml` (+ schema): 66 Method × Resource/service combinations, 29 allowed, each with `source`/`filter` = required/optional/forbidden, ABNF rule and guideline refs (topic columns only; payload columns follow in C-019/C-022).
- **Reference code** (spec tooling only): `internal/abnf` (parser, matcher, regex compiler, sample generator), `internal/il` (IL check, strict base64url, topic element check, dnp-encoding, `ManufacturerUri`, `Oi4Identifier`), `internal/topic` (analysis: rule, AppId/Source/Filter, IL check, warnings; grammar ↔ regex ↔ matrix check), `internal/vectors` (loader + checker).
- **`spectool`**: new command `topic [-check] [TOPIC…]`; `lint` now also checks grammar, matrix (baseline, refs, consistency with 100 random samples per allowed combination) and all vectors.
- **Catalogs** (all `status: draft`):
  - `requirements/il.yaml` REQ-IL-001 … 032 (21 MUST, 5 MUST NOT, 6 SHOULD).
  - `requirements/top.yaml` REQ-TOP-001 … 027 (19 MUST, 3 MUST NOT, 4 SHOULD, 1 SHOULD NOT).
  - The `Event` `Status` answer to `Set`/`Del` (REQ-TOP-014/015) is Level 1 SHOULD and Level 2/3 MUST (C-016).
  - Keyword-less structure rules (REQ-TOP-001, 005, 007, 011, 013, 022, 023) are `implicit` and refer to SF-035 plus the open SF-041/042.
- **Coverage**:
  - IL: 91 inventory items, 68 covered, 23 informative, 0 open.
  - TOP: 76 items, 62 covered, 14 informative, 0 open.
  - Both areas are now `drafted`.
  - `areas.yaml` cycle numbers were aligned with the real cycle sequence (C-018 … C-023; they were still shifted from the C-013 cut).
- **Vectors** (all `status: draft`, 203):
  - `vectors/topic/guideline.yaml`: 60 golden topics from MG 4.2, MG 7 and MG 9.
  - Appendix B topics whose `%20` was replaced by a space record `origin.original` and SF-040.
  - The B1.1.8 Set PublicationList example is rewritten to `…/Data/Network settings` (SF-040, SF-041).
  - `valid.yaml`: 34 constructed valid topics.
  - `invalid.yaml`: 52 constructed invalid topics, each naming the violated REQ and whether it fails the grammar or the IL check. One of them has a trailing line break, which a Python `$`-anchored regex would accept.
  - `il/identification-link.yaml`: 29 vectors. `il/dnp-encoding.yaml`: 15. `il/oi4identifier.yaml`: 13.

## Verification

- `go vet ./...`, `gofmt -l` (clean), `go test ./...` (abnf, catalog, coverage, guideline, il, inventory, topic, vectors, yamlstrict: ok) with go1.27.2 (`GOTOOLCHAIN=auto`).
- `spectool lint` → `ok: 3 area file(s), 104 requirement(s), 68 informative item(s)` and `ok: matrix/method-resource.yaml: 66 combination(s); vectors: topic=146 identification-link=29 dnp-encoding=15 oi4identifier=13`.
- `spectool render -check`, `coverage -check` and `topic -check` all pass. `inventory -rev bc5d063 -check` passes: 1213 items.
- Cross-builds: `go build ./...` succeeds for windows/amd64, darwin/amd64, linux/amd64 and linux/arm64.
- **Independent second engines** were run once from the session folder; nothing is committed. Each run used all 146 topic vectors with the expected outcome "matches `topic`", meaning valid, or invalid only because of the IL check. Vectors with a parse rule were also checked against that rule's pattern. All engines reported 0 mismatches:
  - Node.js 22.13 `RegExp` with the `u` flag, all 59 patterns;
  - Python 3.12 `re.fullmatch`;
  - Rust `regex` 1.13.1 (rustc 1.89);
  - Python package `abnf` 2.9.0 (independent ABNF parser), which loads `topic.abnf` directly (59 rules) and matched every vector against `topic` and its parse rule.

## Decisions (agent, within ADR-0018 – amendment C-018)

- `topic.abnf` is the source and the regexes are generated. The ABNF toolchain is hand-written, with no new dependency.
- Golden string vectors live next to the constructed vectors of the same kind. Lint proves that each one appears verbatim in the cited section; corrections are recorded in `origin.original` and `origin.sf`.
- The matrix holds the topic columns only, so the C-022 matrix work is reduced to payload columns (ROADMAP updated).
- GUARDRAILS G-1.2 still named `49b6fe2` as the baseline. It was updated to `bc5d063`.

## Spec findings / PO input

- **SF-040 … 043 are open.** Grammar, catalogs and vectors use the proposed interpretations from `SPEC-FINDINGS.md`:
  - SF-040: filter encoding is strict (upper-case `%2B`/`%23`/`%2F`/`%25` only), and Appendix B `%20` is treated as a space.
  - SF-041: only the forms listed in MG 9 are allowed, `Call`/`Reply` take an optional `Source`, and the list-resource filter is `<ResourceType>[/<Tag>]`.
  - SF-042: `ServiceType` and `EventCategory` are closed sets, and the OTConnector rule is implicit.
  - SF-043: dnp-encoding covers every UTF-8 octet, and `ManufacturerUri` is lower case.
- The PO was asked at the end of the cycle (SF-040) but was not available. Nothing is confirmed. The questions are repeated in STATUS "Next step".
- A decision that differs from a proposal changes `topic.abnf` (and with it the regexes), the affected REQ notes and the vectors. Lint shows every affected vector.
- The register in the guideline repository (`findings/SPEC-FINDINGS.md`) was not touched (no PO request).

## Next step

- PO: decide SF-040 … 043, and review `requirements/view/il.md`, `requirements/view/top.md` and the vectors (G-3.9).
- C-019: areas `MSG` + `ACT` and the payload columns of the matrix.
