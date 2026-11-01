# ADR-0027 – Go SDK: idiomatic Go package over the C ABI (`oec-ffi`, cgo)

- Status: **accepted (agent, G-3.4 b)** – the PO decided the scope and the phase order (a Go SDK on the Rust core before the demos, C-075 review addendum 3); the agent decided the technique
- Date: 2026-11-01
- Deciders: agent (technique); product owner (Thomas) – scope, phase order, language of the weather demo
- Related: ADR-0011 (S4: Rust core + C ABI, "further languages via a spike between S9 and S1/S2"), ADR-0020 (Rust API), ADR-0021 (C ABI), ADR-0023 (Rust dependencies), ADR-0026 (Rust workspace), ADR-0013 (Go for the tools), `research/sdk-options.md` §9, Q-034

## Context

The PO wants an SDK for Go, and wants it before the demos. The weather demo (D3) is to be written in Go on it. It may use the Rust core. ADR-0011 left further languages to a spike between S9 (core as co-process) and S1/S2 (in-process native or WASM binding). `research/sdk-options.md` §9.6 recommended, for Go, "wazero WASM rather than cgo", to be decided by a small spike.

What exists today (C-061):
- `oec-core` (≈ 18 200 lines): sans-IO protocol logic, including the own MQTT 3.1.1 codec and session.
- `oec` (≈ 7 300 lines): the runtime. It covers the TCP/TLS transport with rustls and PKCS#8 passphrases, the container storage bootstrap (REQ-CNT-*), reconnect, timers, the persistent store, Config validation, masking and persistence, the secret reload and the `log`/Syslog bridge.
- `oec-ffi` (≈ 2 300 lines): the C ABI of ADR-0021 with 43 functions over `oec`. It covers Level 1 … 3: provider, assets, Data/Metadata, Config, events, consumer, `Get`/`Set`/`Del` and the Action `NewDataSetWriterId`. It has opaque handles, JSON values, callbacks with a user-data pointer and a panic guard, and it is tested in C under ASan/UBSan.

## Options

| | Approach | Assessment |
|-|----------|------------|
| **A** | **cgo over `oec-ffi`**: a Go package wraps the C ABI; `liboec_ffi.a` is linked statically | Uses the whole tested runtime. The package is a thin layer over 43 functions. One implementation of the protocol and the runtime. Costs: cgo, so builds for another platform need a C toolchain for the target; there is no `go get` without the Rust library (a release brings prebuilt archives, P9). |
| B | WASM: `oec-core` compiled to WebAssembly, hosted by wazero (pure Go); Go does transport, TLS, timers and storage | No cgo, plain Go cross builds. But `oec-core` has no WASM interface (a second foreign ABI next to ADR-0021). And the 7 300 lines of the `oec` runtime would have to be written again in Go: transport, bootstrap, store, Config persistence, secret reload. That is two runtimes that can drift apart, for one demo. |
| C | Co-process (S9): the Rust runtime as a sidecar, Go talks IPC | Two processes per application container, against the one-app-per-container model (RIG). A new IPC protocol and its versioning. |
| D | Go implementation of its own | A third implementation (Rust, D9 in C, Go). The PO allowed the Rust core so that this is not needed. |
| E | UniFFI with `uniffi-bindgen-go`, or `purego` (dlopen without cgo) | UniFFI: a community generator next to the hand-made C ABI (ADR-0011 chose one C ABI as the only foreign interface). purego: needs the shared library at run time (no static binary), and callbacks from foreign threads are limited. |

## Spike (C-075 review addendum 3, not committed)

A Go program (Go 1.26, cgo) against `liboec_ffi.a` of the release build (macOS arm64):
- It builds a Level 1 `Utility` through `oec_product_*`/`oec_builder_*`/`oec_builder_container_storage`.
- It subscribes to `Health` through `oec_subscribe`. The callback is a Go function exported with `//export` and handed over by a small C file; the user data is a `runtime/cgo.Handle`.
- It runs `oec_app_run` in a goroutine that holds its OS thread.
- It connected to the Mosquitto of `oec/dev` with TLS and the sample storage `oec-dev-app` (opt-in Docker, removed with `down -v` afterwards).
- The callback arrived in Go on the SDK thread with the own Health. `oec_app_shutdown` ended the run, and it returned `OEC_OK`.
- `go vet` is clean when the handle goes through a C helper (`void *` from `uintptr_t`).
- The binary is 16 MB unstripped and links only system libraries, no `liboec`. `liboec_ffi.a` is 30 MB per target.
- Two cgo details showed up and go into the design: a Go-exported function cannot be referenced as a C function pointer from Go (it needs a C trampoline file), and the `oec_received` parameter is `const` in C but not in the cgo export.

## Decision

**A: an idiomatic Go package over the C ABI `oec-ffi` (cgo), linked statically.** The protocol and the runtime stay in Rust once; Go adds no protocol logic. This revises the WASM recommendation of `research/sdk-options.md` §9.6. That recommendation assumed a thin runtime. Since C-050 … C-056 the runtime holds the transport, the bootstrap, the store and the Config handling, and the C ABI is complete.

1. **Location and module:** `oec/sdk/go`, its own Go module `github.com/OI4/oec/sdk/go` (the same naming as `github.com/OI4/oec/conformance`; the import path for publication is decided in P9/Q-011), package `oec`. Go version as in `oec/conformance` (`go 1.26`, toolchain pinned). **Standard library only.**
2. **Linking:**
   - Static `liboec_ffi.a` by default: a self-contained binary with no shared `liboec` at run time, which suits containers.
   - The package finds the archive in `oec/sdk/go/lib/<GOOS>_<GOARCH>/`. That directory is git-ignored and filled by a script of the package (`cargo build --release -p oec-ffi` and copy).
   - The system libraries come from `rustc --print native-static-libs` (ADR-0021 §9): macOS `-lSystem -lc -lm`; Linux glibc `-lgcc_s -lutil -lrt -lpthread -lm -ldl -lc`.
   - `CGO_LDFLAGS`/`CGO_CFLAGS` can override the default. The header is the committed `oec-ffi/include/oec.h`, never a copy.
   - Prebuilt archives and a release of the module come with P9.
3. **ABI check:** at the first use the package compares `oec_abi_version()` with its constant 1 and fails clearly on a mismatch.
4. **API** (idiomatic Go; the concepts of ADR-0020, the reach of ADR-0021):
   - `Product`, `New(Options) (*App, error)` with level, `ServiceType`, container storage or explicit broker/credentials/certificate/IL, serial number, store directory and Syslog level.
   - `App.Run(ctx)` calls `oec_app_run` on a locked goroutine, and cancelling `ctx` calls `oec_app_shutdown`. `App.Start()`, `App.Close()`.
   - `App.This()`, `App.AddAsset(...)`, `Asset.SetHealth`, `Asset.EmitEvent`, `App.Syslog`.
   - `Asset.AddData(def) (*DataSet, error)` with `Update(v any)` (encoding/json) and `OnSet(func(json.RawMessage) error)`.
   - `Asset.AddConfig(def) (*Config, error)` with `Get`/`Set`/`OnSet`.
   - Consumer: `App.Subscribe(Resource, Filter, func(Received)) (*Subscription, error)`; `App.Get`/`Set`/`Del(ctx, …)`, where the deadline of `ctx` becomes `timeout_ms`; `App.NewDataSetWriterID(ctx, …)`.
   - Errors are `*oec.Error` (status, message, OPC UA StatusCode) with sentinels for `errors.Is` (`ErrInvalidArgument`, `ErrNotConnected`, `ErrStatus`, …) mapped from `OEC_ERR_*`.
5. **Callbacks:**
   - A C trampoline file per callback type, with `runtime/cgo.Handle` as user data. The handle is freed with its subscription, DataSet or Config.
   - A Go panic inside a callback is recovered: an `OnSet` then answers Bad, and a subscription logs and drops the message. A panic must never unwind through Rust frames.
   - The ABI refuses blocking calls inside a callback (`OEC_ERR_INVALID_STATE`, ADR-0021 §6); the Go documentation says to start a goroutine for them.
6. **Memory:** C strings are freed with `defer C.free`, and strings returned by the SDK with `oec_string_free`. No Go pointer is kept by C (cgo rules; `GODEBUG=cgocheck=1` stays on in the tests).
7. **Reach:** the whole C ABI, Messaging Level 1 … 3. A missing function is added to `oec-ffi` first: ADR-0021 amendment, header diff in the gate, C test.
8. **Platforms:**
   - linux amd64/arm64 (the container targets) and darwin arm64/amd64 (development).
   - linux armv7 best effort with `crypto-ring` (Q-008).
   - Windows needs MinGW-w64 and the Rust target `x86_64-pc-windows-gnu` for cgo; it goes to P9, as the C tests on Windows (ADR-0021 §9).
   - Cross builds: per target in a container build (Rust stage builds `liboec_ffi.a`, Go stage with `CGO_ENABLED=1`), or with a C cross toolchain; documented in the README.
   - Static linking against musl for the images is decided with the image of the weather demo (P6).
9. **Tests and gate:**
   - Go tests (`go test -race`, `go vet`, `gofmt`) of the package run against the in-process TLS test broker of `oec` (`oec/tests/testbroker`). They use it either through a small Rust test binary that the Go tests start, or through a Rust test that builds and runs the Go test programs, as `oec-ffi/tests/c.rs` does with C. The choice is made in P4c increment 1.
   - Examples `hello` (L1) and an L3 asset source. D7 `test` against the Go examples at `level1` and `level3`: in the gate, if a broker without Docker suffices for it (e.g. the Rust test broker as a helper binary – decided in P4c increment 1 with reasons); otherwise as an opt-in run against `oec/dev` with the result in the log, as the P5 exit run against `demo.rs` (C-069).
   - Opt-in interop against `oec/dev`.
   - The SDK gate `oec/sdk/check.sh` gets a Go section.
10. **Versioning:** the Go module carries the SDK version of the Rust workspace; `oec.Version()` returns `oec_version()`.

## Consequences

- New phase **P4c – Go SDK (D2)** before P6 (PO order). The weather demo (D3) is written in Go on this package (Q-034); the logger stays C (Q-024), the config service Rust (Q-031). The three demos show the three front ends of D2.
- No new Rust dependency, no Go dependency outside the standard library. Licenses unchanged (MIT; Go standard library BSD-3-Clause, not linked into our sources but into binaries – named in the license texts of the images, G-4.8).
- No pure-Go cross compile and no `go get` without the Rust archive until P9 – accepted. Go users with containers build in a multi-stage Dockerfile anyway.
- D7 checks the Go applications like every other (ADR-0006).

## Amendment C-076 (2026-11-01) – increment 1 as built (agent, G-3.4 b)

Details: `log/2026-11-01_C-076_go-sdk-core.md`, REVIEW A-312 … A-317.

1. **§4 `Run(ctx)` revised: `oec_app_start` + `oec_app_wait`, never `oec_app_run`.** The signal handlers that `oec_app_run` installs (tokio, SIGINT/SIGTERM) lack `SA_ONSTACK`, which the Go runtime requires of non-Go handlers, and they would take the signals from the Go program. `App.Run(ctx)` therefore starts the app (if not yet started) and waits in slices of `oec_app_wait` until the run ends, `ctx` is done (→ `oec_app_shutdown`) or `Shutdown`/`Close` is called; a stop by `ctx`, `Shutdown` or `Close` returns nil, a start error (e.g. unusable TLS material) is returned as `*oec.Error`. The program handles signals itself (`signal.NotifyContext`). The two functions were added to `oec-ffi` for this (ADR-0021 amendment C-076).
2. **§4 thread-local errors:** each C call and its read of `oec_last_error`/`oec_last_status_code` run under `runtime.LockOSThread`, because the last error of the C ABI is per OS thread. No goroutine is locked for the lifetime of the app.
3. **§2 linking as built:** `link.go` carries the default `#cgo` flags per OS: `-I${SRCDIR}/../rust/oec-ffi/include` (the committed header, never a copy), `-L${SRCDIR}/lib/<GOOS>_<GOARCH> -loec_ffi` and the system libraries of `rustc --print native-static-libs` – darwin `-liconv -lc -lm` without `-lSystem` (the Go linker adds it; a second one gives a duplicate warning), linux `-lgcc_s -lutil -lrt -lpthread -lm -ldl -lc`. The build tag `oec_external` leaves the defaults out; then `CGO_CFLAGS`/`CGO_LDFLAGS` name everything (README). The archive is built by `go run ./cmd/oec-buildlib` (a Go program instead of a shell script: it runs on every development OS, finds the target directory with `cargo metadata`, maps the Rust target triple to `GOOS_GOARCH`, copies atomically; `-target`, `-profile`).
4. **§4 types:** `ServiceType` and `SyslogLevel` are the C values plus one, so the zero value means "any ServiceType" (wildcard of the consumer) or "default level"; `Health`, `Level`, `ConnectionState`, `StatusCode` keep the C values. Strings with a NUL character are refused with `ErrInvalidArgument` before the call.
5. **§4 lifetime:** `App` has no finalizer (native resources need an explicit `Close`, as `os.File` without its finalizer would); an RW lock keeps every call off a released handle, a second `Close` returns nil, a call after `Close` returns `ErrClosed`.
6. **§9 test harness decided: a Rust helper program the Go tests start** (`oec-ffi` example `go-testbroker`, source `oec-ffi/tests/go/testbroker.rs`). It wraps the routing variant of the in-process TLS test broker (`oec/tests/testbroker`, PKI generated per start into a temporary directory) and speaks JSON lines: the first line names port and PEM files, then one line per observed packet (connect with client id, user and Will topic; publish with topic and payload; subscribe; disconnect); stdin commands send a message to a connection or drop it. Reasons against the alternative (a Rust test that builds and runs Go programs, as `c.rs`): the Go tests stay ordinary `go test` with `-race`, every test gets its own broker, tests can inject messages and drops, and failures show in Go terms. The helper is a Cargo example (built by `clippy --all-targets`, not a published binary); no new dependency.
7. **§9 D7 in the gate:** a broker without Docker suffices – D7 `test --level 1` runs against `examples/hello` in `go test` (`d7_test.go`) with the TLS proxy of D7, the restart hook, `--peer` and `--announce`, PKI of the proxy generated with the Go standard library. `oec-check` is built from `oec/conformance` and run by exec (no code import, ADR-0006). The test skips without `OEC_GO_D7`; the SDK gate sets it.

## Amendment C-077 (2026-11-01) – increment 2 as built (agent, G-3.4 b)

Details: `log/2026-11-01_C-077_go-sdk-provider.md`, REVIEW A-318 … A-324. No change to `oec`/`oec-ffi`.

1. **§4 provider API as built:** `App.AddAsset(AssetOptions{Product, SerialNumber, IdentificationLink})`, `Asset.Remove`, `Asset.EmitEvent(Event{Level, Number, Description, Details any})`. `Asset.AddData(DataDef{Tag, Fields, Mode, Interval, Writable})` – `Field{Name, Type, Unit, Description, Range, Precision}` with the built-in types of MG 8.3.2; the Metadata and the `DataSetClassId` are derived by the SDK, so there is no separate Metadata API. `Mode` zero = the default `ON_REQUEST|FILTER`, `ModeOff` stands for `OFF_0`; `Interval` in whole milliseconds; `Precision` 0 = none. `DataSet.Update(any)`, `OnSet(func(json.RawMessage) error)`, `Remove`. `Asset.AddConfig(ConfigDef{Context, Label, Groups})` with typed `ConfigGroup`/`ConfigObject` (every element of MG 8.3.3; helpers `NumberObject`, `StringObject`, `BooleanObject`) and `AddConfigJSON` for a ready definition; `Config.Get` → `(value, ok, err)`, `Set`, `OnSet(func(ConfigValues) error)` with the whole proposal in clear text.
2. **§4 errors of handlers:** `StatusCode` implements `error`, so a handler returns e.g. `oec.BadOutOfRange`. nil → `Good`; a `StatusCode` or an `*Error` with a Bad/Uncertain code → that code; any other error → `Bad`.
3. **§5 handle lifetime revised:** the `cgo.Handle` of an `OnSet` target lives until `App.Close`, not until the DataSet/Config is removed: the C ABI requires `user_data` to stay valid until `oec_app_free` (a callback may still be running when `oec_dataset_remove` returns). One handle per DataSet/Config; a second `OnSet` swaps the Go function under a mutex instead of registering anew. Close frees all handles after `oec_app_free`.
4. **§5 panics:** a panic in a handler is recovered in the exported trampoline, logged with `log.Printf` and answered with `BadInternalError`.
5. **Locking:** every call on an asset, DataSet or Config goes through `App.do` (read lock of the app, locked OS thread, `ErrClosed` after Close); the handle list has its own mutex, so a handler that runs during Close cannot deadlock against the life lock.
6. **§9 L3 example and D7 `level3`:** `examples/source` – a Level 3 `OTConnector` representing a temperature sensor (writable `Oi4Pv` setpoint, `Config` with the poll interval, Generic event) after `oec/examples/device_source.rs`. D7 `test --level 3` against it runs in the gate (`TestD7Level3Source`): exit 0, 49 pass, 6 skip, 0 fail.

## Amendment C-078 (2026-11-01) – increment 3 as built, P4c exit (agent, G-3.4 b)

Details: `log/2026-11-01_C-078_go-sdk-consumer-exit.md`, REVIEW A-325 … A-333. No change to `oec`/`oec-ffi`.

1. **§4 consumer API as built:** `Resource` (the `oec_resource` values, `String` = the MG name), `Address{ServiceType, App, Source, Filter}` (App and Source as Identification Links in clear text; zero parts are wildcards of a subscription), `Received{Resource, ServiceType, App, Source, Filter, DataSetWriterID, Payload json.RawMessage}`. `App.Subscribe(r, at, fn) (*Subscription, error)`, `Subscription.Close` (idempotent, also inside its own callback and after `App.Close`); `App.Get(ctx, r, at) ([]Received, error)`, `App.Set`/`App.Del(ctx, r, at, payload any)` (payload marshalled with `encoding/json`, nil = none), `App.NewDataSetWriterID(ctx, r, at) (id uint16, ttl time.Duration, err error)`.
2. **§4 `context`:** the deadline of `ctx` is the timeout of the request in whole milliseconds (at least 1, clamped to 2^32−1 ms); without a deadline the SDK default (10 s) applies; a done `ctx` returns its error before the call; an SDK timeout at the deadline of `ctx` matches both `ErrTimeout` and `context.DeadlineExceeded`. The C ABI cannot abort a request in flight – a cancel without a deadline does not end it early (documented; an ABI cancel would be an additive extension if a demo needs it).
3. **§4 errors:** a Status event Bad of the responder is an `*Error` with `Status == StatusStatus` and its `StatusCode`; `errors.Is(err, oec.BadOutOfRange)` matches the code (`Error.Is` with a `StatusCode` target).
4. **§5 subscription handles:** the `cgo.Handle` of a subscription lives until `App.Close` – `oec_subscription_free` from another thread may still let the callback run once (ADR-0021 §6); `Subscription.Close` disables the Go function first, so no call reaches it afterwards. A closed subscription keeps one small handle until `App.Close` (bounded by the number of `Subscribe` calls).
5. **§5 callback rules for all callbacks:** `OnSet` handlers and subscription functions run on the SDK thread one at a time; blocking calls inside (`Get`, `Set`, `Del`, `NewDataSetWriterID`, `App.Close`) return `ErrInvalidState`; panics are recovered and logged (an `OnSet` panic is answered `BadInternalError`, a subscription panic drops the message). Tested with `-race`.
6. **Exit criterion 1:** `abi_test.go` (`TestABICoverage`) parses `oec.h` and the Go sources: all 45 functions are called except `oec_app_run`, which is listed as not wrapped (its signal handlers conflict with the Go runtime, A-314); the README carries the same table.
7. **§8 platforms as built:** darwin arm64 is built and tested in the gate; darwin amd64 is vetted in the gate (`CGO_ENABLED=1 GOARCH=amd64 go vet`: cgo compiles for x86_64, nothing links – the Homebrew `rustc` of the gate has no x86_64 std) and built opt-in (`OEC_GO_CROSS=1`, rustup target `x86_64-apple-darwin`). linux amd64/arm64: opt-in `cross/run.sh` – one `docker run --rm` of `rust:1.89-bookworm` (host platform) with the Debian cross toolchains and Go copied from `golang:1.27-bookworm`, then each binary runs once on its platform in `debian:bookworm-slim`; no image is built, pulled base images are removed again. The directory is `cross/`, not `build/` (git-ignored by the workspace).
8. **§9 opt-in interop:** `interop_test.go` (`OEC_GO_DEV=<oec/dev>`) runs a Level 3 provider on a copy of the sample storage `oec-dev-cert` (client certificate) and a consumer on a copy of `oec-dev-app` (password, probe user – the ACL lets only `oec-probe` send `Set`) against the Mosquitto of `oec/dev`: Get, Set and Subscribe pass. Copies keep `oec/dev/generated` clean.
