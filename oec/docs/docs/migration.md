---
title: Migration notes 1.x → 2.0
sidebar_position: 5
description: What changed between the OEC Development Guideline 1.x and OEC v2.0, and what an application written for 1.x has to do – change by change.
---

# Migration notes 1.x → 2.0

OEC v2.0 is not backward compatible with the OEC Development Guideline 1.x.
The changes were decided in the guideline ADRs GL-ADR-001 … 022; most of
them bring the payload in line with OPC UA Part 14 V1.05.04 and replace
OI4-specific parts with existing standards. This page lists each breaking
change: what changes, and what an application written for 1.x has to do.
The section numbers refer to the current Messaging Guideline and Runtime &
Infrastructure Guideline; the 1.x guideline numbered its chapters
differently. For the new terms see [Concepts](concepts.md).

## At a glance

| Area | 1.x | 2.0 | Decision |
|---|---|---|---|
| Asset identity in topics | `Oi4Identifier` over four topic levels | any IEC 61406-1 Identification Link, base64url, one topic level | GL-ADR-010 |
| Namespace | `Oi4` | `Oi4v2` | GL-ADR-003 |
| `DataSetMessage` keys | `Source`, `Filter` | `DataSetWriterName`, `WriterGroupName` | GL-ADR-004 |
| Link to the causing message | `CorrelationId` | `ReplyTo` | GL-ADR-005 |
| Payload standard | OPC UA Part 14 V1.04 | V1.05.04 with fixed content masks, `NodeId`s as JSON strings | GL-ADR-006, 013 |
| Method calls | message type `MSG` with OI4 objects | OPC UA Actions `ua-action-request`/`ua-action-response`; only `NewDataSetWriterId` | GL-ADR-007, 012, 014 |
| Health | NE107 event category | `Details` in `Health` | GL-ADR-001, 008 |
| Licenses | resources `License`, `LicenseText`, `RtLicense` | SBOM and license texts in `/opt/oi4/licenses` | GL-ADR-002, 008 |
| Other resources | `Interfaces`, `SBoM` | removed | GL-ADR-008 |
| Compliance | compliant or not | two domains, three levels each | GL-ADR-009, 017 |
| Guideline | one Development Guideline | Messaging Guideline and Runtime & Infrastructure Guideline | GL-ADR-011, 015 |

## Migrate step by step

Both versions can run on the same broker at the same time (Messaging
Guideline 7.1): 1.x messages under `Oi4`, 2.0 messages under `Oi4v2`. An
application may support both during a transition. It then publishes and
subscribes each version separately and answers a request in the namespace
it came in; the two are never mixed. So you can move one application after
the other.

The SDKs, demos and tools of this repository implement version 2.0 only.

## Identification Link instead of Oi4Identifier (GL-ADR-010)

**What changes.** In 1.x, `AppId` and `Source` were the `Oi4Identifier`
`ManufacturerUri/Model/ProductCode/SerialNumber`, spread over four topic
levels. In 2.0 an asset is identified by any Identification Link after
IEC 61406-1 – the `Oi4Identifier` is still allowed and is the recommended
choice for an asset without one (Messaging Guideline 4.1, 4.3). In a topic,
the Identification Link is a single level, encoded with base64url without
padding (Messaging Guideline 4.2); in the payload it is unencoded. The
`PublisherId` is `ServiceType/IdentificationLink` (Messaging Guideline
8.2.1), the `ProductInstanceUri` of the `MAM` holds the Identification Link
used on the bus, and the property `Oi4Identifier` of a
`ReferenceDesignation` is now `IdentificationLink`.

**What a 1.x application must do.**

- Choose the Identification Link of each asset: one it already has (on its
  nameplate, its OPC UA `ProductInstanceUri`, the `globalAssetId` of its
  Asset Administration Shell) or the `Oi4Identifier`. Keep it stable and
  unchanged – receivers compare it character by character.
- If you keep the `Oi4Identifier`: `ManufacturerUri` in lower case, the
  other three elements dnp-encoded (Messaging Guideline 4.3.1, A3), at most
  255 characters in total.
- Encode it with base64url without padding for `AppId` and `Source`;
  decode it for display. Use it unencoded in `PublisherId`, `MessageId`,
  `ReplyTo`, `DataSetWriterName` and `ProductInstanceUri`.
- Replace subscriptions that used wildcards on parts of the identifier
  (for example all assets of one manufacturer): a topic level can no
  longer be matched partially. Subscribe to all assets and select by their
  `MAM` instead.

## Namespace `Oi4v2` (GL-ADR-003)

**What changes.** The first topic level names the version of the topic and
payload definitions (Messaging Guideline 7.1). Version 1 keeps `Oi4`;
version 2 uses `Oi4v2`.

**What a 1.x application must do.** Publish and subscribe under `Oi4v2`,
for example `Oi4v2/OTConnector/<AppId>/Pub/Health/<Source>` instead of the
1.x topic under `Oi4`. If it supports both versions for a while, keep them
apart as described in [Migrate step by step](#migrate-step-by-step).

## `DataSetWriterName` and `WriterGroupName` (GL-ADR-004, 018)

**What changes.** The OI4 keys `Source` and `Filter` of a `DataSetMessage`
are replaced by the OPC UA keys `DataSetWriterName` and `WriterGroupName`
(Messaging Guideline 8.2.3). Both are mandatory: `DataSetWriterName` is the
unencoded Identification Link of the asset, `WriterGroupName` the `Filter`
of the topic, or an empty string for a resource without `Filter`. In the
`PublicationList` and `SubscriptionList`, the `WriterGroupName` of each
entry is the `Filter` that addresses the entry, for example `Data/oee`
(Messaging Guideline 7.7).

The topic elements keep their names `Source` and `Filter`, and the fields
`Source` and `Filter` inside the entries of a `PublicationList` or
`SubscriptionList` stay as they are.

**What a 1.x application must do.** Rename the two keys in every
`DataSetMessage` it sends and reads, and always send both.

## `ReplyTo` instead of `CorrelationId` (GL-ADR-005)

**What changes.** The OI4 key `CorrelationId` of the `NetworkMessage` is
replaced by the OPC UA key `ReplyTo` (Messaging Guideline 8.2.1). It is
mandatory: the `MessageId` of the message that caused this one – the `Get`
for its answer, the `Set` for its status `Event` – or an empty string. A
`ua-metadata` message carries it as well (Messaging Guideline 8.2.2).

**What a 1.x application must do.** Rename the key, send it in every
message and set it to `""` for messages that no other message caused.

## OPC UA Part 14 V1.05.04 (GL-ADR-006, 013)

**What changes.** The payload follows OPC UA Part 14 V1.05.04 with content
masks fixed by the guideline (Messaging Guideline 8.1.4). The most visible
differences to 1.x:

- `Messages` of a `NetworkMessage` is always an array.
- The `Status` of a `DataSetMessage` is a `StatusCode` object with `Code`
  and `Symbol`, not a number, and only present when it is not Good.
- A `DataSetMessage` carries either `MinorVersion` or `MetaDataVersion`;
  its `MessageType` is `ua-keyframe` or `ua-event`.
- `ua-metadata` is a flat `DataSetMetaData` message (Messaging Guideline
  8.2.2).
- A `NodeId` is a JSON string: `"i=11"` in namespace 0, otherwise the
  namespace URI form `"nsu=…;i=…"` (Messaging Guideline 8.2.6).

**What a 1.x application must do.** Rework the encoder and decoder of the
payload after Messaging Guideline 8, including the content masks. The JSON
Schemas in [`oec/schema/`](../../schema/README.md) describe every message;
`oec-check validate` checks recorded messages against them
([`oec/conformance/README.md` – `oec-check validate`](../../conformance/README.md#oec-check-validate)).

## Actions instead of `MSG` method calls (GL-ADR-007, 012, 014)

**What changes.** `Call` and `Reply` no longer use the OI4 message type
`MSG` and its objects, but the OPC UA Actions of Part 14: an
`ActionNetworkMessage` of type `ua-action-request` or `ua-action-response`
(Messaging Guideline 8.1.3, 8.2.16 … 8.2.20). The common services
`FileUpload`, `FileDownload`, `FirmwareUpdate` and `Blink` and the
specific services are removed; `NewDataSetWriterId` is the only service
(Messaging Guideline 7.5.2, 8.4.1, 9.2.1).

**What a 1.x application must do.** Drop the removed services. If it calls
or answers `NewDataSetWriterId`, implement it as an Action after Messaging
Guideline 8.4.1 and 9.2.1; the topic of the `Reply` is the topic of the
`Call` with the method replaced (Messaging Guideline 7.4).

## Health without NE107 event (GL-ADR-001, 008)

**What changes.** The event category NE107 is removed. `Health` shows the
most severe state of the asset; the optional `Details` list the single
issues (Messaging Guideline 8.3.2). The order of priority is `FAILURE_1`,
`CHECK_FUNCTION_2`, `OFF_SPEC_3`, `MAINTENANCE_REQUIRED_4`, `NORMAL_0`. An
application publishes its own `Health` at least every 60 s (Messaging
Guideline 7.5.1).

**What a 1.x application must do.** Stop publishing NE107 events; report
the issues in the `Health` with `Details` instead, and check the heartbeat.

## SBOM instead of license resources (GL-ADR-002, 008)

**What changes.** The resources `License`, `LicenseText`, `RtLicense`,
`Interfaces` and `SBoM` are removed; their `DataSetClassId`s are reserved
and never reused (Messaging Guideline A2). The license information of a
containerized application is an SBOM in the image: exactly one of
`/opt/oi4/licenses/sbom.spdx.json` (SPDX) or
`/opt/oi4/licenses/sbom.cdx.json` (CycloneDX), with the text of every
license either in the SBOM or as a file `/opt/oi4/licenses/<License-ID>.txt`
(Runtime & Infrastructure Guideline 4.1.4, 5.3). It is required from
Service Container level 2.

**What a 1.x application must do.** Remove the license resources from its
code, `Profile` and `PublicationList`. Generate the SBOM when building the
image and add the license texts. `oec-infra image` checks the result
([`oec/conformance/README.md` – `oec-infra image`](../../conformance/README.md#oec-infra-image)).

## Compliance levels (GL-ADR-009, 017)

**What changes.** Instead of a single "compliant", compliance has two
independent domains – Messaging and Service Container – with the levels 1
(Minimal), 2 (Intermediate) and 3 (Full) each (Messaging Guideline 1.3,
Runtime & Infrastructure Guideline 1.3). The manufacturer declares one
level per domain. "OI4 compliant" without a level means level 3 in every
domain that applies, which is closest to a 1.x application that
implemented the whole guideline. An application that represents devices
offers for each of them what its own level requires.

**What a 1.x application must do.** Decide on the level per domain,
implement what it requires and state it in the documentation of the
product. The levels are explained in
[Concepts – Compliance levels](concepts.md#compliance-levels).

## Two guidelines (GL-ADR-011, 015)

The 1.x Development Guideline is split into the Messaging Guideline (topics,
payloads, bus behavior), the Runtime & Infrastructure Guideline
(container, storages, platform) and the informative OEC Process
Description. Requirements on containers – storages, secrets, names, image
integrity, networks – are in the Runtime & Infrastructure Guideline now.

## Further changes in 2.0

- **Secrets** (GL-ADR-022): the platform provides the identity of an
  application in `/run/secrets/oi4/`: `credentials` (username and
  password) or `private_key.pem` (with an optional `passphrase`) for its
  certificate, or both. An application
  supports both means of authentication and uses the one the platform
  provides (Runtime & Infrastructure Guideline 4.1.3).
- **Sensitive `Config` values** (GL-ADR-021) are published masked as
  `********`; a `Set` with the mask keeps the value (Messaging Guideline
  8.3.3).
- **Pagination, `Locale` and `Query`** (GL-ADR-019, 020) are clarified;
  `Query` asks for the children of a `ReferenceDesignation` (Messaging
  Guideline 8.3.11 … 8.3.13).

## Check the result

- `oec-check` (D7) tests an application on the bus at its declared
  Messaging level; `oec-infra` (D8) tests images, containers, the broker
  and the host: [`oec/conformance/README.md`](../../conformance/README.md).
- The dev environment with broker and certificates:
  [`oec/dev/README.md`](../../dev/README.md).
- New code is easier with an SDK: [Rust](../../sdk/rust/README.md),
  [Go](../../sdk/go/README.md) or the [Micro SDK](micro-sdk/index.md) for
  microcontrollers.
