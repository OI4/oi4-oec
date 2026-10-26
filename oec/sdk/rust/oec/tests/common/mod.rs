//! A container storage below a temporary root (ADR-0020 §5).

// each test binary uses a subset of the helpers
#![allow(dead_code)]

use std::path::PathBuf;

use oec::app::Bootstrap;
use oec::identity::Product;

/// `base64("user:pa:ss")`: the password contains the separator.
pub(crate) const CREDENTIALS: &str = "dXNlcjpwYTpzcw==\n";

/// The product of the tests.
pub(crate) fn product() -> Product {
    Product::new("example.com", "Hello OEC", "HELLO-1").software_revision("1.0.0")
}

/// A root directory below the target directory of the tests.
pub(crate) struct Root(PathBuf);

impl Root {
    /// An empty root.
    pub(crate) fn new(name: &str) -> Self {
        let dir = PathBuf::from(env!("CARGO_TARGET_TMPDIR")).join(format!("oec-{name}"));
        let _ = std::fs::remove_dir_all(&dir);
        std::fs::create_dir_all(&dir).unwrap();
        Root(dir)
    }

    /// A complete container storage: container name `app-1`, broker.json,
    /// `ca.pem`, credentials and the application specific volume.
    pub(crate) fn container(name: &str) -> Self {
        let root = Self::new(name);
        root.write("etc/hostname", "app-1\n");
        root.write(
            "etc/oi4/mqtt/broker.json",
            r#"{"Address": "mqtt-broker", "SecurePort": 8883, "MaxPacketSize": 262144}"#,
        );
        root.write("etc/oi4/certs/ca.pem", "CA");
        root.write("run/secrets/oi4/credentials", CREDENTIALS);
        std::fs::create_dir_all(root.0.join("etc/oi4/app")).unwrap();
        root
    }

    /// Writes a file, creating its directories.
    pub(crate) fn write(&self, path: &str, content: &str) -> &Self {
        let path = self.0.join(path);
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(path, content).unwrap();
        self
    }

    /// Removes a file or directory.
    pub(crate) fn remove(&self, path: &str) -> &Self {
        let path = self.0.join(path);
        if path.is_dir() {
            std::fs::remove_dir_all(path).unwrap();
        } else {
            std::fs::remove_file(path).unwrap();
        }
        self
    }

    /// The path of `rel` below the root.
    pub(crate) fn path(&self, rel: &str) -> PathBuf {
        self.0.join(rel)
    }

    /// `Bootstrap::ContainerStorage` with this root.
    pub(crate) fn bootstrap(&self) -> Bootstrap {
        Bootstrap::ContainerStorage {
            root: self.0.clone(),
        }
    }
}
