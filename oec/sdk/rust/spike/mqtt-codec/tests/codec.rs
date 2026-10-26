//! Byte-exact tests of the codec spike against hand-encoded MQTT 3.1.1
//! packets, plus the error paths a client must handle (OASIS MQTT 3.1.1).

use oec_mqtt_codec_spike::*;

fn enc(p: Outgoing<'_>) -> Vec<u8> {
    let mut out = Vec::new();
    encode(&p, &mut out).unwrap();
    out
}

#[test]
fn connect_minimal() {
    let c = Connect { client_id: "a", keep_alive_s: 60, clean_session: true, will: None, username: None, password: None };
    assert_eq!(
        enc(Outgoing::Connect(c)),
        [0x10, 0x0D, 0, 4, b'M', b'Q', b'T', b'T', 4, 0x02, 0, 60, 0, 1, b'a']
    );
}

#[test]
fn connect_with_will_and_credentials_and_a_new_will_per_connect() {
    let mk = |payload: &'static [u8]| Connect {
        client_id: "c",
        keep_alive_s: 60,
        clean_session: true,
        will: Some(Will { topic: "w", payload, qos: QoS::AtMostOnce }),
        username: Some("u"),
        password: Some(b"p"),
    };
    let first = enc(Outgoing::Connect(mk(b"x")));
    assert_eq!(
        first,
        [
            0x10, 0x19, 0, 4, b'M', b'Q', b'T', b'T', 4, 0xC6, 0, 60, 0, 1, b'c', 0, 1, b'w', 0, 1, b'x', 0, 1, b'u', 0,
            1, b'p'
        ]
    );
    // REQ-LIFE-016: the runtime builds the Will (new MessageId) for every connect
    assert_ne!(first, enc(Outgoing::Connect(mk(b"y"))));
}

#[test]
fn password_needs_username() {
    let c = Connect { client_id: "c", keep_alive_s: 0, clean_session: true, will: None, username: None, password: Some(b"p") };
    assert!(encode(&Outgoing::Connect(c), &mut Vec::new()).is_err());
}

#[test]
fn publish_subscribe_ping_disconnect() {
    let p = |qos, pkid| enc(Outgoing::Publish { topic: "a/b", payload: b"hi", qos, pkid });
    assert_eq!(p(QoS::AtMostOnce, 0), [0x30, 7, 0, 3, b'a', b'/', b'b', b'h', b'i']);
    assert_eq!(p(QoS::AtLeastOnce, 10), [0x32, 9, 0, 3, b'a', b'/', b'b', 0, 10, b'h', b'i']);
    assert_eq!(
        enc(Outgoing::Subscribe { pkid: 1, filters: &[("a/#", QoS::AtMostOnce)] }),
        [0x82, 8, 0, 1, 0, 3, b'a', b'/', b'#', 0]
    );
    assert_eq!(enc(Outgoing::Unsubscribe { pkid: 2, filters: &["a/#"] }), [0xA2, 7, 0, 2, 0, 3, b'a', b'/', b'#']);
    assert_eq!(enc(Outgoing::PubAck { pkid: 258 }), [0x40, 2, 1, 2]);
    assert_eq!(enc(Outgoing::PingReq), [0xC0, 0]);
    assert_eq!(enc(Outgoing::Disconnect), [0xE0, 0]);
    let bad = Outgoing::Publish { topic: "a/+", payload: b"", qos: QoS::AtMostOnce, pkid: 0 };
    assert!(encode(&bad, &mut Vec::new()).is_err());
}

#[test]
fn remaining_length_boundaries() {
    // MQTT 3.1.1 table 2.4
    for (n, header) in [
        (0usize, vec![0x00]),
        (127, vec![0x7F]),
        (128, vec![0x80, 0x01]),
        (16_383, vec![0xFF, 0x7F]),
        (16_384, vec![0x80, 0x80, 0x01]),
        (2_097_151, vec![0xFF, 0xFF, 0x7F]),
        (2_097_152, vec![0x80, 0x80, 0x80, 0x01]),
    ] {
        if n < 5 {
            continue;
        }
        let payload = vec![0u8; n - 5]; // topic "a/b" = 5 bytes
        let out = enc(Outgoing::Publish { topic: "a/b", payload: &payload, qos: QoS::AtMostOnce, pkid: 0 });
        assert_eq!(&out[1..1 + header.len()], header.as_slice(), "length {n}");
        let mut d = Decoder::new(usize::MAX);
        d.feed(&out);
        assert!(matches!(d.next_packet(), Ok(Some(Incoming::Publish { .. }))), "length {n}");
    }
}

#[test]
fn decodes_broker_packets_in_any_chunking() {
    let stream: Vec<u8> = [
        &[0x20, 2, 1, 0][..],                                   // CONNACK, session present, accepted
        &[0x30, 7, 0, 3, b'a', b'/', b'b', b'h', b'i'][..],     // PUBLISH QoS 0
        &[0x33, 9, 0, 3, b'a', b'/', b'b', 0, 7, b'h', b'i'][..], // PUBLISH QoS 1, retain
        &[0x90, 4, 0, 1, 0, 0x80][..],                          // SUBACK
        &[0xD0, 0][..],                                         // PINGRESP
    ]
    .concat();
    let want = vec![
        Incoming::ConnAck { session_present: true, code: 0 },
        Incoming::Publish { topic: "a/b".into(), payload: b"hi".to_vec(), qos: QoS::AtMostOnce, pkid: 0, retain: false, dup: false },
        Incoming::Publish { topic: "a/b".into(), payload: b"hi".to_vec(), qos: QoS::AtLeastOnce, pkid: 7, retain: true, dup: false },
        Incoming::SubAck { pkid: 1, codes: vec![0, 0x80] },
        Incoming::PingResp,
    ];
    for chunk in [1, 2, 3, 7, stream.len()] {
        let mut d = Decoder::new(1024);
        let mut got = Vec::new();
        for part in stream.chunks(chunk) {
            d.feed(part);
            while let Some(p) = d.next_packet().unwrap() {
                got.push(p);
            }
        }
        assert_eq!(got, want, "chunk size {chunk}");
    }
}

#[test]
fn rejects_malformed_input() {
    let first_error = |bytes: &[u8], max: usize| {
        let mut d = Decoder::new(max);
        d.feed(bytes);
        d.next_packet().unwrap_err()
    };
    assert_eq!(first_error(&[0x30, 0xFF, 0xFF, 0xFF, 0xFF], 1 << 30), Error::MalformedLength);
    assert_eq!(first_error(&[0x30, 0x80, 0x00], 1 << 30), Error::MalformedLength);
    assert_eq!(first_error(&[0x30, 20], 10), Error::TooLarge(22));
    assert!(matches!(first_error(&[0x10, 0], 64), Error::Protocol(_))); // CONNECT from a broker
    assert!(matches!(first_error(&[0x36, 4, 0, 1, b'a', 0], 64), Error::Protocol(_))); // QoS 3
    assert!(matches!(first_error(&[0x30, 3, 0, 1, b'#'], 64), Error::Protocol(_)));
    assert_eq!(first_error(&[0x30, 3, 0, 1, 0xFF], 64), Error::Utf8);
    assert_eq!(first_error(&[0x30, 3, 0, 1, 0x00], 64), Error::Utf8);
    assert!(matches!(first_error(&[0x20, 3, 0, 0, 0], 64), Error::Field(_)));
    assert!(matches!(first_error(&[0x20, 2, 2, 0], 64), Error::Protocol(_)));
}

#[test]
fn keep_alive_is_a_pure_timer() {
    let mut k = KeepAlive::new(60, 0);
    assert_eq!(k.poll(0), Tick::Wait(60_000));
    k.on_sent(30_000);
    assert_eq!(k.poll(60_000), Tick::Wait(90_000));
    assert_eq!(k.poll(90_000), Tick::Ping);
    assert_eq!(k.poll(100_000), Tick::Wait(150_000));
    k.on_ping_resp();
    assert_eq!(k.poll(100_000), Tick::Wait(150_000));
    assert_eq!(k.poll(150_000), Tick::Ping);
    assert_eq!(k.poll(210_000), Tick::Lost);
    assert_eq!(KeepAlive::new(0, 0).poll(1 << 40), Tick::Wait(u64::MAX));
}
