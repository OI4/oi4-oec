//! MQTT 3.1.1 without I/O (REQ-MQTT-001, MG 6.3, ADR-0023 Q-026,
//! ADR-0026 amendment C-049): the control packets ([`Packet`]), a symmetric
//! and canonical codec ([`encode`], [`decode`], [`Decoder`]) and the client
//! session as a state machine ([`Session`]: bytes and time in, bytes and
//! [`Event`]s out).
//!
//! Scope of the SDK: QoS 0 and 1 (REQ-MQTT-017), no Retain on what the
//! client publishes (REQ-MQTT-018), no QoS 2 flow. Transport (TCP, TLS,
//! WebSocket) and timers belong to the runtime of the crate `oec`.

mod codec;
mod packet;
mod session;

pub use codec::{
    CodecError, Decoder, MAX_FIELD_LENGTH, MAX_REMAINING_LENGTH, check_topic_filter,
    check_topic_name, decode, encode, encoded_len,
};
pub use packet::{
    ConnAck, Connect, ConnectReturnCode, Delivery, LastWill, Packet, PacketId, PacketType, Publish,
    QoS, SubAck, Subscribe, SubscribeReturnCode, Unsubscribe,
};
pub use session::{
    CloseReason, Credentials, DEFAULT_MAX_IN_FLIGHT, Event, MIN_MAX_PACKET_SIZE, ProtocolError,
    Session, SessionConfig, SessionError, SessionState,
};
