# ADR-0001 – Authoritative guideline source for v2.0

- Status: accepted
- Date: 2026-10-12
- Deciders: product owner (Thomas), confirmed in C-001

## Context

Two copies of the OEC Development Guideline exist in the workspace:

1. `oi4-oec-documentation/guideline/Members Space/.../OEC - Development Guideline.md` – Confluence export (plus PDFs v1.1.0).
2. `oi4-oec-documentation/docs/guideline/` – the Docusaurus version imported from the Confluence export (`run_import2.js`) and edited since then. It already contains RFC MS-010 (Identification Link replaces Oi4Identifier, base64url-encoded single topic level) – commit `c85ce2a` (2026-10-06).

## Decision

`oi4-oec-documentation/docs/guideline/` (Docusaurus) together with accepted RFCs in `oi4-oec-documentation/rfcs_adrs/` is the single source of truth for v2.0. The Confluence export and PDFs are historical references only.

## Consequences

- All requirements, schemas and code reference sections of the Docusaurus guideline.
- The guideline revision (git commit) used is recorded in each cycle log; changes are diffed against the recorded baseline.
- Contradictions inside the Docusaurus guideline are handled via the spec-findings register (since C-018 only `oi4-oec-documentation/findings/SPEC-FINDINGS.md`, see amendment C-018).

## Alternatives considered

- Confluence export as source – rejected: older, lacks MS-010.
- Both sources with case-by-case decision – rejected: unnecessary ambiguity.

## Amendment C-006 (2026-10-14, guideline baseline `5f070bb`)

The guideline was split (GL-ADR-011) and all guideline ADRs (GL-ADR-001 … 011, `oi4-oec-documentation/rfcs_adrs/`, formerly "MS-0xx" RFCs) were implemented. The source of truth is now:

- **OEC Messaging Guideline** – `oi4-oec-documentation/docs/guideline/` (cited "MG x.y")
- **OEC Runtime & Infrastructure Guideline** – `oi4-oec-documentation/docs/infrastructure-guideline/` (cited "RIG x.y")
- **Guideline ADRs** – `oi4-oec-documentation/rfcs_adrs/ADR-0xx_*.md` (cited "GL-ADR-0xx"); their *Implementation Notes* are binding interpretations.

To avoid confusion with our own project ADRs (`ADR-NNNN`), guideline ADRs are always prefixed `GL-`.

## Amendment C-008 (2026-10-14, guideline baseline `7bead4c`)

- The baseline is now `7bead4c` on branch `feature/spec-findings` of `oi4-oec-documentation`. It adds **GL-ADR-012** (only the common service `NewDataSetWriterId` remains) and **GL-ADR-013** (NodeIds as JSON strings, OPC UA V1.05), and resolves all spec findings SF-001 … 031.
- The guideline repo now has its own copy of our findings register, `findings/SPEC-FINDINGS.md`, with the same SF ids. Our `project/SPEC-FINDINGS.md` remains the place where new findings are first recorded.
- Until the branch is merged, we track the branch commit. When it is merged, the merge commit on the main branch becomes the baseline; the content is identical.

## Amendment C-013 (2026-10-14, guideline baseline `49b6fe2`)

- The baseline is now `49b6fe2` on branch `feature/spec-findings`. It resolves SF-032 … 034 and adds:
  - **MG 1.2 "Normative Keywords"** (BCP 14 / RFC 2119 + RFC 8174; RIG 1.1 applies it to the RIG): requirement strength is taken from the keyword, statements without keyword are informative (see G-1.6 and SF-035).
  - **GL-ADR-014** – the specific services (Read, Write, Subscribe, Unsubscribe, GenericMethod) are removed.
  - **GL-ADR-015** – the former MG chapter 6 (process description) moved to the separate **OEC Process Description** (`docs/process-description/`); MG chapters 7–11 were renumbered to 6–10.
  - **GL-ADR-016** – the empty AAS mapping chapter is removed.
- The **OEC Process Description is informative**. It is not part of the normative baseline; no requirements are derived from it.
- The external document "Requirements for Interoperability" is no longer referenced (SF-034). The normative baseline is MG + RIG + GL-ADRs only.
- **Section numbers:** ADRs, logs and spec findings written before C-013 cite the old MG numbering (old MG n = new MG n−1 for n = 7…10). They are not rewritten; living documents (GUARDRAILS, STATUS, ROADMAP, digest) use the current numbering.

## Amendment C-015 (2026-10-15, guideline baseline `5cb109e`)

- The baseline is now `5cb109e` on branch `feature/spec-findings`. The PO had the agent implement the decided findings directly in the guideline repo (local commits, never pushed):
  - `609cb4a` – **SF-038**: the OEC Messaging Compliance and the OEC Service Container Compliance are **independent** domains (MG 1.3, RIG 1.2, RIG 5.2, glossaries, GL-ADR-009 note). A microcontroller can reach Messaging Level 3 without any Service Container level; a container can reach Service Container Level 3 with Messaging Level 1.
  - `e0b1676` – **SF-037**: Messaging **Level 3 = all methods, resources and common services**; "Resources Without Content" (empty list on `Get`, no unsolicited `Pub`, `Event` Status `Bad` on `Set`/`Del` of unknown objects); every `Set`/`Del` is answered by `Event` Status (MG 7.1.4, 7.2.x, 9.1, GL-ADR-009 note).
  - `0106c5f` – **SF-036**: the set of resources and common services is closed (MG 7.1.5, GL-ADR-014 note).
  - `5cb109e` – the guideline copy `findings/SPEC-FINDINGS.md` is synced with our register.
- Changes the agent makes in the guideline repo on behalf of the PO are part of the baseline like any other guideline commit. Each one references its SF id; the PO reviews them before the branch is merged.

## Amendment C-016 (2026-10-15, guideline baseline `de852d6`)

- The baseline is now `de852d6` on branch `feature/spec-findings`. Again implemented by the agent on PO request (local commits):
  - `a6147e7` – SF-037 amendment (`Event` `Status` answer to `Set`/`Del`: must from Level 2 on, should at Level 1) and **SF-039** (`Get` of `Metadata` without `Data`: no `Metadata`, may answer with `Event` `Status`/`Bad`).
  - `0de223c`, `f830382` – **SF-035**: keywords added to all keyword-less protocol statements of the normative-sentence inventory (and `ReplyTo` in MG 8.2.1). The interim rule "keyword-less behavior = implicit MUST" is no longer needed for the guideline text; keyword-less behavior found later is a new spec finding.
  - `edf76e1`, `de852d6` – findings register sync. No open findings.

## Amendment C-017 (2026-10-15, guideline baseline `bc5d063`)

- The baseline is now `bc5d063` on branch `feature/spec-findings`. Implemented by the agent on PO request (local commits):
  - `f5d69d8` – **GL-ADR-017** "Adapt minimum set of topic elements to compliance levels": MG 7.2 (7.2.1 For Applications, 7.2.2 For Devices) removed. The requirements per level follow from MG 1.3; the rule for directly connected and represented entities is a new section of MG 1.3; "Resources Without Content" moved unchanged to MG 9.1. No requirement of a level changed.
  - `6e1b5c3` – findings register sync.
  - `7677161` – GL-ADR-017 extended (PO request): chapter MG 7 flattened – the content of former 7.1 Topic Namespace Elements is the section "Topic Elements" of MG 7; the topic elements MG 7.1.1 … 7.1.7 are now **MG 7.1 … 7.7** (7.1.5.1/7.1.5.2 → 7.5.1/7.5.2). Note: "MG 7.2" now denotes the ServiceType Element.
  - `9489d5e` – findings register sync.
  - `4d2f919` – MG 7 introduction: generic MQTT topic rules replaced by a link to MQTT v3.1.1 section 4.7 (OASIS), only OEC-specific rules kept; ADR-017 implementation note.
  - `4f5c548` – findings register sync.
  - `10491df` – MG 7 MQTT introduction removed entirely (topic fully defined by schema and 7.1 … 7.7); MQTT link moved to "Topic Elements"; white-space recommendation dropped.
  - `2d6dc59` – findings register sync.
  - `d7d8750` – PO: heading "Topic Elements" removed from the MG 7 index.
  - `edf9305` – dangling `#topic-elements` anchor in MG 1.3 fixed; ADR-017 aligned.
  - `2452494` – findings register sync.
  - `cb50c04` – PO: "White space is allowed, but should not be used." (same keyword as the former MG 7 intro) moved to MG 7.7 `<Topic encoded String>`.
  - `bc5d063` – findings register sync.
- References to MG 7.x in project artifacts written before C-017 (logs, findings, ADRs) use the old numbers and are historical; the REQ catalog, GUARDRAILS, digest, ROADMAP and STATUS were renumbered (C-017).


## Amendment C-018 – one findings register

PO decision (2026-10-15): spec findings are kept **only** in the guideline repo, `oi4-oec-documentation/findings/SPEC-FINDINGS.md`. The project register `project/SPEC-FINDINGS.md` was merged into it (SF-040 … 043 added in guideline commit `d84c642`) and deleted; the statements of amendment C-008 ("Our `project/SPEC-FINDINGS.md` remains the place where new findings are first recorded") and the later "findings register sync" commits are superseded. Commits that only change `findings/` do not change the guideline text and therefore need no re-baseline: the spec baseline stays `bc5d063` (guideline HEAD `d84c642` differs only in `findings/SPEC-FINDINGS.md`).

## Amendment C-019 (2026-10-16, guideline baseline `ea80271`)

- The baseline is now `ea80271` on branch `feature/spec-findings`. Implemented by the agent on PO request (local commits):
  - `7de6cb8` – SF-043 (e): the dnp-encoding stays only as an optional hint at the `Oi4Identifier` (MG 4.3.1 "may be used" for characters not permitted in an Identification Link or a `/`; explained in MG A3) and is removed from MG 4.1 and the structure/field types of MG 4.3.1; MG A3 and the glossary MG A4 aligned; GL-ADR-010 implementation note.
  - `ea80271` – findings register: SF-043 (e) resolved; (a)–(d) stay open.
- Unlike `d84c642`, `7de6cb8` changes guideline text, so the spec was re-baselined (inventory keys remapped, REQ-IL-033 added).

## Amendment C-020 (2026-10-17, guideline baseline `5dedfd5`)

- The baseline is now `5dedfd5` on branch `feature/spec-findings`. Implemented by the agent on PO request (local commits):
  - `6476bd3` – SF-043 (a)–(d): MG A3 (general UTF-8 rule, ASCII characters in Table 4, "comma character", lower-case conversion), MG 4.3.1 and MG 5 (`ManufacturerUri` form; it should correspond to the domain of the asset's Identification Link, `<manufacturer>.<tld>` in case of doubt – PO addition).
  - `d00cce7` – SF-040: MG 7.7 (upper-case hex digits, `%` only in four sequences, no empty Filter level), Appendix B and MG 8.3.3.x without `%20`, `Status/Good`.
  - `a992f9a` – SF-042: closed `ServiceType` and `EventCategory` sets, free `GenericLevel`; the OTConnector-device sentence removed from MG 7.2 (PO: REQ-TOP-007 withdrawn).
  - `4c27c37` – findings register: SF-040, SF-042, SF-043 resolved.
  - `39be075` – MG 4.3.1: the `ManufacturerUri` form keeps its "must".
  - `5dedfd5` – findings register sync.
- SF-041 is decided but its guideline text is not changed yet (not requested in C-020).

## Amendment C-021 (2026-10-17, guideline baseline `2cf0d70`)

- The baseline is now `2cf0d70` on branch `feature/spec-findings`. Implemented by the agent on PO request (local commits):
  - `61a3182` – SF-041 (a)–(c): MG 7 and MG 9 (only the topic forms of MG 9.1.x/9.2.x are allowed), MG 9.1.5 (`Pub` `Metadata` with `Source` and `Filter`), MG 7.4, MG 8.2.16, MG 9.2.1 (`Call`/`Reply` with optional `Source`, no `Filter`).
  - `d4a7560` – SF-041 (d)–(g): MG 7.7 (`<ResourceType>`/`<Tag>` of `PublicationList`/`SubscriptionList`), notes in MG 8.3.8, 9.1.8, 9.1.9, MG B1.1.8 (Set example `…/Data/oee`, valid `Mode`), B1.1.8/B1.1.9 templates, typo in MG 9.1.10.
  - `2cf0d70` – findings register: SF-041 resolved, new SF-044 (open).
- All spec findings SF-001 … 043 are resolved; SF-044 is open with a proposed interpretation.

## Amendment C-022 (2026-10-17, guideline baseline `fd9da6e`)

- The baseline is now `fd9da6e` on branch `feature/spec-findings`. Implemented by the agent on PO request (local commits):
  - `d198e6b` – GL-ADR-018 "Use complete Filter as WriterGroupName of list entries" (SF-044, PO decision C-022: the proposal is accepted and recorded in a guideline ADR): MG 8.2.3 (new rule), MG 9.1.8 and 9.1.9 (templates, examples), MG B1.1.8, B2.6, B2.7.
  - `fd9da6e` – findings register: SF-044 resolved.
- GL-ADR-018 is part of the baseline (G-1.1). All spec findings SF-001 … 044 are resolved; no finding is open.

## Amendment C-024 (2026-10-19, guideline baseline `2f00e9f`)

- The baseline is now `2f00e9f` on branch `feature/spec-findings` (first `a7d18f9`, then `4af61f3` after the second and `2f00e9f` after the third PO review below). Implemented by the agent on PO request (PO review of C-023: SF-045 (b') and SF-046 … 051 accepted, tolerant reader, A2 reserved ids; local commits):
  - `7943098` – Docusaurus config: `markdown.hooks.onBrokenMarkdownLinks` instead of the deprecated `siteConfig.onBrokenMarkdownLinks` (no guideline text change).
  - `590d2be` SF-045, `8e3939f` SF-046, `6950e74` SF-047, `dd43a7a` SF-048, `4a20b99` SF-049, `4d250a2` SF-050, `65e20e6` SF-051 – MG 8, 8.1.2, 8.2.x, 9.2, A2, B1.1.x, B2.x.
  - `6f39017` – SF-052 (new, PO decision): MG 8 – a sender must not extend the objects by user defined keys, a receiver must ignore keys it does not know or use (tolerant reader).
  - `8eddc0d` – SF-053 (new, PO request): MG A2 lists the former use of the reserved `DataSetClassIds`.
  - `a7d18f9` – findings register: SF-045 … SF-053 resolved.
  - Second PO review (2026-10-19): `4b97a92` – SF-052 amendment (MG 8: a receiver *should* ignore unknown keys; note on strict and tolerant validation); `7fccd53` – SF-054 (MG B2 metadata examples rebuilt with recurring values, MG 8.3.2 note); `4af61f3` – findings register sorted ascending, SF-054 resolved, SF-055 open.
  - Third PO review (2026-10-19): `db2e5d3` – MG B1.1.5 with a complete `Metadata` example (SF-054 amendment); `2f00e9f` – findings register: SF-055 resolved as proposed, SF-056 (B1 example defects) open for C-026/C-027.
- No new GL-ADR (PO decision in the second review: SF-045 and SF-052 are not recorded as GL-ADRs). SF-001 … 055 are resolved; SF-056 (defects of the B1 examples) is open for C-026/C-027 and not blocking; next free id SF-057.

## Amendment C-026 (2026-10-19, guideline baseline `c6453c7`)

- The baseline is now `c6453c7` on branch `feature/spec-findings` (first `720d862`, then `c6453c7` after the PO review below). Implemented by the agent on PO request (PO review of C-025: SF-057 … 060 accepted as proposed, SF-058 (a) with the creation timestamp in the Will message, SF-059 (a) without reserving `FAILURE_1`/0; local commits):
  - `d88a86c` – SF-057: normative keywords in MG 6.2.1, 6.2.2, 6.3.1, 6.3.3, 6.3.4, 6.3.5.
  - `2dd0e56` – SF-058: MG 6.3.5 (the `MessageId` of the Will message carries the timestamp of its creation), MG B1.1.1/B1.1.2 (real timestamps, notes, "Close message").
  - `9cd7ca0` – SF-059: MG 6.3.3 … 6.3.5 (`NORMAL_0`/0 only as Close message, `FAILURE_1`/0 also while connected with a new Birth message after recovery, subscriber scope, Birth message first).
  - `1d80d20` – SF-060: MG 6.1 (protocol versions), MG 6.2.2 (broker certificate verification, usernames assigned by the platform), MG 6.2.3 (ClientId without container).
  - `720d862` – findings register: SF-057 … SF-060 resolved.
- No new GL-ADR (PO decision: SF-057 needs no ADR). SF-001 … 055 and SF-057 … 060 are resolved; SF-056 (defects of the B1 examples) is open for C-027/C-028 and not blocking; next free id SF-061.
- PO review of C-026 (2026-10-19): `2c38cf8` – SF-061 (a) (MG 6.3.3, B1.1.1: the Birth message may carry `Timestamp`/`SequenceNumber`); `02a35ac` – SF-061 (b) (MG 6.3.4 Close message "must", MG 6 note and new MG 1.3 row: recommended for Level 1, mandatory from Level 2 on); `47762da` – SF-061 (c) (MG 6.3.4, 6.3.5: subscribers "should" interpret the Close/Will values); `c6453c7` – findings register: SF-061 resolved, next free id SF-062. No GL-ADR.

## Amendment C-027 (2026-10-19, guideline baseline `ca8d1c5`)

- The baseline is now `ca8d1c5` on branch `feature/spec-findings`. Implemented by the agent on PO request (PO review of C-027: SF-056, SF-065 … 068 accepted as proposed; SF-062 … 064 accepted after revision – REST-like paging, no pagination for `Set`/`Del`, MaxPacketSize, OPC UA check of the MAM rules; the PO ordered the implementation and a new GL-ADR; local commits):
  - `2fbde83`, `9d1b00e` – SF-064: MG 5 (VendorNameplate "as far as the rules are satisfied", `ManufacturerUri` domain even for GUIDs, `Model` en-US for the `Oi4Identifier`, revisions SemVer or four numbers, OPC UA "" → "undefined" for `HardwareRevision`/`SoftwareRevision`, `DeviceManual` URL with scheme / "" / no local path, `RevisionCounter` on configuration changes or -1, `GlobalAssetId` should for assets with an AAS); the follow-up withdraws a `DeviceRevision` rule beyond the finding.
  - `a79e32a` – SF-065: MG 8.3.2 ("must reflect", explicit severity order).
  - `acf8b3a` – SF-066: MG 8.3.7, 9.1.7, B1.1.7 (resources and common services, each name once).
  - `1e53e71` – SF-067: MG 8.2.3 (PublicationList/SubscriptionList share one `DataSetWriterId`), 8.3.8, 9.1.8, Status topic `Status/<EventLevel>` in 9.1.3/4/5/8/9/10, B1.1.8.
  - `8ee0fdd` – SF-068: MG 8.3.10, 9.1.10, B1.1.10.
  - `fbc0ce8` – SF-062: MG 8.3.11 (MaxPacketSize RIG 4.1.1, `PaginationRequest` only in `Get`, `Pagination` only in `Pub`, no pagination for `Set`/`Del`), 8.3.12 (Locale only with `Get`), `Get`/`Pub` list items of 9.1.1/2/3/4/7/8/9/10.
  - `8ac06fc` – SF-063: MG 8.3.11.1/8.3.11.2, B1.1.11 (page size, pages from 1, `Page` 0 = all pages, `HasNext`, series).
  - `f94916e` – SF-056: B1 examples (valid JSON, notes instead of comment-only objects, instantiated values).
  - `26c899d` – new GL-ADR-019 "Clarify core resources, pagination and locale" (PO order), describing all changes above.
  - `ca8d1c5` – findings register: SF-056 and SF-062 … 068 resolved, no open finding, next free id SF-069.
- GL-ADR-001 … 019; SF-001 … 068 resolved, no open spec finding; next free id SF-069.

## Amendment C-027 review (2026-10-20, guideline baseline `2bfc0d9`)

- The baseline is now `2bfc0d9` on branch `feature/spec-findings` (PO order in the review of the C-027 views: replace `Oi4Identifier` in the figures by the Identification Link; local commits):
  - `f8d79f4` – figures only: 54 topic placeholders `<Oi4Identifier>` → `<Source>` in the sequence diagrams of MG 1.2 and 9.1.1 … 9.1.10, "generate Oi4Identifier" → "determine Identification Link" in the onboarding diagrams of the Process Description; 12 PNGs re-exported (SF-070). No markdown changed – the inventory only changes its baseline line.
  - `2bfc0d9` – findings register: SF-069 (ReferenceDesignation children) and SF-071 (remaining topic-label deviations of the figures) open, SF-070 resolved, next free id SF-072.
- Figure-only commits change the baseline like text commits (the figures are part of the guideline) but need no requirement remapping; `spectool inventory -check -rev <commit>` confirms it.
- GL-ADR-001 … 019; SF-001 … 068 and SF-070 resolved, SF-069 and SF-071 open (not blocking); next free id SF-072.

## Amendment C-027 review, part 2 (2026-10-20, guideline baseline `70b4c01`)

- The baseline is now `70b4c01` on branch `feature/spec-findings` (PO decisions SF-069 option (b) with a separate GL-ADR and SF-071 as proposed; local commits):
  - `ec76e41` – new MG 8.3.13 `ChildrenRequest` (helper `DataSet`, reserved `DataSetWriterId` 9 in 8.2.3), note in 8.3.10, section in 9.1.10, grading in 1.3 (L1/L2 optional, L3 mandatory), examples in B1.1.10 (SF-069).
  - `8fb837e` – GL-ADR-020 "Request the children of a ReferenceDesignation".
  - `655e97f` – figures: `Oi4v2/` prefix, `Status/<EventLevel>`, typos, Figure 9 of MG 9.1 redrawn as a v2 example; 15 PNGs re-exported (SF-071).
  - `70b4c01` – findings register: SF-069 and SF-071 resolved, SF-072 (B1.1.4 `DataSetClassId`) open, next free id SF-073.
- GL-ADR-001 … 020; SF-001 … 071 resolved, SF-072 open (not blocking); next free id SF-073.

## Amendment C-027 review, part 3 (2026-10-20, guideline baseline `15eaa8a`)

- The baseline is now `15eaa8a` on branch `feature/spec-findings` (PO decisions SF-073 option (c) directly in GL-ADR-020 and SF-072 as proposed; local commits):
  - `dd2628c` – findings register: SF-072 decided, SF-073 open (register only).
  - `30fa928` – new MG 8.3.13 `Query` (generic helper `DataSet`, reserved `DataSetWriterId` 9) and 8.3.13.1 Query of ReferenceDesignation (replaces `ChildrenRequest`); 8.2.3, 8.3.10, 1.3, 9.1.10 and B1.1.10 updated (SF-073).
  - `c2619e6` – GL-ADR-020 amended in place ("Request the children of a ReferenceDesignation with a generic Query").
  - `264fc2c` – B1.1.4: publisher-defined `DataSetClassId` in the `Pub` examples, note rewritten (SF-072).
  - `15eaa8a` – findings register: SF-072 and SF-073 resolved, SF-074 (B1.1.5 `DataSetClassId`) open, next free id SF-075.
- GL-ADR-001 … 020; SF-001 … 073 resolved, SF-074 open (not blocking); next free id SF-075.

## Amendment C-027 review, part 4 (2026-10-20, guideline baseline `ff1562f`)

- The baseline is now `ff1562f` on branch `feature/spec-findings` (PO decision SF-074 as proposed; local commits):
  - `b45c497` – B1.1.5: publisher-defined `DataSetClassId` in the `MetaData` of the `Pub`/`Set` examples, sentence rewritten (SF-074).
  - `ff1562f` – findings register: SF-074 resolved, no open findings, next free id SF-075.
- GL-ADR-001 … 020; SF-001 … 074 resolved, none open; next free id SF-075.

## Amendment C-028 review (2026-10-21, guideline baseline `b2f0fe7`)

- The baseline is now `b2f0fe7` on branch `feature/spec-findings` (PO decisions SF-075 … SF-081 as proposed; local commits):
  - `dd064fe` SF-075 (Event payload structure), `60d20f8` SF-076 (Event level mappings), `491a028` SF-077 (`Data`/`Oi4Pv`), `135ce3a` SF-078 (`Data`/`Metadata` templates), `81d67d7` SF-079 (`Set`/`Del` header), `2656353` SF-080 (`SubscriptionList`), `21c563a` SF-081 (`Filter` in `Set`/`Del`).
  - `b2f0fe7` – findings register: SF-075 … 081 resolved, no open findings, next free id SF-082.
- No new GL-ADR (none ordered by the PO).
- GL-ADR-001 … 020; SF-001 … 081 resolved, none open; next free id SF-082.

## Amendment SF-082 (2026-10-21, guideline baseline `c2be26b`)

- The baseline is now `c2be26b` on branch `feature/spec-findings` (local commits, PO order without review):
  - `2d9cfb5` SF-082 – MG/RIG lists of figures/tables list the captions with links; MG Figure 1 image restored; A2/A5 captions restored, MG Tables 4–8 renumbered to 7–11 (editorial, no normative change).
  - `c2be26b` – findings register: SF-082 resolved, no open findings, next free id SF-083.
- No new GL-ADR.
- GL-ADR-001 … 020; SF-001 … 082 resolved, none open; next free id SF-083.

## Amendment SF-083 (2026-10-21, guideline baseline `5bc2e85`)

- The baseline is now `5bc2e85` on branch `feature/spec-findings` (local commits, PO order without review):
  - `b81d98e` SF-083 – every figure and table of MG, RIG and PD captioned and numbered in reading order (MG 22/23, RIG 2/11, PD 8 figures); MG Tables renumbered (old 3 → 13, old 7/8 → 18/19 among others), MG Figures +1; conversion defects fixed (`&lt;br/&gt;`, empty headers, A6 dead links, B1.1.3 JSON-in-table, PD 2.2 image); lists regenerated (editorial, no normative change).
  - `5bc2e85` – findings register: SF-083 resolved, no open findings, next free id SF-084.
- Table and figure numbers cited in records older than this amendment are the numbers valid at that time.
- No new GL-ADR.
- GL-ADR-001 … 020; SF-001 … 083 resolved, none open; next free id SF-084.

## Amendment GL-ADR-022 (2026-10-22, guideline baseline `6cb6a97`)

- The previous baseline `a77d331` (C-029 review: SF-084 … 088 with GL-ADR-021) is recorded in the C-029 log, addendum 2.
- The baseline is now `6cb6a97` on branch `feature/spec-findings` (local commits, PO order 2026-10-22):
  - `4169156` GL-ADR-022 – secrets of the identity of an application in the secret storage `/run/secrets/oi4` (`credentials`, `private_key.pem`, `passphrase`); naming option chosen by the agent after a neutral evaluation, for PO confirmation.
  - `31f0890` – RIG 4.1.3 rewritten; wording in RIG 4.1.2, 4.2 and 3 ("certificate of the application"); resolves SF-091 (d).
  - `6cb6a97` – findings register: SF-091 (d) resolved, SF-089 … 093 otherwise open, next free id SF-094.
- GL-ADR-001 … 022; SF-001 … 088 resolved, SF-089 … 093 open; next free id SF-094.

## Amendment C-030 closure (2026-10-22, guideline baseline `b942df3`)

- The previous baseline `6cb6a97` (GL-ADR-022) is recorded in the C-030 log, addendum 1.
- The baseline is now `b942df3` on branch `feature/spec-findings` (local commits, PO order 2026-10-22):
  - `f4b2cd8` – findings register: SF-091 decided (revised proposal).
  - `15115e8` SF-093 (MG 9.2 ACL box for Action calls), `527d8d8` SF-090 (RIG 4.2 container name), `e7f5459` SF-092 (RIG 5 keywords, RIG 1.2), `897c1cc` SF-089 (MG A1 Registry).
  - `b942df3` – findings register: SF-089/090/092/093 resolved.
- No new GL-ADR. GL-ADR-022 accepted by the PO.
- GL-ADR-001 … 022; SF-001 … 090, 092, 093 resolved; SF-091 decided, not yet implemented; next free id SF-094.

## Amendment C-032 (2026-10-22, guideline baseline `5fb0fd8`)

- The previous baseline `b942df3` (C-030 closure) is recorded in the C-030 log, addendum 3, and in the C-031 log.
- The baseline is now `5fb0fd8` on branch `feature/spec-findings` (local commits, PO order 2026-10-22):
  - `fbe6173` SF-091 (RIG 4.1.1 conditional `broker_ca.pem` and MaxPacketSize ≤ 268435455, RIG 4.1.2 CA certificate "shall", RIG 4.1.4 table/note, RIG 1.2 note on "Mandatory", MG 6.2.2 CA-signed broker certificate "must", MG 1.2 note).
  - `4036205` SF-094 (MG B2 DataSet examples = the referenced B1.1 payloads), `e25f64f` SF-095 (code block tags, MG 1.2 DataSetWriterId 3456).
  - `5fb0fd8` – findings register: SF-091/094/095 resolved, no open finding.
- No new GL-ADR. GL-ADR-001 … 022; SF-001 … 095 resolved; next free id SF-096. This is the baseline of the P1 exit.

## Amendment C-032 addendum (2026-10-22, guideline baseline `46ffbb1`)

- The previous baseline `5fb0fd8` (P1 exit) is recorded in the C-032 log.
- The baseline is now `46ffbb1` on branch `feature/spec-findings` (local commit, PO order 2026-10-22): MG 7.7 headings only – the "Resource: …" labels are H2 headings, the syslog levels H4 below `<LogLevel>`; no content change.
- No new GL-ADR, no finding. GL-ADR-001 … 022; SF-001 … 095 resolved; next free id SF-096.

## Amendment C-032 addendum 2 (2026-10-22, guideline baseline `dcaeb04`)

- The previous baseline `46ffbb1` is recorded in the amendment above.
- The baseline is now `dcaeb04` on branch `feature/spec-findings` (local commit, PO order 2026-10-22): MG 8.2.3 plain-text references to MG 7.6/7.7 turned into links; no wording change, no finding, no GL-ADR. SF-001 … 095 resolved; next free id SF-096.

## Amendment C-032 addendum 3 (2026-10-22, guideline baseline `e36e762`)

- The previous baseline `dcaeb04` is recorded in the amendment above.
- The baseline is now `e36e762` on branch `feature/spec-findings` (local commit, PO order 2026-10-22): MG 9.1.8 plain-text references turned into links; no wording change, no finding, no GL-ADR. SF-001 … 095 resolved; next free id SF-096.

## Amendment C-032 addendum 4 (2026-10-22, guideline baseline `e0f1488`)

- The previous baseline `e36e762` is recorded in the amendment above.
- The baseline is now `e0f1488` on branch `feature/spec-findings` (local commit, PO order 2026-10-22): RIG 4.1.3 plain-text references turned into links; no wording change, no finding, no GL-ADR. SF-001 … 095 resolved; next free id SF-096.

## Amendment C-032 addendum 5 (2026-10-22, guideline baseline `4e15e02`)

- The previous baseline `e0f1488` is recorded in the amendment above.
- The baseline is now `4e15e02` on branch `feature/spec-findings` (local commit, PO order 2026-10-22): whole-MG pass, plain-text section references turned into links; no wording change, no finding, no GL-ADR. SF-001 … 095 resolved; next free id SF-096.

## Amendment C-032 addendum 6 (2026-10-22, guideline baseline `df752cd`)

- The previous baseline `4e15e02` is recorded in the amendment above.
- The baseline is now `df752cd` on branch `feature/spec-findings` (local commit, PO order 2026-10-22): explicit anchors at all figure/table captions and anchor links in the lists of figures/tables (MG and RIG); no wording change, no finding, no GL-ADR. SF-001 … 095 resolved; next free id SF-096.

## Amendment C-032 addendum 7 (2026-10-22, guideline baseline `a9008a1`)

- The previous baseline `df752cd` is recorded in addendum 6.
- The baseline is now `a9008a1` on branch `feature/spec-findings` (local commit, PO order 2026-10-22): MG 2 introduces the OI4 Guardrails instead of the old OI4 Reference Architecture (paragraph, Figure 2, List of Figures); informative chapter, no inventory change, no finding, no GL-ADR. SF-001 … 095 resolved; next free id SF-096.

## Amendment C-032 addendum 8 (2026-10-22, guideline baseline `9e9f7cc`)

- The previous baseline `a9008a1` is recorded in addendum 7.
- The baseline is now `9e9f7cc` on branch `feature/spec-findings` (local commit, PO order 2026-10-22): MG 2.1 Introduction to Edge Computing updated (neutral to cloud/enterprise IT, offline operation, extended advantages, CRA); informative section, three informative inventory keys remapped, no requirement, no finding, no GL-ADR. SF-001 … 095 resolved; next free id SF-096.

## Amendment C-032 addendum 9 (2026-10-22, guideline baseline `2025207`)

- The previous baseline `9e9f7cc` is recorded in addendum 8.
- The baseline is now `2025207` on branch `feature/spec-findings` (local commit, PO order 2026-10-22): all entries of the MG Content list linked, 8.3.13/8.3.13.1 added to it; no wording change, no inventory change, no finding, no GL-ADR. SF-001 … 095 resolved; next free id SF-096.

## Amendment C-032 addendum 10 (2026-10-22, guideline baseline `2018bc4`)

- The previous baseline `2025207` is recorded in addendum 9.
- The baseline is now `2018bc4` on branch `feature/spec-findings` (local commit, PO order 2026-10-22): MG 6.1 Protocol Version reworded (no MQTT 5.0 features needed, MQTT 3.1.1 minimum for maximum acceptance and interoperability; changes of the minimum are for the work groups and the technical committee). No new finding, no GL-ADR. SF-001 … 095 resolved.
- Spec remap only: REQ-MQTT-001/002 quotes and inventory keys for the reworded sentences (statements unchanged, see log addendum 10).

## Amendment C-032 addendum 11 (2026-10-22, guideline baseline `75da059`)

- The previous baseline `2018bc4` is recorded in addendum 10.
- The baseline is now `75da059` on branch `feature/spec-findings` (local commit, PO order 2026-10-22): MG 6.4 rewritten editorially, no change of meaning, no new finding, no GL-ADR. SF-001 … 095 resolved.
- Spec remap only: REQ-MQTT-019 quotes and inventory keys (see log addendum 11).

## Amendment C-032 addendum 12 (2026-10-22, guideline baseline `c1419dd`)

- The previous baseline `75da059` is recorded in addendum 11.
- The baseline is now `c1419dd` on branch `feature/spec-findings` (local commits `eef0e04`, `afb6acb`, `c1419dd`, PO order 2026-10-22): Welcome and the start pages of MG, RIG and Process Description restructured (title pages removed, documents described, authors referenced), MG 1/1.1 reworked; the RIG start page states the independence of the compliance domains. No change of a requirement, no new finding, no GL-ADR. SF-001 … 095 resolved.
- Spec: seven new informative inventory items, one removed (see log addendum 12).

## Amendment C-032 addendum 13 (2026-10-22, guideline baseline `4811e60`)

- The previous baseline `c1419dd` is recorded in addendum 12.
- The baseline is now `4811e60` on branch `feature/spec-findings` (local commit, PO order 2026-10-22): links to shops selling standards (IEC webstore, Beuth, ISO) and the Amazon BOM link removed, reference texts unchanged; only freely accessible standards are linked. No inventory change, no finding, no GL-ADR. SF-001 … 095 resolved.

## Amendment C-032 addendum 14 (2026-10-22, guideline baseline `69863d3`)

- The previous baseline `4811e60` is recorded in addendum 13.
- The baseline is now `69863d3` on branch `feature/spec-findings` (local commits `62f85f3` – PO edit of the MG lists of figures/tables – and `69863d3` – Docusaurus landing page takes over the Welcome content, `docs/intro.md` removed; PO order 2026-10-22). No normative text changed, no inventory change, no finding, no GL-ADR. SF-001 … 095 resolved.

## Amendment C-032 addendum 15 (2026-10-22, guideline baseline `21bf409`)

- The previous baseline `69863d3` is recorded in addendum 14.
- The baseline is now `21bf409` on branch `feature/spec-findings` (local commits `86dbf25` – MG 1.2 Conventions use MG-only examples and references – and `21bf409` – new RIG 1.2 Conventions, RIG compliance chapter renumbered to 1.3, RIG tables 1–11 → 2–12; PO decision 2026-10-22). Keyword mapping unchanged and identical in MG 1.2 and RIG 1.2 (G-1.6). Spec re-baselined (`249a00b`): key remap of the RIG compliance, label and MG 1.2 example items, 17 new RIG 1.2 items informative; no obligation changed, no finding, no GL-ADR. SF-001 … 095 resolved.

## Amendment C-032 addendum 16 (2026-10-22, guideline baseline `9b6b22f`)

- The previous baseline `21bf409` is recorded in addendum 15.
- The baseline is now `9b6b22f` on branch `feature/spec-findings` (local commits `1b4904e` – PO text edits, headings of MG 5 and MG 4.3.1 without code formatting – and `9b6b22f` – the en-US rule for `Model` moved from MG 5 to MG 4.3.1, MG 5 requires `Model` at least in `en-US` if used for the `Oi4Identifier`; PO request 2026-10-22). Spec re-baselined (`da37e8b`): key remap, REQ-IL-027 and REQ-RES-MAM-006 re-worded and new REQ-RES-MAM-024, all three draft for PO re-review. No finding, no GL-ADR. SF-001 … 095 resolved.

## Amendment C-032 addendum 17 (2026-10-22, guideline baseline `f951bcb`)

- The previous baseline `9b6b22f` is recorded in addendum 16.
- The baseline is now `f951bcb` on branch `feature/spec-findings` (local commit `f951bcb` – links in MG 2.3 and MG 5 `GlobalAssetId` (IDTA-01001), "from Table 8" in MG 4.2; PO request 2026-10-22). Spec re-baselined (`070d054`): one informative key remap, all REQs reviewed. No finding, no GL-ADR. SF-001 … 095 resolved.
