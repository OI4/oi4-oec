# C-027 – Core resources (MAM, Health, Profile, PublicationList, ReferenceDesignation), Pagination and Locale

- Date: 2026-10-19
- Phase: P1
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Status: in progress
- Guideline baseline: `oi4-oec-documentation` @ `c6453c7`, branch `feature/spec-findings` (GL-ADR-001 … 018, SF-001 … 055 and SF-057 … 061 resolved, SF-056 open (B1 examples, C-027/C-028, not blocking), next free id SF-062) – checked at cycle start (`git log`, register header, `spectool inventory -check -rev c6453c7` ok, 1279 items; `lint`, `render -check`, `coverage -check`, `topic -check` ok)

## Intent

PO request (chat, 2026-10-19, hand-over prompt `prompt.md`): execute C-027 as planned in STATUS "Next step":

- Areas `RES-MAM` (MG 5, 8.3.1, 9.1.1), `RES-HEALTH` (MG 8.3.2, 9.1.2), `RES-PROFILE` (MG 8.3.7, 9.1.7), `RES-PUBLIST` (MG 8.3.8, 9.1.8), `RES-REFDES` (MG 8.3.10, 9.1.10), `PAG` (MG 8.3.11) and `LOC` (MG 8.3.12): requirements as `draft`, every inventory item of these sections mapped or listed in `coverage/informative.yaml` with a reason, `lint`/`render`/`coverage`/`topic` green, areas `drafted`.
- Respect the PO decisions SF-035 … SF-061 and GL-ADR-018, in particular SF-052: sender strict (REQ-MSG-001), receiver tolerant (REQ-MSG-046, SHOULD), validation modes strict/tolerant (`payload.Mode`, vector field `mode`). Areas CMP, IL, TOP, MSG, ACT, MQTT, LIFE and all existing vectors (203 topic/IL, V-PAY-001 … 025) are reviewed – changes only with a reason in this log.
- Golden payload vectors (V-PAY-026 ff., `status: draft`, `edits` per ADR-0018 amendments C-013/C-023, G-3.9) for the B1 examples of the resources of this cycle (MG B1.1.1, B1.1.2, B1.1.7, B1.1.8, B1.1.10, B1.1.11, B1.1.12 – as far as not yet curated); the DataSet content is checked against the resource requirements. B1 defects of SF-056 are recorded as `edits`; the guideline correction of SF-056 only on PO request. Payload columns of the matrix follow in C-028.
- New ambiguities become SF-062 ff. in `oi4-oec-documentation/findings/SPEC-FINDINGS.md` (Closed table kept ascending) with a proposed interpretation. Guideline text unchanged, no new GL-ADRs.
- At the end: log, STATUS (ROADMAP/AGENTS/GUARDRAILS only if the baseline changes), views (`spectool render`), short summary with the points for the PO.

## Scope / acceptance criteria

- `oec/spec/requirements/res-mam.yaml`, `res-health.yaml`, `res-profile.yaml`, `res-publist.yaml`, `res-refdes.yaml`, `pag.yaml`, `loc.yaml` (`status: draft`); `areas.yaml` – the seven areas `drafted`.
- Inventory items: RES-MAM 64, RES-HEALTH 27, RES-PROFILE 18, RES-PUBLIST 61, RES-REFDES 43, PAG 28, LOC 3 (244 in total) – 0 open in `coverage/report.md`.
- Reference checks (`tools/internal/payload`) extended by the DataSet content of MAM, Health, Profile, PublicationList, ReferenceDesignation and the helper DataSets PaginationRequest, Pagination, Locale (strict/tolerant), tagged with the new REQ ids; tests.
- Golden payload vectors V-PAY-026 ff. (`draft`) for the not yet curated B1 examples of these resources. The Status events of B1.1.8/B1.1.10 belong to RES-EVENT and follow in C-028.
- New spec findings registered (register-only commit in the guideline repo, no re-baseline – ADR-0001 amendment C-018).
- `go vet`, `gofmt -l`, `go test ./...`, `spectool lint`, `render -check`, `coverage -check`, `topic -check`, `inventory -check -rev c6453c7` pass.

## Inputs (sources read, with revisions)

- Guideline `c6453c7`: MG 5, MG 7.4, MG 7.5 (index, 7.5.1), MG 8.2.3, MG 8.3 index, 8.3.1, 8.3.2, 8.3.7, 8.3.8, 8.3.10, 8.3.11 (+ 8.3.11.1, 8.3.11.2), 8.3.12, MG 9 index, 9.1 index, 9.1.1, 9.1.2, 9.1.7, 9.1.8, 9.1.10, MG B1.1.1, B1.1.2, B1.1.7, B1.1.8, B1.1.10, B1.1.11, B1.1.12, GL-ADR-018; register SF-056.
- Project: AGENTS, README, GUARDRAILS, STATUS, ROADMAP, OPEN-QUESTIONS, C-026 log incl. all addenda, `oec/spec/README.md`, `requirements/` (CMP, TOP, MSG, IL, LIFE), `matrix/method-resource.yaml`, `tools/internal/payload`, `vectors/payload/`.
