# Review and decisions – open items for the PO

_Last updated: 2026-10-26, after C-044 (guideline `168e1d1`, register `f80eba7`)._

This is the single list of everything that waits for the PO:
- items to **review**: drafts, and changes to reviewed items;
- items to **decide**: spec findings, open questions, cycle plan.

Each cycle updates it. Items leave the list once the PO has answered and `spectool review` (or the decision record) has been applied. Details are in the cycle logs; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md), with the same numbering.

**How to answer:** tick the box or write the decision into the "PO" field. You can also answer in the chat, e.g. "D-1 … D-19 wie empfohlen, R-1 … R-5 akzeptiert". Anything not accepted gets a note, and the agent reworks it in C-045.

**Current state:** P3 design, cycles C-042 (SDK APIs), C-043 (libraries) and C-044 (compliance tools) are done. They produced:
- **19 decisions**:
  - the P3 plan;
  - 6 proposed ADRs;
  - 10 open questions;
  - 2 spec findings.
- **5 review items**: sketches, spikes, test catalog, tool design.

No REQ, vector, matrix entry or schema changed in P3, so there is nothing for `spectool review`. Coverage stays at 0 gaps. Schema 2.0.0 is unchanged.

Logs:
- [C-042](log/2026-10-26_C-042_p3-sdk-api.md)
- [C-043](log/2026-10-26_C-043_p3-crates.md)
- [C-044](log/2026-10-26_C-044_p3-tools.md)

**How ADRs and questions relate:** each ADR is written with the recommended answer of its questions. If you choose another option, C-045 amends the ADR before it is set to `accepted`.

## Overview

| # | Item | Kind | Source | Recommendation | PO |
|---|------|------|--------|----------------|----|
| D-1 | P3 plan: cycle cut C-042 … C-045, open points, dependencies, exit criteria (ROADMAP P3) | decide | C-042 | accept | ☐ |
| D-2 | ADR-0020 Rust SDK public API (D2) | decide | C-042 | accept | ☐ |
| D-3 | ADR-0021 C ABI `oec-ffi` | decide | C-042 | accept | ☐ |
| D-4 | ADR-0022 micro SDK API (D9) | decide | C-042 | accept | ☐ |
| D-5 | ADR-0023 Rust SDK dependencies | decide | C-043 | accept | ☐ |
| D-6 | ADR-0024 micro SDK libraries (revises the open JSON candidates of ADR-0017) | decide | C-043 | accept | ☐ |
| D-7 | ADR-0025 compliance tool architecture (D7/D8) | decide | C-044 | accept | ☐ |
| D-8 | Q-021 execution model of the Rust SDK | decide | C-042 | (a) blocking by default | ☐ |
| D-9 | Q-022 Config access | decide | C-042 | (a) string-keyed getters first | ☐ |
| D-10 | Q-023 C ABI option style | decide | C-042 | (a) builder + string setters + JSON | ☐ |
| D-11 | Q-024 logger demo in C | decide | C-042 | (a) yes | ☐ |
| D-12 | Q-025 micro SDK prefix | decide | C-042 | (a) `oecm_` | ☐ |
| D-13 | Q-026 MQTT client of the Rust SDK (`rumqttc` is out) | decide | C-043 | (a) own sans-IO codec | ☐ |
| D-14 | Q-027 JSON reader of the micro SDK | decide | C-043 | (a) own bounded pull reader | ☐ |
| D-15 | Q-028 proxy mode required for a full level claim | decide | C-044 | (a) yes | ☐ |
| D-16 | Q-029 tester identity of D7 | decide | C-044 | (a) not announced by default | ☐ |
| D-17 | Q-030 exit code 4 "inconclusive" | decide | C-044 | (a) yes | ☐ |
| D-18 | SF-101 PublicationList `Mode` and `Get` | decide | C-042 | proposed interpretation + text | ☐ |
| D-19 | SF-102 `ServiceType` of a directly connected smart device | decide | C-042 | `OTConnector` + text | ☐ |
| R-1 | Rust API sketch with DX examples (`oec/sdk/rust/sketch`) | review | C-042 | accept | ☐ |
| R-2 | C ABI sketch `oec.h` and D9 sketch `oecm.h` | review | C-042 | accept | ☐ |
| R-3 | Spikes `schema-embed` and `mqtt-codec` | review | C-043 | accept | ☐ |
| R-4 | Test catalog D7/D8 and its gate (`oec/conformance/catalog/`) | review | C-044 | accept | ☐ |
| R-5 | Tool design `oec/conformance/DESIGN.md` | review | C-044 | accept | ☐ |

## Decisions

### D-1 – P3 plan and cycle cut

- **Where:** [ROADMAP P3](ROADMAP.md) (section "P3 – SDK and conformance tool design"). The reason for the split is in the [C-042 log](log/2026-10-26_C-042_p3-sdk-api.md).
- **Content:**
  - The hand-over was split into C-042 (SDK APIs), C-043 (libraries), C-044 (tools) and **C-045** (exit review after your answers).
  - The table of open points collected from ADR-0011/0014/0017/0019 shows each with the cycle that closes it.
  - Dependencies:
    - P4 needs ADR-0020/0021/0023;
    - P4b needs ADR-0022/0024 and does not depend on P4;
    - P5/P5b need ADR-0025;
    - P6 needs Q-024.
  - Five concrete exit criteria. Criteria 3 and 4 are already gates in `oec/check.sh`: the sketches compile, and the catalog covers every REQ.
  - ROADMAP P4 got increment 10 (C ABI).
- **PO:** ☐

### D-2 – ADR-0020 Rust SDK public API (`proposed`)

- **Where:** [ADR-0020](decisions/ADR-0020-rust-sdk-public-api.md), sketch `oec/sdk/rust/sketch`.
- **Content:**
  - The sketch is the API contract for P4.
  - Blocking by default (Q-021).
  - **The level is chosen, the protocol is automatic.** `build()` rejects configurations that cannot keep the level.
  - Identity (IL/Oi4Identifier) and the optional container-storage bootstrap.
  - TLS only.
  - SDK-owned resources: MAM, Health, Profile, PublicationList, SubscriptionList, ReferenceDesignation, Metadata, Status events.
  - **The application sets state, the SDK publishes** according to the PublicationList (Mode/Interval/Precisions).
  - `Get`, pagination, Locale and Query are invisible to the app.
  - A `Set`/`Del` pipeline with an automatic Status event.
  - Config, events and Syslog.
  - A consumer API.
  - Every decision names its REQ ids; a test checks that the cited REQs exist.
- **PO:** ☐

### D-3 – ADR-0021 C ABI `oec-ffi` (`proposed`)

- **Where:** [ADR-0021](decisions/ADR-0021-c-abi-oec-ffi.md), `oec/sdk/rust/sketch/c/include/oec.h`.
- **Content:**
  - Prefix `oec_`.
  - Opaque handles with clear ownership:
    - the caller frees with `oec_*_free`;
    - child handles belong to their app;
    - the builder is consumed even on error.
  - UTF-8 strings: inputs are copied; strings passed to a callback are borrowed for the callback.
  - Structured values as JSON text.
  - Builder functions instead of option structs (Q-023).
  - Errors:
    - `oec_status` codes plus `oec_last_error()` per thread;
    - Rust panics never cross the ABI;
    - protocol StatusCodes are kept apart from API errors.
  - Threading:
    - every function may be called from any thread;
    - callbacks run one at a time on an SDK thread;
    - no async in the ABI.
  - Versioning: `OEC_ABI_VERSION` / `oec_abi_version()`.
  - The behavior is exactly ADR-0020.
  - Proof by a C logger demo (Q-024).
- **PO:** ☐

### D-4 – ADR-0022 micro SDK API (`proposed`)

- **Where:** [ADR-0022](decisions/ADR-0022-micro-sdk-api.md), `oec/sdk/micro/sketch/include/oecm.h`.
- **Content:**
  - The header is the contract for P4b increment 1 (Level 1).
  - Prefix `oecm_` (Q-025).
  - No heap.
  - A borrowed `const` configuration.
  - A single-threaded event model.
  - The PAL is a struct of function pointers.
  - A Will on every connect.
  - A wall clock is mandatory.
  - Level 2/3 come as additions.
  - Device identity defaults use `OTConnector` (SF-102).
  - The table of shared concepts and deliberate differences to the C ABI is in the ADR.
- **PO:** ☐

### D-5 – ADR-0023 Rust SDK dependencies (`proposed`)

- **Where:** [ADR-0023](decisions/ADR-0023-rust-sdk-dependencies.md).
- **Content:**
  - tokio.
  - An own MQTT codec (Q-026).
  - rustls 0.23 + tokio-rustls with aws-lc-rs: TLS 1.2/1.3 only, trust only from the bus storage.
  - serde_json.
  - jsonschema `=0.60.1` behind the cargo feature `strict-validation` (off in products, on in SDK tests).
  - thiserror, `log`, `time`, uuid.
  - cbindgen as a CLI.
  - cargo-deny from P4 (no EPL/MPL/GPL in linked code).
  - MSRV 1.89.
  - cargo-zigbuild for Linux arm64/ARMv7.
- **Important finding:** `rumqttc`, the assumed MQTT client, is effectively unmaintained and pulls in a vulnerable `rustls-webpki` (see D-13).
- **PO:** ☐

### D-6 – ADR-0024 micro SDK libraries (`proposed`)

- **Where:** [ADR-0024](decisions/ADR-0024-micro-sdk-libraries.md).
- **Content:**
  - The latest ESP-IDF plus esp-mqtt as a managed component, using a checked procedure for a new Will on each reconnect (to be confirmed on hardware in P4b).
  - An own JSON encoder and an own bounded reader (Q-027).
  - Unity/CTest; sanitizers and fuzzing on the host.
  - The Arduino port runs on esp-mqtt from the core; the fallback is arduino-mqtt.
  - M5Unified only in the demo.
- **Effect on a reviewed item:** ADR-0017 named "cJSON or jsmn" as the JSON candidates and left the choice open. The facts changed: jsmn is abandoned, and cJSON is no longer bundled with IDF 6. If you accept D-6, C-045 adds an amendment note to ADR-0017.
- **PO:** ☐

### D-7 – ADR-0025 compliance tool architecture (`proposed`)

- **Where:** [ADR-0025](decisions/ADR-0025-compliance-tool-architecture.md), [`oec/conformance/DESIGN.md`](../oec/conformance/DESIGN.md).
- **Content:**
  - D7 `oec-check`:
    - `monitor`: passive, with record/replay;
    - `test`: active against one CUT, default level 3;
    - `list`, `explain`.
  - D8 `oec-infra`: `image`, `run`, `container`, `host`, `bus`. These replace the ROADMAP working names `inspect-*`.
  - Attachment modes: bus, **proxy** (Q-028), restart hook, peer.
  - Verdict by the REQ obligation at the declared level.
  - Exit codes 0–3 plus **4 inconclusive** (Q-030).
  - Text, JSON and JUnit reports.
  - Libraries: paho.mqtt.golang under EDL-1.0, go-containerregistry, an own minimal Engine API client and SBOM reader.
  - A simulated CUT for positive and negative fixtures.
  - Static cross builds.
  - SDK independence as a test.
- **PO:** ☐

### D-8 … D-17 – Open questions

The full options are in [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md).

| # | Question | Recommendation | Alternative(s) | PO |
|---|---|---|---|---|
| D-8 | **Q-021** Rust execution model | (a) blocking by default; `App::run()` brings its own runtime; only consumer requests are `async` (each with a `_blocking` twin); same model as the C ABI | (b) async-first | ☐ |
| D-9 | **Q-022** Config access | (a) string-keyed typed getters in P4, `#[derive(Config)]` later | (b) derive macro from the start | ☐ |
| D-10 | **Q-023** C ABI option style | (a) builder functions + string setters + JSON text | (b) option structs with size/version | ☐ |
| D-11 | **Q-024** logger demo (D4) in C on the C ABI | (a) yes; stays Messaging L1, configured by file/environment (ADR-0016). ROADMAP P6 still says "via `Config`" and would be aligned in C-045 | (b) Rust | ☐ |
| D-12 | **Q-025** micro SDK prefix | (a) `oecm_` | (b) `oec_micro_`, (c) `oec_` | ☐ |
| D-13 | **Q-026** MQTT client of the Rust SDK | (a) own sans-IO MQTT 3.1.1 codec + session in `oec-core` (spike: 307 lines, 8 tests; P4 ~1–1.5k lines incl. fuzzing) | (b) `rumqttc-v4-next` fork, (c) `paho-mqtt` (C + OpenSSL) | ☐ |
| D-14 | **Q-027** JSON reader of the micro SDK | (a) own bounded pull reader without heap, fuzzed | (b) cJSON with a static arena | ☐ |
| D-15 | **Q-028** proxy mode for a full level claim | (a) yes: without the proxy the connection tests (CONNECT, Will, TLS, certificates, recovery) are `skip` and the run is inconclusive | (b) also accept broker evidence, (c) make these REQs `manual` | ☐ |
| D-16 | **Q-029** D7 tester identity | (a) a generated tester IL, not announced on the bus; `--announce` optional | (b) always announce as a Utility | ☐ |
| D-17 | **Q-030** exit code 4 "inconclusive" | (a) yes; `--allow-skip` maps 4 → 0 | (b) skipped MUST = fail, (c) 0 with a warning | ☐ |

### D-18 – SF-101 PublicationList `Mode` and `Get` (open, register `f80eba7`)

- **Where:** `oi4-oec-documentation/findings/SPEC-FINDINGS.md`, SF-101 (MG 8.3.8).
- **Problem:**
  - `ON_REQUEST_1` says "can be requested with `Get`".
  - `APPLICATION_2`, `SOURCE_3` and `FILTER_4` each say "Additionally, the single `DataSetMessage` can be requested".
  - So the combined values `ON_REQUEST_…_11 … 15` would add nothing, and it is unclear whether `Get` needs `ON_REQUEST`.
  - The reading "only with `ON_REQUEST`" conflicts with REQ-CMP-017 (`MAM`/`Health` available via `Get` from L2).
- **Proposed interpretation:**
  - Every `Mode` except `OFF_0` allows `Get`.
  - `ON_REQUEST_1` on its own forbids any other publication.
  - In a combined value it is informative.
  - Proposed sentence for MG 8.3.8 in the register.
- **Alternative:** drop the "Additionally …" sentences and add an exception for MAM/Health.
- **Effect:**
  - The SDK defaults contain `ON_REQUEST` under both readings.
  - The D7 `Get` expectation (T7-GET-03) follows the decision.
  - Implementing it changes the guideline text, so C-045 needs a re-baseline and therefore a schema pre-release version (`2.0.1-dev`, schema content unchanged).
- **PO:** ☐

### D-19 – SF-102 `ServiceType` of a smart device (open, register `f80eba7`)

- **Where:** SF-102 (MG 7.2, MG 1.3).
- **Problem:** MG 1.3 allows directly connected smart devices. MG 7.2 describes all seven `ServiceType` values as kinds of *application*; none fits a sensor that publishes only itself.
- **Proposed interpretation:**
  - Such a device uses `OTConnector`: it makes an OT asset (itself) available.
  - Its `DeviceClass` is the product's own class, not `Oi4.OTConnector`.
  - One sentence is added to MG 7.2.
- **Alternative:** a new `ServiceType` such as `Device`. That needs a new guideline version and a schema change (2.1.0).
- **PO:** ☐

## Review items

### R-1 – Rust API sketch (`oec/sdk/rust/sketch`)

- **What:** public types and flows as compiling Rust with `todo!()` bodies.
- **DX examples:**
  - `hello.rs`: 16 lines;
  - `device_source.rs`: asset, data, `Set`;
  - `config.rs`;
  - `consumer.rs`.
- **REQ tracing:** `tests/sketch.rs` checks that every REQ id cited in the sketch exists in `oec/spec` and is not deprecated.
- **Gate:** step 5 of `oec/check.sh`.
- **Please check:** whether the hello world and the asset example read the way you want an SDK to feel.
- **PO:** ☐

### R-2 – C ABI sketch and D9 sketch

- **What:**
  - `oec/sdk/rust/sketch/c/include/oec.h` with `hello.c` and `logger.c`;
  - `oec/sdk/micro/sketch/include/oecm.h` with `hello_device.c`.
- **Gate:** the headers and the three examples pass a C17 syntax check with `-Wall -Wextra -Wpedantic -Werror` (step 5 of `oec/check.sh`).
- **Please check:** the ownership and callback rules in the header comments, and the shared concepts vs. deliberate differences between `oec_` and `oecm_` (ADR-0022).
- **PO:** ☐

### R-3 – Spikes (`oec/sdk/rust/spike/`, marked as spikes)

- **`schema-embed`:**
  - All 45 schemas of release 2.0.0 are embedded at compile time.
  - Version and baseline come from the manifest.
  - 630 strict cases give the same verdict as `schema/check.sh`.
  - Cost: +3.1 MB per binary. This is the reason for the opt-in feature `strict-validation`.
- **`mqtt-codec`:** 307 lines, 8 byte-exact tests; evidence for Q-026 (a).
- Both run offline in `oec/check.sh`. They are throw-away code; P4 rewrites them in `oec-core`.
- **PO:** ☐

### R-4 – Test catalog D7/D8 (`oec/conformance/catalog/`)

- **What:**

  | Catalog | Tests | `manual` entries | Covers |
  |---|---|---|---|
  | `d7.yaml` | 54 (19 monitor, 35 test) | 4 | 47 `d7-monitor` + 119 `d7-test` REQs |
  | `d8.yaml` | 17 | 1 | 50 `d8` REQs |

- **Gate** (`catalog_test.go`):
  - REQ existence;
  - mode ↔ verification kind;
  - level = the first level at which one of a test's REQs applies;
  - **full coverage**: P3 exit criterion 4, now permanent.
- **Independence:** `independence_test.go` checks that the module uses no SDK code (Q-009).
- **Please check (spot check):**
  - the groups T7-CON/T7-LIFE (proxy-based) and T7-GET/T7-SET;
  - the `manual` reasons:
    - REQ-CMP-011/012: rule for the tool, i.e. the default level;
    - REQ-LIFE-011…013: subscriber side, not observable.
- **PO:** ☐

### R-5 – Tool design (`oec/conformance/DESIGN.md`)

- **What:** the detailed design behind ADR-0025:
  - package layout;
  - commands and options;
  - capture format;
  - side effects of `test` (restore, `--app` required);
  - report contents;
  - configuration (`--bus-storage`);
  - D8 commands;
  - self-test with a simulated CUT;
  - distribution.
- **Please check:** whether the command names and the CLI shape suit the vendors and operators you have in mind.
- **PO:** ☐

## After your answers (C-045)

1. Amend the ADRs according to D-8 … D-17. Set ADR-0020 … 0025 to `accepted`. Add the ADR-0017 amendment note (D-6).
2. SF-101/102: on your order, change the guideline text, then re-baseline (with the schema pre-release `2.0.1-dev` and an `Unreleased` CHANGELOG entry; no schema content change expected), then the REQ remap.
3. Refine ROADMAP P4/P4b/P5/P5b from the accepted ADRs and close P3. After that, P4, P4b, P5 and P5b can start in parallel.
