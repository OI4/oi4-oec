# Review und Entscheidungen – Erklärung auf Deutsch

_Stand: 26.10.2026, nach deinem Review von C-041 – P2 ist abgeschlossen._

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

**Nichts.** Du hast alle Punkte aus C-041 angenommen, damit ist **P2 abgeschlossen**. Es gibt keinen Entwurf (`draft`), kein offenes Spec-Finding (nächste Nummer SF-101) und keine offene Frage außer der geparkten Q-011. Die Abdeckung liegt bei **0 Lücken**.

Als Nächstes kommt **P3** (C-042): Dort entscheiden wir gemeinsam, wie die SDKs (Rust mit C-Schnittstelle, Micro-SDK in C) und die Compliance-Tools (D7, D8) im Detail aussehen. Die Vorlage dafür steht in `prompt.md`.

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
