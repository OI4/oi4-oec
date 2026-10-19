# Hand-over prompt – cycle C-025 (MQTT + LIFE)

Du arbeitest im Workspace am Projekt OEC v2.0, ich bin Thomas (PO). Deutsch mit mir, Artefakte Englisch. Lies AGENTS.md und folge der Lesereihenfolge (inkl. STATUS.md, neuestes Log C-024 `project/log/2026-10-19_C-024_sf045-051-guideline.md` samt beiden Addenda, `oec/spec/README.md`). Prüfe die Guideline-Baseline (`2f00e9f`, Branch `feature/spec-findings`, GL-ADR-001 … 018, SF-001 … 055 resolved, SF-056 open (B1-Beispiele, eingeplant für C-026/C-027, nicht blockierend), nächste freie Id SF-057).

Führe Zyklus C-025 aus wie in STATUS.md „Next step“:

Bereiche MQTT (MG 6) und LIFE (MG 6.3.3, 6.3.4, 6.3.5): Requirements REQ-MQTT-/REQ-LIFE- als draft, alle Inventory-Items der Sektionen gemappt oder in coverage/informative.yaml begründet, lint/render/coverage/topic grün, Areas auf drafted.
Beachte die bisherigen PO-Entscheidungen (SF-035 … SF-055, GL-ADR-018) und insbesondere SF-052: Sender strikt (REQ-MSG-001), Empfänger tolerant (REQ-MSG-046, SHOULD); Validierung in zwei Modi strict/tolerant (`payload.Mode`, Vektorfeld `mode`). Die Areas CMP, IL, TOP, MSG und ACT und alle bisherigen Vektoren sind reviewed – Änderungen daran nur mit Begründung im Log.
Golden Payload-Vektoren für Birth/Close/Will (MG B1.1.2, MG 6.3.x) dürfen im Zyklus entstehen (vectors/payload/, edits gemäß ADR-0018 Amendment C-013/C-023, status draft, G-3.9). Die Payload-Spalten der Matrix kommen erst in C-027; die Mängel der B1-Beispiele sind in SF-056 erfasst und gehören zu den RES-Zyklen (C-026/C-027) – für Birth/Close/Will nur Abweichungen als edits festhalten und melden, wenn etwas MQTT/LIFE blockiert.
Neue Unklarheiten als SF-057 ff. in oi4-oec-documentation/findings/SPEC-FINDINGS.md (Closed-Tabelle aufsteigend sortiert halten) mit Proposed interpretation eintragen. Den Guideline-Text nicht ohne meinen Auftrag ändern. Keine neuen GL-ADRs ohne meinen Auftrag.
Am Ende: Log, STATUS (inkl. ROADMAP/AGENTS/GUARDRAILS falls Baseline sich ändert), Views (spectool render) und eine kurze Zusammenfassung mit den Punkten, die ich reviewen oder entscheiden soll.
Commit-Regeln wie bisher (jede Änderung einzeln, lokal, nie pushen).

Stand: C-024 ist abgeschlossen, kein Review offen; alle 22 Payload-Vektoren (V-PAY-001 … 022) sind reviewed. Nächste freie Vektor-Id V-PAY-023.
