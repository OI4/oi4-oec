---
title: Concepts
sidebar_position: 1
description: The few ideas behind OEC v2.0 in short – assets and roles, identities, topics, resources and methods, payloads and compliance levels.
---

# Concepts

This page explains the few ideas you need to read the rest of the site. It
is a summary, not the standard: the OEC Messaging Guideline ("Messaging
Guideline") and the OEC Runtime & Infrastructure Guideline ("Runtime &
Infrastructure Guideline") are the reference, and every section below names
the sections it is based on. The requirements derived from them, with their
tests, are listed in the
[requirement catalog](../../spec/README.md#citing-requirements).

## Assets and roles

Everything that takes part in OEC is an **asset**: an application on the
edge platform, a device it talks to, a station of a weather service. Every
asset has an identity of its own (see below) and a Master Asset Model
(`MAM`) – its nameplate, based on the OPC UA VendorNameplate (Messaging
Guideline 5).

An application on the OI4 Message Bus is a *directly connected entity*: it
has its own MQTT connection. A device without its own connection is a
*represented entity* – an application speaks for it as a subordinate asset
(Messaging Guideline 1.3). A smart device that connects to the broker
itself is directly connected and uses the ServiceType `OTConnector`
(Messaging Guideline 7.2).

Every application has one role on the bus, its **ServiceType** (Messaging
Guideline 7.2). It names the main task of the application; the set is
closed:

| ServiceType | Typical role |
|---|---|
| `Registry` | basic services, such as which applications and devices are available ([OEC Registry](registry/index.md)) |
| `OTConnector` | makes the assets of a subordinate network available on the bus |
| `Utility` | utilities such as converters or configuration |
| `Persistence` | persists data |
| `Aggregation` | aggregates data of other sources and publishes the enriched data |
| `OOCConnector` | connects the bus to a higher-level Open Operator Cloud |
| `ITConnector` | connects the bus to IT systems such as MES or ERP |

## Identities

### Identification Link

The identity of an asset is its **Identification Link** after IEC 61406-1
(Messaging Guideline 4.1): a string of at most 255 characters, for example
`company.id/A87BAD34H` or `https://company.id/A87BAD34H`. The scheme is
optional (`https` is recommended); with and without scheme are two
different Identification Links. Identification Links are compared
character by character and never changed by a receiver.

An asset uses the Identification Link it already has – printed on it, as
OPC UA `ProductInstanceUri` or as `globalAssetId` of its Asset
Administration Shell. Without one, the manufacturer defines one, or the
`Oi4Identifier` is used.

### Oi4Identifier

The `Oi4Identifier` is the recommended Identification Link for an asset
that has none (Messaging Guideline 4.3): `ManufacturerUri/Model/ProductCode/SerialNumber`,
built from the `MAM`. `ManufacturerUri` is the domain of the manufacturer
in lower case. The other three elements are dnp-encoded (Messaging
Guideline 4.3.1, A3), for example `OEC Demo` becomes `OEC,20Demo`.

### In topics and payloads

In a topic, an Identification Link is one topic level, encoded with
base64url without padding (Messaging Guideline 4.2):
`company.id/A87BAD34H` becomes `Y29tcGFueS5pZC9BODdCQUQzNEg`. In the
payload it stays unencoded, for example in `PublisherId`,
`DataSetWriterName` and `ProductInstanceUri` of the `MAM`.

The **AppId** is the encoded Identification Link of the application that
connects to the bus (Messaging Guideline 7.3). The **Source** is the
encoded Identification Link of the asset a message is about – the
application itself or one of its subordinate assets (Messaging Guideline
7.6).

## Topics

Every topic follows one schema (Messaging Guideline 7):

```text
Oi4v2/<ServiceType>/<AppId>/<Method>/<Resource>[/<Source>[/<Filter>]]
```

For example, a `Get` of the `MAM` of the asset `company.id/A87BAD34H`,
sent to an `OTConnector`:

```text
Oi4v2/OTConnector/cHJvdmlkZXIuY29tL0ZpZWxkRGF0YVNlcnZpY2UvRkRTLTAwMS8xMjAwLTAzNDU/Get/MAM/Y29tcGFueS5pZC9BODdCQUQzNEg
```

- `Oi4v2` is the namespace of version 2 (Messaging Guideline 7.1).
- For `Pub`, `ServiceType` and `AppId` are those of the publisher; for a
  request (`Get`, `Set`, `Del`, `Call`) they are those of the receiver
  (Messaging Guideline 7.2, 7.3).
- `Filter` narrows a resource, for example to a group of values or the
  category and level of an `Event` (Messaging Guideline 7.7).

Only the topic forms listed per resource are allowed (Messaging Guideline
9.1, 9.2). The SDKs build and parse them for you; the exact grammar and
its regexes are in
[`oec/spec/README.md` – Topic grammar and regexes](../../spec/README.md#topic-grammar-and-regexes).

## Resources and methods

The **resources** are a closed set (Messaging Guideline 7.5.1):

| Resource | Content |
|---|---|
| `MAM` | the nameplate of an asset |
| `Health` | its state, published at the start, on every change and at least every 60 s for the application itself |
| `Config` | its settings, readable and – where offered – writable |
| `Data` | process values |
| `Metadata` | the description of the fields of a `Data` or `Config` |
| `Event` | status events, syslog and generic events |
| `Profile` | the resources an application or device supports |
| `PublicationList`, `SubscriptionList` | what an application publishes and subscribes to |
| `ReferenceDesignation` | the reference designation system of an asset |

The **methods** (Messaging Guideline 7.4) say what happens with a resource:

- `Pub` publishes it – on its own or as the answer to a request.
- `Get` asks for it; the answer is a `Pub`. A `Get` of something without
  content is answered with an empty list.
- `Set` writes it, `Del` deletes it. The receiver answers with an `Event`
  of category `Status` that says whether it worked.
- `Call` and `Reply` invoke a service. The only service is
  `NewDataSetWriterId` (Messaging Guideline 7.5.2, 9.2.1).

Which resources and methods an application must offer depends on its
compliance level (see below); Messaging Guideline 9 describes the exchange
per resource.

Besides, every application connects with a Birth message and leaves with a
`Close` message; the broker publishes its Will if it disappears (Messaging
Guideline 6.3).

## Messages

The payload is JSON after OPC UA Part 14 (PubSub) V1.05.04 with fixed
content masks (Messaging Guideline 8): a `NetworkMessage` with a list of
`DataSetMessages`. `DataSetWriterName` and `WriterGroupName` of a
`DataSetMessage` carry the `Source` and `Filter` of the topic. `ReplyTo`
links an answer to the `MessageId` of the request. A sender must not add
keys of its own; a tolerant receiver ignores keys it does not know
(Messaging Guideline 8).

The JSON Schemas of all messages:
[`oec/schema/README.md`](../../schema/README.md). Check a recorded message
with `oec-check validate`:
[`oec/conformance/README.md` – `oec-check validate`](../../conformance/README.md#oec-check-validate).

## Compliance levels

Compliance is graded in two independent domains, each with three levels
(Messaging Guideline 1.3, Runtime & Infrastructure Guideline 1.3):

| Level | Messaging | Service Container |
|---|---|---|
| 1 Minimal | MQTT with TLS and authentication, Birth and Will, topic and payload format, `Pub` of `MAM` and `Health` | an OCI image |
| 2 Intermediate | adds the request patterns: `Get` of `MAM` and `Health`, `Set`/`Del` where offered, `Close` | adds the SBOM with the license texts in `/opt/oi4/licenses` |
| 3 Full | all methods and resources, the common service, the complete MQTT behavior | storages and secrets, names, image integrity, networks and the platform requirements |

The manufacturer declares one level per domain, for example "Messaging
level 2, Service Container level 3"; there is no message on the bus for it.
A microcontroller without a container has a Messaging level only. "OI4
compliant" without a level means level 3. The minimum requirements on the
edge platform itself have no levels (Runtime & Infrastructure Guideline
5.1).

The tests are grouped by level, so you can check a claim:

- `oec-check` (D7) checks an application on the bus,
- `oec-infra` (D8) checks images, containers, the broker and the host.

Both are described in
[`oec/conformance/README.md`](../../conformance/README.md). The
[OEC Registry](registry/index.md) shows an observed Messaging level of each
participant – informative, not a compliance claim.

## Where next

- Coming from version 1.x: [Migration notes 1.x → 2.0](migration.md).
- See it working: [Demo services](demos/index.md).
- On a microcontroller: [Micro SDK](micro-sdk/index.md).
- Build your own application with an SDK:
  [Rust](../../sdk/rust/README.md), [Go](../../sdk/go/README.md).
