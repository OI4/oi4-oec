---
title: Demo services
sidebar_position: 2
description: Three small OEC applications that work together on the bus of the dev environment – a logger, a weather service and a config service.
---

# Demo services

Three small OEC applications show the SDKs at work. Each serves a small Web
UI of its own that shows what happens on the bus. The Web UIs are not part
of the OEC standard and are not on the bus.

| Demo | What it does | Built on | ServiceType | Messaging level | Web UI |
|---|---|---|---|---|---|
| [Logger](logger.md) | logs the `Event`, `Health` and optionally `Data` publications of all participants | C on the C ABI | `Utility` | 2 | `http://127.0.0.1:8781/` |
| [Weather service](weather.md) | weather stations as assets with `Data`, `Config`, `Health` and warnings | Go on the Go SDK | `OTConnector` | 3 | `http://127.0.0.1:8782/` |
| [Config service](config.md) | finds the applications and edits their `Config` | Rust on the SDK crate `oec` | `Utility` | 1 | `http://127.0.0.1:8783/` |

All three are built as OCI images for `linux/amd64` and `linux/arm64`
(non-root, with an SBOM and the license texts in `/opt/oi4/licenses`) and
meet the Service Container level 3.

## Run all three

You need Docker (with `buildx`), Go and network access for the first build.
From the repository folder `oec/`:

```sh
cd demos && images/build.sh /tmp/oec-images   # build the images (a few minutes)
cd ../dev
go run ./pki                                  # broker, certificates, demo storages
docker compose --env-file generated/demos.env --profile demos up -d --wait
```

Then open the three Web UIs:

1. **Config service** `http://127.0.0.1:8783/` lists the logger and the
   weather service with their `Config`. Change the number of stations of
   the weather service from 3 to 4, or turn on **Log Data** of the logger,
   and press *Set*. The page shows the `Status` event of the target.
2. **Weather service** `http://127.0.0.1:8782/` shows the new station with
   its values, Health and warnings.
3. **Logger** `http://127.0.0.1:8781/` shows the `Status` event of your
   change, the warnings of the stations and – with *Log Data* on – their
   values, with charts.

Stop and clean up:

```sh
docker compose --env-file generated/demos.env --profile demos down -v
cd ../demos && images/build.sh --remove
```

`generated/` holds the keys and passwords of your dev environment; never
commit or share it.

### With the OEC Registry

The OEC Registry lists every asset on the bus. Build its image once
(`registry/images/build.sh /tmp/oec-registry` from `oec/`), add
`--profile registry` to both compose commands above and remove the image at
the end with `registry/images/build.sh --remove`. Its Web UI is at
`https://localhost:5798/` (HTTPS with a certificate of the dev CA
`generated/broker/ca.pem`) and shows the three demos and the stations of
the weather service. Start both profiles together: the demos then start
after the registry, which matters for the config service (it announces
itself only at its start). What the registry shows and how to use it:
[OEC Registry](../registry/index.md). Details:
[`oec/dev/README.md` – Compose profile `registry`](../../../dev/README.md#compose-profile-registry-c-090).

## Good to know

- **No authentication and no TLS on the Web UIs.** They are demos, not
  products. Compose publishes them on `127.0.0.1` only; do not expose them
  to a network.
- The weather service uses the offline provider `simulation` by default.
  The provider `met-no` reads real forecasts from MET Norway and needs your
  contact address (see [Weather service](weather.md)).
- A change of a `Config` persists across restarts. An empty value resets an
  object to its default.
- You can check every demo yourself with the compliance tools: `oec-check`
  (D7) on the bus and `oec-infra` (D8) on the images and containers.

## Details

- Build, layout and tests of the demos:
  [`oec/demos/README.md`](../../../demos/README.md)
- Images, SBOM and `build.sh`:
  [`oec/demos/README.md` – Images](../../../demos/README.md#images-c-084)
- The compose profile `demos`, container names and MQTT users:
  [`oec/dev/README.md` – Compose profile `demos`](../../../dev/README.md#compose-profile-demos-c-084)
