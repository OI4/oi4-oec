# 2026-11-28 – Node-RED nodes for OEC

PO order (outside the cycles): "ich möchte in oi4-oec-demo nun noch einen
Node-Red Node haben. Den du komplett erstellen sollst. Ich habe mir folgendes
Gedacht. Man kann über Config Nodes Apps anlegen. Dann gibt es verschiedene
Nodes, die man verwenden zum publizieren oder auch um auf Get Anfragen zu
reagieren. NAtürlich für die einzelne Ressources getrennt. Wie genau das am
besten zu machen ist, darfst du frei selbst entscheiden". All decisions below
are agent decisions (G-3.4 b) and can be overruled. STATUS and the C-112 log
are not changed here.

## Decisions (agent, G-3.4 b)

- **Folder and package**: source `oec/integrations/node-red/` (ADR-0012
  amendment), npm package `@oi4/node-red-oec` 0.1.0 (release train,
  `TestReleaseVersion`), MIT, Node-RED ≥ 3.1, Node.js ≥ 18. Copied
  unmodified into `oi4-oec-demo/integrations/node-red/`.
- **JavaScript on mqtt.js, no SDK binding**: Node-RED nodes are JavaScript;
  the Rust SDK has no Node.js binding (a native addon per platform would
  make the palette install fragile). A pure-JS port of the Level 2 provider
  in the style of the Rust `oec-core` (sans-IO): `lib/oec.js` (protocol
  core), `lib/provider.js` (application and assets, answers), `lib/session.js`
  (MQTT session), `lib/broker.js` (container storage or set by hand). Only
  runtime dependency `mqtt`; `aedes`, `node-red` and
  `node-red-node-test-helper` are dev dependencies.
- **Nodes**: config nodes `oec-broker` (container storage per RIG 4.1.3 or
  set by hand), `oec-app` (one OEC application: ServiceType, MAM, heartbeat,
  Syslog level; several per flow and per broker) and `oec-asset` (an asset
  the application represents; a resource node with an asset acts for it as
  Source). One node per resource and DataSet: `oec-health`, `oec-data`
  (Data + Metadata, optionally writable), `oec-config` (one context,
  values in the node context), `oec-event` (Syslog or Generic). Consumer
  nodes: `oec-subscribe` (Pub of other applications, listed in the
  SubscriptionList) and `oec-get` (Get with all pages; temporary
  subscription not listed, SF-109).
- **Get answered by the application**: the nodes provide the values; the
  application answers `Get` of every resource from the last values, with
  pagination. So the flow needs no wiring to answer requests, and the
  answers are consistent with the lists.
- **Messaging Level 2**: MAM, Health, Profile, Config, Data, Metadata,
  Event, PublicationList, SubscriptionList; no ReferenceDesignation, no
  `NewDataSetWriterId`. `Set Data` (writable DataSets), `Set Config` and
  `Set PublicationList` (Syslog levels) are accepted automatically and sent
  to the node output; everything else is answered with `BadNotWritable` /
  `BadNotFound`.
- **DataSetWriterIds**: MAM 10, Health 11, Profile 12, PublicationList 13,
  Status 14 … 16, SubscriptionList 17, the same for every Source
  (T7-MON-08 keys the lists by resource only); every other DataSet gets a
  stable FNV hash id from 18 on.
- **Lists**: a Data DataSet is listed from its first value; removals are
  flushed with a 250 ms debounce (one PublicationList per deploy).
- **ClientId** in container mode: the container name (host name); further
  applications on the same broker node add a suffix.
- **Syslog levels** (REQ-TOP-024 … 027, like `oec-core`
  `provider/events.rs`): default threshold `Notice` (Debug never on by
  default, Informational off); the application lists all eight levels with
  `Config` `MODE_1`, a `Set PublicationList` switches `FILTER_4` ↔ `OFF_0`
  (in memory until the next deploy). Syslog events of assets are always
  published. The `oec-event` default severity is `Notice`.
- **Will**: renewed in the mqtt.js `reconnect` event, so its MessageId is
  fresh for every CONNECT (REQ-LIFE-016, found by T7-LIFE-01).
- **Checks outside the gate**: `check.sh` (npm ci on first use, unit and
  node tests); `OEC_D7=<oec-check>` adds the D7 live run with a pinned pass
  list, like the micro SDK.

## Done

- `oec/integrations/node-red/`: `lib/` (protocol core, provider, session,
  broker options, consumer and node helpers), `nodes/` (9 nodes with editor
  and help, IL preview over an admin endpoint), `examples/oec-demo.json`,
  `test/` (29 tests: protocol core, provider, broker options, the nodes and
  the example flow in Node-RED against aedes; opt-in D7 live run with an
  embedded Node-RED), `check.sh`, `README.md`, `LICENSE`, `package.json`,
  `package-lock.json`.
- `oec/README.md` row, ADR-0012 amendment, `TestReleaseVersion`.
- Copy into `oi4-oec-demo` with a section in its README (local commit, not
  pushed).

## Verification

- `./check.sh`: 28 pass, D7 skipped.
- **D7** (`OEC_D7=… ./check.sh`: `oec-check test --level 2 --peer
  --announce` with proxy and restart hook, go-testbroker of the Rust SDK,
  embedded Node-RED in container mode with application, asset, Data,
  writable Data, Config with a sensitive value, Syslog, Health): **PASS**,
  40 pass, 3 warn (SHOULD: T7-GET-03/04 – no ReferenceDesignation;
  T7-ACT-01 – no `NewDataSetWriterId`), 12 skip. The first runs found
  T7-MON-08 (one PublicationList writer id per Source), T7-LIFE-01 (Will
  MessageId created at the close, > 5 s before the reconnect) and SET-12
  (Syslog levels not configurable); all fixed as decided above. One run
  failed T7-MON-05 because the whole bus stalled for about 90 s on the Mac
  (presumably App Nap, see the HA log); the rerun passed.
- Node-RED 5 editor smoke test: the nine nodes load without error, the IL
  preview endpoint answers, the example flow is listed under *Import →
  Examples*.
- Gate `oec/check.sh`: all checks passed (`TestReleaseVersion` changed).

## Re-sync after a change of `oec/integrations/node-red`

From the workspace root:

```sh
src=oec/integrations/node-red
git -C oi4-oec-demo rm -r -q --ignore-unmatch integrations/node-red
rm -rf oi4-oec-demo/integrations/node-red
git ls-files -z "$src" | while IFS= read -r -d '' f; do
  d="oi4-oec-demo/integrations/node-red/${f#"$src"/}"
  mkdir -p "$(dirname "$d")"; cp -p "$f" "$d"; done
git -C oi4-oec-demo add -A
```

## Open for the PO

- **Push** of `oi4-oec-demo` (G-3.8).
- **npm publication** of `@oi4/node-red-oec` (scope `@oi4` on npmjs.com,
  then *Manage palette* finds it; the Node-RED flow library lists packages
  with the keyword `node-red`). Until then: `npm install <folder>` or a
  tarball of `npm pack`.
- A Community page in the guideline site for the nodes.
- Possible next steps: Messaging L3 (ReferenceDesignation,
  `NewDataSetWriterId`), `Set` acceptance under the control of the flow
  (instead of automatic), persistence of the Syslog levels, `Get Metadata`
  of an unknown DataSet (currently not answered).
