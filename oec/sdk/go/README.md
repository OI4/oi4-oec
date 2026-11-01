# Go SDK – `github.com/OI4/oec/sdk/go`

Package `oec` is the Go SDK of OI4 Open Edge Computing v2.0 (deliverable D2,
phase P4c, [ADR-0027](../../../project/decisions/ADR-0027-go-sdk.md)). It is an
idiomatic Go layer over the C ABI [`oec-ffi`](../rust/oec-ffi) of the Rust SDK,
linked statically with cgo. Identity, topics, payloads, Health, MAM, lifecycle,
TLS and reconnection are implemented once, in Rust; this package maps them to Go
types, errors and contexts. Standard library only.

State (C-077, P4c increment 2): application lifecycle (`New`, `Run`, `Start`,
`Shutdown`, `Close`, `State`), own `Health`, `Syslog`, errors (increment 1);
provider Level 2/3 – represented assets, `Data` with its `Metadata`, `Config`,
events, `OnSet` handlers (increment 2); examples `hello` (Level 1) and
`source` (Level 3). The consumer side (increment 3) follows.

## Build

Needs Go (version of `go.mod`, `GOTOOLCHAIN=auto` fetches it), a Rust
toolchain (`oec/sdk/rust`) and a C compiler (cgo).

```sh
cd oec/sdk/go
go run ./cmd/oec-buildlib   # cargo build --release -p oec-ffi → lib/<GOOS>_<GOARCH>/liboec_ffi.a
go build ./...
go run ./examples/hello     # needs a broker, see below
```

`lib/` is git-ignored: the archive is about 30 MB per target and is never
committed. `oec-buildlib` options: `-target <rust triple>` (cross build, e.g.
`aarch64-unknown-linux-gnu`; needs the Rust target and a C cross toolchain),
`-profile debug`, `-dir <rust workspace>`.

The header is always the committed `../rust/oec-ffi/include/oec.h` (never a
copy). Default link flags (`link.go`):

| GOOS | flags |
|------|-------|
| all | `-I${SRCDIR}/../rust/oec-ffi/include`, `-L${SRCDIR}/lib/<GOOS>_<GOARCH> -loec_ffi` |
| darwin | `-liconv -lc -lm` (`-lSystem` is added by the Go linker) |
| linux (glibc) | `-lgcc_s -lutil -lrt -lpthread -lm -ldl -lc` |

The system libraries are those of `rustc --print native-static-libs`
(`cargo rustc -p oec-ffi --release --crate-type staticlib -- --print native-static-libs`).

**Own archive or other system libraries:** build with the tag `oec_external`
– the defaults are left out – and name everything in the environment:

```sh
CGO_CFLAGS="-I/path/to/oec-ffi/include" \
CGO_LDFLAGS="-L/path/to/lib -loec_ffi -lgcc_s -lutil -lrt -lpthread -lm -ldl -lc" \
go build -tags oec_external ./...
```

Containers: build `liboec_ffi.a` in a Rust stage and the Go program in a Go
stage with `CGO_ENABLED=1` for the target platform (linux amd64/arm64); the
result is one binary without a shared `liboec`. Windows (MinGW) comes with P9.

## Hello world

```go
app, err := oec.New(oec.Options{
	Level:       oec.Level1,
	ServiceType: oec.Utility,
	Product: oec.Product{ManufacturerURI: "example.com", Model: "Hello OEC Go", ProductCode: "HELLO-GO-1"},
})
if err != nil {
	return err
}
defer app.Close()
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
return app.Run(ctx)
```

See [`examples/hello`](examples/hello/main.go). In a container the app reads
the container storage (RIG 4.1: `broker.json`, the CA, the credentials or the
client certificate, the hostname); `Options.ContainerRoot` (or `OEC_ROOT` of
the example) moves its root, `Options.Broker` replaces it (then a
`SerialNumber` is needed). Against the dev environment:
`OEC_ROOT=../../dev/generated/roots/oec-dev-app go run ./examples/hello` after
`oec/dev` is up.

## Provider (Level 2/3)

```go
device, err := app.AddAsset(oec.AssetOptions{
	Product:      oec.Product{ManufacturerURI: "sensor-maker.com", Model: "TS-100", ProductCode: "TS-100-A"},
	SerialNumber: "4711", // no IdentificationLink: the Oi4Identifier is derived
})
temperature, err := device.AddData(oec.DataDef{
	Tag: oec.OI4PV,
	Fields: []oec.Field{
		{Name: "Pv", Type: oec.TypeDouble, Unit: "°C", Precision: 0.1},
		{Name: "Sv1", Type: oec.TypeDouble, Unit: "°C", Range: &oec.Range{Low: -20, High: 80}},
	},
	Writable: oec.WritableModeAndInterval, // Set PublicationList may change Mode and Interval
})
err = temperature.Update(map[string]float64{"Pv": 21.5, "Sv1": 22})
err = temperature.OnSet(func(v json.RawMessage) error {
	return oec.BadOutOfRange // or nil: accepted, the SDK publishes the new state
})
```

- **Assets:** `App.This()` is the application itself, `App.AddAsset` adds a
  represented asset (MAM, Health, Profile, PublicationList per `Source`;
  from Level 2 the ReferenceDesignation is offered, which needs a persistent
  store – the container storage or `Options.StoreDir`). `Asset.Remove` withdraws it.
- **Data:** `DataDef` names the fields with their built-in type, unit,
  range and precision; the SDK derives the `Metadata` and the
  `DataSetClassId`. `Mode` zero is the default `ON_REQUEST|FILTER`,
  `ModeOff` stands for `OFF_0`; `Interval` in whole milliseconds. `Update`
  takes any value `encoding/json` encodes to an object of the fields;
  publishing follows the mode (on change, cyclic, on request).
- **Config:** `ConfigDef` with groups and objects (`NumberObject`,
  `StringObject`, `BooleanObject` or a `ConfigObject` with every element of
  MG 8.3.3: `Mandatory`, `Sensitive`, `Min`/`Max`, `MaxLength`, `Pattern`,
  `OneOf`); `AddConfigJSON` takes a ready definition. `Config.Get`/`Set`
  read and write one value (as text); the SDK validates, masks sensitive
  values, persists and answers `Get`/`Set Config`.
- **Events:** `Asset.EmitEvent(oec.Event{Level, Number, Description,
  Details})` publishes a Generic event.
- **`OnSet`:** the handler decides about a `Set Data` (the values as
  `json.RawMessage`) or `Set Config` (the whole proposal as `ConfigValues`,
  clear text). nil accepts; an `oec.StatusCode` (e.g. `oec.BadOutOfRange`)
  or an `*oec.Error` with a Bad code refuses with that code; any other error
  refuses with `Bad`. The SDK checks the request against the fields and
  their types before, answers with the Status event and publishes the new
  state. A second `OnSet` replaces
  the handler.

## Rules

- **Threads:** every method may be called from any goroutine. The C ABI keeps
  its last error per OS thread; the package locks the thread for each call
  and its error read.
- **Signals:** the package installs no signal handler. `Run` uses
  `oec_app_start`/`oec_app_wait`, never `oec_app_run`, whose signal handlers
  conflict with the Go runtime. Stop with a context (`signal.NotifyContext`),
  `Shutdown` or `Close`; such a stop is not an error.
- **Lifetime:** an `App` has no finalizer. Call `Close` once it is no longer
  needed: it sends the Close message (Health, HealthScore 0, on the Will
  topic) and disconnects. A second `Close` returns nil, later calls
  `ErrClosed`.
- **Errors:** `*oec.Error` with `Status`, message and, for a Bad answer, the
  OPC UA `StatusCode`; `errors.Is(err, oec.ErrBootstrap)` etc. Strings with a
  NUL character are refused with `ErrInvalidArgument`.
- **Callbacks** (`OnSet`): they run on a thread of the Rust runtime, one at
  a time, and must return quickly. Blocking SDK calls inside them return
  `ErrInvalidState` – start a goroutine. A panic is recovered, logged with
  `log.Printf` and answered with `BadInternalError`; it never unwinds into
  Rust. The handler state (`cgo.Handle`) lives until `App.Close`, as the C
  ABI requires of its `user_data`.

## Tests

```sh
(cd ../rust && cargo build -p oec-ffi --example go-testbroker)
OEC_GO_REQUIRE_BROKER=1 GODEBUG=cgocheck=1 go test -race ./...
```

The tests run against the in-process TLS test broker of `oec` through the
helper program `go-testbroker` (`../rust/oec-ffi/tests/go/testbroker.rs`,
loopback only, PKI generated per test). Without the helper they skip, unless
`OEC_GO_REQUIRE_BROKER=1`; `OEC_GO_TESTBROKER` names another path.

`OEC_GO_D7=<path of oec-check>` adds D7 `test --level 1` against
`examples/hello` and `test --level 3` against `examples/source` (TLS proxy,
restart hook, peer, announce; ~2 min and ~45 s);
`OEC_GO_D7_REPORT=<file>` and `OEC_GO_D7_L3_REPORT=<file>` keep the JSON
reports. D7 is built from
`oec/conformance` and run as a program – the two modules share no code.
`oec/sdk/check.sh` runs all of it (gofmt, vet, `go test -race`).
