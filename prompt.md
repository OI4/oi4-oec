# Hand-over prompt – cycle C-037 (P2: PublicationList + ReferenceDesignation)

Du arbeitest im Workspace am Projekt OEC v2.0, ich bin Thomas (PO). Lies AGENTS.md und folge der Lesereihenfolge (neuestes Log: `project/log/2026-10-25_C-036_actions-mam-profile.md` inkl. Addenda 1–2; ADR-0019 mit den Amendments C-035 und C-036). Prüfe die Baseline: Guideline `a42a132` auf `feature/spec-findings` (Branch-Head `d4f79f3`, nur `findings/`), SF-001 … 099 resolved, keine offenen Findings, 1398 Inventory-Items, 437 REQs, 159 `schema`/`d7-validate`-REQs mit 48 Lücken, Entry-Schemas 11 von 29. Nächste IDs: SF-100, REQ-CMP-048, V-PAY-218, V-TOP-254, ADR-0020, Q-021. Alle Checks müssen grün sein:
- in `oec/spec/tools` (`GOTOOLCHAIN=auto go run ./cmd/spectool … -spec ..`): `lint`, `render -check`, `coverage -check`, `topic -check`, `inventory -check -rev a42a132` und `go test ./...`;
- `oec/schema/check.sh` (santhosh + jsonschema-rs, 42 Schema-Dateien, 375 Fälle, 0 Abweichungen).

**Review aus C-036 Addendum 2** (siehe `project/REVIEW.md`):
- R-1 REQ-CMP-047: [Antwort]
- R-2 Änderung REQ-CMP-041: [Antwort]
- R-3 Guideline-Text SF-099 (`a42a132`): [Antwort]

Arbeite die Antworten zuerst ein, genauso wie die früheren Reviews: `spectool review <ids>` als eigener Commit, Log-Addendum 3 im C-036-Log, STATUS/ROADMAP, REVIEW.md/REVIEW_DE.md.

Führe dann Zyklus C-037 aus, wie in STATUS.md „Next step“ 1 und ROADMAP P2 beschrieben: **PublicationList und ReferenceDesignation** (Pub/Get/Set/Del, die 8 Topic-Formen; Bereiche RES-PUBLIST und RES-REFDES).

1. **Orakel** (`spectool`) für die Payload-REQs der beiden Ressourcen, soweit es noch fehlt. Vorbild sind C-035/C-036: neue Prüfungen im Orakel, Umbrella-REQs nach ADR-0019 §9 (`coverage/implications.yaml`).
2. **Schemas** in `oec/schema/v2/`: Ressourcen-Module für PublicationList und ReferenceDesignation sowie die Entry-Schemas `pub-`/`get-`/`set-`/`del-publicationlist` und `…-referencedesignation`. Dazu `message.schema.json`-Dispatch, die Harness-Routen und Probes ab P-45; danach `dist/` neu erzeugen.
   - Dabei beachten: GL-ADR-018 (kompletter Filter als WriterGroupName), GL-ADR-020 (`ChildrenRequest`, generische `Query`), SF-098 (`Del ReferenceDesignation` ohne DataSetMessage) und SF-099 (leerer `Pub` als Antwort auf `Get` ohne Inhalt ist gültig).
3. **Vektoren** ab V-PAY-218 (Status `draft`), sodass jede `schema`/`d7-validate`-REQ der beiden Bereiche einen positiven und einen negativen Vektor hat. Offen sind heute nur negative Vektoren für REQ-RES-PUBLIST-001/003/019/022 (021 als Umbrella) und REQ-RES-REFDES-001/003/006/007/012/013 (005 als Umbrella). Erwartete Lücken nach dem Review im Log vorrechnen.
4. Architekturentscheidungen innerhalb von ADR-0019 als **Amendment C-037** (Status proposed). Neue ADRs nur, wenn wirklich nötig.

Ist der Zyklus zu groß, teile ihn selbst auf und begründe das im Log (ROADMAP/STATUS anpassen).

- Reviewte Items (REQs, Vektoren, Matrix) nur mit Begründung im Log ändern; geänderte Items gehen zurück auf `draft`, außer bei rein mechanischen Remaps.
- Unklarheiten in der Spec als SF-100 ff. mit Proposed interpretation ins Register (`oi4-oec-documentation/findings/SPEC-FINDINGS.md`). Guideline-Text und GL-ADRs nur nach meinem Auftrag.
- Ändert sich der Guideline-Commit, zuerst re-baselinen (STATUS „Next step“ 3). `spectool rebaseline` liest den Working Tree: Ist ein reiner Register-Commit Branch-Head, die Text-Revision kurz detached auschecken und danach zurück auf den Branch.
- Im Guideline-Repo ist `.yarn/install-state.gz` lokal geändert – nicht committen.
- Optionaler dritter Validator, informativ: Python `jsonschema` + `referencing` in einem eigenen venv gegen `oec/schema/dist/cases.json` (alle `*.schema.json` aus `dist/` in eine Registry laden, jeden Fall über `{"$ref": case.schema}` prüfen).
- `legacy/` nicht anfassen und keinen Code daraus übernehmen (GUARDRAILS G-1.3).
- Am Ende:
  - Log `project/log/2026-10-xx_C-037_….md`, STATUS, ROADMAP und Views.
  - `project/REVIEW.md` (Englisch) und `project/REVIEW_DE.md` (Deutsch, verständlich erklärt) mit allen Review- und Entscheidungspunkten (D-x/R-x).
  - Eine kurze deutsche Zusammenfassung mit den Punkten, die ich reviewen oder entscheiden soll.
  - Commits wie bisher, einzeln, nie pushen.
