//! Protocol status codes used in Status events and Action responses.

/// OPC UA StatusCode (MG 8.3.6.1) returned by handlers. The SDK maps it to
/// the Status event `Number`, `SymbolicId` and the topic level
/// (REQ-RES-EVENT-003, REQ-RES-EVENT-004). Separate from [`crate::Error`]
/// (ADR-0020 §14).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
#[non_exhaustive]
pub enum StatusCode {
    /// `Good` (0x00000000).
    Good,
    /// `Bad` (0x80000000).
    Bad,
    /// `BadNotFound` (0x803E0000).
    BadNotFound,
    /// `BadNotWritable` (0x803B0000).
    BadNotWritable,
    /// `BadTypeMismatch` (0x80740000).
    BadTypeMismatch,
    /// `BadOutOfRange` (0x803C0000).
    BadOutOfRange,
    /// `BadInvalidArgument` (0x80AB0000).
    BadInvalidArgument,
    /// `BadUserAccessDenied` (0x801F0000).
    BadUserAccessDenied,
    /// `BadInternalError` (0x80020000).
    BadInternalError,
    /// Any other OPC UA StatusCode by its numeric value.
    Other(u32),
}

impl StatusCode {
    /// The numeric OPC UA StatusCode (OPC UA Part 4 7.39, Part 6 A.2), the
    /// Status event `Number` (REQ-RES-EVENT-003).
    pub const fn code(self) -> u32 {
        match self {
            Self::Good => 0,
            Self::Bad => 0x8000_0000,
            Self::BadNotFound => 0x803E_0000,
            Self::BadNotWritable => 0x803B_0000,
            Self::BadTypeMismatch => 0x8074_0000,
            Self::BadOutOfRange => 0x803C_0000,
            Self::BadInvalidArgument => 0x80AB_0000,
            Self::BadUserAccessDenied => 0x801F_0000,
            Self::BadInternalError => 0x8002_0000,
            Self::Other(code) => code,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::StatusCode;

    #[test]
    fn codes() {
        assert_eq!(StatusCode::Good.code(), 0);
        assert_eq!(StatusCode::BadNotFound.code(), 0x803E_0000);
        assert_eq!(StatusCode::Other(0x8034_0000).code(), 0x8034_0000);
    }
}
