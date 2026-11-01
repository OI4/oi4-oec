// Package oec is the Go SDK of OI4 Open Edge Computing v2.0 (ADR-0027).
//
// It is a thin, idiomatic layer over the C ABI oec-ffi (ADR-0021) of the
// Rust SDK, linked statically by cgo: the guideline logic (identity, topics,
// payloads, Health, MAM, lifecycle, reconnection) lives in the Rust core,
// this package maps it to Go types, errors and contexts.
//
// # Building
//
// The package links lib/<GOOS>_<GOARCH>/liboec_ffi.a of this module
// directory. Build it once per target with a Rust toolchain:
//
//	go run ./cmd/oec-buildlib            # host, release profile
//	go run ./cmd/oec-buildlib -target aarch64-unknown-linux-gnu
//
// With the build tag oec_external the default flags are left out and
// CGO_CFLAGS/CGO_LDFLAGS name the header directory, the library and the
// system libraries instead (see README.md).
//
// # Hello world
//
//	app, err := oec.New(oec.Options{
//		Level:       oec.Level1,
//		ServiceType: oec.Utility,
//		Product:     oec.Product{ManufacturerURI: "example.com", Model: "Hello", ProductCode: "HELLO-1"},
//	})
//	if err != nil { ... }
//	defer app.Close()
//	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
//	defer stop()
//	err = app.Run(ctx)
//
// [App.Run] connects, publishes the Birth (MAM and Health) and serves
// until ctx is done, [App.Shutdown] or [App.Close] is called or the app
// fails; a stop by ctx, Shutdown or Close is not an error. [App.Start]
// starts the app in the background instead.
//
// # Provider
//
// [App.This] is the application itself, [App.AddAsset] adds a represented
// asset. [Asset.AddData] adds a Data DataSet with its Metadata,
// [Asset.AddConfig] a Config, [Asset.EmitEvent] publishes a Generic event.
// [DataSet.OnSet] and [Config.OnSet] decide about a Set request: nil
// accepts, a [StatusCode] refuses with that code:
//
//	err = temperature.OnSet(func(v json.RawMessage) error {
//		return oec.BadOutOfRange
//	})
//
// # Threading and resources
//
//   - Every method of [App], [Asset], [DataSet] and [Config] is safe for
//     concurrent use.
//   - Handlers run on a thread of the Rust runtime, one at a time, and must
//     return quickly; blocking SDK calls inside them return
//     [ErrInvalidState] (start a goroutine). A panic in a handler is
//     recovered, logged and answered with BadInternalError.
//   - The C ABI reports errors through thread-local state; the package
//     locks the OS thread for each call and its error read, callers need
//     not care.
//   - The package installs no signal handlers: stopping on SIGINT/SIGTERM
//     is the job of the program (signal.NotifyContext). The Rust runtime
//     is used through oec_app_start/oec_app_wait, never oec_app_run, which
//     would install signal handlers that conflict with the Go runtime.
//   - An [App] owns native resources and has no finalizer: call
//     [App.Close] exactly when the app is no longer needed (a second
//     Close returns nil). Close sends the Close message (Health with
//     HealthScore 0 on the Will topic) and disconnects.
//
// # Errors
//
// Every error of the C ABI is an [*Error] with the [Status], the message
// of oec_last_error and, for a Bad answer, the OPC UA [StatusCode]. The
// sentinels ErrInvalidArgument … ErrInternal match by status with
// errors.Is:
//
//	if errors.Is(err, oec.ErrBootstrap) { ... }
//
// Strings with a NUL character are refused with [ErrInvalidArgument]
// before they reach the C ABI.
package oec
