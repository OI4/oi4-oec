# `oec/docs/` – documentation site D6

The developer documentation of OEC v2.0: a standalone Docusaurus site
(ADR-0012, ADR-0031). The pages are written in Markdown and are readable on
GitHub as they are; the site build is planned for C-099.

| Path | Content |
|---|---|
| `docs/` | the pages; `intro.md` is the root page (`slug: /`) |
| `docs/demos/` | the demo services (P6) |
| `docs/registry/` | the OEC Registry (P7) |
| `docs/micro-sdk/` | the OEC Micro SDK (P4b) |
| `tools/` | Go module `github.com/OI4/oec/docs/tools` (standard library only): `docscheck` |
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
to fails here.

```sh
cd oec/docs/tools && go run ./cmd/docscheck   # the pages only
```

## Build

Not yet: the Docusaurus tree has five licenses outside the allowlist of
ADR-0023 (decision C-098 D-1 of the PO). After the decision, C-099 adds
`package.json`, the lockfile, the configuration and an opt-in `build.sh`
(`npm ci --ignore-scripts`, `docusaurus build` into the git-ignored
`build/`). The site is not published (Q-011, P9).
