//! Property tests of the MQTT codec and the session (ADR-0023 amendment
//! C-049: proptest as the fuzzing tool on stable Rust).
//!
//! - Codec: encode and decode are inverse and canonical; arbitrary and
//!   mutated bytes never panic and decode only into packets that encode
//!   back to the same bytes; any chunking of a stream yields the same
//!   packets.
//! - Session: random operations, broker packets, garbage and timeouts never
//!   panic; the session only transmits complete client packets and never a
//!   PUBLISH with Retain (REQ-MQTT-018) or more QoS 1 in flight than
//!   configured.

use std::time::Duration;

use oec_core::mqtt::{
    ConnAck, Connect, ConnectReturnCode, Decoder, Delivery, Event, LastWill, MAX_REMAINING_LENGTH,
    Packet, PacketId, Publish, QoS, Session, SessionConfig, SessionState, SubAck, Subscribe,
    SubscribeReturnCode, Unsubscribe, decode, encode, encoded_len,
};
use proptest::collection::vec;
use proptest::prelude::*;

// ---------------------------------------------------------------------------
// Strategies

fn qos() -> impl Strategy<Value = QoS> {
    prop_oneof![Just(QoS::AtMostOnce), Just(QoS::AtLeastOnce)]
}

fn packet_id() -> impl Strategy<Value = PacketId> {
    (1..=u16::MAX).prop_map(|n| PacketId::new(n).unwrap())
}

/// Strings that are mostly topic-like, sometimes with wildcards, U+0000 or
/// any other character: valid and invalid input for the same property.
fn text() -> impl Strategy<Value = String> {
    prop_oneof![
        4 => "[a-z0-9/+#]{0,12}",
        1 => "(?s).{0,8}",
    ]
}

fn bytes(max: usize) -> impl Strategy<Value = Vec<u8>> {
    vec(any::<u8>(), 0..max)
}

fn valid_topic() -> impl Strategy<Value = String> {
    "[A-Za-z0-9_.-]{1,6}(/[A-Za-z0-9_.-]{0,6}){0,4}"
}

fn valid_filter() -> impl Strategy<Value = String> {
    prop_oneof![
        "[a-z]{1,4}(/([a-z]{1,4}|\\+)){0,3}(/#)?",
        Just("#".to_owned()),
        Just("+".to_owned()),
    ]
}

fn return_code() -> impl Strategy<Value = ConnectReturnCode> {
    prop_oneof![
        Just(ConnectReturnCode::Accepted),
        Just(ConnectReturnCode::UnacceptableProtocolVersion),
        Just(ConnectReturnCode::IdentifierRejected),
        Just(ConnectReturnCode::ServerUnavailable),
        Just(ConnectReturnCode::BadUserNameOrPassword),
        Just(ConnectReturnCode::NotAuthorized),
    ]
}

fn suback_code() -> impl Strategy<Value = SubscribeReturnCode> {
    prop_oneof![
        qos().prop_map(SubscribeReturnCode::Success),
        Just(SubscribeReturnCode::Failure),
    ]
}

fn delivery() -> impl Strategy<Value = Delivery> {
    prop_oneof![
        Just(Delivery::AtMostOnce),
        (packet_id(), any::<bool>())
            .prop_map(|(packet_id, dup)| Delivery::AtLeastOnce { packet_id, dup }),
    ]
}

/// Any packet, built from `topic` and `filter` strings.
fn packet_from<T, F>(topic: fn() -> T, filter: fn() -> F) -> impl Strategy<Value = Packet>
where
    T: Strategy<Value = String> + 'static,
    F: Strategy<Value = String> + 'static,
{
    let will = (topic(), bytes(32), qos()).prop_map(|(topic, payload, qos)| LastWill {
        topic,
        payload,
        qos,
    });
    let connect = (
        text(),
        any::<u16>(),
        any::<bool>(),
        proptest::option::of(will),
        proptest::option::of(text()),
        proptest::option::of(bytes(16)),
    )
        .prop_map(
            |(client_id, keep_alive, clean_session, will, username, password)| {
                Packet::Connect(Connect {
                    client_id,
                    keep_alive,
                    clean_session,
                    will,
                    username,
                    password,
                })
            },
        );
    let publish = (topic(), bytes(64), delivery(), any::<bool>()).prop_map(
        |(topic, payload, delivery, retain)| {
            Packet::Publish(Publish {
                topic,
                payload,
                delivery,
                retain,
            })
        },
    );
    prop_oneof![
        connect,
        (any::<bool>(), return_code()).prop_map(|(session_present, code)| Packet::ConnAck(
            ConnAck {
                session_present,
                code
            }
        )),
        publish,
        packet_id().prop_map(Packet::PubAck),
        (packet_id(), vec((filter(), qos()), 0..4)).prop_map(|(packet_id, filters)| {
            Packet::Subscribe(Subscribe { packet_id, filters })
        }),
        (packet_id(), vec(suback_code(), 0..4))
            .prop_map(|(packet_id, codes)| Packet::SubAck(SubAck { packet_id, codes })),
        (packet_id(), vec(filter(), 0..4)).prop_map(|(packet_id, filters)| {
            Packet::Unsubscribe(Unsubscribe { packet_id, filters })
        }),
        packet_id().prop_map(Packet::UnsubAck),
        Just(Packet::PingReq),
        Just(Packet::PingResp),
        Just(Packet::Disconnect),
    ]
}

fn any_packet() -> impl Strategy<Value = Packet> {
    packet_from(text, text)
}

/// Packets that keep every rule of MQTT 3.1.1 (and of this codec).
fn valid_packet() -> impl Strategy<Value = Packet> {
    packet_from(valid_topic, valid_filter).prop_map(|mut p| {
        match &mut p {
            Packet::Connect(c) => {
                c.client_id.retain(|ch| ch != '\0');
                if let Some(u) = &mut c.username {
                    u.retain(|ch| ch != '\0');
                }
                if c.username.is_none() {
                    c.password = None;
                }
            }
            Packet::ConnAck(a) if a.code != ConnectReturnCode::Accepted => {
                a.session_present = false;
            }
            Packet::Subscribe(s) if s.filters.is_empty() => {
                s.filters.push(("#".into(), QoS::AtMostOnce));
            }
            Packet::SubAck(s) if s.codes.is_empty() => s.codes.push(SubscribeReturnCode::Failure),
            Packet::Unsubscribe(u) if u.filters.is_empty() => u.filters.push("#".into()),
            _ => {}
        }
        p
    })
}

fn encoded(p: &Packet) -> Vec<u8> {
    let mut out = Vec::new();
    encode(p, &mut out).unwrap();
    out
}

// ---------------------------------------------------------------------------
// Codec

proptest! {
    #![proptest_config(ProptestConfig::with_cases(1024))]

    /// encode Ok ⇔ decode gives the packet back; on error nothing is
    /// written. Every proper prefix needs more input.
    #[test]
    fn encode_and_decode_are_inverse(p in any_packet()) {
        let mut out = vec![0xAA];
        match encode(&p, &mut out) {
            Ok(n) => {
                prop_assert_eq!(n, out.len() - 1);
                prop_assert_eq!(encoded_len(&p), Some(n));
                let (back, used) = decode(&out[1..], MAX_REMAINING_LENGTH).unwrap().unwrap();
                prop_assert_eq!(&back, &p);
                prop_assert_eq!(used, n);
                for k in 0..n {
                    prop_assert_eq!(decode(&out[1..=k], MAX_REMAINING_LENGTH), Ok(None));
                }
            }
            Err(e) => {
                prop_assert_eq!(out, vec![0xAA]);
                prop_assert!(!e.rule().is_empty());
            }
        }
    }

    /// Valid packets always encode.
    #[test]
    fn valid_packets_encode(p in valid_packet()) {
        let bytes = encoded(&p);
        prop_assert_eq!(decode(&bytes, bytes.len()).unwrap(), Some((p, bytes.len())));
    }

    /// Arbitrary bytes never panic; what decodes is canonical.
    #[test]
    fn arbitrary_bytes_decode_canonically(input in bytes(64), limit in 0usize..128) {
        if let Ok(Some((p, n))) = decode(&input, limit) {
            prop_assert_eq!(encoded(&p), &input[..n]);
            prop_assert!(n - 2 <= limit);
        }
    }

    /// Mutated valid packets never panic; what decodes is canonical.
    #[test]
    fn mutated_packets_decode_canonically(
        p in valid_packet(),
        at in any::<prop::sample::Index>(),
        xor in 1u8..,
        cut in any::<prop::sample::Index>(),
    ) {
        let mut bytes = encoded(&p);
        let i = at.index(bytes.len());
        bytes[i] ^= xor;
        bytes.truncate(cut.index(bytes.len()) + 1);
        if let Ok(Some((q, n))) = decode(&bytes, MAX_REMAINING_LENGTH) {
            prop_assert_eq!(encoded(&q), &bytes[..n]);
        }
    }

    /// Any chunking of a stream yields the same packets.
    #[test]
    fn chunking_does_not_matter(
        packets in vec(valid_packet(), 1..8),
        cuts in vec(1usize..16, 1..64),
    ) {
        let stream: Vec<u8> = packets.iter().flat_map(encoded).collect();
        let mut decoder = Decoder::new(MAX_REMAINING_LENGTH);
        let mut out = Vec::new();
        let mut rest = &stream[..];
        let mut cuts = cuts.into_iter().cycle();
        while !rest.is_empty() {
            let n = cuts.next().unwrap().min(rest.len());
            decoder.feed(&rest[..n]);
            rest = &rest[n..];
            while let Some(p) = decoder.next_packet().unwrap() {
                out.push(p);
            }
        }
        prop_assert_eq!(decoder.buffered(), 0);
        prop_assert_eq!(out, packets);
    }
}

// ---------------------------------------------------------------------------
// Session

#[derive(Debug, Clone)]
enum Op {
    Connect,
    Broker(Packet),
    Garbage(Vec<u8>),
    Publish(String, usize, QoS),
    Subscribe(Vec<(String, QoS)>),
    Unsubscribe(Vec<String>),
    Disconnect,
    Lost,
    Wait(u64),
    Timeout,
}

/// Packets a broker sends, with ids of a small range so that they hit the
/// ids of the session.
fn broker_packet() -> impl Strategy<Value = Packet> {
    let id = (1u16..6).prop_map(|n| PacketId::new(n).unwrap());
    prop_oneof![
        3 => (any::<bool>(), return_code()).prop_map(|(sp, code)| Packet::ConnAck(ConnAck {
            session_present: sp && code == ConnectReturnCode::Accepted,
            code,
        })),
        2 => id.clone().prop_map(Packet::PubAck),
        2 => (id.clone(), vec(suback_code(), 1..3))
            .prop_map(|(packet_id, codes)| Packet::SubAck(SubAck { packet_id, codes })),
        1 => id.clone().prop_map(Packet::UnsubAck),
        1 => Just(Packet::PingResp),
        2 => (valid_topic(), bytes(8), delivery(), any::<bool>()).prop_map(
            |(topic, payload, delivery, retain)| Packet::Publish(Publish {
                topic,
                payload,
                delivery,
                retain,
            })
        ),
        1 => Just(Packet::PingReq),
    ]
}

fn op() -> impl Strategy<Value = Op> {
    prop_oneof![
        3 => Just(Op::Connect),
        6 => broker_packet().prop_map(Op::Broker),
        1 => bytes(8).prop_map(Op::Garbage),
        4 => (text(), 0usize..300_000, qos()).prop_map(|(t, n, q)| Op::Publish(t, n, q)),
        2 => vec((text(), qos()), 0..3).prop_map(Op::Subscribe),
        1 => vec(text(), 0..3).prop_map(Op::Unsubscribe),
        1 => Just(Op::Disconnect),
        1 => Just(Op::Lost),
        3 => (0u64..100).prop_map(Op::Wait),
        2 => Just(Op::Timeout),
    ]
}

/// Client packets of the transmit buffer; panics on anything else.
fn client_packets(bytes: &[u8]) -> Vec<Packet> {
    let mut out = Vec::new();
    let mut rest = bytes;
    while !rest.is_empty() {
        let (p, n) = decode(rest, MAX_REMAINING_LENGTH)
            .unwrap()
            .expect("complete packets only");
        out.push(p);
        rest = &rest[n..];
    }
    out
}

proptest! {
    #![proptest_config(ProptestConfig::with_cases(512))]

    #[test]
    fn session_survives_anything(
        clean in any::<bool>(),
        max_in_flight in 1usize..4,
        keep_alive in 1u16..120,
        ops in vec(op(), 0..60),
    ) {
        let mut config = SessionConfig::new("prop");
        config.clean_session = clean;
        config.max_in_flight = max_in_flight;
        config.keep_alive = keep_alive.try_into().unwrap();
        let mut s = Session::new(config).unwrap();
        let mut now = Duration::ZERO;
        for op in ops {
            match op {
                Op::Connect => {
                    let will = LastWill {
                        topic: "w".into(),
                        payload: Vec::new(),
                        qos: QoS::AtMostOnce,
                    };
                    let was = s.state();
                    prop_assert_eq!(s.connect(will, now).is_ok(), was == SessionState::Disconnected);
                }
                Op::Broker(p) => s.handle_input(&encoded(&p), now),
                Op::Garbage(b) => s.handle_input(&b, now),
                Op::Publish(t, n, q) => {
                    let _ = s.publish(&t, vec![0; n], q, now);
                }
                Op::Subscribe(f) => {
                    let _ = s.subscribe(f, now);
                }
                Op::Unsubscribe(f) => {
                    let _ = s.unsubscribe(f, now);
                }
                Op::Disconnect => {
                    let _ = s.disconnect(now);
                }
                Op::Lost => s.connection_lost(),
                Op::Wait(secs) => now += Duration::from_secs(secs),
                Op::Timeout => {
                    if let Some(at) = s.poll_timeout() {
                        now = now.max(at);
                        s.handle_timeout(now);
                    }
                }
            }
            prop_assert!(s.in_flight() <= max_in_flight);
            for p in client_packets(&s.take_transmit()) {
                match p {
                    Packet::Publish(p) => prop_assert!(!p.retain),
                    Packet::Connect(c) => prop_assert_eq!(c.clean_session, clean),
                    Packet::PubAck(_)
                    | Packet::Subscribe(_)
                    | Packet::Unsubscribe(_)
                    | Packet::PingReq
                    | Packet::Disconnect => {}
                    other => prop_assert!(false, "server packet sent: {other:?}"),
                }
            }
            while let Some(e) = s.poll_event() {
                if let Event::Closed(_) = e {
                    prop_assert_eq!(s.state(), SessionState::Disconnected);
                }
            }
            if let Some(at) = s.poll_timeout() {
                prop_assert!(s.state() != SessionState::Disconnected);
                prop_assert!(at <= now + Duration::from_secs(2 * u64::from(keep_alive)));
            }
        }
    }
}
