# Review und Entscheidungen – Erklärung auf Deutsch

_Stand: 25.10.2026, nach Zyklus C-038 (Data, Event und SubscriptionList)._

Diese Datei erklärt die offenen Punkte aus [`REVIEW.md`](REVIEW.md) auf Deutsch und in Alltagssprache. Sie ist eine Lesehilfe auf Wunsch des PO (Ausnahme von der Regel „Projektartefakte auf Englisch“). **Maßgeblich ist `REVIEW.md`**; dort stehen auch die Checkboxen. Die Nummern (D-1, R-1 …) sind in beiden Dateien gleich, du kannst also z. B. im Chat antworten: „D-1 und D-2 wie vorgeschlagen, R-1 bis R-3 akzeptiert“.

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

2 Entscheidungen (D-1, D-2) und 9 neue Testvektoren (R-1 bis R-3). Es gibt kein offenes Spec-Finding und keine offene Frage (Q-011 bleibt geparkt). Details stehen im [C-038-Log](log/2026-10-25_C-038_data-event-sublist.md).

**Worum ging es?** In C-038 habe ich die JSON Schemas für drei weitere Ressourcen gebaut:
- **Data:** die eigentlichen Prozessdaten einer Anwendung. Ihr Aufbau ist frei, die Anwendung beschreibt ihn in den Metadata. Es gibt einen vordefinierten Spezialfall, **Oi4Pv** (siehe unten).
- **Event:** Meldungen einer Anwendung, in drei Kategorien: **Status** (ein OPC-UA-Statuscode wie `Good` oder `BadNotFound`), **Syslog** (eine klassische Log-Zeile mit Priorität) und **Generic** (frei definiert).
- **SubscriptionList:** die Liste, *was* eine Anwendung selbst abonniert hat (jeweils ein MQTT-Topic-Filter mit Intervall).

Dazu gehören 8 neue Einstiegs-Schemas; jetzt haben **26 von 29** Topic-Formen eines. Es fehlt nur noch Config (C-039).

**Zahlen:** Alle Prüfungen sind grün. Drei unabhängige Schema-Validatoren (Go, Rust, Python) kommen bei allen 518 Testfällen zum gleichen Ergebnis. Es gibt heute 33 Abdeckungslücken (vorher 35; zwei Schirm-REQs sind sofort abgedeckt). Gibst du R-1 bis R-3 frei, sind es noch **19**. Die übrigen 19 gehören zu Config und zu allgemeinen Regeln zu Topic, IdentificationLink und Komponenten (C-039, C-040). Data, Event und SubscriptionList haben dann keine Lücke mehr.

### D-1 – Nachtrag C-038 zu ADR-0019 (Schema-Architektur)

Wie bei jedem Zyklus habe ich die Bau-Entscheidungen als Nachtrag ins ADR-0019 geschrieben. Inhaltlich:

1. **Oi4Pv erkennt jetzt das Schema (ändert einen Satz, den du schon angenommen hast).** Oi4Pv ist ein vordefinierter Data-Datensatz: ein Hauptwert `Pv` (Pflicht) und bis zu 255 Nebenwerte `Sv1` … `Sv255`, sonst nichts. Bisher stand in §4: „Ob ein Datensatz Oi4Pv ist, entscheidet die Regelschicht anhand des Topics.“ Der Name steht aber auch in der Nachricht selbst, im Feld `WriterGroupName`. Deshalb prüft jetzt das Schema: Heißt der `WriterGroupName` `Oi4Pv`, muss der Inhalt Oi4Pv sein. Die Regelschicht prüft nur noch, dass `WriterGroupName` und Topic zusammenpassen.
2. **Orakel:** siehe D-2.
3. **Data hat keine feste Klassen-ID.** Andere Ressourcen haben eine feste `DataSetClassId`, an der man sie erkennt. Data nicht, denn jede Anwendung definiert ihre eigenen Datensätze. Das allgemeine Nachrichten-Schema prüft Data deshalb nur allgemein (ohne Topic weiß es nicht, dass es Data ist).
4. **Event-Regeln im Schema.** Einige Event-Regeln vergleichen den `WriterGroupName` (z. B. `Status/Bad` oder `Syslog/Error`) mit dem Inhalt. Das kann das Schema, weil beides in derselben Nachricht steht:
   - die Kategorie im Inhalt passt zur Kategorie im Namen;
   - bei Status-Events passt die Schwere des Codes (gut / unsicher / schlecht) zum Namen, und der symbolische Name beginnt mit `Good`, `Uncertain` oder `Bad`;
   - bei Syslog-Events passt die Priorität (Zahl modulo 8) zum Level im Namen, z. B. 13 → `Notice`.
   Die Bedingungen sind wie im Orakel gebaut, damit ein Fehler genau eine Regelverletzung meldet.
5. **Weiche im allgemeinen Schema:** Event und SubscriptionList werden an ihrer `DataSetClassId` erkannt, wie die anderen Ressourcen.
6. **SubscriptionList-Regel aufgeteilt (REQ-RES-SUBLIST-002).** Das Schema prüft: Ein abonniertes OEC-Topic nennt eine konkrete Ressource (kein `+` und kein `#` an dieser Stelle). Topics außerhalb von OEC prüft es nicht. Ob der `WriterGroupName` richtig aus dem Topic abgeleitet ist, prüft die Regelschicht, weil man dafür das Topic zerlegen muss.
7. **Zählen bei Get, Set und Del** wie in C-037: Ein `Get` enthält keinen Datensatz, ein `Set` (und ein `Del SubscriptionList`) genau einen. In `Set`/`Del` ist keine `SequenceNumber` erlaubt. Ein leeres `Pub` ist gültig (SF-099).
8. **Sechs Schirm-REQs** (Umbrellas), wie in C-036 beschlossen. Sie zählen als abgedeckt, wenn ihre Einzelregeln abgedeckt sind. Was übrig bleibt (z. B. „passen die Felder zu den Metadata?“), prüft D7 mit einer laufenden Anwendung oder eine Person.

Nur Punkt 1 ersetzt einen schon angenommenen Satz; die Prüfung wandert dabei nur von der Regelschicht ins Schema. **Empfehlung:** annehmen.

### D-2 – Orakel: `WriterGroupName` auf einem Topic ohne Filter

- **Frage:** Ein Data-, Config- oder Event-Datensatz trägt im `WriterGroupName` seinen Filter, z. B. den Tag `oee` oder `Status/Good`. Wird er auf einem Topic **ohne** Filter veröffentlicht (z. B. `Pub/Data/<Source>`), gibt es nichts zum Vergleichen. Bisher hat das Orakel dann jeden beliebigen Text akzeptiert.
- **Meine Lesart:** Auch dort muss der `WriterGroupName` die Form eines Filters dieser Ressource haben: bei Data und Config **eine** Ebene im Topic-Format (codiert), bei Event `<Kategorie>/<Level>`. Sonst ist REQ-MSG-024 verletzt.
- **Auswirkung:** Kein freigegebener Vektor ändert sein Ergebnis (die Guideline-Beispiele tragen den Tag `oee`). Schema und Orakel prüfen jetzt dasselbe.
- **Alternative:** Auf solchen Topics nicht prüfen. Dann müsste das Schema je nach Topic eine andere Regel anwenden, und das kann unsere Architektur (ein Einstiegs-Schema je Grammatik-Regel) nicht.
- **Empfehlung:** annehmen.

### R-1 – 3 Data-Vektoren (V-PAY-231 … 233)

- **V-PAY-231** ist ein **gültiges** Oi4Pv-Beispiel mit `Pv` und `Sv1`. Bisher gab es nur das Negativ-Beispiel V-PAY-084 (nur `Sv1`, `Pv` fehlt).
- **V-PAY-232:** ein `Get Data` mit Datensatz (ein `Get` muss leer sein).
- **V-PAY-233:** ein `Set Data` mit zwei Datensätzen (erlaubt ist genau einer).

### R-2 – 3 Event-Vektoren (V-PAY-234 … 236)

- **V-PAY-234:** ein Status-Event ohne `Number` (Pflichtfeld).
- **V-PAY-235:** ein Status-Event, dessen `Details` außer `SymbolicId` noch ein Feld `Code` haben (nicht erlaubt).
- **V-PAY-236:** ein Syslog-Event mit `Description` (bei Syslog steht der Text in `Details.MSG`, eine `Description` ist verboten).

Für Generic-Events braucht es keinen eigenen Vektor; ihre Regel ist eine Schirm-REQ über die beiden Grundregeln für alle Events.

### R-3 – 3 SubscriptionList-Vektoren (V-PAY-237 … 239)

- **V-PAY-237:** ein Eintrag ohne `TopicPath` (Pflichtfeld).
- **V-PAY-238:** ein `Get SubscriptionList` mit Eintrag (ein `Get` muss leer sein).
- **V-PAY-239:** ein `Set SubscriptionList` ohne Eintrag (erlaubt ist genau einer).

Jeder Vektor ist eine Beispielnachricht aus der Guideline mit **genau einem** eingebauten Fehler; das Orakel bestätigt die verletzte Regel. Die Tabellen mit Links stehen in [`REVIEW.md`](REVIEW.md#reviews).

**Nebenbei:** In fünf Begründungen (V-PAY-235 … 239) hatte ich zuerst falsche Kapitelnummern der Guideline angegeben; sie sind korrigiert (`a6bd841`).

### Begriffe aus C-038

- **WriterGroupName:** ein Feld in jedem Datensatz. Bei Data, Config und Event steht dort der Filter aus dem Topic (z. B. der Tag `oee` oder `Status/Good`), bei den Listen der Ressourcenname mit Filter (z. B. `Data/oee`).
- **Schirm-REQ (Umbrella):** siehe unten bei C-036.

### Begriffe aus C-037

- **PublicationList:** die Liste, *was* eine Anwendung *wann* veröffentlicht. Jeder Eintrag nennt die Ressource (z. B. `Data`), die Quelle (`Source`), optional einen Filter (z. B. einen bestimmten Tag) und den Modus (`Mode`, z. B. „nur auf Anfrage“ oder „zyklisch“).
- **ReferenceDesignation:** die Referenzkennzeichnung nach IEC 81346, also „wo sitzt dieses Asset in der Anlage, welche Funktion und welches Produkt ist es“.
- **Regelschicht:** kleine Zusatzprüfungen in D7 für das, was ein JSON Schema allein nicht sehen kann (z. B. das Topic oder „kein Eintrag doppelt“).

### Begriffe aus C-036

- **Action (Call/Reply):** eine Art Funktionsaufruf über MQTT. Eine Anwendung schickt einen **Call** an eine andere („gib mir eine neue DataSetWriterId“), die antwortet mit einem **Reply**. Diese Nachrichten haben einen eigenen `MessageType` (`ua-action-request` bzw. `ua-action-response`) und ein paar eigene Felder: `ResponseAddress` (wohin die Antwort soll), `TimeoutHint` (wie lange der Aufrufer wartet), `RequestorId` (wer fragt).
- **NewDataSetWriterId:** der einzige Standard-Dienst (Common Service) der Guideline. Eine Anwendung fragt damit eine neue, eindeutige `DataSetWriterId` für eine Ressource an. Er hat die reservierte `DataSetWriterId` 8.
- **Status (in der Antwort):** sagt, ob der Aufruf geklappt hat. `{}` heißt „Good“. Sonst steht ein OPC-UA-Code drin, z. B. `BadInvalidArgument`. Bei einem „Bad“-Code darf die Antwort keine Nutzdaten enthalten.
- **MAM:** das „Typenschild“ einer Anwendung oder eines Geräts (Hersteller, Modell, Seriennummer, Handbuch-Link …).
- **Profile:** die Liste der Ressourcen, die eine Anwendung anbietet (MAM, Health, Config …).
- **Umbrella (Schirm-REQ):** eine REQ, die eigentlich mehrere Einzelregeln zusammenfasst. Das Orakel meldet immer die Einzelregel. Deshalb zählt die Schirm-REQ als abgedeckt, wenn alle ihre Einzelregeln abgedeckt sind (so in ADR-0019 §9 beschlossen).


---

## Was aus deinem letzten Review (C-037) geworden ist

- **D-1 – Nachtrag C-037 zu ADR-0019:** angenommen. Im ADR steht der Nachtrag jetzt als „accepted“ (`097b812`).
- **D-2 – Lesart des Eintrags-Filters:** angenommen. Der Filter in der PublicationList ist codiert wie im Topic; das Orakel bleibt, wie es ist.
- **R-1 und R-2:** alles freigegeben. Die 13 Vektoren V-PAY-218 … 230 stehen auf `reviewed` (`69a6bb4`). Die Lücken bei der Vektor-Abdeckung sinken von 48 auf 35. PublicationList und ReferenceDesignation haben keine Lücke mehr.
- **Nebenbei korrigiert:** Das C-037-Log hatte fälschlich das Datum 26.10.; es heißt jetzt `2026-10-25_C-037_publist-refdes.md`.

## Was aus dem Review davor (SF-099 in der Guideline) geworden ist

- **R-1 – REQ-CMP-047:** angenommen und auf `reviewed` gesetzt (`737cb14`).
- **R-2 – REQ-CMP-041:** angenommen; die Regel bleibt `reviewed` mit dem Filter-Satz.
- **R-3 – Guideline-Text (`a42a132`):** angenommen; nichts zu ändern.
