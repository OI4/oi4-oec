//! The ConfigurationVersions of the Metadata of an application
//! (REQ-MSG-018, REQ-MSG-032, REQ-MSG-036): one per DataSet, stable across
//! restarts as long as the Metadata does not change.

use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::message::{ConfigurationVersion, DataSetMetaData, DecodeError};

/// Seconds from 1970-01-01 to 2000-01-01, the epoch of a VersionTime.
const EPOCH_2000: u64 = 946_684_800;

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
struct Entry {
    source: String,
    filter: String,
    data_set_class_id: Uuid,
    hash: Uuid,
    major_version: u32,
    minor_version: u32,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    timestamp: Option<u64>,
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "PascalCase", deny_unknown_fields)]
struct Wire {
    version: u32,
    entries: Vec<Entry>,
}

/// The table of ConfigurationVersions per Source and Tag. A new version
/// is a VersionTime (seconds since 2000-01-01, OPC UA Part 4 7.38): both
/// parts change with the DataSetClassId (the structure), only the
/// MinorVersion with any other change of the Metadata. Each version also
/// keeps the time of its first sending, the Timestamp of the
/// DataSetMetaData (REQ-MSG-032). Persist it with [`Self::to_json`] after
/// every change.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct MetaDataVersions {
    entries: Vec<Entry>,
}

/// The persisted table cannot be read.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct VersionsError(String);

impl core::fmt::Display for VersionsError {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        write!(f, "invalid MetaData version table: {}", self.0)
    }
}

impl std::error::Error for VersionsError {}

impl MetaDataVersions {
    /// An empty table.
    pub fn new() -> Self {
        Self::default()
    }

    /// Reads a table written by [`Self::to_json`].
    ///
    /// # Errors
    ///
    /// [`VersionsError`] for other JSON, an unknown version or a Source
    /// and Filter that occur twice.
    pub fn from_json(json: &[u8]) -> Result<Self, VersionsError> {
        let wire: Wire = serde_json::from_slice(json)
            .map_err(|e| VersionsError(DecodeError::from(e).to_string()))?;
        if wire.version != 1 {
            return Err(VersionsError(format!("unknown version {}", wire.version)));
        }
        for (i, e) in wire.entries.iter().enumerate() {
            if wire.entries[..i]
                .iter()
                .any(|o| o.source == e.source && o.filter == e.filter)
            {
                return Err(VersionsError(format!(
                    "{:?} {:?} occurs twice",
                    e.source, e.filter
                )));
            }
        }
        Ok(Self {
            entries: wire.entries,
        })
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

    /// Number of entries.
    pub fn len(&self) -> usize {
        self.entries.len()
    }

    /// No entry.
    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }

    /// The version of the Metadata `meta` (without ConfigurationVersion)
    /// of `source` and `filter` (the topic-encoded Tag) at `unix_ms`, and
    /// whether the table changed. A version never decreases, also if the
    /// clock goes back.
    pub(crate) fn version(
        &mut self,
        source: &str,
        filter: &str,
        meta: &DataSetMetaData,
        unix_ms: u64,
    ) -> (ConfigurationVersion, bool) {
        let class = meta.data_set_class_id.unwrap_or_default();
        let hash = hash(meta);
        let now = version_time(unix_ms);
        let i = self
            .entries
            .iter()
            .position(|e| e.source == source && e.filter == filter);
        let Some(i) = i else {
            self.entries.push(Entry {
                source: source.to_owned(),
                filter: filter.to_owned(),
                data_set_class_id: class,
                hash,
                major_version: now,
                minor_version: now,
                timestamp: None,
            });
            return (version(now, now), true);
        };
        let e = &mut self.entries[i];
        if e.hash == hash && e.data_set_class_id == class {
            return (version(e.major_version, e.minor_version), false);
        }
        let minor = now.max(e.minor_version.saturating_add(1));
        if e.data_set_class_id == class {
            e.minor_version = minor;
        } else {
            let major = minor.max(e.major_version.saturating_add(1));
            e.major_version = major;
            e.minor_version = major;
        }
        e.data_set_class_id = class;
        e.hash = hash;
        e.timestamp = None;
        (version(e.major_version, e.minor_version), true)
    }

    /// The time of the first sending of the current version of `source`
    /// and `filter`, set to `unix_ms` at the first call; and whether the
    /// table changed.
    pub(crate) fn first_sent(&mut self, source: &str, filter: &str, unix_ms: u64) -> (u64, bool) {
        match self
            .entries
            .iter_mut()
            .find(|e| e.source == source && e.filter == filter)
        {
            Some(Entry {
                timestamp: Some(t), ..
            }) => (*t, false),
            Some(e) => {
                e.timestamp = Some(unix_ms);
                (unix_ms, true)
            }
            None => (unix_ms, false),
        }
    }
}

fn version(major_version: u32, minor_version: u32) -> ConfigurationVersion {
    ConfigurationVersion {
        major_version,
        minor_version,
    }
}

/// The VersionTime of `unix_ms`: seconds since 2000-01-01, at least 1.
pub fn version_time(unix_ms: u64) -> u32 {
    u32::try_from((unix_ms / 1000).saturating_sub(EPOCH_2000))
        .unwrap_or(u32::MAX)
        .max(1)
}

fn hash(meta: &DataSetMetaData) -> Uuid {
    let bytes = serde_json::to_vec(meta).expect("the Metadata always serializes");
    Uuid::new_v5(&super::data::namespace(), &bytes)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn meta(class: u128, description: &str) -> DataSetMetaData {
        let mut m = DataSetMetaData::new("oee");
        m.data_set_class_id = Some(Uuid::from_u128(class));
        m.description = Some(description.into());
        m
    }

    const T0: u64 = 1_639_693_510_094;

    #[test]
    fn versions_follow_the_metadata() {
        let mut t = MetaDataVersions::new();
        let (v, changed) = t.version("S", "oee", &meta(1, "a"), T0);
        assert!(changed);
        // MG A3 example: 2021-12-16T22:25:10Z
        assert_eq!(v, version(693_008_710, 693_008_710));
        assert_eq!(t.version("S", "oee", &meta(1, "a"), T0 + 5000), (v, false));
        let (v2, changed) = t.version("S", "oee", &meta(1, "b"), T0 + 5000);
        assert!(changed);
        assert_eq!(v2, version(693_008_710, 693_008_715));
        let (v3, _) = t.version("S", "oee", &meta(2, "b"), T0 + 9000);
        assert_eq!(v3, version(693_008_719, 693_008_719));
        // the clock goes back: still increasing
        let (v4, _) = t.version("S", "oee", &meta(3, "b"), T0);
        assert_eq!(v4, version(693_008_720, 693_008_720));
        // persisted and read back
        let back = MetaDataVersions::from_json(&t.to_json()).unwrap();
        assert_eq!(back, t);
    }

    #[test]
    fn the_first_sending_is_kept_per_version() {
        let mut t = MetaDataVersions::new();
        t.version("S", "oee", &meta(1, "a"), T0);
        assert_eq!(t.first_sent("S", "oee", T0 + 1), (T0 + 1, true));
        assert_eq!(t.first_sent("S", "oee", T0 + 2), (T0 + 1, false));
        t.version("S", "oee", &meta(1, "b"), T0 + 3000);
        assert_eq!(t.first_sent("S", "oee", T0 + 4000), (T0 + 4000, true));
    }

    #[test]
    fn invalid_tables_are_refused() {
        assert!(MetaDataVersions::from_json(b"{}").is_err());
        assert!(MetaDataVersions::from_json(br#"{"Version":2,"Entries":[]}"#).is_err());
        let mut t = MetaDataVersions::new();
        t.version("S", "oee", &meta(1, "a"), T0);
        let mut twice = t.clone();
        twice.entries.push(t.entries[0].clone());
        assert!(MetaDataVersions::from_json(&twice.to_json()).is_err());
    }
}
