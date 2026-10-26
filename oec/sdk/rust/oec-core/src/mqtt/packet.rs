//! The MQTT 3.1.1 control packets the OEC client exchanges with the broker
//! (REQ-MQTT-001).
//!
//! The types make the forbidden states unrepresentable where that is
//! cheap: a QoS 0 PUBLISH carries no packet id and no DUP flag
//! ([`Delivery`]), a Will has no Retain flag (REQ-MQTT-018, REQ-LIFE-008),
//! QoS 2 does not exist (REQ-MQTT-017; ADR-0026 amendment C-049).

use core::fmt;
use core::num::NonZeroU16;

/// Quality of service of a PUBLISH, a Will or a subscription (MG 6.3.1).
/// QoS 2 is not supported by the SDK (REQ-MQTT-017: QoS 0 by default,
/// other levels may be used).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, PartialOrd, Ord, Default)]
pub enum QoS {
    /// At most once – the default of the guideline (REQ-MQTT-017).
    #[default]
    AtMostOnce,
    /// At least once.
    AtLeastOnce,
}

impl QoS {
    /// The two bits of the wire format.
    pub const fn bits(self) -> u8 {
        match self {
            Self::AtMostOnce => 0,
            Self::AtLeastOnce => 1,
        }
    }
}

/// A packet identifier, never 0 (MQTT-2.3.1-1).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub struct PacketId(NonZeroU16);

impl PacketId {
    /// `None` for 0 (MQTT-2.3.1-1).
    pub const fn new(id: u16) -> Option<Self> {
        match NonZeroU16::new(id) {
            Some(id) => Some(Self(id)),
            None => None,
        }
    }

    /// The id.
    pub const fn get(self) -> u16 {
        self.0.get()
    }
}

impl fmt::Display for PacketId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        self.0.fmt(f)
    }
}

/// The type of a control packet (MQTT 3.1.1 table 2.1).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum PacketType {
    /// 1, client to server.
    Connect,
    /// 2, server to client.
    ConnAck,
    /// 3, both directions.
    Publish,
    /// 4, both directions.
    PubAck,
    /// 5, QoS 2 – not supported.
    PubRec,
    /// 6, QoS 2 – not supported.
    PubRel,
    /// 7, QoS 2 – not supported.
    PubComp,
    /// 8, client to server.
    Subscribe,
    /// 9, server to client.
    SubAck,
    /// 10, client to server.
    Unsubscribe,
    /// 11, server to client.
    UnsubAck,
    /// 12, client to server.
    PingReq,
    /// 13, server to client.
    PingResp,
    /// 14, client to server.
    Disconnect,
}

impl PacketType {
    /// The type of the four high bits of the first byte; `None` for the
    /// reserved values 0 and 15 (MQTT 2.2.1).
    pub const fn from_bits(bits: u8) -> Option<Self> {
        Some(match bits {
            1 => Self::Connect,
            2 => Self::ConnAck,
            3 => Self::Publish,
            4 => Self::PubAck,
            5 => Self::PubRec,
            6 => Self::PubRel,
            7 => Self::PubComp,
            8 => Self::Subscribe,
            9 => Self::SubAck,
            10 => Self::Unsubscribe,
            11 => Self::UnsubAck,
            12 => Self::PingReq,
            13 => Self::PingResp,
            14 => Self::Disconnect,
            _ => return None,
        })
    }

    /// The four high bits of the first byte.
    pub const fn bits(self) -> u8 {
        match self {
            Self::Connect => 1,
            Self::ConnAck => 2,
            Self::Publish => 3,
            Self::PubAck => 4,
            Self::PubRec => 5,
            Self::PubRel => 6,
            Self::PubComp => 7,
            Self::Subscribe => 8,
            Self::SubAck => 9,
            Self::Unsubscribe => 10,
            Self::UnsubAck => 11,
            Self::PingReq => 12,
            Self::PingResp => 13,
            Self::Disconnect => 14,
        }
    }

    /// The fixed flags of the first byte; `None` for PUBLISH, whose flags
    /// carry DUP, QoS and Retain (MQTT 2.2.2, MQTT-2.2.2-1).
    pub const fn fixed_flags(self) -> Option<u8> {
        match self {
            Self::Publish => None,
            Self::PubRel | Self::Subscribe | Self::Unsubscribe => Some(0b0010),
            _ => Some(0),
        }
    }
}

impl fmt::Display for PacketType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(match self {
            Self::Connect => "CONNECT",
            Self::ConnAck => "CONNACK",
            Self::Publish => "PUBLISH",
            Self::PubAck => "PUBACK",
            Self::PubRec => "PUBREC",
            Self::PubRel => "PUBREL",
            Self::PubComp => "PUBCOMP",
            Self::Subscribe => "SUBSCRIBE",
            Self::SubAck => "SUBACK",
            Self::Unsubscribe => "UNSUBSCRIBE",
            Self::UnsubAck => "UNSUBACK",
            Self::PingReq => "PINGREQ",
            Self::PingResp => "PINGRESP",
            Self::Disconnect => "DISCONNECT",
        })
    }
}

/// The Will registered with CONNECT (MG 6.3.5). The Will Retain flag is
/// always 0 (REQ-MQTT-018, REQ-LIFE-008); it has no field here.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LastWill {
    /// Will Topic: a topic name (no wildcards).
    pub topic: String,
    /// Will Message.
    pub payload: Vec<u8>,
    /// Will QoS.
    pub qos: QoS,
}

/// CONNECT (MQTT 3.1): protocol name `MQTT`, level 4 (REQ-MQTT-001).
#[derive(Clone, PartialEq, Eq)]
pub struct Connect {
    /// ClientId (REQ-MQTT-013); may be empty only with Clean Session
    /// (MQTT-3.1.3-7, REQ-MQTT-016).
    pub client_id: String,
    /// Keep Alive in seconds (REQ-MQTT-014); 0 disables it.
    pub keep_alive: u16,
    /// Clean Session (REQ-MQTT-015).
    pub clean_session: bool,
    /// The Will (REQ-LIFE-008).
    pub will: Option<LastWill>,
    /// User Name (REQ-MQTT-009, REQ-MQTT-011: used as provided).
    pub username: Option<String>,
    /// Password; requires a User Name (MQTT-3.1.2-22).
    pub password: Option<Vec<u8>>,
}

impl fmt::Debug for Connect {
    // The password never appears in logs (G-4.5).
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("Connect")
            .field("client_id", &self.client_id)
            .field("keep_alive", &self.keep_alive)
            .field("clean_session", &self.clean_session)
            .field("will", &self.will)
            .field("username", &self.username)
            .field("password", &self.password.as_ref().map(|_| "***"))
            .finish()
    }
}

/// The Connect Return Code of CONNACK (MQTT 3.2.2.3).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum ConnectReturnCode {
    /// 0 – connection accepted.
    Accepted,
    /// 1 – the server does not support MQTT 3.1.1.
    UnacceptableProtocolVersion,
    /// 2 – the ClientId is not allowed.
    IdentifierRejected,
    /// 3 – the MQTT service is unavailable.
    ServerUnavailable,
    /// 4 – malformed User Name or Password.
    BadUserNameOrPassword,
    /// 5 – the client is not authorized.
    NotAuthorized,
}

impl ConnectReturnCode {
    /// The code of the wire format; `None` for the reserved 6 … 255.
    pub const fn from_byte(b: u8) -> Option<Self> {
        Some(match b {
            0 => Self::Accepted,
            1 => Self::UnacceptableProtocolVersion,
            2 => Self::IdentifierRejected,
            3 => Self::ServerUnavailable,
            4 => Self::BadUserNameOrPassword,
            5 => Self::NotAuthorized,
            _ => return None,
        })
    }

    /// The byte of the wire format.
    pub const fn byte(self) -> u8 {
        match self {
            Self::Accepted => 0,
            Self::UnacceptableProtocolVersion => 1,
            Self::IdentifierRejected => 2,
            Self::ServerUnavailable => 3,
            Self::BadUserNameOrPassword => 4,
            Self::NotAuthorized => 5,
        }
    }
}

impl fmt::Display for ConnectReturnCode {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(match self {
            Self::Accepted => "connection accepted",
            Self::UnacceptableProtocolVersion => "unacceptable protocol version",
            Self::IdentifierRejected => "identifier rejected",
            Self::ServerUnavailable => "server unavailable",
            Self::BadUserNameOrPassword => "bad user name or password",
            Self::NotAuthorized => "not authorized",
        })
    }
}

/// CONNACK (MQTT 3.2).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ConnAck {
    /// Session Present; only with [`ConnectReturnCode::Accepted`]
    /// (MQTT-3.2.2-4).
    pub session_present: bool,
    /// Connect Return Code.
    pub code: ConnectReturnCode,
}

/// QoS of a PUBLISH together with the fields that exist only for QoS 1.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Delivery {
    /// QoS 0: no packet id, DUP 0 (MQTT-3.3.1-2).
    AtMostOnce,
    /// QoS 1.
    AtLeastOnce {
        /// Packet Identifier.
        packet_id: PacketId,
        /// DUP: a re-delivery (MQTT-3.3.1-1).
        dup: bool,
    },
}

impl Delivery {
    /// The QoS.
    pub const fn qos(self) -> QoS {
        match self {
            Self::AtMostOnce => QoS::AtMostOnce,
            Self::AtLeastOnce { .. } => QoS::AtLeastOnce,
        }
    }

    /// The packet id of QoS 1.
    pub const fn packet_id(self) -> Option<PacketId> {
        match self {
            Self::AtMostOnce => None,
            Self::AtLeastOnce { packet_id, .. } => Some(packet_id),
        }
    }
}

/// PUBLISH (MQTT 3.3).
///
/// The codec keeps the Retain flag, because a broker may set it towards a
/// subscriber (MQTT-3.3.1-8); the session never sets it on what it
/// publishes (REQ-MQTT-018).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Publish {
    /// Topic Name: no wildcards (MQTT-3.3.2-2).
    pub topic: String,
    /// Application Message (the NetworkMessage, RIG 4.1.1).
    pub payload: Vec<u8>,
    /// QoS, packet id and DUP.
    pub delivery: Delivery,
    /// Retain flag.
    pub retain: bool,
}

/// One Return Code of SUBACK (MQTT 3.9.3).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum SubscribeReturnCode {
    /// Success with the granted maximum QoS.
    Success(QoS),
    /// 0x80 – failure (e.g. refused by an access control list).
    Failure,
}

/// SUBSCRIBE (MQTT 3.8).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Subscribe {
    /// Packet Identifier.
    pub packet_id: PacketId,
    /// Topic filters with the requested QoS; at least one (MQTT-3.8.3-3).
    pub filters: Vec<(String, QoS)>,
}

/// SUBACK (MQTT 3.9).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SubAck {
    /// Packet Identifier of the SUBSCRIBE.
    pub packet_id: PacketId,
    /// One code per topic filter of the SUBSCRIBE, in order (MQTT-3.9.3-1).
    pub codes: Vec<SubscribeReturnCode>,
}

/// UNSUBSCRIBE (MQTT 3.10).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Unsubscribe {
    /// Packet Identifier.
    pub packet_id: PacketId,
    /// Topic filters; at least one (MQTT-3.10.3-2).
    pub filters: Vec<String>,
}

/// A control packet of MQTT 3.1.1 without the QoS 2 flow.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Packet {
    /// CONNECT.
    Connect(Connect),
    /// CONNACK.
    ConnAck(ConnAck),
    /// PUBLISH.
    Publish(Publish),
    /// PUBACK.
    PubAck(PacketId),
    /// SUBSCRIBE.
    Subscribe(Subscribe),
    /// SUBACK.
    SubAck(SubAck),
    /// UNSUBSCRIBE.
    Unsubscribe(Unsubscribe),
    /// UNSUBACK.
    UnsubAck(PacketId),
    /// PINGREQ.
    PingReq,
    /// PINGRESP.
    PingResp,
    /// DISCONNECT.
    Disconnect,
}

impl Packet {
    /// The packet type.
    pub const fn packet_type(&self) -> PacketType {
        match self {
            Self::Connect(_) => PacketType::Connect,
            Self::ConnAck(_) => PacketType::ConnAck,
            Self::Publish(_) => PacketType::Publish,
            Self::PubAck(_) => PacketType::PubAck,
            Self::Subscribe(_) => PacketType::Subscribe,
            Self::SubAck(_) => PacketType::SubAck,
            Self::Unsubscribe(_) => PacketType::Unsubscribe,
            Self::UnsubAck(_) => PacketType::UnsubAck,
            Self::PingReq => PacketType::PingReq,
            Self::PingResp => PacketType::PingResp,
            Self::Disconnect => PacketType::Disconnect,
        }
    }
}
