//! The NetworkMessage envelopes `ua-data` and `ua-metadata` (MG 8.1, MG 8.2)
//! with their identifiers.

use core::fmt;
use core::str::FromStr;

use serde::de::{DeserializeOwned, Error as _};
use serde::{Deserialize, Deserializer, Serialize, Serializer};
use serde_json::Value;
use uuid::Uuid;

use super::helper::{Locale, Pagination, PaginationRequest, Query};
use super::metadata::DataSetMetaData;
use super::value::{ConfigurationVersion, DateTime};
use super::{DecodeError, guid};
use crate::il::IdentificationLink;
use crate::topic::ServiceType;

/// `<ServiceType>/<AppId>` of an application, the AppId as unencoded
/// Identification Link (REQ-MSG-006, REQ-ACT-005).
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct PublisherId {
    service_type: ServiceType,
    app_id: IdentificationLink,
}

impl PublisherId {
    /// The PublisherId of an application.
    pub fn new(service_type: ServiceType, app_id: IdentificationLink) -> Self {
        Self {
            service_type,
            app_id,
        }
    }

    /// The ServiceType.
    pub fn service_type(&self) -> ServiceType {
        self.service_type
    }

    /// The AppId.
    pub fn app_id(&self) -> &IdentificationLink {
        &self.app_id
    }
}

impl fmt::Display for PublisherId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}/{}", self.service_type, self.app_id)
    }
}

impl FromStr for PublisherId {
    type Err = DecodeError;

    fn from_str(s: &str) -> Result<Self, DecodeError> {
        let bad = |why: String| DecodeError::new(format!("PublisherId {s:?}: {why}"));
        let (st, il) = s
            .split_once('/')
            .ok_or_else(|| bad("no \"/\"".to_owned()))?;
        Ok(Self {
            service_type: st.parse().map_err(|e| bad(format!("{e}")))?,
            app_id: il.parse().map_err(|e| bad(format!("{e}")))?,
        })
    }
}

serde_str!(PublisherId);

/// The MessageId `<unixTimestampInMs>[<counter>]-<PublisherId>`
/// (REQ-MSG-007, REQ-ACT-004), also the PaginationId (REQ-PAG-003).
///
/// A receiver treats it as opaque (REQ-MSG-009): equality is the only
/// operation besides the text; the parser checks just the structure.
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct MessageId(String);

impl MessageId {
    /// A MessageId of `publisher` at the Unix time `unix_ms`; `counter`
    /// is appended to the timestamp if the publisher needs it for
    /// uniqueness (REQ-MSG-007, REQ-MSG-008).
    pub fn new(unix_ms: u64, counter: Option<u32>, publisher: &PublisherId) -> Self {
        match counter {
            Some(c) => Self(format!("{unix_ms}{c}-{publisher}")),
            None => Self(format!("{unix_ms}-{publisher}")),
        }
    }

    /// The text.
    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl fmt::Display for MessageId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl FromStr for MessageId {
    type Err = DecodeError;

    /// Checks `^[0-9]+-<ServiceType>/.+` (the schema pattern).
    fn from_str(s: &str) -> Result<Self, DecodeError> {
        let ok = s.split_once('-').is_some_and(|(ts, rest)| {
            !ts.is_empty()
                && ts.bytes().all(|b| b.is_ascii_digit())
                && rest
                    .split_once('/')
                    .is_some_and(|(st, id)| !id.is_empty() && st.parse::<ServiceType>().is_ok())
        });
        if ok {
            Ok(Self(s.to_owned()))
        } else {
            Err(DecodeError::new(format!(
                "MessageId {s:?} is not <unixTimestampInMs>-<PublisherId>"
            )))
        }
    }
}

serde_str!(MessageId);

/// The MessageIds of one publisher: unique (REQ-MSG-008) and with the
/// timestamp of their creation (REQ-MSG-007, REQ-LIFE-016).
///
/// The first id of a millisecond has no counter, the next ones of the same
/// millisecond carry the counter 1, 2, … . The timestamp never goes back:
/// after a backward step of the wall clock the generator stays at the last
/// millisecond and counts on, so uniqueness wins over clock precision.
/// While the timestamp has the same number of digits (13 digits until the
/// year 2286), two ids of the generator differ in their text.
#[derive(Debug, Clone)]
pub struct MessageIds {
    publisher: PublisherId,
    last_ms: u64,
    counter: Option<u32>,
}

impl MessageIds {
    /// The generator of `publisher`; one per application, shared by all its
    /// publications.
    pub fn new(publisher: PublisherId) -> Self {
        Self {
            publisher,
            last_ms: 0,
            counter: None,
        }
    }

    /// The PublisherId.
    pub fn publisher(&self) -> &PublisherId {
        &self.publisher
    }

    /// The next MessageId at the Unix time `unix_ms`.
    pub fn next_id(&mut self, unix_ms: u64) -> MessageId {
        let next = self.counter.map_or(Some(1), |c| c.checked_add(1));
        if unix_ms > self.last_ms {
            self.last_ms = unix_ms;
            self.counter = None;
        } else if let Some(c) = next {
            self.counter = Some(c);
        } else {
            self.last_ms += 1;
            self.counter = None;
        }
        MessageId::new(self.last_ms, self.counter, &self.publisher)
    }
}

/// ReplyTo: the MessageId of the triggering message, `None` (the empty
/// string) for a message that is no reaction (REQ-MSG-015).
mod reply_to {
    use serde::de::Error as _;
    use serde::{Deserialize, Deserializer, Serializer};

    use super::MessageId;

    #[allow(clippy::ref_option)]
    pub(super) fn serialize<S: Serializer>(v: &Option<MessageId>, s: S) -> Result<S::Ok, S::Error> {
        s.serialize_str(v.as_ref().map_or("", MessageId::as_str))
    }

    pub(super) fn deserialize<'de, D: Deserializer<'de>>(
        d: D,
    ) -> Result<Option<MessageId>, D::Error> {
        let s = String::deserialize(d)?;
        if s.is_empty() {
            Ok(None)
        } else {
            s.parse().map(Some).map_err(D::Error::custom)
        }
    }
}

/// A DataSetWriterId an application assigns, 10 or higher (REQ-MSG-020,
/// REQ-MSG-021). The reserved ids of the helper DataSets (REQ-MSG-019) and
/// of the Actions (REQ-ACT-019) are set by the SDK.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, PartialOrd, Ord, Serialize)]
#[serde(transparent)]
pub struct DataSetWriterId(u16);

impl DataSetWriterId {
    /// The first id of an application (REQ-MSG-020).
    pub const FIRST: Self = Self(10);

    /// An application DataSetWriterId.
    ///
    /// # Errors
    ///
    /// [`DecodeError`] for 0 … 9 (REQ-MSG-020).
    pub fn new(id: u16) -> Result<Self, DecodeError> {
        if id >= 10 {
            Ok(Self(id))
        } else {
            Err(DecodeError::new(format!(
                "DataSetWriterId {id} is reserved; applications use 10 or higher (REQ-MSG-020)"
            )))
        }
    }

    /// The id.
    pub fn get(self) -> u16 {
        self.0
    }
}

impl<'de> Deserialize<'de> for DataSetWriterId {
    fn deserialize<D: Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        Self::new(u16::deserialize(d)?).map_err(D::Error::custom)
    }
}

/// `MessageType` of a DataSetMessage (REQ-MSG-029, REQ-MSG-030).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum DataSetMessageType {
    /// `ua-keyframe`: every resource except Event.
    #[serde(rename = "ua-keyframe")]
    KeyFrame,
    /// `ua-event`: the resource Event.
    #[serde(rename = "ua-event")]
    Event,
}

/// The payload of a DataSetMessage: a resource or a helper DataSet.
pub trait DataSet: Serialize + DeserializeOwned {
    /// The predefined DataSetClassId (REQ-MSG-010); `None` for Data, whose
    /// DataSetClassId the publisher defines (REQ-MSG-011).
    const CLASS_ID: Option<Uuid>;
    /// The MessageType of its DataSetMessages (REQ-MSG-029).
    const MESSAGE_TYPE: DataSetMessageType = DataSetMessageType::KeyFrame;
    /// Reserved DataSetWriterId and WriterGroupName of a helper DataSet
    /// (REQ-MSG-019); `None` for a resource.
    #[doc(hidden)]
    const HELPER: Option<(u16, &'static str)> = None;
}

/// The Status of a DataSetMessage: only sent if not Good (REQ-MSG-026),
/// with the numeric Code and preferably the Symbol (REQ-MSG-027). A receiver
/// evaluates only the Code (REQ-MSG-028).
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct DataSetStatus {
    /// The StatusCode (Part 4), never 0 (Good).
    pub code: core::num::NonZeroU32,
    /// The symbolic name, e.g. `BadOutOfService`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub symbol: Option<String>,
}

/// Version of the DataSetMetaData a DataSetMessage refers to: at most one of
/// the two (REQ-MSG-018).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum DataSetVersion {
    /// The full ConfigurationVersion.
    MetaDataVersion(ConfigurationVersion),
    /// Only its MinorVersion.
    MinorVersion(u32),
}

/// A DataSetMessage (MG 8.1.2) with the fixed JsonDataSetMessageContentMask
/// (REQ-MSG-017): DataSetWriterId, DataSetWriterName and WriterGroupName are
/// always sent, PublisherId never; SequenceNumber, Timestamp, Status,
/// MessageType and one version are optional; the fields of the payload use
/// the VerboseEncoding (REQ-MSG-031).
#[derive(Debug, Clone, PartialEq)]
pub struct DataSetMessage<P> {
    data_set_writer_id: u16,
    data_set_writer_name: IdentificationLink,
    writer_group_name: String,
    sequence_number: Option<u32>,
    timestamp: Option<DateTime>,
    version: Option<DataSetVersion>,
    status: Option<DataSetStatus>,
    message_type: Option<DataSetMessageType>,
    payload: P,
}

impl<P: DataSet> DataSetMessage<P> {
    /// A DataSetMessage of a resource. `data_set_writer_name` is the asset
    /// (REQ-MSG-023), `writer_group_name` the topic-encoded Filter or the
    /// empty string (REQ-MSG-024, REQ-MSG-025). It carries the MessageType
    /// of the resource (REQ-MSG-029).
    pub fn new(
        data_set_writer_id: DataSetWriterId,
        data_set_writer_name: IdentificationLink,
        writer_group_name: impl Into<String>,
        payload: P,
    ) -> Self {
        Self::with_id(
            data_set_writer_id.get(),
            data_set_writer_name,
            writer_group_name.into(),
            payload,
        )
    }

    fn with_id(id: u16, name: IdentificationLink, group: String, payload: P) -> Self {
        Self {
            data_set_writer_id: id,
            data_set_writer_name: name,
            writer_group_name: group,
            sequence_number: None,
            timestamp: None,
            version: None,
            status: None,
            message_type: Some(P::MESSAGE_TYPE),
            payload,
        }
    }

    /// Sets the SequenceNumber.
    #[must_use]
    pub fn with_sequence_number(mut self, n: u32) -> Self {
        self.sequence_number = Some(n);
        self
    }

    /// Sets the Timestamp.
    #[must_use]
    pub fn with_timestamp(mut self, t: DateTime) -> Self {
        self.timestamp = Some(t);
        self
    }

    /// Sets MetaDataVersion or MinorVersion (REQ-MSG-018).
    #[must_use]
    pub fn with_version(mut self, v: DataSetVersion) -> Self {
        self.version = Some(v);
        self
    }

    /// Sets a Status that is not Good (REQ-MSG-026, REQ-MSG-027).
    #[must_use]
    pub fn with_status(mut self, s: DataSetStatus) -> Self {
        self.status = Some(s);
        self
    }

    /// Omits the MessageType (`ua-keyframe` is then assumed, REQ-MSG-029).
    #[must_use]
    pub fn without_message_type(mut self) -> Self {
        self.message_type = None;
        self
    }

    /// DataSetWriterId (REQ-MSG-016).
    pub fn data_set_writer_id(&self) -> u16 {
        self.data_set_writer_id
    }

    /// DataSetWriterName (REQ-MSG-023).
    pub fn data_set_writer_name(&self) -> &IdentificationLink {
        &self.data_set_writer_name
    }

    /// WriterGroupName (REQ-MSG-024).
    pub fn writer_group_name(&self) -> &str {
        &self.writer_group_name
    }

    /// SequenceNumber.
    pub fn sequence_number(&self) -> Option<u32> {
        self.sequence_number
    }

    /// Timestamp.
    pub fn timestamp(&self) -> Option<&DateTime> {
        self.timestamp.as_ref()
    }

    /// MetaDataVersion or MinorVersion.
    pub fn version(&self) -> Option<DataSetVersion> {
        self.version
    }

    /// Status; `None` is Good (REQ-MSG-026, REQ-MSG-028).
    pub fn status(&self) -> Option<&DataSetStatus> {
        self.status.as_ref()
    }

    /// The MessageType as sent.
    pub fn message_type(&self) -> Option<DataSetMessageType> {
        self.message_type
    }

    /// The payload.
    pub fn payload(&self) -> &P {
        &self.payload
    }

    /// Consumes the message and returns the payload.
    pub fn into_payload(self) -> P {
        self.payload
    }
}

impl<H: DataSet> DataSetMessage<H> {
    /// A DataSetMessage of a helper DataSet with its reserved
    /// DataSetWriterId and WriterGroupName; `app` is the publishing
    /// application (REQ-MSG-019).
    ///
    /// # Panics
    ///
    /// If `H` is a resource and not a helper DataSet.
    pub fn helper(app: IdentificationLink, payload: H) -> Self {
        let (id, name) = H::HELPER.expect("a helper DataSet");
        Self::with_id(id, app, name.to_owned(), payload)
    }
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
struct DataSetMessageWire<P> {
    data_set_writer_id: u16,
    data_set_writer_name: IdentificationLink,
    writer_group_name: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    sequence_number: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    timestamp: Option<DateTime>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    meta_data_version: Option<ConfigurationVersion>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    minor_version: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    status: Option<DataSetStatus>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    message_type: Option<DataSetMessageType>,
    payload: P,
}

impl<P: DataSet> Serialize for DataSetMessage<P> {
    fn serialize<S: Serializer>(&self, s: S) -> Result<S::Ok, S::Error> {
        let (meta_data_version, minor_version) = match self.version {
            Some(DataSetVersion::MetaDataVersion(v)) => (Some(v), None),
            Some(DataSetVersion::MinorVersion(v)) => (None, Some(v)),
            None => (None, None),
        };
        DataSetMessageWire {
            data_set_writer_id: self.data_set_writer_id,
            data_set_writer_name: self.data_set_writer_name.clone(),
            writer_group_name: self.writer_group_name.clone(),
            sequence_number: self.sequence_number,
            timestamp: self.timestamp.clone(),
            meta_data_version,
            minor_version,
            status: self.status.clone(),
            message_type: self.message_type,
            payload: &self.payload,
        }
        .serialize(s)
    }
}

impl<'de, P: DataSet> Deserialize<'de> for DataSetMessage<P> {
    fn deserialize<D: Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        let w = DataSetMessageWire::<P>::deserialize(d)?;
        let id = w.data_set_writer_id;
        match P::HELPER {
            Some((hid, name)) if id != hid || w.writer_group_name != name => {
                return Err(D::Error::custom(format!(
                    "the {name} DataSet has DataSetWriterId {hid} and WriterGroupName {name:?} (REQ-MSG-019)"
                )));
            }
            None => {
                DataSetWriterId::new(id).map_err(D::Error::custom)?;
            }
            Some(_) => {}
        }
        if P::HELPER.is_none() && w.message_type.is_some_and(|t| t != P::MESSAGE_TYPE) {
            return Err(D::Error::custom(
                "MessageType does not fit the resource (REQ-MSG-029)",
            ));
        }
        let version = match (w.meta_data_version, w.minor_version) {
            (Some(_), Some(_)) => {
                return Err(D::Error::custom(
                    "MetaDataVersion and MinorVersion are not combined (REQ-MSG-018)",
                ));
            }
            (Some(v), None) => Some(DataSetVersion::MetaDataVersion(v)),
            (None, Some(v)) => Some(DataSetVersion::MinorVersion(v)),
            (None, None) => None,
        };
        Ok(Self {
            data_set_writer_id: id,
            data_set_writer_name: w.data_set_writer_name,
            writer_group_name: w.writer_group_name,
            sequence_number: w.sequence_number,
            timestamp: w.timestamp,
            version,
            status: w.status,
            message_type: w.message_type,
            payload: w.payload,
        })
    }
}

/// One item of `Messages`: a helper DataSet by its reserved DataSetWriterId
/// (REQ-MSG-019) or a DataSetMessage of the resource.
#[derive(Debug, Clone, PartialEq)]
pub enum DataSetEntry<P> {
    /// DataSetWriterId 1 (REQ-PAG-001).
    PaginationRequest(DataSetMessage<PaginationRequest>),
    /// DataSetWriterId 2 (REQ-PAG-003).
    Pagination(DataSetMessage<Pagination>),
    /// DataSetWriterId 3 (REQ-LOC-001).
    Locale(DataSetMessage<Locale>),
    /// DataSetWriterId 9 (REQ-QRY-001).
    Query(DataSetMessage<Query>),
    /// A DataSetMessage of the resource (DataSetWriterId 10 or higher).
    Payload(DataSetMessage<P>),
}

impl<P: DataSet> Serialize for DataSetEntry<P> {
    fn serialize<S: Serializer>(&self, s: S) -> Result<S::Ok, S::Error> {
        match self {
            Self::PaginationRequest(m) => m.serialize(s),
            Self::Pagination(m) => m.serialize(s),
            Self::Locale(m) => m.serialize(s),
            Self::Query(m) => m.serialize(s),
            Self::Payload(m) => m.serialize(s),
        }
    }
}

impl<'de, P: DataSet> Deserialize<'de> for DataSetEntry<P> {
    fn deserialize<D: Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        let v = Value::deserialize(d)?;
        match v.get("DataSetWriterId").and_then(Value::as_u64) {
            Some(1) => serde_json::from_value(v).map(Self::PaginationRequest),
            Some(2) => serde_json::from_value(v).map(Self::Pagination),
            Some(3) => serde_json::from_value(v).map(Self::Locale),
            Some(9) => serde_json::from_value(v).map(Self::Query),
            _ => serde_json::from_value(v).map(Self::Payload),
        }
        .map_err(D::Error::custom)
    }
}

/// The reserved DataSetClassIds of MG A2 (REQ-MSG-014).
pub const RESERVED_CLASS_IDS: [Uuid; 26] = [
    uuid::uuid!("2ae0505e-2830-4980-b65e-0bbdf08e2d45"),
    uuid::uuid!("a6e6c727-4057-419f-b2ea-3fe9173e71cf"),
    uuid::uuid!("ebd12d4b-da1c-4671-ab86-db102fecc603"),
    uuid::uuid!("96d22d73-bce6-42d3-9949-45e0d04e4d54"),
    uuid::uuid!("32c0a57a-24fd-4378-8eb3-4a42a481bf56"),
    uuid::uuid!("0721c712-f7b5-4ad0-b2a0-58f0a5744963"),
    uuid::uuid!("d08ade63-777b-464a-8f58-73649dc3ec1c"),
    uuid::uuid!("965ab782-4669-480e-bab4-c6a3b14ee4cf"),
    uuid::uuid!("d2ee7674-cad5-4b59-a37c-3631bb0cd409"),
    uuid::uuid!("a9031581-4c03-4c6a-9c12-dd7fe0123ef5"),
    uuid::uuid!("04cac81f-9056-4af3-a8a0-686fc499b5f9"),
    uuid::uuid!("a04f4f5f-b2cc-40d7-9a6a-0884233ccce7"),
    uuid::uuid!("2b967128-5b7e-4c0f-a6eb-c59af1118ce6"),
    uuid::uuid!("54b8a5c2-812d-442e-8a91-257af7304161"),
    uuid::uuid!("2b4e35a2-437a-4a3d-b007-9b1c27b08127"),
    uuid::uuid!("97439c90-7d10-47b8-8ada-6913a466107d"),
    uuid::uuid!("4292bf9f-b9d0-4de1-a33e-f4736356d4c1"),
    uuid::uuid!("0b3b07af-9cf9-4b40-977f-75b5b02bb0d5"),
    uuid::uuid!("31c6e1c0-88c7-453b-ae9d-85fb6bc50275"),
    uuid::uuid!("3b4a62ba-026f-4ee8-bc99-3a5f85fc9f3b"),
    uuid::uuid!("760abda2-ba40-4e6e-863a-eea8c002b4e4"),
    uuid::uuid!("414e26f6-341b-43b7-90fc-bb9e0b1b0866"),
    uuid::uuid!("3b423a40-a676-4ba0-8017-f0b2cd65bc26"),
    uuid::uuid!("cfa96d87-e65c-42f4-9d9e-026797c37f4c"),
    uuid::uuid!("0c959a0f-ec83-43c1-9ccc-20ab20a0e171"),
    uuid::uuid!("bb725543-9de1-4f22-883f-d32c2de5dea1"),
];

/// A NetworkMessage `ua-data` (MG 8.1.1) with the fields of REQ-MSG-003 and
/// the fixed JsonNetworkMessageContentMask (REQ-MSG-004): PublisherId and ReplyTo are
/// always sent, WriterGroupName and SingleDataSetMessage never, the
/// DataSetClassId as REQ-MSG-011 requires.
#[derive(Debug, Clone, PartialEq)]
pub struct DataNetworkMessage<P> {
    message_id: MessageId,
    publisher_id: PublisherId,
    data_set_class_id: Option<Uuid>,
    reply_to: Option<MessageId>,
    messages: Vec<DataSetEntry<P>>,
}

impl<P: DataSet> DataNetworkMessage<P> {
    /// An empty NetworkMessage of the resource `P`; it carries the
    /// predefined DataSetClassId of `P` (REQ-MSG-010, REQ-MSG-011) and the
    /// empty ReplyTo (REQ-MSG-015).
    pub fn new(message_id: MessageId, publisher_id: PublisherId) -> Self {
        Self {
            message_id,
            publisher_id,
            data_set_class_id: P::CLASS_ID,
            reply_to: None,
            messages: Vec::new(),
        }
    }

    /// Sets ReplyTo to the MessageId of the triggering message
    /// (REQ-MSG-015).
    #[must_use]
    pub fn in_reply_to(mut self, trigger: MessageId) -> Self {
        self.reply_to = Some(trigger);
        self
    }

    /// Sets the publisher-defined DataSetClassId of Data (REQ-MSG-011,
    /// REQ-MSG-013).
    ///
    /// # Errors
    ///
    /// [`DecodeError`] if `P` has a predefined DataSetClassId
    /// (REQ-MSG-010) or the id is reserved (REQ-MSG-014).
    pub fn with_data_set_class_id(mut self, id: Uuid) -> Result<Self, DecodeError> {
        check_class_id::<P>(Some(id))?;
        self.data_set_class_id = Some(id);
        Ok(self)
    }

    /// Appends a DataSetMessage.
    #[must_use]
    pub fn with(mut self, entry: DataSetEntry<P>) -> Self {
        self.messages.push(entry);
        self
    }

    /// Appends a DataSetMessage.
    pub fn push(&mut self, entry: DataSetEntry<P>) {
        self.messages.push(entry);
    }

    /// MessageId (REQ-MSG-007).
    pub fn message_id(&self) -> &MessageId {
        &self.message_id
    }

    /// PublisherId (REQ-MSG-006).
    pub fn publisher_id(&self) -> &PublisherId {
        &self.publisher_id
    }

    /// DataSetClassId (REQ-MSG-011).
    pub fn data_set_class_id(&self) -> Option<Uuid> {
        self.data_set_class_id
    }

    /// ReplyTo; `None` is the empty string (REQ-MSG-015).
    pub fn reply_to(&self) -> Option<&MessageId> {
        self.reply_to.as_ref()
    }

    /// Messages (REQ-MSG-005).
    pub fn messages(&self) -> &[DataSetEntry<P>] {
        &self.messages
    }

    /// Messages (REQ-MSG-005), taken out of the NetworkMessage.
    pub fn into_messages(self) -> Vec<DataSetEntry<P>> {
        self.messages
    }

    /// The DataSetMessages of the resource, without helper DataSets.
    pub fn payloads(&self) -> impl Iterator<Item = &DataSetMessage<P>> {
        self.messages.iter().filter_map(|m| match m {
            DataSetEntry::Payload(p) => Some(p),
            _ => None,
        })
    }
}

fn check_class_id<P: DataSet>(id: Option<Uuid>) -> Result<(), DecodeError> {
    match (P::CLASS_ID, id) {
        (Some(want), Some(got)) if want == got => Ok(()),
        (Some(want), _) => Err(DecodeError::new(format!(
            "the resource has the DataSetClassId {want} (REQ-MSG-010, REQ-MSG-011)"
        ))),
        (None, Some(got)) if RESERVED_CLASS_IDS.contains(&got) => Err(DecodeError::new(format!(
            "DataSetClassId {got} is reserved (REQ-MSG-014)"
        ))),
        (None, _) => Ok(()),
    }
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
struct DataWire<M> {
    message_id: MessageId,
    message_type: String,
    publisher_id: PublisherId,
    #[serde(default, skip_serializing_if = "Option::is_none", with = "guid::opt")]
    data_set_class_id: Option<Uuid>,
    #[serde(with = "reply_to")]
    reply_to: Option<MessageId>,
    messages: M,
}

impl<P: DataSet> Serialize for DataNetworkMessage<P> {
    fn serialize<S: Serializer>(&self, s: S) -> Result<S::Ok, S::Error> {
        DataWire {
            message_id: self.message_id.clone(),
            message_type: "ua-data".to_owned(),
            publisher_id: self.publisher_id.clone(),
            data_set_class_id: self.data_set_class_id,
            reply_to: self.reply_to.clone(),
            messages: &self.messages,
        }
        .serialize(s)
    }
}

impl<'de, P: DataSet> Deserialize<'de> for DataNetworkMessage<P> {
    fn deserialize<D: Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        let w = DataWire::<Vec<DataSetEntry<P>>>::deserialize(d)?;
        if w.message_type != "ua-data" {
            return Err(D::Error::custom("MessageType is not ua-data (REQ-MSG-002)"));
        }
        check_class_id::<P>(w.data_set_class_id).map_err(D::Error::custom)?;
        Ok(Self {
            message_id: w.message_id,
            publisher_id: w.publisher_id,
            data_set_class_id: w.data_set_class_id,
            reply_to: w.reply_to,
            messages: w.messages,
        })
    }
}

/// A DataSetMetaData `ua-metadata` (MG 8.2; REQ-MSG-032). All its fields are
/// required; their invariants live in their types.
#[derive(Debug, Clone, PartialEq)]
pub struct MetaDataNetworkMessage {
    /// MessageId (REQ-MSG-007).
    pub message_id: MessageId,
    /// PublisherId (REQ-MSG-006).
    pub publisher_id: PublisherId,
    /// DataSetWriterId of the described DataSetMessage (REQ-MSG-033).
    pub data_set_writer_id: DataSetWriterId,
    /// WriterGroupName of the described DataSetMessage (REQ-MSG-024,
    /// REQ-MSG-033).
    pub writer_group_name: String,
    /// DataSetWriterName of the described DataSetMessage (REQ-MSG-023,
    /// REQ-MSG-033).
    pub data_set_writer_name: IdentificationLink,
    /// Time of the first sending (REQ-MSG-032).
    pub timestamp: DateTime,
    /// ReplyTo; `None` is the empty string (REQ-MSG-015).
    pub reply_to: Option<MessageId>,
    /// The DataSetMetaDataType; `None` is the empty object of a Get
    /// (REQ-RES-META-001), which a Pub never sends (REQ-RES-META-003).
    pub meta_data: Option<DataSetMetaData>,
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
struct MetaDataWire {
    message_id: MessageId,
    message_type: String,
    publisher_id: PublisherId,
    data_set_writer_id: DataSetWriterId,
    writer_group_name: String,
    data_set_writer_name: IdentificationLink,
    timestamp: DateTime,
    #[serde(with = "reply_to")]
    reply_to: Option<MessageId>,
    meta_data: Value,
}

impl Serialize for MetaDataNetworkMessage {
    fn serialize<S: Serializer>(&self, s: S) -> Result<S::Ok, S::Error> {
        let meta_data = match &self.meta_data {
            Some(m) => serde_json::to_value(m).map_err(serde::ser::Error::custom)?,
            None => Value::Object(serde_json::Map::new()),
        };
        MetaDataWire {
            message_id: self.message_id.clone(),
            message_type: "ua-metadata".to_owned(),
            publisher_id: self.publisher_id.clone(),
            data_set_writer_id: self.data_set_writer_id,
            writer_group_name: self.writer_group_name.clone(),
            data_set_writer_name: self.data_set_writer_name.clone(),
            timestamp: self.timestamp.clone(),
            reply_to: self.reply_to.clone(),
            meta_data,
        }
        .serialize(s)
    }
}

impl<'de> Deserialize<'de> for MetaDataNetworkMessage {
    fn deserialize<D: Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        let w = MetaDataWire::deserialize(d)?;
        if w.message_type != "ua-metadata" {
            return Err(D::Error::custom(
                "MessageType is not ua-metadata (REQ-MSG-002)",
            ));
        }
        let meta_data = match w.meta_data {
            Value::Object(m) if m.is_empty() => None,
            v => Some(serde_json::from_value(v).map_err(D::Error::custom)?),
        };
        Ok(Self {
            message_id: w.message_id,
            publisher_id: w.publisher_id,
            data_set_writer_id: w.data_set_writer_id,
            writer_group_name: w.writer_group_name,
            data_set_writer_name: w.data_set_writer_name,
            timestamp: w.timestamp,
            reply_to: w.reply_to,
            meta_data,
        })
    }
}
