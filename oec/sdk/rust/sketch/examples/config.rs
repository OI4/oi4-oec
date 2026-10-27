// Providing Config: a polling interval and a sensitive API token, validated
// and applied atomically by the SDK; the application reacts to changes.
use oec::prelude::*;

fn main() -> oec::Result<()> {
    let product = Product::new("example.com", "Weather", "WEATHER-1")
        .manufacturer("Example Inc.")
        .software_revision(env!("CARGO_PKG_VERSION"))
        .device_manual("https://example.com/weather/manual.pdf");
    let app = App::builder(Level::L3, ServiceType::OTConnector, product).build()?;

    let config = app.this().add_config(ConfigDef::new("service").group(
        "common",
        "Common settings",
        [
            ConfigObject::number("PollInterval", 60.0)
                .label("Polling interval")
                .unit("s")
                .min(10.0)
                .max(3600.0)
                .mandatory(),
            ConfigObject::string("ApiToken", "").label("API token").sensitive(),
        ],
    ))?;

    config.on_set(|proposed| {
        let poll = proposed.number("common", "PollInterval").unwrap_or(60.0);
        if poll % 10.0 != 0.0 {
            return Err(StatusCode::BadOutOfRange); // own rule beyond the Validation
        }
        Ok(())
    })?;

    let poll = config.values().number("common", "PollInterval");
    println!("polling every {poll:?} s");
    app.run()
}
