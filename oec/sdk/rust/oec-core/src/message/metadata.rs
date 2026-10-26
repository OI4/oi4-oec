//! The DataSetMetaDataType and its sub-objects (MG 8.2.4 … 8.2.15).
//!
//! Mandatory fields are always written, also with their default value
//! (REQ-MSG-034); enumerations use the VerboseEncoding `<Name>_<Value>`.

use serde::{Deserialize, Deserializer, Serialize, Serializer};
use serde_json::{Map, Value};
use uuid::Uuid;

use super::guid;
use super::value::{ConfigurationVersion, LocalizedText};

/// DataSetMetaDataType (REQ-MSG-035).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct DataSetMetaData {
    /// Name of the DataSet; the resource name for a resource (MG B2).
    pub name: String,
    /// Description.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<LocalizedText>,
    /// The fields (REQ-MSG-037).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub fields: Option<Vec<FieldMetaData>>,
    /// The DataSetClassId (REQ-MSG-012, REQ-MSG-014).
    #[serde(default, skip_serializing_if = "Option::is_none", with = "guid::opt")]
    pub data_set_class_id: Option<Uuid>,
    /// ConfigurationVersion (REQ-MSG-036).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub configuration_version: Option<ConfigurationVersion>,
    /// Namespace URIs.
    pub namespaces: Vec<String>,
    /// Structured DataTypes (REQ-MSG-042).
    pub structure_data_types: Vec<StructureDescription>,
    /// Enumeration DataTypes (REQ-MSG-044).
    pub enum_data_types: Vec<EnumDescription>,
    /// Simple DataTypes (REQ-MSG-045).
    pub simple_data_types: Vec<SimpleTypeDescription>,
}

impl DataSetMetaData {
    /// A DataSetMetaDataType with the given name and empty mandatory arrays.
    pub fn new(name: impl Into<String>) -> Self {
        Self {
            name: name.into(),
            description: None,
            fields: None,
            data_set_class_id: None,
            configuration_version: None,
            namespaces: Vec::new(),
            structure_data_types: Vec::new(),
            enum_data_types: Vec::new(),
            simple_data_types: Vec::new(),
        }
    }
}

/// FieldMetaData (REQ-MSG-037, REQ-MSG-038).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct FieldMetaData {
    /// Name, unique in the DataSet (REQ-MSG-038).
    pub name: String,
    /// Description.
    pub description: LocalizedText,
    /// DataSetFieldFlags.
    pub field_flags: u16,
    /// BuiltInType id (0 … 25).
    pub built_in_type: u8,
    /// DataType NodeId (REQ-MSG-039).
    pub data_type: String,
    /// ValueRank.
    pub value_rank: i32,
    /// ArrayDimensions.
    pub array_dimensions: Vec<u32>,
    /// MaxStringLength.
    pub max_string_length: u32,
    /// DataSetFieldId.
    #[serde(with = "guid")]
    pub data_set_field_id: Uuid,
    /// Properties (REQ-MSG-041).
    pub properties: Vec<KeyValuePair>,
}

/// KeyValuePair of a property (REQ-MSG-041): the Value is a Variant in the
/// JSON encoding (`UaType`, `Value`, …).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct KeyValuePair {
    /// QualifiedName of the property.
    pub key: String,
    /// The Variant.
    pub value: Map<String, Value>,
}

/// StructureDescription (REQ-MSG-042).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct StructureDescription {
    /// DataTypeId NodeId.
    pub data_type_id: String,
    /// QualifiedName.
    pub name: String,
    /// The definition.
    pub structure_definition: StructureDefinition,
}

/// StructureDefinition (REQ-MSG-042).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct StructureDefinition {
    /// DefaultEncodingId NodeId.
    pub default_encoding_id: String,
    /// BaseDataType NodeId.
    pub base_data_type: String,
    /// StructureType.
    pub structure_type: StructureType,
    /// The fields (REQ-MSG-043).
    pub fields: Vec<StructureField>,
}

/// StructureType in the VerboseEncoding (REQ-MSG-034, REQ-MSG-042).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum StructureType {
    /// `Structure_0`.
    #[serde(rename = "Structure_0")]
    Structure,
    /// `StructureWithOptionalFields_1`.
    #[serde(rename = "StructureWithOptionalFields_1")]
    StructureWithOptionalFields,
    /// `Union_2`.
    #[serde(rename = "Union_2")]
    Union,
    /// `StructureWithSubtypedValues_3`.
    #[serde(rename = "StructureWithSubtypedValues_3")]
    StructureWithSubtypedValues,
    /// `UnionWithSubtypedValues_4`.
    #[serde(rename = "UnionWithSubtypedValues_4")]
    UnionWithSubtypedValues,
}

/// StructureField (REQ-MSG-043).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct StructureField {
    /// Name, unique in the structure.
    pub name: String,
    /// Description.
    pub description: LocalizedText,
    /// DataType NodeId.
    pub data_type: String,
    /// ValueRank (-1 or at least 1).
    pub value_rank: i32,
    /// ArrayDimensions.
    pub array_dimensions: Vec<u32>,
    /// MaxStringLength.
    pub max_string_length: u32,
    /// IsOptional.
    pub is_optional: bool,
}

/// EnumDescription (REQ-MSG-044).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct EnumDescription {
    /// DataTypeId NodeId.
    pub data_type_id: String,
    /// QualifiedName.
    pub name: String,
    /// The definition.
    pub enum_definition: EnumDefinition,
    /// BuiltInType of the values.
    pub built_in_type: u8,
}

/// EnumDefinition (REQ-MSG-044).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct EnumDefinition {
    /// The values.
    pub fields: Vec<EnumField>,
}

/// EnumField (REQ-MSG-044); the Int64 Value is a JSON string (Part 6
/// 5.4.2.3).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct EnumField {
    /// The value.
    #[serde(with = "int64")]
    pub value: i64,
    /// The name.
    pub name: String,
    /// DisplayName.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub display_name: Option<LocalizedText>,
    /// Description.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<LocalizedText>,
}

/// SimpleTypeDescription (REQ-MSG-045).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct SimpleTypeDescription {
    /// BaseDataType NodeId.
    pub base_data_type: String,
    /// BuiltInType id.
    pub built_in_type: u8,
    /// DataTypeId NodeId.
    pub data_type_id: String,
    /// QualifiedName.
    pub name: String,
}

mod int64 {
    use super::{Deserialize, Deserializer, Serializer};
    use serde::de::Error as _;

    #[allow(clippy::trivially_copy_pass_by_ref)]
    pub(super) fn serialize<S: Serializer>(v: &i64, s: S) -> Result<S::Ok, S::Error> {
        s.collect_str(v)
    }

    pub(super) fn deserialize<'de, D: Deserializer<'de>>(d: D) -> Result<i64, D::Error> {
        let s = String::deserialize(d)?;
        s.parse()
            .map_err(|_| D::Error::custom(format!("Int64 {s:?} is not a decimal string")))
    }
}
