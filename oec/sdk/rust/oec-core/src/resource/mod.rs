//! The payloads of the OEC resources (MG 9.1) as [`DataSet`]s.
//!
//! Each type carries the predefined DataSetClassId of MG A2 (REQ-MSG-010)
//! and uses the VerboseEncoding of enumerations `<Name>_<Value>`
//! (REQ-MSG-031). Cross-field rules of a resource (e.g. the Health
//! aggregation REQ-RES-HEALTH-002 or the Filter rules of a publication
//! REQ-RES-PUBLIST-002) are checked by the schema; where a helper exists
//! the model offers it.
//!
//! [`DataSet`]: crate::message::DataSet

mod config;
mod data;
mod event;
mod health;
mod lists;
mod mam;
mod reference_designation;

pub use config::{
    Config, ConfigObject, ConfigObjectSet, ConfigSet, ConfigType, Context, Group, GroupSet,
    SENSITIVE_MASK, Validation,
};
pub(crate) use data::sv_index;
pub use data::{Data, Oi4Pv};
pub use event::{Event, EventCategory};
pub use health::{DetailObject, Health, HealthState};
pub use lists::{
    Profile, PublicationConfig, PublicationEntry, PublicationMode, SubscriptionConfig,
    SubscriptionEntry,
};
pub use mam::Mam;
pub use reference_designation::{
    Aspect, Designation, DesignationParent, ReferenceDesignation, ReferenceDesignationQuery,
};

/// Implements [`DataSet`](crate::message::DataSet) with a DataSetClassId.
macro_rules! data_set {
    ($ty:ty, $id:literal) => {
        $crate::resource::data_set!($ty, $id, KeyFrame);
    };
    ($ty:ty, $id:literal, $mt:ident) => {
        impl $crate::message::DataSet for $ty {
            const CLASS_ID: Option<uuid::Uuid> = Some(uuid::uuid!($id));
            const MESSAGE_TYPE: $crate::message::DataSetMessageType =
                $crate::message::DataSetMessageType::$mt;
        }
    };
}
pub(crate) use data_set;
