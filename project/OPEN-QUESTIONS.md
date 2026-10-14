# Open questions – decisions with the product owner

New questions are added to the first table. Once decided, the row moves to the second table (with cycle and ADR reference).

## Open

| ID | Question | Context / options | Blocks | Status |
|----|----------|-------------------|--------|--------|
| Q-011 | Publication of (parts of) the work and EU Cyber Resilience Act (CRA) | PO: publication must be reconsidered in light of the CRA (e.g. open-source steward role, SBOM, vulnerability handling, security updates, support period). Note C-006: the guideline itself now requires an SBOM in `/opt/oi4/licenses` from Level 2 (GL-ADR-002), which we generate anyway (G-4.8). | P9 | open – deliberately parked (C-005) |
| Q-016 | Micro SDK (D9): language and reference hardware? | Proposal: Rust `no_std` reusing `oec-core` (+ C header), embedded MQTT 3.1.1 client with TLS (e.g. `embassy` + `rust-mqtt` + `embedded-tls`); reference board ESP32-C3/S3 (Wi-Fi, enough RAM for TLS) – alternative: pure C on top of a vendor SDK (ESP-IDF, Zephyr). TLS ≥ 1.2 with certificates is the main constraint for MCUs. | P3, P4b | open (new C-007) |
| Q-017 | D8 scope: does "infrastructure compliance" include **platform** checks (RIG 5.1: broker TLS-only, `/etc/oi4` provisioning, registry ports, resources) or only the Service Container compliance of app containers? | Assumption in ADR-0014: both – first increment offline OCI image inspection (app containers), platform checks afterwards as far as testable. | P5b | open – assumption taken (C-007) |

## Decided

| ID | Question | Decision | Cycle | Reference |
|----|----------|----------|-------|-----------|
| Q-001 | Which language(s)/runtime and form for the SDK? | S4 – Rust SDK + C API, layered core/runtime/front ends; further languages later via spike S9 vs S1/S2 (`research/sdk-architecture-comparison.md`) | C-004 | ADR-0011 |
| Q-002 | Where does the new code live (repo layout)? | Local git repo at workspace root; deployables in monorepo folder `oec/`; docs as new standalone Docusaurus in `oec/docs` | C-004 | ADR-0012 |
| Q-003 | Are `_ToDelete_` items (License, LicenseText, RtLicense, Interfaces, SBoM, NE107) removed in 2.0? | Yes – ignored by all deliverables (SF-007); confirmed upstream by GL-ADR-008 (C-006) | C-002 | ADR-0005 |
| Q-004 | Who decides spec interpretations? | Product owner | C-003 | ADR-0007 |
| Q-005 | Weather data source for the demo | MET Norway (keyless, CC BY 4.0, commercial use allowed) behind a provider interface + offline simulation; Open-Meteo free tier is non-commercial only | C-003 | ADR-0010 |
| Q-006 | Registry scope: read-only observer or also management actions? | Step 1 read-only; management actions possibly later | C-003 | ADR-0008 |
| Q-007 | Version control for `project/` and new code | Every change committed individually; local repo now, publication later (Q-011) | C-003, amended C-004 | ADR-0009, ADR-0012 |
| Q-008 | Target platforms – keep ARMv7 (32-bit)? | Best effort – where the toolchain allows, not a criterion that rules out a language; amd64 + arm64 mandatory (`research/sdk-options.md` §4). Note C-006: RIG 5.1 still lists ARMv7 for *platforms*; this does not change our decision for SDK-based apps. | C-003 | – |
| Q-009 | Conformance tool independence from the SDK | D7 shares only the D1 JSON Schemas with the SDK – no code, no SDK-derived test data | C-003 | ADR-0006 |
| Q-010 | Conformance tool scope | Applications only (no broker/platform checks); clarified C-006: includes smart devices (treated as applications), tested per Messaging compliance level; app-container + platform checks → separate tool D8 (ADR-0014) | C-003, C-006, C-007 | ADR-0006 |
| Q-012 | Language of the conformance tool (D7) | **Go** – natural independence from the Rust SDK, trivial cross-compile to Windows/macOS/Linux | C-005 | ADR-0013 |
| Q-013 | Does D7 also check Service Container Compliance? | **Two tools:** D7 = application (Messaging) compliance, new D8 = infrastructure compliance (Service Container + platform, Q-017); both Go | C-007 | ADR-0014 |
| Q-014 | Support for v1 traffic (`Oi4`)? | **v2 only** in all deliverables; v1.x is ignored (not detected, not listed) | C-007 | ADR-0015 |
| Q-015 | Compliance level targets; microcontroller support? | SDK supports L1–L3; additional **micro SDK** (D9) for microcontrollers; registry L3; weather demo L3, logger demo L1 (agent's choice of which demo); Service Container L3 for all containers | C-007 | ADR-0016 |
