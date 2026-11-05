# Security policy

> **Draft (C-108).** The deliverables of OEC v2.0 are not published yet
> (Q-011). The reporting channel, the response times and the support period
> below are decided (C-108 D-1 … D-3). This policy takes effect with the
> first release.

## Supported versions

| Version | Supported with security fixes |
|---------|-------------------------------|
| 0.1.x (pre-release, not published) | no – pre-release; fixes go into the next build |
| first release of a release line | yes – for five years from the first release of that line; the newest minor version of the line gets the fixes |

The demo services (logger, weather, config) are examples and have no support
period; their fixes come with the next release.

The schema follows its own version (`2.0.0`, frozen); a security fix of a
deliverable does not change the schema version.

## Reporting a vulnerability

Please **do not** open a public issue, pull request or discussion for a
suspected vulnerability.

Report it privately:

- with **"Report a vulnerability"** (GitHub private vulnerability reporting)
  in the Security tab of the release repository `OI4/oec`, or
- if you have no GitHub account, mail the issue to
  [info@openindustry4.com](mailto:info@openindustry4.com).

Please include:

- the affected deliverable and version (or commit),
- a description of the issue and its impact,
- the steps or a proof of concept to reproduce it,
- whether the issue is known to others or exploited.

## What happens next

1. We acknowledge your report within 3 working days.
2. We assess it (severity with CVSS) within 10 working days and tell you
   the result.
3. We fix it – a fixed version is available for a critical vulnerability
   (CVSS 9.0 or higher) within 14 days, high (7.0 – 8.9) within 30 days,
   medium (4.0 – 6.9) within 90 days, low with the next release – and agree
   the publication date with you (coordinated disclosure, at most 90 days
   after the report).
4. We publish a security advisory (with a CVE if applicable) together with
   the fixed version, and credit you unless you prefer otherwise.

An actively exploited vulnerability is also notified as required by the EU
Cyber Resilience Act (Art. 14: early warning within 24 hours, notification
within 72 hours, final report within 14 days after a fix is available).

## Scope

In scope – the deliverables of this repository:

| Deliverable | Folder |
|-------------|--------|
| OEC Rust SDK and its C ABI `oec-ffi` | `sdk/rust` |
| OEC Go SDK | `sdk/go` |
| OEC micro SDK (C, microcontrollers) | `sdk/micro` |
| D7 `oec-check` and D8 `oec-infra` | `conformance` |
| demo services logger, weather, config | `demos` |
| OEC Registry with its Web UI | `registry` |
| JSON Schema of OEC v2.0 | `schema` |

Out of scope:

- vulnerabilities of a third-party component – report them to its project;
  we update the component in our deliverables (the SBOMs below name them);
- the development setup `dev` (its test PKI and passwords are generated for
  local use only) and the test brokers of the test harnesses;
- the OEC guideline documents (report issues of the specification to the
  OI4 Alliance).

## Software bill of materials and dependency checks

- Every deliverable has an SBOM (SPDX 2.3) with the texts of all licenses
  and a license report: `ci/sbom.sh`, artifact `oec-sbom` of the workflow
  `supply-chain.yml`. Every image carries its SBOM in `/opt/oi4/licenses`
  (RIG 4.1.4).
- The dependencies are checked against the public advisory databases:
  `govulncheck` (Go), `cargo deny check advisories` (Rust, RustSec),
  `npm audit` (Registry Web UI): `ci/audit.sh`, job `audit` of
  `supply-chain.yml` – on every change and, from the first release, weekly;
  Dependabot alerts are enabled on the release repository. The Rust
  dependency policy (`sdk/rust/deny.toml`) also rejects yanked and
  unmaintained crates.

See [`ci/README.md`](ci/README.md) for running both locally.
