# C-022 – SF-044 decided, GL-ADR-018 (WriterGroupName of list entries)

- Date: 2026-10-17
- Phase: P1
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Status: in progress
- Guideline baseline at cycle start: `oi4-oec-documentation` @ `2cf0d70`, branch `feature/spec-findings`

## Intent

PO input (chat, 2026-10-17, after C-021):

- **SF-044:** the proposed interpretation is accepted ("nehmen wir deine Interpretation") and is to be recorded in an ADR.
- The PO asks where exactly the Method × Resource matrix and the vectors can be reviewed (answered in the chat; no artifact change).

Assumptions (agent):

- The decision changes the payload of the guideline (`WriterGroupName` of `PublicationList`/`SubscriptionList` `DataSetMessages`), so it is recorded as a **guideline ADR** (`rfcs_adrs/ADR-018_…`, "GL-ADR-018"), like GL-ADR-017, and not as a project ADR. The project records it in the ADR-0001 amendment C-022.
- As with GL-ADR-017, the ADR is implemented in the guideline text in the same cycle (otherwise the ADR and the guideline would contradict each other). Implementation notes in the ADR list the changed sections.

The planned area cycle MSG + ACT moves to C-023. Later P1 cycles shift by one.

## Scope / acceptance criteria

- **Guideline**
  - `rfcs_adrs/ADR-018_…md` in the format of GL-ADR-017 (context, drivers, options, decision, consequences, more information, implementation notes).
  - MG 8.2.3: the `WriterGroupName` of `PublicationList`/`SubscriptionList` entries is `<ResourceType>[/<Tag>]`.
  - MG 9.1.8, MG 9.1.9 (templates and examples), MG B1.1.8, MG B2.6, MG B2.7 aligned.
  - Register: SF-044 resolved (GL-ADR-018). Link check clean, Docusaurus build ok.
- **`oec/spec`:** re-baseline, keys remapped, all `spectool` checks pass.
- **Docs:** STATUS, ROADMAP (cycle shift), `areas.yaml`, AGENTS, GUARDRAILS G-1.2/G-2.17, digest, ADR-0001 amendment C-022.

## Work done

_(in progress)_
