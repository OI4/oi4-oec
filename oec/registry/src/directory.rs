//! The directory of the registry (ADR-0029 §3): one entry per
//! Identification Link, fed by the `Pub/MAM` and `Pub/Health` messages on
//! the bus. Sans-IO: every message returns the effects for the bus side –
//! list or unlist an available asset in the Registry services (MG A1,
//! REQ-REG-002) and ask an application with `Get` for what is missing
//! (ADR-0029 §4).
//!
//! Rules (MG 6.3, MG A1, REQ-LIFE-011 … 013):
//! 1. A `Pub/MAM` of an application about itself without `ReplyTo` is its
//!    Birth: the application is online. While it is online, such a MAM is
//!    a change of its MAM (REQ-RES-MAM-018).
//! 2. An application not yet known is online with its first `MAM` or
//!    `Health` about itself.
//! 3. A `Health` of an application about itself with `NORMAL_0`/0 is its
//!    Close (offline graceful), with `FAILURE_1`/0 its Will or a broken
//!    state (offline ungraceful); online again only after its next Birth.
//! 4. A device is online while its publisher is online and its MAM was
//!    received since the publisher came online; it goes offline with its
//!    publisher.
//! 5. Available = online with MAM and Health known; only available assets
//!    are listed.
//!
//! Ignored (counted): the registry's own messages, the republications of
//! other registries (a `Source` ≠ `AppId` under ServiceType Registry is a
//! listed asset there, not a represented one), a MAM whose
//! ProductInstanceUri is not its `Source`, and a device message about a
//! known application.

use std::collections::{BTreeMap, BTreeSet};

use oec::ServiceType;
use oec::consumer::{Health, HealthState};
use oec::identity::Mam;

/// What an entry is.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum Kind {
    /// `Source` = `AppId`: an application with its own MQTT client.
    Application,
    /// `Source` ≠ `AppId`: an entity represented by its publisher.
    Device,
}

/// Online state of an entry.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum Availability {
    /// After its Birth or first sighting (device: see rule 4).
    Online,
    /// After its Close; a device whose publisher is online but whose MAM
    /// was not received since.
    OfflineGraceful,
    /// After its Will or `FAILURE_1`/0 about itself.
    OfflineUngraceful,
}

/// One asset (ADR-0029 §3).
#[derive(Debug, Clone)]
pub(crate) struct Entry {
    /// Identification Link (`Source`, `DataSetWriterName`).
    pub(crate) il: String,
    pub(crate) kind: Kind,
    /// ServiceType of the publisher.
    pub(crate) service_type: ServiceType,
    /// AppId of the publisher (the entry itself for an application).
    pub(crate) publisher: String,
    pub(crate) mam: Option<Mam>,
    pub(crate) health: Option<Health>,
    pub(crate) availability: Availability,
    /// Times of the registry clock (Unix ms).
    #[allow(dead_code, reason = "read by the HTTP side of increment 3")]
    pub(crate) first_seen: u64,
    pub(crate) last_seen: u64,
    pub(crate) online_since: Option<u64>,
    /// Time of the last MAM (rule 4).
    mam_at: Option<u64>,
}

impl Entry {
    fn new(il: &str, kind: Kind, o: &Origin<'_>) -> Self {
        Self {
            il: il.to_owned(),
            kind,
            service_type: o.service_type,
            publisher: o.app.to_owned(),
            mam: None,
            health: None,
            availability: Availability::OfflineGraceful,
            first_seen: o.now,
            last_seen: o.now,
            online_since: None,
            mam_at: None,
        }
    }

    /// Available (MG A1): online with MAM and Health known.
    pub(crate) fn available(&self) -> bool {
        self.availability == Availability::Online && self.mam.is_some() && self.health.is_some()
    }

    fn online(&mut self, now: u64) {
        if self.availability != Availability::Online {
            self.availability = Availability::Online;
            self.online_since = Some(now);
        }
    }
}

/// Where a message came from.
#[derive(Debug, Clone, Copy)]
pub(crate) struct Origin<'a> {
    pub(crate) service_type: ServiceType,
    /// AppId of the topic.
    pub(crate) app: &'a str,
    /// `Source` of the message (its DataSetWriterName).
    pub(crate) source: &'a str,
    /// The message answers a `Get` (`ReplyTo` set).
    pub(crate) reply: bool,
    /// Registry clock, Unix ms.
    pub(crate) now: u64,
}

/// What the registry asks an application for (without Source: the
/// answer covers the application and its devices).
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub(crate) enum Want {
    Mam,
    Health,
}

/// A `Get` to an application.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub(crate) struct Fetch {
    pub(crate) service_type: ServiceType,
    pub(crate) app: String,
    pub(crate) want: Want,
}

/// An effect for the bus side.
#[derive(Debug, Clone, PartialEq)]
pub(crate) enum Effect {
    /// List or replace an available asset (MG A1).
    List {
        il: String,
        mam: Box<Mam>,
        health: Health,
    },
    /// Remove an asset that is no longer available.
    Unlist(String),
    /// Ask an application for what is missing.
    Fetch(Fetch),
}

/// The directory: see the [module](self).
#[derive(Debug)]
pub(crate) struct Directory {
    me: String,
    entries: BTreeMap<String, Entry>,
    listed: BTreeSet<String>,
    ignored: u64,
}

fn fetch(o: &Origin<'_>, want: Want) -> Effect {
    Effect::Fetch(Fetch {
        service_type: o.service_type,
        app: o.app.to_owned(),
        want,
    })
}

fn is_close(h: &Health) -> bool {
    h.health == HealthState::Normal && h.health_score == Some(0)
}

fn is_will(h: &Health) -> bool {
    h.health == HealthState::Failure && h.health_score == Some(0)
}

impl Directory {
    /// The directory of the registry with the AppId `me`.
    pub(crate) fn new(me: &str) -> Self {
        Self {
            me: me.to_owned(),
            entries: BTreeMap::new(),
            listed: BTreeSet::new(),
            ignored: 0,
        }
    }

    /// The entries, sorted by Identification Link.
    pub(crate) fn entries(&self) -> impl Iterator<Item = &Entry> {
        self.entries.values()
    }

    /// One entry.
    #[allow(dead_code, reason = "read by the HTTP side of increment 3")]
    pub(crate) fn get(&self, il: &str) -> Option<&Entry> {
        self.entries.get(il)
    }

    /// The number of ignored messages.
    pub(crate) fn ignored(&self) -> u64 {
        self.ignored
    }

    fn ignore(&mut self, o: &Origin<'_>) -> bool {
        let skip = o.app == self.me
            || (o.service_type == ServiceType::Registry && o.source != o.app)
            || (o.source != o.app
                && self
                    .entries
                    .get(o.source)
                    .is_some_and(|e| e.kind == Kind::Application));
        if skip {
            self.ignored += 1;
        }
        skip
    }

    /// A `Pub/MAM` message.
    pub(crate) fn mam(&mut self, o: &Origin<'_>, mam: Mam) -> Vec<Effect> {
        if self.ignore(o) {
            return Vec::new();
        }
        if mam.product_instance_uri != o.source {
            self.ignored += 1;
            return Vec::new();
        }
        let mut out = Vec::new();
        if o.source == o.app {
            let known = self.entries.contains_key(o.app);
            let e = self
                .entries
                .entry(o.app.to_owned())
                .or_insert_with(|| Entry::new(o.app, Kind::Application, o));
            e.last_seen = o.now;
            e.service_type = o.service_type;
            let was_online = known && e.availability == Availability::Online;
            if !known || (!was_online && !o.reply) {
                // Birth or first sighting; a Health from before is stale
                if known {
                    e.health = None;
                }
                e.online(o.now);
            }
            e.mam = Some(mam);
            e.mam_at = Some(o.now);
            if e.availability == Availability::Online && e.health.is_none() {
                out.push(fetch(o, Want::Health));
            }
        } else {
            let e = self.device(o);
            e.mam = Some(mam);
            e.mam_at = Some(o.now);
            let health = e.health.is_none();
            match self.entries.get(o.app).map(|p| p.availability) {
                None => out.push(fetch(o, Want::Mam)),
                Some(Availability::Online) if health => out.push(fetch(o, Want::Health)),
                _ => {}
            }
        }
        out.extend(self.sync(o.app));
        out
    }

    /// A `Pub/Health` message.
    pub(crate) fn health(&mut self, o: &Origin<'_>, health: Health) -> Vec<Effect> {
        if self.ignore(o) {
            return Vec::new();
        }
        let mut out = Vec::new();
        if o.source == o.app {
            let known = self.entries.contains_key(o.app);
            let e = self
                .entries
                .entry(o.app.to_owned())
                .or_insert_with(|| Entry::new(o.app, Kind::Application, o));
            e.last_seen = o.now;
            e.service_type = o.service_type;
            if is_close(&health) {
                e.availability = Availability::OfflineGraceful;
                e.online_since = None;
            } else if is_will(&health) {
                e.availability = Availability::OfflineUngraceful;
                e.online_since = None;
            } else if !known {
                e.online(o.now);
            }
            e.health = Some(health);
            if e.availability == Availability::Online && e.mam.is_none() {
                out.push(fetch(o, Want::Mam));
            }
        } else {
            let e = self.device(o);
            e.health = Some(health);
            let mam = e.mam.is_none();
            match self.entries.get(o.app).map(|p| p.availability) {
                None => out.push(fetch(o, Want::Mam)),
                Some(Availability::Online) if mam => out.push(fetch(o, Want::Mam)),
                _ => {}
            }
        }
        out.extend(self.sync(o.app));
        out
    }

    /// Whether `f` is still needed: the application is unknown (only for
    /// its MAM) or online and lacks the resource for itself or one of its
    /// devices.
    pub(crate) fn missing(&self, f: &Fetch) -> bool {
        let Some(app) = self.entries.get(&f.app) else {
            return f.want == Want::Mam;
        };
        if app.availability != Availability::Online {
            return false;
        }
        let lacks = |e: &Entry| match f.want {
            Want::Mam => e.mam.is_none(),
            Want::Health => e.health.is_none(),
        };
        lacks(app)
            || self
                .entries
                .values()
                .any(|e| e.kind == Kind::Device && e.publisher == f.app && lacks(e))
    }

    fn device(&mut self, o: &Origin<'_>) -> &mut Entry {
        let e = self
            .entries
            .entry(o.source.to_owned())
            .or_insert_with(|| Entry::new(o.source, Kind::Device, o));
        e.last_seen = o.now;
        o.app.clone_into(&mut e.publisher);
        e.service_type = o.service_type;
        e
    }

    /// Applies rule 4 to the devices of `app` and lists or unlists the
    /// application and its devices (rule 5).
    fn sync(&mut self, app: &str) -> Vec<Effect> {
        let (state, since) = match self.entries.get(app) {
            Some(a) if a.kind == Kind::Application => (a.availability, a.online_since),
            _ => (Availability::OfflineGraceful, None),
        };
        let mut out = Vec::new();
        for e in self.entries.values_mut() {
            if e.kind == Kind::Device && e.publisher == app {
                let fresh = matches!((since, e.mam_at), (Some(s), Some(m)) if m >= s);
                match state {
                    Availability::Online if fresh => {
                        let at = e.mam_at.unwrap_or_default();
                        e.online(at);
                    }
                    Availability::Online => e.availability = Availability::OfflineGraceful,
                    offline => e.availability = offline,
                }
                if e.availability != Availability::Online {
                    e.online_since = None;
                }
            } else if e.il != app {
                continue;
            }
            if e.available() {
                self.listed.insert(e.il.clone());
                out.push(Effect::List {
                    il: e.il.clone(),
                    mam: Box::new(e.mam.clone().expect("available")),
                    health: e.health.clone().expect("available"),
                });
            } else if self.listed.remove(&e.il) {
                out.push(Effect::Unlist(e.il.clone()));
            }
        }
        out
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    const ME: &str = "oi4.org/OEC,20Registry/OEC-REGISTRY/reg-1";
    const APP: &str = "vendor.com/Gateway/GW-1/SN-1";
    const PUMP: &str = "vendor.com/Pump/P-1/0815";
    const OTHER: &str = "vendor.com/Other/O-1/1";

    fn mam(piu: &str) -> Mam {
        serde_json::from_value(serde_json::json!({
            "Manufacturer": {"Locale": "en-US", "Text": "Vendor"},
            "ManufacturerUri": "vendor.com",
            "Model": {"Locale": "en-US", "Text": "Gateway"},
            "ProductCode": "GW-1",
            "HardwareRevision": "",
            "SoftwareRevision": "1.0.0",
            "DeviceRevision": "",
            "DeviceManual": "",
            "DeviceClass": "Oi4.Aggregation",
            "SerialNumber": "SN-1",
            "ProductInstanceUri": piu,
            "RevisionCounter": 0
        }))
        .unwrap()
    }

    fn health(state: HealthState, score: u8) -> Health {
        Health {
            health: state,
            health_score: Some(score),
            details: None,
        }
    }

    fn normal() -> Health {
        health(HealthState::Normal, 100)
    }

    fn at<'a>(app: &'a str, source: &'a str, reply: bool, now: u64) -> Origin<'a> {
        Origin {
            service_type: ServiceType::Aggregation,
            app,
            source,
            reply,
            now,
        }
    }

    fn listed(effects: &[Effect]) -> Vec<&str> {
        effects
            .iter()
            .filter_map(|e| match e {
                Effect::List { il, .. } => Some(il.as_str()),
                _ => None,
            })
            .collect()
    }

    fn unlisted(effects: &[Effect]) -> Vec<&str> {
        effects
            .iter()
            .filter_map(|e| match e {
                Effect::Unlist(il) => Some(il.as_str()),
                _ => None,
            })
            .collect()
    }

    fn fetches(effects: &[Effect]) -> Vec<Want> {
        effects
            .iter()
            .filter_map(|e| match e {
                Effect::Fetch(f) => Some(f.want),
                _ => None,
            })
            .collect()
    }

    fn state(d: &Directory, il: &str) -> Availability {
        d.get(il).unwrap().availability
    }

    /// Rules 1, 3 and 5: Birth, then Health → available; Close → offline
    /// graceful and unlisted; a heartbeat does not bring it back; the next
    /// Birth does.
    #[test]
    fn birth_close_and_rebirth() {
        let mut d = Directory::new(ME);
        let fx = d.mam(&at(APP, APP, false, 1), mam(APP));
        assert_eq!(state(&d, APP), Availability::Online);
        assert_eq!(fetches(&fx), [Want::Health], "Health is missing");
        assert!(listed(&fx).is_empty(), "not available without Health");
        let fx = d.health(&at(APP, APP, false, 2), normal());
        assert_eq!(listed(&fx), [APP]);
        assert!(fetches(&fx).is_empty());

        let fx = d.health(&at(APP, APP, false, 3), health(HealthState::Normal, 0));
        assert_eq!(state(&d, APP), Availability::OfflineGraceful);
        assert_eq!(unlisted(&fx), [APP]);
        // answers and heartbeats do not bring it back (REQ-LIFE-013)
        let fx = d.health(&at(APP, APP, false, 4), normal());
        assert_eq!(state(&d, APP), Availability::OfflineGraceful);
        assert!(listed(&fx).is_empty());
        let fx = d.mam(&at(APP, APP, true, 5), mam(APP));
        assert_eq!(state(&d, APP), Availability::OfflineGraceful);
        assert!(listed(&fx).is_empty());

        // the next Birth: online, the old Health is stale
        let fx = d.mam(&at(APP, APP, false, 6), mam(APP));
        assert_eq!(state(&d, APP), Availability::Online);
        assert!(d.get(APP).unwrap().health.is_none());
        assert_eq!(fetches(&fx), [Want::Health]);
        let fx = d.health(&at(APP, APP, false, 7), normal());
        assert_eq!(listed(&fx), [APP]);
        let e = d.get(APP).unwrap();
        assert_eq!((e.first_seen, e.last_seen, e.online_since), (1, 7, Some(6)));
    }

    /// Rule 3: the Will (FAILURE_1/0 about itself).
    #[test]
    fn will() {
        let mut d = Directory::new(ME);
        d.mam(&at(APP, APP, false, 1), mam(APP));
        d.health(&at(APP, APP, false, 2), normal());
        let fx = d.health(&at(APP, APP, false, 3), health(HealthState::Failure, 0));
        assert_eq!(state(&d, APP), Availability::OfflineUngraceful);
        assert_eq!(unlisted(&fx), [APP]);
        // FAILURE_1 with another score is a Health, not a Will
        d.mam(&at(APP, APP, false, 4), mam(APP));
        let fx = d.health(&at(APP, APP, false, 5), health(HealthState::Failure, 10));
        assert_eq!(state(&d, APP), Availability::Online);
        assert_eq!(listed(&fx), [APP]);
    }

    /// Rule 2: an application first seen by its heartbeat is online; the
    /// registry asks for its MAM, the answer (with ReplyTo) makes it
    /// available.
    #[test]
    fn first_sighting_by_heartbeat() {
        let mut d = Directory::new(ME);
        let fx = d.health(&at(APP, APP, false, 1), normal());
        assert_eq!(state(&d, APP), Availability::Online);
        assert_eq!(
            fx,
            [Effect::Fetch(Fetch {
                service_type: ServiceType::Aggregation,
                app: APP.to_owned(),
                want: Want::Mam
            })]
        );
        assert!(d.missing(&Fetch {
            service_type: ServiceType::Aggregation,
            app: APP.to_owned(),
            want: Want::Mam
        }));
        let fx = d.mam(&at(APP, APP, true, 2), mam(APP));
        assert_eq!(listed(&fx), [APP]);
        assert!(!d.missing(&Fetch {
            service_type: ServiceType::Aggregation,
            app: APP.to_owned(),
            want: Want::Mam
        }));
        // a first sighting by an answer works as well
        let mut d = Directory::new(ME);
        d.mam(&at(APP, APP, true, 1), mam(APP));
        assert_eq!(state(&d, APP), Availability::Online);
    }

    /// Rule 4: devices follow their publisher; after a Birth of the
    /// publisher a device is online only with a MAM received since.
    #[test]
    fn devices_follow_their_publisher() {
        let mut d = Directory::new(ME);
        d.mam(&at(APP, APP, false, 1), mam(APP));
        d.health(&at(APP, APP, false, 1), normal());
        let fx = d.mam(&at(APP, PUMP, false, 2), mam(PUMP));
        assert_eq!(d.get(PUMP).unwrap().kind, Kind::Device);
        assert_eq!(state(&d, PUMP), Availability::Online);
        assert_eq!(fetches(&fx), [Want::Health]);
        let fx = d.health(&at(APP, PUMP, false, 3), normal());
        assert_eq!(listed(&fx), [APP, PUMP]);

        // the publisher leaves with its Will: the device goes with it
        let fx = d.health(&at(APP, APP, false, 4), health(HealthState::Failure, 0));
        assert_eq!(unlisted(&fx), [APP, PUMP]);
        assert_eq!(state(&d, PUMP), Availability::OfflineUngraceful);

        // Birth of the publisher: the device waits for its own MAM
        d.mam(&at(APP, APP, false, 5), mam(APP));
        let fx = d.health(&at(APP, APP, false, 6), normal());
        assert_eq!(listed(&fx), [APP]);
        assert_eq!(state(&d, PUMP), Availability::OfflineGraceful);
        let fx = d.mam(&at(APP, PUMP, false, 7), mam(PUMP));
        assert_eq!(listed(&fx), [APP, PUMP]);
        assert_eq!(d.get(PUMP).unwrap().online_since, Some(7));
    }

    /// A device of a publisher not yet known: the registry asks the
    /// publisher for its MAM (and gets the device's again).
    #[test]
    fn device_of_an_unknown_publisher() {
        let mut d = Directory::new(ME);
        let fx = d.health(&at(APP, PUMP, false, 1), normal());
        assert_eq!(fetches(&fx), [Want::Mam]);
        assert_eq!(state(&d, PUMP), Availability::OfflineGraceful);
        // the answer to Get MAM without Source: the publisher, then the device
        let fx = d.mam(&at(APP, APP, true, 2), mam(APP));
        assert_eq!(fetches(&fx), [Want::Health]);
        let fx = d.mam(&at(APP, PUMP, true, 2), mam(PUMP));
        // available on its own: MAM and Health known, its publisher online
        assert_eq!(listed(&fx), [PUMP]);
        let fx = d.health(&at(APP, APP, true, 3), normal());
        assert_eq!(listed(&fx), [APP, PUMP]);
        assert!(!d.missing(&Fetch {
            service_type: ServiceType::Aggregation,
            app: APP.to_owned(),
            want: Want::Health
        }));
    }

    /// A MAM change of an online application (REQ-RES-MAM-018) is no new
    /// Birth: its devices stay online.
    #[test]
    fn mam_change_is_no_birth() {
        let mut d = Directory::new(ME);
        d.mam(&at(APP, APP, false, 1), mam(APP));
        d.health(&at(APP, APP, false, 1), normal());
        d.mam(&at(APP, PUMP, false, 2), mam(PUMP));
        d.health(&at(APP, PUMP, false, 2), normal());
        let mut changed = mam(APP);
        changed.revision_counter = 1;
        let fx = d.mam(&at(APP, APP, false, 3), changed.clone());
        assert_eq!(listed(&fx), [APP, PUMP]);
        assert_eq!(d.get(APP).unwrap().online_since, Some(1));
        assert_eq!(d.get(APP).unwrap().mam, Some(changed));
        assert!(d.get(APP).unwrap().health.is_some());
    }

    #[test]
    fn ignored_messages() {
        let mut d = Directory::new(ME);
        // own messages
        assert!(d.mam(&at(ME, ME, false, 1), mam(ME)).is_empty());
        // the republication of another registry
        let other = "oi4.org/OEC,20Registry/OEC-REGISTRY/reg-2";
        let o = Origin {
            service_type: ServiceType::Registry,
            ..at(other, PUMP, true, 1)
        };
        assert!(d.mam(&o, mam(PUMP)).is_empty());
        // ProductInstanceUri ≠ Source
        assert!(d.mam(&at(APP, APP, false, 1), mam(OTHER)).is_empty());
        assert_eq!(d.ignored(), 3);
        assert_eq!(d.entries().count(), 0);
        // a device message about a known application
        d.mam(&at(OTHER, OTHER, false, 2), mam(OTHER));
        assert!(d.health(&at(APP, OTHER, false, 3), normal()).is_empty());
        assert_eq!(d.get(OTHER).unwrap().kind, Kind::Application);
        assert_eq!(d.ignored(), 4);
        // but another registry itself is an application
        let o = Origin {
            service_type: ServiceType::Registry,
            ..at(other, other, false, 4)
        };
        d.mam(&o, mam(other));
        assert_eq!(d.get(other).unwrap().service_type, ServiceType::Registry);
    }

    /// A Close of an application never seen online is recorded offline.
    #[test]
    fn close_of_an_unknown_application() {
        let mut d = Directory::new(ME);
        let fx = d.health(&at(APP, APP, false, 1), health(HealthState::Normal, 0));
        assert!(fx.is_empty(), "{fx:?}");
        assert_eq!(state(&d, APP), Availability::OfflineGraceful);
        assert!(!d.missing(&Fetch {
            service_type: ServiceType::Aggregation,
            app: APP.to_owned(),
            want: Want::Mam
        }));
    }
}
