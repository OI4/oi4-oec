//! The `Address` of `broker.json` (REQ-CNT-003, SF-100 (b)): an IPv4
//! address in dotted-decimal form without leading zeros, an IPv6 address in
//! the text form of RFC 4291 (the `IPv6address` rule of RFC 3986 3.2.2,
//! without brackets and zone index) or an RFC 1123 host name whose last
//! label is not all digits. Same verdicts as the patterns of
//! `platform/broker.schema.json` and the reference check of the spec tools.

/// Whether `s` is a valid broker address.
pub(crate) fn valid(s: &str) -> bool {
    ipv4(s) || ipv6(s) || hostname(s)
}

/// `dec-octet "." dec-octet "." dec-octet "." dec-octet` without leading
/// zeros.
fn ipv4(s: &str) -> bool {
    let parts: Vec<&str> = s.split('.').collect();
    parts.len() == 4
        && parts.iter().all(|p| {
            !p.is_empty()
                && p.len() <= 3
                && p.bytes().all(|c| c.is_ascii_digit())
                && (p.len() == 1 || !p.starts_with('0'))
                && p.parse::<u16>().is_ok_and(|n| n <= 255)
        })
}

fn h16(s: &str) -> bool {
    (1..=4).contains(&s.len()) && s.bytes().all(|c| c.is_ascii_hexdigit())
}

/// RFC 3986 `IPv6address`: eight 16-bit groups, the last two may be an
/// IPv4 address; one `::` stands for at least one group of zeros.
fn ipv6(s: &str) -> bool {
    let (head, tail, compressed) = match s.split_once("::") {
        Some((h, t)) => (h, t, true),
        None => (s, "", false),
    };
    if compressed && tail.contains("::") {
        return false;
    }
    let groups = |part: &str| -> Vec<String> {
        if part.is_empty() {
            Vec::new()
        } else {
            part.split(':').map(str::to_owned).collect()
        }
    };
    let all: Vec<String> = groups(head).into_iter().chain(groups(tail)).collect();
    // only the group at the very end of the address may be an IPv4 address
    let ends_with_group = !compressed || !tail.is_empty();
    let mut count = 0;
    for (i, g) in all.iter().enumerate() {
        if ends_with_group && i + 1 == all.len() && g.contains('.') {
            if !ipv4(g) {
                return false;
            }
            count += 2;
        } else if h16(g) {
            count += 1;
        } else {
            return false;
        }
    }
    if compressed { count <= 7 } else { count == 8 }
}

/// RFC 1123 host name: labels of 1 … 63 letters, digits and `-` that do not
/// start or end with `-`, separated by `.`, at most 253 characters, no
/// trailing `.`, the last label not all digits.
fn hostname(s: &str) -> bool {
    s.len() <= 253
        && s.split('.').all(label)
        && !s
            .rsplit('.')
            .next()
            .is_some_and(|l| l.bytes().all(|c| c.is_ascii_digit()))
}

/// One RFC 1123 label (also the container name, REQ-CNT-019).
pub(crate) fn label(l: &str) -> bool {
    (1..=63).contains(&l.len())
        && l.bytes().all(|c| c.is_ascii_alphanumeric() || c == b'-')
        && !l.starts_with('-')
        && !l.ends_with('-')
}

#[cfg(test)]
mod tests {
    use super::valid;

    /// Addresses with their verdicts; with `strict-validation` also checked
    /// against the patterns of `platform/broker.schema.json`.
    const CORPUS: &[(&str, bool)] = &[
        ("broker", true),
        ("mqtt.example.com", true),
        ("a-b.c-d", true),
        ("x1.y2", true),
        ("1example.com", true),
        ("192.168.0.1", true),
        ("0.0.0.0", true),
        ("255.255.255.255", true),
        ("::", true),
        ("::1", true),
        ("fe80::1", true),
        ("2001:db8::8a2e:370:7334", true),
        ("2001:0db8:0000:0000:0000:ff00:0042:8329", true),
        ("1:2:3:4:5:6:7::", true),
        ("::2:3:4:5:6:7:8", true),
        ("::ffff:192.0.2.1", true),
        ("1:2:3:4:5:6:1.2.3.4", true),
        ("1::1.2.3.4", true),
        ("", false),
        (" broker", false),
        ("broker.", false),
        (".broker", false),
        ("a..b", false),
        ("-a.b", false),
        ("a-.b", false),
        ("a_b", false),
        ("1.2.3", false),
        ("1.2.3.256", false),
        ("01.2.3.4", false),
        ("1.2.3.4.5", false),
        ("example.123", false),
        ("broker:8883", false),
        ("mqtts://broker", false),
        ("broker/path", false),
        ("[::1]", false),
        ("fe80::1%eth0", false),
        (":::", false),
        ("1::2::3", false),
        ("1:2:3:4:5:6:7:8:9", false),
        ("1:2:3:4:5:6:7", false),
        ("1:2:3:4:5:6:7:8::", false),
        ("::1:2:3:4:5:6:7:8", false),
        ("12345::", false),
        ("g::", false),
        (":1::", false),
        ("1.2.3.4::", false),
        ("::1.2.3.4:1", false),
        ("1:2:3:4:5:6:7:1.2.3.4", false),
        ("::01.2.3.4", false),
    ];

    #[test]
    fn corpus() {
        for &(address, expected) in CORPUS {
            assert_eq!(valid(address), expected, "{address:?}");
        }
        let long = format!("{}.b", vec!["a".repeat(63); 4].join("."));
        assert_eq!(long.len(), 257);
        assert!(!valid(&long));
        let max = format!("{}.{}", vec!["a".repeat(63); 3].join("."), "b".repeat(61));
        assert_eq!(max.len(), 253);
        assert!(valid(&max));
        assert!(!valid(&"a".repeat(64)));
    }

    /// Same verdicts as the schema patterns (REQ-CNT-003, ADR-0019 §7).
    #[cfg(feature = "strict-validation")]
    #[test]
    fn corpus_matches_schema() {
        use oec_core::schema::{Flavor, Validator};
        let validator = Validator::new().expect("embedded schemas compile");
        let entry = validator
            .schema("platform/broker.schema.json")
            .expect("broker schema embedded");
        for &(address, expected) in CORPUS {
            let doc = serde_json::json!({
                "Address": address, "SecurePort": 8883, "MaxPacketSize": 262_144
            });
            let ok = validator.validate(&entry, Flavor::Strict, &doc).is_ok();
            assert_eq!(ok, expected, "{address:?}");
        }
    }
}
