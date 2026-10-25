# Review und Entscheidungen – Erklärung auf Deutsch

_Stand: 25.10.2026, nach deinem Review von C-036 (Guideline `68d878d`)._

Diese Datei erklärt die offenen Punkte aus [`REVIEW.md`](REVIEW.md) auf Deutsch und in Alltagssprache. Sie ist eine Lesehilfe auf Wunsch des PO (Ausnahme von der Regel „Projektartefakte auf Englisch“). **Maßgeblich ist `REVIEW.md`**; dort stehen auch die Checkboxen. Die Nummern (D-1, R-1 …) sind in beiden Dateien gleich, du kannst also z. B. im Chat antworten: „D-1 wie vorgeschlagen, R-1 bis R-4 akzeptiert“.

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

**Nichts zum Reviewen.** Alle Einträge in `oec/spec` stehen auf `reviewed`. SF-099 ist entschieden; es fehlt nur noch dein Auftrag, es in die Guideline einzuarbeiten (siehe unten). Es gibt keine offene Frage für P2 (Q-011 ist geparkt). Es bleiben 48 Abdeckungslücken; die schließen die nächsten Zyklen. Mit C-037 (PublicationList und ReferenceDesignation) kommen neue Punkte dazu.

### Begriffe aus C-036

- **Action (Call/Reply):** eine Art Funktionsaufruf über MQTT. Eine Anwendung schickt einen **Call** an eine andere („gib mir eine neue DataSetWriterId“), die antwortet mit einem **Reply**. Diese Nachrichten haben einen eigenen `MessageType` (`ua-action-request` bzw. `ua-action-response`) und ein paar eigene Felder: `ResponseAddress` (wohin die Antwort soll), `TimeoutHint` (wie lange der Aufrufer wartet), `RequestorId` (wer fragt).
- **NewDataSetWriterId:** der einzige Standard-Dienst (Common Service) der Guideline. Eine Anwendung fragt damit eine neue, eindeutige `DataSetWriterId` für eine Ressource an. Er hat die reservierte `DataSetWriterId` 8.
- **Status (in der Antwort):** sagt, ob der Aufruf geklappt hat. `{}` heißt „Good“. Sonst steht ein OPC-UA-Code drin, z. B. `BadInvalidArgument`. Bei einem „Bad“-Code darf die Antwort keine Nutzdaten enthalten.
- **MAM:** das „Typenschild“ einer Anwendung oder eines Geräts (Hersteller, Modell, Seriennummer, Handbuch-Link …).
- **Profile:** die Liste der Ressourcen, die eine Anwendung anbietet (MAM, Health, Config …).
- **Umbrella (Schirm-REQ):** eine REQ, die eigentlich mehrere Einzelregeln zusammenfasst. Das Orakel meldet immer die Einzelregel. Deshalb zählt die Schirm-REQ als abgedeckt, wenn alle ihre Einzelregeln abgedeckt sind (so in ADR-0019 §9 beschlossen).


---

## Was aus deinem letzten Review (C-036) geworden ist

- **D-1 – SF-099 (leerer `Pub`):** so eingetragen, wie du es entschieden hast (Register-Commit `5491830`, Status `decided`):
  - Fragt jemand per `Get` etwas ab, das es bei einem **bekannten** Asset gerade nicht gibt, antwortet die Anwendung mit einer **leeren Liste** (`"Messages": []`).
  - Fragt jemand nach einem Asset, das die Anwendung **gar nicht kennt**, kommt **keine Antwort** auf dem `Pub`-Topic. Die Anwendung **sollte** stattdessen ein `Event` der Kategorie `Status` mit einem Fehlercode (`Bad`, z. B. `BadNotFound`) schicken.
  - Du hattest recht: Dieses Event-Verhalten steht schon in MG 9.1 „Resources Without Content“, und zwar für `Set`/`Del` auf etwas Unbekanntes und für `Get Metadata` ohne Daten. Für ein `Get` auf ein unbekanntes Asset fehlt dort nur ein Satz. Den Textvorschlag habe ich ins Register geschrieben.
  - Für die Schemas ändert sich nichts: Ein leerer `Pub` bleibt gültig. Ob überhaupt eine Antwort kommt, sieht man nur im laufenden Betrieb (D7 `monitor`).
  - **Bitte bestätigen:** Ich lese „es gibt nichts“ so, dass auch ein Filter, der bei einem bekannten Asset nichts trifft (z. B. ein unbekannter `Tag` bei `Get/Data`), eine leere Liste bekommt.
  - In die Guideline schreibe ich das erst auf deinen Auftrag (danach Re-Baseline und eine neue REQ für den Fall „unbekanntes Asset“).
- **D-2 – Nachtrag C-036 zu ADR-0019:** angenommen. Im ADR steht der Nachtrag jetzt als „accepted“. Punkt 1 beschreibt deine SF-099-Entscheidung.
- **D-3 – Aufteilung:** angenommen. C-037 ist PublicationList und ReferenceDesignation, der P2-Abschluss ist C-041.
- **D-4 – Heuristik für REQ-RES-MAM-006:** angenommen. Das Orakel bleibt, wie es ist.
- **R-1 bis R-4:** alles freigegeben. Die 51 Entwürfe stehen auf `reviewed`. Die Lücken bei der Vektor-Abdeckung sinken von 81 auf 48.

## Was aus dem Review davor (C-035) geworden ist

- **D-1 – Nachtrag zu ADR-0019:** angenommen. Im ADR steht der Nachtrag jetzt als „accepted“: `ResourceMetaData` für die vordefinierten MetaData, Eindeutigkeit von Namen in der Regelschicht (R-3) und die neue Regel R-6 für die erklärte Rolle.
- **D-2 – Metadata schon in C-035:** angenommen. Der Plan bleibt, wie er seit C-035 ist: C-037 enthält nur noch Data, Event und SubscriptionList.
- **D-3 – Topic-Regeln der Rollen:** Option (a). REQ-LIFE-002/006/008 werden weiter live geprüft (`d7-monitor`/`d7-test`). `validate --role` prüft nur den Inhalt.
- **R-1 bis R-4:** alles freigegeben. Die 31 Entwürfe stehen auf `reviewed`, auch REQ-MSG-038 ohne `schema`. Die Lücken bei der Vektor-Abdeckung sinken von 96 auf 77.
