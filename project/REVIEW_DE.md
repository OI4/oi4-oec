# Review und Entscheidungen – Erklärung auf Deutsch

_Stand: 26.10.2026, nach C-047 Nachtrag 1 (P4 läuft). **Für dich ist gerade nichts offen.**_

Diese Datei erklärt die offenen Punkte aus [`REVIEW.md`](REVIEW.md) auf Deutsch und in Alltagssprache. Sie ist eine Lesehilfe auf Wunsch des PO (Ausnahme von der Regel „Projektartefakte auf Englisch“). **Maßgeblich ist `REVIEW.md`**; dort stehen auch die Checkboxen. Die Nummern (D-1, R-1 …) sind in beiden Dateien gleich, du kannst also z. B. im Chat antworten: „D-1 ja, R-1 und R-2 akzeptiert“. Punkte mit A-x sind nur zur Info.

---

## Zuerst: ein paar Begriffe

- **REQ (Requirement):** eine einzelne, prüfbare Regel aus der Guideline, z. B. „`Messages` ist immer ein JSON-Array“. Jede REQ hat eine **Verifikationsmethode**, die angibt, *wer* sie prüft:
  - `schema` – das JSON Schema (D1) kann es an einer einzelnen Nachricht erkennen;
  - `d7-validate` – das Compliance-Tool D7 erkennt es offline an einer einzelnen Nachricht (Topic + Payload), notfalls mit einer kleinen Regelschicht neben dem Schema;
  - `d7-test` – D7 braucht dafür eine **laufende** Anwendung (z. B. „antwortet sie auf ein `Get`?“).
  - `d7-monitor` – D7 erkennt es durch bloßes Mithören auf dem Bus, oft erst über mehrere Nachrichten (z. B. „Health kommt alle 60 Sekunden“);
  - `d8` – das Infrastruktur-Tool D8 prüft es am Container, am Image, am Host oder am Broker;
  - `manual` – kein Tool kann es von außen sehen; es bleibt eine Prüfung durch einen Menschen.
- **Vektor:** eine Beispielnachricht (Topic + JSON-Payload) mit der erwarteten Bewertung. **Positive** Vektoren sind gültig und zeigen, dass eine Regel eingehalten wird. **Negative** Vektoren enthalten genau einen Fehler und nennen die verletzte(n) REQ(s) in `violates`.
- **Orakel:** unser eigener Referenz-Prüfer in `spectool`. Er rechnet für jeden Vektor aus, welche REQs verletzt sind. `spectool lint` schlägt Alarm, wenn das nicht mit `violates` übereinstimmt. So können wir uns bei den Vektoren nicht „verschreiben“.
- **P2-Exit-Kriterium:** Jede REQ mit `schema` oder `d7-validate` braucht **mindestens einen positiven und einen negativen Vektor**, die du reviewt hast. Ein entworfener Vektor (`draft`) zählt erst nach deinem Review.
- **`draft` / `reviewed`:** Alles, was ich neu anlege oder an etwas bereits Reviewtem ändere, steht auf `draft`, bis du es freigibst.

---

## Aktuell offen

**Nichts.** C-047 hat nur Code und Technik-Entscheidungen gebracht:
- Keine reviewte REQ und kein reviewter Vektor wurde geändert.
- Es gibt keinen neuen Vektor.
- Es gibt kein neues Finding in der Guideline. Das SDK, das Schema und die Vektoren waren sich in allen 245 Fällen einig.

Die Punkte A-1 … A-16 in `REVIEW.md` sind nur zur Info. Das sind Technik-Entscheidungen, die ich nach deiner Regel (GUARDRAILS G-3.4 b) selbst treffe. Du kannst sie kippen, musst aber nicht antworten.

Die Lücken bleiben bei 0, das Schema bleibt `2.0.1-dev` mit Baseline `734c560`.

**Nachtrag 1 (deine Anweisung):** In den Guardrails steht jetzt die neue Regel **G-3.10**. Am Ende jedes Zyklus entscheide ich, ob der nächste Zyklus mit einem neuen Agenten oder mit demselben weiterläuft, und schreibe den Grund ins Log.
- Einen **neuen Agenten** nehme ich in der Regel, wenn mein Kontext schon zusammengefasst wurde oder groß ist, oder wenn ein neues Inkrement, eine neue Phase oder ein neues Deliverable beginnt.
- **Derselbe Agent** macht nur kurze Nacharbeiten zum selben Thema weiter, z. B. die Antworten auf dein Review.

Bei einem neuen Agenten schreibe ich den Start-Prompt gleich in `prompt.md`. Diese Datei committe ich nie. Für C-048 habe ich mich für einen **neuen Agenten** entschieden, weil mein Kontext mehrfach zusammengefasst wurde und C-048 in einem neuen Crate arbeitet. `prompt.md` ist schon gefüllt.

Logs: [C-047](log/2026-10-26_C-047_message-model.md), [C-046 inkl. Nachtrag 1 bis 3](log/2026-10-26_C-046_p4-start.md).

### Begriffe aus C-047

- **Nachrichtenmodell:** Rust-Typen für jede OEC-Nachricht. Statt mit losem JSON zu hantieren, baut man z. B. eine „Health-Nachricht“ und das SDK schreibt das JSON genau so, wie die Guideline es verlangt.
- **Kodieren / Dekodieren:** Kodieren macht aus dem Rust-Typ das JSON zum Senden. Dekodieren macht aus empfangenem JSON wieder den Rust-Typ.
- **Toleranter Leser:** Beim Empfangen ignoriert das SDK unbekannte Felder (so verlangt es REQ-MSG-046). Beim Senden schreibt es nur die Felder der Guideline.
- **Content Mask:** die feste Liste, welche Felder eine Nachricht enthält. Die Guideline legt sie fest; das SDK hält sie immer ein.
- **Korrelation (Action):** Woran der Aufrufer erkennt, dass eine Antwort zu seiner Frage gehört: dieselbe `RequestId`, dieselben `CorrelationData`, und die Antwort geht an die `ResponseAddress` aus der Frage.
- **Schema einbetten:** Die JSON-Schemas (D1) werden beim Bauen ins Programm kopiert. Das fertige SDK braucht dann keine Schema-Dateien auf der Platte.
- **Feature `strict-validation`:** ein Schalter beim Bauen. Ist er an, prüft das SDK Nachrichten zusätzlich gegen das Schema. Er zieht deutlich mehr Bibliotheken nach sich (81 statt 20), deshalb ist er optional. Unsere automatische Prüfung baut und testet beide Varianten.
- **Paritätstest:** Der Test lässt alle 245 Payload-Vektoren durch das SDK laufen und vergleicht das Urteil mit dem Schema-Gate: gleiches Ergebnis und gleiche REQ-Nummern.
- **Laufzeit-Abhängigkeit:** eine fremde Bibliothek, die im fertigen SDK mitläuft. Neu sind serde und serde_json (JSON), time (Zeitstempel), uuid (Kennungen) und, nur mit dem Schalter, jsonschema.

### Begriffe aus C-046

- **Workspace (Cargo):** ein Rust-Projekt mit mehreren Bibliotheken (Crates), die zusammen gebaut und getestet werden. Unseres liegt in `oec/sdk/rust`.
- **`oec-core`:** der innere Teil des SDK. Er kennt die Regeln des Protokolls (Topics, Identität, später Nachrichten), sendet aber selbst nichts („sans-IO“). Entwickler benutzen ihn nicht direkt, sondern über `oec`.
- **`oec`:** die eigentliche SDK-Bibliothek, die Entwickler einbinden (ab Inkrement 4). **`oec-ffi`:** die C-Schnittstelle (Inkrement 10).
- **Inkrement:** ein Baustein aus der Liste in ROADMAP P4 (1 Identität, 2 Topics, 3 Nachrichten, …).
- **Export:** JSON-Kopien unserer Testvektoren, automatisch aus den YAML-Dateien erzeugt.
- **cargo-deny:** ein Werkzeug, das alle fremden Rust-Bibliotheken prüft: erlaubte Lizenz? Bekannte Sicherheitslücke? Noch gepflegt? Nur eine TLS-Version?
- **clippy / pedantic:** clippy ist der Rust-„Lektor“ für Code. `pedantic` ist seine strengste übliche Stufe.
- **Warnung vs. Fehler:** Ein Fehler macht ein Topic ungültig. Eine Warnung sagt nur „das sollte man nicht tun“; das Topic bleibt gültig.
- **dnp-encoding (Maskierung):** ein Verfahren aus MG A3. Jedes Zeichen außer `A–Z a–z 0–9 - . _ ~` wird als `,XX` geschrieben (XX = Hex-Code des Bytes). Aus `A/3` wird so `A,2F3`, aus `ABC@home` wird `ABC,40home`.
- **G-3.4 (a)/(b):** die neue Arbeitsteilung. (a) fragt dich, (b) entscheide ich selbst und schreibe es auf.

---

### Zur Info: was ich selbst entschieden oder nachgezogen habe (G-3.4 b)

- **A-1 … A-4** (aus C-046): Aufbau des Rust-Projekts (ADR-0026), Testvektoren als JSON, Reihenfolge in P4, Leerzeichen im Filter nur als Warnung.
- **A-5 … A-8** (aus C-046): der neue Code in `oec-core`, das Format des JSON-Exports, die Prüfung der fremden Bibliotheken mit cargo-deny, die Strenge der Code-Prüfung.
- **A-9** (aus C-046): Folgen aus SF-104. Das Rust-SDK maskiert jetzt immer; die Option „nur auf Wunsch maskieren“ ist weg. D7 und unser Referenz-Prüfer akzeptieren im MAM nur noch die maskierte Form. Kein bisher reviewter Vektor ändert dadurch sein Ergebnis.
- **A-10 (neu) – toleranter, aber typisierter Leser:** Unbekannte Felder werden ignoriert. Abgelehnt wird nur, was nicht in den Typ passt: falscher JSON-Typ, fehlendes Pflichtfeld, unbekannter Aufzählungswert, verletzte Grundregel (z. B. falsche DataSetClassId, reservierte DataSetWriterId). Feinheiten wie Textmuster prüft das Schema. Von den 143 ungültigen Vektoren lehnt der Leser 63 selbst ab, den Rest erkennen Schema oder Regelschicht.
- **A-11 (neu) – Schreiben nach fester Feldliste:** Das SDK schreibt immer genau die Felder der Guideline. `MessageType` im Datensatz wird immer mitgeschickt, `ReplyTo` ist `""`, wenn die Nachricht keine Antwort ist. Die DataSetClassId der Standard-Ressourcen setzt das SDK selbst.
- **A-12 (neu) – nichts geht verloren:** Zeitstempel und Binärdaten (`CorrelationData`) behalten ihren Originaltext. So kommt `CorrelationData` Byte für Byte unverändert zurück, wie REQ-ACT-007 es verlangt.
- **A-13 (neu) – Actions:** Wer eine Anfrage (Call) baut, bekommt Antwortadresse und Absender automatisch aus dem Topic. Die Antwort wird aus der Anfrage erzeugt und übernimmt alle Kennungen. Bei einem Fehlercode ist die Antwort leer, wie die Guideline es verlangt.
- **A-14 (neu) – Form einzelner Ressourcen:** Config-Gruppen sind frei benannt und werden deshalb als Tabelle (Name → Wert) gespeichert. Data ist ein freies JSON-Objekt mit einer bequemen Sicht für `Oi4Pv`. Der `TopicPath` der SubscriptionList bleibt Text, weil er beliebige MQTT-Filter enthalten darf.
- **A-15 (neu) – Schema und Bibliotheken:** Das SDK meldet die Schema-Version des eingebauten Stands (`2.0.1-dev` @ `734c560`), wie du es für C-047 vorgegeben hast. Die tolerante Schema-Variante wird beim Start im Speicher erzeugt statt ein zweites Mal eingebaut. Die neuen Bibliotheken sind auf das Nötigste beschränkt (20 Crates, 81 mit dem Schalter). cargo-deny ist ohne jede Ausnahme grün: alle Lizenzen erlaubt, keine bekannte Lücke. Die automatische Prüfung läuft jetzt mit und ohne den Schalter.
- **A-16 (neu) – Zyklusschnitt:** C-047 wurde nicht geteilt. Modell, Schema-Prüfung und Vektor-Test prüfen sich gegenseitig und gehören zusammen. Der Plan für die nächsten Zyklen bleibt gleich.

---

### Was als Nächstes passiert (C-048)

1. P4 Inkrement 4: die eigentliche SDK-Bibliothek `oec`, mit Start der Anwendung (`App`), Compliance-Level, Konfiguration (explizit oder über den Speicher-Vertrag des Containers), Speicher und Fehlertyp. Neue Bibliotheken: thiserror (Fehlertypen) und log (Logging).

---

## Begriffe aus den letzten Zyklen

### Begriffe aus C-045

- **`Mode` (PublicationList):** sagt, *wann* eine Anwendung einen Datensatz veröffentlicht. Die Grundwerte sind `OFF_0` (nie), `ON_REQUEST_1` (nur auf Anfrage mit `Get`), `APPLICATION_2`, `SOURCE_3`, `FILTER_4` (von sich aus veröffentlichen, unterschiedlich gebündelt: mit allen Datensätzen der Anwendung, je Quelle oder einzeln). Es gibt auch kombinierte Werte, z. B. `ON_REQUEST_APPLICATION_SOURCE_FILTER_15` = „von sich aus *und* auf Anfrage“.
- **anfragbare Ressource:** eine Ressource, die man mit `Get` abfragen kann (Liste in MG 9.1). `Event` gehört *nicht* dazu: Ein Ereignis passiert einmal, man kann es nicht nachträglich abfragen.
- **implizite REQ:** eine Regel, die in der Guideline ohne Schlüsselwort wie „MUST“ steht, aber offensichtlich verbindlich gemeint ist. Wir markieren sie mit `implicit: true` (so mit dir in SF-035 vereinbart).
- **`replaced_by`:** Eine zurückgezogene REQ zeigt auf ihre Nachfolgerin. Nummern werden nie wiederverwendet.
- **Vorab-Version `2.0.1-dev`:** der Arbeitsstand der Schemas nach 2.0.0. Die „1“ hinten heißt: nur eine kleine Änderung (hier nur die Spec-Baseline, kein Schema-Inhalt).

### Begriffe aus P3 (C-042 … C-044)

- **SDK:** eine Bibliothek, mit der Hersteller ihre Anwendung OEC-fähig machen, ohne das Protokoll selbst zu programmieren.
  - **D2:** das Haupt-SDK in Rust, mit C-Schnittstelle.
  - **D9:** das Micro-SDK in C für kleine Geräte (ESP32).
- **API-Skizze:** Code, der kompiliert und zeigt, wie man das SDK benutzt, aber noch nichts tut (die Funktionen sind leer). So kann man die Bedienung beurteilen, bevor jemand Wochen in die Umsetzung steckt.
- **C-ABI (`oec-ffi`):** die Schnittstelle, über die C-Programme (und später andere Sprachen) das Rust-SDK aufrufen. ABI heißt: Sie muss auch auf Binärebene stabil bleiben.
- **PAL (Platform Abstraction Layer):** im Micro-SDK eine kleine Tabelle von Funktionen (Netzwerk, Uhr, Speicher), die je Plattform (ESP-IDF, Arduino) einmal geschrieben wird.
- **Spike:** ein kleines Wegwerf-Experiment, das eine Entscheidung mit Fakten belegt.
- **Crate:** eine Rust-Bibliothek.
- **Sans-IO:** Protokollcode, der selbst nichts sendet oder empfängt, sondern nur Bytes umwandelt. Dadurch ist er leicht zu testen und unabhängig vom Netzwerk.
- **CUT (Component Under Test):** die Anwendung oder das Gerät, das D7 gerade prüft.
- **Proxy-Modus:** D7 stellt sich zwischen die geprüfte Anwendung und den Broker. So sieht D7 auch, *wie* sich die Anwendung verbindet (TLS-Version, Will, Anmeldung). Ein normaler Mithörer sieht das nicht.
- **Testkatalog:** die Liste aller geplanten Prüfungen von D7 und D8. Jede nennt die REQs, die sie prüft.
- **Exit-Code:** die Zahl, mit der ein Kommandozeilenprogramm endet. CI-Systeme (automatische Build-Pipelines) entscheiden daran, ob ein Lauf grün oder rot ist.
- **JUnit-XML:** ein Berichtsformat, das praktisch jedes CI-System anzeigen kann.

### Begriffe aus C-041

- **Gegenprobe (Crosscheck):** Orakel und Schema bewerten dieselben 634 Testfälle. Widersprechen sie sich, schlägt die Prüfung fehl.
- **Release / Manifest:** eine feste, nicht mehr änderbare Version der Schemas. Das Manifest speichert die Fingerabdrücke aller Dateien dieser Version.
- **Vorab-Version (`-dev`):** der Arbeitsstand zwischen zwei Releases.
- **Git-Tag:** ein fester Name für einen Commit, z. B. `oec-schema-2.0.0`.

### Begriffe aus C-040

- **D7 / `oec-check`:** das Prüftool für Anwendungen und Nachrichten. `validate` ist der erste Teil (offline); `monitor` und `test` folgen in P5.
- **Schicht (Layer):** die Stufe, in der D7 einen Fehler findet: Topic, Parse, Schema oder Regel.
- **Heuristik:** eine Prüfung, die fast immer richtig liegt, aber in seltenen Fällen falsch melden kann.
- **Warnung:** eine Abweichung von einer Empfehlung (SHOULD). Die Nachricht bleibt gültig (Exit-Code 0).

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

## Was aus deinem letzten Review (C-046, Nachtrag 2) geworden ist

Du hast geschrieben: „D-1 reviewed und akzeptiert -> übernehmen, R-1 und R-2 akzeptiert“. Daraus wurde Nachtrag 3 zu C-046:
- **D-1 – Guideline-Text:** In MG 4.3.1 steht jetzt: Enthält Model, ProductCode oder SerialNumber andere Zeichen als `A–Z a–z 0–9 - . _ ~`, **muss** der Wert dnp-codiert werden, damit sich der Oi4Identifier aus denselben Werten wieder erzeugen lässt. In MG A3 wurde aus „may be built / may be dnp-encoded“ ein „are built / are dnp-encoded“ (`734c560`, Docusaurus-Build grün). Im Register ist SF-104 jetzt „resolved“ (`fce5cf5`); es gibt keine offenen Findings.
  - Nicht geändert: GL-ADR-010. Das ist ein Entscheidungsprotokoll von damals und zitiert noch das alte „may“; das Register vermerkt, dass SF-104 diesen Punkt ablöst.
- **Neue Baseline `734c560`:** Das Inventory hat jetzt 1399 statt 1401 Einträge. Der Satz in MG 4.3.1 ist jetzt ein MUST statt eines MAY. Die zwei Sätze in A3 enthalten kein Schlüsselwort mehr („are“ statt „may“) und fallen deshalb aus dem Inventory. Bei REQ-IL-033 habe ich nur die Zitate auf den neuen Text umgestellt; der Inhalt, den du freigegeben hast, ist gleich geblieben.
- **R-1, R-2:** REQ-IL-022, REQ-IL-033 und V-PAY-245 stehen auf `reviewed`. Es gibt keinen Entwurf mehr.
- Das Schema bleibt `2.0.1-dev` (nur die Baseline ist jetzt `734c560`), und `oec/check.sh` ist grün.

## Was aus deiner Nachfrage zu SF-104 geworden ist (C-046, Nachtrag 2)

Du hast gefragt, ob es um das Topic oder die Nachricht geht und vor allem um `"`. Antwort: um beides gleich. Die Codierungen beim Übertragen (base64url im Topic, Escaping in JSON) gehen verlustfrei hin und zurück. Ein `"` ist in einem IL gar nicht erlaubt. Die Mehrdeutigkeit entsteht vorher, beim Bilden der Kennung, und betrifft nur die erlaubten Sonderzeichen wie `@`. Dann hast du entschieden: MG 4.1 ist führend. Das ist umgesetzt (REQ-IL-022/033, V-PAY-245, A-9); SF-104 stand danach im Register auf „decided“ (`726f2a3`) und ist seit Nachtrag 3 „resolved“.

## Was aus dem Review davor (C-046) geworden ist

Du hast zu D-4 nachgefragt (daraus wurde SF-104) und bestimmt, dass ich Technik-Entscheidungen wie D-1 selbst treffe. Daraus wurde im Nachtrag 1 zu C-046:
- GUARDRAILS G-3.4 ist zweigeteilt: (a) fragt dich, (b) entscheide ich und protokolliere es.
- D-1, D-2, D-3, D-5 habe ich wie empfohlen entschieden; ADR-0026 ist „accepted (agent)“.
- R-1 … R-4 sind nur noch Info.
- D-4 wurde die Guideline-Frage SF-104 (Nachtrag 1).

## Was aus dem Review davor (C-045) geworden ist

Du hast alles angenommen („R-1 bis R-4, D-1, D-2 reviewed und deine Empfehlungen akzeptiert“). Das habe ich im Nachtrag 1 zu C-045 daraus gemacht:
- **R-1 bis R-3:** REQ-RES-PUBLIST-026, REQ-RES-PUBLIST-027 und REQ-TOP-028 stehen auf `reviewed` (`08a09b6`). Es gibt keinen Entwurf mehr.
- **R-4:** Der Guideline-Text zu SF-101/102 bleibt, wie er ist.
- **D-1 – SF-103:** Die 13 Beispiele in MG B1.1.8/B2.6 zeigen jetzt `ON_REQUEST_APPLICATION_SOURCE_FILTER_15` (`e0262dd`). Im Register steht SF-103 auf „resolved“ (`2453eff`). Danach habe ich auf `e0262dd` neu gebaselinet; die vier Beispiel-Vektoren V-PAY-037 … 040 sind mitgezogen. Schema bleibt `2.0.1-dev`.
- **D-2 – P4 zuerst:** So umgesetzt; C-046 ist der erste P4-Zyklus (siehe oben).

## Was aus dem Review davor (C-042 … C-044) geworden ist

Du hast alles wie empfohlen angenommen („D-1 bis D-19 wie empfohlen, R-1 bis R-5 akzeptiert“). Das habe ich in C-045 daraus gemacht:
- **D-1 – P3-Plan:** umgesetzt; P3 ist jetzt in ROADMAP und STATUS abgeschlossen. Alle fünf Exit-Kriterien sind erfüllt.
- **D-2 bis D-7 – ADR-0020 … 0025:** alle stehen auf „accepted“. Jedes ADR hat einen kurzen Abschnitt „PO decision“, der deine Antworten nennt (`2658c21`). ADR-0017 hat einen Nachtrag: Das Micro-SDK nimmt eigenen JSON-Code, nicht jsmn oder cJSON.
- **D-8 bis D-17 – Q-021 … Q-030:** stehen in `OPEN-QUESTIONS.md` unter „Decided“, alle Option (a). Offen ist nur noch Q-011 (Veröffentlichung/CRA, geparkt).
- **D-18 – SF-101** und **D-19 – SF-102:** in die Guideline eingebaut (`df38e72`, `ccb475a`), im Register „resolved“ (`13d6b1a`). Beim Einbau habe ich den Satz zu SF-101 eingeschränkt (siehe R-4) und ein neues Finding SF-103 gefunden (siehe D-1).
- **Neu-Baseline:** auf `ccb475a`. Daraus kamen 2 neue Inventar-Einträge und die 3 neuen REQs von R-1 bis R-3.
- **Schema:** Version 2.0.0 bleibt eingefroren. Weil sich die Spec-Baseline geändert hat, heißt der Arbeitsstand jetzt `2.0.1-dev`. Am Inhalt der Schemas hat sich nichts geändert. Die Release-Prüfung konnte noch nicht mit Vorab-Versionen umgehen; das habe ich repariert.
- **R-1 bis R-5:** angenommen. Skizzen, Spikes, Testkatalog und Tool-Design bleiben so; die Kommentare in den Skizzen nennen SF-101/102 nicht mehr „offen“.
- **ROADMAP:** P4, P4b, P5, P5b und P6 sind nach deinen Antworten verfeinert. Wichtig: Die Logger-Demo wird in C geschrieben (über die C-Schnittstelle) und über Datei/Umgebungsvariablen konfiguriert. Der alte Satz „konfigurierbar über die `Config`-Ressource“ ist korrigiert.

## Was aus dem Review davor (C-041) geworden ist

- **D-1 – Nachtrag C-041 zu ADR-0019:** angenommen; im ADR steht er jetzt als „accepted“ (`03ec842`).
- **D-2 – P2 abschließen:** angenommen. P2 ist in ROADMAP und STATUS als abgeschlossen markiert; die aktuelle Phase ist jetzt P3.
- **D-3 – Git-Tag:** angenommen. Der Tag `oec-schema-2.0.0` sitzt auf dem Commit, der die Annahme festhält (`03ec842`). Die Schema-Dateien selbst sind seit dem Release-Commit `5a2f16c` unverändert. Der Tag ist nur lokal (wir pushen nie).
- **R-1 – die 7 REQs ohne `schema`:** angenommen; alle 7 stehen auf `reviewed` (`98c0aae`). Es gibt keinen Entwurf mehr, die Lücken bleiben bei **0**.
- **Wichtig für später:** Version 2.0.0 ist eingefroren. Jede künftige Schema-Änderung startet eine Vorab-Version (`2.0.1-dev` oder `2.1.0-dev`); die Freeze-Prüfung in `oec/check.sh` passt darauf auf.

## Was aus dem Review davor (C-040) geworden ist

- **D-1 – Nachtrag C-040 zu ADR-0019:** angenommen; im ADR steht er jetzt als „accepted“ (`7a50660`).
- **D-2, D-3, D-4:** angenommen. Das war schon so umgesetzt; es musste nichts mehr geändert werden.
- **R-1 bis R-3:** alle 8 Vektoren stehen auf `reviewed` (`81b3521`). Die Lücken sinken wie vorgerechnet von 11 auf **0**.
- **R-4 – Schirm-REQs:** angenommen; sie standen schon in `implications.yaml`.

## Was aus dem Review davor (C-039, Nachtrag 2) geworden ist

- **R-1 – REQ-CNT-003** und **R-2 – REQ-CNT-004:** freigegeben, beide stehen auf `reviewed` (`995258d`).
- **R-3 – der neue RIG-Text zu SF-100:** freigegeben; der Absatz mit den genauen Regeln bleibt drin.
- Danach gab es keinen Entwurf mehr; die Lücken lagen bei 15.

## Was aus dem Review davor (C-039) geworden ist

- **D-1 – Nachtrag C-039 zu ADR-0019:** angenommen. Im ADR steht der Nachtrag jetzt als „accepted“. Punkt 7 sagt, dass das `broker.json`-Schema SF-100 umsetzt, wie entschieden (`d7520ba`).
- **D-2 – SF-100:** wie vorgeschlagen entschieden. Den RIG-Text habe ich eingebaut (`168e1d1`), im Register steht SF-100 auf „resolved“ (`7808df7`). Daraus kamen die Punkte R-1 bis R-3 des Nachtrags 2.
- **R-1 bis R-4:** alles freigegeben. Die 2 REQs und die 20 Vektoren stehen auf `reviewed` (`dd5893d`). Die Lücken sinken wie vorgerechnet von 16 auf **15**; Config hat keine Lücke mehr.
- **Nebenbei:** An mehreren Stellen (Vektoren, Schema, Orakel) stand „SF-100, proposed“. Das „proposed“ habe ich entfernt. Am Inhalt hat sich nichts geändert.
