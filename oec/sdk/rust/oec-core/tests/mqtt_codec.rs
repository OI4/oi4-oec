//! Byte-exact tests of the MQTT 3.1.1 codec (REQ-MQTT-001, ADR-0023 Q-026):
//! every packet type against hand-encoded frames of the standard, the
//! Remaining Length boundaries and every refusal with its rule.

use oec_core::mqtt::{
    CodecError, ConnAck, Connect, ConnectReturnCode, Decoder, Delivery, LastWill,
    MAX_REMAINING_LENGTH, Packet, PacketId, PacketType, Publish, QoS, SubAck, Subscribe,
    SubscribeReturnCode, Unsubscribe, decode, encode, encoded_len,
};

fn id(n: u16) -> PacketId {
    PacketId::new(n).unwrap()
}

/// Encodes, compares with `bytes`, decodes `bytes` and compares with the
/// packet.
fn exact(packet: &Packet, bytes: &[u8]) {
    let mut out = Vec::new();
    let n = encode(packet, &mut out).unwrap();
    assert_eq!(out, bytes, "encoding of {packet:?}");
    assert_eq!(n, bytes.len());
    assert_eq!(encoded_len(packet), Some(bytes.len()));
    let (decoded, used) = decode(bytes, MAX_REMAINING_LENGTH).unwrap().unwrap();
    assert_eq!(&decoded, packet);
    assert_eq!(used, bytes.len());
}

fn rule_of(bytes: &[u8]) -> &'static str {
    decode(bytes, MAX_REMAINING_LENGTH)
        .expect_err("must be refused")
        .rule()
}

#[test]
fn connect_minimal() {
    let c = Connect {
        client_id: "c".into(),
        keep_alive: 60,
        clean_session: true,
        will: None,
        username: None,
        password: None,
    };
    exact(
        &Packet::Connect(c),
        &[
            0x10, 0x0D, 0x00, 0x04, b'M', b'Q', b'T', b'T', 0x04, 0x02, 0x00, 0x3C, 0x00, 0x01,
            b'c',
        ],
    );
}

#[test]
fn connect_with_will_and_credentials() {
    let c = Connect {
        client_id: "c".into(),
        keep_alive: 60,
        clean_session: true,
        will: Some(LastWill {
            topic: "t".into(),
            payload: b"w".to_vec(),
            qos: QoS::AtLeastOnce,
        }),
        username: Some("u".into()),
        password: Some(b"p".to_vec()),
    };
    // flags: user name, password, will QoS 1, will, clean session; Will
    // Retain 0 (REQ-MQTT-018)
    exact(
        &Packet::Connect(c.clone()),
        &[
            0x10, 0x19, 0x00, 0x04, b'M', b'Q', b'T', b'T', 0x04, 0xCE, 0x00, 0x3C, 0x00, 0x01,
            b'c', 0x00, 0x01, b't', 0x00, 0x01, b'w', 0x00, 0x01, b'u', 0x00, 0x01, b'p',
        ],
    );
    // The password never appears in Debug (G-4.5).
    assert!(!format!("{c:?}").contains("[112]"));
    assert!(format!("{c:?}").contains("***"));
}

#[test]
fn connect_persistent_with_empty_password() {
    let c = Connect {
        client_id: "id".into(),
        keep_alive: 1,
        clean_session: false,
        will: None,
        username: Some(String::new()),
        password: Some(Vec::new()),
    };
    exact(
        &Packet::Connect(c),
        &[
            0x10, 0x12, 0x00, 0x04, b'M', b'Q', b'T', b'T', 0x04, 0xC0, 0x00, 0x01, 0x00, 0x02,
            b'i', b'd', 0x00, 0x00, 0x00, 0x00,
        ],
    );
}

#[test]
fn connack() {
    exact(
        &Packet::ConnAck(ConnAck {
            session_present: false,
            code: ConnectReturnCode::Accepted,
        }),
        &[0x20, 0x02, 0x00, 0x00],
    );
    exact(
        &Packet::ConnAck(ConnAck {
            session_present: true,
            code: ConnectReturnCode::Accepted,
        }),
        &[0x20, 0x02, 0x01, 0x00],
    );
    exact(
        &Packet::ConnAck(ConnAck {
            session_present: false,
            code: ConnectReturnCode::NotAuthorized,
        }),
        &[0x20, 0x02, 0x00, 0x05],
    );
}

#[test]
fn publish_qos0_and_qos1() {
    exact(
        &Packet::Publish(Publish {
            topic: "a/b".into(),
            payload: b"hi".to_vec(),
            delivery: Delivery::AtMostOnce,
            retain: false,
        }),
        &[0x30, 0x07, 0x00, 0x03, b'a', b'/', b'b', b'h', b'i'],
    );
    exact(
        &Packet::Publish(Publish {
            topic: "a/b".into(),
            payload: b"hi".to_vec(),
            delivery: Delivery::AtLeastOnce {
                packet_id: id(10),
                dup: true,
            },
            retain: false,
        }),
        &[
            0x3A, 0x09, 0x00, 0x03, b'a', b'/', b'b', 0x00, 0x0A, b'h', b'i',
        ],
    );
    // A broker may forward a retained message (MQTT-3.3.1-8): the codec
    // keeps the flag; the session never sets it (REQ-MQTT-018).
    exact(
        &Packet::Publish(Publish {
            topic: "a".into(),
            payload: Vec::new(),
            delivery: Delivery::AtMostOnce,
            retain: true,
        }),
        &[0x31, 0x03, 0x00, 0x01, b'a'],
    );
}

#[test]
fn acks_subscribe_unsubscribe_ping_disconnect() {
    exact(&Packet::PubAck(id(10)), &[0x40, 0x02, 0x00, 0x0A]);
    exact(
        &Packet::Subscribe(Subscribe {
            packet_id: id(1),
            filters: vec![("a/#".into(), QoS::AtLeastOnce)],
        }),
        &[0x82, 0x08, 0x00, 0x01, 0x00, 0x03, b'a', b'/', b'#', 0x01],
    );
    exact(
        &Packet::SubAck(SubAck {
            packet_id: id(1),
            codes: vec![
                SubscribeReturnCode::Success(QoS::AtLeastOnce),
                SubscribeReturnCode::Failure,
                SubscribeReturnCode::Success(QoS::AtMostOnce),
            ],
        }),
        &[0x90, 0x05, 0x00, 0x01, 0x01, 0x80, 0x00],
    );
    exact(
        &Packet::Unsubscribe(Unsubscribe {
            packet_id: id(258),
            filters: vec!["a/#".into(), "+".into()],
        }),
        &[
            0xA2, 0x0A, 0x01, 0x02, 0x00, 0x03, b'a', b'/', b'#', 0x00, 0x01, b'+',
        ],
    );
    exact(&Packet::UnsubAck(id(65535)), &[0xB0, 0x02, 0xFF, 0xFF]);
    exact(&Packet::PingReq, &[0xC0, 0x00]);
    exact(&Packet::PingResp, &[0xD0, 0x00]);
    exact(&Packet::Disconnect, &[0xE0, 0x00]);
}

fn publish_with_remaining(len: usize) -> Packet {
    // topic "t": 3 bytes of variable header
    Packet::Publish(Publish {
        topic: "t".into(),
        payload: vec![0; len - 3],
        delivery: Delivery::AtMostOnce,
        retain: false,
    })
}

#[test]
fn remaining_length_boundaries() {
    for (len, header) in [
        (3usize, vec![0x03u8]),
        (127, vec![0x7F]),
        (128, vec![0x80, 0x01]),
        (16_383, vec![0xFF, 0x7F]),
        (16_384, vec![0x80, 0x80, 0x01]),
        (2_097_151, vec![0xFF, 0xFF, 0x7F]),
        (2_097_152, vec![0x80, 0x80, 0x80, 0x01]),
    ] {
        let p = publish_with_remaining(len);
        let mut out = Vec::new();
        encode(&p, &mut out).unwrap();
        assert_eq!(&out[1..=header.len()], header.as_slice(), "length {len}");
        assert_eq!(out.len(), 1 + header.len() + len);
        assert_eq!(decode(&out, MAX_REMAINING_LENGTH).unwrap().unwrap().0, p);
    }
    // the largest length: the header alone is accepted as incomplete
    assert_eq!(
        decode(&[0x30, 0xFF, 0xFF, 0xFF, 0x7F], MAX_REMAINING_LENGTH).unwrap(),
        None
    );
    // a fifth length byte
    assert_eq!(
        decode(&[0x30, 0xFF, 0xFF, 0xFF, 0xFF, 0x01], MAX_REMAINING_LENGTH),
        Err(CodecError::MalformedRemainingLength)
    );
    // non-minimal encodings (canonical decoding)
    for bytes in [
        &[0xC0, 0x80, 0x00][..],
        &[0x30, 0x83, 0x80, 0x00][..],
        &[0x30, 0x80, 0x80, 0x80, 0x00][..],
    ] {
        assert_eq!(
            decode(bytes, MAX_REMAINING_LENGTH),
            Err(CodecError::MalformedRemainingLength)
        );
    }
}

#[test]
fn limit_is_checked_at_the_header() {
    // MaxPacketSize guard: refused before the body arrives (RIG 4.1.1).
    assert_eq!(
        decode(&[0x30, 0x80, 0x80, 0x01], 16_383),
        Err(CodecError::PacketTooLarge {
            remaining_length: 16_384,
            limit: 16_383
        })
    );
    assert_eq!(decode(&[0x30, 0xFF, 0x7F], 16_383).unwrap(), None);
    // the decoder limit never exceeds MQTT
    assert_eq!(Decoder::new(usize::MAX).limit(), MAX_REMAINING_LENGTH);
}

#[test]
fn incomplete_input_needs_more() {
    let mut full = Vec::new();
    encode(&publish_with_remaining(200), &mut full).unwrap();
    for cut in 0..full.len() {
        assert_eq!(decode(&full[..cut], MAX_REMAINING_LENGTH).unwrap(), None);
    }
}

#[test]
fn fixed_header_refusals() {
    assert_eq!(
        decode(&[0x00, 0x00], MAX_REMAINING_LENGTH),
        Err(CodecError::ReservedPacketType(0))
    );
    assert_eq!(
        decode(&[0xF0, 0x00], MAX_REMAINING_LENGTH),
        Err(CodecError::ReservedPacketType(15))
    );
    // reserved flags (MQTT-2.2.2-2), checked with the first byte
    for first in [0x11u8, 0x21, 0x41, 0x80, 0x83, 0xA0, 0xB1, 0xC8, 0xD1, 0xE2] {
        assert_eq!(rule_of(&[first]), "MQTT-2.2.2-2", "first byte {first:#04x}");
    }
    // the QoS 2 flow (REQ-MQTT-017)
    for first in [0x50u8, 0x62, 0x70] {
        assert!(matches!(
            decode(&[first, 0x02, 0x00, 0x01], MAX_REMAINING_LENGTH),
            Err(CodecError::Unsupported { .. })
        ));
        assert_eq!(rule_of(&[first, 0x02, 0x00, 0x01]), "REQ-MQTT-017");
    }
}

#[test]
fn publish_refusals() {
    // QoS 2 (REQ-MQTT-017), QoS 3 (MQTT-3.3.1-4), DUP with QoS 0 (MQTT-3.3.1-2)
    assert_eq!(
        rule_of(&[0x34, 0x05, 0x00, 0x01, b'a', 0x00, 0x01]),
        "REQ-MQTT-017"
    );
    assert_eq!(
        rule_of(&[0x36, 0x05, 0x00, 0x01, b'a', 0x00, 0x01]),
        "MQTT-3.3.1-4"
    );
    assert_eq!(rule_of(&[0x38, 0x03, 0x00, 0x01, b'a']), "MQTT-3.3.1-2");
    // wildcards, empty topic, UTF-8, U+0000
    assert_eq!(
        rule_of(&[0x30, 0x05, 0x00, 0x03, b'a', b'/', b'+']),
        "MQTT-3.3.2-2"
    );
    assert_eq!(rule_of(&[0x30, 0x03, 0x00, 0x01, b'#']), "MQTT-3.3.2-2");
    assert_eq!(rule_of(&[0x30, 0x02, 0x00, 0x00]), "MQTT-4.7.3-1");
    assert_eq!(rule_of(&[0x30, 0x03, 0x00, 0x01, 0xFF]), "MQTT-1.5.3-1");
    assert_eq!(
        rule_of(&[0x30, 0x05, 0x00, 0x03, 0xED, 0xA0, 0x80]),
        "MQTT-1.5.3-1"
    );
    assert_eq!(rule_of(&[0x30, 0x03, 0x00, 0x01, 0x00]), "MQTT-1.5.3-2");
    // packet id 0 (MQTT-2.3.1-1)
    assert_eq!(
        rule_of(&[0x32, 0x05, 0x00, 0x01, b'a', 0x00, 0x00]),
        "MQTT-2.3.1-1"
    );
    // the Remaining Length ends inside the topic
    assert_eq!(rule_of(&[0x30, 0x02, 0x00, 0x05]), "MQTT 2.2.3");
    assert_eq!(rule_of(&[0x32, 0x04, 0x00, 0x01, b'a', 0x00]), "MQTT 2.2.3");
}

fn connect_bytes(flags: u8, rest: &[u8]) -> Vec<u8> {
    let mut v = vec![0x00, 0x04, b'M', b'Q', b'T', b'T', 0x04, flags, 0x00, 0x3C];
    v.extend_from_slice(rest);
    let mut out = vec![0x10, u8::try_from(v.len()).unwrap()];
    out.extend(v);
    out
}

#[test]
fn connect_refusals() {
    let cid = [0x00, 0x01, b'c'];
    assert!(decode(&connect_bytes(0x02, &cid), MAX_REMAINING_LENGTH).is_ok());
    // protocol name and level
    let mut v = connect_bytes(0x02, &cid);
    v[4] = b'X';
    assert_eq!(rule_of(&v), "MQTT-3.1.2-1");
    let mut v = connect_bytes(0x02, &cid);
    v[8] = 3;
    assert_eq!(rule_of(&v), "REQ-MQTT-001");
    // reserved flag, Will flags, Password without User Name
    assert_eq!(rule_of(&connect_bytes(0x03, &cid)), "MQTT-3.1.2-3");
    assert_eq!(rule_of(&connect_bytes(0x0A, &cid)), "MQTT-3.1.2-13");
    assert_eq!(rule_of(&connect_bytes(0x22, &cid)), "MQTT-3.1.2-15");
    let will = [0x00, 0x01, b'c', 0x00, 0x01, b't', 0x00, 0x00];
    assert_eq!(rule_of(&connect_bytes(0x26, &will)), "REQ-MQTT-018");
    assert_eq!(rule_of(&connect_bytes(0x16, &will)), "REQ-MQTT-017");
    assert_eq!(rule_of(&connect_bytes(0x1E, &will)), "MQTT-3.1.2-14");
    assert_eq!(
        rule_of(&connect_bytes(0x42, &[0x00, 0x01, b'c', 0x00, 0x00])),
        "MQTT-3.1.2-22"
    );
    // a Will topic with a wildcard
    let will = [0x00, 0x01, b'c', 0x00, 0x01, b'#', 0x00, 0x00];
    assert_eq!(rule_of(&connect_bytes(0x06, &will)), "MQTT-3.3.2-2");
    // trailing bytes
    assert_eq!(
        rule_of(&connect_bytes(0x02, &[0x00, 0x01, b'c', 0x00])),
        "MQTT 3.1.3"
    );
}

#[test]
fn ack_refusals() {
    assert_eq!(rule_of(&[0x20, 0x02, 0x02, 0x00]), "MQTT 3.2.2.1");
    assert_eq!(rule_of(&[0x20, 0x02, 0x00, 0x06]), "MQTT 3.2.2.3");
    assert_eq!(rule_of(&[0x20, 0x02, 0x01, 0x05]), "MQTT-3.2.2-4");
    assert_eq!(rule_of(&[0x20, 0x03, 0x00, 0x00, 0x00]), "MQTT 3.2");
    assert_eq!(rule_of(&[0x40, 0x02, 0x00, 0x00]), "MQTT-2.3.1-1");
    assert_eq!(rule_of(&[0x40, 0x03, 0x00, 0x01, 0x00]), "MQTT 3.4");
    assert_eq!(rule_of(&[0xB0, 0x03, 0x00, 0x01, 0x00]), "MQTT 3.11");
    assert_eq!(rule_of(&[0x90, 0x02, 0x00, 0x01]), "MQTT-3.9.3-1");
    assert_eq!(rule_of(&[0x90, 0x03, 0x00, 0x01, 0x03]), "MQTT-3.9.3-2");
    assert_eq!(rule_of(&[0x90, 0x03, 0x00, 0x01, 0x02]), "REQ-MQTT-017");
    for b in [0xC0u8, 0xD0, 0xE0] {
        assert_eq!(rule_of(&[b, 0x01, 0x00]), "MQTT 2.2.3");
    }
}

#[test]
fn subscribe_refusals() {
    assert_eq!(rule_of(&[0x82, 0x02, 0x00, 0x01]), "MQTT-3.8.3-3");
    assert_eq!(
        rule_of(&[0x82, 0x06, 0x00, 0x01, 0x00, 0x01, b'a', 0x04]),
        "MQTT-3.8.3-4"
    );
    assert_eq!(
        rule_of(&[0x82, 0x06, 0x00, 0x01, 0x00, 0x01, b'a', 0x02]),
        "REQ-MQTT-017"
    );
    assert_eq!(
        rule_of(&[
            0x82, 0x0A, 0x00, 0x01, 0x00, 0x05, b'a', b'/', b'#', b'/', b'b', 0x00
        ]),
        "MQTT-4.7.1-2"
    );
    assert_eq!(
        rule_of(&[0x82, 0x08, 0x00, 0x01, 0x00, 0x03, b'a', b'b', b'#', 0x00]),
        "MQTT-4.7.1-2"
    );
    // the requested QoS byte is missing
    assert_eq!(
        rule_of(&[0x82, 0x07, 0x00, 0x01, 0x00, 0x03, b'a', b'/', b'b']),
        "MQTT 2.2.3"
    );
    assert_eq!(
        rule_of(&[0x82, 0x07, 0x00, 0x01, 0x00, 0x02, b'a', b'+', 0x00]),
        "MQTT-4.7.1-3"
    );
    assert_eq!(rule_of(&[0xA2, 0x02, 0x00, 0x01]), "MQTT-3.10.3-2");
    assert_eq!(
        rule_of(&[0xA2, 0x04, 0x00, 0x01, 0x00, 0x00]),
        "MQTT-4.7.3-1"
    );
}

#[test]
fn valid_topic_filters() {
    for f in [
        "#",
        "+",
        "a/#",
        "+/+",
        "a/+/b",
        "/",
        "a//b",
        "+/#",
        "Oi4v2/+/+/Pub/#",
    ] {
        let p = Packet::Subscribe(Subscribe {
            packet_id: id(1),
            filters: vec![(f.into(), QoS::AtMostOnce)],
        });
        let mut out = Vec::new();
        encode(&p, &mut out).unwrap_or_else(|e| panic!("{f}: {e}"));
        assert_eq!(decode(&out, MAX_REMAINING_LENGTH).unwrap().unwrap().0, p);
    }
}

#[test]
fn encoder_refuses_what_the_decoder_refuses() {
    let bad = [
        Packet::Publish(Publish {
            topic: "a/#".into(),
            payload: Vec::new(),
            delivery: Delivery::AtMostOnce,
            retain: false,
        }),
        Packet::Publish(Publish {
            topic: String::new(),
            payload: Vec::new(),
            delivery: Delivery::AtMostOnce,
            retain: false,
        }),
        Packet::Publish(Publish {
            topic: "a\0".into(),
            payload: Vec::new(),
            delivery: Delivery::AtMostOnce,
            retain: false,
        }),
        Packet::Connect(Connect {
            client_id: "c".into(),
            keep_alive: 0,
            clean_session: true,
            will: None,
            username: None,
            password: Some(vec![1]),
        }),
        Packet::ConnAck(ConnAck {
            session_present: true,
            code: ConnectReturnCode::ServerUnavailable,
        }),
        Packet::Subscribe(Subscribe {
            packet_id: id(1),
            filters: Vec::new(),
        }),
        Packet::Subscribe(Subscribe {
            packet_id: id(1),
            filters: vec![("a/#/b".into(), QoS::AtMostOnce)],
        }),
        Packet::SubAck(SubAck {
            packet_id: id(1),
            codes: Vec::new(),
        }),
        Packet::Unsubscribe(Unsubscribe {
            packet_id: id(1),
            filters: Vec::new(),
        }),
        Packet::Connect(Connect {
            client_id: "x".repeat(65_536),
            keep_alive: 0,
            clean_session: true,
            will: None,
            username: None,
            password: None,
        }),
    ];
    for p in bad {
        let mut out = vec![0xAA];
        assert!(encode(&p, &mut out).is_err(), "{:?}", p.packet_type());
        assert_eq!(out, [0xAA], "nothing appended on error");
    }
}

#[test]
fn decoder_streams_and_poisons() {
    let packets = [
        Packet::ConnAck(ConnAck {
            session_present: false,
            code: ConnectReturnCode::Accepted,
        }),
        Packet::Publish(Publish {
            topic: "a".into(),
            payload: vec![7; 300],
            delivery: Delivery::AtLeastOnce {
                packet_id: id(3),
                dup: false,
            },
            retain: false,
        }),
        Packet::PingResp,
    ];
    let mut bytes = Vec::new();
    for p in &packets {
        encode(p, &mut bytes).unwrap();
    }
    let mut d = Decoder::new(MAX_REMAINING_LENGTH);
    let mut got = Vec::new();
    for b in &bytes {
        d.feed(std::slice::from_ref(b));
        while let Some(p) = d.next_packet().unwrap() {
            got.push(p);
        }
    }
    assert_eq!(got, packets);
    assert_eq!(d.buffered(), 0);
    // after an error the stream is unusable until reset
    d.feed(&[0xF0, 0x00, 0xD0, 0x00]);
    assert!(d.next_packet().is_err());
    assert!(d.next_packet().is_err());
    d.reset();
    d.feed(&[0xD0, 0x00]);
    assert_eq!(d.next_packet().unwrap(), Some(Packet::PingResp));
}

#[test]
fn errors_display_their_rule() {
    let e = decode(&[0x38, 0x03, 0x00, 0x01, b'a'], MAX_REMAINING_LENGTH).unwrap_err();
    assert_eq!(
        e,
        CodecError::Malformed {
            packet: PacketType::Publish,
            rule: "MQTT-3.3.1-2",
            problem: "DUP set with QoS 0"
        }
    );
    assert_eq!(e.to_string(), "PUBLISH: DUP set with QoS 0 (MQTT-3.3.1-2)");
}
