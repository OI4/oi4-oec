# Review und Entscheidungen – Erklärung auf Deutsch

_Stand: 25.10.2026, nach C-036 (Guideline `68d878d`, Register `29682b0`)._

Diese Datei erklärt die offenen Punkte aus [`REVIEW.md`](REVIEW.md) auf Deutsch und in Alltagssprache. Sie ist eine Lesehilfe auf Wunsch des PO (Ausnahme von der Regel „Projektartefakte auf Englisch“). **Maßgeblich ist `REVIEW.md`**; dort stehen auch die Checkboxen und die vollständigen Vektor-Tabellen. Die Nummern (D-1, R-1 …) sind in beiden Dateien gleich, du kannst also z. B. im Chat antworten: „D-1 wie vorgeschlagen, D-2 bis D-4 akzeptiert, R-1 bis R-4 akzeptiert“.

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

**4 Entscheidungen und 51 Entwürfe** aus C-036: 42 neue Payload-Vektoren, 1 Topic-Vektor und 8 Guideline-Vektoren, deren REQ-Liste ich erweitert habe. Offen ist ein Spec-Finding (SF-099). Heute gibt es 81 Abdeckungslücken; nach deinem Review sind es **48**.

### Begriffe aus C-036

- **Action (Call/Reply):** eine Art Funktionsaufruf über MQTT. Eine Anwendung schickt einen **Call** an eine andere („gib mir eine neue DataSetWriterId“), die antwortet mit einem **Reply**. Diese Nachrichten haben einen eigenen `MessageType` (`ua-action-request` bzw. `ua-action-response`) und ein paar eigene Felder: `ResponseAddress` (wohin die Antwort soll), `TimeoutHint` (wie lange der Aufrufer wartet), `RequestorId` (wer fragt).
- **NewDataSetWriterId:** der einzige Standard-Dienst (Common Service) der Guideline. Eine Anwendung fragt damit eine neue, eindeutige `DataSetWriterId` für eine Ressource an. Er hat die reservierte `DataSetWriterId` 8.
- **Status (in der Antwort):** sagt, ob der Aufruf geklappt hat. `{}` heißt „Good“. Sonst steht ein OPC-UA-Code drin, z. B. `BadInvalidArgument`. Bei einem „Bad“-Code darf die Antwort keine Nutzdaten enthalten.
- **MAM:** das „Typenschild“ einer Anwendung oder eines Geräts (Hersteller, Modell, Seriennummer, Handbuch-Link …).
- **Profile:** die Liste der Ressourcen, die eine Anwendung anbietet (MAM, Health, Config …).
- **Umbrella (Schirm-REQ):** eine REQ, die eigentlich mehrere Einzelregeln zusammenfasst. Das Orakel meldet immer die Einzelregel. Deshalb zählt die Schirm-REQ als abgedeckt, wenn alle ihre Einzelregeln abgedeckt sind (so in ADR-0019 §9 beschlossen).

---

## Entscheidungen

### D-1 – SF-099: Darf ein `Pub` leer sein?

**Worum es geht:** Wenn jemand per `Get` z. B. die Health-Daten abfragt, antwortet die Anwendung mit einem `Pub`. Was, wenn sie zu dieser Anfrage gar nichts hat?

- Kapitel 9.1 („Resources Without Content“) sagt klar: Dann kommt ein `Pub` mit leerer Liste, `"Messages": []`.
- Die einzelnen Ressourcen-Abschnitte (9.1.1 MAM, 9.1.2 Health, 9.1.7 Profile …) sagen aber: „Der `Pub` enthält DataSetMessages vom Typ X und optional eine Pagination.“ Wörtlich gelesen muss also mindestens eine drin sein.

Die beiden Stellen widersprechen sich. Mein Prototyp aus C-033 hatte sich an die zweite Stelle gehalten und die leere Antwort abgelehnt – das war falsch.

Dazu kommt eine Lücke: Was passiert bei einem `Get` für ein Asset, das die Anwendung gar nicht kennt (z. B. `Get/Health/<unbekanntes Gerät>`)? Leere Antwort oder gar keine?

**Mein Vorschlag:** „enthält DataSetMessages vom Typ X“ sagt, *welche Art* von Einträgen drin sein darf, nicht *wie viele mindestens*. Eine leere Antwort ist erlaubt, wenn sie auf ein `Get` ohne Inhalt antwortet – auch bei einem unbekannten Asset. Ein leerer `Pub` ohne vorherige Anfrage bleibt verboten (REQ-CMP-042). Den Textvorschlag für 9.1 und 9.1.x findest du im Register.

**Alternative:** Bei einem unbekannten Asset antwortet die Anwendung gar nicht, der Fragende läuft in den Timeout. Für die Schemas ist das egal, nur der Guideline-Text wäre anders.

**Was schon umgesetzt ist (umkehrbar):** Die Schemas akzeptieren jetzt einen leeren `Pub`. An den REQs selbst ändert sich nichts.

**Empfehlung:** wie vorgeschlagen. Wenn du willst, arbeite ich es danach in die Guideline ein (dann Re-Baseline).

### D-2 – Nachtrag C-036 zu ADR-0019 (Schema-Architektur)

**Worum es geht:** Beim Bauen der Action- und MAM-Schemas habe ich ein paar Detailregeln festgelegt, die ins ADR gehören:

1. **Leerer `Pub`:** wie D-1.
2. **Schirm-REQs:** Die fünf Sätze „Ein `Pub` von MAM/Health/Profile/PublicationList/ReferenceDesignation enthält Objekte dieses Typs und optional Pagination“ sind keine einzelne Regel. Ein Verstoß ist immer etwas Konkretes: ein Objekt der falschen Ressource, ein fehlerhaftes Objekt, eine PaginationRequest, Locale oder Query im `Pub`. Das Orakel meldet jeweils die konkrete REQ. Deshalb werden diese fünf zu Schirm-REQs. Config, Data und SubscriptionList haben denselben Satz; die entscheide ich in ihren Zyklen.
3. **Wie das Schema eine Action erkennt:** zuerst am `MessageType`, dann an der `DataSetWriterId` 8 (NewDataSetWriterId). Eine Action, die ein Hersteller selbst definiert, prüft das Schema nur im Aufbau, nicht im Inhalt – den kennen wir ja nicht.
4. **Welche REQ bei einem unbekannten Feld gemeldet wird:** in den Action-Objekten (auch im `Status`) REQ-ACT-003; in den Nutzdaten von NewDataSetWriterId REQ-ACT-013 („Nutzdaten sind nach der ActionMetaData codiert“), ebenso bei falschem Datentyp oder Wertebereich. Ein fehlendes Feld oder ein inhaltlich falscher Wert bleibt REQ-ACT-021/022.
5. **Regelschicht (Prüfungen, die ein Schema nicht kann):** Antwortet wirklich der angesprochene Dienst (ACT-005)? Zeigt die `ResponseAddress` auf das eigene Reply-Topic (ACT-006)? Passt die `DeviceClass` im eigenen MAM zum Topic (MAM-014)? Steht der Absender in der MessageId (ACT-004)? Ist ProductInstanceUri = DataSetWriterName (MAM-016)? Dazu die MAM-006-Heuristik (D-4).
6. **Zyklusnummern** nach der Aufteilung (D-3).

**Empfehlung:** annehmen. Keine frühere Entscheidung wird umgedreht.

### D-3 – Aufteilung: PublicationList und ReferenceDesignation in einen eigenen Zyklus

**Worum es geht:** C-036 war geplant für Actions **und** fünf Ressourcen. Schon Actions allein brauchten 28 Vektoren und einen eigenen Nachrichtentyp. PublicationList und ReferenceDesignation sind je etwa so groß wie MAM und Profile zusammen. Zusammen wären es rund 100 Entwürfe für dein Review geworden.

**Neuer Plan:** C-036 = Actions + MAM, Health, Profile (erledigt). **C-037 = PublicationList + ReferenceDesignation** (neu). Danach verschiebt sich alles um eins: C-038 Data/Event/SubscriptionList, C-039 Config + `broker.json`, C-040 D7 `validate`, C-041 P2-Abschluss.

**Empfehlung:** annehmen.

### D-4 – REQ-RES-MAM-006 nur mit einer Heuristik prüfbar

**Worum es geht:** REQ-RES-MAM-006 sagt: *Wenn* eine Anwendung die Kennung ihres Geräts (ProductInstanceUri) selbst als Oi4Identifier erzeugt, dann aus Hersteller, Modell, Produktcode und Seriennummer des MAM. Ob die Kennung erzeugt wurde, sieht man der Nachricht aber nicht an.

**Meine Heuristik:** Wenn die Kennung so *aussieht* wie ein erzeugter Oi4Identifier – kein `https://`, genau vier Teile, der erste Teil ist der Hersteller – dann prüfe ich, ob die anderen drei Teile zu Modell, Produktcode und Seriennummer passen. Alles andere prüfe ich nicht.

**Risiko:** Ein Hersteller vergibt eine eigene Kennung, die zufällig genau so aussieht, aber nicht aus dem MAM erzeugt ist. Dann meldet D7 einen Fehler, der keiner ist. Ich halte das für selten, weil die Form sehr speziell ist.

**Alternative:** MAM-006 gar nicht offline prüfen, nur live (`d7-test`). Dann bleibt die REQ eine Lücke für den P2-Abschluss und Vektor V-PAY-210 fällt weg.

**Empfehlung:** Heuristik annehmen.

---

## Reviews

### R-1 – Action-Vektoren V-PAY-176 … 203

28 Beispielnachrichten auf Basis des Guideline-Beispiels NewDataSetWriterId (Call V-PAY-008, Reply V-PAY-009). Jede enthält genau **einen** Fehler, z. B.: Call ohne `TimeoutHint`, Reply mit `ResponseAddress`, `Ttl` als Text statt Zahl, `DataSetWriterId` 70000 (zu groß), `Status` mit Code 0 statt `{}`, reservierte DataSetWriterId 8 als Antwort. Drei Fehler (V-PAY-182/184/186) findet nicht das Schema, sondern die Regelschicht, weil man dafür das Topic mit der Nachricht vergleichen muss.

V-PAY-203 ist ein **gültiges** Beispiel: Der Dienst lehnt die Anfrage mit `BadInvalidArgument` ab und schickt keine Nutzdaten – so soll eine Ablehnung aussehen.

### R-2 – MAM- und Profile-Vektoren V-PAY-204 … 217

14 Fehler-Beispiele auf Basis der Guideline-Beispiele für MAM und Profile, z. B.: MAM ohne Seriennummer, Hersteller mit `https://` oder Großbuchstaben, Handbuch-Link ohne `https://` oder ohne Host, eine 256 Zeichen lange Kennung (erlaubt sind 255), ein OT-Connector, der sich im eigenen MAM als `Oi4.Utility` ausgibt, eine Profile-Liste mit einem Eintrag, den es nicht gibt (`Status`), oder mit `Health` doppelt, und ein `Get` Profile, das selbst ein Profile mitschickt.

### R-3 – Topic-Vektor V-TOP-253

Die Antwort von NewDataSetWriterId kommt auf `Reply/NewDataSetWriterId[/<Source>]`. Ein zusätzlicher Filter am Ende ist nicht erlaubt. V-TOP-253 hängt einen Filter an. Die Topic-Grammatik lehnt das ab, deshalb nennt der Vektor zwei REQs: ACT-020 (Topics von NewDataSetWriterId) und TOP-013 (erlaubte Topic-Formen).

### R-4 – Acht Guideline-Beispiele bekommen weitere REQs

Die Beispiele aus der Guideline (Health, NewDataSetWriterId, MAM) erfüllen schon REQs, die wir in C-036 neu prüfen, z. B. „Nutzdaten nach ActionMetaData codiert“ (ACT-013) oder „MAM-Kennung aus den MAM-Feldern erzeugt“ (MAM-006). Ich habe diese REQs in die Liste der Beispiele eingetragen; Topic und Nachricht bleiben unverändert. Weil das bereits reviewte Einträge sind, stehen sie wieder auf `draft`. Bis zu deinem Review zählen sie nicht für die Abdeckung, deshalb ist die Lückenzahl vorübergehend von 77 auf 81 gestiegen.

---

## Was aus deinem letzten Review (C-035) geworden ist

- **D-1 – Nachtrag zu ADR-0019:** angenommen. Im ADR steht der Nachtrag jetzt als „accepted“.
- **D-2 – Metadata schon in C-035:** angenommen.
- **D-3 – Topic-Regeln der Rollen:** Option (a). REQ-LIFE-002/006/008 werden weiter live geprüft.
- **R-1 bis R-4:** alles freigegeben. Die 31 Entwürfe stehen auf `reviewed`. Die Lücken bei der Vektor-Abdeckung sanken von 96 auf 77.
