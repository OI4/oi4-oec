//! Value types of the OPC UA JSON encoding used by OEC messages (MG 8,
//! OPC UA Part 6 5.4).

use core::fmt;
use core::str::FromStr;

use serde::{Deserialize, Serialize};
use time::OffsetDateTime;
use time::format_description::well_known::Rfc3339;

use super::DecodeError;

/// An OPC UA DateTime as RFC 3339 string (MG 8, Part 6 5.4.2.6).
///
/// The text is kept as received, so a decoded message encodes to the same
/// string (fraction digits and offset form included); the instant is
/// available as [`OffsetDateTime`]. Parsing rejects dates that do not exist
/// in the calendar (e.g. 30 February), which the schema pattern cannot see
/// (rule layer R-4, ADR-0019 §5).
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct DateTime {
    text: String,
    at: OffsetDateTime,
}

impl DateTime {
    /// The instant.
    pub fn at(&self) -> OffsetDateTime {
        self.at
    }

    /// The RFC 3339 text.
    pub fn as_str(&self) -> &str {
        &self.text
    }
}

impl From<OffsetDateTime> for DateTime {
    /// Formats the instant in UTC with millisecond precision
    /// (`2026-10-26T08:15:00.000Z`).
    fn from(at: OffsetDateTime) -> Self {
        let utc = at.to_offset(time::UtcOffset::UTC);
        let text = format!(
            "{:04}-{:02}-{:02}T{:02}:{:02}:{:02}.{:03}Z",
            utc.year(),
            u8::from(utc.month()),
            utc.day(),
            utc.hour(),
            utc.minute(),
            utc.second(),
            utc.millisecond()
        );
        Self { text, at }
    }
}

impl FromStr for DateTime {
    type Err = DecodeError;

    fn from_str(s: &str) -> Result<Self, DecodeError> {
        // the schema form: upper-case T and Z, no leap second
        let well_formed = s.len() >= 20
            && s.as_bytes()[10] == b'T'
            && !s.contains(['t', 'z'])
            && s.get(17..19).is_some_and(|sec| sec < "60");
        if !well_formed {
            return Err(DecodeError::new(format!("DateTime {s:?} is not RFC 3339")));
        }
        let at = OffsetDateTime::parse(s, &Rfc3339)
            .map_err(|e| DecodeError::new(format!("DateTime {s:?}: {e}")))?;
        Ok(Self {
            text: s.to_owned(),
            at,
        })
    }
}

impl fmt::Display for DateTime {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.text)
    }
}

serde_str!(DateTime);

/// LocalizedText (MG 8, Part 6 5.4.2.14; REQ-MSG-040).
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct LocalizedText {
    /// Locale, e.g. `en-US`.
    pub locale: String,
    /// The text.
    pub text: String,
}

impl LocalizedText {
    /// A text in a locale.
    pub fn new(locale: impl Into<String>, text: impl Into<String>) -> Self {
        Self {
            locale: locale.into(),
            text: text.into(),
        }
    }
}

/// ConfigurationVersion of a DataSetMetaData (REQ-MSG-036).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct ConfigurationVersion {
    /// Changes when the structure changes incompatibly.
    pub major_version: u32,
    /// Changes with every change of the DataSetMetaData.
    pub minor_version: u32,
}

/// A base64 ByteString with padding (RFC 4648 §4), e.g. the CorrelationData
/// of an Action (REQ-ACT-002). The text is kept as received, so it is
/// returned unchanged (REQ-ACT-007).
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct ByteString(String);

const B64: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

impl ByteString {
    /// Encodes octets.
    pub fn from_bytes(octets: &[u8]) -> Self {
        let mut out = String::with_capacity(octets.len().div_ceil(3) * 4);
        for chunk in octets.chunks(3) {
            let b = [
                chunk[0],
                *chunk.get(1).unwrap_or(&0),
                *chunk.get(2).unwrap_or(&0),
            ];
            let n = (u32::from(b[0]) << 16) | (u32::from(b[1]) << 8) | u32::from(b[2]);
            for i in 0..4 {
                if i <= chunk.len() {
                    out.push(char::from(B64[(n >> (18 - 6 * i)) as usize & 63]));
                } else {
                    out.push('=');
                }
            }
        }
        Self(out)
    }

    /// The octets.
    pub fn to_bytes(&self) -> Vec<u8> {
        let digits: Vec<u32> = self
            .0
            .bytes()
            .filter(|&c| c != b'=')
            .map(|c| {
                B64.iter()
                    .position(|&x| x == c)
                    .and_then(|p| u32::try_from(p).ok())
                    .unwrap_or(0)
            })
            .collect();
        let mut out = Vec::with_capacity(digits.len() * 3 / 4);
        for chunk in digits.chunks(4) {
            let n = chunk
                .iter()
                .zip([18, 12, 6, 0])
                .fold(0u32, |n, (d, shift)| n | (d << shift));
            out.extend_from_slice(&n.to_be_bytes()[1..chunk.len()]);
        }
        out
    }

    /// The base64 text.
    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl FromStr for ByteString {
    type Err = DecodeError;

    fn from_str(s: &str) -> Result<Self, DecodeError> {
        let body = s.trim_end_matches('=');
        let pad = s.len() - body.len();
        let ok = s.len() % 4 == 0
            && pad <= 2
            && (pad == 0 || body.len() % 4 == 4 - pad)
            && body.bytes().all(|c| B64.contains(&c));
        if ok {
            Ok(Self(s.to_owned()))
        } else {
            Err(DecodeError::new(format!(
                "{s:?} is not base64 with padding"
            )))
        }
    }
}

impl fmt::Display for ByteString {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

serde_str!(ByteString);

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn byte_string_round_trip() {
        for (bytes, text) in [
            (&b""[..], ""),
            (b"f", "Zg=="),
            (b"fo", "Zm8="),
            (b"foo", "Zm9v"),
            (b"1639693510000", "MTYzOTY5MzUxMDAwMA=="),
        ] {
            assert_eq!(ByteString::from_bytes(bytes).as_str(), text);
            let b: ByteString = text.parse().unwrap();
            assert_eq!(b.to_bytes(), bytes);
        }
        for bad in ["Zg=", "Z===", "Zm9v-A==", "Zm8"] {
            assert!(bad.parse::<ByteString>().is_err(), "{bad}");
        }
    }

    #[test]
    fn date_time() {
        for ok in [
            "2026-10-26T08:15:00.000Z",
            "2026-10-26T08:15:00Z",
            "2026-10-26T10:15:00.123456+02:00",
        ] {
            assert_eq!(ok.parse::<DateTime>().unwrap().as_str(), ok);
        }
        for bad in [
            "2023-02-30T00:00:00Z",
            "2026-10-26 08:15:00Z",
            "2026-10-26t08:15:00Z",
            "2026-10-26T23:59:60Z",
            "2026-10-26",
        ] {
            assert!(bad.parse::<DateTime>().is_err(), "{bad}");
        }
        let t = DateTime::from(OffsetDateTime::UNIX_EPOCH + time::Duration::milliseconds(1500));
        assert_eq!(t.as_str(), "1970-01-01T00:00:01.500Z");
    }
}
