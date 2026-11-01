# ADR-0010 – Weather demo data source

- Status: accepted
- Date: 2026-10-12
- Deciders: product owner (Thomas), C-003
- Related: Q-005, G-4.7

## Context

The demo must work without an API account or key. Open-Meteo's free tier is restricted to non-commercial use; integration into commercial products or promotional activities is explicitly commercial. An alliance demo that companies copy into products is a grey zone.

## Decision

- Default provider: **MET Norway Locationforecast API** (`api.met.no`): no key, data under CC BY 4.0 (commercial use allowed).
- Weather sources are accessed via a **provider interface**, so other providers (e.g. Open-Meteo, Bright Sky/DWD) can be plugged in.
- **Offline simulation provider** for environments without internet access and for tests.

## Consequences

- MET Norway terms must be honored: identifying `User-Agent` (app name + contact/URL, configurable), caching and respecting `Expires`/`If-Modified-Since`, coordinates with max. 4 decimals, no burst requests, attribution (CC BY 4.0) in UI/docs/metadata.
- Terms were checked on 2026-10-12; recheck before release.

## Amendment C-081 (2026-11-01) – the provider `met-no` as built, terms rechecked (agent, G-3.4 b)

Details: `log/2026-11-01_C-081_weather-met-no.md`, REVIEW A-347 … A-352, `oec/demos/weather/README.md`.

1. **Terms rechecked on 2026-11-01** (api.met.no terms of service, licence page, Locationforecast documentation). The data is under NLOD 2.0 and CC BY 4.0, and commercial use is allowed. The source must be credited ("Based on data from MET Norway") and changes indicated; a link is appreciated. No "Yr" name or logo, no implied endorsement. The usage rules are unchanged from the Consequences above. Recheck again before a release (P9).
2. **Implementation** (`oec/demos/weather/metno.go`, Go standard library):
   - compact Locationforecast 2.0;
   - User-Agent `oec-weather/<version> <userAgentContact>`; no request without a contact;
   - cache with `Expires` and `If-Modified-Since`, at most 16 documents, no stale values;
   - `lat`/`lon` with at most 4 decimals and the altitude in whole metres;
   - 1 s between requests;
   - `429` pauses for `Retry-After`, at least 10 min; `203` warns once (A-347).
3. **Config:** `met-no` needs a `userAgentContact` and a `pollInterval` of at least 600 s. A `Set` that breaks this is refused, never clamped. A stored `Config` that breaks it starts with `simulation` (A-348).
4. **Attribution:** in the description of the `provider` Config object (metadata on the bus) and in the weather README; the Web UI follows in C-083 (A-351).
5. **Tests:** offline against an own `httptest` server. The live test against api.met.no is opt-in (`OEC_WEATHER_LIVE_CONTACT`) and was passed on 2026-11-01 (A-352).
