# C-014 – `oec/spec/` skeleton, catalog schema, tools, inventory, area CMP

- Date: 2026-10-14
- Phase: P1
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Status: done – `oec/spec/` skeleton, `spectool` (lint/render/inventory/coverage), inventory of MG + RIG (1232 items), area `CMP` drafted (32 REQs, 88/88 items mapped); SF-037/038 open; CMP awaiting PO review
- Guideline baseline: `oi4-oec-documentation` @ `49b6fe2`, branch `feature/spec-findings` (checked at cycle start: unchanged, working tree clean)

## Intent

First P1 execution cycle (ROADMAP P1, PO-confirmed in C-013): create the machine-readable spec baseline `oec/spec/` (ADR-0018) with its catalog format, the Go tools that keep it consistent, the normative-sentence inventory of MG + RIG as coverage checklist (incl. the SF-035 candidate list), and the first area `CMP` (compliance domains and levels).

## Scope / acceptance criteria

- `oec/spec/` exists with `README.md`, `requirements/` (`catalog.schema.json`, `areas.schema.json`, `areas.yaml`, `cmp.yaml`, generated Markdown views), `coverage/` (generated `inventory.yaml`, hand-written `informative.yaml`, generated report) and `tools/`.
- Go module `oec/spec/tools` with one cross-platform CLI `spectool` (G-4.13):
  - `lint` – strict JSON-compatible YAML (no anchors/aliases/tags/merge keys, no duplicate keys), JSON Schema validation, id/area/level/ref/SF rules; with `-guideline` also checks that every quoted guideline text still exists in the referenced section;
  - `render` – Markdown view per area (`-check` fails if a view is outdated);
  - `inventory` – extracts every normative candidate (keyword sentences, list items, table rows, field `Requirement` attributes, admonitions, keyword-less behavior sentences = SF-035 candidates) from MG + RIG (without Appendix B, list of figures/tables, authors, history) with stable keys;
  - `coverage` – maps inventory items to REQ ids / informative reasons and reports open items per area and section; unknown keys are errors.
- Unit tests for the tools; `go vet` clean.
- Area `CMP` written from MG 1.3, MG 7.2 (7.2.1, 7.2.2) and RIG 1.2; all inventory items of these sections mapped to a REQ or marked informative.
- Status of all new REQ entries: `draft` (PO review per increment, G-3.9).
- New spec findings recorded (SF-037 ff.).

## Inputs

- ADR-0018 (+ amendment C-013), GUARDRAILS G-1.6, G-3.9, G-4.4, G-4.10, G-4.13, C-012 plan §1–2 (as updated by C-013).
- Guideline @ `49b6fe2`: MG 1.2, 1.3, 7.2, 7.2.1, 7.2.2, RIG 1.1, 1.2; all MG/RIG Markdown files for the inventory.

## Plan

1. Toolchain and dependencies (G-4.4): Go `1.27.2` toolchain (current stable; `go` directive 1.26 = oldest supported release), `go.yaml.in/yaml/v3` (maintained continuation of `gopkg.in/yaml.v3` by the YAML organization; node API needed for the strict subset), `github.com/santhosh-tekuri/jsonschema/v6` (JSON Schema 2020-12, candidate named in ADR-0013). Licenses checked (MIT-compatible, G-4.10).
2. Schemas + `areas.yaml` + tool skeleton, `lint`, `render` with tests.
3. `inventory` + `coverage` with tests; generate the inventory at `49b6fe2`.
4. Area `CMP` + mapping of the CMP inventory items; render; coverage report.
5. Record: log, STATUS, findings.

## Work done

1. **Go module `oec/spec/tools`** (`go 1.26.0`, `toolchain go1.27.2` – local Go 1.24.2 fetches it via `GOTOOLCHAIN=auto`), module path `github.com/OI4/oec/spec/tools` (provisional, like the legacy org path; changes only if the GitHub location changes, Q-011).
   - `internal/yamlstrict` – loader for the JSON-compatible YAML subset (ADR-0018 amendment C-014 §2), returns `json.Number`; `Decode` rejects unknown fields.
   - `internal/schema` – JSON Schema 2020-12 compile/validate with readable errors.
   - `internal/guideline` – reads MG + RIG Markdown (excl. `appendix-b/`, list of figures/tables, A5 authors, A6 history), section from the H1 number or a path slug (`index`, `preface`, `appendix-a`); extracts sentences (abbreviation- and inline-code-aware split), list items, table rows ("Header: cell | …"), field definitions (`Value Range`/`Type`/`Access`/`Requirement`) and tracks headings and admonitions; classifies by basis `keyword` / `field` / `admonition` / `implicit` (SF-035 candidates) / `table` (keyword-less row of a table that has keyword rows, e.g. `Set` "Where applicable"). Keys = `<DOC>-<section>-<sha256[:8]>` over doc, section, kind, heading path and normalized text (+ occurrence for duplicates).
   - `internal/inventory`, `internal/catalog` (load + lint), `internal/render`, `internal/coverage`, `cmd/spectool`.
2. **Schemas:** `requirements/catalog.schema.json`, `requirements/areas.schema.json`, `coverage/informative.schema.json` (2020-12, `$id` `urn:oi4:oec:v2:spec:*` – no hosting implied).
3. **`areas.yaml`:** 26 areas with sections, cycle and status; new areas `BUS` (MG 9 general) and `GEN` (MG 2, MG 3, MG A4, RIG 3, RIG A1) so that every section has an owner (no unassigned item).
4. **Inventory** `coverage/inventory.yaml` at `49b6fe2`: 1232 items – MG: 573 keyword, 230 field, 246 implicit, 50 table, 5 admonition; RIG: 89 keyword, 3 field, 13 implicit, 20 table, 3 admonition.
5. **Area CMP** (`requirements/cmp.yaml`, 32 entries, all `draft`):
   - 001–010 rows of the MG 1.3 messaging level table (umbrella entries with `scope`);
   - 011/012 "labeled compliant without level = Level 3" (messaging / service container);
   - 013 Registry may show an observed level (platform, MAY);
   - 014–016 methods Pub/Get/Set, 017 MAM/Health via Get from L2, 018–020 Profile/PublicationList (applications only)/ReferenceDesignation, 021 required resources listed in Profile, 022 Data → Metadata, 023 Set/Del → Event `Status`, 024 smart device with own client = application (implicit, SF-035);
   - 025/026 MG 1.2 field conventions (presence per `Requirement`, `Type`/`Value Range` when present);
   - 027–032 rows of the RIG 1.2 service container level table.
   - 29 CMP items classified informative (`coverage/informative.yaml`, with reasons). CMP status `drafted`.
6. Generated `requirements/view/{README,cmp}.md` and `coverage/report.md`; `oec/spec/README.md`.
7. SF-037, SF-038 recorded; ADR-0018 amendment C-014; ROADMAP area list/cycle cut (BUS, GEN).

## Verification (commands + results)

- Guideline: `git -C oi4-oec-documentation rev-parse --short=7 HEAD` → `49b6fe2` at start and end, working tree clean.
- `cd oec/spec/tools && go vet ./... && go test ./...` → ok (yamlstrict, guideline, inventory, catalog, coverage); `gofmt -l .` empty.
- Cross-build `go build ./cmd/spectool` for windows/amd64, linux/amd64, darwin/amd64, linux/arm64 → ok (G-4.13).
- `spectool lint` → `ok: 1 area file(s), 32 requirement(s), 29 informative item(s)` (incl. refs/scope/areas sections exist and all 36 quotes found verbatim in the guideline). Negative check: altering one quote ("Level 3" → "Level 2") → 2 lint errors as expected.
- `spectool render -check`, `spectool coverage -check` → ok; coverage CMP: 88 items = 59 covered + 29 informative, 0 open; total 1232 items, no section without area.
- Inventory sanity by grep: 233 field items (230 MG + 3 RIG) = 233 `^Requirement:` lines in MG + RIG; admonition box counts (`:::info REQUIRED` 16, `:::tip RECOMMENDED` 5, `:::warning HIGHLY RECOMMENDED` 1) consistent (boxes whose text has a keyword get basis `keyword`); the `Set` "Where applicable" rows of MG 7.2.1/7.2.2 are captured (basis `table`).
- Spot check of all 88 CMP items against MG 1.2, 1.3, 7.2, 7.2.1, 7.2.2, RIG 1.1, 1.2 (full text read): sentences split correctly, level-table rows complete.

## Decisions taken

Agent decisions within ADR-0018 (recorded as ADR-0018 amendment C-014, G-4.4):

- Go libraries `go.yaml.in/yaml/v3` v3.0.5 and `santhosh-tekuri/jsonschema/v6` v6.0.3 (MIT/Apache-2.0); `golang.org/x/text` updated to latest. Not chosen: `goccy/go-yaml` (no need for its extras), `go.yaml.in/yaml/v4` (still release candidate).
- One CLI `spectool` instead of several tools; generated files with `-check` mode for CI.
- New fields `scope`, `quotes`, `inventory`, `replaced_by`; `verification` split into `d7-validate` / `d7-monitor` / `d7-test`; `keyword` = Level 3 obligation; `levels` value `none`.
- Areas `BUS` and `GEN` added (C-019 / C-020).
- Catalog interpretation conventions:
  - `statement` in present tense; the obligation sits in `keyword`/`levels`.
  - Conditional requirements (e.g. Data → Metadata) carry `condition` and the levels of the condition text; where the guideline names no level, the plain keyword applies to all levels unless the level tables exclude it (SF-037).
  - The "no level stated = Level 3" rule is split per domain (one entry each for D7 and D8).
  - Keyword sentences that only restate the MG 1.2 conventions, introduce a table, give examples, or use "can" for possibility are informative.

## Findings

- **SF-037** (open) – Level 3 scope of methods/resources: MG 1.3 "e.g." list vs. MG 7.2.x minimum sets; `Del`/`Call`/`Reply` without level; "Where applicable" not in the MG 1.2 mapping, `Set` sentence without level; Set/Del → Event should (MG 7.1.4) vs. mandatory (MG 7.2.x). Interpretation used by REQ-CMP-010/016/022/023.
- **SF-038** (open) – MG 1.3 Level 2 bullet implies OCI deployment from L2; RIG 1.2 makes it mandatory at L1. REQ-CMP-027/028 follow RIG 1.2.
- Observation (no finding): MG 1.3 contains normative-sounding statements without keyword on the declaration and test procedure ("The compliance level is declared by the manufacturer …", "There is no message on the Message Bus to declare a compliance level", "The compliance is validated per compliance level …"). They are not protocol behavior, so SF-035 does not apply; they are design inputs for D7/D8 (declared level as test-run parameter, cumulative levels) and should be reflected in the D7/D8 design (P3/P5).

## Follow-ups / next step

- **PO review (G-3.9):** `oec/spec/requirements/view/cmp.md` (32 drafts) and the 29 informative classifications in `coverage/informative.yaml`; decide SF-037 and SF-038.
- **C-015:** areas `IL` (MG 4, A3) and `TOP` (MG 7, 7.1), formal topic grammar (`topic/topic.abnf`, `topic-patterns.json`) and topic vectors; golden vectors for IL/TOP examples may start (ADR-0018 amendment C-013). Needs tooling for topic vectors (lint/validation) and the Appendix B extraction (golden vectors) – planned in C-015/C-021.
- Open tool items: Appendix B extractor for golden vectors (C-021 or earlier); CI wiring of `spectool … -check` + `go test` once the repo has CI (P2).
- Re-baseline first if the guideline commit changes (PO update for SF-035/036/037/038 expected): regenerate the inventory, update `baseline` in `areas.yaml`, `cmp.yaml`, `informative.yaml`, remap the keys `lint` reports.
