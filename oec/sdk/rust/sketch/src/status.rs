//! Protocol status codes used in Status events and Action responses.

/// OPC UA StatusCode (MG 8.3.6.1) returned by handlers. The SDK maps it to
/// the Status event `Number`, `SymbolicId` and the topic level
/// (REQ-RES-EVENT-003, REQ-RES-EVENT-004).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[non_exhaustive]
pub enum StatusCode {
    Good,
    Bad,
    BadNotFound,
    BadNotWritable,
    BadTypeMismatch,
    BadOutOfRange,
    BadInvalidArgument,
    BadUserAccessDenied,
    BadInternalError,
    /// Any other OPC UA StatusCode by its numeric value.
    Other(u32),
}
