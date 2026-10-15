# Guideline digest (v2.0, baseline `de852d6`)

This is a compact, implementation-oriented summary of the two v2.0 guidelines in `oi4-oec-documentation`:
- **MG**: OEC Messaging Guideline, `docs/guideline/`
- **RIG**: OEC Runtime & Infrastructure Guideline, `docs/infrastructure-guideline/`

It also covers the guideline ADRs **GL-ADR-001 … 016** in `rfcs_adrs/` and the spec-finding resolutions SF-001 … 039 (branch `feature/spec-findings`). The **OEC Process Description** (`docs/process-description/`, former MG 6) is informative and not summarized here. All section numbers use the numbering of `de852d6` (unchanged since `49b6fe2`) (MQTT 6, Topics 7, Payload 8, Message Bus 9). The digest is **not** normative; always check the referenced section. Known problems are listed in `../SPEC-FINDINGS.md`. Previous digests (baselines `c85ce2a`, `5f070bb`, `7bead4c`, `49b6fe2`, `5cb109e`) are in the git history of this file.

## 0. What changed with the guideline ADRs (vs. `c85ce2a`; 012/013 came with `7bead4c`, 014–016 with `49b6fe2`)

| GL-ADR | Change | Impact for us |
|--------|--------|---------------|
| 001 | Health gets optional `Details[]` (`DetailObject`: `Health`, `DiagnosticCode`, `Location`, `Description`); `Health` = most severe status | schema, SDK health API, registry UI |
| 002 | No license resources; SBOM (SPDX/CycloneDX) + license texts in `/opt/oi4/licenses` inside the image (L2+) | our images generate SBOMs; D7 scope (Q-013) |
| 003 | Topic namespace **`Oi4v2`** (v1 stays `Oi4`); requests are answered in the same namespace | topic grammar, everywhere |
| 004 | DataSetMessage `Source` → **`DataSetWriterName`**, `Filter` → **`WriterGroupName`** (both mandatory, `""` if no filter); topic elements keep their names | schema, SDK codec |
| 005 | NetworkMessage `CorrelationId` → **`ReplyTo`** (mandatory, `""` if none); also in `ua-metadata` (only OI4-specific key) | schema, SDK correlation |
| 006 | OPC UA **1.05.04** JSON with **fixed content masks** (MG 8.1.4); `Status` is an object; `MinorVersion` xor `MetaDataVersion`; DataSetMessage `MessageType`; `Messages` is always an array | schema, codec |
| 007 | `MSG` → OPC UA **Actions** (`ua-action-request` / `ua-action-response`); `Call`/`Reply` remain as topic methods | schema, SDK services |
| 008 | Removed License, LicenseText, RtLicense, Interfaces, SBoM, NE107 event; chapters renumbered | = our ADR-0005 |
| 009 | **Compliance domains** (Messaging / Service Container) × **levels** L1/L2/L3 | REQ catalog, D7 profiles, SDK API |
| 010 | Identification Link (IEC 61406-1), base64url in topics (already in `c85ce2a`) | unchanged |
| 011 | Split into MG + RIG; chapter renumbering (IL = MG 4, MAM = MG 5, container env = RIG 4, platform = RIG 5) | citation scheme |
| 012 | Only common service left: `NewDataSetWriterId` (FileUpload, FileDownload, FirmwareUpdate, Blink removed; DSWIDs 4–7 reserved; their DataSetClassIds reserved) | schema, SDK, D7 smaller |
| 013 | `NodeId` = JSON string per Part 6 V1.05 (`"i=11"`, `"nsu=<uri>;i=6244"`); `ns=` and the V1.04 object form are not used | schema (FieldMetaData etc.), codec |
| 014 | Specific services (Read, Write, Subscribe, Unsubscribe, GenericMethod; former MG 10.3, 8.1.5.3, B1.3) removed | no `SVC` area in the REQ catalog; SDK offers only `NewDataSetWriterId` (closed set: SF-036, resolved C-015) |
| 015 | Former MG 6 (process description) moved to the separate, informative **OEC Process Description**; MG chapters 7–11 renumbered to 6–10 | citation scheme (old MG n = new MG n−1); no requirements from the Process Description |
| 016 | Empty AAS mapping chapter (former MG 11) removed | none |

## 0a. Normative keywords (MG 1.2, RIG 1.1)

- BCP 14 (RFC 2119 + RFC 8174), written in lowercase: shall/must/required/mandatory = **MUST**; shall not/must not = **MUST NOT**; should/recommended/best practice = **SHOULD**; should not/not recommended = **SHOULD NOT**; may/optional/can (permission) = **MAY**.
- Field attribute `Requirement`: Mandatory = MUST, Optional = MAY, Not used = MUST NOT (key absent). Type and Value Range must be met whenever the field is present.
- Level tables: Mandatory = MUST, Recommended = SHOULD, – = not required at that level.
- A keyword in the sentence wins over the admonition type (also in notes). Admonitions: `NOTE` (informative unless it has a keyword), `RECOMMENDED` = SHOULD, `HIGHLY RECOMMENDED` = strong SHOULD, `REQUIRED` = MUST, `ATTENTION`.
- Statements about the operator/platform apply to the platform (RIG 5.1).
- **Statements without keyword, examples and figures are informative.** Since C-016 the protocol behavior carries keywords (SF-035 resolved: Health timing, Birth topic, Status event after Set/Del, pagination continuation, PublicationList `Mode`, `ReplyTo` …). Keyword-less behavior found later → new spec finding (G-1.6).
- `REQUIRED` boxes at `49b6fe2`: 9 ACL boxes (MG 9.1.x), MG 4.1 (scheme), MG 4.2 (base64url), RIG 4.1.1–4.1.4.

## 1. Compliance (MG 1.3, 7.2; RIG 1.2)

**Messaging domain** (applications and smart devices; a smart device with its own MQTT client counts as an application):

| Requirement | L1 | L2 | L3 |
|---|:-:|:-:|:-:|
| MQTT connectivity, TLS + auth, Birth + Will | M | M | M |
| `Oi4v2` topic structure, OPC UA JSON payload | M | M | M |
| `Pub` MAM + Health (self + subordinate assets) | M | M | M |
| `Get` (MAM/Health also via Get); `Set`/`Del` where applicable (resources offered for modification) | – | M | M |
| Full MQTT client behavior (ClientId, keep-alive, clean session, QoS, no retain, Close, publish on change) | R | R | M |
| `Profile`, `PublicationList`, `ReferenceDesignation`, `Config`, `Data`, `Metadata`, `Event` | – | R | M |
| `SubscriptionList`, `Call`/`Reply` + common service `NewDataSetWriterId` (applications only) | – | R | M |
| `Set`/`Del` for every resource they are defined for (MG 9.1) | – | – | M |
| Resources without content: `Get` ⇒ empty list, no own `Pub` | M | M | M |
| `Set`/`Del` on unknown object ⇒ `Event` Status `Bad` | R | M | M |
| `Get` of `Metadata` without `Data` ⇒ no `Metadata`; may answer `Event` Status `Bad`/`BadNotFound` (SF-039) | MAY | MAY | MAY |
| `Set`/`Del` offered ⇒ `Event` category `Status`; every `Set`/`Del` answered by it | R | M | M |

- **Domains are independent (SF-038):** a level in one domain neither requires nor implies a level in the other. A microcontroller can be Messaging L3 without any Service Container level; a container can be Service Container L3 with Messaging L1.
- **Level 3 = all messages (SF-037):** an entity without configuration, data or subscriptions still supports these resources and simply answers `Get` with an empty list.

**Service Container domain** (containerized components only):

| Requirement | L1 | L2 | L3 |
|---|:-:|:-:|:-:|
| OCI container | M | M | M |
| SBOM + licenses | – | M | M |
| Storage conventions, secrets | – | R | M |
| Image integrity, networks, platform-related app requirements | – | R | M |

- Platform and infrastructure requirements (RIG 5.1) are **not** graded by level.
- "OI4 compliant" without a level means L3 in all applicable domains (Service Container only for containerized components).
- **Declaration/validation (MG 1.3):** the manufacturer declares the level per domain (e.g. product documentation, no bus message). Tests are grouped per level; the declared level is a parameter of the test run and includes all lower levels. The registry may show an informative *observed* Messaging level (from `Profile` + `Get` answers).

## 2. Identification Link (MG 4, GL-ADR-010)

- **IL syntax:** IEC 61406-1, `[<scheme>://]<authority>[/<path>][?<query>][#<fragment>]`, at most 255 chars.
  - Scheme: optional, `https` recommended.
  - Authority: lowercase.
  - Characters: ASCII only. Allowed: unreserved characters, and reserved characters only in their URL syntax role. Not allowed: space `"` `%` `<` `>` `\` `^` `` ` `` `{` `|` `}`, control characters.
  - Never altered or normalized; compared char by char, case-sensitive. With and without scheme are two different ILs.
- **In topics** (`AppId`, `Source`): base64url (RFC 4648 §5) of the UTF-8 bytes, no padding, no line breaks. This gives exactly one topic level of at most 340 chars.
- **In payloads** (`PublisherId`, `MessageId`, `ReplyTo`, `DataSetWriterName`, `RequestorId`, `ProductInstanceUri`): unencoded.
- **Oi4Identifier** = `ManufacturerUri/dnp(Model)/dnp(ProductCode)/dnp(SerialNumber)`.
  - `ManufacturerUri`: lowercase domain.
  - A missing serial number becomes `"nd"`; containers use the container name as serial number.
  - dnp encoding: `,` + 2 hex digits (MG A3).

## 3. MAM (MG 5, 8.3.1) – DataSetClassId `360ca8f3-5e66-42a2-8f10-9cdf45f4bf58`

- **Mandatory:**
  - `Manufacturer` (LocalizedText)
  - `ManufacturerUri` (domain)
  - `Model` (LocalizedText)
  - `ProductCode`
  - `HardwareRevision`
  - `SoftwareRevision`
  - `DeviceRevision`
  - `DeviceManual`
  - `DeviceClass`
  - `SerialNumber`
  - `ProductInstanceUri` (= the IL used on the bus)
  - `RevisionCounter` (Int32)
- **Optional:** `Description` (LocalizedText), `GlobalAssetId`.

## 4. Runtime & infrastructure (RIG 4, 5)

| Path | Content |
|------|---------|
| `/etc/oi4/mqtt` (ro) | `broker.json` {`Address`, `SecurePort` (UInt16), `MaxPacketSize` (Int32, **bytes**, min 262144)}, `broker.pem`, `broker_ca.pem` |
| `/etc/oi4/certs` (ro) | `ca.pem`, `<containername>.pem` (no private keys) |
| `/run/secrets/mqtt_credentials` | base64(`user:password`) |
| `/run/secrets/mqtt_private_key.pem`, `/run/secrets/mqtt_passphrase` (base64) | client key + passphrase |
| `/etc/oi4/app`, `/opt/oi4/app` | app config / app data |
| `/opt/oi4/licenses` | exactly one SBOM `sbom.spdx.json` **or** `sbom.cdx.json` (JSON) + license texts embedded or as `<License-ID>.txt` (SPDX id or `LicenseRef-…`); part of the image, not overlaid by host mounts (L2+) |

- **Container name:** globally unique, at most 63 chars, RFC 1123, best practice `<DefaultName>_<uuid>`. It is used as hostname, MQTT ClientId, certificate name and Oi4Identifier serial number.
- **Image integrity:** tags are allowed; digests are recommended. **Networks:** bridge network preferred.
- **Platform (RIG 5.1):**
  - Architectures: ARMv7, ARMv8, amd64.
  - Hardware: ≥ 1 GB RAM, ≥ 4 GB storage.
  - Software: Linux, container daemon, MQTT broker, OEC Registry.

## 5. MQTT (MG 6)

- **Connection:**
  - v3.1.1 as minimum; no MQTT 5 features required (change only by WG/TC).
  - TLS ≥ 1.2, port 8883 (1883 not supported).
  - Auth by user/password or mTLS; mTLS wins if both are available. Username uses RFC 3986 unreserved chars plus `@`.
  - ClientId = container name; keep-alive 60 s; clean session true.
- **Messages:** QoS 0 default; retained messages not used.
- **Lifecycle:**
  - Birth = MAM DataSetMessage of the app on every connect, topic `Oi4v2/<ST>/<AppId>/Pub/MAM/<AppId>`.
  - Close = Health `NORMAL_0`/0.
  - Will = Health `FAILURE_1`/0.
  - Close and Will go to `Oi4v2/<ST>/<AppId>/Pub/Health/<AppId>`. Birth, Close and Will carry no `Timestamp`/`SequenceNumber`.
  - Subscribers: Close = offline (graceful), Will = offline (ungraceful), next Birth = online.
- **Publishing:** on every change (MG 6.4), unless PublicationList says otherwise.
- **By level:** TLS/auth and Birth/Will are mandatory from L1; the rest of the client behavior is mandatory only at L3.

## 6. Topics (MG 7)

`Oi4v2/<ServiceType>/<AppId>/<Method>/<Resource>[/<Source>[/<Filter>]]`

- **Namespace** `Oi4v2`; v1 = `Oi4`. Both may coexist on one broker, but a request is answered in its own namespace (MG 7.1.1).
- **ServiceType:** `Registry | OTConnector | Utility | Persistence | Aggregation | OOCConnector | ITConnector`.
- **Method:** `Pub | Get | Set | Del | Call | Reply`.
  - `Get`/`Set`/`Del`/`Call` use the **target's** ServiceType/AppId.
  - `Reply` uses the same topic as `Call` with the method swapped.
- **Resources:** `MAM, Health, Config, Data, Metadata, Event, Profile, PublicationList, SubscriptionList, ReferenceDesignation`.
- **Common services** (Actions): only `NewDataSetWriterId` (GL-ADR-012).
- **Specific services:** removed (GL-ADR-014). The resource/service set is **closed**: other names below `Oi4v2` are non-conformant (SF-036, resolved in the guideline C-015: MG 7.1.5 "must not be used below the Namespace element").
- **Source:** base64url IL; equals `AppId` if the information is about the app itself.
- **Filter:** each level is a `<Topic encoded String>`; `+ # /` within a level are percent-encoded (`%2B %23 %2F`, `%` → `%25`), nothing else. `/` only separates resource-defined levels, never leading/trailing; `WriterGroupName` uses the same value.

  | Resource | Filter |
  |---|---|
  | Data / Metadata | `<Tag>` |
  | Config | `<Context.Name>` |
  | Event | `<EventCategory>/<EventLevel>` |
  | PublicationList / SubscriptionList | `<ResourceType>[/<Tag>]` |
  | MAM, Health, Profile, ReferenceDesignation | none |

- **Event levels:**
  - Syslog: `Debug…Emergency` (8 levels).
  - Status: `Good|Uncertain|Bad` = severity class only (e.g. 0x80060000 → `Status/Bad`); details in the payload (`Number`, `SymbolicId`).
  - Generic: `High|Medium|Low`.

## 7. Payloads (MG 8) – OPC UA Part 14/6 V1.05.04

**Fixed content masks (MG 8.1.4)**

| Message | Mask | Value |
|---|---|---|
| `ua-data` | JsonNetworkMessageContentMask | `0x3B` (with `DataSetClassId`) / `0x2B` (without) |
| `ua-data` | JsonDataSetMessageContentMask | `0xA41` min / `0xA7F` (with `MetaDataVersion`) / `0xE7D` (with `MinorVersion`) |
| `ua-data` | DataSetFieldContentMask | `0x20` (RawData, VerboseEncoding) |
| Actions | JsonNetworkMessageContentMask | `0x0B` |
| Actions | DataSetFieldContentMask | `0` (CompactEncoding) |

**NetworkMessage `ua-data` (MG 8.2.1)**

| Field | Req. | Content |
|---|---|---|
| `MessageId` | M | `<ms>-<PublisherId>`, globally unique; consumers treat it as opaque |
| `MessageType` | M | `"ua-data"` |
| `PublisherId` | M | `<ServiceType>/<IL>` |
| `DataSetClassId` | O | always the class of the resource addressed by the topic (also for `Get` and with helper DataSets); for Data/Metadata publisher-defined, stable per structure across restarts |
| `ReplyTo` | M | `""` or the causing `MessageId` |
| `Messages[]` | M | always an array |

No `WriterGroupName` at network level.

**DataSetMessage (MG 8.2.3)**

| Field | Req. | Content |
|---|---|---|
| `DataSetWriterId` | M | UInt16, 0 invalid; reserved 1 PaginationRequest, 2 Pagination, 3 Locale, 4–7 future common services, 8 NewDataSetWriterId; apps start at 10; one per resource + writer name + group name; may change on restart |
| `DataSetWriterName` | M | unencoded IL of the asset (= topic `<Source>`) |
| `WriterGroupName` | M | = topic `<Filter>`, or `""` |
| `SequenceNumber` | O | UInt32 |
| `MetaDataVersion` | O | {`MajorVersion`,`MinorVersion`}; **xor** `MinorVersion` |
| `MinorVersion` | O | UInt32; **xor** `MetaDataVersion` |
| `Timestamp` | O | |
| `Status` | O | `{Code, Symbol?}`; only when not Good (missing = Good); receivers evaluate `Code` only |
| `MessageType` | O | `ua-keyframe` (default) / `ua-event` (for Event) |
| `Payload` | M | DataSet: VerboseEncoding, RawData; enums as `<Name>_<Value>`; abstract types and multi-dim arrays as `{Value[, Dimensions]}` |

No `PublisherId` at DataSetMessage level.

**DataSetMetaData `ua-metadata` (MG 8.2.2)**
- Fields: `MessageId`, `MessageType`, `PublisherId`, `DataSetWriterId`, `WriterGroupName`, `DataSetWriterName`, `Timestamp` (M), `ReplyTo` (M, OI4 extension), `MetaData`.
- `MetaData` is a DataSetMetaDataType: `Name`, `Description`, `Fields[]`, `DataSetClassId`, `ConfigurationVersion`, `Namespaces`, `StructureDataTypes`, `EnumDataTypes`, `SimpleDataTypes`.
- **NodeIds** (e.g. `FieldMetaData.DataType`, `StructureDefinition`, `SimpleTypeDescription`) are JSON strings: ns 0 → `"i=12"`, others → `"nsu=http://opcfoundation.org/UA/DI/;i=6244"`; never `ns=` (GL-ADR-013). `DetailObject` (Health `Details`) has no NodeId yet – validated by the Health schema only.

**Actions (MG 8.1.3, 8.2.16–8.2.20, GL-ADR-007)**
- **ActionNetworkMessage:**

  | Field | Request | Response |
  |---|---|---|
  | `MessageId` | M | M |
  | `MessageType` (`ua-action-request` / `ua-action-response`) | M | M |
  | `PublisherId` (always the **Responder**) | M | M |
  | `Timestamp` | M | M |
  | `ResponseAddress` (Reply topic) | M | omitted |
  | `CorrelationData` | O | M if it was in the request |
  | `RequestorId` | M | M |
  | `TimeoutHint` | M | omitted |
  | `Messages[]` | M | M |

- **ActionRequest:** `DataSetWriterId`, `ActionTargetId` (common services: 1), `RequestId` (unique per Requestor), `ActionState` = 1, `Payload` (input arguments as fields); optional `MetaDataVersion`, `Timestamp`.
- **ActionResponse:** same fields plus `Status` (CompactEncoding: `{}` = Good, `{"Code": n}` otherwise, never `Symbol`). `ActionState` is 1 (intermediate) or 2 (done).
- **ActionMetaData:** `ActionTargets[]`, `Request`, `Response` (DataSetMetaDataType). Predefined for common services. The Action's `DataSetClassId` sits in the request/response metadata.
- **ActionState:** 0 Idle (not used), 1 Executing, 2 Done.
- Not used in Action messages: `SequenceNumber`, `DataSetWriterName`, `WriterGroupName`, `MinorVersion`, `MessageType` (DataSetMessage level).

**OI4 resources (MG 8.3)**

- **Health** `d8e7b6df-42ba-448a-975a-199f59e8ffeb`
  - `Health`: `NORMAL_0 | FAILURE_1 | CHECK_FUNCTION_2 | OFF_SPEC_3 | MAINTENANCE_REQUIRED_4`; the most severe status, ordered FAILURE > OFF_SPEC > CHECK_FUNCTION/MAINTENANCE > NORMAL.
  - `HealthScore`: Byte 0..100, optional.
  - `Details[]`: optional, each with `Health` (M), `DiagnosticCode`, `Location`, `Description` (O).
  - Any change of a detail requires publishing Health.
- **Config** `9d5983db-440d-4474-9fd7-1cd7a6c8b6c2`
  - Structure: `{<GroupName>: {Name, Description?, <ConfigName>: {...}}}`.
  - Each config entry: `Type` (`Boolean|ByteString|DateTime|Number|String`), `Value` (string), `Unit?`, `DefaultValue?`, `Mandatory?`, `Sensitive?`, `Name`, `Description?`, `Validation? {Length, Min, Max, Pattern, Values[]}`, `Context? {Name, Description?}`.
  - Names match `^[A-Za-z_][A-Za-z0-9_-]*$`. Set differs from Pub only in which fields are mandatory. Unset = empty string.
- **Data**
  - Free DataSet; optional standard set `Oi4Pv` (`Pv`, `Sv1..Sv255`, filter `Oi4Pv`).
  - `DataSetClassId` is publisher-defined, stable per structure (also across restarts).
- **Metadata:** `ua-metadata` for a Data tag; needs Source + Tag.
- **Event** `543ae05e-b6d9-4161-a0a3-350a0fac5976`
  - Fields: `Number` (M), `Description?`, `Category` (M), `Details?`.
  - Categories: `CAT_SYSLOG_0` (Details `MSG`, `HEADER`), `CAT_STATUS_1` (+ `SymbolicId`), `CAT_GENERIC_99`.
- **Profile** `48017c6a-05c8-48d7-9d85-4b08bbb707f3`: `Resources[]`; mandatory resources depend on the level.
- **PublicationList** `217434d6-6e1e-4230-b907-f52bc9ffe152`
  - Fields: `Resource`, `Source`, `Filter?`, `DataSetWriterId`, `Mode?`, `Interval?` (ms), `Precisions?`, `Config?`.
  - `Mode`: `OFF_0` … `ON_REQUEST_APPLICATION_SOURCE_FILTER_15`; default `ON_REQUEST_1`.
  - `Config`: `NONE_0 | MODE_1 | INTERVAL_2 | MODE_AND_INTERVAL_3`.
  - Keys `Source`/`Filter` are **not** renamed here.
- **SubscriptionList** `e5d68c47-c276-4929-8ab9-4c1090cac785`: `TopicPath`, `Interval?`, `Config?` (`NONE_0|CONF_1`).
- **ReferenceDesignation** `27a75019-164a-496d-a38b-90e8a55c2cfa`: `Function?`, `Product?`, `Location?`, each `{Value, Local?, Parent? {Value, Local?, IdentificationLink?}}`.
- **Pagination helpers:**
  - PaginationRequest (in Get): `PerPage` > 0, `Page` (0 = all, fragmented).
  - Pagination (in Pub): `TotalCount`, `PerPage`, `Page`, `HasNext`, `PaginationId`.
  - Helpers (DSWIDs 1–3): `WriterGroupName` = `PaginationRequest`/`Pagination`/`Locale`, `DataSetWriterName` = IL of the publishing app (requester or responder).
- **Locale** (in Get): `Locale`, `LocaleId` matching `^[a-z]{2,3}(-[A-Z]{2,3})?$` (e.g. `de-DE`, `en-GB`, `de`).

**Common-service Actions (MG 8.4.1, 9.2.1, B1.2.1)**
- NewDataSetWriterId `2aca55bd-0d6f-41b1-a1c2-2d61afcc21f0`:
  - Request `{Resource}`; response `{DataSetWriterId, Ttl}`.
  - DataSetWriterId 8; topic `Call/NewDataSetWriterId` (no Source).
- FileUpload, FileDownload, FirmwareUpdate and Blink were removed (GL-ADR-012); they are not part of the guideline until a new GL-ADR re-introduces them (the specific services were removed as well, GL-ADR-014).
- DataSetClassIds of removed resources and Actions are reserved (MG A2).

## 8. Message Bus behavior (MG 9)

| Resource | Pub | Get | Set | Del | Topic levels |
|----------|-----|-----|-----|-----|--------------|
| MAM | ✓ | ✓ | | | `[/<Source>]` |
| Health | ✓ | ✓ | | | `[/<Source>]` |
| Config | ✓ | ✓ | ✓ (Source+Filter) | | `[/<Source>[/<Context.Name>]]` |
| Data | ✓ | ✓ | ✓ (Source+Tag) | | `[/<Source>[/<Tag>]]` |
| Metadata | ✓ | ✓ (Source+Tag) | ✓ (Source+Tag) | | `/<Source>/<Tag>` |
| Event | ✓ only | | | | `/<Source>/<Category>/<Level>` |
| Profile | ✓ | ✓ | | | `[/<Source>]` |
| PublicationList | ✓ | ✓ | ✓ (Source+Filter) | | `[/<Source>[/<ResourceType>[/<Tag>]]]` |
| SubscriptionList | ✓ | ✓ | ✓ | ✓ (Source+Filter) | `[/<Source>[/<ResourceType>[/<Tag>]]]` |
| ReferenceDesignation | ✓ | ✓ | ✓ (Source) | ✓ (Source) | `[/<Source>]` |

- **Get:** the request is a `ua-data` message with the requested resource's `DataSetClassId` and an empty `Messages`, or only PaginationRequest/Locale (helpers identified by their reserved DSWIDs). The answer goes to the matching `Pub` topic with `ReplyTo` = request `MessageId`. A Get without Source returns all assets of the app (paginated if needed).
- **Set / Del:** answered by `Pub/Event/<Source>/Status/<Level>` – must from Level 2 on, should at Level 1 (MG 7.1.4, 9.1; SF-037 amended C-016) – with `ReplyTo` = request `MessageId` (must, MG 8.2.1). After a successful Set, the new state must be published (MG 9.1.x).
- **Actions:**
  - Request: `Oi4v2/<ST>/<AppId>/Call/<Service>[/<Source>]`.
  - Response: `.../Reply/<Service>[/<Source>]`.
  - Correlation via `RequestId` and `CorrelationData`.
- **Health:** must be published on start, on change and every 60 s (MG 7.1.5.1, keyword since SF-035).

## 9. Registry (MG A1)

- Collects MAM and Health (plus Profile and Event, optionally) of all assets.
- Answers `Oi4v2/Registry/<AppId>/Get/MAM[/<Source>]` and `Get/Health[/<Source>]`.
- **Web UI:** lists applications and devices with MAM, health and availability, supported resources, related publisher and an audit trail.
- **Conformity check:** optional, via schema validation.
- **Observed compliance level** (informative) per application/device, from `Profile` + `Get` answers.
- **Availability** from Birth / Close / Will (see §5).
- Sends `Get` requests after a Birth message.
- Default ports 5798 (UI) and 5799 (API), TLS.
- Platform requirement (RIG 5.1).
