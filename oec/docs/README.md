# `oec/docs/` – documentation D6

The developer documentation of OEC v2.0. Since C-100 addendum 2 its pages
live in the **Community section of the guideline site**
(`oi4-oec-documentation/docs/community`, PO decision Q-036, ADR-0031
amendment C-100 addendum 2):

- [Getting started](https://oi4.github.io/oi4-oec-documentation/docs/community) (`index.md`)
- [Concepts](https://oi4.github.io/oi4-oec-documentation/docs/community/concepts)
- [Demo services](https://oi4.github.io/oi4-oec-documentation/docs/community/demos)
- [OEC Registry](https://oi4.github.io/oi4-oec-documentation/docs/community/registry)
- [OEC Micro SDK](https://oi4.github.io/oi4-oec-documentation/docs/community/micro-sdk)
- [SDKs](https://oi4.github.io/oi4-oec-documentation/docs/community/sdk) – guides
  [Rust](https://oi4.github.io/oi4-oec-documentation/docs/community/sdk/rust),
  [C](https://oi4.github.io/oi4-oec-documentation/docs/community/sdk/c),
  [Go](https://oi4.github.io/oi4-oec-documentation/docs/community/sdk/go) and the
  [API reference](https://oi4.github.io/oi4-oec-documentation/docs/community/sdk/reference)
- [Compliance tools](https://oi4.github.io/oi4-oec-documentation/docs/community/compliance) – guides
  [`oec-check`](https://oi4.github.io/oi4-oec-documentation/docs/community/compliance/oec-check),
  [`oec-infra`](https://oi4.github.io/oi4-oec-documentation/docs/community/compliance/oec-infra),
  [JSON Schemas](https://oi4.github.io/oi4-oec-documentation/docs/community/compliance/schema) and
  [Validate a message](https://oi4.github.io/oi4-oec-documentation/docs/community/compliance/validate)
- [Migration notes 1.x → 2.0](https://oi4.github.io/oi4-oec-documentation/docs/community/migration)

This folder holds their offline check (gate step 10) and the opt-in build.
The guideline site is built and published by the guideline repository
(GitHub Pages from its branch `development`); this repository publishes
nothing (Q-011).

| Path | Content |
|---|---|
| `tools/` | Go module `github.com/OI4/oec/docs/tools` (standard library only): `docscheck` |
| `check.sh` | the docs gate, step 10 of `oec/check.sh` |
| `build.sh` | the opt-in build of the guideline site (yarn of the guideline repository) |

## Writing a page

- Pages are Markdown files under `oi4-oec-documentation/docs/community/`;
  a folder with an `index.md` is a category (`demos/`, `registry/`,
  `micro-sdk/`). The start page is `index.md`.
- Front matter with `title`, `description` and `sidebar_position`; no `slug`
  or `id` – the route follows the file, and the README links rely on it.
- One level-1 heading.
- Link other pages relatively by their file (`../registry/index.md`, not a
  folder or a site path).
- Details stay in the README of the component in `oec/`; link it with its
  anchor as a repository URL,
  `https://github.com/OI4/oi4-oec/blob/main/oec/dev/README.md#compose-profile-demos-c-084`.
  `docscheck` maps the URL to the file of this workspace and checks the file
  and the anchor.
- Refer to the guidelines by section number ("Messaging Guideline 7.5"), do
  not link them (GUARDRAILS G-4.11).
- The guideline site reads `.md` as MDX: `{`, `}` and `<` before a letter,
  `/` or `!` only inside backticks or code blocks.
- Content license: MIT.

A README of `oec/` links a page by its route,
`https://oi4.github.io/oi4-oec-documentation/docs/community/<route>`
(`<route>` = the file without `.md` or `/index.md`, no trailing slash).

## Check

```sh
oec/docs/check.sh
```

Offline, without node: gofmt, `go vet` and `go test` of `tools/`, then
`docscheck` over the Community folder – front matter, `index.md`, a level-1
heading, every link and anchor between the pages and into the files of
`oec/` (anchors as GitHub and Docusaurus derive them), no relative link out
of the section, no hyperlink to the guideline, MDX-safe text – and the links
of the READMEs of `oec/` to the pages (route and anchor). A renamed README
heading that a page links to fails here.

```sh
cd oec/docs/tools && go run ./cmd/docscheck   # pages and README links only
```

## Build

Opt-in, needs node ≥ 18 and, without `node_modules` in the guideline
repository, the network:

```sh
oec/docs/build.sh                                   # docscheck, yarn build of the guideline site
cd ../oi4-oec-documentation && yarn serve           # serve build/ on http://localhost:3000
```

It builds the whole guideline site with its own configuration into its
git-ignored `build/` and fails if a build warning (broken link, broken
Markdown link) names `docs/community` or a page has no HTML page.
