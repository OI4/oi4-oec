//! Sans-IO protocol logic of the OI4 OEC v2 SDK (D2).
//!
//! Internal crate of the SDK crate `oec` (ADR-0011, ADR-0026): no I/O, no
//! runtime, no runtime dependencies and no stability promise of its own. The
//! public API is the one of `oec` (ADR-0020); `oec` re-exports or wraps the
//! types of this crate.
//!
//! | Module | Content | Guideline |
//! |---|---|---|
//! | [`il`] | Identification Link, dnp-encoding, Oi4Identifier | MG 4, MG A3 |
//! | [`topic`] | topic elements, allowed forms, Filter, topic parsing and building, subscription filters | MG 7, MG 9 |
//!
//! Every rule cites its requirement ids (`oec/spec/requirements/`); the
//! shared test vectors (`oec/spec/export/`) are the acceptance test
//! (`tests/vectors.rs`).

/// Defines a closed set of topic element values (REQ-TOP-005, REQ-TOP-011,
/// REQ-TOP-012, REQ-TOP-023) with `ALL`, `as_str`, `Display` and `FromStr`.
macro_rules! element {
    (
        $(#[$meta:meta])*
        $name:ident { $($(#[$vmeta:meta])* $variant:ident = $text:literal),+ $(,)? }
    ) => {
        $(#[$meta])*
        #[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, PartialOrd, Ord)]
        pub enum $name {
            $($(#[$vmeta])* $variant),+
        }

        impl $name {
            /// All values in the order of the guideline.
            pub const ALL: &'static [Self] = &[$(Self::$variant),+];

            /// The topic element (case-sensitive).
            pub const fn as_str(self) -> &'static str {
                match self {
                    $(Self::$variant => $text),+
                }
            }
        }

        impl core::fmt::Display for $name {
            fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
                f.write_str(self.as_str())
            }
        }

        impl core::str::FromStr for $name {
            type Err = $crate::topic::UnknownElement;

            fn from_str(s: &str) -> Result<Self, Self::Err> {
                match s {
                    $($text => Ok(Self::$variant),)+
                    _ => Err($crate::topic::UnknownElement { element: stringify!($name), value: s.to_owned() }),
                }
            }
        }
    };
}

mod base64url;
pub mod il;
pub mod topic;
