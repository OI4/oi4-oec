# `oec/docs/` – documentation site D6

The developer documentation of OEC v2.0: a standalone Docusaurus site
(ADR-0012, ADR-0031). The pages are written in Markdown and are readable on
GitHub as they are. The site build is opt-in (`build.sh`, npm); the gate
checks the pages and the npm policy offline. The site is not published
(Q-011, P9).

| Path | Content |
|---|---|
| `docs/` | the pages; `intro.md` is the root page (`slug: /`) |
| `docs/concepts.md` | the concepts in short (P8 increment 3) |
| `docs/migration.md` | the migration notes 1.x → 2.0 (P8 increment 3) |
| `docs/demos/` | the demo services (P6) |
| `docs/registry/` | the OEC Registry (P7) |
| `docs/micro-sdk/` | the OEC Micro SDK (P4b) |
| `docusaurus.config.mjs` | the site configuration: docs only (no blog, no pages, no sitemap, no analytics, no search), broken links and anchors fail the build, footer with the MIT license |
| `sidebars.mjs` | one sidebar, generated from `docs/` (`sidebar_position`) |
| `src/css/custom.css` | the theme colors |
| `src/remark/repoLinks.mjs` | rewrites relative links into the files of `oec/` to repository links; a missing target fails the build |
| `package.json`, `package-lock.json` | the npm tree: seven exact dependencies (Docusaurus 3.10.2, React 19), lockfile v3 |
| `.npmrc` | registry `registry.npmjs.org`, `ignore-scripts`, exact versions |
| `build.sh` | the opt-in build (npm, network): policy test, `npm ci --ignore-scripts`, `docusaurus build` into `build/` |
| `tools/` | Go module `github.com/OI4/oec/docs/tools` (standard library only): `docscheck`, `npmpolicy` |
| `check.sh` | the docs gate, step 10 of `oec/check.sh` |

## Writing a page

- Front matter with `title`, `description` and `sidebar_position`.
- One level-1 heading.
- Link other pages relatively by their file (`../registry/index.md`, not a
  folder or a site path).
- Details stay in the README of the component; link it relatively with its
  anchor (`../../../dev/README.md#compose-profile-demos-c-084`). The site
  build turns such links into links to the repository.
- Refer to the guidelines by section number ("Messaging Guideline 7.5");
  never link the guideline site (GUARDRAILS G-4.11).
- Content license: MIT (`oec/LICENSE`).

## Check

```sh
oec/docs/check.sh
```

Offline, without node: gofmt, `go vet` and `go test` of `tools/`, then
`docscheck` over `docs/` – front matter, a single root page, a level-1
heading, every link and anchor into the site and into the files of `oec/`
(anchors as GitHub and Docusaurus derive them), no link out of `oec/` and no
hyperlink to the guideline site. A renamed README heading that a page links
to fails here. The test of `tools/npmpolicy` checks the npm tree offline
(ADR-0031 §5, §6; rules of ADR-0029 §10):

- `package.json`: private, MIT, the seven dependencies with exact versions,
  the scripts `build`, `serve` and `start` only, no devDependencies.
- `package-lock.json`: v3, every package from `registry.npmjs.org` with a
  sha512 integrity, npm aliases and install scripts only as reviewed in
  `npmpolicy.go`.
- Licenses: the allowlist of ADR-0023 or the five licenses allowed for this
  site only (0BSD, CC0-1.0, BlueOak-1.0.0, Python-2.0, CC-BY-4.0; C-098
  D-1 (a)); an OR expression needs one allowed choice; a package without a
  `license` field needs a reviewed table entry. Any other license is a
  question to the PO.
- `.npmrc` with the registry and `ignore-scripts=true`; the configuration
  keeps the build rules (broken links throw, no blog, no analytics).

```sh
cd oec/docs/tools && go run ./cmd/docscheck   # the pages only
```

## Build

Opt-in, needs node ≥ 22.12, npm and the network:

```sh
oec/docs/build.sh                     # policy test, npm ci, docusaurus build
cd oec/docs && npm run serve          # serve build/ on http://localhost:3000
```

`build/`, `.docusaurus/` and `node_modules/` are git-ignored and never
committed. The build fails on a broken link or anchor in the site, on a
Markdown link to a missing page and on a link to a missing file of `oec/`;
the anchors of README links are checked by `docscheck` in the gate.

Relative links into the files of `oec/` become links to the repository:
`$OEC_DOCS_REPO_URL/oec/<path>`, default
`https://github.com/OI4/oi4-oec/blob/main`. `OEC_DOCS_URL` sets the site URL
(default `http://localhost:3000`). The site is not published (Q-011, P9).

To update the npm tree: change the exact versions in `package.json`, then

```sh
cd oec/docs
npm install --package-lock-only --ignore-scripts --before=<7 days ago> --no-audit --no-fund
cd tools && go test ./npmpolicy
```

A new license, alias or install script fails the policy test; review it
(ADR-0031 §5) before changing the tables in `npmpolicy.go`.
