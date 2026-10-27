//! The ReferenceDesignation of every source (MG 8.3.10, MG 9.1.10; P4
//! increment 7c, ADR-0026 §16): persisted (REQ-RES-REFDES-002), answered
//! on `Get` with and without Source and with the Query of the direct
//! children (REQ-RES-REFDES-004, REQ-RES-REFDES-011 … 014), replaced as a
//! whole by `Set` and deleted by `Del` after the application decided
//! (REQ-RES-REFDES-006, REQ-RES-REFDES-007), each answered with a Status
//! event (REQ-RES-REFDES-009); a successful `Set` publishes the new object
//! on `Pub/ReferenceDesignation/<Source>` with the ReplyTo
//! (REQ-RES-REFDES-010).

use serde::{Deserialize, Serialize};
use serde_json::{Map, Value as Json};

use super::write::WriteWire;
use super::{Provider, ProviderError, WriterKey};
use crate::il::IdentificationLink;
use crate::lifecycle::Publication;
use crate::message::{DataSetMessage, DataSetWriterId, DecodeError, MessageId};
use crate::resource::{
    Aspect, PublicationEntry, PublicationMode, ReferenceDesignation, ReferenceDesignationQuery,
};
use crate::status::StatusCode;
use crate::topic::{ListFilter, Method, Resource};

/// One persisted ReferenceDesignation.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
struct Stored {
    source: String,
    reference_designation: ReferenceDesignation,
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "PascalCase", deny_unknown_fields)]
struct Wire {
    version: u32,
    entries: Vec<Stored>,
}

/// The persisted ReferenceDesignations by Source (REQ-RES-REFDES-002);
/// persist it with [`StoredReferenceDesignations::to_json`] after every
/// change. An object stays stored while its asset is removed, so it is
/// there again when the asset comes back.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct StoredReferenceDesignations {
    entries: Vec<Stored>,
}

impl StoredReferenceDesignations {
    /// An empty table.
    pub fn new() -> Self {
        Self::default()
    }

    /// Reads a table written by [`Self::to_json`].
    ///
    /// # Errors
    ///
    /// Other JSON, an unknown version or a Source that occurs twice.
    pub fn from_json(json: &[u8]) -> Result<Self, String> {
        let wire: Wire =
            serde_json::from_slice(json).map_err(|e| DecodeError::from(e).to_string())?;
        if wire.version != 1 {
            return Err(format!("unknown version {}", wire.version));
        }
        let mut table = Self::new();
        for e in wire.entries {
            if table.get(&e.source).is_some() {
                return Err(format!("the Source {} occurs twice", e.source));
            }
            table.entries.push(e);
        }
        Ok(table)
    }

    /// The table as JSON (`{"Version":1,"Entries":[…]}`).
    ///
    /// # Panics
    ///
    /// Never: the table has no maps with non-string keys.
    pub fn to_json(&self) -> Vec<u8> {
        serde_json::to_vec(&Wire {
            version: 1,
            entries: self.entries.clone(),
        })
        .expect("the table always serializes")
    }

    /// The ReferenceDesignation of a Source, if set.
    pub fn get(&self, source: &str) -> Option<&ReferenceDesignation> {
        self.entries
            .iter()
            .find(|e| e.source == source)
            .map(|e| &e.reference_designation)
    }

    /// Replaces the ReferenceDesignation of a Source; returns whether the
    /// table changed.
    pub fn set(&mut self, source: &str, value: ReferenceDesignation) -> bool {
        match self.entries.iter_mut().find(|e| e.source == source) {
            Some(e) if e.reference_designation == value => false,
            Some(e) => {
                e.reference_designation = value;
                true
            }
            None => {
                self.entries.push(Stored {
                    source: source.to_owned(),
                    reference_designation: value,
                });
                true
            }
        }
    }

    /// Deletes the ReferenceDesignation of a Source; returns whether it
    /// had one.
    pub fn remove(&mut self, source: &str) -> bool {
        let before = self.entries.len();
        self.entries.retain(|e| e.source != source);
        self.entries.len() != before
    }

    /// Number of Sources with a ReferenceDesignation.
    pub fn len(&self) -> usize {
        self.entries.len()
    }

    /// No ReferenceDesignation.
    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }
}

/// A `Set` (with the new object) or `Del` (without) of the
/// ReferenceDesignation of a source, checked by the provider: the
/// application decides, then [`Provider::complete_reference_designation`]
/// answers.
#[derive(Debug, Clone, PartialEq)]
pub struct ReferenceDesignationRequest {
    source: IdentificationLink,
    value: Option<ReferenceDesignation>,
    message_id: MessageId,
}

impl ReferenceDesignationRequest {
    /// The Source whose ReferenceDesignation is set or deleted.
    pub fn source(&self) -> &IdentificationLink {
        &self.source
    }

    /// The new object of a `Set`, which replaces the old one as a whole
    /// (REQ-RES-REFDES-006); `None` for a `Del`.
    pub fn reference_designation(&self) -> Option<&ReferenceDesignation> {
        self.value.as_ref()
    }

    /// The MessageId of the request: the ReplyTo of the answers.
    pub fn message_id(&self) -> &MessageId {
        &self.message_id
    }
}

/// Whether `rd` is a direct child of `query.parent`: the Parent of the
/// requested aspect, without one of any aspect, has an IdentificationLink
/// equal to it, compared character by character (REQ-RES-REFDES-014,
/// REQ-IL-015).
fn matches(rd: &ReferenceDesignation, query: &ReferenceDesignationQuery) -> bool {
    let aspects = match query.aspect {
        Some(Aspect::Function) => [rd.function.as_ref(), None, None],
        Some(Aspect::Product) => [rd.product.as_ref(), None, None],
        Some(Aspect::Location) => [rd.location.as_ref(), None, None],
        None => [
            rd.function.as_ref(),
            rd.product.as_ref(),
            rd.location.as_ref(),
        ],
    };
    aspects.into_iter().flatten().any(|d| {
        d.parent
            .as_ref()
            .and_then(|p| p.identification_link.as_deref())
            == Some(query.parent.as_str())
    })
}

impl Provider {
    /// The provider offering the resource ReferenceDesignation
    /// (REQ-CMP-020) with the persisted objects (empty at the first start).
    /// From Level 2 on ([`super::ProviderConfig::get`]) every source gets
    /// its DataSetWriterId and the resource appears in the Profiles and
    /// the PublicationList; at Level 1 no request is served.
    ///
    /// # Errors
    ///
    /// [`ProviderError::WriterId`] if no DataSetWriterId can be assigned.
    pub fn with_reference_designations(
        mut self,
        table: StoredReferenceDesignations,
    ) -> Result<Self, ProviderError> {
        self.refdes = Some(table);
        if self.config.get {
            for il in self.sources(None) {
                self.assign(WriterKey::new(Resource::ReferenceDesignation, il.as_str()))?;
            }
        }
        Ok(self)
    }

    /// Whether the resource ReferenceDesignation is offered.
    pub fn offers_reference_designation(&self) -> bool {
        self.refdes.is_some()
    }

    /// The ReferenceDesignation of the application or of an asset, if
    /// offered and set.
    pub fn reference_designation(&self, il: &IdentificationLink) -> Option<&ReferenceDesignation> {
        self.refdes.as_ref()?.get(il.as_str())
    }

    /// The ReferenceDesignations as JSON if they changed since the last
    /// call; persist them (REQ-RES-REFDES-002).
    pub fn take_reference_designations(&mut self) -> Option<Vec<u8>> {
        let table = self.refdes.as_ref()?;
        core::mem::take(&mut self.unsaved.refdes).then(|| table.to_json())
    }

    /// Answers a `Set` or `Del` of a ReferenceDesignation the application
    /// decided: on `Ok` a `Set` replaces the object and publishes it on
    /// `Pub/ReferenceDesignation/<Source>` with the ReplyTo of the request
    /// (REQ-RES-REFDES-010), a `Del` deletes it and publishes nothing on
    /// the resource; then the Status event Good (REQ-RES-REFDES-009). On
    /// `Err` only the Status event with that code, an `Err` with a Good
    /// code as `Bad`. If the source was removed meanwhile, or a `Del` finds
    /// no object any more, the answer is `BadNotFound`.
    pub fn complete_reference_designation(
        &mut self,
        request: &ReferenceDesignationRequest,
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
            match self.apply_reference_designation(request, unix_ms) {
                Ok(p) => out = p,
                Err(c) => code = c,
            }
        }
        out.extend(self.answer(&request.source, code, &request.message_id, unix_ms));
        out
    }

    fn apply_reference_designation(
        &mut self,
        r: &ReferenceDesignationRequest,
        unix_ms: u64,
    ) -> Result<Vec<Publication>, StatusCode> {
        if !self.is_source(&r.source) {
            return Err(StatusCode::BAD_NOT_FOUND);
        }
        let table = self.refdes.as_mut().ok_or(StatusCode::BAD_NOT_FOUND)?;
        let Some(value) = &r.value else {
            if !table.remove(r.source.as_str()) {
                return Err(StatusCode::BAD_NOT_FOUND);
            }
            self.unsaved.refdes = true;
            return Ok(Vec::new());
        };
        self.unsaved.refdes |= table.set(r.source.as_str(), value.clone());
        let topic = self
            .topic(Resource::ReferenceDesignation, Some(&r.source), None)
            .map_err(|_| StatusCode::BAD_INTERNAL_ERROR)?;
        let items = self.refdes_items(Some(&r.source), None);
        let limit = self.config.max_packet_size;
        Ok(self
            .series(items, None, Some(&r.message_id), limit, unix_ms)
            .into_iter()
            .map(|payload| Publication {
                topic: topic.clone(),
                payload,
            })
            .collect())
    }

    /// Checks a `Set` or `Del` of a ReferenceDesignation: a `Set` carries
    /// exactly one DataSetMessage with the object (`BadInvalidArgument`
    /// otherwise, `BadDecodingError` for no object, REQ-RES-REFDES-006;
    /// unknown fields are ignored, REQ-MSG-046); a `Del` needs an object to
    /// delete (`BadNotFound`), its DataSetMessages are ignored.
    pub(super) fn reference_designation_request(
        &self,
        method: Method,
        source: &IdentificationLink,
        wire: &WriteWire,
    ) -> Result<ReferenceDesignationRequest, StatusCode> {
        let value = if method == Method::Set {
            let Some(Json::Object(payload)) = wire.single()?.get("Payload") else {
                return Err(StatusCode::BAD_DECODING_ERROR);
            };
            Some(
                serde_json::from_value(Json::Object(payload.clone()))
                    .map_err(|_| StatusCode::BAD_DECODING_ERROR)?,
            )
        } else {
            self.reference_designation(source)
                .ok_or(StatusCode::BAD_NOT_FOUND)?;
            None
        };
        Ok(ReferenceDesignationRequest {
            source: source.clone(),
            value,
            message_id: wire.message_id.clone(),
        })
    }

    /// The ReferenceDesignations of the sources addressed that have one, in
    /// the order of sources; with a Query only the direct children
    /// (REQ-RES-REFDES-014).
    pub(super) fn refdes_items(
        &self,
        source: Option<&IdentificationLink>,
        query: Option<&ReferenceDesignationQuery>,
    ) -> Vec<DataSetMessage<ReferenceDesignation>> {
        self.sources(source)
            .into_iter()
            .filter_map(|il| {
                let rd = self.reference_designation(&il)?;
                if query.is_some_and(|q| !matches(rd, q)) {
                    return None;
                }
                let id = self.writer(Resource::ReferenceDesignation, &il);
                Some(DataSetMessage::new(id, il, "", rd.clone()))
            })
            .collect()
    }

    /// The PublicationList entry of the ReferenceDesignation of `il`
    /// (ON_REQUEST_SOURCE_10, ADR-0020 §7), if offered.
    pub(super) fn refdes_list_item(
        &self,
        il: &IdentificationLink,
        filter: Option<&ListFilter>,
        list: DataSetWriterId,
    ) -> Option<DataSetMessage<PublicationEntry>> {
        let r = Resource::ReferenceDesignation;
        if self.refdes.is_none()
            || filter.is_some_and(|f| f.resource() != r || f.filter().is_some())
        {
            return None;
        }
        let entry = PublicationEntry {
            resource: r,
            source: il.as_str().to_owned(),
            filter: None,
            data_set_writer_id: self.writer(r, il),
            mode: Some(PublicationMode::OnRequestSource),
            interval: None,
            precisions: None,
            config: None,
        };
        Some(DataSetMessage::new(list, il.clone(), r.as_str(), entry))
    }
}

/// The Query of a `Get`: the Payload of its DataSetMessage with
/// DataSetWriterId 9 (REQ-QRY-001), if any.
pub(super) fn query_of(messages: &[Json]) -> Option<&Map<String, Json>> {
    messages
        .iter()
        .find(|m| m.get("DataSetWriterId").and_then(Json::as_u64) == Some(9))
        .and_then(|m| m.get("Payload"))
        .and_then(Json::as_object)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn rd(parent: &str) -> ReferenceDesignation {
        serde_json::from_value(serde_json::json!({
            "Function": {"Value": "=A1", "Parent": {"Value": "=A", "IdentificationLink": parent}}
        }))
        .unwrap()
    }

    #[test]
    fn round_trip_set_and_remove() {
        let mut t = StoredReferenceDesignations::new();
        assert!(t.set("a.com/M/P/S", rd("x.com/L/C/1")));
        assert!(!t.set("a.com/M/P/S", rd("x.com/L/C/1")));
        assert!(t.set("a.com/M/P/S", rd("x.com/L/C/2")));
        assert!(t.set("b.com/M/P/S", ReferenceDesignation::default()));
        let back = StoredReferenceDesignations::from_json(&t.to_json()).unwrap();
        assert_eq!(back, t);
        assert_eq!(back.len(), 2);
        assert!(t.remove("b.com/M/P/S"));
        assert!(!t.remove("b.com/M/P/S"));
        assert!(t.get("b.com/M/P/S").is_none());
        assert!(!t.is_empty());
    }

    #[test]
    fn rejects_invalid_tables() {
        let mut once = StoredReferenceDesignations::new();
        once.set("a.com/M/P/S", rd("x.com/L/C/1"));
        let twice = StoredReferenceDesignations {
            entries: [once.entries.clone(), once.entries].concat(),
        };
        for bad in [
            b"{}".to_vec(),
            br#"{"Version":2,"Entries":[]}"#.to_vec(),
            br#"{"Version":1,"Entries":[],"X":1}"#.to_vec(),
            twice.to_json(),
        ] {
            assert!(StoredReferenceDesignations::from_json(&bad).is_err());
        }
    }

    #[test]
    fn query_matches_direct_children_of_the_aspect() {
        let q = |aspect| ReferenceDesignationQuery {
            parent: "x.com/L/C/1".to_owned(),
            aspect,
        };
        let child = rd("x.com/L/C/1");
        assert!(matches(&child, &q(None)));
        assert!(matches(&child, &q(Some(Aspect::Function))));
        assert!(!matches(&child, &q(Some(Aspect::Location))));
        assert!(!matches(&rd("x.com/L/C/10"), &q(None)));
        assert!(!matches(&ReferenceDesignation::default(), &q(None)));
    }
}
