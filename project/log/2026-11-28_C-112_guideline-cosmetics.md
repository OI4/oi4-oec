# C-112 – Guideline cosmetics on PO order

- Date: 2026-11-28
- Phase: none open (all phases closed after C-111); PO order on the guideline text (GUARDRAILS G-3.4 a)
- Baseline before: guideline `52d8470` (branch `feature/spec-findings`, head `d2a47f0` = `origin`); after: **`c48bbd4`** (branch head `fa88154`); SF-001 … 117 resolved, next SF-118
- Workspace before the cycle: `2656ed5` (= `origin/main`)

## PO order (2026-11-05 session)

The PO sent cosmetic points for the guideline (German, quoted in translation):

1. **Glossary:** "to add/delete: OOC".
2. **Messaging Guideline, authors list:** "This revision of The OEC Messaging Guideline, the OEC Runtime & Infrastructure Guideline and the OEC Process Description are developed by the Open Edge Computing work group of the Open Industry 4.0 Alliance." The editor line (Konrad Heidrich, Hilscher Gesellschaft für Systemautomation mbH) can be dropped. Involved in the current revision: Werner Blumenstock, Guenes Bilgen, Bastian Schmick, Thomas Weinschenk, Konrad Heidrich, Sebastian Heinze.
3. **History:** everything before V1.0 goes; from V1.0 one page per revision (1.0, 1.1, 1.1.1 and 2.0).
4. **Preface:** can be deleted.
5. **Process Description:** can be hidden, because its content is not up to date.

Push: allowed (C-111 go); documentation-only pushes run no CI (C-111 D-3 (a)).

## Result

Points 2–5 are applied in the guideline (four commits, `b32fb74` … `c48bbd4`); point 1 (OOC) is left unchanged and proposed as **D-1**, because "to add/delete" leaves open which of the two is meant and the term is used in MG 7.2. The spec moved to the new baseline `c48bbd4` (the preface carried two informative inventory items); the feedback package of C-110 now ends at `c48bbd4`, and a counting defect of its diff statistics is fixed. Gate green (see "Gate"). No change to D7/D8 behaviour, the SDKs, the demos, the registry, the report formats or the schema content; no new dependency.

## What was done

### Guideline (`oi4-oec-documentation`, branch `feature/spec-findings`)

- `b32fb74` – **MG A6 History:** the single page `appendix-a/a6-history.md` becomes the folder `appendix-a/a6-history/` (same URL, `_category_.json` "A6 History"):
  - `index.md`: short introduction and Table 22 (the revisions 1.0, 1.1, 1.1.1 and 2.0 with date and link); everything before 1.0 (the drafts 0.x) is removed as ordered.
  - `a6-1-revision-1-0.md`, `a6-2-revision-1-1.md`, `a6-3-revision-1-1-1.md`: the former table rows converted into lists, wording unchanged (revision, date, authors, changes); the footnote mark of the old table became "<sup>\*</sup> Breaking change to guideline v0.12/v1.0."
  - `a6-4-revision-2-0.md` (new, written by the agent – **R-1**): date "in preparation", the six persons of the current revision as authors, the changes derived from the titles of GL-ADR-001 … 022 plus the items that the old history listed under "Vx.x".
  - Links to A6 updated in `list-of-tables.md`, MG `index.md`, RIG `index.md`, RIG `1-1-scope.md` and `process-description/index.md`.
- `bfea5af` – **MG A5 Authors:** the PO sentence as the new introduction (only "The OEC Messaging Guideline" → "the OEC Messaging Guideline" inside the sentence; the verb "are" kept – **R-2**); the editor line removed; new sentence "Involved in the current revision: Guenes Bilgen, Werner Blumenstock, Konrad Heidrich, Sebastian Heinze, Bastian Schmick, Thomas Weinschenk." (alphabetical). Table 21 (all authors with company) unchanged; Guenes Bilgen is not in it, his company is not known to the agent (**D-2**).
- `36f80c8` – **Preface** `docs/guideline/preface.md` deleted (no page linked to it except the sidebar).
- `c48bbd4` – **Process Description hidden:** `docusaurus.config.js` – the docs plugin keeps its default `exclude` patterns and adds `process-description/**`, so the pages are no longer built; navbar and footer entries removed; the card on the home page (`src/components/GuidelineCards/index.js`) removed; the "Related Documents" bullet in MG and RIG `index.md` and the informative sentence that linked it in MG 1.1 removed. The source files stay in the repository (to be revised later). The A5 introduction still names the Process Description, as in the PO wording.
- `f6ec69b` – Community pages `compliance/validate.md`, `oec-check.md`, `schema.md`: the sample output names the baseline `c48bbd4`.
- `fa88154` – feedback package regenerated (see below).
- Site build (`yarn build`, Docusaurus 3.10.2): no warning, no broken link; checked in the output: the A6 pages with nested lists and footnote marks, no `process-description` page, home page without the card.

### Workspace

- `eaa7c84` – spectool (test first, `TestLoadFilesSkipsInformativeParts`): the folder `a6-history` is always excluded from the inventory like the former file; Appendix B stays examples-only (A-632).
- `1e029f6` – **re-baseline** `spectool rebaseline -rev c48bbd4`: inventory 1411 → **1409** (the two informative preface sentences `MG-preface-ffb33878`, `MG-preface-a3a548f2` are gone, no other key changed); `areas.yaml` area CMP no longer lists "MG preface"; their two `informative.yaml` entries removed; `lint`, `render`, `coverage`: 444 REQs, 158 `schema`/`d7-validate` REQs, 0 gaps, no REQ draft.
- `884250b` – schema `2.0.1-dev` with `schema.Baseline = c48bbd4` (no content change, `Unreleased` line in `oec/schema/CHANGELOG.md`); D7/D8 catalogs, `COVERAGE.md` (baseline lines only) and `cmd/oec-check/list_test.go` on `c48bbd4`; `oec/CHANGELOG.md` 0.1.0 names `c48bbd4`.
- `6c1ed39` – **feedback package** (guideline `fa88154`): the first gate run failed at `spectool feedback -check -rev c48bbd4`: the package covers every guideline commit of `5f070bb..baseline`, and the four C-112 commits had no attribution. They are attributed as `wording` with the note "C-112, PO order: …" (A-633). Regenerating showed a defect from C-110: the "Complete diff" line ran `git diff --shortstat` over the whole repository, although the package names the document paths – C-110 printed 389 files instead of 369, and now the Community pages would have added ~180 more. Fixed test first (`TestGitLoad` expects exactly the document files); the package now says 375 files, 10918 insertions, 7232 deletions (equal to the command it prints).

## Decisions (agent, G-3.4 b)

- **A-630 History as one page per revision:** the folder keeps the URL of the old page (`index.md` is the category page) so that existing links stay valid; Table 22 stays as an overview with links; the rows of 1.0 … 1.1.1 are converted mechanically (no word changed); the 2.0 page is a proposal (R-1).
- **A-631 Hiding the Process Description:** excluded from the build (docs `exclude`) and every link to it removed, instead of deleting the files – the PO said "ausblenden", and the content is to be revised. GL-ADR-015 and older history entries still mention it as a document; they are history and stay.
- **A-632 Inventory:** the per-revision history pages are excluded like the former file (they carry no requirements, but contain words like "must" in the change descriptions).
- **A-633 Re-baseline and feedback package now:** `spectool lint` reads the guideline working tree, so the deleted preface fails the gate without a re-baseline; the spec, the schema baseline, the catalogs and the feedback package therefore move to `c48bbd4` in this cycle. The feedback package keeps its role (everything since the takeover); the four C-112 commits are `wording` (no requirement changed). If D-1/D-2 change the guideline again, the baseline moves once more.
- **A-634 Feedback diff statistics:** fixed as a defect (the printed number did not match the printed command); the number in A-615/C-110 (389 files) was too high, correct was 369.
- **A-635 Push:** the guideline branch is pushed (no workflow of `oi4-oec-documentation` runs on `feature/spec-findings`; the site deploys only from `development`). The workspace commits contain code and data (`oec/spec`, `oec/schema`, catalogs), so a push would start all six workflows (~7–8 runner hours, C-111 D-3); they stay local until D-1/D-2 are answered, so that a possible second re-baseline goes into the same run (**D-3**). Until then the hosted gate is not triggered; a manual run on `origin/main` would read the new guideline head and fail at `spectool lint` ("MG preface").

## For the PO

- **D-1 OOC (glossary).** "to add/delete: OOC" – the entry exists ("Open Operator Cloud platform; …", MG A4) and MG 7.2 uses the term ("OOCConnector … the higher-level Open Operator Cloud"). (a) **Recommended:** keep the entry (MG 7.2 needs it); (b) delete it, then also rephrase MG 7.2 (the `ServiceType` value `OOCConnector` stays, it is part of the enumeration); (c) keep it, but shorten it to "Open Operator Cloud – the cloud layer of the OI4 reference architecture".
- **D-2 Authors.** (a) **Recommended:** tell me Guenes Bilgen's company; I add him to Table 21 and keep the table (it lists all authors of all revisions) plus the new sentence; (b) replace Table 21 by the six persons with their companies; (c) leave it as it is.
- **D-3 Push of the workspace.** (a) **Recommended:** push after D-1/D-2 with one full CI run; (b) push now (full CI, ~7–8 runner hours); (c) push now with `[skip ci]` (verified only by the local gate).
- **R-1** the new page A6.4 Revision 2.0 (date "in preparation", authors, change list).
- **R-2** grammar of the introduction: "This revision of … are developed" – proposal "is developed" (or "These revisions … are developed").

## Gate

- First run (before the re-baseline, load ~16–23): failed at step 1, `spectool lint` – "area CMP: section \"MG preface\" does not exist in the guideline" (the lint reads the working tree) → re-baseline (A-633).
- Second run: failed at step 1, `spectool feedback -check -rev c48bbd4` – the four C-112 commits were not attributed → attribution and the diff-statistics fix (A-633, A-634).
- Third run on `6c1ed39` (guideline head `fa88154`, load 18 → 10): **all checks passed** – `lint`: 27 area files, 444 REQs, 313 informative items; `inventory -check -rev c48bbd4`; `feedback`: 117 findings, 215 commits of `5f070bb..c48bbd4` attributed, 135 files current; `docscheck: 19 pages ok, 30 README links to them ok`; `cigen: 7 workflows current`.

## Temporary files

`/tmp/c112/` (conversion script, intermediate pages, build log) and `/tmp/gate.log` – deleted at the end of the cycle. The guideline `build/` folder is git-ignored and existed before.

## Addendum 1 (2026-11-06) – PO answer on D-1 (OOC)

PO (German, in translation): "OOC with Open Operator Cloud is an outdated term at the OI4. Either we find a more suitable name that fits OOC, or we have to rename OOC."

Impact analysis: `OOCConnector` is a normative `ServiceType` value (MG 7.2, enumeration closed: "a new `ServiceType` requires a new version of this guideline"). It appears in the topic grammar and patterns (`oec/spec/topic`), REQ-TOP (`top.yaml`), the topic vectors, the schema (`envelope`, `actions`, `helpers`), the Rust SDK (`ServiceType::OOCConnector`), the C ABI header, the Go SDK, the micro SDK (`oecm.h`), D7 (`rules.go`, `probe.go`), the registry AsyncAPI and the Community page `concepts.md`. A new long form for OOC changes text only (MG A4, MG 7.2 sentence, `concepts.md`); renaming the value is a normative change (SF-118, GL-ADR, re-baseline, schema, every SDK, D7, registry). The PO is asked which way and which name (G-3.4 a).

The PO could not be reached for the choice of the name; nothing is changed in the guideline. D-1 is re-posed in REVIEW.md/REVIEW_DE.md with the options (a) new long form "Operator-Owned Cloud" (recommended, text only), (b) "Open Operations Cloud", (c) no long form, neutral MG 7.2 wording, (d) rename the value (e.g. `CloudConnector`, normative), plus the hint that the glossary entry CCC belongs to the same outdated layer model.

## Addendum 2 (2026-11-06) – PO background on OOC/CCC

PO (German, in translation): "The initial background behind OOC and CCC was the idea that an edge always connects only to the cloud of the operator. The operator clouds connect via CCC with e.g. the manufacturers. This is outdated. The actual problem with the name is that it is neither open nor about the operator. Connectors to manufacturer clouds like Netilion or moneo are allowed just as well as an own cloud of the operator. Correct would actually be CC (cloud connector) as the counterpart of the OT connector. Think again whether you find something fitting for OOC with this information, otherwise we rename it." Attached: the OI4 reference architecture figure (layers 1 Open Edge Connectivity, 2 Open Edge Computing with "OOC Connector", 3 Open Operator Cloud Platform, 4 Common Cloud Central).

## Addendum 3 (2026-11-06) – D-1 implemented: `OOCConnector` → `CloudConnector`

Assessment of addendum 2: no long form for OOC is both honest and fitting – the connector is neither "open" nor bound to the operator, and the third and fourth layers of the figure (operator cloud, CCC) no longer describe the architecture. Following the PO's own conclusion ("otherwise we rename it", "correct would be CC (cloud connector) as counterpart of the OT connector"), D-1 is answered as option (d): the value is renamed to **`CloudConnector`**.

### Guideline (`feature/spec-findings`)

- `472d58d` – SF-118 in the register (MG 7.2 / A4: the term OOC and its layer model are outdated) and **GL-ADR-023** "Rename the ServiceType OOCConnector to CloudConnector".
- `97b6acd` – MG 7.2: `CloudConnector` "Communicates with the Message Bus on one side and with a cloud platform on the other side, e.g. a cloud of the operator or a cloud of a manufacturer"; the OTConnector paragraph says "onboarded by `CloudConnector`"; glossary entries OOC and CCC deleted (MG A4); change line in A6.4.
- `8def86d` – SF-118 resolved (`97b6acd`).
- `479de0f` – feedback package: both commits attributed `GL-ADR-023,SF-118`; pages SF-118 and GL-ADR-023 generated; 118 findings, 217 commits up to `97b6acd`.
- `ca211a0` – Community pages: `concepts.md` (table row), `migration.md` (new section "ServiceType OOCConnector renamed", at-a-glance row, "GL-ADR-001 … 023"), `compliance/schema.md`, `oec-check.md`, `validate.md` (sample output "schemas 2.1.0-dev, baseline 97b6acd", heading "Unreleased (2.1.0-dev)").

### Workspace

- `066237b` – spec: re-baseline to `97b6acd` (inventory 1409 → **1407**: the two glossary sentences OOC/CCC gone; the MG 7.2 item re-keyed `MG-7.2-90e1d54a` → `MG-7.2-0947ae2f` in `informative.yaml`); REQ-TOP-005 lists `CloudConnector` (status `draft`); `topic.abnf`, `topic-patterns.json`, V-TOP-106 with `CloudConnector` (`draft`) and new invalid vector **V-TOP-255** "former ServiceType OOCConnector" (`draft`); exports and views regenerated.
- `633ef8c` – schema: `envelope`, `actions`, `helpers` enumerations; `schema.Version` **2.1.0-dev**, `schema.Baseline` `97b6acd`; CHANGELOG "Unreleased (2.1.0-dev)"; `oec/schema/check.sh` 636 cases, 0 disagreements. `release/2.0.0.json` stays frozen.
- `f22d052` – SDKs, test first: Rust `ServiceType::CloudConnector` (`closed_sets` test failed first); C ABI `OEC_ST_CLOUD_CONNECTOR` (value 5 unchanged, `oec.h` regenerated with cbindgen 0.29.4); Go `oec.CloudConnector` with new `TestServiceTypeNames` (reads `oec.h`; cgo is not allowed in `_test.go`); micro `OECM_ST_CLOUD_CONNECTOR` with new `test_service_type_names`, `vectors_gen.h` regenerated.
- `89d3514` – D7 `rules.go`/`probe.go`, registry `asyncapi.json`, D7/D8 catalogs, `COVERAGE.md` baseline lines, `list_test.go`, `oec/CHANGELOG.md` (baseline `97b6acd`, JSON Schema bundle 2.1.x, CloudConnector line).

### Decisions (agent, G-3.4 b)

- **A-636 Name `CloudConnector`:** not `CCConnector`/`CC` – spelled out like `OTConnector`/`ITConnector`, no clash with the deleted abbreviation CCC; position in the enumeration and C ABI value 5 unchanged.
- **A-637 CCC deleted with OOC:** the term was used nowhere else in the guideline and belongs to the layer model the PO called outdated (review R-3).
- **A-638 Schema 2.1.0-dev:** a new baseline with a visible enumeration change → MINOR. Strictly, removing a value is MAJOR, but 2.0.0 was never published and guideline 2.0 is not released; `release/2.0.0.json` stays frozen. Sample reports in the conformance fixtures keep "2.0.1-dev" (they are frozen test data, not the current version).
- **A-639 No deprecated aliases:** every SDK is pre-release 0.1.0 and unpublished; an alias would keep the outdated term alive. `OECM_API_VERSION` stays 1.
- **A-640 Drafts for review:** REQ-TOP-005, V-TOP-106 and V-TOP-255 are `draft` (G-3.9); draft vectors do not count, the gaps stay 0 because REQ-TOP-005 keeps its reviewed vectors.
- **A-641 Migration note:** `migration.md` documents the rename for 1.x implementers (`OOCConnector` existed in 1.x).

### For the PO

- D-1 answered (rename). Still open: **D-2**, **D-3**, **R-1**, **R-2**.
- **R-3** review: GL-ADR-023, the new MG 7.2 definition of `CloudConnector` and the deletion of the glossary entries OOC and CCC.
- **R-4** review (G-3.9): REQ-TOP-005 and the vectors V-TOP-106 (valid) and V-TOP-255 (invalid), status `draft`.

### Gate

Run on `89d3514` (guideline head `ca211a0`, load 14 → 8): **all checks passed** – `lint`: 27 area files, 444 REQs, 311 informative items, topic vectors 152; coverage 0 open in every area; `docscheck: 19 pages ok, 30 README links to them ok`; `cigen: 7 workflows current`. No flaky test.

### Push

The guideline branch is pushed (`ca211a0`, no CI there). The workspace commits stay local until D-3. `/tmp/gate.log` deleted.

## Addendum 4 (2026-11-06) – PO answers on D-2, D-3, R-1 … R-4

PO (German, in translation):
- "A5 authors: replace the entries in the table by the names from the enumeration."
- "D-2: Guenes Bilgen, Siemens AG."
- "D-3 (a)" – push the workspace once, after D-1/D-2 are applied.
- "R-1 accepted", "R-2 proposal accepted" ("This revision … is developed"), "R-3 accepted", "R-4 accepted" (REQ-TOP-005, V-TOP-106, V-TOP-255 → `reviewed`).
