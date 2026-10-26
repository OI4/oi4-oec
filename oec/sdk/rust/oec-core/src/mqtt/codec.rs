//! Encoder and incremental decoder of the MQTT 3.1.1 control packets
//! (REQ-MQTT-001), sans-IO.
//!
//! The codec is symmetric and canonical: [`encode`] and [`decode`] apply
//! the same rules, and every byte sequence [`decode`] accepts re-encodes to
//! exactly the same bytes (minimal Remaining Length, no reserved bits, no
//! trailing bytes). Every refusal is a structured [`CodecError`] that names
//! the rule of the MQTT 3.1.1 standard (normative statement `MQTT-x.y.z-n`
//! or section) or the OEC requirement it enforces.

use core::fmt;

use super::packet::{
    ConnAck, Connect, ConnectReturnCode, Delivery, LastWill, Packet, PacketId, PacketType, Publish,
    QoS, SubAck, Subscribe, SubscribeReturnCode, Unsubscribe,
};

/// The largest Remaining Length of MQTT 3.1.1 (MQTT 2.2.3), also the
/// upper bound of the MaxPacketSize (RIG 4.1.1).
pub const MAX_REMAINING_LENGTH: usize = 268_435_455;

/// The largest UTF-8 string or binary field (two-byte length, MQTT 1.5.3).
pub const MAX_FIELD_LENGTH: usize = 65_535;

/// A packet that can not be encoded or a byte sequence that is not a valid
/// packet. After a decoding error the byte stream is unusable: the
/// connection is closed (MQTT 4.8).
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub enum CodecError {
    /// The Remaining Length uses more than four bytes or more bytes than
    /// needed (MQTT 2.2.3).
    MalformedRemainingLength,
    /// The Remaining Length exceeds the limit: of the decoder (derived from
    /// the MaxPacketSize, RIG 4.1.1) or of MQTT ([`MAX_REMAINING_LENGTH`]).
    PacketTooLarge {
        /// Remaining Length of the packet.
        remaining_length: usize,
        /// The limit.
        limit: usize,
    },
    /// Packet type 0 or 15 (MQTT 2.2.1).
    ReservedPacketType(u8),
    /// A valid MQTT 3.1.1 feature the SDK does not support: QoS 2, Will
    /// Retain (REQ-MQTT-017, REQ-MQTT-018), another protocol level.
    Unsupported {
        /// The packet.
        packet: PacketType,
        /// The feature.
        feature: &'static str,
    },
    /// A rule of MQTT 3.1.1 is broken.
    Malformed {
        /// The packet.
        packet: PacketType,
        /// The rule: a normative statement (`MQTT-3.1.2-22`) or a section
        /// (`MQTT 3.2.2.3`).
        rule: &'static str,
        /// What is wrong.
        problem: &'static str,
    },
}

impl CodecError {
    /// The rule of MQTT 3.1.1 or the OEC requirement the error enforces.
    pub fn rule(&self) -> &'static str {
        match self {
            Self::MalformedRemainingLength | Self::PacketTooLarge { .. } => "MQTT 2.2.3",
            Self::ReservedPacketType(_) => "MQTT 2.2.1",
            Self::Unsupported { feature, .. } => {
                if feature.contains("Retain") {
                    "REQ-MQTT-018"
                } else if feature.contains("QoS 2") {
                    "REQ-MQTT-017"
                } else {
                    "REQ-MQTT-001"
                }
            }
            Self::Malformed { rule, .. } => rule,
        }
    }
}

impl fmt::Display for CodecError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::MalformedRemainingLength => {
                f.write_str("malformed Remaining Length (MQTT 2.2.3)")
            }
            Self::PacketTooLarge {
                remaining_length,
                limit,
            } => write!(
                f,
                "packet with Remaining Length {remaining_length} exceeds the limit of {limit} bytes"
            ),
            Self::ReservedPacketType(t) => write!(f, "reserved packet type {t} (MQTT 2.2.1)"),
            Self::Unsupported { packet, feature } => {
                write!(f, "{packet}: {feature} is not supported ({})", self.rule())
            }
            Self::Malformed {
                packet,
                rule,
                problem,
            } => write!(f, "{packet}: {problem} ({rule})"),
        }
    }
}

impl std::error::Error for CodecError {}

const fn malformed(packet: PacketType, rule: &'static str, problem: &'static str) -> CodecError {
    CodecError::Malformed {
        packet,
        rule,
        problem,
    }
}

const fn unsupported(packet: PacketType, feature: &'static str) -> CodecError {
    CodecError::Unsupported { packet, feature }
}

// ---------------------------------------------------------------------------
// Field rules shared by encoder and decoder

/// A UTF-8 string field: at most 65535 bytes, no U+0000 (MQTT-1.5.3-2).
/// Rust strings exclude the surrogates U+D800 … U+DFFF (MQTT-1.5.3-1).
fn check_string(packet: PacketType, s: &str) -> Result<(), CodecError> {
    if s.len() > MAX_FIELD_LENGTH {
        return Err(malformed(
            packet,
            "MQTT 1.5.3",
            "UTF-8 string longer than 65535 bytes",
        ));
    }
    if s.contains('\0') {
        return Err(malformed(
            packet,
            "MQTT-1.5.3-2",
            "UTF-8 string contains U+0000",
        ));
    }
    Ok(())
}

fn check_binary(packet: PacketType, b: &[u8]) -> Result<(), CodecError> {
    if b.len() > MAX_FIELD_LENGTH {
        return Err(malformed(
            packet,
            "MQTT 1.5.3",
            "binary field longer than 65535 bytes",
        ));
    }
    Ok(())
}

/// A topic name of PUBLISH or of the Will: not empty (MQTT-4.7.3-1), no
/// wildcards (MQTT-3.3.2-2, MQTT-3.1.3-10).
///
/// # Errors
///
/// [`CodecError::Malformed`] with the broken rule.
pub fn check_topic_name(packet: PacketType, topic: &str) -> Result<(), CodecError> {
    check_string(packet, topic)?;
    if topic.is_empty() {
        return Err(malformed(packet, "MQTT-4.7.3-1", "empty topic name"));
    }
    if topic.contains(['+', '#']) {
        return Err(malformed(
            packet,
            "MQTT-3.3.2-2",
            "wildcard in a topic name",
        ));
    }
    Ok(())
}

/// A topic filter of SUBSCRIBE or UNSUBSCRIBE: not empty (MQTT-4.7.3-1),
/// `#` only alone as the last level (MQTT-4.7.1-2), `+` only alone in a
/// level (MQTT-4.7.1-3).
///
/// # Errors
///
/// [`CodecError::Malformed`] with the broken rule.
pub fn check_topic_filter(packet: PacketType, filter: &str) -> Result<(), CodecError> {
    check_string(packet, filter)?;
    if filter.is_empty() {
        return Err(malformed(packet, "MQTT-4.7.3-1", "empty topic filter"));
    }
    let mut levels = filter.split('/').peekable();
    while let Some(level) = levels.next() {
        if level.contains('#') && (level != "#" || levels.peek().is_some()) {
            return Err(malformed(
                packet,
                "MQTT-4.7.1-2",
                "'#' is not alone in the last level of the topic filter",
            ));
        }
        if level.contains('+') && level != "+" {
            return Err(malformed(
                packet,
                "MQTT-4.7.1-3",
                "'+' does not occupy an entire level of the topic filter",
            ));
        }
    }
    Ok(())
}

/// The rules of a packet that do not depend on the encoding.
fn check(packet: &Packet) -> Result<(), CodecError> {
    let t = packet.packet_type();
    match packet {
        Packet::Connect(c) => {
            check_string(t, &c.client_id)?;
            if let Some(w) = &c.will {
                check_topic_name(t, &w.topic)?;
                check_binary(t, &w.payload)?;
            }
            if let Some(u) = &c.username {
                check_string(t, u)?;
            }
            if let Some(p) = &c.password {
                check_binary(t, p)?;
                if c.username.is_none() {
                    return Err(malformed(t, "MQTT-3.1.2-22", "Password without User Name"));
                }
            }
        }
        Packet::ConnAck(a) => {
            if a.session_present && a.code != ConnectReturnCode::Accepted {
                return Err(malformed(
                    t,
                    "MQTT-3.2.2-4",
                    "Session Present with a non-zero return code",
                ));
            }
        }
        Packet::Publish(p) => check_topic_name(t, &p.topic)?,
        Packet::Subscribe(s) => {
            if s.filters.is_empty() {
                return Err(malformed(t, "MQTT-3.8.3-3", "no topic filter"));
            }
            for (f, _) in &s.filters {
                check_topic_filter(t, f)?;
            }
        }
        Packet::SubAck(s) => {
            if s.codes.is_empty() {
                return Err(malformed(t, "MQTT-3.9.3-1", "no return code"));
            }
        }
        Packet::Unsubscribe(u) => {
            if u.filters.is_empty() {
                return Err(malformed(t, "MQTT-3.10.3-2", "no topic filter"));
            }
            for f in &u.filters {
                check_topic_filter(t, f)?;
            }
        }
        Packet::PubAck(_)
        | Packet::UnsubAck(_)
        | Packet::PingReq
        | Packet::PingResp
        | Packet::Disconnect => {}
    }
    Ok(())
}

// ---------------------------------------------------------------------------
// Encoder

/// Encodes `packet` and appends it to `out`; returns the number of bytes
/// written. On error nothing is appended.
///
/// # Errors
///
/// [`CodecError`] if the packet breaks a rule of MQTT 3.1.1 (e.g. a
/// wildcard in a topic name, a Password without User Name, a string with
/// U+0000) or is larger than [`MAX_REMAINING_LENGTH`].
pub fn encode(packet: &Packet, out: &mut Vec<u8>) -> Result<usize, CodecError> {
    check(packet)?;
    let len = body_len(packet);
    if len > MAX_REMAINING_LENGTH {
        return Err(CodecError::PacketTooLarge {
            remaining_length: len,
            limit: MAX_REMAINING_LENGTH,
        });
    }
    let start = out.len();
    out.reserve(len + 5);
    out.push(first_byte(packet));
    put_remaining_length(len, out);
    put_body(packet, out);
    Ok(out.len() - start)
}

/// The size of the encoded packet without encoding it (fixed header
/// included); `None` if the Remaining Length exceeds
/// [`MAX_REMAINING_LENGTH`]. The rules of [`encode`] are not checked.
pub fn encoded_len(packet: &Packet) -> Option<usize> {
    let len = body_len(packet);
    (len <= MAX_REMAINING_LENGTH).then(|| 1 + remaining_length_size(len) + len)
}

fn first_byte(packet: &Packet) -> u8 {
    let t = packet.packet_type();
    let flags = match packet {
        Packet::Publish(p) => {
            let dup = matches!(p.delivery, Delivery::AtLeastOnce { dup: true, .. });
            (u8::from(dup) << 3) | (p.delivery.qos().bits() << 1) | u8::from(p.retain)
        }
        _ => t.fixed_flags().unwrap_or_default(),
    };
    (t.bits() << 4) | flags
}

fn body_len(packet: &Packet) -> usize {
    match packet {
        Packet::Connect(c) => {
            // protocol name (2 + 4), level, flags, keep alive
            10 + 2
                + c.client_id.len()
                + c.will
                    .as_ref()
                    .map_or(0, |w| 2 + w.topic.len() + 2 + w.payload.len())
                + c.username.as_ref().map_or(0, |u| 2 + u.len())
                + c.password.as_ref().map_or(0, |p| 2 + p.len())
        }
        Packet::ConnAck(_) | Packet::PubAck(_) | Packet::UnsubAck(_) => 2,
        Packet::Publish(p) => {
            2 + p.topic.len()
                + if p.delivery.packet_id().is_some() {
                    2
                } else {
                    0
                }
                + p.payload.len()
        }
        Packet::Subscribe(s) => 2 + s.filters.iter().map(|(f, _)| 3 + f.len()).sum::<usize>(),
        Packet::SubAck(s) => 2 + s.codes.len(),
        Packet::Unsubscribe(u) => 2 + u.filters.iter().map(|f| 2 + f.len()).sum::<usize>(),
        Packet::PingReq | Packet::PingResp | Packet::Disconnect => 0,
    }
}

fn remaining_length_size(len: usize) -> usize {
    match len {
        0..=127 => 1,
        128..=16_383 => 2,
        16_384..=2_097_151 => 3,
        _ => 4,
    }
}

#[allow(clippy::cast_possible_truncation)] // masked to 7 bits
fn put_remaining_length(mut len: usize, out: &mut Vec<u8>) {
    loop {
        let mut byte = (len & 0x7F) as u8;
        len >>= 7;
        if len > 0 {
            byte |= 0x80;
        }
        out.push(byte);
        if len == 0 {
            break;
        }
    }
}

#[allow(clippy::cast_possible_truncation)] // checked: at most 65535 (MQTT 1.5.3)
fn put_u16_len(len: usize, out: &mut Vec<u8>) {
    debug_assert!(len <= MAX_FIELD_LENGTH);
    out.extend_from_slice(&(len as u16).to_be_bytes());
}

fn put_field(b: &[u8], out: &mut Vec<u8>) {
    put_u16_len(b.len(), out);
    out.extend_from_slice(b);
}

fn put_body(packet: &Packet, out: &mut Vec<u8>) {
    match packet {
        Packet::Connect(c) => {
            put_field(b"MQTT", out);
            out.push(4);
            let mut flags = 0u8;
            if c.clean_session {
                flags |= 0x02;
            }
            if let Some(w) = &c.will {
                // Will Retain (0x20) stays 0 (REQ-MQTT-018, REQ-LIFE-008)
                flags |= 0x04 | (w.qos.bits() << 3);
            }
            if c.password.is_some() {
                flags |= 0x40;
            }
            if c.username.is_some() {
                flags |= 0x80;
            }
            out.push(flags);
            out.extend_from_slice(&c.keep_alive.to_be_bytes());
            put_field(c.client_id.as_bytes(), out);
            if let Some(w) = &c.will {
                put_field(w.topic.as_bytes(), out);
                put_field(&w.payload, out);
            }
            if let Some(u) = &c.username {
                put_field(u.as_bytes(), out);
            }
            if let Some(p) = &c.password {
                put_field(p, out);
            }
        }
        Packet::ConnAck(a) => {
            out.push(u8::from(a.session_present));
            out.push(a.code.byte());
        }
        Packet::Publish(p) => {
            put_field(p.topic.as_bytes(), out);
            if let Some(id) = p.delivery.packet_id() {
                out.extend_from_slice(&id.get().to_be_bytes());
            }
            out.extend_from_slice(&p.payload);
        }
        Packet::PubAck(id) | Packet::UnsubAck(id) => out.extend_from_slice(&id.get().to_be_bytes()),
        Packet::Subscribe(s) => {
            out.extend_from_slice(&s.packet_id.get().to_be_bytes());
            for (f, qos) in &s.filters {
                put_field(f.as_bytes(), out);
                out.push(qos.bits());
            }
        }
        Packet::SubAck(s) => {
            out.extend_from_slice(&s.packet_id.get().to_be_bytes());
            out.extend(s.codes.iter().map(|c| match c {
                SubscribeReturnCode::Success(q) => q.bits(),
                SubscribeReturnCode::Failure => 0x80,
            }));
        }
        Packet::Unsubscribe(u) => {
            out.extend_from_slice(&u.packet_id.get().to_be_bytes());
            for f in &u.filters {
                put_field(f.as_bytes(), out);
            }
        }
        Packet::PingReq | Packet::PingResp | Packet::Disconnect => {}
    }
}

// ---------------------------------------------------------------------------
// Decoder

/// Decodes the first packet of `buf`.
///
/// Returns `Ok(None)` while the packet is incomplete, otherwise the packet
/// and the number of bytes it occupies. `limit` bounds the Remaining Length
/// (at most [`MAX_REMAINING_LENGTH`]); a larger packet is refused as soon
/// as its fixed header is complete, before its body arrives.
///
/// # Errors
///
/// [`CodecError`] if the bytes are no valid packet; the fixed header is
/// checked first, so a wrong first byte fails without waiting for more.
pub fn decode(buf: &[u8], limit: usize) -> Result<Option<(Packet, usize)>, CodecError> {
    let Some(&first) = buf.first() else {
        return Ok(None);
    };
    let t = PacketType::from_bits(first >> 4).ok_or(CodecError::ReservedPacketType(first >> 4))?;
    let flags = first & 0x0F;
    if matches!(
        t,
        PacketType::PubRec | PacketType::PubRel | PacketType::PubComp
    ) {
        return Err(unsupported(t, "the QoS 2 flow"));
    }
    if let Some(fixed) = t.fixed_flags()
        && flags != fixed
    {
        return Err(malformed(
            t,
            "MQTT-2.2.2-2",
            "invalid flags in the fixed header",
        ));
    }
    let mut len = 0usize;
    let mut header = 1;
    loop {
        let Some(&b) = buf.get(header) else {
            return Ok(None);
        };
        len |= usize::from(b & 0x7F) << (7 * (header - 1));
        header += 1;
        if b & 0x80 == 0 {
            // minimal encoding: a continued length never ends with 0
            if header > 2 && b == 0 {
                return Err(CodecError::MalformedRemainingLength);
            }
            break;
        }
        if header == 5 {
            return Err(CodecError::MalformedRemainingLength);
        }
    }
    let limit = limit.min(MAX_REMAINING_LENGTH);
    if len > limit {
        return Err(CodecError::PacketTooLarge {
            remaining_length: len,
            limit,
        });
    }
    let Some(body) = buf.get(header..header + len) else {
        return Ok(None);
    };
    let packet = decode_body(t, flags, body)?;
    Ok(Some((packet, header + len)))
}

struct Reader<'a> {
    packet: PacketType,
    buf: &'a [u8],
}

impl<'a> Reader<'a> {
    fn truncated(&self) -> CodecError {
        malformed(
            self.packet,
            "MQTT 2.2.3",
            "the Remaining Length ends inside a field",
        )
    }

    fn take(&mut self, n: usize) -> Result<&'a [u8], CodecError> {
        if self.buf.len() < n {
            return Err(self.truncated());
        }
        let (head, tail) = self.buf.split_at(n);
        self.buf = tail;
        Ok(head)
    }

    fn u8(&mut self) -> Result<u8, CodecError> {
        Ok(self.take(1)?[0])
    }

    fn u16(&mut self) -> Result<u16, CodecError> {
        let b = self.take(2)?;
        Ok(u16::from_be_bytes([b[0], b[1]]))
    }

    fn packet_id(&mut self) -> Result<PacketId, CodecError> {
        PacketId::new(self.u16()?)
            .ok_or_else(|| malformed(self.packet, "MQTT-2.3.1-1", "packet identifier 0"))
    }

    fn binary(&mut self) -> Result<&'a [u8], CodecError> {
        let n = self.u16()?;
        self.take(usize::from(n))
    }

    fn string(&mut self) -> Result<&'a str, CodecError> {
        let b = self.binary()?;
        let s = core::str::from_utf8(b).map_err(|_| {
            malformed(
                self.packet,
                "MQTT-1.5.3-1",
                "UTF-8 string is not well-formed",
            )
        })?;
        check_string(self.packet, s)?;
        Ok(s)
    }

    fn is_empty(&self) -> bool {
        self.buf.is_empty()
    }

    fn rest(self) -> &'a [u8] {
        self.buf
    }

    fn end(self, rule: &'static str) -> Result<(), CodecError> {
        if self.buf.is_empty() {
            Ok(())
        } else {
            Err(malformed(self.packet, rule, "bytes after the last field"))
        }
    }
}

fn decode_body(t: PacketType, flags: u8, body: &[u8]) -> Result<Packet, CodecError> {
    let mut r = Reader {
        packet: t,
        buf: body,
    };
    Ok(match t {
        PacketType::Connect => Packet::Connect(decode_connect(r)?),
        PacketType::ConnAck => Packet::ConnAck(decode_connack(r)?),
        PacketType::Publish => Packet::Publish(decode_publish(r, flags)?),
        PacketType::PubAck => {
            let id = r.packet_id()?;
            r.end("MQTT 3.4")?;
            Packet::PubAck(id)
        }
        PacketType::UnsubAck => {
            let id = r.packet_id()?;
            r.end("MQTT 3.11")?;
            Packet::UnsubAck(id)
        }
        PacketType::Subscribe => Packet::Subscribe(decode_subscribe(r)?),
        PacketType::SubAck => Packet::SubAck(decode_suback(r)?),
        PacketType::Unsubscribe => Packet::Unsubscribe(decode_unsubscribe(r)?),
        PacketType::PingReq | PacketType::PingResp | PacketType::Disconnect => {
            r.end("MQTT 2.2.3")?;
            match t {
                PacketType::PingReq => Packet::PingReq,
                PacketType::PingResp => Packet::PingResp,
                _ => Packet::Disconnect,
            }
        }
        PacketType::PubRec | PacketType::PubRel | PacketType::PubComp => {
            return Err(unsupported(t, "the QoS 2 flow"));
        }
    })
}

fn decode_connack(mut r: Reader<'_>) -> Result<ConnAck, CodecError> {
    let t = PacketType::ConnAck;
    let ack = r.u8()?;
    let code = r.u8()?;
    r.end("MQTT 3.2")?;
    if ack & 0xFE != 0 {
        return Err(malformed(
            t,
            "MQTT 3.2.2.1",
            "reserved bits of the acknowledge flags set",
        ));
    }
    let code = ConnectReturnCode::from_byte(code)
        .ok_or_else(|| malformed(t, "MQTT 3.2.2.3", "reserved Connect Return Code"))?;
    let a = ConnAck {
        session_present: ack == 1,
        code,
    };
    check(&Packet::ConnAck(a))?;
    Ok(a)
}

fn decode_publish(mut r: Reader<'_>, flags: u8) -> Result<Publish, CodecError> {
    let t = PacketType::Publish;
    let qos = (flags >> 1) & 0x03;
    let dup = flags & 0x08 != 0;
    let retain = flags & 0x01 != 0;
    let topic = r.string()?;
    check_topic_name(t, topic)?;
    let delivery = match qos {
        0 if dup => {
            return Err(malformed(t, "MQTT-3.3.1-2", "DUP set with QoS 0"));
        }
        0 => Delivery::AtMostOnce,
        1 => Delivery::AtLeastOnce {
            packet_id: r.packet_id()?,
            dup,
        },
        2 => return Err(unsupported(t, "QoS 2")),
        _ => return Err(malformed(t, "MQTT-3.3.1-4", "QoS bits set to 3")),
    };
    Ok(Publish {
        topic: topic.to_owned(),
        payload: r.rest().to_vec(),
        delivery,
        retain,
    })
}

fn decode_subscribe(mut r: Reader<'_>) -> Result<Subscribe, CodecError> {
    let t = PacketType::Subscribe;
    let packet_id = r.packet_id()?;
    let mut filters = Vec::new();
    while !r.is_empty() {
        let f = r.string()?;
        check_topic_filter(t, f)?;
        let qos = match r.u8()? {
            0 => QoS::AtMostOnce,
            1 => QoS::AtLeastOnce,
            2 => return Err(unsupported(t, "QoS 2")),
            _ => {
                return Err(malformed(
                    t,
                    "MQTT-3.8.3-4",
                    "reserved bits or QoS 3 in the requested QoS",
                ));
            }
        };
        filters.push((f.to_owned(), qos));
    }
    if filters.is_empty() {
        return Err(malformed(t, "MQTT-3.8.3-3", "no topic filter"));
    }
    Ok(Subscribe { packet_id, filters })
}

fn decode_suback(mut r: Reader<'_>) -> Result<SubAck, CodecError> {
    let t = PacketType::SubAck;
    let packet_id = r.packet_id()?;
    let codes = r
        .rest()
        .iter()
        .map(|&c| match c {
            0 => Ok(SubscribeReturnCode::Success(QoS::AtMostOnce)),
            1 => Ok(SubscribeReturnCode::Success(QoS::AtLeastOnce)),
            2 => Err(unsupported(t, "QoS 2")),
            0x80 => Ok(SubscribeReturnCode::Failure),
            _ => Err(malformed(t, "MQTT-3.9.3-2", "reserved return code")),
        })
        .collect::<Result<Vec<_>, _>>()?;
    if codes.is_empty() {
        return Err(malformed(t, "MQTT-3.9.3-1", "no return code"));
    }
    Ok(SubAck { packet_id, codes })
}

fn decode_unsubscribe(mut r: Reader<'_>) -> Result<Unsubscribe, CodecError> {
    let t = PacketType::Unsubscribe;
    let packet_id = r.packet_id()?;
    let mut filters = Vec::new();
    while !r.is_empty() {
        let f = r.string()?;
        check_topic_filter(t, f)?;
        filters.push(f.to_owned());
    }
    if filters.is_empty() {
        return Err(malformed(t, "MQTT-3.10.3-2", "no topic filter"));
    }
    Ok(Unsubscribe { packet_id, filters })
}

fn decode_connect(mut r: Reader<'_>) -> Result<Connect, CodecError> {
    let t = PacketType::Connect;
    if r.binary()? != b"MQTT" {
        return Err(malformed(
            t,
            "MQTT-3.1.2-1",
            "protocol name is not \"MQTT\"",
        ));
    }
    if r.u8()? != 4 {
        return Err(unsupported(t, "a protocol level other than 4 (MQTT 3.1.1)"));
    }
    let flags = r.u8()?;
    let keep_alive = r.u16()?;
    if flags & 0x01 != 0 {
        return Err(malformed(t, "MQTT-3.1.2-3", "reserved flag set"));
    }
    let will_flag = flags & 0x04 != 0;
    let will_qos = (flags >> 3) & 0x03;
    let will_retain = flags & 0x20 != 0;
    if !will_flag && will_qos != 0 {
        return Err(malformed(t, "MQTT-3.1.2-13", "Will QoS without Will"));
    }
    if !will_flag && will_retain {
        return Err(malformed(t, "MQTT-3.1.2-15", "Will Retain without Will"));
    }
    if will_retain {
        return Err(unsupported(t, "Will Retain"));
    }
    let will_qos = match will_qos {
        0 => QoS::AtMostOnce,
        1 => QoS::AtLeastOnce,
        2 => return Err(unsupported(t, "Will QoS 2")),
        _ => return Err(malformed(t, "MQTT-3.1.2-14", "Will QoS 3")),
    };
    let username_flag = flags & 0x80 != 0;
    let password_flag = flags & 0x40 != 0;
    if password_flag && !username_flag {
        return Err(malformed(t, "MQTT-3.1.2-22", "Password without User Name"));
    }
    let client_id = r.string()?.to_owned();
    let will = if will_flag {
        let topic = r.string()?;
        check_topic_name(t, topic)?;
        Some(LastWill {
            topic: topic.to_owned(),
            payload: r.binary()?.to_vec(),
            qos: will_qos,
        })
    } else {
        None
    };
    let username = username_flag
        .then(|| r.string().map(str::to_owned))
        .transpose()?;
    let password = password_flag
        .then(|| r.binary().map(<[u8]>::to_vec))
        .transpose()?;
    r.end("MQTT 3.1.3")?;
    Ok(Connect {
        client_id,
        keep_alive,
        clean_session: flags & 0x02 != 0,
        will,
        username,
        password,
    })
}

/// Incremental decoder of a byte stream from the broker: [`Self::feed`]
/// what the transport read, take packets with [`Self::next_packet`] until
/// it returns `Ok(None)`.
///
/// The decoder buffers at most one incomplete packet plus what was fed
/// after it. After an error it is poisoned and returns the same error
/// again until [`Self::reset`] (the connection is closed, MQTT 4.8).
#[derive(Debug, Clone)]
pub struct Decoder {
    buf: Vec<u8>,
    pos: usize,
    limit: usize,
    failed: Option<CodecError>,
}

impl Decoder {
    /// A decoder that refuses packets whose Remaining Length exceeds
    /// `limit` (capped at [`MAX_REMAINING_LENGTH`]).
    pub fn new(limit: usize) -> Self {
        Self {
            buf: Vec::new(),
            pos: 0,
            limit: limit.min(MAX_REMAINING_LENGTH),
            failed: None,
        }
    }

    /// The limit of the Remaining Length.
    pub fn limit(&self) -> usize {
        self.limit
    }

    /// Appends bytes read from the transport.
    pub fn feed(&mut self, bytes: &[u8]) {
        if self.pos > 0 && self.pos >= self.buf.len() / 2 {
            self.buf.drain(..self.pos);
            self.pos = 0;
        }
        self.buf.extend_from_slice(bytes);
    }

    /// The next complete packet, `Ok(None)` if more bytes are needed.
    ///
    /// # Errors
    ///
    /// The [`CodecError`] of the first invalid packet; the decoder stays
    /// poisoned.
    pub fn next_packet(&mut self) -> Result<Option<Packet>, CodecError> {
        if let Some(e) = &self.failed {
            return Err(e.clone());
        }
        match decode(&self.buf[self.pos..], self.limit) {
            Ok(Some((packet, n))) => {
                self.pos += n;
                if self.pos == self.buf.len() {
                    self.buf.clear();
                    self.pos = 0;
                }
                Ok(Some(packet))
            }
            Ok(None) => Ok(None),
            Err(e) => {
                self.failed = Some(e.clone());
                Err(e)
            }
        }
    }

    /// Bytes buffered and not yet decoded.
    pub fn buffered(&self) -> usize {
        self.buf.len() - self.pos
    }

    /// Drops the buffer and the error (new connection).
    pub fn reset(&mut self) {
        self.buf.clear();
        self.pos = 0;
        self.failed = None;
    }
}
