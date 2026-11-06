//! Topic model (MG 7, MG 4.2, MG 9): closed element sets, the allowed
//! Method × Resource forms, parsing and building of topics and subscription
//! filters.
//!
//! `Oi4v2/<ServiceType>/<AppId>/<Method>/<Resource>[/<Source>[/<Filter>]]`
//! (REQ-TOP-001). The grammar is `oec/spec/topic/topic.abnf`, the forms are
//! `oec/spec/matrix/method-resource.yaml`.

mod filter;
mod subscription;

use core::fmt;

pub use filter::{
    EventFilter, Filter, FilterKind, ListFilter, StatusLevel, SyslogLevel, decode_level,
};
pub use subscription::{SourceSelector, TopicFilter, TopicFilterBuilder, mqtt_match};

use crate::base64url;
use crate::il::{IdentificationLink, IlError, MAX_ENCODED_LENGTH};

/// The namespace element of this version (REQ-TOP-001, MG 7.1).
pub const NAMESPACE: &str = "Oi4v2";

/// A string is not one of the values of a closed element set.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct UnknownElement {
    /// Name of the element type, e.g. `ServiceType`.
    pub element: &'static str,
    /// The rejected value.
    pub value: String,
}

impl fmt::Display for UnknownElement {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{:?} is not a {}", self.value, self.element)
    }
}

impl std::error::Error for UnknownElement {}

element! {
    /// ServiceType element (REQ-TOP-005, MG 7.2).
    ServiceType {
        /// Registry.
        Registry = "Registry",
        /// OT connector, also every device with its own MQTT client (REQ-TOP-028).
        OTConnector = "OTConnector",
        /// Utility.
        Utility = "Utility",
        /// Persistence.
        Persistence = "Persistence",
        /// Aggregation.
        Aggregation = "Aggregation",
        /// Cloud connector: the bus on one side, a cloud platform on the other (SF-118).
        CloudConnector = "CloudConnector",
        /// IT connector.
        ITConnector = "ITConnector",
    }
}

element! {
    /// Method element (REQ-TOP-011, MG 7.4).
    Method {
        /// Publish.
        Pub = "Pub",
        /// Request a publication.
        Get = "Get",
        /// Change.
        Set = "Set",
        /// Delete.
        Del = "Del",
        /// Call a service.
        Call = "Call",
        /// Reply of a service.
        Reply = "Reply",
    }
}

element! {
    /// Resource element (REQ-TOP-012, MG 7.5.1).
    Resource {
        /// Master Asset Model.
        Mam = "MAM",
        /// Health.
        Health = "Health",
        /// Config.
        Config = "Config",
        /// Data.
        Data = "Data",
        /// Metadata.
        Metadata = "Metadata",
        /// Event.
        Event = "Event",
        /// Profile.
        Profile = "Profile",
        /// PublicationList.
        PublicationList = "PublicationList",
        /// SubscriptionList.
        SubscriptionList = "SubscriptionList",
        /// ReferenceDesignation.
        ReferenceDesignation = "ReferenceDesignation",
    }
}

element! {
    /// Service in the Resource position (REQ-TOP-012, MG 7.5.2, MG 9.2).
    Service {
        /// NewDataSetWriterId (MG 9.2.1).
        NewDataSetWriterId = "NewDataSetWriterId",
    }
}

/// The fifth topic level: a resource or a service.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub enum Subject {
    /// A resource (MG 7.5.1).
    Resource(Resource),
    /// A service (MG 7.5.2).
    Service(Service),
}

impl Subject {
    /// The topic element.
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::Resource(r) => r.as_str(),
            Self::Service(s) => s.as_str(),
        }
    }

    /// `resource` or `service`, the `kind` of the matrix.
    pub const fn kind(self) -> &'static str {
        match self {
            Self::Resource(_) => "resource",
            Self::Service(_) => "service",
        }
    }
}

impl fmt::Display for Subject {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

impl core::str::FromStr for Subject {
    type Err = UnknownElement;

    fn from_str(s: &str) -> Result<Self, UnknownElement> {
        s.parse::<Resource>()
            .map(Self::Resource)
            .or_else(|_| s.parse::<Service>().map(Self::Service))
            .map_err(|_| UnknownElement {
                element: "Resource",
                value: s.to_owned(),
            })
    }
}

serde_str!(Subject);

impl From<Resource> for Subject {
    fn from(r: Resource) -> Self {
        Self::Resource(r)
    }
}

impl From<Service> for Subject {
    fn from(s: Service) -> Self {
        Self::Service(s)
    }
}

/// Presence of Source or Filter in a form.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Presence {
    /// Must not be present.
    Forbidden,
    /// May be present.
    Optional,
    /// Must be present.
    Required,
}

impl Presence {
    /// The matrix spelling: `forbidden`, `optional` or `required`.
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::Forbidden => "forbidden",
            Self::Optional => "optional",
            Self::Required => "required",
        }
    }

    fn admits(self, present: bool) -> bool {
        match self {
            Self::Forbidden => !present,
            Self::Optional => true,
            Self::Required => present,
        }
    }
}

/// An allowed Method × Resource combination (REQ-TOP-013).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Form {
    /// Presence of the Source.
    pub source: Presence,
    /// Presence of the Filter.
    pub filter: Presence,
    /// Kind of the Filter, `None` if the Filter is forbidden.
    pub filter_kind: Option<FilterKind>,
}

impl Form {
    /// The form of a combination, `None` if it is not allowed (REQ-TOP-013,
    /// MG 9.1.x, MG 9.2.1; same content as `matrix/method-resource.yaml`).
    pub const fn of(method: Method, subject: Subject) -> Option<Self> {
        use FilterKind as K;
        use Method::{Call, Del, Get, Pub, Reply, Set};
        use Presence::{Forbidden as F, Optional as O, Required as R};
        use Resource as Res;
        use Subject::{Resource as S, Service as Svc};
        let (source, filter, kind) = match (method, subject) {
            (Pub | Get, S(Res::Mam | Res::Health | Res::Profile | Res::ReferenceDesignation))
            | (Call | Reply, Svc(Service::NewDataSetWriterId)) => (O, F, None),
            (Set | Del, S(Res::ReferenceDesignation)) => (R, F, None),
            (Pub | Get, S(Res::Config)) => (O, O, Some(K::ContextName)),
            (Set, S(Res::Config)) => (R, R, Some(K::ContextName)),
            (Pub | Get, S(Res::Data)) => (O, O, Some(K::Tag)),
            (Set, S(Res::Data)) | (Pub | Get | Set, S(Res::Metadata)) => (R, R, Some(K::Tag)),
            (Pub, S(Res::Event)) => (R, R, Some(K::Event)),
            (Pub | Get, S(Res::PublicationList | Res::SubscriptionList)) => (O, O, Some(K::List)),
            (Set, S(Res::PublicationList | Res::SubscriptionList))
            | (Del, S(Res::SubscriptionList)) => (R, R, Some(K::List)),
            _ => return None,
        };
        Some(Self {
            source,
            filter,
            filter_kind: kind,
        })
    }

    /// Name of the grammar rule, e.g. `pub-mam`.
    pub fn rule(method: Method, subject: Subject) -> String {
        format!("{method}-{subject}").to_lowercase()
    }

    /// Checks the presence of Source and Filter and the Filter kind.
    fn check(self, source: bool, filter: Option<&Filter>) -> Result<(), TopicError> {
        let bad = |d: String| Err(TopicError::Grammar(d));
        if !self.source.admits(source) {
            return bad(format!("the Source is {}", self.source.as_str()));
        }
        if !self.filter.admits(filter.is_some()) {
            return bad(format!("the Filter is {}", self.filter.as_str()));
        }
        if let Some(f) = filter {
            if !source {
                return bad("a Filter requires a Source".to_owned());
            }
            if Some(f.kind()) != self.filter_kind {
                return bad(format!("a {} Filter does not fit here", f.kind().as_str()));
            }
            f.validate()?;
        }
        Ok(())
    }
}

/// Why a topic is invalid. The two kinds match the `fails` field of the
/// topic vectors.
#[derive(Debug, Clone, PartialEq, Eq)]
#[non_exhaustive]
pub enum TopicError {
    /// The topic does not match the grammar or no allowed form (REQ-TOP-001,
    /// REQ-TOP-013, REQ-TOP-019 … REQ-TOP-023), including an AppId or Source
    /// that is no canonical encoded Identification Link (REQ-IL-002 …
    /// REQ-IL-004).
    Grammar(String),
    /// The decoded AppId or Source is no valid Identification Link
    /// (REQ-TOP-009, REQ-TOP-018, MG 4.1).
    IdentificationLink {
        /// `AppId` or `Source`.
        element: &'static str,
        /// The violations.
        error: IlError,
    },
}

impl TopicError {
    /// `grammar` or `identification-link`.
    pub fn code(&self) -> &'static str {
        match self {
            Self::Grammar(_) => "grammar",
            Self::IdentificationLink { .. } => "identification-link",
        }
    }
}

impl fmt::Display for TopicError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Grammar(d) => write!(f, "invalid topic: {d}"),
            Self::IdentificationLink { element, error } => {
                write!(f, "invalid topic: {element}: {error}")
            }
        }
    }
}

impl std::error::Error for TopicError {}

impl From<UnknownElement> for TopicError {
    fn from(e: UnknownElement) -> Self {
        Self::Grammar(e.to_string())
    }
}

/// A recommendation broken by a valid topic.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Warning {
    /// A Filter level contains a control character (C0, DEL or C1).
    ControlCharacter,
    /// A Filter level contains white space (REQ-TOP-020).
    Whitespace,
}

impl Warning {
    /// `control-character` or `whitespace`, the codes of the vectors.
    pub const fn code(self) -> &'static str {
        match self {
            Self::ControlCharacter => "control-character",
            Self::Whitespace => "whitespace",
        }
    }
}

/// Checks that a topic level is the canonical base64url encoding of 1 … 255
/// octets (rule `encoded-il`, REQ-IL-002 … REQ-IL-004).
fn check_encoded(element: &str, level: &str) -> Result<(), TopicError> {
    if level.len() > MAX_ENCODED_LENGTH {
        return Err(TopicError::Grammar(format!(
            "{element}: more than {MAX_ENCODED_LENGTH} characters"
        )));
    }
    base64url::decode(level).map(|_| ()).map_err(|e| {
        TopicError::Grammar(format!(
            "{element}: not a canonical unpadded base64url string: {e}"
        ))
    })
}

/// A valid topic (REQ-TOP-001, REQ-TOP-013).
///
/// [`Topic::parse`] and [`fmt::Display`] are exact inverses: the AppId and
/// the Source are canonical encodings and the Filter has exactly one wire
/// form.
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct Topic {
    service_type: ServiceType,
    app_id: IdentificationLink,
    method: Method,
    subject: Subject,
    source: Option<IdentificationLink>,
    filter: Option<Filter>,
}

impl Topic {
    /// Builds a topic and checks it against the allowed forms.
    ///
    /// # Errors
    ///
    /// [`TopicError::Grammar`] if the combination is not allowed, Source or
    /// Filter are missing or forbidden, the Filter kind does not fit or a
    /// Filter value is empty or contains U+0000.
    pub fn new(
        service_type: ServiceType,
        app_id: IdentificationLink,
        method: Method,
        subject: impl Into<Subject>,
        source: Option<IdentificationLink>,
        filter: Option<Filter>,
    ) -> Result<Self, TopicError> {
        let subject = subject.into();
        let form = Form::of(method, subject).ok_or_else(|| {
            TopicError::Grammar(format!("{method}/{subject} is not an allowed combination"))
        })?;
        form.check(source.is_some(), filter.as_ref())?;
        Ok(Self {
            service_type,
            app_id,
            method,
            subject,
            source,
            filter,
        })
    }

    /// Parses and validates a topic name.
    ///
    /// All structural rules are checked first ([`TopicError::Grammar`]),
    /// then the decoded AppId and Source as Identification Links
    /// ([`TopicError::IdentificationLink`]).
    ///
    /// # Errors
    ///
    /// The first rule broken.
    pub fn parse(topic: &str) -> Result<Self, TopicError> {
        let levels: Vec<&str> = topic.split('/').collect();
        if levels.len() < 5 || levels[0] != NAMESPACE {
            return Err(TopicError::Grammar(
                "does not match the topic structure of MG 7".to_owned(),
            ));
        }
        let service_type: ServiceType = levels[1].parse()?;
        check_encoded("AppId", levels[2])?;
        let method: Method = levels[3].parse()?;
        let subject: Subject = levels[4].parse()?;
        let form = Form::of(method, subject).ok_or_else(|| {
            TopicError::Grammar(format!(
                "matches the generic structure, but {method}/{subject} is not an allowed form"
            ))
        })?;
        let source = levels.get(5).copied();
        if let Some(s) = source {
            check_encoded("Source", s)?;
        }
        let filter = match (levels.get(6..), form.filter_kind) {
            (Some([]) | None, _) => None,
            (Some(_), None) => {
                return Err(TopicError::Grammar("the Filter is forbidden".to_owned()));
            }
            (Some(l), Some(kind)) => Some(Filter::parse(kind, l)?),
        };
        form.check(source.is_some(), filter.as_ref())?;
        let app_id = IdentificationLink::decode(levels[2]).map_err(|error| {
            TopicError::IdentificationLink {
                element: "AppId",
                error,
            }
        })?;
        let source = source
            .map(IdentificationLink::decode)
            .transpose()
            .map_err(|error| TopicError::IdentificationLink {
                element: "Source",
                error,
            })?;
        Ok(Self {
            service_type,
            app_id,
            method,
            subject,
            source,
            filter,
        })
    }

    /// ServiceType element.
    pub fn service_type(&self) -> ServiceType {
        self.service_type
    }

    /// Identification Link of the AppId element.
    pub fn app_id(&self) -> &IdentificationLink {
        &self.app_id
    }

    /// Method element.
    pub fn method(&self) -> Method {
        self.method
    }

    /// Resource or service element.
    pub fn subject(&self) -> Subject {
        self.subject
    }

    /// Identification Link of the Source element.
    pub fn source(&self) -> Option<&IdentificationLink> {
        self.source.as_ref()
    }

    /// The Filter.
    pub fn filter(&self) -> Option<&Filter> {
        self.filter.as_ref()
    }

    /// The form of this topic.
    ///
    /// # Panics
    ///
    /// Never: construction and parsing admit allowed forms only.
    pub fn form(&self) -> Form {
        Form::of(self.method, self.subject).expect("a Topic is always of an allowed form")
    }

    /// Name of the grammar rule, e.g. `pub-mam`.
    pub fn rule(&self) -> String {
        Form::rule(self.method, self.subject)
    }

    /// The Reply topic of a Call: equal except for the Method (REQ-TOP-016).
    pub fn reply(&self) -> Option<Self> {
        (self.method == Method::Call).then(|| Self {
            method: Method::Reply,
            ..self.clone()
        })
    }

    /// Recommendations broken by the Filter values: control characters,
    /// then white space (REQ-TOP-020).
    pub fn warnings(&self) -> Vec<Warning> {
        let values = self.filter.as_ref().map(Filter::values).unwrap_or_default();
        let chars = || values.iter().flat_map(|v| v.chars());
        let mut out = Vec::new();
        if chars().any(|c| c <= '\u{1F}' || ('\u{7F}'..='\u{9F}').contains(&c)) {
            out.push(Warning::ControlCharacter);
        }
        if chars().any(char::is_whitespace) {
            out.push(Warning::Whitespace);
        }
        out
    }
}

impl fmt::Display for Topic {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "{NAMESPACE}/{}/{}/{}/{}",
            self.service_type,
            self.app_id.encode(),
            self.method,
            self.subject
        )?;
        if let Some(s) = &self.source {
            write!(f, "/{}", s.encode())?;
        }
        if let Some(filter) = &self.filter {
            write!(f, "/{filter}")?;
        }
        Ok(())
    }
}

impl core::str::FromStr for Topic {
    type Err = TopicError;

    fn from_str(s: &str) -> Result<Self, TopicError> {
        Self::parse(s)
    }
}

serde_str!(Topic);

#[cfg(test)]
mod tests {
    use super::*;

    const APP: &str = "provider.com/FieldDataService/FDS-001/1200-0345";
    const DEV: &str = "company.id/A87BAD34H";

    fn il(s: &str) -> IdentificationLink {
        IdentificationLink::parse(s).unwrap()
    }

    // REQ-TOP-005, REQ-TOP-011, REQ-TOP-012: closed sets, case-sensitive.
    #[test]
    fn closed_sets() {
        assert_eq!(ServiceType::ALL.len(), 7);
        // SF-118, GL-ADR-023: OOCConnector renamed to CloudConnector.
        assert_eq!(
            "CloudConnector".parse::<ServiceType>(),
            Ok(ServiceType::CloudConnector)
        );
        assert!("OOCConnector".parse::<ServiceType>().is_err());
        assert_eq!(Method::ALL.len(), 6);
        assert_eq!(Resource::ALL.len(), 10);
        assert_eq!(
            "MAM".parse::<Subject>(),
            Ok(Subject::Resource(Resource::Mam))
        );
        assert_eq!(
            "NewDataSetWriterId".parse::<Subject>(),
            Ok(Subject::Service(Service::NewDataSetWriterId))
        );
        assert!("mam".parse::<Subject>().is_err());
        assert!("pub".parse::<Method>().is_err());
    }

    // REQ-TOP-013: 29 allowed combinations.
    #[test]
    fn allowed_forms() {
        let subjects = Resource::ALL
            .iter()
            .map(|&r| Subject::from(r))
            .chain([Service::NewDataSetWriterId.into()]);
        let n = subjects
            .flat_map(|s| Method::ALL.iter().filter_map(move |&m| Form::of(m, s)))
            .count();
        assert_eq!(n, 29);
        assert_eq!(
            Form::rule(Method::Call, Service::NewDataSetWriterId.into()),
            "call-newdatasetwriterid"
        );
    }

    #[test]
    fn build_and_parse() {
        let t = Topic::new(
            ServiceType::OTConnector,
            il(APP),
            Method::Set,
            Resource::Config,
            Some(il(DEV)),
            Some(Filter::ContextName("Network settings".to_owned())),
        )
        .unwrap();
        let s = t.to_string();
        assert!(
            s.ends_with("/Set/Config/Y29tcGFueS5pZC9BODdCQUQzNEg/Network settings"),
            "{s}"
        );
        let p = Topic::parse(&s).unwrap();
        assert_eq!(p, t);
        assert_eq!(p.warnings(), [Warning::Whitespace]);
        assert_eq!(p.rule(), "set-config");
    }

    // REQ-TOP-013, REQ-TOP-021: Source/Filter presence.
    #[test]
    fn rejects_wrong_presence() {
        let new = |m, r: Resource, src: Option<&str>, f: Option<Filter>| {
            Topic::new(ServiceType::Utility, il(APP), m, r, src.map(il), f)
        };
        assert!(new(Method::Get, Resource::Event, Some(DEV), None).is_err());
        assert!(new(Method::Pub, Resource::Metadata, Some(DEV), None).is_err());
        assert!(
            new(
                Method::Pub,
                Resource::Mam,
                Some(DEV),
                Some(Filter::Tag("x".into()))
            )
            .is_err()
        );
        assert!(
            new(
                Method::Pub,
                Resource::Data,
                None,
                Some(Filter::Tag("x".into()))
            )
            .is_err()
        );
        assert!(
            new(
                Method::Pub,
                Resource::Data,
                Some(DEV),
                Some(Filter::ContextName("x".into()))
            )
            .is_err()
        );
        assert!(
            new(
                Method::Pub,
                Resource::Data,
                Some(DEV),
                Some(Filter::Tag(String::new()))
            )
            .is_err()
        );
        assert!(
            new(
                Method::Pub,
                Resource::Data,
                Some(DEV),
                Some(Filter::Tag("a\0b".into()))
            )
            .is_err()
        );
        assert!(
            new(
                Method::Pub,
                Resource::Data,
                Some(DEV),
                Some(Filter::Tag("a/b+c".into()))
            )
            .is_ok()
        );
    }

    // REQ-TOP-016.
    #[test]
    fn reply_of_call() {
        let call = Topic::new(
            ServiceType::Registry,
            il(APP),
            Method::Call,
            Service::NewDataSetWriterId,
            None,
            None,
        )
        .unwrap();
        let reply = call.reply().unwrap();
        assert_eq!(
            reply.to_string(),
            call.to_string().replace("/Call/", "/Reply/")
        );
        assert!(reply.reply().is_none());
    }

    // Structure first, Identification Link second.
    #[test]
    fn failure_classification() {
        let bad_app = base64url::encode(b"Company.id/x");
        let good = il(DEV).encode();
        let t = format!("Oi4v2/Utility/{bad_app}/Pub/Health/{good}");
        assert_eq!(Topic::parse(&t).unwrap_err().code(), "identification-link");
        let t = format!("Oi4v2/Utility/{bad_app}/Pub/Health/{good}=");
        assert_eq!(Topic::parse(&t).unwrap_err().code(), "grammar");
        let t = format!("Oi4v2/Utility/{good}/Pub/Data/{good}/");
        assert_eq!(Topic::parse(&t).unwrap_err().code(), "grammar");
    }
}
