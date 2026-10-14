# C-007 – Compliance tools, v2 only, level targets, micro SDK

- Date: 2026-10-14
- Phase: P3
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Guideline baseline: `oi4-oec-documentation` @ `5f070bb` (unchanged)

## Input from product owner

- **Q-013:** best as two different tools – one for application compliance, one for infrastructure compliance.
- **Q-014:** only v2; v1.x can be ignored.
- **Q-015:** as proposed – the SDK supports all levels; an additional micro SDK for microcontrollers; demos and registry aim for L3, but one demo service shall be L1 and the other L3.

## Decisions

- **ADR-0014: two compliance tools.**
  - D7 = application (Messaging) compliance.
  - New **D8** = infrastructure compliance (Service Container + testable platform requirements).
  - Both in Go, in one module `oec/conformance/`, as separate binaries.
  - Amends ADR-0006, ADR-0013 and ADR-0012.
- **ADR-0015: v2 only.** v1 traffic is ignored everywhere, including the registry. This drops the earlier proposal to detect v1 participants.
- **ADR-0016: level targets and micro SDK.**
  - SDK supports L1–L3; registry targets L3.
  - **Weather = L3, logger = L1** (agent's choice, rationale in the ADR).
  - All containers target Service Container L3.
  - New **D9 micro SDK** in `oec/sdk/micro/`.
  - `oec-core` must be `no_std`-compatible (ADR-0011 amended).

## Assumptions taken (autopilot, PO to confirm)

- "Infrastructure compliance" covers Service Container compliance of app containers **and** testable platform requirements (RIG 5.1). The first increment is offline OCI image inspection. Recorded as **Q-017**.
- Logger at L1 means it is configured via file/environment; it does not offer the `Config` resource, because L1 requires no Get/Set.

## Changes

- New ADR-0014, ADR-0015, ADR-0016; amendments to ADR-0006, 0011, 0012 and 0013.
- **OPEN-QUESTIONS:** Q-013…015 moved to "Decided"; new Q-016 (micro SDK language/hardware) and Q-017 (D8 scope).
- **ROADMAP:** new phases P4b (micro SDK) and P5b (D8: `inspect-image` / `inspect-container` / `inspect-platform`); level targets in P6/P7; CI and docs updated.
- **GUARDRAILS:** G-2.1 (v2 only), G-4.6 (micro SDK), G-4.12 (verified by D7 + D8), G-4.13 (D8 cross-platform).
- **Other files:** `project/README.md` and `oec/README.md` (D8, D9, folders), `AGENTS.md`, STATUS.

## Next

- PO answers Q-016 and confirms Q-017.
- Start P1: requirements catalog for MG + RIG, tagged by domain and level.
