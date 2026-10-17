Du arbeitest im Workspace am Projekt OEC v2.0, ich bin Thomas (PO). Deutsch mit mir, Artefakte Englisch.
Lies AGENTS.md und folge der Lesereihenfolge (inkl. STATUS.md, neuestes Log C-022, oec/spec/README.md).
Prüfe die Guideline-Baseline (fd9da6e, Branch feature/spec-findings, GL-ADR-001 … 018, kein offenes Spec Finding, nächste freie Id SF-045).

Führe Zyklus C-023 aus wie in STATUS.md „Next step“:
- Bereiche MSG (MG 8, MG A2) und ACT (MG 8.2.16 … 8.2.20, MG 8.4, MG 9.2): Requirements REQ-MSG-/REQ-ACT- als draft, alle Inventory-Items der Sektionen gemappt oder in coverage/informative.yaml begründet, lint/render/coverage grün, Areas auf drafted.
- Beachte GL-ADR-018 (WriterGroupName von PublicationList-/SubscriptionList-Einträgen = <ResourceType>[/<Tag>]) und die bisherigen PO-Entscheidungen (SF-035 … SF-044).
- Golden Payload-Vektoren aus Appendix B dürfen schon starten (edits gemäß ADR-0018 Amendment C-013, status draft, G-3.9); die Payload-Spalten der Matrix kommen erst in C-026.
- Neue Unklarheiten als SF-045 ff. in oi4-oec-documentation/findings/SPEC-FINDINGS.md mit Proposed interpretation eintragen. Den Guideline-Text nicht ohne meinen Auftrag ändern.
- Am Ende: Log, STATUS, Views (spectool render) und eine kurze Zusammenfassung mit den Punkten, die ich reviewen oder entscheiden soll.

Commit-Regeln wie bisher (jede Änderung einzeln, nie pushen).
