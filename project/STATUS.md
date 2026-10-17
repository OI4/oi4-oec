# Status

_Last updated: 2026-10-17, cycle C-023_

## Current phase

**P1 – Spec baseline (in execution; C-023 done, next C-024)** – C-012 wrote the P1 plan (`log/2026-10-14_C-012_p1-plan.md`), C-013 re-baselined on guideline `49b6fe2` and updated the plan (`log/2026-10-14_C-013_rebaseline-49b6fe2.md`), C-014 built `oec/spec/` with `spectool` and drafted area `CMP` (`log/2026-10-14_C-014_spec-skeleton-cmp.md`), C-015 implemented SF-036 … 038 in the guideline on PO request and re-baselined on `5cb109e` (`log/2026-10-15_C-015_guideline-sf036-038.md`). P3 leftovers (Rust/C API sketches, crate choices) continue in parallel.

Previous phase: **P3 – SDK and conformance tool design** – architecture decided: S4 Rust SDK + C API (ADR-0011); local monorepo `oec/` (ADR-0012). Workspace is now a local git repo.

C-006 re-baselined all project artifacts on guideline `5f070bb` (guideline ADRs GL-ADR-001 … 011 implemented; guideline split into MG + RIG). All project decisions remain valid; ADR-0001/0005/0006/0011 amended.

C-007 decided Q-013–Q-015: second tool D8 for infrastructure compliance (ADR-0014), v2 only (ADR-0015), level targets + micro SDK D9 (ADR-0016).

C-008 re-baselined on guideline `7bead4c` (PO resolved all spec findings SF-001 … 031; new GL-ADR-012 only `NewDataSetWriterId` as common service, GL-ADR-013 NodeIds as JSON strings). No project decision changed; ADR-0001/0006/0008/0014 amended. No open spec findings at that point (new since C-012: SF-032 … 034).

C-009: micro SDK D9 in C17 with ESP32/ESP-IDF as reference (ADR-0017, Q-016); D8 scope incl. platform checks confirmed (Q-017).

C-010: C vs Rust for the micro SDK re-assessed – C17 confirmed as recommendation, Arduino library added as second port (ADR-0017 amendment). Q-011 parked with working assumption: private GitHub repos, no public availability.

C-011: M5Stack added to the micro SDK (`esp32s3` now supported, M5Stack Core2/CoreS3 as second reference device, M5Stack example firmware; ADR-0017 amendment).

C-012: P1 plan written – catalog `REQ-<AREA>-NNN` as YAML in a new `oec/spec/` folder with per-level obligations, "Level 1 first" area order (C-013 … C-022), ABNF + regex topic grammar, Method × Resource matrix in YAML, curated Appendix B golden vectors. New findings SF-032 (normative keywords), SF-033 (immature chapters/specific services), SF-034 (external "Requirements for Interoperability" doc); new question Q-018 (catalog format/location). **PO was not available – nothing confirmed yet, no catalog content written.**

C-013: re-baselined on guideline `49b6fe2` – PO resolved SF-032 (MG 1.2 normative keywords, BCP 14; keyword-less statements are informative), SF-033 (GL-ADR-014 removes the specific services, GL-ADR-015 moves former MG 6 to the informative OEC Process Description and renumbers MG 7–10 → 6–9) and SF-034 (no external document); GL-ADR-016 removes the empty AAS chapter. GUARDRAILS (new G-1.6 keywords), digest, ROADMAP, ADR-0001 (amendment), AGENTS.md updated; living documents use the new MG numbers, historical ADRs/logs keep the old ones. New findings **SF-035** (protocol behavior without keyword, e.g. Health 60 s heartbeat, Status event after Set/Del) and **SF-036** (closed vs. open set of resources/services). P1 plan: decisions 3–5 settled, area `SVC` and field `maturity` dropped, `ACT` folded into the `MSG` cycle, cycle cut C-014 … C-022. **PO decisions (C-013):** Q-018 → catalog as YAML in `oec/spec/`, shared guideline-derived contract (ADR-0018; ADR-0006/0012/0013 amended, G-4.12); SF-035 → PO adds keywords, until then implicit MUST (G-1.6); SF-036 → closed resource/service set (G-2.1); P1 plan confirmed (cycle cut C-014 … C-022, ROADMAP P1); golden examples are checked against the guideline and built as compliant, meaningful test data, reviewed by the PO per increment (ADR-0018 amendment, G-3.9).

C-014: `oec/spec/` created (ADR-0018 amendment C-014): Go module `oec/spec/tools` with the CLI `spectool` (`lint` – strict YAML subset + JSON Schema 2020-12 + catalog rules + verbatim quote check against the guideline; `render` – Markdown views; `inventory` – normative-statement candidates of MG + RIG; `coverage` – per-area mapping report; all with `-check`), schemas, `areas.yaml` (26 areas incl. new `BUS`, `GEN`), inventory at `49b6fe2` (1232 items). **Area `CMP` drafted:** REQ-CMP-001 … 032 (`status: draft`), 88/88 CMP inventory items mapped (59 covered, 29 informative). New findings **SF-037** (Level 3 scope of methods/resources, "Where applicable") and **SF-038** (MG 1.3 Level 2 text vs. RIG 1.2 OCI at Level 1) – open, interpretations used in the drafts.

C-015: PO accepted the CMP view and the informative classification and decided **SF-037** (Level 3 = all messages; a service without Config/Data/subscriptions answers `Get` with an empty list and publishes nothing on its own) and **SF-038** (Messaging and Service Container compliance are independent: microcontroller can be Messaging L3 without any RIG level, a container RIG L3 with Messaging L1). On the PO's request the agent edited the guideline directly (local commits `609cb4a` SF-038, `e0b1676` SF-037, `0106c5f` SF-036, `5cb109e` register sync; Docusaurus build ok). New baseline **`5cb109e`**; `oec/spec/` re-baselined (1255 inventory items): CMP keys/quotes remapped, REQ-CMP-022 deprecated → REQ-CMP-037, new drafts **REQ-CMP-033 … 043**, CMP 110/110 mapped. ADR-0001/0016 amended, GUARDRAILS G-2.13/14, digest, ROADMAP (cycles +1), AGENTS.md updated. New **SF-039** (Get Metadata without Data) and **Q-019** (micro SDK target level) – both open, not blocking.

C-016: PO decisions implemented in the guideline – `Event` `Status` answer to `Set`/`Del` must from Level 2, should at Level 1; **SF-039** (`Get` of `Metadata` without `Data` not answered, may answer `Status` `Bad`); **Q-019** (D9 designed for Messaging L3, L1 first; ADR-0016 amendment). **SF-035** resolved by a keyword pass over the guideline (`0de223c`, `f830382`). Re-baseline on `de852d6` (REQ-CMP-023/043 updated, REQ-CMP-044 new; CMP 0 open). No open spec findings. Log: `log/2026-10-15_C-016_po-decisions-sf035-keywords.md`.

C-017: PO request – guideline **GL-ADR-017** (`f5d69d8`): MG 7.2 "Minimum Set of Topic Elements" (7.2.1 applications, 7.2.2 devices) removed; requirements per level follow from MG 1.3; new MG 1.3 section "Directly Connected and Represented Entities"; "Resources Without Content" moved to MG 9.1; extended (`7677161`): MG 7 flattened, topic elements MG 7.1.1 … 7.1.7 → MG 7.1 … 7.7; generic MQTT topic intro of MG 7 removed, MQTT v3.1.1 §4.7 linked in the MG 7 index (`4d2f919`, `10491df`); heading "Topic Elements" removed by the PO (`d7d8750`); white-space "should" moved to MG 7.7 `Filter` on PO request (`cb50c04`). Re-baseline on `bc5d063`; REQ-CMP remapped, new REQ-CMP-045. **PO review done:** guideline commits of C-016/C-017 and REQ-CMP-023/024/043/044/045 confirmed → area `CMP` and all its REQs `reviewed`. Log: `log/2026-10-15_C-017_gl-adr-017-remove-mg-7-2.md`.

C-018: areas **`IL`** (REQ-IL-001 … 032) and **`TOP`** (REQ-TOP-001 … 027) `drafted` (IL 91, TOP 76 inventory items, 0 open). Formal topic grammar `oec/spec/topic/topic.abnf` (59 rules) with generated portable regexes `topic-patterns.json`; Method × Resource matrix (topic columns, 66 combinations, 29 allowed) `oec/spec/matrix/method-resource.yaml`; 203 vectors in `oec/spec/vectors/` (146 topic incl. 60 golden, 29 IL, 15 dnp-encoding, 13 `Oi4Identifier`), all recomputed by `spectool lint`; new `spectool topic`; cross-checked with Node.js, Python `re`, Rust `regex` and the Python `abnf` package (0 mismatches). ADR-0018 amendment C-018, GUARDRAILS G-1.2 baseline fixed. New findings **SF-040 … 043** (filter encoding, allowed topic forms, closed value sets, dnp-encoding/`ManufacturerUri`) – proposed interpretations in use. **Follow-up (PO, 2026-10-15):** spec findings are kept only in `oi4-oec-documentation/findings/SPEC-FINDINGS.md` (project copy merged and deleted, guideline commit `d84c642`, ADR-0001 amendment C-018); **SF-040, SF-041, SF-042 decided** (proposed interpretations accepted; guideline text not changed yet); **SF-043** – PO asked whether the dnp-encoding is obsolete; agent analysis: obsolete for topics, still needed to build the `Oi4Identifier` (IEC 61406-1 character set, `/` collisions, regenerability) → stays open, GL-ADR-010 not changed. Log: `log/2026-10-15_C-018_il-top-topic-grammar.md`.

C-019: PO decision on **SF-043 (e)** (2026-10-16): the `Oi4Identifier` is one way to build an IEC 61406-1 Identification Link, so all IEC 61406-1 rules apply to it; the dnp-encoding stays only as an optional hint at the `Oi4Identifier` (explained in MG A3) and is removed everywhere else. Guideline commit `7de6cb8`: MG 4.3.1 (elements are plain strings, note "the dnp-encoding (A3) may be used" for non-permitted characters or `/`, structure line `ManufacturerUri/Model/ProductCode/SerialNumber`), MG 4.1 (dnp sentence removed), MG A3 (may instead of must, topic sentence removed), MG A4 (glossary aligned to IEC 61406-1), GL-ADR-010 implementation note; register commit `ea80271` (SF-043 (e) resolved, (a)–(d) open). Link check 714/0, Docusaurus build ok. `oec/spec` re-baselined on **`ea80271`**: REQ-IL-022/029/032 updated, new **REQ-IL-033** (MAY), vectors and reference comments aligned; P1 area cycles shifted by one. Log: `log/2026-10-16_C-019_sf043-dnp-encoding.md`.

C-020: PO input (2026-10-17): SF-043 (a)–(d) proposals accepted plus the addition that the `ManufacturerUri` (MAM) should correspond to the Identification Link of the asset, `<manufacturer>.<tld>` in case of doubt; `il.md` reviewed ("passt"); **REQ-TOP-007 withdrawn** (no such rule for the other ServiceTypes); implement SF-040, SF-042 and SF-043 in the guideline. Guideline commits `6476bd3` (SF-043 a–d: A3 general UTF-8 rule, ASCII Table 4, MG 4.3.1/MG 5 `ManufacturerUri`), `d00cce7` (SF-040: MG 7.7 hex/`%`/empty levels, Appendix B and MG 8.3.3.x without `%20`, `Status/Good`), `a992f9a` (SF-042: closed ServiceType/EventCategory sets, free GenericLevel, OTConnector-device sentence removed), `39be075` (MUST of the `ManufacturerUri` form kept), register `4c27c37`/`5dedfd5`. Link check 716/0, Docusaurus build ok. `oec/spec` re-baselined on **`5dedfd5`**: REQ-TOP-005/019/021/023 and REQ-IL-023/024/032 quote the new text, REQ-TOP-007 `deprecated`, new **REQ-IL-034** (SHOULD, `ManufacturerUri` ↔ IL); B1.1.3 golden vectors without edits; area **IL reviewed** (REQ-IL-001 … 033; 034 draft). SF-041 not requested → decided, guideline text unchanged. P1 cycles shifted by one. Log: `log/2026-10-17_C-020_sf040-042-043-guideline.md`.

C-021: PO input (2026-10-17): implement **SF-041** as proposed; `top.md` reviewed ("passt"); REQ-IL-034 fine ("in case of doubt" = Identification Link host with third-level domain; the scheme is always dropped). Guideline commits `61a3182` (SF-041 a–c: only the topic forms of MG 9.1.x/9.2.x, `Pub` `Metadata` with `Source`/`Filter`, `Call`/`Reply` with optional `Source` and no `Filter`) and `d4a7560` (SF-041 d–g: MG 7.7 `<ResourceType>/<Tag>` definition, MG 8.3.8/9.1.8/9.1.9 notes, B1.1.8 Set example `…/Data/oee` and valid `Mode`, templates, typo), register `2cf0d70` (SF-041 resolved, new **SF-044** open: `WriterGroupName` of `PublicationList`/`SubscriptionList` entries). Link check 724/0, Docusaurus build ok. `oec/spec` re-baselined on **`2cf0d70`**: REQ-TOP-013/016/018/022 quote the new text; golden vector V-TOP-044 without edit; area **TOP reviewed** (REQ-TOP-001 … 027, 007 deprecated), **REQ-IL-034 reviewed**. P1 cycles shifted by one. Log: `log/2026-10-17_C-021_sf041-guideline.md`.

C-022: PO input (2026-10-17): **SF-044** decided as proposed and to be recorded in an ADR; PO asked where to review the matrix and the vectors (answered in the chat: `oec/spec/matrix/method-resource.yaml`, `oec/spec/topic/topic.abnf`, `oec/spec/vectors/`). Guideline commit `d198e6b`: new **GL-ADR-018** "Use complete Filter as WriterGroupName of list entries" (`rfcs_adrs/ADR-018_…`) and its implementation – MG 8.2.3 (the `WriterGroupName` of each `PublicationList`/`SubscriptionList` `DataSetMessage` must be `<ResourceType>[/<Tag>]`, also on a topic without `Filter`), MG 9.1.8/9.1.9 templates and examples, B1.1.8 (`Data/oee`, `Event/Syslog/Notice`), B2.6/B2.7 ToDo placeholders; register `fd9da6e` (SF-044 resolved, **no finding open**). Link check 724/0, Docusaurus build ok. `oec/spec` re-baselined on **`fd9da6e`** without key changes (MG 8/9 belong to the planned payload areas). P1 cycles shifted by one. **Follow-up (PO, 2026-10-17):** `spectool render` also generates review views `oec/spec/matrix/view/method-resource.md` and `oec/spec/vectors/view/*.md` (ADR-0018 amendment C-022); **PO review done:** the topic columns of the Method × Resource matrix and all 203 vectors are `reviewed` (G-3.9). Log: `log/2026-10-17_C-022_gl-adr-018-writergroupname.md`.

C-023: areas **`MSG`** (REQ-MSG-001 … 045; MG 8, A2) and **`ACT`** (REQ-ACT-001 … 023; MG 8.2.16 … 8.2.20, 8.4, 9.2) `drafted` (MSG 160 inventory items: 152 covered, 8 informative; ACT 70: 63 covered, 7 informative; 0 open). REQ-MSG-006 corrected during the cycle: for `Get`/`Set`/`Del` the topic `AppId` is the destination (MG 7.3), so the `PublisherId` is the requester's. **Golden payload vectors started** (ADR-0018 amendment C-023): new vector kind `payload` (`topic` + JSON `payload`, `original` + JSON-Pointer `edits`), reference envelope checks in `oec/spec/tools/internal/payload` (each problem names the violated REQ), 9 `draft` vectors V-PAY-001 … 009 from B1.1.2, B1.1.11, B1.1.12 and B1.2.1 (`vectors/payload/guideline.yaml`, view `vectors/view/payload-guideline.md`); B2.x metadata examples and Disconnect/Will messages not curated yet. New findings **SF-045 … 051** (register commits `367a2e0`, `06eea55`; proposed interpretations used in the drafts, none blocking): `ua-metadata` encoding, `LocalizedText` `Locale`, `KeyValuePair`/`EngineeringUnits`, `DataSetClassId` of `Get`/`Data`, editorial items (a)–(i), unlisted OPC UA fields, Action correlation/`TimeoutHint`. Baseline unchanged (`fd9da6e`, guideline text not changed). Log: `log/2026-10-17_C-023_msg-act.md`.

## Next step

PO gave the go to start (2026-10-14, after C-011). Order: **P1 spec baseline** → **P2 JSON Schema** (+ D7 `validate`); remaining P3 work (Rust/C API sketches, crate choices) in parallel.

1. **PO review of C-023:** `oec/spec/requirements/view/msg.md`, `act.md`, `oec/spec/vectors/view/payload-guideline.md`; decide SF-045 … 051 (register `oi4-oec-documentation/findings/SPEC-FINDINGS.md`); confirm the assumptions listed in the C-023 log (lower-case GUIDs, lenient `ua-metadata` validation until SF-045, OPC UA version of the links).
2. **C-024** – areas `MQTT` and `LIFE` (golden vectors for Birth/Close/Will may be built in the cycle). Workflow per area: `oec/spec/README.md`. Further cycles per ROADMAP P1 (C-025 … C-029).
3. Whenever the guideline commit changes, re-baseline first: `spectool inventory -rev <commit>`, bump `baseline` in all catalog files (quoted), remap the keys `spectool lint` reports. Commits that only touch `findings/` need no re-baseline (ADR-0001 amendment C-018).

## Deliverables

| ID | Deliverable | State |
|----|-------------|-------|
| D1 | JSON Schema | not started – P1 spec baseline in `oec/spec/` (areas CMP, IL, TOP reviewed, MSG and ACT drafted; topic grammar, topic columns of the matrix and 203 topic/IL vectors reviewed; 9 golden payload vectors draft) |
| D2 | SDK | not started – S4 Rust SDK + C API decided (ADR-0011); location `oec/sdk/rust` |
| D3 | Weather demo | not started – target Messaging L3 = all messages (ADR-0016 amendment C-015) |
| D4 | Logger demo | not started – target Messaging L1 (ADR-0016) |
| D5 | Registry | not started – target L3, v2 only |
| D6 | Docusaurus docs | not started – new standalone site in `oec/docs` (ADR-0012) |
| D7 | Application compliance tool (CLI) | not started – Go (ADR-0013), Messaging L1–L3 of apps/devices, shares only schemas; location `oec/conformance` |
| D8 | Infrastructure compliance tool (CLI) | not started – Go, Service Container + platform (ADR-0014); location `oec/conformance` |
| D9 | Micro SDK | not started – C17, ESP-IDF (`esp32`, `esp32s3`) + Arduino, reference ESP32 dev board + M5Stack, designed for Messaging L3, L1 delivered first (Q-019, ADR-0016/0017); location `oec/sdk/micro` |

## Baselines

- Guideline: `oi4-oec-documentation` @ `fd9da6e`, branch `feature/spec-findings` (MG + RIG + GL-ADR-001 … 018, SF-001 … 044 resolved, SF-045 … 051 open (C-023); OEC Process Description informative); previous `2cf0d70`, `5dedfd5`, `ea80271`, `bc5d063`, `de852d6`, `5cb109e`, `49b6fe2`, `7bead4c`, `5f070bb`, `c85ce2a`
- **Container engine:** Docker Desktop 28.4 is installed, but the daemon was **not running** on 2026-10-14. It is not needed for P1/P2, unit tests, host tests or the offline commands of D7/D8. It is **needed from P4 on** for:
  - the TLS dev broker (`oec/dev`, docker compose)
  - integration tests (SDK, D7 live tests, demos, registry)
  - building OCI images with SBOMs
  - D8 checks of running containers
  
  Any Docker-compatible engine works (Docker Desktop, OrbStack, Colima, Podman); CI uses Linux runners with Docker.
- Toolchain seen: node 22.13, npm 11, yarn 4.13, go 1.24.2 (+ go1.27.2 via `GOTOOLCHAIN=auto` for `oec/spec/tools`), Docker 28.4, Python 3.12.9 (macOS arm64)

## Blocking open questions

- None blocking. SF-045 … 051 are open (C-023, proposed interpretations used in the MSG/ACT drafts, not blocking); next free id SF-052.
- Q-011 (publication/CRA) parked – working assumption: private GitHub repos only.

## Decided so far

ADR-0001 source of truth · ADR-0002 languages · ADR-0003 JSON Schema 2020-12 · ADR-0004 MIT · ADR-0005 drop `_ToDelete_` · ADR-0006 conformance tool D7 · ADR-0007 PO decides spec interpretations · ADR-0008 registry read-only (step 1) · ADR-0009 GitHub + single commits · ADR-0010 MET Norway weather provider · ADR-0011 SDK = S4 Rust + C API · ADR-0012 local git repo + `oec/` monorepo, standalone docs · ADR-0013 D7 in Go · ADR-0014 two compliance tools D7/D8 · ADR-0015 v2 only · ADR-0016 level targets + micro SDK · ADR-0017 micro SDK in C17, ESP32 reference · ADR-0018 REQ catalog as YAML in `oec/spec/` (shared contract)

## Cycle history

| Cycle | Date | Topic | Log |
|-------|------|-------|-----|
| C-001 | 2026-10-12 | Onboarding, guardrails, roadmap | `log/2026-10-12_C-001_onboarding.md` |
| C-002 | 2026-10-12 | Scope: conformance tool D7; decisions schema/license/ToDelete | `log/2026-10-12_C-002_scope-conformance-tool.md` |
| C-003 | 2026-10-12 | Decisions Q-004–Q-010; SDK options paper | `log/2026-10-12_C-003_decisions-sdk-options.md` |
| C-004 | 2026-10-12 | SDK = S4, monorepo `oec/`, local git repo | `log/2026-10-12_C-004_sdk-s4-monorepo.md` |
| C-005 | 2026-10-12 | D7 in Go; OPEN-QUESTIONS split into open/decided | `log/2026-10-12_C-005_d7-go-open-questions.md` |
| C-006 | 2026-10-14 | Re-baseline on guideline `5f070bb` (GL-ADRs, MG/RIG split) | `log/2026-10-14_C-006_guideline-rebaseline.md` |
| C-007 | 2026-10-14 | Decisions Q-013–Q-015: D8, v2 only, level targets, micro SDK | `log/2026-10-14_C-007_compliance-tools-levels.md` |
| C-008 | 2026-10-14 | Re-baseline on guideline `7bead4c` (all SF resolved, GL-ADR-012/013) | `log/2026-10-14_C-008_rebaseline-findings-resolved.md` |
| C-009 | 2026-10-14 | Micro SDK in C / ESP32 (Q-016); D8 scope confirmed (Q-017) | `log/2026-10-14_C-009_micro-sdk-c-esp32.md` |
| C-010 | 2026-10-14 | Micro SDK C vs Rust assessment, Arduino port; Q-011 working assumption | `log/2026-10-14_C-010_micro-sdk-assessment-q011.md` |
| C-011 | 2026-10-14 | M5Stack support in the micro SDK | `log/2026-10-14_C-011_m5stack.md` |
| C-012 | 2026-10-14 | P1 plan (catalog format, area order, topic grammar, matrix, golden examples); SF-032 … 034, Q-018 | `log/2026-10-14_C-012_p1-plan.md` |
| C-013 | 2026-10-14 | Re-baseline on guideline `49b6fe2` (SF-032 … 034 resolved, GL-ADR-014 … 016, MG renumbering); SF-035/036 found and decided; Q-018 → ADR-0018; P1 plan + golden-example rules confirmed | `log/2026-10-14_C-013_rebaseline-49b6fe2.md` |
| C-014 | 2026-10-14 | `oec/spec/` skeleton, `spectool` (lint/render/inventory/coverage), inventory of MG + RIG, area `CMP` drafted (REQ-CMP-001 … 032); SF-037/038 | `log/2026-10-14_C-014_spec-skeleton-cmp.md` |
| C-015 | 2026-10-15 | PO decisions SF-037/038 implemented in the guideline (+ SF-036), re-baseline on `5cb109e`, CMP reworked (REQ-CMP-033 … 043, 022 deprecated); SF-039, Q-019 | `log/2026-10-15_C-015_guideline-sf036-038.md` |
| C-016 | 2026-10-15 | PO decisions (Status event L1 should / L2+ must, SF-039, Q-019), SF-035 keyword pass in the guideline, re-baseline on `de852d6` (REQ-CMP-044) | `log/2026-10-15_C-016_po-decisions-sf035-keywords.md` |
| C-017 | 2026-10-15 | GL-ADR-017: MG 7.2 removed (requirements from MG 1.3, represented entities in MG 1.3, resources without content in MG 9.1); MG 7 flattened (7.1.x → 7.x); MG 7 MQTT intro removed; re-baseline on `bc5d063`, REQ-CMP-045, CMP reviewed | `log/2026-10-15_C-017_gl-adr-017-remove-mg-7-2.md` |
| C-018 | 2026-10-15 | Areas `IL` + `TOP` drafted (REQ-IL-001 … 032, REQ-TOP-001 … 027); topic grammar (ABNF + generated regexes), Method × Resource matrix (topic columns), 203 topic/IL vectors, `spectool topic`; SF-040 … 043 (040 … 042 decided); findings register moved to the guideline repo | `log/2026-10-15_C-018_il-top-topic-grammar.md` |
| C-019 | 2026-10-16 | SF-043 (e): dnp-encoding only as optional hint at the `Oi4Identifier` (MG 4.1, 4.3.1, A3, A4, GL-ADR-010); re-baseline on `ea80271`, REQ-IL-033; P1 cycles shifted by one | `log/2026-10-16_C-019_sf043-dnp-encoding.md` |
| C-020 | 2026-10-17 | SF-040, SF-042, SF-043 (a)–(d) in the guideline (+ `ManufacturerUri` ↔ IL), REQ-TOP-007 withdrawn, IL reviewed; re-baseline on `5dedfd5`, REQ-IL-034; P1 cycles shifted by one | `log/2026-10-17_C-020_sf040-042-043-guideline.md` |
| C-021 | 2026-10-17 | SF-041 in the guideline (allowed topic forms, Metadata Source/Tag, Call/Reply Source, PublicationList Filter, B1.1.8 example); TOP + REQ-IL-034 reviewed; SF-044; re-baseline on `2cf0d70`; P1 cycles shifted by one | `log/2026-10-17_C-021_sf041-guideline.md` |
| C-022 | 2026-10-17 | SF-044 decided and recorded in GL-ADR-018 (`WriterGroupName` of `PublicationList`/`SubscriptionList` entries = `<ResourceType>[/<Tag>]`), implemented in MG 8.2.3, 9.1.8, 9.1.9, B1.1.8, B2.6, B2.7; re-baseline on `fd9da6e`; P1 cycles shifted by one; review views of matrix and vectors; matrix (topic columns) + 203 vectors reviewed | `log/2026-10-17_C-022_gl-adr-018-writergroupname.md` |
| C-023 | 2026-10-17 | Areas `MSG` + `ACT` drafted (REQ-MSG-001 … 045, REQ-ACT-001 … 023); golden payload vectors started (kind `payload`, reference envelope checks, V-PAY-001 … 009 draft); SF-045 … 051 | `log/2026-10-17_C-023_msg-act.md` |
