# 2026-11-18 – Guideline cosmetics and guideline links before publishing

PO order (outside the cycles, after the unlisted preview
`2026-11-18_PO_docs-preview.md`): cosmetic points before publishing.

- Runtime & Infrastructure Guideline: no 2.1 without a 2.2; RECOMMENDED in
  4.1.1 shown as text instead of an admonition; missing links in 4.1.1,
  4.1.2 and 4.2; the version on the first page not updated.
- Community pages (Concepts, `oec-check`, `oec-infra`, Migration notes):
  missing links to the guideline sections they cite.

Guideline baseline before: `980cde8` (head `3661c53`); after: **`c7de341`**
(head `aac633b`). SF-001 … 119 resolved, next SF-120; no new finding.

## Decisions

- **Version text (PO, asked):** "Version 2.0" in both guidelines; the
  disclaimer sentence stays. GUARDRAILS G-1.2 notes the new version string.
- **"Link to 4.2.1 in 4.1.1" (agent):** there is no RIG 4.2.1; the
  sentence in 4.1.1 refers to 4.1.2 OI4 Certificate Storage, which is now
  linked.
- **RECOMMENDED (agent):** moved from the table cell into a
  `:::tip RECOMMENDED` admonition below the table – the form the guideline
  uses for its other seven RECOMMENDED notes. The same table cell in 4.1.2 is
  changed the same way. The sentence reads "Do not store any private keys in
  this folder!" (was "… here!", which no longer follows the folder row).
- **2.1 merged into 2 (agent):** chapter 2 had a short introduction and a
  single sub-section; it is now one page with the slug of chapter 2. The
  references to "2.1 Introduction to Containerization" (RIG 1.3 table, RIG
  index, MG 2) name "2 Considered Standards and Technical Decisions".
- **G-4.11 amended (PO order, implied by the request):** Community pages
  linked no guideline section because D6 was a standalone site when the
  rule was written; since C-100 addendum 2 they share the site. A section is
  still named by its number, and the number links the section file
  relatively (`Messaging Guideline [7.2](../guideline/…)`); the URL of the
  site stays forbidden. ADR-0031 amendment 2026-11-18.

## Done

Guideline repository (`oi4-oec-documentation`, branch
`feature/spec-findings`, not pushed):

- `7fa5762` – RIG 2 merged page; RIG 1.3, RIG index and MG 2 references;
  RIG 4.1.1/4.1.2 RECOMMENDED admonitions and links (4.1.2, MG 6.2.2,
  4.1.3, 4.2); RIG 4.2 links (MG 4, MG 4.3, 4.1.2, MG 7.3, MG 6.2.3);
  "Version 2.0" in both guidelines.
- `c7de341` – 72 links from `concepts.md`, `migration.md`,
  `compliance/oec-check.md` and `compliance/oec-infra.md` to the cited MG
  and RIG sections (`%20` for spaces in folder names, code blocks skipped).
- `7eae9a8` – feedback package: `7fa5762` attributed as `wording`; 119
  findings, 221 commits of `5f070bb..c7de341`, 138 files.
- `aac633b` – Community sample outputs (`oec-check.md`, `validate.md`,
  `schema.md`) name `c7de341`; they still named `605fbb7` (missed in
  C-113 addendum 1), checked against `oec-check` built from the workspace.

Workspace:

- `74b4483` – `docscheck` accepts relative links into `docs/guideline` and
  `docs/infrastructure-guideline` and checks file, folder and anchor (paths
  URL-unescaped); tests written first; G-4.11, ADR-0031, `oec/docs` README
  and check scripts.
- `4d865f9` – spec re-baselined on `c7de341`: inventory unchanged at 1411
  items; remapped REQ-CMP-027 (`RIG-1.3-2a895e2a`, scope RIG 2), REQ-CNT-002
  (`RIG-4.1.1-0acc5ff8`, `RIG-4.1.2-b04b4ab2`, quotes now the admonition
  sentence), REQ-PLT-006 (ref RIG 2) and three informative `RIG-2-*` items;
  445 REQs, 0 gaps.
- `8803b5e` – `schema.Baseline` `c7de341` (Version stays `2.1.0-dev`, no
  content change, CHANGELOG line), D7/D8 catalogs, `COVERAGE.md`,
  `list_test.go`, `oec/CHANGELOG.md`.
- Living documents: AGENTS.md, GUARDRAILS G-1.2, STATUS, ADR-0001
  amendment 2026-11-18.

## Verification

- `yarn build` of the guideline site: no broken link or anchor warning;
  spot checks: the admonitions in RIG 4.1.1/4.1.2, "Version 2.0" on both
  first pages, Community links resolve to the guideline routes.
- `docscheck: 20 pages ok, 34 README links to them ok`.
- Gate `oec/check.sh`: **all checks passed** – `lint`: 27 area files, 445
  REQs, 313 informative items; `inventory -check -rev c7de341`; `feedback`:
  119 findings, 221 commits of `5f070bb..c7de341`, 138 files current;
  `docscheck: 20 pages ok, 34 README links to them ok`; `cigen: 7 workflows
  current`. The first run failed in `oec-buildlib` ("llvm-objcopy not
  found"): the shell of the agent had Homebrew's `rustc` before rustup's in
  `PATH`, whose sysroot has no `llvm-tools`; run again with
  `PATH="$HOME/.cargo/bin:$PATH"` (environment only, no code change).

## Open (PO)

- Push both repositories; publish the preview as in
  `2026-11-18_PO_docs-preview.md` (the cosmetics are on
  `feature/spec-findings`, the site deploys from `development`).
