//! The bus side of the registry (ADR-0029 §4, §5): the subscriptions to
//! `Pub/MAM` and `Pub/Health` of every application, the [`Directory`],
//! the listing of the available assets in the Registry services (MG A1,
//! REQ-REG-002, through the SDK's listed assets) and the `Get`s for what is
//! missing.
//!
//! Read-only (ADR-0008): towards other participants the registry only
//! subscribes and sends `Get`; it never sends `Set`, `Del` or `Call`
//! (asserted by the test `read_only`).

use std::collections::{BTreeMap, VecDeque};
use std::sync::mpsc::{self, Receiver, RecvTimeoutError, Sender};
use std::sync::{Arc, Mutex, MutexGuard, Weak};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use oec::app::AppHandle;
use oec::consumer::{Consumer, GetOptions, Payload, Received, Resource, Subscription, Target};
use oec::identity::IdentificationLink;

use crate::directory::{Directory, Effect, Fetch, Origin, Want};

/// The resources the registry collects from every application and offers
/// in the Registry services (MG A1).
pub(crate) const COLLECTED: [Resource; 2] = [Resource::Mam, Resource::Health];

/// The time a `Get` waits, so a Health that follows a Birth at once makes
/// it unnecessary.
const FETCH_DELAY: Duration = Duration::from_millis(500);
/// At most one `Get` per application and resource within this time.
const FETCH_INTERVAL: Duration = Duration::from_secs(10);
/// The time an answer may take.
const GET_TIMEOUT: Duration = Duration::from_secs(5);

fn lock<T>(m: &Mutex<T>) -> MutexGuard<'_, T> {
    m.lock().unwrap_or_else(std::sync::PoisonError::into_inner)
}

fn unix_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_or(0, |d| u64::try_from(d.as_millis()).unwrap_or(u64::MAX))
}

/// The bus side: see the [module](self).
pub(crate) struct Bus {
    app: AppHandle,
    consumer: Consumer,
    dir: Mutex<Directory>,
    jobs: Mutex<Sender<Vec<Effect>>>,
    subscriptions: Mutex<Vec<Subscription>>,
}

impl Bus {
    /// Starts the bus side of `app`: the subscriptions and the worker
    /// thread for the listing and the `Get`s.
    ///
    /// # Errors
    ///
    /// The SDK refuses a subscription, or the thread can not be started.
    pub(crate) fn start(app: AppHandle) -> Result<Arc<Self>, String> {
        let (jobs, queue) = mpsc::channel();
        let bus = Arc::new(Self {
            consumer: app.consumer(),
            dir: Mutex::new(Directory::new(app.identification_link().as_str())),
            app,
            jobs: Mutex::new(jobs),
            subscriptions: Mutex::new(Vec::new()),
        });
        for resource in COLLECTED {
            let me = Arc::downgrade(&bus);
            let sub = bus
                .consumer
                .subscribe(resource, &Target::any(), move |r| {
                    if let Some(bus) = Weak::upgrade(&me) {
                        bus.received(r);
                    }
                })
                .map_err(|e| e.to_string())?;
            lock(&bus.subscriptions).push(sub);
        }
        let worker = Arc::downgrade(&bus);
        std::thread::Builder::new()
            .name("oec-registry-bus".to_owned())
            .spawn(move || work(&worker, &queue))
            .map_err(|e| e.to_string())?;
        Ok(bus)
    }

    /// The directory (for the HTTP side of increment 3).
    #[allow(dead_code, reason = "read by the HTTP side of increment 3")]
    pub(crate) fn directory(&self) -> MutexGuard<'_, Directory> {
        lock(&self.dir)
    }

    /// A `Pub/MAM` or `Pub/Health` of any application.
    fn received(&self, r: &Received) {
        let app = r.app();
        let source = r.data_set_writer_name();
        let o = Origin {
            service_type: r.service_type(),
            app: app.as_str(),
            source: source.as_str(),
            reply: r.reply_to().is_some(),
            now: unix_ms(),
        };
        let effects = {
            let mut dir = lock(&self.dir);
            match r.payload() {
                Payload::Mam(m) => dir.mam(&o, m.clone()),
                Payload::Health(h) => dir.health(&o, h.clone()),
                _ => return,
            }
        };
        if !effects.is_empty() {
            let _ = lock(&self.jobs).send(effects);
        }
    }

    fn apply(&self, effect: Effect, due: &mut VecDeque<(Instant, Fetch)>) {
        match effect {
            Effect::List { il, mam, health } => {
                let listed = IdentificationLink::parse(&il)
                    .and_then(|link| self.app.list_asset(&link, *mam, health));
                if let Err(e) = listed {
                    log::warn!("registry: {il} not listed: {e}");
                }
                self.summary();
            }
            Effect::Unlist(il) => {
                if let Ok(link) = IdentificationLink::parse(&il) {
                    self.app.unlist_asset(&link);
                }
                self.summary();
            }
            Effect::Fetch(f) => {
                if !due.iter().any(|(_, d)| d == &f) {
                    due.push_back((Instant::now() + FETCH_DELAY, f));
                }
            }
        }
    }

    /// Logs the size of the directory after a change of the listing.
    fn summary(&self) {
        let dir = lock(&self.dir);
        let (known, available) = dir
            .entries()
            .fold((0, 0), |(k, a), e| (k + 1, a + usize::from(e.available())));
        log::info!(
            "registry: {available} of {known} assets available, {} messages ignored",
            dir.ignored()
        );
    }

    /// Sends the `Get` of `f` if it is still needed and feeds the answers
    /// into the directory.
    fn fetch(&self, f: &Fetch) {
        if !lock(&self.dir).missing(f) {
            return;
        }
        let Ok(app) = IdentificationLink::parse(&f.app) else {
            return;
        };
        let resource = match f.want {
            Want::Mam => Resource::Mam,
            Want::Health => Resource::Health,
        };
        let options = GetOptions::new().timeout(GET_TIMEOUT);
        match self
            .consumer
            .get_blocking(resource, &Target::app(f.service_type, app), &options)
        {
            Ok(answers) => {
                for r in &answers {
                    self.received(r);
                }
            }
            Err(e) => log::debug!("registry: Get {resource:?} of {}: {e}", f.app),
        }
    }
}

/// The worker: applies the effects in order and sends the `Get`s, delayed
/// by [`FETCH_DELAY`] and at most once per [`FETCH_INTERVAL`].
fn work(bus: &Weak<Bus>, queue: &Receiver<Vec<Effect>>) {
    let mut due: VecDeque<(Instant, Fetch)> = VecDeque::new();
    let mut sent: BTreeMap<Fetch, Instant> = BTreeMap::new();
    loop {
        let wait = due.front().map_or(Duration::from_secs(1), |(at, _)| {
            at.saturating_duration_since(Instant::now())
        });
        let received = queue.recv_timeout(wait);
        let Some(bus) = bus.upgrade() else {
            return;
        };
        match received {
            Ok(effects) => {
                for effect in effects {
                    bus.apply(effect, &mut due);
                }
            }
            Err(RecvTimeoutError::Timeout) => {}
            Err(RecvTimeoutError::Disconnected) => return,
        }
        let now = Instant::now();
        while due.front().is_some_and(|(at, _)| *at <= now) {
            let (_, f) = due.pop_front().expect("checked");
            if sent
                .get(&f)
                .is_some_and(|at| now.duration_since(*at) < FETCH_INTERVAL)
            {
                continue;
            }
            sent.insert(f.clone(), now);
            bus.fetch(&f);
        }
        sent.retain(|_, at| now.duration_since(*at) < FETCH_INTERVAL);
    }
}

#[cfg(test)]
mod tests {
    /// ADR-0008, ADR-0029 §4: the registry never changes another
    /// participant. Of the SDK consumer it may use `subscribe`,
    /// `on_presence`, `apps` and `get` only: no `set`, `del` or
    /// `new_data_set_writer_id` (a `Call`) in its sources, whitespace
    /// ignored, test modules excluded. The sources use no other method of
    /// these names either.
    #[test]
    fn read_only() {
        let dir = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("src");
        let mut seen = 0;
        for entry in std::fs::read_dir(&dir).unwrap() {
            let path = entry.unwrap().path();
            if path.extension().and_then(|e| e.to_str()) != Some("rs") {
                continue;
            }
            seen += 1;
            let text = std::fs::read_to_string(&path).unwrap();
            let code = text.split("#[cfg(test)]").next().unwrap_or_default();
            let code: String = code.chars().filter(|c| !c.is_whitespace()).collect();
            for method in [
                ".set(",
                ".set_blocking(",
                ".del(",
                ".del_blocking(",
                ".new_data_set_writer_id",
            ] {
                assert!(!code.contains(method), "{}: {method}", path.display());
            }
        }
        assert!(seen >= 3, "the sources were not found");
    }
}
