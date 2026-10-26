# Review und Entscheidungen – Erklärung auf Deutsch

_Stand: 26.10.2026, nach C-041 (P2-Exit-Review)._

Diese Datei erklärt die offenen Punkte aus [`REVIEW.md`](REVIEW.md) auf Deutsch und in Alltagssprache. Sie ist eine Lesehilfe auf Wunsch des PO (Ausnahme von der Regel „Projektartefakte auf Englisch“). **Maßgeblich ist `REVIEW.md`**; dort stehen auch die Checkboxen. Die Nummern (D-1, R-1 …) sind in beiden Dateien gleich, du kannst also z. B. im Chat antworten: „D-1 bis D-3 wie vorgeschlagen, R-1 akzeptiert“.

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

C-041 war das Abschluss-Review von P2. Es gibt **3 Entscheidungen** und **1 Review-Punkt**. Die Abdeckung bleibt in jedem Fall bei **0 Lücken**. Es gibt kein offenes Spec-Finding (nächste Nummer SF-101) und keine offene Frage (Q-011 bleibt geparkt). Details stehen im [C-041-Log](log/2026-10-26_C-041_p2-exit.md).

### D-1 – Nachtrag C-041 zu ADR-0019

Der Nachtrag legt fest, wie wir P2 absichern und die Schemas veröffentlichen:

1. **Gegenprobe Orakel ↔ Schema (`spectool crosscheck`).** Bisher haben wir nur gezeigt, dass Orakel und Schema bei den *Vektoren* dasselbe sagen. Jetzt prüft das Orakel zusätzlich alle 634 Testfälle der Schemas, auch die künstlichen „Probes“. Regel: Lehnt das Schema eine Nachricht ab, muss auch das Orakel sie ablehnen (mit mindestens denselben REQs). Akzeptiert das Schema sie, darf das Orakel nur Regelschicht-REQs melden. Damit ist P2-Exit-Kriterium 5 belegt.
2. **Ein Fehler im Orakel, den die Gegenprobe gefunden hat.** `ReplyTo` = `1639693510094-abc` hat das Orakel durchgelassen, das Schema nicht. Richtig ist das Schema: Nach dem Zeitstempel muss der Absender stehen (`<ServiceType>/<IL>`). Das Orakel prüft das jetzt auch. Danach: 0 Abweichungen.
3. **Ein Prüfskript für alles: `oec/check.sh`.** Es ruft alle Prüfungen nacheinander auf (Katalog, Abdeckung, beide Validatoren, Release-Schutz, Gegenprobe, D7-Tests). Am Ende steht „all checks passed“. Das ist unsere lokale CI, solange nichts gepusht wird.
4. **Release-Regeln für die Schemas.**
   - Die Schemas haben jetzt die Version **2.0.0**. Sie gehört zum Guideline-Stand `168e1d1`.
   - Eine Manifest-Datei (`release/2.0.0.json`) speichert für jede Schema-Datei einen Fingerabdruck (Hash). Ändert jemand später eine freigegebene Datei, schlägt die Prüfung fehl.
   - Innerhalb von v2 darf keine `$id` und kein `$defs`-Name einer freigegebenen Version verschwinden. Andere verweisen darauf, und sonst bräche ihr Code.
   - Nach einem Release arbeiten wir mit einer Vorab-Version (z. B. `2.0.1-dev`) weiter. Erst ein neues Release macht daraus eine feste Version.
   - Ändert sich der Guideline-Stand, muss auch die Schema-Version steigen. Das prüft das Skript.
   - Nebenbei gefunden: Im erzeugten Ordner `dist/` lag noch eine alte Datei aus dem Proof of Concept. Der Rust-Validator hat sie mitgeladen („92 Schemas“). Der Ordner wird jetzt vor jedem Lauf geleert; es sind 90 Schemas.
5. **D7 nennt seine Schema-Version.** `oec-check version` und jeder Bericht zeigen jetzt `schemas 2.0.0` und den Guideline-Stand. Passen die eingebauten Schemas nicht zum eingebauten Katalog, startet D7 gar nicht erst.
6. **`schema` heißt wirklich „das Schema prüft es“.** Siehe R-1.

**Empfehlung:** annehmen.

### D-2 – P2 abschließen

Alle 7 Exit-Kriterien von P2 sind erfüllt:

| # | Kriterium | Stand |
|---|---|---|
| 1 | ADR-0019 angenommen | ja (Nachtrag C-041 = D-1) |
| 2 | Jede `schema`/`d7-validate`-REQ hat einen freigegebenen positiven und negativen Vektor | ja, 158 REQs, 0 Lücken |
| 3 | Ein Eingangs-Schema je erlaubter Topic-Form | ja, 29 von 29 |
| 4 | Beide Validatoren sind sich einig | ja, 634 Fälle, 0 Abweichungen (Python ebenso) |
| 5 | Das Orakel prüft alles, was die Schemas prüfen | ja, Gegenprobe ohne Abweichung |
| 6 | D7 läuft unter Windows, macOS und Linux | ja, mit Einschränkung: Es lässt sich für alle drei bauen, getestet ist es aber nur auf macOS |
| 7 | Schemas mit stabilen `$id`s veröffentlicht | ja, mit Einschränkung: 2.0.0 ist eingefroren, aber noch nicht im Netz (hängt an Q-011) |

Die beiden Einschränkungen hängen an der Veröffentlichung (Q-011, gehostete CI), nicht an P2. Sie stehen auch im `CHANGELOG.md` der Schemas.

**Empfehlung:** P2 abschließen, sobald D-1 und R-1 angenommen sind. Danach kommt laut ROADMAP der Rest von P3 (Skizze der Rust-SDK- und C-API, Wahl der Crates, Aufbau der D8-CLI auf Basis von D7) und dann P4 (SDK-Kern). Die Reihenfolge bestimmst du.

### D-3 – Git-Tag `oec-schema-2.0.0`

Nach deiner Freigabe bekommt der Commit mit dem freigegebenen Release ein lokales Git-Tag `oec-schema-2.0.0` (kein Push). So findet man den Stand später schnell wieder.

Warum nicht `oec/schema/v2.0.0`? Go würde das als Version des Go-Moduls lesen. Eine Version 2.x verlangt in Go aber ein `/v2` am Modulnamen, und das hat unser Modul nicht. Das Tag würde dann eher stören.

**Alternative:** gar kein Tag; das Manifest allein kennzeichnet das Release.

**Empfehlung:** Tag setzen.

### R-1 – 7 REQs verlieren die Prüfmethode `schema`

Bei diesen 7 REQs stand `schema`, obwohl kein Schema sie meldet. Alle ihre negativen Vektoren erkennt nur die Regelschicht von D7:

| REQ | Worum es geht | Warum das Schema es nicht sehen kann |
|---|---|---|
| REQ-LIFE-003, 007, 009 | Birth-, Close- und Will-Nachricht | Man sieht einer einzelnen Nachricht nicht an, ob sie eine Birth/Close/Will ist; das geht nur mit `--role` (Regel R-6). |
| REQ-MSG-023 | `DataSetWriterName` = Source im Topic | Das Schema kennt das Topic nicht (Regel R-2). |
| REQ-RES-MAM-016 | `ProductInstanceUri` = `DataSetWriterName` | Zwei Felder müssen gleich sein; das kann JSON Schema nicht (Regel R-3). |
| REQ-RES-PUBLIST-003 | Kein Eintrag doppelt | „Eindeutig über mehrere Felder“ kann JSON Schema nicht (Regel R-3, wie schon REQ-MSG-038 in C-035). |
| REQ-RES-REFDES-013 | `Query` nur ohne Source im Topic | Das Schema kennt das Topic nicht (Regel R-2). |

Die REQ-Texte bleiben unverändert. Es ändert sich nur die Prüfmethode, dazu kommt eine Notiz. Weil alle 7 weiter `d7-validate` haben, bleibt die Abdeckung bei 0 Lücken.

**Empfehlung:** annehmen. Danach setze ich die 7 REQs mit `spectool review` wieder auf `reviewed`.

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

## Was aus deinem letzten Review (C-040) geworden ist

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
