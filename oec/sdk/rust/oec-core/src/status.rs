//! OPC UA StatusCodes (OPC UA Part 4 7.39, Part 6 A.2) as used in Status
//! events (MG 8.3.6.1; REQ-RES-EVENT-003, REQ-RES-EVENT-004).

use core::fmt;

use crate::topic::StatusLevel;

/// An OPC UA StatusCode: the `Number` of a Status event.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub struct StatusCode(pub u32);

/// The StatusCodes the provider uses, with symbolic id and description
/// (OPC UA Part 6 A.2, `StatusCode.csv`).
const KNOWN: [(StatusCode, &str, &str); 11] = [
    (StatusCode::GOOD, "Good", "The operation succeeded."),
    (StatusCode::BAD, "Bad", "The operation failed."),
    (
        StatusCode::BAD_INTERNAL_ERROR,
        "BadInternalError",
        "An internal error occurred as a result of a programming or configuration error.",
    ),
    (
        StatusCode::BAD_DECODING_ERROR,
        "BadDecodingError",
        "Decoding halted because of invalid data in the stream.",
    ),
    (
        StatusCode::BAD_USER_ACCESS_DENIED,
        "BadUserAccessDenied",
        "User does not have permission to perform the requested operation.",
    ),
    (
        StatusCode::BAD_NOT_WRITABLE,
        "BadNotWritable",
        "The access level does not allow writing to the Node.",
    ),
    (
        StatusCode::BAD_OUT_OF_RANGE,
        "BadOutOfRange",
        "The value was out of range.",
    ),
    (
        StatusCode::BAD_NOT_SUPPORTED,
        "BadNotSupported",
        "The requested operation is not supported.",
    ),
    (
        StatusCode::BAD_NOT_FOUND,
        "BadNotFound",
        "A requested item was not found or a search operation ended without success.",
    ),
    (
        StatusCode::BAD_TYPE_MISMATCH,
        "BadTypeMismatch",
        "The value supplied for the attribute is not of the same type as the attribute's value.",
    ),
    (
        StatusCode::BAD_INVALID_ARGUMENT,
        "BadInvalidArgument",
        "One or more arguments are invalid.",
    ),
];

impl StatusCode {
    /// `Good` (0x00000000).
    pub const GOOD: Self = Self(0);
    /// `Bad` (0x80000000).
    pub const BAD: Self = Self(0x8000_0000);
    /// `BadInternalError` (0x80020000).
    pub const BAD_INTERNAL_ERROR: Self = Self(0x8002_0000);
    /// `BadDecodingError` (0x80070000).
    pub const BAD_DECODING_ERROR: Self = Self(0x8007_0000);
    /// `BadUserAccessDenied` (0x801F0000).
    pub const BAD_USER_ACCESS_DENIED: Self = Self(0x801F_0000);
    /// `BadNotWritable` (0x803B0000).
    pub const BAD_NOT_WRITABLE: Self = Self(0x803B_0000);
    /// `BadOutOfRange` (0x803C0000).
    pub const BAD_OUT_OF_RANGE: Self = Self(0x803C_0000);
    /// `BadNotSupported` (0x803D0000).
    pub const BAD_NOT_SUPPORTED: Self = Self(0x803D_0000);
    /// `BadNotFound` (0x803E0000).
    pub const BAD_NOT_FOUND: Self = Self(0x803E_0000);
    /// `BadTypeMismatch` (0x80740000).
    pub const BAD_TYPE_MISMATCH: Self = Self(0x8074_0000);
    /// `BadInvalidArgument` (0x80AB0000).
    pub const BAD_INVALID_ARGUMENT: Self = Self(0x80AB_0000);

    /// The severity class (bits 30–31): the EventLevel of a Status event;
    /// the reserved class 3 is treated as Bad (REQ-RES-EVENT-004).
    pub const fn level(self) -> StatusLevel {
        match self.0 >> 30 {
            0 => StatusLevel::Good,
            1 => StatusLevel::Uncertain,
            _ => StatusLevel::Bad,
        }
    }

    /// Whether the severity class is Good.
    pub const fn is_good(self) -> bool {
        self.0 >> 30 == 0
    }

    /// The symbolic id (e.g. `BadNotFound`) of a code the provider knows;
    /// the `SymbolicId` of the Status event (REQ-RES-EVENT-003).
    pub fn symbolic_id(self) -> Option<&'static str> {
        KNOWN
            .iter()
            .find(|(c, _, _)| *c == self)
            .map(|(_, s, _)| *s)
    }

    /// The description of a code the provider knows; the `Description` of
    /// the Status event.
    pub fn description(self) -> Option<&'static str> {
        KNOWN
            .iter()
            .find(|(c, _, _)| *c == self)
            .map(|(_, _, d)| *d)
    }
}

impl fmt::Display for StatusCode {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self.symbolic_id() {
            Some(s) => write!(f, "{s} (0x{:08X})", self.0),
            None => write!(f, "0x{:08X}", self.0),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn severity_classes() {
        assert_eq!(StatusCode::GOOD.level(), StatusLevel::Good);
        assert_eq!(StatusCode(0x4000_0000).level(), StatusLevel::Uncertain);
        assert_eq!(StatusCode::BAD_NOT_FOUND.level(), StatusLevel::Bad);
        assert_eq!(StatusCode(0xC000_0000).level(), StatusLevel::Bad);
        assert!(StatusCode(0x0000_1234).is_good());
        assert!(!StatusCode::BAD.is_good());
    }

    #[test]
    fn symbolic_ids_start_with_the_class() {
        for (code, symbol, description) in KNOWN {
            assert!(symbol.starts_with(code.level().as_str()), "{symbol}");
            assert!(!description.is_empty());
        }
        assert_eq!(
            StatusCode::GOOD.description(),
            Some("The operation succeeded.")
        );
        assert_eq!(StatusCode(0x8034_0000).symbolic_id(), None);
        assert_eq!(
            StatusCode::BAD_NOT_FOUND.to_string(),
            "BadNotFound (0x803E0000)"
        );
    }
}
