# C-115 – Smart sensor demos: OEC Config of both boards, LED colour at the marker

- Date: 2026-11-28 (wall clock of the session: 2026-11-13)
- Phase: none open (all phases closed after C-111); PO request (G-3.4 a: scope)
- Agent/Author: GitHub Copilot (Claude Opus 5.5)
- Status: done
- Baseline: guideline `980cde8` (unchanged); workspace before the cycle: `b93b7ce`

## Intent

PO request after C-114 addendum 1: "Die LED des ESP32-S3 soll immer die Farbe haben, die auf dem "Balken" unter der Gradzahl gerade ist, also von blau zu grün zu gelb zu rot. Zudem möchte ich sowohl den ESP32-S3 als auch den Core2 über die OEC Configuration konfigurieren können. Beim ESP32-S3 könnte ich mir die Temperatur Bereiche gut vorstellen. Für den Core2 darfst du dir was überlegen."

During the test the PO reported: "den DHT auf 25 erwärmt … Die Anzeige geht auf rot, die LED ist aber grün".

## Done

- **LED byte order (defect of C-114 addendum 1):** the RGB LED of the Waveshare ESP32-S3-LCD-1.47B takes its colour bytes in the order **R, G, B**, not G, R, B as a WS2812B. Red and green were swapped; blue is the same in both orders, which hid the defect in the addendum 1 check ("blue at cold" was right, "green" was read as plausible). Found with the scale set to 10–20 °C at 25 °C (screen pure red, LED green); after the fix the PO confirmed red (10–20 °C) and green (15–31 °C, middle of the scale). `led_set` now sends R, G, B and corrects each channel with gamma 2.2, so the LED shows the colour of the bar at the marker (blue → green → yellow → red) instead of a washed-out mix.
- **ESP32-S3 display board, Config context `Display`** (variant `esp32s3-lcd147` only): group `Scale` with `ColdTemperature` and `HotTemperature` (Number, °C, −40 … 80, defaults from Kconfig 15/30) and group `Led` with `Brightness` (0 … 255, 0 = off, default 48). The display and the LED follow a `Set` at once; the scale works in tenths of a degree.
- **M5Stack Core2, Config context `Display` extended** (agent's choice): `Brightness` of the display (5 … 100 %, default 50), `Cycle` (0 … 3600 s, 0 = only by button or tap; otherwise the next sensor after that many seconds) and the group `Scale` (`ColdTemperature`, `HotTemperature`, defaults 15/30). The temperature is drawn in the colour of the scale, with the same bar and marker as on the S3 board (grey when stale).
- **Persistence:** both demos keep their Config values in NVS (namespace `demo.<context>`), so they survive a restart; before, the values lasted until the next restart. On start a stored value is used only if it is valid against the current definition.
- READMEs of both demos, the Community page "Smart sensor on an ESP32" (guideline repository `0847016`) and the CHANGELOG updated.

## Verification

- `build.sh esp32 esp32s3 esp32s3-lcd147` and the Core2 `build.sh` without warnings (`-Werror`); both boards flashed.
- Config over MQTT (a `Get`/`Set` helper on mosquitto with the probe account): `Get`/`Set` of both contexts on both boards, `HotTemperature` 81 → `BadOutOfRange`, values unchanged after a reset. The PO checked both screens and the LED ("Ja, alles passt"; after the byte-order fix red and green as expected). Defaults restored afterwards (S3 15/30 °C, LED 48; Core2 15/30 °C, 50 %).
- D7 `test` Level 3 without the proxy: S3 board 39 pass, 0 fail, 16 skip; Core2 38 pass, 0 fail, 17 skip (exit 4, INCONCLUSIVE, as before without the proxy). One S3 run had a burst of unanswered `Get`s for about 20 s (T7-GET-03, T7-GET-08; a Wi-Fi gap); two reruns on the same firmware passed.
- Gate green.

## Agent decisions (G-3.4 b)

- **A-676 Config persistence in the demos, not in the Micro SDK:** the core validates every object of a `Set` (type, range, length) but leaves storing to the application (ADR-0030, A-463; `oecm.h`: "the application does it in on_config if needed"); the demos store the values in NVS per context and take a stored value only if it is valid. Not chosen: persistence in the core (a new PAL contract for every port, out of scope of a demo request).
- **A-677 Scale ends without a cross-field check:** the core has no hook to reject a `Set` with `ColdTemperature` ≥ `HotTemperature`; the demos use the lower value as the cold end, equal values span 1 °C. Not chosen: a new SDK callback for cross-field checks (API change for one demo).
- **A-678 Core2 Config:** `Brightness`, `Cycle` and the temperature scale – what a person at the display would set; the same scale as on the S3 board, so both show one temperature in one colour.

Nothing open for the PO. Next free ids: SF-120, A-679, GL-ADR-024, ADR-0036, Q-037, V-TOP-256.
