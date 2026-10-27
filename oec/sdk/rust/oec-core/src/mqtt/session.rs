//! The MQTT client session as a sans-IO state machine (REQ-MQTT-001,
//! MG 6.3, ADR-0026 §2).
//!
//! The [`Session`] owns no socket, no timer and no clock. The caller (the
//! runtime of the crate `oec`, C-050) moves bytes and time through it:
//!
//! 1. [`Session::connect`] with the Will of this connection (a new Will per
//!    CONNECT, REQ-LIFE-016), then write [`Session::take_transmit`] to the
//!    transport;
//! 2. feed what the transport read with [`Session::handle_input`];
//! 3. call [`Session::handle_timeout`] at [`Session::poll_timeout`];
//! 4. after each call write [`Session::take_transmit`] and drain
//!    [`Session::poll_event`];
//! 5. on [`Event::Closed`] close the transport (after writing the transmit
//!    buffer, which holds the DISCONNECT of a graceful close only).
//!
//! Time is a monotonic [`Duration`] since an origin the caller chooses.
//!
//! Defaults follow MG 6.3.1: keep alive 60 s (REQ-MQTT-014), Clean Session
//! (REQ-MQTT-015), QoS 0 (REQ-MQTT-017); Retain is never set
//! (REQ-MQTT-018).

use core::fmt;
use core::num::NonZeroU16;
use core::time::Duration;
use std::collections::{BTreeMap, BTreeSet, VecDeque};

use super::codec::{self, CodecError, Decoder, MAX_FIELD_LENGTH, MAX_REMAINING_LENGTH};
use super::packet::{
    ConnAck, Connect, ConnectReturnCode, Delivery, LastWill, Packet, PacketId, PacketType, Publish,
    QoS, SubAck, Subscribe, SubscribeReturnCode, Unsubscribe,
};

/// The smallest MaxPacketSize of the Message Bus (RIG 4.1.1).
pub const MIN_MAX_PACKET_SIZE: usize = 262_144;

/// Default of [`SessionConfig::max_in_flight`].
pub const DEFAULT_MAX_IN_FLIGHT: usize = 64;

/// User Name and Password of CONNECT (REQ-MQTT-009, REQ-MQTT-011). The
/// password never appears in `Debug` output (G-4.5).
#[derive(Clone, PartialEq, Eq)]
pub struct Credentials {
    /// User Name, used as provided (REQ-MQTT-011).
    pub username: String,
    /// Password octets.
    pub password: Option<Vec<u8>>,
}

impl fmt::Debug for Credentials {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("Credentials")
            .field("username", &self.username)
            .field("password", &self.password.as_ref().map(|_| "***"))
            .finish()
    }
}

/// The settings of a [`Session`]; the sans-IO counterpart of the
/// `ConnectionSettings` of the crate `oec` (host, port, CA and client
/// certificate belong to the transport).
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub struct SessionConfig {
    /// MQTT ClientId (REQ-MQTT-013).
    pub client_id: String,
    /// Keep Alive in seconds (REQ-MQTT-014, default 60).
    pub keep_alive: NonZeroU16,
    /// Clean Session (REQ-MQTT-015, default `true`).
    pub clean_session: bool,
    /// User Name and Password (REQ-MQTT-009); `None` with a client
    /// certificate.
    pub credentials: Option<Credentials>,
    /// MaxPacketSize of the Message Bus: the largest MQTT payload
    /// (RIG 4.1.1, 262144 … 268435455, default 262144). The session
    /// refuses larger payloads it should publish (REQ-PAG-005) and closes
    /// the connection on larger packets it receives.
    pub max_packet_size: usize,
    /// The largest number of unacknowledged QoS 1 publications (default
    /// [`DEFAULT_MAX_IN_FLIGHT`]).
    pub max_in_flight: usize,
}

impl SessionConfig {
    /// The defaults of MG 6.3.1 for `client_id`.
    pub fn new(client_id: impl Into<String>) -> Self {
        Self {
            client_id: client_id.into(),
            keep_alive: NonZeroU16::new(60).unwrap_or(NonZeroU16::MIN),
            clean_session: true,
            credentials: None,
            max_packet_size: MIN_MAX_PACKET_SIZE,
            max_in_flight: DEFAULT_MAX_IN_FLIGHT,
        }
    }

    /// Checks the settings.
    ///
    /// # Errors
    ///
    /// [`SessionError::InvalidConfig`]: an empty ClientId without Clean
    /// Session (REQ-MQTT-016, MQTT-3.1.3-7), a MaxPacketSize outside
    /// RIG 4.1.1, `max_in_flight` 0 or above 65535, a field that does not
    /// fit into CONNECT.
    pub fn validate(&self) -> Result<(), SessionError> {
        if self.client_id.is_empty() && !self.clean_session {
            return Err(SessionError::InvalidConfig(
                "a persistent session needs a ClientId (REQ-MQTT-016)",
            ));
        }
        if !(MIN_MAX_PACKET_SIZE..=MAX_REMAINING_LENGTH).contains(&self.max_packet_size) {
            return Err(SessionError::InvalidConfig(
                "MaxPacketSize outside 262144 … 268435455 (RIG 4.1.1)",
            ));
        }
        if self.max_in_flight == 0 || self.max_in_flight > usize::from(u16::MAX) {
            return Err(SessionError::InvalidConfig(
                "max_in_flight outside 1 … 65535",
            ));
        }
        let probe = Packet::Connect(self.connect_packet(None));
        codec::encode(&probe, &mut Vec::new()).map_err(SessionError::Codec)?;
        Ok(())
    }

    fn connect_packet(&self, will: Option<LastWill>) -> Connect {
        Connect {
            client_id: self.client_id.clone(),
            keep_alive: self.keep_alive.get(),
            clean_session: self.clean_session,
            will,
            username: self.credentials.as_ref().map(|c| c.username.clone()),
            password: self.credentials.as_ref().and_then(|c| c.password.clone()),
        }
    }

    /// The decoder limit for the Remaining Length: a PUBLISH with the
    /// largest topic, a packet id and a payload of MaxPacketSize.
    fn inbound_limit(&self) -> usize {
        self.max_packet_size
            .saturating_add(2 + MAX_FIELD_LENGTH + 2)
            .min(MAX_REMAINING_LENGTH)
    }
}

/// An operation of the [`Session`] that is not possible.
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub enum SessionError {
    /// The [`SessionConfig`] is invalid.
    InvalidConfig(&'static str),
    /// The session is not connected.
    NotConnected,
    /// [`Session::connect`] while connecting or connected.
    AlreadyConnected,
    /// The payload exceeds the MaxPacketSize (RIG 4.1.1, REQ-PAG-005).
    PayloadTooLarge {
        /// Payload size in bytes.
        size: usize,
        /// The MaxPacketSize.
        limit: usize,
    },
    /// All QoS 1 slots ([`SessionConfig::max_in_flight`]) or all packet
    /// identifiers are in use.
    InFlightLimit,
    /// The packet can not be encoded (e.g. a wildcard in a topic name).
    Codec(CodecError),
}

impl fmt::Display for SessionError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::InvalidConfig(why) => write!(f, "invalid session configuration: {why}"),
            Self::NotConnected => f.write_str("the session is not connected"),
            Self::AlreadyConnected => f.write_str("the session is already connecting or connected"),
            Self::PayloadTooLarge { size, limit } => write!(
                f,
                "payload of {size} bytes exceeds the MaxPacketSize of {limit} bytes (REQ-PAG-005)"
            ),
            Self::InFlightLimit => f.write_str("too many unacknowledged packets"),
            Self::Codec(e) => e.fmt(f),
        }
    }
}

impl std::error::Error for SessionError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            Self::Codec(e) => Some(e),
            _ => None,
        }
    }
}

/// A violation of MQTT 3.1.1 by the broker; the session closes the
/// connection without DISCONNECT (MQTT 4.8), so the broker publishes the
/// Will.
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub enum ProtocolError {
    /// The bytes are no valid packet.
    Codec(CodecError),
    /// A packet the broker must not send in this state: anything but
    /// CONNACK first (MQTT-3.2.0-1), a second CONNACK, a client-only packet.
    Unexpected(PacketType),
    /// Session Present although Clean Session was requested
    /// (MQTT-3.2.2-1).
    SessionPresent,
    /// SUBACK with another number of return codes than topic filters
    /// (MQTT-3.9.3-1).
    SubAckMismatch {
        /// Topic filters of the SUBSCRIBE.
        expected: usize,
        /// Return codes of the SUBACK.
        actual: usize,
    },
}

impl fmt::Display for ProtocolError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Codec(e) => e.fmt(f),
            Self::Unexpected(t) => write!(f, "unexpected {t} from the broker"),
            Self::SessionPresent => {
                f.write_str("Session Present with Clean Session (MQTT-3.2.2-1)")
            }
            Self::SubAckMismatch { expected, actual } => write!(
                f,
                "SUBACK with {actual} return codes for {expected} topic filters (MQTT-3.9.3-1)"
            ),
        }
    }
}

impl std::error::Error for ProtocolError {}

/// Why the connection ended ([`Event::Closed`]).
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub enum CloseReason {
    /// [`Session::disconnect`]: DISCONNECT is in the transmit buffer, the
    /// broker discards the Will (MG 6.3.4).
    Graceful,
    /// The broker refused the connection (CONNACK return code ≠ 0).
    Refused(ConnectReturnCode),
    /// No CONNACK within the keep alive interval after CONNECT.
    ConnectTimeout,
    /// No answer to PINGREQ within the keep alive interval (MQTT 3.1.2.10).
    KeepAliveTimeout,
    /// The broker violated MQTT 3.1.1.
    Protocol(ProtocolError),
    /// The transport reported the end of the connection
    /// ([`Session::connection_lost`]).
    TransportClosed,
}

impl fmt::Display for CloseReason {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Graceful => f.write_str("closed by the application"),
            Self::Refused(code) => write!(f, "connection refused: {code}"),
            Self::ConnectTimeout => f.write_str("no CONNACK within the keep alive interval"),
            Self::KeepAliveTimeout => f.write_str("no PINGRESP within the keep alive interval"),
            Self::Protocol(e) => write!(f, "protocol violation: {e}"),
            Self::TransportClosed => f.write_str("transport closed"),
        }
    }
}

/// What happened in the session ([`Session::poll_event`]).
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub enum Event {
    /// CONNACK accepted the connection; the application publishes its
    /// Birth now (REQ-LIFE-002).
    Connected {
        /// Session Present of CONNACK.
        session_present: bool,
    },
    /// A PUBLISH of the broker (already acknowledged for QoS 1).
    Message(Publish),
    /// PUBACK of a QoS 1 publication.
    Published(PacketId),
    /// SUBACK.
    Subscribed {
        /// Packet Identifier of the SUBSCRIBE.
        packet_id: PacketId,
        /// Every topic filter with its return code; failed filters are not
        /// restored after a reconnect.
        results: Vec<(String, SubscribeReturnCode)>,
        /// `true` for the automatic restore after a reconnect without
        /// session (its packet id was not returned by
        /// [`Session::subscribe`]).
        restored: bool,
    },
    /// UNSUBACK.
    Unsubscribed {
        /// Packet Identifier of the UNSUBSCRIBE.
        packet_id: PacketId,
        /// The topic filters.
        filters: Vec<String>,
    },
    /// QoS 1 publications without PUBACK that are lost: the broker has no
    /// session for them after the reconnect (Session Present 0). The
    /// application may publish them again.
    Dropped(Vec<PacketId>),
    /// The connection ended; close the transport.
    Closed(CloseReason),
}

/// The state of a [`Session`].
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SessionState {
    /// No connection (initial, after [`Event::Closed`]).
    Disconnected,
    /// CONNECT sent, CONNACK pending.
    Connecting,
    /// CONNACK accepted.
    Connected,
}

#[derive(Debug, Clone)]
enum Pending {
    Subscribe {
        filters: Vec<(String, QoS)>,
        restored: bool,
    },
    Unsubscribe(Vec<String>),
}

/// The MQTT client session (see the [module documentation](crate::mqtt)).
#[derive(Debug)]
pub struct Session {
    config: SessionConfig,
    state: SessionState,
    decoder: Decoder,
    transmit: Vec<u8>,
    events: VecDeque<Event>,
    next_id: u16,
    seq: u64,
    /// QoS 1 publications without PUBACK, with their send order.
    in_flight: BTreeMap<PacketId, (u64, Publish)>,
    /// SUBSCRIBE/UNSUBSCRIBE without acknowledgement, with their order.
    pending: BTreeMap<PacketId, (u64, Pending)>,
    /// Subscriptions granted by the broker with the requested QoS
    /// (restored after a reconnect without session).
    subscriptions: BTreeMap<String, QoS>,
    since: Duration,
    last_sent: Duration,
    last_received: Duration,
    ping_sent: Option<Duration>,
}

impl Session {
    /// A disconnected session.
    ///
    /// # Errors
    ///
    /// [`SessionError::InvalidConfig`] or [`SessionError::Codec`] (see
    /// [`SessionConfig::validate`]).
    pub fn new(config: SessionConfig) -> Result<Self, SessionError> {
        config.validate()?;
        Ok(Self {
            decoder: Decoder::new(config.inbound_limit()),
            config,
            state: SessionState::Disconnected,
            transmit: Vec::new(),
            events: VecDeque::new(),
            next_id: 0,
            seq: 0,
            in_flight: BTreeMap::new(),
            pending: BTreeMap::new(),
            subscriptions: BTreeMap::new(),
            since: Duration::ZERO,
            last_sent: Duration::ZERO,
            last_received: Duration::ZERO,
            ping_sent: None,
        })
    }

    /// The settings.
    pub fn config(&self) -> &SessionConfig {
        &self.config
    }

    /// Replaces User Name and Password, e.g. after the platform renewed
    /// the secret storage (GL-ADR-022). They apply to the next CONNECT;
    /// the current connection is not touched.
    ///
    /// # Errors
    ///
    /// See [`SessionConfig::validate`]; the previous credentials stay.
    pub fn set_credentials(
        &mut self,
        credentials: Option<Credentials>,
    ) -> Result<(), SessionError> {
        let mut config = self.config.clone();
        config.credentials = credentials;
        config.validate()?;
        self.config = config;
        Ok(())
    }

    /// The state.
    pub fn state(&self) -> SessionState {
        self.state
    }

    /// Starts a connection on a fresh transport: queues CONNECT with
    /// `will`. Every CONNECT carries a new Will (REQ-LIFE-008,
    /// REQ-LIFE-016); see `lifecycle::Lifecycle::will`.
    ///
    /// # Errors
    ///
    /// [`SessionError::AlreadyConnected`], [`SessionError::PayloadTooLarge`]
    /// for a Will above the MaxPacketSize, [`SessionError::Codec`] for an
    /// invalid Will topic.
    pub fn connect(&mut self, will: LastWill, now: Duration) -> Result<(), SessionError> {
        if self.state != SessionState::Disconnected {
            return Err(SessionError::AlreadyConnected);
        }
        self.check_payload(will.payload.len())?;
        let mut out = Vec::new();
        codec::encode(
            &Packet::Connect(self.config.connect_packet(Some(will))),
            &mut out,
        )
        .map_err(SessionError::Codec)?;
        self.decoder.reset();
        self.transmit = out;
        self.state = SessionState::Connecting;
        self.since = now;
        self.last_sent = now;
        self.last_received = now;
        self.ping_sent = None;
        Ok(())
    }

    /// Processes bytes the transport read.
    pub fn handle_input(&mut self, bytes: &[u8], now: Duration) {
        if self.state == SessionState::Disconnected {
            return;
        }
        self.decoder.feed(bytes);
        while self.state != SessionState::Disconnected {
            match self.decoder.next_packet() {
                Ok(Some(packet)) => {
                    self.last_received = now;
                    self.ping_sent = None;
                    self.handle_packet(packet, now);
                }
                Ok(None) => break,
                Err(e) => self.close(CloseReason::Protocol(ProtocolError::Codec(e))),
            }
        }
    }

    /// The transport is closed or broken: the session is disconnected.
    pub fn connection_lost(&mut self) {
        if self.state != SessionState::Disconnected {
            self.close(CloseReason::TransportClosed);
        }
    }

    /// The next instant [`Self::handle_timeout`] has to run; `None` while
    /// disconnected.
    pub fn poll_timeout(&self) -> Option<Duration> {
        let ka = self.keep_alive();
        match self.state {
            SessionState::Disconnected => None,
            SessionState::Connecting => Some(self.since + ka),
            SessionState::Connected => Some(match self.ping_sent {
                Some(t) => t + ka,
                None => self.last_sent.min(self.last_received) + ka,
            }),
        }
    }

    /// Keep alive (MQTT 3.1.2.10): sends PINGREQ when nothing was sent or
    /// received for the keep alive interval and closes the connection
    /// when CONNACK or PINGRESP does not arrive within it.
    pub fn handle_timeout(&mut self, now: Duration) {
        let ka = self.keep_alive();
        match self.state {
            SessionState::Disconnected => {}
            SessionState::Connecting => {
                if now >= self.since + ka {
                    self.close(CloseReason::ConnectTimeout);
                }
            }
            SessionState::Connected => match self.ping_sent {
                Some(t) if now >= t + ka => self.close(CloseReason::KeepAliveTimeout),
                Some(_) => {}
                None => {
                    if now >= self.last_sent.min(self.last_received) + ka {
                        self.queue(&Packet::PingReq, now);
                        self.ping_sent = Some(now);
                    }
                }
            },
        }
    }

    /// Queues a PUBLISH without Retain (REQ-MQTT-018). Returns the packet
    /// id of a QoS 1 publication; [`Event::Published`] reports its PUBACK.
    ///
    /// # Errors
    ///
    /// [`SessionError::NotConnected`] before CONNACK (the Birth is the
    /// first publication, REQ-LIFE-002), [`SessionError::PayloadTooLarge`]
    /// (REQ-PAG-005), [`SessionError::InFlightLimit`],
    /// [`SessionError::Codec`] for an invalid topic name.
    pub fn publish(
        &mut self,
        topic: impl Into<String>,
        payload: Vec<u8>,
        qos: QoS,
        now: Duration,
    ) -> Result<Option<PacketId>, SessionError> {
        if self.state != SessionState::Connected {
            return Err(SessionError::NotConnected);
        }
        self.check_payload(payload.len())?;
        let topic = topic.into();
        codec::check_topic_name(PacketType::Publish, &topic).map_err(SessionError::Codec)?;
        let delivery = match qos {
            QoS::AtMostOnce => Delivery::AtMostOnce,
            QoS::AtLeastOnce => {
                if self.in_flight.len() >= self.config.max_in_flight {
                    return Err(SessionError::InFlightLimit);
                }
                Delivery::AtLeastOnce {
                    packet_id: self.allocate_id()?,
                    dup: false,
                }
            }
        };
        let publish = Publish {
            topic,
            payload,
            delivery,
            retain: false,
        };
        self.queue(&Packet::Publish(publish.clone()), now);
        let id = delivery.packet_id();
        if let Some(id) = id {
            self.seq += 1;
            self.in_flight.insert(id, (self.seq, publish));
        }
        Ok(id)
    }

    /// Subscribes `filters`. Before CONNACK the SUBSCRIBE is held back and
    /// sent once connected; it is sent again after a reconnect until its
    /// SUBACK arrives ([`Event::Subscribed`]).
    ///
    /// # Errors
    ///
    /// [`SessionError::InFlightLimit`] if no packet id is free,
    /// [`SessionError::Codec`] for an invalid filter or none.
    pub fn subscribe(
        &mut self,
        filters: Vec<(String, QoS)>,
        now: Duration,
    ) -> Result<PacketId, SessionError> {
        self.request(
            Pending::Subscribe {
                filters,
                restored: false,
            },
            now,
        )
    }

    /// Unsubscribes `filters`, like [`Self::subscribe`]
    /// ([`Event::Unsubscribed`]).
    ///
    /// # Errors
    ///
    /// As [`Self::subscribe`].
    pub fn unsubscribe(
        &mut self,
        filters: Vec<String>,
        now: Duration,
    ) -> Result<PacketId, SessionError> {
        self.request(Pending::Unsubscribe(filters), now)
    }

    /// Closes the connection gracefully: queues DISCONNECT, so the broker
    /// discards the Will (MG 6.3.4). Publish the Close message before
    /// (REQ-LIFE-005). [`Event::Closed`] with [`CloseReason::Graceful`]
    /// follows.
    ///
    /// # Errors
    ///
    /// [`SessionError::NotConnected`] while disconnected.
    pub fn disconnect(&mut self, now: Duration) -> Result<(), SessionError> {
        if self.state == SessionState::Disconnected {
            return Err(SessionError::NotConnected);
        }
        self.queue(&Packet::Disconnect, now);
        self.state = SessionState::Disconnected;
        self.events.push_back(Event::Closed(CloseReason::Graceful));
        Ok(())
    }

    /// The bytes to write to the transport, in order.
    pub fn take_transmit(&mut self) -> Vec<u8> {
        core::mem::take(&mut self.transmit)
    }

    /// `true` if [`Self::take_transmit`] has bytes.
    pub fn has_transmit(&self) -> bool {
        !self.transmit.is_empty()
    }

    /// The next event.
    pub fn poll_event(&mut self) -> Option<Event> {
        self.events.pop_front()
    }

    /// Unacknowledged QoS 1 publications.
    pub fn in_flight(&self) -> usize {
        self.in_flight.len()
    }

    /// The subscriptions granted by the broker, with the requested QoS.
    pub fn subscriptions(&self) -> impl Iterator<Item = (&str, QoS)> {
        self.subscriptions.iter().map(|(f, q)| (f.as_str(), *q))
    }

    // -- internals ----------------------------------------------------------

    fn keep_alive(&self) -> Duration {
        Duration::from_secs(u64::from(self.config.keep_alive.get()))
    }

    fn check_payload(&self, size: usize) -> Result<(), SessionError> {
        if size > self.config.max_packet_size {
            return Err(SessionError::PayloadTooLarge {
                size,
                limit: self.config.max_packet_size,
            });
        }
        Ok(())
    }

    fn close(&mut self, reason: CloseReason) {
        // No DISCONNECT: the broker publishes the Will (MG 6.3.5).
        self.transmit.clear();
        self.decoder.reset();
        self.state = SessionState::Disconnected;
        self.ping_sent = None;
        self.events.push_back(Event::Closed(reason));
    }

    fn queue(&mut self, packet: &Packet, now: Duration) {
        // Every packet the session builds was checked before.
        if codec::encode(packet, &mut self.transmit).is_ok() {
            self.last_sent = now;
        } else {
            debug_assert!(false, "session built an invalid {}", packet.packet_type());
        }
    }

    fn allocate_id(&mut self) -> Result<PacketId, SessionError> {
        for _ in 0..u16::MAX {
            self.next_id = self.next_id.checked_add(1).unwrap_or(1);
            let id = PacketId::new(self.next_id).ok_or(SessionError::InFlightLimit)?;
            if !self.in_flight.contains_key(&id) && !self.pending.contains_key(&id) {
                return Ok(id);
            }
        }
        Err(SessionError::InFlightLimit)
    }

    fn request_packet(id: PacketId, pending: &Pending) -> Packet {
        match pending {
            Pending::Subscribe { filters, .. } => Packet::Subscribe(Subscribe {
                packet_id: id,
                filters: filters.clone(),
            }),
            Pending::Unsubscribe(filters) => Packet::Unsubscribe(Unsubscribe {
                packet_id: id,
                filters: filters.clone(),
            }),
        }
    }

    fn request(&mut self, pending: Pending, now: Duration) -> Result<PacketId, SessionError> {
        let id = self.allocate_id()?;
        let packet = Self::request_packet(id, &pending);
        let mut probe = Vec::new();
        codec::encode(&packet, &mut probe).map_err(SessionError::Codec)?;
        if self.state == SessionState::Connected {
            self.transmit.extend_from_slice(&probe);
            self.last_sent = now;
        }
        self.seq += 1;
        self.pending.insert(id, (self.seq, pending));
        Ok(id)
    }

    fn handle_packet(&mut self, packet: Packet, now: Duration) {
        let unexpected = |t| CloseReason::Protocol(ProtocolError::Unexpected(t));
        match (self.state, packet) {
            (SessionState::Connecting, Packet::ConnAck(ack)) => self.on_connack(ack, now),
            (SessionState::Connected, Packet::Publish(p)) => {
                if let Some(id) = p.delivery.packet_id() {
                    self.queue(&Packet::PubAck(id), now);
                }
                self.events.push_back(Event::Message(p));
            }
            (SessionState::Connected, Packet::PubAck(id)) => {
                // A PUBACK for an unknown id (e.g. a duplicate) is ignored.
                if self.in_flight.remove(&id).is_some() {
                    self.events.push_back(Event::Published(id));
                }
            }
            (SessionState::Connected, Packet::SubAck(ack)) => self.on_suback(ack),
            (SessionState::Connected, Packet::UnsubAck(id)) => {
                if let Some((_, Pending::Unsubscribe(filters))) = self.pending.get(&id) {
                    let filters = filters.clone();
                    self.pending.remove(&id);
                    for f in &filters {
                        self.subscriptions.remove(f);
                    }
                    self.events.push_back(Event::Unsubscribed {
                        packet_id: id,
                        filters,
                    });
                }
            }
            (SessionState::Connected, Packet::PingResp) => {}
            (_, p) => self.close(unexpected(p.packet_type())),
        }
    }

    fn on_connack(&mut self, ack: ConnAck, now: Duration) {
        if ack.code != ConnectReturnCode::Accepted {
            self.close(CloseReason::Refused(ack.code));
            return;
        }
        if ack.session_present && self.config.clean_session {
            self.close(CloseReason::Protocol(ProtocolError::SessionPresent));
            return;
        }
        self.state = SessionState::Connected;
        self.events.push_back(Event::Connected {
            session_present: ack.session_present,
        });
        if ack.session_present {
            // MQTT-4.4.0-1: re-send unacknowledged PUBLISH with DUP, in order.
            let mut resend: Vec<_> = self.in_flight.values().cloned().collect();
            resend.sort_by_key(|(seq, _)| *seq);
            for (_, mut p) in resend {
                if let Delivery::AtLeastOnce { dup, .. } = &mut p.delivery {
                    *dup = true;
                }
                self.queue(&Packet::Publish(p), now);
            }
        } else {
            if !self.in_flight.is_empty() {
                let ids = core::mem::take(&mut self.in_flight).into_keys().collect();
                self.events.push_back(Event::Dropped(ids));
            }
            self.restore_subscriptions(now);
        }
        let mut pending: Vec<_> = self
            .pending
            .iter()
            .map(|(id, (seq, p))| (*seq, *id, p.clone()))
            .collect();
        pending.sort_by_key(|(seq, ..)| *seq);
        for (_, id, p) in pending {
            self.queue(&Self::request_packet(id, &p), now);
        }
    }

    /// Re-subscribes the granted subscriptions the broker forgot (Session
    /// Present 0), in SUBSCRIBE packets below the MaxPacketSize.
    fn restore_subscriptions(&mut self, now: Duration) {
        let requested: BTreeSet<&String> = self
            .pending
            .values()
            .filter_map(|(_, p)| match p {
                Pending::Subscribe { filters, .. } => Some(filters.iter().map(|(f, _)| f)),
                Pending::Unsubscribe(_) => None,
            })
            .flatten()
            .collect();
        let all: Vec<(String, QoS)> = self
            .subscriptions
            .iter()
            .filter(|(f, _)| !requested.contains(f))
            .map(|(f, q)| (f.clone(), *q))
            .collect();
        let mut chunk = Vec::new();
        let mut size = 2;
        for (f, q) in all {
            let entry = 2 + f.len() + 1;
            if !chunk.is_empty() && size + entry > self.config.max_packet_size {
                self.send_restore(core::mem::take(&mut chunk), now);
                size = 2;
            }
            size += entry;
            chunk.push((f, q));
        }
        if !chunk.is_empty() {
            self.send_restore(chunk, now);
        }
    }

    fn send_restore(&mut self, filters: Vec<(String, QoS)>, now: Duration) {
        let Ok(id) = self.allocate_id() else {
            // No free packet id: the subscriptions stay recorded and are
            // tried again on the next reconnect.
            return;
        };
        let pending = Pending::Subscribe {
            filters,
            restored: true,
        };
        self.queue(&Self::request_packet(id, &pending), now);
        self.seq += 1;
        self.pending.insert(id, (self.seq, pending));
    }

    fn on_suback(&mut self, ack: SubAck) {
        let Some((_, Pending::Subscribe { filters, restored })) = self.pending.get(&ack.packet_id)
        else {
            return;
        };
        if filters.len() != ack.codes.len() {
            let expected = filters.len();
            self.close(CloseReason::Protocol(ProtocolError::SubAckMismatch {
                expected,
                actual: ack.codes.len(),
            }));
            return;
        }
        let restored = *restored;
        let Some((_, Pending::Subscribe { filters, .. })) = self.pending.remove(&ack.packet_id)
        else {
            return;
        };
        let results: Vec<_> = filters
            .into_iter()
            .zip(ack.codes)
            .map(|((f, requested), code)| {
                match code {
                    SubscribeReturnCode::Success(_) => {
                        self.subscriptions.insert(f.clone(), requested);
                    }
                    SubscribeReturnCode::Failure => {
                        self.subscriptions.remove(&f);
                    }
                }
                (f, code)
            })
            .collect();
        self.events.push_back(Event::Subscribed {
            packet_id: ack.packet_id,
            results,
            restored,
        });
    }
}
