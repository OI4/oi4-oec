//! The DataSetWriterIds of an application (REQ-MSG-020, REQ-MSG-021,
//! REQ-RES-PUBLIST-006): one per publication, stable across restarts.

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
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
struct Entry {
    #[serde(flatten)]
    key: WriterKey,
    data_set_writer_id: DataSetWriterId,
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "PascalCase", deny_unknown_fields)]
struct Wire {
    version: u32,
    entries: Vec<Entry>,
}

/// The table of assigned DataSetWriterIds. An id is never reused for
/// another key, also after its publication was removed, so a consumer never
/// mixes up two publications (REQ-RES-PUBLIST-006); persist it with
/// [`WriterIds::to_json`] after every change.
///
/// Besides, it holds transient ids for publications about Sources that are
/// no asset of the application (the Status event `BadNotFound` of a request
/// to an unknown Source, REQ-CMP-043, REQ-CMP-047): they are taken from the
/// top of the range, never persisted and can differ after a restart.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct WriterIds {
    entries: Vec<Entry>,
    transient: Vec<Entry>,
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
    /// below 10 or a key or id that occurs twice.
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
        Ok(table)
    }

    /// The table as JSON (`{"Version":1,"Entries":[…]}`).
    ///
    /// # Panics
    ///
    /// Never: the table has no maps with non-string keys.
    pub fn to_json(&self) -> Vec<u8> {
        serde_json::to_vec(&Wire {
            version: 1,
            entries: self.entries.clone(),
        })
        .expect("the table always serializes")
    }

    /// The id of `key`, if assigned (also transient).
    pub fn get(&self, key: &WriterKey) -> Option<DataSetWriterId> {
        self.entries
            .iter()
            .chain(&self.transient)
            .find(|e| &e.key == key)
            .map(|e| e.data_set_writer_id)
    }

    fn used(&self, id: u16) -> bool {
        self.entries
            .iter()
            .chain(&self.transient)
            .any(|e| e.data_set_writer_id.get() == id)
    }

    /// The transient id of `key`: the highest free id, not persisted. At
    /// most `bound` keys get one; `None` beyond or if no id is free.
    pub fn assign_transient(&mut self, key: WriterKey, bound: usize) -> Option<DataSetWriterId> {
        if let Some(id) = self.get(&key) {
            return Some(id);
        }
        if self.transient.len() >= bound {
            return None;
        }
        let id = (DataSetWriterId::FIRST.get()..=u16::MAX)
            .rev()
            .find(|n| !self.used(*n))?;
        let id = DataSetWriterId::new(id).ok()?;
        self.transient.push(Entry {
            key,
            data_set_writer_id: id,
        });
        Some(id)
    }

    /// Number of assigned ids (without the transient ones).
    pub fn len(&self) -> usize {
        self.entries.len()
    }

    /// No id assigned.
    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }

    /// The id of `key`; a new key gets the lowest id above all assigned
    /// ones (from 10, REQ-MSG-020), or the lowest free one once 65535 is
    /// taken. Returns the id and whether the table changed.
    ///
    /// # Errors
    ///
    /// [`WriterIdError::Exhausted`].
    ///
    /// # Panics
    ///
    /// Never: the ids searched start at 10.
    pub fn assign(&mut self, key: WriterKey) -> Result<(DataSetWriterId, bool), WriterIdError> {
        if let Some(i) = self.transient.iter().position(|e| e.key == key) {
            let e = self.transient.remove(i);
            let id = e.data_set_writer_id;
            self.entries.push(e);
            return Ok((id, true));
        }
        if let Some(id) = self.get(&key) {
            return Ok((id, false));
        }
        let mut next = self
            .entries
            .iter()
            .map(|e| e.data_set_writer_id.get())
            .max()
            .map_or(Some(DataSetWriterId::FIRST.get()), |m| m.checked_add(1));
        while let Some(n) = next.filter(|n| self.used(*n)) {
            next = n.checked_add(1);
        }
        let id = match next {
            Some(n) => n,
            None => (DataSetWriterId::FIRST.get()..=u16::MAX)
                .find(|n| !self.used(*n))
                .ok_or(WriterIdError::Exhausted)?,
        };
        let id = DataSetWriterId::new(id).expect("ids start at 10");
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
        for e in self.entries.iter().chain(&self.transient) {
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
    fn transient_ids_come_from_the_top_and_are_not_persisted() {
        let mut t = WriterIds::new();
        t.pin(WriterKey::new(Resource::Mam, "a"), id(10)).unwrap();
        let foreign = |s: &str| WriterKey {
            resource: Resource::Event,
            source: s.to_owned(),
            filter: "Status/Bad".to_owned(),
        };
        assert_eq!(t.assign_transient(foreign("x"), 2), Some(id(u16::MAX)));
        assert_eq!(t.assign_transient(foreign("x"), 2), Some(id(u16::MAX)));
        assert_eq!(t.assign_transient(foreign("y"), 2), Some(id(u16::MAX - 1)));
        assert_eq!(t.assign_transient(foreign("z"), 2), None);
        assert_eq!(t.len(), 1);
        assert_eq!(WriterIds::from_json(&t.to_json()).unwrap().len(), 1);
        // A transient key that becomes persistent keeps its id.
        assert_eq!(t.assign(foreign("y")).unwrap(), (id(u16::MAX - 1), true));
        assert_eq!(t.len(), 2);
        // Persistent ids skip the transient ones.
        let mut u = WriterIds::new();
        u.pin(WriterKey::new(Resource::Mam, "a"), id(u16::MAX - 1))
            .unwrap();
        u.assign_transient(foreign("x"), 1).unwrap();
        let (n, _) = u.assign(WriterKey::new(Resource::Health, "a")).unwrap();
        assert_eq!(n.get(), 10);
    }

    #[test]
    fn rejects_invalid_tables() {
        for bad in [
            &b"{}"[..],
            br#"{"Version":2,"Entries":[]}"#,
            br#"{"Version":1,"Entries":[{"Resource":"MAM","Source":"a","DataSetWriterId":9}]}"#,
            br#"{"Version":1,"Entries":[{"Resource":"MAM","Source":"a","DataSetWriterId":10},{"Resource":"Health","Source":"a","DataSetWriterId":10}]}"#,
        ] {
            assert!(
                matches!(WriterIds::from_json(bad), Err(WriterIdError::Invalid(_))),
                "{}",
                String::from_utf8_lossy(bad)
            );
        }
    }
}
