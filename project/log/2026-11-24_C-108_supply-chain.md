# C-108 – P9 increment 5: supply chain (SBOM, license reports, vulnerability checks, `SECURITY.md`)

- Date: 2026-11-24
- Phase: P9 – Hardening & release, increment 5 (ADR-0032 §10)
- Baseline: guideline `52d8470` (branch `feature/spec-findings`, head `989eff5` before the cycle, unchanged chapters); SF-001 … 117 resolved, next SF-118
- Workspace before the cycle: `efbc825` (local; `origin/main` = `42f6784`; the CI is paused, C-105 D-2 (b))

## Goal

Increment 5 as handed over: an SBOM and a license report for every deliverable (Rust SDK and C ABI `oec-ffi`, Go SDK, micro SDK, the D7/D8 binaries, the three demos, the registry with its Web UI, the schema), with the existing tools (`oec-sbom`, `cargo deny`), generated in a workflow (test first in `oec/ci/pipelines`, logic in scripts under `oec/ci/`, at most a workflow artifact); the vulnerability checks `govulncheck` (Go modules), `cargo deny check advisories` (Rust) and `npm audit` (registry Web UI) with every finding handled, network opt-in only (not in the gate); `SECURITY.md` and a draft of the vulnerability handling (reporting channel, time frames, support period as a proposal) as CRA input – contacts and commitments are the PO's (Q-011): placeholders and D-x.

## Plan

1. **Baseline and first scans (network, opt-in):** `govulncheck` on every Go module, `cargo deny check advisories` on every `Cargo.lock`, `npm audit` of `oec/registry/ui`; handle each finding (update within the dependency rules or a justified exception).
2. **`oec-sbom` for deliverables without an image** (ADR-0028 amendment C-108, additive, tests first): `-base`/`-debian` optional (only images have a base), the Go modules of a binary or library from `go list -deps` without building it (`-golist`, the license map of C-107 for third-party modules, fail closed), the Go standard library of a binary (`-gostd`), a license report in Markdown (`-report`). The image SBOMs stay unchanged.
3. **`oec/ci/sbom.sh <dir>`:** one SPDX 2.3 SBOM, the license texts and the license report per deliverable; **`oec/ci/audit.sh <dir>`:** the three vulnerability checks, all run, the job fails on any finding; `govulncheck` as a tool pinned in `versions.sh` (`go run …@version`, no module dependency – like actionlint).
4. **Workflow `supply-chain.yml`** (test first, `TestSupplyChain`): job `sbom` (artifact with the SBOMs and reports, 14 days, no release) and job `audit` (artifact with the outputs).
5. **`SECURITY.md`** (`oec/SECURITY.md`, travels with the release repository `OI4/oec`; a pointer at the workspace root) and the draft `project/research/vulnerability-handling.md` (intake, triage, time frames, coordinated disclosure, CRA reporting duties, support period – proposals with placeholders) → D-x for the PO.
6. **Local verification:** `lint.sh`, `sbom.sh` and `audit.sh` on the Mac, both in a fresh Ubuntu 24.04 container (image of `versions.sh`), `linux.sh`, the gate. The hosted runs stay open for C-111 (C-105 D-2 (b)).
7. **Docs:** ADR-0032/0028 amendments C-108, `oec/ci/README.md`, `oec/README.md`, Community pages where a tool changes, ROADMAP, STATUS, REVIEW/REVIEW_DE (A-595 ff., D-1 ff.).

## Result

Increment 5 is built and verified locally: the generated workflow `supply-chain.yml` writes an SPDX 2.3 SBOM, the license texts and a license report for each of the eleven deliverables (job `sbom`, artifact `oec-sbom`) and checks every dependency against the advisory databases (job `audit`: `govulncheck` on 12 Go modules, `cargo deny check advisories` on 4 lock files, `npm audit` of the registry Web UI; artifact `oec-audit`). The first scan found Go vulnerabilities in uncalled code only; all were fixed by updates, no exception was needed; Rust and npm were clean. `oec/SECURITY.md` and the draft of the vulnerability handling are written with placeholders for everything that is a commitment to the outside → **D-1 … D-3** for the PO (Q-011). In a fresh Ubuntu 24.04 (arm64, image of `versions.sh`) both scripts give the same result as on the Mac, a second SBOM run is byte-identical. The hosted runs follow in C-111 (C-105 D-2 (b)).

## What was done

### 1. Vulnerability baseline (A-597)

First scan with network before any change (Mac): `govulncheck` (v1.9.0) reported vulnerabilities in uncalled code only – `golang.org/x/net` v0.61.0 in `oec/conformance` (GO-2026-6866, GO-2026-6860), `golang.org/x/text` v0.14.0 in `oec/schema/tools` (GO-2026-6629, GO-2026-5970, via the JSON Schema validator of the schema tests) and the standard library of go1.27.2 (GO-2026-6809, `crypto/x509`). Fixed by updates (commit `2a1e5b0`): `x/net` v0.62.0 (key of `oec/conformance/images/go-licenses.json` updated, BSD-3-Clause unchanged), `x/text` v0.43.0, `toolchain go1.27.3` in `dev`, `docs/tools`, `schema/tools`, `spec/tools` (the others had it already) and later in `schema` and `spec` (section 5). `cargo deny check advisories` on `sdk/rust`, `registry`, `demos/config`, `schema/tools/rust-check`: ok. `npm audit` of `registry/ui`: 0 vulnerabilities.

A pitfall of the tool: `govulncheck` built with an older Go cannot load the standard library of a newer one ("go1.26.10 … cannot load go1.27"); `audit.sh` therefore runs it per module with `GOTOOLCHAIN=<the module's go version>`.

### 2. `oec-sbom` for deliverables without an image (ADR-0028 amendment C-108, A-595)

Tests first (`TestGoList`, `TestGoStd`, `TestReport`, `TestDeliverable`, `main_test.go`), then `GoList` and `GoStd` in `gobuild.go`, `Result.Report` in `sbom.go`, the flags `-golist`, `-gostd`, `-report` in `cmd/oec-sbom`; `-base`/`-debian` optional but only together. The image path is unchanged (the golden SBOM of C-084 still matches).

### 3. `sbom.sh`, `audit.sh`, `supply-chain.yml` (ADR-0032 amendment C-108, A-596, A-598, A-599)

- `oec/ci/sbom.sh <dir>`: builds `oec-sbom` once; `SOURCE_DATE_EPOCH` = commit time; `cargo tree --locked --offline` (normal edges) over the eight SDK targets after `fetch.sh`; `go list -deps` over six platforms (`CGO_ENABLED=0` for D7/D8, `1` for the Go SDK and the weather service); versions from the manifests. Components on the Mac and in the container: `oec-rust-sdk` 74, `oec-ffi` 75, `oec-go-sdk` 76, `oec-micro-sdk` 1, `oec-check` 11, `oec-infra` 11, `oec-logger` 76, `oec-weather` 78, `oec-config` 75, `oec-registry` 133, `oec-schema` (2.0.1-dev) 1.
- `oec/ci/audit.sh <dir>`: every check runs, one output file each, failure at the end; ~30 s on the Mac with warm caches.
- `versions.sh`: `govulncheck_version=1.9.0`.
- `supply-chain.yml` (jobs `sbom` and `audit`), test first `TestSupplyChain` (steps, artifacts, no `--frozen` for the audit, the upload last) and `TestVersions` (govulncheck pin); `cigen: 6 workflows current`.

### 4. `SECURITY.md` and the vulnerability-handling draft (A-600)

`oec/SECURITY.md` (draft; supported versions, private reporting, what happens next with the proposed time frames, scope table, SBOMs and dependency checks; `[PO: …]` placeholders), `SECURITY.md` at the workspace root (pointer), `project/research/vulnerability-handling.md` (CRA basis, Annex I Part II map, intake, triage and time frames, Art. 14 path, dependencies, support period, D-1 … D-3). Nothing published; no contact address invented.

### 5. Found and fixed (A-601)

- `sbom.sh`: in `cargo_tree`, `set --` overwrote `$2`, so both cargo trees landed in the same file – found on the Mac before the first commit.
- shellcheck 0.9.0 in the container (`shellcheck -x` from the repository root): SC1091 – `# shellcheck source=versions.sh` resolves against the cwd → `source-path=SCRIPTDIR`; SC2016 on the two intended `sh -c '…"$1"…'` lines → annotated (`3e633a2`). Clean afterwards (shellcheck 0.9.0 image, removed again).
- `oec/schema` and `oec/spec` had no `toolchain` line: in the container `go env GOVERSION` gave go1.26.0 and govulncheck listed 47 uncalled module-level standard library findings (verdict ok). Both now name `toolchain go1.27.3` (`e1dfb58`; consumers ignore a dependency's toolchain line; the schema release freeze covers the JSON files only); re-checked: 0 findings.

### 6. Local verification

- `oec/ci/lint.sh` (actionlint 1.7.12): clean; `cd oec/ci && go test ./...`: ok.
- Mac (arm64): `sbom.sh /tmp/c108-sbom` and `audit.sh /tmp/c108-audit` – all checks passed.
- Container trial (own script `/tmp/c108-ct/run.sh`, container `oec-c108-trial`, image `ubuntu_image` of `versions.sh`, `--init --cpus 4`, non-root user `ci`, committed `HEAD` `19d2b9c` cloned from a read-only mount; `packages.sh --go` plus apt `nodejs npm shellcheck`): `setup-tools.sh --toolchain`, `fetch.sh`, `sbom.sh` twice → **byte-identical**, same counts as on the Mac; `setup-tools.sh`, `audit.sh` → **all checks passed** (Ubuntu's npm 9.2.0); shellcheck → the findings of section 5.
- `linux.sh` and the gate: below.

### 7. Hosted runners – **open for C-111**

The two jobs on `ubuntu-24.04` (amd64; the local checks ran on arm64), the runner's preinstalled npm, the time of `setup-tools.sh` (cargo-deny build) without cache, the artifact sizes.

### 8. Documentation

ADR-0032 and ADR-0028 amendments C-108; `oec/ci/README.md` (layout, versions, workflow row, section "Supply chain", local commands), `oec/README.md` (`ci/` row, security link), `oec/demos/README.md` (the new `oec-sbom` flags); Community page `index.md` (a paragraph on SBOMs, license reports, dependency checks and `oec/SECURITY.md`) – guideline commit `064aa39` on `feature/spec-findings` (docscheck 19 pages ok, 30 README links to them ok). `oec-sbom` and the CI scripts have no Community page (internal tools). ROADMAP P9 (increment 5, status paragraph), STATUS, REVIEW, REVIEW_DE.

## Commits

Workspace (local, not pushed): `9bfbdb2` (plan), `2a1e5b0` (Go vulnerability fixes), `0674d4f` (`oec-sbom` deliverable mode, `sbom.sh`), `0c206ae` (`audit.sh`, govulncheck pin), `19d2b9c` (`supply-chain.yml`), `3e633a2` (shellcheck directives), `7c0a0eb` (ADR amendments, READMEs), `ae417f3` (`SECURITY.md`, draft), `e1dfb58` (`schema`/`spec` toolchain), then the documentation and this log. Guideline `feature/spec-findings`: `064aa39` (Community index). Not pushed (C-105 D-2 (b)).

## Agent decisions (G-3.4 b)

A-595 … A-601 in `REVIEW.md`: the SBOM tool for deliverables (A-595), SBOM format, scope and placement (A-596), the vulnerability baseline and fixes (A-597), the audit policy and govulncheck as a tool (A-598), `supply-chain.yml` without schedule (A-599), `SECURITY.md` and the draft (A-600), findings of the local verification (A-601).

## Decisions for the PO (G-3.4 a)

**D-1** reporting channel and contact, **D-2** time frames and support period, **D-3** weekly scheduled checks and Dependabot after the first release (REVIEW "Open for you"; Q-011). Still open from C-104 (A-552): is `OEC_GUIDELINE_TOKEN` read-only? Arduino-ESP32 4.0.0 final: re-check in C-111.

## Docker

Own objects only, all removed (no `prune`): container `oec-c108-trial` (`--rm`), its image `ubuntu:24.04@sha256:fe8156…` of `versions.sh` (pulled, absent before, removed by the trial script), `koalaman/shellcheck:v0.9.0` (pulled for the re-check, removed), the container and image of `linux.sh` (removed by the script). Foreign objects untouched.

## Temporary files

Created and deleted: `/tmp/c108-tree.txt`, `/tmp/c108-err.txt` (cargo tree trials), `/tmp/c108-sbom/` and `/tmp/c108-audit/` (Mac runs), `/tmp/c108-ct/` (trial script, log, the SBOM copy of the container run), `/tmp/c108-linux.log`, `/tmp/gate.log`.

## linux.sh

`caffeinate -dims oec/ci/linux.sh` (async, without `OEC_CI_CA_CERTS`) cloned the committed head `ae417f3` (guideline `064aa39`) into a fresh Ubuntu container (`ubuntu:24.04` of `versions.sh`): all 11 steps green, `cigen: 6 workflows current (conformance.yml, gate.yml, images.yml, micro.yml, sdk.yml, supply-chain.yml)`, `docscheck: 19 pages ok, 30 README links to them ok`, "all checks passed", `linux.sh exit 0`, ~23 min (load up to 11). `e1dfb58` (two `toolchain` lines, checked by the gate below) came after the clone. Container and image removed by the script.

## Gate

`caffeinate -dims oec/check.sh` on the Mac at `e1dfb58` plus the uncommitted documentation (load 4.4 at the start): all 11 steps green on the first run, no repeats, `docscheck: 19 pages ok, 30 README links to them ok`, `cigen: 6 workflows current (conformance.yml, gate.yml, images.yml, micro.yml, sdk.yml, supply-chain.yml)`, "all checks passed", ~17 min.

## Next

C-109 – P9 increment 6: versioning and `release.yml` (ADR-0032 §9) – every deliverable from one commit/tag with `SHA256SUMS`, the SBOMs of `sbom.sh` and release notes as workflow artifacts; nothing published (channels: Q-011, C-104 D-2). The CI stays paused until the exit run C-111 (C-105 D-2 (b)).

Handover (G-3.10): **new agent session** – the context of this session was compacted; C-109 builds on `sbom.sh` but works in other files (`release.yml`, versioning). `prompt.md` written for C-109 (not committed).

## Addendum 1 (2026-11-24) – PO answers D-1 … D-3

The PO answered "D-1, D-2 und D-3 jeweils (a)". Recorded first (REVIEW, REVIEW_DE, this addendum), then applied:

- **D-1 (a):** reporting channel = GitHub private vulnerability reporting on the release repository `OI4/oec` plus a security mailbox of the OI4 Alliance; at least two people triage. The mailbox address and the triage people/role were not given – they stay placeholders `[PO: …]` in `oec/SECURITY.md` and the draft and are listed as missing input in REVIEW "Open for you" (data only, no decision).
- **D-2 (a):** the time frames (acknowledge 3 working days, assess 10; fix critical 14 days, high 30, medium 90, low with the next release; disclosure with the fix, at most 90 days after the report; CRA Art. 14 path) and the support period (five years from the first release of a release line, the newest minor version gets the fixes; the demos without support period) replace the placeholders in `oec/SECURITY.md`. It stays a draft: unpublished until the first release and the CRA review (Q-011).
- **D-3 (a):** from the first release `supply-chain.yml` also runs weekly (a `schedule` trigger in the workflow model `oec/ci/gha`, test first) and Dependabot alerts are enabled on the release repository (a repository setting). Not built now: the first release is not decided (Q-011) and the CI is paused (C-105 D-2 (b)); recorded as a P9 item in the ROADMAP and in ADR-0032 (amendment C-108 item 7).

Applied in `oec/SECURITY.md` (channel, time frames, CRA Art. 14 note, support period, demos without support, weekly checks and Dependabot from the first release; one placeholder left: the mailbox address), `project/research/vulnerability-handling.md` (status, §§2–7), ADR-0032 amendment C-108 item 7, ROADMAP P9, STATUS, `oec/README.md`, the comments of `SupplyChain`/`TestSupplyChain` (no behaviour change, workflows unchanged) and the Community page `index.md` (guideline commit `40204d7` on `feature/spec-findings`). Gate green (load 14 at the start, ~17 min, no repeats; `cigen: 6 workflows current`, `docscheck: 19 pages ok, 30 README links to them ok`). Not pushed (C-105 D-2 (b)).

## Addendum 2 (2026-11-24) – contact address, question on the token

The PO answered: "Als Security E-Mail nimm erstmal die info@openindustry4.com. Betitel es aber nicht als security mailbox, sondern ein mail the issue to…" and asked which permissions `OEC_GUIDELINE_TOKEN` needs and on which repository. Recorded first (REVIEW, REVIEW_DE, this addendum), then applied:

- `oec/SECURITY.md`: "if you have no GitHub account, mail the issue to info@openindustry4.com" – no "security mailbox" wording, no placeholder left; the draft `research/vulnerability-handling.md` §3 follows (the triage people stay a placeholder there, internal).
- Token: the secret is used only as the `token` of the guideline checkout in `gate.yml` (`OI4/oi4-oec-documentation`, `feature/spec-findings`, full history, `persist-credentials: false`). It needs a fine-grained token with resource owner `OI4`, only the repository `OI4/oi4-oec-documentation`, repository permission Contents: Read-only (Metadata: Read-only implied), stored as Actions secret in `OI4/oi4-oec`. The value in the git-ignored `.env` (only its prefix was read, never printed) is a classic token (`ghp_`): it cannot be restricted to one repository or to read access on private repositories, which explains `push: true` of A-552. Recommended to the PO: replace it before the exit run C-111 and revoke the classic one. `oec/ci/README.md` "Activation" now spells the permissions out.

Applied in `oec/SECURITY.md` (no placeholder left), `project/research/vulnerability-handling.md` (§§2, 3, 7), `oec/ci/README.md` "Activation" (token permissions), `oec/README.md`, ADR-0032 amendment C-108 item 7, ROADMAP P9, STATUS. No Community page names the token or a mailbox (unchanged). Gate green (load 4.7, no repeats; `cigen: 6 workflows current`, `docscheck: 19 pages ok, 30 README links to them ok`). Not pushed (C-105 D-2 (b)).

## Addendum 3 (2026-11-24) – new token

The PO: "Das token steht in .env unter dem Key OEC_GUIDELINE_TOKEN. Bitte versuche es damit. Lies aber den Token nicht in den Context." Recorded first; the token value is only passed between processes (never printed, logged or committed; `.env` is git-ignored).

Probe of the new token (value never printed): fine-grained (`github_pat_` prefix checked by pattern), expires 2027-11-06; `OI4/oi4-oec` → 404 (no access, as intended); `OI4/oi4-oec-documentation` metadata → 200 (so the token is approved for the organisation); but branch, contents and `git-upload-pack` → 403 "Resource not accessible by personal access token" with `x-accepted-github-permissions: contents=read` – the token lacks **Contents: Read-only**. `git-receive-pack` → 403 (no write, as intended). The secret was therefore **not** set (the guideline checkout of `gate.yml` would fail); the old secret stays until the PO adds the permission (a fine-grained token's permissions can be edited without a new value), then the probe is repeated and the secret set with `gh secret set OEC_GUIDELINE_TOKEN -R OI4/oi4-oec` from `.env`.

Correction of addendum 2 and A-552: the `permissions` object of `GET /repos/…` (`push: true`) describes the role of the token's owner, not the token's rights; it does not show whether a token is read-only. The probe above (receive-pack, contents) is the reliable test. The recommendation (fine-grained, Contents read-only) stands.
