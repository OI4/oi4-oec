# Review und Entscheidungen – Erklärung auf Deutsch

_Stand: 26.10.2026, nach Zyklus C-037 (PublicationList und ReferenceDesignation)._

Diese Datei erklärt die offenen Punkte aus [`REVIEW.md`](REVIEW.md) auf Deutsch und in Alltagssprache. Sie ist eine Lesehilfe auf Wunsch des PO (Ausnahme von der Regel „Projektartefakte auf Englisch“). **Maßgeblich ist `REVIEW.md`**; dort stehen auch die Checkboxen. Die Nummern (D-1, R-1 …) sind in beiden Dateien gleich, du kannst also z. B. im Chat antworten: „D-1 und D-2 wie vorgeschlagen, R-1 und R-2 akzeptiert“.

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

2 Entscheidungen (D-1, D-2) und 13 neue Testvektoren (R-1, R-2). Es gibt kein offenes Spec-Finding und keine offene Frage (Q-011 bleibt geparkt). Details stehen im [C-037-Log](log/2026-10-26_C-037_publist-refdes.md).

**Worum ging es?** In C-037 habe ich die JSON Schemas für zwei weitere Ressourcen gebaut:
- **PublicationList:** die Liste, *was* eine Anwendung *wann* veröffentlicht. Jeder Eintrag nennt die Ressource (z. B. `Data`), die Quelle (`Source`), optional einen Filter (z. B. einen bestimmten Tag) und den Modus (`Mode`, z. B. „nur auf Anfrage“ oder „zyklisch“).
- **ReferenceDesignation:** die Referenzkennzeichnung nach IEC 81346, also „wo sitzt dieses Asset in der Anlage, welche Funktion und welches Produkt ist es“.

Dazu gehören 7 neue Einstiegs-Schemas (jetzt **18 von 29** Topic-Formen). Geplant waren „8 Topic-Formen“. Es sind aber nur 7, weil es kein `Del PublicationList` gibt; das habe ich in ROADMAP/STATUS korrigiert. An der Spec ändert sich dadurch nichts.

**Zahlen:** Alle Prüfungen sind grün. Drei unabhängige Schema-Validatoren (Go, Rust, Python) kommen bei allen 439 Testfällen zum gleichen Ergebnis. Es gibt heute 48 Abdeckungslücken. Gibst du R-1 und R-2 frei, sind es noch **35**. Die übrigen 35 gehören zu Data, Event, SubscriptionList, Config und ein paar allgemeinen Regeln (C-038 … C-040).

### D-1 – Nachtrag C-037 zu ADR-0019 (Schema-Architektur)

Wie bei jedem Zyklus habe ich die Bau-Entscheidungen als Nachtrag ins ADR-0019 geschrieben. Inhaltlich:

1. **Filter in der PublicationList:** Der Filter eines Eintrags wird geprüft, siehe D-2.
2. **Erlaubte Modi je Ressource:** Manche Ressourcen erlauben nur bestimmte Modi. Bei `Event` gibt es z. B. nur „aus“ oder „gefiltert“, bei `Metadata` nur „auf Anfrage mit Filter“. Das Schema verbietet die anderen Modi. Fehlt der Modus, gilt der Standard „auf Anfrage“; das ist bei diesen Ressourcen ein Fehler.
3. **Zählen bei Set und Del:** Ein `Set` muss **genau ein** Objekt enthalten (die neue Liste bzw. die neue Kennzeichnung). Ein `Del ReferenceDesignation` enthält **gar kein** Objekt (so hast du es in SF-098 entschieden). In `Set`/`Del` ist außerdem keine `SequenceNumber` erlaubt.
4. **Regelschicht:** Drei Dinge kann ein Schema nicht prüfen. Das macht die kleine Regelschicht von D7:
   - Eine ReferenceDesignation-Abfrage (`Query`) ist nur auf dem Topic **ohne** Source erlaubt. Das Schema sieht nur die Nachricht, nicht das Topic, und beide Topics haben dieselbe Grammatik-Regel.
   - Kein Eintrag darf doppelt in der PublicationList stehen. Gemeint ist: gleiche Ressource, Source und Filter. Das Schema kann nur „ganz gleiche“ Einträge erkennen, nicht „gleich im Schlüssel, anders im Modus“.
   - `WriterGroupName` und `DataSetWriterName` müssen zum Eintrag passen. Das Schema prüft nur, dass der `WriterGroupName` mit einem Ressourcennamen beginnt.
5. **Weiche in `message.schema.json`:** Das allgemeine Nachrichten-Schema erkennt die beiden Ressourcen an ihrer `DataSetClassId`, wie schon MAM, Health und Profile. Ob `Pub`, `Get`, `Set` oder `Del` gemeint ist, sieht man nur am Topic. Deshalb stehen die Zählregeln aus Punkt 3 in den Einstiegs-Schemas.

Keine frühere Entscheidung wird dadurch geändert. **Empfehlung:** annehmen.

### D-2 – Wie das Orakel den Filter eines PublicationList-Eintrags liest

- **Frage:** Steht der Filter im Eintrag so wie im Topic, also **codiert** (z. B. `o%2Be` für den Tag `o+e`)? Oder steht dort der „rohe“ Tag?
- **Meine Lesart:** codiert, genau wie im Topic. Die Guideline nennt ihn „den Filter der Veröffentlichung“ (MG 8.3.8). Nach GL-ADR-018 ist der `WriterGroupName` außerdem `<Ressource>/<Filter>` mit dem kompletten Topic-Filter. Darum prüft das Orakel zwei Dinge:
  - **Codierung:** Kein rohes `+`, `#`, `/` oder `%`. Erlaubt sind nur `%2B`, `%23`, `%2F`, `%25`. Ein Verstoß verletzt REQ-RES-PUBLIST-001.
  - **Aufbau:** Bei Data/Metadata/Config hat der Filter **eine** Ebene. Bei Event hat er `<Kategorie>/<Level>`, z. B. `Syslog/Error` oder `Status/Bad`. Ein Verstoß verletzt REQ-RES-PUBLIST-002.
  - Außerdem: Ein Modus als Zahl (`8` statt `APPLICATION_SOURCE_FILTER_8`) ist ein Fehler. Das gilt bei allen Aufzählungen, die als Text geschrieben werden.
- **Auswirkung:** Kein bereits freigegebener Vektor ändert sein Ergebnis. Neu erkannt werden V-PAY-219 (roher `+`) und V-PAY-223 (`Status/Fine` gibt es nicht).
- **Alternative:** Im Eintrag steht der rohe Tag. Dann würde nur der Aufbau geprüft, und V-PAY-219 fiele weg. Das passt aber nicht zu GL-ADR-018. Deshalb habe ich kein neues Spec-Finding angelegt.
- **Empfehlung:** annehmen.

### R-1 – 7 PublicationList-Vektoren (V-PAY-218 … 224)

Jeder Vektor ist eine Beispielnachricht aus der Guideline mit **genau einem** eingebauten Fehler:
- Eintrag ohne `Source`;
- Filter `o+e` nicht codiert;
- derselbe Eintrag doppelt (erkennt die Regelschicht);
- ein `Get` mit Inhalt (ein `Get` muss leer sein);
- ein `Set` ohne Eintrag;
- ein Event-Filter `Status/Fine` (gibt es nicht);
- der Modus als Zahl.

Die Tabelle mit Links steht in [`REVIEW.md`](REVIEW.md#r-1--publicationlist-vectors-v-pay-218--224-draft).

### R-2 – 6 ReferenceDesignation-Vektoren (V-PAY-225 … 230)

Wieder je genau ein Fehler:
- ein Aspekt (Function) ohne Wert;
- ein `Get` mit Inhalt;
- ein `Set` ohne Objekt;
- ein `Del` mit Objekt (seit SF-098 muss es leer sein);
- eine `Query` ohne `Parent`;
- eine eigentlich gültige `Query`, aber auf dem Topic **mit** Source (erkennt die Regelschicht).

Die Tabelle steht in [`REVIEW.md`](REVIEW.md#r-2--referencedesignation-vectors-v-pay-225--230-draft).

### Begriffe aus C-036

- **Action (Call/Reply):** eine Art Funktionsaufruf über MQTT. Eine Anwendung schickt einen **Call** an eine andere („gib mir eine neue DataSetWriterId“), die antwortet mit einem **Reply**. Diese Nachrichten haben einen eigenen `MessageType` (`ua-action-request` bzw. `ua-action-response`) und ein paar eigene Felder: `ResponseAddress` (wohin die Antwort soll), `TimeoutHint` (wie lange der Aufrufer wartet), `RequestorId` (wer fragt).
- **NewDataSetWriterId:** der einzige Standard-Dienst (Common Service) der Guideline. Eine Anwendung fragt damit eine neue, eindeutige `DataSetWriterId` für eine Ressource an. Er hat die reservierte `DataSetWriterId` 8.
- **Status (in der Antwort):** sagt, ob der Aufruf geklappt hat. `{}` heißt „Good“. Sonst steht ein OPC-UA-Code drin, z. B. `BadInvalidArgument`. Bei einem „Bad“-Code darf die Antwort keine Nutzdaten enthalten.
- **MAM:** das „Typenschild“ einer Anwendung oder eines Geräts (Hersteller, Modell, Seriennummer, Handbuch-Link …).
- **Profile:** die Liste der Ressourcen, die eine Anwendung anbietet (MAM, Health, Config …).
- **Umbrella (Schirm-REQ):** eine REQ, die eigentlich mehrere Einzelregeln zusammenfasst. Das Orakel meldet immer die Einzelregel. Deshalb zählt die Schirm-REQ als abgedeckt, wenn alle ihre Einzelregeln abgedeckt sind (so in ADR-0019 §9 beschlossen).


---

## Was aus deinem letzten Review (SF-099 in der Guideline) geworden ist

- **R-1 – REQ-CMP-047:** angenommen und auf `reviewed` gesetzt (`737cb14`).
- **R-2 – REQ-CMP-041:** angenommen; die Regel bleibt `reviewed` mit dem Filter-Satz.
- **R-3 – Guideline-Text (`a42a132`):** angenommen; nichts zu ändern.

## Was aus dem Review davor (C-036) geworden ist

- **D-1 – SF-099 (leerer `Pub`):** so eingetragen, wie du es entschieden hast (Register-Commit `5491830`, Status `decided`):
  - Fragt jemand per `Get` etwas ab, das es bei einem **bekannten** Asset gerade nicht gibt, antwortet die Anwendung mit einer **leeren Liste** (`"Messages": []`).
  - Fragt jemand nach einem Asset, das die Anwendung **gar nicht kennt**, kommt **keine Antwort** auf dem `Pub`-Topic. Die Anwendung **sollte** stattdessen ein `Event` der Kategorie `Status` mit einem Fehlercode (`Bad`, z. B. `BadNotFound`) schicken.
  - Du hattest recht: Dieses Event-Verhalten steht schon in MG 9.1 „Resources Without Content“, und zwar für `Set`/`Del` auf etwas Unbekanntes und für `Get Metadata` ohne Daten. Für ein `Get` auf ein unbekanntes Asset fehlt dort nur ein Satz. Den Textvorschlag habe ich ins Register geschrieben.
  - Für die Schemas ändert sich nichts: Ein leerer `Pub` bleibt gültig. Ob überhaupt eine Antwort kommt, sieht man nur im laufenden Betrieb (D7 `monitor`).
  - Du hast bestätigt: Auch ein Filter, der bei einem bekannten Asset nichts trifft (z. B. ein unbekannter `Tag` bei `Get/Data`), bekommt eine leere Liste.
  - Auf deinen Auftrag steht das jetzt in der Guideline (`a42a132`). Danach habe ich die Re-Baseline gemacht und die neue Regel REQ-CMP-047 angelegt (siehe „Aktuell offen“).
- **D-2 – Nachtrag C-036 zu ADR-0019:** angenommen. Im ADR steht der Nachtrag jetzt als „accepted“. Punkt 1 beschreibt deine SF-099-Entscheidung.
- **D-3 – Aufteilung:** angenommen. C-037 ist PublicationList und ReferenceDesignation, der P2-Abschluss ist C-041.
- **D-4 – Heuristik für REQ-RES-MAM-006:** angenommen. Das Orakel bleibt, wie es ist.
- **R-1 bis R-4:** alles freigegeben. Die 51 Entwürfe stehen auf `reviewed`. Die Lücken bei der Vektor-Abdeckung sinken von 81 auf 48.
