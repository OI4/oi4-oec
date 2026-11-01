//! The test broker of the Go SDK tests (ADR-0027 §9, C-076): the routing
//! in-process TLS test broker of `oec` (`oec/tests/testbroker`) as a
//! program, driven over stdin/stdout with one JSON object per line.
//!
//! `go-testbroker <dir>` writes the PKI to `<dir>` (`ca.pem`,
//! `client.pem`, `client.key`) and prints
//! `{"port":N,"ca":"<dir>/ca.pem","client_cert":…,"client_key":…}`.
//! Then it prints what it sees, per connection `conn` (from 0):
//! `{"seen":"tls","conn":0,"client_cert":false}`,
//! `{"seen":"connect","conn":0,"client_id":…,"username":…,"will":…}`,
//! `{"seen":"publish","conn":0,"topic":…,"payload":…,"retain":false}`,
//! `{"seen":"subscribe","conn":0,"filters":[…]}`, `"unsubscribe"`,
//! `"disconnect"` and `{"seen":"closed","conn":0}` (payloads as UTF-8,
//! lossy). Commands on stdin:
//! `{"cmd":"send","conn":0,"topic":…,"payload":…}` sends a PUBLISH to one
//! client, `{"cmd":"drop","conn":0}` drops a connection without
//! DISCONNECT. It ends when stdin is closed. Loopback only, no network.

#[path = "../../../oec/tests/testbroker/mod.rs"]
mod testbroker;

use std::io::{BufRead, Write};
use std::path::PathBuf;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, mpsc};
use std::{env, fs, io, mem, process, thread};

use oec_core::mqtt::{Delivery, Packet, Publish};
use serde_json::{Value, json};
use testbroker::{Seen, TestBroker, pki};

fn main() {
    let Some(dir) = env::args_os().nth(1).map(PathBuf::from) else {
        eprintln!("usage: go-testbroker <dir>");
        process::exit(2);
    };
    let pki = pki();
    fs::create_dir_all(&dir).expect("create the directory");
    let ca = dir.join("ca.pem");
    let cert = dir.join("client.pem");
    let key = dir.join("client.key");
    fs::write(&ca, &pki.ca).expect("write ca.pem");
    fs::write(&cert, &pki.client_cert).expect("write client.pem");
    fs::write(&key, pki.client_key.serialize_pem()).expect("write client.key");

    let mut tb = TestBroker::routing(&pki);
    print(&json!({"port": tb.port, "ca": ca, "client_cert": cert, "client_key": key}));

    let seen = mem::replace(&mut tb.seen, mpsc::channel().1);
    // The number of connections the broker reported (a guard for "drop").
    let known = Arc::new(AtomicUsize::new(0));
    let reported = Arc::clone(&known);
    thread::spawn(move || {
        while let Ok(s) = seen.recv() {
            let (Seen::Tls(n, _) | Seen::Packet(n, _) | Seen::Closed(n)) = s;
            reported.fetch_max(n + 1, Ordering::SeqCst);
            print(&describe(s));
        }
    });
    for line in io::stdin().lock().lines() {
        let Ok(line) = line else { break };
        if let Err(e) = command(&tb, &line, known.load(Ordering::SeqCst)) {
            eprintln!("go-testbroker: {e}: {line}");
        }
    }
    // Holding stdout lets the printer finish its line and stops it.
    let _out = io::stdout().lock();
    process::exit(0);
}

fn command(tb: &TestBroker, line: &str, known: usize) -> Result<(), String> {
    let v: Value = serde_json::from_str(line).map_err(|e| e.to_string())?;
    let conn = v["conn"]
        .as_u64()
        .and_then(|n| usize::try_from(n).ok())
        .filter(|n| *n < known)
        .ok_or("no such conn")?;
    match v["cmd"].as_str() {
        Some("send") => {
            let topic = v["topic"].as_str().ok_or("no topic")?.to_owned();
            let payload = v["payload"]
                .as_str()
                .unwrap_or_default()
                .as_bytes()
                .to_vec();
            tb.send(
                conn,
                Packet::Publish(Publish {
                    topic,
                    payload,
                    delivery: Delivery::AtMostOnce,
                    retain: false,
                }),
            );
            Ok(())
        }
        Some("drop") => {
            tb.drop_connection(conn);
            Ok(())
        }
        _ => Err("unknown cmd".into()),
    }
}

fn describe(seen: Seen) -> Value {
    match seen {
        Seen::Tls(n, client_cert) => json!({"seen": "tls", "conn": n, "client_cert": client_cert}),
        Seen::Closed(n) => json!({"seen": "closed", "conn": n}),
        Seen::Packet(n, packet) => match packet {
            Packet::Connect(c) => json!({
                "seen": "connect", "conn": n, "client_id": c.client_id,
                "username": c.username, "will": c.will.map(|w| w.topic),
            }),
            Packet::Publish(p) => json!({
                "seen": "publish", "conn": n, "topic": p.topic,
                "payload": String::from_utf8_lossy(&p.payload), "retain": p.retain,
            }),
            Packet::Subscribe(s) => json!({
                "seen": "subscribe", "conn": n,
                "filters": s.filters.into_iter().map(|(f, _)| f).collect::<Vec<_>>(),
            }),
            Packet::Unsubscribe(u) => {
                json!({"seen": "unsubscribe", "conn": n, "filters": u.filters})
            }
            Packet::Disconnect => json!({"seen": "disconnect", "conn": n}),
            _ => json!({"seen": "other", "conn": n}),
        },
    }
}

fn print(v: &Value) {
    let mut out = io::stdout().lock();
    if writeln!(out, "{v}").and_then(|()| out.flush()).is_err() {
        process::exit(0);
    }
}
