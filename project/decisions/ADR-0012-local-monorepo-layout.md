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
