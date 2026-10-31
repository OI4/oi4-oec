# D7 catalog coverage

Generated from `catalog/d7.yaml`, the requirements catalog of `oec/spec` (baseline `7427a4f`) and the indexes of the
self-test fixtures (`testdata/sim/index.yaml`, `testdata/captures/index.yaml`) – do not edit; regenerate with
`go test ./internal/explain -run TestCoverageReport -update` (the gate fails if this file is out of date).
`oec-check list` and `oec-check explain` show the same data.

## Summary

| Tests | Count |
|---|---:|
| all | 55 |
| status `planned` | 0 |
| status `implemented` | 55 |
| mode `monitor` | 19 |
| mode `test` | 36 |
| needs `proxy` | 7 |
| needs `restart-hook` | 2 |
| needs `peer` | 1 |
| needs `none` | 45 |

| Active requirements by verification kind | REQs | checked by a test | `manual` | by `oec-check validate` | not covered |
|---|---:|---:|---:|---:|---:|
| `d7-monitor` | 47 | 47 | 0 | 0 | 0 |
| `d7-test` | 122 | 118 | 4 | 0 | 0 |
| `schema` | 116 | 0 | 0 | 116 | 0 |
| `d7-validate` | 157 | 0 | 0 | 157 | 0 |
| none of these (D8, manual or other tools) | 143 | | | | |

Fixtures: 59 sim, 29 capture; 14 positive; 88 of 88 reviewed.
Tests with a negative fixture (fail, warn or info): 55 of 55; with a positive fixture: 55 of 55.

## Tests

Fixtures by the outcome their index states for the test: `fail`; `warn`/`info` (the verdict of a SHOULD or a
heuristic rule); `skip`; `positive` – positive fixtures that run the test without stating an outcome (pass or not
applicable). A negative fixture is one with fail, warn or info.

| Test | Mode | Level | Needs | Status | Requirements | fail | warn/info | skip | positive |
|---|---|---:|---|---|---|---:|---:|---:|---:|
| T7-MON-01 | monitor | 1 | – | implemented | REQ-TOP-002 REQ-TOP-003 | 5 | 0 | 0 | 12 |
| T7-MON-02 | monitor | 1 | – | implemented | REQ-TOP-006 REQ-TOP-010 | 1 | 0 | 0 | 12 |
| T7-MON-03 | monitor | 1 | – | implemented | REQ-CMP-003 REQ-LIFE-001 REQ-LIFE-002 REQ-LIFE-003 | 4 | 0 | 0 | 12 |
| T7-MON-04 | monitor | 1 | – | implemented | REQ-CMP-006 REQ-CMP-007 REQ-CMP-014 REQ-CMP-045 | 1 | 0 | 0 | 12 |
| T7-MON-05 | monitor | 1 | – | implemented | REQ-TOP-017 REQ-RES-HEALTH-004 REQ-LIFE-014 | 3 | 0 | 0 | 12 |
| T7-MON-06 | monitor | 1 | – | implemented | REQ-MSG-008 REQ-ACT-004 | 1 | 0 | 0 | 12 |
| T7-MON-07 | monitor | 1 | – | implemented | REQ-ACT-011 | 1 | 0 | 0 | 12 |
| T7-MON-08 | monitor | 1 | – | implemented | REQ-MSG-021 REQ-MSG-033 REQ-RES-PUBLIST-005 | 4 | 0 | 0 | 12 |
| T7-MON-09 | monitor | 1 | – | implemented | REQ-MSG-013 | 1 | 0 | 0 | 12 |
| T7-MON-10 | monitor | 1 | – | implemented | REQ-IL-010 REQ-IL-012 | 0 | 4 | 0 | 12 |
| T7-MON-11 | monitor | 1 | – | implemented | REQ-CMP-042 | 1 | 0 | 0 | 12 |
| T7-MON-12 | monitor | 1 | – | implemented | REQ-CMP-009 REQ-MQTT-017 | 1 | 0 | 0 | 12 |
| T7-MON-13 | monitor | 2 | – | implemented | REQ-CMP-010 REQ-CMP-036 REQ-CMP-038 | 1 | 0 | 0 | 12 |
| T7-MON-14 | monitor | 1 | – | implemented | REQ-MQTT-019 REQ-RES-PUBLIST-008 REQ-RES-PUBLIST-009 REQ-RES-PUBLIST-010 REQ-RES-PUBLIST-011 REQ-RES-PUBLIST-013 REQ-RES-PUBLIST-014 REQ-RES-PUBLIST-015 | 3 | 1 | 0 | 12 |
| T7-MON-15 | monitor | 1 | – | implemented | REQ-RES-PUBLIST-017 REQ-RES-PUBLIST-018 REQ-RES-SUBLIST-005 REQ-RES-SUBLIST-006 | 1 | 0 | 0 | 12 |
| T7-MON-16 | monitor | 1 | – | implemented | REQ-RES-MAM-018 | 1 | 0 | 0 | 12 |
| T7-MON-17 | monitor | 1 | – | implemented | REQ-RES-CONFIG-011 | 1 | 0 | 0 | 12 |
| T7-MON-18 | monitor | 1 | – | implemented | REQ-PAG-004 REQ-PAG-005 | 2 | 0 | 0 | 12 |
| T7-MON-19 | monitor | 1 | – | implemented | REQ-TOP-024 | 1 | 0 | 0 | 12 |
| T7-CON-01 | test | 1 | proxy | implemented | REQ-CMP-001 REQ-CMP-009 REQ-MQTT-001 | 2 | 0 | 0 | 8 |
| T7-CON-02 | test | 1 | proxy | implemented | REQ-CMP-002 REQ-MQTT-005 REQ-MQTT-008 | 2 | 0 | 0 | 8 |
| T7-CON-03 | test | 1 | proxy | implemented | REQ-MQTT-009 | 1 | 0 | 0 | 8 |
| T7-CON-04 | test | 1 | proxy | implemented | REQ-MQTT-018 | 1 | 0 | 0 | 8 |
| T7-LIFE-01 | test | 1 | proxy | implemented | REQ-CMP-003 REQ-LIFE-008 REQ-LIFE-010 REQ-LIFE-016 | 2 | 1 | 0 | 8 |
| T7-LIFE-02 | test | 1 | proxy | implemented | REQ-LIFE-001 REQ-LIFE-015 REQ-LIFE-017 | 2 | 1 | 0 | 8 |
| T7-LIFE-03 | test | 1 | proxy | implemented | REQ-LIFE-009 | 1 | 0 | 0 | 8 |
| T7-LIFE-04 | test | 1 | restart-hook | implemented | REQ-CMP-046 REQ-LIFE-005 REQ-LIFE-006 REQ-LIFE-007 | 2 | 0 | 1 | 8 |
| T7-ADR-01 | test | 1 | peer | implemented | REQ-BUS-002 REQ-TOP-006 REQ-TOP-010 | 2 | 0 | 1 | 1 |
| T7-ADR-02 | test | 1 | – | implemented | REQ-TOP-004 | 1 | 0 | 0 | 6 |
| T7-ADR-03 | test | 1 | – | implemented | REQ-IL-001 REQ-IL-013 REQ-IL-015 REQ-TOP-018 | 1 | 0 | 0 | 6 |
| T7-GET-01 | test | 1 | – | implemented | REQ-BUS-003 REQ-CMP-015 REQ-CMP-017 REQ-MSG-015 REQ-RES-HEALTH-006 REQ-RES-MAM-021 | 2 | 0 | 0 | 6 |
| T7-GET-02 | test | 1 | – | implemented | REQ-CMP-018 REQ-CMP-021 REQ-RES-PROFILE-002 REQ-RES-PROFILE-004 | 3 | 0 | 0 | 6 |
| T7-GET-03 | test | 1 | – | implemented | REQ-CMP-008 REQ-CMP-019 REQ-CMP-020 REQ-CMP-035 REQ-CMP-036 REQ-CMP-037 REQ-CMP-039 REQ-RES-CONFIG-017 REQ-RES-DATA-007 REQ-RES-META-002 REQ-RES-PUBLIST-020 REQ-RES-PUBLIST-027 REQ-RES-REFDES-004 REQ-RES-SUBLIST-008 | 4 | 0 | 0 | 6 |
| T7-GET-04 | test | 1 | – | implemented | REQ-CMP-010 REQ-CMP-024 REQ-CMP-045 | 3 | 0 | 0 | 6 |
| T7-GET-05 | test | 1 | – | implemented | REQ-CMP-041 REQ-CMP-044 REQ-CMP-047 | 1 | 2 | 0 | 6 |
| T7-GET-06 | test | 1 | – | implemented | REQ-PAG-002 REQ-PAG-005 | 1 | 0 | 0 | 6 |
| T7-GET-07 | test | 1 | – | implemented | REQ-QRY-001 REQ-QRY-002 REQ-QRY-003 REQ-RES-REFDES-011 REQ-RES-REFDES-013 REQ-RES-REFDES-014 | 2 | 0 | 0 | 6 |
| T7-GET-08 | test | 1 | – | implemented | REQ-LOC-002 | 1 | 0 | 0 | 6 |
| T7-GET-09 | test | 1 | – | implemented | REQ-MSG-046 | 0 | 1 | 0 | 6 |
| T7-SET-01 | test | 1 | – | implemented | REQ-BUS-006 REQ-CMP-016 REQ-CMP-023 REQ-CMP-038 REQ-MQTT-019 REQ-MSG-015 REQ-RES-EVENT-005 REQ-RES-SUBLIST-016 REQ-TOP-014 REQ-TOP-015 | 2 | 0 | 0 | 6 |
| T7-SET-02 | test | 1 | – | implemented | REQ-CMP-043 | 1 | 0 | 0 | 6 |
| T7-SET-03 | test | 3 | – | implemented | REQ-CMP-033 | 3 | 0 | 0 | 6 |
| T7-SET-04 | test | 1 | – | implemented | REQ-BUS-007 REQ-BUS-008 | 1 | 0 | 0 | 6 |
| T7-SET-05 | test | 1 | – | implemented | REQ-RES-CONFIG-005 REQ-RES-CONFIG-021 REQ-RES-CONFIG-022 REQ-RES-CONFIG-023 REQ-RES-CONFIG-024 REQ-RES-CONFIG-025 | 1 | 0 | 0 | 6 |
| T7-SET-06 | test | 1 | – | implemented | REQ-RES-DATA-011 REQ-RES-DATA-012 | 1 | 0 | 0 | 6 |
| T7-SET-07 | test | 1 | – | implemented | REQ-RES-META-006 REQ-RES-META-007 | 2 | 0 | 0 | 6 |
| T7-SET-08 | test | 1 | – | implemented | REQ-RES-PUBLIST-007 REQ-RES-PUBLIST-016 REQ-RES-PUBLIST-024 REQ-RES-PUBLIST-025 | 1 | 0 | 0 | 6 |
| T7-SET-09 | test | 1 | – | implemented | REQ-RES-REFDES-009 REQ-RES-REFDES-010 | 1 | 0 | 0 | 6 |
| T7-SET-10 | test | 1 | restart-hook | implemented | REQ-RES-REFDES-002 | 1 | 0 | 0 | 6 |
| T7-SET-11 | test | 1 | – | implemented | REQ-RES-SUBLIST-004 REQ-RES-SUBLIST-013 REQ-RES-SUBLIST-014 REQ-RES-SUBLIST-015 | 1 | 0 | 0 | 6 |
| T7-SET-12 | test | 1 | – | implemented | REQ-TOP-027 | 0 | 1 | 0 | 6 |
| T7-ACT-01 | test | 1 | – | implemented | REQ-ACT-001 REQ-ACT-006 REQ-ACT-007 REQ-ACT-012 REQ-ACT-014 REQ-ACT-015 REQ-ACT-016 REQ-ACT-020 REQ-ACT-022 REQ-CMP-034 REQ-CMP-040 REQ-TOP-016 | 2 | 0 | 0 | 6 |
| T7-ACT-02 | test | 1 | – | implemented | REQ-ACT-023 | 1 | 0 | 0 | 6 |
| T7-ACT-03 | test | 1 | – | implemented | REQ-ACT-017 | 1 | 0 | 0 | 6 |
| T7-ACT-04 | test | 1 | – | implemented | REQ-ACT-024 | 1 | 0 | 0 | 6 |

## Manual requirements

| Requirement | Reason |
|---|---|
| REQ-CMP-011 | A rule for D7 itself, not a test – `--level` defaults to 3 when the manufacturer states no level. |
| REQ-LIFE-011 | Subscriber side – how a CUT interprets a Close of another component is not observable on the bus in general. |
| REQ-LIFE-012 | Subscriber side – as REQ-LIFE-011 for the Will. |
| REQ-LIFE-013 | Subscriber side – as REQ-LIFE-011 for the next Birth. |

## Requirements checked by tests

| Requirement | Keyword | Verification | Tests | Fixtures with a stated outcome |
|---|---|---|---|---:|
| REQ-ACT-001 | MUST | schema, d7-validate, d7-test | T7-ACT-01 | 0 |
| REQ-ACT-004 | MUST | schema, d7-validate, d7-monitor | T7-MON-06 | 0 |
| REQ-ACT-006 | MUST | schema, d7-validate, d7-test | T7-ACT-01 | 0 |
| REQ-ACT-007 | MUST | d7-test | T7-ACT-01 | 1 |
| REQ-ACT-011 | MUST | d7-monitor | T7-MON-07 | 1 |
| REQ-ACT-012 | MUST | schema, d7-validate, d7-test | T7-ACT-01 | 0 |
| REQ-ACT-014 | MUST | schema, d7-validate, d7-test | T7-ACT-01 | 0 |
| REQ-ACT-015 | MUST | d7-test | T7-ACT-01 | 0 |
| REQ-ACT-016 | MAY | d7-test | T7-ACT-01 | 0 |
| REQ-ACT-017 | MUST NOT | d7-test | T7-ACT-03 | 1 |
| REQ-ACT-020 | MUST | d7-validate, d7-test | T7-ACT-01 | 0 |
| REQ-ACT-022 | MUST | schema, d7-validate, d7-test | T7-ACT-01 | 1 |
| REQ-ACT-023 | MUST | d7-test | T7-ACT-02 | 1 |
| REQ-ACT-024 | MUST | d7-test | T7-ACT-04 | 1 |
| REQ-BUS-002 | MUST | d7-test | T7-ADR-01 | 3 |
| REQ-BUS-003 | MUST | d7-test | T7-GET-01 | 1 |
| REQ-BUS-006 | MUST | d7-test | T7-SET-01 | 1 |
| REQ-BUS-007 | MUST NOT | d7-test | T7-SET-04 | 1 |
| REQ-BUS-008 | MUST | d7-test | T7-SET-04 | 0 |
| REQ-CMP-001 | MUST | d7-test | T7-CON-01 | 1 |
| REQ-CMP-002 | MUST | d7-test | T7-CON-02 | 2 |
| REQ-CMP-003 | MUST | d7-monitor, d7-test | T7-MON-03 T7-LIFE-01 | 3 |
| REQ-CMP-006 | MUST | d7-monitor | T7-MON-04 | 0 |
| REQ-CMP-007 | MUST | d7-monitor | T7-MON-04 | 1 |
| REQ-CMP-008 | MUST | d7-test | T7-GET-03 | 2 |
| REQ-CMP-009 | MUST | d7-test, d7-monitor | T7-MON-12 T7-CON-01 | 2 |
| REQ-CMP-010 | MUST | d7-test, d7-monitor | T7-MON-13 T7-GET-04 | 4 |
| REQ-CMP-014 | MUST | d7-monitor | T7-MON-04 | 0 |
| REQ-CMP-015 | MUST | d7-test | T7-GET-01 | 1 |
| REQ-CMP-016 | MUST | d7-test | T7-SET-01 | 0 |
| REQ-CMP-017 | MUST | d7-test | T7-GET-01 | 1 |
| REQ-CMP-018 | MUST | d7-test | T7-GET-02 | 1 |
| REQ-CMP-019 | MUST | d7-test | T7-GET-03 | 1 |
| REQ-CMP-020 | MUST | d7-test | T7-GET-03 | 1 |
| REQ-CMP-021 | MUST | d7-test | T7-GET-02 | 1 |
| REQ-CMP-023 | MUST | d7-test | T7-SET-01 | 0 |
| REQ-CMP-024 | MUST | d7-test | T7-GET-04 | 2 |
| REQ-CMP-033 | MUST | d7-test | T7-SET-03 | 3 |
| REQ-CMP-034 | MUST | d7-test | T7-ACT-01 | 0 |
| REQ-CMP-035 | MUST | d7-test | T7-GET-03 | 1 |
| REQ-CMP-036 | MUST | d7-test, d7-monitor | T7-MON-13 T7-GET-03 | 2 |
| REQ-CMP-037 | MUST | d7-test | T7-GET-03 | 1 |
| REQ-CMP-038 | MUST | d7-test, d7-monitor | T7-MON-13 T7-SET-01 | 1 |
| REQ-CMP-039 | MUST | d7-test | T7-GET-03 | 2 |
| REQ-CMP-040 | MUST | d7-test | T7-ACT-01 | 0 |
| REQ-CMP-041 | MUST | d7-test | T7-GET-05 | 1 |
| REQ-CMP-042 | MUST NOT | d7-monitor | T7-MON-11 | 1 |
| REQ-CMP-043 | MUST | d7-test | T7-SET-02 | 1 |
| REQ-CMP-044 | MAY | d7-test | T7-GET-05 | 0 |
| REQ-CMP-045 | MUST | d7-test, d7-monitor | T7-MON-04 T7-GET-04 | 1 |
| REQ-CMP-046 | MUST | d7-test | T7-LIFE-04 | 3 |
| REQ-CMP-047 | SHOULD | d7-test | T7-GET-05 | 2 |
| REQ-IL-001 | MUST | d7-validate, d7-test | T7-ADR-03 | 0 |
| REQ-IL-010 | MUST | d7-monitor, manual | T7-MON-10 | 1 |
| REQ-IL-012 | MUST | d7-monitor, manual | T7-MON-10 | 4 |
| REQ-IL-013 | MUST | d7-test, manual | T7-ADR-03 | 0 |
| REQ-IL-015 | MUST | d7-test | T7-ADR-03 | 1 |
| REQ-LIFE-001 | MUST | d7-monitor, d7-test | T7-MON-03 T7-LIFE-02 | 3 |
| REQ-LIFE-002 | MUST | d7-monitor | T7-MON-03 | 0 |
| REQ-LIFE-003 | MUST | d7-validate, d7-monitor | T7-MON-03 | 2 |
| REQ-LIFE-005 | MUST | d7-test | T7-LIFE-04 | 2 |
| REQ-LIFE-006 | MUST | d7-test | T7-LIFE-04 | 1 |
| REQ-LIFE-007 | MUST | d7-validate, d7-test | T7-LIFE-04 | 2 |
| REQ-LIFE-008 | MUST | d7-test | T7-LIFE-01 | 1 |
| REQ-LIFE-009 | MUST | d7-validate, d7-test | T7-LIFE-03 | 1 |
| REQ-LIFE-010 | SHOULD NOT | d7-test | T7-LIFE-01 | 1 |
| REQ-LIFE-014 | MUST NOT | d7-monitor | T7-MON-05 | 1 |
| REQ-LIFE-015 | SHOULD | d7-test | T7-LIFE-02 | 1 |
| REQ-LIFE-016 | MUST | d7-test | T7-LIFE-01 | 1 |
| REQ-LIFE-017 | MUST | d7-test, manual | T7-LIFE-02 | 1 |
| REQ-LOC-002 | MUST | d7-test | T7-GET-08 | 1 |
| REQ-MQTT-001 | MUST | manual, d7-test | T7-CON-01 | 1 |
| REQ-MQTT-005 | MUST | d7-test, manual | T7-CON-02 | 1 |
| REQ-MQTT-008 | MUST | manual, d7-test | T7-CON-02 | 1 |
| REQ-MQTT-009 | MUST | d7-test, manual | T7-CON-03 | 1 |
| REQ-MQTT-017 | SHOULD | d7-monitor | T7-MON-12 | 0 |
| REQ-MQTT-018 | MUST NOT | d7-test | T7-CON-04 | 1 |
| REQ-MQTT-019 | MUST | d7-monitor, d7-test | T7-MON-14 T7-SET-01 | 2 |
| REQ-MSG-008 | MUST | d7-monitor | T7-MON-06 | 1 |
| REQ-MSG-013 | MUST | d7-monitor, manual | T7-MON-09 | 1 |
| REQ-MSG-015 | MUST | schema, d7-test | T7-GET-01 T7-SET-01 | 1 |
| REQ-MSG-021 | MUST | d7-monitor | T7-MON-08 | 4 |
| REQ-MSG-033 | MUST | d7-monitor | T7-MON-08 | 1 |
| REQ-MSG-046 | SHOULD | d7-test | T7-GET-09 | 1 |
| REQ-PAG-002 | MUST | d7-test | T7-GET-06 | 1 |
| REQ-PAG-004 | MUST | d7-validate, d7-monitor | T7-MON-18 | 1 |
| REQ-PAG-005 | MUST | d7-monitor, d7-test | T7-MON-18 T7-GET-06 | 3 |
| REQ-QRY-001 | MUST | schema, d7-validate, d7-test | T7-GET-07 | 0 |
| REQ-QRY-002 | MUST | d7-test | T7-GET-07 | 0 |
| REQ-QRY-003 | MUST | d7-test | T7-GET-07 | 0 |
| REQ-RES-CONFIG-005 | MUST NOT | d7-test | T7-SET-05 | 0 |
| REQ-RES-CONFIG-011 | MUST | d7-monitor | T7-MON-17 | 1 |
| REQ-RES-CONFIG-017 | MUST | d7-test | T7-GET-03 | 0 |
| REQ-RES-CONFIG-021 | MUST | d7-test | T7-SET-05 | 0 |
| REQ-RES-CONFIG-022 | MUST | d7-test | T7-SET-05 | 0 |
| REQ-RES-CONFIG-023 | MUST | d7-test | T7-SET-05 | 1 |
| REQ-RES-CONFIG-024 | MUST | schema, d7-validate, d7-test | T7-SET-05 | 0 |
| REQ-RES-CONFIG-025 | MUST | d7-test | T7-SET-05 | 0 |
| REQ-RES-DATA-007 | MUST | d7-test | T7-GET-03 | 0 |
| REQ-RES-DATA-011 | MUST | d7-test | T7-SET-06 | 0 |
| REQ-RES-DATA-012 | MUST | d7-test | T7-SET-06 | 1 |
| REQ-RES-EVENT-005 | MUST | d7-test | T7-SET-01 | 0 |
| REQ-RES-HEALTH-004 | MUST | d7-monitor | T7-MON-05 | 0 |
| REQ-RES-HEALTH-006 | MUST | d7-test | T7-GET-01 | 1 |
| REQ-RES-MAM-018 | MUST | d7-monitor | T7-MON-16 | 1 |
| REQ-RES-MAM-021 | MUST | d7-test | T7-GET-01 | 0 |
| REQ-RES-META-002 | MUST | d7-test | T7-GET-03 | 1 |
| REQ-RES-META-006 | MUST | d7-test | T7-SET-07 | 2 |
| REQ-RES-META-007 | MUST | d7-test | T7-SET-07 | 0 |
| REQ-RES-PROFILE-002 | MUST | d7-test | T7-GET-02 | 2 |
| REQ-RES-PROFILE-004 | MUST | d7-test | T7-GET-02 | 0 |
| REQ-RES-PUBLIST-005 | MUST | d7-monitor | T7-MON-08 | 1 |
| REQ-RES-PUBLIST-007 | MUST NOT | d7-test | T7-SET-08 | 0 |
| REQ-RES-PUBLIST-008 | MUST NOT | d7-monitor | T7-MON-14 | 1 |
| REQ-RES-PUBLIST-009 | MUST | d7-monitor | T7-MON-14 | 0 |
| REQ-RES-PUBLIST-010 | MUST | d7-monitor | T7-MON-14 | 0 |
| REQ-RES-PUBLIST-011 | MUST | d7-monitor | T7-MON-14 | 1 |
| REQ-RES-PUBLIST-013 | MUST | d7-monitor | T7-MON-14 | 2 |
| REQ-RES-PUBLIST-014 | MUST | d7-monitor | T7-MON-14 | 1 |
| REQ-RES-PUBLIST-015 | SHOULD | d7-monitor | T7-MON-14 | 1 |
| REQ-RES-PUBLIST-016 | MUST | d7-test | T7-SET-08 | 1 |
| REQ-RES-PUBLIST-017 | MUST | d7-monitor | T7-MON-15 | 1 |
| REQ-RES-PUBLIST-018 | MAY | d7-monitor | T7-MON-15 | 0 |
| REQ-RES-PUBLIST-020 | MUST | d7-test | T7-GET-03 | 1 |
| REQ-RES-PUBLIST-024 | MUST | d7-test | T7-SET-08 | 0 |
| REQ-RES-PUBLIST-025 | MUST | d7-test | T7-SET-08 | 0 |
| REQ-RES-PUBLIST-027 | MUST | d7-test | T7-GET-03 | 0 |
| REQ-RES-REFDES-002 | MUST | d7-test | T7-SET-10 | 1 |
| REQ-RES-REFDES-004 | MUST | d7-test | T7-GET-03 | 0 |
| REQ-RES-REFDES-009 | MUST | d7-test | T7-SET-09 | 1 |
| REQ-RES-REFDES-010 | MUST | d7-test | T7-SET-09 | 0 |
| REQ-RES-REFDES-011 | MUST | d7-test | T7-GET-07 | 1 |
| REQ-RES-REFDES-013 | MUST | d7-validate, d7-test | T7-GET-07 | 1 |
| REQ-RES-REFDES-014 | MUST | d7-test | T7-GET-07 | 0 |
| REQ-RES-SUBLIST-004 | MUST | d7-test | T7-SET-11 | 1 |
| REQ-RES-SUBLIST-005 | MUST | d7-monitor | T7-MON-15 | 0 |
| REQ-RES-SUBLIST-006 | MAY | d7-monitor | T7-MON-15 | 0 |
| REQ-RES-SUBLIST-008 | MUST | d7-test | T7-GET-03 | 1 |
| REQ-RES-SUBLIST-013 | MUST | d7-test | T7-SET-11 | 0 |
| REQ-RES-SUBLIST-014 | MUST | d7-test | T7-SET-11 | 0 |
| REQ-RES-SUBLIST-015 | MUST | d7-test | T7-SET-11 | 0 |
| REQ-RES-SUBLIST-016 | MUST | d7-test | T7-SET-01 | 1 |
| REQ-TOP-002 | MUST | d7-validate, d7-monitor | T7-MON-01 | 0 |
| REQ-TOP-003 | MUST NOT | d7-monitor | T7-MON-01 | 0 |
| REQ-TOP-004 | MUST | d7-test | T7-ADR-02 | 1 |
| REQ-TOP-006 | MUST | d7-monitor, d7-test | T7-MON-02 T7-ADR-01 | 4 |
| REQ-TOP-010 | MUST | d7-monitor, d7-test | T7-MON-02 T7-ADR-01 | 3 |
| REQ-TOP-014 | MUST | d7-test | T7-SET-01 | 0 |
| REQ-TOP-015 | MUST | d7-test | T7-SET-01 | 0 |
| REQ-TOP-016 | MUST | d7-test | T7-ACT-01 | 0 |
| REQ-TOP-017 | MUST | d7-monitor | T7-MON-05 | 2 |
| REQ-TOP-018 | MUST | d7-validate, d7-test | T7-ADR-03 | 0 |
| REQ-TOP-024 | MUST NOT | d7-monitor | T7-MON-19 | 1 |
| REQ-TOP-027 | SHOULD | d7-test | T7-SET-12 | 1 |
