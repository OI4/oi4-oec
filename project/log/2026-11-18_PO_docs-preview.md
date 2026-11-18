# 2026-11-18 – Unlisted preview of the guideline site

PO order (outside the cycles): publish the current site before the release,
but not findable – a public teaser page ("Big news to come") at the root,
the OEC 2.0 site only on an unlinked path. No password needed. STATUS is
not changed here.

## Situation

- `OI4/oi4-oec-documentation` is **private**; GitHub Pages serves branch
  `gh-pages` publicly at `https://oi4.github.io/oi4-oec-documentation/`
  (legacy build). The live site still shows the old `development` content.
- `deploy.yml` deploys on push to `development`; it used Node 18 and
  `yarn install --frozen-lockfile`, which fail with Docusaurus 3.10
  (Node >= 20) and Yarn 4.
- Docusaurus has no access control; GitHub Pages neither (only private
  Pages with Enterprise Cloud). An unlisted path matches the PO request.

## Decision (agent, G-3.4 b)

One switch, no change to the sources of the pages:
`OEC_DOCS_PREVIEW=<path>` sets `baseUrl` to `/oi4-oec-documentation/<path>/`
and `noIndex` (noindex/nofollow meta, no sitemap);
`scripts/build-site.sh` adds the static `teaser/index.html` at the root.
The path is not committed anywhere; it is passed as an environment variable
or the repository variable `OEC_DOCS_PREVIEW` of the workflow. For the
release the variable is removed and the site is at the root again, so the
links of the `oec/` READMEs (`docscheck.SiteURL`) stay valid.

## Done

- `oi4-oec-documentation` commit `3661c53` (branch `feature/spec-findings`,
  not pushed): config switch, teaser page, `scripts/build-site.sh`,
  `deploy.yml` (build script, Node 22, `yarn install --immutable`,
  `yarn deploy --skip-build`), README section.

## Verification

- Preview build: `build/` holds only `index.html` (teaser, no reference to
  the path), `img/`, `.nojekyll` and `<path>/`; every page below `<path>/`
  carries `noindex, nofollow`; no `sitemap.xml`. Served locally below
  `/oi4-oec-documentation/`: teaser, its images, the preview home page and
  the JS bundle answer 200.
- Regular build without the variable: unchanged (site at the root, sitemap).

## Open (PO)

- Publish: either `OEC_DOCS_PREVIEW=<path> scripts/build-site.sh` and
  `yarn deploy --skip-build` locally (pushes only `gh-pages`), or set the
  repository variable and push to `development`.
