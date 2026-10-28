//! The DataSetWriterIds of an application (REQ-MSG-020, REQ-MSG-021,
//! REQ-RES-PUBLIST-006): one per publication, stable across restarts, and
//! the ids provided by the Action `NewDataSetWriterId`, reserved for their
//! `Ttl` (REQ-ACT-022, REQ-ACT-023).

use std::collections::BTreeSet;

use serde::{Deserialize, Serialize};

use crate::message::{DataSetWriterId, DecodeError};
use crate::topic::Resource;

/// What a DataSetWriterId identifies: the resource, the Source (the
/// DataSetWriterName) and the Filter (the WriterGroupName) of a
/// publication (REQ-MSG-021). All DataSetMessages of the PublicationList
/// share one id ([`WriterKey::publication_list`]), as D7 T7-MON-08 checks.
#[derive(Debug, Clone, PartialEq, Eq, Hash, PartialOrd, Ord, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct WriterKey {
    /// The resource.
    pub resource: Resource,
    /// The Source: an Identification Link or empty.
    pub source: String,
    /// The Filter in its topic form, empty if the resource has none.
    #[serde(default)]
    pub filter: String,
}

impl WriterKey {
    /// A publication of `resource` about `source` without Filter (MAM,
    /// Health, Profile, ReferenceDesignation).
    pub fn new(resource: Resource, source: &str) -> Self {
        Self {
            resource,
            source: source.to_owned(),
            filter: String::new(),
        }
    }

    /// The one key of all PublicationList DataSetMessages.
    pub fn publication_list() -> Self {
        Self::new(Resource::PublicationList, "")
    }

    /// The one key of all SubscriptionList DataSetMessages.
    pub fn subscription_list() -> Self {
        Self::new(Resource::SubscriptionList, "")
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
struct Entry {
    #[serde(flatten)]
    key: WriterKey,
    data_set_writer_id: DataSetWriterId,
}

/// An id provided by `NewDataSetWriterId`: the requested resource and
/// Source and the end of its `Ttl` in Unix milliseconds.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
struct Reserved {
    data_set_writer_id: DataSetWriterId,
    resource: Resource,
    source: String,
    expires: u64,
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "PascalCase", deny_unknown_fields)]
struct Wire {
    version: u32,
    entries: Vec<Entry>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    reserved: Vec<Reserved>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    last: Option<DataSetWriterId>,
}

/// The most ids provided by `NewDataSetWriterId` whose `Ttl` has not
/// passed yet; one more is refused ([`WriterIdError::Reservations`]).
pub const MAX_RESERVATIONS: usize = 256;

/// The table of assigned DataSetWriterIds. An id is never reused for
/// another key, also after its publication was removed, so a consumer never
/// mixes up two publications (REQ-RES-PUBLIST-006); persist it with
/// [`WriterIds::to_json`] after every change.
///
/// An id provided by `NewDataSetWriterId` ([`WriterIds::reserve`]) is
/// reserved for its `Ttl`; no id at or below the highest id ever provided
/// is assigned or provided again until the ids up to 65535 are used up
/// (SF-112).
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct WriterIds {
    entries: Vec<Entry>,
    reserved: Vec<Reserved>,
    last: Option<DataSetWriterId>,
}

/// A DataSetWriterId that cannot be assigned, or a table that cannot be
/// read.
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub enum WriterIdError {
    /// All ids from 10 to 65535 are assigned.
    Exhausted,
    /// The key already has another id, or the id belongs to another key.
    Conflict {
        /// The requested key.
        key: WriterKey,
        /// The requested id.
        id: u16,
    },
    /// The persisted table is no valid table.
    Invalid(String),
    /// [`MAX_RESERVATIONS`] ids provided by `NewDataSetWriterId` are still
    /// within their `Ttl`.
    Reservations,
}

impl core::fmt::Display for WriterIdError {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        match self {
            Self::Exhausted => f.write_str(
                "all DataSetWriterIds from 10 to 65535 are assigned (REQ-MSG-020, REQ-MSG-021)",
            ),
            Self::Conflict { key, id } => write!(
                f,
                "DataSetWriterId {id} for {} {:?} conflicts with the assigned ids (REQ-MSG-021)",
                key.resource, key.source
            ),
            Self::Invalid(e) => write!(f, "invalid DataSetWriterId table: {e}"),
            Self::Reservations => write!(
                f,
                "{MAX_RESERVATIONS} DataSetWriterIds provided by NewDataSetWriterId are still reserved"
            ),
        }
    }
}

impl std::error::Error for WriterIdError {}

impl WriterIds {
    /// An empty table.
    pub fn new() -> Self {
        Self::default()
    }

    /// Reads a table written by [`Self::to_json`].
    ///
    /// # Errors
    ///
    /// [`WriterIdError::Invalid`] for other JSON, an unknown version, an id
    /// below 10 or a key or id that occurs twice, also among the
    /// reservations.
    pub fn from_json(json: &[u8]) -> Result<Self, WriterIdError> {
        let wire: Wire = serde_json::from_slice(json)
            .map_err(|e| WriterIdError::Invalid(DecodeError::from(e).to_string()))?;
        if wire.version != 1 {
            return Err(WriterIdError::Invalid(format!(
                "unknown version {}",
                wire.version
            )));
        }
        let mut table = Self::new();
        for e in wire.entries {
            table
                .pin(e.key, e.data_set_writer_id)
                .map_err(|e| WriterIdError::Invalid(e.to_string()))?;
        }
        for r in wire.reserved {
            let id = r.data_set_writer_id.get();
            if table.used(id) || table.is_reserved(id) {
                return Err(WriterIdError::Invalid(format!(
                    "the reserved DataSetWriterId {id} is taken"
                )));
            }
            table.reserved.push(r);
        }
        table.last = wire.last;
        Ok(table)
    }

    /// The table as JSON (`{"Version":1,"Entries":[…]}`, with `Reserved`
    /// and `Last` once `NewDataSetWriterId` provided an id).
    ///
    /// # Panics
    ///
    /// Never: the table has no maps with non-string keys.
    pub fn to_json(&self) -> Vec<u8> {
        serde_json::to_vec(&Wire {
            version: 1,
            entries: self.entries.clone(),
            reserved: self.reserved.clone(),
            last: self.last,
        })
        .expect("the table always serializes")
    }

    /// The id of `key`, if assigned.
    pub fn get(&self, key: &WriterKey) -> Option<DataSetWriterId> {
        self.entries
            .iter()
            .find(|e| &e.key == key)
            .map(|e| e.data_set_writer_id)
    }

    fn used(&self, id: u16) -> bool {
        self.entries
            .iter()
            .any(|e| e.data_set_writer_id.get() == id)
    }

    fn is_reserved(&self, id: u16) -> bool {
        self.reserved
            .iter()
            .any(|r| r.data_set_writer_id.get() == id)
    }

    /// The next id to assign or provide: the lowest id above every id
    /// assigned, reserved or ever provided (from 10, REQ-MSG-020), or the
    /// lowest id neither assigned nor reserved once 65535 is taken.
    fn next_id(&self) -> Result<DataSetWriterId, WriterIdError> {
        let taken: BTreeSet<u16> = self
            .entries
            .iter()
            .map(|e| e.data_set_writer_id.get())
            .chain(self.reserved.iter().map(|r| r.data_set_writer_id.get()))
            .collect();
        let high = taken
            .last()
            .copied()
            .max(self.last.map(DataSetWriterId::get));
        let mut next = high.map_or(Some(DataSetWriterId::FIRST.get()), |m| m.checked_add(1));
        while let Some(n) = next.filter(|n| taken.contains(n)) {
            next = n.checked_add(1);
        }
        let id = match next {
            Some(n) => n,
            None => (DataSetWriterId::FIRST.get()..=u16::MAX)
                .find(|n| !taken.contains(n))
                .ok_or(WriterIdError::Exhausted)?,
        };
        DataSetWriterId::new(id).map_err(|_| WriterIdError::Exhausted)
    }

    /// Number of assigned ids.
    pub fn len(&self) -> usize {
        self.entries.len()
    }

    /// No id assigned.
    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }

    /// The id of `key`; a new key gets the lowest id above all assigned,
    /// reserved and provided ones (from 10, REQ-MSG-020), or the lowest
    /// free one once 65535 is taken. Returns the id and whether the table
    /// changed.
    ///
    /// # Errors
    ///
    /// [`WriterIdError::Exhausted`].
    pub fn assign(&mut self, key: WriterKey) -> Result<(DataSetWriterId, bool), WriterIdError> {
        if let Some(id) = self.get(&key) {
            return Ok((id, false));
        }
        let id = self.next_id()?;
        self.entries.push(Entry {
            key,
            data_set_writer_id: id,
        });
        Ok((id, true))
    }

    /// Assigns the fixed `id` to `key` (e.g. MAM 10 and Health 11 of the
    /// application, REQ-LIFE-003/007). Returns whether the table changed.
    ///
    /// # Errors
    ///
    /// [`WriterIdError::Conflict`] if `key` has another id or `id` belongs
    /// to another key.
    pub fn pin(&mut self, key: WriterKey, id: DataSetWriterId) -> Result<bool, WriterIdError> {
        if self.is_reserved(id.get()) {
            return Err(WriterIdError::Conflict { key, id: id.get() });
        }
        for e in &self.entries {
            match (e.key == key, e.data_set_writer_id == id) {
                (true, true) => return Ok(false),
                (false, false) => {}
                _ => return Err(WriterIdError::Conflict { key, id: id.get() }),
            }
        }
        self.entries.push(Entry {
            key,
            data_set_writer_id: id,
        });
        Ok(true)
    }
}

impl WriterIds {
    /// Provides a new, so far unused id for `resource` of `source` and
    /// reserves it until `now_ms + ttl_ms` (NewDataSetWriterId,
    /// REQ-ACT-022, REQ-ACT-023); reservations whose `Ttl` has passed are
    /// dropped first. The table changes: persist it.
    ///
    /// # Errors
    ///
    /// [`WriterIdError::Reservations`] if [`MAX_RESERVATIONS`] are within
    /// their `Ttl`, [`WriterIdError::Exhausted`] if no id is free.
    pub fn reserve(
        &mut self,
        resource: Resource,
        source: &str,
        now_ms: u64,
        ttl_ms: u64,
    ) -> Result<DataSetWriterId, WriterIdError> {
        self.reserved.retain(|r| r.expires > now_ms);
        if self.reserved.len() >= MAX_RESERVATIONS {
            return Err(WriterIdError::Reservations);
        }
        let id = self.next_id()?;
        self.reserved.push(Reserved {
            data_set_writer_id: id,
            resource,
            source: source.to_owned(),
            expires: now_ms.saturating_add(ttl_ms),
        });
        self.last = Some(self.last.map_or(id, |l| l.max(id)));
        Ok(id)
    }

    /// Whether `id` is reserved by [`Self::reserve`] at `now_ms`.
    pub fn is_reserved_at(&self, id: DataSetWriterId, now_ms: u64) -> bool {
        self.reserved
            .iter()
            .any(|r| r.data_set_writer_id == id && r.expires > now_ms)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn id(n: u16) -> DataSetWriterId {
        DataSetWriterId::new(n).unwrap()
    }

    #[test]
    fn assign_is_stable_and_never_reuses() {
        let mut t = WriterIds::new();
        assert_eq!(t.pin(WriterKey::new(Resource::Mam, "a"), id(10)), Ok(true));
        assert_eq!(t.pin(WriterKey::new(Resource::Mam, "a"), id(10)), Ok(false));
        let (h, new) = t.assign(WriterKey::new(Resource::Health, "a")).unwrap();
        assert_eq!((h.get(), new), (11, true));
        assert_eq!(
            t.assign(WriterKey::new(Resource::Health, "a")).unwrap(),
            (h, false)
        );
        let back = WriterIds::from_json(&t.to_json()).unwrap();
        assert_eq!(back, t);
        assert!(matches!(
            t.pin(WriterKey::new(Resource::Profile, "a"), id(10)),
            Err(WriterIdError::Conflict { id: 10, .. })
        ));
        assert!(matches!(
            t.pin(WriterKey::new(Resource::Mam, "a"), id(12)),
            Err(WriterIdError::Conflict { id: 12, .. })
        ));
    }

    #[test]
    fn wraps_to_the_lowest_free_id() {
        let mut t = WriterIds::new();
        t.pin(WriterKey::new(Resource::Mam, "x"), id(u16::MAX))
            .unwrap();
        let (n, _) = t.assign(WriterKey::new(Resource::Mam, "y")).unwrap();
        assert_eq!(n.get(), 10);
    }

    #[test]
    fn reservations_are_unique_and_never_handed_out_again() {
        let mut t = WriterIds::new();
        t.pin(WriterKey::new(Resource::Mam, "a"), id(10)).unwrap();
        let r1 = t.reserve(Resource::ReferenceDesignation, "a", 1_000, 60_000);
        assert_eq!(r1, Ok(id(11)));
        assert!(t.is_reserved_at(id(11), 60_999));
        assert!(!t.is_reserved_at(id(11), 61_000));
        // an assignment skips the reservation
        let (h, _) = t.assign(WriterKey::new(Resource::Health, "a")).unwrap();
        assert_eq!(h.get(), 12);
        assert!(matches!(
            t.pin(WriterKey::new(Resource::Profile, "a"), id(11)),
            Err(WriterIdError::Conflict { id: 11, .. })
        ));
        let json = t.to_json();
        assert!(String::from_utf8_lossy(&json).contains(r#""Last":11"#));
        assert_eq!(WriterIds::from_json(&json).unwrap(), t);
        // after the Ttl the id lapses, but is not handed out again
        assert_eq!(t.reserve(Resource::Data, "a", 61_000, 60_000), Ok(id(13)));
        assert!(!t.is_reserved_at(id(11), 61_000));
        assert_eq!(t.reserved.len(), 1);
        let (p, _) = t.assign(WriterKey::new(Resource::Profile, "a")).unwrap();
        assert_eq!(p.get(), 14);
        // a restart keeps the high-water mark
        let mut back = WriterIds::from_json(&t.to_json()).unwrap();
        back.reserved.clear();
        let (x, _) = back.assign(WriterKey::new(Resource::Mam, "b")).unwrap();
        assert_eq!(x.get(), 15);
    }

    #[test]
    fn reservations_are_capped_and_wrap() {
        let mut t = WriterIds::new();
        for _ in 0..MAX_RESERVATIONS {
            t.reserve(Resource::Data, "a", 0, 10).unwrap();
        }
        assert_eq!(
            t.reserve(Resource::Data, "a", 9, 10),
            Err(WriterIdError::Reservations)
        );
        assert!(t.reserve(Resource::Data, "a", 10, 10).is_ok());
        let mut t = WriterIds::new();
        t.pin(WriterKey::new(Resource::Mam, "x"), id(u16::MAX - 1))
            .unwrap();
        assert_eq!(t.reserve(Resource::Data, "x", 0, 10), Ok(id(u16::MAX)));
        // wrapped: the lowest id neither assigned nor reserved
        assert_eq!(t.reserve(Resource::Data, "x", 0, 10), Ok(id(10)));
        let (n, _) = t.assign(WriterKey::new(Resource::Mam, "y")).unwrap();
        assert_eq!(n.get(), 11);
    }

    #[test]
    fn rejects_invalid_tables() {
        for bad in [
            &b"{}"[..],
            br#"{"Version":2,"Entries":[]}"#,
            br#"{"Version":1,"Entries":[{"Resource":"MAM","Source":"a","DataSetWriterId":9}]}"#,
            br#"{"Version":1,"Entries":[{"Resource":"MAM","Source":"a","DataSetWriterId":10},{"Resource":"Health","Source":"a","DataSetWriterId":10}]}"#,
            br#"{"Version":1,"Entries":[{"Resource":"MAM","Source":"a","DataSetWriterId":10}],"Reserved":[{"DataSetWriterId":10,"Resource":"Data","Source":"a","Expires":5}]}"#,
            br#"{"Version":1,"Entries":[],"Reserved":[{"DataSetWriterId":9,"Resource":"Data","Source":"a","Expires":5}]}"#,
            br#"{"Version":1,"Entries":[],"Last":3}"#,
        ] {
            assert!(
                matches!(WriterIds::from_json(bad), Err(WriterIdError::Invalid(_))),
                "{}",
                String::from_utf8_lossy(bad)
            );
        }
    }
}
