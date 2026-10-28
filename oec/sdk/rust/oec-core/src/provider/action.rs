//! The responder of the common service NewDataSetWriterId (MG 8.4.1,
//! MG 9.2.1; REQ-CMP-040): a `ua-action-request` on
//! `Call/NewDataSetWriterId[/<Source>]` is answered at once with one final
//! ActionResponse per ActionRequest on its ResponseAddress (REQ-ACT-015).

use core::num::NonZeroU32;

use serde::Deserialize;
use serde_json::Value;

use super::sets::date_time;
use super::writers::WriterIdError;
use super::{Handled, Ignored, Provider};
use crate::lifecycle::Publication;
use crate::message::{
    ActionRequest, ActionRequestMessage, ActionResponse, ActionResponseMessage, MessageId,
    NewDataSetWriterIdRequest, NewDataSetWriterIdResponse,
};
use crate::status::StatusCode;
use crate::topic::{Service, Subject, Topic};

/// The `Ttl` of a provided DataSetWriterId in seconds (REQ-ACT-023).
pub const WRITER_ID_TTL_SECONDS: u32 = 60;

/// DataSetWriterId and ActionTargetId of NewDataSetWriterId (REQ-ACT-019).
const ACTION: (u16, u16) = (8, 1);

type Response = ActionResponse<NewDataSetWriterIdResponse>;

impl Provider {
    /// A `Call` to this application (from Level 2). A request that cannot
    /// be decoded or addressed, or whose ResponseAddress is not the Reply
    /// topic of its Call topic, gets no response (REQ-ACT-017).
    pub(super) fn handle_call(
        &mut self,
        topic: &Topic,
        payload: &[u8],
        unix_ms: u64,
    ) -> Result<Handled, Ignored> {
        match topic.subject() {
            Subject::Service(Service::NewDataSetWriterId) if self.config.get => {}
            s => return Err(Ignored::NotServed(s)),
        }
        let request: ActionRequestMessage<Value> = serde_json::from_slice(payload)
            .map_err(|e| Ignored::Payload(format!("ua-action-request: {e}")))?;
        if request.publisher_id() != self.lifecycle.publisher_id() {
            return Err(Ignored::Payload(
                "the PublisherId is not this application (REQ-ACT-005)".to_owned(),
            ));
        }
        if topic.reply().as_ref() != Some(request.response_address()) {
            return Err(Ignored::Payload(
                "the ResponseAddress is not the Reply topic of the Call topic (REQ-ACT-006)"
                    .to_owned(),
            ));
        }
        let source = topic.source().unwrap_or(self.app_id()).clone();
        let known = self.is_source(&source);
        let responses: Vec<Response> = request
            .messages()
            .iter()
            .filter(|r| (r.data_set_writer_id(), r.action_target_id()) == ACTION)
            .map(|r| self.new_data_set_writer_id(r, source.as_str(), known, unix_ms))
            .collect();
        if responses.is_empty() {
            return Err(Ignored::Payload(
                "no ActionRequest addresses NewDataSetWriterId (REQ-ACT-017)".to_owned(),
            ));
        }
        Ok(Handled::Publish(
            self.responses(&request, responses, unix_ms),
        ))
    }

    /// The final response to one ActionRequest: a reserved id, or
    /// BadNotFound for an unknown Source (SF-113), BadInvalidArgument for
    /// an invalid Payload (REQ-ACT-014), BadResourceUnavailable if no id
    /// can be reserved now.
    fn new_data_set_writer_id(
        &mut self,
        request: &ActionRequest<Value>,
        source: &str,
        known: bool,
        unix_ms: u64,
    ) -> Response {
        if !known {
            return request.failed(bad(StatusCode::BAD_NOT_FOUND));
        }
        let Ok(fields) = NewDataSetWriterIdRequest::deserialize(request.payload()) else {
            return request.failed(bad(StatusCode::BAD_INVALID_ARGUMENT));
        };
        let ttl_ms = u64::from(WRITER_ID_TTL_SECONDS) * 1000;
        match self
            .writers
            .reserve(fields.resource, source, unix_ms, ttl_ms)
        {
            Ok(id) => {
                self.unsaved.writers = true;
                request.done(NewDataSetWriterIdResponse {
                    data_set_writer_id: id,
                    ttl: WRITER_ID_TTL_SECONDS,
                })
            }
            Err(WriterIdError::Reservations | WriterIdError::Exhausted) => {
                request.failed(bad(StatusCode::BAD_RESOURCE_UNAVAILABLE))
            }
            Err(_) => request.failed(bad(StatusCode::BAD_INTERNAL_ERROR)),
        }
    }

    /// The `ua-action-response` messages on the ResponseAddress: as many
    /// responses per message as fit the MaxPacketSize (REQ-ACT-015).
    fn responses(
        &mut self,
        request: &ActionRequestMessage<Value>,
        responses: Vec<Response>,
        unix_ms: u64,
    ) -> Vec<Publication> {
        let limit = self.config.max_packet_size;
        let mut out = Vec::new();
        let mut id = self.lifecycle.next_message_id(unix_ms);
        let mut batch: Vec<Response> = Vec::new();
        for r in responses {
            batch.push(r);
            if batch.len() > 1 && response_payload(request, &id, &batch, unix_ms).len() > limit {
                let last = batch.pop().expect("more than one");
                out.push(response_payload(request, &id, &batch, unix_ms));
                id = self.lifecycle.next_message_id(unix_ms);
                batch = vec![last];
            }
        }
        out.push(response_payload(request, &id, &batch, unix_ms));
        out.into_iter()
            .map(|payload| Publication {
                topic: request.response_address().clone(),
                payload,
            })
            .collect()
    }
}

fn response_payload(
    request: &ActionRequestMessage<Value>,
    message_id: &MessageId,
    batch: &[Response],
    unix_ms: u64,
) -> Vec<u8> {
    let mut all = batch.iter().cloned();
    let first = all.next().expect("a batch is not empty");
    let message: ActionResponseMessage<NewDataSetWriterIdResponse> = all.fold(
        request.response(message_id.clone(), date_time(unix_ms), first),
        ActionResponseMessage::with,
    );
    serde_json::to_vec(&message).expect("an ActionResponseMessage always serializes")
}

fn bad(code: StatusCode) -> NonZeroU32 {
    NonZeroU32::new(code.0).expect("a Bad StatusCode is not 0")
}
