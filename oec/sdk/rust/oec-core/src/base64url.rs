//! base64url without padding (RFC 4648 §5), the topic encoding of an
//! Identification Link (MG 4.2). Decoding accepts only the canonical form
//! (REQ-IL-004).

const ALPHABET: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";

/// Why a string is not a canonical unpadded base64url encoding.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DecodeError {
    /// The string is empty.
    Empty,
    /// A character outside `A-Z a-z 0-9 - _`, e.g. `=`, `+`, `/` or a line
    /// break (REQ-IL-003, REQ-IL-004).
    Character(char),
    /// The length is 1 modulo 4: no octet sequence encodes to it.
    Length,
    /// The unused bits of the last character are not zero, so the string is
    /// not the canonical encoding of its octets.
    NonCanonical,
}

impl core::fmt::Display for DecodeError {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        match self {
            Self::Empty => f.write_str("empty topic element"),
            Self::Character(c) => write!(f, "character {c:?} is not in the base64url alphabet"),
            Self::Length => f.write_str("length 1 modulo 4"),
            Self::NonCanonical => f.write_str("unused bits of the last character are not zero"),
        }
    }
}

/// Encodes octets as base64url without padding and without line breaks
/// (REQ-IL-002, REQ-IL-003).
pub(crate) fn encode(octets: &[u8]) -> String {
    let mut out = String::with_capacity(octets.len().div_ceil(3) * 4);
    for chunk in octets.chunks(3) {
        let b = [
            chunk[0],
            *chunk.get(1).unwrap_or(&0),
            *chunk.get(2).unwrap_or(&0),
        ];
        let n = u32::from(b[0]) << 16 | u32::from(b[1]) << 8 | u32::from(b[2]);
        for i in 0..=chunk.len() {
            out.push(char::from(ALPHABET[(n >> (18 - 6 * i) & 0x3F) as usize]));
        }
    }
    out
}

fn value(c: u8) -> Option<u32> {
    let v = match c {
        b'A'..=b'Z' => c - b'A',
        b'a'..=b'z' => c - b'a' + 26,
        b'0'..=b'9' => c - b'0' + 52,
        b'-' => 62,
        b'_' => 63,
        _ => return None,
    };
    Some(u32::from(v))
}

/// Decodes the canonical unpadded base64url form; every other form is
/// rejected (REQ-IL-004: "exactly one valid encoded representation").
pub(crate) fn decode(s: &str) -> Result<Vec<u8>, DecodeError> {
    if s.is_empty() {
        return Err(DecodeError::Empty);
    }
    if let Some(c) = s
        .chars()
        .find(|&c| !c.is_ascii() || value(c as u8).is_none())
    {
        return Err(DecodeError::Character(c));
    }
    let bytes = s.as_bytes();
    if bytes.len() % 4 == 1 {
        return Err(DecodeError::Length);
    }
    let mut out = Vec::with_capacity(bytes.len() * 3 / 4);
    for chunk in bytes.chunks(4) {
        let mut n = 0u32;
        for (i, &c) in chunk.iter().enumerate() {
            n |= value(c).unwrap_or(0) << (18 - 6 * i);
        }
        let octets = chunk.len() - 1;
        let unused = match octets {
            1 => n & 0xFFFF,
            2 => n & 0xFF,
            _ => 0,
        };
        if unused != 0 {
            return Err(DecodeError::NonCanonical);
        }
        out.extend_from_slice(&n.to_be_bytes()[1..=octets]);
    }
    Ok(out)
}

#[cfg(test)]
mod tests {
    use super::*;

    // RFC 4648 §10 test vectors, translated to the URL alphabet without padding.
    #[test]
    fn rfc4648_vectors() {
        for (plain, enc) in [
            ("", ""),
            ("f", "Zg"),
            ("fo", "Zm8"),
            ("foo", "Zm9v"),
            ("foob", "Zm9vYg"),
            ("fooba", "Zm9vYmE"),
            ("foobar", "Zm9vYmFy"),
        ] {
            assert_eq!(encode(plain.as_bytes()), enc);
            if !enc.is_empty() {
                assert_eq!(decode(enc).unwrap(), plain.as_bytes());
            }
        }
    }

    // REQ-IL-003: the URL alphabet replaces "+" and "/".
    #[test]
    fn url_alphabet() {
        assert_eq!(encode(&[0xFB, 0xFF]), "-_8");
        assert_eq!(decode("-_8").unwrap(), [0xFB, 0xFF]);
    }

    // REQ-IL-004: only the canonical unpadded form is accepted.
    #[test]
    fn rejects_non_canonical_forms() {
        assert_eq!(decode(""), Err(DecodeError::Empty));
        assert_eq!(decode("Zg=="), Err(DecodeError::Character('=')));
        assert_eq!(decode("+/8"), Err(DecodeError::Character('+')));
        assert_eq!(decode("Zm9v\nYg"), Err(DecodeError::Character('\n')));
        assert_eq!(decode("Zm9vY"), Err(DecodeError::Length));
        assert_eq!(decode("Zh"), Err(DecodeError::NonCanonical));
        assert_eq!(decode("Zm9"), Err(DecodeError::NonCanonical));
        assert_eq!(decode("Zä"), Err(DecodeError::Character('ä')));
    }

    #[test]
    fn round_trip_all_octets() {
        let all: Vec<u8> = (0..=255).collect();
        for n in 0..all.len() {
            assert_eq!(decode(&encode(&all[..=n])).unwrap(), &all[..=n]);
        }
    }
}
