//! The typed OEC message model (MG 8): NetworkMessages `ua-data` and
//! `ua-metadata`, the Action messages `ua-action-request` and
//! `ua-action-response`, their DataSetMessages and helper DataSets.
//!
//! The model encodes with the fixed content masks of the guideline
//! (REQ-MSG-004, REQ-MSG-017, REQ-ACT-003). Decoding is tolerant towards
//! keys beyond the guideline (REQ-MSG-046) and checks types, required
//! fields, closed enumerations and the invariants the types carry
//! (reserved DataSetWriterIds and DataSetClassIds, MessageType, the version
//! exclusivity). Patterns and cross-field rules are left to the schema
//! (feature `strict-validation`, `schema::Validator`).
//!
//! [`Message::decode`] chooses the message type from the topic.

mod action;
mod envelope;
mod helper;
mod metadata;
mod value;

use core::fmt;

use serde::Serialize;
use serde_json::Value;

pub use action::{
    ActionPayload, ActionRequest, ActionRequestMessage, ActionResponse, ActionResponseMessage,
    ActionState, NewDataSetWriterIdRequest, NewDataSetWriterIdResponse, TimeoutHint, is_bad,
};
pub use envelope::{
    DataNetworkMessage, DataSet, DataSetEntry, DataSetMessage, DataSetMessageType, DataSetStatus,
    DataSetVersion, DataSetWriterId, MessageId, MetaDataNetworkMessage, PublisherId,
    RESERVED_CLASS_IDS,
};
pub use helper::{Locale, Pagination, PaginationRequest, Query};
pub use metadata::{
    DataSetMetaData, EnumDefinition, EnumDescription, EnumField, FieldMetaData, KeyValuePair,
    SimpleTypeDescription, StructureDefinition, StructureDescription, StructureField,
    StructureType,
};
pub use value::{ByteString, ConfigurationVersion, DateTime, LocalizedText};

use crate::resource::{
    Config, ConfigSet, Data, Event, Health, Mam, Profile, PublicationEntry, ReferenceDesignation,
    SubscriptionEntry,
};
use crate::topic::{Method, Resource, Service, Subject, Topic};

/// A message that does not decode into the model, or a value that violates
/// an invariant of the model.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DecodeError(String);

impl DecodeError {
    pub(crate) fn new(msg: impl Into<String>) -> Self {
        Self(msg.into())
    }
}

impl fmt::Display for DecodeError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl std::error::Error for DecodeError {}

impl From<serde_json::Error> for DecodeError {
    fn from(e: serde_json::Error) -> Self {
        Self(e.to_string())
    }
}

/// A decoded OEC message, typed by the topic it was received on.
#[derive(Debug, Clone, PartialEq, Serialize)]
#[serde(untagged)]
pub enum Message {
    /// MAM (MG 9.1.1).
    Mam(DataNetworkMessage<Mam>),
    /// Health (MG 9.1.2).
    Health(DataNetworkMessage<Health>),
    /// Config of Get and Pub (MG 9.1.3).
    Config(DataNetworkMessage<Config>),
    /// Config of Set: only the changed values (REQ-RES-CONFIG-003).
    ConfigSet(DataNetworkMessage<ConfigSet>),
    /// Data (MG 9.1.4).
    Data(DataNetworkMessage<Data>),
    /// Metadata (MG 9.1.5).
    MetaData(MetaDataNetworkMessage),
    /// Event (MG 9.1.6).
    Event(DataNetworkMessage<Event>),
    /// Profile (MG 9.1.7).
    Profile(DataNetworkMessage<Profile>),
    /// PublicationList (MG 9.1.8).
    PublicationList(DataNetworkMessage<PublicationEntry>),
    /// SubscriptionList (MG 9.1.9).
    SubscriptionList(DataNetworkMessage<SubscriptionEntry>),
    /// ReferenceDesignation (MG 9.1.10).
    ReferenceDesignation(DataNetworkMessage<ReferenceDesignation>),
    /// Call NewDataSetWriterId (MG 9.2.1).
    NewDataSetWriterIdRequest(ActionRequestMessage<NewDataSetWriterIdRequest>),
    /// Reply NewDataSetWriterId (MG 9.2.1).
    NewDataSetWriterIdResponse(ActionResponseMessage<NewDataSetWriterIdResponse>),
}

impl Message {
    /// Decodes the MQTT payload received on `topic`.
    ///
    /// # Errors
    ///
    /// [`DecodeError`] if the payload is not JSON or does not fit the
    /// message type of the topic.
    pub fn decode(topic: &Topic, payload: &[u8]) -> Result<Self, DecodeError> {
        let v: Value = serde_json::from_slice(payload)?;
        Self::from_value(topic, v)
    }

    /// Decodes a JSON value received on `topic`.
    ///
    /// # Errors
    ///
    /// As [`Self::decode`].
    pub fn from_value(topic: &Topic, v: Value) -> Result<Self, DecodeError> {
        use serde_json::from_value as from;
        Ok(match (topic.method(), topic.subject()) {
            (Method::Call, Subject::Service(Service::NewDataSetWriterId)) => {
                Self::NewDataSetWriterIdRequest(from(v)?)
            }
            (_, Subject::Service(Service::NewDataSetWriterId)) => {
                Self::NewDataSetWriterIdResponse(from(v)?)
            }
            (Method::Set, Subject::Resource(Resource::Config)) => Self::ConfigSet(from(v)?),
            (_, Subject::Resource(r)) => match r {
                Resource::Mam => Self::Mam(from(v)?),
                Resource::Health => Self::Health(from(v)?),
                Resource::Config => Self::Config(from(v)?),
                Resource::Data => Self::Data(from(v)?),
                Resource::Metadata => Self::MetaData(from(v)?),
                Resource::Event => Self::Event(from(v)?),
                Resource::Profile => Self::Profile(from(v)?),
                Resource::PublicationList => Self::PublicationList(from(v)?),
                Resource::SubscriptionList => Self::SubscriptionList(from(v)?),
                Resource::ReferenceDesignation => Self::ReferenceDesignation(from(v)?),
            },
        })
    }

    /// The JSON value.
    ///
    /// # Errors
    ///
    /// [`DecodeError`] if a payload does not serialize (e.g. a map with
    /// non-string keys in user data).
    pub fn to_value(&self) -> Result<Value, DecodeError> {
        Ok(serde_json::to_value(self)?)
    }

    /// The MQTT payload: compact JSON in UTF-8.
    ///
    /// # Errors
    ///
    /// As [`Self::to_value`].
    pub fn encode(&self) -> Result<Vec<u8>, DecodeError> {
        Ok(serde_json::to_vec(self)?)
    }
}

/// Serde helpers for the OPC UA Guid: lowercase, hyphenated (Part 6
/// 5.4.2.7, MG 8).
pub(crate) mod guid {
    use serde::{Deserialize, Deserializer, Serializer};
    use uuid::Uuid;

    fn parse<E: serde::de::Error>(s: &str) -> Result<Uuid, E> {
        Uuid::try_parse(s)
            .ok()
            .filter(|u| u.hyphenated().to_string() == s)
            .ok_or_else(|| E::custom(format!("{s:?} is not a lowercase hyphenated Guid")))
    }

    #[allow(clippy::trivially_copy_pass_by_ref)]
    pub(crate) fn serialize<S: Serializer>(u: &Uuid, s: S) -> Result<S::Ok, S::Error> {
        s.collect_str(&u.hyphenated())
    }

    pub(crate) fn deserialize<'de, D: Deserializer<'de>>(d: D) -> Result<Uuid, D::Error> {
        parse(&String::deserialize(d)?)
    }

    pub(crate) mod opt {
        use super::{Deserialize, Deserializer, Serializer, Uuid, parse};

        #[allow(clippy::ref_option, clippy::trivially_copy_pass_by_ref)]
        pub(crate) fn serialize<S: Serializer>(u: &Option<Uuid>, s: S) -> Result<S::Ok, S::Error> {
            match u {
                Some(u) => s.collect_str(&u.hyphenated()),
                None => s.serialize_none(),
            }
        }

        pub(crate) fn deserialize<'de, D: Deserializer<'de>>(
            d: D,
        ) -> Result<Option<Uuid>, D::Error> {
            Option::<String>::deserialize(d)?
                .map(|s| parse(&s))
                .transpose()
        }
    }
}
