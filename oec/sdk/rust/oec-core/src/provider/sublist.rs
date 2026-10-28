//! The SubscriptionList of the application (MG 9.1.9; P4 increment 8b,
//! ADR-0026 §18): one entry per lasting subscription of the consumer side
//! (handed in by the runtime, [`Provider::set_consumer_subscriptions`]) and
//! one per DataSet of Data or Config that can be changed with `Set`, with
//! the subscription of its `Set` topic (REQ-RES-SUBLIST-016, SF-107). It
//! is answered on `Get` (REQ-RES-SUBLIST-008), published again after every
//! change (REQ-RES-SUBLIST-005), and its configurable entries are changed
//! by `Set` and deleted by `Del`, each answered with a Status event
//! (REQ-RES-SUBLIST-004, REQ-RES-SUBLIST-010 … 015). What `Set` and `Del`
//! changed is persisted ([`SubscriptionOverrides`]) and handed back to the
//! consumer side ([`Provider::take_filter_overrides`]), which then
//! processes the publications of a filter at most once per Interval or no
//! longer.

use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

use super::write::WriteWire;
use super::{Provider, WriterKey};
use crate::il::IdentificationLink;
use crate::lifecycle::Publication;
use crate::message::{DataSetMessage, DecodeError, MessageId};
use crate::resource::{SubscriptionConfig, SubscriptionEntry};
use crate::status::StatusCode;
use crate::topic::{Filter, ListFilter, Method, Resource, Topic, mqtt_match};

/// One lasting subscription of the consumer side: one topic filter of one
/// subscription of the application.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub struct ConsumerSubscription {
    /// The MQTT topic filter.
    pub topic_path: String,
    /// The minimum time in ms between two processed publications; 0 = all.
    pub interval_ms: u32,
    /// Whether `Set` and `Del SubscriptionList` may change it (`CONF_1`).
    pub configurable: bool,
}

/// What `Set` or `Del SubscriptionList` made of a subscription.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SubscriptionSetting {
    /// The Interval in ms set by `Set`.
    Interval(u32),
    /// Deleted by `Del`: its publications are no longer processed.
    Deleted,
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "PascalCase", deny_unknown_fields)]
struct WireEntry {
    topic_path: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    interval: Option<u32>,
    #[serde(default, skip_serializing_if = "core::ops::Not::not")]
    deleted: bool,
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "PascalCase", deny_unknown_fields)]
struct Wire {
    version: u32,
    entries: Vec<WireEntry>,
}

/// The settings of subscriptions by topic filter, set by `Set` and `Del
/// SubscriptionList`; persist it with [`SubscriptionOverrides::to_json`]
/// after every change.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct SubscriptionOverrides {
    entries: BTreeMap<String, SubscriptionSetting>,
}

impl SubscriptionOverrides {
    /// An empty table.
    pub fn new() -> Self {
        Self::default()
    }

    /// Reads a table written by [`Self::to_json`].
    ///
    /// # Errors
    ///
    /// Other JSON, an unknown version, a topic filter that occurs twice or
    /// an entry with both or neither of Interval and Deleted.
    pub fn from_json(json: &[u8]) -> Result<Self, String> {
        let wire: Wire =
            serde_json::from_slice(json).map_err(|e| DecodeError::from(e).to_string())?;
        if wire.version != 1 {
            return Err(format!("unknown version {}", wire.version));
        }
        let mut table = Self::new();
        for e in wire.entries {
            let setting = match (e.interval, e.deleted) {
                (Some(ms), false) => SubscriptionSetting::Interval(ms),
                (None, true) => SubscriptionSetting::Deleted,
                _ => {
                    return Err(format!(
                        "{:?} needs either Interval or Deleted",
                        e.topic_path
                    ));
                }
            };
            if table
                .entries
                .insert(e.topic_path.clone(), setting)
                .is_some()
            {
                return Err(format!("{:?} occurs twice", e.topic_path));
            }
        }
        Ok(table)
    }

    /// The table as JSON (`{"Version":1,"Entries":[…]}`).
    ///
    /// # Panics
    ///
    /// Never: the table always serializes.
    pub fn to_json(&self) -> Vec<u8> {
        let entries = self
            .entries
            .iter()
            .map(|(path, s)| WireEntry {
                topic_path: path.clone(),
                interval: match s {
                    SubscriptionSetting::Interval(ms) => Some(*ms),
                    SubscriptionSetting::Deleted => None,
                },
                deleted: *s == SubscriptionSetting::Deleted,
            })
            .collect();
        serde_json::to_vec(&Wire {
            version: 1,
            entries,
        })
        .expect("the table always serializes")
    }

    /// The setting of a topic filter, if set.
    pub fn get(&self, topic_path: &str) -> Option<SubscriptionSetting> {
        self.entries.get(topic_path).copied()
    }

    /// Whether `Del` deleted the topic filter.
    pub fn is_deleted(&self, topic_path: &str) -> bool {
        self.get(topic_path) == Some(SubscriptionSetting::Deleted)
    }

    /// The Interval set by `Set`, if any.
    pub fn interval(&self, topic_path: &str) -> Option<u32> {
        match self.get(topic_path) {
            Some(SubscriptionSetting::Interval(ms)) => Some(ms),
            _ => None,
        }
    }

    /// Sets the setting of a topic filter; returns whether the table
    /// changed.
    pub fn set(&mut self, topic_path: &str, setting: SubscriptionSetting) -> bool {
        self.entries.insert(topic_path.to_owned(), setting) != Some(setting)
    }

    /// The topic filters with a setting and their settings, sorted.
    pub fn iter(&self) -> impl Iterator<Item = (&str, SubscriptionSetting)> {
        self.entries.iter().map(|(k, v)| (k.as_str(), *v))
    }

    /// Number of topic filters with a setting.
    pub fn len(&self) -> usize {
        self.entries.len()
    }

    /// No settings.
    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }
}

/// The WriterGroupName of an entry, `<ResourceType>[/<Tag>]` derived from
/// the TopicPath (REQ-RES-SUBLIST-001, REQ-MSG-025): the resource level and,
/// unless one of them is a wildcard or the Source level is `#`, the Filter
/// levels. `None` if the resource level is not concrete.
pub(super) fn group_of(topic_path: &str) -> Option<String> {
    let lv: Vec<&str> = topic_path.split('/').collect();
    let resource = *lv.get(4)?;
    if resource.is_empty() || resource == "+" || resource == "#" || lv[..4].contains(&"#") {
        return None;
    }
    let filter = lv.get(6..).unwrap_or_default();
    let wild = lv.get(5) == Some(&"#") || filter.iter().any(|l| *l == "+" || *l == "#");
    if filter.is_empty() || wild {
        Some(resource.to_owned())
    } else {
        Some(format!("{resource}/{}", filter.join("/")))
    }
}

/// Whether an entry with `topic_path` is addressed by `Get
/// SubscriptionList/<Source>/<ResourceType>[/<Tag>]` (REQ-RES-SUBLIST-008):
/// its resource level is the ResourceType and, with a Tag, its Filter
/// levels are that Tag or match it (wildcards, a `#` Source level).
pub(super) fn addressed(topic_path: &str, lf: &ListFilter) -> bool {
    let lv: Vec<&str> = topic_path.split('/').collect();
    if lv.get(4) != Some(&lf.resource().as_str()) {
        return false;
    }
    let Some(tag) = lf.filter().map(ToString::to_string) else {
        return true;
    };
    if lv.get(5) == Some(&"#") {
        return true;
    }
    lv.len() > 6 && mqtt_match(&lv[6..].join("/"), &tag)
}

/// One entry with its WriterGroupName.
#[derive(Debug, Clone, PartialEq, Eq)]
pub(super) struct Listed {
    pub(super) entry: SubscriptionEntry,
    pub(super) group: String,
}

impl Listed {
    fn configurable(&self) -> bool {
        self.entry.config == Some(SubscriptionConfig::Conf)
    }
}

impl Provider {
    /// The provider with the persisted settings of `Set` and `Del
    /// SubscriptionList` (empty at the first start).
    #[must_use]
    pub fn with_subscription_overrides(mut self, overrides: SubscriptionOverrides) -> Self {
        self.sub_overrides = overrides;
        self
    }

    /// The settings set by `Set` and `Del SubscriptionList`.
    pub fn subscription_overrides(&self) -> &SubscriptionOverrides {
        &self.sub_overrides
    }

    /// The settings set by `Set` and `Del SubscriptionList` as JSON if they
    /// changed since the last call; persist them.
    pub fn take_subscription_overrides(&mut self) -> Option<Vec<u8>> {
        core::mem::take(&mut self.unsaved.subscriptions).then(|| self.sub_overrides.to_json())
    }

    /// The lasting subscriptions of the consumer side changed: they are
    /// listed (from Level 2) and the SubscriptionList is published again if
    /// its entries changed (REQ-RES-SUBLIST-005).
    pub fn set_consumer_subscriptions(&mut self, mut subscriptions: Vec<ConsumerSubscription>) {
        subscriptions.sort();
        if subscriptions != self.consumer_subs {
            self.consumer_subs = subscriptions;
            self.pending.sublist = self.config.get;
        }
    }

    /// The settings the consumer side applies – those of the configurable
    /// subscriptions – if they changed since the last call: a deleted
    /// filter is no longer subscribed, a filter with an Interval is
    /// processed at most once per Interval.
    pub fn take_filter_overrides(&mut self) -> Option<SubscriptionOverrides> {
        let mut effective = SubscriptionOverrides::new();
        for (path, setting) in self.sub_overrides.iter() {
            if self.consumer_configurable(path) == Some(true) {
                effective.set(path, setting);
            }
        }
        if self.sub_sent.as_ref() == Some(&effective) {
            return None;
        }
        self.sub_sent = Some(effective.clone());
        Some(effective)
    }

    /// `Some(true)` if every subscription of the consumer side with the
    /// topic filter is configurable, `None` if there is none.
    fn consumer_configurable(&self, topic_path: &str) -> Option<bool> {
        let mut subs = self
            .consumer_subs
            .iter()
            .filter(|s| s.topic_path == topic_path)
            .peekable();
        subs.peek()?;
        Some(subs.all(|s| s.configurable))
    }

    /// The entries of the SubscriptionList: the subscriptions of the
    /// consumer side per topic filter (all `Pub`, also those of the own
    /// AppId, MG B1.1.9, SF-110), sorted, without those deleted by `Del`;
    /// then the `Set` subscriptions of every Config DataSet and of every
    /// Data DataSet the application accepts (REQ-RES-SUBLIST-016), `NONE_0`.
    /// The subscriptions that serve requests to the application (`Get`,
    /// `Set`, `Del`) are not listed (REQ-RES-SUBLIST-016), nor are the
    /// transient subscriptions of pending requests (SF-109).
    pub(super) fn subscription_entries(&self) -> Vec<Listed> {
        let mut by_filter: BTreeMap<&str, (u32, bool)> = BTreeMap::new();
        for s in &self.consumer_subs {
            let e = by_filter
                .entry(&s.topic_path)
                .or_insert((s.interval_ms, true));
            e.0 = e.0.min(s.interval_ms);
            e.1 &= s.configurable;
        }
        let mut out = Vec::new();
        for (path, (interval, configurable)) in by_filter {
            let Some(group) = group_of(path) else {
                continue;
            };
            let interval = if configurable {
                match self.sub_overrides.get(path) {
                    Some(SubscriptionSetting::Deleted) => continue,
                    Some(SubscriptionSetting::Interval(ms)) => ms,
                    None => interval,
                }
            } else {
                interval
            };
            let config = if configurable {
                SubscriptionConfig::Conf
            } else {
                SubscriptionConfig::None
            };
            out.push(Listed {
                entry: SubscriptionEntry {
                    topic_path: path.to_owned(),
                    interval: (interval > 0).then_some(interval),
                    config: Some(config),
                },
                group,
            });
        }
        out.extend(self.set_entries());
        out
    }

    /// The `Set` subscriptions of the writable Config and Data DataSets
    /// (REQ-RES-SUBLIST-016).
    fn set_entries(&self) -> Vec<Listed> {
        let config = self.configs.iter().map(|c| {
            (
                Resource::Config,
                c.source.clone(),
                Filter::ContextName(c.context.clone()),
            )
        });
        let data = self
            .sets
            .iter()
            .filter(|s| s.settable && s.mode().is_on())
            .map(|s| {
                (
                    Resource::Data,
                    s.source.clone(),
                    Filter::Tag(s.tag().to_owned()),
                )
            });
        let st = self.lifecycle.publisher_id().service_type();
        config
            .chain(data)
            .filter_map(|(resource, source, filter)| {
                let group = format!("{}/{filter}", resource.as_str());
                let topic = Topic::new(
                    st,
                    self.app_id().clone(),
                    Method::Set,
                    resource,
                    Some(source),
                    Some(filter),
                )
                .ok()?;
                Some(Listed {
                    entry: SubscriptionEntry {
                        topic_path: topic.to_string(),
                        interval: None,
                        config: Some(SubscriptionConfig::None),
                    },
                    group,
                })
            })
            .collect()
    }

    /// The SubscriptionList DataSetMessages of `listed`.
    fn sublist_messages(&self, listed: Vec<Listed>) -> Vec<DataSetMessage<SubscriptionEntry>> {
        let id = self
            .writers
            .get(&WriterKey::subscription_list())
            .expect("the SubscriptionList has a DataSetWriterId from Level 2");
        let app = self.app_id().clone();
        listed
            .into_iter()
            .map(|l| DataSetMessage::new(id, app.clone(), l.group, l.entry))
            .collect()
    }

    /// The entries answering `Get SubscriptionList[/<Source>[/<Filter>]]`
    /// (REQ-RES-SUBLIST-008): the application holds every subscription, an
    /// asset none.
    pub(super) fn sublist_items(
        &self,
        source: Option<&IdentificationLink>,
        filter: Option<&ListFilter>,
    ) -> Vec<DataSetMessage<SubscriptionEntry>> {
        if source.is_some_and(|s| s != self.app_id()) {
            return Vec::new();
        }
        let listed = self
            .subscription_entries()
            .into_iter()
            .filter(|l| filter.is_none_or(|lf| addressed(&l.entry.topic_path, lf)))
            .collect();
        self.sublist_messages(listed)
    }

    /// The whole SubscriptionList on `Pub/SubscriptionList` if its entries
    /// differ from those last published (REQ-RES-SUBLIST-005); after a
    /// connect only if it has entries.
    pub(super) fn sublist_changes(&mut self, unix_ms: u64) -> Vec<Publication> {
        let now = self.subscription_entries();
        let changed = match &self.sub_snapshot {
            Some(before) => *before != now,
            None => !now.is_empty(),
        };
        self.sub_snapshot = Some(now.clone());
        if !changed {
            return Vec::new();
        }
        let items = self.sublist_messages(now);
        self.publish(Resource::SubscriptionList, None, items, unix_ms)
    }

    /// The parts of a `Set` or `Del SubscriptionList` common to both: the
    /// ListFilter of the topic, the requested entry and the current one, if
    /// any (REQ-RES-SUBLIST-010, REQ-RES-SUBLIST-011).
    fn sublist_request(
        &self,
        topic: &Topic,
        source: &IdentificationLink,
        wire: &WriteWire,
    ) -> Result<(ListFilter, SubscriptionEntry, Option<Listed>), StatusCode> {
        let Some(Filter::List(lf)) = topic.filter() else {
            return Err(StatusCode::BAD_NOT_FOUND);
        };
        // an asset holds no subscription
        if source != self.app_id() {
            return Err(StatusCode::BAD_NOT_FOUND);
        }
        let payload = wire
            .single()?
            .get("Payload")
            .ok_or(StatusCode::BAD_DECODING_ERROR)?;
        let requested: SubscriptionEntry =
            serde_json::from_value(payload.clone()).map_err(|_| StatusCode::BAD_DECODING_ERROR)?;
        let group = Filter::List(lf.clone()).to_string();
        if group_of(&requested.topic_path).as_deref() != Some(group.as_str()) {
            return Err(StatusCode::BAD_INVALID_ARGUMENT);
        }
        let current = self
            .subscription_entries()
            .into_iter()
            .find(|l| l.entry.topic_path == requested.topic_path);
        Ok((lf.clone(), requested, current))
    }

    /// `Set SubscriptionList` (REQ-RES-SUBLIST-003, REQ-RES-SUBLIST-004,
    /// REQ-RES-SUBLIST-010, REQ-RES-SUBLIST-014): the Interval of a configurable entry; a deleted
    /// configurable subscription of the consumer side is listed again. The
    /// Config is read only (`BadNotWritable`); an entry with `NONE_0` is
    /// neither changed (`BadNotWritable`) nor is a subscription created the
    /// application does not hold (`BadNotSupported`, SF-111). The setting is
    /// persisted and the entry published on
    /// `Pub/SubscriptionList/<Source>/<Filter>` with the ReplyTo.
    pub(super) fn set_subscription_list(
        &mut self,
        topic: &Topic,
        source: &IdentificationLink,
        wire: &WriteWire,
        unix_ms: u64,
    ) -> Result<Vec<Publication>, StatusCode> {
        let (lf, requested, current) = self.sublist_request(topic, source, wire)?;
        let path = requested.topic_path.as_str();
        match current {
            Some(c) if !c.configurable() => return Err(StatusCode::BAD_NOT_WRITABLE),
            Some(_) if requested.config != Some(SubscriptionConfig::Conf) => {
                return Err(StatusCode::BAD_NOT_WRITABLE);
            }
            Some(_) => {}
            None if self.sub_overrides.is_deleted(path)
                && self.consumer_configurable(path) == Some(true) => {}
            None => return Err(StatusCode::BAD_NOT_SUPPORTED),
        }
        let setting = SubscriptionSetting::Interval(requested.interval.unwrap_or(0));
        self.unsaved.subscriptions |= self.sub_overrides.set(path, setting);
        let now = self.subscription_entries();
        let entry: Vec<Listed> = now
            .iter()
            .filter(|l| l.entry.topic_path == path)
            .cloned()
            .collect();
        self.sub_snapshot = Some(now);
        Ok(self.publish_sublist(&lf, entry, &wire.message_id, unix_ms))
    }

    /// `Del SubscriptionList` (REQ-RES-SUBLIST-011, REQ-RES-SUBLIST-015):
    /// deletes a configurable entry (`BadNotFound` for an unknown one,
    /// `BadNotWritable` for `NONE_0`); the deletion is persisted and the
    /// remaining entries of the Filter are published on
    /// `Pub/SubscriptionList/<Source>/<Filter>` with the ReplyTo – nothing if
    /// none remains.
    pub(super) fn del_subscription_list(
        &mut self,
        topic: &Topic,
        source: &IdentificationLink,
        wire: &WriteWire,
        unix_ms: u64,
    ) -> Result<Vec<Publication>, StatusCode> {
        let (lf, requested, current) = self.sublist_request(topic, source, wire)?;
        match current {
            None => return Err(StatusCode::BAD_NOT_FOUND),
            Some(c) if !c.configurable() => return Err(StatusCode::BAD_NOT_WRITABLE),
            Some(_) => {}
        }
        let path = requested.topic_path.as_str();
        self.unsaved.subscriptions |= self.sub_overrides.set(path, SubscriptionSetting::Deleted);
        let now = self.subscription_entries();
        let group = Filter::List(lf.clone()).to_string();
        let remaining: Vec<Listed> = now.iter().filter(|l| l.group == group).cloned().collect();
        self.sub_snapshot = Some(now);
        Ok(self.publish_sublist(&lf, remaining, &wire.message_id, unix_ms))
    }

    fn publish_sublist(
        &mut self,
        lf: &ListFilter,
        listed: Vec<Listed>,
        reply_to: &MessageId,
        unix_ms: u64,
    ) -> Vec<Publication> {
        if listed.is_empty() {
            return Vec::new();
        }
        let app = self.app_id().clone();
        let Ok(topic) = self.topic(
            Resource::SubscriptionList,
            Some(&app),
            Some(Filter::List(lf.clone())),
        ) else {
            return Vec::new();
        };
        let items = self.sublist_messages(listed);
        let limit = self.config.max_packet_size;
        self.series(items, None, Some(reply_to), limit, unix_ms)
            .into_iter()
            .map(|payload| Publication {
                topic: topic.clone(),
                payload,
            })
            .collect()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::topic::{EventFilter, StatusLevel};

    #[test]
    fn group_follows_the_topic_path() {
        let cases = [
            ("Oi4v2/+/+/Pub/MAM/+", Some("MAM")),
            ("Oi4v2/+/+/Pub/Health/#", Some("Health")),
            ("Oi4v2/a/b/Pub/Data/c/t", Some("Data/t")),
            ("Oi4v2/a/b/Pub/Event/c/Status/Bad", Some("Event/Status/Bad")),
            ("Oi4v2/a/b/Pub/Event/c/Status/+", Some("Event")),
            ("Oi4v2/a/b/Pub/Event/+/Syslog/#", Some("Event")),
            ("Oi4v2/a/b/Pub/Data/#", Some("Data")),
            ("Oi4v2/a/b/Pub/Data", Some("Data")),
            ("Oi4v2/a/b/Pub/+/c", None),
            ("Oi4v2/a/b/#", None),
            ("Oi4v2/a/b/Pub", None),
        ];
        for (path, want) in cases {
            assert_eq!(group_of(path).as_deref(), want, "{path}");
        }
    }

    fn lf(resource: Resource, filter: Option<Filter>) -> ListFilter {
        ListFilter::new(resource, filter).unwrap()
    }

    #[test]
    fn get_addresses_matching_entries() {
        let bad = Some(Filter::Event(EventFilter::Status(StatusLevel::Bad)));
        let tag = Some(Filter::Tag("t".to_owned()));
        let cases = [
            (
                "Oi4v2/a/b/Pub/Event/c/Status/Bad",
                lf(Resource::Event, None),
                true,
            ),
            (
                "Oi4v2/a/b/Pub/Event/c/Status/Bad",
                lf(Resource::Event, bad.clone()),
                true,
            ),
            (
                "Oi4v2/a/b/Pub/Event/c/Status/+",
                lf(Resource::Event, bad.clone()),
                true,
            ),
            (
                "Oi4v2/a/b/Pub/Event/c/Syslog/+",
                lf(Resource::Event, bad.clone()),
                false,
            ),
            (
                "Oi4v2/a/b/Pub/Event/#",
                lf(Resource::Event, bad.clone()),
                true,
            ),
            ("Oi4v2/a/b/Pub/Event/c", lf(Resource::Event, bad), false),
            (
                "Oi4v2/a/b/Pub/Data/c/t",
                lf(Resource::Data, tag.clone()),
                true,
            ),
            ("Oi4v2/a/b/Pub/Data/c/u", lf(Resource::Data, tag), false),
            ("Oi4v2/a/b/Pub/Data/c/u", lf(Resource::Mam, None), false),
        ];
        for (path, f, want) in cases {
            assert_eq!(addressed(path, &f), want, "{path} {f:?}");
        }
    }

    #[test]
    fn overrides_round_trip_and_update() {
        let mut t = SubscriptionOverrides::new();
        assert!(t.set("a/+", SubscriptionSetting::Interval(100)));
        assert!(!t.set("a/+", SubscriptionSetting::Interval(100)));
        assert!(t.set("a/+", SubscriptionSetting::Interval(0)));
        assert!(t.set("b/#", SubscriptionSetting::Deleted));
        assert_eq!(t.len(), 2);
        let back = SubscriptionOverrides::from_json(&t.to_json()).unwrap();
        assert_eq!(back, t);
        assert_eq!(back.interval("a/+"), Some(0));
        assert!(back.is_deleted("b/#"));
        assert!(!back.is_deleted("a/+"));
        assert_eq!(back.get("c"), None);
    }

    #[test]
    fn overrides_reject_invalid_tables() {
        for bad in [
            &b"{}"[..],
            br#"{"Version":2,"Entries":[]}"#,
            br#"{"Version":1,"Entries":[{"TopicPath":"a"}]}"#,
            br#"{"Version":1,"Entries":[{"TopicPath":"a","Interval":1,"Deleted":true}]}"#,
            br#"{"Version":1,"Entries":[{"TopicPath":"a","Interval":1},{"TopicPath":"a","Deleted":true}]}"#,
            br#"{"Version":1,"Entries":[],"X":1}"#,
        ] {
            assert!(SubscriptionOverrides::from_json(bad).is_err());
        }
    }
}
