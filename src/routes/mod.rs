// pub mod echo;
mod echo;
use axum::{
    extract::State,
    response::sse::{Event, Sse},
    Json,
};
pub use echo::*;

use axum::{body::Bytes, response::IntoResponse};
use futures::stream::{self, Stream};
use serde::{Deserialize, Serialize};
use serde_json::json;
use std::convert::Infallible;
use std::time::Duration;
use tokio_stream::StreamExt;

use axum::{extract::Request, http::StatusCode};

use crate::{
    json_rpc::{
        self, JsonRpcRequest, JsonRpcResponse, JsonRpcResponseError, JsonRpcResponseSuccess,
    },
    AppError, AppState,
};

pub async fn sse_res() -> Sse<impl Stream<Item = Result<Event, Infallible>>> {
    // Create a stream that yields bytes every 100ms

    let bytes = Bytes::from("Hello, World!\n").to_vec();
    let chunks: Vec<_> = bytes.chunks(3).map(|x| x.to_vec()).collect();

    let s = stream::iter(
        chunks
            .into_iter()
            .map(|x| Ok(Event::default().data(String::from_utf8_lossy(x.as_ref())))),
    )
    .throttle(Duration::from_millis(100))
    .take(10); // Limit to 10 messages

    // Convert the stream into a response
    axum::response::sse::Sse::new(s)
}

pub async fn stream_res() -> impl IntoResponse {
    // Create a stream that yields bytes every 100ms

    // Have to make our own chunks because of the borrow checker
    let bytes = Bytes::from("Hello, World!\n").to_vec();
    let chunks: Vec<_> = bytes.chunks(3).map(|x| x.to_vec()).collect();

    let s = stream::iter(chunks.into_iter().map(|x| Ok::<_, std::io::Error>(x)))
        .throttle(Duration::from_millis(100));

    // Convert the stream into a response
    axum::response::Response::builder()
        .header("Content-Type", "application/octet-stream")
        .body(axum::body::Body::from_stream(s))
        .unwrap()
}

pub async fn stream_handler(request: Request) -> Result<String, (StatusCode, String)> {
    // Convert request body into a stream
    let body_stream = request.into_body().into_data_stream();

    // Collect all chunks into a vector
    let chunks: Vec<_> = body_stream
        .map(|result| {
            result.map_err(|err| {
                (
                    StatusCode::INTERNAL_SERVER_ERROR,
                    format!("Failed to read stream: {}", err),
                )
            })
        })
        .collect::<Result<Vec<Bytes>, _>>()
        .await?;

    // Concatenate all chunks and convert to string
    let full_body = chunks.concat();
    println!("full_body: {:?}", String::from_utf8(full_body.to_vec()));
    String::from_utf8(full_body.to_vec())
        .map_err(|err| (StatusCode::BAD_REQUEST, format!("Invalid UTF-8: {}", err)))
}

pub async fn json_rpc(
    State(_state): State<AppState>, // state must be listed first in params
    Json(payload): Json<JsonRpcRequest>,
) -> Result<Json<serde_json::Value>, AppError> {
    let res: Result<serde_json::Value, anyhow::Error> = match payload.method.as_str() {
        "my_rpc" => match my_rpc(serde_json::from_value(payload.params).unwrap()).await {
            Ok(response) => Ok(response.with_id(payload.id.clone()).into()),
            Err(e) => Err(e),
        },
        "greeting_rpc" => {
            match greeting_rpc(serde_json::from_value(payload.params).unwrap()).await {
                Ok(response) => Ok(response.with_id(payload.id.clone()).into()),
                Err(e) => Err(e),
            }
        }
        _ => Ok(JsonRpcResponseError {
            jsonrpc: payload.jsonrpc.clone(),
            id: payload.id.clone(),
            data: Some::<json_rpc::InternalError>(anyhow::anyhow!("Method not found").into()),
            code: json_rpc::METHOD_NOT_FOUND,
        }
        .into()),
    };

    match res {
        Ok(response) => Ok(Json(response)),
        Err(e) => Ok(Json(
            JsonRpcResponseError {
                jsonrpc: payload.jsonrpc,
                id: payload.id,
                data: Some(json!({ "error": e.to_string() })),
                code: json_rpc::INTERNAL_ERROR,
            }
            .into(),
        )),
    }
}

#[derive(Debug, Serialize, Deserialize)]
pub struct MyRpcParams {
    pub name: String,
}

#[derive(Debug, Serialize)]
pub struct MyRpcResponse {
    pub message: String,
}

pub async fn my_rpc(params: MyRpcParams) -> Result<JsonRpcResponse, anyhow::Error> {
    if params.name == "error" {
        return Ok(JsonRpcResponseError {
            jsonrpc: "2.0".to_string(),
            id: Some(1),
            data: Some(json!({ "error": "Internal error" })),
            code: json_rpc::INTERNAL_ERROR,
        }
        .into());
    }
    Ok(JsonRpcResponseSuccess::from(MyRpcResponse {
        message: format!("Hello, {}!", params.name),
    })
    .into())
}

#[derive(Debug, Serialize, Deserialize)]
pub struct GreetingRpcParams {
    pub name: String,
    pub language: String,
}

#[derive(Debug, Serialize)]
pub struct GreetingRpcResponse {
    pub greeting: String,
    pub translated: bool,
}

async fn greeting_rpc(params: GreetingRpcParams) -> Result<JsonRpcResponse, anyhow::Error> {
    let greeting = match params.language.to_lowercase().as_str() {
        "spanish" => format!("¡Hola, {}!", params.name),
        "french" => format!("Bonjour, {}!", params.name),
        _ => format!("Hello, {}!", params.name),
    };

    Ok(JsonRpcResponseSuccess::from(GreetingRpcResponse {
        greeting,
        translated: params.language.to_lowercase() != "english",
    })
    .into())
}
