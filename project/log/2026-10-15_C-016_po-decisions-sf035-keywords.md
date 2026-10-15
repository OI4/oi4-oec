# C-016 – PO decisions on C-015 follow-ups, SF-039, Q-019 and the SF-035 keyword pass

- Date: 2026-10-15
- Phase: P1
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Status: in progress
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
