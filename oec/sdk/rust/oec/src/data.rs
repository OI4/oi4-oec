//! Data DataSets of an asset with their Metadata and PublicationList
//! entries (MG 8.3.4, 8.3.5, 8.3.8; P4 increment 6b, ADR-0020 §6/§7 and
//! amendment C-054, ADR-0026 §12).
//!
//! A DataSet is declared with [`crate::asset::Asset::add_data`] and its
//! state set with [`DataSet::update`]. The SDK publishes the state, not
//! every call (ADR-0020 §7): what is sent and when follows the
//! PublicationList entry – its Mode (REQ-RES-PUBLIST-007 … 011), its
//! Interval (cyclic, REQ-RES-PUBLIST-013/014) or, at Interval 0, every
//! change beyond the Precisions (REQ-MQTT-019, REQ-RES-PUBLIST-015). The
//! last state answers `Get Data` (REQ-RES-DATA-007); offline nothing is
//! queued and the current state is published after the next connect.
//!
//! A DataSet with a handler ([`DataSet::on_set`]) accepts `Set Data`: the
//! SDK checks the request against the field definitions, the handler
//! decides, the SDK applies the values and answers (REQ-RES-DATA-011/012).
//! The settings a `Set PublicationList` changes within
//! [`Publication::writable`] are persisted and survive a restart
//! (REQ-RES-PUBLIST-016, P4 increment 7a).

use std::fmt;
use std::time::Duration;

use oec_core::provider::data::{DataSetSpec, FieldSpec, OI4PV, PublicationSpec};
use uuid::Uuid;

use crate::app::AppHandle;
use crate::identity::IdentificationLink;
use crate::status::StatusCode;
use crate::{Error, Result};

/// OPC UA built-in types of DataSet fields (REQ-MSG-037); the SDK derives
/// the `FieldMetaData` of the Metadata from it (REQ-RES-META-003).
pub use oec_core::provider::data::BuiltinType;
/// Publication `Mode` of a PublicationList entry (MG 8.3.8): the forms
/// `ON_REQUEST`, `APPLICATION`, `SOURCE` and `FILTER` combine with `|`;
/// every form is combined with `ON_REQUEST` (REQ-RES-PUBLIST-026, SF-101).
/// `Get` is answered for every Mode except `OFF` (REQ-RES-PUBLIST-027).
pub use oec_core::provider::data::Mode;
/// A field value; `From` conversions exist for the Rust primitives,
/// `String`/`&str`, `Vec<u8>`/`&[u8]` and
/// [`crate::identity::LocalizedText`]. It is converted to the type of its
/// field when set (e.g. an integer to every numeric type whose range holds
/// it, a string to a Guid).
pub use oec_core::provider::data::Value;
/// What a `Set PublicationList` may change (`Config`,
/// REQ-RES-PUBLIST-016); every other change is answered with
/// `BadNotWritable`.
pub use oec_core::provider::data::Writable;

/// Definition of one DataSet field; it becomes a `FieldMetaData` of the
/// Metadata (REQ-RES-META-003).
#[derive(Debug, Clone)]
pub struct Field(pub(crate) FieldSpec);

impl Field {
    /// A field; the names are unique within the DataSet (REQ-MSG-038).
    pub fn new(name: &str, ty: BuiltinType) -> Self {
        Self(FieldSpec::new(name, ty))
    }

    /// A `Double` field.
    pub fn double(name: &str) -> Self {
        Self::new(name, BuiltinType::Double)
    }

    /// The display name of the engineering unit (property
    /// `EngineeringUnits`, UnitId -1); numeric fields only.
    #[must_use]
    pub fn unit(mut self, unit: &str) -> Self {
        self.0.unit = Some(unit.to_owned());
        self
    }

    /// The `Description` (`en-US`), e.g. the measuring point of `Pv`
    /// (REQ-RES-DATA-005).
    #[must_use]
    pub fn description(mut self, text: &str) -> Self {
        self.0.description = Some(text.to_owned());
        self
    }

    /// The range of a numeric value (property `EURange`).
    #[must_use]
    pub fn range(mut self, low: f64, high: f64) -> Self {
        self.0.range = Some((low, high));
        self
    }
}

/// The initial PublicationList entry of a DataSet (MG 8.3.8). The SDK
/// applies it to every update (REQ-RES-PUBLIST-007 … 015) and publishes the
/// PublicationList again on every change (REQ-RES-PUBLIST-017).
#[derive(Debug, Clone)]
pub struct Publication {
    spec: PublicationSpec,
    interval: Duration,
}

impl Default for Publication {
    fn default() -> Self {
        Self::default_data()
    }
}

impl Publication {
    /// The SDK default for Data: `Mode::ON_REQUEST | Mode::FILTER`
    /// (`ON_REQUEST_FILTER_9`), Interval 0 (publish on change,
    /// REQ-MQTT-019, REQ-RES-PUBLIST-013), no Precisions,
    /// `Writable::Nothing`.
    pub fn default_data() -> Self {
        Self {
            spec: PublicationSpec::default(),
            interval: Duration::ZERO,
        }
    }

    /// The Mode.
    #[must_use]
    pub fn mode(mut self, mode: Mode) -> Self {
        self.spec.mode = mode;
        self
    }

    /// Cyclic publication in whole milliseconds, whether the values changed
    /// or not; it overrides the Precisions (REQ-RES-PUBLIST-013/014).
    /// `Duration::ZERO` publishes on change.
    #[must_use]
    pub fn interval(mut self, interval: Duration) -> Self {
        self.interval = interval;
        self
    }

    /// The dead band of a numeric field at Interval 0: a change is
    /// published once it exceeds the precision against the last published
    /// value (REQ-RES-PUBLIST-015).
    #[must_use]
    pub fn precision(mut self, field: &str, precision: f64) -> Self {
        self.spec.precisions.insert(field.to_owned(), precision);
        self
    }

    /// What a `Set PublicationList` may change (`Config`). A change is
    /// persisted and replaces the settings of this definition after a
    /// restart, as far as `writable` still allows it.
    #[must_use]
    pub fn writable(mut self, writable: Writable) -> Self {
        self.spec.writable = writable;
        self
    }

    fn resolve(mut self) -> Result<PublicationSpec> {
        let ms = self.interval.as_millis();
        if self.interval.subsec_nanos() % 1_000_000 != 0 {
            return Err(Error::Invalid(format!(
                "the Interval {:?} is no whole number of milliseconds",
                self.interval
            )));
        }
        self.spec.interval_ms = u32::try_from(ms).map_err(|_| {
            Error::Invalid(format!(
                "the Interval {:?} exceeds {} ms (UInt32)",
                self.interval,
                u32::MAX
            ))
        })?;
        Ok(self.spec)
    }
}

/// Definition of a Data DataSet.
#[derive(Debug, Clone)]
pub struct DataSetDef {
    spec: DataSetSpec,
    class_id: Option<String>,
    publication: Publication,
}

impl DataSetDef {
    /// `tag` is the topic Filter (Tag) of the DataSet and its
    /// WriterGroupName (REQ-TOP-019 … 022, REQ-MSG-024).
    pub fn new(tag: &str) -> Self {
        Self {
            spec: DataSetSpec::new(tag),
            class_id: None,
            publication: Publication::default_data(),
        }
    }

    /// The standardized process value DataSet `Oi4Pv`: `pv` becomes the
    /// field `Pv`, further fields are `Sv1` … `Sv255`
    /// (REQ-RES-DATA-002 … 005).
    pub fn oi4pv(pv: Field) -> Self {
        let mut field = pv.0;
        "Pv".clone_into(&mut field.name);
        let mut def = Self::new(OI4PV);
        def.spec.fields.push(field);
        def
    }

    /// Appends a field.
    #[must_use]
    pub fn field(mut self, field: Field) -> Self {
        self.spec.fields.push(field.0);
        self
    }

    /// An explicit DataSetClassId, e.g. of a standardized DataSet. Default:
    /// a UUIDv5 of the Tag and the field structure, stable across restarts
    /// and changing with the structure (REQ-MSG-013).
    #[must_use]
    pub fn class_id(mut self, uuid: &str) -> Self {
        self.class_id = Some(uuid.to_owned());
        self
    }

    /// The `Description` of the Metadata (`en-US`).
    #[must_use]
    pub fn description(mut self, text: &str) -> Self {
        self.spec.description = Some(text.to_owned());
        self
    }

    /// The initial PublicationList entry; default
    /// [`Publication::default_data`].
    #[must_use]
    pub fn publication(mut self, publication: Publication) -> Self {
        self.publication = publication;
        self
    }

    pub(crate) fn resolve(self) -> Result<DataSetSpec> {
        let mut spec = self.spec;
        spec.publication = self.publication.resolve()?;
        if let Some(id) = self.class_id {
            spec.class_id = Some(Uuid::parse_str(&id).map_err(|e| {
                Error::Invalid(format!("the DataSetClassId {id:?} is no UUID: {e}"))
            })?);
        }
        Ok(spec)
    }
}

/// A received `Set Data`, already checked against the field definitions:
/// it carries every field, converted to its type. A DataSet that is not
/// addressed, has no handler, misses a field or has a value that does not
/// fit is answered by the SDK without calling the handler (`BadNotFound`,
/// `BadNotWritable`, `BadInvalidArgument`, `BadTypeMismatch`); unknown
/// fields are ignored (tolerant reader, REQ-MSG-046).
#[derive(Debug, Clone)]
pub struct SetData {
    pub(crate) values: Vec<(String, Value)>,
}

impl SetData {
    /// The requested value of a field.
    pub fn get(&self, field: &str) -> Option<&Value> {
        self.values.iter().find(|(n, _)| n == field).map(|(_, v)| v)
    }

    /// All fields with their requested values in definition order.
    pub fn fields(&self) -> impl Iterator<Item = (&str, &Value)> {
        self.values.iter().map(|(n, v)| (n.as_str(), v))
    }
}

/// The decision about a `Set Data` (see [`DataSet::on_set`]).
pub(crate) type SetHandler = dyn Fn(&SetData) -> core::result::Result<(), StatusCode> + Send + Sync;

/// Handle of a Data DataSet (`Clone + Send + Sync`), usable before and
/// while the application runs.
#[derive(Clone)]
pub struct DataSet {
    pub(crate) handle: AppHandle,
    pub(crate) source: IdentificationLink,
    pub(crate) tag: String,
}

impl DataSet {
    /// The Tag (topic Filter).
    pub fn tag(&self) -> &str {
        &self.tag
    }

    /// The `Source`: the application itself or a represented asset.
    pub fn source(&self) -> &IdentificationLink {
        &self.source
    }

    /// Sets field values, all or none (state). Synchronous and
    /// non-blocking; whether and when a message is sent follows the
    /// PublicationList entry, and the last state answers `Get Data`
    /// (REQ-RES-DATA-007, REQ-PAG-007). The DataSet is published once
    /// every field has a value; the Timestamp of its DataSetMessage is the
    /// time of the last change.
    ///
    /// # Errors
    ///
    /// [`Error::Invalid`] for an unknown field, a value that does not fit
    /// the type of its field or a removed DataSet; then nothing is set.
    pub fn update<K, V>(&self, values: impl IntoIterator<Item = (K, V)>) -> Result<()>
    where
        K: AsRef<str>,
        V: Into<Value>,
    {
        let values = values
            .into_iter()
            .map(|(k, v)| (k.as_ref().to_owned(), v.into()))
            .collect();
        self.handle.update_data(&self.source, &self.tag, values)
    }

    /// The current values in field order, once every field has one.
    pub fn values(&self) -> Option<Vec<Value>> {
        self.handle.data_values(&self.source, &self.tag)
    }

    /// Makes the DataSet writable (`Set Data`, MG 9.1.4). The handler
    /// decides about a checked request ([`SetData`]): on `Ok` the SDK
    /// applies the values as state, publishes the DataSet with the ReplyTo
    /// of the `Set` (REQ-RES-DATA-012) and the Status event Good; on `Err`
    /// only the Status event with that code (REQ-RES-DATA-011,
    /// REQ-RES-EVENT-005), an `Err` with a Good code as `Bad`. Without a
    /// handler a `Set` is answered with `BadNotWritable` (REQ-CMP-043). A
    /// further call replaces the handler.
    ///
    /// The handler runs on the runtime task: keep it short and
    /// non-blocking; a panic is answered with `BadInternalError`.
    ///
    /// # Errors
    ///
    /// [`Error::Invalid`] for a removed DataSet.
    pub fn on_set(
        &self,
        handler: impl Fn(&SetData) -> core::result::Result<(), StatusCode> + Send + Sync + 'static,
    ) -> Result<()> {
        self.handle
            .on_set_data(&self.source, &self.tag, std::sync::Arc::new(handler))
    }

    /// Removes the DataSet: it is no longer published or answered and the
    /// PublicationList is published again (REQ-RES-PUBLIST-017). Its
    /// DataSetWriterId stays reserved for its Tag.
    ///
    /// # Errors
    ///
    /// [`Error::Invalid`] if it is already removed.
    pub fn remove(self) -> Result<()> {
        self.handle.remove_data(&self.source, &self.tag)
    }
}

impl fmt::Debug for DataSet {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("DataSet")
            .field("source", &self.source)
            .field("tag", &self.tag)
            .finish_non_exhaustive()
    }
}
