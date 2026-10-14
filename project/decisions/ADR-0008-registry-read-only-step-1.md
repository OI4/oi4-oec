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

## Amendment C-008 (guideline baseline `7bead4c`)

The read-only registry also covers two new points from MG 7.3.4/7.3.5 and A1.

- **Availability:** the registry derives it from the lifecycle messages.

  | Message | Interpretation |
  |---|---|
  | Close (`NORMAL_0` / `HealthScore` 0 about the app itself) | offline, graceful |
  | Will (`FAILURE_1` / 0) | offline, ungraceful |
  | next Birth | online again |

- **Observed compliance level:** an informative *observed* Messaging compliance level per participant, derived from `Profile` and from the answers to `Get` requests. This needs only `Get` requests, which the read-only scope already allows.
