# C-014 – `oec/spec/` skeleton, catalog schema, tools, inventory, area CMP

- Date: 2026-10-14
- Phase: P1
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Status: in progress
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
