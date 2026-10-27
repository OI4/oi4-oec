//! Data DataSets of the provider (MG 8.3.4, MG 8.3.5, MG 8.3.8; P4
//! increment 6b, ADR-0026 §13): the definition of a DataSet, its field
//! values, the Metadata derived from the fields, the DataSetClassId as
//! UUIDv5 and the publication settings of its PublicationList entry
//! (Mode, Interval, Precisions).
//!
//! The values are state (ADR-0020 §7): [`crate::provider::Provider`] keeps
//! the last complete value set of every DataSet and publishes it as its
//! entry says.

use core::fmt;
use core::ops::BitOr;
use std::collections::BTreeMap;

use serde_json::{Map, Number, Value as Json, json};
use time::OffsetDateTime;
use uuid::Uuid;

use crate::message::{
    ByteString, DataSetMetaData, DateTime, FieldMetaData, KeyValuePair, LocalizedText,
    RESERVED_CLASS_IDS,
};
use crate::resource::{PublicationConfig, PublicationMode};

/// The OPC UA built-in types of a DataSet field (REQ-MSG-037); scalars
/// only (ValueRank −1). The Metadata carries the BuiltInType and the
/// DataType of the type (REQ-RES-META-003).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum BuiltinType {
    /// `Boolean` (1).
    Boolean,
    /// `SByte` (2).
    SByte,
    /// `Byte` (3).
    Byte,
    /// `Int16` (4).
    Int16,
    /// `UInt16` (5).
    UInt16,
    /// `Int32` (6).
    Int32,
    /// `UInt32` (7).
    UInt32,
    /// `Int64` (8), encoded as JSON string (OPC UA Part 6 5.4.2.3).
    Int64,
    /// `UInt64` (9), encoded as JSON string.
    UInt64,
    /// `Float` (10).
    Float,
    /// `Double` (11).
    Double,
    /// `String` (12).
    String,
    /// `DateTime` (13), encoded as RFC 3339 string.
    DateTime,
    /// `Guid` (14), encoded as lower-case string.
    Guid,
    /// `ByteString` (15), encoded as base64 string.
    ByteString,
    /// `LocalizedText` (21).
    LocalizedText,
}

impl BuiltinType {
    /// The BuiltInType id; the DataType of the field is `i=<id>`.
    pub const fn id(self) -> u8 {
        match self {
            Self::Boolean => 1,
            Self::SByte => 2,
            Self::Byte => 3,
            Self::Int16 => 4,
            Self::UInt16 => 5,
            Self::Int32 => 6,
            Self::UInt32 => 7,
            Self::Int64 => 8,
            Self::UInt64 => 9,
            Self::Float => 10,
            Self::Double => 11,
            Self::String => 12,
            Self::DateTime => 13,
            Self::Guid => 14,
            Self::ByteString => 15,
            Self::LocalizedText => 21,
        }
    }

    /// The numeric types, for which a Precision is a dead band
    /// (REQ-RES-PUBLIST-015).
    pub const fn is_numeric(self) -> bool {
        matches!(
            self,
            Self::SByte
                | Self::Byte
                | Self::Int16
                | Self::UInt16
                | Self::Int32
                | Self::UInt32
                | Self::Int64
                | Self::UInt64
                | Self::Float
                | Self::Double
        )
    }

    /// The range of an integer type.
    const fn int_range(self) -> Option<(i128, i128)> {
        Some(match self {
            Self::SByte => (i8::MIN as i128, i8::MAX as i128),
            Self::Byte => (0, u8::MAX as i128),
            Self::Int16 => (i16::MIN as i128, i16::MAX as i128),
            Self::UInt16 => (0, u16::MAX as i128),
            Self::Int32 => (i32::MIN as i128, i32::MAX as i128),
            Self::UInt32 => (0, u32::MAX as i128),
            Self::Int64 => (i64::MIN as i128, i64::MAX as i128),
            Self::UInt64 => (0, u64::MAX as i128),
            _ => return None,
        })
    }
}

impl fmt::Display for BuiltinType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        fmt::Debug::fmt(self, f)
    }
}

/// A field value. It is converted to the type of its field when set: an
/// integer fits every numeric type whose range holds it, a Float or Double
/// a Float or Double field, a string a Guid field (parsed) and a
/// LocalizedText field (locale `en-US`).
#[derive(Debug, Clone, PartialEq)]
#[non_exhaustive]
pub enum Value {
    /// A Boolean.
    Boolean(bool),
    /// A signed integer up to 32 bit.
    Int32(i32),
    /// A signed integer.
    Int64(i64),
    /// An unsigned integer up to 32 bit.
    UInt32(u32),
    /// An unsigned integer.
    UInt64(u64),
    /// A Float; NaN and ±Infinity are encoded as strings.
    Float(f32),
    /// A Double; NaN and ±Infinity are encoded as strings.
    Double(f64),
    /// A String, or a Guid in its string form.
    String(String),
    /// A DateTime as Unix time in milliseconds.
    DateTime(i64),
    /// A ByteString.
    Bytes(Vec<u8>),
    /// A LocalizedText.
    LocalizedText(LocalizedText),
}

macro_rules! value_from {
    ($($t:ty => $v:ident),* $(,)?) => {$(
        impl From<$t> for Value {
            fn from(v: $t) -> Self {
                Self::$v(v.into())
            }
        }
    )*};
}

value_from!(
    bool => Boolean, i8 => Int32, i16 => Int32, i32 => Int32, i64 => Int64,
    u8 => UInt32, u16 => UInt32, u32 => UInt32, u64 => UInt64,
    f32 => Float, f64 => Double, String => String, &str => String,
    Vec<u8> => Bytes, &[u8] => Bytes, LocalizedText => LocalizedText,
);

impl Value {
    fn integer(&self) -> Option<i128> {
        match *self {
            Self::Int32(v) => Some(v.into()),
            Self::Int64(v) => Some(v.into()),
            Self::UInt32(v) => Some(v.into()),
            Self::UInt64(v) => Some(v.into()),
            _ => None,
        }
    }

    /// The value as f64 for the dead band.
    #[allow(clippy::cast_precision_loss)]
    fn number(&self) -> Option<f64> {
        match *self {
            Self::Float(v) => Some(v.into()),
            Self::Double(v) => Some(v),
            _ => self.integer().map(|i| i as f64),
        }
    }

    fn kind(&self) -> &'static str {
        match self {
            Self::Boolean(_) => "Boolean",
            Self::Int32(_) | Self::Int64(_) => "signed integer",
            Self::UInt32(_) | Self::UInt64(_) => "unsigned integer",
            Self::Float(_) => "Float",
            Self::Double(_) => "Double",
            Self::String(_) => "String",
            Self::DateTime(_) => "DateTime",
            Self::Bytes(_) => "ByteString",
            Self::LocalizedText(_) => "LocalizedText",
        }
    }

    /// Equal for the change detection: NaN equals NaN.
    pub(crate) fn same(&self, other: &Self) -> bool {
        match (self.number(), other.number()) {
            (Some(a), Some(b)) if a.is_nan() && b.is_nan() => true,
            _ => self == other,
        }
    }
}

/// The publication forms of a PublicationList entry (MG 8.3.8): the Mode
/// `OFF_0` … `ON_REQUEST_APPLICATION_SOURCE_FILTER_15` as four flags.
///
/// A Mode with a publication form is sent with `ON_REQUEST`
/// (REQ-RES-PUBLIST-026); `Get` is answered for every Mode except `OFF`
/// (REQ-RES-PUBLIST-027).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default)]
pub struct Mode(u8);

const ON_REQUEST: u8 = 1;
const APPLICATION: u8 = 2;
const SOURCE: u8 = 4;
const FILTER: u8 = 8;

impl Mode {
    /// Neither published nor answered (REQ-RES-PUBLIST-007).
    pub const OFF: Self = Self(0);
    /// Only as answer to a `Get` (REQ-RES-PUBLIST-008).
    pub const ON_REQUEST: Self = Self(ON_REQUEST);
    /// With all DataSets of the resource on `Pub/<Resource>`
    /// (REQ-RES-PUBLIST-009).
    pub const APPLICATION: Self = Self(APPLICATION);
    /// With all DataSets of the resource and Source on
    /// `Pub/<Resource>/<Source>` (REQ-RES-PUBLIST-010).
    pub const SOURCE: Self = Self(SOURCE);
    /// Alone on `Pub/<Resource>/<Source>/<Filter>` (REQ-RES-PUBLIST-011).
    pub const FILTER: Self = Self(FILTER);

    /// Both Modes combined, like `|` but usable in `const`.
    #[must_use]
    pub const fn union(self, other: Self) -> Self {
        Self(self.0 | other.0)
    }

    /// Not `OFF`: the DataSet answers `Get` (REQ-RES-PUBLIST-027).
    pub const fn is_on(self) -> bool {
        self.0 != 0
    }

    /// Publishes without `Get`: one of APPLICATION, SOURCE, FILTER.
    pub const fn publishes(self) -> bool {
        self.0 & (APPLICATION | SOURCE | FILTER) != 0
    }

    /// Enables APPLICATION.
    pub const fn application(self) -> bool {
        self.0 & APPLICATION != 0
    }

    /// Enables SOURCE.
    pub const fn source(self) -> bool {
        self.0 & SOURCE != 0
    }

    /// Enables FILTER.
    pub const fn filter(self) -> bool {
        self.0 & FILTER != 0
    }

    /// The Mode of the entry; a publication form is combined with
    /// `ON_REQUEST` (REQ-RES-PUBLIST-026).
    pub const fn to_publication_mode(self) -> PublicationMode {
        use PublicationMode as M;
        match (self.0 | ON_REQUEST) & !ON_REQUEST {
            0 if self.0 == ON_REQUEST => M::OnRequest,
            0 => M::Off,
            FILTER => M::OnRequestFilter,
            SOURCE => M::OnRequestSource,
            APPLICATION => M::OnRequestApplication,
            0b1100 => M::OnRequestSourceFilter,
            0b1010 => M::OnRequestApplicationFilter,
            0b0110 => M::OnRequestApplicationSource,
            _ => M::OnRequestApplicationSourceFilter,
        }
    }
}

impl From<PublicationMode> for Mode {
    fn from(mode: PublicationMode) -> Self {
        use PublicationMode as M;
        let bits = match mode {
            M::Off => 0,
            M::OnRequest => ON_REQUEST,
            M::Application => APPLICATION,
            M::Source => SOURCE,
            M::Filter => FILTER,
            M::ApplicationSource => APPLICATION | SOURCE,
            M::ApplicationFilter => APPLICATION | FILTER,
            M::SourceFilter => SOURCE | FILTER,
            M::ApplicationSourceFilter => APPLICATION | SOURCE | FILTER,
            M::OnRequestFilter => ON_REQUEST | FILTER,
            M::OnRequestSource => ON_REQUEST | SOURCE,
            M::OnRequestApplication => ON_REQUEST | APPLICATION,
            M::OnRequestSourceFilter => ON_REQUEST | SOURCE | FILTER,
            M::OnRequestApplicationFilter => ON_REQUEST | APPLICATION | FILTER,
            M::OnRequestApplicationSource => ON_REQUEST | APPLICATION | SOURCE,
            M::OnRequestApplicationSourceFilter => ON_REQUEST | APPLICATION | SOURCE | FILTER,
        };
        Self(bits)
    }
}

impl BitOr for Mode {
    type Output = Self;

    fn bitor(self, rhs: Self) -> Self {
        Self(self.0 | rhs.0)
    }
}

/// What a `Set PublicationList` may change in the entry: its `Config`
/// (REQ-RES-PUBLIST-001, REQ-RES-PUBLIST-016).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default)]
pub enum Writable {
    /// `NONE_0` (default).
    #[default]
    Nothing,
    /// `MODE_1`.
    Mode,
    /// `INTERVAL_2`: Interval and Precisions.
    Interval,
    /// `MODE_AND_INTERVAL_3`.
    ModeAndInterval,
}

impl Writable {
    /// The `Config` of the entry; `None` for the default `NONE_0`.
    pub const fn to_config(self) -> Option<PublicationConfig> {
        match self {
            Self::Nothing => None,
            Self::Mode => Some(PublicationConfig::Mode),
            Self::Interval => Some(PublicationConfig::Interval),
            Self::ModeAndInterval => Some(PublicationConfig::ModeAndInterval),
        }
    }
}

/// One field of a DataSet; it becomes a FieldMetaData (REQ-MSG-037).
#[derive(Debug, Clone, PartialEq)]
pub struct FieldSpec {
    /// The name, unique in the DataSet (REQ-MSG-038).
    pub name: String,
    /// The type.
    pub ty: BuiltinType,
    /// The Description (`en-US`), e.g. the measuring point of Pv
    /// (REQ-RES-DATA-005).
    pub description: Option<String>,
    /// The display name of the engineering unit (property
    /// `EngineeringUnits`).
    pub unit: Option<String>,
    /// The range of a numeric value (property `EURange`).
    pub range: Option<(f64, f64)>,
}

impl FieldSpec {
    /// A field without description, unit and range.
    pub fn new(name: impl Into<String>, ty: BuiltinType) -> Self {
        Self {
            name: name.into(),
            ty,
            description: None,
            unit: None,
            range: None,
        }
    }
}

/// The settings of the PublicationList entry of a DataSet (MG 8.3.8).
#[derive(Debug, Clone, PartialEq)]
pub struct PublicationSpec {
    /// The Mode; default `ON_REQUEST | FILTER` (`ON_REQUEST_FILTER_9`).
    pub mode: Mode,
    /// The Interval in ms; 0 publishes on change (REQ-RES-PUBLIST-013).
    pub interval_ms: u32,
    /// The dead band of numeric fields at Interval 0
    /// (REQ-RES-PUBLIST-015).
    pub precisions: BTreeMap<String, f64>,
    /// The `Config` of the entry.
    pub writable: Writable,
}

impl Default for PublicationSpec {
    fn default() -> Self {
        Self {
            mode: Mode::ON_REQUEST | Mode::FILTER,
            interval_ms: 0,
            precisions: BTreeMap::new(),
            writable: Writable::Nothing,
        }
    }
}

/// The definition of a Data DataSet.
#[derive(Debug, Clone, PartialEq)]
pub struct DataSetSpec {
    /// The Tag: the Filter of its topics and its WriterGroupName
    /// (REQ-TOP-022, REQ-MSG-024).
    pub tag: String,
    /// The Description of the Metadata (`en-US`).
    pub description: Option<String>,
    /// The fields, in their order.
    pub fields: Vec<FieldSpec>,
    /// An explicit DataSetClassId; default [`class_id`].
    pub class_id: Option<Uuid>,
    /// The PublicationList entry.
    pub publication: PublicationSpec,
}

impl DataSetSpec {
    /// A DataSet with the default publication and no fields yet.
    pub fn new(tag: impl Into<String>) -> Self {
        Self {
            tag: tag.into(),
            description: None,
            fields: Vec::new(),
            class_id: None,
            publication: PublicationSpec::default(),
        }
    }

    /// Checks the definition beyond the Tag, which the topic checks.
    ///
    /// # Errors
    ///
    /// The violated rule.
    pub fn check(&self) -> Result<(), String> {
        if self.fields.is_empty() {
            return Err(format!("the DataSet {:?} has no field", self.tag));
        }
        for (i, f) in self.fields.iter().enumerate() {
            if f.name.is_empty() {
                return Err("a field name is empty".to_owned());
            }
            if self.fields[..i].iter().any(|g| g.name == f.name) {
                return Err(format!(
                    "the field name {:?} occurs twice (REQ-MSG-038)",
                    f.name
                ));
            }
            if let Some((low, high)) = f.range
                && !(low.is_finite() && high.is_finite() && low <= high)
            {
                return Err(format!(
                    "the range of {:?} is not a finite Low ≤ High",
                    f.name
                ));
            }
        }
        if self.tag == OI4PV {
            check_oi4pv(&self.fields)?;
        }
        for (name, p) in &self.publication.precisions {
            let Some(f) = self.fields.iter().find(|f| &f.name == name) else {
                return Err(format!(
                    "the Precision of {name:?} names no field (REQ-RES-PUBLIST-001)"
                ));
            };
            if !f.ty.is_numeric() {
                return Err(format!(
                    "the field {name:?} is no number and has no Precision (REQ-RES-PUBLIST-015)"
                ));
            }
            if !(p.is_finite() && *p >= 0.0) {
                return Err(format!(
                    "the Precision of {name:?} is no Real ≥ 0 (REQ-RES-PUBLIST-001)"
                ));
            }
        }
        if let Some(id) = self.class_id
            && (id.is_nil() || RESERVED_CLASS_IDS.contains(&id))
        {
            return Err(format!(
                "the DataSetClassId {id} is nil or reserved (REQ-MSG-014)"
            ));
        }
        Ok(())
    }
}

/// The Tag of the Oi4Pv DataSet (REQ-RES-DATA-003).
pub const OI4PV: &str = "Oi4Pv";

fn check_oi4pv(fields: &[FieldSpec]) -> Result<(), String> {
    if !fields.iter().any(|f| f.name == "Pv") {
        return Err("the Oi4Pv DataSet has the field Pv (REQ-RES-DATA-004)".to_owned());
    }
    for f in fields {
        if f.name != "Pv" && crate::resource::sv_index(&f.name).is_none() {
            return Err(format!(
                "{:?} is no field of Oi4Pv: Pv and Sv1 … Sv255 only (REQ-RES-DATA-004)",
                f.name
            ));
        }
    }
    Ok(())
}

/// The namespace of the derived ids: UUIDv5 of the URL namespace and
/// `https://schemas.openindustry4.com/oec/v2/sdk/data-set-class`.
pub fn namespace() -> Uuid {
    Uuid::new_v5(
        &Uuid::NAMESPACE_URL,
        b"https://schemas.openindustry4.com/oec/v2/sdk/data-set-class",
    )
}

/// The derived DataSetClassId (REQ-MSG-013): UUIDv5 in [`namespace`] of
/// the JSON array `[<Tag>, [[<Name>, <BuiltInType>, <ValueRank>], …]]` of
/// the Tag and the fields in their order. It stays the same across
/// restarts and changes with the structure; descriptions, units and the
/// publication settings are no part of it.
///
/// # Panics
///
/// Never: the array always serializes.
pub fn class_id(tag: &str, fields: &[FieldSpec]) -> Uuid {
    let fields: Vec<Json> = fields
        .iter()
        .map(|f| json!([f.name, f.ty.id(), -1]))
        .collect();
    let canonical = serde_json::to_vec(&json!([tag, fields])).expect("JSON always serializes");
    let id = Uuid::new_v5(&namespace(), &canonical);
    if RESERVED_CLASS_IDS.contains(&id) {
        // unreachable in practice: a reserved id is no UUIDv5
        return Uuid::new_v5(&id, &canonical);
    }
    id
}

/// The DataSetFieldId of a field: UUIDv5 of its name in the namespace of
/// the DataSetClassId.
pub fn field_id(class_id: Uuid, name: &str) -> Uuid {
    Uuid::new_v5(&class_id, name.as_bytes())
}

/// The DataSetMetaData of a DataSet without ConfigurationVersion
/// (REQ-RES-META-003, REQ-MSG-034).
pub fn meta_data(spec: &DataSetSpec, class_id: Uuid) -> DataSetMetaData {
    let mut m = DataSetMetaData::new(spec.tag.clone());
    m.description = spec.description.as_deref().map(LocalizedText::from);
    m.data_set_class_id = Some(class_id);
    m.fields = Some(
        spec.fields
            .iter()
            .map(|f| field_meta_data(f, class_id))
            .collect(),
    );
    m
}

fn field_meta_data(f: &FieldSpec, class_id: Uuid) -> FieldMetaData {
    let mut properties = Vec::new();
    if let Some((low, high)) = f.range {
        properties.push(extension_object(
            "EURange",
            json!({"UaTypeId": "i=884", "Low": number(low), "High": number(high)}),
        ));
    }
    if let Some(unit) = &f.unit {
        properties.push(extension_object(
            "EngineeringUnits",
            json!({
                "UaTypeId": "i=887",
                "NamespaceUri": "http://www.opcfoundation.org/UA/units/un/cefact",
                "UnitId": -1,
                "DisplayName": {"Text": unit},
            }),
        ));
    }
    FieldMetaData {
        name: f.name.clone(),
        description: LocalizedText::from(f.description.as_deref().unwrap_or_default()),
        field_flags: 0,
        built_in_type: f.ty.id(),
        data_type: format!("i={}", f.ty.id()),
        value_rank: -1,
        array_dimensions: Vec::new(),
        max_string_length: 0,
        data_set_field_id: field_id(class_id, &f.name),
        properties,
    }
}

fn extension_object(key: &str, value: Json) -> KeyValuePair {
    let mut m = Map::new();
    m.insert("UaType".to_owned(), json!(22));
    m.insert("Value".to_owned(), value);
    KeyValuePair {
        key: key.to_owned(),
        value: m,
    }
}

/// A finite f64 as JSON number with the shortest form of an f32 or f64.
fn number(v: f64) -> Json {
    Number::from_f64(v).map_or(Json::Null, Json::Number)
}

/// The earliest and latest OPC UA DateTime (1601-01-01, 9999-12-31) in
/// Unix ms.
const DATE_TIME_RANGE: (i64, i64) = (-11_644_473_600_000, 253_402_300_799_999);

/// Converts `v` to the type `ty` (see [`Value`]).
///
/// # Errors
///
/// The value does not fit the type.
///
/// # Panics
///
/// Never: the integer range of each integer type fits `i128`.
pub fn coerce(ty: BuiltinType, v: Value) -> Result<Value, String> {
    let mismatch = |v: &Value| Err(format!("a {} is no {ty} value", v.kind()));
    match ty {
        BuiltinType::Boolean => match v {
            Value::Boolean(_) => Ok(v),
            v => mismatch(&v),
        },
        BuiltinType::Float => match v {
            Value::Float(_) => Ok(v),
            #[allow(clippy::cast_possible_truncation)]
            Value::Double(d) => {
                let f = d as f32;
                if d.is_finite() && !f.is_finite() {
                    Err(format!("{d} exceeds the range of a Float"))
                } else {
                    Ok(Value::Float(f))
                }
            }
            #[allow(clippy::cast_possible_truncation)]
            v => match v.number() {
                Some(n) => Ok(Value::Float(n as f32)),
                None => mismatch(&v),
            },
        },
        BuiltinType::Double => match v.number() {
            Some(n) => Ok(Value::Double(n)),
            None => mismatch(&v),
        },
        BuiltinType::String => match v {
            Value::String(_) => Ok(v),
            v => mismatch(&v),
        },
        BuiltinType::DateTime => match v {
            Value::DateTime(ms) if (DATE_TIME_RANGE.0..=DATE_TIME_RANGE.1).contains(&ms) => Ok(v),
            Value::DateTime(ms) => Err(format!(
                "{ms} ms is outside the range of an OPC UA DateTime (1601 … 9999)"
            )),
            v => mismatch(&v),
        },
        BuiltinType::Guid => match v {
            Value::String(s) => Uuid::parse_str(&s)
                .map(|u| Value::String(u.hyphenated().to_string()))
                .map_err(|e| format!("{s:?} is no Guid: {e}")),
            v => mismatch(&v),
        },
        BuiltinType::ByteString => match v {
            Value::Bytes(_) => Ok(v),
            v => mismatch(&v),
        },
        BuiltinType::LocalizedText => match v {
            Value::LocalizedText(_) => Ok(v),
            Value::String(s) => Ok(Value::LocalizedText(LocalizedText::from(s))),
            v => mismatch(&v),
        },
        _ => {
            let (low, high) = ty.int_range().expect("the remaining types are integers");
            let Some(i) = v.integer() else {
                return mismatch(&v);
            };
            if !(low..=high).contains(&i) {
                return Err(format!("{i} is outside the range of {ty}"));
            }
            Ok(match ty {
                BuiltinType::Int64 => Value::Int64(i64::try_from(i).expect("in range")),
                BuiltinType::UInt64 => Value::UInt64(u64::try_from(i).expect("in range")),
                _ if low < 0 => Value::Int32(i32::try_from(i).expect("in range")),
                _ => Value::UInt32(u32::try_from(i).expect("in range")),
            })
        }
    }
}

/// The JSON field value of a value converted by [`coerce`] (REQ-MSG-031,
/// OPC UA Part 6 5.4.2): Int64 and UInt64 as strings, NaN and ±Infinity
/// as `"NaN"`, `"Infinity"`, `"-Infinity"`.
pub fn encode(ty: BuiltinType, v: &Value) -> Json {
    match v {
        Value::Boolean(b) => Json::Bool(*b),
        Value::Int64(i) if ty == BuiltinType::Int64 => Json::String(i.to_string()),
        Value::UInt64(i) if ty == BuiltinType::UInt64 => Json::String(i.to_string()),
        Value::Int32(i) => json!(i),
        Value::Int64(i) => json!(i),
        Value::UInt32(i) => json!(i),
        Value::UInt64(i) => json!(i),
        Value::Float(f) => float(f64::from(*f), || f.to_string().parse().ok()),
        Value::Double(d) => float(*d, || Some(*d)),
        Value::String(s) => Json::String(s.clone()),
        Value::DateTime(ms) => {
            let at = OffsetDateTime::from_unix_timestamp_nanos(i128::from(*ms) * 1_000_000)
                .unwrap_or(OffsetDateTime::UNIX_EPOCH);
            Json::String(DateTime::from(at).as_str().to_owned())
        }
        Value::Bytes(b) => Json::String(ByteString::from_bytes(b).as_str().to_owned()),
        Value::LocalizedText(t) => json!({"Locale": t.locale, "Text": t.text}),
    }
}

/// A Float or Double; `shortest` gives the f64 with the shortest decimal
/// form of the value (an f32 as written, not widened).
fn float(v: f64, shortest: impl Fn() -> Option<f64>) -> Json {
    if v.is_nan() {
        Json::String("NaN".to_owned())
    } else if v.is_infinite() {
        Json::String(if v > 0.0 { "Infinity" } else { "-Infinity" }.to_owned())
    } else {
        number(shortest().unwrap_or(v))
    }
}

/// Whether a new value set is published at Interval 0
/// (REQ-RES-PUBLIST-013, REQ-RES-PUBLIST-015): a field changed, a numeric
/// field with a Precision > 0 by at least the Precision from `last`.
pub fn exceeds_dead_band(last: &[Value], new: &[Value], precisions: &[f64]) -> bool {
    last.iter()
        .zip(new)
        .zip(precisions.iter().copied().chain(core::iter::repeat(0.0)))
        .any(|((a, b), p)| {
            if a.same(b) {
                return false;
            }
            match (a.number(), b.number()) {
                (Some(x), Some(y)) if p > 0.0 && x.is_finite() && y.is_finite() => {
                    (y - x).abs() >= p
                }
                _ => true,
            }
        })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn modes_map_to_the_enumeration() {
        use PublicationMode as M;
        assert_eq!(Mode::OFF.to_publication_mode(), M::Off);
        assert_eq!(Mode::ON_REQUEST.to_publication_mode(), M::OnRequest);
        assert_eq!(Mode::FILTER.to_publication_mode(), M::OnRequestFilter);
        assert_eq!(
            (Mode::APPLICATION | Mode::SOURCE | Mode::FILTER).to_publication_mode(),
            M::OnRequestApplicationSourceFilter
        );
        for m in [
            M::Off,
            M::OnRequest,
            M::OnRequestFilter,
            M::OnRequestSource,
            M::OnRequestApplication,
            M::OnRequestSourceFilter,
            M::OnRequestApplicationFilter,
            M::OnRequestApplicationSource,
            M::OnRequestApplicationSourceFilter,
        ] {
            assert_eq!(Mode::from(m).to_publication_mode(), m);
        }
        assert_eq!(
            Mode::from(M::SourceFilter).to_publication_mode(),
            M::OnRequestSourceFilter
        );
    }

    #[test]
    fn values_are_converted_to_the_field_type() {
        use BuiltinType as T;
        assert_eq!(coerce(T::Byte, Value::Int32(255)), Ok(Value::UInt32(255)));
        assert_eq!(coerce(T::Int16, Value::UInt64(7)), Ok(Value::Int32(7)));
        assert!(coerce(T::Byte, Value::Int32(256)).is_err());
        assert!(coerce(T::UInt32, Value::Int32(-1)).is_err());
        assert_eq!(coerce(T::Int64, Value::UInt32(7)), Ok(Value::Int64(7)));
        assert!(coerce(T::Int32, Value::Double(1.0)).is_err());
        assert_eq!(coerce(T::Double, Value::Int32(3)), Ok(Value::Double(3.0)));
        assert_eq!(coerce(T::Float, Value::Double(0.5)), Ok(Value::Float(0.5)));
        assert!(coerce(T::Float, Value::Double(1e300)).is_err());
        assert!(coerce(T::Boolean, Value::Int32(1)).is_err());
        assert_eq!(
            coerce(T::Guid, "3CD47E8B-E246-5A00-A657-68FDB442ACFB".into()),
            Ok(Value::String("3cd47e8b-e246-5a00-a657-68fdb442acfb".into()))
        );
        assert!(coerce(T::Guid, "x".into()).is_err());
        assert_eq!(
            coerce(T::LocalizedText, "Pumpe".into()),
            Ok(Value::LocalizedText(LocalizedText::from("Pumpe")))
        );
        assert!(coerce(T::DateTime, Value::DateTime(i64::MAX)).is_err());
    }

    #[test]
    fn values_encode_as_opc_ua_json() {
        use BuiltinType as T;
        assert_eq!(encode(T::Int64, &Value::Int64(-5)), json!("-5"));
        assert_eq!(encode(T::UInt64, &Value::UInt64(5)), json!("5"));
        assert_eq!(encode(T::Int16, &Value::Int64(-5)), json!(-5));
        assert_eq!(encode(T::Float, &Value::Float(0.1)), json!(0.1));
        assert_eq!(encode(T::Double, &Value::Double(f64::NAN)), json!("NaN"));
        assert_eq!(
            encode(T::Double, &Value::Double(f64::NEG_INFINITY)),
            json!("-Infinity")
        );
        assert_eq!(
            encode(T::DateTime, &Value::DateTime(1_639_693_510_094)),
            json!("2021-12-16T22:25:10.094Z")
        );
        assert_eq!(
            encode(T::ByteString, &Value::Bytes(b"OEC".to_vec())),
            json!("T0VD")
        );
        assert_eq!(
            encode(T::LocalizedText, &Value::LocalizedText("x".into())),
            json!({"Locale": "en-US", "Text": "x"})
        );
    }

    #[test]
    fn the_class_id_follows_the_structure() {
        let a = [FieldSpec::new("Pv", BuiltinType::Double)];
        let mut b = a.clone();
        b[0].description = Some("Temperature".into());
        b[0].unit = Some("°C".into());
        let id = class_id(OI4PV, &a);
        assert_eq!(id.get_version_num(), 5);
        assert_eq!(id, class_id(OI4PV, &b), "descriptions are no structure");
        assert_ne!(id, class_id("Other", &a));
        let c = [FieldSpec::new("Pv", BuiltinType::Float)];
        assert_ne!(id, class_id(OI4PV, &c));
        // pinned (cross-checked with Python's uuid.uuid5): a change breaks
        // REQ-MSG-013 for deployed applications
        assert_eq!(id.to_string(), "7d60caaf-6142-5766-b5c2-68226528ed79");
    }

    #[test]
    fn dead_band() {
        let last = [Value::Double(10.0), Value::String("a".into())];
        let p = [0.5, 0.0];
        assert!(!exceeds_dead_band(&last, &last, &p));
        assert!(!exceeds_dead_band(
            &last,
            &[Value::Double(10.4), Value::String("a".into())],
            &p
        ));
        assert!(exceeds_dead_band(
            &last,
            &[Value::Double(9.5), Value::String("a".into())],
            &p
        ));
        assert!(exceeds_dead_band(
            &last,
            &[Value::Double(10.0), Value::String("b".into())],
            &p
        ));
        assert!(exceeds_dead_band(
            &last,
            &[Value::Double(f64::NAN), Value::String("a".into())],
            &p
        ));
        assert!(!exceeds_dead_band(
            &[Value::Double(f64::NAN)],
            &[Value::Double(f64::NAN)],
            &[0.0]
        ));
    }

    #[test]
    fn definitions_are_checked() {
        let mut s = DataSetSpec::new("oee");
        assert!(s.check().is_err(), "no field");
        s.fields = vec![
            FieldSpec::new("A", BuiltinType::Double),
            FieldSpec::new("A", BuiltinType::Double),
        ];
        assert!(s.check().unwrap_err().contains("REQ-MSG-038"));
        s.fields.pop();
        s.publication.precisions.insert("B".into(), 1.0);
        assert!(s.check().is_err());
        s.publication.precisions = BTreeMap::from([("A".into(), -1.0)]);
        assert!(s.check().is_err());
        s.publication.precisions = BTreeMap::from([("A".into(), 1.0)]);
        assert_eq!(s.check(), Ok(()));
        s.class_id = Some(RESERVED_CLASS_IDS[0]);
        assert!(s.check().unwrap_err().contains("REQ-MSG-014"));
        let mut pv = DataSetSpec::new(OI4PV);
        pv.fields = vec![FieldSpec::new("Sv1", BuiltinType::Double)];
        assert!(pv.check().unwrap_err().contains("REQ-RES-DATA-004"));
        pv.fields
            .insert(0, FieldSpec::new("Pv", BuiltinType::Double));
        assert_eq!(pv.check(), Ok(()));
        pv.fields.push(FieldSpec::new("Sv01", BuiltinType::Double));
        assert!(pv.check().is_err());
    }
}
