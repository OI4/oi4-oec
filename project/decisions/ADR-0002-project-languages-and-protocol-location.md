# ADR-0002 – Project languages and protocol location

- Status: accepted
- Date: 2026-10-12
- Deciders: product owner (Thomas), confirmed in C-001

## Context

The product owner communicates in German; the guideline and the Docusaurus documentation are in English. Work must be traceable and continuable by other agents.

## Decision

- Communication with the product owner: German.
- All artifacts (logs, plans, ADRs, code, comments, docs): English.
- Planning and protocol artifacts live in `project/` at the workspace root, with `AGENTS.md` as entry point. They may move together with the code once Q-002 (repo layout) is decided.

## Consequences

- Any agent can pick up the work from `AGENTS.md` → `project/STATUS.md` → latest `project/log/` entry.
