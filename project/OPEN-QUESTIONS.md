# Open questions – pending decisions with the product owner

| ID | Question | Context / options | Blocks | Status |
|----|----------|-------------------|--------|--------|
| Q-001 | Which language(s)/runtime and form for the SDK? | `research/sdk-architecture-comparison.md` | P3–P7 | **decided C-004: S4 – Rust SDK + C API, layered core/runtime/front ends; further languages later via spike S9 vs S1/S2** (ADR-0011) |
| Q-002 | Where does the new code live (repo layout)? | | P2+ | **decided C-004: local git repo at workspace root, deployables in monorepo folder `oec/`, docs as new standalone Docusaurus in `oec/docs`** (ADR-0012) |
| Q-003 | Are `_ToDelete_` resources (License, LicenseText, RtLicense, Interfaces, SBoM, NE107 event category) removed in 2.0? | See SF-007. Affects mandatory resource set and schema scope. | P1, P2 | **decided C-002: yes, ignore them** (ADR-0005) |
| Q-004 | Who decides spec interpretations? | | P1 | **decided C-003: product owner** (ADR-0007) |
| Q-005 | Weather data source for the demo | Open-Meteo free tier is non-commercial only (grey zone for an alliance demo that companies copy). | P6 | **decided C-003: MET Norway (keyless, CC BY 4.0, commercial use allowed) behind a provider interface + offline simulation** (ADR-0010) |
| Q-006 | Registry scope: read-only observer, or also management actions? | | P7 | **decided C-003: step 1 read-only** (ADR-0008); management actions possibly later |
| Q-007 | Version control for `project/` and new code | | all | **decided C-003: publish everything on GitHub once Q-001/Q-002 are settled; afterwards every change is committed individually** (ADR-0009), amended C-004: local repo now, publication later (ADR-0012, Q-011) |
| Q-008 | Target platforms – keep ARMv7 (32-bit)? | Disadvantages analyzed in `research/sdk-options.md` §4. | P4–P7, P9 | **decided C-003: best effort** – support ARMv7 where the toolchain allows, not a criterion that rules out a language; amd64 + arm64 are mandatory |
| Q-009 | Conformance tool independence from the SDK | | P3, P5 | **decided C-003: D7 shares only the D1 JSON Schemas with the SDK, no code (and no SDK-derived test data)** (ADR-0006); language → Q-001 |
| Q-010 | Conformance tool scope | | P5 | **decided C-003: applications only** (ADR-0006) |
| Q-011 | Publication of (parts of) the work and EU Cyber Resilience Act (CRA) | PO: publication must be reconsidered in light of the CRA (e.g. open-source steward role, SBOM, vulnerability handling, security updates, support period). | P9 | open |
| Q-012 | Language of the conformance tool (D7) | Options: Rust in a separate build without SDK dependencies (one toolchain) or Go (natural independence, trivial cross-compile). Must produce executables for Windows + macOS (+ Linux). | P2 (`validate`), P5 | open |
