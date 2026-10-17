# C-022 – SF-044 decided, GL-ADR-018 (WriterGroupName of list entries)

- Date: 2026-10-17
- Phase: P1
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Status: done
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

### Guideline (`oi4-oec-documentation`, branch `feature/spec-findings`, local commits)

- `d198e6b` – **GL-ADR-018** `rfcs_adrs/ADR-018_Use_complete_Filter_as_WriterGroupName_of_list_entries.md` (context, drivers, two options, decision, consequences, implementation notes) and its implementation:
  - MG 8.2.3 `WriterGroupName`: for `PublicationList` and `SubscriptionList` the `WriterGroupName` of each `DataSetMessage` must be `<ResourceType>[/<Tag>]`, the `Filter` that addresses the entry (e.g. `Data/oee`, `Event/Syslog/Notice`, `MAM`), also on a topic without `Filter`; for a `PublicationList` entry `<ResourceType>` = `Resource`, `<Tag>` = `Filter` if not empty.
  - MG 9.1.8: `Pub`/`Set` templates `<ResourceType>[/<Tag>]` (lower-case `<resourceType>` fixed); examples `Event/<EventCategory>/<EventLevel>`, `Data/<Tag>` (MAM/Health unchanged).
  - MG 9.1.9: `Pub`/`Set`/`Del` templates unified to `<ResourceType>[/<Tag>]`.
  - B1.1.8: 4 × `Data` → `Data/oee`, 2 × `Event` → `Event/Syslog/Notice` (matching the payload `Filter` of each entry).
  - B2.6/B2.7: placeholders `<resourceType_Tag => ToDo>` → `<ResourceType>[/<Tag>]`; the `Filter` of the B2.6 `PublicationList` entry → `<Tag>` (as in MG 9.1.8).
  - B1.1.9 already used `Data/oee` – unchanged.
- `fd9da6e` – register: SF-044 → Closed (decided PO C-022, resolved `d198e6b`, GL-ADR-018); open table empty; next free id stays SF-045.
- Checks: relative-link check 724 links / 0 broken; Docusaurus build ok (temp copy).

### Workspace

- `oec/spec` re-baselined on `fd9da6e`: inventory 1234 items; no key of the reviewed areas moved (the changed texts are MG 8.2.3, 9.1.x and Appendix B – planned areas `MSG`/`RES-*`); vectors regenerated (baseline only). `spectool lint` ok (106 REQs, 70 informative items, 66 matrix rows, 203 vectors), `render`/`coverage`/`topic`/`inventory -check` ok, `go test -count=1 ./...` ok.
- Cycle shift: `areas.yaml` planned cycles +1, cycle hints in `informative.yaml`, `top.yaml`, matrix header; ROADMAP cycle cut (C-022 = this cycle, C-023 `MSG` + `ACT` … C-029 PO review), GL-ADR-001 … 018, exit line; ADR-0018 (MSG cycle C-023).
- Docs: AGENTS.md, GUARDRAILS G-1.2 (baseline) and G-2.8 (`WriterGroupName` of list entries), digest (GL-ADR 018 row, DataSetMessage table), ADR-0001 amendment C-022, STATUS.

### Answer to the PO's review question (chat)

There is no rendered view of the matrix and the vectors; the YAML files are the review objects:

- Matrix: `oec/spec/matrix/method-resource.yaml` (66 Method × Resource/service rows; `allowed`, ABNF rule, `source`/`filter` required/optional/forbidden, refs, notes); grammar `oec/spec/topic/topic.abnf`, generated regexes `oec/spec/topic/topic-patterns.json`.
- Vectors: `oec/spec/vectors/topic/guideline.yaml` (60 golden), `valid.yaml` (34), `invalid.yaml` (52); `oec/spec/vectors/il/identification-link.yaml` (29), `dnp-encoding.yaml` (15), `oi4identifier.yaml` (13).

## Decisions / assumptions

- SF-044 recorded as guideline ADR GL-ADR-018 (not as a project ADR), implemented in the same cycle (agent assumption, reported to the PO).
- The B2.6 `PublicationList` entry `Filter` placeholder was also fixed (`<Tag>`), because it had the same ToDo placeholder.

## Next

C-023 – areas `MSG` + `ACT`, payload columns of the matrix. PO review of the matrix and the vectors is still open.
