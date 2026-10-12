# Guideline digest (v2.0 draft, baseline `c85ce2a`)

Compact, implementation-oriented summary of `oi4-oec-documentation/docs/guideline/`. It is **not** normative – always check the referenced section. Known problems: see `../SPEC-FINDINGS.md`.

## 1. Big picture

- Edge computer runs containerized apps + MQTT broker (Message Bus) + OEC Registry (11.1).
- Every asset (device **or** application instance) has an **Identification Link (IL)** and a **Master Asset Model (MAM)** (3, 4).
- Communication: MQTT 3.1.1 over TLS, OPC UA PubSub JSON payloads, REST-like topic namespace (7–10).

## 2. Identification Link (3, MS-010) – breaking change vs 1.x

- IL = IEC 61406-1 IL string: `[<scheme>://]<authority>[/<path>][?<query>][#<fragment>]`, ≤ 255 chars, ASCII subset (letters, digits, `-._~`, reserved `:/?#[]@!$&'()*+,;=`), no `%`, space, `"<>\^`{|}`; authority lowercase; never altered/normalized; with vs without scheme = different ILs; compared char-by-char.
- In **topics** (`AppId`, `Source`): base64url (RFC 4648 §5) of UTF-8 bytes, **no padding**, exactly one level. Node: `Buffer.from(il).toString('base64url')`.
- In **payloads**: unencoded.
- `Oi4Identifier` = `ManufacturerUri/dnp(Model)/dnp(ProductCode)/dnp(SerialNumber)` (no scheme); "nd" for missing parts; containers use container name as SerialNumber (6.2). dnp-encoding: `,` + 2 hex digits for reserved/prohibited chars (A3).
- Recommended: reuse existing IL (OPC UA ProductInstanceUri, AAS globalAssetId); else generate Oi4Identifier.

## 3. MAM (4, 9.3.1) – DataSetClassId `360ca8f3-5e66-42a2-8f10-9cdf45f4bf58`

Mandatory: `Manufacturer` (LocalizedText), `ManufacturerUri` (domain), `Model` (LocalizedText), `ProductCode`, `HardwareRevision` (`""` for software, `"undefined"` if unknown hw), `SoftwareRevision`, `DeviceRevision`, `DeviceManual` (URL), `DeviceClass` (best practice `Oi4.<ServiceType>`), `SerialNumber`, `ProductInstanceUri` (= the IL, unencoded), `RevisionCounter` (Int32). Optional: `Description` (LocalizedText), `GlobalAssetId` (tbd).

## 4. Container environment (6)

| Path | Content |
|------|---------|
| `/etc/oi4/mqtt` (ro) | `broker.json` {`Address`, `SecurePort`, `MaxPacketSize`}, `broker.pem`, `broker_ca.pem` |
| `/etc/oi4/certs` (ro) | `ca.pem`, `<containername>.pem` (client cert) |
| `/run/secrets/mqtt_credentials` | base64(`user:password`) |
| `/run/secrets/mqtt_private_key.pem`, `/run/secrets/mqtt_passphrase` (base64) | client key + passphrase |
| `/etc/oi4/app`, `/opt/oi4/app` | app config / app data |

Container name: globally unique, ≤ 63 chars, RFC 1123, best practice `<DefaultName>_<uuid>`, set as hostname; used as MQTT ClientId, cert name, Oi4Identifier serial. Bridge network preferred.

## 5. MQTT (7)

v3.1.1; TLS ≥ 1.2; port 8883 default (1883 not supported); auth user/pass or mTLS (mTLS preferred if both); username chars RFC 3986 unreserved + `@`; keep-alive 60 s; clean session true; QoS 0 default; **no retained**; Birth = MAM, Close = Health `NORMAL_0`/0, Will = Health `FAILURE_1`/0 (no Timestamp/SequenceNumber); publish on change (7.4).

## 6. Topics (8)

`Oi4/<ServiceType>/<AppId>/<Method>/<Resource>[/<Source>[/<Filter>]]`

- ServiceType: `Registry | OTConnector | Utility | Persistence | Aggregation | OOCConnector | ITConnector`. For `Get/Set/Del` use the **target's** ServiceType/AppId; for `Pub` the publisher's. `Call`/`Reply` addressing is not explicit (SF-022).
- Method: `Pub | Get | Set | Del | Call | Reply`.
- Resources: `MAM, Health, Config, Data, Metadata, Event, Profile, PublicationList, SubscriptionList, ReferenceDesignation` (`_ToDelete_` License, LicenseText, RtLicense, Interfaces, SBoM are dropped in 2.0 – ADR-0005).
- Services (Call/Reply): common `FileUpload, FileDownload, FirmwareUpdate, Blink, NewDataSetWriterId`; specific `Read, Write, Subscribe, Unsubscribe, GenericMethod`.
- Filter by resource: Data/Metadata `<Tag>`; Config `<Context.Name>`; Event `<EventCategory>/<EventLevel>` (Syslog: `Debug…Emergency`; Status: `Good|Uncertain|Bad`; Generic: `High|Medium|Low`); Publication/SubscriptionList `<ResourceType>[/<Tag>]`; no filter for MAM, Health, Profile, ReferenceDesignation.
- Minimum compliance (8.2): `Pub`, `Get`; `MAM, Health, Profile, PublicationList, ReferenceDesignation` (License/LicenseText dropped – ADR-0005); Data ⇒ Metadata; Set/Del ⇒ Event/Status.

## 7. Payloads (9)

**NetworkMessage** (`ua-data`): `MessageId` (`<ms>-<PublisherId>`), `MessageType`, `PublisherId` (`<ServiceType>/<IL>`), `DataSetClassId` (GUID, mandatory for OI4 resources), `CorrelationId` (opt.), `Messages[]`.
**DataSetMessage**: `DataSetWriterId` (UInt16, 0–9 reserved: 1 PaginationRequest, 2 Pagination, 3 Locale), `SequenceNumber` (UInt32, opt.), `MetaDataVersion` {`MajorVersion`,`MinorVersion`} (opt.), `Timestamp` (ISO 8601, opt.), `Status` (UInt32 StatusCode, omit if Good), `Filter` (cond.), `Source` (IL, mandatory), `Payload`.
**DataSetMetaData** (`ua-metadata`): `MessageId`, `MessageType`, `PublisherId`, `DataSetWriterId`, `Filter`, `Source`, `CorrelationId`, `MetaData` (DataSetMetaDataType: `Name`, `Description`, `Fields[]` FieldMetaData, `DataSetClassId`, `ConfigurationVersion`, `Namespaces[]`, `StructureDataTypes[]`, `EnumDataTypes[]`, `SimpleDataTypes[]`).
**ServiceNetworkMessage** (`MSG`): `MessageId`, `MessageType`, `PublisherId`, `DataSetClassId`, `CorrelationId`, `Message` = `ServiceParametersRequest {MethodsToCall[]: {MethodId, InputArguments[]}}` or `ServiceParametersResponse {Results[]: {StatusCode, InputArgumentResults[], OutputArguments[]}}`.

OI4 DataSets:
- **Health** `d8e7b6df-42ba-448a-975a-199f59e8ffeb`: `Health` ∈ `NORMAL_0, FAILURE_1, CHECK_FUNCTION_2, OFF_SPEC_3, MAINTENANCE_REQUIRED_4`; `HealthScore` 0..100 (opt.).
- **Config** `9d5983db-440d-4474-9fd7-1cd7a6c8b6c2`: `{<GroupName>: {Name, Description?, <ConfigName>: {Type (Boolean|ByteString|DateTime|Number|String), Value (string), Unit?, DefaultValue?, Mandatory?, Sensitive?, Name, Description?, Validation? {Length, Min, Max, Pattern, Values[]}}}, Context? {Name, Description?}}`; names `^[A-Za-z_][A-Za-z0-9_-]*$`; Set has fewer mandatory fields (Name optional); unset = empty string.
- **Data**: free DataSet; optional standard `Oi4Pv` (`Pv`, `Sv1..Sv255`), filter `Oi4Pv`. DataSetClassId publisher-specific.
- **Event** `543ae05e-b6d9-4161-a0a3-350a0fac5976`: `Number` (UInt32), `Description?`, `Category` ∈ `CAT_SYSLOG_0, CAT_STATUS_1, CAT_GENERIC_99` (`CAT_NE107_2` dropped – ADR-0005), `Details?` (Syslog: `MSG`, `HEADER`; Status: `SymbolicId`).
- **Profile** `48017c6a-05c8-48d7-9d85-4b08bbb707f3`: `Resources[]`.
- **PublicationList** `217434d6-6e1e-4230-b907-f52bc9ffe152`: `Resource`, `Source`, `Filter?`, `DataSetWriterId`, `Mode?` (`OFF_0` … `ON_REQUEST_APPLICATION_SOURCE_FILTER_15`, default `ON_REQUEST_1`), `Interval?` (ms, default 0 = on change), `Precisions?` {field: Real}, `Config?` (`NONE_0|MODE_1|INTERVAL_2|MODE_AND_INTERVAL_3`).
- **SubscriptionList** `e5d68c47-c276-4929-8ab9-4c1090cac785`: `TopicPath`, `Interval?`, `Config?` (`NONE_0|CONF_1`).
- **ReferenceDesignation** `27a75019-164a-496d-a38b-90e8a55c2cfa`: `Function?`, `Product?`, `Location?` each `{Value, Local?, Parent? {Value, Local?, IdentificationLink?}}` (IEC 81346).
- **PaginationRequest** (in Get): `PerPage` (>0), `Page` (0 = all, fragmented). **Pagination** (in Pub): `TotalCount`, `PerPage`, `Page`, `HasNext`, `PaginationId`.
- **Locale** (in Get): `Locale` (e.g. `de-DE`).
- **NewDataSetWriterId** `2aca55bd-0d6f-41b1-a1c2-2d61afcc21f0`: in `Resource` → out `DataSetWriterId`, `Ttl` (s).
- Other service class ids: FileUpload `3b4a62ba-…`, FileDownload `760abda2-…`, FirmwareUpdate `414e26f6-…`, Blink `3b423a40-…` (see A2 for full ids + reserved ids).

## 8. Message Bus behavior (10)

| Resource | Pub | Get | Set | Del | Topic levels |
|----------|-----|-----|-----|-----|--------------|
| MAM | ✓ | ✓ | | | `[/<Source>]` |
| Health | ✓ | ✓ | | | `[/<Source>]` |
| Config | ✓ | ✓ | ✓ (needs Source+Filter) | | `[/<Source>[/<Context.Name>]]` |
| Data | ✓ | ✓ | ✓ (needs Source+Tag) | | `[/<Source>[/<Tag>]]` |
| Metadata | ✓ | ✓ (needs Source+Tag) | ✓ (needs Source+Tag) | | `/<Source>/<Tag>` |
| Event | ✓ only | | | | `/<Source>/<Category>/<Level>` |
| Profile | ✓ | ✓ | | | `[/<Source>]` |
| PublicationList | ✓ | ✓ | ✓ (needs Source+Filter) | | `[/<Source>[/<ResourceType>[/<Tag>]]]` |
| SubscriptionList | ✓ | ✓ | ✓ | ✓ (need Source+Filter) | `[/<Source>[/<ResourceType>[/<Tag>]]]` |
| ReferenceDesignation | ✓ | ✓ | ✓ (needs Source) | ✓ (needs Source) | `[/<Source>]` |

- `Get` payload: NetworkMessage with empty `Messages` or only PaginationRequest/Locale; answer on matching `Pub` topic with `CorrelationId` = request `MessageId`.
- `Get` without Source → all assets of the app in one NetworkMessage (paginate if needed).
- `Set`/`Del` → `Pub/Event/<Source>/Status/<Level>` with `CorrelationId`; after successful Set, new state published per PublicationList.
- Services: `Oi4/<ST>/<AppId>/Call/<Service>[/<Source>]` → `Oi4/<ST>/<AppId>/Reply/<Service>[/<Source>]`.
- Health: on start, on change, every 60 s.

## 9. Registry (A1)

Collects MAM + Health (+ Profile, Event optional) of all assets; offers `Oi4/Registry/<AppId>/Get/MAM[/<Source>]` and `Get/Health[/<Source>]`; web UI lists applications and devices with MAM, health/availability, supported resources, related publisher, audit trail; optional conformity check via schema validation; sends `Get`s after a Birth; default ports 5798 (UI) / 5799 (API); TLS; should start first.
