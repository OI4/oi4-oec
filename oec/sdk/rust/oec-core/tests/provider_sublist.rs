//! The SubscriptionList of the provider (P4 increment 8b, ADR-0026 §18):
//! the subscriptions of the consumer side and the `Set` subscriptions of
//! the writable DataSets (REQ-RES-SUBLIST-016) answered on `Get`
//! (REQ-RES-SUBLIST-008), published again after a change
//! (REQ-RES-SUBLIST-005), changed by `Set` and deleted by `Del` with a
//! Status event (REQ-RES-SUBLIST-010 … 015), persisted. The guideline
//! vectors V-PAY-072 … 075 (MG B1.1.9) are replayed against the provider.

use std::time::Duration;

use oec_core::il::IdentificationLink;
use oec_core::lifecycle::{Lifecycle, Publication};
use oec_core::message::DataSetWriterId;
use oec_core::provider::data::{BuiltinType as T, DataSetSpec, FieldSpec, Mode};
use oec_core::provider::{
    ConfigSpec, ConsumerSubscription, Handled, Provider, ProviderConfig, SubscriptionOverrides,
    SubscriptionSetting, WriterIds,
};
use oec_core::resource::{Config, Mam};
use oec_core::status::StatusCode;
use oec_core::topic::ServiceType;
use serde_json::{Value as Json, json};

const MS: u64 = 1_700_000_000_000;
const PUMP: &str = "vendor.com/Pump/P-1/0815";
const CTX: &str = "Overall device settings";
const REQ_ID: &str = "1639693510000-ITConnector/https://id.consumer.com/MEService/Inst07322";
const PRESENCE: [&str; 2] = ["Oi4v2/+/+/Pub/MAM/+", "Oi4v2/+/+/Pub/Health/+"];
const STATUS: &str = "Oi4v2/+/+/Pub/Event/+/Status/+";

fn il(s: &str) -> IdentificationLink {
    IdentificationLink::parse(s).unwrap()
}

fn vector(id: &str) -> (String, Json) {
    let file = concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../spec/export/vectors/payload-guideline.json"
    );
    let all: Json = serde_json::from_str(&std::fs::read_to_string(file).unwrap()).unwrap();
    let v = all["vectors"]
        .as_array()
        .unwrap()
        .iter()
        .find(|v| v["id"] == id)
        .unwrap();
    (
        v["topic"].as_str().unwrap().to_owned(),
        serde_json::from_str(v["payload"].as_str().unwrap()).unwrap(),
    )
}

/// The AppId of MG B1.1.9.
fn app() -> String {
    let (_, p) = vector("V-PAY-072");
    p["Messages"][0]["DataSetWriterName"]
        .as_str()
        .unwrap()
        .to_owned()
}

/// The subscription of MG B1.1.9: the Data `oee` of the application.
fn oee_filter() -> String {
    let (_, p) = vector("V-PAY-072");
    p["Messages"][0]["Payload"]["TopicPath"]
        .as_str()
        .unwrap()
        .to_owned()
}

fn base() -> String {
    format!("Oi4v2/OTConnector/{}", il(&app()).encode())
}

fn mam(piu: &str) -> Mam {
    serde_json::from_value(json!({
        "Manufacturer": {"Locale": "en-US", "Text": "Vendor"},
        "ManufacturerUri": "vendor.com",
        "Model": {"Locale": "en-US", "Text": "Service"},
        "ProductCode": "FDS-001",
        "HardwareRevision": "",
        "SoftwareRevision": "1.0.0",
        "DeviceRevision": "",
        "DeviceManual": "",
        "DeviceClass": "Oi4.OTConnector",
        "SerialNumber": "1200-0345",
        "ProductInstanceUri": piu,
        "RevisionCounter": 0
    }))
    .unwrap()
}

fn provider_with(overrides: SubscriptionOverrides) -> Provider {
    let app = app();
    let lifecycle = Lifecycle::new(
        ServiceType::OTConnector,
        il(&app),
        mam(&app),
        DataSetWriterId::new(10).unwrap(),
        DataSetWriterId::new(11).unwrap(),
    )
    .unwrap();
    let config = ProviderConfig {
        get: true,
        max_packet_size: 262_144,
        publish_limit: 262_144,
    };
    Provider::new(lifecycle, config, WriterIds::new())
        .unwrap()
        .with_subscription_overrides(overrides)
}

fn sub(path: &str, interval_ms: u32, configurable: bool) -> ConsumerSubscription {
    ConsumerSubscription {
        topic_path: path.to_owned(),
        interval_ms,
        configurable,
    }
}

/// The subscriptions of a consumer: presence, the Status events of every
/// application (twice, one with an Interval), and the configurable `oee`
/// subscription of MG B1.1.9.
fn consumer_subscriptions() -> Vec<ConsumerSubscription> {
    let mut subs: Vec<_> = PRESENCE.iter().map(|f| sub(f, 0, false)).collect();
    subs.push(sub(STATUS, 500, true));
    subs.push(sub(STATUS, 200, false));
    subs.push(sub(&oee_filter(), 0, true));
    subs
}

/// The application of MG B1.1.9 with the asset PUMP, the Config of
/// V-PAY-099, a Data DataSet `t` of PUMP the application accepts and one
/// `u` it does not, and the subscriptions of [`consumer_subscriptions`];
/// connected.
fn provider() -> (Provider, IdentificationLink) {
    provider_from(SubscriptionOverrides::new())
}

fn provider_from(overrides: SubscriptionOverrides) -> (Provider, IdentificationLink) {
    let mut p = provider_with(overrides);
    let src = il(&app());
    let pump = p.add_asset(mam(PUMP)).unwrap();
    let (_, config) = vector("V-PAY-099");
    let config: Config = serde_json::from_value(config["Messages"][0]["Payload"].clone()).unwrap();
    let spec = ConfigSpec {
        context: CTX.to_owned(),
        config,
    };
    p.add_config(&src, spec, MS).unwrap();
    for tag in ["t", "u"] {
        let mut d = DataSetSpec::new(tag);
        d.fields = vec![FieldSpec::new("T", T::Double)];
        d.publication.mode = Mode::ON_REQUEST;
        p.add_data(&pump, d, MS).unwrap();
    }
    p.accept_set_data(&pump, "t", true).unwrap();
    p.set_consumer_subscriptions(consumer_subscriptions());
    p.on_connected(Duration::ZERO, MS);
    (p, pump)
}

fn json_of(p: &Publication) -> Json {
    serde_json::from_slice(&p.payload).unwrap()
}

fn topics(ps: &[Publication]) -> Vec<String> {
    ps.iter().map(|p| p.topic.to_string()).collect()
}

fn request(messages: &Json) -> Vec<u8> {
    serde_json::to_vec(&json!({
        "MessageId": REQ_ID,
        "MessageType": "ua-data",
        "PublisherId": "ITConnector/https://id.consumer.com/MEService/Inst07322",
        "ReplyTo": "",
        "Messages": messages
    }))
    .unwrap()
}

fn handle(p: &mut Provider, topic: &str, payload: &[u8]) -> Vec<Publication> {
    match p.handle_message(topic, payload, MS + 1).unwrap() {
        Handled::Publish(out) => out,
        h => panic!("{h:?}"),
    }
}

fn get(p: &mut Provider, rest: &str) -> Json {
    let topic = format!("{}/Get/SubscriptionList{rest}", base());
    let out = handle(p, &topic, &request(&json!([])));
    assert_eq!(topics(&out), [topic.replace("/Get/", "/Pub/")]);
    let v = json_of(&out[0]);
    assert_eq!(v["ReplyTo"], REQ_ID);
    v["Messages"].clone()
}

/// A `Set` or `Del SubscriptionList` of the entry `payload` to
/// `<Source>/<filter>` of the application.
fn write(p: &mut Provider, method: &str, filter: &str, payload: &Json) -> Vec<Publication> {
    let topic = format!(
        "{}/{method}/SubscriptionList/{}/{filter}",
        base(),
        il(&app()).encode()
    );
    let messages = json!([{
        "DataSetWriterId": 227,
        "WriterGroupName": filter,
        "DataSetWriterName": app(),
        "Payload": payload
    }]);
    handle(p, &topic, &request(&messages))
}

/// The StatusCode of the Status event that ends `out`; checks the ReplyTo.
fn status(out: &[Publication]) -> u32 {
    let v = json_of(out.last().expect("a Status event"));
    assert_eq!(v["ReplyTo"], REQ_ID);
    u32::try_from(v["Messages"][0]["Payload"]["Number"].as_u64().unwrap()).unwrap()
}

/// `(TopicPath, Interval, Config, WriterGroupName)` of each entry.
fn entries(messages: &Json) -> Vec<(String, Json, Json, Json)> {
    messages
        .as_array()
        .unwrap()
        .iter()
        .map(|m| {
            let e = &m["Payload"];
            (
                e["TopicPath"].as_str().unwrap().to_owned(),
                e["Interval"].clone(),
                e["Config"].clone(),
                m["WriterGroupName"].clone(),
            )
        })
        .collect()
}

fn set_topic(resource: &str, source: &IdentificationLink, filter: &str) -> String {
    format!("{}/Set/{resource}/{}/{filter}", base(), source.encode())
}

/// Every entry of the application: the consumer subscriptions per filter
/// in topic order, then the `Set` subscriptions of the Config and the
/// accepted Data, `NONE_0` (REQ-RES-SUBLIST-016).
#[test]
fn get_lists_consumer_subscriptions_and_writable_data_sets() {
    let (mut p, pump) = provider();
    let all = get(&mut p, "");
    let none = json!("NONE_0");
    let conf = json!("CONF_1");
    assert_eq!(
        entries(&all),
        [
            // configurable only if every subscription of the filter is; the
            // smallest Interval
            (STATUS.to_owned(), json!(200), none.clone(), json!("Event")),
            (
                PRESENCE[1].to_owned(),
                Json::Null,
                none.clone(),
                json!("Health")
            ),
            (
                PRESENCE[0].to_owned(),
                Json::Null,
                none.clone(),
                json!("MAM")
            ),
            (oee_filter(), Json::Null, conf, json!("Data/oee")),
            (
                set_topic("Config", &il(&app()), CTX),
                Json::Null,
                none.clone(),
                json!(format!("Config/{CTX}"))
            ),
            (
                set_topic("Data", &pump, "t"),
                Json::Null,
                none,
                json!("Data/t")
            ),
        ]
    );
    // one DataSetWriterId, the application holds every subscription
    let ids: Vec<_> = all
        .as_array()
        .unwrap()
        .iter()
        .map(|m| {
            assert_eq!(m["DataSetWriterName"], app().as_str());
            m["DataSetWriterId"].clone()
        })
        .collect();
    assert!(ids.windows(2).all(|w| w[0] == w[1]), "{ids:?}");
    // with Source the application: the same; an asset: none
    assert_eq!(get(&mut p, &format!("/{}", il(&app()).encode())), all);
    assert_eq!(get(&mut p, &format!("/{}", pump.encode())), json!([]));
}

/// With Source and Filter, the entries of that resource and Filter;
/// wildcard entries match the Filter they cover (REQ-RES-SUBLIST-008).
#[test]
fn get_with_filter_addresses_matching_entries() {
    let (mut p, _) = provider();
    let me = il(&app()).encode();
    let paths = |m: &Json| -> Vec<String> { entries(m).into_iter().map(|e| e.0).collect() };
    let data = get(&mut p, &format!("/{me}/Data"));
    assert_eq!(paths(&data).len(), 2);
    assert_eq!(
        paths(&get(&mut p, &format!("/{me}/Data/oee"))),
        [oee_filter()]
    );
    assert_eq!(get(&mut p, &format!("/{me}/Data/x")), json!([]));
    let bad = get(&mut p, &format!("/{me}/Event/Status/Bad"));
    assert_eq!(paths(&bad), [STATUS]);
    assert_eq!(paths(&get(&mut p, &format!("/{me}/MAM"))), [PRESENCE[0]]);
}

/// The Profile of the application lists the SubscriptionList, that of an
/// asset not (A-130).
#[test]
fn only_the_application_profile_lists_the_subscription_list() {
    let (mut p, pump) = provider();
    for (source, listed) in [(il(&app()), true), (pump, false)] {
        let topic = format!("{}/Get/Profile/{}", base(), source.encode());
        let out = handle(&mut p, &topic, &request(&json!([])));
        let resources = json_of(&out[0])["Messages"][0]["Payload"]["Resources"].clone();
        assert_eq!(
            resources
                .as_array()
                .unwrap()
                .contains(&json!("SubscriptionList")),
            listed,
            "{source}"
        );
    }
}

/// After a connect the whole list; again after every change of its
/// entries, and only then (REQ-RES-SUBLIST-005).
#[test]
fn changes_publish_the_list_again() {
    let mut p = provider_with(SubscriptionOverrides::new());
    let out = p.on_connected(Duration::ZERO, MS);
    let list = format!("{}/Pub/SubscriptionList", base());
    // nothing to list: nothing published
    assert!(!topics(&out).contains(&list), "{:?}", topics(&out));

    p.set_consumer_subscriptions(vec![sub(PRESENCE[0], 0, false)]);
    let out = p.poll(Duration::from_millis(1), MS + 1);
    assert_eq!(topics(&out), std::slice::from_ref(&list));
    let v = json_of(&out[0]);
    assert_eq!(v["ReplyTo"], "");
    assert_eq!(entries(&v["Messages"]).len(), 1);
    // the same subscriptions in another order: no change
    p.set_consumer_subscriptions(vec![sub(PRESENCE[0], 0, false)]);
    assert!(p.poll(Duration::from_millis(2), MS + 2).is_empty());

    // a DataSet the application accepts adds its Set subscription
    let src = il(&app());
    let mut d = DataSetSpec::new("t");
    d.fields = vec![FieldSpec::new("T", T::Double)];
    d.publication.mode = Mode::ON_REQUEST;
    p.add_data(&src, d, MS).unwrap();
    let out = p.poll(Duration::from_millis(3), MS + 3);
    // the PublicationList changed, the SubscriptionList not
    assert!(!topics(&out).contains(&list));
    p.accept_set_data(&src, "t", true).unwrap();
    let out = p.poll(Duration::from_millis(4), MS + 4);
    assert_eq!(topics(&out), std::slice::from_ref(&list));
    assert_eq!(entries(&json_of(&out[0])["Messages"]).len(), 2);

    // the last entries removed: the empty list
    p.accept_set_data(&src, "t", false).unwrap();
    p.set_consumer_subscriptions(Vec::new());
    let out = p.poll(Duration::from_millis(5), MS + 5);
    assert_eq!(topics(&out), [list]);
    assert_eq!(json_of(&out[0])["Messages"], json!([]));
}

/// MG B1.1.9: the `Set` V-PAY-072 changes the Interval of the `oee`
/// subscription; the entry is published with the ReplyTo, then the Status
/// event V-PAY-073. The `Del` V-PAY-074 deletes it: no entry remains
/// in `Data/oee`, so only the Status event V-PAY-075 follows.
#[test]
fn guideline_set_and_del_vectors_are_answered() {
    let (mut p, _) = provider();
    p.take_filter_overrides();
    let (set_topic, set) = vector("V-PAY-072");
    let (status_topic, expected) = vector("V-PAY-073");
    let out = handle(&mut p, &set_topic, &serde_json::to_vec(&set).unwrap());
    assert_eq!(
        topics(&out),
        [set_topic.replace("/Set/", "/Pub/"), status_topic.clone()]
    );
    let entry = json_of(&out[0]);
    assert_eq!(entry["ReplyTo"], set["MessageId"]);
    assert_eq!(entry["DataSetClassId"], set["DataSetClassId"]);
    let (got, want) = (&entry["Messages"][0], &set["Messages"][0]);
    for k in ["WriterGroupName", "DataSetWriterName", "Payload"] {
        assert_eq!(got[k], want[k], "{k}");
    }
    let event = json_of(&out[1]);
    assert_eq!(event["ReplyTo"], set["MessageId"]);
    for k in ["WriterGroupName", "DataSetWriterName", "Payload"] {
        assert_eq!(event["Messages"][0][k], expected["Messages"][0][k], "{k}");
    }
    // persisted, handed to the consumer side, not published again
    assert!(p.take_subscription_overrides().is_some());
    let o = p.take_filter_overrides().unwrap();
    assert_eq!(o.interval(&oee_filter()), Some(1000));
    assert!(p.take_filter_overrides().is_none());
    assert!(p.poll(Duration::from_millis(1), MS + 1).is_empty());

    let (del_topic, del) = vector("V-PAY-074");
    let (status_topic, expected) = vector("V-PAY-075");
    let out = handle(&mut p, &del_topic, &serde_json::to_vec(&del).unwrap());
    assert_eq!(topics(&out), [status_topic]);
    let event = json_of(&out[0]);
    for k in ["WriterGroupName", "DataSetWriterName", "Payload"] {
        assert_eq!(event["Messages"][0][k], expected["Messages"][0][k], "{k}");
    }
    let me = il(&app()).encode();
    assert_eq!(get(&mut p, &format!("/{me}/Data/oee")), json!([]));
    assert!(p.take_filter_overrides().unwrap().is_deleted(&oee_filter()));

    // a Set lists the deleted subscription again
    let out = handle(&mut p, &set_topic, &serde_json::to_vec(&set).unwrap());
    assert_eq!(status(&out), StatusCode::GOOD.0);
    assert_eq!(entries(&get(&mut p, &format!("/{me}/Data/oee"))).len(), 1);
    assert_eq!(
        p.take_filter_overrides().unwrap().interval(&oee_filter()),
        Some(1000)
    );
}

/// `Set` and `Del` of entries they may not change (REQ-RES-SUBLIST-004,
/// REQ-RES-SUBLIST-010, REQ-RES-SUBLIST-011).
#[test]
fn writes_are_checked() {
    let (mut p, pump) = provider();
    let conf = |path: &str| json!({"TopicPath": path, "Interval": 5, "Config": "CONF_1"});
    let other = "Oi4v2/+/+/Pub/Data/+/v";
    let cases = [
        // NONE_0: neither changed nor deleted
        ("Set", "Event", conf(STATUS), StatusCode::BAD_NOT_WRITABLE),
        ("Del", "Event", conf(STATUS), StatusCode::BAD_NOT_WRITABLE),
        (
            "Set",
            "MAM",
            conf(PRESENCE[0]),
            StatusCode::BAD_NOT_WRITABLE,
        ),
        // the Config is read only
        (
            "Set",
            "Data/oee",
            json!({"TopicPath": oee_filter(), "Interval": 5}),
            StatusCode::BAD_NOT_WRITABLE,
        ),
        // a subscription the application does not hold is not created
        ("Set", "Data/v", conf(other), StatusCode::BAD_NOT_SUPPORTED),
        ("Del", "Data/v", conf(other), StatusCode::BAD_NOT_FOUND),
        // the Filter of the topic does not fit the TopicPath
        (
            "Set",
            "Data",
            conf(&oee_filter()),
            StatusCode::BAD_INVALID_ARGUMENT,
        ),
        (
            "Del",
            "Data/x",
            conf(&oee_filter()),
            StatusCode::BAD_INVALID_ARGUMENT,
        ),
        // no entry
        (
            "Set",
            "Data/oee",
            json!({"Interval": 5}),
            StatusCode::BAD_DECODING_ERROR,
        ),
    ];
    for (method, filter, payload, code) in cases {
        let out = write(&mut p, method, filter, &payload);
        assert_eq!(out.len(), 1, "{method} {filter} {payload}");
        assert_eq!(status(&out), code.0, "{method} {filter} {payload}");
    }
    // an asset holds no subscription
    let topic = format!("{}/Del/SubscriptionList/{}/Data/oee", base(), pump.encode());
    let messages = json!([{"DataSetWriterId": 227, "Payload": conf(&oee_filter())}]);
    let out = handle(&mut p, &topic, &request(&messages));
    assert_eq!(status(&out), StatusCode::BAD_NOT_FOUND.0);
    // nothing changed
    assert!(p.take_subscription_overrides().is_none());
    assert_eq!(entries(&get(&mut p, "")).len(), 6);
}

/// The settings survive a restart; a subscription that is no longer
/// configurable ignores them.
#[test]
fn settings_are_persisted() {
    let (mut p, _) = provider();
    let set = json!({"TopicPath": oee_filter(), "Interval": 250, "Config": "CONF_1"});
    assert_eq!(status(&write(&mut p, "Set", "Data/oee", &set)), 0);
    let saved = p.take_subscription_overrides().unwrap();
    let table = SubscriptionOverrides::from_json(&saved).unwrap();
    assert_eq!(
        table.get(&oee_filter()),
        Some(SubscriptionSetting::Interval(250))
    );

    let (mut p, _) = provider_from(table.clone());
    let me = il(&app()).encode();
    let oee = entries(&get(&mut p, &format!("/{me}/Data/oee")));
    assert_eq!(oee[0].1, json!(250));
    assert_eq!(
        p.take_filter_overrides().unwrap().interval(&oee_filter()),
        Some(250)
    );

    let mut p = provider_with(table);
    p.set_consumer_subscriptions(vec![sub(&oee_filter(), 0, false)]);
    p.on_connected(Duration::ZERO, MS);
    let oee = entries(&get(&mut p, &format!("/{me}/Data/oee")));
    assert_eq!(
        (oee[0].1.clone(), oee[0].2.clone()),
        (Json::Null, json!("NONE_0"))
    );
    assert!(p.take_filter_overrides().unwrap().is_empty());
}
