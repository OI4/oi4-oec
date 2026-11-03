# C-098 – P8 start: plan, docs check in the gate, Getting started

- Date: 2026-11-14
- Phase: P8 – Documentation (D6), increment 1 (ADR-0031 §7)
- PO input: C-097 addendum 1 (P4b closed, next P8; no publication to the Arduino Library Manager yet). During C-098 a license question to the PO (ask_user) was not answered ("user not available") – it becomes **D-1** in REVIEW; C-098 does only what needs no new npm dependency.
- Baseline: guideline `52d8470` (unchanged since C-078), SF-001 … 117 resolved, next SF-118.

## Goal

Start P8: plan and cycle cut, the ADR of the documentation site, a check of the existing pages in the gate (they had none), the Getting started page, and an answer to the question whether the Docusaurus tree fits the dependency policy.

## What was done

1. **Docusaurus survey (temporary, `/tmp/oec-c098-survey`, removed):**
   - Version: Docusaurus **3.10.2** is the newest stable release (2026-07-10); 4.0 exists as canary only. The guideline site uses `^3.10.2` with React 18; this site would use React 19.3.1 (as the registry Web UI).
   - `package.json` with `@docusaurus/core` and `@docusaurus/preset-classic` 3.10.2, `@mdx-js/react` 3.1.1, `react`/`react-dom` 19.3.1, `clsx` 2.1.1, `prism-react-renderer` 2.4.1; lockfile by `npm install --package-lock-only --ignore-scripts --before=2026-10-27` (7-day cool-down, ADR-0029 amendment C-089): **1275 packages** (lockfile v3, all non-dev).
   - Licenses: MIT 1077, MIT-0 65, ISC 43, Apache-2.0 35, BSD-2-Clause 21, BSD-3-Clause 19; OR expressions with an allowlisted choice: `type-fest` ×2 (`MIT OR CC0-1.0`), `opener`, `path-is-inside` (`WTFPL OR MIT`), `rc` (`BSD-2-Clause OR MIT OR Apache-2.0`); without a `license` field: `eval`, `format` (MIT in the legacy `licenses` array), `require-like` (MIT in its license file).
   - **Outside the allowlist of ADR-0023, no choice (7 entries, 5 licenses):** `tslib` ×2 **0BSD** (TypeScript helpers; dependency of 71 packages, among them every `@docusaurus/*`; `__extends`/`__assign`/`__awaiter` helpers are in the client bundle `main.js`), `mdn-data` ×2 **CC0-1.0** (data of `css-tree`, CSS minifier), `sax` **BlueOak-1.0.0** (XML parser of `svgo`, `sitemap`, `xml-js`), `argparse` **Python-2.0** (via `js-yaml`, front matter), `caniuse-lite` **CC-BY-4.0** (browser data of `browserslist`, `autoprefixer`, `caniuse-api`; build time). All permissive and compatible with MIT. → **D-1**.
   - `npm ci --ignore-scripts`: about 60 s, about 300 MB `node_modules/`.
   - **Feasibility build** with a copy of the existing pages: the README links (`../../../dev/README.md#…`) fail the build. With the function form of `markdown.hooks.onBrokenMarkdownLinks` that maps a target inside `oec/` (outside `oec/docs/`) to `${OEC_DOCS_REPO_URL:-https://github.com/OI4/oi4-oec/blob/main}/oec/<path>#<anchor>` and throws otherwise, `onBrokenLinks`/`onBrokenAnchors: 'throw'` and a root page (`slug: /`; without it "/" is a broken link of the navbar), `docusaurus build` succeeds in about 6 s: 892 KB, 17 JS files.
2. **ADR-0031** (`project/decisions/ADR-0031-documentation-site.md`, accepted (agent, G-3.4 b); §5 licenses pending D-1): layout, offline check, repository links, versions, license rules, build and npm policy for C-099, cycle cut C-098 … C-103 (A-489 … A-494).
3. **`docscheck`** (`oec/docs/tools`, Go module `github.com/OI4/oec/docs/tools`, standard library only, A-490):
   - `docscheck/docscheck.go` (package) and `cmd/docscheck/main.go` (exit 1 on a problem, 2 on an I/O error).
   - Rules: front matter (`title`, `description` non-empty; `sidebar_position` a non-negative integer; `slug` starts with `/`; exactly one root page); a level-1 heading; every link (inline, reference definition, image, HTML `href`/`src`) outside fenced code and code spans: into the site the file must exist (a folder is an error) with its anchor; into `oec/` outside `oec/docs/` the file or folder must exist, an anchor needs a Markdown target and must exist; out of `oec/`, into `oec/docs/` outside the site, site-absolute, other schemes and queries are errors; `http(s)`/`mailto` are not fetched; no hyperlink or bare URL containing `oi4.github.io` or `oi4-oec-documentation` (G-4.11).
   - Anchors: github-slugger (lower case; letters, marks, digits, connector punctuation, `-`, `_` kept; space → `-`; everything else dropped; repeats `-1`, `-2`; `{#id}` wins; HTML `id`/`name`), on the visible text (link text kept, code-span backticks, `*` and tags removed). Docusaurus and GitHub use the same slugger, so one rule serves both the site and the README links.
   - Tests (`docscheck_test.go`): `TestSitePages` (the real pages pass, ≥ 6 pages, `intro.md` has `slug: /`), `TestSiteLinksSeen` (the scanner sees ≥ 5 links on `registry/index.md` – a silent scanner would pass everything), `TestSlug` (9 headings incl. those the pages link: `compose-profile-registry-c-090`, `http-rest-sse-tls-adr-0029-7`, `images-c-084`), `TestAnchors`, `TestCheckGood` (every allowed link form on a temporary tree), `TestCheckBad` (**24 violations**, each caught), `TestLineNumbers`.
   - Mutation check by hand: one README anchor of `demos/index.md` shortened → `demos/index.md:69: link "../../../dev/README.md#compose-profile-registry-c-09": no heading with the anchor …`, exit 1; restored with `git checkout`.
   - The existing six pages passed unchanged (no broken link or anchor found).
4. **Gate step 10** `oec/docs/check.sh` (gofmt, vet, test, `docscheck` over `docs/`), added to `oec/check.sh` with its header comment.
5. **Getting started** `oec/docs/docs/intro.md` (`slug: /`, `sidebar_position: 0`): what OEC is (assets, OI4 Message Bus, `MAM`/`Health`/`Data`/`Config`/`Event`), the two guidelines referenced by name and section number (G-4.11), what the project provides, "See OEC at work" (build the demo and registry images, `go run ./pki`, `docker compose … --profile demos --profile registry up -d --wait`, the four UIs, `down -v`, `--remove`), "Where to go next" (SDK READMEs, `oec.h` and the C examples, micro SDK page, conformance, schema, dev). Commands taken from the demos and registry pages and the `build.sh` scripts (both resolve their paths from `$0`, so `demos/images/build.sh --remove` works from `oec/`). Not run in C-098 (no Docker; the same commands were run in C-091 and C-085) – the newcomer run is the P8 exit (C-103).
6. **READMEs:** `oec/docs/README.md` (new: layout, writing rules, the check, the build after D-1); `oec/README.md` (rows `docs/`, `sdk/micro/`, `demos/weather/`, `demos/logger/` were stale – now P8 since C-098, P4b done, P6 done; logger Messaging L2; the check paragraph names `sdk/micro/check.sh` and `docs/check.sh`).
7. **ROADMAP P8:** plan and cycle cut (C-098 … C-103) under the existing exit criteria.

## Results

| Item | Result |
|---|---|
| `oec/docs/check.sh` | gofmt clean, vet clean, `go test` ok, `docscheck: 7 pages ok` |
| `TestCheckBad` | 24 of 24 violations caught |
| Docusaurus 3.10.2 tree | 1275 packages; 7 entries with 5 licenses outside the allowlist → D-1 |
| Feasibility build (temporary) | ok in ~6 s with the README-link hook and the root page |

## Agent decisions (G-3.4 b)

- **A-489 Layout of `oec/docs/`** (ADR-0031 §1): `docs/` pages, `tools/` Go module, `check.sh` gate step 10, `README.md`; the site files of Docusaurus at the root of `oec/docs/` from C-099; `build/` and `node_modules/` never committed.
- **A-490 Offline docs check** (ADR-0031 §2): Go, standard library, no node in the gate; links into the site and into `oec/` with anchors by the github-slugger rule; G-4.11 enforced; external URLs not fetched (offline gate).
- **A-491 Repository links** (ADR-0031 §3): pages keep relative links to READMEs; the build rewrites them to `${OEC_DOCS_REPO_URL}` (default `https://github.com/OI4/oi4-oec/blob/main`, the `origin`); every other broken link fails the build. Where it is published stays with Q-011/P9.
- **A-492 Versions** (ADR-0031 §4): Docusaurus 3.10.2 (not the 4.0 canary), React 19.3.1, MDX React 3.1.1, clsx 2.1.1, prism-react-renderer 2.4.1; exact pins, 7-day cool-down; re-checked when committed in C-099.
- **A-493 License rules for npm trees** (ADR-0031 §5): an OR expression passes if one alternative is allowlisted; a package without a `license` field needs a reviewed table entry in the policy test. The five licenses without a choice are **not** decided by the agent → D-1.
- **A-494 Cycle cut** (ADR-0031 §7, ROADMAP P8): C-098 plan + check + Getting started; C-099 site scaffold after D-1; C-100 concepts + migration; C-101 SDK guides; C-102 D7/D8/schema; C-103 exit run. Pages that need no npm can be written before D-1.

## Decision for the PO

**D-1 – licenses of the Docusaurus tree** (G-3.4 a: licenses). Five licenses outside the allowlist of ADR-0023, without an allowlisted alternative: 0BSD (`tslib`, also in the client bundle), CC0-1.0 (`mdn-data`), BlueOak-1.0.0 (`sax`), Python-2.0 (`argparse`), CC-BY-4.0 (`caniuse-lite`, build-time data). All are permissive and compatible with MIT; Docusaurus cannot be built without them.
- (a) allow the five **for the npm tree of the documentation site only** (the policy test of C-099 lists them; nothing changes for the SDKs, the registry or the demos) – recommended;
- (b) add the five to the general allowlist of ADR-0023 (all npm trees and, where they apply, Rust/Go);
- (c) no – no Docusaurus build; the pages stay Markdown in the repository (readable on GitHub, checked by `docscheck`), and the exit criterion "docs build without broken links" is read as "docscheck passes".

## Open / next

- C-099 after D-1 (a) or (b): site scaffold, npm policy test, opt-in build. With (c): C-099 continues with the concepts pages and the P8 plan drops the build.
- Writing pages (C-100 ff.) does not depend on D-1.

## Docker

None in C-098 (no container, image, volume or network created).

## Temporary files

`/tmp/oec-c098-survey` (survey `package.json`, lockfile, `node_modules/` ~300 MB, a copy of the pages, the config of the feasibility build and its `build/`) – removed at the end of the cycle. Nothing of it is committed.

## Handover (G-3.10)

**A new session is recommended** for C-099: this session ran C-091 … C-098 and its context was compacted several times. `prompt.md` (workspace root, German, not committed) holds the start prompt; it depends on D-1.

## Gate

`caffeinate -i oec/check.sh`: **all checks passed**, exit 0, 13 min 33 s (most Go and Rust results cached; no repeat needed). The new step 10 `docs/check.sh` reports `docscheck: 7 pages ok`.

## Addendum 1 – PO decision on the licenses of the Docusaurus npm tree (2026-11-14)

PO answer (chat): "D-1 (a)".

### Applied

- **D-1 (a):** 0BSD (`tslib`), CC0-1.0 (`mdn-data`), BlueOak-1.0.0 (`sax`), Python-2.0 (`argparse`) and CC-BY-4.0 (`caniuse-lite`) are allowed **for the npm tree of the documentation site (`oec/docs`) only**. ADR-0031 status line and §5 (PO decision paragraph), §6/§7 and the consequences no longer wait for D-1; ROADMAP P8 plan; STATUS (last updated, current phase, next step, D6 row, blocking questions, cycle row); REVIEW/REVIEW_DE (D-1 answered, nothing open).
- **Unchanged:** the general allowlist of ADR-0023 and the npm policy of the registry Web UI (ADR-0029 §10); SDK, registry and demo code may not depend on packages under these five licenses.
- **C-099:** the site policy test holds the allowlist of ADR-0023 plus this site-only list; any other license outside both is again a PO question (G-3.4 a).
- Fix: the C-098 cycle row in STATUS pointed at the C-097 log; it now points at this log.
