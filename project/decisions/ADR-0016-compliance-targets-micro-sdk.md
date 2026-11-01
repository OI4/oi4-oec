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

## Amendment C-015 (SF-037, SF-038)

- **Messaging Level 3 now means all messages** (SF-037, guideline `e0b1676`): all methods (incl. `Del`, `Call`/`Reply`), resources (incl. `Config`, `Data`, `Metadata`, `Event`, `SubscriptionList`) and the common service `NewDataSetWriterId`. An entity without content for a resource answers `Get` with an empty list and publishes nothing on its own (catalog REQ-CMP-033 … 043). For the L3 artifacts this means:
  - **Weather demo (D3)** and **registry (D5)** additionally need `SubscriptionList`, `Event`, `Del`, `Call`/`Reply` with `NewDataSetWriterId` and the empty-list behavior (e.g. the registry has no `Data` of its own and answers with an empty list).
  - The **SDK (D2)** must provide all of this for apps that select L3; the empty-list behavior is a natural SDK default.
- **The compliance domains are independent** (SF-038, guideline `609cb4a`). The table above is unchanged: the micro SDK has no Service Container level, the logger demo stays Messaging L1 with Service Container L3.
- The guideline now explicitly allows Messaging Level 3 on a microcontroller. Whether the micro SDK (D9) should target L3 instead of "L1 (L2 if feasible)" was open question Q-019 (decided in C-016, see below).

## Amendment C-016 (Q-019)

- **Micro SDK (D9) target: Messaging Level 3**, delivered incrementally – **Level 1 first**, then Level 2 and Level 3 in later P4b increments (PO decision, option a). The row in the table above now reads: D9 = Messaging L1 → L3 (designed for L3 from the start), Service Container n/a.
- Consequence for the design of D9: the architecture (resource registry, request dispatcher, `Event` publisher, payload buffers, pagination) is sized for all L3 messages from the first increment, so that later increments add resources without redesign. A smart device with its own MQTT client fulfills its claimed level itself, like an application (MG 1.3 since GL-ADR-017; formerly MG 7.2.2), so L3 includes `SubscriptionList`, `Call`/`Reply` with `NewDataSetWriterId` and the empty-list behavior of MG 7.2.
- D7 must test D9 devices at every level they declare (L1 … L3).
- **Status event after `Set`/`Del` (PO, C-016):** from Level 2 on must, at Level 1 should (MG 7.1.4). This applies to all artifacts; the logger demo (L1, no `Set`) is not affected.

## Amendment C-075 review (2026-11-01, agent, G-3.4 b)

The PO extended P6 in the C-075 review: three demo services – weather (D3), a new **config service (D10)** that shows and edits the `Config` of the other demos, and the logger (D4) with a history; each with a small Web UI outside the OEC standard. Because the config service edits the logger, the logger now offers a `Config` resource. The level targets of the demos become:

| Artifact | Messaging level | Service Container level | Remark |
|----------|-----------------|-------------------------|--------|
| Weather demo (D3) | **L3** | L3 | unchanged: full provider showcase; `Config` of the application and of every station asset |
| Logger demo (D4) | **L2** | L3 | `Get` answered, `Get`/`Set Config` offered (resources, filters, output), so the `Event` Status after a `Set` is a must (MG 7.1.4, PO C-016); still C on `oec-ffi` (Q-024) |
| Config service (D10) | **L1** | L3 | minimal participant: Birth/Will, `Pub` MAM + Health; it offers nothing to modify and only sends requests (`Get`/`Set Config`) to others; configured by file/environment |

- The PO's wish of C-007 – one demo at L1, one at L3 – still holds; the three demos now show all three Messaging levels.
- The part "configured via file/environment (no `Config` resource)" of the logger row above and of Q-024 is superseded; the language C of Q-024 stays.
- D7 checks each demo at its declared level (`level3`, `level2`, `level1`), D8 each image at `level3` (P6 exit, ROADMAP).
