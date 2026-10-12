# C-005 – Conformance tool language; OPEN-QUESTIONS restructured

- Date: 2026-10-12
- Phase: P3
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Guideline baseline: `oi4-oec-documentation` @ `c85ce2a` (unchanged)

## Input from product owner

- Q-012: conformance tool in **Go** → ADR-0013.
- Q-011 (publication/CRA) stays open.
- Split the table in `OPEN-QUESTIONS.md` into two tables: open questions on top, decided ones below.

## Execution

- `OPEN-QUESTIONS.md` restructured: "Open" table (Q-011) and "Decided" table (Q-001…Q-010, Q-012) with decision, cycle and ADR reference columns; rule for moving rows added.
- ADR-0013 created; ADR-0011, ROADMAP (P3), STATUS, `oec/README.md` updated.
- Committed individually (one commit per logical change).

## Next

- PO confirms next phase order (P1 spec baseline → P2 JSON Schema + D7 `validate`), P3 remainder in parallel.
