# Review und Entscheidungen – Erklärung auf Deutsch

_Stand: 26.10.2026, nach C-044 (P3-Entwurf: SDK-APIs, Bibliotheken, Prüftools)._

Diese Datei erklärt die offenen Punkte aus [`REVIEW.md`](REVIEW.md) auf Deutsch und in Alltagssprache. Sie ist eine Lesehilfe auf Wunsch des PO (Ausnahme von der Regel „Projektartefakte auf Englisch“). **Maßgeblich ist `REVIEW.md`**; dort stehen auch die Checkboxen. Die Nummern (D-1, R-1 …) sind in beiden Dateien gleich, du kannst also z. B. im Chat antworten: „D-1 bis D-19 wie empfohlen, R-1 bis R-5 akzeptiert“.

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

P3 ist die **Entscheidungsphase** für die SDKs und die Prüftools. Es gibt noch keinen fertigen Produktcode, sondern Skizzen, ADRs und Fragen an dich. Ich habe P3 selbst in Zyklen geteilt:
- **C-042:** die Programmierschnittstellen (APIs) der SDKs;
- **C-043:** die Bibliotheken;
- **C-044:** die Architektur der Prüftools.

Daraus ergeben sich:
- **19 Entscheidungen** (D-1 … D-19): der P3-Plan, 6 ADRs, 10 Fragen, 2 Spec-Findings;
- **5 Review-Punkte** (R-1 … R-5): Skizzen, Spikes, Testkatalog, Tool-Design.

Geändert habe ich nichts, was du schon freigegeben hattest: keine REQ, keinen Vektor, kein Schema. Die Abdeckung bleibt bei 0 Lücken, Schema 2.0.0 ist unverändert.

Logs: [C-042](log/2026-10-26_C-042_p3-sdk-api.md), [C-043](log/2026-10-26_C-043_p3-crates.md), [C-044](log/2026-10-26_C-044_p3-tools.md).

**Tipp zum Antworten:** Jedes ADR ist so geschrieben, als hättest du bei seinen Fragen die Empfehlung gewählt. Wählst du bei einer Frage etwas anderes, passe ich das ADR in C-045 an, bevor es auf „accepted“ geht. Wenn du mit allen Empfehlungen einverstanden bist, reicht: „D-1 bis D-19 wie empfohlen, R-1 bis R-5 akzeptiert“.

### Begriffe aus P3

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

---

### D-1 – Der P3-Plan

Der Auftrag war zu groß für einen einzigen Zyklus, den du sinnvoll reviewen kannst. Deshalb:
- C-042 SDK-APIs, C-043 Bibliotheken, C-044 Prüftools – das ist erledigt;
- **C-045** kommt nach deinen Antworten und schließt P3 ab.

In ROADMAP P3 stehen außerdem:
- **Eine Tabelle der offenen Punkte** aus den früheren ADRs, jeweils mit dem Zyklus, der sie schließt. Beispiele: die Crate-Wahl aus ADR-0011, die D9-Kandidaten aus ADR-0017, die Binary-Namen aus ADR-0014.
- **Die Abhängigkeiten:**
  - P4 (Rust-SDK) braucht D-2, D-3 und D-5.
  - P4b (Micro-SDK) braucht D-4 und D-6. Es hängt nicht von P4 ab und kann parallel laufen.
  - P5/P5b (Prüftools) brauchen D-7.
- **Fünf konkrete Exit-Kriterien.** Zwei davon prüft `oec/check.sh` schon automatisch: dass die Skizzen kompilieren und dass jede prüfbare REQ einen geplanten Test hat.

**Empfehlung:** annehmen.

### D-2 – ADR-0020: so sieht das Rust-SDK aus

Die wichtigsten Ideen:

1. **Man wählt das Level, das Protokoll macht das SDK.** Die Anwendung sagt „ich bin Level 2“. Das SDK bietet dann automatisch alles an, was Level 2 verlangt: MAM, Health, Profile, PublicationList usw. Passt eine Einstellung nicht zum Level, lehnt `build()` sie schon beim Start ab. Ein Beispiel ist ein Speicher, der beim Neustart verloren geht, obwohl die ReferenceDesignation bleiben muss.
2. **Die Anwendung setzt Zustände, das SDK verschickt Nachrichten.** Die App sagt nur „die Temperatur ist jetzt 21,3“. *Wann* und *wie oft* das veröffentlicht wird, entscheidet das SDK anhand der PublicationList: Modus, Intervall, Genauigkeit.
3. **`Get`, Seitenaufteilung (Pagination), Sprache (Locale) und Abfragen (Query)** erledigt das SDK komplett. Die App merkt davon nichts.
4. **`Set`/`Del`:** Die App entscheidet nur „erlaubt oder nicht“. Prüfen, Speichern, neu Veröffentlichen und das Status-Event als Antwort macht das SDK.
5. **Nur TLS:** Eine unverschlüsselte Verbindung kann man gar nicht erst einstellen.
6. Jede dieser Entscheidungen nennt die REQs, aus denen sie folgt. Ein Test prüft, dass es diese REQs wirklich gibt.

**Empfehlung:** annehmen.

### D-3 – ADR-0021: die C-Schnittstelle des Rust-SDK

Für die vielen Hersteller, die in C programmieren:
- **Wer räumt auf?** Was man mit `oec_…_new` bekommt, gibt man mit `oec_…_free` wieder frei. Alles andere gehört dem SDK.
- **Strukturierte Werte als JSON-Text.** Damit gibt es keine komplizierten C-Strukturen für zehn Ressourcen. Der C-Entwickler sieht genau das Format, das auch auf dem Bus steht.
- **Fehler** kommen als Rückgabecode, dazu eine Fehlermeldung je Thread.
- **Rückrufe (Callbacks)** laufen nacheinander auf einem Thread des SDK.
- **Das Verhalten ist exakt das des Rust-SDK.** Die C-Schicht ist nur eine dünne Hülle.

**Empfehlung:** annehmen.

### D-4 – ADR-0022: das Micro-SDK für kleine Geräte

Für Mikrocontroller wie den ESP32:
- **Kein dynamischer Speicher (kein Heap).** Alles ist statisch; der Speicherbedarf ist vorher bekannt.
- **Ein einziger Thread**, der Reihe nach abgearbeitet.
- **Plattformanpassung über die PAL** (siehe Begriffe).
- **Bei jeder Verbindung wird der „letzte Wille“ (Will) neu gesetzt**, wie die Guideline es verlangt.
- **Eine echte Uhrzeit ist Pflicht**, weil jede Nachricht einen Zeitstempel braucht.
- **Zuerst Level 1;** Level 2/3 kommen später als Ergänzung dazu.

**Empfehlung:** annehmen.

### D-5 – ADR-0023: Bibliotheken für das Rust-SDK

**Die wichtigste Erkenntnis:** Die MQTT-Bibliothek, mit der alle gerechnet hatten (`rumqttc`), wird praktisch nicht mehr gepflegt. Sie zieht eine TLS-Komponente mit fünf bekannten Sicherheitslücken nach sich, und Korrekturen werden nicht mehr übernommen. Deshalb schlage ich vor, den kleinen Teil von MQTT, den OEC braucht, selbst zu schreiben (siehe D-13).

Der Rest sind bewährte Standardbausteine:
- tokio für die Nebenläufigkeit;
- rustls für TLS 1.2/1.3;
- serde_json für JSON;
- die Schema-Prüfung (jsonschema) nur als Option für Tests, weil sie jedes Programm um ca. 3 MB größer macht;
- cargo-deny ab P4: Es stellt automatisch sicher, dass keine Bibliothek mit unpassender Lizenz hineinrutscht.

**Empfehlung:** annehmen.

### D-6 – ADR-0024: Bibliotheken für das Micro-SDK

- **ESP-IDF und esp-mqtt.** Wie man damit bei jedem Wiederverbinden einen neuen Will setzt, habe ich im Quellcode nachgelesen. Auf echter Hardware prüfen wir das in P4b.
- **JSON:** ein eigener, kleiner Leser ohne Heap (siehe D-14).
- **Arduino** nutzt dieselbe MQTT-Basis wie ESP-IDF.

**Hinweis:** ADR-0017 nannte „cJSON oder jsmn“ als Kandidaten und ließ die Wahl offen. Inzwischen wird jsmn nicht mehr gepflegt, und cJSON ist nicht mehr fest in ESP-IDF enthalten. ADR-0017 selbst habe ich nicht geändert. Nimmst du D-6 an, ergänze ich in C-045 einen Hinweis dort.

**Empfehlung:** annehmen.

### D-7 – ADR-0025: so funktionieren die Prüftools

**D7 `oec-check`** (prüft Anwendungen und Geräte) bekommt zu `validate` drei Funktionen dazu:
- **`monitor`:** hört passiv auf dem Bus mit und prüft jede Nachricht. Es kann eine Aufzeichnung speichern und später offline erneut abspielen; das ist gut für Fehlerberichte.
- **`test`:** prüft eine laufende Anwendung aktiv. Es schickt `Get`/`Set`/`Del`/`Call` und bewertet die Antworten. Was es dabei an der Anwendung ändert, stellt es danach wieder her.
- **`list` / `explain`:** zeigen den Testkatalog und erklären einzelne Tests oder REQs.

**D8 `oec-infra`** (prüft Container und Plattform) hat:
- `image`: das Container-Image offline;
- `run`: das Image probeweise starten;
- `container`: einen laufenden Container;
- `host`: den Rechner der Plattform;
- `bus`: Broker, TLS, Zugriffsrechte, Registry.

Diese Namen ersetzen die Arbeitstitel `inspect-…` aus der ROADMAP.

**Bewertung:** Ob ein Verstoß ein *Fehler* oder nur eine *Warnung* ist, hängt davon ab, ob die REQ beim geprüften Level MUST (Pflicht) oder SHOULD (Empfehlung) ist.

**Berichte** gibt es als Text, als JSON und als JUnit-XML für CI-Systeme.

**Unabhängigkeit:** Die Tools nutzen keinen SDK-Code (Q-009). Ein Test stellt das automatisch sicher.

**Empfehlung:** annehmen.

### D-8 bis D-17 – die offenen Fragen

| # | Frage | Worum es geht | Meine Empfehlung |
|---|---|---|---|
| D-8 | Q-021 | Muss man im Rust-SDK „async“ programmieren (eine anspruchsvollere Technik)? | **Nein, normal (blockierend) als Standard.** Async nur, wer es will. So ist es einfacher und gleich wie in C. |
| D-9 | Q-022 | Wie liest die App ihre Einstellungen (Config)? | **Erst einfach über Namen** (`number("Polling", "Interval")`), eine bequemere typisierte Variante später. |
| D-10 | Q-023 | Wie übergibt man in C Optionen? | **Über einzelne Setter-Funktionen und JSON-Text** statt über C-Strukturen. Das ist stabiler bei neuen Optionen. |
| D-11 | Q-024 | Schreiben wir das Logger-Demo in C? | **Ja.** So beweisen wir, dass der C-Weg für die vielen C-Hersteller wirklich funktioniert. Das Demo bleibt bei Level 1 und wird über Datei/Umgebung konfiguriert (wie in ADR-0016). |
| D-12 | Q-025 | Welches Namenspräfix bekommt das Micro-SDK? | **`oecm_`.** Kurz und ohne Namenskonflikt mit `oec_` des großen SDK. |
| D-13 | Q-026 | Welche MQTT-Bibliothek nutzt das Rust-SDK? | **Einen eigenen, kleinen MQTT-Teil.** OEC braucht nur einen kleinen, festen Ausschnitt von MQTT 3.1.1. Der Spike zeigt: ca. 300 Zeilen für den Kern, im Produkt ca. 1.000–1.500 Zeilen mit Tests. Die Alternativen sind ein junger Fork mit nur einem Betreuer oder eine C-Bibliothek mit OpenSSL (schwieriger zu bauen). |
| D-14 | Q-027 | Wie liest das Micro-SDK JSON? | **Mit einem eigenen kleinen Leser ohne Heap**, mit Zufallstests (Fuzzing) abgesichert. Die Alternative cJSON braucht mehr Speicher. |
| D-15 | Q-028 | Braucht man für eine volle Level-Bestätigung den Proxy-Modus? | **Ja.** Nur so sieht D7, ob die Anwendung TLS ≥ 1.2 nutzt, Zertifikate prüft und den Will setzt. Ohne Proxy werden diese Tests übersprungen, und das Ergebnis heißt „nicht schlüssig“ statt „bestanden“. |
| D-16 | Q-029 | Soll sich D7 beim Testen auf dem Bus wie eine Anwendung anmelden? | **Standardmäßig nein**, damit Testläufe keine Spuren in Registry und Logs hinterlassen. Mit `--announce` ist es möglich. |
| D-17 | Q-030 | Neuer Exit-Code 4 „nicht schlüssig“? | **Ja.** 0 = bestanden, 1 = Verstoß, 2 = falscher Aufruf, 3 = interner Fehler, **4 = kein Verstoß gefunden, aber Pflichttests konnten nicht laufen**. Sonst würde ein unvollständiger Lauf wie „bestanden“ aussehen. |

### D-18 – SF-101: `Get` und der PublicationList-Modus

In der Guideline steht, dass man bei `ON_REQUEST` die Daten per `Get` abfragen kann. Bei den anderen Modi (`APPLICATION`, `SOURCE`, `FILTER`) steht aber *auch*: „Zusätzlich kann man sie abfragen.“ Damit wären die kombinierten Modi (`ON_REQUEST_APPLICATION…`) überflüssig, und es ist unklar, ob `Get` ohne `ON_REQUEST` erlaubt ist.

**Mein Vorschlag:**
- Bei jedem Modus außer `OFF` darf man per `Get` abfragen.
- `ON_REQUEST` allein heißt „*nur* auf Anfrage, sonst nie veröffentlichen“.
- In den kombinierten Werten ist `ON_REQUEST` nur ein Hinweis.
- Dazu ein Satz in MG 8.3.8.

**Die Alternative:** `Get` nur mit `ON_REQUEST`. Dann bräuchten MAM und Health eine Ausnahme, weil sie ab Level 2 immer abfragbar sein müssen.

**Folge:** Setze ich den Text um, ändert sich der Guideline-Stand. Dann muss ich neu baselinen, und die Schemas bekommen formal eine Vorab-Version `2.0.1-dev`; inhaltlich bleiben sie gleich.

### D-19 – SF-102: welcher `ServiceType` für ein Smart Device?

Die Guideline erlaubt Geräte, die sich direkt mit dem Bus verbinden (z. B. ein Temperatursensor mit eigenem MQTT). Die sieben `ServiceType`-Werte beschreiben aber alle *Anwendungen*, und keiner passt richtig zu so einem Gerät.

**Mein Vorschlag:**
- Das Gerät nimmt **`OTConnector`**, denn es stellt ein OT-Asset (sich selbst) auf dem Bus bereit.
- Seine Geräteklasse (`DeviceClass`) ist die eigene Produktklasse.
- Dazu ein Satz in MG 7.2.

**Die Alternative:** ein neuer `ServiceType` `Device`. Das bräuchte eine neue Guideline-Version und eine Schema-Änderung (2.1.0).

---

### R-1 – Rust-Skizze (`oec/sdk/rust/sketch`)

Vier Beispiele zeigen, wie sich das SDK anfühlt:
- **„Hello World“ in 16 Zeilen;**
- ein Gerät als Datenquelle, das auf `Set` reagiert;
- Einstellungen (Config);
- die Gegenseite, die Daten abonniert (Consumer).

**Bitte anschauen:** Liest sich das so, wie du dir ein SDK für Hersteller wünschst?

### R-2 – C-Skizzen (`oec.h` für das große SDK, `oecm.h` für das Micro-SDK)

Beide Header und drei Beispielprogramme werden in `oec/check.sh` als C17 geprüft.

**Bitte anschauen:**
- die Regeln, wer welchen Speicher freigibt;
- die Gegenüberstellung, was bei `oec_` und `oecm_` gleich und was bewusst anders ist (in ADR-0022).

### R-3 – die zwei Spikes

- **`schema-embed`:** Die Schemas werden beim Bauen ins Programm eingebettet und liefern dieselben Ergebnisse wie unsere Schema-Prüfung (630 Fälle). Kosten: ca. 3 MB.
- **`mqtt-codec`:** Der eigene MQTT-Kern ist klein und testbar (Beleg für D-13).

Beides ist Wegwerf-Code und klar als Spike markiert.

### R-4 – der Testkatalog von D7/D8

**D7** hat 54 geplante Tests:
- 19 passive, die nur mithören;
- 35 aktive.

**D8** hat 17 Tests.

Jede REQ, die ein Tool prüfen kann, steckt in mindestens einem Test. Ein automatischer Test passt darauf auf. Kommt durch eine neue Guideline-Version eine REQ dazu, wird `oec/check.sh` rot, bis der Katalog nachgezogen ist.

5 REQs stehen bewusst auf „manuell“, mit Begründung:
- REQ-CMP-011/012: Das sind Regeln *für das Tool*, nämlich dass ohne Angabe Level 3 gilt.
- REQ-LIFE-011…013: Was ein Empfänger intern tut, kann man von außen nicht sehen.

**Bitte stichprobenartig anschauen:** die Gruppen T7-CON/T7-LIFE (Verbindung, Proxy) und T7-GET/T7-SET.

### R-5 – das Tool-Design (`oec/conformance/DESIGN.md`)

Die Details hinter D-7:
- Aufbau des Codes;
- alle Befehle und Optionen;
- Format der Aufzeichnung;
- was `test` an der Anwendung ändert und wie es das zurücksetzt;
- Inhalt der Berichte;
- Konfiguration;
- Selbsttest mit einer simulierten Anwendung;
- Verteilung als einzelne Programmdatei für Windows, macOS und Linux.

**Bitte anschauen:** Passen Befehlsnamen und Bedienung zu den Herstellern und Betreibern, die du im Kopf hast?

---

### Was nach deinen Antworten passiert (C-045)

1. Ich passe die ADRs an deine Antworten an und setze sie auf „accepted“.
2. Wenn du SF-101/102 annimmst, schreibe ich die Sätze in die Guideline und baseline neu.
3. Ich verfeinere die ROADMAP für P4/P4b/P5/P5b und schließe P3 ab. Danach können SDK, Micro-SDK und Prüftools parallel starten.

---

## Begriffe aus den letzten Zyklen

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

## Was aus deinem letzten Review (C-041) geworden ist

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
