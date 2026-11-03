# ADR-0031 – Documentation site (P8, D6)

- Status: **accepted (agent, G-3.4 b)** for the layout, the offline check, the build and the cycle cut (amendment C-099: site scaffold, repository links by a remark plugin, npm policy; **amendment C-100 addendum 2**: on PO decision the pages live in the Community section of the guideline site, §1, §3, §5, §6 replaced); the licenses of the npm tree outside the allowlist of ADR-0023 (§5) **decided by the PO** (C-098 D-1 (a), 2026-11-14, G-3.4 a): allowed for the npm tree of the documentation site only.
- Date: 2026-11-14
- Deciders: agent (layout, tools, check, versions, cycle cut); product owner (Thomas) – licenses, publication, scope
- Related: ADR-0004 (MIT), ADR-0012 (monorepo, docs = standalone Docusaurus site in `oec/docs`), ADR-0023 (dependency policy and license allowlist), ADR-0029 §10 and amendment C-089 (npm policy of the registry Web UI, the model for §6), GUARDRAILS G-4.10 (MIT for new artifacts) and G-4.11 (no hyperlinks to the guideline site), Q-011 (publication), ROADMAP P8

## Context

P8 builds the developer documentation D6. ADR-0012 fixed the tool (Docusaurus) and the place (`oec/docs`). Since P6 every phase added its pages under `oec/docs/docs/` (demos C-085, registry C-091, micro SDK C-097) as plain Markdown, without a site around them and without a check: a broken link or anchor went unnoticed.

The pages link to the READMEs of `oec/` (the details stay in one place, A-379). In a Docusaurus site such a link leaves the site and is a broken link; it has to become a link to the repository.

A survey in C-098 (temporary folder, removed) resolved the tree of Docusaurus 3.10.2 (the newest stable release, 2026-07-10; 4.0 is a canary only):

- `package-lock.json` v3 with **1275 packages** (resolved with `npm install --package-lock-only --ignore-scripts --before=2026-10-27`, the 7-day cool-down of ADR-0029 amendment C-089). All are runtime dependencies of the site project; `npm ci --ignore-scripts` takes about 60 s and about 300 MB of `node_modules/`.
- Licenses: MIT 1077, MIT-0 65, ISC 43, Apache-2.0 35, BSD-2-Clause 21, BSD-3-Clause 19 (allowlist); five OR expressions with an allowlisted choice (`type-fest` ×2 `MIT OR CC0-1.0`, `opener` and `path-is-inside` `WTFPL OR MIT`, `rc` `BSD-2-Clause OR MIT OR Apache-2.0`); three packages without a `license` field (`eval` and `format`: MIT in the legacy `licenses` array; `require-like`: MIT in its license file).
- **Outside the allowlist, without a choice:** `tslib` ×2 (**0BSD**; TypeScript helpers, used by 71 packages, its helpers also in the client bundle), `mdn-data` ×2 (**CC0-1.0**; CSS data of `css-tree`, the CSS minifier), `sax` (**BlueOak-1.0.0**; XML parser of `svgo` and `sitemap`), `argparse` (**Python-2.0**; via `js-yaml`, front matter), `caniuse-lite` (**CC-BY-4.0**; browser data of `browserslist`/`autoprefixer`, build time only). All five are permissive and compatible with MIT; Docusaurus does not build without them.
- Feasibility build with the existing pages: the README links fail the build (`onBrokenMarkdownLinks`) unless rewritten; with the function form of `markdown.hooks.onBrokenMarkdownLinks` that returns the repository URL, a root page (`slug: /`), `onBrokenLinks: 'throw'` and `onBrokenAnchors: 'throw'` the build succeeds in about 6 s (892 KB output, 17 JS files).

## Decision

### 1. Layout (A-489)

| Path | Content |
|---|---|
| `docs/` | the pages (Markdown/MDX, front matter `title`, `description`, `sidebar_position`); `intro.md` is the root page (`slug: /`) |
| `tools/` | Go module `github.com/OI4/oec/docs/tools` (standard library only): package `docscheck` and the command `cmd/docscheck` |
| `check.sh` | the docs gate, step 10 of `oec/check.sh` |
| `README.md` | layout, the check, the build |
| from C-099 | `package.json`, `package-lock.json`, `docusaurus.config.mjs`, `sidebars.mjs`, `src/css/`, `static/`, `.npmrc`, `build.sh`; the output `build/` and `node_modules/` are never committed (`.gitignore`) |

### 2. Offline check in the gate (A-490)

The gate needs no node and no network (as ADR-0029 §10). `docscheck` checks every page of `docs/`:

- front matter: present, non-empty `title` and `description`, `sidebar_position` a non-negative integer, `slug` starting with `/`, exactly one root page (`slug: /`); a level-1 heading;
- every link – inline, reference definitions, images, `href`/`src` of HTML – outside fenced code and code spans:
  - into the site: the page file exists (a folder is an error: link `index.md`), its anchor exists;
  - into `oec/` outside `oec/docs/`: the file or folder exists; an anchor needs a Markdown target and must exist there;
  - out of `oec/`, into `oec/docs/` outside the site, site-absolute (`/…`), other schemes or queries: an error; `https://`, `http://` and `mailto:` are not fetched (offline);
- anchors are derived as GitHub and Docusaurus derive them (github-slugger: lower case, letters, marks, digits, `-`, `_` kept, spaces → `-`, every other character dropped; repeated slugs `-1`, `-2` …; an explicit `{#id}` wins; HTML `id`/`name` anchors count);
- **G-4.11:** no hyperlink and no bare URL to the guideline site or its repository (`oi4.github.io`, `oi4-oec-documentation`); guideline sections are referenced by number.

Tests: the site pages pass, the scanner sees the links of a real page, slug and anchor rules, a good tree, 24 violations on temporary trees (each caught), line numbers.

### 3. Repository links (A-491)

A link from a page to a file of `oec/` stays relative in the source (readable on GitHub and checked by §2). The site build rewrites it with the function form of `markdown.hooks.onBrokenMarkdownLinks` to `${OEC_DOCS_REPO_URL}/oec/<path>#<anchor>`; the default of `OEC_DOCS_REPO_URL` is `https://github.com/OI4/oi4-oec/blob/main` (the `origin` of the workspace). Any other broken link fails the build (`onBrokenLinks`, `onBrokenAnchors`: `throw`). Where the site is published and to which repository it links is a PO decision (Q-011, P9).

### 4. Versions (A-492)

Docusaurus **3.10.2** (`@docusaurus/core`, `@docusaurus/preset-classic`), `react`/`react-dom` **19.3.1** (as the registry Web UI), `@mdx-js/react` 3.1.1, `clsx` 2.1.1, `prism-react-renderer` 2.4.1; exact versions, lockfile resolved with a 7-day `--before` cool-down; re-checked when C-099 commits them.

### 5. Licenses (PO decision C-098 D-1 (a))

The npm policy of ADR-0029 §10 applies to the site, with two clarifications the agent decides (A-493): an OR expression is accepted if one alternative is in the allowlist; a package without a `license` field needs a reviewed entry in a table of the policy test (name, version, license, where it is stated). The five licenses of the survey outside the allowlist (0BSD, CC0-1.0, BlueOak-1.0.0, Python-2.0, CC-BY-4.0) are a license question (G-3.4 a): **D-1 of C-098**. Proposal: allow them for the npm tree of the documentation site only (not for linked code of the SDKs, the registry or the demos).

**PO decision (C-098 addendum 1, D-1 (a), 2026-11-14):** the five licenses 0BSD, CC0-1.0, BlueOak-1.0.0, Python-2.0 and CC-BY-4.0 are allowed **for the npm tree of the documentation site (`oec/docs`) only**. The general allowlist of ADR-0023 and the npm policy of the registry Web UI (ADR-0029 §10) stay unchanged; SDK, registry and demo code may not depend on packages under these licenses. The site policy test (C-099) holds this site-only list next to the allowlist; a new license outside both is again a PO question.

### 6. Build and npm policy of the site (C-099)

- `build.sh` (opt-in, needs the npm cache or network once): `npm ci --ignore-scripts`, then `docusaurus build` into `build/` (git-ignored); not part of the gate. The site is **not published** (Q-011, P9); content license MIT (G-4.10).
- A Go policy test in `tools/` (in the gate, offline) checks `package.json` (private, exact versions, the allowed dependencies), the lockfile (v3, `registry.npmjs.org`, sha512, licenses per §5) – the rules of ADR-0029 §10. No stamp: unlike the registry, nothing built is committed.

### 7. Cycle cut of P8 (A-494)

1. **C-098:** plan, this ADR, `docscheck` with gate step 10, the Getting started page.
2. **C-099**: the site scaffold (§1, §3, §6), the npm policy test, the opt-in build without broken links.
3. **C-100:** Concepts (assets, identities, topics, resources, levels; guideline by section number) and migration notes 1.x → 2.0.
4. **C-101:** SDK guides Rust, C, Go (first application, provider, consumer) and the API reference (links to the generated docs where they exist).
5. **C-102:** D7 and D8 guides, JSON Schema reference and validation how-to.
6. **C-103:** P8 exit run: the build without broken links, a newcomer run of the demos from the site only.

## Consequences

- Every page change is checked in the gate; README anchors referenced from the site can no longer break silently (a renamed README heading fails `docs/check.sh`).
- The Docusaurus build starts in C-099 (D-1 (a) answered). Writing pages does not depend on it: they are Markdown and checked offline.
- The site has its own npm tree (about 1275 packages), separate from the registry Web UI (91 entries); its policy follows ADR-0029 §10.

## Amendment C-099 (2026-11-15) – increment 2: site scaffold and npm policy (agent, G-3.4 b)

Increment 2 of the cycle cut (§7), within §1, §3, §4, §5 and §6. Details: `log/2026-11-15_C-099_docs-site-scaffold.md`, REVIEW A-495 … A-500.

1. **Versions (§4, A-495):** re-checked at the start, unchanged: Docusaurus 3.10.2 (4.0 still canary only), `react`/`react-dom` 19.3.1, `@mdx-js/react` 3.1.1, `clsx` 2.1.1, `prism-react-renderer` 2.4.1. Lockfile v3 resolved with `npm install --package-lock-only --ignore-scripts --before=2026-10-27`: **1275 packages**, the tree of the C-098 survey; licenses as in the context of this ADR (MIT 1077 + 3 without a `license` field, MIT-0 65, ISC 43, Apache-2.0 35, BSD-2-Clause 21, BSD-3-Clause 19, five OR expressions, seven entries under the site-only licenses of §5). No new license, no question to the PO.
2. **Layout and configuration (§1, §6, A-496):** `package.json` (private, MIT, `engines` node ≥ 22.12, scripts `build`, `serve`, `start` only, seven exact dependencies, no devDependencies) **without `"type": "module"`** – with it the server bundle of Docusaurus 3.10.2 fails (`require.resolveWeak is not a function`); the configuration files are ES modules by their extension (`docusaurus.config.mjs`, `sidebars.mjs`). The site is docs-only: the docs plugin at `routeBasePath: '/'`, no pages plugin, no blog, no sitemap, no analytics, no search; `markdown.format: 'detect'` (`.md` as CommonMark-compatible Markdown, `.mdx` as MDX); one autogenerated sidebar; the footer names the MIT license (`oec/LICENSE`). `OEC_DOCS_URL` sets the site URL (default `http://localhost:3000`). **No `static/` folder** (nothing to serve yet; added with the first image). `src/remark/` holds the plugin of item 3.
3. **Repository links (§3 changed, A-497):** the function form of `markdown.hooks.onBrokenMarkdownLinks` sees only links to `.md`/`.mdx` files. A link to another file of `oec/` (e.g. `oec.h`) would be bundled into the site as an asset, a folder link (e.g. `examples/`) reported as broken. Instead a local remark plugin `src/remark/repoLinks.mjs` runs **before** the plugins of Docusaurus (`beforeDefaultRemarkPlugins`) and rewrites every relative link or definition that resolves to an **existing** file or folder of `oec/` outside `oec/docs/` to `${OEC_DOCS_REPO_URL}/oec/<path>[/]#<anchor>` (default `https://github.com/OI4/oi4-oec/blob/main`). Every other link is left to Docusaurus, which fails the build: `onBrokenLinks`, `onBrokenAnchors`, `onDuplicateRoutes`, `markdown.hooks.onBrokenMarkdownLinks` and `onBrokenMarkdownImages` are all `throw`. The anchors of the rewritten links are checked offline by `docscheck` (§2), not by the build.
4. **npm policy (§5, §6, A-498):** Go package `tools/npmpolicy` (standard library only), run by `go test ./...` in gate step 10, offline. Rules of ADR-0029 §10 with A-493, adapted to a site tree with 1275 packages:
   - `package.json`: as item 2; no `optionalDependencies`, `peerDependencies`, `bundle(d)Dependencies`, `overrides`, `workspaces`.
   - `package-lock.json`: v3; root entry = `package.json`; the direct dependencies locked at their versions; every entry under `node_modules/`, no `link`, no `inBundle`, no `dev`; a semantic version (prerelease allowed for transitive packages – six are, e.g. `infima 0.2.0-alpha.45`, `cheerio 1.0.0-rc.12` – but not for a direct dependency); `resolved` = the canonical tarball URL of `https://registry.npmjs.org/`; sha512 `integrity`.
   - Reviewed tables in `npmpolicy.go`, each entry with name, version and reason; a stale entry fails: **npm aliases** (`react-helmet-async` → `@slorber/react-helmet-async`, `react-loadable` → `@docusaurus/react-loadable`, both by Docusaurus), **install scripts** (`core-js` – a funding message; `fsevents` 2.3.3 – optional, darwin, ships its prebuilt binary; never run: `ignore-scripts`), **packages without a `license` field** (`eval` 0.1.8, `format` 0.2.2, `require-like` 0.1.2 – MIT, where it is stated).
   - Licenses: an SPDX evaluator (`AND`: all terms allowed; `OR`: one term allowed; `WITH` or a malformed expression: not allowed) over the allowlist of ADR-0023 plus the site-only list of §5. A new license fails the test and is a question to the PO.
   - `.npmrc`: `registry=https://registry.npmjs.org/`, `ignore-scripts=true`, no scoped registry, no `_auth`/token keys. `docusaurus.config.mjs`: keeps `onBrokenLinks: 'throw'`, `onBrokenAnchors: 'throw'`, `onBrokenMarkdownLinks: 'throw'` and `blog: false` and names no analytics, tag manager, search service or guideline site (G-4.11).
   - Tests: the committed site passes; the SPDX evaluator; **39 violations** on copies, each caught; each of the four files missing.
   The registry policy (`oec/registry/internal/uipolicy`, every OR term allowlisted, no site-only list) is unchanged.
5. **Build (§6, A-499):** `build.sh` (opt-in, network): the policy test, `npm ci --ignore-scripts --no-audit --no-fund`, `npm run build` into `build/`. From a clean folder about 30 s (build about 7 s), `node_modules/` 306 MB, `build/` 904 KB with 8 HTML pages; no broken link or anchor. The only warning is npm's deprecation of `uuid` 8.3.2 (via `sockjs` of the dev server, not in the built site). `build/`, `.docusaurus/` and `node_modules/` stay git-ignored.
6. **Build probes (A-500):** a link appended to `intro.md` failed the build for: a missing anchor in the site, a missing page, a Markdown file of `oec/docs/` outside the site, a missing Markdown file, a missing other file and a missing folder of `oec/`. `OEC_DOCS_REPO_URL=https://example.invalid/repo/blob/abc` changed every repository link.

## Amendment C-100 (2026-11-16) – increment 3: Concepts and migration notes (agent, G-3.4 b)

Increment 3 of the cycle cut (§7), within §2 and §3. Details: `log/2026-11-16_C-100_concepts-migration.md`, REVIEW A-501 … A-505.

1. **Pages:** `docs/concepts.md` (assets and roles, identities, topics, resources and methods, messages, compliance levels) and `docs/migration.md` (per breaking change of GL-ADR-001 … 022: what changes, what a 1.x application must do; side-by-side operation of `Oi4` and `Oi4v2`). Both cite the Messaging Guideline and the Runtime & Infrastructure Guideline by section number only (G-4.11) and link the READMEs of `oec/` for details; nothing is taken from `legacy/` (G-1.3).
2. **Sidebar order** (`sidebar_position`): Getting started 0, Concepts 1, Demo services 2, OEC Registry 3, OEC Micro SDK 4, Migration notes 5. The guides of C-101 and C-102 are placed before the migration notes, which stay last.
3. **Checks:** `docscheck: 9 pages ok` in gate step 10; the opt-in `build.sh` builds 10 HTML pages (1.0 MB) without broken links or anchors. §1 … §6 unchanged.

## Amendment C-100 addendum 2 (2026-11-16) – the pages move into the Community section of the guideline site

**PO decision** (Q-036, D-1 of C-100 addendum 1 answered (b), G-3.4 a): the old Community pages of `oi4-oec-documentation/docs/community` are no longer needed; everything of `oec/docs` is written directly there, and the READMEs may link the Community pages. The **engineering consequences** are agent decisions (G-3.4 b, REVIEW A-506 … A-511, details in `log/2026-11-16_C-100_concepts-migration.md`). They replace §1, §3, §5 and §6 and change §2; §4 (versions) now follows the guideline site, §7 (cycle cut) stays with a new target folder.

1. **Location (§1 replaced, A-506):** the pages are Markdown in `oi4-oec-documentation/docs/community/` (`index.md` = Getting started with the community note, `concepts.md`, `demos/`, `registry/`, `micro-sdk/`, `migration.md`; `_category_.json` of the guideline site kept: label Community, position 4). The old 1.x pages (development edge setup guide, `oi4-oec-service/`, `oi4-oec-demos/`) and the 1.x download `static/download/oi4-oec-demo.zip` were removed; the home page and the footer of the guideline site link Getting started, Concepts and the migration notes instead of the removed tutorial. Commits in the guideline repository go to its working branch `feature/spec-findings` (as the findings register); publication stays with the PO (Q-011). `oec/docs/` keeps `tools/` (docscheck), `check.sh`, `build.sh` and `README.md`.
2. **Repository links (§3 replaced, A-507):** a link into the code is written as an absolute repository URL `https://github.com/OI4/oi4-oec/blob/main/oec/<path>#<anchor>`; docscheck maps it to the file of the workspace `oec/` and checks the file and the anchor offline. The remark plugin and `OEC_DOCS_REPO_URL` are gone: the guideline site builds its own configuration, which this project does not change for its pages.
3. **Offline check (§2 changed, A-508):** `docscheck` checks `docs/community` (flag `-docs`, default `../../oi4-oec-documentation/docs/community` next to the workspace, as the inventory check of gate step 1): `title` and `description`, `sidebar_position` a non-negative integer, **no `slug` and no `id`** (the route follows the file), a level-1 heading, `index.md` at the root; relative links stay inside the section and name files with their anchors; any other URL of `github.com/OI4/oi4-oec/`, site-absolute links and other schemes are errors; G-4.11 as before (no link to `oi4.github.io` or `oi4-oec-documentation`, guideline sections by number); new **MDX rule**: `{`, `}` and `<` followed by a letter, `/` or `!` outside code are errors, because the guideline site reads `.md` as MDX. New **README check** (`CheckReadmes`): every link of a README of `oec/` to `https://oi4.github.io/oi4-oec-documentation/docs/community/<route>` must name an existing page (`<route>.md` or `<route>/index.md`) and anchor, no trailing slash, no `.md`; a relative README link to the removed `docs/docs/` is an error. Output: `docscheck: 9 pages ok, 10 README links to them ok`.
4. **No own npm tree (§5, §6 replaced, A-509):** `package.json`, the lockfile, `.npmrc`, `docusaurus.config.mjs`, `sidebars.mjs`, `src/` and the npm policy test `tools/npmpolicy` were removed from `oec/docs`. The yarn tree of the guideline site belongs to the guideline repository and is not part of our dependency policy; the license allowance of C-098 D-1 (a) has no object any more. Gate step 10 runs no npm policy.
5. **Build (§6 replaced, A-510):** the opt-in `oec/docs/build.sh` runs docscheck, `yarn install --immutable` (only if `node_modules/` is missing) and `yarn build` of the guideline site; it fails if a warning names `docs/community` (the site's `onBrokenLinks` is `warn`) or if a page has no HTML file. About 18 s with an existing `node_modules/`.
6. **Cycle cut (§7):** increments 4 … 6 (C-101 … C-103) write their pages into `docs/community`; the exit criterion "docs build without broken links" is judged by `build.sh`.

## Amendment C-101 (2026-11-17) – increment 4: SDK guides and API reference (agent, G-3.4 b)

Increment 4 of the cycle cut (§7), within the amendment C-100 addendum 2. Details: `log/2026-11-17_C-101_sdk-guides.md`, REVIEW A-512 … A-519.

1. **Pages (A-512, A-513):** a folder `docs/community/sdk/` – `index.md` (SDKs: what the SDKs do per Messaging level, prerequisites, where the application finds the broker, connecting to `oec/dev`, checking with `oec-check`), `rust.md`, `c.md`, `go.md` (each: build or dependency, a first application at Level 1, a provider with Data, `Set`, Config and Generic events, a consumer with subscriptions, `Get` and `Set`, the complete demo of the language) and `reference.md`. Content derived from the SDK READMEs, the examples and `oec.h`; details linked by exact README anchors, not copied.
2. **Sidebar order** (`sidebar_position`): Getting started 0, Concepts 1, Demo services 2, OEC Registry 3, OEC Micro SDK 4, SDKs 5 (inside: Rust 1, C 2, Go 3, API reference 4), Migration notes 6 (still last). Getting started and Concepts link the SDK pages instead of the SDK READMEs.
3. **Code snippets (A-514):** every snippet was compiled against the SDK of the workspace, and its provider and Config parts were run (without a broker); the consumer parts were compiled only. This check is not part of the gate: the pages link the complete examples, which the gate builds and tests; the C-103 exit run repeats the check.
4. **API reference (A-516):** pointers only – rustdoc (`cargo doc -p oec --no-deps --open`), the comments of `oec.h`, `go doc`, the comments of `oecm.h` – and a table that maps the names of the three SDKs. Nothing is generated or published by the site (Q-011); the gate already builds the rustdoc with warnings as errors, compares `oec.h` with the cbindgen output and vets the Go package.
5. **Checks:** `docscheck: 14 pages ok, 19 README links to them ok` in gate step 10; the opt-in `build.sh` built the guideline site without a warning for `docs/community`, 14 HTML pages of the section.
