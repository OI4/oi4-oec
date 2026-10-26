# Review und Entscheidungen – Erklärung auf Deutsch

_Stand: 26.10.2026, nach C-046 (P4-Start)._

Diese Datei erklärt die offenen Punkte aus [`REVIEW.md`](REVIEW.md) auf Deutsch und in Alltagssprache. Sie ist eine Lesehilfe auf Wunsch des PO (Ausnahme von der Regel „Projektartefakte auf Englisch“). **Maßgeblich ist `REVIEW.md`**; dort stehen auch die Checkboxen. Die Nummern (D-1, R-1 …) sind in beiden Dateien gleich, du kannst also z. B. im Chat antworten: „D-1 bis D-5 wie empfohlen, R-1 bis R-4 akzeptiert“.

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

**P4 hat begonnen.** In C-046 habe ich die ersten echten Bausteine des Rust-SDK geschrieben: Identität und Topics. Der Code hängt noch an keiner fremden Bibliothek. Alle gemeinsamen Testvektoren dazu laufen grün durch. Für dich gibt es:
- **5 Entscheidungen** (D-1 … D-5): wie das Rust-Projekt aufgebaut ist, wie die SDKs an die Testvektoren kommen, die Reihenfolge der nächsten Zyklen und zwei kleine Verhaltensfragen;
- **4 Review-Punkte** (R-1 … R-4): der neue Code, der JSON-Export, die Lizenz- und Sicherheitsprüfung der Abhängigkeiten und die Strenge der Code-Prüfung.

An REQs, Vektoren oder der Guideline hat sich nichts geändert. Die Lücken bleiben bei 0, und kein Spec-Finding ist offen.

Log: [C-046](log/2026-10-26_C-046_p4-start.md).

### Begriffe aus C-046

- **Workspace (Cargo):** ein Rust-Projekt mit mehreren Bibliotheken (Crates), die zusammen gebaut und getestet werden. Unseres liegt in `oec/sdk/rust`.
- **`oec-core`:** der innere Teil des SDK. Er kennt die Regeln des Protokolls (Topics, Identität, später Nachrichten), sendet aber selbst nichts („sans-IO“). Entwickler benutzen ihn nicht direkt, sondern über `oec`.
- **`oec`:** die eigentliche SDK-Bibliothek, die Entwickler einbinden (ab Inkrement 4). **`oec-ffi`:** die C-Schnittstelle (Inkrement 10).
- **Inkrement:** ein Baustein aus der Liste in ROADMAP P4 (1 Identität, 2 Topics, 3 Nachrichten, …).
- **Export:** JSON-Kopien unserer Testvektoren, automatisch aus den YAML-Dateien erzeugt.
- **cargo-deny:** ein Werkzeug, das alle fremden Rust-Bibliotheken prüft: erlaubte Lizenz? Bekannte Sicherheitslücke? Noch gepflegt? Nur eine TLS-Version?
- **clippy / pedantic:** clippy ist der Rust-„Lektor“ für Code. `pedantic` ist seine strengste übliche Stufe.
- **Warnung vs. Fehler:** Ein Fehler macht ein Topic ungültig. Eine Warnung sagt nur „das sollte man nicht tun“; das Topic bleibt gültig.

---

### D-1 – ADR-0026: Wie das Rust-Projekt aufgebaut ist

Die Ordnerstruktur im Repo entscheidest du (G-3.4). Mein Vorschlag:
- **Ein Rust-Projekt `oec/sdk/rust`** mit drei Bibliotheken nebeneinander: `oec-core` (Protokollregeln), `oec` (das SDK für Entwickler) und `oec-ffi` (C-Schnittstelle). So steht es schon in ADR-0011.
- **Die Skizze und die Spikes aus P3** bleiben vorerst daneben liegen. Die Skizze wird gelöscht, sobald `oec` ihre API wirklich kann.
- **`oec-core` ist intern.** Es macht keine Stabilitätsversprechen, damit wir es frei umbauen können. Was Entwickler sehen, ist nur `oec`.
- **Fehler** in `oec-core` sind genau (z. B. „Zeichen nicht erlaubt“, „zu lang“). `oec` gibt sie in der einfachen Form der angenommenen Skizze weiter.
- **Fremde Bibliotheken** kommen erst ab Inkrement 3, und nur die, die du in ADR-0023 schon angenommen hast.

**Empfehlung:** annehmen.

### D-2 – Die Testvektoren als JSON

Unsere Testvektoren liegen als YAML in `oec/spec/vectors`. Damit das Rust-SDK und später das C-SDK sie lesen können, ohne einen YAML-Leser einzubauen, erzeugt `spectool` jetzt automatisch eine **JSON-Kopie** in `oec/spec/export/`. `oec/check.sh` passt auf, dass die Kopie immer aktuell ist.

**Warum:**
- Der übliche Rust-YAML-Leser (`serde_yaml`) wird nicht mehr gepflegt.
- Für C müssten wir eine YAML-Bibliothek einbauen.
- JSON können beide sowieso.

Die YAML-Dateien bleiben das Original, das du reviewst.

**Empfehlung:** annehmen.

### D-3 – Reihenfolge in P4

Ich habe P4 in Zyklen geschnitten (ROADMAP P4):
- **C-046 (fertig):** Identität und Topics.
- **C-047:** Nachrichtenmodell (alle Payloads) und das Einbetten der Schemas. Dabei kommen die ersten fremden Bibliotheken dazu, deshalb ein eigener Zyklus.
- **C-048:** das `oec`-Crate mit App, Level und Start-Konfiguration.
- **Dann:** MQTT-Verbindung, gleich danach die **Testumgebung mit TLS-Broker** (Inkrement 9 vorgezogen, weil die Verbindung einen echten Broker zum Testen braucht).
- **Dazwischen P5:** D7 lernt, MQTT-Mitschnitte zu lesen.
- **Dann:** Daten anbieten, auf `Set`/`Del` reagieren, Daten anderer abonnieren (6–8), zuletzt die C-Schnittstelle (10).

Die **Exit-Kriterien** sind jetzt konkret: alle Funktionen aus ADR-0020/0021 sind da, alle Vektoren grün, Tests gegen den Broker (inkl. D7 `test`), C-Beispiele laufen, Prüfungen grün auf allen Plattformen, Beispiel-Apps dokumentiert.

**Gut zu wissen:** Ab der Testumgebung brauche ich eine laufende Container-Umgebung (Docker Desktop).

**Empfehlung:** annehmen.

### D-4 – `Oi4Identifier`: Maskierung nur auf Wunsch

Aus Hersteller-URI, Modell, Produktcode und Seriennummer baut das SDK die Identität eines Assets (`Oi4Identifier`). Laut Guideline *darf* man dabei Sonderzeichen mit der „dnp-encoding“ maskieren, muss aber nicht.

**Mein Vorschlag:**
- Standardmäßig maskiert das SDK **nicht**. Ein `/` in einem Teil ist dann ein Fehler, weil er die vier Teile durcheinanderbringen würde.
- Wer maskieren will, schreibt `.dnp_encoded()` dazu.

**Warum nicht immer maskieren?** Die Maskierung ändert die Identität des Assets. Das soll der Entwickler bewusst entscheiden.

**Empfehlung:** annehmen.

### D-5 – Leerzeichen im Filter: nur eine Warnung

Laut Guideline (MG 7.7) *sollen* Filter-Werte im Topic keine Leerzeichen oder Steuerzeichen enthalten. Das ist ein SHOULD, kein MUST.

**Mein Vorschlag:** Das SDK nimmt solche Topics an und meldet eine Warnung. D7 macht es genauso. Beim Veröffentlichen schreibt `oec` später eine Warnung ins Log und sendet trotzdem.

**Warum nicht ablehnen?** Dann würde das SDK aus einem SHOULD ein MUST machen.

**Empfehlung:** annehmen.

### R-1 – der neue Code (`oec-core`)

In `oec/sdk/rust/oec-core` stehen rund 2.300 Zeilen Rust:
- **Identität:** Prüfung von Identification Links (meldet alle Fehler auf einmal), base64url, Maskierung, `Oi4Identifier`.
- **Topics:** zerlegen, bauen und prüfen; Filter; Abo-Filter mit `+`/`#`.

Der Test liest alle passenden gemeinsamen Vektoren: 29 IL, 15 Maskierung, 13 `Oi4Identifier`, 151 Topics, die Topics aller gültigen Payload-Vektoren und die 66 Zeilen der Methoden-Matrix. **Alle grün.** Ungültige Topics bekommen dieselbe Fehlerart wie in D7.

Eine Code-Abdeckung in Prozent kann ich noch nicht messen, weil dem installierten Rust ein Werkzeug fehlt.

### R-2 – der JSON-Export

Format der Kopie: oben ein Hinweis „automatisch erzeugt, nicht bearbeiten“ und der Pfad der YAML-Datei, dann dieselben Felder wie im YAML. Leere Felder fallen weg; ein fehlendes Feld heißt also „leer“.

### R-3 – die Prüfung der fremden Bibliotheken (cargo-deny)

`oec/check.sh` prüft jetzt mit `cargo deny` alle fremden Rust-Bibliotheken nach den Regeln, die du in ADR-0023 angenommen hast:
- nur erlaubte Lizenzen;
- keine bekannten Sicherheitslücken;
- nichts Ungepflegtes;
- genau eine TLS-Bibliothek;
- kein OpenSSL;
- nur crates.io als Quelle.

Ohne Internet prüft es mit der zuletzt geladenen Datenbank. Neu nötig ist, dass `cargo-deny` installiert ist.

### R-4 – wie streng der Code geprüft wird

clippy läuft in der strengen Stufe `pedantic`, und jede Warnung bricht die Prüfung ab. Unsicherer Code (`unsafe`) ist verboten; nur die C-Schnittstelle bekommt später begründete Ausnahmen. Jede öffentliche Funktion braucht eine Doku.

Eine Regel habe ich abgeschaltet: Sie will Begriffe wie ManufacturerUri in Backticks setzen. Bei uns sind das normale Wörter aus der Guideline.

---

### Was nach deinen Antworten passiert (C-047)

1. ADR-0026 auf „accepted“ (oder wie du es änderst).
2. C-047: Nachrichtenmodell, Schemas einbetten, Payload-Vektoren; die ersten fremden Bibliotheken unter `cargo deny`.

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

## Was aus deinem letzten Review (C-045) geworden ist

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
