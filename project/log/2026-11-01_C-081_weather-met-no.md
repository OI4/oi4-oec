# C-081 – P6 increment 3: MET Norway provider of the weather service

- **Date:** 2026-11-01
- **Phase:** P6 – Demo services (D3, D4, D10), increment 3 of 7 (ADR-0028, A-334)
- **Baseline:** unchanged since C-074 review addendum 2. Guideline `52d8470` (branch `feature/spec-findings`, head `eb821e9`); SF-001 … 117 resolved, no open finding. 1411 inventory items, 444 REQs, 0 gaps, no REQ drafts. Schema 2.0.0 frozen, `2.0.1-dev`. 55 D7 and 17 D8 tests `implemented`, report versions 1 frozen.
- **Agent:** new session (hand-over prompt of C-080); the context was compacted once during the cycle

## Intent

Build increment 3 of P6: a second implementation of the provider interface of the weather service D3 (`oec/demos/weather`), the provider `met-no`. It reads the Locationforecast 2.0 of MET Norway with the HTTP client of the Go standard library and follows ADR-0010:

- an identifying User-Agent;
- caching with `Expires` and `If-Modified-Since`;
- coordinates with at most 4 decimals;
- no bursts;
- a poll interval of at least 600 s;
- attribution under CC BY 4.0.

`met-no` joins the `provider` OneOf and can be switched by `Set` at run time. Provider failures show in the `Health` of the stations and of the application, and the service never publishes invented values. All tests run offline against an own `httptest` server. The live run against api.met.no is opt-in only. No Web UI (C-083), no config service (C-082).

Nothing was open for the PO and no answer arrived. No split: the cycle takes increment 3 only.

## Plan

1. Check the baseline (unchanged since C-078; C-080 ended green).
2. Recheck the terms of MET Norway (ADR-0010: "recheck before release"): terms of service, licence page, ForecastJSON documentation.
3. Tests first:
   - unit tests of the provider against a fake MET server;
   - settings tests;
   - an integration test of the service with the fake server.
4. Implement `metno.go`, the Config rules in `settings.go`, the Health in `service.go` and the test endpoint in `main.go`.
5. Opt-in live run; READMEs; gate; log, STATUS, ROADMAP, ADR-0010/0028 amendments, REVIEW/REVIEW_DE; hand-over.

## Work

### 1. Baseline

The baseline matches the hand-over prompt: guideline `52d8470`, branch head `eb821e9`. Nothing changed since C-080 (`da40e92`).

Before writing code I checked these facts in the SDK and in D7:

- **Empty values:** the Rust core skips every check of a `Config` value for `""` (`check_value`), so an empty `userAgentContact` passes a Pattern.
- **Set Config in D7:** the tests write back the unchanged values and send negatives (unknown object, renamed `Name`, a non-number). D7 never selects another OneOf value, so `met-no` cannot change the D7 run. The 49 pins stay.
- **Set check of the SDK:** the SDK refuses a changed `Description`/`Validation` with `BadNotWritable`. The integration test therefore sends `Value` fields only.

### 2. Terms of MET Norway (recheck of ADR-0010, A-351)

Rechecked on 2026-11-01 (api.met.no terms of service, licence page, Locationforecast documentation):

- **Licence:** the data is licensed under NLOD 2.0 and CC BY 4.0. Commercial use is allowed.
- **Attribution:** credit MET Norway as the source, e.g. "Based on data from MET Norway", and indicate changes. A link to the source is appreciated. Using the "Yr" name or the logos, or implying an endorsement, is not allowed.
- **Usage rules:**
  - an identifying User-Agent (product and contact; a missing or generic one gives `403`);
  - no request before `Expires`, then `If-Modified-Since` (`304`);
  - at most 4 decimals of the coordinates;
  - no bursts (traffic spread over time);
  - throttling on `429`; `203` marks a deprecated product.

This matches ADR-0010. Nothing about the licence or the attribution is unclear, so there is no D-x. The Web UI attribution follows in C-083 (ADR-0028 §3).

### 3. Tests first

These were written before the implementation and failed at first (missing symbols).

`metno_test.go` (unit tests; a fake MET server built on `httptest`):

- The fake server answers `403` without the expected User-Agent, so a wrong User-Agent is detected. It records every request and can send `200`/`203`/`304`/4xx/5xx, `Expires`/`Last-Modified`/`Retry-After`, delays and arbitrary bodies.
- The tests use a logical clock and `Spacing = 0`.

| Test | Checks |
|---|---|
| `TestMetNoUserAgent` | `oec-weather/<version> <contact>`; an empty, spaced, non-ASCII, CR/LF-injecting or overlong contact is refused before any request |
| `TestMetNoQuery` | `lat`/`lon` rounded to 4 decimals, no `-0`, altitude whole |
| `TestMetNoObserve` | mapping to `Oi4Pv` (A-343) from the entry nearest to the time, rounded to the resolution, wind direction mod 360 |
| `TestMetNoCache` | no request before `Expires`; then `If-Modified-Since` = `Last-Modified`; `304` keeps the document and renews `Expires` |
| `TestMetNoNoExpires` | a response without `Expires` is expired at once |
| `TestMetNoErrors` (table) | 400, 404, 500, 503; a wrong path (404); broken JSON; HTML instead of JSON; no timeseries; no entry within 1 h; wrong or missing units; a missing value; a value outside the Range; `304` without a cached document. Each one gives an error and no values. After a failed renewal (502) no stale values are served; they come back after recovery |
| `TestMetNoForbidden` | `403` names the User-Agent |
| `TestMetNoThrottled` | `429` pauses every request for `Retry-After`, at least 10 min |
| `TestMetNoDeprecated` | `203` gives one notice |
| `TestMetNoTimeout` | a slow server ends the request at the context deadline with an error and no values |
| `TestMetNoSpacing` | at least `Spacing` between two requests |
| `TestMetNoCacheBound` | at most 16 cached documents (LRU) |
| `TestMetNoBodyLimit` | at most 4 MiB per response |
| `TestMetNoLive` | opt-in (`OEC_WEATHER_LIVE_CONTACT`), never in the gate |

`settings_test.go`:

- `met`, `metno` and `SIMULATION` are refused as `provider`; a spaced, CR/LF or non-ASCII `userAgentContact` is refused.
- The Pattern of `userAgentContact` is checked with Go `regexp`.
- `TestParseSettingsMetNo`:
  - `met-no` without a contact gives `BadInvalidArgument`;
  - `met-no` with a `pollInterval` below 600 s, also the default 60 s, gives `BadOutOfRange`;
  - in both cases the provider falls back to `simulation` (the rule for stored values);
  - `met-no` with a contact and 600 s is accepted.

`weather_test.go` – `TestMetNoProvider` (integration, against the `-race` build on `go-testbroker` with `OEC_WEATHER_METNO_URL` → an in-test fake server):

1. The service refuses four `Set`s with Status Bad: no contact, 60 s, a spaced contact, and 599 s. No request reaches the server.
2. Switch to `met-no` at run time: three requests with the right User-Agent, at least 900 ms apart, the right query, and the `Data` of each station from the server.
3. Oslo fails (`500`):
   - station 2 gets `FAILURE_1`/0, and the application `OFF_SPEC_3`/66;
   - a Syslog `Warning` and a stdout line are written;
   - stations 1 and 3 go on, and station 2 publishes no `Data`.
4. Recovery: station 2 `NORMAL_0`/100, application `NORMAL_0`/100, Syslog `Notice`, `Data` again.
5. Every request fails: every station `FAILURE_1`/0, application `OFF_SPEC_3`/0.
6. Back to `simulation`: every station `NORMAL_0`, `Data` from the simulation, no further request.
7. Over the whole test, the application never publishes `FAILURE_1` about itself.

`TestSetConfigRefused` now uses `provider=met` as its unknown value, because `met-no` is valid.

### 4. Implementation (`oec/demos/weather`, A-347 … A-350)

- **`metno.go` – the provider `met-no`** (A-347):
  - **Endpoint** `https://api.met.no/weatherapi/locationforecast/2.0/compact`. `NewMetNo` refuses an empty or invalid contact with a message that names `userAgentContact`.
  - **HTTP client:** a clone of `http.DefaultTransport` with a timeout of 30 s. `Accept-Encoding` stays unset, so the transport asks for gzip and decompresses.
  - **Cache** keyed by URL: no request before `Expires`; after that a conditional request with `If-Modified-Since`. A response without `Expires` counts as expired. LRU with at most 16 documents. An expired document is kept for `If-Modified-Since` only, never for values.
  - **Spacing:** at least 1 s between two requests (real time, aware of the context). A poll round therefore spreads its stations.
  - **Throttling:** `429` stops every request until `Retry-After`, at least 10 min. `403` names the User-Agent in the error. `203` gives one Syslog `Warning` (deprecated product).
  - **Response checks:** at most 4 MiB per body. The units of the document are checked. The forecast entry nearest to the poll time (at most 1 h away) is mapped to `Oi4Pv`. Every value must be present and inside the Range of its field, then it is rounded like the simulation.
- **`settings.go`** (A-348):
  - `provider` OneOf `simulation`, `met-no`, with a description that carries the attribution.
  - `userAgentContact` gets the Pattern `^[!-~]+$` (printable ASCII without spaces, so no header injection).
  - Rules across objects: `met-no` needs a contact (`BadInvalidArgument`) and a `pollInterval` of at least 600 s (`BadOutOfRange`).
  - A proposal that breaks a rule is refused. A stored value that breaks a rule falls back to `simulation` with a note on stderr.
- **`service.go`** (A-349):
  - `newProvider` is a method; the `MetNo` instance is kept while the contact stays the same, so its cache, spacing and throttling survive a provider change.
  - `poll` counts the good stations:
    - `stationHealth` sets `FAILURE_1`/0 or `NORMAL_0`/100 on a change, with a stdout line and a Syslog `Warning`/`Notice` of the application;
    - `appHealth` sets `NORMAL_0`/100 or `OFF_SPEC_3` with `100·good/n`.
  - A station without an observation gets no `Data` update.
- **`main.go`:** `OEC_WEATHER_METNO_URL` replaces the endpoint for tests, with a note on stderr (A-350).

No Go SDK or C ABI gap:

- `Asset.SetHealth`, `App.Syslog` and `State` are in the SDK.
- The SDK and the ABI have no Health `Details`, so the reason of a failure goes into the Syslog event and stdout instead (A-349; extending the ABI only for this is not worth it).
- No amendment to ADR-0027 or ADR-0021.

### 5. Opt-in live run against api.met.no

Run on 2026-11-01 from `oec/demos` with `OEC_WEATHER_LIVE_CONTACT=https://github.com/OI4/oi4-oec` (the project URL, no personal address): `go test -count=1 -run TestMetNoLive -v ./weather` → **PASS** (1.3 s). Results:

| Station | Values (14:00 UTC) | Expires | Last-Modified |
|---|---|---|---|
| Frankfurt am Main | 9.9 °C, 60.8 %, 1030.8 hPa, 2.5 m/s from 39° | 14:27:21Z | 13:55:22 GMT |
| Oslo | 9.7 °C, 84.8 %, 1026.4 hPa, 0.5 m/s from 213° | 14:25:36Z | 13:55:24 GMT |

The second `Observe` of each station was served from the cache (no request before `Expires`). The real `Expires` is about 30 min after the response, so a poll interval of 600 s gives mostly cache hits and a conditional request about every 30 min per station.

### 6. D7 `test --level 3`

Unchanged: **49 pass, 6 skip, 0 fail** with the simulation (the default). The pass list did not change.

## Defects found and fixed (test first)

None in the SDKs, `oec-ffi` or D7. In the new code: the first run of `TestMetNoProvider` refused the switch to `met-no` with `BadOutOfRange`. The cause was a stale binary in `weather/target/` built from the C-080 code, not the code itself. After `go build -race` the test passed. `oec/demos/check.sh` always rebuilds the binary, so the gate cannot hit this.

## Agent decisions (G-3.4 b, for information: REVIEW A-347 … A-352)

- **A-347 Provider `met-no`:**
  - Compact Locationforecast 2.0, User-Agent `oec-weather/<version> <userAgentContact>`.
  - Cache with `Expires` and `If-Modified-Since`, at most 16 documents; no stale values.
  - 1 s between requests; 30 s timeout; 4 MiB per response.
  - `429` pauses for `Retry-After`, at least 10 min; `403` names the User-Agent; `203` warns once.
  - Values from the forecast entry nearest to the poll time (≤ 1 h), checked against unit and Range, rounded as in A-343.
  - "Forecast as observation": the Locationforecast has no measurements, so the nearest forecast hour stands for "now". The README says so.
- **A-348 Config rules (A-302 made concrete):**
  - `provider` OneOf `simulation`/`met-no`; `userAgentContact` Pattern `^[!-~]+$`.
  - **Refuse, do not clamp:** `met-no` without a contact → `BadInvalidArgument`, `met-no` with a `pollInterval` below 600 s → `BadOutOfRange`. The `Config` always shows what the service does; clamping would publish 60 s while polling every 600 s.
  - A stored `Config` that breaks the rules (e.g. edited by hand or from an older version) starts with `simulation` and a note on stderr, never with an unthrottled `met-no`.
  - The 600 s bound is only in the description, not in the `Validation` of `pollInterval`, because it depends on another object.
- **A-349 Health:**
  - A station without data: `FAILURE_1`/0. The application with k of n stations without data: `OFF_SPEC_3` with `100·(n−k)/n` (rounded down); `NORMAL_0`/100 when all are good.
  - The application never reports `FAILURE_1` about itself, because it stays available (REQ-LIFE-012/017), and REQ-LIFE-014 forbids `NORMAL_0` with HealthScore 0.
  - Stations are assets of their own, so the order of priority (SF-096) applies per asset; each asset has only one condition at a time.
  - Each transition is a Syslog event of the application (`Warning` on failure, `Notice` on recovery, only while online) and a stdout line.
  - No Health `Details`: neither the Go SDK nor the ABI has them, and adding them only for this is not worth an ABI change.
  - The last `Data` of a failed station stays readable with `Get`; its Health says it is not valid.
- **A-350 Test endpoint:** `OEC_WEATHER_METNO_URL` (environment, with a note on stderr) replaces the endpoint for tests. It is deliberately no `Config` object: a user must not point the service at another endpoint over the bus, and the `Config` stays free of test switches.
- **A-351 Attribution and terms (recheck of ADR-0010):**
  - "Based on data from MET Norway (Locationforecast 2.0, api.met.no), licensed under CC BY 4.0" with the link to the licence and the changes (nearest hour, mapped to `Oi4Pv`, rounded).
  - Placed in the description of the `provider` object (metadata on the bus), in `oec/demos/weather/README.md` and in the code. The Web UI follows in C-083.
  - No "Yr", no logo, no endorsement.
- **A-352 Tests and gate:**
  - Unit tests against the fake MET server, the settings tests, and the integration test `TestMetNoProvider`.
  - The live test is opt-in (`OEC_WEATHER_LIVE_CONTACT`) and never in the gate.
  - D7 L3 unchanged: 49 pins with the simulation.

## Changes to reviewed items

None. No fixture, vector or REQ changed, and no schema change.

## Spec findings

None (next SF-118). The Health mapping follows MG 8.3.2 and REQ-LIFE-012/014/017 without a gap.

## Dependencies

None new. Go: standard library only (`net/http`, `net/http/httptest`) plus the Go SDK of this workspace. C/Rust: none. The MET Norway data is not a code dependency; its licence is recorded in A-351 and ADR-0010.

## D7 and D8 unchanged

No change under `oec/conformance`, the SDKs or `oec-ffi`.

## Risks and open technical points

- The fake server mirrors the documented API (ForecastJSON compact, cache headers). A change of the real API shows only in the opt-in live test; `203` is reported as a Syslog `Warning`.
- The spacing of 1 s makes a poll round with 5 stations take up to ~5 s when every document has expired. That is harmless with ≥ 600 s.
- A long throttling (`429`) shows as `FAILURE_1` of every station and `OFF_SPEC_3`/0 of the application until it ends. This is intended: the values would be invented otherwise.
- The contact is personal data of the operator. It is stored in the `Config` of the application and visible to every `Get Config` on the bus. It is not masked, because it is not a secret (masking per GL-ADR-021 is for sensitive values such as credentials). Noted for the config service and the Web UI.

## Commits

- `385f162` – the provider `met-no`, the Config rules, the Health, the tests, the READMEs
- this commit – docs: log, ADR-0010 amendment C-081, ADR-0028 amendment C-081, ROADMAP (C-081 ✅), STATUS, REVIEW/REVIEW_DE (A-347 … A-352)

## Checks (final)

`oec/check.sh` → "all checks passed" (demos step: `oec/demos` ok, `logger` incl. D7 L2, `weather` incl. D7 L3 and `TestMetNoProvider`).

## For the PO

Nothing to decide: no R-x (no new test data with a verdict), no D-x (the licence and the attribution are clear), no SF. A-347 … A-352 are for information. The most visible ones:

- `met-no` is refused, not clamped, below 600 s or without a contact (A-348);
- the Health mapping: a station without data `FAILURE_1`, the application `OFF_SPEC_3` (A-349);
- the attribution and the recheck of the MET Norway terms (A-351).

## Handover (G-3.10)

**New agent session** for C-082. The config service is new material: Rust on `oec`, its own Cargo package `oec/demos/config`, D7 `level1`, the consumer side of the Rust SDK, and an integration test in which it edits the logger. This session's context was compacted once. `prompt.md` (workspace root, German, not committed) holds the start prompt of C-082.

## Next

C-082 – config service core D10 in Rust on `oec`:

- discovery over `Pub/MAM`;
- `Get`/`Set Config` of the other applications (A-304), with the `Event` Status of each `Set`;
- D7 `test` at `level1` in the gate;
- an integration test in which it edits the logger (and the weather service).
