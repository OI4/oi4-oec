//! Scenarios of the sans-IO MQTT session against a scripted broker that
//! speaks through the same codec (REQ-MQTT-*, MG 6.3, REQ-LIFE-*).

use core::num::NonZeroU16;
use core::time::Duration;

use oec_core::mqtt::{
    CloseReason, CodecError, ConnAck, Connect, ConnectReturnCode, Credentials, Delivery, Event,
    LastWill, MAX_REMAINING_LENGTH, Packet, PacketId, PacketType, ProtocolError, Publish, QoS,
    Session, SessionConfig, SessionError, SessionState, SubAck, SubscribeReturnCode, decode,
    encode,
};

fn t(secs: u64) -> Duration {
    Duration::from_secs(secs)
}

fn id(n: u16) -> PacketId {
    PacketId::new(n).unwrap()
}

fn will() -> LastWill {
    LastWill {
        topic: "Oi4v2/Registry/a/Pub/Health/a".into(),
        payload: b"{}".to_vec(),
        qos: QoS::AtMostOnce,
    }
}

/// All packets of `bytes`.
fn packets(bytes: &[u8]) -> Vec<Packet> {
    let mut out = Vec::new();
    let mut rest = bytes;
    while let Some((p, n)) = decode(rest, MAX_REMAINING_LENGTH).unwrap() {
        out.push(p);
        rest = &rest[n..];
    }
    assert!(rest.is_empty(), "incomplete packet in the transmit buffer");
    out
}

fn sent(s: &mut Session) -> Vec<Packet> {
    packets(&s.take_transmit())
}

fn bytes(p: &Packet) -> Vec<u8> {
    let mut out = Vec::new();
    encode(p, &mut out).unwrap();
    out
}

fn events(s: &mut Session) -> Vec<Event> {
    std::iter::from_fn(|| s.poll_event()).collect()
}

fn connack(session_present: bool) -> Vec<u8> {
    bytes(&Packet::ConnAck(ConnAck {
        session_present,
        code: ConnectReturnCode::Accepted,
    }))
}

/// A session after CONNECT and an accepted CONNACK at `now`.
fn connected(config: SessionConfig, now: Duration) -> Session {
    let mut s = Session::new(config).unwrap();
    s.connect(will(), now).unwrap();
    s.take_transmit();
    s.handle_input(&connack(false), now);
    assert_eq!(
        events(&mut s),
        [Event::Connected {
            session_present: false
        }]
    );
    s
}

fn the_connect(packets: &[Packet]) -> Connect {
    match packets {
        [Packet::Connect(c)] => c.clone(),
        other => panic!("expected one CONNECT, got {other:?}"),
    }
}

#[test]
fn connect_uses_the_defaults_of_mg_6_3_1() {
    let mut config = SessionConfig::new("client");
    config.credentials = Some(Credentials {
        username: "User".into(),
        password: Some(b"secret".to_vec()),
    });
    let mut s = Session::new(config.clone()).unwrap();
    assert_eq!(s.state(), SessionState::Disconnected);
    s.connect(will(), t(0)).unwrap();
    assert_eq!(s.state(), SessionState::Connecting);
    let c = the_connect(&sent(&mut s));
    assert_eq!(c.client_id, "client");
    assert_eq!(c.keep_alive, 60); // REQ-MQTT-014
    assert!(c.clean_session); // REQ-MQTT-015
    assert_eq!(c.will, Some(will())); // REQ-LIFE-008, no Retain
    assert_eq!(c.username.as_deref(), Some("User")); // REQ-MQTT-011
    assert_eq!(c.password.as_deref(), Some(&b"secret"[..]));
    assert!(!format!("{config:?}").contains("secret"));
    assert_eq!(s.connect(will(), t(0)), Err(SessionError::AlreadyConnected));
    // nothing may be published before CONNACK (REQ-LIFE-015)
    assert_eq!(
        s.publish("a", Vec::new(), QoS::AtMostOnce, t(0)),
        Err(SessionError::NotConnected)
    );
    s.handle_input(&connack(false), t(1));
    assert_eq!(s.state(), SessionState::Connected);
}

#[test]
fn config_is_validated() {
    let mut c = SessionConfig::new("");
    assert!(Session::new(c.clone()).is_ok());
    c.clean_session = false; // REQ-MQTT-016
    assert!(matches!(
        Session::new(c),
        Err(SessionError::InvalidConfig(_))
    ));
    let mut c = SessionConfig::new("x");
    c.max_packet_size = 262_143; // RIG 4.1.1
    assert!(matches!(
        Session::new(c),
        Err(SessionError::InvalidConfig(_))
    ));
    let mut c = SessionConfig::new("x");
    c.max_in_flight = 0;
    assert!(matches!(
        Session::new(c),
        Err(SessionError::InvalidConfig(_))
    ));
    let mut c = SessionConfig::new("x\0");
    c.keep_alive = NonZeroU16::new(5).unwrap();
    assert!(matches!(Session::new(c), Err(SessionError::Codec(_))));
}

#[test]
fn every_connect_carries_the_will_it_is_given() {
    // REQ-LIFE-016: a new Will per connection
    let mut s = Session::new(SessionConfig::new("c")).unwrap();
    let mut wills = Vec::new();
    for n in 0..3u8 {
        let w = LastWill {
            payload: vec![n],
            ..will()
        };
        s.connect(w.clone(), t(u64::from(n) * 100)).unwrap();
        assert_eq!(the_connect(&sent(&mut s)).will, Some(w.clone()));
        wills.push(w);
        s.handle_input(&connack(false), t(u64::from(n) * 100));
        s.connection_lost();
        assert_eq!(
            events(&mut s),
            [
                Event::Connected {
                    session_present: false
                },
                Event::Closed(CloseReason::TransportClosed)
            ]
        );
    }
    assert_ne!(wills[0], wills[1]);
}

#[test]
fn graceful_close_sends_disconnect_after_the_close_message() {
    let mut s = connected(SessionConfig::new("c"), t(0));
    s.publish("a/close", b"close".to_vec(), QoS::AtMostOnce, t(1))
        .unwrap();
    s.disconnect(t(1)).unwrap();
    let out = sent(&mut s);
    assert!(matches!(&out[..], [Packet::Publish(p), Packet::Disconnect] if p.topic == "a/close"));
    assert_eq!(events(&mut s), [Event::Closed(CloseReason::Graceful)]);
    assert_eq!(s.state(), SessionState::Disconnected);
    assert_eq!(s.disconnect(t(2)), Err(SessionError::NotConnected));
    assert_eq!(s.poll_timeout(), None);
}

#[test]
fn publish_never_sets_retain_and_checks_the_limits() {
    let mut s = connected(SessionConfig::new("c"), t(0));
    assert_eq!(
        s.publish("a/b", vec![1, 2], QoS::AtMostOnce, t(0)),
        Ok(None)
    );
    let out = sent(&mut s);
    assert_eq!(
        out,
        [Packet::Publish(Publish {
            topic: "a/b".into(),
            payload: vec![1, 2],
            delivery: Delivery::AtMostOnce,
            retain: false, // REQ-MQTT-018
        })]
    );
    // MaxPacketSize is the payload limit (RIG 4.1.1, REQ-PAG-005)
    assert_eq!(
        s.publish("a", vec![0; 262_145], QoS::AtMostOnce, t(0)),
        Err(SessionError::PayloadTooLarge {
            size: 262_145,
            limit: 262_144
        })
    );
    assert!(
        s.publish("a", vec![0; 262_144], QoS::AtMostOnce, t(0))
            .is_ok()
    );
    assert!(matches!(
        s.publish("a/+", Vec::new(), QoS::AtMostOnce, t(0)),
        Err(SessionError::Codec(CodecError::Malformed { .. }))
    ));
}

#[test]
fn qos1_publication_is_acknowledged() {
    let mut s = connected(SessionConfig::new("c"), t(0));
    let pid = s
        .publish("a", b"x".to_vec(), QoS::AtLeastOnce, t(0))
        .unwrap()
        .unwrap();
    assert_eq!(s.in_flight(), 1);
    assert!(matches!(
        &sent(&mut s)[..],
        [Packet::Publish(Publish {
            delivery: Delivery::AtLeastOnce { dup: false, .. },
            ..
        })]
    ));
    // a PUBACK of an unknown id is ignored
    s.handle_input(&bytes(&Packet::PubAck(id(999))), t(1));
    s.handle_input(&bytes(&Packet::PubAck(pid)), t(1));
    assert_eq!(events(&mut s), [Event::Published(pid)]);
    assert_eq!(s.in_flight(), 0);
}

#[test]
fn incoming_qos1_is_acknowledged_at_once() {
    let mut s = connected(SessionConfig::new("c"), t(0));
    let p = Publish {
        topic: "x/y".into(),
        payload: b"m".to_vec(),
        delivery: Delivery::AtLeastOnce {
            packet_id: id(42),
            dup: false,
        },
        retain: true,
    };
    // split across reads
    let b = bytes(&Packet::Publish(p.clone()));
    s.handle_input(&b[..3], t(1));
    assert!(events(&mut s).is_empty());
    s.handle_input(&b[3..], t(1));
    assert_eq!(sent(&mut s), [Packet::PubAck(id(42))]);
    assert_eq!(events(&mut s), [Event::Message(p)]);
}

#[test]
fn in_flight_limit_and_packet_id_reuse() {
    let mut c = SessionConfig::new("c");
    c.max_in_flight = 65_535;
    let mut s = connected(c, t(0));
    let mut ids = std::collections::BTreeSet::new();
    for _ in 0..65_535 {
        let pid = s
            .publish("a", Vec::new(), QoS::AtLeastOnce, t(0))
            .unwrap()
            .unwrap();
        assert!(ids.insert(pid));
    }
    assert_eq!(
        s.publish("a", Vec::new(), QoS::AtLeastOnce, t(0)),
        Err(SessionError::InFlightLimit)
    );
    // QoS 0 needs no packet id
    assert_eq!(s.publish("a", Vec::new(), QoS::AtMostOnce, t(0)), Ok(None));
    s.take_transmit();
    s.handle_input(&bytes(&Packet::PubAck(id(7))), t(1));
    assert_eq!(
        s.publish("a", Vec::new(), QoS::AtLeastOnce, t(1)),
        Ok(Some(id(7)))
    );

    let mut c = SessionConfig::new("c");
    c.max_in_flight = 2;
    let mut s = connected(c, t(0));
    s.publish("a", Vec::new(), QoS::AtLeastOnce, t(0)).unwrap();
    s.publish("a", Vec::new(), QoS::AtLeastOnce, t(0)).unwrap();
    assert_eq!(
        s.publish("a", Vec::new(), QoS::AtLeastOnce, t(0)),
        Err(SessionError::InFlightLimit)
    );
}

#[test]
fn keep_alive_pings_and_times_out() {
    let mut s = connected(SessionConfig::new("c"), t(0));
    assert_eq!(s.poll_timeout(), Some(t(60)));
    s.handle_timeout(t(59));
    assert!(!s.has_transmit());
    s.handle_timeout(t(60));
    assert_eq!(sent(&mut s), [Packet::PingReq]);
    assert_eq!(s.poll_timeout(), Some(t(120)));
    s.handle_input(&bytes(&Packet::PingResp), t(70));
    // sent at 60, received at 70: the next ping is due at 120
    assert_eq!(s.poll_timeout(), Some(t(120)));
    s.handle_timeout(t(120));
    assert_eq!(sent(&mut s), [Packet::PingReq]);
    s.handle_timeout(t(179));
    assert_eq!(s.state(), SessionState::Connected);
    s.handle_timeout(t(180));
    assert_eq!(
        events(&mut s),
        [Event::Closed(CloseReason::KeepAliveTimeout)]
    );
    // no DISCONNECT: the broker publishes the Will (MG 6.3.5)
    assert!(!s.has_transmit());
    assert_eq!(s.state(), SessionState::Disconnected);
}

#[test]
fn traffic_defers_the_ping() {
    let mut s = connected(SessionConfig::new("c"), t(0));
    s.publish("a", Vec::new(), QoS::AtMostOnce, t(30)).unwrap();
    s.take_transmit();
    // received nothing since 0: a ping at 60 checks the broker
    assert_eq!(s.poll_timeout(), Some(t(60)));
    s.handle_input(&bytes(&Packet::PingResp), t(50));
    assert_eq!(s.poll_timeout(), Some(t(90)));
}

#[test]
fn connect_timeout_and_refusal() {
    let mut s = Session::new(SessionConfig::new("c")).unwrap();
    s.connect(will(), t(10)).unwrap();
    s.take_transmit();
    assert_eq!(s.poll_timeout(), Some(t(70)));
    s.handle_timeout(t(69));
    assert_eq!(s.state(), SessionState::Connecting);
    s.handle_timeout(t(70));
    assert_eq!(events(&mut s), [Event::Closed(CloseReason::ConnectTimeout)]);

    s.connect(will(), t(100)).unwrap();
    s.take_transmit();
    s.handle_input(
        &bytes(&Packet::ConnAck(ConnAck {
            session_present: false,
            code: ConnectReturnCode::BadUserNameOrPassword,
        })),
        t(101),
    );
    assert_eq!(
        events(&mut s),
        [Event::Closed(CloseReason::Refused(
            ConnectReturnCode::BadUserNameOrPassword
        ))]
    );
}

#[test]
fn protocol_violations_close_without_disconnect() {
    let cases: Vec<(bool, Vec<u8>, ProtocolError)> = vec![
        // anything but CONNACK first (MQTT-3.2.0-1)
        (
            false,
            bytes(&Packet::PingResp),
            ProtocolError::Unexpected(PacketType::PingResp),
        ),
        // a second CONNACK
        (
            true,
            connack(false),
            ProtocolError::Unexpected(PacketType::ConnAck),
        ),
        // client-only packets
        (
            true,
            bytes(&Packet::PingReq),
            ProtocolError::Unexpected(PacketType::PingReq),
        ),
        (
            true,
            bytes(&Packet::Disconnect),
            ProtocolError::Unexpected(PacketType::Disconnect),
        ),
        // Session Present with Clean Session (MQTT-3.2.2-1)
        (false, connack(true), ProtocolError::SessionPresent),
        // bytes that are no packet
        (
            true,
            vec![0xF0, 0x00],
            ProtocolError::Codec(CodecError::ReservedPacketType(15)),
        ),
        // larger than MaxPacketSize + topic + id (RIG 4.1.1)
        (
            true,
            vec![0x30, 0xFF, 0xFF, 0x7F],
            ProtocolError::Codec(CodecError::PacketTooLarge {
                remaining_length: 2_097_151,
                limit: 262_144 + 65_539,
            }),
        ),
    ];
    for (after_connack, input, expected) in cases {
        let mut s = Session::new(SessionConfig::new("c")).unwrap();
        s.connect(will(), t(0)).unwrap();
        if after_connack {
            s.handle_input(&connack(false), t(0));
            s.publish("a", Vec::new(), QoS::AtMostOnce, t(0)).unwrap();
        }
        events(&mut s);
        s.handle_input(&input, t(1));
        assert_eq!(
            events(&mut s),
            [Event::Closed(CloseReason::Protocol(expected.clone()))],
            "{expected}"
        );
        assert!(!s.has_transmit(), "no DISCONNECT after {expected}");
        assert_eq!(s.state(), SessionState::Disconnected);
        // input after the close is ignored
        s.handle_input(&connack(false), t(2));
        assert!(events(&mut s).is_empty());
    }
}

fn suback(pid: PacketId, codes: Vec<SubscribeReturnCode>) -> Vec<u8> {
    bytes(&Packet::SubAck(SubAck {
        packet_id: pid,
        codes,
    }))
}

#[test]
fn subscriptions_are_held_sent_and_restored() {
    let mut s = Session::new(SessionConfig::new("c")).unwrap();
    // before the connection: held back
    let a = s
        .subscribe(
            vec![
                ("Oi4v2/+/+/Get/#".into(), QoS::AtMostOnce),
                ("denied/#".into(), QoS::AtLeastOnce),
            ],
            t(0),
        )
        .unwrap();
    assert!(!s.has_transmit());
    assert!(matches!(
        s.subscribe(vec![("a/#/b".into(), QoS::AtMostOnce)], t(0)),
        Err(SessionError::Codec(_))
    ));
    s.connect(will(), t(0)).unwrap();
    s.take_transmit();
    s.handle_input(&connack(false), t(1));
    let out = sent(&mut s);
    assert!(matches!(&out[..], [Packet::Subscribe(sub)] if sub.packet_id == a));
    s.handle_input(
        &suback(
            a,
            vec![
                SubscribeReturnCode::Success(QoS::AtMostOnce),
                SubscribeReturnCode::Failure,
            ],
        ),
        t(2),
    );
    let ev = events(&mut s);
    assert!(
        matches!(&ev[1], Event::Subscribed { packet_id, restored: false, results }
        if *packet_id == a && results[1].1 == SubscribeReturnCode::Failure)
    );
    assert_eq!(
        s.subscriptions().collect::<Vec<_>>(),
        [("Oi4v2/+/+/Get/#", QoS::AtMostOnce)]
    );

    // reconnect without session: the granted subscription is restored
    s.connection_lost();
    s.connect(will(), t(10)).unwrap();
    s.take_transmit();
    s.handle_input(&connack(false), t(11));
    let out = sent(&mut s);
    let Packet::Subscribe(restore) = &out[0] else {
        panic!("{out:?}")
    };
    assert_eq!(
        restore.filters,
        [("Oi4v2/+/+/Get/#".to_owned(), QoS::AtMostOnce)]
    );
    events(&mut s);
    s.handle_input(
        &suback(
            restore.packet_id,
            vec![SubscribeReturnCode::Success(QoS::AtMostOnce)],
        ),
        t(12),
    );
    assert!(matches!(
        &events(&mut s)[..],
        [Event::Subscribed { restored: true, .. }]
    ));

    // unsubscribe
    let u = s
        .unsubscribe(vec!["Oi4v2/+/+/Get/#".into()], t(13))
        .unwrap();
    assert!(matches!(&sent(&mut s)[..], [Packet::Unsubscribe(x)] if x.packet_id == u));
    s.handle_input(&bytes(&Packet::UnsubAck(u)), t(14));
    assert_eq!(
        events(&mut s),
        [Event::Unsubscribed {
            packet_id: u,
            filters: vec!["Oi4v2/+/+/Get/#".into()]
        }]
    );
    assert_eq!(s.subscriptions().count(), 0);
}

#[test]
fn pending_requests_are_sent_again_after_a_reconnect() {
    let mut s = connected(SessionConfig::new("c"), t(0));
    let sub = s
        .subscribe(vec![("a".into(), QoS::AtMostOnce)], t(0))
        .unwrap();
    let unsub = s.unsubscribe(vec!["b".into()], t(0)).unwrap();
    s.take_transmit();
    s.connection_lost();
    events(&mut s);
    s.connect(will(), t(5)).unwrap();
    s.take_transmit();
    s.handle_input(&connack(false), t(6));
    let out = sent(&mut s);
    assert!(matches!(&out[..],
        [Packet::Subscribe(x), Packet::Unsubscribe(y)] if x.packet_id == sub && y.packet_id == unsub));
}

#[test]
fn suback_with_the_wrong_number_of_codes_is_a_violation() {
    let mut s = connected(SessionConfig::new("c"), t(0));
    let a = s
        .subscribe(
            vec![("a".into(), QoS::AtMostOnce), ("b".into(), QoS::AtMostOnce)],
            t(0),
        )
        .unwrap();
    s.handle_input(
        &suback(a, vec![SubscribeReturnCode::Success(QoS::AtMostOnce)]),
        t(1),
    );
    assert_eq!(
        events(&mut s),
        [Event::Closed(CloseReason::Protocol(
            ProtocolError::SubAckMismatch {
                expected: 2,
                actual: 1
            }
        ))]
    );
}

fn persistent() -> SessionConfig {
    let mut c = SessionConfig::new("persistent");
    c.clean_session = false;
    c
}

#[test]
fn unacknowledged_qos1_is_resent_with_dup_in_a_present_session() {
    // MQTT-4.4.0-1
    let mut s = connected(persistent(), t(0));
    let p1 = s
        .publish("a", b"1".to_vec(), QoS::AtLeastOnce, t(0))
        .unwrap()
        .unwrap();
    let p2 = s
        .publish("a", b"2".to_vec(), QoS::AtLeastOnce, t(0))
        .unwrap()
        .unwrap();
    s.take_transmit();
    s.connection_lost();
    events(&mut s);
    s.connect(will(), t(10)).unwrap();
    assert!(!the_connect(&sent(&mut s)).clean_session);
    s.handle_input(&connack(true), t(11));
    let out = sent(&mut s);
    let resent: Vec<_> = out
        .iter()
        .map(|p| match p {
            Packet::Publish(Publish {
                delivery: Delivery::AtLeastOnce { packet_id, dup },
                payload,
                ..
            }) => (*packet_id, *dup, payload.clone()),
            other => panic!("{other:?}"),
        })
        .collect();
    assert_eq!(
        resent,
        [(p1, true, b"1".to_vec()), (p2, true, b"2".to_vec())]
    );
    assert_eq!(s.in_flight(), 2);
}

#[test]
fn unacknowledged_qos1_is_dropped_without_session() {
    let mut s = connected(SessionConfig::new("c"), t(0));
    let p1 = s
        .publish("a", b"1".to_vec(), QoS::AtLeastOnce, t(0))
        .unwrap()
        .unwrap();
    s.take_transmit();
    s.connection_lost();
    events(&mut s);
    s.connect(will(), t(10)).unwrap();
    s.take_transmit();
    s.handle_input(&connack(false), t(11));
    assert_eq!(
        events(&mut s),
        [
            Event::Connected {
                session_present: false
            },
            Event::Dropped(vec![p1])
        ]
    );
    assert!(!s.has_transmit());
    assert_eq!(s.in_flight(), 0);
}
