# ADR-0008 – Registry is read-only in step 1

- Status: accepted
- Date: 2026-10-12
- Deciders: product owner (Thomas), C-003
- Related: Q-006

## Decision

The first version of the OEC Registry (D5) is a **read-only observer**: it discovers participants, sends `Get` requests and provides its own `Get/MAM` and `Get/Health` answers (Appendix A1), but offers no UI/API actions that send `Set`, `Del` or `Call` to other participants.

## Consequences

- No write-side authorization concept needed in step 1.
- The architecture should not block later management actions (e.g. Config, PublicationList, ReferenceDesignation editing).
