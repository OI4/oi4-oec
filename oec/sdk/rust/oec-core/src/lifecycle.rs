//! Lifecycle (MG 6.3.3 … 6.3.5) and the Health heartbeat (MG 9.1.2,
//! REQ-TOP-017) as pure state machines on the message model, sans-IO
//! (ADR-0026 amendment C-049).
//!
//! [`Lifecycle`] produces the messages of the application about itself:
//! the Will of every CONNECT (REQ-LIFE-008 … 010, REQ-LIFE-016), the Birth
//! as first publication after every connect (REQ-LIFE-001 … 003,
//! REQ-LIFE-015) and again after a recovery from FAILURE_1/0
//! (REQ-LIFE-017), the Health at connect, on change and every 60 s
//! (REQ-TOP-017, REQ-RES-HEALTH-004) and the Close before a graceful
//! disconnect (REQ-LIFE-005 … 007). [`HealthBeat`] schedules the Health of
//! any asset (REQ-CMP-007).
//!
//! The messages follow the examples of MG 6.3: one DataSetMessage, an empty
//! WriterGroupName, no Timestamp, no SequenceNumber and no MessageType
//! (REQ-LIFE-010, REQ-MSG-029: `ua-keyframe` is assumed).
//!
//! Time comes in twice: `now` is the monotonic time of the caller (as in
//! [`crate::mqtt::Session`]), `unix_ms` the wall clock for the MessageId
//! (REQ-MSG-007).

use core::fmt;
use core::time::Duration;

use crate::il::IdentificationLink;
use crate::message::{
    DataNetworkMessage, DataSetEntry, DataSetMessage, DataSetWriterId, Message, MessageId,
    MessageIds, PublisherId,
};
use crate::mqtt::{LastWill, QoS};
use crate::resource::{Health, HealthState, Mam};
use crate::topic::{Method, Resource, ServiceType, Topic, TopicError};

/// The interval of the Health heartbeat (REQ-TOP-017).
pub const HEALTH_HEARTBEAT: Duration = Duration::from_secs(60);

/// A message to publish with QoS 0 and without Retain (REQ-MQTT-017,
/// REQ-MQTT-018).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Publication {
    /// The topic.
    pub topic: Topic,
    /// The NetworkMessage as compact JSON.
    pub payload: Vec<u8>,
}

/// A value the lifecycle refuses.
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub enum LifecycleError {
    /// The ProductInstanceUri of the MAM is not the AppId (REQ-LIFE-003).
    NotSelf {
        /// The AppId.
        app_id: String,
        /// The ProductInstanceUri of the MAM.
        product_instance_uri: String,
    },
    /// MAM and Health about the application share a DataSetWriterId
    /// (REQ-MSG-020: one per DataSet).
    SharedWriterId(u16),
    /// HealthScore above 100 (REQ-RES-HEALTH-001).
    HealthScore(u8),
    /// Health is not the most severe state of the Details
    /// (REQ-RES-HEALTH-002).
    Aggregation {
        /// Health of the value.
        health: HealthState,
        /// The most severe state of the Details.
        details: HealthState,
    },
    /// NORMAL_0 with HealthScore 0 about the application itself is the
    /// Close message only (REQ-LIFE-014).
    CloseHealth,
    /// The Health of an asset is the Health of the application itself; use
    /// [`Lifecycle::set_health`].
    SelfAsset,
    /// A topic can not be built.
    Topic(TopicError),
}

impl fmt::Display for LifecycleError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::NotSelf {
                app_id,
                product_instance_uri,
            } => write!(
                f,
                "ProductInstanceUri {product_instance_uri:?} of the Birth is not the AppId {app_id:?} (REQ-LIFE-003)"
            ),
            Self::SharedWriterId(id) => write!(
                f,
                "MAM and Health share the DataSetWriterId {id} (REQ-MSG-020)"
            ),
            Self::HealthScore(s) => write!(f, "HealthScore {s} above 100 (REQ-RES-HEALTH-001)"),
            Self::Aggregation { health, details } => write!(
                f,
                "Health {health:?} is not the most severe Details state {details:?} (REQ-RES-HEALTH-002)"
            ),
            Self::CloseHealth => f.write_str(
                "NORMAL_0 with HealthScore 0 about the application itself is reserved for the Close message (REQ-LIFE-014)",
            ),
            Self::SelfAsset => {
                f.write_str("the Health of the application itself is set with Lifecycle::set_health")
            }
            Self::Topic(e) => e.fmt(f),
        }
    }
}

impl std::error::Error for LifecycleError {}

impl From<TopicError> for LifecycleError {
    fn from(e: TopicError) -> Self {
        Self::Topic(e)
    }
}

/// Checks a Health value: HealthScore 0 … 100 (REQ-RES-HEALTH-001), Health
/// is the most severe state of non-empty Details (REQ-RES-HEALTH-002).
///
/// # Errors
///
/// [`LifecycleError::HealthScore`], [`LifecycleError::Aggregation`].
pub fn check_health(health: &Health) -> Result<(), LifecycleError> {
    if let Some(score) = health.health_score
        && score > 100
    {
        return Err(LifecycleError::HealthScore(score));
    }
    if let Some(details) = &health.details
        && let Some(worst) = details
            .iter()
            .map(|d| d.health)
            .max_by_key(|h| h.severity())
        && worst != health.health
    {
        return Err(LifecycleError::Aggregation {
            health: health.health,
            details: worst,
        });
    }
    Ok(())
}

const fn is_failure(health: &Health) -> bool {
    matches!(health.health, HealthState::Failure) && matches!(health.health_score, Some(0))
}

const fn is_close(health: &Health) -> bool {
    matches!(health.health, HealthState::Normal) && matches!(health.health_score, Some(0))
}

/// The schedule of the Health of one asset (REQ-TOP-017,
/// REQ-RES-HEALTH-004): due at the start, after every change (Details
/// included) and [`HEALTH_HEARTBEAT`] after the last publication.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct HealthBeat {
    health: Health,
    changed: bool,
    last: Option<Duration>,
}

impl HealthBeat {
    /// A schedule whose first publication is due at once.
    pub fn new(health: Health) -> Self {
        Self {
            health,
            changed: true,
            last: None,
        }
    }

    /// The current Health.
    pub fn health(&self) -> &Health {
        &self.health
    }

    /// Replaces the Health; a different value (any Details change included)
    /// is due at once. Returns whether it changed.
    pub fn set(&mut self, health: Health) -> bool {
        if health == self.health {
            return false;
        }
        self.health = health;
        self.changed = true;
        true
    }

    /// Makes the Health due at once (e.g. after a reconnect).
    pub fn restart(&mut self) {
        self.changed = true;
    }

    /// The instant the next publication is due; `Duration::ZERO` if due at
    /// once.
    pub fn poll_timeout(&self) -> Duration {
        match self.last {
            Some(last) if !self.changed => last + HEALTH_HEARTBEAT,
            _ => Duration::ZERO,
        }
    }

    /// The Health if it is due at `now`; it then counts as published.
    pub fn poll(&mut self, now: Duration) -> Option<&Health> {
        if now < self.poll_timeout() {
            return None;
        }
        self.changed = false;
        self.last = Some(now);
        Some(&self.health)
    }
}

/// The lifecycle of an application (MG 6.3): Birth, Will, Close and its
/// own Health.
#[derive(Debug, Clone)]
pub struct Lifecycle {
    ids: MessageIds,
    mam: Mam,
    mam_writer: DataSetWriterId,
    health_writer: DataSetWriterId,
    mam_topic: Topic,
    health_topic: Topic,
    beat: HealthBeat,
    /// FAILURE_1/0 about itself was published while connected
    /// (REQ-LIFE-012); the next other Health needs a Birth (REQ-LIFE-017).
    unavailable: bool,
}

impl Lifecycle {
    /// The lifecycle of the application `app_id` with its MAM and the
    /// DataSetWriterIds of its MAM and Health about itself (REQ-LIFE-003,
    /// REQ-LIFE-007, REQ-LIFE-009). The Health starts as NORMAL_0 with
    /// HealthScore 100.
    ///
    /// # Errors
    ///
    /// [`LifecycleError::NotSelf`] if the ProductInstanceUri of `mam` is
    /// not `app_id` (REQ-LIFE-003), [`LifecycleError::SharedWriterId`],
    /// [`LifecycleError::Topic`].
    pub fn new(
        service_type: ServiceType,
        app_id: IdentificationLink,
        mam: Mam,
        mam_writer: DataSetWriterId,
        health_writer: DataSetWriterId,
    ) -> Result<Self, LifecycleError> {
        if mam.product_instance_uri != app_id.as_str() {
            return Err(LifecycleError::NotSelf {
                app_id: app_id.to_string(),
                product_instance_uri: mam.product_instance_uri,
            });
        }
        if mam_writer == health_writer {
            return Err(LifecycleError::SharedWriterId(mam_writer.get()));
        }
        let own = |resource| {
            Topic::new(
                service_type,
                app_id.clone(),
                Method::Pub,
                resource,
                Some(app_id.clone()),
                None,
            )
        };
        Ok(Self {
            mam_topic: own(Resource::Mam)?,
            health_topic: own(Resource::Health)?,
            ids: MessageIds::new(PublisherId::new(service_type, app_id)),
            mam,
            mam_writer,
            health_writer,
            beat: HealthBeat::new(Health {
                health: HealthState::Normal,
                health_score: Some(100),
                details: None,
            }),
            unavailable: false,
        })
    }

    /// The AppId.
    pub fn app_id(&self) -> &IdentificationLink {
        self.ids.publisher().app_id()
    }

    /// The PublisherId.
    pub fn publisher_id(&self) -> &PublisherId {
        self.ids.publisher()
    }

    /// `Pub/MAM/<AppId>` (REQ-LIFE-002).
    pub fn mam_topic(&self) -> &Topic {
        &self.mam_topic
    }

    /// `Pub/Health/<AppId>` (REQ-LIFE-006, REQ-LIFE-008).
    pub fn health_topic(&self) -> &Topic {
        &self.health_topic
    }

    /// The MAM of the Birth.
    pub fn mam(&self) -> &Mam {
        &self.mam
    }

    /// The Health of the application.
    pub fn health(&self) -> &Health {
        self.beat.health()
    }

    /// The next MessageId (REQ-MSG-007, REQ-MSG-008). All publications of
    /// the application take their ids from here, so they never collide
    /// with those of the lifecycle.
    pub fn next_message_id(&mut self, unix_ms: u64) -> MessageId {
        self.ids.next_id(unix_ms)
    }

    /// The Will of the next CONNECT: Health FAILURE_1 with HealthScore 0
    /// on `Pub/Health/<AppId>` with the Health DataSetWriterId, QoS 0, no
    /// Retain (REQ-LIFE-008 … 010), created anew with the MessageId of its
    /// creation (REQ-LIFE-016).
    pub fn will(&mut self, unix_ms: u64) -> LastWill {
        let payload = self.self_health(
            unix_ms,
            Health {
                health: HealthState::Failure,
                health_score: Some(0),
                details: None,
            },
        );
        LastWill {
            topic: self.health_topic.to_string(),
            payload,
            qos: QoS::AtMostOnce,
        }
    }

    /// The Birth: Pub MAM about itself with one DataSetMessage and the MAM
    /// DataSetWriterId (REQ-LIFE-002, REQ-LIFE-003).
    pub fn birth(&mut self, unix_ms: u64) -> Publication {
        let id = self.ids.next_id(unix_ms);
        let dsm = DataSetMessage::new(self.mam_writer, self.app_id().clone(), "", self.mam.clone())
            .without_message_type();
        let msg = DataNetworkMessage::new(id, self.ids.publisher().clone())
            .with(DataSetEntry::Payload(dsm));
        Publication {
            topic: self.mam_topic.clone(),
            payload: encode(&Message::Mam(msg)),
        }
    }

    /// The Close: Health NORMAL_0 with HealthScore 0 about itself with the
    /// Health DataSetWriterId (REQ-LIFE-005 … 007); publish it, then
    /// DISCONNECT.
    pub fn close(&mut self, unix_ms: u64) -> Publication {
        let payload = self.self_health(
            unix_ms,
            Health {
                health: HealthState::Normal,
                health_score: Some(0),
                details: None,
            },
        );
        Publication {
            topic: self.health_topic.clone(),
            payload,
        }
    }

    /// The connection is established (CONNACK): the Birth first
    /// (REQ-LIFE-001, REQ-LIFE-015), then the Health (REQ-TOP-017).
    pub fn on_connected(&mut self, now: Duration, unix_ms: u64) -> Vec<Publication> {
        self.unavailable = false;
        self.beat.restart();
        let mut out = vec![self.birth(unix_ms)];
        out.extend(self.poll(now, unix_ms));
        out
    }

    /// Sets the Health of the application; [`Self::poll`] publishes a
    /// change at once (REQ-TOP-017, REQ-RES-HEALTH-004). Returns whether it
    /// changed.
    ///
    /// # Errors
    ///
    /// [`LifecycleError::CloseHealth`] for NORMAL_0 with HealthScore 0
    /// (REQ-LIFE-014) and the errors of [`check_health`].
    pub fn set_health(&mut self, health: Health) -> Result<bool, LifecycleError> {
        check_health(&health)?;
        if is_close(&health) {
            return Err(LifecycleError::CloseHealth);
        }
        Ok(self.beat.set(health))
    }

    /// The instant [`Self::poll`] has work (monotonic time).
    pub fn poll_timeout(&self) -> Duration {
        self.beat.poll_timeout()
    }

    /// The publications due at `now` while connected: the Health on change
    /// or heartbeat, preceded by the Birth if the application recovers from
    /// FAILURE_1/0 about itself (REQ-LIFE-017).
    pub fn poll(&mut self, now: Duration, unix_ms: u64) -> Vec<Publication> {
        let Some(health) = self.beat.poll(now).cloned() else {
            return Vec::new();
        };
        let mut out = Vec::with_capacity(2);
        let failure = is_failure(&health);
        if self.unavailable && !failure {
            out.push(self.birth(unix_ms));
        }
        self.unavailable = failure;
        out.push(Publication {
            topic: self.health_topic.clone(),
            payload: self.self_health(unix_ms, health),
        });
        out
    }

    /// A Health about an asset of the application (REQ-CMP-007):
    /// `Pub/Health/<asset>` with one DataSetMessage. Schedule it with a
    /// [`HealthBeat`] per asset.
    ///
    /// # Errors
    ///
    /// [`LifecycleError::SelfAsset`] for the AppId, the errors of
    /// [`check_health`], [`LifecycleError::Topic`].
    pub fn asset_health(
        &mut self,
        asset: &IdentificationLink,
        writer: DataSetWriterId,
        health: &Health,
        unix_ms: u64,
    ) -> Result<Publication, LifecycleError> {
        if asset == self.app_id() {
            return Err(LifecycleError::SelfAsset);
        }
        check_health(health)?;
        let topic = Topic::new(
            self.health_topic.service_type(),
            self.app_id().clone(),
            Method::Pub,
            Resource::Health,
            Some(asset.clone()),
            None,
        )?;
        let payload = self.health_message(unix_ms, writer, asset.clone(), health.clone());
        Ok(Publication { topic, payload })
    }

    fn self_health(&mut self, unix_ms: u64, health: Health) -> Vec<u8> {
        let app_id = self.app_id().clone();
        self.health_message(unix_ms, self.health_writer, app_id, health)
    }

    fn health_message(
        &mut self,
        unix_ms: u64,
        writer: DataSetWriterId,
        asset: IdentificationLink,
        health: Health,
    ) -> Vec<u8> {
        let id = self.ids.next_id(unix_ms);
        let dsm = DataSetMessage::new(writer, asset, "", health).without_message_type();
        let msg = DataNetworkMessage::new(id, self.ids.publisher().clone())
            .with(DataSetEntry::Payload(dsm));
        encode(&Message::Health(msg))
    }
}

/// MAM and Health contain no maps with non-string keys, so they always
/// serialize.
fn encode(msg: &Message) -> Vec<u8> {
    msg.encode()
        .expect("MAM and Health messages always serialize to JSON")
}
