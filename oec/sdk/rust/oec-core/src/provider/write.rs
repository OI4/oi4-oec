//! The write side of the provider (P4 increment 7a, ADR-0026 §14): `Set`
//! and `Del` to the application, each answered with a Status event
//! (REQ-RES-EVENT-005, REQ-CMP-043); `Set PublicationList` of the
//! configurable Data entries (REQ-RES-PUBLIST-016, REQ-RES-PUBLIST-022,
//! REQ-RES-PUBLIST-024, REQ-RES-PUBLIST-025); `Set Data` of the DataSets
//! the application accepts, decided by the application outside the
//! provider (REQ-RES-DATA-009, REQ-RES-DATA-011, REQ-RES-DATA-012); and
//! the Status event `BadNotFound` for a `Get` to an unknown Source
//! (REQ-CMP-047).

use std::collections::BTreeMap;

use serde::Deserialize;
use serde_json::{Map, Value as Json};

use super::data::{self, Mode, Value as FieldValue};
use super::overrides::PublicationOverride;
use super::sets::date_time;
use super::{Handled, Ignored, Provider, WriterKey};
use crate::il::IdentificationLink;
use crate::lifecycle::Publication;
use crate::message::{ConfigurationVersion, DataSetMessage, MessageId};
use crate::resource::{Event, EventCategory, PublicationConfig, PublicationEntry, PublicationMode};
use crate::status::StatusCode;
use crate::topic::{
    EventFilter, Filter, ListFilter, Method, Resource, StatusLevel, Subject, Topic,
};

/// At most this many Sources that are no asset of the application get a
/// transient DataSetWriterId for the Status event `BadNotFound`; a request
/// to a further unknown Source is not answered (ADR-0026 §14).
pub const FOREIGN_SOURCES: usize = 64;

/// The Filter `Status/<Level>` of the Status events.
pub(super) fn status_filter(level: StatusLevel) -> Filter {
    Filter::Event(EventFilter::Status(level))
}

/// The key of the Status events of `source` at `level`.
pub(super) fn status_key(source: &str, level: StatusLevel) -> WriterKey {
    WriterKey {
        resource: Resource::Event,
        source: source.to_owned(),
        filter: status_filter(level).to_string(),
    }
}

/// A `Set Data` of a DataSet the application accepts (see
/// [`Provider::accept_set_data`]), checked against the field definitions:
/// the application decides, then [`Provider::complete_set_data`] answers.
#[derive(Debug, Clone, PartialEq)]
pub struct SetDataRequest {
    source: IdentificationLink,
    tag: String,
    values: Vec<(String, FieldValue)>,
    message_id: MessageId,
    version: ConfigurationVersion,
}

impl SetDataRequest {
    /// The Source of the DataSet.
    pub fn source(&self) -> &IdentificationLink {
        &self.source
    }

    /// The Tag of the DataSet.
    pub fn tag(&self) -> &str {
        &self.tag
    }

    /// The values of all fields in definition order, converted to the
    /// field types.
    pub fn values(&self) -> &[(String, FieldValue)] {
        &self.values
    }

    /// The MessageId of the `Set`: the ReplyTo of the answers.
    pub fn message_id(&self) -> &MessageId {
        &self.message_id
    }
}

/// The parts of a `Set` or `Del` NetworkMessage the provider uses.
struct WriteWire {
    message_id: MessageId,
    messages: Option<Json>,
}

impl WriteWire {
    fn parse(payload: &[u8]) -> Result<Self, Ignored> {
        #[derive(Deserialize)]
        #[serde(rename_all = "PascalCase")]
        struct Wire {
            message_id: MessageId,
            #[serde(default)]
            messages: Option<Json>,
        }
        let w: Wire =
            serde_json::from_slice(payload).map_err(|e| Ignored::Payload(e.to_string()))?;
        Ok(Self {
            message_id: w.message_id,
            messages: w.messages,
        })
    }

    /// The one DataSetMessage of the resource; helper DataSets
    /// (DataSetWriterId 1 … 9, REQ-MSG-019) aside.
    fn single(&self) -> Result<&Map<String, Json>, StatusCode> {
        let Some(Json::Array(all)) = &self.messages else {
            return Err(StatusCode::BAD_DECODING_ERROR);
        };
        let mut resource = all.iter().filter(|m| {
            !m.get("DataSetWriterId")
                .and_then(Json::as_u64)
                .is_some_and(|id| (1..10).contains(&id))
        });
        match (resource.next(), resource.next()) {
            (Some(Json::Object(m)), None) => Ok(m),
            (Some(_), None) => Err(StatusCode::BAD_DECODING_ERROR),
            _ => Err(StatusCode::BAD_INVALID_ARGUMENT),
        }
    }
}

/// The settings of a PublicationList entry a `Set` can change.
#[derive(Debug, Clone, PartialEq)]
struct Setting {
    mode: Mode,
    interval: u32,
    /// Only the Precisions > 0 (0 is the default).
    precisions: BTreeMap<String, f64>,
}

impl Setting {
    fn of(e: &PublicationEntry) -> Self {
        let precisions = e
            .precisions
            .iter()
            .flatten()
            .map(|(k, v)| (k.clone(), v.as_f64().unwrap_or(f64::NAN)))
            .filter(|(_, v)| *v != 0.0)
            .collect();
        Self {
            mode: Mode::from(e.mode.unwrap_or(PublicationMode::OnRequest)),
            interval: e.interval.unwrap_or(0),
            precisions,
        }
    }
}

/// The new setting of a `Set PublicationList`, `None` if it changes
/// nothing; `BadNotWritable` for a change of a read-only field or of a
/// setting its `Config` does not allow (REQ-RES-PUBLIST-016).
fn change(
    current: &PublicationEntry,
    requested: &PublicationEntry,
) -> Result<Option<Setting>, StatusCode> {
    let config = |e: &PublicationEntry| e.config.unwrap_or(PublicationConfig::None);
    if requested.resource != current.resource
        || requested.source != current.source
        || requested.filter != current.filter
        || requested.data_set_writer_id != current.data_set_writer_id
        || config(requested) != config(current)
    {
        return Err(StatusCode::BAD_NOT_WRITABLE);
    }
    let (old, new) = (Setting::of(current), Setting::of(requested));
    let mode = old.mode.to_publication_mode() != new.mode.to_publication_mode();
    let interval = old.interval != new.interval || old.precisions != new.precisions;
    if !mode && !interval {
        return Ok(None);
    }
    let (may_mode, may_interval) = match config(current) {
        PublicationConfig::None => (false, false),
        PublicationConfig::Mode => (true, false),
        PublicationConfig::Interval => (false, true),
        PublicationConfig::ModeAndInterval => (true, true),
    };
    if (mode && !may_mode) || (interval && !may_interval) {
        return Err(StatusCode::BAD_NOT_WRITABLE);
    }
    Ok(Some(new))
}

impl Provider {
    /// Whether `il` is the application or one of its assets.
    pub(super) fn is_source(&self, il: &IdentificationLink) -> bool {
        il == self.app_id() || self.find(il).is_some()
    }

    /// `spec` with the persisted setting of `Set PublicationList` for the
    /// DataSet, as far as its `Config` allows and the result is valid.
    pub(super) fn overridden(
        &self,
        source: &IdentificationLink,
        filter: &str,
        mut spec: data::DataSetSpec,
    ) -> data::DataSetSpec {
        let Some(o) = self.overrides.get(source.as_str(), filter) else {
            return spec;
        };
        let original = spec.clone();
        let w = spec.publication.writable;
        if w.mode() {
            spec.publication.mode = Mode::from(o.mode);
        }
        if w.interval() {
            spec.publication.interval_ms = o.interval;
            spec.publication.precisions.clone_from(&o.precisions);
        }
        if spec.check().is_ok() { spec } else { original }
    }

    /// Accepts `Set Data` for a DataSet (a handler decides) or not (the
    /// default: `BadNotWritable`, REQ-CMP-043).
    ///
    /// # Errors
    ///
    /// [`super::ProviderError::UnknownDataSet`].
    pub fn accept_set_data(
        &mut self,
        source: &IdentificationLink,
        tag: &str,
        accept: bool,
    ) -> Result<(), super::ProviderError> {
        let i = self.set_index(source, tag)?;
        self.sets[i].settable = accept;
        Ok(())
    }

    /// The Status event about a request to `source` on
    /// `Pub/Event/<Source>/Status/<Level>` (REQ-RES-EVENT-003 … 005): the
    /// StatusCode with Description and SymbolicId, if known, and the
    /// ReplyTo. `None` if `source` is no asset and the bound of
    /// [`FOREIGN_SOURCES`] is reached.
    pub(super) fn status_event(
        &mut self,
        source: &IdentificationLink,
        code: StatusCode,
        reply_to: Option<&MessageId>,
        unix_ms: u64,
    ) -> Option<Publication> {
        let level = code.level();
        let key = status_key(source.as_str(), level);
        let id = match self.writers.get(&key) {
            Some(id) => id,
            None => self.writers.assign_transient(key, FOREIGN_SOURCES)?,
        };
        let details = code.symbolic_id().map(|s| {
            let mut m = Map::new();
            m.insert("SymbolicId".to_owned(), Json::String(s.to_owned()));
            m
        });
        let event = Event {
            number: code.0,
            category: EventCategory::Status,
            description: code.description().map(str::to_owned),
            details,
        };
        let filter = status_filter(level);
        let item = DataSetMessage::new(id, source.clone(), filter.to_string(), event)
            .with_timestamp(date_time(unix_ms));
        let topic = self
            .topic(Resource::Event, Some(source), Some(filter))
            .ok()?;
        let limit = self.config.max_packet_size;
        let payload = self
            .series(vec![item], None, reply_to, limit, unix_ms)
            .into_iter()
            .next()?;
        Some(Publication { topic, payload })
    }

    /// The answer to a request to a Source that is no asset: the Status
    /// event `BadNotFound` (REQ-CMP-043, REQ-CMP-047).
    pub(super) fn unknown_source(
        &mut self,
        source: &IdentificationLink,
        message_id: &MessageId,
        unix_ms: u64,
    ) -> Result<Handled, Ignored> {
        self.status_event(source, StatusCode::BAD_NOT_FOUND, Some(message_id), unix_ms)
            .map(|p| Handled::Publish(vec![p]))
            .ok_or_else(|| Ignored::UnknownSource(source.clone()))
    }

    /// A `Set` or `Del` to this application (from Level 2): every request
    /// with a valid MessageId gets a Status event.
    pub(super) fn handle_write(
        &mut self,
        topic: &Topic,
        payload: &[u8],
        unix_ms: u64,
    ) -> Result<Handled, Ignored> {
        let resource = match (topic.subject(), topic.source()) {
            (Subject::Resource(r), Some(_)) if self.config.get => r,
            (s, _) => return Err(Ignored::NotServed(s)),
        };
        let source = topic.source().cloned().expect("checked above");
        let wire = WriteWire::parse(payload)?;
        if !self.is_source(&source) {
            return self.unknown_source(&source, &wire.message_id, unix_ms);
        }
        let result = match (topic.method(), resource) {
            (Method::Set, Resource::Data) => match self.set_data(topic, &source, &wire) {
                Ok(request) => return Ok(Handled::SetData(request)),
                Err(code) => Err(code),
            },
            (Method::Set, Resource::PublicationList) => {
                self.set_publication_list(topic, &source, &wire, unix_ms)
            }
            (Method::Set, Resource::Metadata) => Err(self.set_meta_data(topic, &source)),
            _ => Err(StatusCode::BAD_NOT_FOUND),
        };
        let (mut out, code) = match result {
            Ok(out) => (out, StatusCode::GOOD),
            Err(code) => (Vec::new(), code),
        };
        out.extend(self.status_event(&source, code, Some(&wire.message_id), unix_ms));
        Ok(Handled::Publish(out))
    }

    /// The Metadata follows from the fields: `BadNotWritable` for an
    /// available DataSet, else `BadNotFound`.
    fn set_meta_data(&self, topic: &Topic, source: &IdentificationLink) -> StatusCode {
        match topic.filter() {
            Some(Filter::Tag(tag))
                if self
                    .find_set(source, tag)
                    .is_some_and(|i| self.sets[i].mode().is_on()) =>
            {
                StatusCode::BAD_NOT_WRITABLE
            }
            _ => StatusCode::BAD_NOT_FOUND,
        }
    }

    /// Checks a `Set Data` (REQ-RES-DATA-009): `BadNotFound` for a DataSet
    /// the source does not provide (or OFF_0), `BadNotWritable` if the
    /// application does not accept it, `BadInvalidArgument` unless exactly
    /// one DataSetMessage with every field, `BadTypeMismatch` for a value
    /// that does not fit its field. Unknown fields are ignored (tolerant
    /// reader, REQ-MSG-046).
    fn set_data(
        &self,
        topic: &Topic,
        source: &IdentificationLink,
        wire: &WriteWire,
    ) -> Result<SetDataRequest, StatusCode> {
        let Some(Filter::Tag(tag)) = topic.filter() else {
            return Err(StatusCode::BAD_NOT_FOUND);
        };
        let i = self
            .find_set(source, tag)
            .filter(|i| self.sets[*i].mode().is_on())
            .ok_or(StatusCode::BAD_NOT_FOUND)?;
        let s = &self.sets[i];
        if !s.settable {
            return Err(StatusCode::BAD_NOT_WRITABLE);
        }
        let Some(Json::Object(payload)) = wire.single()?.get("Payload") else {
            return Err(StatusCode::BAD_DECODING_ERROR);
        };
        let mut values = Vec::with_capacity(s.spec.fields.len());
        for f in &s.spec.fields {
            let json = payload
                .get(&f.name)
                .ok_or(StatusCode::BAD_INVALID_ARGUMENT)?;
            let v = data::decode(f.ty, json).map_err(|_| StatusCode::BAD_TYPE_MISMATCH)?;
            values.push((f.name.clone(), v));
        }
        Ok(SetDataRequest {
            source: source.clone(),
            tag: tag.clone(),
            values,
            message_id: wire.message_id.clone(),
            version: s.version(),
        })
    }

    /// Answers a `Set Data` the application decided: on `Ok` the values
    /// are applied and the DataSet is published on
    /// `Pub/Data/<Source>/<Tag>` with the ReplyTo of the `Set`
    /// (REQ-RES-DATA-012), then the Status event Good; on `Err` the Status
    /// event with that code (REQ-RES-DATA-011), an `Err` with a Good code
    /// as `Bad`. If the DataSet was removed or redefined meanwhile, the
    /// answer is `BadNotFound`.
    pub fn complete_set_data(
        &mut self,
        request: &SetDataRequest,
        result: Result<(), StatusCode>,
        unix_ms: u64,
    ) -> Vec<Publication> {
        let mut code = match result {
            Ok(()) => StatusCode::GOOD,
            Err(c) if c.is_good() => StatusCode::BAD,
            Err(c) => c,
        };
        let mut out = Vec::new();
        if code.is_good() {
            match self.apply_set_data(request, unix_ms) {
                Ok(p) => out = p,
                Err(c) => code = c,
            }
        }
        out.extend(self.status_event(&request.source, code, Some(&request.message_id), unix_ms));
        out
    }

    fn apply_set_data(
        &mut self,
        r: &SetDataRequest,
        unix_ms: u64,
    ) -> Result<Vec<Publication>, StatusCode> {
        let i = self
            .find_set(&r.source, &r.tag)
            .filter(|i| self.sets[*i].mode().is_on() && self.sets[*i].version() == r.version)
            .ok_or(StatusCode::BAD_NOT_FOUND)?;
        self.sets[i]
            .update(r.values.clone(), unix_ms)
            .map_err(|_| StatusCode::BAD_TYPE_MISMATCH)?;
        let topic = self
            .topic(
                Resource::Data,
                Some(&r.source),
                Some(Filter::Tag(r.tag.clone())),
            )
            .map_err(|_| StatusCode::BAD_INTERNAL_ERROR)?;
        let s = &self.sets[i];
        let (items, classes) = (vec![s.message()], [s.class_id]);
        let limit = self.config.max_packet_size;
        let payloads = self.series_of(items, &classes, None, Some(&r.message_id), limit, unix_ms);
        self.sets[i].answered();
        Ok(payloads
            .into_iter()
            .map(|payload| Publication {
                topic: topic.clone(),
                payload,
            })
            .collect())
    }

    /// `Set PublicationList` of one entry (REQ-RES-PUBLIST-016,
    /// REQ-RES-PUBLIST-022): `BadNotFound` for an entry the source does
    /// not have, `BadInvalidArgument` unless exactly one DataSetMessage or
    /// for Precisions of unknown or non-numeric fields, `BadDecodingError`
    /// for no PublicationList entry, `BadNotWritable` for a change its
    /// Config does not allow. The new setting is persisted and the entry
    /// published on `Pub/PublicationList/<Source>/<Filter>` with the
    /// ReplyTo (REQ-RES-PUBLIST-018, REQ-RES-PUBLIST-025); an unchanged
    /// entry is a success, too.
    fn set_publication_list(
        &mut self,
        topic: &Topic,
        source: &IdentificationLink,
        wire: &WriteWire,
        unix_ms: u64,
    ) -> Result<Vec<Publication>, StatusCode> {
        let Some(Filter::List(lf)) = topic.filter() else {
            return Err(StatusCode::BAD_NOT_FOUND);
        };
        let current = self
            .exact_entries(source, lf)
            .first()
            .map(|m| m.payload().clone())
            .ok_or(StatusCode::BAD_NOT_FOUND)?;
        let payload = wire
            .single()?
            .get("Payload")
            .ok_or(StatusCode::BAD_DECODING_ERROR)?;
        let requested: PublicationEntry =
            serde_json::from_value(payload.clone()).map_err(|_| StatusCode::BAD_DECODING_ERROR)?;
        if let Some(setting) = change(&current, &requested)? {
            self.reconfigure(source, lf, setting)?;
        }
        Ok(self.publish_entry(source, lf, &wire.message_id, unix_ms))
    }

    /// Applies a new setting to a Data DataSet and persists it.
    fn reconfigure(
        &mut self,
        source: &IdentificationLink,
        lf: &ListFilter,
        setting: Setting,
    ) -> Result<(), StatusCode> {
        let (Resource::Data, Some(Filter::Tag(tag))) = (lf.resource(), lf.filter()) else {
            return Err(StatusCode::BAD_NOT_WRITABLE);
        };
        let i = self
            .find_set(source, tag)
            .ok_or(StatusCode::BAD_NOT_FOUND)?;
        let mut spec = self.sets[i].spec.clone();
        spec.publication.mode = setting.mode;
        spec.publication.interval_ms = setting.interval;
        spec.publication.precisions.clone_from(&setting.precisions);
        spec.check().map_err(|_| StatusCode::BAD_INVALID_ARGUMENT)?;
        let toggled = self.sets[i].reconfigure(spec.publication);
        self.pending.list |= toggled;
        let entry = PublicationOverride {
            source: source.as_str().to_owned(),
            filter: self.sets[i].filter.clone(),
            mode: setting.mode.to_publication_mode(),
            interval: setting.interval,
            precisions: setting.precisions,
        };
        self.unsaved.overrides |= self.overrides.set(entry);
        Ok(())
    }

    /// The PublicationList entries `<Resource>[/<Filter>]` of `source`
    /// exactly (not all Data entries for `Data`).
    fn exact_entries(
        &self,
        source: &IdentificationLink,
        lf: &ListFilter,
    ) -> Vec<DataSetMessage<PublicationEntry>> {
        let want = lf.filter().map(ToString::to_string);
        self.list_items(Some(source), Some(lf))
            .into_iter()
            .filter(|m| m.payload().filter == want)
            .collect()
    }

    fn publish_entry(
        &mut self,
        source: &IdentificationLink,
        lf: &ListFilter,
        reply_to: &MessageId,
        unix_ms: u64,
    ) -> Vec<Publication> {
        let items = self.exact_entries(source, lf);
        let Ok(topic) = self.topic(
            Resource::PublicationList,
            Some(source),
            Some(Filter::List(lf.clone())),
        ) else {
            return Vec::new();
        };
        let limit = self.config.max_packet_size;
        self.series(items, None, Some(reply_to), limit, unix_ms)
            .into_iter()
            .map(|payload| Publication {
                topic: topic.clone(),
                payload,
            })
            .collect()
    }
}
