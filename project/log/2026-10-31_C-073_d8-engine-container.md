# C-073 – P5b increment 4: Engine API and `oec-infra container`

- **Date:** 2026-10-31
- **Phase:** P5b – Infrastructure compliance tool (D8), increment 4
- **Baseline:** guideline `7427a4f` (branch head `5a6f0b1`, findings only), SF-001 … 116 resolved, no open finding; 1410 inventory items, 444 REQs, 158 `schema`/`d7-validate` REQs, 0 gaps, no REQ drafts; 29/29 entry schemas, 245 payload vectors, 18 `broker.json` vectors; schema 2.0.0 frozen, `2.0.1-dev` @ `7427a4f`; fixtures: 29 captures, 59 sim (58 faults), 19 images, 29 bus scenarios, 55 host scenarios, all reviewed; catalog: 55 D7 tests and 10 of 17 D8 tests `implemented`
- **Agent:** new agent session (start prompt of the C-072 handover, G-3.10)

## Intent

PO order (start prompt): P5b increment 4, test first:
- an own, read-only Docker/Podman Engine API client `internal/engine` (Unix socket, Windows named pipe, TCP; reuse the endpoints of A-266);
- `oec-infra container` with T8-CTR-01 … 03 (networks, name/hostname, licenses not overlaid by the host);
- T8-HST-03 (application storages not shared).

The self-test runs against a fake Engine API with recorded inspect documents (no network, no Docker daemon). Fixtures with a verdict start as `draft`. An opt-in run against the running containers of `oec/dev`; deviations in `oec/dev` are fixed or become an SF. Findings as SF-117 ff. No new dependency unless the standard library is not enough.

Open for the PO at the start: nothing.

## Baseline check

`oec/check.sh` reported "all checks passed" at the start. The counts of the start prompt are confirmed. cbindgen 0.29.4 is present. Guideline `7427a4f`, branch head `5a6f0b1`, no open finding. Docker Desktop 4.47 (Engine 28.4.0, API 1.51, linux/arm64) is running; Podman is not installed.

## Design (agent, G-3.4 b)

- **One engine client for both modes.** `internal/engine` sends only `GET` requests: `/_ping`, `/version`, `/containers/json?all=1`, `/containers/{id}/json` and `/networks/{id}`. The paths carry no API version, so the engine answers in its own version; Docker and the compatible API of Podman share these paths. HTTP/1.1 runs over a Unix socket, a Windows named pipe or plain TCP.
- **Named pipe on the standard library.** `syscall.CreateFile` with overlapped I/O, wrapped by `os.NewFile` so that deadlines work. Neither `golang.org/x/sys` nor `go-winio` is needed.
- **Endpoints.** `--daemon`, then `DOCKER_HOST`, then `CONTAINER_HOST`, then the default sockets of the OS. This is the list of A-266; it moved from `internal/host` to `internal/engine`, and a default socket that does not exist is skipped. `engine.Connect` pings the first reachable endpoint. `container` and `host` both use it.
- **Only what D8 judges is decoded.** Name, hostname, image, state, network mode, attached networks and mounts. Mounts include the `--tmpfs` entries of `HostConfig.Tmpfs`, which Docker does not list in `Mounts`. No report contains a mount source, the environment or the labels of a container.
- **Self-test against recordings.** `enginetest` is a fake Engine API that answers `GET` only. It serves inspect documents recorded from Docker Engine 28.4 (`testdata/containers/engine/`). A scenario changes the documents or the engine; the network is loopback only, and no Docker daemon is involved.
- **T8-HST-03 lives in `host`.** The catalog puts it there: it is a platform property across containers. It reads every container of the engine, not a single one.

## Work

1. **`internal/engine`** + `enginetest` (commit `6728917`):
   - the client with endpoints, ping, version, container list and inspect, and network inspect;
   - a response limit of 32 MiB;
   - `ErrNotFound` (404), `ErrNotEngine` (an endpoint that answers but is no Engine API) and `ErrUnsupported` (`ssh://`, `https://` or another scheme D8 does not speak; a named pipe outside Windows);
   - `Within` for path nesting;
   - default endpoints per OS (`defaults_unix.go`, `defaults_windows.go`), named pipe (`pipe_windows.go`, `pipe_other.go`).

   `internal/host` uses `engine.Probe`/`Endpoints`/`Defaults`/`ErrUnsupported` (test first: the moved tests pass unchanged). The 55 reviewed host scenarios are unchanged.
2. **Recordings** (same commit). Four containers were created from the pinned Mosquitto image of `oec/dev` and recorded as `app`, `host`, `stopped` and `sidecar` (description in `testdata/containers/index.yaml`), plus four networks and the version document. They are sanitized:
   - mount sources rewritten to `/srv/oec/rec/…`;
   - no personal path (checked by `TestRecordings`);
   - no secret – the environment is that of the public image.

   The containers, networks and volumes of the recording were removed afterwards.
3. **`internal/container`** with T8-CTR-01 … 03; **`oec-infra container`** (`cmd/oec-infra/container.go`, usage text); **report** (`internal/report/infra.go`: `ContainerRun`, `Engine`, `Container`, text writer); **`infra-report.schema.json`** (additive: mode `container`, key `container`, evidence `container`); `verdict.Evidence.Container` (D7 never sets it); catalog T8-CTR-01 … 03 `implemented` (commit `4e65008`). Tests first:
   - `internal/container/fixtures_test.go`: 26 scenarios, coverage, report, level, Podman, defaults, recordings;
   - `cmd/oec-infra/container_test.go`: run, fail, no engine, usage, each JSON report against the schema;
   - `infra_test.go`: index check, `TestInfraContainerReport`.
4. **T8-HST-03** in `internal/host/storages.go` (commit `d2bcb35`). Also: `host.Config.Self` and `--self`, `TStorages` in `host.Implemented`, catalog T8-HST-03 `implemented`, and `engine.Connect` moved from `container` to `engine`. Tests first: 14 new host scenarios, `TestStorageEvidence` (names the other container and its destination, never a source) and the CLI test `TestHostStorages`.
5. **Live test** `TestContainerDev` in `cmd/oec-infra/live_test.go`, step in `live.sh` (commit `feda3df`; see Runs).

### Fixtures (`draft` → R-1)

**`testdata/containers/index.yaml`** has 26 scenarios against the four recorded containers:

| Scenario | Expected |
|---|---|
| **positive** | |
| `none` (`app`) | every test passes |
| `T8-CTR-01/default-bridge` | pass |
| `T8-CTR-01/sidecar` | pass: the network of `app`, followed one step |
| `T8-CTR-02/max-length` | pass: 63 characters, 26 before `-<uuid>` |
| `T8-CTR-02/podman` | pass: Podman version document and a name without a slash |
| **negative, informative or inconclusive** | |
| `recorded-host` | T8-CTR-01 warn (REQ-CMP-031, REQ-CNT-023); T8-CTR-02 fail (REQ-CNT-019); T8-CTR-03 fail (REQ-CNT-017) |
| `T8-CTR-01/host-network`, `/sidecar-of-host` | warn (REQ-CMP-031, REQ-CNT-023) |
| `T8-CTR-01/none`, `/macvlan`, `/pasta` | info (REQ-CMP-031, REQ-CNT-023) |
| `T8-CTR-01/network-gone` | skip, inconclusive |
| `T8-CTR-02/hostname-id`, `/hostname-case`, `/underscore`, `/too-long`, `/duplicate-case` | fail (REQ-CNT-019) |
| `T8-CTR-02/no-uuid` | warn (REQ-CNT-020) |
| `T8-CTR-03/bind`, `/bind-parent`, `/bind-below`, `/tmpfs` | fail (REQ-CNT-017) |
| `T8-CTR-03/volume` | pass, REQ-CNT-017 `manual` |
| `T8-CTR-03/image-mount` | info (REQ-CNT-017) |
| `engine/unreachable`, `/not-an-engine` | every requirement skipped, inconclusive |

**`testdata/hosts/index.yaml`** has 14 new T8-HST-03 scenarios (69 in total) against `enginetest`, with the recordings plus a second application `app2`, a foreign container `other` and a named volume:

| Scenario | Expected |
|---|---|
| **positive** | |
| `two-apps`, `self-excluded` | pass |
| `missing-app-storage` | pass, REQ-CNT-011 `manual` |
| **negative or inconclusive** | |
| `shared-volume`, `volume-elsewhere`, `recorded-host` | fail (REQ-CNT-011) |
| `shared-secrets`, `backup-secrets`, `covering-secrets` | fail (REQ-CNT-024) |
| `d8-container` | fail (REQ-CNT-024) and REQ-CNT-011 `manual` |
| `no-persistence`, `self-unknown`, `inspect-fails`, `engine-down` | skip (REQ-CNT-011, REQ-CNT-024) |

`none` and `T8-HST-02/certificate` now also run T8-HST-03, which passes on them; their expectations are unchanged, but their content changed (an engine with `app` and `app2`), so they are back to `draft` (see "Changes to reviewed items").

### What a self-test cannot check deterministically (agent decision)

- Windows named pipe: compiled and vetted (`GOOS=windows`), unit-tested only through the endpoint parser; no Windows machine with Docker in this cycle.
- Podman: the version document, a name without a leading slash and the network mode `pasta` are synthetic (A-271); Podman is not installed.
- Other engines of the same platform (a container name must be unique within the OEC platform, REQ-CNT-019): D8 sees one engine and says so in the evidence.

## Runs

- **Gate** (`oec/check.sh`, with `caffeinate -i`): "all checks passed" after each code commit (`gate2` before `4e65008`, `gate3` before `d2bcb35`) and at the end (see Checks).
- **Cross builds** (`CGO_ENABLED=0`, `oec-check` and `oec-infra`): windows/amd64, windows/arm64, linux/amd64, linux/arm64, linux/arm, linux/386, darwin/arm64, darwin/amd64 – all build; `GOOS=windows go vet` of `engine`, `host`, `container` and `cmd/oec-infra` is clean.
- **D8 tests:** `go test -race ./internal/engine/... ./internal/container ./internal/host` pass in about 7 s in total.
- **Opt-in live run** (`oec/conformance/live.sh`, Docker Desktop 4.47, Engine 28.4.0, API 1.51, 2026-10-31): every step green ("live: all tests passed"): `TestMonitorDevBroker`, `TestBusDevBroker` (8883, 8884), `TestHostDevRoots`, `TestHostDevContainer`, and the new `TestContainerDev`. `TestHostDevContainer` now also runs T8-HST-03 inside a container without an engine socket; the requirements are skipped as inconclusive, which the test accepts, because it only checks its selected tests.

  `TestContainerDev` starts, next to the dev broker, two containers from the pinned broker image (`sleep 600`):
  - **app** `oec-live-app-<uuid>`: hostname = name; on the bridge network `oec-dev_default` of the broker; own volumes on `/etc/oi4/app` and `/opt/oi4/app`; a temp-dir bind on `/run/secrets/oi4` (ro);
  - **bad** `oec-live-bad_<8 hex>`: hostname `oec-live-bad`; host network; a temp-dir bind on `/opt/oi4/licenses` (ro); the configuration volume of the app.

  | Run | Exit | Result |
  |---|---|---|
  | `container` app | 0 | T8-CTR-01 … 03 pass |
  | `container` bad | 1 | T8-CTR-01 warn (host network: REQ-CMP-031 warning, REQ-CNT-023); T8-CTR-02 fail (REQ-CNT-019: `_` is no RFC 1123 character, the hostname is not the name; REQ-CNT-020 warns: no `-<uuid>`); T8-CTR-03 fail (REQ-CNT-017: a bind mount of the host on `/opt/oi4/licenses`) |
  | `container` broker `oec-dev-mosquitto-1` | 1 | T8-CTR-01 pass (bridge); T8-CTR-02 fail (REQ-CNT-019: hostname `mosquitto` ≠ the compose name; REQ-CNT-020 warns); T8-CTR-03 pass |
  | `host --root generated/roots/oec-dev-app --offline --tests T8-HST-03` | 1 | fail REQ-CNT-011: bad mounts the volume of app on `/etc/oi4/app`, named on both sides; REQ-CNT-024 pass |
  | the same after removing bad | 0 | pass |

  No mount source appears in any report (checked by the test). Afterwards both containers and both volumes were removed (`docker ps -a`, `docker volume ls`: no `oec-live-*`), the trap of `live.sh` ran `docker compose down -v`, and `oec/dev/generated/` is unchanged (`git status` clean).
- **The dev broker and REQ-CNT-019:** Mosquitto is a third-party broker of the platform, not an "OI4 Alliance-compliant container" (RIG 4.2), so RIG 4.2 does not apply to it. This is not a deviation of `oec/dev` and not an SF; `oec/dev` stays unchanged (A-279).

## Agent decisions (G-3.4 b, for information: REVIEW A-271 … A-280)

- **A-271 Engine API client:** own read-only client on the standard library (`GET` only, five endpoints, unversioned paths, response limit 32 MiB); Unix socket, Windows named pipe (`syscall.CreateFile`), plain TCP; `ssh://`, `https://` and other schemes are `ErrUnsupported` (inconclusive), a `tcp://` daemon that requires TLS is not reached (no TLS client in this cycle); the endpoint order of A-266 (moved from `host`), and a default socket that does not exist is skipped; `engine.Connect` is shared by `container` and `host`. Self-test against `enginetest` with recordings from Docker 28.4; the Podman shapes are synthetic; the Windows pipe is not run live.
- **A-272 Command `oec-infra container`:**
  - arguments: names, ids or unique id prefixes;
  - flags: `--daemon`, `--timeout` (10 s), `--level` (default 3), `--tests`, `--allow-skip`, `--format`, `--report`, `--config`;
  - an unknown container is a usage error (exit 2); no engine leaves every requirement inconclusive (exit 4);
  - report key `container` (additive, report version 1): engine and containers with networks and mounts (destination, type, mode). It never contains a source, the environment or labels;
  - new evidence field `container`.
- **A-273 T8-CTR-01:**
  - a bridge network passes;
  - the host network violates REQ-CNT-023 (SHOULD → warn) and is a *warning* on REQ-CMP-031: the MUST refers to RIG 4.4, which allows the host network for low-level Ethernet communication, and D8 cannot see that need;
  - `none`, user-mode networks (`pasta`, `slirp4netns`) and other drivers (`macvlan`, `ipvlan`, overlay) are notes: RIG 4.4 only compares bridge and host;
  - `container:<id>` is followed one step;
  - an unreadable network is inconclusive.
- **A-274 T8-CTR-02:** the name is one RFC 1123 label (letters, digits, hyphens, not at the start or end, at most 63 characters). It must be unique case-insensitively among all containers of the engine; other engines of the platform are not seen. The hostname must equal the name exactly. `-<uuid>` is the 8-4-4-4-12 hex form, and the default name before it has at most 26 characters (REQ-CNT-020, SHOULD).
- **A-275 T8-CTR-03:** a bind mount or a tmpfs on `/opt/oi4/licenses`, covering it or below it fails REQ-CNT-017. A named or anonymous volume is `manual`: REQ-CNT-018 allows a volume populated from the image, and D8 cannot see where the content came from. An image mount is a note; another mount type is `manual`; no mount passes.
- **A-276 T8-HST-03 scope:**
  - an OI4 container has a mount on, below or above `/etc/oi4`, `/opt/oi4` or `/run/secrets/oi4` (above: e.g. `/run/secrets`, never `/`);
  - every container of the engine counts, also stopped and created ones (their mounts exist), and the evidence notes the state;
  - `--self` (name, id or a prefix of at least 3 characters) excludes the container D8 runs in. There is no auto-detection: the hostname or the cgroup path is not reliable across engines. An unknown `--self` is inconclusive.
- **A-277 Sharing:**
  - only bind and volume mounts are shareable;
  - two pieces overlap if they are the same named volume or one source lies within the other, compared as the engine reports them – no symlink resolution and no `/host_mnt` normalization;
  - a container that binds a whole host tree (e.g. a monitoring agent with `/`) counts as sharing – correct by REQ-CNT-024/011, but it may surprise.

  The evidence names the other container and its destination, never a source.
- **A-278 REQ-CNT-011:** the requirement applies only "if the platform provides a persistent layer". D8 takes that as given when some OI4 container persists `/etc/oi4/app` or `/opt/oi4/app` with a bind or volume; otherwise the requirement is skipped. When it applies:
  - a missing application mount of an OI4 container is `manual` (the application may not need one);
  - a tmpfs or image mount on it fails ("no host-defined path");
  - a shared one fails.

  REQ-CNT-024 is skipped when no OI4 container has a secret storage.
- **A-279 Dev broker naming:** the dev broker fails REQ-CNT-019 in `container` mode (hostname `mosquitto`, compose name `oec-dev-mosquitto-1`). It is no OI4 Alliance-compliant container (RIG 4.2), so this is not a deviation and not an SF; `oec/dev` is unchanged.
- **A-280 Live test and cut:** `TestContainerDev` in `live.sh`; the throw-away containers and volumes are named `oec-live-*` and removed by the test. Increment 5 (`run`) is not pulled forward: it needs purpose-built test images and its own live harness (`live-infra.sh`), which is a cycle of its own (C-074).

## Changes to reviewed items

- Host scenarios `none` and `T8-HST-02/certificate` (reviewed in C-072): they now also run T8-HST-03, against a fake engine with `app` and `app2`. Their expectations are unchanged (T8-HST-03 passes), but they are back to `draft` (R-1), because the content of a reviewed fixture changed.
- `internal/host` (reviewed behaviour of T8-HST-04 REQ-PLT-006): the daemon endpoints and the probe moved to `internal/engine` without a change of behaviour. The 53 other host scenarios are unchanged and stay `reviewed`.
- D7: only the shared package `verdict` gained the optional `Evidence.Container` (never set by D7, omitted when empty); `report.schema.json`, the 55 D7 tests, sim fixtures, replays and `COVERAGE.md` are unchanged (gate).

## Spec findings

None. RIG 4.1.4, 4.2, 4.4 and MG 6.4 define what T8-CTR-01 … 03 and T8-HST-03 judge. What D8 cannot see is a limit of observation, not a gap in the definition, and is graded `manual` or as a note:
- the need for low-level Ethernet communication;
- a volume populated from the image;
- other engines of the platform;
- whether an application needs an application storage.

Next free SF-117.

## Dependencies

No new dependency (standard library only: `net/http`, `net`, `syscall` for the named pipe). `golang.org/x/sys` and `go-winio` are not needed (ADR-0025 amendment C-073 item 1).

## Risks and open technical points

- The Windows named pipe and Podman are not exercised against a real engine (A-271). They should be checked once a Windows or Podman host is available (P5b exit run or CI in P9).
- During the recording I ran `docker volume prune -f` on this Mac's Docker Desktop to remove the recording volumes. On Docker 28 it removes **all unused anonymous volumes** of the engine, not only those of the recording. Named volumes are not affected. If another project relied on an anonymous volume of a stopped container, it is gone. Later cleanups remove only the named `oec-live-*`/`oec-rec-*` volumes.

## Commits

| Commit | Content |
|---|---|
| `6728917` | `internal/engine` + `enginetest`, recordings, host on the engine package |
| `4e65008` | `internal/container`, `oec-infra container`, report/schema container mode, 26 container scenarios, catalog T8-CTR-01 … 03 |
| `d2bcb35` | T8-HST-03 (`internal/host/storages.go`, `--self`), 14 host scenarios, `engine.Connect`, catalog T8-HST-03 |
| `feda3df` | live test `TestContainerDev`, `live.sh` |
| (this commit) | docs: DESIGN §2/§9/§10, README, ADR-0025 amendment C-073, REVIEW/REVIEW_DE, STATUS, ROADMAP, this log |

## Checks (final)

- `oec/check.sh`: "all checks passed" (final run before the docs commit).
- 14 of 17 D8 tests `implemented` (T8-IMG-01 … 04, T8-BUS-01 … 03, T8-HST-01 … 04, T8-CTR-01 … 03); 3 `planned` (T8-RUN-01 … 03). 55 D7 tests `implemented`. `COVERAGE.md` unchanged.
- Fixtures: 29 captures, 59 sim, 19 images, 29 bus scenarios – all `reviewed`. Host scenarios 69: 53 `reviewed`, 16 `draft`. Container scenarios 26 `draft`.
- No key, certificate or `prompt.md` committed; `oec/dev/generated/` clean; `legacy/` untouched.

## For the PO

- **R-1:** 26 container scenarios and 16 host scenarios (14 new T8-HST-03, plus `none` and `T8-HST-02/certificate`) to review.
- No decision (no SF).
- A-271 … A-280 for information.
- Please note the `docker volume prune -f` under "Risks".

## Handover (G-3.10)

New agent session for C-074. Increment 5 (`run`) is a new command with its own sandbox and test images; the context of C-073 is not needed beyond the docs. The start prompt is in `prompt.md` (not committed).

## Next

C-074 – P5b increment 5: `oec-infra run` with T8-RUN-01 … 03.
- Purpose-built test images (built at run time with `ocitest`, no registry).
- A sandbox with generated storages, hostname and a D8 MQTT test endpoint (`mqtttest` broker), reusing `internal/engine` (read-only) plus the few write calls `run` needs (create, start, stop, remove of its own containers only – DESIGN §9).
- `live-infra.sh` (opt-in).

## Addendum 1 – PO review: R-1 (2026-10-31)

PO answer (verbatim): "R-1 akzeptiert".

- **R-1 accepted:** the 26 container scenarios of `oec/conformance/testdata/containers/index.yaml` (5 positive) and the 16 host scenarios of `oec/conformance/testdata/hosts/index.yaml` (14 T8-HST-03 scenarios, `none`, `T8-HST-02/certificate`) are `reviewed`. All 69 host and 26 container scenarios are `reviewed`; no draft, nothing open.
- The notes of `none` and `T8-HST-02/certificate` say "re-reviewed in the C-073 review (R-1)" instead of "hence draft again"; expectations unchanged.
- Updated: REVIEW (R-1 under "Answered earlier"), REVIEW_DE, STATUS (last updated, C-073 review block, Next step, open questions, history row), README/DESIGN/ADR-0025/ROADMAP (status of the scenarios). Gate: `oec/check.sh` "all checks passed".
