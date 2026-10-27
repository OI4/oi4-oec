//! Data (MG 9.1.4): DataSets whose fields the publisher defines in its
//! Metadata.

use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};
use serde_json::{Map, Value};

use crate::message::DataSet;

/// The payload of a Data DataSetMessage: the fields of the DataSet in the
/// VerboseEncoding (REQ-MSG-031). The DataSetClassId is the publisher's
/// (REQ-MSG-011, REQ-MSG-013).
#[derive(Debug, Clone, PartialEq, Default, Serialize, Deserialize)]
#[serde(transparent)]
pub struct Data(pub Map<String, Value>);

impl DataSet for Data {
    const CLASS_ID: Option<uuid::Uuid> = None;
}

impl Data {
    /// The Oi4Pv view of the payload of a DataSetMessage with the
    /// WriterGroupName `Oi4Pv` (REQ-RES-DATA-004).
    ///
    /// # Errors
    ///
    /// [`DecodeError`](crate::message::DecodeError) if Pv is missing or a
    /// field is not Pv or Sv1 … Sv255.
    pub fn oi4_pv(&self) -> Result<Oi4Pv, crate::message::DecodeError> {
        Oi4Pv::try_from(self.0.clone())
    }
}

/// The Oi4Pv DataSet: the process value Pv and the secondary values
/// Sv1 … Sv255 (REQ-RES-DATA-004).
#[derive(Debug, Clone, PartialEq)]
pub struct Oi4Pv {
    /// The process value.
    pub pv: Value,
    /// The secondary values by index 1 … 255.
    pub sv: BTreeMap<u8, Value>,
}

/// The index of `Sv1` … `Sv255` without leading zeros.
pub(crate) fn sv_index(key: &str) -> Option<u8> {
    let n = key.strip_prefix("Sv")?;
    if n.starts_with('0') {
        return None;
    }
    n.parse().ok().filter(|&i| i >= 1)
}

impl TryFrom<Map<String, Value>> for Oi4Pv {
    type Error = crate::message::DecodeError;

    fn try_from(mut m: Map<String, Value>) -> Result<Self, Self::Error> {
        let pv = m
            .remove("Pv")
            .ok_or_else(|| Self::Error::new("Oi4Pv has no Pv (REQ-RES-DATA-004)"))?;
        let sv = m
            .into_iter()
            .map(|(k, v)| {
                sv_index(&k).map(|i| (i, v)).ok_or_else(|| {
                    Self::Error::new(format!("{k:?} is not a field of Oi4Pv (REQ-RES-DATA-004)"))
                })
            })
            .collect::<Result<_, _>>()?;
        Ok(Self { pv, sv })
    }
}

impl From<Oi4Pv> for Data {
    fn from(p: Oi4Pv) -> Self {
        let mut m = Map::new();
        m.insert("Pv".to_owned(), p.pv);
        for (i, v) in p.sv {
            m.insert(format!("Sv{i}"), v);
        }
        Self(m)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn oi4_pv_round_trip() {
        let Value::Object(m) = json!({"Pv": 1.5, "Sv2": "a", "Sv255": true}) else {
            unreachable!()
        };
        let data = Data(m.clone());
        let pv = data.oi4_pv().unwrap();
        assert_eq!(pv.sv.len(), 2);
        assert_eq!(Data::from(pv), data);
    }

    #[test]
    fn oi4_pv_rejects_other_fields() {
        for bad in [
            json!({"Sv1": 1}),
            json!({"Pv": 1, "Sv0": 1}),
            json!({"Pv": 1, "Sv01": 1}),
            json!({"Pv": 1, "Sv256": 1}),
            json!({"Pv": 1, "X": 1}),
        ] {
            let Value::Object(m) = bad else {
                unreachable!()
            };
            assert!(Oi4Pv::try_from(m).is_err());
        }
    }
}
