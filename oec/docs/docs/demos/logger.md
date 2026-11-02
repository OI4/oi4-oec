---
title: Logger
sidebar_position: 2
description: The demo logger (D4) logs the Event, Health and Data publications of all participants and shows them in its Web UI.
---

# Logger

The logger listens to the bus and writes one record for every `Event`,
`Health` and – if you turn it on – `Data` publication of **all**
participants. It is written in C on the C ABI of the SDK and shows the
consumer side of the SDK.

| | |
|---|---|
| ServiceType | `Utility` |
| Messaging level | 2 – it offers `Get` and `Set` of its own `Config` |
| Service Container level | 3 |
| Image | `oi4/oec-logger:0.1.0` |
| Web UI | `http://127.0.0.1:8781/`, read only |

## Start

- **Locally:** `make` in `oec/demos/logger` with `liboec_ffi.a` of the Rust
  SDK, then `OEC_ROOT=<container storage> target/oec-logger`
  ([Build and run](../../../demos/logger/README.md#build-and-run)).
- **As an image:** `docker run` with the three read-only mounts of a
  container storage ([Image](../../../demos/logger/README.md#image)).
- **With compose:** together with the other demos, see
  [Run all three](index.md#run-all-three).

## Settings

The settings are the OEC `Config` of the logger (context `logger`). Change
them with the [config service](config.md) or any other OEC client; they
persist across restarts.

| Setting | Default | Meaning |
|---|---|---|
| Log Events / Log Health / Log Data | on / on / off | which resources it subscribes to |
| Sources | all | only these applications or assets (and the ones below them) |
| Minimum Event level | `Informational` | less severe events are dropped |
| Output format | `json` | one JSON object or one text line per record |
| Output file | none | a file in `/opt/oi4/app` the records are appended to as well |
| History size | 1000 | records kept for the Web UI |

All objects with their limits:
[Configuration](../../../demos/logger/README.md#configuration).

## Web UI

`http://127.0.0.1:8781/` shows the last records with filters (resource,
Event level, text), live; small charts of the numeric `Data` values and of
the HealthScore; and the settings in force (read only – change them through
the `Config`). Requests and limits:
[Web UI](../../../demos/logger/README.md#web-ui).

## Limits

- No authentication and no TLS on the Web UI (demo).
- The records in memory are lost at a restart; use the output file to keep
  them.
- It subscribes to everything on the bus; on a busy bus use **Sources** and
  **Minimum Event level**.
