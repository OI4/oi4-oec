//! The state of one Data DataSet of the provider: its definition, the last
//! complete value set and when it is due (ADR-0020 §7: state, not
//! messages).

use core::time::Duration;

use serde_json::{Map, Number};
use time::OffsetDateTime;
use uuid::Uuid;

use super::data::{DataSetSpec, Mode, Value, coerce, encode, exceeds_dead_band};
use crate::il::IdentificationLink;
use crate::message::{
    ConfigurationVersion, DataSetMessage, DataSetMetaData, DataSetVersion, DataSetWriterId,
    DateTime,
};
use crate::resource::{Data, PublicationEntry, PublicationMode};
use crate::topic::Resource;

/// One Data DataSet of a source.
#[derive(Debug, Clone)]
pub(super) struct Set {
    pub(super) source: IdentificationLink,
    pub(super) spec: DataSetSpec,
    /// The topic-encoded Tag: Filter and WriterGroupName (REQ-MSG-040).
    pub(super) filter: String,
    pub(super) class_id: Uuid,
    /// The Metadata with its ConfigurationVersion.
    pub(super) meta: DataSetMetaData,
    pub(super) writer: DataSetWriterId,
    values: Vec<Option<Value>>,
    /// Unix ms of the last change of a value: the Timestamp of the
    /// DataSetMessage.
    updated: u64,
    /// The value set last published, for the dead band.
    sent: Option<Vec<Value>>,
    /// The Data is due at the next poll.
    pub(super) pending: bool,
    /// The next cyclic publication with an Interval > 0.
    next_due: Option<Duration>,
    /// The Metadata is due at the next poll.
    pub(super) meta_pending: bool,
    precisions: Vec<f64>,
}

impl Set {
    pub(super) fn new(
        source: IdentificationLink,
        spec: DataSetSpec,
        filter: String,
        class_id: Uuid,
        meta: DataSetMetaData,
        writer: DataSetWriterId,
    ) -> Self {
        let precisions = spec
            .fields
            .iter()
            .map(|f| {
                spec.publication
                    .precisions
                    .get(&f.name)
                    .copied()
                    .unwrap_or(0.0)
            })
            .collect();
        Self {
            values: vec![None; spec.fields.len()],
            source,
            spec,
            filter,
            class_id,
            meta,
            writer,
            updated: 0,
            sent: None,
            pending: false,
            next_due: None,
            meta_pending: true,
            precisions,
        }
    }

    pub(super) fn mode(&self) -> Mode {
        self.spec.publication.mode
    }

    pub(super) fn tag(&self) -> &str {
        &self.spec.tag
    }

    fn interval(&self) -> Option<Duration> {
        let ms = self.spec.publication.interval_ms;
        (ms > 0).then(|| Duration::from_millis(u64::from(ms)))
    }

    /// The ConfigurationVersion of the Metadata.
    pub(super) fn version(&self) -> ConfigurationVersion {
        self.meta
            .configuration_version
            .expect("the provider sets the version")
    }

    /// Every field has a value: the DataSet exists on the Message Bus.
    pub(super) fn present(&self) -> bool {
        self.values.iter().all(Option::is_some)
    }

    /// Accessible on the Message Bus: Mode not OFF_0 (REQ-RES-PUBLIST-007)
    /// and every field has a value.
    pub(super) fn available(&self) -> bool {
        self.mode().is_on() && self.present()
    }

    /// The values of the fields in definition order.
    pub(super) fn values(&self) -> Option<Vec<Value>> {
        self.values.iter().cloned().collect()
    }

    /// Sets field values atomically; returns whether a value changed.
    ///
    /// # Errors
    ///
    /// An unknown field or a value that does not fit its type; nothing
    /// changes then.
    pub(super) fn update(
        &mut self,
        changes: Vec<(String, Value)>,
        unix_ms: u64,
    ) -> Result<bool, String> {
        let mut next = self.values.clone();
        for (name, v) in changes {
            let i = self
                .spec
                .fields
                .iter()
                .position(|f| f.name == name)
                .ok_or_else(|| format!("the DataSet {:?} has no field {name:?}", self.tag()))?;
            let v =
                coerce(self.spec.fields[i].ty, v).map_err(|e| format!("field {name:?}: {e}"))?;
            next[i] = Some(v);
        }
        let differs = next.iter().zip(&self.values).any(|(a, b)| match (a, b) {
            (Some(a), Some(b)) => !a.same(b),
            (a, b) => a.is_some() != b.is_some(),
        });
        if !differs {
            return Ok(false);
        }
        self.values = next;
        self.updated = unix_ms;
        if self.mode().publishes() && self.present() {
            let current = self.values().expect("present");
            self.pending |= match (&self.sent, self.interval()) {
                (None, _) => true,
                (Some(_), Some(_)) => false,
                (Some(last), None) => exceeds_dead_band(last, &current, &self.precisions),
            };
        }
        Ok(true)
    }

    /// Restarts the publication after a (re)connect: the current state at
    /// once, then every cycle anew.
    pub(super) fn restart(&mut self) {
        self.meta_pending = self.mode().is_on();
        self.pending = self.mode().publishes() && self.present();
        self.next_due = None;
    }

    /// The Data is due at `now`.
    pub(super) fn due(&self, now: Duration) -> bool {
        self.mode().publishes()
            && self.present()
            && (self.pending || self.next_due.is_some_and(|d| now >= d))
    }

    /// The instant of the next cyclic publication.
    pub(super) fn deadline(&self) -> Option<Duration> {
        if self.mode().publishes() && self.present() {
            self.next_due
        } else {
            None
        }
    }

    /// The DataSet was published at `now` because it was due: the next
    /// cycle starts.
    pub(super) fn published_due(&mut self, now: Duration) {
        self.pending = false;
        self.next_due = self.interval().map(|i| now + i);
        self.sent = self.values();
    }

    /// The DataSet was published in a group without being due: the dead
    /// band starts from these values (REQ-RES-PUBLIST-015).
    pub(super) fn published_along(&mut self) {
        self.sent = self.values();
    }

    /// The Data DataSetMessage of the current values.
    pub(super) fn message(&self) -> DataSetMessage<Data> {
        let mut payload = Map::new();
        for (f, v) in self.spec.fields.iter().zip(&self.values) {
            if let Some(v) = v {
                payload.insert(f.name.clone(), encode(f.ty, v));
            }
        }
        DataSetMessage::new(
            self.writer,
            self.source.clone(),
            self.filter.clone(),
            Data(payload),
        )
        .with_timestamp(date_time(self.updated))
        .with_version(DataSetVersion::MetaDataVersion(self.version()))
    }

    /// The PublicationList entries of the DataSet: Data with Mode,
    /// Interval, Precisions and Config, and, while it is not OFF_0, its
    /// Metadata with ON_REQUEST_FILTER_9 (REQ-RES-PUBLIST-012).
    pub(super) fn entries(
        &self,
        list_writer: DataSetWriterId,
    ) -> Vec<DataSetMessage<PublicationEntry>> {
        let p = &self.spec.publication;
        let precisions: std::collections::BTreeMap<String, Number> = p
            .precisions
            .iter()
            .filter_map(|(k, v)| Some((k.clone(), Number::from_f64(*v)?)))
            .collect();
        let data = PublicationEntry {
            resource: Resource::Data,
            source: self.source.as_str().to_owned(),
            filter: Some(self.filter.clone()),
            data_set_writer_id: self.writer,
            mode: Some(p.mode.to_publication_mode()),
            interval: (p.interval_ms > 0).then_some(p.interval_ms),
            precisions: (!precisions.is_empty()).then_some(precisions),
            config: p.writable.to_config(),
        };
        let mut out = vec![DataSetMessage::new(
            list_writer,
            self.source.clone(),
            format!("Data/{}", self.filter),
            data,
        )];
        if self.mode().is_on() {
            let meta = PublicationEntry {
                resource: Resource::Metadata,
                source: self.source.as_str().to_owned(),
                filter: Some(self.filter.clone()),
                data_set_writer_id: self.writer,
                mode: Some(PublicationMode::OnRequestFilter),
                interval: None,
                precisions: None,
                config: None,
            };
            out.push(DataSetMessage::new(
                list_writer,
                self.source.clone(),
                format!("Metadata/{}", self.filter),
                meta,
            ));
        }
        out
    }
}

/// The DateTime of `unix_ms`, clamped to the representable range.
pub(super) fn date_time(unix_ms: u64) -> DateTime {
    let at = OffsetDateTime::from_unix_timestamp_nanos(i128::from(unix_ms) * 1_000_000)
        .unwrap_or(OffsetDateTime::UNIX_EPOCH);
    DateTime::from(at)
}
