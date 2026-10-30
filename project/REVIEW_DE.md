# Review und Entscheidungen – Erklärung auf Deutsch

_Stand: 30.10.2026, C-071-Review Nachtrag 4: Du hast R-2 akzeptiert („R-2 akzeptiert“). REQ-MQTT-007 und alle 29 Broker-Szenarien stehen auf `reviewed`. **Für dich ist nichts offen.**_

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

Nichts. Alle Punkte aus dem C-071-Review (R-1, D-1, R-2) sind beantwortet und umgesetzt.

### Was aus deinem Review C-071 geworden ist (Kurzfassung)

- **R-1:** akzeptiert („R-1 akzeptiert“). Alle Broker-Szenarien von `oec-infra bus` stehen auf `reviewed`.
- **Korrektur der Anzahl:** In C-071 habe ich „28 Szenarien“ geschrieben (und im Chat „6 gut, 22 mit Fehler“). Richtig sind **29** (6 gut, 23 mit Fehler) – genau die, die in R-1 einzeln aufgezählt waren. Die Zahl ist überall korrigiert; inhaltlich hat sich nichts geändert.
- **D-1:** wie vorgeschlagen entschieden („D-1 wie vorgeschlagen“) und umgesetzt: der Satz steht in der Guideline (MG 6.2.2, `b955783`), SF-115 ist erledigt, die Baseline ist nachgezogen (1410 Fundstellen, 444 Regeln, keine Lücke), D8 schreibt „(MG 6.2.2)“ statt „(SF-115, proposed)“. Kein Finding mehr offen. Die geänderte Regel und die zwei Szenarien → R-2.
- **R-2:** akzeptiert („R-2 akzeptiert“). Die erweiterte Regel REQ-MQTT-007 (Broker-Zertifikat passt zur Adresse aus `broker.json` und ist gültig) und die Szenarien `T8-BUS-01/wrong-host` und `/expired` stehen auf `reviewed`; damit sind alle 29 Broker-Szenarien abgenommen. Kein Entwurf, kein offenes Finding.

### Zur Info: was C-071 gebaut hat

- **`oec-infra bus`** (T8-BUS-01 … 03 fertig, jetzt 7 von 17 D8-Tests): verbindet sich wie `oec-check monitor` (Broker-Adresse oder `--bus-storage`, CA, Client-Zertifikat oder Benutzer/Passwort; ohne Zertifikatsprüfung nur mit ausdrücklichem `--insecure`).
  - **T8-BUS-01:** probiert TLS 1.2 und 1.3 (eins muss gehen) und 1.0/1.1 (darf nicht gehen), sucht einen unverschlüsselten Port, prüft das Zertifikat gegen die CA, MQTT 3.1.1 und ob eine Nachricht der Größe `MaxPacketSize` aus `broker.json` durchkommt (SF-105).
  - **T8-BUS-02:** ein zweiter, nicht berechtigter Client (`--credentials-2` oder `--bus-storage-2`) schickt Set, Del und Call – aber nur an D8s eigene Probe-App, nie an eine echte Anwendung. Nichts davon darf ankommen. Vorher prüft D8, dass dieselben Anfragen von ihm selbst ankommen (sonst wäre „kommt nicht an“ nichts wert).
  - **T8-BUS-03:** D8 meldet zwei Probe-Komponenten an und fragt die OEC Registry nach ihrem Typenschild und Gesundheitszustand; nach Abmelden bzw. Absturz müssen sie verschwinden. Eine Registry gibt es erst mit P7, bis dahin ist das Ergebnis gegen unseren Dev-Broker „übersprungen“.
- **Test-Broker erweitert** (`mqtttest`, nur Ergänzungen; die 55 D7-Tests laufen unverändert), dazu eine nachgebaute Registry mit sieben Fehlern; 29 Szenarien → R-1.
- **Lauf gegen unseren Dev-Broker** (`oec/dev`, freiwillig, jetzt Teil von `live.sh`): T8-BUS-01 besteht auf beiden Ports (TLS 1.2 und 1.3, MaxPacketSize 262144), T8-BUS-02 **warnt** (siehe unten), T8-BUS-03 übersprungen (keine Registry).
- **Der Dev-Broker hat jetzt Zugriffsregeln** (`oec/dev/mosquitto/acl`): nur der Prüf-Benutzer `oec-probe` darf Set, Del und Call schicken; alle dürfen mitlesen und Pub, Get und Reply schicken. Mosquitto kann die App-Kennung im Topic nicht an einen Benutzer binden, und Apps lesen ihre eigenen Set/Config-Nachrichten – darum die Warnung REQ-RES-CONFIG-026. Das ist eine Grenze unserer Entwicklungsumgebung, kein Fehler der Guideline (A-258).
- **Zwei Fehler, die erst der Lauf gegen den Dev-Broker gezeigt hat, sind behoben** (zuerst ein Test, dann die Korrektur):
  - D8 hat bei den TLS-Proben sein Client-Zertifikat nicht mitgeschickt. Auf Port 8884 (verlangt ein Client-Zertifikat) sah TLS 1.2 deshalb „abgelehnt“ aus, und eine Ablehnung von TLS 1.1 hätte fälschlich als „geprüft“ gelten können. Neue Szenarien `mtls` und `mtls-tls11`.
  - In den Zugriffsregeln fehlte `Reply`: die Demo-Apps konnten ihre Antwort auf den Call nicht senden, D7 meldete T7-ACT-01/02/04 als durchgefallen. Danach: Demo-Lauf 39 bestanden, 16 übersprungen (brauchen die Zwischenstation), Interop-Test grün.
- Keine neue Bibliothek, nichts Geprüftes geändert, D7 und `COVERAGE.md` unverändert. `host` (Rechner prüfen) kommt wie geplant in C-072.

### Zur Info: was C-070 gebaut hat

- **Plan für P5b** (ROADMAP P5b): sechs Stücke in C-070 … C-075 – `image` (Images prüfen), `bus` (Broker prüfen), `host` (Rechner prüfen), Anbindung an Docker/Podman + `container` (laufende Container prüfen), `run` (ein Image probeweise starten), am Ende `list`/`explain`, eingefrorener Bericht und Abschlusslauf (A-247).
- **Abschlussbedingungen genauer gefasst:** D8 wird gegen eigens gebaute gute und fehlerhafte Images getestet, ohne Netz und ohne Docker im Prüfskript; Läufe mit Docker und gegen unseren Entwicklungs-Broker sind freiwillig. „Unsere Demo- und Registry-Images bestehen Level 3“ wandert in die Abschlüsse von P6/P7 (dann gibt es die Images), die CI nach P9.
- **Entwurf geprüft:** Gemeinsam mit D7 sind Katalog, Bewertung und Berichte. D8 bekommt ein **eigenes Berichts-Schema** (`infra-report.schema.json`), statt das eingefrorene von D7 zu ändern (A-244). Statt der geplanten Bibliothek `go-containerregistry` liest D8 Images mit Bordmitteln von Go; aus einer Registry lädt D8 nichts selbst (A-239, A-240).
- **`oec-infra image`:** liest ein Image als OCI-Ordner oder OCI-Archiv (z. B. aus `docker save`), ohne Docker und ohne Netz, und prüft: Format und Prüfsummen (T8-IMG-01), Architekturen (T8-IMG-02), SBOM und Lizenztexte (T8-IMG-03), SHA-256 (T8-IMG-04). Bericht als Text, JSON oder JUnit. Die Berichte von D7 sind Byte für Byte gleich geblieben.
- **Probe mit Docker** (freiwillig): `docker save` des Mosquitto-Images auf Docker 28 wird gelesen; es fällt bei der Lizenzprüfung durch, wie erwartet (kein OEC-Image). Builds für Windows, Linux, macOS je amd64/arm64 klappen (je 9–10 MB).
- Keine neue Bibliothek, nichts Geprüftes geändert, D7 unverändert.

### Was aus deinem Review C-070 geworden ist (Kurzfassung)

- **R-2:** akzeptiert („R-2 akzeptiert“). REQ-CNT-013, -014, -026, -027 und die 18 Test-Images stehen auf `reviewed`; damit sind alle 19 Test-Images von `oec-infra image` abgenommen. Kein Entwurf, kein Finding offen.

- **R-1:** akzeptiert, die 13 Test-Images standen auf `reviewed`. Durch D-1 haben sich 12 davon geändert; sie sind jetzt Teil von R-2.
- **D-1 (SF-114):** Deine erste Antwort (alle Lizenzen lizenzkonform nennen, „CRA und RED ready“, Versionen nicht einschränken, Texte rechtskonform) hat meinen Vorschlag geändert; den überarbeiteten Text hast du bestätigt („D-1 überarbeitet ok, einarbeiten“). Umgesetzt:
  - **Guideline `a55f475`**, RIG 4.1.4: die SBOM nennt jede Komponente mit Name, Version und Lizenzen; SPDX oder CycloneDX in beliebiger Version; jede Lizenz eindeutig benannt; Text jeder Lizenz und Ausnahme liegt bei; empfohlen eine Package-URL/CPE je Komponente; Hinweis auf CRA, RED und die Verantwortung des Anbieters (RIG 5.3). **SF-114 erledigt** (`3a86b5e`), kein Finding offen.
  - **Re-Baseline:** 1409 Guideline-Stellen (3 neue Sätze), 444 Regeln, 0 Lücken. Schema bleibt `2.0.1-dev`, nur die Baseline ist neu.
  - **D8:** liest jetzt auch SPDX 3, prüft Name, Version, Lizenzangabe und Kennung jeder Komponente und die Texte von Ausnahmen; eine Version, die D8 nicht lesen kann, ergibt „kein sicheres Urteil“ statt „durchgefallen“ (A-249). Keine neue Bibliothek.

### Was aus dem Review C-069 geworden ist (Kurzfassung)

Deine Antwort „D-1 (a)“ ist eingetragen: P5 ist abgeschlossen, P5b läuft (Details weiter unten).

### Zur Info: was C-069 gebaut hat

- **`oec-check list`:** zeigt den Katalog – je Test Kennung, Art (mithören/aktiv), Level, Stand, was er braucht (Zwischenstation, Neustart, zweite App) und die Regeln. Filter z. B. „nur aktive Tests bis Level 1, die nichts Besonderes brauchen“: `list --mode test --level 1 --needs none`.
- **`oec-check explain <Test oder Regel>`:** erklärt einen Test (seine Regeln mit „muss/sollte“, Level und Fundstelle in der Guideline, seine Testfälle mit dem erwarteten Ergebnis) oder eine Regel (Text, Fundstellen, Zitate, welche Tests sie prüfen). Alles steckt im Programm selbst, es braucht kein Netz (A-233).
- **Schema des Berichts** (`report.schema.json`, `oec-check report-schema`): legt fest, wie der JSON-Bericht von `monitor` und `test` aussieht (Version 1, jetzt eingefroren). Das Prüfskript kontrolliert **jeden** Bericht, den es erzeugt, gegen das Schema (A-231). Dabei fiel ein kleiner Fehler in D7 auf: die erste Frage nach dem Profil hatte im Bericht keine Bezeichnung, jetzt `discovery` (A-232).
- **Abdeckungsbericht** `oec/conformance/catalog/COVERAGE.md` (wird erzeugt und vom Prüfskript verglichen): welche Regel von welchem Test geprüft wird, je Test die Testfälle nach Ergebnis; keine Regel ohne Prüfung (A-234).
- **Zählfehler korrigiert:** D7 hat **55** Tests, nicht 54. T7-ACT-04 kam mit der Re-Baseline nach deinem Review C-060 dazu; die Texte seitdem zählten falsch (A-235).
- **Abschlusslauf gegen `demo.rs`** (wie in C-068): **49 bestanden, 0 durchgefallen, 6 übersprungen, Exit 0**; Stand danach wie vorher; der Bericht passt zum Schema; die Demo fragte die zweite App und D7 nach ihrem Typenschild. Am SDK musste nichts geändert werden.
- Keine neuen oder geänderten Testfälle (also nichts zu prüfen), kein Finding, keine neue Bibliothek, keine Schema-Änderung (A-238).

Die Technik-Entscheidungen A-229 … A-238 sind nur zur Info (unten). Einen Blick wert: **A-236** (warum drei Tests mit Warnung/Hinweis statt „durchgefallen“ zählen), **A-231** (was das Berichts-Schema festlegt) und **A-235** (Zählfehler).

### Was aus dem Review C-068 geworden ist

Deine Antwort „R-6 und R-7 akzeptiert“ ist eingetragen:
- R-6 (C-068): der geänderte Testfall `none` (läuft mit der zweiten App und `--announce`, T7-ADR-01 besteht);
- R-7 (C-068): die 3 neuen Testfälle zu T7-ADR-01 (`own-app`, `service-type`, `silent`).

Damit sind alle 59 Testfälle gegen die simulierte App (`oec/conformance/testdata/sim/index.yaml`) und alle 29 Aufzeichnungen (`oec/conformance/testdata/captures/index.yaml`) geprüft.

### Zur Info: was C-068 gebaut hat

- **Simulierte zweite App (`--peer`):** D7 meldet eine zweite App an, `Utility` mit der Kennung `oi4.org/oec-check/Peer/<Zufall>`, über eine eigene Verbindung. Sie verhält sich wie eine einfache echte App: letzter Wille, Birth (Typenschild und Zustand), Zustandsmeldung alle 30 s, Abschiedsmeldung am Ende. Sie beantwortet Fragen nach Typenschild, Zustand und Profil (A-224).
- **`--announce`** (deine Antwort (a) auf Q-029): D7 meldet sich selbst genauso an, unter seiner Tester-Kennung (`--tester-il` muss dann die Form `Hersteller/Modell/Produktcode/Seriennummer` haben). Ohne den Schalter bleibt D7 wie bisher unsichtbar (A-223).
- **„Ich bin da“-Meldung (A-225):** Sobald D7 die zu prüfende App gefunden hat, schicken die angemeldeten Apps ihre Zustandsmeldung noch einmal. Über die Zwischenstation verbindet sich die App nämlich erst nach deren Birth und hätte sie sonst verpasst.
- **T7-ADR-01:** prüft jede Frage der App an die zweite App (siehe R-7). Ohne `--peer` ist der Test „nicht entscheidbar“ – ein Lauf ohne `--peer` endet deshalb mit Exit 4, außer mit `--allow-skip` (A-226). Damit sind **alle 54 Tests von D7 gebaut**.
- **Bericht:** neuer Abschnitt `announced` – je angemeldeter App die Rolle, die Kennung und wie viele Fragen sie bekam und beantwortete (A-228).
- **Gefunden und behoben** (Test zuerst): nur im Selbsttest – die simulierte App abonnierte die Typenschilder anderer Apps vor ihren eigenen Anfrage-Topics. D7 hält das erste bestätigte Abonnement für „bereit“ und fragte zu früh. Jetzt kommen die neuen Abonnements zuletzt (A-227). D7 und SDK waren in Ordnung.
- **Lauf gegen die Beispiel-App `demo.rs`** über die Zwischenstation, mit Neustart, `--peer` und `--announce` (Level 2): **49 bestanden, 0 durchgefallen, 6 übersprungen, Exit 0.**
  - Die Demo fragte die zweite App und D7 2,4 s nach dem Start nach ihrem Typenschild, beide Male auf dem richtigen Topic; beide antworteten. T7-ADR-01 bestanden.
  - Übersprungen wie in C-067 (unter anderem T7-SET-10, weil die Demo keine ReferenceDesignation hat).
  - Der Stand der App war danach wieder derselbe. Am SDK musste nichts geändert werden.
- Kein Finding, keine neue Bibliothek.

Die Technik-Entscheidungen A-222 … A-228 sind nur zur Info (unten). Einen Blick wert: **A-226** (wann T7-ADR-01 bewertet und warum Läufe ohne `--peer` „nicht entscheidbar“ sind) und **A-225** („Ich bin da“-Meldung).

### Zur Info: was C-067 gebaut hat

- **Neustart auf Kommando:** Der Tester gibt D7 ein Kommando, das die App geordnet beendet und wieder startet (`--restart-cmd`, oder getrennt `--stop-cmd` und `--start-cmd`; Wartezeit `--restart-wait`, Standard 2 min). D7 startet selbst keine Programme oder Container, weil es nicht wissen kann, wie die App betrieben wird (A-216). Ohne Kommando werden die beiden Neustart-Tests übersprungen, der Lauf ist dann „nicht entscheidbar“ (Exit 4).
- **Drei neue Tests:** T7-LIFE-04 (Abschiedsmeldung), T7-SET-10 (ReferenceDesignation übersteht den Neustart) und T7-MON-14 (PublicationList). Damit sind **alle 19 passiven Regeln** fertig.
- **Zuschnitt (A-215):** Inkrement 6 ist geteilt. Die simulierte zweite App, `--announce` (Q-029) und T7-ADR-01 kommen in C-068; `list`/`explain`, das Schema des Berichts und der Abschlusslauf von P5 in C-069.
- **Gefunden und behoben** (Test zuerst): nur im Selbsttest – die Anfrage nach der PublicationList in einer Aufzeichnung hatte keine Klassen-Kennung, die sie braucht. D7 und SDK waren in Ordnung.
- **Lauf gegen die Beispiel-App `demo.rs`** über die Zwischenstation, mit Neustart (Level 2): **48 bestanden, 0 durchgefallen, 6 übersprungen, Exit 0.**
  - Die Demo schickt beim Beenden ihre Abschiedsmeldung und meldet sich ab; nach dem Neustart kam sie mit Birth zurück (T7-LIFE-04 bestanden).
  - T7-MON-14 bestanden; T7-SET-10 übersprungen, weil die Demo keine ReferenceDesignation hat.
  - 146 Anfragen, 143 beantwortet (die 3 anderen sind wieder die absichtlich unsinnigen von T7-ACT-03). Der Stand der App war danach wieder derselbe. Am SDK musste nichts geändert werden.
- Kein Finding, keine neue Bibliothek.

Die Technik-Entscheidungen A-215 … A-221 sind nur zur Info (unten). Einen Blick wert: **A-215** (Zuschnitt), **A-216** (Neustart-Kommando), **A-218** (was als Abschiedsmeldung zählt), **A-220** (was T7-MON-14 prüft und was nicht).

### Zur Info: was C-066 gebaut hat

- **D7 als Zwischenstation (Proxy):** Mit `--proxy-listen` verbindet sich die App nicht direkt mit dem Broker, sondern mit D7. D7 reicht alles unverändert weiter und sieht dabei die Anmeldung, den letzten Willen, die TLS-Version und Benutzer/Passwort (das Passwort steht nie im Bericht). Gebaut als kleines eigenes Programmstück ohne neue Bibliothek, weil die MQTT-Bibliothek nur die Client-Seite kann (A-203).
  - Für die Zertifikate gibt der Tester die Zertifizierungsstelle an, der die App vertraut (A-204). Eine App, die sich **nur** mit einem Client-Zertifikat anmeldet, kann über die Zwischenstation noch nicht getestet werden.
  - Ohne Zwischenstation werden die Verbindungs-Tests übersprungen, der Lauf ist dann „nicht entscheidbar“ (Exit 4).
- **Sieben neue Tests:** T7-CON-01 … 04 (MQTT 3.1.1 und fester Client-Name, TLS ab 1.2 mit Zertifikatsprüfung, Anmeldung, keine gespeicherten Nachrichten) und T7-LIFE-01 … 03 (letzter Wille bei jeder Verbindung neu, Birth zuerst nach jedem Verbinden und nach einer Erholung, der Wille wird bei einem Abbruch wirklich verschickt).
- **T7-MON-14 ist nochmal verschoben, nach C-067** (A-202): Die Zwischenstation hat den Zyklus gefüllt.
- **Gefunden und behoben** (Test zuerst):
  - Zwei falsche Befunde der passiven Prüfung: T7-MON-03 meldete eine Birth, die nach dem Zustand kommt, als fehlend; T7-MON-10 zählte die Birth nach einer Erholung als „zweite Birth“ (A-211). Die geprüften Aufzeichnungen bleiben dabei gleich.
  - Der erste Lauf gegen die Demo fand einen Fehler in D7: Die Demo abonniert ihre Topics kurz nach der Birth, und D7 hatte seine erste Frage schon vorher geschickt – sie ging verloren. Jetzt wartet D7 auf das Abonnement (A-205).
- **Lauf gegen die Beispiel-App `demo.rs`** über die Zwischenstation (Level 2): **46 bestanden, 0 durchgefallen, 5 übersprungen, Exit 0.**
  - Die Demo hat alle drei schlechten TLS-Angebote abgelehnt, sich nach der Trennung nach etwa 6 s neu verbunden und zuerst ihre Birth geschickt.
  - 145 Anfragen, 142 beantwortet (die 3 anderen sind wieder die absichtlich unsinnigen von T7-ACT-03).
  - Der Stand der App war danach wieder derselbe. Am SDK musste nichts geändert werden.
- Kein Finding, keine neue Bibliothek.

Die Technik-Entscheidungen A-202 … A-214 sind nur zur Info (unten). Einen Blick wert: **A-202** (Zuschnitt), **A-204** (Zertifikate), **A-205** (Warten auf das Abonnement), **A-207/A-208** (wie Client-Name und „neu erzeugter“ Wille bewertet werden).

### Zur Info: was C-065 gebaut hat

- **Die Aktions-Tests sind fertig** (T7-ACT-01 … 04). D7 fragt die App nach einer neuen Datensatz-Nummer und prüft:
  - Die Antwort kommt genau einmal endgültig an die angegebene Adresse.
  - Die Nummer ist neu und wird von der App nicht schon benutzt.
  - Solange sie reserviert ist, wird sie nicht noch einmal vergeben.
  - Auf unsinnige Anfragen antwortet die App gar nicht.
  - Für ein unbekanntes Gerät antwortet sie „nicht gefunden“.
  
  D7 benutzt die Nummer nie. Deshalb ändert sich an der App nichts, und der Vergleich vorher/nachher bleibt gleich.
- **Fünf weitere passive Regeln** (jetzt 18 von 19):
  - eine Klassen-Kennung gehört zu genau einem Datensatz-Aufbau;
  - Apps mit Geräten bieten Daten und Ereignisse an;
  - nach einer Änderung wird die Liste neu geschickt;
  - der Änderungszähler im Typenschild steigt;
  - wer die Konfiguration ändert, schickt alle Pflichtfelder mit.
  
  **T7-MON-14** (ob die App so oft und so genau schickt, wie in der PublicationList steht) braucht ein eigenes Zeitmodell und kommt in C-066 (A-191).
- **Gefunden und behoben** (Test zuerst): Im Bericht fehlten die Aktions-Anfragen von D7 in der Liste der Anfragen. Dazu kleinere Dinge an der simulierten App und den Testfällen (A-201).
- **Lauf gegen die Beispiel-App `demo.rs`** (Level 2): **38 bestanden, 0 durchgefallen, 6 übersprungen.**
  - 145 Anfragen, 142 beantwortet. Die 3 unbeantworteten sind die absichtlich unsinnigen Anfragen von T7-ACT-03, auf die die App richtig schweigt.
  - Der Stand der App war danach wieder derselbe. Am SDK musste nichts geändert werden.
  - T7-MON-17 wird übersprungen, weil die Demo selbst keine Konfiguration anderer Apps ändert.
- Kein Finding, keine neue Bibliothek, kein geprüfter Testfall verändert.

Die Technik-Entscheidungen A-191 … A-201 sind nur zur Info (unten). Einen Blick wert:
- **A-191:** T7-MON-14 kommt erst in C-066.
- **A-195:** was T7-ACT-02/03 nicht sehen können.
- **A-199/A-200:** was der Änderungszähler und die Pflichtfelder in einem `test`-Lauf zeigen können.

### Zur Info: was C-064 gebaut hat

- **Die Schreib-Tests sind fertig** (T7-SET-01 … 09, 11, 12): D7 schreibt Konfiguration, Daten, Metadaten, PublicationList, ReferenceDesignation und SubscriptionList. Es prüft, dass die App jede Änderung mit einer Status-Meldung beantwortet und nach einer erfolgreichen Änderung den neuen Stand schickt. Verbotene Änderungen (unbekannte Einträge, nur lesbare Felder, fest eingestellte Einträge) müssen abgelehnt werden. Dazu kommen das Löschen und die umschaltbare Log-Stufe. T7-SET-10 (nach einem Neustart noch da) kommt mit Inkrement 6.
- **D7 räumt hinterher auf:** Es schreibt nur zurück, was die App selbst geschickt hat, löscht nur, was es selbst angelegt hat, und macht jede Änderung rückgängig. Vor dem ersten und nach dem letzten Schreib-Test liest D7 den Stand der App und vergleicht. Das Ergebnis steht im Bericht (`restore`); weicht etwas ab, gibt es eine Warnung. Es ist keine Bewertung der App.
- **Die simulierte App ist jetzt beschreibbar**, mit 13 neuen eingebauten Fehlern (von dir als R-1 akzeptiert).
- **Gefunden und behoben** (Test zuerst): Fehler in D7 und in der simulierten App. Nach einer Code-Durchsicht kamen drei weitere dazu:
  - Auf Level 1 schreibt D7 eine Konfiguration mit verdecktem Passwort (`********`) nicht mehr zurück. Dort ist nur empfohlen, dass die App das verdeckte Passwort richtig behandelt. Sonst könnte das echte Passwort überschrieben werden, ohne dass man es sieht.
  - D7 löscht eine ReferenceDesignation nur, wenn die App vorher das Zurückschreiben angenommen hat.
  - Bricht ein Lauf mitten in den Schreib-Tests ab, warnt D7, dass die App verändert sein kann.
- **Lauf gegen die Beispiel-App `demo.rs`** (Level 2): **30 bestanden, 0 durchgefallen, 5 übersprungen**, alle 136 Fragen beantwortet. Der Stand der App war danach wieder derselbe, geprüft in drei Läufen. Am SDK musste nichts geändert werden. Die App legt beim Schreiben drei Dateien mit unveränderten Werten an. Das ist kein Verstoß.
- Kein Finding, keine neue Bibliothek.

Die Technik-Entscheidungen A-176 … A-190 sind nur zur Info (unten). Einen Blick wert:
- **A-177/A-178:** was „wiederhergestellt“ heißt; nur gemeldet, nicht bewertet.
- **A-180, A-188, A-189:** was D7 löscht oder überschreibt und was es bewusst nicht anfasst.

### Zur Info: was C-063 gebaut hat

- **Alle Lese-Tests fertig:**
  - T7-GET-03: jede Ressource mit und ohne Kennung und mit Filter; die Einträge der PublicationList; zu jedem Datensatz die Metadaten.
  - T7-GET-04: jedes Gerät erfüllt das Level.
  - T7-GET-06: seitenweise Antworten.
  - T7-GET-07: die Query.
  - T7-GET-08: die Sprache.
  - T7-ADR-02: keine Antworten in der alten Version.
  - T7-ADR-03: exakte Geräte-Kennungen.
- **Zwei Fehlalarme der passiven Regeln behoben** (Test zuerst):
  - Eine Antwort auf Deutsch wurde mit der englischen Anmeldung verglichen.
  - Die Antwort auf die Frage nach einer einzelnen Seite galt als „unvollständig“.
  - Der zweite Fehlalarm betraf einen Aufzeichnungs-Testfall → R-2.
- **Lauf gegen die Beispiel-App `demo.rs`** (Level 2):
  - Erst 20 bestanden, 1 durchgefallen. Der Fehler lag bei D7, nicht in der App: „Query ignorieren“ heißt „gleiche Antwort wie ohne Query“. D7 hatte aber einen bestimmten Inhalt erwartet, und die Demo hat gar keine ReferenceDesignation-Einträge.
  - Behoben, dazu der neue Testfall `T7-GET-07/empty`.
  - Danach **21 bestanden, 0 durchgefallen, 3 übersprungen**, alle 82 Fragen beantwortet. Am SDK musste nichts geändert werden.
- Kein Finding, keine neue Bibliothek.

Die Technik-Entscheidungen A-164 … A-175 sind nur zur Info (unten). Einen Blick wert:
- **A-166:** Aufteilen nach Größe kann D7 nicht auslösen; das prüft die passive Regel T7-MON-18.
- **A-167:** T7-ADR-02 kann eine App nicht zwingen, Version 1 zu sprechen. Ohne Verkehr in Version 1 wird der Test übersprungen.
- **A-168:** „Ignorieren“ heißt „antwortet wie ohne“.

### Zur Info: was C-062 gebaut hat

C-062 ist der **Start von P5**.
- **Plan:** In der ROADMAP steht jetzt der Plan für P5 in 8 Inkrementen (C-062 … C-068): erst die restlichen Lese-Tests, dann die Schreib-Tests (`Set`/`Del`, mit Wiederherstellen des alten Zustands), dann die Aktion und die noch fehlenden passiven Regeln, dann der Proxy-Modus, Neustart/Peer, zuletzt `list`/`explain`, das Report-Schema und der Abschlusslauf. Der Zuschnitt ist meine Entscheidung (A-163).
- **`oec-check test`:** D7 sucht die angegebene App, fragt sie als „Tester“ (eine eigene, nicht angemeldete Kennung) und ordnet die Antworten über `ReplyTo` zu. Gleichzeitig laufen die passiven Monitor-Regeln mit; ein aufgezeichneter Lauf lässt sich mit `monitor --replay` wieder abspielen.
- **Vier Tests fertig:** T7-GET-01 (MAM und Health für alle Geräte), T7-GET-02 (das Profil nennt genau, was die App beantwortet), T7-GET-05 (leere Liste; unbekanntes Gerät → Status-Ereignis), T7-GET-09 (unbekannte Felder werden ignoriert).
- **Gefunden und behoben:** D7 selbst hat seine Fragen mit QoS 1 geschickt; die Guideline sieht QoS 0 als Standard vor (REQ-MQTT-017). Der neue Selbsttest hat das gefunden (A-154).
- **Echter Lauf gegen die Beispiel-App `demo.rs`** (Level 2, auf einer Kopie eines Dev-Ordners, Dev-Broker): **14 bestanden, 0 durchgefallen, 3 übersprungen**, alle 18 Fragen beantwortet.
- Kein Finding, nichts Reviewtes geändert, keine neue Bibliothek.

Die Technik-Entscheidungen A-153 … A-163 sind nur zur Info (unten). Einen Blick wert: **A-159** (ein `test`-Lauf allein reicht nicht für eine Level-Aussage, es braucht zusätzlich einen längeren `monitor`-Lauf), **A-155** (der Tester meldet sich nicht an) und **A-154** (QoS 0).

### Zur Info: was C-061 gebaut hat

C-061 war **P4 Inkrement 10**: die **C-Schnittstelle `oec-ffi`**. Ich habe es nicht geteilt (**A-145**).
- **Was das ist:** Eine Bibliothek, mit der man das Rust-SDK aus C-Programmen benutzen kann – als „Shared Library“ (wird beim Start geladen, z. B. `liboec_ffi.dylib`) und als „Static Library“ (wird fest ins Programm eingebaut, `liboec_ffi.a`). Dazu gehört die Header-Datei `oec.h`, die einem C-Programm sagt, welche Funktionen es gibt. Die Logger-Demo (P6) wird in C darauf geschrieben.
- **Umfang:** 46 C-Funktionen über das ganze SDK: App starten und stoppen, Geräte, Datensätze, Config, Ereignisse, Abos, `Get`/`Set`/`Del` an andere Apps und die Aktion `NewDataSetWriterId`.
- **Der Header wird erzeugt, nicht von Hand geschrieben:** Das Werkzeug cbindgen (Version 0.29.4, fest vorgegeben) liest den Rust-Code und schreibt `oec.h`. Das Prüfskript erzeugt den Header jedes Mal neu und schlägt Alarm, wenn er vom eingecheckten abweicht – so können Code und Header nicht auseinanderlaufen.
- **Der Vertrag aus P3 bleibt fast unverändert.** Die wenigen Änderungen stehen in ADR-0021 (Nachtrag C-061) und in A-147 … A-151; die wichtigsten: Bei Fragen an andere Apps gibt man jetzt den Service-Typ mit an (das Rust-SDK braucht ihn seit C-058), Ereignisse bekommen eine Stufe, und die Genauigkeit (Precision) eines Wertes steht in der Feldbeschreibung statt in einer eigenen Funktion.
- **Sicherheit für C:** Ein Absturz im Rust-Teil (Panic) wird an der Grenze abgefangen und als Fehlercode gemeldet. Falsche Eingaben aus C (NULL, kaputter Text, ungültige Zahlen) geben einen Fehler statt eines Absturzes. Funktionen, die warten würden, sind aus einem Callback heraus gesperrt, damit sich das Programm nicht selbst blockiert.
- **Geprüft:** Die beiden C-Beispiele und ein großes C-Testprogramm laufen im Prüfskript gegen den eingebauten Test-Broker – gegen beide Bibliotheksformen, und mit **AddressSanitizer** und **UBSan** (Werkzeuge des C-Compilers, die Speicherfehler und undefiniertes Verhalten sofort melden). Ein Lasttest mit 6 Threads × 150 Runden ist dabei. Einmal zusätzlich unter Linux mit **LeakSanitizer** (findet vergessenen Speicher) und Testbauten für Linux amd64, Windows und ARMv7. Interop 14 grün; die C-Beispiele einmal gegen den echten Mosquitto.
- Keine neue Bibliothek, nichts Reviewtes geändert, keine neuen Testdaten, kein Finding.

Die Technik-Entscheidungen A-145 … A-152 sind nur zur Info (unten). Einen Blick wert: **A-148** (Änderungen am C-Vertrag), **A-150** (Regeln für Callbacks) und **A-151** (was es in C noch nicht gibt).

### Zur Info: was C-060 gebaut hat

Dein Review zu C-060 ist eingetragen (D-1, D-2 und R-1 aus C-059, siehe unten „Was aus dem Review C-060 geworden ist“); das daraus entstandene R-1 (REQ-ACT-017, -023, -024) hast du akzeptiert.

Zur Erinnerung – C-060 war **P4 Inkrement 8c**. Ich habe es nicht weiter geteilt (**A-137**). Reihenfolge jetzt: 8c → 10 (C-Schnittstelle `oec-ffi`).
- **Die Aktion `NewDataSetWriterId` (MG 9.2):** Eine App kann eine andere App fragen: „Gib mir eine neue, freie Datensatz-Nummer (DataSetWriterId) für eine Ressource.“ Gebraucht wird das, wenn ein IT-Programm in einem OT-Connector etwas Neues anlegen will (z. B. eine ReferenceDesignation für ein Gerät).
  - **Antworten (jede App ab Level 2):** Das SDK beantwortet jede solche Frage sofort mit genau einer Antwort. Die neue Nummer ist 60 Sekunden für den Fragenden reserviert (`Ttl`) und wird danach nie wieder vergeben – auch nicht nach einem Neustart, denn die Reservierung steht in der gespeicherten Nummern-Tabelle. Mehr als 256 offene Reservierungen gleichzeitig lehnt das SDK ab (`BadResourceUnavailable`). Im Profil der App steht jetzt `NewDataSetWriterId`.
  - **Fragen (für App-Entwickler):** `Consumer::new_data_set_writer_id(Ressource, Ziel-App, Optionen)` liefert die Nummer und ihre Gültigkeitsdauer. Die Form weicht etwas von der Skizze ab (gleiches `Target` wie bei `get`, Optionen mit Zeitlimit, Standard 10 s) – festgehalten in ADR-0020.
- **Die API-Skizze ist gelöscht:** Ihre Beispiele (`hello`, `config`, `device_source`, `consumer`) liegen jetzt im SDK unter `oec/sdk/rust/oec/examples/` neben der Beispiel-App `demo.rs`. Der C-Vertrag `oec.h` und die C-Beispiele liegen jetzt in `oec/sdk/rust/oec-ffi/`, wo Inkrement 10 sie umsetzt. Die Prüfung „jede im Code genannte REQ gibt es und sie ist aktiv“ läuft jetzt im SDK (`oec/tests/citations.rs`). Die Prüfskripte sind angepasst.
- **Geprüft:** Tests ohne Netzwerk (gegen die Guideline-Beispiele aus B1.2.1 und die Fehler-Vektoren), Tests mit zwei Apps über den eingebauten Test-Broker, 25 Läufe hintereinander unter Last grün, Interop gegen den echten Mosquitto (14 grün, 1 bewusst ausgelassen) und einmal D7 gegen die Beispiel-App: 13 bestanden, 0 durchgefallen – **der D7-Test für Aktionen (T7-MON-07) läuft jetzt** und ist grün; alle 101 Nachrichten gültig nach Schema.
- **Fehler gefunden und behoben:** Der erste Interop-Lauf scheiterte, weil die Beispiel-App beim Ausprobieren ihre Daten in den erzeugten Test-Ordner geschrieben hatte. Der Test räumt sich den Ordner jetzt selbst leer (`Dev::root`). Außerdem nannte ein Testkommentar zwei falsche REQ-Nummern – korrigiert.
- Keine neue Bibliothek, nichts Reviewtes geändert, keine neuen Testdaten.

Die Technik-Entscheidungen A-137 … A-144 sind nur zur Info (unten). Einen Blick wert: **A-139** (Reservierung), **A-142** (Form der Funktion) und **A-143** (Abschied von der Skizze).

### Zu deiner Frage: Ja, der Zweck der SubscriptionList ist verständlich

Du hast geschrieben: Wenn Service A mit dem OT-Service B spricht, soll man über die SubscriptionList von A steuern können, welche Werte A verfolgt, z. B. nur 1 von 100 Geräten bei B. So habe ich es verstanden und im Log festgehalten.

Was das SDK dafür heute kann und was nicht:
- **Geht:** Ein einstellbares Abo von A (`Subscription::configurable()`) kann von außen **abgeschaltet** (`Del`) oder **langsamer** gestellt werden (`Set` mit Intervall). Abonniert A die Geräte einzeln, kann man so 99 von 100 abschalten.
- **Geht nicht:** Abonniert A alle 100 Geräte mit **einem** Platzhalter-Abo (`…/Pub/Data/#`), kann man es nicht auf ein Gerät **verengen**. Dafür bräuchte man ein `Set` mit einem neuen, engeren Topic, und das lehnt das SDK nach D-3 ab (`BadNotSupported`).
- **Idee für später (meine Entscheidung, keine Guideline-Frage):** Das SDK könnte ein `Set` annehmen, dessen Topic in einem schon vorhandenen einstellbaren Abo der App enthalten ist („verengen“), oder der App einen Handler anbieten, mit dem sie neue Topics selbst annimmt. Das passt zu D-3 (Anlegen ist freiwillig). Ich habe es als Vorschlag für einen späteren Zyklus notiert, gebaut wird es jetzt nicht. Sag Bescheid, wenn du es früh haben willst.

### Zur Info: was C-059 gebaut hat

C-059 war **P4 Inkrement 8b**: die SubscriptionList (jede App ab Level 2 zeigt, was sie abonniert hat; von außen einstellbar) und die Beispiel-App `demo.rs`. Deine Entscheidungen zu D-1 … D-3 (SF-109 … SF-111) sind umgesetzt. Details in den Punkten A-128 … A-136 unten.

---

Die Punkte A-1 … A-238 in `REVIEW.md` sind nur zur Info. Das sind Technik-Entscheidungen, die ich nach deiner Regel (GUARDRAILS G-3.4 b) selbst treffe. Du kannst sie kippen, musst aber nicht antworten. Die neuen aus C-061 (A-145 … A-152) stehen unten.

Aus C-056: Deine Zustimmung zu D-1 (SF-107), D-2 (SF-108) und R-1 ist umgesetzt, siehe unten „Was aus dem Review C-056 geworden ist“.

Aus C-052: Deine Zustimmung zu D-1 (SF-106), R-1 (Testdaten) und R-2 (REQ-TOP-017) ist umgesetzt, siehe unten „Was aus dem Review davor (C-052) geworden ist“.

**Nachtrag 1 (deine Anweisung):** In den Guardrails steht jetzt die neue Regel **G-3.10**. Am Ende jedes Zyklus entscheide ich, ob der nächste Zyklus mit einem neuen Agenten oder mit demselben weiterläuft, und schreibe den Grund ins Log.
- Einen **neuen Agenten** nehme ich in der Regel, wenn mein Kontext schon zusammengefasst wurde oder groß ist, oder wenn ein neues Inkrement, eine neue Phase oder ein neues Deliverable beginnt.
- **Derselbe Agent** macht nur kurze Nacharbeiten zum selben Thema weiter, z. B. die Antworten auf dein Review.

Bei einem neuen Agenten schreibe ich den Start-Prompt gleich in `prompt.md`. Diese Datei committe ich nie. Für C-048 habe ich mich für einen **neuen Agenten** entschieden, weil mein Kontext mehrfach zusammengefasst wurde und C-048 in einem neuen Crate arbeitet. `prompt.md` ist schon gefüllt.

Logs: [C-067](log/2026-10-29_C-067_d7-restart-publist.md), [C-066](log/2026-10-28_C-066_d7-proxy-lifecycle.md), [C-065](log/2026-10-28_C-065_d7-action-monitor.md), [C-064](log/2026-10-28_C-064_d7-set-tests.md), [C-063](log/2026-10-28_C-063_d7-get-tests.md), [C-062](log/2026-10-28_C-062_d7-test-engine.md), [C-061](log/2026-10-28_C-061_oec-ffi.md), [C-060](log/2026-10-28_C-060_new-data-set-writer-id.md), [C-059](log/2026-10-28_C-059_subscription-list.md), [C-058](log/2026-10-27_C-058_consumer-core.md), [C-057](log/2026-10-27_C-057_refdes-events.md), [C-056](log/2026-10-27_C-056_config.md), [C-055](log/2026-10-27_C-055_write-side.md), [C-054](log/2026-10-27_C-054_data-sets-metadata.md), [C-053](log/2026-10-27_C-053_provider-registry-get.md), [C-052](log/2026-10-26_C-052_d7-monitor.md), [C-051](log/2026-10-26_C-051_dev-environment-interop.md), [C-050](log/2026-10-26_C-050_runtime-tokio-rustls.md), [C-049](log/2026-10-26_C-049_mqtt-session-lifecycle.md), [C-048](log/2026-10-26_C-048_oec-crate-bootstrap.md), [C-047](log/2026-10-26_C-047_message-model.md), [C-046 inkl. Nachtrag 1 bis 3](log/2026-10-26_C-046_p4-start.md).

### Begriffe aus C-061

- **C-ABI / FFI:** die „Steckdose“, über die ein C-Programm Funktionen einer Rust-Bibliothek aufrufen kann (ABI = Binärschnittstelle, FFI = Aufruf über Sprachgrenzen).
- **Header (`oec.h`):** die Datei, die einem C-Programm sagt, welche Funktionen, Typen und Konstanten es gibt.
- **cbindgen:** ein Werkzeug, das den Header aus dem Rust-Code erzeugt.
- **Shared / Static Library:** dieselbe Bibliothek einmal zum Nachladen beim Programmstart und einmal zum festen Einbauen ins Programm.
- **Handle:** ein Zeiger, den das C-Programm vom SDK bekommt (z. B. für eine App oder einen Datensatz) und bei jedem Aufruf wieder mitgibt; was es bedeutet, weiß nur das SDK.
- **Callback:** eine C-Funktion, die das SDK aufruft, wenn etwas passiert (z. B. eine Nachricht kommt).
- **Panic:** ein Absturz im Rust-Code; er darf nicht nach C „durchschlagen“ und wird deshalb an der Grenze abgefangen.
- **AddressSanitizer / UBSan / LeakSanitizer:** Prüfwerkzeuge des C-Compilers. Sie melden sofort Speicherfehler (z. B. doppelt freigegeben), undefiniertes Verhalten (z. B. Überlauf) bzw. vergessenen Speicher.
- **MSVC:** der C-Compiler von Microsoft für Windows; die andere Windows-Variante ist GNU (mingw).

### Begriffe aus C-058

- **Consumer (Empfänger):** eine App, die bei anderen Apps fragt oder ihnen zuhört. Das Gegenstück ist der **Provider**, der antwortet. Eine App kann beides sein.
- **Korrelation:** die Zuordnung einer Antwort zu ihrer Frage. Die Antwort trägt in `ReplyTo` die Nachrichten-Nummer (`MessageId`) der Frage.
- **SUBACK:** die Bestätigung des Brokers, dass ein Abo gilt. Erst danach schickt das SDK die Frage ab, sonst könnte die Antwort verloren gehen (der Bus hebt nichts auf).
- **Paginierung:** Eine lange Antwort kommt in mehreren Seiten; das SDK setzt sie in der richtigen Reihenfolge wieder zusammen.
- **Präsenz (Presence):** ob eine App gerade auf dem Bus ist. Anmeldung (Typenschild), Abmeldung (Health „normal“ mit 0) und Absturz (Health „Fehler“ mit 0 über den letzten Willen) erkennt das SDK.
- **Letzter Wille (Last Will):** eine Nachricht, die der Broker im Namen einer App verschickt, wenn deren Verbindung unerwartet abreißt.
- **blockierend:** eine Funktion, die wartet, bis die Antwort da ist, für Programme ohne `async`.

### Begriffe aus C-057

- **ReferenceDesignation (Betriebsmittelkennzeichnung):** wo ein Gerät in der Anlage sitzt und wozu es dient, nach IEC 81346 – aufgeteilt in Funktion, Produkt und Einbauort, jeweils mit einem Verweis auf das übergeordnete Gerät (Parent).
- **Query (Abfrage der Kinder):** ein `Get ReferenceDesignation`, das nicht ein Gerät meint, sondern alle Geräte, deren übergeordnetes Gerät das genannte ist.
- **Syslog:** das klassische Format für Log-Meldungen mit acht Stufen von „Emergency“ bis „Debug“. Die Guideline überträgt Syslog-Meldungen als Events (`Pub/Event/<Quelle>/Syslog/<Stufe>`).
- **Generic Event (eigenes Ereignis):** ein Ereignis, dessen Bedeutung die App selbst festlegt (Kategorie `CAT_GENERIC_99`), z. B. „Filter tauschen“. Die App muss die Nummern und Details dokumentieren.
- **Brücke (Bridge):** das Stück SDK, das die normale Rust-Log-Schnittstelle (`log`) mit dem Bus verbindet, damit die App nichts Besonderes tun muss.
- **Drosselung (Rate Limit):** eine Obergrenze, wie viele Meldungen pro Sekunde auf den Bus gehen, damit eine „redselige“ App den Bus nicht flutet.
- **Rückkopplung:** Ein Log-Eintrag würde beim Senden einen neuen Log-Eintrag erzeugen, der wieder gesendet wird – endlos. Das SDK verhindert das.

### Begriffe aus C-056

- **Config (Konfiguration):** die Einstellungen einer App, in Gruppen geordnet. Jede Einstellung hat einen Typ (Zahl, Text, Ja/Nein, Datum, Binärdaten), einen Standardwert und optional Grenzen, ein Textmuster oder eine Auswahlliste.
- **Kontext (Context):** der Name einer Config einer Quelle, z. B. `service`. Er steht hinten im Topic (`Pub/Config/<Quelle>/service`). Eine Quelle kann mehrere Configs haben.
- **sensibel (Sensitive):** eine geheime Einstellung, z. B. ein Passwort. Auf dem Bus steht statt des Werts immer `********` (Maskierung).
- **Teil-Update:** ein `Set Config`, das nur die geänderten Einstellungen schickt, nicht die ganze Config.
- **Regex / Textmuster (Pattern):** eine Regel, wie ein Text aussehen muss, z. B. „vier Zahlen mit Punkten“ für eine IP-Adresse. Es gibt mehrere Regex-Dialekte, die sich in Kleinigkeiten unterscheiden; die Guideline sagt das selbst.
- **Secrets:** Zugangsdaten und Schlüssel, die die Plattform der App unter `/run/secrets/oi4` bereitlegt (GL-ADR-022). Nicht zu verwechseln mit sensiblen Config-Werten, die die App selbst verwaltet.

### Begriffe aus C-055

- **`Set` / `Del`:** Anfragen, die etwas ändern (setzen) bzw. löschen wollen. `Get` fragt nur ab.
- **Status-Event:** die Antwort auf eine `Set`/`Del`-Anfrage: „Good“, „Uncertain“ oder „Bad“, mit einer Nummer und einem Namen für den Grund (z. B. `BadNotFound` = gibt es nicht, `BadNotWritable` = darf man nicht ändern, `BadTypeMismatch` = falscher Typ).
- **`ReplyTo`:** die Nummer der Anfrage, die in der Antwort steht. So weiß der Anfragende, welche Antwort zu ihm gehört.
- **`Set PublicationList`:** von außen die Sende-Einstellungen eines Datensatzes ändern (Modus, Intervall, Totband), soweit die App es erlaubt.
- **`Set Data` / `on_set`:** von außen die Werte eines Datensatzes setzen. Die App entscheidet in ihrem Handler `on_set`, ob sie die Werte annimmt.
- **SubscriptionList:** die Liste der Abos einer App (was sie empfängt). Kommt mit Inkrement 8.

### Begriffe aus C-054

- **Datensatz (Data DataSet):** eine Gruppe von Messwerten, die immer zusammen verschickt wird, z. B. Temperatur und Sollwert. Er hat einen Namen (Tag), der auch im Topic steht.
- **`Oi4Pv`:** der genormte Datensatz für einen Prozesswert: das Feld `Pv` (Istwert) und optional `Sv1` … `Sv255` (z. B. Sollwerte).
- **Metadaten (Metadata):** die Beschreibung eines Datensatzes: Feldnamen, Typen, Einheit, Messbereich. Ein Empfänger braucht sie, um die Werte zu verstehen.
- **ConfigurationVersion:** die Versionsnummer der Metadaten. Jede Datennachricht nennt sie, damit ein Empfänger merkt, wenn sich die Beschreibung geändert hat.
- **UUIDv5:** eine Kennung, die aus einem Text berechnet wird. Gleicher Text ergibt immer dieselbe Kennung, ein anderer Text eine andere. Wir berechnen sie aus Tag und Feldern.
- **Intervall:** „sende alle n Millisekunden den aktuellen Stand“. 0 heißt „sende bei Änderung“.
- **Totband (Precision):** „sende erst, wenn sich der Wert um mehr als x geändert hat“. Das spart Nachrichten bei rauschenden Messwerten. Mit Intervall gilt das Totband nicht.
- **Sende-Modus `ON_REQUEST_FILTER_9`:** der Standard für Datensätze: jeder Datensatz einzeln auf seinem eigenen Topic und zusätzlich auf Anfrage.

### Begriffe aus C-053

- **Asset (vertretenes Gerät):** ein Gerät ohne eigenen MQTT-Zugang, für das eine App spricht, z. B. Sensoren hinter einem Gateway. Es hat eine eigene Identification Link (IL) und erscheint in den Topics als `Source`.
- **Source:** das Ende eines Topics, das sagt, *über wen* eine Nachricht ist: die App selbst oder eines ihrer Assets.
- **Profile:** die Liste der Ressourcen, die eine App bzw. ein Asset anbietet (z. B. MAM, Health, Profile, PublicationList).
- **PublicationList:** das „Sendeprogramm“ einer App: welche Nachricht über wen mit welcher Nummer (DataSetWriterId) und wann (`Mode`) verschickt wird.
- **DataSetWriterId:** eine Nummer pro Veröffentlichung. Empfänger erkennen daran, welche Nachricht zu welcher Quelle gehört. Deshalb darf sie sich nie ändern und nie für etwas anderes wiederverwendet werden.
- **`ON_REQUEST_SOURCE_10`:** der Sende-Modus „bei Änderung über das Source-Topic und auf Anfrage“. So stehen MAM, Health und Profile in der Liste.
- **Paginierung (Seiten):** Passt eine Antwort nicht in eine MQTT-Nachricht (`MaxPacketSize`), wird sie in nummerierte Seiten aufgeteilt. Wer fragt, kann eine Seitengröße und eine Seite wünschen.
- **sans-IO:** Programmteile, die nur rechnen und entscheiden, aber selbst nichts senden oder lesen. Sie lassen sich ohne Netzwerk vollständig testen; das Senden macht ein dünner Teil darüber.
- **FNV-1a:** ein einfaches, schnelles Prüfsummen-Verfahren. Wir machen damit aus einer langen IL einen kurzen Dateinamen für den Speicher.

### Begriffe aus C-052

- **Mithören (`monitor`):** D7 meldet sich wie eine weitere App am Broker an, abonniert alle OEC-Topics und bewertet, was die anderen senden. Es sendet selbst nichts. Was dabei nicht sichtbar ist (z. B. *wie* sich eine App anmeldet), kann erst der spätere Proxy-Modus prüfen.
- **Capture (Aufzeichnung):** eine Textdatei mit einer Nachricht pro Zeile (Topic, Inhalt, Empfangszeit, QoS, Retain). Oben steht ein Kopf (womit und wann aufgezeichnet), unten ein Abschluss (Ende, Anzahl). `validate` kann so eine Datei auch lesen.
- **Replay (Abspielen):** D7 liest eine Aufzeichnung und bewertet sie so, als käme sie gerade live. Dasselbe Capture liefert immer dasselbe Ergebnis.
- **Fixture:** eine Aufzeichnung als Testdatum mit dem erwarteten Ergebnis. Unsere 18 Fixtures sind R-1.
- **Teilnehmer:** eine App auf dem Bus, erkannt an ihrer PublisherId. Der Bericht fasst die Ergebnisse je Teilnehmer zusammen.
- **Verdict (Urteil):** das Ergebnis je Prüfung: bestanden, Fehler, Warnung, Hinweis (info) oder übersprungen. Wie schwer ein Verstoß wiegt, folgt aus dem Schlüsselwort der REQ (MUST, SHOULD …) und dem geprüften Level.
- **Exit-Code 4 (nicht beurteilbar):** D7 konnte eine MUST-Regel nicht prüfen, z. B. weil der Lauf zu kurz war: Für den Herzschlag braucht es mindestens 2 × 60 + 5 Sekunden. Das ist kein Fehler der App, aber auch kein „bestanden“. Mit `--allow-skip` wird daraus 0.
- **paho:** eine verbreitete, freie MQTT-Bibliothek für Go von der Eclipse Foundation (Lizenz EDL-1.0, in ADR-0025 erlaubt). D7 nutzt sie für die Verbindung zum Broker. Das SDK benutzt sie nicht.
- **govulncheck:** ein Werkzeug der Go-Entwickler, das die benutzten Bibliotheken gegen eine Liste bekannter Sicherheitslücken prüft, ähnlich wie cargo-deny bei Rust.
- **`--insecure`:** schaltet die Prüfung des Broker-Zertifikats ab. Nur für Tests gedacht. D7 warnt dann laut und schreibt es in den Bericht, damit so ein Ergebnis nie als echtes durchgeht.

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
- **A-21 – Konfiguration:** Die Config-Definition wird beim Bauen geprüft (Namen, Typen, Standardwerte). Werte liest man über Namen als Text-Schlüssel. Textmuster (`Pattern`) werden erst ab Inkrement 7 geprüft, weil dafür eine Regex-Bibliothek nötig ist (seit C-056 erledigt, A-106).
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

Neu aus C-052 (D7 `monitor`):
- **A-51 (neu) – Zyklusschnitt:** C-052 wurde nicht geteilt. Aufzeichnen, Abspielen und die Prüfregeln sind klein im Vergleich zu `validate`; für den Live-Teil fehlte nur die MQTT-Anbindung, der Broker war aus C-051 schon da.
- **A-52 (neu) – Bibliothek paho:**
  - D7 verbindet sich über paho v1.5.1 (Lizenz EDL-1.0, in ADR-0025 vorgesehen). Die Version ist fest eingetragen.
  - Mit paho kommen drei kleine Go-Bibliotheken (x/net, x/sync, gorilla/websocket), alle unter BSD-Lizenz.
  - Go-Version jetzt 1.27.3, weil x/net sie verlangt.
  - govulncheck findet keine bekannte Lücke. Es läuft von Hand vor jeder Bibliotheks-Änderung, nicht in der normalen Prüfung, weil es dafür Internet braucht.
- **A-53 (neu) – eigener Test-Broker:** Für die automatischen Tests habe ich einen kleinen Broker im Testcode geschrieben (etwa 400 Zeilen), statt eine weitere Bibliothek (mochi) zu holen. Er kann genau das, was die Tests brauchen, z. B. den Will verschicken, wenn eine Verbindung abreißt.
- **A-54 (neu) – Verbindung und Sicherheit:**
  - Nur verschlüsselt (`mqtts://`), nur TLS 1.2 und 1.3.
  - Das Broker-Zertifikat wird gegen die CA aus `--ca` bzw. aus dem `/etc/oi4`-Ordner geprüft. Ohne CA-Datei nimmt D7 die CAs des Betriebssystems. Das ist anders als beim SDK (A-34): D7 ist ein Werkzeug, das man auch gegen einen Broker mit gekauftem Zertifikat richten können soll.
  - Anmeldung mit Passwort oder Client-Zertifikat; verschlüsselte Schlüssel (PKCS#8, wie im Dev-Ordner) entschlüsselt D7 selbst.
  - Kein automatischer Neuaufbau: Bricht die Verbindung ab, endet der Lauf mit einem Fehler im Bericht. Sonst hätte die Aufzeichnung eine unbemerkte Lücke.
  - `--insecure` nur ausdrücklich, mit Warnung und Vermerk im Bericht.
  - Passwörter und Passphrasen nur aus Dateien, nie auf der Kommandozeile.
- **A-55 (neu) – Aufzeichnungsformat Version 1:** Kopf, eine Zeile je Nachricht, Abschluss (siehe „Capture“). Nicht-JSON-Inhalte stehen Base64-kodiert drin. Der Leser ist streng: Springt die Zeit zurück oder steht nach dem Abschluss noch etwas, lehnt er die Datei ab.
- **A-56 (neu) – Urteile:** Eine Prüfung, die für eine App gar nicht zutrifft (z. B. „Antwort auf Anfrage“, wenn keine Anfrage kam), verschlechtert das Ergebnis nicht. Kann eine MUST-Regel nicht beurteilt werden, endet D7 mit Exit-Code 4.
- **A-57 (neu) – Prüfregeln im Einzelnen:**
  - **Will erkennen:** Der Broker verschickt den Will, aber im Topic sieht er aus wie ein normales Health. D7 erkennt ihn an der Zeit in seiner MessageId: Der Will wird beim Anmelden vorbereitet, ist also nicht neuer als die letzte Birth (REQ-LIFE-016).
  - **Herzschlag** so, wie ich es in SF-106 vorgeschlagen und du es entschieden hast (D-1).
  - REQ-RES-HEALTH-004 lässt sich durch Mithören nicht beurteilen und wird übersprungen.
  - Identitäts-Konflikt (T7-MON-10) nur als Hinweis.
  - Welche QoS eine App benutzt, sieht D7 an der QoS, mit der der Broker die Nachricht zustellt (D7 abonniert mit QoS 1).
- **A-58 (neu) – Sechs Prüfungen verschoben:** T7-MON-09 und 13 bis 17 brauchen ein Modell der PublicationList und der Config einer App (was soll sie wann senden?). Das entsteht ohnehin für `test`; bis dahin bleiben sie „geplant“.
- **A-59 (neu) – Testdaten als Aufzeichnungen:** Die Fixtures (R-1) sind Aufzeichnungen plus `index.yaml`. Ein Test verlangt für jede fertige Prüfung mindestens eine gute und eine schlechte Aufzeichnung.
- **A-60 (neu) – Bedienung:**
  - `--level` ist standardmäßig 3, weil eine volle Konformitätsaussage Level 3 ist (REQ-CMP-011); niedrigere Level muss man ausdrücklich angeben.
  - `--duration` standardmäßig 5 Minuten.
  - `--app` wählt die zu prüfende App; `--config` liest die Einstellungen aus einer YAML-Datei.
  - Neu: `--key-passphrase-file` und `--client-id`.
- **A-61 (neu) – Berichte:** Text, JSON und JUnit-XML (für CI). Das JSON-Schema für den Bericht kommt erst mit `test`, damit ein Schema beide Befehle abdeckt.
- **A-62 (neu) – Live-Test nur auf Wunsch:** `oec/conformance/live.sh` startet den Dev-Broker, lässt D7 zweimal mithören (einmal mit Client-Zertifikat, einmal mit Passwort) und eine simulierte App Birth, Abbruch mit Will und Neuaufbau durchspielen. Ohne Docker meldet es „übersprungen“. Ergebnis am 26.10.: grün in 6 Sekunden; beide sahen 5 Nachrichten, 2 Births und 1 Will. Exit-Code 4 ist dabei richtig, weil 6 Sekunden für den Herzschlag zu kurz sind.

Neu aus C-053 (P4 Inkrement 6a, das SDK als Anbieter):
- **A-63 (neu) – Zyklusschnitt:** Inkrement 6 ist geteilt. 6a (dieser Zyklus) bringt die Standard-Ressourcen, die Antworten auf `Get` und die Seitenaufteilung, schon rund 3200 Zeilen mit Tests. 6b (C-054) bringt die eigenen Daten der App, Sende-Modi, Intervalle und Totbänder. Die Reihenfolge 6 → 7 → 8 → 10 bleibt, weil 7 und 8 auf der PublicationList und den `Get`-Antworten aus 6 aufbauen.
- **A-64 (neu) – Aufbau:** Die ganze Logik (wer, was, wann, in welcher Reihenfolge) steckt in `oec-core` und sendet selbst nichts. Der Laufzeit-Teil in `oec` füttert sie nur und verschickt, was sie liefert. Deshalb lässt sich fast alles ohne Netzwerk testen.
- **A-65 (neu) – Ohne Verbindung kein Rückstau:** Ändert sich etwas, während die Verbindung weg ist, merkt sich das SDK nur „hat sich geändert“. Nach dem Wiederverbinden sendet es den aktuellen Stand jeder Quelle. Empfänger bekommen also keine Flut veralteter Zwischenstände, und der Speicher wächst nicht. Das schließt das offene Risiko aus C-050.
- **A-66 (neu) – Reihenfolge beim Verbinden:** Birth → Health der App → je Asset MAM, Health und (ab Level 2) Profile → Profile der App → PublicationList. Den 60-Sekunden-Herzschlag hat nur die App selbst, so wie du es bei SF-106 entschieden hast.
- **A-67 (neu) – Inhalt von Profile und PublicationList** (ab Level 2): Das Profile nennt MAM, Health, Profile und PublicationList. Die PublicationList enthält je Quelle MAM, Health und Profile mit dem Modus `ON_REQUEST_SOURCE_10`, sich selbst aber nicht. Level 1 hat beides nicht und beantwortet kein `Get`.
- **A-68 (neu) – Feste Nummern:** Die DataSetWriterIds werden gespeichert und nie wiederverwendet, auch nicht nach dem Entfernen eines Assets. Ist die gespeicherte Tabelle kaputt, startet die App mit einer Fehlermeldung, statt still neu zu nummerieren. Sonst würden Empfänger Nachrichten falsch zuordnen.
- **A-69 (neu) – Voreinstellungen für Assets:**
  - Im Typenschild steht als Hardware-Revision `"undefined"` (die Guideline verlangt das für Geräte mit unbekannter Revision), die DeviceClass bleibt leer, solange die App nichts anderes angibt. Health startet mit „normal, 100“.
  - Den RevisionCounter zählt entweder das SDK (gespeichert, Start bei 0) oder er kommt fest vom Typenschild des Geräts.
  - Gespeichert wird er unter einem kurzen Namen aus der Prüfsumme der IL, weil eine IL bis zu 255 Zeichen lang sein kann und Zeichen enthält, die im Dateinamen nicht gehen (offen seit C-048).
- **A-70 (neu) – Größengrenzen:** Antworten auf `Get` dürfen so groß werden, wie der Bus erlaubt (`MaxPacketSize`). Für alles, was die App von sich aus sendet, gilt eine eigene, kleinere Grenze, wenn die App eine gesetzt hat. Was nicht passt, wird in Seiten geteilt. Unser Prüftool D7 hat die Seitenaufteilung akzeptiert.
- **A-71 (neu) – Anfrage zu einem unbekannten Gerät:** Fragt jemand nach einem Gerät, das die App nicht vertritt, antwortet das SDK nicht (das ist Pflicht). Die empfohlene Fehlermeldung (Status-Event `BadNotFound`) kommt erst mit Inkrement 7, weil dort die Events gebaut werden.
- **A-72 (neu) – Kleine Abweichungen von der API-Skizze:**
  - Neu: `assets()` (alle Assets), `mam()` und `is_this()` an einem Asset, `HealthDetail::new`.
  - `bump_revision_counter` meldet jetzt einen Fehler zurück, wenn das Speichern scheitert (in der Skizze ohne Rückgabe).
  - `mam()` hängt jetzt am Handle der App und liefert eine Kopie, weil sich das Typenschild zur Laufzeit ändern kann.
  - Kein Herzschlag für Assets (nach SF-106; die Skizze sagte noch „automatisch für alle“).
- **A-73 (neu) – Tests:** neue Tests in `oec-core` (darunter zwei Zufallstests) und in `oec`, drei davon gegen den eingebauten Test-Broker; ein neuer Interop-Test gegen Mosquitto (zusammen 9 grün). Einmalig habe ich D7 mithören lassen: 10 bestanden, 0 durchgefallen, 3 übersprungen. Die Aufzeichnung habe ich nicht eingecheckt: Die D7-Testdaten werden aus Szenarien erzeugt, und für echte Aufzeichnungen bräuchte D7 eine neue Art von Testdaten (Sache von P5). Deshalb gibt es diesmal keine Testdaten zum Review.
- **A-74 (neu) – Keine neue Bibliothek.** In 6b kommt voraussichtlich eine Zusatzfunktion der schon vorhandenen Bibliothek `uuid` dazu (für die DataSetClassId).

Neu aus C-054 (P4 Inkrement 6b, eigene Daten der App):
- **A-75 (neu) – Zyklusschnitt:** 6b passte in einen Zyklus (rund 2400 Zeilen Code und 1500 Zeilen Tests, ein Thema: der Weg eines Messwerts bis auf den Bus). Das Ändern der PublicationList von außen (`Set PublicationList`) gehört zu Inkrement 7, weil dort `Set` und die Status-Meldungen gebaut werden. Reihenfolge 7 → 8 → 10 bleibt; 7 schneide ich zu Beginn von C-055 neu, falls es zu groß ist.
- **A-76 (neu) – Aufbau:** Die Logik steckt wieder in `oec-core` (drei neue Teile: Datentypen und Berechnungen, Zustand je Datensatz, Versionsnummern), die API in `oec::data`. Die Versionsnummern werden wie die DataSetWriterIds gespeichert; ist die Datei kaputt, startet die App mit Fehler.
- **A-77 (neu) – DataSetClassId:** berechnet aus Tag, Feldnamen und Feldtypen. Einheit, Beschreibung und Messbereich zählen nicht mit, denn sie ändern die Struktur nicht. Wer eine eigene Kennung braucht, kann sie angeben.
- **A-78 (neu) – Inhalt der Metadaten:** je Feld Typ, Beschreibung, Einheit und Messbereich. Bei der Einheit setzen wir nur den Text (z. B. „%“) und die Kennnummer „unbekannt“ (−1). Das Beispiel in der Guideline nutzt eine echte UNECE-Nummer; die können wir später als Zusatz anbieten, ohne die API zu brechen. Versionsnummer: ändert sich die Struktur, steigen beide Teile, sonst nur der zweite.
- **A-79 (neu) – Wann Metadaten gesendet werden:** beim Anlegen, nach jedem Verbinden und bei einer neuen Version, immer vor den Daten. Auf Anfrage nur für Datensätze, die in der PublicationList stehen und nicht abgeschaltet sind; sonst schweigt das SDK (so erlaubt es die Guideline).
- **A-80 (neu) – Nur vollständige Datensätze:** Eine Datennachricht enthält immer alle Felder. Deshalb geht ein Datensatz erst raus, wenn jedes Feld einen Wert hat. `update` setzt alle angegebenen Werte oder keinen. Werte werden in den Feldtyp umgewandelt (z. B. die Zahl 5 in ein Byte-Feld); passt ein Wert nicht, gibt es einen Fehler.
- **A-81 (neu) – Sende-Modi im Detail:** „einzeln“ schickt nur den geänderten Datensatz. „Je Quelle“ bzw. „ganze App“ schicken bei einer Änderung den aktuellen Stand aller Datensätze dieser Gruppe in einer Nachricht. „Aus“ (`OFF_0`) heißt: nie senden, nie antworten; der Datensatz steht trotzdem mit „aus“ in der PublicationList.
- **A-82 (neu) – Intervall und Totband:** Mit Intervall sendet das SDK regelmäßig und holt verpasste Takte nicht nach; der erste vollständige Stand und jeder Neuverbindung gehen sofort raus. Ohne Intervall sendet es bei Änderung, mit Totband erst ab der Mindeständerung je Feld. Die Zeit kommt von außen, damit die Tests ohne Warten laufen.
- **A-83 (neu) – Anfragen und Listen:** `Get Data` liefert den aktuellen Stand, aufgeteilt in Seiten, wenn nötig; ein unbekannter Tag ergibt eine leere Liste. Profile und PublicationList nennen jetzt auch Data und Metadata. Wird ein Datensatz entfernt, bleiben seine Nummer und seine Version reserviert.
- **A-84 (neu) – Kleine Abweichungen von der API-Skizze:**
  - Neu: Messbereich je Feld (`range`), am Datensatz `tag()`, `source()` und `values()`.
  - `update` meldet jetzt einen Fehler zurück (falscher Feldname, falscher Typ), statt ihn zu verschlucken.
  - Der „Roh-JSON“-Wert der Skizze entfällt, weil das SDK ihn nicht gegen den Feldtyp prüfen könnte; dafür gibt es für jeden Feldtyp einen passenden Werttyp.
  - Schreibbare Datensätze (`on_set`) kommen mit Inkrement 7.
- **A-85 (neu) – Tests und ein Fehler in D7:** neue Tests in `oec-core` und `oec`, ein neuer Interop-Test (zusammen 10 grün). Beim einmaligen Mithören mit D7 gab es zuerst 2 Fehlschläge, beide nicht im SDK: Mein Test hatte dieselbe Nachrichtennummer zweimal benutzt (behoben), und **D7 hielt unaufgeforderte Metadaten für „leer“**, weil es den Inhalt an der falschen Stelle suchte (bei Metadaten steht er in `MetaData`, nicht in `Messages`). Das habe ich in D7 behoben, zuerst mit einem Test, der den Fehler zeigt. Deine 18 reviewten Testaufzeichnungen und ihre Ergebnisse ändern sich dadurch nicht. Danach: 10 bestanden, 0 durchgefallen, 3 übersprungen.
- **A-86 (neu) – Bibliothek:** `uuid` bekommt die Funktion `v5`; dafür kommt die kleine Bibliothek `sha1_smol` (BSD-3-Lizenz, ohne weitere Abhängigkeiten) dazu. SHA-1 dient hier nur zum Berechnen der Kennung, nicht der Sicherheit. Die Lizenzprüfung (`cargo deny`) läuft ohne Ausnahme.


Neu aus C-055 (P4 Inkrement 7a, die Schreib-Seite):
- **A-87 – Zyklusschnitt:** 7 ist geteilt. 7a (jetzt) ist ein Mechanismus: Anfrage prüfen, mit einer Status-Meldung antworten, über `ReplyTo` zuordnen; schon so rund 1500 Zeilen Code und 1330 Zeilen Tests. 7b (Config) ist ein eigenes, sicherheitsrelevantes Thema (Maskierung von Passwörtern, Secrets) und nutzt den Weg aus 7a unverändert. 7c (ReferenceDesignation, Syslog- und Generic-Events) sind kleine Ergänzungen. Die SubscriptionList gehört zur Empfänger-Seite (8). Reihenfolge 7a → 7b → 7c → 8 → 10.
- **A-88 – Aufbau:** Die Logik steckt in `oec-core` (Status-Codes, Schreib-Anfragen, gespeicherte Sende-Einstellungen). Bei `Set Data` fragt `oec-core` nicht selbst den Handler der App, sondern gibt die geprüfte Anfrage an die Laufzeit zurück; die fragt den Handler und gibt das Ergebnis zurück (zwei Schritte). So läuft im Kern nie fremder Code.
- **A-89 – Ab welchem Level:** Änderungs-Anfragen nimmt das SDK erst ab Level 2 an. Level 1 bietet nichts Beschreibbares an; `on_set` meldet dort einen Fehler. Eine Anfrage, die kein gültiges JSON ist oder keine Nachrichtennummer hat, kann man nicht beantworten (kein `ReplyTo`) – sie wird ignoriert. Eine Anfrage muss genau einen Datensatz enthalten.
- **A-90 – Form der Status-Meldung:** Topic `Pub/Event/<Absender>/Status/<Good|Uncertain|Bad>`, mit `ReplyTo`, Nummer und – für bekannte Codes – Name und Beschreibung. Jede Quelle bekommt feste Nummern (DataSetWriterIds) für ihre drei Status-Stufen; die PublicationList nennt sie, das Profile nennt `Event`.
- **A-91 – Unbekannte Geräte:** `BadNotFound` geht heute unter dem Namen des angefragten (fremden) Geräts raus, so wie es wörtlich in der Guideline steht. Diese Antworten bekommen vorübergehende Nummern, höchstens 64 fremde Geräte je Lauf; danach antwortet das SDK nicht mehr (Schutz gegen Überflutung). Seit deiner Entscheidung zu D-2 abgelöst durch A-109.
- **A-92 – Was noch nicht geht:** `Set` auf Config, SubscriptionList, ReferenceDesignation und jedes `Del` → `BadNotFound`, bis 7b/7c/8 sie bringen (Config seit C-056 erledigt). `Set Metadata` → `BadNotWritable` (Metadaten folgen immer der Beschreibung in der App).
- **A-93 – Regeln für `Set PublicationList`:** Der Eintrag wird über Ressource, Absender und Filter gefunden. Unbekannt → `BadNotFound`; nicht lesbar → `BadDecodingError`; eine Änderung, die `Config` nicht erlaubt (oder eines festen Felds) → `BadNotWritable`; ein ungültiges Totband → `BadInvalidArgument`; nichts geändert → trotzdem „Good“. Bei Erfolg: erst der geänderte Eintrag, dann „Good“. Die ganze Liste kommt nur neu, wenn ein Datensatz ein- oder ausgeschaltet wurde.
- **A-94 – Speichern:** Die geänderten Einstellungen werden gespeichert (wie die Nummern und Versionen). Ist die Datei kaputt, startet die App mit Fehler. Beim Start gilt eine gespeicherte Einstellung nur, wenn die App sie noch erlaubt – ändert der Entwickler sein Programm, erbt es keine verbotene Einstellung.
- **A-95 – Regeln für `Set Data`:** unbekannter oder abgeschalteter Datensatz → `BadNotFound`; kein Handler → `BadNotWritable`; Feld fehlt → `BadInvalidArgument`; falscher Typ → `BadTypeMismatch`; unbekannte Felder werden ignoriert (so verlangt es die Guideline für Empfänger). Dann entscheidet der Handler der App (siehe oben). Stürzt der Handler ab, meldet das SDK `BadInternalError`. Der Handler muss kurz sein, weil er auf dem Bus-Takt läuft (steht in der Doku).
- **A-96 – Kleine Abweichungen von der API-Skizze:** `on_set` meldet jetzt einen Fehler zurück (z. B. auf Level 1), `SetData` kann alle Felder aufzählen. Die Skizze ist angepasst.
- **A-97 – Tests:** neue Tests in `oec-core` und `oec`, ein neuer Interop-Test (zusammen 11 grün). D7 fand beim ersten Mithören 5 ungültige Anfragen – alle von meinem Test, nicht vom SDK; behoben. Danach 9 bestanden, 0 durchgefallen, 4 übersprungen (einer wegen D-2, drei, weil der Lauf zu kurz war oder die Situation nicht vorkam). Keine neue Bibliothek.


Neu aus C-056 (P4 Inkrement 7b, die Config-Ressource):
- **A-98 (neu) – Zyklusschnitt:** 7b passte in einen Zyklus (rund 2000 Zeilen Code und 1450 Zeilen Tests). Config, Maskierung und Secrets gehören zusammen: Ohne Maskierung würde eine Config Passwörter im Klartext senden. Reihenfolge bleibt 7b → 7c → 8 → 10.
- **A-99 (neu) – Aufbau:** Die Logik steckt in `oec-core` (`provider::config`): Prüfen der Beschreibung und der Werte, Maskieren, Prüfen eines `Set Config`, Speichertabelle. Den Handler der App fragt – wie bei `Set Data` – die Laufzeit, in zwei Schritten.
- **A-100 (neu) – Wann die Config gesendet wird:** beim Anlegen, nach jedem Verbindungsaufbau und nach jeder Änderung, immer maskiert. Auf Level 1 wird sie nur gesendet; Abfragen und Ändern gibt es ab Level 2, dann steht `Config` auch im Profile. Die PublicationList bekommt je Config einen Eintrag („auf Anfrage und bei Änderung“).
- **A-101 (neu) – Regeln für `Set Config`:** nur einzelne Werte schicken ist erlaubt, aber die Pflichtfelder einer mitgeschickten Gruppe müssen dabei sein. Feste Angaben (Name, Einheit, Grenzen …) darf man unverändert mitschicken, aber nicht ändern. Der erste Fehler bestimmt die Antwort: unbekannt → `BadNotFound`; Wert oder Pflichtfeld fehlt → `BadInvalidArgument`; kein JSON-Objekt → `BadDecodingError`; falscher Typ → `BadTypeMismatch`; feste Angabe geändert → `BadNotWritable`; außerhalb der Grenzen oder Config zu groß → `BadOutOfRange`. Ein leerer Wert löscht die Einstellung. Unbekannte Zusatzangaben werden ignoriert.
- **A-102 (neu) – Reihenfolge der Antworten:** erst die neue Config, dann „Good“ – wie bei `Set Data`. Das Beispiel in Anhang B1.1.3 zeigt es andersherum, ist aber nur informativ, und keine Regel legt die Reihenfolge fest. Deshalb kein Finding.
- **A-103 (neu) – Maskierung:** Auf dem Bus steht für einen geheimen Wert immer `********` (ein leerer Wert bleibt leer). Sternchen in einem `Set` heißen „unverändert lassen“. Die App selbst kann die Sternchen nicht als echten Wert setzen. In Log-Ausgaben und Fehlermeldungen steht nie ein Wert. Der Handler der App sieht die Werte im Klartext – er braucht sie ja.
- **A-104 (neu) – Speichern:** Gespeichert werden nur geänderte Werte; beim Start gelten sie nur, wenn die Einstellung noch existiert und der Wert noch gültig ist. Eine kaputte Speicherdatei → die App startet mit Fehler. Jede Änderung erhöht den Änderungszähler im Typenschild. **Wichtig:** Geheime Werte liegen in der Speicherdatei im Klartext; geschützt sind sie durch die Dateirechte (nur der Besitzer darf lesen). Verschlüsselung verlangt die Guideline nicht; eine Plattform kann das Datenverzeichnis verschlüsselt einbinden.
- **A-105 (neu) – Secrets erneuern:** Vor jedem neuen Verbindungsaufbau liest das SDK die Zugangsdaten und Schlüssel neu ein. Unverändert → nichts passiert; neu → sie gelten ab sofort; kaputt → die letzten guten bleiben, mit einer Warnung ohne Inhalt. Geheime Config-Werte legt das SDK **nicht** unter `/run/secrets/oi4` ab; das Verzeichnis gehört laut GL-ADR-022 der Plattform.
- **A-106 (neu) – Textmuster:** geprüft mit der Rust-Bibliothek `regex` (findet das Muster irgendwo im Text, arbeitet ohne Gefahr von Endlos-Rechnungen). Ein Muster, das nicht übersetzbar ist, wird schon beim Anlegen abgelehnt. D7 nutzt den Go-Dialekt; bei Spezialfällen (z. B. `\d` auch für nicht-lateinische Ziffern) können sich beide unterscheiden – die Guideline weist selbst darauf hin. Die Bibliothek ist neu (MIT/Apache-Lizenz), `cargo deny` ohne Ausnahme. Damit ist A-21 erledigt.
- **A-107 (neu) – Kleine Abweichungen von der API-Skizze:** `on_set` meldet einen Fehler zurück (z. B. auf Level 1), neu sind ein Anzeigename für die Config (`label`), `context()`, `source()` und `remove()`. Die Skizze ist angepasst.
- **A-108 (neu) – Tests:** neue Tests in `oec-core` und `oec`, ein neuer Interop-Test (zusammen 12 grün), D7 10 bestanden / 0 durchgefallen / 3 übersprungen nach der Korrektur in D7 (siehe oben).

Neu aus deinem Review von C-056:
- **A-109 (neu) – SF-108 im SDK:** Eine Anfrage (`Get`, `Set`, `Del`) zu einem Gerät, das die App weder ist noch vertritt, beantwortet das SDK mit `BadNotFound` unter dem **eigenen Namen der App** (`Pub/Event/<AppId>/Status/Bad`, mit `ReplyTo`). Die vorübergehenden Nummern und die Grenze von 64 fremden Geräten sind weg: Es gibt keinen fremden Absender mehr, also auch nichts zu begrenzen. Zuerst die Tests geändert (sie schlugen fehl), dann den Code. Dazu ein Fehler im Interop-Test (erstes `Get` zu früh, siehe oben). D7 einmal mitgehört: 10 bestanden, 0 durchgefallen, 3 übersprungen; T7-MON-04 besteht jetzt.

Neu aus C-057 (P4 Inkrement 7c):
- **A-110 (neu) – Zyklusschnitt:** 7c passte in einen Zyklus (rund 1700 Zeilen Code und 1850 Zeilen Tests). Reihenfolge innerhalb: ReferenceDesignation → eigene Ereignisse → Syslog-Brücke. Danach bleibt es bei 8 → 10.
- **A-111 (neu) – ReferenceDesignation:** ab Level 2; Abfrage für ein Gerät, für alle oder für die direkt untergeordneten (Query, Vergleich Zeichen für Zeichen). Setzen ersetzt die ganze Kennzeichnung, Löschen entfernt sie; der Handler der App entscheidet, ein Absturz des Handlers wird zu `BadInternalError`. Dasselbe noch einmal setzen → „Good“ ohne Speichern; Löschen, wo nichts ist → `BadNotFound`; kaputte Nachricht → `BadDecodingError`. Wird beim Verbindungsaufbau nicht gesendet, nur auf Anfrage. Für `Set`/`Del SubscriptionList` bleibt es bis Inkrement 8 bei `BadNotFound`.
- **A-112 (neu) – Speichern:** in `app/reference-designations`; eine kaputte Datei → die App startet mit Fehler. Wird ein Gerät entfernt, bleibt seine Kennzeichnung gespeichert und ist wieder da, wenn es zurückkommt.
- **A-113 (neu) – Änderungszähler:** siehe oben; das ist eine Auslegung von MG 5, kein Finding. Auf Wunsch SF-109.
- **A-114 (neu) – Späte Antworten:** Wird ein Gerät entfernt, während sein Handler noch über eine Anfrage entscheidet, antwortet das SDK unter dem Namen der App (wie bei SF-108). Gilt für ReferenceDesignation, Config und Data.
- **A-115 (neu) – Eigene Ereignisse:** höchstens 16 Arten pro Gerät; Details müssen ein JSON-Objekt sein; ohne Verbindung oder bei 256 wartenden Ereignissen wird verworfen (die App bekommt trotzdem „Ok“, weil ein Ereignis von „jetzt“ handelt); auch auf Level 1 erlaubt.
- **A-116 (neu) – Syslog-Events:** Nummer = Syslog-Priorität (Facility „user“ × 8 + Stufe), Details `MSG` und `HEADER` wie im Beispiel B1.1.6. Acht Einträge in der PublicationList; Informational und Debug sind aus (Regel der Guideline), alle anderen an. Umschalten nur zwischen an und aus; gespeichert.
- **A-117 (neu) – Log-Brücke:** wird beim ersten `build()` eingerichtet, wenn die App noch keinen Logger hat; wer einen eigenen behalten will, nimmt `install_bridge`. Fehler → Error, Warnung → Warning, Info → Informational, Debug/Trace → Debug. SDK- und Netzwerk-Meldungen bleiben lokal, damit es keine Rückkopplung gibt. Drosselung 50 auf einmal, dann 10 pro Sekunde; Text höchstens 1024 Byte. **Keine Geheimnisse loggen** (GL-ADR-021).
- **A-118 (neu) – Kleine Abweichungen von der API-Skizze:** einige Funktionen melden jetzt einen Fehler zurück (`Result`), eigene Ereignisse baut man mit `GenericEvent::new(…)`, neu sind `reference_designation()` und `install_bridge`. Die Skizze ist angepasst.
- **A-119 (neu) – Tests:** neue Tests in `oec-core` und `oec`, die Syslog-Tests in einem eigenen Testprogramm (der Logger gilt für den ganzen Prozess), ein neuer Interop-Test (zusammen 13 grün), D7 9 bestanden / 0 durchgefallen / 4 übersprungen. Keine neue Bibliothek.

Neu aus C-058 (P4 Inkrement 8a):
- **A-120 (neu) – Zyklusschnitt:** Inkrement 8 hat drei Themen: die Empfänger-Seite (eine neue Rolle mit eigener Logik), die SubscriptionList (sie listet die Abos der Empfänger-Seite, braucht diese also) und das Löschen der Skizze (geht erst, wenn alles aus der Skizze im SDK steht). Allein die Empfänger-Seite sind rund 1500 Zeilen Code und 1430 Zeilen Tests. Deshalb 8a = Empfänger-Seite (C-058), 8b = SubscriptionList, Aktion `NewDataSetWriterId`, Skizze löschen, Beispiel-App (C-059; der nächste Agent darf die Aktion als 8c abtrennen).
- **A-121 (neu) – Erst Abo, dann Frage:** Das SDK schickt eine Frage erst ab, wenn der Broker die Abos für die Antworten bestätigt hat. Lehnt der Broker ein Abo ab (fehlende Rechte), endet die Frage mit einem Fehler, und das SDK versucht das Abo nicht immer wieder.
- **A-122 (neu) – Zuordnung:** Eine Antwort zählt nur, wenn `ReplyTo` passt, sie vom gefragten Programm kommt (dessen `Pub/`-Topic) und nach der Frage eintrifft. Die Fragen tragen kein `MessageType` im Datensatz – genau wie die Beispiele der Guideline; die Tests vergleichen sie mit den Anfrage-Vektoren.
- **A-123 (neu) – Wer ist da:** Anmeldung = Typenschild ohne `ReplyTo`; Abmeldung = Health „normal“ mit 0; Absturz = Health „Fehler“ mit 0; ein Lebenszeichen einer unbekannten App → sie gilt als da, und das SDK fragt einmal nach ihrem Typenschild.
- **A-124 (neu) – Ohne Verbindung:** keine Warteschlange; `Error::NotConnected`. Reißt die Verbindung ab, enden die offenen Fragen mit diesem Fehler. Abos bleiben über einen Neuaufbau der Verbindung erhalten.
- **A-125 (neu) – Datensatz-Nummer bei `Set`/`Del`:** Die Guideline verlangt die Nummer des Datensatzes (DataSetWriterId). Das SDK findet sie selbst: bei Data/Config durch eine Frage, bei der ReferenceDesignation über die PublicationList (die Kennzeichnung gibt es vielleicht noch nicht – das hat ein Test gezeigt). Sonst gibt die App sie an.
- **A-126 (neu) – Abweichungen von der API-Skizze:** Optionen werden mit „Baukasten“-Funktionen gebaut, `get` braucht eine bestimmte App (Geräte finden geht über `on_presence`/`apps()`), Antworten haben typisierte Zugriffe, neue Fehler `Error::Status` und `Error::NotConnected`. Die blockierenden Varianten verweigern den Aufruf aus einem `async`-Thread (sonst würde das Programm hängen). Die Skizze bleibt unverändert, weil 8b sie löscht; alles steht in ADR-0020.
- **A-127 (neu) – Tests:** 14 Tests ohne Netzwerk (Antworten kommen vom echten Provider des SDK und werden gegen das Schema geprüft), 4 Tests mit zwei Apps über den Test-Broker (der kann jetzt Nachrichten zwischen Apps weiterleiten), ein neuer Interop-Test (zusammen 14 grün), D7 11 / 0 / 2. Keine neue Bibliothek.

---

Neu aus C-059 (P4 Inkrement 8b):
- **A-128 (neu) – Zyklusschnitt:** 8b hatte zwei etwa gleich große Themen: die SubscriptionList (eine Liste der App, die von der Empfänger-Seite gefüllt wird und auf sie zurückwirkt) und die Aktion `NewDataSetWriterId` (eine ganz andere Nachrichtenart mit eigenem Aufbau). Die Skizze kann erst nach der Aktion gelöscht werden. Deshalb 8b = SubscriptionList + Beispiel-App (C-059), 8c = Aktion + Abschied von der Skizze (C-060).
- **A-129 (neu) – Was in der Liste steht:** zuerst je ein Eintrag pro abonniertem Topic der Empfänger-Seite (Intervall = das kleinste der Abos auf diesem Topic; „einstellbar“ nur, wenn alle Abos auf dem Topic einstellbar sind), dann je beschreibbarem Datensatz das `Set`-Abo (nicht einstellbar, SF-107). Eine gemeinsame Datensatz-Nummer für die ganze Liste. Die Liste gibt es ab Level 2; das Profile der App nennt sie, die PublicationList nicht. Die kurzen Antwort-Abos stehen nicht drin (SF-109).
- **A-130 (neu) – Neu schicken:** die ganze Liste, sobald sie sich geändert hat; nach dem Verbinden nur, wenn sie nicht leer ist.
- **A-131 (neu) – `Set`/`Del`:** Gründe für „Bad“: unbekannt (`BadNotFound`), nicht lesbar (`BadDecodingError`), Topic außerhalb der angesprochenen Gruppe (`BadInvalidArgument`), Eintrag nicht einstellbar (`BadNotWritable`), **Topic gar nicht abonniert → `BadNotSupported`** – das SDK legt keine Abos auf Zuruf von außen an (SF-111). Ein gelöschter Eintrag lässt sich mit `Set` wiederherstellen.
- **A-132 (neu) – Wie das Intervall wirkt:** Kommt eine Nachricht desselben Topics innerhalb des Intervalls nach der letzten zugestellten, verwirft das SDK sie. Es liefert am Ende des Intervalls nicht „den letzten Wert“ nach (das würde einen Zeitgeber pro Topic brauchen; die Guideline sagt nur „Verarbeitungsintervall“). Ein gelöschtes Abo wird beim Broker abgemeldet.
- **A-133 (neu) – Speichern:** Änderungen von außen unter `app/subscriptions`; eine kaputte Datei → die App startet mit Fehler (wie bei den anderen Tabellen).
- **A-134 (neu) – API:** `Subscription::interval` und `configurable` wie in der Skizze. Für `Set`/`Del SubscriptionList` findet das SDK die Datensatz-Nummer selbst über eine Frage.
- **A-135 (neu) – Beispiel-App:** `examples/demo.rs` – eine Level-2-App mit einer Pumpe (Datensatz `oee` beschreibbar, mit Bereichsprüfung), Config, ReferenceDesignation und einem einstellbaren Abo mit 1 s Intervall; läuft auf Wunsch eine feste Zeit (`OEC_DEMO_SECONDS`) und meldet sich dann sauber ab.
- **A-136 (neu) – Tests:** neue Tests in `oec-core` und `oec` (darunter die vier Beispiele aus B1.1.9), der behobene Fehler aus C-058 (siehe oben), Interop 14 grün, D7 12 / 0 / 1. Keine neue Bibliothek.

---

Neu aus C-060 (P4 Inkrement 8c):
- **A-137 (neu) – Zyklusschnitt:** nicht geteilt. Die Aktion baut auf Vorhandenem auf (Nachrichten-Modell seit C-047, Nummern-Tabelle seit C-053, Fragen mit Zeitlimit seit C-058); der Abschied von der Skizze ist reine Umzugsarbeit.
- **A-138 (neu) – Antworten:** ab Level 2 jede Frage sofort mit genau einer abschließenden Antwort. Keine Antwort, wenn die Frage nicht lesbar ist, nicht an diese App geht oder die Antwort-Adresse nicht passt (so verlangt es die Guideline). Ein kaputter Eintrag lässt nur seine eigene Frage scheitern (`BadInvalidArgument`). Unbekanntes Gerät → `BadNotFound` (Vorschlag D-2). Mehrere Antworten passen in eine Nachricht bis zur Größengrenze.
- **A-139 (neu) – Reservierung:** Die neue Nummer liegt über allen bisher vergebenen, ist 60 s reserviert, wird gespeichert und nie wieder vergeben (strenger als nötig, passt zu Vorschlag D-1). Höchstens 256 offene Reservierungen. Ein `Set` benutzt weiter die Nummer, die die App anzeigt.
- **A-140 (neu) – Profil:** `NewDataSetWriterId` steht ab Level 2 im Profil der App und jedes Geräts (Level 2: „sollte“, Level 3: „muss“).
- **A-141 (neu) – Fragen:** Die Erkennungsmarke der Antwort (CorrelationData) ist die Fragenummer; das Zeitlimit wird mitgeschickt; nur die abschließende Antwort auf die eigene Frage zählt; eine „gute“ Antwort ohne Nummer gilt als Fehler.
- **A-142 (neu) – API:** `new_data_set_writer_id(Ressource, &Target, &ActionOptions) -> ProvidedWriterId`; Fehler wie bei `get` (`Error::Status`, `Error::Timeout`); Standard-Zeitlimit 10 s.
- **A-143 (neu) – Abschied von der Skizze:** Beispiele ins SDK (das Konsumenten-Beispiel fragt jetzt einen bestimmten OT-Connector statt „alle“, weil `get` seit C-058 eine bestimmte App braucht), `oec.h` + C-Beispiele nach `oec-ffi/` (die Aktion kommt in C mit Inkrement 10), REQ-Prüfung im SDK, Skizze gelöscht.
- **A-144 (neu) – Tests und Läufe:** 10 + 3 neue Tests ohne Netzwerk, 1 mit zwei Apps, 25 Läufe unter Last grün, Interop 14 grün (nach dem Fix am Test-Ordner), D7 13 / 0 mit T7-MON-07. Keine neue Bibliothek.

---

Neu aus C-061 (P4 Inkrement 10):
- **A-145 (neu) – Zyklusschnitt:** nicht geteilt. Das Rust-SDK ist seit C-060 vollständig; die C-Schnittstelle ist eine Übersetzungsschicht derselben Art für alles (Handles, Texte, JSON-Werte, Callbacks, Fehler). Die kleinen Ergänzungen am Rust-SDK kamen zuerst in einem eigenen Commit.
- **A-146 (neu) – Ergänzungen am Rust-SDK:** Funktionen, die Werte als JSON annehmen bzw. ausgeben (Config-Definition, Config-Werte, Datensatz-Werte, Werte eines `Set`), und eine Syslog-Funktion pro App. Nichts Bestehendes geändert (ADR-0020, Nachtrag C-061).
- **A-147 (neu) – Erzeugter Header:** cbindgen 0.29.4 fest vorgegeben, Header eingecheckt und im Prüfskript verglichen. Die Aufzählungen (z. B. Level, Health) sind Zahlen mit festen Namen statt C-„enums“: Ein ungültiger enum-Wert wäre in Rust undefiniertes Verhalten, eine ungültige Zahl dagegen gibt sauber einen Fehler. C-Code, der die Namen benutzt, ändert sich nicht.
- **A-148 (neu) – Änderungen an Funktionen:** Abos und Fragen an andere Apps nennen den Service-Typ (bei Abos „egal“ möglich: `OEC_ST_ANY`); Ereignisse bekommen ihre Stufe (`level`); `oec_dataset_set_precision` entfällt – die Genauigkeit steht als `"Precision"` in der Feldbeschreibung, wie im Rust-SDK.
- **A-149 (neu) – Fehler:** Die Fehlermeldung (`oec_last_error`) wird nur bei einem Fehler gesetzt und gilt pro Thread. Neu: „nicht verbunden“ (`OEC_ERR_NOT_CONNECTED`) und „die andere App hat mit Bad geantwortet“ (`OEC_ERR_STATUS`, der Code über `oec_last_status_code`). Ein abgefangener Absturz macht die App unbrauchbar (`OEC_ERR_PANIC`), statt das Programm mitzureißen.
- **A-150 (neu) – Threads und Handles:** Callbacks laufen auf dem Thread, der die App laufen lässt, immer einer nach dem anderen. In einem Callback sind wartende Aufrufe, `oec_app_run` und `oec_app_free` gesperrt (sie würden auf sich selbst warten). Handles bleiben gültig, bis die App freigegeben wird – auch nach dem Entfernen eines Geräts oder Datensatzes; Aufrufe darauf geben dann einen Fehler statt eines Absturzes. Einschränkung: Startet man die App im Hintergrund (`oec_app_start`) und der Start scheitert, erfährt das C-Programm das nicht direkt (Risiko; `oec_app_run` meldet es).
- **A-151 (neu) – Ergänzungen und Lücken:** neu in C: `oec_set`, `oec_del`, die Aktion `oec_new_data_set_writer_id`, Datensatz entfernen, Config-Wert aus der App setzen, Builder freigeben. Noch nicht in C (kann später dazukommen, die Logger-Demo braucht nichts davon): u. a. Anwesenheit anderer Apps, einstellbare Abos, ReferenceDesignation, Verbindungsstatus. Auslieferung: Bibliothek + Header; fertige Pakete kommen mit P9.
- **A-152 (neu) – Tests:** 8 Rust-Tests der Schicht ohne Netzwerk; 3 C-Tests (hello, logger, das große Testprogramm) gegen beide Bibliotheksformen mit AddressSanitizer + UBSan, Pflicht im Prüfskript; Lasttest mit 6 Threads; einmal Linux mit LeakSanitizer und Testbauten für Linux amd64, Windows (GNU) und ARMv7; Interop 14 grün. Keine neue Bibliothek.

---

Neu aus C-062 (P5 Inkrement 1):
- **A-153 (neu) – ein Abo für alles:** Ein `test`-Lauf abonniert den Bus einmal; jede Nachricht geht an die passiven Regeln (und die Aufzeichnung), dann an die aktiven Tests. Deshalb lässt sich ein Testlauf wieder abspielen.
- **A-154 (neu) – QoS 0:** D7 schickt seine Fragen mit QoS 0 (Standard laut REQ-MQTT-017). Die Fragen sind gültige Nachrichten, bis auf das absichtlich unbekannte Feld in T7-GET-09.
- **A-155 (neu) – Tester-Kennung:** `Utility/oi4.org/oec-check/D7/<Zufall>`, je Lauf neu, ohne Anmeldung (keine Birth/MAM; Anmelden kommt mit `--announce` in Inkrement 6).
- **A-156 (neu) – App finden:** Frage nach dem MAM auf dem angegebenen Service-Typ oder auf allen sieben; notfalls reicht eine gesehene Nachricht der App. Nicht gefunden → Exit 4 („nicht beurteilbar“).
- **A-157 (neu) – Warten:** Tests nacheinander; nach einer Antwort 250 ms für Doppelte; wenn keine Antwort erwartet wird, wartet D7 die volle Zeit (`--timeout`, 10 s).
- **A-158 (neu) – Zuständigkeit:** Jeder aktive Test urteilt nur über die REQs aus seinem Katalogeintrag.
- **A-159 (neu) – Kurzer Lauf:** Was ein kurzer Lauf nicht beurteilen kann (Verbindungsaufbau nicht gesehen, kürzer als der Herzschlag), wird übersprungen („test run: …“) statt „nicht beurteilbar“. Für eine Level-Aussage braucht es zusätzlich einen `monitor`-Lauf von mindestens 125 s.
- **A-160 (neu) – Bericht:** derselbe Bericht wie bei `monitor`, mit Modus `test`, Tester und der Liste der Fragen; das Report-Schema kommt in Inkrement 7.
- **A-161 (neu) – Auswahl:** `--tests` nimmt Test-Ids oder Muster (`T7-GET-*`); nicht ausgewählte Tests werden übersprungen.
- **A-162 (neu) – Aufruf:** `--app` ist Pflicht und muss eine gültige Kennung sein, d. h. mit kodierten Sonderzeichen (`OEC Demo` → `OEC,20Demo`); die Fehlermeldung sagt das.
- **A-163 (neu) – Einzelheiten und Zuschnitt:** wie GET-05 die leere Liste und das unbekannte Gerät provoziert, GET-01 fragt höchstens 20 Geräte; der P5-Zyklusschnitt.

Neu aus C-063 (P5 Inkrement 2):
- **A-164 (neu) – Begrenzte Fragen:** höchstens 5 Filter je Ressource, 50 Einträge der PublicationList, 20 untergeordnete Geräte. Eine große App wird stichprobenartig geprüft, nicht vollständig.
- **A-165 (neu) – Metadaten:** D7 fragt Metadaten nur zu den Datensätzen, die in der PublicationList stehen. Metadaten ohne Datensatz (REQ-CMP-044) sind freiwillig und werden nicht abgefragt.
- **A-166 (neu) – Aufteilen nach Größe:** D7 kann den Inhalt der App nicht größer machen als `MaxPacketSize`. GET-06 prüft das Aufteilen auf Bitte, die Größe jeder Nachricht prüft die passive Regel T7-MON-18.
- **A-167 (neu) – Versionen:** D7 spricht nur Version 2 und hört bei Version 1 (`Oi4`) nur mit. Kommt dort eine Antwort auf eine Frage von D7 → durchgefallen; kommt dort gar nichts → übersprungen. Eine App, die beide Versionen kann und während des Laufs unter `Oi4` schweigt, besteht.
- **A-168 (neu) – „Ignorieren“:** heißt „antwortet wie auf dieselbe Frage ohne den Zusatz“. Das gilt für die Query mit Absender-Kennung, die Query bei MAM, unbekannte Query-Felder und unbekannte Felder. Ohne ReferenceDesignation-Einträge wird die Query nicht beurteilt.
- **A-169 (neu) – Sprache:** Auf eine Frage auf Deutsch darf die Antwort Deutsch, Englisch (Standardsprache) oder sprachneutral sein. Ob die App Deutsch kann, sieht man von außen nicht.
- **A-170 (neu) – Fehlalarm Sprache:** Eine Antwort auf eine Frage mit Sprache wird nicht mehr mit der Anmeldung (Birth) verglichen.
- **A-171 (neu) – Fehlalarm Seite:** Die Antwort auf die Frage nach einer bestimmten Seite gilt als vollständig. Deshalb wurde der Testfall `pagination-broken` korrigiert (R-2).
- **A-172 (neu) – Inhalt der simulierten App:** Es kamen drei geprüfte Beispielnachrichten dazu (Metadaten, MAM auf Deutsch, ReferenceDesignation des Geräts). Der Konfig-Gruppenname wird ohne Leerzeichen als Filter benutzt, damit keine Warnung entsteht.
- **A-173 (neu) – Selbstprüfung der Fragen von D7:** Ohne Fehler sind genau die absichtlich „falschen“ Fragen beanstandet (unbekannte Felder, Query bei MAM, Query mit Absender-Kennung). Mit Fehler ist es ein Teil davon, weil ein Test früher abbrechen kann.
- **A-174 (neu) – Wartezeiten im Selbsttest:** 3 s statt 2 s je Antwort, damit 19 parallele Testfälle auch auf einem ausgelasteten Rechner stabil sind.
- **A-175 (neu) – Zuschnitt und Findings:** Inkrement 2 wurde in einem Zyklus erledigt. Kein Finding: Ich habe geprüft, ob der Filter der Config und der Name der Config-Gruppe auseinanderfallen können. Nach SF-085 passen beide zusammen.

Neu aus C-064 (P5 Inkrement 3):
- **A-176 (neu) – Zuschnitt:** Inkrement 3 in einem Zyklus. T7-SET-10 braucht einen Neustart der App und kommt mit Inkrement 6.
- **A-177 (neu) – Was verglichen wird:** Konfiguration, PublicationList, ReferenceDesignation und SubscriptionList, jeweils vor und nach den Schreib-Tests. Die Messdaten nicht, weil sie sich von selbst ändern. Auch nicht, was die App auf der Festplatte speichert.
- **A-178 (neu) – Nur gemeldet:** Das Ergebnis des Vergleichs sagt etwas über D7, nicht über die App. Es steht im Bericht und erzeugt eine Warnung, ist aber kein „durchgefallen“.
- **A-179 (neu) – Zurückschreiben:** D7 schreibt, was die App geschickt hat. Die Konfiguration geht mit verdecktem Passwort zurück; Daten nur dort, wo die App sagt, dass sie änderbar sind.
- **A-180 (neu) – Grenzen:** D7 löscht nur eigene Abonnements, eine ReferenceDesignation nur mit bekanntem Stand. Feste Einträge bekommen nur eine Änderung, die abgelehnt werden muss (sonst rückgängig). Umgestellte Modi werden zurückgestellt.
- **A-181 (neu) – „Nichts mehr geschickt“:** heißt: nichts bis zur Status-Meldung plus kurze Nachfrist.
- **A-182 (neu) – T7-SET-03:** läuft zuletzt und nutzt die Reaktionen der anderen Schreib-Tests; nur Fehlendes wird zusätzlich angestoßen.
- **A-183 (neu) – Änderbar gelistet:** wie im Katalog beschrieben: angenommen → bestanden, „nicht schreibbar/unterstützt“ → durchgefallen, sonst übersprungen.
- **A-184 (neu) – Anstöße:** unbekannte Einträge heißen `d7…`; SET-05 schickt absichtlich eine ungültige Zahl.
- **A-185 (neu) – Status-Meldung erkennen:** Ereignis der Kategorie Status mit der Nummer der Frage als `ReplyTo`; Absender wie in SF-108.
- **A-186 (neu) – Simulierte App:** beschreibbar, merkt sich Änderungen, eigene Kennung je Status-Art; neue Inhalte (Passwort, Log-Stufen, änderbare Einträge).
- **A-187 (neu) – Selbsttest prüft das Aufräumen:** Nach jedem Testfall muss die simulierte App wieder im Ausgangszustand sein.
- **A-188 (neu) – Verdecktes Passwort auf Level 1:** wird nicht zurückgeschrieben, weil die App dort das verdeckte Passwort nicht richtig behandeln muss.
- **A-189 (neu) – ReferenceDesignation löschen:** nur, wenn das Zurückschreiben vorher geklappt hat (neuer erlaubter Testfall `set-rejected`).
- **A-190 (neu) – Abbruch:** Bricht ein Lauf in den Schreib-Tests ab, meldet D7 „nicht geprüft, App evtl. verändert“ und warnt.

Neu aus C-065 (P5 Inkrement 4):
- **A-191 (neu) – Zuschnitt:** T7-MON-14 prüft über den ganzen Lauf, ob die App so oft und so genau schickt, wie in der PublicationList steht. Dafür braucht es ein eigenes Zeitmodell; der Test kommt in C-066. Der Rest von Inkrement 4 in einem Zyklus.
- **A-192 (neu) – Die Anfrage:** D7 fragt auf `Call/NewDataSetWriterId` und nennt als Antwortadresse das passende `Reply`-Topic. Antworten erkennt D7 an seiner eigenen Kennung und der Anfrage-Nummer, egal auf welchem Antwort-Topic der App. So ist eine Antwort an der falschen Stelle ein Befund und nicht bloß „keine Antwort“.
- **A-193 (neu) – Im Schreib-Fenster:** Die Aktions-Tests laufen nach den Schreib-Tests, vor dem Vergleich vorher/nachher. D7 benutzt die Nummer nie, also gibt es nichts aufzuräumen.
- **A-194 (neu) – Muss die App antworten?** Ja, wenn ihr Profil den Dienst nennt oder das Level ihn vorschreibt (REQ-CMP-040). Sonst wird „keine Antwort“ übersprungen.
- **A-195 (neu) – Grenzen:** Ob eine vergebene Nummer innerhalb der Reservierung tatsächlich benutzt wird, sieht D7 nicht, weil es keinen Datensatz anlegt (bleibt Handprüfung). T7-ACT-03 schickt absichtlich zwei ungültige Anfragen und bewertet das Schweigen nur, wenn die App eine gültige Anfrage daneben beantwortet.
- **A-196 (neu) – „Aufbau“ eines Datensatzes:** die sortierten Feldnamen. Die Datentypen der Felder werden nicht verglichen.
- **A-197 (neu) – Nichts zu sehen:** Hat eine App weder Profil noch Daten/Ereignisse geschickt, wird T7-MON-13 einfach übersprungen, nicht „nicht entscheidbar“. Sonst wäre der geprüfte Testfall `lifecycle-pass` plötzlich nicht mehr „bestanden“.
- **A-198 (neu) – Liste neu geschickt:** zählt, wenn die Liste nach der Bestätigung innerhalb von 5 s kommt, ohne `ReplyTo` oder mit dem `ReplyTo` der Änderung. Nach dem Löschen aus der SubscriptionList wird das Ausbleiben nicht bewertet: Ist kein Eintrag mehr übrig, wird nichts geschickt, und das sieht man von außen nicht.
- **A-199 (neu) – Änderungszähler:** darf nie sinken und nicht zwischen -1 (kein Zähler) und einer Zahl wechseln. Nach einer bestätigten Änderung eines Konfig-Werts muss er steigen, aber nicht genau um 1 (es können mehrere Änderungen dazwischen liegen). Passwörter und ReferenceDesignation zählen nicht mit (A-113 bleibt). D7 selbst ändert keine Werte wirklich; im `test`-Lauf wird deshalb nur geprüft, dass der Zähler nicht sinkt.
- **A-200 (neu) – Pflichtfelder:** geprüft wird der Absender der Änderung, gegen die zuletzt geschickte Konfiguration der Ziel-App. Im `test`-Lauf zählt nur die App selbst, also nur ihre eigenen Änderungen an anderen Apps.
- **A-201 (neu) – Kleinkram:** Konfig-Gruppe ohne Leerzeichen in drei Aufzeichnungen; die Zwischenmeldung der simulierten App enthält jetzt Nummer und `Ttl`; `answers-unknown` erwartet auch den Befund von T7-MON-01; der Bericht listet die Aktions-Anfragen mit.

Neu aus C-066 (P5 Inkrement 5):
- **A-202 (neu) – Zuschnitt:** T7-MON-14 kommt erst in C-067; die Zwischenstation hat den Zyklus gefüllt.
- **A-203 (neu) – Eigene Zwischenstation:** auf der Go-Standardbibliothek, ohne neue Bibliothek. Die MQTT-Bibliothek hat keine Server-Seite und liest Pakete zu großzügig; D7 muss sie aber genau sehen. Weitergereicht wird jedes Paket unverändert.
- **A-204 (neu) – Zertifikate:** Der Tester gibt die Zertifizierungsstelle an, der die App vertraut (`--proxy-ca`, `--proxy-ca-key`; Testmaterial nur in einem Temp-Verzeichnis). Daraus stellt D7 die Zertifikate für die Zwischenstation und für den falschen Rechnernamen aus; dazu eine eigene fremde Stelle. Richtung Broker nimmt D7 seine eigenen TLS-Einstellungen und die Anmeldung der App. Ein Client-Zertifikat der App sieht D7, kann es aber nicht weiterreichen.
- **A-205 (neu) – Warten vor der ersten Frage:** D7 wartet auf die Birth und das erste bestätigte Abonnement der App (höchstens `--timeout` nach der Anmeldung). Eine App darf nach ihrer Birth abonnieren (das SDK macht es so).
- **A-206 (neu) – Reihenfolge:** Die Verbindungsprobe kommt zuletzt, nach allen anderen Tests: TLS 1.1, fremde Stelle, falscher Rechnername, harte Trennung, Warten auf das Neuverbinden (`--proxy-wait`, Standard 2 min). Kommt die App nicht so weit, ist das „nicht entscheidbar“.
- **A-207 (neu) – Client-Name und Einstellungen:** Ein anderes Keep-Alive als 60 s oder „Clean Session“ aus sind nur Hinweise; ein leerer oder wechselnder Client-Name ist ein Verstoß.
- **A-208 (neu) – „Neu erzeugter“ Wille (REQ-LIFE-016):** Die Zeit in der Nachrichten-Kennung des Willens liegt höchstens 5 s (`--tolerance`) neben der Anmeldung, und keine Kennung wiederholt sich. Eine Kennung ohne Zeit ist ein Verstoß.
- **A-209 (neu) – Birth:** Das Typenschild der App über sich selbst ohne `ReplyTo`; es muss die erste Nachricht jeder Verbindung sein und rechtzeitig kommen. Nach „ausgefallen“ muss vor der nächsten anderen Zustandsmeldung eine neue Birth kommen; fällt die App nie aus, wird das einfach übersprungen.
- **A-210 (neu) – Ohne Zwischenstation:** Verbindungs-Tests „nicht entscheidbar“, Exit 4 (außer mit `--allow-skip`). Eine gespeicherte Nachricht der App fällt trotzdem auf (T7-CON-04).
- **A-211 (neu) – Passive Prüfung:** Eine fehlende Birth nach dem Willen meldet T7-MON-03 erst, wenn die Verbindung oder die Beobachtung ohne Birth endet (eine späte Birth bewertet T7-LIFE-02). T7-MON-10 zählt eine Birth nach einer Erholung nicht als zweite.
- **A-212 (neu) – Simulierte App:** startet über die Zwischenstation unabhängig von D7, verbindet sich nach einer Trennung immer wieder neu, erzeugt den Willen je Verbindung mit QoS 0 (vorher 1); neue Variante `late-subscribe`.
- **A-213 (neu) – Selbsttest:** alle Testfälle laufen über die Zwischenstation; ein Eintrag darf seine Tests auswählen (`tests`); fehlt ein erwartetes Ergebnis, ist das ein Fehler; `none` wieder `draft`.
- **A-214 (neu) – Bericht:** neue Felder `attachment` „proxy“, `proxy_listen` und `connections` (jeder Verbindungsversuch mit Phase, TLS-Version, Anmeldung ohne Passwort, Antwort des Brokers, Nachrichten, Trennung). Die Version des Berichts bleibt 1 (nur ergänzt).

---

Neu aus C-067 (P5 Inkrement 6a):
- **A-215 (neu) – Zuschnitt:** Inkrement 6 ist geteilt. C-067: Neustart, T7-LIFE-04, T7-SET-10, T7-MON-14. C-068: die simulierte zweite App, `--announce` (Q-029 (a)) und T7-ADR-01 – eine zweite App mit eigenem Lebenszyklus ist neue Technik wie die Zwischenstation. C-069: `list`/`explain`, Schema des Berichts, Abschlusslauf von P5. Die ROADMAP-Tabelle ist angepasst.
- **A-216 (neu) – Neustart-Kommando:** `--restart-cmd`, oder `--stop-cmd` und `--start-cmd`; ausgeführt über die Shell (`sh -c`, unter Windows `cmd /C`). Ein Start im Hintergrund muss seine Ausgabe umleiten, sonst wartet D7 bis zu 10 s darauf. `--restart-wait` (Standard 2 min) begrenzt das Warten auf die App. D7 beendet selbst keine Programme oder Container.
- **A-217 (neu) – Wann der Neustart läuft:** im Schreib-Fenster, nach den anderen Schreib-Tests und vor dem Vergleich vorher/nachher. T7-SET-10 ändert, das Kommando läuft, D7 wartet auf die neue Birth (über die Zwischenstation auch auf die neue Verbindung und ihr erstes Abonnement) und eine Antwort auf „Zustand?“, dann wird bewertet und T7-SET-10 stellt den alten Wert wieder her. Der Vergleich vorher/nachher deckt damit auch den Neustart ab.
- **A-218 (neu) – Was als Abschiedsmeldung zählt:** die letzte Zustandsmeldung der App (keine Antwort) nach dem Start des Kommandos und vor der Abmeldung (über die Zwischenstation) bzw. vor der neuen Birth (direkt am Bus). Geprüft: ob sie kommt, ob sie auf dem Zustands-Topic der App kommt, ob sie „normal“ (`NORMAL_0`/0) ist und die Datensatz-Nummer aus der normalen Zustandsmeldung trägt. Nachrichten nach der Abschiedsmeldung sind nur ein Hinweis. „Nicht entscheidbar“: kein Kommando, die Verbindung endet nicht, keine Abmeldung, der letzte Wille statt Abmeldung, keine neue Birth.
- **A-219 (neu) – ReferenceDesignation:** D7 hängt „D7“ an Value und Local des ersten Teils der Kennzeichnung der App. Hat die App keine ReferenceDesignation oder lehnt sie die Änderung ab, wird T7-SET-10 einfach übersprungen: die Regel gilt nur, wenn die App das Ändern anbietet.
- **A-220 (neu) – Was T7-MON-14 prüft:** die Einträge der zuletzt gesehenen PublicationList der App (ohne Mode gilt „nur auf Anfrage“). Ändert jemand die Liste, gilt bis zur nächsten veröffentlichten Liste nichts; kurz (Toleranz) nach einer Änderung oder Verbindung wird nichts bewertet; Birth, Wille und Abschied nie. Geprüft: ungefragt nur, wenn der Mode es erlaubt, und nur über erlaubte Topic-Formen; der Abstand höchstens Interval plus Toleranz; bei Interval 0 nicht schon bei kleineren Änderungen als die Precision („sollte“); eine in einer Antwort sichtbare Änderung muss auch ungefragt kommen. Nicht geprüft: `OFF_0` (das macht T7-SET-08), Abstände bei Ereignissen, ob jede erlaubte Form auch benutzt wird, Antworten in einer anderen Sprache. Schickt eine App, ohne dass ihre Liste bekannt ist, ist das „nicht entscheidbar“ (`test` fragt die Liste ab). Kein Finding nötig.
- **A-221 (neu) – Selbsttest:** Der Neustart läuft bei allen Testfällen der simulierten App; `T7-LIFE-02/no-birth` und `/health-first` wählen nur T7-LIFE-01 … 03; `lifecycle-pass` bekommt die PublicationList von B (wieder `draft`); die Anfrage nach der PublicationList bekommt im Selbsttest ihre Klassen-Kennung.

Neu aus C-068 (P5 Inkrement 6b):
- **A-222 (neu) – Zuschnitt:** Inkrement 6b ist in C-068 ganz fertig. C-069 bleibt wie geplant (`list`/`explain`, Schema des Berichts, Abschlusslauf).
- **A-223 (neu) – `--announce`:** D7 ist dann eine einfache App (Level 1, `Utility`) unter seiner Tester-Kennung, über seine Hauptverbindung; der letzte Wille steht in seiner Anmeldung. Birth, „Ich bin da“-Meldung, Zustand alle 30 s, Abschiedsmeldung; beantwortet Fragen nach Typenschild, Zustand und Profil. Ohne Schalter bleibt D7 unsichtbar (Q-029 (a)).
- **A-224 (neu) – Zweite App:** `Utility/oi4.org/oec-check/Peer/<12 Zeichen Zufall>`, für jeden Lauf neu, mit eigener Verbindung und demselben Verhalten. Fragt man sie nach einem unbekannten Gerät, antwortet sie „nicht gefunden“ (Status Bad, REQ-CMP-047); eine Frage mit Filter bekommt eine leere Liste; andere Fragen ignoriert sie. Bricht ihre Verbindung ab, endet der Lauf mit einem Fehler.
- **A-225 (neu) – „Ich bin da“-Meldung:** nach dem Finden der App schickt jede angemeldete App ihre Zustandsmeldung noch einmal, damit eine später verbundene App sie bemerkt – ohne gespeicherte Nachrichten (die sind verboten, REQ-MQTT-018).
- **A-226 (neu) – T7-ADR-01:** Als Frage an die zweite App zählt eine Get/Set/Del/Call-Nachricht der App, deren Topic die Kennung der zweiten App enthält oder sie als Quelle nennt. Geprüft je Frage: Dienst-Typ, Kennung, Topic; je falschem Topic ein Befund; bewertet am Ende des Laufs (Fragen nach dem Neustart zählen mit). Ohne `--peer` „nicht entscheidbar“, mit zweiter App aber ohne Frage einfach übersprungen. Fragen an den angemeldeten D7 werden nur gezählt.
- **A-227 (neu) – Simulierte App und Selbsttest:** Die simulierte App fragt wie das SDK jede neue App nach ihrem Typenschild; ihre Abonnements dafür kommen zuletzt. 3 neue Fehler; neue Felder `peer` und `announce` je Eintrag; der Selbsttest prüft die angemeldeten Apps. `none` läuft mit beiden und ist wieder `draft` (R-6).
- **A-228 (neu) – Bericht:** neuer Abschnitt `announced` und eine Textzeile je angemeldeter App; die Version des Berichts bleibt 1 (nur ergänzt).

Neu aus C-069 (P5 Inkremente 7/8):
- **A-229 (neu) – Zuschnitt:** Inkremente 7 und 8 zusammen in C-069, wie geplant; 7 bringt keine neuen Tests oder Testfälle, 8 ist ein Lauf nach dem Rezept aus C-068.
- **A-230 (neu) – Wo das Schema liegt:** im obersten Go-Paket `conformance` des Moduls, zusammen mit dem Einlesen der Testfall-Listen. Nur von dort kann Go die Dateien unter `testdata/` einbetten, und nur so kann der Selbsttest gegen die simulierte App das Schema benutzen (sonst gäbe es einen Kreis von Abhängigkeiten).
- **A-231 (neu) – Berichts-Schema:** JSON Schema 2020-12, Version 1 für `monitor` und `test`; unbekannte Schlüssel sind verboten; je nach Art des Laufs sind bestimmte Schlüssel Pflicht (z. B. `test` braucht `app` und `tester`, die Zwischenstation `proxy_listen`). Der Bericht von `validate` (aus P2) ist nicht dabei. Geprüft werden die Berichte aus den Kommandozeilen-Tests, aus jeder abgespielten Aufzeichnung und aus jedem Testfall gegen die simulierte App. Ändert sich ein Schlüssel oder seine Bedeutung, gibt es eine neue Version.
- **A-232 (neu) – Bezeichnung der Fragen im Bericht:** jede Frage von D7 trägt die Test-Kennung oder eine Phase: `discovery` (Finden der App), `state` (Stand vorher/nachher), `restart` (nach dem Neustart). Die erste Frage nach dem Profil hatte keine Bezeichnung – jetzt `discovery`.
- **A-233 (neu) – `list`/`explain`/`report-schema`:** nur aus den eingebauten Daten; `explain` zeigt die Testfälle mit dem Ergebnis, das in ihrer Liste steht (ob es wirklich so kommt, prüft der Selbsttest); gute Testfälle erscheinen als „besteht oder passt nicht“. Falsche Eingaben enden mit Exit 2. `version` nennt jetzt auch Katalog-Stand und Berichts-Version.
- **A-234 (neu) – Abdeckungsbericht in D7:** Geplant war er im `spectool`. Die Werkzeuge in `oec/spec` dürfen aber nichts aus `conformance/` benutzen (ADR-0018), also erzeugt ihn D7 selbst. Zusätzlich prüft das Prüfskript jetzt: jeder gebaute Test hat einen schlechten (durchgefallen, Warnung oder Hinweis) und einen guten Testfall.
- **A-235 (neu) – Zählfehler:** 55 Tests (19 mithören, 36 aktiv). In STATUS, ROADMAP, README und DESIGN korrigiert; alte Logs bleiben, wie sie sind.
- **A-236 (neu) – Abschluss-Bedingung 2:** „jeder schlechte Testfall fällt durch“ heißt: er ergibt das Ergebnis, das zur Strenge der Regel passt. Bei „sollte“-Regeln (T7-GET-09, T7-SET-12) ist das eine Warnung, bei der Vermutung T7-MON-10 (dass eine Kennung eindeutig ist, kann man durch Mithören nicht beweisen) ein Hinweis. Alle anderen Tests haben einen Testfall mit „durchgefallen“.
- **A-237 (neu) – CI und Container-Image:** kommen mit P9. Festgehalten: Prüfskript auf dem Mac (arm64) und erfolgreiche Builds für Windows, Linux und macOS je mit amd64 und arm64 (je etwa 14–16 MB).
- **A-238 (neu) – Sonst:** keine neue Bibliothek (das Schema prüft der Validator, den `validate` schon hat), keine Änderung am JSON Schema (D1), SDK und Demo unverändert, kein Finding.

Neu aus C-070 (P5b Plan und Inkrement 1):
- **A-239 (neu) – Bordmittel statt Bibliothek:** D8 liest Images und SBOMs mit der Standardbibliothek von Go. `go-containerregistry` hätte viel Registry- und Anmelde-Code mitgebracht, den D8 nicht braucht.
- **A-240 (neu) – Eingabe:** ein OCI-Ordner oder ein OCI-Archiv. Eine Registry-Adresse, ein altes Docker-Archiv oder ein gepacktes Archiv lehnt D8 mit einem Hinweis ab (Exit 2): erst mit `docker save` (ab Docker 25), `skopeo` oder `podman save` holen.
- **A-241 (neu) – Lese-Regeln:** das Docker-Format „Schema 2“ wird gelesen, das uralte „Schema 1“ nicht; Nachweis-Manifeste (Attestations) zählen nicht als Image; mit zstd gepackte Schichten kann D8 nicht lesen → kein Urteil (Exit 4).
- **A-242 (neu) – Architekturen:** erlaubt sind linux auf amd64, arm64 und arm/v7 (RIG 5.1). Keine davon → durchgefallen; nur eine → Warnung.
- **A-243 (neu) – Prüfsummen:** eine falsche Prüfsumme lässt T8-IMG-01 durchfallen; ein anderes Verfahren als SHA-256 gibt nur eine Warnung (T8-IMG-04); Signaturen prüft D8 nicht (keine Regel).
- **A-244 (neu) – Bericht:** eigenes Schema `infra-report.schema.json`, Version 1, darf bis zum Abschluss von P5b nur wachsen, dann eingefroren. Gemeinsame Bausteine mit D7; die Berichte von D7 sind unverändert.
- **A-245 (neu) – SBOM-Regeln:** wie im Vorschlag zu SF-114; Hinweise (keine Fehler) bei einem Volume über `/opt/oi4`, Paketen ohne Lizenzangabe und Verweisen auf fremde SBOM-Dokumente.
- **A-246 (neu) – Test-Images:** ein Test erzeugt sie immer gleich und vergleicht sie Byte für Byte; jeder Test hat ein gutes und ein schlechtes Image; zusammen etwa 260 KB.
- **A-247 (neu) – Plan:** sechs Stücke C-070 … C-075, Abschlussbedingungen wie oben.
- **A-248 (neu) – Entwurf nachgezogen:** DESIGN §9 korrigiert; `list`/`explain` und der Abdeckungsbericht für D8 kommen mit dem letzten Stück, in einer eigenen Datei, damit der Bericht von D7 gleich bleibt.

Neu aus dem C-070-Review (nach deinem D-1):
- **A-249 (neu, ersetzt A-245) – SBOM-Regeln:**
  - Lesbar sind SPDX 2 und 3 sowie CycloneDX 1; eine andere Version ist kein Fehler, sondern „kein sicheres Urteil“ (Exit 4).
  - Eine Datei, die gar keine SBOM ihres Formats ist, fällt durch.
  - Komponenten sind die Pakete; das Hauptprodukt einer CycloneDX-SBOM (`metadata.component`) braucht Lizenztexte, aber keine Kennung.
  - Fehlender Name oder fehlende Version → durchgefallen; `NOASSERTION` oder keine Lizenzangabe → durchgefallen, `NONE` ist ok; keine Package-URL/CPE → Warnung.
  - Jede Lizenz, `LicenseRef-`, Ausnahme und `AdditionRef-` braucht einen Text.
  - Lizenz-Kennungen prüft D8 nur auf ihre Schreibweise, nicht gegen die SPDX-Liste (die ändert sich mit der Zeit).
  - Hinweise statt Fehler: Verweise auf andere SBOM-Dokumente, ein Volume über `/opt/oi4`, eine leere Komponentenliste.
  - Keine neue Bibliothek.

---

### Was als Nächstes passiert (C-072)

1. **C-072:** das dritte Stück von D8, `oec-infra host`: prüft den Rechner (Ablage der Storages und ihre Rechte, `broker.json`, Plattform-Eigenschaften). Selbsttest gegen erzeugte Beispiel-Rechner-Verzeichnisse, freiwillig gegen unsere Entwicklungsumgebung. Läuft in einer **neuen Sitzung**; der Start-Prompt steht in `prompt.md`.

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

## Was aus deinem letzten Review (C-069) geworden ist

Du hast geschrieben: „D-1 (a)“. Daraus wurde Nachtrag 1 zu C-069:
- **P5 ist abgeschlossen** (ROADMAP: Überschrift von P5 und ein Absatz „Decision“ nach der Bewertung der Abschlussbedingungen; STATUS).
- **Punkt 4** (CI auf Windows, macOS und Linux) und das **Container-Image von D7** wandern in die **CI von P9** (neuer Punkt bei P9).
- **Nächste Phase: P5b** (D8 `oec-infra`, das Prüf-Tool für Container und Plattform). Sie beginnt in C-070 mit einem neuen Agenten, der zuerst Plan und Zuschnitt macht. Der Abschlusslauf von D8 gegen die echten Demo- und Registry-Images kommt mit P6/P7.

Zur Erinnerung, worum es ging:

**Worum es geht:** Alle acht Inkremente von P5 (das Prüf-Tool D7 `oec-check`) sind gebaut. Die ROADMAP nennt vier Bedingungen für den Abschluss von P5. Ich habe sie einzeln geprüft (Belege im Log C-069):

| # | Bedingung | Stand |
|---|---|---|
| 1 | `test` besteht gegen die Beispiel-App `demo.rs` | **erfüllt**: Level 2, über die Zwischenstation, mit Neustart, zweiter App und angemeldetem D7: 49 bestanden, 0 durchgefallen, 6 übersprungen (passen nicht zur Demo), Exit 0, Stand danach wie vorher. Damit ist auch der offene Punkt 3 aus P4 erledigt (deine Entscheidung D-1 (a) aus C-061). |
| 2 | Absichtlich fehlerhafte Testfälle fallen durch; jeder Test hat einen guten und einen schlechten Testfall | **erfüllt**: 59 Testfälle gegen die simulierte App und 29 Aufzeichnungen, alle von dir geprüft; das Prüfskript kontrolliert jedes erwartete Ergebnis und seit C-069 auch „je Test ein guter und ein schlechter Testfall“ für alle 55 Tests. Bei drei Tests ist die Regel nur ein „sollte“ oder eine Vermutung (T7-GET-09, T7-SET-12, T7-MON-10): dort ergibt der schlechte Testfall eine **Warnung** bzw. einen **Hinweis**, kein „durchgefallen“ – so streng, wie die Regel ist (A-236). |
| 3 | Jeder Eintrag im Katalog ist gebaut oder als „nur von Hand prüfbar“ begründet | **erfüllt**: 55 von 55 Tests gebaut, 4 Regeln mit Begründung „von Hand“; keine Regel ohne Prüfung. |
| 4 | Läuft in einer CI auf Windows, macOS und Linux | **kommt mit P9** (wie beim SDK, deine Entscheidung D-1 (a) aus C-061): Das Prüfskript läuft hier auf dem Mac (arm64); gebaut für Windows, Linux und macOS je mit amd64 und arm64 – alles klappt. Auch das Container-Image von D7 gehört zur Veröffentlichung in P9 (A-237). |

**Möglichkeiten:**
- **(a)** P5 abschließen (Punkt 4 wandert in die CI von P9). Weiter mit **P5b** – D8 `oec-infra`, das Prüf-Tool für Container und Plattform.
- **(b)** wie (a), aber weiter mit **P4b** – das Micro-SDK in C für den ESP32 (D9).
- **(c)** wie (a), aber weiter mit **P6** – die Demos (Wetter in Rust, Logger in C). Deren Abschluss braucht aber auch D8; der bliebe dann offen.
- **(d)** P5 offen lassen, bis es eine CI gibt.

**Meine Empfehlung: (a).** Nach deiner Reihenfolge aus C-045 kommen P4b und P5b nach P4/P5. P5b zuerst, weil die Demos (P6) und die Registry (P7) am Ende auch D8 bestehen müssen. D8 nutzt dasselbe Go-Modul, denselben Katalog-Aufbau (`d8.yaml`, 17 Tests geplant), dieselbe Bewertung und denselben Bericht wie D7 – das ist jetzt alles frisch. P4b hängt von beidem nicht ab und kann danach kommen. Die CI gehört zur Veröffentlichung (deine Entscheidung) und damit nach P9.

**Gut zu wissen zu (a):** Der Abschluss von P5b verlangt, dass „unsere Demo- und Registry-Images Level 3 bestehen“. Diese Container-Images gibt es noch nicht (sie entstehen in P6/P7). P5b kann D8 aber gegen eigens gebaute Images (gute und fehlerhafte) bauen und selbst testen; der Abschlusslauf gegen die echten Images käme dann mit P6/P7 – so wie der Lauf von D7 gegen die Demo auf P4 gewartet hat.

## Was aus dem Review C-061 geworden ist

Du hast geschrieben: „D-1 (a)“. Daraus wurde Nachtrag 1 zu C-061:
- **P4 ist abgeschlossen** (ROADMAP: Überschrift von P4 und die Bewertung der Abschlussbedingungen; STATUS).
- **Punkt 3** (D7 `test` gegen die Beispiel-App `demo.rs`) gehört jetzt zum Abschluss von **P5**; dort stand er schon als Bedingung.
- **Punkt 5, Teil Plattformen** (Windows mit MSVC, echte Testläufe auf amd64 und Windows) wandert in die **CI von P9**.
- **Nächste Phase: P5** (D7 `test`, dazu Proxy und `list`/`explain`). Sie beginnt in C-062 mit einem neuen Agenten.

Zur Erinnerung, worum es ging:

#### D-1 – Ist P4 fertig, und womit geht es weiter? (entschieden: (a))

**Worum es geht:** Alle zehn Inkremente von P4 (das Rust-SDK und seine C-Schnittstelle) sind gebaut. Die ROADMAP nennt sechs Bedingungen für den Abschluss von P4. Ich habe sie einzeln geprüft (Belege im Log C-061):

| # | Bedingung | Stand |
|---|---|---|
| 1 | Alles, was die API-Entscheidungen (ADR-0020/0021) versprechen, gibt es im SDK und in der C-Schnittstelle; Skizze gelöscht | **erfüllt** (eine Funktion ist bewusst ersetzt, siehe A-148) |
| 2 | Alle gemeinsamen Testvektoren laufen im SDK | **erfüllt** |
| 3 | Tests gegen den echten TLS-Broker | **fürs SDK erfüllt** (14 grün, die C-Beispiele einmal gegen Mosquitto); **offen: D7 `test`** gegen die Beispiel-App – D7 `test` ist Arbeit von P5 |
| 4 | Die C-Beispiele laufen gegen `oec-ffi`, auch mit AddressSanitizer | **erfüllt** (im Prüfskript) |
| 5 | Prüfskript grün; das SDK baut auf Linux, macOS und Windows (amd64 + arm64), ARMv7 so gut es geht | **Prüfskript grün; Plattformen teilweise:** getestet auf macOS arm64 und Linux arm64, gebaut (nicht getestet) für Linux amd64, Windows amd64 (GNU-Werkzeuge) und ARMv7. **Offen:** Windows mit dem Microsoft-Compiler (MSVC) und echte Testläufe auf amd64 und Windows – dafür braucht es eine CI mit solchen Rechnern (P9) |
| 6 | Die Beispiel-Apps sind beschrieben und werden im Prüfskript gebaut | **erfüllt** |

**Möglichkeiten:**
- **(a)** P4 abschließen; Punkt 3 (D7 `test`) wandert in den Abschluss von P5, Punkt 5 (Plattformen) in die CI von P9. Weiter mit **P5** (D7 `test`: D7 prüft eine laufende App aktiv, dazu Proxy, `list`/`explain`).
- **(b)** wie (a), aber weiter mit **P6** (Logger-Demo in C auf `oec-ffi`, Wetter-Demo in Rust); P5 danach.
- **(c)** P4 offen lassen, bis 3 und 5 erfüllt sind (dafür braucht es aber erst P5 und eine CI).

**Meine Empfehlung: (a).** P5 macht den letzten offenen Punkt von P4 grün, und die Demos aus P6 müssen am Ende sowieso D7 bestehen – dann ist D7 schon fertig. Die CI für Windows/amd64 hängt an der Veröffentlichung (deine Entscheidung) und gehört nach P9. P4b (Micro-SDK) bleibt nach deiner Reihenfolge aus C-045 hinter P5.

Deine Antwort: „D-1 (a)“.

---

## Was aus dem Review C-060 geworden ist

Du hast geschrieben: „Ich habe alle Punkte reviewed und akzeptiere deine Vorschläge.“ Daraus wurde Nachtrag 1 zu C-060:
- **Guideline `eb7889c`, zwei Sätze:**
  - D-1 (SF-112, MG 8.4.1): „Das `Set`, das den `DataSet` anlegt, trägt diese `DataSetWriterId`; für einen `DataSet`, den der Publisher schon in seiner `PublicationList` führt, wird die dort geführte `DataSetWriterId` verwendet und keine neue gebraucht.“
  - D-2 (SF-113, MG 9.2.1): „Ist oder vertritt die Anwendung die `<Source>` nicht, antwortet sie mit der abschließenden `ActionResponse` mit dem `Status` `BadNotFound` und leerem `Payload`.“
- **Register:** SF-112 und SF-113 sind „resolved“ (`4fac7d2`). Kein offenes Finding; das nächste heißt SF-114.
- **Neu-Baseline auf `eb7889c`:** 1406 Guideline-Stellen (vorher 1405; der Satz aus D-2 ist neu, der aus D-1 enthält kein Schlüsselwort wie „muss“ und zählt deshalb nicht als eigene Stelle), 442 Regeln, 0 Lücken. Schema bleibt `2.0.1-dev`, nur die Baseline ist neu. D7/D8-Kataloge auf `eb7889c`.
- **Regeln:** REQ-ACT-017 und -023 bekommen Notizen, REQ-ACT-024 ist neu → R-1, von dir akzeptiert (Nachtrag 2) → `reviewed`.
- **R-1 aus C-059 (REQ-RES-SUBLIST-004, -010, -016):** akzeptiert → `reviewed`.
- **SDK:** keine Verhaltensänderung; es hat beides schon so umgesetzt.

Zur Erinnerung, worum es ging:

#### D-1 – SF-112: Wofür ist eine per Aktion geholte Datensatz-Nummer da? (entschieden: wie vorgeschlagen)

**Das Problem:** Die Guideline sagt nur, die Nummer sei „für das kommende `Set`“. Offen bleibt: (a) Was macht die App mit einem `Set`, das diese Nummer trägt? (b) Welche Nummer nimmt das `Set`, wenn die App für dieses Gerät schon eine Nummer anzeigt? Das ist häufig, denn nach MG 9.1.8 zeigt eine App die ReferenceDesignation jedes Geräts mit Nummer an, auch wenn sie leer ist. (c) Darf eine Nummer, die nach Ablauf der 60 Sekunden ungenutzt blieb, wieder vergeben werden?

**Mein Vorschlag:**
1. Ein `Set` trägt die Nummer, die die App für den Datensatz anzeigt. Nur wenn sie noch keine anzeigt, trägt es eine per Aktion geholte Nummer; der neue Datensatz behält sie.
2. Zeigt die App schon eine Nummer an, gilt diese; die geholte verfällt einfach.
3. Eine ungenutzte geholte Nummer wird nicht wieder vergeben, solange es noch nie benutzte gibt.

Dazu ein Satz in MG 8.4.1: „Das `Set`, das den `DataSet` anlegt, trägt diese `DataSetWriterId`; für einen `DataSet`, den der Publisher schon in seiner `PublicationList` führt, wird die dort geführte `DataSetWriterId` verwendet und keine neue gebraucht.“

**Auswirkung:** Keine aufs SDK, es macht es schon so. Die Regel REQ-ACT-023 bekommt eine Notiz und geht danach als R-x zu dir.

**Was du tun musst:** „D-1 wie vorgeschlagen“ oder deine Lesart.

#### D-2 – SF-113: Antwortet eine App, wenn nach einem unbekannten Gerät gefragt wird? (entschieden: wie vorgeschlagen)

**Das Problem:** Fragt jemand per `Get`/`Set`/`Del` nach einem Gerät, das die App nicht kennt, sagt die Guideline seit SF-099/SF-108: keine Daten, aber ein Status-Ereignis `BadNotFound`. Für die Aktion sagt sie nichts. Es gibt zwei Lesarten:
- **(a) Keine Antwort:** Die Frage „kann nicht zugeordnet werden“ (REQ-ACT-017). Der Fragende wartet dann bis zu seinem Zeitlimit.
- **(b) Eine Antwort mit Fehler:** Die Frage ist an die App adressiert und lesbar. Also verlangt REQ-ACT-015 genau eine abschließende Antwort, hier mit „Bad“.

**Mein Vorschlag:** (b). Die App antwortet abschließend mit `BadNotFound` und leerem Inhalt. „Kann nicht zugeordnet werden“ in REQ-ACT-017 meint die technische Adresse der Aktion (Nummern 8/1), nicht das Gerät. Dazu ein Satz in MG 9.2.1: „Ist oder vertritt die Anwendung die `<Source>` nicht, antwortet sie mit der abschließenden `ActionResponse` mit dem `Status` `BadNotFound` und leerem `Payload`.“

**Auswirkung:** Keine aufs SDK, es macht (b). Bei (a) ändern sich ein Zweig und zwei Tests. Die Regel REQ-ACT-017 bekommt eine Notiz und geht danach als R-x zu dir. D7 bleibt unverändert.

**Was du tun musst:** „D-2 wie vorgeschlagen“ oder deine Lesart.

#### R-1 – Drei Regeln zur SubscriptionList folgen dem neuen Text (akzeptiert)

**Warum:** Deine Entscheidungen haben MG 9.1.9 geändert (`57ebe91`). Drei Regeln, die du schon abgenommen hattest, folgen jetzt dem neuen Text und stehen deshalb wieder auf `draft`. Lesbar in `oec/spec/requirements/view/res-sublist.md`.

- **REQ-RES-SUBLIST-004** (die App): Neu ist der Satz: Eine App, die keine neuen Einträge anlegen kann, lehnt ein `Set` mit einem noch nicht abonnierten Topic mit `Bad` ab (z. B. `BadNotSupported`). (D-3)
- **REQ-RES-SUBLIST-010** (der Anfragende): „Ein noch nicht abonniertes Topic legt einen neuen Eintrag an“ gilt jetzt nur, **wenn die App das Anlegen unterstützt**; sonst wird das `Set` abgelehnt (Verweis auf -004). (D-3)
- **REQ-RES-SUBLIST-016** (die App): Nicht eingetragen werden die eigenen **Anfrage**-Abos (`Get`, `Del`) und die **kurzen Abos für Antworten**. Die `Pub`-Abos auf die eigene `AppId` (wie in B1.1.9) bleiben drin. Der Titel ist kürzer geworden (das Feld hat höchstens 100 Zeichen). (D-1, D-2)

**Auswirkung:** keine auf das SDK (es macht es schon so), das Schema oder die Testdaten. Beim künftigen D7-Test T7-SET-11 steht jetzt eine Notiz: Lehnt die App das Anlegen ab, ist das kein Fehler.

**Was du tun musst:** „R-1 akzeptiert“ oder Änderungswünsche.

#### R-1 – Drei Regeln zur Aktion folgen deinen Entscheidungen (akzeptiert)

**Warum:** Deine Entscheidungen D-1/D-2 haben MG 8.4.1 und MG 9.2.1 geändert (Guideline `eb7889c`). Zwei Regeln, die du schon abgenommen hattest, haben eine Notiz bekommen und stehen deshalb wieder auf `draft`; eine Regel ist neu. Lesbar in `oec/spec/requirements/view/act.md`.

- **REQ-ACT-017** (die antwortende App; „eine Anfrage, die nicht zugeordnet werden kann, wird nicht beantwortet“): Der Regeltext bleibt. Neu ist die Notiz: „Nicht zugeordnet“ meint die technische Adresse der Aktion (Nummern 8/1), nicht das Gerät im Topic. Eine Frage nach einem unbekannten Gerät wird mit `BadNotFound` beantwortet (REQ-ACT-024). (D-2)
- **REQ-ACT-023** (Fragender und antwortende App; „die geholte Nummer gilt `Ttl` Sekunden“): Der Regeltext bleibt. Neu sind der Verweis auf SF-112, der neue Satz aus MG 8.4.1 als Zitat und die Notiz: Das `Set`, das den Datensatz anlegt, trägt die geholte Nummer. Zeigt die App für den Datensatz schon eine Nummer in ihrer `PublicationList`, gilt diese, und die geholte verfällt nach `Ttl`. Eine ungenutzte geholte Nummer wird nicht wieder vergeben, solange es noch nie benutzte gibt. (D-1)
- **REQ-ACT-024 (neu)** (die antwortende App; MUST): „Ist oder vertritt die App die `<Source>` einer `NewDataSetWriterId`-Frage nicht, antwortet sie mit der abschließenden Antwort (`ActionState` 2) mit dem Status `BadNotFound` und leerem Inhalt.“ Ich habe dafür eine neue Regel angelegt, statt REQ-ACT-017 zu ändern: -017 ist ein Verbot („nicht antworten“), -024 ein Gebot („mit Fehler antworten“). Getrennt lässt sich beides sauberer prüfen. Dazu gibt es im D7-Katalog den geplanten Test T7-ACT-04. (D-2)

**Auswirkung:** keine aufs SDK (es macht beides schon so und nennt jetzt REQ-ACT-024), aufs Schema oder auf die Testdaten.

**Was du tun musst:** „R-1 akzeptiert“ oder Änderungswünsche.

---

## Was aus dem Review C-059 geworden ist

Du hast geschrieben: „D-1 … entspricht deiner Empfehlung“ und „D-2, D-3: Dein Vorschlag ist akzeptiert“. Daraus wurde Nachtrag 1 zu C-059:
- **Guideline `57ebe91` (MG 9.1.9), drei Sätze:**
  - D-1 (SF-109): Abos, die nur für die Dauer einer Anfrage bestehen (für die Antwort oder das Status-Event), werden nicht eingetragen.
  - D-2 (SF-110): Nur die Abos auf **Anfrage**-Topics der eigenen `AppId` (z. B. `Get`, `Del`) fallen heraus; B1.1.9 bleibt, wie es ist.
  - D-3 (SF-111): Eine App, die keine neuen Einträge anlegen kann, lehnt so ein `Set` mit `Bad` ab (z. B. `BadNotSupported`).
- **Register:** SF-109 … SF-111 sind „resolved“ (`7a2bda2`). Kein offenes Finding; das nächste heißt SF-112.
- **Neu-Baseline auf `57ebe91`:** 1405 Guideline-Stellen (vorher 1403; zwei neue Sätze), 441 Regeln, 0 Lücken. Schema bleibt `2.0.1-dev`, nur die Baseline ist neu. D7/D8-Kataloge auf `57ebe91`.
- **Regeln:** REQ-RES-SUBLIST-004, -010 und -016 folgen dem Text → R-1 (oben).
- **SDK:** keine Änderung; es hat alle drei Punkte schon so umgesetzt.

Zur Erinnerung, worum es ging:

#### D-1 – SF-109: Stehen die kurzen Abos für Antworten in der SubscriptionList? (entschieden: wie vorgeschlagen)

**Wo es auffiel:** beim Bau der SubscriptionList im SDK.

**Was die Guideline sagt:**
- MG 9.1.9: Die SubscriptionList zeigt „die tatsächlich vorhandenen/eingestellten Abos“ einer App; bei jedem Hinzufügen oder Entfernen muss sie neu geschickt werden.
- Wer eine Frage stellt (`Get`/`Set`/`Del`), abonniert kurz das Topic, auf dem die Antwort kommt, und meldet das Abo nach der Antwort wieder ab.

**Das Problem, an einem Beispiel:** Ein Leitsystem fragt jede Sekunde den Zustand einer Pumpe ab. Würden die kurzen Antwort-Abos mitgezählt, müsste es seine SubscriptionList bei jeder Frage zweimal neu schicken (Abo dazu, Abo weg). Einstellen könnte man diese Einträge auch nicht sinnvoll. Die Guideline sagt nicht, ob sie gemeint sind.

**Mein Vorschlag (empfohlen):** Nicht eintragen. Ein Satz in MG 9.1.9: „Abos, die nur für die Dauer einer Anfrage bestehen (um ihre Antwort oder ihr Status-Event zu empfangen), werden nicht eingetragen.“ Der englische Text steht im Register.

**Andere Möglichkeit:** (b) für die Dauer der Anfrage eintragen (als nicht einstellbar).

**Was passiert, wenn du „wie vorgeschlagen“ sagst:**
- Ich ergänze MG 9.1.9 und baseline neu.
- Ein Hinweis in den REQs zur SubscriptionList folgt dem Text und kommt als `draft` noch einmal zu dir.
- Das SDK macht es schon so; D7 ändert sich nicht.

#### D-2 – SF-110: Der Satz „andere Abos auf Topics der eigenen AppId werden nicht eingetragen“ widerspricht dem Beispiel B1.1.9 (entschieden: wie vorgeschlagen)

**Wo es auffiel:** beim Einbau deiner Entscheidung SF-107 ins SDK.

**Was die Guideline sagt:**
- MG 9.1.9 (Text aus SF-107): Ein beschreibbarer Datensatz steht mit dem Abo auf sein `Set`-Topic in der Liste. „Andere Abos auf Topics der eigenen `AppId` werden nicht eingetragen.“
- Das Beispiel B1.1.9 (unser Vektor V-PAY-072) zeigt aber genau so ein Abo: Die App abonniert ihre **eigenen** `Pub/Data`-Daten (`…/<FDS>/Pub/Data/<FDS>/oee`) und trägt es ein.

**Das Problem:** Wörtlich gelesen verbietet der Satz das Beispiel. Gemeint war bei SF-107 (C-055, D-1): Die eigenen *Anfrage*-Abos (`Get/…`, `Del/…`) werden nicht eingetragen – die beschreibt schon das Profile. Der Satz ist aber allgemeiner geworden.

**Mein Vorschlag (empfohlen):** Nur die eigenen Anfrage-Abos fallen heraus; `Pub`-Abos werden immer eingetragen, egal von welcher `AppId`. Neuer Wortlaut: „Andere Abos der App auf Anfrage-Topics ihrer eigenen `AppId` (z. B. `Get` oder `Del`) werden nicht eingetragen.“ B1.1.9 bleibt, wie es ist.

**Andere Möglichkeit:** (b) der Satz bleibt, und das Beispiel B1.1.9 und der Vektor V-PAY-072 bekommen ein Topic einer fremden App (dann kommen Beispiel und Vektor noch einmal zu dir).

**Was passiert, wenn du „wie vorgeschlagen“ sagst:**
- Ich ändere den Satz in MG 9.1.9 und baseline neu.
- REQ-RES-SUBLIST-016 folgt dem Text und kommt als `draft` noch einmal zu dir.
- Das SDK macht es schon so; D7 ändert sich nicht.

#### D-3 – SF-111: Muss eine App bei `Set SubscriptionList` ein neues Abo anlegen? (entschieden: wie vorgeschlagen)

**Wo es auffiel:** beim Bau von `Set SubscriptionList` im SDK.

**Was die Guideline sagt:**
- MG 9.1.9: „Hat die `Source` den `TopicPath` noch nicht abonniert, legt das `Set` einen neuen Eintrag an“ (Überschrift „neuen Eintrag anlegen oder Verhalten ändern“).
- MG 8.3.9: Wie weit man Abos einstellen kann (anlegen / löschen / gar nicht), hängt „von der Implementierung und/oder technischen Erfordernissen“ ab.

**Das Problem, an einem Beispiel:** Ein Programm schickt einer Wetter-App ein `Set` mit dem Topic der Pumpen-Daten einer anderen App. Die Wetter-App kann mit Pumpen-Daten nichts anfangen. Muss sie das Abo trotzdem anlegen? Und welche Antwort bekommt der Anfragende, wenn sie es nicht tut? Das SDK als Bibliothek kann ein fremdes Abo der App nicht sinnvoll unterschieben.

**Mein Vorschlag (empfohlen):** Anlegen ist freiwillig („hängt von der Implementierung ab“). Wer es nicht unterstützt, lehnt das `Set` mit einem Status-Event `Bad` ab (z. B. `BadNotSupported`). Ein Satz dazu in MG 9.1.9; der englische Text steht im Register.

**Andere Möglichkeit:** (b) Anlegen ist Pflicht für jede App. Dann braucht das SDK einen Handler, mit dem die App ein neues Topic annimmt oder ablehnt (eine kleine API-Ergänzung in 8c).

**Was passiert, wenn du „wie vorgeschlagen“ sagst:**
- Ich ergänze MG 9.1.9 und baseline neu.
- Ein Hinweis in REQ-RES-SUBLIST-010 („legt einen neuen Eintrag an“) folgt dem Text und kommt als `draft` noch einmal zu dir.
- Das SDK antwortet schon mit `BadNotSupported`; D7 ändert sich nicht.

---

## Was aus dem Review C-056 geworden ist

Du hast geschrieben: „D-1 und D-2 sind reviewed und akzeptiert“. Daraus wurde Nachtrag 1 zu C-056:
- **D-1 (SF-107):** In MG 9.1.9 steht jetzt: Ein Data- oder Config-Datensatz, den man mit `Set` ändern kann, steht mit dem Abo seines `Set`-Topics in der SubscriptionList (`Config` `NONE_0`); andere Abos auf Topics der eigenen `AppId` stehen nicht drin. In B1.1.3/B1.1.4 heißt es jetzt nur noch: „Check the `SubscriptionList` to find out about accessibility.“ (`580f1c6`). Das SDK baut das mit der SubscriptionList (Inkrement 8) ein; die neue Regel REQ-RES-SUBLIST-016 ist Teil von R-1.
- **D-2 (SF-108):** In MG 9.1 steht jetzt: Das Event zu einem Gerät, das die App nicht vertritt, geht unter der `AppId` raus; der Anfragende findet es über `ReplyTo`; dasselbe gilt für `Set` und `Del` (`2e32a6b`). Das SDK macht es schon so (A-109); REQ-CMP-047/043 sind Teil von R-1.
- **Register:** SF-107 und SF-108 sind „resolved“ (`d2dc9ca`). Kein offenes Finding; das nächste heißt SF-109.
- **Neu-Baseline auf `2e32a6b`:** 1403 Guideline-Stellen (vorher 1402), 441 Regeln, 0 Lücken. Schema bleibt `2.0.1-dev`, nur die Baseline ist neu.
- **SDK:** Eine Anfrage zu einem fremden Gerät beantwortet die App jetzt unter ihrem eigenen Namen (`Pub/Event/<AppId>/Status/Bad`). Die vorübergehenden Nummern für fremde Geräte sind weg (A-109). D7 hört jetzt mit: Test T7-MON-04 besteht, obwohl ein fremdes Gerät angefragt wurde (in C-055 musste er noch passen).
- **Nebenbei:** Ein Interop-Test hatte manchmal sein erstes `Get` geschickt, bevor der Broker das Abo der App kannte. Das war ein Fehler im Test, nicht im SDK. Jetzt wartet der Test auf die erste Config-Nachricht. Danach: zweimal 12 grün.

Danach hast du geschrieben: „R-1 reviewed“. Daraus wurde Nachtrag 2 zu C-056:
- **R-1:** REQ-CMP-043, REQ-CMP-047 und REQ-RES-SUBLIST-016 stehen auf `reviewed` (`ee9f0e9`). Es gibt keinen Entwurf mehr.

Zur Erinnerung, worum es ging:

#### D-1 – SF-107: Woran sieht man, dass ein Datensatz von außen beschreibbar ist? (entschieden: wie vorgeschlagen)

**Wo es auffiel:** beim Bau von `Set Data` im SDK.

**Was die Guideline sagt:**
- Anhang B1.1.3 und B1.1.4 (nur informativ): „Nicht jeder Datensatz ist über den Bus beschreibbar. Prüfe `SubscriptionList` und/oder `PublicationList`, um das herauszufinden.“
- Die PublicationList hat je Eintrag ein Feld `Config`. Es sagt aber nur, welche **Sende-Einstellungen** (Modus, Intervall, Totband) man ändern darf – nicht, ob man die **Werte** setzen darf.
- Die SubscriptionList zeigt laut MG 9.1.9 „die tatsächlichen Abonnements“. Die Beispiele dort abonnieren aber nur Daten **anderer** Apps.

**Das Problem, an einem Beispiel:** Ein Leitsystem will den Sollwert einer Pumpe setzen. Vorher möchte es wissen, ob der Datensatz „Sollwert“ überhaupt beschreibbar ist. Keine Regel der Guideline sagt, wo das steht. Heute bleibt nur: einfach schicken und auf `BadNotWritable` warten.

**Mein Vorschlag (so entschieden):** Die **SubscriptionList** zeigt es. Eine App, die `Set Data` für einen Datensatz annimmt, trägt ihr eigenes Abo auf das passende `Set`-Topic dort ein, z. B. `Oi4v2/…/<AppId>/Set/Data/<Gerät>/sollwert`. Andere eigene Abos (z. B. für `Get`) werden nicht eingetragen, die beschreibt schon das Profile. Das Feld `Config` der PublicationList behält seine Bedeutung. Der englische Textvorschlag für MG 9.1.9 und B1.1.3/B1.1.4 steht im Register.

**Andere Möglichkeiten:** (b) die PublicationList zeigt es (ein neues Merkmal; das ändert die Bedeutung von `Config` und das Schema); (c) keine Liste zeigt es, die Guideline sagt „schick das `Set`, `BadNotWritable` sagt dir Bescheid“ (nur Textänderung).

**Was passiert, wenn du „wie vorgeschlagen“ sagst:**
- Ich ändere MG 9.1.9 und B1.1.3/B1.1.4 in der Guideline und baseline neu.
- Die REQs zur SubscriptionList folgen dem Text und kommen als `draft` noch einmal zu dir.
- Das SDK trägt die Abos ein, sobald die SubscriptionList kommt (Inkrement 8). An 7a ändert sich nichts.

#### D-2 – SF-108: Unter welchem Absender antwortet die App auf eine Anfrage zu einem fremden Gerät? (entschieden: wie vorgeschlagen)

**Wo es auffiel:** beim einmaligen Mithören mit D7 gegen das SDK.

**Was die Guideline sagt:**
- MG 9.1: Fragt jemand nach einem Gerät, das die App weder ist noch vertritt, soll sie mit einem Status-Event `BadNotFound` antworten.
- Die Antwort geht an `Pub/Event/<Source>/Status/…`. **Welche** `Source` (Absender) gemeint ist, steht nicht da.
- MG 1.3: Für sich selbst und **jedes** vertretene Gerät muss eine App MAM (Typenschild) und Health senden.

**Das Problem, an einem Beispiel:** Jemand fragt die App „Pumpen-Connector“ nach dem Gerät „Pumpe 4711“, das sie gar nicht kennt. Wörtlich gelesen antwortet sie unter dem Absender „Pumpe 4711“. Wer mithört, sieht jetzt: Der Connector sendet etwas für „Pumpe 4711“ – also vertritt er sie wohl? Dann fehlen aber Typenschild und Health für dieses Gerät. Genau das hat D7 bemerkt (Test T7-MON-04 konnte nicht entscheiden). Außerdem könnte jeder die App dazu bringen, unter beliebigen fremden Namen zu senden.

**Mein Vorschlag (so entschieden):** Eine Status-Meldung trägt immer einen Absender, den die App ist oder vertritt. Für eine Anfrage zu einem fremden Gerät antwortet die App **unter ihrem eigenen Namen** (`AppId`): `Pub/Event/<AppId>/Status/Bad`. Der Anfragende findet die Antwort trotzdem, über `ReplyTo`. Alle anderen Anfragen behalten den Absender aus der Anfrage. Der englische Textvorschlag für MG 9.1 steht im Register.

**Andere Möglichkeiten:** (b) der Absender bleibt das fremde Gerät, und die Guideline sagt dazu, dass ein Absender, der nur in solchen Status-Antworten vorkommt, kein vertretenes Gerät ist (dann muss D7 das berücksichtigen; das SDK bleibt); (c) Text lassen, wie er ist.

**Was passiert, wenn du „wie vorgeschlagen“ sagst:**
- Ich ändere MG 9.1 in der Guideline und baseline neu.
- Die Hinweise in REQ-CMP-047/043 folgen dem Text und kommen als `draft` noch einmal zu dir.
- Das SDK ändere ich im nächsten Zyklus (kleine Änderung: Antwort unter der eigenen `AppId`). D7 bleibt, wie es ist.


#### R-1 – Drei Regeln nach SF-107/108 (akzeptiert, G-3.9)

**Warum:** Du hast SF-107 und SF-108 wie vorgeschlagen entschieden. Damit hat sich der Text von MG 9.1 und MG 9.1.9 geändert. REQ-CMP-047 und REQ-CMP-043 waren schon `reviewed`; ihre Aussage ändert sich (nicht nur eine verschobene Stelle), deshalb stehen sie wieder auf `draft`. REQ-RES-SUBLIST-016 ist neu.

**REQ-CMP-047** (Empfehlung, alle Level) – Anfrage (`Get`) zu einem fremden Gerät:
- Vorher: „Keine Antwort auf dem `Pub`-Topic; empfohlen ist ein Status-Event `BadNotFound`.“
- Jetzt zusätzlich (Englisch, wie in der Datei): „Because the entity does not represent the addressed asset, this Event is published with the Source of the entity itself (AppId): Pub/Event/<AppId>/Status/Bad; the requester finds it through the ReplyTo.“ Auf Deutsch: Das Event geht unter dem **eigenen Namen der App** raus; der Anfragende findet es über `ReplyTo`.
- Titel entsprechend ergänzt, ein zweites Zitat (der neue Satz aus MG 9.1), Verweis auf SF-108, in den Notizen der Grund (kein fremder Absender, der wie ein vertretenes Gerät ohne Typenschild und Health aussieht).

**REQ-CMP-043** (Pflicht ab Level 2, Empfehlung auf Level 1) – `Set`/`Del` auf etwas, das die App nicht hat:
- Jetzt zusätzlich: „If the Source of the Set or Del addresses an asset that the entity neither is nor represents, the Event is published with the Source of the entity itself (AppId).“ Also dieselbe Regel auch für `Set` und `Del`.
- Ein zweites Zitat („The same applies to the status Event of a Set or Del whose Source addresses such an asset.“), Verweis auf SF-108, eine Notiz.

**REQ-RES-SUBLIST-016** (neu, Pflicht, alle Level, Prüfung durch D7 im Test T7-SET-01):
- Titel: „Writable Data and Config DataSets are listed with their Set subscription“.
- Aussage: Eine App mit SubscriptionList trägt jeden Data- oder Config-Datensatz, den man mit `Set` ändern kann, mit ihrem Abo auf das passende `Set`-Topic ein (`Oi4v2/<ServiceType>/<AppId>/Set/<Resource>/<Source>/<Tag>`, `Config` `NONE_0`). Andere eigene Abos (z. B. für `Get` oder `Del`) trägt sie nicht ein.
- Zitate: die zwei neuen Sätze aus MG 9.1.9; Verweise MG 9.1.9, MG 8.3.9, SF-107. In den Notizen: die üblichen Regeln für Einträge (kein Platzhalter im Topic, `NONE_0` heißt „weder änderbar noch löschbar“), das Feld `Config` der PublicationList behält seine Bedeutung, bei Config ist der „Tag“ der Kontextname.

**Geprüft wurde:** Sagen die drei Regeln genau das, was MG 9.1 und MG 9.1.9 jetzt sagen?

**Ergebnis:** Du hast zugestimmt; alle drei stehen auf `reviewed` (`ee9f0e9`).

---

## Was aus dem Review davor (C-052) geworden ist

Du hast geschrieben: „Ich stimme D-1 und R-1 zu, bzw. habe das Review abgeschlossen“. Daraus wurde Nachtrag 1 zu C-052:
- **Guideline-Text:** In MG 7.5.1 (Hinweis zu `Health`) steht jetzt dein entschiedener Text (`9044929`): Health kommt beim Start und bei jeder Änderung; das Health der App selbst (`Source` = `AppId`) zusätzlich mindestens alle 60 Sekunden als Herzschlag; für untergeordnete Assets ist das freiwillig.
- **Register:** SF-106 ist „resolved“ (`b94e803`). Es gibt kein offenes Finding mehr; das nächste heißt SF-107.
- **Neu-Baseline auf `9044929`:** Die Zahl der Guideline-Stellen steigt von 1401 auf 1402, weil aus dem einen Satz zwei wurden. Beide gehören zu REQ-TOP-017; die stand deshalb wieder auf `draft` (R-2).
- **Schema:** Version `2.0.1-dev` mit Baseline `9044929`, am Inhalt ändert sich nichts.
- **D7 und SDK:** D7 (T7-MON-05) prüfte schon so, das SDK schickt den Herzschlag ohnehin für jedes Asset. Daran ändert sich nichts.
- **R-1 – Testdaten:** Die 18 Aufzeichnungen stehen jetzt in `index.yaml` auf `reviewed`. Neue Aufzeichnungen beginnen weiter als `draft`.
- **R-2 – REQ-TOP-017:** Du hast geschrieben „Geänderter R-2 – REQ-TOP-017 ist reviewed“. Die REQ steht jetzt auf `reviewed` (`8d7538b`, Nachtrag 2). Es gibt keinen Entwurf mehr.

Zur Erinnerung, was in R-2 stand:

**Warum:** Du hast SF-106 so entschieden, wie ich es vorgeschlagen hatte (D-1). Damit hat sich der Text von MG 7.5.1 geändert (`9044929`). REQ-TOP-017 ist die Regel, die genau diesen Satz abbildet. Sie war schon `reviewed`; weil sich ihre Aussage ändert (nicht nur eine Stelle verschoben), steht sie wieder auf `draft` und kommt zu dir.

**Vorher:** „Health wird beim Start gesendet, bei jeder Änderung und alle 60 Sekunden als Herzschlag.“

**Jetzt (Englisch, wie in der Datei):**
- Titel: „Health is published at start, on change, and for the application itself at least every 60 seconds“.
- Aussage: „Health is published once when the application is initialized and whenever the value of Health changes. The Health of the application itself (Source = AppId) is additionally published at least every 60 seconds as heartbeat; for subordinate assets a periodic publication is optional.“
- Auf Deutsch: Health kommt beim Start und bei jeder Änderung. Das Health **der App selbst** kommt zusätzlich **mindestens** alle 60 Sekunden; für die Assets darunter ist das freiwillig.
- Die zwei Zitate sind wörtlich die zwei neuen Sätze aus MG 7.5.1; Verweise: MG 7.5.1 und SF-106.
- In den Notizen steht: Jedes Health der App setzt die 60 Sekunden neu („mindestens“). Die Guideline nennt keine Toleranz; D7 nimmt 5 Sekunden. Das freiwillige Health der Assets prüft D7 nicht.

An D7 und am SDK ändert sich dadurch nichts, beide verhalten sich schon so.

Zur Erinnerung, worum es bei D-1 und R-1 ging:

#### D-1 – SF-106: Gilt der 60-Sekunden-Herzschlag für jedes Asset? (entschieden: wie vorgeschlagen)

**Wo es auffiel:** beim Bau der D7-Prüfung „Health alle 60 Sekunden“ (REQ-TOP-017).

**Was die Guideline sagt:**
- MG 7.5.1: Health wird beim Start gesendet, bei jeder Änderung und „alle 60 Sekunden als eine Art Herzschlag“. Dort steht „die Anwendung“.
- MG 1.3 und MG 9.1.2: Eine App veröffentlicht Health „für sich selbst und alle untergeordneten Assets“, also ein eigenes Health je Asset.

**Das Problem, an einem Beispiel:** Ein Connector bindet 500 Sensoren an.
- (a) Muss er für jeden der 500 Sensoren jede Minute ein Health schicken? Das wären 500 Nachrichten pro Minute, ohne neue Information. Ob die Verbindung lebt, zeigt schon der Will. Oder gilt der Herzschlag nur für das Health der App selbst?
- (b) Zählt ein Health, das wegen einer Änderung oder auf Anfrage kam, auch als Herzschlag (die 60 Sekunden beginnen dann neu)? Oder ist es ein fester Takt?
- (c) Wie genau ist „60 Sekunden“? Ein Prüftool braucht eine Toleranz, weil Netz und Rechner nie auf die Millisekunde genau sind.

**Mein Vorschlag (so entschieden):**
- (a) Der Herzschlag gilt nur für das Health der App selbst. Ein Asset sendet sein Health beim Start (bzw. wenn es auftaucht) und bei jeder Änderung; regelmäßig wiederholen darf es, muss es aber nicht. Vorgeschlagener Satz für MG 7.5.1 (Englisch, steht im Register): „… The `Health` of the application itself (`Source` = `AppId`) must additionally be published at least every 60 seconds as a kind of heartbeat; for subordinate assets, a periodic publication is optional.“
- (b) „Mindestens alle 60 Sekunden“: Jedes Health der App setzt die Uhr zurück.
- (c) Die Guideline nennt keine Toleranz; das Prüftool legt seine eigene fest und schreibt sie dazu (D7: 5 Sekunden, einstellbar mit `--tolerance`).

**Andere Möglichkeiten:** (d) Herzschlag für jedes Asset (dann prüft D7 jedes einzelne Health); (e) Text lassen, wie er ist (jedes Tool und jedes SDK rät weiter).

**Was passiert, wenn du „wie vorgeschlagen“ sagst:**
- Ich ändere MG 7.5.1 in der Guideline und baseline neu.
- REQ-TOP-017 folgt dem neuen Text und kommt als `draft` noch einmal zu dir.
- D7 prüft heute schon so. Das SDK schickt den Herzschlag ohnehin für jedes Asset und erfüllt damit beide Lesarten; dort ändert sich nichts.

#### R-1 – Aufgezeichnete Testdaten für `monitor` (akzeptiert, G-3.9)

**Was das ist:** 18 kleine Aufzeichnungen („Captures“) von Bus-Verkehr in `oec/conformance/testdata/captures/`. Jede Zeile ist eine MQTT-Nachricht mit Topic, Inhalt, Empfangszeit, QoS und Retain-Flag. In `index.yaml` steht zu jeder Datei, welches Ergebnis D7 liefern **muss** und welche REQ verletzt ist. Die Nachrichten sind aus reviewten Payload-Vektoren abgeleitet; ein Test schlägt Alarm, wenn eine Datei von ihrem Szenario abweicht.

**Zwei „gute“ Aufzeichnungen** (alles muss bestehen):
- `lifecycle-pass`: zwei Apps von der Birth bis zum Close, über gut 2 Minuten mit Herzschlag;
- `reconnect-pass`: eine App bricht ab, der Broker verschickt ihren Will, sie kommt mit einer neuen Birth zurück.

**16 „schlechte“ Aufzeichnungen**, je mit genau dem Fehler, den eine Prüfung finden soll. Beispiele:
- `will-then-publish`: nach dem Will sendet die App etwas anderes als eine Birth;
- `heartbeat-gap`: das Health der App bleibt länger als 65 Sekunden aus (hängt an D-1);
- `close-then-publish`: nach dem Close kommt noch eine Nachricht;
- `message-id-duplicate`: dieselbe MessageId zweimal;
- `retain-qos`: eine Nachricht mit Retain-Flag (Fehler) und mit QoS 1 (nur Warnung, weil QoS 0 nur ein SHOULD ist);
- `identity-conflict`: zwei Apps mit demselben Identification Link; das ist nur ein **Hinweis** (info), kein Fehler, weil D7 es nicht sicher wissen kann.

Die vollständige Liste mit den erwarteten REQ-Nummern steht in `REVIEW.md` bei R-1.

**Worauf du achten solltest:** Ist jedes Szenario wirklich ein Verstoß gegen die genannte REQ (und nur gegen diese)? Sind die beiden guten Aufzeichnungen wirklich in Ordnung? Besonders: wie D7 einen Will erkennt (A-57), die Herzschlag-Regel (hängt an D-1) und dass der Identitäts-Konflikt nur ein Hinweis ist.

**Wenn du zustimmst:** Die Einträge in `index.yaml` gehen auf `reviewed`.

---

## Was aus dem Review davor (C-051) geworden ist

Deine erste Antwort war „Option b, da sie präzise ist“. (b) war aber nur ein Teil meines Vorschlags. Darum habe ich nachgefragt und bis zu deiner Antwort nichts an der Guideline geändert. Deine Präzisierung lautete „Vorschlag komplett“, also (a) + (b) + (c). Daraus wurde Nachtrag 1 zu C-051:
- **Guideline-Text:** In RIG 4.1.1, Karte `MaxPacketSize`, stehen jetzt zwei neue Sätze (`59e3ae3`):
  - „Nutzlast + Topic-Länge + 4 dürfen 268435455 nicht übersteigen.“
  - „Der Broker nimmt jede Nutzlast bis `MaxPacketSize` an; begrenzt er das ganze Paket, muss die Grenze um 65544 Byte größer sein.“
  - Zu zu großen Nachrichten steht bewusst nichts (Teil (c)).
- **Register:** SF-105 ist „resolved“ (`d220c48`). Es gibt kein offenes Finding mehr; das nächste heißt SF-106.
- **Neu-Baseline auf `59e3ae3`:** Die Zahl der Guideline-Stellen steigt von 1399 auf 1401. Die zwei neuen gehören zu REQ-CNT-003; die stand deshalb wieder auf `draft` (R-1).
- **Schema:** Version `2.0.1-dev` mit Baseline `59e3ae3`, am Inhalt ändert sich nichts.
- **Dev-Broker, SDK und Interop-Test** waren schon so gebaut. Daran ändert sich nichts.
- **R-1 – REQ-CNT-003:** Du hast geschrieben „R-1, REQ-CNT-003 akzeptiert“. Die REQ steht jetzt auf `reviewed` (`41c9f44`, Nachtrag 2). Es gibt keinen Entwurf mehr.
  - Neu steht in ihr: Der Broker nimmt jede Nutzlast bis `MaxPacketSize` an; begrenzt er das ganze Paket, ist die Grenze um 65544 Byte größer; Nutzlast + Topic-Länge + 4 bleiben unter 268435455.

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
