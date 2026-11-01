//! A minimal HTTP/1.1 server on `std::net` for the API of the config
//! service (A-357): one request per connection (`Connection: close`),
//! bounded in size, time and number of connections. Without TLS and
//! authentication, like every demo UI (A-305): it listens on loopback by
//! default and refuses requests whose `Host` is a DNS name other than
//! `localhost` or whose `Origin` is another site (DNS rebinding, CSRF).

use std::collections::HashMap;
use std::io::{Read, Write};
use std::net::{IpAddr, TcpListener, TcpStream};
use std::sync::Arc;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::{Duration, Instant};

/// At most this many connections at a time; more get a 503.
pub(crate) const MAX_CONNECTIONS: usize = 16;
/// Request line and headers.
pub(crate) const MAX_HEAD: usize = 8 * 1024;
/// Body.
pub(crate) const MAX_BODY: usize = 64 * 1024;
/// Time to receive a request.
const READ_TIME: Duration = Duration::from_secs(5);
/// Time to drop what a client sends after the response.
const DRAIN_TIME: Duration = Duration::from_millis(100);

/// A request.
#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct Request {
    pub(crate) method: String,
    /// The path without query.
    pub(crate) path: String,
    pub(crate) body: Vec<u8>,
}

/// A response.
#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct Response {
    pub(crate) status: u16,
    pub(crate) content_type: &'static str,
    pub(crate) body: Vec<u8>,
    /// Extra headers, e.g. `Allow`.
    pub(crate) headers: Vec<(&'static str, String)>,
}

impl Response {
    /// A JSON response.
    pub(crate) fn json(status: u16, value: &serde_json::Value) -> Self {
        Self {
            status,
            content_type: "application/json",
            body: value.to_string().into_bytes(),
            headers: Vec::new(),
        }
    }

    /// An error as `{"error": message}`.
    pub(crate) fn error(status: u16, message: &str) -> Self {
        Self::json(status, &serde_json::json!({ "error": message }))
    }

    /// A text response.
    pub(crate) fn text(status: u16, text: &str) -> Self {
        Self {
            status,
            content_type: "text/plain; charset=utf-8",
            body: text.as_bytes().to_vec(),
            headers: Vec::new(),
        }
    }
}

fn reason(status: u16) -> &'static str {
    match status {
        200 => "OK",
        400 => "Bad Request",
        403 => "Forbidden",
        404 => "Not Found",
        405 => "Method Not Allowed",
        408 => "Request Timeout",
        409 => "Conflict",
        413 => "Content Too Large",
        415 => "Unsupported Media Type",
        431 => "Request Header Fields Too Large",
        501 => "Not Implemented",
        503 => "Service Unavailable",
        504 => "Gateway Timeout",
        _ => "Error",
    }
}

/// The handler of the requests.
pub(crate) type Handler = Arc<dyn Fn(&Request) -> Response + Send + Sync>;

/// Serves `listener` until it fails; one thread per connection.
pub(crate) fn serve(listener: &TcpListener, handler: &Handler) {
    let open = Arc::new(AtomicUsize::new(0));
    for stream in listener.incoming() {
        let Ok(mut stream) = stream else { continue };
        if open.fetch_add(1, Ordering::SeqCst) >= MAX_CONNECTIONS {
            open.fetch_sub(1, Ordering::SeqCst);
            let _ = stream.set_write_timeout(Some(READ_TIME));
            let _ = write(&mut stream, &Response::error(503, "too many connections"));
            finish(&mut stream);
            continue;
        }
        let (open, handler) = (Arc::clone(&open), Arc::clone(handler));
        let spawned = std::thread::Builder::new()
            .name("oec-config-http".to_owned())
            .spawn(move || {
                connection(stream, &handler);
                open.fetch_sub(1, Ordering::SeqCst);
            });
        if spawned.is_err() {
            log::warn!("HTTP: no thread for a connection");
        }
    }
}

fn connection(mut stream: TcpStream, handler: &Handler) {
    let _ = stream.set_read_timeout(Some(READ_TIME));
    let _ = stream.set_write_timeout(Some(READ_TIME));
    let response = match read(&mut stream) {
        Ok(req) => handler(&req),
        Err(resp) => resp,
    };
    let _ = write(&mut stream, &response);
    finish(&mut stream);
}

/// Closes after the response: what the client still sends is read and
/// dropped for a short time, so that the close does not reset the
/// connection before the client has read the response.
fn finish(stream: &mut TcpStream) {
    let _ = stream.shutdown(std::net::Shutdown::Write);
    let _ = stream.set_read_timeout(Some(DRAIN_TIME));
    let mut sink = [0u8; 4096];
    let mut left = MAX_HEAD + MAX_BODY;
    while left > 0 {
        match stream.read(&mut sink) {
            Ok(0) | Err(_) => break,
            Ok(n) => left = left.saturating_sub(n),
        }
    }
}

fn write(stream: &mut TcpStream, r: &Response) -> std::io::Result<()> {
    let mut head = format!(
        "HTTP/1.1 {} {}\r\nContent-Type: {}\r\nContent-Length: {}\r\nConnection: close\r\nCache-Control: no-store\r\nX-Content-Type-Options: nosniff\r\n",
        r.status,
        reason(r.status),
        r.content_type,
        r.body.len()
    );
    for (name, value) in &r.headers {
        use std::fmt::Write as _;
        let _ = write!(head, "{name}: {value}\r\n");
    }
    head.push_str("\r\n");
    stream.write_all(head.as_bytes())?;
    stream.write_all(&r.body)?;
    stream.flush()
}

fn find(haystack: &[u8], needle: &[u8]) -> Option<usize> {
    haystack.windows(needle.len()).position(|w| w == needle)
}

/// Reads one request; an error response for everything refused.
fn read(stream: &mut impl Read) -> Result<Request, Response> {
    let deadline = Instant::now() + READ_TIME;
    let mut buf = Vec::new();
    let mut chunk = [0u8; 4096];
    let head_end = loop {
        if let Some(i) = find(&buf, b"\r\n\r\n") {
            break i;
        }
        if buf.len() > MAX_HEAD {
            return Err(Response::error(431, "request head too large"));
        }
        let n = fill(stream, &mut chunk, deadline)?;
        buf.extend_from_slice(&chunk[..n]);
    };
    if head_end > MAX_HEAD {
        return Err(Response::error(431, "request head too large"));
    }
    let head = std::str::from_utf8(&buf[..head_end])
        .map_err(|_| Response::error(400, "request head is not UTF-8"))?;
    let mut lines = head.split("\r\n");
    let line = lines.next().unwrap_or_default();
    let mut parts = line.split(' ');
    let (Some(method), Some(target), Some(version), None) =
        (parts.next(), parts.next(), parts.next(), parts.next())
    else {
        return Err(Response::error(400, "invalid request line"));
    };
    if !version.starts_with("HTTP/1.") || !target.starts_with('/') {
        return Err(Response::error(400, "invalid request line"));
    }
    let mut headers = HashMap::new();
    for l in lines {
        let (name, value) = l
            .split_once(':')
            .ok_or_else(|| Response::error(400, "invalid header"))?;
        if headers
            .insert(name.trim().to_ascii_lowercase(), value.trim().to_owned())
            .is_some()
        {
            return Err(Response::error(400, "repeated header"));
        }
    }
    check_headers(method, &headers)?;
    let length = match headers.get("content-length") {
        None => 0,
        Some(v) => v
            .parse::<usize>()
            .map_err(|_| Response::error(400, "invalid Content-Length"))?,
    };
    if length > MAX_BODY {
        return Err(Response::error(413, "body too large"));
    }
    let mut body = buf[head_end + 4..].to_vec();
    while body.len() < length {
        let n = fill(stream, &mut chunk, deadline)?;
        body.extend_from_slice(&chunk[..n]);
    }
    if body.len() != length {
        return Err(Response::error(400, "body longer than Content-Length"));
    }
    let path = target.split('?').next().unwrap_or(target).to_owned();
    Ok(Request {
        method: method.to_owned(),
        path,
        body,
    })
}

fn fill(stream: &mut impl Read, chunk: &mut [u8], deadline: Instant) -> Result<usize, Response> {
    if Instant::now() > deadline {
        return Err(Response::error(408, "request too slow"));
    }
    match stream.read(chunk) {
        Ok(0) => Err(Response::error(400, "incomplete request")),
        Ok(n) => Ok(n),
        Err(_) => Err(Response::error(408, "request too slow")),
    }
}

/// The `Host` is `localhost` or an IP address; an `Origin` is the same
/// site; no `Transfer-Encoding`; a `POST` carries JSON.
fn check_headers(method: &str, headers: &HashMap<String, String>) -> Result<(), Response> {
    let host = headers
        .get("host")
        .ok_or_else(|| Response::error(400, "Host is required"))?;
    if !host_allowed(host) {
        return Err(Response::error(
            403,
            "Host is not localhost or an IP address",
        ));
    }
    if let Some(origin) = headers.get("origin")
        && *origin != format!("http://{host}")
    {
        return Err(Response::error(403, "cross-origin request"));
    }
    if headers.contains_key("transfer-encoding") {
        return Err(Response::error(501, "Transfer-Encoding is not supported"));
    }
    if method == "POST" {
        let json = headers
            .get("content-type")
            .and_then(|c| c.split(';').next())
            .is_some_and(|c| c.trim().eq_ignore_ascii_case("application/json"));
        if !json {
            return Err(Response::error(
                415,
                "Content-Type application/json is required",
            ));
        }
    }
    Ok(())
}

/// `localhost` or an IP literal, with an optional port.
fn host_allowed(host: &str) -> bool {
    let name = if let Some(rest) = host.strip_prefix('[') {
        match rest.split_once(']') {
            Some((ip, port)) if port.is_empty() || valid_port(port) => {
                return ip.parse::<std::net::Ipv6Addr>().is_ok();
            }
            _ => return false,
        }
    } else {
        match host.rsplit_once(':') {
            Some((name, port)) if valid_port(&format!(":{port}")) => name,
            Some(_) => return false,
            None => host,
        }
    };
    name.eq_ignore_ascii_case("localhost") || name.parse::<IpAddr>().is_ok()
}

fn valid_port(p: &str) -> bool {
    p.strip_prefix(':')
        .is_some_and(|p| !p.is_empty() && p.len() <= 5 && p.bytes().all(|b| b.is_ascii_digit()))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn parse(raw: &str) -> Result<Request, u16> {
        read(&mut raw.as_bytes()).map_err(|r| r.status)
    }

    #[test]
    fn requests() {
        let r = parse("GET /api/apps?x=1 HTTP/1.1\r\nHost: 127.0.0.1:8783\r\n\r\n").unwrap();
        assert_eq!((r.method.as_str(), r.path.as_str()), ("GET", "/api/apps"));
        let r = parse(
            "POST /api/set HTTP/1.1\r\nHost: localhost:8783\r\nOrigin: http://localhost:8783\r\nContent-Type: application/json; charset=utf-8\r\nContent-Length: 2\r\n\r\n{}",
        )
        .unwrap();
        assert_eq!(r.body, b"{}");
    }

    #[test]
    fn refused_requests() {
        let big = format!(
            "POST /api/set HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n",
            MAX_BODY + 1
        );
        let long_head = format!(
            "GET / HTTP/1.1\r\nHost: localhost\r\nX: {}\r\n\r\n",
            "a".repeat(MAX_HEAD)
        );
        for (raw, status) in [
            ("GET / HTTP/1.1\r\n\r\n", 400),
            ("GET / HTTP/1.1\r\nHost: evil.example:8783\r\n\r\n", 403),
            (
                "GET / HTTP/1.1\r\nHost: localhost\r\nOrigin: http://evil.example\r\n\r\n",
                403,
            ),
            (
                "GET / HTTP/1.1\r\nHost: localhost\r\nOrigin: null\r\n\r\n",
                403,
            ),
            (
                "POST /api/set HTTP/1.1\r\nHost: localhost\r\nContent-Length: 2\r\n\r\n{}",
                415,
            ),
            (
                "POST /api/set HTTP/1.1\r\nHost: localhost\r\nContent-Type: text/plain\r\nContent-Length: 2\r\n\r\n{}",
                415,
            ),
            (
                "POST / HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nTransfer-Encoding: chunked\r\n\r\n",
                501,
            ),
            (big.as_str(), 413),
            (long_head.as_str(), 431),
            ("GET / HTTP/1.1\r\nHost: a\r\nHost: b\r\n\r\n", 400),
            ("GET http://x/ HTTP/1.1\r\nHost: localhost\r\n\r\n", 400),
            (
                "GET / HTTP/1.1\r\nHost: localhost\r\nContent-Length: 4\r\n\r\nab",
                400,
            ),
            ("GET / HTTP/1.1\r\nHost: localhost\r\n", 400),
        ] {
            assert_eq!(parse(raw).unwrap_err(), status, "{raw:?}");
        }
    }

    #[test]
    fn hosts() {
        for (host, ok) in [
            ("localhost", true),
            ("LOCALHOST:8783", true),
            ("127.0.0.1", true),
            ("192.168.1.5:8783", true),
            ("[::1]:8783", true),
            ("[::1]", true),
            ("example.com", false),
            ("localhost.example.com", false),
            ("localhost:abc", false),
            ("[::1]x", false),
            ("[nope]", false),
        ] {
            assert_eq!(host_allowed(host), ok, "{host}");
        }
    }

    #[test]
    fn served_over_loopback_with_a_connection_limit() {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();
        let handler: Handler = Arc::new(|r: &Request| Response::text(200, &r.path));
        std::thread::spawn(move || serve(&listener, &handler));
        let get = |path: &str| {
            let mut s = TcpStream::connect(addr).unwrap();
            write!(s, "GET {path} HTTP/1.1\r\nHost: {addr}\r\n\r\n").unwrap();
            let mut out = Vec::new();
            let _ = s.read_to_end(&mut out);
            String::from_utf8(out).unwrap()
        };
        let out = get("/hello");
        assert!(out.starts_with("HTTP/1.1 200 OK\r\n"), "{out}");
        assert!(out.contains("Cache-Control: no-store\r\n"));
        assert!(out.contains("X-Content-Type-Options: nosniff\r\n"));
        assert!(out.ends_with("\r\n\r\n/hello"));
        // idle connections occupy the slots
        let idle: Vec<TcpStream> = (0..MAX_CONNECTIONS)
            .map(|_| TcpStream::connect(addr).unwrap())
            .collect();
        std::thread::sleep(Duration::from_millis(200));
        assert!(get("/x").starts_with("HTTP/1.1 503 "));
        drop(idle);
        let mut ok = false;
        for _ in 0..50 {
            std::thread::sleep(Duration::from_millis(100));
            if get("/x").starts_with("HTTP/1.1 200 ") {
                ok = true;
                break;
            }
        }
        assert!(ok, "the slots are free again");
    }
}
