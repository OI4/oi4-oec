# ADR-0035 – User management of the OEC Registry HTTP side

- Status: **accepted (agent, G-3.4 b)** for the design as built. **Decision of the PO** (G-3.4 a, scope): the registry gets "a very simple, yet secure user management for the front end and the web services" (C-113 D-2, 2026-11-28).
- Date: 2026-11-28
- Deciders: agent (design, implementation); product owner (Thomas) – the scope
- Related: ADR-0029 §7 (HTTP side of the registry), ADR-0028 amendment C-083 (hardening contract of the Web UIs), ADR-0023 (dependency policy; amendment C-113 below), GL-ADR-022 (secret storage `/run/secrets/oi4`), C-113 (red/blue-team exercise, finding of the unauthenticated registry HTTP side), RIG REQ-REG-007 (HTTPS)

## Context

The HTTP side of the registry (ADR-0029 §7) was read-only and had no login: anyone who could reach the ports 5798/5799 could read the whole directory (MAM, Health, Events, Data, PublicationList, SubscriptionList of every asset). The red team of C-113 listed this as an information disclosure in a shared network; the PO decided (D-2) to add a very simple but secure user management for the Web UI and the REST API. Neither MG nor RIG defines users of an application's Web UI, so this is an engineering decision within the scope set by the PO.

## Decision

### 1. Users and roles

- Two roles: `admin` (reads everything, manages the users) and `user` (reads everything). The bus side is unchanged (still read-only, ADR-0008).
- At most 64 users. A name has 1 … 64 characters `[A-Za-z0-9._@-]`. A password has 12 … 256 characters and is not the name (case-insensitive). There are no further composition rules (NIST SP 800-63B).
- The last `admin` can neither be deleted nor demoted (`409`).
- A password set by an admin (a new user, a reset) must be changed at the next login (`mustChange`). Until it is changed, a session may only read `/api/session`, log out and change the password (`403` otherwise).

### 2. Password storage

- PBKDF2-HMAC-SHA256 with 600 000 iterations (OWASP 2023), a 16-octet random salt and a 32-octet hash, stored as `pbkdf2-sha256$<iterations>$<salt hex>$<hash hex>`. The iterations are read from the stored value, so they can be raised later.
- The implementation is aws-lc-rs, the crypto provider of rustls that the registry already links (no new crate, amendment of ADR-0023 below). The comparison is constant-time (`pbkdf2::verify`). For an unknown name a dummy hash is checked, so the answer takes the same time.
- Hashing runs on the blocking pool of tokio (`spawn_blocking`), at most two at a time (a semaphore; a request that waits more than 5 s gets `503`).
- The store is `<root>/etc/oi4/app/registry/users.json` on the application-specific volume (REQ-CNT-010): directory mode `0700`, file `0600`, written atomically (temporary file and rename). Without the volume the users live in memory only (a warning). An invalid store stops the registry with an error; deleting the file is the recovery (the next start bootstraps again).

### 3. Bootstrap

Without a store the registry creates the user `admin`:

- with the password of the file `OEC_REGISTRY_ADMIN_PASSWORD_FILE` (default `<root>/run/secrets/oi4/registry_admin_password`, the secret storage of GL-ADR-022; one trailing line break is removed; the policy of §1 applies) – the platform or the operator chose it, so it need not be changed; or
- without that file, with a random password of 20 characters that is printed once on stderr (not through `log`, whose records also go to Syslog on the bus) and must be changed at the first login.

A named `OEC_REGISTRY_ADMIN_PASSWORD_FILE` that cannot be read is an error. The file is only read for the bootstrap.

### 4. Sessions

- A login creates a session: a random token of 32 octets (64 hex characters). The server keeps only its SHA-256 (a leaked memory dump does not reveal live tokens); at most 256 sessions, the oldest is evicted.
- A session ends after 30 min without a request (an open event stream counts as activity), 12 h after the login, at logout, when its user is deleted or an admin sets the user's password. A password change by the user ends the user's other sessions.
- The role and `mustChange` are looked up on every request; a role change takes effect at once.
- The token is sent as the cookie `__Host-oec_session` (TLS: `Secure; HttpOnly; SameSite=Strict; Path=/`; plain HTTP of development: `oec_session` without `Secure`) or as `Authorization: Bearer <token>` (scripts, the tests). The cookie has no `Max-Age`; the server enforces the lifetime.
- An event stream checks its session at every poll and ends when the session ends.

### 5. Login throttling

Per client IP address: after 5 failed logins (or wrong current passwords) in a row the address gets `429` with `Retry-After` for 60 s; a successful login resets it. The table holds at most 1024 addresses. Behind a NAT or Docker port publishing all clients share one address – acceptable for a demo; the throttle is a brake, the password policy and PBKDF2 are the defence.

### 6. Endpoints and order of checks

| Path | Methods | Login |
|---|---|---|
| Web UI files, `/api/openapi.json`, `/api/asyncapi.json` | `GET` | no |
| `/api/login` | `POST` `{name, password}` → `{name, role, mustChange, token}` + cookie | no |
| `/api/logout` | `POST` → `204`, clears the cookie | no (ends the session if any) |
| `/api/session` | `GET` → `{name, role, mustChange}` | yes (`401` without) |
| `/api/password` | `POST` `{current, new}` | yes |
| `/api/assets`, `/api/assets/{il}`, `/api/events`, `/api/data`, `/api/stream` | `GET` | yes |
| `/api/users` | `GET` (list), `POST` `{name, role, password}` | `admin` |
| `/api/users/{name}` | `PUT` `{role?, password?}`, `DELETE` | `admin` |

The order: the guards of ADR-0028 amendment C-083 (`Host`, `Origin`, `Transfer-Encoding`, `Content-Length` ≤ 64 KiB; the rule "GET only" is dropped) → the route (`404`) → the method (`405` with the `Allow` of the route) → the session (`401`) → `mustChange` and the role (`403`) → the body: `Content-Type: application/json` (`415`), at most 4 KiB (`413`), read within the request time, valid JSON with exactly the members of the operation (`400`). Both ports serve the same API. The same-origin `Origin` check, `SameSite=Strict` and the JSON content type together protect against cross-site requests.

### 7. Web UI

A login form before anything else; a forced password change when `mustChange`; the user name with a logout button in the header; an admin page `#/users` (list, create, set role, reset password, delete). Any `401` returns to the login form.

## Consequences

- Every client of the REST API logs in first (`oec/ci/getting-started.sh`, the Go tests, the README examples). The dev PKI (`oec/dev/pki`) writes `registry_admin_password` for the compose profile `registry`.
- No password ever reaches the bus or the log; the generated bootstrap password appears once on stderr.
- Out of scope: OIDC/LDAP, MFA, per-asset permissions, an audit trail. A later P-phase may add them.

## Amendment of ADR-0023 (C-113, agent, G-3.4 b)

`oec-registry` declares `aws-lc-rs` (already in its `Cargo.lock` through rustls, same version) as a direct dependency for PBKDF2, SHA-256 and the random generator; no new crate enters the dependency tree.
