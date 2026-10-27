//! Decoding of a received publication into its DataSetMessages with the
//! topic context.

use serde_json::Value;

use crate::il::IdentificationLink;
use crate::message::{
    DataNetworkMessage, DataSet, DataSetEntry, DataSetMetaData, DataSetStatus, DateTime,
    DecodeError, Message, MessageId, Pagination, PublisherId,
};
use crate::resource::{
    Config, Data, Event, EventCategory, Health, Mam, Profile, PublicationEntry,
    ReferenceDesignation, SubscriptionEntry,
};
use crate::status::StatusCode;
use crate::topic::Topic;

/// The typed Payload of a received DataSetMessage. Unknown fields were
/// ignored (tolerant reader, REQ-MSG-046); [`Received::json`] keeps them.
#[derive(Debug, Clone, PartialEq)]
#[non_exhaustive]
pub enum Payload {
    /// MAM (MG 9.1.1).
    Mam(Mam),
    /// Health (MG 9.1.2).
    Health(Health),
    /// Config (MG 9.1.3).
    Config(Config),
    /// Data (MG 9.1.4).
    Data(Data),
    /// The filled DataSetMetaData of a Data DataSet (MG 9.1.5).
    MetaData(Box<DataSetMetaData>),
    /// Event (MG 9.1.6).
    Event(Event),
    /// Profile (MG 9.1.7).
    Profile(Profile),
    /// One PublicationList entry (MG 9.1.8).
    PublicationList(PublicationEntry),
    /// One SubscriptionList entry (MG 9.1.9).
    SubscriptionList(SubscriptionEntry),
    /// ReferenceDesignation (MG 9.1.10).
    ReferenceDesignation(ReferenceDesignation),
}

impl Payload {
    /// The StatusCode of a Status event (REQ-RES-EVENT-003), else `None`.
    pub fn status(&self) -> Option<StatusCode> {
        match self {
            Self::Event(e) if e.category == EventCategory::Status => Some(StatusCode(e.number)),
            _ => None,
        }
    }
}

/// One received DataSetMessage of a resource with the context of its
/// NetworkMessage and topic; helper DataSets (REQ-MSG-019) are not
/// delivered as items.
#[derive(Debug, Clone, PartialEq)]
pub struct Received {
    /// The topic it was published on.
    pub topic: Topic,
    /// MessageId of the NetworkMessage (REQ-MSG-007).
    pub message_id: MessageId,
    /// PublisherId of the NetworkMessage (REQ-MSG-006).
    pub publisher_id: PublisherId,
    /// ReplyTo of the NetworkMessage; `None` for a publication not
    /// triggered by a request (REQ-MSG-015).
    pub reply_to: Option<MessageId>,
    /// DataSetWriterId (REQ-MSG-018).
    pub data_set_writer_id: u16,
    /// DataSetWriterName: the asset (REQ-MSG-023).
    pub data_set_writer_name: IdentificationLink,
    /// WriterGroupName: the topic-encoded Filter or empty (REQ-MSG-024).
    pub writer_group_name: String,
    /// SequenceNumber, if sent.
    pub sequence_number: Option<u32>,
    /// Timestamp, if sent.
    pub timestamp: Option<DateTime>,
    /// Status of the DataSetMessage, if not Good (REQ-MSG-026).
    pub status: Option<DataSetStatus>,
    /// The typed Payload.
    pub payload: Payload,
    /// The Payload as received, unknown fields included.
    pub json: Value,
}

/// A received publication: its resource DataSetMessages and the
/// Pagination helper, if any.
#[derive(Debug, Clone, PartialEq)]
pub(crate) struct Decoded {
    pub(crate) reply_to: Option<MessageId>,
    pub(crate) pagination: Option<Pagination>,
    pub(crate) items: Vec<Received>,
}

/// Decodes a `Pub` NetworkMessage received on `topic`.
pub(crate) fn decode(topic: &Topic, payload: &[u8]) -> Result<Decoded, DecodeError> {
    let v: Value = serde_json::from_slice(payload)?;
    let raw: Vec<Value> = match v.get("Messages") {
        Some(Value::Array(all)) => all
            .iter()
            .map(|m| m.get("Payload").cloned().unwrap_or(Value::Null))
            .collect(),
        _ => Vec::new(),
    };
    let meta_json = v.get("MetaData").cloned();
    Ok(match Message::from_value(topic, v)? {
        Message::Mam(m) => data(topic, m, &raw, Payload::Mam),
        Message::Health(m) => data(topic, m, &raw, Payload::Health),
        Message::Config(m) => data(topic, m, &raw, Payload::Config),
        Message::Data(m) => data(topic, m, &raw, Payload::Data),
        Message::Event(m) => data(topic, m, &raw, Payload::Event),
        Message::Profile(m) => data(topic, m, &raw, Payload::Profile),
        Message::PublicationList(m) => data(topic, m, &raw, Payload::PublicationList),
        Message::SubscriptionList(m) => data(topic, m, &raw, Payload::SubscriptionList),
        Message::ReferenceDesignation(m) => data(topic, m, &raw, Payload::ReferenceDesignation),
        Message::MetaData(m) => {
            let Some(meta) = m.meta_data else {
                return Err(DecodeError::new(
                    "a Pub Metadata carries the filled DataSetMetaData (REQ-RES-META-003)",
                ));
            };
            Decoded {
                reply_to: m.reply_to.clone(),
                pagination: None,
                items: vec![Received {
                    topic: topic.clone(),
                    message_id: m.message_id,
                    publisher_id: m.publisher_id,
                    reply_to: m.reply_to,
                    data_set_writer_id: m.data_set_writer_id.get(),
                    data_set_writer_name: m.data_set_writer_name,
                    writer_group_name: m.writer_group_name,
                    sequence_number: None,
                    timestamp: Some(m.timestamp),
                    status: None,
                    payload: Payload::MetaData(Box::new(meta)),
                    json: meta_json.unwrap_or(Value::Null),
                }],
            }
        }
        Message::ConfigSet(_)
        | Message::NewDataSetWriterIdRequest(_)
        | Message::NewDataSetWriterIdResponse(_) => {
            return Err(DecodeError::new("not a publication of a resource"));
        }
    })
}

fn data<P: DataSet>(
    topic: &Topic,
    m: DataNetworkMessage<P>,
    raw: &[Value],
    wrap: fn(P) -> Payload,
) -> Decoded {
    let message_id = m.message_id().clone();
    let publisher_id = m.publisher_id().clone();
    let reply_to = m.reply_to().cloned();
    let mut pagination = None;
    let mut items = Vec::new();
    for (i, entry) in m.into_messages().into_iter().enumerate() {
        match entry {
            DataSetEntry::Pagination(p) => pagination = Some(p.into_payload()),
            DataSetEntry::Payload(d) => items.push(Received {
                topic: topic.clone(),
                message_id: message_id.clone(),
                publisher_id: publisher_id.clone(),
                reply_to: reply_to.clone(),
                data_set_writer_id: d.data_set_writer_id(),
                data_set_writer_name: d.data_set_writer_name().clone(),
                writer_group_name: d.writer_group_name().to_owned(),
                sequence_number: d.sequence_number(),
                timestamp: d.timestamp().cloned(),
                status: d.status().cloned(),
                json: raw.get(i).cloned().unwrap_or(Value::Null),
                payload: wrap(d.into_payload()),
            }),
            DataSetEntry::PaginationRequest(_)
            | DataSetEntry::Locale(_)
            | DataSetEntry::Query(_) => {}
        }
    }
    Decoded {
        reply_to,
        pagination,
        items,
    }
}
