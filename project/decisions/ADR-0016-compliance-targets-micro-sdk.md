# ADR-0016 – Compliance level targets; additional micro SDK

- Status: accepted
- Date: 2026-10-14
- Deciders: product owner (Thomas), C-007
- Related: Q-015, Q-016, ADR-0011, MG 1.3, RIG 1.2, GL-ADR-009, G-4.6

## Decision

| Artifact | Messaging level | Service Container level | Remark |
|----------|-----------------|-------------------------|--------|
| SDK (D2) | L1, L2 and L3 (selectable per app) | supports L3 (bootstrap, SBOM tooling for images) | ADR-0011 |
| **Micro SDK (D9)** – new | L1 (L2 if feasible) | n/a (no container) | for microcontroller-class smart devices; C17 on ESP32 (ADR-0017) |
| Weather demo (D3) | **L3** | L3 | full showcase: Data + Metadata, Config, Profile, PublicationList, ReferenceDesignation, Get/Set |
| Logger demo (D4) | **L1** | L3 | minimal participant: Birth/Will, Pub MAM + Health, consumes topics; configured via file/environment (no `Config` resource, since L1 needs no Get/Set) |
| Registry (D5) | L3 | L3 | |

Which demo gets L1 was chosen by the agent: the PO asked for one demo at L1 and one at L3.
- **Logger at L1:** it is a pure consumer, so L1 shows how little is needed to join the bus.
- **Weather at L3:** it exercises the full provider side of the SDK.

## Consequences

- **`oec-core` must stay `no_std`-compatible** (`alloc` optional) from the start, so that the micro SDK can reuse its codec and topic logic. ADR-0011 is amended accordingly.
- The micro SDK is a separate deliverable D9 in `oec/sdk/micro/` with its own roadmap phase P4b. Its scope is limited to the Messaging levels it targets.
- D7 must be able to test devices running the micro SDK (Messaging L1). D8 does not apply to them.
- The roadmap item "optional smart-device example" is replaced by the micro SDK example.

## Amendment C-009 (ADR-0017)

The micro SDK is written in **C17**, with ESP-IDF on an ESP32 as the reference platform. It does **not** reuse the Rust `oec-core`, so the consequence "`oec-core` must stay `no_std`-compatible" is **dropped**. Consistency between the two SDKs comes from the shared D1 schemas, the test vectors and D7.
