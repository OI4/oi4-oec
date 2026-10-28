//! The C examples and the C test program against the built library
//! (ADR-0021 §9, ADR-0026 §20): each is compiled with the system C
//! compiler (C17, all warnings as errors) and runs against the in-process
//! TLS test broker of `oec` – once linked to the shared library and once
//! to the static library under AddressSanitizer and UBSan. Without
//! sanitizer support in the toolchain the second variant is skipped unless
//! `OEC_FFI_REQUIRE_SANITIZERS=1` (set by `oec/sdk/check.sh`).
//!
//! Do not run this test binary in parallel processes: the roots below
//! `CARGO_TARGET_TMPDIR` are fixed per test.

#[path = "../../oec/tests/testbroker/mod.rs"]
mod testbroker;

use std::io::{BufRead, BufReader};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::OnceLock;
use std::sync::mpsc::{Receiver, channel};
use std::thread;
use std::time::{Duration, Instant};

use oec::app::{AppHandle, ConnectionState};
use oec::prelude::*;
use oec_core::mqtt::Packet;
use testbroker::{Behavior, Pki, Seen, TestBroker, WAIT, pki};

// ---------------------------------------------------------------- build

/// How a program is linked.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
enum Link {
    /// `liboec_ffi` as shared library.
    Shared,
    /// `liboec_ffi.a` with `-fsanitize=address,undefined`.
    StaticSanitized,
}

impl Link {
    fn suffix(self) -> &'static str {
        match self {
            Link::Shared => "shared",
            Link::StaticSanitized => "asan",
        }
    }
}

const SANITIZE: &[&str] = &[
    "-fsanitize=address,undefined",
    "-fno-omit-frame-pointer",
    "-fno-sanitize-recover=all",
];

/// The libraries the static library needs (`--print native-static-libs`).
#[cfg(target_os = "macos")]
const NATIVE: &[&str] = &["-liconv", "-lSystem", "-lc", "-lm"];
#[cfg(not(target_os = "macos"))]
const NATIVE: &[&str] = &[
    "-lgcc_s",
    "-lutil",
    "-lrt",
    "-lpthread",
    "-lm",
    "-ldl",
    "-lc",
];

fn crate_dir() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
}

/// `bin` (programs) or `run` (roots, stores) below the test directory.
fn out_dir(kind: &str) -> PathBuf {
    let dir = PathBuf::from(env!("CARGO_TARGET_TMPDIR"))
        .join("oec-ffi-c")
        .join(kind);
    std::fs::create_dir_all(&dir).unwrap();
    dir
}

/// The directory with `liboec_ffi` of this build (next to the test
/// binary or one level up).
fn lib_dir() -> PathBuf {
    let exe = std::env::current_exe().unwrap();
    let deps = exe.parent().unwrap().to_path_buf();
    [deps.clone(), deps.parent().unwrap().to_path_buf()]
        .into_iter()
        .find(|d| d.join("liboec_ffi.a").exists())
        .expect("liboec_ffi.a next to the test binary")
}

fn cc() -> String {
    std::env::var("CC").unwrap_or_else(|_| "cc".to_owned())
}

/// Compiles `source` (relative to the crate) to an executable.
fn compile(source: &str, link: Link) -> PathBuf {
    let name = Path::new(source).file_stem().unwrap().to_str().unwrap();
    let exe = out_dir("bin").join(format!("{name}-{}", link.suffix()));
    let lib = lib_dir();
    let mut cmd = Command::new(cc());
    cmd.args([
        "-std=c17",
        "-Wall",
        "-Wextra",
        "-Wpedantic",
        "-Werror",
        "-g",
    ])
    .arg("-I")
    .arg(crate_dir().join("include"))
    .arg(crate_dir().join(source))
    .arg("-o")
    .arg(&exe)
    .arg("-pthread");
    match link {
        Link::Shared => {
            cmd.arg(format!("-L{}", lib.display()))
                .arg("-loec_ffi")
                .arg(format!("-Wl,-rpath,{}", lib.display()));
        }
        Link::StaticSanitized => {
            cmd.args(SANITIZE)
                .arg(lib.join("liboec_ffi.a"))
                .args(NATIVE);
        }
    }
    let out = cmd.output().expect("C compiler");
    assert!(
        out.status.success(),
        "{cmd:?}\n{}",
        String::from_utf8_lossy(&out.stderr)
    );
    exe
}

/// Whether the toolchain builds and runs a sanitized program; required
/// with `OEC_FFI_REQUIRE_SANITIZERS=1`.
fn sanitizers() -> bool {
    static OK: OnceLock<bool> = OnceLock::new();
    *OK.get_or_init(|| {
        let dir = out_dir("bin");
        let src = dir.join("probe.c");
        let exe = dir.join("probe");
        std::fs::write(&src, "int main(void) { return 0; }\n").unwrap();
        let ok = Command::new(cc())
            .args(SANITIZE)
            .arg(&src)
            .arg("-o")
            .arg(&exe)
            .output()
            .is_ok_and(|o| o.status.success())
            && Command::new(&exe).status().is_ok_and(|s| s.success());
        if !ok {
            assert!(
                std::env::var("OEC_FFI_REQUIRE_SANITIZERS").as_deref() != Ok("1"),
                "OEC_FFI_REQUIRE_SANITIZERS=1, but {} cannot build with {SANITIZE:?}",
                cc()
            );
            eprintln!("skipped: no AddressSanitizer/UBSan in {}", cc());
        }
        ok
    })
}

/// The links to test: both, or the shared one without sanitizers.
fn links() -> Vec<Link> {
    if sanitizers() {
        vec![Link::Shared, Link::StaticSanitized]
    } else {
        vec![Link::Shared]
    }
}

// ---------------------------------------------------------------- run

/// A running C program with its stdout lines.
struct Program {
    child: Child,
    lines: Receiver<String>,
    seen: Vec<String>,
}

impl Program {
    fn start(exe: &Path, args: &[&str], root: Option<&Path>) -> Self {
        let mut cmd = Command::new(exe);
        cmd.args(args)
            .env("ASAN_OPTIONS", "detect_leaks=0:halt_on_error=1")
            .env("UBSAN_OPTIONS", "print_stacktrace=1:halt_on_error=1")
            .stdout(Stdio::piped());
        if let Some(root) = root {
            cmd.env("OEC_ROOT", root);
        }
        let mut child = cmd.spawn().unwrap();
        let stdout = child.stdout.take().unwrap();
        let (tx, lines) = channel();
        thread::spawn(move || {
            for line in BufReader::new(stdout).lines().map_while(Result::ok) {
                if tx.send(line).is_err() {
                    break;
                }
            }
        });
        Self {
            child,
            lines,
            seen: Vec::new(),
        }
    }

    /// Waits for a line with `prefix` that satisfies `matches`.
    fn line(&mut self, prefix: &str, matches: impl Fn(&str) -> bool) -> String {
        let start = Instant::now();
        loop {
            let left = WAIT.saturating_sub(start.elapsed());
            match self.lines.recv_timeout(left) {
                Ok(l) if l.starts_with(prefix) && matches(&l) => return l,
                Ok(l) => self.seen.push(l),
                Err(e) => panic!("no line {prefix:?} ({e}); seen: {:#?}", self.seen),
            }
        }
    }

    /// SIGTERM; the program shuts down and exits with 0.
    fn terminate(mut self) {
        let pid = self.child.id().to_string();
        assert!(
            Command::new("kill")
                .args(["-TERM", &pid])
                .status()
                .unwrap()
                .success()
        );
        self.exits_with_success();
    }

    fn exits_with_success(&mut self) {
        let start = Instant::now();
        loop {
            if let Some(status) = self.child.try_wait().unwrap() {
                assert!(status.success(), "{status}; stdout: {:#?}", self.seen);
                return;
            }
            assert!(start.elapsed() < WAIT, "the program does not exit");
            thread::sleep(Duration::from_millis(5));
        }
    }
}

impl Drop for Program {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

/// An empty directory below the target directory.
fn dir(name: &str) -> PathBuf {
    let dir = out_dir("run").join(name);
    let _ = std::fs::remove_dir_all(&dir);
    std::fs::create_dir_all(&dir).unwrap();
    dir
}

fn write(path: &Path, content: &str) {
    std::fs::create_dir_all(path.parent().unwrap()).unwrap();
    std::fs::write(path, content).unwrap();
}

/// A container storage (REQ-CNT-004) for the test broker on `port`.
fn container(name: &str, pki: &Pki, port: u16) -> PathBuf {
    let root = dir(name);
    write(&root.join("etc/hostname"), "app-1\n");
    write(
        &root.join("etc/oi4/mqtt/broker.json"),
        &format!(r#"{{"Address": "localhost", "SecurePort": {port}, "MaxPacketSize": 262144}}"#),
    );
    write(&root.join("etc/oi4/certs/ca.pem"), &pki.ca);
    // base64("user:pa:ss")
    write(
        &root.join("run/secrets/oi4/credentials"),
        "dXNlcjpwYTpzcw==\n",
    );
    std::fs::create_dir_all(root.join("etc/oi4/app")).unwrap();
    root
}

fn payload(p: &oec_core::mqtt::Publish) -> serde_json::Value {
    serde_json::from_slice(&p.payload).unwrap()
}

// ---------------------------------------------------------------- tests

/// hello.c: Birth, Health, and on SIGTERM Close and DISCONNECT
/// (REQ-LIFE-001, REQ-LIFE-005).
#[test]
fn hello_runs_and_closes() {
    for link in links() {
        let exe = compile("examples/hello.c", link);
        let pki = pki();
        let tb = TestBroker::start(&pki, Behavior::default());
        let root = container(&format!("hello-{}", link.suffix()), &pki, tb.port);
        let mut hello = Program::start(&exe, &[], Some(&root));
        hello.line("running", |_| true);

        let (n, connect) = tb.connect();
        assert_eq!(connect.client_id, "app-1", "{link:?}");
        let will = connect.will.expect("Will");
        let birth = next_publish(&tb);
        assert!(birth.topic.contains("/Pub/MAM/"), "{}", birth.topic);
        let health = next_publish(&tb);
        assert_eq!(health.topic, will.topic);

        hello.terminate();
        let tail: Vec<_> = tb
            .until_closed(n)
            .into_iter()
            .filter(|p| !matches!(p, Packet::PingReq))
            .collect();
        match tail.as_slice() {
            [Packet::Publish(close), Packet::Disconnect] => {
                assert_eq!(close.topic, will.topic);
                assert_eq!(payload(close)["Messages"][0]["Payload"]["HealthScore"], 0);
            }
            other => panic!("{link:?}: expected Close, DISCONNECT; got {other:?}"),
        }
    }
}

/// Waits until the broker saw SUBSCRIBEs of filters with each of `parts`.
fn wait_subscribed(tb: &TestBroker, parts: &[&str]) {
    let mut missing = parts.to_vec();
    while !missing.is_empty() {
        if let Seen::Packet(_, Packet::Subscribe(s)) = tb.next() {
            missing.retain(|part| !s.filters.iter().any(|(f, _)| f.contains(part)));
        }
    }
}

/// The next PUBLISH, skipping other packets.
fn next_publish(tb: &TestBroker) -> oec_core::mqtt::Publish {
    loop {
        if let (_, Packet::Publish(p)) = tb.packet() {
            return p;
        }
    }
}

/// A Rust application on the routing broker that emits a Generic event.
fn peer(pki: &Pki, tb: &TestBroker, name: &str) -> (AppHandle, thread::JoinHandle<()>) {
    let root = dir(name);
    write(&root.join("ca.pem"), &pki.ca);
    let broker = Broker::new("localhost")
        .port(tb.port)
        .ca_pem(root.join("ca.pem"))
        .credentials("user", "pa:ss");
    let app = App::builder(
        Level::L1,
        ServiceType::Utility,
        Product::new("example.com", "Peer", "PEER-1").software_revision("1.0.0"),
    )
    .store(oec::store::FileStore::new(root.join("store")))
    .bootstrap(Bootstrap::Explicit(broker))
    .serial_number("peer")
    .build()
    .unwrap();
    let handle = app.handle();
    (handle, thread::spawn(move || app.run().unwrap()))
}

/// logger.c: prints the Health and the Events of another application
/// (REQ-RES-SUBLIST-001, REQ-RES-EVENT-009).
#[test]
fn logger_prints_events_and_health() {
    const PEER: &str = "example.com/Peer/PEER-1/peer";
    for link in links() {
        let exe = compile("examples/logger.c", link);
        let pki = pki();
        let tb = TestBroker::routing(&pki);
        let root = container(&format!("logger-{}", link.suffix()), &pki, tb.port);
        let mut logger = Program::start(&exe, &[], Some(&root));
        logger.line("logging", |_| true);
        wait_subscribed(&tb, &["/Pub/Health", "/Pub/Event"]);

        let (peer, run) = peer(&pki, &tb, &format!("peer-{}", link.suffix()));
        let health = logger.line("Health", |l| l.contains(PEER));
        assert!(health.contains("\"Health\":\"NORMAL_0\""), "{health}");
        let start = Instant::now();
        while peer.state() != ConnectionState::Online {
            assert!(start.elapsed() < WAIT);
            thread::sleep(Duration::from_millis(5));
        }
        peer.this()
            .emit_event(GenericEvent::new("Maintenance", 42).description("check"))
            .unwrap();
        let event = logger.line("Event", |l| l.contains(PEER));
        assert!(
            event.contains(" Maintenance ") || event.contains("/Maintenance"),
            "{event}"
        );
        assert!(event.contains("\"Number\":42"), "{event}");
        peer.shutdown();
        run.join().unwrap();
        logger.terminate();
    }
}

/// tests/c/abi.c: the whole ABI with two apps in one process, the
/// callback rules and threads under load (REQ-MSG-015, REQ-CMP-047,
/// REQ-ACT-017, REQ-RES-EVENT-005).
#[test]
fn abi_program() {
    for link in links() {
        let exe = compile("tests/c/abi.c", link);
        let pki = pki();
        let tb = TestBroker::routing(&pki);
        let work = dir(&format!("abi-{}", link.suffix()));
        let ca = work.join("ca.pem");
        write(&ca, &pki.ca);
        let port = tb.port.to_string();
        let mut abi = Program::start(
            &exe,
            &[&port, ca.to_str().unwrap(), work.to_str().unwrap()],
            None,
        );
        abi.line("abi ok", |_| true);
        abi.exits_with_success();
    }
}
