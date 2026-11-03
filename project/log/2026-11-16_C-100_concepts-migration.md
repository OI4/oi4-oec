# C-100 – P8 increment 3: Concepts and migration notes 1.x → 2.0

- Date: 2026-11-16
- Phase: P8 – Documentation (D6), increment 3 (ADR-0031 §7)
- PO input: the C-100 start prompt (nothing open in REVIEW.md; no answer, no new order to record).
- Baseline: guideline `52d8470` (branch `feature/spec-findings`, head `eb821e9` – `findings/` only), unchanged since C-078; SF-001 … 117 resolved, next SF-118. 1411 inventory items, 444 REQs, 158 `schema`/`d7-validate` REQs with 0 gaps, no drafts. 55 D7 and 17 D8 tests `implemented`; report version 1 frozen; schema 2.0.0 frozen, work state `2.0.1-dev` @ `52d8470` – all unchanged.

## Goal

Two pages of the site: a short **Concepts** page and the **Migration notes 1.x → 2.0** (ROADMAP P8, ADR-0031 §7 item 3). Content derived from the guideline (G-1), the guideline named by section number only, no links to the guideline site (G-4.11), details linked to the READMEs instead of copied.

## What was done

1. **Sources read.** Messaging Guideline 1.1, 1.3, 2, 4 (4.1, 4.2, 4.3, 4.3.1), 5, 6.3, 7 (7.1 … 7.7), 8 (index, 8.1.4, 8.2.1, 8.2.2, 8.2.3, 8.2.6, 8.3.2, 8.3.3), 9, A2; Runtime & Infrastructure Guideline 1.3, 4.1.3, 4.1.4, 5.1; the guideline ADRs GL-ADR-001 … 022 (context, outcome, implementation notes). `legacy/` was only read to confirm the 1.x topic form (`Oi4/<ServiceType>/<four levels of the Oi4Identifier>/…`); nothing was copied (G-1.3).
2. **`oec/docs/docs/concepts.md`** (`sidebar_position: 1`, A-501) – sections:
   - *Assets and roles*: asset, `MAM` (MG 5), directly connected and represented entities (MG 1.3), the seven ServiceTypes with their main task (MG 7.2), a smart device uses `OTConnector`;
   - *Identities*: Identification Link after IEC 61406-1 (MG 4.1: at most 255 characters, optional scheme, compared character by character, sources), `Oi4Identifier` (MG 4.3, dnp-encoded elements, MG 4.3.1, A3), base64url in topics and unencoded in payloads (MG 4.2), `AppId` and `Source` (MG 7.3, 7.6);
   - *Topics*: the schema `Oi4v2/<ServiceType>/<AppId>/<Method>/<Resource>[/<Source>[/<Filter>]]` with the example of MG 4.2, publisher vs. receiver for `ServiceType`/`AppId` (MG 7.2, 7.3), `Filter` (MG 7.7), only the forms of MG 9.1/9.2; link to the topic grammar in `oec/spec/README.md`;
   - *Resources and methods*: the ten resources (MG 7.5.1), the six methods (MG 7.4), the status `Event` after `Set`/`Del`, `NewDataSetWriterId` as the only service (MG 7.5.2, 9.2.1), Birth/Close/Will (MG 6.3);
   - *Messages*: OPC UA Part 14 V1.05.04 JSON (MG 8), `DataSetWriterName`/`WriterGroupName`, `ReplyTo`, no own keys and tolerant receivers (MG 8); links to `oec/schema/README.md` and `oec-check validate`;
   - *Compliance levels*: the two domains with three levels each as a table (MG 1.3, RIG 1.3), declaration by the manufacturer, "OI4 compliant" = level 3, platform requirements without levels (RIG 5.1), D7/D8 and the observed level of the registry;
   - *Where next*.
3. **`oec/docs/docs/migration.md`** (`sidebar_position: 5`, A-502) – an *At a glance* table (area, 1.x, 2.0, decision) and per change **What changes** / **What a 1.x application must do**:
   - *Migrate step by step*: `Oi4` and `Oi4v2` side by side on one broker, answer in the namespace of the request, never mixed (MG 7.1); the deliverables of this repository are v2 only (ADR-0015);
   - Identification Link instead of `Oi4Identifier` (GL-ADR-010) – one topic level, `PublisherId`, `ProductInstanceUri`, `ReferenceDesignation` property `IdentificationLink`, no partial wildcards any more (select by `MAM`);
   - `Oi4v2` (GL-ADR-003); `DataSetWriterName`/`WriterGroupName` (GL-ADR-004, 018 – topic elements and list-entry fields keep their names); `ReplyTo` instead of `CorrelationId` (GL-ADR-005); OPC UA V1.05.04 with fixed content masks and `NodeId` strings (GL-ADR-006, 013); Actions instead of `MSG` (GL-ADR-007, 012, 014); Health `Details` instead of the NE107 event, order of priority (GL-ADR-001, 008, SF-096); SBOM in `/opt/oi4/licenses` instead of the license resources, removed resources with reserved `DataSetClassId`s (GL-ADR-002, 008, MG A2, RIG 4.1.4, 5.3); compliance levels (GL-ADR-009, 017); two guidelines (GL-ADR-011, 015);
   - *Further changes in 2.0*: secrets in `/run/secrets/oi4/` and both means of authentication (GL-ADR-022, SF-117, RIG 4.1.3), masked sensitive `Config` values (GL-ADR-021, MG 8.3.3), pagination/`Locale`/`Query` (GL-ADR-019, 020);
   - *Check the result*: D7/D8, `oec/dev`, the SDKs.
   GL-ADR-016 (an empty chapter removed) needs no action and is not listed.
4. **Sidebar (A-503).** `sidebar_position` of `demos/index.md`, `registry/index.md`, `micro-sdk/index.md` 1/2/3 → 2/3/4; order: Getting started, Concepts, Demo services, OEC Registry, OEC Micro SDK, Migration notes. The guides of C-101/C-102 go before the migration notes.
5. **Intro and README.** `intro.md` "Where to go next" links both pages; the sentence that concepts and migration notes follow is reduced to the how-tos. `oec/docs/README.md` layout table lists the two pages.
6. **Checks.** `oec/docs/check.sh`: `docscheck: 9 pages ok` (the README anchors `#citing-requirements`, `#topic-grammar-and-regexes`, `#oec-check-validate`, `#oec-infra-image` resolve). Opt-in `oec/docs/build.sh` (A-504): see Results.

## Results

- **Build (opt-in, A-504):** `oec/docs/build.sh` about 31 s, `[SUCCESS] Generated static files`, `docs: ok`; no broken link or anchor; `build/` 1.0 MB with 10 HTML pages (the 9 pages and 404). Sidebar order in the built HTML: `/`, `/concepts`, `/demos/`, `/registry/`, `/micro-sdk/`, `/migration`. Repository links rewritten to `https://github.com/OI4/oi4-oec/blob/main/oec/…` (8 distinct on each new page, with the README anchors); no `oi4.github.io` link. The only warning is again npm's deprecation of `uuid` 8.3.2 (A-499).
- No contradiction or gap in the guideline found → no SF, no D-x.
- D7, D8, SDKs, demos, registry, micro SDK, schemas, catalog, `COVERAGE.md` and the npm tree unchanged; no new dependency.

## Agent decisions (G-3.4 b)

- **A-501** Concepts page: scope and sources as in item 2; a table each for ServiceTypes, resources and levels; the guideline by section number only; details linked to `oec/spec`, `oec/schema`, `oec/conformance`, the SDK READMEs and the site pages.
- **A-502** Migration notes: structure *What changes* / *What a 1.x application must do*; beyond the ROADMAP list (GL-ADR-001 … 011) also GL-ADR-012 … 015, 017 … 022 where a 1.x application has to act; 1.x facts from the GL-ADR contexts, MG 7.1 and MG A2, current section numbers only; no content from `legacy/`.
- **A-503** Sidebar order: Concepts right after Getting started, the migration notes last; the three product pages move by one.
- **A-504** Opt-in build result as above; no new probes (the C-099 probes cover the link rules).
- **A-505** Increment 4 (SDK guides, API reference) stays in C-101, in a new session (G-3.10).

## Decision for the PO

None. No R-x, no SF.

## Open / next

- C-101 – P8 increment 4: SDK guides Rust, C, Go (first application, provider, consumer) and the API reference.
- Unchanged for information: A-113 and A-422.

## Docker

Not used in C-100.

## Temporary files

`/tmp/docs-build.log` and the gate log `/tmp/gate.log` – removed at the end. The git-ignored `oec/docs/node_modules/`, `oec/docs/build/` and `oec/docs/.docusaurus/` stay for the next increment.

## Handover (G-3.10)

**A new session is recommended** for C-101: this session's context was compacted. `prompt.md` (workspace root, German, not committed) holds the start prompt for C-101 (P8 increment 4).

## Gate

`caffeinate -i oec/check.sh` after the page commit `512878b` (load 16.7 at start, up to 22): **all checks passed**, exit 0, 09:13:50 → 09:26:44 (about 13 min), first run, no flaky test; step 10 `docscheck: 9 pages ok`.

## Addendum 1 (2026-11-16) – PO: the guides belong also in the Community section

**PO (2026-11-16, after the C-100 handover), quoted:** "Kurze Frage, wo schreibst du die Anleitungen, nur in die Readme? Da gehört sie auch hin, sie gehört final aber auch in den Community Teil der oec Docusaurus."

**Answer given:** the guides are not only in the READMEs. They are pages of the standalone site `oec/docs` (ADR-0012, ADR-0031, gate step 10); the READMEs stay the source of details and are linked, not copied.

**New requirement (scope and publication, G-3.4 a):** the documentation shall in the end also be part of the Community section of the guideline Docusaurus site (`oi4-oec-documentation/docs/community`, navbar "Community"). Today that section holds the 1.x material: the development edge setup guide, the TypeScript service library `oi4-oec-service` and `oi4-oec-demos`. This touches G-4.11/ADR-0012 (standalone site, no links to the guideline site), Q-011 (publication) and the guideline repository, outside `findings/`. The PO was asked how; no answer yet. It is recorded as **Q-036** and **D-1 (C-100 addendum 1)**; the agent does not decide it.

**Proposal (a):** `oec/docs` stays the gate-checked source. A later P8 increment, after C-103 and before the P8 exit, ports the pages into `docs/community` of the guideline site:
- the pages go into their own subfolder, e.g. `docs/community/oec-v2/`;
- references to guideline sections may become real links there, an exception to G-4.11 for that copy only;
- links into `oec/` remain repository URLs, as in the remark plugin;
- the 1.x pages are kept and marked as 1.x, or replaced, as the PO decides;
- a check prevents the two copies from drifting apart.

**Alternatives:**
- (b) from C-101 on, write directly in `docs/community` and retire `oec/docs`; ADR-0012/0031 and gate step 10 change;
- (c) maintain both copies by hand.

**Effect on C-101:** none. The pages are plain Markdown either way. C-101 keeps them portable (relative links, no site-specific components) so that (a) or (b) needs no rewrite.

## Addendum 2 (2026-11-16) – the pages move into the Community section

**PO answer to D-1 (C-100 addendum 1), quoted:** "die alte oi4-oec-documentation/docs/community benötigen wir nicht mehr. Schreibe alles was unter oec/docs ist direkt in oi4-oec-documentation/docs/community. Verweise in den readmes auch gerne auf die oi4-oec-documentation/docs/community. Baue jetzt alle oec-docs soweit um, dass sie in /Users/thomas/work/github/oi4/oi4-oec/oi4-oec/oi4-oec-documentation sind"

This is option (b), with the old Community pages removed. Q-036 is decided. The implementation follows below.

### What was done

1. **Recorded first** (`6ce98ce`): log quote, Q-036 decided (b), D-1 answered in REVIEW/REVIEW_DE, STATUS.
2. **docscheck v2, tests first** (`oec/docs/tools`, standard library only): the new test cases were written first, then `Check(docs, oec)` and `CheckReadmes(oec, docs)` were rewritten:
   - pages of `docs/community`: `title` and `description`, `sidebar_position` an integer ≥ 0, no `slug` and no `id` (the route follows the file), a level-1 heading, `index.md` at the root;
   - relative links stay inside the section and name a file (not a folder), anchors exist;
   - `https://github.com/OI4/oi4-oec/blob/main/oec/<path>#<anchor>` is mapped to the workspace `oec/` and checked for the file and the anchor; any other `github.com/OI4/oi4-oec/` URL, site-absolute links, other schemes and queries are errors;
   - G-4.11: no link to `oi4.github.io` or `oi4-oec-documentation` – guideline sections by number;
   - MDX rule: `{`, `}` and `<` followed by a letter, `/` or `!` outside code are errors (the guideline site reads `.md` as MDX);
   - README check: every link of `oec/**/README.md` (without `node_modules`, `target`, `build`, `dist`, `.docusaurus`, `.git`, `managed_components`) to `https://oi4.github.io/oi4-oec-documentation/docs/community/<route>` needs `<route>.md` or `<route>/index.md` and the anchor; no trailing slash, no `.md`; a relative link to the removed `docs/docs/` is an error. The test of the real READMEs requires at least five links.
   - `cmd/docscheck`: flags `-docs`, `-oec`; output `docscheck: 9 pages ok, 10 README links to them ok`.
3. **Guideline repository** (`oi4-oec-documentation`, branch `feature/spec-findings`, commit `cbc293a`, not pushed):
   - removed the 1.x Community pages (`development-edge-setup-guide.md`, `oi4-oec-service/` with `usage/`, `oi4-oec-demos/`) and `static/download/oi4-oec-demo.zip` (only the 1.x tutorial linked it); `_category_.json` (Community, position 4) and the badges in `static/img` kept;
   - moved the nine pages in: `intro.md` → `index.md` (front matter without `slug`, the `:::note` community disclaimer of the old index kept, "this site" → "this section"), `concepts.md`, `migration.md`, `demos/{index,logger,weather,config}.md`, `registry/index.md`, `micro-sdk/index.md`;
   - 49 relative links into `oec/` rewritten to repository URLs; page-to-page links stay relative;
   - `src/pages/index.js` (home page) and the footer in `docusaurus.config.js` link Getting started, Concepts and Migration notes instead of the removed tutorial; the rest of the configuration (`onBrokenLinks: 'warn'`, sidebars) unchanged;
   - `.yarn/install-state.gz` (changed by `yarn install`) not committed.
4. **Workspace** (`430d505`): `oec/docs/docs/**`, `package.json`, `package-lock.json`, `.npmrc`, `docusaurus.config.mjs`, `sidebars.mjs`, `src/` and `tools/npmpolicy/` removed (and the git-ignored `node_modules/`, `build/`, `.docusaurus/` deleted); `check.sh`, `build.sh`, `README.md` of `oec/docs` rewritten; the header comment of gate step 10 in `oec/check.sh`; links to the Community pages in `oec/README.md`, `oec/demos/README.md`, `oec/registry/README.md`, `oec/sdk/micro/README.md` and `oec/docs/README.md`.
5. **Paperwork:** GUARDRAILS G-4.11 amended, ADR-0012 amendment, ADR-0031 amendment C-100 addendum 2, ROADMAP P8, STATUS, AGENTS.md (guideline branch head), REVIEW/REVIEW_DE (A-506 … A-511).

### Results

- `oec/docs/check.sh`: gofmt, vet, tests ok; `docscheck: 9 pages ok, 10 README links to them ok`.
- Opt-in `oec/docs/build.sh` (existing `node_modules/` of the guideline site): about 18 s, the whole guideline site built, **0 warnings**, every Community page has its HTML page (`build/docs/community.html`, `build/docs/community/{concepts,migration}.html`, `…/demos.html`, `…/demos/{logger,weather,config}.html`, `…/registry.html`, `…/micro-sdk.html`); sidebar order Getting started, Concepts, Demo services, OEC Registry, OEC Micro SDK, Migration notes. An earlier full build with a fresh `yarn install --immutable`: about 34 s, 0 warnings.
- Probe: a link to a missing page appended to `concepts.md` – docscheck and the warning filter of `build.sh` both caught it; the file was restored (`git diff` empty).

### Agent decisions (G-3.4 b)

A-506 location and removals, A-507 repository URLs instead of the remark plugin, A-508 docscheck v2 (MDX rule, README check), A-509 no own npm tree (npm policy removed; C-098 D-1 (a) has no object), A-510 build of the guideline site with its yarn, A-511 README links and G-4.11 wording. Details in REVIEW.md and ADR-0031 amendment C-100 addendum 2.

### Decision for the PO

None. No guideline gap found: no SF, no D-x, no R-x.

### Docker

Not used.

### Temporary files

`/tmp/concepts.bak`, `/tmp/probe.log`, `/tmp/guide-build.log` (probe and build) and `/tmp/gate.log` – deleted. The build output `oi4-oec-documentation/build/` and `.docusaurus/` are git-ignored there.

### Gate

`caffeinate -i oec/check.sh` on the code commit `430d505` (load 10.1 at the start): about 13 min, **all checks passed**; step 10 `docscheck: 9 pages ok, 10 README links to them ok`. No flaky test repeated.

### Handover (G-3.10)

C-101 (P8 increment 4: SDK guides Rust, C, Go and the API reference) starts in a new session: the context of C-100 is large, and the pages of C-101 need fresh reads of the SDK READMEs. `prompt.md` (not committed) is updated: pages go into `oi4-oec-documentation/docs/community`, links into `oec/` as repository URLs, MDX-safe text; next free ids SF-118, A-512, ADR-0032, Q-037.
