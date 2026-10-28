//! The Syslog bridge (ADR-0020 amendment C-057): `log` records of the
//! application become `Pub/Event/<AppId>/Syslog/<Level>` against the
//! in-process TLS test broker. Its own test binary: the bridge is the
//! global logger of the process.

mod common;
mod testbroker;

use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{Duration, Instant};

use oec::app::ConnectionState;
use oec::event::SyslogLevel;
use oec::prelude::*;
use oec_core::mqtt::{Delivery, Packet, Publish};
use serde_json::Value;

use common::{Root, product};
use testbroker::{Behavior, TestBroker, WAIT, pki};

const REQUEST_ID: &str = "1700000000000-Aggregation/vendor.com/Client/C-1/1";

fn json(payload: &[u8]) -> Value {
    serde_json::from_slice(payload).unwrap()
}

fn encoded(il: &str) -> String {
    oec_core::il::IdentificationLink::parse(il)
        .unwrap()
        .encode()
}

/// The next PUBLISH on `topic`; the others in between go to `others`.
fn next_on(tb: &TestBroker, topic: &str, others: &mut Vec<Publish>) -> Publish {
    loop {
        match tb.packet() {
            (_, Packet::Publish(p)) if p.topic == topic => return p,
            (_, Packet::Publish(p)) => others.push(p),
            (_, Packet::PingReq | Packet::Subscribe(_)) => {}
            other => panic!("expected PUBLISH, got {other:?}"),
        }
    }
}

/// The next PUBLISH.
fn next_any(tb: &TestBroker) -> Publish {
    loop {
        match tb.packet() {
            (_, Packet::Publish(p)) => return p,
            (_, Packet::PingReq | Packet::Subscribe(_)) => {}
            other => panic!("expected PUBLISH, got {other:?}"),
        }
    }
}

/// The running application under test.
struct Run {
    tb: TestBroker,
    n: usize,
    prefix: String,
    me: String,
    il: String,
    handle: AppHandle,
    thread: thread::JoinHandle<oec::Result<()>>,
}

impl Run {
    fn start(root: &Root) -> Self {
        let pki = pki();
        let tb = TestBroker::start(&pki, Behavior::default());
        root.write("ca.pem", &pki.ca);
        let broker = Broker::new("localhost")
            .port(tb.port)
            .ca_pem(root.path("ca.pem"))
            .credentials("user", "pa:ss");
        let online = Arc::new(Mutex::new(false));
        let flag = online.clone();
        let app = App::builder(Level::L2, ServiceType::Utility, product())
            .reference_designation(false)
            .bootstrap(Bootstrap::Explicit(broker))
            .serial_number("SN-1")
            .client_id("app-1")
            .keep_alive(Duration::from_secs(1))
            .on_connection(move |s| {
                if s == ConnectionState::Online {
                    *flag.lock().unwrap() = true;
                }
            })
            .build()
            .unwrap();
        // the default Notice: the bridge lets Warn and Error through
        assert_eq!(log::max_level(), log::LevelFilter::Warn);
        let il = app.identification_link().as_str().to_owned();
        let me = encoded(&il);
        let prefix = format!("Oi4v2/Utility/{me}/");
        let handle = app.handle();
        let thread = thread::spawn(move || app.run());
        let (n, _) = tb.connect();
        next_on(
            &tb,
            &format!("{prefix}Pub/PublicationList"),
            &mut Vec::new(),
        );
        wait_until(|| *online.lock().unwrap());
        Self {
            tb,
            n,
            prefix,
            me,
            il,
            handle,
            thread,
        }
    }

    fn topic(&self, level: &str) -> String {
        format!("{}Pub/Event/{}/Syslog/{level}", self.prefix, self.me)
    }

    /// The Payload of the next Syslog event of `level`; the other Syslog
    /// events in between go to `others`.
    fn next_event(&self, level: &str, others: &mut Vec<Publish>) -> Value {
        let p = next_on(&self.tb, &self.topic(level), others);
        let v = json(&p.payload);
        assert_eq!(v["MessageType"], "ua-data");
        assert_eq!(v["Messages"][0]["MessageType"], "ua-event");
        v["Messages"][0]["Payload"].clone()
    }

    /// Switches the Syslog `level` with `Set PublicationList`
    /// (REQ-TOP-027) and waits for the Good Status event.
    fn switch(&self, level: &str, mode: &str) {
        let mut entry = self.entry(level);
        entry["Mode"] = Value::from(mode);
        let set = serde_json::json!({
            "MessageId": REQUEST_ID,
            "MessageType": "ua-data",
            "PublisherId": "Aggregation/vendor.com/Client/C-1/1",
            "ReplyTo": "",
            "Messages": [{"DataSetWriterId": 20, "DataSetWriterName": self.il, "Payload": entry}]
        });
        let topic = format!(
            "{}Set/PublicationList/{}/Event/Syslog/{level}",
            self.prefix, self.me
        );
        self.tb.send(
            self.n,
            Packet::Publish(Publish {
                topic,
                payload: serde_json::to_vec(&set).unwrap(),
                delivery: Delivery::AtMostOnce,
                retain: false,
            }),
        );
        let status = format!("{}Pub/Event/{}/Status/", self.prefix, self.me);
        loop {
            let p = next_any(&self.tb);
            if p.topic.starts_with(&status) && json(&p.payload)["ReplyTo"] == REQUEST_ID {
                assert!(
                    p.topic.ends_with("/Good"),
                    "{}",
                    String::from_utf8_lossy(&p.payload)
                );
                return;
            }
        }
    }

    /// The entry of the Syslog `level` from `Get PublicationList`.
    fn entry(&self, level: &str) -> Value {
        let get = serde_json::json!({
            "MessageId": REQUEST_ID,
            "MessageType": "ua-data",
            "PublisherId": "Aggregation/vendor.com/Client/C-1/1",
            "ReplyTo": "",
            "Messages": []
        });
        let topic = format!("{}Get/PublicationList", self.prefix);
        self.tb.send(
            self.n,
            Packet::Publish(Publish {
                topic,
                payload: serde_json::to_vec(&get).unwrap(),
                delivery: Delivery::AtMostOnce,
                retain: false,
            }),
        );
        let list = format!("{}Pub/PublicationList", self.prefix);
        let filter = format!("Syslog/{level}");
        loop {
            let v = json(&next_on(&self.tb, &list, &mut Vec::new()).payload);
            if v["ReplyTo"] != REQUEST_ID {
                continue;
            }
            return v["Messages"]
                .as_array()
                .unwrap()
                .iter()
                .find(|e| e["Payload"]["Filter"] == filter)
                .map(|e| e["Payload"].clone())
                .expect("the Syslog levels are listed");
        }
    }

    fn stop(self) {
        self.handle.shutdown();
        self.thread.join().unwrap().unwrap();
    }
}

fn wait_until(f: impl Fn() -> bool) {
    let start = Instant::now();
    while !f() {
        assert!(start.elapsed() < WAIT, "condition not reached");
        thread::sleep(Duration::from_millis(5));
    }
}

fn msg(payload: &Value) -> String {
    payload["Details"]["MSG"].as_str().unwrap().to_owned()
}

/// `Warn` → `Warning` with the PRI, `MSG` and `HEADER` (REQ-RES-EVENT-007);
/// `Info` is off by default (REQ-TOP-024); the records of the SDK are not
/// published.
fn records_are_mapped(r: &Run) {
    log::warn!("pump {} is dry", 1);
    let warning = r.next_event("Warning", &mut Vec::new());
    assert_eq!(warning["Category"], "CAT_SYSLOG_0");
    assert_eq!(warning["Number"], 8 + 4);
    let pid = std::process::id();
    let m = msg(&warning);
    assert!(m.ends_with(&format!("[{pid}]: pump 1 is dry")), "{m}");
    assert!(!warning["Details"]["HEADER"].as_str().unwrap().is_empty());

    log::info!("not published");
    log::warn!(target: "oec::runtime", "a record of the SDK");
    log::warn!(target: "rustls::conn", "a record of the transport");
    log::error!("marker");
    let mut others = Vec::new();
    let error = r.next_event("Error", &mut others);
    assert_eq!(error["Number"], 8 + 3);
    assert!(msg(&error).ends_with(": marker"));
    assert!(
        !others.iter().any(|p| p.topic.contains("/Syslog/")),
        "{:?}",
        others.iter().map(|p| &p.topic).collect::<Vec<_>>()
    );
}

/// `Set PublicationList` switches `Informational` on (REQ-TOP-027): the
/// bridge raises the `log` maximum level and publishes `Info`;
/// [`oec::event::syslog`] and [`oec::app::AppHandle::emit_syslog`] publish
/// directly; a level that is off is dropped.
fn levels_are_switched(r: &Run) {
    r.switch("Informational", "FILTER_4");
    wait_until(|| log::max_level() == log::LevelFilter::Info);
    log::info!("now visible");
    let info = r.next_event("Informational", &mut Vec::new());
    assert_eq!(info["Number"], 8 + 6);
    assert!(msg(&info).ends_with(": now visible"));
    oec::event::syslog(SyslogLevel::Notice, "direct");
    let notice = r.next_event("Notice", &mut Vec::new());
    assert!(msg(&notice).ends_with(": direct"));
    r.handle.emit_syslog(SyslogLevel::Debug, "off");
    r.handle.emit_syslog(SyslogLevel::Notice, "of this app");
    let notice = r.next_event("Notice", &mut Vec::new());
    assert!(msg(&notice).ends_with(": of this app"));
    r.switch("Informational", "OFF_0");
    wait_until(|| log::max_level() == log::LevelFilter::Warn);
}

/// Beyond the burst the records are dropped; their number follows as a
/// `Warning` with the next published record.
fn bursts_are_limited(r: &Run) {
    for i in 0..80 {
        log::error!("burst {i}");
    }
    let mut published = 0;
    let mut others = Vec::new();
    while published < 40 {
        r.next_event("Error", &mut others);
        published += 1;
    }
    thread::sleep(Duration::from_millis(250));
    log::error!("after the burst");
    let mut between = Vec::new();
    let warning = r.next_event("Warning", &mut between);
    assert!(
        msg(&warning).contains("log records dropped by the rate limit"),
        "{}",
        msg(&warning)
    );
    let errors = others
        .iter()
        .chain(&between)
        .filter(|p| p.topic.ends_with("/Syslog/Error"))
        .count()
        + published;
    assert!(errors < 80, "{errors} of 80 published");
}

#[test]
fn log_records_become_syslog_events() {
    let root = Root::new("syslog");
    let r = Run::start(&root);
    records_are_mapped(&r);
    levels_are_switched(&r);
    bursts_are_limited(&r);
    r.stop();
}
