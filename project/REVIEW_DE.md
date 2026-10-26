# Review und Entscheidungen – Erklärung auf Deutsch

_Stand: 26.10.2026, nach deinem Review von C-051 (P4 läuft). **Für dich offen: ein Review, R-1 (REQ-CNT-003 nach SF-105).**_

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

C-051 hat die **Entwicklungsumgebung** gebracht: einen echten Mosquitto-Broker in Docker mit TLS, selbst erzeugte Test-Zertifikate und Beispiel-Ordner für `/etc/oi4`. Dagegen laufen jetzt die **Interop-Tests** des SDK (8 grün, kein Fehler im SDK).

Deine Entscheidung zu D-1 (SF-105, „Vorschlag komplett“) ist umgesetzt, siehe unten „Was aus deinem letzten Review (C-051) geworden ist“. Dadurch hat sich eine reviewte REQ geändert. Die schaust du dir bitte noch einmal an (R-1).

### R-1 – REQ-CNT-003: neuer Text nach SF-105

**Worum geht es?** REQ-CNT-003 beschreibt den Inhalt von `broker.json` und was der Broker damit tun muss. Durch deine Entscheidung stehen in RIG 4.1.1 zwei neue Sätze. Die REQ muss ihnen folgen. Weil sich ihr Text ändert, steht sie wieder auf `draft`.

**Was sich ändert:**
- **Bisher** stand am Ende nur: „Der Broker nimmt Nutzlasten bis MaxPacketSize an.“ Das war von uns abgeleitet, die Guideline sagte es nicht ausdrücklich.
- **Neu** steht dort:
  - Der Broker nimmt jede Nutzlast bis `MaxPacketSize` an.
  - Begrenzt er das ganze Paket, muss seine Grenze um 65544 Byte größer sein (Platz für Kopf und Topic).
  - Nutzlast + Topic-Länge + 4 dürfen 268435455 nicht übersteigen.
- Dazu kommen die drei neuen Sätze als Zitate und die zwei neuen Guideline-Stellen in der Liste der Fundstellen.
- In den Notizen steht jetzt:
  - Zu große Nachrichten regeln wir bewusst nicht (Teil (c)). Der Broker darf sie verwerfen oder die App trennen.
  - Die Obergrenze aus (b) ist eine Eigenschaft von MQTT selbst; dafür braucht es keine eigene Prüfung.
  - D8 prüft nur, dass erlaubte Größen durchkommen (A-50).

**Was gleich bleibt:**
- „muss“ (MUST) und die Prüfung durch D8;
- das Schema für `broker.json` und die 18 `broker.json`-Vektoren, weil sich die Form der Datei nicht ändert.

Den vollständigen Text findest du in `oec/spec/requirements/view/cnt.md` unter REQ-CNT-003.

Antwort z. B. „R-1 akzeptiert“.

Die Punkte A-1 … A-50 in `REVIEW.md` sind nur zur Info. Das sind Technik-Entscheidungen, die ich nach deiner Regel (GUARDRAILS G-3.4 b) selbst treffe. Du kannst sie kippen, musst aber nicht antworten.

Die Lücken bleiben bei 0. Das Schema bleibt `2.0.1-dev`, hat jetzt aber die Baseline `59e3ae3`; am Inhalt ändert sich nichts.

**Nachtrag 1 (deine Anweisung):** In den Guardrails steht jetzt die neue Regel **G-3.10**. Am Ende jedes Zyklus entscheide ich, ob der nächste Zyklus mit einem neuen Agenten oder mit demselben weiterläuft, und schreibe den Grund ins Log.
- Einen **neuen Agenten** nehme ich in der Regel, wenn mein Kontext schon zusammengefasst wurde oder groß ist, oder wenn ein neues Inkrement, eine neue Phase oder ein neues Deliverable beginnt.
- **Derselbe Agent** macht nur kurze Nacharbeiten zum selben Thema weiter, z. B. die Antworten auf dein Review.

Bei einem neuen Agenten schreibe ich den Start-Prompt gleich in `prompt.md`. Diese Datei committe ich nie. Für C-048 habe ich mich für einen **neuen Agenten** entschieden, weil mein Kontext mehrfach zusammengefasst wurde und C-048 in einem neuen Crate arbeitet. `prompt.md` ist schon gefüllt.

Logs: [C-051](log/2026-10-26_C-051_dev-environment-interop.md), [C-050](log/2026-10-26_C-050_runtime-tokio-rustls.md), [C-049](log/2026-10-26_C-049_mqtt-session-lifecycle.md), [C-048](log/2026-10-26_C-048_oec-crate-bootstrap.md), [C-047](log/2026-10-26_C-047_message-model.md), [C-046 inkl. Nachtrag 1 bis 3](log/2026-10-26_C-046_p4-start.md).

### Begriffe aus C-051

- **Entwicklungsumgebung (`oec/dev`):** ein fertig eingerichteter Broker zum Ausprobieren auf dem eigenen Rechner. Gestartet wird er mit `docker compose up`; die Anleitung steht in `oec/dev/README.md`.
- **Mosquitto:** ein verbreiteter, freier MQTT-Broker. Wir nutzen Version 2.1.2 und haben das Image über seinen Digest (eine Prüfsumme) festgenagelt. Es kommt also immer genau dasselbe Image, auch wenn jemand den Versions-Namen neu belegt.
- **Listener:** ein Port, an dem der Broker Verbindungen annimmt. Wir haben zwei, beide nur mit TLS:
  - 8883 für Anmeldung mit Benutzername und Passwort;
  - 8884 für Anmeldung mit Client-Zertifikat.
- **Dev-PKI:** die Test-Zertifikate (eine eigene CA, ein Zertifikat für den Broker, Zertifikate für die Apps). Ein kleines Go-Programm erzeugt sie mit `go run ./pki`. Sie landen im Ordner `generated/`, der nie committet wird. Jeder erzeugt sich seine eigenen.
- **Interop-Test:** ein Test, ob unser SDK mit einem fremden, echten Broker zusammenarbeitet. Er braucht Docker und läuft deshalb nur auf Wunsch über `oec/dev/interop.sh`. Ohne Docker meldet das Skript „übersprungen“ und bricht nichts. Die normale Prüfung `oec/check.sh` braucht weiter kein Docker.
- **Probe:** ein zweiter, mithörender MQTT-Client in den Tests. Er sieht, was die App wirklich am Broker veröffentlicht, so wie eine andere App auf dem Bus.
- **Kindprozess:** Um den Will zu testen, muss die App „hart sterben“, ohne sich abzumelden. Der Test startet die App deshalb als eigenen Prozess und beendet ihn gewaltsam (wie `kill -9`). Dann muss der Broker den Will verschicken.

### Begriffe aus C-050

- **Laufzeit (Runtime):** der Teil des SDK, der wirklich arbeitet: Netzwerkverbindung öffnen, Bytes senden und empfangen, Zeit messen, Wartezeiten einhalten. Er „treibt“ die reine Logik aus C-049 (Sitzung und Lebenszyklus) an.
- **tokio:** die verbreitetste Rust-Bibliothek für solche Laufzeiten. Sie kann viele Verbindungen und Zeitgeber gleichzeitig verwalten, ohne für jede einen eigenen Thread zu brauchen.
- **TLS / rustls:** TLS verschlüsselt die Verbindung zum Broker und prüft per Zertifikat, dass es wirklich der richtige Broker ist. rustls ist eine TLS-Bibliothek in Rust. Die eigentliche Kryptografie liefert eine zweite Bibliothek: aws-lc-rs (Standard) oder ring (wählbar).
- **CA (Zertifizierungsstelle):** das Zertifikat, dem man vertraut. Nur Broker, deren Zertifikat von dieser CA unterschrieben ist, werden akzeptiert. Wir vertrauen **nur** der CA aus dem Container-Speicher bzw. der, die die App mitgibt, nie den vielen CAs des Betriebssystems.
- **Client-Zertifikat:** die App weist sich beim Broker mit einem eigenen Zertifikat aus statt mit Benutzername und Passwort. Ist eines da, hat es Vorrang (REQ-MQTT-010).
- **Passphrase:** ein Passwort, mit dem der private Schlüssel der App verschlüsselt auf der Platte liegt. Das SDK entschlüsselt ihn beim Start.
- **Backoff:** Bricht die Verbindung ab, versucht die App es erneut: erst nach etwa 1 Sekunde, dann 2, 4, 8 … höchstens 60 Sekunden. Ein Zufallsanteil sorgt dafür, dass nach einem Broker-Neustart nicht alle Apps im selben Moment anklopfen.
- **Test-Broker:** ein kleiner Broker, den die Tests selbst starten, nur auf dem eigenen Rechner. Er versteht genug MQTT, um zu prüfen, was die App schickt, und kann absichtlich Fehler machen (Verbindung kappen, Anmeldung ablehnen).
- **Smoke-Test:** ein kurzer Test gegen einen echten Broker (z. B. Mosquitto). Er läuft nur, wenn man ihn ausdrücklich einschaltet; die normale Prüfung braucht keinen Broker und kein Docker.

### Begriffe aus C-049

- **MQTT:** das Protokoll, über das alle OEC-Anwendungen mit dem Broker (der „Post-Zentrale“) sprechen. Es besteht aus wenigen Paketarten: CONNECT (anmelden), PUBLISH (Nachricht senden), SUBSCRIBE (Thema abonnieren), PING (lebe ich noch?), DISCONNECT (sauber abmelden) und den passenden Antworten.
- **Codec:** der Teil, der diese Pakete in Bytes umwandelt (kodieren) und Bytes wieder in Pakete (dekodieren). Unser Codec ist „kanonisch“: Es gibt für jedes Paket genau eine richtige Byte-Folge; alles andere lehnt er mit einer Begründung ab, die die verletzte Regel nennt.
- **Session (Sitzung):** die Logik rund um eine Verbindung: anmelden, Keep Alive (regelmäßig „lebe noch“ senden und auf Antwort warten), Bestätigungen für wichtige Nachrichten (QoS 1) zählen, Abos nach einem Neuaufbau wiederherstellen.
- **Sans-IO / Zustandsmaschine:** Die Session öffnet selbst keine Netzwerkverbindung und hat keine eigene Uhr. Man gibt ihr empfangene Bytes und die aktuelle Zeit, sie gibt zu sendende Bytes und Ereignisse zurück. Dadurch ist sie ohne Netzwerk komplett testbar; das Netzwerk kommt in C-050 dazu.
- **Will („letzter Wille“):** eine Nachricht, die die App beim Anmelden beim Broker hinterlegt. Bricht die Verbindung unerwartet ab, verschickt der Broker sie (Health FAILURE_1/0). Bei einem sauberen Abmelden verwirft er sie. Jede Anmeldung bekommt einen neuen Will (REQ-LIFE-016).
- **Birth / Close:** Birth ist die erste Nachricht nach dem Anmelden (das MAM, „hier bin ich“); Close ist die letzte vor dem sauberen Abmelden (Health NORMAL_0/0).
- **Heartbeat:** Health wird alle 60 Sekunden gesendet und zusätzlich sofort bei jeder Änderung.
- **Property-Tests (proptest):** Statt einzelner Beispiele erzeugt der Test tausende Zufallsfälle und prüft Grundregeln, z. B. „was kodiert wird, kommt beim Dekodieren gleich zurück“ oder „kaputte Bytes führen nie zum Absturz“.
- **cargo-fuzz:** ein ähnliches Werkzeug, das noch tiefer sucht, aber eine Test-Version des Rust-Compilers (nightly) braucht. Deshalb nicht in der automatischen Prüfung.

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
- **A-10 – toleranter, aber typisierter Leser:** Unbekannte Felder werden ignoriert. Abgelehnt wird nur, was nicht in den Typ passt: falscher JSON-Typ, fehlendes Pflichtfeld, unbekannter Aufzählungswert, verletzte Grundregel (z. B. falsche DataSetClassId, reservierte DataSetWriterId). Feinheiten wie Textmuster prüft das Schema. Von den 143 ungültigen Vektoren lehnt der Leser 63 selbst ab, den Rest erkennen Schema oder Regelschicht.
- **A-11 – Schreiben nach fester Feldliste:** Das SDK schreibt immer genau die Felder der Guideline. `MessageType` im Datensatz wird immer mitgeschickt, `ReplyTo` ist `""`, wenn die Nachricht keine Antwort ist. Die DataSetClassId der Standard-Ressourcen setzt das SDK selbst.
- **A-12 – nichts geht verloren:** Zeitstempel und Binärdaten (`CorrelationData`) behalten ihren Originaltext. So kommt `CorrelationData` Byte für Byte unverändert zurück, wie REQ-ACT-007 es verlangt.
- **A-13 – Actions:** Wer eine Anfrage (Call) baut, bekommt Antwortadresse und Absender automatisch aus dem Topic. Die Antwort wird aus der Anfrage erzeugt und übernimmt alle Kennungen. Bei einem Fehlercode ist die Antwort leer, wie die Guideline es verlangt.
- **A-14 – Form einzelner Ressourcen:** Config-Gruppen sind frei benannt und werden deshalb als Tabelle (Name → Wert) gespeichert. Data ist ein freies JSON-Objekt mit einer bequemen Sicht für `Oi4Pv`. Der `TopicPath` der SubscriptionList bleibt Text, weil er beliebige MQTT-Filter enthalten darf.
- **A-15 – Schema und Bibliotheken:** Das SDK meldet die Schema-Version des eingebauten Stands (`2.0.1-dev` @ `734c560`), wie du es für C-047 vorgegeben hast. Die tolerante Schema-Variante wird beim Start im Speicher erzeugt statt ein zweites Mal eingebaut. Die neuen Bibliotheken sind auf das Nötigste beschränkt (20 Crates, 81 mit dem Schalter). cargo-deny ist ohne jede Ausnahme grün: alle Lizenzen erlaubt, keine bekannte Lücke. Die automatische Prüfung läuft jetzt mit und ohne den Schalter.
- **A-16 – Zyklusschnitt:** C-047 wurde nicht geteilt. Modell, Schema-Prüfung und Vektor-Test prüfen sich gegenseitig und gehören zusammen. Der Plan für die nächsten Zyklen bleibt gleich.

Neu aus C-048 (die eigentliche SDK-Bibliothek `oec`):
- **A-17 – Aufbau und Fehler:** `oec` gibt die fertigen Bausteine aus `oec-core` direkt weiter und verpackt nur die, deren Fehler Teil der API sind. Es gibt einen einzigen Fehlertyp `Error` mit den sechs Arten aus ADR-0020 (`Invalid`, `LevelNotMet`, `Bootstrap`, `Io`, `Timeout`, `Closed`). Fehler aus `oec-core` werden zu `Invalid` und behalten ihre Fehlercodes. Dateizugriffe gibt es nur in `oec`; `oec-core` bleibt reine Rechenlogik ohne Ein-/Ausgabe.
- **A-18 – Container-Speichervertrag:** Der Container-Name kommt aus dem Hostnamen und muss ein gültiger Hostname sein. In `broker.json` müssen Port und Paketgröße ganze Zahlen sein: `8883.0` lehnen wir ab (wie unser Go-Prüfer), obwohl das Schema es durchließe; kein Vektor betrifft das. Ohne CA-Zertifikat (`broker_ca.pem`, sonst `ca.pem`) startet die App im Container nicht, denn Vertrauen kommt nur aus dem Speicher, nicht aus dem Betriebssystem. Liegen Zertifikat und Passwort beide vor, gewinnt das Zertifikat.
- **A-19 – Gerät ohne Container:** Die Seriennummer ist Pflicht. Ohne Vorgabe wird die MQTT-ClientId aus einem Hash des Identification Link gebildet; sie bleibt so über Neustarts gleich. Ein Broker ohne Anmeldung (weder Passwort noch Zertifikat) wird mit `LevelNotMet` abgelehnt, weil jedes Level Anmeldung verlangt.
- **A-20 – Level und Speicher:** Neuer Schalter `reference_designation(bool)`: auf Level 1 aus, auf Level 2/3 an; auf Level 3 lässt er sich nicht abschalten (REQ-CMP-020). Wer die ReferenceDesignation anbietet, braucht einen dauerhaften Speicher (REQ-RES-REFDES-002); ein `MemoryStore` reicht dann nicht und `build()` meldet `LevelNotMet`. `MemoryStore` wird jetzt mit `MemoryStore::new()` erzeugt (kleine API-Änderung, in ADR-0020 nachgetragen).
- **A-21 – Konfiguration:** Die Config-Definition wird beim Bauen geprüft (Namen, Typen, Standardwerte). Werte liest man über Namen als Text-Schlüssel. Textmuster (`Pattern`) werden erst ab Inkrement 7 geprüft, weil dafür eine Regex-Bibliothek nötig ist.
- **A-22 – Kleinkram mit Wirkung:** Gespeichert wird „atomar“ (erst in eine Hilfsdatei, dann umbenennen), damit bei Stromausfall nie eine halbe Datei bleibt. Keep Alive: 1 bis 65535 Sekunden; 0 würde das Will-Signal bei Verbindungsabbruch abschalten. Passwörter erscheinen nie im Debug-Text.
- **A-23 – Bibliotheken:** Nur thiserror und log kamen dazu, beide mit erlaubter Lizenz (MIT/Apache-2.0). cargo-deny ist ohne Ausnahme grün. Base64 dekodieren wir selbst (ein paar Zeilen), statt eine weitere Bibliothek zu holen.
- **A-24 – Zyklusschnitt:** C-048 wurde nicht geteilt. Inkrement 5 (Verbindung) ist dagegen zu groß für einen Zyklus und wird geteilt: **C-049** baut die reine Protokoll-Logik (MQTT-Nachrichten, Sitzung, Lebenszyklus; testbar ohne Netzwerk), **C-050** das Netzwerk (tokio, TLS) und `App::run`. Bis dahin bleiben `run`/`run_async` in der Skizze.
- **Offen für später (Technik, nicht für dich):** verschlüsselte private Schlüssel mit Passphrase und die CA eines Geräts ohne Container (C-050, inzwischen entschieden: A-33, A-34), Regex für `Pattern` (Inkrement 7), Länge der Speicher-Schlüssel für Assets (Inkrement 6).

Neu aus C-049 (MQTT und Lebenszyklus, noch ohne Netzwerk):
- **A-25 (neu) – MQTT-Codec:** Wir haben den MQTT-Teil selbst geschrieben, wie in ADR-0023 (Q-026) beschlossen; der Spike war nur Vorlage. Der Codec kann alle Pakete, die eine App braucht, aber kein QoS 2 und keinen gespeicherten („retained“) Will, beides verbietet die Guideline. Er ist streng: Jede Byte-Folge, die nicht exakt dem Standard entspricht, wird abgelehnt, und der Fehler nennt die Regel (z. B. `MQTT-3.1.2-22` oder REQ-MQTT-017).
- **A-26 (neu) – Sitzung:** Die Sitzung meldet sich bei jedem Verbindungsaufbau mit einem neuen Will an. Sie sendet nach einer Minute Stille einen PING; kommt keine Antwort, gilt die Verbindung als tot. Bei jedem Fehler trennt sie **ohne** sauberes Abmelden, damit der Broker den Will verschickt und alle sehen, dass die App weg ist. Abos werden nach einem Neuaufbau automatisch wiederhergestellt. Unbestätigte wichtige Nachrichten meldet sie als „verloren“, die Laufzeit entscheidet dann, ob sie neu gesendet werden. Zu große Nachrichten (über der MaxPacketSize des Brokers) lehnt sie ab.
- **A-27 (neu) – Form der Lebenszyklus-Nachrichten:** Will, Birth und Close sehen genau so aus wie die Beispiele in MG 6.3. Dort fehlt das Feld `MessageType` im Datensatz; laut REQ-MSG-029 bedeutet das „Keyframe“. Deshalb lassen wir es bei diesen Nachrichten weg, obwohl das SDK es sonst immer mitschickt (A-11). Beide Formen sind gültig.
- **A-28 (neu) – Regeln für Health:** Gibt es eine Detail-Liste, muss Health **genau** dem schlimmsten Eintrag entsprechen (REQ-RES-HEALTH-002 sagt „equals“). In der Skizze stand „mindestens“; das korrigieren wir in C-050 (in ADR-0020 nachgetragen). NORMAL_0 mit Score 0 darf die App nicht selbst setzen, weil das „ich melde mich ab“ bedeutet (REQ-LIFE-014). War die App auf FAILURE_1/0 und erholt sich, sendet sie zuerst wieder eine Birth (REQ-LIFE-017).
- **A-29 (neu) – MessageIds:** Jede Nachricht bekommt eine eindeutige Kennung aus Zeit und Absender. Kommen mehrere in derselben Millisekunde, wird eine laufende Zahl direkt an den Zeitstempel angehängt (aus `…123-App` wird `…1231-App`, `…1232-App` usw., so verlangt es REQ-MSG-007). Korrektur in C-050: Hier stand vorher fälschlich `-1`, `-2`; der Code war schon richtig, nur die Beschreibung nicht. Springt die Uhr zurück, bleibt die Kennung trotzdem eindeutig und aufsteigend.
- **A-30 (neu) – Zufallstests:** Für die Tests mit Zufallseingaben nehmen wir proptest. Es wird nur beim Testen gebraucht und ist nicht im fertigen SDK enthalten; die Lizenz ist MIT/Apache-2.0, cargo-deny ist ohne Ausnahme grün. cargo-fuzz haben wir nicht eingerichtet, weil es eine Test-Version des Compilers braucht und die automatische Prüfung sonst nicht durchliefe.
- **A-31 (neu) – Übergabe an C-050 und Zyklusschnitt:** `oec-core` hat eigene Verbindungs-Einstellungen; die Übersetzung aus den Einstellungen von `oec` (`ConnectionSettings`) schreibt C-050 zusammen mit dem Netzwerkteil. C-049 wurde nicht geteilt, der Plan bleibt gleich.
- **Offen für später (Technik, nicht für dich):** verschlüsselte private Schlüssel mit Passphrase und die CA eines Geräts ohne Container (unverändert, C-050), Birth als erste Nachricht auch bei einer dauerhaften Sitzung (C-050), Health setzen, während Details gesetzt sind (C-050). Die ersten drei hat C-050 entschieden (A-33, A-34, A-35); das letzte kommt mit Inkrement 6.

Neu aus C-050 (das Netzwerk, `App::run`):
- **A-32 (neu) – TLS-Verbindung:** Nur TLS 1.3 und 1.2, ältere Versionen lehnen wir ab (REQ-MQTT-005). Vertraut wird nur der CA aus dem Speicher bzw. aus dem mitgegebenen `Broker`. Hat die App ein Client-Zertifikat, meldet sie sich damit an und schickt kein Passwort mit. Zertifikate werden vor jedem Neuaufbau frisch eingelesen, damit erneuerte Zertifikate ohne Neustart greifen. Sind sie dabei kaputt, nimmt die App die letzten guten.
- **A-33 (neu) – Schlüssel mit Passphrase:** Unterstützt wird das heute übliche Format (PKCS#8, so schreibt es OpenSSL 3). Ich habe das mit echten OpenSSL-Schlüsseln ausprobiert. Alte oder schwache Verfahren (das alte OpenSSL-Format, SHA-1) lehnt das SDK mit einer klaren Meldung ab, inklusive Hinweis, wie man den Schlüssel umwandelt. Die Guideline schreibt kein Format vor; deshalb ist das eine Technik-Entscheidung und kein Finding.
- **A-34 (neu) – Eigener Broker braucht eine CA:** Gibt eine App ohne Container den Broker selbst im Code an, muss sie auch das CA-Zertifikat mitgeben, sonst meldet `build()` einen Fehler. Die Alternative wäre, den CAs des Betriebssystems zu vertrauen; das würde aber jeden Broker mit einem gekauften Zertifikat akzeptieren und widerspricht der Regel „Vertrauen nur aus dem Speicher“. Die Skizze erlaubte einen Broker ohne CA; deshalb steht das als Nachtrag in ADR-0020. Wenn du die Betriebssystem-CAs als Option möchtest, kann man sie später ergänzen, ohne etwas kaputt zu machen.
- **A-35 (neu) – Birth zuerst:** Die App meldet sich immer mit „frischer Sitzung“ an (Clean Session). Dann hat der Broker keine alten Nachrichten für sie, die vor der Birth ankommen könnten. Meldet der Broker trotzdem eine alte Sitzung, bricht die App die Verbindung ab. Die Tests prüfen: erst Birth, dann Health.
- **A-36 (neu) – Neuaufbau und Wartezeiten:** Wartezeit zwischen Versuchen von 1 bis 60 Sekunden mit Zufallsanteil (siehe „Backoff“). Die App gibt nie auf. Lehnt der Broker die Anmeldung ab (z. B. falsches Passwort), wird das als Fehler geloggt und weiter probiert, denn die Plattform kann das Passwort reparieren, ohne dass die App neu startet. Verbindungsaufbau und Senden dürfen höchstens so lange dauern wie das Keep-Alive-Intervall; für das saubere Abmelden gibt es 5 Sekunden.
- **A-37 (neu) – `run` und `run_async`:** `run` bringt alles selbst mit und beendet sich sauber bei Strg-C bzw. wenn der Container gestoppt wird (SIGTERM): erst Close, dann Abmelden. `run_async` ist für Apps, die selbst tokio benutzen. Dort fängt das SDK **keine** Signale ab, denn die gehören der App; sie beendet das SDK über `AppHandle::shutdown`. In ADR-0020 stand „wie `run`“; das ist jetzt als Nachtrag präzisiert.
- **A-38 (neu) – Verbindungsstatus:** `Closed` (vor und nach `run`), `Connecting` (erste Anmeldung läuft), `Online`, `Reconnecting` (nach einem Abbruch). `on_connection` meldet jede Änderung. `shutdown` darf man beliebig oft aufrufen, auch vor `run`. Empfangene Nachrichten werden vorerst ignoriert; das Lesen und Beantworten kommt mit den Inkrementen 6 bis 8.
- **A-39 (neu) – Tests:** 12 Tests gegen den Test-Broker mit selbst erzeugten Zertifikaten. Sie prüfen u. a. die Reihenfolge Birth → Health → Close, Neuaufbau mit neuem Will und neuer Birth, Trennen ohne Abmelden, wenn der Broker nicht mehr antwortet (damit der Broker den Will verschickt), abgelehnte Anmeldung, falschen Broker, Client-Zertifikat, verschlüsselten Schlüssel und Beenden. Sie dauern etwa 3 Sekunden. Einmal habe ich zusätzlich von Hand gegen einen echten Mosquitto in Docker getestet; die Reihenfolge stimmte. Die richtigen Tests gegen Mosquitto kommen mit Inkrement 9.
- **A-40 (neu) – Bibliotheken:** tokio, rustls, tokio-rustls, rustls-pki-types und pkcs8 (für die Passphrase), alle in ADR-0023 erlaubt bzw. dort nachgetragen; zum Testen rcgen (erzeugt Zertifikate). Das SDK zieht damit 73 statt 22 Bibliotheken nach sich, vor allem wegen tokio und der Kryptografie. Alle Lizenzen sind auf der erlaubten Liste, cargo-deny ist ohne Ausnahme grün. Wer statt aws-lc-rs lieber ring nimmt (z. B. weil dafür kein C-Compiler mit Assembler nötig ist), schaltet das Feature `crypto-ring` ein.
- **A-41 (neu) – Zyklusschnitt und Kleinkram:** C-050 wurde nicht geteilt. Als Nächstes kommt Inkrement 9 (Entwicklungsumgebung mit echtem Broker). `set_health` zieht erst mit Inkrement 6 aus der Skizze ins SDK; in der Skizze steht aber schon „genau“ statt „mindestens“ (A-28). Den Schreibfehler bei A-29 habe ich in ADR-0026 und hier korrigiert.


Neu aus C-051 (Entwicklungsumgebung und Interop-Tests):
- **A-42 (neu) – Entwicklungsumgebung und Zertifikate:**
  - Die Test-Zertifikate erzeugt ein kleines Go-Programm statt eines Shell-Skripts mit `openssl`. Grund: `openssl` verhält sich auf macOS, Linux und Windows unterschiedlich, Go läuft überall gleich.
  - Es braucht keine fremde Bibliothek.
  - Seine Tests prüfen auch die Broker-Einstellungen und laufen in der normalen Prüfung mit, ohne Docker.
- **A-43 (neu) – Broker:**
  - Mosquitto 2.1.2, fest über den Digest.
  - Zwei TLS-Ports: 8883 mit Passwort, 8884 mit Client-Zertifikat. Der Name im Zertifikat ist dort der Benutzername.
  - TLS 1.2 ist das Minimum, 1.3 geht auch. TLS 1.0/1.1 lehnt der Broker ab; das habe ich mit OpenSSL nachgeprüft.
  - Kein unverschlüsselter Zugang, keine Anmeldung ohne Passwort.
  - Die Ports sind nur auf dem eigenen Rechner erreichbar.
- **A-44 (neu) – Größengrenzen am Broker:** So eingestellt, wie ich es in SF-105 vorschlage:
  - Nutzlast bis 262144 Byte (= `MaxPacketSize`);
  - das ganze Paket bis 262144 + 65544 Byte.
  - Ein Test prüft beides.
- **A-45 (neu) – Beispiel-Ordner für `/etc/oi4`:** Es gibt zwei fertige Beispiele:
  - eine App mit Passwort;
  - eine App mit Client-Zertifikat und verschlüsseltem Schlüssel.
  - Das SDK liest sie direkt ein. `broker.json` hat genau die Form unserer Testvektoren, es ist kein neues Format.
- **A-46 (neu) – Nur auf Wunsch:**
  - Die Interop-Tests laufen nur mit `OEC_INTEROP=1`, und das setzt `interop.sh`.
  - Ohne Docker meldet das Skript „übersprungen“ und endet ohne Fehler.
  - Podman geht über die Variable `OEC_COMPOSE`.
  - Images werden nur in diesem Pfad heruntergeladen.
- **A-47 (neu) – Wie die Tests prüfen:**
  - Ein mithörender Client (Probe) sieht, was am Broker ankommt.
  - Für den Will wird die App als Kindprozess hart beendet.
  - Beim Broker-Neustart werden alte, vom Broker nochmals zugestellte Nachrichten an ihrer Zeit in der MessageId erkannt und übersprungen. So kann kein alter Will den Test fälschlich bestehen lassen.
- **A-48 (neu) – Smoke-Test ersetzt, keine neuen Bibliotheken:**
  - Der Smoke-Test aus C-050 ist durch den Interop-Test `explicit_broker` ersetzt.
  - Es gibt keine neue Rust- oder Go-Bibliothek.
  - Im SDK wurde kein Fehler gefunden. Zwei Fehler in meinen eigenen Tests habe ich vor dem Commit behoben.
- **A-49 (neu) – Zyklusschnitt:**
  - C-051 wurde nicht geteilt.
  - Als Nächstes kommt der P5-Einschub (D7: `monitor`, erst offline mit aufgezeichneten Nachrichten, dann live gegen diesen Broker), danach die Inkremente 6 bis 8 und 10.
  - Erledigt aus den C-050-Risiken: Zusammenspiel mit einem echten Broker und Neuaufbau nach einem Broker-Neustart.

Neu aus deinem Review von C-051:

- **A-50 (neu) – D8 prüft die Paketgröße am Broker:**
  - Der geplante D8-Test T8-BUS-01 deckt jetzt auch REQ-CNT-003 ab.
  - Er schickt eine Nachricht mit genau `MaxPacketSize` Nutzlast über ein langes Topic. Sie muss ankommen, und die Verbindung muss stehen bleiben.
  - Größere Nachrichten prüft er nicht (Teil (c)).
  - Bisher hat D8 REQ-CNT-003 nur am Host geprüft, also nur den Inhalt von `broker.json`.

---

### Was als Nächstes passiert (C-052)

1. **Dein Review von R-1** (REQ-CNT-003).
2. **P5-Einschub – D7** (das Compliance-Tool in Go):
   - eine eigene MQTT-Anbindung (`internal/mqttwire`);
   - Aufzeichnen und Abspielen von Nachrichten (`internal/capture`, `monitor --replay`);
   - danach `monitor` live gegen den Dev-Broker aus C-051.
   - Läuft in einem **neuen Agenten**; der Start-Prompt steht in `prompt.md`.

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

## Was aus deinem letzten Review (C-051) geworden ist

Deine erste Antwort war „Option b, da sie präzise ist“. (b) war aber nur ein Teil meines Vorschlags. Darum habe ich nachgefragt und bis zu deiner Antwort nichts an der Guideline geändert. Deine Präzisierung lautete „Vorschlag komplett“, also (a) + (b) + (c). Daraus wurde Nachtrag 1 zu C-051:
- **Guideline-Text:** In RIG 4.1.1, Karte `MaxPacketSize`, stehen jetzt zwei neue Sätze (`59e3ae3`):
  - „Nutzlast + Topic-Länge + 4 dürfen 268435455 nicht übersteigen.“
  - „Der Broker nimmt jede Nutzlast bis `MaxPacketSize` an; begrenzt er das ganze Paket, muss die Grenze um 65544 Byte größer sein.“
  - Zu zu großen Nachrichten steht bewusst nichts (Teil (c)).
- **Register:** SF-105 ist „resolved“ (`d220c48`). Es gibt kein offenes Finding mehr; das nächste heißt SF-106.
- **Neu-Baseline auf `59e3ae3`:** Die Zahl der Guideline-Stellen steigt von 1399 auf 1401. Die zwei neuen gehören zu REQ-CNT-003; die ist jetzt wieder `draft` (R-1).
- **Schema:** Version `2.0.1-dev` mit Baseline `59e3ae3`, am Inhalt ändert sich nichts.
- **Dev-Broker, SDK und Interop-Test** waren schon so gebaut. Daran ändert sich nichts.

Zur Erinnerung, worum es ging:

#### D-1 – SF-105: Wie groß darf eine Nachricht am Broker sein? (RIG 4.1.1)

**Worum geht es?** In `broker.json` steht `MaxPacketSize`. Laut Guideline ist das die größte erlaubte **Nutzlast** (Payload) einer MQTT-Nachricht, also nur der JSON-Teil. So haben wir es in SF-091 auch festgelegt. Die Guideline sagt aber nicht, wie die Plattform den **Broker** einstellen soll.

**Das Problem:**
- Broker begrenzen meist das **ganze Paket**, nicht nur die Nutzlast. Bei Mosquitto heißt die Einstellung `max_packet_size`, und in MQTT 5 ist es genauso.
- Zum Paket gehören aber außer der Nutzlast noch Kopf und Topic, zusammen bis zu 65544 Byte.
- Stellt eine Plattform den Broker-Wert einfach auf `MaxPacketSize`, dann wirft der Broker eine App hinaus, die sich genau an die Regel hält und eine Nutzlast von genau `MaxPacketSize` schickt. Schlimmer noch: Er verschickt dabei ihren Will, alle sehen „App ausgefallen“.
- Das habe ich mit Mosquitto 2.1.2 nachgemessen, es ist also nicht nur Theorie.
- Zwei Kleinigkeiten kommen dazu:
  - Der genannte Höchstwert 268435455 ist eigentlich die Grenze für das ganze Paket. Eine Nutzlast kann nie so groß werden.
  - Offen ist auch, was mit einer zu großen Nachricht passiert: Mosquitto kann sie still verwerfen oder die App trennen.

**Mein Vorschlag (so entschieden):**
- (a) `MaxPacketSize` bleibt eine Grenze für die Nutzlast. Neu wird ausdrücklich gesagt: Der Broker muss jede Nachricht bis `MaxPacketSize` Nutzlast annehmen. Begrenzt er das ganze Paket, muss seine Grenze um den Platz für Kopf und Topic größer sein (65544 Byte). Ein fertiger Satz für RIG 4.1.1 steht in `REVIEW.md`.
- (b) Ein Hinweis, dass Nutzlast + Topic-Länge + 4 nicht über 268435455 gehen dürfen.
- (c) Keine Regel für zu große Nachrichten: Die sind ohnehin verboten (MG 8.3.11), der Broker darf sie verwerfen oder die App trennen. D8 prüft später nur, dass erlaubte Größen durchkommen.

**Alternativen:**
- (d) `MaxPacketSize` zur Grenze für das ganze Paket machen. Dann ändern sich SF-091, REQ-PAG-005 und das SDK.
- (e) Nichts ändern. Dann können Plattformen weiter regeltreue Apps hinauswerfen.

**Was dann passiert:**
- Ich ändere den Text der Guideline nach deinem Auftrag und baseline neu.
- REQ-CNT-003 passt sich an und kommt als R-x zu dir zurück.
- Unser Dev-Broker und das SDK verhalten sich schon so, wie ich es vorschlage.

---

## Was aus dem Review davor (C-046, Nachtrag 2) geworden ist

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
