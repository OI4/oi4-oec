//! The message model from the sender side: messages built with the API
//! encode with the fixed content masks, pass the strict entry schema of
//! their topic (feature `strict-validation`) and decode to themselves;
//! the Action correlation and the invariants of the model hold.

use std::num::NonZeroU32;

use oec_core::il::IdentificationLink;
use oec_core::message::{
    ActionRequest, ActionRequestMessage, ActionState, ByteString, DataNetworkMessage, DataSetEntry,
    DataSetMessage, DataSetMessageType, DataSetVersion, DataSetWriterId, DateTime, Locale,
    LocalizedText, Message, MessageId, NewDataSetWriterIdRequest, NewDataSetWriterIdResponse,
    Pagination, PaginationRequest, PublisherId, Query, RESERVED_CLASS_IDS, TimeoutHint,
};
use oec_core::resource::{
    Data, DetailObject, Event, EventCategory, Health, HealthState, Mam, ReferenceDesignationQuery,
};
use oec_core::topic::{Resource, ServiceType, Topic};
use serde_json::{Value, json};
use time::OffsetDateTime;

const NOW_MS: u64 = 1_639_693_510_094;

fn asset() -> IdentificationLink {
    IdentificationLink::parse("provider.com/FieldDataService/FDS-001/1200-0345").unwrap()
}

fn connector() -> PublisherId {
    PublisherId::new(ServiceType::OTConnector, asset())
}

fn consumer() -> PublisherId {
    PublisherId::new(
        ServiceType::ITConnector,
        IdentificationLink::parse("https://id.consumer.com/MEService/Inst07322").unwrap(),
    )
}

fn now() -> DateTime {
    DateTime::from(
        OffsetDateTime::UNIX_EPOCH + time::Duration::milliseconds(i64::try_from(NOW_MS).unwrap()),
    )
}

fn topic(method: &str, subject: &str, source: bool) -> Topic {
    sourced(method, subject, source, "")
}

/// A topic with an optional Source and a filter after it (`/<Tag>`).
fn sourced(method: &str, subject: &str, source: bool, filter: &str) -> Topic {
    let app = asset().encode();
    let mut t = format!("Oi4v2/OTConnector/{app}/{method}/{subject}");
    if source {
        t.push('/');
        t.push_str(&app);
    }
    t.push_str(filter);
    Topic::parse(&t).unwrap()
}

/// Encodes, checks the strict schema of the topic and decodes again.
fn round_trip(topic: &Topic, m: &Message) -> Value {
    let v = m.to_value().unwrap();
    #[cfg(feature = "strict-validation")]
    {
        use oec_core::schema::{Flavor, Validator};
        Validator::new()
            .unwrap()
            .validate_topic(topic, Flavor::Strict, &v)
            .unwrap_or_else(|e| panic!("{topic}: {e}\n{v:#}"));
    }
    let back = Message::decode(topic, &m.encode().unwrap()).unwrap();
    assert_eq!(&back, m, "{topic}");
    v
}

fn mam() -> Mam {
    Mam {
        manufacturer: LocalizedText::new("en-US", "Provider"),
        manufacturer_uri: "provider.com".into(),
        model: LocalizedText::new("en-US", "FieldDataService"),
        product_code: "FDS-001".into(),
        hardware_revision: String::new(),
        software_revision: "2.0.0".into(),
        device_revision: String::new(),
        device_manual: String::new(),
        device_class: "Software".into(),
        serial_number: "1200-0345".into(),
        product_instance_uri: "provider.com/FieldDataService/FDS-001/1200-0345".into(),
        revision_counter: 0,
        description: None,
        global_asset_id: None,
    }
}

#[test]
fn get_and_pub_mam_with_reply_to_and_pagination() {
    // the request: helper DataSets only (REQ-RES-MAM-020)
    let get = DataNetworkMessage::<Mam>::new(MessageId::new(NOW_MS, None, &consumer()), consumer())
        .with(DataSetEntry::PaginationRequest(DataSetMessage::helper(
            asset(),
            PaginationRequest {
                per_page: 10,
                page: 1,
            },
        )))
        .with(DataSetEntry::Locale(DataSetMessage::helper(
            asset(),
            Locale {
                locale: "de-DE".into(),
            },
        )));
    let v = round_trip(&topic("Get", "MAM", true), &Message::Mam(get.clone()));
    assert_eq!(v["ReplyTo"], "");
    assert_eq!(v["DataSetClassId"], "360ca8f3-5e66-42a2-8f10-9cdf45f4bf58");
    assert_eq!(v["Messages"][0]["DataSetWriterId"], 1);
    assert_eq!(v["Messages"][1]["WriterGroupName"], "Locale");

    // the response refers to the request (REQ-MSG-015)
    let id = MessageId::new(NOW_MS + 1, Some(7), &connector());
    assert_eq!(
        id.as_str(),
        "16396935100957-OTConnector/provider.com/FieldDataService/FDS-001/1200-0345"
    );
    let pagination = Pagination {
        total_count: 1,
        per_page: 10,
        page: 1,
        has_next: false,
        pagination_id: id.clone(),
    };
    let reply = DataNetworkMessage::<Mam>::new(id, connector())
        .in_reply_to(get.message_id().clone())
        .with(DataSetEntry::Payload(
            DataSetMessage::new(DataSetWriterId::FIRST, asset(), "", mam())
                .with_sequence_number(1)
                .with_timestamp(now())
                .with_version(DataSetVersion::MinorVersion(3)),
        ))
        .with(DataSetEntry::Pagination(DataSetMessage::helper(
            asset(),
            pagination,
        )));
    let v = round_trip(&topic("Pub", "MAM", true), &Message::Mam(reply.clone()));
    assert_eq!(v["ReplyTo"], get.message_id().as_str());
    // fixed content mask (REQ-MSG-017): no PublisherId in a DataSetMessage
    assert!(v["Messages"][0].get("PublisherId").is_none());
    assert_eq!(v["Messages"][0]["MessageType"], "ua-keyframe");
    assert_eq!(reply.payloads().count(), 1);
}

#[test]
fn health_and_event() {
    let health = Health::aggregate(vec![DetailObject {
        health: HealthState::MaintenanceRequired,
        diagnostic_code: Some("M-17".into()),
        location: None,
        description: None,
    }]);
    let m =
        DataNetworkMessage::<Health>::new(MessageId::new(NOW_MS, None, &connector()), connector())
            .with(DataSetEntry::Payload(DataSetMessage::new(
                DataSetWriterId::new(11).unwrap(),
                asset(),
                "",
                health,
            )));
    let v = round_trip(&topic("Pub", "Health", true), &Message::Health(m));
    assert_eq!(
        v["Messages"][0]["Payload"]["Health"],
        "MAINTENANCE_REQUIRED_4"
    );

    let event = Event {
        number: 0x8034_0000,
        category: EventCategory::Status,
        description: Some("out of service".into()),
        details: Some(
            json!({"SymbolicId": "BadOutOfService"})
                .as_object()
                .unwrap()
                .clone(),
        ),
    };
    let m =
        DataNetworkMessage::<Event>::new(MessageId::new(NOW_MS, None, &connector()), connector())
            .with(DataSetEntry::Payload(
                DataSetMessage::new(
                    DataSetWriterId::new(12).unwrap(),
                    asset(),
                    "Status/Bad",
                    event,
                )
                .with_timestamp(now()),
            ));
    let v = round_trip(
        &sourced("Pub", "Event", true, "/Status/Bad"),
        &Message::Event(m.clone()),
    );
    assert_eq!(v["Messages"][0]["MessageType"], "ua-event");
    let first = m.payloads().next().unwrap();
    assert_eq!(first.message_type(), Some(DataSetMessageType::Event));
}

#[test]
fn data_with_publisher_class_id_and_query() {
    let class_id = uuid::uuid!("0f1e2d3c-4b5a-4978-8796-a5b4c3d2e1f0");
    let mut payload = serde_json::Map::new();
    payload.insert("Pv".into(), json!(21.5));
    let m =
        DataNetworkMessage::<Data>::new(MessageId::new(NOW_MS, None, &connector()), connector())
            .with_data_set_class_id(class_id)
            .unwrap()
            .with(DataSetEntry::Payload(DataSetMessage::new(
                DataSetWriterId::new(20).unwrap(),
                asset(),
                "Oi4Pv",
                Data(payload),
            )));
    let v = round_trip(
        &sourced("Pub", "Data", true, "/Oi4Pv"),
        &Message::Data(m.clone()),
    );
    assert_eq!(v["DataSetClassId"], class_id.to_string());
    assert_eq!(
        m.payloads().next().unwrap().payload().oi4_pv().unwrap().pv,
        json!(21.5)
    );

    // reserved DataSetClassIds (REQ-MSG-014) and predefined ones (REQ-MSG-010)
    let fresh =
        || DataNetworkMessage::<Data>::new(MessageId::new(NOW_MS, None, &connector()), connector());
    assert!(
        fresh()
            .with_data_set_class_id(RESERVED_CLASS_IDS[0])
            .is_err()
    );
    let mam =
        DataNetworkMessage::<Mam>::new(MessageId::new(NOW_MS, None, &connector()), connector());
    assert!(mam.with_data_set_class_id(class_id).is_err());

    // a Query of ReferenceDesignation (REQ-RES-REFDES-012)
    let q = Query::from(ReferenceDesignationQuery {
        parent: asset().to_string(),
        aspect: None,
    });
    let get = DataNetworkMessage::<oec_core::resource::ReferenceDesignation>::new(
        MessageId::new(NOW_MS, None, &consumer()),
        consumer(),
    )
    .with(DataSetEntry::Query(DataSetMessage::helper(asset(), q)));
    round_trip(
        &topic("Get", "ReferenceDesignation", true),
        &Message::ReferenceDesignation(get),
    );
}

#[test]
fn new_data_set_writer_id_correlation() {
    let call = topic("Call", "NewDataSetWriterId", false);
    let request = ActionRequestMessage::new(
        &call,
        consumer(),
        MessageId::new(NOW_MS, None, &consumer()),
        now(),
        TimeoutHint::from_millis(NonZeroU32::new(5000).unwrap()),
        ActionRequest::common(
            42,
            NewDataSetWriterIdRequest {
                resource: Resource::Data,
            },
        ),
    )
    .unwrap()
    .with_correlation_data(ByteString::from_bytes(b"session-1"));
    // the Responder is the application of the topic (REQ-ACT-005), the
    // ResponseAddress its Reply topic (REQ-ACT-006)
    assert_eq!(request.publisher_id(), &connector());
    assert_eq!(request.response_address(), &call.reply().unwrap());
    let v = round_trip(&call, &Message::NewDataSetWriterIdRequest(request.clone()));
    assert_eq!(v["Messages"][0]["DataSetWriterId"], 8);
    assert_eq!(v["Messages"][0]["ActionTargetId"], 1);
    assert_eq!(v["Messages"][0]["ActionState"], 1);

    let item = &request.messages()[0];
    let ok = request.response(
        MessageId::new(NOW_MS + 5, None, &connector()),
        now(),
        item.done(NewDataSetWriterIdResponse {
            data_set_writer_id: DataSetWriterId::new(77).unwrap(),
            ttl: 60,
        }),
    );
    assert!(ok.correlates(&request));
    assert!(ok.messages()[0].answers(item));
    assert_eq!(ok.correlation_data(), request.correlation_data());
    let reply = request.response_address().clone();
    let v = round_trip(&reply, &Message::NewDataSetWriterIdResponse(ok));
    assert_eq!(v["Messages"][0]["Status"], json!({}));
    assert_eq!(v["Messages"][0]["ActionState"], 2);
    assert!(v.get("ResponseAddress").is_none() && v.get("TimeoutHint").is_none());

    // BadInvalidArgument: empty Payload (REQ-ACT-012), Code only (REQ-ACT-014)
    let code = NonZeroU32::new(0x80AB_0000).unwrap();
    let failed = request.response(
        MessageId::new(NOW_MS + 6, None, &connector()),
        now(),
        item.failed::<NewDataSetWriterIdResponse>(code),
    );
    let v = round_trip(&reply, &Message::NewDataSetWriterIdResponse(failed.clone()));
    assert_eq!(v["Messages"][0]["Status"], json!({"Code": 0x80AB_0000_u32}));
    assert_eq!(v["Messages"][0]["Payload"], json!({}));
    assert!(failed.messages()[0].payload().is_none());
    assert_eq!(failed.messages()[0].action_state(), ActionState::Done);
    assert!(
        item.done(NewDataSetWriterIdResponse {
            data_set_writer_id: DataSetWriterId::FIRST,
            ttl: 1
        })
        .with_status(code)
        .is_err()
    );

    // a Call topic is required
    assert!(
        ActionRequestMessage::new(
            &topic("Get", "MAM", false),
            consumer(),
            MessageId::new(NOW_MS, None, &consumer()),
            now(),
            TimeoutHint::from_millis(NonZeroU32::new(1).unwrap()),
            ActionRequest::common(
                1,
                NewDataSetWriterIdRequest {
                    resource: Resource::Mam
                }
            ),
        )
        .is_err()
    );
}

#[test]
fn decoder_refuses_what_breaks_the_model() {
    let t = topic("Pub", "MAM", true);
    let base = json!({
        "MessageId": "1-OTConnector/a", "MessageType": "ua-data",
        "PublisherId": "OTConnector/provider.com/FieldDataService/FDS-001/1200-0345",
        "DataSetClassId": "360ca8f3-5e66-42a2-8f10-9cdf45f4bf58", "ReplyTo": "",
        "Messages": []
    });
    assert!(Message::from_value(&t, base.clone()).is_ok());
    let refuse = |patch: &dyn Fn(&mut Value), why: &str| {
        let mut v = base.clone();
        patch(&mut v);
        assert!(Message::from_value(&t, v).is_err(), "{why}");
    };
    refuse(
        &|v| v["DataSetClassId"] = json!("d8e7b6df-42ba-448a-975a-199f59e8ffeb"),
        "REQ-MSG-010",
    );
    refuse(
        &|v| v["DataSetClassId"] = json!("360CA8F3-5E66-42A2-8F10-9CDF45F4BF58"),
        "Guid form",
    );
    refuse(&|v| v["MessageType"] = json!("ua-metadata"), "REQ-MSG-002");
    refuse(&|v| v["ReplyTo"] = json!("x"), "REQ-MSG-007 (ReplyTo)");
    refuse(&|v| v["PublisherId"] = json!("Robot/x"), "REQ-MSG-006");
    let item = |extra: Value| {
        let mut m = json!({"DataSetWriterId": 10, "DataSetWriterName": asset().to_string(),
            "WriterGroupName": "", "Payload": serde_json::to_value(mam()).unwrap()});
        for (k, x) in extra.as_object().unwrap() {
            m[k] = x.clone();
        }
        m
    };
    let mut ok = base.clone();
    ok["Messages"] = json!([item(json!({}))]);
    assert!(Message::from_value(&t, ok).is_ok());
    for (extra, why) in [
        (json!({"DataSetWriterId": 5}), "REQ-MSG-020"),
        (json!({"MessageType": "ua-event"}), "REQ-MSG-029"),
        (
            json!({"MetaDataVersion": {"MajorVersion": 1, "MinorVersion": 2}, "MinorVersion": 2}),
            "REQ-MSG-018",
        ),
        (json!({"Status": {"Code": 0}}), "REQ-MSG-026"),
        (
            json!({"Timestamp": "2026-02-30T00:00:00Z"}),
            "REQ-MSG-016 (DateTime)",
        ),
    ] {
        refuse(&|v| v["Messages"] = json!([item(extra.clone())]), why);
    }
    // keys beyond the guideline are ignored (REQ-MSG-046)
    let mut tolerant = base.clone();
    tolerant["Vendor"] = json!(1);
    let m = Message::from_value(&t, tolerant).unwrap();
    assert!(m.to_value().unwrap().get("Vendor").is_none());
}
