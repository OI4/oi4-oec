# C-080 – P6 increment 2: weather service core D3 in Go

- **Date:** 2026-11-01
- **Phase:** P6 – Demo services (D3, D4, D10), increment 2 of 7 (ADR-0028, A-334)
- **Baseline:** unchanged since C-074 review addendum 2. Guideline `52d8470` (branch `feature/spec-findings`, head `eb821e9`); SF-001 … 117 resolved, no open finding. 1411 inventory items, 444 REQs, 0 gaps, no REQ drafts. Schema 2.0.0 frozen, `2.0.1-dev`. 55 D7 and 17 D8 tests `implemented`, report versions 1 frozen.
- **Agent:** new session (hand-over prompt of C-079)

## Intent

Build increment 2 of P6: the core of the weather service D3, in Go on the Go SDK (ADR-0027). It is `package main` in `oec/demos/weather` of the module `github.com/OI4/oec/demos`, as fixed by ADR-0028. Scope:

- an `OTConnector` at Messaging Level 3;
- a provider interface with the offline, deterministic simulation provider;
- weather stations as represented assets, each with `Data` `Oi4Pv` and the `Metadata` derived from it, and a station `Config`;
- the application `Config` of A-302.

Tests come first, then offline integration tests with `internal/demotest`, then D7 `test --level 3` against the service in the gate. No MET Norway (C-081) and no Web UI (C-083). Nothing was open for the PO and no answer arrived.

**Split (allowed by ROADMAP P6):** the Frost/Storm warnings planned for C-081 were small and need no network, so C-080 takes them as well. C-081 keeps the MET Norway provider (opt-in live, ADR-0010 terms), the provider `Health`, the attribution, `met-no` in `provider`, and the rule of at least 600 s for `pollInterval` with `met-no`.

## Plan

1. Check the baseline (unchanged since C-078; C-079 ended green).
2. Write the tests: unit tests (simulation, settings, Config definitions, identity helpers) and integration tests against the test broker (defaults, stations, poll interval, station Config, refused Sets, warnings, restart, D7 L3).
3. Implement `provider.go`, `simulation.go`, `settings.go`, `service.go` and `main.go`; extend `go.mod` with `require`/`replace` of the Go SDK.
4. Pin the D7 L3 pass list; extend `oec/demos/check.sh` with the weather service.
5. Update the READMEs, this log, STATUS, ROADMAP, ADR-0028 (amendment) and REVIEW/REVIEW_DE, then do the hand-over.

## Work

### 1. Baseline

The baseline matches the hand-over prompt:

- guideline `52d8470`, branch head `eb821e9` (`findings/` only);
- 1411 items, 444 REQs, 158 `schema`/`d7-validate` REQs, 0 gaps;
- fixtures 29/59/19/29/69/26/20, all reviewed;
- 55 D7 and 17 D8 tests `implemented`;
- 29 entry schemas, 245 payload vectors and 18 `broker.json` vectors;
- Go SDK: 44 of 45 ABI functions wrapped;
- logger D7 L2: 46 pass.

C-079 ended with the gate green, and nothing changed since. Before writing code I checked these facts:

- **Hostname:** the SDK reads the container name from `proc/sys/kernel/hostname`, then `etc/hostname`, below the storage root (`oec/src/container.rs`).
- **Config Pattern:** the SDK applies `Pattern` to every type. The regex is unanchored, so the service uses `^…$`.
- **Set of a station's `Data`:** with no `OnSet`, the `Data` stays read-only. A `Set` of it is answered with `BadNotWritable`, which T7-SET-03 accepts (REQ-CMP-033 only fails on `BadNotSupported`).
- **Asset close:** MG defines no Close message for a represented asset. The Close message has Source = AppId (REQ-LIFE-006/007).

### 2. Tests first

These were written before the implementation and failed at first (missing symbols).

`simulation_test.go`:

- determinism by seed and location, where the name has no influence;
- every value within the Range of its field and rounded to its resolution (seeds 0–3, the whole globe, altitudes −400 … 8800, one year);
- plausible climate:
  - Frankfurt January mean −6 … 6 °C, July 14 … 24 °C;
  - Oslo colder than Frankfurt;
  - Zugspitze at least 12 K colder than Frankfurt;
  - Sydney's seasons reversed;
  - more wind on the Zugspitze;
  - Frankfurt mean pressure 1000 … 1025 hPa;
- smoothness from minute to minute;
- a canceled context returns an error.

`settings_test.go`:

- the defaults equal the Config definition;
- `parseSettings` refuses unknown or invalid values with a Bad `StatusCode`;
- station defaults 1 … 5, and `parseLocation` with its limits and decimal places;
- the Patterns checked with Go `regexp`;
- `hostname` (etc, proc overrides, trim) and `stationSerial`.

`weather_test.go` (`//go:build unix`), the integration tests against the ASan-free `-race` build on `go-testbroker`:

- `TestDefaults`: the application Config, the MAM of the application and of the stations, station Health `NORMAL_0`/100, the station Config, `Metadata` `Oi4Pv` with a name and description per field, `Data` within the Ranges, no fourth station.
- `TestStations`: 5 stations, then 1 (the `PublicationList` without stations 2 … 5, no further publication of them), then 2 again.
- `TestPollInterval`: 1 s gives several poll rounds.
- `TestStationConfig`: Singapore accepted and the `Data` follows; latitude 91, longitude with 5 decimals, altitude 12.5 and a 65-character name refused.
- `TestSetConfigRefused`: `met-no`, 6 stations, `pollInterval` 0, seed 1.5 and `frostThreshold` −60 are refused with Status Bad and do not take effect.
- `TestWarnings`: thresholds that are always crossed give Frost/Storm with `Number` 1 at every station; thresholds that are never crossed give `Number` 0.
- `TestRestartKeepsConfig`: after SIGTERM (exit 0, Close message) and a restart there are 4 stations, and station 1 keeps the name "Home".
- `TestD7Level3`.

### 3. Implementation (`oec/demos/weather`, A-341 … A-346)

- **`provider.go`:**
  - `Location`, `Observation`, and `Provider` (`Name`, `Observe(ctx, Location, time)`);
  - the five fields of `Oi4Pv`: `Pv` temperature °C, `Sv1` relative humidity %, `Sv2` pressure at sea level hPa, `Sv3` wind speed m/s, `Sv4` wind direction °;
  - each field with Range and Precision, from which the SDK derives the `Metadata` (A-343).
- **`simulation.go`:** the offline provider. It is a pure function of seed, coordinates, altitude and time:
  - climate by latitude, a seasonal cycle by hemisphere, a diurnal cycle with its maximum at 15:00 local solar time;
  - lapse rate 6.5 K/km;
  - smooth value noise (SplitMix64 hash, smoothstep) on a slow and a fast time scale per quantity;
  - clamped to the Range and rounded to 0.1 (wind direction to 1°).
- **`settings.go`:** the two Config definitions (A-342) and their parsing:
  - `""` means the default (REQ-RES-CONFIG-010);
  - the definition's checks are repeated for stored values;
  - `hostname(root)` and `stationSerial`.
- **`service.go`:** one poll goroutine owns the stations and makes every SDK call after the start (A-344).
  - The `OnSet` handlers run on the SDK thread. They only check the proposal, hand it over under a mutex and wake the goroutine.
  - An accepted `Set` takes effect in an immediate poll round: provider, number of stations (`AddAsset`/`Remove`), locations, then `Observe` → `DataSet.Update` → warnings.
  - The first round waits for the connection.
  - Warnings are evaluated only while connected (A-345).
- **`main.go`:** identity (A-341), `Level3`/`OTConnector`, own Health, `signal.NotifyContext`, `App.Run`. The poll goroutine is stopped and awaited before `Close`.
- **`go.mod`:** `require github.com/OI4/oec/sdk/go v0.0.0` with `replace … => ../sdk/go` (ADR-0028 §1). There are no external modules, so there is no `go.sum`.
- **`oec/demos/check.sh`:**
  - `go run ./cmd/oec-buildlib` in `oec/sdk/go` (puts `liboec_ffi.a` for cgo into `lib/<GOOS>_<GOARCH>`, ignored by git);
  - `go build -race` of the service into `weather/target/`;
  - `OEC_WEATHER` for `go test -race ./...`.
  - The header of `oec/check.sh` names the new content of step 7.

No Go SDK or C ABI gap. Everything needed is in the SDK: `AddAsset`/`Remove` while running, `AddConfig` with `OnSet`, `AddData`/`Update`, `EmitEvent`, `State`. The known gaps were worked around or not needed:

- A-324 (no IL getter): the hostname is read like the SDK does.
- A-326 (no cancel) and A-327 (handle lifetime): not needed.

No amendment to ADR-0027 or ADR-0021.

### 4. D7 `test --level 3` against the weather service

Result: **49 pass, 6 skip, 0 fail, exit 0** (~46 s). This is the same set as against the Go SDK example `source`. The 49 passes are pinned in `weather_test.go`. D7 skips six tests:

- T7-MON-07 (REQ-ACT-011), T7-MON-17 (REQ-RES-CONFIG-011), T7-ADR-01 (REQ-BUS-002) and T7-ADR-02 (REQ-TOP-004) judge requests or publications the CUT does not make: the weather service sends no request and publishes nothing under `Oi4`.
- T7-SET-04 (REQ-BUS-007) and T7-SET-10 (REQ-RES-REFDES-002) need a ReferenceDesignation object, which the fresh store of the run does not have.

T7-SET-03 passes: D7 sets `Config`, `Data`, `Metadata` and `ReferenceDesignation` of a represented station and of the application. A `Set` of the station `Data` is answered with `BadNotWritable`, which is not `BadNotSupported`.

`oec/demos/check.sh` takes ~57 s now, up from ~45 s; the logger and weather D7 runs overlap (`t.Parallel` per package).

## Defects found and fixed (test first)

None in the SDKs, `oec-ffi` or D7. In the new code:

- **First poll before the connection.** The first poll round ran before the connection. The SDK drops events while offline, so an initial Frost begin was lost. `TestWarnings` found it: Zugspitze is below 0 °C with the defaults. The goroutine now waits for `Online` before the first round, and warnings are evaluated only while connected, so a change during a connection loss is reported after the reconnect.
- **Two test errors:**
  - `TestStations` expected the `PublicationList` topic with a `Source`, but the complete list is published without one.
  - `TestWarnings` looked for the Frost begin only after the `Set`, but a warning may already be on with the defaults.
- **Late `OnSet` after removal.** A late `OnSet` of a removed station could have moved a re-added station with the same index. Pending moves are now keyed by the station object, not the index. Found in review, no test.

## Agent decisions (G-3.4 b, for information: REVIEW A-341 … A-346)

- **A-341 Identity:**
  - Application: `oi4.org` / `OEC Weather` / `OEC-WEATHER`, Manufacturer "Open Industry 4.0 Alliance", SoftwareRevision 0.1.0, `OTConnector`, Level 3, SerialNumber = hostname of the container storage (Options.SerialNumber unset).
  - Stations: `oi4.org` / `OEC Weather Station` / `OEC-WEATHER-STATION`, SerialNumber `<hostname>-station-<n>`. That makes it unique per container and stable for an index, so a station keeps its persisted Config across restarts and across removal and re-adding. Health `NORMAL_0`/100.
  - **A-302 corrected:** stations "leave with their Close" is not possible. MG defines no Close message for a represented asset (the Close message has Source = AppId, REQ-LIFE-006/007). A removed station leaves the `PublicationList` (REQ-RES-PUBLIST-017) and publishes nothing more. Not a gap in the guideline: a consumer sees the change in the `PublicationList`.
- **A-342 Config objects (A-302 made concrete), group `common`:**
  - Application, context `weather`:
    - `provider` (OneOf `simulation`; `met-no` comes with C-081);
    - `userAgentContact` (≤ 256);
    - `pollInterval` (1 … 86400 s, whole, default 60);
    - `stations` (1 … 5, default 3);
    - `frostThreshold` (−50 … 50 °C, default 0);
    - `stormThreshold` (0 … 100 m/s, default 20.8 = Beaufort 9);
    - `simulationSeed` (0 … 999999999, default 1).
  - Station, context `station`:
    - `name` (≤ 64);
    - `latitude`/`longitude` (WGS 84, at most 4 decimals by Pattern, MET Norway terms);
    - `altitude` (−500 … 9000 m, whole).
    - Defaults: Frankfurt am Main, Oslo, Zugspitze, Hamburg, Munich.
  - Deviation from A-302: `altitude` is not "optional" but has a default per station, because `""` resets to it.
  - Patterns are anchored (`^…$`), because the SDK applies them unanchored.
  - A `Set` is applied completely or refused (SDK); the service repeats the checks for stored values and falls back to the default with a message on stderr.
- **A-343 Mapping to `Oi4Pv` and the simulation:**
  - `Pv` = air temperature, `Sv1` = relative humidity, `Sv2` = air pressure at sea level, `Sv3` = wind speed, `Sv4` = wind direction (from).
  - Range and Precision per field. The `Metadata` is derived by the SDK, and the default Mode publishes a field self-initiated on a change of at least its Precision.
  - The simulation is deterministic (seed, coordinates, altitude, time; not the name) and offline (G-4.7, ADR-0010).
- **A-344 Worker design:** one poll goroutine owns the stations and every SDK call after the start.
  - Handlers only check the proposal and hand it over, so they do not block the SDK thread.
  - An accepted `Set` takes effect at once in a poll round.
  - The first round waits for the connection.
  - Shutdown: the context is canceled, the goroutine ends, then `App.Close`.
  - One line per change and per round on stdout, which the tests read.
- **A-345 Warnings (pulled into C-080):**
  - Frost when the temperature is ≤ `frostThreshold`; Storm when the wind speed is ≥ `stormThreshold`.
  - A Generic event of the station `Pub/Event/<station>/Generic/Frost` (or `Storm`), with `Number` 1 when the warning begins and 0 when it ends, a `Description` with the station name, and `Details` with the value and the threshold.
  - Evaluated only while connected (events are dropped offline). A warning that is on at the start is reported with the first round.
- **A-346 Tests and gate:** unit and integration tests in `oec/demos/weather`; `oec/demos/check.sh` builds `liboec_ffi.a` for cgo and the service with `-race`; D7 L3 pinned (49).

## Changes to reviewed items

None. No fixture, vector or REQ changed, and no schema change. A-302 (an agent proposal for information, not test data) is made concrete and corrected by A-341/A-342.

## Spec findings

None (next SF-118). The missing Close of a represented asset is not a gap (A-341).

## Dependencies

None new. Go: standard library and the Go SDK of this workspace (`replace ../sdk/go`); no external module, no `go.sum`. C/Rust: none.

## D7 and D8 unchanged

No change under `oec/conformance`, the SDKs or `oec-ffi`.

## Risks and open technical points

- The simulation is plausible, not realistic: there are no fronts and no correlation between quantities beyond the diurnal cycle. That is enough for a demo; MET Norway follows in C-081.
- A removed station keeps its persisted Config in the store of the application. It reappears with it when added again (intended, documented in the README).
- Stdout lines are the interface for the tests. The Web UI (C-083) gets its data from the service itself.
- `oec/demos/check.sh` also runs `oec-buildlib` (a cached cargo build); in the gate the SDK step before it has already built it.

## Commits

- `06e6167` – weather service core (Go), tests, D7 L3, `oec/demos/check.sh`, READMEs, the `oec/check.sh` header
- this commit – docs: log, ADR-0028 amendment C-080, ROADMAP (C-080 ✅), STATUS, REVIEW/REVIEW_DE (A-341 … A-346)

## Checks (final)

`oec/check.sh` → "all checks passed" (demos step: `oec/demos` ok, `logger` incl. D7 L2, `weather` incl. D7 L3).

## For the PO

Nothing to decide: no R-x (no new test data with a verdict), no D-x, no SF. A-341 … A-346 are for information. The most visible ones:

- the station identity and the correction of A-302: no Close of a station, it leaves the PublicationList (A-341);
- the Config objects (A-342);
- the warnings pulled from C-081 into C-080 (A-345).

## Handover (G-3.10)

**New agent session** for C-081. MET Norway brings other material: HTTP client (standard library), ADR-0010 terms (User-Agent, `Expires`/`If-Modified-Since`, ≥ 600 s, attribution), opt-in live test and an offline HTTP test server. This session's context was compacted. `prompt.md` (workspace root, German, not committed) holds the start prompt of C-081.

## Next

C-081 – weather: the MET Norway provider (ADR-0010; opt-in live, offline tests against an own HTTP test server), provider `Health`, `met-no` in `provider`, ≥ 600 s rule, attribution.
