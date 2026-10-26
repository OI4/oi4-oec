//! Persistence of SDK state: RevisionCounter (REQ-RES-MAM-018),
//! ReferenceDesignation (REQ-RES-REFDES-002), changed PublicationList /
//! SubscriptionList entries and Config values.
//!
//! Keys are paths of segments separated by `/`; each segment consists of
//! `A–Z a–z 0–9 . _ -`, does not start with `.` and is not empty. The SDK
//! uses keys below `app/` for the application itself, e.g.
//! `app/revision-counter`.

use std::collections::HashMap;
use std::fs;
use std::io::{self, Write};
use std::path::{Path, PathBuf};
use std::sync::Mutex;

/// Key/value persistence used by the SDK. Writes are atomic per key.
pub trait Store: Send + Sync {
    /// The value of `key`, `None` if it was never saved.
    ///
    /// # Errors
    ///
    /// I/O failures and keys the store cannot represent
    /// ([`io::ErrorKind::InvalidInput`]).
    fn load(&self, key: &str) -> io::Result<Option<Vec<u8>>>;

    /// Saves `value` under `key`, replacing an earlier value atomically.
    ///
    /// # Errors
    ///
    /// I/O failures and keys the store cannot represent
    /// ([`io::ErrorKind::InvalidInput`]).
    fn save(&self, key: &str, value: &[u8]) -> io::Result<()>;

    /// Whether values survive a restart. A store that does not cannot hold
    /// a ReferenceDesignation (REQ-RES-REFDES-002), so
    /// [`crate::app::AppBuilder::build`] rejects that combination. Default
    /// `true`.
    fn is_persistent(&self) -> bool {
        true
    }
}

/// Files below a directory (default in a container: `/etc/oi4/app/oec`,
/// REQ-CNT-010, REQ-CNT-011). One file per key; a save writes a temporary
/// file, syncs it and renames it over the old one.
#[derive(Debug, Clone)]
pub struct FileStore {
    dir: PathBuf,
}

impl FileStore {
    /// A store below `dir`; the directory is created on the first save.
    pub fn new(dir: impl Into<PathBuf>) -> Self {
        FileStore { dir: dir.into() }
    }

    /// The directory of the store.
    pub fn dir(&self) -> &Path {
        &self.dir
    }

    fn path(&self, key: &str) -> io::Result<PathBuf> {
        check_key(key)?;
        Ok(key.split('/').fold(self.dir.clone(), |p, s| p.join(s)))
    }
}

impl Store for FileStore {
    fn load(&self, key: &str) -> io::Result<Option<Vec<u8>>> {
        match fs::read(self.path(key)?) {
            Ok(v) => Ok(Some(v)),
            Err(e) if e.kind() == io::ErrorKind::NotFound => Ok(None),
            Err(e) => Err(e),
        }
    }

    fn save(&self, key: &str, value: &[u8]) -> io::Result<()> {
        let path = self.path(key)?;
        let dir = path.parent().unwrap_or(&self.dir);
        fs::create_dir_all(dir)?;
        let name = path.file_name().unwrap_or_default().to_string_lossy();
        // segments never start with '.', so the temporary name is no key
        let tmp = dir.join(format!(".{name}.tmp"));
        {
            let mut f = fs::File::create(&tmp)?;
            f.write_all(value)?;
            f.sync_all()?;
        }
        fs::rename(&tmp, &path)?;
        #[cfg(unix)]
        fs::File::open(dir)?.sync_all()?;
        Ok(())
    }
}

/// Volatile store (tests, Level 1/2 without persistence). Not persistent:
/// no ReferenceDesignation can be offered with it (REQ-RES-REFDES-002).
#[derive(Debug, Default)]
pub struct MemoryStore {
    values: Mutex<HashMap<String, Vec<u8>>>,
}

impl MemoryStore {
    /// An empty store.
    pub fn new() -> Self {
        Self::default()
    }

    fn values(&self) -> std::sync::MutexGuard<'_, HashMap<String, Vec<u8>>> {
        // a panic while holding the lock cannot leave a map half-written
        self.values
            .lock()
            .unwrap_or_else(std::sync::PoisonError::into_inner)
    }
}

impl Store for MemoryStore {
    fn load(&self, key: &str) -> io::Result<Option<Vec<u8>>> {
        check_key(key)?;
        Ok(self.values().get(key).cloned())
    }

    fn save(&self, key: &str, value: &[u8]) -> io::Result<()> {
        check_key(key)?;
        self.values().insert(key.to_owned(), value.to_vec());
        Ok(())
    }

    fn is_persistent(&self) -> bool {
        false
    }
}

/// Same key rules for every store of the SDK, so a key that works in tests
/// with [`MemoryStore`] also works with [`FileStore`].
fn check_key(key: &str) -> io::Result<()> {
    let ok = !key.is_empty()
        && key.split('/').all(|s| {
            !s.is_empty()
                && !s.starts_with('.')
                && s.bytes()
                    .all(|c| c.is_ascii_alphanumeric() || matches!(c, b'.' | b'_' | b'-'))
        });
    if ok {
        Ok(())
    } else {
        Err(io::Error::new(
            io::ErrorKind::InvalidInput,
            format!("invalid store key {key:?}"),
        ))
    }
}

#[cfg(test)]
mod tests {
    use super::{FileStore, MemoryStore, Store};

    fn dir(name: &str) -> std::path::PathBuf {
        let d = std::env::temp_dir().join(format!("oec-store-{}-{name}", std::process::id()));
        let _ = std::fs::remove_dir_all(&d);
        d
    }

    #[test]
    fn file_store_round_trip() {
        let d = dir("round-trip");
        let s = FileStore::new(&d);
        assert_eq!(s.load("app/revision-counter").unwrap(), None);
        s.save("app/revision-counter", b"1").unwrap();
        s.save("app/revision-counter", b"2").unwrap();
        assert_eq!(s.load("app/revision-counter").unwrap(), Some(b"2".to_vec()));
        assert!(d.join("app/revision-counter").is_file());
        assert!(!d.join("app/.revision-counter.tmp").exists());
        assert!(s.is_persistent());
        std::fs::remove_dir_all(d).unwrap();
    }

    #[test]
    fn memory_store_round_trip() {
        let s = MemoryStore::new();
        assert_eq!(s.load("a").unwrap(), None);
        s.save("a", b"x").unwrap();
        assert_eq!(s.load("a").unwrap(), Some(b"x".to_vec()));
        assert!(!s.is_persistent());
    }

    #[test]
    fn keys_are_checked() {
        let s = FileStore::new(dir("keys"));
        let m = MemoryStore::new();
        for key in [
            "", "/a", "a/", "a//b", "../a", "a/..", ".a", "a b", "a\\b", "ä",
        ] {
            let e = s.load(key).unwrap_err();
            assert_eq!(e.kind(), std::io::ErrorKind::InvalidInput, "{key:?}");
            assert!(m.save(key, b"").is_err(), "{key:?}");
        }
        for key in ["a", "app/x.y_z-1", "A/B/C"] {
            assert!(m.save(key, b"").is_ok(), "{key:?}");
        }
    }
}
