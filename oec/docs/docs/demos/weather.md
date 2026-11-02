---
title: Weather service
sidebar_position: 3
description: The demo weather service (D3) represents weather stations as assets with Data, Config, Health and warnings.
---

# Weather service

The weather service is an `OTConnector` that represents 1 … 5 weather
stations as assets of its own. Each station publishes its weather as `Data`,
has a `Config` with its location and raises the events `Frost` and `Storm`.
It is written in Go on the Go SDK.

| | |
|---|---|
| ServiceType | `OTConnector` |
| Messaging level | 3 – `Get` and `Set` of every resource of the application and of each station |
| Service Container level | 3 |
| Image | `oi4/oec-weather:0.1.0` |
| Web UI | `http://127.0.0.1:8782/`, read only |

## Start

- **Locally:** `go build -o weather/target/oec-weather ./weather` in
  `oec/demos`, then `OEC_ROOT=<container storage> weather/target/oec-weather`
  ([Build and run](../../../demos/weather/README.md#build-and-run)).
- **As an image:** `docker run` with the three read-only mounts of a
  container storage ([Image](../../../demos/weather/README.md#image)).
- **With compose:** together with the other demos, see
  [Run all three](index.md#run-all-three).

## Where the weather comes from

| Provider | What it does |
|---|---|
| `simulation` (default) | offline and deterministic: the same seed gives the same weather |
| `met-no` | the forecast of MET Norway (Locationforecast 2.0). Needs your e-mail address or URL in **User-Agent contact** and a poll interval of at least 600 s, as the terms of api.met.no require |

Weather data of `met-no`: **Based on data from MET Norway**, licensed under
CC BY 4.0. The Web UI shows this credit while `met-no` is active.

## Settings

Two kinds of OEC `Config`, both changed with the [config service](config.md)
or any other OEC client; they persist across restarts.

| Application (context `weather`) | Default |
|---|---|
| Weather data provider | `simulation` |
| User-Agent contact | none |
| Poll interval | 60 s |
| Stations (1 … 5) | 3 |
| Frost threshold / Storm threshold | 0 °C / 20.8 m/s |
| Simulation seed | 1 |

| Each station (context `station`) | Default of station 1 |
|---|---|
| Name | Frankfurt am Main |
| Latitude / Longitude | 50.1109 / 8.6821 |
| Altitude | 112 m |

A value out of range or `met-no` without a contact is refused with a `Bad`
status; nothing changes. All objects and rules:
[Configuration](../../../demos/weather/README.md#configuration).

## What it publishes

- `Data` `Oi4Pv` per station: temperature (`Pv`), humidity, pressure, wind
  speed and direction (`Sv1` … `Sv4`) with units in the `Metadata`.
- `Frost` and `Storm` events per station when a threshold is crossed.
- `Health`: a station without data is `FAILURE_1`, the application is then
  `OFF_SPEC_3` with a lower score. No values are invented.

Details: [Data and events](../../../demos/weather/README.md#data-and-events),
[Health](../../../demos/weather/README.md#health).

## Web UI

`http://127.0.0.1:8782/` shows the provider and the settings (read only),
and per station its location, Health, current values, warnings and a short
history of temperature and wind, live. Requests and limits:
[Web UI](../../../demos/weather/README.md#web-ui).

## Limits

- No authentication and no TLS on the Web UI (demo).
- At most 5 stations.
- `met-no` gives a forecast of the nearest hour, not a measurement, and
  needs internet access from the container.
