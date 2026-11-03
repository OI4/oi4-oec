---
title: Getting started
slug: /
sidebar_position: 0
description: What OI4 Open Edge Computing v2.0 is, what this project provides and how to see OEC applications at work on your own machine in a few minutes.
---

# Getting started

OI4 Open Edge Computing (OEC) lets applications on edge devices work
together without knowing each other in advance. Every application and every
device it represents is an *asset* with an identity. They meet on the
**OI4 Message Bus**, an MQTT broker of the OEC platform, and talk in JSON
messages with a fixed topic structure: an asset announces itself with its
Master Asset Model (`MAM`), reports its `Health`, publishes `Data`, offers
its `Config` for change and sends `Event`s. Other applications find it and
use what it offers.

Two guidelines of the OI4 define OEC v2.0:

- the **OEC Messaging Guideline** – identities, topics, messages and the
  conformity levels 1 to 3 of an application on the bus;
- the **OEC Runtime & Infrastructure Guideline** – the platform: the broker,
  the Service Container that runs the applications (as OCI images) and the
  files the platform provides to them.

This site refers to their sections by number, for example "Messaging
Guideline 7.5" or "Runtime & Infrastructure Guideline 4.1".

## What this project provides

| | What it is for |
|---|---|
| **JSON Schemas** | check every OEC message of version 2.0 |
| **SDKs** for Rust, C and Go | write an OEC application; the SDK does the bus part (connect, announce, Health, answering requests) |
| **Micro SDK** in C | make a microcontroller (ESP32) an OEC smart device |
| **Demo services** | three small OEC applications that work together |
| **OEC Registry** | lists every participant on the bus, with a Web UI |
| **Compliance tools** `oec-check` and `oec-infra` | check an application on the bus and a platform or image against the guidelines |
| **Development environment** | a local broker with TLS and the files an OEC platform provides |

Everything is open source under the MIT license and lives in the folder
`oec/` of the repository.

## See OEC at work

The fastest way to see what OEC is about: run the three demo services and
the OEC Registry on your machine. You need Docker (with `buildx` and
Compose v2), Go and network access for the first build. From the repository
folder `oec/`:

```sh
cd demos && images/build.sh /tmp/oec-images        # the images of the three demos
cd .. && registry/images/build.sh /tmp/oec-registry # the image of the registry
cd dev
go run ./pki                                       # broker, certificates, storages
docker compose --env-file generated/demos.env --profile demos --profile registry up -d --wait
```

The first build takes a few minutes. Then open:

- the **OEC Registry** at `https://localhost:5798/` – every participant on
  the bus with its `MAM`, `Health` and availability. The page uses a
  certificate of your development CA (`oec/dev/generated/broker/ca.pem`):
  trust it or accept the warning of the browser.
- the **config service** at `http://127.0.0.1:8783/` – change the number
  of stations of the weather service from 3 to 4;
- the **weather service** at `http://127.0.0.1:8782/` – the new station
  appears, and a moment later also in the registry;
- the **logger** at `http://127.0.0.1:8781/` – the `Status` event of your
  change and the warnings of the stations.

Stop and clean up:

```sh
docker compose --env-file generated/demos.env --profile demos --profile registry down -v
cd .. && demos/images/build.sh --remove && registry/images/build.sh --remove
```

`oec/dev/generated/` holds the keys and passwords of your development
environment; never commit or share it. What each demo does:
[Demo services](demos/index.md); what the registry shows:
[OEC Registry](registry/index.md).

## Where to go next

- **Write an application** with the SDK of your language:
  [Rust](../../sdk/rust/README.md), C on the C ABI of the Rust SDK
  (the header [`oec.h`](../../sdk/rust/oec-ffi/include/oec.h) and the
  [C examples](../../sdk/rust/oec-ffi/examples/)) or
  [Go](../../sdk/go/README.md). The demos are complete examples, one in
  each language.
- **Build a smart device** on a microcontroller:
  [OEC Micro SDK](micro-sdk/index.md).
- **Check your application** on the bus with `oec-check` and your image or
  platform with `oec-infra`:
  [`oec/conformance/README.md`](../../conformance/README.md).
- **Validate messages** against the JSON Schemas:
  [`oec/schema/README.md`](../../schema/README.md).
- **Run against a broker** while you develop:
  [`oec/dev/README.md`](../../dev/README.md).

More guides – the concepts of OEC, the changes from version 1.x and how-tos
for each SDK and tool – follow on this site.
