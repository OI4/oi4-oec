// Remark plugin of the site (ADR-0031 §3, amendment C-099): a relative link
// of a page to a file or folder of oec/ outside the site oec/docs/ (a
// README, a header, an example folder) becomes a link to the repository,
// <repo>/oec/<path>#<anchor>. It runs before the plugins of Docusaurus,
// which would otherwise bundle a non-Markdown file as an asset or report a
// folder as a broken link. Every other link is left to Docusaurus
// (onBrokenLinks, onBrokenAnchors, onBrokenMarkdownLinks: 'throw').
// docscheck (tools/) checks the same links offline in the gate.
import {existsSync} from 'node:fs';
import path from 'node:path';

const scheme = /^[a-z][a-z0-9+.-]*:/i;

const within = (file, dir) => {
  const rel = path.relative(dir, file);
  return rel === '' || (!rel.startsWith('..') && !path.isAbsolute(rel));
};

// rewrite returns the repository URL of url (a link of the page source) or
// undefined if the link is not a link to an existing file or folder of oec/
// outside the site (Docusaurus then fails the build on it).
export function rewrite(url, source, {oec, site, repo}) {
  if (url === '' || scheme.test(url) || url.startsWith('/') || url.startsWith('#')) {
    return undefined;
  }
  const hash = url.indexOf('#');
  const target = hash < 0 ? url : url.slice(0, hash);
  if (target.includes('?')) {
    return undefined;
  }
  const file = path.resolve(path.dirname(source), decodeURI(target));
  if (!within(file, oec) || within(file, site) || !existsSync(file)) {
    return undefined;
  }
  const rel = path.relative(oec, file).split(path.sep).join('/');
  const folder = target.endsWith('/') ? '/' : '';
  return `${repo}/oec/${encodeURI(rel)}${folder}${hash < 0 ? '' : url.slice(hash)}`;
}

export default function repoLinks(options) {
  return (tree, file) => {
    const visit = (node) => {
      if ((node.type === 'link' || node.type === 'definition') && typeof node.url === 'string') {
        node.url = rewrite(node.url, file.path, options) ?? node.url;
      }
      for (const child of node.children ?? []) {
        visit(child);
      }
    };
    visit(tree);
  };
}
