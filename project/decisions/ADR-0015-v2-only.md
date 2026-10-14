# ADR-0015 – v2.0 only; v1.x traffic is ignored

- Status: accepted
- Date: 2026-10-14
- Deciders: product owner (Thomas), C-007
- Related: Q-014, GL-ADR-003, MG 8.1.1, G-2.1

## Decision

All deliverables implement **only guideline v2.0** (namespace `Oi4v2`):
- SDK
- micro SDK
- demos
- registry
- D7/D8

v1.x traffic (namespace `Oi4`) is **ignored**:
- not subscribed
- not detected
- not listed in the registry
- not reported by D7

## Consequences

- Subscriptions always use `Oi4v2/...` and never `#` on the broker root. As a side effect, v1 traffic on a shared broker is never seen.
- No migration or compatibility layer. The docs (P8) contain migration *notes* for developers, but no code support.
