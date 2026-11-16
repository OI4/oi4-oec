# C-117 – Smart sensor demos: out of range = OFF_SPEC, broken sensor wire = FAILURE

- Date: 2026-11-28 (wall clock of the session: 2026-11-16)
- Phase: none open (all phases closed after C-111); PO request (G-3.4 a: scope)
- Agent/Author: GitHub Copilot (Claude Opus 5.5)
- Status: done
- Baseline: guideline `980cde8` (unchanged); workspace before the cycle: `12939e2`

## Intent

PO request after C-116: "Zum ESP Noch etwas. Rot soll "out of range" sein und im Falle, dass er in diesem Bereich ist die Health auf out-of-specification gehen. Zudem, falls wir ds noch nicht haben, soll der Health auf maintance required gehen, wenn die Verbindung zum DHT nicht funktionert, also z.B. ein Draht ab ist".

PO answer on how red is shown: within `ColdTemperature` … `HotTemperature` only blue → green → yellow; red only above `HotTemperature` = `OFF_SPEC_3`.

After the first wire test the PO noted: "ein Kabelunterbruch ist wohl eher FAILURE". The agent proposed `FAILURE_1` with HealthScore 10 (not 0, MG 6.3.5), and the PO chose it.

## Done

- **ESP32-S3 display board (variant `esp32s3-lcd147`):**
  - Within the range the scale runs blue → green → yellow (hue 240° … 60°). Above `HotTemperature` there is a red field at the right end of the bar; the marker sits in it when out of range.
  - The temperature and the LED turn red above `HotTemperature`, and the Health is `OFF_SPEC_3` with HealthScore 50. There is no hysteresis: exactly at `HotTemperature` the colour is yellow and the Health `NORMAL_0`, so display, LED and Health always agree.
  - The footer shows the Health in words with its NE107 colour (MG 8.3.2 Table 13): "Normal" green, "Off spec" yellow, "Failure" red.
  - The Config descriptions of the scale and the Kconfig help were reworded.
- **All variants:** after three failed rounds of readings in a row (e.g. a broken wire), the Health is `FAILURE_1` with HealthScore 10 instead of `FAILURE_1`/0. The display shows "Failure", the LED is off, and the next good reading sets `NORMAL_0`/100.
  - The Health is computed in one function (`health_now`) for the device loop and the display task, and it is set only on a change.
- **Core2 climate display:**
  - Same scale: blue … yellow within the range, red above it. The bar has a red field at the right end. The temperature, the curve of the graph and the knob turn red above the range.
  - The Health chip uses the NE107 colours. Before, check function was cyan, off spec orange and maintenance yellow; now they are orange, yellow and blue.
- Updated: the READMEs of both demos, the Community page "Smart sensor on an ESP32" (guideline repository `0e3d812`) and the CHANGELOG.

## Verification

- `build.sh esp32 esp32s3 esp32s3-lcd147` and the Core2 `build.sh` build without warnings. Both boards are flashed.
- **Out of range:** Config `Set` `HotTemperature` 20 at 22.5 °C → `OFF_SPEC_3`/50 on the bus. Back to 30 → `NORMAL_0`/100.
- **Broken wire (PO, DHT22 data wire pulled):**
  - First version: `MAINTENANCE_REQUIRED_4`/25 on the bus, then `NORMAL_0`.
  - Final version: `FAILURE_1`/10 at 18:50:03 and `NORMAL_0`/100 at 18:50:13 (session clock).
  - The PO confirmed both displays ("Failure kam und ging wieder") and the red field on the Core2 ("alles passt").
- D7 `test` Level 3 without the proxy:
  - sensor: 39 pass, 0 fail, 16 skip;
  - Core2: 39 pass, 0 fail, 16 skip (one test that was skipped before now runs).
  - Both exit 4, INCONCLUSIVE, as before without the proxy.
- Gate green.

## Agent decisions (G-3.4 b)

- **A-683 Out of range = `OFF_SPEC_3`/50, display variant only.** "Out of specification" in NE107 is an uncertain signal of a running device, which fits a temperature beyond the configured range.
  - Only the display variant has the Config scale; the plain sensor has no range and stays `NORMAL_0`.
  - There is no hysteresis, so that colour, LED and Health always agree. A temperature right at the hot end may toggle the Health; that is acceptable for a demo that shows the colours.
- **A-684 Broken sensor = `FAILURE_1`/10** (after the PO's remark; first built as `MAINTENANCE_REQUIRED_4`/25 as requested).
  - MG 8.3.2: maintenance required is a *valid* signal, failure an *invalid* one; without readings the signal is invalid.
  - The HealthScore is 10, not 0: MG 6.3.5 makes `FAILURE_1` with score 0 of an application about itself mean "offline until the next Birth". The Registry would show the still-connected device as offline. The C-114 demo used `FAILURE_1`/0, which was wrong for that reason.
- **A-685 NE107 colours on both displays** (MG 8.3.2 Table 13): normal green, failure red, check function orange, off spec yellow, maintenance required blue. The Core2 scale is aligned with the S3 (red only out of range).

Nothing open for the PO. Next free ids: SF-120, A-686, GL-ADR-024, ADR-0036, Q-037, V-TOP-256.
