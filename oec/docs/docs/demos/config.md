---
title: Config service
sidebar_position: 4
description: The demo config service (D10) finds the OEC applications on the bus and edits their Config with Set Config.
---

# Config service

The config service is a generic configuration tool for every OEC application
with a `Config` resource. It finds the applications on the bus, reads their
`Config`, shows which `Config` is writable and changes values with
`Set Config`. It reports the `Status` event that the target answers with. It
is written in Rust on the SDK crate `oec`.

| | |
|---|---|
| ServiceType | `Utility` |
| Messaging level | 1 – it only sends requests and answers none |
| Service Container level | 3 |
| Image | `oi4/oec-config:0.1.0` |
| Web UI | `http://127.0.0.1:8783/`, reads and edits the `Config` of other applications |

## Start

- **Locally:** `cargo build --offline --locked` in `oec/demos/config`, then
  `OEC_ROOT=<container storage> target/debug/oec-config`
  ([Build and run](../../../demos/config/README.md#build-and-run)).
- **As an image:** `docker run` with the three read-only mounts of a
  container storage ([Image](../../../demos/config/README.md#image)).
- **With compose:** together with the other demos, see
  [Run all three](index.md#run-all-three). There it uses an MQTT user of
  its own that may publish `Set Config` and no other request.

## How it finds the applications

- It listens to the Birth messages (`Pub/MAM`) and asks the applications
  of its setting `apps` with `Get MAM` at every connect.
- For each application it reads the `Profile`, the `Config` and the
  `SubscriptionList`.
- A `Config` is **writable** if the application subscribes to its `Set`
  topic (SubscriptionList).

Details: [Discovery](../../../demos/config/README.md#discovery).

## Settings

It is not configured over the bus and has no `Config` of its own. An
optional JSON file `etc/oi4/app/config-service.json` sets the address of
the Web UI, the request timeout, the applications to ask at connect and the
length of the `Set` history. See
[Settings](../../../demos/config/README.md#settings).

## Web UI

`http://127.0.0.1:8783/` lists the applications with their state and
Health, live. For the chosen application it shows one form per `Config`,
built from the definition in the `Config` itself (type, unit, range,
choices, default, description). Only a writable `Config` can be edited.
*Send Set Config* sends the changed values; the result appears below the
form, for example `Good` or `BadOutOfRange`. The application decides on the
values; the service checks none itself.

Sensitive values stay masked as `********`: left unchanged, the mask is
sent back and the application keeps its secret; a new value is sent but
never shown.

The same functions are a small JSON API (`/api/apps`, `/api/set`,
`/api/sets`, `/api/refresh`, `/api/events`):
[API](../../../demos/config/README.md#api).

## Limits

- No authentication and no TLS on the Web UI and its API (demo). Anyone who
  reaches the port can change the `Config` of every application.
- It edits `Config` only, no other resource.
- Its `Set` history is kept in memory only.
