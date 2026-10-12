# ADR-0005 – `_ToDelete_` items are not part of v2.0

- Status: accepted
- Date: 2026-10-12
- Deciders: product owner (Thomas), C-002
- Related: SF-007, Q-003

## Context

The guideline draft still contains chapters prefixed `_ToDelete_`: resources `License`, `LicenseText`, `RtLicense`, `Interfaces`, `SBoM` and the NAMUR NE107 event category (`CAT_NE107_2`, 9.3.9.3), while other sections (8.2.1, 11.3, 8.1.7) still reference them.

## Decision

Everything marked `_ToDelete_` is dropped in v2.0 and ignored by all deliverables: not mandatory, not in the JSON Schema, not implemented in the SDK, not tested by the conformance tool, not documented (except in migration notes).

## Consequences

- Minimum compliance set: methods `Pub`, `Get`; resources `MAM`, `Health`, `Profile`, `PublicationList`, `ReferenceDesignation` (+ conditional rules).
- Event categories: `CAT_SYSLOG_0`, `CAT_STATUS_1`, `CAT_GENERIC_99`.
- Their DataSetClassIds remain reserved (not reused).
- Remaining references in the guideline are reported to the work group (SF-007).
