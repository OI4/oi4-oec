//! The Syslog and Generic events (MG 8.3.6.2, MG 8.3.6.3; P4 increment 7c,
//! ADR-0026 §16): emitted by the application while connected, queued and
//! published at the next [`Provider::poll`] on
//! `Pub/Event/<Source>/<Category>/<Level>` (REQ-RES-EVENT-011), each with
//! its PublicationList entry (REQ-RES-PUBLIST-012). The Syslog levels are
//! switched on and off by `Set PublicationList` (REQ-TOP-027), Debug and
//! Informational are off by default (REQ-TOP-024); the setting is
//! persisted with the other overrides (REQ-RES-PUBLIST-016).

use std::collections::VecDeque;

use serde_json::{Map, Value as Json};
use time::OffsetDateTime;

use super::sets::date_time;
use super::{Provider, ProviderError, WriterKey};
use crate::il::IdentificationLink;
use crate::lifecycle::Publication;
use crate::message::{DataSetMessage, DataSetWriterId};
use crate::provider::data::Mode;
use crate::provider::overrides::PublicationOverride;
use crate::resource::{
    Event, EventCategory, Mam, PublicationConfig, PublicationEntry, PublicationMode,
};
use crate::status::StatusCode;
use crate::topic::{EventFilter, Filter, ListFilter, Resource, SyslogLevel};

/// At most this many events wait for the next [`Provider::poll`]; more are
/// dropped.
pub const MAX_QUEUED_EVENTS: usize = 256;

/// At most this many levels of Generic events per source, each with its
/// DataSetWriterId and PublicationList entry.
pub const MAX_GENERIC_LEVELS: usize = 16;

/// The syslog Facility of the events: 1, user-level messages (RFC 3164
/// 4.1.1).
const FACILITY: u32 = 1;

/// An event waiting for the next poll.
#[derive(Debug, Clone)]
pub(super) struct QueuedEvent {
    source: IdentificationLink,
    filter: EventFilter,
    event: Event,
    unix_ms: u64,
}

/// The events of the provider.
#[derive(Debug, Clone, Default)]
pub(super) struct Events {
    /// The most verbose Syslog level published by default; `None` if the
    /// application publishes no Syslog events.
    syslog: Option<SyslogLevel>,
    /// The levels of Generic events per source, in the order of their
    /// first event.
    generic: Vec<(IdentificationLink, String)>,
    queue: VecDeque<QueuedEvent>,
}

impl Events {
    pub(super) fn pending(&self) -> bool {
        !self.queue.is_empty()
    }

    /// Forgets the queued events (offline).
    pub(super) fn clear(&mut self) {
        self.queue.clear();
    }

    /// Forgets the levels and queued events of a removed asset.
    pub(super) fn remove(&mut self, il: &IdentificationLink) {
        self.generic.retain(|(s, _)| s != il);
        self.queue.retain(|e| &e.source != il);
    }
}

fn syslog_filter(level: SyslogLevel) -> String {
    Filter::Event(EventFilter::Syslog(level)).to_string()
}

fn event_key(source: &IdentificationLink, filter: &EventFilter) -> WriterKey {
    WriterKey {
        resource: Resource::Event,
        source: source.as_str().to_owned(),
        filter: Filter::Event(filter.clone()).to_string(),
    }
}

/// The HEADER of a syslog message (RFC 3164 4.1.2): the TIMESTAMP
/// `Mmm dd hh:mm:ss` (the day padded with a space) in UTC and the
/// HOSTNAME, e.g. `Dec 16 22:25:10 FDS-001-1200-0345`.
pub fn syslog_header(unix_ms: u64, hostname: &str) -> String {
    const MONTHS: [&str; 12] = [
        "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
    ];
    let secs = i64::try_from(unix_ms / 1000).unwrap_or(i64::MAX);
    let at = OffsetDateTime::from_unix_timestamp(secs).unwrap_or(OffsetDateTime::UNIX_EPOCH);
    format!(
        "{} {:>2} {:02}:{:02}:{:02} {hostname}",
        MONTHS[usize::from(u8::from(at.month())) - 1],
        at.day(),
        at.hour(),
        at.minute(),
        at.second()
    )
}

/// The HOSTNAME of the syslog messages of an application: its ProductCode
/// and SerialNumber joined with `-`, every character a HOSTNAME may not
/// contain (outside `!` … `~`, RFC 3164 4.1.2) replaced by `-`, as in the
/// example of MG B1.1.6 (`FDS-001-1200-0345`).
pub fn syslog_hostname(mam: &Mam) -> String {
    format!("{}-{}", mam.product_code, mam.serial_number)
        .chars()
        .map(|c| if ('!'..='~').contains(&c) { c } else { '-' })
        .collect()
}

/// The TAG of the syslog messages of an application (RFC 3164 4.1.3): the
/// alphanumeric characters of its Model, at most 32, `oec` if none, as in
/// the example of MG B1.1.6 (`FieldDataService`).
pub fn syslog_tag(mam: &Mam) -> String {
    let tag: String = mam
        .model
        .text
        .chars()
        .filter(char::is_ascii_alphanumeric)
        .take(32)
        .collect();
    if tag.is_empty() {
        "oec".to_owned()
    } else {
        tag
    }
}

impl Provider {
    /// The provider publishing the Syslog events of the application
    /// itself: the levels down to `threshold` are on by default, the
    /// others off (Debug and Informational must be off by default,
    /// REQ-TOP-024; the caller ensures that); from Level 2 each level is
    /// switched by `Set PublicationList` (Config `MODE_1`, REQ-TOP-027).
    ///
    /// # Errors
    ///
    /// [`ProviderError::WriterId`] if no DataSetWriterId can be assigned.
    pub fn with_syslog(mut self, threshold: SyslogLevel) -> Result<Self, ProviderError> {
        let app = self.app_id().clone();
        for &level in SyslogLevel::ALL {
            self.assign(event_key(&app, &EventFilter::Syslog(level)))?;
        }
        self.events.syslog = Some(threshold);
        Ok(self)
    }

    /// Whether Syslog events of `level` are published now: as set by
    /// `Set PublicationList`, else by the threshold of
    /// [`Self::with_syslog`].
    pub fn syslog_enabled(&self, level: SyslogLevel) -> bool {
        let Some(threshold) = self.events.syslog else {
            return false;
        };
        match self
            .overrides
            .get(self.app_id().as_str(), &syslog_filter(level))
        {
            Some(o) => o.mode == PublicationMode::Filter,
            None => level <= threshold,
        }
    }

    /// The most verbose Syslog level published now, `None` if none.
    pub fn syslog_max_level(&self) -> Option<SyslogLevel> {
        SyslogLevel::ALL
            .iter()
            .rev()
            .copied()
            .find(|l| self.syslog_enabled(*l))
    }

    /// The HOSTNAME and TAG of the Syslog events (see [`syslog_hostname`]
    /// and [`syslog_tag`]).
    pub fn syslog_identity(&self) -> (String, String) {
        let mam = self.lifecycle.mam();
        (syslog_hostname(mam), syslog_tag(mam))
    }

    /// Queues a Syslog event of the application (REQ-RES-EVENT-007/008):
    /// Number the PRI (Facility 1 × 8 + severity), no Description, Details
    /// `MSG` and `HEADER`. Returns `false` if it is dropped: offline, the
    /// level off, or [`MAX_QUEUED_EVENTS`] waiting.
    pub fn emit_syslog(
        &mut self,
        level: SyslogLevel,
        msg: &str,
        header: &str,
        unix_ms: u64,
    ) -> bool {
        if !self.syslog_enabled(level) {
            return false;
        }
        // the severity is the index in ALL (Emergency 0 … Debug 7)
        let severity = SyslogLevel::ALL
            .iter()
            .zip(0..)
            .find_map(|(l, s)| (*l == level).then_some(s))
            .unwrap_or(7);
        let mut details = Map::new();
        details.insert("MSG".to_owned(), Json::String(msg.to_owned()));
        details.insert("HEADER".to_owned(), Json::String(header.to_owned()));
        let event = Event {
            number: FACILITY * 8 + severity,
            category: EventCategory::Syslog,
            description: None,
            details: Some(details),
        };
        let source = self.app_id().clone();
        self.queue_event(source, EventFilter::Syslog(level), event, unix_ms)
    }

    /// Queues a Generic event of the application or of an asset
    /// (REQ-RES-EVENT-009) at the level `level`, a free value; a new level
    /// gets its DataSetWriterId and PublicationList entry. Returns
    /// `Ok(false)` if it is dropped: offline or [`MAX_QUEUED_EVENTS`]
    /// waiting.
    ///
    /// # Errors
    ///
    /// [`ProviderError::UnknownAsset`], [`ProviderError::Topic`] for a
    /// level that is no valid topic level, [`ProviderError::Event`] for
    /// more than [`MAX_GENERIC_LEVELS`] levels of the source,
    /// [`ProviderError::WriterId`].
    pub fn emit_generic(
        &mut self,
        source: &IdentificationLink,
        level: &str,
        event: GenericEvent,
        unix_ms: u64,
    ) -> Result<bool, ProviderError> {
        if !self.is_source(source) {
            return Err(ProviderError::UnknownAsset(source.to_string()));
        }
        let filter = EventFilter::Generic(level.to_owned());
        self.topic(
            Resource::Event,
            Some(source),
            Some(Filter::Event(filter.clone())),
        )
        .map_err(ProviderError::Topic)?;
        let known = self
            .events
            .generic
            .iter()
            .any(|(s, l)| s == source && l == level);
        if !known {
            let n = self
                .events
                .generic
                .iter()
                .filter(|(s, _)| s == source)
                .count();
            if n >= MAX_GENERIC_LEVELS {
                return Err(ProviderError::Event(format!(
                    "{source} has {MAX_GENERIC_LEVELS} levels of Generic events already"
                )));
            }
            self.assign(event_key(source, &filter))?;
            self.events.generic.push((source.clone(), level.to_owned()));
            self.pending.list = self.config.get;
        }
        let event = Event {
            number: event.number,
            category: EventCategory::Generic,
            description: event.description,
            details: event.details,
        };
        Ok(self.queue_event(source.clone(), filter, event, unix_ms))
    }

    fn queue_event(
        &mut self,
        source: IdentificationLink,
        filter: EventFilter,
        event: Event,
        unix_ms: u64,
    ) -> bool {
        if !self.connected || self.events.queue.len() >= MAX_QUEUED_EVENTS {
            return false;
        }
        self.events.queue.push_back(QueuedEvent {
            source,
            filter,
            event,
            unix_ms,
        });
        true
    }

    /// The queued events, each on its topic.
    pub(super) fn event_changes(&mut self) -> Vec<Publication> {
        let limit = self.config.publish_limit.min(self.config.max_packet_size);
        let mut out = Vec::new();
        while let Some(e) = self.events.queue.pop_front() {
            let filter = Filter::Event(e.filter.clone());
            let Ok(topic) = self.topic(Resource::Event, Some(&e.source), Some(filter.clone()))
            else {
                continue;
            };
            let Some(id) = self.writers.get(&event_key(&e.source, &e.filter)) else {
                continue;
            };
            let item = DataSetMessage::new(id, e.source, filter.to_string(), e.event)
                .with_timestamp(date_time(e.unix_ms));
            out.extend(
                self.series(vec![item], None, None, limit, e.unix_ms)
                    .into_iter()
                    .map(|payload| Publication {
                        topic: topic.clone(),
                        payload,
                    }),
            );
        }
        out
    }

    /// The PublicationList entries of the Syslog events (the application
    /// only; Config `MODE_1`, FILTER_4 or OFF_0) and the Generic events
    /// (FILTER_4) of `il`.
    pub(super) fn event_list_items(
        &self,
        il: &IdentificationLink,
        filter: Option<&ListFilter>,
        list: DataSetWriterId,
    ) -> Vec<DataSetMessage<PublicationEntry>> {
        let mut filters = Vec::new();
        if il == self.app_id() && self.events.syslog.is_some() {
            filters.extend(SyslogLevel::ALL.iter().map(|l| EventFilter::Syslog(*l)));
        }
        filters.extend(
            self.events
                .generic
                .iter()
                .filter(|(s, _)| s == il)
                .map(|(_, l)| EventFilter::Generic(l.clone())),
        );
        filters
            .into_iter()
            .filter_map(|ef| {
                let f = Filter::Event(ef.clone());
                if filter.is_some_and(|lf| {
                    lf.resource() != Resource::Event || lf.filter().is_some_and(|t| t != &f)
                }) {
                    return None;
                }
                let (mode, config) = match &ef {
                    EventFilter::Syslog(l) if self.syslog_enabled(*l) => {
                        (PublicationMode::Filter, Some(PublicationConfig::Mode))
                    }
                    EventFilter::Syslog(_) => (PublicationMode::Off, Some(PublicationConfig::Mode)),
                    _ => (PublicationMode::Filter, None),
                };
                let entry = PublicationEntry {
                    resource: Resource::Event,
                    source: il.as_str().to_owned(),
                    filter: Some(f.to_string()),
                    data_set_writer_id: self.writers.get(&event_key(il, &ef))?,
                    mode: Some(mode),
                    interval: None,
                    precisions: None,
                    config,
                };
                Some(DataSetMessage::new(
                    list,
                    il.clone(),
                    format!("Event/{f}"),
                    entry,
                ))
            })
            .collect()
    }

    /// `Set PublicationList` of a Syslog level of the application: only
    /// FILTER_4 and OFF_0 (REQ-RES-PUBLIST-012), persisted.
    pub(super) fn reconfigure_syslog(
        &mut self,
        source: &IdentificationLink,
        level: SyslogLevel,
        mode: Mode,
    ) -> Result<(), StatusCode> {
        if source != self.app_id() || self.events.syslog.is_none() {
            return Err(StatusCode::BAD_NOT_FOUND);
        }
        // an event is never answered on Get: no ON_REQUEST form
        let mode = match mode {
            Mode::FILTER => PublicationMode::Filter,
            Mode::OFF => PublicationMode::Off,
            _ => return Err(StatusCode::BAD_INVALID_ARGUMENT),
        };
        let entry = PublicationOverride {
            source: source.as_str().to_owned(),
            filter: syslog_filter(level),
            mode,
            interval: 0,
            precisions: std::collections::BTreeMap::new(),
        };
        self.unsaved.overrides |= self.overrides.set(entry);
        Ok(())
    }
}

/// A Generic event (MG 8.3.6.3): its Number and the optional Description
/// and Details, defined and documented by the application
/// (REQ-RES-EVENT-010).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct GenericEvent {
    /// The Number.
    pub number: u32,
    /// The Description.
    pub description: Option<String>,
    /// The Details.
    pub details: Option<Map<String, Json>>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn header_pads_the_day() {
        // the example of MG B1.1.6: 2021-12-16T22:25:10.094Z
        assert_eq!(
            syslog_header(1_639_693_510_094, "FDS-001-1200-0345"),
            "Dec 16 22:25:10 FDS-001-1200-0345"
        );
        // 2021-12-06T01:02:03Z
        assert_eq!(syslog_header(1_638_752_523_000, "h"), "Dec  6 01:02:03 h");
        assert_eq!(syslog_header(0, "h"), "Jan  1 00:00:00 h");
    }
}
