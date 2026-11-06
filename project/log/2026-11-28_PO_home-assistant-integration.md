# 2026-11-28 – Home Assistant integration for OEC (HACS)

PO order (outside the cycles): "Überlege ob es möglich wäre für die OEC eine
Homeassistant Integration zu entwickeln, also dass es über HACS installierbar
wäre. Falls es machbar ist, baue dazu einen Entwicklung in oi4-oec-demo
(Subfolder darfst du entscheiden, demos passt nicht so recht finde ich.
Vielleicht Integrations? Node-Red könnte als nächstes folgen)". The PO was not
available during the work: all decisions below are agent decisions
(G-3.4 b) and can be overruled. STATUS and the C-112 log are not changed here.

## Feasibility

Feasible. A HACS integration is a Home Assistant custom integration
(Python) in `custom_components/<domain>/` of a public GitHub repository
with a `hacs.json`. Home Assistant has no OEC support; its MQTT integration
cannot be used, because an OEC application needs its own MQTT 5 session
(ClientId, Will, TLS with the CA and the credentials of the OEC platform)
and must answer requests on the bus. So the integration runs its own MQTT
client (aiomqtt, also used by Home Assistant core) and is itself an OEC
application.

## Decisions (agent, G-3.4 b)

- **Folder**: source `oec/integrations/home-assistant/` (ADR-0012
  amendment), Node-RED later as `oec/integrations/node-red/`. Copy into
  `oi4-oec-demo`: HACS needs `custom_components/oi4_oec/` and `hacs.json`
  at the repository root (`content_in_root: false`); the rest goes to
  `integrations/home-assistant/`.
- **OEC role**: ServiceType `ITConnector` (MG 7.2: connects OT data to an
  IT system), Messaging Level 2, Oi4Identifier
  `oi4.org/OEC,20Home,20Assistant/OEC-HOME-ASSISTANT/<HA instance id>`.
  Read only: it offers nothing for modification.
- **Set/Del and unknown Source**: every `Set`/`Del` to its AppId gets the
  `Event` `Status`/`Bad` (`BadNotSupported`; `BadNotFound` for an unknown
  `Source`), a `Get` for an unknown `Source` `BadNotFound` (MG 9.1/9.2,
  SF-099/108). The Set/Del requirements are conditional, but D7 cannot
  know that the integration offers nothing and sends them: answering costs
  little and makes the behaviour explicit. `Profile`: `MAM`, `Health`,
  `Profile`, `Event`.
- **Home Assistant model**: a service device for the integration; a
  device per application (via the integration) and per represented asset
  (via its application); binary sensor *Online* per application (Birth,
  Close, Will); sensors *Health*/*Health score* per asset; a (binary)
  sensor per scalar `Data` field with name, unit and device class from the
  `Metadata`.
- **Authority**: an application is the authority for itself. Entries
  about it by other applications are ignored; of a `Registry` only its own
  entries are used. Found in the smoke test: the OEC Registry answers
  `Get MAM`/`Health` with the entries of the whole bus (incl. the
  integration itself), which made the Registry the parent of every
  application and the integration a device of itself (via loop).
- **Minimum Home Assistant 2026.10** (`hacs.json`): the test harness pins
  2026.10.4 (pytest-homeassistant-custom-component 0.13.375, Python 3.14);
  `via_device_id` (`via_device` is deprecated) exists there.
- **Close on shutdown**: Home Assistant cancels the background tasks of
  config entries *before* it fires `EVENT_HOMEASSISTANT_STOP`; a listener
  there came too late (the broker sent the Will). The integration uses a
  shutdown job (`hass.async_add_shutdown_job`), which runs before.
- **Checks outside the gate**: `check.sh` (stdlib unittests); with
  `OEC_HA_FULL=1` the Home Assistant tests and hassfest (from the Home
  Assistant core source of the pinned version: the image
  `ghcr.io/home-assistant/hassfest` was refused by the local Docker
  daemon although anonymous pulls work with curl). `TestReleaseVersion`
  pins `const.VERSION` and `manifest.json` to `oec/VERSION`.
- **Version and docs URL**: 0.1.0 (release train); documentation
  `https://github.com/OI4/oi4-oec-demo/tree/HEAD/integrations/home-assistant`
  (`HEAD` follows the default branch `development`).

## Done

- `oec/integrations/home-assistant/`: `custom_components/oi4_oec/`
  (protocol core `oec.py`, bus state `model.py` – both standard library
  only –, MQTT session `bus.py`, Home Assistant glue, translations en/de,
  brand icons), `hacs.json`, `tests/` (22 unittests), `tests_ha/`
  (12 Home Assistant tests: config flow incl. errors and duplicate,
  discovery with devices/entities/names/units, Close/Birth availability,
  connection loss, unload, shutdown, diagnostics redaction),
  `tools/run_bus.py` (bus part without Home Assistant), `check.sh`,
  `README.md`.
- `oec/README.md` row, ADR-0012 amendment, `TestReleaseVersion`.
- Copy into `oi4-oec-demo` with a section in its README (local commit, not
  pushed).

## Verification

- `./check.sh` and `OEC_HA_FULL=1 ./check.sh`: 22 unittests, 12 Home
  Assistant tests, hassfest "Invalid integrations: 0".
- **D7** (`oec-check test --level 2` with proxy, restart hook and peer;
  the bus part `tools/run_bus.py` in a Linux container as the CUT):
  **PASS**, 33 pass, 4 warn (SHOULD: T7-MON-13, T7-GET-03/04 – no `Data`,
  `Config`, `PublicationList`, `SubscriptionList`,
  `ReferenceDesignation`; T7-ACT-01 – no `NewDataSetWriterId`), 18 skip.
  The first runs found the missing Set/Del status events (decided above).
- **Real Home Assistant 2026.11.0** (container on `oec-dev_default`,
  onboarding and config flow over the REST API): wrong password →
  `invalid_auth`; entry loaded; devices for the integration, the
  applications of `oec/dev` (weather, config, logger, registry), the
  weather stations and the ESP32 DHT22, 40 entities, e.g.
  station temperature °C/`temperature`, pressure hPa/`pressure`, humidity
  %/`humidity`, wind km/h/`wind_speed`, the ESP32 temperature and humidity;
  Close (`NORMAL_0`, `HealthScore` 0) on the bus when Home Assistant stops.
- macOS note: a background Python process on the host had its timers
  delayed by minutes (App Nap) and missed the 60 s heartbeat; in a Linux
  container the timing is exact. Two clients with the same serial number
  take over each other's MQTT session (same ClientId).
- Gate `oec/check.sh`: all checks passed (`TestReleaseVersion` changed).

## Re-sync after a change of `oec/integrations/home-assistant`

From the workspace root:

```sh
src=oec/integrations/home-assistant
git -C oi4-oec-demo rm -r -q --ignore-unmatch custom_components hacs.json integrations/home-assistant
rm -rf oi4-oec-demo/custom_components oi4-oec-demo/hacs.json oi4-oec-demo/integrations/home-assistant
git ls-files -z "$src" | while IFS= read -r -d '' f; do
  r="${f#"$src"/}"
  case "$r" in custom_components/*|hacs.json) d="oi4-oec-demo/$r" ;; *) d="oi4-oec-demo/integrations/home-assistant/$r" ;; esac
  mkdir -p "$(dirname "$d")"; cp -p "$f" "$d"; done
git -C oi4-oec-demo add -A
```

## Open for the PO

- **Publication**: `OI4/oi4-oec-demo` is private (GitHub API, 2026-11-28;
  the log 2026-11-28_PO_demo-repo calls it public). HACS installs only
  from public repositories: make it public and push (G-3.8, Q-011).
- HACS default store (optional, later): repository description and topics,
  GitHub releases, a workflow with `hacs/action` and hassfest in
  `OI4/oi4-oec-demo`; then a pull request to `hacs/default`. A custom
  repository works without them.
- A Community page in the guideline site (`docs/community`) for the
  integration.
- Next: Node-RED nodes in `oec/integrations/node-red/`.
- Possible next steps of the integration: `Set` of `Config` (e.g. as
  Home Assistant number/select entities), reauthentication and
  reconfiguration flows, Messaging L3 (`PublicationList`,
  `SubscriptionList`).

## Addendum 1 – publication and install test with the Home Assistant Marketplace

PO: "Ist nun public. Für einen Test kannst du lokal auf meinem Mac gerne im
Docker HA installieren". `OI4/oi4-oec-demo` is public (GitHub API). On PO
approval (ask_user) the agent pushed `development` (`3c4df4a`; GitHub
reported the bypassed rule "Changes must be made through a pull request",
admin rights of the PO account).

Findings:

- **HACS is replaced** in Home Assistant 2026.11 by the built-in
  **Marketplace** (`get.hacs.xyz` refuses to install on 2026.11). The
  Marketplace keeps the repository format (public repository, `hacs.json`,
  `custom_components/<domain>/`); adding a custom repository needs a GitHub
  connection (device code, done by the PO).
- The Marketplace (like HACS) installs the **latest release**: the first
  GitHub release without the pre-release flag in the order of the GitHub
  API. The API orders by the version in the tag name, not by date: the
  release `v1.1.0` (OEC 1.x, target `main`) came before a newer release
  `oec-v0.1.0` (marked *Latest*), and adding the repository failed with
  "no content the Marketplace can install in version v1.1.0". A release
  with the GitHub pre-release flag is skipped (unless the beta switch of
  the repository is on), so it cannot be used either.
- PO decision (ask_user): "v2.0.0 aber mit Snapshot oder Prerelease
  vermerk". Release **`v2.0.0-snapshot.1`** on `3c4df4a` (normal release,
  *Latest*, SemVer pre-release suffix in the tag; the release `oec-v0.1.0`
  and its tag were deleted). GitHub now lists it before `v1.1.0`.

Test (Home Assistant 2026.11.0 in docker, network `oec-dev_default`,
`127.0.0.1:8123`): Marketplace *Add from link* `OI4/oi4-oec-demo`, type
integration → available version `v2.0.0-snapshot.1`, domain `oi4_oec`;
*Install* wrote `custom_components/oi4_oec` identical to the release; the
config flow (host `mosquitto`, credentials of `oec-dev-app`, CA) created
the entry without a restart. After the 60 s heartbeats: 40 entities of the
integration, the weather service with 3 stations, config, logger, registry
and the ESP32 DHT22 (`°C`, `hPa`, `%`, `km/h`, `°`). The applications appear
with their first publication (no bus-wide request in OEC), now stated in the
README. The README describes the install with the Marketplace (2026.11+) and
HACS (2026.10), and the latest-release rule.

Open for the PO: a later release of the integration must sort above
`v2.0.0-snapshot.1` (e.g. `v2.0.0-snapshot.2`, `v2.0.0`), otherwise the
Marketplace keeps installing the old one.
