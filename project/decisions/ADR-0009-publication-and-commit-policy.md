# ADR-0009 – Publication on GitHub and commit policy

- Status: accepted, **amended by ADR-0012** (C-004): local git repository now, publication later after CRA review (Q-011); single-change commits apply immediately
- Date: 2026-10-12
- Deciders: product owner (Thomas), C-003
- Related: Q-007, Q-002

## Decision

- All artifacts (including `AGENTS.md` and `project/`) are published on **GitHub** as soon as Q-001 (language) and Q-002 (repo layout) are decided.
- From then on, **every change is committed individually** (one logical change per commit, no batch commits), with a meaningful message and a reference to the cycle (`C-NNN`) and, where applicable, REQ/SF/ADR ids.
- Until publication, work happens in the local workspace; cycle logs remain the record.

## Consequences

- The initial commit will contain the state of `project/` at publication time; history before that is documented in the cycle logs.
- Commit conventions (e.g. Conventional Commits, branch model, PR flow) are fixed when the repository is set up.
