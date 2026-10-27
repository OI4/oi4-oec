//! Sans-IO protocol logic of the OI4 OEC v2 SDK (D2).
//!
//! Internal crate of the SDK crate `oec` (ADR-0011, ADR-0026): no I/O, no
//! runtime and no stability promise of its own; time is a parameter of the
//! state machines. The public API is the one of
//! `oec` (ADR-0020); `oec` re-exports or wraps the types of this crate.
//! Runtime dependencies follow ADR-0023 (serde, serde_json, time, uuid;
//! jsonschema behind the feature `strict-validation`).
//!
//! | Module | Content | Guideline |
//! |---|---|---|
//! | [`il`] | Identification Link, dnp-encoding, Oi4Identifier | MG 4, MG A3 |
//! | [`topic`] | topic elements, allowed forms, Filter, topic parsing and building, subscription filters | MG 7, MG 9 |
//! | [`message`] | typed NetworkMessages (`ua-data`, `ua-metadata`, Actions) with the fixed content masks, MessageId/`ReplyTo`, Action correlation, helper DataSets, DataSetMetaData | MG 8, MG 9.2 |
//! | [`resource`] | the payloads of the resources (MAM, Health, Profile, PublicationList, SubscriptionList, ReferenceDesignation, Config, Data/Oi4Pv, Event) | MG 9 |
//! | [`schema`] | embedded D1 schemas: version, baseline and (feature `strict-validation`) validation with REQ attribution | ADR-0019 |
//! | [`mqtt`] | MQTT 3.1.1 codec (QoS 0/1, no Retain) and the client session as a state machine: connect with Will, Keep Alive, packet ids, QoS 1 acknowledgements, subscriptions, close | MG 6, RIG 4.1, ADR-0023 Q-026 |
//! | [`lifecycle`] | Birth, Will, Close and the Health heartbeat (`Details[]` included) as state machines on the message model | MG 6.3, MG 9.1.2 |
//! | [`provider`] | the provider side: sources (the application and its assets), MAM/Health/Profile/PublicationList per Source, stable DataSetWriterIds, `Get` answers with ReplyTo, pagination within the MaxPacketSize | MG 8.3.11, MG 9.1 |
//!
//! Every rule cites its requirement ids (`oec/spec/requirements/`); the
//! shared test vectors (`oec/spec/export/`) are the acceptance test
//! (`tests/vectors.rs`, `tests/payload.rs`, `tests/lifecycle.rs`,
//! `tests/provider.rs`); the
//! codec and the session have scenario and property tests
//! (`tests/mqtt_codec.rs`, `tests/session.rs`, `tests/proptest_mqtt.rs`).

/// Implements `Serialize` and `Deserialize` as the JSON string of `Display`
/// and `FromStr`, so a decoded value always satisfies the invariants of its
/// parser.
macro_rules! serde_str {
    ($name:ty) => {
        impl serde::Serialize for $name {
            fn serialize<S: serde::Serializer>(&self, s: S) -> Result<S::Ok, S::Error> {
                s.collect_str(self)
            }
        }

        impl<'de> serde::Deserialize<'de> for $name {
            fn deserialize<D: serde::Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
                let s = String::deserialize(d)?;
                s.parse().map_err(serde::de::Error::custom)
            }
        }
    };
}

/// Defines a closed set of topic element values (REQ-TOP-005, REQ-TOP-011,
/// REQ-TOP-012, REQ-TOP-023) with `ALL`, `as_str`, `Display` and `FromStr`.
macro_rules! element {
    (
        $(#[$meta:meta])*
        $name:ident { $($(#[$vmeta:meta])* $variant:ident = $text:literal),+ $(,)? }
    ) => {
        $(#[$meta])*
        #[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, PartialOrd, Ord)]
        pub enum $name {
            $($(#[$vmeta])* $variant),+
        }

        impl $name {
            /// All values in the order of the guideline.
            pub const ALL: &'static [Self] = &[$(Self::$variant),+];

            /// The topic element (case-sensitive).
            pub const fn as_str(self) -> &'static str {
                match self {
                    $(Self::$variant => $text),+
                }
            }
        }

        impl core::fmt::Display for $name {
            fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
                f.write_str(self.as_str())
            }
        }

        impl core::str::FromStr for $name {
            type Err = $crate::topic::UnknownElement;

            fn from_str(s: &str) -> Result<Self, Self::Err> {
                match s {
                    $($text => Ok(Self::$variant),)+
                    _ => Err($crate::topic::UnknownElement { element: stringify!($name), value: s.to_owned() }),
                }
            }
        }

        serde_str!($name);
    };
}

mod base64url;
pub mod il;
pub mod lifecycle;
pub mod message;
pub mod mqtt;
pub mod provider;
pub mod resource;
pub mod schema;
pub mod status;
pub mod topic;
