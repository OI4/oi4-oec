//! The runtime of an [`App`] (ADR-0020 §2/§5, ADR-0026 §9/§10): drives the
//! sans-IO `oec_core::mqtt::Session` and `oec_core::lifecycle::Lifecycle`
//! over the TLS transport, with timers, reconnect/backoff and the graceful
//! close.
//!
//! One connection: TCP + TLS, CONNECT with a new Will (REQ-LIFE-008,
//! REQ-LIFE-016), after CONNACK the Birth first and then the Health
//! (REQ-LIFE-001, REQ-LIFE-015, REQ-TOP-017), the Health heartbeat and keep
//! alive while connected. A lost connection is closed without DISCONNECT,
//! so the broker publishes the Will, and the runtime reconnects with
//! backoff. Shutdown publishes the Close, then DISCONNECT (REQ-LIFE-005,
//! MG 6.3.4).

use std::future::pending;
use std::num::NonZeroU16;
use std::sync::Arc;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use oec_core::lifecycle::{Lifecycle, Publication};
use oec_core::message::DataSetWriterId;
use oec_core::mqtt::{CloseReason, Credentials, Event, QoS, Session, SessionConfig, SessionState};
use tokio::sync::watch;
use tokio::time::{Instant, sleep, sleep_until, timeout};

use crate::app::{App, AppHandle, Auth, ConnectionSettings, ConnectionState};
use crate::transport::{self, Either, Stream, race};
use crate::{Error, Result};

/// DataSetWriterId of the MAM of the application about itself (Birth,
/// REQ-LIFE-003); the first id of an application (REQ-MSG-020).
pub(crate) const MAM_WRITER: u16 = 10;
/// DataSetWriterId of the Health of the application about itself (Will,
/// Close and heartbeat, REQ-LIFE-007/009).
pub(crate) const HEALTH_WRITER: u16 = 11;
/// First reconnect delay; doubled per failed attempt up to [`BACKOFF_MAX`].
pub(crate) const BACKOFF_MIN: Duration = Duration::from_secs(1);
/// Largest reconnect delay.
pub(crate) const BACKOFF_MAX: Duration = Duration::from_secs(60);
/// Time for Close, DISCONNECT and the TLS close_notify on shutdown.
const CLOSE_TIMEOUT: Duration = Duration::from_secs(5);
/// Read buffer: one TLS record.
const READ_BUFFER: usize = 16 * 1024;

/// How a connection ended.
enum Outcome {
    /// [`AppHandle::shutdown`]: Close and DISCONNECT were sent.
    Shutdown,
    /// The connection was lost; the broker publishes the Will.
    Lost(CloseReason),
}

/// Runs `app` until [`AppHandle::shutdown`] (see [`App::run_async`]).
pub(crate) async fn run(app: App) -> Result<()> {
    let handle = app.handle();
    let mut shutdown = handle.subscribe();
    if *shutdown.borrow() {
        handle.set_state(ConnectionState::Closed);
        return Ok(());
    }
    let settings = app.connection().clone();
    let name = transport::server_name(settings.host())?;
    let mut tls = transport::client_config(&settings)?;
    let mut lifecycle = lifecycle(&app)?;
    let mut session = Session::new(session_config(&settings))
        .map_err(|e| Error::Invalid(format!("MQTT session: {e}")))?;
    let clock = Clock::new();
    let mut backoff = Backoff::new(settings.client_id());
    let mut connected_before = false;
    handle.set_state(ConnectionState::Connecting);
    loop {
        let attempt = race(
            shutdown_requested(&mut shutdown),
            transport::connect(
                &settings,
                name.clone(),
                Arc::clone(&tls),
                settings.keep_alive(),
            ),
        )
        .await;
        let outcome = match attempt {
            Either::Left(()) => break,
            Either::Right(Err(e)) => {
                log::warn!(
                    "connecting to {}:{} failed: {e}",
                    settings.host(),
                    settings.port()
                );
                None
            }
            Either::Right(Ok(stream)) => Some(
                drive(
                    stream,
                    &mut session,
                    &mut lifecycle,
                    &handle,
                    &mut shutdown,
                    &clock,
                    &mut backoff,
                    &mut connected_before,
                )
                .await?,
            ),
        };
        match outcome {
            Some(Outcome::Shutdown) => break,
            Some(Outcome::Lost(reason)) => {
                match reason {
                    CloseReason::Refused(code) => {
                        log::error!("the broker refused the connection: {code}");
                    }
                    other => log::warn!("connection lost: {other}"),
                }
                if connected_before {
                    handle.set_state(ConnectionState::Reconnecting);
                }
            }
            None => {}
        }
        let delay = backoff.next_delay();
        log::info!("reconnecting in {} ms", delay.as_millis());
        if let Either::Left(()) = race(shutdown_requested(&mut shutdown), sleep(delay)).await {
            break;
        }
        // Certificates may have been renewed: reload them, keep the last
        // good ones if that fails.
        match transport::client_config(&settings) {
            Ok(config) => tls = config,
            Err(e) => log::warn!("{e}; keeping the previous TLS settings"),
        }
    }
    handle.set_state(ConnectionState::Closed);
    Ok(())
}

/// One connection, from CONNECT to its end.
#[allow(clippy::too_many_arguments)]
async fn drive(
    mut stream: Stream,
    session: &mut Session,
    lifecycle: &mut Lifecycle,
    handle: &AppHandle,
    shutdown: &mut watch::Receiver<bool>,
    clock: &Clock,
    backoff: &mut Backoff,
    connected_before: &mut bool,
) -> Result<Outcome> {
    session
        .connect(lifecycle.will(unix_ms()), clock.now())
        .map_err(|e| Error::Invalid(format!("MQTT CONNECT: {e}")))?;
    let keep_alive = Duration::from_secs(u64::from(session.config().keep_alive.get()));
    let mut buf = vec![0; READ_BUFFER];
    loop {
        while let Some(event) = session.poll_event() {
            match event {
                Event::Connected { session_present } => {
                    // Clean Session is always set (REQ-MQTT-015) and the
                    // session refuses Session Present with it, so no
                    // resent message can precede the Birth (REQ-LIFE-015).
                    debug_assert!(!session_present);
                    backoff.reset();
                    *connected_before = true;
                    let now = clock.now();
                    for p in lifecycle.on_connected(now, unix_ms()) {
                        publish(session, p, now);
                    }
                    handle.set_state(ConnectionState::Online);
                }
                Event::Closed(reason) => return Ok(Outcome::Lost(reason)),
                Event::Message(p) => log::debug!("message on {} ignored", p.topic),
                other => log::debug!("MQTT: {other:?}"),
            }
        }
        if session.state() == SessionState::Connected {
            let now = clock.now();
            for p in lifecycle.poll(now, unix_ms()) {
                publish(session, p, now);
            }
        }
        if session.has_transmit() {
            let bytes = session.take_transmit();
            match timeout(keep_alive, transport::write_all(&mut stream, &bytes)).await {
                Ok(Ok(())) => {}
                Ok(Err(e)) => {
                    log::debug!("write: {e}");
                    session.connection_lost();
                    continue;
                }
                Err(_) => {
                    log::debug!("write: no progress within the keep alive interval");
                    session.connection_lost();
                    continue;
                }
            }
        }
        let deadline = match session.poll_timeout() {
            Some(t) if session.state() == SessionState::Connected => {
                Some(t.min(lifecycle.poll_timeout()))
            }
            t => t,
        };
        let timer = async {
            match deadline {
                Some(d) => sleep_until(clock.at(d)).await,
                None => pending().await,
            }
        };
        let wake = race(
            shutdown_requested(shutdown),
            race(timer, transport::read(&mut stream, &mut buf)),
        )
        .await;
        match wake {
            Either::Left(()) => {
                close(&mut stream, session, lifecycle, clock).await;
                return Ok(Outcome::Shutdown);
            }
            Either::Right(Either::Left(())) => session.handle_timeout(clock.now()),
            Either::Right(Either::Right(Ok(0))) => session.connection_lost(),
            Either::Right(Either::Right(Ok(n))) => session.handle_input(&buf[..n], clock.now()),
            Either::Right(Either::Right(Err(e))) => {
                log::debug!("read: {e}");
                session.connection_lost();
            }
        }
    }
}

/// Graceful close: the Close message if connected (REQ-LIFE-005 … 007),
/// then DISCONNECT, so the broker discards the Will (MG 6.3.4), then the
/// TLS close_notify.
async fn close(
    stream: &mut Stream,
    session: &mut Session,
    lifecycle: &mut Lifecycle,
    clock: &Clock,
) {
    let now = clock.now();
    if session.state() == SessionState::Connected {
        publish(session, lifecycle.close(unix_ms()), now);
    }
    if session.state() != SessionState::Disconnected {
        let _ = session.disconnect(now);
    }
    let bytes = session.take_transmit();
    let flushed = timeout(CLOSE_TIMEOUT, async {
        transport::write_all(stream, &bytes).await?;
        transport::shutdown(stream).await
    })
    .await;
    match flushed {
        Ok(Ok(())) => {}
        Ok(Err(e)) => log::warn!("close: {e}"),
        Err(_) => log::warn!("close: the broker did not take the Close within {CLOSE_TIMEOUT:?}"),
    }
    while session.poll_event().is_some() {}
}

/// Queues a lifecycle publication with QoS 0 (REQ-MQTT-017).
fn publish(session: &mut Session, p: Publication, now: Duration) {
    if let Err(e) = session.publish(p.topic.to_string(), p.payload, QoS::AtMostOnce, now) {
        log::error!("publishing on {} failed: {e}", p.topic);
    }
}

async fn shutdown_requested(rx: &mut watch::Receiver<bool>) {
    // An error means the sender is gone: nobody can ask any more, stop too.
    let _ = rx.wait_for(|requested| *requested).await;
}

/// The lifecycle of the application about itself (ADR-0026 §9.4/§9.5).
fn lifecycle(app: &App) -> Result<Lifecycle> {
    let writer = |id| {
        DataSetWriterId::new(id).map_err(|e| Error::Invalid(format!("DataSetWriterId {id}: {e}")))
    };
    Lifecycle::new(
        app.service_type(),
        app.identification_link().0.clone(),
        app.mam().clone(),
        writer(MAM_WRITER)?,
        writer(HEALTH_WRITER)?,
    )
    .map_err(|e| Error::Invalid(format!("lifecycle: {e}")))
}

/// [`ConnectionSettings`] → [`SessionConfig`]: ClientId (REQ-MQTT-013),
/// Keep Alive in seconds (REQ-MQTT-014; `build` checked 1 … 65535), Clean
/// Session true (REQ-MQTT-015), User Name and Password only without a
/// client certificate (REQ-MQTT-010), MaxPacketSize (RIG 4.1.1).
pub(crate) fn session_config(settings: &ConnectionSettings) -> SessionConfig {
    let mut config = SessionConfig::new(settings.client_id());
    if let Some(secs) = u16::try_from(settings.keep_alive().as_secs())
        .ok()
        .and_then(NonZeroU16::new)
    {
        config.keep_alive = secs;
    }
    config.clean_session = true;
    config.credentials = match settings.auth() {
        Auth::Credentials { username, password } => Some(Credentials {
            username: username.clone(),
            password: Some(password.expose().to_vec()),
        }),
        Auth::ClientCertificate { .. } => None,
    };
    config.max_packet_size = settings.max_packet_size();
    config
}

/// Monotonic time since the start of the runtime (the time base of
/// `Session` and `Lifecycle`).
struct Clock {
    origin: Instant,
}

impl Clock {
    fn new() -> Self {
        Self {
            origin: Instant::now(),
        }
    }

    fn now(&self) -> Duration {
        self.origin.elapsed()
    }

    fn at(&self, d: Duration) -> Instant {
        self.origin + d
    }
}

/// Wall clock in Unix milliseconds for the MessageIds (REQ-MSG-007).
fn unix_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_or(0, |d| u64::try_from(d.as_millis()).unwrap_or(u64::MAX))
}

/// Exponential reconnect backoff with jitter: 1 s, 2 s, 4 s … 60 s, each
/// randomized to 50 … 100 % so that many applications do not reconnect in
/// lockstep after a broker restart; reset by a CONNACK.
pub(crate) struct Backoff {
    next: Duration,
    rng: u64,
}

impl Backoff {
    pub(crate) fn new(seed: &str) -> Self {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map_or(0, |d| u64::from(d.subsec_nanos()));
        let rng = seed
            .bytes()
            .fold(0xcbf2_9ce4_8422_2325_u64 ^ nanos, |h, b| {
                (h ^ u64::from(b)).wrapping_mul(0x0000_0100_0000_01b3)
            });
        Self {
            next: BACKOFF_MIN,
            rng: rng | 1,
        }
    }

    pub(crate) fn reset(&mut self) {
        self.next = BACKOFF_MIN;
    }

    pub(crate) fn next_delay(&mut self) -> Duration {
        let base = self.next;
        self.next = (base * 2).min(BACKOFF_MAX);
        // xorshift64
        self.rng ^= self.rng << 13;
        self.rng ^= self.rng >> 7;
        self.rng ^= self.rng << 17;
        let half = base / 2;
        let jitter = u32::try_from(self.rng % 1001).unwrap_or(0);
        half + half * jitter / 1000
    }
}

/// Waits for SIGTERM/SIGINT (Unix) or Ctrl-C/Close/Shutdown (Windows) and
/// shuts the application down (ADR-0020 §5, ADR-0023 §1); used by
/// [`App::run`] only.
pub(crate) async fn signals(handle: AppHandle) {
    match wait_for_signal().await {
        Ok(name) => {
            log::info!("{name}: shutting down");
            handle.shutdown();
        }
        Err(e) => log::warn!("cannot listen for signals: {e}"),
    }
}

#[cfg(unix)]
async fn wait_for_signal() -> std::io::Result<&'static str> {
    use tokio::signal::unix::{SignalKind, signal};
    let mut term = signal(SignalKind::terminate())?;
    let mut int = signal(SignalKind::interrupt())?;
    Ok(match race(term.recv(), int.recv()).await {
        Either::Left(_) => "SIGTERM",
        Either::Right(_) => "SIGINT",
    })
}

#[cfg(windows)]
async fn wait_for_signal() -> std::io::Result<&'static str> {
    use tokio::signal::windows::{ctrl_c, ctrl_close, ctrl_shutdown};
    let mut c = ctrl_c()?;
    let mut close = ctrl_close()?;
    let mut shut = ctrl_shutdown()?;
    Ok(
        match race(c.recv(), race(close.recv(), shut.recv())).await {
            Either::Left(_) => "Ctrl-C",
            Either::Right(Either::Left(_)) => "Ctrl-Close",
            Either::Right(Either::Right(_)) => "Ctrl-Shutdown",
        },
    )
}

#[cfg(not(any(unix, windows)))]
async fn wait_for_signal() -> std::io::Result<&'static str> {
    tokio::signal::ctrl_c().await.map(|()| "Ctrl-C")
}

#[cfg(test)]
mod tests {
    use super::{BACKOFF_MAX, BACKOFF_MIN, Backoff};

    #[test]
    fn backoff_doubles_with_jitter_and_resets() {
        let mut b = Backoff::new("oec-test");
        let mut base = BACKOFF_MIN;
        for _ in 0..12 {
            let d = b.next_delay();
            assert!(d >= base / 2 && d <= base, "{d:?} for {base:?}");
            base = (base * 2).min(BACKOFF_MAX);
        }
        assert!(b.next_delay() >= BACKOFF_MAX / 2);
        b.reset();
        assert!(b.next_delay() <= BACKOFF_MIN);
    }
}
