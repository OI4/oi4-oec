# Hand-over prompt – cycle C-033 (P2 start: JSON Schema)

Du arbeitest im Workspace am Projekt OEC v2.0, ich bin Thomas (PO). Lies AGENTS.md und folge der Lesereihenfolge (neuestes Log: `project/log/2026-10-22_C-032_p1-exit.md` inkl. Addenda 1–53). Prüfe die Baseline (`5b8fe59` auf `feature/spec-findings`, keine offenen Findings, nächste SF-097, nächster Vektor V-PAY-127, nächstes ADR-0019, 1396 Inventory-Items, Coverage `open=0`, alle fünf `spectool`-Checks grün).

Führe Zyklus C-033 aus wie in STATUS.md „Next step“ 1 und ROADMAP P2: P1 ist abgeschlossen, jetzt startet P2 – JSON Schema (D1).

1. **P2 planen:** Inkremente und Zyklen bis zum P2-Exit festlegen (ROADMAP P2 und STATUS anpassen). Zum Exit gehören die 21 `schema`/`d7-validate`-REQs ohne Vektor aus der Gap-Liste im C-031-Log („Gap list for P2/D7“).
2. **Schema-Architektur-ADR (ADR-0019)** schreiben, Status `proposed`. Zu entscheiden sind:
   - JSON Schema 2020-12; modulare `$defs` (OPC-UA-Basistypen, Envelopes, OI4-Ressourcen, Services/Actions); `unevaluatedProperties: false` für OI4-definierte Objekte;
   - Diskriminierung über `MessageType`/`DataSetClassId`;
   - topic-aware Validierung „message = topic + payload“ (Schema plus kleine Regelschicht vs. Schema pro Topic-Pattern) – Optionen mit Vor-/Nachteilen und einer Empfehlung;
   - stabiles `$id`-Schema und Versionierung; Ablageort im Monorepo `oec/` (ADR-0012);
   - Testsuite: bestehende Vektoren (`oec/spec/vectors`) als Eingabe, CI mit mindestens zwei unabhängigen 2020-12-Validatoren;
   - Zuordnung der REQ-Verifikation `schema` vs. `d7-validate` (was das Schema abdeckt, was die Regelschicht von D7 `validate` übernimmt).
3. Ein kleiner Proof of Concept zur Absicherung des ADR ist erlaubt, z. B. NetworkMessage/DataSetMessage-Envelope plus `Health` gegen die vorhandenen Vektoren. Der vollständige Schema-Ausbau erst, wenn ich das ADR angenommen habe.

Ist der Zyklus zu groß, teile ihn selbst auf und begründe das im Log (ROADMAP/STATUS anpassen).

- Reviewte Items (REQs, Vektoren, Matrix) nur mit Begründung im Log ändern.
- Unklarheiten in der Spec als SF-097 ff. mit Proposed interpretation ins Register (`oi4-oec-documentation/findings/SPEC-FINDINGS.md`). Guideline-Text und GL-ADRs nur nach meinem Auftrag. Ändert sich der Guideline-Commit, zuerst re-baselinen (STATUS „Next step“ 3).
- `legacy/` nicht anfassen und keinen Code daraus übernehmen (GUARDRAILS G-1.3).
- Am Ende: Log (`project/log/2026-10-xx_C-033_….md`), STATUS, ROADMAP und Views. Dann eine kurze Zusammenfassung mit den Punkten, die ich reviewen oder entscheiden soll (insbesondere die ADR-0019-Optionen). Commits wie bisher, einzeln, nie pushen.
