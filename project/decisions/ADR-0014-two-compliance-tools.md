# ADR-0014 – Two compliance tools: application (D7) and infrastructure (D8)

- Status: accepted
- Date: 2026-10-14
- Deciders: product owner (Thomas), C-007
- Related: Q-013, Q-017, ADR-0006, ADR-0013, RIG 1.2, MG 1.3, GL-ADR-009

## Context

Since baseline `5f070bb` the guideline defines two compliance domains: **Messaging** (MG) and **Service Container** (RIG), each with levels L1–L3. In addition, the RIG states platform requirements (RIG 5.1) that are not graded by level. Q-013 asked whether D7 should also check the container side.

## Decision

There are **two separate command-line tools**:

| | D7 – application compliance tool | D8 – infrastructure compliance tool |
|---|---|---|
| Checks | OEC **Messaging Compliance** L1/L2/L3 of applications and smart devices (live via MQTT + offline `validate`) | **Service Container Compliance** L1/L2/L3 of application containers (OCI image, SBOM + license texts in `/opt/oi4/licenses`, storage conventions, image integrity, networks) and – as far as the RIG makes them testable – **platform requirements** (Q-017) |
| Input | broker connection, topic/payload files | OCI image (registry reference or archive), optionally a running container / platform host |
| Guideline | MG | RIG |

Common rules for both tools:

- **Language:** Go. ADR-0013 is extended to D8; the reasons are the same (cross-compiling, independence from the Rust SDK).
- **Location:** one Go module in `oec/conformance/` with two separate binaries (`cmd/…`, names fixed in P3). Internal packages such as the report writer (JSON/JUnit) and the test-catalog model may be shared between D7 and D8.
- **Independence:** neither tool uses SDK code (ADR-0006).
- **Same CLI conventions:** report formats, exit codes, test cases referencing REQ ids, and the profiles `level1`/`level2`/`level3`.
- **Platforms:** Windows, macOS and Linux (G-4.13).
- **Q-010 still holds for D7** (applications only). Infrastructure checks are D8's job.

## Assumption (to be confirmed in Q-017)

"Infrastructure compliance" covers the whole RIG: Service Container compliance of app containers first, then platform checks where the RIG is testable.

Platform checks include:
- broker reachable only via TLS
- `/etc/oi4` provisioning
- registry ports

The first increment is offline image inspection, because it needs no running platform.

## Consequences

- New deliverable **D8**, new roadmap phase **P5b**.
- The REQ catalog (P1) must cover the RIG with the same rigor as the MG.
- SBOM layout is canonical since `7bead4c` (SF-027, RIG 4.1.4): D8 checks for exactly one of `/opt/oi4/licenses/sbom.spdx.json` / `sbom.cdx.json` and that every license is either embedded in the SBOM or present as `<License-ID>.txt`.
