# Hand-over prompt – cycle C-029 (RES-CONFIG)

Du arbeitest im Workspace am Projekt OEC v2.0, ich bin Thomas (PO). Deutsch mit mir, Artefakte Englisch. Lies AGENTS.md und folge der Lesereihenfolge (inkl. STATUS.md, neuestes Log C-028 `project/log/2026-10-20_C-028_data-meta-event-sublist-bus.md` samt allen Addenda 1–4, `oec/spec/README.md`). Prüfe die Guideline-Baseline (`5bc2e85`, Branch `feature/spec-findings`, GL-ADR-001 … 020, SF-001 … 083 resolved, keine offenen Findings, nächste freie Id SF-084).

Führe Zyklus C-029 aus wie in STATUS.md „Next step“:

Bereich RES-CONFIG (MG 8.3.3 inkl. 8.3.3.1/8.3.3.2, 9.1.3; 144 Inventory-Items): Requirements als draft, alle Inventory-Items der Sektionen gemappt oder in coverage/informative.yaml begründet, lint/render/coverage/topic grün, Area auf drafted. Die Aufteilung aus C-028 bleibt: C-029 enthält nur RES-CONFIG. Wird der Zyklus trotzdem zu groß, entscheide selbst über eine sinnvolle Aufteilung, plane sie in ROADMAP/STATUS ein und begründe sie im Log. Die RIG-Areas bleiben in C-030.

Beachte die bisherigen PO-Entscheidungen (SF-035 … SF-083, GL-ADR-018 … 020), insbesondere:
- SF-052: Sender strikt (REQ-MSG-001), Empfänger tolerant (REQ-MSG-046, SHOULD); Validierung in zwei Modi strict/tolerant (`payload.Mode`, Vektorfeld `mode`).
- SF-079/SF-081: Set/Del-Requests tragen die DataSetWriterId der adressierten Ressource; der `Filter` ist nur Pflicht, wo die Ressource einen definiert – für Set Config ja (Matrix: `filter: required`, `filter_rule: context-name`).
- GL-ADR-020: generischer Query (DataSetWriterId 9). Neue Query-Parameter für Config nur als SF-Vorschlag, nicht ohne meinen Auftrag.
- Status-Events nach Set gehören zu RES-EVENT; das Status-Event-Beispiel von B1.1.3 ist bereits V-PAY-076 (reviewed). Config-Requirements verweisen darauf, statt es zu wiederholen.

Alle Areas außer RES-CONFIG (und den geplanten RIG-Areas CNT, PLT, APP, LIC, REG, GEN) sind reviewed: CMP, IL, TOP, MSG, ACT, MQTT, LIFE, RES-MAM, RES-HEALTH, RES-PROFILE, RES-PUBLIST, RES-REFDES, PAG, LOC, QRY, RES-DATA, RES-META, RES-EVENT, RES-SUBLIST, BUS. Ebenso alle bisherigen Vektoren (203 Topic/IL, V-PAY-001 … 087) und die Topic- und Payload-Spalten der Method × Resource-Matrix. Änderungen daran nur mit Begründung im Log.

Payload-Spalten der Matrix (matrix/method-resource.yaml) für die drei erlaubten Config-Zeilen (Pub, Get, Set) nach dem Schema aus C-028: MessageType, DataSetWriterId- und DataSetClassId-Regel, Payload-Inhalt und Antwortverhalten (Pub als Antwort, Status-Event). Den Kommentar „Config rows get their payload columns with area RES-CONFIG (C-029)“ auflösen. Status draft, eigenes Review.

Golden Payload-Vektoren für die B1.1.3-Config-Beispiele (ohne das Status-Event, siehe V-PAY-076). Ablage in vectors/payload/, Status draft (G-3.9), nächste freie Vektor-Id V-PAY-088. Die drei JSON-Payloads der „Set explicit Config tag …“-Beispiele wurden mit SF-083 nur formal aus Tabellenzellen in Codeblöcke zurückgeführt; ihr Inhalt ist ungeprüft und wird jetzt gegen die Requirements geprüft. Die DataSet-Inhalte (Config-Struktur, `Context`/`Name`-Filter, Teilmengen bei Set, Kategorien/Gruppen/Parameter) werden gegen die RES-CONFIG-Requirements geprüft (Checker unter oec/spec/tools erweitern, Tests grün). Vektoren sind wortgleich zu den B1-Beispielen; neue Mängel als SF-084 ff. erfassen und im Vektor als edits festhalten (ADR-0018 Amendment C-013/C-023). Die Guideline-Korrektur nur nach meinem Auftrag.

Neue Unklarheiten als SF-084 ff. in oi4-oec-documentation/findings/SPEC-FINDINGS.md (Closed-Tabelle aufsteigend sortiert halten) mit Proposed interpretation eintragen. Den Guideline-Text nicht ohne meinen Auftrag ändern. Keine neuen GL-ADRs ohne meinen Auftrag. Tabellen- und Abbildungsnummern nach SF-083 verwenden (z. B. MG Table 13 NE107, A3 Table 18/19).

Am Ende: Log, STATUS (inkl. ROADMAP/AGENTS/GUARDRAILS falls Baseline sich ändert), Views (spectool render) und eine kurze Zusammenfassung mit den Punkten, die ich reviewen oder entscheiden soll. Commit-Regeln wie bisher (jede Änderung einzeln, lokal, nie pushen).

Stand: C-028 ist abgeschlossen und reviewed (inkl. SF-082/SF-083, Re-Baseline `5bc2e85`), kein Review offen; nichts ist mehr draft. Nächste freie Vektor-Id V-PAY-088, nächste freie SF-Id SF-084.
