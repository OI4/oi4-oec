# OEC Weather (demo D3)

A small OEC application in Go on the Go SDK (ADR-0027): an `OTConnector`
that represents weather stations as assets of its own. Each station publishes
its weather as the `Data` resource `Oi4Pv`, has a `Config` with its location,
and raises the Generic events `Frost` and `Storm`. A provider supplies the
weather. Today the only provider is `simulation`, which is offline and
deterministic (ADR-0010). MET Norway follows in C-081, the Web UI in C-083.

| | |
|---|---|
| Identity | `oi4.org` / `OEC Weather` / `OEC-WEATHER`, SerialNumber = hostname (container name, REQ-CNT-021) |
| Stations | `oi4.org` / `OEC Weather Station` / `OEC-WEATHER-STATION`, SerialNumber `<hostname>-station-<n>`, n = 1 … 5 |
| ServiceType | `OTConnector` |
| Messaging level | **L3** (`Get`/`Set` of every resource of the application and of each station; D7 `test --level 3` in the gate) |
| Storage | container storage (REQ-CNT-004) below `OEC_ROOT` (default `/`) |
| Web UI | not yet (port 8782, localhost only, no authentication – C-083) |

## Build and run

The Go SDK links `liboec_ffi.a` with cgo. Put it in place once with
`go run ./cmd/oec-buildlib` in `oec/sdk/go`; it is never committed. Then:

```sh
cd oec/demos
go build -o weather/target/oec-weather ./weather
OEC_ROOT=/path/to/container-storage weather/target/oec-weather
```

The container storage holds `etc/hostname`, `etc/oi4/mqtt/broker.json`,
`etc/oi4/certs/ca.pem` and the credentials in `run/secrets/oi4/` (RIG 4.1).
Run the service on a **copy** of a storage, because it writes its store
(Config values, RevisionCounter) to `etc/oi4/app`. SIGTERM or SIGINT end it
with the Close message.

Standard output gets one line per change and per poll round, which the tests
read:

```text
config provider=simulation stations=3 pollInterval=1m0s
station 1 Frankfurt am Main at 50.1109, 8.6821, 112 m added
running 0.1.0
event station 3 Frost begins
poll 3 stations
```

## Configuration

Both `Config` resources (group `common`) are read and changed over the bus
with `Get`/`Set Config`, e.g. by the config service D10 or `oec-probe`.
Values persist across restarts, and `""` resets an object to its default. A
`Set` changes all values at once or nothing: an invalid value is refused
with Event `Status` Bad. An accepted `Set` takes effect in an immediate poll
round.

Application (context `weather`):

| Object | Type | Default | Meaning |
|---|---|---|---|
| `provider` | String, one of `simulation` | `simulation` | the weather data provider (`met-no` follows in C-081) |
| `userAgentContact` | String (≤ 256) | `""` | e-mail address or URL in the User-Agent of a provider that requires it (MET Norway, C-081) |
| `pollInterval` | Number 1 … 86400, whole | `60` s | time between two observations of all stations |
| `stations` | Number 1 … 5, whole | `3` | number of stations; new ones appear, removed ones leave the `PublicationList` |
| `frostThreshold` | Number −50 … 50 | `0` °C | a temperature at or below it raises `Frost` |
| `stormThreshold` | Number 0 … 100 | `20.8` m/s | a wind speed at or above it (Beaufort 9) raises `Storm` |
| `simulationSeed` | Number 0 … 999999999, whole | `1` | seed of the simulated weather |

Station n (context `station`):

| Object | Type | Default (n = 1 … 5) | Meaning |
|---|---|---|---|
| `name` | String (≤ 64) | Frankfurt am Main, Oslo, Zugspitze, Hamburg, Munich | name of the location |
| `latitude` | Number −90 … 90, ≤ 4 decimals | 50.1109, 59.9139, 47.4211, 53.5511, 48.1374 | WGS 84, north positive (°) |
| `longitude` | Number −180 … 180, ≤ 4 decimals | 8.6821, 10.7522, 10.9853, 9.9937, 11.5755 | WGS 84, east positive (°) |
| `altitude` | Number −500 … 9000, whole | 112, 23, 2962, 6, 519 | height above sea level (m) |

A station that is removed and added again keeps its stored `Config`.

## Data and events

`Data` `Oi4Pv` of each station. Its `Metadata` comes from the field
definitions below. It is published self-initiated on a change of at least
the Precision, and consumers may change Mode and Interval with
`Set PublicationList`.

| Field | Quantity | Unit | Range | Precision |
|---|---|---|---|---|
| `Pv` | air temperature | °C | −90 … 60 | 0.1 |
| `Sv1` | relative humidity | % | 0 … 100 | 1 |
| `Sv2` | air pressure at sea level | hPa | 870 … 1085 | 0.1 |
| `Sv3` | wind speed | m/s | 0 … 75 | 0.1 |
| `Sv4` | wind direction (from, clockwise from north) | ° | 0 … 360 | 1 |

Warnings are Generic events of the station (`Pub/Event/<station>/Generic/Frost`
or `…/Storm`). `Number` 1 means the warning begins and 0 means it ends.
`Details` carry the value and the threshold. A warning is evaluated only
while connected, so a change during a connection loss is reported after the
reconnect.

## Simulation

The provider `simulation` needs no network. Its weather is a pure function of
the seed, the coordinates, the altitude and the time:

- a climate by latitude (warmer at the equator);
- a seasonal cycle that is reversed in the southern hemisphere;
- a diurnal cycle with its maximum at 15:00 local solar time;
- the standard lapse rate of 6.5 K/km;
- smooth value noise per quantity on a slow scale (one to one and a half
  days) and a fast scale (one to three hours).

The same inputs always give the same values. The station name has no
influence.

## Tests

`oec/demos/check.sh` (a step of `oec/check.sh`) builds the service with
`-race` and runs the tests in this directory:

- unit tests for the simulation (determinism, ranges, climate, smoothness),
  the settings and the Config definitions;
- integration tests against the in-process TLS test broker:
  - defaults;
  - adding and removing stations;
  - the poll interval;
  - `Set Config` of a station, and refused values;
  - Frost/Storm events;
  - persistence across a restart;
- D7 `oec-check test --level 3`: 49 pass, 6 skip.

See [`../README.md`](../README.md).

## Code

| File | Content |
|---|---|
| `provider.go` | `Location`, `Observation`, the `Provider` interface, the quantities of `Oi4Pv` |
| `simulation.go` | the offline provider `simulation` |
| `settings.go` | the Config definitions, their parsing and checks, the identity helpers |
| `service.go` | the stations and the poll goroutine (applies accepted Configs, polls, publishes, raises warnings) |
| `main.go` | the app: identity, start, signals |
