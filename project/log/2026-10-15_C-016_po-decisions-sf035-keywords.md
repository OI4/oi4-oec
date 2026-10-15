# C-016 – PO decisions on C-015 follow-ups, SF-039, Q-019 and the SF-035 keyword pass

- Date: 2026-10-15
- Phase: P1
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Status: done
- Guideline baseline at cycle start: `oi4-oec-documentation` @ `5cb109e`, branch `feature/spec-findings` (checked: unchanged, working tree clean)

## Intent

The PO answered the C-015 report (chat, 2026-10-15):

1. **Status event after `Set`/`Del`:** not "must at every level" as proposed by the agent, but **Level 1 should, Level 2 and 3 must**. Put it into the guideline.
2. Question: what does the agent mean by "device representations" (answered in the chat; see "Answers" below).
3. **SF-039:** option (a) – a `Get` of `Metadata` without `Data` is not answered – plus a **may** for the `Event` `Status`/`Bad` (`BadNotFound`). Put it into the guideline.
4. **Q-019:** option (a) – the micro SDK (D9) is designed for Messaging Level 3 and delivers Level 1 first.
5. **SF-035:** if the agent has enough information, fix it (add the keywords to the guideline).

The planned area cycle "IL + TOP + topic grammar" moves to C-017 (all later P1 cycles shift by one again).

## Scope / acceptance criteria

- Guideline: level rule for the Status event (MG 7.1.4, 7.2, 7.2.1, 7.2.2, 9.1, GL-ADR-009 note); SF-039 rule (MG 7.2, 9.1.5, 8.3.6.1, GL-ADR-009 note); SF-035 keyword pass over all keyword-less inventory items (`basis: implicit`) outside CMP sections, using the decided interim rule (b) as default: observable protocol behavior → MUST, explanatory text stays informative; deviations (SHOULD/MAY) flagged for the PO.
- No broken links; Docusaurus build ok; guideline findings register in sync.
- `project/SPEC-FINDINGS.md`: SF-037 amendment, SF-039 and SF-035 resolved with commits.
- `oec/spec/` re-baselined on the new guideline commit; CMP adapted (REQ-CMP-023/043 Level 1 SHOULD, new entry for SF-039); all `spectool` checks pass.
- ADR-0016 (Q-019 decision), OPEN-QUESTIONS, GUARDRAILS (G-1.6, G-2.13, G-2.14), digest, ROADMAP, AGENTS.md, ADR-0001, STATUS and this log updated.

## Work done

Guideline repo `oi4-oec-documentation` (branch `feature/spec-findings`, local commits, not pushed):

| Commit | Content |
|--------|---------|
| `a6147e7` | SF-037 amendment: `Event` `Status` answer to `Set`/`Del` must from Level 2, should at Level 1 (MG 7.1.4, 7.2 index, 7.2.1/7.2.2 notes, 9.1 index); `Set`/`Del` on unknown object ⇒ `Bad` (L2+ must, L1 should). SF-039: `Get` of `Metadata` without `Data` is not answered with `Metadata`; the entity may answer with `Event` `Status` `Bad` (`BadNotFound`); requesters only ask for `Data` announced in the PublicationList (MG 7.2 index, 9.1.5, 8.3.6.1). GL-ADR-009 implementation note. |
| `0de223c` | SF-035 keyword pass over the 253 keyword-less inventory items outside MG 1.x, MG 7.2.x and RIG 1.x: 122 must, 17 must not, 8 may, 106 kept informative (explanations, purpose sentences, topic labels). |
| `f830382` | SF-035 follow-up: MG 8.2.1 `ReplyTo` must contain the triggering `MessageId`, otherwise must be empty (sentence had no inventory trigger term). |
| `edf76e1`, `de852d6` | `findings/SPEC-FINDINGS.md` synced with `project/SPEC-FINDINGS.md`. |

Method of the SF-035 pass: four sub-agents classified disjoint worklists of the inventory (`basis: implicit`) and edited the guideline; the agent reviewed every decision and reverted 32 over-eager edits (purpose sentences "Is used to …"/"Used for …" in MG 7.1.4/7.1.5/7.1.5.1/7.1.7, the "Publish explicit DataSetMessage …" topic labels in MG 9.1.x) back to informative.

Workspace commits: `adac19b` (log kickoff), `7e2f586` (Q-019 decided, ADR-0016 amendment C-016), `6aefb06` (ROADMAP P4b), `5ed8e73` (SPEC-FINDINGS: SF-035/039 resolved, SF-037 amended), `108bc51` (`oec/spec` re-baseline `edf76e1`: REQ-CMP-023/043 L1 SHOULD, new REQ-CMP-044 for SF-039), `8ac57b6` (re-baseline `de852d6`), `074190e` (AGENTS, ADR-0001, GUARDRAILS, digest, ROADMAP), plus this log + STATUS.

## Verification

- Relative link + anchor check over `docs/`: 0 broken. Full Docusaurus build (copy outside the repo): success, only deprecation warnings.
- `grep` checks of the edited guideline sentences (Status level rule, SF-039 bullet, ReplyTo).
- `spectool lint`, `render -check`, `coverage -check`, `inventory -check`, `go test ./...`: pass. Inventory at `de852d6`: 1260 items, `implicit` 114 (was 261). CMP: 111 items, 79 covered, 32 informative, 0 open; 44 entries.
- Both registers identical except the baseline line and the mirror paragraph.

## Decisions

- Status event after `Set`/`Del`: L1 should, L2/L3 must (PO). Recorded in guideline, REQ-CMP-023/043, G-2.13/G-2.14, digest.
- SF-039: option (a) + may (PO) → REQ-CMP-044 (MAY at all levels).
- Q-019: option (a) (PO) → ADR-0016 amendment C-016, ROADMAP P4b.
- SF-035: interim rule (b) used as default for the keyword pass (agent, per PO mandate "fix it if you have enough information"). G-1.6: keyword-less behavior found later becomes a new spec finding.

## Items flagged for PO review (SF-035 pass)

- **May instead of must** (permission semantics in the text): MG 7 wildcard use and "a single `#` only at the end"; MG 7.1.5.2 service name used for security; MG 8.2.20 optional `Executing` response; MG 8.3.11.1/8.3.11.2 `PerPage`; MG 8.3.3.1/8.3.3.2 self-defined unit.
- MG 6.4: publish on change is now "must" – check against the PublicationList rules note.
- MG 8.3.3.1/8.3.3.2: "cannot be changed from outside" → "must not be changed" (4 items).
- MG 9.1 "likely requires" (`MG-9.1-7d286bd3`) kept informative.

## Answers

**Device representations** (PO question 2): MG 7.2.2 "For Devices" covers assets without their own MQTT client (e.g. field devices behind a connector or gateway application). The application represents them on the bus: it publishes `MAM`, `Health`, `Data` … for them under its own AppId with `Source` = the device's Oi4Identifier and answers `Get`/`Set`/`Del` for them. PublicationList, SubscriptionList and the common services (`NewDataSetWriterId`, `Call`/`Reply`) exist once per application, not per device – hence the device table lists fewer resources. A smart device with its own MQTT client counts as an application (MG 7.2.1).

## Follow-ups

- PO review: guideline commits `a6147e7`, `0de223c`, `f830382` and the flagged items above; REQ-CMP-023/043/044.
- C-017: areas `IL`, `TOP`, topic grammar + topic vectors (STATUS "Next step").

