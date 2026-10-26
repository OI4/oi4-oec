# C-039 – Config schemas and `broker.json`

- **Date:** 2026-10-25
- **Phase:** P2 – JSON Schema (D1)
- **Baseline:** guideline `a42a132` at the start; re-baselined in this cycle on **`23f6cfd`** (PO merge of origin with the MG A3 formatting fix `30fea57`, id only, see §4). Register head `68141cb` (SF-100 opened, `findings/` only).
- **Start:** workspace `da7d0c9` (C-038 addendum 1 applied, all items `reviewed`)

## Intent

PO (chat, 2026-10-25): "Starte mit C-039" – the next cycle of ROADMAP P2 / STATUS "Next step" 1: **C-039 – Config + `broker.json`** (area RES-CONFIG with the gaps REQ-RES-CONFIG-010/011/015/018; `platform/broker.schema.json` for REQ-CNT-003), in the pattern of C-035 … C-038:

1. oracle checks for the payload REQs where missing; umbrellas per ADR-0019 §9;
2. resource module, entry schemas `pub-`/`get-`/`set-config`, `message.schema.json` dispatch, probes from P-83; the platform schema of `broker.json`; regenerate `dist/`;
3. vectors from V-PAY-240 (`draft`) so that every `schema`/`d7-validate` REQ of RES-CONFIG has a positive and a negative vector; `broker.json` vectors from RIG 4.1.1; projected gaps after the review;
4. architecture decisions as ADR-0019 amendment C-039 (`proposed`).

## Baseline check

All checks green at the start (`spectool lint`, `render -check`, `coverage -check`, `topic -check`, `inventory -check -rev a42a132`, `go test ./...`, `oec/schema/check.sh`): 437 REQs, 1398 inventory items, 239 payload and 147 topic vectors, 159 `schema`/`d7-validate` REQs with 19 gaps, entry schemas 26 of 29, 82 schema files, 518 cases, 0 disagreements.

## No split

The cycle stayed whole, as planned (decision by the agent, allowed by the PO). Config has 3 topic forms and one module (large, but one object model), and RES-CONFIG had only 4 gaps, of which only REQ-RES-CONFIG-015 needed new vectors. `broker.json` is one small file with one REQ (REQ-CNT-003); splitting it off would have cost a full cycle of overhead for one schema. It did need a new vector kind and an oracle package, but no change of an existing artifact beyond the harness.

## Work

### 1. Oracle, verification changes, umbrellas (`4e52e5c`, `10a7d8d`)

**REQ-RES-CONFIG-015 (unique Filter per Source).** A Pub Config with two Config DataSetMessages of the same Source (DataSetWriterName) and the same Filter (WriterGroupName) reported nothing; the oracle now reports REQ-RES-CONFIG-015. A Set keeps reporting only REQ-RES-CONFIG-019 (exactly one DataSetMessage). Three new cases in `payload_test.go` (two contexts valid, repeated context invalid, same context of another Source valid).

**ByteString with CR/LF (`10a7d8d`).** Go's `base64.StdEncoding` skips `\r` and `\n`, so the oracle accepted a ByteString Value with a line break while the schema pattern (REQ-RES-CONFIG-007) rejects it. `configValue` now rejects CR/LF; new test case "byte string newline" (fails without the fix). No reviewed vector changes its verdict.

**Verification changes (`draft`, R-1 and R-2):**

| REQ | Change | Reason |
|---|---|---|
| REQ-RES-CONFIG-011 (Mandatory objects in every Set of their group) | `d7-validate` → `d7-monitor` | A Set carries no Mandatory flags; whether a Set lacks a Mandatory object needs the last Pub Config of the addressed Source and Filter. Not decidable on one message. |
| REQ-RES-CONFIG-024 (Sensitive values masked in every Pub) | +`schema` | `pub-config` restricts `Value`/`DefaultValue` of an object with `Sensitive` = true to `""` or the mask `********` (probes P-93/P-94, reviewed negative V-PAY-115). |

**Umbrellas** (ADR-0019 §9, `coverage/implications.yaml`):

| Umbrella | Detail REQs | Remainder |
|---|---|---|
| REQ-RES-CONFIG-010 (unset by empty string) | REQ-TOP-013, REQ-RES-CONFIG-003, 007 | that the receiver unsets the value is responder behavior (d7-test, REQ-RES-CONFIG-023) |
| REQ-RES-CONFIG-018 (Pub content) | REQ-MSG-010, REQ-RES-CONFIG-002, REQ-PAG-008, REQ-LOC-003, REQ-QRY-001 | empty `Pub` only as `Get` answer (REQ-CMP-042, d7-monitor) |

Both are "derived" at once. Gaps 19 → **16** (−1 REQ-RES-CONFIG-011 leaves the count, −2 umbrellas); 158 `schema`/`d7-validate` REQs.

### 2. Config schemas (`8a26340`)

`resources/config.schema.json` (new module):

- **Pub family** (`Config`, `Context`, `Group`, `ConfigObject`, `Validation`, `ConfigDataSetMessage`; REQ-RES-CONFIG-002) and **Set family** (`ConfigSet…`; REQ-RES-CONFIG-003: a Set is a partial update, only `Value` is mandatory). Both closed (REQ-MSG-001).
- GroupName pattern (REQ-RES-CONFIG-004), Type enumeration Boolean/ByteString/DateTime/Number/String (REQ-RES-CONFIG-006).
- **Representation per Type** (REQ-RES-CONFIG-007, SF-084): Boolean `true`/`false`, Number in JSON number syntax, DateTime RFC 3339 syntax, ByteString Base64 without CR/LF, String any; the empty string is valid for every Type (unset); the mask is valid only with `Sensitive` = true (GL-ADR-021). The calendar validity of a DateTime is rule layer R-4.
- **Masking** (REQ-RES-CONFIG-024): in the Pub family a Sensitive object carries `Value`/`DefaultValue` only as `""` or `********`; a Set may carry the clear value.
- `ConfigDataSetMessage`: WriterGroupName = Context.Name (one topic-encoded level, REQ-MSG-024), `ua-keyframe`.

Entry schemas (3, now **29 of 29**):

| Entry schema | Content |
|---|---|
| `pub-config` | Config DataSetMessages + at most one Pagination; `"Messages": []` valid (SF-099); DataSetClassId const |
| `get-config` | only PaginationRequest and Locale (REQ-RES-CONFIG-016); Query → REQ-QRY-001 |
| `set-config` | exactly one Config DataSetMessage (REQ-RES-CONFIG-019, `contains` with `minContains`/`maxContains` 1); helpers rejected; REQ-BUS-010 |

Further changes:

- `message.schema.json` dispatches Config by DataSetClassId to the **Set** family: Pub and Set are indistinguishable without the topic, and the Set family is the superset (option A accepts every valid Pub and Set; probes P-107/P-108).
- Expectation **V-PAY-108** (reviewed vector, unchanged, routed for the first time): schema none, rule REQ-RES-CONFIG-008 (R-3) – a Value satisfies the Validation of its own configuration object; JSON Schema cannot use a sibling value as a constraint.
- The 30 reviewed Config vectors (V-PAY-088 … 116, 128) are now routed; every negative reports exactly its `violates` in the schema (V-PAY-108 by the expectation). All 241 payload vectors are routed to an entry schema.
- Probes **P-83 … P-108** (Config): empty `Pub` (valid), Context without Group, Group/Context without Name, missing Value, GroupName/ConfigName patterns, WriterGroupName with two levels, Type and representation per Type (incl. DateTime and ByteString edge cases), masking in Pub and Set, Validation types, Query in Get, SequenceNumber in Set, two `message` dispatch probes.
- `oec/schema/README.md`: state C-039, Config module, 29 of 29.

### 3. Config vectors (`de117e2`)

Only REQ-RES-CONFIG-015 needed vectors (the other three gaps were umbrellas or moved to `d7-monitor`).

| Vector | Base | Content | reqs / violates | Layer |
|---|---|---|---|---|
| V-PAY-240 | – | valid unsolicited Pub Config on `Pub/Config/<Source>` (no Filter) with two contexts "Network settings" (DataSetWriterId 42) and "Overall device settings" (43); a masked password `********` | reqs incl. CONFIG-015, CONFIG-024 | – |
| V-PAY-241 | 240 | the second DataSetMessage renamed to "Network settings" | CONFIG-015 | rule (R-3) |

Expectation V-PAY-241: schema none, rule REQ-RES-CONFIG-015 (R-3) – uniqueness across array items by a pair of keys is not expressible in JSON Schema.

### 4. Re-baseline on `23f6cfd` (`889c569`, `dd5405d`)

During the cycle the guideline branch head moved to `23f6cfd`: a PO merge of origin with Konrad Heidrich's commit `30fea57` (MG A3 table, `<sub>` formatting only). `spectool rebaseline -rev 23f6cfd` changed only the `baseline` ids (82 files); the inventory stayed at 1398 items, no key, quote or REQ changed. ADR-0001 amendment C-039, AGENTS.md, GUARDRAILS G-1.2 and the STATUS baseline line name `23f6cfd` (`dd5405d`). Information for the PO, nothing to decide.

### 5. `broker.json` (`1161973`, `3f40059`)

**Oracle** `oec/spec/tools/internal/platform` (new): `CheckBroker` reports REQ-CNT-003 if `broker.json` is not an object or lacks `Address`, `SecurePort` or `MaxPacketSize`, if `Address` is no IPv4 address, IPv6 address or host name, if `SecurePort` is not an integer 1 … 65535 or `MaxPacketSize` not an integer 262144 … 268435455. The address patterns are built from RFC 3986 (`IPv4address`, `IPv6address`) and RFC 1123 (host name labels); a unit test compares the IPv6 pattern with `net.ParseIP`.

**SF-100 (open, register `68141cb`).** RIG 4.1.1 leaves open (a) whether further elements are allowed, (b) the syntax of `Address` (IPv6 with brackets/zone? host name with `_`, trailing `.`, port, URL?) and (c) whether `SecurePort` 0 is allowed. Proposed: (a) allowed, clients ignore them; (b) IPv4 dotted-decimal, IPv6 without brackets/zone, RFC 1123 host name whose last label is not all digits, no port/scheme; (c) 1 … 65535; with proposed RIG text. REQ-CNT-003 stays unchanged until the decision; oracle, schema and vectors implement the proposal (D-2).

**Vector kind `broker-json`** (`V-BRK-xxx`, field `content`, `oec/spec/vectors/platform/broker-json.yaml`; `vectors.schema.json`, lint, review view `vectors/view/platform-broker-json.md`). 18 constructed vectors (RIG 4.1.1 has no example), all `draft`:

| Vectors | Content |
|---|---|
| V-BRK-001 … 004 (valid) | host name `mqtt-broker`/8883/262144; IPv4 with MaxPacketSize 268435455; IPv6 `fd00:10::5`/443; FQDN with a further element `Comment` (SF-100 (a)) |
| V-BRK-005 … 007 | `Address`, `SecurePort`, `MaxPacketSize` missing |
| V-BRK-008 … 010 | MaxPacketSize 262143, 268435456, as string |
| V-BRK-011 … 013 | SecurePort 65536, as string, 0 (SF-100 (c)) |
| V-BRK-014 … 016 | Address with port, as URL, empty (SF-100 (b)) |
| V-BRK-017, 018 | camelCase element names; an array instead of an object |

Every invalid vector violates REQ-CNT-003 (the only REQ of the file).

**Schema** `oec/schema/v2/platform/broker.schema.json` (new, part of the bundle): `required` Address/SecurePort/MaxPacketSize; `Address` `anyOf` IPv4 / IPv6 / host name (`maxLength` 253, `not` numeric last label); integer ranges; no closure (SF-100 (a)), so strict and tolerant are equal. The patterns are identical to the oracle's (compared programmatically).

**Harness** (`schematest`): loads `vectors/platform/`, validates every `content` against the broker schema (valid passes, invalid fails with exactly `violates`), writes the cases to `dist/cases.json`; the `$comment` tracing accepts `d8` instead of `schema` in `platform/` files (REQ-CNT-003 keeps `[d8]`, D8 is the consumer); probes **P-109 … P-128**: IPv6 loopback, IPv4-mapped IPv6 (valid), IPv6 in brackets, with zone, with two `::`, IPv4 octet 256 and leading zero, host name with `_`, trailing `.`, label starting with `-`, label of 64 characters (invalid), label of 63 characters and host name of 253 characters (valid), 254 characters (invalid), label starting with a digit and `localhost` (valid), SecurePort 65535 (valid) and 443.5, Address `null`, negative MaxPacketSize (invalid). Cross-check (temporary, not committed): oracle and schema agree on all 38 broker cases (18 vectors + 20 probes).

REQ-CNT-003 has verification `d8`, so the V-BRK vectors do not count in the `schema`/`d7-validate` coverage; the coverage report is unchanged by them.

### 6. ADR-0019 amendment C-039 (`a1b68c1`, `proposed`)

1. §2 Config Pub/Set def families.
2. §3 Value representation per Type and masking in the schema; DateTime calendar R-4; ByteString CR/LF oracle tweak.
3. §4 topic-agnostic Config dispatch with the Set family.
4. §5/§8 rule layer: REQ-RES-CONFIG-008/015 R-3; REQ-RES-CONFIG-011 → d7-monitor.
5. §8 Config entry schemas (29 of 29).
6. §9 umbrellas REQ-RES-CONFIG-010/018.
7. §7 platform module `broker.json` (SF-100 proposed interpretation).
8. §1 `$comment` tracing exception: `d8` in `platform/` files.
9. §8 vector kind `broker-json`, probes on broker bases.

No new ADR. One new spec finding (SF-100).

## Changes to reviewed items

| Item | Change | Reason |
|------|--------|--------|
| REQ-RES-CONFIG-011 | verification `d7-validate` → `d7-monitor`; `draft` | §1; R-1. |
| REQ-RES-CONFIG-024 | verification +`schema`; `draft` | §1; R-2. |
| ADR-0019 (accepted) | amendment C-039 (`proposed`) | §6; D-1. |
| Oracle `configValue`, `content.go` | ByteString CR/LF rejected; REQ-RES-CONFIG-015 reported | §1; no reviewed vector changes its verdict; part of D-1. |
| `coverage/implications.yaml` | two new umbrellas | §1; ADR-0019 §9 (accepted rule), part of D-1. |
| `tests/rule-layer.yaml` | expectation for the reviewed V-PAY-108 | mechanical: routed for the first time; split per ADR-0019 §5, part of D-1. |
| catalog `baseline` ids | `a42a132` → `23f6cfd` | mechanical re-baseline (§4). |

No vector or matrix row changed.

## Coverage

- **Now** (drafts not counted): 158 `schema`/`d7-validate` REQs, **16 gaps** (19 → 16: REQ-RES-CONFIG-011 → `d7-monitor`, umbrellas REQ-RES-CONFIG-010 and 018 derived). Entry schemas **29 of 29**. 241 payload vectors, 147 topic vectors, 18 broker-json vectors.
- **After the PO review of this cycle** (V-PAY-240/241 reviewed): **15 gaps** (REQ-RES-CONFIG-015 covered). RES-CONFIG has no gap left.
- Left for C-040 (15): CMP 4 (004, 005, 025, 026), IL 4 (001, 004, 005, 008), TOP 7 (002, 009, 011, 012, 018, 020, 021).
- Accepting R-1 does not change the projection (REQ-RES-CONFIG-011 already left the count); rejecting R-1 would bring it back as a gap needing a rule-layer vector based on the published Config.

## Commits

Workspace:

| Commit | Content |
|--------|---------|
| `4e52e5c` | oracle REQ-RES-CONFIG-015; REQ-RES-CONFIG-011/024 verification (draft); umbrellas CONFIG-010/018; coverage |
| `10a7d8d` | oracle: ByteString without CR/LF |
| `8a26340` | `resources/config`, `pub-`/`get-`/`set-config`, `message` dispatch; expectation V-PAY-108; probes P-83 … P-108; README |
| `de117e2` | V-PAY-240/241; expectation V-PAY-241; views, coverage |
| `889c569` | re-baseline on `23f6cfd` (ids only) |
| `dd5405d` | ADR-0001 amendment C-039, AGENTS, GUARDRAILS, STATUS baseline |
| `1161973` | oracle `internal/platform`, vector kind `broker-json`, V-BRK-001 … 018, views, spec README |
| `3f40059` | `platform/broker.schema.json`, harness routing, `d8` tracing exception, probes P-109 … P-128, README |
| `a1b68c1` | ADR-0019 amendment C-039 (`proposed`) |
| (docs, after `a1b68c1`) | this log; ROADMAP, STATUS, REVIEW, REVIEW_DE |

Guideline repo: `68141cb` (register: SF-100 opened; `findings/` only, no re-baseline).

## Checks (final)

- `spectool lint` (437 REQs, 241 payload, 147 topic, 18 broker-json vectors), `render -check`, `coverage -check`, `topic -check`, `inventory -check -rev 23f6cfd` (1398 items), `go test ./...`: green.
- `oec/schema/check.sh` (gofmt, vet, `schematest`, jsonschema-rs): 92 schema files (46 strict + 46 tolerant), 628 cases (241 payload vectors routed, 18 broker-json vectors, 128 probes), 0 disagreements between santhosh and jsonschema-rs.
- Python `jsonschema` + `referencing` (third validator, informative): 92 schemas, 628 cases, 0 disagreements.

## For the PO (review)

Collected in [`REVIEW.md`](../REVIEW.md) (German explanation: [`REVIEW_DE.md`](../REVIEW_DE.md)):

1. D-1 ADR-0019 amendment C-039.
2. D-2 SF-100 (a)/(b)/(c) – content of `broker.json`.
3. R-1 REQ-RES-CONFIG-011 → `d7-monitor`; R-2 REQ-RES-CONFIG-024 +`schema`.
4. R-3 V-PAY-240/241 (Config); R-4 V-BRK-001 … 018 (`broker.json`).
5. Info: re-baseline on `23f6cfd`.

Next free ids: SF-101, REQ-CMP-048, V-PAY-242, V-TOP-254, V-BRK-019, ADR-0020, Q-021, probe P-129. Next cycle: C-040 – D7 `validate` (ROADMAP P2).

## Addendum 1 – PO review (2026-10-26)

PO (chat, 2026-10-26): "R1 - R4 reviewed und akzeptiert · D-1 D-2 reviewed und akzeptiert · D-2 bitte in Guideline einbauen". Recorded in [`REVIEW.md`](../REVIEW.md) (commit `4ad2f96`, kept as the review record): D-1 ADR-0019 amendment C-039 **accepted**; D-2 SF-100 **decided as proposed**, with the order to implement the RIG text (addendum 2); R-1 … R-4 **accepted**.

Review applied (`dd5893d`, `spectool review`):

- 22 drafts → `reviewed`: REQ-RES-CONFIG-011 (`d7-monitor`), REQ-RES-CONFIG-024 (+`schema`), V-PAY-240/241, V-BRK-001 … 018.
- Coverage (`coverage/vectors.md`): `schema`/`d7-validate` gaps 16 → **15** (as projected); Config has no gap. The 15: REQ-CMP-004/005/025/026, REQ-IL-001/004/005/008, REQ-TOP-002/009/011/012/018/020/021.

ADR-0019 (`d7520ba`): status line and amendment C-039 heading say accepted; item 7 states that `broker.schema.json` implements SF-100 as decided and implemented (`168e1d1`).

No reviewed item changed beyond the accepted proposals.

## Addendum 2 – SF-100 in the guideline, re-baseline on `168e1d1` (2026-10-26)

Guideline (`oi4-oec-documentation`, branch `feature/spec-findings`):

- `168e1d1` – RIG 4.1.1, the text proposed in the register:
  - `Address`: value range "`<IPv4 address, IPv6 address (without brackets) or host name (RFC 1123)>`"; the description defines the forms – "An IPv4 address is written in dotted-decimal form without leading zeros, an IPv6 address in the text form of RFC 4291 without brackets and without zone index. A host name consists of labels of 1 to 63 letters, digits and `-`, separated by `.`; a label does not start or end with `-`, the last label is not all digits, and the host name has at most 253 characters and does not end with `.`. `Address` contains no port, no scheme and no path." The register proposal named these rules under (b) but its text proposal changed only the value range; the description makes the decided rules readable in the guideline and quotable by REQ-CNT-003 (agent choice within the decision, R-3 below).
  - `SecurePort`: value range "`1 … 65535`" (type UINT16 unchanged).
  - After the element list: "Clients ignore elements of `broker.json` they do not know."
  - No GL-ADR (like SF-099: a decided finding without protocol change). `yarn build` green.
- `7808df7` – register: SF-100 `resolved (168e1d1)`, moved to Closed; no open finding; next SF-101.

Re-baseline (`b70eb87`; `spectool rebaseline -rev 168e1d1` with `168e1d1` checked out detached, because the register commit `7808df7` is the branch head and touches only `findings/`; back on the branch afterwards):

- Inventory 1398 → **1399** items: + RIG-4.1.1-77f0d05d "Clients ignore elements of `broker.json` they do not know." (implicit, term `ignore`; the inventory files it under the heading of the last card, `MaxPacketSize`). The value ranges and the new `Address` description carry no inventory item; no key changed, only line numbers.
- **REQ-CNT-003** (reviewed item, changed; reason: guideline text of SF-100): statement spells out the `Address` forms, `SecurePort` 1 … 65535 and "It may contain further elements."; new quotes "Address contains no port, no scheme and no path." and "Clients ignore elements of broker.json they do not know."; + RIG-4.1.1-77f0d05d; refs + SF-100; note names the schema, the oracle and V-BRK. → `draft` (R-1). Verification stays `[d8]`.
- **REQ-CNT-004** (reviewed item, changed; same reason): statement + "It ignores elements of broker.json it does not know."; quote; + RIG-4.1.1-77f0d05d (an inventory item may back several REQs: the platform may add elements – REQ-CNT-003 –, the application ignores them – REQ-CNT-004); refs + SF-100. → `draft` (R-2). Levels and verification `[d8, manual]` unchanged.
- Unlike REQ-CMP-041 in C-036 addendum 2 (kept `reviewed`), both REQs go back to `draft`, as the hand-over rule for changed reviewed items says; neither is `schema`/`d7-validate`, so coverage is unaffected.
- Mechanical (no content or expectation change, items stay `reviewed`): "SF-100 …, proposed" → "SF-100 …" in the V-BRK reasons and notes, the description of `vectors/platform/broker-json.yaml` (+ SecurePort 1 … 65535, "RIG 4.1.1 since guideline 168e1d1"), the comments of `internal/platform`, `oec/spec/README.md`; in `oec/schema` (`4a4461e`) the description and `$comment`s of `platform/broker.schema.json`, the probe comment and `oec/schema/README.md`. Constraints, oracle and vectors unchanged.

Docs: ADR-0001 amendment C-039 addendum 2; AGENTS.md, GUARDRAILS G-1.2, STATUS, ROADMAP baseline `168e1d1`; `REVIEW.md` / `REVIEW_DE.md` R-1 … R-3 (REQ-CNT-003, REQ-CNT-004, guideline text).

Checks: `spectool lint` (437 REQs, 2 drafts; a deliberately wrong quote was rejected), `render -check`, `coverage -check` (gaps 15, CNT open=0), `topic -check`, `inventory -check -rev 168e1d1` (1399 items), `go test ./...`, `oec/schema/check.sh` (92 schema files, 628 cases, 0 disagreements), Python validator (628 cases, 0 disagreements): green.

Next free ids: SF-101, REQ-CMP-048, V-PAY-242, V-TOP-254, V-BRK-019, ADR-0020, Q-021, probe P-129. Next cycle: C-040 – D7 `validate` (ROADMAP P2).
