//! Data DataSets with PublicationList semantics and Config DataSets of an
//! asset (ADR-0021 §5, §6; ADR-0020 §4, §5).

// The ABI boundary: raw pointers and `extern "C"` functions (ADR-0026 §4).
#![allow(unsafe_code)]

use std::ffi::{c_char, c_uint, c_void};
use std::sync::Arc;
use std::time::Duration;

use oec::config::{ConfigDef, ConfigHandle};
use oec::data::{BuiltinType, DataSetDef, Field, Publication};
use oec::status::StatusCode as Code;
use serde::Deserialize;

use crate::abi::{
    Fail, Handle, Res, Status, UserData, c_string, callback, guard, handle, out, text,
};
use crate::app::{Asset, Shared};
use crate::types::{self, StatusCode, Writable};

/// Handler of a `Set Data` or `Set Config`: gets the checked request as
/// JSON (borrowed for the call) and returns a StatusCode; a Good one
/// (severity bits 0) accepts, any other rejects with that code
/// (REQ-RES-DATA-011, REQ-RES-CONFIG-023). Runs on the SDK thread.
pub type SetFn =
    Option<unsafe extern "C" fn(user_data: *mut c_void, json: *const c_char) -> StatusCode>;

/// Calls the C handler `f` with `json`.
fn decide(
    f: unsafe extern "C" fn(*mut c_void, *const c_char) -> StatusCode,
    user_data: UserData,
    json: &str,
) -> Result<(), Code> {
    let json = c_string(json);
    // SAFETY: the caller's handler with its user_data (header contract).
    let code = callback(|| unsafe { f(user_data.get(), json.as_ptr()) });
    if code & 0xC000_0000 == 0 {
        Ok(())
    } else {
        Err(Code::from_code(code))
    }
}

fn handler(f: SetFn) -> Res<unsafe extern "C" fn(*mut c_void, *const c_char) -> StatusCode> {
    f.ok_or_else(|| Fail::invalid("fn is NULL"))
}

// -------------------------------------------------------------------- data

/// A Data DataSet; owned by the app.
pub struct DataSet {
    data: oec::data::DataSet,
    shared: Arc<Shared>,
}

impl Handle for DataSet {
    fn shared(&self) -> &Arc<Shared> {
        &self.shared
    }
}

/// One element of `fields_json`.
#[derive(Deserialize)]
#[serde(deny_unknown_fields, rename_all = "PascalCase")]
struct FieldJson {
    name: String,
    #[serde(rename = "Type")]
    ty: String,
    unit: Option<String>,
    description: Option<String>,
    range: Option<RangeJson>,
    precision: Option<f64>,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields, rename_all = "PascalCase")]
struct RangeJson {
    low: f64,
    high: f64,
}

fn builtin_type(name: &str) -> Res<BuiltinType> {
    Ok(match name {
        "Boolean" => BuiltinType::Boolean,
        "SByte" => BuiltinType::SByte,
        "Byte" => BuiltinType::Byte,
        "Int16" => BuiltinType::Int16,
        "UInt16" => BuiltinType::UInt16,
        "Int32" => BuiltinType::Int32,
        "UInt32" => BuiltinType::UInt32,
        "Int64" => BuiltinType::Int64,
        "UInt64" => BuiltinType::UInt64,
        "Float" => BuiltinType::Float,
        "Double" => BuiltinType::Double,
        "String" => BuiltinType::String,
        "DateTime" => BuiltinType::DateTime,
        "Guid" => BuiltinType::Guid,
        "ByteString" => BuiltinType::ByteString,
        "LocalizedText" => BuiltinType::LocalizedText,
        other => return Err(Fail::invalid(format!("{other:?} is no BuiltinType"))),
    })
}

/// The definition of a DataSet from the C arguments.
fn data_set_def(
    tag: &str,
    fields_json: &str,
    mode: c_uint,
    interval_ms: u32,
    writable: Writable,
) -> Res<DataSetDef> {
    let fields: Vec<FieldJson> = serde_json::from_str(fields_json)
        .map_err(|e| Fail::invalid(format!("fields_json: {e}")))?;
    let mut publication = Publication::default_data()
        .mode(types::mode(mode)?)
        .interval(Duration::from_millis(u64::from(interval_ms)))
        .writable(types::writable(writable)?);
    let mut def = DataSetDef::new(tag);
    for f in fields {
        let mut field = Field::new(&f.name, builtin_type(&f.ty)?);
        if let Some(unit) = &f.unit {
            field = field.unit(unit);
        }
        if let Some(d) = &f.description {
            field = field.description(d);
        }
        if let Some(r) = f.range {
            field = field.range(r.low, r.high);
        }
        if let Some(p) = f.precision {
            publication = publication.precision(&f.name, p);
        }
        def = def.field(field);
    }
    Ok(def.publication(publication))
}

/// Adds a Data DataSet with its Metadata (REQ-RES-META-003) and
/// PublicationList entry. `fields_json` is a JSON array of
/// `{"Name": "Pv", "Type": "Double", "Unit": "°C", "Description": "…",
/// "Range": {"Low": 0, "High": 100}, "Precision": 0.5}`; `Type` is an
/// OPC UA BuiltinType name, the other members are optional, `Precision`
/// is the Precisions entry of the field (REQ-RES-PUBLIST-013). `mode`:
/// `OEC_MODE_*` flags (0 = `OFF_0`); `interval_ms` 0 = on change;
/// `writable`: what a `Set PublicationList` may change.
///
/// # Safety
///
/// `asset` is NULL or a live asset handle; strings are NULL or
/// NUL-terminated; `out` is NULL or writable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_asset_add_data(
    asset: *mut Asset,
    tag: *const c_char,
    fields_json: *const c_char,
    mode: c_uint,
    interval_ms: u32,
    writable: Writable,
    out: *mut *mut DataSet,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let out = unsafe { self::out(out, "out") }?;
        // SAFETY: header contract.
        let (asset, tag, fields) = unsafe {
            (
                handle(asset, "asset")?,
                text(tag, "tag")?,
                text(fields_json, "fields_json")?,
            )
        };
        let def = data_set_def(tag, fields, mode, interval_ms, writable)?;
        let data = asset.asset.add_data(def)?;
        *out = asset.shared.adopt(DataSet {
            data,
            shared: Arc::clone(&asset.shared),
        });
        Ok(())
    })
}

/// Sets field values, all or none: `values_json` is a JSON object of
/// field values in their OPC UA JSON form (REQ-MSG-031), e.g.
/// `{"Pv": 21.5}`. Synchronous and non-blocking; when it is published
/// follows the PublicationList entry.
///
/// # Safety
///
/// `ds` is NULL or a live DataSet handle; `values_json` is NULL or
/// NUL-terminated.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_dataset_update(
    ds: *mut DataSet,
    values_json: *const c_char,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let (ds, json) = unsafe { (handle(ds, "ds")?, text(values_json, "values_json")?) };
        Ok(ds.data.update_json(json)?)
    })
}

/// Makes the DataSet writable (`Set Data`, Level 2 on): `f` gets the
/// checked request with every field as JSON object; the SDK answers with
/// the Status event and publishes the new state (REQ-RES-DATA-011,
/// REQ-RES-DATA-012, REQ-RES-EVENT-005). A further call replaces the
/// handler.
///
/// # Safety
///
/// `ds` is NULL or a live DataSet handle; `f` and `user_data` are usable
/// on the SDK thread until the app is freed.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_dataset_on_set(
    ds: *mut DataSet,
    f: SetFn,
    user_data: *mut c_void,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let ds = unsafe { handle(ds, "ds") }?;
        let f = handler(f)?;
        let user_data = UserData::new(user_data);
        Ok(ds
            .data
            .on_set(move |set| decide(f, user_data, &set.to_json()))?)
    })
}

/// Removes the DataSet (REQ-RES-PUBLIST-017); the handle stays valid, its
/// calls fail.
///
/// # Safety
///
/// `ds` is NULL or a live DataSet handle.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_dataset_remove(ds: *mut DataSet) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let ds = unsafe { handle(ds, "ds") }?;
        Ok(ds.data.clone().remove()?)
    })
}

// ------------------------------------------------------------------ config

/// A Config DataSet; owned by the app.
pub struct Config {
    config: ConfigHandle,
    shared: Arc<Shared>,
}

impl Handle for Config {
    fn shared(&self) -> &Arc<Shared> {
        &self.shared
    }
}

/// Adds a Config DataSet: `definition_json` is its Payload as published
/// (groups and objects, MG 8.3.3), validated on add (REQ-RES-CONFIG-004,
/// REQ-RES-CONFIG-006, REQ-RES-CONFIG-007); persisted values of the same
/// `context` are restored.
///
/// # Safety
///
/// `asset` is NULL or a live asset handle; strings are NULL or
/// NUL-terminated; `out` is NULL or writable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_asset_add_config(
    asset: *mut Asset,
    context: *const c_char,
    definition_json: *const c_char,
    out: *mut *mut Config,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let out = unsafe { self::out(out, "out") }?;
        // SAFETY: header contract.
        let (asset, context, json) = unsafe {
            (
                handle(asset, "asset")?,
                text(context, "context")?,
                text(definition_json, "definition_json")?,
            )
        };
        let config = asset
            .asset
            .add_config(ConfigDef::from_json(context, json)?)?;
        *out = asset.shared.adopt(Config {
            config,
            shared: Arc::clone(&asset.shared),
        });
        Ok(())
    })
}

/// Called with the validated, complete proposal of a `Set Config` (Level 2
/// on) as JSON object `{"<group>": {"<name>": "<value>", …}, …}` with the
/// values in clear text (also of sensitive objects); on accept the SDK
/// applies and persists all values, on reject none (REQ-RES-CONFIG-023).
/// Without handler the values are applied. A further call replaces it.
///
/// # Safety
///
/// `cfg` is NULL or a live Config handle; `f` and `user_data` are usable
/// on the SDK thread until the app is freed.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_config_on_set(
    cfg: *mut Config,
    f: SetFn,
    user_data: *mut c_void,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let cfg = unsafe { handle(cfg, "cfg") }?;
        let f = handler(f)?;
        let user_data = UserData::new(user_data);
        Ok(cfg
            .config
            .on_set(move |values| decide(f, user_data, &values.to_json()))?)
    })
}

/// The current value of one object in clear text; `*value` is owned
/// (release with `oec_string_free`) or NULL if the object is unset or
/// unknown (REQ-RES-CONFIG-010).
///
/// # Safety
///
/// `cfg` is NULL or a live Config handle; strings are NULL or
/// NUL-terminated; `value` is NULL or writable.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_config_get(
    cfg: *mut Config,
    group: *const c_char,
    name: *const c_char,
    value: *mut *mut c_char,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let value = unsafe { out(value, "value") }?;
        // SAFETY: header contract.
        let (cfg, group, name) = unsafe {
            (
                handle(cfg, "cfg")?,
                text(group, "group")?,
                text(name, "name")?,
            )
        };
        if let Some(v) = cfg.config.values().string(group, name) {
            *value = c_string(v).into_raw();
        }
        Ok(())
    })
}

/// Changes a value from the application side (e.g. a local UI): the
/// checks of a `Set Config`, `""` unsets it; a change is persisted and
/// published. No Status event, no call of the `oec_config_on_set`
/// handler.
///
/// # Safety
///
/// `cfg` is NULL or a live Config handle; strings are NULL or
/// NUL-terminated.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn oec_config_set(
    cfg: *mut Config,
    group: *const c_char,
    name: *const c_char,
    value: *const c_char,
) -> Status {
    guard(|| {
        // SAFETY: header contract.
        let (cfg, group, name, value) = unsafe {
            (
                handle(cfg, "cfg")?,
                text(group, "group")?,
                text(name, "name")?,
                text(value, "value")?,
            )
        };
        Ok(cfg.config.set(group, name, value)?)
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::{OEC_MODE_ON_REQUEST, OEC_WRITABLE_NOTHING};

    #[test]
    fn fields_json() {
        let ok = r#"[{"Name":"Pv","Type":"Double","Unit":"°C","Description":"d",
            "Range":{"Low":0,"High":100},"Precision":0.5},{"Name":"S","Type":"String"}]"#;
        assert!(data_set_def("T", ok, OEC_MODE_ON_REQUEST, 0, OEC_WRITABLE_NOTHING).is_ok());
        for bad in [
            r#"[{"Name":"Pv","Type":"Real"}]"#,
            r#"[{"Name":"Pv","Type":"Double","Extra":1}]"#,
            r#"[{"Name":"Pv"}]"#,
            r#"{"Name":"Pv","Type":"Double"}"#,
            "not json",
        ] {
            assert!(
                data_set_def("T", bad, OEC_MODE_ON_REQUEST, 0, OEC_WRITABLE_NOTHING).is_err(),
                "{bad}"
            );
        }
        assert!(data_set_def("T", "[]", 16, 0, OEC_WRITABLE_NOTHING).is_err());
        assert!(data_set_def("T", "[]", 0, 0, 9).is_err());
        for name in [
            "Boolean",
            "SByte",
            "Byte",
            "Int16",
            "UInt16",
            "Int32",
            "UInt32",
            "Int64",
            "UInt64",
            "Float",
            "Double",
            "String",
            "DateTime",
            "Guid",
            "ByteString",
            "LocalizedText",
        ] {
            assert!(builtin_type(name).is_ok(), "{name}");
        }
    }

    unsafe extern "C" fn answer(user_data: *mut c_void, json: *const c_char) -> StatusCode {
        // SAFETY: the test passes a u32 and a C string.
        unsafe {
            assert_eq!(
                std::ffi::CStr::from_ptr(json).to_str().unwrap(),
                "{\"A\":1}"
            );
            *user_data.cast::<u32>()
        }
    }

    #[test]
    fn handlers_decide_by_severity() {
        for (code, expected) in [
            (0, Ok(())),
            (0x0000_0001, Ok(())),
            (0x4000_0000, Err(Code::Other(0x4000_0000))),
            (0x803E_0000, Err(Code::BadNotFound)),
        ] {
            let mut c: u32 = code;
            let user_data = UserData::new(std::ptr::from_mut(&mut c).cast());
            assert_eq!(decide(answer, user_data, "{\"A\":1}"), expected);
        }
        assert!(handler(None).is_err());
    }
}
