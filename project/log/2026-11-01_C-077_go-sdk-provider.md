# C-077 – P4c increment 2: Go SDK provider Level 2/3 (assets, Data, Config, events, `OnSet`), L3 example `source`, D7 `level3` in the gate

- **Date:** 2026-11-01
- **Phase:** P4c – Go SDK (D2 Go binding), increment 2 of 3 (ADR-0027, A-312)
- **Baseline:** unchanged since C-076 – guideline `52d8470` (branch head `eb821e9`), SF-001 … 117 resolved, no open finding; 1411 inventory items, 444 REQs, 0 gaps, no REQ drafts; schema 2.0.0 frozen, `2.0.1-dev`; 55 D7 and 17 D8 tests `implemented`, report versions 1 frozen
- **Agent:** same session as C-076 (PO order of 2026-11-01: all of P4c in one go)

## Intent

ROADMAP P4c increment 2: the provider side of the Go SDK at Level 2/3 – represented assets, `Data` with `Update`/`OnSet` (and the Metadata), `Config` with `Get`/`Set`/`OnSet`, events; C trampoline in its own `.c` file with `cgo.Handle`, panics recovered; an L3 example (asset source) and D7 `test` at `level3` against it in the gate. Nothing was open for the PO; no answer arrived.

## Work

### 1. Research (no code)

- The provider part of the C ABI (`oec_app_add_asset` … `oec_config_set`, 14 functions) and its contracts: `user_data` of `oec_dataset_on_set`/`oec_config_on_set` must stay valid until `oec_app_free`; callbacks run on a runtime thread, blocking calls inside return `OEC_ERR_INVALID_STATE`.
- The Rust model behind it: `PublicationSpec` (default Mode `ON_REQUEST|FILTER`), Config validation and masking (`oec/src/config.rs`, `oec-core/src/resource/config.rs`), `oec/examples/device_source.rs` (the L3 model).
- D7 at Level 3: T7-SET-03 is the only test above Level 2. The ReferenceDesignation (from Level 2) needs a persistent store; the container storage gives one.
- **No Close deadlock:** callbacks run only while the run lives; `Close` shuts the run down and waits before it takes the exclusive life lock, so a handler that calls the SDK cannot block `Close`.

### 2. Provider API (`721db4a`, tests first)

- `provider.go` – `AssetOptions`, `App.AddAsset`, `Asset.Remove` (a later `SetHealth` has no effect, as in Rust), `Event` + `Asset.EmitEvent`, `BuiltinType`, `Range`, `Field`, `Mode` (zero = default, `ModeOff` for `OFF_0`), `Writable`, `OI4PV`, `DataDef` (`Interval` whole ms), `Asset.AddData`, `DataSet.Update(any)`/`OnSet`/`Remove`.
- `config.go` – `ConfigType`, `ConfigObject` (every element of MG 8.3.3), `NumberObject`/`StringObject`/`BooleanObject`, `ConfigGroup`, `ConfigDef` (→ the MG 8.3.3 JSON), `Asset.AddConfig`/`AddConfigJSON`, `Config.Get`/`Set`/`OnSet`, `ConfigValues` with `Value`/`Number`/`Bool`.
- `callbacks.c` – the trampoline `go_set_fn` and `go_handle` (the cgo traps of the spike: no `//export` against a `const` declaration in the preamble, no `unsafe.Pointer(uintptr)`).
- `callbacks.go` – `setTarget` (one per DataSet/Config, swappable function), `App.onSet` (handle created, deleted on failure, else kept), `//export goSet` with `recover` (panic → log + `BadInternalError`), `statusCodeOf`.
- `app.go` – `App.do` (read lock, locked OS thread, `ErrClosed`), handle list under its own mutex, freed after `oec_app_free`; `types.go` – `StatusCode` implements `error`.
- `provider_test.go` – against the TLS test broker: `TestProviderData` (MAM/Health/Profile/Data of an asset, Set accepted → Status Good and new state, refused by `BadOutOfRange` and by a plain error → `Bad`, panic → `BadInternalError`, `Update` with a wrong type → `ErrInvalidArgument`, `OnSet` swap, `Remove` of DataSet and asset, `This().Remove()` refused), `TestDataDefRejects`, `TestStatusCodeOf`, `TestProviderConfig` (Get/Set from Go, Set Config by the broker accepted/refused, masking of a sensitive value), `TestConfigDefRejects`; calls after `Close` → `ErrClosed`.

### 3. L3 example `source` and D7 `level3` (`c1d668c`)

- `examples/source` – Level 3 `OTConnector` "Demo Connector Go" (`CONN-GO-1`) representing the temperature sensor `sensor-maker.com/TS-100/TS-100-A`, serial `4711` (Oi4Identifier, no IL of its own): `Oi4Pv` with `Pv` (precision 0.1 °C) and the writable setpoint `Sv1` (−20 … 80 °C, handler refuses with `BadOutOfRange`), `Writable` Mode and Interval; `Config` `acquisition/common/interval` (poll interval, 0.1 … 60 s); a Generic event "setpoint reached" per setpoint; SIGINT/SIGTERM via `signal.NotifyContext`.
- `TestExampleSource` – builds and runs it: Oi4Pv `{"Pv":21.0,"Sv1":22.0}`, Set Config of the interval → Good, Set `Sv1` 99 → Bad `BadOutOfRange`, `Sv1` 23 → Good, the Generic event, SIGTERM → DISCONNECT, exit 0.
- `d7_test.go` – the L1 run is now the helper `runD7`; new `TestD7Level3Source`: `oec-check test --level 3` with proxy, restart hook, peer and announce against `source`. **Result: exit 0 – 49 pass, 0 warn, 6 skip, 0 fail, ~45 s.** All 49 passes are pinned. Skipped, as for the Rust demo since C-067: T7-MON-07, T7-MON-17, T7-ADR-01, T7-ADR-02 (they judge requests of the CUT – `source` sends none) and T7-SET-04, T7-SET-10 (they need a ReferenceDesignation object; the fresh store of the run has none, the SDK creates one only on a `Set` from outside). T7-SET-03 (Level 3) passes.

### 4. Docs

`README.md` (state, provider section with example, `OnSet` rules, callbacks, D7 L3), `doc.go` (provider, handler rules), ADR-0027 amendment C-077, ROADMAP P4c, STATUS, REVIEW/REVIEW_DE.

## Defects found and fixed (test first)

None in `oec`/`oec-ffi`. Own new code: a doubled value is published as `21.0` (MG `Double`), the example test expected `21` – the test was wrong.

## Agent decisions (G-3.4 b, for information: REVIEW A-318 … A-324)

- A-318 Provider API as built (ADR-0027 amendment C-077 §1); Metadata derived, no own API.
- A-319 Handler results: `StatusCode` is an `error`; nil → Good, Bad code → that code, other errors → Bad; panic → `BadInternalError` + log.
- A-320 `cgo.Handle` lives until `App.Close` (deviation from ADR-0027 §5 "freed with its object", required by the `user_data` contract of the ABI); one per DataSet/Config, a second `OnSet` swaps the function.
- A-321 `App.do` and a separate mutex for the handle list (no lock-order cycle with `Close`).
- A-322 L3 example `source` after `device_source.rs`.
- A-323 D7 `level3` against `source` in the gate: 49 pass, 6 skip, 0 fail; the skips equal those of the Rust demo.
- A-324 No ABI accessor for the Identification Link of an asset/app; not needed so far (tests compute it). Added in C-078 only if the consumer or the demos need it.

## Changes to reviewed items

None. No fixture, vector or REQ changed; no schema change.

## Spec findings

None (next SF-118).

## Dependencies

None new. Go: standard library only.

## D7 and D8 unchanged

No change under `oec/conformance`.

## Risks and open technical points

- The gate is ~1 min longer (D7 L3 ~45 s, provider and example tests).
- T7-SET-04/T7-SET-10 are not exercised against a Go app (no ReferenceDesignation object); they are exercised by the sim fixtures of D7 (C-067).

## Commits

- `721db4a` – Go provider API (`provider.go`, `config.go`, `callbacks.go`/`.c`, `app.go`, `types.go`, `errors.go`) with tests
- `c1d668c` – example `source`, `TestExampleSource`, `runD7` + `TestD7Level3Source`
- this commit – docs: log, README/godoc, ADR-0027 amendment C-077, ROADMAP P4c, STATUS, REVIEW/REVIEW_DE

## Checks (final)

`oec/check.sh` → "all checks passed" (Go section `go test -race` 176 s including D7 L1 and L3).

## For the PO

Nothing to decide. No R-x (no test data), no D-x, no SF. A-318 … A-324 for information. P4c continues with C-078 without a pause (PO order).

## Next

C-078 – P4c increment 3 and exit: consumer (`Subscribe`, `Get`/`Set`/`Del`, `NewDataSetWriterID`), callback hardening, ABI coverage test, cross builds, opt-in interop, README/godoc complete, exit run and assessment.
