//! Persistence of SDK state (RevisionCounter, ReferenceDesignation,
//! changed list entries, Config values).

use std::path::PathBuf;

/// Key/value persistence used by the SDK. Writes are atomic per key.
pub trait Store: Send + Sync {
    fn load(&self, key: &str) -> std::io::Result<Option<Vec<u8>>>;
    fn save(&self, key: &str, value: &[u8]) -> std::io::Result<()>;
}

/// Files below a directory (default in a container: `/etc/oi4/app/oec`,
/// REQ-CNT-010).
pub struct FileStore {
    dir: PathBuf,
}

impl FileStore {
    pub fn new(dir: impl Into<PathBuf>) -> Self {
        FileStore { dir: dir.into() }
    }
}

impl Store for FileStore {
    fn load(&self, key: &str) -> std::io::Result<Option<Vec<u8>>> {
        unimplemented!()
    }
    fn save(&self, key: &str, value: &[u8]) -> std::io::Result<()> {
        unimplemented!()
    }
}

/// Volatile store (tests, Level 1/2 without persistence).
pub struct MemoryStore;

impl Store for MemoryStore {
    fn load(&self, key: &str) -> std::io::Result<Option<Vec<u8>>> {
        unimplemented!()
    }
    fn save(&self, key: &str, value: &[u8]) -> std::io::Result<()> {
        unimplemented!()
    }
}
