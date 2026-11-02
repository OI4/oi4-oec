# C-089 – P7 increment 4: Registry Web UI (React/TypeScript/Vite, npm policy)

- Date: 2026-11-05
- Phase: P7 (OEC Registry D5), increment 4 of 6 (ROADMAP P7 cycle cut, A-381)
- Agent session: new (handover of C-088, `prompt.md`)
- Baseline: guideline `52d8470` (branch `feature/spec-findings`, head `eb821e9`), SF-001 … 117 resolved, next SF-118; workspace head `2018825`

## Intent

1. **Web UI** in `oec/registry/ui/` (ADR-0029 §9): React 19.3.1 + react-dom, TypeScript 7.0.2, Vite 7.3.x (not Vite 8, A-382); JSX through esbuild, no `@vitejs/plugin-react`. Applications and devices, MAM, latest Health and history, availability, Profile, events, observed level (informative), conformity (counters, last violations); live over `/api/stream`, read-only, nothing loaded from elsewhere (CSP `default-src 'self'`).
2. **npm policy** (ADR-0029 §10, A-384): exact versions, `package-lock.json` v3 committed, every package `resolved` from `registry.npmjs.org` with `integrity`, licenses of the allowlist of ADR-0023; `npm ci --ignore-scripts` + `npm run build` in the opt-in `ui/build.sh`; `ui/dist/` and a stamp committed.
3. **Gate without node and network:** a Go test checks the lockfile rules, the licenses and the stamp.
4. **Embedding:** `ui/dist/` by `include_bytes!` through a generated table, replacing the placeholder on `/` of the web port; `/api` unchanged; content types, hardening contract, `404` outside the table. Rust test table = `dist/`; Go test: the UI files over HTTPS with the right headers.

## Plan

1. Baseline and gate.
2. Re-check the versions, write `package.json`, resolve the lockfile, check licenses and the tree.
3. UI sources (`ui/src/`), Vite/TypeScript configuration for the CSP, first build.
4. Go `internal/uipolicy` (policy, CSP rules of `dist/`, stamp, table) with `cmd/uigen`; `ui/build.sh`.
5. Rust: generated `src/ui_assets.rs`, router and server; tests.
6. Go tests: policy (positive and violations), UI files over HTTPS.
7. `oec/registry/check.sh`, a browser smoke run, the full gate, docs.

## Work

### 1. Baseline

Guideline `52d8470` on `feature/spec-findings` (head `eb821e9`, only `findings/` after it); SF-001 … 117 resolved, next SF-118; inventory 1411 items, 444 REQs, 0 gaps; 55 D7 and 17 D8 tests `implemented`. `oec/check.sh` on the C-088 head: "all checks passed", rc 0. node v22.13.0, npm 11.0.0 (development tools only).

### 2. Versions and the npm tree (A-407, A-414)

Re-checked on the npm registry at the start: `react`/`react-dom` 19.3.1 (2026-10-14), `typescript` 7.0.2 (2026-07-08, the native compiler with a platform binary `@typescript/typescript-<os>-<cpu>`), `vite` 7.3.7 (2026-10-06, the newest of the 7 line; Vite 8 stays out, A-382), `@types/react`/`@types/react-dom` 19.3.0. Pinned exactly in `ui/package.json` (private, MIT, `engines.node >= 22.12.0`, the only script `build`).

The lockfile was resolved with `npm install --ignore-scripts --before=2026-10-26` – no package published in the last seven days enters (a supply-chain cool-down against freshly hijacked releases). It took rollup 4.65.0 instead of 4.67.4; the direct pins were not affected.

| | |
|---|---|
| Lockfile | v3, **91 entries**: 18 packages for every platform + the optional platform binaries of esbuild (26), rollup (25), TypeScript (20), `fsevents` and `@napi-rs/lzma-linux-x64-gnu` |
| Licenses | **MIT 68, Apache-2.0 21, ISC 1, BSD-3-Clause 1** – all in the allowlist of ADR-0023, no question to the PO |
| Installed (darwin/arm64) | 22 packages, 58 MB in `node_modules/` (git-ignored, never committed) |
| Shipped (non-dev) | `react` 19.3.1, `react-dom` 19.3.1, `scheduler` 0.28.0 – MIT |
| Build tools | `vite` 7.3.7, `esbuild` 0.28.2, `rollup` 4.65.0, `postcss` 8.5.31, `typescript` 7.0.2, `tinyglobby` 0.2.17, `fdir` 6.5.0, `picomatch` 4.0.7, `nanoid` 3.3.20, `picocolors` 1.1.1 (ISC), `source-map-js` 1.2.2 (BSD-3-Clause), types `@types/react`, `@types/react-dom`, `@types/estree`, `csstype` |

`@napi-rs/lzma-linux-x64-gnu` 1.5.1 is an optional dependency of rollup ≥ 4.64 (MIT, napi-rs project); it is never installed on darwin, but it stays in the lockfile like every platform binary (A-414). `engine-strict` is not set: that package declares `node ^22.20` and would break `npm ci` on linux with node 22.12 … 22.19, although node ≥ 22.12 is enough for the build.

### 3. UI (A-408, A-412)

`ui/src/` – about 1,240 lines of TypeScript/TSX and one CSS file (436 lines):

- `live.ts`: `useLive` – a reducer over `EventSource("/api/stream")` (`snapshot`, `asset`, `directory`, `event`), events bounded by the registry's `eventTrail`, the clock skew to the registry, reconnect 5 s after a closed stream.
- `App.tsx`: header (registry IL and version, counts, connection state), hash routes `#/asset/<IL>`, a 5 s tick for "… ago", footer links to the API documents.
- `AssetTable.tsx`: applications with their devices nested below, filter by name, IL, ServiceType; badges for availability, Health, observed level, conformity.
- `AssetDetail.tsx`: fetches `/api/assets/{il}` and again at most once a second while its summary changes; availability with times, MAM (`DeviceManual` as text, not a link), Health and its details, the Health history (`HealthChart.tsx`, SVG step chart), observed level with the missing requirements and the conformity with the last violations – both marked "informative" –, Profile and the answers to the `Get`s, PublicationList, SubscriptionList, devices, the asset's events.
- `EventList.tsx`: the Event trail, latest first, with pause.
- `style.css`: light and dark (`prefers-color-scheme`), system fonts, no web fonts.

Build (`vite.config.mjs`, `tsconfig.json`): esbuild `jsx: "automatic"`, `legalComments: "eof"` (the five `@license` comments of react, react-dom and scheduler stay at the end of the bundle), `assetsInlineLimit: 0`, one CSS file, no module-preload polyfill, no source maps, target ES2022; `tsc --noEmit` strict with `noUncheckedIndexedAccess`, `verbatimModuleSyntax`. `dist/`: `index.html` 605 B (module script and stylesheet as files with `crossorigin`, no inline code), `assets/index-<hash>.js` 244 kB (76 kB gzip), `assets/style-<hash>.css` 4.8 kB, `favicon.svg`. The bundle contains no `eval` and no URL except the namespace strings of React and `https://react.dev/errors/`.

### 4. Policy, stamp, table (A-409, A-410)

Go package `oec/registry/internal/uipolicy` (standard library): `CheckPackages` (the rules of ADR-0029 §10 and A-409), `CheckDist` (the CSP rules), `Stamp`, `Table`, `Check` (all of them plus: the committed `ui/build.stamp` and `src/ui_assets.rs` equal the generated ones, with a diff in the message), `Write`. `cmd/uigen` writes the stamp and the table. `ui/build.sh` (opt-in): `npm ci --ignore-scripts --no-audit --no-fund`, `npm run build`, `go run ./internal/uipolicy/cmd/uigen`, `go test ./internal/uipolicy`. `ui/.npmrc` pins the registry, `save-exact`, `ignore-scripts`; `ui/.gitattributes` `* -text`.

The rebuild is reproducible: repeated runs of `build.sh` on the same sources gave byte-identical `dist/` files (same content hashes in the names).

### 5. Embedding (A-411)

`src/ui_assets.rs` (generated) lists `UiFile { path, content_type, bytes: include_bytes!(…) }` for every file of `dist/`; `src/index.html` (the placeholder) is removed. `api::route` serves a table file on the web port before the API paths (`/` = `/index.html`); `http.rs` answers it with the common headers. No change to `/api`, the OpenAPI document or the guards.

### 6. Tests (A-413)

- Rust (32 unit tests): `ui_table_is_dist`; `routes` (UI files only on the web port; `/x.js`, `/assets/`, `/index.htm`, `//index.html`, `/../index.html` → none); `served_over_loopback` (every table file `200` with its content type on the web port, `404` on the API port, `/nope.js` `404`).
- Go `internal/uipolicy`: `TestUI` (the committed UI passes; 91 packages, shipped exactly react, react-dom, scheduler), `TestViolations` – 31 violations on a copy of the UI, each caught with its message: caret version, extra dependency, install script, `overrides`, not private, lockfile v2, root entry differs, direct dependency not locked at its pin, another registry, a git dependency, missing and sha1 integrity, MPL-2.0, `(MIT OR GPL-3.0-only)`, no license, a build tool shipped, a link, a package outside `node_modules/`, a changed source, a new source, a changed `dist/` file, no stamp, no `dist/`, inline script, inline style, inline handler, an external script, a `data:` URL in CSS, a source map, no `@license` comments, a stale table; `TestWrite`.
- Go `http_test.go` `checkUI` (in `TestHTTPSelfSigned`, HTTPS with the pinned self-signed certificate): every file of `ui/dist/` on the web port with identical bytes, the content type of the table and the headers of the contract (`CheckHeaders`), `/` = `index.html`, `404` for `/assets/`, `/assets/nope.js`, `/index.htm`, `/dist/index.html`, `/favicon.svg/x` and for every UI path on the API port; a request with the same-origin `Origin` (as a `crossorigin` module request sends it) passes, a foreign `Origin` is `403`.
- Manual smoke run (not in the gate, no file committed): headless Chrome against the registry on the Go test broker with an announced participant (a temporary Go test, deleted afterwards; screenshots only locally): the list showed the participant with availability, Health, observed level and conformity badges; the detail showed MAM, Health, the history chart, the observed level with the missing requirements and the violations (marked informative); the connection badge "Live" and the Event trail. `--virtual-time-budget` never ends with an open SSE stream – `--timeout` works. One cosmetic defect found and fixed: an empty `DeviceManual` showed as blank instead of "–".

### 7. Gate

See "Checks (final)".

## Agent decisions (G-3.4 b, for information: REVIEW A-407 … A-415)

A-407 versions and the seven-day cool-down of the lockfile; A-408 build configuration for the CSP (esbuild JSX, nothing inline, `@license` comments kept, no `engine-strict`); A-409 the policy rules in detail (canonical tarball URL, only the script `build`, no overrides/optional/peer/bundled, shipped set); A-410 the stamp also covers `dist/`, the CSP rules of `dist/`, honour-system limit; A-411 embedding by a generated table on the web port only, `no-store` kept; A-412 UI design (hash routes, throttled detail fetch, `DeviceManual` as text, informative markings, no write action); A-413 tests; A-414 the optional `@napi-rs/lzma` and the platform binaries stay in the lockfile; A-415 `.gitignore` re-includes `oec/registry/ui/dist/`.

## Test data for review (G-3.9)

- None. No D7/D8 fixture, scenario or verdict was added or changed; the policy test data are synthetic copies made at run time.

## Changes to reviewed items

- None.

## Spec findings

- None. The UI only shows what the API of C-088 provides.

## Dependencies

- npm (ADR-0029 §10, `oec/registry/ui` only): see §2 – direct `react` 19.3.1, `react-dom` 19.3.1 (shipped), `typescript` 7.0.2, `vite` 7.3.7, `@types/react` 19.3.0, `@types/react-dom` 19.3.0 (dev); lockfile 91 entries, MIT 68 / Apache-2.0 21 / ISC 1 / BSD-3-Clause 1; shipped react, react-dom, scheduler (MIT); installed 22 packages, 58 MB.
- Rust: none new (ADR-0023 unchanged). Go: standard library only. C: none.

## D7, SDKs, demos

- Unchanged: D7, D8, Rust SDK, C ABI, Go SDK, demos (pins 46/49/22), HTTP API and OpenAPI (only `/` and the UI files on the web port changed). Report schemas frozen, `COVERAGE.md` unchanged. A-113 untouched.

## Docker objects (own only)

- No Docker was used in C-089 (no image, container, volume or network created). Before = after: none.

## Commits

- `87ca48d` Code: `.gitignore`, `oec/registry/ui/` (sources, `package.json`, `package-lock.json`, configuration, `build.sh`, `build.stamp`, `dist/`, README), `internal/uipolicy/`, `src/ui_assets.rs`, `src/api.rs`, `src/http.rs`, `src/main.rs`, `src/index.html` removed, `http_test.go`, `check.sh`, `README.md`.
- This commit: ADR-0029 amendment C-089, this log, REVIEW, REVIEW_DE, STATUS, ROADMAP, `oec/README.md`, the step-8 comment of `oec/check.sh`.

## Checks (final)

- `oec/check.sh` → "all checks passed", rc 0, on the final code (load ~9 at the start, ~4 at the end; no flake). Step 8: `registry/rust` fmt, clippy, 32 tests, deny, build; `registry/go` `ok github.com/OI4/oec/registry 49.1s` (TestD8Registry T8-BUS-03 pass, TestD7Level3 49 pins/0 fail, TestReferenceDesignationPersists, the TestHTTP* incl. `checkUI`), `internal/regtest` ok, `internal/uipolicy` ok (6.2 s). Demo pins unchanged (Logger L2 46, Weather L3 49, Config L1 22).
- Before: `oec/registry/check.sh` alone green; `go test ./internal/uipolicy` green after each rebuild.
- After the gate run only documentation and comments changed (ADR amendment, log, REVIEW, REVIEW_DE, STATUS, ROADMAP, `oec/README.md`, the step-8 comment in `oec/check.sh`; `bash -n` ok) – nothing the gate checks.

## For the PO

- Nothing to review (no R-x), nothing to decide (no D-x), no SF.
- For information: A-407 … A-415 – especially A-407 (lockfile resolved with a seven-day cool-down), A-410 (what the stamp can and cannot prove) and A-411 (the UI files are served with `no-store` like the API).

## Handover (G-3.10)

**A new session is recommended** for C-090: this session's context was compacted. `prompt.md` (workspace root, German, not committed) holds the start prompt of C-090.

## Next

C-090 – P7 increment 5: image of the registry (ADR-0029 §12, as the demos: linux/amd64 + arm64, non-root, SBOM and license texts incl. react, react-dom, scheduler of the bundle), compose profile `registry` in `oec/dev`, D8 `image`/`run` at `--level 3` (opt-in), D8 `bus` T8-BUS-03 against `oec/dev` (opt-in).
