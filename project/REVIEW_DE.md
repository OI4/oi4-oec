# Review und Entscheidungen – Erklärung auf Deutsch

_Stand: 26.10.2026, nach C-045 (P3-Abschluss)._

Diese Datei erklärt die offenen Punkte aus [`REVIEW.md`](REVIEW.md) auf Deutsch und in Alltagssprache. Sie ist eine Lesehilfe auf Wunsch des PO (Ausnahme von der Regel „Projektartefakte auf Englisch“). **Maßgeblich ist `REVIEW.md`**; dort stehen auch die Checkboxen. Die Nummern (D-1, R-1 …) sind in beiden Dateien gleich, du kannst also z. B. im Chat antworten: „R-1 bis R-4 akzeptiert, D-1 und D-2 wie empfohlen“.

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

**P3 ist abgeschlossen.** Was aus deinen Antworten geworden ist, steht weiter unten. Beim Umsetzen sind ein paar kleine neue Punkte entstanden:
- **2 Entscheidungen** (D-1, D-2): ein neues, rein redaktionelles Spec-Finding und die Reihenfolge der nächsten Phasen;
- **4 Review-Punkte** (R-1 … R-4): drei neue REQs (`draft`) und der genaue Wortlaut der zwei neuen Guideline-Sätze.

Nichts davon blockiert den Start der nächsten Phase. Die Abdeckung bleibt bei 0 Lücken.

Log: [C-045](log/2026-10-26_C-045_p3-exit.md).

### Begriffe aus C-045

- **`Mode` (PublicationList):** sagt, *wann* eine Anwendung einen Datensatz veröffentlicht. Die Grundwerte sind `OFF_0` (nie), `ON_REQUEST_1` (nur auf Anfrage mit `Get`), `APPLICATION_2`, `SOURCE_3`, `FILTER_4` (von sich aus veröffentlichen, unterschiedlich gebündelt: mit allen Datensätzen der Anwendung, je Quelle oder einzeln). Es gibt auch kombinierte Werte, z. B. `ON_REQUEST_APPLICATION_SOURCE_FILTER_15` = „von sich aus *und* auf Anfrage“.
- **anfragbare Ressource:** eine Ressource, die man mit `Get` abfragen kann (Liste in MG 9.1). `Event` gehört *nicht* dazu: Ein Ereignis passiert einmal, man kann es nicht nachträglich abfragen.
- **implizite REQ:** eine Regel, die in der Guideline ohne Schlüsselwort wie „MUST“ steht, aber offensichtlich verbindlich gemeint ist. Wir markieren sie mit `implicit: true` (so mit dir in SF-035 vereinbart).
- **`replaced_by`:** Eine zurückgezogene REQ zeigt auf ihre Nachfolgerin. Nummern werden nie wiederverwendet.
- **Vorab-Version `2.0.1-dev`:** der Arbeitsstand der Schemas nach 2.0.0. Die „1“ hinten heißt: nur eine kleine Änderung (hier nur die Spec-Baseline, kein Schema-Inhalt).

---

### D-1 – SF-103: die Beispiele in der Guideline passen nicht zur neuen Empfehlung

**Worum geht es?** Mit SF-101 steht jetzt in MG 8.3.8: Für anfragbare Ressourcen *sollen* Anwendungen die kombinierten Werte nehmen, also `ON_REQUEST_APPLICATION_SOURCE_FILTER_15` statt `APPLICATION_SOURCE_FILTER_8`. Beide bedeuten technisch dasselbe; der kombinierte Wert sagt nur ausdrücklich „man kann auch fragen“.

**Das Problem:** Die Beispiele der Guideline zeigen noch die kurze Form, und zwar 13 Einträge (12 in B1.1.8, 1 in B2.6). Wer die Beispiele abschreibt, folgt der Empfehlung also nicht. Die `Event`-Einträge (`FILTER_4`) sind richtig so.

**Die Wahl:**
- **(a) Beispiele anpassen** (Empfehlung): die 13 Einträge auf den kombinierten Wert ändern, `Event` bleibt. Entwickler schreiben Beispiele ab, also sollten sie die Empfehlung zeigen. Danach baseline ich neu; neue REQs erwarte ich nicht.
- **(b) so lassen:** Die Beispiele sind nicht falsch, nur nicht vorbildlich.

### D-2 – Womit geht es weiter?

P3 ist fertig. Jetzt könnten vier Phasen starten: **P4** (Rust-SDK), **P4b** (Micro-SDK für ESP32), **P5** (D7 prüft laufende Anwendungen) und **P5b** (D8 prüft Container). Fachlich hängen sie kaum voneinander ab, aber ich arbeite einen Zyklus nach dem anderen ab. Deshalb brauche ich eine Reihenfolge.

**Meine Empfehlung:**
1. **P4 zuerst.** Das Rust-SDK liegt auf dem „kritischen Pfad“: Ohne SDK keine Demos (P6), ohne Demos keine Registry (P7). Auch der Abschlusstest von D7 braucht eine Beispiel-App aus dem SDK. Die ersten Schritte (Identität, Topics, Nachrichtenmodell) laufen komplett offline.
2. **P5 dazwischen:** D7 lernt zuerst, MQTT-Mitschnitte zu lesen und offline auszuwerten; live geht es, sobald P4 den Test-Broker liefert.
3. **P4b und P5b danach** – oder früher, wenn dir die ESP32-Demo oder die Container-Prüfung wichtiger ist.

**Gut zu wissen:** Ab dem Test-Broker (P4, Schritt 5/9) brauche ich eine laufende Container-Umgebung (z. B. Docker Desktop). Die lief beim letzten Blick (14.10.) nicht.

### R-1 – REQ-RES-PUBLIST-026: kombinierte Werte nehmen (SHOULD)

Die Empfehlung aus SF-101 als REQ: Wer bei einer anfragbaren Ressource von sich aus veröffentlicht, *soll* den kombinierten `ON_REQUEST_…`-Wert nehmen. Prüfung: `manual`, weil beide Werte gültig sind und kein Tool einen Fehler melden darf. Später kann D7 beim Mithören eine *Warnung* ausgeben (Kandidat für P5, wenn D-1 entschieden ist).

### R-2 – REQ-RES-PUBLIST-027: `Get` geht immer, außer bei `OFF_0`

Die Klarstellung aus SF-101 als REQ: Eine anfragbare Ressource antwortet auf `Get` bei jedem Modus außer `OFF_0`. In der Guideline steht „can“; wir behandeln es als verbindlich (implizites MUST nach SF-035), weil sonst `Get` nicht verlässlich wäre. D7 prüft das im Test T7-GET-03.

### R-3 – REQ-TOP-028: ein Smart Device ist ein `OTConnector`

Die Regel aus SF-102 als REQ: Ein Gerät, das direkt am Message Bus hängt, meldet sich mit dem `ServiceType` `OTConnector`. Prüfung `manual`: D7 kann von außen nicht erkennen, ob ein Teilnehmer ein Gerät oder eine Software ist.

**Ein Detail dazu:** Genau diese Regel gab es schon einmal als REQ-TOP-007. Du hattest sie in C-020 zurückgezogen, weil die Guideline damals nichts dazu sagte. Jetzt sagt sie es. REQ-TOP-007 bleibt zurückgezogen (Nummern werden nie neu vergeben) und zeigt jetzt auf die Nachfolgerin REQ-TOP-028.

### R-4 – der genaue Wortlaut der zwei neuen Guideline-Sätze

**SF-102 (MG 7.2)** – wie vorgeschlagen: „A smart device connected directly to the Message Bus (1.3 Compliance Domains and Levels) uses the `ServiceType` `OTConnector`.“

**SF-101 (MG 8.3.8)** – **hier bin ich vom angenommenen Text abgewichen.** Vorgeschlagen war: „Jeder Modus außer `OFF_0` erlaubt `Get`.“ Das wäre aber für `Event` falsch: Dieselbe Seite sagt, dass man `Event`-Einträge nicht anfragen kann. Ich habe deshalb beide Sätze auf anfragbare Ressourcen eingeschränkt:

> „For resources that can be requested with `Get` (9.1 Resources), the `DataSetMessage` can be requested with `Get` with every `Mode` except `OFF_0`. For these resources, the values combining `ON_REQUEST_1` with publishing modes are equivalent to the publishing modes alone; publishers should use the combined values to make the availability on request explicit.“

**Bitte anschauen:** Ist die Einschränkung so in Ordnung? Wenn nicht, sag mir, wie es heißen soll; ich ändere den Text und baseline neu.

---

### Was nach deinen Antworten passiert (C-046)

1. Die angenommenen REQs setze ich auf `reviewed`.
2. Bei D-1 (a) passe ich die 13 Beispiele an, trage SF-103 als erledigt ein und baseline neu (Schema bleibt `2.0.1-dev`).
3. Ich starte die Phase, die du bei D-2 wählst, mit ihrem Planungszyklus.

---

## Begriffe aus den letzten Zyklen

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

## Was aus deinem letzten Review (C-042 … C-044) geworden ist

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
