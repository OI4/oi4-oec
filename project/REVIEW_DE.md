# Review und Entscheidungen – Erklärung auf Deutsch

_Stand: 26.10.2026, nach C-040 (D7 `validate`)._

Diese Datei erklärt die offenen Punkte aus [`REVIEW.md`](REVIEW.md) auf Deutsch und in Alltagssprache. Sie ist eine Lesehilfe auf Wunsch des PO (Ausnahme von der Regel „Projektartefakte auf Englisch“). **Maßgeblich ist `REVIEW.md`**; dort stehen auch die Checkboxen. Die Nummern (D-1, R-1 …) sind in beiden Dateien gleich, du kannst also z. B. im Chat antworten: „D-1 bis D-4 wie vorgeschlagen, R-1 bis R-4 akzeptiert“.

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

4 Entscheidungen (D-1 bis D-4) und 4 Review-Punkte (R-1 bis R-4) aus C-040. Es gibt kein offenes Spec-Finding (nächste Nummer SF-101) und keine offene Frage (Q-011 bleibt geparkt). Details stehen im [C-040-Log](log/2026-10-26_C-040_d7-validate.md).

**Was ist passiert?** Es gibt jetzt das erste Stück des Prüftools D7: `oec-check validate`. Man gibt ihm eine Nachricht (Topic + JSON-Payload), und es sagt offline, ob sie gültig ist und welche REQs verletzt sind. Es prüft in vier Schichten:
1. **Topic** – stimmt der Aufbau (`Oi4v2/…/Pub/Health/…`), ist die Kombination Methode × Ressource erlaubt, sind die Identification Links richtig?
2. **Parse** – ist die Payload sauberes JSON?
3. **Schema** – passt die Payload zum JSON Schema der Topic-Form?
4. **Regeln** – was ein Schema nicht kann, z. B. „der Name im Datensatz muss gleich dem Source im Topic sein“.

D7 besteht alle Vektoren, die wir haben. Dabei hat es bei einem Vektor eine fehlende REQ gefunden (R-1).

**Die Lücken:** Vor C-040 hatten noch 15 REQs keinen freigegebenen positiven *und* negativen Vektor. Jetzt sind es 11, und **nach deinem Review sind es 0**. Das habe ich auf einer Kopie mit allen Entwürfen als „reviewed“ nachgerechnet; die Exit-Prüfung `coverage -gate` ist dort grün. Damit wäre das P2-Exit-Kriterium 2 erfüllt.

### D-1 – Nachtrag C-040 zu ADR-0019 (wie D7 arbeitet)

Ich schlage vor, die Bauweise von D7 im ADR festzuhalten:
- die **vier Schichten** oben; jeder Befund nennt Schicht, REQ, Stelle in der Nachricht und Text;
- **keine Doppelmeldungen:** Sehen Schema und Regelschicht denselben Fehler an derselben Stelle, meldet D7 ihn nur einmal;
- **R-4 als „Probe“:** Feinheiten bei Zahlen und Datum (z. B. 30. Februar) prüft D7, indem es den Wert durch eine saubere Form ersetzt und das Schema noch einmal fragt. Das ist mit Unit-Tests abgesichert; heute braucht es kein Vektor;
- **die Kommandozeile:** Optionen, Dateiformate, Exit-Codes (0 = gültig, Warnungen erlaubt; 1 = ungültig; 2 = falscher Aufruf oder Datei; 3 = interner Fehler). Das D8-Tool heißt später `oec-infra`;
- **läuft überall:** reines Go ohne C-Teile. Es lässt sich für Windows, Linux und macOS bauen (P2-Exit-Kriterium 6); getestet habe ich bisher nur auf macOS.

Dazu kommen die Punkte D-2 bis D-4 und R-4, die auch im Nachtrag stehen.

### D-2 – Kaputtes JSON meldet REQ-CMP-005

Ist die Payload gar kein gültiges JSON (z. B. ein Schlüssel doppelt oder ein Syntaxfehler), hatte D7 bisher keine REQ zum Nennen. Jetzt nennt es **REQ-CMP-005** („die Nachricht ist OPC-UA-konformes JSON“). Die Alternative wäre eine neue REQ „die Payload ist JSON“ über ein neues Spec-Finding. Das würde aber nur REQ-CMP-005 wiederholen.

### D-3 – Codierter Identification Link in der Payload (REQ-IL-005)

Im Topic steht ein Identification Link **base64url-codiert**, in der Payload dagegen **im Klartext** (REQ-IL-005). Ein typischer Fehler ist, die codierte Form aus dem Topic in die Payload zu kopieren. Orakel und D7 erkennen das jetzt mit einer **Heuristik**: Ist ein Wert kein gültiger Link, aber seine base64url-Decodierung ist einer, dann wird REQ-IL-005 gemeldet. Geprüft werden PublisherId, RequestorId, der Absender in MessageId und ReplyTo und jeder DataSetWriterName.
- **Risiko:** Ein Wert, der zufällig wie ein codierter Link aussieht, würde fälschlich gemeldet. Ein solcher Fall ist mir aus der Guideline nicht bekannt.
- **Alternative:** REQ-IL-005 nur als Schirm-REQ über die Formatregeln der einzelnen Felder führen. Dann würde D7 den typischen Fehler aber nicht gezielt erkennen.

### D-4 – Leerzeichen im Filter zählen als „negativer“ Vektor für REQ-TOP-020

REQ-TOP-020 ist eine **Empfehlung** (SHOULD): kein Leerzeichen im Filter. Ein Topic mit Leerzeichen ist trotzdem gültig; D7 gibt nur eine **Warnung** aus. Für die Abdeckungszählung schlage ich vor: Ein solcher Vektor zählt als **negatives** Beispiel für REQ-TOP-020 (er zeigt, dass D7 die Abweichung bemerkt), nicht als positives. Die 8 schon freigegebenen Leerzeichen-Vektoren bleiben unverändert; nur die Zählung ändert sich. Das positive Beispiel ist V-TOP-137 (R-3).

### R-1 – V-TOP-218 bekommt eine zweite REQ

V-TOP-218 ist ein `Call` an NewDataSetWriterId mit einem Filter im Topic. Bisher stand dort nur REQ-TOP-013. D7 meldet zusätzlich **REQ-ACT-020**, weil diese Action gar keinen Filter haben darf. V-TOP-253 hat das schon so. Der Vektor war freigegeben und steht deshalb wieder auf `draft`.

### R-2 – Neue Payload-Vektoren V-PAY-242 bis 244

- V-PAY-242 (ungültig): ein `Pub Health`, dessen ReplyTo den Link codiert enthält.
- V-PAY-243 (ungültig): ein `Call`, dessen RequestorId und MessageId den Link codiert enthalten.
- V-PAY-244 (gültig): ein `Pub Health` mit allen Links im Klartext.

Das JSON Schema kann so etwas nicht sehen (es kann kein base64url decodieren); das macht die Regelschicht.

### R-3 – Neue Topic-Vektoren V-TOP-135 bis 137 und V-TOP-254

- V-TOP-135 (gültig): `Pub Health` eines Geräts (mit Source).
- V-TOP-136 (gültig): `Call NewDataSetWriterId` mit Source.
- V-TOP-137 (gültig): `Pub Event` mit Filter `Status/Bad`.
- V-TOP-254 (ungültig): `…/Pub/Health/` – die Source-Ebene ist leer (REQ-TOP-018).

Sie liefern die noch fehlenden Beispiele für einige TOP- und IL-REQs.

### R-4 – Neue Schirm-REQs

Sieben allgemeine REQs fasse ich als Schirm über ihre Einzelregeln (so wie in ADR-0019 §9 beschlossen):
- REQ-CMP-004 (Topic ist konform) → die einzelnen Topic-Regeln;
- REQ-CMP-025/026 (Felder) → die „Fields of …“-REQs;
- REQ-CMP-005 (OPC-UA-JSON) → die JSON-Regeln, Rest: sauberes JSON (D-2);
- REQ-TOP-002 → CMP-004 und CMP-005;
- REQ-IL-001 und REQ-IL-008 → die einzelnen Link-Regeln.

Was offline nicht prüfbar ist (z. B. „antwortet auf dem richtigen Topic“), steht als Rest im Schirm und gehört zu `d7-monitor`/`d7-test`.

---

## Neue Begriffe aus C-040

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

## Was aus deinem letzten Review (C-039, Nachtrag 2) geworden ist

- **R-1 – REQ-CNT-003** und **R-2 – REQ-CNT-004:** freigegeben, beide stehen auf `reviewed` (`995258d`).
- **R-3 – der neue RIG-Text zu SF-100:** freigegeben; der Absatz mit den genauen Regeln bleibt drin.
- Danach gab es keinen Entwurf mehr; die Lücken lagen bei 15.

## Was aus dem Review davor (C-039) geworden ist

- **D-1 – Nachtrag C-039 zu ADR-0019:** angenommen. Im ADR steht der Nachtrag jetzt als „accepted“. Punkt 7 sagt, dass das `broker.json`-Schema SF-100 umsetzt, wie entschieden (`d7520ba`).
- **D-2 – SF-100:** wie vorgeschlagen entschieden. Den RIG-Text habe ich eingebaut (`168e1d1`), im Register steht SF-100 auf „resolved“ (`7808df7`). Daraus kamen die Punkte R-1 bis R-3 des Nachtrags 2.
- **R-1 bis R-4:** alles freigegeben. Die 2 REQs und die 20 Vektoren stehen auf `reviewed` (`dd5893d`). Die Lücken sinken wie vorgerechnet von 16 auf **15**; Config hat keine Lücke mehr.
- **Nebenbei:** An mehreren Stellen (Vektoren, Schema, Orakel) stand „SF-100, proposed“. Das „proposed“ habe ich entfernt. Am Inhalt hat sich nichts geändert.
