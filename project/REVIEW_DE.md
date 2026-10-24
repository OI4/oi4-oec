# Review und Entscheidungen – Erklärung auf Deutsch

_Stand: 24.10.2026, nach deinem Review von C-034 (Guideline `68d878d`, Register `ff0d842`)._

Diese Datei erklärt die offenen Punkte aus [`REVIEW.md`](REVIEW.md) auf Deutsch und in Alltagssprache. Sie ist eine Lesehilfe auf Wunsch des PO (Ausnahme von der Regel „Projektartefakte auf Englisch“). **Maßgeblich ist `REVIEW.md`**; dort stehen auch die Checkboxen. Die Nummern (D-1, R-1 …) sind in beiden Dateien gleich, du kannst also z. B. im Chat antworten: „D-1 wie vorgeschlagen, R-1 bis R-5 akzeptiert“.

---

## Zuerst: ein paar Begriffe

- **REQ (Requirement):** eine einzelne, prüfbare Regel aus der Guideline, z. B. „`Messages` ist immer ein JSON-Array“. Jede REQ hat eine **Verifikationsmethode**, die angibt, *wer* sie prüft:
  - `schema` – das JSON Schema (D1) kann es an einer einzelnen Nachricht erkennen;
  - `d7-validate` – das Compliance-Tool D7 erkennt es offline an einer einzelnen Nachricht (Topic + Payload), notfalls mit einer kleinen Regelschicht neben dem Schema;
  - `d7-test` – D7 braucht dafür eine **laufende** Anwendung (z. B. „antwortet sie auf ein `Get`?“).
- **Vektor:** eine Beispielnachricht (Topic + JSON-Payload) mit der erwarteten Bewertung. **Positive** Vektoren sind gültig und zeigen, dass eine Regel eingehalten wird. **Negative** Vektoren enthalten genau einen Fehler und nennen die verletzte(n) REQ(s) in `violates`.
- **Orakel:** unser eigener Referenz-Prüfer in `spectool`. Er rechnet für jeden Vektor aus, welche REQs verletzt sind. `spectool lint` schlägt Alarm, wenn das nicht mit `violates` übereinstimmt. So können wir uns bei den Vektoren nicht „verschreiben“.
- **P2-Exit-Kriterium:** Jede REQ mit `schema` oder `d7-validate` braucht **mindestens einen positiven und einen negativen Vektor**, die du reviewt hast. Ein entworfener Vektor (`draft`) zählt erst nach deinem Review.
- **`draft` / `reviewed`:** Alles, was ich neu anlege oder an etwas bereits Reviewtem ändere, steht auf `draft`, bis du es freigibst.

---

## Aktuell offen

**Nichts.** Alle Einträge in `oec/spec` sind `reviewed`, es gibt kein offenes Spec-Finding und keine offene Frage für P2 (Q-011 ist geparkt). Mit C-035 (MetaData) kommen neue Punkte dazu.

## Was aus deinem letzten Review (C-034) geworden ist

- **D-1 – SF-098:** Ein `Del` enthält höchstens eine DataSetMessage, so wie es die Ressource in Kapitel 9.1 festlegt: keine bei `ReferenceDesignation`, genau den zu löschenden Eintrag bei `SubscriptionList`. Wie angekreuzt habe ich das in die Guideline eingearbeitet (MG 8.3.11, Commit `68d878d`, GL-ADR-019 ergänzt). Danach habe ich das Projekt auf diesen Stand umgestellt (Re-Baseline). Die Zahl der Inventory-Items bleibt 1397, weil der geänderte Satz kein Schlüsselwort enthält. REQ-PAG-009 sagt jetzt dasselbe.
- **D-2 – Q-020:** Option (a). Ein Vektor bekommt optional eine Rolle (`birth`/`close`/`will`), und D7 bekommt `validate --role`. So lassen sich Birth, Close und Will offline prüfen. Das baue ich in C-035, die CLI-Option selbst in C-039.
- **D-3 – Aufteilung:** bleibt so. C-035 ist MetaData, der P2-Abschluss ist C-040.
- **R-1 bis R-5:** Alles ist freigegeben. Die 32 Entwürfe stehen auf `reviewed`. Die Lücken bei der Vektor-Abdeckung sinken von 129 auf 96.
