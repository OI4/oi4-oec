# ADR-0012 – Local workspace repository and `oec/` monorepo

- Status: accepted
- Date: 2026-10-12
- Deciders: product owner (Thomas), C-004
- Related: Q-002, Q-007, Q-011, ADR-0009 (amended)

## Decision

1. **The workspace is a local git repository** (root `/Users/thomas/work/github/oi4/oi4-oec/oi4-oec`, no remote). It versions `AGENTS.md`, `prompt.txt`, `project/` and `oec/` together. `legacy/` and `oi4-oec-documentation/` are separate git repositories and are excluded via `.gitignore`; IDE files too.
2. **All deployables live in the monorepo folder `oec/`**: `schema/`, `sdk/rust/`, `conformance/`, `demos/weather/`, `demos/logger/`, `registry/`, `docs/`, `dev/` (see `oec/README.md`). Folders are created when their phase starts.
3. **Documentation (D6) is a new standalone Docusaurus site in `oec/docs/`**, without links to the guideline site `oi4-oec-documentation`. Guideline sections may be referenced by number in text. The existing `oi4-oec-documentation` stays untouched (it remains the source of truth per ADR-0001).
4. **Publication:** nothing is published for now. Parts may be published later; this must be reconsidered in light of the EU **Cyber Resilience Act** (Q-011).

## Consequences

- ADR-0009 is amended: "publish on GitHub" is replaced by "local git repository; publication later after CRA review". The rule "every change is committed individually" applies **now**.
- Later publication of parts can use `git subtree split` or a fresh export per artifact.
- `conformance/` must not depend on `sdk/` (ADR-0006); CI/scripts enforce this once both exist.

## Amendment C-007 (ADR-0014, ADR-0016)

- `oec/conformance/` hosts both compliance tools, D7 and D8.
- New folder `oec/sdk/micro/` for the micro SDK D9.

## Amendment C-010 (Q-011 working assumption)

The future publication target is **private GitHub repositories**, not public availability on the internet. Until the PO gives the go, the repository stays local (no remote, nothing pushed). The CRA review (Q-011) is only needed before a public release.

## Amendment C-013 (ADR-0018)

- New folder `oec/spec/` for the machine-readable spec baseline (REQ catalog in YAML, topic grammar, Method × Resource matrix, test vectors, Go tools). It is created in the first P1 execution cycle.
- `oec/spec/` must not depend on `sdk/` or `conformance/`; `conformance/` and the SDKs may read its data files.

## Amendment C-100 addendum 2 (PO, 2026-11-16) – documentation in the guideline site

Point 3 is changed by the PO (Q-036, D-1 of C-100 addendum 1 answered (b)): the documentation D6 is no longer a standalone site in `oec/docs/`. Its pages are written directly in the Community section of the guideline site, `oi4-oec-documentation/docs/community`, which replaced the old 1.x Community pages. `oec/docs/` keeps only the offline check (`docscheck`, gate step 10), the opt-in build script and its README (ADR-0031 amendment C-100 addendum 2). The guideline chapters, the guideline ADRs and the findings register in `oi4-oec-documentation` stay as before (ADR-0001); its changes for the Community section are committed on the working branch `feature/spec-findings`. Guideline sections are still referenced by number, not by link (GUARDRAILS G-4.11).

## Amendment 2026-11-28 (PO) – demo repository `oi4-oec-demo`

PO order: "alles von oi4-oec-demo löschen und alles was wir unter oec für die Demos haben dort hin kopieren … nur noch die neuen Demos dort haben". The repository `OI4/oi4-oec-demo` (public, default branch `development`) is cloned as `oi4-oec-demo/` in the workspace root – a separate git repository like `legacy/` and `oi4-oec-documentation/`, excluded via `.gitignore` (not the read-only copy in `legacy/`, G-1.3).

- Its content is replaced: the OEC 1.x demo is deleted (it stays in its history); `demos/` is an **unmodified copy of `oec/demos`** (the tracked files, same blobs and modes), next to a README of its own, `LICENSE` (MIT, `oec/LICENSE`) and a `.gitignore`. The repository is laid out like the OEC source tree but contains only `demos/`; paths that leave `demos/` refer to the OEC source tree.
- **`oec/demos` stays the source** (agent, G-3.4 b): the registry, the tool image, `ci/sbom.sh` and the gate use parts of it (`internal/sbom`, `images/licenses`), and rewriting the relative paths would fork the demos from their gate tests. A change is made in `oec/demos` and copied again (log 2026-11-28_PO_demo-repo).
- Nothing is pushed without the PO (G-3.8); the repository is public, so a push is a publication (Q-011).

## Amendment 2026-11-28 (PO) – registry repository `oi4-oec-registry`

PO order: "Mach nun das Gleiche bitte für oi4-oec-registry". `OI4/oi4-oec-registry` (public, default branch `development`) is cloned as `oi4-oec-registry/` in the workspace root, a separate git repository excluded via `.gitignore`, under the same rules as the demo repository above: the OEC 1.x registry is deleted (it stays in its history); `registry/` is an **unmodified copy of `oec/registry`** (incl. the committed `ui/dist/`), next to a README of its own, `LICENSE` and a `.gitignore`; `oec/registry` stays the source (gate step 8; `oec/demos/internal/sbom` reads its `ui/package-lock.json`); no push without the PO (log 2026-11-28_PO_registry-repo).

## Amendment 2026-11-28 (PO) – smart sensor demos `oec/demos-smart-sensors`

PO order: "bring das ESP Beispiel nun auch dort hin. Mach dazu vielleicht noch ein demos-smart-sensors parallel zu demos". The DHT22 firmware of the hardware run (log 2026-11-28_HW_esp32-dht22) becomes a demo of its own:

- **Source `oec/demos-smart-sensors/`** (agent, G-3.4 b), next to `oec/demos`: one folder per board and sensor, first `esp32-dht22/` (ESP-IDF 6.1, Micro SDK component `oecm` by the relative path `../../../sdk/micro/port/esp-idf/oecm`, so it builds only in the OEC source tree; `build.sh`, `provision.sh` for `oec/dev`). Firmware is not part of `oec/check.sh` (no ESP-IDF in the gate); `ci/micro-esp-idf.sh` builds it in the Espressif container of `micro.yml`; `TestReleaseVersion` pins its `PROJECT_VER` to `oec/VERSION`.
- **Copy `oi4-oec-demo/demos-smart-sensors/`**: an unmodified copy of the tracked files, like `demos/` (amendment above); changes are made in `oec/demos-smart-sensors` and copied again. No push without the PO (Q-011).

## Amendment 2026-11-28 (PO) – integrations `oec/integrations/`

PO order: "Überlege ob es möglich wäre für die OEC eine Homeassistant Integration zu entwickeln, also dass es über HACS installierbar wäre. Falls es machbar ist, baue dazu einen Entwicklung in oi4-oec-demo (Subfolder darfst du entscheiden … Vielleicht Integrations? Node-Red könnte als nächstes folgen)". Decisions of the agent (G-3.4 b; log 2026-11-28_PO_home-assistant-integration):

- **Source `oec/integrations/<platform>/`**, next to `oec/demos`: integrations of OEC into third-party platforms, first `home-assistant/` (Python custom integration `oi4_oec`; Python because Home Assistant integrations are Python only), later e.g. `node-red/`. Not part of `oec/check.sh` (no Python and no Home Assistant in the gate); `integrations/home-assistant/check.sh` is opt-in. `TestReleaseVersion` pins its version to `oec/VERSION`.
- **Copy into `oi4-oec-demo`** in the layout HACS requires: `custom_components/oi4_oec/` and `hacs.json` at the repository root, everything else of `oec/integrations/home-assistant/` in `integrations/home-assistant/` (unmodified files; the checks find `custom_components/` in both layouts). Changes are made in `oec/integrations` and copied again. HACS installs only from a public repository; `OI4/oi4-oec-demo` is private at the time of writing (GitHub API, 2026-11-28) – making it public and pushing are decisions of the PO (G-3.8, Q-011). The PO made it public and approved the push and the release `v2.0.0-snapshot.1` (addendum 1 of the log): from Home Assistant 2026.11 the built-in Marketplace replaces HACS with the same repository format and installs the latest GitHub release.

## Amendment 2026-11-28 (PO) – Node-RED nodes `oec/integrations/node-red/`

PO order: "ich möchte in oi4-oec-demo nun noch einen Node-Red Node haben. Den du komplett erstellen sollst … Man kann über Config Nodes Apps anlegen. Dann gibt es verschiedene Nodes, die man verwenden zum publizieren oder auch um auf Get Anfragen zu reagieren … für die einzelne Ressources getrennt". Decisions of the agent (G-3.4 b; log 2026-11-28_PO_node-red-integration):

- **Source `oec/integrations/node-red/`**: the npm package `@oi4/node-red-oec` (JavaScript only, dependency `mqtt`), self-contained with its own `package-lock.json`, tests and `check.sh` (opt-in, not part of `oec/check.sh`: no Node.js test run of integrations in the gate; `OEC_D7=<oec-check>` adds the D7 live run). `TestReleaseVersion` pins `package.json` and `package-lock.json` to `oec/VERSION`.
- **Copy into `oi4-oec-demo`**: everything of `oec/integrations/node-red/` (tracked files, no `node_modules`) unmodified in `integrations/node-red/`. Changes are made in `oec/integrations` and copied again.
