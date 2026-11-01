# C-079 – P6 start: demo architecture (ADR-0028), increment 1 logger core in C

- **Date:** 2026-11-01
- **Phase:** P6 – Demo services (D3, D4, D10), increment 1 of 7 (ADR-0028, A-334)
- **Baseline:** unchanged since C-074 review addendum 2 – guideline `52d8470` (branch `feature/spec-findings`, head `eb821e9`), SF-001 … 117 resolved, no open finding; 1411 inventory items, 444 REQs, 0 gaps, no REQ drafts; schema 2.0.0 frozen, `2.0.1-dev`; 55 D7 and 17 D8 tests `implemented`, report versions 1 frozen
- **Agent:** new session (hand-over prompt of C-078 addendum 1)

## Intent

Start P6 after the PO closed P4c (C-078 addendum 1, D-1 (a)): confirm the cycle cut of A-306/A-307 and sharpen the exit, record the demo architecture as ADR-0028 and build increment 1 – the logger core D4 in C on `oec-ffi`: subscriptions, structured output, an own `Config` at Messaging Level 2, a history ring buffer; tests first (C unit tests under ASan/UBSan, integration tests against the in-process TLS test broker) and D7 `test --level 2` against the logger in the gate. No Web UI yet (increment 5). Nothing was open for the PO; no answer arrived.

## Plan

1. Baseline and gate check (green at the start: `oec/check.sh` "all checks passed").
2. ADR-0028, ROADMAP P6 (cut confirmed with cycles C-079 … C-085, exit sharpened).
3. Logger: C unit tests and Go integration tests first, then the implementation in `oec/demos/logger`, `oec/demos/check.sh` as a step of `oec/check.sh`, D7 `level2` in the gate.
4. README, STATUS, REVIEW/REVIEW_DE, hand-over.

## Work

_(in progress)_
