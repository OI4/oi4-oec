// Docusaurus configuration of the documentation site D6 (ADR-0031). Docs
// only (the pages of docs/ at the root, intro.md is "/"), no blog, no
// analytics, no search service; every broken link or anchor fails the
// build. Links to the files of oec/ point at the repository
// (OEC_DOCS_REPO_URL, ADR-0031 §3). The site is not published (Q-011).
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {themes} from 'prism-react-renderer';
import repoLinks from './src/remark/repoLinks.mjs';

const site = path.dirname(fileURLToPath(import.meta.url));
const repo = (process.env.OEC_DOCS_REPO_URL || 'https://github.com/OI4/oi4-oec/blob/main').replace(/\/+$/, '');

/** @type {import('@docusaurus/types').Config} */
export default {
  title: 'OI4 OEC v2.0',
  tagline: 'Developer documentation of OI4 Open Edge Computing v2.0',
  url: process.env.OEC_DOCS_URL || 'http://localhost:3000',
  baseUrl: '/',
  onBrokenLinks: 'throw',
  onBrokenAnchors: 'throw',
  onDuplicateRoutes: 'throw',
  markdown: {
    format: 'detect',
    hooks: {
      onBrokenMarkdownLinks: 'throw',
      onBrokenMarkdownImages: 'throw',
    },
  },
  i18n: {defaultLocale: 'en', locales: ['en']},
  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          path: 'docs',
          routeBasePath: '/',
          sidebarPath: './sidebars.mjs',
          beforeDefaultRemarkPlugins: [
            [repoLinks, {oec: path.resolve(site, '..'), site, repo}],
          ],
        },
        blog: false,
        pages: false,
        sitemap: false,
        theme: {customCss: './src/css/custom.css'},
      }),
    ],
  ],
  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      colorMode: {respectPrefersColorScheme: true},
      docs: {sidebar: {hideable: true}},
      navbar: {
        title: 'OI4 OEC v2.0',
        items: [{type: 'docSidebar', sidebarId: 'docs', position: 'left', label: 'Docs'}],
      },
      footer: {
        style: 'dark',
        links: [{label: 'MIT License', href: `${repo}/oec/LICENSE`}],
        copyright: 'Copyright © 2026 Open Industry 4.0 Alliance - Community. Content and code under the MIT License.',
      },
      prism: {
        theme: themes.github,
        darkTheme: themes.dracula,
        additionalLanguages: ['bash'],
      },
    }),
};
