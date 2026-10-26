//! Filter element (MG 7.7, REQ-TOP-019 … REQ-TOP-023).

use core::fmt;

use super::{Resource, TopicError};

element! {
    /// Syslog levels of the Event Filter `Syslog/<Level>` (REQ-TOP-023,
    /// RFC 5424), ordered from the most to the least severe.
    SyslogLevel {
        /// System is unusable.
        Emergency = "Emergency",
        /// Action must be taken immediately.
        Alert = "Alert",
        /// Critical conditions.
        Critical = "Critical",
        /// Error conditions.
        Error = "Error",
        /// Warning conditions.
        Warning = "Warning",
        /// Normal but significant condition.
        Notice = "Notice",
        /// Informational messages.
        Informational = "Informational",
        /// Debug-level messages.
        Debug = "Debug",
    }
}

element! {
    /// Levels of the Event Filter `Status/<Level>` (REQ-TOP-014,
    /// REQ-TOP-023).
    StatusLevel {
        /// Good.
        Good = "Good",
        /// Uncertain.
        Uncertain = "Uncertain",
        /// Bad.
        Bad = "Bad",
    }
}

/// Kind of Filter of a form (`filter_rule` of the matrix).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum FilterKind {
    /// `<Tag>` of Data and Metadata.
    Tag,
    /// `<Context.Name>` of Config.
    ContextName,
    /// `<EventCategory>/<EventLevel>` of Event.
    Event,
    /// `<ResourceType>[/<Tag>]` of PublicationList and SubscriptionList.
    List,
}

impl FilterKind {
    /// Name of the grammar rule: `tag`, `context-name`, `event-filter` or
    /// `list-filter`.
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::Tag => "tag",
            Self::ContextName => "context-name",
            Self::Event => "event-filter",
            Self::List => "list-filter",
        }
    }
}

/// Event Filter `<EventCategory>/<EventLevel>` (REQ-TOP-023).
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub enum EventFilter {
    /// `Syslog/<Level>`.
    Syslog(SyslogLevel),
    /// `Status/<Level>`.
    Status(StatusLevel),
    /// `Generic/<Level>` with a free level.
    Generic(String),
}

impl EventFilter {
    fn values(&self) -> [&str; 2] {
        match self {
            Self::Syslog(l) => ["Syslog", l.as_str()],
            Self::Status(l) => ["Status", l.as_str()],
            Self::Generic(l) => ["Generic", l],
        }
    }

    fn parse(levels: &[&str]) -> Result<Self, TopicError> {
        let [category, level] = levels else {
            return Err(TopicError::Grammar(
                "the Event Filter has exactly two levels".to_owned(),
            ));
        };
        match *category {
            "Syslog" => Ok(Self::Syslog(level.parse()?)),
            "Status" => Ok(Self::Status(level.parse()?)),
            "Generic" => Ok(Self::Generic(decode_level(level)?)),
            other => Err(TopicError::Grammar(format!(
                "{other:?} is not an event category"
            ))),
        }
    }
}

/// PublicationList/SubscriptionList Filter `<ResourceType>[/<Tag>]`, where
/// `<Tag>` is the Filter of the addressed resource (REQ-TOP-022).
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct ListFilter {
    resource: Resource,
    filter: Option<Box<Filter>>,
}

impl ListFilter {
    /// The resource type and the optional Filter of that resource: a Tag
    /// for Data and Metadata, a Context.Name for Config, an Event Filter
    /// for Event, none for the other resources.
    ///
    /// # Errors
    ///
    /// The Filter does not fit the resource or has an invalid value.
    pub fn new(resource: Resource, filter: Option<Filter>) -> Result<Self, TopicError> {
        let lf = Self {
            resource,
            filter: filter.map(Box::new),
        };
        lf.validate()?;
        Ok(lf)
    }

    /// The addressed resource type.
    pub fn resource(&self) -> Resource {
        self.resource
    }

    /// The Filter of the addressed resource.
    pub fn filter(&self) -> Option<&Filter> {
        self.filter.as_deref()
    }

    fn sub_kind(resource: Resource) -> Option<FilterKind> {
        match resource {
            Resource::Data | Resource::Metadata => Some(FilterKind::Tag),
            Resource::Config => Some(FilterKind::ContextName),
            Resource::Event => Some(FilterKind::Event),
            _ => None,
        }
    }

    fn validate(&self) -> Result<(), TopicError> {
        match (&self.filter, Self::sub_kind(self.resource)) {
            (None, _) => Ok(()),
            (Some(f), Some(kind)) if f.kind() == kind => f.validate(),
            (Some(f), _) => Err(TopicError::Grammar(format!(
                "a {} Filter does not fit the resource type {}",
                f.kind().as_str(),
                self.resource
            ))),
        }
    }
}

/// The Filter element, held decoded (REQ-TOP-019 … REQ-TOP-023).
///
/// [`fmt::Display`] writes the wire form: the levels joined by `/`, each
/// with `%`, `+`, `#` and `/` percent-encoded.
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub enum Filter {
    /// `<Tag>` of Data and Metadata.
    Tag(String),
    /// `<Context.Name>` of Config.
    ContextName(String),
    /// Event Filter.
    Event(EventFilter),
    /// PublicationList/SubscriptionList Filter.
    List(ListFilter),
}

impl Filter {
    /// The `<Context.Name>` Filter of a Config DataSet (REQ-RES-CONFIG-015).
    ///
    /// # Errors
    ///
    /// [`TopicError::Grammar`] if the value is empty or contains U+0000
    /// (REQ-TOP-019).
    pub fn context_name(value: &str) -> Result<Self, TopicError> {
        validate_value(value)?;
        Ok(Self::ContextName(value.to_owned()))
    }

    /// Kind of the Filter.
    pub fn kind(&self) -> FilterKind {
        match self {
            Self::Tag(_) => FilterKind::Tag,
            Self::ContextName(_) => FilterKind::ContextName,
            Self::Event(_) => FilterKind::Event,
            Self::List(_) => FilterKind::List,
        }
    }

    /// The decoded levels.
    pub fn values(&self) -> Vec<&str> {
        match self {
            Self::Tag(v) | Self::ContextName(v) => vec![v.as_str()],
            Self::Event(e) => e.values().to_vec(),
            Self::List(l) => {
                let mut out = vec![l.resource.as_str()];
                if let Some(f) = &l.filter {
                    out.extend(f.values());
                }
                out
            }
        }
    }

    /// Checks every free value: non-empty and without U+0000
    /// (REQ-TOP-019), and the Filter of a list filter.
    pub(super) fn validate(&self) -> Result<(), TopicError> {
        match self {
            Self::Tag(v) | Self::ContextName(v) | Self::Event(EventFilter::Generic(v)) => {
                validate_value(v)
            }
            Self::Event(_) => Ok(()),
            Self::List(l) => l.validate(),
        }
    }

    /// Parses the Filter levels of a topic.
    pub(super) fn parse(kind: FilterKind, levels: &[&str]) -> Result<Self, TopicError> {
        let one = |levels: &[&str]| match levels {
            [l] => decode_level(l),
            _ => Err(TopicError::Grammar(format!(
                "the {} Filter has exactly one level",
                kind.as_str()
            ))),
        };
        match kind {
            FilterKind::Tag => one(levels).map(Self::Tag),
            FilterKind::ContextName => one(levels).map(Self::ContextName),
            FilterKind::Event => EventFilter::parse(levels).map(Self::Event),
            FilterKind::List => {
                let (first, rest) = levels.split_first().expect("at least one Filter level");
                let resource: Resource = first.parse()?;
                let filter = match (ListFilter::sub_kind(resource), rest) {
                    (_, []) => None,
                    (Some(k), rest) => Some(Self::parse(k, rest)?),
                    (None, _) => {
                        return Err(TopicError::Grammar(format!(
                            "the resource type {resource} has no Filter"
                        )));
                    }
                };
                Ok(Self::List(ListFilter {
                    resource,
                    filter: filter.map(Box::new),
                }))
            }
        }
    }
}

impl fmt::Display for Filter {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        for (i, v) in self.values().into_iter().enumerate() {
            if i > 0 {
                f.write_str("/")?;
            }
            f.write_str(&encode_level(v))?;
        }
        Ok(())
    }
}

fn validate_value(v: &str) -> Result<(), TopicError> {
    if v.is_empty() {
        return Err(TopicError::Grammar("a Filter level is empty".to_owned()));
    }
    if v.contains('\0') {
        return Err(TopicError::Grammar(
            "a Filter level contains U+0000".to_owned(),
        ));
    }
    Ok(())
}

/// `<Topic encoded String>`: `%`, `+`, `#` and `/` become `%25`, `%2B`,
/// `%23` and `%2F`; nothing else is encoded (REQ-TOP-019).
pub(super) fn encode_level(value: &str) -> String {
    let mut out = String::with_capacity(value.len());
    for c in value.chars() {
        match c {
            '%' => out.push_str("%25"),
            '+' => out.push_str("%2B"),
            '#' => out.push_str("%23"),
            '/' => out.push_str("%2F"),
            c => out.push(c),
        }
    }
    out
}

/// Decodes one Filter level; the only accepted escapes are `%2B`, `%23`,
/// `%2F` and `%25` (rule `filter-level`).
pub(super) fn decode_level(level: &str) -> Result<String, TopicError> {
    let bad = |what: &str| {
        Err(TopicError::Grammar(format!(
            "Filter level {level:?}: {what}"
        )))
    };
    if level.is_empty() {
        return bad("empty");
    }
    let mut out = String::with_capacity(level.len());
    let mut rest = level;
    while let Some(c) = rest.chars().next() {
        rest = &rest[c.len_utf8()..];
        match c {
            '%' => {
                let decoded = match rest.get(..2) {
                    Some("2B") => '+',
                    Some("23") => '#',
                    Some("2F") => '/',
                    Some("25") => '%',
                    _ => return bad("\"%\" not followed by 2B, 23, 2F or 25"),
                };
                rest = &rest[2..];
                out.push(decoded);
            }
            '+' | '#' => return bad("unencoded \"+\" or \"#\""),
            '\0' => return bad("U+0000"),
            c => out.push(c),
        }
    }
    Ok(out)
}

#[cfg(test)]
mod tests {
    use super::*;

    // REQ-TOP-019: exactly these four characters are encoded, upper-case.
    #[test]
    fn level_encoding() {
        assert_eq!(encode_level("a+b#c/d%e f"), "a%2Bb%23c%2Fd%25e f");
        assert_eq!(decode_level("a%2Bb%23c%2Fd%25e f").unwrap(), "a+b#c/d%e f");
        for bad in ["", "a%2b", "a%41", "a%", "a%2", "a+b", "a#", "a\0"] {
            assert!(decode_level(bad).is_err(), "{bad:?}");
        }
        assert_eq!(decode_level("%252B").unwrap(), "%2B");
    }

    // REQ-TOP-022, REQ-TOP-023.
    #[test]
    fn list_and_event_filters() {
        let parse = |l: &[&str]| Filter::parse(FilterKind::List, l);
        assert!(parse(&["MAM"]).is_ok());
        assert!(parse(&["MAM", "x"]).is_err());
        assert!(parse(&["Data", "Temperature"]).is_ok());
        assert!(parse(&["Data", "a", "b"]).is_err());
        assert!(parse(&["Event", "Syslog", "Warning"]).is_ok());
        assert!(parse(&["Event", "Syslog"]).is_err());
        assert!(parse(&["Event", "Syslog", "Warn"]).is_err());
        assert!(parse(&["NewDataSetWriterId"]).is_err());
        let f = parse(&["Config", "a%2Fb"]).unwrap();
        assert_eq!(f.values(), ["Config", "a/b"]);
        assert_eq!(f.to_string(), "Config/a%2Fb");
        assert!(ListFilter::new(Resource::Health, Some(Filter::Tag("x".into()))).is_err());
        assert!(
            ListFilter::new(
                Resource::Event,
                Some(Filter::Event(EventFilter::Status(StatusLevel::Bad)))
            )
            .is_ok()
        );
    }

    #[test]
    fn syslog_order() {
        assert!(SyslogLevel::Emergency < SyslogLevel::Debug);
        assert_eq!(SyslogLevel::ALL.len(), 8);
    }
}
