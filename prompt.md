# Hand-over prompt – cycle C-027 (core resources + PAG + LOC)

Du arbeitest im Workspace am Projekt OEC v2.0, ich bin Thomas (PO). Deutsch mit mir, Artefakte Englisch. Lies AGENTS.md und folge der Lesereihenfolge (inkl. STATUS.md, neuestes Log C-026 `project/log/2026-10-19_C-026_sf057-060-guideline.md` samt allen Addenda, `oec/spec/README.md`). Prüfe die Guideline-Baseline (`c6453c7`, Branch `feature/spec-findings`, GL-ADR-001 … 018, SF-001 … 055 und SF-057 … 061 resolved, SF-056 open (B1-Beispiele, eingeplant für C-027/C-028, nicht blockierend), nächste freie Id SF-062).

Führe Zyklus C-027 aus wie in STATUS.md „Next step“:

Bereiche RES-MAM (MG 5, 8.3.1, 9.1.1), RES-HEALTH (MG 8.3.2, 9.1.2), RES-PROFILE (MG 8.3.7, 9.1.7), RES-PUBLIST (MG 8.3.8, 9.1.8), RES-REFDES (MG 8.3.10, 9.1.10), PAG (MG 8.3.11) und LOC (MG 8.3.12): Requirements als draft, alle Inventory-Items der Sektionen gemappt oder in coverage/informative.yaml begründet, lint/render/coverage/topic grün, Areas auf drafted.
Beachte die bisherigen PO-Entscheidungen (SF-035 … SF-061, GL-ADR-018) und insbesondere SF-052: Sender strikt (REQ-MSG-001), Empfänger tolerant (REQ-MSG-046, SHOULD); Validierung in zwei Modi strict/tolerant (`payload.Mode`, Vektorfeld `mode`). Die Areas CMP, IL, TOP, MSG, ACT, MQTT und LIFE und alle bisherigen Vektoren (203 Topic/IL, V-PAY-001 … 025) sind reviewed – Änderungen daran nur mit Begründung im Log.
Golden Payload-Vektoren für die B1-Beispiele der Ressourcen dieses Zyklus (MG B1.1.1 MAM, B1.1.2 Health, B1.1.7 Profile, B1.1.8 PublicationList, B1.1.10 ReferenceDesignation, B1.1.11 Pagination, B1.1.12 Locale – soweit noch nicht kuratiert) dürfen entstehen (vectors/payload/, edits gemäß ADR-0018 Amendment C-013/C-023, status draft, G-3.9); die DataSet-Inhalte werden gegen die Ressourcen-Requirements geprüft. Die Mängel der B1-Beispiele sind in SF-056 erfasst: im Vektor als edits festhalten; die Guideline-Korrektur gemäß Proposed interpretation von SF-056 nur nach meinem Auftrag. Die Payload-Spalten der Matrix kommen erst in C-028.
Neue Unklarheiten als SF-062 ff. in oi4-oec-documentation/findings/SPEC-FINDINGS.md (Closed-Tabelle aufsteigend sortiert halten) mit Proposed interpretation eintragen. Den Guideline-Text nicht ohne meinen Auftrag ändern. Keine neuen GL-ADRs ohne meinen Auftrag.
Am Ende: Log, STATUS (inkl. ROADMAP/AGENTS/GUARDRAILS falls Baseline sich ändert), Views (spectool render) und eine kurze Zusammenfassung mit den Punkten, die ich reviewen oder entscheiden soll.
Commit-Regeln wie bisher (jede Änderung einzeln, lokal, nie pushen).

Stand: C-026 ist abgeschlossen, kein Review offen; nichts ist mehr draft. Nächste freie Vektor-Id V-PAY-026.
