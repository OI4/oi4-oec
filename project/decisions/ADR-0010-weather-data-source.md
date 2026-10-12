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
