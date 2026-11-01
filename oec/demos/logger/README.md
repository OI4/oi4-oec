# OEC Logger (demo D4)

A small OEC application in C on the C ABI of the SDK (`oec-ffi`, ADR-0021):
it subscribes to the `Event`, `Health` and optionally `Data` publications of
**all** participants of the bus, writes one record per accepted
DataSetMessage to standard output (and optionally a file) and keeps the last
records in memory for its Web UI (increment 5, C-083).

| | |
|---|---|
| Identity | `oi4.org` / `OEC Logger` / `OEC-LOGGER`, SerialNumber = hostname |
| ServiceType | `Utility` |
| Messaging level | **L2** (own `Config` with `Get`/`Set`; D7 `test --level 2` in the gate) |
| Storage | container storage (REQ-CNT-004) below `OEC_ROOT` (default `/`) |
| Web UI | not yet (port 8781, localhost only, no authentication – C-083) |

## Build and run

Needs a C17 compiler, GNU make and `liboec_ffi.a`
(`cargo build --release -p oec-ffi` in `oec/sdk/rust`):

```sh
cd oec/demos/logger
make                 # target/oec-logger
make test            # C unit tests of the core (no SDK needed)
make SANITIZE=1 all test   # the same with AddressSanitizer + UBSan in target/asan/
OEC_ROOT=/path/to/container-storage target/oec-logger
```

`OEC_INCLUDE` (directory of `oec.h`) and `OEC_LIB` (`liboec_ffi.a`) override
the defaults. The container storage holds `etc/hostname`,
`etc/oi4/mqtt/broker.json`, `etc/oi4/certs/ca.pem` and the credentials in
`run/secrets/oi4/` (RIG 4.1); run the logger on a **copy** of a storage, it
writes its store (Config values, RevisionCounter) to `etc/oi4/app`.
SIGTERM or SIGINT end it with the Close message. Diagnostics of the logger
itself go to standard error, errors also as a Syslog `Event`.

## Configuration

The `Config` resource of the logger (context `logger`, group `common`) is
read and changed over the bus with `Get`/`Set Config` (e.g. by the config
service D10 or `oec-probe`); values persist across restarts, `""` resets an
object to its default. A `Set` changes all values at once or nothing.

| Object | Type | Default | Meaning |
|---|---|---|---|
| `event` | Boolean | `true` | subscribe to the `Event` resource |
| `health` | Boolean | `true` | subscribe to the `Health` resource |
| `data` | Boolean | `false` | subscribe to the `Data` resource |
| `sources` | String (≤ 2048) | `""` | Identification Links (unencoded, separated by white space); a record is kept if its AppId or Source equals an entry or lies below it (prefix of whole levels, e.g. `vendor.com/Pump`); empty: all |
| `minEventLevel` | String | `Informational` | Events less severe than this Syslog level are dropped (`Emergency` … `Debug`) |
| `format` | String | `json` | `json` (one JSON object per line) or `text` |
| `file` | String (≤ 64) | `""` | file name in `/opt/oi4/app` the records are appended to as well (rotated to `<file>.1` at 10 MiB); empty: standard output only |
| `historySize` | Number 10 … 10000 | `1000` | records kept in memory for the Web UI |

The Event level is the Syslog severity: for `Syslog` events PRI % 8, for
`Status` events by the severity of `Number` (Good → Informational,
Uncertain → Warning, Bad → Error), otherwise Notice.

## Records

JSON (one line each):

```json
{"time":"2026-11-01T11:50:34.689Z","resource":"Event","serviceType":"OTConnector","app":"vendor.com/Pump/P-3/0815","source":"vendor.com/Pump/P-3/0815","filter":"Syslog/Notice","level":"Notice","payload":{"Number":13,"Category":"CAT_SYSLOG_0","Details":{"MSG":"pump notice","HEADER":""}}}
```

`time` is the receive time (UTC), `app`, `source` and `filter` come from the
topic (Identification Links unencoded, `null` if absent), `level` only for
Events, `payload` is the DataSetMessage Payload. Text:

```text
2026-11-01T11:50:34.689Z Event Notice OTConnector vendor.com/Pump/P-3/0815 vendor.com/Pump/P-3/0815 Syslog/Notice {"Number":13,…}
```

The history keeps the last `historySize` records and, per resource,
application, source, filter and numeric field of `Data`/`Health`, the last
240 values (at most 256 series) for the charts of the Web UI.

## Tests

`oec/demos/check.sh` (a step of `oec/check.sh`) runs the C unit tests under
ASan/UBSan and the Go integration tests in this directory against the ASan
build on the in-process TLS test broker: defaults, records of other
applications, `Set Config` of every object, refused values, persistence
across a restart, and D7 `oec-check test --level 2` (46 pass, 9 skip). See
[`../README.md`](../README.md).

## Code

| File | Content |
|---|---|
| `src/json.{h,c}` | small JSON reader (RFC 8259, UTF-8 checked, depth ≤ 64) and string writer – no C dependency |
| `src/core.{h,c}` | settings, Config definition, filter, Event level, records – free of `oec.h` |
| `src/history.{h,c}` | thread-safe ring buffer of records and numeric series |
| `src/main.c` | glue to the C ABI: app, Config, subscriptions, output, file |
| `test/unit.c` | unit tests of the core |
