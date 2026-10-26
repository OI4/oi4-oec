# Review and decisions – open items for the PO

_Last updated: 2026-10-26, after C-045 (guideline `ccb475a`, register `13d6b1a`); P3 closed._

This is the single list of everything that waits for the PO:
- items to **review**: drafts, and changes to reviewed items;
- items to **decide**: spec findings, open questions, cycle plan.

Each cycle updates it. Items leave the list once the PO has answered and `spectool review` (or the decision record) has been applied. Details are in the cycle logs; this file links to them.

A German explanation of every item in plain language is in [`REVIEW_DE.md`](REVIEW_DE.md), with the same numbering.

**How to answer:** tick the box or write the decision into the "PO" field. You can also answer in the chat, e.g. "R-1 … R-4 akzeptiert, D-1 und D-2 wie empfohlen".

**Current state:** C-045 recorded your answers to C-042 … C-044 (all accepted as recommended). It:
- set ADR-0020 … 0025 to `accepted` and amended ADR-0017;
- moved Q-021 … Q-030 to "Decided";
- implemented SF-101/102 in the guideline (`df38e72`, `ccb475a`) and re-baselined (1401 inventory items);
- moved the schema bundle to the pre-release `2.0.1-dev` (no schema content change);
- closed P3 in ROADMAP/STATUS and refined P4 … P6.

The re-baseline produced three draft REQs and one agent deviation from the accepted wording (R-4). Implementing SF-101 also exposed an editorial gap in the guideline examples (SF-103, D-1). Coverage stays at 0 gaps; the drafts are not `schema`/`d7-validate` REQs.

Log: [C-045](log/2026-10-26_C-045_p3-exit.md).

## Overview

| # | Item | Kind | Source | Recommendation | PO |
|---|------|------|--------|----------------|----|
| D-1 | SF-103: guideline examples without the combined `ON_REQUEST_…` values | decide | C-045 | change the 13 examples to `ON_REQUEST_APPLICATION_SOURCE_FILTER_15` | ☐ |
| D-2 | Order of the next phases (P4, P4b, P5, P5b) | decide | C-045 | P4 first (critical path to P6/P7), P5 interleaved; P4b/P5b afterwards | ☐ |
| R-1 | REQ-RES-PUBLIST-026 (SHOULD combined values, `manual`) | review | C-045 | accept | ☐ |
| R-2 | REQ-RES-PUBLIST-027 (`Get` with every `Mode` except `OFF_0`, `d7-test`) | review | C-045 | accept | ☐ |
| R-3 | REQ-TOP-028 (smart device → `OTConnector`) and REQ-TOP-007 `replaced_by` | review | C-045 | accept | ☐ |
| R-4 | Guideline wording of SF-101 (scoped by the agent) and SF-102 | review | C-045 | accept | ☐ |

## Decisions

### D-1 – SF-103 examples without the combined `ON_REQUEST_…` values (open, register `13d6b1a`)

- **Where:** `oi4-oec-documentation/findings/SPEC-FINDINGS.md`, SF-103 (MG B1.1.8, B2.6).
- **Problem:** MG 8.3.8 now recommends the combined values for requestable resources (SF-101). The PublicationList examples do not follow it: 12 entries in B1.1.8 (`Data`, `MAM`, `Health`, `Profile`, `ReferenceDesignation`) and 1 in B2.6 use `APPLICATION_SOURCE_FILTER_8`. The `Event` entries with `FILTER_4` are correct.
- **Not a protocol defect:** both values allow `Get`; the recommendation is a SHOULD.
- **Options:**
  - (a) change the 13 entries to `ON_REQUEST_APPLICATION_SOURCE_FILTER_15` and keep `FILTER_4` for `Event` – **recommended**: examples are what implementers copy.
  - (b) keep the examples (they show the equivalent short form).
- **Effect:** with (a) the agent changes the guideline on your order and re-baselines (no new REQ expected; the example vectors stay valid either way, their values can follow the examples). With (a) a D7 `monitor` warning for REQ-RES-PUBLIST-026 becomes a candidate for P5.
- **PO:** ☐ (a) ☐ (b)

### D-2 – Order of the next phases

P3 is closed. P4 (D2), P4b (D9), P5 (D7 `monitor`/`test`) and P5b (D8) can run in parallel by dependency, but the agent works one cycle at a time.

- **Critical path:** P4 → P6 (demos, logger in C needs P4 increment 10) → P7 (registry) → P9. D7 `test` (P5 exit) also needs an SDK example app.
- **Recommendation:**
  1. P4 first (C-046 = P4 plan + increments 1–3: identity, topic model, message model; all offline, no container engine needed).
  2. P5 interleaved: `internal/mqttwire` + capture + `monitor --replay` (offline), then `monitor` live with the dev broker of P4 increment 9.
  3. P4b and P5b afterwards, or earlier if you want the ESP32 demo or the container checks sooner.
- **Prerequisite from P4 increment 5/9 on:** a running container engine (Docker Desktop was not running on 2026-10-14; STATUS "Baselines").
- **PO:** ☐ as recommended ☐ other order: …

## Review items

### R-1 – REQ-RES-PUBLIST-026 (`draft`)

- **Rule:** for an entry of a resource that can be requested with `Get`, a publisher that enables publishing modes uses the value combined with `ON_REQUEST_1` (e.g. `ON_REQUEST_APPLICATION_SOURCE_FILTER_15` instead of `APPLICATION_SOURCE_FILTER_8`).
- SHOULD at L1–L3, `applies_to [application]`, inventory MG-8.3.8-b1ebc429.
- **Verification `manual`:** both values are valid, so schema and D7 `validate` accept both. A D7 `monitor` warning is a candidate after D-1.
- **PO:** ☐ accepted

### R-2 – REQ-RES-PUBLIST-027 (`draft`)

- **Rule:** the `DataSetMessage` of a resource that can be requested with `Get` can be requested with every `Mode` except `OFF_0`.
- Implicit MUST (SF-035; the guideline sentence says "can"), inventory MG-8.3.8-dd632f21.
- **Verification `d7-test`:** added to catalog test T7-GET-03.
- **PO:** ☐ accepted

### R-3 – REQ-TOP-028 (`draft`) and REQ-TOP-007

- **Rule:** a smart device connected directly to the Message Bus uses the `ServiceType` `OTConnector` (MG 7.2 since `ccb475a`).
- Implicit MUST (SF-035), `applies_to [device]`, `manual` (D7 cannot tell a device from an application). No inventory item: the guideline sentence has no keyword.
- **REQ-TOP-007** (withdrawn by you in C-020 because the guideline had no such rule) stays `deprecated` and gets `replaced_by: REQ-TOP-028`. Ids are never reused.
- **PO:** ☐ accepted

### R-4 – Guideline wording of SF-101 and SF-102

- **SF-101, MG 8.3.8 (`df38e72`):** "For resources that can be requested with `Get` (9.1 Resources), the `DataSetMessage` can be requested with `Get` with every `Mode` except `OFF_0`. For these resources, the values combining `ON_REQUEST_1` with publishing modes are equivalent to the publishing modes alone; publishers should use the combined values to make the availability on request explicit."
  - **Agent deviation:** the accepted proposal had no scope ("every `Mode` except `OFF_0` allows `Get`"). MG 8.3.8 also says that `Event` entries cannot be requested, and MG 9.1 lists the requestable resources. The unscoped sentence would contradict both, so the agent added "For resources that can be requested with `Get`".
- **SF-102, MG 7.2 (`ccb475a`):** "A smart device connected directly to the Message Bus (1.3 Compliance Domains and Levels) uses the `ServiceType` `OTConnector`."
- **If not accepted:** the agent changes the text on your order and re-baselines again.
- **PO:** ☐ accepted

## After your answers (C-046)

1. `spectool review REQ-RES-PUBLIST-026 REQ-RES-PUBLIST-027 REQ-TOP-028` for the accepted REQs.
2. D-1 (a): change the 13 examples, register, re-baseline (schema stays `2.0.1-dev`).
3. Start the phase chosen in D-2 with its planning cycle.
