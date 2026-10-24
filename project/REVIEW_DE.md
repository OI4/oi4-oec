# Review und Entscheidungen – Erklärung auf Deutsch

_Stand: 24.10.2026, nach C-034 (Workspace `6169023`, Guideline `cc2fb4b`, Register `72dcd60`)._

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

## Entscheidungen

### D-1 – SF-098: Darf ein `Del` eine DataSetMessage enthalten?

**Worum geht es?**
Ein `Del` ist eine Lösch-Anfrage: Eine Anwendung bittet eine andere, etwas zu entfernen, z. B. ein Abonnement aus der `SubscriptionList`. Die Guideline widerspricht sich, ob in so einer Nachricht ein Datensatz (DataSetMessage) mitgeschickt wird:

- **MG 8.3.11** (Kapitel Pagination) sagt allgemein: „ein `Del` enthält **keine** DataSetMessage“.
- **MG 9.1.9** (`SubscriptionList`) sagt: beim `Del` steht **genau ein** Eintrag in der Payload – nämlich der zu löschende. Das Beispiel zeigt das auch so.
- **MG 9.1.10** (`ReferenceDesignation`) zeigt beim `Del` eine leere Liste.
- **MG 8.2.3** spricht von „der DataSetMessage“ eines `Del`, also von einer vorhandenen.

**Warum ist das wichtig?**
Das Schema und D7 müssen sich festlegen. Je nach Lesart wäre das Guideline-Beispiel `Del SubscriptionList` entweder richtig oder falsch.

**Mein Vorschlag (empfohlen):** Die Regel der jeweiligen Ressource gilt. Ein `Del` enthält **höchstens eine** DataSetMessage, so wie es die Ressource in Kapitel 9.1 festlegt: keine bei `ReferenceDesignation`, genau eine bei `SubscriptionList`. Der Satz in 8.3.11 wollte eigentlich nur sagen, dass `Set` und `Del` nicht seitenweise (paginiert) verschickt werden. Vorgeschlagener neuer Text für 8.3.11: „… und ein `Del` enthält höchstens eine `DataSetMessage`, wie für die Ressource in 9.1 definiert.“

- Folge: Nur der Text von REQ-PAG-009 wird angepasst. Alles andere (Regeln, Orakel, Vektoren) passt schon.

**Alternative:** Ein `Del SubscriptionList` enthält **keine** DataSetMessage, weil das Topic (`Source` / `Filter`) den Eintrag ohnehin schon adressiert.

- Folge: MG 9.1.9, REQ-RES-SUBLIST-011, das Orakel und bereits reviewte Vektoren müssten geändert werden.

**Zusätzlich zu entscheiden:** Soll ich die Änderung gleich in die Guideline einarbeiten (danach Re-Baseline), oder zunächst nur die Entscheidung im Register festhalten?

### D-2 – Q-020: Wie prüft D7 die Birth-, Close- und Will-Nachrichten?

**Worum geht es?**
Beim Start, beim geordneten Beenden und bei einem Absturz verschickt eine Anwendung besondere Nachrichten:

- **Birth** – „ich bin da“: die MAM-Nachricht über sich selbst (REQ-LIFE-003);
- **Close** – „ich gehe ordentlich“: Health `NORMAL_0` mit HealthScore 0 (REQ-LIFE-007);
- **Will** – „ich bin abgestürzt“: Health `FAILURE_1` mit HealthScore 0. Diese Nachricht hinterlegt die Anwendung vorab beim Broker, der sie bei einem Verbindungsabbruch verschickt (REQ-LIFE-009).

**Das Problem:** Auf dem Draht sehen diese Nachrichten genauso aus wie ein normales `Pub MAM` bzw. `Pub Health`. Aus einer einzelnen Nachricht kann D7 nicht erkennen, ob es eine Close-Nachricht ist oder ein ganz normaler Health-Status. Darum lassen sich diese drei Regeln offline nicht prüfen. Eine normale Health-Meldung mit `FAILURE_1` und Score 50 ist ja völlig in Ordnung; als Will-Nachricht wäre sie aber falsch.

**Option (a) – empfohlen:** Wer prüft, sagt D7, welche Rolle die Nachricht hat:

- In den Testvektoren gibt es dafür ein optionales Feld `role` (`birth` / `close` / `will`).
- Auf der Kommandozeile heißt das z. B. `validate --role will nachricht.json`.
- D7 prüft dann zusätzlich die Regeln für diese Rolle.
- Vorteil: Ein Entwickler kann seine Will-Nachricht schon vor dem Deployment prüfen, und wir bekommen Testvektoren dafür.
- Nachteil: ein Feld und eine Option mehr. Die Rolle wird behauptet, nicht bewiesen.

**Option (b):** Diese drei Regeln werden nur im Live-Test (`d7-test`) geprüft. D7 verbindet sich dabei mit der laufenden Anwendung, beendet sie und schaut, was ankommt.

- Vorteil: nichts Neues.
- Nachteil: keine Offline-Prüfung, keine Vektoren vor P5.

### D-3 – Zyklus aufgeteilt: MetaData bekommt einen eigenen Zyklus C-035

**Worum geht es?**
C-034 sollte ursprünglich auch die **MetaData** enthalten. MetaData sind die Nachrichten, die beschreiben, wie die Daten aufgebaut sind: Feldnamen, Datentypen, Strukturen und Aufzählungen. Das ist praktisch ein eigenes Typsystem mit 15 Regeln. Drei davon haben noch gar keinen Vektor, und 8 MetaData-Vektoren sind noch keinem Schema zugeordnet.

**Was ich gemacht habe:** C-034 enthält jetzt nur das Fundament, also Werkzeuge, den Umschlag normaler Datennachrichten und die Hilfsdatensätze (Pagination, Locale, Query). MetaData kommt in einen neuen Zyklus **C-035**. Die folgenden Zyklen rücken um eins nach; der P2-Abschluss ist jetzt **C-040** statt C-039.

**Warum:** Sonst hätte sich der Umfang von C-034 ungefähr verdoppelt, und du hättest auf einmal weit über 50 Vektoren reviewen müssen. Du hattest mir erlaubt, zu große Zyklen selbst zu teilen; die Begründung steht im C-034-Log.

---

## Reviews

### R-1 – 23 neue „Fehler-Beispiele“ (V-PAY-129 … 151)

**Worum geht es?**
Für jede Regel brauchen wir ein Beispiel, das sie **verletzt**. Nur so wissen wir, dass Schema und D7 den Fehler auch erkennen. Ich habe dafür jeweils eine bereits von dir geprüfte, gültige Nachricht genommen und **genau einen typischen Fehler** eingebaut. Welche Regel verletzt ist, rechnet das Orakel aus und `spectool lint` kontrolliert das.

**Was du prüfen solltest:**

- Ist der eingebaute Fehler realistisch, also etwas, das ein Entwickler tatsächlich falsch machen könnte?
- Stimmen Titel und Begründung?
- Fehlt ein naheliegender Fehler?

Die Nachrichten mit Payload siehst du in [`payload-constructed.md`](../oec/spec/vectors/view/payload-constructed.md#v-pay-129).

In Alltagssprache:

| Vektor | Der Fehler | Verletzte Regel |
|---|---|---|
| V-PAY-129 | Der Umschlag sagt „ich bin MetaData“ (`ua-metadata`), enthält aber Health-Daten. | MSG-002 |
| V-PAY-130 | Das Pflichtfeld `ReplyTo` fehlt. Es muss immer da sein, notfalls als leerer Text. | MSG-003 |
| V-PAY-131 | `Messages` ist ein einzelnes Objekt statt einer Liste. | MSG-005 |
| V-PAY-132 | Im Absender (`PublisherId`) fehlt der Diensttyp vorn (z. B. `OTConnector/`). | MSG-006 |
| V-PAY-133 | Der Nachrichten-ID (`MessageId`) fehlt der Zeitstempel vorn. | MSG-007 |
| V-PAY-134 | Die Kennung der Ressourcenart (`DataSetClassId`) fehlt. | MSG-011 |
| V-PAY-135 | `ReplyTo` enthält nur einen Zeitstempel statt der vollständigen ID der Anfrage. | MSG-015 |
| V-PAY-136 | Der Absender steht zusätzlich im Datensatz, wo er nicht hingehört. | MSG-016 |
| V-PAY-137 | Der Datensatz hat zwei Versionsangaben gleichzeitig (`MetaDataVersion` **und** `MinorVersion`); erlaubt ist nur eine. | MSG-018 |
| V-PAY-138 | Der Hilfsdatensatz „Pagination“ steht unter dem falschen Gruppennamen („Paging“). | MSG-019 |
| V-PAY-139 | Health benutzt die Writer-Nummer 5; die Nummern 4–8 sind reserviert. | MSG-020 |
| V-PAY-140 | Health trägt einen Gruppennamen, obwohl Health keinen Filter hat (dann muss er leer sein). | MSG-024 |
| V-PAY-141 | Ein Status „alles gut“ (Code 0) wird mitgeschickt; er soll nur bei Problemen dabei sein. | MSG-026 |
| V-PAY-142 | Ein Status ohne die Pflichtangabe „Code“. | MSG-027 |
| V-PAY-143 | Health ist als „Ereignis“ (`ua-event`) markiert; das ist nur für die Ressource Event erlaubt. | MSG-029 |
| V-PAY-144 | Health wird als „Änderungs-Frame“ (`ua-deltaframe`) geschickt; OEC sendet immer vollständige Datensätze. | MSG-029, MSG-030 |
| V-PAY-145 | Der Health-Wert ist die Zahl `0` statt des Textes `"NORMAL_0"`. | MSG-031, RES-HEALTH-001 |
| V-PAY-146 | Die Sprachangabe (Locale) ist `"German"` statt eines Codes wie `de-DE`. | LOC-001 |
| V-PAY-147 | Ein `Get Health` enthält eine Suchanfrage (Query); Health unterstützt keine Query. | QRY-001 |
| V-PAY-148 | Der Datensatz-Name nennt ein anderes Gerät als das Topic. **Das Schema allein sieht das nicht**, weil es das Topic nicht kennt; die Regelschicht von D7 erkennt es. Darum steht dieser Vektor auf der „Erwartungsliste“ der Regelschicht. | MSG-023 |
| V-PAY-149 | Ein Schreibbefehl (`Set Data`) wird mit Seiten-Info (Pagination) verschickt; Schreibbefehle werden nie aufgeteilt. | PAG-008, PAG-009 |
| V-PAY-150 | Ein Schreibbefehl enthält eine laufende Nummer (`SequenceNumber`); die gibt es nur bei Veröffentlichungen. | BUS-010 |
| V-PAY-151 | Ein Eintrag der PublicationList heißt nur `oee` statt vollständig `Data/oee`. | MSG-025, RES-PUBLIST-002 |

**Zwei Korrekturen beim Bauen**, damit du dich nicht wunderst:

- **V-PAY-132** verletzt nur eine Regel. Man könnte erwarten, dass auch die `MessageId` nicht mehr zum Absender passt. Das Orakel vergleicht beide aber nur, wenn der Absender selbst korrekt aufgebaut ist; sonst würde ein einziger Fehler doppelt gemeldet.
- **V-PAY-151** verletzt zwei Regeln. Der falsche Name ist sowohl formal falsch als auch nicht passend zum Eintrag.

### R-2 – Sechs bereits reviewte Beispiele stehen wieder auf `draft`

**Worum geht es?**
Für einige Regeln gab es bisher nur negative Beispiele, aber keine positiven, also keine gültige Nachricht, von der ausdrücklich gesagt wird: „hier wird die Regel eingehalten“. Sechs der bereits reviewten Guideline-Beispiele erfüllen diese Regeln längst. Ich habe sie nur in deren Liste `reqs` („welche Regeln zeigt dieses Beispiel?“) ergänzt.

**Am Inhalt hat sich nichts geändert:** Topic und Payload sind unverändert. Weil sich aber ein reviewtes Element geändert hat, steht es nach unseren Regeln wieder auf `draft`.

| Beispiel | Was es ist | Neu als „zeigt Regel“ eingetragen |
|---|---|---|
| V-PAY-003 | Health-Antwort auf ein `Get` (B1.1.2) | keine eigenen Zusatzfelder (MSG-001), Antwort verweist auf die Anfrage (MSG-014) |
| V-PAY-005 | `Get MAM` mit Seitenanfrage (B1.1.11) | korrekte PaginationRequest (PAG-001, PAG-008) |
| V-PAY-006 | `Pub MAM` mit Seiten-Info (B1.1.11) | korrekte Pagination (PAG-003, PAG-004, PAG-008) |
| V-PAY-007 | `Get MAM` mit Sprachwunsch (B1.1.12) | genau eine Locale in einem `Get` (LOC-003) |
| V-PAY-046 | `Del ReferenceDesignation` (B1.1.10) | Löschbefehl ohne Seiten-Info (PAG-009) |
| V-PAY-061 | `Set Data` (B1.1.4) | Schreibbefehl ohne Seiten-Info (PAG-009) |

### R-3 – Drei Regeln mit geänderter Prüfmethode

**Worum geht es?**
Bei drei Regeln habe ich festgelegt, wer sie prüft (siehe die Begriffe oben). Alle drei stehen deshalb wieder auf `draft`.

| Regel | Inhalt | Vorher → nachher | Warum |
|---|---|---|---|
| REQ-MSG-026 | Einen Status nur schicken, wenn **nicht** alles gut ist. | nur D7 → **Schema** + D7 | „Code 0 = gut“ sieht man der einzelnen Nachricht direkt an, das kann das Schema prüfen. Das war im ADR-0019 schon als Kandidat vorgesehen. Beispiel: V-PAY-141. |
| REQ-PAG-009 | `Set` und `Del` werden nicht in Seiten aufgeteilt. | nur D7 → **Schema** + D7 | Ob in einem `Set`/`Del` ein Seiten-Hilfsdatensatz steckt, sieht man an Topic + Payload. Die Frage, wie viele Datensätze ein `Del` haben darf, hängt an D-1. Beispiel: V-PAY-149. |
| REQ-BUS-002 | Wer eine Anfrage schickt, adressiert sie an die richtige Ziel-Anwendung. | D7 offline → **D7 live-Test** | Eine einzelne Nachricht zeigt zwar, an wen sie adressiert ist, aber nicht, ob das die *gemeinte* Anwendung ist. Das geht nur im Live-Test mit der echten Anwendung; ein Fehler-Beispiel ist offline gar nicht möglich. |

### R-4 – „Sammelregeln“ und ihre Teilregeln (`implications.yaml`)

**Worum geht es?**
Manche Regeln der Guideline sind **Sammelregeln**, die eigentlich aus mehreren Einzelregeln bestehen. Beispiel: „Der Umschlag verwendet die festgelegte Inhaltsmaske“ (REQ-MSG-004). Die Inhaltsmaske selbst sieht man auf dem Draht nicht. Man sieht nur, welche Felder da sind, und genau das prüfen die Einzelregeln (ReplyTo da? Messages eine Liste? DataSetClassId da?).

Ein Fehler-Beispiel nennt immer die *Einzelregel*, nie die Sammelregel. Damit die Sammelregel trotzdem als abgedeckt gilt, legt eine Liste fest, welche Einzelregeln zusammen die Sammelregel ergeben. Sind alle Einzelregeln abgedeckt, gilt die Sammelregel als abgedeckt. Diese Regel hast du mit ADR-0019 §9 schon angenommen; jetzt geht es um die konkrete Liste.

| Sammelregel | Einzelregeln | Was übrig bleibt |
|---|---|---|
| MSG-004 – Inhalt des Umschlags (NetworkMessage) | MSG-003, 005, 011 | nichts |
| MSG-017 – Inhalt des Datensatzes (DataSetMessage) | MSG-016, 018, 024, 029, 031 | nichts |
| BUS-004 – Inhalt eines `Set` | BUS-010, MSG-030, TOP-013 und die `Set`-Regeln der einzelnen Ressourcen | ob das Objekt zur Ziel-Anwendung gehört → Live-Test |
| BUS-005 – Inhalt eines `Del` | BUS-010, TOP-013 und die `Del`-Regeln der Ressourcen | ob das zu löschende Objekt existiert → Live-Test |

**Was du prüfen solltest:** Ergeben die Einzelregeln wirklich die Sammelregel, und ist „was übrig bleibt“ richtig beschrieben?

Die Sammelregeln für Topics und Identification Link (CMP, TOP, IL) kommen mit D7 `validate` in C-039.

### R-5 – Klarstellungen im ADR-0019

**Worum geht es?**
Beim Ausbau sind mir drei technische Stolperfallen begegnet. Ich habe sie im ADR als Bauregeln notiert. **Keine Entscheidung ist dadurch geändert.**

1. **Ein Prüfer bricht früh ab.** Der Go-Validator (santhosh) hört bei einem fehlgeschlagenen Wertevergleich sofort auf und prüft die Nachbarbedingungen nicht mehr. Wenn ein Wert zwei Regeln verletzt (z. B. `"Health": 0` ist weder ein erlaubter Health-Wert noch richtig als Text kodiert), wurde nur eine gemeldet. Bauregel: Jede Regel bekommt ihren eigenen Prüfblock. Danach melden beide Validatoren beide Regeln.
2. **Typ-Wächter nur an der richtigen Stelle.** Wenn `Messages` gar keine Liste ist, meldete das Schema zusätzlich unsinnige Folgefehler (Pagination, Locale). Die Bauregel legt fest, wo die Prüfung „ist das eine Liste?“ stehen darf, damit nur der eigentliche Fehler gemeldet wird.
3. **Kommentare werden gegen den Katalog geprüft.** Jede Schema-Regel trägt im Kommentar ihre REQ-Nummer und das Guideline-Kapitel. Ein neuer Test prüft, ob die Nummer existiert und ob das Kapitel stimmt. Dabei fielen mehrere falsche Kapitelangaben aus dem Proof of Concept auf, die jetzt korrigiert sind.

Außerdem liegt die Health-Schemadatei jetzt wie geplant im Ordner `resources/`.

---

## Zur Information (nichts zu entscheiden)

- **Wirkung deines Reviews:** Derzeit fehlen bei 129 von 159 Regeln noch positive oder negative Beispiele. Nach Freigabe von R-1 bis R-3 sind es laut Probelauf **96**. Die Bereiche Umschlag, Pagination, Locale und Query sind dann vollständig; offen bleiben MetaData (C-035) und die Ressourcen (C-036 bis C-038).
- **Qualität:** Drei unabhängige Prüfprogramme (Go, Rust, Python) kommen bei allen 172 Testfällen zum gleichen Ergebnis; alle `spectool`-Prüfungen sind grün.
- **Geparkt:** Q-011 (Veröffentlichung / Cyber Resilience Act) – für P2 nicht nötig.

## Nach deinem Review

Ich setze die freigegebenen Punkte auf `reviewed`, trage deine Antworten im C-034-Log ein, setze D-1 bis D-3 um, aktualisiere `REVIEW.md` und diese Datei und starte C-035 (MetaData).
