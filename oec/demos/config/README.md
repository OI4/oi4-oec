# Config service – D10 (Rust on `oec`)

A generic configuration tool for every OEC application with a `Config`
resource (ADR-0028 §7, A-304): it finds the applications on the bus, reads
their `Config`, shows which `Config` is writable and changes values with
`Set Config`, reporting the `Status` event of the target. Rust on the SDK
crate `oec` (`oec/sdk/rust`), ServiceType `Utility`, Messaging Level 1:
the service only sends requests and answers none.

Increment 4 (C-082) is the core with a small HTTP/JSON API; the Web UI on
this API follows in C-083, the image, the compose profile and the ACL user
in C-084.

## Build and run

An own Cargo package outside the SDK workspace with its own `Cargo.lock`;
`oec` is a path dependency (`../../sdk/rust/oec`):

```sh
cd oec/demos/config
cargo build --offline --locked          # target/debug/oec-config
OEC_ROOT=<container storage root> target/debug/oec-config
```

The broker, the credentials and the identity come from the container
storage under `OEC_ROOT` (default `/`) through the SDK: `broker.json`,
`/etc/oi4/mqtt/…`, `/run/secrets/oi4` (REQ-CNT-003, GL-ADR-022). Run it on
a **copy** of an example storage, never on `oec/dev/generated/` itself.

## Settings

Not on the bus, so no OEC `Config` of its own (A-354). An optional JSON file
`<OEC_ROOT>/etc/oi4/app/config-service.json` (or the path in
`OEC_CONFIG_FILE`, which must then exist):

```json
{
  "listen": "127.0.0.1:8783",
  "requestTimeoutSeconds": 5,
  "apps": ["Utility/oi4.org/OEC,20Logger/OEC-LOGGER/logger-1"],
  "setHistory": 50
}
```

| Key | Meaning | Default |
|---|---|---|
| `listen` | address of the HTTP API; `""` turns it off | `127.0.0.1:8783` |
| `requestTimeoutSeconds` | timeout of every request, 1 … 60 | 5 |
| `apps` | applications asked with `Get MAM` at every connect, as `<ServiceType>/<AppId>` | none |
| `setHistory` | results of `Set` kept for `/api/sets`, 1 … 1000 | 50 |

Unknown keys and invalid values stop the service with a message (exit ≠ 0).
Environment: `OEC_UI_LISTEN` overrides `listen` (empty: off; the container
of C-084 sets `0.0.0.0:8783`), `OEC_CONFIG_LOG` = `error`, `warn`,
`info` (default) or `debug` for the log on stderr.

## Discovery

- every Birth message `Pub/MAM` of an application (REQ-LIFE-003,
  REQ-LIFE-013) and, at every connect, `Get MAM` to the applications of
  `apps` (A-355); after a reconnect of the service, every online
  application is read again;
- for a new or again online application: `Get Profile`, then `Get Config`
  and `Get SubscriptionList` if the Profile lists them;
- a `Config` is **writable** if the SubscriptionList lists its `Set` topic,
  also by a wildcard (SF-107, REQ-RES-SUBLIST-016);
- `Pub/Config` of any application updates the cached `Config`; a Close
  message or a Will marks the application offline (REQ-LIFE-011/012);
- the service never lists itself.

## API

JSON over HTTP/1.1, one request per connection (`Connection: close`):

| Request | Answer |
|---|---|
| `GET /api/apps` | `{"apps":[{serviceType, app, state, health, healthScore, mam, profile, subscriptionList, configs:[{source, context, writable, dataSetWriterId, config}], errors, discoveredMs}]}` |
| `POST /api/refresh` | body empty or `{}`: discovery of every online application; `{"serviceType","app"}`: of one (unknown: `Get MAM` first, 404 without answer); answers like `/api/apps` |
| `POST /api/set` | body `{"serviceType","app","source","context","values":{"<group>":{"<object>":"<value>"}}}`; answers the recorded result |
| `GET /api/sets` | `{"sets":[…]}`, the newest first: `{id, timeMs, serviceType, app, source, context, values, status:{code, symbol, good}, error}` |

`POST /api/set` checks only the structure (A-356): 400 for an unknown
group or object, a value that is not a string (REQ-RES-CONFIG-007) or no
value; 404 for an unknown application or `Config`; 409 if the application
is offline or the `Config` is not writable; 503 while not connected. The
target decides on the values (REQ-RES-CONFIG-023): its `Status` event comes
back as `status` with HTTP 200, also for a Bad status (e.g. `BadOutOfRange`,
`good: false`); no Status event within the timeout is 504 with `error`.
Every `Mandatory` object not in `values` is sent with its current value
(REQ-RES-CONFIG-011). A `Sensitive` value is published as the mask
`********` and goes back unchanged as the mask, which keeps the secret
(REQ-RES-CONFIG-025, GL-ADR-021); a new secret is sent as given and is
never shown – the history shows the mask. After a Good status the service
reads the `Config` again.

The API is for loopback and the Web UI of the service (A-357): no
authentication, no TLS (a demo, A-305); it refuses a `Host` that is not
`localhost` or an IP address and an `Origin` other than its own (403), a
POST without `application/json` (415), `Transfer-Encoding` (501), bodies
above 64 KiB (413), heads above 8 KiB (431), slow requests (408) and more
than 16 connections (503).

## Tests

```sh
cargo test --offline --locked           # unit tests
```

The Go integration tests in `config_test.go` (package of the Go module
`oec/demos`) start the test broker, the logger (D4), the weather service
(D3) and the config service on loopback: discovery, read, `Set` of the
logger's `minEventLevel` (Good, arrives at the logger), a rejected `Set` of
the weather service (`met-no` with `pollInterval` 599 → `BadOutOfRange`,
A-348), the error answers, the request guards, invalid settings, and D7
`oec-check test --level 1` with a pinned list of passes. `OEC_CONFIG` is the
path of the binary (default `target/debug/oec-config`),
`OEC_CONFIG_D7_REPORT=<file>` keeps the D7 report. `oec/demos/check.sh` runs
fmt, clippy (pedantic, `-D warnings`), the unit tests, cargo-deny with the
`deny.toml` of the SDK and the integration tests.
