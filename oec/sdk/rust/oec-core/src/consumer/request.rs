//! The requests of a consumer: `Get` with its helper DataSets, `Set` and
//! `Del` (MG 7.4, MG 8.3, MG 9.1).

use serde_json::Value;
use time::OffsetDateTime;

use super::ConsumerError;
use crate::il::IdentificationLink;
use crate::message::{
    DataNetworkMessage, DataSet, DataSetEntry, DataSetMessage, DataSetWriterId, DateTime, Locale,
    MessageId, MetaDataNetworkMessage, PaginationRequest, PublisherId, Query,
};
use crate::resource::{
    Config, ConfigSet, Data, Health, Mam, Profile, PublicationEntry, ReferenceDesignation,
    ReferenceDesignationQuery, SubscriptionEntry,
};
use crate::topic::{Filter, Form, Method, Resource, ServiceType, Subject, Topic};

/// The addressed application and, optionally, asset and Filter of a
/// request (MG 7): `Oi4v2/<ServiceType>/<AppId>/<Method>/<Resource>[/<Source>[/<Filter>]]`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Address {
    /// ServiceType of the addressed application.
    pub service_type: ServiceType,
    /// AppId of the addressed application.
    pub app: IdentificationLink,
    /// The addressed asset; `None` addresses all assets of the application.
    pub source: Option<IdentificationLink>,
    /// The Filter; needs a Source.
    pub filter: Option<Filter>,
}

impl Address {
    /// The application, without Source and Filter.
    pub fn app(service_type: ServiceType, app: IdentificationLink) -> Self {
        Self {
            service_type,
            app,
            source: None,
            filter: None,
        }
    }

    /// The topic of a request.
    ///
    /// # Errors
    ///
    /// [`ConsumerError::Topic`] if the combination is not allowed or Source
    /// or Filter are missing or forbidden (REQ-TOP-013).
    pub fn topic(&self, method: Method, resource: Resource) -> Result<Topic, ConsumerError> {
        Topic::new(
            self.service_type,
            self.app.clone(),
            method,
            resource,
            self.source.clone(),
            self.filter.clone(),
        )
        .map_err(ConsumerError::Topic)
    }

    /// The topic prefix of every publication of the addressed application:
    /// `Oi4v2/<ServiceType>/<AppId>/Pub/`.
    pub(super) fn pub_prefix(&self) -> String {
        format!(
            "{}/{}/{}/Pub/",
            crate::topic::NAMESPACE,
            self.service_type,
            self.app.encode()
        )
    }
}

/// The helper DataSets of a `Get` (MG 8.3).
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct GetParams {
    /// PaginationRequest with this PerPage and Page 0, i.e. all pages
    /// (REQ-PAG-001, REQ-PAG-002); `None` sends none and the responder
    /// paginates only what does not fit (REQ-PAG-007).
    pub per_page: Option<u32>,
    /// Locale (REQ-LOC-001 … 003).
    pub locale: Option<String>,
    /// Query of ReferenceDesignation, only without Source (REQ-QRY-001,
    /// REQ-RES-REFDES-012/013); the answer is checked against it
    /// (REQ-RES-REFDES-015).
    pub query: Option<ReferenceDesignationQuery>,
    /// The DataSetWriterId of the Data DataSet whose Metadata is requested
    /// (REQ-RES-META-001, MG 8.2); needed for [`Resource::Metadata`] only.
    pub data_set_writer_id: Option<DataSetWriterId>,
}

/// The one DataSetMessage of a `Set` or `Del` (REQ-BUS-010): the
/// DataSetWriterId of the addressed DataSet and its Payload; the
/// DataSetWriterName is the Source, the WriterGroupName the topic-encoded
/// Filter or empty (REQ-MSG-023, REQ-MSG-024).
#[derive(Debug, Clone, PartialEq)]
pub struct WriteEntry {
    /// DataSetWriterId of the addressed DataSet.
    pub data_set_writer_id: DataSetWriterId,
    /// The Payload: the resource object (for Metadata the DataSetMetaData).
    pub payload: Value,
}

pub(super) fn date_time(unix_ms: u64) -> DateTime {
    let at = OffsetDateTime::from_unix_timestamp_nanos(i128::from(unix_ms) * 1_000_000)
        .unwrap_or(OffsetDateTime::UNIX_EPOCH);
    DateTime::from(at)
}

/// What every request shares.
pub(super) struct Sender<'a> {
    pub(super) app: &'a IdentificationLink,
    pub(super) publisher: &'a PublisherId,
    pub(super) message_id: MessageId,
    pub(super) unix_ms: u64,
}

/// The `Get` NetworkMessage (REQ-RES-*-Get: no resource object, only
/// helper DataSets; Metadata as an empty DataSetMetaData).
pub(super) fn get(
    resource: Resource,
    address: &Address,
    params: &GetParams,
    sender: Sender<'_>,
) -> Result<Vec<u8>, ConsumerError> {
    if params.query.is_some()
        && (resource != Resource::ReferenceDesignation || address.source.is_some())
    {
        return Err(ConsumerError::Request(
            "a Query only in a Get of ReferenceDesignation without Source (REQ-QRY-001, REQ-RES-REFDES-013)"
                .to_owned(),
        ));
    }
    if params.per_page == Some(0) {
        return Err(ConsumerError::Request(
            "PerPage is at least 1 (REQ-PAG-001)".to_owned(),
        ));
    }
    let helpers = Helpers { params, sender };
    match resource {
        Resource::Mam => helpers.encode::<Mam>(),
        Resource::Health => helpers.encode::<Health>(),
        Resource::Config => helpers.encode::<Config>(),
        Resource::Data => helpers.encode::<Data>(),
        Resource::Profile => helpers.encode::<Profile>(),
        Resource::PublicationList => helpers.encode::<PublicationEntry>(),
        Resource::SubscriptionList => helpers.encode::<SubscriptionEntry>(),
        Resource::ReferenceDesignation => helpers.encode::<ReferenceDesignation>(),
        Resource::Metadata => get_meta_data(address, params, helpers.sender),
        Resource::Event => Err(ConsumerError::Request(
            "Event has no Get (REQ-TOP-013)".to_owned(),
        )),
    }
}

struct Helpers<'a> {
    params: &'a GetParams,
    sender: Sender<'a>,
}

impl Helpers<'_> {
    fn encode<P: DataSet>(self) -> Result<Vec<u8>, ConsumerError> {
        let app = self.sender.app;
        let mut m =
            DataNetworkMessage::<P>::new(self.sender.message_id, self.sender.publisher.clone());
        if let Some(per_page) = self.params.per_page {
            let h = DataSetMessage::helper(app.clone(), PaginationRequest { per_page, page: 0 });
            m.push(DataSetEntry::PaginationRequest(h.without_message_type()));
        }
        if let Some(locale) = &self.params.locale {
            let locale = locale.clone();
            let h = DataSetMessage::helper(app.clone(), Locale { locale });
            m.push(DataSetEntry::Locale(h.without_message_type()));
        }
        if let Some(q) = &self.params.query {
            let h = DataSetMessage::helper(app.clone(), Query::from(q.clone()));
            m.push(DataSetEntry::Query(h.without_message_type()));
        }
        encode(&m)
    }
}

fn get_meta_data(
    address: &Address,
    params: &GetParams,
    sender: Sender<'_>,
) -> Result<Vec<u8>, ConsumerError> {
    let (Some(source), Some(Filter::Tag(tag)), Some(id)) =
        (&address.source, &address.filter, params.data_set_writer_id)
    else {
        return Err(ConsumerError::Request(
            "a Get Metadata addresses Source and Tag and carries the DataSetWriterId of the Data DataSet (REQ-RES-META-001)"
                .to_owned(),
        ));
    };
    let m = MetaDataNetworkMessage {
        message_id: sender.message_id,
        publisher_id: sender.publisher.clone(),
        data_set_writer_id: id,
        writer_group_name: Filter::Tag(tag.clone()).to_string(),
        data_set_writer_name: source.clone(),
        timestamp: date_time(sender.unix_ms),
        reply_to: None,
        meta_data: None,
    };
    encode(&m)
}

/// The `Set` or `Del` NetworkMessage: exactly one DataSetMessage of the
/// resource (REQ-BUS-010), none for a `Del ReferenceDesignation`
/// (REQ-RES-REFDES-008); the requester omits the publisher-defined
/// DataSetClassId of Data (REQ-MSG-011) and sends no SequenceNumber.
/// Like the requests of the guideline (MG B1), the DataSetMessages of a
/// request carry no MessageType ("ua-keyframe" is assumed, REQ-MSG-029).
pub(super) fn write(
    method: Method,
    resource: Resource,
    address: &Address,
    entry: Option<&WriteEntry>,
    sender: Sender<'_>,
) -> Result<Vec<u8>, ConsumerError> {
    let form = Form::of(method, Subject::Resource(resource));
    if !matches!(method, Method::Set | Method::Del) || form.is_none() {
        return Err(ConsumerError::Request(format!(
            "{method}/{resource} is not an allowed combination (REQ-TOP-013)"
        )));
    }
    let del_ref = method == Method::Del && resource == Resource::ReferenceDesignation;
    let entry = match (entry, del_ref) {
        (None, true) => None,
        (Some(e), false) => Some(e),
        (Some(_), true) => {
            return Err(ConsumerError::Request(
                "a Del ReferenceDesignation carries no object (REQ-RES-REFDES-008)".to_owned(),
            ));
        }
        (None, false) => {
            return Err(ConsumerError::Request(format!(
                "a {method} {resource} carries exactly one DataSetMessage (REQ-BUS-010)"
            )));
        }
    };
    let source = address.source.as_ref().ok_or_else(|| {
        ConsumerError::Request(format!("a {method} {resource} addresses a Source"))
    })?;
    let group = address
        .filter
        .as_ref()
        .map(ToString::to_string)
        .unwrap_or_default();
    let item = Item {
        entry,
        source,
        group,
        sender,
    };
    match resource {
        Resource::Config => item.encode::<ConfigSet>(),
        Resource::Data => item.encode::<Data>(),
        Resource::PublicationList => item.encode::<PublicationEntry>(),
        Resource::SubscriptionList => item.encode::<SubscriptionEntry>(),
        Resource::ReferenceDesignation => item.encode::<ReferenceDesignation>(),
        Resource::Metadata => item.meta_data(),
        Resource::Mam | Resource::Health | Resource::Event | Resource::Profile => {
            unreachable!("no Set or Del form")
        }
    }
}

struct Item<'a> {
    entry: Option<&'a WriteEntry>,
    source: &'a IdentificationLink,
    group: String,
    sender: Sender<'a>,
}

impl Item<'_> {
    fn encode<P: DataSet>(self) -> Result<Vec<u8>, ConsumerError> {
        let mut m =
            DataNetworkMessage::<P>::new(self.sender.message_id, self.sender.publisher.clone());
        if let Some(e) = self.entry {
            let payload: P = serde_json::from_value(e.payload.clone())
                .map_err(|e| ConsumerError::Request(format!("the Payload: {e}")))?;
            m.push(DataSetEntry::Payload(
                DataSetMessage::new(
                    e.data_set_writer_id,
                    self.source.clone(),
                    self.group,
                    payload,
                )
                .with_timestamp(date_time(self.sender.unix_ms))
                .without_message_type(),
            ));
        }
        encode(&m)
    }

    fn meta_data(self) -> Result<Vec<u8>, ConsumerError> {
        let e = self.entry.expect("a Set Metadata has its entry");
        let meta = serde_json::from_value(e.payload.clone())
            .map_err(|e| ConsumerError::Request(format!("the DataSetMetaData: {e}")))?;
        encode(&MetaDataNetworkMessage {
            message_id: self.sender.message_id,
            publisher_id: self.sender.publisher.clone(),
            data_set_writer_id: e.data_set_writer_id,
            writer_group_name: self.group,
            data_set_writer_name: self.source.clone(),
            timestamp: date_time(self.sender.unix_ms),
            reply_to: None,
            meta_data: Some(meta),
        })
    }
}

fn encode<T: serde::Serialize>(m: &T) -> Result<Vec<u8>, ConsumerError> {
    serde_json::to_vec(m).map_err(|e| ConsumerError::Request(e.to_string()))
}
