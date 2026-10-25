# Review und Entscheidungen – Erklärung auf Deutsch

_Stand: 25.10.2026, nach C-039 (Config und `broker.json`)._

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

2 Entscheidungen (D-1, D-2), 2 geänderte REQs (R-1, R-2) und 20 neue Testvektoren (R-3, R-4). Es gibt ein offenes Spec-Finding (**SF-100**, in D-2) und keine offene Frage (Q-011 bleibt geparkt). Ein Info-Punkt (neue Guideline-Baseline) braucht keine Entscheidung. Details stehen im [C-039-Log](log/2026-10-25_C-039_config-broker.md).

**Worum ging es?** In C-039 habe ich zwei Dinge gebaut:
- **Config:** die Einstellungen einer Anwendung. Sie veröffentlicht sie in Gruppen (`Pub Config`), und man kann sie mit `Set Config` ändern. Jede Einstellung hat einen Typ (z. B. Zahl, Text, Datum), einen Wert und optional Prüfregeln (`Validation`). Geheime Werte (z. B. Passwörter) sind als `Sensitive` markiert und werden nur maskiert veröffentlicht (`********`).
- **`broker.json`:** die kleine Datei, mit der eine OEC-Plattform den Anwendungen sagt, wo der MQTT-Broker läuft (Adresse, Port, maximale Nachrichtengröße; RIG 4.1.1). Das ist **keine Nachricht**, sondern eine Datei auf der Plattform. Sie gehört deshalb zum Infrastruktur-Prüftool D8, nicht zu D7.

Mit Config hat jetzt **jede der 29 Topic-Formen** ein Einstiegs-Schema; das Schema-Bundle ist damit für alle Nachrichten vollständig.

**Zahlen:** Alle Prüfungen sind grün. Drei unabhängige Schema-Validatoren (Go, Rust, Python) kommen bei allen 628 Testfällen zum gleichen Ergebnis. Es gibt heute 16 Abdeckungslücken (vorher 19). Gibst du R-3 frei, sind es noch **15**. Config hat dann keine Lücke mehr. Die übrigen 15 sind allgemeine Regeln zu Topics, IdentificationLink und Komponenten; die meisten davon prüft die Regelschicht von D7 (nächster Zyklus C-040).

### D-1 – Nachtrag C-039 zu ADR-0019 (Schema-Architektur)

Wie bei jedem Zyklus stehen die Bau-Entscheidungen als Nachtrag im ADR-0019. Kein schon angenommener Satz wird ersetzt. Inhaltlich:

1. **Zwei Bauformen für Config.** Ein `Pub` enthält jede Einstellung vollständig (Name, Typ, Wert, Pflicht-Kennzeichen …). Ein `Set` ist eine Teil-Änderung: Dort ist nur der neue `Value` Pflicht, alles andere darf fehlen.
2. **Wert passend zum Typ.** Das Schema prüft, ob der Wert zum `Type` passt: `true`/`false` bei Boolean, eine Zahl bei Number, ein Datum im Format RFC 3339 bei DateTime, Base64 bei ByteString. Ein leerer Text `""` heißt „nicht gesetzt“ und ist immer erlaubt. Die Maske `********` ist nur bei `Sensitive` = true erlaubt, und in einem `Pub` muss ein geheimer Wert leer oder maskiert sein (siehe R-2). Ob ein Datum im Kalender existiert (z. B. 30. Februar), prüft die Regelschicht. Nebenbei habe ich das Orakel angeglichen: Base64 mit Zeilenumbruch lehnt es jetzt wie das Schema ab.
3. **Weiche im allgemeinen Schema.** Ohne Topic weiß man nicht, ob ein Config-Datensatz aus einem `Pub` oder einem `Set` stammt. Das allgemeine Schema prüft deshalb nach den lockereren `Set`-Regeln; die strengen `Pub`-Regeln prüft das Einstiegs-Schema `pub-config`.
4. **Zwei Regeln für die Regelschicht.** „Der Wert hält seine eigenen Prüfregeln ein“ (z. B. Minimum, Muster) und „jeder Kontext kommt pro Quelle nur einmal vor“ kann ein JSON Schema nicht ausdrücken. Das prüft die Regelschicht von D7.
5. **Einstiegs-Schemas:** `Get Config` enthält keinen Datensatz, `Set Config` genau einen (ohne `SequenceNumber`), ein leeres `Pub` ist gültig (SF-099), wie bei den anderen Ressourcen. Damit sind es **29 von 29**.
6. **Zwei Schirm-REQs** (Umbrellas), wie in C-036 beschlossen.
7. **Schema für `broker.json`** als eigenes Modul `platform/broker.schema.json`. Es setzt den Vorschlag aus SF-100 um (siehe D-2). Das Orakel hat dafür eine eigene Prüfung bekommen; beide nutzen dieselben Muster.
8. **Ausnahme bei der Rückverfolgung.** Jede Regel im Schema nennt ihre REQ. Normalerweise muss die REQ dann die Methode `schema` haben. Die `broker.json`-REQ (REQ-CNT-003) behält aber `d8`, denn sie gehört zum Infrastruktur-Prüftool. Für Dateien im Ordner `platform/` ist das jetzt erlaubt. Sie zählt nicht zu den 158 REQs, die wir mit Vektoren abdecken müssen.
9. **Neue Vektor-Art `broker-json`** (`V-BRK-xxx`): statt Topic + Nachricht steht dort der Datei-Inhalt. Orakel und Schema prüfen sie wie die Nachrichten-Vektoren.

**Empfehlung:** annehmen.

### D-2 – SF-100: Was darf in `broker.json` stehen?

Die RIG (4.1.1) nennt drei Pflichtfelder, lässt aber drei Fragen offen. Ich habe sie als **SF-100** ins Findings-Register geschrieben, mit diesem Vorschlag:

- **(a) Zusätzliche Felder:** erlaubt. Eine Anwendung ignoriert Felder, die sie nicht kennt. So kann eine spätere Version der Datei neue Felder bekommen, ohne alte Anwendungen zu stören. *Alternative:* verboten.
- **(b) Form der Adresse:** „IP-Adresse oder Host-Name“ ist zu ungenau. Vorschlag: eine IPv4-Adresse (z. B. `192.168.0.10`, ohne führende Nullen), eine IPv6-Adresse ohne eckige Klammern, oder ein Host-Name nach RFC 1123 (Buchstaben, Ziffern, Bindestrich; höchstens 253 Zeichen). **Kein** Port, **kein** `mqtts://` davor. *Alternative:* jeder nicht leere Text.
- **(c) Port 0:** Der Datentyp erlaubt 0, aber auf Port 0 kann kein Broker lauschen. Vorschlag: nur 1 bis 65535.

Dazu habe ich einen Textvorschlag für die RIG ins Register geschrieben. Schema, Orakel und Vektoren setzen den Vorschlag schon um. Entscheidest du anders, passe ich sie an. Den Guideline-Text ändere ich nur auf deinen Auftrag.

**Empfehlung:** wie vorgeschlagen; gern mit dem Auftrag, den RIG-Text einzuarbeiten.

### R-1 – REQ-RES-CONFIG-011 wird `d7-monitor`

Die Regel: „Eine Pflicht-Einstellung (`Mandatory` = true) muss in jedem `Set` ihrer Gruppe enthalten sein.“ In einem `Set` steht aber nicht, welche Einstellungen Pflicht sind. Das sieht man nur im letzten `Pub Config` derselben Quelle. Eine einzelne Nachricht reicht also nicht. D7 muss den Verkehr mitlesen (`d7-monitor`). Dadurch fällt die REQ aus der Vektor-Pflicht heraus (159 → 158), und ihre Lücke verschwindet.

### R-2 – REQ-RES-CONFIG-024 bekommt zusätzlich `schema`

Die Regel: „Geheime Werte sind in jedem `Pub` maskiert.“ Das kann das Schema `pub-config` an der Nachricht selbst prüfen: Bei `Sensitive` = true ist als `Value`/`DefaultValue` nur `""` oder `********` erlaubt. Die bisherigen Methoden bleiben (`d7-test` prüft z. B., dass auch nach einem `Set` maskiert wird).

### R-3 – 2 Config-Vektoren (V-PAY-240, 241)

- **V-PAY-240** (gültig): ein Gerät veröffentlicht alle Einstellungen auf einmal (Topic ohne Filter), in zwei Kontexten „Network settings“ und „Overall device settings“, mit einem maskierten Passwort.
- **V-PAY-241** (ungültig): derselbe Vektor, aber der zweite Kontext heißt auch „Network settings“. Damit gibt es den Kontext zweimal für dieselbe Quelle (REQ-RES-CONFIG-015). Das erkennt nur die Regelschicht, nicht das Schema.

### R-4 – 18 `broker.json`-Vektoren (V-BRK-001 … 018)

Die RIG hat kein Beispiel für `broker.json`, also habe ich alle Vektoren selbst gebaut:
- **4 gültige:** Host-Name, IPv4-Adresse, IPv6-Adresse, voller Domain-Name mit einem Zusatzfeld (hängt an SF-100 (a)). Dazu die kleinste und größte erlaubte Paketgröße.
- **14 ungültige**, jeweils mit **genau einem** Fehler: ein Pflichtfeld fehlt (3×), Paketgröße zu klein, zu groß oder als Text, Port zu groß, als Text oder 0 (SF-100 (c)), Adresse mit Port, als URL oder leer (SF-100 (b)), Feldnamen klein geschrieben, Datei ist kein JSON-Objekt.

Die Tabelle mit Links steht in [`REVIEW.md`](REVIEW.md#r-4--brokerjson-vectors-v-brk-001--018-draft).

### Info – neue Guideline-Baseline `23f6cfd`

Du hast den Guideline-Branch mit einem Commit von Konrad Heidrich zusammengeführt (`30fea57`: Formatierung `<sub>` in der Tabelle in MG A3). Inhaltlich ändert sich nichts. Ich habe darauf neu aufgesetzt; nur die Kennungen im Inventar haben sich geändert, die 1398 Einträge sind gleich geblieben. Nichts zu entscheiden.

### Begriffe aus C-039

- **Config:** die Einstellungen einer Anwendung, gruppiert in Kontexte (ein Datensatz je Kontext) und Gruppen.
- **Sensitive / Maske:** Ein geheimer Wert (z. B. ein Passwort) ist `Sensitive` und wird nur als `""` oder `********` veröffentlicht.
- **`broker.json`:** eine Datei der OEC-Plattform mit Adresse, Port und maximaler Paketgröße des MQTT-Brokers.
- **D8:** das Infrastruktur-Prüftool; es prüft die Plattform (z. B. `broker.json`), nicht die Nachrichten einer Anwendung.
- **Plattform-Datei:** eine Datei, die die Plattform für Anwendungen bereitstellt. Ihre Schemas liegen im Ordner `oec/schema/v2/platform/`.

### Begriffe aus C-038

- **WriterGroupName:** ein Feld in jedem Datensatz. Bei Data, Config und Event steht dort der Filter aus dem Topic (z. B. der Tag `oee` oder `Status/Good`), bei den Listen der Ressourcenname mit Filter (z. B. `Data/oee`).
- **Oi4Pv:** ein vordefinierter Data-Datensatz mit einem Hauptwert `Pv` (Pflicht) und bis zu 255 Nebenwerten `Sv1` … `Sv255`.
- **Event:** Meldung einer Anwendung, Kategorie Status (OPC-UA-Statuscode), Syslog (Log-Zeile mit Priorität) oder Generic (frei).
- **SubscriptionList:** die Liste, *was* eine Anwendung selbst abonniert hat (MQTT-Topic-Filter mit Intervall).
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

## Was aus deinem letzten Review (C-038) geworden ist

- **D-1 – Nachtrag C-038 zu ADR-0019:** angenommen. Im ADR steht der Nachtrag jetzt als „accepted“. An dem Satz in §4, den Punkt 1 ersetzt (Oi4Pv), steht ein Verweis auf den Nachtrag, damit man ihn beim Lesen nicht übersieht (`574b8e0`).
- **D-2 – `WriterGroupName` auf einem Topic ohne Filter:** angenommen. Das Orakel bleibt, wie es ist (`d6d5010`).
- **R-1 bis R-3:** alles freigegeben. Die 9 Vektoren V-PAY-231 … 239 stehen auf `reviewed` (`dc80c8a`). Die Lücken sinken wie vorgerechnet von 33 auf 19. Data, Event und SubscriptionList haben keine Lücke mehr.

## Was aus dem Review davor (C-037) geworden ist

- **D-1 – Nachtrag C-037 zu ADR-0019:** angenommen. Im ADR steht der Nachtrag jetzt als „accepted“ (`097b812`).
- **D-2 – Lesart des Eintrags-Filters:** angenommen. Der Filter in der PublicationList ist codiert wie im Topic; das Orakel bleibt, wie es ist.
- **R-1 und R-2:** alles freigegeben. Die 13 Vektoren V-PAY-218 … 230 stehen auf `reviewed` (`69a6bb4`). Die Lücken bei der Vektor-Abdeckung sinken von 48 auf 35. PublicationList und ReferenceDesignation haben keine Lücke mehr.
- **Nebenbei korrigiert:** Das C-037-Log hatte fälschlich das Datum 26.10.; es heißt jetzt `2026-10-25_C-037_publist-refdes.md`.
