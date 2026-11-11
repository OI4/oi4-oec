# C-113 – Red team / blue team exercise

- Date: 2026-11-28 (wall clock of the session: 2026-11-11)
- Phase: none open (all phases closed after C-111); PO order (G-3.4 a: scope)
- Agent/Author: GitHub Copilot (Claude Opus 5.5)
- Status: in progress
- Baseline: guideline `605fbb7` (unchanged); workspace before the cycle: `1df3064`

## Intent

The PO ordered a red-teaming and blue-teaming exercise: the red team attacks the deliverables, the blue team fixes what it finds.

## Scope (PO answers, session 2026-11-11)

1. **Target:** the whole `oec/` monorepo, prioritised by attack surface – registry and its Web UI, SDK transport/codec (MQTT, TLS, JSON), Config/secrets, demos, micro SDK, D7/D8, CI/supply chain; the integrations (Node-RED, Home Assistant) are included.
2. **Depth:** static analysis plus live attacks with proofs of concept, **only against the local Docker**, without any harm to the PO's machine.
3. **Blue team:** fixes findings directly (test first, gate green, one commit per fix); findings on the guideline become `SF-xxx`, larger design changes become decisions in `REVIEW.md`.

## Rules of engagement (host safety)

- The live range is an isolated copy of `oec/dev/compose.yaml` in the session folder (never committed): compose project `oec-redteam`, its own dev PKI (`go run ./pki -out …`), an **internal** network (no internet, no route to the host), **no published ports**, every container with `mem_limit` 384 MiB, 1 CPU, 256 PIDs; the demo and registry containers keep `read_only`, `cap_drop: ALL`, `no-new-privileges`.
- The attacker is a container in the same network (Alpine with `mosquitto-clients`, `curl`, `openssl`, `python3`), unprivileged (`nobody`), `cap_drop: ALL`, only its scripts mounted read-only.
- Loads are bounded (≤ 200 slow connections per target, ≤ 1500 fake assets of ≤ 60 KB); other containers of the PO on the same engine (`oec-node-red`, `oec-hw-fwd-8883`, `fieldgate-demo`, …) are not touched.
- The images under test are the local `oi4/oec-*:0.1.0` images.

## Acceptance

- Red team report: every finding with id, severity, location, scenario and confidence; live findings with their reproduction.
- Every implementation finding fixed test first, or recorded with a reason (accepted risk, decision for the PO, SF).
- Gate `oec/check.sh` green before every commit; the live attacks rerun against rebuilt images after the fixes.
