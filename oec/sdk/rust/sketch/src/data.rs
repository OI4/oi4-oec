//! Data DataSets with PublicationList semantics (MG 8.3.4, 8.3.8).

use std::ops::BitOr;
use std::time::Duration;

use crate::identity::{IdentificationLink, LocalizedText};
use crate::status::StatusCode;
use crate::Result;

/// OPC UA built-in types usable in DataSet fields (REQ-MSG-037). The SDK
/// derives the `FieldMetaData` (BuiltInType, ValueRank, DataType NodeId) of
/// the Metadata from it (REQ-RES-META-003).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum BuiltinType {
    Boolean,
    SByte,
    Byte,
    Int16,
    UInt16,
    Int32,
    UInt32,
    Int64,
    UInt64,
    Float,
    Double,
    String,
    DateTime,
    Guid,
    ByteString,
    LocalizedText,
}

/// A field value. `From` conversions exist for the Rust primitives,
/// `String`/`&str`, byte vectors and [`LocalizedText`]; a value is
/// converted to the type of its field when set (amendment C-054: the
/// `Json` escape hatch is replaced by typed variants).
#[derive(Debug, Clone, PartialEq)]
#[non_exhaustive]
pub enum Value {
    Boolean(bool),
    Int32(i32),
    Int64(i64),
    UInt32(u32),
    UInt64(u64),
    Float(f32),
    Double(f64),
    /// A String, or a Guid in its string form.
    String(String),
    /// Unix time in ms; encoded as ISO 8601 DateTime.
    DateTime(i64),
    /// A ByteString.
    Bytes(Vec<u8>),
    LocalizedText(LocalizedText),
}

impl From<bool> for Value {
    fn from(v: bool) -> Self {
        Value::Boolean(v)
    }
}
impl From<i32> for Value {
    fn from(v: i32) -> Self {
        Value::Int32(v)
    }
}
impl From<f64> for Value {
    fn from(v: f64) -> Self {
        Value::Double(v)
    }
}
impl From<&str> for Value {
    fn from(v: &str) -> Self {
        Value::String(v.to_owned())
    }
}

/// Definition of one DataSet field (becomes a `FieldMetaData`).
#[derive(Debug, Clone)]
pub struct Field {
    name: String,
    ty: BuiltinType,
}

impl Field {
    /// Field names are unique within the DataSet (REQ-MSG-038).
    pub fn new(name: &str, ty: BuiltinType) -> Self {
        unimplemented!()
    }

    pub fn double(name: &str) -> Self {
        Field::new(name, BuiltinType::Double)
    }

    /// EngineeringUnit of the field (Metadata property).
    pub fn unit(self, unit: &str) -> Self {
        unimplemented!()
    }

    pub fn description(self, text: &str) -> Self {
        unimplemented!()
    }

    /// Range of the values (`EURange` property of the Metadata,
    /// amendment C-054).
    pub fn range(self, low: f64, high: f64) -> Self {
        unimplemented!()
    }
}

/// Publication `Mode` of a PublicationList entry (MG 8.3.8). The four
/// publication forms combine; the SDK maps the combination to the enum value
/// (`OFF_0` … `ON_REQUEST_APPLICATION_SOURCE_FILTER_15`).
///
/// Defaults of the built-in entries: MAM, Health, Profile and
/// ReferenceDesignation `ON_REQUEST_SOURCE_10`; Metadata
/// `ON_REQUEST_FILTER_9` (fixed, REQ-RES-PUBLIST-012); Event entries
/// `FILTER_4`, Syslog Debug/Informational `OFF_0` (REQ-TOP-024). The
/// defaults of all resources that support `Get` contain `ON_REQUEST`, as
/// MG 8.3.8 recommends (REQ-RES-PUBLIST-026, SF-101); `Get` is answered for
/// every Mode except `OFF_0` (REQ-RES-PUBLIST-027).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Mode {
    on_request: bool,
    application: bool,
    source: bool,
    filter: bool,
}

impl Mode {
    /// Neither published nor answered (REQ-RES-PUBLIST-007).
    pub const OFF: Mode = Mode { on_request: false, application: false, source: false, filter: false };
    /// Only as answer to a `Get` (REQ-RES-PUBLIST-008).
    pub const ON_REQUEST: Mode = Mode { on_request: true, application: false, source: false, filter: false };
    /// With all DataSets of the resource on `Pub/<Resource>` (REQ-RES-PUBLIST-009).
    pub const APPLICATION: Mode = Mode { on_request: false, application: true, source: false, filter: false };
    /// With all DataSets of the resource and Source on `Pub/<Resource>/<Source>` (REQ-RES-PUBLIST-010).
    pub const SOURCE: Mode = Mode { on_request: false, application: false, source: true, filter: false };
    /// Alone on `Pub/<Resource>/<Source>/<Filter>` (REQ-RES-PUBLIST-011).
    pub const FILTER: Mode = Mode { on_request: false, application: false, source: false, filter: true };

    /// `self | other` in `const` contexts (amendment C-054).
    pub const fn union(self, other: Mode) -> Mode {
        Mode {
            on_request: self.on_request || other.on_request,
            application: self.application || other.application,
            source: self.source || other.source,
            filter: self.filter || other.filter,
        }
    }
}

impl BitOr for Mode {
    type Output = Mode;
    fn bitor(self, rhs: Mode) -> Mode {
        Mode {
            on_request: self.on_request || rhs.on_request,
            application: self.application || rhs.application,
            source: self.source || rhs.source,
            filter: self.filter || rhs.filter,
        }
    }
}

/// Which settings of the PublicationList entry consumers may change with a
/// `Set PublicationList` (`Config`, REQ-RES-PUBLIST-016).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Writable {
    /// `NONE_0` (default).
    Nothing,
    /// `MODE_1`.
    Mode,
    /// `INTERVAL_2`: Interval and Precisions.
    Interval,
    /// `MODE_AND_INTERVAL_3`.
    ModeAndInterval,
}

/// Initial PublicationList entry of a DataSet. The SDK applies it to every
/// update (REQ-RES-PUBLIST-007 … 015), persists changes made by a consumer
/// and republishes the PublicationList on every change (REQ-RES-PUBLIST-017).
#[derive(Debug, Clone)]
pub struct Publication {
    mode: Mode,
}

impl Publication {
    /// SDK default for Data: `Mode::ON_REQUEST | Mode::FILTER`
    /// (`ON_REQUEST_FILTER_9`), Interval 0 (publish on change,
    /// REQ-MQTT-019, REQ-RES-PUBLIST-013), no Precisions, `Writable::Nothing`.
    pub fn default_data() -> Self {
        unimplemented!()
    }

    pub fn mode(self, mode: Mode) -> Self {
        unimplemented!()
    }

    /// Cyclic publication; overrides Precisions (REQ-RES-PUBLIST-013/014).
    pub fn interval(self, interval: Duration) -> Self {
        unimplemented!()
    }

    /// Dead band of a numeric field for Interval 0 (REQ-RES-PUBLIST-015).
    pub fn precision(self, field: &str, precision: f64) -> Self {
        unimplemented!()
    }

    pub fn writable(self, writable: Writable) -> Self {
        unimplemented!()
    }
}

/// Definition of a Data DataSet.
#[derive(Debug, Clone)]
pub struct DataSetDef {
    tag: String,
}

impl DataSetDef {
    /// `tag` is the topic Filter (Tag) of the DataSet (REQ-TOP-019 … 022).
    pub fn new(tag: &str) -> Self {
        unimplemented!()
    }

    /// The standardized process value DataSet `Oi4Pv` with field `Pv` and
    /// optional `Sv1` … `Sv255` (REQ-RES-DATA-002 … 005).
    pub fn oi4pv(pv: Field) -> Self {
        unimplemented!()
    }

    pub fn field(self, field: Field) -> Self {
        unimplemented!()
    }

    /// Explicit DataSetClassId. Default: a UUIDv5 derived from the tag and
    /// the field structure, so it is stable across restarts and changes
    /// with the structure (REQ-MSG-013).
    pub fn class_id(self, uuid: &str) -> Self {
        unimplemented!()
    }

    pub fn description(self, text: &str) -> Self {
        unimplemented!()
    }

    pub fn publication(self, publication: Publication) -> Self {
        unimplemented!()
    }
}

/// A received `Set Data`, already checked against the field definitions
/// (missing field → `BadInvalidArgument`, wrong type → `BadTypeMismatch`
/// without calling the handler; unknown fields are ignored, tolerant reader
/// REQ-MSG-046 – amendment C-055).
#[derive(Debug)]
pub struct SetData {
    _private: (),
}

impl SetData {
    pub fn get(&self, field: &str) -> Option<&Value> {
        unimplemented!()
    }

    pub fn fields(&self) -> impl Iterator<Item = (&str, &Value)> {
        std::iter::empty()
    }
}

/// Handle of a Data DataSet (`Clone + Send + Sync`).
#[derive(Clone)]
pub struct DataSet {
    _private: (),
}

impl DataSet {
    /// The Tag (topic Filter, amendment C-054).
    pub fn tag(&self) -> &str {
        unimplemented!()
    }

    /// The `Source`: the application itself or a represented asset
    /// (amendment C-054).
    pub fn source(&self) -> &IdentificationLink {
        unimplemented!()
    }

    /// Updates field values (state), all or none. Synchronous and
    /// non-blocking. Whether and when a message is sent follows the
    /// PublicationList entry; the last state answers `Get Data`
    /// (REQ-RES-DATA-007), a series of pages is built from one snapshot
    /// (REQ-PAG-007). An unknown field or a value that does not fit its
    /// field is an error (amendment C-054: returns `Result`).
    pub fn update<K, V>(&self, values: impl IntoIterator<Item = (K, V)>) -> Result<()>
    where
        K: AsRef<str>,
        V: Into<Value>,
    {
        unimplemented!()
    }

    /// The current values in field order, once every field has one
    /// (amendment C-054).
    pub fn values(&self) -> Option<Vec<Value>> {
        unimplemented!()
    }

    /// Makes the DataSet writable (`Set Data`). On `Ok` the SDK applies
    /// the values, publishes the changed DataSet (REQ-RES-DATA-012) and a
    /// Status event Good; on `Err` a Status event with the code
    /// (REQ-RES-DATA-011, REQ-RES-EVENT-005, REQ-TOP-014). Without handler
    /// a `Set` is answered with `BadNotWritable` (REQ-CMP-043). Returns
    /// an error for a removed DataSet (amendment C-055).
    pub fn on_set(
        &self,
        f: impl Fn(&SetData) -> core::result::Result<(), StatusCode> + Send + Sync + 'static,
    ) -> Result<()> {
        unimplemented!()
    }

    /// Removes the DataSet (REQ-RES-PUBLIST-017).
    pub fn remove(self) -> Result<()> {
        unimplemented!()
    }
}
