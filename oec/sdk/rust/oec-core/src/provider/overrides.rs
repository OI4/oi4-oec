//! The publication settings set by `Set PublicationList` (REQ-RES-PUBLIST-016,
//! REQ-RES-PUBLIST-025): persisted, so they survive a restart; applied to a
//! DataSet again when it is added, within what its `Config` allows.

use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

use crate::message::DecodeError;
use crate::resource::PublicationMode;

/// The settings of one Data DataSet as set over the Message Bus.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct PublicationOverride {
    /// The Source of the DataSet.
    pub source: String,
    /// The topic-encoded Tag of the DataSet.
    pub filter: String,
    /// The Mode.
    pub mode: PublicationMode,
    /// The Interval in ms.
    pub interval: u32,
    /// The Precisions (only those > 0).
    #[serde(default)]
    pub precisions: BTreeMap<String, f64>,
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "PascalCase", deny_unknown_fields)]
struct Wire {
    version: u32,
    entries: Vec<PublicationOverride>,
}

/// The table of [`PublicationOverride`]s; persist it with
/// [`PublicationOverrides::to_json`] after every change.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct PublicationOverrides {
    entries: Vec<PublicationOverride>,
}

impl PublicationOverrides {
    /// An empty table.
    pub fn new() -> Self {
        Self::default()
    }

    /// Reads a table written by [`Self::to_json`].
    ///
    /// # Errors
    ///
    /// Other JSON, an unknown version or a DataSet that occurs twice.
    pub fn from_json(json: &[u8]) -> Result<Self, String> {
        let wire: Wire =
            serde_json::from_slice(json).map_err(|e| DecodeError::from(e).to_string())?;
        if wire.version != 1 {
            return Err(format!("unknown version {}", wire.version));
        }
        let mut table = Self::new();
        for e in wire.entries {
            if table.get(&e.source, &e.filter).is_some() {
                return Err(format!(
                    "the DataSet {} {:?} occurs twice",
                    e.source, e.filter
                ));
            }
            table.entries.push(e);
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

    /// The settings of a DataSet, if set.
    pub fn get(&self, source: &str, filter: &str) -> Option<&PublicationOverride> {
        self.entries
            .iter()
            .find(|e| e.source == source && e.filter == filter)
    }

    /// Sets the settings of a DataSet; returns whether the table changed.
    pub fn set(&mut self, entry: PublicationOverride) -> bool {
        match self
            .entries
            .iter_mut()
            .find(|e| e.source == entry.source && e.filter == entry.filter)
        {
            Some(e) if *e == entry => false,
            Some(e) => {
                *e = entry;
                true
            }
            None => {
                self.entries.push(entry);
                true
            }
        }
    }

    /// Number of DataSets with settings.
    pub fn len(&self) -> usize {
        self.entries.len()
    }

    /// No settings.
    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn entry(filter: &str, interval: u32) -> PublicationOverride {
        PublicationOverride {
            source: "a.com/M/P/S".to_owned(),
            filter: filter.to_owned(),
            mode: PublicationMode::OnRequestFilter,
            interval,
            precisions: BTreeMap::from([("T".to_owned(), 0.5)]),
        }
    }

    #[test]
    fn round_trip_and_update() {
        let mut t = PublicationOverrides::new();
        assert!(t.set(entry("t", 0)));
        assert!(!t.set(entry("t", 0)));
        assert!(t.set(entry("t", 100)));
        assert!(t.set(entry("u", 0)));
        assert_eq!(t.len(), 2);
        let back = PublicationOverrides::from_json(&t.to_json()).unwrap();
        assert_eq!(back, t);
        assert_eq!(back.get("a.com/M/P/S", "t").unwrap().interval, 100);
        assert!(back.get("a.com/M/P/S", "v").is_none());
    }

    #[test]
    fn rejects_invalid_tables() {
        let twice = PublicationOverrides {
            entries: vec![entry("t", 0), entry("t", 1)],
        };
        for bad in [
            b"{}".to_vec(),
            br#"{"Version":2,"Entries":[]}"#.to_vec(),
            twice.to_json(),
        ] {
            assert!(PublicationOverrides::from_json(&bad).is_err());
        }
    }
}
