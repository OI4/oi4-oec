# C-010 – Micro SDK: C vs Rust assessment; Q-011 working assumption

- Date: 2026-10-14
- Phase: P3
- Agent: GitHub Copilot CLI (Claude Opus 5.5)
- Guideline baseline: `oi4-oec-documentation` @ `7bead4c` (unchanged)

## Input from product owner

- About Q-016: does C match the agent's recommendation? Would Rust be better? But C has the most libraries for Arduino and ESP32.
- Q-011 stays parked. Working assumption: not publicly available on the internet; private GitHub repos instead.

## Assessment (details in the ADR-0017 amendment)

- The agent's earlier Rust proposal was driven by reusing `oec-core`. For a micro SDK whose scope is L1 (L2 if feasible), that reuse matters little.
- **C wins on:** adoption by device makers, integration into ESP-IDF, Arduino and vendor SDKs, the default ESP32 toolchain, and Arduino library support.
- **Rust wins on:** memory safety and a single protocol implementation.
- **Recommendation: C17.** It is confirmed, with mitigations: static analysis, sanitizers in the host tests, fuzzing of the parsers. The shared test vectors and D7 keep the C and Rust SDKs aligned.
- **Added:** an Arduino library (Arduino-ESP32) as a second port, best effort after the ESP-IDF reference port. AVR boards are out of scope because they cannot do TLS.

## Changes

- **ADR-0017:** amendment with the options table, the mitigations and the Arduino port.
- **ADR-0012:** amendment – publication target is private GitHub repos; the repo stays local until the PO gives the go.
- **OPEN-QUESTIONS:** Q-011 gets the working assumption.
- **ROADMAP:** P4b (Arduino port, C safety measures) and P9 (publication target).
- **Other files:** GUARDRAILS G-3.8, `oec/README.md`, STATUS.

## Next

- PO confirms the phase order P1 → P2. Then P1 starts.
