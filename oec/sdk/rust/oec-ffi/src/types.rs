//! The protocol enums of the ABI: integer types with fixed values
//! (ADR-0021 §2, amendment C-061). A value out of range is refused with
//! `OEC_ERR_INVALID_ARGUMENT`; a C enum type in the signature would make it
//! undefined behavior on the Rust side.

use std::ffi::c_uint;

use oec::ServiceType as St;
use oec::asset::HealthState;
use oec::consumer::Resource as Res_;
use oec::data::{Mode, Writable as W};
use oec::event::SyslogLevel as Sl;

use crate::abi::{Fail, Res};

/// Messaging compliance level (`OEC_LEVEL_*`).
pub type Level = i32;
/// Level 1: MQTT + TLS + auth, Birth/Will, Pub MAM + Health.
pub const OEC_LEVEL_1: Level = 1;
/// Level 2: Level 1 + `Get`, `Set`/`Del` where offered, Status events.
pub const OEC_LEVEL_2: Level = 2;
/// Level 3: all resources, full MQTT client behavior.
pub const OEC_LEVEL_3: Level = 3;

/// ServiceType of an application (REQ-TOP-005, `OEC_ST_*`).
pub type ServiceType = i32;
/// Every ServiceType: the `+` wildcard of a subscription.
pub const OEC_ST_ANY: ServiceType = -1;
/// `Registry`.
pub const OEC_ST_REGISTRY: ServiceType = 0;
/// `OTConnector`.
pub const OEC_ST_OT_CONNECTOR: ServiceType = 1;
/// `Utility`.
pub const OEC_ST_UTILITY: ServiceType = 2;
/// `Persistence`.
pub const OEC_ST_PERSISTENCE: ServiceType = 3;
/// `Aggregation`.
pub const OEC_ST_AGGREGATION: ServiceType = 4;
/// `CloudConnector`.
pub const OEC_ST_CLOUD_CONNECTOR: ServiceType = 5;
/// `ITConnector`.
pub const OEC_ST_IT_CONNECTOR: ServiceType = 6;

/// Health state (MG 8.3.2, `OEC_HEALTH_*`).
pub type Health = i32;
/// `NORMAL_0`.
pub const OEC_HEALTH_NORMAL: Health = 0;
/// `FAILURE_1`.
pub const OEC_HEALTH_FAILURE: Health = 1;
/// `CHECK_FUNCTION_2`.
pub const OEC_HEALTH_CHECK_FUNCTION: Health = 2;
/// `OFF_SPEC_3`.
pub const OEC_HEALTH_OFF_SPEC: Health = 3;
/// `MAINTENANCE_REQUIRED_4`.
pub const OEC_HEALTH_MAINTENANCE_REQUIRED: Health = 4;

/// Resource (REQ-TOP-012, `OEC_RES_*`).
pub type Resource = i32;
/// `MAM`.
pub const OEC_RES_MAM: Resource = 0;
/// `Health`.
pub const OEC_RES_HEALTH: Resource = 1;
/// `Config`.
pub const OEC_RES_CONFIG: Resource = 2;
/// `Data`.
pub const OEC_RES_DATA: Resource = 3;
/// `Metadata`.
pub const OEC_RES_METADATA: Resource = 4;
/// `Event`.
pub const OEC_RES_EVENT: Resource = 5;
/// `Profile`.
pub const OEC_RES_PROFILE: Resource = 6;
/// `PublicationList`.
pub const OEC_RES_PUBLICATION_LIST: Resource = 7;
/// `SubscriptionList`.
pub const OEC_RES_SUBSCRIPTION_LIST: Resource = 8;
/// `ReferenceDesignation`.
pub const OEC_RES_REFERENCE_DESIGNATION: Resource = 9;

/// PublicationList Mode flag `ON_REQUEST` (REQ-RES-PUBLIST-008); no flag
/// is `OFF_0` (REQ-RES-PUBLIST-007).
pub const OEC_MODE_ON_REQUEST: c_uint = 1u32;
/// Mode flag `APPLICATION` (REQ-RES-PUBLIST-009).
pub const OEC_MODE_APPLICATION: c_uint = 2u32;
/// Mode flag `SOURCE` (REQ-RES-PUBLIST-010).
pub const OEC_MODE_SOURCE: c_uint = 4u32;
/// Mode flag `FILTER` (REQ-RES-PUBLIST-011).
pub const OEC_MODE_FILTER: c_uint = 8u32;

/// What a `Set PublicationList` may change (REQ-RES-PUBLIST-016,
/// `OEC_WRITABLE_*`).
pub type Writable = i32;
/// `NONE_0`.
pub const OEC_WRITABLE_NOTHING: Writable = 0;
/// `MODE_1`.
pub const OEC_WRITABLE_MODE: Writable = 1;
/// `INTERVAL_2`: Interval and Precisions.
pub const OEC_WRITABLE_INTERVAL: Writable = 2;
/// `MODE_AND_INTERVAL_3`.
pub const OEC_WRITABLE_MODE_AND_INTERVAL: Writable = 3;

/// OPC UA StatusCode (MG 8.3.6.1): returned by the Set handlers, reported
/// by `oec_set`/`oec_del` and `oec_last_status_code`. Separate from
/// `oec_status`.
pub type StatusCode = u32;
/// `Good` (0x00000000).
pub const OEC_GOOD: StatusCode = 0u32;
/// `Bad` (0x80000000).
pub const OEC_BAD: StatusCode = 0x8000_0000_u32;
/// `BadNotFound` (0x803E0000).
pub const OEC_BAD_NOT_FOUND: StatusCode = 0x803E_0000_u32;
/// `BadNotWritable` (0x803B0000).
pub const OEC_BAD_NOT_WRITABLE: StatusCode = 0x803B_0000_u32;
/// `BadTypeMismatch` (0x80740000).
pub const OEC_BAD_TYPE_MISMATCH: StatusCode = 0x8074_0000_u32;
/// `BadOutOfRange` (0x803C0000).
pub const OEC_BAD_OUT_OF_RANGE: StatusCode = 0x803C_0000_u32;
/// `BadInvalidArgument` (0x80AB0000).
pub const OEC_BAD_INVALID_ARGUMENT: StatusCode = 0x80AB_0000_u32;
/// `BadUserAccessDenied` (0x801F0000).
pub const OEC_BAD_USER_ACCESS_DENIED: StatusCode = 0x801F_0000_u32;
/// `BadInternalError` (0x80020000).
pub const OEC_BAD_INTERNAL_ERROR: StatusCode = 0x8002_0000_u32;

/// Syslog level (RFC 5424, REQ-TOP-023, `OEC_SYSLOG_*`), from the most to
/// the least severe.
pub type SyslogLevel = i32;
/// `Emergency`.
pub const OEC_SYSLOG_EMERGENCY: SyslogLevel = 0;
/// `Alert`.
pub const OEC_SYSLOG_ALERT: SyslogLevel = 1;
/// `Critical`.
pub const OEC_SYSLOG_CRITICAL: SyslogLevel = 2;
/// `Error`.
pub const OEC_SYSLOG_ERROR: SyslogLevel = 3;
/// `Warning`.
pub const OEC_SYSLOG_WARNING: SyslogLevel = 4;
/// `Notice`.
pub const OEC_SYSLOG_NOTICE: SyslogLevel = 5;
/// `Informational`.
pub const OEC_SYSLOG_INFORMATIONAL: SyslogLevel = 6;
/// `Debug`.
pub const OEC_SYSLOG_DEBUG: SyslogLevel = 7;

/// Connection state of an app (`OEC_STATE_*`, `oec_app_state`).
pub type ConnectionState = i32;
/// Connecting for the first time (TCP, TLS, CONNECT); retried with backoff
/// until the broker accepts.
pub const OEC_STATE_CONNECTING: ConnectionState = 0;
/// Connected; the Birth is sent as first message (REQ-LIFE-001,
/// REQ-LIFE-015).
pub const OEC_STATE_ONLINE: ConnectionState = 1;
/// Connection lost; the SDK reconnects with backoff and sends the Birth
/// again (REQ-LIFE-001).
pub const OEC_STATE_RECONNECTING: ConnectionState = 2;
/// No connection and no attempt: before the run, after a start error and
/// after the Close (REQ-LIFE-005).
pub const OEC_STATE_CLOSED: ConnectionState = 3;

/// `timeout_ms` of `oec_app_wait`: wait without a time limit.
pub const OEC_WAIT_FOREVER: u32 = u32::MAX;

fn index<T: Copy>(all: &[T], value: i32, what: &str) -> Res<T> {
    usize::try_from(value)
        .ok()
        .and_then(|i| all.get(i).copied())
        .ok_or_else(|| Fail::invalid(format!("{value} is no {what}")))
}

pub(crate) fn level(value: Level) -> Res<oec::Level> {
    index(
        &[oec::Level::L1, oec::Level::L2, oec::Level::L3],
        value - 1,
        "oec_level",
    )
    .map_err(|_| Fail::invalid(format!("{value} is no oec_level")))
}

/// `None` for `OEC_ST_ANY`.
pub(crate) fn service_type(value: ServiceType) -> Res<Option<St>> {
    if value == OEC_ST_ANY {
        return Ok(None);
    }
    index(St::ALL, value, "oec_service_type").map(Some)
}

pub(crate) fn health(value: Health) -> Res<HealthState> {
    index(
        &[
            HealthState::Normal,
            HealthState::Failure,
            HealthState::CheckFunction,
            HealthState::OffSpec,
            HealthState::MaintenanceRequired,
        ],
        value,
        "oec_health",
    )
}

pub(crate) fn resource(value: Resource) -> Res<Res_> {
    index(Res_::ALL, value, "oec_resource")
}

pub(crate) fn resource_value(resource: Res_) -> Resource {
    Res_::ALL
        .iter()
        .position(|&r| r == resource)
        .and_then(|i| Resource::try_from(i).ok())
        .unwrap_or(-1)
}

pub(crate) fn mode(value: c_uint) -> Res<Mode> {
    if value & !0xF != 0 {
        return Err(Fail::invalid(format!(
            "{value:#x} has flags other than OEC_MODE_*"
        )));
    }
    Ok([
        (OEC_MODE_ON_REQUEST, Mode::ON_REQUEST),
        (OEC_MODE_APPLICATION, Mode::APPLICATION),
        (OEC_MODE_SOURCE, Mode::SOURCE),
        (OEC_MODE_FILTER, Mode::FILTER),
    ]
    .into_iter()
    .filter(|(flag, _)| value & flag != 0)
    .fold(Mode::OFF, |m, (_, flag)| m.union(flag)))
}

pub(crate) fn writable(value: Writable) -> Res<W> {
    index(
        &[W::Nothing, W::Mode, W::Interval, W::ModeAndInterval],
        value,
        "oec_writable",
    )
}

pub(crate) fn syslog_level(value: SyslogLevel) -> Res<Sl> {
    index(Sl::ALL, value, "oec_syslog_level")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn values_match_the_rust_enums() {
        assert_eq!(level(OEC_LEVEL_2).unwrap(), oec::Level::L2);
        assert!(level(0).is_err() && level(4).is_err());
        assert_eq!(service_type(OEC_ST_ANY).unwrap(), None);
        assert_eq!(
            service_type(OEC_ST_IT_CONNECTOR).unwrap(),
            Some(St::ITConnector)
        );
        assert_eq!(
            service_type(OEC_ST_OT_CONNECTOR).unwrap(),
            Some(St::OTConnector)
        );
        assert!(service_type(7).is_err() && service_type(-2).is_err());
        assert_eq!(
            health(OEC_HEALTH_MAINTENANCE_REQUIRED).unwrap(),
            HealthState::MaintenanceRequired
        );
        assert!(health(5).is_err());
        for (i, r) in Res_::ALL.iter().enumerate() {
            assert_eq!(resource(resource_value(*r)).unwrap(), *r);
            assert_eq!(usize::try_from(resource_value(*r)).unwrap(), i);
        }
        assert_eq!(
            resource(OEC_RES_REFERENCE_DESIGNATION).unwrap(),
            Res_::ReferenceDesignation
        );
        assert_eq!(
            resource(OEC_RES_PUBLICATION_LIST).unwrap(),
            Res_::PublicationList
        );
        assert!(resource(10).is_err());
        assert_eq!(
            mode(OEC_MODE_ON_REQUEST | OEC_MODE_FILTER).unwrap(),
            Mode::ON_REQUEST.union(Mode::FILTER)
        );
        assert_eq!(mode(0).unwrap(), Mode::OFF);
        assert!(mode(16).is_err());
        assert_eq!(writable(OEC_WRITABLE_INTERVAL).unwrap(), W::Interval);
        assert!(writable(4).is_err());
        assert_eq!(syslog_level(OEC_SYSLOG_DEBUG).unwrap(), Sl::Debug);
        assert_eq!(syslog_level(OEC_SYSLOG_WARNING).unwrap(), Sl::Warning);
        assert!(syslog_level(8).is_err());
        for code in [
            OEC_GOOD,
            OEC_BAD,
            OEC_BAD_NOT_FOUND,
            OEC_BAD_NOT_WRITABLE,
            OEC_BAD_TYPE_MISMATCH,
            OEC_BAD_OUT_OF_RANGE,
            OEC_BAD_INVALID_ARGUMENT,
            OEC_BAD_USER_ACCESS_DENIED,
            OEC_BAD_INTERNAL_ERROR,
        ] {
            let known = oec::status::StatusCode::from_code(code);
            assert!(!matches!(known, oec::status::StatusCode::Other(_)));
            assert_eq!(known.code(), code);
        }
    }
}
