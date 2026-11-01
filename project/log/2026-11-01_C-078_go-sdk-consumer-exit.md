# C-078 – P4c increment 3: Go SDK consumer, ABI coverage, cross builds, opt-in interop, P4c exit run

- **Date:** 2026-11-01
- **Phase:** P4c – Go SDK (D2 Go binding), increment 3 of 3 and exit (ADR-0027, A-312)
- **Baseline:** unchanged since C-076 – guideline `52d8470` (branch head `eb821e9`), SF-001 … 117 resolved, no open finding; 1411 inventory items, 444 REQs, 0 gaps, no REQ drafts; schema 2.0.0 frozen, `2.0.1-dev`; 55 D7 and 17 D8 tests `implemented`, report versions 1 frozen
- **Agent:** same session as C-076/C-077 (PO order of 2026-11-01: all of P4c in one go)

## Intent

ROADMAP P4c increment 3: the consumer side of the Go SDK (`Subscribe`, `Get`/`Set`/`Del` with `context` deadlines, `NewDataSetWriterID`), race/panic hardening of all callbacks, a test that compares the wrapped functions with `oec.h`, README and godoc complete, cross builds (darwin amd64, linux amd64/arm64), opt-in interop against `oec/dev` – then the **P4c exit run and assessment**. Nothing was open for the PO; no answer arrived.

## Work

### 1. Consumer API (tests first)

`consumer_test.go` first: a Level 3 provider (OTConnector with the pump `vendor.com/Pump/P-3/0815` and its writable DataSet `speed`, Setpoint 0 … 3000 decided by an `OnSet` handler) and a Level 2 consumer (Aggregation) on the in-process TLS test broker. Two SDK apps in one test need two hostnames – the MQTT client id is the hostname, so the consumer's storage gets `app-2`.

- `TestConsumerRequests`: Get Data/MAM, Set accepted and refused (`BadOutOfRange` via `errors.Is`), Del Data refused locally (REQ-TOP-013), Del ReferenceDesignation → `BadNotFound`, Get of an unknown asset → `BadNotFound` (SF-099), `NewDataSetWriterID` (id ≥ 10, ttl 60 s; unknown Source → `BadNotFound` (SF-113); a Filter → `ErrInvalidArgument`), invalid arguments.
- `TestConsumerContext`: deadline bounds a request (`ErrTimeout` and `context.DeadlineExceeded`), a done context is not sent, no deadline = SDK default, far deadlines clamped to 2^32−1 ms.
- `TestConsumerSubscribe`: delivery with all topic parts (ModeSource publishes on `Pub/Data/<Source>` without a Filter), a panic in the callback is recovered and logged (next message arrives), blocking calls inside → `ErrInvalidState`, `Close` inside the callback, `Close` after `App.Close`, `ErrClosed`.
- `TestResourceNames`.

Implementation `consumer.go`: `Resource`, `Address`, `Received`, `Subscription`; the trampoline `go_received_fn` in `callbacks.c`; `Error.Is` matches a `StatusCode` target. All pass with `-race` and `GODEBUG=cgocheck=1`; `TestConsumer*` 5× in a row green.

### 2. ABI coverage (exit criterion 1)

`abi_test.go` (`TestABICoverage`) reads `oec.h` (45 functions) and every non-test Go source of the package: 44 are called, `oec_app_run` is listed as not wrapped (signal handlers vs. Go runtime, A-314). A type conversion `C.oec_resource(r)` is not a call – a name with a typedef in the header is accepted. The README has the same table.

### 3. Godoc and README

`doc.go`: consumer section, `context` semantics, callback rules for `OnSet` handlers and subscriptions, `errors.Is` with a `StatusCode`. README: state line, consumer, C ABI coverage, other platforms, tests (cross, interop); the dev-environment hints now run on a copy of the sample storage (see §6).

### 4. darwin amd64

- In the gate (`oec/sdk/check.sh`, on darwin arm64): `CGO_ENABLED=1 GOARCH=amd64 go vet ./...` – cgo compiles for x86_64 with Apple clang, nothing is linked. A full build needs the Rust std for `x86_64-apple-darwin`, which the Homebrew `rustc` 1.89.0 of the gate does not have.
- Opt-in `OEC_GO_CROSS=1`: `go run ./cmd/oec-buildlib -target x86_64-apple-darwin` and `GOARCH=amd64 go build ./examples/...`; needs the rustup target (installed on this Mac with `rustup target add x86_64-apple-darwin`, toolchain stable 1.89.0; `PATH=$HOME/.cargo/bin:…` first). Result: archive ~21 s, `hello`/`source` Mach-O x86_64. The package tests as amd64 under Rosetta (without `-race`) pass; only `TestExampleHello`/`TestExampleSource` fail there because `xcrun` has no x86_64 slice on this Mac – a limit of the environment, not of the code.

### 5. linux amd64/arm64 (exit criterion 4)

`oec/sdk/go/cross/run.sh` + `build.sh` (opt-in, Docker and network): one `docker run --rm` of `rust:1.89-bookworm` (host platform) with the Debian cross toolchains, Go copied out of `golang:1.27-bookworm` (`docker create`/`cp`/`rm`), the source mounted read-only and tar-copied; `oec-buildlib` for `x86_64-`/`aarch64-unknown-linux-gnu`, `go vet` and `go build ./examples/...` for both. Then each binary runs once on its platform in `debian:bookworm-slim` and must end with the bootstrap error of the SDK (it loads and runs the Rust core). No image is built; base images pulled by the script are removed.

- Run: exit 0 (first run 2 min 17 s); rustc 1.89.0, go1.27.3; ELF x86-64 and aarch64, `hello` 19.3/18.2 MB, `source` 20.5/19.4 MB (debug info, not stripped); both platforms print `oec: bootstrap: … no hostname … (REQ-CNT-021)`.
- The first run left the other-platform variant of `debian:bookworm-slim` untagged (pulling one tag for two platforms in the classic image store). Removed by its id; `run.sh` now removes the run image after each platform. Second run: exit 0, no new container, volume, network or image, no pre-existing image removed.
- The directory is `cross/`: the workspace `.gitignore` ignores `build/`.

### 6. Opt-in interop against `oec/dev`

`interop_test.go` (`TestInteropDev`, skipped unless `OEC_GO_DEV=<oec/dev>`): provider Level 3 on a copy of the sample storage `oec-dev-cert` (client certificate), consumer on a copy of `oec-dev-app` (user name/password) with the credentials of the probe user – the ACL of `oec/dev` lets only `oec-probe` send `Set` (MG 9.1, REQ-GEN-001). Get MAM/Data, Set, Get after Set and a subscription to the `Pub/Data` of an update through Mosquitto. Run: `docker compose up -d --wait`, `go test -race -run TestInteropDev` → PASS (1.4 s), `docker compose down -v`; no container, volume, network or image left, `generated/` unchanged.

On the way: an app store `etc/oi4/app/oec/app/writer-ids` from a C-076 opt-in run of `hello` directly on `generated/roots/oec-dev-app` was still there – removed (own file; `generated/` is git-ignored and not committed). README now runs the examples on a copy.

## Defects found and fixed (test first)

- **Own new code:** `TestConsumerContext` failed once in the first gate run – the SDK's timer ended a moment before the deadline of `ctx`, so `ctx.Err()` was still nil and the timeout was not marked `context.DeadlineExceeded`. Fix: the timeout *is* the deadline of `ctx` by construction, so a timeout with a timeout taken from the deadline (not clamped) is always wrapped. 10× green afterwards.
- None in `oec`/`oec-ffi`.

## Agent decisions (G-3.4 b, for information: REVIEW A-325 … A-333)

- A-325 Consumer API as built (ADR-0027 amendment C-078 §1).
- A-326 `context`: deadline = timeout; no cancel of a request in flight (no ABI cancel; additive later if a demo needs it).
- A-327 Subscription handles live until `App.Close` (callback may run once after `oec_subscription_free` from another thread).
- A-328 Errors and callback rules: `errors.Is` with a `StatusCode`; one rule set for all callbacks.
- A-329 ABI coverage test; only `oec_app_run` not wrapped. A-324 stays: no IL accessor needed (the consumer receives ILs as text).
- A-330 darwin amd64: vet in the gate, full build opt-in (rustup target).
- A-331 linux amd64/arm64: opt-in `cross/run.sh` without image build; result above.
- A-332 Opt-in interop `TestInteropDev` against `oec/dev` with the probe user; stray store removed.
- A-333 P4c exit run and assessment (below); ROADMAP wording on darwin amd64 adjusted.

## Changes to reviewed items

None. No fixture, vector or REQ changed; no schema change.

## Spec findings

None (next SF-118).

## Dependencies

None new. Go: standard library only. Rust: none (the rustup target `x86_64-apple-darwin` is a local toolchain component for the opt-in build, not a dependency).

## D7 and D8 unchanged

No change under `oec/conformance`.

## P4c exit run and assessment

Exit run = the final gate of this cycle (`oec/check.sh` → "all checks passed") plus the opt-in runs of §4–§6.

| # | Criterion (ROADMAP P4c) | Result |
|---|---|---|
| 1 | The whole C ABI is reachable from Go; a test compares with `oec.h` | ✅ `TestABICoverage`: 45 functions, 44 wrapped, `oec_app_run` not wrapped (README) |
| 2 | Go examples pass D7 `level1` (`hello`) and `level3` (asset source), exit 0, no fail, in the gate | ✅ both in `go test` of the SDK gate, no Docker; the test fails on exit ≠ 0, on any fail and on a listed test that does not pass. L1 (`hello`): 21 pass, 5 warn (SHOULDs at Level 1), 29 skip, 0 fail (C-076); L3 (`source`): the 49 pinned pass, 6 skip, 0 fail |
| 3 | gofmt, vet, `go test -race` with `cgocheck=1` clean; standard library only; no new Rust dependency; `oec`/`oec-ffi` only additive (ABI version 1) | ✅ gate; `go.mod` has no `require`; `oec-ffi` changed only in C-076 (`oec_app_wait`/`oec_app_state`, additive, ABI 1) |
| 4 | Examples build for linux amd64/arm64 (container, opt-in, result in the log) and darwin arm64 | ✅ linux: §5; darwin arm64 in the gate; additionally darwin amd64 vet in the gate and opt-in build (§4) |
| 5 | README and godoc: build, linking, overriding the flags, threading rules of callbacks | ✅ `oec/sdk/go/README.md`, `doc.go` |
| 6 | `oec/check.sh` green | ✅ (Checks below) |

Deviation from the cycle-cut wording: "on macOS darwin arm64/amd64 in the gate" – darwin amd64 is vet-only in the gate (no x86_64 Rust std in the gate's toolchain), the full build is opt-in (A-330). Not an exit criterion (criterion 4 names darwin arm64).

**Assessment:** all six exit criteria are met. Carried: Windows (MinGW), prebuilt archives and publication of the module → P9 (Q-011); the C ABI has no cancel of a request in flight (A-326) and no IL getter (A-324) – both additive if P6 needs them. **Proposal: close P4c and start P6** (first cycle with ADR-0028, increment 1 = logger core in C, A-307) – D-1 for the PO.

## Risks and open technical points

- The Go part of the gate is ~3 min (D7 L1 ~2 min, L3 ~45 s, consumer ~10 s).
- A closed subscription keeps one `cgo.Handle` until `App.Close` (A-327) – bounded by the number of `Subscribe` calls; a demo that subscribes and unsubscribes in a loop for a long time would grow slowly. Fixable additively (an ABI "callback finished" guarantee) if it matters.
- The linux cross build depends on public images and Debian packages (opt-in only).

## Commits

- `1170ace` – consumer API (`consumer.go`, `callbacks.c`, `errors.go`, `doc.go`) with `consumer_test.go` and `abi_test.go`
- `428cb36` – `cross/run.sh`/`build.sh`, `interop_test.go`, `oec/sdk/check.sh` (darwin amd64 vet, `OEC_GO_CROSS`), README
- this commit – docs: log, ADR-0027 amendment C-078, ROADMAP P4c (C-078 ✅, exit assessment), STATUS, REVIEW/REVIEW_DE (D-1, A-325 … A-333; also the overview rows A-318 … A-324 and the C-077 history entry that C-077 had left out)

## Checks (final)

`oec/check.sh` → "all checks passed" (Go section `go test -race` 174 s including D7 L1/L3 and the consumer tests; the first run failed in `TestConsumerContext`, see Defects).

## For the PO

**D-1:** close P4c (all six exit criteria met) and start P6 – recommended (a). A-325 … A-333 for information. No R-x (no test data), no SF.

## Handover (G-3.10)

**New agent session** for the next cycle: P4c is complete and P6 is a new phase with other deliverables (C logger, Rust config service, Go weather service) and mostly other files; the context of this session was compacted. `prompt.md` (workspace root, German, not committed) holds the start prompt of C-079: reading order incl. this log, ADR-0027, ADR-0021, ADR-0016/0017 and ROADMAP P6; baseline; D-1 open; G-3.4; tasks; constraints; cycle-end deliverables.

## Next

PO: D-1. After D-1 (a): C-079 – first P6 cycle: ADR-0028 (demo architecture: Web UI per D-3 (a), ports, images), increment 1 = logger core in C (A-307).
