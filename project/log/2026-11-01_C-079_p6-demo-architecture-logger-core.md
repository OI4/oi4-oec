# C-079 – P6 start: demo architecture (ADR-0028), increment 1 logger core in C

- **Date:** 2026-11-01
- **Phase:** P6 – Demo services (D3, D4, D10), increment 1 of 7 (ADR-0028, A-334)
- **Baseline:** unchanged since C-074 review addendum 2 – guideline `52d8470` (branch `feature/spec-findings`, head `eb821e9`), SF-001 … 117 resolved, no open finding; 1411 inventory items, 444 REQs, 0 gaps, no REQ drafts; schema 2.0.0 frozen, `2.0.1-dev`; 55 D7 and 17 D8 tests `implemented`, report versions 1 frozen
- **Agent:** new session (hand-over prompt of C-078 addendum 1)

## Intent

Start P6 after the PO closed P4c (C-078 addendum 1, D-1 (a)): confirm the cycle cut of A-306/A-307 and sharpen the exit, record the demo architecture as ADR-0028 and build increment 1 – the logger core D4 in C on `oec-ffi`: subscriptions, structured output, an own `Config` at Messaging Level 2, a history ring buffer; tests first (C unit tests under ASan/UBSan, integration tests against the in-process TLS test broker) and D7 `test --level 2` against the logger in the gate. No Web UI yet (increment 5). Nothing was open for the PO; no answer arrived.

## Plan

1. Baseline and gate check (green at the start: `oec/check.sh` "all checks passed").
2. ADR-0028, ROADMAP P6 (cut confirmed with cycles C-079 … C-085, exit sharpened).
3. Logger: C unit tests and Go integration tests first, then the implementation in `oec/demos/logger`, `oec/demos/check.sh` as a step of `oec/check.sh`, D7 `level2` in the gate.
4. README, STATUS, REVIEW/REVIEW_DE, hand-over.

## Work

### 1. Baseline, ADR-0028, ROADMAP P6

Baseline verified as in the hand-over prompt (guideline `52d8470`, head `eb821e9`; 1411 items, 444 REQs, 158 `schema`/`d7-validate` REQs, 0 gaps; fixtures 29/59/19/29/69/26/20 reviewed; 55 D7 and 17 D8 tests `implemented`; 29 entry schemas, 245 payload and 18 `broker.json` vectors; Go SDK 44 of 45 ABI functions). Gate green at the start. ADR-0028 (demo architecture) and ROADMAP P6 (cut confirmed with C-079 … C-085, exit sharpened to 7 criteria) committed first (`7e364d8`).

### 2. Logger core (tests first)

`oec/demos/logger/` – C17 on `oec-ffi`, linked statically with `liboec_ffi.a`:

- **Core free of `oec.h`** (`src/json`, `src/core`, `src/history`) so it is unit-testable without the SDK. The headers and `test/unit.c` (11 test groups: JSON valid/invalid, access, write, settings defaults vs. Config definition, single `Set`, proposals, levels, filter, accept, records, history) were written first and failed (empty implementation, no link); then the implementation. One error in the test itself (expected UTF-8 length 9, correct 8) fixed in the test.
- **JSON:** an own small reader (RFC 8259 incl. UTF-8 validation, escaped surrogate pairs, depth limit 64, `\u0000` refused in strings the logger copies) and a string writer – no C dependency (A-338).
- **Config** (context `logger`, group `common`, A-337): `event`, `health`, `data` (Boolean), `sources` (String ≤ 2048, Identification Links, prefix of whole levels on AppId or Source), `minEventLevel` (Syslog names), `format` (`json`/`text`), `file` (name in `/opt/oi4/app`, Pattern + length 64, no path), `historySize` (Number 10 … 10000). The SDK validates (Values, Pattern, Min/Max) and persists; the logger applies the complete proposal atomically (file and history first, then the subscriptions) or answers `BadInvalidArgument`; `""` resets to the default (REQ-RES-CONFIG-007).
- **Event level** (A-337): Syslog → PRI % 8; Status → by the severity class of `Number` (Good → Informational, Uncertain → Warning, Bad → Error); others → Notice.
- **Records:** one JSON object or one text line per DataSetMessage (receive time, resource, ServiceType, AppId, Source, Filter, level for Events, Payload) to stdout, optionally appended to the file (rotation to `<file>.1` at 10 MiB, A-339); a failing file write is reported once to stderr and as a Syslog Error event.
- **History** (A-339): a mutex-protected ring of the last `historySize` record strings with sequence numbers, plus numeric series per (resource, AppId, Source, Filter, field) of `Data`/`Health` (240 points each, at most 256 series, overflow counted) – the data source of the Web UI (increment 5).
- **`src/main.c`:** product `oi4.org` / `OEC Logger` / `OEC-LOGGER` (A-336), `Utility`, Level 2, container storage at `OEC_ROOT` (default `/`); one subscription per selected resource with wildcards (`OEC_ST_ANY`, any app, source, filter); `oec_app_run` handles SIGTERM/SIGINT (Close message, exit 0).
- **Makefile** (GNU make; `make`, `make test`, `SANITIZE=1` → `target/asan/` with ASan + UBSan, `-fno-sanitize-recover=all`); C17 with `-Wall -Wextra -Wpedantic -Werror -Wshadow -Wstrict-prototypes -Wmissing-prototypes -Wformat=2 -Wvla`. ADR-0028 §7 said "POSIX Makefile" – corrected to GNU make in the same cycle (the ADR was accepted in this cycle; no reviewed item).

No C ABI gap: everything the logger needs is in `oec.h` (subscribe/free inside callbacks, `oec_config_on_set` with the complete proposal, `oec_config_get`, `oec_app_syslog`). A-326 (no cancel) and A-324 (no IL getter) are not needed; A-327 is Go-specific. No ADR-0021 amendment.

### 3. Test harness and integration tests

- **`oec/demos` Go module** (`github.com/OI4/oec/demos`, standard library only) with `internal/demotest` (A-340): the test broker `go-testbroker` as a program (events as JSON lines, `send` to a connection), container storages, the demo as a process with captured stdout lines, NetworkMessages of other applications for injection (DataSetClassIds of MG 9.1 as in the reviewed vectors V-PAY-003/058/062), `Get` and `Set` requests, and the D7 runner (proxy attachment, restart hook, peer, announce – ported from `oec/sdk/go/d7_test.go`). `reqs_test.go`: every REQ id cited under `oec/demos` exists and is active.
- **`logger/logger_test.go`** (against the ASan/UBSan build in the gate): `TestDefaults` (Config defaults = definition, Event and Health subscribed, Data not, the own Health logged as a JSON record – via a `Get Health`, since the Birth Health can precede the SUBSCRIBE), `TestOtherApplications` (Syslog Notice logged with level, Syslog Debug dropped, Health logged, Data not subscribed, Status Bad → Error), `TestSetConfig` (all objects at once: Data subscribed, source filter, Debug, text, file written; reset with `""`), `TestSetConfigRefused` (`historySize` 5, `file` `../escape.log`, `format` `xml`, `minEventLevel` `Loud` → Status Bad, nothing written, still JSON), `TestRestartKeepsConfig` (SIGTERM → exit 0 + Close message; values persisted), `TestD7Level2`. 10× repeated without a failure.
- **D7 `oec-check test --level 2`** against the logger: **46 pass, 9 skip, 0 fail, exit 0** (~35 s). Pinned: the 46 passes. Skipped by D7: T7-MON-07, T7-MON-17, T7-ADR-01, T7-ADR-02 (judge requests of the CUT – none sent), T7-MON-09, T7-MON-14 (judge Data/Metadata and cyclic publications – the logger has none), T7-SET-04, T7-SET-10 (need a ReferenceDesignation object), T7-SET-03 (Level 3).
- **`oec/demos/check.sh`** (step 7 of `oec/check.sh`, ~45 s): `liboec_ffi.a` (release) and `go-testbroker`, `make` and `make SANITIZE=1 all test`, gofmt, vet, `oec-check` into a temporary directory, `go test -race` with `OEC_DEMOS_REQUIRE=1`. `ASAN_OPTIONS=detect_leaks=0` on macOS (LeakSanitizer unsupported by Apple clang), `=1` elsewhere; `halt_on_error=1`.
- READMEs: `oec/demos/README.md` (overview, tests) and `oec/demos/logger/README.md` (build, run, Config objects, record format, tests, code map; no Web UI and no authentication yet).

## Defects found and fixed (test first)

- None in `oec`/`oec-ffi`/D7. In the new code: the expected UTF-8 length in a unit test (test error); `TestDefaults` first waited for the Birth Health, which can precede the SUBSCRIBE at the broker (race in the test design) – it now asks with `Get Health`; a refused-Set check expected a new own Health (only every 60 s) – it now injects one. Build: `-Wformat-nonliteral` in the error report (fixed by a fixed format), a duplicate-library linker warning on macOS (no explicit `-lSystem -lc`).

## Agent decisions (G-3.4 b, for information: REVIEW A-334 … A-340)

- A-334 Cycle cut of A-306/A-307 confirmed (C-079 … C-085, one increment per cycle), exit sharpened (7 criteria).
- A-335 ADR-0028 demo architecture (layout, ports 8781/8782/8783 on localhost, images, compose profile `demos`, offline testing, D7 at the declared level in the gate); §7 corrected to GNU make.
- A-336 Identity of the logger: `oi4.org` / `OEC Logger` / `OEC-LOGGER`, Manufacturer "Open Industry 4.0 Alliance", SoftwareRevision 0.1.0, `Utility`, SerialNumber = hostname.
- A-337 Logger `Config` (8 objects, atomic apply, `""` = default) and the Event level mapping (Syslog PRI % 8; Status by severity class; others Notice).
- A-338 Own JSON reader/writer in C, no C dependency (the JSON the logger reads is the SDK's validated output; ~350 lines with unit tests).
- A-339 History (records ring + numeric series 240 × ≤ 256) and the log file (in `/opt/oi4/app`, name only, rotation at 10 MiB to `<file>.1`, one error report until a write succeeds).
- A-340 Test harness `oec/demos/internal/demotest`, gate step `oec/demos/check.sh`, D7 L2 pinned list (46).

## Changes to reviewed items

None. No fixture, vector or REQ changed; no schema change. ADR-0028 §7 wording (accepted in this cycle, not reviewed) corrected.

## Spec findings

None (next SF-118).

## Dependencies

None new. C: only libc/POSIX and `liboec_ffi.a`. Go: standard library only (`oec/demos/go.mod` has no `require`). Rust: none.

## D7 and D8 unchanged

No change under `oec/conformance`, the SDKs or `oec-ffi`.

## Risks and open technical points

- macOS has no LeakSanitizer: leaks of the logger are only found on Linux (`detect_leaks=1` there).
- The demos step adds ~45 s to the gate (D7 L2 ~35 s).
- The record `time` is the receive time, not the DataSetMessage `Timestamp` (which is optional); the Payload keeps everything else.
- The history is only unit-tested; its HTTP/SSE view comes with increment 5 (C-083).

## Commits

- `7e364d8` – ADR-0028, ROADMAP P6, log skeleton
- `55a269c` – logger core (C), Go harness and integration tests, D7 L2, `oec/demos/check.sh` in `oec/check.sh`, READMEs
- this commit – docs: log, ADR-0028 §7 (GNU make), ROADMAP (C-079 ✅), STATUS, REVIEW/REVIEW_DE (A-334 … A-340)

## Checks (final)

`oec/check.sh` → "all checks passed" (demos step: unit tests ok, `oec/demos` 1.2 s, `oec/demos/logger` 40 s incl. D7 L2). Load at the start 1.2.

## For the PO

Nothing to decide: no R-x (no new test data with a verdict), no D-x, no SF. A-334 … A-340 for information.

## Handover (G-3.10)

**New agent session** for C-080: the next increment is the weather service in Go on the Go SDK – another language, other files (the Go SDK, the MET Norway terms of ADR-0010 for C-081), and the context of this session was compacted. The shared harness `oec/demos/internal/demotest` is ready for it. `prompt.md` (workspace root, German, not committed) holds the start prompt of C-080.

## Next

C-080 – weather core in Go (increment 2, A-307): simulation provider, station assets, `Data`/`Metadata`/`Config` (A-302), Level 3, offline tests, D7 `test --level 3` in the gate.
