//! P3 spike (C-043, ADR-0023 option a, Q-026): the MQTT 3.1.1 client subset
//! that OEC needs, as a **sans-IO** codec in the style of `oec-core`
//! (ADR-0011). The codec does no I/O and holds no clock: the runtime feeds it
//! the bytes it received and the current time, and it returns packets to send
//! and events. TCP, TLS (rustls), reconnect timers and backoff stay in the
//! runtime crate `oec`.
//!
//! Scope (MQTT 3.1.1, OASIS 2014; REQ-MQTT-001):
//! - client → broker: CONNECT (Will built anew for every connect,
//!   REQ-LIFE-016), PUBLISH QoS 0/1 without retain (REQ-MQTT-017/018),
//!   PUBACK, SUBSCRIBE, UNSUBSCRIBE, PINGREQ, DISCONNECT;
//! - broker → client: CONNACK, PUBLISH, PUBACK, SUBACK, UNSUBACK, PINGRESP;
//!   every other packet type is a protocol error;
//! - keep alive as a pure timer (REQ-MQTT-014).
//!
//! Not product code: no QoS 2, no session resumption, no inflight window.

/// Largest remaining length that MQTT can encode (4 length bytes).
pub const MAX_REMAINING_LENGTH: usize = 268_435_455;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum QoS {
    AtMostOnce = 0,
    AtLeastOnce = 1,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Will<'a> {
    pub topic: &'a str,
    pub payload: &'a [u8],
    pub qos: QoS,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Connect<'a> {
    pub client_id: &'a str,
    pub keep_alive_s: u16,
    pub clean_session: bool,
    pub will: Option<Will<'a>>,
    pub username: Option<&'a str>,
    pub password: Option<&'a [u8]>,
}

/// Outgoing packets. PUBLISH has no retain flag on purpose (REQ-MQTT-018).
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Outgoing<'a> {
    Connect(Connect<'a>),
    Publish { topic: &'a str, payload: &'a [u8], qos: QoS, pkid: u16 },
    PubAck { pkid: u16 },
    Subscribe { pkid: u16, filters: &'a [(&'a str, QoS)] },
    Unsubscribe { pkid: u16, filters: &'a [&'a str] },
    PingReq,
    Disconnect,
}

/// Incoming packets (owned, so the decoder buffer can be reused).
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Incoming {
    ConnAck { session_present: bool, code: u8 },
    Publish { topic: String, payload: Vec<u8>, qos: QoS, pkid: u16, retain: bool, dup: bool },
    PubAck { pkid: u16 },
    SubAck { pkid: u16, codes: Vec<u8> },
    UnsubAck { pkid: u16 },
    PingResp,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Error {
    /// More than 4 remaining-length bytes or a non-minimal encoding.
    MalformedLength,
    /// The packet is larger than the configured maximum (broker MaxPacketSize).
    TooLarge(usize),
    /// A packet type or flag combination a client must not receive.
    Protocol(&'static str),
    /// A string is not UTF-8 or contains U+0000.
    Utf8,
    /// A field does not fit its length prefix.
    Field(&'static str),
}

fn put_u16(out: &mut Vec<u8>, v: u16) {
    out.extend_from_slice(&v.to_be_bytes());
}

fn put_bytes(out: &mut Vec<u8>, b: &[u8]) -> Result<(), Error> {
    put_u16(out, u16::try_from(b.len()).map_err(|_| Error::Field("longer than 65535 bytes"))?);
    out.extend_from_slice(b);
    Ok(())
}

fn put_str(out: &mut Vec<u8>, s: &str) -> Result<(), Error> {
    if s.contains('\0') {
        return Err(Error::Utf8);
    }
    put_bytes(out, s.as_bytes())
}

fn put_length(out: &mut Vec<u8>, mut len: usize) -> Result<(), Error> {
    if len > MAX_REMAINING_LENGTH {
        return Err(Error::TooLarge(len));
    }
    loop {
        let mut byte = (len % 128) as u8;
        len /= 128;
        if len > 0 {
            byte |= 0x80;
        }
        out.push(byte);
        if len == 0 {
            return Ok(());
        }
    }
}

/// Encodes one packet and appends it to `out`.
pub fn encode(p: &Outgoing<'_>, out: &mut Vec<u8>) -> Result<(), Error> {
    let mut body = Vec::new();
    let first = match p {
        Outgoing::Connect(c) => {
            body.extend_from_slice(&[0, 4, b'M', b'Q', b'T', b'T', 4]);
            let mut flags = 0u8;
            if c.clean_session {
                flags |= 0x02;
            }
            if let Some(w) = &c.will {
                flags |= 0x04 | ((w.qos as u8) << 3);
            }
            if c.password.is_some() {
                if c.username.is_none() {
                    return Err(Error::Field("password without username (MQTT-3.1.2-22)"));
                }
                flags |= 0x40;
            }
            if c.username.is_some() {
                flags |= 0x80;
            }
            body.push(flags);
            put_u16(&mut body, c.keep_alive_s);
            put_str(&mut body, c.client_id)?;
            if let Some(w) = &c.will {
                put_str(&mut body, w.topic)?;
                put_bytes(&mut body, w.payload)?;
            }
            if let Some(u) = c.username {
                put_str(&mut body, u)?;
            }
            if let Some(pw) = c.password {
                put_bytes(&mut body, pw)?;
            }
            0x10
        }
        Outgoing::Publish { topic, payload, qos, pkid } => {
            if topic.is_empty() || topic.contains(['+', '#']) {
                return Err(Error::Field("publish topic must be a non-empty name without wildcards"));
            }
            put_str(&mut body, topic)?;
            if *qos != QoS::AtMostOnce {
                put_u16(&mut body, *pkid);
            }
            body.extend_from_slice(payload);
            0x30 | ((*qos as u8) << 1)
        }
        Outgoing::PubAck { pkid } => {
            put_u16(&mut body, *pkid);
            0x40
        }
        Outgoing::Subscribe { pkid, filters } => {
            if filters.is_empty() {
                return Err(Error::Field("SUBSCRIBE needs a filter"));
            }
            put_u16(&mut body, *pkid);
            for (f, q) in *filters {
                put_str(&mut body, f)?;
                body.push(*q as u8);
            }
            0x82
        }
        Outgoing::Unsubscribe { pkid, filters } => {
            if filters.is_empty() {
                return Err(Error::Field("UNSUBSCRIBE needs a filter"));
            }
            put_u16(&mut body, *pkid);
            for f in *filters {
                put_str(&mut body, f)?;
            }
            0xA2
        }
        Outgoing::PingReq => 0xC0,
        Outgoing::Disconnect => 0xE0,
    };
    out.push(first);
    put_length(out, body.len())?;
    out.extend_from_slice(&body);
    Ok(())
}

/// Incremental decoder: feed any byte chunks, take complete packets.
pub struct Decoder {
    buf: Vec<u8>,
    max_packet: usize,
}

impl Decoder {
    /// `max_packet` bounds the whole packet (e.g. the broker MaxPacketSize).
    pub fn new(max_packet: usize) -> Self {
        Decoder { buf: Vec::new(), max_packet }
    }

    pub fn feed(&mut self, bytes: &[u8]) {
        self.buf.extend_from_slice(bytes);
    }

    /// The next complete packet, `Ok(None)` if more bytes are needed. After
    /// an error the connection must be closed (MQTT-4.8.0-1).
    pub fn next_packet(&mut self) -> Result<Option<Incoming>, Error> {
        let Some((len, hdr)) = remaining_length(&self.buf)? else { return Ok(None) };
        let total = 1 + hdr + len;
        if total > self.max_packet {
            return Err(Error::TooLarge(total));
        }
        if self.buf.len() < total {
            return Ok(None);
        }
        let first = self.buf[0];
        let body: Vec<u8> = self.buf[1 + hdr..total].to_vec();
        self.buf.drain(..total);
        parse(first, &body).map(Some)
    }
}

fn remaining_length(buf: &[u8]) -> Result<Option<(usize, usize)>, Error> {
    let mut len = 0usize;
    for i in 0..4 {
        let Some(&b) = buf.get(1 + i) else { return Ok(None) };
        len |= ((b & 0x7F) as usize) << (7 * i);
        if b & 0x80 == 0 {
            if i > 0 && b == 0 {
                return Err(Error::MalformedLength);
            }
            return Ok(Some((len, i + 1)));
        }
    }
    Err(Error::MalformedLength)
}

struct Reader<'a>(&'a [u8]);

impl Reader<'_> {
    fn u16(&mut self) -> Result<u16, Error> {
        if self.0.len() < 2 {
            return Err(Error::Field("truncated u16"));
        }
        let v = u16::from_be_bytes([self.0[0], self.0[1]]);
        self.0 = &self.0[2..];
        Ok(v)
    }
    fn string(&mut self) -> Result<String, Error> {
        let n = self.u16()? as usize;
        if self.0.len() < n {
            return Err(Error::Field("truncated string"));
        }
        let s = std::str::from_utf8(&self.0[..n]).map_err(|_| Error::Utf8)?;
        if s.contains('\0') {
            return Err(Error::Utf8);
        }
        self.0 = &self.0[n..];
        Ok(s.to_string())
    }
}

fn parse(first: u8, body: &[u8]) -> Result<Incoming, Error> {
    let flags = first & 0x0F;
    let mut r = Reader(body);
    let exact = |n: usize| if body.len() == n { Ok(()) } else { Err(Error::Field("wrong remaining length")) };
    match first >> 4 {
        2 if flags == 0 => {
            exact(2)?;
            if body[0] & 0xFE != 0 {
                return Err(Error::Protocol("CONNACK reserved flags"));
            }
            Ok(Incoming::ConnAck { session_present: body[0] & 1 == 1, code: body[1] })
        }
        3 => {
            let qos = match (flags >> 1) & 3 {
                0 => QoS::AtMostOnce,
                1 => QoS::AtLeastOnce,
                2 => return Err(Error::Protocol("QoS 2 is not subscribed by OEC clients")),
                _ => return Err(Error::Protocol("PUBLISH with QoS 3")),
            };
            let topic = r.string()?;
            if topic.is_empty() || topic.contains(['+', '#']) {
                return Err(Error::Protocol("PUBLISH topic with wildcard"));
            }
            let pkid = if qos == QoS::AtMostOnce { 0 } else { r.u16()? };
            Ok(Incoming::Publish { topic, payload: r.0.to_vec(), qos, pkid, retain: flags & 1 == 1, dup: flags & 8 == 8 })
        }
        4 if flags == 0 => {
            exact(2)?;
            Ok(Incoming::PubAck { pkid: r.u16()? })
        }
        9 if flags == 0 => {
            let pkid = r.u16()?;
            if r.0.is_empty() || r.0.iter().any(|c| !matches!(c, 0 | 1 | 2 | 0x80)) {
                return Err(Error::Protocol("SUBACK return codes"));
            }
            Ok(Incoming::SubAck { pkid, codes: r.0.to_vec() })
        }
        11 if flags == 0 => {
            exact(2)?;
            Ok(Incoming::UnsubAck { pkid: r.u16()? })
        }
        13 if flags == 0 => {
            exact(0)?;
            Ok(Incoming::PingResp)
        }
        _ => Err(Error::Protocol("packet type or flags not allowed from a broker")),
    }
}

/// What the runtime has to do after [`KeepAlive::poll`].
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Tick {
    /// Nothing to do before the returned deadline (ms).
    Wait(u64),
    /// Send PINGREQ now.
    Ping,
    /// No PINGRESP within the keep-alive interval: close and reconnect.
    Lost,
}

/// Keep alive as a pure timer (MQTT-3.1.2-23/24): the client sends PINGREQ
/// when it has sent nothing for `keep_alive` and treats a missing PINGRESP
/// within one more interval as a lost connection.
pub struct KeepAlive {
    interval_ms: u64,
    last_sent_ms: u64,
    ping_sent_ms: Option<u64>,
}

impl KeepAlive {
    pub fn new(keep_alive_s: u16, now_ms: u64) -> Self {
        KeepAlive { interval_ms: keep_alive_s as u64 * 1000, last_sent_ms: now_ms, ping_sent_ms: None }
    }
    pub fn on_sent(&mut self, now_ms: u64) {
        self.last_sent_ms = now_ms;
    }
    pub fn on_ping_resp(&mut self) {
        self.ping_sent_ms = None;
    }
    pub fn poll(&mut self, now_ms: u64) -> Tick {
        if self.interval_ms == 0 {
            return Tick::Wait(u64::MAX);
        }
        if let Some(t) = self.ping_sent_ms {
            return if now_ms >= t + self.interval_ms { Tick::Lost } else { Tick::Wait(t + self.interval_ms) };
        }
        let due = self.last_sent_ms + self.interval_ms;
        if now_ms >= due {
            self.ping_sent_ms = Some(now_ms);
            self.last_sent_ms = now_ms;
            Tick::Ping
        } else {
            Tick::Wait(due)
        }
    }
}
