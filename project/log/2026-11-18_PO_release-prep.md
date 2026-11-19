# 2026-11-18 – Release preparation OEC 0.1.0, pre-release of the guideline site (2026-11-19)

PO request (after pushing the guideline cosmetics, workspace `b9bbdc3`,
guideline head `aac633b`): "What do we need for the release? Can you take
it over?" The agent prepares everything that needs no PO right; push, tag,
GitHub settings and the go stay with the PO (ADR-0009, ADR-0034 §5,
C-110 D-2). Nothing pushed, no tag, no remote resource created.

## Findings on the pushed commit `b9bbdc3`

CI: `gate`, `images`, `micro`, `conformance` green; `supply-chain` and
`sdk` red. Both are release blockers (the tag build runs the same jobs).

- **supply-chain:** `npm audit` of the Registry Web UI reports
  GHSA-fc5q-39xc-vjpf (picomatch ReDoS, high, `<= 4.0.7`, no patched
  version; 4.0.7 is the newest). picomatch comes in through vite
  (fdir/tinyglobby), dev dependencies only; the registry ships the built
  `ui/dist`, embedded with `include_bytes!`, which contains no picomatch.
- **sdk, job `Go SDK (windows-2025)`:** `invalid flag in #cgo LDFLAGS:
  -l:libpthread.a`. Go 1.27.4 tightened the cgo allow list
  (`-l([^@\-:=/\\][^:=/\\]*)`); 1.27.3 accepted the flag. Every user with
  the current Go would hit it on Windows.
- **Local gate (agent's machine):** the Homebrew `rustc` in `PATH` hides the
  rustup toolchain with `llvm-tools` (`oec-buildlib: llvm-objcopy not
  found`); environment only, the gate passes with `~/.cargo/bin` first.

## Decisions (agent, G-3.4 b)

- **picomatch accepted (ADR-0023 §2):** an advisory without a fix in a
  build-only dependency may be accepted with a reason in an ignore list, as
  `deny.toml` does for Rust. New `oec/ci/npm-audit.mjs` judges
  `npm audit --json` against `<ui>/audit-ignore.txt` (one line per advisory:
  id + reason); an unlisted advisory still fails, a listed one that npm no
  longer reports gives a warning so the entry gets removed. Remove the entry
  once picomatch ships a fix (Dependabot after the release).
- **Static winpthreads with `-Wl,-Bstatic -lpthread -Wl,-Bdynamic`:** both
  flags are on the allow list (checked in Go 1.24.2 and 1.27.4); `-Wl,-Bstatic`
  forces the external linker, which cgo with the Rust archive uses anyway.
  `ffi-lib.sh` keeps `-l:libpthread.a` in `native-static-libs.txt`: that
  file is for linking C programs with gcc, where the flag is valid.
- **Release commit on a local branch:** schema 2.1.0 and the CHANGELOG
  date are prepared on `release/0.1.0` (both repositories), not on `main`,
  so `main` stays releasable as `-dev` until the work group's okay.

## Done

Workspace `main`:

- `1fb50f8` npm ignore list (`oec/ci/npm-audit.mjs`, test
  `TestNpmAudit` 7 cases, `audit.sh`, `oec/registry/ui/audit-ignore.txt`,
  `oec/ci/README.md`, `oec/SECURITY.md`); `audit.sh` all 17 checks pass.
- `8422829` Windows link flags (`oec/sdk/go/link.go`, `link_test.go` pins
  the flags and checks every `-l` against the 1.27.4 pattern, README).
  Verified with a MinGW cross build in Docker (Ubuntu 24.04,
  `gcc-mingw-w64-x86-64-posix`, `x86_64-pc-windows-gnu`, go1.27.4): `go vet`,
  test binary and examples link, the binary imports no
  `libwinpthread-1.dll`; the old flags reproduce the CI error.
- `9fd64eb` `oec/registry/ui/build.stamp` regenerated (`ui/build.sh`):
  `audit-ignore.txt` is an input of `ui/`; `dist/` unchanged.

Workspace branch `release/0.1.0` (on `main`, one commit): schema
`2.1.0` (`schema.go`, `CHANGELOG.md` heading "2.1.0", frozen
`release/2.1.0.json`, README state line), `oec/CHANGELOG.md`
"0.1.0 – 2026-11-18" (placeholder date), schema-embed spike pins 2.1.0 /
`c7de341`. Tag build simulated (`GITHUB_REF_TYPE=tag
GITHUB_REF_NAME=oec-v0.1.0 oec/ci/release.sh … source schema`): ok. Full
gate on the branch: all checks passed.

Guideline branch `release/0.1.0` (on `feature/spec-findings` `aac633b`,
`6969c03`): Community schema page names 2.1.0 the current release, sample
outputs "schemas 2.1.0", changelog anchor `#210`. It must be merged together
with the workspace branch (docscheck checks the anchors in the workspace).

## Checklist for the PO (in this order)

1. Push workspace `main` (`1fb50f8`, `8422829`, `9fd64eb`, this log);
   check that `supply-chain` and `sdk` turn green.
2. Okay of every work-group member (C-110 D-2).
3. Q-011: decide the visibility of `OI4/oec`, then create the repository.
   Note: `OI4/oi4-oec` itself is already public.
4. `OI4/oi4-oec` → Settings → Environments: `release` with required
   reviewers and the deployment tag rule `oec-v*` (before the first tag).
5. Secret `OEC_RELEASE_TOKEN` (classic PAT, `repo` + `write:packages`,
   ideally of a machine account) and variable `OEC_RELEASE_USER`.
6. Set the real date in `oec/CHANGELOG.md` on `release/0.1.0`
   (`git commit --amend`), merge it into `main`, push, tag `oec-v0.1.0`
   on that commit and push the tag; approve the job `publish`.
7. After the release: GHCR package visibility of `ghcr.io/oi4/oec-*`
   (private by default), the weekly `supply-chain.yml` schedule and
   Dependabot (C-108 D-3 (a)); remove the picomatch entry once fixed.

Guideline site (`oi4-oec-documentation`, deploys on push to
`development`):

1. ~~Preview first: repository variable `OEC_DOCS_PREVIEW`~~ – done
   2026-11-19, see "Pre-release" below.
2. Merge `feature/spec-findings` (with `release/0.1.0` once the software is
   released) into `development`. `development` is protected (pull request
   with one approval, linear history): no merge commit, no squash, and
   GitHub's "rebase and merge" rewrites the commits; keep the baseline
   commit `c7de341` with a fast-forward push after the approval.
3. Final release: delete the variable and redeploy.

## Pre-release of the guideline site (2026-11-19, PO order)

PO: "Also Pre-Release mit der vorgeschalteten Seite" – publish now as a
pre-release behind the teaser page (approval of G-3.8 for this
publication). The software has no pre-release channel (`release.sh` accepts
`MAJOR.MINOR.PATCH` only, `OI4/oec` does not exist yet, Q-011), so the
pre-release is the guideline site only; the software release keeps the
checklist above.

- Built from the pushed `feature/spec-findings` `aac633b`, not from
  `release/0.1.0`: its Community links name the anchor `#210` of the schema
  changelog, which exists on GitHub only after the software release.
- `OEC_DOCS_PREVIEW=v2-preview-d4c86m8cob scripts/build-site.sh` in a
  worktree; deployed by hand like `yarn deploy --skip-build`: commit
  `083f781` "Deploy website - based on aac633b (unlisted preview, teaser at
  the root)" on `gh-pages` (replaces `cbc8d95` of 2025-04-30, the site of
  `development` `2b412d5`). `development` untouched (protected; the workflow
  `deploy.yml` never ran, its secret `PAT_GITHUB_ACTION` dates from
  2025-04-30).
- Repository variable `OEC_DOCS_PREVIEW=v2-preview-d4c86m8cob` set, so a
  later merge to `development` deploys the preview as well, not the full
  site at the root.
- Verified online after the Pages build: root = teaser ("Open Edge
  Computing 2.0 – Big news to come"), no reference to the path;
  `/oi4-oec-documentation/v2-preview-d4c86m8cob/` with Messaging Guideline,
  Infrastructure Guideline, Community and 25 random pages: 200, JS bundle
  200, pages `noindex, nofollow`, no sitemap.
- Side effect: the pages of the old site at the root answer 404 now; so do
  the site links of the `oec/` READMEs (`docscheck.SiteURL`) on the public
  `OI4/oi4-oec` until the variable is removed for the final release.
- Preview URL (share by hand only):
  `https://oi4.github.io/oi4-oec-documentation/v2-preview-d4c86m8cob/`.
  Withdraw: push the old tree `cbc8d95` back to `gh-pages`.

Not changed: Arduino-ESP32 4.0.0 is still not released (RC1 only), the
re-pin in `oec/ci/versions.sh` stays open (P9 D-2 (a)).
