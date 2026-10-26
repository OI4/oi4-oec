//! Identification Link (MG 4.1, MG 4.2), dnp-encoding (MG A3) and
//! Oi4Identifier (MG 4.3).

use core::fmt;
use core::str::FromStr;

use crate::base64url;
pub use crate::base64url::DecodeError as EncodingError;

/// Maximum length of an Identification Link in characters (REQ-IL-019,
/// REQ-IL-029). A valid Identification Link is ASCII, so characters and
/// octets are the same.
pub const MAX_LENGTH: usize = 255;

/// Maximum length of the encoded topic element (REQ-IL-019).
pub const MAX_ENCODED_LENGTH: usize = 340;

/// Represents a missing Oi4Identifier element (REQ-IL-028).
pub const NO_DATA: &str = "nd";

/// One broken rule. [`Violation::code`] returns the code used by the shared
/// test vectors (`oec/spec/vectors/il/`).
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub enum Violation {
    /// The Identification Link is empty (REQ-IL-019).
    Empty,
    /// More than 255 characters (REQ-IL-019, REQ-IL-029).
    TooLong {
        /// Length in octets.
        length: usize,
    },
    /// Characters outside the IEC 61406-1 set, each listed once
    /// (REQ-IL-016).
    ForbiddenCharacters(Vec<char>),
    /// The mandatory authority is empty (REQ-IL-009).
    EmptyAuthority,
    /// The authority contains upper-case characters (REQ-IL-018).
    UppercaseAuthority,
    /// The topic element is not the canonical base64url encoding
    /// (REQ-IL-002 … REQ-IL-004).
    Encoding(EncodingError),
    /// The ManufacturerUri is not a lower-case domain name without scheme,
    /// path and `www.` (REQ-IL-024).
    ManufacturerUri,
    /// An Oi4Identifier element contains `/` and the dnp-encoding is off
    /// (REQ-IL-022, REQ-IL-033).
    SlashInElement {
        /// `Model`, `ProductCode` or `SerialNumber`.
        element: &'static str,
    },
}

impl Violation {
    /// Stable code of the violation, e.g. `forbidden-character`.
    pub fn code(&self) -> &'static str {
        match self {
            Self::Empty => "empty",
            Self::TooLong { .. } => "too-long",
            Self::ForbiddenCharacters(_) => "forbidden-character",
            Self::EmptyAuthority => "empty-authority",
            Self::UppercaseAuthority => "uppercase-authority",
            Self::Encoding(_) => "encoding",
            Self::ManufacturerUri => "manufacturer-uri",
            Self::SlashInElement { .. } => "slash-in-element",
        }
    }
}

impl fmt::Display for Violation {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Empty => f.write_str("the Identification Link is empty"),
            Self::TooLong { length } => write!(f, "{length} characters, maximum {MAX_LENGTH}"),
            Self::ForbiddenCharacters(chars) => {
                f.write_str("characters not permitted:")?;
                for c in chars {
                    write!(f, " {c:?}")?;
                }
                Ok(())
            }
            Self::EmptyAuthority => f.write_str("the authority is mandatory"),
            Self::UppercaseAuthority => f.write_str("the authority contains upper-case characters"),
            Self::Encoding(e) => write!(f, "not a canonical unpadded base64url string: {e}"),
            Self::ManufacturerUri => f.write_str(
                "the ManufacturerUri is not a domain name of the form <manufacturer>.<tld> without www.",
            ),
            Self::SlashInElement { element } => {
                write!(f, "{element} contains \"/\" (enable the dnp-encoding)")
            }
        }
    }
}

/// An invalid Identification Link or Oi4Identifier with all violations
/// found.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct IlError {
    violations: Vec<Violation>,
}

impl IlError {
    fn new(violations: Vec<Violation>) -> Self {
        debug_assert!(!violations.is_empty());
        Self { violations }
    }

    /// The violations, at least one.
    pub fn violations(&self) -> &[Violation] {
        &self.violations
    }
}

impl fmt::Display for IlError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str("invalid Identification Link: ")?;
        for (i, v) in self.violations.iter().enumerate() {
            if i > 0 {
                f.write_str("; ")?;
            }
            write!(f, "{v}")?;
        }
        Ok(())
    }
}

impl std::error::Error for IlError {}

/// Letters, digits, unreserved and reserved characters of IEC 61406-1
/// Annex A (REQ-IL-016).
fn allowed(c: char) -> bool {
    c.is_ascii_alphanumeric() || "-._~:/?#[]@!$&'()*+,;=".contains(c)
}

/// Length of the scheme including `://`, 0 without scheme (RFC 3986
/// `scheme = ALPHA *( ALPHA / DIGIT / "+" / "-" / "." )`).
fn scheme_len(s: &str) -> usize {
    let Some(end) = s.find("://") else { return 0 };
    let scheme = &s[..end];
    let mut chars = scheme.chars();
    let valid = chars.next().is_some_and(|c| c.is_ascii_alphabetic())
        && chars.all(|c| c.is_ascii_alphanumeric() || "+-.".contains(c));
    if valid { end + 3 } else { 0 }
}

/// The authority: the text after the optional scheme up to the first `/`,
/// `?` or `#` (REQ-IL-009).
fn authority(s: &str) -> &str {
    let rest = &s[scheme_len(s)..];
    rest.find(['/', '?', '#']).map_or(rest, |i| &rest[..i])
}

/// All machine-checkable violations of MG 4.1; empty means valid. Not
/// checkable: uniqueness, control of the domain and the use of reserved
/// characters "according to their function" (REQ-IL-010, REQ-IL-016).
fn violations(s: &str) -> Vec<Violation> {
    if s.is_empty() {
        return vec![Violation::Empty];
    }
    let mut out = Vec::new();
    if s.len() > MAX_LENGTH {
        out.push(Violation::TooLong { length: s.len() });
    }
    let mut bad: Vec<char> = Vec::new();
    for c in s.chars().filter(|&c| !allowed(c)) {
        if !bad.contains(&c) {
            bad.push(c);
        }
    }
    if !bad.is_empty() {
        out.push(Violation::ForbiddenCharacters(bad));
    }
    let a = authority(s);
    if a.is_empty() {
        out.push(Violation::EmptyAuthority);
    } else if a.to_lowercase() != a {
        out.push(Violation::UppercaseAuthority);
    }
    out
}

/// A validated Identification Link (IEC 61406-1, MG 4.1).
///
/// Validated on construction (REQ-IL-008, REQ-IL-009, REQ-IL-016,
/// REQ-IL-018, REQ-IL-019), never altered or normalized afterwards
/// (REQ-IL-013) and compared character by character (REQ-IL-015).
#[derive(Debug, Clone, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub struct IdentificationLink(String);

impl IdentificationLink {
    /// Validates an existing Identification Link, e.g. from a nameplate or
    /// an OPC UA ProductInstanceUri (REQ-IL-006, REQ-IL-031). The string is
    /// kept exactly as given (REQ-IL-013).
    ///
    /// # Errors
    ///
    /// All violations of MG 4.1 that can be checked by a machine.
    pub fn parse(s: &str) -> Result<Self, IlError> {
        let v = violations(s);
        if v.is_empty() {
            Ok(Self(s.to_owned()))
        } else {
            Err(IlError::new(v))
        }
    }

    /// Decodes and validates an AppId or Source topic element
    /// (REQ-IL-002 … REQ-IL-004, REQ-TOP-009, REQ-TOP-018).
    ///
    /// # Errors
    ///
    /// [`Violation::Encoding`] if the element is not the canonical unpadded
    /// base64url encoding, otherwise the violations of the decoded
    /// Identification Link (octets that are no UTF-8 count as forbidden
    /// characters).
    pub fn decode(element: &str) -> Result<Self, IlError> {
        let octets =
            base64url::decode(element).map_err(|e| IlError::new(vec![Violation::Encoding(e)]))?;
        match String::from_utf8(octets) {
            Ok(s) => Self::parse(&s),
            Err(e) => {
                // Every replacement character marks an invalid sequence; the
                // lossy string only serves the violation list.
                let lossy = String::from_utf8_lossy(e.as_bytes()).into_owned();
                Err(IlError::new(violations(&lossy)))
            }
        }
    }

    /// The topic element: base64url of the UTF-8 octets without padding
    /// (REQ-IL-002, REQ-IL-003).
    pub fn encode(&self) -> String {
        base64url::encode(self.0.as_bytes())
    }

    /// The Identification Link exactly as assigned, the payload form
    /// (REQ-IL-005, REQ-IL-013).
    pub fn as_str(&self) -> &str {
        &self.0
    }

    /// Consumes the link and returns its string.
    pub fn into_string(self) -> String {
        self.0
    }
}

impl fmt::Display for IdentificationLink {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl AsRef<str> for IdentificationLink {
    fn as_ref(&self) -> &str {
        &self.0
    }
}

impl FromStr for IdentificationLink {
    type Err = IlError;

    fn from_str(s: &str) -> Result<Self, IlError> {
        Self::parse(s)
    }
}

impl TryFrom<&str> for IdentificationLink {
    type Error = IlError;

    fn try_from(s: &str) -> Result<Self, IlError> {
        Self::parse(s)
    }
}

impl TryFrom<String> for IdentificationLink {
    type Error = IlError;

    fn try_from(s: String) -> Result<Self, IlError> {
        let v = violations(&s);
        if v.is_empty() {
            Ok(Self(s))
        } else {
            Err(IlError::new(v))
        }
    }
}

impl From<IdentificationLink> for String {
    fn from(il: IdentificationLink) -> Self {
        il.0
    }
}

/// The dnp-encoding (MG A3, REQ-IL-032): keeps `A-Z a-z 0-9 - . _ ~` and
/// writes every other octet of the UTF-8 representation as `,` followed by
/// two upper-case hex digits.
pub fn dnp_encode(s: &str) -> String {
    use fmt::Write as _;
    let mut out = String::with_capacity(s.len());
    for b in s.bytes() {
        if b.is_ascii_alphanumeric() || b"-._~".contains(&b) {
            out.push(char::from(b));
        } else {
            let _ = write!(out, ",{b:02X}");
        }
    }
    out
}

/// A ManufacturerUri is a lower-case domain name with at least two labels,
/// without scheme, port, path and a leading `www.` (REQ-IL-024).
fn valid_manufacturer_uri(s: &str) -> bool {
    let label = |l: &str| {
        let b = l.as_bytes();
        !b.is_empty()
            && b.iter()
                .all(|&c| c.is_ascii_lowercase() || c.is_ascii_digit() || c == b'-')
            && b[0] != b'-'
            && b[b.len() - 1] != b'-'
    };
    !s.starts_with("www.") && s.split('.').count() >= 2 && s.split('.').all(label)
}

/// Builder of an Oi4Identifier
/// `<ManufacturerUri>/<Model>/<ProductCode>/<SerialNumber>` (MG 4.3,
/// REQ-IL-007, REQ-IL-022 … REQ-IL-034).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Oi4Identifier {
    manufacturer_uri: String,
    model: String,
    product_code: String,
    serial_number: Option<String>,
    dnp_encode: bool,
}

impl Oi4Identifier {
    /// The type-level elements; `manufacturer_uri` is lower-cased
    /// (REQ-IL-023). For the LocalizedText Model, pass the text in `en-US`
    /// (REQ-IL-027).
    pub fn new(manufacturer_uri: &str, model: &str, product_code: &str) -> Self {
        Self {
            manufacturer_uri: manufacturer_uri.to_lowercase(),
            model: model.to_owned(),
            product_code: product_code.to_owned(),
            serial_number: None,
            dnp_encode: false,
        }
    }

    /// The instance-level element. Without it, or if empty, the element is
    /// `nd` (REQ-IL-028).
    #[must_use]
    pub fn serial_number(mut self, serial_number: &str) -> Self {
        self.serial_number = Some(serial_number.to_owned());
        self
    }

    /// Masks Model, ProductCode and SerialNumber with the dnp-encoding
    /// (REQ-IL-032, REQ-IL-033). Off by default: without it, a `/` or a
    /// character not permitted in an Identification Link is an error.
    ///
    /// SF-104 (open): the guideline makes the masking optional, so the same
    /// nameplate can give two Oi4Identifiers; the proposed reading is to
    /// always mask. The default may change with the decision.
    #[must_use]
    pub fn dnp_encoded(mut self) -> Self {
        self.dnp_encode = true;
        self
    }

    /// Builds the Identification Link.
    ///
    /// # Errors
    ///
    /// [`Violation::ManufacturerUri`] (REQ-IL-024); without dnp-encoding
    /// [`Violation::SlashInElement`]; otherwise the violations of the
    /// resulting Identification Link, e.g. more than 255 characters
    /// including the masking characters (REQ-IL-029) or characters not
    /// permitted (REQ-IL-016).
    pub fn build(&self) -> Result<IdentificationLink, IlError> {
        if !valid_manufacturer_uri(&self.manufacturer_uri) {
            return Err(IlError::new(vec![Violation::ManufacturerUri]));
        }
        let elements = [
            ("Model", self.model.as_str()),
            ("ProductCode", self.product_code.as_str()),
            ("SerialNumber", self.serial_number.as_deref().unwrap_or("")),
        ];
        let mut id = self.manufacturer_uri.clone();
        for (element, value) in elements {
            id.push('/');
            if value.is_empty() {
                id.push_str(NO_DATA);
            } else if self.dnp_encode {
                id.push_str(&dnp_encode(value));
            } else if value.contains('/') {
                return Err(IlError::new(vec![Violation::SlashInElement { element }]));
            } else {
                id.push_str(value);
            }
        }
        IdentificationLink::try_from(id)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn codes(r: Result<IdentificationLink, IlError>) -> Vec<&'static str> {
        r.unwrap_err()
            .violations()
            .iter()
            .map(Violation::code)
            .collect()
    }

    // REQ-IL-013, REQ-IL-015: kept as given, compared character by character.
    #[test]
    fn kept_exactly_and_compared_exactly() {
        let a = IdentificationLink::parse("https://company.id/A87BAD34H").unwrap();
        let b = IdentificationLink::parse("company.id/A87BAD34H").unwrap();
        assert_eq!(a.as_str(), "https://company.id/A87BAD34H");
        assert_ne!(a, b);
        assert_ne!(
            IdentificationLink::parse("company.id/a").unwrap(),
            IdentificationLink::parse("company.id/A").unwrap()
        );
    }

    // REQ-IL-009, REQ-IL-016, REQ-IL-018, REQ-IL-019: all violations at once.
    #[test]
    fn reports_every_violation() {
        assert_eq!(codes(IdentificationLink::parse("")), ["empty"]);
        assert_eq!(
            codes(IdentificationLink::parse("Company.id/A 1")),
            ["forbidden-character", "uppercase-authority"]
        );
        assert_eq!(
            codes(IdentificationLink::parse("https:///x")),
            ["empty-authority"]
        );
        let long = format!("example.com/{}", "a".repeat(244));
        assert_eq!(codes(IdentificationLink::parse(&long)), ["too-long"]);
        assert!(IdentificationLink::parse(&long[..255]).is_ok());
    }

    // The scheme is optional; an invalid scheme counts as authority.
    #[test]
    fn authority_after_optional_scheme() {
        assert_eq!(authority("https://a.b/c?d"), "a.b");
        assert_eq!(authority("a.b#x"), "a.b");
        assert_eq!(authority("1x://a.b"), "1x:");
        assert!(IdentificationLink::parse("https://Example.com/x").is_err());
        assert!(IdentificationLink::parse("https://example.com/X").is_ok());
    }

    // REQ-IL-002 … REQ-IL-004: encode/decode, non-UTF-8 octets.
    #[test]
    fn topic_element() {
        let il = IdentificationLink::parse("company.id/A87BAD34H").unwrap();
        assert_eq!(il.encode(), "Y29tcGFueS5pZC9BODdCQUQzNEg");
        assert_eq!(
            IdentificationLink::decode("Y29tcGFueS5pZC9BODdCQUQzNEg").unwrap(),
            il
        );
        assert_eq!(
            codes(IdentificationLink::decode("Y29tcGFueS5pZC9BODdCQUQzNEg=")),
            ["encoding"]
        );
        let non_utf8 = base64url::encode(b"a.b/\xFF");
        assert_eq!(
            codes(IdentificationLink::decode(&non_utf8)),
            ["forbidden-character"]
        );
    }

    // REQ-IL-032: MG A3 Table 18 and multi-octet characters.
    #[test]
    fn dnp_encoding() {
        assert_eq!(dnp_encode("ABC DEF"), "ABC,20DEF");
        assert_eq!(dnp_encode("a/b,c"), "a,2Fb,2Cc");
        assert_eq!(dnp_encode("ä"), ",C3,A4");
        assert_eq!(dnp_encode("Az09-._~"), "Az09-._~");
    }

    // REQ-IL-022, REQ-IL-023, REQ-IL-028, REQ-IL-033.
    #[test]
    fn oi4identifier() {
        let id = Oi4Identifier::new("Provider.com", "FieldDataService", "FDS-001")
            .serial_number("1200-0345")
            .build()
            .unwrap();
        assert_eq!(
            id.as_str(),
            "provider.com/FieldDataService/FDS-001/1200-0345"
        );
        let nd = Oi4Identifier::new("provider.com", "", "P").build().unwrap();
        assert_eq!(nd.as_str(), "provider.com/nd/P/nd");
        // Without dnp-encoding, "/" and forbidden characters are errors.
        let slash = Oi4Identifier::new("provider.com", "M", "123/4").serial_number("S");
        assert_eq!(codes(slash.build()), ["slash-in-element"]);
        assert_eq!(
            slash.dnp_encoded().build().unwrap().as_str(),
            "provider.com/M/123,2F4/S"
        );
        let space = Oi4Identifier::new("provider.com", "A B", "P");
        assert_eq!(codes(space.build()), ["forbidden-character"]);
        // Reserved characters are permitted in an IL and stay unmasked.
        let at = Oi4Identifier::new("provider.com", "A@B", "P").serial_number("S");
        assert_eq!(at.build().unwrap().as_str(), "provider.com/A@B/P/S");
    }

    // REQ-IL-024: domain name without scheme, port, path, www.
    #[test]
    fn manufacturer_uri() {
        for ok in ["provider.com", "id.provider.com", "a-1.de"] {
            assert!(valid_manufacturer_uri(ok), "{ok}");
        }
        for bad in [
            "www.provider.com",
            "provider",
            "provider.com/x",
            "https://provider.com",
            "provider.com:80",
            "-a.com",
            "a..com",
        ] {
            assert!(!valid_manufacturer_uri(bad), "{bad}");
        }
    }
}
