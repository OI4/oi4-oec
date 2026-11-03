---
title: OEC Registry
sidebar_position: 3
description: The OEC Registry (D5) lists every participant on the OI4 Message Bus with its MAM, Health and availability, answers the Registry services and shows the directory over HTTPS and in a Web UI.
---

# OEC Registry

The OEC Registry watches the OI4 Message Bus and keeps a directory of every
participant: the applications and the devices they represent. For each it
shows the Master Asset Model (`MAM`), the latest `Health`, whether the asset
is available, its `Profile` and its `Event`s. It offers the available
assets to other applications with the Registry services of the Messaging
Guideline (`Get MAM` and `Get Health` on its own AppId) and to people over
HTTPS: a REST API, an event stream and a Web UI. It is written in Rust on
the SDK crate `oec`.

| | |
|---|---|
| ServiceType | `Registry` |
| Messaging level | 3 |
| Service Container level | 3 |
| Image | `oi4/oec-registry:0.1.0` (linux/amd64 and linux/arm64) |
| Web UI | `https://<host>:5798/`, read-only |
| REST API | `https://<host>:5798/api/…` and `https://<host>:5799/api/…`, read-only |

## What it shows

For every asset on the bus:

- **Availability:** *online*; *offline (graceful)* after its `Close`;
  *offline (ungraceful)* after its Will or a `FAILURE_1` about itself. An
  asset is *available* when it is online and both its `MAM` and `Health`
  are known; only available assets are answered in the Registry services.
  An application that went offline is online again with its next Birth.
- **MAM** and **Health**, with a short history of the `Health` changes.
- **Profile**, **PublicationList**, **SubscriptionList** and the answers to
  the registry's own `Get`s.
- **Observed Messaging level** – derived from what the participant does on
  the bus, with the requirements it misses for the next level. It is
  informative, not a compliance claim; use `oec-check` (D7) for that.
- **Schema conformity** – every received message is checked against the
  OEC JSON Schemas: number of messages, number of violations and the last
  violations. Also informative.
- The **Event trail** of all participants.

Devices are listed under the application that represents them. Tools that
connect for a while (for example the probes of `oec-infra bus`) are
listed as well and stay offline after they leave. The registry does not
list itself; it ignores its own messages.

The registry asks a participant with `Get` for what it does not know yet
(at most once per online period and setting `GetInterval`). It never sends
`Set`, `Del` or `Call`.

## Start

### With the dev environment and the demos

The quickest way is the compose profile `registry` of `oec/dev`, together
with the [demo services](../demos/index.md#with-the-oec-registry). From
`oec/`:

```sh
demos/images/build.sh /tmp/oec-images        # once: the demo images
registry/images/build.sh /tmp/oec-registry   # once: the registry image
cd dev
go run ./pki      # keys and certificates of the dev environment (-force renews an old one)
docker compose --env-file generated/demos.env --profile demos --profile registry up -d --wait
```

Open `https://localhost:5798/`. The registry presents a certificate of the
dev CA: trust `generated/broker/ca.pem` or accept the warning of the
browser. Stop and clean up:

```sh
docker compose --env-file generated/demos.env --profile demos --profile registry down -v
cd .. && registry/images/build.sh --remove && demos/images/build.sh --remove
```

`--env-file generated/demos.env` is needed: it sets the container names,
which are the SerialNumbers of the applications and the name of the
registry's client certificate. Details:
[`oec/dev/README.md` – Compose profile `registry`](../../../dev/README.md#compose-profile-registry-c-090).

### As an image

`oi4/oec-registry` uses the container storage of the Runtime &
Infrastructure Guideline like every OEC application: mount the broker
settings (`/etc/oi4/mqtt/broker.json`), the certificates (`/etc/oi4/certs`)
and the credentials or the private key (`/run/secrets/oi4`), name the
container `oec-registry-<uuid>` and use the same name as hostname. It runs
as user 65532 with a read-only root file system; `/etc/oi4/app` and
`/opt/oi4/app` are volumes. The SBOM and the license texts are in
`/opt/oi4/licenses`. Build and check:
[`oec/registry/README.md` – Image](../../../registry/README.md#image-adr-0029-12).

### Local, without a container

```sh
cd oec/registry
cargo build --locked
OEC_ROOT=<copy of a container storage> target/debug/oec-registry
```

`OEC_ROOT` is the root of a container storage, for example a copy of
`oec/dev/generated/roots/oec-registry`. Details:
[`oec/registry/README.md` – Build and run](../../../registry/README.md#build-and-run).

## Settings

- **On the bus:** the registry has a `Config` of its own (group
  `directory`): the interval and timeout of its `Get`s and how much history
  it keeps (Health changes, Events, schema violations). Change it with any
  config tool, for example the [config service](../demos/config.md).
- **At the start:** environment variables for the HTTP ports, TLS, further
  host names and the log level (`OEC_REGISTRY_WEB`, `OEC_REGISTRY_API`,
  `OEC_REGISTRY_TLS`, `OEC_REGISTRY_HOSTS`, `OEC_REGISTRY_LOG`).

Defaults and ranges:
[`oec/registry/README.md` – Config `registry`](../../../registry/README.md#config-registry)
and [HTTP](../../../registry/README.md#http-rest-sse-tls-adr-0029-7).

## Web UI

One page, live over the event stream. A header with the number of
applications, devices and online assets and the connection state; the
applications with their devices (with a filter) and below them the Event
trail, which you can pause; next to them the selected asset with its
availability, MAM, Health and its history, observed level, conformity,
Profile, the answers to the `Get`s, PublicationList and SubscriptionList.
Each asset has its own address (`#/asset/<IL>`) that you can bookmark. The
page loads nothing from other sites.

## REST and event stream

| Request | Answer |
|---|---|
| `GET /api/assets` | the directory: every asset with a summary |
| `GET /api/assets/{il}` | one asset with all details (IL percent-encoded) |
| `GET /api/events` | the Event trail |
| `GET /api/stream` | server-sent events: a snapshot, then every change |

For example, against the dev environment, from `oec/dev`:

```sh
curl --cacert generated/broker/ca.pem https://localhost:5799/api/assets
```

The API is described in OpenAPI 3.2 (`/api/openapi.json`), the MQTT side of
the registry in AsyncAPI 3.1 (`/api/asyncapi.json`). Every method other
than `GET` is answered with `405`.

## Ports and TLS

- **5798:** Web UI and API; **5799:** API only (the default ports of the
  Messaging Guideline). Either can be switched off.
- **HTTPS** is on by default. With a client certificate in its container
  storage (as in `oec/dev`) the registry presents that certificate;
  without one it creates a self-signed certificate at every start.
- The server accepts only known host names (`localhost`, the container
  name, `OEC_REGISTRY_HOSTS`) and IP addresses, and refuses requests from
  other web sites.

## Levels and checks

The registry is an application of Messaging level 3 and Service Container
level 3. The checks of the project run `oec-check test --level 3` (D7)
against it and `oec-infra bus` (D8), which finds it on the bus
(T8-BUS-03). The image passes `oec-infra image`, `run` and `container` at
level 3.

## Limits

- **Read-only.** The registry changes nothing: no `Set`, `Del` or `Call` on
  the bus, only `GET` over HTTP. Writing (for example configuring
  participants from the registry) is not part of this version.
- **No authentication** on the HTTP API and the Web UI – they are
  read-only. Publish the ports only where the directory may be read.
- **Start order with Level 1 participants.** The registry lists an asset
  after its Birth or its first heartbeat and then asks for its `MAM`. A
  Level 1 application (such as the config service) answers no `Get`: if it
  started before the registry, the registry sees its heartbeat but no
  `MAM` and lists it as available only after its next Birth (restart it).
  Start the registry first; the compose profiles do this for the demos.
- The directory is kept in memory only: after a restart the registry finds
  the participants again with their next heartbeat (at least every 60 s).
- Only OEC v2 traffic (`Oi4v2/…`) is read.

## Details

- Everything about the registry – settings, HTTP, Web UI, image, layout and
  tests: [`oec/registry/README.md`](../../../registry/README.md)
- The compose profile `registry`, the client certificate and the start
  order: [`oec/dev/README.md` – Compose profile `registry`](../../../dev/README.md#compose-profile-registry-c-090)
