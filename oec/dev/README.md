# oec/dev – development environment

**Development only.** A local MQTT broker (Mosquitto) with TLS, a dev PKI and
sample container storages, so OEC applications and the SDKs can run against a
real broker. Nothing here is fit for production; every generated file is a
development secret and is never committed (`generated/` is git-ignored).

Decisions: ADR-0012 (monorepo `oec/dev/`), ADR-0026 amendment C-051 (dev
environment and interop test contract), ADR-0028 §5 and amendment C-084
(compose profile `demos`).

| Path | Content |
|---|---|
| `compose.yaml` | Mosquitto 2.1.2, pinned by tag and digest, ports on loopback only; opt-in profile `demos` with the three demo services |
| `mosquitto/mosquitto.conf` | TLS 1.2/1.3 only, credentials listener, client certificate listener, packet limits |
| `pki/` | Go generator of the dev PKI, the password file, the sample container storages and the storages of the demos (standard library only) |
| `interop.sh` | Opt-in interop tests of the Rust SDK against this broker |
| `../conformance/live.sh` | Opt-in live run of `oec-check monitor` (D7) against this broker (C-052) |
| `generated/` | Output of `go run ./pki` – **never commit** |

## Prerequisites

- Go (the version of `go.mod`; with `GOTOOLCHAIN=auto` Go downloads it).
- A container engine with Compose v2: Docker (`docker compose`) or Podman
  (`podman compose`/`podman-compose`, set `OEC_COMPOSE`, see below). The
  first start pulls `eclipse-mosquitto:2.1.2-alpine`.

## Quick start

```sh
cd oec/dev
go run ./pki                      # writes generated/ (once)
docker compose up -d --wait       # start the broker
docker compose logs -f mosquitto  # connects, disconnects, Wills
docker compose down               # stop (keeps the persistent sessions)
docker compose down -v            # stop and remove the broker data volume
```

`go run ./pki` refuses to overwrite an existing `generated/`; `-force`
replaces it (only a directory it created itself). After regenerating, restart
the broker: `docker compose up -d --force-recreate --wait`.

Options of `go run ./pki`:

| Flag | Default | Meaning |
|---|---|---|
| `-out` | `generated` | output directory |
| `-force` | off | replace an existing output directory |
| `-port` | `8883` | SecurePort of the credentials sample (`oec-dev-app`) |
| `-cert-port` | `8884` | SecurePort of the certificate sample (`oec-dev-cert`) |
| `-address` | `localhost` | Address in the sample `broker.json` |
| `-san` | – | additional DNS names / IPs of the broker certificate, comma separated |

The broker certificate always contains `localhost`, `mosquitto`, `127.0.0.1`
and `::1`. To reach the broker from another machine or container, add its
name with `-san` and use it as `-address`.

## Broker

| Host port | Listener | Authentication |
|---|---|---|
| `127.0.0.1:8883` | TLS 1.2/1.3 | user name and password (`password_file`, REQ-MQTT-009) |
| `127.0.0.1:8884` | TLS 1.2/1.3 | client certificate of the dev CA; its CN is the user name (REQ-MQTT-010) |

- No plain-text and no anonymous listener (REQ-MQTT-005, REQ-MQTT-006).
- The broker certificate is signed by the dev CA (REQ-MQTT-007).
- `OEC_DEV_BIND`, `OEC_DEV_MQTT_PORT` and `OEC_DEV_CERT_PORT` change the
  published address and ports. If you change the ports, pass the same ports to
  `go run ./pki -port … -cert-port …`.
- Persistence is on (named volume `data`): sessions with Clean Session 0
  survive `docker compose restart`.
- **Packet size (RIG 4.1.1, SF-105):** the sample `broker.json` announces
  `MaxPacketSize` 262144. This limits the *payload*, so the broker drops larger
  payloads (`message_size_limit 262144`). Its whole-packet limit
  (`max_packet_size`) is 65544 octets higher, which leaves room for the
  largest PUBLISH header. With `max_packet_size 262144`, Mosquitto would
  disconnect a client that publishes a payload of exactly MaxPacketSize.
- **ACL (MG 9.1, REQ-GEN-001; C-071, C-084):** [`mosquitto/acl`](mosquitto/acl). Only
  the probe user `oec-probe` (the test tools, the interop probe) may publish
  `Set`, `Del` and `Call`; the user `oec-config` of the config service (profile
  `demos`) may publish `Set/Config` and no other request; every client reads
  `Oi4v2/#` and publishes `Pub`, `Get` and `Reply`. Mosquitto drops (and acknowledges) a denied PUBLISH. Limitation of
  the dev environment: Mosquitto cannot bind the AppId topic level to a user,
  and an application must read its own `Set/Config`, so every client can read
  the `Set/Config` of the others – `oec-infra bus` warns on
  REQ-RES-CONFIG-026 here. An application that sends `Set`, `Del` or `Call`
  to others needs a `user` entry with these rights.

The entrypoint copies the TLS material, the password file and the ACL from the
read-only mount into `/mosquitto/tls` (owner `mosquitto`, mode 0600), so host
ownership and permissions of `generated/` don't matter. On SELinux hosts the
bind mounts may need the `:z` option.

## Generated files

```text
generated/
├── .oec-dev-pki                 marker: -force may replace this directory
├── broker/                      mounted into the broker
│   ├── ca.pem                   dev CA (ECDSA P-256)
│   ├── broker.pem, broker.key   broker certificate and key
│   └── passwd                   Mosquitto password file ($7$ PBKDF2-SHA512)
├── probe/                       a second client for tools and the interop tests
│   ├── username, password       credentials (plain text)
│   └── probe.pem, probe.key     client certificate and key (unencrypted)
├── demos.env                    container names of the profile demos
└── roots/                       sample container storages (RIG 4.1)
    ├── oec-dev-app/             credentials, SecurePort 8883
    ├── oec-dev-cert/            client certificate, SecurePort 8884
    └── oec-logger/, oec-weather/, oec-config/
                                 storages of the profile demos (below)
```

Each sample root is the file system of an application container below `/`:

| File | RIG / REQ | `oec-dev-app` | `oec-dev-cert` |
|---|---|---|---|
| `etc/hostname` | container name, REQ-CNT-019/021 | `oec-dev-app` | `oec-dev-cert` |
| `etc/oi4/mqtt/broker.json` | RIG 4.1.1, REQ-CNT-003 | port 8883 | port 8884 |
| `etc/oi4/mqtt/broker.pem` | RIG 4.1.1 | broker certificate | broker certificate |
| `etc/oi4/certs/ca.pem` | RIG 4.1.2, REQ-CNT-005/006 | dev CA | dev CA |
| `etc/oi4/certs/<container>.pem` | RIG 4.1.2, REQ-CNT-007 | – | client certificate |
| `run/secrets/oi4/credentials` | RIG 4.1.3, REQ-CNT-008 | `base64(user:password)` | – |
| `run/secrets/oi4/private_key.pem` | RIG 4.1.3, REQ-CNT-008 | – | PKCS#8 PBES2 (AES-256-CBC, PBKDF2-SHA256) |
| `run/secrets/oi4/passphrase` | RIG 4.1.3, REQ-CNT-008 | – | base64 passphrase |
| `etc/oi4/app/` | RIG 4.1.4, REQ-CNT-010 | empty | empty |

`broker.json` has exactly the keys of the vectors V-BRK-001 … 018:
`{"Address": "localhost", "SecurePort": 8883, "MaxPacketSize": 262144}`.
There is no `broker_ca.pem`, so clients verify the broker against
`etc/oi4/certs/ca.pem` (RIG 4.1.1). Secret files have mode 0600 (the
credentials of the demo storages: see the profile `demos`).

## Compose profile `demos` (C-084)

The three demo services of `oec/demos` (ADR-0028) as containers next to the
broker – opt-in, `docker compose up` alone still starts only the broker.
The images are local (`oi4/oec-<demo>:0.1.0`, built by
[`../demos/images/build.sh`](../demos/images/build.sh); `pull_policy: never`,
compose never pulls them).

```sh
cd oec/demos && images/build.sh /tmp/oec-images   # once: build, D8 image + run, load
cd ../dev
go run ./pki            # a generated/ of an older cycle: go run ./pki -force
docker compose --env-file generated/demos.env --profile demos up -d --wait
# Web UIs: http://127.0.0.1:8781 (logger), :8782 (weather), :8783 (config)
docker compose --env-file generated/demos.env --profile demos down -v
cd ../demos && images/build.sh --remove           # remove the loaded images
```

| Service | Image | Web UI | Messaging level | MQTT user |
|---|---|---|---|---|
| `oec-logger` | `oi4/oec-logger` | `127.0.0.1:8781` | L2 | `oec-logger` |
| `oec-weather` | `oi4/oec-weather` | `127.0.0.1:8782` | L3 | `oec-weather` |
| `oec-config` | `oi4/oec-config` | `127.0.0.1:8783` | L1 | `oec-config` (ACL: `Set/Config`) |

- **Container name = hostname** `<service>-<uuid>` (REQ-CNT-019, REQ-CNT-020):
  `go run ./pki` draws a UUID per demo and installation and writes the names
  to `generated/demos.env`; compose reads them with `--env-file`. Without it
  the containers are named `oec-logger`, `oec-weather`, `oec-config` (valid,
  but without the UUID that REQ-CNT-020 recommends). The name is the
  SerialNumber of the AppId (REQ-CNT-021).
- **Storages:** `generated/roots/<service>/` holds `broker.json` (Address
  `mosquitto`, SecurePort 8883 – the broker in the compose network, not the
  host port), `broker.pem`, `ca.pem` and the credentials, mounted read-only at
  `/etc/oi4/mqtt`, `/etc/oi4/certs` and `/run/secrets/oi4`. The image runs as
  UID 65532, so the credentials file is readable by all; its `run/`
  directory (mode 0700) keeps the other users of the host out. The
  application storages `/etc/oi4/app` and `/opt/oi4/app` are named volumes
  (`down -v` removes them); nothing is mounted over `/opt/oi4/licenses`
  (REQ-CNT-017).
- **Config service:** `generated/roots/oec-config/etc/oec-config/config-service.json`
  lists the logger and the weather service by AppId (`apps`), so the service
  asks them with `Get MAM` at every connect, also after a restart of its own.
- **Weather service:** starts with the provider `simulation` (offline); it
  calls `api.met.no` only after a `Set` of `provider` and `userAgentContact`.
- **Hardening:** read-only root file system, all capabilities dropped,
  `no-new-privileges`, bridge network of the project (REQ-CNT-023). The Web
  UIs listen on `0.0.0.0` inside the container and are published on
  `127.0.0.1` only (A-305: no authentication, no TLS).

## Run an application against the broker

On the host, with the Rust SDK (`Bootstrap::ContainerStorage`, ADR-0020 §5):

```rust
App::builder(Level::L1, ServiceType::Utility, product)
    .bootstrap(Bootstrap::ContainerStorage {
        root: "oec/dev/generated/roots/oec-dev-app".into(),
    })
    .build()?
    .run()?;
```

The serial number is the container name (`oec-dev-app`); the application
connects to `localhost:8883` with the credentials of the sample.

In a container, mount the sample root at the paths of RIG 4.1 and use the
broker name as Address (`go run ./pki -force -address mosquitto`), e.g. with
an additional compose file `my-app.yaml` started together with the broker
(`docker compose -f compose.yaml -f my-app.yaml up -d`):

```yaml
services:
  my-app:
    image: my-app:dev
    hostname: oec-dev-app
    volumes:
      - ./generated/roots/oec-dev-app/etc/oi4/mqtt:/etc/oi4/mqtt:ro
      - ./generated/roots/oec-dev-app/etc/oi4/certs:/etc/oi4/certs:ro
      - ./generated/roots/oec-dev-app/run/secrets/oi4:/run/secrets/oi4:ro
```

Inside the project network, the broker listens on `mosquitto:8883` and
`mosquitto:8884`.

To watch the bus with the Mosquitto tools in the broker container:

```sh
docker compose exec mosquitto mosquitto_sub -h localhost -p 8883 \
  --cafile /mosquitto/tls/ca.pem -u oec-probe -P "$(cat generated/probe/password)" \
  -t 'Oi4v2/#' -v
```

## Interop tests

```sh
oec/dev/interop.sh
```

The script generates the PKI if it is missing, starts the broker, runs
`cargo test -p oec --test interop` with `OEC_INTEROP=1`, and then removes the
broker and its volume (`OEC_INTEROP_KEEP=1` keeps it running). Without a
running container engine it prints a note and exits 0. It is **not** part of
the gate `oec/check.sh`: the gate needs neither a container engine nor the
network. Without `OEC_INTEROP=1` the interop tests pass at once.

The tests (`oec/sdk/rust/oec/tests/interop.rs`) cover:

- login with credentials and with a client certificate (encrypted key);
- Birth → Health → Close → DISCONNECT;
- the Will of a killed process;
- a broker restart: reconnect with a new Will and a new Birth;
- refused logins and a wrong CA;
- TLS 1.2 and 1.3;
- the packet limits;
- the explicit bootstrap.

The tests restart the broker with `$OEC_COMPOSE restart mosquitto`. They
run on the host and need the default `-address localhost` of the PKI.

Environment:

| Variable | Default | Meaning |
|---|---|---|
| `OEC_COMPOSE` | `docker compose` | compose command, e.g. `podman compose` |
| `OEC_INTEROP_KEEP` | – | `1`: leave the broker running |
| `OEC_DEV_DIR` | `oec/dev` | location of `compose.yaml` and `generated/` (set by the script) |

**Platforms:** `interop.sh` and `go run ./pki` run on Linux and macOS. On
Windows, use Git Bash or WSL, or run the steps by hand:

```powershell
cd oec/dev
go run ./pki
docker compose up -d --wait
cd ../sdk/rust
$env:OEC_INTEROP = "1"
cargo test -p oec --test interop -- --test-threads=1
cd ../../dev
docker compose down -v
```

## Live run of `oec-check monitor` (D7) and `oec-infra bus` (D8)

```sh
oec/conformance/live.sh
```

Same pattern as `interop.sh` (PKI if missing, broker up, tests, broker down;
`OEC_LIVE_KEEP=1` keeps it, `OEC_LIVE_OUT=DIR` keeps the recorded captures).
Since C-071 it also runs `go test ./cmd/oec-infra -run TestBusDevBroker`:
`oec-infra bus` on 8883 (probe credentials, unprivileged client
`oec-dev-app`) and 8884 (probe certificate, unprivileged client
`oec-dev-cert`) – T8-BUS-01 passes, T8-BUS-02 warns REQ-RES-CONFIG-026 (see
the ACL above), T8-BUS-03 is inconclusive (no OEC Registry before P7). First
it runs `go test ./cmd/oec-check -run TestMonitorDevBroker` with
`OEC_LIVE=1`: two monitors attach with the sample storages
(`--bus-storage generated/roots/oec-dev-cert`, client certificate on 8884;
`--bus-storage generated/roots/oec-dev-app`, credentials on 8883), a
simulated component logs in with the probe credentials, publishes its Birth,
is killed (the broker publishes its Will) and reconnects with a new Birth.
Both monitors must record the five messages and judge the lifecycle as
passed (the heartbeat stays inconclusive in 6 s, exit 4). By hand:

```sh
cd oec/conformance
go run ./cmd/oec-check monitor --bus-storage ../dev/generated/roots/oec-dev-cert \
  --duration 3m --record /tmp/bus.jsonl
go run ./cmd/oec-check monitor --replay /tmp/bus.jsonl
```

`podman-compose` 1.x does not support `up --wait`; use `podman compose` with
the Docker Compose provider, or start the broker by hand first.
