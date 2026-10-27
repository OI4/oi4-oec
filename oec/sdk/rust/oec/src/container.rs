//! The container storage contract of the RIG (ADR-0011, ADR-0020 §5): the
//! Message Bus storage `/etc/oi4/mqtt`, the OI4 certificate storage
//! `/etc/oi4/certs`, the secret storage `/run/secrets/oi4`, the container
//! name from the hostname and the application specific volume
//! `/etc/oi4/app`. All paths are below a root (`/` in a container).

use std::io;
use std::path::{Path, PathBuf};

use serde_json::Value;

use crate::app::{Auth, Secret};
use crate::{Error, Result};

/// Bounds of MaxPacketSize (RIG 4.1.1, REQ-CNT-003).
pub(crate) const MIN_PACKET_SIZE: usize = 262_144;
pub(crate) const MAX_PACKET_SIZE: usize = 268_435_455;

/// The fields of `broker.json` (REQ-CNT-003).
#[derive(Debug)]
pub(crate) struct BrokerJson {
    pub(crate) address: String,
    pub(crate) secure_port: u16,
    pub(crate) max_packet_size: usize,
}

/// Everything the container storage provides.
#[derive(Debug)]
pub(crate) struct Container {
    pub(crate) name: String,
    pub(crate) broker: BrokerJson,
    pub(crate) ca: PathBuf,
    pub(crate) auth: Auth,
    /// `<root>/etc/oi4/app/oec` if the platform provides the persistent
    /// layer (REQ-CNT-010, REQ-CNT-011).
    pub(crate) store_dir: Option<PathBuf>,
}

/// Parses and checks `broker.json` (REQ-CNT-003); elements it does not
/// know are ignored (REQ-CNT-004). The error lists every problem.
pub(crate) fn parse_broker_json(text: &str) -> std::result::Result<BrokerJson, String> {
    let doc: Value =
        serde_json::from_str(text).map_err(|e| format!("broker.json is no JSON: {e}"))?;
    let Value::Object(m) = doc else {
        return Err("broker.json must be a JSON object (REQ-CNT-003)".to_owned());
    };
    let mut problems = Vec::new();
    for k in ["Address", "SecurePort", "MaxPacketSize"] {
        if !m.contains_key(k) {
            problems.push(format!("missing mandatory element {k}"));
        }
    }
    let address = match m.get("Address") {
        Some(Value::String(s)) if crate::address::valid(s) => s.clone(),
        Some(Value::String(s)) => {
            problems.push(format!("Address {s:?} is no IP address or host name"));
            String::new()
        }
        Some(_) => {
            problems.push("Address must be a string".to_owned());
            String::new()
        }
        None => String::new(),
    };
    let integer = |k: &str, min: u64, max: u64| -> Option<u64> {
        m.get(k)?.as_u64().filter(|n| (min..=max).contains(n))
    };
    let secure_port = integer("SecurePort", 1, 65_535);
    if m.contains_key("SecurePort") && secure_port.is_none() {
        problems.push(format!(
            "SecurePort must be an integer 1 … 65535, got {}",
            m["SecurePort"]
        ));
    }
    let max_packet_size = integer(
        "MaxPacketSize",
        MIN_PACKET_SIZE as u64,
        MAX_PACKET_SIZE as u64,
    );
    if m.contains_key("MaxPacketSize") && max_packet_size.is_none() {
        problems.push(format!(
            "MaxPacketSize must be an integer {MIN_PACKET_SIZE} … {MAX_PACKET_SIZE}, got {}",
            m["MaxPacketSize"]
        ));
    }
    for k in m.keys() {
        if !matches!(k.as_str(), "Address" | "SecurePort" | "MaxPacketSize") {
            log::debug!("broker.json: ignoring unknown element {k:?} (REQ-CNT-004)");
        }
    }
    match (secure_port, max_packet_size) {
        (Some(port), Some(size)) if problems.is_empty() => Ok(BrokerJson {
            address,
            secure_port: u16::try_from(port).unwrap_or_default(),
            max_packet_size: usize::try_from(size).unwrap_or_default(),
        }),
        _ => Err(format!("{} (REQ-CNT-003)", problems.join("; "))),
    }
}

/// Reads the container storage below `root`.
pub(crate) fn load(root: &Path) -> Result<Container> {
    let name = container_name(root)?;
    let broker_path = root.join("etc/oi4/mqtt/broker.json");
    let text = std::fs::read_to_string(&broker_path).map_err(|e| {
        Error::Bootstrap(format!(
            "cannot read {} (REQ-CNT-004): {e}",
            broker_path.display()
        ))
    })?;
    let broker = parse_broker_json(&text)
        .map_err(|e| Error::Bootstrap(format!("{}: {e}", broker_path.display())))?;
    log::debug!("broker.json: {broker:?}");
    let ca = ca(root)?;
    let auth = auth(root, &name)?;
    let app_volume = root.join("etc/oi4/app");
    let store_dir = app_volume.is_dir().then(|| app_volume.join("oec"));
    Ok(Container {
        name,
        broker,
        ca,
        auth,
        store_dir,
    })
}

/// The container name from the hostname (REQ-CNT-019, REQ-CNT-021).
fn container_name(root: &Path) -> Result<String> {
    let files = [
        root.join("proc/sys/kernel/hostname"),
        root.join("etc/hostname"),
    ];
    let Some(text) = files.iter().find_map(|f| std::fs::read_to_string(f).ok()) else {
        return Err(Error::Bootstrap(format!(
            "no hostname in {} or {} (REQ-CNT-021)",
            files[0].display(),
            files[1].display()
        )));
    };
    let name = text.trim();
    if !crate::address::label(name) {
        return Err(Error::Bootstrap(format!(
            "the hostname {name:?} is no container name: letters, digits and \"-\", \
             at most 63 characters (REQ-CNT-019, REQ-CNT-021)"
        )));
    }
    Ok(name.to_owned())
}

/// `broker_ca.pem` of the Message Bus storage, else `ca.pem` of the OI4
/// certificate storage (REQ-CNT-004, REQ-CNT-005, REQ-MQTT-008).
fn ca(root: &Path) -> Result<PathBuf> {
    let broker_ca = root.join("etc/oi4/mqtt/broker_ca.pem");
    let ca = root.join("etc/oi4/certs/ca.pem");
    if broker_ca.is_file() {
        Ok(broker_ca)
    } else if ca.is_file() {
        log::debug!(
            "no {}, verifying the broker with {} (REQ-CNT-004)",
            broker_ca.display(),
            ca.display()
        );
        Ok(ca)
    } else {
        Err(Error::Bootstrap(format!(
            "neither {} nor {} (REQ-CNT-004, REQ-CNT-005)",
            broker_ca.display(),
            ca.display()
        )))
    }
}

/// The means of authentication of the secret storage (REQ-CNT-008,
/// REQ-CNT-009); a client certificate wins over credentials (REQ-MQTT-010).
pub(crate) fn auth(root: &Path, name: &str) -> Result<Auth> {
    let secrets = root.join("run/secrets/oi4");
    let credentials = read_optional(&secrets.join("credentials"))?
        .map(|raw| credentials(&raw))
        .transpose()?;
    let key = secrets.join("private_key.pem");
    let cert = root.join(format!("etc/oi4/certs/{name}.pem"));
    let passphrase = read_optional(&secrets.join("passphrase"))?
        .map(|raw| {
            decode_base64(&raw).map(Secret).ok_or_else(|| {
                Error::Bootstrap("passphrase is not base64 (REQ-CNT-008)".to_owned())
            })
        })
        .transpose()?;
    match (key.is_file(), cert.is_file(), credentials) {
        (true, true, credentials) => {
            if credentials.is_some() {
                log::info!(
                    "client certificate {} wins over the credentials (REQ-MQTT-010)",
                    cert.display()
                );
            }
            Ok(Auth::ClientCertificate {
                cert,
                key,
                passphrase,
            })
        }
        (key_present, _, Some(credentials)) => {
            if key_present {
                log::warn!(
                    "private_key.pem without certificate {}; using the credentials (REQ-CNT-008)",
                    cert.display()
                );
            }
            Ok(credentials)
        }
        (true, false, None) => Err(Error::Bootstrap(format!(
            "private_key.pem without certificate {} (REQ-CNT-008, REQ-CNT-009)",
            cert.display()
        ))),
        (false, _, None) => Err(Error::Bootstrap(format!(
            "no means of authentication in {}: neither credentials nor private_key.pem \
             (REQ-CNT-008)",
            secrets.display()
        ))),
    }
}

fn read_optional(path: &Path) -> Result<Option<Vec<u8>>> {
    match std::fs::read(path) {
        Ok(v) => Ok(Some(v)),
        Err(e) if e.kind() == io::ErrorKind::NotFound => Ok(None),
        Err(e) => Err(Error::Bootstrap(format!(
            "cannot read {}: {e}",
            path.display()
        ))),
    }
}

/// `base64(<username>:<password>)`, split at the first `:` (REQ-CNT-009).
fn credentials(raw: &[u8]) -> Result<Auth> {
    let invalid = |what: &str| {
        Error::Bootstrap(format!(
            "credentials {what}; expected base64(<username>:<password>) (REQ-CNT-008, REQ-CNT-009)"
        ))
    };
    let decoded = decode_base64(raw).ok_or_else(|| invalid("are not base64"))?;
    let colon = decoded
        .iter()
        .position(|&c| c == b':')
        .ok_or_else(|| invalid("contain no \":\""))?;
    let username = String::from_utf8(decoded[..colon].to_vec())
        .map_err(|_| invalid("have a username that is not UTF-8"))?;
    Ok(Auth::Credentials {
        username,
        password: Secret(decoded[colon + 1..].to_vec()),
    })
}

/// Standard base64 (RFC 4648 §4) with optional padding; surrounding white
/// space (e.g. a final line break) is ignored.
pub(crate) fn decode_base64(raw: &[u8]) -> Option<Vec<u8>> {
    let text = raw.trim_ascii();
    let body = text
        .strip_suffix(b"==")
        .or_else(|| text.strip_suffix(b"="))
        .unwrap_or(text);
    if body.len() % 4 == 1 || (body.len() != text.len() && text.len() % 4 != 0) {
        return None;
    }
    let digit = |c: u8| -> Option<u32> {
        match c {
            b'A'..=b'Z' => Some(u32::from(c - b'A')),
            b'a'..=b'z' => Some(u32::from(c - b'a') + 26),
            b'0'..=b'9' => Some(u32::from(c - b'0') + 52),
            b'+' => Some(62),
            b'/' => Some(63),
            _ => None,
        }
    };
    let mut out = Vec::with_capacity(body.len() * 3 / 4);
    for chunk in body.chunks(4) {
        let mut n = 0u32;
        for (i, &c) in chunk.iter().enumerate() {
            n |= digit(c)? << (18 - 6 * i);
        }
        let bytes = n.to_be_bytes();
        let take = chunk.len() - 1;
        // the unused bits of a short final chunk are zero (canonical form)
        if bytes[1 + take..].iter().any(|&b| b != 0) {
            return None;
        }
        out.extend_from_slice(&bytes[1..=take]);
    }
    Some(out)
}

#[cfg(test)]
mod tests {
    use super::{decode_base64, parse_broker_json};

    #[test]
    fn base64() {
        assert_eq!(decode_base64(b"dXNlcjpwYXNz").unwrap(), b"user:pass");
        assert_eq!(decode_base64(b"dXNlcjpwYXNzMQ==\n").unwrap(), b"user:pass1");
        assert_eq!(decode_base64(b"dXNlcjpwYXNzMQ").unwrap(), b"user:pass1");
        assert_eq!(decode_base64(b"dXNlcjpwYXNzMTI=").unwrap(), b"user:pass12");
        assert_eq!(decode_base64(b"").unwrap(), b"");
        for bad in [
            &b"dXNlcjpwYXNzMQ="[..],
            b"dXNl*jpw",
            b"d",
            b"dXNlcjpwYXNzMR==",
            b"dX=l",
        ] {
            assert!(
                decode_base64(bad).is_none(),
                "{:?}",
                String::from_utf8_lossy(bad)
            );
        }
    }

    #[test]
    fn broker_json() {
        let ok = parse_broker_json(
            r#"{"Address":"broker","SecurePort":8883,"MaxPacketSize":262144,"Extra":1}"#,
        )
        .unwrap();
        assert_eq!(ok.address, "broker");
        assert_eq!(ok.secure_port, 8883);
        assert_eq!(ok.max_packet_size, 262_144);
        let e = parse_broker_json(r#"{"Address":"a b","SecurePort":8883.0}"#).unwrap_err();
        assert!(e.contains("Address \"a b\""), "{e}");
        assert!(e.contains("SecurePort must be an integer"), "{e}");
        assert!(e.contains("missing mandatory element MaxPacketSize"), "{e}");
        assert!(e.ends_with("(REQ-CNT-003)"), "{e}");
        assert!(parse_broker_json("[]").unwrap_err().contains("JSON object"));
        assert!(parse_broker_json("{").unwrap_err().contains("no JSON"));
    }
}
