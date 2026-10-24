# Project organization – OI4 OEC v2.0

## Goal

Deliver, step by step and traceable, a clean-room v2.0 implementation of the OI4 Open Edge Computing ecosystem:

| # | Deliverable | Short description |
|---|-------------|-------------------|
| D1 | JSON Schema | Validates every OEC message (topic + payload) as precisely as possible, based on the latest JSON Schema draft |
| D2 | SDK / library | Makes building OEC-compliant applications easy; hides protocol complexity |
| D3 | Demo: weather service | OT-connector-like demo publishing weather data; must work without a paid/registered API subscription |
| D4 | Demo: logger service | Consumes and logs messages from the Message Bus |
| D5 | OEC Registry | State-of-the-art registry listing all participants and what they support |
| D6 | Documentation | Docusaurus docs explaining SDK, JSON Schema, conformance tool and demos, simple and understandable |
| D7 | Conformance test tool | Command-line tool (Windows + macOS, Linux for CI) that checks messages and live OEC participants against the guideline; doubles as integration test harness |
| D8 | Infrastructure compliance tool | Command-line tool (Windows + macOS + Linux) that checks the Service Container Compliance of app containers and testable platform requirements (ADR-0014) |
| D9 | Micro SDK | SDK for microcontroller-class smart devices (Messaging Level 1) without container; C17, reference hardware ESP32 dev board + M5Stack (ADR-0016/0017) |

## Folder layout

```
AGENTS.md                 entry point for agents (workspace root)
project/
  README.md               this file
  GUARDRAILS.md           non-negotiable rules (protocol, engineering, process)
  ROADMAP.md              phases, deliverables, exit criteria
  STATUS.md               current state + next step (always kept up to date)
  OPEN-QUESTIONS.md       pending decisions with the product owner
  REVIEW.md               everything waiting for the PO: drafts to review, decisions (updated every cycle)
  decisions/              Architecture Decision Records (ADR-xxxx)
  log/                    one file per work cycle (chronological protocol)
  research/               digests and analyses (guideline digest, legacy analysis, ...)
oec/                      monorepo with all deployables D1–D9 (see oec/README.md, ADR-0012)
```

The workspace root is a **local git repository** (no remote). `legacy/` and `oi4-oec-documentation/` are separate repos and git-ignored.

Spec findings (ambiguities / contradictions found in the guideline, `SF-xxx`) are kept **only** in the guideline repo: `oi4-oec-documentation/findings/SPEC-FINDINGS.md` (PO decision, C-018; the former `project/SPEC-FINDINGS.md` was merged into it and deleted).

## Way of working: the cycle

Every unit of work is a **cycle** `C-NNN`. A cycle is small (hours, not weeks) and always follows:

1. **Plan** – write the intent, scope and acceptance criteria into a new log file `log/YYYY-MM-DD_C-NNN_<slug>.md`.
2. **Align** – design decisions, scope changes and anything listed in GUARDRAILS G-3.4 are confirmed by the product owner *before* implementation.
3. **Execute** – implement / research.
4. **Verify** – run tests / builds / validations; record the commands and results.
5. **Record** – complete the log file (what was done, why, results, deviations, follow-ups), update `STATUS.md` and `REVIEW.md` (open review items and decisions for the PO), add ADRs / spec findings / open questions where needed.

A cycle is only "done" when step 5 is complete. A different agent must be able to continue from `STATUS.md` + the latest log without additional context.

## Templates

### Cycle log (`log/YYYY-MM-DD_C-NNN_<slug>.md`)

```markdown
# C-NNN – <title>
- Date: YYYY-MM-DD
- Phase: P<n>
- Agent/Author: <name/model>
- Status: planned | in progress | done | aborted

## Intent
## Scope / acceptance criteria
## Inputs (sources read, with revisions)
## Work done
## Verification (commands + results)
## Decisions taken (link ADRs)
## Findings (link SF-xxx, Q-xxx)
## Follow-ups / next step
```

### ADR (`decisions/ADR-NNNN-<slug>.md`)

```markdown
# ADR-NNNN – <title>
- Status: proposed | accepted | superseded by ADR-xxxx
- Date: YYYY-MM-DD
- Deciders: <who>
## Context
## Decision
## Consequences
## Alternatives considered
```

## Numbering

- Cycles: `C-001`, `C-002`, ... (global, never reused)
- ADRs: `ADR-0001`, ...
- Spec findings: `SF-001`, ...
- Open questions: `Q-001`, ...
- Requirements (from Phase 1): `REQ-<area>-NNN` (e.g. `REQ-TOP-001`), kept as YAML in `oec/spec/requirements/` (ADR-0018)
