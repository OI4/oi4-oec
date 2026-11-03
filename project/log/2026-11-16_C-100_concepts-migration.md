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
