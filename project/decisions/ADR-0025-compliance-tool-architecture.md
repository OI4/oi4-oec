# ADR-0025 – Architecture of the compliance tools D7 `oec-check` and D8 `oec-infra`

- Status: proposed
- Date: 2026-10-26
- Deciders: product owner (Thomas) – proposed by the agent in C-044
- Related: ADR-0006 (independence of D7 from the SDKs, modes), ADR-0013/0014 (Go, one module, two binaries), ADR-0016 (levels), ADR-0018 (spec baseline), ADR-0019 §5 and amendment C-040 item 7 (`validate` contract), ADR-0023 (license rules, dual licenses), Q-008, Q-009, Q-011, Q-028, Q-029, Q-030; design: [`oec/conformance/DESIGN.md`](../../oec/conformance/DESIGN.md)

## Context

P2 delivered `oec-check validate`: a static checker of single messages, with exit codes 0–3 and a JSON report. P3 must decide how D7 grows into a live tester and how D8 is built. P5/P5b then implement the result.

The inputs:
- 434 active REQs. Their verification kinds:
  - 119 `d7-test`: needs a stimulus;
  - 47 `d7-monitor`: observable passively, sometimes over many messages;
  - 50 `d8`: container, image, host or broker;
  - 117 `manual`.
- ROADMAP P3 exit criterion 4: every `d7-test`/`d8-*` REQ maps to a planned test case.
- Q-009: the tools share only the schemas and the spec baseline with the SDKs.

## Decision

1. **Commands.**
   - `oec-check` has `validate` (unchanged), `monitor` (passive; `--record`/`--replay`), `test` (active against one CUT; default `--level 3` per REQ-CMP-011), `list`, `explain` and `version`.
   - `oec-infra` has `image`, `run`, `container`, `host`, `bus`, `list`, `explain` and `version`.
   - Both binaries live in the one module `oec/conformance` and may share `internal/` packages.
2. **Attachment modes for D7.**
   - Bus client is the default.
   - **Proxy:** D7 is the CUT's TLS MQTT endpoint and relays to the broker. This is the only black-box way to observe CONNECT, Will, TLS version, certificate verification and recovery.
   - Restart hook: a user-supplied command.
   - Peer: D7 plays a second component.
   - A catalog test declares what it `needs`. A missing need gives `skip` with a reason.
3. **Test catalog as data** (`oec/conformance/catalog/d7.yaml`, `d8.yaml`, embedded in the binaries).
   - Each test has an id, mode, level, REQs, needs and status.
   - `manual:` entries list live REQs that a black-box tool cannot observe, each with a reason.
   - `catalog_test.go` enforces, against `oec/spec`:
     - id rules;
     - mode ↔ verification kind;
     - REQ existence;
     - level = the lowest level at which one of the test's REQs applies;
     - **full coverage of every `d7-monitor`, `d7-test` and `d8` REQ**.
   - At C-044:
     - D7: 54 tests (19 monitor, 35 test) and 4 manual entries.
     - D8: 17 tests and 1 manual entry.
4. **Verdict model.**
   - Outcomes: pass/fail/warn/info/skip/manual.
   - The severity of each REQ comes from its obligation at the declared level (MUST → fail, SHOULD → warn, MAY → info). Ungraded REQs use their keyword.
   - Applicability is taken at run time from Profile, Config and the PublicationList.
5. **Exit codes.**
   - Codes 0–3 as in `validate`.
   - New: **4 = inconclusive.** No failure occurred, but a test with a MUST REQ at the declared level was skipped. `--allow-skip` maps 4 to 0.
6. **Reports.**
   - Formats: text; JSON (versioned by `report.schema.json`); JUnit XML. Several at once via `--report fmt=path`.
   - Each report names the tool, schema release, spec baseline, catalog version, declared level, CUT and attachment mode, and gives evidence per REQ.
7. **Libraries.**
   - `eclipse/paho.mqtt.golang` (client plus `packets` codec for the proxy). It is dual EPL-2.0/EDL-1.0; taken under **EDL-1.0** (BSD-3-Clause).
   - `google/go-containerregistry` (Apache-2.0).
   - Own minimal read-only Engine API client and a minimal SBOM reader.
   - An embedded test broker (`mochi-mqtt/server`, MIT) is allowed in tests only.
   - No cgo.
8. **Independence.**
   - `catalog/independence_test.go` fails if `go.mod` references an `oec/` module other than `spec` or `schema`.
   - Captures from SDK-based apps can serve as regression data, never as code.
9. **Self-test.**
   - A simulated CUT (`internal/sim`) with one injectable fault per catalog test.
   - An `implemented` test needs a positive and a negative fixture.
10. **Distribution.**
    - Static cross builds for windows/darwin/linux × amd64/arm64; linux/arm/v7 best effort.
    - A multi-arch `FROM scratch` image.
    - Release tooling and signing are decided in P9 together with Q-011.

## Consequences

- Without proxy mode, D7 cannot judge connection-level REQs (T7-CON-01…04, T7-LIFE-01…03). A bus-only run of a full level claim therefore ends as inconclusive (exit code 4), not as pass (Q-028, Q-030).
- `test` changes the CUT's state. D7 restores what it changed, reports what it could not restore, and refuses to run without `--app`.
- The catalog is the single list of D7/D8 tests. New REQs, or REQs whose verification changes, fail `oec/check.sh` until the catalog is updated. This keeps the spec and the tools in step at every re-baseline.
- The catalog levels are derived from the REQs. A test runs from the first level at which any of its REQs applies, and each REQ is judged at its own obligation. One test can therefore pass at L1 and fail at L2 for the same CUT behavior; this is intended.
- P5 starts with `internal/mqttwire` + `internal/capture` + `monitor --replay`. P5b starts with `image`.

## Alternatives considered

- **One binary with subcommands for D7 and D8.** Rejected: ADR-0014 decided two tools for two audiences (app vendors vs. platform/operations). The shared module already gives code reuse.
- **Tests in Go code only, no data catalog.** Rejected: coverage would not be checkable against the spec without parsing code, and `list`/`explain`/the reports would need a second registry.
- **Broker-side observation** (a plugin or log access for Mosquitto) instead of proxy mode. Rejected as the default: it ties D7 to one broker product and needs operator access. It remains possible later as an additional mode.
- **Exit code 1 for skipped MUST tests.** Rejected: it mixes "violates the spec" with "could not be checked". **Exit code 0 with warnings** was also rejected: a CI run would claim compliance it did not check.
- **`eclipse/paho.golang`.** Rejected: MQTT 5 only. **An own Go MQTT codec**: not needed, because the `packets` package of paho.mqtt.golang already exposes the codec.
- **The Docker SDK (`github.com/docker/docker/client`).** Rejected: a large dependency tree for a few read-only endpoints, and Podman compatibility is no better.
