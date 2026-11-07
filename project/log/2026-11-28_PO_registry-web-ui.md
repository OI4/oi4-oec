# 2026-11-28 – Registry Web UI in the OI4 style

PO order (outside the cycles): "Die Webseite der Registry finde ich optisch
nicht ansprechend und auch von der Useability her könnte sie besser werden.
Zudem macht es Sinn, sie im Stile der Open Industry 4.0 Alliance zu
gestallten. Mach dir bitte Gedanken und überarbeite die Webpage der Registry
selbständing". All decisions below are agent decisions (G-3.4 b) and can be
overruled. STATUS and the C-112 log are not changed here.

Before this order the PO could not reach the UI in `oec/dev`: it is served
only over HTTPS on the web port (`https://localhost:5798/`, certificate of
the dev CA `oec/dev/generated/broker/ca.pem`); the API port 5799 serves
`/api` only (`/` = `404`). No change was needed for that.

## Decisions (agent, G-3.4 b)

- **Within ADR-0029 §9/§10**: still React 19.3.1 + react-dom only, no new
  npm package, no CSS framework, no web font (the system font stack);
  policy, CSP rules and stamp unchanged.
- **OI4 style**: colours of the alliance's site (green `#76B82A`, navy
  `#061520`, grey `#60696E`, blue `#2EA3F2`), navy app bar and footer with a
  green rule, uppercase section titles with a green bar, cards on a light
  grey page; dark theme with the same accents. Theme: system by default,
  "system / light / dark" in the app bar (`localStorage`
  `oec-registry-theme`).
- **No official logo**: the OI4 logo is available only as PNG (not allowed
  in `dist/`) and is a trademark of the alliance. The app bar and the
  favicon show a simplified own signet (SVG). Open for the PO below.
- **Usability**: key figures as buttons (applications, devices, attention,
  Events) that filter the list or open the Event trail; the asset list with
  search (name, IL, ServiceType, manufacturer), filter, sort and collapsible
  devices, assets that need attention marked; an overview pane while
  nothing is selected (Health distribution, attention list, ServiceTypes,
  observed levels, latest Events, the registry itself); the detail in tabs
  (Overview, Nameplate, Interfaces, Conformity, Events) instead of one long
  page; the PublicationList filterable by resource; the Event trail as a
  page of its own with search, severity, category and pause. Routes
  `#/events` and `#/asset/<IL>[/<tab>]` (bookmarkable), `/` = search,
  `Esc` = close. One column below 1000 px, the selected detail first.
- **Event severity**: Syslog by level; a Status Event by the class of its
  StatusCode (bits 30/31 of `Number`, MG 8.3.6.1), `Number` in hex.
- **Bug fix (also in the old UI)**: the detail reader aborted its request on
  every summary change; the summary of a busy application (OEC Weather,
  `lastSeen`) changes faster than a request takes, so its detail stayed at
  "Loading …" forever. Now one request in flight plus one queued, at most
  once per second, no abort.

## Done

- `oec/registry/ui/src`: `App.tsx` (shell, routes, theme, key figures,
  keyboard), `Dashboard.tsx` (new), `AssetList.tsx` (new, replaces
  `AssetTable.tsx`), `AssetDetail.tsx` (tabs, reader fix), `EventList.tsx`,
  `parts.tsx`, `format.ts`, `HealthChart.tsx`, `style.css` (rewritten);
  `index.html` (title, `theme-color`, description), `public/favicon.svg`.
- `ui/build.sh`: `dist/`, `build.stamp`, `src/ui_assets.rs` regenerated
  (JS 271 kB, CSS 22 kB).
- `oec/registry/README.md` and the Community page
  `oi4-oec-documentation/docs/community/registry/index.md` (section Web UI);
  ADR-0029 amendment 2026-11-28.
- Image rebuilt, compose service `oec-registry` recreated, `oi4-oec-registry`
  re-synced.

## Verification

- `ui/build.sh`: `tsc` strict, vite, uipolicy – "ui: ok".
- Visual check with `vite preview` (proxy to the running dev registry,
  8 assets) in headless Chrome: overview light and dark, detail tabs of
  OEC Logger and OEC Weather (now loads), Event trail with four sample
  Events (Warning, Error, Status Bad `0x80000000`, Notice – published once
  to the dev broker under the logger's AppId; gone with the next recreate),
  600 px width.
- `oec/registry/check.sh`: Rust 32 tests, clippy, deny; Go incl. D8
  T8-BUS-03, D7 level 3, HTTP and uipolicy – ok.
- `oec/check.sh`: all checks passed.

## Open for the PO

- An official OI4 logo as SVG (and the permission to use it) would replace
  the own signet in the app bar and the favicon.
- Push of the workspace and of `oi4-oec-registry` (not done).
