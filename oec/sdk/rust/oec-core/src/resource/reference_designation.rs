//! ReferenceDesignation (MG 9.1.10) and its Query (MG 8.3.13.1).

use serde::{Deserialize, Serialize};
use serde_json::{Map, Value};

use crate::message::{DecodeError, Query};

/// The parent of a designation (REQ-RES-REFDES-001).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct DesignationParent {
    /// Value.
    pub value: String,
    /// Local.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub local: Option<String>,
    /// IdentificationLink of the parent asset.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub identification_link: Option<String>,
}

/// One aspect of a reference designation (REQ-RES-REFDES-001).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct Designation {
    /// Value.
    pub value: String,
    /// Local.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub local: Option<String>,
    /// Parent.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub parent: Option<DesignationParent>,
}

/// The ReferenceDesignation of an asset (REQ-RES-REFDES-001).
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct ReferenceDesignation {
    /// Function aspect.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub function: Option<Designation>,
    /// Product aspect.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub product: Option<Designation>,
    /// Location aspect.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub location: Option<Designation>,
}

super::data_set!(ReferenceDesignation, "27a75019-164a-496d-a38b-90e8a55c2cfa");

/// An aspect of a reference designation.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[allow(missing_docs)]
pub enum Aspect {
    Function,
    Product,
    Location,
}

/// The Query parameters of ReferenceDesignation (REQ-RES-REFDES-012):
/// the children of Parent, optionally of one aspect.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct ReferenceDesignationQuery {
    /// The unencoded IdentificationLink of the parent, not empty.
    pub parent: String,
    /// The aspect.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub aspect: Option<Aspect>,
}

impl ReferenceDesignationQuery {
    /// Whether `rd` is a direct child of [`Self::parent`]: the Parent of the
    /// requested aspect, without one of any aspect, has an
    /// IdentificationLink equal to it, compared character by character
    /// (REQ-RES-REFDES-014, REQ-IL-015). The responder selects with it;
    /// the requester checks the answer with it, as a responder without
    /// support for the Query answers with all objects (REQ-RES-REFDES-015,
    /// REQ-QRY-004).
    pub fn matches(&self, rd: &ReferenceDesignation) -> bool {
        let aspects = match self.aspect {
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
                == Some(self.parent.as_str())
        })
    }
}

impl TryFrom<&Query> for ReferenceDesignationQuery {
    type Error = DecodeError;

    fn try_from(q: &Query) -> Result<Self, DecodeError> {
        Ok(serde_json::from_value(Value::Object(q.0.clone()))?)
    }
}

impl From<ReferenceDesignationQuery> for Query {
    fn from(q: ReferenceDesignationQuery) -> Self {
        let mut m = Map::new();
        m.insert("Parent".to_owned(), Value::String(q.parent));
        if let Some(a) = q.aspect {
            m.insert(
                "Aspect".to_owned(),
                serde_json::to_value(a).unwrap_or(Value::Null),
            );
        }
        Self(m)
    }
}
