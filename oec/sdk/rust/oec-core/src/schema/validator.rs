//! Validation against the embedded schemas (feature `strict-validation`).

use std::collections::{BTreeSet, HashMap};
use std::sync::{Arc, Mutex, PoisonError};

use jsonschema::{Draft, Registry};
use serde_json::{Value, json};

use super::embedded::SOURCES;
use super::{BASE_URI, TOLERANT_BASE_URI};
use crate::topic::Topic;

/// The flavor of the schema tree (ADR-0019 §3).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum Flavor {
    /// The source: a sender adds no key beyond the guideline (REQ-MSG-001).
    Strict,
    /// The strict tree without its closures: unknown keys are ignored as by
    /// a tolerant receiver (REQ-MSG-046); explicitly forbidden keys stay
    /// forbidden.
    Tolerant,
}

impl Flavor {
    fn base(self) -> &'static str {
        match self {
            Self::Strict => BASE_URI,
            Self::Tolerant => TOLERANT_BASE_URI,
        }
    }
}

/// The schema a payload is validated against: the entry schema of its topic
/// form, the generic message schema, or a bare MetaData.
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct Entry {
    /// Path below the base URI, with an optional fragment.
    path: String,
    name: String,
}

impl Entry {
    fn topic(rule: &str) -> Self {
        Self {
            path: format!("topics/{rule}.schema.json"),
            name: rule.to_owned(),
        }
    }

    /// The topic-agnostic schema of every OEC message (`message.schema.json`).
    pub fn message() -> Self {
        Self {
            path: "message.schema.json".to_owned(),
            name: "message".to_owned(),
        }
    }

    /// A bare DataSetMetaDataType that is the predefined MetaData of a
    /// resource (MG B2), as the schema gate routes vectors with
    /// `object: DataSetMetaDataType`.
    pub fn resource_metadata() -> Self {
        Self {
            path: "metadata.schema.json#/$defs/ResourceMetaData".to_owned(),
            name: "(bare) ResourceMetaData".to_owned(),
        }
    }

    /// A bare DataSetMetaDataType (REQ-MSG-035) without the resource rules.
    pub fn data_set_metadata() -> Self {
        Self {
            path: "metadata.schema.json#/$defs/DataSetMetaData".to_owned(),
            name: "(bare) DataSetMetaData".to_owned(),
        }
    }

    /// The topic form (e.g. `pub-health`) or the name of the schema.
    pub fn name(&self) -> &str {
        &self.name
    }

    /// The absolute schema reference in the given flavor.
    pub fn reference(&self, flavor: Flavor) -> String {
        format!("{}{}", flavor.base(), self.path)
    }
}

/// The embedded schema tree is unusable. This is a build defect: the crate
/// tests load the same tree.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SetupError(String);

impl core::fmt::Display for SetupError {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        write!(f, "embedded schema tree: {}", self.0)
    }
}

impl std::error::Error for SetupError {}

/// One error of a rejected payload.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SchemaError {
    /// JSON pointer into the payload.
    pub instance_path: String,
    /// Evaluation path through the schemas, including `$ref` traversals.
    pub evaluation_path: String,
    /// Message of the validator.
    pub message: String,
    /// Requirement ids of the deepest `$comment` on the evaluation path.
    pub reqs: Vec<String>,
}

/// A payload the schema rejects, with the violated requirements.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SchemaViolation {
    /// The absolute schema reference that rejected the payload.
    pub entry: String,
    /// The requirement ids of all errors, sorted and unique; the schema gate
    /// compares exactly this set with the `violates` of a vector.
    pub reqs: Vec<String>,
    /// The single errors.
    pub errors: Vec<SchemaError>,
}

impl core::fmt::Display for SchemaViolation {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        write!(
            f,
            "{} rejects the payload ({}):",
            self.entry,
            self.reqs.join(", ")
        )?;
        for e in &self.errors {
            write!(f, " {} {};", e.instance_path, e.message)?;
        }
        Ok(())
    }
}

impl std::error::Error for SchemaViolation {}

/// Validator over the embedded schema tree, strict and tolerant flavor.
///
/// It owns the schema registry and builds the validator of an entry on its
/// first use (the cache belongs to the instance; there is no global state,
/// ADR-0026). Create it once and share it; validation needs `&self` only.
pub struct Validator {
    docs: HashMap<String, Value>,
    registry: Registry<'static>,
    cache: Mutex<HashMap<String, Arc<jsonschema::Validator>>>,
}

impl core::fmt::Debug for Validator {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        f.debug_struct("Validator")
            .field("schemas", &self.docs.len())
            .finish_non_exhaustive()
    }
}

impl Validator {
    /// Loads the embedded tree: every file must be a draft 2020-12 schema
    /// whose `$id` is its path below [`BASE_URI`].
    ///
    /// # Errors
    ///
    /// [`SetupError`] if the embedded tree is inconsistent (a build defect).
    pub fn new() -> Result<Self, SetupError> {
        let mut docs = HashMap::new();
        for (rel, text) in SOURCES {
            let doc: Value =
                serde_json::from_str(text).map_err(|e| SetupError(format!("{rel}: {e}")))?;
            let id = format!("{BASE_URI}{rel}");
            if doc["$id"].as_str() != Some(id.as_str()) {
                return Err(SetupError(format!("{rel}: $id is not {id}")));
            }
            if doc["$schema"].as_str() != Some("https://json-schema.org/draft/2020-12/schema") {
                return Err(SetupError(format!("{rel}: $schema is not draft 2020-12")));
            }
            let mut tol = tolerant(&doc);
            let tol_id = format!("{TOLERANT_BASE_URI}{rel}");
            tol["$id"] = Value::String(tol_id.clone());
            docs.insert(id, doc);
            docs.insert(tol_id, tol);
        }
        let registry = Registry::new()
            .extend(docs.iter().map(|(id, doc)| (id.clone(), doc.clone())))
            .and_then(jsonschema::RegistryBuilder::prepare)
            .map_err(|e| SetupError(e.to_string()))?;
        Ok(Self {
            docs,
            registry,
            cache: Mutex::new(HashMap::new()),
        })
    }

    /// The entry schema of a topic form (e.g. `pub-health`), if the tree has
    /// one.
    pub fn entry(&self, rule: &str) -> Option<Entry> {
        let entry = Entry::topic(rule);
        self.docs
            .contains_key(&entry.reference(Flavor::Strict))
            .then_some(entry)
    }

    /// Any schema of the tree by its path below the base URI, with an
    /// optional fragment, e.g. `platform/broker.schema.json` (REQ-CNT-003)
    /// or `resources/mam.schema.json#/$defs/MAM`; `None` if the tree has no
    /// such file.
    pub fn schema(&self, path: &str) -> Option<Entry> {
        let file = path.split('#').next().unwrap_or(path);
        self.docs
            .contains_key(&format!("{BASE_URI}{file}"))
            .then(|| Entry {
                path: path.to_owned(),
                name: path.to_owned(),
            })
    }

    /// The schema for a payload on `topic`: the entry schema of its topic
    /// form, otherwise the generic [`Entry::message`] (the routing of the
    /// schema gate, ADR-0019 §8).
    pub fn entry_for(&self, topic: &Topic) -> Entry {
        self.entry(&topic.rule()).unwrap_or_else(Entry::message)
    }

    /// Validates a payload on `topic` against [`Self::entry_for`].
    ///
    /// # Errors
    ///
    /// [`SchemaViolation`] with the violated requirements.
    pub fn validate_topic(
        &self,
        topic: &Topic,
        flavor: Flavor,
        payload: &Value,
    ) -> Result<(), SchemaViolation> {
        self.validate(&self.entry_for(topic), flavor, payload)
    }

    /// Validates an instance against an entry.
    ///
    /// # Errors
    ///
    /// [`SchemaViolation`] with the violated requirements: per error the REQ
    /// ids of the deepest `$comment` on its evaluation path, as the schema
    /// gate attributes them.
    ///
    /// # Panics
    ///
    /// If an embedded entry schema does not compile, a build defect the
    /// crate tests exclude.
    pub fn validate(
        &self,
        entry: &Entry,
        flavor: Flavor,
        instance: &Value,
    ) -> Result<(), SchemaViolation> {
        let reference = entry.reference(flavor);
        let validator = self.compiled(&reference);
        if validator.is_valid(instance) {
            return Ok(());
        }
        let mut all = BTreeSet::new();
        let errors: Vec<SchemaError> = validator
            .iter_errors(instance)
            .map(|e| {
                let evaluation_path = e.evaluation_path().as_str().to_owned();
                let reqs = self.reqs(&reference, &evaluation_path);
                all.extend(reqs.iter().cloned());
                SchemaError {
                    instance_path: e.instance_path().as_str().to_owned(),
                    evaluation_path,
                    message: e.to_string(),
                    reqs,
                }
            })
            .collect();
        Err(SchemaViolation {
            entry: reference,
            reqs: all.into_iter().collect(),
            errors,
        })
    }

    fn compiled(&self, reference: &str) -> Arc<jsonschema::Validator> {
        let mut cache = self.cache.lock().unwrap_or_else(PoisonError::into_inner);
        Arc::clone(cache.entry(reference.to_owned()).or_insert_with(|| {
            let v = jsonschema::options()
                .with_draft(Draft::Draft202012)
                .with_registry(&self.registry)
                .build(&json!({ "$ref": reference }))
                .unwrap_or_else(|e| panic!("embedded schema {reference}: {e}"));
            Arc::new(v)
        }))
    }

    /// The REQ ids of the deepest `$comment` with leading REQ ids on the
    /// evaluation path (the attribution of the schema gate). A closure has
    /// its own comment, so an undefined key maps to the REQ of its closure.
    fn reqs(&self, reference: &str, evaluation_path: &str) -> Vec<String> {
        let root = json!({ "$ref": reference });
        let mut cur = Vec::new();
        let mut base = String::new();
        let mut node = &root;
        for token in pointer_tokens(evaluation_path) {
            if token == "$ref" {
                let Some(target) = node.get("$ref").and_then(Value::as_str) else {
                    break;
                };
                let (doc, fragment) = resolve(&base, target);
                let Some(d) = self.docs.get(&doc) else { break };
                base = doc;
                node = d;
                take_comment(node, &mut cur);
                for t in pointer_tokens(&fragment) {
                    match child(node, &t) {
                        Some(n) => node = n,
                        None => return cur,
                    }
                    take_comment(node, &mut cur);
                }
            } else {
                match child(node, &token) {
                    Some(n) => node = n,
                    None => break,
                }
                take_comment(node, &mut cur);
            }
        }
        cur
    }
}

/// Derives the tolerant flavor: a deep copy without the closures
/// (`unevaluatedProperties` is used for closures only, ADR-0019 §3).
fn tolerant(v: &Value) -> Value {
    match v {
        Value::Object(m) => Value::Object(
            m.iter()
                .filter(|(k, _)| k.as_str() != "unevaluatedProperties")
                .map(|(k, e)| (k.clone(), tolerant(e)))
                .collect(),
        ),
        Value::Array(a) => Value::Array(a.iter().map(tolerant).collect()),
        other => other.clone(),
    }
}

fn pointer_tokens(pointer: &str) -> Vec<String> {
    let p = pointer.strip_prefix('/').unwrap_or(pointer);
    if p.is_empty() {
        return Vec::new();
    }
    p.split('/')
        .map(|t| t.replace("~1", "/").replace("~0", "~"))
        .collect()
}

fn child<'a>(node: &'a Value, token: &str) -> Option<&'a Value> {
    match node {
        Value::Object(m) => m.get(token),
        Value::Array(a) => token.parse::<usize>().ok().and_then(|i| a.get(i)),
        _ => None,
    }
}

/// Resolves a `$ref` against the URI of the current document; returns the
/// document URI and the JSON pointer fragment.
fn resolve(base: &str, reference: &str) -> (String, String) {
    let (rel, fragment) = reference.split_once('#').unwrap_or((reference, ""));
    let doc = if rel.is_empty() {
        base.to_owned()
    } else if rel.contains("://") {
        rel.to_owned()
    } else {
        let dir = base.rsplit_once('/').map_or("", |(d, _)| d);
        let (authority, path) = match dir.find("://") {
            Some(i) => {
                let after = i + 3;
                let slash = dir[after..].find('/').map_or(dir.len(), |s| after + s);
                (&dir[..slash], &dir[slash..])
            }
            None => ("", dir),
        };
        let mut segments: Vec<&str> = path.split('/').filter(|s| !s.is_empty()).collect();
        for s in rel.split('/') {
            match s {
                "." | "" => {}
                ".." => {
                    segments.pop();
                }
                s => segments.push(s),
            }
        }
        format!("{authority}/{}", segments.join("/"))
    };
    (doc, fragment.to_owned())
}

fn take_comment(node: &Value, cur: &mut Vec<String>) {
    if let Some(c) = node.get("$comment").and_then(Value::as_str) {
        let ids = leading_reqs(c);
        if !ids.is_empty() {
            *cur = ids;
        }
    }
}

/// The REQ ids at the start of a comment (`REQ-MSG-001 (MG 8): …`).
fn leading_reqs(comment: &str) -> Vec<String> {
    comment
        .split_whitespace()
        .map(|f| f.trim_end_matches([',', ';', ':']))
        .take_while(|f| is_req_id(f))
        .map(str::to_owned)
        .collect()
}

fn is_req_id(s: &str) -> bool {
    let Some(rest) = s.strip_prefix("REQ-") else {
        return false;
    };
    let Some((area, num)) = rest.rsplit_once('-') else {
        return false;
    };
    num.len() == 3
        && num.bytes().all(|b| b.is_ascii_digit())
        && !area.is_empty()
        && area
            .split('-')
            .all(|p| !p.is_empty() && p.bytes().all(|b| b.is_ascii_uppercase()))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn resolves_relative_references() {
        let base = "https://schemas.openindustry4.com/oec/v2/resources/mam.schema.json";
        assert_eq!(
            resolve(base, "../opcua.schema.json#/$defs/Int32"),
            (
                "https://schemas.openindustry4.com/oec/v2/opcua.schema.json".to_owned(),
                "/$defs/Int32".to_owned()
            )
        );
        assert_eq!(resolve(base, "#/$defs/MAM").0, base);
    }

    #[test]
    fn leading_req_ids() {
        assert_eq!(
            leading_reqs("REQ-MSG-001 (MG 8): a sender"),
            vec!["REQ-MSG-001"]
        );
        assert_eq!(
            leading_reqs("REQ-RES-MAM-004, REQ-IL-005: x"),
            vec!["REQ-RES-MAM-004", "REQ-IL-005"]
        );
        assert!(leading_reqs("Lower-case GUID string (MG A2).").is_empty());
    }

    #[test]
    fn embedded_tree_loads_and_every_entry_compiles() {
        let v = Validator::new().unwrap();
        let mut entries = vec![
            Entry::message(),
            Entry::resource_metadata(),
            Entry::data_set_metadata(),
        ];
        for (rel, _) in SOURCES {
            if let Some(rule) = rel
                .strip_prefix("topics/")
                .and_then(|r| r.strip_suffix(".schema.json"))
            {
                entries.push(v.entry(rule).unwrap());
            }
        }
        assert_eq!(entries.len(), 3 + 29);
        for e in &entries {
            for flavor in [Flavor::Strict, Flavor::Tolerant] {
                let _ = v.validate(e, flavor, &json!({}));
            }
        }
    }
}
