# C-102 – P8 increment 5: compliance guides – `oec-check`, `oec-infra`, JSON Schemas, validation how-to

- Date: 2026-11-18
- Phase: P8 – Documentation (D6), increment 5 (ADR-0031 §7, amendments C-100 addendum 2 and C-101)
- PO input: the C-102 start prompt (nothing open in REVIEW.md; no answer, no new order to record).
- Baseline: guideline `52d8470` (branch `feature/spec-findings`, head `0539e3a` at start = register `eb821e9` + the Community pages incl. the SDK guides, no guideline chapter changed), unchanged since C-078; SF-001 … 117 resolved, next SF-118. 1411 inventory items, 444 REQs, 158 `schema`/`d7-validate` REQs with 0 gaps, no drafts. 55 D7 and 17 D8 tests `implemented`; report version 1 frozen; schema 2.0.0 frozen, work state `2.0.1-dev` @ `52d8470` – all unchanged. cbindgen 0.29.4, node 22.13, yarn 4.13 (guideline site).

## Goal

Guides for the compliance tools and the schemas as pages in `oi4-oec-documentation/docs/community` (ROADMAP P8 increment 5): D7 `oec-check` (`validate`, `monitor` live and replay, `test` with proxy, restart hook and peer, report, `list`/`explain`), D8 `oec-infra` (`image`, `host`, `bus`, `container`, `run`, prerequisites, limits, report), a JSON Schema reference and a validation how-to. Derived from the READMEs, short, details by exact README anchors, every command and output matching the code.

## What was done

1. **Sources read:** `oec/conformance/README.md` (whole), `DESIGN.md` §5 (verdicts, exit codes), the catalogs and report schemas, `oec/schema/README.md`, `CHANGELOG.md`, the `v2/` tree and `release/2.0.0.json`, `oec/spec/topic/topic-patterns.json`, `oec/dev/README.md`, the SDK example `oec/sdk/rust/oec/examples/demo.rs`; the Community pages as style template.
2. **Pages** in `oi4-oec-documentation/docs/community/compliance/` (guideline commit `691f47c` on `feature/spec-findings`, not pushed; `.yarn/install-state.gz` not committed):
   - `index.md` – Compliance tools (sidebar 6): what each tool checks, a one-minute try (offline: `list`, `monitor --replay` of a fixture), install, verdicts (outcomes, levels, exit codes 0 … 4), reports (text/JSON/JUnit, report version 1 frozen, both report schemas), `list`/`explain`, the guides.
   - `oec-check.md` (1): `validate` (the four layers, a valid and a rule-layer example, message files, `--role`, `--tolerant`, `--object`, `--format json`, exit codes), `monitor` (live against `oec/dev` with `--bus-storage`, `--record`, replay, `--app`, ≥ 125 s for the heartbeat), `test` (the Rust SDK example application on a copy of the sample storage, the probe user, the AppId, the warning that `test` changes the application, proxy/restart hook/peer as a table, a proxy CA with OpenSSL), the report, the warning against `monitor` during `oec-infra bus`.
   - `oec-infra.md` (2): `image` (`docker save`, other OCI archives, what T8-IMG-01 … 04 check), `run` (three phases, never pulls, own containers only), `container`, `bus` (the dev command, the expected verdicts on `oec/dev`, the overlap warning), `host` (the sample root, the macOS failure of REQ-PLT-003 by design, the full check in a Linux container), what a human judges (`manual`).
   - `schema.md` (3): what the files cover (links into `oec/schema/v2`), from a topic to its entry schema (`<method>-<resource>`, `topic-patterns.json`), `$id`s are names (not hosted, load all files), strict and tolerant (generated into `dist/`), what a schema cannot check, versions (2.0.0 frozen, `2.0.1-dev`, PATCH/MINOR/v3, CHANGELOG anchors), how the schemas are tested.
   - `validate.md` (4): one `Health` message checked with `oec-check validate` (valid; `HealthScore` 101 + an undefined key), with any JSON Schema validator (steps + a Python `jsonschema` script), and what the schema does not see (a `DataSetWriterName` that differs from the Source: schema-valid, rule-layer invalid; rule layer, `monitor`, `test`).
   - `migration.md` moved to sidebar position 7 (stays last); Getting started ("Where to go next", the "more guides follow" line removed), Concepts (messages, compliance levels, "Where next"), SDKs ("Check your application") and Demo services link the new pages instead of the READMEs.
3. **Commands verified offline** (`/tmp/c102`, binaries `go install`ed from the workspace, version `dev`, schemas `2.0.1-dev`, baseline `52d8470`): every `validate` example (valid, schema-invalid, `--tolerant`, rule-layer, `--role birth`/`will`, JSONL, standard input, a capture), `monitor --replay` of `lifecycle-pass.jsonl` (17 pass, 2 skip, exit 0) and `heartbeat-gap.jsonl` (the T7-MON-05 text of the page, exit 1), `oec-infra image` of `good.tar` (4 pass) and `missing-license-text.tar` (T8-IMG-03 fails REQ-CNT-014/REQ-LIC-002), `oec-infra host --root dev/generated/roots/oec-dev-app --offline` on macOS (the output of the page, REQ-PLT-003 fails), `list`/`explain`/`report-schema` of both tools (`report-schema` equals the committed schemas); the OpenSSL proxy CA was accepted by `oec-check test` (exit 3 only because no broker ran), a broken key gives exit 2. Outputs on the pages are copied from these runs (long topics shortened with `…`).
4. **Generic validator:** the Python script of `validate.md` ran in a temporary venv (`jsonschema` 4.26.0, `referencing` 0.37.0) against `oec/schema/v2`: the valid message exit 0; the invalid one reports the same two errors as `oec-check` (maximum, unevaluated `Color`), exit 1; the rule-layer example is schema-valid, exit 0.
5. **README fix (A-527):** `oec/conformance/README.md` said that the keys of `monitor --config` are the flag names; the code (`cmd/oec-check/monitor.go`) excludes `config` and `replay` by design – the README now says so (workspace commit `6cea052`).
6. **READMEs** (workspace commit `a3ee101`): `oec/docs/README.md` (page list), `oec/conformance/README.md` and `oec/schema/README.md` link the new pages (`https://oi4.github.io/oi4-oec-documentation/docs/community/compliance…`).
7. **Paperwork:** ADR-0031 amendment C-102, ROADMAP P8 increment 5 ✅, STATUS, REVIEW/REVIEW_DE (A-520 … A-528), AGENTS.md (guideline head `691f47c`).

## Results

- `docscheck: 19 pages ok, 30 README links to them ok` (was 14 / 19).
- Opt-in `oec/docs/build.sh`: about 19 s, exit 0, no warning for `docs/community`; 19 HTML pages of the section (`community.html` and 18 below `community/`) incl. `community/compliance.html` and `community/compliance/{oec-check,oec-infra,schema,validate}.html`; sidebar order in the HTML: Getting started, Concepts, Demo services, OEC Registry, OEC Micro SDK, SDKs, Compliance tools (`oec-check`, `oec-infra`, JSON Schemas, Validate a message), Migration notes.
- No change to D7, D8, the schemas, an SDK, the demos or the registry; `COVERAGE.md` unchanged; report schemas and schema 2.0.0 unchanged.

## Agent decisions (G-3.4 b)

A-520 layout (`compliance/`), A-521 content and order, A-522 commands verified offline, A-523 live commands from the README, A-524 generic validator outside the gate, A-525 schema reference hand-written, A-526 links and checks, A-527 README `--config` fix, A-528 no live run, increment 6 in C-103 – details in REVIEW.md "C-102".

## Decision for the PO

None. No defect in D7, D8, the schemas or an SDK, no guideline gap: no SF, no D-x, no R-x.

## Open / next

- C-103 – P8 increment 6, the P8 exit run: the build without broken links, a newcomer run of the demos from the site only, the live run of the SDK snippets and of the live commands of the compliance guides (`monitor`, `test`, `bus`, `container`, `run`) against `oec/dev` – needs a working Docker (G-3.10: new session).

## Docker

The Docker daemon answered HTTP 500 during the whole session; nothing was started, no container, volume, network or image was created. Increment 6 was therefore not started.

## Temporary files

Created and removed: `/tmp/c102/` (binaries, sample messages, `check.py`, the Python venv, the test proxy CA and key, `build.log`), `/tmp/gate.log` (gate output, removed after reading). The guideline site's `build/` and `.docusaurus/` stay (git-ignored, as in C-100/C-101).

## Handover (G-3.10)

**A new session is recommended** for C-103: this session's context was compacted, and the exit run needs Docker. `prompt.md` (workspace root, German, not committed) holds the start prompt for C-103 (P8 increment 6, exit run).

## Gate

`caffeinate -i oec/check.sh` on the workspace with the README commits `a3ee101` and `6cea052` and the pages of guideline commit `691f47c`: started 14:49 (load ~14), ended 15:02 (load ~7) with **"all checks passed"**, no failure and no re-run; step 10 reported `docscheck: 19 pages ok, 30 README links to them ok`. The paperwork commit changes only `project/` and `AGENTS.md`, which no gate step reads.
