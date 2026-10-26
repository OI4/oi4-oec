//! Action messages `ua-action-request` and `ua-action-response` (MG 8.4)
//! and the common service NewDataSetWriterId (MG 9.2.1).
//!
//! Action messages use the CompactEncoding with fixed content masks
//! (REQ-ACT-003): no DataSetClassId or ReplyTo in the NetworkMessage, no
//! SequenceNumber, MessageType, PublisherId, DataSetWriterName,
//! WriterGroupName or MinorVersion in an item, ActionState as number and
//! the Status without Symbol.

use core::num::NonZeroU32;

use serde::de::{DeserializeOwned, Error as _};
use serde::{Deserialize, Deserializer, Serialize, Serializer};
use serde_json::{Map, Number, Value};

use super::DecodeError;
use super::envelope::{DataSetWriterId, MessageId, PublisherId};
use super::value::{ByteString, ConfigurationVersion, DateTime};
use crate::topic::{Method, Resource, Topic};

/// Request or response fields of an Action (REQ-ACT-013).
pub trait ActionPayload: Serialize + DeserializeOwned {
    /// DataSetWriterId and ActionTargetId of a common service
    /// (REQ-ACT-019); `None` if the Action defines them.
    const COMMON: Option<(u16, u16)> = None;
}

/// TimeoutHint: Duration in milliseconds, greater than 0 (REQ-ACT-002,
/// REQ-ACT-008, REQ-ACT-009).
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize)]
#[serde(transparent)]
pub struct TimeoutHint(Number);

impl TimeoutHint {
    /// A whole number of milliseconds.
    pub fn from_millis(ms: core::num::NonZeroU32) -> Self {
        Self(Number::from(ms.get()))
    }

    /// The milliseconds.
    pub fn as_millis(&self) -> f64 {
        self.0.as_f64().unwrap_or(0.0)
    }
}

impl TryFrom<f64> for TimeoutHint {
    type Error = DecodeError;

    fn try_from(ms: f64) -> Result<Self, DecodeError> {
        Number::from_f64(ms)
            .filter(|_| ms > 0.0)
            .map(Self)
            .ok_or_else(|| DecodeError::new(format!("TimeoutHint {ms} is not greater than 0")))
    }
}

impl<'de> Deserialize<'de> for TimeoutHint {
    fn deserialize<D: Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        let n = Number::deserialize(d)?;
        if n.as_f64().is_some_and(|ms| ms > 0.0) {
            Ok(Self(n))
        } else {
            Err(D::Error::custom(format!(
                "TimeoutHint {n} is not greater than 0 (REQ-ACT-002)"
            )))
        }
    }
}

/// ActionState of an item (REQ-ACT-010, REQ-ACT-012), a number in the
/// CompactEncoding.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum ActionState {
    /// 1: executing; the only state of a request, an intermediate state of a
    /// response.
    Executing,
    /// 2: done, the final response.
    Done,
}

impl Serialize for ActionState {
    fn serialize<S: Serializer>(&self, s: S) -> Result<S::Ok, S::Error> {
        s.serialize_u8(match self {
            Self::Executing => 1,
            Self::Done => 2,
        })
    }
}

impl<'de> Deserialize<'de> for ActionState {
    fn deserialize<D: Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        match u64::deserialize(d)? {
            1 => Ok(Self::Executing),
            2 => Ok(Self::Done),
            n => Err(D::Error::custom(format!(
                "ActionState {n} is not 1 or 2 (REQ-ACT-012)"
            ))),
        }
    }
}

/// An ActionRequest (REQ-ACT-010); its ActionState is always 1.
#[derive(Debug, Clone, PartialEq)]
pub struct ActionRequest<P> {
    data_set_writer_id: u16,
    action_target_id: u16,
    request_id: u16,
    meta_data_version: Option<ConfigurationVersion>,
    timestamp: Option<DateTime>,
    payload: P,
}

impl<P: ActionPayload> ActionRequest<P> {
    /// A request of a common service with its reserved DataSetWriterId and
    /// ActionTargetId (REQ-ACT-019). `request_id` is unique among the
    /// pending requests of the Requestor (REQ-ACT-011).
    ///
    /// # Panics
    ///
    /// If `P` is not a common service.
    pub fn common(request_id: u16, payload: P) -> Self {
        let (dsw, target) = P::COMMON.expect("a common service");
        Self::new(dsw, target, request_id, payload)
    }

    /// A request with explicit DataSetWriterId and ActionTargetId.
    pub fn new(
        data_set_writer_id: u16,
        action_target_id: u16,
        request_id: u16,
        payload: P,
    ) -> Self {
        Self {
            data_set_writer_id,
            action_target_id,
            request_id,
            meta_data_version: None,
            timestamp: None,
            payload,
        }
    }

    /// Sets the MetaDataVersion of the ActionMetaData.
    #[must_use]
    pub fn with_meta_data_version(mut self, v: ConfigurationVersion) -> Self {
        self.meta_data_version = Some(v);
        self
    }

    /// Sets the Timestamp.
    #[must_use]
    pub fn with_timestamp(mut self, t: DateTime) -> Self {
        self.timestamp = Some(t);
        self
    }

    /// DataSetWriterId.
    pub fn data_set_writer_id(&self) -> u16 {
        self.data_set_writer_id
    }

    /// ActionTargetId.
    pub fn action_target_id(&self) -> u16 {
        self.action_target_id
    }

    /// RequestId (REQ-ACT-011).
    pub fn request_id(&self) -> u16 {
        self.request_id
    }

    /// MetaDataVersion.
    pub fn meta_data_version(&self) -> Option<ConfigurationVersion> {
        self.meta_data_version
    }

    /// Timestamp.
    pub fn timestamp(&self) -> Option<&DateTime> {
        self.timestamp.as_ref()
    }

    /// The request fields.
    pub fn payload(&self) -> &P {
        &self.payload
    }

    /// The final, successful response: same DataSetWriterId, ActionTargetId
    /// and RequestId, Status Good, ActionState 2 (REQ-ACT-012, REQ-ACT-014).
    pub fn done<R: ActionPayload>(&self, payload: R) -> ActionResponse<R> {
        self.response(None, ActionState::Done, Some(payload))
    }

    /// The final response with a Bad StatusCode and an empty Payload
    /// (REQ-ACT-012, REQ-ACT-014), e.g. BadInvalidArgument 0x80AB0000.
    pub fn failed<R: ActionPayload>(&self, code: NonZeroU32) -> ActionResponse<R> {
        self.response(Some(code), ActionState::Done, None)
    }

    fn response<R>(
        &self,
        status: Option<NonZeroU32>,
        state: ActionState,
        payload: Option<R>,
    ) -> ActionResponse<R> {
        ActionResponse {
            data_set_writer_id: self.data_set_writer_id,
            action_target_id: self.action_target_id,
            request_id: self.request_id,
            meta_data_version: None,
            timestamp: None,
            status,
            action_state: state,
            payload,
        }
    }
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
struct RequestWire<P> {
    data_set_writer_id: u16,
    action_target_id: u16,
    request_id: u16,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    meta_data_version: Option<ConfigurationVersion>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    timestamp: Option<DateTime>,
    payload: P,
    action_state: ActionState,
}

impl<P: ActionPayload> Serialize for ActionRequest<P> {
    fn serialize<S: Serializer>(&self, s: S) -> Result<S::Ok, S::Error> {
        RequestWire {
            data_set_writer_id: self.data_set_writer_id,
            action_target_id: self.action_target_id,
            request_id: self.request_id,
            meta_data_version: self.meta_data_version,
            timestamp: self.timestamp.clone(),
            payload: &self.payload,
            action_state: ActionState::Executing,
        }
        .serialize(s)
    }
}

fn check_common<P: ActionPayload>(dsw: u16, target: u16) -> Result<(), String> {
    match P::COMMON {
        Some((d, t)) if d != dsw || t != target => Err(format!(
            "the common service uses DataSetWriterId {d} and ActionTargetId {t} (REQ-ACT-019)"
        )),
        _ => Ok(()),
    }
}

impl<'de, P: ActionPayload> Deserialize<'de> for ActionRequest<P> {
    fn deserialize<D: Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        let w = RequestWire::<P>::deserialize(d)?;
        if w.action_state != ActionState::Executing {
            return Err(D::Error::custom(
                "the ActionState of a request is 1 (REQ-ACT-010)",
            ));
        }
        check_common::<P>(w.data_set_writer_id, w.action_target_id).map_err(D::Error::custom)?;
        Ok(Self {
            data_set_writer_id: w.data_set_writer_id,
            action_target_id: w.action_target_id,
            request_id: w.request_id,
            meta_data_version: w.meta_data_version,
            timestamp: w.timestamp,
            payload: w.payload,
        })
    }
}

/// An ActionResponse (REQ-ACT-012): Status `{}` for Good, otherwise
/// `{"Code": n}` (REQ-ACT-014); a Bad response has an empty Payload.
#[derive(Debug, Clone, PartialEq)]
pub struct ActionResponse<P> {
    data_set_writer_id: u16,
    action_target_id: u16,
    request_id: u16,
    meta_data_version: Option<ConfigurationVersion>,
    timestamp: Option<DateTime>,
    status: Option<NonZeroU32>,
    action_state: ActionState,
    payload: Option<P>,
}

/// Whether a StatusCode is Bad: the most significant bit is set
/// (REQ-ACT-012, Part 4 7.39).
pub fn is_bad(code: u32) -> bool {
    code & 0x8000_0000 != 0
}

impl<P: ActionPayload> ActionResponse<P> {
    /// Marks the response as intermediate (ActionState 1).
    #[must_use]
    pub fn executing(mut self) -> Self {
        self.action_state = ActionState::Executing;
        self
    }

    /// Sets an Uncertain StatusCode of a response with Payload.
    ///
    /// # Errors
    ///
    /// [`DecodeError`] if the code is Bad: use [`ActionRequest::failed`].
    pub fn with_status(mut self, code: NonZeroU32) -> Result<Self, DecodeError> {
        if is_bad(code.get()) {
            return Err(DecodeError::new(
                "a Bad response has an empty Payload (REQ-ACT-012)",
            ));
        }
        self.status = Some(code);
        Ok(self)
    }

    /// Sets the Timestamp.
    #[must_use]
    pub fn with_timestamp(mut self, t: DateTime) -> Self {
        self.timestamp = Some(t);
        self
    }

    /// DataSetWriterId of the request.
    pub fn data_set_writer_id(&self) -> u16 {
        self.data_set_writer_id
    }

    /// ActionTargetId of the request.
    pub fn action_target_id(&self) -> u16 {
        self.action_target_id
    }

    /// RequestId of the request (REQ-ACT-011).
    pub fn request_id(&self) -> u16 {
        self.request_id
    }

    /// MetaDataVersion.
    pub fn meta_data_version(&self) -> Option<ConfigurationVersion> {
        self.meta_data_version
    }

    /// Timestamp.
    pub fn timestamp(&self) -> Option<&DateTime> {
        self.timestamp.as_ref()
    }

    /// The StatusCode; `None` is Good (REQ-ACT-014).
    pub fn status(&self) -> Option<NonZeroU32> {
        self.status
    }

    /// ActionState.
    pub fn action_state(&self) -> ActionState {
        self.action_state
    }

    /// The response fields; `None` for a Bad response (REQ-ACT-012).
    pub fn payload(&self) -> Option<&P> {
        self.payload.as_ref()
    }

    /// Whether the response answers `request` (same DataSetWriterId,
    /// ActionTargetId and RequestId).
    pub fn answers<Q>(&self, request: &ActionRequest<Q>) -> bool {
        self.data_set_writer_id == request.data_set_writer_id
            && self.action_target_id == request.action_target_id
            && self.request_id == request.request_id
    }
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
struct ActionStatusWire {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    code: Option<NonZeroU32>,
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
struct ResponseWire<P> {
    data_set_writer_id: u16,
    action_target_id: u16,
    request_id: u16,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    meta_data_version: Option<ConfigurationVersion>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    timestamp: Option<DateTime>,
    status: ActionStatusWire,
    action_state: ActionState,
    payload: P,
}

impl<P: ActionPayload> Serialize for ActionResponse<P> {
    fn serialize<S: Serializer>(&self, s: S) -> Result<S::Ok, S::Error> {
        let payload = match &self.payload {
            Some(p) => serde_json::to_value(p).map_err(serde::ser::Error::custom)?,
            None => Value::Object(Map::new()),
        };
        ResponseWire {
            data_set_writer_id: self.data_set_writer_id,
            action_target_id: self.action_target_id,
            request_id: self.request_id,
            meta_data_version: self.meta_data_version,
            timestamp: self.timestamp.clone(),
            status: ActionStatusWire { code: self.status },
            action_state: self.action_state,
            payload,
        }
        .serialize(s)
    }
}

impl<'de, P: ActionPayload> Deserialize<'de> for ActionResponse<P> {
    fn deserialize<D: Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        let w = ResponseWire::<Value>::deserialize(d)?;
        check_common::<P>(w.data_set_writer_id, w.action_target_id).map_err(D::Error::custom)?;
        let bad = w.status.code.is_some_and(|c| is_bad(c.get()));
        let payload = match w.payload {
            Value::Object(m) if bad && m.is_empty() => None,
            _ if bad => {
                return Err(D::Error::custom(
                    "a Bad response has an empty Payload (REQ-ACT-012)",
                ));
            }
            v => Some(serde_json::from_value(v).map_err(D::Error::custom)?),
        };
        Ok(Self {
            data_set_writer_id: w.data_set_writer_id,
            action_target_id: w.action_target_id,
            request_id: w.request_id,
            meta_data_version: w.meta_data_version,
            timestamp: w.timestamp,
            status: w.status.code,
            action_state: w.action_state,
            payload,
        })
    }
}

/// A `ua-action-request` (REQ-ACT-001, REQ-ACT-002).
#[derive(Debug, Clone, PartialEq)]
pub struct ActionRequestMessage<P> {
    message_id: MessageId,
    publisher_id: PublisherId,
    timestamp: DateTime,
    requestor_id: PublisherId,
    correlation_data: Option<ByteString>,
    response_address: Topic,
    timeout_hint: TimeoutHint,
    messages: Vec<ActionRequest<P>>,
}

impl<P: ActionPayload> ActionRequestMessage<P> {
    /// A request on the Call topic `call`: the PublisherId is the Responder
    /// of the topic (REQ-ACT-005), the ResponseAddress its Reply topic
    /// (REQ-ACT-006); `message_id` is formed with the RequestorId
    /// (REQ-ACT-004).
    ///
    /// # Errors
    ///
    /// [`DecodeError`] if `call` is not a Call topic.
    pub fn new(
        call: &Topic,
        requestor_id: PublisherId,
        message_id: MessageId,
        timestamp: DateTime,
        timeout_hint: TimeoutHint,
        first: ActionRequest<P>,
    ) -> Result<Self, DecodeError> {
        let response_address = call
            .reply()
            .filter(|_| call.method() == Method::Call)
            .ok_or_else(|| DecodeError::new(format!("{call} is not a Call topic")))?;
        Ok(Self {
            message_id,
            publisher_id: PublisherId::new(call.service_type(), call.app_id().clone()),
            timestamp,
            requestor_id,
            correlation_data: None,
            response_address,
            timeout_hint,
            messages: vec![first],
        })
    }

    /// Sets the CorrelationData (REQ-ACT-002, REQ-ACT-007).
    #[must_use]
    pub fn with_correlation_data(mut self, data: ByteString) -> Self {
        self.correlation_data = Some(data);
        self
    }

    /// Appends an ActionRequest with its own RequestId (REQ-ACT-011).
    #[must_use]
    pub fn with(mut self, request: ActionRequest<P>) -> Self {
        self.messages.push(request);
        self
    }

    /// The response to this request: PublisherId, RequestorId and
    /// CorrelationData are copied (REQ-ACT-005, REQ-ACT-007); `message_id`
    /// is formed with the PublisherId of the Responder (REQ-ACT-004). It is
    /// published to [`Self::response_address`] (REQ-ACT-006).
    pub fn response<R: ActionPayload>(
        &self,
        message_id: MessageId,
        timestamp: DateTime,
        first: ActionResponse<R>,
    ) -> ActionResponseMessage<R> {
        ActionResponseMessage {
            message_id,
            publisher_id: self.publisher_id.clone(),
            timestamp,
            requestor_id: self.requestor_id.clone(),
            correlation_data: self.correlation_data.clone(),
            messages: vec![first],
        }
    }

    /// MessageId (REQ-ACT-004).
    pub fn message_id(&self) -> &MessageId {
        &self.message_id
    }

    /// PublisherId of the Responder (REQ-ACT-005).
    pub fn publisher_id(&self) -> &PublisherId {
        &self.publisher_id
    }

    /// Time of the first sending.
    pub fn timestamp(&self) -> &DateTime {
        &self.timestamp
    }

    /// RequestorId (REQ-ACT-005).
    pub fn requestor_id(&self) -> &PublisherId {
        &self.requestor_id
    }

    /// CorrelationData.
    pub fn correlation_data(&self) -> Option<&ByteString> {
        self.correlation_data.as_ref()
    }

    /// ResponseAddress, the Reply topic (REQ-ACT-006).
    pub fn response_address(&self) -> &Topic {
        &self.response_address
    }

    /// TimeoutHint (REQ-ACT-008, REQ-ACT-009).
    pub fn timeout_hint(&self) -> &TimeoutHint {
        &self.timeout_hint
    }

    /// The ActionRequests, at least one.
    pub fn messages(&self) -> &[ActionRequest<P>] {
        &self.messages
    }
}

/// A `ua-action-response` (REQ-ACT-001, REQ-ACT-002); it has neither
/// ResponseAddress nor TimeoutHint.
#[derive(Debug, Clone, PartialEq)]
pub struct ActionResponseMessage<P> {
    message_id: MessageId,
    publisher_id: PublisherId,
    timestamp: DateTime,
    requestor_id: PublisherId,
    correlation_data: Option<ByteString>,
    messages: Vec<ActionResponse<P>>,
}

impl<P: ActionPayload> ActionResponseMessage<P> {
    /// Appends an ActionResponse.
    #[must_use]
    pub fn with(mut self, response: ActionResponse<P>) -> Self {
        self.messages.push(response);
        self
    }

    /// Whether this response belongs to `request`: same Responder,
    /// RequestorId and CorrelationData (REQ-ACT-005, REQ-ACT-007).
    pub fn correlates<Q>(&self, request: &ActionRequestMessage<Q>) -> bool {
        self.publisher_id == request.publisher_id
            && self.requestor_id == request.requestor_id
            && self.correlation_data == request.correlation_data
    }

    /// MessageId (REQ-ACT-004).
    pub fn message_id(&self) -> &MessageId {
        &self.message_id
    }

    /// PublisherId of the Responder (REQ-ACT-005).
    pub fn publisher_id(&self) -> &PublisherId {
        &self.publisher_id
    }

    /// Time of the first sending.
    pub fn timestamp(&self) -> &DateTime {
        &self.timestamp
    }

    /// RequestorId (REQ-ACT-005).
    pub fn requestor_id(&self) -> &PublisherId {
        &self.requestor_id
    }

    /// CorrelationData of the request, unchanged (REQ-ACT-007).
    pub fn correlation_data(&self) -> Option<&ByteString> {
        self.correlation_data.as_ref()
    }

    /// The ActionResponses, at least one.
    pub fn messages(&self) -> &[ActionResponse<P>] {
        &self.messages
    }
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
struct ActionWire<M> {
    message_id: MessageId,
    message_type: String,
    publisher_id: PublisherId,
    timestamp: DateTime,
    requestor_id: PublisherId,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    correlation_data: Option<ByteString>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    response_address: Option<Topic>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    timeout_hint: Option<TimeoutHint>,
    messages: M,
}

impl<P: ActionPayload> Serialize for ActionRequestMessage<P> {
    fn serialize<S: Serializer>(&self, s: S) -> Result<S::Ok, S::Error> {
        ActionWire {
            message_id: self.message_id.clone(),
            message_type: "ua-action-request".to_owned(),
            publisher_id: self.publisher_id.clone(),
            timestamp: self.timestamp.clone(),
            requestor_id: self.requestor_id.clone(),
            correlation_data: self.correlation_data.clone(),
            response_address: Some(self.response_address.clone()),
            timeout_hint: Some(self.timeout_hint.clone()),
            messages: &self.messages,
        }
        .serialize(s)
    }
}

impl<'de, P: ActionPayload> Deserialize<'de> for ActionRequestMessage<P> {
    fn deserialize<D: Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        let w = ActionWire::<Vec<ActionRequest<P>>>::deserialize(d)?;
        if w.message_type != "ua-action-request" {
            return Err(D::Error::custom(
                "MessageType is not ua-action-request (REQ-ACT-001)",
            ));
        }
        let (Some(response_address), Some(timeout_hint)) = (w.response_address, w.timeout_hint)
        else {
            return Err(D::Error::custom(
                "a request has ResponseAddress and TimeoutHint (REQ-ACT-002)",
            ));
        };
        if response_address.method() != Method::Reply {
            return Err(D::Error::custom(
                "the ResponseAddress is a Reply topic (REQ-ACT-006)",
            ));
        }
        if w.messages.is_empty() {
            return Err(D::Error::custom("Messages is empty (REQ-ACT-002)"));
        }
        Ok(Self {
            message_id: w.message_id,
            publisher_id: w.publisher_id,
            timestamp: w.timestamp,
            requestor_id: w.requestor_id,
            correlation_data: w.correlation_data,
            response_address,
            timeout_hint,
            messages: w.messages,
        })
    }
}

impl<P: ActionPayload> Serialize for ActionResponseMessage<P> {
    fn serialize<S: Serializer>(&self, s: S) -> Result<S::Ok, S::Error> {
        ActionWire {
            message_id: self.message_id.clone(),
            message_type: "ua-action-response".to_owned(),
            publisher_id: self.publisher_id.clone(),
            timestamp: self.timestamp.clone(),
            requestor_id: self.requestor_id.clone(),
            correlation_data: self.correlation_data.clone(),
            response_address: None,
            timeout_hint: None,
            messages: &self.messages,
        }
        .serialize(s)
    }
}

impl<'de, P: ActionPayload> Deserialize<'de> for ActionResponseMessage<P> {
    fn deserialize<D: Deserializer<'de>>(d: D) -> Result<Self, D::Error> {
        let w = ActionWire::<Vec<ActionResponse<P>>>::deserialize(d)?;
        if w.message_type != "ua-action-response" {
            return Err(D::Error::custom(
                "MessageType is not ua-action-response (REQ-ACT-001)",
            ));
        }
        if w.response_address.is_some() || w.timeout_hint.is_some() {
            return Err(D::Error::custom(
                "a response has neither ResponseAddress nor TimeoutHint (REQ-ACT-002)",
            ));
        }
        if w.messages.is_empty() {
            return Err(D::Error::custom("Messages is empty (REQ-ACT-002)"));
        }
        Ok(Self {
            message_id: w.message_id,
            publisher_id: w.publisher_id,
            timestamp: w.timestamp,
            requestor_id: w.requestor_id,
            correlation_data: w.correlation_data,
            messages: w.messages,
        })
    }
}

/// Request fields of NewDataSetWriterId (REQ-ACT-021).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct NewDataSetWriterIdRequest {
    /// The resource the DataSetWriterId is used for.
    pub resource: Resource,
}

impl ActionPayload for NewDataSetWriterIdRequest {
    const COMMON: Option<(u16, u16)> = Some((8, 1));
}

/// Response fields of NewDataSetWriterId (REQ-ACT-022).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct NewDataSetWriterIdResponse {
    /// A new, so far unused DataSetWriterId of the Responder.
    pub data_set_writer_id: DataSetWriterId,
    /// Validity in seconds.
    pub ttl: u32,
}

impl ActionPayload for NewDataSetWriterIdResponse {
    const COMMON: Option<(u16, u16)> = Some((8, 1));
}
