//! Subscription filters with MQTT wildcards (MG 7, MQTT 3.1.1 §4.7).

use core::fmt;

use super::{Filter, Form, Method, NAMESPACE, Presence, ServiceType, Subject, Topic, TopicError};
use crate::il::IdentificationLink;

/// Which Source levels a [`TopicFilter`] matches.
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub enum SourceSelector {
    /// With and without Source and Filter: `<Resource>/#`.
    #[default]
    All,
    /// Only topics without Source: `<Resource>`.
    None,
    /// Every Source: `<Resource>/+`.
    Any,
    /// One Source.
    Exact(IdentificationLink),
}

/// Builder of a [`TopicFilter`]; the defaults are `+` for ServiceType and
/// AppId and [`SourceSelector::All`].
#[derive(Debug, Clone)]
pub struct TopicFilterBuilder {
    service_type: Option<ServiceType>,
    app_id: Option<IdentificationLink>,
    method: Method,
    subject: Subject,
    source: SourceSelector,
    filter: Option<Filter>,
}

impl TopicFilterBuilder {
    /// Only this ServiceType.
    #[must_use]
    pub fn service_type(mut self, service_type: ServiceType) -> Self {
        self.service_type = Some(service_type);
        self
    }

    /// Only this application (REQ-TOP-009).
    #[must_use]
    pub fn app_id(mut self, app_id: IdentificationLink) -> Self {
        self.app_id = Some(app_id);
        self
    }

    /// Only topics without Source.
    #[must_use]
    pub fn without_source(mut self) -> Self {
        self.source = SourceSelector::None;
        self
    }

    /// Every Source (`+`).
    #[must_use]
    pub fn any_source(mut self) -> Self {
        self.source = SourceSelector::Any;
        self
    }

    /// Only this Source (REQ-TOP-018).
    #[must_use]
    pub fn source(mut self, source: IdentificationLink) -> Self {
        self.source = SourceSelector::Exact(source);
        self
    }

    /// Only this Filter; needs [`Self::any_source`] or [`Self::source`].
    #[must_use]
    pub fn filter(mut self, filter: Filter) -> Self {
        self.filter = Some(filter);
        self
    }

    /// Checks the selection against the form of the combination and
    /// renders the topic filter.
    ///
    /// # Errors
    ///
    /// The combination is not allowed, the selection can never match a
    /// valid topic (e.g. without Source where the Source is required) or
    /// the Filter does not fit.
    pub fn build(self) -> Result<TopicFilter, TopicError> {
        let (method, subject) = (self.method, self.subject);
        let form = Form::of(method, subject).ok_or_else(|| {
            TopicError::Grammar(format!("{method}/{subject} is not an allowed combination"))
        })?;
        let mut s = format!(
            "{NAMESPACE}/{}/{}/{method}/{subject}",
            self.service_type.map_or("+", ServiceType::as_str),
            self.app_id
                .as_ref()
                .map_or_else(|| "+".to_owned(), IdentificationLink::encode),
        );
        let source = match &self.source {
            SourceSelector::All => {
                if self.filter.is_some() {
                    return Err(TopicError::Grammar(
                        "a Filter needs a Source selection".to_owned(),
                    ));
                }
                s.push_str("/#");
                return Ok(TopicFilter(s));
            }
            SourceSelector::None => {
                form.check(false, self.filter.as_ref())?;
                return Ok(TopicFilter(s));
            }
            SourceSelector::Any => "+".to_owned(),
            SourceSelector::Exact(il) => il.encode(),
        };
        if form.source == Presence::Forbidden {
            return Err(TopicError::Grammar("the Source is forbidden".to_owned()));
        }
        s.push('/');
        s.push_str(&source);
        match &self.filter {
            Some(f) => {
                form.check(true, Some(f))?;
                s.push('/');
                s.push_str(&f.to_string());
            }
            None if form.filter != Presence::Forbidden => s.push_str("/#"),
            None => {}
        }
        Ok(TopicFilter(s))
    }
}

/// A validated MQTT topic filter for OEC topics.
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct TopicFilter(String);

impl TopicFilter {
    /// A builder for the topics of one Method × Resource combination,
    /// default `Oi4v2/+/+/<Method>/<Resource>/#`.
    pub fn builder(method: Method, subject: impl Into<Subject>) -> TopicFilterBuilder {
        TopicFilterBuilder {
            service_type: None,
            app_id: None,
            method,
            subject: subject.into(),
            source: SourceSelector::All,
            filter: None,
        }
    }

    /// The filter string for SUBSCRIBE.
    pub fn as_str(&self) -> &str {
        &self.0
    }

    /// Whether a topic name matches (MQTT 3.1.1 §4.7).
    pub fn matches(&self, topic: &str) -> bool {
        mqtt_match(&self.0, topic)
    }

    /// Whether a topic matches.
    pub fn matches_topic(&self, topic: &Topic) -> bool {
        self.matches(&topic.to_string())
    }
}

impl fmt::Display for TopicFilter {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

/// Matches a topic name against a topic filter with the wildcards `+` (one
/// level) and `#` (this and all lower levels, also the parent); a wildcard
/// in the first level does not match a topic starting with `$`
/// (MQTT 3.1.1 §4.7.1, §4.7.2).
pub fn mqtt_match(filter: &str, topic: &str) -> bool {
    if topic.starts_with('$') && (filter.starts_with('+') || filter.starts_with('#')) {
        return false;
    }
    let mut f = filter.split('/');
    let mut t = topic.split('/');
    loop {
        match (f.next(), t.next()) {
            (Some("#"), _) | (None, None) => return true,
            (Some("+"), Some(_)) => {}
            (Some(a), Some(b)) if a == b => {}
            _ => return false,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::super::{EventFilter, Resource, Service, SyslogLevel};
    use super::*;

    fn il(s: &str) -> IdentificationLink {
        IdentificationLink::parse(s).unwrap()
    }

    #[test]
    fn mqtt_wildcards() {
        assert!(mqtt_match("a/#", "a"));
        assert!(mqtt_match("a/#", "a/b/c"));
        assert!(mqtt_match("a/+/c", "a/b/c"));
        assert!(mqtt_match("a/+/c", "a//c"));
        assert!(!mqtt_match("a/+", "a/b/c"));
        assert!(!mqtt_match("a/b", "a"));
        assert!(mqtt_match("#", "a"));
        assert!(!mqtt_match("#", "$SYS/x"));
        assert!(!mqtt_match("+/x", "$SYS/x"));
    }

    #[test]
    fn renders_selections() {
        let b = || TopicFilter::builder(Method::Pub, Resource::Event);
        assert_eq!(b().build().unwrap().as_str(), "Oi4v2/+/+/Pub/Event/#");
        assert!(b().without_source().build().is_err());
        assert_eq!(
            b().any_source().build().unwrap().as_str(),
            "Oi4v2/+/+/Pub/Event/+/#"
        );
        let f = Filter::Event(EventFilter::Syslog(SyslogLevel::Error));
        assert_eq!(
            b().any_source().filter(f.clone()).build().unwrap().as_str(),
            "Oi4v2/+/+/Pub/Event/+/Syslog/Error"
        );
        assert!(b().filter(f).build().is_err());

        let app = il("provider.com/FieldDataService/FDS-001/1200-0345");
        let mam = TopicFilter::builder(Method::Get, Resource::Mam)
            .service_type(ServiceType::OTConnector)
            .app_id(app.clone())
            .any_source()
            .build()
            .unwrap();
        assert_eq!(
            mam.as_str(),
            format!("Oi4v2/OTConnector/{}/Get/MAM/+", app.encode())
        );
        let t = Topic::new(
            ServiceType::OTConnector,
            app.clone(),
            Method::Get,
            Resource::Mam,
            Some(app.clone()),
            None,
        )
        .unwrap();
        assert!(mam.matches_topic(&t));
        let none = TopicFilter::builder(Method::Call, Service::NewDataSetWriterId)
            .without_source()
            .build()
            .unwrap();
        assert_eq!(none.as_str(), "Oi4v2/+/+/Call/NewDataSetWriterId");
        assert!(
            TopicFilter::builder(Method::Get, Resource::Event)
                .build()
                .is_err()
        );
        assert!(
            TopicFilter::builder(Method::Pub, Resource::Mam)
                .any_source()
                .filter(Filter::Tag("x".into()))
                .build()
                .is_err()
        );
    }
}
