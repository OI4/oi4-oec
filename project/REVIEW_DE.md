# Review und Entscheidungen – Erklärung auf Deutsch

_Stand: 26.10.2026, nach deinem Review von C-039 und dem Einbau von SF-100 in die RIG._

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

3 Review-Punkte (R-1 bis R-3). Sie sind entstanden, weil ich SF-100 wie beauftragt in die RIG eingebaut habe. Es gibt kein offenes Spec-Finding (nächste Nummer SF-101) und keine offene Frage (Q-011 bleibt geparkt). Die Abdeckungslücken bleiben bei 15, denn beide REQs prüft D8, nicht das Schema. Details stehen im [C-039-Log, Nachtrag 2](log/2026-10-25_C-039_config-broker.md#addendum-2--sf-100-in-the-guideline-re-baseline-on-168e1d1-2026-10-26).

**Was ist passiert?** Der neue RIG-Text (Commit `168e1d1`) sagt jetzt genau, wie `Address` aussieht, dass `SecurePort` 1 bis 65535 ist und dass Anwendungen unbekannte Felder ignorieren. Danach habe ich auf den neuen Guideline-Stand umgestellt (Re-Baseline). Dabei ist ein neuer Satz im Inventar aufgetaucht („Clients ignore elements …“). Zwei schon freigegebene REQs habe ich an den neuen Text angepasst. Nach unserer Regel stehen sie deshalb wieder auf `draft`.

### R-1 – REQ-CNT-003 (Inhalt von `broker.json`) angepasst

Die REQ beschreibt, was in `broker.json` stehen muss. Neu darin:
- die genaue Form der **Adresse**: IPv4 ohne führende Nullen, IPv6 ohne eckige Klammern, oder ein Host-Name nach RFC 1123; kein Port, kein `mqtts://`, kein Pfad;
- der **Port** liegt zwischen 1 und 65535;
- die Datei **darf weitere Felder** enthalten.

Dazu kommen zwei neue Zitate aus der RIG und ein Hinweis, dass das Schema, das Orakel und die 18 Vektoren genau das prüfen. Die Prüfmethode bleibt `d8`.

### R-2 – REQ-CNT-004 (Anwendung nutzt `broker.json`) ergänzt

Die REQ sagt, dass eine Anwendung Adresse, Port und Paketgröße aus `broker.json` übernimmt. Neu ist der Satz: **„Sie ignoriert Felder, die sie nicht kennt.“** Das ist die Gegenseite zu R-1: Die Plattform darf Felder hinzufügen, und die Anwendung stolpert nicht darüber. D8 kann das testen, indem es dem Container eine `broker.json` mit einem Zusatzfeld gibt. Die Stufen bleiben gleich (L2 SHOULD, L3 MUST).

### R-3 – Der neue Text in der RIG (`168e1d1`)

- **Address:** Wertebereich „IPv4-Adresse, IPv6-Adresse (ohne Klammern) oder Host-Name (RFC 1123)“. Dazu ein Absatz, der die Formen genau erklärt (siehe [`REVIEW.md`](REVIEW.md#r-3--guideline-wording-168e1d1)).
- **SecurePort:** Wertebereich „1 … 65535“.
- **Nach der Feldliste:** „Clients ignore elements of `broker.json` they do not know.“

**Ein Punkt geht über den Vorschlag im Register hinaus:** Dort stand als Textvorschlag für `Address` nur der neue Wertebereich. Die genauen Regeln (keine führenden Nullen, höchstens 253 Zeichen usw.) standen nur in der Begründung. Ich habe sie als Absatz in die RIG übernommen, damit sie in der Guideline selbst stehen und nicht nur im Register. Wenn dir die kurze Fassung lieber ist, nehme ich den Absatz wieder heraus.

Es gibt kein GL-ADR, wie bei SF-099. Der Docusaurus-Build ist grün.

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

## Was aus deinem letzten Review (C-039) geworden ist

- **D-1 – Nachtrag C-039 zu ADR-0019:** angenommen. Im ADR steht der Nachtrag jetzt als „accepted“. Punkt 7 sagt, dass das `broker.json`-Schema SF-100 umsetzt, wie entschieden (`d7520ba`).
- **D-2 – SF-100:** wie vorgeschlagen entschieden. Den RIG-Text habe ich eingebaut (`168e1d1`), im Register steht SF-100 auf „resolved“ (`7808df7`). Daraus kommen die neuen Punkte R-1 bis R-3 oben.
- **R-1 bis R-4:** alles freigegeben. Die 2 REQs und die 20 Vektoren stehen auf `reviewed` (`dd5893d`). Die Lücken sinken wie vorgerechnet von 16 auf **15**; Config hat keine Lücke mehr.
- **Nebenbei:** An mehreren Stellen (Vektoren, Schema, Orakel) stand „SF-100, proposed“. Das „proposed“ habe ich entfernt. Am Inhalt hat sich nichts geändert.

## Was aus dem Review davor (C-038) geworden ist

- **D-1 – Nachtrag C-038 zu ADR-0019:** angenommen. Im ADR steht der Nachtrag jetzt als „accepted“. An dem Satz in §4, den Punkt 1 ersetzt (Oi4Pv), steht ein Verweis auf den Nachtrag, damit man ihn beim Lesen nicht übersieht (`574b8e0`).
- **D-2 – `WriterGroupName` auf einem Topic ohne Filter:** angenommen. Das Orakel bleibt, wie es ist (`d6d5010`).
- **R-1 bis R-3:** alles freigegeben. Die 9 Vektoren V-PAY-231 … 239 stehen auf `reviewed` (`dc80c8a`). Die Lücken sinken wie vorgerechnet von 33 auf 19. Data, Event und SubscriptionList haben keine Lücke mehr.
