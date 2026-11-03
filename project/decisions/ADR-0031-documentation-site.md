# ADR-0031 – Documentation site (P8, D6)

- Status: **accepted (agent, G-3.4 b)** for the layout, the offline check, the build and the cycle cut; the licenses of the npm tree outside the allowlist of ADR-0023 (§5) are **pending the PO** (C-098 D-1, G-3.4 a). Until D-1 is answered, no `package.json` and no lockfile of the site are committed.
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
| from C-099 (after D-1) | `package.json`, `package-lock.json`, `docusaurus.config.mjs`, `sidebars.mjs`, `src/css/`, `static/`, `.npmrc`, `build.sh`; the output `build/` and `node_modules/` are never committed (`.gitignore`) |

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

### 5. Licenses (pending, C-098 D-1)

The npm policy of ADR-0029 §10 applies to the site, with two clarifications the agent decides (A-493): an OR expression is accepted if one alternative is in the allowlist; a package without a `license` field needs a reviewed entry in a table of the policy test (name, version, license, where it is stated). The five licenses of the survey outside the allowlist (0BSD, CC0-1.0, BlueOak-1.0.0, Python-2.0, CC-BY-4.0) are a license question (G-3.4 a): **D-1 of C-098**. Proposal: allow them for the npm tree of the documentation site only (not for linked code of the SDKs, the registry or the demos).

### 6. Build and npm policy of the site (C-099, after D-1)

- `build.sh` (opt-in, needs the npm cache or network once): `npm ci --ignore-scripts`, then `docusaurus build` into `build/` (git-ignored); not part of the gate. The site is **not published** (Q-011, P9); content license MIT (G-4.10).
- A Go policy test in `tools/` (in the gate, offline) checks `package.json` (private, exact versions, the allowed dependencies), the lockfile (v3, `registry.npmjs.org`, sha512, licenses per §5) – the rules of ADR-0029 §10. No stamp: unlike the registry, nothing built is committed.

### 7. Cycle cut of P8 (A-494)

1. **C-098:** plan, this ADR, `docscheck` with gate step 10, the Getting started page.
2. **C-099** (after D-1): the site scaffold (§1, §3, §6), the npm policy test, the opt-in build without broken links.
3. **C-100:** Concepts (assets, identities, topics, resources, levels; guideline by section number) and migration notes 1.x → 2.0.
4. **C-101:** SDK guides Rust, C, Go (first application, provider, consumer) and the API reference (links to the generated docs where they exist).
5. **C-102:** D7 and D8 guides, JSON Schema reference and validation how-to.
6. **C-103:** P8 exit run: the build without broken links, a newcomer run of the demos from the site only.

## Consequences

- Every page change is checked in the gate; README anchors referenced from the site can no longer break silently (a renamed README heading fails `docs/check.sh`).
- The Docusaurus build waits for D-1. Writing pages does not: they are Markdown and checked offline.
- The site has its own npm tree (about 1275 packages), separate from the registry Web UI (91 entries); its policy follows ADR-0029 §10.
