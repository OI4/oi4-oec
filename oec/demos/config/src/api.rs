//! The routes of the HTTP API (A-357), used by the tests and by the Web UI
//! of C-083:
//!
//! | Request | Answer |
//! |---|---|
//! | `GET /` | a short text |
//! | `GET /api/apps` | the applications, their Configs and whether writable |
//! | `POST /api/refresh` | discover again: `{}` every online one, or `{"serviceType", "app"}` |
//! | `POST /api/set` | a `Set Config`, waits for its Status event |
//! | `GET /api/sets` | the last `Set` results, newest first |

use serde_json::Value;

use crate::directory::{ApiError, AppKey, SetRequest};
use crate::http::{Request, Response};

/// What the routes need of the service.
pub(crate) trait Api {
    fn apps(&self) -> Value;
    fn sets(&self) -> Value;
    fn refresh(&self, key: Option<AppKey>) -> Result<Value, ApiError>;
    fn set(&self, req: &SetRequest) -> Result<(u16, Value), ApiError>;
}

impl Api for crate::service::Service {
    fn apps(&self) -> Value {
        Self::apps(self)
    }
    fn sets(&self) -> Value {
        Self::sets(self)
    }
    fn refresh(&self, key: Option<AppKey>) -> Result<Value, ApiError> {
        Self::refresh(self, key)
    }
    fn set(&self, req: &SetRequest) -> Result<(u16, Value), ApiError> {
        Self::set(self, req)
    }
}

const INDEX: &str = "OEC config service (D10). API: GET /api/apps, POST /api/refresh, \
POST /api/set, GET /api/sets. The Web UI follows.\n";

fn body(req: &Request) -> Result<Value, ApiError> {
    if req.body.is_empty() {
        return Ok(Value::Object(serde_json::Map::new()));
    }
    let v: Value = serde_json::from_slice(&req.body)
        .map_err(|e| ApiError::new(400, format!("invalid JSON: {e}")))?;
    if v.is_object() {
        Ok(v)
    } else {
        Err(ApiError::new(400, "the body is a JSON object"))
    }
}

fn refresh_key(body: &Value) -> Result<Option<AppKey>, ApiError> {
    let get = |k: &str| body.get(k).and_then(Value::as_str).map(str::to_owned);
    match (get("serviceType"), get("app")) {
        (Some(st), Some(app)) => Ok(Some((st, app))),
        (None, None) if body.get("serviceType").is_none() && body.get("app").is_none() => Ok(None),
        _ => Err(ApiError::new(
            400,
            "serviceType and app: both strings or none",
        )),
    }
}

fn result(r: Result<(u16, Value), ApiError>) -> Response {
    match r {
        Ok((status, v)) => Response::json(status, &v),
        Err(e) => Response::error(e.status, &e.message),
    }
}

fn not_allowed(allow: &str) -> Response {
    let mut r = Response::error(405, "method not allowed");
    r.headers.push(("Allow", allow.to_owned()));
    r
}

/// Answers one request.
pub(crate) fn route(api: &dyn Api, req: &Request) -> Response {
    match (req.path.as_str(), req.method.as_str()) {
        ("/", "GET") => Response::text(200, INDEX),
        ("/api/apps", "GET") => Response::json(200, &api.apps()),
        ("/api/sets", "GET") => Response::json(200, &api.sets()),
        ("/api/refresh", "POST") => result(
            body(req)
                .and_then(|b| refresh_key(&b))
                .and_then(|k| api.refresh(k))
                .map(|v| (200, v)),
        ),
        ("/api/set", "POST") => result(
            body(req)
                .and_then(|b| SetRequest::parse(&b))
                .and_then(|r| api.set(&r)),
        ),
        ("/" | "/api/apps" | "/api/sets", _) => not_allowed("GET"),
        ("/api/refresh" | "/api/set", _) => not_allowed("POST"),
        _ => Response::error(404, "not found"),
    }
}

#[cfg(test)]
mod tests {
    use std::sync::Mutex;

    use serde_json::json;

    use super::*;

    #[derive(Default)]
    struct Fake {
        refreshed: Mutex<Vec<Option<AppKey>>>,
        set: Mutex<Vec<SetRequest>>,
    }

    impl Api for Fake {
        fn apps(&self) -> Value {
            json!({"apps": []})
        }
        fn sets(&self) -> Value {
            json!({"sets": []})
        }
        fn refresh(&self, key: Option<AppKey>) -> Result<Value, ApiError> {
            self.refreshed.lock().unwrap().push(key);
            Ok(self.apps())
        }
        fn set(&self, req: &SetRequest) -> Result<(u16, Value), ApiError> {
            self.set.lock().unwrap().push(req.clone());
            if req.app == "offline" {
                return Err(ApiError::new(409, "the application is offlineGraceful"));
            }
            Ok((200, json!({"id": 1})))
        }
    }

    fn req(method: &str, path: &str, body: &str) -> Request {
        Request {
            method: method.to_owned(),
            path: path.to_owned(),
            body: body.as_bytes().to_vec(),
        }
    }

    fn json_of(r: &Response) -> Value {
        serde_json::from_slice(&r.body).unwrap()
    }

    #[test]
    fn routes() {
        let fake = Fake::default();
        let r = route(&fake, &req("GET", "/", ""));
        assert_eq!(
            (r.status, r.content_type),
            (200, "text/plain; charset=utf-8")
        );
        assert_eq!(
            json_of(&route(&fake, &req("GET", "/api/apps", ""))),
            json!({"apps": []})
        );
        assert_eq!(
            json_of(&route(&fake, &req("GET", "/api/sets", ""))),
            json!({"sets": []})
        );
        assert_eq!(route(&fake, &req("POST", "/api/refresh", "")).status, 200);
        assert_eq!(
            route(
                &fake,
                &req(
                    "POST",
                    "/api/refresh",
                    r#"{"serviceType":"Utility","app":"a/b/c/d"}"#
                )
            )
            .status,
            200
        );
        assert_eq!(
            *fake.refreshed.lock().unwrap(),
            vec![None, Some(("Utility".to_owned(), "a/b/c/d".to_owned()))]
        );
        let set = r#"{"serviceType":"Utility","app":"a","source":"a","context":"c","values":{"g":{"o":"v"}}}"#;
        let r = route(&fake, &req("POST", "/api/set", set));
        assert_eq!((r.status, json_of(&r)), (200, json!({"id": 1})));
        let r = route(
            &fake,
            &req(
                "POST",
                "/api/set",
                &set.replace(r#""app":"a""#, r#""app":"offline""#),
            ),
        );
        assert_eq!(
            (r.status, json_of(&r)),
            (409, json!({"error": "the application is offlineGraceful"}))
        );
        assert_eq!(fake.set.lock().unwrap().len(), 2);
    }

    #[test]
    fn refused() {
        let fake = Fake::default();
        for (method, path, body, status) in [
            ("GET", "/nope", "", 404),
            ("POST", "/api/apps", "", 405),
            ("GET", "/api/set", "", 405),
            ("POST", "/api/set", "", 400),
            ("POST", "/api/set", "[]", 400),
            ("POST", "/api/set", "{", 400),
            ("POST", "/api/set", r#"{"serviceType":"Utility"}"#, 400),
            ("POST", "/api/refresh", r#"{"app":"a"}"#, 400),
            (
                "POST",
                "/api/refresh",
                r#"{"serviceType":1,"app":"a"}"#,
                400,
            ),
        ] {
            let r = route(&fake, &req(method, path, body));
            assert_eq!(r.status, status, "{method} {path} {body}");
            assert!(json_of(&r)["error"].is_string());
        }
        let r = route(&fake, &req("DELETE", "/api/set", ""));
        assert_eq!(r.headers, vec![("Allow", "POST".to_owned())]);
        assert!(fake.set.lock().unwrap().is_empty());
    }
}
