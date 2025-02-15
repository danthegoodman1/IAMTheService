use axum::response::{IntoResponse, Response};
use serde::{Deserialize, Serialize};
use serde_json::Value;

// JSON-RPC Error Codes
pub const PARSE_ERROR: i64 = -32700;
pub const INVALID_REQUEST: i64 = -32600;
pub const METHOD_NOT_FOUND: i64 = -32601;
pub const INVALID_PARAMS: i64 = -32602;
pub const INTERNAL_ERROR: i64 = -32603;

// Basic JSON-RPC structures (same as before)
#[derive(Debug, Deserialize, Serialize)]
pub struct JsonRpcRequest {
    pub jsonrpc: String,
    pub method: String,
    pub params: Value,
    pub id: Option<i64>,
}

#[derive(Debug, Serialize)]
pub struct JsonRpcResponseSuccess<T: Serialize> {
    pub jsonrpc: String,
    pub result: T,
    pub id: Option<i64>,
}

impl<T: Serialize> JsonRpcResponseSuccess<T> {
    pub fn with_id(mut self, id: Option<i64>) -> Self {
        self.id = id;
        self
    }
}

#[derive(Debug, Serialize)]
pub struct JsonRpcResponseError<T: Serialize> {
    pub jsonrpc: String,
    pub id: Option<i64>,
    pub data: Option<T>,
    pub code: i64,
}

impl<T: Serialize> JsonRpcResponseError<T> {
    pub fn with_id(mut self, id: Option<i64>) -> Self {
        self.id = id;
        self
    }
}

#[derive(Debug, Serialize)]
pub struct JsonRpcResponse {
    pub jsonrpc: String,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub result: Option<Value>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub id: Option<i64>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<Value>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub code: Option<i64>,
}

impl JsonRpcResponse {
    pub fn with_id(mut self, id: Option<i64>) -> Self {
        self.id = id;
        self
    }
}

impl IntoResponse for JsonRpcResponse {
    fn into_response(self) -> Response {
        match serde_json::to_string(&self) {
            Ok(json) => Response::builder()
                .header("Content-Type", "application/json")
                .body(axum::body::Body::from(json))
                .unwrap_or_else(|_| {
                    Response::builder()
                        .status(500)
                        .body("Failed to create response".into())
                        .unwrap()
                }),
            Err(_) => Response::builder()
                .status(500)
                .body("Failed to serialize response".into())
                .unwrap(),
        }
    }
}

#[derive(Debug, Serialize)]
pub struct InternalError {
    pub message: String,
}

impl From<anyhow::Error> for InternalError {
    fn from(e: anyhow::Error) -> Self {
        Self {
            message: e.to_string(),
        }
    }
}

// Convert a value to a JsonRpcResponseSuccess
impl<T: Serialize> From<T> for JsonRpcResponseSuccess<T> {
    fn from(e: T) -> Self {
        Self {
            jsonrpc: "2.0".to_string(),
            result: e,
            id: None,
        }
    }
}

// Convert a JsonRpcResponseSuccess to a JsonRpcResponse
impl<T: Serialize> From<JsonRpcResponseSuccess<T>> for JsonRpcResponse {
    fn from(e: JsonRpcResponseSuccess<T>) -> Self {
        Self {
            jsonrpc: "2.0".to_string(),
            result: Some(serde_json::to_value(e.result).unwrap()),
            id: e.id,
            data: None,
            code: None,
        }
    }
}

// Convert a JsonRpcResponseSuccess to a Value via JsonRpcResponse
impl<T: Serialize> From<JsonRpcResponseSuccess<T>> for Value {
    fn from(e: JsonRpcResponseSuccess<T>) -> Self {
        let response: JsonRpcResponse = e.into();
        serde_json::to_value(response).unwrap()
    }
}

// Convert a value to a JsonRpcResponseError
impl<T: Serialize> From<T> for JsonRpcResponseError<T> {
    fn from(e: T) -> Self {
        Self {
            jsonrpc: "2.0".to_string(),
            id: None,
            data: Some(e),
            code: INTERNAL_ERROR,
        }
    }
}

// Convert a JsonRpcResponseError to a JsonRpcResponse via JsonRpcResponse
impl<T: Serialize> From<JsonRpcResponseError<T>> for JsonRpcResponse {
    fn from(e: JsonRpcResponseError<T>) -> Self {
        Self {
            jsonrpc: "2.0".to_string(),
            result: None,
            id: e.id,
            data: e.data.map(|d| serde_json::to_value(d).unwrap()),
            code: Some(e.code),
        }
    }
}

// Convert a JsonRpcResponseError to a Value via JsonRpcResponse
impl<T: Serialize> From<JsonRpcResponseError<T>> for Value {
    fn from(e: JsonRpcResponseError<T>) -> Self {
        let response: JsonRpcResponse = e.into();
        serde_json::to_value(response).unwrap()
    }
}

impl From<JsonRpcResponse> for Value {
    fn from(e: JsonRpcResponse) -> Self {
        serde_json::to_value(e).unwrap()
    }
}
