//! Hello World: a Level 1 OEC application in a container (DX goal: ≤ 20
//! lines). Birth, Will, Close, Health heartbeat and the MAM come from the SDK.
use oec::prelude::*;

fn main() -> oec::Result<()> {
    let product = Product::new("example.com", "Hello OEC", "HELLO-1")
        .manufacturer("Example Inc.")
        .software_revision(env!("CARGO_PKG_VERSION"))
        .device_manual("https://example.com/hello/manual.pdf");

    let app = App::builder(Level::L1, ServiceType::Utility, product)
        .bootstrap(Bootstrap::container())
        .build()?;
    app.this().set_health(HealthState::Normal, Some(100));
    app.run()
}
