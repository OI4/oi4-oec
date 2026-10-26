//! Health (MG 9.1.2).

use serde::{Deserialize, Serialize};

/// HealthState in the VerboseEncoding (REQ-RES-HEALTH-001).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum HealthState {
    /// `NORMAL_0`.
    #[serde(rename = "NORMAL_0")]
    Normal,
    /// `FAILURE_1`.
    #[serde(rename = "FAILURE_1")]
    Failure,
    /// `CHECK_FUNCTION_2`.
    #[serde(rename = "CHECK_FUNCTION_2")]
    CheckFunction,
    /// `OFF_SPEC_3`.
    #[serde(rename = "OFF_SPEC_3")]
    OffSpec,
    /// `MAINTENANCE_REQUIRED_4`.
    #[serde(rename = "MAINTENANCE_REQUIRED_4")]
    MaintenanceRequired,
}

impl HealthState {
    /// Severity in the aggregation order of REQ-RES-HEALTH-002:
    /// FAILURE > CHECK_FUNCTION > OFF_SPEC > MAINTENANCE_REQUIRED > NORMAL.
    pub const fn severity(self) -> u8 {
        match self {
            Self::Normal => 0,
            Self::MaintenanceRequired => 1,
            Self::OffSpec => 2,
            Self::CheckFunction => 3,
            Self::Failure => 4,
        }
    }
}

/// A detail of the Health (REQ-RES-HEALTH-001).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct DetailObject {
    /// State of the detail.
    pub health: HealthState,
    /// DiagnosticCode.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub diagnostic_code: Option<String>,
    /// Location.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub location: Option<String>,
    /// Description.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
}

/// The Health of an asset (REQ-RES-HEALTH-001).
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct Health {
    /// The overall state.
    pub health: HealthState,
    /// HealthScore 0 … 100.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub health_score: Option<u8>,
    /// Details.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub details: Option<Vec<DetailObject>>,
}

impl Health {
    /// A Health whose state is the most severe state of its details
    /// (REQ-RES-HEALTH-002); without details it is NORMAL_0.
    pub fn aggregate(details: Vec<DetailObject>) -> Self {
        let health = details
            .iter()
            .map(|d| d.health)
            .max_by_key(|h| h.severity())
            .unwrap_or(HealthState::Normal);
        Self {
            health,
            health_score: None,
            details: Some(details),
        }
    }
}

super::data_set!(Health, "d8e7b6df-42ba-448a-975a-199f59e8ffeb");

#[cfg(test)]
mod tests {
    use super::*;

    fn detail(h: HealthState) -> DetailObject {
        DetailObject {
            health: h,
            diagnostic_code: None,
            location: None,
            description: None,
        }
    }

    #[test]
    fn aggregate_takes_the_most_severe_detail() {
        use HealthState::*;
        let h = Health::aggregate(vec![
            detail(MaintenanceRequired),
            detail(CheckFunction),
            detail(OffSpec),
        ]);
        assert_eq!(h.health, CheckFunction);
        assert_eq!(
            Health::aggregate(vec![detail(Normal), detail(Failure)]).health,
            Failure
        );
        assert_eq!(Health::aggregate(Vec::new()).health, Normal);
    }
}
