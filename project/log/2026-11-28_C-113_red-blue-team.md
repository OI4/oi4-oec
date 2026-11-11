# C-113 – Red team / blue team exercise

- Date: 2026-11-28 (wall clock of the session: 2026-11-11)
- Phase: none open (all phases closed after C-111); PO order (G-3.4 a: scope)
- Agent/Author: GitHub Copilot (Claude Opus 5.5)
- Status: done – D-1 … D-4 answered and applied (addendum 1)
- Baseline: guideline `605fbb7` during the exercise; **`980cde8`** after addendum 1 (SF-119); workspace before the cycle: `1df3064`

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

## Red team

### Method

- **Static:** five parallel security reviews (registry; demos and integrations; Rust SDK/FFI/Go binding; micro SDK; D7/D8, dev environment and CI), each with source locations, exploit scenario and confidence.
- **Live:** the isolated range described above; attack harness `run.py` with the stages `broker`, `acl`, `spoof`, `xss`, `web`, `malformed`, `slowloris N`, `flood N`, `squat` (session folder, not committed).

### Live results – defended

- Broker: plaintext MQTT, anonymous login, a wrong password and port 8884 without a client certificate are refused; TLS 1.0/1.1 refused, TLS 1.3 negotiated.
- ACL: `Set`, `Del` and `Call` of a normal application to another application are dropped (REQ-GEN-001).
- Registry HTTPS: a foreign `Host` or `Origin` → 403, no CORS, CSP/`nosniff`/`no-referrer`, path traversal → 404, `POST`/`PUT`/`DELETE`/`TRACE` → 405, TLS 1.0/1.1 refused. Demo UIs: a foreign `Host` → 403, CSP set; the config service refuses `/api/set` and `/api/refresh` with a foreign or `null` `Origin`.
- Malformed traffic (nesting depth 100 000, invalid UTF-8, 260 KB payloads, wrong types, odd and long topics): no container crashed; the registry ignores schema-invalid messages.
- XSS: a schema-valid fake asset with `<script>`/`onerror` texts and `DeviceManual` `javascript://x%0Aalert(…)` is stored raw and rendered as text by the registry UI (no link for `DeviceManual`).
- Spoofing the registry's own `Health` and squatting the weather service's IL as a device before its Birth: the registry ignored both (the squat test did not reach the vulnerable order live; RR-1 below was proven in a unit test).

### Live results – findings

- **Memory exhaustion (RR-3, RC-1, RS-3):** one authenticated participant published 1500 schema-valid fake applications (`Pub/MAM` of ~60 KB, `Pub/Health`) in 4.4 s. The registry grew from 32 MiB to its 384 MiB limit and was **OOM-killed**; the config service was **OOM-killed** as well; the logger stayed at 12 MiB.
- **Pub spoofing (SF-119):** a second client published `Pub/Health` `FAILURE_1` under the `AppId` of the weather service; the registry recorded it as the weather service's health. The guideline restricts only `Set`/`Del`/`Call` by ACL.
- **Slow connections (RR-4, low):** 16 slow connections make the config UI answer 503 for about 5 s (header timeout); it recovers by itself.

### Findings

| Id | Sev. | Area | Finding | Outcome |
|----|------|------|---------|---------|
| RD-3 | high | Node-RED nodes | Prototype pollution via a bus `Set Config` (`constructor`/`prototype`/`__proto__` group or object names write `Object.prototype.Value`) | fixed |
| RT-1 | medium-high | D7 proxy | Relayed clients reached the broker with D7's own client certificate (`oec-probe`, `readwrite #` in oec/dev) | fixed |
| RR-3 | medium (live) | registry | Unbounded directory, conformity map, lists and values → OOM | fixed |
| RC-1 | medium (live) | config demo | Unbounded application directory → OOM | fixed |
| RS-3 | medium | Rust SDK consumer | Unbounded application directory and interval bookkeeping | fixed |
| RR-1 | medium | registry | A device entry created before an application's Birth kept the attacker as publisher (squatting) | fixed |
| RD-2 | medium | config demo | A forged `Pub/Config` without `Sensitive` uncovered the next secret typed in | fixed (sticky `Sensitive`; UI sends shown Mandatory values) |
| RS-1 | low-medium | Rust SDK, micro SDK, Node-RED | The status of a `Set Config` revealed whether a guessed `DefaultValue` equals a sensitive default | fixed in all three |
| RM-2 | low | micro SDK | `PerPage` near 2^32 wrapped the page count on 32-bit `size_t` → no answer | fixed |
| RT-3 | low | CI | `curl sh.rustup.rs \| sh` without checksum | fixed (pinned `rustup-init` + SHA-256, policy test) |
| H-1 | hardening | Rust SDK store | Temporary file followed symlinks; directories not tightened | fixed |
| SF-119 | guideline | MG 3/9 | No requirement binds `Pub`/`Reply` under an `AppId` to its owner | SF-119 open → PO |
| RR-2 | medium | registry | `/api/data`, `/api/events` unauthenticated; image binds `0.0.0.0` | decision for the PO |
| RD-1 | medium | config demo | Web API unauthenticated (documented demo risk) | decision for the PO |
| RR-4 | low | registry, demos | HTTP slot exhaustion (16 connections, 8 streams, no per-client limit) | decision for the PO |
| RS-2 | low | Rust SDK | `NewDataSetWriterId` range exhaustion by churn | recorded (follow-up) |
| RT-2 | low | D8 `run` | Sandbox container without `CapDrop`/`no-new-privileges`/limits | recorded (follow-up) |
| RT-4 | low | CI | Espressif board index unpinned (pin via release asset + SHA-256) | recorded (follow-up) |
| RM-1 | low | micro demos | UART console on by default, `time` sets the clock | recorded (follow-up) |
| RM-3 | info | micro SDK | Fuzzing gaps (ESP `on_data` reassembly, `-m32` not available on macOS arm64) | recorded |

## Blue team

Seven agents fixed the findings in parallel, each in its own module (registry, config demo, Rust SDK, Node-RED, micro SDK, D7, CI), **test first** (every new test failed before its fix), each with the check of its area; none committed. A follow-up added RS-3 to the Rust SDK after the config agent found the unbounded consumer directory. Then the full gate ran on the union and the fixes were committed one module at a time (A-648).

| Commit | Fix | Tests (new) |
|--------|-----|-------------|
| `2a613e2` | spectool `feedback -check`: an open/decided/reported finding has no commit yet (A-649) | fixture SF-006 in `TestProblems`/`TestBuild` |
| `d7ce456` | RD-3 + RS-1 analog (Node-RED) | `test/hardening.test.js` (13 tests) |
| `7b5cc88` | RS-1 (Rust SDK) | `a_sensitive_default_is_no_oracle` |
| `919883c` | H-1 FileStore | `file_store_does_not_follow_a_temporary_symlink`, `file_store_is_owner_only` extended |
| `afc7f78` | RS-3 consumer directory limits | `the_directory_is_bounded`, `offline_applications_are_evicted_first`, `an_oversized_mam_is_not_stored`, `interval_bookkeeping_is_bounded` |
| `ed336ba` | RT-1 D7 proxy | `TestUpstreamIdentity`, `TestProxyUpstreamIdentity`, `TestLoopback`, usage cases |
| `bf6f405` | RM-2 + RS-1 analog (micro SDK) | `test_page_count_does_not_wrap`, `test_get_pagination_largest_per_page_is_one_page`, `test_set_config_sensitive_default_is_no_oracle` |
| `1b1fc70` | RR-1 + RR-3 (registry) | `device_cannot_squat_an_application`, `a_participant_is_no_device`, `device_of_an_online_publisher_stays`, `entries_are_bounded`, `oversized_values_are_not_kept`, `lists_are_bounded`, `conformity_is_bounded`, `health_history_without_details`, `stream_is_bounded` |
| `7aa0935` | RC-1 + RD-2 (config demo) | `a_flood_of_applications_is_bounded`, `the_oldest_offline_application_is_evicted_first`, `an_application_cannot_pin_more_than_its_budget`, `the_total_budget_evicts_offline_applications`, `sensitive_stays_sensitive` |
| `ca620f5` | RT-3 (CI) | `TestPipesToShell`, `TestNoPipeToShell`, `TestRustup`, `TestCheckRejects` case |

Guideline repository (branch `feature/spec-findings`, not pushed): `9fd40d2` SF-119 in the register and the regenerated feedback package; `0ac3ba0` Community pages (registry limits, config demo, `oec-check` proxy).

## Verification

- **Gate:** first run on the union failed in `conformance/internal/probe` (`TestSimFixtures`, `TestSimActionRequests`: "no Health about itself" of the simulated application, REQ-TOP-017) at a host load of 31 (IDE indexing, Spotlight); the package is unchanged in C-113 and passed alone (141 s); second run: **all checks passed** – `feedback: 119 findings, 219 commits of 5f070bb..605fbb7 attributed, 138 files current`; `cigen: 7 workflows current` (A-660).
- Node-RED (`check.sh` without `OEC_D7`): 41 pass, 1 skip.
- **Images rebuilt** (`oec/registry/images/build.sh`, `oec/demos/images/build.sh`): D8 `image` and `run` at level 3 exit 0 for registry, logger, weather and config; the local `oi4/oec-*:0.1.0` images are now the fixed ones.
- **Live rerun against the rebuilt images:**

| Attack | Before | After |
|--------|--------|-------|
| 1500 fake applications × 60 KB | registry 32 → 384 MiB, **OOM-killed**; config service **OOM-killed** | registry 14 → 19 MiB, config 6 MiB, all running |
| then 6000 × 6 KB | – | registry 23 MiB, 2912 assets, `ignored` 4275; config service exactly 1000 applications (cap); the real applications stay listed and online |
| `all` (broker, ACL, spoof, web) | defended | defended (no regression) |

- Range torn down (`docker compose down -v --rmi local`): no container, network, volume or image of `oec-redteam` left; the PO's containers (`oec-node-red`, `oec-hw-fwd-8883`, `fieldgate-demo`) were running throughout, untouched.

## Agent decisions (G-3.4 b)

- **A-647 Isolated range:** a copy of the dev compose in the session folder with an internal network and no published ports, limits per container, an unprivileged attacker; never committed (the PO's host safety condition).
- **A-648 Gate on the union, then one commit per module:** the seven fixes touch disjoint modules; the gate ran once on all of them, the commits are slices of the verified tree. Running the 20-minute gate ten times would not check anything more.
- **A-649 Pending findings in `feedback -check`:** the check required a commit for every finding, so a newly registered open finding (SF-119) blocked the gate until the PO decides; now only a resolved finding needs one, pages of pending findings say "None yet" (spec README updated).
- **A-650 Registry limits:** 4096 assets/conformity entries, 8 KiB per stored value, 4096 list entries, 512 characters per violation text; the offline unlisted asset seen longest ago gives way. 8 KiB because values are held twice (directory and SDK); 4096 × 2 × 8 KiB ≈ 64 MiB worst case. Rule 7: an application's own Birth replaces a device entry about it.
- **A-651 Config demo:** 1000 applications, 256 KiB per application, 64 MiB in total; `Sensitive` sticky for the lifetime of the entry; the UI sends the Mandatory non-sensitive values it shows (API clients that send only changed values keep working, noted in the README).
- **A-652 SDK consumer limits:** 1000 applications, 16 KiB per MAM/Health, 4096 interval topics; `set_directory_limits()` in Rust only (additive), C ABI and Go use the defaults.
- **A-653 Sensitive `DefaultValue` in a `Set`** is compared with its published form (mask, or empty for an empty default) in the Rust SDK, the micro SDK and Node-RED – the status no longer depends on the secret (REQ-RES-CONFIG-023/024, GL-ADR-021). Node-RED additionally accepts the mask for an empty default; harmless, aligned at the next touch.
- **A-654 `__proto__`** is refused by the Node-RED nodes as a Config group/object or Data field name (the guideline name pattern allows it; a JS object cannot hold it as a plain key).
- **A-655 D7 proxy:** relayed connections never carry D7's client certificate; `--proxy-upstream-cert/-key` give the identity of the component under test; a non-loopback `--proxy-listen` warns instead of failing (the documented ESP32 flow uses it).
- **A-656 Micro pagination:** overflow-free page count; `PerPage` echoed unchanged (REQ-PAG-004 allows the requested value).
- **A-657 rustup-init 1.29.3** pinned by SHA-256 for the six runner platforms; a policy test forbids piping a download into a shell in `oec/ci/*.sh` and generated run steps.
- **A-658 FileStore:** exclusive create (`create_new`, 0600) instead of `O_NOFOLLOW` (no `libc` dependency); directories tightened with a warning if not ours.
- **A-659 Recorded, not fixed (follow-ups):** RS-2 (`NewDataSetWriterId` exhaustion, also by asset churn in the SDK listing), RT-2 (D8 sandbox without `CapDrop`/limits), RT-4 (Espressif index: pin the release asset by SHA-256), RM-1 (UART console default), RM-3 (fuzzing gaps), online entries of the SDK consumer never expire (1000 fake online applications block discovery, memory stays bounded), the registry stream has no "removed" message, `ui/src/api.ts` `Limits` lacks the three new fields (UI rebuild), process substitution (`bash <(curl …)`) not covered by the policy.
- **A-660 Gate flake under load:** the D7 simulation fixtures depend on wall-clock timing; no change in C-113, rerun green. A repeat under normal load gets a closer look (as A-644).

## Open for the PO

- **D-1 – SF-119:** add a SHOULD for an ACL that binds `Pub`/`Reply` under an `AppId` to its owner (MG 3, MG 9) plus a consumer note – (a) as proposed (recommended), (b) informative note only, (c) no change.
- **D-2 – Registry HTTP API without authentication (RR-2, RR-4):** `/api/data` shows the latest values of every DataSet on the bus to anyone who reaches the port; compose publishes on `127.0.0.1` only, the Community page says "publish the ports only where the directory may be read" – (a) keep, documented (recommended: on a platform, access control belongs to the platform, RIG 5.1), (b) data view opt-in (off by default), (c) token/basic authentication in the registry.
- **D-3 – Config demo API without authentication (RD-1):** documented demo risk, compose binds `127.0.0.1` – (a) keep (recommended, demo), (b) per-start token printed in the log.
- **D-4 – Push:** the workspace (12 commits; full CI ≈ US$ 6), the guideline branch (2 commits) and afterwards the re-sync of the copies `oi4-oec-registry` (registry) and `oi4-oec-demo` (Node-RED nodes) – (a) push and re-sync now (recommended), (b) later.

## Hand-over (G-3.10)

Same session: the exercise is closed; the next work depends on the answers D-1 … D-4 (answered, addendum 1).

## Addendum 1 – PO answers D-1 … D-4 (2026-11-28)

PO: "D-1 (a) reicht hier should oder ist ein shall? D-2 erweitere die Registry um ein ganz einfaches, dennoch sicheres User-Management für das Front-End und die Webservices. D-3 (a) da nur demo. D-4 (a) nach den Anpassungen D-1 bis D-3."

### D-1 – SF-119 as SHOULD (A-661, A-662)

- **Answer to "should or shall":** SHOULD is enough for now and was kept (A-661). Reasons: (1) a MUST needs a defined way for the platform to learn the `AppId` of an application before it starts – the platform knows the container name, not the full `AppId` (Model, ProductCode, SerialNumber come from the application); the guideline defines no such hand-over; (2) the same strength as the existing read restriction of `Set/Config` (MG 9.1.3); (3) a MUST without a way to meet it would make every current platform non-compliant. A MUST is a one-word change once the guideline defines how the platform learns the `AppId` (or once messages are signed); then D8 T8-BUS-02 turns the warning into a failure.
- Guideline (branch `feature/spec-findings`): `980cde8` MG 3 bullet and MG 9 RECOMMENDED box with the consumer note; `2f17f39` SF-119 resolved (no GL-ADR – a recommendation, no protocol change); `bf668d7` Community page of `oec-infra` (T8-BUS-02).
- Workspace: `f463ad0` re-baseline on `980cde8` (inventory 1407 → 1411: four new items, BUS 26 → 29, GEN 77 → 78; new **REQ-GEN-002** (SHOULD, platform, verified by D8) and two informative MG 9 items; 445 REQs, 0 gaps; `schema.Baseline`, D7/D8 catalogs, feedback package `5f070bb..980cde8`); `4a8a4a1` D8 T8-BUS-02 publishes `Pub/Health` and `Reply/NewDataSetWriterId` under a foreign `AppId` with the unprivileged client and warns REQ-GEN-002 when the broker delivers it; new fault fixture `T8-BUS-02/foreign-pub`.
- **A-662 Dev ACL not bound:** the dev Mosquitto ACL of `oec/dev` cannot bind the `AppId` topic level to a user (the dev user names and certificate identities are not the `AppId`s the applications choose); D8 warns REQ-GEN-002 there, documented in `oec/dev/README.md` as a dev limitation (like REQ-RES-CONFIG-026). A platform that knows the `AppId` of each client can bind it.

### D-2 – user management of the registry (A-663 … A-666)

- **ADR-0035** (`d4ea119`, `accepted (agent, G-3.4 b)`, scope by the PO): two roles `admin`/`user`, at most 64 users, password 12 … 256 characters and not the name; PBKDF2-HMAC-SHA256 600 000 iterations (aws-lc-rs, already linked through rustls – ADR-0023 amendment, no new crate); store `etc/oi4/app/registry/users.json` (0600, atomic); bootstrap user `admin` from `run/secrets/oi4/registry_admin_password` (GL-ADR-022) or a random password printed once on stderr with a forced change; sessions of 32 random octets (only the SHA-256 kept), 30 min idle, 12 h maximum, at most 256; cookie `__Host-oec_session` (`Secure; HttpOnly; SameSite=Strict`) or bearer token; per-IP throttle (5 failures → 60 s `429`); last admin protected; password changes and deletions end sessions.
- Backend `0f39be0`: `auth.rs`, routes `/api/login`, `/api/logout`, `/api/session`, `/api/password`, `/api/users[/{name}]`; every data route needs a session; the event stream ends with its session; `openapi.json`; Rust unit tests (48 in `cargo test`), Go host tests `TestHTTPUsers`, `TestHTTPGeneratedPassword` and the harness login (`regtest`).
- UI `432cdb4`: login form, forced password change, `#/password`, `#/users` (admins), user menu, any `401` returns to the login; the UI never keeps the token (the cookie is enough). Verified end to end with Playwright (headless Chromium) against the Go host harness: 12 steps (bootstrap login, forced change, data views, users page, role change, reset, delete, logout, throttle message) without a console error.
- Dev and docs `7c1f6aa`: `oec/dev/pki` writes `registry_admin_password` (32 random characters) into the registry storage; `oec/ci/getting-started.sh` checks the `401` without a session and logs in; READMEs. Guideline `9d6f961`: Community registry page ("Users and login") and the login hints of the demo pages.
- **A-663 PBKDF2 instead of Argon2:** aws-lc-rs is already in the binary; Argon2 would add a crate (ADR-0023) for a demo directory; 600 000 iterations meet OWASP 2023, the iterations are stored per hash and can be raised.
- **A-664 Cookie plus bearer:** the UI uses only the `HttpOnly` cookie (no token in JavaScript, XSS cannot read it), `SameSite=Strict` plus the existing `Origin` guard against CSRF; scripts and tests use the bearer token of the login answer.
- **A-665 The PO's existing `oec/dev/generated/`** predates the file: the agent added `registry_admin_password` to it (random, not committed, git-ignored) instead of `go run ./pki -force`, so the PO's certificates and volumes stay; `getting-started.sh` names `-force` when the file is missing.
- **A-666 Live check:** registry image rebuilt (`images/build.sh`: D8 `image` and `run` level 3 exit 0); `OEC_GS_KEEP=1 oec/ci/getting-started.sh` against the PO's compose project `oec-dev`: "getting-started ok" (401 without login, login, assets via bearer). The story step's `stations = 4` of the weather service was set back to 3 and the stack stopped (`stop`, volumes kept); the PO's other containers were not touched. The registry log names the bootstrap user, never the password.

### D-3 – config demo API (A-667)

- **A-667** (a) kept: the config demo stays without a login (demo; compose binds `127.0.0.1`; the page and the README warn). No change.

### D-4 – push (A-668)

- **A-668** after D-1 … D-3: gate green (below), then the workspace to `origin/main`, the guideline branch `feature/spec-findings`, the re-sync of `oi4-oec-registry` (the registry with the login) and `oi4-oec-demo` (Node-RED nodes of C-113), the hosted runs evaluated.

### Verification (addendum 1)

- `oec/registry/check.sh`: registry ok (Rust tests, Go host tests, uipolicy, cargo deny); `oec/dev` `go test ./...`, `oec/ci` `go test ./...` ok; `oec/docs/check.sh`: 20 pages ok.
- Gate: the first run failed in `oec/demos` (`config` `TestD7Level1`, T7-MON-05: 3 min 51 s between two Health messages within a 103 s test) – the Mac went into maintenance sleep on battery during the run (`pmset -g log`: Sleep 16:22, DarkWake 16:29); no change in the config demo since the last green gate. Second run under `caffeinate`: **all checks passed** (`feedback: 119 findings, 220 commits of 5f070bb..980cde8 attributed, 138 files current`; `cigen: 7 workflows current`).
- `oec/demos-smart-sensors/esp32-dht22/README.md`: login hint for the registry (not in the gate).

