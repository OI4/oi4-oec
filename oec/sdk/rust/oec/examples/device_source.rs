//! A Level 3 OT connector that represents a field device (asset without own
//! MQTT client), publishes its process value and reacts to `Set Data`.
use std::time::Duration;

use oec::data::{Value, Writable};
use oec::prelude::*;

fn main() -> oec::Result<()> {
    let product = Product::new("example.com", "Demo Connector", "CONN-1")
        .manufacturer("Example Inc.")
        .software_revision(env!("CARGO_PKG_VERSION"))
        .device_manual("https://example.com/connector/manual.pdf");
    let app = App::builder(Level::L3, ServiceType::OTConnector, product)
        .bootstrap(Bootstrap::container())
        .build()?;

    // The field device: no Identification Link of its own → Oi4Identifier.
    let sensor = app.add_asset(AssetDef::with_oi4_identifier(
        Product::new("sensor-maker.com", "TS-100", "TS-100-A").hardware_revision("2"),
        "4711",
    )?)?;
    sensor.set_health(HealthState::Normal, Some(100));

    // Oi4Pv DataSet: published on change of at least 0.1 °C; consumers may
    // change Mode and Interval via Set PublicationList.
    let temperature = sensor.add_data(
        DataSetDef::oi4pv(Field::double("Pv").unit("°C").description("Temperature"))
            .field(Field::double("Sv1").unit("°C").description("Setpoint"))
            .publication(
                Publication::default_data()
                    .precision("Pv", 0.1)
                    .writable(Writable::ModeAndInterval),
            ),
    )?;

    // Writable setpoint: the SDK validates the types, sends the Status event
    // and publishes the new state; the handler only decides.
    temperature.on_set(|req| match req.get("Sv1") {
        Some(Value::Double(sv)) if (-20.0..=80.0).contains(sv) => Ok(()),
        _ => Err(StatusCode::BadOutOfRange),
    })?;

    // The DataSet is published once every field has a value.
    temperature.update([("Pv", 21.0), ("Sv1", 22.0)])?;

    // Field bus polling in a plain thread: update() is sync and
    // non-blocking; it fails only for an unknown field, a wrong type or a
    // removed DataSet.
    let pv = temperature.clone();
    std::thread::spawn(move || {
        while pv.update([("Pv", 21.5)]).is_ok() {
            std::thread::sleep(Duration::from_secs(1));
        }
    });

    app.run()
}
