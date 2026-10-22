# C-031 – Golden vectors, coverage report and consistency review

- Date: 2026-10-22
- Phase: P1
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Status: done, awaiting PO review (C-032)
- Guideline baseline: `oi4-oec-documentation` @ `b942df3`, branch `feature/spec-findings` (GL-ADR-001 … 022, SF-001 … 090, 092, 093 resolved, SF-091 decided but not implemented, next free id SF-094). During the cycle the register-only commit `5ef2cc1` added SF-094/095 (open). The guideline text did not change, so there was no re-baseline.

## Intent

PO order (chat, 2026-10-22, after the C-030 addendum 3): "Starte danach direkt mit C-031 soweit für C-030 alles abgeschlossen ist". C-031 is the ROADMAP P1 cycle "golden vectors + coverage report". The P1 exit review by the PO follows as C-032.

## Plan

**No split.** The area work is finished: all 27 areas are reviewed and the inventory coverage is complete (1382 items, 0 open, checked at cycle start). What remains is to:

1. Make the coverage of the **guideline examples** measurable. P1 exit requires "all Appendix B examples" as golden vectors (ADR-0018 amendment C-013), and until now nothing showed which code blocks of MG and RIG are covered by a vector.
2. Build golden vectors for the concrete examples that are still missing.
3. Run a **consistency review** of the whole catalog (status, deprecated references, vector gaps, verification/domain, duplicates).
4. Register new findings (SF-094 ff.), then write the log, STATUS/ROADMAP and views.

## Scope / acceptance criteria (C-031)

- [x] Every fenced code block of MG (incl. Appendix B) and RIG is covered by a golden payload vector or explained in `coverage/examples.yaml`; `coverage -check` fails on open blocks.
- [x] Concrete MG body examples are golden vectors (V-PAY-117 … 121, `draft`).
- [x] Consistency review done: findings fixed or recorded below.
- [x] New findings SF-094/095 in the register (open, register only, no re-baseline); guideline text unchanged, no GL-ADR.
- [x] `lint`, `render -check`, `coverage -check`, `topic -check`, `inventory -check -rev b942df3`, `go vet`, `gofmt`, `go test` green.

## Inputs

- Guideline `b942df3` (MG incl. Appendix A/B, RIG); register `5ef2cc1`.
- `oec/spec/` at the C-030 closure (`12ec6cb`): 434 REQs, 306 informative items, payload vectors V-PAY-001 … 116.
- ADR-0018 incl. amendments C-013 (golden examples), C-023 (payload vectors) and C-024.

## Work

### 1. Examples coverage (tooling)

New package `oec/spec/tools/internal/examples` (with unit tests), wired into `spectool coverage`:

- Extracts all fenced code blocks of MG and RIG. Each block gets a content-derived key `<Doc>-<Section>-<8 hex>` (same scheme as the inventory keys).
- **Covered:** a golden payload vector with the same `origin.ref` whose payload (or `original`) is contained in the whitespace-normalized block. **Fragment:** the block (or `{`+block+`}` for member fragments such as `"Payload": {…}` in MG 6.3.4/6.3.5) parses as JSON and is deep-contained in a golden vector.
- Every other block needs an explanation in `coverage/examples.yaml` (+ `examples.schema.json`): keys plus a reason.
- `coverage -check` fails on open blocks, stale keys, duplicate keys, explanations of covered blocks, and a baseline that differs from the inventory. `rebaseline` updates the baseline of `examples.yaml`.
- `coverage/report.md` gets the section "Guideline examples".

ADR-0018 amendment C-031 records these agent decisions (G-4.4).

Result at `b942df3`:

| Part | Blocks | Golden | Explained | Open |
|------|-------:|-------:|----------:|-----:|
| MG (body) | 42 | 8 | 34 | 0 |
| MG Appendix B | 98 | 92 | 6 | 0 |
| RIG | 4 | 0 | 4 | 0 |
| **Total** | **144** | **100** | **44** | **0** |

The explained blocks are:

- **MG body:** message templates with `<…>` placeholders (C-012 §5, amendment C-013), the base64url implementation snippets of MG 4.2 (Python, JavaScript, C#; covered by the dnp-encoding/IL vectors) and the command-line output of MG 1.2 (SF-095 (a)).
- **MG Appendix B:** the six B2 DataSet examples that differ from B1.1 (SF-094).
- **RIG:** deployment snippets (Docker Compose mounts of RIG 4.1.3, the `docker run` line of RIG 4.2 (SF-095 (c)), `docker network ls` output of RIG 4.4). D8 checks these; they are not payload vectors.

### 2. Golden vectors from the MG body (`vectors/payload/guideline.yaml`, `draft`)

| Vector | Ref | Content | Note |
|--------|-----|---------|------|
| V-PAY-117 | MG 1.2 | `Pub` Health reply of the convention example | Topic completed (edit `completed`); DSWId 3875 differs from B1.1 (SF-095 (b)) |
| V-PAY-118 | MG 8.1.1 | NetworkMessage example with an `https` Identification Link as AppId | Topic completed |
| V-PAY-119 | MG 8.1.3 | `Call` with `CorrelationData` | Topic verbatim in the text |
| V-PAY-120 | MG 8.1.3 | `Reply` with `CorrelationData` | Topic verbatim in the text |
| V-PAY-121 | MG 8.1.2 Figure 6 | `Pub` Metadata (`ua-metadata`) of a temperature sensor | Topic completed. Also covers the KeyValuePair fragment of MG 8.2.8. Figure 6 was not classified as a concrete message before C-031. |

V-PAY-117, 118 and 121 are the first vectors that use a topic edit (`{target: topic, kind: completed}`, ADR-0018 amendment C-023 item 2).

### 3. Spec findings (register `5ef2cc1`, open, editorial)

- **SF-094** – MG B2.2, B2.3, B2.5 … B2.8: the example DataSets of the B2 MetaData pages differ from the B1.1 payloads they refer to. Proposed interpretation: replace them with the B1.1 payloads:
  - B2.2 → V-PAY-004
  - B2.3 → V-PAY-091
  - B2.5 → V-PAY-032
  - B2.6 → V-PAY-037
  - B2.7 → V-PAY-069
  - B2.8 → V-PAY-043
- **SF-095** – (a) The MG 1.2 "line numbering" convention example is a PowerShell session tagged `json`, without line numbers. (b) MG 1.2 uses DSWId 3875 instead of 3456 of B1.1. (c) The RIG 4.2 `docker run` example is tagged `json` instead of `bash`. Proposed interpretation: correct the text and the tags.

Both are editorial and do not block the P1 exit. The catalog does not depend on them; the explanations in `examples.yaml` reference them.

### 4. Consistency review

I ran an ad-hoc review script (session file, not committed) over all catalog files.

| Check | Result | Action |
|-------|--------|--------|
| (a) References to deprecated REQs | REQ-CMP-022 ("Data ⇒ Metadata", deprecated since SF-037, replaced by REQ-CMP-037) is still cited by REQ-RES-DATA-001 (statement) and as the condition of REQ-RES-META-002/003. REQ-CMP-037 itself cites it legitimately ("Replaces …"). | Fixed, see "Changes to reviewed items" below. |
| (b) Status | 434 REQs: 422 reviewed, 9 draft, 3 deprecated; 27 areas reviewed. | – |
| (c) `schema`/`d7-validate` REQs without a vector | 26 at cycle start. Five of them have a reference check in `tools/internal/payload` but no vector. | Constructed invalid vectors **V-PAY-122 … 126** (`draft`, `constructed.yaml`) for REQ-MSG-014 (reserved DataSetClassId; the checker also reports REQ-MSG-010, the wrong class id for Health), REQ-PAG-001 (PaginationRequest `PerPage` 0, `Page` missing), REQ-PAG-003 (`HasNext` not Boolean), REQ-PAG-008 (`Pagination` with `Get`) and REQ-RES-HEALTH-005 (`Get` Health with a Health object). **21 remain** – see "Gap list for P2/D7". |
| (d) Domain vs verification | Messaging REQs are not verified with d8 alone. `platform`/`service-container` REQs with d8 are correct. REQ-MQTT-013 (ClientId = container name, domain messaging, `d8`+`manual`) is fine: the container name is only known on the infrastructure side. | none |
| (g) Duplicate titles | REQ-RES-PUBLIST-018 / REQ-RES-SUBLIST-006 and REQ-RES-PUBLIST-025 / REQ-RES-SUBLIST-014: parallel rules of the two list resources. | Accepted |
| (h) Inventory keys mapped by several REQs | 24 keys: one guideline sentence with several testable rules, or an umbrella REQ (level tables) plus its details. | Accepted |
| (i) REQs without inventory key | REQ-ACT-013 (MG 8.2.17/8.2.18) and REQ-ACT-023 (MG 8.4.1): rules derived with SF-035 without an own inventory sentence; refs present, reviewed. | Accepted |

**Gap list for P2/D7** (`schema`/`d7-validate` REQs without a vector after C-031):

- Umbrella REQs whose details have vectors: REQ-CMP-004/005/025/026, REQ-TOP-002, REQ-IL-001/008.
- Rules the reference checker does not implement yet, to be covered by the P2 schemas (P2 exit: positive and negative vector per schema-testable REQ):
  - REQ-ACT-013;
  - REQ-IL-005;
  - REQ-MSG-004/017;
  - REQ-MSG-042/043/045 (deep structure of the data type descriptions, SF-050);
  - REQ-PAG-009;
  - REQ-RES-CONFIG-015;
  - REQ-RES-DATA-003;
  - REQ-RES-HEALTH-007;
  - REQ-RES-MAM-006/014;
  - REQ-TOP-018.

This is consistent with ROADMAP P2: P1 requires golden examples, and P2 requires a positive and a negative vector per schema-testable REQ.

## Changes to reviewed items

| Item | Change | Reason | Status |
|------|--------|--------|--------|
| REQ-RES-DATA-001 | "… and are described by the Metadata of the same Source and Tag (REQ-CMP-022)" → "… When the component supports the resource Metadata (REQ-CMP-037), the fields are described by the Metadata of the same Source and Tag." | REQ-CMP-022 is deprecated (SF-037). Metadata is L1 none, L2 SHOULD, L3 MUST (REQ-CMP-037), so Data at L1/L2 does not require Metadata. | → `draft` (semantic correction) |
| REQ-RES-META-002, REQ-RES-META-003 | Condition "The component publishes Data (REQ-CMP-022)." → "The component publishes Data and supports the resource Metadata (REQ-CMP-036, REQ-CMP-037)." | Same reason. With the old condition both REQs (levels L1–L3 MUST) would require Metadata from every L1 publisher of Data. | → `draft` |
| `res-data.yaml`, `res-meta.yaml` header comments | Support line points to REQ-CMP-037 | Same reason | – |

The new conditions narrow the scope and do not add obligations.

## Commits

Workspace:

- `e16d12a` – V-PAY-117 … 121 and regenerated views.
- `829f6e0` – examples coverage tooling, `coverage/examples.{yaml,schema.json}`, report, README, ADR-0018 amendment C-031.
- `74e4ecd` – consistency review: REQ-CMP-022 references, V-PAY-122 … 126, views.
- This log with STATUS, ROADMAP, AGENTS and GUARDRAILS (next commit).

Guideline: `5ef2cc1` (register: SF-094/095 open, next free SF-096).

## Checks (final)

- `spectool lint`: ok – 27 area files, 434 REQs, 306 informative items, 66 matrix combinations; vectors topic=146, identification-link=29, dnp-encoding=15, oi4identifier=13, payload=126.
- `render -check`, `coverage -check` (inventory 0 open, examples 0 open), `topic -check`, `inventory -check -rev b942df3`: ok.
- `go vet ./...`, `gofmt -l .`, `go test ./...`: ok.

## For the PO (C-032 – P1 exit review)

1. **Golden vectors V-PAY-117 … 121** (`vectors/view/payload-guideline.md`) and the topic edits of V-PAY-117/118/121.
2. **Constructed vectors V-PAY-122 … 126** (`vectors/view/payload-constructed.md`).
3. **Examples coverage:** section "Guideline examples" in `coverage/report.md` and the reasons in `coverage/examples.yaml`; ADR-0018 amendment C-031.
4. **REQ-RES-DATA-001, REQ-RES-META-002/003** (REQ-CMP-022 references, `draft`).
5. **SF-094/095** (editorial): accept the proposed interpretations and order the guideline correction?
6. **SF-091**: order the guideline implementation (decided 2026-10-22).
7. C-030 follow-up drafts REQ-CNT-019/020/021, REQ-REG-002/007, REQ-MQTT-007; guideline commits `15115e8`, `527d8d8`, `e7f5459`, `897c1cc`; the C-030 agent decisions (C-030 log, addendum 3).
8. Gap list for P2/D7 (21 REQs without a vector): agree that they move to the P2 exit criterion.
9. After that: P1 exit (ROADMAP P1 exit criteria) → P2 JSON Schema.
