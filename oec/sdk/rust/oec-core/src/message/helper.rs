//! The helper DataSets PaginationRequest, Pagination, Locale and Query
//! (MG 8.3; REQ-MSG-019).

use serde::{Deserialize, Serialize};
use serde_json::{Map, Value};

use super::envelope::{DataSet, MessageId};

/// PaginationRequest, DataSetWriterId 1 (REQ-PAG-001).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct PaginationRequest {
    /// Entries per page, at least 1.
    pub per_page: u32,
    /// The requested page.
    pub page: u32,
}

impl DataSet for PaginationRequest {
    const CLASS_ID: Option<uuid::Uuid> = None;
    const HELPER: Option<(u16, &'static str)> = Some((1, "PaginationRequest"));
}

/// Pagination, DataSetWriterId 2 (REQ-PAG-003).
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct Pagination {
    /// Number of all entries.
    pub total_count: u32,
    /// Entries per page.
    pub per_page: u32,
    /// This page.
    pub page: u32,
    /// Whether a further page follows.
    pub has_next: bool,
    /// Identifies the paginated result, formed like a MessageId
    /// (REQ-PAG-003).
    pub pagination_id: MessageId,
}

impl DataSet for Pagination {
    const CLASS_ID: Option<uuid::Uuid> = None;
    const HELPER: Option<(u16, &'static str)> = Some((2, "Pagination"));
}

/// Locale, DataSetWriterId 3 (REQ-LOC-001).
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct Locale {
    /// The requested locale, e.g. `de-DE`.
    pub locale: String,
}

impl DataSet for Locale {
    const CLASS_ID: Option<uuid::Uuid> = None;
    const HELPER: Option<(u16, &'static str)> = Some((3, "Locale"));
}

/// Query, DataSetWriterId 9 (REQ-QRY-001): an object whose fields the
/// resource defines (e.g.
/// [`ReferenceDesignationQuery`](crate::resource::ReferenceDesignationQuery)).
#[derive(Debug, Clone, PartialEq, Default, Serialize, Deserialize)]
#[serde(transparent)]
pub struct Query(pub Map<String, Value>);

impl DataSet for Query {
    const CLASS_ID: Option<uuid::Uuid> = None;
    const HELPER: Option<(u16, &'static str)> = Some((9, "Query"));
}
