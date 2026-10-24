# Review und Entscheidungen – Erklärung auf Deutsch

_Stand: 24.10.2026, nach C-035 (MetaData + Rollen; Guideline `68d878d`)._

Diese Datei erklärt die offenen Punkte aus [`REVIEW.md`](REVIEW.md) auf Deutsch und in Alltagssprache. Sie ist eine Lesehilfe auf Wunsch des PO (Ausnahme von der Regel „Projektartefakte auf Englisch“). **Maßgeblich ist `REVIEW.md`**; dort stehen auch die Checkboxen. Die Nummern (D-1, R-1 …) sind in beiden Dateien gleich, du kannst also z. B. im Chat antworten: „D-1 wie vorgeschlagen, R-1 bis R-4 akzeptiert“.

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

**31 Entwürfe** (4 REQs, 27 Vektoren) und **3 Entscheidungen**. Es gibt kein offenes Spec-Finding: In C-035 habe ich keine Unklarheit in der Guideline gefunden. Nach deinem Review sinken die Abdeckungslücken von 96 auf **77**.

### Neue Begriffe in diesem Zyklus

- **MetaData (DataSetMetaData):** die „Bauanleitung“ zu einem Datensatz. Eine Data-Nachricht schickt nur Werte, z. B. `{"Position": 12.5, "Mode": 1}`. Die MetaData sagt, was die Werte bedeuten: Feldname, OPC-UA-Datentyp (`BuiltInType`, `DataType`), ob es ein Array ist (`ValueRank`), Beschreibung, Einheit (als Property). Wer Daten richtig lesen will, braucht die MetaData dazu.
- **Eigene Datentypen** in der MetaData – drei Listen, die die Guideline erlaubt, aber nie mit einem Beispiel zeigt:
  - **StructureDataTypes:** ein zusammengesetzter Typ, z. B. `Range` mit den Feldern `Low` und `High`;
  - **EnumDataTypes:** eine Aufzählung, z. B. `AxisMode` mit `Manual = 0`, `Auto = 1`;
  - **SimpleDataTypes:** ein umbenannter Grundtyp, z. B. `Percent` = Double.
- **Vordefinierte Ressourcen-MetaData (MG B2):** Für die Standard-Ressourcen (Health, MAM, ReferenceDesignation …) liefert die Guideline die MetaData fertig mit. Jede hat eine feste `DataSetClassId` aus Tabelle A2, an der man die Ressource erkennt.
- **Regelschicht (R-1 … R-6):** ein paar kleine Prüfungen in D7 `validate`, die ein JSON Schema grundsätzlich nicht kann (ADR-0019 §5). Beispiel: R-2 „`DataSetWriterName` ist gleich der `Source` im Topic“, denn das Schema sieht das Topic nicht.
- **Erklärte Rolle (`--role`):** Eine Birth-Nachricht sieht aus wie ein ganz normales `Pub MAM`, eine Close- oder Will-Nachricht wie ein normales `Pub Health`. Woran man sie unterscheidet, steht nicht in der Nachricht. Deshalb sagt man D7 beim Prüfen dazu, welche Rolle die Nachricht haben soll, z. B. `validate --role will`. Dann prüft D7 die Zusatzregeln dieser Rolle. So hast du es in Q-020 (a) entschieden.

---

## Entscheidungen

### D-1 – Nachtrag zu ADR-0019 (Schema-Architektur)

**Worum geht es?**
Beim Bauen der MetaData-Schemas haben sich vier Punkte ergeben, die das angenommene ADR-0019 nicht genau genug regelt. Ich habe sie als Nachtrag „Amendment C-035“ ans ADR gehängt. Nichts, was du schon entschieden hast, wird dadurch umgedreht.

1. **`ResourceMetaData` für vordefinierte MetaData.** Bisher sollte jede einzelne MetaData gegen das allgemeine Schema `DataSetMetaData` geprüft werden. Das allgemeine Schema weiß aber nicht, dass z. B. die MetaData mit dem Namen `ReferenceDesignation` die feste ID aus Tabelle A2 tragen muss. Unser Orakel prüft das jedoch (REQ-MSG-012/010). Damit Schema und Orakel dasselbe Ergebnis liefern, gibt es zusätzlich `ResourceMetaData`: das allgemeine Schema plus „wenn der Name `ReferenceDesignation` ist, dann muss die ID … sein“, einmal pro Ressource. Für herstellereigene Datensätze bleibt es beim allgemeinen Schema.
2. **„Namen sind eindeutig“ gehört in die Regelschicht (R-3).** Feldnamen in einem Datensatz, Feldnamen einer Struktur und Namen in einer Aufzählung dürfen sich nicht wiederholen. JSON Schema kann das nicht ausdrücken. Es kann nur prüfen, ob zwei *ganze* Einträge gleich sind, nicht ob zwei Einträge denselben *Namen* haben. Darum prüft das die Regelschicht.
3. **Neue Regel R-6 „erklärte Rolle“** – die technische Umsetzung von Q-020 (a), siehe Begriffe oben.
4. **REQ-LIFE-003/007/009** bekommen nach ADR-0019 §9 zusätzlich `d7-validate` (siehe R-4).

**Mein Vorschlag:** annehmen.

### D-2 – Metadata-Ressource schon jetzt statt in C-037

**Worum geht es?**
Laut Plan sollten die Schemas für die Nachrichten `Pub`/`Get`/`Set Metadata` (die Ressource „Metadata“, mit der eine Anwendung ihre MetaData veröffentlicht) erst in C-037 kommen. Diese drei Schemas sind aber nur eine dünne Hülle um genau die MetaData-Objekte, die ich in C-035 ohnehin gebaut habe, plus die Regel „leeres `{}` nur beim `Get`“. Ich habe sie deshalb gleich in C-035 mitgemacht. C-037 enthält jetzt nur noch Data, Event und SubscriptionList (8 statt 11 Topic-Formen).

**Mein Vorschlag:** annehmen. C-035 ist dadurch nur um 3 Schema-Dateien und 2 Vektoren gewachsen.

### D-3 – Soll D7 bei einer Rolle auch das Topic prüfen?

**Worum geht es?**
Zu jeder Rolle gibt es zwei Arten von Regeln:

- **Inhalt** (deine Entscheidung Q-020 bezog sich darauf): Die Birth ist das eigene MAM (REQ-LIFE-003). Die Close hat `NORMAL_0` und HealthScore 0 (REQ-LIFE-007). Die Will hat `FAILURE_1` und HealthScore 0 (REQ-LIFE-009). Das prüft R-6 jetzt.
- **Topic:** Die Nachricht wird unter der eigenen AppId als `Source` veröffentlicht (REQ-LIFE-002 Birth, REQ-LIFE-006 Close, REQ-LIFE-008 Will). Diese REQs werden heute live geprüft (`d7-monitor`/`d7-test`).

Weil das Topic die eigene AppId und die `Source` beide enthält, *könnte* D7 mit `--role` auch die Topic-Regel offline prüfen.

- **Option (a) – empfohlen: so lassen.** Bei der Birth folgt „Source = eigene AppId“ ohnehin schon aus anderen Regeln, die D7 offline prüft (REQ-RES-MAM-016, R-2 und REQ-LIFE-003). Close und Will prüft man am besten live, denn nur dann sieht man, dass der Broker die Will wirklich ausliefert. Nachteil: Der `--role`-Bericht nennt REQ-LIFE-006/008 nicht.
- **Option (b): auch `d7-validate` dazunehmen.** Vorteil: Der Offline-Bericht ist vollständig. Nachteil: Drei weitere schon reviewte REQs müssten geändert werden. Außerdem enthält REQ-LIFE-008 auch Dinge, die nur live prüfbar sind („in jedem CONNECT angemeldet“, „Retain = false“). Diese REQ müsste man also aufteilen oder mit beiden Methoden führen.

---

## Reviews

### R-1 – MetaData-Vektoren V-PAY-152 … 170

**Was ist das?**
**V-PAY-152** ist ein vollständiges, gültiges Beispiel: Ein Hersteller veröffentlicht die MetaData seines Datensatzes `axis` mit allen drei Arten eigener Datentypen (`Range`, `AxisMode`, `Percent`). Das ist nötig, weil die Guideline dafür kein Beispiel hat. Die übrigen 18 Vektoren nehmen ein gültiges Beispiel und bauen **genau einen typischen Fehler** ein. Beispiele:

- Datentyp als Text `"Double"` statt als Zahl (V-PAY-158);
- NodeId in der verbotenen Form `ns=1;…` (V-PAY-160);
- Beschreibung ohne Sprache (V-PAY-161);
- zwei Felder mit gleichem Namen (V-PAY-159, gefunden von der Regelschicht, nicht vom Schema);
- eine Aufzählungs-Zahl als JSON-Zahl statt als Text (V-PAY-166);
- ein leeres `{}` bei `Pub`/`Set Metadata`, wo nur ein `Get` leer sein darf (V-PAY-169/170).

V-PAY-155 verletzt absichtlich zwei REQs: Ein Strukturtyp als Zahl ist zugleich „falsche Kodierung“ (REQ-MSG-034) und „kein erlaubter Wert“ (REQ-MSG-042).

**Bitte prüfen:** Ist jeder Fehler realistisch, und stimmen Titel und Notiz? Welche REQs verletzt sind, rechnet das Orakel aus. Die Tabelle steht in `REVIEW.md`.

### R-2 – Rollen-Vektoren V-PAY-171 … 175

**Was ist das?**
Fünf Nachrichten, die als *normale* Nachricht gültig sind, aber ihre erklärte Rolle nicht erfüllen:

- eine Birth mit zwei Datensätzen statt nur dem eigenen MAM (V-PAY-171);
- eine Close ohne HealthScore oder mit `FAILURE_1` (V-PAY-172/173);
- eine Will mit `NORMAL_0` oder HealthScore 10 (V-PAY-174/175).

Das Schema allein lässt alle fünf durch, denn es kennt die Rolle nicht. Erst R-6 meldet sie. Eine Birth „über ein fremdes Gerät“ habe ich bewusst nicht als Vektor gebaut, weil sie zusätzlich die Topic-Regel REQ-LIFE-002 verletzen würde (siehe D-3). Sie ist mit Unit-Tests abgedeckt.

### R-3 – Die Guideline-Beispiele Birth/Close/Will bekommen eine Rolle

**Was ist das?**
V-PAY-023/024/025 sind die Beispiele aus MG B1.1.1/B1.1.2 und waren schon reviewt. Sie haben jetzt zusätzlich `role: birth`/`close`/`will` und einen Satz in den Notizen. Nachricht und Topic sind unverändert. Damit sind sie die positiven Beispiele für die Rollenprüfung. Weil sich etwas Reviewtes geändert hat, stehen sie wieder auf `draft`.

### R-4 – Geänderte Prüfmethoden von vier REQs

- **REQ-LIFE-003/007/009** (Inhalt von Birth, Close und Will) bekommen zusätzlich `d7-validate`, weil D7 sie mit `--role` jetzt offline prüfen kann. Das ist die Umsetzung deiner Entscheidung Q-020 (a).
- **REQ-MSG-038** („Feldnamen sind eindeutig“) verliert `schema`, weil ein JSON Schema das nicht ausdrücken kann (siehe D-1 Punkt 2). Geprüft wird es von der Regelschicht. REQ-MSG-043/044 behalten `schema`, weil der größte Teil davon (Aufbau der Einträge) im Schema steckt. Nur ihre Eindeutigkeits-Teile prüft die Regelschicht.

---

## Was aus deinem letzten Review (C-034) geworden ist

- **D-1 – SF-098:** Ein `Del` enthält höchstens eine DataSetMessage, so wie es die Ressource in Kapitel 9.1 festlegt: keine bei `ReferenceDesignation`, genau den zu löschenden Eintrag bei `SubscriptionList`. Wie angekreuzt habe ich das in die Guideline eingearbeitet (MG 8.3.11, Commit `68d878d`, GL-ADR-019 ergänzt). Danach habe ich das Projekt auf diesen Stand umgestellt (Re-Baseline). Die Zahl der Inventory-Items bleibt 1397, weil der geänderte Satz kein Schlüsselwort enthält. REQ-PAG-009 sagt jetzt dasselbe.
- **D-2 – Q-020:** Option (a). Ein Vektor bekommt optional eine Rolle (`birth`/`close`/`will`), und D7 bekommt `validate --role`. So lassen sich Birth, Close und Will offline prüfen. Das baue ich in C-035, die CLI-Option selbst in C-039.
- **D-3 – Aufteilung:** bleibt so. C-035 ist MetaData, der P2-Abschluss ist C-040.
- **R-1 bis R-5:** Alles ist freigegeben. Die 32 Entwürfe stehen auf `reviewed`. Die Lücken bei der Vektor-Abdeckung sinken von 129 auf 96.
