//! The directory of the applications seen on the bus: Birth, Close and
//! Will (MG 6.3, REQ-LIFE-*), refreshed by the Health heartbeat
//! (REQ-TOP-017, SF-106) and by MAM answers.

use std::collections::BTreeMap;

use super::received::{Payload, Received};
use crate::il::IdentificationLink;
use crate::message::MessageId;
use crate::resource::{Health, HealthState, Mam};
use crate::topic::{ServiceType, Topic};

/// Presence of an application.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum AppState {
    /// Seen online: Birth, a MAM answer or a Health heartbeat.
    Online,
    /// Went offline with its Close message (REQ-LIFE-005).
    OfflineGraceful,
    /// Went offline with its Will (REQ-LIFE-007).
    OfflineUngraceful,
}

/// What the consumer knows about an application.
#[derive(Debug, Clone, PartialEq)]
pub struct AppInfo {
    /// ServiceType of the application.
    pub service_type: ServiceType,
    /// AppId of the application.
    pub app: IdentificationLink,
    /// Its presence.
    pub state: AppState,
    /// Its MAM about itself, once seen.
    pub mam: Option<Mam>,
    /// Its last Health about itself.
    pub health: Option<Health>,
}

/// A change of presence.
#[derive(Debug, Clone, PartialEq)]
#[non_exhaustive]
pub enum PresenceChange {
    /// The application is online; `mam` is its MAM, if already known. An
    /// application first seen by its heartbeat is reported again once its
    /// MAM arrives.
    Online {
        /// ServiceType of the application.
        service_type: ServiceType,
        /// AppId of the application.
        app: IdentificationLink,
        /// Its MAM.
        mam: Option<Box<Mam>>,
    },
    /// The application went offline with its Close message.
    OfflineGraceful {
        /// ServiceType of the application.
        service_type: ServiceType,
        /// AppId of the application.
        app: IdentificationLink,
    },
    /// The application went offline with its Will.
    OfflineUngraceful {
        /// ServiceType of the application.
        service_type: ServiceType,
        /// AppId of the application.
        app: IdentificationLink,
    },
}

#[derive(Debug, Default)]
pub(super) struct Directory {
    apps: BTreeMap<(ServiceType, String), AppInfo>,
}

/// What a Health of an application about itself means.
enum Signal {
    Close,
    Will,
    Heartbeat,
}

fn signal(h: &Health) -> Signal {
    match (h.health, h.health_score) {
        (HealthState::Normal, Some(0)) => Signal::Close,
        (HealthState::Failure, Some(0)) => Signal::Will,
        _ => Signal::Heartbeat,
    }
}

impl Directory {
    pub(super) fn apps(&self) -> Vec<AppInfo> {
        self.apps.values().cloned().collect()
    }

    /// Takes the items of a MAM or Health publication; returns the changes
    /// and, for an application first seen by its heartbeat, its address to
    /// ask for its MAM.
    pub(super) fn observe(
        &mut self,
        topic: &Topic,
        reply_to: Option<&MessageId>,
        items: &[Received],
    ) -> (
        Vec<PresenceChange>,
        Option<(ServiceType, IdentificationLink)>,
    ) {
        let st = topic.service_type();
        let app = topic.app_id();
        let mut changes = Vec::new();
        let mut discover = None;
        for item in items.iter().filter(|i| &i.data_set_writer_name == app) {
            let key = (st, app.encode());
            match &item.payload {
                Payload::Mam(mam) => {
                    if let Some(c) = self.mam(key, st, app, reply_to.is_none(), mam) {
                        changes.push(c);
                    }
                }
                Payload::Health(h) => {
                    let (c, d) = self.health(key, st, app, h);
                    changes.extend(c);
                    if d {
                        discover = Some((st, app.clone()));
                    }
                }
                _ => {}
            }
        }
        (changes, discover)
    }

    fn mam(
        &mut self,
        key: (ServiceType, String),
        st: ServiceType,
        app: &IdentificationLink,
        birth: bool,
        mam: &Mam,
    ) -> Option<PresenceChange> {
        let online = || PresenceChange::Online {
            service_type: st,
            app: app.clone(),
            mam: Some(Box::new(mam.clone())),
        };
        match self.apps.get_mut(&key) {
            None => {
                self.apps.insert(
                    key,
                    AppInfo {
                        service_type: st,
                        app: app.clone(),
                        state: AppState::Online,
                        mam: Some(mam.clone()),
                        health: None,
                    },
                );
                Some(online())
            }
            Some(info) => {
                let first_mam = info.mam.is_none();
                info.mam = Some(mam.clone());
                if birth {
                    // Only a Birth brings an offline application back
                    // (REQ-LIFE-013).
                    info.state = AppState::Online;
                    Some(online())
                } else if info.state == AppState::Online && first_mam {
                    Some(online())
                } else {
                    None
                }
            }
        }
    }

    fn health(
        &mut self,
        key: (ServiceType, String),
        st: ServiceType,
        app: &IdentificationLink,
        h: &Health,
    ) -> (Option<PresenceChange>, bool) {
        let state = match signal(h) {
            Signal::Close => AppState::OfflineGraceful,
            Signal::Will => AppState::OfflineUngraceful,
            Signal::Heartbeat => AppState::Online,
        };
        match self.apps.get_mut(&key) {
            None => {
                self.apps.insert(
                    key,
                    AppInfo {
                        service_type: st,
                        app: app.clone(),
                        state,
                        mam: None,
                        health: Some(h.clone()),
                    },
                );
                if state == AppState::Online {
                    let c = PresenceChange::Online {
                        service_type: st,
                        app: app.clone(),
                        mam: None,
                    };
                    (Some(c), true)
                } else {
                    (None, false)
                }
            }
            Some(info) => {
                if info.state != AppState::Online {
                    // Offline until the next Birth (REQ-LIFE-013).
                    return (None, false);
                }
                info.health = Some(h.clone());
                info.state = state;
                let c = match state {
                    AppState::Online => None,
                    AppState::OfflineGraceful => Some(PresenceChange::OfflineGraceful {
                        service_type: st,
                        app: app.clone(),
                    }),
                    AppState::OfflineUngraceful => Some(PresenceChange::OfflineUngraceful {
                        service_type: st,
                        app: app.clone(),
                    }),
                };
                (c, false)
            }
        }
    }
}
